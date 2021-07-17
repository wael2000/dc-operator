/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"
	//"github.com/prometheus/common/log"
	appv1 "github.com/wael2000/dc-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// DeploymentConfigReconciler reconciles a DeploymentConfig object
type DeploymentConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=app.stakater.com,resources=deploymentconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=app.stakater.com,resources=deploymentconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=app.stakater.com,resources=deploymentconfigs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DeploymentConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *DeploymentConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// your logic here
	_ = context.Background()
	//_ = r.Log.WithValues("deploymentconfig", req.NamespacedName)

	// Fetch the DeploymentConfig instance
	instance := &appv1.DeploymentConfig{}
	err := r.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	deploymentConfig := instance

	// Check if the deployment already exists, if not create a new one
	associatedDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deploymentConfig.Name, Namespace: deploymentConfig.Namespace}, associatedDeployment)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep := r.createNewDeployment(deploymentConfig)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		// Deployment created successfully - return and requeue
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the deployment size is the same as the spec
	// This is required in case you edit the deploymentconfig object and changing the replicas
	replicas := deploymentConfig.Spec.Replicas
	if *associatedDeployment.Spec.Replicas != replicas {
		associatedDeployment.Spec.Replicas = &replicas
		err = r.Update(ctx, associatedDeployment)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", associatedDeployment.Namespace, "Deployment.Name", associatedDeployment.Name)
			return ctrl.Result{}, err
		}
		// Spec updated - return and requeue
		return ctrl.Result{Requeue: true}, nil
	}

	// Update the status with AvailabeReplicas from Deployment Object
	status := appv1.DeploymentConfigStatus{
		AvailableReplicas: associatedDeployment.Status.AvailableReplicas,
	}

	if !reflect.DeepEqual(deploymentConfig.Status, status) {
		deploymentConfig.Status = status
		err = r.Status().Update(context.TODO(), deploymentConfig)
		if err != nil {
			r.Log.Error(err, "Failed to update DeploymentConfig status")
			return reconcile.Result{}, err
		}
	}
	// Log the message passed along with the DeploymentConfig Object
	log.Info(deploymentConfig.Spec.Message)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1.DeploymentConfig{}).
		Complete(r)
}

// createNewDeployment returns Deployment object
func (r *DeploymentConfigReconciler) createNewDeployment(dc *appv1.DeploymentConfig) *appsv1.Deployment {
	ls := labelsForDeploymentConfig(dc.Name)
	replicas := dc.Spec.Replicas

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dc.Name,
			Namespace: dc.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    "busybox",
						Image:   "busybox",
						Command: []string{"sleep", "3600"},
					}},
				},
			},
		},
	}
	// Set DeploymentConfig instance as the owner and controller
	ctrl.SetControllerReference(dc, dep, r.Scheme)
	return dep
}

// labelsForDeploymentConfig returns the labels for selecting the resources
// belonging to the given deploymentconfig CR name.
func labelsForDeploymentConfig(name string) map[string]string {
	return map[string]string{"app": "deploymentconfig", "deploymentconfig_cr": name}
}

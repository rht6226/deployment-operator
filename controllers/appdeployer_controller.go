/*
Copyright 2022.

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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deployerv1alpha1 "radisys.com/app-deployer/api/v1alpha1"
)

// AppDeployerReconciler reconciles a AppDeployer object
type AppDeployerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=deployer.radisys.com,resources=appdeployers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=deployer.radisys.com,resources=appdeployers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=deployer.radisys.com,resources=appdeployers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppDeployer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *AppDeployerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reading the CR")

	cr := &deployerv1alpha1.AppDeployer{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, cr); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("resource not found. must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "failed to get the resource")
		return ctrl.Result{}, err

	}

	// create deployment from cr

	deployment := &appsv1.Deployment{}

	if err := r.Get(ctx, types.NamespacedName{
		Name:      fmt.Sprintf("%s-deployment", cr.Name),
		Namespace: cr.Namespace,
	}, deployment); err != nil {

		if errors.IsNotFound(err) {
			// deployemnt does not exists create it

			deployment := r.getDeployment(cr)

			if err := r.Create(ctx, deployment); err != nil {
				logger.Error(err, "Failed to create the deployment")
				return ctrl.Result{}, err
			}

			logger.Info("Deployment created sucessfully:" + fmt.Sprintf("%s-deployment", cr.Name))

		} else {
			// some other error occured
			logger.Error(err, "Unable to read the deployment")
		}
	} else {
		// Deployment already exist
		// Here we can write code for updating the deployment if any change in spec occurs
		logger.Info("Deployment Already exists:" + fmt.Sprintf("%s-deployment", cr.Name))

		if !reflect.DeepEqual(r.getDeploymentSpecFromExisting(deployment), r.getDeployment(cr).Spec) {
			logger.Info("Updating Deployment...")
			deployment.Spec = r.getDeployment(cr).Spec
			err = r.Update(ctx, deployment)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// create service from cr
	if cr.Spec.CreateService {

		service := &corev1.Service{}

		if err := r.Get(ctx, types.NamespacedName{
			Name:      fmt.Sprintf("%s-service", cr.Name),
			Namespace: cr.Namespace,
		}, service); err != nil {
			if errors.IsNotFound(err) {
				// service does not exists create it

				service := r.getService(cr)

				if err := r.Create(ctx, service); err != nil {
					logger.Error(err, "Failed to create the service")
					return ctrl.Result{}, err
				}

				logger.Info("Service created sucessfully:" + fmt.Sprintf("%s-service", cr.Name))

			} else {
				// some other error occured
				logger.Error(err, "Unable to read the service")
			}
		} else {
			// service already exists
			// Here we can write the code to update the service if any change in spec occurs
			logger.Info("Service already exists: " + fmt.Sprintf("%s-service", cr.Name))

			if svcSpec := r.getService(cr).Spec; !reflect.DeepEqual(r.getServiceSpecFromExisting(service), svcSpec) {
				logger.Info("Updating service...")
				service.Spec = svcSpec
				err = r.Update(ctx, service)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

// get deployment
func (r *AppDeployerReconciler) getDeployment(cr *deployerv1alpha1.AppDeployer) *appsv1.Deployment {

	labels := map[string]string{
		"app": fmt.Sprintf("%s-deployment", cr.Name),
	}

	deployemnt := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-deployment", cr.Name),
			Namespace: cr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},

				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  cr.Name,
							Image: cr.Spec.ImageName,
							Ports: []corev1.ContainerPort{
								{
									Name:          "service-port",
									ContainerPort: cr.Spec.Port,
								},
							},
						},
					},
				},
			},
		},
	}

	controllerutil.SetOwnerReference(cr, deployemnt, r.Scheme)
	return deployemnt
}

func (r *AppDeployerReconciler) getDeploymentSpecFromExisting(existing *appsv1.Deployment) *appsv1.DeploymentSpec {

	return &appsv1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: existing.Labels,
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: existing.Spec.Template.Labels,
			},

			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  existing.Spec.Template.Spec.Containers[0].Name,
						Image: existing.Spec.Template.Spec.Containers[0].Image,
						Ports: []corev1.ContainerPort{
							{
								Name:          "service-port",
								ContainerPort: existing.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort,
							},
						},
					},
				},
			},
		},
	}
}

// get service
func (r *AppDeployerReconciler) getService(cr *deployerv1alpha1.AppDeployer) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-service", cr.Name),
			Namespace: cr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": fmt.Sprintf("%s-deployment", cr.Name),
			},
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       cr.Spec.Port,
					TargetPort: intstr.IntOrString{IntVal: cr.Spec.Port},
				},
			},
		},
	}

	controllerutil.SetControllerReference(cr, service, r.Scheme)
	return service
}

func (r *AppDeployerReconciler) getServiceSpecFromExisting(existing *corev1.Service) *corev1.ServiceSpec {
	return &corev1.ServiceSpec{
		Type:     existing.Spec.Type,
		Selector: existing.Spec.Selector,
		Ports:    existing.Spec.Ports,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppDeployerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deployerv1alpha1.AppDeployer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

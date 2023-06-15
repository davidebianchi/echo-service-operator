/*
Copyright 2023.

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

package controller

import (
	"context"
	"fmt"

	"github.com/davidebianchi/echo-service-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	echoServicePortName = "echo-service"

	typeAvailableEchoService = "Available"
)

// EchoServiceReconciler reconciles a EchoService object
type EchoServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=echo-service-operator.davidebianchi.github.io,resources=echoservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=echo-service-operator.davidebianchi.github.io,resources=echoservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=echo-service-operator.davidebianchi.github.io,resources=echoservices/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EchoService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *EchoServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	echoService := &v1alpha1.EchoService{}
	if err := r.Get(ctx, req.NamespacedName, echoService); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("EchoService resource not found. Object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "fails to retrieve echo service")
		return ctrl.Result{}, err
	}

	deploy, err := r.getDeployment(echoService)
	if err != nil {
		log.Error(err, "fails to define deployment")
		return ctrl.Result{}, err
	}
	actualDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, req.NamespacedName, actualDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			if err = r.Create(ctx, deploy); err != nil {
				log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.Update(ctx, deploy); err != nil {
			log.Error(err, "Failed to update Deployment", "Service.Namespace", deploy.Namespace, "Service.Name", deploy.Name)
			return ctrl.Result{}, err
		}
	}

	svc, err := r.getService(echoService)
	if err != nil {
		log.Error(err, "fails to define service")
		return ctrl.Result{}, err
	}
	actualService := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, actualService); err != nil {
		if apierrors.IsNotFound(err) {
			if err = r.Create(ctx, svc); err != nil {
				log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.Update(ctx, deploy); err != nil {
			log.Error(err, "Failed to update Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return ctrl.Result{}, err
		}
	}

	meta.SetStatusCondition(&echoService.Status.Conditions, metav1.Condition{
		Type:    typeAvailableEchoService,
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) created successfully", echoService.Name),
	})

	if err := r.Status().Update(ctx, echoService); err != nil {
		log.Error(err, "Failed to update EchoService status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EchoServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.EchoService{}).
		Complete(r)
}

func (r *EchoServiceReconciler) getService(echoService *v1alpha1.EchoService) (*corev1.Service, error) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      echoService.GetName(),
			Namespace: echoService.GetNamespace(),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromString(echoServicePortName),
				},
			},
			Selector: map[string]string{
				"app": echoService.GetName(),
			},
		},
	}

	// // Set the ownerRef for the Deployment
	// // More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(echoService, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

func (r *EchoServiceReconciler) getDeployment(echoService *v1alpha1.EchoService) (*appsv1.Deployment, error) {
	replicas := echoService.GetReplicas()
	image := echoService.GetImage()
	env := []corev1.EnvVar{}
	if echoService.Spec.Config.ResponseDelay != "" {
		env = append(env, corev1.EnvVar{
			Name:  "RESPONSE_DELAY",
			Value: echoService.Spec.Config.ResponseDelay,
		})
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      echoService.GetName(),
			Namespace: echoService.GetNamespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": echoService.GetName(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": echoService.GetName(),
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						// IMPORTANT: seccomProfile was introduced with Kubernetes 1.19
						// If you are looking for to produce solutions to be supported
						// on lower versions you must remove this option.
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "echo-service",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env:             env,
						// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{1001}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          echoServicePortName,
						}},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromString(echoServicePortName),
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromString(echoServicePortName),
								},
							},
						},
					}},
				},
			},
		},
	}

	// // Set the ownerRef for the Deployment
	// // More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(echoService, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

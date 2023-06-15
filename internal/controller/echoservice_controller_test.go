package controller

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/davidebianchi/echo-service-operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("EchoService controller", func() {
	Context("EchoService controller test", func() {
		const EchoServiceName = "test-echo"
		var echoServiceNamespace string
		ctx := context.Background()

		var namespace *corev1.Namespace
		BeforeEach(func() {
			echoServiceNamespace = randString(12)

			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      echoServiceNamespace,
					Namespace: echoServiceNamespace,
				},
			}

			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// TODO(user): Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)
		})

		It("should successfully reconcile a custom resource for EchoService", func() {
			typeNamespaceName := types.NamespacedName{Name: EchoServiceName, Namespace: echoServiceNamespace}

			getOwnerRef := func(obj client.Object) []metav1.OwnerReference {
				return []metav1.OwnerReference{
					{
						Kind:               "EchoService",
						APIVersion:         "echo-service-operator.davidebianchi.github.io/v1alpha1",
						Name:               EchoServiceName,
						UID:                obj.GetOwnerReferences()[0].UID,
						Controller:         getPtr(true),
						BlockOwnerDeletion: getPtr(true),
					},
				}
			}

			By("Creating the custom resource for the Kind EchoService with empty Spec")
			echoService := &v1alpha1.EchoService{}
			err := k8sClient.Get(ctx, typeNamespaceName, echoService)
			if err != nil && apierrors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				echoService = &v1alpha1.EchoService{
					ObjectMeta: metav1.ObjectMeta{
						Name:      EchoServiceName,
						Namespace: namespace.Name,
					},
					Spec: v1alpha1.EchoServiceSpec{},
				}

				err = k8sClient.Create(ctx, echoService)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &v1alpha1.EchoService{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			echoServiceReconciler := &EchoServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = echoServiceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was successfully created in the reconciliation")
			deploy := &appsv1.Deployment{}

			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespaceName, deploy)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(deploy.Spec.Replicas).To(Equal(getPtr(int32(1))))
			Expect(deploy.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deploy.Spec.Template.Spec.Containers[0]).To(Equal(corev1.Container{
				Name:            "echo-service",
				Image:           "davidebianchi/echo-service:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{{
					ContainerPort: 8080,
					Name:          echoServicePortName,
					Protocol:      corev1.ProtocolTCP,
				}},
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
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: "File",
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/-/ready",
							Port:   intstr.FromString(echoServicePortName),
							Scheme: "HTTP",
						},
					},
					TimeoutSeconds:   1,
					PeriodSeconds:    10,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/-/healthz",
							Port:   intstr.FromString(echoServicePortName),
							Scheme: "HTTP",
						},
					},
					TimeoutSeconds:   1,
					PeriodSeconds:    10,
					SuccessThreshold: 1,
					FailureThreshold: 3,
				},
			}))
			Expect(deploy.OwnerReferences).To(Equal(getOwnerRef(deploy)))

			By("Checking if Service was successfully created in the reconciliation")
			svc := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespaceName, svc)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(svc.Spec.Ports).To(Equal([]corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromString(echoServicePortName),
				},
			}))
			Expect(svc.OwnerReferences).To(Equal(getOwnerRef(svc)))

			By("Check that deploy and service selector are equal")
			Expect(deploy.Spec.Selector.MatchLabels).To(Equal(svc.Spec.Selector))

			By("Check status")
			echoService = &v1alpha1.EchoService{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespaceName, echoService)
			}, time.Minute, time.Second).Should(Succeed())

			Expect(echoService.Status.Conditions).To(HaveLen(1))
			echoService.Status.Conditions[0].LastTransitionTime = metav1.Time{}
			Expect(echoService.Status.Conditions).To(Equal([]metav1.Condition{
				{
					Type:    "Available",
					Status:  "True",
					Reason:  "Reconciling",
					Message: "Deployment for custom resource (test-echo) created successfully",
				},
			}))

			By("update existent EchoService", func() {
				By("Apply the custom resource for the Kind EchoService with ResponseDelay")
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				echoService := &v1alpha1.EchoService{
					TypeMeta: metav1.TypeMeta{
						Kind:       "EchoService",
						APIVersion: "echo-service-operator.davidebianchi.github.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      EchoServiceName,
						Namespace: namespace.Name,
					},
					Spec: v1alpha1.EchoServiceSpec{
						Replicas: getPtr(int32(3)),
					},
				}

				err := k8sClient.Patch(ctx, echoService, client.Apply, &client.PatchOptions{
					FieldManager: "echo-service-controller",
					Force:        getPtr(true),
				})
				Expect(err).To(Not(HaveOccurred()))

				By("Checking if the custom resource was successfully updated")
				Eventually(func() error {
					found := &v1alpha1.EchoService{}
					if err := k8sClient.Get(ctx, typeNamespaceName, found); err != nil {
						return err
					}
					if *found.GetReplicas() != *echoService.GetReplicas() {
						return fmt.Errorf("not correct replicas: %v - %v", *found.GetReplicas(), *echoService.GetReplicas())
					}
					return nil
				}, time.Minute, time.Second).Should(Succeed())

				By("Reconciling the custom resource created")
				echoServiceReconciler := &EchoServiceReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err = echoServiceReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				Expect(err).To(Not(HaveOccurred()))

				By("Checking if Deployment was successfully created in the reconciliation")
				Eventually(func() error {
					found := &appsv1.Deployment{}
					err := k8sClient.Get(ctx, typeNamespaceName, found)
					if err != nil {
						return err
					}
					replicas := found.Spec.Replicas
					if replicas != nil && *replicas == *echoService.GetReplicas() {
						return nil
					}
					return fmt.Errorf("RESPONSE_DELAY not correctly set: %v", found)
				}, time.Minute, time.Second).Should(Succeed())
			})

			By("delete the EchoService resource", func() {
				err := k8sClient.Delete(ctx, echoService)
				Expect(err).ToNot(HaveOccurred())

				Eventually(func() error {
					found := &v1alpha1.EchoService{}
					err := k8sClient.Get(ctx, typeNamespaceName, found)
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return err
					}
					return fmt.Errorf("EchoService not yet deleted")
				}, time.Minute, time.Second).Should(Succeed())

				By("Reconciling the custom resource deleted")
				echoServiceReconciler := &EchoServiceReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				result, err := echoServiceReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: typeNamespaceName,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		It("add responseDelay config", func() {
			typeNamespaceName := types.NamespacedName{Name: EchoServiceName, Namespace: echoServiceNamespace}

			By("Apply the custom resource for the Kind EchoService with ResponseDelay")
			// Let's mock our custom resource at the same way that we would
			// apply on the cluster the manifest under config/samples
			echoService := &v1alpha1.EchoService{
				TypeMeta: metav1.TypeMeta{
					Kind:       "EchoService",
					APIVersion: "echo-service-operator.davidebianchi.github.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      EchoServiceName,
					Namespace: namespace.Name,
				},
				Spec: v1alpha1.EchoServiceSpec{
					Config: v1alpha1.EchoServiceConfig{
						ResponseDelay: "3s",
					},
				},
			}

			err := k8sClient.Patch(ctx, echoService, client.Apply, &client.PatchOptions{
				FieldManager: "echo-service-controller",
				Force:        getPtr(true),
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if the custom resource was successfully updated")
			Eventually(func() error {
				found := &v1alpha1.EchoService{}
				err := k8sClient.Get(ctx, typeNamespaceName, found)
				if err != nil {
					return err
				}
				if found.Spec.Config.ResponseDelay != echoService.Spec.Config.ResponseDelay {
					return fmt.Errorf("not correct responseDelay")
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			echoServiceReconciler := &EchoServiceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = echoServiceReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, typeNamespaceName, found)
				if err != nil {
					return err
				}
				env := found.Spec.Template.Spec.Containers[0].Env
				for _, envVar := range env {
					if envVar.Name == "RESPONSE_DELAY" && envVar.Value == "3s" {
						return nil
					}
				}
				return fmt.Errorf("RESPONSE_DELAY not correctly set: %v", found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking if Service was successfully created in the reconciliation")
			Eventually(func() error {
				found := &corev1.Service{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())
		})

	})
})

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		random, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterBytes))))
		if err != nil {
			panic(fmt.Errorf("random error: %s", err))
		}
		b[i] = letterBytes[random.Int64()]
	}
	return string(b)
}

func getPtr[T any](item T) *T {
	return &item
}

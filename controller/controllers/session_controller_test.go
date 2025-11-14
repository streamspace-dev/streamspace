package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
)

var _ = Describe("Session Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating a new Session", func() {
		It("Should create a Deployment for running state", func() {
			ctx := context.Background()

			// Create a Template first
			template := &streamv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.TemplateSpec{
					DisplayName: "Test Template",
					BaseImage:   "lscr.io/linuxserver/firefox:latest",
					DefaultResources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("1000m"),
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "vnc",
							ContainerPort: 3000,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VNC: &streamv1alpha1.VNCConfig{
						Enabled:  true,
						Port:     3000,
						Protocol: "websocket",
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create a Session
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "test-template",
					State:          "running",
					PersistentHome: true,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
							corev1.ResourceCPU:    resource.MustParse("1000m"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Verify Deployment is created
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-test-session",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			Expect(deployment.Spec.Replicas).To(Equal(int32Ptr(1)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(Equal("lscr.io/linuxserver/firefox:latest"))
		})

		It("Should scale Deployment to 0 for hibernated state", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-session",
				Namespace: "default",
			}, session)).To(Succeed())

			// Update session to hibernated
			session.Spec.State = "hibernated"
			Expect(k8sClient.Update(ctx, session)).To(Succeed())

			// Verify Deployment is scaled to 0
			deployment := &appsv1.Deployment{}
			Eventually(func() int32 {
				_ = k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-test-session",
					Namespace: "default",
				}, deployment)
				if deployment.Spec.Replicas != nil {
					return *deployment.Spec.Replicas
				}
				return -1
			}, timeout, interval).Should(Equal(int32(0)))
		})

		It("Should create a Service for the session", func() {
			ctx := context.Background()

			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-test-session-svc",
					Namespace: "default",
				}, service)
			}, timeout, interval).Should(Succeed())

			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(3000)))
			Expect(service.Spec.Selector["session"]).To(Equal("test-session"))
		})

		It("Should create a PVC for persistent home", func() {
			ctx := context.Background()

			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "home-testuser",
					Namespace: "default",
				}, pvc)
			}, timeout, interval).Should(Succeed())

			Expect(pvc.Spec.AccessModes).To(ContainElement(corev1.ReadWriteMany))
			Expect(pvc.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("50Gi")))
		})
	})

	Context("When reconciling session status", func() {
		It("Should update session status with pod information", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{}
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{
					Name:      "test-session",
					Namespace: "default",
				}, session)
				return session.Status.Phase
			}, timeout, interval).ShouldNot(BeEmpty())

			Expect(session.Status.URL).ToNot(BeEmpty())
		})
	})
})

var _ = Describe("Session Controller State Transitions", func() {
	It("Should handle running -> hibernated -> running transition", func() {
		ctx := context.Background()

		// Get existing session
		session := &streamv1alpha1.Session{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      "test-session",
			Namespace: "default",
		}, session)).To(Succeed())

		// Ensure it's running first
		session.Spec.State = "running"
		Expect(k8sClient.Update(ctx, session)).To(Succeed())

		// Wait for deployment to scale up
		deployment := &appsv1.Deployment{}
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-test-session",
				Namespace: "default",
			}, deployment)
			if deployment.Spec.Replicas != nil {
				return *deployment.Spec.Replicas
			}
			return -1
		}, time.Second*5, time.Millisecond*100).Should(Equal(int32(1)))

		// Hibernate
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      "test-session",
			Namespace: "default",
		}, session)).To(Succeed())
		session.Spec.State = "hibernated"
		Expect(k8sClient.Update(ctx, session)).To(Succeed())

		// Wait for deployment to scale down
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-test-session",
				Namespace: "default",
			}, deployment)
			if deployment.Spec.Replicas != nil {
				return *deployment.Spec.Replicas
			}
			return -1
		}, time.Second*5, time.Millisecond*100).Should(Equal(int32(0)))

		// Resume (back to running)
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      "test-session",
			Namespace: "default",
		}, session)).To(Succeed())
		session.Spec.State = "running"
		Expect(k8sClient.Update(ctx, session)).To(Succeed())

		// Wait for deployment to scale up again
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-test-session",
				Namespace: "default",
			}, deployment)
			if deployment.Spec.Replicas != nil {
				return *deployment.Spec.Replicas
			}
			return -1
		}, time.Second*5, time.Millisecond*100).Should(Equal(int32(1)))
	})
})

// Helper function to create int32 pointer
func int32Ptr(i int32) *int32 {
	return &i
}

// Cleanup function to run after tests
var _ = AfterSuite(func() {
	ctx := context.Background()

	// Clean up test resources
	session := &streamv1alpha1.Session{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-session",
			Namespace: "default",
		},
	}
	_ = k8sClient.Delete(ctx, session)

	template := &streamv1alpha1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-template",
			Namespace: "default",
		},
	}
	_ = k8sClient.Delete(ctx, template)
})

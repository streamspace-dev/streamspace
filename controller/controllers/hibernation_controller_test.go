package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	streamv1alpha1 "github.com/streamspace/streamspace/api/v1alpha1"
)

var _ = Describe("Hibernation Controller", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	Context("When a Session has an idle timeout", func() {
		It("Should hibernate the session after idle timeout", func() {
			ctx := context.Background()

			// Create template
			template := &streamv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hibernate-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.TemplateSpec{
					DisplayName: "Hibernate Test Template",
					BaseImage:   "lscr.io/linuxserver/firefox:latest",
					DefaultResources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
							corev1.ResourceCPU:    resource.MustParse("500m"),
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "vnc",
							ContainerPort: 3000,
						},
					},
					VNC: &streamv1alpha1.VNCConfig{
						Enabled: true,
						Port:    3000,
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create session with very short idle timeout (3 seconds for testing)
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "hibernate-test-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "hibernate-template",
					State:          "running",
					IdleTimeout:    "3s", // Very short for testing
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Set last activity to 5 seconds ago (exceeds 3s timeout)
			createdSession := &streamv1alpha1.Session{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "hibernate-test-session",
					Namespace: "default",
				}, createdSession)
			}, timeout, interval).Should(Succeed())

			pastTime := metav1.NewTime(time.Now().Add(-5 * time.Second))
			createdSession.Status.LastActivity = &pastTime
			Expect(k8sClient.Status().Update(ctx, createdSession)).To(Succeed())

			// Wait for hibernation controller to hibernate the session
			// The hibernation controller checks periodically, so this may take a few seconds
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "hibernate-test-session",
					Namespace: "default",
				}, createdSession)
				if err != nil {
					return ""
				}
				return createdSession.Spec.State
			}, timeout, interval).Should(Equal("hibernated"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
		})

		It("Should not hibernate if last activity is recent", func() {
			ctx := context.Background()

			// Create session with 30 minute idle timeout
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "active-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "activeuser",
					Template:       "hibernate-template",
					State:          "running",
					IdleTimeout:    "30m",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Set last activity to now (recently active)
			createdSession := &streamv1alpha1.Session{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "active-session",
					Namespace: "default",
				}, createdSession)
			}, timeout, interval).Should(Succeed())

			now := metav1.Now()
			createdSession.Status.LastActivity = &now
			Expect(k8sClient.Status().Update(ctx, createdSession)).To(Succeed())

			// Wait a bit and verify session is still running
			time.Sleep(3 * time.Second)

			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "active-session",
				Namespace: "default",
			}, createdSession)).To(Succeed())

			Expect(createdSession.Spec.State).To(Equal("running"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})

		It("Should skip sessions without idle timeout", func() {
			ctx := context.Background()

			// Create session without idle timeout
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-timeout-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "notimeoutuser",
					Template:       "hibernate-template",
					State:          "running",
					// No IdleTimeout specified
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait a bit
			time.Sleep(3 * time.Second)

			// Verify session is still running
			createdSession := &streamv1alpha1.Session{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "no-timeout-session",
				Namespace: "default",
			}, createdSession)).To(Succeed())

			Expect(createdSession.Spec.State).To(Equal("running"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})

	Context("When a Session is not in running state", func() {
		It("Should skip hibernated sessions", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "already-hibernated-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "hibernateduser",
					Template:       "hibernate-template",
					State:          "hibernated",
					IdleTimeout:    "1s",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait a bit
			time.Sleep(3 * time.Second)

			// Verify session remains hibernated (not re-processed)
			createdSession := &streamv1alpha1.Session{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "already-hibernated-session",
				Namespace: "default",
			}, createdSession)).To(Succeed())

			Expect(createdSession.Spec.State).To(Equal("hibernated"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})
})

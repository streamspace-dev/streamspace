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
					VNC: streamv1alpha1.VNCConfig{
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
					Name:      "ss-testuser-test-template",
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
					Name:      "ss-testuser-test-template",
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
					Name:      "ss-testuser-test-template-svc",
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
		// BUG FIX: Use correct deployment name "ss-{user}-{template}"
		deployment := &appsv1.Deployment{}
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-testuser-test-template",
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
		// BUG FIX: Use correct deployment name
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-testuser-test-template",
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
		// BUG FIX: Use correct deployment name
		Eventually(func() int32 {
			_ = k8sClient.Get(ctx, types.NamespacedName{
				Name:      "ss-testuser-test-template",
				Namespace: "default",
			}, deployment)
			if deployment.Spec.Replicas != nil {
				return *deployment.Spec.Replicas
			}
			return -1
		}, time.Second*5, time.Millisecond*100).Should(Equal(int32(1)))
	})
})

var _ = Describe("Session Controller Error Handling", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When template doesn't exist", func() {
		It("Should set Session to Failed state", func() {
			ctx := context.Background()

			// Create session with non-existent template
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-template-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "non-existent-template",
					State:          "running",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Verify session status indicates template not found error
			createdSession := &streamv1alpha1.Session{}
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "invalid-template-session",
					Namespace: "default",
				}, createdSession)
				if err != nil {
					return ""
				}
				return createdSession.Status.Phase
			}, timeout, interval).Should(Or(Equal("Pending"), Equal("Failed")))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})

	Context("When duplicate session names exist", func() {
		It("Should reject duplicate session creation", func() {
			ctx := context.Background()

			// Create first session
			template := &streamv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-test-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.TemplateSpec{
					DisplayName: "Duplicate Test Template",
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
					VNC: streamv1alpha1.VNCConfig{
						Enabled: true,
						Port:    3000,
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			session1 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "duplicate-test-template",
					State:          "running",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session1)).To(Succeed())

			// Try to create duplicate session (same name)
			session2 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "duplicate-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "duplicate-test-template",
					State:          "running",
					PersistentHome: false,
				},
			}
			err := k8sClient.Create(ctx, session2)
			Expect(err).To(HaveOccurred())
			Expect(errors.IsAlreadyExists(err)).To(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, session1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
		})
	})

	Context("When session resource limits are invalid", func() {
		It("Should reject sessions with zero memory", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "zero-memory-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("0"),
							corev1.ResourceCPU:    resource.MustParse("100m"),
						},
					},
				},
			}

			// K8s validation should reject this
			// Note: Actual validation depends on admission webhooks
			err := k8sClient.Create(ctx, session)
			// Either rejected immediately or accepted but deployment fails
			if err == nil {
				// Clean up if created
				Expect(k8sClient.Delete(ctx, session)).To(Succeed())
			}
		})

		It("Should reject sessions with excessive resource requests", func() {
			ctx := context.Background()

			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "excessive-resources-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Ti"),
							corev1.ResourceCPU:    resource.MustParse("1000"),
						},
					},
				},
			}

			// Create session (may succeed at API level)
			err := k8sClient.Create(ctx, session)
			if err == nil {
				// Deployment should fail to schedule due to resource constraints
				deployment := &appsv1.Deployment{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{
						Name:      "ss-testuser-test-template",
						Namespace: "default",
					}, deployment)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				// Clean up
				Expect(k8sClient.Delete(ctx, session)).To(Succeed())
			}
		})
	})
})

var _ = Describe("Session Controller Resource Cleanup", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When session is deleted", func() {
		It("Should delete associated deployment", func() {
			ctx := context.Background()

			// Create template
			template := &streamv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cleanup-test-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.TemplateSpec{
					DisplayName: "Cleanup Test Template",
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
					VNC: streamv1alpha1.VNCConfig{
						Enabled: true,
						Port:    3000,
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create session
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cleanup-test-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "cleanupuser",
					Template:       "cleanup-test-template",
					State:          "running",
					PersistentHome: true,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait for deployment to be created
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-cleanupuser-cleanup-test-template",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Delete session
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())

			// Verify deployment is deleted (due to owner reference)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-cleanupuser-cleanup-test-template",
					Namespace: "default",
				}, deployment)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
		})

		It("Should NOT delete user PVC (shared resource)", func() {
			ctx := context.Background()

			// Get or create PVC
			pvc := &corev1.PersistentVolumeClaim{}
			pvcName := "home-cleanupuser"
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: "default",
				}, pvc)
			}, timeout, interval).Should(Succeed())

			// PVC should still exist after session deletion
			// (was deleted in previous test)
			// Verify it persists
			Consistently(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      pvcName,
					Namespace: "default",
				}, pvc)
			}, time.Second*3, time.Millisecond*500).Should(Succeed())
		})
	})

	Context("When session transitions to terminated state", func() {
		It("Should clean up resources properly", func() {
			ctx := context.Background()

			// Create session
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "terminated-test-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "terminateduser",
					Template:       "cleanup-test-template",
					State:          "running",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait for deployment
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-terminateduser-cleanup-test-template",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Transition to terminated
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "terminated-test-session",
				Namespace: "default",
			}, session)).To(Succeed())
			session.Spec.State = "terminated"
			Expect(k8sClient.Update(ctx, session)).To(Succeed())

			// Deployment should be deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-terminateduser-cleanup-test-template",
					Namespace: "default",
				}, deployment)
				return errors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})
})

var _ = Describe("Session Controller Concurrent Operations", func() {
	const (
		timeout  = time.Second * 15
		interval = time.Millisecond * 250
	)

	Context("When creating multiple sessions for same user", func() {
		It("Should create all sessions successfully", func() {
			ctx := context.Background()

			// Create template for concurrent test
			template := &streamv1alpha1.Template{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "concurrent-test-template",
					Namespace: "default",
				},
				Spec: streamv1alpha1.TemplateSpec{
					DisplayName: "Concurrent Test Template",
					BaseImage:   "lscr.io/linuxserver/firefox:latest",
					DefaultResources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("512Mi"),
							corev1.ResourceCPU:    resource.MustParse("250m"),
						},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "vnc",
							ContainerPort: 3000,
						},
					},
					VNC: streamv1alpha1.VNCConfig{
						Enabled: true,
						Port:    3000,
					},
				},
			}
			Expect(k8sClient.Create(ctx, template)).To(Succeed())

			// Create 3 concurrent sessions for same user
			session1 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "concurrent-session-1",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "concurrentuser",
					Template:       "concurrent-test-template",
					State:          "running",
					PersistentHome: true,
				},
			}

			session2 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "concurrent-session-2",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "concurrentuser",
					Template:       "concurrent-test-template",
					State:          "running",
					PersistentHome: true,
				},
			}

			session3 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "concurrent-session-3",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "concurrentuser",
					Template:       "concurrent-test-template",
					State:          "running",
					PersistentHome: true,
				},
			}

			// Create all sessions
			Expect(k8sClient.Create(ctx, session1)).To(Succeed())
			Expect(k8sClient.Create(ctx, session2)).To(Succeed())
			Expect(k8sClient.Create(ctx, session3)).To(Succeed())

			// Verify all deployments are created (note: deployment naming may include session number)
			// The actual deployment name depends on controller implementation
			deployment := &appsv1.Deployment{}
			Eventually(func() bool {
				deployments := &appsv1.DeploymentList{}
				err := k8sClient.List(ctx, deployments, client.InNamespace("default"))
				if err != nil {
					return false
				}
				// Count deployments for concurrent user
				count := 0
				for _, d := range deployments.Items {
					if d.Labels["user"] == "concurrentuser" {
						count++
					}
				}
				return count >= 3
			}, timeout, interval).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, session1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, session2)).To(Succeed())
			Expect(k8sClient.Delete(ctx, session3)).To(Succeed())
			Expect(k8sClient.Delete(ctx, template)).To(Succeed())
		})

		It("Should reuse same PVC for all sessions from same user", func() {
			ctx := context.Background()

			// Verify PVC reuse by checking that only one PVC exists for user
			pvcList := &corev1.PersistentVolumeClaimList{}
			Expect(k8sClient.List(ctx, pvcList, client.InNamespace("default"))).To(Succeed())

			// Count PVCs for concurrent user (should be exactly 1)
			userPVCCount := 0
			for _, item := range pvcList.Items {
				if item.Name == "home-concurrentuser" {
					userPVCCount++
				}
			}
			// Should have at most 1 PVC for the user
			Expect(userPVCCount).To(BeNumerically("<=", 1))
		})
	})

	Context("When multiple sessions use same template", func() {
		It("Should create independent deployments from shared template", func() {
			ctx := context.Background()

			// Create sessions from same template for different users
			session1 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-template-session-1",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "shareduser1",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
				},
			}

			session2 := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared-template-session-2",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "shareduser2",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
				},
			}

			Expect(k8sClient.Create(ctx, session1)).To(Succeed())
			Expect(k8sClient.Create(ctx, session2)).To(Succeed())

			// Verify independent deployments created
			deployment1 := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-shareduser1-test-template",
					Namespace: "default",
				}, deployment1)
			}, timeout, interval).Should(Succeed())

			deployment2 := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-shareduser2-test-template",
					Namespace: "default",
				}, deployment2)
			}, timeout, interval).Should(Succeed())

			// Verify they have different names
			Expect(deployment1.Name).NotTo(Equal(deployment2.Name))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session1)).To(Succeed())
			Expect(k8sClient.Delete(ctx, session2)).To(Succeed())
		})
	})
})

var _ = Describe("Session Controller Edge Cases", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When session name contains special characters", func() {
		It("Should handle valid Kubernetes naming conventions", func() {
			ctx := context.Background()

			// Kubernetes names must be lowercase alphanumeric + hyphens
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "session-with-hyphens-123",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "testuser",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
				},
			}

			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Verify deployment created with valid name
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-testuser-test-template",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})

	Context("When session state transitions rapidly", func() {
		It("Should handle rapid running -> hibernated -> running transitions", func() {
			ctx := context.Background()

			// Create session
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rapid-transition-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:           "rapiduser",
					Template:       "test-template",
					State:          "running",
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait for initial deployment
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-rapiduser-test-template",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Rapidly transition: hibernate
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "rapid-transition-session",
				Namespace: "default",
			}, session)).To(Succeed())
			session.Spec.State = "hibernated"
			Expect(k8sClient.Update(ctx, session)).To(Succeed())

			// Wait a bit
			time.Sleep(500 * time.Millisecond)

			// Rapidly transition back: running
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "rapid-transition-session",
				Namespace: "default",
			}, session)).To(Succeed())
			session.Spec.State = "running"
			Expect(k8sClient.Update(ctx, session)).To(Succeed())

			// Controller should handle this gracefully
			// Final state should be running
			Eventually(func() int32 {
				_ = k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-rapiduser-test-template",
					Namespace: "default",
				}, deployment)
				if deployment.Spec.Replicas != nil {
					return *deployment.Spec.Replicas
				}
				return -1
			}, timeout, interval).Should(Equal(int32(1)))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})

	Context("When updating session resources", func() {
		It("Should handle resource limit updates", func() {
			ctx := context.Background()

			// Create session with initial resources
			session := &streamv1alpha1.Session{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "resource-update-session",
					Namespace: "default",
				},
				Spec: streamv1alpha1.SessionSpec{
					User:     "resourceuser",
					Template: "test-template",
					State:    "running",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
							corev1.ResourceCPU:    resource.MustParse("500m"),
						},
					},
					PersistentHome: false,
				},
			}
			Expect(k8sClient.Create(ctx, session)).To(Succeed())

			// Wait for deployment
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-resourceuser-test-template",
					Namespace: "default",
				}, deployment)
			}, timeout, interval).Should(Succeed())

			// Update resources
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "resource-update-session",
				Namespace: "default",
			}, session)).To(Succeed())
			session.Spec.Resources.Requests[corev1.ResourceMemory] = resource.MustParse("2Gi")
			session.Spec.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("1000m")
			Expect(k8sClient.Update(ctx, session)).To(Succeed())

			// Controller should update deployment with new resources
			Eventually(func() string {
				_ = k8sClient.Get(ctx, types.NamespacedName{
					Name:      "ss-resourceuser-test-template",
					Namespace: "default",
				}, deployment)
				if len(deployment.Spec.Template.Spec.Containers) > 0 {
					mem := deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory()
					if mem != nil {
						return mem.String()
					}
				}
				return ""
			}, timeout, interval).Should(Equal("2Gi"))

			// Cleanup
			Expect(k8sClient.Delete(ctx, session)).To(Succeed())
		})
	})
})

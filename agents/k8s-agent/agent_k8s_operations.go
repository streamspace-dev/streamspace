package main

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// createSessionDeployment creates a Kubernetes Deployment for a session.
//
// The deployment is created based on the session spec and template.
// It includes resource limits, environment variables, and volume mounts.
func createSessionDeployment(client *kubernetes.Clientset, namespace string, spec *SessionSpec) (*appsv1.Deployment, error) {
	// Parse resource requirements
	memoryLimit, err := resource.ParseQuantity(spec.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory value: %w", err)
	}

	cpuLimit, err := resource.ParseQuantity(spec.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU value: %w", err)
	}

	replicas := int32(1)

	// Create deployment manifest
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.SessionID,
			Namespace: namespace,
			Labels: map[string]string{
				"app":      "streamspace-session",
				"session":  spec.SessionID,
				"user":     spec.User,
				"template": spec.Template,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"session": spec.SessionID,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":      "streamspace-session",
						"session":  spec.SessionID,
						"user":     spec.User,
						"template": spec.Template,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "session",
							Image: getTemplateImage(spec.Template), // TODO: Fetch from template
							Ports: []corev1.ContainerPort{
								{
									Name:          "vnc",
									ContainerPort: 3000,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "USER",
									Value: spec.User,
								},
								{
									Name:  "SESSION_ID",
									Value: spec.SessionID,
								},
								{
									Name:  "PUID",
									Value: "1000",
								},
								{
									Name:  "PGID",
									Value: "1000",
								},
								{
									Name:  "TZ",
									Value: "UTC",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: memoryLimit,
									corev1.ResourceCPU:    cpuLimit,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: memoryLimit,
									corev1.ResourceCPU:    cpuLimit,
								},
							},
						},
					},
				},
			},
		},
	}

	// Add persistent volume if requested
	if spec.PersistentHome {
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "user-home",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: spec.SessionID + "-home",
					},
				},
			},
		}

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "user-home",
				MountPath: "/config",
			},
		}
	}

	// Create deployment
	ctx := context.Background()
	created, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Printf("[K8sOps] Created deployment: %s", created.Name)
	return created, nil
}

// createSessionService creates a Kubernetes Service for a session.
//
// The service exposes the VNC port (3000) as ClusterIP.
func createSessionService(client *kubernetes.Clientset, namespace string, spec *SessionSpec) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.SessionID,
			Namespace: namespace,
			Labels: map[string]string{
				"app":      "streamspace-session",
				"session":  spec.SessionID,
				"user":     spec.User,
				"template": spec.Template,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"session": spec.SessionID,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "vnc",
					Port:       3000,
					TargetPort: intstr.FromInt(3000),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// Create service
	ctx := context.Background()
	created, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Printf("[K8sOps] Created service: %s", created.Name)
	return created, nil
}

// createSessionPVC creates a PersistentVolumeClaim for persistent user home.
//
// The PVC is created with ReadWriteOnce access mode and 10Gi storage.
func createSessionPVC(client *kubernetes.Clientset, namespace string, spec *SessionSpec) (*corev1.PersistentVolumeClaim, error) {
	storageClass := "standard" // TODO: Make configurable
	storage, _ := resource.ParseQuantity("10Gi")

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.SessionID + "-home",
			Namespace: namespace,
			Labels: map[string]string{
				"app":      "streamspace-session",
				"session":  spec.SessionID,
				"user":     spec.User,
				"template": spec.Template,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: &storageClass,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storage,
				},
			},
		},
	}

	// Create PVC
	ctx := context.Background()
	created, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	log.Printf("[K8sOps] Created PVC: %s", created.Name)
	return created, nil
}

// waitForPodReady waits for a pod to reach Running state.
//
// It polls the pod status every 2 seconds until the pod is ready or timeout occurs.
func waitForPodReady(client *kubernetes.Clientset, namespace, sessionID string, timeoutSeconds int) (string, error) {
	ctx := context.Background()
	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	labelSelector := fmt.Sprintf("session=%s", sessionID)

	for {
		select {
		case <-timeout:
			return "", fmt.Errorf("timeout waiting for pod to be ready")

		case <-ticker.C:
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return "", err
			}

			if len(pods.Items) == 0 {
				continue
			}

			pod := pods.Items[0]

			// Check if pod is running and ready
			if pod.Status.Phase == corev1.PodRunning {
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
						log.Printf("[K8sOps] Pod ready: %s (IP: %s)", pod.Name, pod.Status.PodIP)
						return pod.Status.PodIP, nil
					}
				}
			}
		}
	}
}

// scaleDeployment scales a deployment to the specified number of replicas.
func scaleDeployment(client *kubernetes.Clientset, namespace, sessionID string, replicas int32) error {
	ctx := context.Background()

	// Get the deployment
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, sessionID, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Update replicas
	deployment.Spec.Replicas = &replicas

	// Update deployment
	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	log.Printf("[K8sOps] Scaled deployment %s to %d replicas", sessionID, replicas)
	return nil
}

// deleteDeployment deletes a deployment.
func deleteDeployment(client *kubernetes.Clientset, namespace, sessionID string) error {
	ctx := context.Background()
	deletePolicy := metav1.DeletePropagationForeground

	err := client.AppsV1().Deployments(namespace).Delete(ctx, sessionID, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return err
	}

	log.Printf("[K8sOps] Deleted deployment: %s", sessionID)
	return nil
}

// deleteService deletes a service.
func deleteService(client *kubernetes.Clientset, namespace, sessionID string) error {
	ctx := context.Background()

	err := client.CoreV1().Services(namespace).Delete(ctx, sessionID, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	log.Printf("[K8sOps] Deleted service: %s", sessionID)
	return nil
}

// deletePVC deletes a PersistentVolumeClaim.
func deletePVC(client *kubernetes.Clientset, namespace, sessionID string) error {
	ctx := context.Background()
	pvcName := sessionID + "-home"

	err := client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	log.Printf("[K8sOps] Deleted PVC: %s", pvcName)
	return nil
}

// getTemplateImage returns the container image for a template.
//
// TODO: This should fetch the template from the Control Plane API
// and return the actual image. For now, we use a hardcoded mapping.
func getTemplateImage(templateName string) string {
	// Default template images (from LinuxServer.io)
	templates := map[string]string{
		"firefox":     "lscr.io/linuxserver/firefox:latest",
		"chrome":      "lscr.io/linuxserver/chromium:latest",
		"vscode":      "lscr.io/linuxserver/code-server:latest",
		"ubuntu":      "lscr.io/linuxserver/webtop:ubuntu-mate",
		"kali":        "lscr.io/linuxserver/kali-linux:latest",
		"libreoffice": "lscr.io/linuxserver/libreoffice:latest",
	}

	if image, ok := templates[templateName]; ok {
		return image
	}

	// Default to Firefox if template not found
	return "lscr.io/linuxserver/firefox:latest"
}

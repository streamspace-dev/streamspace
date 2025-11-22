package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// GVR definitions for StreamSpace CRDs
var (
	templateGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "templates",
	}

	sessionGVR = schema.GroupVersionResource{
		Group:    "stream.space",
		Version:  "v1alpha1",
		Resource: "sessions",
	}
)

// Template represents a StreamSpace Template CRD
type Template struct {
	Name         string
	Namespace    string
	DisplayName  string
	Description  string
	BaseImage    string
	AppType      string // desktop, webapp
	DefaultResources struct {
		Memory string
		CPU    string
	}
	Ports []struct {
		Name          string
		ContainerPort int32
		Protocol      string
	}
	Env          []corev1.EnvVar
	VolumeMounts []corev1.VolumeMount
	VNC          *VNCConfig
}

// VNCConfig represents VNC configuration for desktop apps
type VNCConfig struct {
	Enabled  bool
	Port     int32
	Protocol string
}

// parseTemplateFromPayload parses template manifest from command payload.
//
// v2.0-beta: API sends full template manifest (from database) in command payload,
// eliminating need for agent to fetch Template CRD from Kubernetes.
// This allows API to run outside K8s cluster.
func parseTemplateFromPayload(payload map[string]interface{}, namespace string) (*Template, error) {
	// Get templateManifest from payload
	manifestInterface, ok := payload["templateManifest"]
	if !ok {
		return nil, fmt.Errorf("templateManifest not found in payload")
	}

	// Convert to map[string]interface{} (unstructured format)
	var manifestMap map[string]interface{}
	switch v := manifestInterface.(type) {
	case map[string]interface{}:
		manifestMap = v
	case []byte:
		// If it's JSON bytes, unmarshal it
		if err := json.Unmarshal(v, &manifestMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal templateManifest bytes: %w", err)
		}
	default:
		return nil, fmt.Errorf("templateManifest has invalid type: %T", manifestInterface)
	}

	// Create unstructured object
	obj := &unstructured.Unstructured{Object: manifestMap}

	// Use existing parseTemplateCRD to convert to Template struct
	template, err := parseTemplateCRD(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template manifest: %w", err)
	}

	// Override namespace if not set
	if template.Namespace == "" {
		template.Namespace = namespace
	}

	log.Printf("[K8sOps] Parsed template from payload: %s (image: %s, ports: %d)", template.Name, template.BaseImage, len(template.Ports))
	return template, nil
}

// fetchTemplateCRD fetches a Template CRD from Kubernetes.
//
// v2.0-beta: This is now a FALLBACK for backwards compatibility.
// Normally, template manifest is sent in command payload.
func fetchTemplateCRD(dynamicClient dynamic.Interface, namespace, templateName string) (*Template, error) {
	ctx := context.Background()

	// Fetch the Template CRD
	obj, err := dynamicClient.Resource(templateGVR).Namespace(namespace).Get(ctx, templateName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get template %s: %w", templateName, err)
	}

	// Parse the unstructured object into Template struct
	template, err := parseTemplateCRD(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	log.Printf("[K8sOps] Fetched template from K8s: %s (image: %s, ports: %d)", template.Name, template.BaseImage, len(template.Ports))
	return template, nil
}

// parseTemplateCRD parses an unstructured Template CRD into a Template struct.
func parseTemplateCRD(obj *unstructured.Unstructured) (*Template, error) {
	template := &Template{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}

	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid template spec")
	}

	// Parse basic fields
	if displayName, ok := spec["displayName"].(string); ok {
		template.DisplayName = displayName
	}

	if description, ok := spec["description"].(string); ok {
		template.Description = description
	}

	if baseImage, ok := spec["baseImage"].(string); ok {
		template.BaseImage = baseImage
	} else {
		return nil, fmt.Errorf("template missing baseImage")
	}

	if appType, ok := spec["appType"].(string); ok {
		template.AppType = appType
	}

	// Parse default resources
	if resources, ok := spec["defaultResources"].(map[string]interface{}); ok {
		if memory, ok := resources["memory"].(string); ok {
			template.DefaultResources.Memory = memory
		}
		if cpu, ok := resources["cpu"].(string); ok {
			template.DefaultResources.CPU = cpu
		}
	}

	// Parse ports
	if ports, ok := spec["ports"].([]interface{}); ok {
		template.Ports = make([]struct {
			Name          string
			ContainerPort int32
			Protocol      string
		}, 0, len(ports))

		for _, portInterface := range ports {
			portMap, ok := portInterface.(map[string]interface{})
			if !ok {
				continue
			}

			port := struct {
				Name          string
				ContainerPort int32
				Protocol      string
			}{}

			if name, ok := portMap["name"].(string); ok {
				port.Name = name
			}

			if containerPort, ok := portMap["containerPort"].(float64); ok {
				port.ContainerPort = int32(containerPort)
			}

			if protocol, ok := portMap["protocol"].(string); ok {
				port.Protocol = protocol
			} else {
				port.Protocol = "TCP"
			}

			template.Ports = append(template.Ports, port)
		}
	}

	// Parse environment variables
	if env, ok := spec["env"].([]interface{}); ok {
		template.Env = make([]corev1.EnvVar, 0, len(env))

		for _, envInterface := range env {
			envMap, ok := envInterface.(map[string]interface{})
			if !ok {
				continue
			}

			envVar := corev1.EnvVar{}
			if name, ok := envMap["name"].(string); ok {
				envVar.Name = name
			}
			if value, ok := envMap["value"].(string); ok {
				envVar.Value = value
			}

			template.Env = append(template.Env, envVar)
		}
	}

	// Parse VNC configuration
	if vnc, ok := spec["vnc"].(map[string]interface{}); ok {
		vncConfig := &VNCConfig{}

		if enabled, ok := vnc["enabled"].(bool); ok {
			vncConfig.Enabled = enabled
		}

		if port, ok := vnc["port"].(float64); ok {
			vncConfig.Port = int32(port)
		}

		if protocol, ok := vnc["protocol"].(string); ok {
			vncConfig.Protocol = protocol
		}

		template.VNC = vncConfig
	}

	return template, nil
}

// createSessionCRD creates a Session Custom Resource in Kubernetes.
//
// This creates the Session CRD after Deployment/Service/PVC are created,
// establishing the session record in Kubernetes.
func createSessionCRD(dynamicClient dynamic.Interface, namespace string, spec *SessionSpec, podName, podIP string) error {
	ctx := context.Background()

	// Determine VNC port (default 3000)
	vncPort := int32(3000)

	// Build Session CRD object
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "stream.space/v1alpha1",
			"kind":       "Session",
			"metadata": map[string]interface{}{
				"name":      spec.SessionID,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app":      "streamspace-session",
					"session":  spec.SessionID,
					"user":     spec.User,
					"template": spec.Template,
				},
			},
			"spec": map[string]interface{}{
				"user":           spec.User,
				"template":       spec.Template,
				"state":          "running",
				"persistentHome": spec.PersistentHome,
			},
		},
	}

	// Add optional spec fields
	sessionSpec := obj.Object["spec"].(map[string]interface{})

	if spec.Memory != "" || spec.CPU != "" {
		resources := make(map[string]interface{})
		if spec.Memory != "" {
			resources["memory"] = spec.Memory
		}
		if spec.CPU != "" {
			resources["cpu"] = spec.CPU
		}
		sessionSpec["resources"] = resources
	}

	// Add status subresource
	obj.Object["status"] = map[string]interface{}{
		"phase":   "Running",
		"podName": podName,
		"url":     fmt.Sprintf("http://%s:%d", podIP, vncPort),
	}

	// Create the Session CRD
	_, err := dynamicClient.Resource(sessionGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Session CRD: %w", err)
	}

	log.Printf("[K8sOps] Created Session CRD: %s (pod: %s, url: http://%s:%d)", spec.SessionID, podName, podIP, vncPort)
	return nil
}

// createSessionDeployment creates a Kubernetes Deployment for a session.
//
// The deployment is created based on the session spec and template.
// It includes resource limits, environment variables, and volume mounts.
func createSessionDeployment(client *kubernetes.Clientset, namespace string, spec *SessionSpec, template *Template) (*appsv1.Deployment, error) {
	// Parse resource requirements (use session spec or template defaults)
	memory := spec.Memory
	if memory == "" && template.DefaultResources.Memory != "" {
		memory = template.DefaultResources.Memory
	}
	if memory == "" {
		memory = "2Gi" // Fallback default
	}

	cpu := spec.CPU
	if cpu == "" && template.DefaultResources.CPU != "" {
		cpu = template.DefaultResources.CPU
	}
	if cpu == "" {
		cpu = "1000m" // Fallback default
	}

	memoryLimit, err := resource.ParseQuantity(memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory value: %w", err)
	}

	cpuLimit, err := resource.ParseQuantity(cpu)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU value: %w", err)
	}

	replicas := int32(1)

	// Build container ports from template
	containerPorts := make([]corev1.ContainerPort, 0)
	if len(template.Ports) > 0 {
		for _, port := range template.Ports {
			protocol := corev1.ProtocolTCP
			if port.Protocol == "UDP" {
				protocol = corev1.ProtocolUDP
			}
			containerPorts = append(containerPorts, corev1.ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      protocol,
			})
		}
	} else if template.VNC != nil && template.VNC.Enabled {
		// Fallback: Use VNC port if no ports defined
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "vnc",
			ContainerPort: template.VNC.Port,
			Protocol:      corev1.ProtocolTCP,
		})
	} else {
		// Fallback: Default VNC port
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          "vnc",
			ContainerPort: 3000,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Build environment variables (merge template env + session-specific env)
	envVars := make([]corev1.EnvVar, 0)

	// Add template-defined env vars first
	envVars = append(envVars, template.Env...)

	// Add session-specific env vars (these override template env if name conflicts)
	sessionEnv := []corev1.EnvVar{
		{Name: "USER", Value: spec.User},
		{Name: "SESSION_ID", Value: spec.SessionID},
		{Name: "PUID", Value: "1000"},
		{Name: "PGID", Value: "1000"},
		{Name: "TZ", Value: "UTC"},
	}

	// Merge env vars (session env overrides template env)
	envMap := make(map[string]string)
	for _, env := range envVars {
		envMap[env.Name] = env.Value
	}
	for _, env := range sessionEnv {
		envMap[env.Name] = env.Value
	}

	finalEnv := make([]corev1.EnvVar, 0, len(envMap))
	for k, v := range envMap {
		finalEnv = append(finalEnv, corev1.EnvVar{Name: k, Value: v})
	}

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
							Name:      "session",
							Image:     template.BaseImage,
							Ports:     containerPorts,
							Env:       finalEnv,
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
func waitForPodReady(client *kubernetes.Clientset, namespace, sessionID string, timeoutSeconds int) (podName string, podIP string, err error) {
	ctx := context.Background()
	timeout := time.After(time.Duration(timeoutSeconds) * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	labelSelector := fmt.Sprintf("session=%s", sessionID)

	for {
		select {
		case <-timeout:
			return "", "", fmt.Errorf("timeout waiting for pod to be ready")

		case <-ticker.C:
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				return "", "", err
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
						return pod.Name, pod.Status.PodIP, nil
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

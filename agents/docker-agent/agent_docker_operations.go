package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// Template represents a StreamSpace template parsed from payload.
type Template struct {
	Name         string
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
		ContainerPort int
		Protocol      string
	}
	Env          []string
	VolumeMounts []VolumeMount
	VNC          *VNCConfig
}

// VolumeMount represents a volume mount configuration.
type VolumeMount struct {
	Name      string
	MountPath string
}

// VNCConfig represents VNC configuration for desktop apps.
type VNCConfig struct {
	Enabled  bool
	Port     int
	Protocol string
}

// parseTemplateFromPayload parses template manifest from command payload.
//
// v2.0-beta: API sends full template manifest (from database) in command payload,
// eliminating need for agent to fetch templates from external sources.
func parseTemplateFromPayload(payload map[string]interface{}) (*Template, error) {
	// Get templateManifest from payload
	manifestInterface, ok := payload["templateManifest"]
	if !ok {
		return nil, fmt.Errorf("templateManifest not found in payload")
	}

	// Convert to map[string]interface{}
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

	// Parse the template manifest
	return parseTemplateManifest(manifestMap)
}

// parseTemplateManifest parses a template manifest map into a Template struct.
func parseTemplateManifest(manifestMap map[string]interface{}) (*Template, error) {
	// Get spec from manifest
	spec, ok := manifestMap["spec"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid template spec")
	}

	// Get metadata
	metadata, ok := manifestMap["metadata"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid template metadata")
	}

	template := &Template{
		Name:        getString(metadata, "name"),
		DisplayName: getString(spec, "displayName"),
		Description: getString(spec, "description"),
		BaseImage:   getString(spec, "baseImage"),
		AppType:     getString(spec, "appType"),
	}

	// Parse default resources
	if resources, ok := spec["defaultResources"].(map[string]interface{}); ok {
		template.DefaultResources.Memory = getString(resources, "memory")
		template.DefaultResources.CPU = getString(resources, "cpu")
	}

	// Parse ports
	if ports, ok := spec["ports"].([]interface{}); ok {
		for _, p := range ports {
			if portMap, ok := p.(map[string]interface{}); ok {
				template.Ports = append(template.Ports, struct {
					Name          string
					ContainerPort int
					Protocol      string
				}{
					Name:          getString(portMap, "name"),
					ContainerPort: getInt(portMap, "containerPort"),
					Protocol:      getString(portMap, "protocol"),
				})
			}
		}
	}

	// Parse environment variables
	if env, ok := spec["env"].([]interface{}); ok {
		for _, e := range env {
			if envMap, ok := e.(map[string]interface{}); ok {
				name := getString(envMap, "name")
				value := getString(envMap, "value")
				if name != "" {
					template.Env = append(template.Env, fmt.Sprintf("%s=%s", name, value))
				}
			}
		}
	}

	// Parse volume mounts
	if mounts, ok := spec["volumeMounts"].([]interface{}); ok {
		for _, m := range mounts {
			if mountMap, ok := m.(map[string]interface{}); ok {
				template.VolumeMounts = append(template.VolumeMounts, VolumeMount{
					Name:      getString(mountMap, "name"),
					MountPath: getString(mountMap, "mountPath"),
				})
			}
		}
	}

	// Parse VNC config
	if vnc, ok := spec["vnc"].(map[string]interface{}); ok {
		template.VNC = &VNCConfig{
			Enabled:  getBool(vnc, "enabled"),
			Port:     getInt(vnc, "port"),
			Protocol: getString(vnc, "protocol"),
		}
	}

	return template, nil
}

// Helper functions for safe type extraction
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	if v, ok := m[key].(int); ok {
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// ensureNetwork ensures the StreamSpace network exists.
func (a *DockerAgent) ensureNetwork(ctx context.Context) error {
	// Check if network exists
	networks, err := a.dockerClient.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, net := range networks {
		if net.Name == a.config.NetworkName {
			log.Printf("[Docker] Network %s already exists", a.config.NetworkName)
			return nil
		}
	}

	// Create network
	log.Printf("[Docker] Creating network: %s", a.config.NetworkName)
	_, err = a.dockerClient.NetworkCreate(ctx, a.config.NetworkName, types.NetworkCreate{
		Driver: "bridge",
		Labels: map[string]string{
			"app":       "streamspace",
			"component": "session-network",
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	log.Printf("[Docker] Network %s created successfully", a.config.NetworkName)
	return nil
}

// createSessionContainer creates a Docker container for a session.
func (a *DockerAgent) createSessionContainer(ctx context.Context, sessionID string, template *Template, resources map[string]string, persistentHome bool) (string, error) {
	// Pull image if needed
	if err := a.pullImage(ctx, template.BaseImage); err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	// Prepare container configuration
	config := &container.Config{
		Image: template.BaseImage,
		Env:   template.Env,
		Labels: map[string]string{
			"app":        "streamspace",
			"component":  "session",
			"session-id": sessionID,
		},
	}

	// Add exposed ports
	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, port := range template.Ports {
		natPort := nat.Port(fmt.Sprintf("%d/%s", port.ContainerPort, strings.ToLower(port.Protocol)))
		exposedPorts[natPort] = struct{}{}
		// Map to random host port
		portBindings[natPort] = []nat.PortBinding{{HostIP: "0.0.0.0"}}
	}
	config.ExposedPorts = exposedPorts

	// Prepare host configuration
	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Set resource limits
	if memory, ok := resources["memory"]; ok && memory != "" {
		hostConfig.Resources.Memory = parseMemory(memory)
	} else if template.DefaultResources.Memory != "" {
		hostConfig.Resources.Memory = parseMemory(template.DefaultResources.Memory)
	}

	if cpu, ok := resources["cpu"]; ok && cpu != "" {
		hostConfig.Resources.NanoCPUs = parseCPU(cpu)
	} else if template.DefaultResources.CPU != "" {
		hostConfig.Resources.NanoCPUs = parseCPU(template.DefaultResources.CPU)
	}

	// Add volume mounts
	mounts := []mount.Mount{}
	if persistentHome {
		// Create persistent volume for home directory
		volumeName := fmt.Sprintf("streamspace-%s-home", sessionID)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: "/home/streamspace",
		})
	}

	// Add template-defined mounts
	for _, vm := range template.VolumeMounts {
		volumeName := fmt.Sprintf("streamspace-%s-%s", sessionID, vm.Name)
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: vm.MountPath,
		})
	}
	hostConfig.Mounts = mounts

	// Network configuration
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			a.config.NetworkName: {},
		},
	}

	// Create container
	containerName := fmt.Sprintf("streamspace-%s", sessionID)
	log.Printf("[Docker] Creating container: %s (image: %s)", containerName, template.BaseImage)

	resp, err := a.dockerClient.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	log.Printf("[Docker] Container created: %s (ID: %s)", containerName, resp.ID[:12])
	return resp.ID, nil
}

// pullImage pulls a Docker image if not already present.
func (a *DockerAgent) pullImage(ctx context.Context, image string) error {
	// Check if image exists locally
	_, _, err := a.dockerClient.ImageInspectWithRaw(ctx, image)
	if err == nil {
		log.Printf("[Docker] Image %s already exists locally", image)
		return nil
	}

	// Pull image
	log.Printf("[Docker] Pulling image: %s", image)
	reader, err := a.dockerClient.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	defer reader.Close()

	// Wait for pull to complete
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read pull response: %w", err)
	}

	log.Printf("[Docker] Image %s pulled successfully", image)
	return nil
}

// startContainer starts a Docker container.
func (a *DockerAgent) startContainer(ctx context.Context, containerID string) error {
	log.Printf("[Docker] Starting container: %s", containerID[:12])

	if err := a.dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	log.Printf("[Docker] Container started: %s", containerID[:12])
	return nil
}

// waitForContainerRunning waits for a container to be running.
func (a *DockerAgent) waitForContainerRunning(ctx context.Context, containerID string, timeout time.Duration) error {
	log.Printf("[Docker] Waiting for container to be running: %s", containerID[:12])

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inspect, err := a.dockerClient.ContainerInspect(ctx, containerID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if inspect.State.Running {
			log.Printf("[Docker] Container is running: %s", containerID[:12])
			return nil
		}

		if inspect.State.Status == "exited" || inspect.State.Status == "dead" {
			return fmt.Errorf("container exited unexpectedly (status: %s, exit code: %d)",
				inspect.State.Status, inspect.State.ExitCode)
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for container to be running")
}

// stopContainer stops a Docker container.
func (a *DockerAgent) stopContainer(ctx context.Context, containerID string) error {
	log.Printf("[Docker] Stopping container: %s", containerID[:12])

	timeout := 10 // seconds
	if err := a.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	log.Printf("[Docker] Container stopped: %s", containerID[:12])
	return nil
}

// removeContainer removes a Docker container.
func (a *DockerAgent) removeContainer(ctx context.Context, containerID string) error {
	log.Printf("[Docker] Removing container: %s", containerID[:12])

	if err := a.dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: false, // Keep volumes for now
	}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	log.Printf("[Docker] Container removed: %s", containerID[:12])
	return nil
}

// getContainerBySession finds a container by session ID.
func (a *DockerAgent) getContainerBySession(ctx context.Context, sessionID string) (string, error) {
	containers, err := a.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if sessionLabel, ok := container.Labels["session-id"]; ok && sessionLabel == sessionID {
			return container.ID, nil
		}
	}

	return "", fmt.Errorf("container not found for session: %s", sessionID)
}

// parseMemory converts memory string (e.g., "2Gi", "512Mi") to bytes.
func parseMemory(memory string) int64 {
	memory = strings.TrimSpace(memory)
	if memory == "" {
		return 0
	}

	// Parse Gi, Mi, G, M suffixes
	if strings.HasSuffix(memory, "Gi") {
		val := strings.TrimSuffix(memory, "Gi")
		if num, err := parseFloat(val); err == nil {
			return int64(num * 1024 * 1024 * 1024)
		}
	}
	if strings.HasSuffix(memory, "Mi") {
		val := strings.TrimSuffix(memory, "Mi")
		if num, err := parseFloat(val); err == nil {
			return int64(num * 1024 * 1024)
		}
	}
	if strings.HasSuffix(memory, "G") {
		val := strings.TrimSuffix(memory, "G")
		if num, err := parseFloat(val); err == nil {
			return int64(num * 1000 * 1000 * 1000)
		}
	}
	if strings.HasSuffix(memory, "M") {
		val := strings.TrimSuffix(memory, "M")
		if num, err := parseFloat(val); err == nil {
			return int64(num * 1000 * 1000)
		}
	}

	return 0
}

// parseCPU converts CPU string (e.g., "1000m", "2") to nano CPUs.
func parseCPU(cpu string) int64 {
	cpu = strings.TrimSpace(cpu)
	if cpu == "" {
		return 0
	}

	// Parse millicores (e.g., "1000m" = 1 CPU)
	if strings.HasSuffix(cpu, "m") {
		val := strings.TrimSuffix(cpu, "m")
		if num, err := parseFloat(val); err == nil {
			// 1000m = 1 CPU = 1e9 nano CPUs
			return int64(num * 1000000)
		}
	}

	// Parse cores (e.g., "2" = 2 CPUs)
	if num, err := parseFloat(cpu); err == nil {
		return int64(num * 1000000000)
	}

	return 0
}

// parseFloat parses a float from string.
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

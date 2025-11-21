package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// VNCTunnelManager manages VNC tunnels for sessions.
//
// Each VNC tunnel consists of:
//   - Port-forward from agent to pod's VNC port (5900 or 3000)
//   - Data relay between port-forward and WebSocket
//   - Connection lifecycle management
//
// Multiple tunnels can run concurrently, one per session.
type VNCTunnelManager struct {
	// kubeClient is the Kubernetes API client
	kubeClient *kubernetes.Clientset

	// config is the REST config for port-forward
	restConfig *rest.Config

	// namespace is the Kubernetes namespace for sessions
	namespace string

	// tunnels maps sessionID -> active tunnel
	tunnels map[string]*VNCTunnel
	mutex   sync.RWMutex

	// agent is the parent K8s agent (for sending VNC messages)
	agent *K8sAgent
}

// VNCTunnel represents a single VNC tunnel to a pod.
//
// The tunnel consists of a Kubernetes port-forward and data relay.
type VNCTunnel struct {
	// sessionID identifies the session
	sessionID string

	// podName is the name of the pod
	podName string

	// vncPort is the pod's VNC port (5900 or 3000)
	vncPort int

	// localPort is the locally forwarded port
	localPort int

	// conn is the local connection to the forwarded port
	conn net.Conn

	// stopChan signals the tunnel to stop
	stopChan chan struct{}

	// readyChan signals when the tunnel is ready
	readyChan chan bool

	// portForwarder is the Kubernetes port-forward
	portForwarder *portforward.PortForwarder
}

// NewVNCTunnelManager creates a new VNC tunnel manager.
func NewVNCTunnelManager(kubeClient *kubernetes.Clientset, restConfig *rest.Config, namespace string, agent *K8sAgent) *VNCTunnelManager {
	return &VNCTunnelManager{
		kubeClient: kubeClient,
		restConfig: restConfig,
		namespace:  namespace,
		tunnels:    make(map[string]*VNCTunnel),
		agent:      agent,
	}
}

// CreateTunnel creates a VNC tunnel to a session's pod.
//
// Steps:
//  1. Find the pod for the session
//  2. Create port-forward to pod's VNC port
//  3. Wait for port-forward to be ready
//  4. Connect to local forwarded port
//  5. Start data relay goroutine
//  6. Notify Control Plane that VNC is ready
func (m *VNCTunnelManager) CreateTunnel(sessionID string) error {
	log.Printf("[VNCTunnel] Creating tunnel for session: %s", sessionID)

	// Check if tunnel already exists
	m.mutex.Lock()
	if _, exists := m.tunnels[sessionID]; exists {
		m.mutex.Unlock()
		return fmt.Errorf("tunnel already exists for session %s", sessionID)
	}
	m.mutex.Unlock()

	// Find the pod for this session
	podName, vncPort, err := m.findSessionPod(sessionID)
	if err != nil {
		return fmt.Errorf("failed to find pod: %w", err)
	}

	log.Printf("[VNCTunnel] Found pod %s with VNC port %d", podName, vncPort)

	// Create tunnel
	tunnel := &VNCTunnel{
		sessionID: sessionID,
		podName:   podName,
		vncPort:   vncPort,
		stopChan:  make(chan struct{}),
		readyChan: make(chan bool, 1),
	}

	// Start port-forward
	if err := m.startPortForward(tunnel); err != nil {
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	// Wait for port-forward to be ready (with timeout)
	select {
	case <-tunnel.readyChan:
		log.Printf("[VNCTunnel] Port-forward ready for session %s", sessionID)
	case <-time.After(30 * time.Second):
		close(tunnel.stopChan)
		return fmt.Errorf("timeout waiting for port-forward")
	}

	// Connect to local forwarded port
	if err := m.connectToForwardedPort(tunnel); err != nil {
		close(tunnel.stopChan)
		return fmt.Errorf("failed to connect to forwarded port: %w", err)
	}

	// Store tunnel
	m.mutex.Lock()
	m.tunnels[sessionID] = tunnel
	m.mutex.Unlock()

	// Start data relay
	go m.relayData(tunnel)

	// Notify Control Plane that VNC is ready
	m.agent.sendVNCReady(sessionID, vncPort, podName)

	log.Printf("[VNCTunnel] Tunnel created successfully for session %s (local port: %d)", sessionID, tunnel.localPort)
	return nil
}

// CloseTunnel closes a VNC tunnel.
func (m *VNCTunnelManager) CloseTunnel(sessionID string) error {
	m.mutex.Lock()
	tunnel, exists := m.tunnels[sessionID]
	if !exists {
		m.mutex.Unlock()
		return fmt.Errorf("tunnel not found for session %s", sessionID)
	}
	delete(m.tunnels, sessionID)
	m.mutex.Unlock()

	log.Printf("[VNCTunnel] Closing tunnel for session %s", sessionID)

	// Stop port-forward
	close(tunnel.stopChan)

	// Close connection
	if tunnel.conn != nil {
		tunnel.conn.Close()
	}

	log.Printf("[VNCTunnel] Tunnel closed for session %s", sessionID)
	return nil
}

// SendData sends VNC data from Control Plane to pod.
//
// The data is base64-decoded and written to the port-forward connection.
func (m *VNCTunnelManager) SendData(sessionID string, base64Data string) error {
	m.mutex.RLock()
	tunnel, exists := m.tunnels[sessionID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("tunnel not found for session %s", sessionID)
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return fmt.Errorf("failed to decode base64: %w", err)
	}

	// Write to connection
	if tunnel.conn == nil {
		return fmt.Errorf("connection not established")
	}

	_, err = tunnel.conn.Write(data)
	if err != nil {
		log.Printf("[VNCTunnel] Write error for session %s: %v", sessionID, err)
		// Close tunnel on write error
		go m.CloseTunnel(sessionID)
		return err
	}

	return nil
}

// findSessionPod finds the pod name and VNC port for a session.
func (m *VNCTunnelManager) findSessionPod(sessionID string) (string, int, error) {
	ctx := context.Background()

	// List pods with session label
	pods, err := m.kubeClient.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("session=%s", sessionID),
	})
	if err != nil {
		return "", 0, err
	}

	if len(pods.Items) == 0 {
		return "", 0, fmt.Errorf("no pod found for session %s", sessionID)
	}

	pod := pods.Items[0]

	// Check if pod is running
	if pod.Status.Phase != corev1.PodRunning {
		return "", 0, fmt.Errorf("pod not running (phase: %s)", pod.Status.Phase)
	}

	// Find VNC port (usually 3000 for LinuxServer.io images, or 5900 for standard VNC)
	vncPort := 3000 // Default
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			if port.Name == "vnc" {
				vncPort = int(port.ContainerPort)
				break
			}
		}
	}

	return pod.Name, vncPort, nil
}

// startPortForward starts a Kubernetes port-forward to the pod.
func (m *VNCTunnelManager) startPortForward(tunnel *VNCTunnel) error {
	// Build URL for port-forward
	req := m.kubeClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(m.namespace).
		Name(tunnel.podName).
		SubResource("portforward")

	// Use a local ephemeral port (0 = auto-assign)
	ports := []string{fmt.Sprintf("0:%d", tunnel.vncPort)}

	// Create SPDY transport
	transport, upgrader, err := spdy.RoundTripperFor(m.restConfig)
	if err != nil {
		return err
	}

	// Create port-forwarder
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})

	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, outBuf, errBuf)
	if err != nil {
		return err
	}

	tunnel.portForwarder = pf

	// Start port-forward in goroutine
	go func() {
		if err := pf.ForwardPorts(); err != nil {
			log.Printf("[VNCTunnel] Port-forward error for %s: %v", tunnel.sessionID, err)
			log.Printf("[VNCTunnel] Stdout: %s", outBuf.String())
			log.Printf("[VNCTunnel] Stderr: %s", errBuf.String())
		}
	}()

	// Wait for ready signal
	go func() {
		<-readyChan

		// Get the actual local port
		forwardedPorts, err := pf.GetPorts()
		if err != nil || len(forwardedPorts) == 0 {
			log.Printf("[VNCTunnel] Failed to get forwarded ports: %v", err)
			tunnel.readyChan <- false
			return
		}

		tunnel.localPort = int(forwardedPorts[0].Local)
		log.Printf("[VNCTunnel] Port-forward established: localhost:%d -> %s:%d",
			tunnel.localPort, tunnel.podName, tunnel.vncPort)

		tunnel.readyChan <- true
	}()

	return nil
}

// connectToForwardedPort connects to the locally forwarded port.
func (m *VNCTunnelManager) connectToForwardedPort(tunnel *VNCTunnel) error {
	// Connect to localhost:localPort
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", tunnel.localPort))
	if err != nil {
		return err
	}

	tunnel.conn = conn
	log.Printf("[VNCTunnel] Connected to forwarded port %d", tunnel.localPort)
	return nil
}

// relayData relays data from pod to Control Plane.
//
// Reads from the port-forward connection and sends to Control Plane via WebSocket.
func (m *VNCTunnelManager) relayData(tunnel *VNCTunnel) {
	defer func() {
		log.Printf("[VNCTunnel] Data relay stopped for session %s", tunnel.sessionID)
		m.CloseTunnel(tunnel.sessionID)
	}()

	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		select {
		case <-tunnel.stopChan:
			return

		default:
			// Set read deadline to allow checking stopChan
			tunnel.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			n, err := tunnel.conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected, continue
					continue
				}
				if err != io.EOF {
					log.Printf("[VNCTunnel] Read error for session %s: %v", tunnel.sessionID, err)
					m.agent.sendVNCError(tunnel.sessionID, err.Error())
				}
				return
			}

			if n > 0 {
				// Base64-encode data for JSON transport
				base64Data := base64.StdEncoding.EncodeToString(buffer[:n])

				// Send to Control Plane
				if err := m.agent.sendVNCData(tunnel.sessionID, base64Data); err != nil {
					log.Printf("[VNCTunnel] Failed to send VNC data for session %s: %v", tunnel.sessionID, err)
					return
				}
			}
		}
	}
}

// CloseAll closes all active tunnels (for agent shutdown).
func (m *VNCTunnelManager) CloseAll() {
	m.mutex.Lock()
	sessionIDs := make([]string, 0, len(m.tunnels))
	for sessionID := range m.tunnels {
		sessionIDs = append(sessionIDs, sessionID)
	}
	m.mutex.Unlock()

	for _, sessionID := range sessionIDs {
		m.CloseTunnel(sessionID)
	}

	log.Println("[VNCTunnel] All tunnels closed")
}

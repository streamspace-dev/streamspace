package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// VNC message types (matching Control Plane protocol)
const (
	vncMessageTypeData  = "vnc_data"
	vncMessageTypeClose = "vnc_close"
	vncMessageTypeReady = "vnc_ready"
	vncMessageTypeError = "vnc_error"
)

// VNCDataMessage represents VNC data being tunneled.
type VNCDataMessage struct {
	SessionID string `json:"sessionId"`
	Data      string `json:"data"` // base64-encoded
}

// VNCCloseMessage represents a request to close a VNC tunnel.
type VNCCloseMessage struct {
	SessionID string `json:"sessionId"`
	Reason    string `json:"reason,omitempty"`
}

// VNCReadyMessage indicates a VNC tunnel is ready.
type VNCReadyMessage struct {
	SessionID string `json:"sessionId"`
	VNCPort   int    `json:"vncPort"`
	PodName   string `json:"podName,omitempty"`
}

// VNCErrorMessage reports a VNC tunnel error.
type VNCErrorMessage struct {
	SessionID string `json:"sessionId"`
	Error     string `json:"error"`
}

// handleVNCDataMessage processes incoming VNC data from Control Plane.
//
// The data is relayed to the pod via the VNC tunnel (port-forward).
func (a *K8sAgent) handleVNCDataMessage(payload json.RawMessage) error {
	var msg VNCDataMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("failed to parse VNC data message: %w", err)
	}

	if a.vncManager == nil {
		return fmt.Errorf("VNC manager not initialized")
	}

	// Send data to pod via tunnel
	if err := a.vncManager.SendData(msg.SessionID, msg.Data); err != nil {
		log.Printf("[VNCHandler] Failed to send data to pod for session %s: %v", msg.SessionID, err)
		return err
	}

	return nil
}

// handleVNCCloseMessage processes a VNC tunnel close request.
//
// This is sent when the client disconnects from the VNC session.
func (a *K8sAgent) handleVNCCloseMessage(payload json.RawMessage) error {
	var msg VNCCloseMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("failed to parse VNC close message: %w", err)
	}

	log.Printf("[VNCHandler] Closing VNC tunnel for session %s (reason: %s)", msg.SessionID, msg.Reason)

	if a.vncManager == nil {
		return fmt.Errorf("VNC manager not initialized")
	}

	// Close the tunnel
	if err := a.vncManager.CloseTunnel(msg.SessionID); err != nil {
		log.Printf("[VNCHandler] Failed to close tunnel: %v", err)
		return err
	}

	return nil
}

// sendVNCReady sends a VNC ready notification to Control Plane.
//
// This is called when the VNC tunnel is established and ready for connections.
func (a *K8sAgent) sendVNCReady(sessionID string, vncPort int, podName string) error {
	ready := map[string]interface{}{
		"type":      vncMessageTypeReady,
		"timestamp": time.Now(),
		"payload": VNCReadyMessage{
			SessionID: sessionID,
			VNCPort:   vncPort,
			PodName:   podName,
		},
	}

	if err := a.sendMessage(ready); err != nil {
		log.Printf("[VNCHandler] Failed to send VNC ready for session %s: %v", sessionID, err)
		return err
	}

	log.Printf("[VNCHandler] Sent VNC ready for session %s", sessionID)
	return nil
}

// sendVNCData sends VNC data from pod to Control Plane.
//
// The data is base64-encoded for transport over JSON WebSocket.
func (a *K8sAgent) sendVNCData(sessionID string, base64Data string) error {
	data := map[string]interface{}{
		"type":      vncMessageTypeData,
		"timestamp": time.Now(),
		"payload": VNCDataMessage{
			SessionID: sessionID,
			Data:      base64Data,
		},
	}

	return a.sendMessage(data)
}

// sendVNCError sends a VNC error notification to Control Plane.
//
// This is called when the VNC tunnel encounters an error.
func (a *K8sAgent) sendVNCError(sessionID string, errorMsg string) error {
	errMsg := map[string]interface{}{
		"type":      vncMessageTypeError,
		"timestamp": time.Now(),
		"payload": VNCErrorMessage{
			SessionID: sessionID,
			Error:     errorMsg,
		},
	}

	if err := a.sendMessage(errMsg); err != nil {
		log.Printf("[VNCHandler] Failed to send VNC error for session %s: %v", sessionID, err)
		return err
	}

	log.Printf("[VNCHandler] Sent VNC error for session %s: %s", sessionID, errorMsg)
	return nil
}

// initVNCTunnelForSession creates a VNC tunnel when a session starts.
//
// This is called automatically after a session is started successfully.
func (a *K8sAgent) initVNCTunnelForSession(sessionID string) error {
	if a.vncManager == nil {
		return fmt.Errorf("VNC manager not initialized")
	}

	log.Printf("[VNCHandler] Initializing VNC tunnel for session %s", sessionID)

	// Create the tunnel in a goroutine to avoid blocking command completion
	go func() {
		// Wait a bit for pod to be fully ready
		time.Sleep(2 * time.Second)

		if err := a.vncManager.CreateTunnel(sessionID); err != nil {
			log.Printf("[VNCHandler] Failed to create VNC tunnel for session %s: %v", sessionID, err)
			a.sendVNCError(sessionID, err.Error())
		}
	}()

	return nil
}

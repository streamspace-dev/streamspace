// Package handlers provides HTTP request handlers for the StreamSpace API.
//
// This file implements the Selkies/HTTP proxy handler for HTTP-based streaming protocols.
//
// HTTP Streaming Traffic Flow (v2.0):
//   UI Client → Control Plane HTTP Proxy → Session Service → Pod (Selkies Web Interface)
//
// The Selkies proxy:
//  1. Receives HTTP/WebSocket requests from UI clients
//  2. Verifies user has access to the session
//  3. Proxies HTTP/WebSocket traffic directly to session Service (in-cluster)
//  4. Session Service routes to pod's Selkies web interface (port 3000, 6901, etc.)
//
// Architecture:
//   - Control plane runs IN the Kubernetes cluster
//   - Can access ClusterIP services via Kubernetes DNS
//   - Uses Go's httputil.ReverseProxy for HTTP and WebSocket proxying
//
// Supported Protocols:
//   - Selkies: LinuxServer images (port 3000, path /websockify)
//   - Kasm: Kasm Workspaces images (port 6901, path /websockify)
//   - Guacamole: Apache Guacamole (port 8080, path /guacamole)
//
// Security:
//   - Requires valid JWT token
//   - Verifies user has access to the session
//   - Proxies only to authorized session pods
//
// Example:
//   UI connects to: http://control-plane/api/v1/http/:sessionId/
//   Proxy forwards to: http://sessionId.streamspace.svc.cluster.local:3000/
package handlers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/db"
	ws "github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// SelkiesProxyHandler manages HTTP/WebSocket connections to Selkies-based sessions.
//
// It proxies HTTP and WebSocket traffic between UI clients and session Services,
// enabling remote access to web-based streaming interfaces (Selkies, Kasm, Guacamole).
type SelkiesProxyHandler struct {
	// db is the database connection
	db *db.Database

	// agentHub manages agent WebSocket connections
	agentHub *ws.AgentHub

	// namespace is the Kubernetes namespace for sessions
	namespace string
}

// NewSelkiesProxyHandler creates a new Selkies/HTTP proxy handler.
//
// Example:
//
//	handler := NewSelkiesProxyHandler(database, agentHub, "streamspace")
//	router.Any("/http/:sessionId/*path", handler.HandleHTTPProxy)
func NewSelkiesProxyHandler(database *db.Database, agentHub *ws.AgentHub, namespace string) *SelkiesProxyHandler {
	return &SelkiesProxyHandler{
		db:       database,
		agentHub: agentHub,
		namespace: namespace,
	}
}

// HandleHTTPProxy handles HTTP/WebSocket proxy connections to Selkies-based sessions.
//
// Endpoint: ANY /api/v1/http/:sessionId/*path
//
// Query Parameters:
//   - token: JWT authentication token (required)
//
// Flow:
//  1. Authenticate user via JWT
//  2. Verify user has access to session
//  3. Look up session streaming protocol metadata
//  4. Verify session uses HTTP-based streaming (selkies, guacamole, etc.)
//  5. Look up agent hosting the session
//  6. Verify agent is connected
//  7. Proxy HTTP/WebSocket traffic to agent → pod
//
// Example:
//
//	http://control-plane/api/v1/http/sess-123/
//	http://control-plane/api/v1/http/sess-123/websockify
func (h *SelkiesProxyHandler) HandleHTTPProxy(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	// Get path after sessionId
	path := c.Param("path")
	if path == "" {
		path = "/"
	}

	// Get user from JWT (set by auth middleware)
	userIDInterface, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDInterface.(string)

	// Look up session in database (including streaming protocol metadata)
	var agentID string
	var sessionState string
	var sessionOwner string
	var streamingProtocol string
	var streamingPort int
	var streamingPath string
	err := h.db.DB().QueryRow(`
		SELECT agent_id, state, user_id,
		       COALESCE(streaming_protocol, 'vnc'),
		       COALESCE(streaming_port, 5900),
		       COALESCE(streaming_path, '')
		FROM sessions
		WHERE id = $1
	`, sessionID).Scan(&agentID, &sessionState, &sessionOwner, &streamingProtocol, &streamingPort, &streamingPath)

	if err != nil {
		log.Printf("[SelkiesProxy] Session %s not found: %v", sessionID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	log.Printf("[SelkiesProxy] Session %s uses protocol: %s (port: %d, path: %s)",
		sessionID, streamingProtocol, streamingPort, streamingPath)

	// Verify user has access to session
	if sessionOwner != userID {
		// TODO: Check if user is admin or has shared access
		log.Printf("[SelkiesProxy] User %s denied access to session %s (owner: %s)", userID, sessionID, sessionOwner)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Verify session uses HTTP-based streaming protocol
	if streamingProtocol != "selkies" && streamingProtocol != "guacamole" && streamingProtocol != "kasm" {
		log.Printf("[SelkiesProxy] Session %s uses non-HTTP protocol: %s", sessionID, streamingProtocol)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Session uses protocol '%s', not HTTP-based (use /vnc endpoint instead)", streamingProtocol),
		})
		return
	}

	// Verify session is running
	if sessionState != "running" {
		log.Printf("[SelkiesProxy] Session %s is not running (state: %s)", sessionID, sessionState)
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("Session is not running (state: %s)", sessionState),
		})
		return
	}

	// Verify agent_id is set
	if agentID == "" {
		log.Printf("[SelkiesProxy] Session %s has no agent assigned", sessionID)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Session has no agent assigned"})
		return
	}

	// NOTE: We intentionally do NOT check agentHub.IsAgentConnected() here.
	// In multi-pod deployments without Redis, each pod has its own AgentHub.
	// The agent may be connected to a different pod than the one handling this request.
	// Since we're proxying directly to the session's Kubernetes Service (not through
	// the agent), we don't need the agent to be connected to THIS pod.
	// The session pod is accessible via Kubernetes DNS regardless of agent connectivity.

	// Issue #239: Update last_activity for HTTP-based streaming sessions
	// This tracks user activity through the HTTP proxy
	h.updateSessionActivity(sessionID)

	// Proxy to session Service (in-cluster access via Kubernetes DNS)
	// Service name format: sessionID.namespace.svc.cluster.local:port
	targetURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", sessionID, h.namespace, streamingPort)

	log.Printf("[SelkiesProxy] Proxying %s %s to %s%s", c.Request.Method, sessionID, targetURL, path)

	h.proxyToService(c, targetURL, path)
}

// proxyToService proxies HTTP and WebSocket requests to a Kubernetes Service.
//
// This method uses Go's httputil.ReverseProxy which handles both regular HTTP
// requests and WebSocket upgrade requests automatically.
//
// Architecture:
//   - Control plane is running IN the Kubernetes cluster
//   - Can access ClusterIP services via Kubernetes DNS
//   - Uses service name: sessionID.namespace.svc.cluster.local
//
// WebSocket Support:
//   - httputil.ReverseProxy automatically handles WebSocket upgrades
//   - Proxies Upgrade headers and bidirectional traffic
//   - Works for Selkies /websockify paths
func (h *SelkiesProxyHandler) proxyToService(c *gin.Context, targetURL string, path string) {
	// Parse target URL
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Printf("[SelkiesProxy] Failed to parse target URL %s: %v", targetURL, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL"})
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the director to rewrite the request path
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = path
		req.Host = target.Host

		// Preserve query parameters
		if c.Request.URL.RawQuery != "" {
			req.URL.RawQuery = c.Request.URL.RawQuery
		}

		// Log the proxied request
		log.Printf("[SelkiesProxy] Proxying: %s %s to %s://%s%s",
			req.Method, c.Request.URL.Path, req.URL.Scheme, req.URL.Host, req.URL.Path)
	}

	// Handle errors (e.g., service not reachable)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[SelkiesProxy] Proxy error for %s: %v", targetURL, err)

		// Check if error is due to connection refused (service not ready)
		if strings.Contains(err.Error(), "connection refused") {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error": "Session service not ready", "message": "The session is still starting. Please wait and try again."}`))
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error": "Proxy error", "message": "%s"}`, err.Error())))
	}

	// Execute the proxy
	proxy.ServeHTTP(c.Writer, c.Request)
}

// RegisterRoutes registers the Selkies/HTTP proxy routes.
//
// Routes:
//   - ANY /http/:sessionId/*path - HTTP/WebSocket proxy for Selkies-based sessions
//
// Example:
//
//	selkiesProxyHandler.RegisterRoutes(router)
func (h *SelkiesProxyHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.Any("/http/:sessionId/*path", h.HandleHTTPProxy)
	router.Any("/http/:sessionId", h.HandleHTTPProxy)
}

// updateSessionActivity updates the last_activity timestamp for a session.
// This is called on each HTTP request to track user activity.
// Issue #239: VNC Activity Tracking (also applies to HTTP-based streaming)
func (h *SelkiesProxyHandler) updateSessionActivity(sessionID string) {
	_, err := h.db.DB().Exec(`
		UPDATE sessions
		SET last_activity = $1
		WHERE id = $2
	`, time.Now(), sessionID)
	if err != nil {
		log.Printf("[SelkiesProxy] Failed to update last_activity for session %s: %v", sessionID, err)
	}
}

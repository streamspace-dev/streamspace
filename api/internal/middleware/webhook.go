// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements webhook authentication using HMAC-SHA256 signatures.
//
// Purpose:
// The webhook authentication middleware validates that incoming webhook requests
// are genuinely from authorized sources by verifying cryptographic signatures.
// This prevents unauthorized parties from triggering webhook endpoints and
// injecting malicious data.
//
// Implementation Details:
// - HMAC-SHA256: Industry-standard message authentication algorithm
// - Secret-based signing: Shared secret between sender and receiver
// - Hex-encoded signatures: URL-safe, easy to debug
// - Constant-time comparison: Prevents timing attacks
// - Header-based delivery: Signature sent in X-Webhook-Signature header
//
// Security Notes:
// Without webhook authentication, attackers could:
// 1. Trigger automated actions (e.g., send notifications, create resources)
// 2. Inject malicious payloads (e.g., XSS in notification messages)
// 3. Cause denial of service (e.g., flood system with fake events)
// 4. Impersonate legitimate webhook sources (e.g., GitHub, Stripe, Slack)
//
// HMAC-SHA256 prevents these attacks:
// - Only parties with the secret can create valid signatures
// - Signatures are deterministic (same payload + secret = same signature)
// - Cannot forge signatures without knowing the secret
// - Constant-time comparison prevents timing attacks
//
// How It Works:
// 1. Sender (e.g., GitHub) computes HMAC-SHA256 of request body using secret
// 2. Sender includes signature in X-Webhook-Signature header
// 3. Receiver (StreamSpace) reads request body
// 4. Receiver computes expected signature using same secret
// 5. Receiver compares signatures using constant-time comparison
// 6. If match: Request is authentic, proceed
// 7. If mismatch: Request is invalid or tampered, reject with 401
//
// Signature Format:
//   X-Webhook-Signature: <hex-encoded-hmac-sha256>
//   Example: "a1b2c3d4e5f67890abcdef1234567890abcdef1234567890abcdef1234567890"
//
// Thread Safety:
// Safe for concurrent use. HMAC computation is stateless.
//
// Usage:
//   // Create webhook auth with secret
//   webhookAuth := middleware.NewWebhookAuth("your-secret-key-here")
//
//   // Apply to webhook endpoints
//   router.POST("/api/webhooks/github",
//       webhookAuth.Middleware(),
//       handlers.HandleGitHubWebhook,
//   )
//
//   // Generate signature for testing (sender side)
//   payload := []byte(`{"event": "push", "repo": "streamspace"}`)
//   signature := webhookAuth.Sign(payload)
//   // Send as: curl -H "X-Webhook-Signature: $signature" -d "$payload" /api/webhooks
//
// Configuration:
//   secret: Shared secret between sender and receiver (keep confidential!)
//   Recommended: Generate with: openssl rand -hex 32
package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookAuth validates webhook requests using HMAC-SHA256 signatures
type WebhookAuth struct {
	secret []byte
}

// NewWebhookAuth creates a new webhook authentication middleware
func NewWebhookAuth(secret string) *WebhookAuth {
	return &WebhookAuth{
		secret: []byte(secret),
	}
}

// Middleware returns a Gin middleware that validates webhook signatures
// Expects signature in X-Webhook-Signature header as hex-encoded HMAC-SHA256
func (w *WebhookAuth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get signature from header
		signature := c.GetHeader("X-Webhook-Signature")
		if signature == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Missing webhook signature",
			})
			c.Abort()
			return
		}

		// Read request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to read request body",
			})
			c.Abort()
			return
		}

		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// Compute HMAC
		mac := hmac.New(sha256.New, w.secret)
		mac.Write(body)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		// Compare signatures using constant-time comparison
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid webhook signature",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Sign generates an HMAC-SHA256 signature for the given payload
// This is a helper function for testing or generating signatures
func (w *WebhookAuth) Sign(payload []byte) string {
	mac := hmac.New(sha256.New, w.secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

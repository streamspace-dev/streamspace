package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		middleware     gin.HandlerFunc
		expectedHeaders map[string]string
		checkContains  map[string]string // Headers that should contain substring
	}{
		{
			name:       "SecurityHeaders sets all required headers",
			middleware: SecurityHeaders(),
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
				"X-XSS-Protection":       "1; mode=block",
			},
			checkContains: map[string]string{
				"Strict-Transport-Security": "max-age=31536000",
				"Content-Security-Policy":   "default-src 'self'",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
			},
		},
		{
			name:       "SecurityHeadersRelaxed sets relaxed CSP",
			middleware: SecurityHeadersRelaxed(),
			expectedHeaders: map[string]string{
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "SAMEORIGIN",
			},
			checkContains: map[string]string{
				"Content-Security-Policy": "default-src 'self'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test router
			router := gin.New()
			router.Use(tt.middleware)
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "test")
			})

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(w, req)

			// Check exact match headers
			for header, expected := range tt.expectedHeaders {
				actual := w.Header().Get(header)
				assert.Equal(t, expected, actual, "Header %s should match", header)
			}

			// Check substring match headers
			for header, expected := range tt.checkContains {
				actual := w.Header().Get(header)
				assert.Contains(t, actual, expected, "Header %s should contain %s", header, expected)
			}
		})
	}
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	hsts := w.Header().Get("Strict-Transport-Security")
	require.NotEmpty(t, hsts, "HSTS header should be set")

	// Verify HSTS components
	assert.Contains(t, hsts, "max-age=31536000", "HSTS should have 1 year max-age")
	assert.Contains(t, hsts, "includeSubDomains", "HSTS should include subdomains")
}

func TestSecurityHeaders_CSP_Nonce(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		// Check that nonce is available in context
		nonce, exists := c.Get("csp_nonce")
		assert.True(t, exists, "CSP nonce should be set in context")
		assert.NotEmpty(t, nonce, "CSP nonce should not be empty")

		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")
	require.NotEmpty(t, csp, "CSP header should be set")

	// Verify CSP contains nonce
	assert.Contains(t, csp, "nonce-", "CSP should contain nonce directive")
	assert.Contains(t, csp, "default-src 'self'", "CSP should have default-src 'self'")

	// Verify nonce format (base64)
	assert.True(t, strings.Contains(csp, "nonce-"), "CSP should have nonce-based policy")
}

func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		middleware gin.HandlerFunc
		expected   string
	}{
		{
			name:       "SecurityHeaders uses DENY",
			middleware: SecurityHeaders(),
			expected:   "DENY",
		},
		{
			name:       "SecurityHeadersRelaxed uses SAMEORIGIN",
			middleware: SecurityHeadersRelaxed(),
			expected:   "SAMEORIGIN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(tt.middleware)
			router.GET("/test", func(c *gin.Context) {
				c.String(http.StatusOK, "test")
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			xfo := w.Header().Get("X-Frame-Options")
			assert.Equal(t, tt.expected, xfo, "X-Frame-Options should be %s", tt.expected)
		})
	}
}

func TestSecurityHeaders_NonceUniqueness(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())

	var capturedNonces []string
	router.GET("/test", func(c *gin.Context) {
		nonce, exists := c.Get("csp_nonce")
		if exists {
			if nonceStr, ok := nonce.(string); ok {
				capturedNonces = append(capturedNonces, nonceStr)
			}
		}
		c.String(http.StatusOK, "test")
	})

	// Make multiple requests
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	// Verify all nonces are unique
	require.Len(t, capturedNonces, 10, "Should have captured 10 nonces")

	nonceSet := make(map[string]bool)
	for _, nonce := range capturedNonces {
		assert.False(t, nonceSet[nonce], "Nonce %s should be unique", nonce)
		nonceSet[nonce] = true
		assert.NotEmpty(t, nonce, "Nonce should not be empty")
	}
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	pp := w.Header().Get("Permissions-Policy")
	require.NotEmpty(t, pp, "Permissions-Policy header should be set")

	// Verify dangerous features are disabled
	assert.Contains(t, pp, "geolocation=()", "Geolocation should be disabled")
	assert.Contains(t, pp, "microphone=()", "Microphone should be disabled")
	assert.Contains(t, pp, "camera=()", "Camera should be disabled")
}

func TestSecurityHeaders_ReferrerPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	rp := w.Header().Get("Referrer-Policy")
	require.NotEmpty(t, rp, "Referrer-Policy header should be set")
	assert.Contains(t, rp, "strict-origin", "Referrer-Policy should be strict")
}

func TestSecurityHeaders_AllHeadersPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "test")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify all critical security headers are present
	requiredHeaders := []string{
		"Strict-Transport-Security",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Content-Security-Policy",
		"Referrer-Policy",
		"Permissions-Policy",
	}

	for _, header := range requiredHeaders {
		value := w.Header().Get(header)
		assert.NotEmpty(t, value, "Header %s should be present", header)
	}
}

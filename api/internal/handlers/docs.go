// Package handlers provides HTTP handlers for the StreamSpace API.
// This file implements API documentation endpoints serving OpenAPI/Swagger specification
// and interactive documentation UI.
//
// ENDPOINTS:
// - GET /api/docs        - Swagger UI (interactive documentation)
// - GET /api/docs/       - Swagger UI (with trailing slash)
// - GET /api/openapi.yaml - OpenAPI 3.0 specification (YAML)
// - GET /api/openapi.json - OpenAPI 3.0 specification (JSON)
//
// FEATURES:
// - Embedded Swagger UI via CDN (no local assets required)
// - OpenAPI 3.0 compliant specification
// - YAML and JSON format support
// - No authentication required (public documentation)
package handlers

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

//go:embed swagger.yaml
var swaggerYAML []byte

// DocsHandler handles API documentation endpoints
type DocsHandler struct{}

// NewDocsHandler creates a new documentation handler
func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

// RegisterRoutes registers documentation routes (no auth required)
func (h *DocsHandler) RegisterRoutes(router *gin.RouterGroup) {
	// Swagger UI
	router.GET("/docs", h.SwaggerUI)
	router.GET("/docs/", h.SwaggerUI)

	// OpenAPI spec in different formats
	router.GET("/openapi.yaml", h.OpenAPIYAML)
	router.GET("/openapi.json", h.OpenAPIJSON)

	// Convenience aliases
	router.GET("/swagger.yaml", h.OpenAPIYAML)
	router.GET("/swagger.json", h.OpenAPIJSON)
}

// SwaggerUI serves the Swagger UI HTML page
// @Summary API Documentation UI
// @Description Interactive Swagger UI for exploring the StreamSpace API
// @Tags documentation
// @Produce html
// @Success 200 {string} string "HTML page"
// @Router /api/docs [get]
func (h *DocsHandler) SwaggerUI(c *gin.Context) {
	// Get the base URL for the spec
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	specURL := scheme + "://" + host + "/api/openapi.yaml"

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>StreamSpace API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <link rel="icon" type="image/png" href="https://unpkg.com/swagger-ui-dist@5/favicon-32x32.png" sizes="32x32">
    <style>
        html { box-sizing: border-box; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
        .swagger-ui .topbar { display: none; }
        .swagger-ui .info .title { color: #3b4151; }
        .swagger-ui .info hgroup.main { margin: 0 0 20px 0; }
        .swagger-ui .info .description { margin-bottom: 30px; }
        /* Custom branding */
        .swagger-ui .info .title small {
            font-size: 14px;
            background: #49cc90;
            color: white;
            padding: 2px 8px;
            border-radius: 4px;
            margin-left: 10px;
            vertical-align: middle;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "` + specURL + `",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout",
                validatorUrl: null,
                supportedSubmitMethods: ['get', 'post', 'put', 'delete', 'patch'],
                defaultModelsExpandDepth: 1,
                defaultModelExpandDepth: 1,
                docExpansion: 'list',
                filter: true,
                showExtensions: true,
                showCommonExtensions: true,
                requestInterceptor: function(req) {
                    // Add bearer token from localStorage if available
                    var token = localStorage.getItem('streamspace_token');
                    if (token) {
                        req.headers['Authorization'] = 'Bearer ' + token;
                    }
                    return req;
                },
                onComplete: function() {
                    // Add version badge to title
                    var title = document.querySelector('.swagger-ui .info .title');
                    if (title && !title.querySelector('small')) {
                        var badge = document.createElement('small');
                        badge.textContent = 'v2.0-beta';
                        title.appendChild(badge);
                    }
                }
            });
        };
    </script>
</body>
</html>`

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, html)
}

// OpenAPIYAML serves the OpenAPI specification in YAML format
// @Summary OpenAPI Specification (YAML)
// @Description Get the OpenAPI 3.0 specification in YAML format
// @Tags documentation
// @Produce application/x-yaml
// @Success 200 {string} string "OpenAPI YAML specification"
// @Router /api/openapi.yaml [get]
func (h *DocsHandler) OpenAPIYAML(c *gin.Context) {
	c.Header("Content-Type", "application/x-yaml")
	c.Header("Content-Disposition", "inline; filename=\"openapi.yaml\"")
	c.String(http.StatusOK, string(swaggerYAML))
}

// OpenAPIJSON serves the OpenAPI specification in JSON format
// @Summary OpenAPI Specification (JSON)
// @Description Get the OpenAPI 3.0 specification in JSON format
// @Tags documentation
// @Produce application/json
// @Success 200 {object} map[string]interface{} "OpenAPI JSON specification"
// @Router /api/openapi.json [get]
func (h *DocsHandler) OpenAPIJSON(c *gin.Context) {
	// Parse YAML and convert to JSON
	var spec map[string]interface{}
	if err := yaml.Unmarshal(swaggerYAML, &spec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "ParseError",
			"message": "Failed to parse OpenAPI specification",
		})
		return
	}

	c.Header("Content-Disposition", "inline; filename=\"openapi.json\"")
	c.JSON(http.StatusOK, spec)
}

// GetSwaggerSpec returns the raw swagger specification bytes (for testing)
func GetSwaggerSpec() []byte {
	return swaggerYAML
}

// GetSwaggerSpecPath returns the OpenAPI spec URL path
func GetSwaggerSpecPath() string {
	return "/api/openapi.yaml"
}

// IsDocsPath checks if a path is a documentation path (for middleware exclusion)
func IsDocsPath(path string) bool {
	docsPaths := []string{
		"/api/docs",
		"/api/openapi.yaml",
		"/api/openapi.json",
		"/api/swagger.yaml",
		"/api/swagger.json",
	}
	for _, p := range docsPaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

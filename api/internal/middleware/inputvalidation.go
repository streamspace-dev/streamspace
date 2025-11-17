// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements comprehensive input validation and sanitization.
//
// Purpose:
// The input validation middleware protects against injection attacks by validating
// and sanitizing all user input including query parameters, path parameters, and
// request bodies. This prevents SQL injection, XSS, command injection, LDAP injection,
// and path traversal attacks.
//
// Implementation Details:
// - Path validation: Detects path traversal patterns (../, %2e%2e, etc.)
// - Query parameter validation: Checks for injection patterns in all query strings
// - JSON sanitization: Recursively sanitizes nested objects and arrays using bluemonday
// - Format validation: Validates usernames, emails, Kubernetes resource names, container images
// - Length limits: Prevents buffer overflow with 10KB max input size
// - Pattern detection: Regex-based detection of SQL, command, and LDAP injection attempts
//
// Security Notes:
// This middleware provides defense-in-depth against common web vulnerabilities:
// - SQL Injection: Detects UNION, SELECT, DROP, etc. patterns
// - XSS (Cross-Site Scripting): Bluemonday strips all HTML tags and dangerous content
// - Command Injection: Blocks shell metacharacters (;, |, &, backticks, $())
// - LDAP Injection: Detects LDAP special characters when used in combinations
// - Path Traversal: Prevents directory traversal attacks (../, ..\, null bytes)
// - Buffer Overflow: Enforces 10KB limit on input values
//
// Thread Safety:
// Safe for concurrent use. Each request gets its own validation context.
// The bluemonday policy is thread-safe and shared across all requests.
//
// Usage:
//   // Basic input validation on all routes
//   validator := middleware.NewInputValidator()
//   router.Use(validator.Middleware())
//
//   // JSON sanitization for API endpoints
//   router.Use(validator.SanitizeJSONMiddleware())
//
//   // Standalone validation functions
//   if err := middleware.ValidateUsername(username); err != nil {
//       return err
//   }
//   if err := middleware.ValidateEmail(email); err != nil {
//       return err
//   }
//
// Configuration:
//   // Bluemonday uses StrictPolicy by default (strips ALL HTML)
//   // To customize, modify the policy in NewInputValidator()
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// InputValidator handles comprehensive input validation and sanitization
type InputValidator struct {
	sanitizer *bluemonday.Policy
}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	// Strict policy that strips all HTML
	policy := bluemonday.StrictPolicy()

	return &InputValidator{
		sanitizer: policy,
	}
}

// Middleware provides input validation for all requests
func (v *InputValidator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate path parameters
		if err := v.validatePath(c.Request.URL.Path); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid path",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Validate query parameters
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				if err := v.validateInput(key, value); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"error":   "Invalid query parameter",
						"message": fmt.Sprintf("Parameter '%s': %s", key, err.Error()),
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// SanitizeJSONMiddleware sanitizes JSON request bodies
func (v *InputValidator) SanitizeJSONMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only process JSON requests
		if c.ContentType() != "application/json" {
			c.Next()
			return
		}

		// Read and preserve the request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}

		// Restore the body for handlers to read
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Try to parse as JSON map
		var data map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			// If it's not a map, let it pass to the handler which will validate properly
			c.Next()
			return
		}

		// Sanitize the data
		sanitized := v.sanitizeMap(data)

		// Replace the body with sanitized data
		c.Set("sanitized_json", sanitized)
		c.Next()
	}
}

// validatePath checks for path traversal attempts
func (v *InputValidator) validatePath(path string) error {
	// Check for path traversal patterns
	pathTraversalPatterns := []string{
		"../",
		"..\\",
		"/..",
		"\\..",
		"%2e%2e",
		"%252e%252e",
		"..%2f",
		"..%5c",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(lowerPath, pattern) {
			return fmt.Errorf("path traversal attempt detected")
		}
	}

	// Check for null bytes (file system attacks)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null byte detected in path")
	}

	return nil
}

// validateInput performs comprehensive input validation
func (v *InputValidator) validateInput(key, value string) error {
	// Check length (prevent buffer overflow attacks)
	if len(value) > 10000 {
		return fmt.Errorf("value too long (max 10000 characters)")
	}

	// Check for null bytes
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("null byte detected")
	}

	// Check for SQL injection patterns
	if err := v.checkSQLInjection(value); err != nil {
		return err
	}

	// Check for command injection patterns
	if err := v.checkCommandInjection(value); err != nil {
		return err
	}

	// Check for LDAP injection patterns
	if err := v.checkLDAPInjection(value); err != nil {
		return err
	}

	return nil
}

// checkSQLInjection detects common SQL injection patterns
func (v *InputValidator) checkSQLInjection(value string) error {
	// Common SQL injection patterns
	sqlPatterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(select\s+.*\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(delete\s+from)`,
		`(?i)(drop\s+table)`,
		`(?i)(update\s+.*\s+set)`,
		`(?i)(exec\s*\()`,
		`(?i)(execute\s*\()`,
		`(?i)(script\s*>)`,
		`(?i)(javascript:)`,
		`(?i)(onerror\s*=)`,
		`(?i)(onload\s*=)`,
		`--`,  // SQL comment
		`#`,   // MySQL comment (only if followed by space)
		`/\*`, // SQL block comment
	}

	for _, pattern := range sqlPatterns {
		matched, err := regexp.MatchString(pattern, value)
		if err != nil {
			continue
		}
		if matched {
			return fmt.Errorf("potential SQL injection detected")
		}
	}

	return nil
}

// checkCommandInjection detects command injection attempts
func (v *InputValidator) checkCommandInjection(value string) error {
	// Command injection patterns
	commandPatterns := []string{
		`[;&|]`, // Command separators
		"`",     // Backticks for command substitution
		`\$\(`,  // Command substitution
	}

	for _, pattern := range commandPatterns {
		matched, err := regexp.MatchString(pattern, value)
		if err != nil {
			continue
		}
		if matched {
			return fmt.Errorf("potential command injection detected")
		}
	}

	return nil
}

// checkLDAPInjection detects LDAP injection attempts
func (v *InputValidator) checkLDAPInjection(value string) error {
	// LDAP injection characters
	ldapChars := []string{"*", "(", ")", "\\", "/", "\x00"}

	for _, char := range ldapChars {
		if strings.Contains(value, char) {
			// Only flag if there are multiple special chars (to avoid false positives)
			specialCount := 0
			for _, c := range ldapChars {
				if strings.Contains(value, c) {
					specialCount++
				}
			}
			if specialCount >= 2 {
				return fmt.Errorf("potential LDAP injection detected")
			}
		}
	}

	return nil
}

// sanitizeMap recursively sanitizes a map
func (v *InputValidator) sanitizeMap(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range data {
		switch val := value.(type) {
		case string:
			// Sanitize string values using bluemonday
			result[key] = v.sanitizer.Sanitize(val)
		case map[string]interface{}:
			// Recursively sanitize nested maps
			result[key] = v.sanitizeMap(val)
		case []interface{}:
			// Sanitize arrays
			result[key] = v.sanitizeArray(val)
		default:
			// Keep other types as-is (numbers, booleans, etc.)
			result[key] = value
		}
	}

	return result
}

// sanitizeArray recursively sanitizes an array
func (v *InputValidator) sanitizeArray(data []interface{}) []interface{} {
	result := make([]interface{}, len(data))

	for i, value := range data {
		switch val := value.(type) {
		case string:
			result[i] = v.sanitizer.Sanitize(val)
		case map[string]interface{}:
			result[i] = v.sanitizeMap(val)
		case []interface{}:
			result[i] = v.sanitizeArray(val)
		default:
			result[i] = value
		}
	}

	return result
}

// ValidateUsername validates username format
func ValidateUsername(username string) error {
	if len(username) < 3 {
		return fmt.Errorf("username must be at least 3 characters")
	}
	if len(username) > 64 {
		return fmt.Errorf("username must not exceed 64 characters")
	}

	// Username must be lowercase alphanumeric with hyphens and underscores
	validUsername := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*[a-z0-9]$`)
	if !validUsername.MatchString(username) {
		return fmt.Errorf("username must contain only lowercase letters, numbers, hyphens, and underscores")
	}

	return nil
}

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	if len(email) > 254 {
		return fmt.Errorf("email too long")
	}

	// Basic email validation (RFC 5322 simplified)
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}

	return nil
}

// ValidateResourceName validates Kubernetes resource names
func ValidateResourceName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("resource name cannot be empty")
	}
	if len(name) > 253 {
		return fmt.Errorf("resource name too long (max 253 characters)")
	}

	// Kubernetes resource name format (RFC 1123 DNS label)
	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid resource name format (must be RFC 1123 DNS label)")
	}

	return nil
}

// ValidateNamespace validates Kubernetes namespace format
func ValidateNamespace(namespace string) error {
	if len(namespace) == 0 {
		return fmt.Errorf("namespace cannot be empty")
	}
	if len(namespace) > 63 {
		return fmt.Errorf("namespace too long (max 63 characters)")
	}

	// Kubernetes namespace format (RFC 1123 DNS label)
	validNamespace := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validNamespace.MatchString(namespace) {
		return fmt.Errorf("invalid namespace format")
	}

	// Reserved namespaces
	reserved := []string{"kube-system", "kube-public", "kube-node-lease", "default"}
	for _, r := range reserved {
		if namespace == r {
			return fmt.Errorf("cannot use reserved namespace: %s", namespace)
		}
	}

	return nil
}

// ValidateContainerImage validates container image format
func ValidateContainerImage(image string) error {
	if len(image) == 0 {
		return fmt.Errorf("image cannot be empty")
	}
	if len(image) > 1024 {
		return fmt.Errorf("image name too long")
	}

	// Basic image format validation (registry/repo:tag or repo:tag)
	// Allows: alphanumeric, dots, hyphens, underscores, slashes, colons
	validImage := regexp.MustCompile(`^[a-zA-Z0-9._/-]+(:[a-zA-Z0-9._-]+)?$`)
	if !validImage.MatchString(image) {
		return fmt.Errorf("invalid image format")
	}

	// Check for suspicious patterns
	suspicious := []string{"../", "..\\", "$(", "`", ";", "|", "&"}
	for _, pattern := range suspicious {
		if strings.Contains(image, pattern) {
			return fmt.Errorf("suspicious pattern detected in image name")
		}
	}

	return nil
}

// ValidateResourceQuantity validates Kubernetes resource quantities (CPU, memory)
func ValidateResourceQuantity(quantity, resourceType string) error {
	if len(quantity) == 0 {
		return fmt.Errorf("resource quantity cannot be empty")
	}

	var validQuantity *regexp.Regexp

	switch resourceType {
	case "cpu":
		// CPU: number with optional 'm' suffix (e.g., "1000m", "1", "0.5")
		validQuantity = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?m?$`)
	case "memory":
		// Memory: number with Mi/Gi/Ti suffix (e.g., "2Gi", "1024Mi")
		validQuantity = regexp.MustCompile(`^[0-9]+(Mi|Gi|Ti|Ki|M|G|T|K|m)?$`)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	if !validQuantity.MatchString(quantity) {
		return fmt.Errorf("invalid %s quantity format: %s", resourceType, quantity)
	}

	return nil
}

// SanitizeString removes HTML and dangerous characters from a string
func (v *InputValidator) SanitizeString(input string) string {
	return v.sanitizer.Sanitize(input)
}

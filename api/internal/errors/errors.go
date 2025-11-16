// Package errors provides standardized error handling for StreamSpace API.
//
// This package implements a consistent error format across all API endpoints:
//   - Structured error responses with error codes
//   - Automatic HTTP status code mapping
//   - Optional error details for debugging
//   - Machine-readable error codes for client error handling
//
// Error Structure:
//   - Code: Machine-readable error identifier (e.g., "QUOTA_EXCEEDED")
//   - Message: Human-readable error message
//   - Details: Optional additional context (wrapped errors, stack traces)
//   - StatusCode: HTTP status code (400, 401, 403, 404, 500, etc.)
//
// Error Categories:
//   - Client Errors (4xx): Bad request, unauthorized, forbidden, not found
//   - Server Errors (5xx): Internal errors, database errors, service unavailable
//
// Usage patterns:
//
//	// Simple error
//	return errors.NotFound("session")
//
//	// Error with custom message
//	return errors.QuotaExceeded("Maximum 5 sessions allowed")
//
//	// Wrap underlying error
//	return errors.DatabaseError(err)
//
//	// In HTTP handler
//	c.JSON(err.StatusCode, err.ToResponse())
//
// JSON Response Format:
//
//	{
//	  "error": "QUOTA_EXCEEDED",
//	  "message": "Session quota exceeded",
//	  "code": "QUOTA_EXCEEDED",
//	  "details": "5/5 sessions active"
//	}
package errors

import (
	"fmt"
	"net/http"
)

// AppError represents a standardized application error with HTTP context.
//
// AppError provides:
//   - Machine-readable error code for client error handling
//   - Human-readable message for display to users
//   - Optional details for debugging (not always shown to clients)
//   - Automatic HTTP status code mapping
//
// Example:
//
//	err := &AppError{
//	    Code: "QUOTA_EXCEEDED",
//	    Message: "Session quota exceeded: 5/5 sessions active",
//	    Details: "user1 has 5 running sessions, max allowed is 5",
//	    StatusCode: 403,
//	}
type AppError struct {
	// Code is a machine-readable error identifier.
	// Format: UPPER_SNAKE_CASE (e.g., "QUOTA_EXCEEDED", "NOT_FOUND")
	// Used by clients for programmatic error handling.
	Code string `json:"code"`

	// Message is a human-readable error description.
	// Should be suitable for display to end users.
	// Example: "Session quota exceeded: 5/5 sessions active"
	Message string `json:"message"`

	// Details provides additional context for debugging (optional).
	// May contain wrapped error messages, stack traces, or technical details.
	// Should not be shown to end users in production.
	// Example: "database query failed: connection timeout"
	Details string `json:"details,omitempty"`

	// StatusCode is the HTTP status code to return.
	// Automatically set based on error code.
	// Not included in JSON response (marked with `json:"-"`)
	StatusCode int `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s - %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorResponse represents the JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// Error codes
const (
	// Client errors (4xx)
	ErrCodeBadRequest          = "BAD_REQUEST"
	ErrCodeUnauthorized        = "UNAUTHORIZED"
	ErrCodeForbidden           = "FORBIDDEN"
	ErrCodeNotFound            = "NOT_FOUND"
	ErrCodeConflict            = "CONFLICT"
	ErrCodeValidationFailed    = "VALIDATION_FAILED"
	ErrCodeQuotaExceeded       = "QUOTA_EXCEEDED"
	ErrCodeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	ErrCodeSessionNotRunning   = "SESSION_NOT_RUNNING"
	ErrCodeSessionNotFound     = "SESSION_NOT_FOUND"
	ErrCodeTemplateNotFound    = "TEMPLATE_NOT_FOUND"
	ErrCodeUserNotFound        = "USER_NOT_FOUND"
	ErrCodeGroupNotFound       = "GROUP_NOT_FOUND"
	ErrCodeInvalidCredentials  = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired        = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid        = "TOKEN_INVALID"

	// Server errors (5xx)
	ErrCodeInternalServer      = "INTERNAL_SERVER_ERROR"
	ErrCodeDatabaseError       = "DATABASE_ERROR"
	ErrCodeKubernetesError     = "KUBERNETES_ERROR"
	ErrCodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
)

// New creates a new AppError
func New(code string, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCodeForErrorCode(code),
	}
}

// NewWithDetails creates a new AppError with details
func NewWithDetails(code string, message string, details string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		Details:    details,
		StatusCode: getStatusCodeForErrorCode(code),
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(code string, message string, err error) *AppError {
	details := ""
	if err != nil {
		details = err.Error()
	}
	return NewWithDetails(code, message, details)
}

// getStatusCodeForErrorCode returns the HTTP status code for an error code
func getStatusCodeForErrorCode(code string) int {
	switch code {
	case ErrCodeBadRequest, ErrCodeValidationFailed:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidCredentials, ErrCodeTokenExpired, ErrCodeTokenInvalid:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodeQuotaExceeded:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeSessionNotFound, ErrCodeTemplateNotFound, ErrCodeUserNotFound, ErrCodeGroupNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeInternalServer, ErrCodeDatabaseError, ErrCodeKubernetesError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ToResponse converts AppError to ErrorResponse
func (e *AppError) ToResponse() ErrorResponse {
	return ErrorResponse{
		Error:   e.Code,
		Message: e.Message,
		Code:    e.Code,
		Details: e.Details,
	}
}

// Common error constructors for convenience

func BadRequest(message string) *AppError {
	return New(ErrCodeBadRequest, message)
}

func Unauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message)
}

func Forbidden(message string) *AppError {
	return New(ErrCodeForbidden, message)
}

func NotFound(resource string) *AppError {
	return New(ErrCodeNotFound, fmt.Sprintf("%s not found", resource))
}

func Conflict(message string) *AppError {
	return New(ErrCodeConflict, message)
}

func ValidationFailed(message string) *AppError {
	return New(ErrCodeValidationFailed, message)
}

func QuotaExceeded(message string) *AppError {
	return New(ErrCodeQuotaExceeded, message)
}

func SessionNotRunning(sessionID string) *AppError {
	return New(ErrCodeSessionNotRunning, fmt.Sprintf("Session %s is not running", sessionID))
}

func SessionNotFound(sessionID string) *AppError {
	return New(ErrCodeSessionNotFound, fmt.Sprintf("Session %s not found", sessionID))
}

func TemplateNotFound(templateName string) *AppError {
	return New(ErrCodeTemplateNotFound, fmt.Sprintf("Template %s not found", templateName))
}

func UserNotFound(username string) *AppError {
	return New(ErrCodeUserNotFound, fmt.Sprintf("User %s not found", username))
}

func GroupNotFound(groupName string) *AppError {
	return New(ErrCodeGroupNotFound, fmt.Sprintf("Group %s not found", groupName))
}

func InvalidCredentials() *AppError {
	return New(ErrCodeInvalidCredentials, "Invalid username or password")
}

func TokenExpired() *AppError {
	return New(ErrCodeTokenExpired, "Authentication token has expired")
}

func TokenInvalid() *AppError {
	return New(ErrCodeTokenInvalid, "Invalid authentication token")
}

func InternalServer(message string) *AppError {
	return New(ErrCodeInternalServer, message)
}

func DatabaseError(err error) *AppError {
	return Wrap(ErrCodeDatabaseError, "Database operation failed", err)
}

func KubernetesError(err error) *AppError {
	return Wrap(ErrCodeKubernetesError, "Kubernetes operation failed", err)
}

func ServiceUnavailable(service string) *AppError {
	return New(ErrCodeServiceUnavailable, fmt.Sprintf("%s is currently unavailable", service))
}

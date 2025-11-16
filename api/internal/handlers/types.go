// Package handlers provides HTTP handlers for the StreamSpace API.
// This file defines common response types used across all handler files.
//
// COMMON TYPES:
// - ErrorResponse: Standardized error response format
// - SuccessResponse: Standardized success message format
//
// These types provide consistency across all API endpoints for error handling
// and success messaging. All handlers in this package use these types to
// ensure uniform API response structures.
//
// Thread Safety:
// - These are simple data structures with no shared state
//
// Dependencies:
// - None (pure data types)
package handlers

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Message string `json:"message"`
}

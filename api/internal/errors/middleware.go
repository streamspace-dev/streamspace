// Package errors provides standardized error handling for StreamSpace API.
//
// This file implements error handling middleware for Gin framework.
//
// Purpose:
// - Centralize error handling across all API endpoints
// - Convert AppError to consistent JSON responses
// - Log errors with appropriate severity levels
// - Recover from panics gracefully
// - Provide helper functions for error responses
//
// Features:
// - Automatic error logging (ERROR for 5xx, WARN for 4xx)
// - Panic recovery with error response
// - Consistent error response format
// - Error severity classification
// - Request abort on critical errors
//
// Middleware Functions:
//   - ErrorHandler: Handles AppError and generic errors
//   - Recovery: Recovers from panics
//   - HandleError: Helper for error responses in handlers
//   - AbortWithError: Helper to abort request with error
//
// Implementation Details:
// - Integrates with Gin's error handling mechanism (c.Errors)
// - Logs errors using standard library log (consider upgrading to structured logging)
// - Preserves error details for debugging
// - Automatically sets HTTP status codes
//
// Thread Safety:
// - Middleware is thread-safe
// - Safe for concurrent requests
//
// Dependencies:
// - github.com/gin-gonic/gin for HTTP framework
//
// Example Usage:
//
//	// Apply error handling middleware
//	router.Use(errors.Recovery())
//	router.Use(errors.ErrorHandler())
//
//	// In handler: return error and let middleware handle it
//	func handler(c *gin.Context) {
//	    session, err := getSession(id)
//	    if err != nil {
//	        errors.HandleError(c, errors.SessionNotFound(id))
//	        return
//	    }
//	    c.JSON(200, session)
//	}
//
//	// Or abort immediately
//	if !authorized {
//	    errors.AbortWithError(c, errors.Forbidden("Access denied"))
//	    return
//	}
package errors

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorHandler is a middleware that handles errors consistently
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Check if it's an AppError
			if appErr, ok := err.Err.(*AppError); ok {
				// Log the error with details
				if appErr.StatusCode >= 500 {
					log.Printf("[ERROR] %s - %s (Details: %s)", appErr.Code, appErr.Message, appErr.Details)
				} else {
					log.Printf("[WARN] %s - %s", appErr.Code, appErr.Message)
				}

				// Send the error response
				c.JSON(appErr.StatusCode, appErr.ToResponse())
				return
			}

			// Handle generic errors
			log.Printf("[ERROR] Unhandled error: %v", err.Err)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   ErrCodeInternalServer,
				Message: "An unexpected error occurred",
				Code:    ErrCodeInternalServer,
			})
		}
	}
}

// Recovery is a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[PANIC] Recovered from panic: %v", err)

				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Error:   ErrCodeInternalServer,
					Message: "An unexpected error occurred",
					Code:    ErrCodeInternalServer,
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}

// HandleError is a helper function to handle errors in handlers
func HandleError(c *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		c.Error(appErr)
		c.JSON(appErr.StatusCode, appErr.ToResponse())
	} else {
		internalErr := InternalServer(err.Error())
		c.Error(internalErr)
		c.JSON(internalErr.StatusCode, internalErr.ToResponse())
	}
}

// AbortWithError is a helper to abort request with error
func AbortWithError(c *gin.Context, err *AppError) {
	c.Error(err)
	c.AbortWithStatusJSON(err.StatusCode, err.ToResponse())
}

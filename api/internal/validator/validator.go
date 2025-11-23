package validator

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// validate is the singleton validator instance
var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom validators
	validate.RegisterValidation("password", validatePassword)
	validate.RegisterValidation("username", validateUsername)
}

// ValidateStruct validates a struct and returns user-friendly error messages
func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

// ValidateRequest validates a request struct and returns formatted errors
// Returns nil if validation passes, or a map of field errors
func ValidateRequest(s interface{}) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	errors := make(map[string]string)

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrs {
			field := strings.ToLower(e.Field())
			errors[field] = formatValidationError(e)
		}
	}

	return errors
}

// BindAndValidate binds JSON and validates in one step
// Returns true if successful, false if validation failed (and sets error response)
func BindAndValidate(c *gin.Context, req interface{}) bool {
	// Bind JSON
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return false
	}

	// Validate
	if errs := ValidateRequest(req); errs != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "Validation failed",
			"fields": errs,
		})
		return false
	}

	return true
}

// formatValidationError converts validator errors to human-readable messages
func formatValidationError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "email":
		return "Invalid email format"
	case "min":
		return fmt.Sprintf("Must be at least %s characters", e.Param())
	case "max":
		return fmt.Sprintf("Must be at most %s characters", e.Param())
	case "uuid":
		return "Must be a valid UUID"
	case "url":
		return "Must be a valid URL"
	case "oneof":
		return fmt.Sprintf("Must be one of: %s", e.Param())
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s", e.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s", e.Param())
	case "password":
		return "Password must be at least 8 characters with uppercase, lowercase, number, and special character"
	case "username":
		return "Username must be 3-50 characters, alphanumeric with hyphens/underscores only"
	default:
		return fmt.Sprintf("Validation failed: %s", e.Tag())
	}
}

// Custom Validators

// validatePassword ensures password meets security requirements
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// validateUsername ensures username follows allowed pattern
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()

	if len(username) < 3 || len(username) > 50 {
		return false
	}

	// Only alphanumeric, hyphens, and underscores
	for _, char := range username {
		if !((char >= 'a' && char <= 'z') ||
		     (char >= 'A' && char <= 'Z') ||
		     (char >= '0' && char <= '9') ||
		     char == '-' || char == '_') {
			return false
		}
	}

	return true
}

package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test structs
type TestUserRequest struct {
	Username string `json:"username" validate:"required,username"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,password"`
	Age      int    `json:"age" validate:"gte=0,lte=150"`
}

type TestSessionRequest struct {
	TemplateID string `json:"template_id" validate:"required,uuid"`
	Name       string `json:"name" validate:"required,min=3,max=100"`
	Timeout    int    `json:"timeout" validate:"gte=60,lte=86400"`
}

func TestValidateStruct_Success(t *testing.T) {
	req := TestSessionRequest{
		TemplateID: "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Test Session",
		Timeout:    3600,
	}

	err := ValidateStruct(req)
	assert.NoError(t, err)
}

func TestValidateStruct_RequiredFields(t *testing.T) {
	req := TestSessionRequest{
		// Missing required fields
	}

	err := ValidateStruct(req)
	assert.Error(t, err)
}

func TestValidateRequest_Success(t *testing.T) {
	req := TestUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Password: "SecureP@ss123",
		Age:      25,
	}

	errs := ValidateRequest(req)
	assert.Nil(t, errs)
}

func TestValidateRequest_MultipleErrors(t *testing.T) {
	req := TestUserRequest{
		// Invalid fields
		Username: "ab", // too short
		Email:    "not-an-email",
		Password: "weak",
		Age:      200, // too old
	}

	errs := ValidateRequest(req)
	assert.NotNil(t, errs)
	assert.Contains(t, errs, "username")
	assert.Contains(t, errs, "email")
	assert.Contains(t, errs, "password")
	assert.Contains(t, errs, "age")
}

func TestValidatePassword_Valid(t *testing.T) {
	validPasswords := []string{
		"SecureP@ss123",
		"MyP@ssw0rd!",
		"C0mpl3x!Pass",
		"Str0ng#Password",
	}

	for _, password := range validPasswords {
		req := TestUserRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: password,
			Age:      25,
		}

		errs := ValidateRequest(req)
		assert.Nil(t, errs, "Password should be valid: %s", password)
	}
}

func TestValidatePassword_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"too short", "Short1!"},
		{"no uppercase", "password123!"},
		{"no lowercase", "PASSWORD123!"},
		{"no number", "Password!"},
		{"no special", "Password123"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := TestUserRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: tt.password,
				Age:      25,
			}

			errs := ValidateRequest(req)
			assert.NotNil(t, errs)
			assert.Contains(t, errs, "password")
		})
	}
}

func TestValidateUsername_Valid(t *testing.T) {
	validUsernames := []string{
		"user",
		"test123",
		"my-user",
		"user_name",
		"User-Name_123",
	}

	for _, username := range validUsernames {
		req := TestUserRequest{
			Username: username,
			Email:    "test@example.com",
			Password: "SecureP@ss123",
			Age:      25,
		}

		errs := ValidateRequest(req)
		assert.Nil(t, errs, "Username should be valid: %s", username)
	}
}

func TestValidateUsername_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{"too short", "ab"},
		{"too long", "this_username_is_way_too_long_and_exceeds_the_fifty_character_limit"},
		{"invalid chars", "user@name"},
		{"spaces", "user name"},
		{"special chars", "user!name"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := TestUserRequest{
				Username: tt.username,
				Email:    "test@example.com",
				Password: "SecureP@ss123",
				Age:      25,
			}

			errs := ValidateRequest(req)
			assert.NotNil(t, errs)
			assert.Contains(t, errs, "username")
		})
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	invalidEmails := []string{
		"not-an-email",
		"@example.com",
		"user@",
		"user @example.com",
		"",
	}

	for _, email := range invalidEmails {
		req := TestUserRequest{
			Username: "testuser",
			Email:    email,
			Password: "SecureP@ss123",
			Age:      25,
		}

		errs := ValidateRequest(req)
		assert.NotNil(t, errs, "Email should be invalid: %s", email)
		assert.Contains(t, errs, "email")
	}
}

func TestValidateUUID_Valid(t *testing.T) {
	req := TestSessionRequest{
		TemplateID: "123e4567-e89b-12d3-a456-426614174000",
		Name:       "Test",
		Timeout:    60,
	}

	errs := ValidateRequest(req)
	assert.Nil(t, errs)
}

func TestValidateUUID_Invalid(t *testing.T) {
	invalidUUIDs := []string{
		"not-a-uuid",
		"123456",
		"123e4567-e89b-12d3-a456",
		"",
	}

	for _, uuid := range invalidUUIDs {
		req := TestSessionRequest{
			TemplateID: uuid,
			Name:       "Test",
			Timeout:    60,
		}

		errs := ValidateRequest(req)
		assert.NotNil(t, errs, "UUID should be invalid: %s", uuid)
		assert.Contains(t, errs, "templateid")
	}
}

func TestValidateMinMax_Strings(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		shouldErr bool
	}{
		{"valid", "Test Session", false},
		{"too short", "ab", true},
		{"too long", string(make([]byte, 101)), true},
		{"min length", "abc", false},
		{"max length", string(make([]byte, 100)), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := TestSessionRequest{
				TemplateID: "123e4567-e89b-12d3-a456-426614174000",
				Name:       tt.value,
				Timeout:    60,
			}

			errs := ValidateRequest(req)
			if tt.shouldErr {
				assert.NotNil(t, errs)
				assert.Contains(t, errs, "name")
			} else {
				assert.Nil(t, errs)
			}
		})
	}
}

func TestValidateRange_Numbers(t *testing.T) {
	tests := []struct {
		name      string
		timeout   int
		shouldErr bool
	}{
		{"valid", 3600, false},
		{"too small", 30, true},
		{"too large", 100000, true},
		{"min value", 60, false},
		{"max value", 86400, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := TestSessionRequest{
				TemplateID: "123e4567-e89b-12d3-a456-426614174000",
				Name:       "Test",
				Timeout:    tt.timeout,
			}

			errs := ValidateRequest(req)
			if tt.shouldErr {
				assert.NotNil(t, errs)
				assert.Contains(t, errs, "timeout")
			} else {
				assert.Nil(t, errs)
			}
		})
	}
}

func TestFormatValidationError(t *testing.T) {
	// Test that error messages are user-friendly
	req := TestUserRequest{
		Username: "",
		Email:    "invalid",
		Password: "weak",
		Age:      -1,
	}

	errs := ValidateRequest(req)
	assert.NotNil(t, errs)

	// Check that error messages are descriptive
	for field, msg := range errs {
		assert.NotEmpty(t, msg, "Error message should not be empty for field: %s", field)
		assert.NotContains(t, msg, "Validation failed", "Should use custom error message")
	}
}

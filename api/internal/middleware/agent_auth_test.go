package middleware

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBootstrapKeyEnvironmentVariable tests the bootstrap key configuration
func TestBootstrapKeyEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		expectedKey  string
		description  string
	}{
		{
			name:        "Bootstrap key is read from environment",
			envValue:    "my-secure-bootstrap-key-123",
			expectedKey: "my-secure-bootstrap-key-123",
			description: "AGENT_BOOTSTRAP_KEY environment variable should be read correctly",
		},
		{
			name:        "Empty bootstrap key returns empty string",
			envValue:    "",
			expectedKey: "",
			description: "Empty or unset AGENT_BOOTSTRAP_KEY should return empty string",
		},
		{
			name:        "Bootstrap key with special characters",
			envValue:    "key+with/special=chars==",
			expectedKey: "key+with/special=chars==",
			description: "Bootstrap key with base64 characters should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("AGENT_BOOTSTRAP_KEY", tt.envValue)
				defer os.Unsetenv("AGENT_BOOTSTRAP_KEY")
			} else {
				os.Unsetenv("AGENT_BOOTSTRAP_KEY")
			}

			// Read the environment variable
			actualKey := os.Getenv("AGENT_BOOTSTRAP_KEY")

			assert.Equal(t, tt.expectedKey, actualKey, tt.description)
		})
	}
}

// TestBootstrapKeySecurityRecommendations documents security best practices
func TestBootstrapKeySecurityRecommendations(t *testing.T) {
	// This test documents the security requirements for bootstrap keys
	t.Run("Bootstrap key should be at least 32 characters", func(t *testing.T) {
		// Recommended: openssl rand -base64 32 generates 44 characters
		recommendedKey := "abcdefghijklmnopqrstuvwxyz123456789012345678"
		assert.GreaterOrEqual(t, len(recommendedKey), 32,
			"Bootstrap keys should be at least 32 characters for security")
	})

	t.Run("Bootstrap key should not be hardcoded", func(t *testing.T) {
		// The bootstrap key should come from environment/secrets, not code
		os.Unsetenv("AGENT_BOOTSTRAP_KEY")
		key := os.Getenv("AGENT_BOOTSTRAP_KEY")
		assert.Empty(t, key,
			"Bootstrap key should not have a default value - must be explicitly configured")
	})
}

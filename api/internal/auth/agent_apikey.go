// Package auth provides authentication and authorization utilities.
// This file implements API key authentication for agents.
//
// SECURITY: Agent API Key Authentication
//
// Agents authenticate using API keys instead of JWT tokens because:
//   - Agents are not users (no username/password)
//   - Agents are long-running services (no interactive login)
//   - API keys are simpler and more suitable for service-to-service auth
//
// API Key Format:
//   - 64 hexadecimal characters (32 bytes of randomness)
//   - Generated using crypto/rand
//   - Example: "a1b2c3d4e5f6...789" (64 chars)
//
// API Key Storage:
//   - Plaintext key given to agent ONCE during deployment
//   - Bcrypt hash stored in database (cost factor 12)
//   - Hash never exposed in API responses
//
// API Key Usage:
//   - Agent sends key in X-Agent-API-Key header
//   - API validates key against bcrypt hash in database
//   - Updates api_key_last_used_at on successful auth
//
// API Key Rotation:
//   - Admin can generate new key via /api/v1/admin/agents/:id/rotate-key
//   - Old key immediately invalidated
//   - New key returned ONCE (must be saved by admin)
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyLength is the length of generated API keys in bytes (32 bytes = 64 hex chars)
	APIKeyLength = 32

	// BcryptCost is the cost factor for bcrypt hashing (12 = ~250ms per hash)
	BcryptCost = 12
)

// GenerateAPIKey generates a cryptographically random API key.
//
// Returns a 64-character hexadecimal string (32 bytes of randomness).
//
// Example:
//
//	key, err := GenerateAPIKey()
//	// key = "a1b2c3d4e5f6...789" (64 chars)
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, APIKeyLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// HashAPIKey hashes an API key using bcrypt.
//
// The hash can be safely stored in the database and compared against
// provided keys using CompareAPIKey.
//
// Cost factor is set to 12 (~250ms per hash) for security.
//
// Example:
//
//	hash, err := HashAPIKey("a1b2c3d4e5f6...789")
//	// Store hash in database
func HashAPIKey(key string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(key), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}
	return string(bytes), nil
}

// CompareAPIKey compares a plaintext API key against a bcrypt hash.
//
// Returns true if the key matches the hash, false otherwise.
//
// Example:
//
//	valid := CompareAPIKey("a1b2c3d4e5f6...789", storedHash)
//	if valid {
//	    // Key is valid
//	}
func CompareAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// APIKeyMetadata contains metadata about an API key.
//
// Used when generating new keys to return both the plaintext key
// and metadata for storage in the database.
type APIKeyMetadata struct {
	// PlaintextKey is the unhashed API key (64 hex chars)
	// SECURITY: This should only be shown to the admin ONCE
	PlaintextKey string

	// Hash is the bcrypt hash of the key
	// This is what gets stored in the database
	Hash string

	// CreatedAt is when the key was generated
	CreatedAt time.Time
}

// GenerateAPIKeyWithMetadata generates a new API key and returns both
// the plaintext key and metadata for database storage.
//
// The plaintext key should be shown to the admin ONCE and then discarded.
// Only the hash should be stored in the database.
//
// Example:
//
//	metadata, err := GenerateAPIKeyWithMetadata()
//	if err != nil {
//	    return err
//	}
//
//	// Show to admin ONCE
//	fmt.Printf("New API key: %s\n", metadata.PlaintextKey)
//	fmt.Println("SAVE THIS KEY - it will not be shown again")
//
//	// Store in database
//	_, err = db.Exec(
//	    "UPDATE agents SET api_key_hash = $1, api_key_created_at = $2 WHERE id = $3",
//	    metadata.Hash, metadata.CreatedAt, agentID,
//	)
func GenerateAPIKeyWithMetadata() (*APIKeyMetadata, error) {
	// Generate random key
	key, err := GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	// Hash the key
	hash, err := HashAPIKey(key)
	if err != nil {
		return nil, err
	}

	return &APIKeyMetadata{
		PlaintextKey: key,
		Hash:         hash,
		CreatedAt:    time.Now(),
	}, nil
}

// ValidateAPIKeyFormat checks if an API key has the correct format.
//
// Valid format: 64 hexadecimal characters (32 bytes)
//
// Returns error if format is invalid.
//
// Example:
//
//	if err := ValidateAPIKeyFormat(key); err != nil {
//	    return fmt.Errorf("invalid API key format: %w", err)
//	}
func ValidateAPIKeyFormat(key string) error {
	if len(key) != APIKeyLength*2 { // 2 hex chars per byte
		return fmt.Errorf("API key must be %d characters (got %d)", APIKeyLength*2, len(key))
	}

	// Check if all characters are hexadecimal
	if _, err := hex.DecodeString(key); err != nil {
		return fmt.Errorf("API key must contain only hexadecimal characters")
	}

	return nil
}

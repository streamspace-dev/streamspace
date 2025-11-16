// Package auth provides authentication and authorization mechanisms for StreamSpace.
// This file implements secure token generation and hashing for API tokens, session
// tokens, and other authentication credentials.
//
// TOKEN TYPES AND USE CASES:
//
// StreamSpace uses different token types for different purposes:
//
// 1. API Tokens (Long-lived):
//    - Used for programmatic API access
//    - Stored in database with bcrypt hash
//    - 384 bits of entropy (48 bytes)
//    - Never expire (until revoked by user)
//    - Example: Personal access tokens, integration keys
//
// 2. Session Tokens (Short-lived):
//    - Used for web session management
//    - Stored in database with SHA256 hash
//    - 256 bits of entropy (32 bytes)
//    - Expire after inactivity or logout
//    - Example: Browser session cookies
//
// 3. Generic Secure Tokens:
//    - Used for password reset, email verification, etc.
//    - Stored with bcrypt hash for security
//    - Configurable length
//    - Single-use tokens with expiration
//
// HASHING ALGORITHMS:
//
// Two hashing algorithms are provided based on use case requirements:
//
// 1. bcrypt (For API Tokens):
//    - Intentionally slow (prevents brute force attacks)
//    - Adaptive work factor (can increase over time)
//    - Salt included automatically
//    - Recommended for long-lived tokens
//    - Cost factor: 10 (good security/performance balance)
//
// 2. SHA256 (For Session Tokens):
//    - Fast hashing (suitable for high-volume lookups)
//    - 256-bit output (sufficient for session tokens)
//    - No built-in salt (tokens are cryptographically random)
//    - Recommended for short-lived, high-frequency tokens
//
// WHY DIFFERENT ALGORITHMS?
//
// bcrypt vs SHA256 trade-offs:
//
// bcrypt Advantages:
// - Slow hashing prevents brute force attacks
// - Adaptive cost factor (security improves over time)
// - Best practice for password-like credentials
//
// bcrypt Disadvantages:
// - Slower performance (intentional, but impacts throughput)
// - Not suitable for high-frequency validation
//
// SHA256 Advantages:
// - Fast validation (thousands of lookups per second)
// - Suitable for session tokens with high request rates
// - Simple implementation
//
// SHA256 Disadvantages:
// - Fast hashing makes brute force easier
// - No adaptive cost factor
// - Not recommended for long-lived credentials
//
// SECURITY BEST PRACTICES:
//
// 1. Token Generation:
//    - Use crypto/rand for cryptographically secure randomness
//    - Never use math/rand (predictable, insecure)
//    - Sufficient entropy: 32+ bytes for session, 48+ bytes for API
//    - Base64 URL encoding for safe transmission
//
// 2. Token Storage:
//    - NEVER store plain tokens in database
//    - Always hash before storage (bcrypt or SHA256)
//    - Store hash only, discard plain token after giving to user
//    - Use prepared statements to prevent SQL injection
//
// 3. Token Transmission:
//    - Send tokens over HTTPS only
//    - Use secure, httpOnly cookies for session tokens
//    - Never log or expose tokens in error messages
//    - Clear tokens from memory after use
//
// 4. Token Expiration:
//    - Session tokens: Short lifetime (hours to days)
//    - API tokens: Long lifetime but allow user revocation
//    - Reset tokens: Very short lifetime (minutes to hours)
//    - Always enforce expiration checks
//
// 5. Token Revocation:
//    - Support manual revocation by users
//    - Revoke all tokens on password change
//    - Track last used timestamp for auditing
//    - Remove expired tokens regularly
//
// TOKEN GENERATION PROCESS:
//
// 1. Generate Random Bytes:
//    - Use crypto/rand.Read() for secure randomness
//    - Generate sufficient bytes for desired entropy
//    - Check for errors (rare, but possible on some systems)
//
// 2. Encode Plain Token:
//    - Base64 URL encode random bytes
//    - URL-safe encoding (no +, /, or = padding issues)
//    - Result is alphanumeric string safe for URLs
//
// 3. Hash Token:
//    - Hash plain token with bcrypt or SHA256
//    - Store hash in database
//    - Return plain token to user (shown only once)
//
// 4. User Storage:
//    - User stores plain token securely (password manager, env var)
//    - User includes token in API requests
//    - Server hashes incoming token and compares with stored hash
//
// EXAMPLE USAGE:
//
//   hasher := NewTokenHasher()
//
//   // Generate API token (long-lived, bcrypt)
//   plainToken, hashedToken, err := hasher.GenerateAPIToken()
//   if err != nil {
//       log.Fatal(err)
//   }
//   // Store hashedToken in database
//   // Give plainToken to user (show only once!)
//
//   // Generate session token (short-lived, SHA256)
//   plainSession, hashedSession, err := hasher.GenerateSessionToken()
//   if err != nil {
//       log.Fatal(err)
//   }
//   // Store hashedSession in database
//   // Set plainSession as secure cookie
//
//   // Verify API token (bcrypt)
//   valid := hasher.VerifyToken(userProvidedToken, storedHash)
//   if !valid {
//       return errors.New("invalid token")
//   }
//
//   // Verify session token (SHA256 - faster)
//   valid := hasher.VerifyTokenSHA256(cookieToken, storedSessionHash)
//   if !valid {
//       return errors.New("invalid session")
//   }
//
// COMMON VULNERABILITIES TO AVOID:
//
// 1. Weak Random Number Generation:
//    - ❌ DON'T use math/rand (predictable)
//    - ✅ DO use crypto/rand (cryptographically secure)
//
// 2. Storing Plain Tokens:
//    - ❌ DON'T store plain tokens in database
//    - ✅ DO store only hashed tokens
//
// 3. Insufficient Entropy:
//    - ❌ DON'T use short tokens (< 16 bytes)
//    - ✅ DO use 32+ bytes for sessions, 48+ for API tokens
//
// 4. No Expiration:
//    - ❌ DON'T allow tokens to live forever
//    - ✅ DO enforce expiration and support revocation
//
// 5. Timing Attacks:
//    - ❌ DON'T use == for token comparison
//    - ✅ DO use bcrypt.CompareHashAndPassword (constant-time)
//
// PERFORMANCE CONSIDERATIONS:
//
// bcrypt Cost Factor Trade-offs:
//
// Cost 10 (Default):
// - ~60ms per hash on modern CPU
// - ~16 hashes per second per core
// - Suitable for login and API token validation
//
// Cost 12 (Higher Security):
// - ~240ms per hash
// - ~4 hashes per second per core
// - Use for extra-sensitive tokens
//
// Cost 14 (Maximum Security):
// - ~960ms per hash
// - ~1 hash per second per core
// - May impact user experience
//
// SHA256 Performance:
// - Microseconds per hash
// - Millions of hashes per second
// - Use for session tokens with high request rates
//
// THREAD SAFETY:
//
// All methods are thread-safe and can be called concurrently from multiple
// goroutines. Each token generation operation is independent.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// TokenHasher handles secure token generation and hashing
type TokenHasher struct {
	bcryptCost int
}

// NewTokenHasher creates a new token hasher
func NewTokenHasher() *TokenHasher {
	return &TokenHasher{
		bcryptCost: bcrypt.DefaultCost, // Cost 10 for good security/performance balance
	}
}

// GenerateSecureToken generates a cryptographically secure random token
// Returns the plain token (for giving to user) and the hashed token (for storage)
func (t *TokenHasher) GenerateSecureToken(length int) (plainToken string, hashedToken string, err error) {
	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random token: %w", err)
	}

	// Encode as base64 for the plain token
	plainToken = base64.URLEncoding.EncodeToString(bytes)

	// Hash the token for storage
	hashedToken, err = t.HashToken(plainToken)
	if err != nil {
		return "", "", err
	}

	return plainToken, hashedToken, nil
}

// HashToken hashes a token using bcrypt for secure storage
// bcrypt is intentionally slow to prevent brute force attacks
func (t *TokenHasher) HashToken(token string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(token), t.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}
	return string(hashedBytes), nil
}

// VerifyToken verifies a plain token against a hashed token
func (t *TokenHasher) VerifyToken(plainToken, hashedToken string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedToken), []byte(plainToken))
	return err == nil
}

// HashTokenSHA256 provides a faster hash for session tokens where lookup speed is critical
// Use this for session tokens that need fast validation
// Note: Less secure than bcrypt for password-like tokens, but acceptable for session tokens
func (t *TokenHasher) HashTokenSHA256(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// VerifyTokenSHA256 verifies a token against a SHA256 hash
func (t *TokenHasher) VerifyTokenSHA256(plainToken, hashedToken string) bool {
	computedHash := t.HashTokenSHA256(plainToken)
	return computedHash == hashedToken
}

// GenerateSessionToken generates a session-specific token
// Returns plain token and SHA256 hash (faster for session validation)
func (t *TokenHasher) GenerateSessionToken() (plainToken string, hashedToken string, err error) {
	// 32 bytes = 256 bits of entropy
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate session token: %w", err)
	}

	plainToken = base64.URLEncoding.EncodeToString(bytes)
	hashedToken = t.HashTokenSHA256(plainToken)

	return plainToken, hashedToken, nil
}

// GenerateAPIToken generates an API token (uses bcrypt for better security)
// Returns plain token and bcrypt hash
func (t *TokenHasher) GenerateAPIToken() (plainToken string, hashedToken string, err error) {
	// 48 bytes = 384 bits of entropy for long-lived tokens
	bytes := make([]byte, 48)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("failed to generate API token: %w", err)
	}

	plainToken = base64.URLEncoding.EncodeToString(bytes)

	// Use bcrypt for API tokens (they're long-lived and need stronger protection)
	hashedToken, err = t.HashToken(plainToken)
	if err != nil {
		return "", "", err
	}

	return plainToken, hashedToken, nil
}

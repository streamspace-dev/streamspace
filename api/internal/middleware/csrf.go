// Package middleware provides HTTP middleware for the StreamSpace API.
// This file implements CSRF (Cross-Site Request Forgery) protection.
//
// SECURITY ENHANCEMENT (2025-11-14):
// Added CSRF protection using double-submit cookie pattern with constant-time comparison.
//
// CSRF Attack Scenario (Without Protection):
// 1. User logs into StreamSpace (gets session cookie)
// 2. User visits malicious site evil.com
// 3. evil.com contains: <form action="https://streamspace.io/api/delete-account" method="POST">
// 4. Browser automatically sends session cookie with the malicious request
// 5. StreamSpace deletes user's account (thinks it's a legitimate request)
//
// CSRF Protection (Double-Submit Cookie Pattern):
// 1. GET request: Server generates random CSRF token, sends in both cookie AND header
// 2. Client stores header token (JavaScript can read it)
// 3. POST request: Client sends token in both cookie AND custom header
// 4. Server compares: cookie token == header token (using constant-time comparison)
// 5. If match: Request is from legitimate client (evil.com can't read/set custom headers)
// 6. If mismatch: Request is CSRF attack (blocked)
//
// Why This Works:
// - Malicious sites can trigger POST requests (via forms, fetch)
// - Browsers automatically send cookies with requests (even cross-site)
// - BUT: Malicious sites CANNOT read cookies or set custom headers (Same-Origin Policy)
// - So attacker cannot get the token to put in the custom header
//
// Implementation Details:
// - Token: 32 random bytes, base64-encoded (256 bits of entropy)
// - Comparison: Constant-time (prevents timing attacks)
// - Storage: In-memory map with automatic cleanup (24-hour expiry)
// - Exempt: GET, HEAD, OPTIONS requests (safe methods, no state change)
//
// Usage:
//   router.Use(middleware.CSRFProtection())
package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CSRF Constants define CSRF protection configuration.
const (
	// CSRFTokenLength is the length of CSRF tokens in bytes
	CSRFTokenLength = 32

	// CSRFTokenHeader is the HTTP header for CSRF tokens
	CSRFTokenHeader = "X-CSRF-Token"

	// CSRFCookieName is the name of the CSRF cookie
	CSRFCookieName = "csrf_token"

	// CSRFTokenExpiry is how long CSRF tokens are valid
	CSRFTokenExpiry = 24 * time.Hour
)

// CSRFStore stores CSRF tokens with expiration
type CSRFStore struct {
	tokens map[string]time.Time
	mu     sync.RWMutex
}

var (
	globalCSRFStore = &CSRFStore{
		tokens: make(map[string]time.Time),
	}
	csrfCleanupOnce sync.Once

	// tokenGenerationMu protects against race conditions when multiple
	// GET requests arrive simultaneously and try to generate new tokens
	tokenGenerationMu sync.Mutex
)

// generateCSRFToken generates a cryptographically secure random CSRF token.
//
// The token is used in the double-submit cookie pattern to prevent CSRF attacks.
// It must be unpredictable and unique per session to be effective.
//
// TOKEN GENERATION:
//
// 1. Generate 32 random bytes from crypto/rand (256 bits of entropy)
// 2. Encode as base64 URL-safe string (43 characters)
// 3. Return token (e.g., "x7k9m2n4p8q3r5s1t6u0v2w4y8z1a3b5c7d9e0f2g4h6j8k0")
//
// WHY 32 BYTES (256 BITS):
//
// - NIST recommends minimum 128 bits for session tokens
// - 256 bits provides very high security margin
// - Probability of collision: 1 in 2^256 (practically impossible)
// - Cannot be brute-forced even with all computers on Earth
//
// SECURITY: crypto/rand vs math/rand
//
// ✅ crypto/rand: Cryptographically secure (uses OS entropy pool)
// ❌ math/rand: NOT secure (predictable, seedable)
//
// Using math/rand would allow attackers to predict tokens:
// 1. Attacker observes one token
// 2. Attacker reverse-engineers seed
// 3. Attacker generates future tokens
// 4. CSRF protection bypassed
//
// BASE64 URL ENCODING:
//
// - Standard Base64 uses: +, /, =
// - URL-safe Base64 uses: -, _, (no padding)
// - Safe for URLs, cookies, headers
// - No escaping needed
//
// ERROR HANDLING:
//
// crypto/rand.Read returns error only if system entropy pool is unavailable.
// This is extremely rare and indicates serious system issues:
// - Out of memory
// - /dev/urandom unavailable (Linux)
// - CryptGenRandom unavailable (Windows)
//
// If this happens, server should NOT proceed with CSRF protection disabled.
//
// EXAMPLE:
//
//   token, err := generateCSRFToken()
//   if err != nil {
//       log.Fatal("Cannot generate secure tokens:", err)
//   }
//   // token = "x7k9m2n4p8q3r5s1t6u0v2w4y8z1a3b5c7d9e0f2g4h6j8k0"
func generateCSRFToken() (string, error) {
	// Allocate 32-byte buffer for random data
	bytes := make([]byte, CSRFTokenLength)

	// Fill buffer with cryptographically secure random bytes
	// crypto/rand uses OS entropy pool (/dev/urandom on Linux)
	if _, err := rand.Read(bytes); err != nil {
		// System entropy unavailable - should never happen
		// Do NOT fall back to insecure random source
		return "", err
	}

	// Encode as base64 URL-safe string (no +, /, = characters)
	// Results in 43-character string for 32 bytes of input
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// addToken adds a token to the store with expiration
func (cs *CSRFStore) addToken(token string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.tokens[token] = time.Now().Add(CSRFTokenExpiry)
}

// validateToken checks if a token is valid and not expired
func (cs *CSRFStore) validateToken(token string) bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	
	expiry, exists := cs.tokens[token]
	if !exists {
		return false
	}
	
	// Check if expired
	if time.Now().After(expiry) {
		return false
	}
	
	return true
}

// removeToken removes a token from the store
func (cs *CSRFStore) removeToken(token string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.tokens, token)
}

// cleanup removes expired tokens from the store periodically.
//
// This background goroutine prevents the token store from growing unbounded
// by removing expired tokens every hour.
//
// CLEANUP STRATEGY:
//
// - Runs every 1 hour
// - Scans all tokens in store
// - Deletes tokens where expiry < now
// - Holds write lock only during deletion (not entire hour)
//
// MEMORY MANAGEMENT:
//
// Without cleanup, tokens would accumulate indefinitely:
// - 1000 req/sec = 86,400,000 tokens/day
// - Each token ~100 bytes (string + time.Time)
// - Total: ~8.6 GB/day memory leak
//
// With hourly cleanup:
// - Max tokens = requests in 24 hours (token expiry)
// - Cleanup removes tokens > 24 hours old
// - Steady-state memory usage
//
// CONCURRENCY SAFETY:
//
// - Uses mu.Lock() for write access
// - Safe to run concurrently with addToken/validateToken
// - Other goroutines block during cleanup but only briefly
//
// WHY HOURLY:
//
// - Balance between memory usage and CPU overhead
// - More frequent = less memory, more CPU
// - Less frequent = more memory, less CPU
// - 1 hour is reasonable middle ground
//
// PRODUCTION ALTERNATIVES:
//
// For high-traffic production:
// 1. Use Redis for token storage (automatic expiry)
// 2. Use LRU cache with size limit
// 3. Use database with TTL index
// 4. Partition tokens by expiry time (faster cleanup)
//
// GOROUTINE LIFECYCLE:
//
// This goroutine runs forever (until process exits).
// Started once via sync.Once in CSRFProtection middleware.
// No graceful shutdown implemented (acceptable for background task).
func (cs *CSRFStore) cleanup() {
	// Create ticker that fires every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Infinite loop: cleanup on every tick
	for range ticker.C {
		// STEP 1: Acquire write lock
		// Blocks all token operations during cleanup
		cs.mu.Lock()

		// STEP 2: Get current time for comparison
		now := time.Now()

		// STEP 3: Scan all tokens
		// Delete those that have expired
		for token, expiry := range cs.tokens {
			if now.After(expiry) {
				delete(cs.tokens, token)
			}
		}

		// STEP 4: Release lock
		// Allow other operations to proceed
		cs.mu.Unlock()
	}
}

// CSRFProtection returns a Gin middleware that protects against Cross-Site Request
// Forgery (CSRF) attacks using the double-submit cookie pattern.
//
// CSRF ATTACK OVERVIEW:
//
// CSRF attacks exploit the browser's automatic cookie sending behavior to perform
// unauthorized actions on behalf of an authenticated user.
//
// Attack scenario without protection:
//   1. User logs into StreamSpace → gets session cookie
//   2. User visits malicious site evil.com
//   3. evil.com triggers: POST https://streamspace.io/api/delete-account
//   4. Browser automatically sends session cookie with request
//   5. StreamSpace sees valid session → executes action
//   6. User's account is deleted without their knowledge
//
// DOUBLE-SUBMIT COOKIE PATTERN:
//
// This implementation uses the double-submit cookie pattern, which requires:
// 1. Server generates random token
// 2. Server sends token in BOTH cookie AND custom header
// 3. Client JavaScript reads token from header
// 4. Client sends token in BOTH cookie AND custom header on state-changing requests
// 5. Server validates: cookie token == header token
//
// Why this works:
// - Malicious sites can trigger requests and browsers send cookies automatically
// - BUT: Same-Origin Policy prevents malicious sites from:
//   * Reading the token from response headers
//   * Setting custom headers on cross-origin requests
// - Therefore, attacker cannot provide matching tokens in both places
//
// PROTECTION FLOW:
//
// Safe Request (GET, HEAD, OPTIONS):
//   1. Client: GET /api/sessions
//   2. Server: Generates CSRF token (e.g., "abc123...")
//   3. Server: Sets X-CSRF-Token header to "abc123..."
//   4. Server: Sets csrf_token cookie to "abc123..."
//   5. Client: Stores token from header in memory/localStorage
//   6. Response returned
//
// State-Changing Request (POST, PUT, DELETE, PATCH):
//   1. Client: POST /api/delete-account
//   2. Client: Sets X-CSRF-Token header to "abc123..." (from previous GET)
//   3. Client: Browser automatically sends csrf_token cookie "abc123..."
//   4. Server: Reads token from header → "abc123..."
//   5. Server: Reads token from cookie → "abc123..."
//   6. Server: Compares using constant-time comparison
//   7. Server: Validates token exists and not expired
//   8. If all checks pass: Request processed
//   9. If any check fails: 403 Forbidden
//
// CSRF Attack Scenario (With Protection):
//   1. Attacker: POST https://streamspace.io/api/delete-account
//   2. Browser: Sends csrf_token cookie automatically → "abc123..."
//   3. Attacker: Cannot set X-CSRF-Token header (Same-Origin Policy blocks)
//   4. Server: headerToken = "" (missing)
//   5. Server: cookieToken = "abc123..." (from cookie)
//   6. Server: Comparison fails (empty ≠ "abc123...")
//   7. Server: 403 Forbidden - Attack blocked!
//
// SECURITY FEATURES:
//
// 1. Constant-Time Comparison:
//    - Uses subtle.ConstantTimeCompare instead of ==
//    - Prevents timing attacks
//    - Timing attack: measure comparison time to guess token byte-by-byte
//    - Constant-time: comparison always takes same time regardless of input
//
// 2. Token Expiration:
//    - Tokens expire after 24 hours
//    - Limits window for token theft
//    - Forces periodic token refresh
//
// 3. Cryptographically Secure Random:
//    - Uses crypto/rand (not math/rand)
//    - 256 bits of entropy
//    - Unpredictable and unique
//
// 4. HttpOnly Cookie:
//    - Cookie not accessible to JavaScript
//    - Prevents XSS attacks from stealing token
//
// 5. Secure Cookie (Production):
//    - Cookie only sent over HTTPS
//    - Prevents token theft via network sniffing
//
// USAGE:
//
//   router := gin.Default()
//   router.Use(middleware.CSRFProtection())
//
//   // All routes now protected:
//   router.GET("/api/sessions", handler)  // Generates token
//   router.POST("/api/sessions", handler) // Validates token
//
// CLIENT IMPLEMENTATION:
//
// JavaScript client must send token in header:
//
//   // Store token from first GET request
//   let csrfToken = null;
//
//   // GET request: capture token
//   const response = await fetch('/api/sessions');
//   csrfToken = response.headers.get('X-CSRF-Token');
//
//   // POST request: send token in header
//   await fetch('/api/delete-account', {
//     method: 'POST',
//     headers: {
//       'X-CSRF-Token': csrfToken,  // REQUIRED
//       'Content-Type': 'application/json',
//     },
//     credentials: 'include',  // Send cookies
//   });
//
// HTML FORM IMPLEMENTATION:
//
//   <!-- Store token in hidden field -->
//   <form method="POST" action="/api/delete-account">
//     <input type="hidden" name="csrf_token" value="{{ .CSRFToken }}">
//     <button type="submit">Delete Account</button>
//   </form>
//
// EXEMPT METHODS:
//
// GET, HEAD, OPTIONS are exempt because they are "safe methods":
// - SHOULD NOT modify server state (read-only)
// - Idempotent (can be repeated safely)
// - No CSRF risk if properly implemented
//
// IMPORTANT: If you use GET for state changes (antipattern), CSRF protection
// will NOT work. Always use POST/PUT/DELETE for state changes.
//
// LIMITATIONS:
//
// 1. Subdomain Attacks:
//    - If attacker controls subdomain (evil.example.com)
//    - They can set cookies for *.example.com
//    - Mitigation: Validate Origin/Referer headers (not implemented)
//
// 2. XSS Attacks:
//    - If site has XSS vulnerability
//    - Attacker can read token from headers
//    - Mitigation: Prevent XSS (input validation, CSP)
//
// 3. Token Storage:
//    - Tokens stored in memory (lost on restart)
//    - Mitigation: Use Redis for persistent storage
//
// COMMON ERRORS:
//
// "CSRF token missing":
//   - Client didn't send csrf_token cookie
//   - Solution: Ensure credentials: 'include' in fetch
//
// "CSRF token mismatch":
//   - Header token doesn't match cookie token
//   - Solution: Ensure X-CSRF-Token header is set correctly
//
// "CSRF token invalid":
//   - Token expired (>24 hours old)
//   - Server restarted (tokens lost)
//   - Solution: Refresh token by making GET request
func CSRFProtection() gin.HandlerFunc {
	// INITIALIZATION: Start cleanup goroutine once
	//
	// sync.Once ensures cleanup goroutine is started exactly once,
	// even if CSRFProtection is called multiple times.
	//
	// WHY ONCE: Prevents multiple cleanup goroutines from running
	// and competing for the same lock.
	csrfCleanupOnce.Do(func() {
		go globalCSRFStore.cleanup()
	})

	return func(c *gin.Context) {
		// EXEMPTION: JWT-Authenticated API Clients
		//
		// Skip CSRF validation for requests authenticated with JWT tokens.
		// JWT tokens provide sufficient authentication for programmatic API clients
		// (curl, scripts, CI/CD, integrations) and don't require CSRF protection.
		//
		// WHY THIS IS SAFE:
		//
		// CSRF attacks exploit the browser's automatic cookie-sending behavior.
		// JWT authentication requires clients to explicitly include the token in
		// the Authorization header, which attackers cannot do cross-origin.
		//
		// CSRF Attack Scenario (Session Cookies):
		//   1. User logs in → gets session cookie
		//   2. User visits evil.com
		//   3. evil.com: fetch('https://streamspace.io/api/delete', {method: 'POST'})
		//   4. Browser automatically sends session cookie
		//   5. Attack succeeds (without CSRF protection)
		//
		// JWT Attack Scenario (Bearer Tokens):
		//   1. User logs in → gets JWT token in response body
		//   2. User visits evil.com
		//   3. evil.com: fetch('https://streamspace.io/api/delete', {method: 'POST'})
		//   4. Browser does NOT send JWT (not in cookie, must be in header)
		//   5. Attack fails (no Authorization header)
		//
		// IMPORTANT: This exemption only applies to Bearer token authentication.
		// Session-based authentication (cookies) still requires CSRF protection.
		//
		// USE CASES:
		// - CLI tools (curl, httpie)
		// - CI/CD scripts (GitHub Actions, Jenkins)
		// - API integrations (Zapier, custom scripts)
		// - Mobile apps
		// - Server-to-server communication
		//
		// SECURITY CONSIDERATIONS:
		// - JWT tokens must be kept secure (not exposed in URLs or logs)
		// - Use HTTPS to prevent token interception
		// - Implement token expiration and refresh
		// - Validate JWT signature on every request
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			// Request has JWT token, skip CSRF validation
			c.Next()
			return
		}

		// BRANCH 1: SAFE METHODS (GET, HEAD, OPTIONS)
		//
		// These methods should not modify state, so we generate and send
		// a new CSRF token for use in subsequent state-changing requests.
		//
		// WHY EXEMPT: Safe methods are idempotent and read-only by HTTP specification.
		// They should not have side effects, so CSRF is not a risk.
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			// Use mutex to prevent race conditions with parallel GET requests
			// Without this, multiple simultaneous GETs could each generate a new token,
			// causing the cookie and JavaScript to have different tokens
			tokenGenerationMu.Lock()

			// Check if client already has a valid token
			// Reuse existing token to prevent token churn that causes mismatches
			existingToken, err := c.Cookie(CSRFCookieName)
			if err == nil && existingToken != "" && globalCSRFStore.validateToken(existingToken) {
				// Existing token is still valid, send it back in header
				tokenGenerationMu.Unlock()
				c.Header(CSRFTokenHeader, existingToken)
				c.Next()
				return
			}

			// STEP 1: Generate new CSRF token
			// Uses crypto/rand for cryptographic security
			token, err := generateCSRFToken()
			if err != nil {
				// CRITICAL ERROR: Cannot generate secure random
				// This indicates serious system issues (no entropy)
				// Do NOT proceed without CSRF protection
				tokenGenerationMu.Unlock()
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to generate CSRF token",
				})
				return
			}

			// STEP 2: Store token in server-side store
			// Required for validation in step 4 of state-changing requests
			globalCSRFStore.addToken(token)

			// STEP 3: Send token in response header
			// JavaScript clients read this header and store token
			c.Header(CSRFTokenHeader, token)

			// STEP 4: Send token in cookie
			// Browser automatically sends this cookie on subsequent requests
			//
			// Cookie parameters:
			// - Name: "csrf_token"
			// - Value: token (e.g., "abc123...")
			// - MaxAge: 86400 seconds (24 hours)
			// - Path: "/" (available to all endpoints)
			// - Domain: "" (current domain only)
			// - Secure: true in production (HTTPS-only), false in debug mode
			// - HttpOnly: true (not accessible to JavaScript - prevents XSS)

			// Determine if we should use secure cookies
			// In debug/development mode, allow HTTP for local testing
			secureCookie := gin.Mode() != gin.DebugMode

			c.SetCookie(
				CSRFCookieName,
				token,
				int(CSRFTokenExpiry.Seconds()),
				"/",
				"",
				secureCookie, // Secure: HTTPS-only in production, HTTP allowed in debug
				true,         // HttpOnly: JavaScript cannot access (XSS protection)
			)

			// Release lock after setting cookie so subsequent parallel requests use this token
			tokenGenerationMu.Unlock()

			// Continue to next handler
			c.Next()
			return
		}

		// BRANCH 2: STATE-CHANGING METHODS (POST, PUT, DELETE, PATCH)
		//
		// These methods modify server state, so we validate the CSRF token
		// to ensure the request is from a legitimate client, not a CSRF attack.

		// STEP 1: Get token from custom header
		// Legitimate clients set this header with JavaScript
		// Attackers cannot set this header due to Same-Origin Policy
		headerToken := c.GetHeader(CSRFTokenHeader)

		// STEP 2: Get token from cookie
		// Browser sends this automatically (even for cross-site requests)
		cookieToken, err := c.Cookie(CSRFCookieName)
		if err != nil {
			// Cookie not found: User never made a GET request to get token
			// OR: Cookie expired
			// OR: Browser blocked cookies
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "CSRF token missing",
				"message": "CSRF cookie not found",
			})
			return
		}

		// STEP 3: Compare tokens using constant-time comparison
		//
		// SECURITY: MUST use subtle.ConstantTimeCompare, NOT ==
		//
		// WHY CONSTANT-TIME:
		// Regular comparison (==) returns immediately on first mismatch:
		//   "abc123" == "xyz123"
		//        ^-- Mismatch at position 0, returns in ~1ns
		//   "abc123" == "abc999"
		//           ^-- Mismatch at position 3, returns in ~3ns
		//
		// Attacker can measure response time to guess token byte-by-byte:
		//   Try "a??????" → 1ns → correct first byte
		//   Try "b??????" → 0ns → incorrect first byte
		//   Try "ab?????" → 2ns → correct second byte
		//   ... repeat to recover entire token
		//
		// Constant-time comparison always takes same time:
		//   "abc123" == "xyz123" → always 6ns
		//   "abc123" == "abc999" → always 6ns
		//
		// Returns 1 if equal, 0 if not equal
		if subtle.ConstantTimeCompare([]byte(headerToken), []byte(cookieToken)) != 1 {
			// Tokens don't match: CSRF attack detected
			// OR: Client bug (not sending header correctly)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "CSRF token mismatch",
				"message": "CSRF tokens do not match",
			})
			return
		}

		// STEP 4: Validate token exists in store and is not expired
		// Even if tokens match, verify token was issued by this server
		// and has not expired (>24 hours old)
		if !globalCSRFStore.validateToken(cookieToken) {
			// Token expired or never existed
			// User needs to refresh token by making GET request
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "CSRF token invalid",
				"message": "CSRF token has expired or is invalid",
			})
			return
		}

		// All checks passed: Request is legitimate
		// Continue to next handler
		c.Next()
	}
}

// GetCSRFToken returns the current CSRF token for the request
// Useful for rendering in HTML forms or passing to frontend
func GetCSRFToken(c *gin.Context) string {
	return c.GetHeader(CSRFTokenHeader)
}

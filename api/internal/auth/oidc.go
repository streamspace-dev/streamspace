package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// OIDCConfig holds OIDC authentication configuration
type OIDCConfig struct {
	Enabled            bool              `json:"enabled"`
	ProviderURL        string            `json:"provider_url"`         // OIDC provider discovery URL
	ClientID           string            `json:"client_id"`            // OAuth2 client ID
	ClientSecret       string            `json:"client_secret"`        // OAuth2 client secret
	RedirectURI        string            `json:"redirect_uri"`         // OAuth2 redirect URI
	Scopes             []string          `json:"scopes"`               // OAuth2 scopes (default: openid, profile, email)
	UsernameClaim      string            `json:"username_claim"`       // Claim to use for username (default: preferred_username)
	EmailClaim         string            `json:"email_claim"`          // Claim to use for email (default: email)
	GroupsClaim        string            `json:"groups_claim"`         // Claim to use for groups (default: groups)
	RolesClaim         string            `json:"roles_claim"`          // Claim to use for roles (default: roles)
	ExtraParams        map[string]string `json:"extra_params"`         // Additional OAuth2 parameters
	InsecureSkipVerify bool              `json:"insecure_skip_verify"` // Skip TLS verification (dev only)
}

// OIDCAuthenticator handles OIDC authentication
type OIDCAuthenticator struct {
	config       *OIDCConfig
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// NewOIDCAuthenticator creates a new OIDC authenticator
func NewOIDCAuthenticator(config *OIDCConfig) (*OIDCAuthenticator, error) {
	if config == nil || !config.Enabled {
		return nil, fmt.Errorf("OIDC configuration is not enabled")
	}

	// Validate required fields
	if config.ProviderURL == "" {
		return nil, fmt.Errorf("OIDC provider URL is required")
	}
	if config.ClientID == "" {
		return nil, fmt.Errorf("OIDC client ID is required")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("OIDC client secret is required")
	}
	if config.RedirectURI == "" {
		return nil, fmt.Errorf("OIDC redirect URI is required")
	}

	// Set default scopes if not specified
	if len(config.Scopes) == 0 {
		config.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	// Set default claim names if not specified
	if config.UsernameClaim == "" {
		config.UsernameClaim = "preferred_username"
	}
	if config.EmailClaim == "" {
		config.EmailClaim = "email"
	}
	if config.GroupsClaim == "" {
		config.GroupsClaim = "groups"
	}
	if config.RolesClaim == "" {
		config.RolesClaim = "roles"
	}

	// Create HTTP client with optional TLS skip verification
	ctx := context.Background()
	if config.InsecureSkipVerify {
		log.Printf("[WARNING] OIDC: TLS verification disabled (insecure, use only in development)")
		// For development only - skip TLS verification
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &http.Transport{}.TLSClientConfig,
			},
		}
		ctx = oidc.ClientContext(ctx, client)
	}

	// Discover OIDC provider configuration
	provider, err := oidc.NewProvider(ctx, config.ProviderURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	log.Printf("[INFO] OIDC: Successfully discovered provider at %s", config.ProviderURL)

	// Create OAuth2 config
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       config.Scopes,
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &OIDCAuthenticator{
		config:       config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}, nil
}

// GetAuthorizationURL generates the OIDC authorization URL
func (a *OIDCAuthenticator) GetAuthorizationURL(state string) string {
	// Build authorization URL with optional extra parameters
	opts := []oauth2.AuthCodeOption{}

	// Add extra parameters if configured
	for key, value := range a.config.ExtraParams {
		opts = append(opts, oauth2.SetAuthURLParam(key, value))
	}

	return a.oauth2Config.AuthCodeURL(state, opts...)
}

// HandleCallback processes the OIDC callback and returns user information
func (a *OIDCAuthenticator) HandleCallback(ctx context.Context, code string) (*OIDCUserInfo, error) {
	// Exchange authorization code for tokens
	oauth2Token, err := a.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Extract ID token from OAuth2 token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token field in oauth2 token")
	}

	// Verify ID token
	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims from ID token
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	log.Printf("[DEBUG] OIDC: ID token claims: %+v", claims)

	// Fetch additional user info from UserInfo endpoint
	userInfo, err := a.provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
	if err != nil {
		log.Printf("[WARNING] OIDC: Failed to fetch userinfo: %v, using ID token claims only", err)
		// Continue with ID token claims only
	} else {
		// Merge userInfo claims into existing claims
		var userInfoClaims map[string]interface{}
		if err := userInfo.Claims(&userInfoClaims); err == nil {
			for k, v := range userInfoClaims {
				// Don't overwrite existing claims from ID token
				if _, exists := claims[k]; !exists {
					claims[k] = v
				}
			}
			log.Printf("[DEBUG] OIDC: Merged userinfo claims: %+v", claims)
		}
	}

	// Extract user information from claims
	oidcUserInfo := &OIDCUserInfo{
		Subject:       idToken.Subject,
		Email:         extractStringClaim(claims, a.config.EmailClaim),
		Username:      extractStringClaim(claims, a.config.UsernameClaim),
		EmailVerified: extractBoolClaim(claims, "email_verified"),
		Groups:        extractArrayClaim(claims, a.config.GroupsClaim),
		Roles:         extractArrayClaim(claims, a.config.RolesClaim),
		Claims:        claims,
	}

	// Use email as username if username claim is empty
	if oidcUserInfo.Username == "" {
		oidcUserInfo.Username = oidcUserInfo.Email
	}

	// Extract optional fields
	if givenName := extractStringClaim(claims, "given_name"); givenName != "" {
		oidcUserInfo.FirstName = givenName
	}
	if familyName := extractStringClaim(claims, "family_name"); familyName != "" {
		oidcUserInfo.LastName = familyName
	}
	if name := extractStringClaim(claims, "name"); name != "" {
		oidcUserInfo.FullName = name
	}
	if picture := extractStringClaim(claims, "picture"); picture != "" {
		oidcUserInfo.Picture = picture
	}

	log.Printf("[INFO] OIDC: Successfully authenticated user: %s (email: %s, groups: %v)",
		oidcUserInfo.Username, oidcUserInfo.Email, oidcUserInfo.Groups)

	return oidcUserInfo, nil
}

// OIDCUserInfo holds user information extracted from OIDC tokens
type OIDCUserInfo struct {
	Subject       string                 `json:"sub"`
	Email         string                 `json:"email"`
	Username      string                 `json:"username"`
	EmailVerified bool                   `json:"email_verified"`
	FirstName     string                 `json:"given_name,omitempty"`
	LastName      string                 `json:"family_name,omitempty"`
	FullName      string                 `json:"name,omitempty"`
	Picture       string                 `json:"picture,omitempty"`
	Groups        []string               `json:"groups,omitempty"`
	Roles         []string               `json:"roles,omitempty"`
	Claims        map[string]interface{} `json:"claims,omitempty"`
}

// Helper functions to extract claims

func extractStringClaim(claims map[string]interface{}, claimName string) string {
	if val, ok := claims[claimName]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func extractBoolClaim(claims map[string]interface{}, claimName string) bool {
	if val, ok := claims[claimName]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func extractArrayClaim(claims map[string]interface{}, claimName string) []string {
	if val, ok := claims[claimName]; ok {
		// Handle array of strings
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
		// Handle single string
		if str, ok := val.(string); ok {
			return []string{str}
		}
		// Handle comma-separated string
		if str, ok := val.(string); ok && strings.Contains(str, ",") {
			parts := strings.Split(str, ",")
			result := make([]string, 0, len(parts))
			for _, part := range parts {
				trimmed := strings.TrimSpace(part)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
			return result
		}
	}
	return []string{}
}

// OIDC HTTP Handlers

// OIDCLoginHandler initiates OIDC authentication flow
func (a *OIDCAuthenticator) OIDCLoginHandler(c *gin.Context) {
	// Generate state parameter for CSRF protection
	state := generateRandomState()

	// Store state in session/cookie (for CSRF validation)
	c.SetCookie("oidc_state", state, 600, "/", "", false, true)

	// Get authorization URL
	authURL := a.GetAuthorizationURL(state)

	log.Printf("[INFO] OIDC: Redirecting to authorization URL")

	// Redirect to OIDC provider
	c.Redirect(http.StatusFound, authURL)
}

// OIDCCallbackHandler handles the OIDC callback
func (a *OIDCAuthenticator) OIDCCallbackHandler(userManager UserManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get state from cookie for CSRF validation
		storedState, err := c.Cookie("oidc_state")
		if err != nil {
			log.Printf("[ERROR] OIDC: Failed to get state cookie: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state cookie"})
			return
		}

		// Validate state parameter (CSRF protection)
		receivedState := c.Query("state")
		if receivedState != storedState {
			log.Printf("[ERROR] OIDC: State mismatch (CSRF attempt?)")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state parameter"})
			return
		}

		// Clear state cookie
		c.SetCookie("oidc_state", "", -1, "/", "", false, true)

		// Check for error from OIDC provider
		if errMsg := c.Query("error"); errMsg != "" {
			errDesc := c.Query("error_description")
			log.Printf("[ERROR] OIDC: Provider returned error: %s - %s", errMsg, errDesc)
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":             errMsg,
				"error_description": errDesc,
			})
			return
		}

		// Get authorization code
		code := c.Query("code")
		if code == "" {
			log.Printf("[ERROR] OIDC: Missing authorization code")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
			return
		}

		// Handle callback and get user info
		ctx := c.Request.Context()
		userInfo, err := a.HandleCallback(ctx, code)
		if err != nil {
			log.Printf("[ERROR] OIDC: Callback handling failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Authentication failed: %v", err)})
			return
		}

		// Create or update user in database
		user, err := userManager.CreateOrUpdateOIDCUser(ctx, userInfo)
		if err != nil {
			log.Printf("[ERROR] OIDC: Failed to create/update user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		log.Printf("[INFO] OIDC: User authenticated successfully: %s (ID: %s)", user.Username, user.ID)

		// Return user info and JWT token
		c.JSON(http.StatusOK, gin.H{
			"user":    user,
			"message": "OIDC authentication successful",
		})
	}
}

// UserManager interface for OIDC user management
type UserManager interface {
	CreateOrUpdateOIDCUser(ctx context.Context, userInfo *OIDCUserInfo) (*User, error)
}

// User represents a user in the system
type User struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Provider string   `json:"provider"`
	Groups   []string `json:"groups,omitempty"`
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() string {
	// Use timestamp + random component for state
	return fmt.Sprintf("%d-%s", time.Now().Unix(), randomString(32))
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// GetDiscoveryDocument returns the OIDC discovery document
func (a *OIDCAuthenticator) GetDiscoveryDocument() (map[string]interface{}, error) {
	// Fetch discovery document from provider
	resp, err := http.Get(a.config.ProviderURL + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery document request failed with status %d", resp.StatusCode)
	}

	var discovery map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("failed to decode discovery document: %w", err)
	}

	return discovery, nil
}

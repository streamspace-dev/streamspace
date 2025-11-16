package main

import ("context"; "encoding/json"; "fmt"; "github.com/yourusername/streamspace/api/internal/plugins"; "github.com/coreos/go-oidc/v3/oidc"; "golang.org/x/oauth2")

type OAuthPlugin struct {
	plugins.BasePlugin
	config OAuthConfig
	provider *oidc.Provider
	oauth2Config *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

type OAuthConfig struct {
	Enabled            bool              `json:"enabled"`
	Provider           string            `json:"provider"`
	ProviderURL        string            `json:"providerURL"`
	ClientID           string            `json:"clientID"`
	ClientSecret       string            `json:"clientSecret"`
	RedirectURI        string            `json:"redirectURI"`
	Scopes             []string          `json:"scopes"`
	UsernameClaim      string            `json:"usernameClaim"`
	EmailClaim         string            `json:"emailClaim"`
	GroupsClaim        string            `json:"groupsClaim"`
	AutoProvisionUsers bool              `json:"autoProvisionUsers"`
	DefaultRole        string            `json:"defaultRole"`
}

func (p *OAuthPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("OAuth authentication is disabled")
		return nil
	}

	// Set defaults
	if len(p.config.Scopes) == 0 {
		p.config.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if p.config.UsernameClaim == "" {
		p.config.UsernameClaim = "preferred_username"
	}
	if p.config.EmailClaim == "" {
		p.config.EmailClaim = "email"
	}

	// Discover OIDC provider
	provider, err := oidc.NewProvider(context.Background(), p.config.ProviderURL)
	if err != nil {
		return fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	// Create OAuth2 config
	oauth2Config := &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,
		RedirectURL:  p.config.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       p.config.Scopes,
	}

	// Create ID token verifier
	verifier := provider.Verifier(&oidc.Config{
		ClientID: p.config.ClientID,
	})

	p.provider = provider
	p.oauth2Config = oauth2Config
	p.verifier = verifier

	ctx.Logger.Info("OAuth authentication initialized", "provider", p.config.Provider, "providerURL", p.config.ProviderURL)
	return nil
}

func (p *OAuthPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("OAuth Authentication plugin loaded")
	return nil
}

func (p *OAuthPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	userMap, _ := user.(map[string]interface{})
	authMethod := userMap["auth_method"]
	if authMethod == "oauth" || authMethod == "oidc" {
		ctx.Logger.Info("OAuth user login", "user", userMap["username"], "provider", p.config.Provider)
	}
	return nil
}

// GetAuthorizationURL generates the OAuth authorization URL
func (p *OAuthPlugin) GetAuthorizationURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// HandleCallback processes the OAuth callback
func (p *OAuthPlugin) HandleCallback(ctx context.Context, code string) (map[string]interface{}, error) {
	// Exchange authorization code for tokens
	oauth2Token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Extract ID token
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token field in oauth2 token")
	}

	// Verify ID token
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract claims
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse ID token claims: %w", err)
	}

	// Build user info
	user := map[string]interface{}{
		"auth_method": "oauth",
		"provider":    p.config.Provider,
		"subject":     idToken.Subject,
		"email":       extractClaim(claims, p.config.EmailClaim),
		"username":    extractClaim(claims, p.config.UsernameClaim),
		"groups":      extractArrayClaim(claims, p.config.GroupsClaim),
		"claims":      claims,
	}

	// Use email as username if username is empty
	if user["username"] == "" {
		user["username"] = user["email"]
	}

	// Set default role if auto-provisioning
	if p.config.AutoProvisionUsers {
		user["role"] = p.config.DefaultRole
	}

	return user, nil
}

func extractClaim(claims map[string]interface{}, key string) string {
	if val, ok := claims[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func extractArrayClaim(claims map[string]interface{}, key string) []string {
	if val, ok := claims[key]; ok {
		if arr, ok := val.([]interface{}); ok {
			result := make([]string, len(arr))
			for i, v := range arr {
				if str, ok := v.(string); ok {
					result[i] = str
				}
			}
			return result
		}
	}
	return []string{}
}

func init() {
	plugins.Register("streamspace-auth-oauth", &OAuthPlugin{})
}

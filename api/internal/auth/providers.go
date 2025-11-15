package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
)

// SAMLProvider represents a SAML identity provider configuration
type SAMLProvider string

const (
	// Supported SAML providers
	ProviderOkta            SAMLProvider = "okta"
	ProviderAzureAD         SAMLProvider = "azuread"
	ProviderGoogleWorkspace SAMLProvider = "google"
	ProviderAuth0           SAMLProvider = "auth0"
	ProviderKeycloak        SAMLProvider = "keycloak"
	ProviderAuthentik       SAMLProvider = "authentik"
	ProviderGeneric         SAMLProvider = "generic"
)

// OIDCProvider represents an OIDC identity provider configuration
type OIDCProvider string

const (
	// Supported OIDC providers
	OIDCProviderKeycloak OIDCProvider = "keycloak"
	OIDCProviderOkta     OIDCProvider = "okta"
	OIDCProviderAuth0    OIDCProvider = "auth0"
	OIDCProviderGoogle   OIDCProvider = "google"
	OIDCProviderAzureAD  OIDCProvider = "azuread"
	OIDCProviderGitHub   OIDCProvider = "github"
	OIDCProviderGitLab   OIDCProvider = "gitlab"
	OIDCProviderGeneric  OIDCProvider = "generic"
)

// ProviderConfig holds provider-specific configuration templates
type ProviderConfig struct {
	Provider            SAMLProvider
	DefaultMapping      AttributeMapping
	MetadataURLTemplate string
}

// GetProviderConfig returns the configuration template for a provider
func GetProviderConfig(provider SAMLProvider) *ProviderConfig {
	configs := map[SAMLProvider]*ProviderConfig{
		ProviderOkta: {
			Provider: ProviderOkta,
			DefaultMapping: AttributeMapping{
				Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
				LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
				Groups:    "groups",
			},
			MetadataURLTemplate: "https://{domain}/app/{app_id}/sso/saml/metadata",
		},
		ProviderAzureAD: {
			Provider: ProviderAzureAD,
			DefaultMapping: AttributeMapping{
				Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
				LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
				Groups:    "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
			},
			MetadataURLTemplate: "https://login.microsoftonline.com/{tenant_id}/federationmetadata/2007-06/federationmetadata.xml",
		},
		ProviderGoogleWorkspace: {
			Provider: ProviderGoogleWorkspace,
			DefaultMapping: AttributeMapping{
				Email:     "email",
				Username:  "email",
				FirstName: "firstName",
				LastName:  "lastName",
				Groups:    "groups",
			},
			MetadataURLTemplate: "https://accounts.google.com/o/saml2/idp?idpid={idp_id}",
		},
		ProviderAuth0: {
			Provider: ProviderAuth0,
			DefaultMapping: AttributeMapping{
				Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
				LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
				Groups:    "groups",
			},
			MetadataURLTemplate: "https://{domain}/samlp/metadata/{client_id}",
		},
		ProviderKeycloak: {
			Provider: ProviderKeycloak,
			DefaultMapping: AttributeMapping{
				Email:     "email",
				Username:  "username",
				FirstName: "firstName",
				LastName:  "lastName",
				Groups:    "groups",
			},
			MetadataURLTemplate: "https://{domain}/auth/realms/{realm}/protocol/saml/descriptor",
		},
		ProviderAuthentik: {
			Provider: ProviderAuthentik,
			DefaultMapping: AttributeMapping{
				Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
				Username:  "http://schemas.goauthentik.io/2021/02/saml/username",
				FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
				Groups:    "http://schemas.xmlsoap.org/claims/Group",
			},
			MetadataURLTemplate: "https://{domain}/application/saml/{slug}/metadata/",
		},
		ProviderGeneric: {
			Provider: ProviderGeneric,
			DefaultMapping: AttributeMapping{
				Email:     "email",
				Username:  "username",
				FirstName: "firstName",
				LastName:  "lastName",
				Groups:    "groups",
			},
			MetadataURLTemplate: "",
		},
	}

	if config, ok := configs[provider]; ok {
		return config
	}
	return configs[ProviderGeneric]
}

// OIDCProviderConfig holds OIDC provider-specific configuration templates
type OIDCProviderConfig struct {
	Provider      OIDCProvider
	DiscoveryURL  string   // Well-known discovery URL
	DefaultScopes []string // Default OAuth2 scopes
	UsernameClaim string   // Default username claim
	EmailClaim    string   // Default email claim
	GroupsClaim   string   // Default groups claim
}

// GetOIDCProviderConfig returns the configuration template for an OIDC provider
func GetOIDCProviderConfig(provider OIDCProvider) *OIDCProviderConfig {
	configs := map[OIDCProvider]*OIDCProviderConfig{
		OIDCProviderKeycloak: {
			Provider:      OIDCProviderKeycloak,
			DiscoveryURL:  "https://{domain}/auth/realms/{realm}", // Will append /.well-known/openid-configuration
			DefaultScopes: []string{"openid", "profile", "email", "groups"},
			UsernameClaim: "preferred_username",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
		OIDCProviderOkta: {
			Provider:      OIDCProviderOkta,
			DiscoveryURL:  "https://{domain}/oauth2/default",
			DefaultScopes: []string{"openid", "profile", "email", "groups"},
			UsernameClaim: "preferred_username",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
		OIDCProviderAuth0: {
			Provider:      OIDCProviderAuth0,
			DiscoveryURL:  "https://{domain}",
			DefaultScopes: []string{"openid", "profile", "email"},
			UsernameClaim: "nickname",
			EmailClaim:    "email",
			GroupsClaim:   "https://{domain}/claims/groups",
		},
		OIDCProviderGoogle: {
			Provider:      OIDCProviderGoogle,
			DiscoveryURL:  "https://accounts.google.com",
			DefaultScopes: []string{"openid", "profile", "email"},
			UsernameClaim: "email",
			EmailClaim:    "email",
			GroupsClaim:   "groups", // Google Workspace only
		},
		OIDCProviderAzureAD: {
			Provider:      OIDCProviderAzureAD,
			DiscoveryURL:  "https://login.microsoftonline.com/{tenant}/v2.0",
			DefaultScopes: []string{"openid", "profile", "email"},
			UsernameClaim: "preferred_username",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
		OIDCProviderGitHub: {
			Provider:      OIDCProviderGitHub,
			DiscoveryURL:  "https://github.com", // GitHub doesn't fully support OIDC discovery
			DefaultScopes: []string{"read:user", "user:email"},
			UsernameClaim: "login",
			EmailClaim:    "email",
			GroupsClaim:   "orgs", // GitHub uses "orgs" for organization membership
		},
		OIDCProviderGitLab: {
			Provider:      OIDCProviderGitLab,
			DiscoveryURL:  "https://gitlab.com",
			DefaultScopes: []string{"openid", "profile", "email"},
			UsernameClaim: "nickname",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
		OIDCProviderGeneric: {
			Provider:      OIDCProviderGeneric,
			DiscoveryURL:  "", // Must be provided by user
			DefaultScopes: []string{"openid", "profile", "email"},
			UsernameClaim: "preferred_username",
			EmailClaim:    "email",
			GroupsClaim:   "groups",
		},
	}

	if config, ok := configs[provider]; ok {
		return config
	}
	return configs[OIDCProviderGeneric]
}

// LoadCertificate loads an X.509 certificate from PEM file
func LoadCertificate(certPath string) (*x509.Certificate, error) {
	certPEM, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// LoadPrivateKey loads an RSA private key from PEM file
func LoadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	keyPEM, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not an RSA private key")
		}
	}

	return key, nil
}

// AuthConfig represents the complete authentication configuration
type AuthConfig struct {
	// JWT configuration (existing)
	JWTSecret     string
	JWTExpiration int

	// SAML configuration
	SAML *SAMLConfig

	// OIDC configuration
	OIDC *OIDCConfig

	// Authentication mode
	Mode AuthMode
}

// AuthMode defines the authentication mode
type AuthMode string

const (
	AuthModeJWT    AuthMode = "jwt"    // JWT only (default)
	AuthModeSAML   AuthMode = "saml"   // SAML only
	AuthModeHybrid AuthMode = "hybrid" // Both JWT and SAML
	AuthModeOIDC   AuthMode = "oidc"   // OIDC authentication
)

// ValidateConfig validates the authentication configuration
func ValidateConfig(config *AuthConfig) error {
	if config.Mode == "" {
		config.Mode = AuthModeJWT
	}

	switch config.Mode {
	case AuthModeJWT:
		if config.JWTSecret == "" {
			return fmt.Errorf("JWT secret is required for JWT mode")
		}
	case AuthModeSAML:
		if config.SAML == nil || !config.SAML.Enabled {
			return fmt.Errorf("SAML configuration is required for SAML mode")
		}
	case AuthModeHybrid:
		if config.JWTSecret == "" {
			return fmt.Errorf("JWT secret is required for hybrid mode")
		}
		if config.SAML == nil || !config.SAML.Enabled {
			return fmt.Errorf("SAML configuration is required for hybrid mode")
		}
	case AuthModeOIDC:
		if config.OIDC == nil || !config.OIDC.Enabled {
			return fmt.Errorf("OIDC configuration is required for OIDC mode")
		}
		// Validate OIDC configuration
		if config.OIDC.ProviderURL == "" {
			return fmt.Errorf("OIDC provider URL is required")
		}
		if config.OIDC.ClientID == "" {
			return fmt.Errorf("OIDC client ID is required")
		}
		if config.OIDC.ClientSecret == "" {
			return fmt.Errorf("OIDC client secret is required")
		}
		if config.OIDC.RedirectURI == "" {
			return fmt.Errorf("OIDC redirect URI is required")
		}
	default:
		return fmt.Errorf("invalid authentication mode: %s", config.Mode)
	}

	return nil
}

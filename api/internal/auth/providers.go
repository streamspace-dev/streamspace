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
	ProviderOkta         SAMLProvider = "okta"
	ProviderAzureAD      SAMLProvider = "azuread"
	ProviderGoogleWorkspace SAMLProvider = "google"
	ProviderAuth0        SAMLProvider = "auth0"
	ProviderKeycloak     SAMLProvider = "keycloak"
	ProviderAuthentik    SAMLProvider = "authentik"
	ProviderGeneric      SAMLProvider = "generic"
)

// ProviderConfig holds provider-specific configuration templates
type ProviderConfig struct {
	Provider         SAMLProvider
	DefaultMapping   AttributeMapping
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

	// Authentication mode
	Mode AuthMode
}

// AuthMode defines the authentication mode
type AuthMode string

const (
	AuthModeJWT       AuthMode = "jwt"        // JWT only (default)
	AuthModeSAML      AuthMode = "saml"       // SAML only
	AuthModeHybrid    AuthMode = "hybrid"     // Both JWT and SAML
	AuthModeOIDC      AuthMode = "oidc"       // OIDC (future)
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
		return fmt.Errorf("OIDC mode is not yet implemented")
	default:
		return fmt.Errorf("invalid authentication mode: %s", config.Mode)
	}

	return nil
}

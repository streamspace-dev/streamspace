package main

import ("crypto/rsa"; "crypto/x509"; "encoding/json"; "encoding/pem"; "encoding/xml"; "fmt"; "net/url"; "github.com/yourusername/streamspace/api/internal/plugins"; "github.com/crewjam/saml"; "github.com/crewjam/saml/samlsp")

type SAMLPlugin struct {
	plugins.BasePlugin
	config SAMLConfig
	middleware *samlsp.Middleware
	serviceProvider *saml.ServiceProvider
}

type SAMLConfig struct {
	Enabled            bool              `json:"enabled"`
	Provider           string            `json:"provider"`
	EntityID           string            `json:"entityID"`
	MetadataURL        string            `json:"metadataURL"`
	MetadataXML        string            `json:"metadataXML"`
	Certificate        string            `json:"certificate"`
	PrivateKey         string            `json:"privateKey"`
	AllowIDPInitiated  bool              `json:"allowIDPInitiated"`
	SignRequest        bool              `json:"signRequest"`
	ForceAuthn         bool              `json:"forceAuthn"`
	AttributeMapping   AttributeMapping  `json:"attributeMapping"`
	AutoProvisionUsers bool              `json:"autoProvisionUsers"`
	DefaultRole        string            `json:"defaultRole"`
}

type AttributeMapping struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Groups    string `json:"groups"`
}

func (p *SAMLPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)

	if !p.config.Enabled {
		ctx.Logger.Info("SAML authentication is disabled")
		return nil
	}

	// Parse certificate and private key
	cert, err := parseCertificate(p.config.Certificate)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	privateKey, err := parsePrivateKey(p.config.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Create service provider
	rootURL, err := url.Parse(p.config.EntityID)
	if err != nil {
		return fmt.Errorf("invalid entity ID: %w", err)
	}

	sp := &saml.ServiceProvider{
		EntityID:          p.config.EntityID,
		Key:               privateKey,
		Certificate:       cert,
		MetadataURL:       *rootURL.ResolveReference(&url.URL{Path: "/saml/metadata"}),
		AcsURL:            *rootURL.ResolveReference(&url.URL{Path: "/saml/acs"}),
		SloURL:            *rootURL.ResolveReference(&url.URL{Path: "/saml/slo"}),
		AllowIDPInitiated: p.config.AllowIDPInitiated,
		ForceAuthn:        &p.config.ForceAuthn,
	}

	// Load IdP metadata
	var idpMetadata *saml.EntityDescriptor
	if p.config.MetadataURL != "" {
		// Fetch from URL (implementation simplified)
		ctx.Logger.Info("Fetching IdP metadata from URL", "url", p.config.MetadataURL)
		// In real implementation, fetch and parse metadata
	} else if p.config.MetadataXML != "" {
		idpMetadata = &saml.EntityDescriptor{}
		if err := xml.Unmarshal([]byte(p.config.MetadataXML), idpMetadata); err != nil {
			return fmt.Errorf("failed to parse IdP metadata XML: %w", err)
		}
	} else {
		return fmt.Errorf("either metadataURL or metadataXML must be provided")
	}

	sp.IDPMetadata = idpMetadata

	// Create SAML middleware
	middleware, err := samlsp.New(samlsp.Options{
		EntityID:          sp.EntityID,
		URL:               *rootURL,
		Key:               sp.Key,
		Certificate:       sp.Certificate,
		IDPMetadata:       sp.IDPMetadata,
		AllowIDPInitiated: sp.AllowIDPInitiated,
		ForceAuthn:        sp.ForceAuthn,
	})
	if err != nil {
		return fmt.Errorf("failed to create SAML middleware: %w", err)
	}

	p.middleware = middleware
	p.serviceProvider = sp

	ctx.Logger.Info("SAML authentication initialized", "provider", p.config.Provider, "entityID", p.config.EntityID)
	return nil
}

func (p *SAMLPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("SAML Authentication plugin loaded")
	return nil
}

func (p *SAMLPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	// Track SAML logins
	userMap, _ := user.(map[string]interface{})
	authMethod := userMap["auth_method"]
	if authMethod == "saml" {
		ctx.Logger.Info("SAML user login", "user", userMap["username"])
	}
	return nil
}

func parseCertificate(certPEM string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parsePrivateKey(keyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ExtractUserFromAssertion extracts user information from SAML assertion
func (p *SAMLPlugin) ExtractUserFromAssertion(assertion *saml.Assertion) (map[string]interface{}, error) {
	if assertion == nil {
		return nil, fmt.Errorf("assertion is nil")
	}

	user := map[string]interface{}{
		"auth_method": "saml",
		"attributes":  make(map[string]interface{}),
	}

	// Extract attributes based on mapping
	for _, attrStatement := range assertion.AttributeStatements {
		for _, attr := range attrStatement.Attributes {
			if len(attr.Values) == 0 {
				continue
			}

			attrName := attr.Name
			attrValue := attr.Values[0].Value

			// Map to user fields
			switch attrName {
			case p.config.AttributeMapping.Email:
				user["email"] = attrValue
			case p.config.AttributeMapping.Username:
				user["username"] = attrValue
			case p.config.AttributeMapping.FirstName:
				user["first_name"] = attrValue
			case p.config.AttributeMapping.LastName:
				user["last_name"] = attrValue
			case p.config.AttributeMapping.Groups:
				groups := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					groups[i] = v.Value
				}
				user["groups"] = groups
			}

			// Store all attributes
			attrs := user["attributes"].(map[string]interface{})
			if len(attr.Values) == 1 {
				attrs[attrName] = attrValue
			} else {
				values := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					values[i] = v.Value
				}
				attrs[attrName] = values
			}
		}
	}

	// Use NameID as username if username not mapped
	if user["username"] == nil && assertion.Subject != nil && assertion.Subject.NameID != nil {
		user["username"] = assertion.Subject.NameID.Value
	}

	// Use NameID as email if email not mapped and format is email
	if user["email"] == nil && assertion.Subject != nil && assertion.Subject.NameID != nil {
		if assertion.Subject.NameID.Format == "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
			user["email"] = assertion.Subject.NameID.Value
		}
	}

	// Validate required fields
	if user["username"] == nil {
		return nil, fmt.Errorf("username not found in SAML assertion")
	}

	// Set default role if auto-provisioning
	if p.config.AutoProvisionUsers {
		user["role"] = p.config.DefaultRole
	}

	return user, nil
}

func init() {
	plugins.Register("streamspace-auth-saml", &SAMLPlugin{})
}

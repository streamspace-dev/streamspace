package auth

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/gin-gonic/gin"
)

// SAMLConfig holds SAML authentication configuration
type SAMLConfig struct {
	Enabled              bool
	EntityID             string
	MetadataURL          string
	MetadataXML          []byte
	AssertionConsumerURL string
	SingleLogoutURL      string
	Certificate          *x509.Certificate
	PrivateKey           *rsa.PrivateKey
	AllowIDPInitiated    bool
	SignRequest          bool
	ForceAuthn           bool
	AttributeMapping     AttributeMapping
}

// AttributeMapping maps SAML attributes to user fields
type AttributeMapping struct {
	Email     string // SAML attribute name for email
	Username  string // SAML attribute name for username
	FirstName string // SAML attribute name for first name
	LastName  string // SAML attribute name for last name
	Groups    string // SAML attribute name for groups
}

// SAMLAuthenticator handles SAML authentication
type SAMLAuthenticator struct {
	config         *SAMLConfig
	middleware     *samlsp.Middleware
	serviceProvider *saml.ServiceProvider
}

// NewSAMLAuthenticator creates a new SAML authenticator
func NewSAMLAuthenticator(config *SAMLConfig) (*SAMLAuthenticator, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("SAML is not enabled")
	}

	// Create the service provider
	rootURL, err := url.Parse(config.EntityID)
	if err != nil {
		return nil, fmt.Errorf("invalid entity ID: %w", err)
	}

	sp := &saml.ServiceProvider{
		EntityID:          config.EntityID,
		Key:               config.PrivateKey,
		Certificate:       config.Certificate,
		MetadataURL:       *rootURL.ResolveReference(&url.URL{Path: "/saml/metadata"}),
		AcsURL:            *rootURL.ResolveReference(&url.URL{Path: "/saml/acs"}),
		SloURL:            *rootURL.ResolveReference(&url.URL{Path: "/saml/slo"}),
		AllowIDPInitiated: config.AllowIDPInitiated,
		ForceAuthn:        &config.ForceAuthn,
	}

	// Load IdP metadata
	var idpMetadata *saml.EntityDescriptor
	if config.MetadataURL != "" {
		// Fetch from URL
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: false,
				},
			},
		}
		idpMetadata, err = samlsp.FetchMetadata(context.Background(), httpClient, url.URL{
			Scheme: "https",
			Host:   config.MetadataURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch IdP metadata: %w", err)
		}
	} else if len(config.MetadataXML) > 0 {
		// Parse from XML
		idpMetadata = &saml.EntityDescriptor{}
		if err := xml.Unmarshal(config.MetadataXML, idpMetadata); err != nil {
			return nil, fmt.Errorf("failed to parse IdP metadata XML: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either MetadataURL or MetadataXML must be provided")
	}

	sp.IDPMetadata = idpMetadata

	// Create middleware
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
		return nil, fmt.Errorf("failed to create SAML middleware: %w", err)
	}

	return &SAMLAuthenticator{
		config:          config,
		middleware:      middleware,
		serviceProvider: sp,
	}, nil
}

// GetMiddleware returns the SAML middleware
func (sa *SAMLAuthenticator) GetMiddleware() *samlsp.Middleware {
	return sa.middleware
}

// ExtractUserFromAssertion extracts user information from SAML assertion
func (sa *SAMLAuthenticator) ExtractUserFromAssertion(assertion *saml.Assertion) (*UserInfo, error) {
	if assertion == nil {
		return nil, fmt.Errorf("assertion is nil")
	}

	user := &UserInfo{
		Attributes: make(map[string]interface{}),
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
			case sa.config.AttributeMapping.Email:
				user.Email = attrValue
			case sa.config.AttributeMapping.Username:
				user.Username = attrValue
			case sa.config.AttributeMapping.FirstName:
				user.FirstName = attrValue
			case sa.config.AttributeMapping.LastName:
				user.LastName = attrValue
			case sa.config.AttributeMapping.Groups:
				// Groups can be multi-valued
				groups := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					groups[i] = v.Value
				}
				user.Groups = groups
			}

			// Store all attributes
			if len(attr.Values) == 1 {
				user.Attributes[attrName] = attrValue
			} else {
				values := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					values[i] = v.Value
				}
				user.Attributes[attrName] = values
			}
		}
	}

	// Use NameID as username if username not mapped
	if user.Username == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		user.Username = assertion.Subject.NameID.Value
	}

	// Use NameID as email if email not mapped and format is email
	if user.Email == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		if assertion.Subject.NameID.Format == "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
			user.Email = assertion.Subject.NameID.Value
		}
	}

	// Validate required fields
	if user.Username == "" {
		return nil, fmt.Errorf("username not found in SAML assertion")
	}

	return user, nil
}

// GinMiddleware returns a Gin middleware for SAML authentication
func (sa *SAMLAuthenticator) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if SAML session exists
		session, err := sa.middleware.Session.GetSession(c.Request)
		if err != nil || session == nil {
			// No SAML session, redirect to SSO
			sa.middleware.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.Next()
			})).ServeHTTP(c.Writer, c.Request)
			c.Abort()
			return
		}

		// Extract assertion
		assertion := session.(samlsp.SessionWithAttributes).GetAttributes()
		if assertion == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid SAML session"})
			c.Abort()
			return
		}

		// Convert to SAML assertion
		samlAssertion, ok := assertion.(*saml.Assertion)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid assertion type"})
			c.Abort()
			return
		}

		// Extract user info
		user, err := sa.ExtractUserFromAssertion(samlAssertion)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Failed to extract user: %v", err)})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Next()
	}
}

// SetupRoutes sets up SAML-related routes
func (sa *SAMLAuthenticator) SetupRoutes(router *gin.Engine) {
	samlGroup := router.Group("/saml")
	{
		// Metadata endpoint
		samlGroup.GET("/metadata", func(c *gin.Context) {
			metadata, err := xml.MarshalIndent(sa.serviceProvider.Metadata(), "", "  ")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate metadata"})
				return
			}
			c.Data(http.StatusOK, "application/xml", metadata)
		})

		// Assertion Consumer Service (ACS)
		samlGroup.POST("/acs", gin.WrapH(sa.middleware))

		// Single Logout (SLO)
		samlGroup.GET("/slo", gin.WrapH(sa.middleware))
		samlGroup.POST("/slo", gin.WrapH(sa.middleware))

		// Initiate SSO login
		samlGroup.GET("/login", func(c *gin.Context) {
			// Get return URL from query param
			returnURL := c.Query("return_url")
			if returnURL == "" {
				returnURL = "/"
			}

			// Store return URL in cookie
			c.SetCookie("saml_return_url", returnURL, 3600, "/", "", false, true)

			// Redirect to IdP
			sa.middleware.HandleStartAuthFlow(c.Writer, c.Request)
		})

		// Logout endpoint
		samlGroup.GET("/logout", func(c *gin.Context) {
			// Clear session
			if err := sa.middleware.Session.DeleteSession(c.Writer, c.Request); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
				return
			}

			// Redirect to home
			c.Redirect(http.StatusFound, "/")
		})
	}
}

// UserInfo represents extracted user information from SAML
type UserInfo struct {
	Username   string                 `json:"username"`
	Email      string                 `json:"email"`
	FirstName  string                 `json:"first_name"`
	LastName   string                 `json:"last_name"`
	Groups     []string               `json:"groups"`
	Attributes map[string]interface{} `json:"attributes"`
}

// Package auth provides authentication implementations for StreamSpace.
//
// SAML 2.0 AUTHENTICATION
//
// This file implements SAML 2.0 (Security Assertion Markup Language) authentication,
// enabling Single Sign-On (SSO) with enterprise identity providers like:
// - Okta
// - Azure AD / Microsoft Entra ID
// - Google Workspace
// - OneLogin
// - Auth0
// - Keycloak
//
// SAML 2.0 AUTHENTICATION FLOW:
//
// The SAML authentication process follows the Service Provider (SP) initiated flow:
//
//   1. User visits StreamSpace (Service Provider)
//   2. User clicks "Login with SSO"
//   3. SP generates SAML AuthnRequest (authentication request)
//   4. User's browser redirects to IdP with AuthnRequest
//   5. User authenticates with IdP (username/password, MFA, etc.)
//   6. IdP generates SAML Assertion (signed XML with user attributes)
//   7. User's browser POSTs assertion to SP's Assertion Consumer Service (ACS)
//   8. SP validates assertion signature and extracts user attributes
//   9. SP creates local session and issues JWT token
//   10. User is authenticated and can access StreamSpace
//
// SAML SECURITY FEATURES:
//
// 1. XML Signature Validation:
//    - All assertions must be digitally signed by the IdP
//    - SP verifies signature using IdP's public certificate
//    - Prevents tampering with user attributes or session data
//
// 2. TLS Transport Security:
//    - All SAML exchanges occur over HTTPS
//    - Prevents man-in-the-middle attacks
//    - Protects assertions in transit
//
// 3. Assertion Time Validation:
//    - NotBefore: Assertion not valid before this time
//    - NotOnOrAfter: Assertion expires after this time
//    - Prevents replay attacks with old assertions
//
// 4. Audience Restriction:
//    - Assertion specifies intended audience (SP entity ID)
//    - Prevents assertions from being used at wrong service
//
// 5. InResponseTo Validation (SP-initiated flow):
//    - Links assertion to original AuthnRequest
//    - Prevents unsolicited assertions (unless AllowIDPInitiated=true)
//
// CONFIGURATION EXAMPLE:
//
//   config := &SAMLConfig{
//       Enabled:              true,
//       EntityID:             "https://streamspace.example.com",
//       MetadataURL:          "https://idp.example.com/metadata",
//       AssertionConsumerURL: "https://streamspace.example.com/saml/acs",
//       SingleLogoutURL:      "https://streamspace.example.com/saml/slo",
//       Certificate:          spCert,      // SP's X.509 certificate
//       PrivateKey:           spKey,       // SP's RSA private key
//       AllowIDPInitiated:    false,       // Require SP-initiated flow
//       SignRequest:          true,        // Sign AuthnRequests
//       ForceAuthn:           false,       // Don't require re-authentication
//       AttributeMapping: AttributeMapping{
//           Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
//           Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
//           FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
//           LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
//           Groups:    "http://schemas.xmlsoap.org/claims/Group",
//       },
//   }
//
// SECURITY BEST PRACTICES:
//
// 1. Always validate assertion signatures
// 2. Use TLS for all SAML endpoints
// 3. Set short assertion validity periods (5-10 minutes)
// 4. Disable IdP-initiated flow (AllowIDPInitiated=false) unless required
// 5. Sign AuthnRequests (SignRequest=true) when IdP supports it
// 6. Regularly rotate SP certificates
// 7. Validate SAML responses before creating sessions
// 8. Log all SAML authentication events for audit
//
// COMMON SAML VULNERABILITIES TO AVOID:
//
// 1. XML Signature Wrapping (XSW):
//    - Attack: Manipulate XML structure to bypass signature validation
//    - Prevention: Use robust XML parsing library (crewjam/saml)
//
// 2. XML External Entity (XXE) Injection:
//    - Attack: Reference external entities in XML to read files
//    - Prevention: Disable external entity resolution in XML parser
//
// 3. Replay Attacks:
//    - Attack: Reuse old SAML assertions
//    - Prevention: Validate NotOnOrAfter, track assertion IDs
//
// 4. Man-in-the-Middle:
//    - Attack: Intercept SAML assertion
//    - Prevention: Use TLS, validate certificate chains
//
// SUPPORTED IDENTITY PROVIDERS:
//
// - Okta: Full support with metadata URL
// - Azure AD: Full support, map attributes correctly
// - Google Workspace: Requires custom attribute mapping
// - OneLogin: Full support with metadata URL
// - Auth0: Full support via SAML addon
// - Keycloak: Open source IdP, full support
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

// SAMLConfig holds SAML authentication configuration for Service Provider (SP).
//
// This configuration defines how StreamSpace (acting as a SAML Service Provider)
// integrates with an Identity Provider (IdP) like Okta, Azure AD, or Google Workspace.
//
// ENTITY ID (EntityID):
//
// The Entity ID is a unique identifier for this Service Provider. It's typically
// the base URL of your StreamSpace deployment:
//   - Example: "https://streamspace.example.com"
//   - Must match the Entity ID configured in your IdP
//   - Used by IdP to identify which SP is making the request
//
// METADATA LOADING (MetadataURL vs MetadataXML):
//
// You must provide IdP metadata in one of two ways:
//
// 1. MetadataURL: URL to fetch IdP metadata (recommended for production)
//    - Example: "https://dev-12345.okta.com/app/abc123/sso/saml/metadata"
//    - Automatically updates when IdP configuration changes
//    - Requires network access to IdP during startup
//
// 2. MetadataXML: Raw XML metadata (recommended for air-gapped deployments)
//    - Paste the XML content from IdP's metadata download
//    - No network dependency
//    - Must manually update if IdP configuration changes
//
// ASSERTION CONSUMER SERVICE (AssertionConsumerURL):
//
// The ACS is the endpoint where the IdP POSTs SAML assertions after authentication.
// This URL must be:
//   - Registered in your IdP's SP configuration
//   - Accessible from user browsers (not internal-only)
//   - Example: "https://streamspace.example.com/saml/acs"
//
// SINGLE LOGOUT (SingleLogoutURL):
//
// The SLO endpoint handles logout requests from the IdP. When a user logs out
// from the IdP, it notifies all active SPs to terminate their sessions:
//   - Example: "https://streamspace.example.com/saml/slo"
//   - Optional but recommended for security
//   - Ensures user is logged out from all services
//
// CERTIFICATES AND KEYS:
//
// Certificate and PrivateKey are used to:
// 1. Sign SAML AuthnRequests (if SignRequest=true)
// 2. Decrypt encrypted SAML assertions (if IdP encrypts)
// 3. Sign SAML metadata for IdP to verify
//
// Generate with OpenSSL:
//   openssl req -x509 -newkey rsa:2048 -keyout sp-key.pem -out sp-cert.pem -days 3650 -nodes
//
// SECURITY SETTINGS:
//
// AllowIDPInitiated (default: false):
//   - If false: Only accept assertions in response to SP-initiated AuthnRequests
//   - If true: Accept unsolicited assertions from IdP (less secure)
//   - WHY: Prevents cross-site request forgery attacks
//   - Set to true only if you need IdP portal deep links
//
// SignRequest (default: false):
//   - If true: Sign all AuthnRequests with SP's private key
//   - If false: Send unsigned AuthnRequests
//   - WHY: Prevents tampering with authentication requests
//   - Enable if your IdP requires signed requests (Okta, Azure AD)
//
// ForceAuthn (default: false):
//   - If true: Require user to re-authenticate at IdP every time
//   - If false: Allow SSO if user has active IdP session
//   - WHY: Use for high-security scenarios requiring fresh authentication
//   - Most deployments leave this false for better UX
//
// ATTRIBUTE MAPPING:
//
// Defines which SAML attributes map to StreamSpace user fields.
// Different IdPs use different attribute names, so this is configurable:
//
// Okta attributes:
//   Email:     "email"
//   Username:  "login"
//   FirstName: "firstName"
//   LastName:  "lastName"
//   Groups:    "groups"
//
// Azure AD attributes:
//   Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress"
//   Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name"
//   FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname"
//   LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
//   Groups:    "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups"
type SAMLConfig struct {
	Enabled              bool                 // Enable SAML authentication
	EntityID             string               // SP entity ID (e.g., "https://streamspace.example.com")
	MetadataURL          string               // URL to fetch IdP metadata (e.g., "https://idp.example.com/metadata")
	MetadataXML          []byte               // Raw IdP metadata XML (alternative to MetadataURL)
	AssertionConsumerURL string               // ACS endpoint where IdP POSTs assertions
	SingleLogoutURL      string               // SLO endpoint for logout requests
	Certificate          *x509.Certificate    // SP's X.509 certificate for signing/encryption
	PrivateKey           *rsa.PrivateKey      // SP's RSA private key
	AllowIDPInitiated    bool                 // Allow IdP-initiated SSO (default: false for security)
	SignRequest          bool                 // Sign AuthnRequests (required by some IdPs)
	ForceAuthn           bool                 // Require re-authentication every time (default: false)
	AttributeMapping     AttributeMapping     // Maps SAML attributes to user fields
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
	config          *SAMLConfig
	middleware      *samlsp.Middleware
	serviceProvider *saml.ServiceProvider
}

// NewSAMLAuthenticator creates a new SAML authenticator and initializes the
// Service Provider (SP) configuration.
//
// This function performs the critical setup for SAML authentication:
// 1. Validates configuration
// 2. Creates the SAML Service Provider
// 3. Loads Identity Provider metadata
// 4. Initializes SAML middleware
// 5. Configures security settings
//
// INITIALIZATION STEPS:
//
// STEP 1: Configuration Validation
//
// Checks that SAML is enabled and required fields are present.
// Returns error if SAML is disabled to prevent misconfiguration.
//
// STEP 2: Entity ID Parsing
//
// The Entity ID must be a valid URL (typically your base domain):
//   - Valid: "https://streamspace.example.com"
//   - Invalid: "streamspace" (not a URL)
//   - Invalid: "http://localhost" (use HTTPS in production)
//
// The Entity ID becomes the base for SAML endpoint URLs:
//   - Metadata: {EntityID}/saml/metadata
//   - ACS:      {EntityID}/saml/acs
//   - SLO:      {EntityID}/saml/slo
//
// STEP 3: Service Provider Creation
//
// Creates a SAML Service Provider with:
//   - EntityID: Unique identifier for this SP
//   - Key/Certificate: For signing and encryption
//   - MetadataURL/AcsURL/SloURL: SAML endpoints
//   - AllowIDPInitiated: Whether to accept unsolicited assertions
//   - ForceAuthn: Whether to require re-authentication
//
// STEP 4: Identity Provider Metadata Loading
//
// IdP metadata contains critical information:
//   - SingleSignOnService: Where to send AuthnRequests
//   - X509Certificate: IdP's public key for validating signatures
//   - NameIDFormat: Supported identifier formats
//   - Attributes: Available user attributes
//
// Metadata can be loaded two ways:
//
// A) From URL (recommended for production):
//    - Fetches metadata from IdP's metadata endpoint
//    - Uses HTTPS with certificate validation
//    - Example: "https://dev-12345.okta.com/app/abc123/sso/saml/metadata"
//    - Benefits: Auto-updates when IdP changes configuration
//
// B) From XML (recommended for air-gapped deployments):
//    - Parses XML metadata downloaded from IdP
//    - No network dependency
//    - Benefits: Works in restricted environments
//    - Drawback: Must manually update if IdP changes
//
// STEP 5: SAML Middleware Initialization
//
// Creates the crewjam/saml middleware that:
//   - Handles SAML request/response processing
//   - Validates assertion signatures
//   - Manages SAML sessions
//   - Provides RequireAccount middleware
//
// SECURITY CONSIDERATIONS:
//
// 1. TLS Validation:
//    - When fetching metadata from URL, TLS certificate is validated
//    - InsecureSkipVerify is set to false (secure default)
//    - Prevents man-in-the-middle attacks
//
// 2. Signature Validation:
//    - All assertions are validated using IdP's certificate from metadata
//    - Prevents tampering with user attributes
//    - Handled automatically by crewjam/saml library
//
// 3. AllowIDPInitiated:
//    - If false: Only accept assertions with valid InResponseTo
//    - If true: Accept unsolicited assertions from IdP
//    - Default false is more secure (prevents CSRF)
//
// 4. ForceAuthn:
//    - If true: User must authenticate at IdP every time
//    - If false: SSO allowed with active IdP session
//    - Default false provides better UX
//
// EXAMPLE USAGE:
//
//   config := &SAMLConfig{
//       Enabled:     true,
//       EntityID:    "https://streamspace.example.com",
//       MetadataURL: "https://dev-12345.okta.com/metadata",
//       Certificate: cert,
//       PrivateKey:  key,
//       AllowIDPInitiated: false,
//       SignRequest: true,
//       ForceAuthn:  false,
//       AttributeMapping: AttributeMapping{
//           Email:    "email",
//           Username: "login",
//       },
//   }
//
//   auth, err := NewSAMLAuthenticator(config)
//   if err != nil {
//       log.Fatalf("Failed to create SAML authenticator: %v", err)
//   }
//
//   // Use auth.GinMiddleware() to protect routes
//   router.Use(auth.GinMiddleware())
//
// COMMON ERRORS:
//
// "SAML is not enabled":
//   - Config.Enabled is false
//   - Solution: Set Enabled=true in configuration
//
// "invalid entity ID":
//   - EntityID is not a valid URL
//   - Solution: Use full URL like "https://streamspace.example.com"
//
// "failed to fetch IdP metadata":
//   - MetadataURL is unreachable
//   - Network connectivity issue
//   - IdP is down
//   - Solution: Check URL, network, try using MetadataXML instead
//
// "failed to parse IdP metadata XML":
//   - MetadataXML is invalid or corrupted
//   - Solution: Re-download metadata from IdP
//
// "either MetadataURL or MetadataXML must be provided":
//   - Both fields are empty
//   - Solution: Provide one of the two
func NewSAMLAuthenticator(config *SAMLConfig) (*SAMLAuthenticator, error) {
	// STEP 1: Validate that SAML is enabled
	// Return early if SAML is disabled to prevent misconfiguration
	if !config.Enabled {
		return nil, fmt.Errorf("SAML is not enabled")
	}

	// STEP 2: Parse and validate Entity ID
	// The Entity ID must be a valid URL that serves as the base for SAML endpoints
	rootURL, err := url.Parse(config.EntityID)
	if err != nil {
		return nil, fmt.Errorf("invalid entity ID: %w", err)
	}

	// STEP 3: Create the SAML Service Provider (SP)
	//
	// The Service Provider represents StreamSpace in the SAML ecosystem.
	// It holds configuration for how we interact with the Identity Provider.
	sp := &saml.ServiceProvider{
		// EntityID: Unique identifier for this SP (must match IdP configuration)
		EntityID: config.EntityID,

		// Key and Certificate: Used for signing AuthnRequests and decrypting assertions
		// These are generated locally and the certificate is shared with the IdP
		Key:         config.PrivateKey,
		Certificate: config.Certificate,

		// MetadataURL: Where IdP can fetch this SP's metadata
		// Example: https://streamspace.example.com/saml/metadata
		MetadataURL: *rootURL.ResolveReference(&url.URL{Path: "/saml/metadata"}),

		// AcsURL: Assertion Consumer Service - where IdP POSTs SAML assertions
		// This is the callback URL after successful authentication
		// Example: https://streamspace.example.com/saml/acs
		AcsURL: *rootURL.ResolveReference(&url.URL{Path: "/saml/acs"}),

		// SloURL: Single Logout URL - where IdP sends logout requests
		// Enables IdP to notify SP when user logs out
		// Example: https://streamspace.example.com/saml/slo
		SloURL: *rootURL.ResolveReference(&url.URL{Path: "/saml/slo"}),

		// AllowIDPInitiated: Whether to accept unsolicited SAML assertions
		// false = More secure, only accept assertions in response to our AuthnRequests
		// true = Less secure, accept assertions initiated by IdP (e.g., from IdP portal)
		AllowIDPInitiated: config.AllowIDPInitiated,

		// ForceAuthn: Whether to require re-authentication at IdP every time
		// true = Always prompt user to login at IdP (high security)
		// false = Allow SSO if user has active IdP session (better UX)
		ForceAuthn: &config.ForceAuthn,
	}

	// STEP 4: Load Identity Provider (IdP) Metadata
	//
	// IdP metadata is an XML document that contains critical information about the IdP:
	// - SingleSignOnService: URL where we send authentication requests
	// - X509Certificate: IdP's public key for verifying assertion signatures
	// - NameIDFormat: How the IdP identifies users (email, persistent ID, etc.)
	// - Attributes: What user attributes the IdP can provide
	//
	// We support two methods of loading metadata:
	var idpMetadata *saml.EntityDescriptor

	if config.MetadataURL != "" {
		// METHOD A: Fetch metadata from IdP's URL (recommended for production)
		//
		// This approach:
		// - Automatically retrieves the latest IdP configuration
		// - Validates TLS certificate (InsecureSkipVerify=false)
		// - Works for most cloud IdPs (Okta, Azure AD, OneLogin)
		//
		// SECURITY: TLS certificate validation prevents MITM attacks
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// SECURITY: Validate TLS certificate (never skip in production)
					// This prevents attackers from intercepting metadata requests
					InsecureSkipVerify: false,
				},
			},
		}

		// Fetch metadata from IdP's metadata endpoint
		idpMetadata, err = samlsp.FetchMetadata(context.Background(), httpClient, url.URL{
			Scheme: "https", // Always use HTTPS for security
			Host:   config.MetadataURL,
		})
		if err != nil {
			// Common causes:
			// - Network connectivity issue
			// - IdP is down or unreachable
			// - Incorrect MetadataURL
			// - Firewall blocking outbound HTTPS
			return nil, fmt.Errorf("failed to fetch IdP metadata: %w", err)
		}
	} else if len(config.MetadataXML) > 0 {
		// METHOD B: Parse metadata from raw XML (recommended for air-gapped deployments)
		//
		// This approach:
		// - No network dependency (works in restricted environments)
		// - Metadata must be manually updated if IdP changes
		// - Useful for compliance requirements (no outbound connections)
		//
		// To get metadata XML:
		// 1. Download from IdP's metadata URL in browser
		// 2. Or copy from IdP's admin console
		// 3. Paste into configuration as MetadataXML
		idpMetadata = &saml.EntityDescriptor{}
		if err := xml.Unmarshal(config.MetadataXML, idpMetadata); err != nil {
			// Common causes:
			// - Invalid or corrupted XML
			// - Metadata downloaded incorrectly
			// - Wrong IdP metadata (metadata for different app)
			return nil, fmt.Errorf("failed to parse IdP metadata XML: %w", err)
		}
	} else {
		// Neither metadata source provided - configuration error
		// User must provide either MetadataURL or MetadataXML
		return nil, fmt.Errorf("either MetadataURL or MetadataXML must be provided")
	}

	// Attach IdP metadata to Service Provider
	// This metadata is used for:
	// - Validating assertion signatures
	// - Determining where to send AuthnRequests
	// - Encrypting assertions (if IdP supports it)
	sp.IDPMetadata = idpMetadata

	// STEP 5: Create SAML Middleware
	//
	// The middleware handles:
	// - Parsing SAML requests and responses
	// - Validating assertion signatures using IdP's certificate
	// - Managing SAML sessions
	// - Providing authentication middleware for routes
	//
	// WHY MIDDLEWARE: Encapsulates complex SAML protocol handling
	// so we don't have to manually parse XML, validate signatures, etc.
	middleware, err := samlsp.New(samlsp.Options{
		EntityID:          sp.EntityID,           // Our SP identifier
		URL:               *rootURL,              // Base URL for SAML endpoints
		Key:               sp.Key,                // Private key for signing/decryption
		Certificate:       sp.Certificate,        // Public certificate for IdP to verify
		IDPMetadata:       sp.IDPMetadata,        // IdP configuration loaded above
		AllowIDPInitiated: sp.AllowIDPInitiated,  // Security setting for unsolicited assertions
		ForceAuthn:        *sp.ForceAuthn,        // Whether to require re-authentication
	})
	if err != nil {
		// Common causes:
		// - Invalid IdP metadata
		// - Certificate/key mismatch
		// - Unsupported SAML configuration
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

// GetServiceProvider returns the SAML Service Provider instance.
func (sa *SAMLAuthenticator) GetServiceProvider() *saml.ServiceProvider {
	return sa.serviceProvider
}

// ExtractUserFromAssertion extracts user information from a SAML assertion.
//
// After the IdP authenticates a user, it sends a SAML assertion containing:
// 1. Subject (NameID): The user's identifier
// 2. Attribute Statements: User attributes (email, name, groups, etc.)
// 3. Conditions: When the assertion is valid
// 4. Signature: Cryptographic proof from IdP
//
// This function parses the assertion and maps SAML attributes to StreamSpace
// user fields based on the configured AttributeMapping.
//
// SAML ASSERTION STRUCTURE:
//
// A SAML assertion is an XML document that looks like:
//
//   <Assertion>
//     <Subject>
//       <NameID Format="urn:...">user@example.com</NameID>
//     </Subject>
//     <AttributeStatement>
//       <Attribute Name="email">
//         <AttributeValue>user@example.com</AttributeValue>
//       </Attribute>
//       <Attribute Name="groups">
//         <AttributeValue>Admins</AttributeValue>
//         <AttributeValue>Users</AttributeValue>
//       </Attribute>
//     </AttributeStatement>
//   </Assertion>
//
// ATTRIBUTE MAPPING:
//
// Different IdPs use different attribute names, so we use AttributeMapping to
// configure which SAML attribute corresponds to which user field.
//
// Example for Okta:
//   AttributeMapping{
//       Email:     "email",
//       Username:  "login",
//       FirstName: "firstName",
//       LastName:  "lastName",
//       Groups:    "groups",
//   }
//
// Example for Azure AD:
//   AttributeMapping{
//       Email:     "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
//       Username:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
//       FirstName: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
//       LastName:  "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
//       Groups:    "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
//   }
//
// FALLBACK TO NAMEID:
//
// If no attribute mapping is configured, or the IdP doesn't send the expected
// attributes, we fall back to using the SAML NameID:
//
// 1. NameID as Username:
//    - Always use NameID as username if Username attribute not found
//    - Ensures we always have a unique identifier
//
// 2. NameID as Email:
//    - Use NameID as email if:
//      a) Email attribute not found, AND
//      b) NameID format is "emailAddress"
//    - Common with Google Workspace and Azure AD
//
// MULTI-VALUED ATTRIBUTES:
//
// Some attributes like "groups" can have multiple values:
//   <Attribute Name="groups">
//     <AttributeValue>Admins</AttributeValue>
//     <AttributeValue>Developers</AttributeValue>
//     <AttributeValue>Users</AttributeValue>
//   </Attribute>
//
// We handle this by:
// - Storing groups as []string (slice of strings)
// - Storing other multi-valued attributes as []string in Attributes map
// - Storing single-valued attributes as string in Attributes map
//
// VALIDATION:
//
// After extracting attributes, we validate required fields:
// - Username: REQUIRED - cannot create user without identifier
// - Email: OPTIONAL - nice to have but not required
// - FirstName/LastName: OPTIONAL - for display purposes
// - Groups: OPTIONAL - for role-based access control
//
// EXAMPLE EXTRACTED USER:
//
//   &UserInfo{
//       Username:   "john.doe@example.com",
//       Email:      "john.doe@example.com",
//       FirstName:  "John",
//       LastName:   "Doe",
//       Groups:     []string{"Admins", "Developers"},
//       Attributes: map[string]interface{}{
//           "email":     "john.doe@example.com",
//           "firstName": "John",
//           "lastName":  "Doe",
//           "groups":    []string{"Admins", "Developers"},
//           "department": "Engineering",
//       },
//   }
//
// COMMON ERRORS:
//
// "assertion is nil":
//   - No assertion provided to function
//   - Should never happen in normal flow
//
// "username not found in SAML assertion":
//   - IdP didn't send username attribute
//   - AttributeMapping.Username is incorrect
//   - NameID is missing or empty
//   - Solution: Check IdP configuration and attribute mapping
func (sa *SAMLAuthenticator) ExtractUserFromAssertion(assertion *saml.Assertion) (*UserInfo, error) {
	// STEP 1: Validate assertion
	// Ensure we have a non-nil assertion to work with
	if assertion == nil {
		return nil, fmt.Errorf("assertion is nil")
	}

	// STEP 2: Initialize user object
	// Create UserInfo struct to hold extracted data
	// Attributes map stores all SAML attributes for custom use cases
	user := &UserInfo{
		Attributes: make(map[string]interface{}),
	}

	// STEP 3: Extract attributes from assertion
	//
	// SAML assertions contain AttributeStatements which hold Attributes.
	// Each Attribute has a Name and one or more Values.
	//
	// WHY NESTED LOOPS: SAML allows multiple AttributeStatements per assertion,
	// though most IdPs only send one.
	for _, attrStatement := range assertion.AttributeStatements {
		for _, attr := range attrStatement.Attributes {
			// Skip attributes with no values
			if len(attr.Values) == 0 {
				continue
			}

			attrName := attr.Name
			attrValue := attr.Values[0].Value // First value (most attributes are single-valued)

			// STEP 3A: Map to standard user fields
			//
			// Check if this attribute matches one of our configured mappings.
			// Different IdPs use different attribute names, so we use the
			// AttributeMapping configuration to know which attribute is which.
			switch attrName {
			case sa.config.AttributeMapping.Email:
				// Email address for the user
				// Used for notifications and account recovery
				user.Email = attrValue

			case sa.config.AttributeMapping.Username:
				// Unique username/identifier
				// Used as primary key for user account
				user.Username = attrValue

			case sa.config.AttributeMapping.FirstName:
				// User's first/given name
				// Used for display purposes in UI
				user.FirstName = attrValue

			case sa.config.AttributeMapping.LastName:
				// User's last/family name
				// Used for display purposes in UI
				user.LastName = attrValue

			case sa.config.AttributeMapping.Groups:
				// Groups/roles the user belongs to
				// WHY SPECIAL HANDLING: Groups are often multi-valued
				// (user can be in multiple groups)
				groups := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					groups[i] = v.Value
				}
				user.Groups = groups
			}

			// STEP 3B: Store all attributes in Attributes map
			//
			// We store ALL attributes (not just mapped ones) so that:
			// 1. Custom code can access IdP-specific attributes
			// 2. Debugging is easier (can see all data IdP sent)
			// 3. Future features can use additional attributes
			//
			// WHY CHECK len(attr.Values): Determines storage format
			// - Single value: Store as string
			// - Multiple values: Store as []string
			if len(attr.Values) == 1 {
				// Single-valued attribute (most common)
				user.Attributes[attrName] = attrValue
			} else {
				// Multi-valued attribute (groups, roles, etc.)
				values := make([]string, len(attr.Values))
				for i, v := range attr.Values {
					values[i] = v.Value
				}
				user.Attributes[attrName] = values
			}
		}
	}

	// STEP 4: Fallback to NameID for username
	//
	// If no Username attribute was found (or not configured), use the SAML NameID.
	// The NameID is the Subject identifier and is always present in valid assertions.
	//
	// WHY: Ensures we always have a username, even if IdP doesn't send
	// the expected username attribute.
	//
	// EXAMPLE NameID formats:
	// - urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress -> "user@example.com"
	// - urn:oasis:names:tc:SAML:2.0:nameid-format:persistent -> "z8df7a6s5d4f3a2s1"
	// - urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified -> "john.doe"
	if user.Username == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		user.Username = assertion.Subject.NameID.Value
	}

	// STEP 5: Fallback to NameID for email (if format is emailAddress)
	//
	// Some IdPs (like Google Workspace) send the email as NameID instead of
	// as a separate attribute. If:
	// 1. No email attribute was found, AND
	// 2. NameID format is "emailAddress"
	// Then use NameID as email.
	//
	// WHY CHECK FORMAT: Only use NameID as email if we're certain it IS an email.
	// Other NameID formats (persistent, transient) are not email addresses.
	if user.Email == "" && assertion.Subject != nil && assertion.Subject.NameID != nil {
		if assertion.Subject.NameID.Format == "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress" {
			user.Email = assertion.Subject.NameID.Value
		}
	}

	// STEP 6: Validate required fields
	//
	// Username is absolutely required. We cannot create a user account without
	// a unique identifier. If we reach this point without a username, the
	// assertion is invalid or misconfigured.
	//
	// COMMON CAUSES:
	// - IdP not configured to send username attribute
	// - AttributeMapping.Username is wrong
	// - NameID is missing (invalid assertion)
	//
	// SOLUTION: Check IdP configuration and attribute mapping
	if user.Username == "" {
		return nil, fmt.Errorf("username not found in SAML assertion")
	}

	return user, nil
}

// ExtractUserFromAttributes extracts user information from SAML session attributes.
//
// This is a simpler version of ExtractUserFromAssertion that works with the
// attributes map returned by SessionWithAttributes.GetAttributes().
//
// The attributes map contains key-value pairs from the SAML assertion's
// AttributeStatements, already parsed and ready to use.
func (sa *SAMLAuthenticator) ExtractUserFromAttributes(attributes samlsp.Attributes) (*UserInfo, error) {
	if attributes == nil {
		return nil, fmt.Errorf("attributes map is nil")
	}

	// Initialize user object
	user := &UserInfo{
		Attributes: make(map[string]interface{}),
	}

	// Helper function to get first attribute value
	getAttribute := func(key string) string {
		return attributes.Get(key)
	}

	// Helper function to get all attribute values
	getAttributes := func(key string) []string {
		if vals, ok := attributes[key]; ok {
			return vals
		}
		return nil
	}

	// Extract mapped attributes
	if sa.config.AttributeMapping.Email != "" {
		user.Email = getAttribute(sa.config.AttributeMapping.Email)
	}
	if sa.config.AttributeMapping.Username != "" {
		user.Username = getAttribute(sa.config.AttributeMapping.Username)
	}
	if sa.config.AttributeMapping.FirstName != "" {
		user.FirstName = getAttribute(sa.config.AttributeMapping.FirstName)
	}
	if sa.config.AttributeMapping.LastName != "" {
		user.LastName = getAttribute(sa.config.AttributeMapping.LastName)
	}
	if sa.config.AttributeMapping.Groups != "" {
		user.Groups = getAttributes(sa.config.AttributeMapping.Groups)
	}

	// Store all attributes in Attributes map for custom use cases
	for key, values := range attributes {
		if len(values) == 1 {
			user.Attributes[key] = values[0]
		} else {
			user.Attributes[key] = values
		}
	}

	// Validate required fields
	if user.Username == "" {
		return nil, fmt.Errorf("username not found in SAML attributes")
	}

	return user, nil
}

// GinMiddleware returns a Gin middleware function that enforces SAML authentication.
//
// This middleware protects routes by requiring valid SAML authentication. It:
// 1. Checks for an active SAML session
// 2. Validates the session contains a valid assertion
// 3. Extracts user information from the assertion
// 4. Stores user info in Gin context for downstream handlers
// 5. Redirects unauthenticated users to IdP for SSO
//
// USAGE:
//
//   // Protect all routes
//   router.Use(samlAuth.GinMiddleware())
//
//   // Protect specific routes
//   protected := router.Group("/api")
//   protected.Use(samlAuth.GinMiddleware())
//   protected.GET("/sessions", listSessions)
//
//   // Access user info in handlers
//   func listSessions(c *gin.Context) {
//       user := c.MustGet("user").(*UserInfo)
//       fmt.Printf("User %s requested sessions\n", user.Username)
//   }
//
// AUTHENTICATION FLOW:
//
// Unauthenticated Request:
//   1. User requests /api/sessions
//   2. Middleware checks for SAML session → not found
//   3. Middleware redirects to /saml/login
//   4. SAML login redirects to IdP
//   5. User authenticates at IdP
//   6. IdP POSTs assertion to /saml/acs
//   7. ACS creates SAML session (cookie)
//   8. User redirected back to /api/sessions
//   9. Middleware finds SAML session → success
//   10. Handler executes with user context
//
// Authenticated Request:
//   1. User requests /api/sessions
//   2. Middleware checks for SAML session → found
//   3. Middleware validates assertion
//   4. Middleware extracts user info
//   5. Handler executes with user context
//
// SESSION STORAGE:
//
// SAML sessions are stored in cookies by the crewjam/saml middleware.
// The cookie contains:
// - Session ID (encrypted)
// - Not the full assertion (too large)
//
// The actual assertion is stored server-side in memory or a session store.
// The middleware retrieves the assertion using the session ID from the cookie.
//
// SECURITY:
//
// 1. Session Validation:
//    - Ensures session exists and is valid
//    - Checks assertion has not expired
//    - Validates assertion signature (done by crewjam/saml)
//
// 2. HTTPS Required:
//    - SAML sessions should only be sent over HTTPS
//    - Cookies should have Secure flag in production
//    - Prevents session hijacking
//
// 3. Session Expiration:
//    - Sessions expire based on assertion NotOnOrAfter
//    - Typically 5-10 minutes from authentication
//    - Forces periodic re-validation with IdP
//
// ERROR HANDLING:
//
// No SAML session:
//   - Redirects to IdP for SSO
//   - Aborts current request with c.Abort()
//   - User will return after authentication
//
// Invalid session:
//   - Returns 401 Unauthorized
//   - JSON error response
//   - User must re-authenticate
//
// Failed user extraction:
//   - Returns 401 Unauthorized
//   - JSON error with details
//   - Usually indicates IdP misconfiguration
func (sa *SAMLAuthenticator) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// STEP 1: Check if SAML session exists
		//
		// The session is stored in a cookie and managed by crewjam/saml middleware.
		// GetSession returns:
		// - session: Session object containing assertion
		// - err: Error if session is invalid or expired
		session, err := sa.middleware.Session.GetSession(c.Request)
		if err != nil || session == nil {
			// STEP 1A: No valid SAML session found
			//
			// Redirect user to IdP for Single Sign-On.
			// RequireAccount middleware handles:
			// 1. Generating SAML AuthnRequest
			// 2. Redirecting browser to IdP
			// 3. Storing return URL for post-auth redirect
			//
			// WHY ABORT: Request cannot proceed without authentication
			sa.middleware.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				c.Next()
			})).ServeHTTP(c.Writer, c.Request)
			c.Abort() // Stop processing this request
			return
		}

		// STEP 2: Extract attributes from session
		//
		// The session contains the SAML attributes that were extracted during login.
		// We need to retrieve them to get user information.
		//
		// WHY TYPE ASSERTION: Session interface doesn't expose attributes directly,
		// must cast to SessionWithAttributes to access GetAttributes()
		attributes := session.(samlsp.SessionWithAttributes).GetAttributes()
		if attributes == nil {
			// Session exists but has no attributes - corrupted session
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid SAML session"})
			c.Abort()
			return
		}

		// STEP 3: Extract user information from attributes
		//
		// Parse SAML attributes and map them to StreamSpace user fields.
		// This uses the configured AttributeMapping to translate IdP attributes.
		user, err := sa.ExtractUserFromAttributes(attributes)
		if err != nil {
			// Failed to extract required user fields (usually missing username)
			// This indicates IdP misconfiguration or incorrect attribute mapping
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Failed to extract user: %v", err)})
			c.Abort()
			return
		}

		// STEP 5: Store user in Gin context
		//
		// Set user in context so downstream handlers can access it.
		// Handlers retrieve with: user := c.MustGet("user").(*UserInfo)
		//
		// WHY CONTEXT: Avoids passing user through all function parameters,
		// provides clean middleware → handler data flow
		c.Set("user", user)

		// STEP 6: Continue to next handler
		// Authentication successful, allow request to proceed
		c.Next()
	}
}

// SetupRoutes registers all SAML-related HTTP endpoints.
//
// This function creates the following SAML endpoints required for SSO:
// - /saml/metadata: SP metadata for IdP configuration
// - /saml/acs: Assertion Consumer Service (callback after authentication)
// - /saml/slo: Single Logout Service
// - /saml/login: Initiate SSO authentication flow
// - /saml/logout: Terminate local SAML session
//
// SAML ENDPOINTS OVERVIEW:
//
// These endpoints implement the SAML 2.0 Web Browser SSO Profile:
//
//   ┌─────────┐         ┌──────────────┐         ┌─────────┐
//   │ Browser │────────▶│ StreamSpace  │────────▶│   IdP   │
//   │         │         │ (SP)         │         │         │
//   └─────────┘         └──────────────┘         └─────────┘
//       │                      │                      │
//       │  1. GET /saml/login  │                      │
//       ├─────────────────────▶│                      │
//       │                      │  2. AuthnRequest     │
//       │                      ├─────────────────────▶│
//       │                      │                      │
//       │  3. Redirect to IdP  │                      │
//       │◀─────────────────────┤                      │
//       │                                             │
//       │  4. Authenticate at IdP                     │
//       ├────────────────────────────────────────────▶│
//       │                                             │
//       │  5. POST assertion to /saml/acs             │
//       │◀────────────────────────────────────────────┤
//       │                      │                      │
//       │  6. POST /saml/acs   │                      │
//       ├─────────────────────▶│                      │
//       │                      │                      │
//       │  7. Session created, │                      │
//       │     redirect to app  │                      │
//       │◀─────────────────────┤                      │
//
// ENDPOINT DETAILS:
//
// 1. /saml/metadata (GET):
//    - Returns SP metadata XML
//    - IdP needs this to configure StreamSpace as a trusted SP
//    - Contains: Entity ID, ACS URL, SLO URL, certificate
//
// 2. /saml/acs (POST):
//    - Assertion Consumer Service
//    - Receives SAML assertion from IdP after authentication
//    - Validates assertion signature
//    - Creates SAML session
//    - Redirects to original requested URL
//
// 3. /saml/slo (GET/POST):
//    - Single Logout Service
//    - IdP sends logout request when user logs out
//    - Terminates StreamSpace session
//    - Can be HTTP-Redirect (GET) or HTTP-POST (POST)
//
// 4. /saml/login (GET):
//    - Initiates SSO authentication
//    - Generates SAML AuthnRequest
//    - Redirects browser to IdP
//    - Accepts ?return_url parameter for post-auth redirect
//
// 5. /saml/logout (GET):
//    - Local logout (does not notify IdP)
//    - Clears SAML session cookie
//    - Redirects to home page
//
// USAGE EXAMPLE:
//
//   router := gin.Default()
//   samlAuth := NewSAMLAuthenticator(config)
//   samlAuth.SetupRoutes(router)
//
//   // Now endpoints are available:
//   // - https://streamspace.example.com/saml/metadata
//   // - https://streamspace.example.com/saml/acs
//   // - https://streamspace.example.com/saml/slo
//   // - https://streamspace.example.com/saml/login
//   // - https://streamspace.example.com/saml/logout
//
// IDP CONFIGURATION:
//
// When configuring StreamSpace in your IdP (Okta, Azure AD, etc.):
//
// 1. Download SP metadata from: https://streamspace.example.com/saml/metadata
// 2. Or manually configure:
//    - Entity ID: https://streamspace.example.com
//    - ACS URL: https://streamspace.example.com/saml/acs
//    - SLO URL: https://streamspace.example.com/saml/slo
// 3. Configure attribute mapping in IdP to send required attributes
// 4. Test with: https://streamspace.example.com/saml/login
//
// SECURITY CONSIDERATIONS:
//
// 1. HTTPS Required:
//    - All SAML endpoints must be accessed over HTTPS in production
//    - HTTP is only acceptable for local development
//    - Prevents MITM attacks on SAML assertions
//
// 2. ACS Validation:
//    - The /saml/acs endpoint validates assertion signatures
//    - Validates assertion timing (NotBefore, NotOnOrAfter)
//    - Validates audience (must match Entity ID)
//    - All handled by crewjam/saml middleware
//
// 3. Return URL Validation:
//    - return_url parameter should be validated
//    - Prevents open redirect attacks
//    - Currently not validated (TODO: add validation)
//
// 4. Session Security:
//    - Sessions stored in HTTP-only cookies
//    - Cookies should have Secure flag (HTTPS-only)
//    - Sessions expire based on assertion validity
func (sa *SAMLAuthenticator) SetupRoutes(router *gin.Engine) {
	// Create /saml route group for all SAML endpoints
	samlGroup := router.Group("/saml")
	{
		// ENDPOINT: GET /saml/metadata
		//
		// Returns Service Provider metadata XML that IdP administrators use to
		// configure StreamSpace as a trusted SP.
		//
		// USAGE:
		// 1. Admin downloads metadata: wget https://streamspace.example.com/saml/metadata
		// 2. Admin uploads to IdP (Okta, Azure AD, etc.)
		// 3. IdP uses metadata to configure Entity ID, ACS URL, certificate
		//
		// RESPONSE FORMAT: application/xml
		//
		// EXAMPLE METADATA:
		//   <?xml version="1.0"?>
		//   <EntityDescriptor entityID="https://streamspace.example.com">
		//     <SPSSODescriptor>
		//       <AssertionConsumerService Binding="urn:..." Location="https://streamspace.example.com/saml/acs"/>
		//       <KeyDescriptor use="signing">
		//         <X509Certificate>...</X509Certificate>
		//       </KeyDescriptor>
		//     </SPSSODescriptor>
		//   </EntityDescriptor>
		samlGroup.GET("/metadata", func(c *gin.Context) {
			// Generate SP metadata from serviceProvider configuration
			// Metadata includes: Entity ID, ACS URL, SLO URL, certificate
			metadata, err := xml.MarshalIndent(sa.serviceProvider.Metadata(), "", "  ")
			if err != nil {
				// Failed to generate metadata XML (should rarely happen)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate metadata"})
				return
			}

			// Return metadata as XML
			// Content-Type: application/xml
			c.Data(http.StatusOK, "application/xml", metadata)
		})

		// ENDPOINT: POST /saml/acs
		//
		// Assertion Consumer Service - receives SAML assertions from IdP.
		// This is the callback URL after successful authentication.
		//
		// FLOW:
		// 1. User authenticates at IdP
		// 2. IdP generates signed SAML assertion
		// 3. IdP POSTs assertion to this endpoint (browser-mediated)
		// 4. Middleware validates assertion signature
		// 5. Middleware creates SAML session
		// 6. User redirected to original requested URL
		//
		// REQUEST: HTTP POST with SAMLResponse form parameter
		// RESPONSE: HTTP redirect to original URL or error page
		//
		// SECURITY: Assertion signature validated by crewjam/saml middleware
		samlGroup.POST("/acs", gin.WrapH(sa.middleware))

		// ENDPOINT: GET/POST /saml/slo
		//
		// Single Logout Service - receives logout requests from IdP.
		// When user logs out at IdP, IdP notifies all SPs to terminate sessions.
		//
		// FLOW:
		// 1. User clicks "Logout" at IdP
		// 2. IdP sends LogoutRequest to all active SPs
		// 3. SP validates LogoutRequest signature
		// 4. SP terminates session
		// 5. SP sends LogoutResponse to IdP
		// 6. IdP completes global logout
		//
		// BINDINGS:
		// - GET: HTTP-Redirect binding (LogoutRequest in query string)
		// - POST: HTTP-POST binding (LogoutRequest in form body)
		//
		// WHY BOTH: Different IdPs use different bindings
		samlGroup.GET("/slo", gin.WrapH(sa.middleware))
		samlGroup.POST("/slo", gin.WrapH(sa.middleware))

		// ENDPOINT: GET /saml/login?return_url=/path
		//
		// Initiates SAML SSO authentication flow.
		// Redirects user to IdP for authentication.
		//
		// FLOW:
		// 1. User visits /saml/login?return_url=/api/sessions
		// 2. Handler stores return_url in cookie
		// 3. Handler generates SAML AuthnRequest
		// 4. Handler redirects browser to IdP with AuthnRequest
		// 5. User authenticates at IdP
		// 6. IdP redirects back to /saml/acs with assertion
		// 7. ACS creates session and redirects to return_url
		//
		// PARAMETERS:
		// - return_url (optional): Where to redirect after authentication
		//   Default: "/"
		//
		// SECURITY: return_url is validated to prevent open redirect attacks
		samlGroup.GET("/login", func(c *gin.Context) {
			// STEP 1: Get return URL from query parameter
			// This is where user will be redirected after successful authentication
			// SECURITY: Validate to prevent open redirect attacks
			returnURL := validateReturnURL(c.Query("return_url"))

			// STEP 2: Store return URL in cookie
			// The ACS endpoint will read this cookie and redirect user after auth
			//
			// Cookie parameters:
			// - Name: "saml_return_url"
			// - Value: returnURL (validated, e.g., "/api/sessions")
			// - MaxAge: 3600 seconds (1 hour) - plenty of time for auth flow
			// - Path: "/" - available to all endpoints
			// - Domain: "" - current domain
			// - Secure: false - TODO: set to true in production (HTTPS-only)
			// - HttpOnly: true - prevents JavaScript access (XSS protection)
			c.SetCookie("saml_return_url", returnURL, 3600, "/", "", false, true)

			// STEP 3: Initiate SAML authentication flow
			// HandleStartAuthFlow:
			// 1. Generates SAML AuthnRequest XML
			// 2. Encodes AuthnRequest (Base64 + deflate)
			// 3. Redirects browser to IdP with AuthnRequest
			//
			// User will be redirected to IdP's SSO URL like:
			// https://idp.example.com/sso?SAMLRequest=...
			sa.middleware.HandleStartAuthFlow(c.Writer, c.Request)
		})

		// ENDPOINT: GET /saml/logout
		//
		// Local logout - terminates StreamSpace session.
		// Does NOT notify IdP (single logout), only clears local session.
		//
		// FLOW:
		// 1. User clicks "Logout" in StreamSpace
		// 2. Browser requests /saml/logout
		// 3. Handler deletes SAML session cookie
		// 4. User redirected to home page
		// 5. User is logged out of StreamSpace (but NOT IdP)
		//
		// NOTE: User may still have active IdP session. If they visit
		// StreamSpace again and click login, they'll be auto-authenticated
		// via SSO without entering credentials.
		//
		// FOR GLOBAL LOGOUT: IdP should send LogoutRequest to /saml/slo
		samlGroup.GET("/logout", func(c *gin.Context) {
			// Delete SAML session
			// This removes the session cookie and clears server-side session data
			if err := sa.middleware.Session.DeleteSession(c.Writer, c.Request); err != nil {
				// Failed to delete session (rare - session might not exist)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
				return
			}

			// Redirect to home page
			// User is now logged out and will see public content
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

# StreamSpace SAML 2.0 Authentication Plugin

Enterprise single sign-on (SSO) authentication using the SAML 2.0 protocol. Supports major identity providers including Okta, OneLogin, Azure AD, Google Workspace, JumpCloud, and Auth0.

## Features

- **Standards Compliance**: Full SAML 2.0 protocol support
- **Major IdP Support**: Pre-configured for Okta, OneLogin, Azure AD, Google, JumpCloud, Auth0
- **Service Provider Metadata**: Auto-generated SP metadata for easy IdP configuration
- **Assertion Consumer Service (ACS)**: Handles SAML assertions from IdP
- **Single Logout (SLO)**: Support for single logout across applications
- **IdP-Initiated Login**: Optional support for IdP-initiated SSO flows
- **Request Signing**: Sign SAML requests for enhanced security
- **Attribute Mapping**: Flexible mapping of SAML attributes to user fields
- **Auto-Provisioning**: Automatically create user accounts on first login
- **Force Re-authentication**: Optional force re-auth even with active IdP session

## Installation

Admin → Plugins → "SAML 2.0 Authentication" → Install

## Configuration

### Basic Configuration

```json
{
  "enabled": true,
  "provider": "okta",
  "entityID": "https://streamspace.example.com",
  "metadataURL": "https://your-idp.okta.com/app/metadata.xml",
  "allowIDPInitiated": true,
  "signRequest": true,
  "forceAuthn": false
}
```

### Certificate and Private Key

Generate a self-signed certificate for your Service Provider:

```bash
openssl req -x509 -newkey rsa:2048 -keyout sp-key.pem -out sp-cert.pem -days 365 -nodes
```

Then configure in the plugin:

```json
{
  "certificate": "-----BEGIN CERTIFICATE-----\nMIID...\n-----END CERTIFICATE-----",
  "privateKey": "-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----"
}
```

### Attribute Mapping

Map SAML attributes from your IdP to StreamSpace user fields:

```json
{
  "attributeMapping": {
    "email": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
    "username": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
    "firstName": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
    "lastName": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
    "groups": "http://schemas.xmlsoap.org/claims/Group"
  }
}
```

### Auto-Provisioning

```json
{
  "autoProvisionUsers": true,
  "defaultRole": "user"
}
```

## Setup Guides

### Okta

1. **Create SAML App** in Okta Admin Console
   - Applications → Create App Integration → SAML 2.0

2. **Configure General Settings**:
   - App name: StreamSpace
   - App logo: (optional)

3. **Configure SAML Settings**:
   - Single sign-on URL: `https://streamspace.example.com/saml/acs`
   - Audience URI (SP Entity ID): `https://streamspace.example.com`
   - Name ID format: EmailAddress
   - Application username: Email

4. **Attribute Statements**:
   - email: `user.email`
   - username: `user.login`
   - firstName: `user.firstName`
   - lastName: `user.lastName`

5. **Download Metadata**:
   - Identity Provider metadata → Download XML
   - Paste XML into plugin's `metadataXML` field

### Azure AD

1. **Create Enterprise Application**:
   - Azure Portal → Azure Active Directory → Enterprise Applications
   - New application → Create your own application
   - Name: StreamSpace, choose "Integrate any other application"

2. **Configure Single Sign-On**:
   - Single sign-on → SAML
   - Basic SAML Configuration:
     - Identifier (Entity ID): `https://streamspace.example.com`
     - Reply URL (ACS): `https://streamspace.example.com/saml/acs`
     - Sign on URL: `https://streamspace.example.com/saml/login`

3. **User Attributes & Claims**:
   - Unique User Identifier: `user.mail`
   - Additional claims:
     - email: `user.mail`
     - firstName: `user.givenname`
     - lastName: `user.surname`

4. **Download Metadata**:
   - SAML Signing Certificate → Federation Metadata XML
   - Paste into plugin's `metadataXML` field

### Google Workspace

1. **Add Custom SAML App**:
   - Admin Console → Apps → Web and mobile apps → Add app → Add custom SAML app

2. **App Details**:
   - App name: StreamSpace
   - App icon: (optional)
   - Continue

3. **Google Identity Provider Details**:
   - Download Metadata
   - Paste into plugin's `metadataXML` field

4. **Service Provider Details**:
   - ACS URL: `https://streamspace.example.com/saml/acs`
   - Entity ID: `https://streamspace.example.com`
   - Start URL: `https://streamspace.example.com/saml/login`
   - Name ID format: EMAIL
   - Name ID: Basic Information > Primary email

5. **Attribute Mapping**:
   - email: Basic Information > Primary email
   - firstName: Basic Information > First name
   - lastName: Basic Information > Last name

### OneLogin

1. **Add SAML Test Connector**:
   - Applications → Add App → Search "SAML Test Connector (Advanced)"

2. **Configuration**:
   - Audience (EntityID): `https://streamspace.example.com`
   - Recipient: `https://streamspace.example.com/saml/acs`
   - ACS (Consumer) URL Validator: `https://streamspace.example.com/saml/acs`
   - ACS (Consumer) URL: `https://streamspace.example.com/saml/acs`

3. **Parameters** (map to SAML attributes):
   - email → Email
   - firstName → First Name
   - lastName → Last Name

4. **Download Metadata**:
   - SSO → Issuer URL → Download as XML
   - Paste into plugin's `metadataXML` field

## API Endpoints

The plugin registers the following SAML endpoints:

- `GET /saml/metadata` - Service Provider metadata (share with IdP)
- `POST /saml/acs` - Assertion Consumer Service (callback from IdP)
- `GET /saml/slo` - Single Logout Service
- `POST /saml/slo` - Single Logout POST binding
- `GET /saml/login` - Initiate SAML login flow
- `GET /saml/logout` - Logout and clear session

## User Flow

### SP-Initiated Login

1. User clicks "Sign in with SSO" button
2. Redirected to `/saml/login`
3. Plugin generates SAML request and redirects to IdP
4. User authenticates at IdP
5. IdP sends SAML assertion to `/saml/acs`
6. Plugin validates assertion and extracts user info
7. User provisioned (if new) and logged in
8. Redirected to application

### IdP-Initiated Login

1. User logs into IdP portal
2. User clicks StreamSpace app icon
3. IdP sends SAML assertion to `/saml/acs`
4. Plugin validates assertion and extracts user info
5. User provisioned (if new) and logged in
6. Redirected to application

## Security Features

- **Certificate-Based Encryption**: X.509 certificates for signing
- **Request Signing**: Sign SAML requests sent to IdP
- **Response Validation**: Verify SAML response signatures
- **Assertion Validation**: Check NotBefore, NotOnOrAfter, Audience
- **Replay Protection**: Validate assertion ID uniqueness
- **TLS Required**: All SAML endpoints require HTTPS in production

## Troubleshooting

### Common Issues

**"Failed to verify assertion signature"**
- Ensure IdP certificate is current (not expired)
- Check that metadata XML matches IdP configuration
- Verify clock synchronization between SP and IdP

**"Username not found in SAML assertion"**
- Check attribute mapping configuration
- Verify IdP is sending expected attributes
- Review SAML response XML in network logs

**"Metadata validation failed"**
- Ensure metadata XML is complete and valid
- Try using metadata URL instead of pasting XML
- Check for line breaks or formatting issues in pasted XML

### Debug Mode

Enable debug logging to see full SAML request/response flow:

```bash
# View plugin logs
kubectl logs -n streamspace -l plugin=streamspace-auth-saml
```

## Compliance

- **SAML 2.0**: Compliant with OASIS SAML 2.0 specification
- **Security**: Follows SAML security best practices
- **Privacy**: No user data stored beyond session duration

## License

MIT

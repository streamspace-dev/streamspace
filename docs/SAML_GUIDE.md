# SAML SSO Integration Guide

This guide explains how to configure SAML-based Single Sign-On (SSO) for StreamSpace.

## Overview

StreamSpace supports SAML 2.0 authentication with multiple identity providers:
- **Okta**
- **Azure AD** (Microsoft Entra ID)
- **Google Workspace**
- **Auth0**
- **Keycloak**
- **Authentik**
- **Generic SAML 2.0** providers

## Architecture

```
┌──────────┐         ┌────────────┐         ┌─────────┐
│ End User │ ──────> │ StreamSpace│ ──────> │   IdP   │
│ (Browser)│ <────── │    API     │ <────── │ (SAML)  │
└──────────┘         └────────────┘         └─────────┘
     │                     │
     │   SAML Assertion    │
     └─────────────────────┘
```

**Flow:**
1. User accesses StreamSpace UI
2. UI redirects to `/saml/login`
3. API initiates SAML authentication with IdP
4. IdP authenticates user and returns SAML assertion
5. API validates assertion and creates session
6. User is redirected back to StreamSpace UI

## Quick Start

### 1. Enable SAML in Helm Chart

```yaml
# values.yaml
api:
  auth:
    mode: saml  # or 'hybrid' for both JWT and SAML

    saml:
      enabled: true
      provider: okta  # or azuread, google, etc.
      entityID: https://streamspace.example.com
      metadataURL: https://your-idp.example.com/metadata
```

### 2. Deploy or Upgrade

```bash
helm upgrade streamspace ./chart \
  --namespace streamspace \
  --values values.yaml
```

### 3. Configure Your IdP

Add StreamSpace as a SAML application in your IdP with:
- **ACS URL**: `https://streamspace.example.com/saml/acs`
- **Entity ID**: `https://streamspace.example.com`
- **Metadata URL**: `https://streamspace.example.com/saml/metadata`

## Provider-Specific Setup

### Okta

**1. Create SAML App Integration in Okta**
- Navigate to Applications > Create App Integration
- Select SAML 2.0
- App name: StreamSpace

**2. Configure SAML Settings**
- **Single sign-on URL**: `https://streamspace.example.com/saml/acs`
- **Audience URI**: `https://streamspace.example.com`
- **Name ID format**: EmailAddress
- **Application username**: Email

**3. Attribute Statements**
```
email     -> user.email
firstName -> user.firstName
lastName  -> user.lastName
groups    -> user.groups
```

**4. Get Metadata URL**
- Go to Sign On tab
- Copy "Metadata URL"

**5. Helm Configuration**
```yaml
api:
  auth:
    mode: saml
    saml:
      enabled: true
      provider: okta
      entityID: https://streamspace.example.com
      metadataURL: https://your-domain.okta.com/app/abc123/sso/saml/metadata

      okta:
        domain: your-domain.okta.com
        appID: abc123
```

### Azure AD (Microsoft Entra ID)

**1. Register Enterprise Application**
- Azure Portal > Enterprise Applications > New application
- Create your own application > SAML-based SSO

**2. Configure SAML**
- **Identifier (Entity ID)**: `https://streamspace.example.com`
- **Reply URL (ACS)**: `https://streamspace.example.com/saml/acs`

**3. Attributes & Claims**
```
http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress -> user.mail
http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name -> user.userprincipalname
http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname -> user.givenname
http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname -> user.surname
http://schemas.microsoft.com/ws/2008/06/identity/claims/groups -> user.groups
```

**4. Download Federation Metadata XML**
- Save the XML file

**5. Helm Configuration**
```yaml
api:
  auth:
    mode: saml
    saml:
      enabled: true
      provider: azuread
      entityID: https://streamspace.example.com
      metadataXML: |
        <?xml version="1.0" encoding="UTF-8"?>
        <EntityDescriptor...>
        ...paste metadata XML here...
        </EntityDescriptor>

      azuread:
        tenantID: your-tenant-id
```

### Google Workspace

**1. Set up SAML App**
- Admin Console > Apps > Web and mobile apps
- Add custom SAML app

**2. Google IdP Information**
- Download Metadata or copy SSO URL and Certificate

**3. Service Provider Details**
- **ACS URL**: `https://streamspace.example.com/saml/acs`
- **Entity ID**: `https://streamspace.example.com`
- **Start URL**: `https://streamspace.example.com`
- **Name ID format**: EMAIL

**4. Attribute Mapping**
```
email     -> Primary email
firstName -> First name
lastName  -> Last name
```

**5. Helm Configuration**
```yaml
api:
  auth:
    mode: saml
    saml:
      enabled: true
      provider: google
      entityID: https://streamspace.example.com
      metadataURL: https://accounts.google.com/o/saml2/idp?idpid=YOUR_IDP_ID

      google:
        idpID: YOUR_IDP_ID
```

### Keycloak

**1. Create SAML Client**
- Clients > Create
- Client Protocol: saml
- Client ID: `https://streamspace.example.com`

**2. Configure Client**
- **Valid Redirect URIs**: `https://streamspace.example.com/saml/acs`
- **Base URL**: `https://streamspace.example.com`
- **IDP Initiated SSO URL Name**: streamspace
- **Name ID Format**: email

**3. Mappers**
Add mappers for email, username, firstName, lastName, groups

**4. Get Metadata**
- Installation tab > SAML Metadata IDPSSODescriptor
- Copy the URL

**5. Helm Configuration**
```yaml
api:
  auth:
    mode: saml
    saml:
      enabled: true
      provider: keycloak
      entityID: https://streamspace.example.com
      metadataURL: https://keycloak.example.com/auth/realms/myrealm/protocol/saml/descriptor

      keycloak:
        domain: keycloak.example.com
        realm: myrealm
```

### Authentik

**1. Create Provider**
- Applications > Providers > Create
- Type: SAML Provider
- Name: StreamSpace

**2. Configure Provider**
- **ACS URL**: `https://streamspace.example.com/saml/acs`
- **Issuer**: `https://streamspace.example.com`
- **Service Provider Binding**: Post

**3. Property Mappings**
Select default property mappings or create custom ones

**4. Create Application**
- Applications > Create
- Name: StreamSpace
- Provider: Select created provider
- Launch URL: `https://streamspace.example.com`

**5. Helm Configuration**
```yaml
api:
  auth:
    mode: saml
    saml:
      enabled: true
      provider: authentik
      entityID: https://streamspace.example.com
      metadataURL: https://authentik.example.com/application/saml/streamspace/metadata/

      authentik:
        domain: authentik.example.com
        slug: streamspace
```

## Advanced Configuration

### Hybrid Mode (JWT + SAML)

Support both local authentication and SSO:

```yaml
api:
  auth:
    mode: hybrid  # Supports both JWT and SAML

    jwt:
      secret: your-secret-key
      expiration: 24h

    saml:
      enabled: true
      provider: okta
      # ... SAML config
```

Users can choose:
- **SSO Login**: `/saml/login`
- **Local Login**: `/api/auth/login` (username/password)

### Custom Attribute Mapping

Override default attribute names:

```yaml
api:
  auth:
    saml:
      attributeMapping:
        email: http://custom.domain/claims/email
        username: http://custom.domain/claims/username
        firstName: http://custom.domain/claims/givenname
        lastName: http://custom.domain/claims/surname
        groups: http://custom.domain/claims/groups
```

### Request Signing

Generate certificate and key for signing SAML requests:

```bash
# Generate private key
openssl genrsa -out saml.key 2048

# Generate certificate
openssl req -new -x509 -key saml.key -out saml.crt -days 3650 \
  -subj "/CN=streamspace.example.com"

# Create Kubernetes secret
kubectl create secret generic streamspace-saml \
  --from-file=saml-cert=saml.crt \
  --from-file=saml-key=saml.key \
  -n streamspace
```

Configure in Helm:

```yaml
api:
  auth:
    saml:
      signRequest: true
      existingSecret: streamspace-saml
      existingSecretCertKey: saml-cert
      existingSecretKeyKey: saml-key
```

### Force Re-authentication

Require users to re-authenticate even if they have an active IdP session:

```yaml
api:
  auth:
    saml:
      forceAuthn: true
```

### Disable IdP-Initiated SSO

Only allow SP-initiated flows:

```yaml
api:
  auth:
    saml:
      allowIDPInitiated: false
```

## Testing

### 1. Access Metadata

Verify your SP metadata is accessible:

```bash
curl https://streamspace.example.com/saml/metadata
```

### 2. Initiate SSO

Navigate to: `https://streamspace.example.com/saml/login`

### 3. Check Logs

View API logs for SAML authentication events:

```bash
kubectl logs -n streamspace deploy/streamspace-api -f | grep -i saml
```

### 4. Verify Session

After successful login, verify user session:

```bash
curl -H "Cookie: saml_session=..." \
  https://streamspace.example.com/api/auth/me
```

## Troubleshooting

### Issue: Metadata URL not accessible

**Error**: `Failed to fetch IdP metadata`

**Solution**:
- Verify the metadata URL is correct
- Check network connectivity from API pod to IdP
- Try using `metadataXML` instead of `metadataURL`

### Issue: Invalid signature

**Error**: `Signature verification failed`

**Solution**:
- Verify certificate configuration
- Check that IdP certificate matches the one in metadata
- Ensure time synchronization (NTP) between SP and IdP

### Issue: Attribute not found

**Error**: `username not found in SAML assertion`

**Solution**:
- Check attribute mapping configuration
- Verify IdP sends the required attributes
- Review assertion in logs to see actual attribute names

### Issue: Redirect loop

**Error**: Browser keeps redirecting between StreamSpace and IdP

**Solution**:
- Check `entityID` and `acsURL` match IdP configuration
- Verify cookie domain settings
- Check for path mismatches in redirect URLs

### Issue: Certificate expired

**Error**: `Certificate has expired`

**Solution**:
- Generate new certificate and key
- Update secret: `kubectl create secret generic streamspace-saml ...`
- Restart API pods

## Security Best Practices

1. **Use HTTPS**: Always use TLS for production deployments
2. **Validate Assertions**: Ensure `signRequest: true` and proper certificate validation
3. **Short Session TTL**: Set reasonable session expiration times
4. **Rotate Certificates**: Regularly rotate SAML certificates (annually recommended)
5. **Audit Logging**: Enable audit logs for authentication events
6. **Network Policies**: Restrict API access to required endpoints only
7. **Secrets Management**: Use Kubernetes secrets or external secret managers

## Migration from JWT to SAML

**Step 1**: Enable hybrid mode

```yaml
api:
  auth:
    mode: hybrid  # Allows both JWT and SAML
```

**Step 2**: Deploy and test SAML

Verify SAML login works for test users

**Step 3**: Migrate users

Communicate migration timeline to users

**Step 4**: Switch to SAML-only

```yaml
api:
  auth:
    mode: saml  # Disable JWT
```

## Support

For issues or questions:
- **Documentation**: https://docs.streamspace.io
- **GitHub Issues**: https://github.com/streamspace/streamspace/issues
- **Community**: https://discord.gg/streamspace

## References

- [SAML 2.0 Specification](http://docs.oasis-open.org/security/saml/Post2.0/sstc-saml-tech-overview-2.0.html)
- [Okta SAML Documentation](https://developer.okta.com/docs/guides/build-sso-integration/saml2/main/)
- [Azure AD SAML Tutorial](https://learn.microsoft.com/en-us/azure/active-directory/manage-apps/add-application-portal-setup-sso)
- [Google Workspace SAML](https://support.google.com/a/answer/6087519)

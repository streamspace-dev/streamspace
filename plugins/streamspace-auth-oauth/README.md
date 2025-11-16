# StreamSpace OAuth2 / OIDC Authentication Plugin

Modern authentication using OAuth2 and OpenID Connect protocols. Supports Google, GitHub, GitLab, Okta, Azure AD, Auth0, Keycloak, and any custom OIDC provider.

## Features

- **OAuth2 / OIDC Standards**: Full OAuth 2.0 and OpenID Connect 1.0 support
- **Major Providers**: Pre-configured for Google, GitHub, GitLab, Okta, Azure AD, Auth0, Keycloak
- **Automatic Discovery**: OIDC discovery for automatic endpoint configuration
- **Flexible Claims**: Map any OIDC claim to user fields
- **Auto-Provisioning**: Automatically create user accounts on first login
- **Multi-Provider**: Support multiple OAuth providers simultaneously

## Installation

Admin → Plugins → "OAuth2 / OIDC Authentication" → Install

## Configuration

### Google

```json
{
  "enabled": true,
  "provider": "google",
  "providerURL": "https://accounts.google.com",
  "clientID": "your-client-id.apps.googleusercontent.com",
  "clientSecret": "your-client-secret",
  "redirectURI": "https://streamspace.example.com/oauth/callback",
  "scopes": ["openid", "profile", "email"],
  "autoProvisionUsers": true,
  "defaultRole": "user"
}
```

### GitHub

```json
{
  "enabled": true,
  "provider": "github",
  "providerURL": "https://token.actions.githubusercontent.com",
  "clientID": "your-github-client-id",
  "clientSecret": "your-github-client-secret",
  "redirectURI": "https://streamspace.example.com/oauth/callback",
  "scopes": ["read:user", "user:email"]
}
```

### Azure AD

```json
{
  "enabled": true,
  "provider": "azure-ad",
  "providerURL": "https://login.microsoftonline.com/YOUR_TENANT_ID/v2.0",
  "clientID": "your-application-id",
  "clientSecret": "your-client-secret",
  "redirectURI": "https://streamspace.example.com/oauth/callback",
  "scopes": ["openid", "profile", "email"]
}
```

### Okta

```json
{
  "enabled": true,
  "provider": "okta",
  "providerURL": "https://your-domain.okta.com/oauth2/default",
  "clientID": "your-okta-client-id",
  "clientSecret": "your-okta-client-secret",
  "redirectURI": "https://streamspace.example.com/oauth/callback",
  "scopes": ["openid", "profile", "email", "groups"]
}
```

## API Endpoints

- `GET /oauth/login?provider=google` - Initiate OAuth login flow
- `GET /oauth/callback` - OAuth callback endpoint (set as redirect URI in provider)
- `GET /oauth/logout` - Logout and clear session

## License

MIT

# User and Group Management

This document describes the user and group management system in StreamSpace.

## Overview

The user and group management system provides:
- Multi-provider authentication (local, SAML, OIDC)
- User and group CRUD operations
- Resource quota management (sessions, CPU, memory, storage)
- Role-based access control (user, operator, admin)
- JWT-based API authentication
- Group hierarchies and membership management

## Architecture

### Components

1. **Data Models** (`internal/models/user.go`)
   - User, UserQuota
   - Group, GroupQuota, GroupMembership

2. **Database Layer** (`internal/db/`)
   - `users.go`: User CRUD, authentication, quota operations
   - `groups.go`: Group CRUD, membership, quota operations

3. **API Handlers** (`internal/handlers/`)
   - `users.go`: User management endpoints
   - `groups.go`: Group management endpoints

4. **Authentication** (`internal/auth/`)
   - `jwt.go`: JWT token generation and validation
   - `middleware.go`: Authentication middleware for Gin
   - `handlers.go`: Login, refresh, SAML endpoints

5. **Quota Enforcement** (`internal/quota/`)
   - `enforcer.go`: Quota checking and resource tracking

### Database Schema

#### Users Table
```sql
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    full_name VARCHAR(255),
    role VARCHAR(50) DEFAULT 'user',
    provider VARCHAR(50) DEFAULT 'local',
    active BOOLEAN DEFAULT true,
    password_hash VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);
```

#### User Quotas Table
```sql
CREATE TABLE user_quotas (
    user_id VARCHAR(255) PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    max_sessions INT DEFAULT 5,
    max_cpu VARCHAR(50) DEFAULT '4000m',
    max_memory VARCHAR(50) DEFAULT '16Gi',
    max_storage VARCHAR(50) DEFAULT '100Gi',
    used_sessions INT DEFAULT 0,
    used_cpu VARCHAR(50) DEFAULT '0',
    used_memory VARCHAR(50) DEFAULT '0',
    used_storage VARCHAR(50) DEFAULT '0',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Groups Table
```sql
CREATE TABLE groups (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    type VARCHAR(50) DEFAULT 'team',
    parent_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Group Quotas Table
```sql
CREATE TABLE group_quotas (
    group_id VARCHAR(255) PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    max_sessions INT DEFAULT 20,
    max_cpu VARCHAR(50) DEFAULT '16000m',
    max_memory VARCHAR(50) DEFAULT '64Gi',
    max_storage VARCHAR(50) DEFAULT '500Gi',
    used_sessions INT DEFAULT 0,
    used_cpu VARCHAR(50) DEFAULT '0',
    used_memory VARCHAR(50) DEFAULT '0',
    used_storage VARCHAR(50) DEFAULT '0',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

#### Group Memberships Table
```sql
CREATE TABLE group_memberships (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE CASCADE,
    group_id VARCHAR(255) REFERENCES groups(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'member',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, group_id)
);
```

## API Endpoints

### Authentication

#### POST /api/v1/auth/login
Login with username and password (local auth).

**Request:**
```json
{
  "username": "john.doe",
  "password": "securepassword"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": "2025-11-15T10:30:00Z",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "john.doe",
    "email": "john@example.com",
    "fullName": "John Doe",
    "role": "user",
    "provider": "local",
    "active": true,
    "createdAt": "2025-11-14T10:00:00Z",
    "updatedAt": "2025-11-14T10:00:00Z"
  }
}
```

#### POST /api/v1/auth/refresh
Refresh an expiring JWT token.

**Request:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response:** Same as login response with new token.

#### POST /api/v1/auth/logout
Logout (client-side token removal).

**Response:**
```json
{
  "message": "Logged out successfully"
}
```

### User Management

#### GET /api/v1/users
List all users (admin/operator only).

**Query Parameters:**
- `role`: Filter by role (user, operator, admin)
- `provider`: Filter by provider (local, saml, oidc)
- `active`: Filter by active status (true, false)

**Response:**
```json
{
  "users": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "username": "john.doe",
      "email": "john@example.com",
      "fullName": "John Doe",
      "role": "user",
      "provider": "local",
      "active": true,
      "createdAt": "2025-11-14T10:00:00Z"
    }
  ],
  "total": 1
}
```

#### POST /api/v1/users
Create a new user (admin only).

**Request:**
```json
{
  "username": "jane.smith",
  "email": "jane@example.com",
  "fullName": "Jane Smith",
  "role": "user",
  "provider": "local",
  "password": "securepassword"
}
```

**Response:** User object (without password).

#### GET /api/v1/users/me
Get current authenticated user.

**Headers:** `Authorization: Bearer <token>`

**Response:** User object with quota.

#### GET /api/v1/users/:id
Get user by ID.

**Response:** User object with quota.

#### PATCH /api/v1/users/:id
Update user information.

**Request:**
```json
{
  "fullName": "Jane Doe",
  "email": "jane.doe@example.com",
  "role": "operator",
  "active": false
}
```

**Response:** Updated user object.

#### DELETE /api/v1/users/:id
Delete a user (admin only).

**Response:**
```json
{
  "message": "User deleted successfully"
}
```

#### GET /api/v1/users/:id/quota
Get user's resource quota.

**Response:**
```json
{
  "userId": "550e8400-e29b-41d4-a716-446655440000",
  "maxSessions": 5,
  "maxCpu": "4000m",
  "maxMemory": "16Gi",
  "maxStorage": "100Gi",
  "usedSessions": 2,
  "usedCpu": "2000m",
  "usedMemory": "4Gi",
  "usedStorage": "20Gi"
}
```

#### PUT /api/v1/users/:id/quota
Set user's resource quota (admin only).

**Request:**
```json
{
  "maxSessions": 10,
  "maxCpu": "8000m",
  "maxMemory": "32Gi",
  "maxStorage": "200Gi"
}
```

**Response:** Updated quota object.

#### GET /api/v1/users/:id/groups
Get user's group memberships.

**Response:**
```json
{
  "groups": [
    {
      "id": "group-1",
      "name": "engineering",
      "displayName": "Engineering Team",
      "type": "team",
      "role": "member"
    }
  ],
  "total": 1
}
```

### Group Management

#### GET /api/v1/groups
List all groups.

**Query Parameters:**
- `type`: Filter by type (team, department, project)
- `parentId`: Filter by parent group

**Response:**
```json
{
  "groups": [
    {
      "id": "group-1",
      "name": "engineering",
      "displayName": "Engineering Team",
      "description": "Software engineering team",
      "type": "team",
      "parentId": null,
      "createdAt": "2025-11-14T10:00:00Z",
      "updatedAt": "2025-11-14T10:00:00Z"
    }
  ],
  "total": 1
}
```

#### POST /api/v1/groups
Create a new group (admin only).

**Request:**
```json
{
  "name": "engineering",
  "displayName": "Engineering Team",
  "description": "Software engineering team",
  "type": "team",
  "parentId": null
}
```

**Response:** Group object.

#### GET /api/v1/groups/:id
Get group by ID.

**Response:** Group object with quota.

#### PATCH /api/v1/groups/:id
Update group information (admin only).

**Request:**
```json
{
  "displayName": "Updated Team Name",
  "description": "New description"
}
```

**Response:** Updated group object.

#### DELETE /api/v1/groups/:id
Delete a group (admin only).

**Response:**
```json
{
  "message": "Group deleted successfully"
}
```

#### GET /api/v1/groups/:id/members
Get group members.

**Response:**
```json
{
  "members": [
    {
      "user": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "username": "john.doe",
        "email": "john@example.com",
        "fullName": "John Doe"
      },
      "role": "member",
      "joinedAt": "2025-11-14T10:00:00Z"
    }
  ],
  "total": 1
}
```

#### POST /api/v1/groups/:id/members
Add a member to a group (admin/group owner only).

**Request:**
```json
{
  "userId": "550e8400-e29b-41d4-a716-446655440000",
  "role": "member"
}
```

**Response:**
```json
{
  "message": "User added to group successfully"
}
```

#### DELETE /api/v1/groups/:id/members/:userId
Remove a member from a group (admin/group owner only).

**Response:**
```json
{
  "message": "User removed from group successfully"
}
```

#### PATCH /api/v1/groups/:id/members/:userId
Update member's role in a group (admin/group owner only).

**Request:**
```json
{
  "role": "admin"
}
```

**Response:**
```json
{
  "message": "Member role updated successfully"
}
```

#### GET /api/v1/groups/:id/quota
Get group's resource quota.

**Response:**
```json
{
  "groupId": "group-1",
  "maxSessions": 20,
  "maxCpu": "16000m",
  "maxMemory": "64Gi",
  "maxStorage": "500Gi",
  "usedSessions": 5,
  "usedCpu": "5000m",
  "usedMemory": "10Gi",
  "usedStorage": "50Gi"
}
```

#### PUT /api/v1/groups/:id/quota
Set group's resource quota (admin only).

**Request:**
```json
{
  "maxSessions": 50,
  "maxCpu": "32000m",
  "maxMemory": "128Gi",
  "maxStorage": "1Ti"
}
```

**Response:** Updated quota object.

## Quota Enforcement

### How Quotas Work

1. **Default Quotas**: Created automatically when user is created
   - Users: 5 sessions, 4 CPU cores, 16GB RAM, 100GB storage
   - Groups: 20 sessions, 16 CPU cores, 64GB RAM, 500GB storage

2. **Quota Checking**: Occurs before session creation
   - Checks user quota first
   - Optionally checks group quota
   - Rejects session creation if quota exceeded

3. **Usage Tracking**: Updated on session creation/deletion
   - Incremented when session created
   - Decremented when session deleted
   - Stored in Kubernetes-style format (e.g., "2Gi", "1000m")

4. **Resource Parsing**: Supports multiple formats
   - CPU: "1000m" (millicores), "2" (cores)
   - Memory: "2Gi", "512Mi", "1G", "500M"
   - Storage: "50Gi", "100G", "1Ti"

### Quota Check Example

When creating a session:

```go
// Check user quota
quotaReq := &quota.SessionRequest{
    UserID:  "user-id",
    Memory:  "2Gi",
    CPU:     "1000m",
    Storage: "50Gi",
}

result, err := quotaEnforcer.CheckSessionQuota(ctx, quotaReq)
if !result.Allowed {
    // Quota exceeded, return error with details
    return HTTP 403 {
        "error": "Quota exceeded",
        "message": result.Reason,
        "quota": {
            "current": result.CurrentUsage,
            "requested": result.RequestedUsage,
            "available": result.AvailableQuota
        }
    }
}

// Quota OK, create session
// ...

// Update quota usage
quotaEnforcer.UpdateSessionQuota(ctx, userID, memory, cpu, storage, true)
```

## Authentication Flow

### Local Authentication

1. User sends username/password to POST /api/v1/auth/login
2. Server verifies password with bcrypt
3. Server generates JWT token with user claims
4. Client stores token and includes in future requests
5. Middleware validates token on protected routes

### JWT Token Structure

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "john.doe",
  "email": "john@example.com",
  "role": "user",
  "groups": ["group-1", "group-2"],
  "iss": "streamspace-api",
  "sub": "550e8400-e29b-41d4-a716-446655440000",
  "iat": 1700000000,
  "exp": 1700086400,
  "nbf": 1700000000
}
```

### SAML Authentication (Planned)

1. User clicks "Login with SAML"
2. Server redirects to SAML IdP
3. User authenticates with IdP
4. IdP sends SAML assertion to callback URL
5. Server validates assertion and extracts user attributes
6. Server creates/updates user in database
7. Server generates JWT token
8. Client stores token

## Role-Based Access Control

### Roles

- **user**: Standard user, can manage own sessions
- **operator**: Can view all sessions and users
- **admin**: Full access to all resources

### Permission Matrix

| Action                  | User | Operator | Admin |
|------------------------|------|----------|-------|
| View own profile       | ✓    | ✓        | ✓     |
| Create own sessions    | ✓    | ✓        | ✓     |
| View own sessions      | ✓    | ✓        | ✓     |
| Delete own sessions    | ✓    | ✓        | ✓     |
| View all users         | ✗    | ✓        | ✓     |
| View all sessions      | ✗    | ✓        | ✓     |
| Create users           | ✗    | ✗        | ✓     |
| Update users           | ✗    | ✗        | ✓     |
| Delete users           | ✗    | ✗        | ✓     |
| Manage quotas          | ✗    | ✗        | ✓     |
| Create groups          | ✗    | ✗        | ✓     |
| Manage group members   | ✗    | ✗        | ✓     |

### Middleware Usage

```go
// Require authentication
router.Use(auth.Middleware(jwtManager, userDB))

// Require admin role
router.Use(auth.RequireRole("admin"))

// Require admin or operator
router.Use(auth.RequireAnyRole("admin", "operator"))

// Optional authentication (allow both authenticated and anonymous)
router.Use(auth.OptionalAuth(jwtManager, userDB))
```

## Configuration

### Environment Variables

- `JWT_SECRET`: Secret key for JWT signing (change in production!)
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`: Database connection
- `API_PORT`: API server port (default: 8000)

### Default Admin User

Created during database migration:
- Username: `admin`
- Email: `admin@streamspace.local`
- Role: `admin`
- Quota: 100 sessions, 64 cores, 256GB RAM, 1TB storage

**Note:** Set password after first deployment!

## Usage Examples

### Creating a User with cURL

```bash
curl -X POST http://localhost:8000/api/v1/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "username": "jane.smith",
    "email": "jane@example.com",
    "fullName": "Jane Smith",
    "role": "user",
    "provider": "local",
    "password": "securepassword"
  }'
```

### Logging In

```bash
TOKEN=$(curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "jane.smith",
    "password": "securepassword"
  }' | jq -r '.token')
```

### Creating a Session (with Quota Check)

```bash
curl -X POST http://localhost:8000/api/v1/sessions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "user": "jane.smith",
    "template": "firefox-browser",
    "resources": {
      "memory": "2Gi",
      "cpu": "1000m"
    }
  }'
```

### Setting User Quota

```bash
curl -X PUT http://localhost:8000/api/v1/users/jane-id/quota \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "maxSessions": 10,
    "maxCpu": "8000m",
    "maxMemory": "32Gi",
    "maxStorage": "200Gi"
  }'
```

## Best Practices

1. **Passwords**: Use strong passwords (minimum 8 characters, enforced in API)
2. **JWT Secret**: Change `JWT_SECRET` in production, use a long random string
3. **Token Storage**: Store JWT tokens securely (httpOnly cookies recommended for web apps)
4. **Token Refresh**: Implement automatic token refresh before expiration
5. **Quota Monitoring**: Monitor quota usage and adjust limits as needed
6. **RBAC**: Use roles appropriately, grant minimum necessary permissions
7. **Audit Logging**: Enable audit logging for user/group changes (future feature)
8. **SAML Setup**: Configure SAML for enterprise SSO (future feature)

## Future Enhancements

- [ ] SAML authentication integration
- [ ] OIDC authentication support
- [ ] Password reset flow
- [ ] Email verification
- [ ] Two-factor authentication (2FA)
- [ ] API rate limiting per user
- [ ] Audit log for user/group changes
- [ ] Group quota aggregation (sum of member quotas)
- [ ] Quota alerts and notifications
- [ ] User self-service quota requests

# StreamSpace Web UI

React TypeScript frontend for StreamSpace - Stream any containerized application to your browser.

## Features

### ✅ Implemented

- **User Authentication**
  - JWT-based authentication
  - Demo mode support
  - SAML/OIDC integration support
  - Token refresh and session management
- **Dashboard**
  - Session statistics
  - Active connections count
  - Recent sessions overview
- **Session Management**
  - View all user sessions
  - Start/Stop (Running ↔ Hibernated)
  - Connect to running sessions
  - Delete sessions
  - Real-time status updates (polling)
- **Template Catalog**
  - Browse installed templates
  - Browse marketplace templates
  - Create sessions from templates
  - Filter by category and tags
- **Repository Management**
  - View all template repositories
  - Add new repositories
  - Sync repositories (individual or all)
  - Delete repositories
  - Repository status tracking
- **Admin Panel**
  - User management (create, edit, delete users)
  - User quota management (sessions, CPU, memory, storage)
  - Group management (create, edit, delete groups)
  - Group member management (add, remove members)
  - Group quota management
  - Role-based access control

## Tech Stack

- **Framework**: React 18
- **Language**: TypeScript
- **Build Tool**: Vite
- **UI Library**: Material-UI (MUI) v5
- **State Management**: Zustand
- **Data Fetching**: TanStack Query (React Query)
- **Routing**: React Router v6
- **HTTP Client**: Axios

## Project Structure

```
ui/
├── public/                     # Static assets
├── src/
│   ├── components/
│   │   └── Layout.tsx         # Main layout with sidebar and app bar
│   ├── hooks/
│   │   └── useApi.ts          # React Query hooks for API
│   ├── lib/
│   │   └── api.ts             # API client with JWT auth
│   ├── pages/
│   │   ├── Login.tsx          # Login page with JWT auth
│   │   ├── Dashboard.tsx      # Overview dashboard
│   │   ├── Sessions.tsx       # Session management
│   │   ├── SessionViewer.tsx  # Session viewer
│   │   ├── Catalog.tsx        # Template catalog browser
│   │   ├── Repositories.tsx   # Repository management
│   │   └── admin/             # Admin pages (protected)
│   │       ├── Dashboard.tsx  # Admin dashboard
│   │       ├── Nodes.tsx      # Node management
│   │       ├── Quotas.tsx     # Quota management
│   │       ├── Users.tsx      # User list and management
│   │       ├── UserDetail.tsx # User detail and edit
│   │       ├── CreateUser.tsx # Create new user
│   │       ├── Groups.tsx     # Group list and management
│   │       ├── GroupDetail.tsx # Group detail and members
│   │       └── CreateGroup.tsx # Create new group
│   ├── store/
│   │   └── userStore.ts       # Zustand auth state with JWT
│   ├── App.tsx                # Main app with protected routes
│   ├── main.tsx               # Entry point
│   └── index.css              # Global styles
├── index.html                 # HTML template
├── package.json               # Dependencies
├── tsconfig.json              # TypeScript configuration
├── vite.config.ts             # Vite configuration
├── Dockerfile                 # Multi-stage production build
├── nginx.conf                 # Nginx configuration
├── .dockerignore              # Docker build exclusions
└── README.md                  # This file
```

## Development

### Prerequisites

- Node.js 18+
- npm or yarn
- StreamSpace API backend running on `http://localhost:8000`

### Installation

```bash
cd ui
npm install
```

### Running Locally

```bash
npm run dev
```

The UI will start on `http://localhost:3000` with proxy to API backend.

### Building for Production

```bash
npm run build
```

Build output will be in `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

## Configuration

### Environment Variables

Create `.env.local` for environment-specific configuration:

```bash
# API Base URL (optional, uses proxy in development)
VITE_API_URL=http://localhost:8000/api/v1
```

### Vite Proxy

Development proxy is configured in `vite.config.ts`:

```typescript
server: {
  port: 3000,
  proxy: {
    '/api': {
      target: 'http://localhost:8000',
      changeOrigin: true,
    }
  }
}
```

## Features Overview

### Login

- JWT-based authentication with username/password
- Demo mode support for development (no password required)
- SAML/OIDC SSO integration (configurable via environment)
- Token storage and automatic refresh
- Role-based access (user, operator, admin)

### Dashboard

- Session count by state (running, hibernated)
- Active connections count
- Template and repository counts
- Recent sessions list
- Real-time metrics

### Sessions Page

- View all user sessions
- Session cards with:
  - Template name
  - State and phase status
  - Resource allocation
  - Active connections
  - Access URL
- Actions:
  - **Connect**: Open session in new tab
  - **Play/Pause**: Toggle running ↔ hibernated
  - **Delete**: Remove session
- Auto-refresh every 5 seconds

### Catalog Page

Two tabs:
1. **Installed Templates**: Templates ready to use
2. **Marketplace**: Templates from repositories

Features:
- Browse templates by category
- View template details (description, tags, app type)
- Create session from template (one-click)
- Filter by category and tags

### Repositories Page

- Table view of all repositories
- Repository details:
  - Name, URL, branch
  - Sync status (pending, syncing, synced, failed)
  - Template count
  - Last sync timestamp
- Actions:
  - **Add Repository**: Add new Git repository
  - **Sync**: Trigger sync for repository
  - **Sync All**: Sync all repositories
  - **Delete**: Remove repository

### Admin Panel (Admin users only)

#### User Management
- List all users with filtering by role, provider, and status
- Create new users (local, SAML, or OIDC)
- View and edit user details
- Manage user quotas (max sessions, CPU, memory, storage)
- View user's group memberships
- Delete users

#### Group Management
- List all groups with filtering by type
- Create new groups (organization, team, project)
- View and edit group details
- Manage group members (add, remove)
- Set member roles within groups
- Manage group quotas (shared limits for all members)
- Delete groups (system groups cannot be deleted)

## API Integration

All API calls go through `src/lib/api.ts` which provides:

### Session Management
- `listSessions(user?)` - List sessions
- `getSession(id)` - Get session details
- `createSession(data)` - Create new session
- `updateSessionState(id, state)` - Update session state
- `deleteSession(id)` - Delete session
- `connectSession(id, user)` - Connect to session
- `sendHeartbeat(id, connectionId)` - Send connection heartbeat

### Template Management
- `listTemplates(category?)` - List templates
- `getTemplate(id)` - Get template details
- `createTemplate(data)` - Create template
- `deleteTemplate(id)` - Delete template

### Catalog & Repositories
- `listCatalogTemplates(category?, tag?)` - Browse catalog
- `installCatalogTemplate(id)` - Install from catalog
- `listRepositories()` - List repositories
- `addRepository(data)` - Add repository
- `syncRepository(id)` - Sync repository
- `deleteRepository(id)` - Delete repository

### Authentication
- `login(username, password)` - JWT login
- `refreshToken(token)` - Refresh JWT token
- `logout()` - Logout (clear tokens)

### User Management (Admin)
- `listUsers(role?, provider?, active?)` - List users with filters
- `getUser(id)` - Get user details
- `getCurrentUser()` - Get current authenticated user
- `createUser(data)` - Create new user
- `updateUser(id, data)` - Update user
- `deleteUser(id)` - Delete user
- `getUserQuota(id)` - Get user quota
- `setUserQuota(id, data)` - Set user quota
- `getUserGroups(id)` - Get user's groups

### Group Management (Admin)
- `listGroups(type?, parentId?)` - List groups with filters
- `getGroup(id)` - Get group details
- `createGroup(data)` - Create new group
- `updateGroup(id, data)` - Update group
- `deleteGroup(id)` - Delete group
- `getGroupMembers(id)` - Get group members
- `addGroupMember(id, data)` - Add member to group
- `removeGroupMember(id, userId)` - Remove member from group
- `updateMemberRole(id, userId, role)` - Update member's role
- `getGroupQuota(id)` - Get group quota
- `setGroupQuota(id, data)` - Set group quota

### React Query Hooks

All hooks auto-refresh and provide loading/error states:

```typescript
// Example usage
const { data: sessions, isLoading, error } = useSessions(username);
const createSession = useCreateSession();

// Mutations automatically invalidate related queries
createSession.mutate(sessionData);
```

## Roadmap

### Phase 2 UI (Complete)
- ✅ Login page (demo mode)
- ✅ Dashboard with stats
- ✅ Session management
- ✅ Template catalog browser
- ✅ Repository management
- ✅ Responsive layout
- ✅ Dark theme

### Phase 3 (Complete)
- ✅ JWT authentication
- ✅ SAML/OIDC integration support
- ✅ User management (create, edit, delete)
- ✅ User quota management
- ✅ Group management (create, edit, delete)
- ✅ Group member management
- ✅ Group quota management
- ✅ Admin panel with protected routes
- ✅ Role-based access control

### Phase 4 (Future)
- [ ] User profile and settings page
- [ ] Advanced filtering and search
- [ ] Session resource customization UI
- [ ] WebSocket real-time updates
- [ ] Pod logs viewer
- [ ] Terminal (exec into containers)
- [ ] Session sharing
- [ ] Metrics and analytics charts

### Phase 5 (Future)
- [ ] Cluster management UI
- [ ] YAML editor for resources
- [ ] Node management
- [ ] Deployment management
- [ ] Service management

## Contributing

See the main [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.

## License

MIT License - See [LICENSE](../LICENSE)

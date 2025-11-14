# Plugin API Reference

Complete API reference for StreamSpace plugin development.

**Version**: 1.0.0
**Last Updated**: 2025-11-14

---

## Table of Contents

- [Overview](#overview)
- [StreamSpace Global Object](#streamspace-global-object)
- [Session API](#session-api)
- [User API](#user-api)
- [Template API](#template-api)
- [Notification API](#notification-api)
- [Email API](#email-api)
- [Storage API](#storage-api)
- [UI API](#ui-api)
- [Command API](#command-api)
- [Logging API](#logging-api)
- [Plugin Lifecycle Hooks](#plugin-lifecycle-hooks)
- [Event Handlers](#event-handlers)
- [Plugin Context](#plugin-context)
- [Error Handling](#error-handling)

---

## Overview

The StreamSpace Plugin API provides a comprehensive set of methods and hooks for extending platform functionality. All API methods are available through the global `streamspace` object.

### API Stability

- **Stable APIs**: Marked with ✅ - Safe to use in production
- **Beta APIs**: Marked with ⚠️ - May change in future versions
- **Experimental APIs**: Marked with 🧪 - Use with caution

---

## StreamSpace Global Object

The `streamspace` object is globally available in all plugin code.

```typescript
interface StreamSpace {
  api: SessionAPI & UserAPI & TemplateAPI;
  notify: NotificationAPI;
  email: EmailAPI;
  storage: StorageAPI;
  ui: UIAPI;
  commands: CommandAPI;
  log: LoggingAPI;
  version: string;
  config: PlatformConfig;
}
```

### Version Information

```javascript
// Get StreamSpace version
const version = streamspace.version; // "1.0.0"

// Check if API is available
if (streamspace.api.getSessions) {
  // Safe to use
}
```

### Platform Configuration

```javascript
// Access platform configuration
const config = streamspace.config;

console.log(config.namespace);        // "streamspace"
console.log(config.ingressDomain);    // "streamspace.local"
console.log(config.hibernationEnabled); // true
```

---

## Session API

### ✅ getSessions()

Get all sessions in the system.

**Signature**:
```typescript
getSessions(options?: GetSessionsOptions): Promise<Session[]>

interface GetSessionsOptions {
  user?: string;           // Filter by username
  state?: SessionState;    // Filter by state
  template?: string;       // Filter by template
  limit?: number;          // Max results (default: 100)
  offset?: number;         // Pagination offset
}

type SessionState = 'running' | 'hibernated' | 'terminated';

interface Session {
  id: string;
  name: string;
  user: string;
  template: string;
  state: SessionState;
  resources: {
    memory: string;
    cpu: string;
  };
  persistentHome: boolean;
  idleTimeout: string;
  createdAt: string;
  updatedAt: string;
  url?: string;
  podName?: string;
  lastActivity?: string;
}
```

**Example**:
```javascript
// Get all sessions
const sessions = await streamspace.api.getSessions();

// Get user's sessions
const userSessions = await streamspace.api.getSessions({
  user: 'john'
});

// Get running sessions
const runningSessions = await streamspace.api.getSessions({
  state: 'running'
});

// Paginated results
const page2 = await streamspace.api.getSessions({
  limit: 20,
  offset: 20
});
```

### ✅ getSession()

Get a single session by ID.

**Signature**:
```typescript
getSession(sessionId: string): Promise<Session>
```

**Example**:
```javascript
const session = await streamspace.api.getSession('user1-firefox');

console.log(session.state);        // "running"
console.log(session.url);          // "https://user1-firefox.streamspace.local"
console.log(session.lastActivity); // "2025-11-14T10:30:00Z"
```

**Errors**:
- `SessionNotFoundError`: Session ID does not exist

### ✅ createSession()

Create a new session.

**Signature**:
```typescript
createSession(options: CreateSessionOptions): Promise<Session>

interface CreateSessionOptions {
  user: string;              // Username (required)
  template: string;          // Template name (required)
  state?: SessionState;      // Initial state (default: "running")
  resources?: {
    memory?: string;         // Memory limit (default: template default)
    cpu?: string;           // CPU limit (default: template default)
  };
  persistentHome?: boolean;  // Mount user PVC (default: true)
  idleTimeout?: string;      // Auto-hibernate timeout (default: "30m")
  maxSessionDuration?: string; // Max session lifetime (default: "8h")
  env?: Record<string, string>; // Additional environment variables
  labels?: Record<string, string>; // Additional labels
}
```

**Example**:
```javascript
// Basic session
const session = await streamspace.api.createSession({
  user: 'john',
  template: 'firefox-browser'
});

// Custom resources
const session = await streamspace.api.createSession({
  user: 'john',
  template: 'vscode',
  resources: {
    memory: '4Gi',
    cpu: '2000m'
  },
  idleTimeout: '1h'
});

// With environment variables
const session = await streamspace.api.createSession({
  user: 'john',
  template: 'jupyter',
  env: {
    JUPYTER_TOKEN: 'secret-token',
    CUSTOM_VAR: 'value'
  }
});
```

**Errors**:
- `UserNotFoundError`: User does not exist
- `TemplateNotFoundError`: Template does not exist
- `QuotaExceededError`: User has reached session limit
- `InsufficientResourcesError`: Cluster lacks resources

### ✅ updateSession()

Update an existing session.

**Signature**:
```typescript
updateSession(sessionId: string, updates: UpdateSessionOptions): Promise<Session>

interface UpdateSessionOptions {
  state?: SessionState;     // Change session state
  resources?: {
    memory?: string;
    cpu?: string;
  };
  idleTimeout?: string;
  labels?: Record<string, string>;
}
```

**Example**:
```javascript
// Hibernate session
await streamspace.api.updateSession('user1-firefox', {
  state: 'hibernated'
});

// Wake session
await streamspace.api.updateSession('user1-firefox', {
  state: 'running'
});

// Update resources
await streamspace.api.updateSession('user1-firefox', {
  resources: {
    memory: '4Gi'
  }
});

// Update idle timeout
await streamspace.api.updateSession('user1-firefox', {
  idleTimeout: '2h'
});
```

**Errors**:
- `SessionNotFoundError`: Session ID does not exist
- `InvalidStateTransitionError`: Cannot transition to requested state

### ✅ deleteSession()

Delete a session permanently.

**Signature**:
```typescript
deleteSession(sessionId: string): Promise<void>
```

**Example**:
```javascript
await streamspace.api.deleteSession('user1-firefox');
```

**Errors**:
- `SessionNotFoundError`: Session ID does not exist

### ✅ getUserSessions()

Get all sessions for a specific user.

**Signature**:
```typescript
getUserSessions(username: string): Promise<Session[]>
```

**Example**:
```javascript
const sessions = await streamspace.api.getUserSessions('john');

console.log(`${sessions.length} sessions for john`);
```

**Errors**:
- `UserNotFoundError`: User does not exist

---

## User API

### ✅ getUsers()

Get all users in the system.

**Signature**:
```typescript
getUsers(options?: GetUsersOptions): Promise<User[]>

interface GetUsersOptions {
  tier?: string;           // Filter by tier
  limit?: number;
  offset?: number;
}

interface User {
  username: string;
  fullName: string;
  email: string;
  tier: string;           // 'free' | 'pro' | 'enterprise'
  quotas: {
    memory: string;
    maxSessions: number;
    storage: string;
    maxSessionDuration: string;
  };
  createdAt: string;
  updatedAt: string;
  lastLogin?: string;
}
```

**Example**:
```javascript
// Get all users
const users = await streamspace.api.getUsers();

// Get pro users
const proUsers = await streamspace.api.getUsers({
  tier: 'pro'
});
```

### ✅ getUser()

Get a single user by username.

**Signature**:
```typescript
getUser(username: string): Promise<User>
```

**Example**:
```javascript
const user = await streamspace.api.getUser('john');

console.log(user.fullName);  // "John Doe"
console.log(user.tier);      // "pro"
console.log(user.quotas);    // { memory: "16Gi", maxSessions: 5, ... }
```

**Errors**:
- `UserNotFoundError`: Username does not exist

### ✅ createUser()

Create a new user.

**Signature**:
```typescript
createUser(options: CreateUserOptions): Promise<User>

interface CreateUserOptions {
  username: string;         // Username (required)
  fullName: string;         // Full name (required)
  email: string;           // Email address (required)
  tier?: string;           // User tier (default: "free")
  quotas?: {
    memory?: string;
    maxSessions?: number;
    storage?: string;
    maxSessionDuration?: string;
  };
}
```

**Example**:
```javascript
const user = await streamspace.api.createUser({
  username: 'jane',
  fullName: 'Jane Smith',
  email: 'jane@example.com',
  tier: 'pro',
  quotas: {
    memory: '32Gi',
    maxSessions: 10,
    storage: '200Gi',
    maxSessionDuration: '12h'
  }
});
```

**Errors**:
- `UserAlreadyExistsError`: Username already taken
- `InvalidEmailError`: Invalid email format

### ✅ updateUser()

Update an existing user.

**Signature**:
```typescript
updateUser(username: string, updates: UpdateUserOptions): Promise<User>

interface UpdateUserOptions {
  fullName?: string;
  email?: string;
  tier?: string;
  quotas?: {
    memory?: string;
    maxSessions?: number;
    storage?: string;
    maxSessionDuration?: string;
  };
}
```

**Example**:
```javascript
// Upgrade tier
await streamspace.api.updateUser('john', {
  tier: 'enterprise',
  quotas: {
    memory: '64Gi',
    maxSessions: 20
  }
});

// Update email
await streamspace.api.updateUser('john', {
  email: 'john.new@example.com'
});
```

**Errors**:
- `UserNotFoundError`: Username does not exist

### ✅ deleteUser()

Delete a user and all their sessions.

**Signature**:
```typescript
deleteUser(username: string, options?: DeleteUserOptions): Promise<void>

interface DeleteUserOptions {
  deleteSessions?: boolean;  // Delete user's sessions (default: true)
  deleteStorage?: boolean;   // Delete user's PVC (default: false)
}
```

**Example**:
```javascript
// Delete user and sessions
await streamspace.api.deleteUser('john');

// Delete user, sessions, and storage
await streamspace.api.deleteUser('john', {
  deleteSessions: true,
  deleteStorage: true
});
```

**Errors**:
- `UserNotFoundError`: Username does not exist

---

## Template API

### ✅ getTemplates()

Get all available templates.

**Signature**:
```typescript
getTemplates(options?: GetTemplatesOptions): Promise<Template[]>

interface GetTemplatesOptions {
  category?: string;       // Filter by category
  tag?: string;           // Filter by tag
  limit?: number;
  offset?: number;
}

interface Template {
  name: string;
  displayName: string;
  description: string;
  category: string;
  icon?: string;
  baseImage: string;
  defaultResources: {
    memory: string;
    cpu: string;
  };
  ports: Array<{
    name: string;
    containerPort: number;
    protocol: string;
  }>;
  env: Array<{
    name: string;
    value: string;
  }>;
  capabilities: string[];
  tags: string[];
  createdAt: string;
  updatedAt: string;
}
```

**Example**:
```javascript
// Get all templates
const templates = await streamspace.api.getTemplates();

// Filter by category
const browsers = await streamspace.api.getTemplates({
  category: 'Web Browsers'
});

// Filter by tag
const devTools = await streamspace.api.getTemplates({
  tag: 'development'
});
```

### ✅ getTemplate()

Get a single template by name.

**Signature**:
```typescript
getTemplate(templateName: string): Promise<Template>
```

**Example**:
```javascript
const template = await streamspace.api.getTemplate('firefox-browser');

console.log(template.displayName);     // "Firefox Web Browser"
console.log(template.defaultResources); // { memory: "2Gi", cpu: "1000m" }
console.log(template.capabilities);     // ["Network", "Audio", "Clipboard"]
```

**Errors**:
- `TemplateNotFoundError`: Template name does not exist

### ⚠️ createTemplate()

Create a new template (admin only).

**Signature**:
```typescript
createTemplate(options: CreateTemplateOptions): Promise<Template>

interface CreateTemplateOptions {
  name: string;              // Template name (required)
  displayName: string;       // Display name (required)
  description: string;       // Description (required)
  category: string;          // Category (required)
  baseImage: string;         // Container image (required)
  icon?: string;
  defaultResources: {
    memory: string;
    cpu: string;
  };
  ports: Array<{
    name: string;
    containerPort: number;
    protocol?: string;
  }>;
  env?: Array<{
    name: string;
    value: string;
  }>;
  capabilities?: string[];
  tags?: string[];
}
```

**Example**:
```javascript
const template = await streamspace.api.createTemplate({
  name: 'my-app',
  displayName: 'My Custom App',
  description: 'A custom application',
  category: 'Custom',
  baseImage: 'myregistry/myapp:latest',
  defaultResources: {
    memory: '4Gi',
    cpu: '2000m'
  },
  ports: [
    {
      name: 'vnc',
      containerPort: 5900,
      protocol: 'TCP'
    }
  ],
  capabilities: ['Network'],
  tags: ['custom', 'productivity']
});
```

**Errors**:
- `TemplateAlreadyExistsError`: Template name already taken
- `InvalidImageError`: Base image is invalid or inaccessible

---

## Notification API

### ✅ notify()

Send a notification to a specific user.

**Signature**:
```typescript
streamspace.notify(userId: string, notification: Notification): Promise<void>

interface Notification {
  title: string;               // Notification title (required)
  message: string;             // Notification message (required)
  type?: NotificationType;     // Notification type (default: "info")
  actions?: NotificationAction[];
  dismissible?: boolean;       // Can be dismissed (default: true)
  timeout?: number;           // Auto-dismiss after ms (default: 5000)
  persistent?: boolean;        // Show in notification center (default: true)
}

type NotificationType = 'success' | 'info' | 'warning' | 'error';

interface NotificationAction {
  label: string;
  url?: string;
  handler?: () => void;
}
```

**Example**:
```javascript
// Basic notification
await streamspace.notify('john', {
  title: 'Session Ready',
  message: 'Your Firefox session is ready to use!',
  type: 'success'
});

// With actions
await streamspace.notify('john', {
  title: 'Update Available',
  message: 'A new version of the template is available',
  type: 'info',
  actions: [
    {
      label: 'Update Now',
      url: '/sessions/update'
    },
    {
      label: 'Remind Later',
      handler: () => {
        // Schedule reminder
      }
    }
  ]
});

// Auto-dismiss
await streamspace.notify('john', {
  title: 'Session Created',
  message: 'Your session is starting...',
  type: 'info',
  timeout: 3000,
  dismissible: false
});

// Persistent notification
await streamspace.notify('john', {
  title: 'Important Update',
  message: 'Scheduled maintenance in 1 hour',
  type: 'warning',
  persistent: true,
  dismissible: true
});
```

### ✅ notifyAll()

Send a notification to all users.

**Signature**:
```typescript
streamspace.notifyAll(notification: Notification): Promise<void>
```

**Example**:
```javascript
// System-wide notification
await streamspace.notifyAll({
  title: 'System Maintenance',
  message: 'StreamSpace will be unavailable from 2-4 AM',
  type: 'warning',
  persistent: true
});
```

### ⚠️ notifyAdmins()

Send a notification to admin users only.

**Signature**:
```typescript
streamspace.notifyAdmins(notification: Notification): Promise<void>
```

**Example**:
```javascript
await streamspace.notifyAdmins({
  title: 'High Resource Usage',
  message: 'Cluster memory usage is at 85%',
  type: 'warning',
  actions: [
    {
      label: 'View Metrics',
      url: '/admin/metrics'
    }
  ]
});
```

---

## Email API

### ✅ send()

Send an email.

**Signature**:
```typescript
streamspace.email.send(options: EmailOptions): Promise<void>

interface EmailOptions {
  to: string | string[];      // Recipient(s) (required)
  subject: string;            // Subject line (required)
  template?: string;          // Email template name
  data?: Record<string, any>; // Template data
  html?: string;             // Raw HTML (if no template)
  text?: string;             // Plain text version
  from?: string;             // Sender (default: system email)
  cc?: string | string[];    // CC recipients
  bcc?: string | string[];   // BCC recipients
  attachments?: EmailAttachment[];
}

interface EmailAttachment {
  filename: string;
  content: string | Buffer;
  contentType?: string;
}
```

**Example**:
```javascript
// Using template
await streamspace.email.send({
  to: 'john@example.com',
  subject: 'Welcome to StreamSpace',
  template: 'welcome',
  data: {
    username: 'john',
    loginUrl: 'https://streamspace.local/login'
  }
});

// Raw HTML
await streamspace.email.send({
  to: ['john@example.com', 'jane@example.com'],
  subject: 'Session Report',
  html: '<h1>Your Session Report</h1><p>...</p>',
  text: 'Your Session Report\n...'
});

// With attachments
await streamspace.email.send({
  to: 'john@example.com',
  subject: 'Session Logs',
  template: 'session-logs',
  data: { sessionId: 'user1-firefox' },
  attachments: [
    {
      filename: 'session.log',
      content: logContent,
      contentType: 'text/plain'
    }
  ]
});
```

**Errors**:
- `EmailNotConfiguredError`: Email service not configured
- `InvalidEmailAddressError`: Invalid recipient email
- `TemplateNotFoundError`: Email template does not exist

---

## Storage API

Persistent key-value storage for plugin data.

### ✅ get()

Retrieve data from storage.

**Signature**:
```typescript
streamspace.storage.get(key: string): Promise<any>
```

**Example**:
```javascript
const data = await streamspace.storage.get('my-plugin-config');

if (data) {
  console.log('Config loaded:', data);
} else {
  console.log('No config found');
}
```

### ✅ set()

Store data.

**Signature**:
```typescript
streamspace.storage.set(key: string, value: any): Promise<void>
```

**Example**:
```javascript
await streamspace.storage.set('my-plugin-config', {
  lastSync: Date.now(),
  stats: {
    totalSessions: 42,
    activeUsers: 10
  }
});
```

### ✅ delete()

Delete data.

**Signature**:
```typescript
streamspace.storage.delete(key: string): Promise<void>
```

**Example**:
```javascript
await streamspace.storage.delete('my-plugin-cache');
```

### ✅ keys()

List all keys.

**Signature**:
```typescript
streamspace.storage.keys(prefix?: string): Promise<string[]>
```

**Example**:
```javascript
// All keys
const allKeys = await streamspace.storage.keys();

// Keys with prefix
const configKeys = await streamspace.storage.keys('config-');
// Returns: ['config-app', 'config-database', ...]
```

### ✅ clear()

Clear all plugin data.

**Signature**:
```typescript
streamspace.storage.clear(): Promise<void>
```

**Example**:
```javascript
// Clear all plugin storage
await streamspace.storage.clear();
```

---

## UI API

### ✅ registerWidget()

Register a dashboard widget.

**Signature**:
```typescript
streamspace.ui.registerWidget(id: string, options: WidgetOptions): void

interface WidgetOptions {
  title: string;              // Widget title (required)
  component: string;          // Component path (required)
  position?: 'top' | 'sidebar' | 'bottom'; // Position (default: "top")
  width?: 'full' | 'half' | 'third';      // Width (default: "full")
  icon?: string;             // Icon name
  permissions?: string[];    // Required permissions
}
```

**Example**:
```javascript
streamspace.ui.registerWidget('session-stats', {
  title: 'Session Statistics',
  component: './components/SessionStats.jsx',
  position: 'top',
  width: 'full',
  icon: 'bar-chart'
});
```

### ✅ addMenuItem()

Add a menu item to the navigation.

**Signature**:
```typescript
streamspace.ui.addMenuItem(options: MenuItemOptions): void

interface MenuItemOptions {
  label: string;              // Menu label (required)
  path: string;              // Route path (required)
  component: string;          // Component path (required)
  icon?: string;             // Icon name
  position?: number;         // Menu position
  permissions?: string[];    // Required permissions
}
```

**Example**:
```javascript
streamspace.ui.addMenuItem({
  label: 'Session Analytics',
  path: '/analytics',
  component: './pages/Analytics.jsx',
  icon: 'chart-line',
  position: 5
});
```

### ✅ registerSettingsPage()

Add a plugin settings page.

**Signature**:
```typescript
streamspace.ui.registerSettingsPage(id: string, options: SettingsPageOptions): void

interface SettingsPageOptions {
  title: string;              // Page title (required)
  component: string;          // Component path (required)
  icon?: string;             // Icon name
}
```

**Example**:
```javascript
streamspace.ui.registerSettingsPage('my-plugin-settings', {
  title: 'Plugin Settings',
  component: './pages/Settings.jsx',
  icon: 'cog'
});
```

### ⚠️ showDialog()

Show a modal dialog.

**Signature**:
```typescript
streamspace.ui.showDialog(options: DialogOptions): Promise<any>

interface DialogOptions {
  title: string;              // Dialog title (required)
  message: string;            // Dialog message (required)
  type?: 'info' | 'warning' | 'error' | 'confirm';
  buttons?: DialogButton[];
}

interface DialogButton {
  label: string;
  value: any;
  variant?: 'text' | 'outlined' | 'contained';
  color?: 'primary' | 'secondary' | 'error';
}
```

**Example**:
```javascript
// Confirm dialog
const confirmed = await streamspace.ui.showDialog({
  title: 'Delete Session',
  message: 'Are you sure you want to delete this session?',
  type: 'confirm',
  buttons: [
    { label: 'Cancel', value: false },
    { label: 'Delete', value: true, color: 'error' }
  ]
});

if (confirmed) {
  await streamspace.api.deleteSession(sessionId);
}

// Info dialog
await streamspace.ui.showDialog({
  title: 'Success',
  message: 'Session created successfully!',
  type: 'info'
});
```

---

## Command API

### ✅ register()

Register a custom command.

**Signature**:
```typescript
streamspace.commands.register(options: CommandOptions): void

interface CommandOptions {
  name: string;               // Command name (required)
  description: string;        // Description (required)
  handler: (args: any) => Promise<any>; // Handler function (required)
  permissions?: string[];    // Required permissions
}
```

**Example**:
```javascript
streamspace.commands.register({
  name: 'sync-templates',
  description: 'Sync templates from external source',
  handler: async (args) => {
    console.log('Syncing templates...');

    const templates = await fetchExternalTemplates();

    for (const template of templates) {
      await streamspace.api.createTemplate(template);
    }

    console.log(`Synced ${templates.length} templates`);
  },
  permissions: ['admin']
});

// Execute command
await streamspace.commands.execute('sync-templates');
```

---

## Logging API

### ✅ log(), info(), warn(), error()

Write logs.

**Signature**:
```typescript
streamspace.log.log(level: string, message: string, data?: any): void
streamspace.log.info(message: string, data?: any): void
streamspace.log.warn(message: string, data?: any): void
streamspace.log.error(message: string, data?: any): void
```

**Example**:
```javascript
// Info log
streamspace.log.info('Plugin initialized', {
  version: '1.0.0',
  config: this.config
});

// Warning log
streamspace.log.warn('High memory usage detected', {
  usage: '85%',
  threshold: '80%'
});

// Error log
streamspace.log.error('Failed to sync data', {
  error: error.message,
  stack: error.stack
});
```

---

## Plugin Lifecycle Hooks

### onLoad()

Called when the plugin is loaded.

**Signature**:
```typescript
async onLoad(): Promise<void>
```

**Example**:
```javascript
module.exports = {
  async onLoad() {
    console.log('Plugin loading...');

    // Validate configuration
    if (!this.config.apiKey) {
      throw new Error('API key is required');
    }

    // Initialize connections
    this.connection = await createConnection(this.config);

    // Register UI components
    streamspace.ui.registerWidget('my-widget', {
      title: 'My Widget',
      component: './components/Widget.jsx'
    });

    console.log('Plugin loaded successfully');
  }
};
```

### onUnload()

Called when the plugin is unloaded.

**Signature**:
```typescript
async onUnload(): Promise<void>
```

**Example**:
```javascript
module.exports = {
  connections: [],
  timers: [],

  async onUnload() {
    console.log('Plugin unloading...');

    // Close connections
    await Promise.all(
      this.connections.map(conn => conn.close())
    );

    // Clear timers
    this.timers.forEach(timer => clearInterval(timer));

    console.log('Plugin unloaded');
  }
};
```

---

## Event Handlers

### Session Events

#### onSessionCreated()

Called when a session is created.

**Signature**:
```typescript
async onSessionCreated(session: Session): Promise<void>
```

**Example**:
```javascript
async onSessionCreated(session) {
  console.log(`Session created: ${session.id}`);

  await streamspace.notify(session.user, {
    title: 'Session Created',
    message: `Your ${session.template} session is starting...`,
    type: 'info'
  });
}
```

#### onSessionStarted()

Called when a session starts running.

**Signature**:
```typescript
async onSessionStarted(session: Session): Promise<void>
```

#### onSessionStopped()

Called when a session stops.

**Signature**:
```typescript
async onSessionStopped(session: Session): Promise<void>
```

#### onSessionHibernated()

Called when a session is hibernated.

**Signature**:
```typescript
async onSessionHibernated(session: Session): Promise<void>
```

#### onSessionWoken()

Called when a session wakes from hibernation.

**Signature**:
```typescript
async onSessionWoken(session: Session): Promise<void>
```

#### onSessionDeleted()

Called when a session is deleted.

**Signature**:
```typescript
async onSessionDeleted(session: Session): Promise<void>
```

### User Events

#### onUserCreated()

Called when a user is created.

**Signature**:
```typescript
async onUserCreated(user: User): Promise<void>
```

**Example**:
```javascript
async onUserCreated(user) {
  console.log(`User created: ${user.username}`);

  // Send welcome email
  await streamspace.email.send({
    to: user.email,
    subject: 'Welcome to StreamSpace',
    template: 'welcome',
    data: { user }
  });
}
```

#### onUserUpdated()

Called when a user is updated.

**Signature**:
```typescript
async onUserUpdated(user: User): Promise<void>
```

#### onUserDeleted()

Called when a user is deleted.

**Signature**:
```typescript
async onUserDeleted(user: User): Promise<void>
```

#### onUserLogin()

Called when a user logs in.

**Signature**:
```typescript
async onUserLogin(user: User): Promise<void>
```

**Example**:
```javascript
async onUserLogin(user) {
  console.log(`User logged in: ${user.username}`);

  // Track login
  await streamspace.storage.set(`last-login:${user.username}`, Date.now());

  // Send notification
  if (this.config.notifyOnLogin) {
    await streamspace.notify(user.username, {
      title: 'Welcome Back',
      message: `Welcome back, ${user.fullName}!`,
      type: 'info',
      timeout: 3000
    });
  }
}
```

#### onUserLogout()

Called when a user logs out.

**Signature**:
```typescript
async onUserLogout(user: User): Promise<void>
```

---

## Plugin Context

The `this` context in plugin code provides access to plugin configuration and utilities.

### this.config

Plugin configuration values from user settings.

**Example**:
```javascript
module.exports = {
  async onLoad() {
    console.log('API Key:', this.config.apiKey);
    console.log('Enabled:', this.config.enabled);
  }
};
```

### this.manifest

Plugin manifest metadata.

**Example**:
```javascript
console.log(this.manifest.name);        // "my-plugin"
console.log(this.manifest.version);     // "1.0.0"
console.log(this.manifest.permissions); // ["read:sessions", ...]
```

---

## Error Handling

### Standard Errors

Plugins should handle these standard errors:

- `SessionNotFoundError`
- `UserNotFoundError`
- `TemplateNotFoundError`
- `QuotaExceededError`
- `InsufficientResourcesError`
- `PermissionDeniedError`
- `InvalidStateTransitionError`

**Example**:
```javascript
async onSessionCreated(session) {
  try {
    const user = await streamspace.api.getUser(session.user);
    // Handle user
  } catch (error) {
    if (error.name === 'UserNotFoundError') {
      console.error('User not found:', session.user);
    } else {
      throw error;
    }
  }
}
```

### Error Logging

Always log errors for debugging:

```javascript
async onSessionCreated(session) {
  try {
    await this.processSession(session);
  } catch (error) {
    streamspace.log.error('Failed to process session', {
      error: error.message,
      stack: error.stack,
      sessionId: session.id
    });

    // Optionally notify admins
    await streamspace.notifyAdmins({
      title: 'Plugin Error',
      message: `Failed to process session: ${error.message}`,
      type: 'error'
    });
  }
}
```

---

## Additional Resources

- [Plugin Development Guide](../PLUGIN_DEVELOPMENT.md)
- [Plugin Manifest Schema](PLUGIN_MANIFEST.md)
- [Example Plugins](https://github.com/streamspace/example-plugins)
- [Support](https://discord.gg/streamspace)

---

**Version**: 1.0.0
**Last Updated**: 2025-11-14

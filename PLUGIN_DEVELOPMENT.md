# Plugin Development Guide

A comprehensive guide to building plugins for StreamSpace.

**Version**: 1.0.0
**Last Updated**: 2025-11-14

---

## 📋 Table of Contents

- [Introduction](#introduction)
- [Getting Started](#getting-started)
- [Plugin Types](#plugin-types)
- [Manifest File](#manifest-file)
- [Plugin Structure](#plugin-structure)
- [Extension Plugins](#extension-plugins)
- [Webhook Plugins](#webhook-plugins)
- [API Integration Plugins](#api-integration-plugins)
- [UI Theme Plugins](#ui-theme-plugins)
- [Configuration Schema](#configuration-schema)
- [API Reference](#api-reference)
- [Testing and Debugging](#testing-and-debugging)
- [Best Practices](#best-practices)
- [Security Guidelines](#security-guidelines)
- [Publishing Plugins](#publishing-plugins)
- [Examples](#examples)

---

## Introduction

StreamSpace plugins extend the platform's functionality without modifying core code. Plugins can add new features, integrate with external services, customize workflows, and enhance the user interface.

### What You Can Build

- **Extensions**: Add new features and UI components
- **Webhooks**: React to system events (session created, user logged in, etc.)
- **API Integrations**: Connect to external services (Slack, GitHub, Jira, etc.)
- **UI Themes**: Customize the web interface appearance
- **CLI Tools**: Add new command-line utilities

### Prerequisites

- Basic JavaScript/TypeScript knowledge
- Understanding of Node.js and npm
- Familiarity with REST APIs (for API plugins)
- Git for version control

---

## Getting Started

### 1. Create Plugin Directory

```bash
mkdir my-plugin
cd my-plugin
npm init -y
```

### 2. Create Manifest File

Create `manifest.json` with basic plugin information:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "displayName": "My Plugin",
  "description": "A simple StreamSpace plugin",
  "type": "extension",
  "author": "Your Name",
  "license": "MIT",
  "permissions": ["read:sessions"],
  "entrypoints": {
    "main": "index.js"
  }
}
```

### 3. Create Entry Point

Create `index.js`:

```javascript
// index.js
module.exports = {
  async onLoad() {
    console.log('Plugin loaded!');
  },

  async onUnload() {
    console.log('Plugin unloaded!');
  }
};
```

### 4. Test Locally

```bash
# Package plugin
tar -czf my-plugin.tar.gz manifest.json index.js

# Test in StreamSpace (see Testing section)
```

---

## Plugin Types

### Extension

Add new features and extend existing functionality.

**Use Cases**:
- Custom dashboard widgets
- New session management features
- Enhanced user profiles
- Custom reports

**Example**:
```javascript
module.exports = {
  async onLoad() {
    // Register custom widget
    streamspace.ui.registerWidget('my-widget', {
      title: 'My Custom Widget',
      component: './components/MyWidget.jsx'
    });
  }
};
```

### Webhook

React to system events in real-time.

**Available Events**:
- `session.created`
- `session.started`
- `session.stopped`
- `session.hibernated`
- `session.woken`
- `session.deleted`
- `user.created`
- `user.updated`
- `user.deleted`
- `user.login`
- `user.logout`

**Example**:
```javascript
module.exports = {
  async onSessionCreated(session) {
    console.log(`Session ${session.id} created for user ${session.user}`);

    // Send notification
    await streamspace.notify(session.user, {
      title: 'Session Created',
      message: `Your ${session.template} session is ready!`
    });
  },

  async onUserLogin(user) {
    console.log(`User ${user.username} logged in`);
  }
};
```

### API Integration

Connect StreamSpace to external services.

**Use Cases**:
- Slack notifications
- GitHub integration
- Jira ticket creation
- Custom metrics to external monitoring
- Backup automation

**Example**:
```javascript
const axios = require('axios');

module.exports = {
  async onSessionCreated(session) {
    // Post to Slack
    await axios.post(this.config.slackWebhookUrl, {
      text: `New session created: ${session.template} for ${session.user}`
    });
  },

  async createJiraTicket(summary, description) {
    const response = await axios.post(
      `${this.config.jiraUrl}/rest/api/2/issue`,
      {
        fields: {
          project: { key: this.config.projectKey },
          summary: summary,
          description: description,
          issuetype: { name: 'Task' }
        }
      },
      {
        auth: {
          username: this.config.jiraUsername,
          password: this.config.jiraApiToken
        }
      }
    );

    return response.data;
  }
};
```

### UI Theme

Customize the web interface appearance.

**Example**:
```javascript
module.exports = {
  theme: {
    colors: {
      primary: '#6366f1',
      secondary: '#8b5cf6',
      background: '#0f172a',
      surface: '#1e293b',
      text: '#f1f5f9'
    },
    typography: {
      fontFamily: '"Inter", sans-serif',
      fontSize: {
        small: '0.875rem',
        medium: '1rem',
        large: '1.25rem'
      }
    },
    spacing: {
      unit: 8
    }
  }
};
```

---

## Manifest File

The `manifest.json` file defines your plugin's metadata and capabilities.

### Required Fields

```json
{
  "name": "plugin-name",
  "version": "1.0.0",
  "displayName": "Plugin Display Name",
  "description": "What your plugin does",
  "type": "extension",
  "author": "Your Name",
  "entrypoints": {
    "main": "index.js"
  }
}
```

### Optional Fields

```json
{
  "license": "MIT",
  "homepage": "https://example.com/plugin",
  "repository": "https://github.com/user/plugin",
  "icon": "icon.png",
  "category": "Productivity",
  "tags": ["automation", "notifications"],
  "permissions": [
    "read:sessions",
    "write:sessions",
    "read:users",
    "notifications",
    "network"
  ],
  "dependencies": {
    "axios": "^1.6.0",
    "lodash": "^4.17.21"
  },
  "configSchema": {
    "type": "object",
    "properties": {
      "apiKey": {
        "type": "string",
        "title": "API Key",
        "description": "Your service API key"
      }
    },
    "required": ["apiKey"]
  }
}
```

### Entry Points

Specify different entry points for different plugin types:

```json
{
  "entrypoints": {
    "main": "index.js",          // Main plugin code
    "ui": "ui/index.jsx",         // UI components
    "api": "api/routes.js",       // API endpoints
    "webhook": "webhook.js",      // Webhook handlers
    "cli": "cli/commands.js"      // CLI commands
  }
}
```

---

## Plugin Structure

### Basic Structure

```
my-plugin/
├── manifest.json        # Plugin metadata
├── index.js            # Main entry point
├── package.json        # npm dependencies
├── README.md           # Plugin documentation
└── icon.png           # Plugin icon (optional)
```

### Advanced Structure

```
my-plugin/
├── manifest.json
├── package.json
├── README.md
├── icon.png
├── src/
│   ├── index.js       # Main plugin logic
│   ├── api.js         # API integrations
│   └── utils.js       # Utility functions
├── ui/
│   ├── index.jsx      # UI components
│   ├── Widget.jsx
│   └── Settings.jsx
├── webhooks/
│   ├── session.js     # Session event handlers
│   └── user.js        # User event handlers
└── tests/
    ├── index.test.js
    └── api.test.js
```

---

## Extension Plugins

Extensions can add new UI components, API endpoints, and features.

### Registering UI Components

```javascript
module.exports = {
  async onLoad() {
    // Register a dashboard widget
    streamspace.ui.registerWidget('session-stats', {
      title: 'Session Statistics',
      component: './ui/SessionStats.jsx',
      position: 'top',
      width: 'full'
    });

    // Register a settings page
    streamspace.ui.registerSettingsPage('my-settings', {
      title: 'Plugin Settings',
      component: './ui/Settings.jsx',
      icon: 'settings'
    });

    // Add menu item
    streamspace.ui.addMenuItem({
      label: 'Custom Feature',
      icon: 'star',
      path: '/custom-feature',
      component: './ui/CustomFeature.jsx'
    });
  }
};
```

### React Component Example

```jsx
// ui/SessionStats.jsx
import React, { useEffect, useState } from 'react';
import { Card, CardContent, Typography } from '@mui/material';

export default function SessionStats() {
  const [stats, setStats] = useState(null);

  useEffect(() => {
    // Use StreamSpace API
    streamspace.api.getSessions().then(sessions => {
      setStats({
        total: sessions.length,
        running: sessions.filter(s => s.state === 'running').length,
        hibernated: sessions.filter(s => s.state === 'hibernated').length
      });
    });
  }, []);

  if (!stats) return <div>Loading...</div>;

  return (
    <Card>
      <CardContent>
        <Typography variant="h6">Session Statistics</Typography>
        <Typography>Total: {stats.total}</Typography>
        <Typography>Running: {stats.running}</Typography>
        <Typography>Hibernated: {stats.hibernated}</Typography>
      </CardContent>
    </Card>
  );
}
```

### Adding API Endpoints

```javascript
module.exports = {
  async onLoad() {
    // Register custom API endpoint
    streamspace.api.registerEndpoint({
      method: 'GET',
      path: '/api/plugins/my-plugin/stats',
      handler: async (req, res) => {
        const sessions = await streamspace.api.getSessions();

        res.json({
          total: sessions.length,
          running: sessions.filter(s => s.state === 'running').length
        });
      },
      permissions: ['read:sessions']
    });

    // POST endpoint
    streamspace.api.registerEndpoint({
      method: 'POST',
      path: '/api/plugins/my-plugin/action',
      handler: async (req, res) => {
        const { action, sessionId } = req.body;

        // Perform action
        await streamspace.api.updateSession(sessionId, { state: action });

        res.json({ success: true });
      },
      permissions: ['write:sessions']
    });
  }
};
```

---

## Webhook Plugins

Webhook plugins react to system events.

### All Available Events

```javascript
module.exports = {
  // Session Events
  async onSessionCreated(session) {
    console.log('Session created:', session.id);
  },

  async onSessionStarted(session) {
    console.log('Session started:', session.id);
  },

  async onSessionStopped(session) {
    console.log('Session stopped:', session.id);
  },

  async onSessionHibernated(session) {
    console.log('Session hibernated:', session.id);
  },

  async onSessionWoken(session) {
    console.log('Session woken:', session.id);
  },

  async onSessionDeleted(session) {
    console.log('Session deleted:', session.id);
  },

  // User Events
  async onUserCreated(user) {
    console.log('User created:', user.username);
  },

  async onUserUpdated(user) {
    console.log('User updated:', user.username);
  },

  async onUserDeleted(user) {
    console.log('User deleted:', user.username);
  },

  async onUserLogin(user) {
    console.log('User logged in:', user.username);
  },

  async onUserLogout(user) {
    console.log('User logged out:', user.username);
  }
};
```

### Practical Example: Welcome Notifications

```javascript
module.exports = {
  async onUserCreated(user) {
    // Send welcome notification
    await streamspace.notify(user.id, {
      title: 'Welcome to StreamSpace!',
      message: `Hi ${user.fullName}, welcome aboard! Get started by browsing the application catalog.`,
      type: 'info',
      actions: [
        {
          label: 'Browse Catalog',
          url: '/catalog'
        }
      ]
    });

    // Send welcome email (if configured)
    if (this.config.sendWelcomeEmail) {
      await streamspace.email.send({
        to: user.email,
        subject: 'Welcome to StreamSpace',
        template: 'welcome',
        data: { user }
      });
    }
  },

  async onSessionCreated(session) {
    // Notify user
    await streamspace.notify(session.user, {
      title: 'Session Ready',
      message: `Your ${session.template} session is ready to use!`,
      type: 'success',
      actions: [
        {
          label: 'Open Session',
          url: `/sessions/${session.id}`
        }
      ]
    });
  }
};
```

### External Service Integration

```javascript
const axios = require('axios');

module.exports = {
  async onSessionCreated(session) {
    // Post to Slack
    if (this.config.slackEnabled) {
      await axios.post(this.config.slackWebhookUrl, {
        text: `🚀 New session created`,
        blocks: [
          {
            type: 'section',
            text: {
              type: 'mrkdwn',
              text: `*User:* ${session.user}\n*Template:* ${session.template}\n*Status:* ${session.state}`
            }
          }
        ]
      });
    }

    // Create Jira ticket
    if (this.config.jiraEnabled && session.template === 'support-workspace') {
      await this.createJiraTicket(
        `Support session for ${session.user}`,
        `Session ID: ${session.id}\nTemplate: ${session.template}`
      );
    }
  },

  async createJiraTicket(summary, description) {
    return axios.post(
      `${this.config.jiraUrl}/rest/api/2/issue`,
      {
        fields: {
          project: { key: this.config.jiraProjectKey },
          summary,
          description,
          issuetype: { name: 'Task' }
        }
      },
      {
        auth: {
          username: this.config.jiraUsername,
          password: this.config.jiraApiToken
        }
      }
    );
  }
};
```

---

## API Integration Plugins

Connect StreamSpace to external services.

### Slack Integration Example

```javascript
const axios = require('axios');

module.exports = {
  async onLoad() {
    // Register custom command
    streamspace.commands.register({
      name: 'notify-slack',
      description: 'Send notification to Slack',
      handler: async (args) => {
        await this.sendSlackMessage(args.message);
      }
    });
  },

  async sendSlackMessage(message) {
    await axios.post(this.config.webhookUrl, {
      text: message,
      username: 'StreamSpace',
      icon_emoji: ':computer:'
    });
  },

  async onSessionCreated(session) {
    await this.sendSlackMessage(
      `New ${session.template} session created for ${session.user}`
    );
  },

  async onSessionHibernated(session) {
    await this.sendSlackMessage(
      `Session ${session.id} hibernated after inactivity`
    );
  }
};
```

### GitHub Integration Example

```javascript
const { Octokit } = require('@octokit/rest');

module.exports = {
  octokit: null,

  async onLoad() {
    this.octokit = new Octokit({
      auth: this.config.githubToken
    });

    // Register GitHub sync command
    streamspace.commands.register({
      name: 'github-sync',
      description: 'Sync templates from GitHub',
      handler: async () => {
        await this.syncTemplates();
      }
    });
  },

  async syncTemplates() {
    const { data: repos } = await this.octokit.repos.listForOrg({
      org: this.config.orgName,
      type: 'public'
    });

    for (const repo of repos) {
      if (repo.topics.includes('streamspace-template')) {
        await streamspace.api.createTemplate({
          name: repo.name,
          displayName: repo.name.replace(/-/g, ' '),
          description: repo.description,
          repository: repo.html_url
        });
      }
    }
  },

  async onSessionCreated(session) {
    // Create GitHub issue for tracking
    await this.octokit.issues.create({
      owner: this.config.orgName,
      repo: this.config.trackingRepo,
      title: `Session created: ${session.template}`,
      body: `User: ${session.user}\nTemplate: ${session.template}\nSession ID: ${session.id}`,
      labels: ['session', 'auto-created']
    });
  }
};
```

---

## UI Theme Plugins

Customize the StreamSpace web interface.

### Complete Theme Example

```javascript
module.exports = {
  type: 'theme',

  theme: {
    // Color Palette
    colors: {
      // Primary colors
      primary: {
        main: '#6366f1',
        light: '#818cf8',
        dark: '#4f46e5',
        contrastText: '#ffffff'
      },

      // Secondary colors
      secondary: {
        main: '#8b5cf6',
        light: '#a78bfa',
        dark: '#7c3aed',
        contrastText: '#ffffff'
      },

      // Background
      background: {
        default: '#0f172a',
        paper: '#1e293b',
        elevated: '#334155'
      },

      // Text
      text: {
        primary: '#f1f5f9',
        secondary: '#cbd5e1',
        disabled: '#64748b'
      },

      // Status colors
      success: '#10b981',
      warning: '#f59e0b',
      error: '#ef4444',
      info: '#3b82f6'
    },

    // Typography
    typography: {
      fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
      fontSize: 14,
      fontWeightLight: 300,
      fontWeightRegular: 400,
      fontWeightMedium: 500,
      fontWeightBold: 700,

      h1: {
        fontSize: '2.5rem',
        fontWeight: 700,
        lineHeight: 1.2
      },
      h2: {
        fontSize: '2rem',
        fontWeight: 700,
        lineHeight: 1.3
      },
      h3: {
        fontSize: '1.75rem',
        fontWeight: 600,
        lineHeight: 1.4
      },
      h4: {
        fontSize: '1.5rem',
        fontWeight: 600,
        lineHeight: 1.4
      },
      h5: {
        fontSize: '1.25rem',
        fontWeight: 600,
        lineHeight: 1.5
      },
      h6: {
        fontSize: '1rem',
        fontWeight: 600,
        lineHeight: 1.6
      },

      body1: {
        fontSize: '1rem',
        lineHeight: 1.5
      },
      body2: {
        fontSize: '0.875rem',
        lineHeight: 1.5
      },

      button: {
        textTransform: 'none',
        fontWeight: 500
      }
    },

    // Spacing
    spacing: 8,

    // Border Radius
    shape: {
      borderRadius: 8
    },

    // Shadows
    shadows: {
      small: '0 1px 3px rgba(0,0,0,0.12)',
      medium: '0 4px 6px rgba(0,0,0,0.1)',
      large: '0 10px 15px rgba(0,0,0,0.1)',
      xlarge: '0 20px 25px rgba(0,0,0,0.15)'
    },

    // Component Overrides
    components: {
      button: {
        borderRadius: 8,
        padding: '10px 24px',
        fontWeight: 500
      },

      card: {
        borderRadius: 12,
        boxShadow: '0 4px 6px rgba(0,0,0,0.1)'
      },

      input: {
        borderRadius: 8,
        borderColor: '#475569'
      }
    }
  }
};
```

### Light Theme Example

```javascript
module.exports = {
  type: 'theme',

  theme: {
    colors: {
      primary: {
        main: '#2563eb',
        light: '#3b82f6',
        dark: '#1d4ed8',
        contrastText: '#ffffff'
      },

      background: {
        default: '#ffffff',
        paper: '#f8fafc',
        elevated: '#f1f5f9'
      },

      text: {
        primary: '#0f172a',
        secondary: '#475569',
        disabled: '#94a3b8'
      }
    },

    typography: {
      fontFamily: '"Open Sans", sans-serif'
    }
  }
};
```

---

## Configuration Schema

Define a schema for your plugin configuration to generate automatic UI forms.

### Basic Schema

```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "apiKey": {
        "type": "string",
        "title": "API Key",
        "description": "Your service API key"
      },
      "enabled": {
        "type": "boolean",
        "title": "Enable Integration",
        "default": true
      },
      "maxRetries": {
        "type": "number",
        "title": "Max Retries",
        "description": "Number of retry attempts",
        "minimum": 0,
        "maximum": 10,
        "default": 3
      }
    },
    "required": ["apiKey"]
  }
}
```

### Advanced Schema with Enums

```json
{
  "configSchema": {
    "type": "object",
    "properties": {
      "webhookUrl": {
        "type": "string",
        "title": "Webhook URL",
        "description": "Slack webhook URL",
        "pattern": "^https://hooks\\.slack\\.com/.*$"
      },
      "channel": {
        "type": "string",
        "title": "Channel",
        "description": "Slack channel name",
        "default": "#general"
      },
      "notifyOn": {
        "type": "enum",
        "title": "Notify On",
        "description": "Which events to notify about",
        "enum": ["all", "sessions", "users", "errors"],
        "default": "all"
      },
      "includeDetails": {
        "type": "boolean",
        "title": "Include Details",
        "description": "Include detailed information in notifications",
        "default": true
      },
      "rateLimit": {
        "type": "number",
        "title": "Rate Limit (messages/hour)",
        "description": "Maximum messages per hour",
        "minimum": 1,
        "maximum": 100,
        "default": 20
      }
    },
    "required": ["webhookUrl"]
  }
}
```

### Accessing Configuration in Code

```javascript
module.exports = {
  async onLoad() {
    // Access configuration values
    const apiKey = this.config.apiKey;
    const enabled = this.config.enabled;
    const maxRetries = this.config.maxRetries || 3;

    console.log('Plugin loaded with config:', this.config);
  },

  async onSessionCreated(session) {
    // Check if enabled
    if (!this.config.enabled) {
      return;
    }

    // Use config values
    const response = await this.callApi(this.config.apiKey, session);
  }
};
```

---

## API Reference

### StreamSpace API

Available in `streamspace` global object.

#### Sessions API

```javascript
// Get all sessions
const sessions = await streamspace.api.getSessions();

// Get single session
const session = await streamspace.api.getSession(sessionId);

// Create session
const newSession = await streamspace.api.createSession({
  user: 'username',
  template: 'firefox-browser',
  state: 'running',
  resources: {
    memory: '2Gi',
    cpu: '1000m'
  }
});

// Update session
await streamspace.api.updateSession(sessionId, {
  state: 'hibernated'
});

// Delete session
await streamspace.api.deleteSession(sessionId);

// Get user's sessions
const userSessions = await streamspace.api.getUserSessions(username);
```

#### Users API

```javascript
// Get all users
const users = await streamspace.api.getUsers();

// Get single user
const user = await streamspace.api.getUser(username);

// Create user
const newUser = await streamspace.api.createUser({
  username: 'john',
  fullName: 'John Doe',
  email: 'john@example.com',
  tier: 'pro'
});

// Update user
await streamspace.api.updateUser(username, {
  tier: 'enterprise'
});

// Delete user
await streamspace.api.deleteUser(username);
```

#### Templates API

```javascript
// Get all templates
const templates = await streamspace.api.getTemplates();

// Get single template
const template = await streamspace.api.getTemplate(templateName);

// Create template
const newTemplate = await streamspace.api.createTemplate({
  name: 'my-app',
  displayName: 'My App',
  category: 'Custom',
  baseImage: 'myregistry/myapp:latest'
});
```

#### Notifications API

```javascript
// Send notification to user
await streamspace.notify(userId, {
  title: 'Important Update',
  message: 'Your session is ready!',
  type: 'success', // 'success' | 'info' | 'warning' | 'error'
  actions: [
    {
      label: 'View Session',
      url: '/sessions/123'
    }
  ],
  dismissible: true,
  timeout: 5000 // Auto-dismiss after 5 seconds
});

// Send notification to all users
await streamspace.notifyAll({
  title: 'System Maintenance',
  message: 'Scheduled maintenance in 1 hour',
  type: 'warning'
});
```

#### Email API

```javascript
// Send email
await streamspace.email.send({
  to: 'user@example.com',
  subject: 'Welcome to StreamSpace',
  template: 'welcome', // Use template
  data: { username: 'john', loginUrl: 'https://...' }
});

// Send custom HTML email
await streamspace.email.send({
  to: 'user@example.com',
  subject: 'Custom Email',
  html: '<h1>Hello</h1><p>Welcome!</p>'
});
```

#### Storage API

```javascript
// Store plugin data
await streamspace.storage.set('my-key', { foo: 'bar' });

// Retrieve plugin data
const data = await streamspace.storage.get('my-key');

// Delete plugin data
await streamspace.storage.delete('my-key');

// List all keys
const keys = await streamspace.storage.keys();
```

#### UI API

```javascript
// Register widget
streamspace.ui.registerWidget('my-widget', {
  title: 'My Widget',
  component: './components/MyWidget.jsx',
  position: 'top', // 'top' | 'sidebar' | 'bottom'
  width: 'full' // 'full' | 'half' | 'third'
});

// Add menu item
streamspace.ui.addMenuItem({
  label: 'Custom Feature',
  icon: 'star',
  path: '/custom-feature',
  component: './components/CustomFeature.jsx'
});

// Register settings page
streamspace.ui.registerSettingsPage('my-settings', {
  title: 'Plugin Settings',
  component: './components/Settings.jsx',
  icon: 'settings'
});
```

---

## Testing and Debugging

### Local Testing

1. **Create test environment**:

```bash
# Package your plugin
tar -czf my-plugin.tar.gz manifest.json index.js package.json

# Upload to StreamSpace test instance
curl -X POST http://localhost:3000/api/plugins/install \
  -F "file=@my-plugin.tar.gz" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

2. **View logs**:

```bash
# View plugin logs
kubectl logs -n streamspace -l app=streamspace-api --tail=100 | grep "my-plugin"
```

3. **Debug mode**:

```javascript
module.exports = {
  async onLoad() {
    if (process.env.DEBUG) {
      console.log('Plugin config:', this.config);
      console.log('StreamSpace API:', streamspace.api);
    }
  }
};
```

### Unit Testing

Use Jest or Mocha for unit tests:

```javascript
// tests/index.test.js
const plugin = require('../index');

describe('My Plugin', () => {
  test('should send notification on session created', async () => {
    const session = {
      id: '123',
      user: 'testuser',
      template: 'firefox'
    };

    const mockNotify = jest.fn();
    global.streamspace = {
      notify: mockNotify
    };

    await plugin.onSessionCreated(session);

    expect(mockNotify).toHaveBeenCalledWith(
      'testuser',
      expect.objectContaining({
        title: expect.any(String)
      })
    );
  });
});
```

### Integration Testing

Test with actual StreamSpace API:

```javascript
// tests/integration.test.js
const axios = require('axios');

describe('Plugin Integration', () => {
  const apiUrl = 'http://localhost:3000';
  let authToken;

  beforeAll(async () => {
    // Login and get token
    const response = await axios.post(`${apiUrl}/api/auth/login`, {
      username: 'testuser',
      password: 'testpass'
    });
    authToken = response.data.token;
  });

  test('should create session via plugin', async () => {
    const response = await axios.post(
      `${apiUrl}/api/plugins/my-plugin/create-session`,
      { template: 'firefox' },
      { headers: { Authorization: `Bearer ${authToken}` } }
    );

    expect(response.status).toBe(200);
    expect(response.data.session).toBeDefined();
  });
});
```

---

## Best Practices

### 1. Error Handling

Always handle errors gracefully:

```javascript
module.exports = {
  async onSessionCreated(session) {
    try {
      await this.sendNotification(session);
    } catch (error) {
      console.error('Failed to send notification:', error);

      // Log error to StreamSpace
      await streamspace.log.error('my-plugin', 'notification-failed', {
        error: error.message,
        sessionId: session.id
      });
    }
  }
};
```

### 2. Configuration Validation

Validate configuration on load:

```javascript
module.exports = {
  async onLoad() {
    // Validate required config
    if (!this.config.apiKey) {
      throw new Error('API key is required. Please configure the plugin.');
    }

    if (!this.config.webhookUrl.startsWith('https://')) {
      throw new Error('Webhook URL must use HTTPS');
    }

    // Test connection
    try {
      await this.testConnection();
    } catch (error) {
      console.warn('Failed to connect to external service:', error);
    }
  }
};
```

### 3. Rate Limiting

Implement rate limiting for external API calls:

```javascript
class RateLimiter {
  constructor(maxRequests, timeWindow) {
    this.maxRequests = maxRequests;
    this.timeWindow = timeWindow;
    this.requests = [];
  }

  async throttle() {
    const now = Date.now();
    this.requests = this.requests.filter(
      time => now - time < this.timeWindow
    );

    if (this.requests.length >= this.maxRequests) {
      const oldestRequest = this.requests[0];
      const waitTime = this.timeWindow - (now - oldestRequest);
      await new Promise(resolve => setTimeout(resolve, waitTime));
      return this.throttle();
    }

    this.requests.push(now);
  }
}

module.exports = {
  rateLimiter: null,

  async onLoad() {
    // 10 requests per minute
    this.rateLimiter = new RateLimiter(10, 60000);
  },

  async callExternalApi(data) {
    await this.rateLimiter.throttle();
    return axios.post(this.config.apiUrl, data);
  }
};
```

### 4. Async Operations

Handle async operations properly:

```javascript
module.exports = {
  async onSessionCreated(session) {
    // Run operations in parallel when possible
    await Promise.all([
      this.sendSlackNotification(session),
      this.createJiraTicket(session),
      this.logToAnalytics(session)
    ]);

    // Run sequential operations when needed
    const ticket = await this.createJiraTicket(session);
    await this.linkTicketToSession(session.id, ticket.id);
  }
};
```

### 5. Resource Cleanup

Clean up resources on unload:

```javascript
module.exports = {
  connections: [],
  timers: [],

  async onLoad() {
    // Create connections
    this.connection = await createConnection(this.config);
    this.connections.push(this.connection);

    // Create timers
    this.timer = setInterval(() => {
      this.syncData();
    }, 60000);
    this.timers.push(this.timer);
  },

  async onUnload() {
    // Close all connections
    await Promise.all(
      this.connections.map(conn => conn.close())
    );

    // Clear all timers
    this.timers.forEach(timer => clearInterval(timer));
  }
};
```

### 6. Logging

Use structured logging:

```javascript
module.exports = {
  log(level, message, data = {}) {
    const logEntry = {
      plugin: 'my-plugin',
      level,
      message,
      data,
      timestamp: new Date().toISOString()
    };

    console.log(JSON.stringify(logEntry));
  },

  async onSessionCreated(session) {
    this.log('info', 'Session created', {
      sessionId: session.id,
      user: session.user,
      template: session.template
    });
  }
};
```

---

## Security Guidelines

### 1. Validate Input

Always validate and sanitize input:

```javascript
function validateSessionId(sessionId) {
  if (typeof sessionId !== 'string') {
    throw new Error('Session ID must be a string');
  }

  if (!/^[a-zA-Z0-9-]+$/.test(sessionId)) {
    throw new Error('Invalid session ID format');
  }

  return sessionId;
}

module.exports = {
  async onLoad() {
    streamspace.api.registerEndpoint({
      method: 'POST',
      path: '/api/plugins/my-plugin/action',
      handler: async (req, res) => {
        const sessionId = validateSessionId(req.body.sessionId);
        // ... proceed safely
      }
    });
  }
};
```

### 2. Secure Credentials

Never hardcode credentials:

```javascript
// ❌ BAD - Hardcoded credentials
const apiKey = 'sk-1234567890';

// ✅ GOOD - Use configuration
const apiKey = this.config.apiKey;

// ✅ BETTER - Use environment variables for sensitive data
const apiKey = process.env.API_KEY || this.config.apiKey;
```

### 3. Use HTTPS

Always use HTTPS for external requests:

```javascript
async onLoad() {
  if (!this.config.webhookUrl.startsWith('https://')) {
    throw new Error('Webhook URL must use HTTPS for security');
  }
}
```

### 4. Limit Permissions

Request only necessary permissions:

```json
{
  "permissions": [
    "read:sessions",    // ✅ Only what you need
    "notifications"
  ]
  // ❌ Don't request "admin" unless absolutely necessary
}
```

### 5. Sanitize Output

Prevent injection attacks:

```javascript
function escapeHtml(text) {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

async sendNotification(session) {
  await streamspace.notify(session.user, {
    title: escapeHtml(session.template),
    message: escapeHtml(`Session ${session.id} is ready`)
  });
}
```

---

## Publishing Plugins

### 1. Prepare Plugin

```bash
# Ensure all files are included
ls -la

# Test plugin locally
npm test

# Build if needed
npm run build
```

### 2. Create Repository

Create a Git repository for your plugin:

```bash
git init
git add .
git commit -m "Initial commit"
git remote add origin https://github.com/username/streamspace-plugin-name
git push -u origin main
```

### 3. Create manifest.json

Ensure manifest.json is complete:

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "displayName": "My Plugin",
  "description": "Detailed description of what your plugin does",
  "type": "extension",
  "author": "Your Name <email@example.com>",
  "license": "MIT",
  "homepage": "https://github.com/username/streamspace-plugin-name",
  "repository": "https://github.com/username/streamspace-plugin-name",
  "icon": "icon.png",
  "category": "Productivity",
  "tags": ["automation", "notifications"],
  "permissions": ["read:sessions", "notifications"]
}
```

### 4. Add to Repository

Users can add your plugin repository to StreamSpace:

```bash
# Via kubectl
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: Repository
metadata:
  name: my-plugin-repo
  namespace: streamspace
spec:
  url: https://github.com/username/streamspace-plugin-name
  branch: main
  authType: public
EOF
```

### 5. Version Updates

Follow semantic versioning:

```bash
# Bug fix: 1.0.0 -> 1.0.1
# New feature: 1.0.1 -> 1.1.0
# Breaking change: 1.1.0 -> 2.0.0

# Update version in manifest.json
git commit -am "Release v1.1.0"
git tag v1.1.0
git push origin main --tags
```

---

## Examples

### Complete Example: Session Analytics Plugin

```javascript
// manifest.json
{
  "name": "session-analytics",
  "version": "1.0.0",
  "displayName": "Session Analytics",
  "description": "Track and analyze session usage patterns",
  "type": "extension",
  "author": "StreamSpace Team",
  "permissions": ["read:sessions", "read:users", "notifications"],
  "entrypoints": {
    "main": "index.js",
    "ui": "ui/Dashboard.jsx"
  },
  "configSchema": {
    "type": "object",
    "properties": {
      "trackingEnabled": {
        "type": "boolean",
        "title": "Enable Tracking",
        "default": true
      },
      "reportInterval": {
        "type": "number",
        "title": "Report Interval (hours)",
        "minimum": 1,
        "maximum": 168,
        "default": 24
      }
    }
  }
}
```

```javascript
// index.js
const analytics = {
  sessions: new Map(),

  async onLoad() {
    console.log('Session Analytics plugin loaded');

    // Register UI dashboard
    streamspace.ui.registerWidget('analytics-dashboard', {
      title: 'Session Analytics',
      component: './ui/Dashboard.jsx',
      position: 'top',
      width: 'full'
    });

    // Register API endpoints
    streamspace.api.registerEndpoint({
      method: 'GET',
      path: '/api/plugins/analytics/stats',
      handler: this.getStats.bind(this),
      permissions: ['read:sessions']
    });

    // Start periodic reporting
    if (this.config.trackingEnabled) {
      this.startReporting();
    }
  },

  async onSessionCreated(session) {
    this.trackEvent('session_created', session);
  },

  async onSessionStarted(session) {
    this.sessions.set(session.id, {
      startTime: Date.now(),
      user: session.user,
      template: session.template
    });
    this.trackEvent('session_started', session);
  },

  async onSessionStopped(session) {
    const data = this.sessions.get(session.id);
    if (data) {
      const duration = Date.now() - data.startTime;
      this.trackEvent('session_stopped', {
        ...session,
        duration
      });
      this.sessions.delete(session.id);
    }
  },

  trackEvent(event, data) {
    streamspace.storage.get('events').then(events => {
      events = events || [];
      events.push({
        event,
        data,
        timestamp: Date.now()
      });
      return streamspace.storage.set('events', events);
    });
  },

  async getStats(req, res) {
    const events = await streamspace.storage.get('events') || [];
    const sessions = await streamspace.api.getSessions();

    const stats = {
      totalSessions: sessions.length,
      activeSessions: sessions.filter(s => s.state === 'running').length,
      hibernatedSessions: sessions.filter(s => s.state === 'hibernated').length,
      events: events.length,
      topTemplates: this.getTopTemplates(events),
      topUsers: this.getTopUsers(events)
    };

    res.json(stats);
  },

  getTopTemplates(events) {
    const counts = {};
    events
      .filter(e => e.event === 'session_created')
      .forEach(e => {
        counts[e.data.template] = (counts[e.data.template] || 0) + 1;
      });

    return Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([template, count]) => ({ template, count }));
  },

  getTopUsers(events) {
    const counts = {};
    events
      .filter(e => e.event === 'session_created')
      .forEach(e => {
        counts[e.data.user] = (counts[e.data.user] || 0) + 1;
      });

    return Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5)
      .map(([user, count]) => ({ user, count }));
  },

  startReporting() {
    const interval = this.config.reportInterval * 60 * 60 * 1000;

    this.reportTimer = setInterval(async () => {
      const stats = await this.generateReport();

      // Notify admins
      await streamspace.notifyAdmins({
        title: 'Session Analytics Report',
        message: `${stats.totalSessions} total sessions, ${stats.activeSessions} active`,
        type: 'info'
      });
    }, interval);
  },

  async generateReport() {
    const response = await streamspace.api.request(
      '/api/plugins/analytics/stats'
    );
    return response.data;
  },

  async onUnload() {
    if (this.reportTimer) {
      clearInterval(this.reportTimer);
    }
  }
};

module.exports = analytics;
```

```jsx
// ui/Dashboard.jsx
import React, { useEffect, useState } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Grid,
  List,
  ListItem,
  ListItemText
} from '@mui/material';

export default function AnalyticsDashboard() {
  const [stats, setStats] = useState(null);

  useEffect(() => {
    loadStats();
    const interval = setInterval(loadStats, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, []);

  const loadStats = async () => {
    const response = await streamspace.api.request(
      '/api/plugins/analytics/stats'
    );
    setStats(response.data);
  };

  if (!stats) return <div>Loading...</div>;

  return (
    <Box>
      <Typography variant="h5" gutterBottom>
        Session Analytics
      </Typography>

      <Grid container spacing={3}>
        <Grid item xs={12} md={4}>
          <Card>
            <CardContent>
              <Typography color="textSecondary" gutterBottom>
                Total Sessions
              </Typography>
              <Typography variant="h3">
                {stats.totalSessions}
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} md={4}>
          <Card>
            <CardContent>
              <Typography color="textSecondary" gutterBottom>
                Active Sessions
              </Typography>
              <Typography variant="h3" color="success.main">
                {stats.activeSessions}
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} md={4}>
          <Card>
            <CardContent>
              <Typography color="textSecondary" gutterBottom>
                Hibernated Sessions
              </Typography>
              <Typography variant="h3" color="warning.main">
                {stats.hibernatedSessions}
              </Typography>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Top Templates
              </Typography>
              <List>
                {stats.topTemplates.map(({ template, count }) => (
                  <ListItem key={template}>
                    <ListItemText
                      primary={template}
                      secondary={`${count} sessions`}
                    />
                  </ListItem>
                ))}
              </List>
            </CardContent>
          </Card>
        </Grid>

        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Top Users
              </Typography>
              <List>
                {stats.topUsers.map(({ user, count }) => (
                  <ListItem key={user}>
                    <ListItemText
                      primary={user}
                      secondary={`${count} sessions`}
                    />
                  </ListItem>
                ))}
              </List>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
}
```

---

## Additional Resources

- [Plugin API Reference](docs/PLUGIN_API.md)
- [Plugin Manifest Schema](docs/PLUGIN_MANIFEST.md)
- [Example Plugins Repository](https://github.com/streamspace/example-plugins)
- [Plugin Development Discord](https://discord.gg/streamspace)

---

## Support

- **Documentation**: https://docs.streamspace.io/plugins
- **GitHub Issues**: https://github.com/streamspace/streamspace/issues
- **Discord**: https://discord.gg/streamspace
- **Email**: plugins@streamspace.io

---

**Happy Plugin Development!** 🚀

# StreamSpace Slack Integration Plugin

Send real-time notifications about StreamSpace events to your Slack channels.

## Features

- 🚀 Session event notifications (created, hibernated, deleted)
- 👤 User event notifications (created, login, logout)
- ⚙️ Configurable notification preferences
- 🚦 Rate limiting to prevent spam
- 📊 Detailed or summary notifications
- 🎨 Rich Slack attachments with colors and formatting

## Installation

### Via StreamSpace UI

1. Navigate to **Admin** → **Plugins**
2. Search for "Slack Integration"
3. Click **Install**
4. Configure your Slack webhook URL
5. Enable the plugin

### Via kubectl

```bash
kubectl apply -f - <<EOF
apiVersion: stream.space/v1alpha1
kind: InstalledPlugin
metadata:
  name: streamspace-slack
  namespace: streamspace
spec:
  catalogPluginName: streamspace-slack
  enabled: true
  config:
    webhookUrl: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
    channel: "#general"
    username: "StreamSpace"
    iconEmoji: ":computer:"
    notifyOnSessionCreated: true
    notifyOnSessionHibernated: false
    notifyOnUserCreated: true
    includeDetails: true
    rateLimit: 20
EOF
```

## Configuration

### Slack Webhook Setup

1. Go to your Slack workspace settings
2. Navigate to **Apps** → **Incoming Webhooks**
3. Click **Add to Slack**
4. Select the channel for notifications
5. Copy the webhook URL

### Plugin Configuration

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `webhookUrl` | string | *required* | Your Slack incoming webhook URL |
| `channel` | string | `#general` | Default Slack channel for notifications |
| `username` | string | `StreamSpace` | Bot username for messages |
| `iconEmoji` | string | `:computer:` | Emoji icon for messages |
| `notifyOnSessionCreated` | boolean | `true` | Send notification when session created |
| `notifyOnSessionHibernated` | boolean | `false` | Send notification when session hibernated |
| `notifyOnUserCreated` | boolean | `true` | Send notification when user created |
| `notifyOnQuotaExceeded` | boolean | `true` | Send notification on quota exceeded |
| `includeDetails` | boolean | `true` | Include detailed information |
| `rateLimit` | number | `20` | Maximum messages per hour |

## Example Notifications

### Session Created
```
🚀 New Session Created

User: john@example.com
Template: firefox-browser
Session ID: john-firefox-abc123
Memory: 2Gi
CPU: 1000m
```

### Session Hibernated
```
💤 Session Hibernated

User: john@example.com
Session ID: john-firefox-abc123

Session hibernated due to inactivity.
```

### User Created
```
👤 New User Created

Username: jane
Full Name: Jane Smith
Email: jane@example.com
Tier: pro
```

## Events

This plugin listens to the following StreamSpace events:

- `session.created` - New session created
- `session.hibernated` - Session hibernated
- `session.deleted` - Session deleted
- `user.created` - New user created
- `user.login` - User logged in
- `user.logout` - User logged out
- `quota.exceeded` - User exceeded quota

## Rate Limiting

To prevent Slack from being overwhelmed, the plugin includes configurable rate limiting:

- Default: 20 messages per hour
- Configurable via `rateLimit` setting
- Excess notifications are logged but not sent

## Permissions

This plugin requires:

- `network` - To send HTTP requests to Slack

## Troubleshooting

### Webhook Not Working

1. Verify webhook URL is correct
2. Check that webhook hasn't been revoked in Slack
3. Ensure Slack workspace allows incoming webhooks
4. Check StreamSpace logs for error messages

### No Notifications

1. Verify plugin is enabled
2. Check notification settings (e.g., `notifyOnSessionCreated`)
3. Check rate limit hasn't been exceeded
4. Verify events are being triggered in StreamSpace

### Too Many Notifications

1. Reduce `rateLimit` setting
2. Disable specific event notifications
3. Set `includeDetails` to `false` for shorter messages

## Development

### Building from Source

```bash
cd plugins/streamspace-slack
go build -o slack-plugin.so -buildmode=plugin slack_plugin.go
```

### Testing

```bash
go test ./...
```

## License

MIT License - see LICENSE file

## Support

- [Documentation](https://docs.streamspace.io/plugins/slack)
- [Issues](https://github.com/JoshuaAFerguson/streamspace-plugins/issues)
- [Discord](https://discord.gg/streamspace)

## Version History

### 1.0.0 (2025-11-16)
- Initial release
- Session event notifications
- User event notifications
- Rate limiting
- Rich Slack attachments

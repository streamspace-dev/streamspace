# StreamSpace Sentry Plugin

Error tracking and performance monitoring integration with Sentry.

## Features

- **Error Tracking** - Automatically capture and report errors and exceptions
- **Performance Monitoring** - Track transaction performance and bottlenecks
- **Breadcrumbs** - Detailed event trail leading to errors
- **Source Maps** - Link errors to exact code locations
- **Releases** - Track errors across deployments
- **User Context** - Associate errors with specific users and sessions
- **Custom Tags** - Organize and filter errors
- **Ignore Patterns** - Filter out expected errors and noise

## Installation

### Via Plugin Marketplace

1. Navigate to **Admin → Plugins**
2. Search for "Sentry Error Tracking"
3. Click **Install**
4. Configure with your Sentry DSN
5. Click **Enable**

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "dsn": "https://[key]@[organization].ingest.sentry.io/[project]",
  "environment": "production"
}
```

### Full Configuration

```json
{
  "enabled": true,
  "dsn": "https://examplePublicKey@o0.ingest.sentry.io/0",
  "environment": "production",
  "release": "streamspace@1.0.0",
  "serverName": "api-server-01",
  "enableTracing": true,
  "tracesSampleRate": 0.1,
  "attachStacktrace": true,
  "sendDefaultPii": false,
  "captureSessionErrors": true,
  "captureAPIErrors": true,
  "captureUnhandledErrors": true,
  "ignoreErrors": [
    "context canceled",
    "connection reset by peer",
    "broken pipe"
  ],
  "tags": {
    "service": "streamspace",
    "region": "us-east-1",
    "team": "platform"
  }
}
```

### Getting Your Sentry DSN

1. Log into Sentry.io
2. Go to **Settings → Projects → [Your Project]**
3. Click **Client Keys (DSN)**
4. Copy the DSN URL

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable Sentry integration |
| `dsn` | string | *required* | Sentry Data Source Name |
| `environment` | string | `production` | Environment name |
| `release` | string | `1.0.0` | Release version for tracking |
| `serverName` | string | `streamspace-api` | Server identifier |
| `enableTracing` | boolean | `true` | Enable performance tracing |
| `tracesSampleRate` | number | `0.1` | % of transactions to trace (0.0-1.0) |
| `attachStacktrace` | boolean | `true` | Include stack traces |
| `sendDefaultPii` | boolean | `false` | Send user IDs and IPs |
| `captureSessionErrors` | boolean | `true` | Capture session errors |
| `captureAPIErrors` | boolean | `true` | Capture API errors |
| `captureUnhandledErrors` | boolean | `true` | Capture unhandled exceptions |
| `ignoreErrors` | array | `[]` | Error patterns to ignore (regex) |
| `tags` | object | `{}` | Global tags for all events |

## Usage

### View Errors in Sentry

1. Log into Sentry.io
2. Navigate to **Issues**
3. Filter by:
   - Environment (production, staging)
   - Release version
   - User ID
   - Session ID
   - Tags

### Error Details

Each error in Sentry includes:
- **Stack Trace** - Full stack trace with code context
- **Breadcrumbs** - Events leading up to error
- **User Context** - User ID, session ID
- **Tags** - Categorization and filtering
- **Environment** - Where error occurred

### Creating Alerts

#### High Error Rate Alert

```
Alert Conditions:
- Number of events > 100
- In 1 minute
- For errors matching: is:unresolved

Actions:
- Send Slack notification to #alerts
- Send email to platform-team@company.com
```

#### New Error Type Alert

```
Alert Conditions:
- A new issue is created
- For errors matching: is:unresolved level:error

Actions:
- Create PagerDuty incident
- Post to #platform-alerts Slack channel
```

#### Session Error Spike

```
Alert Conditions:
- Number of events > 50
- In 5 minutes
- For errors matching: session_id:*

Actions:
- Send webhook to monitoring system
- Email ops-team@company.com
```

### Releases and Deploys

Track which errors came from which deployment:

```bash
# Create a release
sentry-cli releases new streamspace@1.2.0

# Associate commits
sentry-cli releases set-commits streamspace@1.2.0 --auto

# Deploy
sentry-cli releases deploys streamspace@1.2.0 new -e production

# Finalize
sentry-cli releases finalize streamspace@1.2.0
```

### Performance Monitoring

View transaction performance:

1. Navigate to **Performance** in Sentry
2. View slow transactions
3. Analyze bottlenecks
4. Track improvements over releases

## Events Captured

### Automatic Events

- **Session Errors** - Errors during session creation/termination
- **API Errors** - Failed API requests and validations
- **Unhandled Exceptions** - Panics and uncaught errors
- **Database Errors** - Query failures and connection issues

### Manual Events

You can manually capture errors in your code:

```go
// Capture an error
plugin.CaptureError(err, map[string]interface{}{
    "user_id": userID,
    "session_id": sessionID,
    "action": "create_session",
})

// Capture a message
plugin.CaptureMessage("Important event occurred", sentry.LevelWarning, map[string]interface{}{
    "detail": "xyz",
})

// Start a transaction (performance)
span := plugin.StartTransaction("session.create", "http.request")
defer span.Finish()
```

## Breadcrumbs

Breadcrumbs provide context about what happened before an error:

**Automatic Breadcrumbs**:
- Session created
- Session terminated
- User created
- API requests
- Database queries

**Example Breadcrumb Trail**:
```
1. User logged in (user_id: 123)
2. Session created (session_id: abc, template: firefox)
3. API request: GET /api/sessions/abc
4. Database query: SELECT * FROM sessions WHERE id = 'abc'
5. ERROR: Session not found
```

## Ignore Patterns

Prevent noise from expected errors:

```json
{
  "ignoreErrors": [
    "context canceled",          // User canceled operation
    "connection reset",          // Network issues
    "broken pipe",               // Client disconnected
    "session not found",         // Expected 404
    "unauthorized",              // Auth failures (use rate limit instead)
    "EOF"                        // Connection closed
  ]
}
```

## Troubleshooting

### Errors not appearing in Sentry

**Problem**: Events not showing up

**Solution**:
- Verify DSN is correct
- Check `enabled` is `true`
- Review Sentry project quota (may be exhausted)
- Check error doesn't match ignore patterns
- Wait 30-60 seconds for events to appear

### Too many errors

**Problem**: Error quota exhausted, high Sentry costs

**Solution**:
- Add ignore patterns for noisy errors
- Reduce `tracesSampleRate` (e.g., 0.01 = 1%)
- Set up error grouping rules
- Use Sentry's spike protection
- Upgrade Sentry plan or add more quota

### Missing stack traces

**Problem**: Errors don't show code context

**Solution**:
- Ensure `attachStacktrace: true`
- Upload source maps for minified code
- Check stack trace depth limits
- Verify release is set correctly

### High memory usage

**Problem**: Sentry SDK using too much memory

**Solution**:
- Reduce `tracesSampleRate`
- Disable `attachStacktrace` if not needed
- Limit breadcrumb buffer size
- Review event size limits

## Best Practices

1. **Set Releases** - Always set release version for tracking
2. **Use Environments** - Separate production, staging, development
3. **Add Context** - Include user_id, session_id in error context
4. **Create Alerts** - Proactive alerting on new/high error rates
5. **Review Weekly** - Triage new issues, resolve old ones
6. **Ignore Wisely** - Filter noise but don't over-filter
7. **Track Performance** - Use tracing to find bottlenecks
8. **Monitor Quota** - Track Sentry usage to control costs

## Integration with Other Tools

### Slack

```
Sentry → Settings → Integrations → Slack
- Link Slack workspace
- Choose #alerts channel
- Configure notification rules
```

### Jira

```
Sentry → Settings → Integrations → Jira
- Link Jira instance
- Auto-create tickets for new issues
- Link Sentry issues to Jira tickets
```

### GitHub

```
Sentry → Settings → Integrations → GitHub
- Link GitHub repository
- Create GitHub issues from Sentry
- See suspect commits in error details
```

## Support

- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Docs: https://docs.streamspace.io/plugins/sentry
- Sentry Docs: https://docs.sentry.io/

## License

MIT License

## Version History

- **1.0.0** (2025-01-15)
  - Initial release
  - Error tracking
  - Performance monitoring
  - Breadcrumbs
  - Custom tags and ignore patterns

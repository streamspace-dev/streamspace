# StreamSpace Datadog Plugin

Comprehensive monitoring integration with Datadog for metrics, traces, and logs.

## Features

### Metrics Collection
- **Session Metrics** - Track session lifecycle (created, terminated, active count, duration)
- **Resource Metrics** - Monitor CPU, memory, and storage usage per session
- **User Metrics** - Track user activity (created, login, logout counts)
- **Custom Metrics** - Send any StreamSpace metric to Datadog

### Events
- Session lifecycle events (created, terminated)
- Plugin lifecycle events (loaded, unloaded)
- User activity events

### Automatic Tracking
- Active session count
- Session duration tracking
- Resource utilization over time
- User engagement metrics

## Installation

### Via Plugin Marketplace (Recommended)

1. Navigate to **Admin → Plugins**
2. Search for "Datadog Monitoring"
3. Click **Install**
4. Configure settings (see Configuration section)
5. Click **Enable**

### Manual Installation

```bash
# Copy plugin files to plugins directory
cp -r streamspace-datadog /path/to/streamspace/plugins/

# Restart StreamSpace API
systemctl restart streamspace-api
```

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "apiKey": "your-datadog-api-key",
  "site": "datadoghq.com",
  "enableMetrics": true,
  "globalTags": ["env:production", "service:streamspace"]
}
```

### Full Configuration

```json
{
  "enabled": true,
  "apiKey": "your-datadog-api-key",
  "appKey": "your-datadog-app-key",
  "site": "datadoghq.com",
  "enableMetrics": true,
  "enableTraces": true,
  "enableLogs": false,
  "globalTags": [
    "env:production",
    "service:streamspace",
    "region:us-east-1",
    "team:platform"
  ],
  "metricsInterval": 60,
  "trackSessionMetrics": true,
  "trackResourceMetrics": true,
  "trackUserMetrics": true
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable Datadog integration |
| `apiKey` | string | *required* | Your Datadog API key |
| `appKey` | string | optional | Datadog application key (for advanced features) |
| `site` | string | `datadoghq.com` | Datadog site (US, EU, etc.) |
| `enableMetrics` | boolean | `true` | Send metrics to Datadog |
| `enableTraces` | boolean | `true` | Send APM traces to Datadog |
| `enableLogs` | boolean | `false` | Send logs to Datadog |
| `globalTags` | array | `["env:production"]` | Tags applied to all metrics |
| `metricsInterval` | integer | `60` | How often to flush metrics (seconds) |
| `trackSessionMetrics` | boolean | `true` | Track session lifecycle metrics |
| `trackResourceMetrics` | boolean | `true` | Track CPU/memory/storage metrics |
| `trackUserMetrics` | boolean | `true` | Track user activity metrics |

### Datadog Sites

Choose the correct site based on your Datadog account region:

- **US1** (default): `datadoghq.com`
- **US3**: `us3.datadoghq.com`
- **US5**: `us5.datadoghq.com`
- **EU1**: `datadoghq.eu`
- **AP1**: `ap1.datadoghq.com`

## Metrics Reference

### Session Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `streamspace.session.created` | count | Number of sessions created |
| `streamspace.session.terminated` | count | Number of sessions terminated |
| `streamspace.session.active` | gauge | Current number of active sessions |
| `streamspace.session.duration` | gauge | Session duration in seconds |

**Tags**: `user:<user_id>`, `template:<template_name>`

### Resource Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `streamspace.session.cpu_usage` | gauge | CPU usage percentage (0-100) |
| `streamspace.session.memory_usage` | gauge | Memory usage in bytes |
| `streamspace.session.storage_usage` | gauge | Storage usage in bytes |

**Tags**: `session:<session_id>`, `user:<user_id>`, `template:<template_name>`

### User Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `streamspace.user.created` | count | Number of users created |
| `streamspace.user.login` | count | Number of user logins |
| `streamspace.user.logout` | count | Number of user logouts |
| `streamspace.users.total` | count | Total user count |

**Tags**: `user:<user_id>`

## Usage

### View Metrics in Datadog

1. Log into your Datadog account
2. Navigate to **Metrics → Explorer**
3. Search for metrics starting with `streamspace.*`
4. Create custom dashboards and graphs

### Create Dashboards

#### Session Overview Dashboard

```
Widget 1: Active Sessions (Timeseries)
- Metric: streamspace.session.active
- Visualization: Line graph

Widget 2: Session Duration (Heatmap)
- Metric: streamspace.session.duration
- Visualization: Heatmap

Widget 3: Sessions Created (Top List)
- Metric: streamspace.session.created
- Group by: template
- Visualization: Top list

Widget 4: Resource Usage (Stacked Area)
- Metrics: streamspace.session.cpu_usage, streamspace.session.memory_usage
- Visualization: Stacked area
```

#### User Activity Dashboard

```
Widget 1: User Logins (Timeseries)
- Metric: streamspace.user.login
- Visualization: Bars

Widget 2: Active Users (Query Value)
- Metric: streamspace.users.total
- Visualization: Query value

Widget 3: User Sessions (Table)
- Metrics: streamspace.session.active
- Group by: user
- Visualization: Table
```

### Create Monitors

#### High Session Count Alert

```
Metric: streamspace.session.active
Condition: Alert when avg(last_5m) > 100
Message: StreamSpace has {{value}} active sessions (threshold: 100)
Tags: @slack-platform-alerts
```

#### Long Session Duration Alert

```
Metric: streamspace.session.duration
Condition: Alert when max(last_15m) > 28800  # 8 hours
Message: Session {{session.name}} has been running for {{value}} seconds
Tags: @pagerduty-platform
```

#### High Resource Usage Alert

```
Metric: streamspace.session.cpu_usage
Condition: Alert when avg(last_10m) > 90
Message: Session {{session.name}} CPU usage is {{value}}%
Tags: @ops-team
```

## Events Reference

### Session Events

- **Session Created**: Triggered when a new session is created
- **Session Terminated**: Triggered when a session is terminated

### Plugin Events

- **Plugin Loaded**: Triggered when the Datadog plugin is loaded
- **Plugin Unloaded**: Triggered when the Datadog plugin is unloaded

## Troubleshooting

### Metrics not appearing in Datadog

**Problem**: Metrics not showing up in Datadog UI

**Solution**:
- Verify API key is correct
- Check Datadog site setting matches your account region
- Review plugin logs: `tail -f /var/log/streamspace/plugins/datadog.log`
- Verify `enableMetrics` is `true`
- Check metrics interval hasn't been set too high

### Authentication errors

**Problem**: 403 Forbidden errors in logs

**Solution**:
- Verify your Datadog API key is valid
- Check API key permissions in Datadog
- Ensure API key hasn't expired
- Try regenerating API key in Datadog settings

### Metrics delayed

**Problem**: Metrics appear in Datadog with significant delay

**Solution**:
- Lower `metricsInterval` (minimum: 10 seconds)
- Check network connectivity to Datadog
- Verify no rate limiting is occurring
- Check for high metric cardinality

### High cardinality warnings

**Problem**: Datadog warns about high metric cardinality

**Solution**:
- Reduce number of tags in `globalTags`
- Disable detailed resource tracking if not needed
- Use tag aggregation in Datadog
- Consider using distributions instead of gauges

## Best Practices

1. **Tag Wisely** - Use meaningful tags but avoid high cardinality (user IDs, session IDs in global tags)
2. **Set Appropriate Interval** - Balance between freshness and API usage (60s recommended)
3. **Create Dashboards** - Build dashboards before you need them during incidents
4. **Set Up Monitors** - Proactive alerting prevents issues from escalating
5. **Use Events** - Correlate metrics with events for better context
6. **Review Costs** - Monitor Datadog usage to control costs (custom metrics pricing)

## API Reference

### Getting Datadog Configuration

```bash
GET /api/plugins/datadog/config
Authorization: Bearer <token>
```

**Response**:
```json
{
  "enabled": true,
  "site": "datadoghq.com",
  "enableMetrics": true,
  "enableTraces": true,
  "metricsInterval": 60
}
```

### Sending Custom Metrics

While the plugin handles most metrics automatically, you can send custom metrics via the plugin API:

```bash
POST /api/plugins/datadog/metrics
Authorization: Bearer <token>
Content-Type: application/json

{
  "metric": "streamspace.custom.metric",
  "value": 42,
  "type": "gauge",
  "tags": ["custom:tag"]
}
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Documentation: https://docs.streamspace.io/plugins/datadog
- Datadog Documentation: https://docs.datadoghq.com/

## License

MIT License - see LICENSE file for details

## Version History

- **1.0.0** (2025-01-15)
  - Initial release
  - Session, resource, and user metrics
  - Event tracking
  - Scheduled metric flushing

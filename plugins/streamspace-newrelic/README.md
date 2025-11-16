# StreamSpace New Relic Plugin

Full-stack observability integration with New Relic for metrics, events, traces, and logs.

## Features

- **Custom Metrics** - Send session, resource, and user metrics to New Relic
- **Custom Events** - Track session lifecycle and user activity events
- **APM Integration** - Distributed tracing for API requests
- **Real-time Monitoring** - Live dashboards and alerting
- **Flexible Configuration** - Track only what you need

## Installation

### Via Plugin Marketplace

1. Navigate to **Admin → Plugins**
2. Search for "New Relic Monitoring"
3. Click **Install** and configure
4. Click **Enable**

### Manual Installation

```bash
cp -r streamspace-newrelic /path/to/streamspace/plugins/
systemctl restart streamspace-api
```

## Configuration

### Required Settings

```json
{
  "enabled": true,
  "licenseKey": "your-newrelic-license-key",
  "accountId": "your-account-id",
  "region": "US"
}
```

### Full Configuration

```json
{
  "enabled": true,
  "licenseKey": "NRII-...",
  "accountId": "1234567",
  "region": "US",
  "appName": "StreamSpace Production",
  "enableMetrics": true,
  "enableEvents": true,
  "enableTraces": true,
  "enableLogs": false,
  "metricsInterval": 60,
  "trackSessionMetrics": true,
  "trackResourceMetrics": true,
  "trackUserMetrics": true,
  "customAttributes": {
    "environment": "production",
    "datacenter": "us-east-1",
    "team": "platform"
  }
}
```

### Getting Your Keys

1. Log into New Relic
2. Navigate to **Account Settings → API Keys**
3. Copy your **Ingest - License** key
4. Note your **Account ID** from the URL

### Regions

- **US**: `https://insights-collector.newrelic.com`
- **EU**: `https://insights-collector.eu01.nr-data.net`

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `streamspace.session.created` | count | Sessions created |
| `streamspace.session.terminated` | count | Sessions terminated |
| `streamspace.session.active` | gauge | Active sessions |
| `streamspace.session.duration` | gauge | Session duration (seconds) |
| `streamspace.session.cpu` | gauge | CPU usage (%) |
| `streamspace.session.memory` | gauge | Memory usage (bytes) |
| `streamspace.session.storage` | gauge | Storage usage (bytes) |
| `streamspace.user.created` | count | Users created |
| `streamspace.user.login` | count | User logins |
| `streamspace.user.logout` | count | User logouts |

## Events

- **SessionCreated** - New session started
- **SessionTerminated** - Session ended
- **UserCreated** - New user registered
- **PluginLoaded** - Plugin activated
- **PluginUnloaded** - Plugin deactivated

## Usage

### Query Metrics (NRQL)

```sql
-- Active sessions over time
SELECT average(streamspace.session.active)
FROM Metric
SINCE 1 hour ago
TIMESERIES

-- Session duration by template
SELECT average(streamspace.session.duration)
FROM Metric
FACET template
SINCE 1 day ago

-- CPU usage by session
SELECT max(streamspace.session.cpu)
FROM Metric
FACET sessionId
SINCE 30 minutes ago
```

### Query Events (NRQL)

```sql
-- Recent session creations
SELECT * FROM SessionCreated
SINCE 1 hour ago

-- User activity
SELECT count(*) FROM UserCreated, UserLogin
FACET eventType
SINCE 1 day ago

-- Session duration histogram
SELECT histogram(duration, 100, 10)
FROM SessionTerminated
SINCE 1 day ago
```

### Create Dashboards

1. Go to **Dashboards → Create dashboard**
2. Add widgets with NRQL queries
3. Set up auto-refresh intervals
4. Share with team

### Create Alerts

```sql
-- Alert: High active sessions
SELECT average(streamspace.session.active)
FROM Metric
WHERE appName = 'StreamSpace'

Threshold: Alert when > 100 for 5 minutes
```

```sql
-- Alert: High CPU usage
SELECT max(streamspace.session.cpu)
FROM Metric

Threshold: Alert when > 90 for 10 minutes
```

```sql
-- Alert: Long-running sessions
SELECT max(duration) FROM SessionTerminated

Threshold: Alert when > 28800 (8 hours)
```

## Troubleshooting

### Metrics not appearing

- Verify license key and account ID
- Check region setting (US vs EU)
- Review logs: `tail -f /var/log/streamspace/plugins/newrelic.log`
- Wait 1-2 minutes for data to appear

### Authentication errors

- Regenerate license key in New Relic
- Ensure using **Ingest - License** key (not User API key)
- Check key hasn't been deleted or rotated

### High data ingestion costs

- Reduce `metricsInterval` (increase from 60 to 120+ seconds)
- Disable `trackResourceMetrics` if not needed
- Use fewer custom attributes
- Review New Relic pricing and data limits

## Best Practices

1. **Start Simple** - Enable basic session metrics first
2. **Use Custom Attributes** - Add environment, region, team tags
3. **Set Up Alerts** - Proactive monitoring prevents issues
4. **Create Dashboards** - Visualize trends before incidents
5. **Monitor Costs** - Track data ingestion to control New Relic bills

## Support

- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Docs: https://docs.streamspace.io/plugins/newrelic
- New Relic Docs: https://docs.newrelic.com/

## License

MIT License

## Version History

- **1.0.0** (2025-01-15) - Initial release with metrics, events, and custom attributes

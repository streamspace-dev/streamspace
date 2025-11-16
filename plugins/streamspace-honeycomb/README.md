# StreamSpace Honeycomb Plugin

High-definition observability integration with Honeycomb for deep system analysis and debugging.

## Features

- **High-Cardinality Events** - Track unlimited unique dimensions
- **Deep Debugging** - Drill down into any attribute or combination
- **Session Tracking** - Complete session lifecycle with duration
- **Resource Monitoring** - CPU, memory, storage metrics
- **User Activity** - Track user behavior patterns
- **BubbleUp Analysis** - Automatically find outliers and anomalies
- **Custom Fields** - Add any metadata to events

## Installation

1. Navigate to **Admin → Plugins**
2. Search for "Honeycomb Observability"
3. Click **Install** and configure
4. Click **Enable**

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "apiKey": "your-honeycomb-api-key",
  "dataset": "streamspace"
}
```

### Full Configuration

```json
{
  "enabled": true,
  "apiKey": "hcaik_1234567890abcdef",
  "dataset": "streamspace-production",
  "apiHost": "https://api.honeycomb.io",
  "sampleRate": 1,
  "sendFrequency": 1000,
  "maxBatchSize": 100,
  "trackSessions": true,
  "trackResources": true,
  "trackUsers": true,
  "enableTracing": true,
  "customFields": {
    "service": "streamspace",
    "environment": "production",
    "region": "us-east-1",
    "team": "platform"
  }
}
```

### Getting Your API Key

1. Log into Honeycomb.io
2. Navigate to **Team Settings → API Keys**
3. Create new key or copy existing
4. Use in plugin configuration

## Events Sent

### Session Events

- **session.created** - New session started
  - Fields: `session_id`, `user_id`, `template`, `duration_ms`
- **session.terminated** - Session ended
  - Fields: `session_id`, `user_id`, `template`, `duration_ms`, `duration_sec`
- **session.heartbeat** - Resource usage snapshot
  - Fields: `session_id`, `cpu_usage_percent`, `memory_mb`, `storage_mb`

### User Events

- **user.created** - New user registered
  - Fields: `user_id`
- **user.login** - User logged in
  - Fields: `user_id`
- **user.logout** - User logged out
  - Fields: `user_id`

### Plugin Events

- **plugin.loaded** - Plugin activated
- **plugin.unloaded** - Plugin deactivated

## Usage in Honeycomb

### Query Examples

#### Find Slow Sessions
```
VISUALIZE: HEATMAP(duration_sec)
WHERE: name = "session.terminated"
GROUP BY: template
```

#### Session Count by User
```
VISUALIZE: COUNT
WHERE: name = "session.created"
GROUP BY: user_id
```

#### High CPU Usage
```
VISUALIZE: P99(cpu_usage_percent)
WHERE: name = "session.heartbeat"
GROUP BY: template
```

#### Memory Usage Trends
```
VISUALIZE: AVG(memory_mb)
WHERE: name = "session.heartbeat"
GROUP BY: session_id
```

### BubbleUp Analysis

Automatically find what's different about slow sessions:

1. Create query: `WHERE name = "session.terminated"`
2. Filter for slow sessions: `duration_sec > 3600`
3. Click **BubbleUp**
4. Honeycomb shows which attributes correlate with slow sessions

### Tracing

View distributed traces:

1. Query: `WHERE name = "session.created"`
2. Click on an event
3. View **Trace Timeline**
4. See all related events in chronological order

## Queries & Boards

### Session Overview Board

```
Widget 1: Session Rate
- COUNT WHERE name = "session.created"
- VISUALIZE: Line chart, Group by time

Widget 2: Active Sessions by Template
- COUNT WHERE name IN ("session.created", "session.terminated")
- VISUALIZE: Stacked area, Group by template

Widget 3: Average Session Duration
- AVG(duration_sec) WHERE name = "session.terminated"
- VISUALIZE: Heatmap, Group by template

Widget 4: CPU Usage Distribution
- HEATMAP(cpu_usage_percent) WHERE name = "session.heartbeat"
- VISUALIZE: Heatmap
```

### Resource Utilization Board

```
Widget 1: CPU Usage P99
- P99(cpu_usage_percent) WHERE name = "session.heartbeat"
- VISUALIZE: Line chart, Group by template

Widget 2: Memory Usage Trend
- AVG(memory_mb) WHERE name = "session.heartbeat"
- VISUALIZE: Line chart, Group by session_id (top 10)

Widget 3: Storage by User
- SUM(storage_mb) WHERE name = "session.heartbeat"
- VISUALIZE: Bar chart, Group by user_id
```

## Triggers (Alerts)

### High Session Creation Rate

```
Query: COUNT WHERE name = "session.created"
Frequency: Check every 1 minute
Threshold: Alert when > 100
Recipients: #platform-alerts Slack channel
```

### Long-Running Sessions

```
Query: MAX(duration_sec) WHERE name = "session.terminated"
Frequency: Check every 5 minutes
Threshold: Alert when > 28800 (8 hours)
Recipients: ops-team@company.com
```

### High CPU Usage

```
Query: AVG(cpu_usage_percent) WHERE name = "session.heartbeat"
Frequency: Check every 1 minute
Threshold: Alert when > 90
Recipients: PagerDuty integration
```

## Best Practices

1. **Start Broad** - Query all events, then filter down
2. **Use BubbleUp** - Let Honeycomb find patterns automatically
3. **Add Context** - Use customFields for environment, region, version
4. **Create Boards** - Build dashboards for common views
5. **Set Up Triggers** - Proactive alerts on anomalies
6. **Sample Wisely** - Use sampleRate=1 unless very high volume
7. **Batch Events** - Don't set sendFrequency too low

## Sampling

Control data volume and costs:

```json
{
  "sampleRate": 10  // 1 in 10 events (10%)
}
```

Honeycomb adjusts counts automatically when displaying results.

**Recommendations**:
- Development: `sampleRate: 1` (100%)
- Production (low volume): `sampleRate: 1` (100%)
- Production (high volume): `sampleRate: 10-100` (10%-1%)

## Troubleshooting

### Events not appearing

- Verify API key is correct
- Check dataset name matches
- Review plugin logs
- Wait 10-30 seconds for events to appear
- Check Honeycomb team quota

### High costs

- Increase `sampleRate` (10, 100, 1000)
- Reduce `maxBatchSize`
- Disable `trackResources` if not needed (high frequency)
- Review Honeycomb pricing and event volume

### Missing fields

- Ensure custom fields are in `customFields` config
- Check event data contains expected fields
- Verify no field name conflicts

## Advanced Features

### Derived Columns

Create calculated fields in Honeycomb:

```
Column: session_hours
Formula: duration_sec / 3600
```

### Query Specifications

Save complex queries:

1. Build query in Honeycomb
2. Click **Save Query Spec**
3. Share with team
4. Reuse in multiple boards

### Service Level Objectives (SLOs)

Track service health:

```
SLO: 99% of sessions start within 5 seconds
Query: P99(duration_ms) WHERE name = "session.created" < 5000
```

## Support

- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Docs: https://docs.streamspace.io/plugins/honeycomb
- Honeycomb Docs: https://docs.honeycomb.io/

## License

MIT License

## Version History

- **1.0.0** (2025-01-15)
  - Initial release
  - Session, resource, and user tracking
  - High-cardinality events
  - BubbleUp support
  - Distributed tracing

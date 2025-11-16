# StreamSpace Elastic APM Plugin

Application Performance Monitoring integration with Elastic APM for distributed tracing and performance analysis.

## Features

- **Distributed Tracing** - Track requests across services
- **Performance Monitoring** - Identify slow transactions and bottlenecks
- **Resource Tracking** - Monitor CPU, memory, storage usage
- **Session Lifecycle** - Track session creation, duration, termination
- **Custom Labels** - Tag transactions for filtering and analysis
- **Error Tracking** - Capture and analyze errors

## Installation

1. Navigate to **Admin → Plugins**
2. Search for "Elastic APM"
3. Click **Install** and configure
4. Click **Enable**

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "serverUrl": "http://apm-server:8200",
  "serviceName": "streamspace",
  "environment": "production"
}
```

### With Authentication

```json
{
  "enabled": true,
  "serverUrl": "https://your-deployment.apm.elastic-cloud.com:443",
  "secretToken": "your-secret-token",
  "serviceName": "streamspace",
  "serviceVersion": "1.0.0",
  "environment": "production",
  "transactionSampleRate": 1.0,
  "globalLabels": {
    "team": "platform",
    "region": "us-east-1"
  }
}
```

### Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `serverUrl` | string | *required* | APM Server URL |
| `secretToken` | string | - | Authentication token |
| `apiKey` | string | - | API key (alternative to token) |
| `serviceName` | string | `streamspace` | Service name in APM |
| `serviceVersion` | string | `1.0.0` | Service version |
| `environment` | string | `production` | Environment name |
| `transactionSampleRate` | number | `1.0` | Sample rate (0.0-1.0) |
| `captureBody` | string | `errors` | Capture request bodies |
| `captureHeaders` | boolean | `true` | Capture HTTP headers |
| `globalLabels` | object | `{}` | Labels for all events |

## Usage

### View in Kibana

1. Open Kibana
2. Navigate to **Observability → APM**
3. Select **streamspace** service
4. View:
   - **Transactions** - Session lifecycle events
   - **Errors** - Captured errors and exceptions
   - **Metrics** - CPU, memory, throughput
   - **Service Map** - Dependencies and connections

### Transaction Types

- **session-lifecycle** - Session creation/termination
- **session-monitor** - Heartbeat and resource monitoring
- **user-lifecycle** - User creation and activity
- **plugin-lifecycle** - Plugin load/unload

### Analyzing Performance

#### Slow Transactions
```
APM → Transactions → Sort by Latency
- Identify slow session operations
- Analyze transaction timeline
- Review span details
```

#### Error Rate
```
APM → Errors → Group by error type
- See most common errors
- Track error trends over time
- Link to affected transactions
```

#### Resource Usage
```
APM → Metrics → Select metric
- CPU usage trends
- Memory consumption patterns
- Session count over time
```

## Elastic Cloud Setup

### Getting APM Credentials

1. Log into Elastic Cloud
2. Create deployment or use existing
3. Navigate to **APM & Fleet**
4. Copy **APM Server URL** and **Secret Token**
5. Use these in plugin configuration

### Self-Hosted APM Server

If running your own APM Server:

```yaml
# apm-server.yml
apm-server:
  host: "0.0.0.0:8200"
  secret_token: "your-secret-token"

output.elasticsearch:
  hosts: ["localhost:9200"]
```

## Best Practices

1. **Sample Rate** - Use 1.0 in development, 0.1-0.5 in production
2. **Global Labels** - Add environment, region, team for filtering
3. **Service Versions** - Update version on each deployment
4. **Monitor Errors** - Set up alerts for error spikes
5. **Review Weekly** - Check slow transactions and optimize

## Troubleshooting

### Transactions not appearing

- Check APM server URL is accessible
- Verify secret token is correct
- Review APM server logs
- Ensure `transactionSampleRate` > 0

### High APM costs

- Reduce `transactionSampleRate` (e.g., 0.1 = 10%)
- Disable `captureBody` if not needed
- Limit `transactionMaxSpans`
- Review Elastic Cloud pricing

### Missing spans

- Increase `transactionMaxSpans`
- Check `stackTraceLimit` setting
- Verify transactions are being sampled

## Support

- GitHub: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Docs: https://docs.streamspace.io/plugins/elastic-apm
- Elastic APM Docs: https://www.elastic.co/guide/en/apm/get-started/current/overview.html

## License

MIT License

## Version History

- **1.0.0** (2025-01-15) - Initial release with distributed tracing and performance monitoring

# StreamSpace Advanced Analytics & Reporting Plugin

Comprehensive analytics and reporting system for usage trends, session metrics, user engagement, resource utilization, and cost analysis.

## Features

### Usage Analytics
- **Trends Analysis**: Time-series data for sessions, users, and teams
- **Template Usage**: Most popular templates and usage patterns
- **User Analytics**: Per-user and per-team usage breakdown
- **Historical Data**: Up to 365 days of historical trends

### Session Analytics
- **Duration Analysis**: Session length distribution with percentiles
- **Lifecycle Metrics**: Session states and transitions
- **Peak Usage Times**: Hourly and daily peak usage patterns
- **Session Quality**: Average duration, connection stability

### User Engagement
- **Active Users**: DAU (Daily Active Users), WAU, MAU metrics
- **Retention Analysis**: User retention and churn rates
- **Engagement Ratios**: DAU/WAU, DAU/MAU ratios
- **Power Users**: Identify highly engaged users (10+ sessions/month)

### Resource Analytics
- **Utilization Metrics**: CPU, memory, storage usage
- **Resource Trends**: Historical resource consumption
- **Waste Detection**: Idle sessions, short sessions, underutilized resources
- **Optimization Recommendations**: Actionable insights to reduce waste

### Cost Analytics
- **Cost Estimation**: Calculate infrastructure costs based on usage
- **Cost by Team**: Team-level cost breakdown
- **Cost by Template**: Template-level cost analysis
- **Top Spenders**: Identify highest-cost users and teams

### Automated Reports
- **Daily Reports**: Comprehensive daily summary with key metrics
- **Weekly Reports**: Week-over-week trends and insights
- **Monthly Reports**: Month-over-month analysis
- **Email Delivery**: Scheduled report delivery to stakeholders

## Installation

Admin → Plugins → "Advanced Analytics & Reporting" → Install

## Configuration

```json
{
  "enabled": true,
  "costModel": {
    "cpuCostPerHour": 0.01,
    "memCostPerGBHour": 0.005,
    "storageCostPerGBMonth": 0.10
  },
  "retentionDays": 90,
  "reportSchedule": {
    "dailyEnabled": true,
    "weeklyEnabled": true,
    "monthlyEnabled": true,
    "emailRecipients": ["admin@example.com"]
  },
  "thresholds": {
    "shortSessionMinutes": 5,
    "idleTimeoutMinutes": 30
  }
}
```

## API Endpoints

### Usage Analytics
- `GET /analytics/usage/trends?days=30` - Usage trends over time
- `GET /analytics/usage/by-template?days=30` - Usage grouped by template
- `GET /analytics/usage/by-user` - Per-user usage statistics
- `GET /analytics/usage/by-team` - Per-team usage statistics

### Session Analytics
- `GET /analytics/sessions/duration` - Session duration distribution
- `GET /analytics/sessions/lifecycle` - Session lifecycle metrics
- `GET /analytics/sessions/peak-times` - Peak usage by hour and day

### User Engagement
- `GET /analytics/engagement/active-users` - DAU, WAU, MAU metrics
- `GET /analytics/engagement/retention` - User retention analysis
- `GET /analytics/engagement/frequency` - Usage frequency patterns

### Resource Analytics
- `GET /analytics/resources/utilization` - Current resource utilization
- `GET /analytics/resources/trends` - Historical resource trends
- `GET /analytics/resources/waste` - Waste detection and recommendations

### Cost Analytics
- `GET /analytics/cost/estimate` - Overall cost estimate
- `GET /analytics/cost/by-team` - Team-level cost breakdown
- `GET /analytics/cost/by-template` - Template-level cost analysis

### Reports
- `GET /analytics/reports/daily?date=2025-01-15` - Daily summary report
- `GET /analytics/reports/weekly` - Weekly summary report
- `GET /analytics/reports/monthly` - Monthly summary report

## Example: Usage Trends

**Request**:
```bash
GET /analytics/usage/trends?days=7
```

**Response**:
```json
{
  "trends": [
    {
      "date": "2025-01-15",
      "totalSessions": 142,
      "runningSessions": 38,
      "uniqueUsers": 67,
      "teamsActive": 12
    },
    ...
  ],
  "period": "7 days"
}
```

## Example: Cost Estimate

**Request**:
```bash
GET /analytics/cost/estimate
```

**Response**:
```json
{
  "period": "30 days",
  "totalCost": {
    "cpu": 125.50,
    "memory": 62.75,
    "total": 188.25
  },
  "totalSessionHours": 12550,
  "costModel": {
    "cpuCostPerHour": 0.01,
    "memCostPerHour": 0.005
  },
  "topUserCosts": [
    {
      "userId": "user123",
      "hours": 245.5,
      "estimatedCost": 4.91
    }
  ],
  "note": "Costs are estimates based on session duration and resource allocation"
}
```

## Example: Resource Waste

**Request**:
```bash
GET /analytics/resources/waste
```

**Response**:
```json
{
  "waste": {
    "shortSessions": 23,
    "longIdleSessions": 15,
    "shouldBeHibernated": 8
  },
  "recommendations": [
    "Consider auto-hibernation after 30 minutes of inactivity (15 sessions affected)",
    "Review short sessions to identify configuration issues (23 sessions)",
    "Enable aggressive hibernation to save resources (8 sessions ready)"
  ]
}
```

## Scheduled Jobs

### Generate Daily Report
- **Schedule**: Daily at 1:00 AM
- **Description**: Generates comprehensive daily analytics report
- **Storage**: Saved to `analytics_reports` table
- **Email**: Sent to configured recipients (if enabled)

### Cleanup Old Analytics
- **Schedule**: Weekly on Sunday at 2:00 AM
- **Description**: Removes analytics data older than retention period
- **Retention**: Configurable (default: 90 days)

## Database Schema

### analytics_cache
Caches expensive analytics queries for performance.

```sql
CREATE TABLE analytics_cache (
  id SERIAL PRIMARY KEY,
  cache_key VARCHAR(255) UNIQUE,
  data JSONB,
  expires_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

### analytics_reports
Stores generated reports for historical reference.

```sql
CREATE TABLE analytics_reports (
  id SERIAL PRIMARY KEY,
  report_type VARCHAR(100),   -- 'daily', 'weekly', 'monthly'
  report_date DATE,
  data JSONB,
  generated_at TIMESTAMP DEFAULT NOW()
);
```

## Cost Model Configuration

Configure your infrastructure costs to get accurate cost estimates:

- **cpuCostPerHour**: Cost per CPU core per hour (default: $0.01)
- **memCostPerGBHour**: Cost per GB of memory per hour (default: $0.005)
- **storageCostPerGBMonth**: Cost per GB of storage per month (default: $0.10)

Example AWS pricing:
```json
{
  "cpuCostPerHour": 0.0416,      // t3.medium vCPU cost
  "memCostPerGBHour": 0.0052,    // t3.medium memory cost
  "storageCostPerGBMonth": 0.10  // EBS gp3 storage cost
}
```

Example Azure pricing:
```json
{
  "cpuCostPerHour": 0.0452,      // B2s vCPU cost
  "memCostPerGBHour": 0.0113,    // B2s memory cost
  "storageCostPerGBMonth": 0.05  // Standard SSD cost
}
```

## Performance Optimization

The plugin uses several techniques to ensure fast analytics:

1. **Query Caching**: Expensive queries are cached with configurable TTL
2. **Aggregation Tables**: Pre-computed aggregates for common queries
3. **Indexed Columns**: Database indexes on frequently queried columns
4. **Batch Processing**: Reports generated asynchronously
5. **Retention Policies**: Old data automatically pruned

## Metrics Collected

- Total sessions created
- Active sessions
- Unique users (daily, weekly, monthly)
- Session duration (avg, median, percentiles)
- Template usage counts
- Team activity
- Resource consumption (CPU, memory, storage)
- Connection counts
- Session state transitions
- Peak usage times

## Use Cases

### Infrastructure Planning
Use trends and resource utilization data to forecast capacity needs and plan infrastructure scaling.

### Cost Optimization
Identify resource waste, idle sessions, and high-cost users to optimize spending.

### User Engagement
Track DAU/WAU/MAU metrics to measure platform adoption and user engagement.

### Template Performance
Analyze which templates are most popular and how users interact with them.

### Compliance Reporting
Generate historical reports for audit and compliance requirements.

## License
MIT

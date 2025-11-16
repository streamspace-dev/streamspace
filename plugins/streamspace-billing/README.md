# StreamSpace Billing Plugin

Comprehensive billing and usage tracking system for StreamSpace with Stripe integration.

## Features

### Usage Tracking
- **Real-time session tracking** - Monitor CPU, memory, and storage usage for all active sessions
- **Hourly usage calculation** - Automated usage metering with configurable intervals
- **Resource-based pricing** - Separate pricing for CPU cores, memory (GB), and storage
- **Historical usage data** - Complete audit trail of resource consumption

### Billing Modes
- **Usage-based** - Pay only for what you use (compute hours, storage)
- **Subscription** - Fixed monthly/annual plans with quotas
- **Hybrid** - Combination of subscription base + usage overages

### Invoicing
- **Automated invoice generation** - Monthly invoices created automatically
- **Customizable invoice day** - Choose day of month for billing (1-28)
- **Credits support** - Apply credits to reduce invoice totals
- **Multiple invoice statuses** - Draft, sent, paid, overdue

### Payment Processing
- **Stripe integration** - Secure payment processing via Stripe
- **Checkout sessions** - Pre-built checkout pages for subscriptions
- **Payment methods** - Support for cards, ACH, and other Stripe methods
- **Webhook handling** - Real-time payment confirmation

### Quota Management
- **Usage alerts** - Notify users when approaching quota limits (80% default)
- **Auto-suspend** - Optionally suspend sessions when quota exceeded
- **Grace period** - Configurable grace period before service suspension
- **Per-user quotas** - Different limits for different subscription tiers

### Admin Features
- **Billing dashboard** - View all users' billing status
- **Manual credits** - Add credits to user accounts
- **Invoice management** - Manually generate or modify invoices
- **Usage reports** - Export usage data for analysis

## Installation

### Via Plugin Marketplace (Recommended)

1. Navigate to **Admin → Plugins**
2. Search for "Billing & Usage Tracking"
3. Click **Install**
4. Configure settings (see Configuration section)
5. Click **Enable**

### Manual Installation

```bash
# Copy plugin files to plugins directory
cp -r streamspace-billing /path/to/streamspace/plugins/

# Restart StreamSpace API
systemctl restart streamspace-api
```

## Configuration

### Basic Setup

```json
{
  "enabled": true,
  "billingMode": "usage",
  "computeRates": {
    "cpu_per_core_hour": 0.05,
    "memory_per_gb_hour": 0.01,
    "storage_per_gb_month": 0.10
  }
}
```

### Stripe Integration

```json
{
  "stripeEnabled": true,
  "stripeSecretKey": "sk_live_...",
  "stripeWebhookSecret": "whsec_..."
}
```

**Important:** Never commit Stripe keys to version control. Use environment variables or secrets management.

### Subscription Plans

```json
{
  "billingMode": "subscription",
  "subscriptionPlans": [
    {
      "id": "free",
      "name": "Free Tier",
      "price": 0,
      "interval": "month",
      "cpu_limit": 2,
      "memory_limit": 4,
      "storage_limit": 10
    },
    {
      "id": "pro",
      "name": "Professional",
      "price": 29.99,
      "interval": "month",
      "cpu_limit": 8,
      "memory_limit": 16,
      "storage_limit": 100
    },
    {
      "id": "enterprise",
      "name": "Enterprise",
      "price": 99.99,
      "interval": "month",
      "cpu_limit": 32,
      "memory_limit": 64,
      "storage_limit": 500
    }
  ]
}
```

### Usage Calculation

```json
{
  "usageCalculationInterval": "0 * * * *",
  "invoiceDay": 1,
  "alertThreshold": 80,
  "autoSuspendOnOverage": false,
  "gracePeriodDays": 7
}
```

## Usage

### For End Users

#### View Current Usage

1. Navigate to **Billing & Usage** in the sidebar
2. View current month's usage breakdown
3. See costs by resource type (CPU, memory, storage)

#### View Invoices

1. Go to **Billing & Usage → Invoices**
2. Download PDF invoices
3. View payment history

#### Manage Subscription

1. Go to **Billing & Usage → Subscription**
2. Upgrade or downgrade plan
3. Update payment method

### For Administrators

#### View All Billing

1. Navigate to **Admin → Billing Management**
2. View usage across all users
3. Filter by user, date range, or status

#### Add Credits

```bash
# Via API
curl -X POST https://streamspace.example.com/api/plugins/billing/credits \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "user_id": "john@example.com",
    "amount": 50.00,
    "reason": "Service credit for downtime"
  }'
```

#### Generate Manual Invoice

```bash
# Via API
curl -X POST https://streamspace.example.com/api/plugins/billing/invoices \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "user_id": "john@example.com",
    "period_start": "2025-01-01",
    "period_end": "2025-01-31"
  }'
```

## Database Schema

The plugin creates the following tables:

### billing_usage_records
- Tracks individual usage events (CPU hours, memory hours, storage)
- Used for detailed usage reports and invoice line items

### billing_invoices
- Stores generated invoices with totals and status
- Links to usage records for detailed breakdowns

### billing_subscriptions
- Manages user subscription plans and periods
- Integrates with Stripe subscription IDs

### billing_payments
- Records payment transactions
- Links invoices to Stripe payment intents

### billing_credits
- Stores account credits with expiration dates
- Applied automatically to invoices

## API Endpoints

### User Endpoints

- `GET /api/plugins/billing/usage` - Current usage and costs
- `GET /api/plugins/billing/invoices` - User's invoices
- `GET /api/plugins/billing/subscription` - Active subscription
- `POST /api/plugins/billing/create-checkout` - Start Stripe checkout
- `GET /api/plugins/billing/payment-methods` - Saved payment methods

### Admin Endpoints

- `GET /api/plugins/billing/admin/users` - All users' billing status
- `POST /api/plugins/billing/admin/credits` - Add credits to account
- `POST /api/plugins/billing/admin/invoices` - Generate manual invoice
- `GET /api/plugins/billing/admin/reports` - Usage reports

## Events

The plugin emits the following events:

- `billing.quota.warning` - User approaching quota limit
- `billing.quota.exceeded` - User exceeded quota
- `billing.invoice.created` - New invoice generated
- `billing.invoice.paid` - Invoice payment received
- `billing.payment.failed` - Payment attempt failed

## Scheduled Jobs

- **calculate-usage** - Runs every hour (configurable)
  - Calculates usage for all active sessions
  - Updates usage records in database

- **generate-invoices** - Runs monthly on configured day
  - Generates invoices for all users
  - Sends invoice emails (if email plugin enabled)

- **check-quotas** - Runs every 15 minutes
  - Checks users against quota limits
  - Emits warnings when thresholds exceeded

## Pricing Examples

### Usage-Based Pricing

**Configuration:**
- CPU: $0.05/core-hour
- Memory: $0.01/GB-hour
- Storage: $0.10/GB-month

**Example Session:**
- 2 CPU cores for 10 hours = 20 core-hours × $0.05 = $1.00
- 4 GB memory for 10 hours = 40 GB-hours × $0.01 = $0.40
- **Total: $1.40**

### Subscription Pricing

**Pro Plan:** $29.99/month
- Includes 8 CPU cores, 16 GB memory, 100 GB storage
- Overages charged at usage rates
- Example: 10 cores used = 2 cores × $0.05/hour overage

## Stripe Integration

### Setup Stripe Webhook

1. In Stripe Dashboard, go to **Developers → Webhooks**
2. Add endpoint: `https://streamspace.example.com/api/plugins/billing/webhook`
3. Select events:
   - `invoice.paid`
   - `invoice.payment_failed`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
4. Copy webhook signing secret to plugin config

### Test Stripe Integration

```bash
# Use Stripe CLI for local testing
stripe listen --forward-to http://localhost:8080/api/plugins/billing/webhook

# Trigger test events
stripe trigger payment_intent.succeeded
stripe trigger invoice.paid
```

## Troubleshooting

### Usage not tracking

**Problem:** Sessions created but no usage recorded

**Solution:**
- Check plugin is enabled
- Verify `session.created` event is firing
- Check plugin logs: `tail -f /var/log/streamspace/plugins/billing.log`

### Invoices not generating

**Problem:** Monthly invoices not created automatically

**Solution:**
- Check scheduled job is running: `GET /api/plugins/billing/jobs/status`
- Verify `invoiceDay` configuration
- Manually trigger: `POST /api/plugins/billing/jobs/generate-invoices`

### Stripe payments failing

**Problem:** Users unable to complete checkout

**Solution:**
- Verify Stripe API keys are correct
- Check webhook is configured and receiving events
- Review Stripe Dashboard logs
- Ensure test mode keys used in development

## Best Practices

1. **Start with test mode** - Use Stripe test keys until ready for production
2. **Monitor quotas** - Set up alerts before users hit limits
3. **Regular reports** - Review monthly usage patterns
4. **Credit policy** - Have clear policy for issuing credits
5. **Grace periods** - Don't suspend immediately on payment failure
6. **Backup billing data** - Include billing tables in database backups

## Support

For issues or questions:
- GitHub Issues: https://github.com/JoshuaAFerguson/streamspace-plugins/issues
- Documentation: https://docs.streamspace.io/plugins/billing
- Community: https://discord.gg/streamspace

## License

MIT License - see LICENSE file for details

## Version History

- **1.0.0** (2025-01-15)
  - Initial release
  - Usage tracking and invoicing
  - Stripe integration
  - Quota management
  - Admin dashboard

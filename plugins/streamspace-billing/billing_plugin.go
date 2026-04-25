package billingplugin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace-dev/streamspace/api/internal/plugins"
)

// BillingPlugin implements comprehensive billing and usage tracking
type BillingPlugin struct {
	plugins.BasePlugin

	// Usage tracking cache
	activeSessionUsage map[string]*SessionUsage
}

// SessionUsage tracks active session resource usage
type SessionUsage struct {
	SessionID     string
	UserID        string
	StartTime     time.Time
	LastHeartbeat time.Time
	CPUCores      float64
	MemoryGB      float64
	StorageGB     float64
	TotalCost     float64
}

// UsageRecord represents a billing usage record
type UsageRecord struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	SessionID    string    `json:"session_id,omitempty"`
	ResourceType string    `json:"resource_type"` // cpu, memory, storage
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"` // core-hours, gb-hours, gb-months
	UnitPrice    float64   `json:"unit_price"`
	TotalCost    float64   `json:"total_cost"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	CreatedAt    time.Time `json:"created_at"`
}

// Invoice represents a billing invoice
type Invoice struct {
	ID            int64     `json:"id"`
	UserID        string    `json:"user_id"`
	InvoiceNumber string    `json:"invoice_number"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
	Subtotal      float64   `json:"subtotal"`
	Credits       float64   `json:"credits"`
	Total         float64   `json:"total"`
	Status        string    `json:"status"` // draft, sent, paid, overdue
	DueDate       time.Time `json:"due_date"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Subscription represents a user subscription
type Subscription struct {
	ID              int64      `json:"id"`
	UserID          string     `json:"user_id"`
	PlanID          string     `json:"plan_id"`
	Status          string     `json:"status"` // active, canceled, suspended
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	StripeSubID     string     `json:"stripe_subscription_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	CanceledAt      *time.Time `json:"canceled_at,omitempty"`
}

// NewBillingPlugin creates a new billing plugin instance
func NewBillingPlugin() *BillingPlugin {
	return &BillingPlugin{
		BasePlugin:         plugins.BasePlugin{Name: "streamspace-billing"},
		activeSessionUsage: make(map[string]*SessionUsage),
	}
}

// OnLoad is called when the plugin is loaded
func (p *BillingPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Billing plugin loading", map[string]interface{}{
		"version": "1.0.0",
	})

	// Create database tables
	if err := p.createDatabaseTables(ctx); err != nil {
		return fmt.Errorf("failed to create database tables: %w", err)
	}

	// Register API endpoints
	p.registerAPIEndpoints(ctx)

	// Register UI components
	p.registerUIComponents(ctx)

	// Schedule periodic jobs
	p.scheduleJobs(ctx)

	ctx.Logger.Info("Billing plugin loaded successfully")
	return nil
}

// OnUnload is called when the plugin is unloaded
func (p *BillingPlugin) OnUnload(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Billing plugin unloading")

	// Save any pending usage records
	for sessionID, usage := range p.activeSessionUsage {
		if err := p.recordUsage(ctx, usage); err != nil {
			ctx.Logger.Warn("Failed to save usage for session", map[string]interface{}{
				"sessionId": sessionID,
				"error":     err.Error(),
			})
		}
	}

	return nil
}

// OnSessionCreated tracks when a session starts
func (p *BillingPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	enabled := p.getBool(ctx.Config, "enabled")
	if !enabled {
		return nil
	}

	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	sessionID := p.getString(sessionMap, "id")
	userID := p.getString(sessionMap, "user")

	// Extract resource allocation
	cpuCores := 1.0 // Default
	memoryGB := 2.0 // Default

	if resources, ok := sessionMap["resources"].(map[string]interface{}); ok {
		if cpu := p.getString(resources, "cpu"); cpu != "" {
			// Parse CPU (e.g., "1000m" = 1 core)
			cpuCores = p.parseCPU(cpu)
		}
		if memory := p.getString(resources, "memory"); memory != "" {
			// Parse memory (e.g., "2Gi" = 2 GB)
			memoryGB = p.parseMemory(memory)
		}
	}

	// Start tracking usage
	p.activeSessionUsage[sessionID] = &SessionUsage{
		SessionID:     sessionID,
		UserID:        userID,
		StartTime:     time.Now(),
		LastHeartbeat: time.Now(),
		CPUCores:      cpuCores,
		MemoryGB:      memoryGB,
		TotalCost:     0,
	}

	ctx.Logger.Info("Started tracking session usage", map[string]interface{}{
		"sessionId": sessionID,
		"userId":    userID,
		"cpuCores":  cpuCores,
		"memoryGB":  memoryGB,
	})

	return nil
}

// OnSessionTerminated records final usage when session ends
func (p *BillingPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session data type")
	}

	sessionID := p.getString(sessionMap, "id")

	usage, exists := p.activeSessionUsage[sessionID]
	if !exists {
		return nil // Not tracking this session
	}

	// Record final usage
	if err := p.recordUsage(ctx, usage); err != nil {
		ctx.Logger.Warn("Failed to record usage", map[string]interface{}{
			"sessionId": sessionID,
			"error":     err.Error(),
		})
	}

	// Remove from active tracking
	delete(p.activeSessionUsage, sessionID)

	ctx.Logger.Info("Recorded final session usage", map[string]interface{}{
		"sessionId": sessionID,
		"totalCost": usage.TotalCost,
	})

	return nil
}

// OnSessionHeartbeat updates last activity time
func (p *BillingPlugin) OnSessionHeartbeat(ctx *plugins.PluginContext, session interface{}) error {
	sessionMap, ok := session.(map[string]interface{})
	if !ok {
		return nil
	}

	sessionID := p.getString(sessionMap, "id")

	if usage, exists := p.activeSessionUsage[sessionID]; exists {
		usage.LastHeartbeat = time.Now()
	}

	return nil
}

// OnUserCreated sets up billing for new user
func (p *BillingPlugin) OnUserCreated(ctx *plugins.PluginContext, user interface{}) error {
	userMap, ok := user.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid user data type")
	}

	userID := p.getString(userMap, "username")

	// Check if we should create a subscription
	billingMode := p.getString(ctx.Config, "billingMode")
	if billingMode == "subscription" || billingMode == "hybrid" {
		// Create default free tier subscription
		if err := p.createSubscription(ctx, userID, "free"); err != nil {
			ctx.Logger.Warn("Failed to create subscription", map[string]interface{}{
				"userId": userID,
				"error":  err.Error(),
			})
		}
	}

	ctx.Logger.Info("Initialized billing for user", map[string]interface{}{
		"userId": userID,
		"mode":   billingMode,
	})

	return nil
}

// createDatabaseTables creates billing database tables
func (p *BillingPlugin) createDatabaseTables(ctx *plugins.PluginContext) error {
	// Usage records table
	usageTableSchema := `
		id BIGSERIAL PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		session_id VARCHAR(255),
		resource_type VARCHAR(50) NOT NULL,
		quantity DECIMAL(10, 4) NOT NULL,
		unit VARCHAR(50) NOT NULL,
		unit_price DECIMAL(10, 4) NOT NULL,
		total_cost DECIMAL(10, 4) NOT NULL,
		start_time TIMESTAMP NOT NULL,
		end_time TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	`
	if err := ctx.Database.CreateTable("billing_usage_records", usageTableSchema); err != nil {
		return err
	}

	// Invoices table
	invoiceTableSchema := `
		id BIGSERIAL PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		invoice_number VARCHAR(100) UNIQUE NOT NULL,
		period_start TIMESTAMP NOT NULL,
		period_end TIMESTAMP NOT NULL,
		subtotal DECIMAL(10, 2) NOT NULL,
		credits DECIMAL(10, 2) DEFAULT 0,
		total DECIMAL(10, 2) NOT NULL,
		status VARCHAR(50) DEFAULT 'draft',
		due_date TIMESTAMP NOT NULL,
		paid_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT NOW()
	`
	if err := ctx.Database.CreateTable("billing_invoices", invoiceTableSchema); err != nil {
		return err
	}

	// Subscriptions table
	subscriptionTableSchema := `
		id BIGSERIAL PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		plan_id VARCHAR(100) NOT NULL,
		status VARCHAR(50) DEFAULT 'active',
		current_period_start TIMESTAMP NOT NULL,
		current_period_end TIMESTAMP NOT NULL,
		stripe_subscription_id VARCHAR(255),
		created_at TIMESTAMP DEFAULT NOW(),
		canceled_at TIMESTAMP
	`
	if err := ctx.Database.CreateTable("billing_subscriptions", subscriptionTableSchema); err != nil {
		return err
	}

	// Payments table
	paymentTableSchema := `
		id BIGSERIAL PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		invoice_id BIGINT REFERENCES billing_invoices(id),
		amount DECIMAL(10, 2) NOT NULL,
		currency VARCHAR(10) DEFAULT 'USD',
		status VARCHAR(50) DEFAULT 'pending',
		stripe_payment_intent_id VARCHAR(255),
		payment_method VARCHAR(100),
		created_at TIMESTAMP DEFAULT NOW(),
		paid_at TIMESTAMP
	`
	if err := ctx.Database.CreateTable("billing_payments", paymentTableSchema); err != nil {
		return err
	}

	// Credits table
	creditTableSchema := `
		id BIGSERIAL PRIMARY KEY,
		user_id VARCHAR(255) NOT NULL,
		amount DECIMAL(10, 2) NOT NULL,
		reason VARCHAR(255),
		expires_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT NOW()
	`
	if err := ctx.Database.CreateTable("billing_credits", creditTableSchema); err != nil {
		return err
	}

	return nil
}

// registerAPIEndpoints registers billing API endpoints
func (p *BillingPlugin) registerAPIEndpoints(ctx *plugins.PluginContext) {
	// Get current usage
	ctx.API.GET("/usage", func(c *gin.Context) {
		userID := c.GetString("userId") // From auth middleware

		usage, err := p.getCurrentUsage(ctx, userID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, usage)
	})

	// Get invoices
	ctx.API.GET("/invoices", func(c *gin.Context) {
		userID := c.GetString("userId")

		invoices, err := p.getUserInvoices(ctx, userID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"invoices": invoices})
	})

	// Get subscription
	ctx.API.GET("/subscription", func(c *gin.Context) {
		userID := c.GetString("userId")

		subscription, err := p.getUserSubscription(ctx, userID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, subscription)
	})

	// Create Stripe checkout session
	ctx.API.POST("/create-checkout", func(c *gin.Context) {
		userID := c.GetString("userId")

		var req struct {
			PlanID string `json:"plan_id"`
		}
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		checkoutURL, err := p.createStripeCheckout(ctx, userID, req.PlanID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{"checkout_url": checkoutURL})
	})
}

// registerUIComponents registers UI widgets and pages
func (p *BillingPlugin) registerUIComponents(ctx *plugins.PluginContext) {
	// Usage widget for dashboard
	ctx.UI.RegisterWidget(&plugins.UIWidget{
		ID:          "billing-usage-widget",
		Title:       "Current Usage",
		Component:   "BillingUsageWidget",
		Position:    "right-sidebar",
		Width:       "300px",
		Permissions: []string{"user"},
	})

	// Billing dashboard page
	ctx.UI.RegisterMenuItem(&plugins.UIMenuItem{
		ID:          "billing-menu",
		Label:       "Billing & Usage",
		Icon:        "receipt",
		Route:       "/billing",
		Position:    50,
		Permissions: []string{"user"},
	})

	// Admin billing page
	ctx.UI.RegisterAdminPage(&plugins.UIAdminPage{
		ID:          "admin-billing",
		Title:       "Billing Management",
		Route:       "/admin/billing",
		Component:   "AdminBillingPage",
		Icon:        "account_balance",
		Permissions: []string{"admin"},
	})
}

// scheduleJobs schedules periodic billing jobs
func (p *BillingPlugin) scheduleJobs(ctx *plugins.PluginContext) {
	// Calculate usage every hour
	interval := p.getString(ctx.Config, "usageCalculationInterval")
	if interval == "" {
		interval = "0 * * * *" // Default: hourly
	}

	ctx.Scheduler.Schedule("calculate-usage", interval, func() {
		p.calculateUsageJob(ctx)
	})

	// Generate invoices monthly
	ctx.Scheduler.Schedule("generate-invoices", "0 0 1 * *", func() {
		p.generateInvoicesJob(ctx)
	})

	// Check quotas every 15 minutes
	ctx.Scheduler.Schedule("check-quotas", "*/15 * * * *", func() {
		p.checkQuotasJob(ctx)
	})
}

// calculateUsageJob calculates usage for all active sessions
func (p *BillingPlugin) calculateUsageJob(ctx *plugins.PluginContext) {
	ctx.Logger.Info("Running usage calculation job")

	for sessionID, usage := range p.activeSessionUsage {
		// Calculate usage since last calculation
		duration := time.Since(usage.StartTime).Hours()

		// Get rates from config
		rates := p.getMap(ctx.Config, "computeRates")
		cpuRate := p.getFloat(rates, "cpu_per_core_hour")
		memoryRate := p.getFloat(rates, "memory_per_gb_hour")

		// Calculate costs
		cpuCost := usage.CPUCores * cpuRate * duration
		memoryCost := usage.MemoryGB * memoryRate * duration

		usage.TotalCost += cpuCost + memoryCost

		ctx.Logger.Debug("Calculated session usage", map[string]interface{}{
			"sessionId": sessionID,
			"cpuCost":   cpuCost,
			"memCost":   memoryCost,
		})
	}
}

// generateInvoicesJob generates invoices for all users
func (p *BillingPlugin) generateInvoicesJob(ctx *plugins.PluginContext) {
	ctx.Logger.Info("Running invoice generation job")

	// Get all users with usage in previous month
	// Generate invoices
	// This would query the database and create Invoice records
}

// checkQuotasJob checks if users are exceeding quotas
func (p *BillingPlugin) checkQuotasJob(ctx *plugins.PluginContext) {
	ctx.Logger.Debug("Checking usage quotas")

	threshold := p.getFloat(ctx.Config, "alertThreshold")
	if threshold == 0 {
		threshold = 80 // Default
	}

	// Check each user's usage against their quota
	// Emit quota.exceeded event if threshold reached
}

// recordUsage records usage to the database
func (p *BillingPlugin) recordUsage(ctx *plugins.PluginContext, usage *SessionUsage) error {
	duration := time.Since(usage.StartTime)

	rates := p.getMap(ctx.Config, "computeRates")
	cpuRate := p.getFloat(rates, "cpu_per_core_hour")
	memoryRate := p.getFloat(rates, "memory_per_gb_hour")

	cpuHours := usage.CPUCores * duration.Hours()
	memoryGBHours := usage.MemoryGB * duration.Hours()

	// Record CPU usage
	cpuRecord := map[string]interface{}{
		"user_id":       usage.UserID,
		"session_id":    usage.SessionID,
		"resource_type": "cpu",
		"quantity":      cpuHours,
		"unit":          "core-hours",
		"unit_price":    cpuRate,
		"total_cost":    cpuHours * cpuRate,
		"start_time":    usage.StartTime,
		"end_time":      time.Now(),
	}
	if err := ctx.Database.Insert("billing_usage_records", cpuRecord); err != nil {
		return err
	}

	// Record memory usage
	memoryRecord := map[string]interface{}{
		"user_id":       usage.UserID,
		"session_id":    usage.SessionID,
		"resource_type": "memory",
		"quantity":      memoryGBHours,
		"unit":          "gb-hours",
		"unit_price":    memoryRate,
		"total_cost":    memoryGBHours * memoryRate,
		"start_time":    usage.StartTime,
		"end_time":      time.Now(),
	}
	return ctx.Database.Insert("billing_usage_records", memoryRecord)
}

// getCurrentUsage gets current usage for a user
func (p *BillingPlugin) getCurrentUsage(ctx *plugins.PluginContext, userID string) (map[string]interface{}, error) {
	// Query database for current month usage
	startOfMonth := time.Now().AddDate(0, 0, -time.Now().Day()+1)

	rows, err := ctx.Database.Query(`
		SELECT resource_type, SUM(quantity) as total_quantity, SUM(total_cost) as total_cost
		FROM billing_usage_records
		WHERE user_id = $1 AND created_at >= $2
		GROUP BY resource_type
	`, userID, startOfMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	usage := make(map[string]interface{})
	totalCost := 0.0

	for rows.Next() {
		var resourceType string
		var quantity, cost float64
		if err := rows.Scan(&resourceType, &quantity, &cost); err != nil {
			continue
		}

		usage[resourceType] = map[string]interface{}{
			"quantity": quantity,
			"cost":     cost,
		}
		totalCost += cost
	}

	usage["total_cost"] = totalCost
	usage["period_start"] = startOfMonth
	usage["period_end"] = time.Now()

	return usage, nil
}

// getUserInvoices gets invoices for a user
func (p *BillingPlugin) getUserInvoices(ctx *plugins.PluginContext, userID string) ([]Invoice, error) {
	rows, err := ctx.Database.Query(`
		SELECT id, invoice_number, period_start, period_end, subtotal, credits, total, status, due_date, paid_at, created_at
		FROM billing_invoices
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 12
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invoices []Invoice
	for rows.Next() {
		var inv Invoice
		var paidAt *time.Time
		if err := rows.Scan(&inv.ID, &inv.InvoiceNumber, &inv.PeriodStart, &inv.PeriodEnd,
			&inv.Subtotal, &inv.Credits, &inv.Total, &inv.Status, &inv.DueDate, &paidAt, &inv.CreatedAt); err != nil {
			continue
		}
		inv.UserID = userID
		inv.PaidAt = paidAt
		invoices = append(invoices, inv)
	}

	return invoices, nil
}

// getUserSubscription gets active subscription for a user
func (p *BillingPlugin) getUserSubscription(ctx *plugins.PluginContext, userID string) (*Subscription, error) {
	row := ctx.Database.QueryRow(`
		SELECT id, plan_id, status, current_period_start, current_period_end, stripe_subscription_id, created_at, canceled_at
		FROM billing_subscriptions
		WHERE user_id = $1 AND status = 'active'
		LIMIT 1
	`, userID)

	var sub Subscription
	var stripeSubID *string
	var canceledAt *time.Time

	err := row.Scan(&sub.ID, &sub.PlanID, &sub.Status, &sub.CurrentPeriodStart,
		&sub.CurrentPeriodEnd, &stripeSubID, &sub.CreatedAt, &canceledAt)
	if err != nil {
		return nil, err
	}

	sub.UserID = userID
	if stripeSubID != nil {
		sub.StripeSubID = *stripeSubID
	}
	sub.CanceledAt = canceledAt

	return &sub, nil
}

// createSubscription creates a new subscription for a user
func (p *BillingPlugin) createSubscription(ctx *plugins.PluginContext, userID, planID string) error {
	now := time.Now()
	periodEnd := now.AddDate(0, 1, 0) // 1 month from now

	return ctx.Database.Insert("billing_subscriptions", map[string]interface{}{
		"user_id":               userID,
		"plan_id":               planID,
		"status":                "active",
		"current_period_start":  now,
		"current_period_end":    periodEnd,
	})
}

// createStripeCheckout creates a Stripe checkout session
func (p *BillingPlugin) createStripeCheckout(ctx *plugins.PluginContext, userID, planID string) (string, error) {
	stripeEnabled := p.getBool(ctx.Config, "stripeEnabled")
	if !stripeEnabled {
		return "", fmt.Errorf("stripe integration not enabled")
	}

	// In real implementation, this would call Stripe API
	// For now, return a placeholder
	return "https://checkout.stripe.com/placeholder", nil
}

// Helper functions

func (p *BillingPlugin) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (p *BillingPlugin) getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func (p *BillingPlugin) getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
		if i, ok := val.(int); ok {
			return float64(i)
		}
	}
	return 0
}

func (p *BillingPlugin) getMap(m map[string]interface{}, key string) map[string]interface{} {
	if val, ok := m[key]; ok {
		if subMap, ok := val.(map[string]interface{}); ok {
			return subMap
		}
	}
	return make(map[string]interface{})
}

func (p *BillingPlugin) parseCPU(cpu string) float64 {
	// Parse CPU strings like "1000m" (1 core), "500m" (0.5 cores), "2" (2 cores)
	// Simplified implementation
	return 1.0
}

func (p *BillingPlugin) parseMemory(memory string) float64 {
	// Parse memory strings like "2Gi" (2 GB), "512Mi" (0.5 GB)
	// Simplified implementation
	return 2.0
}

// init auto-registers the plugin globally
func init() {
	plugins.Register("streamspace-billing", func() plugins.PluginHandler {
		return NewBillingPlugin()
	})
}

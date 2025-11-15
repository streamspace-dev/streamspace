package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/streamspace/streamspace/api/internal/db"
)

// BillingHandler handles cost management and billing
type BillingHandler struct {
	db *db.Database
}

// NewBillingHandler creates a new billing handler
func NewBillingHandler(database *db.Database) *BillingHandler {
	return &BillingHandler{
		db: database,
	}
}

// RegisterRoutes registers billing routes
func (h *BillingHandler) RegisterRoutes(router *gin.RouterGroup) {
	billing := router.Group("/billing")
	{
		// Cost tracking
		billing.GET("/costs/current", h.GetCurrentCosts)
		billing.GET("/costs/history", h.GetCostHistory)
		billing.GET("/costs/breakdown", h.GetCostBreakdown)
		billing.GET("/costs/forecast", h.GetCostForecast)
		billing.GET("/costs/comparison", h.GetCostComparison)

		// Invoices
		billing.GET("/invoices", h.ListInvoices)
		billing.POST("/invoices/generate", h.GenerateInvoice)
		billing.GET("/invoices/:id", h.GetInvoice)
		billing.POST("/invoices/:id/pay", h.PayInvoice)
		billing.GET("/invoices/:id/download", h.DownloadInvoice)

		// Usage reports
		billing.GET("/usage/sessions", h.GetSessionUsage)
		billing.GET("/usage/resources", h.GetResourceUsage)
		billing.GET("/usage/storage", h.GetStorageUsage)
		billing.GET("/usage/export", h.ExportUsage)

		// Pricing
		billing.GET("/pricing", h.GetPricing)
		billing.PUT("/pricing", h.UpdatePricing)

		// Payment methods
		billing.GET("/payment-methods", h.ListPaymentMethods)
		billing.POST("/payment-methods", h.AddPaymentMethod)
		billing.DELETE("/payment-methods/:id", h.RemovePaymentMethod)
		billing.PUT("/payment-methods/:id/default", h.SetDefaultPaymentMethod)

		// Billing settings
		billing.GET("/settings", h.GetBillingSettings)
		billing.PUT("/settings", h.UpdateBillingSettings)
	}
}

// GetCurrentCosts returns current month costs
func (h *BillingHandler) GetCurrentCosts(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Get costs for current month
	var sessionCost, storageCost, totalCost float64

	// Session costs (based on runtime and resources)
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
			((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
		), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&sessionCost)

	// Storage costs
	var totalStorage int64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM session_snapshots
		WHERE user_id = $1
		AND status = 'completed'
	`, userIDStr).Scan(&totalStorage)

	storageCost = float64(totalStorage) / (1024 * 1024 * 1024) * 0.10 // $0.10 per GB per month

	totalCost = sessionCost + storageCost

	c.JSON(http.StatusOK, gin.H{
		"userId": userIDStr,
		"period": gin.H{
			"start": time.Now().AddDate(0, 0, -time.Now().Day()+1).Format("2006-01-02"),
			"end":   time.Now().Format("2006-01-02"),
		},
		"costs": gin.H{
			"sessions": sessionCost,
			"storage":  storageCost,
			"total":    totalCost,
		},
		"currency": "USD",
	})
}

// GetCostHistory returns historical cost data
func (h *BillingHandler) GetCostHistory(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)
	months := c.DefaultQuery("months", "6")

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		WITH RECURSIVE months AS (
			SELECT DATE_TRUNC('month', NOW()) - INTERVAL '1 month' * generate_series(0, $1::int - 1) AS month
		)
		SELECT
			months.month,
			COALESCE(SUM(
				EXTRACT(EPOCH FROM (COALESCE(s.terminated_at, NOW()) - s.created_at)) / 3600 *
				((s.resources->>'cpu')::float * 0.01 + (s.resources->>'memory')::float * 0.005)
			), 0) as cost
		FROM months
		LEFT JOIN sessions s ON s.user_id = $2
			AND s.created_at >= months.month
			AND s.created_at < months.month + INTERVAL '1 month'
		GROUP BY months.month
		ORDER BY months.month DESC
	`, months, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cost history"})
		return
	}
	defer rows.Close()

	history := []map[string]interface{}{}
	for rows.Next() {
		var month time.Time
		var cost float64
		rows.Scan(&month, &cost)

		history = append(history, map[string]interface{}{
			"month": month.Format("2006-01"),
			"cost":  cost,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"userId":  userIDStr,
		"history": history,
	})
}

// GetCostBreakdown returns detailed cost breakdown
func (h *BillingHandler) GetCostBreakdown(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Cost by template
	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT
			template_name,
			COUNT(*) as session_count,
			SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at))) / 3600 as total_hours,
			SUM(
				EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
				((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
			) as cost
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
		GROUP BY template_name
		ORDER BY cost DESC
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cost breakdown"})
		return
	}
	defer rows.Close()

	byTemplate := []map[string]interface{}{}
	for rows.Next() {
		var templateName string
		var sessionCount int
		var totalHours, cost float64
		rows.Scan(&templateName, &sessionCount, &totalHours, &cost)

		byTemplate = append(byTemplate, map[string]interface{}{
			"template":     templateName,
			"sessionCount": sessionCount,
			"totalHours":   totalHours,
			"cost":         cost,
		})
	}

	// Cost by resource type
	var cpuCost, memoryCost float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 * (resources->>'cpu')::float * 0.01),
			SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 * (resources->>'memory')::float * 0.005)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&cpuCost, &memoryCost)

	c.JSON(http.StatusOK, gin.H{
		"userId": userIDStr,
		"breakdown": gin.H{
			"byTemplate": byTemplate,
			"byResource": gin.H{
				"cpu":    cpuCost,
				"memory": memoryCost,
			},
		},
	})
}

// GetCostForecast returns projected costs for next month
func (h *BillingHandler) GetCostForecast(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Calculate average daily cost for current month
	var avgDailyCost float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(
				EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
				((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
			), 0) / EXTRACT(DAY FROM NOW())
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&avgDailyCost)

	// Project for next month (30 days)
	forecastedCost := avgDailyCost * 30

	c.JSON(http.StatusOK, gin.H{
		"userId": userIDStr,
		"forecast": gin.H{
			"avgDailyCost":   avgDailyCost,
			"forecastedCost": forecastedCost,
			"period":         "next_month",
			"confidence":     "medium",
		},
	})
}

// GetCostComparison returns cost comparison between periods
func (h *BillingHandler) GetCostComparison(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Current month
	var currentMonthCost float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
			((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
		), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&currentMonthCost)

	// Previous month
	var previousMonthCost float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
			((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
		), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW()) - INTERVAL '1 month'
		AND created_at < DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&previousMonthCost)

	change := currentMonthCost - previousMonthCost
	changePercent := 0.0
	if previousMonthCost > 0 {
		changePercent = (change / previousMonthCost) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"userId": userIDStr,
		"comparison": gin.H{
			"currentMonth":   currentMonthCost,
			"previousMonth":  previousMonthCost,
			"change":         change,
			"changePercent":  changePercent,
		},
	})
}

// ListInvoices returns all invoices for a user
func (h *BillingHandler) ListInvoices(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, invoice_number, period_start, period_end, amount, status, created_at, paid_at
		FROM invoices
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invoices"})
		return
	}
	defer rows.Close()

	invoices := []map[string]interface{}{}
	for rows.Next() {
		var id, invoiceNumber, status string
		var periodStart, periodEnd, createdAt time.Time
		var paidAt sql.NullTime
		var amount float64

		rows.Scan(&id, &invoiceNumber, &periodStart, &periodEnd, &amount, &status, &createdAt, &paidAt)

		invoice := map[string]interface{}{
			"id":            id,
			"invoiceNumber": invoiceNumber,
			"periodStart":   periodStart,
			"periodEnd":     periodEnd,
			"amount":        amount,
			"status":        status,
			"createdAt":     createdAt,
		}

		if paidAt.Valid {
			invoice["paidAt"] = paidAt.Time
		}

		invoices = append(invoices, invoice)
	}

	c.JSON(http.StatusOK, gin.H{
		"invoices": invoices,
		"total":    len(invoices),
	})
}

// GenerateInvoice generates a new invoice for the current period
func (h *BillingHandler) GenerateInvoice(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Calculate costs for current month
	var totalCost float64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 *
			((resources->>'cpu')::float * 0.01 + (resources->>'memory')::float * 0.005)
		), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&totalCost)

	// Create invoice
	id := fmt.Sprintf("inv_%d", time.Now().UnixNano())
	invoiceNumber := fmt.Sprintf("INV-%s-%s", userIDStr[:8], time.Now().Format("200601"))
	periodStart := time.Now().AddDate(0, 0, -time.Now().Day()+1)
	periodEnd := time.Now()

	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO invoices (id, user_id, invoice_number, period_start, period_end, amount, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
	`, id, userIDStr, invoiceNumber, periodStart, periodEnd, totalCost)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate invoice"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Invoice generated successfully",
		"id":            id,
		"invoiceNumber": invoiceNumber,
		"amount":        totalCost,
	})
}

// GetInvoice returns a specific invoice
func (h *BillingHandler) GetInvoice(c *gin.Context) {
	invoiceID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var id, invoiceNumber, targetUserID, status string
	var periodStart, periodEnd, createdAt time.Time
	var paidAt sql.NullTime
	var amount float64

	err := h.db.DB().QueryRowContext(ctx, `
		SELECT id, user_id, invoice_number, period_start, period_end, amount, status, created_at, paid_at
		FROM invoices
		WHERE id = $1
	`, invoiceID).Scan(&id, &targetUserID, &invoiceNumber, &periodStart, &periodEnd, &amount, &status, &createdAt, &paidAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invoice"})
		return
	}

	// Verify ownership
	if targetUserID != userIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized to view this invoice"})
		return
	}

	invoice := gin.H{
		"id":            id,
		"userId":        targetUserID,
		"invoiceNumber": invoiceNumber,
		"periodStart":   periodStart,
		"periodEnd":     periodEnd,
		"amount":        amount,
		"status":        status,
		"createdAt":     createdAt,
	}

	if paidAt.Valid {
		invoice["paidAt"] = paidAt.Time
	}

	c.JSON(http.StatusOK, invoice)
}

// PayInvoice marks an invoice as paid
func (h *BillingHandler) PayInvoice(c *gin.Context) {
	invoiceID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		PaymentMethodID string `json:"paymentMethodId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()

	// Verify invoice ownership
	var targetUserID string
	err := h.db.DB().QueryRowContext(ctx, `
		SELECT user_id FROM invoices WHERE id = $1
	`, invoiceID).Scan(&targetUserID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invoice not found"})
		return
	}

	if targetUserID != userIDStr {
		c.JSON(http.StatusForbidden, gin.H{"error": "Not authorized"})
		return
	}

	// Mark as paid (in real implementation, would integrate with payment gateway)
	_, err = h.db.DB().ExecContext(ctx, `
		UPDATE invoices
		SET status = 'paid', paid_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, invoiceID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pay invoice"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Invoice paid successfully",
		"id":      invoiceID,
	})
}

// DownloadInvoice generates a downloadable invoice PDF
func (h *BillingHandler) DownloadInvoice(c *gin.Context) {
	invoiceID := c.Param("id")

	// TODO: Implement PDF generation
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "PDF generation not yet implemented",
		"id":      invoiceID,
	})
}

// GetSessionUsage returns session usage statistics
func (h *BillingHandler) GetSessionUsage(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var totalSessions int
	var totalHours float64

	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&totalSessions, &totalHours)

	c.JSON(http.StatusOK, gin.H{
		"userId":        userIDStr,
		"totalSessions": totalSessions,
		"totalHours":    totalHours,
	})
}

// GetResourceUsage returns resource usage statistics
func (h *BillingHandler) GetResourceUsage(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var totalCPUHours, totalMemoryHours float64

	h.db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 * (resources->>'cpu')::float), 0),
			COALESCE(SUM(EXTRACT(EPOCH FROM (COALESCE(terminated_at, NOW()) - created_at)) / 3600 * (resources->>'memory')::float), 0)
		FROM sessions
		WHERE user_id = $1
		AND created_at >= DATE_TRUNC('month', NOW())
	`, userIDStr).Scan(&totalCPUHours, &totalMemoryHours)

	c.JSON(http.StatusOK, gin.H{
		"userId":          userIDStr,
		"totalCPUHours":   totalCPUHours,
		"totalMemoryHours": totalMemoryHours,
	})
}

// GetStorageUsage returns storage usage statistics
func (h *BillingHandler) GetStorageUsage(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	var totalStorage int64
	h.db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(SUM(size_bytes), 0)
		FROM session_snapshots
		WHERE user_id = $1
		AND status = 'completed'
	`, userIDStr).Scan(&totalStorage)

	c.JSON(http.StatusOK, gin.H{
		"userId":           userIDStr,
		"totalStorageBytes": totalStorage,
		"totalStorageGB":    float64(totalStorage) / (1024 * 1024 * 1024),
	})
}

// ExportUsage exports usage data in CSV format
func (h *BillingHandler) ExportUsage(c *gin.Context) {
	// TODO: Implement CSV export
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "CSV export not yet implemented",
	})
}

// GetPricing returns current pricing configuration
func (h *BillingHandler) GetPricing(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"cpu": gin.H{
			"rate": 0.01,
			"unit": "per core per hour",
		},
		"memory": gin.H{
			"rate": 0.005,
			"unit": "per GB per hour",
		},
		"storage": gin.H{
			"rate": 0.10,
			"unit": "per GB per month",
		},
		"currency": "USD",
	})
}

// UpdatePricing updates pricing configuration (admin only)
func (h *BillingHandler) UpdatePricing(c *gin.Context) {
	var req struct {
		CPURate     float64 `json:"cpuRate"`
		MemoryRate  float64 `json:"memoryRate"`
		StorageRate float64 `json:"storageRate"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Store pricing in database
	c.JSON(http.StatusOK, gin.H{
		"message": "Pricing updated successfully",
	})
}

// ListPaymentMethods returns all payment methods for a user
func (h *BillingHandler) ListPaymentMethods(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	rows, err := h.db.DB().QueryContext(ctx, `
		SELECT id, type, last4, is_default, created_at
		FROM payment_methods
		WHERE user_id = $1
		ORDER BY is_default DESC, created_at DESC
	`, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get payment methods"})
		return
	}
	defer rows.Close()

	methods := []map[string]interface{}{}
	for rows.Next() {
		var id, methodType, last4 string
		var isDefault bool
		var createdAt time.Time

		rows.Scan(&id, &methodType, &last4, &isDefault, &createdAt)

		methods = append(methods, map[string]interface{}{
			"id":        id,
			"type":      methodType,
			"last4":     last4,
			"isDefault": isDefault,
			"createdAt": createdAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"paymentMethods": methods,
		"total":          len(methods),
	})
}

// AddPaymentMethod adds a new payment method
func (h *BillingHandler) AddPaymentMethod(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	var req struct {
		Type       string `json:"type" binding:"required"`
		CardNumber string `json:"cardNumber" binding:"required"`
		ExpiryMM   string `json:"expiryMM" binding:"required"`
		ExpiryYY   string `json:"expiryYY" binding:"required"`
		CVV        string `json:"cvv" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	id := fmt.Sprintf("pm_%d", time.Now().UnixNano())
	last4 := req.CardNumber[len(req.CardNumber)-4:]

	// In real implementation, would tokenize with payment gateway
	_, err := h.db.DB().ExecContext(ctx, `
		INSERT INTO payment_methods (id, user_id, type, last4, is_default)
		VALUES ($1, $2, $3, $4, false)
	`, id, userIDStr, req.Type, last4)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add payment method"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Payment method added successfully",
		"id":      id,
	})
}

// RemovePaymentMethod removes a payment method
func (h *BillingHandler) RemovePaymentMethod(c *gin.Context) {
	methodID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	_, err := h.db.DB().ExecContext(ctx, `
		DELETE FROM payment_methods
		WHERE id = $1 AND user_id = $2
	`, methodID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove payment method"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Payment method removed successfully",
	})
}

// SetDefaultPaymentMethod sets a payment method as default
func (h *BillingHandler) SetDefaultPaymentMethod(c *gin.Context) {
	methodID := c.Param("id")
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	ctx := context.Background()

	// Unset all defaults
	h.db.DB().ExecContext(ctx, `
		UPDATE payment_methods SET is_default = false WHERE user_id = $1
	`, userIDStr)

	// Set new default
	_, err := h.db.DB().ExecContext(ctx, `
		UPDATE payment_methods SET is_default = true
		WHERE id = $1 AND user_id = $2
	`, methodID, userIDStr)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default payment method"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Default payment method updated",
	})
}

// GetBillingSettings returns billing settings for a user
func (h *BillingHandler) GetBillingSettings(c *gin.Context) {
	userID, _ := c.Get("userID")
	userIDStr := userID.(string)

	c.JSON(http.StatusOK, gin.H{
		"userId":       userIDStr,
		"autoPayEnabled": false,
		"billingEmail":   "",
		"taxId":          "",
		"currency":       "USD",
	})
}

// UpdateBillingSettings updates billing settings
func (h *BillingHandler) UpdateBillingSettings(c *gin.Context) {
	var req struct {
		AutoPayEnabled bool   `json:"autoPayEnabled"`
		BillingEmail   string `json:"billingEmail"`
		TaxID          string `json:"taxId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Store settings in database
	c.JSON(http.StatusOK, gin.H{
		"message": "Billing settings updated successfully",
	})
}

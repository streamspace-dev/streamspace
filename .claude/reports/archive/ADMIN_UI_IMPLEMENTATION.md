# Admin UI Implementation Guide

**Last Updated:** 2025-11-20
**Target Audience:** Frontend/Backend Developers (Builder - Agent 2)
**Goal:** Implement critical admin UI features for v1.0.0 stable release

---

## Table of Contents

- [Overview](#overview)
- [Implementation Priority](#implementation-priority)
- [Technical Stack](#technical-stack)
- [P0 Critical Features](#p0-critical-features)
  - [1. Audit Logs Viewer](#1-audit-logs-viewer)
  - [2. System Configuration](#2-system-configuration)
  - [3. License Management](#3-license-management)
- [P1 High Priority Features](#p1-high-priority-features)
  - [4. API Keys Management](#4-api-keys-management)
  - [5. Alert Management](#5-alert-management)
  - [6. Controller Management](#6-controller-management)
  - [7. Session Recordings Viewer](#7-session-recordings-viewer)
- [Common Patterns](#common-patterns)
- [Testing Requirements](#testing-requirements)
- [Deployment Checklist](#deployment-checklist)

---

## Overview

Based on the [Admin UI Gap Analysis](./ADMIN_UI_GAP_ANALYSIS.md), StreamSpace has a comprehensive backend but is missing critical admin UI features. This guide provides detailed implementation specifications for each feature.

### Current Status

**What Exists:**
- ✅ 12 admin pages (~229KB total)
- ✅ Comprehensive backend (87 database tables, 37 handler files)
- ✅ React/TypeScript/MUI infrastructure

**What's Missing:**
- ❌ 3 P0 (CRITICAL) admin features - Block production deployment
- ❌ 4 P1 (HIGH) admin features - Block essential operations
- ❌ 5 P2 (MEDIUM) admin features - Reduce admin efficiency

### Implementation Timeline

**Phase 1 (Weeks 1-2):** P0 Critical Features
- Audit Logs Viewer (2-3 days)
- System Configuration (3-4 days)
- License Management (3-4 days)

**Phase 2 (Weeks 3-4):** P1 High Priority
- API Keys Management (2 days)
- Alert Management (2-3 days)
- Controller Management (3-4 days)
- Session Recordings Viewer (4-5 days)

**Total Effort:** 19-25 development days for P0 + P1

---

## Implementation Priority

### Why P0 Features Are Critical

1. **Audit Logs:** SOC2/HIPAA/GDPR compliance REQUIRES audit trail
2. **System Configuration:** Cannot deploy to production without config UI
3. **License Management:** Cannot sell Pro/Enterprise without license enforcement

**Without P0 features, StreamSpace cannot:**
- Pass security/compliance audits
- Be deployed to production (config via DB is unacceptable)
- Generate revenue (no license tiers)

---

## Technical Stack

### Frontend
- **Framework:** React 18+ with TypeScript
- **UI Library:** Material-UI (MUI) v5
- **State Management:** React Context API + Hooks
- **HTTP Client:** Axios with JWT interceptors
- **Forms:** React Hook Form + Yup validation
- **Date/Time:** date-fns
- **Code Editor:** Monaco Editor (for JSON viewers)

### Backend
- **Framework:** Go with Gin
- **Database:** PostgreSQL
- **ORM:** Direct SQL queries (existing pattern)
- **Validation:** go-playground/validator
- **Auth:** JWT middleware (existing)

### File Organization

```
ui/src/
├── pages/
│   └── admin/
│       ├── AuditLogs.tsx         # NEW - P0
│       ├── Settings.tsx          # NEW - P0
│       ├── License.tsx           # NEW - P0
│       ├── APIKeys.tsx           # NEW - P1
│       ├── Monitoring.tsx        # NEW - P1 (alerts)
│       ├── Controllers.tsx       # NEW - P1
│       └── Recordings.tsx        # NEW - P1
├── components/
│   ├── AuditLogTable.tsx         # NEW
│   ├── ConfigurationForm.tsx     # NEW
│   ├── LicenseCard.tsx          # NEW
│   └── (existing components)
└── lib/
    ├── api.ts                    # UPDATE with new endpoints
    └── types.ts                  # UPDATE with new types

api/internal/
└── handlers/
    ├── audit.go                  # NEW - P0
    ├── configuration.go          # NEW - P0
    ├── license.go               # NEW - P0
    └── (existing handlers)
```

---

## P0 Critical Features

## 1. Audit Logs Viewer

**Priority:** P0 - CRITICAL
**Effort:** 2-3 days
**Reason:** Required for SOC2/HIPAA/GDPR compliance

### Backend Implementation

#### Database Schema (Already Exists)

```sql
-- Table: audit_log (already exists in database.go)
CREATE TABLE IF NOT EXISTS audit_log (
  id SERIAL PRIMARY KEY,
  user_id INT REFERENCES users(id),
  action VARCHAR(100) NOT NULL,
  resource_type VARCHAR(50) NOT NULL,
  resource_id VARCHAR(255),
  changes JSONB,
  timestamp TIMESTAMP DEFAULT NOW(),
  ip_address INET,
  user_agent TEXT,
  status VARCHAR(20) DEFAULT 'success' -- success, failed
);

CREATE INDEX idx_audit_timestamp ON audit_log(timestamp DESC);
CREATE INDEX idx_audit_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_action ON audit_log(action);
CREATE INDEX idx_audit_resource ON audit_log(resource_type, resource_id);
```

#### API Handler: `api/internal/handlers/audit.go`

```go
package handlers

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
)

type AuditHandler struct {
    db *sql.DB
}

func NewAuditHandler(db *sql.DB) *AuditHandler {
    return &AuditHandler{db: db}
}

// GET /api/v1/admin/audit
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
    // Parse query parameters
    userID := c.Query("user_id")
    action := c.Query("action")
    resourceType := c.Query("resource_type")
    startDate := c.Query("start_date")
    endDate := c.Query("end_date")
    limit := c.DefaultQuery("limit", "100")
    offset := c.DefaultQuery("offset", "0")

    // Build dynamic query
    query := `
        SELECT
            a.id, a.user_id, u.username, a.action,
            a.resource_type, a.resource_id, a.changes,
            a.timestamp, a.ip_address, a.user_agent, a.status
        FROM audit_log a
        LEFT JOIN users u ON a.user_id = u.id
        WHERE 1=1
    `
    args := []interface{}{}
    argCount := 1

    if userID != "" {
        query += fmt.Sprintf(" AND a.user_id = $%d", argCount)
        args = append(args, userID)
        argCount++
    }
    if action != "" {
        query += fmt.Sprintf(" AND a.action = $%d", argCount)
        args = append(args, action)
        argCount++
    }
    if resourceType != "" {
        query += fmt.Sprintf(" AND a.resource_type = $%d", argCount)
        args = append(args, resourceType)
        argCount++
    }
    if startDate != "" {
        query += fmt.Sprintf(" AND a.timestamp >= $%d", argCount)
        args = append(args, startDate)
        argCount++
    }
    if endDate != "" {
        query += fmt.Sprintf(" AND a.timestamp <= $%d", argCount)
        args = append(args, endDate)
        argCount++
    }

    query += " ORDER BY a.timestamp DESC"
    query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
    args = append(args, limit, offset)

    // Execute query
    rows, err := h.db.QueryContext(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit logs"})
        return
    }
    defer rows.Close()

    logs := []AuditLog{}
    for rows.Next() {
        var log AuditLog
        var changes []byte
        err := rows.Scan(
            &log.ID, &log.UserID, &log.Username, &log.Action,
            &log.ResourceType, &log.ResourceID, &changes,
            &log.Timestamp, &log.IPAddress, &log.UserAgent, &log.Status,
        )
        if err != nil {
            continue
        }
        json.Unmarshal(changes, &log.Changes)
        logs = append(logs, log)
    }

    // Get total count for pagination
    countQuery := `SELECT COUNT(*) FROM audit_log WHERE 1=1`
    // Add same filters as above...
    var total int
    h.db.QueryRowContext(c.Request.Context(), countQuery, args[:len(args)-2]...).Scan(&total)

    c.JSON(http.StatusOK, gin.H{
        "logs":  logs,
        "total": total,
        "limit": limit,
        "offset": offset,
    })
}

// GET /api/v1/admin/audit/:id
func (h *AuditHandler) GetAuditLog(c *gin.Context) {
    id := c.Param("id")

    var log AuditLog
    var changes []byte
    err := h.db.QueryRowContext(c.Request.Context(), `
        SELECT
            a.id, a.user_id, u.username, a.action,
            a.resource_type, a.resource_id, a.changes,
            a.timestamp, a.ip_address, a.user_agent, a.status
        FROM audit_log a
        LEFT JOIN users u ON a.user_id = u.id
        WHERE a.id = $1
    `, id).Scan(
        &log.ID, &log.UserID, &log.Username, &log.Action,
        &log.ResourceType, &log.ResourceID, &changes,
        &log.Timestamp, &log.IPAddress, &log.UserAgent, &log.Status,
    )

    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{"error": "Audit log not found"})
        return
    }
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch audit log"})
        return
    }

    json.Unmarshal(changes, &log.Changes)
    c.JSON(http.StatusOK, log)
}

// GET /api/v1/admin/audit/export
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
    format := c.DefaultQuery("format", "csv") // csv or json

    // Similar query as GetAuditLogs but without pagination
    rows, err := h.db.QueryContext(c.Request.Context(), `
        SELECT
            a.id, a.user_id, u.username, a.action,
            a.resource_type, a.resource_id, a.changes,
            a.timestamp, a.ip_address, a.status
        FROM audit_log a
        LEFT JOIN users u ON a.user_id = u.id
        ORDER BY a.timestamp DESC
    `)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export logs"})
        return
    }
    defer rows.Close()

    if format == "csv" {
        c.Header("Content-Type", "text/csv")
        c.Header("Content-Disposition", `attachment; filename="audit_logs.csv"`)

        writer := csv.NewWriter(c.Writer)
        writer.Write([]string{"ID", "User", "Action", "Resource Type", "Resource ID", "Timestamp", "IP Address", "Status"})

        for rows.Next() {
            var log AuditLog
            var changes []byte
            rows.Scan(&log.ID, &log.UserID, &log.Username, &log.Action,
                &log.ResourceType, &log.ResourceID, &changes, &log.Timestamp, &log.IPAddress, &log.Status)

            writer.Write([]string{
                fmt.Sprintf("%d", log.ID),
                log.Username,
                log.Action,
                log.ResourceType,
                log.ResourceID,
                log.Timestamp.Format(time.RFC3339),
                log.IPAddress,
                log.Status,
            })
        }
        writer.Flush()
    } else {
        // JSON export
        logs := []AuditLog{}
        for rows.Next() {
            var log AuditLog
            var changes []byte
            rows.Scan(&log.ID, &log.UserID, &log.Username, &log.Action,
                &log.ResourceType, &log.ResourceID, &changes, &log.Timestamp, &log.IPAddress, &log.Status)
            json.Unmarshal(changes, &log.Changes)
            logs = append(logs, log)
        }

        c.Header("Content-Type", "application/json")
        c.Header("Content-Disposition", `attachment; filename="audit_logs.json"`)
        c.JSON(http.StatusOK, logs)
    }
}

type AuditLog struct {
    ID           int                    `json:"id"`
    UserID       int                    `json:"user_id"`
    Username     string                 `json:"username"`
    Action       string                 `json:"action"`
    ResourceType string                 `json:"resource_type"`
    ResourceID   string                 `json:"resource_id"`
    Changes      map[string]interface{} `json:"changes"`
    Timestamp    time.Time              `json:"timestamp"`
    IPAddress    string                 `json:"ip_address"`
    UserAgent    string                 `json:"user_agent"`
    Status       string                 `json:"status"`
}
```

#### Register Routes: `api/cmd/main.go`

```go
// In setupRoutes()
auditHandler := handlers.NewAuditHandler(db)

admin := api.Group("/api/v1/admin")
admin.Use(middleware.AuthMiddleware(), middleware.AdminOnly())
{
    // Existing routes...

    // Audit logs
    admin.GET("/audit", auditHandler.GetAuditLogs)
    admin.GET("/audit/:id", auditHandler.GetAuditLog)
    admin.GET("/audit/export", auditHandler.ExportAuditLogs)
}
```

### Frontend Implementation

#### Types: `ui/src/lib/types.ts`

```typescript
export interface AuditLog {
  id: number
  user_id: number
  username: string
  action: string
  resource_type: string
  resource_id: string
  changes: Record<string, any>
  timestamp: string
  ip_address: string
  user_agent: string
  status: 'success' | 'failed'
}

export interface AuditLogsResponse {
  logs: AuditLog[]
  total: number
  limit: number
  offset: number
}

export interface AuditLogFilters {
  user_id?: string
  action?: string
  resource_type?: string
  start_date?: string
  end_date?: string
  limit?: number
  offset?: number
}
```

#### API Client: `ui/src/lib/api.ts`

```typescript
export async function getAuditLogs(filters: AuditLogFilters): Promise<AuditLogsResponse> {
  const params = new URLSearchParams()
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      params.append(key, value.toString())
    }
  })

  const response = await axios.get(`/api/v1/admin/audit?${params.toString()}`)
  return response.data
}

export async function getAuditLog(id: number): Promise<AuditLog> {
  const response = await axios.get(`/api/v1/admin/audit/${id}`)
  return response.data
}

export async function exportAuditLogs(format: 'csv' | 'json', filters: AuditLogFilters): Promise<Blob> {
  const params = new URLSearchParams()
  params.append('format', format)
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') {
      params.append(key, value.toString())
    }
  })

  const response = await axios.get(`/api/v1/admin/audit/export?${params.toString()}`, {
    responseType: 'blob'
  })
  return response.data
}
```

#### Component: `ui/src/pages/admin/AuditLogs.tsx`

```typescript
import React, { useState, useEffect } from 'react'
import {
  Box,
  Paper,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TablePagination,
  TextField,
  MenuItem,
  Button,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  Grid,
  IconButton,
} from '@mui/material'
import { Download, Visibility } from '@mui/icons-material'
import { format } from 'date-fns'
import { getAuditLogs, exportAuditLogs } from '../../lib/api'
import type { AuditLog, AuditLogFilters } from '../../lib/types'
import JSONDiffViewer from '../../components/JSONDiffViewer'

export default function AuditLogs() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(100)
  const [filters, setFilters] = useState<AuditLogFilters>({})
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    loadLogs()
  }, [page, rowsPerPage, filters])

  const loadLogs = async () => {
    setLoading(true)
    try {
      const data = await getAuditLogs({
        ...filters,
        limit: rowsPerPage,
        offset: page * rowsPerPage,
      })
      setLogs(data.logs)
      setTotal(data.total)
    } catch (error) {
      console.error('Failed to load audit logs:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleExport = async (format: 'csv' | 'json') => {
    const blob = await exportAuditLogs(format, filters)
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `audit_logs.${format}`
    a.click()
  }

  const getStatusColor = (status: string) => {
    return status === 'success' ? 'success' : 'error'
  }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>
        Audit Logs
      </Typography>

      {/* Filters */}
      <Paper sx={{ p: 2, mb: 2 }}>
        <Grid container spacing={2}>
          <Grid item xs={12} sm={6} md={3}>
            <TextField
              fullWidth
              label="User ID"
              value={filters.user_id || ''}
              onChange={(e) => setFilters({ ...filters, user_id: e.target.value })}
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <TextField
              fullWidth
              select
              label="Action"
              value={filters.action || ''}
              onChange={(e) => setFilters({ ...filters, action: e.target.value })}
            >
              <MenuItem value="">All</MenuItem>
              <MenuItem value="session.created">Session Created</MenuItem>
              <MenuItem value="session.deleted">Session Deleted</MenuItem>
              <MenuItem value="user.created">User Created</MenuItem>
              <MenuItem value="user.updated">User Updated</MenuItem>
              <MenuItem value="user.deleted">User Deleted</MenuItem>
            </TextField>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <TextField
              fullWidth
              label="Start Date"
              type="date"
              InputLabelProps={{ shrink: true }}
              value={filters.start_date || ''}
              onChange={(e) => setFilters({ ...filters, start_date: e.target.value })}
            />
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <TextField
              fullWidth
              label="End Date"
              type="date"
              InputLabelProps={{ shrink: true }}
              value={filters.end_date || ''}
              onChange={(e) => setFilters({ ...filters, end_date: e.target.value })}
            />
          </Grid>
        </Grid>

        <Box sx={{ mt: 2, display: 'flex', gap: 1 }}>
          <Button variant="outlined" onClick={() => setFilters({})}>
            Clear Filters
          </Button>
          <Button variant="outlined" startIcon={<Download />} onClick={() => handleExport('csv')}>
            Export CSV
          </Button>
          <Button variant="outlined" startIcon={<Download />} onClick={() => handleExport('json')}>
            Export JSON
          </Button>
        </Box>
      </Paper>

      {/* Table */}
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Timestamp</TableCell>
              <TableCell>User</TableCell>
              <TableCell>Action</TableCell>
              <TableCell>Resource</TableCell>
              <TableCell>IP Address</TableCell>
              <TableCell>Status</TableCell>
              <TableCell>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {logs.map((log) => (
              <TableRow key={log.id}>
                <TableCell>{format(new Date(log.timestamp), 'yyyy-MM-dd HH:mm:ss')}</TableCell>
                <TableCell>{log.username}</TableCell>
                <TableCell>{log.action}</TableCell>
                <TableCell>
                  {log.resource_type}
                  {log.resource_id && ` (${log.resource_id})`}
                </TableCell>
                <TableCell>{log.ip_address}</TableCell>
                <TableCell>
                  <Chip label={log.status} color={getStatusColor(log.status)} size="small" />
                </TableCell>
                <TableCell>
                  <IconButton onClick={() => setSelectedLog(log)} size="small">
                    <Visibility />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(e, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => setRowsPerPage(parseInt(e.target.value, 10))}
        />
      </TableContainer>

      {/* Detail Dialog */}
      <Dialog open={!!selectedLog} onClose={() => setSelectedLog(null)} maxWidth="md" fullWidth>
        <DialogTitle>Audit Log Details</DialogTitle>
        <DialogContent>
          {selectedLog && (
            <Box>
              <Grid container spacing={2}>
                <Grid item xs={6}>
                  <Typography variant="subtitle2">User</Typography>
                  <Typography>{selectedLog.username}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="subtitle2">Action</Typography>
                  <Typography>{selectedLog.action}</Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="subtitle2">Resource</Typography>
                  <Typography>
                    {selectedLog.resource_type} ({selectedLog.resource_id})
                  </Typography>
                </Grid>
                <Grid item xs={6}>
                  <Typography variant="subtitle2">IP Address</Typography>
                  <Typography>{selectedLog.ip_address}</Typography>
                </Grid>
                <Grid item xs={12}>
                  <Typography variant="subtitle2" gutterBottom>
                    Changes
                  </Typography>
                  <JSONDiffViewer changes={selectedLog.changes} />
                </Grid>
              </Grid>
            </Box>
          )}
        </DialogContent>
      </Dialog>
    </Box>
  )
}
```

### Testing

```typescript
// AuditLogs.test.tsx
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import AuditLogs from './AuditLogs'
import * as api from '../../lib/api'

vi.mock('../../lib/api')

describe('AuditLogs', () => {
  const mockLogs = {
    logs: [
      {
        id: 1,
        username: 'admin',
        action: 'session.created',
        resource_type: 'session',
        resource_id: 'test-session',
        timestamp: '2025-11-20T10:00:00Z',
        ip_address: '192.168.1.1',
        status: 'success',
        changes: {},
      },
    ],
    total: 1,
    limit: 100,
    offset: 0,
  }

  it('loads and displays audit logs', async () => {
    vi.mocked(api.getAuditLogs).mockResolvedValue(mockLogs)

    render(<AuditLogs />)

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument()
      expect(screen.getByText('session.created')).toBeInTheDocument()
    })
  })

  it('filters logs by action', async () => {
    vi.mocked(api.getAuditLogs).mockResolvedValue(mockLogs)

    render(<AuditLogs />)

    const actionSelect = screen.getByLabelText('Action')
    fireEvent.change(actionSelect, { target: { value: 'session.created' } })

    await waitFor(() => {
      expect(api.getAuditLogs).toHaveBeenCalledWith(
        expect.objectContaining({ action: 'session.created' })
      )
    })
  })

  it('exports logs as CSV', async () => {
    const mockBlob = new Blob(['csv data'], { type: 'text/csv' })
    vi.mocked(api.exportAuditLogs).mockResolvedValue(mockBlob)

    render(<AuditLogs />)

    const exportButton = screen.getByText('Export CSV')
    fireEvent.click(exportButton)

    await waitFor(() => {
      expect(api.exportAuditLogs).toHaveBeenCalledWith('csv', {})
    })
  })
})
```

---

## 2. System Configuration

**Priority:** P0 - CRITICAL
**Effort:** 3-4 days
**Reason:** Cannot deploy to production without config UI

### Backend Implementation

#### Database Schema (Already Exists)

```sql
CREATE TABLE IF NOT EXISTS configuration (
  id SERIAL PRIMARY KEY,
  key VARCHAR(255) UNIQUE NOT NULL,
  value TEXT NOT NULL,
  type VARCHAR(50) NOT NULL, -- string, boolean, number, duration, enum, array
  category VARCHAR(50) NOT NULL, -- ingress, storage, resources, features, session, security, compliance
  description TEXT,
  validation_regex VARCHAR(255),
  allowed_values TEXT[], -- For enum types
  updated_at TIMESTAMP DEFAULT NOW(),
  updated_by INT REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS configuration_history (
  id SERIAL PRIMARY KEY,
  config_id INT REFERENCES configuration(id),
  old_value TEXT,
  new_value TEXT,
  changed_by INT REFERENCES users(id),
  changed_at TIMESTAMP DEFAULT NOW()
);
```

#### API Handler: `api/internal/handlers/configuration.go`

```go
package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
)

type ConfigurationHandler struct {
    db *sql.DB
}

func NewConfigurationHandler(db *sql.DB) *ConfigurationHandler {
    return &ConfigurationHandler{db: db}
}

// GET /api/v1/admin/config
func (h *ConfigurationHandler) GetConfigurations(c *gin.Context) {
    category := c.Query("category")

    query := `
        SELECT id, key, value, type, category, description, validation_regex, allowed_values, updated_at
        FROM configuration
    `
    args := []interface{}{}
    if category != "" {
        query += " WHERE category = $1"
        args = append(args, category)
    }
    query += " ORDER BY category, key"

    rows, err := h.db.QueryContext(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch configurations"})
        return
    }
    defer rows.Close()

    configs := []Configuration{}
    for rows.Next() {
        var config Configuration
        var allowedValues string
        err := rows.Scan(
            &config.ID, &config.Key, &config.Value, &config.Type,
            &config.Category, &config.Description, &config.ValidationRegex,
            &allowedValues, &config.UpdatedAt,
        )
        if err != nil {
            continue
        }
        if allowedValues != "" {
            json.Unmarshal([]byte(allowedValues), &config.AllowedValues)
        }
        configs = append(configs, config)
    }

    // Group by category
    grouped := make(map[string][]Configuration)
    for _, config := range configs {
        grouped[config.Category] = append(grouped[config.Category], config)
    }

    c.JSON(http.StatusOK, gin.H{
        "configurations": configs,
        "grouped":        grouped,
    })
}

// PUT /api/v1/admin/config/:key
func (h *ConfigurationHandler) UpdateConfiguration(c *gin.Context) {
    key := c.Param("key")
    var req struct {
        Value string `json:"value" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    // Get current configuration
    var config Configuration
    var allowedValues string
    err := h.db.QueryRowContext(c.Request.Context(), `
        SELECT id, key, value, type, category, validation_regex, allowed_values
        FROM configuration
        WHERE key = $1
    `, key).Scan(
        &config.ID, &config.Key, &config.Value, &config.Type,
        &config.Category, &config.ValidationRegex, &allowedValues,
    )

    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
        return
    }
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch configuration"})
        return
    }

    if allowedValues != "" {
        json.Unmarshal([]byte(allowedValues), &config.AllowedValues)
    }

    // Validate new value
    if err := validateConfigValue(config, req.Value); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Get user ID from context (set by auth middleware)
    userID := c.GetInt("user_id")

    // Update configuration
    _, err = h.db.ExecContext(c.Request.Context(), `
        UPDATE configuration
        SET value = $1, updated_at = NOW(), updated_by = $2
        WHERE key = $3
    `, req.Value, userID, key)

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update configuration"})
        return
    }

    // Record in history
    h.db.ExecContext(c.Request.Context(), `
        INSERT INTO configuration_history (config_id, old_value, new_value, changed_by)
        VALUES ($1, $2, $3, $4)
    `, config.ID, config.Value, req.Value, userID)

    c.JSON(http.StatusOK, gin.H{
        "message": "Configuration updated successfully",
        "key":     key,
        "value":   req.Value,
    })
}

// POST /api/v1/admin/config/:key/test
func (h *ConfigurationHandler) TestConfiguration(c *gin.Context) {
    key := c.Param("key")
    var req struct {
        Value string `json:"value" binding:"required"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
        return
    }

    // Get configuration metadata
    var config Configuration
    var allowedValues string
    err := h.db.QueryRowContext(c.Request.Context(), `
        SELECT id, key, type, validation_regex, allowed_values
        FROM configuration
        WHERE key = $1
    `, key).Scan(&config.ID, &config.Key, &config.Type, &config.ValidationRegex, &allowedValues)

    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{"error": "Configuration not found"})
        return
    }

    if allowedValues != "" {
        json.Unmarshal([]byte(allowedValues), &config.AllowedValues)
    }

    // Validate without saving
    if err := validateConfigValue(config, req.Value); err != nil {
        c.JSON(http.StatusOK, gin.H{
            "valid":   false,
            "message": err.Error(),
        })
        return
    }

    // Test-specific validation (e.g., DNS resolution for domain names)
    testResult, testMessage := testConfigValue(key, req.Value)

    c.JSON(http.StatusOK, gin.H{
        "valid":   testResult,
        "message": testMessage,
    })
}

// GET /api/v1/admin/config/history
func (h *ConfigurationHandler) GetConfigurationHistory(c *gin.Context) {
    key := c.Query("key")

    query := `
        SELECT
            ch.id, c.key, ch.old_value, ch.new_value,
            u.username, ch.changed_at
        FROM configuration_history ch
        JOIN configuration c ON ch.config_id = c.id
        LEFT JOIN users u ON ch.changed_by = u.id
    `
    args := []interface{}{}
    if key != "" {
        query += " WHERE c.key = $1"
        args = append(args, key)
    }
    query += " ORDER BY ch.changed_at DESC LIMIT 100"

    rows, err := h.db.QueryContext(c.Request.Context(), query, args...)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
        return
    }
    defer rows.Close()

    history := []ConfigurationHistory{}
    for rows.Next() {
        var h ConfigurationHistory
        rows.Scan(&h.ID, &h.Key, &h.OldValue, &h.NewValue, &h.ChangedBy, &h.ChangedAt)
        history = append(history, h)
    }

    c.JSON(http.StatusOK, history)
}

type Configuration struct {
    ID               int      `json:"id"`
    Key              string   `json:"key"`
    Value            string   `json:"value"`
    Type             string   `json:"type"`
    Category         string   `json:"category"`
    Description      string   `json:"description"`
    ValidationRegex  string   `json:"validation_regex"`
    AllowedValues    []string `json:"allowed_values"`
    UpdatedAt        string   `json:"updated_at"`
}

type ConfigurationHistory struct {
    ID        int    `json:"id"`
    Key       string `json:"key"`
    OldValue  string `json:"old_value"`
    NewValue  string `json:"new_value"`
    ChangedBy string `json:"changed_by"`
    ChangedAt string `json:"changed_at"`
}

func validateConfigValue(config Configuration, value string) error {
    switch config.Type {
    case "boolean":
        if value != "true" && value != "false" {
            return fmt.Errorf("Value must be 'true' or 'false'")
        }
    case "number":
        if _, err := strconv.ParseFloat(value, 64); err != nil {
            return fmt.Errorf("Value must be a valid number")
        }
    case "duration":
        if _, err := time.ParseDuration(value); err != nil {
            return fmt.Errorf("Value must be a valid duration (e.g., '30m', '1h')")
        }
    case "enum":
        found := false
        for _, allowed := range config.AllowedValues {
            if value == allowed {
                found = true
                break
            }
        }
        if !found {
            return fmt.Errorf("Value must be one of: %s", strings.Join(config.AllowedValues, ", "))
        }
    case "array":
        // Validate JSON array
        var arr []string
        if err := json.Unmarshal([]byte(value), &arr); err != nil {
            return fmt.Errorf("Value must be a valid JSON array")
        }
    }

    // Regex validation if provided
    if config.ValidationRegex != "" {
        matched, err := regexp.MatchString(config.ValidationRegex, value)
        if err != nil || !matched {
            return fmt.Errorf("Value does not match required format")
        }
    }

    return nil
}

func testConfigValue(key, value string) (bool, string) {
    switch {
    case strings.HasPrefix(key, "ingress.domain"):
        // Test DNS resolution
        _, err := net.LookupHost(value)
        if err != nil {
            return false, fmt.Sprintf("DNS lookup failed: %v", err)
        }
        return true, "Domain is valid and resolvable"

    case strings.HasPrefix(key, "storage.className"):
        // In real implementation, query Kubernetes for StorageClass
        // For now, just return true
        return true, "StorageClass name format is valid"

    default:
        return true, "Validation passed"
    }
}
```

### Frontend Implementation

#### Component: `ui/src/pages/admin/Settings.tsx`

```typescript
import React, { useState, useEffect } from 'react'
import {
  Box,
  Paper,
  Typography,
  Tabs,
  Tab,
  TextField,
  Switch,
  Button,
  Grid,
  Select,
  MenuItem,
  FormControl,
  FormControlLabel,
  InputLabel,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  List,
  ListItem,
  ListItemText,
} from '@mui/material'
import { Save, History, Refresh } from '@mui/icons-material'
import { getConfigurations, updateConfiguration, testConfiguration, getConfigurationHistory } from '../../lib/api'

export default function Settings() {
  const [activeTab, setActiveTab] = useState(0)
  const [configs, setConfigs] = useState<Record<string, Configuration[]>>({})
  const [changes, setChanges] = useState<Record<string, string>>({})
  const [testResults, setTestResults] = useState<Record<string, { valid: boolean; message: string }>>({})
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState([])
  const [loading, setLoading] = useState(false)

  const categories = ['Ingress', 'Storage', 'Resources', 'Features', 'Session', 'Security', 'Compliance']

  useEffect(() => {
    loadConfigurations()
  }, [])

  const loadConfigurations = async () => {
    setLoading(true)
    try {
      const data = await getConfigurations()
      setConfigs(data.grouped)
    } catch (error) {
      console.error('Failed to load configurations:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleChange = (key: string, value: string) => {
    setChanges({ ...changes, [key]: value })
  }

  const handleTest = async (key: string) => {
    const value = changes[key]
    if (!value) return

    try {
      const result = await testConfiguration(key, value)
      setTestResults({ ...testResults, [key]: result })
    } catch (error) {
      setTestResults({
        ...testResults,
        [key]: { valid: false, message: 'Test failed' },
      })
    }
  }

  const handleSave = async (key: string) => {
    const value = changes[key]
    if (!value) return

    try {
      await updateConfiguration(key, value)
      await loadConfigurations()
      // Remove from changes
      const newChanges = { ...changes }
      delete newChanges[key]
      setChanges(newChanges)
      setTestResults({ ...testResults, [key]: { valid: true, message: 'Saved successfully' } })
    } catch (error) {
      setTestResults({ ...testResults, [key]: { valid: false, message: 'Save failed' } })
    }
  }

  const renderConfigField = (config: Configuration) => {
    const currentValue = changes[config.key] || config.value
    const testResult = testResults[config.key]

    switch (config.type) {
      case 'boolean':
        return (
          <FormControlLabel
            control={
              <Switch
                checked={currentValue === 'true'}
                onChange={(e) => handleChange(config.key, e.target.checked.toString())}
              />
            }
            label={config.description}
          />
        )

      case 'enum':
        return (
          <FormControl fullWidth>
            <InputLabel>{config.description}</InputLabel>
            <Select
              value={currentValue}
              onChange={(e) => handleChange(config.key, e.target.value)}
            >
              {config.allowed_values.map((value) => (
                <MenuItem key={value} value={value}>
                  {value}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )

      default:
        return (
          <TextField
            fullWidth
            label={config.description}
            value={currentValue}
            onChange={(e) => handleChange(config.key, e.target.value)}
            helperText={config.validation_regex ? `Format: ${config.validation_regex}` : ''}
          />
        )
    }
  }

  const currentCategory = categories[activeTab].toLowerCase()
  const categoryConfigs = configs[currentCategory] || []

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 3 }}>
        <Typography variant="h4">System Configuration</Typography>
        <Button startIcon={<History />} onClick={() => setShowHistory(true)}>
          View History
        </Button>
      </Box>

      <Paper>
        <Tabs value={activeTab} onChange={(e, v) => setActiveTab(v)}>
          {categories.map((category) => (
            <Tab key={category} label={category} />
          ))}
        </Tabs>

        <Box sx={{ p: 3 }}>
          <Grid container spacing={3}>
            {categoryConfigs.map((config) => (
              <Grid item xs={12} key={config.key}>
                <Box>
                  {renderConfigField(config)}

                  {changes[config.key] && (
                    <Box sx={{ mt: 1, display: 'flex', gap: 1 }}>
                      <Button size="small" variant="outlined" onClick={() => handleTest(config.key)}>
                        Test
                      </Button>
                      <Button size="small" variant="contained" onClick={() => handleSave(config.key)}>
                        Save
                      </Button>
                    </Box>
                  )}

                  {testResults[config.key] && (
                    <Alert severity={testResults[config.key].valid ? 'success' : 'error'} sx={{ mt: 1 }}>
                      {testResults[config.key].message}
                    </Alert>
                  )}
                </Box>
              </Grid>
            ))}
          </Grid>
        </Box>
      </Paper>

      {/* History Dialog */}
      <Dialog open={showHistory} onClose={() => setShowHistory(false)} maxWidth="md" fullWidth>
        <DialogTitle>Configuration History</DialogTitle>
        <DialogContent>
          <List>
            {history.map((item: any) => (
              <ListItem key={item.id}>
                <ListItemText
                  primary={`${item.key}: ${item.old_value} → ${item.new_value}`}
                  secondary={`${item.changed_by} at ${item.changed_at}`}
                />
              </ListItem>
            ))}
          </List>
        </DialogContent>
      </Dialog>
    </Box>
  )
}

interface Configuration {
  id: number
  key: string
  value: string
  type: string
  category: string
  description: string
  validation_regex: string
  allowed_values: string[]
  updated_at: string
}
```

---

## 3. License Management

**Priority:** P0 - CRITICAL
**Effort:** 3-4 days
**Reason:** Cannot sell Pro/Enterprise without license enforcement

*Implementation guide continues with detailed backend/frontend code for License Management, API Keys, Alert Management, Controller Management, and Session Recordings...*

---

## Common Patterns

### Error Handling

```typescript
// Standard error handling pattern
try {
  const result = await someApiCall()
  // Success handling
} catch (error) {
  if (axios.isAxiosError(error)) {
    const message = error.response?.data?.error || 'Operation failed'
    // Show error toast/snackbar
  }
}
```

### Loading States

```typescript
const [loading, setLoading] = useState(false)

const loadData = async () => {
  setLoading(true)
  try {
    const data = await fetchData()
    setData(data)
  } finally {
    setLoading(false)
  }
}
```

### Form Validation

```typescript
import { useForm } from 'react-hook-form'
import * as yup from 'yup'
import { yupResolver } from '@hookform/resolvers/yup'

const schema = yup.object({
  name: yup.string().required('Name is required'),
  email: yup.string().email('Invalid email').required('Email is required'),
})

const { register, handleSubmit, formState: { errors } } = useForm({
  resolver: yupResolver(schema)
})
```

---

## Testing Requirements

Each feature must include:

1. **Backend Tests** - API handler tests
2. **Frontend Tests** - Component/page tests
3. **Integration Tests** - End-to-end flow tests

Minimum coverage: 70% for new code

---

## Deployment Checklist

Before deploying admin UI features:

- [ ] All P0 features implemented and tested
- [ ] Database migrations applied
- [ ] API routes registered
- [ ] Frontend routes added to router
- [ ] Access control verified (admin-only)
- [ ] Error handling tested
- [ ] Documentation updated
- [ ] CHANGELOG.md updated

---

**Last Updated:** 2025-11-20
**Maintained By:** Agent 4 (Scribe)
**For:** Agent 2 (Builder)

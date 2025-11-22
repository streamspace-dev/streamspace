# Bug Report: Database Testability Issue

**Reporter**: Validator (Agent 3)
**Date**: 2025-11-20
**Priority**: HIGH (P1) - Blocks test coverage expansion
**Affected Component**: `api/internal/db/database.go`
**Assigned To**: Builder (Agent 2)

---

## Summary

The `db.Database` struct wraps `*sql.DB` in a private field, making it impossible to inject mock databases for unit testing. This blocks comprehensive test coverage for all handlers that depend on `*db.Database`.

## Problem Description

### Current Architecture

```go
// api/internal/db/database.go
type Database struct {
	db *sql.DB  // Private field - cannot be mocked
}

func NewDatabase(config Config) (*Database, error) {
	// Constructor requires real database connection
}
```

### Impact on Testing

Handlers that use `*db.Database` cannot be unit tested with mocks:

```go
// api/internal/handlers/audit.go
type AuditHandler struct {
	database *db.Database  // Cannot inject mock
}
```

**Affected Handlers** (P0 Admin Features):
1. ✅ **audit.go** (573 lines) - Audit Logs Viewer
2. ✅ **configuration.go** (465 lines) - System Configuration
3. ✅ **license.go** (755 lines) - License Management
4. ⚠️ **apikeys.go** (538 lines) - API Keys (uses raw *sql.DB, not affected)

**Additional Affected Handlers**: Likely all new handlers that follow the `*db.Database` pattern.

---

## Current Workaround

The `security.go` handler uses raw `*sql.DB` which can be mocked:

```go
// api/internal/handlers/security.go
type SecurityHandler struct {
	DB *sql.DB  // Can be mocked with sqlmock
}

// Tests work fine:
func setupSecurityTest(t *testing.T) (*SecurityHandler, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	handler := &SecurityHandler{DB: db}  // ✅ Works!
	return handler, mock, cleanup
}
```

---

## Proposed Solutions

### Option 1: Interface-Based Dependency Injection (Recommended)

Create a database interface that can be mocked:

```go
// api/internal/db/database.go
type Database interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	// Add other needed methods
}

type postgresDatabase struct {
	db *sql.DB
}

func NewDatabase(config Config) (Database, error) {
	// Return interface instead of concrete type
}
```

**Pros**:
- Clean dependency injection
- Easy to mock for tests
- Follows SOLID principles
- Allows for multiple database implementations

**Cons**:
- Requires refactoring all handlers
- More code changes

### Option 2: Expose Test Constructor

Add a test-only constructor that accepts `*sql.DB`:

```go
// api/internal/db/database.go
type Database struct {
	db *sql.DB
}

// NewDatabaseForTesting creates a Database from an existing sql.DB connection
// ONLY FOR TESTING - Do not use in production code
func NewDatabaseForTesting(db *sql.DB) *Database {
	return &Database{db: db}
}
```

**Pros**:
- Minimal code changes
- Backward compatible
- Quick to implement

**Cons**:
- Exposes internal implementation
- Could be misused in production code
- Less clean architecture

### Option 3: Expose DB Field for Testing

Make the field public or add a getter:

```go
type Database struct {
	DB *sql.DB  // Now public
}

// Or add getter:
func (d *Database) GetDB() *sql.DB {
	return d.db
}
```

**Pros**:
- Very simple
- Minimal changes

**Cons**:
- Breaks encapsulation
- Allows direct access to internal state

---

## Recommended Action

**Option 1 (Interface-Based)** is recommended for long-term maintainability, but requires more work.

**Option 2 (Test Constructor)** is a quick fix that unblocks testing immediately.

### Implementation Priority

**Phase 1 (Immediate - 1-2 hours)**:
- Implement Option 2 (test constructor) to unblock Validator's test coverage work
- Apply to all affected handlers

**Phase 2 (Future - v1.1+ or when time allows)**:
- Refactor to Option 1 (interface-based) for better architecture
- Include in technical debt backlog

---

## Evidence

### Test File Created

`api/internal/handlers/audit_test.go` - 23 comprehensive test cases (currently skipped)

**Test Coverage Attempted**:
- ✅ ListAuditLogs: 13 test cases (pagination, filters, edge cases)
- ✅ GetAuditLog: 3 test cases (success, not found, invalid ID)
- ✅ ExportAuditLogs: 6 test cases (JSON, CSV, errors)
- ✅ Benchmarks: 1 performance test

**Current Status**: All tests skip with message: "Pending: db.Database refactoring required"

### Code Reference

```go
// api/internal/handlers/audit_test.go:43-65
func setupAuditTest(t *testing.T) (*AuditHandler, sqlmock.Sqlmock, func()) {
	// SKIP ALL TESTS: db.Database needs refactoring for testability
	t.Skip("Pending: db.Database refactoring required - see comments below")

	// Cannot inject mock into *db.Database
	handler := &AuditHandler{
		database: nil, // ❌ No way to create testable database
	}
	// ...
}
```

---

## Impact Analysis

### Test Coverage Blocked

**Without Fix**:
- ❌ Cannot test audit.go (573 lines, 0% coverage)
- ❌ Cannot test configuration.go (465 lines, 0% coverage)
- ❌ Cannot test license.go (755 lines, 0% coverage)
- ❌ Cannot test any new handlers using *db.Database
- **Total Blocked**: 1,793+ lines of critical P0 code

**With Fix (Option 2)**:
- ✅ Can test all 3 P0 admin features
- ✅ Can test future handlers
- ✅ Target: 70%+ coverage achievable

### Time Estimate

**Option 2 Implementation**: 1-2 hours
- Add `NewDatabaseForTesting()` function
- Update test setup functions
- Verify tests pass

**Validator Can Resume Testing**: Immediately after fix

---

## Related Files

- `api/internal/db/database.go` - Needs refactoring
- `api/internal/handlers/audit.go` - Blocked from testing
- `api/internal/handlers/configuration.go` - Blocked from testing
- `api/internal/handlers/license.go` - Blocked from testing
- `api/internal/handlers/audit_test.go` - Test template ready (currently skipped)

---

## Next Steps

1. **Builder**: Implement Option 2 (test constructor) - 1-2 hours
2. **Validator**: Update test files to use new constructor - 30 minutes
3. **Validator**: Verify tests pass and provide coverage report
4. **Builder**: (Optional, v1.1+) Refactor to Option 1 (interface-based)

---

## Questions for Builder

1. Do you prefer Option 1, 2, or 3?
2. Should we apply this to all handlers or just P0 features first?
3. Is there a reason the private field pattern was used initially?
4. Are there other similar testability issues in the codebase?

---

**Status**: OPEN - Awaiting Builder response and implementation
**Blocker**: Yes - Blocks API handler test coverage expansion (P0 task)
**Estimated Fix Time**: 1-2 hours for Option 2

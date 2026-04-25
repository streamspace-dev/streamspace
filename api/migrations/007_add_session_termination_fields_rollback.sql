-- Rollback Migration 007: Remove session termination tracking fields
-- Purpose: Revert changes from 007_add_session_termination_fields.sql

-- Drop indexes
DROP INDEX IF EXISTS idx_sessions_termination_reason;
DROP INDEX IF EXISTS idx_sessions_terminated_at;

-- Drop columns
ALTER TABLE sessions
DROP COLUMN IF EXISTS terminated_at;

ALTER TABLE sessions
DROP COLUMN IF EXISTS termination_reason;

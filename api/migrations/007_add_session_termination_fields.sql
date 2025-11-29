-- Migration 007: Add session termination tracking fields
-- Purpose: Support Session Reconciliation Loop (Issue #235)
--
-- Adds fields to track why and when sessions were terminated,
-- especially for force-terminated sessions when agents are unavailable.

-- Add termination_reason column to track why sessions terminated
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS termination_reason VARCHAR(255);

-- Add terminated_at column to track when sessions were terminated
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS terminated_at TIMESTAMP;

-- Add index on terminated_at for efficient queries
CREATE INDEX IF NOT EXISTS idx_sessions_terminated_at
ON sessions(terminated_at);

-- Add index on termination_reason for analytics
CREATE INDEX IF NOT EXISTS idx_sessions_termination_reason
ON sessions(termination_reason);

COMMENT ON COLUMN sessions.termination_reason IS
'Reason for session termination (e.g., user_requested, agent_unavailable, timeout, error)';

COMMENT ON COLUMN sessions.terminated_at IS
'Timestamp when session entered terminated state';

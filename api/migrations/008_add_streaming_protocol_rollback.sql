-- Rollback Migration 008: Remove streaming protocol support
-- Purpose: Revert changes from 008_add_streaming_protocol.sql

-- Drop index
DROP INDEX IF EXISTS idx_sessions_streaming_protocol;

-- Drop columns
ALTER TABLE sessions
DROP COLUMN IF EXISTS streaming_path;

ALTER TABLE sessions
DROP COLUMN IF EXISTS streaming_port;

ALTER TABLE sessions
DROP COLUMN IF EXISTS streaming_protocol;

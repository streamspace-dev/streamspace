-- Rollback: Remove agent and cluster tracking from sessions table
-- Created: 2025-11-21
-- Purpose: Rollback for 002_add_agent_tracking_to_sessions.sql
--
-- This rollback script removes the agent_id and cluster_id columns and
-- their associated indexes from the sessions table, reverting to single-agent
-- architecture.

-- Drop indexes
DROP INDEX IF EXISTS idx_sessions_agent_state;
DROP INDEX IF EXISTS idx_sessions_cluster_id;
DROP INDEX IF EXISTS idx_sessions_agent_id;

-- Drop foreign key constraint
ALTER TABLE sessions
DROP CONSTRAINT IF EXISTS fk_sessions_agent_id;

-- Drop columns
ALTER TABLE sessions
DROP COLUMN IF EXISTS cluster_id;

ALTER TABLE sessions
DROP COLUMN IF EXISTS agent_id;

-- Rollback: Remove tags column from sessions table
-- Created: 2025-11-21
-- Purpose: Rollback for 001_add_tags_to_sessions.sql
--
-- This rollback script removes the tags column and its index from
-- the sessions table, reverting to the pre-tagging schema.

-- Drop the GIN index
DROP INDEX IF EXISTS idx_sessions_tags;

-- Drop the tags column
ALTER TABLE sessions
DROP COLUMN IF EXISTS tags;

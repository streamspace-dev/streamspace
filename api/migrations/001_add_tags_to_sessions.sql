-- Migration: Add tags column to sessions table
-- Created: 2025-11-21
-- Purpose: Support session tagging for organization and filtering
--
-- This migration adds a tags column to the sessions table to support
-- the v2.0-beta architecture where all session metadata is stored in
-- the database (not Kubernetes CRDs).

-- Add tags column with default empty array
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT ARRAY[]::TEXT[];

-- Add GIN index for efficient array overlap queries (tags && $1)
CREATE INDEX IF NOT EXISTS idx_sessions_tags
ON sessions USING GIN (tags);

-- Update any existing sessions to have empty tags array if NULL
UPDATE sessions
SET tags = ARRAY[]::TEXT[]
WHERE tags IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN sessions.tags IS 'User-defined tags for organizing and filtering sessions';

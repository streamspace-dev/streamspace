-- Migration: Add updated_at column to agent_commands table
-- Bug: P1-SCHEMA-002
-- Description: Adds updated_at timestamp column to track when commands are modified.
--              This column is required by CommandDispatcher for accurate status tracking.

-- Add updated_at column with default value
ALTER TABLE agent_commands
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

-- Backfill existing rows with created_at value
UPDATE agent_commands
SET updated_at = created_at
WHERE updated_at IS NULL;

-- Create trigger function to auto-update updated_at on row changes
CREATE OR REPLACE FUNCTION update_agent_commands_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to automatically update updated_at on every UPDATE
DROP TRIGGER IF EXISTS agent_commands_updated_at_trigger ON agent_commands;
CREATE TRIGGER agent_commands_updated_at_trigger
BEFORE UPDATE ON agent_commands
FOR EACH ROW
EXECUTE FUNCTION update_agent_commands_updated_at();

-- Verify migration
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'agent_commands' AND column_name = 'updated_at'
    ) THEN
        RAISE NOTICE 'Migration 004 completed successfully: updated_at column added';
    ELSE
        RAISE EXCEPTION 'Migration 004 failed: updated_at column not found';
    END IF;
END $$;

-- Rollback Migration: Remove updated_at column from agent_commands table
-- Bug: P1-SCHEMA-002
-- Description: Rolls back the addition of updated_at column and associated trigger.

-- Drop trigger first
DROP TRIGGER IF EXISTS agent_commands_updated_at_trigger ON agent_commands;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_agent_commands_updated_at();

-- Remove updated_at column
ALTER TABLE agent_commands
DROP COLUMN IF EXISTS updated_at;

-- Verify rollback
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'agent_commands' AND column_name = 'updated_at'
    ) THEN
        RAISE NOTICE 'Migration 004 rollback completed successfully: updated_at column removed';
    ELSE
        RAISE EXCEPTION 'Migration 004 rollback failed: updated_at column still exists';
    END IF;
END $$;

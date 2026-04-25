-- Rollback Migration 005: Remove API key authentication from agents table
--
-- This rollback removes the API key columns added in migration 005.
--
-- WARNING: This will remove all agent API keys and disable API key authentication.
--          Agents will be unable to authenticate after this rollback.

-- Drop the api_key_last_used_at column
ALTER TABLE agents
DROP COLUMN IF EXISTS api_key_last_used_at;

-- Drop the api_key_created_at column
ALTER TABLE agents
DROP COLUMN IF EXISTS api_key_created_at;

-- Drop the index
DROP INDEX IF EXISTS idx_agents_api_key_hash;

-- Drop the api_key_hash column
ALTER TABLE agents
DROP COLUMN IF EXISTS api_key_hash;

-- Rollback successful
DO $$
BEGIN
    RAISE NOTICE 'Migration 005 rollback completed: API key authentication columns removed';
END $$;

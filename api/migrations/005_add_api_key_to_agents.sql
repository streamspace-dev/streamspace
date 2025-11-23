-- Migration 005: Add API key authentication to agents table
--
-- SECURITY FIX: Phase 1.1 - Add API key column for agent authentication
--
-- This migration adds an api_key column to the agents table to support
-- secure agent-to-API authentication. API keys are hashed using bcrypt
-- and validated on agent registration and WebSocket connection.
--
-- Security Requirements:
--   - API keys must be cryptographically random (32+ bytes)
--   - API keys are hashed with bcrypt before storage (cost factor 12)
--   - API keys are never exposed in API responses
--   - API keys are rotatable (admin can generate new keys)
--
-- Usage:
--   1. Generate API key: openssl rand -hex 32
--   2. Hash with bcrypt (done by application)
--   3. Store hash in api_key_hash column
--   4. Provide plaintext key to agent (once, during deployment)
--   5. Agent sends key in X-Agent-API-Key header

-- Add api_key_hash column (stores bcrypt hash of API key)
ALTER TABLE agents
ADD COLUMN api_key_hash VARCHAR(255);

-- Add index for faster API key lookups during authentication
CREATE INDEX idx_agents_api_key_hash ON agents(api_key_hash);

-- Add api_key_created_at to track key age (for rotation policies)
ALTER TABLE agents
ADD COLUMN api_key_created_at TIMESTAMP;

-- Add api_key_last_used_at to track key usage (for anomaly detection)
ALTER TABLE agents
ADD COLUMN api_key_last_used_at TIMESTAMP;

-- Add constraint: api_key_hash cannot be NULL for new agents
-- (Existing agents will have NULL until key is generated)
-- We don't enforce NOT NULL to allow gradual migration

-- Add comment explaining the column
COMMENT ON COLUMN agents.api_key_hash IS 'Bcrypt hash of agent API key (cost factor 12). Used for agent authentication on registration and WebSocket connections.';
COMMENT ON COLUMN agents.api_key_created_at IS 'Timestamp when the API key was generated. Used for key rotation policies.';
COMMENT ON COLUMN agents.api_key_last_used_at IS 'Timestamp when the API key was last used successfully. Used for anomaly detection and auditing.';

-- Migration successful
DO $$
BEGIN
    RAISE NOTICE 'Migration 005 completed successfully: API key authentication columns added';
END $$;

-- Rollback Migration 006: Remove organizations table and org_id columns
-- WARNING: This will remove org isolation - only use for emergency rollback

-- Remove org_id from agents
ALTER TABLE agents DROP COLUMN IF EXISTS org_id;

-- Remove org_id from webhooks
ALTER TABLE webhooks DROP COLUMN IF EXISTS org_id;

-- Remove org_id from api_keys
ALTER TABLE api_keys DROP COLUMN IF EXISTS org_id;

-- Remove org_id from audit_log
ALTER TABLE audit_log DROP COLUMN IF EXISTS org_id;

-- Remove org_id from sessions
ALTER TABLE sessions DROP COLUMN IF EXISTS org_id;

-- Remove org_id from users
ALTER TABLE users DROP COLUMN IF EXISTS org_id;

-- Drop indexes (they are dropped automatically with columns)

-- Drop organizations table
DROP TABLE IF EXISTS organizations;

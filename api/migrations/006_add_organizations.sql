-- Migration 006: Add organizations table and org_id to users
-- This migration implements multi-tenancy by adding organization support
--
-- SECURITY: This is a P0 critical security fix to prevent cross-tenant data access

-- Create organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    k8s_namespace VARCHAR(255) NOT NULL DEFAULT 'streamspace',
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for organizations
CREATE INDEX IF NOT EXISTS idx_organizations_name ON organizations(name);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations(status);
CREATE INDEX IF NOT EXISTS idx_organizations_k8s_namespace ON organizations(k8s_namespace);

-- Add org_id to users table (nullable initially for backward compatibility)
ALTER TABLE users ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_users_org_id ON users(org_id);

-- Add org_id to sessions table
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_sessions_org_id ON sessions(org_id);

-- Add org_id to audit_log table (if exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'audit_log') THEN
        ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE;
        CREATE INDEX IF NOT EXISTS idx_audit_log_org_id ON audit_log(org_id);
    END IF;
END $$;

-- Add org_id to api_keys table (if exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'api_keys') THEN
        ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE;
        CREATE INDEX IF NOT EXISTS idx_api_keys_org_id ON api_keys(org_id);
    END IF;
END $$;

-- Add org_id to webhooks table (if exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'webhooks') THEN
        ALTER TABLE webhooks ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE;
        CREATE INDEX IF NOT EXISTS idx_webhooks_org_id ON webhooks(org_id);
    END IF;
END $$;

-- Add org_id to agents table (for org-scoped agent access)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'agents') THEN
        ALTER TABLE agents ADD COLUMN IF NOT EXISTS org_id VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE;
        CREATE INDEX IF NOT EXISTS idx_agents_org_id ON agents(org_id);
    END IF;
END $$;

-- Create a default organization for existing data
INSERT INTO organizations (id, name, display_name, description, k8s_namespace, status)
VALUES ('default-org', 'default', 'Default Organization', 'Default organization for existing data', 'streamspace', 'active')
ON CONFLICT (id) DO NOTHING;

-- Update existing users to belong to default org (if org_id is null)
UPDATE users SET org_id = 'default-org' WHERE org_id IS NULL;

-- Update existing sessions to belong to default org (if org_id is null)
UPDATE sessions SET org_id = 'default-org' WHERE org_id IS NULL;

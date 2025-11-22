-- Rollback: Remove cluster fields from agents table
-- Created: 2025-11-21
-- Purpose: Rollback for 003_add_cluster_fields_to_agents.sql
--
-- This rollback script removes the cluster_id and cluster_name columns
-- and their associated indexes from the agents table.

-- Drop indexes
DROP INDEX IF EXISTS idx_agents_cluster_status;
DROP INDEX IF EXISTS idx_agents_cluster_id;

-- Drop columns
DROP COLUMN IF EXISTS cluster_name;

ALTER TABLE agents
DROP COLUMN IF EXISTS cluster_id;

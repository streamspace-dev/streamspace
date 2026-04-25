-- Migration: Add cluster fields to agents table
-- Created: 2025-11-21
-- Purpose: Add cluster identification fields for multi-cluster support
--
-- This migration adds cluster_id and cluster_name to agents table to support:
-- - Multiple Kubernetes clusters managed by one API
-- - Cluster-based session routing
-- - Cluster affinity and preferences
-- - Multi-region deployments

-- Add cluster_id column to identify which cluster this agent belongs to
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(255);

-- Add cluster_name column for human-readable cluster identification
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS cluster_name VARCHAR(255);

-- Create index on cluster_id for efficient queries
-- Enables fast lookups like: "Show all agents in cluster X"
CREATE INDEX IF NOT EXISTS idx_agents_cluster_id
ON agents(cluster_id);

-- Create composite index for cluster + status queries
-- Enables fast lookups like: "Show online agents in cluster X"
CREATE INDEX IF NOT EXISTS idx_agents_cluster_status
ON agents(cluster_id, status);

-- Add comments for documentation
COMMENT ON COLUMN agents.cluster_id IS 'Unique identifier for the Kubernetes cluster this agent manages';
COMMENT ON COLUMN agents.cluster_name IS 'Human-readable name for the cluster (e.g., "prod-us-east-1")';

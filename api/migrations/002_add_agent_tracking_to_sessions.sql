-- Migration: Add agent and cluster tracking to sessions table
-- Created: 2025-11-21
-- Purpose: Enable multi-agent support for v2.0-beta architecture
--
-- This migration adds tracking fields to identify which agent and cluster
-- owns each session. This enables:
-- - Multi-cluster deployments (sessions can run on different K8s clusters)
-- - Agent load balancing (distribute sessions across multiple agents)
-- - Cluster affinity (route user sessions to preferred clusters)
-- - Agent health monitoring (identify sessions on offline agents)

-- Add agent_id column to track which agent owns this session
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS agent_id VARCHAR(255);

-- Add cluster_id column to track which cluster the session runs on
ALTER TABLE sessions
ADD COLUMN IF NOT EXISTS cluster_id VARCHAR(255);

-- Add foreign key constraint to agents table
-- ON DELETE SET NULL: If agent is deleted, session agent_id becomes null
ALTER TABLE sessions
ADD CONSTRAINT fk_sessions_agent_id
FOREIGN KEY (agent_id) REFERENCES agents(agent_id)
ON DELETE SET NULL;

-- Create index on agent_id for efficient queries
-- Enables fast lookups like: "Show all sessions on agent X"
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id
ON sessions(agent_id);

-- Create index on cluster_id for efficient queries
-- Enables fast lookups like: "Show all sessions in cluster Y"
CREATE INDEX IF NOT EXISTS idx_sessions_cluster_id
ON sessions(cluster_id);

-- Create composite index for agent + state queries
-- Enables fast lookups like: "Show running sessions on agent X"
CREATE INDEX IF NOT EXISTS idx_sessions_agent_state
ON sessions(agent_id, state);

-- Add comment for documentation
COMMENT ON COLUMN sessions.agent_id IS 'ID of the agent managing this session (for multi-agent routing)';
COMMENT ON COLUMN sessions.cluster_id IS 'ID of the Kubernetes cluster where this session runs';

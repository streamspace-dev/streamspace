// Package services provides business logic services for StreamSpace API.
//
// This file implements the AgentSelector service which handles intelligent
// routing of session creation requests to appropriate agents in multi-agent
// deployments.
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/streamspace-dev/streamspace/api/internal/websocket"
)

// AgentSelector handles selection of appropriate agents for session creation.
//
// The selector implements multiple strategies:
//   - Load balancing: Distribute sessions evenly across healthy agents
//   - Cluster affinity: Route to specific clusters when requested
//   - Region preference: Prefer agents in specific regions
//   - Capacity-based: Consider agent resource capacity
//   - Health filtering: Only select online agents with recent heartbeats
//
// Thread Safety: Safe for concurrent use from multiple goroutines.
type AgentSelector struct {
	db      *sql.DB
	agentHub *websocket.AgentHub
}

// AgentInfo represents agent metadata for selection decisions.
type AgentInfo struct {
	AgentID       string                 `json:"agent_id"`
	ClusterID     string                 `json:"cluster_id"`
	ClusterName   string                 `json:"cluster_name"`
	Platform      string                 `json:"platform"`
	Region        string                 `json:"region"`
	Status        string                 `json:"status"`
	SessionCount  int                    `json:"session_count"` // Current session load
	Capacity      map[string]interface{} `json:"capacity"`      // Resource capacity
	IsConnected   bool                   `json:"is_connected"`  // WebSocket connected
}

// SelectionCriteria defines criteria for selecting an agent.
type SelectionCriteria struct {
	// ClusterID restricts selection to a specific cluster (optional)
	ClusterID string

	// Region restricts selection to a specific region (optional)
	Region string

	// Platform restricts selection to a specific platform (kubernetes, docker, etc.)
	Platform string

	// PreferLowLoad prefers agents with fewer active sessions (default: true)
	PreferLowLoad bool

	// RequireConnected only selects agents with active WebSocket connections (default: true)
	RequireConnected bool
}

// NewAgentSelector creates a new AgentSelector instance.
//
// Parameters:
//   - db: Database connection for querying agent metadata
//   - agentHub: AgentHub for checking WebSocket connection status
//
// Example:
//
//	selector := services.NewAgentSelector(database.DB(), agentHub)
//	agent, err := selector.SelectAgent(ctx, &services.SelectionCriteria{})
func NewAgentSelector(db *sql.DB, agentHub *websocket.AgentHub) *AgentSelector {
	return &AgentSelector{
		db:      db,
		agentHub: agentHub,
	}
}

// SelectAgent selects the best available agent based on criteria.
//
// Selection Algorithm:
//  1. Filter agents by status (only 'online')
//  2. Filter by WebSocket connection (if RequireConnected)
//  3. Apply criteria filters (cluster, region, platform)
//  4. Calculate session load for each candidate
//  5. Select agent with lowest load (if PreferLowLoad)
//  6. Return selected agent or error if none available
//
// Returns:
//   - AgentInfo: Selected agent metadata
//   - error: If no suitable agent found or database error
//
// Example:
//
//	criteria := &SelectionCriteria{
//	    Region: "us-east-1",
//	    PreferLowLoad: true,
//	    RequireConnected: true,
//	}
//	agent, err := selector.SelectAgent(ctx, criteria)
func (s *AgentSelector) SelectAgent(ctx context.Context, criteria *SelectionCriteria) (*AgentInfo, error) {
	// Set defaults
	if criteria == nil {
		criteria = &SelectionCriteria{}
	}
	if !criteria.RequireConnected {
		criteria.RequireConnected = true // Default to requiring connection
	}
	if !criteria.PreferLowLoad {
		criteria.PreferLowLoad = true // Default to preferring low load
	}

	// Get all online agents from database
	agents, err := s.getOnlineAgents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online agents: %w", err)
	}

	if len(agents) == 0 {
		return nil, fmt.Errorf("no online agents available")
	}

	log.Printf("[AgentSelector] Found %d online agents", len(agents))

	// Filter by criteria
	candidates := s.filterAgents(agents, criteria)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no agents match selection criteria")
	}

	log.Printf("[AgentSelector] %d agents match criteria", len(candidates))

	// Calculate session load for each candidate
	for _, agent := range candidates {
		count, err := s.getAgentSessionCount(ctx, agent.AgentID)
		if err != nil {
			log.Printf("[AgentSelector] Warning: Failed to get session count for agent %s: %v", agent.AgentID, err)
			agent.SessionCount = 0
		} else {
			agent.SessionCount = count
		}
	}

	// Select agent with lowest load
	selected := candidates[0]
	if criteria.PreferLowLoad && len(candidates) > 1 {
		for _, agent := range candidates[1:] {
			if agent.SessionCount < selected.SessionCount {
				selected = agent
			}
		}
	}

	log.Printf("[AgentSelector] Selected agent %s (cluster: %s, load: %d sessions)",
		selected.AgentID, selected.ClusterID, selected.SessionCount)

	return selected, nil
}

// getOnlineAgents retrieves all agents with status='online' from database.
func (s *AgentSelector) getOnlineAgents(ctx context.Context) ([]*AgentInfo, error) {
	query := `
		SELECT
			agent_id, COALESCE(cluster_id, ''), COALESCE(cluster_name, ''),
			platform, COALESCE(region, ''), status, COALESCE(capacity, '{}'::jsonb)
		FROM agents
		WHERE status = 'online'
		ORDER BY last_heartbeat DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*AgentInfo
	for rows.Next() {
		agent := &AgentInfo{}
		var capacityJSON []byte

		err := rows.Scan(
			&agent.AgentID, &agent.ClusterID, &agent.ClusterName,
			&agent.Platform, &agent.Region, &agent.Status, &capacityJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan agent row: %w", err)
		}

		// Parse capacity JSON
		if len(capacityJSON) > 0 {
			if err := json.Unmarshal(capacityJSON, &agent.Capacity); err != nil {
				log.Printf("[AgentSelector] Warning: Failed to parse capacity for agent %s: %v", agent.AgentID, err)
				agent.Capacity = make(map[string]interface{})
			}
		}

		agents = append(agents, agent)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agent rows: %w", err)
	}

	return agents, nil
}

// filterAgents filters agents based on selection criteria.
func (s *AgentSelector) filterAgents(agents []*AgentInfo, criteria *SelectionCriteria) []*AgentInfo {
	var candidates []*AgentInfo

	for _, agent := range agents {
		// Check WebSocket connection if required
		if criteria.RequireConnected {
			agent.IsConnected = s.agentHub.IsAgentConnected(agent.AgentID)
			if !agent.IsConnected {
				log.Printf("[AgentSelector] Skipping agent %s (not connected via WebSocket)", agent.AgentID)
				continue
			}
		}

		// Filter by cluster
		if criteria.ClusterID != "" && agent.ClusterID != criteria.ClusterID {
			continue
		}

		// Filter by region
		if criteria.Region != "" && agent.Region != criteria.Region {
			continue
		}

		// Filter by platform
		if criteria.Platform != "" && agent.Platform != criteria.Platform {
			continue
		}

		candidates = append(candidates, agent)
	}

	return candidates
}

// getAgentSessionCount counts active sessions for an agent.
func (s *AgentSelector) getAgentSessionCount(ctx context.Context, agentID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM sessions
		WHERE agent_id = $1 AND state IN ('running', 'hibernated', 'pending')
	`

	var count int
	err := s.db.QueryRowContext(ctx, query, agentID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions for agent %s: %w", agentID, err)
	}

	return count, nil
}

// GetAgentInfo retrieves information about a specific agent.
//
// This is useful for displaying agent details or validating agent availability.
//
// Example:
//
//	info, err := selector.GetAgentInfo(ctx, "k8s-prod-us-east-1")
func (s *AgentSelector) GetAgentInfo(ctx context.Context, agentID string) (*AgentInfo, error) {
	query := `
		SELECT
			agent_id, COALESCE(cluster_id, ''), COALESCE(cluster_name, ''),
			platform, COALESCE(region, ''), status, COALESCE(capacity, '{}'::jsonb)
		FROM agents
		WHERE agent_id = $1
	`

	agent := &AgentInfo{}
	var capacityJSON []byte

	err := s.db.QueryRowContext(ctx, query, agentID).Scan(
		&agent.AgentID, &agent.ClusterID, &agent.ClusterName,
		&agent.Platform, &agent.Region, &agent.Status, &capacityJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent info: %w", err)
	}

	// Parse capacity JSON
	if len(capacityJSON) > 0 {
		if err := json.Unmarshal(capacityJSON, &agent.Capacity); err != nil {
			log.Printf("[AgentSelector] Warning: Failed to parse capacity: %v", err)
			agent.Capacity = make(map[string]interface{})
		}
	}

	// Check WebSocket connection
	agent.IsConnected = s.agentHub.IsAgentConnected(agentID)

	// Get session count
	count, err := s.getAgentSessionCount(ctx, agentID)
	if err != nil {
		log.Printf("[AgentSelector] Warning: Failed to get session count: %v", err)
		agent.SessionCount = 0
	} else {
		agent.SessionCount = count
	}

	return agent, nil
}

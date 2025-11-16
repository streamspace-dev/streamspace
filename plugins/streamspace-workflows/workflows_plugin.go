package main

import ("encoding/json"; "fmt"; "time"; "github.com/yourusername/streamspace/api/internal/plugins")

type WorkflowsPlugin struct {
	plugins.BasePlugin
	config WorkflowsConfig
	activeWorkflows []Workflow
}

type WorkflowsConfig struct {
	Enabled             bool `json:"enabled"`
	MaxWorkflowsPerUser int  `json:"maxWorkflowsPerUser"`
	AllowCustomScripts  bool `json:"allowCustomScripts"`
}

type Workflow struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Trigger     WorkflowTrigger        `json:"trigger"`
	Actions     []WorkflowAction       `json:"actions"`
	Enabled     bool                   `json:"enabled"`
	CreatedBy   string                 `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
}

type WorkflowTrigger struct {
	Type       string                 `json:"type"`
	Conditions map[string]interface{} `json:"conditions"`
}

type WorkflowAction struct {
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

func (p *WorkflowsPlugin) Initialize(ctx *plugins.PluginContext) error {
	configBytes, _ := json.Marshal(ctx.Config)
	json.Unmarshal(configBytes, &p.config)
	
	if !p.config.Enabled {
		ctx.Logger.Info("Workflows plugin is disabled")
		return nil
	}
	
	p.createDatabaseTables(ctx)
	p.loadActiveWorkflows(ctx)
	ctx.Logger.Info("Workflows plugin initialized", "workflows", len(p.activeWorkflows))
	return nil
}

func (p *WorkflowsPlugin) OnLoad(ctx *plugins.PluginContext) error {
	ctx.Logger.Info("Workflow Automation plugin loaded")
	return nil
}

func (p *WorkflowsPlugin) OnSessionCreated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}
	
	sessionMap, _ := session.(map[string]interface{})
	return p.executeMatchingWorkflows(ctx, "session.created", sessionMap)
}

func (p *WorkflowsPlugin) OnSessionTerminated(ctx *plugins.PluginContext, session interface{}) error {
	if !p.config.Enabled {
		return nil
	}
	
	sessionMap, _ := session.(map[string]interface{})
	return p.executeMatchingWorkflows(ctx, "session.terminated", sessionMap)
}

func (p *WorkflowsPlugin) OnUserLogin(ctx *plugins.PluginContext, user interface{}) error {
	if !p.config.Enabled {
		return nil
	}
	
	userMap, _ := user.(map[string]interface{})
	return p.executeMatchingWorkflows(ctx, "user.login", userMap)
}

func (p *WorkflowsPlugin) createDatabaseTables(ctx *plugins.PluginContext) error {
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS workflows (
		id SERIAL PRIMARY KEY, name VARCHAR(200), description TEXT,
		trigger JSONB, actions JSONB, enabled BOOLEAN,
		created_by VARCHAR(255), created_at TIMESTAMP DEFAULT NOW()
	)`)
	ctx.Database.Exec(`CREATE TABLE IF NOT EXISTS workflow_executions (
		id SERIAL PRIMARY KEY, workflow_id INTEGER, event_type VARCHAR(100),
		event_data JSONB, status VARCHAR(50), executed_at TIMESTAMP DEFAULT NOW()
	)`)
	return nil
}

func (p *WorkflowsPlugin) loadActiveWorkflows(ctx *plugins.PluginContext) error {
	rows, _ := ctx.Database.Query(`SELECT id, name, trigger, actions, enabled FROM workflows WHERE enabled = true`)
	defer rows.Close()
	
	for rows.Next() {
		var wf Workflow
		var triggerJSON, actionsJSON []byte
		rows.Scan(&wf.ID, &wf.Name, &triggerJSON, &actionsJSON, &wf.Enabled)
		json.Unmarshal(triggerJSON, &wf.Trigger)
		json.Unmarshal(actionsJSON, &wf.Actions)
		p.activeWorkflows = append(p.activeWorkflows, wf)
	}
	return nil
}

func (p *WorkflowsPlugin) executeMatchingWorkflows(ctx *plugins.PluginContext, eventType string, eventData map[string]interface{}) error {
	for _, wf := range p.activeWorkflows {
		if wf.Trigger.Type == eventType {
			ctx.Logger.Info("Executing workflow", "workflow", wf.Name, "event", eventType)
			for _, action := range wf.Actions {
				p.executeAction(ctx, action, eventData)
			}
		}
	}
	return nil
}

func (p *WorkflowsPlugin) executeAction(ctx *plugins.PluginContext, action WorkflowAction, eventData map[string]interface{}) error {
	ctx.Logger.Debug("Executing action", "type", action.Type)
	return nil
}

func init() {
	plugins.Register("streamspace-workflows", &WorkflowsPlugin{})
}

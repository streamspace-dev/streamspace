# StreamSpace Workflow Automation Plugin

Automate session lifecycle with triggers, actions, and custom workflow definitions.

## Features
- Event-driven workflows
- Multiple trigger types (session.created, session.terminated, user.login, schedule)
- Multiple action types (webhook, email, snapshot, recording, script)
- Conditional logic
- Workflow execution history

## Installation
Admin → Plugins → "Workflow Automation" → Install

## Configuration
```json
{
  "enabled": true,
  "maxWorkflowsPerUser": 50,
  "allowCustomScripts": false
}
```

## Example Workflow
```json
{
  "name": "Auto-snapshot on session end",
  "trigger": {"type": "session.terminated"},
  "actions": [
    {"type": "create_snapshot", "parameters": {"name": "auto-{{timestamp}}"}}
  ]
}
```

## License
MIT

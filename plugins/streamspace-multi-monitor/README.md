# StreamSpace Multi-Monitor Support Plugin

Enables advanced multi-monitor configurations for sessions with independent VNC streams for each display.

## Features
- Create custom monitor layouts (horizontal, vertical, grid, custom)
- Support for up to 16 monitors per session
- Independent VNC streams for each display
- Monitor-specific settings (resolution, rotation, scale)
- Save and reuse monitor configurations

## Installation
Install via Plugin Marketplace: Admin > Plugins > Search "Multi-Monitor"

## Configuration
```json
{
  "maxMonitorsPerSession": 8,
  "defaultLayout": "horizontal",
  "allowCustomLayouts": true
}
```

## API Endpoints
All endpoints are prefixed with `/api/plugins/streamspace-multi-monitor`

- `POST /sessions/:sessionId/monitors` - Create monitor configuration
- `GET /sessions/:sessionId/monitors` - List configurations
- `POST /sessions/:sessionId/monitors/:configId/activate` - Activate configuration
- `GET /sessions/:sessionId/monitors/:configId/streams` - Get VNC stream URLs

## Database Tables
- `monitor_configurations` - Saved monitor layouts
- `monitor_displays` - Individual display settings

## License
MIT - StreamSpace Team

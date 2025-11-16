# StreamSpace Calendar Integration Plugin

Integrate Google Calendar and Outlook Calendar with automated session scheduling and iCal export.

## Features
- Google Calendar OAuth integration
- Microsoft Outlook Calendar OAuth integration
- Auto-sync scheduled sessions to calendar
- iCalendar (.ics) export for scheduled sessions
- Automatic session creation from calendar events
- Configurable sync intervals

## Installation
Install via Plugin Marketplace: Admin > Plugins > Search "Calendar"

## Configuration
```json
{
  "googleClientId": "YOUR_GOOGLE_CLIENT_ID",
  "googleClientSecret": "YOUR_GOOGLE_CLIENT_SECRET",
  "microsoftClientId": "YOUR_MICROSOFT_CLIENT_ID",
  "microsoftClientSecret": "YOUR_MICROSOFT_CLIENT_SECRET",
  "autoSyncInterval": 300,
  "createEventsForScheduledSessions": true
}
```

## Setup
1. Create Google OAuth credentials at console.cloud.google.com
2. Create Microsoft OAuth app at portal.azure.com
3. Configure callback URL: `https://your-domain/api/plugins/streamspace-calendar/calendar/oauth/callback`
4. Enter credentials in plugin configuration

## API Endpoints
All endpoints are prefixed with `/api/plugins/streamspace-calendar`

- `POST /calendar/integrations/:provider` - Connect calendar (google/outlook)
- `GET /calendar/integrations` - List connected calendars
- `POST /calendar/integrations/:id/sync` - Sync calendar
- `GET /calendar/export` - Export iCalendar file
- `DELETE /calendar/integrations/:id` - Disconnect calendar

## Database Tables
- `calendar_integrations` - Connected calendar accounts
- `calendar_oauth_states` - OAuth flow state tracking
- `calendar_events` - Synced calendar events

## License
MIT - StreamSpace Team

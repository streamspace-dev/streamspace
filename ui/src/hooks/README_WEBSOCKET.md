# Enterprise WebSocket Integration

Real-time push notifications for StreamSpace enterprise features.

## Quick Start

### 1. Wrap your app with the WebSocket provider

```tsx
import EnterpriseWebSocketProvider from './components/EnterpriseWebSocketProvider';

function App() {
  return (
    <EnterpriseWebSocketProvider enableNotifications={true}>
      <YourApp />
    </EnterpriseWebSocketProvider>
  );
}
```

This automatically:
- Connects to the WebSocket endpoint
- Reconnects on connection loss
- Displays toast notifications for events
- Handles all enterprise event types

### 2. Listen to specific events in your components

```tsx
import { useSecurityAlertEvents } from '../hooks/useEnterpriseWebSocket';

function SecurityDashboard() {
  const [alerts, setAlerts] = useState([]);

  useSecurityAlertEvents((data) => {
    console.log('Security alert:', data);
    setAlerts(prev => [...prev, data]);
  });

  return <div>...</div>;
}
```

## Available Event Hooks

### Security Events
```tsx
useSecurityAlertEvents((data) => {
  // data.alert_type, data.severity, data.message
  console.log('Security alert:', data);
});
```

### Webhook Delivery Events
```tsx
useWebhookDeliveryEvents((data) => {
  // data.webhook_id, data.delivery_id, data.status
  console.log('Webhook delivery:', data);
});
```

### Schedule Events
```tsx
useScheduleEvents((data) => {
  // data.schedule_id, data.event, data.session_id
  console.log('Schedule event:', data);
});
```

### Node Health Events (Admin)
```tsx
useNodeHealthEvents((data) => {
  // data.node_name, data.health_status, data.cpu_percent, data.memory_percent
  console.log('Node health:', data);
});
```

### Scaling Events (Admin)
```tsx
useScalingEvents((data) => {
  // data.policy_id, data.action, data.result
  console.log('Scaling event:', data);
});
```

### Compliance Violation Events
```tsx
useComplianceViolationEvents((data) => {
  // data.violation_id, data.policy_id, data.severity
  console.log('Compliance violation:', data);
});
```

## Advanced Usage

### Custom WebSocket hook

```tsx
import { useEnterpriseWebSocket } from '../hooks/useEnterpriseWebSocket';

function MyComponent() {
  const {
    isConnected,
    lastMessage,
    sendMessage,
    reconnectAttempts
  } = useEnterpriseWebSocket({
    onMessage: (message) => {
      console.log('Received:', message);
    },
    onError: (error) => {
      console.error('WebSocket error:', error);
    },
    onClose: () => {
      console.log('Connection closed');
    },
    onOpen: () => {
      console.log('Connection opened');
    },
    autoReconnect: true,
    reconnectInterval: 3000,
    maxReconnectAttempts: 10,
  });

  return (
    <div>
      Status: {isConnected ? 'Connected' : 'Disconnected'}
      {reconnectAttempts > 0 && <p>Reconnecting... ({reconnectAttempts})</p>}
    </div>
  );
}
```

### Listen to specific event types

```tsx
import { useWebSocketEvent } from '../hooks/useEnterpriseWebSocket';

function MyComponent() {
  useWebSocketEvent('custom.event', (data) => {
    console.log('Custom event:', data);
  });

  return <div>...</div>;
}
```

## Event Types

| Event Type | Description | User/Admin |
|------------|-------------|------------|
| `webhook.delivery` | Webhook delivery status update | User |
| `security.alert` | Security alert triggered | User |
| `schedule.event` | Scheduled session lifecycle event | User |
| `node.health` | Cluster node health status | Admin |
| `scaling.event` | Auto-scaling operation | Admin |
| `compliance.violation` | Compliance policy violation | User/Admin |
| `connection` | WebSocket connection status | User/Admin |

## Message Format

All WebSocket messages follow this format:

```typescript
interface WebSocketMessage {
  type: string;           // Event type (e.g., "security.alert")
  timestamp: string;      // ISO 8601 timestamp
  data: Record<string, any>; // Event-specific data
}
```

### Example Messages

**Security Alert:**
```json
{
  "type": "security.alert",
  "timestamp": "2025-11-15T10:30:00Z",
  "data": {
    "alert_type": "failed_login",
    "severity": "high",
    "message": "Multiple failed login attempts"
  }
}
```

**Webhook Delivery:**
```json
{
  "type": "webhook.delivery",
  "timestamp": "2025-11-15T10:30:00Z",
  "data": {
    "webhook_id": 5,
    "delivery_id": 10,
    "status": "success"
  }
}
```

**Scheduled Session:**
```json
{
  "type": "schedule.event",
  "timestamp": "2025-11-15T09:00:00Z",
  "data": {
    "schedule_id": 3,
    "event": "started",
    "session_id": "user1-vscode-123"
  }
}
```

**Node Health:**
```json
{
  "type": "node.health",
  "timestamp": "2025-11-15T10:30:00Z",
  "data": {
    "node_name": "worker-1",
    "health_status": "healthy",
    "cpu_percent": 45.2,
    "memory_percent": 62.8
  }
}
```

## Troubleshooting

### WebSocket not connecting

**Check:**
1. Authentication token is present in localStorage
2. WebSocket URL is correct (ws:// or wss://)
3. Network allows WebSocket connections
4. Backend WebSocket handler is running

**Browser console:**
```javascript
// Check WebSocket connection
const ws = new WebSocket('ws://localhost:8000/api/v1/ws/enterprise');
ws.onopen = () => console.log('Connected');
ws.onerror = (e) => console.error('Error:', e);
```

### Reconnection failing

**Check:**
1. Max reconnection attempts (default: 10)
2. Reconnection interval (default: 3000ms)
3. Backend server status

**Adjust settings:**
```tsx
useEnterpriseWebSocket({
  autoReconnect: true,
  reconnectInterval: 5000,     // 5 seconds
  maxReconnectAttempts: 20,    // 20 attempts
});
```

### Not receiving events

**Check:**
1. WebSocket is connected (`isConnected === true`)
2. Event type matches exactly
3. Backend is broadcasting events
4. User has permissions for the event type

**Debug:**
```tsx
const { lastMessage, isConnected } = useEnterpriseWebSocket({
  onMessage: (msg) => console.log('Message:', msg),
});

console.log('Connected:', isConnected);
console.log('Last message:', lastMessage);
```

## Performance Tips

1. **Use specific event hooks** instead of subscribing to all events
2. **Memoize handlers** to prevent unnecessary re-renders
3. **Clean up subscriptions** when components unmount (automatic with hooks)
4. **Limit notification frequency** to avoid UI spam

```tsx
// Good: Specific event hook
useSecurityAlertEvents(handleAlert);

// Avoid: Subscribing to all events
useEnterpriseWebSocket({ onMessage: handleAllMessages });
```

## Security Notes

- WebSocket endpoint requires JWT authentication
- Token is automatically included from localStorage
- Connection is rejected if token is invalid/expired
- Use WSS (WebSocket Secure) in production
- Validate all received data before using

## Backend Integration

The WebSocket endpoint is:
```
ws://localhost:8000/api/v1/ws/enterprise
```

Authentication:
- Token from localStorage is used
- Middleware validates on connection

Broadcasting from backend:
```go
// Broadcast to specific user
handlers.BroadcastSecurityAlert("user1", "failed_login", "high", "Multiple failed attempts")

// Broadcast to all admins
handlers.BroadcastNodeHealthUpdate("worker-1", "unhealthy", 95.0, 98.0)
```

## Examples

See:
- `components/EnterpriseWebSocketProvider.tsx` - Full provider implementation
- `hooks/useEnterpriseWebSocket.ts` - Hook implementation
- `pages/SecuritySettings.test.tsx` - Test examples

import { ReactNode, useCallback, useEffect } from 'react';
import { Snackbar, Alert } from '@mui/material';
import { useState } from 'react';
import {
  useEnterpriseWebSocket,
  WebSocketMessage,
  useSecurityAlertEvents,
  useWebhookDeliveryEvents,
  useScheduleEvents,
  useScalingEvents,
  useComplianceViolationEvents,
} from '../hooks/useEnterpriseWebSocket';

interface EnterpriseWebSocketProviderProps {
  children: ReactNode;
  enableNotifications?: boolean;
}

interface Notification {
  id: string;
  message: string;
  severity: 'success' | 'info' | 'warning' | 'error';
}

/**
 * EnterpriseWebSocketProvider - Provider for WebSocket connection and notifications
 *
 * Manages enterprise WebSocket connection for real-time events and displays
 * toast notifications for various system events. Handles connection lifecycle,
 * reconnection attempts, and event routing to appropriate handlers.
 *
 * Supported events:
 * - webhook.delivery: Webhook delivery status updates
 * - security.alert: Security alerts and warnings
 * - schedule.event: Scheduled session events
 * - node.health: Cluster node health updates
 * - scaling.event: Auto-scaling events
 * - compliance.violation: Compliance violation detections
 * - connection: WebSocket connection status
 *
 * Features:
 * - Real-time WebSocket connection management
 * - Auto-reconnection with exponential backoff
 * - Toast notifications for events
 * - Stacked notification display
 * - Connection status indicator
 * - Event-specific notification handling
 * - Optional notification enable/disable
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {ReactNode} props.children - Child components
 * @param {boolean} [props.enableNotifications=true] - Whether to show notifications
 *
 * @returns {JSX.Element} Provider with children and notification UI
 *
 * @example
 * <EnterpriseWebSocketProvider enableNotifications={true}>
 *   <App />
 * </EnterpriseWebSocketProvider>
 *
 * @see useEnterpriseWebSocket for WebSocket hook
 * @see useSecurityAlertEvents for security event handling
 * @see useWebhookDeliveryEvents for webhook event handling
 */
export default function EnterpriseWebSocketProvider({
  children,
  enableNotifications = true,
}: EnterpriseWebSocketProviderProps) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [reconnectDismissed, setReconnectDismissed] = useState(false); // Track if reconnect banner was dismissed

  const addNotification = useCallback((message: string, severity: Notification['severity']) => {
    const id = `${Date.now()}-${Math.random()}`;
    setNotifications((prev) => [...prev, { id, message, severity }]);
  }, []);

  const removeNotification = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }, []);

  const handleMessage = useCallback(
    (message: WebSocketMessage) => {
      // console.log('[EnterpriseWebSocket] Received message:', message);

      if (!enableNotifications) {
        return;
      }

      // Handle different message types
      switch (message.type) {
        case 'webhook.delivery':
          handleWebhookDelivery(message.data);
          break;
        case 'security.alert':
          handleSecurityAlert(message.data);
          break;
        case 'schedule.event':
          handleScheduleEvent(message.data);
          break;
        case 'node.health':
          handleNodeHealth(message.data);
          break;
        case 'scaling.event':
          handleScalingEvent(message.data);
          break;
        case 'compliance.violation':
          handleComplianceViolation(message.data);
          break;
        case 'connection':
          if (message.data.status === 'connected') {
            addNotification('Real-time updates connected', 'success');
          }
          break;
        default:
          // console.log('[EnterpriseWebSocket] Unknown message type:', message.type);
          break;
      }
    },
    [enableNotifications, addNotification]
  );

  const handleWebhookDelivery = (data: any) => {
    const status = data.status;
    const severity = status === 'success' ? 'success' : status === 'failed' ? 'error' : 'info';
    addNotification(`Webhook delivery ${status}`, severity);
  };

  const handleSecurityAlert = (data: any) => {
    const severity = data.severity === 'high' || data.severity === 'critical' ? 'error' : 'warning';
    addNotification(`Security Alert: ${data.message}`, severity);
  };

  const handleScheduleEvent = (data: any) => {
    const event = data.event;
    if (event === 'started') {
      addNotification(`Scheduled session started: ${data.session_id}`, 'success');
    } else if (event === 'failed') {
      addNotification(`Scheduled session failed to start`, 'error');
    }
  };

  const handleNodeHealth = (data: any) => {
    const status = data.health_status;
    if (status === 'unhealthy') {
      addNotification(`Node ${data.node_name} is unhealthy`, 'error');
    }
  };

  const handleScalingEvent = (data: any) => {
    const action = data.action;
    const result = data.result;
    const severity = result === 'success' ? 'success' : 'error';
    addNotification(`Scaling ${action}: ${result}`, severity);
  };

  const handleComplianceViolation = (data: any) => {
    const severity = data.severity === 'high' || data.severity === 'critical' ? 'error' : 'warning';
    addNotification(`Compliance violation detected (${data.severity})`, severity);
  };

  const { isConnected, reconnectAttempts } = useEnterpriseWebSocket({
    onMessage: handleMessage,
    onError: (error) => {
      console.error('[EnterpriseWebSocket] Error:', error);
      if (enableNotifications) {
        addNotification('Real-time updates disconnected', 'error');
      }
    },
    onClose: () => {
      // console.log('[EnterpriseWebSocket] Connection closed');
      if (enableNotifications && reconnectAttempts > 0) {
        addNotification('Reconnecting to real-time updates...', 'info');
      }
    },
    autoReconnect: true,
    reconnectInterval: 3000,
    maxReconnectAttempts: 10,
  });

  // Show connection status indicator
  useEffect(() => {
    if (isConnected && reconnectAttempts > 0) {
      addNotification('Real-time updates reconnected', 'success');
    }
  }, [isConnected, reconnectAttempts, addNotification]);

  return (
    <>
      {children}

      {/* Toast notifications for WebSocket events */}
      {notifications.map((notification, index) => (
        <Snackbar
          key={notification.id}
          open={true}
          autoHideDuration={6000}
          onClose={() => removeNotification(notification.id)}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          sx={{
            bottom: 24 + index * 70, // Stack notifications
          }}
        >
          <Alert
            onClose={() => removeNotification(notification.id)}
            severity={notification.severity}
            variant="filled"
            sx={{ width: '100%' }}
          >
            {notification.message}
          </Alert>
        </Snackbar>
      ))}

      {/* Connection status indicator (optional) */}
      {!isConnected && reconnectAttempts > 0 && !reconnectDismissed && (
        <Snackbar
          open={true}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
          onClose={() => setReconnectDismissed(true)}
        >
          <Alert
            severity="info"
            variant="outlined"
            onClose={() => setReconnectDismissed(true)}
            sx={{
              backgroundColor: 'background.paper',
              boxShadow: 1,
            }}
          >
            Reconnecting... (Attempt {reconnectAttempts}/10)
          </Alert>
        </Snackbar>
      )}
    </>
  );
}

// Example usage in component:
/*
import EnterpriseWebSocketProvider from './components/EnterpriseWebSocketProvider';

function App() {
  return (
    <EnterpriseWebSocketProvider enableNotifications={true}>
      <YourApp />
    </EnterpriseWebSocketProvider>
  );
}
*/

// Example usage of individual event hooks:
/*
import { useSecurityAlertEvents } from './hooks/useEnterpriseWebSocket';

function SecurityDashboard() {
  const [alerts, setAlerts] = useState([]);

  useSecurityAlertEvents((data) => {
    console.log('Security alert received:', data);
    setAlerts(prev => [...prev, data]);
  });

  return (
    <div>
      {alerts.map(alert => (
        <Alert severity={alert.severity}>{alert.message}</Alert>
      ))}
    </div>
  );
}
*/

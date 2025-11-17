import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import { useUserStore } from '../store/userStore';

export interface WebSocketMessage {
  type: string;
  timestamp: string;
  data: Record<string, any>;
}

export type WebSocketMessageHandler = (message: WebSocketMessage) => void;

interface UseEnterpriseWebSocketOptions {
  onMessage?: WebSocketMessageHandler;
  onError?: (error: Event) => void;
  onClose?: () => void;
  onOpen?: () => void;
  autoReconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

interface UseEnterpriseWebSocketReturn {
  isConnected: boolean;
  lastMessage: WebSocketMessage | null;
  sendMessage: (message: any) => void;
  connect: () => void;
  disconnect: () => void;
  reconnectAttempts: number;
}

/**
 * Custom hook for managing enterprise WebSocket connections.
 *
 * Provides a high-level WebSocket interface with automatic reconnection,
 * message handling, and connection status. Designed for enterprise features
 * like webhooks, security alerts, node health monitoring, and compliance.
 *
 * Features:
 * - Automatic reconnection with custom backoff (30s, 15s, 15s, then 60s)
 * - Authentication via query parameter (reactive to token changes)
 * - Page visibility handling (reconnects when page becomes visible)
 * - Typed message handling with WebSocketMessage interface
 * - Manual connection control (connect, disconnect)
 *
 * Message Format:
 * All messages follow the WebSocketMessage interface:
 * ```ts
 * {
 *   type: string;        // Event type (e.g., 'webhook.delivery', 'security.alert')
 *   timestamp: string;   // ISO 8601 timestamp
 *   data: object;        // Event-specific data payload
 * }
 * ```
 *
 * Reconnection Strategy:
 * - 1st retry: 30 seconds after disconnect
 * - 2nd retry: 15 seconds
 * - 3rd retry: 15 seconds
 * - 4th+ retries: 60 seconds each (up to maxReconnectAttempts)
 *
 * @param options - WebSocket configuration options
 * @param options.onMessage - Optional callback when message received
 * @param options.onError - Optional callback when error occurs
 * @param options.onClose - Optional callback when connection closes
 * @param options.onOpen - Optional callback when connection opens
 * @param options.autoReconnect - Enable automatic reconnection (default: true)
 * @param options.reconnectInterval - Ignored (kept for backwards compatibility)
 * @param options.maxReconnectAttempts - Maximum reconnection attempts (default: 10)
 *
 * @returns WebSocket connection state and controls
 * @returns isConnected - Boolean indicating if WebSocket is currently connected
 * @returns lastMessage - Most recent WebSocketMessage received (or null)
 * @returns sendMessage - Function to send JSON message (auto-stringified)
 * @returns connect - Function to manually initiate connection
 * @returns disconnect - Function to close connection and prevent reconnection
 * @returns reconnectAttempts - Number of reconnection attempts made
 *
 * @example
 * ```tsx
 * const { isConnected, lastMessage } = useEnterpriseWebSocket({
 *   onMessage: (message) => {
 *     console.log('Received:', message.type, message.data);
 *   },
 *   onError: (error) => console.error('WebSocket error:', error),
 * });
 * ```
 */
export function useEnterpriseWebSocket(
  options: UseEnterpriseWebSocketOptions = {}
): UseEnterpriseWebSocketReturn {
  const {
    onMessage,
    onError,
    onClose,
    onOpen,
    autoReconnect = true,
    reconnectInterval = 3000, // Not used with custom backoff
    maxReconnectAttempts = 10,
  } = options;

  // Custom backoff pattern: 30s, 15s, 15s, then 60s for all subsequent attempts
  const getReconnectDelay = (attemptNumber: number): number => {
    if (attemptNumber === 0) return 30000; // 30 seconds for first retry
    if (attemptNumber === 1) return 15000; // 15 seconds for second retry
    if (attemptNumber === 2) return 15000; // 15 seconds for third retry
    return 60000; // 60 seconds for all subsequent retries
  };

  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const shouldReconnectRef = useRef(true);
  const reconnectAttemptsRef = useRef(0);

  // Store callbacks in refs to avoid reconnection when they change
  const onMessageRef = useRef(onMessage);
  const onErrorRef = useRef(onError);
  const onOpenRef = useRef(onOpen);
  const onCloseRef = useRef(onClose);

  // Update refs when callbacks change
  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

  useEffect(() => {
    onOpenRef.current = onOpen;
  }, [onOpen]);

  useEffect(() => {
    onCloseRef.current = onClose;
  }, [onClose]);

  // Get token directly from Zustand store - automatically reactive
  const token = useUserStore((state) => state?.token);

  // Memoize WebSocket URL, recalculate when token changes
  const wsUrl = useMemo(() => {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.host;

      // Don't connect without a token
      if (!token) {
        return '';
      }

      // Include token as query parameter for WebSocket authentication
      // Browsers cannot send custom headers in WebSocket connections
      return `${protocol}//${host}/api/v1/ws/enterprise?token=${encodeURIComponent(token)}`;
    } catch (error) {
      console.error('[useEnterpriseWebSocket] Error building URL:', error);
      return '';
    }
  }, [token]); // Recalculate when token changes

  const connect = useCallback(() => {
    // Don't create multiple connections
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      return;
    }

    // Don't connect if no token available
    if (!wsUrl) {
      console.log('[WebSocket] No authentication token, skipping connection');
      return;
    }

    try {
      // console.log(`[WebSocket] Connecting to ${wsUrl}`);

      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        // console.log('[WebSocket] Connected to enterprise WebSocket');
        setIsConnected(true);
        setReconnectAttempts(0);
        reconnectAttemptsRef.current = 0;
        shouldReconnectRef.current = true;

        if (onOpenRef.current) {
          onOpenRef.current();
        }
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          // console.log('[WebSocket] Received message:', message);

          setLastMessage(message);

          if (onMessageRef.current) {
            onMessageRef.current(message);
          }
        } catch (error) {
          console.error('[WebSocket] Failed to parse message:', error);
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
        setIsConnected(false);

        if (onErrorRef.current) {
          onErrorRef.current(error);
        }
      };

      wsRef.current.onclose = () => {
        // console.log('[WebSocket] Connection closed');
        setIsConnected(false);

        if (onCloseRef.current) {
          onCloseRef.current();
        }

        // Attempt reconnection if enabled and within retry limit
        const currentAttempts = reconnectAttemptsRef.current;
        if (
          shouldReconnectRef.current &&
          autoReconnect &&
          currentAttempts < maxReconnectAttempts
        ) {
          const delay = getReconnectDelay(currentAttempts);
          console.log(
            `[WebSocket] Attempting reconnection in ${delay / 1000}s (attempt ${currentAttempts + 1}/${maxReconnectAttempts})`
          );

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptsRef.current += 1;
            setReconnectAttempts((prev) => prev + 1);
            connect();
          }, delay);
        } else if (currentAttempts >= maxReconnectAttempts) {
          console.error('[WebSocket] Max reconnection attempts reached');
        }
      };
    } catch (error) {
      console.error('[WebSocket] Failed to create WebSocket connection:', error);
      setIsConnected(false);
    }
  }, [wsUrl, autoReconnect, maxReconnectAttempts]); // Removed reconnectInterval since we use getReconnectDelay

  const disconnect = useCallback(() => {
    // console.log('[WebSocket] Disconnecting...');
    shouldReconnectRef.current = false;

    // Clear reconnection timeout
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // Close WebSocket connection
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setIsConnected(false);
  }, []);

  const sendMessage = useCallback((message: any) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
    } else {
      console.warn('[WebSocket] Cannot send message: WebSocket is not connected');
    }
  }, []);

  // Auto-connect on mount (only if URL is not empty)
  useEffect(() => {
    if (!wsUrl) {
      return; // Don't connect without a valid URL
    }

    connect();

    // Cleanup on unmount
    return () => {
      shouldReconnectRef.current = false;

      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }

      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [wsUrl, connect]); // React to URL changes so we connect when token becomes available

  // Handle page visibility changes (reconnect when page becomes visible)
  // Store isConnected in ref to avoid effect recreation
  const isConnectedRef = useRef(isConnected);
  useEffect(() => {
    isConnectedRef.current = isConnected;
  }, [isConnected]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible' && !isConnectedRef.current) {
        // console.log('[WebSocket] Page visible, attempting reconnection');
        connect();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []); // Only set up listener once on mount

  return {
    isConnected,
    lastMessage,
    sendMessage,
    connect,
    disconnect,
    reconnectAttempts,
  };
}

/**
 * Hook for listening to specific WebSocket event types.
 *
 * Filters WebSocket messages by event type and calls the handler only when
 * messages of the specified type are received. Uses useEnterpriseWebSocket
 * internally for connection management.
 *
 * @param eventType - Event type to listen for (e.g., 'webhook.delivery')
 * @param handler - Callback function called when matching event received
 * @param handler.data - Event data payload from message.data
 * @param enabled - Whether event handling is enabled (default: true)
 *
 * @example
 * ```tsx
 * useWebSocketEvent('security.alert', (data) => {
 *   showNotification(`Security Alert: ${data.message}`);
 * });
 * ```
 */
export function useWebSocketEvent(
  eventType: string,
  handler: (data: any) => void,
  enabled = true
) {
  const { lastMessage } = useEnterpriseWebSocket({
    autoReconnect: true,
  });

  // Store handler in ref to avoid re-running effect when handler changes
  const handlerRef = useRef(handler);

  useEffect(() => {
    handlerRef.current = handler;
  }, [handler]);

  useEffect(() => {
    if (enabled && lastMessage && lastMessage.type === eventType) {
      handlerRef.current(lastMessage.data);
    }
  }, [lastMessage, eventType, enabled]);
}

// ============================================================================
// Predefined Event Hooks
// ============================================================================
//
// These hooks provide convenient type-specific event listeners for common
// enterprise WebSocket events. Each hook uses useWebSocketEvent internally
// to filter messages by event type.
//
// Event Types:
// - webhook.delivery: Webhook delivery status updates
// - security.alert: Security alerts and violations
// - schedule.event: Session schedule events (start, stop, missed)
// - node.health: Kubernetes node health changes
// - scaling.event: Auto-scaling events (scale up/down)
// - compliance.violation: Compliance policy violations
// - user.event: User creation, updates, deletions
// - group.event: Group membership changes
// - quota.event: Quota threshold warnings
// - plugin.event: Plugin install, update, uninstall events
// - template.event: Template catalog updates
// - repository.event: Repository sync status changes
// - integration.event: Third-party integration events

/** Hook for webhook delivery status updates */
export function useWebhookDeliveryEvents(handler: (data: any) => void) {
  useWebSocketEvent('webhook.delivery', handler);
}

/** Hook for security alerts and violations */
export function useSecurityAlertEvents(handler: (data: any) => void) {
  useWebSocketEvent('security.alert', handler);
}

/** Hook for session schedule events */
export function useScheduleEvents(handler: (data: any) => void) {
  useWebSocketEvent('schedule.event', handler);
}

/** Hook for node health changes */
export function useNodeHealthEvents(handler: (data: any) => void) {
  useWebSocketEvent('node.health', handler);
}

/** Hook for auto-scaling events */
export function useScalingEvents(handler: (data: any) => void) {
  useWebSocketEvent('scaling.event', handler);
}

/** Hook for compliance policy violations */
export function useComplianceViolationEvents(handler: (data: any) => void) {
  useWebSocketEvent('compliance.violation', handler);
}

/** Hook for user lifecycle events */
export function useUserEvents(handler: (data: any) => void) {
  useWebSocketEvent('user.event', handler);
}

/** Hook for group membership changes */
export function useGroupEvents(handler: (data: any) => void) {
  useWebSocketEvent('group.event', handler);
}

/** Hook for quota threshold warnings */
export function useQuotaEvents(handler: (data: any) => void) {
  useWebSocketEvent('quota.event', handler);
}

/** Hook for plugin lifecycle events */
export function usePluginEvents(handler: (data: any) => void) {
  useWebSocketEvent('plugin.event', handler);
}

/** Hook for template catalog updates */
export function useTemplateEvents(handler: (data: any) => void) {
  useWebSocketEvent('template.event', handler);
}

/** Hook for repository sync status changes */
export function useRepositoryEvents(handler: (data: any) => void) {
  useWebSocketEvent('repository.event', handler);
}

/** Hook for third-party integration events */
export function useIntegrationEvents(handler: (data: any) => void) {
  useWebSocketEvent('integration.event', handler);
}

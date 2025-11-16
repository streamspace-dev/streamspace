import { useEffect, useRef, useCallback, useState } from 'react';

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
 * Custom hook for managing enterprise WebSocket connections
 * Provides automatic reconnection, message handling, and connection status
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
    reconnectInterval = 3000,
    maxReconnectAttempts = 10,
  } = options;

  const [isConnected, setIsConnected] = useState(false);
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const shouldReconnectRef = useRef(true);

  const getWebSocketUrl = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    return `${protocol}//${host}/api/v1/ws/enterprise`;
  }, []);

  const connect = useCallback(() => {
    // Don't create multiple connections
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      return;
    }

    try {
      const token = localStorage.getItem('token');
      if (!token) {
        console.error('No authentication token found');
        return;
      }

      const wsUrl = getWebSocketUrl();
      // console.log(`[WebSocket] Connecting to ${wsUrl}`);

      wsRef.current = new WebSocket(wsUrl);

      wsRef.current.onopen = () => {
        // console.log('[WebSocket] Connected to enterprise WebSocket');
        setIsConnected(true);
        setReconnectAttempts(0);
        shouldReconnectRef.current = true;

        if (onOpen) {
          onOpen();
        }
      };

      wsRef.current.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          // console.log('[WebSocket] Received message:', message);

          setLastMessage(message);

          if (onMessage) {
            onMessage(message);
          }
        } catch (error) {
          console.error('[WebSocket] Failed to parse message:', error);
        }
      };

      wsRef.current.onerror = (error) => {
        console.error('[WebSocket] Error:', error);
        setIsConnected(false);

        if (onError) {
          onError(error);
        }
      };

      wsRef.current.onclose = () => {
        // console.log('[WebSocket] Connection closed');
        setIsConnected(false);

        if (onClose) {
          onClose();
        }

        // Attempt reconnection if enabled and within retry limit
        if (
          shouldReconnectRef.current &&
          autoReconnect &&
          reconnectAttempts < maxReconnectAttempts
        ) {
          // console.log(
          //   `[WebSocket] Attempting reconnection in ${reconnectInterval}ms (attempt ${reconnectAttempts + 1}/${maxReconnectAttempts})`
          // );

          reconnectTimeoutRef.current = setTimeout(() => {
            setReconnectAttempts((prev) => prev + 1);
            connect();
          }, reconnectInterval);
        } else if (reconnectAttempts >= maxReconnectAttempts) {
          console.error('[WebSocket] Max reconnection attempts reached');
        }
      };
    } catch (error) {
      console.error('[WebSocket] Failed to create WebSocket connection:', error);
      setIsConnected(false);
    }
  }, [
    getWebSocketUrl,
    onOpen,
    onMessage,
    onError,
    onClose,
    autoReconnect,
    reconnectInterval,
    maxReconnectAttempts,
    reconnectAttempts,
  ]);

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

  // Auto-connect on mount
  useEffect(() => {
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
  }, []); // Empty dependency array - only run on mount/unmount

  // Handle page visibility changes (reconnect when page becomes visible)
  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible' && !isConnected) {
        // console.log('[WebSocket] Page visible, attempting reconnection');
        connect();
      }
    };

    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, [isConnected, connect]);

  return {
    isConnected,
    lastMessage,
    sendMessage,
    connect,
    disconnect,
    reconnectAttempts,
  };
}

// Hook for listening to specific event types
export function useWebSocketEvent(
  eventType: string,
  handler: (data: any) => void,
  enabled = true
) {
  const { lastMessage } = useEnterpriseWebSocket({
    autoReconnect: true,
  });

  useEffect(() => {
    if (enabled && lastMessage && lastMessage.type === eventType) {
      handler(lastMessage.data);
    }
  }, [lastMessage, eventType, handler, enabled]);
}

// Predefined hooks for enterprise events
export function useWebhookDeliveryEvents(handler: (data: any) => void) {
  useWebSocketEvent('webhook.delivery', handler);
}

export function useSecurityAlertEvents(handler: (data: any) => void) {
  useWebSocketEvent('security.alert', handler);
}

export function useScheduleEvents(handler: (data: any) => void) {
  useWebSocketEvent('schedule.event', handler);
}

export function useNodeHealthEvents(handler: (data: any) => void) {
  useWebSocketEvent('node.health', handler);
}

export function useScalingEvents(handler: (data: any) => void) {
  useWebSocketEvent('scaling.event', handler);
}

export function useComplianceViolationEvents(handler: (data: any) => void) {
  useWebSocketEvent('compliance.violation', handler);
}

export function useUserEvents(handler: (data: any) => void) {
  useWebSocketEvent('user.event', handler);
}

export function useGroupEvents(handler: (data: any) => void) {
  useWebSocketEvent('group.event', handler);
}

export function useQuotaEvents(handler: (data: any) => void) {
  useWebSocketEvent('quota.event', handler);
}

export function usePluginEvents(handler: (data: any) => void) {
  useWebSocketEvent('plugin.event', handler);
}

export function useTemplateEvents(handler: (data: any) => void) {
  useWebSocketEvent('template.event', handler);
}

export function useRepositoryEvents(handler: (data: any) => void) {
  useWebSocketEvent('repository.event', handler);
}

export function useIntegrationEvents(handler: (data: any) => void) {
  useWebSocketEvent('integration.event', handler);
}

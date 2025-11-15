import { useEffect, useRef, useState, useCallback } from 'react';

interface UseWebSocketOptions {
  url: string;
  onMessage: (data: any) => void;
  onError?: (error: Event) => void;
  onOpen?: () => void;
  onClose?: () => void;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

interface UseWebSocketReturn {
  isConnected: boolean;
  reconnectAttempts: number;
  sendMessage: (message: any) => void;
  close: () => void;
}

/**
 * Custom hook for WebSocket connections with automatic reconnection
 *
 * @param options - WebSocket configuration options
 * @returns WebSocket connection state and controls
 */
export function useWebSocket({
  url,
  onMessage,
  onError,
  onOpen,
  onClose,
  reconnectInterval = 3000,
  maxReconnectAttempts = 10,
}: UseWebSocketOptions): UseWebSocketReturn {
  const [isConnected, setIsConnected] = useState(false);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const shouldReconnectRef = useRef(true);

  const connect = useCallback(() => {
    try {
      const ws = new WebSocket(url);

      ws.onopen = () => {
        // console.log(`WebSocket connected: ${url}`);
        setIsConnected(true);
        setReconnectAttempts(0);
        onOpen?.();
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessage(data);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        onError?.(error);
      };

      ws.onclose = () => {
        // console.log(`WebSocket closed: ${url}`);
        setIsConnected(false);
        onClose?.();

        // Attempt reconnection with exponential backoff
        if (shouldReconnectRef.current && reconnectAttempts < maxReconnectAttempts) {
          const delay = Math.min(reconnectInterval * Math.pow(1.5, reconnectAttempts), 30000);
          // console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts + 1}/${maxReconnectAttempts})`);

          reconnectTimeoutRef.current = setTimeout(() => {
            setReconnectAttempts((prev) => prev + 1);
            connect();
          }, delay);
        } else if (reconnectAttempts >= maxReconnectAttempts) {
          console.error(`Max reconnection attempts (${maxReconnectAttempts}) reached for ${url}`);
        }
      };

      wsRef.current = ws;
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
    }
  }, [url, onMessage, onError, onOpen, onClose, reconnectInterval, maxReconnectAttempts, reconnectAttempts]);

  const sendMessage = useCallback((message: any) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
    } else {
      console.warn('WebSocket is not connected. Message not sent:', message);
    }
  }, []);

  const close = useCallback(() => {
    shouldReconnectRef.current = false;

    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  useEffect(() => {
    shouldReconnectRef.current = true;
    connect();

    return () => {
      close();
    };
  }, [connect, close]);

  return {
    isConnected,
    reconnectAttempts,
    sendMessage,
    close,
  };
}

/**
 * Hook for subscribing to session updates via WebSocket
 */
export function useSessionsWebSocket(onUpdate: (sessions: any[]) => void) {
  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8000';
  const wsUrl = apiUrl.replace(/^http/, 'ws') + '/api/v1/ws/sessions';

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      if (data.type === 'sessions_update' && data.sessions) {
        onUpdate(data.sessions);
      }
    },
    // onOpen: () => console.log('Sessions WebSocket connected'),
    // onClose: () => console.log('Sessions WebSocket disconnected'),
  });
}

/**
 * Hook for subscribing to cluster metrics via WebSocket
 */
export function useMetricsWebSocket(onUpdate: (metrics: any) => void) {
  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8000';
  const wsUrl = apiUrl.replace(/^http/, 'ws') + '/api/v1/ws/cluster';

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      if (data.type === 'metrics_update' && data.metrics) {
        onUpdate(data.metrics);
      }
    },
    // onOpen: () => console.log('Metrics WebSocket connected'),
    // onClose: () => console.log('Metrics WebSocket disconnected'),
  });
}

/**
 * Hook for subscribing to pod logs via WebSocket
 */
export function useLogsWebSocket(namespace: string, podName: string, onLog: (log: string) => void) {
  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8000';
  const wsUrl = apiUrl.replace(/^http/, 'ws') + `/api/v1/ws/logs/${namespace}/${podName}`;

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      if (typeof data === 'string') {
        onLog(data);
      }
    },
    // onOpen: () => console.log(`Logs WebSocket connected: ${namespace}/${podName}`),
    // onClose: () => console.log(`Logs WebSocket disconnected: ${namespace}/${podName}`),
  });
}

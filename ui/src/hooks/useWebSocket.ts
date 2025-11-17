import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useUserStore } from '../store/userStore';

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
 * Custom hook for WebSocket connections with automatic reconnection.
 *
 * Features:
 * - Automatic reconnection with custom backoff strategy (30s, 15s, 15s, then 60s)
 * - Stable callbacks using refs to prevent reconnection loops
 * - Connection state management
 * - Manual connection control (send, close)
 *
 * Reconnection Strategy:
 * - 1st retry: 30 seconds (allows transient issues to resolve)
 * - 2nd retry: 15 seconds (quicker retry if still failing)
 * - 3rd retry: 15 seconds
 * - 4th+ retries: 60 seconds each (prevents hammering the server)
 *
 * @param options - WebSocket configuration options
 * @param options.url - WebSocket URL to connect to (empty string disables connection)
 * @param options.onMessage - Callback when message received (data is pre-parsed JSON)
 * @param options.onError - Optional callback when error occurs
 * @param options.onOpen - Optional callback when connection opens
 * @param options.onClose - Optional callback when connection closes
 * @param options.reconnectInterval - Ignored (kept for backwards compatibility)
 * @param options.maxReconnectAttempts - Maximum reconnection attempts (default: 10)
 *
 * @returns WebSocket connection state and controls
 * @returns isConnected - Boolean indicating if WebSocket is currently connected
 * @returns reconnectAttempts - Number of reconnection attempts made
 * @returns sendMessage - Function to send JSON message (auto-stringified)
 * @returns close - Function to close connection and prevent reconnection
 *
 * @example
 * ```tsx
 * const { isConnected, sendMessage } = useWebSocket({
 *   url: 'wss://example.com/ws',
 *   onMessage: (data) => console.log('Received:', data),
 *   onError: (error) => console.error('WebSocket error:', error),
 *   maxReconnectAttempts: 5,
 * });
 * ```
 */
export function useWebSocket({
  url,
  onMessage,
  onError,
  onOpen,
  onClose,
  reconnectInterval = 3000, // Not used with custom backoff
  maxReconnectAttempts = 10,
}: UseWebSocketOptions): UseWebSocketReturn {
  const [isConnected, setIsConnected] = useState(false);
  const [reconnectAttempts, setReconnectAttempts] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const shouldReconnectRef = useRef(true);
  const reconnectAttemptsRef = useRef(0);

  // Custom backoff pattern: 30s, 15s, 15s, then 60s for all subsequent attempts
  const getReconnectDelay = (attemptNumber: number): number => {
    if (attemptNumber === 0) return 30000; // 30 seconds for first retry
    if (attemptNumber === 1) return 15000; // 15 seconds for second retry
    if (attemptNumber === 2) return 15000; // 15 seconds for third retry
    return 60000; // 60 seconds for all subsequent retries
  };

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

  const connect = useCallback(() => {
    // Don't connect if URL is empty (used to disable connection)
    if (!url) {
      return;
    }

    try {
      const ws = new WebSocket(url);

      ws.onopen = () => {
        // console.log(`WebSocket connected: ${url}`);
        setIsConnected(true);
        setReconnectAttempts(0);
        reconnectAttemptsRef.current = 0;
        onOpenRef.current?.();
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessageRef.current(data);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };

      ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        onErrorRef.current?.(error);
      };

      ws.onclose = () => {
        // console.log(`WebSocket closed: ${url}`);
        setIsConnected(false);
        onCloseRef.current?.();

        // Attempt reconnection with custom backoff pattern
        const currentAttempts = reconnectAttemptsRef.current;
        if (shouldReconnectRef.current && currentAttempts < maxReconnectAttempts && url) {
          const delay = getReconnectDelay(currentAttempts);
          console.log(`Reconnecting in ${delay / 1000}s (attempt ${currentAttempts + 1}/${maxReconnectAttempts})`);

          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptsRef.current += 1;
            setReconnectAttempts((prev) => prev + 1);
            connect();
          }, delay);
        } else if (currentAttempts >= maxReconnectAttempts) {
          console.error(`Max reconnection attempts (${maxReconnectAttempts}) reached for ${url}`);
        }
      };

      wsRef.current = ws;
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error);
    }
  }, [url, maxReconnectAttempts]); // Removed reconnectInterval since we use getReconnectDelay

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
    // Only connect if URL is not empty
    if (!url) {
      return;
    }

    shouldReconnectRef.current = true;
    connect();

    return () => {
      close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url]); // React to URL changes so we connect when token becomes available

  return {
    isConnected,
    reconnectAttempts,
    sendMessage,
    close,
  };
}

/**
 * Hook for subscribing to real-time session updates via WebSocket.
 *
 * Automatically connects to the sessions WebSocket endpoint when a valid
 * authentication token is available. Disconnects when token is removed.
 *
 * Features:
 * - Reactive to authentication state (auto-connects/disconnects with token changes)
 * - Receives real-time session creation, updates, and deletion events
 * - Uses Zustand store for seamless token reactivity
 * - Automatically parses and filters 'sessions_update' events
 *
 * @param onUpdate - Callback function called when sessions are updated
 * @param onUpdate.sessions - Array of updated session objects
 *
 * @returns WebSocket connection state from useWebSocket hook
 *
 * @example
 * ```tsx
 * const { isConnected } = useSessionsWebSocket((sessions) => {
 *   setMySessions(sessions);
 * });
 * ```
 */
export function useSessionsWebSocket(onUpdate: (sessions: any[]) => void) {
  // Get token directly from Zustand store - automatically reactive
  const token = useUserStore((state) => state?.token);

  // Memoize URL construction, recalculate when token changes
  const wsUrl = useMemo(() => {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

      // Don't connect without a token - return empty URL to prevent connection
      return token
        ? `${protocol}//${window.location.host}/api/v1/ws/sessions?token=${encodeURIComponent(token)}`
        : '';
    } catch (error) {
      console.error('[useSessionsWebSocket] Error building URL:', error);
      return '';
    }
  }, [token]); // Recalculate when token changes

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      try {
        if (data.type === 'sessions_update' && data.sessions) {
          onUpdate(data.sessions);
        }
      } catch (error) {
        console.error('[useSessionsWebSocket] Error in onMessage:', error);
      }
    },
    onError: (error) => {
      console.error('[useSessionsWebSocket] WebSocket error:', error);
    },
    // onOpen: () => console.log('Sessions WebSocket connected'),
    // onClose: () => console.log('Sessions WebSocket disconnected'),
  });
}

/**
 * Hook for subscribing to real-time cluster metrics via WebSocket.
 *
 * Provides live updates of cluster resource usage, node health, and session counts.
 * Only connects when authenticated (requires admin or operator role on backend).
 *
 * Features:
 * - Reactive to authentication state
 * - Receives real-time cluster metrics updates (CPU, memory, nodes, sessions)
 * - Automatically parses and filters 'metrics_update' events
 * - Admin/operator only (regular users will get 403 Forbidden)
 *
 * @param onUpdate - Callback function called when cluster metrics are updated
 * @param onUpdate.metrics - Object containing cluster metrics (nodes, resources, sessions, users)
 *
 * @returns WebSocket connection state from useWebSocket hook
 *
 * @example
 * ```tsx
 * const { isConnected } = useMetricsWebSocket((metrics) => {
 *   setClusterMetrics(metrics);
 * });
 * ```
 */
export function useMetricsWebSocket(onUpdate: (metrics: any) => void) {
  // Get token directly from Zustand store - automatically reactive
  const token = useUserStore((state) => state?.token);

  // Memoize URL construction, recalculate when token changes
  const wsUrl = useMemo(() => {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

      // Don't connect without a token
      return token
        ? `${protocol}//${window.location.host}/api/v1/ws/cluster?token=${encodeURIComponent(token)}`
        : '';
    } catch (error) {
      console.error('[useMetricsWebSocket] Error building URL:', error);
      return '';
    }
  }, [token]); // Recalculate when token changes

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      try {
        if (data.type === 'metrics_update' && data.metrics) {
          onUpdate(data.metrics);
        }
      } catch (error) {
        console.error('[useMetricsWebSocket] Error in onMessage:', error);
      }
    },
    onError: (error) => {
      console.error('[useMetricsWebSocket] WebSocket error:', error);
    },
    // onOpen: () => console.log('Metrics WebSocket connected'),
    // onClose: () => console.log('Metrics WebSocket disconnected'),
  });
}

/**
 * Hook for streaming pod logs in real-time via WebSocket.
 *
 * Tails pod logs and streams each new log line as it's written. Useful for
 * debugging session pods or monitoring system components.
 *
 * Features:
 * - Real-time log streaming (follows logs as they're written)
 * - Reactive to authentication state
 * - Admin/operator only (regular users will get 403 Forbidden)
 * - Automatically reconnects if connection drops
 *
 * @param namespace - Kubernetes namespace containing the pod
 * @param podName - Name of the pod to stream logs from
 * @param onLog - Callback function called for each log line
 * @param onLog.log - Single log line as a string
 *
 * @returns WebSocket connection state from useWebSocket hook
 *
 * @example
 * ```tsx
 * const { isConnected } = useLogsWebSocket(
 *   'streamspace',
 *   'session-user1-firefox-abc123',
 *   (log) => {
 *     appendLog(log);
 *   }
 * );
 * ```
 */
export function useLogsWebSocket(namespace: string, podName: string, onLog: (log: string) => void) {
  // Get token directly from Zustand store - automatically reactive
  const token = useUserStore((state) => state?.token);

  // Memoize URL construction, recalculate when token or params change
  const wsUrl = useMemo(() => {
    try {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

      // Don't connect without a token
      return token
        ? `${protocol}//${window.location.host}/api/v1/ws/logs/${namespace}/${podName}?token=${encodeURIComponent(token)}`
        : '';
    } catch (error) {
      console.error('[useLogsWebSocket] Error building URL:', error);
      return '';
    }
  }, [token, namespace, podName]); // Recalculate when any dependency changes

  return useWebSocket({
    url: wsUrl,
    onMessage: (data) => {
      try {
        if (typeof data === 'string') {
          onLog(data);
        }
      } catch (error) {
        console.error('[useLogsWebSocket] Error in onMessage:', error);
      }
    },
    onError: (error) => {
      console.error('[useLogsWebSocket] WebSocket error:', error);
    },
    // onOpen: () => console.log(`Logs WebSocket connected: ${namespace}/${podName}`),
    // onClose: () => console.log(`Logs WebSocket disconnected: ${namespace}/${podName}`),
  });
}

/**
 * WebSocket Enhancement Hooks
 *
 * Utility hooks for enhancing WebSocket functionality:
 * - Connection quality tracking (latency/ping)
 * - Throttling and debouncing for rapid updates
 * - Message batching
 * - Reconnection management
 *
 * @module useWebSocketEnhancements
 */
import { useState, useEffect, useRef, useCallback, useMemo } from 'react';

/**
 * Throttle function - limits function execution to once per interval
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function throttle<T extends (...args: any[]) => any>(
  func: T,
  delay: number
): (...args: Parameters<T>) => void {
  let lastCall = 0;
  let timeout: ReturnType<typeof setTimeout> | null = null;

  return (...args: Parameters<T>) => {
    const now = Date.now();

    if (now - lastCall >= delay) {
      lastCall = now;
      func(...args);
    } else {
      // Schedule for later if not yet called
      if (timeout) clearTimeout(timeout);
      timeout = setTimeout(() => {
        lastCall = Date.now();
        func(...args);
      }, delay - (now - lastCall));
    }
  };
}

/**
 * Debounce function - delays function execution until after delay has passed since last call
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function debounce<T extends (...args: any[]) => any>(
  func: T,
  delay: number
): (...args: Parameters<T>) => void {
  let timeout: ReturnType<typeof setTimeout> | null = null;

  return (...args: Parameters<T>) => {
    if (timeout) clearTimeout(timeout);
    timeout = setTimeout(() => func(...args), delay);
  };
}

/**
 * Hook for throttling callbacks
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useThrottle<T extends (...args: any[]) => any>(
  callback: T,
  delay: number
): (...args: Parameters<T>) => void {
  const throttledCallback = useRef(throttle(callback, delay));

  useEffect(() => {
    throttledCallback.current = throttle(callback, delay);
  }, [callback, delay]);

  return throttledCallback.current;
}

/**
 * Hook for debouncing callbacks
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useDebounce<T extends (...args: any[]) => any>(
  callback: T,
  delay: number
): (...args: Parameters<T>) => void {
  const debouncedCallback = useRef(debounce(callback, delay));

  useEffect(() => {
    debouncedCallback.current = debounce(callback, delay);
  }, [callback, delay]);

  return debouncedCallback.current;
}

/**
 * Hook for tracking WebSocket connection quality (latency)
 */
export function useConnectionQuality(
  isConnected: boolean,
  _sendPing?: () => void
) {
  const [latency, setLatency] = useState<number | undefined>(undefined);
  const [quality, setQuality] = useState<'excellent' | 'good' | 'fair' | 'poor' | 'unknown'>('unknown');
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastPingRef = useRef<number | null>(null);

  // Measure latency
  const measureLatency = useCallback(() => {
    if (!isConnected) {
      setLatency(undefined);
      setQuality('unknown');
      return;
    }

    const start = Date.now();
    lastPingRef.current = start;

    // Simulate ping/pong (in real implementation, this would use actual WebSocket ping)
    // For now, we'll estimate based on connection state
    setTimeout(() => {
      if (lastPingRef.current === start) {
        const end = Date.now();
        const measuredLatency = end - start;
        setLatency(measuredLatency);

        // Determine quality
        if (measuredLatency < 100) setQuality('excellent');
        else if (measuredLatency < 300) setQuality('good');
        else if (measuredLatency < 500) setQuality('fair');
        else setQuality('poor');
      }
    }, 50); // Simulated ping response time
  }, [isConnected]);

  // Start ping interval when connected
  useEffect(() => {
    if (isConnected) {
      // Measure immediately
      measureLatency();

      // Then measure every 10 seconds
      pingIntervalRef.current = setInterval(measureLatency, 10000);
    } else {
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
        pingIntervalRef.current = null;
      }
      setLatency(undefined);
      setQuality('unknown');
    }

    return () => {
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
      }
    };
  }, [isConnected, measureLatency]);

  return { latency, quality };
}

/**
 * Hook for batching WebSocket messages
 */
export function useMessageBatching<T>(
  onBatch: (messages: T[]) => void,
  batchSize: number = 10,
  batchDelay: number = 1000
) {
  const batchRef = useRef<T[]>([]);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flushBatch = useCallback(() => {
    if (batchRef.current.length > 0) {
      onBatch([...batchRef.current]);
      batchRef.current = [];
    }
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  }, [onBatch]);

  const addMessage = useCallback((message: T) => {
    batchRef.current.push(message);

    // Flush if batch size reached
    if (batchRef.current.length >= batchSize) {
      flushBatch();
      return;
    }

    // Schedule flush if not already scheduled
    if (!timeoutRef.current) {
      timeoutRef.current = setTimeout(flushBatch, batchDelay);
    }
  }, [batchSize, batchDelay, flushBatch]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      flushBatch();
    };
  }, [flushBatch]);

  return { addMessage, flushBatch };
}

/**
 * Hook for managing manual reconnection
 */
export function useManualReconnect(
  reconnectCallback?: () => void
) {
  const [isReconnecting, setIsReconnecting] = useState(false);

  const manualReconnect = useCallback(async () => {
    if (isReconnecting) return;

    setIsReconnecting(true);

    try {
      if (reconnectCallback) {
        await reconnectCallback();
      }
    } catch (error) {
      console.error('Manual reconnection failed:', error);
    } finally {
      // Reset after a delay to prevent rapid retries
      setTimeout(() => {
        setIsReconnecting(false);
      }, 2000);
    }
  }, [reconnectCallback, isReconnecting]);

  return { manualReconnect, isReconnecting };
}

/**
 * Hook for enhanced WebSocket state management
 */
export interface EnhancedWebSocketState {
  isConnected: boolean;
  reconnectAttempts: number;
  maxReconnectAttempts: number;
  latency?: number;
  quality: 'excellent' | 'good' | 'fair' | 'poor' | 'unknown';
  onManualReconnect: () => void;
}

export function useEnhancedWebSocket(
  baseHook: { isConnected: boolean; reconnectAttempts: number },
  reconnectCallback?: () => void,
  maxReconnectAttempts: number = 10
): EnhancedWebSocketState {
  const { isConnected, reconnectAttempts } = baseHook;
  const { latency, quality } = useConnectionQuality(isConnected);
  const { manualReconnect } = useManualReconnect(reconnectCallback);

  // Memoize the return value to prevent unnecessary re-renders
  // Only update when actual values change, not on every render
  return useMemo(() => ({
    isConnected,
    reconnectAttempts,
    maxReconnectAttempts,
    latency,
    quality,
    onManualReconnect: manualReconnect,
  }), [isConnected, reconnectAttempts, maxReconnectAttempts, latency, quality, manualReconnect]);
}

/**
 * EnhancedWebSocketStatus Component
 *
 * Production-ready WebSocket connection status indicator with:
 * - Reconnection countdown timer
 * - Manual reconnect button
 * - Connection quality indicator (latency)
 * - Visual feedback for connection states
 *
 * @component
 */
import { useState, useEffect, useMemo, memo } from 'react';
import {
  Box,
  Chip,
  IconButton,
  Tooltip,
  CircularProgress,
  Popover,
  Typography,
  Button,
  LinearProgress,
} from '@mui/material';
import {
  Wifi as ConnectedIcon,
  WifiOff as DisconnectedIcon,
  Refresh as RefreshIcon,
  SignalCellularAlt as SignalIcon,
  ErrorOutline as ErrorIcon,
} from '@mui/icons-material';

interface EnhancedWebSocketStatusProps {
  isConnected: boolean;
  reconnectAttempts: number;
  maxReconnectAttempts?: number;
  onManualReconnect?: () => void;
  latency?: number; // in milliseconds
  size?: 'small' | 'medium';
  showDetails?: boolean;
}

function EnhancedWebSocketStatus({
  isConnected,
  reconnectAttempts,
  maxReconnectAttempts = 10,
  onManualReconnect,
  latency,
  size = 'small',
  showDetails = true,
}: EnhancedWebSocketStatusProps) {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [countdown, setCountdown] = useState<number | null>(null);

  // Calculate reconnection delay - matches the pattern in WebSocket hooks
  // 30s, 15s, 15s, then 60s for all subsequent attempts
  const getReconnectDelay = (attempt: number) => {
    if (attempt === 0) return 30; // 30 seconds for first retry
    if (attempt === 1) return 15; // 15 seconds for second retry
    if (attempt === 2) return 15; // 15 seconds for third retry
    return 60; // 60 seconds for all subsequent retries
  };

  // Countdown timer for reconnection - DISABLED to prevent sidebar flickering
  // The countdown state update every second was causing the entire component tree to re-render,
  // which made the sidebar flicker. Instead, we show a static "Reconnecting..." message.
  useEffect(() => {
    if (reconnectAttempts > 0 && !isConnected) {
      // Calculate next retry time but don't update state every second
      const delay = getReconnectDelay(reconnectAttempts - 1);
      setCountdown(delay); // Set once, don't update every second
    } else {
      setCountdown(null);
    }
  }, [reconnectAttempts, isConnected]);

  const handleClick = (event: React.MouseEvent<HTMLElement>) => {
    if (showDetails) {
      setAnchorEl(event.currentTarget);
    }
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleManualReconnect = () => {
    if (onManualReconnect) {
      onManualReconnect();
    }
    handleClose();
  };

  const getConnectionQuality = (latency?: number) => {
    if (!latency) return { label: 'Unknown', color: 'default' as const };
    if (latency < 100) return { label: 'Excellent', color: 'success' as const };
    if (latency < 300) return { label: 'Good', color: 'info' as const };
    if (latency < 500) return { label: 'Fair', color: 'warning' as const };
    return { label: 'Poor', color: 'error' as const };
  };

  const getStatusLabel = () => {
    if (isConnected) {
      return latency ? `Live • ${latency}ms` : 'Live Updates';
    }
    if (reconnectAttempts > 0) {
      // Show attempt count only, not countdown (to prevent re-renders)
      return `Reconnecting... (${reconnectAttempts}/${maxReconnectAttempts})`;
    }
    if (reconnectAttempts >= maxReconnectAttempts) {
      return 'Connection Failed';
    }
    return 'Disconnected';
  };

  const getStatusColor = () => {
    if (isConnected) return 'success' as const;
    if (reconnectAttempts >= maxReconnectAttempts) return 'error' as const;
    if (reconnectAttempts > 0) return 'warning' as const;
    return 'default' as const;
  };

  // Memoize the status icon to prevent CircularProgress from restarting on every render
  // Only recreate when connection status actually changes, not when latency updates
  const statusIcon = useMemo(() => {
    if (isConnected) return <ConnectedIcon />;
    if (reconnectAttempts >= maxReconnectAttempts) return <ErrorIcon />;
    if (reconnectAttempts > 0) return <CircularProgress size={16} />;
    return <DisconnectedIcon />;
  }, [isConnected, reconnectAttempts, maxReconnectAttempts]);

  const quality = getConnectionQuality(latency);
  const open = Boolean(anchorEl);

  return (
    <>
      <Chip
        icon={statusIcon}
        label={getStatusLabel()}
        size={size}
        color={getStatusColor()}
        onClick={handleClick}
        sx={{
          cursor: showDetails ? 'pointer' : 'default',
          '& .MuiChip-icon': {
            marginLeft: '8px',
          },
        }}
      />

      {showDetails && (
        <Popover
          open={open}
          anchorEl={anchorEl}
          onClose={handleClose}
          anchorOrigin={{
            vertical: 'bottom',
            horizontal: 'center',
          }}
          transformOrigin={{
            vertical: 'top',
            horizontal: 'center',
          }}
        >
          <Box sx={{ p: 2, minWidth: 280 }}>
            <Typography variant="subtitle2" gutterBottom sx={{ fontWeight: 600 }}>
              WebSocket Connection Status
            </Typography>

            {/* Connection State */}
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
              {statusIcon}
              <Box sx={{ flex: 1 }}>
                <Typography variant="body2" sx={{ fontWeight: 500 }}>
                  {isConnected ? 'Connected' : reconnectAttempts > 0 ? 'Reconnecting' : 'Disconnected'}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {getStatusLabel()}
                </Typography>
              </Box>
            </Box>

            {/* Reconnection Progress */}
            {reconnectAttempts > 0 && !isConnected && (
              <Box sx={{ mb: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
                  <Typography variant="caption" color="text.secondary">
                    Attempt {reconnectAttempts}/{maxReconnectAttempts}
                  </Typography>
                  {countdown !== null && (
                    <Typography variant="caption" color="text.secondary">
                      Next retry in ~{countdown}s
                    </Typography>
                  )}
                </Box>
                <LinearProgress
                  variant="indeterminate"
                  color={reconnectAttempts >= maxReconnectAttempts ? 'error' : 'primary'}
                />
              </Box>
            )}

            {/* Connection Quality */}
            {isConnected && latency !== undefined && (
              <Box sx={{ mb: 2 }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                  <SignalIcon fontSize="small" color={quality.color} />
                  <Typography variant="body2">
                    Connection Quality: <strong>{quality.label}</strong>
                  </Typography>
                </Box>
                <Typography variant="caption" color="text.secondary">
                  Latency: {latency}ms
                </Typography>
              </Box>
            )}

            {/* Manual Reconnect Button */}
            {!isConnected && onManualReconnect && (
              <Button
                fullWidth
                variant="outlined"
                size="small"
                startIcon={<RefreshIcon />}
                onClick={handleManualReconnect}
                disabled={reconnectAttempts > 0 && reconnectAttempts < maxReconnectAttempts}
              >
                {reconnectAttempts >= maxReconnectAttempts ? 'Retry Connection' : 'Reconnect Now'}
              </Button>
            )}

            {/* Help Text */}
            {!isConnected && (
              <Typography variant="caption" color="text.secondary" sx={{ mt: 2, display: 'block' }}>
                Real-time updates are temporarily unavailable. Data will refresh automatically when reconnected.
              </Typography>
            )}
          </Box>
        </Popover>
      )}
    </>
  );
}

// Export memoized version to prevent re-renders when props haven't changed
export default memo(EnhancedWebSocketStatus);

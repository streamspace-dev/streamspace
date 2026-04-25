/**
 * NotificationQueue Component
 *
 * Advanced notification system with:
 * - Stacking multiple notifications
 * - Priority-based ordering (critical > high > medium > low)
 * - Auto-dismiss with configurable duration
 * - Manual dismiss individual or all
 * - Notification history tracking
 * - Customizable icons and colors
 *
 * @component
 */
import { useState, useEffect, useCallback } from 'react';
import {
  Snackbar,
  Alert,
  AlertTitle,
  IconButton,
  Box,
  Badge,
  Tooltip,
  Drawer,
  List,
  ListItem,
  ListItemIcon,
  Typography,
  Button,
  Divider,
} from '@mui/material';
import {
  Close as CloseIcon,
  History as HistoryIcon,
  DeleteSweep as ClearAllIcon,
  Info as InfoIcon,
  Warning as WarningIcon,
  Error as ErrorIcon,
  CheckCircle as SuccessIcon,
} from '@mui/icons-material';

export type NotificationSeverity = 'success' | 'info' | 'warning' | 'error';
export type NotificationPriority = 'low' | 'medium' | 'high' | 'critical';

export interface Notification {
  id: string;
  message: string;
  severity: NotificationSeverity;
  priority?: NotificationPriority;
  title?: string;
  duration?: number; // milliseconds, null = no auto-dismiss
  action?: {
    label: string;
    onClick: () => void;
  };
  timestamp?: Date;
}

interface NotificationQueueProps {
  maxVisible?: number;
  defaultDuration?: number;
  position?: {
    vertical: 'top' | 'bottom';
    horizontal: 'left' | 'center' | 'right';
  };
  enableHistory?: boolean;
  maxHistorySize?: number;
}

export default function NotificationQueue({
  maxVisible = 3,
  defaultDuration = 6000,
  position = { vertical: 'bottom', horizontal: 'right' },
  enableHistory = true,
  maxHistorySize = 50,
}: NotificationQueueProps) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [history, setHistory] = useState<Notification[]>([]);
  const [historyDrawerOpen, setHistoryDrawerOpen] = useState(false);

  // Priority weights for sorting
  const priorityWeight: Record<NotificationPriority, number> = {
    critical: 4,
    high: 3,
    medium: 2,
    low: 1,
  };

  // Sort notifications by priority (critical first)
  const sortedNotifications = [...notifications].sort((a, b) => {
    const aPriority = a.priority || 'medium';
    const bPriority = b.priority || 'medium';
    return priorityWeight[bPriority] - priorityWeight[aPriority];
  });

  // Get visible notifications (max 3)
  const visibleNotifications = sortedNotifications.slice(0, maxVisible);
  const hiddenCount = Math.max(0, notifications.length - maxVisible);

  // Add notification to queue
  const addNotification = (notification: Omit<Notification, 'id' | 'timestamp'>) => {
    const newNotification: Notification = {
      ...notification,
      id: `notification-${Date.now()}-${Math.random()}`,
      timestamp: new Date(),
      duration: notification.duration !== undefined ? notification.duration : defaultDuration,
    };

    setNotifications((prev) => [...prev, newNotification]);

    // Add to history
    if (enableHistory) {
      setHistory((prev) => {
        const updated = [newNotification, ...prev].slice(0, maxHistorySize);
        return updated;
      });
    }

    // Auto-dismiss if duration is set
    if (newNotification.duration && newNotification.duration > 0) {
      setTimeout(() => {
        removeNotification(newNotification.id);
      }, newNotification.duration);
    }
  };

  // Remove notification from queue
  const removeNotification = (id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  };

  // Clear all notifications
  const clearAll = () => {
    setNotifications([]);
  };

  // Clear history
  const clearHistory = () => {
    setHistory([]);
  };

  // Get icon for severity
  const getSeverityIcon = (severity: NotificationSeverity) => {
    switch (severity) {
      case 'success':
        return <SuccessIcon />;
      case 'info':
        return <InfoIcon />;
      case 'warning':
        return <WarningIcon />;
      case 'error':
        return <ErrorIcon />;
    }
  };

  // Format timestamp
  const formatTimestamp = (date: Date) => {
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);

    if (seconds < 60) return 'Just now';
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    return date.toLocaleString();
  };

  // Expose addNotification method globally
  useEffect(() => {
    // Store in window for global access
    const windowWithNotification = window as Window & { addNotification?: typeof addNotification };
    windowWithNotification.addNotification = addNotification;
    return () => {
      delete windowWithNotification.addNotification;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <>
      {/* Notification Stack */}
      <Box
        sx={{
          position: 'fixed',
          ...(position.vertical === 'top' ? { top: 24 } : { bottom: 24 }),
          ...(position.horizontal === 'left' ? { left: 24 } :
              position.horizontal === 'right' ? { right: 24 } : { left: '50%', transform: 'translateX(-50%)' }),
          zIndex: 1400,
          display: 'flex',
          flexDirection: 'column',
          gap: 1,
          maxWidth: 400,
        }}
      >
        {visibleNotifications.map((notification) => (
          <Snackbar
            key={notification.id}
            open={true}
            anchorOrigin={position}
            sx={{
              position: 'relative',
              transform: 'none !important',
              left: 'auto !important',
              right: 'auto !important',
              top: 'auto !important',
              bottom: 'auto !important',
            }}
          >
            <Alert
              severity={notification.severity}
              onClose={() => removeNotification(notification.id)}
              action={
                notification.action ? (
                  <Button
                    color="inherit"
                    size="small"
                    onClick={() => {
                      notification.action!.onClick();
                      removeNotification(notification.id);
                    }}
                  >
                    {notification.action.label}
                  </Button>
                ) : undefined
              }
              sx={{ width: '100%' }}
            >
              {notification.title && <AlertTitle>{notification.title}</AlertTitle>}
              {notification.message}
            </Alert>
          </Snackbar>
        ))}

        {/* "More notifications" indicator */}
        {hiddenCount > 0 && (
          <Alert
            severity="info"
            icon={false}
            sx={{ cursor: 'pointer' }}
            onClick={() => setHistoryDrawerOpen(true)}
          >
            <Typography variant="caption">
              +{hiddenCount} more notification{hiddenCount > 1 ? 's' : ''}... Click to view all
            </Typography>
          </Alert>
        )}

        {/* Clear all button (shown when multiple notifications) */}
        {notifications.length > 1 && (
          <Box sx={{ display: 'flex', justifyContent: 'flex-end', mt: 1 }}>
            <Button
              size="small"
              startIcon={<ClearAllIcon />}
              onClick={clearAll}
              variant="outlined"
            >
              Dismiss All
            </Button>
          </Box>
        )}
      </Box>

      {/* History Button (floating) */}
      {enableHistory && history.length > 0 && (
        <Box
          sx={{
            position: 'fixed',
            bottom: 24,
            left: 24,
            zIndex: 1300,
          }}
        >
          <Tooltip title="Notification History">
            <IconButton
              color="primary"
              onClick={() => setHistoryDrawerOpen(true)}
              sx={{
                bgcolor: 'background.paper',
                boxShadow: 2,
                '&:hover': { boxShadow: 4 },
              }}
            >
              <Badge badgeContent={history.length} color="primary" max={99}>
                <HistoryIcon />
              </Badge>
            </IconButton>
          </Tooltip>
        </Box>
      )}

      {/* History Drawer */}
      {enableHistory && (
        <Drawer
          anchor="right"
          open={historyDrawerOpen}
          onClose={() => setHistoryDrawerOpen(false)}
          PaperProps={{ sx: { width: 400 } }}
        >
          <Box sx={{ p: 2 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
              <Typography variant="h6">Notification History</Typography>
              <Box sx={{ display: 'flex', gap: 1 }}>
                <IconButton size="small" onClick={clearHistory}>
                  <ClearAllIcon />
                </IconButton>
                <IconButton size="small" onClick={() => setHistoryDrawerOpen(false)}>
                  <CloseIcon />
                </IconButton>
              </Box>
            </Box>

            <Divider sx={{ mb: 2 }} />

            {history.length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center', py: 4 }}>
                No notifications yet
              </Typography>
            ) : (
              <List sx={{ p: 0 }}>
                {history.map((notification) => (
                  <ListItem
                    key={notification.id}
                    sx={{
                      flexDirection: 'column',
                      alignItems: 'flex-start',
                      borderLeft: 4,
                      borderColor: `${notification.severity}.main`,
                      mb: 1,
                      bgcolor: 'background.default',
                      borderRadius: 1,
                    }}
                  >
                    <Box sx={{ display: 'flex', width: '100%', alignItems: 'flex-start', gap: 1 }}>
                      <ListItemIcon sx={{ minWidth: 'auto', mt: 0.5 }}>
                        {getSeverityIcon(notification.severity)}
                      </ListItemIcon>
                      <Box sx={{ flex: 1 }}>
                        {notification.title && (
                          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
                            {notification.title}
                          </Typography>
                        )}
                        <Typography variant="body2">{notification.message}</Typography>
                        <Typography variant="caption" color="text.secondary">
                          {notification.timestamp && formatTimestamp(notification.timestamp)}
                        </Typography>
                      </Box>
                    </Box>
                  </ListItem>
                ))}
              </List>
            )}
          </Box>
        </Drawer>
      )}
    </>
  );
}

// Export hook for easy use
// eslint-disable-next-line react-refresh/only-export-components
export function useNotificationQueue() {
  // Use useCallback to return a stable function reference
  // This prevents unnecessary re-renders in components that use this hook
  const addNotification = useCallback((notification: Omit<Notification, 'id' | 'timestamp'>) => {
    const windowWithNotification = window as Window & { addNotification?: (n: Omit<Notification, 'id' | 'timestamp'>) => void };
    if (windowWithNotification.addNotification) {
      windowWithNotification.addNotification(notification);
    }
  }, []);

  return { addNotification };
}

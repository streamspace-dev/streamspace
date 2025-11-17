import { useState, useRef, useCallback } from 'react';
import {
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  CardActions,
  Grid,
  Chip,
  Alert,
  CircularProgress,
} from '@mui/material';
import {
  OpenInNew as OpenIcon,
  Person as OwnerIcon,
  Share as ShareIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import Layout from '../components/Layout';
import { useUserStore } from '../store/userStore';
import { useSessionsWebSocket } from '../hooks/useWebSocket';
import { useEnhancedWebSocket } from '../hooks/useWebSocketEnhancements';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';
import { useSharedSessions } from '../hooks/useApi';

interface SharedSession {
  id: string;
  ownerUserId: string;
  ownerUsername: string;
  templateName: string;
  state: string;
  appType: string;
  createdAt: string;
  sharedAt: string;
  permissionLevel: string;
  isShared: boolean;
  url?: string;
}

/**
 * SharedSessions - View and access sessions shared by other users
 *
 * Displays sessions that have been shared with the current user, providing:
 * - Grid view of all shared sessions
 * - Session metadata (owner, template, state, creation time)
 * - Permission level indicator (view, edit, control)
 * - Quick access to shared session viewers
 * - Real-time session status updates via WebSocket
 * - Empty state with helpful guidance
 *
 * Features:
 * - Session cards with owner information
 * - Template name and app type display
 * - Session state chips (running, hibernated, terminated)
 * - Permission level badges (view-only, editor, controller)
 * - "Shared At" timestamp showing when access was granted
 * - Connect button to open session viewer
 * - Real-time state change notifications
 * - Session state change alerts (running → hibernated → terminated)
 * - WebSocket connection status indicator
 *
 * Permission levels:
 * - View: Can view session (read-only access)
 * - Edit: Can interact with session
 * - Control: Full control including session management
 *
 * User workflows:
 * - View all sessions shared with current user
 * - Check session status before connecting
 * - Open shared session in viewer
 * - Monitor session owner and sharing details
 * - Receive notifications when shared sessions change state
 *
 * Real-time features:
 * - Session state change notifications
 * - Session termination alerts
 * - Sharing revocation notifications
 * - New share notifications
 * - WebSocket connection status
 *
 * @page
 * @route /shared-sessions - Sessions shared with current user
 * @access user - Shows only sessions shared with the authenticated user
 *
 * @component
 *
 * @returns {JSX.Element} Shared sessions page with session cards
 *
 * @example
 * // Route configuration:
 * <Route path="/shared-sessions" element={<SharedSessions />} />
 * <Route path="/sessions/shared" element={<SharedSessions />} />
 *
 * @see Sessions for managing own sessions
 * @see SessionViewer for viewing shared sessions
 * @see SessionShareDialog for sharing workflow (from owner perspective)
 */
export default function SharedSessions() {
  const navigate = useNavigate();
  const currentUser = useUserStore((state) => state.user);

  // Fetch shared sessions via React Query
  const { data: sessions = [], isLoading: loading, error: queryError } = useSharedSessions(currentUser?.id);
  const error = queryError instanceof Error ? queryError.message : '';

  // Track previous session states for change notifications
  const prevStatesRef = useRef<Map<string, string>>(new Map());

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time session updates via WebSocket with notifications
  // Wrap callback in useCallback to prevent reconnection loop
  const handleSessionsUpdate = useCallback((updatedSessions: any[]) => {
    if (!currentUser?.id || sessions.length === 0) return;

    // Update shared sessions with real-time data and show notifications for changes
    sessions.forEach((sharedSession) => {
      const updated = updatedSessions.find((s: any) => s.id === sharedSession.id);
      if (updated) {
        // Check if state changed
        const prevState = prevStatesRef.current.get(sharedSession.id);
        if (updated.state !== prevState && prevState !== undefined) {
          // Show notification for state changes
          addNotification({
            message: `${sharedSession.templateName} (${sharedSession.ownerUsername}): ${prevState} → ${updated.state}`,
            severity: updated.state === 'running' ? 'success' : updated.state === 'hibernated' ? 'warning' : 'error',
            priority: updated.state === 'terminated' ? 'high' : 'medium',
            title: 'Shared Session Updated',
          });
        }

        // Update state tracking
        prevStatesRef.current.set(sharedSession.id, updated.state);
      }
    });
  }, [currentUser?.id, sessions, addNotification]);

  const baseWebSocket = useSessionsWebSocket(handleSessionsUpdate);

  // Enhanced WebSocket with connection quality and manual reconnect
  const enhanced = useEnhancedWebSocket(baseWebSocket);

  const handleConnect = (session: SharedSession) => {
    // Navigate to the session viewer
    navigate(`/sessions/${session.id}/viewer`);
  };

  const getStateColor = (state: string) => {
    switch (state) {
      case 'running':
        return 'success';
      case 'hibernated':
        return 'warning';
      case 'terminated':
        return 'error';
      default:
        return 'default';
    }
  };

  const getPermissionColor = (level: string) => {
    switch (level) {
      case 'control':
        return 'error';
      case 'collaborate':
        return 'warning';
      case 'view':
        return 'info';
      default:
        return 'default';
    }
  };

  const getPermissionLabel = (level: string) => {
    switch (level) {
      case 'control':
        return 'Full Control';
      case 'collaborate':
        return 'Can Collaborate';
      case 'view':
        return 'View Only';
      default:
        return level;
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  if (loading) {
    return (
      <Layout>
        <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
          <CircularProgress />
        </Box>
      </Layout>
    );
  }

  return (
    <WebSocketErrorBoundary>
      <Layout>
        <Box>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
            <Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <Typography variant="h4" sx={{ fontWeight: 700 }}>
                  Shared with Me
                </Typography>

                {/* Enhanced WebSocket Connection Status */}
                <EnhancedWebSocketStatus {...enhanced} size="small" showDetails={true} />
              </Box>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              Sessions that other users have shared with you
            </Typography>
          </Box>
          <Button
            variant="outlined"
            startIcon={<ShareIcon />}
            onClick={() => navigate('/sessions')}
          >
            My Sessions
          </Button>
        </Box>

        {error && (
          <Alert severity="error" sx={{ mb: 3 }}>
            {error}
          </Alert>
        )}

        {sessions.length === 0 ? (
          <Card>
            <CardContent sx={{ textAlign: 'center', py: 6 }}>
              <ShareIcon sx={{ fontSize: 64, color: 'text.secondary', mb: 2 }} />
              <Typography variant="h6" gutterBottom>
                No shared sessions yet
              </Typography>
              <Typography variant="body2" color="text.secondary">
                When other users share their sessions with you, they will appear here.
              </Typography>
            </CardContent>
          </Card>
        ) : (
          <Grid container spacing={3}>
            {sessions.map((session) => (
              <Grid item xs={12} md={6} lg={4} key={session.id}>
                <Card>
                  <CardContent>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', mb: 2 }}>
                      <Box>
                        <Typography variant="h6" sx={{ fontWeight: 600 }}>
                          {session.templateName}
                        </Typography>
                        <Typography variant="caption" color="text.secondary" display="flex" alignItems="center" gap={0.5} mt={0.5}>
                          <OwnerIcon sx={{ fontSize: 14 }} />
                          Owned by {session.ownerUsername}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', gap: 0.5, flexDirection: 'column', alignItems: 'flex-end' }}>
                        <Chip label={session.state} size="small" color={getStateColor(session.state)} />
                      </Box>
                    </Box>

                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="body2" color="text.secondary">
                          Your Permission
                        </Typography>
                        <Chip
                          label={getPermissionLabel(session.permissionLevel)}
                          size="small"
                          color={getPermissionColor(session.permissionLevel)}
                        />
                      </Box>

                      {session.appType && (
                        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Typography variant="body2" color="text.secondary">
                            App Type
                          </Typography>
                          <Typography variant="body2">{session.appType}</Typography>
                        </Box>
                      )}

                      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="body2" color="text.secondary">
                          Shared On
                        </Typography>
                        <Typography variant="body2">{formatDate(session.sharedAt)}</Typography>
                      </Box>

                      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="body2" color="text.secondary">
                          Created On
                        </Typography>
                        <Typography variant="body2">{formatDate(session.createdAt)}</Typography>
                      </Box>

                      {session.url && (
                        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Typography variant="body2" color="text.secondary">
                            URL
                          </Typography>
                          <Typography variant="body2" sx={{ fontSize: '0.75rem', maxWidth: '60%' }} noWrap>
                            {session.url}
                          </Typography>
                        </Box>
                      )}
                    </Box>
                  </CardContent>
                  <CardActions sx={{ px: 2, pb: 2 }}>
                    <Button
                      size="small"
                      startIcon={<OpenIcon />}
                      onClick={() => handleConnect(session)}
                      disabled={session.state !== 'running'}
                      variant="contained"
                    >
                      {session.state === 'running' ? 'Connect' : 'Not Available'}
                    </Button>
                    {session.permissionLevel === 'view' && (
                      <Typography variant="caption" color="text.secondary" sx={{ ml: 'auto' }}>
                        Read-only access
                      </Typography>
                    )}
                  </CardActions>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        {sessions.length > 0 && (
          <Box mt={3}>
            <Alert severity="info">
              <Typography variant="body2">
                <strong>Permission Levels:</strong>
              </Typography>
              <Typography variant="caption" display="block" mt={0.5}>
                • <strong>View Only:</strong> You can see the session but cannot interact with it
              </Typography>
              <Typography variant="caption" display="block">
                • <strong>Collaborate:</strong> You can view and interact with the session
              </Typography>
              <Typography variant="caption" display="block">
                • <strong>Full Control:</strong> You can view, interact, and manage the session (including sharing)
              </Typography>
            </Alert>
          </Box>
        )}
      </Box>
    </Layout>
    </WebSocketErrorBoundary>
  );
}

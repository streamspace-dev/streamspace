import { useState, useMemo, useRef, useEffect } from 'react';
import {
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  CardActions,
  Grid,
  Chip,
  IconButton,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  MenuItem,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Pause as PauseIcon,
  Delete as DeleteIcon,
  OpenInNew as OpenIcon,
  LocalOffer as TagIcon,
  Share as ShareIcon,
  Link as LinkIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import Layout from '../components/Layout';
import TagChip from '../components/TagChip';
import TagManager from '../components/TagManager';
import SessionShareDialog from '../components/SessionShareDialog';
import SessionInvitationDialog from '../components/SessionInvitationDialog';
import QuotaAlert from '../components/QuotaAlert';
import ActivityIndicator from '../components/ActivityIndicator';
import IdleTimer from '../components/IdleTimer';
import { useSessions, useUpdateSessionState, useDeleteSession } from '../hooks/useApi';
import { useSessionsWebSocket } from '../hooks/useWebSocket';
import { useUserStore } from '../store/userStore';
import { Session } from '../lib/api';
import { api } from '../lib/api';
import { useEnhancedWebSocket } from '../hooks/useWebSocketEnhancements';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

/**
 * Sessions - Session management page for viewing and controlling user sessions
 *
 * Comprehensive session management interface providing:
 * - Grid view of all user sessions with detailed status
 * - Session state control (start, pause/hibernate, terminate/delete)
 * - Real-time session updates via WebSocket
 * - Tag management for organizing sessions
 * - Session sharing with other users or via invitation links
 * - Resource usage and activity monitoring
 * - Idle timeout tracking with visual progress indicators
 * - Tag-based filtering of sessions
 * - Quota alerts for resource limits
 *
 * Session actions available:
 * - Connect to running sessions (opens SessionViewer)
 * - Hibernate sessions to save resources
 * - Wake hibernated sessions
 * - Delete/terminate sessions
 * - Share sessions with specific users (direct sharing)
 * - Create invitation links for session access
 * - Manage session tags for organization
 *
 * Real-time features:
 * - Session state change notifications
 * - Activity indicators (active/idle status)
 * - Resource usage updates
 * - Active connection counts
 * - Idle time countdown with auto-hibernate warnings
 *
 * @page
 * @route /sessions - User sessions management page
 * @access user - Shows only current user's sessions
 *
 * @component
 *
 * @returns {JSX.Element} Sessions management page with session cards and controls
 *
 * @example
 * // Route configuration:
 * <Route path="/sessions" element={<Sessions />} />
 *
 * @see SessionViewer for connecting to running sessions
 * @see Catalog for creating new sessions from templates
 * @see SharedSessions for viewing sessions shared by others
 */
export default function Sessions() {
  const navigate = useNavigate();
  const username = useUserStore((state) => state.user?.username);
  const [sessions, setSessions] = useState<Session[]>([]);
  // Initial REST fetch — seeds `sessions` so the page isn't blank during the
  // window between mount and the first WebSocket push (or forever if the WS
  // can't connect, which used to leave Sessions silently empty).
  const { data: initialSessions } = useSessions(username);
  const updateSessionState = useUpdateSessionState();
  const deleteSession = useDeleteSession();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);
  const [tagManagerOpen, setTagManagerOpen] = useState(false);
  const [selectedSession, setSelectedSession] = useState<Session | null>(null);
  const [selectedTagFilter, setSelectedTagFilter] = useState<string>('');

  // Sharing state
  const [shareDialogOpen, setShareDialogOpen] = useState(false);
  const [invitationDialogOpen, setInvitationDialogOpen] = useState(false);
  const [sessionToShare, setSessionToShare] = useState<Session | null>(null);

  // Track previous session states for change notifications
  const prevStatesRef = useRef<Map<string, string>>(new Map());

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time sessions updates via WebSocket with notifications
  const baseSessionsWs = useSessionsWebSocket((updatedSessions) => {
    // Filter to only show current user's sessions
    const userSessions = username
      ? updatedSessions.filter((s: Session) => s.user === username)
      : updatedSessions;

    // Check for state changes and show notifications
    userSessions.forEach((session) => {
      const prevState = prevStatesRef.current.get(session.name);
      if (prevState && prevState !== session.state) {
        addNotification({
          message: `${session.template}: ${prevState} → ${session.state}`,
          severity: session.state === 'running' ? 'success' : session.state === 'hibernated' ? 'warning' : 'error',
          priority: session.state === 'terminated' ? 'high' : 'medium',
          title: 'Session Status Changed',
        });
      }
      prevStatesRef.current.set(session.name, session.state);
    });

    setSessions(userSessions);
  });

  // Enhanced WebSocket with connection quality and manual reconnect
  const sessionsWs = useEnhancedWebSocket(baseSessionsWs);

  // Seed sessions from the initial REST fetch on mount. We only set if local
  // state is still empty so a faster WebSocket push wins the race; if WS never
  // connects (mock mode, network glitch), the page still renders the data we
  // got from REST instead of being permanently empty.
  useEffect(() => {
    if (initialSessions && sessions.length === 0) {
      const filtered = username
        ? initialSessions.filter((s) => s.user === username)
        : initialSessions;
      setSessions(filtered);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialSessions]);

  const handleStateChange = (id: string, state: 'running' | 'hibernated') => {
    updateSessionState.mutate({ id, state });
  };

  const handleDelete = () => {
    if (sessionToDelete) {
      deleteSession.mutate(sessionToDelete, {
        onSuccess: () => {
          setDeleteDialogOpen(false);
          setSessionToDelete(null);
        },
      });
    }
  };

  const handleConnect = (session: Session) => {
    // Navigate to the session viewer
    navigate(`/sessions/${session.name}/viewer`);
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

  const getPhaseColor = (phase: string) => {
    switch (phase) {
      case 'Running':
        return 'success';
      case 'Pending':
        return 'info';
      case 'Hibernated':
        return 'warning';
      case 'Failed':
        return 'error';
      default:
        return 'default';
    }
  };

  const handleManageTags = (session: Session) => {
    setSelectedSession(session);
    setTagManagerOpen(true);
  };

  // BUG FIX: Add isMounted ref to prevent setState after unmount
  const isMountedRef = useRef(true);

  // Set up mount tracking with cleanup
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  // BUG FIX: Add error handling and mount check to prevent memory leaks
  const handleSaveTags = async (tags: string[]) => {
    if (!selectedSession) return;

    try {
      await api.updateSessionTags(selectedSession.name, tags);

      // Only update state if component is still mounted
      if (isMountedRef.current) {
        // Update local state
        setSessions(sessions.map(s =>
          s.name === selectedSession.name ? { ...s, tags } : s
        ));
      }
    } catch (error) {
      console.error('Failed to update session tags:', error);
      // Error notification is already handled by API interceptor
      // Re-throw to allow TagManager to handle UI state
      throw error;
    }
  };

  const handleOpenShareDialog = (session: Session) => {
    setSessionToShare(session);
    setShareDialogOpen(true);
  };

  const handleOpenInvitationDialog = (session: Session) => {
    setSessionToShare(session);
    setInvitationDialogOpen(true);
  };

  // Get all unique tags from sessions
  const allTags = useMemo(() => {
    const tagSet = new Set<string>();
    sessions.forEach(session => {
      session.tags?.forEach(tag => tagSet.add(tag));
    });
    return Array.from(tagSet).sort();
  }, [sessions]);

  // Filter sessions by selected tag
  const filteredSessions = useMemo(() => {
    if (!selectedTagFilter) return sessions;
    return sessions.filter(session =>
      session.tags?.includes(selectedTagFilter)
    );
  }, [sessions, selectedTagFilter]);

  return (
    <WebSocketErrorBoundary>
      <Layout>
        <Box>
          <QuotaAlert />

          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              My Sessions
            </Typography>
            <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
              {allTags.length > 0 && (
                <TextField
                  select
                  size="small"
                  label="Filter by Tag"
                  value={selectedTagFilter}
                  onChange={(e) => setSelectedTagFilter(e.target.value)}
                  sx={{ minWidth: 150 }}
                >
                  <MenuItem value="">All Sessions</MenuItem>
                  {allTags.map(tag => (
                    <MenuItem key={tag} value={tag}>
                      {tag}
                    </MenuItem>
                  ))}
                </TextField>
              )}

              {/* Enhanced WebSocket Connection Status */}
              <EnhancedWebSocketStatus
                {...sessionsWs}
                size="small"
                showDetails={true}
              />
            </Box>
          </Box>

        {filteredSessions.length === 0 && sessions.length > 0 ? (
          <Alert severity="info">
            No sessions found with the selected tag. Clear the filter to see all sessions.
          </Alert>
        ) : sessions.length === 0 ? (
          <Alert
            severity="info"
            action={
              <Button color="inherit" size="small" onClick={() => navigate('/')}>
                Browse Applications
              </Button>
            }
          >
            You don't have any sessions yet. Pick an application to launch one.
          </Alert>
        ) : (
          <Grid container spacing={3}>
            {filteredSessions.map((session) => (
              <Grid item xs={12} md={6} lg={4} key={session.name}>
                <Card>
                  <CardContent>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2, gap: 1 }}>
                      <Box sx={{ minWidth: 0, flex: 1 }}>
                        <Typography
                          variant="h6"
                          sx={{
                            fontWeight: 600,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                          title={session.template}
                        >
                          {session.template}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{
                            display: 'block',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                          title={session.name}
                        >
                          {session.name}
                        </Typography>
                      </Box>
                      {/* Single status pill — phase is the most informative
                          (Running/Pending/Hibernated/Failed); the lowercase
                          `state` Chip and a separate ActivityIndicator pill
                          were stacked here previously, three pills for the
                          same conceptual thing. ActivityIndicator now only
                          appears inline below when the session is idle. */}
                      <Chip
                        label={session.status.phase || session.state}
                        size="small"
                        color={getPhaseColor(session.status.phase) || getStateColor(session.state)}
                      />
                    </Box>

                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                      {session.state === 'running' && session.lastActivity && session.idleThreshold && (
                        <Box>
                          <IdleTimer
                            lastActivity={session.lastActivity}
                            idleDuration={session.idleDuration}
                            idleThreshold={session.idleThreshold}
                            showProgress={session.isIdle || false}
                            compact={true}
                          />
                        </Box>
                      )}
                      <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                        <Typography variant="body2" color="text.secondary">
                          Resources
                        </Typography>
                        <Typography variant="body2">
                          {session.resources?.memory || 'N/A'} / {session.resources?.cpu || 'N/A'}
                        </Typography>
                      </Box>
                      {session.activeConnections !== undefined && (
                        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Typography variant="body2" color="text.secondary">
                            Active Connections
                          </Typography>
                          <Typography variant="body2">{session.activeConnections}</Typography>
                        </Box>
                      )}
                      {session.status.url && (
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 1, minWidth: 0 }}>
                          <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>
                            URL
                          </Typography>
                          <Typography
                            variant="body2"
                            sx={{
                              fontSize: '0.75rem',
                              minWidth: 0,
                              flex: 1,
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                              textAlign: 'right',
                            }}
                            title={session.status.url}
                          >
                            {session.status.url}
                          </Typography>
                        </Box>
                      )}
                      {/* Idle indicator only when actively idle — not as a
                          third stacked status pill. */}
                      {session.isIdle && session.state === 'running' && (
                        <Box sx={{ display: 'flex', justifyContent: 'flex-start' }}>
                          <ActivityIndicator
                            isActive={session.isActive}
                            isIdle={session.isIdle}
                            state={session.state}
                            size="small"
                          />
                        </Box>
                      )}
                      {session.tags && session.tags.length > 0 && (
                        <Box sx={{ mt: 1 }}>
                          <Typography variant="caption" color="text.secondary" display="block" gutterBottom>
                            Tags
                          </Typography>
                          <Box display="flex" flexWrap="wrap" gap={0.5}>
                            {session.tags.map(tag => (
                              <TagChip key={tag} tag={tag} />
                            ))}
                          </Box>
                        </Box>
                      )}
                    </Box>
                  </CardContent>
                  {/* Action row: previously wrapped inconsistently (Connect on
                      its own line on narrow cards). flexWrap + gap keeps the
                      primary action (Connect/Resume) and the icon row on the
                      same line at sensible card widths and wraps cleanly when
                      they don't fit. */}
                  <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2, flexWrap: 'wrap', gap: 1 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      {session.state === 'running' ? (
                        <>
                          <Button
                            size="small"
                            startIcon={<OpenIcon />}
                            onClick={() => handleConnect(session)}
                            disabled={session.status.phase !== 'Running'}
                          >
                            Connect
                          </Button>
                          <IconButton
                            size="small"
                            color="warning"
                            onClick={() => handleStateChange(session.name, 'hibernated')}
                            disabled={updateSessionState.isPending}
                            title="Hibernate"
                          >
                            <PauseIcon />
                          </IconButton>
                        </>
                      ) : (
                        <IconButton
                          size="small"
                          color="success"
                          onClick={() => handleStateChange(session.name, 'running')}
                          disabled={updateSessionState.isPending}
                          title="Resume"
                        >
                          <PlayIcon />
                        </IconButton>
                      )}
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                      <IconButton
                        size="small"
                        color="primary"
                        onClick={() => handleOpenShareDialog(session)}
                        title="Share with User"
                      >
                        <ShareIcon />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="primary"
                        onClick={() => handleOpenInvitationDialog(session)}
                        title="Create Invitation Link"
                      >
                        <LinkIcon />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="primary"
                        onClick={() => handleManageTags(session)}
                        title="Manage Tags"
                      >
                        <TagIcon />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => {
                          setSessionToDelete(session.name);
                          setDeleteDialogOpen(true);
                        }}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </Box>
                  </CardActions>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Delete Session</DialogTitle>
          <DialogContent>
            Are you sure you want to delete this session? This action cannot be undone.
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleDelete} color="error" variant="contained" disabled={deleteSession.isPending}>
              Delete
            </Button>
          </DialogActions>
        </Dialog>

        {selectedSession && (
          <TagManager
            open={tagManagerOpen}
            session={selectedSession}
            onClose={() => {
              setTagManagerOpen(false);
              setSelectedSession(null);
            }}
            onSave={handleSaveTags}
          />
        )}

        {sessionToShare && (
          <>
            <SessionShareDialog
              open={shareDialogOpen}
              sessionId={sessionToShare.name}
              sessionName={sessionToShare.name}
              onClose={() => {
                setShareDialogOpen(false);
                setSessionToShare(null);
              }}
            />
            <SessionInvitationDialog
              open={invitationDialogOpen}
              sessionId={sessionToShare.name}
              sessionName={sessionToShare.name}
              onClose={() => {
                setInvitationDialogOpen(false);
                setSessionToShare(null);
              }}
            />
          </>
        )}
      </Box>
    </Layout>
    </WebSocketErrorBoundary>
  );
}

import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  AppBar,
  Toolbar,
  Typography,
  IconButton,
  Button,
  Chip,
  CircularProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Tooltip,
  Snackbar,
} from '@mui/material';
import {
  Close as CloseIcon,
  Fullscreen as FullscreenIcon,
  FullscreenExit as FullscreenExitIcon,
  Refresh as RefreshIcon,
  Info as InfoIcon,
  Share as ShareIcon,
  People as PeopleIcon,
  Link as LinkIcon,
  Wifi as ConnectedIcon,
  WifiOff as DisconnectedIcon,
} from '@mui/icons-material';
import { api } from '../lib/api';
import { useUserStore } from '../store/userStore';
import { useSessionsWebSocket } from '../hooks/useWebSocket';
import SessionShareDialog from '../components/SessionShareDialog';
import SessionInvitationDialog from '../components/SessionInvitationDialog';
import SessionCollaboratorsPanel from '../components/SessionCollaboratorsPanel';

export default function SessionViewer() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const navigate = useNavigate();
  const username = useUserStore((state) => state.user?.username);

  const [session, setSession] = useState<any>(null);
  const [connectionId, setConnectionId] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [infoDialogOpen, setInfoDialogOpen] = useState(false);

  // Sharing state
  const [shareDialogOpen, setShareDialogOpen] = useState(false);
  const [invitationDialogOpen, setInvitationDialogOpen] = useState(false);
  const [collaboratorsDialogOpen, setCollaboratorsDialogOpen] = useState(false);
  const [isOwner, setIsOwner] = useState(false);

  // WebSocket state
  const [stateChangeNotification, setStateChangeNotification] = useState<string | null>(null);

  const iframeRef = useRef<HTMLIFrameElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const heartbeatIntervalRef = useRef<NodeJS.Timeout | null>(null);

  // Real-time session updates via WebSocket
  const { isConnected, reconnectAttempts } = useSessionsWebSocket((updatedSessions) => {
    if (!sessionId) return;

    // Find this session in the update
    const updatedSession = updatedSessions.find((s: any) => s.id === sessionId);
    if (updatedSession && session) {
      // Check if state changed
      if (updatedSession.state !== session.state) {
        setStateChangeNotification(
          `Session state changed: ${session.state} → ${updatedSession.state}`
        );

        // If session was hibernated or terminated, show alert
        if (updatedSession.state === 'hibernated' || updatedSession.state === 'terminated') {
          setError(`Session has been ${updatedSession.state}. Please close this viewer.`);
        }
      }

      // Update session data
      setSession(updatedSession);
    }
  });

  useEffect(() => {
    if (!sessionId || !username) {
      setError('Missing session ID or username');
      setLoading(false);
      return;
    }

    loadSession();

    // Cleanup on unmount
    return () => {
      if (heartbeatIntervalRef.current) {
        clearInterval(heartbeatIntervalRef.current);
      }
      if (connectionId) {
        handleDisconnect();
      }
    };
  }, [sessionId, username]);

  const loadSession = async () => {
    if (!sessionId || !username) return;

    setLoading(true);
    setError('');

    try {
      // Get session details
      const sessionData = await api.getSession(sessionId);
      setSession(sessionData);

      // Check if current user is the session owner
      setIsOwner(sessionData.user === username);

      // Check if session is running
      if (sessionData.state !== 'running') {
        setError('Session is not running. Please start the session first.');
        setLoading(false);
        return;
      }

      if (sessionData.status.phase !== 'Running') {
        setError(`Session is not ready yet. Current phase: ${sessionData.status.phase}`);
        setLoading(false);
        return;
      }

      // Connect to the session
      const connectionResult = await api.connectSession(sessionId, username);
      setConnectionId(connectionResult.connectionId);

      // Start heartbeat
      startHeartbeat(sessionId, connectionResult.connectionId);

      setLoading(false);
    } catch (err: any) {
      console.error('Failed to load session:', err);
      setError(err.response?.data?.message || 'Failed to connect to session');
      setLoading(false);
    }
  };

  const startHeartbeat = (sessionId: string, connId: string) => {
    // Send heartbeat every 30 seconds
    heartbeatIntervalRef.current = setInterval(async () => {
      try {
        await api.sendHeartbeat(sessionId, connId);
      } catch (err) {
        console.error('Heartbeat failed:', err);
        // Stop heartbeat on error
        if (heartbeatIntervalRef.current) {
          clearInterval(heartbeatIntervalRef.current);
        }
      }
    }, 30000);
  };

  const handleDisconnect = async () => {
    if (!sessionId || !connectionId) return;

    try {
      await api.disconnectSession(sessionId, connectionId);
    } catch (err) {
      console.error('Failed to disconnect:', err);
    }
  };

  const handleClose = async () => {
    if (heartbeatIntervalRef.current) {
      clearInterval(heartbeatIntervalRef.current);
    }

    if (connectionId) {
      await handleDisconnect();
    }

    navigate('/sessions');
  };

  const toggleFullscreen = () => {
    if (!containerRef.current) return;

    if (!isFullscreen) {
      if (containerRef.current.requestFullscreen) {
        containerRef.current.requestFullscreen();
      }
    } else {
      if (document.exitFullscreen) {
        document.exitFullscreen();
      }
    }

    setIsFullscreen(!isFullscreen);
  };

  const handleRefresh = () => {
    if (iframeRef.current) {
      iframeRef.current.src = iframeRef.current.src;
    }
  };

  if (loading) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100vh',
          bgcolor: 'background.default',
        }}
      >
        <CircularProgress size={60} />
        <Typography variant="h6" sx={{ mt: 3 }}>
          Connecting to session...
        </Typography>
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100vh',
          bgcolor: 'background.default',
          p: 3,
        }}
      >
        <Alert severity="error" sx={{ mb: 3, maxWidth: 600 }}>
          {error}
        </Alert>
        <Button variant="contained" onClick={() => navigate('/sessions')}>
          Back to Sessions
        </Button>
      </Box>
    );
  }

  if (!session || !session.status.url) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100vh',
          bgcolor: 'background.default',
        }}
      >
        <Alert severity="warning">Session URL not available</Alert>
        <Button variant="contained" onClick={() => navigate('/sessions')} sx={{ mt: 2 }}>
          Back to Sessions
        </Button>
      </Box>
    );
  }

  return (
    <Box ref={containerRef} sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <AppBar position="static" elevation={1}>
        <Toolbar>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>
            {session.template} - {session.name}
          </Typography>

          {/* WebSocket Connection Status */}
          <Chip
            icon={isConnected ? <ConnectedIcon /> : <DisconnectedIcon />}
            label={
              isConnected
                ? 'Live Updates'
                : reconnectAttempts > 0
                ? `Reconnecting... (${reconnectAttempts})`
                : 'Disconnected'
            }
            size="small"
            color={isConnected ? 'success' : 'default'}
            sx={{ mr: 2 }}
          />

          <Chip
            label={`${session.activeConnections || 0} connection(s)`}
            size="small"
            sx={{ mr: 2 }}
          />

          {isOwner && (
            <>
              <Tooltip title="Share with User">
                <IconButton color="inherit" onClick={() => setShareDialogOpen(true)}>
                  <ShareIcon />
                </IconButton>
              </Tooltip>

              <Tooltip title="Create Invitation Link">
                <IconButton color="inherit" onClick={() => setInvitationDialogOpen(true)}>
                  <LinkIcon />
                </IconButton>
              </Tooltip>
            </>
          )}

          <Tooltip title="View Collaborators">
            <IconButton color="inherit" onClick={() => setCollaboratorsDialogOpen(true)}>
              <PeopleIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title="Session Info">
            <IconButton color="inherit" onClick={() => setInfoDialogOpen(true)}>
              <InfoIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title="Refresh">
            <IconButton color="inherit" onClick={handleRefresh}>
              <RefreshIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}>
            <IconButton color="inherit" onClick={toggleFullscreen}>
              {isFullscreen ? <FullscreenExitIcon /> : <FullscreenIcon />}
            </IconButton>
          </Tooltip>

          <Tooltip title="Close Session">
            <IconButton color="inherit" onClick={handleClose}>
              <CloseIcon />
            </IconButton>
          </Tooltip>
        </Toolbar>
      </AppBar>

      <Box sx={{ flex: 1, position: 'relative', bgcolor: '#000' }}>
        <iframe
          ref={iframeRef}
          src={session.status.url}
          style={{
            width: '100%',
            height: '100%',
            border: 'none',
            display: 'block',
          }}
          title={`Session: ${session.name}`}
          allow="clipboard-read; clipboard-write"
        />
      </Box>

      {/* Sharing Dialogs */}
      {sessionId && session && (
        <>
          <SessionShareDialog
            open={shareDialogOpen}
            sessionId={sessionId}
            sessionName={session.name}
            onClose={() => setShareDialogOpen(false)}
          />
          <SessionInvitationDialog
            open={invitationDialogOpen}
            sessionId={sessionId}
            sessionName={session.name}
            onClose={() => setInvitationDialogOpen(false)}
          />
        </>
      )}

      {/* Collaborators Dialog */}
      <Dialog
        open={collaboratorsDialogOpen}
        onClose={() => setCollaboratorsDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          <Box display="flex" justifyContent="space-between" alignItems="center">
            <Typography variant="h6">Session Collaborators</Typography>
            <IconButton onClick={() => setCollaboratorsDialogOpen(false)} size="small">
              <CloseIcon />
            </IconButton>
          </Box>
        </DialogTitle>
        <DialogContent>
          {sessionId && (
            <SessionCollaboratorsPanel
              sessionId={sessionId}
              isOwner={isOwner}
              onUpdate={loadSession}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCollaboratorsDialogOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>

      {/* Session Info Dialog */}
      <Dialog open={infoDialogOpen} onClose={() => setInfoDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Session Information</DialogTitle>
        <DialogContent>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <Box>
              <Typography variant="caption" color="text.secondary">
                Session Name
              </Typography>
              <Typography variant="body1">{session.name}</Typography>
            </Box>

            <Box>
              <Typography variant="caption" color="text.secondary">
                Template
              </Typography>
              <Typography variant="body1">{session.template}</Typography>
            </Box>

            <Box>
              <Typography variant="caption" color="text.secondary">
                User
              </Typography>
              <Typography variant="body1">{session.user}</Typography>
            </Box>

            <Box>
              <Typography variant="caption" color="text.secondary">
                State
              </Typography>
              <Chip label={session.state} size="small" color="success" />
            </Box>

            <Box>
              <Typography variant="caption" color="text.secondary">
                Phase
              </Typography>
              <Chip label={session.status.phase} size="small" color="success" />
            </Box>

            {session.status.podName && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Pod Name
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                  {session.status.podName}
                </Typography>
              </Box>
            )}

            <Box>
              <Typography variant="caption" color="text.secondary">
                Resources
              </Typography>
              <Typography variant="body2">
                CPU: {session.resources?.cpu || 'N/A'}
                {' • '}
                Memory: {session.resources?.memory || 'N/A'}
              </Typography>
            </Box>

            {session.status.resourceUsage && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Current Usage
                </Typography>
                <Typography variant="body2">
                  CPU: {session.status.resourceUsage.cpu || 'N/A'}
                  {' • '}
                  Memory: {session.status.resourceUsage.memory || 'N/A'}
                </Typography>
              </Box>
            )}

            <Box>
              <Typography variant="caption" color="text.secondary">
                Active Connections
              </Typography>
              <Typography variant="body1">{session.activeConnections || 0}</Typography>
            </Box>

            {session.idleTimeout && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Idle Timeout
                </Typography>
                <Typography variant="body1">{session.idleTimeout}</Typography>
              </Box>
            )}

            {connectionId && (
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Connection ID
                </Typography>
                <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.75rem' }}>
                  {connectionId}
                </Typography>
              </Box>
            )}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setInfoDialogOpen(false)}>Close</Button>
        </DialogActions>
      </Dialog>

      {/* State Change Notification */}
      <Snackbar
        open={!!stateChangeNotification}
        autoHideDuration={6000}
        onClose={() => setStateChangeNotification(null)}
        message={stateChangeNotification}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      />
    </Box>
  );
}

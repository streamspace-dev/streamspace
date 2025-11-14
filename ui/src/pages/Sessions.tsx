import { useState } from 'react';
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
  CircularProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Pause as PauseIcon,
  Delete as DeleteIcon,
  OpenInNew as OpenIcon,
  SignalWifiStatusbar4Bar as ConnectedIcon,
  SignalWifiStatusbarConnectedNoInternet4 as DisconnectedIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import Layout from '../components/Layout';
import { useUpdateSessionState, useDeleteSession, useConnectSession } from '../hooks/useApi';
import { useSessionsWebSocket } from '../hooks/useWebSocket';
import { useUserStore } from '../store/userStore';
import { Session } from '../lib/api';

export default function Sessions() {
  const navigate = useNavigate();
  const username = useUserStore((state) => state.username);
  const [sessions, setSessions] = useState<Session[]>([]);
  const updateSessionState = useUpdateSessionState();
  const deleteSession = useDeleteSession();
  const connectSession = useConnectSession();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);

  // Real-time sessions updates via WebSocket
  const sessionsWs = useSessionsWebSocket((updatedSessions) => {
    // Filter to only show current user's sessions
    const userSessions = username
      ? updatedSessions.filter((s: Session) => s.user === username)
      : updatedSessions;
    setSessions(userSessions);
  });

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

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            My Sessions
          </Typography>
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
            <Chip
              icon={sessionsWs.isConnected ? <ConnectedIcon /> : <DisconnectedIcon />}
              label={sessionsWs.isConnected ? 'Live Updates' : 'Reconnecting...'}
              color={sessionsWs.isConnected ? 'success' : 'warning'}
              size="small"
              variant="outlined"
            />
            {sessionsWs.reconnectAttempts > 0 && (
              <Typography variant="caption" color="text.secondary">
                Attempt {sessionsWs.reconnectAttempts}
              </Typography>
            )}
          </Box>
        </Box>

        {sessions.length === 0 ? (
          <Alert severity="info">
            You don't have any sessions yet. Visit the Template Catalog to create one!
          </Alert>
        ) : (
          <Grid container spacing={3}>
            {sessions.map((session) => (
              <Grid item xs={12} md={6} lg={4} key={session.name}>
                <Card>
                  <CardContent>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', mb: 2 }}>
                      <Box>
                        <Typography variant="h6" sx={{ fontWeight: 600 }}>
                          {session.template}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {session.name}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', gap: 0.5, flexDirection: 'column', alignItems: 'flex-end' }}>
                        <Chip label={session.state} size="small" color={getStateColor(session.state)} />
                        <Chip label={session.status.phase} size="small" color={getPhaseColor(session.status.phase)} />
                      </Box>
                    </Box>

                    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
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
                        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                          <Typography variant="body2" color="text.secondary">
                            URL
                          </Typography>
                          <Typography variant="body2" sx={{ fontSize: '0.75rem', maxWidth: '60%' }} noWrap>
                            {session.status.url}
                          </Typography>
                        </Box>
                      )}
                    </Box>
                  </CardContent>
                  <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
                    <Box>
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
                        >
                          <PlayIcon />
                        </IconButton>
                      )}
                    </Box>
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
      </Box>
    </Layout>
  );
}

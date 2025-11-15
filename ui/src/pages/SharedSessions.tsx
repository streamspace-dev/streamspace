import { useState, useEffect } from 'react';
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
  Wifi as ConnectedIcon,
  WifiOff as DisconnectedIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import Layout from '../components/Layout';
import { api } from '../lib/api';
import { useUserStore } from '../store/userStore';
import { useSessionsWebSocket } from '../hooks/useWebSocket';

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

export default function SharedSessions() {
  const navigate = useNavigate();
  const currentUser = useUserStore((state) => state.user);
  const [sessions, setSessions] = useState<SharedSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Real-time session updates via WebSocket
  const { isConnected, reconnectAttempts } = useSessionsWebSocket((updatedSessions) => {
    if (!currentUser?.id || sessions.length === 0) return;

    // Update shared sessions with real-time data
    const updatedSharedSessions = sessions.map((sharedSession) => {
      const updated = updatedSessions.find((s: any) => s.id === sharedSession.id);
      if (updated) {
        return {
          ...sharedSession,
          state: updated.state,
          url: updated.status?.url || sharedSession.url,
        };
      }
      return sharedSession;
    });

    setSessions(updatedSharedSessions);
  });

  useEffect(() => {
    if (currentUser?.id) {
      loadSharedSessions();
    }
  }, [currentUser]);

  const loadSharedSessions = async () => {
    if (!currentUser?.id) return;

    try {
      setLoading(true);
      const sessionsData = await api.listSharedSessions(currentUser.id);
      setSessions(sessionsData);
      setError('');
    } catch (err) {
      console.error('Failed to load shared sessions:', err);
      setError(err instanceof Error ? err.message : 'Failed to load shared sessions');
    } finally {
      setLoading(false);
    }
  };

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
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Box>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Typography variant="h4" sx={{ fontWeight: 700 }}>
                Shared with Me
              </Typography>
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
              />
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
  );
}

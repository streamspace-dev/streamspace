import { useState } from 'react';
import { Grid, Paper, Typography, Box, Card, CardContent, Chip } from '@mui/material';
import {
  Computer as ComputerIcon,
  Apps as AppsIcon,
  Folder as FolderIcon,
  Timeline as TimelineIcon,
  SignalWifiStatusbar4Bar as ConnectedIcon,
  SignalWifiStatusbarConnectedNoInternet4 as DisconnectedIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import QuotaCard from '../components/QuotaCard';
import { useTemplates, useRepositories } from '../hooks/useApi';
import { useMetricsWebSocket, useSessionsWebSocket } from '../hooks/useWebSocket';
import { useUserStore } from '../store/userStore';
import type { Session } from '../lib/api';

export default function Dashboard() {
  const username = useUserStore((state) => state.user?.username);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [metrics, setMetrics] = useState<any>(null);

  const { data: templates = [], isLoading: templatesLoading } = useTemplates();
  const { data: repositories = [], isLoading: reposLoading } = useRepositories();

  // Real-time sessions updates via WebSocket
  const sessionsWs = useSessionsWebSocket((updatedSessions) => {
    // Filter to only show current user's sessions
    const userSessions = username
      ? updatedSessions.filter((s: Session) => s.user === username)
      : updatedSessions;
    setSessions(userSessions);
  });

  // Real-time metrics updates via WebSocket
  const metricsWs = useMetricsWebSocket((updatedMetrics) => {
    setMetrics(updatedMetrics);
  });

  const stats = [
    {
      title: 'My Sessions',
      value: sessions.length,
      icon: <ComputerIcon sx={{ fontSize: 40 }} />,
      color: '#3f51b5',
      loading: false,
    },
    {
      title: 'Available Templates',
      value: templates.length,
      icon: <AppsIcon sx={{ fontSize: 40 }} />,
      color: '#f50057',
      loading: templatesLoading,
    },
    {
      title: 'Repositories',
      value: repositories.length,
      icon: <FolderIcon sx={{ fontSize: 40 }} />,
      color: '#4caf50',
      loading: reposLoading,
    },
    {
      title: 'Active Connections',
      value: metrics?.activeConnections || 0,
      icon: <TimelineIcon sx={{ fontSize: 40 }} />,
      color: '#ff9800',
      loading: !metricsWs.isConnected,
    },
  ];

  const runningSessions = sessions.filter((s) => s.state === 'running');
  const hibernatedSessions = sessions.filter((s) => s.state === 'hibernated');

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Welcome back, {username}!
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

        <Grid container spacing={3} sx={{ mb: 4 }}>
          {stats.map((stat) => (
            <Grid item xs={12} sm={6} md={3} key={stat.title}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Box>
                      <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                        {stat.title}
                      </Typography>
                      <Typography variant="h4" sx={{ fontWeight: 700 }}>
                        {stat.loading ? '...' : stat.value}
                      </Typography>
                    </Box>
                    <Box sx={{ color: stat.color }}>{stat.icon}</Box>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>

        <Grid container spacing={3}>
          <Grid item xs={12} md={6}>
            <QuotaCard />
          </Grid>

          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                Session Overview
              </Typography>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Running
                  </Typography>
                  <Chip label={runningSessions.length} color="success" size="small" />
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Hibernated
                  </Typography>
                  <Chip label={hibernatedSessions.length} color="warning" size="small" />
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Total
                  </Typography>
                  <Chip label={sessions.length} color="primary" size="small" />
                </Box>
              </Box>
            </Paper>
          </Grid>

          <Grid item xs={12}>
            <Paper sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                Recent Sessions
              </Typography>
              {sessions.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No sessions yet. Create one from the Template Catalog!
                </Typography>
              ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                  {sessions.slice(0, 5).map((session) => (
                    <Box
                      key={session.name}
                      sx={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        p: 1,
                        borderRadius: 1,
                        '&:hover': { bgcolor: 'action.hover' },
                      }}
                    >
                      <Box>
                        <Typography variant="body2" sx={{ fontWeight: 500 }}>
                          {session.template}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          {session.name}
                        </Typography>
                      </Box>
                      <Chip
                        label={session.state}
                        size="small"
                        color={session.state === 'running' ? 'success' : 'default'}
                      />
                    </Box>
                  ))}
                </Box>
              )}
            </Paper>
          </Grid>
        </Grid>
      </Box>
    </Layout>
  );
}

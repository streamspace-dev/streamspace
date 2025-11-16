import { useState, useEffect, useRef } from 'react';
import {
  Box,
  Typography,
  Card,
  CardContent,
  Grid,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  CircularProgress,
  Alert,
  LinearProgress,
} from '@mui/material';
import {
  Computer as ComputerIcon,
  Apps as AppsIcon,
  People as PeopleIcon,
  Storage as StorageIcon,
  Speed as SpeedIcon,
  Memory as MemoryIcon,
  CheckCircle as CheckCircleIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import { api } from '../../lib/api';
import { useMetricsWebSocket } from '../../hooks/useWebSocket';
import { useEnhancedWebSocket } from '../../hooks/useWebSocketEnhancements';
import { useNotificationQueue } from '../../components/NotificationQueue';
import EnhancedWebSocketStatus from '../../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../../components/WebSocketErrorBoundary';

/**
 * AdminDashboard - Platform administration overview and metrics
 *
 * Provides comprehensive administrative dashboard with real-time platform statistics,
 * cluster health monitoring, resource utilization metrics, and recent activity overview.
 * Central control panel for platform administrators to monitor system health, user
 * activity, resource consumption, and receive critical alerts.
 *
 * Features:
 * - Real-time cluster metrics via WebSocket
 * - Cluster health status indicator
 * - Node status monitoring (ready/not ready)
 * - Active sessions and user tracking
 * - CPU and memory utilization graphs
 * - Pod capacity monitoring
 * - Session distribution visualization
 * - Recent sessions table
 * - Critical alert notifications
 *
 * Administrative capabilities:
 * - Monitor all active sessions across the platform
 * - Track cluster node health in real-time
 * - View resource utilization trends
 * - Identify capacity bottlenecks
 * - Receive automatic alerts for critical events
 * - Quick access to detailed admin functions
 *
 * Real-time metrics:
 * - Cluster nodes (total, ready, not ready)
 * - Sessions (total, running, hibernated, terminated)
 * - Users (total, active)
 * - CPU utilization (used, total, percentage)
 * - Memory utilization (used, total, percentage)
 * - Pod capacity (used, total, percentage)
 *
 * Critical notifications:
 * - Node health degradation
 * - CPU utilization > 90%
 * - Memory utilization > 90%
 * - Pod capacity > 90%
 *
 * User workflows:
 * - Monitor platform health at a glance
 * - Identify resource bottlenecks
 * - Track user activity patterns
 * - Respond to critical alerts
 * - Navigate to detailed admin functions
 *
 * @page
 * @route /admin - Platform administration dashboard
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Administrative dashboard with real-time platform metrics
 *
 * @example
 * // Route configuration:
 * <Route path="/admin" element={<AdminDashboard />} />
 *
 * @see Users for user management
 * @see Nodes for detailed cluster node monitoring
 * @see Plugins for plugin administration
 * @see Scaling for load balancing and auto-scaling
 */
interface ClusterMetrics {
  nodes: {
    total: number;
    ready: number;
    notReady: number;
  };
  sessions: {
    total: number;
    running: number;
    hibernated: number;
    terminated: number;
  };
  resources: {
    cpu: {
      total: string;
      used: string;
      percent: number;
    };
    memory: {
      total: string;
      used: string;
      percent: number;
    };
    pods: {
      total: number;
      used: number;
      percent: number;
    };
  };
  users: {
    total: number;
    active: number;
  };
}

interface RecentSession {
  name: string;
  user: string;
  template: string;
  state: string;
  createdAt: string;
}

export default function AdminDashboard() {
  const [metrics, setMetrics] = useState<ClusterMetrics | null>(null);
  const [recentSessions, setRecentSessions] = useState<RecentSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Track previous metrics for critical change notifications
  const prevMetricsRef = useRef<ClusterMetrics | null>(null);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time metrics updates with critical change notifications
  const baseMetricsWs = useMetricsWebSocket((updatedMetrics) => {
    if (updatedMetrics.cluster) {
      const newMetrics = updatedMetrics.cluster;

      // Check for critical changes
      if (prevMetricsRef.current) {
        const prev = prevMetricsRef.current;

        // Node health changes
        if (newMetrics.nodes.ready < prev.nodes.ready) {
          addNotification({
            message: `Cluster nodes decreased: ${prev.nodes.ready} → ${newMetrics.nodes.ready} ready`,
            severity: 'error',
            priority: 'critical',
            title: 'Node Health Alert',
            duration: null, // Don't auto-dismiss critical alerts
          });
        }

        // CPU critical threshold
        if (newMetrics.resources.cpu.percent > 90 && prev.resources.cpu.percent <= 90) {
          addNotification({
            message: `CPU utilization critical: ${newMetrics.resources.cpu.percent.toFixed(1)}%`,
            severity: 'error',
            priority: 'high',
            title: 'Resource Alert',
          });
        }

        // Memory critical threshold
        if (newMetrics.resources.memory.percent > 90 && prev.resources.memory.percent <= 90) {
          addNotification({
            message: `Memory utilization critical: ${newMetrics.resources.memory.percent.toFixed(1)}%`,
            severity: 'error',
            priority: 'high',
            title: 'Resource Alert',
          });
        }

        // Pod capacity warning
        if (newMetrics.resources.pods.percent > 90 && prev.resources.pods.percent <= 90) {
          addNotification({
            message: `Pod capacity critical: ${newMetrics.resources.pods.percent.toFixed(1)}%`,
            severity: 'warning',
            priority: 'high',
            title: 'Capacity Alert',
          });
        }
      }

      prevMetricsRef.current = newMetrics;
      setMetrics(newMetrics);
    }
  });

  // Enhanced WebSocket with connection quality and manual reconnect
  const metricsWs = useEnhancedWebSocket(baseMetricsWs);

  useEffect(() => {
    loadDashboardData();
  }, []);

  const loadDashboardData = async () => {
    setLoading(true);
    setError('');

    try {
      const [metricsData, sessionsData] = await Promise.all([
        api.getMetrics(),
        api.listSessions(),
      ]);

      setMetrics(metricsData.cluster);
      setRecentSessions(sessionsData.slice(0, 10));
    } catch (err: any) {
      console.error('Failed to load dashboard data:', err);
      setError(err.response?.data?.message || 'Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  };

  const formatBytes = (bytes: string): string => {
    const num = parseInt(bytes);
    if (num < 1024) return `${num}B`;
    if (num < 1024 * 1024) return `${(num / 1024).toFixed(1)}KB`;
    if (num < 1024 * 1024 * 1024) return `${(num / (1024 * 1024)).toFixed(1)}MB`;
    return `${(num / (1024 * 1024 * 1024)).toFixed(1)}GB`;
  };

  const getHealthStatus = () => {
    if (!metrics) return { status: 'Unknown', color: 'warning', icon: <WarningIcon /> };

    const nodeHealth = metrics.nodes.ready / metrics.nodes.total;
    const cpuHealth = 1 - (metrics.resources.cpu.percent / 100);
    const memHealth = 1 - (metrics.resources.memory.percent / 100);

    const overallHealth = (nodeHealth + cpuHealth + memHealth) / 3;

    if (overallHealth > 0.8) {
      return { status: 'Healthy', color: 'success', icon: <CheckCircleIcon /> };
    } else if (overallHealth > 0.5) {
      return { status: 'Warning', color: 'warning', icon: <WarningIcon /> };
    } else {
      return { status: 'Critical', color: 'error', icon: <WarningIcon /> };
    }
  };

  const health = getHealthStatus();

  if (loading) {
    return (
      <Layout>
        <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
          <CircularProgress />
        </Box>
      </Layout>
    );
  }

  if (!metrics) {
    return (
      <Layout>
        <Alert severity="error">Failed to load metrics</Alert>
      </Layout>
    );
  }

  return (
    <WebSocketErrorBoundary>
      <Layout>
        <Box>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Admin Dashboard
            </Typography>
            <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
              <Chip
                icon={health.icon}
                label={`Cluster Status: ${health.status}`}
                color={health.color as any}
                sx={{ fontWeight: 600 }}
              />

              {/* Enhanced WebSocket Connection Status */}
              <EnhancedWebSocketStatus
                {...metricsWs}
                size="small"
                showDetails={true}
              />
            </Box>
          </Box>

        {error && (
          <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        {/* Overview Stats */}
        <Grid container spacing={3} sx={{ mb: 4 }}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Box>
                    <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                      Cluster Nodes
                    </Typography>
                    <Typography variant="h4" sx={{ fontWeight: 700 }}>
                      {metrics.nodes.ready}/{metrics.nodes.total}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Ready
                    </Typography>
                  </Box>
                  <Box sx={{ color: '#3f51b5' }}>
                    <ComputerIcon sx={{ fontSize: 40 }} />
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Box>
                    <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                      Active Sessions
                    </Typography>
                    <Typography variant="h4" sx={{ fontWeight: 700 }}>
                      {metrics.sessions.running}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {metrics.sessions.total} total
                    </Typography>
                  </Box>
                  <Box sx={{ color: '#4caf50' }}>
                    <AppsIcon sx={{ fontSize: 40 }} />
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Box>
                    <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                      Active Users
                    </Typography>
                    <Typography variant="h4" sx={{ fontWeight: 700 }}>
                      {metrics.users.active}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {metrics.users.total} total
                    </Typography>
                  </Box>
                  <Box sx={{ color: '#f50057' }}>
                    <PeopleIcon sx={{ fontSize: 40 }} />
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Box>
                    <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                      Hibernated Sessions
                    </Typography>
                    <Typography variant="h4" sx={{ fontWeight: 700 }}>
                      {metrics.sessions.hibernated}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      Scaled to zero
                    </Typography>
                  </Box>
                  <Box sx={{ color: '#ff9800' }}>
                    <StorageIcon sx={{ fontSize: 40 }} />
                  </Box>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Resource Utilization */}
        <Grid container spacing={3} sx={{ mb: 4 }}>
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                  CPU Utilization
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  <SpeedIcon sx={{ mr: 1, color: '#f50057' }} />
                  <Typography variant="body1">
                    {metrics.resources.cpu.used} / {metrics.resources.cpu.total}
                  </Typography>
                </Box>
                <LinearProgress
                  variant="determinate"
                  value={metrics.resources.cpu.percent}
                  sx={{
                    height: 10,
                    borderRadius: 5,
                    backgroundColor: 'rgba(245, 0, 87, 0.1)',
                    '& .MuiLinearProgress-bar': {
                      backgroundColor: metrics.resources.cpu.percent > 80 ? '#f44336' : '#f50057',
                    },
                  }}
                />
                <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
                  {metrics.resources.cpu.percent.toFixed(1)}% utilized
                </Typography>
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                  Memory Utilization
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'center', mb: 1 }}>
                  <MemoryIcon sx={{ mr: 1, color: '#ff9800' }} />
                  <Typography variant="body1">
                    {formatBytes(metrics.resources.memory.used)} / {formatBytes(metrics.resources.memory.total)}
                  </Typography>
                </Box>
                <LinearProgress
                  variant="determinate"
                  value={metrics.resources.memory.percent}
                  sx={{
                    height: 10,
                    borderRadius: 5,
                    backgroundColor: 'rgba(255, 152, 0, 0.1)',
                    '& .MuiLinearProgress-bar': {
                      backgroundColor: metrics.resources.memory.percent > 80 ? '#f44336' : '#ff9800',
                    },
                  }}
                />
                <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
                  {metrics.resources.memory.percent.toFixed(1)}% utilized
                </Typography>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Session Distribution */}
        <Grid container spacing={3} sx={{ mb: 4 }}>
          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                Session Distribution
              </Typography>
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Running
                  </Typography>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <LinearProgress
                      variant="determinate"
                      value={(metrics.sessions.running / metrics.sessions.total) * 100}
                      sx={{ width: 100, height: 8, borderRadius: 4 }}
                    />
                    <Chip label={metrics.sessions.running} color="success" size="small" />
                  </Box>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Hibernated
                  </Typography>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <LinearProgress
                      variant="determinate"
                      value={(metrics.sessions.hibernated / metrics.sessions.total) * 100}
                      sx={{ width: 100, height: 8, borderRadius: 4 }}
                      color="warning"
                    />
                    <Chip label={metrics.sessions.hibernated} color="warning" size="small" />
                  </Box>
                </Box>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                  <Typography variant="body2" color="text.secondary">
                    Terminated
                  </Typography>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <LinearProgress
                      variant="determinate"
                      value={(metrics.sessions.terminated / metrics.sessions.total) * 100}
                      sx={{ width: 100, height: 8, borderRadius: 4 }}
                      color="error"
                    />
                    <Chip label={metrics.sessions.terminated} size="small" />
                  </Box>
                </Box>
              </Box>
            </Paper>
          </Grid>

          <Grid item xs={12} md={6}>
            <Paper sx={{ p: 3 }}>
              <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
                Pod Capacity
              </Typography>
              <Box sx={{ textAlign: 'center', py: 2 }}>
                <Typography variant="h2" sx={{ fontWeight: 700, mb: 1 }}>
                  {metrics.resources.pods.used}
                </Typography>
                <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
                  of {metrics.resources.pods.total} pods used
                </Typography>
                <LinearProgress
                  variant="determinate"
                  value={metrics.resources.pods.percent}
                  sx={{
                    height: 12,
                    borderRadius: 6,
                    backgroundColor: 'rgba(63, 81, 181, 0.1)',
                    '& .MuiLinearProgress-bar': {
                      backgroundColor: metrics.resources.pods.percent > 80 ? '#f44336' : '#3f51b5',
                    },
                  }}
                />
                <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
                  {metrics.resources.pods.percent.toFixed(1)}% capacity
                </Typography>
              </Box>
            </Paper>
          </Grid>
        </Grid>

        {/* Recent Sessions */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" sx={{ mb: 2, fontWeight: 600 }}>
            Recent Sessions
          </Typography>
          <TableContainer>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Session Name</TableCell>
                  <TableCell>User</TableCell>
                  <TableCell>Template</TableCell>
                  <TableCell>State</TableCell>
                  <TableCell>Created</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {recentSessions.map((session) => (
                  <TableRow key={session.name}>
                    <TableCell>{session.name}</TableCell>
                    <TableCell>{session.user}</TableCell>
                    <TableCell>{session.template}</TableCell>
                    <TableCell>
                      <Chip
                        label={session.state}
                        size="small"
                        color={
                          session.state === 'running' ? 'success' :
                          session.state === 'hibernated' ? 'warning' : 'default'
                        }
                      />
                    </TableCell>
                    <TableCell>
                      {new Date(session.createdAt).toLocaleString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      </Box>
    </Layout>
    </WebSocketErrorBoundary>
  );
}

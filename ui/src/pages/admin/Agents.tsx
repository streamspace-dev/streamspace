import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
  Chip,
  Alert,
  CircularProgress,
  Paper,
  InputAdornment,
  MenuItem,
  Select,
  FormControl,
  InputLabel,
  Grid,
  Tooltip,
  Stack,
  Divider,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Delete as DeleteIcon,
  Search as SearchIcon,
  CheckCircle as OnlineIcon,
  Cancel as OfflineIcon,
  Warning as WarningIcon,
  Cloud as K8sIcon,
  Storage as DockerIcon,
  CloudQueue as VMIcon,
  CloudCircle as CloudIcon,
  Computer as AgentIcon,
  CheckCircleOutline as ApproveIcon,
  Block as RejectIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';
import { api } from '../../lib/api';
import { formatDistanceToNow } from 'date-fns';

/**
 * Agents - Platform agent management (v2.0)
 *
 * Administrative interface for managing distributed platform agents
 * in the v2.0 multi-platform architecture.
 *
 * Features:
 * - List all registered agents with real-time status
 * - View agent details and capacity
 * - Filter by platform, status, and region
 * - Remove agents
 * - Auto-refresh agent status
 * - Platform distribution charts
 *
 * Agent Platforms:
 * - kubernetes: Kubernetes cluster agent
 * - docker: Docker host agent
 * - vm: VM platform agent
 * - cloud: Cloud provider agent
 *
 * Agent Status:
 * - online: Last heartbeat < 30 seconds ago
 * - warning: Last heartbeat 30-60 seconds ago
 * - offline: Last heartbeat > 60 seconds ago
 *
 * @page
 * @route /admin/agents - Agent management
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Agent management interface
 */
export default function Agents() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  const [searchQuery, setSearchQuery] = useState('');
  const [platformFilter, setPlatformFilter] = useState<string>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [regionFilter, setRegionFilter] = useState<string>('all');
  const [approvalFilter, setApprovalFilter] = useState<string>('all');
  const [detailsDialogOpen, setDetailsDialogOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [approveConfirmOpen, setApproveConfirmOpen] = useState(false);
  const [rejectConfirmOpen, setRejectConfirmOpen] = useState(false);
  const [selectedAgent, setSelectedAgent] = useState<any>(null);

  // Fetch agents with filters
  const {
    data: agentsData,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ['agents', platformFilter, statusFilter, regionFilter],
    queryFn: async () => {
      const params: any = {};
      if (platformFilter !== 'all') params.platform = platformFilter;
      if (statusFilter !== 'all') params.status = statusFilter;
      if (regionFilter !== 'all') params.region = regionFilter;

      return await api.listAgents(params);
    },
    refetchInterval: 10000, // Auto-refresh every 10 seconds
  });

  const agents = agentsData?.agents || [];

  // Get unique regions for filter dropdown
  const regions = ['all', ...new Set(agents.map((a: any) => a.region).filter(Boolean))];

  // Delete agent mutation
  const deleteAgent = useMutation({
    mutationFn: async (agentId: string) => {
      await api.deleteAgent(agentId);
    },
    onSuccess: () => {
      addNotification({
        message: 'Agent removed successfully',
        severity: 'success',
      });
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      setDeleteConfirmOpen(false);
      setSelectedAgent(null);
    },
    onError: (error: any) => {
      addNotification({
        message: error.response?.data?.error || 'Failed to remove agent',
        severity: 'error',
      });
    },
  });

  // Approve agent mutation (Issue #234)
  const approveAgent = useMutation({
    mutationFn: async (agentId: string) => {
      await api.approveAgent(agentId);
    },
    onSuccess: (_, agentId) => {
      addNotification({
        message: `Agent ${agentId} approved successfully`,
        severity: 'success',
      });
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      setApproveConfirmOpen(false);
      setSelectedAgent(null);
    },
    onError: (error: any) => {
      addNotification({
        message: error.response?.data?.error || 'Failed to approve agent',
        severity: 'error',
      });
    },
  });

  // Reject agent mutation (Issue #234)
  const rejectAgent = useMutation({
    mutationFn: async (agentId: string) => {
      await api.rejectAgent(agentId);
    },
    onSuccess: (_, agentId) => {
      addNotification({
        message: `Agent ${agentId} rejected successfully`,
        severity: 'success',
      });
      queryClient.invalidateQueries({ queryKey: ['agents'] });
      setRejectConfirmOpen(false);
      setSelectedAgent(null);
    },
    onError: (error: any) => {
      addNotification({
        message: error.response?.data?.error || 'Failed to reject agent',
        severity: 'error',
      });
    },
  });

  // Filter agents by search query and approval status
  const filteredAgents = agents.filter((agent: any) => {
    const matchesSearch =
      searchQuery === '' ||
      agent.agentId.toLowerCase().includes(searchQuery.toLowerCase()) ||
      agent.platform?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      agent.region?.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesApproval =
      approvalFilter === 'all' ||
      (agent.approvalStatus || 'approved') === approvalFilter;

    return matchesSearch && matchesApproval;
  });

  // Get agent status based on last heartbeat
  const getAgentStatus = (lastHeartbeat: string) => {
    if (!lastHeartbeat) return 'offline';

    const heartbeatTime = new Date(lastHeartbeat).getTime();
    const now = Date.now();
    const secondsSinceHeartbeat = (now - heartbeatTime) / 1000;

    if (secondsSinceHeartbeat < 30) return 'online';
    if (secondsSinceHeartbeat < 60) return 'warning';
    return 'offline';
  };

  // Get platform icon
  const getPlatformIcon = (platform: string) => {
    switch (platform?.toLowerCase()) {
      case 'kubernetes':
        return <K8sIcon />;
      case 'docker':
        return <DockerIcon />;
      case 'vm':
        return <VMIcon />;
      case 'cloud':
        return <CloudIcon />;
      default:
        return <AgentIcon />;
    }
  };

  // Get status icon and color - uses database status field, not calculated
  const getStatusBadge = (agent: any) => {
    // Use status from database instead of calculating from lastHeartbeat
    const status = agent.status || 'offline';

    switch (status.toLowerCase()) {
      case 'online':
        return <Chip icon={<OnlineIcon />} label="Online" color="success" size="small" />;
      case 'draining':
        return <Chip icon={<WarningIcon />} label="Draining" color="warning" size="small" />;
      case 'offline':
        return <Chip icon={<OfflineIcon />} label="Offline" color="error" size="small" />;
      default:
        return <Chip label="Unknown" size="small" />;
    }
  };

  // Get approval status badge (Issue #234)
  const getApprovalBadge = (approvalStatus: string | undefined) => {
    const status = approvalStatus || 'approved'; // Default to approved for backward compatibility

    switch (status) {
      case 'approved':
        return <Chip label="Approved" color="success" size="small" />;
      case 'pending':
        return <Chip label="Pending Approval" color="warning" size="small" />;
      case 'rejected':
        return <Chip label="Rejected" color="error" size="small" />;
      default:
        return <Chip label={status} size="small" />;
    }
  };

  // Format time ago
  const getTimeAgo = (timestamp: string) => {
    if (!timestamp) return 'Never';
    return formatDistanceToNow(new Date(timestamp), { addSuffix: true });
  };

  // Calculate platform distribution
  const getPlatformDistribution = () => {
    const distribution: Record<string, number> = {};
    agents.forEach((agent: any) => {
      const platform = agent.platform || 'unknown';
      distribution[platform] = (distribution[platform] || 0) + 1;
    });
    return distribution;
  };

  // Get total sessions across all agents
  const getTotalSessions = () => {
    return agents.reduce((total: number, agent: any) => {
      return total + (agent.capacity?.active_sessions || 0);
    }, 0);
  };

  const handleViewDetails = (agent: any) => {
    setSelectedAgent(agent);
    setDetailsDialogOpen(true);
  };

  const handleDeleteClick = (agent: any) => {
    setSelectedAgent(agent);
    setDeleteConfirmOpen(true);
  };

  const handleDeleteConfirm = () => {
    if (selectedAgent) {
      deleteAgent.mutate(selectedAgent.agentId);
    }
  };

  // Issue #234: Agent approval handlers
  const handleApproveClick = (agent: any) => {
    setSelectedAgent(agent);
    setApproveConfirmOpen(true);
  };

  const handleApproveConfirm = () => {
    if (selectedAgent) {
      approveAgent.mutate(selectedAgent.agentId);
    }
  };

  const handleRejectClick = (agent: any) => {
    setSelectedAgent(agent);
    setRejectConfirmOpen(true);
  };

  const handleRejectConfirm = () => {
    if (selectedAgent) {
      rejectAgent.mutate(selectedAgent.agentId);
    }
  };

  return (
    <AdminPortalLayout title="Agent Management">
      <Container maxWidth="xl">
        <Box sx={{ mt: 3 }}>
          {/* Header */}
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 3 }}>
            <Typography variant="h4">Platform Agents</Typography>
            <Button
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={() => refetch()}
              disabled={isLoading}
            >
              Refresh
            </Button>
          </Stack>

          {/* Summary Cards */}
          <Grid container spacing={3} sx={{ mb: 3 }}>
            <Grid item xs={12} sm={6} md={3}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom variant="body2">
                    Total Agents
                  </Typography>
                  <Typography variant="h4">{agents.length}</Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom variant="body2">
                    Online Agents
                  </Typography>
                  <Typography variant="h4" color="success.main">
                    {agents.filter((a: any) => getAgentStatus(a.last_heartbeat) === 'online').length}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom variant="body2">
                    Total Sessions
                  </Typography>
                  <Typography variant="h4">{getTotalSessions()}</Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom variant="body2">
                    Platforms
                  </Typography>
                  <Typography variant="h4">{Object.keys(getPlatformDistribution()).length}</Typography>
                </CardContent>
              </Card>
            </Grid>
          </Grid>

          {/* Filters */}
          <Card sx={{ mb: 3 }}>
            <CardContent>
              <Grid container spacing={2} alignItems="center">
                <Grid item xs={12} sm={6} md={3}>
                  <TextField
                    fullWidth
                    placeholder="Search agents..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    InputProps={{
                      startAdornment: (
                        <InputAdornment position="start">
                          <SearchIcon />
                        </InputAdornment>
                      ),
                    }}
                  />
                </Grid>
                <Grid item xs={12} sm={6} md={2}>
                  <FormControl fullWidth>
                    <InputLabel>Platform</InputLabel>
                    <Select
                      value={platformFilter}
                      label="Platform"
                      onChange={(e) => setPlatformFilter(e.target.value)}
                    >
                      <MenuItem value="all">All Platforms</MenuItem>
                      <MenuItem value="kubernetes">Kubernetes</MenuItem>
                      <MenuItem value="docker">Docker</MenuItem>
                      <MenuItem value="vm">VM</MenuItem>
                      <MenuItem value="cloud">Cloud</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
                <Grid item xs={12} sm={6} md={2}>
                  <FormControl fullWidth>
                    <InputLabel>Status</InputLabel>
                    <Select
                      value={statusFilter}
                      label="Status"
                      onChange={(e) => setStatusFilter(e.target.value)}
                    >
                      <MenuItem value="all">All Status</MenuItem>
                      <MenuItem value="online">Online</MenuItem>
                      <MenuItem value="offline">Offline</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
                <Grid item xs={12} sm={6} md={2}>
                  <FormControl fullWidth>
                    <InputLabel>Region</InputLabel>
                    <Select
                      value={regionFilter}
                      label="Region"
                      onChange={(e) => setRegionFilter(e.target.value)}
                    >
                      {regions.map((region) => (
                        <MenuItem key={region} value={region}>
                          {region === 'all' ? 'All Regions' : region}
                        </MenuItem>
                      ))}
                    </Select>
                  </FormControl>
                </Grid>
                <Grid item xs={12} sm={6} md={3}>
                  <FormControl fullWidth>
                    <InputLabel>Approval Status</InputLabel>
                    <Select
                      value={approvalFilter}
                      label="Approval Status"
                      onChange={(e) => setApprovalFilter(e.target.value)}
                    >
                      <MenuItem value="all">All Statuses</MenuItem>
                      <MenuItem value="approved">Approved</MenuItem>
                      <MenuItem value="pending">Pending Approval</MenuItem>
                      <MenuItem value="rejected">Rejected</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
              </Grid>
            </CardContent>
          </Card>

          {/* Agents Table */}
          <Card>
            <CardContent>
              {error && (
                <Alert severity="error" sx={{ mb: 2 }}>
                  Failed to load agents: {(error as any).message}
                </Alert>
              )}

              {isLoading ? (
                <Box sx={{ display: 'flex', justifyContent: 'center', p: 4 }}>
                  <CircularProgress />
                </Box>
              ) : filteredAgents.length === 0 ? (
                <Box sx={{ textAlign: 'center', p: 4 }}>
                  <Typography color="textSecondary">No agents found</Typography>
                </Box>
              ) : (
                <TableContainer component={Paper}>
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableCell>Agent ID</TableCell>
                        <TableCell>Platform</TableCell>
                        <TableCell>Region</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>Approval</TableCell>
                        <TableCell>Sessions</TableCell>
                        <TableCell>Capacity</TableCell>
                        <TableCell>Last Heartbeat</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {filteredAgents.map((agent: any) => (
                        <TableRow
                          key={agent.agentId}
                          hover
                          sx={{ cursor: 'pointer' }}
                          onClick={() => handleViewDetails(agent)}
                        >
                          <TableCell>
                            <Typography variant="body2" fontFamily="monospace">
                              {agent.agentId}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Stack direction="row" spacing={1} alignItems="center">
                              {getPlatformIcon(agent.platform)}
                              <Typography variant="body2" sx={{ textTransform: 'capitalize' }}>
                                {agent.platform || 'Unknown'}
                              </Typography>
                            </Stack>
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2">{agent.region || 'N/A'}</Typography>
                          </TableCell>
                          <TableCell>{getStatusBadge(agent)}</TableCell>
                          <TableCell>{getApprovalBadge(agent.approvalStatus)}</TableCell>
                          <TableCell>
                            <Typography variant="body2">
                              {agent.capacity?.active_sessions || 0} /{' '}
                              {agent.capacity?.max_sessions || 'N/A'}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2" color="textSecondary">
                              CPU: {agent.capacity?.cpu || 'N/A'}
                              <br />
                              Memory: {agent.capacity?.memory || 'N/A'}
                            </Typography>
                          </TableCell>
                          <TableCell>
                            <Typography variant="body2" color="textSecondary">
                              {getTimeAgo(agent.lastHeartbeat)}
                            </Typography>
                          </TableCell>
                          <TableCell align="right">
                            <Stack direction="row" spacing={1} justifyContent="flex-end">
                              {(agent.approvalStatus || 'approved') === 'pending' && (
                                <>
                                  <Tooltip title="Approve Agent">
                                    <IconButton
                                      size="small"
                                      color="success"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        handleApproveClick(agent);
                                      }}
                                    >
                                      <ApproveIcon />
                                    </IconButton>
                                  </Tooltip>
                                  <Tooltip title="Reject Agent">
                                    <IconButton
                                      size="small"
                                      color="warning"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        handleRejectClick(agent);
                                      }}
                                    >
                                      <RejectIcon />
                                    </IconButton>
                                  </Tooltip>
                                </>
                              )}
                              <Tooltip title="Remove Agent">
                                <IconButton
                                  size="small"
                                  color="error"
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    handleDeleteClick(agent);
                                  }}
                                >
                                  <DeleteIcon />
                                </IconButton>
                              </Tooltip>
                            </Stack>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </CardContent>
          </Card>
        </Box>

        {/* Agent Details Dialog */}
        <Dialog
          open={detailsDialogOpen}
          onClose={() => setDetailsDialogOpen(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            Agent Details
            {selectedAgent && (
              <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
                {selectedAgent.agent_id}
              </Typography>
            )}
          </DialogTitle>
          <DialogContent>
            {selectedAgent && (
              <Box>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Platform
                    </Typography>
                    <Stack direction="row" spacing={1} alignItems="center" sx={{ mt: 0.5 }}>
                      {getPlatformIcon(selectedAgent.platform)}
                      <Typography sx={{ textTransform: 'capitalize' }}>
                        {selectedAgent.platform}
                      </Typography>
                    </Stack>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Region
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>{selectedAgent.region || 'Not specified'}</Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Status
                    </Typography>
                    <Box sx={{ mt: 0.5 }}>{getStatusBadge(selectedAgent)}</Box>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Last Heartbeat
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>{getTimeAgo(selectedAgent.last_heartbeat)}</Typography>
                  </Grid>
                </Grid>

                <Divider sx={{ my: 2 }} />

                <Typography variant="h6" gutterBottom>
                  Capacity
                </Typography>
                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Max Sessions
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>
                      {selectedAgent.capacity?.max_sessions || 'Not specified'}
                    </Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Active Sessions
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>
                      {selectedAgent.capacity?.active_sessions || 0}
                    </Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      CPU
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>{selectedAgent.capacity?.cpu || 'Not specified'}</Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Memory
                    </Typography>
                    <Typography sx={{ mt: 0.5 }}>
                      {selectedAgent.capacity?.memory || 'Not specified'}
                    </Typography>
                  </Grid>
                </Grid>

                {selectedAgent.metadata && Object.keys(selectedAgent.metadata).length > 0 && (
                  <>
                    <Divider sx={{ my: 2 }} />
                    <Typography variant="h6" gutterBottom>
                      Metadata
                    </Typography>
                    <Paper variant="outlined" sx={{ p: 2, bgcolor: 'grey.50' }}>
                      <pre style={{ margin: 0, fontSize: '0.875rem', overflow: 'auto' }}>
                        {JSON.stringify(selectedAgent.metadata, null, 2)}
                      </pre>
                    </Paper>
                  </>
                )}

                <Divider sx={{ my: 2 }} />

                <Grid container spacing={2}>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Created At
                    </Typography>
                    <Typography variant="body2" sx={{ mt: 0.5 }}>
                      {new Date(selectedAgent.created_at).toLocaleString()}
                    </Typography>
                  </Grid>
                  <Grid item xs={12} sm={6}>
                    <Typography variant="subtitle2" color="textSecondary">
                      Updated At
                    </Typography>
                    <Typography variant="body2" sx={{ mt: 0.5 }}>
                      {new Date(selectedAgent.updated_at).toLocaleString()}
                    </Typography>
                  </Grid>
                </Grid>
              </Box>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDetailsDialogOpen(false)}>Close</Button>
          </DialogActions>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)}>
          <DialogTitle>Confirm Agent Removal</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to remove agent <strong>{selectedAgent?.agent_id}</strong>?
            </Typography>
            <Alert severity="warning" sx={{ mt: 2 }}>
              This will permanently remove the agent from the system. Any sessions running on this agent
              may be affected.
            </Alert>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteConfirmOpen(false)}>Cancel</Button>
            <Button
              onClick={handleDeleteConfirm}
              color="error"
              variant="contained"
              disabled={deleteAgent.isPending}
            >
              {deleteAgent.isPending ? 'Removing...' : 'Remove'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Approve Confirmation Dialog (Issue #234) */}
        <Dialog open={approveConfirmOpen} onClose={() => setApproveConfirmOpen(false)}>
          <DialogTitle>Approve Agent</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to approve agent <strong>{selectedAgent?.agent_id}</strong>?
            </Typography>
            <Alert severity="info" sx={{ mt: 2 }}>
              The agent will be granted access to the platform and will be able to manage sessions.
            </Alert>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setApproveConfirmOpen(false)}>Cancel</Button>
            <Button
              onClick={handleApproveConfirm}
              color="success"
              variant="contained"
              disabled={approveAgent.isPending}
            >
              {approveAgent.isPending ? 'Approving...' : 'Approve'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Reject Confirmation Dialog (Issue #234) */}
        <Dialog open={rejectConfirmOpen} onClose={() => setRejectConfirmOpen(false)}>
          <DialogTitle>Reject Agent</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to reject agent <strong>{selectedAgent?.agent_id}</strong>?
            </Typography>
            <Alert severity="warning" sx={{ mt: 2 }}>
              The agent will be denied access to the platform and will not be able to manage sessions.
            </Alert>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setRejectConfirmOpen(false)}>Cancel</Button>
            <Button
              onClick={handleRejectConfirm}
              color="warning"
              variant="contained"
              disabled={rejectAgent.isPending}
            >
              {rejectAgent.isPending ? 'Rejecting...' : 'Reject'}
            </Button>
          </DialogActions>
        </Dialog>
      </Container>
    </AdminPortalLayout>
  );
}

import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Tabs,
  Tab,
  Card,
  CardContent,
  Button,
  Chip,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Grid,
  LinearProgress,
  Alert,
  Paper,
  Snackbar,
} from '@mui/material';
import {
  CloudQueue as CloudIcon,
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  TrendingUp as ScaleUpIcon,
  TrendingDown as ScaleDownIcon,
  Computer as NodeIcon,
  Speed as PerformanceIcon,
  Wifi as ConnectedIcon,
  WifiOff as DisconnectedIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import api from '../../lib/api';
import { toast } from '../../lib/toast';
import { useScalingEvents } from '../../hooks/useEnterpriseWebSocket';

interface LoadBalancingPolicy {
  id: number;
  name: string;
  strategy: string;
  enabled: boolean;
  session_affinity: boolean;
  created_at: string;
}

interface NodeStatus {
  node_name: string;
  status: string;
  cpu_percent: number;
  memory_percent: number;
  active_sessions: number;
  health_status: string;
  region?: string;
}

interface AutoScalingPolicy {
  id: number;
  name: string;
  target_type: string;
  target_id: string;
  scaling_mode: string;
  min_replicas: number;
  max_replicas: number;
  metric_type: string;
  enabled: boolean;
}

interface ScalingEvent {
  id: number;
  policy_id: number;
  action: string;
  previous_replicas: number;
  new_replicas: number;
  trigger: string;
  created_at: string;
}

export default function Scaling() {
  const [currentTab, setCurrentTab] = useState(0);
  const [lbPolicies, setLbPolicies] = useState<LoadBalancingPolicy[]>([]);
  const [nodes, setNodes] = useState<NodeStatus[]>([]);
  const [asPolicies, setAsPolicies] = useState<AutoScalingPolicy[]>([]);
  const [scalingHistory, setScalingHistory] = useState<ScalingEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);
  const [scalingEventNotification, setScalingEventNotification] = useState<string | null>(null);

  const [lbDialog, setLbDialog] = useState(false);
  const [asDialog, setAsDialog] = useState(false);

  // Real-time scaling events via WebSocket
  useScalingEvents((data: any) => {
    console.log('Scaling event:', data);
    setWsConnected(true);

    // Show notification for scaling events
    if (data.action && data.policy_name) {
      setScalingEventNotification(
        `Scaling ${data.action}: ${data.policy_name} (${data.previous_replicas} → ${data.new_replicas} replicas)`
      );
    }

    // Refresh data when we receive a scaling event
    loadAllData();
  });

  const [lbForm, setLbForm] = useState({
    name: '',
    strategy: 'round_robin',
    session_affinity: false,
  });

  const [asForm, setAsForm] = useState({
    name: '',
    target_type: 'template',
    target_id: '',
    scaling_mode: 'horizontal',
    min_replicas: 1,
    max_replicas: 10,
    metric_type: 'cpu',
    target_metric_value: 70,
  });

  // Load initial data
  useEffect(() => {
    loadLBPolicies();
    loadNodes();
    loadASPolicies();
    loadScalingHistory();
  }, []);

  const loadLBPolicies = async () => {
    try {
      const response = await api.listLoadBalancingPolicies();
      setLbPolicies(response.policies);
    } catch (error) {
      console.error('Failed to load load balancing policies:', error);
    }
  };

  const loadNodes = async () => {
    try {
      const response = await api.getNodeStatus();
      setNodes(response.nodes);
    } catch (error) {
      console.error('Failed to load nodes:', error);
    }
  };

  const loadASPolicies = async () => {
    try {
      const response = await api.listAutoScalingPolicies();
      setAsPolicies(response.policies);
    } catch (error) {
      console.error('Failed to load auto-scaling policies:', error);
    }
  };

  const loadScalingHistory = async () => {
    try {
      const response = await api.getScalingHistory();
      setScalingHistory(response.events);
    } catch (error) {
      console.error('Failed to load scaling history:', error);
    }
  };

  const handleCreateLBPolicy = async () => {
    setLoading(true);
    try {
      await api.createLoadBalancingPolicy({
        name: lbForm.name,
        strategy: lbForm.strategy,
        session_affinity: lbForm.session_affinity,
      });
      toast.success('Load balancing policy created');
      setLbDialog(false);
      setLbForm({ name: '', strategy: 'round_robin', session_affinity: false });
      loadLBPolicies();
    } catch (error) {
      toast.error('Failed to create load balancing policy');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateASPolicy = async () => {
    setLoading(true);
    try {
      await api.createAutoScalingPolicy({
        name: asForm.name,
        target_type: asForm.target_type,
        target_id: asForm.target_id,
        scaling_mode: asForm.scaling_mode,
        min_replicas: asForm.min_replicas,
        max_replicas: asForm.max_replicas,
        metric_type: asForm.metric_type,
        target_metric_value: asForm.target_metric_value,
      });
      toast.success('Auto-scaling policy created');
      setAsDialog(false);
      setAsForm({
        name: '',
        target_type: 'template',
        target_id: '',
        scaling_mode: 'horizontal',
        min_replicas: 1,
        max_replicas: 10,
        metric_type: 'cpu',
        target_metric_value: 70,
      });
      loadASPolicies();
    } catch (error) {
      toast.error('Failed to create auto-scaling policy');
    } finally {
      setLoading(false);
    }
  };

  const handleTriggerScaling = async (policyId: number, action: 'scale_up' | 'scale_down') => {
    setLoading(true);
    try {
      await api.triggerScaling(policyId, { action });
      toast.success(`Scaling ${action === 'scale_up' ? 'up' : 'down'} triggered`);
      loadScalingHistory();
    } catch (error) {
      toast.error('Failed to trigger scaling');
    } finally {
      setLoading(false);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'ready':
      case 'healthy':
        return 'success';
      case 'not_ready':
      case 'unhealthy':
        return 'error';
      default:
        return 'warning';
    }
  };

  const getProgressColor = (percent: number) => {
    if (percent < 70) return 'success';
    if (percent < 85) return 'warning';
    return 'error';
  };

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Load Balancing & Auto-scaling
            </Typography>
            <Chip
              icon={wsConnected ? <ConnectedIcon /> : <DisconnectedIcon />}
              label={wsConnected ? 'Live Events' : 'Polling'}
              size="small"
              color={wsConnected ? 'success' : 'default'}
            />
          </Box>
        </Box>

        <Tabs value={currentTab} onChange={(_, v) => setCurrentTab(v)} sx={{ mb: 3 }}>
          <Tab label="Node Status" />
          <Tab label="Load Balancing" />
          <Tab label="Auto-scaling" />
          <Tab label="Scaling History" />
        </Tabs>

        {/* Node Status Tab */}
        {currentTab === 0 && (
          <Box>
            <Grid container spacing={3} sx={{ mb: 3 }}>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Total Nodes</Typography>
                    <Typography variant="h3">{nodes.length}</Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Healthy Nodes</Typography>
                    <Typography variant="h3" color="success.main">
                      {nodes.filter((n) => n.health_status === 'healthy').length}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Avg CPU</Typography>
                    <Typography variant="h3">
                      {nodes.length > 0
                        ? Math.round(nodes.reduce((sum, n) => sum + n.cpu_percent, 0) / nodes.length)
                        : 0}
                      %
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Active Sessions</Typography>
                    <Typography variant="h3">
                      {nodes.reduce((sum, n) => sum + n.active_sessions, 0)}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>

            <Card>
              <CardContent>
                <Typography variant="h6" sx={{ mb: 2 }}>
                  Node Details
                </Typography>
                <TableContainer>
                  <Table>
                    <TableHead>
                      <TableRow>
                        <TableCell>Node</TableCell>
                        <TableCell>Status</TableCell>
                        <TableCell>CPU</TableCell>
                        <TableCell>Memory</TableCell>
                        <TableCell>Sessions</TableCell>
                        <TableCell>Region</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {nodes.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={6} align="center">
                            <Typography color="text.secondary">No nodes available</Typography>
                          </TableCell>
                        </TableRow>
                      ) : (
                        nodes.map((node) => (
                          <TableRow key={node.node_name}>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                                <NodeIcon sx={{ mr: 1 }} />
                                {node.node_name}
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Chip
                                label={node.health_status}
                                color={getStatusColor(node.health_status)}
                                size="small"
                              />
                            </TableCell>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Box sx={{ flexGrow: 1 }}>
                                  <LinearProgress
                                    variant="determinate"
                                    value={node.cpu_percent}
                                    color={getProgressColor(node.cpu_percent)}
                                  />
                                </Box>
                                <Typography variant="body2">{Math.round(node.cpu_percent)}%</Typography>
                              </Box>
                            </TableCell>
                            <TableCell>
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Box sx={{ flexGrow: 1 }}>
                                  <LinearProgress
                                    variant="determinate"
                                    value={node.memory_percent}
                                    color={getProgressColor(node.memory_percent)}
                                  />
                                </Box>
                                <Typography variant="body2">{Math.round(node.memory_percent)}%</Typography>
                              </Box>
                            </TableCell>
                            <TableCell>{node.active_sessions}</TableCell>
                            <TableCell>{node.region || '-'}</TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </TableContainer>
              </CardContent>
            </Card>
          </Box>
        )}

        {/* Load Balancing Tab */}
        {currentTab === 1 && (
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Load Balancing Policies</Typography>
                <Button variant="contained" startIcon={<AddIcon />} onClick={() => setLbDialog(true)}>
                  New Policy
                </Button>
              </Box>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Strategy</TableCell>
                      <TableCell>Session Affinity</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {lbPolicies.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} align="center">
                          <Typography color="text.secondary">No load balancing policies</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      lbPolicies.map((policy) => (
                        <TableRow key={policy.id}>
                          <TableCell>{policy.name}</TableCell>
                          <TableCell>
                            <Chip label={policy.strategy.replace('_', ' ')} size="small" />
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={policy.session_affinity ? 'Yes' : 'No'}
                              color={policy.session_affinity ? 'primary' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={policy.enabled ? 'Enabled' : 'Disabled'}
                              color={policy.enabled ? 'success' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <IconButton size="small">
                              <EditIcon />
                            </IconButton>
                            <IconButton size="small">
                              <DeleteIcon />
                            </IconButton>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        )}

        {/* Auto-scaling Tab */}
        {currentTab === 2 && (
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Auto-scaling Policies</Typography>
                <Button variant="contained" startIcon={<AddIcon />} onClick={() => setAsDialog(true)}>
                  New Policy
                </Button>
              </Box>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Target</TableCell>
                      <TableCell>Mode</TableCell>
                      <TableCell>Replicas</TableCell>
                      <TableCell>Metric</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {asPolicies.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={7} align="center">
                          <Typography color="text.secondary">No auto-scaling policies</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      asPolicies.map((policy) => (
                        <TableRow key={policy.id}>
                          <TableCell>{policy.name}</TableCell>
                          <TableCell>
                            {policy.target_type}: {policy.target_id}
                          </TableCell>
                          <TableCell>
                            <Chip label={policy.scaling_mode} size="small" />
                          </TableCell>
                          <TableCell>
                            {policy.min_replicas} - {policy.max_replicas}
                          </TableCell>
                          <TableCell>{policy.metric_type}</TableCell>
                          <TableCell>
                            <Chip
                              label={policy.enabled ? 'Enabled' : 'Disabled'}
                              color={policy.enabled ? 'success' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <IconButton size="small" onClick={() => handleTriggerScaling(policy.id, 'scale_up')}>
                              <ScaleUpIcon />
                            </IconButton>
                            <IconButton size="small" onClick={() => handleTriggerScaling(policy.id, 'scale_down')}>
                              <ScaleDownIcon />
                            </IconButton>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        )}

        {/* Scaling History Tab */}
        {currentTab === 3 && (
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Recent Scaling Events
              </Typography>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Time</TableCell>
                      <TableCell>Policy</TableCell>
                      <TableCell>Action</TableCell>
                      <TableCell>Replicas</TableCell>
                      <TableCell>Trigger</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {scalingHistory.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} align="center">
                          <Typography color="text.secondary">No scaling events</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      scalingHistory.map((event) => (
                        <TableRow key={event.id}>
                          <TableCell>{new Date(event.created_at).toLocaleString()}</TableCell>
                          <TableCell>Policy #{event.policy_id}</TableCell>
                          <TableCell>
                            <Chip
                              label={event.action}
                              color={event.action === 'scale_up' ? 'success' : 'warning'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            {event.previous_replicas} → {event.new_replicas}
                          </TableCell>
                          <TableCell>
                            <Chip label={event.trigger} size="small" />
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        )}

        {/* Create LB Policy Dialog */}
        <Dialog open={lbDialog} onClose={() => setLbDialog(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Create Load Balancing Policy</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <TextField
                label="Name"
                fullWidth
                value={lbForm.name}
                onChange={(e) => setLbForm({ ...lbForm, name: e.target.value })}
              />
              <FormControl fullWidth>
                <InputLabel>Strategy</InputLabel>
                <Select
                  value={lbForm.strategy}
                  onChange={(e) => setLbForm({ ...lbForm, strategy: e.target.value })}
                >
                  <MenuItem value="round_robin">Round Robin</MenuItem>
                  <MenuItem value="least_loaded">Least Loaded</MenuItem>
                  <MenuItem value="resource_based">Resource Based</MenuItem>
                  <MenuItem value="geographic">Geographic</MenuItem>
                  <MenuItem value="weighted">Weighted</MenuItem>
                </Select>
              </FormControl>
              <FormControl fullWidth>
                <InputLabel>Session Affinity</InputLabel>
                <Select
                  value={lbForm.session_affinity ? 'yes' : 'no'}
                  onChange={(e) => setLbForm({ ...lbForm, session_affinity: e.target.value === 'yes' })}
                >
                  <MenuItem value="yes">Yes (Sticky Sessions)</MenuItem>
                  <MenuItem value="no">No</MenuItem>
                </Select>
              </FormControl>
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setLbDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleCreateLBPolicy}>
              Create
            </Button>
          </DialogActions>
        </Dialog>

        {/* Create AS Policy Dialog */}
        <Dialog open={asDialog} onClose={() => setAsDialog(false)} maxWidth="md" fullWidth>
          <DialogTitle>Create Auto-scaling Policy</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <TextField
                label="Name"
                fullWidth
                value={asForm.name}
                onChange={(e) => setAsForm({ ...asForm, name: e.target.value })}
              />
              <Grid container spacing={2}>
                <Grid item xs={6}>
                  <FormControl fullWidth>
                    <InputLabel>Target Type</InputLabel>
                    <Select
                      value={asForm.target_type}
                      onChange={(e) => setAsForm({ ...asForm, target_type: e.target.value })}
                    >
                      <MenuItem value="template">Template</MenuItem>
                      <MenuItem value="deployment">Deployment</MenuItem>
                    </Select>
                  </FormControl>
                </Grid>
                <Grid item xs={6}>
                  <TextField
                    label="Target ID"
                    fullWidth
                    value={asForm.target_id}
                    onChange={(e) => setAsForm({ ...asForm, target_id: e.target.value })}
                  />
                </Grid>
              </Grid>
              <FormControl fullWidth>
                <InputLabel>Scaling Mode</InputLabel>
                <Select
                  value={asForm.scaling_mode}
                  onChange={(e) => setAsForm({ ...asForm, scaling_mode: e.target.value })}
                >
                  <MenuItem value="horizontal">Horizontal (Replicas)</MenuItem>
                  <MenuItem value="vertical">Vertical (Resources)</MenuItem>
                </Select>
              </FormControl>
              <Grid container spacing={2}>
                <Grid item xs={6}>
                  <TextField
                    label="Min Replicas"
                    type="number"
                    fullWidth
                    value={asForm.min_replicas}
                    onChange={(e) => setAsForm({ ...asForm, min_replicas: parseInt(e.target.value) })}
                  />
                </Grid>
                <Grid item xs={6}>
                  <TextField
                    label="Max Replicas"
                    type="number"
                    fullWidth
                    value={asForm.max_replicas}
                    onChange={(e) => setAsForm({ ...asForm, max_replicas: parseInt(e.target.value) })}
                  />
                </Grid>
              </Grid>
              <FormControl fullWidth>
                <InputLabel>Metric Type</InputLabel>
                <Select
                  value={asForm.metric_type}
                  onChange={(e) => setAsForm({ ...asForm, metric_type: e.target.value })}
                >
                  <MenuItem value="cpu">CPU Utilization</MenuItem>
                  <MenuItem value="memory">Memory Utilization</MenuItem>
                  <MenuItem value="custom">Custom Metric</MenuItem>
                </Select>
              </FormControl>
              <TextField
                label="Target Metric Value (%)"
                type="number"
                fullWidth
                value={asForm.target_metric_value}
                onChange={(e) => setAsForm({ ...asForm, target_metric_value: parseInt(e.target.value) })}
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setAsDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleCreateASPolicy}>
              Create
            </Button>
          </DialogActions>
        </Dialog>

        {/* Scaling Event Notification */}
        <Snackbar
          open={!!scalingEventNotification}
          autoHideDuration={6000}
          onClose={() => setScalingEventNotification(null)}
          message={scalingEventNotification}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        />
      </Box>
    </Layout>
  );
}

/**
 * Nodes Admin Page
 *
 * Provides comprehensive Kubernetes node management for StreamSpace administrators.
 *
 * Features:
 * - View all cluster nodes with health status (Ready, NotReady)
 * - Monitor node resources (CPU, memory, pods) and utilization
 * - Add/remove node labels for session pod scheduling
 * - Add/remove node taints for workload isolation
 * - Cordon nodes to prevent new pod scheduling
 * - Uncordon nodes to resume scheduling
 * - Drain nodes to evict pods for maintenance
 * - Real-time updates via WebSocket (node health events)
 * - Cluster-wide statistics (total capacity, allocatable resources)
 *
 * Node Operations:
 * - Label Management: Click "Manage Labels" to add labels like `gpu=true`
 * - Taint Management: Click "Manage Taints" to add taints (NoSchedule, PreferNoSchedule, NoExecute)
 * - Cordon: Mark node as unschedulable (existing pods continue running)
 * - Drain: Evict all pods gracefully with configurable grace period
 *
 * Typical Workflow (Node Maintenance):
 * 1. Cordon node to prevent new sessions from scheduling
 * 2. Drain node to move existing sessions to other nodes
 * 3. Perform maintenance (OS updates, hardware changes, etc.)
 * 4. Uncordon node to resume normal scheduling
 *
 * Real-time Updates:
 * Subscribes to `node.health` WebSocket events for live node status changes.
 *
 * @component
 */
import { useState, useEffect, useRef } from 'react';
import {
  Box,
  Typography,
  Card,
  CardContent,
  Grid,
  Chip,
  Button,

  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Alert,

  CircularProgress,
} from '@mui/material';
import {
  Computer as ComputerIcon,
  Memory as MemoryIcon,
  Speed as SpeedIcon,
  Block as BlockIcon,
  CheckCircle as CheckCircleIcon,
  Warning as WarningIcon,
  Error as ErrorIcon,
  Label as LabelIcon,
  LocalOffer as TaintIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import { api } from '../../lib/api';
import { useNodeHealthEvents } from '../../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../../components/NotificationQueue';
import EnhancedWebSocketStatus from '../../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../../components/WebSocketErrorBoundary';

interface NodeInfo {
  name: string;
  labels: Record<string, string>;
  taints: Array<{ key: string; value: string; effect: string }>;
  status: string;
  capacity: {
    cpu: string;
    memory: string;
    pods: string;
    'nvidia.com/gpu'?: string;
  };
  allocatable: {
    cpu: string;
    memory: string;
    pods: string;
    'nvidia.com/gpu'?: string;
  };
  usage?: {
    cpu: string;
    memory: string;
    cpuPercent: number;
    memoryPercent: number;
  };
  info: {
    osImage: string;
    kernelVersion: string;
    kubeletVersion: string;
    containerRuntime: string;
  };
  conditions: Array<{
    type: string;
    status: string;
    message: string;
  }>;
  pods: number;
  age: string;
  provider?: string;
  region?: string;
  zone?: string;
}

interface ClusterStats {
  totalNodes: number;
  readyNodes: number;
  notReadyNodes: number;
  totalCapacity: {
    cpu: string;
    memory: string;
    pods: number;
  };
  totalAllocatable: {
    cpu: string;
    memory: string;
    pods: number;
  };
  totalUsage?: {
    cpu: string;
    memory: string;
    cpuPercent: number;
    memoryPercent: number;
  };
}

export default function AdminNodes() {
  const [nodes, setNodes] = useState<NodeInfo[]>([]);
  const [stats, setStats] = useState<ClusterStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedNode, setSelectedNode] = useState<NodeInfo | null>(null);

  // Dialog states
  const [labelDialogOpen, setLabelDialogOpen] = useState(false);
  const [taintDialogOpen, setTaintDialogOpen] = useState(false);
  const [labelKey, setLabelKey] = useState('');
  const [labelValue, setLabelValue] = useState('');
  const [taintKey, setTaintKey] = useState('');
  const [taintValue, setTaintValue] = useState('');
  const [taintEffect, setTaintEffect] = useState<string>('NoSchedule');

  // Track WebSocket connection state and node health statuses
  const [wsConnected, setWsConnected] = useState(false);
  const prevNodeStatusesRef = useRef<Map<string, string>>(new Map());

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time node health updates via WebSocket with notifications
  const baseWebSocket = useNodeHealthEvents((data: any) => {
    console.log('Node health event:', data);
    setWsConnected(true);

    // Show notification for node health changes
    if (data.node_name && data.status) {
      const prevStatus = prevNodeStatusesRef.current.get(data.node_name);

      if (prevStatus && prevStatus !== data.status) {
        addNotification({
          message: `Node ${data.node_name}: ${prevStatus} → ${data.status}`,
          severity: data.status === 'Ready' ? 'success' : data.status === 'NotReady' ? 'error' : 'warning',
          priority: data.status === 'NotReady' ? 'high' : 'medium',
          title: 'Node Status Changed',
        });
      }

      prevNodeStatusesRef.current.set(data.node_name, data.status);
    }

    // Show critical alerts for node failures
    if (data.status === 'NotReady' || data.event_type === 'node_failure') {
      addNotification({
        message: `Node ${data.node_name} is experiencing issues: ${data.message || 'Not Ready'}`,
        severity: 'error',
        priority: 'critical',
        title: 'Node Health Alert',
        duration: null, // Don't auto-dismiss critical alerts
      });
    }

    // Refresh node data when we receive a health update
    if (data.node_name) {
      loadNodesAndStats();
    }
  });

  useEffect(() => {
    loadNodesAndStats();
  }, []);

  const loadNodesAndStats = async () => {
    setLoading(true);
    setError('');

    try {
      const [nodesData, statsData] = await Promise.all([
        api.listNodes().catch(() => []), // Return empty array if API not implemented
        api.getClusterStats().catch(() => null), // Return null if API not implemented
      ]);

      // Ensure nodesData is always an array to prevent undefined errors
      setNodes(Array.isArray(nodesData) ? nodesData : []);
      setStats(statsData || null);
    } catch (err: any) {
      console.error('Failed to load nodes:', err);
      setError(err.response?.data?.message || 'Failed to load node information');
      // Set empty array on error to prevent undefined
      setNodes([]);
      setStats(null);
    } finally {
      setLoading(false);
    }
  };

  const handleAddLabel = async () => {
    if (!selectedNode || !labelKey || !labelValue) return;

    try {
      await api.addNodeLabel(selectedNode.name, labelKey, labelValue);
      addNotification({
        message: `Label ${labelKey}=${labelValue} added to ${selectedNode.name}`,
        severity: 'success',
        priority: 'medium',
        title: 'Label Added',
      });
      setLabelDialogOpen(false);
      setLabelKey('');
      setLabelValue('');
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to add label:', err);
      const errorMsg = err.response?.data?.message || 'Failed to add label';
      setError(errorMsg);
      addNotification({
        message: errorMsg,
        severity: 'error',
        priority: 'high',
        title: 'Label Addition Failed',
      });
    }
  };

  const handleRemoveLabel = async (nodeName: string, key: string) => {
    try {
      await api.removeNodeLabel(nodeName, key);
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to remove label:', err);
      setError(err.response?.data?.message || 'Failed to remove label');
    }
  };

  const handleAddTaint = async () => {
    if (!selectedNode || !taintKey || !taintValue) return;

    try {
      await api.addNodeTaint(selectedNode.name, {
        key: taintKey,
        value: taintValue,
        effect: taintEffect,
      });
      setTaintDialogOpen(false);
      setTaintKey('');
      setTaintValue('');
      setTaintEffect('NoSchedule');
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to add taint:', err);
      setError(err.response?.data?.message || 'Failed to add taint');
    }
  };

  const handleRemoveTaint = async (nodeName: string, key: string) => {
    try {
      await api.removeNodeTaint(nodeName, key);
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to remove taint:', err);
      setError(err.response?.data?.message || 'Failed to remove taint');
    }
  };

  const handleCordon = async (nodeName: string) => {
    try {
      await api.cordonNode(nodeName);
      addNotification({
        message: `Node ${nodeName} has been cordoned (unschedulable)`,
        severity: 'warning',
        priority: 'medium',
        title: 'Node Cordoned',
      });
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to cordon node:', err);
      const errorMsg = err.response?.data?.message || 'Failed to cordon node';
      setError(errorMsg);
      addNotification({
        message: errorMsg,
        severity: 'error',
        priority: 'high',
        title: 'Cordon Failed',
      });
    }
  };

  const handleUncordon = async (nodeName: string) => {
    try {
      await api.uncordonNode(nodeName);
      addNotification({
        message: `Node ${nodeName} has been uncordoned (schedulable)`,
        severity: 'success',
        priority: 'medium',
        title: 'Node Uncordoned',
      });
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to uncordon node:', err);
      const errorMsg = err.response?.data?.message || 'Failed to uncordon node';
      setError(errorMsg);
      addNotification({
        message: errorMsg,
        severity: 'error',
        priority: 'high',
        title: 'Uncordon Failed',
      });
    }
  };

  const handleDrain = async (nodeName: string) => {
    if (!window.confirm('Are you sure you want to drain this node? All pods will be evicted.')) {
      return;
    }

    try {
      addNotification({
        message: `Draining node ${nodeName}... This may take a few minutes.`,
        severity: 'info',
        priority: 'high',
        title: 'Draining Node',
      });

      await api.drainNode(nodeName, 30);

      addNotification({
        message: `Node ${nodeName} has been drained successfully`,
        severity: 'success',
        priority: 'high',
        title: 'Node Drained',
      });
      loadNodesAndStats();
    } catch (err: any) {
      console.error('Failed to drain node:', err);
      const errorMsg = err.response?.data?.message || 'Failed to drain node';
      setError(errorMsg);
      addNotification({
        message: errorMsg,
        severity: 'error',
        priority: 'critical',
        title: 'Drain Failed',
      });
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'Ready':
        return <CheckCircleIcon color="success" />;
      case 'NotReady':
        return <ErrorIcon color="error" />;
      default:
        return <WarningIcon color="warning" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'Ready':
        return 'success';
      case 'NotReady':
        return 'error';
      default:
        return 'warning';
    }
  };

  const isNodeSchedulable = (node: NodeInfo) => {
    return !node.taints.some(t => t.effect === 'NoSchedule' || t.effect === 'NoExecute');
  };

  if (loading) {
    return (
      <Layout>
        <Box display="flex" justifyContent="center" alignItems="center" minHeight="60vh">
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
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <Typography variant="h4" sx={{ fontWeight: 700 }}>
                Cluster Nodes
              </Typography>

              {/* Enhanced WebSocket Connection Status */}
              <EnhancedWebSocketStatus
                isConnected={wsConnected}
                reconnectAttempts={0}
                size="small"
                showDetails={true}
              />
            </Box>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={loadNodesAndStats}
          >
            Refresh
          </Button>
        </Box>

        {error && (
          <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        {stats && (
          <Grid container spacing={3} sx={{ mb: 4 }}>
            <Grid item xs={12} sm={6} md={3}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <Box>
                      <Typography color="text.secondary" variant="body2" sx={{ mb: 1 }}>
                        Total Nodes
                      </Typography>
                      <Typography variant="h4" sx={{ fontWeight: 700 }}>
                        {stats.totalNodes}
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
                        Ready Nodes
                      </Typography>
                      <Typography variant="h4" sx={{ fontWeight: 700 }}>
                        {stats.readyNodes}
                      </Typography>
                    </Box>
                    <Box sx={{ color: '#4caf50' }}>
                      <CheckCircleIcon sx={{ fontSize: 40 }} />
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
                        Total CPU
                      </Typography>
                      <Typography variant="h4" sx={{ fontWeight: 700 }}>
                        {stats.totalCapacity.cpu}
                      </Typography>
                      {stats.totalUsage && (
                        <Typography variant="caption" color="text.secondary">
                          {stats.totalUsage.cpuPercent.toFixed(1)}% used
                        </Typography>
                      )}
                    </Box>
                    <Box sx={{ color: '#f50057' }}>
                      <SpeedIcon sx={{ fontSize: 40 }} />
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
                        Total Memory
                      </Typography>
                      <Typography variant="h4" sx={{ fontWeight: 700 }}>
                        {stats.totalCapacity.memory}
                      </Typography>
                      {stats.totalUsage && (
                        <Typography variant="caption" color="text.secondary">
                          {stats.totalUsage.memoryPercent.toFixed(1)}% used
                        </Typography>
                      )}
                    </Box>
                    <Box sx={{ color: '#ff9800' }}>
                      <MemoryIcon sx={{ fontSize: 40 }} />
                    </Box>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        )}

        <Grid container spacing={3}>
          {nodes.map((node) => (
            <Grid item xs={12} md={6} key={node.name}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', mb: 2 }}>
                    <Box sx={{ flex: 1 }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        {getStatusIcon(node.status)}
                        <Typography variant="h6" sx={{ fontWeight: 600 }}>
                          {node.name}
                        </Typography>
                      </Box>
                      <Typography variant="caption" color="text.secondary">
                        {node.info.osImage} • {node.info.kubeletVersion}
                      </Typography>
                      {node.provider && (
                        <Typography variant="caption" color="text.secondary" display="block">
                          {node.provider} {node.region && `• ${node.region}`} {node.zone && `• ${node.zone}`}
                        </Typography>
                      )}
                    </Box>
                    <Box sx={{ display: 'flex', gap: 0.5, flexDirection: 'column', alignItems: 'flex-end' }}>
                      <Chip
                        label={node.status}
                        size="small"
                        color={getStatusColor(node.status)}
                      />
                      <Chip
                        label={isNodeSchedulable(node) ? 'Schedulable' : 'Unschedulable'}
                        size="small"
                        color={isNodeSchedulable(node) ? 'success' : 'warning'}
                      />
                    </Box>
                  </Box>

                  <Grid container spacing={2} sx={{ mb: 2 }}>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">
                        CPU
                      </Typography>
                      <Typography variant="body2">
                        {node.allocatable.cpu} cores
                      </Typography>
                      {node.usage && (
                        <Typography variant="caption" color="text.secondary">
                          {node.usage.cpuPercent.toFixed(1)}% used
                        </Typography>
                      )}
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">
                        Memory
                      </Typography>
                      <Typography variant="body2">
                        {node.allocatable.memory}
                      </Typography>
                      {node.usage && (
                        <Typography variant="caption" color="text.secondary">
                          {node.usage.memoryPercent.toFixed(1)}% used
                        </Typography>
                      )}
                    </Grid>
                    <Grid item xs={6}>
                      <Typography variant="caption" color="text.secondary">
                        Pods
                      </Typography>
                      <Typography variant="body2">
                        {node.pods} / {node.allocatable.pods}
                      </Typography>
                    </Grid>
                    {node.allocatable['nvidia.com/gpu'] && (
                      <Grid item xs={6}>
                        <Typography variant="caption" color="text.secondary">
                          GPUs
                        </Typography>
                        <Typography variant="body2">
                          {node.allocatable['nvidia.com/gpu']}
                        </Typography>
                      </Grid>
                    )}
                  </Grid>

                  <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 2 }}>
                    <Button
                      size="small"
                      startIcon={<LabelIcon />}
                      onClick={() => {
                        setSelectedNode(node);
                        setLabelDialogOpen(true);
                      }}
                    >
                      Labels ({Object.keys(node.labels).length})
                    </Button>
                    <Button
                      size="small"
                      startIcon={<TaintIcon />}
                      onClick={() => {
                        setSelectedNode(node);
                        setTaintDialogOpen(true);
                      }}
                    >
                      Taints ({node.taints.length})
                    </Button>
                  </Box>

                  <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                    {isNodeSchedulable(node) ? (
                      <Button
                        size="small"
                        variant="outlined"
                        color="warning"
                        startIcon={<BlockIcon />}
                        onClick={() => handleCordon(node.name)}
                      >
                        Cordon
                      </Button>
                    ) : (
                      <Button
                        size="small"
                        variant="outlined"
                        color="success"
                        onClick={() => handleUncordon(node.name)}
                      >
                        Uncordon
                      </Button>
                    )}
                    <Button
                      size="small"
                      variant="outlined"
                      color="error"
                      onClick={() => handleDrain(node.name)}
                    >
                      Drain
                    </Button>
                  </Box>
                </CardContent>
              </Card>
            </Grid>
          ))}
        </Grid>

        {/* Label Dialog */}
        <Dialog open={labelDialogOpen} onClose={() => setLabelDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>
            Node Labels - {selectedNode?.name}
          </DialogTitle>
          <DialogContent>
            <Box sx={{ mb: 3 }}>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>
                Current Labels
              </Typography>
              <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                {selectedNode && Object.entries(selectedNode.labels).map(([key, value]) => (
                  <Chip
                    key={key}
                    label={`${key}=${value}`}
                    size="small"
                    onDelete={() => handleRemoveLabel(selectedNode.name, key)}
                  />
                ))}
              </Box>
            </Box>

            <Typography variant="subtitle2" sx={{ mb: 2 }}>
              Add New Label
            </Typography>
            <TextField
              fullWidth
              label="Key"
              value={labelKey}
              onChange={(e) => setLabelKey(e.target.value)}
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="Value"
              value={labelValue}
              onChange={(e) => setLabelValue(e.target.value)}
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setLabelDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleAddLabel} variant="contained" disabled={!labelKey || !labelValue}>
              Add Label
            </Button>
          </DialogActions>
        </Dialog>

        {/* Taint Dialog */}
        <Dialog open={taintDialogOpen} onClose={() => setTaintDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>
            Node Taints - {selectedNode?.name}
          </DialogTitle>
          <DialogContent>
            <Box sx={{ mb: 3 }}>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>
                Current Taints
              </Typography>
              <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                {selectedNode?.taints.map((taint) => (
                  <Chip
                    key={taint.key}
                    label={`${taint.key}=${taint.value}:${taint.effect}`}
                    size="small"
                    onDelete={() => handleRemoveTaint(selectedNode.name, taint.key)}
                  />
                ))}
              </Box>
            </Box>

            <Typography variant="subtitle2" sx={{ mb: 2 }}>
              Add New Taint
            </Typography>
            <TextField
              fullWidth
              label="Key"
              value={taintKey}
              onChange={(e) => setTaintKey(e.target.value)}
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="Value"
              value={taintValue}
              onChange={(e) => setTaintValue(e.target.value)}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth>
              <InputLabel>Effect</InputLabel>
              <Select
                value={taintEffect}
                label="Effect"
                onChange={(e) => setTaintEffect(e.target.value)}
              >
                <MenuItem value="NoSchedule">NoSchedule</MenuItem>
                <MenuItem value="PreferNoSchedule">PreferNoSchedule</MenuItem>
                <MenuItem value="NoExecute">NoExecute</MenuItem>
              </Select>
            </FormControl>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setTaintDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleAddTaint} variant="contained" disabled={!taintKey || !taintValue}>
              Add Taint
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
    </WebSocketErrorBoundary>
  );
}

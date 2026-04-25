import { useState } from 'react';
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
  Tabs,
  Tab,
} from '@mui/material';
import {
  Add as AddIcon,
  Refresh as RefreshIcon,
  Delete as DeleteIcon,
  Check as CheckIcon,
  CheckCircle as ResolveIcon,
  Edit as EditIcon,
  Search as SearchIcon,
  Warning as WarningIcon,
  Error as ErrorIcon,
  Info as InfoIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';

interface AlertData {
  id: string;
  name: string;
  description?: string;
  severity: string;
  condition: string;
  threshold: number;
  status: string;
  triggeredAt?: string;
}

/**
 * Monitoring - Alert management and monitoring dashboard
 *
 * Administrative interface for managing monitoring alerts and viewing
 * system health metrics.
 *
 * Features:
 * - Active alerts dashboard
 * - Alert rule configuration (name, condition, threshold, severity)
 * - Alert history with filtering
 * - Acknowledge and resolve alerts
 * - Integration settings (webhooks, notifications)
 * - Severity-based color coding
 *
 * Alert Statuses:
 * - Triggered: Alert condition met, needs attention
 * - Acknowledged: Alert seen by administrator
 * - Resolved: Alert condition resolved
 *
 * Severity Levels:
 * - Critical: Immediate action required
 * - Warning: Issue needs attention
 * - Info: Informational alert
 *
 * @page
 * @route /admin/monitoring - Monitoring and alerts
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Monitoring dashboard with alert management
 */
export default function Monitoring() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState(0);
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedAlert, setSelectedAlert] = useState<AlertData | null>(null);

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    severity: 'warning',
    condition: '',
    threshold: 0,
  });

  // Fetch alerts
  const { data: alertsData, isLoading, refetch } = useQuery({
    queryKey: ['alerts', statusFilter],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (statusFilter !== 'all') {
        params.append('status', statusFilter);
      }

      const response = await fetch(`/api/v1/monitoring/alerts?${params}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch alerts');
      }

      const data = await response.json();
      return data.alerts || [];
    },
  });

  // Create alert mutation
  const createMutation = useMutation({
    mutationFn: async (data: typeof formData) => {
      const response = await fetch('/api/v1/monitoring/alerts', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to create alert');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
      setCreateDialogOpen(false);
      setFormData({
        name: '',
        description: '',
        severity: 'warning',
        condition: '',
        threshold: 0,
      });
      addNotification({
        message: 'Alert rule created successfully',
        severity: 'success',
        priority: 'high',
        title: 'Alert Created',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to create alert: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Creation Failed',
      });
    },
  });

  // Update alert mutation
  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: string; data: typeof formData }) => {
      const response = await fetch(`/api/v1/monitoring/alerts/${id}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to update alert');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
      setEditDialogOpen(false);
      setSelectedAlert(null);
      addNotification({
        message: 'Alert updated successfully',
        severity: 'success',
        priority: 'medium',
        title: 'Alert Updated',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to update alert: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Update Failed',
      });
    },
  });

  // Acknowledge alert mutation
  const acknowledgeMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/v1/monitoring/alerts/${id}/acknowledge`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to acknowledge alert');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
      addNotification({
        message: 'Alert acknowledged',
        severity: 'success',
        priority: 'medium',
        title: 'Alert Acknowledged',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to acknowledge alert: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Acknowledge Failed',
      });
    },
  });

  // Resolve alert mutation
  const resolveMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/v1/monitoring/alerts/${id}/resolve`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to resolve alert');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
      addNotification({
        message: 'Alert resolved',
        severity: 'success',
        priority: 'medium',
        title: 'Alert Resolved',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to resolve alert: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Resolve Failed',
      });
    },
  });

  // Delete alert mutation
  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/v1/monitoring/alerts/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to delete alert');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['alerts'] });
      setDeleteConfirmOpen(false);
      setSelectedAlert(null);
      addNotification({
        message: 'Alert deleted successfully',
        severity: 'success',
        priority: 'medium',
        title: 'Alert Deleted',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to delete alert: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Delete Failed',
      });
    },
  });

  const handleCreateAlert = () => {
    createMutation.mutate(formData);
  };

  const handleUpdateAlert = () => {
    if (selectedAlert) {
      updateMutation.mutate({ id: selectedAlert.id, data: formData });
    }
  };

  const handleEditClick = (alert: AlertData) => {
    setSelectedAlert(alert);
    setFormData({
      name: alert.name,
      description: alert.description || '',
      severity: alert.severity,
      condition: alert.condition,
      threshold: alert.threshold,
    });
    setEditDialogOpen(true);
  };

  const handleDeleteClick = (alert: AlertData) => {
    setSelectedAlert(alert);
    setDeleteConfirmOpen(true);
  };

  const getSeverityColor = (severity: string) => {
    switch (severity.toLowerCase()) {
      case 'critical':
        return 'error';
      case 'warning':
        return 'warning';
      case 'info':
        return 'info';
      default:
        return 'default';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity.toLowerCase()) {
      case 'critical':
        return <ErrorIcon />;
      case 'warning':
        return <WarningIcon />;
      case 'info':
        return <InfoIcon />;
      default:
        return <InfoIcon />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case 'triggered':
        return 'error';
      case 'acknowledged':
        return 'warning';
      case 'resolved':
        return 'success';
      default:
        return 'default';
    }
  };

  const filteredAlerts = (alertsData || []).filter((alert: AlertData) => {
    return (
      alert.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      alert.description?.toLowerCase().includes(searchQuery.toLowerCase())
    );
  });

  const activeAlerts = filteredAlerts.filter((a: AlertData) => a.status === 'triggered');
  const acknowledgedAlerts = filteredAlerts.filter((a: AlertData) => a.status === 'acknowledged');
  const resolvedAlerts = filteredAlerts.filter((a: AlertData) => a.status === 'resolved');

  if (isLoading) {
    return (
      <AdminPortalLayout title="Monitoring">
        <Container maxWidth="xl">
          <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
            <CircularProgress />
          </Box>
        </Container>
      </AdminPortalLayout>
    );
  }

  return (
    <AdminPortalLayout title="Monitoring & Alerts">
      <Container maxWidth="xl">
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              Monitoring & Alerts
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage alert rules and monitor system health
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={() => refetch()}
            >
              Refresh
            </Button>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => setCreateDialogOpen(true)}
            >
              Create Alert
            </Button>
          </Box>
        </Box>

        {/* Alert Summary Cards */}
        <Grid container spacing={2} sx={{ mb: 3 }}>
          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <Box>
                    <Typography variant="h3" color="error.main">
                      {activeAlerts.length}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Active Alerts
                    </Typography>
                  </Box>
                  <ErrorIcon sx={{ fontSize: 48, color: 'error.main', opacity: 0.3 }} />
                </Box>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <Box>
                    <Typography variant="h3" color="warning.main">
                      {acknowledgedAlerts.length}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Acknowledged
                    </Typography>
                  </Box>
                  <WarningIcon sx={{ fontSize: 48, color: 'warning.main', opacity: 0.3 }} />
                </Box>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <Box>
                    <Typography variant="h3" color="success.main">
                      {resolvedAlerts.length}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Resolved
                    </Typography>
                  </Box>
                  <CheckIcon sx={{ fontSize: 48, color: 'success.main', opacity: 0.3 }} />
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Filters */}
        <Card sx={{ mb: 3 }}>
          <CardContent>
            <Grid container spacing={2} alignItems="center">
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  placeholder="Search alerts..."
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
              <Grid item xs={12} md={3}>
                <FormControl fullWidth>
                  <InputLabel>Status</InputLabel>
                  <Select
                    value={statusFilter}
                    label="Status"
                    onChange={(e) => setStatusFilter(e.target.value)}
                  >
                    <MenuItem value="all">All</MenuItem>
                    <MenuItem value="triggered">Triggered</MenuItem>
                    <MenuItem value="acknowledged">Acknowledged</MenuItem>
                    <MenuItem value="resolved">Resolved</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} md={3}>
                <Typography variant="body2" color="text.secondary">
                  Showing {filteredAlerts.length} alerts
                </Typography>
              </Grid>
            </Grid>
          </CardContent>
        </Card>

        {/* Tabs */}
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
          <Tabs value={activeTab} onChange={(_, newValue) => setActiveTab(newValue)}>
            <Tab label={`Active (${activeAlerts.length})`} />
            <Tab label={`Acknowledged (${acknowledgedAlerts.length})`} />
            <Tab label={`Resolved (${resolvedAlerts.length})`} />
            <Tab label="All Alerts" />
          </Tabs>
        </Box>

        {/* Alerts Table */}
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Alert</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Condition</TableCell>
                <TableCell>Threshold</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Triggered</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(activeTab === 0 ? activeAlerts :
                activeTab === 1 ? acknowledgedAlerts :
                activeTab === 2 ? resolvedAlerts :
                filteredAlerts).length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No alerts found
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                (activeTab === 0 ? activeAlerts :
                  activeTab === 1 ? acknowledgedAlerts :
                  activeTab === 2 ? resolvedAlerts :
                  filteredAlerts).map((alert: AlertData) => (
                  <TableRow key={alert.id}>
                    <TableCell>
                      <Typography variant="body2" fontWeight="medium">
                        {alert.name}
                      </Typography>
                      {alert.description && (
                        <Typography variant="caption" color="text.secondary" display="block">
                          {alert.description}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip
                        icon={getSeverityIcon(alert.severity)}
                        label={alert.severity}
                        color={getSeverityColor(alert.severity)}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                        {alert.condition}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {alert.threshold}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={alert.status}
                        color={getStatusColor(alert.status)}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      {alert.triggeredAt && (
                        <Typography variant="body2">
                          {new Date(alert.triggeredAt).toLocaleString()}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5}>
                        {alert.status === 'triggered' && (
                          <Tooltip title="Acknowledge">
                            <IconButton
                              size="small"
                              onClick={() => acknowledgeMutation.mutate(alert.id)}
                              disabled={acknowledgeMutation.isPending}
                            >
                              <CheckIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        )}
                        {(alert.status === 'triggered' || alert.status === 'acknowledged') && (
                          <Tooltip title="Resolve">
                            <IconButton
                              size="small"
                              color="success"
                              onClick={() => resolveMutation.mutate(alert.id)}
                              disabled={resolveMutation.isPending}
                            >
                              <ResolveIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        )}
                        <Tooltip title="Edit">
                          <IconButton
                            size="small"
                            onClick={() => handleEditClick(alert)}
                          >
                            <EditIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Delete">
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => handleDeleteClick(alert)}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </Stack>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        {/* Create Alert Dialog */}
        <Dialog
          open={createDialogOpen}
          onClose={() => setCreateDialogOpen(false)}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>Create Alert Rule</DialogTitle>
          <DialogContent>
            <TextField
              fullWidth
              label="Name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              sx={{ mt: 2, mb: 2 }}
              required
            />
            <TextField
              fullWidth
              label="Description"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              multiline
              rows={2}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth sx={{ mb: 2 }}>
              <InputLabel>Severity</InputLabel>
              <Select
                value={formData.severity}
                label="Severity"
                onChange={(e) => setFormData({ ...formData, severity: e.target.value })}
              >
                <MenuItem value="critical">Critical</MenuItem>
                <MenuItem value="warning">Warning</MenuItem>
                <MenuItem value="info">Info</MenuItem>
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Condition"
              value={formData.condition}
              onChange={(e) => setFormData({ ...formData, condition: e.target.value })}
              sx={{ mb: 2 }}
              placeholder="cpu_usage > threshold"
              helperText="Condition expression to evaluate"
            />
            <TextField
              fullWidth
              label="Threshold"
              type="number"
              value={formData.threshold}
              onChange={(e) => setFormData({ ...formData, threshold: parseFloat(e.target.value) })}
              helperText="Numeric threshold value"
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleCreateAlert}
              variant="contained"
              disabled={!formData.name || !formData.condition || createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Edit Alert Dialog */}
        <Dialog
          open={editDialogOpen}
          onClose={() => {
            setEditDialogOpen(false);
            setSelectedAlert(null);
          }}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>Edit Alert Rule</DialogTitle>
          <DialogContent>
            <TextField
              fullWidth
              label="Name"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              sx={{ mt: 2, mb: 2 }}
              required
            />
            <TextField
              fullWidth
              label="Description"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              multiline
              rows={2}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth sx={{ mb: 2 }}>
              <InputLabel>Severity</InputLabel>
              <Select
                value={formData.severity}
                label="Severity"
                onChange={(e) => setFormData({ ...formData, severity: e.target.value })}
              >
                <MenuItem value="critical">Critical</MenuItem>
                <MenuItem value="warning">Warning</MenuItem>
                <MenuItem value="info">Info</MenuItem>
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Condition"
              value={formData.condition}
              onChange={(e) => setFormData({ ...formData, condition: e.target.value })}
              sx={{ mb: 2 }}
              placeholder="cpu_usage > threshold"
            />
            <TextField
              fullWidth
              label="Threshold"
              type="number"
              value={formData.threshold}
              onChange={(e) => setFormData({ ...formData, threshold: parseFloat(e.target.value) })}
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => {
              setEditDialogOpen(false);
              setSelectedAlert(null);
            }}>
              Cancel
            </Button>
            <Button
              onClick={handleUpdateAlert}
              variant="contained"
              disabled={!formData.name || !formData.condition || updateMutation.isPending}
            >
              {updateMutation.isPending ? 'Updating...' : 'Update'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog
          open={deleteConfirmOpen}
          onClose={() => {
            setDeleteConfirmOpen(false);
            setSelectedAlert(null);
          }}
          maxWidth="xs"
        >
          <DialogTitle>Delete Alert?</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to delete this alert rule? This action cannot be undone.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => {
              setDeleteConfirmOpen(false);
              setSelectedAlert(null);
            }}>
              Cancel
            </Button>
            <Button
              onClick={() => selectedAlert && deleteMutation.mutate(selectedAlert.id)}
              color="error"
              variant="contained"
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogActions>
        </Dialog>
      </Container>
    </AdminPortalLayout>
  );
}

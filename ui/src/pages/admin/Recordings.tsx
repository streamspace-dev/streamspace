import { useState } from 'react';
import {
  Box,
  Card,
  CardContent,
  Typography,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  IconButton,
  Chip,
  TextField,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Tab,
  Tabs,
  Grid,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Tooltip,
  Alert,
  LinearProgress,
} from '@mui/material';
import {
  Download as DownloadIcon,
  Delete as DeleteIcon,
  Close as CloseIcon,
  Add as AddIcon,
  Edit as EditIcon,
  VideoLibrary as VideoIcon,
  Policy as PolicyIcon,
  History as HistoryIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';

interface Recording {
  id: number;
  session_id: string;
  recording_type: string;
  storage_path: string;
  file_size_bytes: number;
  file_size_mb: number;
  duration_seconds: number;
  duration_formatted: string;
  started_at: string;
  ended_at: string;
  status: string;
  error_message?: string;
  created_by?: string;
  session_name?: string;
  user_name?: string;
  created_at: string;
  updated_at: string;
}

interface RecordingPolicy {
  id: number;
  name: string;
  description?: string;
  auto_record: boolean;
  recording_format: string;
  retention_days: number;
  apply_to_users: string[] | null;
  apply_to_teams: string[] | null;
  apply_to_templates: string[] | null;
  require_reason: boolean;
  allow_user_playback: boolean;
  allow_user_download: boolean;
  require_approval: boolean;
  notify_on_recording: boolean;
  metadata: Record<string, unknown> | null;
  enabled: boolean;
  priority: number;
  created_at: string;
  updated_at: string;
}

interface AccessLogEntry {
  id: number;
  recording_id: number;
  user_id?: string;
  user_name?: string;
  action: string;
  accessed_at: string;
  ip_address?: string;
  user_agent?: string;
}

function Recordings() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  // State
  const [activeTab, setActiveTab] = useState(0);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_selectedRecording, setSelectedRecording] = useState<Recording | null>(null);
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [_playerOpen, setPlayerOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [recordingToDelete, setRecordingToDelete] = useState<number | null>(null);
  const [accessLogDialogOpen, setAccessLogDialogOpen] = useState(false);
  const [selectedRecordingForLog, setSelectedRecordingForLog] = useState<number | null>(null);
  const [policyDialogOpen, setPolicyDialogOpen] = useState(false);
  const [editingPolicy, setEditingPolicy] = useState<RecordingPolicy | null>(null);

  // Filters
  const [statusFilter, setStatusFilter] = useState('');
  const [searchQuery, setSearchQuery] = useState('');

  // Policy form
  const [policyForm, setPolicyForm] = useState({
    name: '',
    description: '',
    auto_record: false,
    recording_format: 'webm',
    retention_days: 30,
    require_reason: false,
    allow_user_playback: true,
    allow_user_download: true,
    require_approval: false,
    notify_on_recording: true,
    enabled: true,
    priority: 0,
  });

  // Fetch recordings
  const { data: recordingsData, isLoading: loadingRecordings } = useQuery({
    queryKey: ['admin-recordings', statusFilter, searchQuery],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (statusFilter) params.append('status', statusFilter);
      if (searchQuery) params.append('search', searchQuery);

      const response = await fetch(`/api/v1/admin/recordings?${params}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch recordings');
      }

      return response.json();
    },
  });

  // Fetch policies
  const { data: policiesData, isLoading: loadingPolicies } = useQuery({
    queryKey: ['recording-policies'],
    queryFn: async () => {
      const response = await fetch('/api/v1/admin/recording-policies', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch policies');
      }

      return response.json();
    },
  });

  // Fetch access log
  const { data: accessLogData, isLoading: loadingAccessLog } = useQuery({
    queryKey: ['recording-access-log', selectedRecordingForLog],
    queryFn: async () => {
      if (!selectedRecordingForLog) return null;

      const response = await fetch(`/api/v1/admin/recordings/${selectedRecordingForLog}/access-log`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch access log');
      }

      return response.json();
    },
    enabled: !!selectedRecordingForLog,
  });

  // Delete recording mutation
  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const response = await fetch(`/api/v1/admin/recordings/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to delete recording');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-recordings'] });
      setDeleteDialogOpen(false);
      setRecordingToDelete(null);
      addNotification({
        message: 'Recording deleted successfully',
        severity: 'success',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to delete recording: ${error.message}`,
        severity: 'error',
      });
    },
  });

  // Create/Update policy mutation
  const savePolicyMutation = useMutation({
    mutationFn: async (data: typeof policyForm) => {
      const url = editingPolicy
        ? `/api/v1/admin/recording-policies/${editingPolicy.id}`
        : '/api/v1/admin/recording-policies';

      const response = await fetch(url, {
        method: editingPolicy ? 'PUT' : 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to save policy');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['recording-policies'] });
      setPolicyDialogOpen(false);
      setEditingPolicy(null);
      resetPolicyForm();
      addNotification({
        message: `Policy ${editingPolicy ? 'updated' : 'created'} successfully`,
        severity: 'success',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to save policy: ${error.message}`,
        severity: 'error',
      });
    },
  });

  // Delete policy mutation
  const deletePolicyMutation = useMutation({
    mutationFn: async (id: number) => {
      const response = await fetch(`/api/v1/admin/recording-policies/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to delete policy');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['recording-policies'] });
      addNotification({
        message: 'Policy deleted successfully',
        severity: 'success',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to delete policy: ${error.message}`,
        severity: 'error',
      });
    },
  });

  const recordings = recordingsData?.recordings || [];
  const policies = policiesData?.policies || [];
  const accessLog = accessLogData?.access_log || [];

  const resetPolicyForm = () => {
    setPolicyForm({
      name: '',
      description: '',
      auto_record: false,
      recording_format: 'webm',
      retention_days: 30,
      require_reason: false,
      allow_user_playback: true,
      allow_user_download: true,
      require_approval: false,
      notify_on_recording: true,
      enabled: true,
      priority: 0,
    });
  };

  const handleDownload = (recording: Recording) => {
    window.open(`/api/v1/admin/recordings/${recording.id}/download`, '_blank');
  };

  const handleDelete = (id: number) => {
    setRecordingToDelete(id);
    setDeleteDialogOpen(true);
  };

  const confirmDelete = () => {
    if (recordingToDelete) {
      deleteMutation.mutate(recordingToDelete);
    }
  };

  const handleViewAccessLog = (id: number) => {
    setSelectedRecordingForLog(id);
    setAccessLogDialogOpen(true);
  };

  const handleCreatePolicy = () => {
    setEditingPolicy(null);
    resetPolicyForm();
    setPolicyDialogOpen(true);
  };

  const handleEditPolicy = (policy: RecordingPolicy) => {
    setEditingPolicy(policy);
    setPolicyForm({
      name: policy.name,
      description: policy.description || '',
      auto_record: policy.auto_record,
      recording_format: policy.recording_format,
      retention_days: policy.retention_days,
      require_reason: policy.require_reason,
      allow_user_playback: policy.allow_user_playback,
      allow_user_download: policy.allow_user_download,
      require_approval: policy.require_approval,
      notify_on_recording: policy.notify_on_recording,
      enabled: policy.enabled,
      priority: policy.priority,
    });
    setPolicyDialogOpen(true);
  };

  const handleDeletePolicy = (id: number) => {
    if (window.confirm('Are you sure you want to delete this policy?')) {
      deletePolicyMutation.mutate(id);
    }
  };

  const handleSavePolicy = () => {
    savePolicyMutation.mutate(policyForm);
  };

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'completed':
        return 'success';
      case 'recording':
        return 'primary';
      case 'failed':
        return 'error';
      case 'processing':
        return 'warning';
      default:
        return 'default';
    }
  };

  return (
    <AdminPortalLayout>
      <Box sx={{ p: 3 }}>
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography variant="h4" sx={{ fontWeight: 600 }}>
            Session Recordings
          </Typography>
        </Box>

        {/* Tabs */}
        <Tabs value={activeTab} onChange={(_, v) => setActiveTab(v)} sx={{ mb: 3 }}>
          <Tab icon={<VideoIcon />} label="Recordings" iconPosition="start" />
          <Tab icon={<PolicyIcon />} label="Policies" iconPosition="start" />
        </Tabs>

        {/* Recordings Tab */}
        {activeTab === 0 && (
          <>
            {/* Filters */}
            <Card sx={{ mb: 3 }}>
              <CardContent>
                <Grid container spacing={2}>
                  <Grid item xs={12} md={4}>
                    <TextField
                      fullWidth
                      size="small"
                      label="Search"
                      placeholder="Search by session or user..."
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                    />
                  </Grid>
                  <Grid item xs={12} md={4}>
                    <FormControl fullWidth size="small">
                      <InputLabel>Status</InputLabel>
                      <Select
                        value={statusFilter}
                        label="Status"
                        onChange={(e) => setStatusFilter(e.target.value)}
                      >
                        <MenuItem value="">All</MenuItem>
                        <MenuItem value="recording">Recording</MenuItem>
                        <MenuItem value="completed">Completed</MenuItem>
                        <MenuItem value="processing">Processing</MenuItem>
                        <MenuItem value="failed">Failed</MenuItem>
                      </Select>
                    </FormControl>
                  </Grid>
                </Grid>
              </CardContent>
            </Card>

            {/* Recordings Table */}
            <Card>
              <CardContent>
                {loadingRecordings ? (
                  <LinearProgress />
                ) : recordings.length === 0 ? (
                  <Alert severity="info">No recordings found</Alert>
                ) : (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell>Session</TableCell>
                          <TableCell>User</TableCell>
                          <TableCell>Duration</TableCell>
                          <TableCell>Size</TableCell>
                          <TableCell>Status</TableCell>
                          <TableCell>Started</TableCell>
                          <TableCell>Actions</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {recordings.map((recording: Recording) => (
                          <TableRow key={recording.id} hover>
                            <TableCell>
                              <Typography variant="body2" fontWeight="medium">
                                {recording.session_name || recording.session_id}
                              </Typography>
                              <Typography variant="caption" color="text.secondary">
                                {recording.recording_type}
                              </Typography>
                            </TableCell>
                            <TableCell>{recording.user_name || recording.created_by || 'N/A'}</TableCell>
                            <TableCell>{recording.duration_formatted || '0s'}</TableCell>
                            <TableCell>{recording.file_size_mb?.toFixed(2) || '0.00'} MB</TableCell>
                            <TableCell>
                              <Chip
                                label={recording.status}
                                size="small"
                                color={getStatusColor(recording.status)}
                              />
                            </TableCell>
                            <TableCell>
                              {recording.started_at ? new Date(recording.started_at).toLocaleString() : 'N/A'}
                            </TableCell>
                            <TableCell>
                              <Box sx={{ display: 'flex', gap: 1 }}>
                                {recording.status === 'completed' && (
                                  <Tooltip title="Download">
                                    <IconButton
                                      size="small"
                                      color="primary"
                                      onClick={() => handleDownload(recording)}
                                    >
                                      <DownloadIcon fontSize="small" />
                                    </IconButton>
                                  </Tooltip>
                                )}
                                <Tooltip title="View Access Log">
                                  <IconButton
                                    size="small"
                                    color="info"
                                    onClick={() => handleViewAccessLog(recording.id)}
                                  >
                                    <HistoryIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                                <Tooltip title="Delete">
                                  <IconButton
                                    size="small"
                                    color="error"
                                    onClick={() => handleDelete(recording.id)}
                                  >
                                    <DeleteIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                              </Box>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}
              </CardContent>
            </Card>
          </>
        )}

        {/* Policies Tab */}
        {activeTab === 1 && (
          <>
            <Box sx={{ mb: 2, display: 'flex', justifyContent: 'flex-end' }}>
              <Button
                variant="contained"
                startIcon={<AddIcon />}
                onClick={handleCreatePolicy}
              >
                Create Policy
              </Button>
            </Box>

            <Card>
              <CardContent>
                {loadingPolicies ? (
                  <LinearProgress />
                ) : policies.length === 0 ? (
                  <Alert severity="info">No recording policies configured</Alert>
                ) : (
                  <TableContainer>
                    <Table>
                      <TableHead>
                        <TableRow>
                          <TableCell>Name</TableCell>
                          <TableCell>Auto Record</TableCell>
                          <TableCell>Format</TableCell>
                          <TableCell>Retention</TableCell>
                          <TableCell>Priority</TableCell>
                          <TableCell>Enabled</TableCell>
                          <TableCell>Actions</TableCell>
                        </TableRow>
                      </TableHead>
                      <TableBody>
                        {policies.map((policy: RecordingPolicy) => (
                          <TableRow key={policy.id} hover>
                            <TableCell>
                              <Typography variant="body2" fontWeight="medium">
                                {policy.name}
                              </Typography>
                              {policy.description && (
                                <Typography variant="caption" color="text.secondary">
                                  {policy.description}
                                </Typography>
                              )}
                            </TableCell>
                            <TableCell>
                              <Chip
                                label={policy.auto_record ? 'Yes' : 'No'}
                                size="small"
                                color={policy.auto_record ? 'success' : 'default'}
                              />
                            </TableCell>
                            <TableCell>{policy.recording_format.toUpperCase()}</TableCell>
                            <TableCell>{policy.retention_days} days</TableCell>
                            <TableCell>{policy.priority}</TableCell>
                            <TableCell>
                              <Chip
                                label={policy.enabled ? 'Enabled' : 'Disabled'}
                                size="small"
                                color={policy.enabled ? 'success' : 'default'}
                              />
                            </TableCell>
                            <TableCell>
                              <Box sx={{ display: 'flex', gap: 1 }}>
                                <Tooltip title="Edit">
                                  <IconButton
                                    size="small"
                                    color="primary"
                                    onClick={() => handleEditPolicy(policy)}
                                  >
                                    <EditIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                                <Tooltip title="Delete">
                                  <IconButton
                                    size="small"
                                    color="error"
                                    onClick={() => handleDeletePolicy(policy.id)}
                                  >
                                    <DeleteIcon fontSize="small" />
                                  </IconButton>
                                </Tooltip>
                              </Box>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </TableContainer>
                )}
              </CardContent>
            </Card>
          </>
        )}

        {/* Delete Confirmation Dialog */}
        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Confirm Delete</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to delete this recording? This action cannot be undone and the video file will be permanently deleted.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
            <Button
              onClick={confirmDelete}
              color="error"
              variant="contained"
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Access Log Dialog */}
        <Dialog
          open={accessLogDialogOpen}
          onClose={() => setAccessLogDialogOpen(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <Typography variant="h6">Recording Access Log</Typography>
              <IconButton onClick={() => setAccessLogDialogOpen(false)} size="small">
                <CloseIcon />
              </IconButton>
            </Box>
          </DialogTitle>
          <DialogContent dividers>
            {loadingAccessLog ? (
              <LinearProgress />
            ) : accessLog.length === 0 ? (
              <Alert severity="info">No access log entries found</Alert>
            ) : (
              <TableContainer>
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>User</TableCell>
                      <TableCell>Action</TableCell>
                      <TableCell>Timestamp</TableCell>
                      <TableCell>IP Address</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {accessLog.map((log: AccessLogEntry) => (
                      <TableRow key={log.id}>
                        <TableCell>{log.user_name || log.user_id || 'Anonymous'}</TableCell>
                        <TableCell>
                          <Chip label={log.action} size="small" />
                        </TableCell>
                        <TableCell>{new Date(log.accessed_at).toLocaleString()}</TableCell>
                        <TableCell>{log.ip_address || 'N/A'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </DialogContent>
        </Dialog>

        {/* Policy Dialog */}
        <Dialog
          open={policyDialogOpen}
          onClose={() => setPolicyDialogOpen(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>
            {editingPolicy ? 'Edit Recording Policy' : 'Create Recording Policy'}
          </DialogTitle>
          <DialogContent dividers>
            <Grid container spacing={2} sx={{ mt: 1 }}>
              <Grid item xs={12}>
                <TextField
                  fullWidth
                  label="Policy Name"
                  value={policyForm.name}
                  onChange={(e) => setPolicyForm({ ...policyForm, name: e.target.value })}
                  required
                />
              </Grid>
              <Grid item xs={12}>
                <TextField
                  fullWidth
                  label="Description"
                  value={policyForm.description}
                  onChange={(e) => setPolicyForm({ ...policyForm, description: e.target.value })}
                  multiline
                  rows={2}
                />
              </Grid>
              <Grid item xs={12} md={6}>
                <FormControl fullWidth>
                  <InputLabel>Recording Format</InputLabel>
                  <Select
                    value={policyForm.recording_format}
                    label="Recording Format"
                    onChange={(e) => setPolicyForm({ ...policyForm, recording_format: e.target.value })}
                  >
                    <MenuItem value="webm">WebM</MenuItem>
                    <MenuItem value="mp4">MP4</MenuItem>
                    <MenuItem value="mkv">MKV</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  type="number"
                  label="Retention Days"
                  value={policyForm.retention_days}
                  onChange={(e) => setPolicyForm({ ...policyForm, retention_days: parseInt(e.target.value) })}
                />
              </Grid>
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  type="number"
                  label="Priority"
                  value={policyForm.priority}
                  onChange={(e) => setPolicyForm({ ...policyForm, priority: parseInt(e.target.value) })}
                  helperText="Higher priority policies are evaluated first"
                />
              </Grid>
              <Grid item xs={12}>
                <FormControl component="fieldset">
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.auto_record}
                        onChange={(e) => setPolicyForm({ ...policyForm, auto_record: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Auto-record sessions</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.allow_user_playback}
                        onChange={(e) => setPolicyForm({ ...policyForm, allow_user_playback: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Allow user playback</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.allow_user_download}
                        onChange={(e) => setPolicyForm({ ...policyForm, allow_user_download: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Allow user download</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.require_approval}
                        onChange={(e) => setPolicyForm({ ...policyForm, require_approval: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Require approval to access</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.notify_on_recording}
                        onChange={(e) => setPolicyForm({ ...policyForm, notify_on_recording: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Notify user when recording</Typography>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center' }}>
                      <input
                        type="checkbox"
                        checked={policyForm.enabled}
                        onChange={(e) => setPolicyForm({ ...policyForm, enabled: e.target.checked })}
                        style={{ marginRight: 8 }}
                      />
                      <Typography>Policy enabled</Typography>
                    </Box>
                  </Box>
                </FormControl>
              </Grid>
            </Grid>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setPolicyDialogOpen(false)}>Cancel</Button>
            <Button
              onClick={handleSavePolicy}
              variant="contained"
              disabled={!policyForm.name || savePolicyMutation.isPending}
            >
              {savePolicyMutation.isPending ? 'Saving...' : editingPolicy ? 'Update' : 'Create'}
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </AdminPortalLayout>
  );
}

export default Recordings;

import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Button,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  LinearProgress,
  Chip,
  Tooltip,
  CircularProgress,
} from '@mui/material';
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import { api, UserQuota, SetQuotaRequest } from '../../lib/api';

export default function AdminQuotas() {
  const [quotas, setQuotas] = useState<UserQuota[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedQuota, setSelectedQuota] = useState<UserQuota | null>(null);

  // Form state
  const [username, setUsername] = useState('');
  const [maxSessions, setMaxSessions] = useState('10');
  const [maxCPU, setMaxCPU] = useState('4000m');
  const [maxMemory, setMaxMemory] = useState('8Gi');
  const [maxStorage, setMaxStorage] = useState('50Gi');

  useEffect(() => {
    loadQuotas();
  }, []);

  const loadQuotas = async () => {
    setLoading(true);
    setError('');

    try {
      const quotasData = await api.listUserQuotas();
      setQuotas(quotasData);
    } catch (err: any) {
      console.error('Failed to load quotas:', err);
      setError(err.response?.data?.message || 'Failed to load user quotas');
    } finally {
      setLoading(false);
    }
  };

  const handleOpenEdit = (quota?: UserQuota) => {
    if (quota) {
      setSelectedQuota(quota);
      setUsername(quota.username);
      setMaxSessions(quota.limits.maxSessions.toString());
      setMaxCPU(quota.limits.maxCPU);
      setMaxMemory(quota.limits.maxMemory);
      setMaxStorage(quota.limits.maxStorage);
    } else {
      setSelectedQuota(null);
      setUsername('');
      setMaxSessions('10');
      setMaxCPU('4000m');
      setMaxMemory('8Gi');
      setMaxStorage('50Gi');
    }
    setEditDialogOpen(true);
  };

  const handleSaveQuota = async () => {
    if (!username.trim()) {
      setError('Username is required');
      return;
    }

    try {
      const quotaData: SetQuotaRequest = {
        username: username.trim(),
        maxSessions: parseInt(maxSessions),
        maxCPU,
        maxMemory,
        maxStorage,
      };

      await api.setUserQuota(quotaData);
      setEditDialogOpen(false);
      loadQuotas();
    } catch (err: any) {
      console.error('Failed to save quota:', err);
      setError(err.response?.data?.message || 'Failed to save user quota');
    }
  };

  const handleDeleteQuota = async () => {
    if (!selectedQuota) return;

    try {
      await api.deleteUserQuota(selectedQuota.username);
      setDeleteDialogOpen(false);
      setSelectedQuota(null);
      loadQuotas();
    } catch (err: any) {
      console.error('Failed to delete quota:', err);
      setError(err.response?.data?.message || 'Failed to delete user quota');
    }
  };

  const calculatePercentage = (used: number, limit: number): number => {
    if (limit === 0) return 0;
    return (used / limit) * 100;
  };

  const parseResourceString = (resource: string): number => {
    // Parse resource strings like "2Gi", "4000m", etc.
    const match = resource.match(/^(\d+)([a-zA-Z]+)?$/);
    if (!match) return 0;

    const value = parseInt(match[1]);
    const unit = match[2] || '';

    if (unit === 'Gi') return value * 1024;
    if (unit === 'Mi') return value;
    if (unit === 'm') return value;

    return value;
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
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            User Quotas
          </Typography>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Button variant="outlined" startIcon={<RefreshIcon />} onClick={loadQuotas}>
              Refresh
            </Button>
            <Button variant="contained" startIcon={<AddIcon />} onClick={() => handleOpenEdit()}>
              Add Quota
            </Button>
          </Box>
        </Box>

        {error && (
          <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Username</TableCell>
                <TableCell>Sessions</TableCell>
                <TableCell>CPU</TableCell>
                <TableCell>Memory</TableCell>
                <TableCell>Storage</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {quotas.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <Typography variant="body2" color="text.secondary" sx={{ py: 3 }}>
                      No user quotas configured
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                quotas.map((quota) => {
                  const sessionPercent = calculatePercentage(quota.used.sessions, quota.limits.maxSessions);
                  const cpuPercent = calculatePercentage(
                    parseResourceString(quota.used.cpu),
                    parseResourceString(quota.limits.maxCPU)
                  );
                  const memoryPercent = calculatePercentage(
                    parseResourceString(quota.used.memory),
                    parseResourceString(quota.limits.maxMemory)
                  );
                  const storagePercent = calculatePercentage(
                    parseResourceString(quota.used.storage),
                    parseResourceString(quota.limits.maxStorage)
                  );

                  return (
                    <TableRow key={quota.username}>
                      <TableCell>
                        <Typography variant="body1" sx={{ fontWeight: 500 }}>
                          {quota.username}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5 }}>
                            <Typography variant="body2">
                              {quota.used.sessions} / {quota.limits.maxSessions}
                            </Typography>
                            {sessionPercent > 90 && (
                              <Tooltip title="Near quota limit">
                                <WarningIcon color="warning" fontSize="small" />
                              </Tooltip>
                            )}
                          </Box>
                          <LinearProgress
                            variant="determinate"
                            value={Math.min(sessionPercent, 100)}
                            sx={{
                              height: 6,
                              borderRadius: 3,
                              backgroundColor: 'rgba(63, 81, 181, 0.1)',
                              '& .MuiLinearProgress-bar': {
                                backgroundColor: sessionPercent > 90 ? '#f44336' : '#3f51b5',
                              },
                            }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" sx={{ mb: 0.5 }}>
                            {quota.used.cpu} / {quota.limits.maxCPU}
                          </Typography>
                          <LinearProgress
                            variant="determinate"
                            value={Math.min(cpuPercent, 100)}
                            sx={{
                              height: 6,
                              borderRadius: 3,
                              backgroundColor: 'rgba(245, 0, 87, 0.1)',
                              '& .MuiLinearProgress-bar': {
                                backgroundColor: cpuPercent > 90 ? '#f44336' : '#f50057',
                              },
                            }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" sx={{ mb: 0.5 }}>
                            {quota.used.memory} / {quota.limits.maxMemory}
                          </Typography>
                          <LinearProgress
                            variant="determinate"
                            value={Math.min(memoryPercent, 100)}
                            sx={{
                              height: 6,
                              borderRadius: 3,
                              backgroundColor: 'rgba(255, 152, 0, 0.1)',
                              '& .MuiLinearProgress-bar': {
                                backgroundColor: memoryPercent > 90 ? '#f44336' : '#ff9800',
                              },
                            }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell>
                        <Box>
                          <Typography variant="body2" sx={{ mb: 0.5 }}>
                            {quota.used.storage} / {quota.limits.maxStorage}
                          </Typography>
                          <LinearProgress
                            variant="determinate"
                            value={Math.min(storagePercent, 100)}
                            sx={{
                              height: 6,
                              borderRadius: 3,
                              backgroundColor: 'rgba(76, 175, 80, 0.1)',
                              '& .MuiLinearProgress-bar': {
                                backgroundColor: storagePercent > 90 ? '#f44336' : '#4caf50',
                              },
                            }}
                          />
                        </Box>
                      </TableCell>
                      <TableCell align="right">
                        <IconButton size="small" onClick={() => handleOpenEdit(quota)}>
                          <EditIcon />
                        </IconButton>
                        <IconButton
                          size="small"
                          color="error"
                          onClick={() => {
                            setSelectedQuota(quota);
                            setDeleteDialogOpen(true);
                          }}
                        >
                          <DeleteIcon />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>

        {/* Edit/Add Quota Dialog */}
        <Dialog open={editDialogOpen} onClose={() => setEditDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>{selectedQuota ? 'Edit User Quota' : 'Add User Quota'}</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 2 }}>
              <TextField
                fullWidth
                label="Username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={!!selectedQuota}
                helperText={selectedQuota ? 'Username cannot be changed' : 'Enter the username'}
              />
              <TextField
                fullWidth
                label="Max Sessions"
                type="number"
                value={maxSessions}
                onChange={(e) => setMaxSessions(e.target.value)}
                helperText="Maximum number of concurrent sessions"
              />
              <TextField
                fullWidth
                label="Max CPU"
                value={maxCPU}
                onChange={(e) => setMaxCPU(e.target.value)}
                helperText="e.g., 4000m (4 cores) or 8000m (8 cores)"
              />
              <TextField
                fullWidth
                label="Max Memory"
                value={maxMemory}
                onChange={(e) => setMaxMemory(e.target.value)}
                helperText="e.g., 8Gi or 16Gi"
              />
              <TextField
                fullWidth
                label="Max Storage"
                value={maxStorage}
                onChange={(e) => setMaxStorage(e.target.value)}
                helperText="e.g., 50Gi or 100Gi"
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setEditDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleSaveQuota} variant="contained">
              {selectedQuota ? 'Update' : 'Create'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Delete User Quota</DialogTitle>
          <DialogContent>
            Are you sure you want to delete the quota for user <strong>{selectedQuota?.username}</strong>? This action
            cannot be undone.
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
            <Button onClick={handleDeleteQuota} color="error" variant="contained">
              Delete
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

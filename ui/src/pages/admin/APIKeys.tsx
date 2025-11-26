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
} from '@mui/material';
import {
  Add as AddIcon,
  Refresh as RefreshIcon,
  Delete as DeleteIcon,
  Block as RevokeIcon,
  Search as SearchIcon,
  ContentCopy as CopyIcon,
  Visibility as VisibilityIcon,
  VisibilityOff as VisibilityOffIcon,
  Check as CheckIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';

/**
 * APIKeys - System-wide API key management for administrators
 *
 * Administrative interface for managing all API keys in the platform.
 * Provides visibility into all keys, creation, revocation, and deletion
 * capabilities.
 *
 * Features:
 * - List all API keys system-wide
 * - Filter by user, status, expiration
 * - Create API keys with scopes and rate limits
 * - Revoke/delete keys
 * - View usage statistics
 * - Copy key on creation (shown once)
 *
 * Security:
 * - Keys shown in full only once during creation
 * - Key prefix display (sk_xxxxx...)
 * - SHA-256 hashed storage
 * - Scope-based access control
 * - Rate limit configuration
 *
 * @page
 * @route /admin/api-keys - API key management
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} API key management interface
 */
export default function APIKeys() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [newKeyDialogOpen, setNewKeyDialogOpen] = useState(false);
  const [createdKey, setCreatedKey] = useState<string>('');
  const [showCreatedKey, setShowCreatedKey] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null);

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    scopes: [] as string[],
    rateLimit: 1000,
    expiresIn: '1y',
  });

  // Available scopes (can be fetched from API in real implementation)
  const availableScopes = [
    'sessions:read',
    'sessions:write',
    'sessions:delete',
    'templates:read',
    'templates:write',
    'users:read',
    'users:write',
    'admin:all',
  ];

  // Fetch API keys
  const { data: apiKeys, isLoading, refetch } = useQuery({
    queryKey: ['api-keys-admin'],
    queryFn: async () => {
      const response = await fetch('/api/v1/admin/apikeys', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch API keys');
      }

      const data = await response.json();
      return Array.isArray(data) ? data : [];
    },
  });

  // Create API key mutation
  const createMutation = useMutation({
    mutationFn: async (data: typeof formData) => {
      const response = await fetch('/api/v1/apikeys', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to create API key');
      }

      return response.json();
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ['api-keys-admin'] });
      setCreateDialogOpen(false);
      setCreatedKey(data.key || '');
      setNewKeyDialogOpen(true);
      setFormData({
        name: '',
        description: '',
        scopes: [],
        rateLimit: 1000,
        expiresIn: '1y',
      });
      addNotification({
        message: 'API key created successfully',
        severity: 'success',
        priority: 'high',
        title: 'API Key Created',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to create API key: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Creation Failed',
      });
    },
  });

  // Revoke API key mutation
  const revokeMutation = useMutation({
    mutationFn: async (id: number) => {
      const response = await fetch(`/api/v1/apikeys/${id}/revoke`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to revoke API key');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys-admin'] });
      addNotification({
        message: 'API key revoked successfully',
        severity: 'success',
        priority: 'medium',
        title: 'API Key Revoked',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to revoke API key: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Revoke Failed',
      });
    },
  });

  // Delete API key mutation
  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const response = await fetch(`/api/v1/apikeys/${id}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.error || 'Failed to delete API key');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['api-keys-admin'] });
      setDeleteConfirmOpen(false);
      setSelectedKeyId(null);
      addNotification({
        message: 'API key deleted successfully',
        severity: 'success',
        priority: 'medium',
        title: 'API Key Deleted',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to delete API key: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Delete Failed',
      });
    },
  });

  const handleCreateKey = () => {
    createMutation.mutate(formData);
  };

  const handleRevokeKey = (id: number) => {
    revokeMutation.mutate(id);
  };

  const handleDeleteKey = () => {
    if (selectedKeyId !== null) {
      deleteMutation.mutate(selectedKeyId);
    }
  };

  const handleCopyKey = () => {
    navigator.clipboard.writeText(createdKey);
    addNotification({
      message: 'API key copied to clipboard',
      severity: 'success',
      priority: 'low',
      title: 'Copied',
    });
  };

  const filteredKeys = (apiKeys || []).filter((key: any) => {
    const matchesSearch =
      key.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      key.userId?.toLowerCase().includes(searchQuery.toLowerCase()) ||
      key.keyPrefix?.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesStatus =
      statusFilter === 'all' ||
      (statusFilter === 'active' && key.isActive) ||
      (statusFilter === 'inactive' && !key.isActive) ||
      (statusFilter === 'expired' && key.expiresAt && new Date(key.expiresAt) < new Date());

    return matchesSearch && matchesStatus;
  });

  if (isLoading) {
    return (
      <AdminPortalLayout title="API Keys">
        <Container maxWidth="xl">
          <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
            <CircularProgress />
          </Box>
        </Container>
      </AdminPortalLayout>
    );
  }

  return (
    <AdminPortalLayout title="API Keys">
      <Container maxWidth="xl">
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              API Keys Management
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage API keys for programmatic access - {filteredKeys.length} total keys
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
              Create API Key
            </Button>
          </Box>
        </Box>

        {/* Filters */}
        <Card sx={{ mb: 3 }}>
          <CardContent>
            <Grid container spacing={2} alignItems="center">
              <Grid item xs={12} md={6}>
                <TextField
                  fullWidth
                  placeholder="Search by name, user, or key prefix..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  InputProps={{
                    startAdornment: (
                      <InputAdornment position="start">
                        <SearchIcon />
                      </InputAdornment>
                    ),
                  }}
                  inputProps={{
                    'aria-label': 'Search API keys',
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
                    <MenuItem value="active">Active</MenuItem>
                    <MenuItem value="inactive">Inactive</MenuItem>
                    <MenuItem value="expired">Expired</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} md={3}>
                <Typography variant="body2" color="text.secondary">
                  Showing {filteredKeys.length} of {apiKeys?.length || 0} keys
                </Typography>
              </Grid>
            </Grid>
          </CardContent>
        </Card>

        {/* API Keys Table */}
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Key Prefix</TableCell>
                <TableCell>User</TableCell>
                <TableCell>Scopes</TableCell>
                <TableCell>Rate Limit</TableCell>
                <TableCell>Usage</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Expires</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filteredKeys.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={9} align="center">
                    <Typography variant="body2" color="text.secondary">
                      No API keys found
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                filteredKeys.map((key: any) => (
                  <TableRow key={key.id}>
                    <TableCell>
                      <Typography variant="body2" fontWeight="medium">
                        {key.name}
                      </Typography>
                      {key.description && (
                        <Typography variant="caption" color="text.secondary" display="block">
                          {key.description}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                        {key.keyPrefix}...
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{key.userId}</Typography>
                    </TableCell>
                    <TableCell>
                      <Stack direction="row" spacing={0.5} flexWrap="wrap">
                        {(key.scopes || []).slice(0, 2).map((scope: string) => (
                          <Chip key={scope} label={scope} size="small" />
                        ))}
                        {(key.scopes || []).length > 2 && (
                          <Chip label={`+${key.scopes.length - 2}`} size="small" />
                        )}
                      </Stack>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">{key.rateLimit}/hr</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2">
                        {key.useCount} calls
                      </Typography>
                      {key.lastUsedAt && (
                        <Typography variant="caption" color="text.secondary" display="block">
                          Last: {new Date(key.lastUsedAt).toLocaleDateString()}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={key.isActive ? 'Active' : 'Inactive'}
                        color={key.isActive ? 'success' : 'default'}
                        size="small"
                      />
                    </TableCell>
                    <TableCell>
                      {key.expiresAt ? (
                        <>
                          <Typography variant="body2">
                            {new Date(key.expiresAt).toLocaleDateString()}
                          </Typography>
                          {new Date(key.expiresAt) < new Date() && (
                            <Chip label="Expired" color="error" size="small" />
                          )}
                        </>
                      ) : (
                        <Typography variant="body2" color="text.secondary">
                          Never
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Box sx={{ display: 'flex', gap: 0.5 }}>
                        {key.isActive && (
                          <Tooltip title="Revoke">
                            <IconButton
                              size="small"
                              onClick={() => handleRevokeKey(key.id)}
                              disabled={revokeMutation.isPending}
                            >
                              <RevokeIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        )}
                        <Tooltip title="Delete">
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => {
                              setSelectedKeyId(key.id);
                              setDeleteConfirmOpen(true);
                            }}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>

        {/* Create API Key Dialog */}
        <Dialog
          open={createDialogOpen}
          onClose={() => setCreateDialogOpen(false)}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>Create API Key</DialogTitle>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" paragraph>
              Create a new API key for programmatic access. The key will be shown only once.
            </Typography>
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
              <InputLabel>Scopes</InputLabel>
              <Select
                multiple
                value={formData.scopes}
                label="Scopes"
                onChange={(e) => setFormData({ ...formData, scopes: e.target.value as string[] })}
                renderValue={(selected) => (
                  <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                    {selected.map((value) => (
                      <Chip key={value} label={value} size="small" />
                    ))}
                  </Box>
                )}
              >
                {availableScopes.map((scope) => (
                  <MenuItem key={scope} value={scope}>
                    {scope}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            <TextField
              fullWidth
              label="Rate Limit (requests/hour)"
              type="number"
              value={formData.rateLimit}
              onChange={(e) => setFormData({ ...formData, rateLimit: parseInt(e.target.value) })}
              sx={{ mb: 2 }}
            />
            <FormControl fullWidth>
              <InputLabel>Expires In</InputLabel>
              <Select
                value={formData.expiresIn}
                label="Expires In"
                onChange={(e) => setFormData({ ...formData, expiresIn: e.target.value })}
              >
                <MenuItem value="30d">30 days</MenuItem>
                <MenuItem value="90d">90 days</MenuItem>
                <MenuItem value="1y">1 year</MenuItem>
                <MenuItem value="never">Never</MenuItem>
              </Select>
            </FormControl>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateDialogOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleCreateKey}
              variant="contained"
              disabled={!formData.name || createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* New Key Dialog (shows key once) */}
        <Dialog
          open={newKeyDialogOpen}
          onClose={() => {
            setNewKeyDialogOpen(false);
            setCreatedKey('');
            setShowCreatedKey(false);
          }}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>API Key Created</DialogTitle>
          <DialogContent>
            <Alert severity="warning" sx={{ mb: 2 }}>
              This is the only time you will see this key. Copy it now and store it securely.
            </Alert>
            <TextField
              fullWidth
              label="API Key"
              value={createdKey}
              type={showCreatedKey ? 'text' : 'password'}
              InputProps={{
                readOnly: true,
                sx: { fontFamily: 'monospace' },
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton onClick={() => setShowCreatedKey(!showCreatedKey)} edge="end">
                      {showCreatedKey ? <VisibilityOffIcon /> : <VisibilityIcon />}
                    </IconButton>
                    <IconButton onClick={handleCopyKey} edge="end">
                      <CopyIcon />
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />
          </DialogContent>
          <DialogActions>
            <Button
              onClick={() => {
                setNewKeyDialogOpen(false);
                setCreatedKey('');
                setShowCreatedKey(false);
              }}
              variant="contained"
              startIcon={<CheckIcon />}
            >
              I've Saved It
            </Button>
          </DialogActions>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog
          open={deleteConfirmOpen}
          onClose={() => {
            setDeleteConfirmOpen(false);
            setSelectedKeyId(null);
          }}
          maxWidth="xs"
        >
          <DialogTitle>Delete API Key?</DialogTitle>
          <DialogContent>
            <Typography>
              This action cannot be undone. Any applications using this key will lose access immediately.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => {
              setDeleteConfirmOpen(false);
              setSelectedKeyId(null);
            }}>
              Cancel
            </Button>
            <Button
              onClick={handleDeleteKey}
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

import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Button,
  Grid,
  CircularProgress,
  Alert,
  ToggleButtonGroup,
  ToggleButton,
  TextField,
  InputAdornment,
  Snackbar,
  Paper,
  Tabs,
  Tab,
} from '@mui/material';
import {
  Add as AddIcon,
  Sync as SyncIcon,
  Refresh as RefreshIcon,
  Search as SearchIcon,
  ViewList as ViewListIcon,
  ViewModule as ViewModuleIcon,
  CheckCircle as CheckCircleIcon,
  Error as ErrorIcon,
} from '@mui/icons-material';
import AdminPortalLayout from '../components/AdminPortalLayout';
import RepositoryCard from '../components/RepositoryCard';
import RepositoryDialog from '../components/RepositoryDialog';
import {
  useRepositories,
  useAddRepository,
  useUpdateRepository,
  useSyncRepository,
  useDeleteRepository,
  useSyncAllRepositories,
} from '../hooks/useApi';
import { Repository } from '../lib/api';
import { useRepositoryEvents } from '../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

/**
 * EnhancedRepositories - Advanced template repository management with real-time updates
 *
 * Enhanced version of repository management with real-time WebSocket updates, advanced
 * filtering, multiple view modes, and comprehensive statistics. Provides a rich user
 * experience for managing template repositories with live sync status monitoring and
 * automatic refresh capabilities.
 *
 * Features:
 * - Real-time repository sync status via WebSocket
 * - Statistics dashboard (total, synced, syncing, failed)
 * - Grid and list view modes
 * - Advanced filtering by status
 * - Search across repositories
 * - Live notification system for sync events
 * - Auto-refresh every 10 seconds
 * - WebSocket connection status indicator
 *
 * Real-time features:
 * - Live sync progress updates
 * - Automatic UI refresh on repository changes
 * - Connection status monitoring
 * - Event-driven notifications
 *
 * User workflows:
 * - Monitor repository sync status in real-time
 * - Quick access to repository statistics
 * - Filter and search for specific repositories
 * - Switch between grid and list views
 * - Receive instant notifications for sync events
 *
 * @page
 * @route /repositories - Enhanced template repository management
 * @access user - Available to all authenticated users
 *
 * @component
 *
 * @returns {JSX.Element} Enhanced repository management interface with real-time updates
 *
 * @example
 * // Route configuration:
 * <Route path="/repositories" element={<EnhancedRepositories />} />
 *
 * @see Repositories for basic repository management
 * @see RepositoryCard for individual repository display
 * @see RepositoryDialog for add/edit repository dialog
 */
function EnhancedRepositoriesContent() {
  const { data: repositories = [], isLoading, refetch } = useRepositories();
  const addRepository = useAddRepository();
  const updateRepository = useUpdateRepository();
  const syncRepository = useSyncRepository();
  const deleteRepository = useDeleteRepository();
  const syncAll = useSyncAllRepositories();

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingRepository, setEditingRepository] = useState<Repository | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({
    open: false,
    message: '',
    severity: 'success',
  });

  // WebSocket connection state
  const [wsConnected, setWsConnected] = useState(false);
  const [wsReconnectAttempts, setWsReconnectAttempts] = useState(0);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time repository events via WebSocket
  useRepositoryEvents((data: any) => {
    setWsConnected(true);
    setWsReconnectAttempts(0);

    // Show notifications for repository events
    if (data.event_type === 'repository.sync.started') {
      addNotification({
        message: `Repository sync started: ${data.repository_name || 'Unknown'}`,
        severity: 'info',
        priority: 'medium',
        title: 'Sync Started',
      });
      // Refresh repository list
      refetch();
    } else if (data.event_type === 'repository.sync.completed') {
      addNotification({
        message: `Repository synced successfully: ${data.repository_name || 'Unknown'}`,
        severity: 'success',
        priority: 'medium',
        title: 'Sync Complete',
      });
      // Refresh repository list
      refetch();
    } else if (data.event_type === 'repository.sync.failed') {
      addNotification({
        message: `Repository sync failed: ${data.repository_name || 'Unknown'} - ${data.error || 'Unknown error'}`,
        severity: 'error',
        priority: 'high',
        title: 'Sync Failed',
        autoDismiss: false,
      });
      // Refresh repository list
      refetch();
    } else if (data.event_type === 'repository.added') {
      addNotification({
        message: `New repository added: ${data.repository_name || 'Unknown'}`,
        severity: 'success',
        priority: 'medium',
        title: 'Repository Added',
      });
      // Refresh repository list
      refetch();
    } else if (data.event_type === 'repository.deleted') {
      addNotification({
        message: `Repository removed: ${data.repository_name || 'Unknown'}`,
        severity: 'warning',
        priority: 'medium',
        title: 'Repository Removed',
      });
      // Refresh repository list
      refetch();
    }
  });

  // Auto-refresh every 10 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      refetch();
    }, 10000);

    return () => clearInterval(interval);
  }, [refetch]);

  const handleAddClick = () => {
    setEditingRepository(null);
    setDialogOpen(true);
  };

  const handleEditClick = (repository: Repository) => {
    setEditingRepository(repository);
    setDialogOpen(true);
  };

  const handleSave = (data: any) => {
    if (editingRepository) {
      updateRepository.mutate(
        { id: editingRepository.id, data },
        {
          onSuccess: () => {
            setDialogOpen(false);
            setSnackbar({ open: true, message: 'Repository updated successfully', severity: 'success' });
          },
          onError: (error: any) => {
            setSnackbar({ open: true, message: error.message || 'Failed to update repository', severity: 'error' });
          },
        }
      );
    } else {
      addRepository.mutate(data, {
        onSuccess: () => {
          setDialogOpen(false);
          setSnackbar({ open: true, message: 'Repository added successfully', severity: 'success' });
        },
        onError: (error: any) => {
          setSnackbar({ open: true, message: error.message || 'Failed to add repository', severity: 'error' });
        },
      });
    }
  };

  const handleSync = (id: number) => {
    syncRepository.mutate(id, {
      onSuccess: () => {
        setSnackbar({ open: true, message: 'Repository sync started', severity: 'success' });
        // Refresh after a short delay to show the syncing status
        setTimeout(() => refetch(), 1000);
      },
      onError: (error: any) => {
        setSnackbar({ open: true, message: error.message || 'Failed to sync repository', severity: 'error' });
      },
    });
  };

  const handleSyncAll = () => {
    syncAll.mutate(undefined, {
      onSuccess: () => {
        setSnackbar({ open: true, message: 'Syncing all repositories', severity: 'success' });
        setTimeout(() => refetch(), 1000);
      },
      onError: (error: any) => {
        setSnackbar({ open: true, message: error.message || 'Failed to sync repositories', severity: 'error' });
      },
    });
  };

  const handleDelete = (id: number) => {
    if (!confirm('Are you sure you want to delete this repository? All associated templates will be removed from the catalog.')) {
      return;
    }

    deleteRepository.mutate(id, {
      onSuccess: () => {
        setSnackbar({ open: true, message: 'Repository deleted successfully', severity: 'success' });
      },
      onError: (error: any) => {
        setSnackbar({ open: true, message: error.message || 'Failed to delete repository', severity: 'error' });
      },
    });
  };

  const filteredRepositories = repositories.filter((repo) => {
    const matchesSearch =
      repo.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      repo.url.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesStatus = statusFilter === 'all' || repo.status === statusFilter;

    return matchesSearch && matchesStatus;
  });

  const syncingCount = repositories.filter((r) => r.status === 'syncing').length;
  const syncedCount = repositories.filter((r) => r.status === 'synced').length;
  const failedCount = repositories.filter((r) => r.status === 'failed').length;
  const totalTemplates = repositories.reduce((sum, r) => sum + r.templateCount, 0);

  if (isLoading) {
    return (
      <AdminPortalLayout>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
          <CircularProgress />
        </Box>
      </AdminPortalLayout>
    );
  }

  return (
    <AdminPortalLayout>
      <Box>
        {/* Header */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700, mb: 0.5 }}>
              Template Repositories
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage external template repositories and sync status
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
            <EnhancedWebSocketStatus
              isConnected={wsConnected}
              reconnectAttempts={wsReconnectAttempts}
              size="small"
            />
            <Button
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={() => refetch()}
            >
              Refresh
            </Button>
            <Button
              variant="outlined"
              startIcon={<SyncIcon />}
              onClick={handleSyncAll}
              disabled={syncAll.isPending || syncingCount > 0}
            >
              Sync All
            </Button>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={handleAddClick}
            >
              Add Repository
            </Button>
          </Box>
        </Box>

        {/* Statistics Cards */}
        <Grid container spacing={2} sx={{ mb: 3 }}>
          <Grid item xs={12} sm={6} md={3}>
            <Paper sx={{ p: 2, textAlign: 'center' }}>
              <Typography variant="h4" color="primary" sx={{ fontWeight: 700 }}>
                {repositories.length}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Total Repositories
              </Typography>
            </Paper>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Paper sx={{ p: 2, textAlign: 'center' }}>
              <Box display="flex" alignItems="center" justifyContent="center" gap={0.5} mb={0.5}>
                <CheckCircleIcon color="success" />
                <Typography variant="h4" color="success.main" sx={{ fontWeight: 700 }}>
                  {syncedCount}
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary">
                Synced
              </Typography>
            </Paper>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Paper sx={{ p: 2, textAlign: 'center' }}>
              <Box display="flex" alignItems="center" justifyContent="center" gap={0.5} mb={0.5}>
                {syncingCount > 0 && <SyncIcon color="info" className="spin" />}
                <Typography variant="h4" color="info.main" sx={{ fontWeight: 700 }}>
                  {syncingCount}
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary">
                Syncing
              </Typography>
            </Paper>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Paper sx={{ p: 2, textAlign: 'center' }}>
              <Box display="flex" alignItems="center" justifyContent="center" gap={0.5} mb={0.5}>
                {failedCount > 0 && <ErrorIcon color="error" />}
                <Typography variant="h4" color={failedCount > 0 ? 'error.main' : 'text.primary'} sx={{ fontWeight: 700 }}>
                  {totalTemplates}
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary">
                Total Templates
              </Typography>
            </Paper>
          </Grid>
        </Grid>

        {/* Filters and View Controls */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3, flexWrap: 'wrap', gap: 2 }}>
          <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', flex: 1 }}>
            <TextField
              placeholder="Search repositories..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              size="small"
              sx={{ minWidth: 300 }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon />
                  </InputAdornment>
                ),
              }}
            />

            <Tabs value={statusFilter} onChange={(_, v) => setStatusFilter(v)}>
              <Tab label="All" value="all" />
              <Tab label="Synced" value="synced" />
              <Tab label="Syncing" value="syncing" />
              <Tab label="Failed" value="failed" />
              <Tab label="Pending" value="pending" />
            </Tabs>
          </Box>

          <ToggleButtonGroup
            value={viewMode}
            exclusive
            onChange={(_, value) => value && setViewMode(value)}
            size="small"
          >
            <ToggleButton value="grid">
              <ViewModuleIcon />
            </ToggleButton>
            <ToggleButton value="list">
              <ViewListIcon />
            </ToggleButton>
          </ToggleButtonGroup>
        </Box>

        {/* Repository List */}
        {filteredRepositories.length === 0 ? (
          <Alert severity={repositories.length === 0 ? 'info' : 'warning'}>
            {repositories.length === 0
              ? 'No repositories configured. Add your first repository to populate the template catalog!'
              : 'No repositories match your search criteria.'}
          </Alert>
        ) : (
          <Grid container spacing={3}>
            {filteredRepositories.map((repo) => (
              <Grid item xs={12} sm={viewMode === 'grid' ? 6 : 12} md={viewMode === 'grid' ? 4 : 12} key={repo.id}>
                <RepositoryCard
                  repository={repo}
                  onSync={handleSync}
                  onEdit={handleEditClick}
                  onDelete={handleDelete}
                  isSyncing={syncRepository.isPending}
                />
              </Grid>
            ))}
          </Grid>
        )}

        {/* Repository Dialog */}
        <RepositoryDialog
          open={dialogOpen}
          onClose={() => setDialogOpen(false)}
          onSave={handleSave}
          repository={editingRepository}
          isSaving={addRepository.isPending || updateRepository.isPending}
        />

        {/* Snackbar for notifications */}
        <Snackbar
          open={snackbar.open}
          autoHideDuration={6000}
          onClose={() => setSnackbar({ ...snackbar, open: false })}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        >
          <Alert
            onClose={() => setSnackbar({ ...snackbar, open: false })}
            severity={snackbar.severity}
            sx={{ width: '100%' }}
          >
            {snackbar.message}
          </Alert>
        </Snackbar>

        <style>{`
          @keyframes spin {
            from { transform: rotate(0deg); }
            to { transform: rotate(360deg); }
          }
          .spin {
            animation: spin 1s linear infinite;
          }
        `}</style>
      </Box>
    </AdminPortalLayout>
  );
}

export default function EnhancedRepositories() {
  return (
    <WebSocketErrorBoundary>
      <EnhancedRepositoriesContent />
    </WebSocketErrorBoundary>
  );
}

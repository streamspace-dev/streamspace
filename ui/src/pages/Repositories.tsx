import { useState } from 'react';
import {
  Box,
  Typography,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Chip,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  CircularProgress,
  Alert,
} from '@mui/material';
import {
  Add as AddIcon,
  Sync as SyncIcon,
  Delete as DeleteIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import {
  useRepositories,
  useAddRepository,
  useSyncRepository,
  useDeleteRepository,
  useSyncAllRepositories,
} from '../hooks/useApi';

/**
 * Repositories - Template repository management page
 *
 * Provides interface for managing external Git repositories containing application templates.
 * Users can add, sync, and remove template repositories to populate the catalog with
 * available applications. Repositories are synchronized to pull the latest templates,
 * which become available in the template catalog for session creation.
 *
 * Features:
 * - Add new Git repositories with authentication options
 * - Manual and automatic repository synchronization
 * - View sync status and template counts
 * - Delete repositories and their templates
 * - Bulk sync all repositories
 * - Real-time sync status updates
 *
 * User workflows:
 * - Add custom template repositories (GitHub, GitLab, etc.)
 * - Sync repositories to update template catalog
 * - Monitor sync progress and status
 * - Remove outdated or unused repositories
 *
 * @page
 * @route /repositories - Template repository management
 * @access user - Available to all authenticated users
 *
 * @component
 *
 * @returns {JSX.Element} Repository management interface with sync controls
 *
 * @example
 * // Route configuration:
 * <Route path="/repositories" element={<Repositories />} />
 *
 * @see EnhancedRepositories for advanced repository management with WebSocket updates
 * @see TemplateCatalog for browsing templates from repositories
 */
export default function Repositories() {
  const { data: repositories = [], isLoading, refetch } = useRepositories();
  const addRepository = useAddRepository();
  const syncRepository = useSyncRepository();
  const deleteRepository = useDeleteRepository();
  const syncAll = useSyncAllRepositories();

  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    url: '',
    branch: 'main',
    authType: 'none',
  });

  const handleAdd = () => {
    addRepository.mutate(formData, {
      onSuccess: () => {
        setAddDialogOpen(false);
        setFormData({ name: '', url: '', branch: 'main', authType: 'none' });
      },
    });
  };

  const handleSync = (id: number) => {
    syncRepository.mutate(id);
  };

  const handleSyncAll = () => {
    syncAll.mutate();
  };

  const handleDelete = (id: number) => {
    if (confirm('Are you sure you want to delete this repository?')) {
      deleteRepository.mutate(id);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'synced':
        return 'success';
      case 'syncing':
        return 'info';
      case 'failed':
        return 'error';
      default:
        return 'default';
    }
  };

  if (isLoading) {
    return (
      <Layout>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
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
            Template Repositories
          </Typography>
          <Box sx={{ display: 'flex', gap: 1 }}>
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
              disabled={syncAll.isPending}
            >
              Sync All
            </Button>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => setAddDialogOpen(true)}
            >
              Add Repository
            </Button>
          </Box>
        </Box>

        {repositories.length === 0 ? (
          <Alert severity="info">
            No repositories configured. Add your first repository to populate the template catalog!
          </Alert>
        ) : (
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell>URL</TableCell>
                  <TableCell>Branch</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell align="right">Templates</TableCell>
                  <TableCell>Last Sync</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {repositories.map((repo) => (
                  <TableRow key={repo.id} hover>
                    <TableCell>
                      <Typography variant="body2" sx={{ fontWeight: 500 }}>
                        {repo.name}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" sx={{ maxWidth: 300 }} noWrap>
                        {repo.url}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip label={repo.branch} size="small" variant="outlined" />
                    </TableCell>
                    <TableCell>
                      <Chip label={repo.status} size="small" color={getStatusColor(repo.status)} />
                    </TableCell>
                    <TableCell align="right">{repo.templateCount}</TableCell>
                    <TableCell>
                      {repo.lastSync
                        ? new Date(repo.lastSync).toLocaleString()
                        : 'Never'}
                    </TableCell>
                    <TableCell align="right">
                      <IconButton
                        size="small"
                        color="primary"
                        onClick={() => handleSync(repo.id)}
                        disabled={syncRepository.isPending || repo.status === 'syncing'}
                      >
                        <SyncIcon />
                      </IconButton>
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => handleDelete(repo.id)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}

        <Dialog open={addDialogOpen} onClose={() => setAddDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Add Template Repository</DialogTitle>
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
              label="Git URL"
              value={formData.url}
              onChange={(e) => setFormData({ ...formData, url: e.target.value })}
              placeholder="https://github.com/username/repository"
              sx={{ mb: 2 }}
              required
            />
            <TextField
              fullWidth
              label="Branch"
              value={formData.branch}
              onChange={(e) => setFormData({ ...formData, branch: e.target.value })}
              sx={{ mb: 2 }}
            />
            <Alert severity="info">
              Repository will be synced automatically after adding. Make sure it contains valid Template YAML files.
            </Alert>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setAddDialogOpen(false)}>Cancel</Button>
            <Button
              onClick={handleAdd}
              variant="contained"
              disabled={!formData.name || !formData.url || addRepository.isPending}
            >
              {addRepository.isPending ? 'Adding...' : 'Add'}
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

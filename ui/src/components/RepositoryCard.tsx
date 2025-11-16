import { useState } from 'react';
import {
  Card,
  CardContent,
  CardActions,
  Typography,
  Box,
  Chip,
  IconButton,
  Button,
  LinearProgress,
  Tooltip,
  Menu,
  MenuItem,
  ListItemIcon,
  ListItemText,
  Collapse,
  Alert,
} from '@mui/material';
import {
  Sync as SyncIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  MoreVert as MoreIcon,
  Check as CheckIcon,
  Error as ErrorIcon,
  Schedule as ScheduleIcon,
  Storage as StorageIcon,
  Code as CodeIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
  GitHub as GitHubIcon,
} from '@mui/icons-material';
import { Repository } from '../lib/api';

interface RepositoryCardProps {
  repository: Repository;
  onSync: (id: number) => void;
  onEdit: (repository: Repository) => void;
  onDelete: (id: number) => void;
  isSyncing: boolean;
}

/**
 * RepositoryCard - Display card for a Git repository configuration
 *
 * Shows repository information including sync status, branch, authentication type,
 * and template count. Provides actions for syncing, editing, and deleting repositories.
 * Includes expandable details section and visual sync progress indicator.
 *
 * Features:
 * - Repository status badges (synced/syncing/failed/pending)
 * - Branch and authentication type indicators
 * - Template count display
 * - Last sync timestamp
 * - Sync progress indicator during sync
 * - Error message display
 * - Expandable details section
 * - Sync, edit, and delete actions
 * - Three-dot menu for additional actions
 * - Spinning sync icon animation
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {Repository} props.repository - Repository data to display
 * @param {Function} props.onSync - Callback to sync repository
 * @param {Function} props.onEdit - Callback to edit repository
 * @param {Function} props.onDelete - Callback to delete repository
 * @param {boolean} props.isSyncing - Whether repository is currently syncing
 *
 * @returns {JSX.Element} Rendered repository card
 *
 * @example
 * <RepositoryCard
 *   repository={repoData}
 *   onSync={handleSync}
 *   onEdit={handleEdit}
 *   onDelete={handleDelete}
 *   isSyncing={syncingRepoId === repoData.id}
 * />
 *
 * @see RepositoryDialog for repository edit/create dialog
 */
export default function RepositoryCard({
  repository,
  onSync,
  onEdit,
  onDelete,
  isSyncing,
}: RepositoryCardProps) {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [showDetails, setShowDetails] = useState(false);

  const getStatusIcon = () => {
    switch (repository.status) {
      case 'synced':
        return <CheckIcon fontSize="small" />;
      case 'syncing':
        return <SyncIcon fontSize="small" className="spin" />;
      case 'failed':
        return <ErrorIcon fontSize="small" />;
      default:
        return <ScheduleIcon fontSize="small" />;
    }
  };

  const getStatusColor = () => {
    switch (repository.status) {
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

  const formatDate = (date?: string) => {
    if (!date) return 'Never';
    return new Date(date).toLocaleString();
  };

  const getAuthLabel = () => {
    switch (repository.authType) {
      case 'none':
        return 'Public';
      case 'ssh':
        return 'SSH Key';
      case 'token':
        return 'Token';
      case 'basic':
        return 'Basic Auth';
      default:
        return repository.authType;
    }
  };

  return (
    <Card sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      {repository.status === 'syncing' && <LinearProgress />}

      <CardContent sx={{ flexGrow: 1 }}>
        <Box display="flex" justifyContent="space-between" alignItems="flex-start" mb={2}>
          <Box flex={1}>
            <Box display="flex" alignItems="center" gap={1} mb={1}>
              <GitHubIcon color="action" />
              <Typography variant="h6" component="div" sx={{ fontWeight: 600 }}>
                {repository.name}
              </Typography>
            </Box>

            <Typography
              variant="body2"
              color="text.secondary"
              sx={{
                mb: 1,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {repository.url}
            </Typography>
          </Box>

          <IconButton
            size="small"
            onClick={(e) => setAnchorEl(e.currentTarget)}
          >
            <MoreIcon />
          </IconButton>

          <Menu
            anchorEl={anchorEl}
            open={Boolean(anchorEl)}
            onClose={() => setAnchorEl(null)}
          >
            <MenuItem
              onClick={() => {
                setAnchorEl(null);
                onSync(repository.id);
              }}
              disabled={isSyncing || repository.status === 'syncing'}
            >
              <ListItemIcon>
                <SyncIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText>Sync Now</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                setAnchorEl(null);
                onEdit(repository);
              }}
            >
              <ListItemIcon>
                <EditIcon fontSize="small" />
              </ListItemIcon>
              <ListItemText>Edit</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                setAnchorEl(null);
                onDelete(repository.id);
              }}
              sx={{ color: 'error.main' }}
            >
              <ListItemIcon>
                <DeleteIcon fontSize="small" color="error" />
              </ListItemIcon>
              <ListItemText>Delete</ListItemText>
            </MenuItem>
          </Menu>
        </Box>

        <Box display="flex" gap={1} flexWrap="wrap" mb={2}>
          <Chip
            icon={getStatusIcon()}
            label={repository.status}
            size="small"
            color={getStatusColor()}
          />
          <Chip
            label={`Branch: ${repository.branch}`}
            size="small"
            variant="outlined"
          />
          <Chip
            label={getAuthLabel()}
            size="small"
            variant="outlined"
            color={repository.authType !== 'none' ? 'primary' : 'default'}
          />
        </Box>

        <Box display="flex" alignItems="center" gap={3} mb={1}>
          <Box display="flex" alignItems="center" gap={0.5}>
            <CodeIcon fontSize="small" color="action" />
            <Typography variant="body2" color="text.secondary">
              {repository.templateCount} templates
            </Typography>
          </Box>
          <Box display="flex" alignItems="center" gap={0.5}>
            <StorageIcon fontSize="small" color="action" />
            <Typography variant="body2" color="text.secondary">
              {formatDate(repository.lastSync)}
            </Typography>
          </Box>
        </Box>

        {repository.errorMessage && (
          <Alert severity="error" sx={{ mt: 2, py: 0.5 }}>
            <Typography variant="caption">{repository.errorMessage}</Typography>
          </Alert>
        )}

        <Button
          size="small"
          onClick={() => setShowDetails(!showDetails)}
          endIcon={showDetails ? <ExpandLessIcon /> : <ExpandMoreIcon />}
          sx={{ mt: 1 }}
        >
          {showDetails ? 'Hide' : 'Show'} Details
        </Button>

        <Collapse in={showDetails}>
          <Box sx={{ mt: 2, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
            <Typography variant="caption" color="text.secondary" display="block" mb={0.5}>
              <strong>Repository ID:</strong> {repository.id}
            </Typography>
            <Typography variant="caption" color="text.secondary" display="block" mb={0.5}>
              <strong>Created:</strong> {formatDate(repository.createdAt)}
            </Typography>
            <Typography variant="caption" color="text.secondary" display="block" mb={0.5}>
              <strong>Updated:</strong> {formatDate(repository.updatedAt)}
            </Typography>
            <Typography variant="caption" color="text.secondary" display="block">
              <strong>Full URL:</strong> {repository.url}
            </Typography>
          </Box>
        </Collapse>
      </CardContent>

      <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
        <Tooltip title="Sync repository">
          <span>
            <IconButton
              color="primary"
              onClick={() => onSync(repository.id)}
              disabled={isSyncing || repository.status === 'syncing'}
              size="small"
            >
              <SyncIcon />
            </IconButton>
          </span>
        </Tooltip>

        <Box>
          <Tooltip title="Edit repository">
            <IconButton
              size="small"
              onClick={() => onEdit(repository)}
            >
              <EditIcon />
            </IconButton>
          </Tooltip>

          <Tooltip title="Delete repository">
            <IconButton
              size="small"
              color="error"
              onClick={() => onDelete(repository.id)}
            >
              <DeleteIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </CardActions>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
        .spin {
          animation: spin 1s linear infinite;
        }
      `}</style>
    </Card>
  );
}

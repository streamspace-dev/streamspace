import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  IconButton,
  List,
  ListItem,
  ListItemAvatar,
  ListItemText,
  Avatar,
  Chip,
  Alert,
  CircularProgress,
  Tooltip,
  Divider,
} from '@mui/material';
import {
  Person,
  Delete,
  FiberManualRecord,
  AccessTime,
} from '@mui/icons-material';
import { api } from '../lib/api';
import { useUserStore } from '../store/userStore';

interface SessionCollaboratorsPanelProps {
  sessionId: string;
  isOwner?: boolean;
  onUpdate?: () => void;
}

interface Collaborator {
  id: string;
  sessionId: string;
  userId: string;
  permissionLevel: string;
  joinedAt: string;
  lastActivity: string;
  isActive: boolean;
  user: {
    username: string;
    fullName: string;
  };
}

/**
 * SessionCollaboratorsPanel - Display and manage active session collaborators
 *
 * Shows all users currently collaborating on a session with their activity status,
 * permission levels, and join times. Provides real-time updates of collaborator
 * activity and allows session owners to remove collaborators. Auto-refreshes
 * every 10 seconds to show current activity.
 *
 * Features:
 * - List of active collaborators with avatars
 * - Real-time activity indicators (active/idle)
 * - Permission level badges
 * - Relative timestamps (last activity, join time)
 * - Remove collaborator action (owner only)
 * - Auto-refresh every 10 seconds
 * - Current user highlighting
 * - Empty state message
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {string} props.sessionId - ID of session to show collaborators for
 * @param {boolean} [props.isOwner=false] - Whether current user is session owner
 * @param {Function} [props.onUpdate] - Callback when collaborators list changes
 *
 * @returns {JSX.Element} Rendered collaborators panel
 *
 * @example
 * <SessionCollaboratorsPanel
 *   sessionId="session-123"
 *   isOwner={true}
 *   onUpdate={handleUpdate}
 * />
 *
 * @see api.listCollaborators for loading collaborators
 * @see api.removeCollaborator for removing access
 */
export default function SessionCollaboratorsPanel({
  sessionId,
  isOwner = false,
  onUpdate,
}: SessionCollaboratorsPanelProps) {
  const currentUser = useUserStore((state) => state.user);
  const [collaborators, setCollaborators] = useState<Collaborator[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    loadCollaborators();

    // Refresh collaborators every 10 seconds
    const interval = setInterval(loadCollaborators, 10000);

    return () => clearInterval(interval);
  }, [sessionId]);

  const loadCollaborators = async () => {
    try {
      const collaboratorsData = await api.listCollaborators(sessionId);
      setCollaborators(collaboratorsData);
      setError('');
    } catch (err) {
      console.error('Failed to load collaborators:', err);
      setError('Failed to load collaborators');
    } finally {
      setLoading(false);
    }
  };

  const handleRemoveCollaborator = async (userId: string, username: string) => {
    if (!confirm(`Remove ${username} from this session?`)) return;

    try {
      await api.removeCollaborator(sessionId, userId);
      await loadCollaborators();
      if (onUpdate) onUpdate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove collaborator');
    }
  };

  const getPermissionColor = (level: string) => {
    switch (level) {
      case 'control':
        return 'error';
      case 'collaborate':
        return 'warning';
      case 'view':
        return 'info';
      default:
        return 'default';
    }
  };

  const formatRelativeTime = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const getInitials = (name: string) => {
    return name
      .split(' ')
      .map(n => n[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  if (loading) {
    return (
      <Box display="flex" justifyContent="center" alignItems="center" py={3}>
        <CircularProgress size={24} />
      </Box>
    );
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="h6" fontWeight={600}>
          Collaborators ({collaborators.length})
        </Typography>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      {collaborators.length === 0 ? (
        <Box
          sx={{
            textAlign: 'center',
            py: 4,
            px: 2,
            border: 1,
            borderColor: 'divider',
            borderRadius: 1,
            bgcolor: 'background.paper',
          }}
        >
          <Person sx={{ fontSize: 48, color: 'text.secondary', mb: 1 }} />
          <Typography variant="body2" color="text.secondary">
            No active collaborators
          </Typography>
          <Typography variant="caption" color="text.secondary">
            Share this session to collaborate with others
          </Typography>
        </Box>
      ) : (
        <List sx={{ bgcolor: 'background.paper', borderRadius: 1 }}>
          {collaborators.map((collaborator, index) => {
            const isCurrentUser = collaborator.userId === currentUser?.id;
            const canRemove = isOwner && !isCurrentUser;

            return (
              <Box key={collaborator.id}>
                {index > 0 && <Divider />}
                <ListItem
                  sx={{
                    py: 1.5,
                    px: 2,
                  }}
                >
                  <ListItemAvatar>
                    <Avatar
                      sx={{
                        bgcolor: collaborator.isActive ? 'success.main' : 'grey.400',
                      }}
                    >
                      {getInitials(collaborator.user.fullName)}
                    </Avatar>
                  </ListItemAvatar>

                  <ListItemText
                    primary={
                      <Box display="flex" alignItems="center" gap={1}>
                        <Typography variant="body1" fontWeight={500}>
                          {collaborator.user.fullName}
                        </Typography>
                        {isCurrentUser && (
                          <Chip label="You" size="small" color="primary" variant="outlined" />
                        )}
                        {collaborator.isActive && (
                          <Tooltip title="Currently active">
                            <FiberManualRecord
                              sx={{
                                fontSize: 12,
                                color: 'success.main',
                              }}
                            />
                          </Tooltip>
                        )}
                      </Box>
                    }
                    secondary={
                      <Box mt={0.5}>
                        <Box display="flex" gap={0.5} alignItems="center" flexWrap="wrap">
                          <Chip
                            label={collaborator.permissionLevel}
                            size="small"
                            color={getPermissionColor(collaborator.permissionLevel)}
                          />
                          <Typography variant="caption" color="text.secondary" display="flex" alignItems="center" gap={0.5}>
                            <AccessTime sx={{ fontSize: 14 }} />
                            {formatRelativeTime(collaborator.lastActivity)}
                          </Typography>
                        </Box>
                        <Typography variant="caption" color="text.secondary" display="block" mt={0.5}>
                          Joined {formatDate(collaborator.joinedAt)}
                        </Typography>
                      </Box>
                    }
                  />

                  {canRemove && (
                    <Tooltip title="Remove collaborator">
                      <IconButton
                        edge="end"
                        color="error"
                        onClick={() =>
                          handleRemoveCollaborator(collaborator.userId, collaborator.user.username)
                        }
                        size="small"
                      >
                        <Delete />
                      </IconButton>
                    </Tooltip>
                  )}
                </ListItem>
              </Box>
            );
          })}
        </List>
      )}

      <Box mt={2}>
        <Typography variant="caption" color="text.secondary" display="flex" alignItems="center" gap={0.5}>
          <FiberManualRecord sx={{ fontSize: 10, color: 'success.main' }} />
          Active in the last 5 minutes
        </Typography>
      </Box>
    </Box>
  );
}

import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  Box,
  Typography,
  IconButton,
  MenuItem,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Alert,
  Chip,
  Divider,
} from '@mui/material';
import {
  Close,
  PersonAdd,
  Delete,
  SwapHoriz,
} from '@mui/icons-material';
import { api } from '../lib/api';
import { useUserStore } from '../store/userStore';

interface SessionShareDialogProps {
  open: boolean;
  sessionId: string;
  sessionName: string;
  onClose: () => void;
}

interface Share {
  id: string;
  sessionId: string;
  ownerUserId: string;
  sharedWithUserId: string;
  permissionLevel: string;
  shareToken: string;
  createdAt: string;
  expiresAt?: string;
  acceptedAt?: string;
  user: {
    id: string;
    username: string;
    fullName: string;
    email: string;
  };
}

/**
 * SessionShareDialog - Dialog for sharing sessions with specific users
 *
 * Allows session owners to share sessions with other users by granting specific
 * permission levels (view, collaborate, control). Displays existing shares with
 * their status and permissions. Supports share expiration, revocation, and
 * ownership transfer.
 *
 * Features:
 * - Share with specific users from user list
 * - Permission levels: view, collaborate, control
 * - Optional expiration dates
 * - View and manage existing shares
 * - Share status indicators (pending/accepted/expired)
 * - Revoke share access
 * - Transfer session ownership (irreversible)
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {boolean} props.open - Whether the dialog is open
 * @param {string} props.sessionId - ID of session to share
 * @param {string} props.sessionName - Display name of session
 * @param {Function} props.onClose - Callback when dialog is closed
 *
 * @returns {JSX.Element} Rendered share dialog
 *
 * @example
 * <SessionShareDialog
 *   open={isOpen}
 *   sessionId="session-123"
 *   sessionName="My Firefox Session"
 *   onClose={() => setIsOpen(false)}
 * />
 *
 * @see api.createShare for sharing API
 * @see api.revokeShare for revoke API
 * @see api.transferOwnership for ownership transfer
 */
export default function SessionShareDialog({
  open,
  sessionId,
  sessionName,
  onClose,
}: SessionShareDialogProps) {
  const currentUser = useUserStore((state) => state.user);
  const [shares, setShares] = useState<Share[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Form state for creating new share
  const [selectedUserId, setSelectedUserId] = useState('');
  const [selectedUsername, setSelectedUsername] = useState('');
  const [permissionLevel, setPermissionLevel] = useState<'view' | 'collaborate' | 'control'>('view');
  const [expiresAt, setExpiresAt] = useState('');
  const [availableUsers, setAvailableUsers] = useState<Array<{ id: string; username: string; fullName: string }>>([]);

  // Transfer ownership state
  const [transferUserId, setTransferUserId] = useState('');
  const [showTransferConfirm, setShowTransferConfirm] = useState(false);

  useEffect(() => {
    if (open) {
      loadShares();
      loadUsers();
    }
  }, [open, sessionId]);

  const loadShares = async () => {
    try {
      const sharesData = await api.listShares(sessionId);
      setShares(sharesData);
    } catch (err) {
      console.error('Failed to load shares:', err);
    }
  };

  const loadUsers = async () => {
    try {
      const users = await api.listUsers();
      // Filter out current user
      const filteredUsers = users.filter(u => u.id !== currentUser?.id);
      setAvailableUsers(filteredUsers);
    } catch (err) {
      console.error('Failed to load users:', err);
    }
  };

  const handleCreateShare = async () => {
    if (!selectedUserId) {
      setError('Please select a user to share with');
      return;
    }

    setLoading(true);
    setError('');
    setSuccess('');

    try {
      const data: {
        sharedWithUserId: string;
        permissionLevel: 'view' | 'collaborate' | 'control';
        expiresAt?: string;
      } = {
        sharedWithUserId: selectedUserId,
        permissionLevel,
      };

      if (expiresAt) {
        data.expiresAt = new Date(expiresAt).toISOString();
      }

      await api.createShare(sessionId, data);
      setSuccess(`Session shared with ${selectedUsername}`);

      // Reset form
      setSelectedUserId('');
      setSelectedUsername('');
      setPermissionLevel('view');
      setExpiresAt('');

      // Reload shares
      await loadShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create share');
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeShare = async (shareId: string, username: string) => {
    if (!confirm(`Revoke access for ${username}?`)) return;

    setError('');
    setSuccess('');

    try {
      await api.revokeShare(sessionId, shareId);
      setSuccess(`Access revoked for ${username}`);
      await loadShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke share');
    }
  };

  const handleTransferOwnership = async () => {
    if (!transferUserId) {
      setError('Please select a user to transfer ownership to');
      return;
    }

    setLoading(true);
    setError('');
    setSuccess('');

    try {
      await api.transferOwnership(sessionId, transferUserId);
      setSuccess('Ownership transferred successfully');
      setShowTransferConfirm(false);
      setTransferUserId('');

      // Close dialog after transfer as current user is no longer owner
      setTimeout(() => {
        onClose();
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to transfer ownership');
    } finally {
      setLoading(false);
    }
  };

  const handleUserSelect = (event: any) => {
    const userId = event.target.value;
    setSelectedUserId(userId);
    const user = availableUsers.find(u => u.id === userId);
    setSelectedUsername(user?.username || '');
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

  const isExpired = (expiresAt?: string) => {
    if (!expiresAt) return false;
    return new Date(expiresAt) < new Date();
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

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Typography variant="h6">Share Session</Typography>
          <IconButton onClick={onClose} size="small">
            <Close />
          </IconButton>
        </Box>
      </DialogTitle>

      <DialogContent>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          Session: <strong>{sessionName}</strong>
        </Typography>

        {error && (
          <Alert severity="error" sx={{ mt: 2 }} onClose={() => setError('')}>
            {error}
          </Alert>
        )}

        {success && (
          <Alert severity="success" sx={{ mt: 2 }} onClose={() => setSuccess('')}>
            {success}
          </Alert>
        )}

        {/* Create New Share */}
        <Box mt={3}>
          <Typography variant="subtitle1" fontWeight={600} gutterBottom>
            Share with User
          </Typography>

          <Box display="flex" flexDirection="column" gap={2}>
            <TextField
              select
              label="Select User"
              value={selectedUserId}
              onChange={handleUserSelect}
              fullWidth
              size="small"
            >
              <MenuItem value="">
                <em>Select a user</em>
              </MenuItem>
              {availableUsers.map((user) => (
                <MenuItem key={user.id} value={user.id}>
                  {user.fullName} (@{user.username})
                </MenuItem>
              ))}
            </TextField>

            <TextField
              select
              label="Permission Level"
              value={permissionLevel}
              onChange={(e) => setPermissionLevel(e.target.value as 'view' | 'collaborate' | 'control')}
              fullWidth
              size="small"
              helperText={
                permissionLevel === 'view'
                  ? 'Can only view the session'
                  : permissionLevel === 'collaborate'
                  ? 'Can view and interact with the session'
                  : 'Can view, interact, and manage the session'
              }
            >
              <MenuItem value="view">View Only</MenuItem>
              <MenuItem value="collaborate">Collaborate</MenuItem>
              <MenuItem value="control">Full Control</MenuItem>
            </TextField>

            <TextField
              label="Expires At (Optional)"
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              fullWidth
              size="small"
              InputLabelProps={{ shrink: true }}
              helperText="Leave empty for no expiration"
            />

            <Button
              variant="contained"
              startIcon={<PersonAdd />}
              onClick={handleCreateShare}
              disabled={loading || !selectedUserId}
              fullWidth
            >
              Share Session
            </Button>
          </Box>
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* Existing Shares */}
        <Box>
          <Typography variant="subtitle1" fontWeight={600} gutterBottom>
            Current Shares ({shares.length})
          </Typography>

          {shares.length === 0 ? (
            <Typography variant="body2" color="text.secondary" fontStyle="italic">
              This session is not shared with anyone yet
            </Typography>
          ) : (
            <List>
              {shares.map((share) => (
                <ListItem
                  key={share.id}
                  sx={{
                    border: 1,
                    borderColor: 'divider',
                    borderRadius: 1,
                    mb: 1,
                  }}
                >
                  <ListItemText
                    primary={
                      <Box display="flex" alignItems="center" gap={1}>
                        <Typography variant="body1" fontWeight={500}>
                          {share.user.fullName}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          (@{share.user.username})
                        </Typography>
                      </Box>
                    }
                    secondary={
                      <Box mt={0.5}>
                        <Box display="flex" gap={1} alignItems="center" flexWrap="wrap">
                          <Chip
                            label={share.permissionLevel}
                            size="small"
                            color={getPermissionColor(share.permissionLevel)}
                          />
                          {share.expiresAt && (
                            <Chip
                              label={isExpired(share.expiresAt) ? 'Expired' : `Expires ${formatDate(share.expiresAt)}`}
                              size="small"
                              color={isExpired(share.expiresAt) ? 'error' : 'default'}
                            />
                          )}
                          {share.acceptedAt ? (
                            <Chip label="Accepted" size="small" color="success" />
                          ) : (
                            <Chip label="Pending" size="small" color="warning" />
                          )}
                        </Box>
                        <Typography variant="caption" color="text.secondary" display="block" mt={0.5}>
                          Shared on {formatDate(share.createdAt)}
                        </Typography>
                      </Box>
                    }
                  />
                  <ListItemSecondaryAction>
                    <IconButton
                      edge="end"
                      color="error"
                      onClick={() => handleRevokeShare(share.id, share.user.username)}
                      title="Revoke access"
                    >
                      <Delete />
                    </IconButton>
                  </ListItemSecondaryAction>
                </ListItem>
              ))}
            </List>
          )}
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* Transfer Ownership */}
        <Box>
          <Typography variant="subtitle1" fontWeight={600} gutterBottom color="error">
            Transfer Ownership
          </Typography>
          <Typography variant="body2" color="text.secondary" gutterBottom>
            Transfer complete ownership of this session to another user. You will lose all access unless they share it back with you.
          </Typography>

          {!showTransferConfirm ? (
            <Button
              variant="outlined"
              color="error"
              startIcon={<SwapHoriz />}
              onClick={() => setShowTransferConfirm(true)}
              sx={{ mt: 1 }}
            >
              Transfer Ownership
            </Button>
          ) : (
            <Box mt={2} p={2} bgcolor="error.50" borderRadius={1} border={1} borderColor="error.main">
              <Typography variant="body2" fontWeight={600} color="error" gutterBottom>
                ⚠️ Warning: This action cannot be undone!
              </Typography>
              <TextField
                select
                label="Transfer to User"
                value={transferUserId}
                onChange={(e) => setTransferUserId(e.target.value)}
                fullWidth
                size="small"
                sx={{ mt: 1, mb: 1 }}
              >
                <MenuItem value="">
                  <em>Select a user</em>
                </MenuItem>
                {availableUsers.map((user) => (
                  <MenuItem key={user.id} value={user.id}>
                    {user.fullName} (@{user.username})
                  </MenuItem>
                ))}
              </TextField>
              <Box display="flex" gap={1}>
                <Button
                  variant="contained"
                  color="error"
                  onClick={handleTransferOwnership}
                  disabled={loading || !transferUserId}
                  size="small"
                >
                  Confirm Transfer
                </Button>
                <Button
                  variant="outlined"
                  onClick={() => {
                    setShowTransferConfirm(false);
                    setTransferUserId('');
                  }}
                  size="small"
                >
                  Cancel
                </Button>
              </Box>
            </Box>
          )}
        </Box>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}

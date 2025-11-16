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
  InputAdornment,
} from '@mui/material';
import {
  Close,
  Add,
  Delete,
  ContentCopy,
  Link as LinkIcon,
} from '@mui/icons-material';
import { api } from '../lib/api';

interface SessionInvitationDialogProps {
  open: boolean;
  sessionId: string;
  sessionName: string;
  onClose: () => void;
}

interface Invitation {
  id: string;
  sessionId: string;
  createdBy: string;
  invitationToken: string;
  permissionLevel: string;
  maxUses: number;
  useCount: number;
  expiresAt?: string;
  createdAt: string;
  isExpired?: boolean;
  isExhausted?: boolean;
}

/**
 * SessionInvitationDialog - Dialog for creating shareable invitation links
 *
 * Allows session owners to create invitation links that can be shared with anyone
 * to grant access to a session. Supports configurable permission levels, usage
 * limits, and expiration dates. Displays existing invitations with their status
 * and provides easy copy-to-clipboard functionality.
 *
 * Features:
 * - Create shareable invitation links
 * - Permission levels: view, collaborate, control
 * - Configure max uses (or unlimited)
 * - Optional expiration dates
 * - Copy invitation URL to clipboard
 * - View invitation statistics (uses, status)
 * - Revoke invitation links
 * - Status indicators (active/expired/exhausted)
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {boolean} props.open - Whether the dialog is open
 * @param {string} props.sessionId - ID of session to create invitations for
 * @param {string} props.sessionName - Display name of session
 * @param {Function} props.onClose - Callback when dialog is closed
 *
 * @returns {JSX.Element} Rendered invitation dialog
 *
 * @example
 * <SessionInvitationDialog
 *   open={isOpen}
 *   sessionId="session-123"
 *   sessionName="My Firefox Session"
 *   onClose={() => setIsOpen(false)}
 * />
 *
 * @see api.createInvitation for creating invitations
 * @see api.revokeInvitation for revoking invitations
 */
export default function SessionInvitationDialog({
  open,
  sessionId,
  sessionName,
  onClose,
}: SessionInvitationDialogProps) {
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  // Form state for creating new invitation
  const [permissionLevel, setPermissionLevel] = useState<'view' | 'collaborate' | 'control'>('view');
  const [maxUses, setMaxUses] = useState<number>(10);
  const [expiresAt, setExpiresAt] = useState('');

  // Copy feedback
  const [copiedToken, setCopiedToken] = useState('');

  useEffect(() => {
    if (open) {
      loadInvitations();
    }
  }, [open, sessionId]);

  const loadInvitations = async () => {
    try {
      const invitationsData = await api.listInvitations(sessionId);
      setInvitations(invitationsData);
    } catch (err) {
      console.error('Failed to load invitations:', err);
    }
  };

  const handleCreateInvitation = async () => {
    setLoading(true);
    setError('');
    setSuccess('');

    try {
      const data: {
        permissionLevel: 'view' | 'collaborate' | 'control';
        maxUses?: number;
        expiresAt?: string;
      } = {
        permissionLevel,
      };

      if (maxUses && maxUses > 0) {
        data.maxUses = maxUses;
      }

      if (expiresAt) {
        data.expiresAt = new Date(expiresAt).toISOString();
      }

      const result = await api.createInvitation(sessionId, data);
      setSuccess(result.message || 'Invitation created successfully');

      // Reset form
      setPermissionLevel('view');
      setMaxUses(10);
      setExpiresAt('');

      // Reload invitations
      await loadInvitations();

      // Auto-copy the new token
      copyToClipboard(getInvitationUrl(result.invitationToken), result.invitationToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create invitation');
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeInvitation = async (token: string) => {
    if (!confirm('Revoke this invitation? All unused links will become invalid.')) return;

    setError('');
    setSuccess('');

    try {
      await api.revokeInvitation(token);
      setSuccess('Invitation revoked successfully');
      await loadInvitations();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke invitation');
    }
  };

  const getInvitationUrl = (token: string) => {
    const baseUrl = window.location.origin;
    return `${baseUrl}/invite/${token}`;
  };

  const copyToClipboard = async (url: string, token: string) => {
    try {
      await navigator.clipboard.writeText(url);
      setCopiedToken(token);
      setTimeout(() => setCopiedToken(''), 3000);
    } catch (err) {
      console.error('Failed to copy to clipboard:', err);
      setError('Failed to copy link to clipboard');
    }
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

  const getInvitationStatus = (invitation: Invitation) => {
    if (invitation.isExpired) {
      return { label: 'Expired', color: 'error' as const };
    }
    if (invitation.isExhausted) {
      return { label: 'Exhausted', color: 'default' as const };
    }
    const remaining = invitation.maxUses - invitation.useCount;
    return { label: `${remaining} uses left`, color: 'success' as const };
  };

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Box display="flex" alignItems="center" gap={1}>
            <LinkIcon />
            <Typography variant="h6">Invitation Links</Typography>
          </Box>
          <IconButton onClick={onClose} size="small">
            <Close />
          </IconButton>
        </Box>
      </DialogTitle>

      <DialogContent>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          Session: <strong>{sessionName}</strong>
        </Typography>

        <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
          Create shareable invitation links that allow anyone with the link to access this session.
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

        {/* Create New Invitation */}
        <Box mt={3}>
          <Typography variant="subtitle1" fontWeight={600} gutterBottom>
            Create Invitation Link
          </Typography>

          <Box display="flex" flexDirection="column" gap={2}>
            <TextField
              select
              label="Permission Level"
              value={permissionLevel}
              onChange={(e) => setPermissionLevel(e.target.value as 'view' | 'collaborate' | 'control')}
              fullWidth
              size="small"
              helperText={
                permissionLevel === 'view'
                  ? 'Recipients can only view the session'
                  : permissionLevel === 'collaborate'
                  ? 'Recipients can view and interact with the session'
                  : 'Recipients can view, interact, and manage the session'
              }
            >
              <MenuItem value="view">View Only</MenuItem>
              <MenuItem value="collaborate">Collaborate</MenuItem>
              <MenuItem value="control">Full Control</MenuItem>
            </TextField>

            <TextField
              label="Maximum Uses"
              type="number"
              value={maxUses}
              onChange={(e) => setMaxUses(parseInt(e.target.value) || 0)}
              fullWidth
              size="small"
              helperText="How many people can use this link (0 for unlimited)"
              inputProps={{ min: 0, max: 1000 }}
            />

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
              startIcon={<Add />}
              onClick={handleCreateInvitation}
              disabled={loading}
              fullWidth
            >
              Create Invitation Link
            </Button>
          </Box>
        </Box>

        <Divider sx={{ my: 3 }} />

        {/* Existing Invitations */}
        <Box>
          <Typography variant="subtitle1" fontWeight={600} gutterBottom>
            Active Invitations ({invitations.filter(i => !i.isExpired && !i.isExhausted).length})
          </Typography>

          {invitations.length === 0 ? (
            <Typography variant="body2" color="text.secondary" fontStyle="italic">
              No invitation links created yet
            </Typography>
          ) : (
            <List>
              {invitations.map((invitation) => {
                const status = getInvitationStatus(invitation);
                const inviteUrl = getInvitationUrl(invitation.invitationToken);
                const isCopied = copiedToken === invitation.invitationToken;

                return (
                  <ListItem
                    key={invitation.id}
                    sx={{
                      border: 1,
                      borderColor: 'divider',
                      borderRadius: 1,
                      mb: 1,
                      flexDirection: 'column',
                      alignItems: 'stretch',
                    }}
                  >
                    <Box display="flex" width="100%" justifyContent="space-between" mb={1}>
                      <ListItemText
                        primary={
                          <Box display="flex" gap={1} alignItems="center" flexWrap="wrap">
                            <Chip
                              label={invitation.permissionLevel}
                              size="small"
                              color={getPermissionColor(invitation.permissionLevel)}
                            />
                            <Chip label={status.label} size="small" color={status.color} />
                            <Chip
                              label={`${invitation.useCount}/${invitation.maxUses || '∞'} used`}
                              size="small"
                              variant="outlined"
                            />
                            {invitation.expiresAt && (
                              <Chip
                                label={`Expires ${formatDate(invitation.expiresAt)}`}
                                size="small"
                                variant="outlined"
                              />
                            )}
                          </Box>
                        }
                        secondary={
                          <Typography variant="caption" color="text.secondary" display="block" mt={0.5}>
                            Created on {formatDate(invitation.createdAt)}
                          </Typography>
                        }
                      />
                      <ListItemSecondaryAction>
                        <IconButton
                          edge="end"
                          color="error"
                          onClick={() => handleRevokeInvitation(invitation.invitationToken)}
                          title="Revoke invitation"
                        >
                          <Delete />
                        </IconButton>
                      </ListItemSecondaryAction>
                    </Box>

                    {/* Invitation URL */}
                    <TextField
                      value={inviteUrl}
                      size="small"
                      fullWidth
                      InputProps={{
                        readOnly: true,
                        endAdornment: (
                          <InputAdornment position="end">
                            <IconButton
                              onClick={() => copyToClipboard(inviteUrl, invitation.invitationToken)}
                              size="small"
                              color={isCopied ? 'success' : 'primary'}
                            >
                              <ContentCopy fontSize="small" />
                            </IconButton>
                          </InputAdornment>
                        ),
                      }}
                      helperText={isCopied ? '✓ Copied to clipboard!' : 'Click the icon to copy'}
                      sx={{
                        '& .MuiInputBase-input': {
                          fontSize: '0.75rem',
                          fontFamily: 'monospace',
                        },
                      }}
                    />
                  </ListItem>
                );
              })}
            </List>
          )}
        </Box>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
}

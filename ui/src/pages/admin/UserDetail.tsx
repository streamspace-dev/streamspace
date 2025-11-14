import { useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControl,
  Grid,
  IconButton,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  ArrowBack as ArrowBackIcon,
  Edit as EditIcon,
  Save as SaveIcon,
  Cancel as CancelIcon,
  Delete as DeleteIcon,
  Groups as GroupsIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type User, type UserQuota, type UpdateUserRequest, type SetQuotaRequest } from '../../lib/api';

export default function UserDetail() {
  const { userId } = useParams<{ userId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [editMode, setEditMode] = useState(false);
  const [editQuotaMode, setEditQuotaMode] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Form state
  const [formData, setFormData] = useState<Partial<UpdateUserRequest>>({});
  const [quotaData, setQuotaData] = useState<Partial<SetQuotaRequest>>({});

  // Fetch user data
  const { data: user, isLoading } = useQuery({
    queryKey: ['user', userId],
    queryFn: () => api.getUser(userId!),
    enabled: !!userId,
  });

  // Fetch user quota
  const { data: quota } = useQuery({
    queryKey: ['user-quota', userId],
    queryFn: () => api.getUserQuota(userId!),
    enabled: !!userId,
  });

  // Fetch user groups
  const { data: userGroups } = useQuery({
    queryKey: ['user-groups', userId],
    queryFn: () => api.getUserGroups(userId!),
    enabled: !!userId,
  });

  // Update user mutation
  const updateUserMutation = useMutation({
    mutationFn: (data: UpdateUserRequest) => api.updateUser(userId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user', userId] });
      setEditMode(false);
      setFormData({});
    },
  });

  // Update quota mutation
  const updateQuotaMutation = useMutation({
    mutationFn: (data: SetQuotaRequest) => api.setUserQuota(userId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-quota', userId] });
      setEditQuotaMode(false);
      setQuotaData({});
    },
  });

  // Delete user mutation
  const deleteUserMutation = useMutation({
    mutationFn: () => api.deleteUser(userId!),
    onSuccess: () => {
      navigate('/admin/users');
    },
  });

  const handleEdit = () => {
    if (user) {
      setFormData({
        email: user.email,
        fullName: user.fullName,
        role: user.role,
        active: user.active,
      });
      setEditMode(true);
    }
  };

  const handleSave = () => {
    updateUserMutation.mutate(formData as UpdateUserRequest);
  };

  const handleCancel = () => {
    setEditMode(false);
    setFormData({});
  };

  const handleEditQuota = () => {
    if (quota) {
      setQuotaData({
        maxSessions: quota.maxSessions,
        maxCpu: quota.maxCpu,
        maxMemory: quota.maxMemory,
        maxStorage: quota.maxStorage,
      });
      setEditQuotaMode(true);
    }
  };

  const handleSaveQuota = () => {
    updateQuotaMutation.mutate(quotaData as SetQuotaRequest);
  };

  const handleCancelQuota = () => {
    setEditQuotaMode(false);
    setQuotaData({});
  };

  const handleDelete = () => {
    deleteUserMutation.mutate();
  };

  if (isLoading || !user) {
    return (
      <Container maxWidth="xl" sx={{ py: 4 }}>
        <Typography>Loading user...</Typography>
      </Container>
    );
  }

  return (
    <Container maxWidth="xl" sx={{ py: 4 }}>
      <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <IconButton onClick={() => navigate('/admin/users')}>
            <ArrowBackIcon />
          </IconButton>
          <Typography variant="h4" component="h1">
            User Details
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          {!editMode && (
            <Button
              variant="outlined"
              startIcon={<EditIcon />}
              onClick={handleEdit}
            >
              Edit
            </Button>
          )}
          <Button
            variant="outlined"
            color="error"
            startIcon={<DeleteIcon />}
            onClick={() => setDeleteDialogOpen(true)}
          >
            Delete
          </Button>
        </Box>
      </Box>

      <Grid container spacing={3}>
        {/* User Information */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                User Information
              </Typography>
              <Divider sx={{ mb: 2 }} />

              {editMode ? (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <TextField
                    label="Username"
                    value={user.username}
                    disabled
                    fullWidth
                    size="small"
                  />
                  <TextField
                    label="Email"
                    value={formData.email || ''}
                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                    fullWidth
                    size="small"
                  />
                  <TextField
                    label="Full Name"
                    value={formData.fullName || ''}
                    onChange={(e) => setFormData({ ...formData, fullName: e.target.value })}
                    fullWidth
                    size="small"
                  />
                  <FormControl fullWidth size="small">
                    <InputLabel>Role</InputLabel>
                    <Select
                      value={formData.role || user.role}
                      label="Role"
                      onChange={(e) => setFormData({ ...formData, role: e.target.value as any })}
                    >
                      <MenuItem value="user">User</MenuItem>
                      <MenuItem value="operator">Operator</MenuItem>
                      <MenuItem value="admin">Admin</MenuItem>
                    </Select>
                  </FormControl>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Typography>Active:</Typography>
                    <Switch
                      checked={formData.active ?? user.active}
                      onChange={(e) => setFormData({ ...formData, active: e.target.checked })}
                    />
                  </Box>
                  <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                    <Button
                      variant="contained"
                      startIcon={<SaveIcon />}
                      onClick={handleSave}
                      disabled={updateUserMutation.isPending}
                    >
                      Save
                    </Button>
                    <Button
                      variant="outlined"
                      startIcon={<CancelIcon />}
                      onClick={handleCancel}
                    >
                      Cancel
                    </Button>
                  </Box>
                </Box>
              ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Username
                    </Typography>
                    <Typography variant="body1">{user.username}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Email
                    </Typography>
                    <Typography variant="body1">{user.email}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Full Name
                    </Typography>
                    <Typography variant="body1">{user.fullName}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Role
                    </Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <Chip label={user.role.toUpperCase()} size="small" />
                    </Box>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Provider
                    </Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <Chip label={user.provider.toUpperCase()} size="small" variant="outlined" />
                    </Box>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Status
                    </Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <Chip
                        label={user.active ? 'Active' : 'Inactive'}
                        size="small"
                        color={user.active ? 'success' : 'default'}
                      />
                    </Box>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Last Login
                    </Typography>
                    <Typography variant="body1">
                      {user.lastLogin ? new Date(user.lastLogin).toLocaleString() : 'Never'}
                    </Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Created
                    </Typography>
                    <Typography variant="body1">
                      {new Date(user.createdAt).toLocaleString()}
                    </Typography>
                  </Box>
                </Box>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Resource Quota */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">
                  Resource Quota
                </Typography>
                {!editQuotaMode && quota && (
                  <Button
                    size="small"
                    startIcon={<EditIcon />}
                    onClick={handleEditQuota}
                  >
                    Edit
                  </Button>
                )}
              </Box>
              <Divider sx={{ mb: 2 }} />

              {quota ? (
                editQuotaMode ? (
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <TextField
                      label="Max Sessions"
                      type="number"
                      value={quotaData.maxSessions || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxSessions: parseInt(e.target.value) })}
                      fullWidth
                      size="small"
                    />
                    <TextField
                      label="Max CPU"
                      value={quotaData.maxCpu || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxCpu: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 4000m"
                    />
                    <TextField
                      label="Max Memory"
                      value={quotaData.maxMemory || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxMemory: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 16Gi"
                    />
                    <TextField
                      label="Max Storage"
                      value={quotaData.maxStorage || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxStorage: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 100Gi"
                    />
                    <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                      <Button
                        variant="contained"
                        startIcon={<SaveIcon />}
                        onClick={handleSaveQuota}
                        disabled={updateQuotaMutation.isPending}
                      >
                        Save
                      </Button>
                      <Button
                        variant="outlined"
                        startIcon={<CancelIcon />}
                        onClick={handleCancelQuota}
                      >
                        Cancel
                      </Button>
                    </Box>
                  </Box>
                ) : (
                  <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <Box>
                      <Typography variant="caption" color="text.secondary">
                        Sessions
                      </Typography>
                      <Typography variant="body1">
                        {quota.usedSessions} / {quota.maxSessions}
                      </Typography>
                    </Box>
                    <Box>
                      <Typography variant="caption" color="text.secondary">
                        CPU
                      </Typography>
                      <Typography variant="body1">
                        {quota.usedCpu} / {quota.maxCpu}
                      </Typography>
                    </Box>
                    <Box>
                      <Typography variant="caption" color="text.secondary">
                        Memory
                      </Typography>
                      <Typography variant="body1">
                        {quota.usedMemory} / {quota.maxMemory}
                      </Typography>
                    </Box>
                    <Box>
                      <Typography variant="caption" color="text.secondary">
                        Storage
                      </Typography>
                      <Typography variant="body1">
                        {quota.usedStorage} / {quota.maxStorage}
                      </Typography>
                    </Box>
                  </Box>
                )
              ) : (
                <Typography color="text.secondary">
                  No quota set for this user
                </Typography>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* User Groups */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
                <GroupsIcon />
                <Typography variant="h6">
                  Group Memberships
                </Typography>
              </Box>
              <Divider sx={{ mb: 2 }} />

              {userGroups && userGroups.groups.length > 0 ? (
                <TableContainer component={Paper} variant="outlined">
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Name</TableCell>
                        <TableCell>Display Name</TableCell>
                        <TableCell>Type</TableCell>
                        <TableCell>Members</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {userGroups.groups.map((group) => (
                        <TableRow key={group.id} hover>
                          <TableCell>{group.name}</TableCell>
                          <TableCell>{group.displayName}</TableCell>
                          <TableCell>
                            <Chip label={group.type} size="small" variant="outlined" />
                          </TableCell>
                          <TableCell>{group.memberCount || 0}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              ) : (
                <Typography color="text.secondary">
                  User is not a member of any groups
                </Typography>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
        <DialogTitle>Delete User</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete user <strong>{user.username}</strong>?
            This action cannot be undone and will delete all associated sessions and data.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleDelete}
            color="error"
            variant="contained"
            disabled={deleteUserMutation.isPending}
          >
            {deleteUserMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}

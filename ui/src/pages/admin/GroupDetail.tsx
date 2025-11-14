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
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
  Autocomplete,
} from '@mui/material';
import {
  ArrowBack as ArrowBackIcon,
  Edit as EditIcon,
  Save as SaveIcon,
  Cancel as CancelIcon,
  Delete as DeleteIcon,
  PersonAdd as PersonAddIcon,
  PersonRemove as PersonRemoveIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  api,
  type Group,
  type GroupQuota,
  type UpdateGroupRequest,
  type SetQuotaRequest,
  type AddGroupMemberRequest,
  type User,
} from '../../lib/api';

export default function GroupDetail() {
  const { groupId } = useParams<{ groupId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [editMode, setEditMode] = useState(false);
  const [editQuotaMode, setEditQuotaMode] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [addMemberDialogOpen, setAddMemberDialogOpen] = useState(false);
  const [removeMemberDialogOpen, setRemoveMemberDialogOpen] = useState(false);
  const [memberToRemove, setMemberToRemove] = useState<string | null>(null);

  // Form state
  const [formData, setFormData] = useState<Partial<UpdateGroupRequest>>({});
  const [quotaData, setQuotaData] = useState<Partial<SetQuotaRequest>>({});
  const [newMember, setNewMember] = useState<{ userId: string; role: string }>({
    userId: '',
    role: 'member',
  });

  // Fetch group data
  const { data: group, isLoading } = useQuery({
    queryKey: ['group', groupId],
    queryFn: () => api.getGroup(groupId!),
    enabled: !!groupId,
  });

  // Fetch group quota
  const { data: quota } = useQuery({
    queryKey: ['group-quota', groupId],
    queryFn: () => api.getGroupQuota(groupId!),
    enabled: !!groupId,
  });

  // Fetch group members
  const { data: membersData } = useQuery({
    queryKey: ['group-members', groupId],
    queryFn: () => api.getGroupMembers(groupId!),
    enabled: !!groupId,
  });

  // Fetch all users for member selection
  const { data: allUsers = [] } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.listUsers(),
  });

  // Update group mutation
  const updateGroupMutation = useMutation({
    mutationFn: (data: UpdateGroupRequest) => api.updateGroup(groupId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group', groupId] });
      setEditMode(false);
      setFormData({});
    },
  });

  // Update quota mutation
  const updateQuotaMutation = useMutation({
    mutationFn: (data: SetQuotaRequest) => api.setGroupQuota(groupId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-quota', groupId] });
      setEditQuotaMode(false);
      setQuotaData({});
    },
  });

  // Add member mutation
  const addMemberMutation = useMutation({
    mutationFn: (data: AddGroupMemberRequest) => api.addGroupMember(groupId!, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-members', groupId] });
      setAddMemberDialogOpen(false);
      setNewMember({ userId: '', role: 'member' });
    },
  });

  // Remove member mutation
  const removeMemberMutation = useMutation({
    mutationFn: (userId: string) => api.removeGroupMember(groupId!, userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['group-members', groupId] });
      setRemoveMemberDialogOpen(false);
      setMemberToRemove(null);
    },
  });

  // Delete group mutation
  const deleteGroupMutation = useMutation({
    mutationFn: () => api.deleteGroup(groupId!),
    onSuccess: () => {
      navigate('/admin/groups');
    },
  });

  const handleEdit = () => {
    if (group) {
      setFormData({
        displayName: group.displayName,
        description: group.description,
        type: group.type,
      });
      setEditMode(true);
    }
  };

  const handleSave = () => {
    updateGroupMutation.mutate(formData as UpdateGroupRequest);
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

  const handleAddMember = () => {
    if (newMember.userId) {
      addMemberMutation.mutate(newMember);
    }
  };

  const handleRemoveMemberClick = (userId: string) => {
    setMemberToRemove(userId);
    setRemoveMemberDialogOpen(true);
  };

  const handleRemoveMemberConfirm = () => {
    if (memberToRemove) {
      removeMemberMutation.mutate(memberToRemove);
    }
  };

  const handleDelete = () => {
    deleteGroupMutation.mutate();
  };

  // Filter out users who are already members
  const availableUsers = allUsers.filter(
    (user) => !membersData?.members.some((m) => m.userId === user.id)
  );

  if (isLoading || !group) {
    return (
      <Container maxWidth="xl" sx={{ py: 4 }}>
        <Typography>Loading group...</Typography>
      </Container>
    );
  }

  return (
    <Container maxWidth="xl" sx={{ py: 4 }}>
      <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          <IconButton onClick={() => navigate('/admin/groups')}>
            <ArrowBackIcon />
          </IconButton>
          <Typography variant="h4" component="h1">
            Group Details
          </Typography>
        </Box>
        <Box sx={{ display: 'flex', gap: 2 }}>
          {!editMode && group.type !== 'system' && (
            <Button variant="outlined" startIcon={<EditIcon />} onClick={handleEdit}>
              Edit
            </Button>
          )}
          {group.type !== 'system' && (
            <Button
              variant="outlined"
              color="error"
              startIcon={<DeleteIcon />}
              onClick={() => setDeleteDialogOpen(true)}
            >
              Delete
            </Button>
          )}
        </Box>
      </Box>

      <Grid container spacing={3}>
        {/* Group Information */}
        <Grid item xs={12} md={6}>
          <Card>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Group Information
              </Typography>
              <Divider sx={{ mb: 2 }} />

              {editMode ? (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <TextField
                    label="Name"
                    value={group.name}
                    disabled
                    fullWidth
                    size="small"
                  />
                  <TextField
                    label="Display Name"
                    value={formData.displayName || ''}
                    onChange={(e) => setFormData({ ...formData, displayName: e.target.value })}
                    fullWidth
                    size="small"
                  />
                  <TextField
                    label="Description"
                    value={formData.description || ''}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    fullWidth
                    size="small"
                    multiline
                    rows={3}
                  />
                  <FormControl fullWidth size="small">
                    <InputLabel>Type</InputLabel>
                    <Select
                      value={formData.type || group.type}
                      label="Type"
                      onChange={(e) => setFormData({ ...formData, type: e.target.value })}
                    >
                      <MenuItem value="organization">Organization</MenuItem>
                      <MenuItem value="team">Team</MenuItem>
                      <MenuItem value="project">Project</MenuItem>
                    </Select>
                  </FormControl>
                  <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                    <Button
                      variant="contained"
                      startIcon={<SaveIcon />}
                      onClick={handleSave}
                      disabled={updateGroupMutation.isPending}
                    >
                      Save
                    </Button>
                    <Button variant="outlined" startIcon={<CancelIcon />} onClick={handleCancel}>
                      Cancel
                    </Button>
                  </Box>
                </Box>
              ) : (
                <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Name
                    </Typography>
                    <Typography variant="body1">{group.name}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Display Name
                    </Typography>
                    <Typography variant="body1">{group.displayName}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Description
                    </Typography>
                    <Typography variant="body1">{group.description || '-'}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Type
                    </Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <Chip label={group.type.toUpperCase()} size="small" />
                    </Box>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Members
                    </Typography>
                    <Typography variant="body1">{group.memberCount || 0}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">
                      Created
                    </Typography>
                    <Typography variant="body1">
                      {new Date(group.createdAt).toLocaleString()}
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
                <Typography variant="h6">Resource Quota</Typography>
                {!editQuotaMode && quota && (
                  <Button size="small" startIcon={<EditIcon />} onClick={handleEditQuota}>
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
                      onChange={(e) =>
                        setQuotaData({ ...quotaData, maxSessions: parseInt(e.target.value) })
                      }
                      fullWidth
                      size="small"
                    />
                    <TextField
                      label="Max CPU"
                      value={quotaData.maxCpu || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxCpu: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 16000m"
                    />
                    <TextField
                      label="Max Memory"
                      value={quotaData.maxMemory || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxMemory: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 64Gi"
                    />
                    <TextField
                      label="Max Storage"
                      value={quotaData.maxStorage || ''}
                      onChange={(e) => setQuotaData({ ...quotaData, maxStorage: e.target.value })}
                      fullWidth
                      size="small"
                      placeholder="e.g., 500Gi"
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
                <Typography color="text.secondary">No quota set for this group</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>

        {/* Group Members */}
        <Grid item xs={12}>
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Members ({membersData?.total || 0})</Typography>
                <Button
                  variant="outlined"
                  startIcon={<PersonAddIcon />}
                  onClick={() => setAddMemberDialogOpen(true)}
                >
                  Add Member
                </Button>
              </Box>
              <Divider sx={{ mb: 2 }} />

              {membersData && membersData.members.length > 0 ? (
                <TableContainer component={Paper} variant="outlined">
                  <Table size="small">
                    <TableHead>
                      <TableRow>
                        <TableCell>Username</TableCell>
                        <TableCell>Email</TableCell>
                        <TableCell>Role in Group</TableCell>
                        <TableCell>Joined</TableCell>
                        <TableCell align="right">Actions</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {membersData.members.map((member) => (
                        <TableRow key={member.userId} hover>
                          <TableCell>{member.username}</TableCell>
                          <TableCell>{member.email}</TableCell>
                          <TableCell>
                            <Chip label={member.role.toUpperCase()} size="small" variant="outlined" />
                          </TableCell>
                          <TableCell>{new Date(member.joinedAt).toLocaleDateString()}</TableCell>
                          <TableCell align="right">
                            <Tooltip title="Remove from group">
                              <IconButton
                                size="small"
                                color="error"
                                onClick={() => handleRemoveMemberClick(member.userId)}
                              >
                                <PersonRemoveIcon fontSize="small" />
                              </IconButton>
                            </Tooltip>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              ) : (
                <Typography color="text.secondary">No members in this group</Typography>
              )}
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      {/* Add Member Dialog */}
      <Dialog open={addMemberDialogOpen} onClose={() => setAddMemberDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Add Member</DialogTitle>
        <DialogContent>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 1 }}>
            <Autocomplete
              options={availableUsers}
              getOptionLabel={(user) => `${user.username} (${user.email})`}
              onChange={(_, user) => setNewMember({ ...newMember, userId: user?.id || '' })}
              renderInput={(params) => <TextField {...params} label="Select User" />}
            />
            <FormControl fullWidth>
              <InputLabel>Role</InputLabel>
              <Select
                value={newMember.role}
                label="Role"
                onChange={(e) => setNewMember({ ...newMember, role: e.target.value })}
              >
                <MenuItem value="member">Member</MenuItem>
                <MenuItem value="admin">Admin</MenuItem>
                <MenuItem value="owner">Owner</MenuItem>
              </Select>
            </FormControl>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setAddMemberDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleAddMember}
            variant="contained"
            disabled={!newMember.userId || addMemberMutation.isPending}
          >
            {addMemberMutation.isPending ? 'Adding...' : 'Add Member'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Remove Member Dialog */}
      <Dialog open={removeMemberDialogOpen} onClose={() => setRemoveMemberDialogOpen(false)}>
        <DialogTitle>Remove Member</DialogTitle>
        <DialogContent>
          <Typography>Are you sure you want to remove this member from the group?</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRemoveMemberDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleRemoveMemberConfirm}
            color="error"
            variant="contained"
            disabled={removeMemberMutation.isPending}
          >
            {removeMemberMutation.isPending ? 'Removing...' : 'Remove'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete Group Dialog */}
      <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
        <DialogTitle>Delete Group</DialogTitle>
        <DialogContent>
          <Typography>
            Are you sure you want to delete group <strong>{group.displayName}</strong>? This action
            cannot be undone. Group members will not be deleted.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleDelete}
            color="error"
            variant="contained"
            disabled={deleteGroupMutation.isPending}
          >
            {deleteGroupMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
  );
}

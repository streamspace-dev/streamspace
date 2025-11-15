import { useState } from 'react';
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
  DialogContentText,
  DialogTitle,
  FormControl,
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
} from '@mui/material';
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
  Refresh as RefreshIcon,
  Groups as GroupsIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { api, type Group } from '../../lib/api';
import { useGroupEvents } from '../../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../../components/NotificationQueue';
import EnhancedWebSocketStatus from '../../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../../components/WebSocketErrorBoundary';

export default function Groups() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  // Filters
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [searchQuery, setSearchQuery] = useState('');

  // Delete confirmation dialog
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [groupToDelete, setGroupToDelete] = useState<Group | null>(null);

  // WebSocket connection state
  const [wsConnected, setWsConnected] = useState(false);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time group events via WebSocket with notifications
  useGroupEvents((data: any) => {
    console.log('Group event:', data);
    setWsConnected(true);

    // Show notifications for group events
    if (data.event_type === 'group.created') {
      addNotification({
        message: `New group created: ${data.group_name}`,
        severity: 'info',
        priority: 'medium',
        title: 'Group Created',
      });
      // Refresh group list
      queryClient.invalidateQueries({ queryKey: ['groups'] });
    } else if (data.event_type === 'group.updated') {
      addNotification({
        message: `Group ${data.group_name} has been updated`,
        severity: 'info',
        priority: 'low',
        title: 'Group Updated',
      });
      // Refresh group list
      queryClient.invalidateQueries({ queryKey: ['groups'] });
    } else if (data.event_type === 'group.deleted') {
      addNotification({
        message: `Group ${data.group_name} has been deleted`,
        severity: 'warning',
        priority: 'medium',
        title: 'Group Deleted',
      });
      // Refresh group list
      queryClient.invalidateQueries({ queryKey: ['groups'] });
    } else if (data.event_type === 'group.member_added') {
      addNotification({
        message: `Member added to group ${data.group_name}`,
        severity: 'success',
        priority: 'low',
        title: 'Member Added',
      });
    } else if (data.event_type === 'group.member_removed') {
      addNotification({
        message: `Member removed from group ${data.group_name}`,
        severity: 'info',
        priority: 'low',
        title: 'Member Removed',
      });
    }
  });

  // Fetch groups
  const { data: groups = [], isLoading, refetch } = useQuery({
    queryKey: ['groups', typeFilter],
    queryFn: () => api.listGroups(typeFilter || undefined),
  });

  // Delete group mutation
  const deleteGroupMutation = useMutation({
    mutationFn: (groupId: string) => api.deleteGroup(groupId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      setDeleteDialogOpen(false);
      setGroupToDelete(null);
    },
  });

  // Filter groups by search query
  const filteredGroups = groups.filter(group =>
    group.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    group.displayName.toLowerCase().includes(searchQuery.toLowerCase()) ||
    (group.description && group.description.toLowerCase().includes(searchQuery.toLowerCase()))
  );

  const handleDeleteClick = (group: Group) => {
    setGroupToDelete(group);
    setDeleteDialogOpen(true);
  };

  const handleDeleteConfirm = () => {
    if (groupToDelete) {
      deleteGroupMutation.mutate(groupToDelete.id);
    }
  };

  const getTypeColor = (type: string) => {
    switch (type.toLowerCase()) {
      case 'system':
        return 'error';
      case 'organization':
        return 'primary';
      case 'team':
        return 'success';
      case 'project':
        return 'info';
      default:
        return 'default';
    }
  };

  return (
    <WebSocketErrorBoundary>
      <Container maxWidth="xl" sx={{ py: 4 }}>
        <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <GroupsIcon sx={{ fontSize: 40 }} />
            <Typography variant="h4" component="h1">
              Group Management
            </Typography>

            {/* Enhanced WebSocket Connection Status */}
            <EnhancedWebSocketStatus
              isConnected={wsConnected}
              reconnectAttempts={0}
              size="small"
              showDetails={true}
            />
          </Box>
          <Box sx={{ display: 'flex', gap: 2 }}>
            <Tooltip title="Refresh">
              <IconButton onClick={() => refetch()}>
                <RefreshIcon />
              </IconButton>
            </Tooltip>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => navigate('/admin/groups/create')}
            >
              Create Group
            </Button>
          </Box>
        </Box>

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Filters
          </Typography>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
            <TextField
              label="Search"
              variant="outlined"
              size="small"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Name or description..."
              sx={{ minWidth: 250 }}
            />
            <FormControl size="small" sx={{ minWidth: 150 }}>
              <InputLabel>Type</InputLabel>
              <Select
                value={typeFilter}
                label="Type"
                onChange={(e) => setTypeFilter(e.target.value)}
              >
                <MenuItem value="">All</MenuItem>
                <MenuItem value="system">System</MenuItem>
                <MenuItem value="organization">Organization</MenuItem>
                <MenuItem value="team">Team</MenuItem>
                <MenuItem value="project">Project</MenuItem>
              </Select>
            </FormControl>
          </Box>
        </CardContent>
      </Card>

      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>Name</TableCell>
              <TableCell>Display Name</TableCell>
              <TableCell>Description</TableCell>
              <TableCell>Type</TableCell>
              <TableCell>Members</TableCell>
              <TableCell>Created</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={7} align="center">
                  Loading groups...
                </TableCell>
              </TableRow>
            ) : filteredGroups.length === 0 ? (
              <TableRow>
                <TableCell colSpan={7} align="center">
                  No groups found
                </TableCell>
              </TableRow>
            ) : (
              filteredGroups.map((group) => (
                <TableRow key={group.id} hover>
                  <TableCell>
                    <Typography variant="body2" fontWeight="medium">
                      {group.name}
                    </Typography>
                  </TableCell>
                  <TableCell>{group.displayName}</TableCell>
                  <TableCell>
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{
                        maxWidth: 300,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {group.description || '-'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={group.type.toUpperCase()}
                      size="small"
                      color={getTypeColor(group.type)}
                    />
                  </TableCell>
                  <TableCell>{group.memberCount || 0}</TableCell>
                  <TableCell>
                    {new Date(group.createdAt).toLocaleDateString()}
                  </TableCell>
                  <TableCell align="right">
                    <Tooltip title="Edit">
                      <IconButton
                        size="small"
                        onClick={() => navigate(`/admin/groups/${group.id}`)}
                      >
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Delete">
                      <IconButton
                        size="small"
                        onClick={() => handleDeleteClick(group)}
                        color="error"
                        disabled={group.type === 'system'}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      <Box sx={{ mt: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography variant="body2" color="text.secondary">
          Showing {filteredGroups.length} of {groups.length} groups
        </Typography>
      </Box>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
        <DialogTitle>Delete Group</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete group <strong>{groupToDelete?.displayName}</strong>?
            This action cannot be undone. Group members will not be deleted.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
          <Button
            onClick={handleDeleteConfirm}
            color="error"
            variant="contained"
            disabled={deleteGroupMutation.isPending}
          >
            {deleteGroupMutation.isPending ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Container>
    </WebSocketErrorBoundary>
  );
}

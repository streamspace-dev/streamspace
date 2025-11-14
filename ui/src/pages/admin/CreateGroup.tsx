import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  FormControl,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
  Alert,
} from '@mui/material';
import {
  ArrowBack as ArrowBackIcon,
  Save as SaveIcon,
  Cancel as CancelIcon,
  GroupAdd as GroupAddIcon,
} from '@mui/icons-material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type CreateGroupRequest } from '../../lib/api';

export default function CreateGroup() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [formData, setFormData] = useState<CreateGroupRequest>({
    name: '',
    displayName: '',
    description: '',
    type: 'team',
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  // Create group mutation
  const createGroupMutation = useMutation({
    mutationFn: (data: CreateGroupRequest) => api.createGroup(data),
    onSuccess: (group) => {
      queryClient.invalidateQueries({ queryKey: ['groups'] });
      navigate(`/admin/groups/${group.id}`);
    },
    onError: (error: any) => {
      if (error.response?.data?.message) {
        setErrors({ general: error.response.data.message });
      } else {
        setErrors({ general: 'Failed to create group' });
      }
    },
  });

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.name) {
      newErrors.name = 'Name is required';
    } else if (!/^[a-z0-9_-]+$/.test(formData.name)) {
      newErrors.name =
        'Name can only contain lowercase letters, numbers, hyphens, and underscores';
    }

    if (!formData.displayName) {
      newErrors.displayName = 'Display name is required';
    }

    if (!formData.type) {
      newErrors.type = 'Type is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (validateForm()) {
      createGroupMutation.mutate(formData);
    }
  };

  const handleCancel = () => {
    navigate('/admin/groups');
  };

  return (
    <Container maxWidth="md" sx={{ py: 4 }}>
      <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', gap: 2 }}>
        <IconButton onClick={handleCancel}>
          <ArrowBackIcon />
        </IconButton>
        <GroupAddIcon sx={{ fontSize: 32 }} />
        <Typography variant="h4" component="h1">
          Create New Group
        </Typography>
      </Box>

      {errors.general && (
        <Alert severity="error" sx={{ mb: 3 }}>
          {errors.general}
        </Alert>
      )}

      <Card>
        <CardContent>
          <form onSubmit={handleSubmit}>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
              <TextField
                label="Name"
                value={formData.name}
                onChange={(e) =>
                  setFormData({ ...formData, name: e.target.value.toLowerCase() })
                }
                error={!!errors.name}
                helperText={
                  errors.name || 'Lowercase letters, numbers, hyphens, and underscores only'
                }
                required
                fullWidth
                autoFocus
              />

              <TextField
                label="Display Name"
                value={formData.displayName}
                onChange={(e) => setFormData({ ...formData, displayName: e.target.value })}
                error={!!errors.displayName}
                helperText={errors.displayName || 'Human-readable name for the group'}
                required
                fullWidth
              />

              <TextField
                label="Description"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                multiline
                rows={4}
                fullWidth
                placeholder="Optional description of the group's purpose..."
              />

              <FormControl fullWidth required>
                <InputLabel>Type</InputLabel>
                <Select
                  value={formData.type}
                  label="Type"
                  onChange={(e) => setFormData({ ...formData, type: e.target.value })}
                  error={!!errors.type}
                >
                  <MenuItem value="organization">Organization</MenuItem>
                  <MenuItem value="team">Team</MenuItem>
                  <MenuItem value="project">Project</MenuItem>
                </Select>
              </FormControl>

              <Box sx={{ display: 'flex', gap: 2, mt: 2 }}>
                <Button
                  type="submit"
                  variant="contained"
                  startIcon={<SaveIcon />}
                  disabled={createGroupMutation.isPending}
                  fullWidth
                >
                  {createGroupMutation.isPending ? 'Creating...' : 'Create Group'}
                </Button>
                <Button
                  variant="outlined"
                  startIcon={<CancelIcon />}
                  onClick={handleCancel}
                  fullWidth
                >
                  Cancel
                </Button>
              </Box>
            </Box>
          </form>
        </CardContent>
      </Card>

      <Box sx={{ mt: 3 }}>
        <Alert severity="info">
          <Typography variant="body2">
            <strong>Group Types:</strong>
          </Typography>
          <Typography variant="body2" component="ul" sx={{ mt: 1, pl: 2 }}>
            <li>
              <strong>Organization:</strong> Top-level groups representing departments or
              organizations
            </li>
            <li>
              <strong>Team:</strong> Groups of users working together on related tasks
            </li>
            <li>
              <strong>Project:</strong> Groups associated with specific projects or initiatives
            </li>
          </Typography>
        </Alert>
      </Box>
    </Container>
  );
}

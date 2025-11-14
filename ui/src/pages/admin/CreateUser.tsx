import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  FormControl,
  FormControlLabel,
  IconButton,
  InputLabel,
  MenuItem,
  Select,
  Switch,
  TextField,
  Typography,
  Alert,
} from '@mui/material';
import {
  ArrowBack as ArrowBackIcon,
  Save as SaveIcon,
  Cancel as CancelIcon,
  PersonAdd as PersonAddIcon,
} from '@mui/icons-material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api, type CreateUserRequest } from '../../lib/api';

export default function CreateUser() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [formData, setFormData] = useState<CreateUserRequest>({
    username: '',
    email: '',
    fullName: '',
    password: '',
    role: 'user',
    provider: 'local',
    active: true,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [passwordConfirm, setPasswordConfirm] = useState('');

  // Create user mutation
  const createUserMutation = useMutation({
    mutationFn: (data: CreateUserRequest) => api.createUser(data),
    onSuccess: (user) => {
      queryClient.invalidateQueries({ queryKey: ['users'] });
      navigate(`/admin/users/${user.id}`);
    },
    onError: (error: any) => {
      if (error.response?.data?.message) {
        setErrors({ general: error.response.data.message });
      } else {
        setErrors({ general: 'Failed to create user' });
      }
    },
  });

  const validateForm = (): boolean => {
    const newErrors: Record<string, string> = {};

    if (!formData.username) {
      newErrors.username = 'Username is required';
    } else if (!/^[a-z0-9_-]+$/.test(formData.username)) {
      newErrors.username = 'Username can only contain lowercase letters, numbers, hyphens, and underscores';
    }

    if (!formData.email) {
      newErrors.email = 'Email is required';
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      newErrors.email = 'Invalid email format';
    }

    if (!formData.fullName) {
      newErrors.fullName = 'Full name is required';
    }

    if (formData.provider === 'local') {
      if (!formData.password) {
        newErrors.password = 'Password is required for local users';
      } else if (formData.password.length < 8) {
        newErrors.password = 'Password must be at least 8 characters';
      }

      if (formData.password !== passwordConfirm) {
        newErrors.passwordConfirm = 'Passwords do not match';
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (validateForm()) {
      createUserMutation.mutate(formData);
    }
  };

  const handleCancel = () => {
    navigate('/admin/users');
  };

  return (
    <Container maxWidth="md" sx={{ py: 4 }}>
      <Box sx={{ mb: 4, display: 'flex', alignItems: 'center', gap: 2 }}>
        <IconButton onClick={handleCancel}>
          <ArrowBackIcon />
        </IconButton>
        <PersonAddIcon sx={{ fontSize: 32 }} />
        <Typography variant="h4" component="h1">
          Create New User
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
                label="Username"
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value.toLowerCase() })}
                error={!!errors.username}
                helperText={errors.username || 'Lowercase letters, numbers, hyphens, and underscores only'}
                required
                fullWidth
                autoFocus
              />

              <TextField
                label="Email"
                type="email"
                value={formData.email}
                onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                error={!!errors.email}
                helperText={errors.email}
                required
                fullWidth
              />

              <TextField
                label="Full Name"
                value={formData.fullName}
                onChange={(e) => setFormData({ ...formData, fullName: e.target.value })}
                error={!!errors.fullName}
                helperText={errors.fullName}
                required
                fullWidth
              />

              <FormControl fullWidth required>
                <InputLabel>Role</InputLabel>
                <Select
                  value={formData.role}
                  label="Role"
                  onChange={(e) => setFormData({ ...formData, role: e.target.value as any })}
                >
                  <MenuItem value="user">User</MenuItem>
                  <MenuItem value="operator">Operator</MenuItem>
                  <MenuItem value="admin">Admin</MenuItem>
                </Select>
              </FormControl>

              <FormControl fullWidth required>
                <InputLabel>Authentication Provider</InputLabel>
                <Select
                  value={formData.provider}
                  label="Authentication Provider"
                  onChange={(e) => setFormData({ ...formData, provider: e.target.value as any })}
                >
                  <MenuItem value="local">Local</MenuItem>
                  <MenuItem value="saml">SAML</MenuItem>
                  <MenuItem value="oidc">OIDC</MenuItem>
                </Select>
              </FormControl>

              {formData.provider === 'local' && (
                <>
                  <TextField
                    label="Password"
                    type="password"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    error={!!errors.password}
                    helperText={errors.password || 'Minimum 8 characters'}
                    required
                    fullWidth
                  />

                  <TextField
                    label="Confirm Password"
                    type="password"
                    value={passwordConfirm}
                    onChange={(e) => setPasswordConfirm(e.target.value)}
                    error={!!errors.passwordConfirm}
                    helperText={errors.passwordConfirm}
                    required
                    fullWidth
                  />
                </>
              )}

              <FormControlLabel
                control={
                  <Switch
                    checked={formData.active}
                    onChange={(e) => setFormData({ ...formData, active: e.target.checked })}
                  />
                }
                label="Active"
              />

              <Box sx={{ display: 'flex', gap: 2, mt: 2 }}>
                <Button
                  type="submit"
                  variant="contained"
                  startIcon={<SaveIcon />}
                  disabled={createUserMutation.isPending}
                  fullWidth
                >
                  {createUserMutation.isPending ? 'Creating...' : 'Create User'}
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
            <strong>Note:</strong> For SAML and OIDC users, authentication is handled by the external provider.
            Users will be automatically created or updated on their first login.
          </Typography>
        </Alert>
      </Box>
    </Container>
  );
}

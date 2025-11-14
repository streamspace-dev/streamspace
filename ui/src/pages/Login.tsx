import { useState } from 'react';
import {
  Box,
  Paper,
  TextField,
  Button,
  Typography,
  Container,
  Alert,
  Divider,
  CircularProgress,
} from '@mui/material';
import { Computer as ComputerIcon, Login as LoginIcon } from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { useUserStore } from '../store/userStore';
import { api } from '../lib/api';

// Authentication mode from environment
const AUTH_MODE = import.meta.env.VITE_AUTH_MODE || 'jwt';
const SAML_LOGIN_URL = import.meta.env.VITE_SAML_LOGIN_URL || '/saml/login';

export default function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();
  const setUser = useUserStore((state) => state.setUser);

  const handleJWTLogin = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!username.trim()) {
      setError('Please enter a username');
      return;
    }

    if (!password.trim() && AUTH_MODE !== 'jwt') {
      setError('Please enter a password');
      return;
    }

    setLoading(true);
    setError('');

    try {
      if (AUTH_MODE === 'jwt') {
        // JWT authentication
        const result = await api.login(username, password);

        // Store token and user info
        localStorage.setItem('streamspace_token', result.token);
        localStorage.setItem('streamspace_user', JSON.stringify(result.user));

        // Update user store
        const role = result.user.role || (username === 'admin' ? 'admin' : 'user');
        setUser(username, role);

        navigate('/');
      } else {
        // Demo mode for development
        const role = username === 'admin' ? 'admin' : 'user';
        setUser(username, role);
        navigate('/');
      }
    } catch (err: any) {
      console.error('Login failed:', err);
      setError(err.response?.data?.message || 'Login failed. Please check your credentials.');
    } finally {
      setLoading(false);
    }
  };

  const handleSAMLLogin = () => {
    // Redirect to SAML login endpoint
    window.location.href = SAML_LOGIN_URL;
  };

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'background.default',
      }}
    >
      <Container maxWidth="sm">
        <Paper
          elevation={3}
          sx={{
            p: 4,
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
          }}
        >
          <ComputerIcon sx={{ fontSize: 60, color: 'primary.main', mb: 2 }} />
          <Typography component="h1" variant="h4" sx={{ mb: 1, fontWeight: 700 }}>
            StreamSpace
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Stream any containerized application to your browser
          </Typography>

          {error && (
            <Alert severity="error" sx={{ width: '100%', mb: 2 }}>
              {error}
            </Alert>
          )}

          {(AUTH_MODE === 'jwt' || AUTH_MODE === 'hybrid') && (
            <Box component="form" onSubmit={handleJWTLogin} sx={{ width: '100%' }}>
              <TextField
                margin="normal"
                required
                fullWidth
                id="username"
                label="Username"
                name="username"
                autoComplete="username"
                autoFocus
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                disabled={loading}
              />
              <TextField
                margin="normal"
                required
                fullWidth
                name="password"
                label="Password"
                type="password"
                id="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={loading}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                size="large"
                sx={{ mt: 3, mb: 2 }}
                disabled={loading}
                startIcon={loading ? <CircularProgress size={20} /> : <LoginIcon />}
              >
                {loading ? 'Signing in...' : 'Sign In'}
              </Button>
            </Box>
          )}

          {AUTH_MODE === 'hybrid' && (
            <Divider sx={{ my: 2, width: '100%' }}>
              <Typography variant="caption" color="text.secondary">
                OR
              </Typography>
            </Divider>
          )}

          {(AUTH_MODE === 'saml' || AUTH_MODE === 'hybrid') && (
            <Button
              fullWidth
              variant="outlined"
              size="large"
              onClick={handleSAMLLogin}
              sx={{ mb: 2 }}
            >
              Sign in with SSO
            </Button>
          )}

          {AUTH_MODE !== 'jwt' && AUTH_MODE !== 'saml' && AUTH_MODE !== 'hybrid' && (
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', textAlign: 'center' }}>
              Demo Mode: Enter any username to continue
            </Typography>
          )}
        </Paper>
      </Container>
    </Box>
  );
}

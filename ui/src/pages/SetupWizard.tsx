import { useState, useEffect } from 'react';
import {
  Box,
  Paper,
  TextField,
  Button,
  Typography,
  Container,
  Alert,
  CircularProgress,
  LinearProgress,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
} from '@mui/material';
import {
  AdminPanelSettings as AdminIcon,
  Check as CheckIcon,
  Error as ErrorIcon,
  Info as InfoIcon,
} from '@mui/icons-material';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';

/**
 * SetupWizard - First-run admin user onboarding
 *
 * Provides a browser-based setup wizard for configuring the initial admin password
 * when no password has been set via Helm chart or environment variable.
 *
 * Features:
 * - Check if setup is required (admin has no password)
 * - Password strength validation (12+ characters for admin)
 * - Password confirmation to prevent typos
 * - Email validation for admin contact
 * - Auto-redirect to login after successful setup
 * - Fallback for account recovery
 *
 * Priority in admin onboarding:
 * 1. Helm Chart with Kubernetes Secret (production)
 * 2. ADMIN_PASSWORD environment variable (docker-compose, manual)
 * 3. Setup Wizard (this page) - fallback for first-run or recovery
 *
 * @page
 * @route /setup - Admin setup wizard (public, only when setup required)
 * @access public - Available to anyone when admin has no password
 *
 * @component
 *
 * @returns {JSX.Element} Setup wizard interface
 *
 * @example
 * // Route configuration:
 * <Route path="/setup" element={<SetupWizard />} />
 */
export default function SetupWizard() {
  const [password, setPassword] = useState('');
  const [passwordConfirm, setPasswordConfirm] = useState('');
  const [email, setEmail] = useState('admin@streamspace.local');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [loading, setLoading] = useState(false);
  const [checkingStatus, setCheckingStatus] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);
  const [statusMessage, setStatusMessage] = useState('');
  const navigate = useNavigate();

  // Check if setup is required on component mount
  useEffect(() => {
    const checkSetupStatus = async () => {
      try {
        const status = await api.getSetupStatus();
        setSetupRequired(status.setupRequired);
        setStatusMessage(status.message || '');

        if (!status.setupRequired) {
          // Setup not required - redirect to login after 3 seconds
          setTimeout(() => {
            navigate('/login');
          }, 3000);
        }
      } catch (err: any) {
        console.error('Failed to check setup status:', err);
        setError('Failed to check setup status. Please try again.');
      } finally {
        setCheckingStatus(false);
      }
    };

    checkSetupStatus();
  }, [navigate]);

  const validatePassword = (pwd: string): string | null => {
    if (pwd.length < 12) {
      return 'Password must be at least 12 characters long (NIST recommendation for admin accounts)';
    }
    if (pwd.length > 128) {
      return 'Password must be 128 characters or less';
    }
    // Check for common weak passwords
    const weakPasswords = [
      '123456789012',
      'password1234',
      'admin1234567',
      'changeme1234',
    ];
    if (weakPasswords.includes(pwd.toLowerCase())) {
      return 'Password is too common - please choose a stronger password';
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setSuccess('');

    // Validate password
    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    // Validate password confirmation
    if (password !== passwordConfirm) {
      setError('Passwords do not match');
      return;
    }

    // Validate email
    if (!email || !email.includes('@')) {
      setError('Please enter a valid email address');
      return;
    }

    setLoading(true);

    try {
      const response = await api.setupAdmin(password, passwordConfirm, email);
      setSuccess(response.message);

      // Redirect to login after 2 seconds
      setTimeout(() => {
        navigate('/login');
      }, 2000);
    } catch (err: any) {
      const errorMessage = err.response?.data?.error || err.message || 'Failed to configure admin account';
      const hint = err.response?.data?.hint;
      setError(hint ? `${errorMessage}\n${hint}` : errorMessage);
    } finally {
      setLoading(false);
    }
  };

  // Loading state while checking setup status
  if (checkingStatus) {
    return (
      <Container maxWidth="sm">
        <Box
          sx={{
            minHeight: '100vh',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: 'center',
          }}
        >
          <CircularProgress size={60} sx={{ mb: 2 }} />
          <Typography variant="h6" color="text.secondary">
            Checking setup status...
          </Typography>
        </Box>
      </Container>
    );
  }

  // Setup not required - show message and redirect
  if (!setupRequired) {
    return (
      <Container maxWidth="sm">
        <Box
          sx={{
            minHeight: '100vh',
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            alignItems: 'center',
          }}
        >
          <Paper
            elevation={3}
            sx={{
              p: 4,
              width: '100%',
              textAlign: 'center',
            }}
          >
            <InfoIcon color="primary" sx={{ fontSize: 60, mb: 2 }} />
            <Typography variant="h5" gutterBottom>
              Setup Not Required
            </Typography>
            <Typography variant="body1" color="text.secondary" paragraph>
              {statusMessage || 'Admin account is already configured.'}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Redirecting to login page...
            </Typography>
            <LinearProgress sx={{ mt: 2 }} />
          </Paper>
        </Box>
      </Container>
    );
  }

  // Setup required - show wizard
  return (
    <Container maxWidth="md">
      <Box
        sx={{
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          py: 4,
        }}
      >
        {/* Header */}
        <Box sx={{ textAlign: 'center', mb: 4 }}>
          <AdminIcon color="primary" sx={{ fontSize: 60, mb: 2 }} />
          <Typography variant="h3" gutterBottom>
            StreamSpace Setup Wizard
          </Typography>
          <Typography variant="h6" color="text.secondary">
            Configure Your Admin Account
          </Typography>
        </Box>

        {/* Setup Form */}
        <Paper elevation={3} sx={{ p: 4, mb: 3 }}>
          {error && (
            <Alert severity="error" sx={{ mb: 3 }} onClose={() => setError('')}>
              {error}
            </Alert>
          )}

          {success && (
            <Alert severity="success" sx={{ mb: 3 }}>
              {success}
            </Alert>
          )}

          <Typography variant="h5" gutterBottom>
            Initial Admin Configuration
          </Typography>

          <Typography variant="body2" color="text.secondary" paragraph>
            No admin password has been configured via Helm chart or environment variable.
            Please set up your admin credentials to continue.
          </Typography>

          <form onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="Username"
              value="admin"
              disabled
              margin="normal"
              helperText="Admin username is fixed"
              InputProps={{
                startAdornment: (
                  <Box sx={{ mr: 1, display: 'flex', alignItems: 'center' }}>
                    <AdminIcon color="action" />
                  </Box>
                ),
              }}
            />

            <TextField
              fullWidth
              label="Email Address"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              margin="normal"
              required
              helperText="Admin contact email for notifications"
            />

            <TextField
              fullWidth
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              margin="normal"
              required
              helperText="Minimum 12 characters (NIST recommendation for admin accounts)"
            />

            <TextField
              fullWidth
              label="Confirm Password"
              type="password"
              value={passwordConfirm}
              onChange={(e) => setPasswordConfirm(e.target.value)}
              margin="normal"
              required
              helperText="Re-enter password to confirm"
            />

            <Button
              type="submit"
              variant="contained"
              size="large"
              fullWidth
              disabled={loading || success !== ''}
              sx={{ mt: 3, py: 1.5 }}
            >
              {loading ? (
                <CircularProgress size={24} color="inherit" />
              ) : success ? (
                'Redirecting to Login...'
              ) : (
                'Configure Admin Account'
              )}
            </Button>
          </form>
        </Paper>

        {/* Information Panel */}
        <Paper elevation={3} sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>
            About Admin Onboarding
          </Typography>

          <Typography variant="body2" color="text.secondary" paragraph>
            StreamSpace uses a multi-layered approach for admin configuration:
          </Typography>

          <List dense>
            <ListItem>
              <ListItemIcon>
                <CheckIcon color="primary" />
              </ListItemIcon>
              <ListItemText
                primary="Priority 1: Helm Chart"
                secondary="Auto-generates secure password in Kubernetes Secret (production recommended)"
              />
            </ListItem>

            <ListItem>
              <ListItemIcon>
                <CheckIcon color="primary" />
              </ListItemIcon>
              <ListItemText
                primary="Priority 2: Environment Variable"
                secondary="Set ADMIN_PASSWORD for docker-compose or manual deployments"
              />
            </ListItem>

            <ListItem>
              <ListItemIcon>
                <CheckIcon color="success" />
              </ListItemIcon>
              <ListItemText
                primary="Priority 3: Setup Wizard (You Are Here)"
                secondary="Browser-based configuration for first-run or account recovery"
              />
            </ListItem>
          </List>

          <Alert severity="info" sx={{ mt: 2 }}>
            <Typography variant="body2">
              <strong>Note:</strong> This setup wizard automatically disables after you configure the
              admin password. For security, change the password via the web UI after first login.
            </Typography>
          </Alert>
        </Paper>

        {/* Footer */}
        <Box sx={{ textAlign: 'center', mt: 3 }}>
          <Typography variant="body2" color="text.secondary">
            StreamSpace v1.0.0 - Open Source Container Streaming Platform
          </Typography>
        </Box>
      </Box>
    </Container>
  );
}

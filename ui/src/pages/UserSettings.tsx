import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Card,
  CardContent,
  Grid,
  TextField,
  Button,
  Switch,
  FormControlLabel,
  Divider,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Stepper,
  Step,
  StepLabel,
  Paper,
  Chip,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  IconButton,
} from '@mui/material';
import {
  Settings as SettingsIcon,
  Security as SecurityIcon,
  Palette as PaletteIcon,
  Lock as LockIcon,
  VpnKey as KeyIcon,
  Delete as DeleteIcon,
  Check as CheckIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import { QRCodeSVG } from 'qrcode.react';
import Layout from '../components/Layout';
import QuotaCard from '../components/QuotaCard';
import { useUserStore } from '../store/userStore';
import { useMFAMethods } from '../hooks/useApi';
import { useQueryClient } from '@tanstack/react-query';
import api from '../lib/api';
import { toast } from '../lib/toast';

/**
 * UserSettings - User account settings and preferences
 *
 * Provides a central location for users to manage their account settings including:
 * - Resource quota overview
 * - Password change
 * - MFA (TOTP) setup and management
 * - UI theme preference (light/dark mode)
 *
 * @page
 * @route /settings
 * @access user - All authenticated users
 */
export default function UserSettings() {
  const { user } = useUserStore();
  const queryClient = useQueryClient();
  const { data: mfaMethods = [], isLoading: mfaLoading } = useMFAMethods();

  // Password change state
  const [passwordForm, setPasswordForm] = useState({
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  });
  const [passwordError, setPasswordError] = useState('');
  const [passwordSuccess, setPasswordSuccess] = useState(false);
  const [changingPassword, setChangingPassword] = useState(false);

  // Theme state
  const [darkMode, setDarkMode] = useState(() => {
    const stored = localStorage.getItem('theme');
    return stored ? stored === 'dark' : true; // Default to dark
  });

  // MFA setup state
  const [mfaDialogOpen, setMfaDialogOpen] = useState(false);
  const [mfaStep, setMfaStep] = useState(0);
  const [totpSecret, setTotpSecret] = useState('');
  const [totpUri, setTotpUri] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);
  const [settingUpMfa, setSettingUpMfa] = useState(false);

  // Check if TOTP is already enabled
  const totpMethod = mfaMethods.find((m: any) => m.type === 'totp');
  const isTotpEnabled = totpMethod?.enabled || false;

  // Handle password change
  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError('');
    setPasswordSuccess(false);

    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      setPasswordError('New passwords do not match');
      return;
    }

    if (passwordForm.newPassword.length < 8) {
      setPasswordError('Password must be at least 8 characters');
      return;
    }

    setChangingPassword(true);
    try {
      await api.changePassword({
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword,
      });
      setPasswordSuccess(true);
      setPasswordForm({ currentPassword: '', newPassword: '', confirmPassword: '' });
      toast.success('Password changed successfully');
    } catch (error: any) {
      setPasswordError(error.response?.data?.message || 'Failed to change password');
    } finally {
      setChangingPassword(false);
    }
  };

  // Handle theme toggle
  const handleThemeToggle = () => {
    const newTheme = !darkMode;
    setDarkMode(newTheme);
    localStorage.setItem('theme', newTheme ? 'dark' : 'light');
    // Note: Full theme switching would require updating the ThemeProvider in App.tsx
    // For now, this saves the preference
    toast.info(`Theme preference saved. Refresh to apply ${newTheme ? 'dark' : 'light'} mode.`);
  };

  // Start MFA setup
  const handleStartMfaSetup = async () => {
    setSettingUpMfa(true);
    try {
      const response = await api.setupMFA('totp');
      setTotpSecret(response.secret);
      setTotpUri(response.uri);
      setMfaDialogOpen(true);
      setMfaStep(0);
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Failed to start MFA setup');
    } finally {
      setSettingUpMfa(false);
    }
  };

  // Verify MFA code
  const handleVerifyMfa = async () => {
    if (verificationCode.length !== 6) {
      toast.error('Please enter a 6-digit code');
      return;
    }

    try {
      const response = await api.verifyMFA('totp', verificationCode);
      setBackupCodes(response.backupCodes || []);
      setMfaStep(2);
      queryClient.invalidateQueries({ queryKey: ['mfa-methods'] });
      toast.success('MFA enabled successfully');
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Invalid verification code');
    }
  };

  // Disable MFA
  const handleDisableMfa = async () => {
    if (!confirm('Are you sure you want to disable MFA? This will reduce your account security.')) {
      return;
    }

    try {
      await api.disableMFA('totp');
      queryClient.invalidateQueries({ queryKey: ['mfa-methods'] });
      toast.success('MFA disabled');
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Failed to disable MFA');
    }
  };

  // Close MFA dialog
  const handleCloseMfaDialog = () => {
    setMfaDialogOpen(false);
    setMfaStep(0);
    setTotpSecret('');
    setTotpUri('');
    setVerificationCode('');
    setBackupCodes([]);
  };

  return (
    <Layout>
      <Box>
        <Typography variant="h4" sx={{ fontWeight: 700, mb: 3 }}>
          Settings
        </Typography>

        <Grid container spacing={3}>
          {/* Resource Quota */}
          <Grid item xs={12} md={6}>
            <QuotaCard />
          </Grid>

          {/* Theme Settings */}
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center" mb={2}>
                  <PaletteIcon sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="h6">Appearance</Typography>
                </Box>
                <FormControlLabel
                  control={
                    <Switch
                      checked={darkMode}
                      onChange={handleThemeToggle}
                      color="primary"
                    />
                  }
                  label={darkMode ? 'Dark Mode' : 'Light Mode'}
                />
                <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                  Choose your preferred color scheme for the interface.
                </Typography>
              </CardContent>
            </Card>
          </Grid>

          {/* Password Change */}
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center" mb={2}>
                  <LockIcon sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="h6">Change Password</Typography>
                </Box>

                {passwordSuccess && (
                  <Alert severity="success" sx={{ mb: 2 }}>
                    Password changed successfully!
                  </Alert>
                )}

                {passwordError && (
                  <Alert severity="error" sx={{ mb: 2 }}>
                    {passwordError}
                  </Alert>
                )}

                <form onSubmit={handlePasswordChange}>
                  <TextField
                    fullWidth
                    type="password"
                    label="Current Password"
                    value={passwordForm.currentPassword}
                    onChange={(e) =>
                      setPasswordForm({ ...passwordForm, currentPassword: e.target.value })
                    }
                    margin="normal"
                    required
                  />
                  <TextField
                    fullWidth
                    type="password"
                    label="New Password"
                    value={passwordForm.newPassword}
                    onChange={(e) =>
                      setPasswordForm({ ...passwordForm, newPassword: e.target.value })
                    }
                    margin="normal"
                    required
                    helperText="Minimum 8 characters"
                  />
                  <TextField
                    fullWidth
                    type="password"
                    label="Confirm New Password"
                    value={passwordForm.confirmPassword}
                    onChange={(e) =>
                      setPasswordForm({ ...passwordForm, confirmPassword: e.target.value })
                    }
                    margin="normal"
                    required
                  />
                  <Button
                    type="submit"
                    variant="contained"
                    fullWidth
                    sx={{ mt: 2 }}
                    disabled={changingPassword}
                  >
                    {changingPassword ? 'Changing...' : 'Change Password'}
                  </Button>
                </form>
              </CardContent>
            </Card>
          </Grid>

          {/* MFA Settings */}
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Box display="flex" alignItems="center" mb={2}>
                  <SecurityIcon sx={{ mr: 1, color: 'primary.main' }} />
                  <Typography variant="h6">Two-Factor Authentication</Typography>
                </Box>

                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Add an extra layer of security to your account using an authenticator app.
                </Typography>

                {isTotpEnabled ? (
                  <Box>
                    <Alert severity="success" sx={{ mb: 2 }}>
                      <Box display="flex" alignItems="center">
                        <CheckIcon sx={{ mr: 1 }} />
                        MFA is enabled
                      </Box>
                    </Alert>
                    <Button
                      variant="outlined"
                      color="error"
                      onClick={handleDisableMfa}
                      startIcon={<DeleteIcon />}
                    >
                      Disable MFA
                    </Button>
                  </Box>
                ) : (
                  <Box>
                    <Alert severity="warning" sx={{ mb: 2 }}>
                      <Box display="flex" alignItems="center">
                        <WarningIcon sx={{ mr: 1 }} />
                        MFA is not enabled
                      </Box>
                    </Alert>
                    <Button
                      variant="contained"
                      onClick={handleStartMfaSetup}
                      disabled={settingUpMfa}
                      startIcon={<KeyIcon />}
                    >
                      {settingUpMfa ? 'Setting up...' : 'Enable MFA'}
                    </Button>
                  </Box>
                )}
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* MFA Setup Dialog */}
        <Dialog open={mfaDialogOpen} onClose={handleCloseMfaDialog} maxWidth="sm" fullWidth>
          <DialogTitle>Set Up Two-Factor Authentication</DialogTitle>
          <DialogContent>
            <Stepper activeStep={mfaStep} sx={{ mb: 3 }}>
              <Step>
                <StepLabel>Scan QR Code</StepLabel>
              </Step>
              <Step>
                <StepLabel>Verify Code</StepLabel>
              </Step>
              <Step>
                <StepLabel>Save Backup Codes</StepLabel>
              </Step>
            </Stepper>

            {mfaStep === 0 && (
              <Box textAlign="center">
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.)
                </Typography>
                {totpUri && (
                  <Box sx={{ display: 'flex', justifyContent: 'center', mb: 2 }}>
                    <Paper sx={{ p: 2, bgcolor: 'white' }}>
                      <QRCodeSVG value={totpUri} size={200} />
                    </Paper>
                  </Box>
                )}
                <Typography variant="caption" color="text.secondary">
                  Or enter this key manually: <strong>{totpSecret}</strong>
                </Typography>
                <Box sx={{ mt: 3 }}>
                  <Button variant="contained" onClick={() => setMfaStep(1)}>
                    Next
                  </Button>
                </Box>
              </Box>
            )}

            {mfaStep === 1 && (
              <Box textAlign="center">
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Enter the 6-digit code from your authenticator app
                </Typography>
                <TextField
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="000000"
                  inputProps={{
                    maxLength: 6,
                    style: { textAlign: 'center', fontSize: 24, letterSpacing: 8 },
                  }}
                  sx={{ mb: 2, width: 200 }}
                />
                <Box>
                  <Button variant="contained" onClick={handleVerifyMfa} disabled={verificationCode.length !== 6}>
                    Verify
                  </Button>
                </Box>
              </Box>
            )}

            {mfaStep === 2 && (
              <Box>
                <Alert severity="success" sx={{ mb: 2 }}>
                  MFA has been enabled successfully!
                </Alert>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  Save these backup codes in a safe place. You can use them to access your account if you lose your authenticator device.
                </Typography>
                <Paper sx={{ p: 2, bgcolor: 'background.default' }}>
                  <Grid container spacing={1}>
                    {backupCodes.map((code, index) => (
                      <Grid item xs={6} key={index}>
                        <Typography variant="body2" fontFamily="monospace">
                          {code}
                        </Typography>
                      </Grid>
                    ))}
                  </Grid>
                </Paper>
                <Typography variant="caption" color="error" sx={{ mt: 1, display: 'block' }}>
                  Warning: These codes will only be shown once!
                </Typography>
              </Box>
            )}
          </DialogContent>
          <DialogActions>
            {mfaStep === 2 ? (
              <Button onClick={handleCloseMfaDialog} variant="contained">
                Done
              </Button>
            ) : (
              <Button onClick={handleCloseMfaDialog}>Cancel</Button>
            )}
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

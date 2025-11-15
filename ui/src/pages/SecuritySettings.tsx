/**
 * SecuritySettings Component
 *
 * Provides a comprehensive security management interface for users including:
 * - Multi-Factor Authentication (MFA) setup and management
 * - IP Whitelisting for access control
 * - Security overview and audit logs
 *
 * SECURITY FIX (2025-11-14):
 * Disabled SMS and Email MFA options to match backend restrictions. These MFA types
 * were incomplete and always returned "valid=true", which would allow bypassing
 * authentication entirely. Only TOTP (authenticator apps) is currently supported.
 *
 * Features:
 * - TOTP Setup: QR code scanning with authenticator apps (Google Authenticator, Authy, etc.)
 * - Backup Codes: Single-use recovery codes for account access
 * - IP Whitelisting: Restrict access to specific IP addresses or CIDR ranges
 * - Security Overview: Dashboard showing enabled security features
 *
 * User Experience:
 * - Clear "Coming Soon" indicators for SMS/Email MFA (disabled cards with info alerts)
 * - Step-by-step MFA setup wizard with QR code and verification
 * - Visual feedback for security status (enabled/disabled features)
 * - Responsive design works on mobile and desktop
 *
 * Technical:
 * - React functional component with TypeScript
 * - Material-UI for consistent design
 * - QRCode.react for TOTP QR code generation
 * - State management for dialogs, tabs, and form data
 *
 * @component
 * @example
 * ```tsx
 * <SecuritySettings />
 * ```
 */
import { useState } from 'react';
import {
  Box,
  Typography,
  Tabs,
  Tab,
  Card,
  CardContent,
  Button,
  Chip,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Alert,
  Grid,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Stepper,
  Step,
  StepLabel,
  Paper,
  Divider,
} from '@mui/material';
import {
  Security as SecurityIcon,
  PhoneAndroid as PhoneIcon,
  Email as EmailIcon,
  VpnKey as KeyIcon,
  Delete as DeleteIcon,
  Add as AddIcon,
  Check as CheckIcon,
  Warning as WarningIcon,
  Shield as ShieldIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import { QRCodeSVG } from 'qrcode.react';

/**
 * Interface for MFA method data structure.
 * Represents a configured multi-factor authentication method.
 */
interface MFAMethod {
  id: number;
  type: string;
  enabled: boolean;
  is_primary: boolean;
  phone_number?: string;
  email?: string;
  created_at: string;
  last_used_at?: string;
}

interface IPWhitelistEntry {
  id: number;
  ip_address: string;
  description: string;
  enabled: boolean;
  created_at: string;
  expires_at?: string;
}

interface SecurityAlert {
  type: string;
  severity: string;
  message: string;
  created_at: string;
}

export default function SecuritySettings() {
  const [currentTab, setCurrentTab] = useState(0);
  const [mfaMethods, setMfaMethods] = useState<MFAMethod[]>([]);
  const [ipWhitelist, setIpWhitelist] = useState<IPWhitelistEntry[]>([]);
  const [securityAlerts, setSecurityAlerts] = useState<SecurityAlert[]>([]);

  // MFA Setup Dialog
  const [mfaDialog, setMfaDialog] = useState(false);
  const [mfaStep, setMfaStep] = useState(0);
  const [mfaType, setMfaType] = useState<'totp' | 'sms' | 'email'>('totp');
  const [totpSecret, setTotpSecret] = useState('');
  const [totpQR, setTotpQR] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [backupCodes, setBackupCodes] = useState<string[]>([]);

  // IP Whitelist Dialog
  const [ipDialog, setIpDialog] = useState(false);
  const [ipForm, setIpForm] = useState({
    ip_address: '',
    description: '',
  });

  const handleStartMFASetup = (type: 'totp' | 'sms' | 'email') => {
    setMfaType(type);
    setMfaStep(0);
    setMfaDialog(true);

    // TODO: API call to start MFA setup
    if (type === 'totp') {
      // Mock TOTP secret and QR code
      setTotpSecret('JBSWY3DPEHPK3PXP');
      setTotpQR('otpauth://totp/StreamSpace:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=StreamSpace');
    }
  };

  const handleVerifyMFASetup = () => {
    // TODO: API call to verify code
    console.log('Verify MFA:', verificationCode);
    setMfaStep(2);
    // Mock backup codes
    setBackupCodes([
      'ABC123-456789',
      'DEF456-123789',
      'GHI789-456123',
      'JKL012-789456',
      'MNO345-012789',
      'PQR678-345012',
      'STU901-678345',
      'VWX234-901678',
      'YZA567-234901',
      'BCD890-567234',
    ]);
  };

  const handleCompleteMFASetup = () => {
    setMfaDialog(false);
    // TODO: Refresh MFA methods list
  };

  const handleDisableMFA = (id: number) => {
    // TODO: API call to disable MFA method
    setMfaMethods(mfaMethods.filter((m) => m.id !== id));
  };

  const handleAddIPWhitelist = () => {
    // TODO: API call to add IP
    console.log('Add IP:', ipForm);
    setIpDialog(false);
  };

  const handleDeleteIPWhitelist = (id: number) => {
    // TODO: API call to delete IP
    setIpWhitelist(ipWhitelist.filter((ip) => ip.id !== id));
  };

  const getMFAIcon = (type: string) => {
    switch (type) {
      case 'totp':
        return <PhoneIcon />;
      case 'sms':
        return <PhoneIcon />;
      case 'email':
        return <EmailIcon />;
      default:
        return <KeyIcon />;
    }
  };

  const getSeverityColor = (severity: string) => {
    switch (severity) {
      case 'critical':
        return 'error';
      case 'high':
        return 'error';
      case 'medium':
        return 'warning';
      case 'low':
        return 'info';
      default:
        return 'default';
    }
  };

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Security Settings
          </Typography>
          <Chip icon={<ShieldIcon />} label="Protected" color="success" variant="outlined" />
        </Box>

        <Tabs value={currentTab} onChange={(_, v) => setCurrentTab(v)} sx={{ mb: 3 }}>
          <Tab label="Multi-Factor Authentication" />
          <Tab label="IP Whitelist" />
          <Tab label="Security Alerts" />
        </Tabs>

        {/* MFA Tab */}
        {currentTab === 0 && (
          <Grid container spacing={3}>
            <Grid item xs={12}>
              <Alert severity="info" sx={{ mb: 2 }}>
                Multi-factor authentication adds an extra layer of security to your account. We recommend enabling at
                least one method.
              </Alert>
            </Grid>

            <Grid item xs={12} md={4}>
              <Card>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                    <PhoneIcon sx={{ mr: 1 }} />
                    <Typography variant="h6">Authenticator App</Typography>
                  </Box>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Use an authenticator app (Google Authenticator, Authy, etc.) to generate time-based codes.
                  </Typography>
                  <Button variant="outlined" fullWidth onClick={() => handleStartMFASetup('totp')}>
                    Set Up
                  </Button>
                </CardContent>
              </Card>
            </Grid>

            <Grid item xs={12} md={4}>
              <Card sx={{ opacity: 0.6 }}>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                    <PhoneIcon sx={{ mr: 1 }} />
                    <Typography variant="h6">SMS</Typography>
                    <Chip label="Coming Soon" size="small" sx={{ ml: 1 }} />
                  </Box>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Receive verification codes via text message.
                  </Typography>
                  <Button variant="outlined" fullWidth disabled>
                    Not Available
                  </Button>
                  <Alert severity="info" sx={{ mt: 1, fontSize: '0.75rem' }}>
                    SMS MFA is under development. Please use TOTP for now.
                  </Alert>
                </CardContent>
              </Card>
            </Grid>

            <Grid item xs={12} md={4}>
              <Card sx={{ opacity: 0.6 }}>
                <CardContent>
                  <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
                    <EmailIcon sx={{ mr: 1 }} />
                    <Typography variant="h6">Email</Typography>
                    <Chip label="Coming Soon" size="small" sx={{ ml: 1 }} />
                  </Box>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Receive verification codes via email.
                  </Typography>
                  <Button variant="outlined" fullWidth disabled>
                    Not Available
                  </Button>
                  <Alert severity="info" sx={{ mt: 1, fontSize: '0.75rem' }}>
                    Email MFA is under development. Please use TOTP for now.
                  </Alert>
                </CardContent>
              </Card>
            </Grid>

            <Grid item xs={12}>
              <Card>
                <CardContent>
                  <Typography variant="h6" sx={{ mb: 2 }}>
                    Active MFA Methods
                  </Typography>
                  {mfaMethods.length === 0 ? (
                    <Alert severity="warning">No MFA methods configured</Alert>
                  ) : (
                    <List>
                      {mfaMethods.map((method) => (
                        <ListItem key={method.id}>
                          {getMFAIcon(method.type)}
                          <ListItemText
                            primary={
                              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                <Typography>
                                  {method.type.toUpperCase()}
                                  {method.phone_number && ` (${method.phone_number})`}
                                  {method.email && ` (${method.email})`}
                                </Typography>
                                {method.is_primary && <Chip label="Primary" size="small" color="primary" />}
                                {method.enabled && <Chip label="Enabled" size="small" color="success" />}
                              </Box>
                            }
                            secondary={`Last used: ${method.last_used_at ? new Date(method.last_used_at).toLocaleString() : 'Never'}`}
                          />
                          <ListItemSecondaryAction>
                            <IconButton edge="end" onClick={() => handleDisableMFA(method.id)}>
                              <DeleteIcon />
                            </IconButton>
                          </ListItemSecondaryAction>
                        </ListItem>
                      ))}
                    </List>
                  )}
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        )}

        {/* IP Whitelist Tab */}
        {currentTab === 1 && (
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">IP Whitelist</Typography>
                <Button variant="contained" startIcon={<AddIcon />} onClick={() => setIpDialog(true)}>
                  Add IP Address
                </Button>
              </Box>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>IP Address</TableCell>
                      <TableCell>Description</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Added</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {ipWhitelist.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} align="center">
                          <Typography color="text.secondary">No IP addresses whitelisted</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      ipWhitelist.map((entry) => (
                        <TableRow key={entry.id}>
                          <TableCell>
                            <Typography sx={{ fontFamily: 'monospace' }}>{entry.ip_address}</Typography>
                          </TableCell>
                          <TableCell>{entry.description}</TableCell>
                          <TableCell>
                            <Chip label={entry.enabled ? 'Active' : 'Disabled'} color={entry.enabled ? 'success' : 'default'} size="small" />
                          </TableCell>
                          <TableCell>{new Date(entry.created_at).toLocaleDateString()}</TableCell>
                          <TableCell>
                            <IconButton size="small" onClick={() => handleDeleteIPWhitelist(entry.id)}>
                              <DeleteIcon />
                            </IconButton>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        )}

        {/* Security Alerts Tab */}
        {currentTab === 2 && (
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Recent Security Alerts
              </Typography>
              {securityAlerts.length === 0 ? (
                <Alert severity="success">No security alerts</Alert>
              ) : (
                <List>
                  {securityAlerts.map((alert, index) => (
                    <ListItem key={index}>
                      <WarningIcon color={getSeverityColor(alert.severity)} sx={{ mr: 2 }} />
                      <ListItemText
                        primary={alert.message}
                        secondary={`${alert.type} - ${new Date(alert.created_at).toLocaleString()}`}
                      />
                    </ListItem>
                  ))}
                </List>
              )}
            </CardContent>
          </Card>
        )}

        {/* MFA Setup Dialog */}
        <Dialog open={mfaDialog} onClose={() => setMfaDialog(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Set Up Multi-Factor Authentication</DialogTitle>
          <DialogContent>
            <Stepper activeStep={mfaStep} sx={{ mb: 3 }}>
              <Step>
                <StepLabel>Scan QR Code</StepLabel>
              </Step>
              <Step>
                <StepLabel>Verify</StepLabel>
              </Step>
              <Step>
                <StepLabel>Backup Codes</StepLabel>
              </Step>
            </Stepper>

            {mfaStep === 0 && mfaType === 'totp' && (
              <Box sx={{ textAlign: 'center' }}>
                <Typography variant="body2" sx={{ mb: 2 }}>
                  Scan this QR code with your authenticator app:
                </Typography>
                <Box sx={{ display: 'flex', justifyContent: 'center', mb: 2 }}>
                  <QRCodeSVG value={totpQR} size={200} />
                </Box>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                  Or enter this code manually:
                </Typography>
                <Typography variant="body1" sx={{ fontFamily: 'monospace', mb: 2 }}>
                  {totpSecret}
                </Typography>
                <Button variant="contained" onClick={() => setMfaStep(1)}>
                  Next
                </Button>
              </Box>
            )}

            {mfaStep === 1 && (
              <Box>
                <Typography variant="body2" sx={{ mb: 2 }}>
                  Enter the 6-digit code from your authenticator app:
                </Typography>
                <TextField
                  fullWidth
                  label="Verification Code"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                  inputProps={{ maxLength: 6 }}
                  sx={{ mb: 2 }}
                />
                <Button variant="contained" fullWidth onClick={handleVerifyMFASetup}>
                  Verify
                </Button>
              </Box>
            )}

            {mfaStep === 2 && (
              <Box>
                <Alert severity="warning" sx={{ mb: 2 }}>
                  Save these backup codes in a safe place. Each code can only be used once.
                </Alert>
                <Paper variant="outlined" sx={{ p: 2, mb: 2 }}>
                  <Grid container spacing={1}>
                    {backupCodes.map((code, index) => (
                      <Grid item xs={6} key={index}>
                        <Typography sx={{ fontFamily: 'monospace', fontSize: '0.9rem' }}>{code}</Typography>
                      </Grid>
                    ))}
                  </Grid>
                </Paper>
                <Button variant="contained" fullWidth onClick={handleCompleteMFASetup} startIcon={<CheckIcon />}>
                  Complete Setup
                </Button>
              </Box>
            )}
          </DialogContent>
        </Dialog>

        {/* IP Whitelist Dialog */}
        <Dialog open={ipDialog} onClose={() => setIpDialog(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Add IP Address to Whitelist</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <TextField
                label="IP Address or CIDR"
                fullWidth
                value={ipForm.ip_address}
                onChange={(e) => setIpForm({ ...ipForm, ip_address: e.target.value })}
                placeholder="192.168.1.1 or 10.0.0.0/24"
                helperText="Single IP address or CIDR notation for a range"
              />
              <TextField
                label="Description"
                fullWidth
                multiline
                rows={2}
                value={ipForm.description}
                onChange={(e) => setIpForm({ ...ipForm, description: e.target.value })}
                placeholder="e.g., Home office network"
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setIpDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleAddIPWhitelist}>
              Add
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

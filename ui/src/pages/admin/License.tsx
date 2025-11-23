import { useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  Grid,
  TextField,
  Typography,
  Alert,
  CircularProgress,
  LinearProgress,
  Chip,
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  List,
  ListItem,
  ListItemText,
  Divider,
  ToggleButtonGroup,
  ToggleButton,
} from '@mui/material';
import {
  Refresh as RefreshIcon,
  Visibility as VisibilityIcon,
  VisibilityOff as VisibilityOffIcon,
  Check as CheckIcon,
  Close as CloseIcon,
  Warning as WarningIcon,
  Info as InfoIcon,
  TrendingUp as TrendingUpIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';

/**
 * License - License management dashboard for administrators
 *
 * Platform licensing and feature enforcement interface. Displays current
 * license information, usage statistics, and provides license activation
 * and renewal capabilities.
 *
 * Features:
 * - Current license display (tier, expiration, features)
 * - Usage dashboard (users, sessions, nodes vs. limits)
 * - License activation and validation
 * - Historical usage graphs (7/30/90 days)
 * - Limit warnings and expiration alerts
 * - License key masking/unmasking
 *
 * License Tiers:
 * - Community (Free): 10 users, 20 sessions, 3 nodes, basic auth
 * - Pro: 100 users, 200 sessions, 10 nodes, SAML/OIDC/MFA/recordings
 * - Enterprise: Unlimited users/sessions/nodes, all features + SLA
 *
 * @page
 * @route /admin/license - License management
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} License management dashboard
 */
export default function License() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  const [showLicenseKey, setShowLicenseKey] = useState(false);
  const [activateLicenseDialogOpen, setActivateLicenseDialogOpen] = useState(false);
  const [newLicenseKey, setNewLicenseKey] = useState('');
  const [validateDialogOpen, setValidateDialogOpen] = useState(false);
  const [validationResult, setValidationResult] = useState<any>(null);
  const [usageHistoryDays, setUsageHistoryDays] = useState<number>(30);

  // Fetch current license
  const { data: licenseData, isLoading: licenseLoading, error: licenseError, refetch: refetchLicense } = useQuery({
    queryKey: ['license'],
    queryFn: async () => {
      const response = await fetch('/api/v1/admin/license', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        // BUG FIX P0-2: Don't throw on 401, return null to show Community Edition
        if (response.status === 401 || response.status === 404) {
          return null;
        }
        throw new Error('Failed to fetch license');
      }

      return response.json();
    },
  });

  // Fetch usage history
  const { data: usageHistory, isLoading: historyLoading } = useQuery({
    queryKey: ['license-history', usageHistoryDays],
    queryFn: async () => {
      const response = await fetch(`/api/v1/admin/license/history?days=${usageHistoryDays}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch usage history');
      }

      return response.json();
    },
  });

  // Activate license mutation
  const activateMutation = useMutation({
    mutationFn: async (licenseKey: string) => {
      const response = await fetch('/api/v1/admin/license/activate', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ license_key: licenseKey }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to activate license');
      }

      return response.json();
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['license'] });
      queryClient.invalidateQueries({ queryKey: ['license-history'] });
      setActivateLicenseDialogOpen(false);
      setNewLicenseKey('');
      addNotification({
        message: 'License activated successfully',
        severity: 'success',
        priority: 'high',
        title: 'License Activated',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to activate license: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Activation Failed',
      });
    },
  });

  // Validate license mutation
  const validateMutation = useMutation({
    mutationFn: async (licenseKey: string) => {
      const response = await fetch('/api/v1/admin/license/validate', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ license_key: licenseKey }),
      });

      if (!response.ok) {
        throw new Error('Failed to validate license');
      }

      return response.json();
    },
    onSuccess: (data) => {
      setValidationResult(data);
      setValidateDialogOpen(true);
    },
    onError: (error: Error) => {
      addNotification({
        message: `Failed to validate license: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Validation Failed',
      });
    },
  });

  const handleActivateLicense = () => {
    if (newLicenseKey.trim().length < 10) {
      addNotification({
        message: 'Please enter a valid license key',
        severity: 'warning',
        priority: 'medium',
        title: 'Invalid License Key',
      });
      return;
    }
    activateMutation.mutate(newLicenseKey.trim());
  };

  const handleValidateLicense = () => {
    if (newLicenseKey.trim().length < 10) {
      addNotification({
        message: 'Please enter a valid license key',
        severity: 'warning',
        priority: 'medium',
        title: 'Invalid License Key',
      });
      return;
    }
    validateMutation.mutate(newLicenseKey.trim());
  };

  const maskLicenseKey = (key: string): string => {
    if (key.length <= 8) return '***';
    return `${key.substring(0, 4)}${'*'.repeat(key.length - 8)}${key.substring(key.length - 4)}`;
  };

  const getTierColor = (tier: string | null | undefined) => {
    // BUG FIX P0-2: Add null check before calling toLowerCase()
    if (!tier) return 'default';

    switch (tier.toLowerCase()) {
      case 'community':
        return 'default';
      case 'pro':
        return 'primary';
      case 'enterprise':
        return 'secondary';
      default:
        return 'default';
    }
  };

  const getUsageColor = (percentage: number | null | undefined) => {
    if (percentage === null || percentage === undefined) return 'success';
    if (percentage >= 100) return 'error';
    if (percentage >= 90) return 'error';
    if (percentage >= 80) return 'warning';
    return 'success';
  };

  const formatNumber = (num: number | null | undefined): string => {
    if (num === null || num === undefined) return 'Unlimited';
    return num.toString();
  };

  if (licenseLoading) {
    return (
      <AdminPortalLayout title="License Management">
        <Container maxWidth="lg">
          <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
            <CircularProgress />
          </Box>
        </Container>
      </AdminPortalLayout>
    );
  }

  // BUG FIX P0-2: Provide default values for Community Edition when no license data
  const license = licenseData?.license || {
    tier: 'Community',
    license_key: 'COMMUNITY-EDITION',
    issued_at: new Date().toISOString(),
    activated_at: null,
    expires_at: null,
    features: {
      basic_auth: true,
      saml_sso: false,
      oidc_sso: false,
      mfa: false,
      session_recording: false,
      audit_logs: false,
      rbac: false,
    },
  };
  const usage = licenseData?.usage || {
    current_users: 0,
    max_users: 10,
    user_percent: 0,
    current_sessions: 0,
    max_sessions: 20,
    session_percent: 0,
    current_nodes: 0,
    max_nodes: 3,
    node_percent: 0,
  };
  const warnings = licenseData?.limit_warnings || [];
  const isExpired = licenseData?.is_expired || false;
  const isExpiringSoon = licenseData?.is_expiring_soon || false;
  const daysUntilExpiry = licenseData?.days_until_expiry;

  return (
    <AdminPortalLayout title="License Management">
      <Container maxWidth="lg">
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              License Management
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage platform licensing, view usage statistics, and activate new licenses
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={() => {
                refetchLicense();
                queryClient.invalidateQueries({ queryKey: ['license-history'] });
              }}
            >
              Refresh
            </Button>
            <Button
              variant="contained"
              onClick={() => setActivateLicenseDialogOpen(true)}
            >
              Activate License
            </Button>
          </Box>
        </Box>

        {/* Community Edition info banner */}
        {!licenseData && (
          <Alert severity="info" sx={{ mb: 2 }}>
            You are running StreamSpace <strong>Community Edition</strong>. Activate a Pro or Enterprise license to unlock advanced features and remove limits.
          </Alert>
        )}

        {/* Expiration alerts */}
        {isExpired && (
          <Alert severity="error" sx={{ mb: 2 }}>
            Your license expired {Math.abs(daysUntilExpiry || 0)} day(s) ago. Please renew your license to continue using premium features.
          </Alert>
        )}
        {isExpiringSoon && !isExpired && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            Your license will expire in {daysUntilExpiry} day(s). Please renew soon to avoid service interruption.
          </Alert>
        )}

        {/* Limit warnings */}
        {warnings.length > 0 && (
          <Alert
            severity={warnings.some((w: any) => w.severity === 'exceeded') ? 'error' : 'warning'}
            sx={{ mb: 2 }}
          >
            <Typography variant="subtitle2" gutterBottom>
              License Limit Warnings:
            </Typography>
            {warnings.map((warning: any, index: number) => (
              <Typography key={index} variant="body2">
                • {warning.message}
              </Typography>
            ))}
          </Alert>
        )}

        <Grid container spacing={3}>
          {/* Current License Card */}
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                  <Typography variant="h6">Current License</Typography>
                  <Chip label={license?.tier || 'Unknown'} color={getTierColor(license?.tier)} />
                </Box>

                <List dense>
                  <ListItem>
                    <ListItemText
                      primary="License Key"
                      secondary={
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                            {showLicenseKey ? license.license_key : maskLicenseKey(license.license_key || '')}
                          </Typography>
                          {license.tier !== 'Community' && (
                            <IconButton
                              size="small"
                              onClick={() => setShowLicenseKey(!showLicenseKey)}
                            >
                              {showLicenseKey ? <VisibilityOffIcon fontSize="small" /> : <VisibilityIcon fontSize="small" />}
                            </IconButton>
                          )}
                        </Box>
                      }
                    />
                  </ListItem>
                  <ListItem>
                    <ListItemText
                      primary="Issued"
                      secondary={license.issued_at ? new Date(license.issued_at).toLocaleDateString() : 'N/A'}
                    />
                  </ListItem>
                  <ListItem>
                    <ListItemText
                      primary="Activated"
                      secondary={license.activated_at ? new Date(license.activated_at).toLocaleDateString() : 'N/A'}
                    />
                  </ListItem>
                  <ListItem>
                    <ListItemText
                      primary="Expires"
                      secondary={
                        license.expires_at ? (
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            {new Date(license.expires_at).toLocaleDateString()}
                            {!isExpired && daysUntilExpiry !== undefined && (
                              <Chip
                                label={`${daysUntilExpiry} days left`}
                                size="small"
                                color={isExpiringSoon ? 'warning' : 'success'}
                              />
                            )}
                          </Box>
                        ) : (
                          'Never'
                        )
                      }
                    />
                  </ListItem>
                </List>
              </CardContent>
            </Card>
          </Grid>

          {/* Features Card */}
          <Grid item xs={12} md={6}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Features
                </Typography>
                <Grid container spacing={1}>
                  {license?.features && Object.entries(license.features).map(([key, value]: [string, any]) => (
                    <Grid item xs={6} key={key}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                        {value ? (
                          <CheckIcon fontSize="small" color="success" />
                        ) : (
                          <CloseIcon fontSize="small" color="disabled" />
                        )}
                        <Typography variant="body2" color={value ? 'text.primary' : 'text.disabled'}>
                          {key.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())}
                        </Typography>
                      </Box>
                    </Grid>
                  ))}
                </Grid>
              </CardContent>
            </Card>
          </Grid>

          {/* Usage Statistics */}
          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  User Usage
                </Typography>
                <Box sx={{ mb: 2 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                    <Typography variant="body2">
                      {usage?.current_users || 0} / {formatNumber(usage?.max_users)}
                    </Typography>
                    {usage?.user_percent !== null && usage?.user_percent !== undefined && (
                      <Typography variant="body2">
                        {usage.user_percent.toFixed(1)}%
                      </Typography>
                    )}
                  </Box>
                  <LinearProgress
                    variant="determinate"
                    value={Math.min(usage?.user_percent || 0, 100)}
                    color={getUsageColor(usage?.user_percent)}
                  />
                </Box>
                {usage?.user_percent && usage.user_percent >= 80 && (
                  <Alert severity={usage.user_percent >= 100 ? 'error' : 'warning'} icon={<WarningIcon />}>
                    {usage.user_percent >= 100 ? 'User limit reached' : 'Approaching user limit'}
                  </Alert>
                )}
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Session Usage
                </Typography>
                <Box sx={{ mb: 2 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                    <Typography variant="body2">
                      {usage?.current_sessions || 0} / {formatNumber(usage?.max_sessions)}
                    </Typography>
                    {usage?.session_percent !== null && usage?.session_percent !== undefined && (
                      <Typography variant="body2">
                        {usage.session_percent.toFixed(1)}%
                      </Typography>
                    )}
                  </Box>
                  <LinearProgress
                    variant="determinate"
                    value={Math.min(usage?.session_percent || 0, 100)}
                    color={getUsageColor(usage?.session_percent)}
                  />
                </Box>
                {usage?.session_percent && usage.session_percent >= 80 && (
                  <Alert severity={usage.session_percent >= 100 ? 'error' : 'warning'} icon={<WarningIcon />}>
                    {usage.session_percent >= 100 ? 'Session limit reached' : 'Approaching session limit'}
                  </Alert>
                )}
              </CardContent>
            </Card>
          </Grid>

          <Grid item xs={12} md={4}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Node Usage
                </Typography>
                <Box sx={{ mb: 2 }}>
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 1 }}>
                    <Typography variant="body2">
                      {usage?.current_nodes || 0} / {formatNumber(usage?.max_nodes)}
                    </Typography>
                    {usage?.node_percent !== null && usage?.node_percent !== undefined && (
                      <Typography variant="body2">
                        {usage.node_percent.toFixed(1)}%
                      </Typography>
                    )}
                  </Box>
                  <LinearProgress
                    variant="determinate"
                    value={Math.min(usage?.node_percent || 0, 100)}
                    color={getUsageColor(usage?.node_percent)}
                  />
                </Box>
                {usage?.node_percent && usage.node_percent >= 80 && (
                  <Alert severity={usage.node_percent >= 100 ? 'error' : 'warning'} icon={<WarningIcon />}>
                    {usage.node_percent >= 100 ? 'Node limit reached' : 'Approaching node limit'}
                  </Alert>
                )}
              </CardContent>
            </Card>
          </Grid>

          {/* Usage History Graph */}
          <Grid item xs={12}>
            <Card>
              <CardContent>
                <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                  <Typography variant="h6">Usage History</Typography>
                  <ToggleButtonGroup
                    value={usageHistoryDays}
                    exclusive
                    onChange={(_, newValue) => {
                      if (newValue !== null) {
                        setUsageHistoryDays(newValue);
                      }
                    }}
                    size="small"
                  >
                    <ToggleButton value={7}>7 Days</ToggleButton>
                    <ToggleButton value={30}>30 Days</ToggleButton>
                    <ToggleButton value={90}>90 Days</ToggleButton>
                  </ToggleButtonGroup>
                </Box>

                {historyLoading ? (
                  <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
                    <CircularProgress />
                  </Box>
                ) : usageHistory && usageHistory.length > 0 ? (
                  <ResponsiveContainer width="100%" height={300}>
                    <LineChart data={[...usageHistory].reverse()}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="snapshot_date" />
                      <YAxis />
                      <Tooltip />
                      <Legend />
                      <Line type="monotone" dataKey="active_users" stroke="#8884d8" name="Users" />
                      <Line type="monotone" dataKey="active_sessions" stroke="#82ca9d" name="Sessions" />
                      <Line type="monotone" dataKey="active_nodes" stroke="#ffc658" name="Nodes" />
                    </LineChart>
                  </ResponsiveContainer>
                ) : (
                  <Alert severity="info" icon={<InfoIcon />}>
                    No usage history available yet. Usage data is collected daily.
                  </Alert>
                )}
              </CardContent>
            </Card>
          </Grid>

          {/* Upgrade Information */}
          <Grid item xs={12}>
            <Card>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  Upgrade Your License
                </Typography>
                <Typography variant="body2" color="text.secondary" paragraph>
                  Need more users, sessions, or features? Upgrade to Pro or Enterprise for expanded limits and premium capabilities.
                </Typography>
                <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
                  <Button variant="outlined" startIcon={<TrendingUpIcon />}>
                    Contact Sales
                  </Button>
                  <Button variant="text">
                    View Pricing
                  </Button>
                  <Button variant="text">
                    Compare Tiers
                  </Button>
                </Box>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Activate License Dialog */}
        <Dialog
          open={activateLicenseDialogOpen}
          onClose={() => setActivateLicenseDialogOpen(false)}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>Activate License</DialogTitle>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" paragraph>
              Enter your license key to activate a new license. This will replace the current active license.
            </Typography>
            <TextField
              fullWidth
              label="License Key"
              value={newLicenseKey}
              onChange={(e) => setNewLicenseKey(e.target.value)}
              placeholder="XXXX-XXXX-XXXX-XXXX"
              sx={{ mt: 2 }}
              multiline
              rows={3}
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setActivateLicenseDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleValidateLicense} disabled={validateMutation.isPending}>
              Validate
            </Button>
            <Button
              onClick={handleActivateLicense}
              variant="contained"
              disabled={activateMutation.isPending}
            >
              {activateMutation.isPending ? 'Activating...' : 'Activate'}
            </Button>
          </DialogActions>
        </Dialog>

        {/* Validation Result Dialog */}
        <Dialog
          open={validateDialogOpen}
          onClose={() => {
            setValidateDialogOpen(false);
            setValidationResult(null);
          }}
          maxWidth="sm"
          fullWidth
        >
          <DialogTitle>License Validation Result</DialogTitle>
          <DialogContent>
            {validationResult && (
              <>
                <Alert severity={validationResult.valid ? 'success' : 'error'} sx={{ mb: 2 }}>
                  {validationResult.message}
                </Alert>
                {validationResult.valid && (
                  <List dense>
                    <ListItem>
                      <ListItemText primary="Tier" secondary={validationResult.tier} />
                    </ListItem>
                    <ListItem>
                      <ListItemText
                        primary="Expires"
                        secondary={new Date(validationResult.expires_at).toLocaleDateString()}
                      />
                    </ListItem>
                    <Divider />
                    <ListItem>
                      <ListItemText
                        primary="Features"
                        secondary={
                          <Box sx={{ mt: 1 }}>
                            {validationResult.features && Object.entries(validationResult.features).map(([key, value]: [string, any]) => (
                              <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                                {value ? (
                                  <CheckIcon fontSize="small" color="success" />
                                ) : (
                                  <CloseIcon fontSize="small" color="disabled" />
                                )}
                                <Typography variant="body2">
                                  {key.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())}
                                </Typography>
                              </Box>
                            ))}
                          </Box>
                        }
                      />
                    </ListItem>
                  </List>
                )}
              </>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => {
              setValidateDialogOpen(false);
              setValidationResult(null);
            }}>
              Close
            </Button>
            {validationResult?.valid && (
              <Button
                onClick={() => {
                  setValidateDialogOpen(false);
                  handleActivateLicense();
                }}
                variant="contained"
              >
                Activate This License
              </Button>
            )}
          </DialogActions>
        </Dialog>
      </Container>
    </AdminPortalLayout>
  );
}

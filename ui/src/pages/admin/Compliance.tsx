import { useState, useEffect } from 'react';
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
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Grid,
  Alert,
  Paper,
  List,
  ListItem,
  ListItemText,
  Divider,
} from '@mui/material';
import {
  Gavel as ComplianceIcon,
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Assessment as ReportIcon,
  Warning as ViolationIcon,
  CheckCircle as CheckIcon,
  Error as ErrorIcon,
  Dashboard as DashboardIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import api from '../../lib/api';
import { toast } from '../../lib/toast';

interface ComplianceFramework {
  id: number;
  name: string;
  display_name: string;
  version: string;
  enabled: boolean;
  created_at: string;
}

interface CompliancePolicy {
  id: number;
  name: string;
  framework_name: string;
  enforcement_level: string;
  enabled: boolean;
  created_at: string;
}

interface ComplianceViolation {
  id: number;
  policy_name: string;
  user_id: string;
  violation_type: string;
  severity: string;
  description: string;
  status: string;
  created_at: string;
}

interface ComplianceMetrics {
  total_policies: number;
  active_policies: number;
  total_open_violations: number;
  violations_by_severity: {
    critical: number;
    high: number;
    medium: number;
    low: number;
  };
}

export default function Compliance() {
  const [currentTab, setCurrentTab] = useState(0);
  const [frameworks, setFrameworks] = useState<ComplianceFramework[]>([]);
  const [policies, setPolicies] = useState<CompliancePolicy[]>([]);
  const [violations, setViolations] = useState<ComplianceViolation[]>([]);
  const [metrics, setMetrics] = useState<ComplianceMetrics>({
    total_policies: 0,
    active_policies: 0,
    total_open_violations: 0,
    violations_by_severity: {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
    },
  });
  const [loading, setLoading] = useState(false);

  const [frameworkDialog, setFrameworkDialog] = useState(false);
  const [policyDialog, setPolicyDialog] = useState(false);
  const [reportDialog, setReportDialog] = useState(false);

  const [policyForm, setPolicyForm] = useState({
    name: '',
    framework_id: 0,
    enforcement_level: 'warning',
    applies_to: 'all_users',
    data_retention_days: 90,
  });

  const [reportForm, setReportForm] = useState({
    framework_id: 0,
    report_type: 'summary' as 'summary' | 'detailed' | 'attestation',
    start_date: '',
    end_date: '',
  });

  // Load initial data
  useEffect(() => {
    loadFrameworks();
    loadPolicies();
    loadViolations();
    loadDashboard();
  }, []);

  const loadFrameworks = async () => {
    try {
      const response = await api.listComplianceFrameworks();
      setFrameworks(response.frameworks);
    } catch (error) {
      console.error('Failed to load frameworks:', error);
    }
  };

  const loadPolicies = async () => {
    try {
      const response = await api.listCompliancePolicies();
      setPolicies(response.policies);
    } catch (error) {
      console.error('Failed to load policies:', error);
    }
  };

  const loadViolations = async () => {
    try {
      const response = await api.listComplianceViolations();
      setViolations(response.violations);
    } catch (error) {
      console.error('Failed to load violations:', error);
    }
  };

  const loadDashboard = async () => {
    try {
      const dashboard = await api.getComplianceDashboard();
      setMetrics({
        total_policies: dashboard.total_policies,
        active_policies: dashboard.active_policies,
        total_open_violations: dashboard.total_open_violations,
        violations_by_severity: dashboard.violations_by_severity,
      });
    } catch (error) {
      console.error('Failed to load dashboard:', error);
    }
  };

  const handleCreatePolicy = async () => {
    setLoading(true);
    try {
      await api.createCompliancePolicy({
        name: policyForm.name,
        framework_id: policyForm.framework_id,
        applies_to: {
          all_users: policyForm.applies_to === 'all_users',
          user_ids: policyForm.applies_to === 'specific_users' ? [] : undefined,
          roles: policyForm.applies_to === 'specific_roles' ? [] : undefined,
        },
        enforcement_level: policyForm.enforcement_level as any,
        data_retention: {
          session_data_days: policyForm.data_retention_days,
          recording_days: policyForm.data_retention_days,
          audit_log_days: policyForm.data_retention_days,
        },
      });
      toast.success('Compliance policy created');
      setPolicyDialog(false);
      loadPolicies();
      loadDashboard();
    } catch (error) {
      toast.error('Failed to create policy');
    } finally {
      setLoading(false);
    }
  };

  const handleGenerateReport = async () => {
    setLoading(true);
    try {
      const report = await api.generateComplianceReport({
        framework_id: reportForm.framework_id || undefined,
        report_type: reportForm.report_type,
        start_date: reportForm.start_date,
        end_date: reportForm.end_date,
      });
      toast.success('Compliance report generated');
      setReportDialog(false);
      // Note: Report data is available in the 'report' variable if you want to
      // download it as JSON or display it in a modal
    } catch (error) {
      toast.error('Failed to generate report');
    } finally {
      setLoading(false);
    }
  };

  const handleResolveViolation = async (id: number) => {
    setLoading(true);
    try {
      await api.resolveComplianceViolation(id, {
        resolution: 'Violation resolved by administrator',
        status: 'resolved',
      });
      toast.success('Violation resolved');
      loadViolations();
      loadDashboard();
    } catch (error) {
      toast.error('Failed to resolve violation');
    } finally {
      setLoading(false);
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

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'open':
        return 'error';
      case 'acknowledged':
        return 'warning';
      case 'remediated':
      case 'resolved':
      case 'closed':
        return 'success';
      default:
        return 'default';
    }
  };

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Compliance & Governance
          </Typography>
          <Button variant="contained" startIcon={<ReportIcon />} onClick={() => setReportDialog(true)}>
            Generate Report
          </Button>
        </Box>

        <Tabs value={currentTab} onChange={(_, v) => setCurrentTab(v)} sx={{ mb: 3 }}>
          <Tab label="Dashboard" />
          <Tab label="Frameworks" />
          <Tab label="Policies" />
          <Tab label="Violations" />
        </Tabs>

        {/* Dashboard Tab */}
        {currentTab === 0 && (
          <Box>
            <Grid container spacing={3}>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Total Policies</Typography>
                    <Typography variant="h3">{metrics.total_policies}</Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Active Policies</Typography>
                    <Typography variant="h3" color="success.main">
                      {metrics.active_policies}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Open Violations</Typography>
                    <Typography variant="h3" color="error.main">
                      {metrics.total_open_violations}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={3}>
                <Card>
                  <CardContent>
                    <Typography variant="h6">Critical Issues</Typography>
                    <Typography variant="h3" color="error.main">
                      {metrics.violations_by_severity.critical}
                    </Typography>
                  </CardContent>
                </Card>
              </Grid>

              <Grid item xs={12}>
                <Card>
                  <CardContent>
                    <Typography variant="h6" sx={{ mb: 2 }}>
                      Violations by Severity
                    </Typography>
                    <Grid container spacing={2}>
                      {Object.entries(metrics.violations_by_severity).map(([severity, count]) => (
                        <Grid item xs={6} md={3} key={severity}>
                          <Paper variant="outlined" sx={{ p: 2, textAlign: 'center' }}>
                            <Chip
                              label={severity.toUpperCase()}
                              color={getSeverityColor(severity)}
                              size="small"
                              sx={{ mb: 1 }}
                            />
                            <Typography variant="h4">{count}</Typography>
                          </Paper>
                        </Grid>
                      ))}
                    </Grid>
                  </CardContent>
                </Card>
              </Grid>

              <Grid item xs={12}>
                <Card>
                  <CardContent>
                    <Typography variant="h6" sx={{ mb: 2 }}>
                      Recent Violations
                    </Typography>
                    <List>
                      {violations.slice(0, 5).map((violation, index) => (
                        <Box key={violation.id}>
                          {index > 0 && <Divider />}
                          <ListItem>
                            <ListItemText
                              primary={
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                  <Chip label={violation.severity} color={getSeverityColor(violation.severity)} size="small" />
                                  <Typography>{violation.description}</Typography>
                                </Box>
                              }
                              secondary={`${violation.violation_type} - User: ${violation.user_id} - ${new Date(violation.created_at).toLocaleString()}`}
                            />
                            <Chip label={violation.status} color={getStatusColor(violation.status)} size="small" />
                          </ListItem>
                        </Box>
                      ))}
                    </List>
                  </CardContent>
                </Card>
              </Grid>
            </Grid>
          </Box>
        )}

        {/* Frameworks Tab */}
        {currentTab === 1 && (
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Compliance Frameworks</Typography>
                <Button variant="contained" startIcon={<AddIcon />} onClick={() => setFrameworkDialog(true)}>
                  New Framework
                </Button>
              </Box>
              <Grid container spacing={2}>
                {frameworks.map((framework) => (
                  <Grid item xs={12} md={6} key={framework.id}>
                    <Card variant="outlined">
                      <CardContent>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
                          <Typography variant="h6">{framework.display_name}</Typography>
                          <Chip
                            label={framework.enabled ? 'Enabled' : 'Disabled'}
                            color={framework.enabled ? 'success' : 'default'}
                            size="small"
                          />
                        </Box>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                          {framework.name} - Version {framework.version}
                        </Typography>
                        <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                          <Button size="small" variant="outlined">
                            View Controls
                          </Button>
                          <Button size="small" variant="outlined">
                            Configure
                          </Button>
                        </Box>
                      </CardContent>
                    </Card>
                  </Grid>
                ))}
              </Grid>
            </CardContent>
          </Card>
        )}

        {/* Policies Tab */}
        {currentTab === 2 && (
          <Card>
            <CardContent>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                <Typography variant="h6">Compliance Policies</Typography>
                <Button variant="contained" startIcon={<AddIcon />} onClick={() => setPolicyDialog(true)}>
                  New Policy
                </Button>
              </Box>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Framework</TableCell>
                      <TableCell>Enforcement Level</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {policies.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} align="center">
                          <Typography color="text.secondary">No policies configured</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      policies.map((policy) => (
                        <TableRow key={policy.id}>
                          <TableCell>{policy.name}</TableCell>
                          <TableCell>{policy.framework_name}</TableCell>
                          <TableCell>
                            <Chip label={policy.enforcement_level} size="small" />
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={policy.enabled ? 'Enabled' : 'Disabled'}
                              color={policy.enabled ? 'success' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <IconButton size="small">
                              <EditIcon />
                            </IconButton>
                            <IconButton size="small">
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

        {/* Violations Tab */}
        {currentTab === 3 && (
          <Card>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Policy Violations
              </Typography>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Severity</TableCell>
                      <TableCell>Policy</TableCell>
                      <TableCell>User</TableCell>
                      <TableCell>Violation Type</TableCell>
                      <TableCell>Description</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Time</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {violations.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={8} align="center">
                          <Typography color="text.secondary">No violations</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      violations.map((violation) => (
                        <TableRow key={violation.id}>
                          <TableCell>
                            <Chip label={violation.severity} color={getSeverityColor(violation.severity)} size="small" />
                          </TableCell>
                          <TableCell>{violation.policy_name}</TableCell>
                          <TableCell>{violation.user_id}</TableCell>
                          <TableCell>{violation.violation_type}</TableCell>
                          <TableCell>{violation.description}</TableCell>
                          <TableCell>
                            <Chip label={violation.status} color={getStatusColor(violation.status)} size="small" />
                          </TableCell>
                          <TableCell>{new Date(violation.created_at).toLocaleString()}</TableCell>
                          <TableCell>
                            {violation.status === 'open' && (
                              <Button size="small" onClick={() => handleResolveViolation(violation.id)}>
                                Resolve
                              </Button>
                            )}
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

        {/* Create Policy Dialog */}
        <Dialog open={policyDialog} onClose={() => setPolicyDialog(false)} maxWidth="md" fullWidth>
          <DialogTitle>Create Compliance Policy</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <TextField
                label="Policy Name"
                fullWidth
                value={policyForm.name}
                onChange={(e) => setPolicyForm({ ...policyForm, name: e.target.value })}
              />
              <FormControl fullWidth>
                <InputLabel>Framework</InputLabel>
                <Select
                  value={policyForm.framework_id}
                  onChange={(e) => setPolicyForm({ ...policyForm, framework_id: e.target.value as number })}
                >
                  {frameworks.map((f) => (
                    <MenuItem key={f.id} value={f.id}>
                      {f.display_name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth>
                <InputLabel>Enforcement Level</InputLabel>
                <Select
                  value={policyForm.enforcement_level}
                  onChange={(e) => setPolicyForm({ ...policyForm, enforcement_level: e.target.value })}
                >
                  <MenuItem value="advisory">Advisory (Log Only)</MenuItem>
                  <MenuItem value="warning">Warning (Alert)</MenuItem>
                  <MenuItem value="blocking">Blocking (Prevent)</MenuItem>
                </Select>
              </FormControl>
              <FormControl fullWidth>
                <InputLabel>Applies To</InputLabel>
                <Select
                  value={policyForm.applies_to}
                  onChange={(e) => setPolicyForm({ ...policyForm, applies_to: e.target.value })}
                >
                  <MenuItem value="all_users">All Users</MenuItem>
                  <MenuItem value="specific_users">Specific Users</MenuItem>
                  <MenuItem value="specific_roles">Specific Roles</MenuItem>
                </Select>
              </FormControl>
              <TextField
                label="Data Retention (days)"
                type="number"
                fullWidth
                value={policyForm.data_retention_days}
                onChange={(e) => setPolicyForm({ ...policyForm, data_retention_days: parseInt(e.target.value) })}
                helperText="How long to retain session data and audit logs"
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setPolicyDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleCreatePolicy}>
              Create
            </Button>
          </DialogActions>
        </Dialog>

        {/* Generate Report Dialog */}
        <Dialog open={reportDialog} onClose={() => setReportDialog(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Generate Compliance Report</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <FormControl fullWidth>
                <InputLabel>Framework</InputLabel>
                <Select
                  value={reportForm.framework_id}
                  onChange={(e) => setReportForm({ ...reportForm, framework_id: e.target.value as number })}
                >
                  <MenuItem value={0}>All Frameworks</MenuItem>
                  {frameworks.map((f) => (
                    <MenuItem key={f.id} value={f.id}>
                      {f.display_name}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth>
                <InputLabel>Report Type</InputLabel>
                <Select
                  value={reportForm.report_type}
                  onChange={(e) => setReportForm({ ...reportForm, report_type: e.target.value })}
                >
                  <MenuItem value="summary">Summary</MenuItem>
                  <MenuItem value="detailed">Detailed</MenuItem>
                  <MenuItem value="attestation">Attestation</MenuItem>
                </Select>
              </FormControl>
              <TextField
                label="Start Date"
                type="date"
                fullWidth
                value={reportForm.start_date}
                onChange={(e) => setReportForm({ ...reportForm, start_date: e.target.value })}
                InputLabelProps={{ shrink: true }}
              />
              <TextField
                label="End Date"
                type="date"
                fullWidth
                value={reportForm.end_date}
                onChange={(e) => setReportForm({ ...reportForm, end_date: e.target.value })}
                InputLabelProps={{ shrink: true }}
              />
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setReportDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleGenerateReport} startIcon={<ReportIcon />}>
              Generate
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

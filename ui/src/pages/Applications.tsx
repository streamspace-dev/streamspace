import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Button,
  Grid,
  Card,
  CardContent,
  CardActions,
  IconButton,
  Chip,
  Switch,
  CircularProgress,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  MenuItem,
  List,
  ListItem,
  ListItemText,
  ListItemSecondaryAction,
  Avatar,
  Divider,
  Tabs,
  Tab,
  FormControl,
  InputLabel,
  Select,
} from '@mui/material';
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  Group as GroupIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import AdminPortalLayout from '../components/AdminPortalLayout';
import {
  api,
  type InstalledApplication,
  type CatalogTemplate,
  type Group,
  type ApplicationGroupAccess,
  type InstallApplicationRequest,
} from '../lib/api';
import { useNotificationQueue } from '../components/NotificationQueue';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

interface TabPanelProps {
  children?: React.ReactNode;
  index: number;
  value: number;
}

function TabPanel(props: TabPanelProps) {
  const { children, value, index, ...other } = props;
  return (
    <div role="tabpanel" hidden={value !== index} {...other}>
      {value === index && <Box sx={{ py: 2 }}>{children}</Box>}
    </div>
  );
}

function ApplicationsContent() {
  const [loading, setLoading] = useState(true);
  const [applications, setApplications] = useState<InstalledApplication[]>([]);
  const [selectedApp, setSelectedApp] = useState<InstalledApplication | null>(null);
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [editDialogOpen, setEditDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [catalogTemplates, setCatalogTemplates] = useState<CatalogTemplate[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [editTab, setEditTab] = useState(0);

  // Add dialog state
  const [selectedTemplate, setSelectedTemplate] = useState<number | ''>('');
  const [newDisplayName, setNewDisplayName] = useState('');
  const [selectedGroups, setSelectedGroups] = useState<string[]>([]);
  const [selectedPlatform, setSelectedPlatform] = useState<string>('kubernetes');

  // Edit dialog state
  const [editDisplayName, setEditDisplayName] = useState('');
  const [editConfiguration, setEditConfiguration] = useState<Record<string, unknown>>({});
  const [appGroups, setAppGroups] = useState<ApplicationGroupAccess[]>([]);
  const [newGroupId, setNewGroupId] = useState('');
  const [newGroupAccessLevel, setNewGroupAccessLevel] = useState<'view' | 'launch' | 'admin'>('launch');

  const { addNotification } = useNotificationQueue();

  useEffect(() => {
    loadApplications();
    loadCatalogTemplates();
    loadGroups();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const loadApplications = async () => {
    setLoading(true);
    try {
      const data = await api.listApplications();
      setApplications(data.applications || []);
    } catch (error) {
      console.error('Failed to load applications:', error);
      addNotification({
        message: 'Failed to load applications',
        severity: 'error',
        priority: 'high',
      });
    } finally {
      setLoading(false);
    }
  };

  const loadCatalogTemplates = async () => {
    try {
      const data = await api.listCatalogTemplates({ limit: 100 });
      setCatalogTemplates(data.templates || []);
    } catch (error) {
      console.error('Failed to load catalog templates:', error);
    }
  };

  const loadGroups = async () => {
    try {
      const data = await api.listGroups();
      setGroups(data || []);
    } catch (error) {
      console.error('Failed to load groups:', error);
    }
  };

  const handleAddApplication = async () => {
    if (!selectedTemplate) return;

    try {
      const request: InstallApplicationRequest = {
        catalogTemplateId: selectedTemplate,
        displayName: newDisplayName || undefined,
        platform: selectedPlatform,
        groupIds: selectedGroups.length > 0 ? selectedGroups : undefined,
      };

      await api.installApplication(request);
      addNotification({
        message: 'Application installed successfully',
        severity: 'success',
        priority: 'medium',
      });
      setAddDialogOpen(false);
      resetAddDialog();
      await loadApplications();
    } catch (error) {
      console.error('Failed to install application:', error);
      addNotification({
        message: 'Failed to install application',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const handleEditApplication = async () => {
    if (!selectedApp) return;

    try {
      await api.updateApplication(selectedApp.id, {
        displayName: editDisplayName,
        configuration: editConfiguration,
      });
      addNotification({
        message: 'Application updated successfully',
        severity: 'success',
        priority: 'medium',
      });
      setEditDialogOpen(false);
      await loadApplications();
    } catch (error) {
      console.error('Failed to update application:', error);
      addNotification({
        message: 'Failed to update application',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const handleDeleteApplication = async () => {
    if (!selectedApp) return;

    try {
      await api.deleteApplication(selectedApp.id);
      // Optimistically remove from state immediately for instant UI feedback
      setApplications(prev => prev.filter(app => app.id !== selectedApp.id));
      addNotification({
        message: 'Application deleted successfully',
        severity: 'success',
        priority: 'medium',
      });
      setDeleteDialogOpen(false);
      setSelectedApp(null);
      // Still refresh to ensure consistency with server
      await loadApplications();
    } catch (error) {
      console.error('Failed to delete application:', error);
      addNotification({
        message: 'Failed to delete application',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const handleToggleEnabled = async (app: InstalledApplication) => {
    try {
      await api.setApplicationEnabled(app.id, !app.enabled);
      addNotification({
        message: `Application ${app.enabled ? 'disabled' : 'enabled'} successfully`,
        severity: 'success',
        priority: 'low',
      });
      await loadApplications();
    } catch (error) {
      console.error('Failed to toggle application:', error);
      addNotification({
        message: 'Failed to update application status',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const handleOpenEdit = async (app: InstalledApplication) => {
    setSelectedApp(app);
    setEditDisplayName(app.displayName);
    setEditConfiguration(app.configuration || {});
    setEditTab(0);

    // Load group access for this application
    try {
      const data = await api.getApplicationGroups(app.id);
      setAppGroups(data.groups || []);
    } catch (error) {
      console.error('Failed to load application groups:', error);
      setAppGroups([]);
    }

    setEditDialogOpen(true);
  };

  const handleAddGroupAccess = async () => {
    if (!selectedApp || !newGroupId) return;

    try {
      await api.addApplicationGroupAccess(selectedApp.id, {
        groupId: newGroupId,
        accessLevel: newGroupAccessLevel,
      });

      // Reload groups
      const data = await api.getApplicationGroups(selectedApp.id);
      setAppGroups(data.groups || []);
      setNewGroupId('');

      addNotification({
        message: 'Group access added successfully',
        severity: 'success',
        priority: 'low',
      });
    } catch (error) {
      console.error('Failed to add group access:', error);
      addNotification({
        message: 'Failed to add group access',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const handleRemoveGroupAccess = async (groupId: string) => {
    if (!selectedApp) return;

    try {
      await api.removeApplicationGroupAccess(selectedApp.id, groupId);

      // Reload groups
      const data = await api.getApplicationGroups(selectedApp.id);
      setAppGroups(data.groups || []);

      addNotification({
        message: 'Group access removed',
        severity: 'success',
        priority: 'low',
      });
    } catch (error) {
      console.error('Failed to remove group access:', error);
      addNotification({
        message: 'Failed to remove group access',
        severity: 'error',
        priority: 'high',
      });
    }
  };

  const resetAddDialog = () => {
    setSelectedTemplate('');
    setNewDisplayName('');
    setSelectedGroups([]);
    setSelectedPlatform('kubernetes');
  };

  const getSelectedTemplateName = () => {
    if (!selectedTemplate) return '';
    const template = catalogTemplates.find(t => t.id === selectedTemplate);
    return template?.displayName || '';
  };

  return (
    <AdminPortalLayout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Applications
          </Typography>
          <Box display="flex" gap={2}>
            <Button
              startIcon={<RefreshIcon />}
              onClick={loadApplications}
              disabled={loading}
            >
              Refresh
            </Button>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              onClick={() => setAddDialogOpen(true)}
            >
              Add Application
            </Button>
          </Box>
        </Box>

        {loading ? (
          <Box display="flex" justifyContent="center" py={8}>
            <CircularProgress />
          </Box>
        ) : applications.length === 0 ? (
          <Alert severity="info">
            No applications installed. Click "Add Application" to install your first application.
          </Alert>
        ) : (
          <Grid container spacing={3}>
            {applications.map((app) => (
              <Grid item xs={12} sm={6} md={4} key={app.id}>
                <Card
                  sx={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    opacity: app.enabled ? 1 : 0.7,
                  }}
                >
                  <CardContent sx={{ flexGrow: 1 }}>
                    <Box display="flex" alignItems="center" gap={2} mb={2}>
                      <Avatar
                        src={app.icon}
                        sx={{ width: 48, height: 48 }}
                      >
                        {app.displayName?.charAt(0) || 'A'}
                      </Avatar>
                      <Box flexGrow={1}>
                        <Typography variant="h6" noWrap>
                          {app.displayName}
                        </Typography>
                        <Typography variant="body2" color="text.secondary" noWrap>
                          {app.category}
                        </Typography>
                      </Box>
                      <Switch
                        checked={app.enabled}
                        onChange={() => handleToggleEnabled(app)}
                        size="small"
                      />
                    </Box>

                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        display: '-webkit-box',
                        WebkitLineClamp: 2,
                        WebkitBoxOrient: 'vertical',
                        mb: 2,
                      }}
                    >
                      {app.description || 'No description available'}
                    </Typography>

                    <Box display="flex" gap={1} flexWrap="wrap">
                      {app.enabled ? (
                        <Chip label="Enabled" color="success" size="small" />
                      ) : (
                        <Chip label="Disabled" color="default" size="small" />
                      )}
                      {app.groups && app.groups.length > 0 && (
                        <Chip
                          icon={<GroupIcon />}
                          label={`${app.groups.length} group${app.groups.length !== 1 ? 's' : ''}`}
                          size="small"
                          variant="outlined"
                        />
                      )}
                    </Box>
                  </CardContent>

                  <CardActions sx={{ justifyContent: 'flex-end', pt: 0 }}>
                    <IconButton
                      size="small"
                      onClick={() => handleOpenEdit(app)}
                      title="Edit"
                    >
                      <EditIcon />
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={() => {
                        setSelectedApp(app);
                        setDeleteDialogOpen(true);
                      }}
                      title="Delete"
                      color="error"
                    >
                      <DeleteIcon />
                    </IconButton>
                  </CardActions>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        {/* Add Application Dialog */}
        <Dialog open={addDialogOpen} onClose={() => setAddDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Add Application</DialogTitle>
          <DialogContent>
            <Box display="flex" flexDirection="column" gap={2} mt={1}>
              <FormControl fullWidth>
                <InputLabel>Select Application</InputLabel>
                <Select
                  value={selectedTemplate}
                  onChange={(e) => setSelectedTemplate(e.target.value as number)}
                  label="Select Application"
                >
                  {[...catalogTemplates]
                    .sort((a, b) => (a.displayName || '').localeCompare(b.displayName || ''))
                    .map((template) => (
                    <MenuItem key={template.id} value={template.id}>
                      {template.displayName} ({template.category})
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <TextField
                label="Display Name (optional)"
                value={newDisplayName}
                onChange={(e) => setNewDisplayName(e.target.value)}
                placeholder={getSelectedTemplateName() || 'Custom display name'}
                helperText="Name shown on user dashboard. Leave blank to use default."
              />

              <FormControl fullWidth>
                <InputLabel>Grant Access to Groups</InputLabel>
                <Select
                  multiple
                  value={selectedGroups}
                  onChange={(e) => setSelectedGroups(e.target.value as string[])}
                  label="Grant Access to Groups"
                  renderValue={(selected) => (
                    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                      {selected.map((id) => {
                        const group = groups.find(g => g.id === id);
                        return <Chip key={id} label={group?.displayName || id} size="small" />;
                      })}
                    </Box>
                  )}
                >
                  {groups.map((group) => (
                    <MenuItem key={group.id} value={group.id}>
                      {group.displayName}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl fullWidth>
                <InputLabel>Target Platform</InputLabel>
                <Select
                  value={selectedPlatform}
                  onChange={(e) => setSelectedPlatform(e.target.value)}
                  label="Target Platform"
                >
                  <MenuItem value="kubernetes">Kubernetes</MenuItem>
                  <MenuItem value="docker">Docker</MenuItem>
                  <MenuItem value="hyperv">Hyper-V</MenuItem>
                  <MenuItem value="vcenter">vCenter</MenuItem>
                </Select>
              </FormControl>
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setAddDialogOpen(false)}>Cancel</Button>
            <Button
              variant="contained"
              onClick={handleAddApplication}
              disabled={!selectedTemplate}
            >
              Install Application
            </Button>
          </DialogActions>
        </Dialog>

        {/* Edit Application Dialog */}
        <Dialog open={editDialogOpen} onClose={() => setEditDialogOpen(false)} maxWidth="md" fullWidth>
          <DialogTitle>
            Edit Application: {selectedApp?.displayName}
          </DialogTitle>
          <DialogContent>
            <Tabs value={editTab} onChange={(_, v) => setEditTab(v)} sx={{ borderBottom: 1, borderColor: 'divider' }}>
              <Tab label="General" />
              <Tab label="Group Access" />
              <Tab label="Configuration" />
            </Tabs>

            <TabPanel value={editTab} index={0}>
              <Box display="flex" flexDirection="column" gap={2}>
                <TextField
                  label="Display Name"
                  value={editDisplayName}
                  onChange={(e) => setEditDisplayName(e.target.value)}
                  fullWidth
                  helperText="Name shown on user dashboard"
                />
                <Typography variant="body2" color="text.secondary">
                  Template: {selectedApp?.templateDisplayName || selectedApp?.templateName}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Folder Path: {selectedApp?.folderPath}
                </Typography>
              </Box>
            </TabPanel>

            <TabPanel value={editTab} index={1}>
              <Box mb={2}>
                <Typography variant="subtitle2" gutterBottom>
                  Groups with Access
                </Typography>
                {appGroups.length === 0 ? (
                  <Alert severity="info" sx={{ mb: 2 }}>
                    No groups have access to this application
                  </Alert>
                ) : (
                  <List dense>
                    {appGroups.map((access) => (
                      <ListItem key={access.groupId}>
                        <ListItemText
                          primary={access.groupDisplayName || access.groupName}
                          secondary={`Access Level: ${access.accessLevel}`}
                        />
                        <ListItemSecondaryAction>
                          <IconButton
                            edge="end"
                            size="small"
                            onClick={() => handleRemoveGroupAccess(access.groupId)}
                          >
                            <DeleteIcon />
                          </IconButton>
                        </ListItemSecondaryAction>
                      </ListItem>
                    ))}
                  </List>
                )}
              </Box>

              <Divider sx={{ my: 2 }} />

              <Typography variant="subtitle2" gutterBottom>
                Add Group Access
              </Typography>
              <Box display="flex" gap={2} alignItems="flex-start">
                <FormControl sx={{ minWidth: 200 }}>
                  <InputLabel>Group</InputLabel>
                  <Select
                    value={newGroupId}
                    onChange={(e) => setNewGroupId(e.target.value)}
                    label="Group"
                    size="small"
                  >
                    {groups
                      .filter(g => !appGroups.some(ag => ag.groupId === g.id))
                      .map((group) => (
                        <MenuItem key={group.id} value={group.id}>
                          {group.displayName}
                        </MenuItem>
                      ))}
                  </Select>
                </FormControl>
                <FormControl sx={{ minWidth: 120 }}>
                  <InputLabel>Access Level</InputLabel>
                  <Select
                    value={newGroupAccessLevel}
                    onChange={(e) => setNewGroupAccessLevel(e.target.value as 'view' | 'launch' | 'admin')}
                    label="Access Level"
                    size="small"
                  >
                    <MenuItem value="view">View</MenuItem>
                    <MenuItem value="launch">Launch</MenuItem>
                    <MenuItem value="admin">Admin</MenuItem>
                  </Select>
                </FormControl>
                <Button
                  variant="outlined"
                  onClick={handleAddGroupAccess}
                  disabled={!newGroupId}
                >
                  Add
                </Button>
              </Box>
            </TabPanel>

            <TabPanel value={editTab} index={2}>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                Application-specific configuration. Edit the JSON below to customize settings.
              </Typography>
              <TextField
                multiline
                rows={10}
                fullWidth
                value={JSON.stringify(editConfiguration, null, 2)}
                onChange={(e) => {
                  try {
                    setEditConfiguration(JSON.parse(e.target.value));
                  } catch {
                    // Invalid JSON, ignore
                  }
                }}
                sx={{ fontFamily: 'monospace' }}
              />
            </TabPanel>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setEditDialogOpen(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleEditApplication}>
              Save Changes
            </Button>
          </DialogActions>
        </Dialog>

        {/* Delete Confirmation Dialog */}
        <Dialog open={deleteDialogOpen} onClose={() => setDeleteDialogOpen(false)}>
          <DialogTitle>Delete Application</DialogTitle>
          <DialogContent>
            <Typography>
              Are you sure you want to delete "{selectedApp?.displayName}"? This action cannot be undone.
            </Typography>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDeleteDialogOpen(false)}>Cancel</Button>
            <Button variant="contained" color="error" onClick={handleDeleteApplication}>
              Delete
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </AdminPortalLayout>
  );
}

/**
 * Applications - Installed applications management page
 *
 * This page allows administrators to:
 * - View all installed applications
 * - Install new applications from the catalog
 * - Enable/disable applications
 * - Configure application settings
 * - Manage group access to applications
 * - Edit display names for user dashboards
 *
 * @page
 * @route /admin/applications
 * @access admin - Only administrators can access this page
 */
export default function Applications() {
  return (
    <WebSocketErrorBoundary>
      <ApplicationsContent />
    </WebSocketErrorBoundary>
  );
}

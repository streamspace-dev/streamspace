import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  IconButton,
  Switch,
  Tooltip,
  Skeleton,
  Alert,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Grid,
  Card,
  CardContent,
} from '@mui/material';
import {
  Delete as DeleteIcon,
  Settings as SettingsIcon,
  Info as InfoIcon,
  Extension as ExtensionIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import Layout from '../../components/Layout';
import { api, type InstalledPlugin } from '../../lib/api';
import { toast } from '../../lib/toast';
import { usePluginEvents } from '../../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../../components/NotificationQueue';
import EnhancedWebSocketStatus from '../../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../../components/WebSocketErrorBoundary';

/**
 * AdminPlugins - System-wide plugin management for administrators
 *
 * Administrative interface for managing platform plugins that extend functionality across
 * the entire system. Administrators can install, configure, enable/disable, and uninstall
 * plugins that affect all users. Provides real-time WebSocket updates for plugin lifecycle
 * events and comprehensive statistics on plugin usage.
 *
 * Features:
 * - View all installed plugins system-wide
 * - Enable/disable plugins globally
 * - Configure plugin settings with JSON editor
 * - View plugin details (type, version, installer)
 * - Uninstall plugins with confirmation
 * - Real-time plugin event notifications via WebSocket
 * - Plugin statistics dashboard
 * - Filter plugins by type and status
 *
 * Administrative capabilities:
 * - System-wide plugin installation management
 * - Global enable/disable controls
 * - Plugin configuration editing (JSON)
 * - Plugin lifecycle monitoring
 * - Usage statistics and metrics
 * - Plugin dependency management
 *
 * Plugin types:
 * - extension: Platform extensions
 * - webhook: Webhook integrations
 * - api: API extensions
 * - ui: UI components and themes
 * - theme: Appearance themes
 *
 * Real-time features:
 * - Live plugin installation notifications
 * - Enable/disable event alerts
 * - Update notifications
 * - Error alerts for plugin failures
 * - WebSocket connection monitoring
 *
 * User workflows:
 * - Review installed plugins across the platform
 * - Enable/disable plugins for all users
 * - Configure global plugin settings
 * - Monitor plugin performance and errors
 * - Uninstall problematic plugins
 *
 * @page
 * @route /admin/plugins - System-wide plugin management
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Plugin management interface with system-wide controls
 *
 * @example
 * // Route configuration:
 * <Route path="/admin/plugins" element={<AdminPlugins />} />
 *
 * @see PluginCatalog for installing new plugins
 * @see InstalledPlugins for user-level plugin management
 */
const pluginTypeColors: Record<string, string> = {
  extension: '#4CAF50',
  webhook: '#FF9800',
  api: '#2196F3',
  ui: '#9C27B0',
  theme: '#E91E63',
};

export default function AdminPlugins() {
  const [loading, setLoading] = useState(true);
  const [plugins, setPlugins] = useState<InstalledPlugin[]>([]);
  const [selectedPlugin, setSelectedPlugin] = useState<InstalledPlugin | null>(null);
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [configDialogOpen, setConfigDialogOpen] = useState(false);
  const [configJson, setConfigJson] = useState('');

  // WebSocket connection state
  const [wsConnected, setWsConnected] = useState(false);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time plugin events via WebSocket with notifications
  usePluginEvents((data: any) => {
    console.log('Plugin event:', data);
    setWsConnected(true);

    // Show notifications for plugin events
    if (data.event_type === 'plugin.installed') {
      addNotification({
        message: `Plugin installed: ${data.plugin_name} v${data.version}`,
        severity: 'success',
        priority: 'medium',
        title: 'Plugin Installed',
      });
      // Refresh plugin list
      loadPlugins();
    } else if (data.event_type === 'plugin.uninstalled') {
      addNotification({
        message: `Plugin uninstalled: ${data.plugin_name}`,
        severity: 'warning',
        priority: 'medium',
        title: 'Plugin Uninstalled',
      });
      // Refresh plugin list
      loadPlugins();
    } else if (data.event_type === 'plugin.enabled') {
      addNotification({
        message: `Plugin enabled: ${data.plugin_name}`,
        severity: 'success',
        priority: 'low',
        title: 'Plugin Enabled',
      });
      // Refresh plugin list
      loadPlugins();
    } else if (data.event_type === 'plugin.disabled') {
      addNotification({
        message: `Plugin disabled: ${data.plugin_name}`,
        severity: 'info',
        priority: 'low',
        title: 'Plugin Disabled',
      });
      // Refresh plugin list
      loadPlugins();
    } else if (data.event_type === 'plugin.updated') {
      addNotification({
        message: `Plugin updated: ${data.plugin_name} to v${data.new_version}`,
        severity: 'info',
        priority: 'medium',
        title: 'Plugin Updated',
      });
      // Refresh plugin list
      loadPlugins();
    } else if (data.event_type === 'plugin.error') {
      addNotification({
        message: `Plugin error: ${data.plugin_name} - ${data.error_message}`,
        severity: 'error',
        priority: 'high',
        title: 'Plugin Error',
      });
    }
  });

  useEffect(() => {
    loadPlugins();
  }, []);

  const loadPlugins = async () => {
    setLoading(true);
    try {
      const data = await api.listInstalledPlugins();
      // Ensure plugins is always an array to prevent undefined errors
      setPlugins(Array.isArray(data) ? data : []);
    } catch (error) {
      console.error('Failed to load plugins:', error);
      toast.error('Failed to load plugins');
      // Set empty array on error to prevent undefined
      setPlugins([]);
    } finally {
      setLoading(false);
    }
  };

  const handleTogglePlugin = async (plugin: InstalledPlugin) => {
    try {
      if (plugin.enabled) {
        await api.disablePlugin(plugin.id);
        toast.success(`${plugin.displayName || plugin.name} disabled globally`);
      } else {
        await api.enablePlugin(plugin.id);
        toast.success(`${plugin.displayName || plugin.name} enabled globally`);
      }
      await loadPlugins();
    } catch (error) {
      console.error('Failed to toggle plugin:', error);
      toast.error('Failed to toggle plugin');
    }
  };

  const handleViewDetails = (plugin: InstalledPlugin) => {
    setSelectedPlugin(plugin);
    setDetailDialogOpen(true);
  };

  const handleOpenConfig = (plugin: InstalledPlugin) => {
    setSelectedPlugin(plugin);
    setConfigJson(JSON.stringify(plugin.config || {}, null, 2));
    setConfigDialogOpen(true);
  };

  const handleSaveConfig = async () => {
    if (!selectedPlugin) return;

    try {
      const config = JSON.parse(configJson);
      await api.updatePluginConfig(selectedPlugin.id, config);
      toast.success('Configuration updated');
      setConfigDialogOpen(false);
      await loadPlugins();
    } catch (error) {
      console.error('Failed to update configuration:', error);
      toast.error('Invalid JSON or failed to update configuration');
    }
  };

  const handleUninstall = async (plugin: InstalledPlugin) => {
    if (!confirm(`Are you sure you want to uninstall ${plugin.displayName || plugin.name}? This will affect all users.`)) {
      return;
    }

    try {
      await api.uninstallPlugin(plugin.id);
      toast.success(`${plugin.displayName || plugin.name} uninstalled`);
      await loadPlugins();
    } catch (error) {
      console.error('Failed to uninstall plugin:', error);
      toast.error('Failed to uninstall plugin');
    }
  };

  const stats = {
    total: plugins?.length ?? 0,
    enabled: plugins?.filter(p => p.enabled).length ?? 0,
    disabled: plugins?.filter(p => !p.enabled).length ?? 0,
    byType: {
      extension: plugins?.filter(p => p.pluginType === 'extension').length ?? 0,
      webhook: plugins?.filter(p => p.pluginType === 'webhook').length ?? 0,
      api: plugins?.filter(p => p.pluginType === 'api').length ?? 0,
      ui: plugins?.filter(p => p.pluginType === 'ui').length ?? 0,
      theme: plugins?.filter(p => p.pluginType === 'theme').length ?? 0,
    },
  };

  return (
    <WebSocketErrorBoundary>
      <Layout>
        <Box>
          <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
            <Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                <Typography variant="h4" sx={{ fontWeight: 700 }}>
                  Plugin Management
                </Typography>

                {/* Enhanced WebSocket Connection Status */}
                <EnhancedWebSocketStatus
                  isConnected={wsConnected}
                  reconnectAttempts={0}
                  size="small"
                  showDetails={true}
                />
              </Box>
              <Typography variant="body2" color="text.secondary">
                Manage system-wide plugin installations
              </Typography>
            </Box>
            <Button
              startIcon={<RefreshIcon />}
              onClick={loadPlugins}
              disabled={loading}
            >
              Refresh
            </Button>
          </Box>

        {/* Stats Cards */}
        <Grid container spacing={2} mb={3}>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary">
                  Total Plugins
                </Typography>
                <Typography variant="h4">{stats.total}</Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary">
                  Enabled
                </Typography>
                <Typography variant="h4" color="success.main">
                  {stats.enabled}
                </Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary">
                  Disabled
                </Typography>
                <Typography variant="h4" color="text.secondary">
                  {stats.disabled}
                </Typography>
              </CardContent>
            </Card>
          </Grid>
          <Grid item xs={12} sm={6} md={3}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary">
                  Extensions
                </Typography>
                <Typography variant="h4">{stats.byType.extension}</Typography>
              </CardContent>
            </Card>
          </Grid>
        </Grid>

        {/* Plugin Table */}
        {loading ? (
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Plugin</TableCell>
                  <TableCell>Type</TableCell>
                  <TableCell>Version</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Installed By</TableCell>
                  <TableCell>Installed At</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {Array.from({ length: 5 }).map((_, index) => (
                  <TableRow key={index}>
                    <TableCell><Skeleton width={200} /></TableCell>
                    <TableCell><Skeleton width={80} /></TableCell>
                    <TableCell><Skeleton width={60} /></TableCell>
                    <TableCell><Skeleton width={80} /></TableCell>
                    <TableCell><Skeleton width={100} /></TableCell>
                    <TableCell><Skeleton width={100} /></TableCell>
                    <TableCell align="right"><Skeleton width={150} /></TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        ) : plugins.length === 0 ? (
          <Alert severity="info">No plugins installed in the system.</Alert>
        ) : (
          <TableContainer component={Paper}>
            <Table>
              <TableHead>
                <TableRow>
                  <TableCell>Plugin</TableCell>
                  <TableCell>Type</TableCell>
                  <TableCell>Version</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Installed By</TableCell>
                  <TableCell>Installed At</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {plugins.map((plugin) => (
                  <TableRow key={plugin.id} hover>
                    <TableCell>
                      <Box display="flex" alignItems="center" gap={1}>
                        <ExtensionIcon fontSize="small" />
                        <Box>
                          <Typography variant="body2" fontWeight={600}>
                            {plugin.displayName || plugin.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {plugin.name}
                          </Typography>
                        </Box>
                      </Box>
                    </TableCell>
                    <TableCell>
                      {plugin.pluginType && (
                        <Chip
                          label={plugin.pluginType}
                          size="small"
                          sx={{
                            bgcolor: pluginTypeColors[plugin.pluginType] + '20',
                            color: pluginTypeColors[plugin.pluginType],
                          }}
                        />
                      )}
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" fontFamily="monospace">
                        {plugin.version}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={plugin.enabled ? 'Enabled' : 'Disabled'}
                        size="small"
                        color={plugin.enabled ? 'success' : 'default'}
                      />
                    </TableCell>
                    <TableCell>{plugin.installedBy}</TableCell>
                    <TableCell>
                      {new Date(plugin.installedAt).toLocaleDateString()}
                    </TableCell>
                    <TableCell align="right">
                      <Box display="flex" gap={1} justifyContent="flex-end">
                        <Switch
                          checked={plugin.enabled}
                          onChange={() => handleTogglePlugin(plugin)}
                          size="small"
                        />
                        <Tooltip title="Configure">
                          <IconButton
                            size="small"
                            onClick={() => handleOpenConfig(plugin)}
                          >
                            <SettingsIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Details">
                          <IconButton
                            size="small"
                            onClick={() => handleViewDetails(plugin)}
                          >
                            <InfoIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                        <Tooltip title="Uninstall">
                          <IconButton
                            size="small"
                            color="error"
                            onClick={() => handleUninstall(plugin)}
                          >
                            <DeleteIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </Box>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}

        {/* Plugin Detail Dialog */}
        <Dialog open={detailDialogOpen} onClose={() => setDetailDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Plugin Details</DialogTitle>
          <DialogContent>
            {selectedPlugin && (
              <Box>
                <Typography variant="h6" gutterBottom>
                  {selectedPlugin.displayName || selectedPlugin.name}
                </Typography>
                <Box display="flex" flexDirection="column" gap={1}>
                  <Box>
                    <Typography variant="caption" color="text.secondary">Name:</Typography>
                    <Typography variant="body2">{selectedPlugin.name}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">Version:</Typography>
                    <Typography variant="body2">{selectedPlugin.version}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">Type:</Typography>
                    <Typography variant="body2">{selectedPlugin.pluginType}</Typography>
                  </Box>
                  {selectedPlugin.description && (
                    <Box>
                      <Typography variant="caption" color="text.secondary">Description:</Typography>
                      <Typography variant="body2">{selectedPlugin.description}</Typography>
                    </Box>
                  )}
                  <Box>
                    <Typography variant="caption" color="text.secondary">Installed By:</Typography>
                    <Typography variant="body2">{selectedPlugin.installedBy}</Typography>
                  </Box>
                  <Box>
                    <Typography variant="caption" color="text.secondary">Installed At:</Typography>
                    <Typography variant="body2">
                      {new Date(selectedPlugin.installedAt).toLocaleString()}
                    </Typography>
                  </Box>
                </Box>
              </Box>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
          </DialogActions>
        </Dialog>

        {/* Configuration Dialog */}
        <Dialog open={configDialogOpen} onClose={() => setConfigDialogOpen(false)} maxWidth="md" fullWidth>
          <DialogTitle>
            Configure {selectedPlugin?.displayName || selectedPlugin?.name}
          </DialogTitle>
          <DialogContent>
            <Typography variant="body2" color="text.secondary" mb={2}>
              Edit the plugin configuration as JSON. This affects all users.
            </Typography>
            <TextField
              fullWidth
              multiline
              rows={12}
              value={configJson}
              onChange={(e) => setConfigJson(e.target.value)}
              placeholder="{}"
              sx={{ fontFamily: 'monospace' }}
            />
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setConfigDialogOpen(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleSaveConfig}>
              Save Configuration
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
    </WebSocketErrorBoundary>
  );
}

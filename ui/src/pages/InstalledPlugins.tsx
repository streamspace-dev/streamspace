import { useState, useEffect, useMemo } from 'react';
import {
  Box,
  Typography,
  Grid,
  Card,
  CardContent,
  CardActions,
  Button,
  IconButton,
  Chip,
  Alert,
  Switch,
  FormControlLabel,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Tooltip,
  InputAdornment,
  Tabs,
  Tab,
} from '@mui/material';
import {
  Settings as SettingsIcon,
  Delete as DeleteIcon,
  Info as InfoIcon,
  Extension as ExtensionIcon,
  Webhook as WebhookIcon,
  Api as ApiIcon,
  Dashboard as UiIcon,
  Palette as ThemeIcon,
  Search as SearchIcon,
  ExtensionOff as NoPluginsIcon,
  ShoppingCart as ShopIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import PluginCardSkeleton from '../components/PluginCardSkeleton';
import PluginConfigForm from '../components/PluginConfigForm';
import { api, type InstalledPlugin } from '../lib/api';
import { toast } from '../lib/toast';
import { useNavigate } from 'react-router-dom';
import { usePluginEvents } from '../hooks/useEnterpriseWebSocket';
import { useInstalledPlugins } from '../hooks/useApi';
import { useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

/**
 * InstalledPlugins - Management page for installed StreamSpace plugins
 *
 * Comprehensive plugin management interface providing:
 * - Grid view of all installed plugins with status
 * - Enable/disable toggle for each plugin
 * - Configuration management for plugin settings
 * - Uninstallation with confirmation dialogs
 * - Real-time plugin status updates via WebSocket
 * - Tab-based filtering (All, Enabled, Disabled)
 * - Search functionality for finding plugins
 * - Plugin type icons and categorization
 *
 * Plugin management features:
 * - Quick enable/disable toggle switches
 * - Configure button opens plugin-specific settings form
 * - View plugin details (version, author, description)
 * - Uninstall with dependency checking
 * - Real-time status updates (enabled → disabled)
 * - Plugin event notifications (installed, updated, uninstalled)
 * - Empty state with link to plugin catalog
 *
 * Plugin types supported:
 * - Extension: Core platform extensions
 * - Webhook: External webhook integrations
 * - API: API integrations and connectors
 * - UI: User interface components and themes
 * - Theme: Visual themes for the platform
 *
 * User workflows:
 * - View all installed plugins at a glance
 * - Enable/disable plugins without uninstalling
 * - Configure plugin settings via config modal
 * - Uninstall plugins with confirmation
 * - Filter by status (enabled/disabled)
 * - Search plugins by name
 * - Navigate to catalog to install more plugins
 *
 * Real-time features:
 * - Plugin state change notifications
 * - Installation/uninstallation events
 * - Configuration update events
 * - WebSocket connection status indicator
 *
 * @page
 * @route /plugins - Installed plugins management page
 * @access user - Shows only user-accessible plugins
 *
 * @component
 *
 * @returns {JSX.Element} Installed plugins page with management controls
 *
 * @example
 * // Route configuration:
 * <Route path="/plugins" element={<InstalledPlugins />} />
 * <Route path="/plugins/installed" element={<InstalledPlugins />} />
 *
 * @see PluginCatalog for installing new plugins
 * @see PluginConfigForm for plugin configuration interface
 */

const pluginTypeIcons: Record<string, JSX.Element> = {
  extension: <ExtensionIcon fontSize="small" />,
  webhook: <WebhookIcon fontSize="small" />,
  api: <ApiIcon fontSize="small" />,
  ui: <UiIcon fontSize="small" />,
  theme: <ThemeIcon fontSize="small" />,
};

const pluginTypeColors: Record<string, string> = {
  extension: '#4CAF50',
  webhook: '#FF9800',
  api: '#2196F3',
  ui: '#9C27B0',
  theme: '#E91E63',
};

function InstalledPluginsContent() {
  const navigate = useNavigate();
  const [filter, setFilter] = useState<'all' | 'enabled' | 'disabled'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [configDialogOpen, setConfigDialogOpen] = useState(false);
  const [selectedPlugin, setSelectedPlugin] = useState<InstalledPlugin | null>(null);
  const [configJson, setConfigJson] = useState('');
  const [configFormData, setConfigFormData] = useState<Record<string, any>>({});
  const [configMode, setConfigMode] = useState<'form' | 'json'>('form');

  // Fetch plugins via React Query
  const { data: plugins = [], isLoading: loading } = useInstalledPlugins();
  const queryClient = useQueryClient();

  // WebSocket connection state
  const [wsConnected, setWsConnected] = useState(false);
  const [wsReconnectAttempts, setWsReconnectAttempts] = useState(0);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time plugin events via WebSocket
  usePluginEvents((data: any) => {
    setWsConnected(true);
    setWsReconnectAttempts(0);

    // Show notifications for plugin events
    if (data.event_type === 'plugin.installed') {
      addNotification({
        message: `Plugin installed: ${data.plugin_name || 'Unknown'}`,
        severity: 'success',
        priority: 'medium',
        title: 'Plugin Installed',
      });
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } else if (data.event_type === 'plugin.enabled') {
      addNotification({
        message: `Plugin enabled: ${data.plugin_name || 'Unknown'}`,
        severity: 'success',
        priority: 'low',
        title: 'Plugin Enabled',
      });
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } else if (data.event_type === 'plugin.disabled') {
      addNotification({
        message: `Plugin disabled: ${data.plugin_name || 'Unknown'}`,
        severity: 'warning',
        priority: 'low',
        title: 'Plugin Disabled',
      });
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } else if (data.event_type === 'plugin.uninstalled') {
      addNotification({
        message: `Plugin uninstalled: ${data.plugin_name || 'Unknown'}`,
        severity: 'info',
        priority: 'medium',
        title: 'Plugin Uninstalled',
      });
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } else if (data.event_type === 'plugin.error') {
      addNotification({
        message: `Plugin error: ${data.plugin_name || 'Unknown'} - ${data.error || 'Unknown error'}`,
        severity: 'error',
        priority: 'high',
        title: 'Plugin Error',
        autoDismiss: false,
      });
    } else if (data.event_type === 'plugin.updated') {
      addNotification({
        message: `Plugin updated: ${data.plugin_name || 'Unknown'}`,
        severity: 'info',
        priority: 'medium',
        title: 'Plugin Updated',
      });
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    }
  });

  const handleTogglePlugin = async (plugin: InstalledPlugin) => {
    try {
      if (plugin.enabled) {
        await api.disablePlugin(plugin.id);
        toast.success(`${plugin.displayName || plugin.name} disabled`);
      } else {
        await api.enablePlugin(plugin.id);
        toast.success(`${plugin.displayName || plugin.name} enabled`);
      }
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } catch (error) {
      console.error('Failed to toggle plugin:', error);
      toast.error('Failed to toggle plugin');
    }
  };

  const handleOpenConfig = (plugin: InstalledPlugin) => {
    setSelectedPlugin(plugin);
    const config = plugin.config || {};
    setConfigJson(JSON.stringify(config, null, 2));
    setConfigFormData(config);
    // Use form mode if schema is available, otherwise JSON mode
    setConfigMode(plugin.manifest?.configSchema ? 'form' : 'json');
    setConfigDialogOpen(true);
  };

  const handleSaveConfig = async () => {
    if (!selectedPlugin) return;

    try {
      let config: Record<string, any>;

      if (configMode === 'form') {
        config = configFormData;
      } else {
        config = JSON.parse(configJson);
      }

      await api.updatePluginConfig(selectedPlugin.id, config);
      toast.success('Configuration updated');
      setConfigDialogOpen(false);
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } catch (error) {
      console.error('Failed to update configuration:', error);
      toast.error(configMode === 'json' ? 'Invalid JSON or failed to update configuration' : 'Failed to update configuration');
    }
  };

  const handleConfigFormChange = (data: Record<string, any>) => {
    setConfigFormData(data);
    setConfigJson(JSON.stringify(data, null, 2));
  };

  const handleConfigJsonChange = (json: string) => {
    setConfigJson(json);
    try {
      const data = JSON.parse(json);
      setConfigFormData(data);
    } catch {
      // Invalid JSON, don't update form data
    }
  };

  const handleUninstall = async (plugin: InstalledPlugin) => {
    if (!confirm(`Are you sure you want to uninstall ${plugin.displayName || plugin.name}?`)) {
      return;
    }

    try {
      await api.uninstallPlugin(plugin.id);
      toast.success(`${plugin.displayName || plugin.name} uninstalled`);
      queryClient.invalidateQueries({ queryKey: ['installed-plugins'] });
    } catch (error) {
      console.error('Failed to uninstall plugin:', error);
      toast.error('Failed to uninstall plugin');
    }
  };

  const filteredPlugins = useMemo(() => {
    return plugins.filter(plugin => {
      // Filter by enabled/disabled status
      if (filter === 'enabled' && !plugin.enabled) return false;
      if (filter === 'disabled' && plugin.enabled) return false;

      // Filter by search query
      if (searchQuery) {
        const query = searchQuery.toLowerCase();
        const matchesName = plugin.name?.toLowerCase().includes(query);
        const matchesDisplayName = plugin.displayName?.toLowerCase().includes(query);
        const matchesDescription = plugin.description?.toLowerCase().includes(query);
        const matchesType = plugin.pluginType?.toLowerCase().includes(query);

        if (!matchesName && !matchesDisplayName && !matchesDescription && !matchesType) {
          return false;
        }
      }

      return true;
    });
  }, [plugins, filter, searchQuery]);

  return (
    <Layout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Installed Plugins
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage your installed plugins
            </Typography>
          </Box>
          <Box display="flex" gap={2} alignItems="center">
            <EnhancedWebSocketStatus
              isConnected={wsConnected}
              reconnectAttempts={wsReconnectAttempts}
              size="small"
            />
            <Button variant="outlined" onClick={() => navigate('/plugin-catalog')}>
              <ShopIcon sx={{ mr: 1 }} />
              Browse Catalog
            </Button>
          </Box>
        </Box>

        {/* Search and Filter */}
        <Box mb={3}>
          <TextField
            fullWidth
            placeholder="Search installed plugins..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            sx={{ mb: 2 }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon />
                </InputAdornment>
              ),
            }}
          />
          <Box display="flex" gap={1} flexWrap="wrap">
            <Chip
              label={`All (${plugins.length})`}
              onClick={() => setFilter('all')}
              color={filter === 'all' ? 'primary' : 'default'}
              variant={filter === 'all' ? 'filled' : 'outlined'}
            />
            <Chip
              label={`Enabled (${plugins.filter(p => p.enabled).length})`}
              onClick={() => setFilter('enabled')}
              color={filter === 'enabled' ? 'primary' : 'default'}
              variant={filter === 'enabled' ? 'filled' : 'outlined'}
            />
            <Chip
              label={`Disabled (${plugins.filter(p => !p.enabled).length})`}
              onClick={() => setFilter('disabled')}
              color={filter === 'disabled' ? 'primary' : 'default'}
              variant={filter === 'disabled' ? 'filled' : 'outlined'}
            />
            {searchQuery && (
              <Chip
                label={`Search: "${searchQuery}"`}
                size="small"
                onDelete={() => setSearchQuery('')}
              />
            )}
          </Box>
        </Box>

        {/* Results */}
        {loading ? (
          <Grid container spacing={3}>
            {Array.from({ length: 6 }).map((_, index) => (
              <Grid item xs={12} sm={6} md={4} key={index}>
                <PluginCardSkeleton />
              </Grid>
            ))}
          </Grid>
        ) : filteredPlugins.length === 0 ? (
          <Box
            display="flex"
            flexDirection="column"
            alignItems="center"
            justifyContent="center"
            py={8}
            px={2}
            textAlign="center"
          >
            <NoPluginsIcon sx={{ fontSize: 80, color: 'text.secondary', mb: 2, opacity: 0.5 }} />
            <Typography variant="h5" gutterBottom>
              {plugins.length === 0 ? 'No Plugins Installed' : 'No Matching Plugins'}
            </Typography>
            <Typography variant="body2" color="text.secondary" mb={3} maxWidth={500}>
              {plugins.length === 0
                ? 'You haven\'t installed any plugins yet. Browse the plugin catalog to discover and install plugins.'
                : searchQuery
                ? `No plugins match "${searchQuery}". Try a different search term.`
                : `No ${filter} plugins found.`}
            </Typography>
            {plugins.length === 0 ? (
              <Button
                variant="contained"
                startIcon={<ShopIcon />}
                onClick={() => navigate('/plugins/catalog')}
              >
                Browse Plugin Catalog
              </Button>
            ) : searchQuery ? (
              <Button variant="outlined" onClick={() => setSearchQuery('')}>
                Clear Search
              </Button>
            ) : (
              <Button variant="outlined" onClick={() => setFilter('all')}>
                Show All Plugins
              </Button>
            )}
          </Box>
        ) : (
          <Grid container spacing={3}>
            {filteredPlugins.map((plugin) => (
              <Grid item xs={12} sm={6} md={4} key={plugin.id}>
                <Card
                  sx={{
                    height: '100%',
                    display: 'flex',
                    flexDirection: 'column',
                    opacity: plugin.enabled ? 1 : 0.6,
                  }}
                >
                  <CardContent sx={{ flexGrow: 1 }}>
                    <Box display="flex" justifyContent="space-between" alignItems="start" mb={2}>
                      <Box display="flex" alignItems="center" gap={1}>
                        {plugin.pluginType && (
                          <Box
                            sx={{
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              width: 32,
                              height: 32,
                              borderRadius: '50%',
                              bgcolor: pluginTypeColors[plugin.pluginType] || '#757575',
                              color: 'white',
                            }}
                          >
                            {pluginTypeIcons[plugin.pluginType] || <ExtensionIcon fontSize="small" />}
                          </Box>
                        )}
                        <Box>
                          <Typography variant="h6" sx={{ fontSize: '1rem', fontWeight: 600 }}>
                            {plugin.displayName || plugin.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            v{plugin.version}
                          </Typography>
                        </Box>
                      </Box>
                      <FormControlLabel
                        control={
                          <Switch
                            checked={plugin.enabled}
                            onChange={() => handleTogglePlugin(plugin)}
                            size="small"
                          />
                        }
                        label=""
                      />
                    </Box>

                    {plugin.description && (
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        sx={{
                          mb: 2,
                          minHeight: 40,
                          display: '-webkit-box',
                          WebkitLineClamp: 2,
                          WebkitBoxOrient: 'vertical',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                        }}
                      >
                        {plugin.description}
                      </Typography>
                    )}

                    <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
                      <Chip
                        label={plugin.enabled ? 'Enabled' : 'Disabled'}
                        size="small"
                        color={plugin.enabled ? 'success' : 'default'}
                      />
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
                    </Box>

                    <Typography variant="caption" color="text.secondary" display="block" mt={1}>
                      Installed by {plugin.installedBy} on {new Date(plugin.installedAt).toLocaleDateString()}
                    </Typography>
                  </CardContent>

                  <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
                    <Box display="flex" gap={1}>
                      <Tooltip title="Configure">
                        <IconButton
                          size="small"
                          onClick={() => handleOpenConfig(plugin)}
                        >
                          <SettingsIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      {plugin.manifest && (
                        <Tooltip title="View Details">
                          <IconButton size="small">
                            <InfoIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </Box>
                    <Tooltip title="Uninstall">
                      <IconButton
                        size="small"
                        color="error"
                        onClick={() => handleUninstall(plugin)}
                      >
                        <DeleteIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </CardActions>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        {/* Configuration Dialog */}
        <Dialog open={configDialogOpen} onClose={() => setConfigDialogOpen(false)} maxWidth="md" fullWidth>
          <DialogTitle>
            Configure {selectedPlugin?.displayName || selectedPlugin?.name}
          </DialogTitle>
          <DialogContent>
            {selectedPlugin?.manifest?.configSchema && (
              <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
                <Tabs value={configMode} onChange={(_, v) => setConfigMode(v)}>
                  <Tab label="Form" value="form" />
                  <Tab label="JSON" value="json" />
                </Tabs>
              </Box>
            )}

            {configMode === 'form' ? (
              <PluginConfigForm
                schema={selectedPlugin?.manifest?.configSchema}
                value={configFormData}
                onChange={handleConfigFormChange}
              />
            ) : (
              <Box>
                <Typography variant="body2" color="text.secondary" mb={2}>
                  Edit the plugin configuration as JSON. Invalid JSON will not be saved.
                </Typography>
                <TextField
                  fullWidth
                  multiline
                  rows={12}
                  value={configJson}
                  onChange={(e) => handleConfigJsonChange(e.target.value)}
                  placeholder="{}"
                  sx={{ fontFamily: 'monospace' }}
                />
              </Box>
            )}
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
  );
}

export default function InstalledPlugins() {
  return (
    <WebSocketErrorBoundary>
      <InstalledPluginsContent />
    </WebSocketErrorBoundary>
  );
}

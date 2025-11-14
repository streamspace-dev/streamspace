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

  useEffect(() => {
    loadPlugins();
  }, []);

  const loadPlugins = async () => {
    setLoading(true);
    try {
      const data = await api.listInstalledPlugins();
      setPlugins(data);
    } catch (error) {
      console.error('Failed to load plugins:', error);
      toast.error('Failed to load plugins');
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
    total: plugins.length,
    enabled: plugins.filter(p => p.enabled).length,
    disabled: plugins.filter(p => !p.enabled).length,
    byType: {
      extension: plugins.filter(p => p.pluginType === 'extension').length,
      webhook: plugins.filter(p => p.pluginType === 'webhook').length,
      api: plugins.filter(p => p.pluginType === 'api').length,
      ui: plugins.filter(p => p.pluginType === 'ui').length,
      theme: plugins.filter(p => p.pluginType === 'theme').length,
    },
  };

  return (
    <Layout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Plugin Management
            </Typography>
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
  );
}

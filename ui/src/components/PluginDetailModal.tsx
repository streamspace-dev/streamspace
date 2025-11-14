import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Box,
  Typography,
  Chip,
  Tabs,
  Tab,
  TextField,
  Rating,
  Avatar,
  Divider,
  CircularProgress,
  Alert,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Tooltip,
  ListItemIcon,
} from '@mui/material';
import {
  Close as CloseIcon,
  GetApp as InstallIcon,
  Star as StarIcon,
  Extension as ExtensionIcon,
  Webhook as WebhookIcon,
  Api as ApiIcon,
  Dashboard as UiIcon,
  Palette as ThemeIcon,
  Security as SecurityIcon,
  Code as CodeIcon,
  Warning as WarningIcon,
  CheckCircle as CheckIcon,
} from '@mui/icons-material';
import RatingStars from './RatingStars';
import { api, type CatalogPlugin, type PluginRating } from '../lib/api';
import { useUserStore } from '../store/userStore';

// Permission descriptions and risk levels
const permissionInfo: Record<string, { description: string; risk: 'low' | 'medium' | 'high' }> = {
  'read:sessions': { description: 'Read session data and status', risk: 'low' },
  'write:sessions': { description: 'Create, modify, or delete sessions', risk: 'high' },
  'read:users': { description: 'View user information', risk: 'medium' },
  'write:users': { description: 'Modify user accounts and settings', risk: 'high' },
  'read:templates': { description: 'Access template catalog', risk: 'low' },
  'write:templates': { description: 'Create or modify templates', risk: 'medium' },
  'admin': { description: 'Full administrative access', risk: 'high' },
  'network': { description: 'Make external network requests', risk: 'medium' },
  'filesystem': { description: 'Access local filesystem', risk: 'high' },
  'execute': { description: 'Execute external commands', risk: 'high' },
  'database': { description: 'Direct database access', risk: 'high' },
  'api': { description: 'Access StreamSpace API', risk: 'low' },
  'webhooks': { description: 'Create and manage webhooks', risk: 'medium' },
  'notifications': { description: 'Send user notifications', risk: 'low' },
};

function getPermissionInfo(permission: string) {
  return permissionInfo[permission] || {
    description: permission,
    risk: 'medium' as const,
  };
}

interface PluginDetailModalProps {
  open: boolean;
  plugin: CatalogPlugin | null;
  onClose: () => void;
  onInstall?: (plugin: CatalogPlugin) => void;
}

const pluginTypeIcons: Record<string, JSX.Element> = {
  extension: <ExtensionIcon />,
  webhook: <WebhookIcon />,
  api: <ApiIcon />,
  ui: <UiIcon />,
  theme: <ThemeIcon />,
};

const pluginTypeColors: Record<string, string> = {
  extension: '#4CAF50',
  webhook: '#FF9800',
  api: '#2196F3',
  ui: '#9C27B0',
  theme: '#E91E63',
};

export default function PluginDetailModal({
  open,
  plugin,
  onClose,
  onInstall,
}: PluginDetailModalProps) {
  const [tabValue, setTabValue] = useState(0);
  const [ratings, setRatings] = useState<PluginRating[]>([]);
  const [loadingRatings, setLoadingRatings] = useState(false);
  const [userRating, setUserRating] = useState(0);
  const [userReview, setUserReview] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const username = useUserStore((state) => state.user?.username);

  useEffect(() => {
    if (open && plugin) {
      // Load ratings if on ratings tab
      if (tabValue === 1) {
        loadRatings();
      }
    }
  }, [open, plugin, tabValue]);

  const loadRatings = async () => {
    if (!plugin) return;

    setLoadingRatings(true);
    try {
      const data = await api.getPluginRatings(plugin.id);
      setRatings(data);

      // Check if user has already rated
      const existingRating = data.find(r => r.username === username);
      if (existingRating) {
        setUserRating(existingRating.rating);
        setUserReview(existingRating.review || '');
      }
    } catch (error) {
      console.error('Failed to load ratings:', error);
    } finally {
      setLoadingRatings(false);
    }
  };

  const handleSubmitRating = async () => {
    if (!plugin || userRating === 0) return;

    setSubmitting(true);
    try {
      await api.ratePlugin(plugin.id, userRating, userReview);
      await loadRatings();
      setUserRating(0);
      setUserReview('');
    } catch (error) {
      console.error('Failed to submit rating:', error);
    } finally {
      setSubmitting(false);
    }
  };

  const handleInstall = () => {
    if (plugin) {
      onInstall?.(plugin);
      onClose();
    }
  };

  if (!plugin) return null;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box display="flex" justifyContent="space-between" alignItems="start">
          <Box display="flex" alignItems="center" gap={1}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 40,
                height: 40,
                borderRadius: 1,
                bgcolor: pluginTypeColors[plugin.pluginType] || '#757575',
                color: 'white',
              }}
            >
              {pluginTypeIcons[plugin.pluginType] || <ExtensionIcon />}
            </Box>
            <Box>
              <Typography variant="h5" sx={{ fontWeight: 600 }}>
                {plugin.displayName}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                v{plugin.version} • {plugin.repository.name}
              </Typography>
            </Box>
          </Box>
          <IconButton onClick={onClose} size="small">
            <CloseIcon />
          </IconButton>
        </Box>
      </DialogTitle>

      <DialogContent>
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
          <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)}>
            <Tab label="Details" />
            <Tab label={`Reviews (${plugin.ratingCount})`} />
          </Tabs>
        </Box>

        {tabValue === 0 && (
          <Box>
            {plugin.iconUrl && (
              <Box
                component="img"
                src={plugin.iconUrl}
                alt={plugin.displayName}
                sx={{
                  width: 120,
                  height: 120,
                  borderRadius: 2,
                  objectFit: 'cover',
                  mb: 2,
                }}
                onError={(e) => {
                  e.currentTarget.style.display = 'none';
                }}
              />
            )}

            <Box mb={2}>
              <RatingStars
                rating={plugin.avgRating}
                count={plugin.ratingCount}
                size="medium"
              />
            </Box>

            <Typography variant="body1" paragraph>
              {plugin.description}
            </Typography>

            <Box display="flex" gap={1} flexWrap="wrap" mb={2}>
              <Chip label={plugin.category} variant="outlined" />
              <Chip
                label={plugin.pluginType}
                sx={{
                  bgcolor: pluginTypeColors[plugin.pluginType] + '20',
                  color: pluginTypeColors[plugin.pluginType],
                  fontWeight: 600,
                }}
              />
              {plugin.tags.map((tag) => (
                <Chip key={tag} label={tag} size="small" />
              ))}
            </Box>

            <Divider sx={{ my: 2 }} />

            {/* Plugin Metadata */}
            <Box mb={2}>
              <Typography variant="subtitle2" gutterBottom>
                Plugin Information
              </Typography>
              <Box display="flex" gap={4} flexWrap="wrap">
                <Box>
                  <Typography variant="caption" color="text.secondary" display="block">
                    Author
                  </Typography>
                  <Typography variant="body2">
                    {plugin.manifest.author || 'Unknown'}
                  </Typography>
                </Box>
                <Box>
                  <Typography variant="caption" color="text.secondary" display="block">
                    Installs
                  </Typography>
                  <Box display="flex" alignItems="center" gap={0.5}>
                    <InstallIcon fontSize="small" />
                    <Typography variant="body2">
                      {plugin.installCount.toLocaleString()}
                    </Typography>
                  </Box>
                </Box>
              </Box>
            </Box>

            {/* Permissions */}
            {plugin.manifest.permissions && plugin.manifest.permissions.length > 0 && (
              <>
                <Divider sx={{ my: 2 }} />
                <Box mb={2}>
                  <Box display="flex" alignItems="center" gap={1} mb={1}>
                    <SecurityIcon fontSize="small" color="warning" />
                    <Typography variant="subtitle2">
                      Required Permissions
                    </Typography>
                  </Box>
                  <Alert severity="info" sx={{ mb: 1, fontSize: '0.8rem' }}>
                    This plugin requires the following permissions to function properly.
                  </Alert>
                  <List dense>
                    {plugin.manifest.permissions.map((permission, index) => {
                      const info = getPermissionInfo(permission);
                      const riskColor = info.risk === 'high' ? 'error' : info.risk === 'medium' ? 'warning' : 'success';
                      return (
                        <Tooltip
                          key={index}
                          title={
                            <Box>
                              <Typography variant="body2" fontWeight={600} gutterBottom>
                                {permission}
                              </Typography>
                              <Typography variant="caption">
                                {info.description}
                              </Typography>
                              <Box mt={0.5}>
                                <Chip
                                  label={`${info.risk.toUpperCase()} RISK`}
                                  size="small"
                                  color={riskColor}
                                  sx={{ height: 16, fontSize: '0.65rem' }}
                                />
                              </Box>
                            </Box>
                          }
                          placement="right"
                          arrow
                        >
                          <ListItem>
                            <ListItemIcon sx={{ minWidth: 36 }}>
                              {info.risk === 'high' ? (
                                <WarningIcon fontSize="small" color="error" />
                              ) : (
                                <CheckIcon fontSize="small" color={riskColor} />
                              )}
                            </ListItemIcon>
                            <ListItemText
                              primary={permission}
                              secondary={info.description}
                              primaryTypographyProps={{ variant: 'body2', fontWeight: 500 }}
                              secondaryTypographyProps={{ variant: 'caption' }}
                            />
                          </ListItem>
                        </Tooltip>
                      );
                    })}
                  </List>
                </Box>
              </>
            )}

            {/* Dependencies */}
            {plugin.manifest.dependencies && Object.keys(plugin.manifest.dependencies).length > 0 && (
              <>
                <Divider sx={{ my: 2 }} />
                <Box mb={2}>
                  <Box display="flex" alignItems="center" gap={1} mb={1}>
                    <CodeIcon fontSize="small" />
                    <Typography variant="subtitle2">
                      Dependencies
                    </Typography>
                  </Box>
                  <List dense>
                    {Object.entries(plugin.manifest.dependencies).map(([name, version]) => (
                      <ListItem key={name}>
                        <ListItemText
                          primary={`${name}: ${version}`}
                          primaryTypographyProps={{ variant: 'body2', fontFamily: 'monospace' }}
                        />
                      </ListItem>
                    ))}
                  </List>
                </Box>
              </>
            )}

            {/* Entrypoints */}
            {plugin.manifest.entrypoints && (
              <>
                <Divider sx={{ my: 2 }} />
                <Box mb={2}>
                  <Typography variant="subtitle2" gutterBottom>
                    Entry Points
                  </Typography>
                  <Box display="flex" gap={1} flexWrap="wrap">
                    {plugin.manifest.entrypoints.main && (
                      <Chip label="Main" size="small" variant="outlined" />
                    )}
                    {plugin.manifest.entrypoints.ui && (
                      <Chip label="UI" size="small" variant="outlined" />
                    )}
                    {plugin.manifest.entrypoints.api && (
                      <Chip label="API" size="small" variant="outlined" />
                    )}
                    {plugin.manifest.entrypoints.webhook && (
                      <Chip label="Webhook" size="small" variant="outlined" />
                    )}
                    {plugin.manifest.entrypoints.cli && (
                      <Chip label="CLI" size="small" variant="outlined" />
                    )}
                  </Box>
                </Box>
              </>
            )}
          </Box>
        )}

        {tabValue === 1 && (
          <Box>
            {username && (
              <Box mb={3} p={2} sx={{ bgcolor: 'background.default', borderRadius: 1 }}>
                <Typography variant="subtitle2" gutterBottom>
                  Rate this plugin
                </Typography>
                <Box display="flex" alignItems="center" gap={1} mb={2}>
                  <Rating
                    value={userRating}
                    onChange={(_, value) => setUserRating(value || 0)}
                    size="large"
                  />
                  {userRating > 0 && (
                    <Typography variant="body2" color="text.secondary">
                      {userRating} / 5
                    </Typography>
                  )}
                </Box>
                <TextField
                  fullWidth
                  multiline
                  rows={3}
                  placeholder="Write a review (optional)"
                  value={userReview}
                  onChange={(e) => setUserReview(e.target.value)}
                  sx={{ mb: 2 }}
                />
                <Button
                  variant="contained"
                  onClick={handleSubmitRating}
                  disabled={userRating === 0 || submitting}
                  startIcon={<StarIcon />}
                >
                  {submitting ? 'Submitting...' : 'Submit Rating'}
                </Button>
              </Box>
            )}

            {loadingRatings ? (
              <Box display="flex" justifyContent="center" py={4}>
                <CircularProgress />
              </Box>
            ) : ratings.length === 0 ? (
              <Alert severity="info">No reviews yet. Be the first to rate this plugin!</Alert>
            ) : (
              <Box>
                {ratings.map((rating) => (
                  <Box key={rating.id} mb={2} pb={2} sx={{ borderBottom: 1, borderColor: 'divider' }}>
                    <Box display="flex" alignItems="center" gap={1} mb={1}>
                      <Avatar sx={{ width: 32, height: 32 }}>
                        {rating.username[0].toUpperCase()}
                      </Avatar>
                      <Box>
                        <Typography variant="subtitle2">{rating.fullName || rating.username}</Typography>
                        <RatingStars rating={rating.rating} showCount={false} size="small" />
                      </Box>
                      <Typography variant="caption" color="text.secondary" sx={{ ml: 'auto' }}>
                        {new Date(rating.createdAt).toLocaleDateString()}
                      </Typography>
                    </Box>
                    {rating.review && (
                      <Typography variant="body2" color="text.secondary">
                        {rating.review}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Box>
            )}
          </Box>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Close</Button>
        <Button
          variant="contained"
          startIcon={<InstallIcon />}
          onClick={handleInstall}
        >
          Install Plugin
        </Button>
      </DialogActions>
    </Dialog>
  );
}

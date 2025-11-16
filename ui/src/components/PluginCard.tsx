import { memo } from 'react';
import {
  Card,
  CardContent,
  CardActions,
  Typography,
  Box,
  Chip,
  Button,
  IconButton,
  Tooltip,
} from '@mui/material';
import {
  GetApp as InstallIcon,
  Info as InfoIcon,
  Extension as ExtensionIcon,
  Webhook as WebhookIcon,
  Api as ApiIcon,
  Dashboard as UiIcon,
  Palette as ThemeIcon,
} from '@mui/icons-material';
import RatingStars from './RatingStars';
import type { CatalogPlugin } from '../lib/api';

interface PluginCardProps {
  plugin: CatalogPlugin;
  onInstall?: (plugin: CatalogPlugin) => void;
  onViewDetails?: (plugin: CatalogPlugin) => void;
  mode?: 'catalog' | 'installed';
}

/**
 * PluginCard - Display card for a plugin in the catalog or installed plugins list
 *
 * Presents plugin information in a compact, visually appealing card format with
 * plugin type badge, icon, rating, description, tags, and action buttons. Supports
 * both catalog mode (with install button) and installed mode. Includes hover effects
 * and memoization for performance.
 *
 * Features:
 * - Plugin type badge with color coding and icons
 * - Star rating display with review count
 * - Truncated description with ellipsis
 * - Category and tag chips
 * - Install count statistics
 * - Install and view details actions
 * - Memoized to prevent unnecessary re-renders
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {CatalogPlugin} props.plugin - Plugin data to display
 * @param {Function} [props.onInstall] - Callback when install button is clicked
 * @param {Function} [props.onViewDetails] - Callback when info button is clicked
 * @param {'catalog' | 'installed'} [props.mode='catalog'] - Display mode for the card
 *
 * @returns {JSX.Element} Rendered plugin card
 *
 * @example
 * <PluginCard
 *   plugin={pluginData}
 *   mode="catalog"
 *   onInstall={handleInstall}
 *   onViewDetails={handleViewDetails}
 * />
 *
 * @see RatingStars for rating display component
 * @see PluginDetailModal for detailed plugin view
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

function PluginCard({
  plugin,
  onInstall,
  onViewDetails,
  mode = 'catalog',
}: PluginCardProps) {
  return (
    <Card
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        position: 'relative',
        '&:hover': {
          boxShadow: 4,
          transform: 'translateY(-2px)',
          transition: 'all 0.2s ease-in-out',
        },
      }}
    >
      {/* Plugin Type Badge */}
      <Box
        sx={{
          position: 'absolute',
          top: 8,
          right: 8,
          zIndex: 1,
        }}
      >
        <Tooltip title={`${plugin.pluginType} plugin`}>
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
        </Tooltip>
      </Box>

      <CardContent sx={{ flexGrow: 1, pb: 1 }}>
        <Box display="flex" alignItems="center" gap={1} mb={1}>
          {plugin.iconUrl && (
            <Box
              component="img"
              src={plugin.iconUrl}
              alt={plugin.displayName}
              sx={{
                width: 40,
                height: 40,
                borderRadius: 1,
                objectFit: 'cover',
              }}
              onError={(e) => {
                e.currentTarget.style.display = 'none';
              }}
            />
          )}
          <Box flexGrow={1}>
            <Typography variant="h6" sx={{ fontWeight: 600, fontSize: '1rem' }}>
              {plugin.displayName}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              v{plugin.version} • {plugin.repository.name}
            </Typography>
          </Box>
        </Box>

        {plugin.avgRating > 0 && (
          <Box mb={1}>
            <RatingStars
              rating={plugin.avgRating}
              count={plugin.ratingCount}
              size="small"
            />
          </Box>
        )}

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
          {plugin.description || 'No description available'}
        </Typography>

        <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
          {plugin.category && (
            <Chip label={plugin.category} size="small" variant="outlined" />
          )}
          <Chip
            label={plugin.pluginType}
            size="small"
            sx={{
              bgcolor: pluginTypeColors[plugin.pluginType] + '20',
              color: pluginTypeColors[plugin.pluginType],
              fontWeight: 600,
            }}
          />
        </Box>

        {plugin.tags && plugin.tags.length > 0 && (
          <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
            {plugin.tags.slice(0, 3).map((tag) => (
              <Chip key={tag} label={tag} size="small" sx={{ fontSize: '0.7rem' }} />
            ))}
            {plugin.tags.length > 3 && (
              <Chip label={`+${plugin.tags.length - 3}`} size="small" sx={{ fontSize: '0.7rem' }} />
            )}
          </Box>
        )}

        <Box display="flex" gap={2} mt={2}>
          <Tooltip title="Installs">
            <Box display="flex" alignItems="center" gap={0.5}>
              <InstallIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
              <Typography variant="caption" color="text.secondary">
                {plugin.installCount.toLocaleString()}
              </Typography>
            </Box>
          </Tooltip>
        </Box>
      </CardContent>

      <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
        {mode === 'catalog' && (
          <Button
            size="small"
            variant="contained"
            startIcon={<InstallIcon />}
            onClick={() => onInstall?.(plugin)}
          >
            Install
          </Button>
        )}
        <IconButton
          size="small"
          onClick={() => onViewDetails?.(plugin)}
          title="View Details"
        >
          <InfoIcon />
        </IconButton>
      </CardActions>
    </Card>
  );
}

// Memoize to prevent re-renders when plugin data hasn't changed
export default memo(PluginCard, (prevProps, nextProps) => {
  return (
    prevProps.plugin.id === nextProps.plugin.id &&
    prevProps.plugin.installCount === nextProps.plugin.installCount &&
    prevProps.plugin.avgRating === nextProps.plugin.avgRating &&
    prevProps.plugin.ratingCount === nextProps.plugin.ratingCount &&
    prevProps.mode === nextProps.mode
  );
});

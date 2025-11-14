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
  Star as FeaturedIcon,
  Visibility as ViewIcon,
  GetApp as InstallIcon,
  Info as InfoIcon,
} from '@mui/icons-material';
import RatingStars from './RatingStars';
import type { CatalogTemplate } from '../lib/api';

interface TemplateCardProps {
  template: CatalogTemplate;
  onInstall?: (template: CatalogTemplate) => void;
  onViewDetails?: (template: CatalogTemplate) => void;
  mode?: 'catalog' | 'installed';
}

export default function TemplateCard({
  template,
  onInstall,
  onViewDetails,
  mode = 'catalog',
}: TemplateCardProps) {
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
      {template.isFeatured && (
        <Box
          sx={{
            position: 'absolute',
            top: 8,
            right: 8,
            zIndex: 1,
          }}
        >
          <Tooltip title="Featured Template">
            <FeaturedIcon sx={{ color: '#FFA500', fontSize: 24 }} />
          </Tooltip>
        </Box>
      )}

      <CardContent sx={{ flexGrow: 1, pb: 1 }}>
        <Box display="flex" alignItems="center" gap={1} mb={1}>
          {template.icon && (
            <Box
              component="img"
              src={template.icon}
              alt={template.displayName}
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
              {template.displayName}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              v{template.version} • {template.repository.name}
            </Typography>
          </Box>
        </Box>

        {template.avgRating > 0 && (
          <Box mb={1}>
            <RatingStars
              rating={template.avgRating}
              count={template.ratingCount}
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
          {template.description || 'No description available'}
        </Typography>

        <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
          {template.category && (
            <Chip label={template.category} size="small" variant="outlined" />
          )}
          {template.appType && (
            <Chip label={template.appType} size="small" color="primary" variant="outlined" />
          )}
        </Box>

        {template.tags && template.tags.length > 0 && (
          <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
            {template.tags.slice(0, 3).map((tag) => (
              <Chip key={tag} label={tag} size="small" sx={{ fontSize: '0.7rem' }} />
            ))}
            {template.tags.length > 3 && (
              <Chip label={`+${template.tags.length - 3}`} size="small" sx={{ fontSize: '0.7rem' }} />
            )}
          </Box>
        )}

        <Box display="flex" gap={2} mt={2}>
          <Tooltip title="Installs">
            <Box display="flex" alignItems="center" gap={0.5}>
              <InstallIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
              <Typography variant="caption" color="text.secondary">
                {template.installCount.toLocaleString()}
              </Typography>
            </Box>
          </Tooltip>
          <Tooltip title="Views">
            <Box display="flex" alignItems="center" gap={0.5}>
              <ViewIcon sx={{ fontSize: 16, color: 'text.secondary' }} />
              <Typography variant="caption" color="text.secondary">
                {template.viewCount.toLocaleString()}
              </Typography>
            </Box>
          </Tooltip>
        </Box>
      </CardContent>

      <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
        {mode === 'catalog' ? (
          <Button
            size="small"
            variant="contained"
            onClick={() => onInstall?.(template)}
          >
            Install
          </Button>
        ) : (
          <Button
            size="small"
            variant="contained"
            onClick={() => onInstall?.(template)}
          >
            Create Session
          </Button>
        )}
        <IconButton
          size="small"
          onClick={() => onViewDetails?.(template)}
          title="View Details"
        >
          <InfoIcon />
        </IconButton>
      </CardActions>
    </Card>
  );
}

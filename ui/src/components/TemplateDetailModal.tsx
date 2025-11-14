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
} from '@mui/material';
import {
  Close as CloseIcon,
  GetApp as InstallIcon,
  Visibility as ViewIcon,
  Star as StarIcon,
} from '@mui/icons-material';
import RatingStars from './RatingStars';
import { api, type CatalogTemplate, type TemplateRating } from '../lib/api';
import { useUserStore } from '../store/userStore';

interface TemplateDetailModalProps {
  open: boolean;
  template: CatalogTemplate | null;
  onClose: () => void;
  onInstall?: (template: CatalogTemplate) => void;
}

export default function TemplateDetailModal({
  open,
  template,
  onClose,
  onInstall,
}: TemplateDetailModalProps) {
  const [tabValue, setTabValue] = useState(0);
  const [ratings, setRatings] = useState<TemplateRating[]>([]);
  const [loadingRatings, setLoadingRatings] = useState(false);
  const [userRating, setUserRating] = useState(0);
  const [userReview, setUserReview] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const username = useUserStore((state) => state.user?.username);

  useEffect(() => {
    if (open && template) {
      // Record view
      api.recordTemplateView(template.id).catch(console.error);

      // Load ratings if on ratings tab
      if (tabValue === 1) {
        loadRatings();
      }
    }
  }, [open, template, tabValue]);

  const loadRatings = async () => {
    if (!template) return;

    setLoadingRatings(true);
    try {
      const data = await api.getTemplateRatings(template.id);
      setRatings(data.ratings);

      // Check if user has already rated
      const existingRating = data.ratings.find(r => r.username === username);
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
    if (!template || userRating === 0) return;

    setSubmitting(true);
    try {
      await api.addTemplateRating(template.id, userRating, userReview);
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
    if (template) {
      onInstall?.(template);
      onClose();
    }
  };

  if (!template) return null;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        <Box display="flex" justifyContent="space-between" alignItems="start">
          <Box>
            <Typography variant="h5" sx={{ fontWeight: 600 }}>
              {template.displayName}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              v{template.version} • {template.repository.name}
            </Typography>
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
            <Tab label={`Reviews (${template.ratingCount})`} />
          </Tabs>
        </Box>

        {tabValue === 0 && (
          <Box>
            {template.icon && (
              <Box
                component="img"
                src={template.icon}
                alt={template.displayName}
                sx={{
                  width: 120,
                  height: 120,
                  borderRadius: 2,
                  objectFit: 'cover',
                  mb: 2,
                }}
              />
            )}

            <Box mb={2}>
              <RatingStars
                rating={template.avgRating}
                count={template.ratingCount}
                size="medium"
              />
            </Box>

            <Typography variant="body1" paragraph>
              {template.description}
            </Typography>

            <Box display="flex" gap={1} flexWrap="wrap" mb={2}>
              <Chip label={template.category} variant="outlined" />
              <Chip label={template.appType} color="primary" variant="outlined" />
              {template.tags.map((tag) => (
                <Chip key={tag} label={tag} size="small" />
              ))}
            </Box>

            <Divider sx={{ my: 2 }} />

            <Box display="flex" gap={4}>
              <Box>
                <Typography variant="caption" color="text.secondary" display="block">
                  Installs
                </Typography>
                <Box display="flex" alignItems="center" gap={0.5}>
                  <InstallIcon fontSize="small" />
                  <Typography variant="h6">
                    {template.installCount.toLocaleString()}
                  </Typography>
                </Box>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary" display="block">
                  Views
                </Typography>
                <Box display="flex" alignItems="center" gap={0.5}>
                  <ViewIcon fontSize="small" />
                  <Typography variant="h6">
                    {template.viewCount.toLocaleString()}
                  </Typography>
                </Box>
              </Box>
            </Box>
          </Box>
        )}

        {tabValue === 1 && (
          <Box>
            {username && (
              <Box mb={3} p={2} sx={{ bgcolor: 'background.default', borderRadius: 1 }}>
                <Typography variant="subtitle2" gutterBottom>
                  Rate this template
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
              <Alert severity="info">No reviews yet. Be the first to rate this template!</Alert>
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
          Install Template
        </Button>
      </DialogActions>
    </Dialog>
  );
}

import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Grid,
  TextField,
  InputAdornment,
  MenuItem,
  CircularProgress,
  Alert,
  Pagination,
  Button,
  Chip,
} from '@mui/material';
import {
  Search as SearchIcon,
  FilterList as FilterIcon,
  Star as FeaturedIcon,
  Refresh as RefreshIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import TemplateCard from '../components/TemplateCard';
import TemplateDetailModal from '../components/TemplateDetailModal';
import { api, type CatalogTemplate, type CatalogFilters } from '../lib/api';
import { useNavigate } from 'react-router-dom';
import { useTemplateEvents } from '../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

/**
 * EnhancedCatalogContent - Internal component for EnhancedCatalog page logic
 */
function EnhancedCatalogContent() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [templates, setTemplates] = useState<CatalogTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState<CatalogTemplate | null>(null);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [filters, setFilters] = useState<CatalogFilters>({
    search: '',
    category: '',
    tag: '',
    appType: '',
    featured: false,
    sort: 'popular',
    page: 1,
    limit: 12,
  });
  const [totalPages, setTotalPages] = useState(1);
  const [categories, setCategories] = useState<string[]>([]);
  const [appTypes, setAppTypes] = useState<string[]>([]);

  // WebSocket connection state
  const [wsConnected, setWsConnected] = useState(false);
  const [wsReconnectAttempts, setWsReconnectAttempts] = useState(0);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time template events via WebSocket
  useTemplateEvents((data: any) => {
    setWsConnected(true);
    setWsReconnectAttempts(0);

    // Show notifications for template events
    if (data.event_type === 'template.created' || data.event_type === 'template.added') {
      addNotification({
        message: `New template available: ${data.template_name || 'Unknown'}`,
        severity: 'success',
        priority: 'medium',
        title: 'New Template',
      });
      // Refresh template list
      loadTemplates();
    } else if (data.event_type === 'template.updated') {
      addNotification({
        message: `Template updated: ${data.template_name || 'Unknown'}`,
        severity: 'info',
        priority: 'low',
        title: 'Template Updated',
      });
      // Refresh template list
      loadTemplates();
    } else if (data.event_type === 'template.deleted') {
      addNotification({
        message: `Template removed: ${data.template_name || 'Unknown'}`,
        severity: 'warning',
        priority: 'medium',
        title: 'Template Removed',
      });
      // Refresh template list
      loadTemplates();
    } else if (data.event_type === 'template.featured') {
      addNotification({
        message: `New featured template: ${data.template_name || 'Unknown'}`,
        severity: 'info',
        priority: 'high',
        title: 'Featured Template',
      });
      // Refresh template list if showing featured
      if (filters.featured) {
        loadTemplates();
      }
    }
  });

  useEffect(() => {
    loadTemplates();
  }, [filters]);

  useEffect(() => {
    // Extract unique categories and app types from templates
    const uniqueCategories = Array.from(new Set(templates.map(t => t.category).filter(Boolean)));
    const uniqueAppTypes = Array.from(new Set(templates.map(t => t.appType).filter(Boolean)));
    setCategories(uniqueCategories);
    setAppTypes(uniqueAppTypes);
  }, [templates]);

  const loadTemplates = async () => {
    setLoading(true);
    try {
      const data = await api.listCatalogTemplates(filters);
      setTemplates(data.templates);
      setTotalPages(data.totalPages);
    } catch (error) {
      console.error('Failed to load templates:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSearch = (value: string) => {
    setFilters({ ...filters, search: value, page: 1 });
  };

  const handleCategoryChange = (category: string) => {
    setFilters({ ...filters, category, page: 1 });
  };

  const handleAppTypeChange = (appType: string) => {
    setFilters({ ...filters, appType, page: 1 });
  };

  const handleSortChange = (sort: CatalogFilters['sort']) => {
    setFilters({ ...filters, sort, page: 1 });
  };

  const handleFeaturedToggle = () => {
    setFilters({ ...filters, featured: !filters.featured, page: 1 });
  };

  const handlePageChange = (_: unknown, page: number) => {
    setFilters({ ...filters, page });
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleViewDetails = (template: CatalogTemplate) => {
    setSelectedTemplate(template);
    setDetailModalOpen(true);
  };

  const handleInstall = async (template: CatalogTemplate) => {
    try {
      // Record install
      await api.recordTemplateInstall(template.id);

      // Navigate to create session with this template
      navigate('/sessions', { state: { createSession: true, template: template.name } });
    } catch (error) {
      console.error('Failed to install template:', error);
    }
  };

  const clearFilters = () => {
    setFilters({
      search: '',
      category: '',
      tag: '',
      appType: '',
      featured: false,
      sort: 'popular',
      page: 1,
      limit: 12,
    });
  };

  const hasActiveFilters = filters.search || filters.category || filters.appType || filters.featured;

  return (
    <Layout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Template Catalog
          </Typography>
          <Box display="flex" gap={2} alignItems="center">
            <EnhancedWebSocketStatus
              isConnected={wsConnected}
              reconnectAttempts={wsReconnectAttempts}
              size="small"
            />
            <Button
              startIcon={<RefreshIcon />}
              onClick={loadTemplates}
              disabled={loading}
            >
              Refresh
            </Button>
          </Box>
        </Box>

        {/* Search and Filters */}
        <Box mb={3}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} md={4}>
              <TextField
                fullWidth
                placeholder="Search templates..."
                value={filters.search}
                onChange={(e) => handleSearch(e.target.value)}
                InputProps={{
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon />
                    </InputAdornment>
                  ),
                }}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={2}>
              <TextField
                select
                fullWidth
                label="Category"
                value={filters.category}
                onChange={(e) => handleCategoryChange(e.target.value)}
              >
                <MenuItem value="">All Categories</MenuItem>
                {categories.map((cat) => (
                  <MenuItem key={cat} value={cat}>
                    {cat}
                  </MenuItem>
                ))}
              </TextField>
            </Grid>
            <Grid item xs={12} sm={6} md={2}>
              <TextField
                select
                fullWidth
                label="App Type"
                value={filters.appType}
                onChange={(e) => handleAppTypeChange(e.target.value)}
              >
                <MenuItem value="">All Types</MenuItem>
                {appTypes.map((type) => (
                  <MenuItem key={type} value={type}>
                    {type}
                  </MenuItem>
                ))}
              </TextField>
            </Grid>
            <Grid item xs={12} sm={6} md={2}>
              <TextField
                select
                fullWidth
                label="Sort By"
                value={filters.sort}
                onChange={(e) => handleSortChange(e.target.value as CatalogFilters['sort'])}
              >
                <MenuItem value="popular">Popular</MenuItem>
                <MenuItem value="rating">Highest Rated</MenuItem>
                <MenuItem value="recent">Recently Added</MenuItem>
                <MenuItem value="installs">Most Installed</MenuItem>
                <MenuItem value="views">Most Viewed</MenuItem>
              </TextField>
            </Grid>
            <Grid item xs={12} sm={6} md={2}>
              <Button
                fullWidth
                variant={filters.featured ? 'contained' : 'outlined'}
                startIcon={<FeaturedIcon />}
                onClick={handleFeaturedToggle}
                sx={{ height: 56 }}
              >
                Featured Only
              </Button>
            </Grid>
          </Grid>

          {hasActiveFilters && (
            <Box mt={2} display="flex" gap={1} alignItems="center">
              <FilterIcon fontSize="small" color="action" />
              <Typography variant="body2" color="text.secondary">
                Active filters:
              </Typography>
              {filters.search && <Chip label={`Search: "${filters.search}"`} size="small" onDelete={() => handleSearch('')} />}
              {filters.category && <Chip label={`Category: ${filters.category}`} size="small" onDelete={() => handleCategoryChange('')} />}
              {filters.appType && <Chip label={`Type: ${filters.appType}`} size="small" onDelete={() => handleAppTypeChange('')} />}
              {filters.featured && <Chip label="Featured" size="small" onDelete={handleFeaturedToggle} />}
              <Button size="small" onClick={clearFilters}>Clear All</Button>
            </Box>
          )}
        </Box>

        {/* Results */}
        {loading ? (
          <Box display="flex" justifyContent="center" py={8}>
            <CircularProgress />
          </Box>
        ) : templates.length === 0 ? (
          <Alert severity="info">
            No templates found. {hasActiveFilters ? 'Try adjusting your filters.' : 'Check back later!'}
          </Alert>
        ) : (
          <>
            <Box mb={2}>
              <Typography variant="body2" color="text.secondary">
                Showing {templates.length} templates (Page {filters.page} of {totalPages})
              </Typography>
            </Box>

            <Grid container spacing={3}>
              {templates.map((template) => (
                <Grid item xs={12} sm={6} md={4} key={template.id}>
                  <TemplateCard
                    template={template}
                    onInstall={handleInstall}
                    onViewDetails={handleViewDetails}
                    mode="catalog"
                  />
                </Grid>
              ))}
            </Grid>

            {totalPages > 1 && (
              <Box display="flex" justifyContent="center" mt={4}>
                <Pagination
                  count={totalPages}
                  page={filters.page}
                  onChange={handlePageChange}
                  color="primary"
                  size="large"
                />
              </Box>
            )}
          </>
        )}

        <TemplateDetailModal
          open={detailModalOpen}
          template={selectedTemplate}
          onClose={() => setDetailModalOpen(false)}
          onInstall={handleInstall}
        />
      </Box>
    </Layout>
  );
}

/**
 * EnhancedCatalog - Advanced template catalog with search, filtering, and sorting
 *
 * Comprehensive template discovery interface providing:
 * - Advanced search across template names and descriptions
 * - Multi-level filtering (category, app type, tags, featured)
 * - Multiple sort options (popular, rating, recent, installs, views)
 * - Paginated results with configurable page size
 * - Template detail modal with full metadata
 * - Install tracking and analytics
 * - Featured template highlighting
 * - Real-time catalog updates via WebSocket
 *
 * Features:
 * - Text search with real-time filtering
 * - Category and app type dropdown filters
 * - Featured templates toggle filter
 * - Active filter chips with individual removal
 * - "Clear All Filters" quick action
 * - Pagination with page navigation
 * - Template cards with ratings, install counts, and preview images
 * - Detailed template modal with full description and metadata
 * - One-click install with usage tracking
 * - WebSocket notifications for catalog changes (new, updated, deleted, featured)
 *
 * User workflows:
 * - Search templates by name or keyword
 * - Filter by category (e.g., "Browsers", "Development")
 * - Filter by app type (e.g., "GUI", "CLI")
 * - Toggle featured templates only
 * - Sort by popularity, rating, recency, or install count
 * - View template details before installing
 * - Install template and navigate to create session
 * - Clear filters to browse all templates
 *
 * @page
 * @route /enhanced-catalog - Advanced template catalog (default catalog route)
 * @access user - Available to all authenticated users
 *
 * @component
 *
 * @returns {JSX.Element} Enhanced template catalog with search and filters
 *
 * @example
 * // Route configuration:
 * <Route path="/enhanced-catalog" element={<EnhancedCatalog />} />
 * <Route path="/templates" element={<EnhancedCatalog />} />
 *
 * @see Catalog for basic catalog without advanced features
 * @see Sessions for managing installed templates
 * @see Repositories for managing template sources
 */
export default function EnhancedCatalog() {
  return (
    <WebSocketErrorBoundary>
      <EnhancedCatalogContent />
    </WebSocketErrorBoundary>
  );
}

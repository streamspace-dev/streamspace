import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Grid,
  TextField,
  InputAdornment,
  MenuItem,
  Alert,
  Pagination,
  Button,
  Chip,
  Link,
} from '@mui/material';
import {
  Search as SearchIcon,
  FilterList as FilterIcon,
  Refresh as RefreshIcon,
  ExtensionOff as NoPluginsIcon,
  AddCircleOutline as AddIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import PluginCard from '../components/PluginCard';
import PluginCardSkeleton from '../components/PluginCardSkeleton';
import PluginDetailModal from '../components/PluginDetailModal';
import { api, type CatalogPlugin, type PluginFilters } from '../lib/api';
import { toast } from '../lib/toast';
import { useNavigate } from 'react-router-dom';

export default function PluginCatalog() {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [plugins, setPlugins] = useState<CatalogPlugin[]>([]);
  const [selectedPlugin, setSelectedPlugin] = useState<CatalogPlugin | null>(null);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [filters, setFilters] = useState<PluginFilters>({
    search: '',
    category: '',
    pluginType: undefined,
    tag: '',
    sort: 'popular',
    page: 1,
    limit: 12,
  });
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(1);
  const [categories, setCategories] = useState<string[]>([]);
  const [pluginTypes, setPluginTypes] = useState<string[]>(['extension', 'webhook', 'api', 'ui', 'theme']);

  useEffect(() => {
    loadPlugins();
  }, [filters]);

  useEffect(() => {
    // Extract unique categories from plugins
    const uniqueCategories = Array.from(new Set(plugins.map(p => p.category).filter(Boolean)));
    setCategories(uniqueCategories);
  }, [plugins]);

  const loadPlugins = async () => {
    setLoading(true);
    try {
      const data = await api.browsePlugins(filters);
      setPlugins(data.plugins);
      setTotal(data.total);
      setTotalPages(Math.ceil(data.total / (filters.limit || 12)));
    } catch (error) {
      console.error('Failed to load plugins:', error);
      toast.error('Failed to load plugins');
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

  const handlePluginTypeChange = (pluginType: string) => {
    setFilters({ ...filters, pluginType: pluginType || undefined, page: 1 });
  };

  const handleSortChange = (sort: PluginFilters['sort']) => {
    setFilters({ ...filters, sort, page: 1 });
  };

  const handlePageChange = (_: unknown, page: number) => {
    setFilters({ ...filters, page });
    window.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const handleViewDetails = (plugin: CatalogPlugin) => {
    setSelectedPlugin(plugin);
    setDetailModalOpen(true);
  };

  const handleInstall = async (plugin: CatalogPlugin) => {
    try {
      const result = await api.installPlugin(plugin.id);
      toast.success(`${plugin.displayName} installed successfully!`);

      // Reload plugins to update install counts
      await loadPlugins();

      setDetailModalOpen(false);
    } catch (error) {
      console.error('Failed to install plugin:', error);
      toast.error('Failed to install plugin');
    }
  };

  const clearFilters = () => {
    setFilters({
      search: '',
      category: '',
      pluginType: undefined,
      tag: '',
      sort: 'popular',
      page: 1,
      limit: 12,
    });
  };

  const hasActiveFilters = filters.search || filters.category || filters.pluginType;

  return (
    <Layout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Box>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Plugin Catalog
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Extend StreamSpace with powerful plugins
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

        {/* Search and Filters */}
        <Box mb={3}>
          <Grid container spacing={2} alignItems="center">
            <Grid item xs={12} md={4}>
              <TextField
                fullWidth
                placeholder="Search plugins..."
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
                label="Plugin Type"
                value={filters.pluginType || ''}
                onChange={(e) => handlePluginTypeChange(e.target.value)}
              >
                <MenuItem value="">All Types</MenuItem>
                {pluginTypes.map((type) => (
                  <MenuItem key={type} value={type}>
                    {type.charAt(0).toUpperCase() + type.slice(1)}
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
                onChange={(e) => handleSortChange(e.target.value as PluginFilters['sort'])}
              >
                <MenuItem value="popular">Popular</MenuItem>
                <MenuItem value="rating">Highest Rated</MenuItem>
                <MenuItem value="recent">Recently Added</MenuItem>
                <MenuItem value="installs">Most Installed</MenuItem>
              </TextField>
            </Grid>
            <Grid item xs={12} sm={6} md={2}>
              <Button
                fullWidth
                variant="outlined"
                onClick={clearFilters}
                disabled={!hasActiveFilters}
                sx={{ height: 56 }}
              >
                Clear Filters
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
              {filters.pluginType && <Chip label={`Type: ${filters.pluginType}`} size="small" onDelete={() => handlePluginTypeChange('')} />}
            </Box>
          )}
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
        ) : plugins.length === 0 ? (
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
              {hasActiveFilters ? 'No Matching Plugins' : 'No Plugins Available'}
            </Typography>
            <Typography variant="body2" color="text.secondary" mb={3} maxWidth={500}>
              {hasActiveFilters
                ? 'Try adjusting your filters or search terms to find more plugins.'
                : 'No plugins are currently available in the catalog. Add a plugin repository to get started.'}
            </Typography>
            {hasActiveFilters ? (
              <Button variant="outlined" onClick={clearFilters}>
                Clear All Filters
              </Button>
            ) : (
              <Button
                variant="contained"
                startIcon={<AddIcon />}
                onClick={() => navigate('/repositories')}
              >
                Add Plugin Repository
              </Button>
            )}
          </Box>
        ) : (
          <>
            <Box mb={2}>
              <Typography variant="body2" color="text.secondary">
                Showing {plugins.length} of {total} plugins (Page {filters.page} of {totalPages})
              </Typography>
            </Box>

            <Grid container spacing={3}>
              {plugins.map((plugin) => (
                <Grid item xs={12} sm={6} md={4} key={plugin.id}>
                  <PluginCard
                    plugin={plugin}
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

        <PluginDetailModal
          open={detailModalOpen}
          plugin={selectedPlugin}
          onClose={() => setDetailModalOpen(false)}
          onInstall={handleInstall}
        />
      </Box>
    </Layout>
  );
}

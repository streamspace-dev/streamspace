import { useState, useEffect } from 'react';
import {
  Box,
  Grid,
  Card,
  CardContent,
  CardActionArea,
  Typography,
  TextField,
  InputAdornment,
  Chip,
  Avatar,
  CircularProgress,
  IconButton,
  Tooltip,
} from '@mui/material';
import {
  Search as SearchIcon,
  Star as StarIcon,
  StarBorder as StarBorderIcon,
  Apps as AppsIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import { useTemplates, useSessions } from '../hooks/useApi';
import { useUserStore } from '../store/userStore';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { toast } from '../lib/toast';
import type { Template, Session } from '../lib/api';

/**
 * OktaStyleDashboard - Okta-style app launcher with grid of application tiles
 *
 * This is the main user dashboard featuring a clean, minimalist design inspired by
 * Okta's user portal. Users see a grid of application "chicklets" (tiles) that they
 * can click to launch. The dashboard focuses solely on app launching, with all other
 * functionality (admin, plugins, repositories) moved to separate portals.
 *
 * Features:
 * - Grid layout of application tiles with icons and descriptions
 * - Search and filter applications
 * - Favorite/starred apps appear first
 * - One-click app launching (creates new session)
 * - Shows active sessions with status indicators
 * - Responsive grid layout (adapts to screen size)
 * - Clean, uncluttered interface focused on app discovery and launching
 *
 * Future enhancements:
 * - Group-based template filtering (show only apps user has access to)
 * - User-defined custom grouping and organization
 * - Drag-and-drop tile reordering
 * - Recently used apps section
 *
 * @page
 * @route / - Main user dashboard (replaces old Dashboard)
 * @access user - All authenticated users
 *
 * @component
 *
 * @returns {JSX.Element} Okta-style app launcher dashboard
 *
 * @example
 * // Route configuration:
 * <Route path="/" element={<OktaStyleDashboard />} />
 */
export default function OktaStyleDashboard() {
  const username = useUserStore((state) => state.user?.username);
  const navigate = useNavigate();

  const { data: templates = [], isLoading: templatesLoading } = useTemplates();
  const { data: sessions = [], isLoading: sessionsLoading, refetch: refetchSessions } = useSessions();

  const [searchQuery, setSearchQuery] = useState('');
  const [favorites, setFavorites] = useState<Set<string>>(new Set());
  const [launching, setLaunching] = useState<Set<string>>(new Set());

  // Load user favorites from backend
  useEffect(() => {
    const loadFavorites = async () => {
      try {
        // TODO: Implement API endpoint to fetch user's favorite templates
        // For now, use localStorage as temporary solution
        const stored = localStorage.getItem(`favorites_${username}`);
        if (stored) {
          setFavorites(new Set(JSON.parse(stored)));
        }
      } catch (error) {
        console.error('Failed to load favorites:', error);
      }
    };
    if (username) {
      loadFavorites();
    }
  }, [username]);

  // Save favorites to localStorage (temporary until backend endpoint exists)
  const toggleFavorite = (templateName: string) => {
    const newFavorites = new Set(favorites);
    if (newFavorites.has(templateName)) {
      newFavorites.delete(templateName);
    } else {
      newFavorites.add(templateName);
    }
    setFavorites(newFavorites);
    localStorage.setItem(`favorites_${username}`, JSON.stringify(Array.from(newFavorites)));
  };

  // Launch an application (create new session)
  const launchApp = async (template: Template) => {
    if (!username) {
      toast.error('User not authenticated');
      return;
    }

    // Check if user already has an active session for this template
    const existingSession = sessions.find(
      (s: Session) => s.template === template.name && (s.state === 'running' || s.state === 'hibernated')
    );

    if (existingSession) {
      // Navigate to existing session instead of creating new one
      if (existingSession.state === 'hibernated') {
        toast.info(`Waking up hibernated session for ${template.displayName}...`);
        try {
          await api.updateSession(existingSession.name, { state: 'running' });
          await refetchSessions();
        } catch (error) {
          toast.error('Failed to wake session');
        }
      }
      navigate('/sessions');
      return;
    }

    // Create new session
    setLaunching(new Set(launching).add(template.name));
    try {
      const sessionName = `${username}-${template.name}-${Date.now()}`.toLowerCase();
      await api.createSession({
        name: sessionName,
        namespace: 'streamspace',
        user: username,
        template: template.name,
        state: 'running',
        persistentHome: true,
        resources: template.defaultResources,
      });

      toast.success(`Launching ${template.displayName}...`);
      await refetchSessions();

      // Navigate to sessions page to view/access the new session
      setTimeout(() => {
        navigate('/sessions');
      }, 1000);
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Failed to launch application');
    } finally {
      const newLaunching = new Set(launching);
      newLaunching.delete(template.name);
      setLaunching(newLaunching);
    }
  };

  // Get active session for a template (if exists)
  const getActiveSession = (templateName: string): Session | undefined => {
    return sessions.find(
      (s: Session) => s.template === templateName && (s.state === 'running' || s.state === 'hibernated')
    );
  };

  // Filter templates based on search query
  const filteredTemplates = templates.filter((template: Template) => {
    const query = searchQuery.toLowerCase();
    return (
      template.displayName.toLowerCase().includes(query) ||
      template.description.toLowerCase().includes(query) ||
      template.category.toLowerCase().includes(query) ||
      template.tags?.some((tag) => tag.toLowerCase().includes(query))
    );
  });

  // Sort: favorites first, then by display name
  const sortedTemplates = [...filteredTemplates].sort((a, b) => {
    const aFav = favorites.has(a.name);
    const bFav = favorites.has(b.name);
    if (aFav && !bFav) return -1;
    if (!aFav && bFav) return 1;
    return a.displayName.localeCompare(b.displayName);
  });

  // Get category color
  const getCategoryColor = (category: string): string => {
    const colors: Record<string, string> = {
      'Web Browsers': '#3f51b5',
      'Development': '#4caf50',
      'Design': '#f50057',
      'Productivity': '#ff9800',
      'Media': '#9c27b0',
      'Games': '#e91e63',
      'Education': '#00bcd4',
    };
    return colors[category] || '#607d8b';
  };

  if (templatesLoading || sessionsLoading) {
    return (
      <Layout>
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '60vh' }}>
          <CircularProgress />
        </Box>
      </Layout>
    );
  }

  return (
    <Layout>
      <Box>
        {/* Header */}
        <Box sx={{ mb: 4 }}>
          <Typography variant="h4" sx={{ fontWeight: 700, mb: 1 }}>
            My Applications
          </Typography>
          <Typography variant="body1" color="text.secondary">
            Click on any application to launch it
          </Typography>
        </Box>

        {/* Search Bar */}
        <TextField
          fullWidth
          placeholder="Search applications..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          sx={{ mb: 4, maxWidth: 600 }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">
                <SearchIcon />
              </InputAdornment>
            ),
          }}
        />

        {/* Application Grid */}
        {sortedTemplates.length === 0 ? (
          <Box sx={{ textAlign: 'center', py: 8 }}>
            <AppsIcon sx={{ fontSize: 64, color: 'text.disabled', mb: 2 }} />
            <Typography variant="h6" color="text.secondary">
              No applications found
            </Typography>
            <Typography variant="body2" color="text.disabled">
              {searchQuery ? 'Try a different search term' : 'No applications available'}
            </Typography>
          </Box>
        ) : (
          <Grid container spacing={3}>
            {sortedTemplates.map((template: Template) => {
              const activeSession = getActiveSession(template.name);
              const isLaunching = launching.has(template.name);
              const isFavorite = favorites.has(template.name);

              return (
                <Grid item xs={12} sm={6} md={4} lg={3} key={template.name}>
                  <Card
                    sx={{
                      height: '100%',
                      position: 'relative',
                      transition: 'all 0.2s',
                      '&:hover': {
                        transform: 'translateY(-4px)',
                        boxShadow: 4,
                      },
                    }}
                  >
                    {/* Favorite Button */}
                    <IconButton
                      sx={{
                        position: 'absolute',
                        top: 8,
                        right: 8,
                        zIndex: 1,
                        bgcolor: 'background.paper',
                        '&:hover': { bgcolor: 'background.paper' },
                      }}
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleFavorite(template.name);
                      }}
                    >
                      {isFavorite ? (
                        <StarIcon sx={{ color: '#ffa726' }} />
                      ) : (
                        <StarBorderIcon />
                      )}
                    </IconButton>

                    <CardActionArea
                      onClick={() => !isLaunching && launchApp(template)}
                      disabled={isLaunching}
                      sx={{ height: '100%' }}
                    >
                      <CardContent sx={{ textAlign: 'center', pt: 5 }}>
                        {/* App Icon/Avatar */}
                        <Avatar
                          sx={{
                            width: 64,
                            height: 64,
                            margin: '0 auto 16px',
                            bgcolor: getCategoryColor(template.category),
                            fontSize: 28,
                          }}
                        >
                          {template.icon ? (
                            <img
                              src={template.icon}
                              alt={template.displayName}
                              style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                            />
                          ) : (
                            template.displayName.charAt(0).toUpperCase()
                          )}
                        </Avatar>

                        {/* App Name */}
                        <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
                          {template.displayName}
                        </Typography>

                        {/* App Description */}
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{
                            mb: 2,
                            minHeight: 40,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            display: '-webkit-box',
                            WebkitLineClamp: 2,
                            WebkitBoxOrient: 'vertical',
                          }}
                        >
                          {template.description}
                        </Typography>

                        {/* Category Badge */}
                        <Chip
                          label={template.category}
                          size="small"
                          sx={{
                            bgcolor: getCategoryColor(template.category),
                            color: 'white',
                            fontWeight: 500,
                            mb: 1,
                          }}
                        />

                        {/* Active Session Indicator */}
                        {activeSession && (
                          <Box sx={{ mt: 1 }}>
                            <Chip
                              label={activeSession.state === 'running' ? 'Active' : 'Hibernated'}
                              size="small"
                              color={activeSession.state === 'running' ? 'success' : 'warning'}
                              variant="outlined"
                            />
                          </Box>
                        )}

                        {/* Launching Indicator */}
                        {isLaunching && (
                          <Box sx={{ mt: 2 }}>
                            <CircularProgress size={24} />
                          </Box>
                        )}
                      </CardContent>
                    </CardActionArea>
                  </Card>
                </Grid>
              );
            })}
          </Grid>
        )}

        {/* TODO: Future Enhancement - Group-based filtering */}
        {/* TODO: Add user custom groups and organization */}
        {/* TODO: Add drag-and-drop tile reordering */}
      </Box>
    </Layout>
  );
}

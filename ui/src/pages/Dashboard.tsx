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
import { useUserApplications, useSessions } from '../hooks/useApi';
import { useUserStore } from '../store/userStore';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { toast } from '../lib/toast';
import type { InstalledApplication, Session } from '../lib/api';

/**
 * Dashboard - App launcher with grid of application tiles
 *
 * This is the main user dashboard featuring a clean, minimalist design.
 * Users see a grid of application tiles that they can click to launch.
 * The dashboard focuses solely on app launching, with all other
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
 * - Group-based application filtering (shows only apps user has access to)
 *
 * Future enhancements:
 * - User-defined custom grouping and organization
 * - Drag-and-drop tile reordering
 * - Recently used apps section
 *
 * @page
 * @route / - Main user dashboard
 * @access user - All authenticated users
 *
 * @component
 *
 * @returns {JSX.Element} App launcher dashboard
 *
 * @example
 * // Route configuration:
 * <Route path="/" element={<Dashboard />} />
 */
export default function Dashboard() {
  const username = useUserStore((state) => state.user?.username);
  const navigate = useNavigate();

  const { data: applications = [], isLoading: applicationsLoading } = useUserApplications();
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
  const launchApp = async (app: InstalledApplication) => {
    if (!username) {
      toast.error('User not authenticated');
      return;
    }

    const templateName = app.templateName || app.name;

    // Check if user already has an active session for this template
    const existingSession = sessions.find(
      (s: Session) => s.template === templateName && (s.state === 'running' || s.state === 'hibernated')
    );

    if (existingSession) {
      // Navigate to existing session instead of creating new one
      if (existingSession.state === 'hibernated') {
        toast.info(`Waking up hibernated session for ${app.displayName}...`);
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
    setLaunching(new Set(launching).add(app.id));
    try {
      const sessionName = `${username}-${templateName}-${Date.now()}`.toLowerCase();
      await api.createSession({
        name: sessionName,
        namespace: 'streamspace',
        user: username,
        template: templateName,
        state: 'running',
        persistentHome: true,
      });

      toast.success(`Launching ${app.displayName}...`);
      await refetchSessions();

      // Navigate to sessions page to view/access the new session
      setTimeout(() => {
        navigate('/sessions');
      }, 1000);
    } catch (error: any) {
      toast.error(error.response?.data?.message || 'Failed to launch application');
    } finally {
      const newLaunching = new Set(launching);
      newLaunching.delete(app.id);
      setLaunching(newLaunching);
    }
  };

  // Get active session for a template (if exists)
  const getActiveSession = (templateName: string): Session | undefined => {
    return sessions.find(
      (s: Session) => s.template === templateName && (s.state === 'running' || s.state === 'hibernated')
    );
  };

  // Filter applications based on search query
  const filteredApplications = applications.filter((app: InstalledApplication) => {
    const query = searchQuery.toLowerCase();
    return (
      app.displayName.toLowerCase().includes(query) ||
      (app.description || '').toLowerCase().includes(query) ||
      (app.category || '').toLowerCase().includes(query)
    );
  });

  // Sort: favorites first, then by display name
  const sortedApplications = [...filteredApplications].sort((a, b) => {
    const aFav = favorites.has(a.id);
    const bFav = favorites.has(b.id);
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

  if (applicationsLoading || sessionsLoading) {
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
        {sortedApplications.length === 0 ? (
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
            {sortedApplications.map((app: InstalledApplication) => {
              const templateName = app.templateName || app.name;
              const activeSession = getActiveSession(templateName);
              const isLaunching = launching.has(app.id);
              const isFavorite = favorites.has(app.id);

              return (
                <Grid item xs={12} sm={6} md={4} lg={3} key={app.id}>
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
                        toggleFavorite(app.id);
                      }}
                    >
                      {isFavorite ? (
                        <StarIcon sx={{ color: '#ffa726' }} />
                      ) : (
                        <StarBorderIcon />
                      )}
                    </IconButton>

                    <CardActionArea
                      onClick={() => !isLaunching && launchApp(app)}
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
                            bgcolor: getCategoryColor(app.category || 'Other'),
                            fontSize: 28,
                          }}
                        >
                          {app.icon ? (
                            <img
                              src={app.icon}
                              alt={app.displayName}
                              style={{ width: '100%', height: '100%', objectFit: 'cover' }}
                            />
                          ) : (
                            app.displayName.charAt(0).toUpperCase()
                          )}
                        </Avatar>

                        {/* App Name */}
                        <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
                          {app.displayName}
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
                          {app.description || 'No description'}
                        </Typography>

                        {/* Category Badge */}
                        <Chip
                          label={app.category || 'Other'}
                          size="small"
                          sx={{
                            bgcolor: getCategoryColor(app.category || 'Other'),
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

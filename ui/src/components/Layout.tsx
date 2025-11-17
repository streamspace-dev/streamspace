import { useState, ReactNode, memo } from 'react';
import {
  Box,
  Drawer,
  AppBar,
  Toolbar,
  List,
  Typography,
  Divider,
  IconButton,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Avatar,
  Menu,
  MenuItem,
} from '@mui/material';
import {
  Menu as MenuIcon,
  Dashboard as DashboardIcon,
  Computer as ComputerIcon,
  Share as ShareIcon,
  Apps as AppsIcon,
  Folder as FolderIcon,
  Extension as ExtensionIcon,
  Settings as SettingsIcon,
  Logout as LogoutIcon,
  AdminPanelSettings as AdminIcon,
  Storage as StorageIcon,
  People as PeopleIcon,
  Groups as GroupsIcon,
  Schedule as ScheduleIcon,
  Security as SecurityIcon,
  Hub as IntegrationIcon,
  TrendingUp as ScalingIcon,
  Policy as ComplianceIcon,
} from '@mui/icons-material';
import { useNavigate, useLocation } from 'react-router-dom';
import { useUserStore } from '../store/userStore';

const drawerWidth = 240;

interface LayoutProps {
  children: ReactNode;
}

/**
 * Layout - Main application layout with navigation sidebar and app bar
 *
 * Provides the consistent layout structure for the entire StreamSpace application,
 * including a responsive navigation drawer with user and admin menu items, a top
 * app bar with user profile menu, and the main content area. Handles mobile
 * responsive behavior with collapsible drawer.
 *
 * Features:
 * - Responsive navigation drawer (permanent on desktop, temporary on mobile)
 * - User-specific and admin menu items with role-based access
 * - User profile avatar and logout menu
 * - Active route highlighting
 * - Material-UI theming integration
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {ReactNode} props.children - Child components to render in main content area
 *
 * @returns {JSX.Element} Rendered layout with navigation and content
 *
 * @example
 * <Layout>
 *   <Dashboard />
 * </Layout>
 *
 * @see useUserStore for user authentication state
 * @see useNavigate for route navigation
 */
function Layout({ children }: LayoutProps) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const { user, clearAuth } = useUserStore();
  const username = user?.username;
  const role = user?.role;

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const handleLogout = () => {
    clearAuth();
    navigate('/login');
  };

  const menuItems = [
    { text: 'Dashboard', icon: <DashboardIcon />, path: '/' },
    { text: 'My Sessions', icon: <ComputerIcon />, path: '/sessions' },
    { text: 'Shared with Me', icon: <ShareIcon />, path: '/shared-sessions' },
    { text: 'Template Catalog', icon: <AppsIcon />, path: '/catalog' },
    { text: 'Plugin Catalog', icon: <ExtensionIcon />, path: '/plugins/catalog' },
    { text: 'My Plugins', icon: <ExtensionIcon />, path: '/plugins/installed' },
    { text: 'Repositories', icon: <FolderIcon />, path: '/repositories' },
    { text: 'Scheduling', icon: <ScheduleIcon />, path: '/scheduling' },
    { text: 'Security', icon: <SecurityIcon />, path: '/security' },
  ];

  const adminMenuItems = [
    { text: 'Admin Dashboard', icon: <AdminIcon />, path: '/admin/dashboard' },
    { text: 'Users', icon: <PeopleIcon />, path: '/admin/users' },
    { text: 'Groups', icon: <GroupsIcon />, path: '/admin/groups' },
    { text: 'Cluster Nodes', icon: <StorageIcon />, path: '/admin/nodes' },
    { text: 'User Quotas', icon: <PeopleIcon />, path: '/admin/quotas' },
    { text: 'Plugin Management', icon: <ExtensionIcon />, path: '/admin/plugins' },
    { text: 'Integrations', icon: <IntegrationIcon />, path: '/admin/integrations' },
    { text: 'Scaling', icon: <ScalingIcon />, path: '/admin/scaling' },
    { text: 'Compliance', icon: <ComplianceIcon />, path: '/admin/compliance' },
  ];

  const drawer = (
    <div>
      <Toolbar>
        <Typography variant="h6" noWrap component="div" sx={{ fontWeight: 700 }}>
          StreamSpace
        </Typography>
      </Toolbar>
      <Divider />
      <List>
        {menuItems.map((item) => (
          <ListItem key={item.text} disablePadding>
            <ListItemButton
              selected={location.pathname === item.path}
              onClick={() => navigate(item.path)}
            >
              <ListItemIcon>{item.icon}</ListItemIcon>
              <ListItemText primary={item.text} />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
      {role === 'admin' && (
        <>
          <Divider />
          <Typography variant="caption" sx={{ px: 2, py: 1, color: 'text.secondary' }}>
            ADMIN
          </Typography>
          <List>
            {adminMenuItems.map((item) => (
              <ListItem key={item.text} disablePadding>
                <ListItemButton
                  selected={location.pathname === item.path}
                  onClick={() => navigate(item.path)}
                >
                  <ListItemIcon>{item.icon}</ListItemIcon>
                  <ListItemText primary={item.text} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
        </>
      )}
      <Divider />
      <List>
        <ListItem disablePadding>
          <ListItemButton>
            <ListItemIcon>
              <SettingsIcon />
            </ListItemIcon>
            <ListItemText primary="Settings" />
          </ListItemButton>
        </ListItem>
      </List>
    </div>
  );

  return (
    <Box sx={{ display: 'flex' }}>
      <AppBar
        position="fixed"
        sx={{
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          ml: { sm: `${drawerWidth}px` },
        }}
      >
        <Toolbar>
          <IconButton
            color="inherit"
            aria-label="open drawer"
            edge="start"
            onClick={handleDrawerToggle}
            sx={{ mr: 2, display: { sm: 'none' } }}
          >
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            {menuItems.find((item) => item.path === location.pathname)?.text || 'StreamSpace'}
          </Typography>
          <IconButton onClick={handleMenuOpen} sx={{ p: 0 }}>
            <Avatar sx={{ bgcolor: 'secondary.main' }}>
              {username?.charAt(0).toUpperCase() || 'U'}
            </Avatar>
          </IconButton>
          <Menu
            anchorEl={anchorEl}
            open={Boolean(anchorEl)}
            onClose={handleMenuClose}
            anchorOrigin={{
              vertical: 'bottom',
              horizontal: 'right',
            }}
          >
            <MenuItem disabled>
              <Typography variant="body2">{username}</Typography>
            </MenuItem>
            <Divider />
            <MenuItem onClick={handleLogout}>
              <ListItemIcon>
                <LogoutIcon fontSize="small" />
              </ListItemIcon>
              Logout
            </MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>
      <Box
        component="nav"
        sx={{ width: { sm: drawerWidth }, flexShrink: { sm: 0 } }}
        aria-label="navigation"
      >
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={handleDrawerToggle}
          ModalProps={{
            keepMounted: true, // Better open performance on mobile.
          }}
          sx={{
            display: { xs: 'block', sm: 'none' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
          }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          sx={{
            display: { xs: 'none', sm: 'block' },
            '& .MuiDrawer-paper': { boxSizing: 'border-box', width: drawerWidth },
          }}
          open
        >
          {drawer}
        </Drawer>
      </Box>
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          p: 3,
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          minHeight: '100vh',
          bgcolor: 'background.default',
        }}
      >
        <Toolbar />
        {children}
      </Box>
    </Box>
  );
}

// Export memoized version to prevent unnecessary re-renders
// Only re-render when location changes, not when parent re-renders
export default memo(Layout);

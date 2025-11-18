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
  Chip,
} from '@mui/material';
import {
  Menu as MenuIcon,
  AdminPanelSettings as AdminIcon,
  People as PeopleIcon,
  Groups as GroupsIcon,
  Storage as StorageIcon,
  Extension as ExtensionIcon,
  Hub as IntegrationIcon,
  TrendingUp as ScalingIcon,
  Policy as ComplianceIcon,
  Apps as AppsIcon,
  Folder as FolderIcon,
  Logout as LogoutIcon,
  Dashboard as DashboardIcon,
  Schedule as ScheduleIcon,
  Security as SecurityIcon,
} from '@mui/icons-material';
import { useNavigate, useLocation } from 'react-router-dom';
import { useUserStore } from '../store/userStore';

const drawerWidth = 260;

interface AdminPortalLayoutProps {
  children: ReactNode;
}

/**
 * AdminPortalLayout - Dedicated layout for admin portal
 *
 * This layout is specifically designed for the admin portal, which opens in a
 * separate window. It provides navigation for all administrative functions including
 * user/group management, plugin management, repository management, and system
 * configuration.
 *
 * Features:
 * - Dedicated admin navigation without user features
 * - Clean, focused interface for administrative tasks
 * - Can be opened in separate window for multi-tasking
 * - Contains plugin catalog, repository management, and template management
 * - System administration and configuration options
 *
 * Navigation Sections:
 * - Overview: Admin dashboard
 * - Content Management: Templates, Plugins, Repositories
 * - User Management: Users, Groups, Quotas
 * - System: Nodes, Integrations, Scaling, Compliance
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {ReactNode} props.children - Child components to render in main content area
 *
 * @returns {JSX.Element} Admin portal layout with navigation and content
 *
 * @example
 * <AdminPortalLayout>
 *   <AdminDashboard />
 * </AdminPortalLayout>
 */
function AdminPortalLayout({ children }: AdminPortalLayoutProps) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const { user, clearAuth } = useUserStore();
  const username = user?.username;

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

  const handleBackToMain = () => {
    // If opened in new window, close it and focus main window
    if (window.opener) {
      window.opener.focus();
      window.close();
    } else {
      // If not in separate window, navigate to user dashboard
      navigate('/');
    }
  };

  const menuSections = [
    {
      title: 'Overview',
      items: [
        { text: 'Admin Dashboard', icon: <DashboardIcon />, path: '/admin/dashboard' },
      ],
    },
    {
      title: 'Content Management',
      items: [
        { text: 'Template Catalog', icon: <AppsIcon />, path: '/admin/templates' },
        { text: 'Plugin Management', icon: <ExtensionIcon />, path: '/admin/plugins' },
        { text: 'Repositories', icon: <FolderIcon />, path: '/admin/repositories' },
      ],
    },
    {
      title: 'User Management',
      items: [
        { text: 'Users', icon: <PeopleIcon />, path: '/admin/users' },
        { text: 'Groups', icon: <GroupsIcon />, path: '/admin/groups' },
        { text: 'User Quotas', icon: <PeopleIcon />, path: '/admin/quotas' },
      ],
    },
    {
      title: 'System',
      items: [
        { text: 'Cluster Nodes', icon: <StorageIcon />, path: '/admin/nodes' },
        { text: 'Integrations', icon: <IntegrationIcon />, path: '/admin/integrations' },
        { text: 'Scaling', icon: <ScalingIcon />, path: '/admin/scaling' },
        { text: 'Scheduling', icon: <ScheduleIcon />, path: '/admin/scheduling' },
        { text: 'Security Settings', icon: <SecurityIcon />, path: '/admin/security' },
        { text: 'Compliance', icon: <ComplianceIcon />, path: '/admin/compliance' },
      ],
    },
  ];

  const drawer = (
    <div>
      <Toolbar>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <AdminIcon sx={{ color: 'primary.main' }} />
          <Typography variant="h6" noWrap component="div" sx={{ fontWeight: 700 }}>
            Admin Portal
          </Typography>
        </Box>
      </Toolbar>
      <Divider />

      {menuSections.map((section, idx) => (
        <Box key={section.title}>
          <Typography
            variant="caption"
            sx={{
              px: 2,
              py: 1,
              display: 'block',
              color: 'text.secondary',
              fontWeight: 600,
              textTransform: 'uppercase',
              letterSpacing: 0.5,
            }}
          >
            {section.title}
          </Typography>
          <List dense>
            {section.items.map((item) => (
              <ListItem key={item.text} disablePadding>
                <ListItemButton
                  selected={location.pathname === item.path}
                  onClick={() => navigate(item.path)}
                >
                  <ListItemIcon sx={{ minWidth: 40 }}>{item.icon}</ListItemIcon>
                  <ListItemText primary={item.text} />
                </ListItemButton>
              </ListItem>
            ))}
          </List>
          {idx < menuSections.length - 1 && <Divider sx={{ my: 1 }} />}
        </Box>
      ))}
    </div>
  );

  // Get current page title
  const getCurrentPageTitle = () => {
    for (const section of menuSections) {
      const item = section.items.find((i) => i.path === location.pathname);
      if (item) return item.text;
    }
    return 'Admin Portal';
  };

  return (
    <Box sx={{ display: 'flex' }}>
      <AppBar
        position="fixed"
        sx={{
          width: { sm: `calc(100% - ${drawerWidth}px)` },
          ml: { sm: `${drawerWidth}px` },
          bgcolor: 'primary.dark',
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
            {getCurrentPageTitle()}
          </Typography>

          {/* Admin Badge */}
          <Chip
            icon={<AdminIcon />}
            label="Administrator"
            size="small"
            sx={{
              bgcolor: 'rgba(255, 255, 255, 0.2)',
              color: 'white',
              fontWeight: 600,
              mr: 2,
            }}
          />

          {/* User Menu */}
          <IconButton onClick={handleMenuOpen} sx={{ p: 0 }}>
            <Avatar sx={{ bgcolor: 'secondary.main' }}>
              {username?.charAt(0).toUpperCase() || 'A'}
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
            <MenuItem onClick={handleBackToMain}>
              Back to Main Dashboard
            </MenuItem>
            <MenuItem onClick={handleLogout}>
              <ListItemIcon>
                <LogoutIcon fontSize="small" />
              </ListItemIcon>
              Logout
            </MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>

      {/* Navigation Drawer */}
      <Box
        component="nav"
        sx={{ width: { sm: drawerWidth }, flexShrink: { sm: 0 } }}
        aria-label="admin navigation"
      >
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={handleDrawerToggle}
          ModalProps={{
            keepMounted: true,
          }}
          sx={{
            display: { xs: 'block', sm: 'none' },
            '& .MuiDrawer-paper': {
              boxSizing: 'border-box',
              width: drawerWidth,
              borderRight: '2px solid',
              borderColor: 'primary.main',
            },
          }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          sx={{
            display: { xs: 'none', sm: 'block' },
            '& .MuiDrawer-paper': {
              boxSizing: 'border-box',
              width: drawerWidth,
              borderRight: '2px solid',
              borderColor: 'primary.main',
            },
          }}
          open
        >
          {drawer}
        </Drawer>
      </Box>

      {/* Main Content */}
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

export default memo(AdminPortalLayout);

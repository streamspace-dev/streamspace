import { Box, Typography, Alert, Button } from '@mui/material';
import { Extension as PluginIcon, ShoppingCart as CatalogIcon } from '@mui/icons-material';
import AdminPortalLayout from '../../components/AdminPortalLayout';
import { useNavigate } from 'react-router-dom';

/**
 * PluginAdministration - System-wide plugin administration page
 *
 * BUG FIX P1-4: Placeholder page for Plugin Administration feature
 *
 * This page will provide system-wide plugin management in v2.1:
 * - Global plugin enable/disable
 * - System-wide plugin settings
 * - Plugin dependency management
 * - Plugin update policies
 * - Plugin security settings
 *
 * For v2.0-beta.1, this is a placeholder directing users to the Plugin Catalog
 * for individual plugin management.
 *
 * @page
 * @route /admin/plugin-administration
 * @access admin
 *
 * @component
 * @returns {JSX.Element} Plugin Administration placeholder page
 */
export default function PluginAdministration() {
  const navigate = useNavigate();

  return (
    <AdminPortalLayout>
      <Box sx={{ p: 3 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3 }}>
          <PluginIcon sx={{ fontSize: 40, color: 'primary.main' }} />
          <Typography variant="h4">
            Plugin Administration
          </Typography>
        </Box>

        <Alert severity="info" sx={{ mb: 3 }}>
          <Typography variant="h6" gutterBottom>
            Coming in v2.1
          </Typography>
          <Typography variant="body2" paragraph>
            System-wide plugin administration features are planned for the v2.1 release.
            This will include global plugin management, security policies, and update controls.
          </Typography>
          <Typography variant="body2">
            For now, you can manage individual plugins through the Plugin Catalog.
          </Typography>
        </Alert>

        <Box sx={{ display: 'flex', gap: 2 }}>
          <Button
            variant="contained"
            startIcon={<CatalogIcon />}
            onClick={() => navigate('/admin/plugin-catalog')}
          >
            Go to Plugin Catalog
          </Button>
          <Button
            variant="outlined"
            startIcon={<PluginIcon />}
            onClick={() => navigate('/admin/installed-plugins')}
          >
            View Installed Plugins
          </Button>
        </Box>

        <Box sx={{ mt: 4 }}>
          <Typography variant="h6" gutterBottom>
            Planned Features for v2.1
          </Typography>
          <Typography variant="body2" component="ul" sx={{ pl: 2 }}>
            <li>Global plugin enable/disable for all users</li>
            <li>System-wide plugin configuration defaults</li>
            <li>Plugin dependency and conflict resolution</li>
            <li>Automated plugin update policies</li>
            <li>Plugin security scanning and approval workflows</li>
            <li>Plugin resource limits and quotas</li>
            <li>Plugin usage analytics and reporting</li>
          </Typography>
        </Box>
      </Box>
    </AdminPortalLayout>
  );
}

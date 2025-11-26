import { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Container,
  FormControl,
  FormControlLabel,
  Grid,
  InputLabel,
  MenuItem,
  Select,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography,
  Alert,
  CircularProgress,
  FormHelperText,
} from '@mui/material';
import {
  Save as SaveIcon,
  Refresh as RefreshIcon,
  Download as DownloadIcon,
} from '@mui/icons-material';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';

/**
 * Configuration setting structure from API
 */
interface Configuration {
  key: string;
  value: string;
  type: string; // string, boolean, number, duration, enum, array, url, email
  category: string;
  description: string;
  updated_at: string;
  updated_by?: string;
}

/**
 * API response structure for configurations
 */
interface ConfigurationListResponse {
  configurations: Configuration[];
  grouped: Record<string, Configuration[]>;
}

/**
 * Settings - System configuration management for administrators
 *
 * Administrative interface for managing platform-wide configuration settings.
 * Provides category-based organization and type-aware validation for all
 * platform settings.
 *
 * Features:
 * - Category-based tabs (7 categories)
 * - Type-aware form fields with validation
 * - Bulk update support
 * - Export configuration to JSON
 * - Real-time validation
 * - Audit trail of changes
 *
 * Configuration categories:
 * 1. Ingress: Domain, TLS settings
 * 2. Storage: Storage class, sizes, allowed classes
 * 3. Resources: CPU/memory limits and defaults
 * 4. Features: Feature toggles (metrics, hibernation, recordings)
 * 5. Session: Idle timeout, max duration, allowed images
 * 6. Security: MFA, SAML, OIDC, IP whitelist
 * 7. Compliance: Frameworks, retention, archiving
 *
 * @page
 * @route /admin/settings - System configuration
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Configuration management interface
 */
export default function Settings() {
  const { addNotification } = useNotificationQueue();
  const queryClient = useQueryClient();

  const [activeTab, setActiveTab] = useState(0);
  const [editedValues, setEditedValues] = useState<Record<string, string>>({});
  const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});

  // Category names matching backend
  const categories = [
    'ingress',
    'storage',
    'resources',
    'features',
    'session',
    'security',
    'compliance',
  ];

  const categoryLabels: Record<string, string> = {
    ingress: 'Ingress',
    storage: 'Storage',
    resources: 'Resources',
    features: 'Features',
    session: 'Session',
    security: 'Security',
    compliance: 'Compliance',
  };

  // Fetch all configurations
  const { data, isLoading, refetch } = useQuery<ConfigurationListResponse>({
    queryKey: ['configurations'],
    queryFn: async () => {
      const response = await fetch('/api/v1/admin/config', {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch configurations');
      }

      return response.json();
    },
  });

  const configurations = data?.configurations || [];
  const grouped = data?.grouped || {};

  // Update configuration mutation
  const updateMutation = useMutation({
    mutationFn: async ({ key, value }: { key: string; value: string }) => {
      const response = await fetch(`/api/v1/admin/config/${key}`, {
        method: 'PUT',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ value }),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || 'Failed to update configuration');
      }

      return response.json();
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['configurations'] });
      // Remove from edited values
      setEditedValues((prev) => {
        const newValues = { ...prev };
        delete newValues[variables.key];
        return newValues;
      });
      addNotification({
        message: `Configuration "${variables.key}" updated successfully`,
        severity: 'success',
        priority: 'low',
        title: 'Configuration Updated',
      });
    },
    onError: (error: Error, variables) => {
      setValidationErrors((prev) => ({
        ...prev,
        [variables.key]: error.message,
      }));
      addNotification({
        message: `Failed to update "${variables.key}": ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Update Failed',
      });
    },
  });

  // Bulk update mutation
  const bulkUpdateMutation = useMutation({
    mutationFn: async (updates: Record<string, string>) => {
      const response = await fetch('/api/v1/admin/config/bulk', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ updates }),
      });

      if (!response.ok) {
        throw new Error('Failed to update configurations');
      }

      return response.json();
    },
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: ['configurations'] });
      setEditedValues({});
      setValidationErrors({});
      addNotification({
        message: `Updated ${result.updated.length} configuration(s)`,
        severity: 'success',
        priority: 'medium',
        title: 'Bulk Update Complete',
      });
    },
    onError: (error: Error) => {
      addNotification({
        message: `Bulk update failed: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Update Failed',
      });
    },
  });

  // Handle value change
  const handleValueChange = (key: string, value: string) => {
    setEditedValues((prev) => ({
      ...prev,
      [key]: value,
    }));
    // Clear validation error when user starts typing
    if (validationErrors[key]) {
      setValidationErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[key];
        return newErrors;
      });
    }
  };

  // Save single configuration
  const handleSave = (key: string) => {
    const value = editedValues[key];
    if (value !== undefined) {
      updateMutation.mutate({ key, value });
    }
  };

  // Save all changes
  const handleSaveAll = () => {
    if (Object.keys(editedValues).length > 0) {
      bulkUpdateMutation.mutate(editedValues);
    }
  };

  // Export configuration
  const handleExport = () => {
    const configData = configurations.reduce((acc, config) => {
      acc[config.key] = config.value;
      return acc;
    }, {} as Record<string, string>);

    const blob = new Blob([JSON.stringify(configData, null, 2)], {
      type: 'application/json',
    });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `streamspace-config-${new Date().toISOString().split('T')[0]}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);

    addNotification({
      message: 'Configuration exported successfully',
      severity: 'success',
      priority: 'low',
      title: 'Export Complete',
    });
  };

  // Render form field based on type
  const renderField = (config: Configuration) => {
    const currentValue = editedValues[config.key] !== undefined
      ? editedValues[config.key]
      : config.value;
    const hasError = !!validationErrors[config.key];
    const isModified = editedValues[config.key] !== undefined;

    switch (config.type) {
      case 'boolean':
        return (
          <Box>
            <FormControlLabel
              control={
                <Switch
                  checked={currentValue === 'true'}
                  onChange={(e) => handleValueChange(config.key, e.target.checked ? 'true' : 'false')}
                />
              }
              label={currentValue === 'true' ? 'Enabled' : 'Disabled'}
            />
            <FormHelperText>{config.description}</FormHelperText>
          </Box>
        );

      case 'number':
        return (
          <TextField
            fullWidth
            type="number"
            value={currentValue}
            onChange={(e) => handleValueChange(config.key, e.target.value)}
            error={hasError}
            helperText={hasError ? validationErrors[config.key] : config.description}
            InputProps={{
              sx: isModified ? { backgroundColor: 'action.hover' } : {},
            }}
          />
        );

      case 'enum':
        // For enums, we'd need allowed values from backend
        // Simplified here as text field
        return (
          <TextField
            fullWidth
            value={currentValue}
            onChange={(e) => handleValueChange(config.key, e.target.value)}
            error={hasError}
            helperText={hasError ? validationErrors[config.key] : config.description}
            InputProps={{
              sx: isModified ? { backgroundColor: 'action.hover' } : {},
            }}
          />
        );

      default:
        // string, duration, array, url, email
        return (
          <TextField
            fullWidth
            value={currentValue}
            onChange={(e) => handleValueChange(config.key, e.target.value)}
            error={hasError}
            helperText={hasError ? validationErrors[config.key] : config.description}
            placeholder={config.type === 'duration' ? '30m, 1h, 24h' : ''}
            InputProps={{
              sx: isModified ? { backgroundColor: 'action.hover' } : {},
            }}
          />
        );
    }
  };

  if (isLoading) {
    return (
      <AdminPortalLayout title="Settings">
        <Container maxWidth="lg">
          <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: 400 }}>
            <CircularProgress />
          </Box>
        </Container>
      </AdminPortalLayout>
    );
  }

  const currentCategory = categories[activeTab];
  const categoryConfigs = grouped[currentCategory] || [];

  return (
    <AdminPortalLayout title="Settings">
      <Container maxWidth="lg">
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              System Configuration
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Manage platform-wide settings - {configurations.length} total settings
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            {Object.keys(editedValues).length > 0 && (
              <Button
                variant="contained"
                startIcon={<SaveIcon />}
                onClick={handleSaveAll}
                disabled={bulkUpdateMutation.isPending}
              >
                Save All ({Object.keys(editedValues).length})
              </Button>
            )}
            <Button
              variant="outlined"
              startIcon={<DownloadIcon />}
              onClick={handleExport}
            >
              Export
            </Button>
            <Button
              variant="outlined"
              startIcon={<RefreshIcon />}
              onClick={() => refetch()}
            >
              Refresh
            </Button>
          </Box>
        </Box>

        {/* Modified settings alert */}
        {Object.keys(editedValues).length > 0 && (
          <Alert severity="info" sx={{ mb: 2 }}>
            You have {Object.keys(editedValues).length} unsaved change(s). Click "Save All" to apply them.
          </Alert>
        )}

        {/* Category tabs */}
        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}>
          <Tabs value={activeTab} onChange={(_, newValue) => setActiveTab(newValue)}>
            {categories.map((category) => (
              <Tab
                key={category}
                label={categoryLabels[category]}
                disabled={!grouped[category] || grouped[category].length === 0}
              />
            ))}
          </Tabs>
        </Box>

        {/* Configuration fields */}
        {categoryConfigs.length === 0 ? (
          <Card>
            <CardContent>
              <Typography color="text.secondary" align="center">
                No configuration settings in this category
              </Typography>
            </CardContent>
          </Card>
        ) : (
          <Grid container spacing={3}>
            {categoryConfigs.map((config) => (
              <Grid item xs={12} key={config.key}>
                <Card>
                  <CardContent>
                    <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 2 }}>
                      <Box sx={{ flex: 1 }}>
                        <Typography variant="h6" gutterBottom>
                          {config.key}
                        </Typography>
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 2 }}>
                          Type: {config.type} | Last updated: {new Date(config.updated_at).toLocaleString()}
                          {config.updated_by && ` by ${config.updated_by}`}
                        </Typography>
                        {renderField(config)}
                      </Box>
                      {editedValues[config.key] !== undefined && (
                        <Button
                          variant="contained"
                          size="small"
                          onClick={() => handleSave(config.key)}
                          disabled={updateMutation.isPending}
                          sx={{ ml: 2 }}
                        >
                          Save
                        </Button>
                      )}
                    </Box>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}
      </Container>
    </AdminPortalLayout>
  );
}

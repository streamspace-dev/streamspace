import { useState } from 'react';
import {
  Box,
  Typography,
  Grid,
  Card,
  CardContent,
  CardActions,
  Button,
  Chip,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  CircularProgress,
  Alert,
  Tabs,
  Tab,
} from '@mui/material';
import { Add as AddIcon } from '@mui/icons-material';
import Layout from '../components/Layout';
import { useTemplates, useCatalogTemplates, useCreateSession } from '../hooks/useApi';
import { useUserStore } from '../store/userStore';
import { useTemplateEvents } from '../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';
import { useQueryClient } from '@tanstack/react-query';

function CatalogContent() {
  const username = useUserStore((state) => state.user?.username);
  const [tabValue, setTabValue] = useState(0);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);
  const { data: installedTemplates = [], isLoading: installedLoading } = useTemplates();
  const { data: catalogResponse, isLoading: catalogLoading } = useCatalogTemplates();
  const catalogTemplates = catalogResponse?.templates || [];
  const createSession = useCreateSession();
  const queryClient = useQueryClient();

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
      // Refresh template lists
      queryClient.invalidateQueries({ queryKey: ['templates'] });
      queryClient.invalidateQueries({ queryKey: ['catalogTemplates'] });
    } else if (data.event_type === 'template.updated') {
      addNotification({
        message: `Template updated: ${data.template_name || 'Unknown'}`,
        severity: 'info',
        priority: 'low',
        title: 'Template Updated',
      });
      // Refresh template lists
      queryClient.invalidateQueries({ queryKey: ['templates'] });
      queryClient.invalidateQueries({ queryKey: ['catalogTemplates'] });
    } else if (data.event_type === 'template.deleted') {
      addNotification({
        message: `Template removed: ${data.template_name || 'Unknown'}`,
        severity: 'warning',
        priority: 'medium',
        title: 'Template Removed',
      });
      // Refresh template lists
      queryClient.invalidateQueries({ queryKey: ['templates'] });
      queryClient.invalidateQueries({ queryKey: ['catalogTemplates'] });
    }
  });

  const handleCreateSession = () => {
    if (!selectedTemplate || !username) return;

    createSession.mutate(
      {
        user: username,
        template: selectedTemplate,
        persistentHome: true,
      },
      {
        onSuccess: () => {
          setCreateDialogOpen(false);
          setSelectedTemplate(null);
        },
      }
    );
  };

  const templates = tabValue === 0 ? installedTemplates : catalogTemplates;
  const isLoading = tabValue === 0 ? installedLoading : catalogLoading;

  return (
    <Layout>
      <Box>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={3}>
          <Typography variant="h4" sx={{ fontWeight: 700 }}>
            Template Catalog
          </Typography>
          <EnhancedWebSocketStatus
            isConnected={wsConnected}
            reconnectAttempts={wsReconnectAttempts}
            size="small"
          />
        </Box>

        <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}>
          <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)}>
            <Tab label="Installed Templates" />
            <Tab label="Marketplace" />
          </Tabs>
        </Box>

        {isLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
            <CircularProgress />
          </Box>
        ) : templates.length === 0 ? (
          <Alert severity="info">
            {tabValue === 0
              ? 'No templates installed yet. Check the Marketplace or add a Repository!'
              : 'No templates available in catalog. Add repositories in the Repositories page.'}
          </Alert>
        ) : (
          <Grid container spacing={3}>
            {templates.map((template: any) => (
              <Grid item xs={12} sm={6} md={4} key={template.name || template.id}>
                <Card>
                  <CardContent>
                    <Typography variant="h6" sx={{ fontWeight: 600, mb: 1 }}>
                      {template.displayName}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{ mb: 2, minHeight: 40 }}>
                      {template.description || 'No description available'}
                    </Typography>
                    <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap', mb: 1 }}>
                      {template.category && (
                        <Chip label={template.category} size="small" variant="outlined" />
                      )}
                      {template.appType && (
                        <Chip label={template.appType} size="small" color="primary" variant="outlined" />
                      )}
                    </Box>
                    {template.tags && template.tags.length > 0 && (
                      <Box sx={{ display: 'flex', gap: 0.5, flexWrap: 'wrap' }}>
                        {template.tags.slice(0, 3).map((tag: string) => (
                          <Chip key={tag} label={tag} size="small" />
                        ))}
                      </Box>
                    )}
                  </CardContent>
                  <CardActions>
                    {tabValue === 0 ? (
                      <Button
                        size="small"
                        variant="contained"
                        startIcon={<AddIcon />}
                        onClick={() => {
                          setSelectedTemplate(template.name);
                          setCreateDialogOpen(true);
                        }}
                      >
                        Create Session
                      </Button>
                    ) : (
                      <Button size="small" variant="outlined">
                        Install
                      </Button>
                    )}
                  </CardActions>
                </Card>
              </Grid>
            ))}
          </Grid>
        )}

        <Dialog open={createDialogOpen} onClose={() => setCreateDialogOpen(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Create New Session</DialogTitle>
          <DialogContent>
            <TextField
              fullWidth
              label="Template"
              value={selectedTemplate || ''}
              disabled
              sx={{ mt: 2, mb: 2 }}
            />
            <Alert severity="info" sx={{ mb: 2 }}>
              A new session will be created with default settings. You can customize resources after creation.
            </Alert>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setCreateDialogOpen(false)}>Cancel</Button>
            <Button
              onClick={handleCreateSession}
              variant="contained"
              disabled={createSession.isPending}
            >
              {createSession.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogActions>
        </Dialog>
      </Box>
    </Layout>
  );
}

export default function Catalog() {
  return (
    <WebSocketErrorBoundary>
      <CatalogContent />
    </WebSocketErrorBoundary>
  );
}

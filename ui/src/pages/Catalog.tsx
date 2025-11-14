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

export default function Catalog() {
  const username = useUserStore((state) => state.username);
  const [tabValue, setTabValue] = useState(0);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<string | null>(null);
  const { data: installedTemplates = [], isLoading: installedLoading } = useTemplates();
  const { data: catalogTemplates = [], isLoading: catalogLoading } = useCatalogTemplates();
  const createSession = useCreateSession();

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
        <Typography variant="h4" sx={{ mb: 3, fontWeight: 700 }}>
          Template Catalog
        </Typography>

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

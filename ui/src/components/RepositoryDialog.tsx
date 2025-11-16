import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  TextField,
  MenuItem,
  Alert,
  Box,
  Typography,
  IconButton,
  InputAdornment,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  Tooltip,
} from '@mui/material';
import {
  Close as CloseIcon,
  ExpandMore as ExpandMoreIcon,
  Visibility as VisibilityIcon,
  VisibilityOff as VisibilityOffIcon,
  Help as HelpIcon,
} from '@mui/icons-material';
import { Repository } from '../lib/api';

interface RepositoryDialogProps {
  open: boolean;
  onClose: () => void;
  onSave: (data: any) => void;
  repository?: Repository | null;
  isSaving: boolean;
}

/**
 * RepositoryDialog - Dialog for adding or editing Git repository configurations
 *
 * Provides a form for configuring Git repository connections with authentication.
 * Supports multiple authentication methods (none, token, SSH, basic auth) and
 * validates Git URL formats. Includes helpful tooltips and examples.
 *
 * Features:
 * - Add new or edit existing repositories
 * - Repository name, URL, and branch configuration
 * - Multiple authentication types (public, token, SSH key, basic)
 * - Authentication secret management (show/hide toggle)
 * - Git URL validation (GitHub, GitLab, Bitbucket)
 * - Helpful tooltips and descriptions
 * - Collapsible authentication section
 * - Form validation with error messages
 * - Password/secret visibility toggle
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {boolean} props.open - Whether the dialog is open
 * @param {Function} props.onClose - Callback when dialog is closed
 * @param {Function} props.onSave - Callback with form data when saved
 * @param {Repository | null} [props.repository] - Repository to edit (null for new)
 * @param {boolean} props.isSaving - Whether save operation is in progress
 *
 * @returns {JSX.Element} Rendered repository dialog
 *
 * @example
 * // Add new repository
 * <RepositoryDialog
 *   open={isOpen}
 *   onClose={() => setIsOpen(false)}
 *   onSave={handleCreateRepo}
 *   isSaving={saving}
 * />
 *
 * @example
 * // Edit existing repository
 * <RepositoryDialog
 *   open={isOpen}
 *   repository={selectedRepo}
 *   onClose={() => setIsOpen(false)}
 *   onSave={handleUpdateRepo}
 *   isSaving={saving}
 * />
 *
 * @see RepositoryCard for repository display
 */
export default function RepositoryDialog({
  open,
  onClose,
  onSave,
  repository,
  isSaving,
}: RepositoryDialogProps) {
  const isEdit = !!repository;

  const [formData, setFormData] = useState({
    name: '',
    url: '',
    branch: 'main',
    authType: 'none',
    authSecret: '',
  });

  const [showSecret, setShowSecret] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (repository) {
      setFormData({
        name: repository.name,
        url: repository.url,
        branch: repository.branch || 'main',
        authType: repository.authType || 'none',
        authSecret: '',
      });
    } else {
      setFormData({
        name: '',
        url: '',
        branch: 'main',
        authType: 'none',
        authSecret: '',
      });
    }
    setErrors({});
    setShowSecret(false);
  }, [repository, open]);

  const validate = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.name.trim()) {
      newErrors.name = 'Repository name is required';
    }

    if (!formData.url.trim()) {
      newErrors.url = 'Repository URL is required';
    } else if (!isValidGitUrl(formData.url)) {
      newErrors.url = 'Invalid Git URL format';
    }

    if (!formData.branch.trim()) {
      newErrors.branch = 'Branch name is required';
    }

    if (formData.authType !== 'none' && !formData.authSecret.trim() && !isEdit) {
      newErrors.authSecret = 'Authentication secret is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const isValidGitUrl = (url: string) => {
    const patterns = [
      /^https?:\/\/.+\/.+\.git$/i,
      /^https?:\/\/github\.com\/.+\/.+$/i,
      /^https?:\/\/gitlab\.com\/.+\/.+$/i,
      /^https?:\/\/bitbucket\.org\/.+\/.+$/i,
      /^git@.+:.+\/.+\.git$/i,
    ];
    return patterns.some((pattern) => pattern.test(url));
  };

  const handleSave = () => {
    if (!validate()) return;

    const data = {
      name: formData.name,
      url: formData.url,
      branch: formData.branch,
      authType: formData.authType,
    };

    // Only include authSecret if it's set (for edit, empty means don't change)
    if (formData.authSecret) {
      (data as any).authSecret = formData.authSecret;
    }

    onSave(data);
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="md"
      fullWidth
      PaperProps={{
        sx: { minHeight: '400px' },
      }}
    >
      <DialogTitle>
        <Box display="flex" justifyContent="space-between" alignItems="center">
          <Typography variant="h6">{isEdit ? 'Edit' : 'Add'} Repository</Typography>
          <IconButton size="small" onClick={onClose}>
            <CloseIcon />
          </IconButton>
        </Box>
      </DialogTitle>

      <DialogContent>
        <Box sx={{ pt: 1 }}>
          <TextField
            fullWidth
            label="Repository Name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            error={!!errors.name}
            helperText={errors.name || 'A friendly name for this repository'}
            sx={{ mb: 2 }}
            required
            autoFocus
          />

          <TextField
            fullWidth
            label="Git Repository URL"
            value={formData.url}
            onChange={(e) => setFormData({ ...formData, url: e.target.value })}
            error={!!errors.url}
            helperText={errors.url || 'HTTPS or SSH URL to the Git repository'}
            placeholder="https://github.com/username/repository.git"
            sx={{ mb: 2 }}
            required
          />

          <TextField
            fullWidth
            label="Branch"
            value={formData.branch}
            onChange={(e) => setFormData({ ...formData, branch: e.target.value })}
            error={!!errors.branch}
            helperText={errors.branch || 'Branch to sync (default: main)'}
            sx={{ mb: 3 }}
            required
          />

          <Accordion defaultExpanded={formData.authType !== 'none'}>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography sx={{ fontWeight: 500 }}>Authentication</Typography>
            </AccordionSummary>
            <AccordionDetails>
              <TextField
                select
                fullWidth
                label="Authentication Type"
                value={formData.authType}
                onChange={(e) => setFormData({ ...formData, authType: e.target.value })}
                sx={{ mb: 2 }}
              >
                <MenuItem value="none">None (Public Repository)</MenuItem>
                <MenuItem value="token">Personal Access Token</MenuItem>
                <MenuItem value="ssh">SSH Key</MenuItem>
                <MenuItem value="basic">Basic Authentication</MenuItem>
              </TextField>

              {formData.authType !== 'none' && (
                <Box>
                  <TextField
                    fullWidth
                    label={getSecretLabel()}
                    type={showSecret ? 'text' : 'password'}
                    value={formData.authSecret}
                    onChange={(e) => setFormData({ ...formData, authSecret: e.target.value })}
                    error={!!errors.authSecret}
                    helperText={errors.authSecret || getSecretHelperText()}
                    required={!isEdit}
                    multiline={formData.authType === 'ssh'}
                    rows={formData.authType === 'ssh' ? 4 : 1}
                    InputProps={{
                      endAdornment: (
                        <InputAdornment position="end">
                          <IconButton
                            onClick={() => setShowSecret(!showSecret)}
                            edge="end"
                            size="small"
                          >
                            {showSecret ? <VisibilityOffIcon /> : <VisibilityIcon />}
                          </IconButton>
                          <Tooltip title={getSecretTooltip()}>
                            <IconButton size="small">
                              <HelpIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        </InputAdornment>
                      ),
                    }}
                  />

                  {isEdit && (
                    <Alert severity="info" sx={{ mt: 2 }}>
                      Leave empty to keep existing authentication credentials
                    </Alert>
                  )}
                </Box>
              )}
            </AccordionDetails>
          </Accordion>

          {!isEdit && (
            <Alert severity="info" sx={{ mt: 3 }}>
              Repository will be synced automatically after adding. Make sure it contains valid
              Template YAML files in a templates/ directory.
            </Alert>
          )}
        </Box>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button onClick={handleSave} variant="contained" disabled={isSaving}>
          {isSaving ? 'Saving...' : isEdit ? 'Update' : 'Add Repository'}
        </Button>
      </DialogActions>
    </Dialog>
  );

  function getSecretLabel() {
    switch (formData.authType) {
      case 'token':
        return 'Personal Access Token';
      case 'ssh':
        return 'SSH Private Key';
      case 'basic':
        return 'Password';
      default:
        return 'Secret';
    }
  }

  function getSecretHelperText() {
    switch (formData.authType) {
      case 'token':
        return 'GitHub/GitLab personal access token with repo read permissions';
      case 'ssh':
        return 'Private SSH key for repository access (PEM format)';
      case 'basic':
        return 'Password for basic authentication';
      default:
        return '';
    }
  }

  function getSecretTooltip() {
    switch (formData.authType) {
      case 'token':
        return 'Generate a token in your Git provider settings with repository read access';
      case 'ssh':
        return 'Paste the content of your private SSH key file (usually ~/.ssh/id_rsa)';
      case 'basic':
        return 'Your Git account password';
      default:
        return '';
    }
  }
}

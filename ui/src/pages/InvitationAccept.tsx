import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  Alert,
  CircularProgress,
  Paper,
} from '@mui/material';
import {
  CheckCircle,
  Error as ErrorIcon,
  Link as LinkIcon,
  ArrowForward,
} from '@mui/icons-material';
import { useParams, useNavigate } from 'react-router-dom';
import Layout from '../components/Layout';
import { api } from '../lib/api';
import { useUserStore } from '../store/userStore';

/**
 * InvitationAccept - Session invitation acceptance page
 *
 * Handles the workflow for accepting session sharing invitations. Users who receive
 * an invitation link can use this page to accept the invitation and gain access to
 * a shared session. Requires user authentication before accepting the invitation.
 *
 * Features:
 * - Display invitation details and token
 * - Show accepting user information
 * - Validate invitation token
 * - Automatic redirect to login if not authenticated
 * - Success confirmation with auto-redirect to session
 * - Error handling for invalid or expired invitations
 *
 * User workflows:
 * - Receive invitation link via email or direct sharing
 * - Authenticate if not already logged in
 * - Review invitation details
 * - Accept invitation to join shared session
 * - Automatic redirect to session viewer
 *
 * @page
 * @route /invite/:token - Accept session invitation
 * @access public - Available to all users with valid invitation link
 *
 * @component
 *
 * @returns {JSX.Element} Invitation acceptance interface with token validation
 *
 * @example
 * // Route configuration:
 * <Route path="/invite/:token" element={<InvitationAccept />} />
 *
 * @see SessionShare for session sharing management
 * @see SessionViewer for viewing shared sessions
 */
export default function InvitationAccept() {
  const { token } = useParams<{ token: string }>();
  const navigate = useNavigate();
  const currentUser = useUserStore((state) => state.user);

  const [accepting, setAccepting] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [error, setError] = useState('');
  const [sessionId, setSessionId] = useState('');

  useEffect(() => {
    // If user is not logged in, redirect to login
    if (!currentUser) {
      // Store the invitation token to redirect back after login
      sessionStorage.setItem('pending_invitation', token || '');
      navigate('/login', { state: { from: `/invite/${token}` } });
    }
  }, [currentUser, token, navigate]);

  const handleAcceptInvitation = async () => {
    if (!token || !currentUser?.id) {
      setError('Invalid invitation or user not logged in');
      return;
    }

    setAccepting(true);
    setError('');

    try {
      const result = await api.acceptInvitation(token, currentUser.id);
      setSessionId(result.sessionId);
      setAccepted(true);

      // Clear any pending invitation from session storage
      sessionStorage.removeItem('pending_invitation');

      // Redirect to the session after 2 seconds
      setTimeout(() => {
        navigate(`/sessions/${result.sessionId}/viewer`);
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to accept invitation');
    } finally {
      setAccepting(false);
    }
  };

  if (!currentUser) {
    return (
      <Layout>
        <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
          <CircularProgress />
        </Box>
      </Layout>
    );
  }

  if (accepted) {
    return (
      <Layout>
        <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
          <Card sx={{ maxWidth: 500, width: '100%' }}>
            <CardContent sx={{ textAlign: 'center', py: 6 }}>
              <CheckCircle sx={{ fontSize: 80, color: 'success.main', mb: 2 }} />
              <Typography variant="h5" fontWeight={600} gutterBottom>
                Invitation Accepted!
              </Typography>
              <Typography variant="body1" color="text.secondary" gutterBottom>
                You now have access to this session.
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
                Redirecting to the session...
              </Typography>
              <CircularProgress size={24} sx={{ mt: 2 }} />
            </CardContent>
          </Card>
        </Box>
      </Layout>
    );
  }

  return (
    <Layout>
      <Box display="flex" justifyContent="center" alignItems="center" minHeight="400px">
        <Card sx={{ maxWidth: 600, width: '100%' }}>
          <CardContent sx={{ p: 4 }}>
            <Box display="flex" alignItems="center" gap={2} mb={3}>
              <LinkIcon sx={{ fontSize: 40, color: 'primary.main' }} />
              <Box>
                <Typography variant="h5" fontWeight={600}>
                  Session Invitation
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  You've been invited to join a session
                </Typography>
              </Box>
            </Box>

            {error && (
              <Alert
                severity="error"
                icon={<ErrorIcon />}
                sx={{ mb: 3 }}
                onClose={() => setError('')}
              >
                {error}
              </Alert>
            )}

            <Paper variant="outlined" sx={{ p: 3, mb: 3, bgcolor: 'background.default' }}>
              <Typography variant="body2" color="text.secondary" gutterBottom>
                Invitation Token
              </Typography>
              <Typography
                variant="body2"
                sx={{
                  fontFamily: 'monospace',
                  wordBreak: 'break-all',
                  bgcolor: 'grey.100',
                  p: 1,
                  borderRadius: 1,
                  fontSize: '0.75rem',
                }}
              >
                {token}
              </Typography>

              <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }} gutterBottom>
                Accepting as
              </Typography>
              <Typography variant="body1" fontWeight={500}>
                {currentUser.fullName} (@{currentUser.username})
              </Typography>
            </Paper>

            <Alert severity="info" sx={{ mb: 3 }}>
              <Typography variant="body2">
                By accepting this invitation, you will gain access to a shared session. The session owner has granted you specific permissions to view or collaborate.
              </Typography>
            </Alert>

            <Box display="flex" gap={2}>
              <Button
                variant="contained"
                size="large"
                fullWidth
                onClick={handleAcceptInvitation}
                disabled={accepting}
                endIcon={accepting ? <CircularProgress size={20} /> : <ArrowForward />}
              >
                {accepting ? 'Accepting...' : 'Accept Invitation'}
              </Button>
              <Button
                variant="outlined"
                size="large"
                onClick={() => navigate('/sessions')}
                disabled={accepting}
              >
                Cancel
              </Button>
            </Box>

            <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 2, textAlign: 'center' }}>
              If you didn't expect this invitation, you can safely ignore it or contact the sender.
            </Typography>
          </CardContent>
        </Card>
      </Box>
    </Layout>
  );
}

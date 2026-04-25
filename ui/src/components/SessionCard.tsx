import { memo } from 'react';
import {
  Card,
  CardContent,
  CardActions,
  Box,
  Typography,
  Chip,
  Button,
  IconButton,
} from '@mui/material';
import {
  PlayArrow as PlayIcon,
  Pause as PauseIcon,
  Delete as DeleteIcon,
  OpenInNew as OpenIcon,
  LocalOffer as TagIcon,
  Share as ShareIcon,
  Link as LinkIcon,
  Cloud as K8sIcon,
  Storage as DockerIcon,
  CloudQueue as VMIcon,
  CloudCircle as CloudIcon,
  Computer as AgentIcon,
} from '@mui/icons-material';
import TagChip from './TagChip';
import ActivityIndicator from './ActivityIndicator';
import IdleTimer from './IdleTimer';
import { Session } from '../lib/api';

interface SessionCardProps {
  session: Session;
  onConnect: (session: Session) => void;
  onStateChange: (id: string, state: 'running' | 'hibernated') => void;
  onDelete: (id: string) => void;
  onManageTags: (session: Session) => void;
  onShare: (session: Session) => void;
  onInvitation: (session: Session) => void;
  isUpdating?: boolean;
}

/**
 * SessionCard - Display card for a containerized session
 *
 * Presents session information with state management controls, resource usage,
 * activity indicators, and collaboration features. Supports session lifecycle
 * actions (connect, pause, delete) and sharing capabilities. Includes real-time
 * activity monitoring with idle timer and connection count.
 *
 * Features:
 * - Session state and phase indicators with color coding
 * - Activity indicator (active/idle/hibernated)
 * - Idle timer with auto-hibernation countdown
 * - Resource usage display (CPU/memory)
 * - Active connections counter
 * - Session URL display
 * - Tag management
 * - Sharing and invitation links
 * - Connect, pause/resume, and delete actions
 * - Memoized for performance optimization
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {Session} props.session - Session data to display
 * @param {Function} props.onConnect - Callback to connect to session
 * @param {Function} props.onStateChange - Callback to change session state (run/hibernate)
 * @param {Function} props.onDelete - Callback to delete session
 * @param {Function} props.onManageTags - Callback to open tag management dialog
 * @param {Function} props.onShare - Callback to open share dialog
 * @param {Function} props.onInvitation - Callback to open invitation dialog
 * @param {boolean} [props.isUpdating=false] - Whether session is currently updating
 *
 * @returns {JSX.Element} Rendered session card
 *
 * @example
 * <SessionCard
 *   session={sessionData}
 *   onConnect={handleConnect}
 *   onStateChange={handleStateChange}
 *   onDelete={handleDelete}
 *   onManageTags={handleManageTags}
 *   onShare={handleShare}
 *   onInvitation={handleInvitation}
 * />
 *
 * @see IdleTimer for idle time display
 * @see ActivityIndicator for activity status
 * @see TagChip for tag display
 */
function SessionCard({
  session,
  onConnect,
  onStateChange,
  onDelete,
  onManageTags,
  onShare,
  onInvitation,
  isUpdating = false,
}: SessionCardProps) {
  const getStateColor = (state: string) => {
    switch (state) {
      case 'running':
        return 'success';
      case 'hibernated':
        return 'warning';
      case 'terminated':
        return 'error';
      default:
        return 'default';
    }
  };

  const getPhaseColor = (phase: string) => {
    switch (phase) {
      case 'Running':
        return 'success';
      case 'Pending':
        return 'info';
      case 'Hibernated':
        return 'warning';
      case 'Failed':
        return 'error';
      default:
        return 'default';
    }
  };

  const getPlatformIcon = (platform?: string) => {
    switch (platform?.toLowerCase()) {
      case 'kubernetes':
        return <K8sIcon fontSize="small" />;
      case 'docker':
        return <DockerIcon fontSize="small" />;
      case 'vm':
        return <VMIcon fontSize="small" />;
      case 'cloud':
        return <CloudIcon fontSize="small" />;
      default:
        return <AgentIcon fontSize="small" />;
    }
  };

  return (
    <Card component="article">
      <CardContent>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'start', mb: 2 }}>
          <Box>
            <Typography variant="h6" sx={{ fontWeight: 600 }}>
              {session.template}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {session.name}
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 0.5, flexDirection: 'column', alignItems: 'flex-end' }}>
            <Chip
              label={session.state}
              size="small"
              color={getStateColor(session.state)}
              aria-label={`Session state: ${session.state}`}
            />
            <Chip
              label={session.status.phase}
              size="small"
              color={getPhaseColor(session.status.phase)}
              aria-label={`Session phase: ${session.status.phase}`}
            />
            <ActivityIndicator
              isActive={session.isActive}
              isIdle={session.isIdle}
              state={session.state}
              size="small"
            />
          </Box>
        </Box>

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {session.state === 'running' && session.lastActivity && session.idleThreshold && (
            <Box>
              <IdleTimer
                lastActivity={session.lastActivity}
                idleDuration={session.idleDuration}
                idleThreshold={session.idleThreshold}
                showProgress={session.isIdle || false}
                compact={true}
              />
            </Box>
          )}
          <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
            <Typography variant="body2" color="text.secondary">
              Resources
            </Typography>
            <Typography variant="body2">
              {session.resources?.memory || 'N/A'} / {session.resources?.cpu || 'N/A'}
            </Typography>
          </Box>
          {session.activeConnections !== undefined && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
              <Typography variant="body2" color="text.secondary">
                Active Connections
              </Typography>
              <Typography variant="body2">{session.activeConnections}</Typography>
            </Box>
          )}
          {/* v2.0 Platform/Agent information */}
          {session.platform && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
              <Typography variant="body2" color="text.secondary">
                Platform
              </Typography>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                {getPlatformIcon(session.platform)}
                <Typography variant="body2" sx={{ textTransform: 'capitalize' }}>
                  {session.platform}
                </Typography>
              </Box>
            </Box>
          )}
          {session.agent_id && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
              <Typography variant="body2" color="text.secondary">
                Agent
              </Typography>
              <Typography variant="body2" sx={{ fontSize: '0.75rem', fontFamily: 'monospace' }} noWrap>
                {session.agent_id}
              </Typography>
            </Box>
          )}
          {session.region && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
              <Typography variant="body2" color="text.secondary">
                Region
              </Typography>
              <Typography variant="body2">{session.region}</Typography>
            </Box>
          )}
          {session.status.url && (
            <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
              <Typography variant="body2" color="text.secondary">
                URL
              </Typography>
              <Typography variant="body2" sx={{ fontSize: '0.75rem', maxWidth: '60%' }} noWrap>
                {session.status.url}
              </Typography>
            </Box>
          )}
          {session.tags && session.tags.length > 0 && (
            <Box sx={{ mt: 1 }}>
              <Typography variant="caption" color="text.secondary" display="block" gutterBottom>
                Tags
              </Typography>
              <Box display="flex" flexWrap="wrap" gap={0.5}>
                {session.tags.map(tag => (
                  <TagChip key={tag} tag={tag} />
                ))}
              </Box>
            </Box>
          )}
        </Box>
      </CardContent>
      <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
        <Box>
          {session.state === 'running' ? (
            <>
              <Button
                size="small"
                startIcon={<OpenIcon />}
                onClick={() => onConnect(session)}
                disabled={session.status.phase !== 'Running' || !session.status.url}
              >
                Connect
              </Button>
              <IconButton
                size="small"
                color="warning"
                onClick={() => onStateChange(session.name, 'hibernated')}
                disabled={isUpdating}
                aria-label="Hibernate Session"
                title="Hibernate Session"
              >
                <PauseIcon />
              </IconButton>
            </>
          ) : (
            <IconButton
              size="small"
              color="success"
              onClick={() => onStateChange(session.name, 'running')}
              disabled={isUpdating}
              aria-label="Resume Session"
              title="Resume Session"
            >
              <PlayIcon />
            </IconButton>
          )}
        </Box>
        <Box>
          <IconButton
            size="small"
            color="primary"
            onClick={() => onShare(session)}
            title="Share with User"
            aria-label="Share with User"
          >
            <ShareIcon />
          </IconButton>
          <IconButton
            size="small"
            color="primary"
            onClick={() => onInvitation(session)}
            title="Create Invitation Link"
            aria-label="Create Invitation Link"
          >
            <LinkIcon />
          </IconButton>
          <IconButton
            size="small"
            color="primary"
            onClick={() => onManageTags(session)}
            title="Manage Tags"
            aria-label="Manage Tags"
          >
            <TagIcon />
          </IconButton>
          <IconButton
            size="small"
            color="error"
            onClick={() => onDelete(session.name)}
            title="Delete Session"
            aria-label="Delete Session"
          >
            <DeleteIcon />
          </IconButton>
        </Box>
      </CardActions>
    </Card>
  );
}

// Memoize the component to prevent unnecessary re-renders
// Only re-render if session data or callbacks change
export default memo(SessionCard, (prevProps, nextProps) => {
  // Custom comparison function for better memoization
  return (
    prevProps.session.name === nextProps.session.name &&
    prevProps.session.state === nextProps.session.state &&
    prevProps.session.status.phase === nextProps.session.status.phase &&
    prevProps.session.isActive === nextProps.session.isActive &&
    prevProps.session.isIdle === nextProps.session.isIdle &&
    prevProps.session.activeConnections === nextProps.session.activeConnections &&
    prevProps.session.lastActivity === nextProps.session.lastActivity &&
    prevProps.isUpdating === nextProps.isUpdating &&
    JSON.stringify(prevProps.session.tags) === JSON.stringify(nextProps.session.tags)
  );
});

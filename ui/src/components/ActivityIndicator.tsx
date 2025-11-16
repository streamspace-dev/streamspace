import { memo } from 'react';
import { Chip, Tooltip } from '@mui/material';
import {
  Circle as ActiveIcon,
  RemoveCircleOutline as IdleIcon,
  Bedtime as HibernatingIcon,
} from '@mui/icons-material';

interface ActivityIndicatorProps {
  isActive?: boolean;
  isIdle?: boolean;
  state?: 'running' | 'hibernated' | 'terminated';
  size?: 'small' | 'medium';
  showLabel?: boolean;
}

/**
 * ActivityIndicator - Display session activity status with visual indicator
 *
 * Shows session activity state with color-coded chip or icon. Combines session
 * state (running/hibernated/terminated) with activity flags (active/idle) to
 * display appropriate status. Includes tooltip with detailed description.
 *
 * Features:
 * - Multiple activity states (active/idle/hibernated/terminated)
 * - Color-coded chips (green/yellow/gray/red)
 * - Icon-only or chip with label display
 * - Tooltip with status description
 * - Configurable size (small/medium)
 * - Memoized for performance
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {boolean} [props.isActive=false] - Whether session has recent activity
 * @param {boolean} [props.isIdle=false] - Whether session is idle
 * @param {'running' | 'hibernated' | 'terminated'} [props.state='running'] - Session state
 * @param {'small' | 'medium'} [props.size='small'] - Indicator size
 * @param {boolean} [props.showLabel=true] - Whether to show text label
 *
 * @returns {JSX.Element} Rendered activity indicator
 *
 * @example
 * <ActivityIndicator
 *   isActive={session.isActive}
 *   isIdle={session.isIdle}
 *   state={session.state}
 * />
 *
 * @example
 * // Icon only (no label)
 * <ActivityIndicator
 *   isActive={true}
 *   showLabel={false}
 *   size="medium"
 * />
 *
 * @see SessionCard for usage with sessions
 */
function ActivityIndicator({
  isActive = false,
  isIdle = false,
  state = 'running',
  size = 'small',
  showLabel = true,
}: ActivityIndicatorProps) {
  // Determine status based on session state and activity
  let color: 'success' | 'warning' | 'default' | 'error';
  let icon: React.ReactElement;
  let label: string;
  let tooltip: string;

  if (state === 'terminated') {
    color = 'default';
    icon = <Circle style={{ fontSize: size === 'small' ? 12 : 16 }} />;
    label = 'Terminated';
    tooltip = 'Session has been terminated';
  } else if (state === 'hibernated') {
    color = 'default';
    icon = <HibernatingIcon style={{ fontSize: size === 'small' ? 12 : 16 }} />;
    label = 'Hibernated';
    tooltip = 'Session is hibernated to save resources';
  } else if (isIdle) {
    color = 'warning';
    icon = <IdleIcon style={{ fontSize: size === 'small' ? 12 : 16 }} />;
    label = 'Idle';
    tooltip = 'No activity detected - may hibernate soon';
  } else if (isActive) {
    color = 'success';
    icon = <ActiveIcon style={{ fontSize: size === 'small' ? 12 : 16 }} />;
    label = 'Active';
    tooltip = 'Session is active';
  } else {
    color = 'default';
    icon = <Circle style={{ fontSize: size === 'small' ? 12 : 16 }} />;
    label = 'Unknown';
    tooltip = 'Activity status unknown';
  }

  // Simple icon without label
  if (!showLabel) {
    return (
      <Tooltip title={tooltip}>
        {icon}
      </Tooltip>
    );
  }

  // Chip with label
  return (
    <Tooltip title={tooltip}>
      <Chip
        icon={icon}
        label={label}
        color={color}
        size={size}
        variant="outlined"
      />
    </Tooltip>
  );
}

// Helper component for just the circle icon
function Circle({ style }: { style?: React.CSSProperties }) {
  return (
    <svg viewBox="0 0 24 24" style={style}>
      <circle cx="12" cy="12" r="8" fill="currentColor" />
    </svg>
  );
}

// Memoize to prevent re-renders when activity status hasn't changed
export default memo(ActivityIndicator);

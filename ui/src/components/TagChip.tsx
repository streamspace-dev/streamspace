import { memo } from 'react';
import { Chip } from '@mui/material';
import { LocalOffer } from '@mui/icons-material';

interface TagChipProps {
  tag: string;
  onDelete?: () => void;
  onClick?: () => void;
  size?: 'small' | 'medium';
  variant?: 'filled' | 'outlined';
}

/**
 * TagChip - Reusable chip component for displaying session tags
 *
 * Displays a tag in a Material-UI Chip component with consistent styling.
 * Supports optional delete and click handlers for tag management and filtering.
 * Includes a tag icon and is memoized to prevent unnecessary re-renders.
 *
 * Features:
 * - Tag icon (LocalOffer)
 * - Optional delete button
 * - Optional click handler for filtering
 * - Configurable size (small/medium)
 * - Configurable variant (filled/outlined)
 * - Primary color theme
 * - Memoized for performance
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {string} props.tag - Tag text to display
 * @param {Function} [props.onDelete] - Callback when delete button clicked
 * @param {Function} [props.onClick] - Callback when chip is clicked
 * @param {'small' | 'medium'} [props.size='small'] - Chip size
 * @param {'filled' | 'outlined'} [props.variant='filled'] - Chip variant
 *
 * @returns {JSX.Element} Rendered tag chip
 *
 * @example
 * <TagChip tag="production" />
 *
 * @example
 * // With delete handler
 * <TagChip
 *   tag="development"
 *   onDelete={() => removeTag('development')}
 * />
 *
 * @example
 * // With click handler for filtering
 * <TagChip
 *   tag="testing"
 *   onClick={() => filterByTag('testing')}
 * />
 *
 * @see TagManager for tag management
 */
function TagChip({ tag, onDelete, onClick, size = 'small', variant = 'filled' }: TagChipProps) {
  return (
    <Chip
      icon={<LocalOffer />}
      label={tag}
      size={size}
      variant={variant}
      onDelete={onDelete}
      onClick={onClick}
      color="primary"
      sx={{
        mr: 0.5,
        mb: 0.5,
        cursor: onClick ? 'pointer' : 'default'
      }}
    />
  );
}

// Memoize to prevent re-renders when tag hasn't changed
export default memo(TagChip);

import { memo } from 'react';
import { Box, Typography } from '@mui/material';
import { Star, StarHalf, StarOutline } from '@mui/icons-material';

interface RatingStarsProps {
  rating: number;
  count?: number;
  size?: 'small' | 'medium' | 'large';
  showCount?: boolean;
  interactive?: boolean;
  onRate?: (rating: number) => void;
}

/**
 * RatingStars - Star rating display and input component
 *
 * Displays a 5-star rating with full, half, and empty stars. Supports both
 * read-only display mode and interactive rating mode. Shows review count or
 * average rating value. Memoized to prevent unnecessary re-renders.
 *
 * Features:
 * - Full, half, and empty star icons
 * - Configurable size (small/medium/large)
 * - Optional review count display
 * - Optional average rating display
 * - Interactive mode for user rating input
 * - Click handlers for each star
 * - Memoized for performance
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {number} props.rating - Average rating (0-5)
 * @param {number} [props.count] - Number of reviews/ratings
 * @param {'small' | 'medium' | 'large'} [props.size='small'] - Star size
 * @param {boolean} [props.showCount=true] - Whether to show review count
 * @param {boolean} [props.interactive=false] - Whether stars are clickable
 * @param {Function} [props.onRate] - Callback when star is clicked (rating: number)
 *
 * @returns {JSX.Element} Rendered rating stars
 *
 * @example
 * // Display mode
 * <RatingStars rating={4.5} count={120} size="medium" />
 *
 * @example
 * // Interactive mode
 * <RatingStars
 *   rating={userRating}
 *   interactive={true}
 *   onRate={setUserRating}
 *   showCount={false}
 * />
 *
 * @see PluginDetailModal for usage in plugin ratings
 * @see TemplateDetailModal for usage in template ratings
 */
function RatingStars({
  rating,
  count,
  size = 'small',
  showCount = true,
  interactive = false,
  onRate,
}: RatingStarsProps) {
  const iconSize = size === 'small' ? 16 : size === 'medium' ? 20 : 24;
  const fullStars = Math.floor(rating);
  const hasHalfStar = rating % 1 >= 0.5;
  const emptyStars = 5 - fullStars - (hasHalfStar ? 1 : 0);

  const handleClick = (starIndex: number) => {
    if (interactive && onRate) {
      onRate(starIndex + 1);
    }
  };

  return (
    <Box display="flex" alignItems="center" gap={0.5}>
      <Box display="flex" alignItems="center">
        {[...Array(fullStars)].map((_, i) => (
          <Star
            key={`full-${i}`}
            sx={{
              fontSize: iconSize,
              color: '#FFA500',
              cursor: interactive ? 'pointer' : 'default',
            }}
            onClick={() => handleClick(i)}
          />
        ))}
        {hasHalfStar && (
          <StarHalf
            sx={{
              fontSize: iconSize,
              color: '#FFA500',
              cursor: interactive ? 'pointer' : 'default',
            }}
            onClick={() => handleClick(fullStars)}
          />
        )}
        {[...Array(emptyStars)].map((_, i) => (
          <StarOutline
            key={`empty-${i}`}
            sx={{
              fontSize: iconSize,
              color: '#FFA500',
              cursor: interactive ? 'pointer' : 'default',
            }}
            onClick={() => handleClick(fullStars + (hasHalfStar ? 1 : 0) + i)}
          />
        ))}
      </Box>
      {showCount && count !== undefined && (
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ ml: 0.5 }}
        >
          ({count})
        </Typography>
      )}
      {!showCount && rating > 0 && (
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ ml: 0.5, fontWeight: 600 }}
        >
          {rating.toFixed(1)}
        </Typography>
      )}
    </Box>
  );
}

// Memoize to prevent re-renders when rating hasn't changed
export default memo(RatingStars);

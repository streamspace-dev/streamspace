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

export default function RatingStars({
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

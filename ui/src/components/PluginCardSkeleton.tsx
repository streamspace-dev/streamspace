import {
  Card,
  CardContent,
  CardActions,
  Box,
  Skeleton,
} from '@mui/material';

export default function PluginCardSkeleton() {
  return (
    <Card
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <CardContent sx={{ flexGrow: 1, pb: 1 }}>
        {/* Icon badge skeleton */}
        <Box
          sx={{
            position: 'absolute',
            top: 8,
            right: 8,
          }}
        >
          <Skeleton variant="circular" width={32} height={32} />
        </Box>

        {/* Header with icon and title */}
        <Box display="flex" alignItems="center" gap={1} mb={1}>
          <Skeleton variant="rounded" width={40} height={40} />
          <Box flexGrow={1}>
            <Skeleton variant="text" width="70%" height={24} />
            <Skeleton variant="text" width="50%" height={16} />
          </Box>
        </Box>

        {/* Rating stars */}
        <Box mb={1}>
          <Skeleton variant="text" width={120} height={20} />
        </Box>

        {/* Description */}
        <Box mb={2}>
          <Skeleton variant="text" width="100%" />
          <Skeleton variant="text" width="90%" />
        </Box>

        {/* Tags */}
        <Box display="flex" gap={0.5} flexWrap="wrap" mb={1}>
          <Skeleton variant="rounded" width={60} height={24} />
          <Skeleton variant="rounded" width={80} height={24} />
        </Box>

        {/* Stats */}
        <Box display="flex" gap={2} mt={2}>
          <Skeleton variant="text" width={60} height={20} />
        </Box>
      </CardContent>

      <CardActions sx={{ justifyContent: 'space-between', px: 2, pb: 2 }}>
        <Skeleton variant="rounded" width={80} height={32} />
        <Skeleton variant="circular" width={32} height={32} />
      </CardActions>
    </Card>
  );
}

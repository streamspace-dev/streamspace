import { useEffect, useState } from 'react';
import {
  Card,
  CardContent,
  Typography,
  Box,
  LinearProgress,
  Alert,
  Skeleton,
  Chip,
} from '@mui/material';
import {
  Memory as MemoryIcon,
  Speed as CPUIcon,
  Storage as StorageIcon,
  Workspaces as SessionsIcon,
  Warning as WarningIcon,
} from '@mui/icons-material';
import { api, type UserQuota } from '../lib/api';

interface QuotaMetric {
  label: string;
  used: number;
  max: number;
  unit: string;
  icon: React.ReactNode;
  color: 'primary' | 'secondary' | 'success' | 'warning' | 'error';
}

/**
 * QuotaCard - Display user's resource quota usage in a card
 *
 * Shows current resource usage against allocated quotas for sessions, CPU,
 * memory, and storage. Displays progress bars with color coding based on
 * usage percentage. Automatically fetches current user's quota on mount.
 *
 * Features:
 * - Real-time resource usage display
 * - Progress bars with color coding (green/yellow/red)
 * - Sessions, CPU, memory, and storage metrics
 * - Warning indicator when approaching limits (75%+)
 * - Automatic unit conversion (GiB for memory/storage, cores for CPU)
 * - Loading skeleton state
 * - Error handling with user-friendly messages
 *
 * @component
 *
 * @returns {JSX.Element} Rendered quota card
 *
 * @example
 * <QuotaCard />
 *
 * @see api.getCurrentUserQuota for quota data fetching
 * @see QuotaAlert for alert-style quota warnings
 */
export default function QuotaCard() {
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadQuota();
  }, []);

  const loadQuota = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getCurrentUserQuota();
      setQuota(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load quota');
    } finally {
      setLoading(false);
    }
  };

  const parseMemory = (mem: string): number => {
    if (!mem || mem === '0') return 0;
    const match = mem.match(/^(\d+(?:\.\d+)?)(Mi|Gi|M|G)?$/);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = match[2];
    if (unit === 'Gi' || unit === 'G') return value;
    if (unit === 'Mi' || unit === 'M') return value / 1024;
    return 0;
  };

  const parseCPU = (cpu: string): number => {
    if (!cpu || cpu === '0') return 0;
    const match = cpu.match(/^(\d+(?:\.\d+)?)(m)?$/);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = match[2];
    if (unit === 'm') return value / 1000;
    return value;
  };

  const parseStorage = (storage: string): number => {
    if (!storage || storage === '0') return 0;
    const match = storage.match(/^(\d+(?:\.\d+)?)(Ti|Gi|T|G)?$/);
    if (!match) return 0;
    const value = parseFloat(match[1]);
    const unit = match[2];
    if (unit === 'Ti' || unit === 'T') return value * 1024;
    return value;
  };

  const getPercentage = (used: number, max: number): number => {
    if (max === 0) return 0;
    return Math.min((used / max) * 100, 100);
  };

  const getColor = (percentage: number): 'primary' | 'success' | 'warning' | 'error' => {
    if (percentage >= 90) return 'error';
    if (percentage >= 75) return 'warning';
    if (percentage >= 50) return 'primary';
    return 'success';
  };

  if (loading) {
    return (
      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Resource Quota
          </Typography>
          <Box>
            {[1, 2, 3, 4].map((i) => (
              <Box key={i} mb={2}>
                <Skeleton variant="text" width="40%" />
                <Skeleton variant="rectangular" height={8} sx={{ mt: 1 }} />
              </Box>
            ))}
          </Box>
        </CardContent>
      </Card>
    );
  }

  if (error || !quota) {
    return (
      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            Resource Quota
          </Typography>
          <Alert severity="error">{error || 'Unable to load quota information'}</Alert>
        </CardContent>
      </Card>
    );
  }

  const metrics: QuotaMetric[] = [
    {
      label: 'Sessions',
      used: quota.usedSessions,
      max: quota.maxSessions,
      unit: '',
      icon: <SessionsIcon />,
      color: getColor(getPercentage(quota.usedSessions, quota.maxSessions)),
    },
    {
      label: 'CPU',
      used: parseCPU(quota.usedCpu),
      max: parseCPU(quota.maxCpu),
      unit: ' cores',
      icon: <CPUIcon />,
      color: getColor(getPercentage(parseCPU(quota.usedCpu), parseCPU(quota.maxCpu))),
    },
    {
      label: 'Memory',
      used: parseMemory(quota.usedMemory),
      max: parseMemory(quota.maxMemory),
      unit: ' GiB',
      icon: <MemoryIcon />,
      color: getColor(getPercentage(parseMemory(quota.usedMemory), parseMemory(quota.maxMemory))),
    },
    {
      label: 'Storage',
      used: parseStorage(quota.usedStorage),
      max: parseStorage(quota.maxStorage),
      unit: ' GiB',
      icon: <StorageIcon />,
      color: getColor(getPercentage(parseStorage(quota.usedStorage), parseStorage(quota.maxStorage))),
    },
  ];

  const hasWarnings = metrics.some(m => getPercentage(m.used, m.max) >= 75);

  return (
    <Card>
      <CardContent>
        <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
          <Typography variant="h6">Resource Quota</Typography>
          {hasWarnings && (
            <Chip
              icon={<WarningIcon />}
              label="Approaching Limits"
              color="warning"
              size="small"
            />
          )}
        </Box>

        {metrics.map((metric) => {
          const percentage = getPercentage(metric.used, metric.max);

          return (
            <Box key={metric.label} mb={2.5}>
              <Box display="flex" alignItems="center" mb={0.5}>
                <Box color="text.secondary" display="flex" alignItems="center" mr={1}>
                  {metric.icon}
                </Box>
                <Typography variant="body2" color="text.secondary" flexGrow={1}>
                  {metric.label}
                </Typography>
                <Typography variant="body2" fontWeight={600}>
                  {metric.used.toFixed(metric.label === 'Sessions' ? 0 : 1)}
                  {metric.unit} / {metric.max.toFixed(metric.label === 'Sessions' ? 0 : 1)}
                  {metric.unit}
                </Typography>
              </Box>
              <Box display="flex" alignItems="center" gap={1}>
                <LinearProgress
                  variant="determinate"
                  value={percentage}
                  color={metric.color}
                  sx={{ flexGrow: 1, height: 8, borderRadius: 1 }}
                />
                <Typography variant="caption" color="text.secondary" minWidth={45} textAlign="right">
                  {percentage.toFixed(0)}%
                </Typography>
              </Box>
            </Box>
          );
        })}

        {hasWarnings && (
          <Alert severity="warning" sx={{ mt: 2 }}>
            You are approaching your resource limits. Consider freeing up resources or contacting your administrator.
          </Alert>
        )}
      </CardContent>
    </Card>
  );
}

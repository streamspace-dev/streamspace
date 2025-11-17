import { useEffect, useState, memo } from 'react';
import { Alert, AlertTitle, Box, LinearProgress, Typography } from '@mui/material';
import { api, type UserQuota } from '../lib/api';

interface QuotaAlertProps {
  onQuotaLoad?: (quota: UserQuota) => void;
}

/**
 * QuotaAlert - Alert banner for resource quota warnings
 *
 * Displays a dismissible alert when user's resource usage exceeds 75% of their
 * allocated quota. Shows detailed breakdown of resources approaching or at limits
 * with progress bars. Only renders when threshold is exceeded.
 *
 * Features:
 * - Conditional rendering (only shows when usage >= 75%)
 * - Severity levels (warning at 75%, error at 90%)
 * - Detailed resource breakdown with progress bars
 * - Sessions, CPU, memory, and storage metrics
 * - Silent failure (optional display)
 * - Callback when quota data loads
 *
 * @component
 *
 * @param {Object} props - Component props
 * @param {Function} [props.onQuotaLoad] - Callback with quota data when loaded
 *
 * @returns {JSX.Element | null} Rendered alert or null if under threshold
 *
 * @example
 * <QuotaAlert onQuotaLoad={(quota) => console.log(quota)} />
 *
 * @example
 * // Typical placement at top of dashboard
 * <Box>
 *   <QuotaAlert />
 *   <Dashboard />
 * </Box>
 *
 * @see api.getCurrentUserQuota for quota data fetching
 * @see QuotaCard for detailed quota display
 */
function QuotaAlert({ onQuotaLoad }: QuotaAlertProps) {
  const [quota, setQuota] = useState<UserQuota | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadQuota();
  }, []);

  const loadQuota = async () => {
    try {
      setLoading(true);
      const data = await api.getCurrentUserQuota();
      setQuota(data);
      if (onQuotaLoad) {
        onQuotaLoad(data);
      }
    } catch (err) {
      // Silently fail - quota alerts are optional
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

  if (loading || !quota) {
    return null;
  }

  const sessionPercentage = getPercentage(quota.usedSessions, quota.maxSessions);
  const cpuPercentage = getPercentage(parseCPU(quota.usedCpu), parseCPU(quota.maxCpu));
  const memoryPercentage = getPercentage(parseMemory(quota.usedMemory), parseMemory(quota.maxMemory));
  const storagePercentage = getPercentage(parseStorage(quota.usedStorage), parseStorage(quota.maxStorage));

  const maxPercentage = Math.max(sessionPercentage, cpuPercentage, memoryPercentage, storagePercentage);

  // Only show alert if any quota is >= 75%
  if (maxPercentage < 75) {
    return null;
  }

  const severity = maxPercentage >= 90 ? 'error' : 'warning';
  const title = maxPercentage >= 90 ? 'Quota Limit Reached' : 'Approaching Quota Limits';
  const message =
    maxPercentage >= 90
      ? 'You have reached your resource quota limits. Free up resources or contact your administrator.'
      : 'You are approaching your resource quota limits. Consider freeing up resources soon.';

  const warnings: string[] = [];
  if (sessionPercentage >= 75) {
    warnings.push(`Sessions: ${quota.usedSessions}/${quota.maxSessions} (${sessionPercentage.toFixed(0)}%)`);
  }
  if (cpuPercentage >= 75) {
    warnings.push(`CPU: ${parseCPU(quota.usedCpu).toFixed(1)}/${parseCPU(quota.maxCpu).toFixed(1)} cores (${cpuPercentage.toFixed(0)}%)`);
  }
  if (memoryPercentage >= 75) {
    warnings.push(`Memory: ${parseMemory(quota.usedMemory).toFixed(1)}/${parseMemory(quota.maxMemory).toFixed(1)} GiB (${memoryPercentage.toFixed(0)}%)`);
  }
  if (storagePercentage >= 75) {
    warnings.push(`Storage: ${parseStorage(quota.usedStorage).toFixed(1)}/${parseStorage(quota.maxStorage).toFixed(1)} GiB (${storagePercentage.toFixed(0)}%)`);
  }

  return (
    <Alert severity={severity} sx={{ mb: 3 }}>
      <AlertTitle>{title}</AlertTitle>
      {message}
      <Box mt={2}>
        {warnings.map((warning) => (
          <Box key={warning} mb={1}>
            <Typography variant="body2">{warning}</Typography>
            <LinearProgress
              variant="determinate"
              value={parseFloat(warning.match(/\((\d+)%\)/)?.[1] || '0')}
              color={severity}
              sx={{ height: 6, borderRadius: 1, mt: 0.5 }}
            />
          </Box>
        ))}
      </Box>
    </Alert>
  );
}

// Export memoized version to prevent unnecessary re-renders when parent component updates
export default memo(QuotaAlert);

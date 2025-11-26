import { useState } from 'react';
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  Grid,
  IconButton,
  InputLabel,
  MenuItem,
  Pagination,
  Paper,
  Select,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material';
import {
  Download as DownloadIcon,
  Refresh as RefreshIcon,
  Search as SearchIcon,
  Visibility as VisibilityIcon,
  FilterList as FilterListIcon,
} from '@mui/icons-material';
import { useQuery } from '@tanstack/react-query';
import { DateTimePicker } from '@mui/x-date-pickers/DateTimePicker';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { AdapterDateFns } from '@mui/x-date-pickers/AdapterDateFns';
import { useNotificationQueue } from '../../components/NotificationQueue';
import AdminPortalLayout from '../../components/AdminPortalLayout';
import { api } from '../../lib/api';

/**
 * Audit log entry structure from API
 */
interface AuditLog {
  id: number;
  user_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  changes?: Record<string, any>;
  timestamp: string;
  ip_address: string;
}

/**
 * API response structure for paginated audit logs
 */
interface AuditLogListResponse {
  logs: AuditLog[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

/**
 * AuditLogs - Audit log viewer for administrators
 *
 * Administrative interface for viewing, filtering, and exporting audit logs for compliance
 * and security investigations. Provides comprehensive audit trail access with advanced
 * filtering, pagination, and export capabilities.
 *
 * Features:
 * - View all audit logs in paginated table format
 * - Filter by user ID, action, resource type, IP address, status code
 * - Date range filtering with calendar pickers
 * - Search functionality
 * - Export to CSV or JSON for compliance reports
 * - View detailed audit log entry with JSON diff viewer
 * - Pagination support (100 entries per page)
 *
 * Compliance support:
 * - SOC2: Complete audit trail of system changes
 * - HIPAA: PHI access logging with 6-year retention
 * - GDPR: Data processing activity records
 * - ISO 27001: User activity and security event logging
 *
 * Use cases:
 * - Security incident investigation (who did what when)
 * - Compliance audits and reporting
 * - User activity analysis
 * - Failed access attempt detection
 * - System change tracking
 *
 * @page
 * @route /admin/audit - Audit log viewer
 * @access admin - Restricted to administrators only
 *
 * @component
 *
 * @returns {JSX.Element} Audit log viewer interface with filtering and export
 *
 * @example
 * // Route configuration:
 * <Route path="/admin/audit" element={<AuditLogs />} />
 */
export default function AuditLogs() {
  const { addNotification } = useNotificationQueue();

  // Filters
  const [userIdFilter, setUserIdFilter] = useState('');
  const [actionFilter, setActionFilter] = useState('');
  const [resourceTypeFilter, setResourceTypeFilter] = useState('');
  const [ipAddressFilter, setIpAddressFilter] = useState('');
  const [statusCodeFilter, setStatusCodeFilter] = useState('');
  const [startDate, setStartDate] = useState<Date | null>(null);
  const [endDate, setEndDate] = useState<Date | null>(null);

  // Pagination
  const [page, setPage] = useState(1);
  const [pageSize] = useState(100);

  // Detail dialog
  const [detailDialogOpen, setDetailDialogOpen] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);

  // Build query parameters for API
  const buildQueryParams = () => {
    const params: Record<string, string> = {
      page: page.toString(),
      page_size: pageSize.toString(),
    };

    if (userIdFilter) params.user_id = userIdFilter;
    if (actionFilter) params.action = actionFilter;
    if (resourceTypeFilter) params.resource_type = resourceTypeFilter;
    if (ipAddressFilter) params.ip_address = ipAddressFilter;
    if (statusCodeFilter) params.status_code = statusCodeFilter;
    if (startDate) params.start_date = startDate.toISOString();
    if (endDate) params.end_date = endDate.toISOString();

    return params;
  };

  // Fetch audit logs
  const { data, isLoading, refetch } = useQuery<AuditLogListResponse>({
    queryKey: ['auditLogs', page, userIdFilter, actionFilter, resourceTypeFilter, ipAddressFilter, statusCodeFilter, startDate, endDate],
    queryFn: async () => {
      const params = buildQueryParams();
      const query = new URLSearchParams(params).toString();
      const response = await fetch(`/api/v1/admin/audit?${query}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch audit logs');
      }

      return response.json();
    },
  });

  const logs = data?.logs || [];
  const totalPages = data?.total_pages || 1;
  const total = data?.total || 0;

  // Handle export
  const handleExport = async (format: 'csv' | 'json') => {
    try {
      const params = buildQueryParams();
      params.format = format;
      params.limit = '10000'; // Export limit
      delete params.page; // Remove pagination for export
      delete params.page_size;

      const query = new URLSearchParams(params).toString();
      const response = await fetch(`/api/v1/admin/audit/export?${query}`, {
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to export audit logs');
      }

      // Download file
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit_logs_${new Date().toISOString().split('T')[0]}.${format}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);

      addNotification({
        message: `Exported ${format.toUpperCase()} successfully`,
        severity: 'success',
        priority: 'low',
        title: 'Export Complete',
      });
    } catch (error) {
      addNotification({
        message: `Failed to export: ${error.message}`,
        severity: 'error',
        priority: 'high',
        title: 'Export Failed',
      });
    }
  };

  // Handle view details
  const handleViewDetails = (log: AuditLog) => {
    setSelectedLog(log);
    setDetailDialogOpen(true);
  };

  // Handle clear filters
  const handleClearFilters = () => {
    setUserIdFilter('');
    setActionFilter('');
    setResourceTypeFilter('');
    setIpAddressFilter('');
    setStatusCodeFilter('');
    setStartDate(null);
    setEndDate(null);
    setPage(1);
  };

  // Format timestamp for display
  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  // Get status code chip color
  const getStatusCodeColor = (statusCode: number): 'success' | 'warning' | 'error' | 'info' => {
    if (statusCode >= 200 && statusCode < 300) return 'success';
    if (statusCode >= 300 && statusCode < 400) return 'info';
    if (statusCode >= 400 && statusCode < 500) return 'warning';
    return 'error';
  };

  return (
    <AdminPortalLayout title="Audit Logs">
      <Container maxWidth="xl">
        {/* Header */}
        <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box>
            <Typography variant="h4" gutterBottom>
              Audit Logs
            </Typography>
            <Typography variant="body2" color="text.secondary">
              Security and compliance audit trail - {total.toLocaleString()} total entries
            </Typography>
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Tooltip title="Export to CSV">
              <Button
                variant="outlined"
                startIcon={<DownloadIcon />}
                onClick={() => handleExport('csv')}
              >
                CSV
              </Button>
            </Tooltip>
            <Tooltip title="Export to JSON">
              <Button
                variant="outlined"
                startIcon={<DownloadIcon />}
                onClick={() => handleExport('json')}
              >
                JSON
              </Button>
            </Tooltip>
            <Tooltip title="Refresh">
              <IconButton onClick={() => refetch()} aria-label="Refresh">
                <RefreshIcon />
              </IconButton>
            </Tooltip>
          </Box>
        </Box>

        {/* Filters */}
        <Card sx={{ mb: 3 }}>
          <CardContent>
            <Typography variant="h6" gutterBottom sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <FilterListIcon /> Filters
            </Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} md={4}>
                <TextField
                  fullWidth
                  label="User ID"
                  value={userIdFilter}
                  onChange={(e) => setUserIdFilter(e.target.value)}
                  placeholder="user-123"
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <FormControl fullWidth>
                  <InputLabel>Action</InputLabel>
                  <Select
                    value={actionFilter}
                    label="Action"
                    onChange={(e) => setActionFilter(e.target.value)}
                  >
                    <MenuItem value="">All</MenuItem>
                    <MenuItem value="GET">GET</MenuItem>
                    <MenuItem value="POST">POST</MenuItem>
                    <MenuItem value="PUT">PUT</MenuItem>
                    <MenuItem value="PATCH">PATCH</MenuItem>
                    <MenuItem value="DELETE">DELETE</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} md={4}>
                <TextField
                  fullWidth
                  label="Resource Type"
                  value={resourceTypeFilter}
                  onChange={(e) => setResourceTypeFilter(e.target.value)}
                  placeholder="/api/sessions"
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <TextField
                  fullWidth
                  label="IP Address"
                  value={ipAddressFilter}
                  onChange={(e) => setIpAddressFilter(e.target.value)}
                  placeholder="192.168.1.1"
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <FormControl fullWidth>
                  <InputLabel>Status Code</InputLabel>
                  <Select
                    value={statusCodeFilter}
                    label="Status Code"
                    onChange={(e) => setStatusCodeFilter(e.target.value)}
                  >
                    <MenuItem value="">All</MenuItem>
                    <MenuItem value="200">200 OK</MenuItem>
                    <MenuItem value="201">201 Created</MenuItem>
                    <MenuItem value="400">400 Bad Request</MenuItem>
                    <MenuItem value="401">401 Unauthorized</MenuItem>
                    <MenuItem value="403">403 Forbidden</MenuItem>
                    <MenuItem value="404">404 Not Found</MenuItem>
                    <MenuItem value="500">500 Internal Server Error</MenuItem>
                  </Select>
                </FormControl>
              </Grid>
              <Grid item xs={12} md={4} />
              <Grid item xs={12} md={6}>
                <LocalizationProvider dateAdapter={AdapterDateFns}>
                  <DateTimePicker
                    label="Start Date"
                    value={startDate}
                    onChange={(newValue) => setStartDate(newValue)}
                    slotProps={{ textField: { fullWidth: true } }}
                  />
                </LocalizationProvider>
              </Grid>
              <Grid item xs={12} md={6}>
                <LocalizationProvider dateAdapter={AdapterDateFns}>
                  <DateTimePicker
                    label="End Date"
                    value={endDate}
                    onChange={(newValue) => setEndDate(newValue)}
                    slotProps={{ textField: { fullWidth: true } }}
                  />
                </LocalizationProvider>
              </Grid>
            </Grid>
            <Box sx={{ mt: 2, display: 'flex', gap: 1 }}>
              <Button
                variant="outlined"
                onClick={handleClearFilters}
              >
                Clear Filters
              </Button>
            </Box>
          </CardContent>
        </Card>

        {/* Audit Logs Table */}
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Timestamp</TableCell>
                <TableCell>User</TableCell>
                <TableCell>Action</TableCell>
                <TableCell>Resource</TableCell>
                <TableCell>Resource ID</TableCell>
                <TableCell>IP Address</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Duration</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading && (
                <TableRow>
                  <TableCell colSpan={9} align="center">Loading...</TableCell>
                </TableRow>
              )}
              {!isLoading && logs.length === 0 && (
                <TableRow>
                  <TableCell colSpan={9} align="center">No audit logs found</TableCell>
                </TableRow>
              )}
              {logs.map((log) => (
                <TableRow key={log.id} hover>
                  <TableCell>{formatTimestamp(log.timestamp)}</TableCell>
                  <TableCell>
                    {log.user_id || (
                      <Typography variant="body2" color="text.secondary">
                        Unauthenticated
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip label={log.action} size="small" />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>
                      {log.resource_type}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>
                      {log.resource_id || '-'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>
                      {log.ip_address}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    {log.changes?.status_code && (
                      <Chip
                        label={log.changes.status_code}
                        size="small"
                        color={getStatusCodeColor(log.changes.status_code)}
                      />
                    )}
                  </TableCell>
                  <TableCell>
                    {log.changes?.duration_ms && (
                      <Typography variant="body2">
                        {log.changes.duration_ms}ms
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Tooltip title="View Details">
                      <IconButton
                        size="small"
                        onClick={() => handleViewDetails(log)}
                        aria-label="View Details"
                      >
                        <VisibilityIcon />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>

        {/* Pagination */}
        {totalPages > 1 && (
          <Box sx={{ mt: 3, display: 'flex', justifyContent: 'center' }}>
            <Pagination
              count={totalPages}
              page={page}
              onChange={(_, value) => setPage(value)}
              color="primary"
            />
          </Box>
        )}

        {/* Detail Dialog */}
        <Dialog
          open={detailDialogOpen}
          onClose={() => setDetailDialogOpen(false)}
          maxWidth="md"
          fullWidth
        >
          <DialogTitle>Audit Log Details</DialogTitle>
          <DialogContent>
            {selectedLog && (
              <Box>
                <Typography variant="subtitle2" gutterBottom>
                  <strong>ID:</strong> {selectedLog.id}
                </Typography>
                <Typography variant="subtitle2" gutterBottom>
                  <strong>Timestamp:</strong> {formatTimestamp(selectedLog.timestamp)}
                </Typography>
                <Typography variant="subtitle2" gutterBottom>
                  <strong>User ID:</strong> {selectedLog.user_id || 'Unauthenticated'}
                </Typography>
                <Typography variant="subtitle2" gutterBottom>
                  <strong>Action:</strong> {selectedLog.action}
                </Typography>
                <Typography variant="subtitle2" gutterBottom>
                  <strong>Resource Type:</strong> {selectedLog.resource_type}
                </Typography>
                {selectedLog.resource_id && (
                  <Typography variant="subtitle2" gutterBottom>
                    <strong>Resource ID:</strong> {selectedLog.resource_id}
                  </Typography>
                )}
                <Typography variant="subtitle2" gutterBottom>
                  <strong>IP Address:</strong> {selectedLog.ip_address}
                </Typography>

                {selectedLog.changes && (
                  <Box sx={{ mt: 2 }}>
                    <Typography variant="subtitle2" gutterBottom>
                      <strong>Change Details:</strong>
                    </Typography>
                    <Paper
                      sx={{
                        p: 2,
                        bgcolor: 'grey.100',
                        fontFamily: 'monospace',
                        fontSize: '0.85rem',
                        maxHeight: 400,
                        overflow: 'auto',
                      }}
                    >
                      <pre>{JSON.stringify(selectedLog.changes, null, 2)}</pre>
                    </Paper>
                  </Box>
                )}
              </Box>
            )}
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setDetailDialogOpen(false)}>Close</Button>
          </DialogActions>
        </Dialog>
      </Container>
    </AdminPortalLayout>
  );
}

import { useState, useEffect } from 'react';
import {
  Box,
  Typography,
  Card,
  CardContent,
  Button,
  Chip,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  Switch,
  FormControlLabel,
  Grid,
  Alert,
  Tabs,
  Tab,
  Snackbar,
} from '@mui/material';
import {
  Schedule as ScheduleIcon,
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
  PlayArrow as RunIcon,
  Pause as PauseIcon,
  CalendarMonth as CalendarIcon,
  Link as LinkIcon,
  Wifi as ConnectedIcon,
  WifiOff as DisconnectedIcon,
} from '@mui/icons-material';
import Layout from '../components/Layout';
import api from '../lib/api';
import { toast } from '../lib/toast';
import { useScheduleEvents } from '../hooks/useEnterpriseWebSocket';
import { useNotificationQueue } from '../components/NotificationQueue';
import EnhancedWebSocketStatus from '../components/EnhancedWebSocketStatus';
import WebSocketErrorBoundary from '../components/WebSocketErrorBoundary';

interface ScheduledSession {
  id: number;
  name: string;
  template_id: string;
  schedule: {
    type: string;
    time_of_day?: string;
    days_of_week?: number[];
    day_of_month?: number;
    cron_expr?: string;
  };
  enabled: boolean;
  next_run_at: string;
  last_run_at?: string;
  last_run_status?: string;
}

interface CalendarIntegration {
  id: number;
  provider: string;
  account_email: string;
  enabled: boolean;
  sync_enabled: boolean;
  last_synced_at?: string;
}

function SchedulingContent() {
  const [currentTab, setCurrentTab] = useState(0);
  const [schedules, setSchedules] = useState<ScheduledSession[]>([]);
  const [calendarIntegrations, setCalendarIntegrations] = useState<CalendarIntegration[]>([]);
  const [scheduleDialog, setScheduleDialog] = useState(false);
  const [connectCalendarDialog, setConnectCalendarDialog] = useState(false);
  const [loading, setLoading] = useState(false);
  const [wsConnected, setWsConnected] = useState(false);
  const [wsReconnectAttempts, setWsReconnectAttempts] = useState(0);

  // Enhanced notification system
  const { addNotification } = useNotificationQueue();

  // Real-time schedule events via WebSocket
  useScheduleEvents((data: any) => {
    console.log('Schedule event:', data);
    setWsConnected(true);
    setWsReconnectAttempts(0);

    // Show notification for schedule events
    if (data.event_type === 'schedule.started') {
      addNotification({
        message: `Scheduled session started: ${data.schedule_name || 'Unknown'}`,
        severity: 'success',
        priority: 'medium',
        title: 'Schedule Started',
      });
    } else if (data.event_type === 'schedule.completed') {
      addNotification({
        message: `Scheduled session completed: ${data.schedule_name || 'Unknown'}`,
        severity: 'success',
        priority: 'low',
        title: 'Schedule Completed',
      });
    } else if (data.event_type === 'schedule.failed') {
      addNotification({
        message: `Scheduled session failed: ${data.schedule_name || 'Unknown'} - ${data.error || 'Unknown error'}`,
        severity: 'error',
        priority: 'high',
        title: 'Schedule Failed',
        autoDismiss: false,
      });
    } else if (data.event_type === 'schedule.created' || data.event_type === 'schedule.updated') {
      addNotification({
        message: `Schedule ${data.event_type === 'schedule.created' ? 'created' : 'updated'}: ${data.schedule_name || 'Unknown'}`,
        severity: 'info',
        priority: 'low',
        title: data.event_type === 'schedule.created' ? 'Schedule Created' : 'Schedule Updated',
      });
    }

    // Refresh schedules
    loadSchedules();
  });

  const [scheduleForm, setScheduleForm] = useState({
    name: '',
    template_id: '',
    schedule_type: 'daily',
    time_of_day: '09:00',
    days_of_week: [] as number[],
    day_of_month: 1,
    cron_expr: '',
    timezone: 'UTC',
    auto_terminate: false,
    terminate_after: 480,
    pre_warm: false,
    pre_warm_minutes: 5,
  });

  // Load initial data
  useEffect(() => {
    loadSchedules();
    loadCalendarIntegrations();
  }, []);

  const loadSchedules = async () => {
    try {
      const response = await api.listScheduledSessions();
      setSchedules(response.schedules);
    } catch (error) {
      console.error('Failed to load schedules:', error);
    }
  };

  const loadCalendarIntegrations = async () => {
    try {
      const response = await api.listCalendarIntegrations();
      setCalendarIntegrations(response.integrations);
    } catch (error) {
      console.error('Failed to load calendar integrations:', error);
    }
  };

  const handleCreateSchedule = async () => {
    setLoading(true);
    try {
      const requestData = {
        name: scheduleForm.name,
        template_id: scheduleForm.template_id,
        timezone: scheduleForm.timezone,
        schedule: {
          type: scheduleForm.schedule_type as any,
          time_of_day: scheduleForm.time_of_day,
          days_of_week: scheduleForm.days_of_week,
          day_of_month: scheduleForm.day_of_month,
          cron_expr: scheduleForm.cron_expr,
        },
        auto_terminate: scheduleForm.auto_terminate,
        terminate_after: scheduleForm.terminate_after,
        pre_warm: scheduleForm.pre_warm,
        pre_warm_minutes: scheduleForm.pre_warm_minutes,
      };

      await api.createScheduledSession(requestData);
      toast.success('Scheduled session created successfully');
      setScheduleDialog(false);
      loadSchedules();
    } catch (error) {
      toast.error('Failed to create scheduled session');
    } finally {
      setLoading(false);
    }
  };

  const handleToggleSchedule = async (id: number, enabled: boolean) => {
    setLoading(true);
    try {
      if (enabled) {
        await api.disableScheduledSession(id);
        toast.success('Schedule disabled');
      } else {
        await api.enableScheduledSession(id);
        toast.success('Schedule enabled');
      }
      loadSchedules();
    } catch (error) {
      toast.error('Failed to toggle schedule');
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteSchedule = async (id: number) => {
    if (!confirm('Are you sure you want to delete this schedule?')) return;

    setLoading(true);
    try {
      await api.deleteScheduledSession(id);
      toast.success('Schedule deleted');
      loadSchedules();
    } catch (error) {
      toast.error('Failed to delete schedule');
    } finally {
      setLoading(false);
    }
  };

  const handleConnectCalendar = async (provider: 'google' | 'outlook') => {
    setLoading(true);
    try {
      const response = await api.connectCalendar(provider);
      toast.success(response.message);
      // Redirect to OAuth URL
      if (response.auth_url) {
        window.location.href = response.auth_url;
      }
      setConnectCalendarDialog(false);
    } catch (error) {
      toast.error('Failed to connect calendar');
    } finally {
      setLoading(false);
    }
  };

  const handleDisconnectCalendar = async (id: number) => {
    if (!confirm('Are you sure you want to disconnect this calendar?')) return;

    setLoading(true);
    try {
      await api.disconnectCalendar(id);
      toast.success('Calendar disconnected');
      loadCalendarIntegrations();
    } catch (error) {
      toast.error('Failed to disconnect calendar');
    } finally {
      setLoading(false);
    }
  };

  const handleSyncCalendar = async (id: number) => {
    setLoading(true);
    try {
      await api.syncCalendar(id);
      toast.success('Calendar synced successfully');
      loadCalendarIntegrations();
    } catch (error) {
      toast.error('Failed to sync calendar');
    } finally {
      setLoading(false);
    }
  };

  const handleExportICal = async () => {
    setLoading(true);
    try {
      const blob = await api.exportICalendar();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'streamspace-schedule.ics';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
      toast.success('iCalendar file downloaded');
    } catch (error) {
      toast.error('Failed to export calendar');
    } finally {
      setLoading(false);
    }
  };

  const getDayName = (day: number) => {
    const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    return days[day];
  };

  const getScheduleDescription = (schedule: ScheduledSession['schedule']) => {
    switch (schedule.type) {
      case 'once':
        return 'One-time';
      case 'daily':
        return `Daily at ${schedule.time_of_day}`;
      case 'weekly':
        return `Weekly on ${schedule.days_of_week?.map(getDayName).join(', ')} at ${schedule.time_of_day}`;
      case 'monthly':
        return `Monthly on day ${schedule.day_of_month} at ${schedule.time_of_day}`;
      case 'cron':
        return `Cron: ${schedule.cron_expr}`;
      default:
        return 'Unknown';
    }
  };

  return (
    <Layout>
      <Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
            <Typography variant="h4" sx={{ fontWeight: 700 }}>
              Session Scheduling
            </Typography>
            <EnhancedWebSocketStatus
              isConnected={wsConnected}
              reconnectAttempts={wsReconnectAttempts}
              size="small"
            />
          </Box>
          <Box sx={{ display: 'flex', gap: 1 }}>
            <Button variant="outlined" startIcon={<CalendarIcon />} onClick={handleExportICal}>
              Export iCal
            </Button>
            <Button variant="contained" startIcon={<AddIcon />} onClick={() => setScheduleDialog(true)}>
              New Schedule
            </Button>
          </Box>
        </Box>

        <Tabs value={currentTab} onChange={(_, v) => setCurrentTab(v)} sx={{ mb: 3 }}>
          <Tab label="Scheduled Sessions" />
          <Tab label="Calendar Integration" />
        </Tabs>

        {/* Scheduled Sessions Tab */}
        {currentTab === 0 && (
          <Card>
            <CardContent>
              <TableContainer>
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>Name</TableCell>
                      <TableCell>Schedule</TableCell>
                      <TableCell>Next Run</TableCell>
                      <TableCell>Last Run</TableCell>
                      <TableCell>Status</TableCell>
                      <TableCell>Actions</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {schedules.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} align="center">
                          <Typography color="text.secondary">No scheduled sessions</Typography>
                        </TableCell>
                      </TableRow>
                    ) : (
                      schedules.map((schedule) => (
                        <TableRow key={schedule.id}>
                          <TableCell>{schedule.name}</TableCell>
                          <TableCell>{getScheduleDescription(schedule.schedule)}</TableCell>
                          <TableCell>
                            {schedule.next_run_at ? new Date(schedule.next_run_at).toLocaleString() : '-'}
                          </TableCell>
                          <TableCell>
                            {schedule.last_run_at ? (
                              <Box>
                                <Typography variant="body2">
                                  {new Date(schedule.last_run_at).toLocaleString()}
                                </Typography>
                                {schedule.last_run_status && (
                                  <Chip
                                    label={schedule.last_run_status}
                                    size="small"
                                    color={schedule.last_run_status === 'success' ? 'success' : 'error'}
                                  />
                                )}
                              </Box>
                            ) : (
                              '-'
                            )}
                          </TableCell>
                          <TableCell>
                            <Chip
                              label={schedule.enabled ? 'Enabled' : 'Disabled'}
                              color={schedule.enabled ? 'success' : 'default'}
                              size="small"
                            />
                          </TableCell>
                          <TableCell>
                            <IconButton
                              size="small"
                              onClick={() => handleToggleSchedule(schedule.id, schedule.enabled)}
                              title={schedule.enabled ? 'Disable' : 'Enable'}
                            >
                              {schedule.enabled ? <PauseIcon /> : <RunIcon />}
                            </IconButton>
                            <IconButton size="small" title="Delete" onClick={() => handleDeleteSchedule(schedule.id)}>
                              <DeleteIcon />
                            </IconButton>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </CardContent>
          </Card>
        )}

        {/* Calendar Integration Tab */}
        {currentTab === 1 && (
          <Box>
            <Grid container spacing={3}>
              <Grid item xs={12}>
                <Alert severity="info">
                  Connect your calendar to automatically sync scheduled sessions. Sessions will appear as events in your
                  calendar.
                </Alert>
              </Grid>

              {calendarIntegrations.length === 0 ? (
                <Grid item xs={12}>
                  <Card>
                    <CardContent sx={{ textAlign: 'center', py: 4 }}>
                      <CalendarIcon sx={{ fontSize: 60, color: 'text.secondary', mb: 2 }} />
                      <Typography variant="h6" sx={{ mb: 1 }}>
                        No Calendar Connected
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                        Connect Google Calendar or Outlook to sync your scheduled sessions
                      </Typography>
                      <Button variant="contained" onClick={() => setConnectCalendarDialog(true)}>
                        Connect Calendar
                      </Button>
                    </CardContent>
                  </Card>
                </Grid>
              ) : (
                calendarIntegrations.map((integration) => (
                  <Grid item xs={12} md={6} key={integration.id}>
                    <Card>
                      <CardContent>
                        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
                          <Typography variant="h6">{integration.provider}</Typography>
                          <Chip
                            label={integration.enabled ? 'Connected' : 'Disconnected'}
                            color={integration.enabled ? 'success' : 'default'}
                            size="small"
                          />
                        </Box>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                          {integration.account_email}
                        </Typography>
                        {integration.last_synced_at && (
                          <Typography variant="caption" color="text.secondary">
                            Last synced: {new Date(integration.last_synced_at).toLocaleString()}
                          </Typography>
                        )}
                        <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
                          <Button size="small" variant="outlined" onClick={() => handleSyncCalendar(integration.id)}>
                            Sync Now
                          </Button>
                          <Button
                            size="small"
                            variant="outlined"
                            color="error"
                            onClick={() => handleDisconnectCalendar(integration.id)}
                          >
                            Disconnect
                          </Button>
                        </Box>
                      </CardContent>
                    </Card>
                  </Grid>
                ))
              )}

              {calendarIntegrations.length > 0 && (
                <Grid item xs={12}>
                  <Button variant="outlined" onClick={() => setConnectCalendarDialog(true)} startIcon={<AddIcon />}>
                    Connect Another Calendar
                  </Button>
                </Grid>
              )}
            </Grid>
          </Box>
        )}

        {/* Create Schedule Dialog */}
        <Dialog open={scheduleDialog} onClose={() => setScheduleDialog(false)} maxWidth="md" fullWidth>
          <DialogTitle>Create Scheduled Session</DialogTitle>
          <DialogContent>
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
              <TextField
                label="Name"
                fullWidth
                value={scheduleForm.name}
                onChange={(e) => setScheduleForm({ ...scheduleForm, name: e.target.value })}
                placeholder="e.g., Daily standup session"
              />

              <FormControl fullWidth>
                <InputLabel>Template</InputLabel>
                <Select
                  value={scheduleForm.template_id}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, template_id: e.target.value })}
                >
                  <MenuItem value="firefox">Firefox</MenuItem>
                  <MenuItem value="vscode">VS Code</MenuItem>
                  <MenuItem value="ubuntu">Ubuntu Desktop</MenuItem>
                </Select>
              </FormControl>

              <FormControl fullWidth>
                <InputLabel>Schedule Type</InputLabel>
                <Select
                  value={scheduleForm.schedule_type}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, schedule_type: e.target.value })}
                >
                  <MenuItem value="once">One-time</MenuItem>
                  <MenuItem value="daily">Daily</MenuItem>
                  <MenuItem value="weekly">Weekly</MenuItem>
                  <MenuItem value="monthly">Monthly</MenuItem>
                  <MenuItem value="cron">Cron Expression</MenuItem>
                </Select>
              </FormControl>

              {(scheduleForm.schedule_type === 'daily' ||
                scheduleForm.schedule_type === 'weekly' ||
                scheduleForm.schedule_type === 'monthly') && (
                <TextField
                  label="Time of Day"
                  type="time"
                  fullWidth
                  value={scheduleForm.time_of_day}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, time_of_day: e.target.value })}
                  InputLabelProps={{ shrink: true }}
                />
              )}

              {scheduleForm.schedule_type === 'weekly' && (
                <FormControl fullWidth>
                  <InputLabel>Days of Week</InputLabel>
                  <Select
                    multiple
                    value={scheduleForm.days_of_week}
                    onChange={(e) =>
                      setScheduleForm({
                        ...scheduleForm,
                        days_of_week: typeof e.target.value === 'string' ? [] : e.target.value,
                      })
                    }
                    renderValue={(selected) => selected.map(getDayName).join(', ')}
                  >
                    {['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'].map((day, index) => (
                      <MenuItem key={index} value={index}>
                        {day}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              )}

              {scheduleForm.schedule_type === 'monthly' && (
                <TextField
                  label="Day of Month"
                  type="number"
                  fullWidth
                  value={scheduleForm.day_of_month}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, day_of_month: parseInt(e.target.value) })}
                  InputProps={{ inputProps: { min: 1, max: 31 } }}
                />
              )}

              {scheduleForm.schedule_type === 'cron' && (
                <TextField
                  label="Cron Expression"
                  fullWidth
                  value={scheduleForm.cron_expr}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, cron_expr: e.target.value })}
                  placeholder="0 9 * * *"
                  helperText="Use cron syntax (minute hour day month weekday)"
                />
              )}

              <FormControl fullWidth>
                <InputLabel>Timezone</InputLabel>
                <Select
                  value={scheduleForm.timezone}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, timezone: e.target.value })}
                >
                  <MenuItem value="UTC">UTC</MenuItem>
                  <MenuItem value="America/New_York">Eastern Time</MenuItem>
                  <MenuItem value="America/Chicago">Central Time</MenuItem>
                  <MenuItem value="America/Los_Angeles">Pacific Time</MenuItem>
                  <MenuItem value="Europe/London">London</MenuItem>
                </Select>
              </FormControl>

              <FormControlLabel
                control={
                  <Switch
                    checked={scheduleForm.auto_terminate}
                    onChange={(e) => setScheduleForm({ ...scheduleForm, auto_terminate: e.target.checked })}
                  />
                }
                label="Auto-terminate after duration"
              />

              {scheduleForm.auto_terminate && (
                <TextField
                  label="Terminate After (minutes)"
                  type="number"
                  fullWidth
                  value={scheduleForm.terminate_after}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, terminate_after: parseInt(e.target.value) })}
                />
              )}

              <FormControlLabel
                control={
                  <Switch
                    checked={scheduleForm.pre_warm}
                    onChange={(e) => setScheduleForm({ ...scheduleForm, pre_warm: e.target.checked })}
                  />
                }
                label="Pre-warm session before scheduled time"
              />

              {scheduleForm.pre_warm && (
                <TextField
                  label="Pre-warm Minutes"
                  type="number"
                  fullWidth
                  value={scheduleForm.pre_warm_minutes}
                  onChange={(e) => setScheduleForm({ ...scheduleForm, pre_warm_minutes: parseInt(e.target.value) })}
                />
              )}
            </Box>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setScheduleDialog(false)}>Cancel</Button>
            <Button variant="contained" onClick={handleCreateSchedule}>
              Create
            </Button>
          </DialogActions>
        </Dialog>

        {/* Connect Calendar Dialog */}
        <Dialog open={connectCalendarDialog} onClose={() => setConnectCalendarDialog(false)} maxWidth="sm" fullWidth>
          <DialogTitle>Connect Calendar</DialogTitle>
          <DialogContent>
            <Grid container spacing={2} sx={{ pt: 1 }}>
              <Grid item xs={12}>
                <Button
                  variant="outlined"
                  fullWidth
                  sx={{ py: 2 }}
                  startIcon={<CalendarIcon />}
                  onClick={() => handleConnectCalendar('google')}
                >
                  Connect Google Calendar
                </Button>
              </Grid>
              <Grid item xs={12}>
                <Button
                  variant="outlined"
                  fullWidth
                  sx={{ py: 2 }}
                  startIcon={<CalendarIcon />}
                  onClick={() => handleConnectCalendar('outlook')}
                >
                  Connect Outlook Calendar
                </Button>
              </Grid>
            </Grid>
          </DialogContent>
          <DialogActions>
            <Button onClick={() => setConnectCalendarDialog(false)}>Cancel</Button>
          </DialogActions>
        </Dialog>

      </Box>
    </Layout>
  );
}

export default function Scheduling() {
  return (
    <WebSocketErrorBoundary>
      <SchedulingContent />
    </WebSocketErrorBoundary>
  );
}

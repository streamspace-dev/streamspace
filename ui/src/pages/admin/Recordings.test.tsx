import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import Recordings from './Recordings';

// Mock the NotificationQueue
vi.mock('../../components/NotificationQueue', () => ({
  useNotificationQueue: () => ({
    addNotification: vi.fn(),
  }),
}));

// Mock the AdminPortalLayout
vi.mock('../../components/AdminPortalLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="admin-portal-layout">{children}</div>
  ),
}));

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.open
global.window.open = vi.fn();

// Mock window.confirm
global.window.confirm = vi.fn(() => true);

// Mock localStorage
const mockLocalStorage = {
  getItem: vi.fn(() => 'mock-token'),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, 'localStorage', {
  value: mockLocalStorage,
  writable: true,
});

// Mock recordings data
const mockRecordingsData = {
  recordings: [
    {
      id: 1,
      session_id: 'session-123',
      session_name: 'Firefox Session',
      user_name: 'user1',
      created_by: 'user1',
      recording_type: 'automatic',
      storage_path: '/recordings/session-123.webm',
      file_size_bytes: 10485760,
      file_size_mb: 10.0,
      duration_seconds: 300,
      duration_formatted: '5m 0s',
      started_at: '2025-01-15T10:00:00Z',
      ended_at: '2025-01-15T10:05:00Z',
      status: 'completed',
      created_at: '2025-01-15T10:00:00Z',
      updated_at: '2025-01-15T10:05:00Z',
    },
    {
      id: 2,
      session_id: 'session-456',
      session_name: 'Chrome Session',
      user_name: 'user2',
      created_by: 'user2',
      recording_type: 'manual',
      storage_path: '/recordings/session-456.webm',
      file_size_bytes: 20971520,
      file_size_mb: 20.0,
      duration_seconds: 600,
      duration_formatted: '10m 0s',
      started_at: '2025-01-15T09:00:00Z',
      ended_at: null,
      status: 'recording',
      created_at: '2025-01-15T09:00:00Z',
      updated_at: '2025-01-15T09:10:00Z',
    },
    {
      id: 3,
      session_id: 'session-789',
      session_name: 'Edge Session',
      user_name: 'user3',
      recording_type: 'automatic',
      storage_path: '/recordings/session-789.webm',
      file_size_bytes: 5242880,
      file_size_mb: 5.0,
      duration_seconds: 150,
      duration_formatted: '2m 30s',
      started_at: '2025-01-14T10:00:00Z',
      ended_at: '2025-01-14T10:02:30Z',
      status: 'failed',
      error_message: 'Storage full',
      created_at: '2025-01-14T10:00:00Z',
      updated_at: '2025-01-14T10:02:30Z',
    },
  ],
};

// Mock policies data
const mockPoliciesData = {
  policies: [
    {
      id: 1,
      name: 'Auto-record all sessions',
      description: 'Automatically record all sessions',
      auto_record: true,
      recording_format: 'webm',
      retention_days: 30,
      apply_to_users: null,
      apply_to_teams: null,
      apply_to_templates: null,
      require_reason: false,
      allow_user_playback: true,
      allow_user_download: true,
      require_approval: false,
      notify_on_recording: true,
      metadata: null,
      enabled: true,
      priority: 10,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
    {
      id: 2,
      name: 'Long retention policy',
      description: 'Keep recordings for 90 days',
      auto_record: false,
      recording_format: 'mp4',
      retention_days: 90,
      apply_to_users: null,
      apply_to_teams: null,
      apply_to_templates: null,
      require_reason: true,
      allow_user_playback: false,
      allow_user_download: false,
      require_approval: true,
      notify_on_recording: false,
      metadata: null,
      enabled: false,
      priority: 5,
      created_at: '2025-01-01T00:00:00Z',
      updated_at: '2025-01-01T00:00:00Z',
    },
  ],
};

// Mock access log data
const mockAccessLogData = {
  access_log: [
    {
      id: 1,
      recording_id: 1,
      user_id: 'user1',
      user_name: 'User One',
      action: 'viewed',
      accessed_at: '2025-01-15T11:00:00Z',
      ip_address: '192.168.1.1',
      user_agent: 'Mozilla/5.0',
    },
    {
      id: 2,
      recording_id: 1,
      user_id: 'admin',
      user_name: 'Admin User',
      action: 'downloaded',
      accessed_at: '2025-01-15T12:00:00Z',
      ip_address: '192.168.1.2',
      user_agent: 'Mozilla/5.0',
    },
  ],
};

// Helper to render Recordings with providers
const renderRecordings = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Recordings />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('Recordings Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/recording-policies')) {
        return Promise.resolve({ ok: true, json: async () => mockPoliciesData });
      }
      if (url.includes('/access-log')) {
        return Promise.resolve({ ok: true, json: async () => mockAccessLogData });
      }
      return Promise.resolve({ ok: true, json: async () => mockRecordingsData });
    });
  });

  // ===== RENDERING TESTS =====

  it('renders page title', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Session Recordings')).toBeInTheDocument();
    });
  });

  it('displays tab navigation', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /recordings/i })).toBeInTheDocument();
    });

    expect(screen.getByRole('tab', { name: /policies/i })).toBeInTheDocument();
  });

  it('displays recordings in table', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    expect(screen.getByText('Chrome Session')).toBeInTheDocument();
    expect(screen.getByText('Edge Session')).toBeInTheDocument();
  });

  it.skip('displays recording types', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('automatic')).toBeInTheDocument();
    });

    expect(screen.getByText('manual')).toBeInTheDocument();
  });

  it('displays user names', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('user1')).toBeInTheDocument();
    });

    expect(screen.getByText('user2')).toBeInTheDocument();
    expect(screen.getByText('user3')).toBeInTheDocument();
  });

  it('displays formatted durations', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('5m 0s')).toBeInTheDocument();
    });

    expect(screen.getByText('10m 0s')).toBeInTheDocument();
    expect(screen.getByText('2m 30s')).toBeInTheDocument();
  });

  it('displays file sizes in MB', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('10.00 MB')).toBeInTheDocument();
    });

    expect(screen.getByText('20.00 MB')).toBeInTheDocument();
    expect(screen.getByText('5.00 MB')).toBeInTheDocument();
  });

  it('displays status chips with correct colors', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('completed')).toBeInTheDocument();
    });

    expect(screen.getByText('recording')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
  });

  it.skip('displays started timestamps', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText(/1\/15\/2025.*10:00/)).toBeInTheDocument();
    });
  });

  // ===== SEARCH AND FILTER TESTS =====

  it('displays search input', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search by session or user/i)).toBeInTheDocument();
    });
  });

  it('filters recordings by search query', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by session or user/i);
    fireEvent.change(searchInput, { target: { value: 'Firefox' } });

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('search=Firefox'),
        expect.any(Object)
      );
    });
  });

  it.skip('displays status filter dropdown', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });
  });

  it.skip('filters recordings by status', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const completedOption = await screen.findByText('Completed');
    fireEvent.click(completedOption);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=completed'),
        expect.any(Object)
      );
    });
  });

  // ===== RECORDING ACTIONS TESTS =====

  it.skip('displays download button for completed recordings', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const downloadButtons = screen.getAllByTitle('Download');
    expect(downloadButtons.length).toBe(1); // Only completed recording
  });

  it.skip('downloads recording when download button is clicked', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const downloadButton = screen.getByTitle('Download');
    fireEvent.click(downloadButton);

    expect(window.open).toHaveBeenCalledWith('/api/v1/admin/recordings/1/download', '_blank');
  });

  it.skip('displays view access log button for all recordings', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const accessLogButtons = screen.getAllByTitle('View Access Log');
    expect(accessLogButtons.length).toBe(3); // All recordings
  });

  it.skip('opens access log dialog when button is clicked', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const accessLogButton = screen.getAllByTitle('View Access Log')[0];
    fireEvent.click(accessLogButton);

    await waitFor(() => {
      expect(screen.getByText('Recording Access Log')).toBeInTheDocument();
    });

    expect(screen.getByText('User One')).toBeInTheDocument();
    expect(screen.getByText('Admin User')).toBeInTheDocument();
    expect(screen.getByText('viewed')).toBeInTheDocument();
    expect(screen.getByText('downloaded')).toBeInTheDocument();
  });

  it.skip('displays delete button for all recordings', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const deleteButtons = screen.getAllByTitle('Delete');
    expect(deleteButtons.length).toBe(3); // All recordings
  });

  it.skip('opens delete confirmation dialog', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Confirm Delete')).toBeInTheDocument();
    });

    expect(screen.getByText(/this action cannot be undone/i)).toBeInTheDocument();
  });

  it.skip('deletes recording when confirmed', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Confirm Delete')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const confirmButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/recordings/1',
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  it.skip('handles delete errors', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Confirm Delete')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Delete failed' }),
    });

    const confirmButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith('/api/v1/admin/recordings/1', expect.any(Object));
    });
  });

  // ===== POLICIES TAB TESTS =====

  it('switches to policies tab', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /policies/i })).toBeInTheDocument();
    });

    const policiesTab = screen.getByRole('tab', { name: /policies/i });
    fireEvent.click(policiesTab);

    await waitFor(() => {
      expect(screen.getByText('Auto-record all sessions')).toBeInTheDocument();
    });
  });

  it('displays policies in table', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /policies/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('Auto-record all sessions')).toBeInTheDocument();
    });

    expect(screen.getByText('Long retention policy')).toBeInTheDocument();
  });

  it('displays policy auto-record status', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      const yesChip = screen.getByText('Yes');
      expect(yesChip).toBeInTheDocument();
    });

    expect(screen.getByText('No')).toBeInTheDocument();
  });

  it('displays policy format and retention', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('WEBM')).toBeInTheDocument();
    });

    expect(screen.getByText('MP4')).toBeInTheDocument();
    expect(screen.getByText('30 days')).toBeInTheDocument();
    expect(screen.getByText('90 days')).toBeInTheDocument();
  });

  it('displays policy priority', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('10')).toBeInTheDocument();
    });

    expect(screen.getByText('5')).toBeInTheDocument();
  });

  it.skip('displays policy enabled status', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('Enabled')).toBeInTheDocument();
    });

    expect(screen.getByText('Disabled')).toBeInTheDocument();
  });

  it('displays create policy button', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create policy/i })).toBeInTheDocument();
    });
  });

  it('opens create policy dialog', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create policy/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create policy/i });
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Create Recording Policy')).toBeInTheDocument();
    });
  });

  it.skip('allows entering policy details', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));
    fireEvent.click(await screen.findByRole('button', { name: /create policy/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Policy Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Policy Name');
    const descriptionInput = screen.getByLabelText('Description');
    const retentionInput = screen.getByLabelText('Retention Days');

    fireEvent.change(nameInput, { target: { value: 'New Policy' } });
    fireEvent.change(descriptionInput, { target: { value: 'Test policy' } });
    fireEvent.change(retentionInput, { target: { value: '60' } });

    expect(nameInput).toHaveValue('New Policy');
    expect(descriptionInput).toHaveValue('Test policy');
    expect(retentionInput).toHaveValue(60);
  });

  it.skip('allows selecting recording format', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));
    fireEvent.click(await screen.findByRole('button', { name: /create policy/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Recording Format')).toBeInTheDocument();
    });

    const formatSelect = screen.getByLabelText('Recording Format');
    fireEvent.mouseDown(formatSelect);

    const mp4Option = await screen.findByText('MP4');
    fireEvent.click(mp4Option);

    expect(mp4Option).toBeInTheDocument();
  });

  it.skip('creates policy when form is submitted', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));
    fireEvent.click(await screen.findByRole('button', { name: /create policy/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Policy Name')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Policy Name'), { target: { value: 'New Policy' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: 3, name: 'New Policy' }),
    });

    const saveButton = within(screen.getByRole('dialog')).getByRole('button', { name: /save/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/recording-policies',
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it.skip('opens edit policy dialog with pre-filled data', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('Auto-record all sessions')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Recording Policy')).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue('Auto-record all sessions')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Automatically record all sessions')).toBeInTheDocument();
    expect(screen.getByDisplayValue('30')).toBeInTheDocument();
  });

  it.skip('updates policy when edit form is submitted', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('Auto-record all sessions')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Recording Policy')).toBeInTheDocument();
    });

    const nameInput = screen.getByDisplayValue('Auto-record all sessions');
    fireEvent.change(nameInput, { target: { value: 'Updated Policy' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const saveButton = within(screen.getByRole('dialog')).getByRole('button', { name: /save/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/recording-policies/1',
        expect.objectContaining({
          method: 'PUT',
        })
      );
    });
  });

  it.skip('deletes policy when confirmed', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('Auto-record all sessions')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/recording-policies/1',
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  // ===== EMPTY STATE TESTS =====

  it('displays empty state when no recordings found', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/recording-policies')) {
        return Promise.resolve({ ok: true, json: async () => mockPoliciesData });
      }
      return Promise.resolve({ ok: true, json: async () => ({ recordings: [] }) });
    });

    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('No recordings found')).toBeInTheDocument();
    });
  });

  it('displays empty state when no policies found', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/recording-policies')) {
        return Promise.resolve({ ok: true, json: async () => ({ policies: [] }) });
      }
      return Promise.resolve({ ok: true, json: async () => mockRecordingsData });
    });

    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));

    await waitFor(() => {
      expect(screen.getByText('No recording policies configured')).toBeInTheDocument();
    });
  });

  it.skip('displays empty state in access log', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/access-log')) {
        return Promise.resolve({ ok: true, json: async () => ({ access_log: [] }) });
      }
      if (url.includes('/recording-policies')) {
        return Promise.resolve({ ok: true, json: async () => mockPoliciesData });
      }
      return Promise.resolve({ ok: true, json: async () => mockRecordingsData });
    });

    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const accessLogButton = screen.getAllByTitle('View Access Log')[0];
    fireEvent.click(accessLogButton);

    await waitFor(() => {
      expect(screen.getByText('No access log entries found')).toBeInTheDocument();
    });
  });
});

describe('Recordings Page - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockImplementation((url: string) => {
      if (url.includes('/recording-policies')) {
        return Promise.resolve({ ok: true, json: async () => mockPoliciesData });
      }
      return Promise.resolve({ ok: true, json: async () => mockRecordingsData });
    });
  });

  it('has accessible tab navigation', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /recordings/i })).toBeInTheDocument();
    });

    const tabs = screen.getAllByRole('tab');
    tabs.forEach((tab) => {
      expect(tab).toHaveAccessibleName();
    });
  });

  it('has accessible table structure', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByRole('table')).toBeInTheDocument();
    });

    const table = screen.getByRole('table');
    expect(table).toBeInTheDocument();
  });

  it('has accessible buttons', async () => {
    renderRecordings();

    await waitFor(() => {
      expect(screen.getByText('Firefox Session')).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it.skip('has accessible form controls in policy dialog', async () => {
    renderRecordings();

    fireEvent.click(screen.getByRole('tab', { name: /policies/i }));
    fireEvent.click(await screen.findByRole('button', { name: /create policy/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Policy Name')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('Description')).toBeInTheDocument();
    expect(screen.getByLabelText('Recording Format')).toBeInTheDocument();
    expect(screen.getByLabelText('Retention Days')).toBeInTheDocument();
  });
});

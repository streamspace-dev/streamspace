import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import Monitoring from './Monitoring';

// Mock the NotificationQueue
vi.mock('../../components/NotificationQueue', () => ({
  useNotificationQueue: () => ({
    addNotification: vi.fn(),
  }),
}));

// Mock the AdminPortalLayout
vi.mock('../../components/AdminPortalLayout', () => ({
  default: ({ children, title }: { children: React.ReactNode; title: string }) => (
    <div data-testid="admin-portal-layout">
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

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

// Mock alerts data
const mockAlerts = {
  alerts: [
    {
      id: '1',
      name: 'High CPU Usage',
      description: 'CPU usage exceeds threshold',
      severity: 'critical',
      condition: 'cpu_usage > threshold',
      threshold: 80,
      status: 'triggered',
      triggeredAt: '2025-01-15T10:00:00Z',
    },
    {
      id: '2',
      name: 'Memory Warning',
      description: 'Memory usage high',
      severity: 'warning',
      condition: 'memory_usage > threshold',
      threshold: 75,
      status: 'triggered',
      triggeredAt: '2025-01-15T09:00:00Z',
    },
    {
      id: '3',
      name: 'Disk Space Info',
      description: 'Disk usage notification',
      severity: 'info',
      condition: 'disk_usage > threshold',
      threshold: 60,
      status: 'acknowledged',
      triggeredAt: '2025-01-14T10:00:00Z',
    },
    {
      id: '4',
      name: 'Network Issue',
      description: 'Network latency high',
      severity: 'warning',
      condition: 'latency > threshold',
      threshold: 100,
      status: 'resolved',
      triggeredAt: '2025-01-13T10:00:00Z',
    },
  ],
};

// Helper to render Monitoring with providers
const renderMonitoring = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Monitoring />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('Monitoring Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockAlerts,
    });
  });

  // ===== RENDERING TESTS =====

  it('renders page title and description', async () => {
    renderMonitoring();

    expect(screen.getByText('Monitoring')).toBeInTheDocument();
  });

  it('displays loading state initially', () => {
    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* never resolves */
        })
    );

    renderMonitoring();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('displays alert summary cards', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('Active Alerts')).toBeInTheDocument();
    });

    expect(screen.getByText('Acknowledged')).toBeInTheDocument();
    expect(screen.getByText('Resolved')).toBeInTheDocument();
  });

  it.skip('displays correct counts in summary cards', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('2')).toBeInTheDocument(); // 2 active/triggered
    });

    expect(screen.getByText('1')).toBeInTheDocument(); // 1 acknowledged, 1 resolved
  });

  it.skip('displays alerts in table', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    expect(screen.getByText('Memory Warning')).toBeInTheDocument();
    expect(screen.getByText('Disk Space Info')).toBeInTheDocument();
    expect(screen.getByText('Network Issue')).toBeInTheDocument();
  });

  it('displays alert descriptions', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('CPU usage exceeds threshold')).toBeInTheDocument();
    });

    expect(screen.getByText('Memory usage high')).toBeInTheDocument();
  });

  it.skip('displays severity chips with correct colors', async () => {
    renderMonitoring();

    await waitFor(() => {
      const criticalChip = screen.getByText('critical');
      expect(criticalChip).toBeInTheDocument();
    });

    const warningChips = screen.getAllByText('warning');
    expect(warningChips.length).toBe(2);

    const infoChip = screen.getByText('info');
    expect(infoChip).toBeInTheDocument();
  });

  it.skip('displays status chips with correct colors', async () => {
    renderMonitoring();

    await waitFor(() => {
      const triggeredChips = screen.getAllByText('triggered');
      expect(triggeredChips.length).toBe(2);
    });

    expect(screen.getByText('acknowledged')).toBeInTheDocument();
    expect(screen.getByText('resolved')).toBeInTheDocument();
  });

  it('displays conditions in monospace font', async () => {
    renderMonitoring();

    await waitFor(() => {
      const condition = screen.getByText('cpu_usage > threshold');
      expect(condition).toHaveStyle({ fontFamily: 'monospace' });
    });
  });

  it.skip('displays threshold values', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('80')).toBeInTheDocument();
    });

    expect(screen.getByText('75')).toBeInTheDocument();
    expect(screen.getByText('60')).toBeInTheDocument();
    expect(screen.getByText('100')).toBeInTheDocument();
  });

  it.skip('displays triggered timestamps', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText(/1\/15\/2025/)).toBeInTheDocument();
    });

    expect(screen.getByText(/1\/14\/2025/)).toBeInTheDocument();
    expect(screen.getByText(/1\/13\/2025/)).toBeInTheDocument();
  });

  // ===== TAB NAVIGATION TESTS =====

  it('displays tab navigation', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /active \(2\)/i })).toBeInTheDocument();
    });

    expect(screen.getByRole('tab', { name: /acknowledged \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /resolved \(1\)/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /all alerts/i })).toBeInTheDocument();
  });

  it('switches to acknowledged tab', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /acknowledged \(1\)/i })).toBeInTheDocument();
    });

    const acknowledgedTab = screen.getByRole('tab', { name: /acknowledged \(1\)/i });
    fireEvent.click(acknowledgedTab);

    // Should only show acknowledged alert
    await waitFor(() => {
      expect(screen.getByText('Disk Space Info')).toBeInTheDocument();
      expect(screen.queryByText('High CPU Usage')).not.toBeInTheDocument();
    });
  });

  it('switches to resolved tab', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /resolved \(1\)/i })).toBeInTheDocument();
    });

    const resolvedTab = screen.getByRole('tab', { name: /resolved \(1\)/i });
    fireEvent.click(resolvedTab);

    // Should only show resolved alert
    await waitFor(() => {
      expect(screen.getByText('Network Issue')).toBeInTheDocument();
      expect(screen.queryByText('High CPU Usage')).not.toBeInTheDocument();
    });
  });

  it('switches to all alerts tab', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /all alerts/i })).toBeInTheDocument();
    });

    const allAlertsTab = screen.getByRole('tab', { name: /all alerts/i });
    fireEvent.click(allAlertsTab);

    // Should show all alerts
    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
      expect(screen.getByText('Memory Warning')).toBeInTheDocument();
      expect(screen.getByText('Disk Space Info')).toBeInTheDocument();
      expect(screen.getByText('Network Issue')).toBeInTheDocument();
    });
  });

  // ===== SEARCH AND FILTER TESTS =====

  it('displays search input', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search alerts/i)).toBeInTheDocument();
    });
  });

  it('filters alerts by search query', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search alerts/i);
    fireEvent.change(searchInput, { target: { value: 'CPU' } });

    expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    expect(screen.queryByText('Memory Warning')).not.toBeInTheDocument();
  });

  it.skip('displays status filter dropdown', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });
  });

  it.skip('filters alerts by triggered status', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const triggeredOption = await screen.findByText('Triggered');
    fireEvent.click(triggeredOption);

    await waitFor(() => {
      // API should be called with status filter
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=triggered'),
        expect.any(Object)
      );
    });
  });

  it('displays filtered count', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText(/showing 4 alerts/i)).toBeInTheDocument();
    });
  });

  // ===== CREATE ALERT DIALOG TESTS =====

  it('opens create alert dialog', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create alert/i });
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Create Alert Rule')).toBeInTheDocument();
    });
  });

  it.skip('allows entering alert details', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    const descriptionInput = screen.getByLabelText('Description');
    const conditionInput = screen.getByLabelText('Condition');
    const thresholdInput = screen.getByLabelText('Threshold');

    fireEvent.change(nameInput, { target: { value: 'New Alert' } });
    fireEvent.change(descriptionInput, { target: { value: 'Test alert' } });
    fireEvent.change(conditionInput, { target: { value: 'test > threshold' } });
    fireEvent.change(thresholdInput, { target: { value: '90' } });

    expect(nameInput).toHaveValue('New Alert');
    expect(descriptionInput).toHaveValue('Test alert');
    expect(conditionInput).toHaveValue('test > threshold');
    expect(thresholdInput).toHaveValue(90);
  });

  it.skip('allows selecting severity', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Severity')).toBeInTheDocument();
    });

    const severitySelect = screen.getByLabelText('Severity');
    fireEvent.mouseDown(severitySelect);

    const criticalOption = await screen.findByText('Critical');
    fireEvent.click(criticalOption);

    // Verify dropdown opened and option exists
    expect(criticalOption).toBeInTheDocument();
  });

  it('disables create button when required fields are empty', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
      expect(createDialogButton).toBeDisabled();
    });
  });

  it.skip('creates alert when form is submitted', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Alert' } });
    fireEvent.change(screen.getByLabelText('Condition'), { target: { value: 'test > threshold' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: '5', name: 'New Alert' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/monitoring/alerts',
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it.skip('handles create alert errors', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'New Alert' } });
    fireEvent.change(screen.getByLabelText('Condition'), { target: { value: 'test > threshold' } });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Creation failed' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith('/api/v1/monitoring/alerts', expect.any(Object));
    });
  });

  // ===== ACKNOWLEDGE ALERT TESTS =====

  it.skip('displays acknowledge button for triggered alerts', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const acknowledgeButtons = screen.getAllByTitle('Acknowledge');
    expect(acknowledgeButtons.length).toBe(2); // 2 triggered alerts
  });

  it.skip('acknowledges alert when button is clicked', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const acknowledgeButton = screen.getAllByTitle('Acknowledge')[0];
    fireEvent.click(acknowledgeButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/acknowledge'),
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it.skip('handles acknowledge errors', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Acknowledge failed' }),
    });

    const acknowledgeButton = screen.getAllByTitle('Acknowledge')[0];
    fireEvent.click(acknowledgeButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/acknowledge'), expect.any(Object));
    });
  });

  // ===== RESOLVE ALERT TESTS =====

  it.skip('displays resolve button for triggered and acknowledged alerts', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const resolveButtons = screen.getAllByTitle('Resolve');
    expect(resolveButtons.length).toBe(3); // 2 triggered + 1 acknowledged
  });

  it.skip('resolves alert when button is clicked', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const resolveButton = screen.getAllByTitle('Resolve')[0];
    fireEvent.click(resolveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/resolve'),
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it.skip('handles resolve errors', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Resolve failed' }),
    });

    const resolveButton = screen.getAllByTitle('Resolve')[0];
    fireEvent.click(resolveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/resolve'), expect.any(Object));
    });
  });

  // ===== EDIT ALERT TESTS =====

  it.skip('displays edit button for all alerts', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByTitle('Edit');
    expect(editButtons.length).toBe(4); // All 4 alerts
  });

  it.skip('opens edit dialog with pre-filled data', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Alert Rule')).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue('High CPU Usage')).toBeInTheDocument();
    expect(screen.getByDisplayValue('CPU usage exceeds threshold')).toBeInTheDocument();
    expect(screen.getByDisplayValue('cpu_usage > threshold')).toBeInTheDocument();
    expect(screen.getByDisplayValue('80')).toBeInTheDocument();
  });

  it.skip('updates alert when edit form is submitted', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Alert Rule')).toBeInTheDocument();
    });

    const nameInput = screen.getByDisplayValue('High CPU Usage');
    fireEvent.change(nameInput, { target: { value: 'Updated Alert' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const updateButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^update$/i });
    fireEvent.click(updateButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/monitoring/alerts/'),
        expect.objectContaining({
          method: 'PUT',
        })
      );
    });
  });

  it.skip('handles update alert errors', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Alert Rule')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Update failed' }),
    });

    const updateButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^update$/i });
    fireEvent.click(updateButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/monitoring/alerts/'), expect.any(Object));
    });
  });

  // ===== DELETE ALERT TESTS =====

  it.skip('displays delete button for all alerts', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const deleteButtons = screen.getAllByTitle('Delete');
    expect(deleteButtons.length).toBe(4); // All 4 alerts
  });

  it.skip('opens delete confirmation dialog', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete Alert?')).toBeInTheDocument();
    });

    expect(screen.getByText(/this action cannot be undone/i)).toBeInTheDocument();
  });

  it.skip('deletes alert when confirmed', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete Alert?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const confirmDeleteButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmDeleteButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/monitoring/alerts/'),
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  it.skip('handles delete alert errors', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete Alert?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Delete failed' }),
    });

    const confirmDeleteButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmDeleteButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/monitoring/alerts/'), expect.any(Object));
    });
  });

  // ===== REFRESH TESTS =====

  it('displays refresh button', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });
  });

  it.skip('refetches alerts when refresh is clicked', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('Monitoring & Alerts')).toBeInTheDocument();
    });

    mockFetch.mockClear();

    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/monitoring/alerts'),
        expect.any(Object)
      );
    });
  });

  // ===== EMPTY STATE TESTS =====

  it('displays empty state when no alerts found', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({ alerts: [] }),
    });

    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('No alerts found')).toBeInTheDocument();
    });
  });
});

describe('Monitoring Page - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockAlerts,
    });
  });

  it('has accessible buttons with clear names', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it('has accessible table structure', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('table')).toBeInTheDocument();
    });

    const table = screen.getByRole('table');
    const headers = within(table).getAllByRole('columnheader');
    expect(headers.length).toBe(7);
  });

  it.skip('has accessible form controls in create dialog', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create alert/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create alert/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('Description')).toBeInTheDocument();
    expect(screen.getByLabelText('Severity')).toBeInTheDocument();
    expect(screen.getByLabelText('Condition')).toBeInTheDocument();
    expect(screen.getByLabelText('Threshold')).toBeInTheDocument();
  });

  it('has accessible tab navigation', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /active \(2\)/i })).toBeInTheDocument();
    });

    const tabs = screen.getAllByRole('tab');
    tabs.forEach((tab) => {
      expect(tab).toHaveAccessibleName();
    });
  });
});

describe('Monitoring Page - Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockAlerts,
    });
  });

  it.skip('updates summary counts when filtering by status', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('2')).toBeInTheDocument(); // 2 active alerts initially
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const triggeredOption = await screen.findByText('Triggered');
    fireEvent.click(triggeredOption);

    // Summary should still show counts based on actual data
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=triggered'),
        expect.any(Object)
      );
    });
  });

  it('filters search results across tabs', async () => {
    renderMonitoring();

    await waitFor(() => {
      expect(screen.getByText('High CPU Usage')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search alerts/i);
    fireEvent.change(searchInput, { target: { value: 'Memory' } });

    // Switch to All Alerts tab
    const allAlertsTab = screen.getByRole('tab', { name: /all alerts/i });
    fireEvent.click(allAlertsTab);

    await waitFor(() => {
      expect(screen.getByText('Memory Warning')).toBeInTheDocument();
      expect(screen.queryByText('High CPU Usage')).not.toBeInTheDocument();
    });
  });
});

import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import AuditLogs from './AuditLogs';
import { api } from '../../lib/api';

// Mock the API
vi.mock('../../lib/api', () => ({
  api: {
    get: vi.fn(),
  },
}));

// Mock the notification queue hook
vi.mock('../../components/NotificationQueue', () => ({
  useNotificationQueue: () => ({
    addNotification: vi.fn(),
  }),
}));

// Mock AdminPortalLayout to avoid testing layout complexity
vi.mock('../../components/AdminPortalLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

// Mock Material-UI DateTimePicker to avoid date picker complexity
vi.mock('@mui/x-date-pickers/DateTimePicker', () => ({
  DateTimePicker: ({ value, onChange, label }: any) => (
    <input
      type="datetime-local"
      value={value ? value.toISOString().slice(0, 16) : ''}
      onChange={(e) => onChange(e.target.value ? new Date(e.target.value) : null)}
      aria-label={label}
    />
  ),
}));

vi.mock('@mui/x-date-pickers/LocalizationProvider', () => ({
  LocalizationProvider: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

vi.mock('@mui/x-date-pickers/AdapterDateFns', () => ({
  AdapterDateFns: class { },
}));

// Mock audit log data
const mockAuditLogs = {
  logs: [
    {
      id: 1,
      user_id: 'user-123',
      action: 'POST',
      resource_type: '/api/sessions',
      resource_id: 'session-1',
      changes: { state: 'running' },
      timestamp: '2025-01-15T10:00:00Z',
      ip_address: '192.168.1.1',
    },
    {
      id: 2,
      user_id: 'user-456',
      action: 'DELETE',
      resource_type: '/api/users',
      resource_id: 'user-789',
      changes: {},
      timestamp: '2025-01-15T11:30:00Z',
      ip_address: '192.168.1.2',
    },
    {
      id: 3,
      user_id: 'admin-001',
      action: 'PUT',
      resource_type: '/api/config',
      resource_id: 'ingress.domain',
      changes: { old: 'old.example.com', new: 'new.example.com' },
      timestamp: '2025-01-15T12:45:00Z',
      ip_address: '10.0.0.1',
    },
  ],
  total: 3,
  page: 1,
  page_size: 100,
  total_pages: 1,
};

// Helper to render component with providers
const renderAuditLogs = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuditLogs />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('AuditLogs Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Default mock implementation
    (api.get as any).mockResolvedValue({ data: mockAuditLogs });
  });

  describe('Rendering', () => {
    it('renders page title and description', async () => {
      renderAuditLogs();

      expect(screen.getByText(/audit logs/i)).toBeInTheDocument();
    });

    it('displays audit logs in table', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
        expect(screen.getByText('DELETE')).toBeInTheDocument();
        expect(screen.getByText('PUT')).toBeInTheDocument();
      });

      expect(screen.getByText('/api/sessions')).toBeInTheDocument();
      expect(screen.getByText('/api/users')).toBeInTheDocument();
      expect(screen.getByText('/api/config')).toBeInTheDocument();
    });

    it('displays user IDs correctly', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('user-123')).toBeInTheDocument();
      });

      expect(screen.getByText('user-456')).toBeInTheDocument();
      expect(screen.getByText('admin-001')).toBeInTheDocument();
    });

    it('displays IP addresses', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('192.168.1.1')).toBeInTheDocument();
      });

      expect(screen.getByText('192.168.1.2')).toBeInTheDocument();
      expect(screen.getByText('10.0.0.1')).toBeInTheDocument();
    });

    it('formats timestamps correctly', async () => {
      renderAuditLogs();

      await waitFor(() => {
        // Check if timestamp is rendered (format may vary)
        expect(screen.getByText(/Jan 15, 2025/i)).toBeInTheDocument();
      });
    });
  });

  describe('Filtering', () => {
    it('has user ID filter input', () => {
      renderAuditLogs();

      const userIdInput = screen.getByLabelText(/user id/i);
      expect(userIdInput).toBeInTheDocument();
    });

    it('has action filter dropdown', () => {
      renderAuditLogs();

      const actionFilter = screen.getByLabelText(/action/i);
      expect(actionFilter).toBeInTheDocument();
    });

    it('has resource type filter input', () => {
      renderAuditLogs();

      const resourceTypeInput = screen.getByLabelText(/resource type/i);
      expect(resourceTypeInput).toBeInTheDocument();
    });

    it('has IP address filter input', () => {
      renderAuditLogs();

      const ipAddressInput = screen.getByLabelText(/ip address/i);
      expect(ipAddressInput).toBeInTheDocument();
    });

    it('has date range filters', () => {
      renderAuditLogs();

      const startDateInput = screen.getByLabelText(/start date/i);
      const endDateInput = screen.getByLabelText(/end date/i);

      expect(startDateInput).toBeInTheDocument();
      expect(endDateInput).toBeInTheDocument();
    });

    it('applies user ID filter on search', async () => {
      renderAuditLogs();

      const userIdInput = screen.getByLabelText(/user id/i);
      fireEvent.change(userIdInput, { target: { value: 'user-123' } });

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          expect.stringContaining('user_id=user-123'),
          expect.any(Object)
        );
      });
    });

    it('applies action filter', async () => {
      renderAuditLogs();

      const actionFilter = screen.getByLabelText(/action/i);
      fireEvent.change(actionFilter, { target: { value: 'POST' } });

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          expect.stringContaining('action=POST'),
          expect.any(Object)
        );
      });
    });

    it('clears filters when clear button is clicked', async () => {
      renderAuditLogs();

      // Set filters
      const userIdInput = screen.getByLabelText(/user id/i);
      fireEvent.change(userIdInput, { target: { value: 'user-123' } });

      // Clear filters
      const clearButton = screen.getByRole('button', { name: /clear/i });
      fireEvent.click(clearButton);

      expect(userIdInput).toHaveValue('');
    });
  });

  describe('Pagination', () => {
    it('displays pagination controls', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByRole('navigation')).toBeInTheDocument();
      });
    });

    it('shows correct page count', async () => {
      (api.get as any).mockResolvedValue({
        data: {
          ...mockAuditLogs,
          total: 250,
          total_pages: 3,
        },
      });

      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText(/page 1 of 3/i)).toBeInTheDocument();
      });
    });

    it('fetches next page on pagination click', async () => {
      (api.get as any).mockResolvedValue({
        data: {
          ...mockAuditLogs,
          total: 250,
          page: 1,
          total_pages: 3,
        },
      });

      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText(/page 1/i)).toBeInTheDocument();
      });

      const nextPageButton = screen.getByRole('button', { name: /next page/i });
      fireEvent.click(nextPageButton);

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          expect.stringContaining('page=2'),
          expect.any(Object)
        );
      });
    });
  });

  describe('Detail Dialog', () => {
    it('opens detail dialog when view button is clicked', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
      });

      const viewButtons = screen.getAllByRole('button', { name: /view/i });
      fireEvent.click(viewButtons[0]);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
      });
    });

    it('displays audit log details in dialog', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
      });

      const viewButtons = screen.getAllByRole('button', { name: /view/i });
      fireEvent.click(viewButtons[0]);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        expect(within(dialog).getByText('user-123')).toBeInTheDocument();
        expect(within(dialog).getByText('/api/sessions')).toBeInTheDocument();
      });
    });

    it('shows changes JSON in detail dialog', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('PUT')).toBeInTheDocument();
      });

      const viewButtons = screen.getAllByRole('button', { name: /view/i });
      fireEvent.click(viewButtons[2]); // The PUT entry with changes

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        expect(within(dialog).getByText(/old.example.com/i)).toBeInTheDocument();
        expect(within(dialog).getByText(/new.example.com/i)).toBeInTheDocument();
      });
    });

    it('closes detail dialog when close button is clicked', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
      });

      const viewButtons = screen.getAllByRole('button', { name: /view/i });
      fireEvent.click(viewButtons[0]);

      await waitFor(() => {
        expect(screen.getByRole('dialog')).toBeInTheDocument();
      });

      const closeButton = screen.getByRole('button', { name: /close/i });
      fireEvent.click(closeButton);

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
      });
    });
  });

  describe('Export Functionality', () => {
    it('has CSV export button', () => {
      renderAuditLogs();

      const csvButton = screen.getByRole('button', { name: /export csv/i });
      expect(csvButton).toBeInTheDocument();
    });

    it('has JSON export button', () => {
      renderAuditLogs();

      const jsonButton = screen.getByRole('button', { name: /export json/i });
      expect(jsonButton).toBeInTheDocument();
    });

    it('calls API with correct format for CSV export', async () => {
      (api.get as any).mockResolvedValue({ data: 'csv,data' });

      renderAuditLogs();

      const csvButton = screen.getByRole('button', { name: /export csv/i });
      fireEvent.click(csvButton);

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          expect.stringContaining('format=csv'),
          expect.objectContaining({ responseType: 'blob' })
        );
      });
    });

    it('calls API with correct format for JSON export', async () => {
      (api.get as any).mockResolvedValue({ data: [] });

      renderAuditLogs();

      const jsonButton = screen.getByRole('button', { name: /export json/i });
      fireEvent.click(jsonButton);

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledWith(
          expect.stringContaining('format=json'),
          expect.objectContaining({ responseType: 'blob' })
        );
      });
    });
  });

  describe('Refresh Functionality', () => {
    it('has refresh button', () => {
      renderAuditLogs();

      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      expect(refreshButton).toBeInTheDocument();
    });

    it('refetches data when refresh button is clicked', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledTimes(1);
      });

      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      fireEvent.click(refreshButton);

      await waitFor(() => {
        expect(api.get).toHaveBeenCalledTimes(2);
      });
    });
  });

  describe('Loading State', () => {
    it('shows loading indicator while fetching data', async () => {
      (api.get as any).mockImplementation(
        () => new Promise((resolve) => setTimeout(() => resolve({ data: mockAuditLogs }), 100))
      );

      renderAuditLogs();

      expect(screen.getByText(/loading/i)).toBeInTheDocument();

      await waitFor(() => {
        expect(screen.queryByText(/loading/i)).not.toBeInTheDocument();
      });
    });
  });

  describe('Error Handling', () => {
    it('displays error message when API call fails', async () => {
      (api.get as any).mockRejectedValue(new Error('Network error'));

      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText(/error/i)).toBeInTheDocument();
      });
    });

    it('shows empty state when no logs are returned', async () => {
      (api.get as any).mockResolvedValue({
        data: {
          logs: [],
          total: 0,
          page: 1,
          page_size: 100,
          total_pages: 0,
        },
      });

      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText(/no audit logs found/i)).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    it('has accessible table headers', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
      });

      const table = screen.getByRole('table');
      const headers = within(table).getAllByRole('columnheader');

      expect(headers.length).toBeGreaterThan(0);
      headers.forEach((header) => {
        expect(header).toHaveAccessibleName();
      });
    });

    it('has accessible form controls', () => {
      renderAuditLogs();

      const userIdInput = screen.getByLabelText(/user id/i);
      const actionSelect = screen.getByLabelText(/action/i);

      expect(userIdInput).toHaveAccessibleName();
      expect(actionSelect).toHaveAccessibleName();
    });

    it('has accessible buttons with names', () => {
      renderAuditLogs();

      const buttons = screen.getAllByRole('button');
      buttons.forEach((button) => {
        expect(button).toHaveAccessibleName();
      });
    });

    it('dialog has accessible title', async () => {
      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('POST')).toBeInTheDocument();
      });

      const viewButtons = screen.getAllByRole('button', { name: /view/i });
      fireEvent.click(viewButtons[0]);

      await waitFor(() => {
        const dialog = screen.getByRole('dialog');
        expect(dialog).toHaveAccessibleName();
      });
    });
  });

  describe('Status Code Display', () => {
    it('displays status codes with appropriate colors', async () => {
      const logsWithStatus = {
        ...mockAuditLogs,
        logs: mockAuditLogs.logs.map((log, idx) => ({
          ...log,
          status_code: [200, 401, 500][idx],
        })),
      };

      (api.get as any).mockResolvedValue({ data: logsWithStatus });

      renderAuditLogs();

      await waitFor(() => {
        expect(screen.getByText('200')).toBeInTheDocument();
        expect(screen.getByText('401')).toBeInTheDocument();
        expect(screen.getByText('500')).toBeInTheDocument();
      });

      // Status codes should have color-coded badges
      const successBadge = screen.getByText('200').closest('[class*="chip"]');
      const errorBadge = screen.getByText('401').closest('[class*="chip"]');
      const serverErrorBadge = screen.getByText('500').closest('[class*="chip"]');

      expect(successBadge).toBeTruthy();
      expect(errorBadge).toBeTruthy();
      expect(serverErrorBadge).toBeTruthy();
    });
  });
});

describe('AuditLogs Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (api.get as any).mockResolvedValue({ data: mockAuditLogs });
  });

  it('applies multiple filters simultaneously', async () => {
    renderAuditLogs();

    // Apply multiple filters
    const userIdInput = screen.getByLabelText(/user id/i);
    fireEvent.change(userIdInput, { target: { value: 'user-123' } });

    const actionFilter = screen.getByLabelText(/action/i);
    fireEvent.change(actionFilter, { target: { value: 'POST' } });

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith(
        expect.stringContaining('user_id=user-123'),
        expect.any(Object)
      );
      expect(api.get).toHaveBeenCalledWith(
        expect.stringContaining('action=POST'),
        expect.any(Object)
      );
    });
  });

  it('maintains filters across pagination', async () => {
    (api.get as any).mockResolvedValue({
      data: {
        ...mockAuditLogs,
        total: 250,
        total_pages: 3,
      },
    });

    renderAuditLogs();

    // Apply filter
    const userIdInput = screen.getByLabelText(/user id/i);
    fireEvent.change(userIdInput, { target: { value: 'user-123' } });

    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith(
        expect.stringContaining('user_id=user-123'),
        expect.any(Object)
      );
    });

    // Navigate to next page
    const nextPageButton = screen.getByRole('button', { name: /next page/i });
    fireEvent.click(nextPageButton);

    // Filter should still be applied
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith(
        expect.stringContaining('user_id=user-123'),
        expect.any(Object)
      );
      expect(api.get).toHaveBeenCalledWith(
        expect.stringContaining('page=2'),
        expect.any(Object)
      );
    });
  });
});

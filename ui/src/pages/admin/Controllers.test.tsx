import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import Controllers from './Controllers';

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

// Mock controllers data
const mockControllers = [
  {
    id: '1',
    controller_id: 'k8s-cluster-1',
    display_name: 'Production Kubernetes',
    platform: 'kubernetes',
    status: 'connected',
    version: 'v1.0.0',
    capabilities: ['sessions', 'templates', 'volumes'],
    last_heartbeat: '2025-01-15T10:00:00Z',
  },
  {
    id: '2',
    controller_id: 'docker-host-1',
    display_name: 'Development Docker',
    platform: 'docker',
    status: 'connected',
    version: 'v1.0.1',
    capabilities: ['sessions', 'templates'],
    last_heartbeat: '2025-01-15T09:30:00Z',
  },
  {
    id: '3',
    controller_id: 'hyperv-server-1',
    display_name: 'Test Hyper-V',
    platform: 'hyperv',
    status: 'disconnected',
    version: 'v0.9.0',
    capabilities: ['sessions'],
    last_heartbeat: '2025-01-14T10:00:00Z',
  },
  {
    id: '4',
    controller_id: 'vcenter-1',
    display_name: 'vCenter Controller',
    platform: 'vcenter',
    status: 'unknown',
    version: null,
    capabilities: [],
    last_heartbeat: null,
  },
];

// Helper to render Controllers with providers
const renderControllers = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Controllers />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('Controllers Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockControllers,
    });
  });

  // ===== RENDERING TESTS =====

  it('renders page title and description', async () => {
    renderControllers();

    expect(screen.getByText('Platform Controllers')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText(/manage distributed controllers/i)).toBeInTheDocument();
    });
  });

  it('displays loading state initially', () => {
    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* never resolves */
        })
    );

    renderControllers();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('displays summary cards', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Total Controllers')).toBeInTheDocument();
    });

    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });

  it('displays correct counts in summary cards', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('4')).toBeInTheDocument(); // Total
    });

    expect(screen.getByText('2')).toBeInTheDocument(); // Connected
    expect(screen.getByText('1')).toBeInTheDocument(); // Disconnected
  });

  it('displays controllers in table', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    expect(screen.getByText('Development Docker')).toBeInTheDocument();
    expect(screen.getByText('Test Hyper-V')).toBeInTheDocument();
    expect(screen.getByText('vCenter Controller')).toBeInTheDocument();
  });

  it('displays controller IDs in monospace font', async () => {
    renderControllers();

    await waitFor(() => {
      const controllerId = screen.getByText('k8s-cluster-1');
      expect(controllerId).toHaveStyle({ fontFamily: 'monospace' });
    });
  });

  it('displays platform chips', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('kubernetes')).toBeInTheDocument();
    });

    expect(screen.getByText('docker')).toBeInTheDocument();
    expect(screen.getByText('hyperv')).toBeInTheDocument();
    expect(screen.getByText('vcenter')).toBeInTheDocument();
  });

  it('displays status chips with correct colors', async () => {
    renderControllers();

    await waitFor(() => {
      const connectedChips = screen.getAllByText('connected');
      expect(connectedChips.length).toBe(2);
    });

    expect(screen.getByText('disconnected')).toBeInTheDocument();
    expect(screen.getByText('unknown')).toBeInTheDocument();
  });

  it('displays versions', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('v1.0.0')).toBeInTheDocument();
    });

    expect(screen.getByText('v1.0.1')).toBeInTheDocument();
    expect(screen.getByText('v0.9.0')).toBeInTheDocument();
    expect(screen.getByText('N/A')).toBeInTheDocument(); // For null version
  });

  it('displays capabilities chips', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('sessions')).toBeInTheDocument();
    });

    expect(screen.getByText('templates')).toBeInTheDocument();
    expect(screen.getByText('volumes')).toBeInTheDocument();
  });

  it('displays "+N" chip when more than 2 capabilities', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('+1')).toBeInTheDocument(); // Production K8s has 3 capabilities
    });
  });

  it('displays last heartbeat timestamps', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText(/1\/15\/2025/)).toBeInTheDocument();
    });

    expect(screen.getByText(/1\/14\/2025/)).toBeInTheDocument();
  });

  it('displays "Never" for controllers without heartbeat', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Never')).toBeInTheDocument();
    });
  });

  // ===== SEARCH AND FILTER TESTS =====

  it('displays search input', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search controllers/i)).toBeInTheDocument();
    });
  });

  it('filters controllers by search query (name)', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    fireEvent.change(searchInput, { target: { value: 'Production' } });

    expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    expect(screen.queryByText('Development Docker')).not.toBeInTheDocument();
  });

  it('filters controllers by search query (ID)', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    fireEvent.change(searchInput, { target: { value: 'docker-host' } });

    expect(screen.getByText('Development Docker')).toBeInTheDocument();
    expect(screen.queryByText('Production Kubernetes')).not.toBeInTheDocument();
  });

  it('filters controllers by search query (platform)', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    fireEvent.change(searchInput, { target: { value: 'kubernetes' } });

    expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    expect(screen.queryByText('Development Docker')).not.toBeInTheDocument();
  });

  it('displays platform filter dropdown', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByLabelText('Platform')).toBeInTheDocument();
    });
  });

  it('filters controllers by platform', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const platformSelect = screen.getByLabelText('Platform');
    fireEvent.mouseDown(platformSelect);

    const dockerOption = await screen.findByText('Docker');
    fireEvent.click(dockerOption);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('platform=docker'),
        expect.any(Object)
      );
    });
  });

  it('displays status filter dropdown', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });
  });

  it('filters controllers by connected status', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const connectedOption = await screen.findByText('Connected');
    fireEvent.click(connectedOption);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=connected'),
        expect.any(Object)
      );
    });
  });

  it('displays filtered count', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText(/showing 4 controllers/i)).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    fireEvent.change(searchInput, { target: { value: 'Production' } });

    await waitFor(() => {
      expect(screen.getByText(/showing 1 controllers/i)).toBeInTheDocument();
    });
  });

  // ===== REGISTER CONTROLLER DIALOG TESTS =====

  it('opens register controller dialog', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    const registerButton = screen.getByRole('button', { name: /register controller/i });
    fireEvent.click(registerButton);

    await waitFor(() => {
      expect(screen.getByText('Register Platform Controller')).toBeInTheDocument();
    });
  });

  it('allows entering controller registration details', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Controller ID')).toBeInTheDocument();
    });

    const idInput = screen.getByLabelText('Controller ID');
    const nameInput = screen.getByLabelText('Display Name');
    const versionInput = screen.getByLabelText('Version');

    fireEvent.change(idInput, { target: { value: 'new-controller-1' } });
    fireEvent.change(nameInput, { target: { value: 'New Controller' } });
    fireEvent.change(versionInput, { target: { value: 'v1.0.0' } });

    expect(idInput).toHaveValue('new-controller-1');
    expect(nameInput).toHaveValue('New Controller');
    expect(versionInput).toHaveValue('v1.0.0');
  });

  it('allows selecting platform', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Platform')).toBeInTheDocument();
    });

    const platformSelect = screen.getByLabelText('Platform');
    fireEvent.mouseDown(platformSelect);

    const dockerOption = await screen.findByText('Docker');
    fireEvent.click(dockerOption);

    expect(dockerOption).toBeInTheDocument();
  });

  it('disables register button when required fields are empty', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      const registerDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^register$/i });
      expect(registerDialogButton).toBeDisabled();
    });
  });

  it('registers controller when form is submitted', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Controller ID')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Controller ID'), { target: { value: 'new-controller-1' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ id: '5', controller_id: 'new-controller-1' }),
    });

    const registerDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^register$/i });
    fireEvent.click(registerDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/controllers/register',
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it('handles register controller errors', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Controller ID')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText('Controller ID'), { target: { value: 'new-controller-1' } });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Controller already exists' }),
    });

    const registerDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^register$/i });
    fireEvent.click(registerDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith('/api/v1/admin/controllers/register', expect.any(Object));
    });
  });

  // ===== EDIT CONTROLLER TESTS =====

  it('displays edit button for all controllers', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const editButtons = screen.getAllByTitle('Edit');
    expect(editButtons.length).toBe(4); // All 4 controllers
  });

  it('opens edit dialog with pre-filled data', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Controller')).toBeInTheDocument();
    });

    expect(screen.getByDisplayValue('Production Kubernetes')).toBeInTheDocument();
    expect(screen.getByDisplayValue('v1.0.0')).toBeInTheDocument();
  });

  it('updates controller when edit form is submitted', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Controller')).toBeInTheDocument();
    });

    const nameInput = screen.getByDisplayValue('Production Kubernetes');
    fireEvent.change(nameInput, { target: { value: 'Updated Controller' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const updateButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^update$/i });
    fireEvent.click(updateButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/admin/controllers/'),
        expect.objectContaining({
          method: 'PUT',
        })
      );
    });
  });

  it('handles update controller errors', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const editButton = screen.getAllByTitle('Edit')[0];
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByText('Edit Controller')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Update failed' }),
    });

    const updateButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^update$/i });
    fireEvent.click(updateButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/admin/controllers/'), expect.any(Object));
    });
  });

  // ===== DELETE CONTROLLER TESTS =====

  it('displays unregister button for all controllers', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const deleteButtons = screen.getAllByTitle('Unregister');
    expect(deleteButtons.length).toBe(4); // All 4 controllers
  });

  it('opens unregister confirmation dialog', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Unregister')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Unregister Controller?')).toBeInTheDocument();
    });

    expect(screen.getByText(/workloads may be affected/i)).toBeInTheDocument();
  });

  it('unregisters controller when confirmed', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Unregister')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Unregister Controller?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const confirmButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^unregister$/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/admin/controllers/'),
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  it('handles unregister errors', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Unregister')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Unregister Controller?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Delete failed' }),
    });

    const confirmButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^unregister$/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/admin/controllers/'), expect.any(Object));
    });
  });

  // ===== REFRESH TESTS =====

  it('displays refresh button', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });
  });

  it('refetches controllers when refresh is clicked', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Platform Controllers')).toBeInTheDocument();
    });

    mockFetch.mockClear();

    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/admin/controllers'),
        expect.any(Object)
      );
    });
  });

  // ===== EMPTY STATE TESTS =====

  it('displays empty state when no controllers found', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => [],
    });

    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('No controllers found')).toBeInTheDocument();
    });
  });
});

describe('Controllers Page - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockControllers,
    });
  });

  it('has accessible buttons with clear names', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it('has accessible table structure', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('table')).toBeInTheDocument();
    });

    const table = screen.getByRole('table');
    const headers = within(table).getAllByRole('columnheader');
    expect(headers.length).toBe(7);
  });

  it('has accessible form controls in register dialog', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /register controller/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /register controller/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Controller ID')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('Platform')).toBeInTheDocument();
    expect(screen.getByLabelText('Display Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Version')).toBeInTheDocument();
  });

  it('has accessible search input', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search controllers/i)).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    expect(searchInput).toHaveAccessibleName();
  });
});

describe('Controllers Page - Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockControllers,
    });
  });

  it('updates summary counts when filtering by status', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('2')).toBeInTheDocument(); // 2 connected initially
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const connectedOption = await screen.findByText('Connected');
    fireEvent.click(connectedOption);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('status=connected'),
        expect.any(Object)
      );
    });
  });

  it('maintains search query across filter changes', async () => {
    renderControllers();

    await waitFor(() => {
      expect(screen.getByText('Production Kubernetes')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search controllers/i);
    fireEvent.change(searchInput, { target: { value: 'kubernetes' } });

    // Change platform filter
    const platformSelect = screen.getByLabelText('Platform');
    fireEvent.mouseDown(platformSelect);

    const k8sOption = await screen.findByText('Kubernetes');
    fireEvent.click(k8sOption);

    // Search query should still be applied
    await waitFor(() => {
      expect(searchInput).toHaveValue('kubernetes');
    });
  });
});

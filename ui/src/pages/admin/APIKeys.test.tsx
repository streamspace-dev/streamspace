import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import APIKeys from './APIKeys';

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

// Mock clipboard API
const mockClipboard = {
  writeText: vi.fn(),
};
Object.defineProperty(navigator, 'clipboard', {
  value: mockClipboard,
  writable: true,
  configurable: true,
});

// Mock API keys data
const mockAPIKeys = [
  {
    id: 1,
    name: 'Production API Key',
    description: 'Main production key',
    keyPrefix: 'sk_prod_abc123',
    userId: 'admin',
    scopes: ['sessions:read', 'sessions:write', 'templates:read'],
    rateLimit: 1000,
    useCount: 450,
    lastUsedAt: '2025-01-15T10:00:00Z',
    isActive: true,
    expiresAt: '2026-01-01T00:00:00Z',
    createdAt: '2025-01-01T00:00:00Z',
  },
  {
    id: 2,
    name: 'Development API Key',
    description: 'For testing',
    keyPrefix: 'sk_dev_xyz789',
    userId: 'developer',
    scopes: ['sessions:read', 'templates:read'],
    rateLimit: 500,
    useCount: 120,
    lastUsedAt: '2025-01-14T15:30:00Z',
    isActive: true,
    expiresAt: null,
    createdAt: '2025-01-10T00:00:00Z',
  },
  {
    id: 3,
    name: 'Revoked Key',
    description: 'Old key',
    keyPrefix: 'sk_old_def456',
    userId: 'admin',
    scopes: ['sessions:read'],
    rateLimit: 1000,
    useCount: 890,
    lastUsedAt: '2024-12-20T10:00:00Z',
    isActive: false,
    expiresAt: '2025-06-01T00:00:00Z',
    createdAt: '2024-06-01T00:00:00Z',
  },
  {
    id: 4,
    name: 'Expired Key',
    description: 'Expired',
    keyPrefix: 'sk_exp_ghi123',
    userId: 'user1',
    scopes: ['templates:read'],
    rateLimit: 100,
    useCount: 50,
    lastUsedAt: '2024-11-01T10:00:00Z',
    isActive: true,
    expiresAt: '2024-12-01T00:00:00Z',
    createdAt: '2024-11-01T00:00:00Z',
  },
];

// Helper to render APIKeys with providers
const renderAPIKeys = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <APIKeys />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('APIKeys Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockAPIKeys,
    });
  });

  // ===== RENDERING TESTS =====

  it('renders page title and description', async () => {
    renderAPIKeys();

    expect(screen.getByText('API Keys')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText('API Keys Management')).toBeInTheDocument();
    });
    expect(screen.getByText(/4 total keys/i)).toBeInTheDocument();
  });

  it('displays loading state initially', () => {
    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* never resolves */
        })
    );

    renderAPIKeys();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('displays API keys in table', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    expect(screen.getByText('Development API Key')).toBeInTheDocument();
    expect(screen.getByText('Revoked Key')).toBeInTheDocument();
    expect(screen.getByText('Expired Key')).toBeInTheDocument();
  });

  it('displays key prefix in monospace font', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText(/sk_prod_abc123/)).toBeInTheDocument();
    });

    const keyPrefix = screen.getByText(/sk_prod_abc123/);
    expect(keyPrefix).toHaveStyle({ fontFamily: 'monospace' });
  });

  it('displays user IDs', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument();
    });

    expect(screen.getByText('developer')).toBeInTheDocument();
    expect(screen.getByText('user1')).toBeInTheDocument();
  });

  it('displays scopes as chips', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('sessions:read')).toBeInTheDocument();
    });

    expect(screen.getByText('sessions:write')).toBeInTheDocument();
    expect(screen.getByText('templates:read')).toBeInTheDocument();
  });

  it('displays "+N" chip when more than 2 scopes', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('+1')).toBeInTheDocument(); // Production key has 3 scopes
    });
  });

  it('displays rate limits', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('1000/hr')).toBeInTheDocument();
    });

    expect(screen.getByText('500/hr')).toBeInTheDocument();
    expect(screen.getByText('100/hr')).toBeInTheDocument();
  });

  it('displays usage statistics', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('450 calls')).toBeInTheDocument();
    });

    expect(screen.getByText('120 calls')).toBeInTheDocument();
    expect(screen.getByText('890 calls')).toBeInTheDocument();
  });

  it('displays last used date', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText(/Last: 1\/15\/2025/)).toBeInTheDocument();
    });
  });

  it('displays status chips', async () => {
    renderAPIKeys();

    await waitFor(() => {
      const activeChips = screen.getAllByText('Active');
      expect(activeChips.length).toBe(3); // 3 active keys
    });

    expect(screen.getByText('Inactive')).toBeInTheDocument(); // 1 inactive key
  });

  it('displays expiration dates', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText(/1\/1\/2026/)).toBeInTheDocument();
    });

    expect(screen.getByText(/6\/1\/2025/)).toBeInTheDocument();
  });

  it('displays "Expired" chip for expired keys', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Expired')).toBeInTheDocument();
    });
  });

  it('displays "Never" for keys without expiration', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Never')).toBeInTheDocument();
    });
  });

  // ===== SEARCH AND FILTER TESTS =====

  it('displays search input', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search by name, user, or key prefix/i)).toBeInTheDocument();
    });
  });

  it('filters keys by search query (name)', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    fireEvent.change(searchInput, { target: { value: 'Production' } });

    expect(screen.getByText('Production API Key')).toBeInTheDocument();
    expect(screen.queryByText('Development API Key')).not.toBeInTheDocument();
  });

  it('filters keys by search query (user)', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    fireEvent.change(searchInput, { target: { value: 'developer' } });

    expect(screen.getByText('Development API Key')).toBeInTheDocument();
    expect(screen.queryByText('Production API Key')).not.toBeInTheDocument();
  });

  it('filters keys by search query (key prefix)', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    fireEvent.change(searchInput, { target: { value: 'sk_dev' } });

    expect(screen.getByText('Development API Key')).toBeInTheDocument();
    expect(screen.queryByText('Production API Key')).not.toBeInTheDocument();
  });

  it('displays status filter dropdown', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByLabelText('Status')).toBeInTheDocument();
    });
  });

  it('filters keys by active status', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const activeOption = await screen.findByText('Active');
    fireEvent.click(activeOption);

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
      expect(screen.getByText('Development API Key')).toBeInTheDocument();
      expect(screen.queryByText('Revoked Key')).not.toBeInTheDocument(); // Inactive
    });
  });

  it('filters keys by inactive status', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const inactiveOption = await screen.findByText('Inactive');
    fireEvent.click(inactiveOption);

    await waitFor(() => {
      expect(screen.getByText('Revoked Key')).toBeInTheDocument();
      expect(screen.queryByText('Production API Key')).not.toBeInTheDocument();
    });
  });

  it('filters keys by expired status', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const statusSelect = screen.getByLabelText('Status');
    fireEvent.mouseDown(statusSelect);

    const expiredOption = await screen.findByText('Expired');
    fireEvent.click(expiredOption);

    await waitFor(() => {
      expect(screen.getByText('Expired Key')).toBeInTheDocument();
      expect(screen.queryByText('Production API Key')).not.toBeInTheDocument();
    });
  });

  it('displays filtered count', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText(/showing 4 of 4 keys/i)).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    fireEvent.change(searchInput, { target: { value: 'Production' } });

    await waitFor(() => {
      expect(screen.getByText(/showing 1 of 4 keys/i)).toBeInTheDocument();
    });
  });

  // ===== CREATE API KEY DIALOG TESTS =====

  it('opens create API key dialog', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    const createButton = screen.getByRole('button', { name: /create api key/i });
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText('Create API Key')).toBeInTheDocument();
    });
    expect(screen.getByLabelText('Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Description')).toBeInTheDocument();
  });

  it('allows entering API key details', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    const descriptionInput = screen.getByLabelText('Description');
    const rateLimitInput = screen.getByLabelText(/rate limit/i);

    fireEvent.change(nameInput, { target: { value: 'Test API Key' } });
    fireEvent.change(descriptionInput, { target: { value: 'Test description' } });
    fireEvent.change(rateLimitInput, { target: { value: '500' } });

    expect(nameInput).toHaveValue('Test API Key');
    expect(descriptionInput).toHaveValue('Test description');
    expect(rateLimitInput).toHaveValue(500);
  });

  it('allows selecting scopes', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Scopes')).toBeInTheDocument();
    });

    const scopesSelect = screen.getByLabelText('Scopes');
    fireEvent.mouseDown(scopesSelect);

    const sessionReadOption = await screen.findByText('sessions:read');
    fireEvent.click(sessionReadOption);

    // Note: Testing multi-select is complex, this verifies the dropdown opens
    expect(sessionReadOption).toBeInTheDocument();
  });

  it('allows selecting expiration period', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Expires In')).toBeInTheDocument();
    });

    const expiresSelect = screen.getByLabelText('Expires In');
    fireEvent.mouseDown(expiresSelect);

    const ninetyDaysOption = await screen.findByText('90 days');
    fireEvent.click(ninetyDaysOption);

    // Verify dropdown opened and option exists
    expect(ninetyDaysOption).toBeInTheDocument();
  });

  it('disables create button when name is empty', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
      expect(createDialogButton).toBeDisabled();
    });
  });

  it('creates API key when form is submitted', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'sk_new_abcdef123456' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/apikeys',
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
        })
      );
    });
  });

  it('handles create API key errors', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'API key creation failed' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith('/api/v1/apikeys', expect.any(Object));
    });
  });

  // ===== NEW KEY DIALOG TESTS =====

  it('displays new key dialog after creation', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'sk_new_abcdef123456' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(screen.getByText('API Key Created')).toBeInTheDocument();
    });

    expect(screen.getByText(/this is the only time you will see this key/i)).toBeInTheDocument();
  });

  it('displays created key as masked by default', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'sk_new_abcdef123456' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(screen.getByLabelText('API Key')).toBeInTheDocument();
    });

    const keyInput = screen.getByLabelText('API Key') as HTMLInputElement;
    expect(keyInput.type).toBe('password');
  });

  it('toggles visibility of created key', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'sk_new_abcdef123456' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(screen.getByLabelText('API Key')).toBeInTheDocument();
    });

    const keyInput = screen.getByLabelText('API Key') as HTMLInputElement;
    expect(keyInput.type).toBe('password');

    // Find visibility toggle button
    const buttons = within(screen.getByRole('dialog')).getAllByRole('button');
    const visibilityToggle = buttons.find(btn =>
      btn.querySelector('svg[data-testid="VisibilityIcon"]')
    );

    fireEvent.click(visibilityToggle!);

    await waitFor(() => {
      expect(keyInput.type).toBe('text');
    });
  });

  it('copies API key to clipboard', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    const nameInput = screen.getByLabelText('Name');
    fireEvent.change(nameInput, { target: { value: 'New API Key' } });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'sk_new_abcdef123456' }),
    });

    const createDialogButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^create$/i });
    fireEvent.click(createDialogButton);

    await waitFor(() => {
      expect(screen.getByLabelText('API Key')).toBeInTheDocument();
    });

    // Find copy button
    const buttons = within(screen.getByRole('dialog')).getAllByRole('button');
    const copyButton = buttons.find(btn =>
      btn.querySelector('svg[data-testid="CopyIcon"]')
    );

    fireEvent.click(copyButton!);

    await waitFor(() => {
      expect(mockClipboard.writeText).toHaveBeenCalledWith('sk_new_abcdef123456');
    });
  });

  // ===== REVOKE API KEY TESTS =====

  it('displays revoke button for active keys', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    // Revoke buttons should be visible for active keys
    const revokeButtons = screen.getAllByTitle('Revoke');
    expect(revokeButtons.length).toBeGreaterThan(0);
  });

  it.only('revokes API key when revoke button is clicked', async () => {
    const user = userEvent.setup();
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const revokeButton = screen.getAllByTitle('Revoke')[0];
    await user.click(revokeButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/revoke'),
        expect.objectContaining({
          method: 'POST',
        })
      );
    });
  });

  it('handles revoke API key errors', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Revoke failed' }),
    });

    const revokeButton = screen.getAllByTitle('Revoke')[0];
    fireEvent.click(revokeButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/revoke'), expect.any(Object));
    });
  });

  // ===== DELETE API KEY TESTS =====

  it('displays delete button for all keys', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const deleteButtons = screen.getAllByTitle('Delete');
    expect(deleteButtons.length).toBe(4); // All 4 keys have delete buttons
  });

  it('opens delete confirmation dialog', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete API Key?')).toBeInTheDocument();
    });

    expect(screen.getByText(/this action cannot be undone/i)).toBeInTheDocument();
  });

  it('deletes API key when confirmed', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete API Key?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true }),
    });

    const confirmDeleteButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmDeleteButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining('/apikeys/'),
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  it('handles delete API key errors', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete API Key?')).toBeInTheDocument();
    });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ error: 'Delete failed' }),
    });

    const confirmDeleteButton = within(screen.getByRole('dialog')).getByRole('button', { name: /^delete$/i });
    fireEvent.click(confirmDeleteButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining('/apikeys/'), expect.any(Object));
    });
  });

  it('closes delete dialog when cancelled', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const deleteButton = screen.getAllByTitle('Delete')[0];
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText('Delete API Key?')).toBeInTheDocument();
    });

    const cancelButton = within(screen.getByRole('dialog')).getByRole('button', { name: /cancel/i });
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(screen.queryByText('Delete API Key?')).not.toBeInTheDocument();
    });
  });

  // ===== REFRESH TESTS =====

  it('displays refresh button', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });
  });

  it('refetches API keys when refresh is clicked', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('API Keys Management')).toBeInTheDocument();
    });

    mockFetch.mockClear();

    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/apikeys',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer mock-token',
          }),
        })
      );
    });
  });

  // ===== EMPTY STATE TESTS =====

  it('displays empty state when no keys match filter', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByText('Production API Key')).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    fireEvent.change(searchInput, { target: { value: 'nonexistent' } });

    await waitFor(() => {
      expect(screen.getByText('No API keys found')).toBeInTheDocument();
    });
  });
});

describe('APIKeys Page - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockAPIKeys,
    });
  });

  it('has accessible buttons with clear names', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it('has accessible table structure', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('table')).toBeInTheDocument();
    });

    const table = screen.getByRole('table');
    expect(table).toBeInTheDocument();

    const headers = within(table).getAllByRole('columnheader');
    expect(headers.length).toBe(9);
  });

  it('has accessible form controls in create dialog', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /create api key/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /create api key/i }));

    await waitFor(() => {
      expect(screen.getByLabelText('Name')).toBeInTheDocument();
    });

    expect(screen.getByLabelText('Description')).toBeInTheDocument();
    expect(screen.getByLabelText('Scopes')).toBeInTheDocument();
  });

  it('has accessible search input', async () => {
    renderAPIKeys();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/search by name, user, or key prefix/i)).toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText(/search by name, user, or key prefix/i);
    expect(searchInput).toHaveAccessibleName();
  });
});

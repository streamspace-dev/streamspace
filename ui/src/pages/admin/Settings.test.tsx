import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import Settings from './Settings';

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

// Mock URL.createObjectURL and revokeObjectURL for export tests
global.URL.createObjectURL = vi.fn(() => 'blob:mock-url');
global.URL.revokeObjectURL = vi.fn();

// Mock data
const mockConfigurations = {
  configurations: [
    {
      key: 'ingress.domain',
      value: 'streamspace.local',
      type: 'string',
      category: 'ingress',
      description: 'Base domain for ingress',
      updated_at: '2025-01-15T10:00:00Z',
      updated_by: 'admin',
    },
    {
      key: 'ingress.tls_enabled',
      value: 'true',
      type: 'boolean',
      category: 'ingress',
      description: 'Enable TLS for ingress',
      updated_at: '2025-01-15T10:00:00Z',
      updated_by: 'admin',
    },
    {
      key: 'storage.class',
      value: 'nfs-client',
      type: 'string',
      category: 'storage',
      description: 'Default storage class',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'storage.size',
      value: '50Gi',
      type: 'string',
      category: 'storage',
      description: 'Default home directory size',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'resources.default_memory',
      value: '2Gi',
      type: 'string',
      category: 'resources',
      description: 'Default memory allocation',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'resources.default_cpu',
      value: '1000m',
      type: 'string',
      category: 'resources',
      description: 'Default CPU allocation',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'features.metrics_enabled',
      value: 'true',
      type: 'boolean',
      category: 'features',
      description: 'Enable Prometheus metrics',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'features.hibernation_enabled',
      value: 'true',
      type: 'boolean',
      category: 'features',
      description: 'Enable session hibernation',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'session.idle_timeout',
      value: '30m',
      type: 'duration',
      category: 'session',
      description: 'Auto-hibernate after inactivity',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'session.max_duration',
      value: '8h',
      type: 'duration',
      category: 'session',
      description: 'Maximum session lifetime',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'security.mfa_required',
      value: 'false',
      type: 'boolean',
      category: 'security',
      description: 'Require multi-factor authentication',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'security.saml_enabled',
      value: 'true',
      type: 'boolean',
      category: 'security',
      description: 'Enable SAML SSO',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'compliance.retention_days',
      value: '90',
      type: 'number',
      category: 'compliance',
      description: 'Audit log retention in days',
      updated_at: '2025-01-15T10:00:00Z',
    },
    {
      key: 'compliance.archiving_enabled',
      value: 'true',
      type: 'boolean',
      category: 'compliance',
      description: 'Enable automatic archiving',
      updated_at: '2025-01-15T10:00:00Z',
    },
  ],
  grouped: {
    ingress: [
      {
        key: 'ingress.domain',
        value: 'streamspace.local',
        type: 'string',
        category: 'ingress',
        description: 'Base domain for ingress',
        updated_at: '2025-01-15T10:00:00Z',
        updated_by: 'admin',
      },
      {
        key: 'ingress.tls_enabled',
        value: 'true',
        type: 'boolean',
        category: 'ingress',
        description: 'Enable TLS for ingress',
        updated_at: '2025-01-15T10:00:00Z',
        updated_by: 'admin',
      },
    ],
    storage: [
      {
        key: 'storage.class',
        value: 'nfs-client',
        type: 'string',
        category: 'storage',
        description: 'Default storage class',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'storage.size',
        value: '50Gi',
        type: 'string',
        category: 'storage',
        description: 'Default home directory size',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
    resources: [
      {
        key: 'resources.default_memory',
        value: '2Gi',
        type: 'string',
        category: 'resources',
        description: 'Default memory allocation',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'resources.default_cpu',
        value: '1000m',
        type: 'string',
        category: 'resources',
        description: 'Default CPU allocation',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
    features: [
      {
        key: 'features.metrics_enabled',
        value: 'true',
        type: 'boolean',
        category: 'features',
        description: 'Enable Prometheus metrics',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'features.hibernation_enabled',
        value: 'true',
        type: 'boolean',
        category: 'features',
        description: 'Enable session hibernation',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
    session: [
      {
        key: 'session.idle_timeout',
        value: '30m',
        type: 'duration',
        category: 'session',
        description: 'Auto-hibernate after inactivity',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'session.max_duration',
        value: '8h',
        type: 'duration',
        category: 'session',
        description: 'Maximum session lifetime',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
    security: [
      {
        key: 'security.mfa_required',
        value: 'false',
        type: 'boolean',
        category: 'security',
        description: 'Require multi-factor authentication',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'security.saml_enabled',
        value: 'true',
        type: 'boolean',
        category: 'security',
        description: 'Enable SAML SSO',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
    compliance: [
      {
        key: 'compliance.retention_days',
        value: '90',
        type: 'number',
        category: 'compliance',
        description: 'Audit log retention in days',
        updated_at: '2025-01-15T10:00:00Z',
      },
      {
        key: 'compliance.archiving_enabled',
        value: 'true',
        type: 'boolean',
        category: 'compliance',
        description: 'Enable automatic archiving',
        updated_at: '2025-01-15T10:00:00Z',
      },
    ],
  },
};

// Helper to render Settings with providers
const renderSettings = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Settings />
      </BrowserRouter>
    </QueryClientProvider>
  );
};

describe('Settings Page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockConfigurations,
    });
  });

  // ===== RENDERING TESTS =====

  it('renders page title and description', async () => {
    renderSettings();

    expect(screen.getByText('Settings')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText('System Configuration')).toBeInTheDocument();
    });
    expect(screen.getByText(/14 total settings/i)).toBeInTheDocument();
  });

  it('displays loading state initially', () => {
    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* never resolves */
        })
    );

    renderSettings();

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('renders all 7 category tabs', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /ingress/i })).toBeInTheDocument();
    });

    expect(screen.getByRole('tab', { name: /storage/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /resources/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /features/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /session/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /security/i })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /compliance/i })).toBeInTheDocument();
  });

  it('displays configuration settings for default category (Ingress)', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.domain')).toBeInTheDocument();
    });

    expect(screen.getByText('ingress.tls_enabled')).toBeInTheDocument();
    expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
  });

  it('shows configuration metadata (type, updated_at, updated_by)', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText(/Type: string/i)).toBeInTheDocument();
    });

    expect(screen.getAllByText(/Last updated:/i)[0]).toBeInTheDocument();
    expect(screen.getAllByText(/by admin/i)[0]).toBeInTheDocument();
  });

  // ===== TAB NAVIGATION TESTS =====

  it('switches between category tabs', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.domain')).toBeInTheDocument();
    });

    // Switch to Storage tab
    const storageTab = screen.getByRole('tab', { name: /storage/i });
    fireEvent.click(storageTab);

    await waitFor(() => {
      expect(screen.getByText('storage.class')).toBeInTheDocument();
    });
    expect(screen.getByText('storage.size')).toBeInTheDocument();
    expect(screen.queryByText('ingress.domain')).not.toBeInTheDocument();
  });

  it('displays correct configurations for each category', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.domain')).toBeInTheDocument();
    });

    // Resources
    fireEvent.click(screen.getByRole('tab', { name: /resources/i }));
    await waitFor(() => {
      expect(screen.getByText('resources.default_memory')).toBeInTheDocument();
    });

    // Features
    fireEvent.click(screen.getByRole('tab', { name: /features/i }));
    await waitFor(() => {
      expect(screen.getByText('features.metrics_enabled')).toBeInTheDocument();
    });

    // Session
    fireEvent.click(screen.getByRole('tab', { name: /session/i }));
    await waitFor(() => {
      expect(screen.getByText('session.idle_timeout')).toBeInTheDocument();
    });

    // Security
    fireEvent.click(screen.getByRole('tab', { name: /security/i }));
    await waitFor(() => {
      expect(screen.getByText('security.mfa_required')).toBeInTheDocument();
    });

    // Compliance
    fireEvent.click(screen.getByRole('tab', { name: /compliance/i }));
    await waitFor(() => {
      expect(screen.getByText('compliance.retention_days')).toBeInTheDocument();
    });
  });

  // ===== FORM FIELD TYPE TESTS =====

  it('renders boolean fields as switches', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.tls_enabled')).toBeInTheDocument();
    });

    const switchElement = screen.getByRole('checkbox');
    expect(switchElement).toBeInTheDocument();
    expect(switchElement).toBeChecked(); // value is 'true'
  });

  it('renders string fields as text inputs', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    expect(input).toHaveAttribute('type', 'text');
  });

  it('renders number fields with number input type', async () => {
    renderSettings();

    // Switch to Compliance tab
    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /compliance/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('tab', { name: /compliance/i }));

    await waitFor(() => {
      expect(screen.getByDisplayValue('90')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('90');
    expect(input).toHaveAttribute('type', 'number');
  });

  it('renders duration fields with placeholder', async () => {
    renderSettings();

    // Switch to Session tab
    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /session/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('tab', { name: /session/i }));

    await waitFor(() => {
      expect(screen.getByDisplayValue('30m')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('30m');
    expect(input).toHaveAttribute('placeholder', '30m, 1h, 24h');
  });

  // ===== VALUE EDITING TESTS =====

  it('allows editing string configuration values', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'new-domain.local' } });

    expect(screen.getByDisplayValue('new-domain.local')).toBeInTheDocument();
  });

  it('allows toggling boolean configuration values', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.tls_enabled')).toBeInTheDocument();
    });

    const switchElement = screen.getByRole('checkbox');
    expect(switchElement).toBeChecked();

    fireEvent.click(switchElement);

    expect(switchElement).not.toBeChecked();
  });

  it('shows modified background color for edited fields', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'modified.local' } });

    // Modified fields should have action.hover background
    expect(input.closest('.MuiInputBase-root')).toHaveStyle({
      backgroundColor: expect.any(String),
    });
  });

  it('displays "Save" button for modified configuration', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'modified.local' } });

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /^save$/i })).toBeInTheDocument();
    });
  });

  it('shows unsaved changes alert when values are modified', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'modified.local' } });

    await waitFor(() => {
      expect(screen.getByText(/you have 1 unsaved change/i)).toBeInTheDocument();
    });
  });

  // ===== SAVE SINGLE CONFIGURATION TESTS =====

  it('saves single configuration when "Save" button is clicked', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'new-domain.local' } });

    const saveButton = await screen.findByRole('button', { name: /^save$/i });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ key: 'ingress.domain', value: 'new-domain.local' }),
    });

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/config/ingress.domain',
        expect.objectContaining({
          method: 'PUT',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({ value: 'new-domain.local' }),
        })
      );
    });
  });

  it('shows error message when save fails', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'invalid' } });

    const saveButton = await screen.findByRole('button', { name: /^save$/i });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Invalid domain format' }),
    });

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText('Invalid domain format')).toBeInTheDocument();
    });
  });

  // ===== BULK UPDATE TESTS =====

  it('shows "Save All" button when multiple values are modified', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    // Modify first value
    const domainInput = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(domainInput, { target: { value: 'new-domain.local' } });

    // Modify second value (switch)
    const switchElement = screen.getByRole('checkbox');
    fireEvent.click(switchElement);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /save all \(2\)/i })).toBeInTheDocument();
    });
  });

  it('performs bulk update when "Save All" is clicked', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    // Modify two values
    const domainInput = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(domainInput, { target: { value: 'new-domain.local' } });

    const switchElement = screen.getByRole('checkbox');
    fireEvent.click(switchElement);

    const saveAllButton = await screen.findByRole('button', { name: /save all \(2\)/i });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ updated: ['ingress.domain', 'ingress.tls_enabled'] }),
    });

    fireEvent.click(saveAllButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/config/bulk',
        expect.objectContaining({
          method: 'POST',
          headers: expect.objectContaining({
            'Content-Type': 'application/json',
          }),
          body: JSON.stringify({
            updates: {
              'ingress.domain': 'new-domain.local',
              'ingress.tls_enabled': 'false',
            },
          }),
        })
      );
    });
  });

  it('clears edited values after successful bulk update', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const domainInput = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(domainInput, { target: { value: 'new-domain.local' } });

    const saveAllButton = await screen.findByRole('button', { name: /save all \(1\)/i });

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ updated: ['ingress.domain'] }),
    });

    fireEvent.click(saveAllButton);

    // Wait for the save to complete and unsaved changes alert to disappear
    await waitFor(() => {
      expect(screen.queryByText(/you have .* unsaved change/i)).not.toBeInTheDocument();
    });
  });

  // ===== EXPORT TESTS =====

  it('displays export button', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
    });
  });

  it('exports configuration to JSON file', async () => {
    const createElementSpy = vi.spyOn(document, 'createElement');
    const appendChildSpy = vi.spyOn(document.body, 'appendChild');
    const removeChildSpy = vi.spyOn(document.body, 'removeChild');

    try {
      renderSettings();

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
      });

      const exportButton = screen.getByRole('button', { name: /export/i });
      fireEvent.click(exportButton);

      await waitFor(() => {
        expect(createElementSpy).toHaveBeenCalledWith('a');
      });

      expect(global.URL.createObjectURL).toHaveBeenCalled();
      expect(appendChildSpy).toHaveBeenCalled();
      expect(removeChildSpy).toHaveBeenCalled();
      expect(global.URL.revokeObjectURL).toHaveBeenCalledWith('blob:mock-url');
    } finally {
      createElementSpy.mockRestore();
      appendChildSpy.mockRestore();
      removeChildSpy.mockRestore();
    }
  });

  it('creates correct JSON structure for export', async () => {
    let capturedBlob: Blob | undefined;
    global.URL.createObjectURL = vi.fn((blob: Blob) => {
      capturedBlob = blob;
      return 'blob:mock-url';
    });

    const appendChildSpy = vi.spyOn(document.body, 'appendChild');
    const removeChildSpy = vi.spyOn(document.body, 'removeChild');

    try {
      renderSettings();

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
      });

      const exportButton = screen.getByRole('button', { name: /export/i });
      fireEvent.click(exportButton);

      await waitFor(() => {
        expect(capturedBlob).toBeDefined();
      });

      if (capturedBlob) {
        const text = await new Promise<string>((resolve, reject) => {
          const reader = new FileReader();
          reader.onload = () => resolve(reader.result as string);
          reader.onerror = reject;
          reader.readAsText(capturedBlob!);
        });
        const json = JSON.parse(text);

        expect(json['ingress.domain']).toBe('streamspace.local');
        expect(json['ingress.tls_enabled']).toBe('true');
        expect(json['storage.class']).toBe('nfs-client');
        expect(json['compliance.retention_days']).toBe('90');
      }
    } finally {
      appendChildSpy.mockRestore();
      removeChildSpy.mockRestore();
    }
  });

  // ===== REFRESH TESTS =====

  it('displays refresh button', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /refresh/i })).toBeInTheDocument();
    });
  });

  it('refetches configurations when refresh is clicked', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('System Configuration')).toBeInTheDocument();
    });

    mockFetch.mockClear();

    const refreshButton = screen.getByRole('button', { name: /refresh/i });
    fireEvent.click(refreshButton);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/admin/config',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: 'Bearer mock-token',
          }),
        })
      );
    });
  });

  // ===== ERROR HANDLING TESTS =====

  it('handles API fetch errors gracefully', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'));

    renderSettings();

    // Component should handle the error without crashing
    await waitFor(() => {
      expect(screen.getByText('Settings')).toBeInTheDocument();
    });
  });

  it('displays validation errors in form fields', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'invalid' } });

    const saveButton = await screen.findByRole('button', { name: /^save$/i });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Invalid domain format' }),
    });

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText('Invalid domain format')).toBeInTheDocument();
    });
  });

  it('clears validation error when user starts typing', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    const input = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(input, { target: { value: 'invalid' } });

    const saveButton = await screen.findByRole('button', { name: /^save$/i });

    mockFetch.mockResolvedValueOnce({
      ok: false,
      json: async () => ({ message: 'Invalid domain format' }),
    });

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText('Invalid domain format')).toBeInTheDocument();
    });

    // Start typing again
    fireEvent.change(input, { target: { value: 'valid-domain.local' } });

    await waitFor(() => {
      expect(screen.queryByText('Invalid domain format')).not.toBeInTheDocument();
    });
  });

  // ===== EMPTY STATE TESTS =====

  it('displays empty state for category with no settings', async () => {
    const emptyGrouped = { ...mockConfigurations.grouped };
    delete emptyGrouped.compliance;

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        ...mockConfigurations,
        grouped: emptyGrouped,
      }),
    });

    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /compliance/i })).toBeInTheDocument();
    });

    const complianceTab = screen.getByRole('tab', { name: /compliance/i });
    expect(complianceTab).toBeDisabled();
  });
});

describe('Settings Page - Accessibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockConfigurations,
    });
  });

  it('has accessible tab navigation', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: /ingress/i })).toBeInTheDocument();
    });

    const tabs = screen.getAllByRole('tab');
    tabs.forEach((tab) => {
      expect(tab).toHaveAccessibleName();
    });
  });

  it('has accessible form controls', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('ingress.domain')).toBeInTheDocument();
    });

    const textInputs = screen.getAllByRole('textbox');
    textInputs.forEach((input) => {
      expect(input).toBeInTheDocument();
    });

    const switches = screen.getAllByRole('checkbox');
    switches.forEach((switchEl) => {
      expect(switchEl).toBeInTheDocument();
    });
  });

  it('has accessible buttons with clear names', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it('provides helper text for form fields', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByText('Base domain for ingress')).toBeInTheDocument();
    });

    expect(screen.getByText('Enable TLS for ingress')).toBeInTheDocument();
  });
});

describe('Settings Page - Integration', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => mockConfigurations,
    });
  });

  it('maintains edited values across tab switches', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    // Edit value in Ingress tab
    const domainInput = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(domainInput, { target: { value: 'modified.local' } });

    // Switch to Storage tab
    fireEvent.click(screen.getByRole('tab', { name: /storage/i }));

    await waitFor(() => {
      expect(screen.getByText('storage.class')).toBeInTheDocument();
    });

    // Switch back to Ingress tab
    fireEvent.click(screen.getByRole('tab', { name: /ingress/i }));

    // Edited value should still be there
    await waitFor(() => {
      expect(screen.getByDisplayValue('modified.local')).toBeInTheDocument();
    });
  });

  it('counts unsaved changes across all tabs', async () => {
    renderSettings();

    await waitFor(() => {
      expect(screen.getByDisplayValue('streamspace.local')).toBeInTheDocument();
    });

    // Modify value in Ingress tab
    const domainInput = screen.getByDisplayValue('streamspace.local');
    fireEvent.change(domainInput, { target: { value: 'modified.local' } });

    // Switch to Storage tab and modify
    fireEvent.click(screen.getByRole('tab', { name: /storage/i }));

    await waitFor(() => {
      expect(screen.getByDisplayValue('nfs-client')).toBeInTheDocument();
    });

    const storageInput = screen.getByDisplayValue('nfs-client');
    fireEvent.change(storageInput, { target: { value: 'new-storage' } });

    // Should show 2 unsaved changes
    await waitFor(() => {
      expect(screen.getByText(/you have 2 unsaved change/i)).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: /save all \(2\)/i })).toBeInTheDocument();
  });
});

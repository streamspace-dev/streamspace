import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import SecuritySettings from './SecuritySettings';
import * as api from '../lib/api';

// Mock the API module
vi.mock('../lib/api');

// Mock Layout component
vi.mock('../components/Layout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

// Mock QRCodeSVG component
vi.mock('qrcode.react', () => ({
  QRCodeSVG: ({ value }: { value: string }) => <div data-testid="qr-code">{value}</div>,
}));

const renderWithRouter = (component: React.ReactElement) => {
  return render(<BrowserRouter>{component}</BrowserRouter>);
};

describe('SecuritySettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('MFA Methods Tab', () => {
    it('renders MFA methods tab', () => {
      renderWithRouter(<SecuritySettings />);

      expect(screen.getByText('Authenticator App')).toBeInTheDocument();
      expect(screen.getByText('SMS')).toBeInTheDocument();
      expect(screen.getByText('Email')).toBeInTheDocument();
    });

    it('displays setup instructions for TOTP', async () => {
      const mockSetupMFA = vi.spyOn(api, 'setupMFA').mockResolvedValue({
        mfa_id: 1,
        secret: 'JBSWY3DPEHPK3PXP',
        qr_code_url: 'otpauth://totp/StreamSpace:user@example.com?secret=JBSWY3DPEHPK3PXP',
      });

      renderWithRouter(<SecuritySettings />);

      const setupButton = screen.getAllByText('Set Up')[0];
      fireEvent.click(setupButton);

      await waitFor(() => {
        expect(mockSetupMFA).toHaveBeenCalledWith('totp');
      });

      // MFA setup dialog should open
      await waitFor(() => {
        expect(screen.getByText(/Scan this QR code/i)).toBeInTheDocument();
      });
    });

    it('shows verification step after QR code scan', async () => {
      vi.spyOn(api, 'setupMFA').mockResolvedValue({
        mfa_id: 1,
        secret: 'JBSWY3DPEHPK3PXP',
        qr_code_url: 'otpauth://totp/StreamSpace:user@example.com?secret=JBSWY3DPEHPK3PXP',
      });

      renderWithRouter(<SecuritySettings />);

      const setupButton = screen.getAllByText('Set Up')[0];
      fireEvent.click(setupButton);

      await waitFor(() => {
        expect(screen.getByTestId('qr-code')).toBeInTheDocument();
      });

      const nextButton = screen.getByText('Next');
      fireEvent.click(nextButton);

      expect(screen.getByText(/Enter the 6-digit code/i)).toBeInTheDocument();
    });

    it('verifies MFA code and displays backup codes', async () => {
      const mockSetupMFA = vi.spyOn(api, 'setupMFA').mockResolvedValue({
        mfa_id: 1,
        secret: 'JBSWY3DPEHPK3PXP',
        qr_code_url: 'otpauth://totp/StreamSpace:user@example.com?secret=JBSWY3DPEHPK3PXP',
      });

      const mockVerifyMFA = vi.spyOn(api, 'verifyMFASetup').mockResolvedValue({
        verified: true,
        backup_codes: ['ABC123-DEF456', 'GHI789-JKL012'],
      });

      renderWithRouter(<SecuritySettings />);

      // Step 1: Setup
      const setupButton = screen.getAllByText('Set Up')[0];
      fireEvent.click(setupButton);

      await waitFor(() => {
        expect(mockSetupMFA).toHaveBeenCalled();
      });

      // Step 2: Next
      const nextButton = screen.getByText('Next');
      fireEvent.click(nextButton);

      // Step 3: Verify
      const codeInput = screen.getByPlaceholderText(/Enter 6-digit code/i);
      fireEvent.change(codeInput, { target: { value: '123456' } });

      const verifyButton = screen.getByText('Verify');
      fireEvent.click(verifyButton);

      await waitFor(() => {
        expect(mockVerifyMFA).toHaveBeenCalledWith(1, '123456');
      });

      // Step 4: Backup codes
      await waitFor(() => {
        expect(screen.getByText(/Save these backup codes/i)).toBeInTheDocument();
        expect(screen.getByText('ABC123-DEF456')).toBeInTheDocument();
        expect(screen.getByText('GHI789-JKL012')).toBeInTheDocument();
      });
    });

    it('handles verification error', async () => {
      vi.spyOn(api, 'setupMFA').mockResolvedValue({
        mfa_id: 1,
        secret: 'JBSWY3DPEHPK3PXP',
        qr_code_url: 'otpauth://totp/StreamSpace:user@example.com?secret=JBSWY3DPEHPK3PXP',
      });

      vi.spyOn(api, 'verifyMFASetup').mockRejectedValue(new Error('Invalid code'));

      renderWithRouter(<SecuritySettings />);

      const setupButton = screen.getAllByText('Set Up')[0];
      fireEvent.click(setupButton);

      await waitFor(() => {
        expect(screen.getByText('Next')).toBeInTheDocument();
      });

      fireEvent.click(screen.getByText('Next'));

      const codeInput = screen.getByPlaceholderText(/Enter 6-digit code/i);
      fireEvent.change(codeInput, { target: { value: '000000' } });

      fireEvent.click(screen.getByText('Verify'));

      await waitFor(() => {
        expect(screen.getByText(/Invalid code/i)).toBeInTheDocument();
      });
    });
  });

  describe('IP Whitelist Tab', () => {
    it('renders IP whitelist tab', () => {
      renderWithRouter(<SecuritySettings />);

      const ipWhitelistTab = screen.getByText('IP Whitelist');
      fireEvent.click(ipWhitelistTab);

      expect(screen.getByText('Add IP Address')).toBeInTheDocument();
    });

    it('adds new IP address', async () => {
      const mockCreateIPWhitelist = vi.spyOn(api, 'createIPWhitelist').mockResolvedValue({
        id: 1,
      });

      vi.spyOn(api, 'listIPWhitelist').mockResolvedValue({
        entries: [],
      });

      renderWithRouter(<SecuritySettings />);

      const ipWhitelistTab = screen.getByText('IP Whitelist');
      fireEvent.click(ipWhitelistTab);

      const addButton = screen.getByText('Add IP Address');
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(screen.getByLabelText(/IP Address or CIDR/i)).toBeInTheDocument();
      });

      const ipInput = screen.getByLabelText(/IP Address or CIDR/i);
      fireEvent.change(ipInput, { target: { value: '192.168.1.100' } });

      const descInput = screen.getByLabelText(/Description/i);
      fireEvent.change(descInput, { target: { value: 'Office IP' } });

      const saveButton = screen.getByText('Save');
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockCreateIPWhitelist).toHaveBeenCalledWith({
          ip_address: '192.168.1.100',
          description: 'Office IP',
          enabled: true,
        });
      });
    });

    it('validates IP address format', async () => {
      renderWithRouter(<SecuritySettings />);

      const ipWhitelistTab = screen.getByText('IP Whitelist');
      fireEvent.click(ipWhitelistTab);

      const addButton = screen.getByText('Add IP Address');
      fireEvent.click(addButton);

      const ipInput = screen.getByLabelText(/IP Address or CIDR/i);
      fireEvent.change(ipInput, { target: { value: 'invalid-ip' } });

      const saveButton = screen.getByText('Save');
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(screen.getByText(/Invalid IP address/i)).toBeInTheDocument();
      });
    });

    it('deletes IP whitelist entry', async () => {
      const mockDeleteIPWhitelist = vi.spyOn(api, 'deleteIPWhitelist').mockResolvedValue();

      vi.spyOn(api, 'listIPWhitelist').mockResolvedValue({
        entries: [
          {
            id: 1,
            ip_address: '192.168.1.100',
            description: 'Office IP',
            enabled: true,
            created_at: '2025-11-15T10:00:00Z',
          },
        ],
      });

      renderWithRouter(<SecuritySettings />);

      const ipWhitelistTab = screen.getByText('IP Whitelist');
      fireEvent.click(ipWhitelistTab);

      await waitFor(() => {
        expect(screen.getByText('192.168.1.100')).toBeInTheDocument();
      });

      const deleteButton = screen.getByLabelText(/delete/i);
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(mockDeleteIPWhitelist).toHaveBeenCalledWith(1);
      });
    });
  });

  describe('Security Alerts Tab', () => {
    it('renders security alerts tab', () => {
      renderWithRouter(<SecuritySettings />);

      const alertsTab = screen.getByText('Security Alerts');
      fireEvent.click(alertsTab);

      expect(screen.getByText(/Recent security alerts/i)).toBeInTheDocument();
    });

    it('displays security alerts', async () => {
      vi.spyOn(api, 'getSecurityAlerts').mockResolvedValue({
        alerts: [
          {
            id: 1,
            type: 'failed_login',
            severity: 'high',
            message: 'Multiple failed login attempts',
            created_at: '2025-11-15T10:00:00Z',
            status: 'open',
          },
          {
            id: 2,
            type: 'ip_violation',
            severity: 'medium',
            message: 'Access from non-whitelisted IP',
            created_at: '2025-11-15T09:00:00Z',
            status: 'acknowledged',
          },
        ],
      });

      renderWithRouter(<SecuritySettings />);

      const alertsTab = screen.getByText('Security Alerts');
      fireEvent.click(alertsTab);

      await waitFor(() => {
        expect(screen.getByText('Multiple failed login attempts')).toBeInTheDocument();
        expect(screen.getByText('Access from non-whitelisted IP')).toBeInTheDocument();
      });
    });

    it('filters alerts by severity', async () => {
      vi.spyOn(api, 'getSecurityAlerts').mockResolvedValue({
        alerts: [
          {
            id: 1,
            type: 'failed_login',
            severity: 'high',
            message: 'Critical alert',
            created_at: '2025-11-15T10:00:00Z',
            status: 'open',
          },
        ],
      });

      renderWithRouter(<SecuritySettings />);

      const alertsTab = screen.getByText('Security Alerts');
      fireEvent.click(alertsTab);

      const severityFilter = screen.getByLabelText(/Severity/i);
      fireEvent.change(severityFilter, { target: { value: 'high' } });

      await waitFor(() => {
        expect(api.getSecurityAlerts).toHaveBeenCalledWith({ severity: 'high' });
      });
    });
  });

  describe('Active MFA Methods Tab', () => {
    it('displays active MFA methods', async () => {
      vi.spyOn(api, 'listMFAMethods').mockResolvedValue({
        methods: [
          {
            id: 1,
            user_id: 'user1',
            type: 'totp',
            enabled: true,
            verified: true,
            is_primary: true,
            created_at: '2025-11-15T10:00:00Z',
          },
          {
            id: 2,
            user_id: 'user1',
            type: 'email',
            enabled: true,
            verified: true,
            is_primary: false,
            created_at: '2025-11-15T11:00:00Z',
          },
        ],
      });

      renderWithRouter(<SecuritySettings />);

      const methodsTab = screen.getByText('Active MFA Methods');
      fireEvent.click(methodsTab);

      await waitFor(() => {
        expect(screen.getByText('TOTP')).toBeInTheDocument();
        expect(screen.getByText('Email')).toBeInTheDocument();
        expect(screen.getByText('Primary')).toBeInTheDocument();
      });
    });

    it('deletes MFA method', async () => {
      const mockDeleteMFA = vi.spyOn(api, 'deleteMFAMethod').mockResolvedValue();

      vi.spyOn(api, 'listMFAMethods').mockResolvedValue({
        methods: [
          {
            id: 1,
            user_id: 'user1',
            type: 'totp',
            enabled: true,
            verified: true,
            is_primary: true,
            created_at: '2025-11-15T10:00:00Z',
          },
        ],
      });

      renderWithRouter(<SecuritySettings />);

      const methodsTab = screen.getByText('Active MFA Methods');
      fireEvent.click(methodsTab);

      await waitFor(() => {
        expect(screen.getByText('TOTP')).toBeInTheDocument();
      });

      const deleteButton = screen.getByLabelText(/delete/i);
      fireEvent.click(deleteButton);

      await waitFor(() => {
        expect(mockDeleteMFA).toHaveBeenCalledWith(1);
      });
    });
  });
});

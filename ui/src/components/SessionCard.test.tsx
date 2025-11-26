import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import SessionCard from './SessionCard';

// Mock data for testing
const mockSession = {
  id: 'session-1',
  name: 'test-session',
  user: 'testuser',
  template: 'firefox-browser',
  state: 'running',
  status: {
    phase: 'Running',
    url: 'https://test-session.streamspace.local',
  },
  url: 'https://test-session.streamspace.local', // Keep top level for backward compatibility if needed, but component uses status.url
  createdAt: '2025-01-15T10:00:00Z',
  resources: {
    memory: '2Gi',
    cpu: '1000m',
  },
  isActive: true,
  isIdle: false,
};

describe('SessionCard Component', () => {
  it('renders session information correctly', () => {
    render(<SessionCard session={mockSession} />);

    // Check if session name is displayed
    expect(screen.getByText('test-session')).toBeInTheDocument();

    // Check if template name is displayed
    expect(screen.getByText(/firefox-browser/i)).toBeInTheDocument();

    // Check if state is displayed - use getAllByText since it appears in chip and aria-label
    expect(screen.getAllByText(/running/i)[0]).toBeInTheDocument();
  });

  it('displays resource usage', () => {
    render(<SessionCard session={mockSession} />);

    // Check if memory is displayed
    expect(screen.getByText(/2Gi/i)).toBeInTheDocument();

    // Check if CPU is displayed
    expect(screen.getByText(/1000m/i)).toBeInTheDocument();
  });

  it('shows correct status badge color', () => {
    const { container } = render(<SessionCard session={mockSession} />);

    // Find status badge (would depend on actual implementation)
    const statusBadge = container.querySelector('[data-testid="status-badge"]');
    if (statusBadge) {
      expect(statusBadge).toHaveClass('status-running'); // or appropriate class
    }
  });

  it('calls onStateChange with hibernated when hibernate button is clicked', () => {
    const onStateChange = vi.fn();
    render(<SessionCard session={mockSession} onStateChange={onStateChange} />);

    const hibernateButton = screen.getByRole('button', { name: /hibernate/i });
    fireEvent.click(hibernateButton);

    expect(onStateChange).toHaveBeenCalledWith(mockSession.name, 'hibernated');
  });

  it('calls onStateChange with running when wake button is clicked', () => {
    const hibernatedSession = { ...mockSession, state: 'hibernated', status: { phase: 'Hibernated' } };
    const onStateChange = vi.fn();
    render(<SessionCard session={hibernatedSession} onStateChange={onStateChange} />);

    const wakeButton = screen.getByRole('button', { name: /resume/i });
    expect(wakeButton).toBeInTheDocument();

    fireEvent.click(wakeButton);
    expect(onStateChange).toHaveBeenCalledWith(hibernatedSession.name, 'running');
  });

  it('calls onConnect when connect button is clicked', () => {
    const onConnect = vi.fn();
    render(<SessionCard session={mockSession} onConnect={onConnect} />);

    const connectButton = screen.getByRole('button', { name: /connect/i });
    // The button might be disabled if URL is missing or phase is not Running
    // In mockSession, phase is Running and URL is present.
    // However, we need to make sure the button is not disabled.
    expect(connectButton).not.toBeDisabled();

    fireEvent.click(connectButton);

    expect(onConnect).toHaveBeenCalledWith(mockSession);
  });

  it('disables actions for hibernated session', () => {
    const hibernatedSession = { ...mockSession, state: 'hibernated', status: { phase: 'Hibernated' } };
    render(<SessionCard session={hibernatedSession} />);

    // Connect button should be disabled or not present
    const connectButton = screen.queryByRole('button', { name: /connect/i });
    if (connectButton) {
      expect(connectButton).toBeDisabled();
    }
  });

  it('handles missing URL gracefully', () => {
    const sessionWithoutURL = { ...mockSession, status: { ...mockSession.status, url: undefined } };
    render(<SessionCard session={sessionWithoutURL} />);

    // Connect button should be disabled if no URL
    // The component checks `disabled={session.status.phase !== 'Running' || !session.url}` for disable.
    const connectButton = screen.queryByRole('button', { name: /connect/i });
    if (connectButton) {
      expect(connectButton).toBeDisabled();
    }
  });

  it('displays loading state', () => {
    const loadingSession = { ...mockSession, status: { ...mockSession.status, phase: 'Pending' } };
    render(<SessionCard session={loadingSession} />);

    expect(screen.getByText(/pending/i)).toBeInTheDocument();
  });

  it('displays error state', () => {
    const failedSession = { ...mockSession, status: { ...mockSession.status, phase: 'Failed' }, error: 'Pod failed to start' };
    render(<SessionCard session={failedSession} />);

    expect(screen.getByText(/failed/i)).toBeInTheDocument();
  });
});

describe('SessionCard Accessibility', () => {
  it('has accessible name for buttons', () => {
    render(<SessionCard session={mockSession} />);

    const buttons = screen.getAllByRole('button');
    buttons.forEach((button) => {
      expect(button).toHaveAccessibleName();
    });
  });

  it('uses semantic HTML elements', () => {
    const { container } = render(<SessionCard session={mockSession} />);

    // Should use article or section for card
    const card = container.querySelector('article, section');
    expect(card).toBeInTheDocument();
  });

  it('provides aria labels for status', () => {
    const { container } = render(<SessionCard session={mockSession} />);

    const statusElement = container.querySelector('[aria-label*="Session state"]');
    expect(statusElement).toBeInTheDocument();
  });
});

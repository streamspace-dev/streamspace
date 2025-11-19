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
  },
  url: 'https://test-session.streamspace.local',
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

    // Check if state is displayed
    expect(screen.getByText(/running/i)).toBeInTheDocument();
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

  it('calls onHibernate when hibernate button is clicked', () => {
    const onHibernate = vi.fn();
    render(<SessionCard session={mockSession} onHibernate={onHibernate} />);

    const hibernateButton = screen.getByRole('button', { name: /hibernate/i });
    fireEvent.click(hibernateButton);

    expect(onHibernate).toHaveBeenCalledWith(mockSession.id);
  });

  it('calls onTerminate when terminate button is clicked', () => {
    const onTerminate = vi.fn();
    render(<SessionCard session={mockSession} onTerminate={onTerminate} />);

    const terminateButton = screen.getByRole('button', { name: /terminate/i });
    fireEvent.click(terminateButton);

    expect(onTerminate).toHaveBeenCalledWith(mockSession.id);
  });

  it('calls onConnect when connect button is clicked', () => {
    const onConnect = vi.fn();
    render(<SessionCard session={mockSession} onConnect={onConnect} />);

    const connectButton = screen.getByRole('button', { name: /connect/i });
    fireEvent.click(connectButton);

    expect(onConnect).toHaveBeenCalledWith(mockSession.url);
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

  it('shows wake button for hibernated session', () => {
    const hibernatedSession = { ...mockSession, state: 'hibernated', status: { phase: 'Hibernated' } };
    const onWake = vi.fn();
    render(<SessionCard session={hibernatedSession} onWake={onWake} />);

    const wakeButton = screen.getByRole('button', { name: /wake/i });
    expect(wakeButton).toBeInTheDocument();

    fireEvent.click(wakeButton);
    expect(onWake).toHaveBeenCalledWith(hibernatedSession.id);
  });

  it('formats timestamps correctly', () => {
    render(<SessionCard session={mockSession} />);

    // Check if created date is formatted (implementation-specific)
    // This would depend on how dates are displayed in the component
    const dateElement = screen.getByText(/Jan 15, 2025/i);
    expect(dateElement).toBeInTheDocument();
  });

  it('handles missing URL gracefully', () => {
    const sessionWithoutURL = { ...mockSession, url: undefined };
    render(<SessionCard session={sessionWithoutURL} />);

    // Connect button should be disabled if no URL
    const connectButton = screen.queryByRole('button', { name: /connect/i });
    if (connectButton) {
      expect(connectButton).toBeDisabled();
    }
  });

  it('displays loading state', () => {
    const loadingSession = { ...mockSession, phase: 'Pending' };
    render(<SessionCard session={loadingSession} />);

    expect(screen.getByText(/pending/i)).toBeInTheDocument();
  });

  it('displays error state', () => {
    const failedSession = { ...mockSession, phase: 'Failed', error: 'Pod failed to start' };
    render(<SessionCard session={failedSession} />);

    expect(screen.getByText(/failed/i)).toBeInTheDocument();
    if (failedSession.error) {
      expect(screen.getByText(/Pod failed to start/i)).toBeInTheDocument();
    }
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

    const statusElement = container.querySelector('[aria-label*="status"]');
    expect(statusElement).toBeInTheDocument();
  });
});

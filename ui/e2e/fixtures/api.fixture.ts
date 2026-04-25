/**
 * API Fixtures for Playwright E2E Tests
 *
 * Provides API mocking and helper functions for testing
 * StreamSpace UI with controlled backend responses.
 */

import { Page } from '@playwright/test';

export const API_URL = process.env.API_URL || 'http://localhost:8000';

/**
 * Mock session data for testing
 */
export const MOCK_SESSIONS = {
  running: {
    name: 'test-session-running',
    user: 'admin',
    template: 'chromium',
    state: 'running',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'selkies',
    streamingPort: 3000,
    streamingPath: '/websockify',
    status: {
      phase: 'Running',
      url: 'http://test-session-running.streamspace.svc.cluster.local:3000',
      podName: 'test-session-running-abc123',
    },
    activeConnections: 0,
    resources: { cpu: '500m', memory: '2Gi' },
  },
  hibernated: {
    name: 'test-session-hibernated',
    user: 'admin',
    template: 'firefox',
    state: 'hibernated',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'vnc',
    streamingPort: 5900,
    status: {
      phase: 'Hibernated',
    },
    activeConnections: 0,
    resources: { cpu: '500m', memory: '2Gi' },
  },
  vnc: {
    name: 'test-session-vnc',
    user: 'admin',
    template: 'firefox',
    state: 'running',
    platform: 'kubernetes',
    agent_id: 'k8s-agent-1',
    streamingProtocol: 'vnc',
    streamingPort: 5900,
    status: {
      phase: 'Running',
      url: 'http://test-session-vnc.streamspace.svc.cluster.local:5900',
      podName: 'test-session-vnc-def456',
    },
    activeConnections: 1,
    resources: { cpu: '500m', memory: '2Gi' },
  },
};

/**
 * Mock templates for testing
 */
export const MOCK_TEMPLATES = [
  {
    name: 'chromium',
    displayName: 'Chromium Browser',
    description: 'Chromium web browser',
    category: 'browsers',
    baseImage: 'lscr.io/linuxserver/chromium:latest',
    defaultResources: { memory: '2Gi', cpu: '500m' },
  },
  {
    name: 'firefox',
    displayName: 'Firefox Browser',
    description: 'Firefox web browser',
    category: 'browsers',
    baseImage: 'lscr.io/linuxserver/firefox:latest',
    defaultResources: { memory: '2Gi', cpu: '500m' },
  },
  {
    name: 'vscode',
    displayName: 'VS Code',
    description: 'Visual Studio Code editor',
    category: 'development',
    baseImage: 'lscr.io/linuxserver/code-server:latest',
    defaultResources: { memory: '4Gi', cpu: '1000m' },
  },
];

/**
 * Mock agent data for testing
 */
export const MOCK_AGENTS = [
  {
    agent_id: 'k8s-agent-1',
    name: 'K8s Agent 1',
    platform: 'kubernetes',
    region: 'us-east-1',
    status: 'online',
    capacity: { maxCpu: '64', maxMemory: '256Gi', maxSessions: 100 },
    current: { activeSessions: 5, cpuUsed: '2500m', memoryUsed: '10Gi' },
    last_heartbeat: new Date().toISOString(),
  },
];

/**
 * API Mock helper class for intercepting and mocking API calls
 */
export class APIMocker {
  private page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  /**
   * Mock all common API endpoints with default responses
   */
  async mockAllEndpoints(): Promise<void> {
    // Mock sessions list
    await this.page.route('**/api/v1/sessions', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([MOCK_SESSIONS.running, MOCK_SESSIONS.hibernated]),
        });
      } else if (route.request().method() === 'POST') {
        // Session creation
        const body = route.request().postDataJSON();
        const newSession = {
          ...MOCK_SESSIONS.running,
          name: `session-${Date.now()}`,
          template: body.template,
        };
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify(newSession),
        });
      } else {
        await route.continue();
      }
    });

    // Mock single session
    await this.page.route('**/api/v1/sessions/*', async (route) => {
      const url = route.request().url();
      const sessionId = url.split('/').pop()?.split('?')[0];

      if (route.request().method() === 'GET') {
        const session = Object.values(MOCK_SESSIONS).find(s => s.name === sessionId) || MOCK_SESSIONS.running;
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ...session, name: sessionId }),
        });
      } else {
        await route.continue();
      }
    });

    // Mock templates
    await this.page.route('**/api/v1/templates', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_TEMPLATES),
      });
    });

    // Mock agents
    await this.page.route('**/api/v1/agents', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(MOCK_AGENTS),
      });
    });

    // Mock auth endpoints
    await this.page.route('**/api/v1/auth/me', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          user_id: 'admin',
          username: 'admin',
          email: 'admin@streamspace.local',
          role: 'admin',
        }),
      });
    });
  }

  /**
   * Mock session connect endpoint
   */
  async mockSessionConnect(): Promise<void> {
    await this.page.route('**/api/v1/sessions/*/connect', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          connectionId: `conn-${Date.now()}`,
          sessionUrl: 'http://test.local:3000',
          state: 'running',
          message: 'Connected successfully',
        }),
      });
    });
  }

  /**
   * Mock session heartbeat endpoint
   */
  async mockHeartbeat(): Promise<void> {
    await this.page.route('**/api/v1/sessions/*/heartbeat', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ status: 'ok' }),
      });
    });
  }

  /**
   * Mock HTTP proxy for Selkies streaming
   */
  async mockHTTPProxy(): Promise<void> {
    await this.page.route('**/api/v1/http/**', async (route) => {
      // Return a simple HTML page that indicates the proxy is working
      await route.fulfill({
        status: 200,
        contentType: 'text/html',
        body: `
          <!DOCTYPE html>
          <html>
          <head><title>StreamSpace Session</title></head>
          <body data-testid="stream-content">
            <h1>Stream Connected</h1>
            <p>Session streaming is working</p>
          </body>
          </html>
        `,
      });
    });
  }

  /**
   * Mock API error response
   */
  async mockError(urlPattern: string, status: number, message: string): Promise<void> {
    await this.page.route(urlPattern, async (route) => {
      await route.fulfill({
        status,
        contentType: 'application/json',
        body: JSON.stringify({ error: message }),
      });
    });
  }
}

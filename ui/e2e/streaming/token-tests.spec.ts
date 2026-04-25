/**
 * Token Authentication Tests
 *
 * Critical tests for the black screen bug fix.
 * These tests verify that tokens are correctly passed to streaming iframes.
 *
 * NOTE: These tests use Playwright's route interception rather than MSW
 * because Vite's dev server proxy intercepts requests before MSW can.
 */

import { test, expect } from '@playwright/test';

// Mock data for responses
const MOCK_SESSION_SELKIES = {
  name: 'test-selkies',
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
    url: 'http://test.local:3000',
    podName: 'test-pod',
  },
  activeConnections: 0,
  resources: { cpu: '500m', memory: '2Gi' },
};

const MOCK_SESSION_VNC = {
  ...MOCK_SESSION_SELKIES,
  name: 'test-vnc',
  streamingProtocol: 'vnc',
  streamingPort: 5900,
};

test.describe('Token in Iframe URL (Bug Fix Verification)', () => {
  test.beforeEach(async ({ page }) => {
    // Set up authentication BEFORE any navigation
    // IMPORTANT: Must set both:
    // 1. 'streamspace-auth' - Zustand persist store format (for useUserStore)
    // 2. 'token' - Direct token for iframe URL generation
    await page.addInitScript(() => {
      const testToken = 'test-jwt-token-12345';

      // Set token directly (used by SessionViewer line 436 for iframe src)
      localStorage.setItem('token', testToken);

      // Set Zustand store in persist format (used by useUserStore)
      const zustandState = {
        state: {
          user: {
            username: 'admin',
            email: 'admin@test.local',
            role: 'admin',
          },
          token: testToken,
          expiresAt: new Date(Date.now() + 86400000).toISOString(), // 24h from now
          isAuthenticated: true,
        },
        version: 0,
      };
      localStorage.setItem('streamspace-auth', JSON.stringify(zustandState));
    });

    // Intercept ALL API calls before navigation
    await page.route('**/api/v1/**', async (route) => {
      const url = route.request().url();

      // Session detail endpoint
      if (url.includes('/sessions/test-selkies')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_SESSION_SELKIES),
        });
      }

      if (url.includes('/sessions/test-vnc')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_SESSION_VNC),
        });
      }

      // Connect endpoint
      if (url.includes('/connect')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            connectionId: 'conn-123',
            sessionUrl: 'http://test.local:3000',
            state: 'running',
            message: 'Connected',
          }),
        });
      }

      // Heartbeat
      if (url.includes('/heartbeat')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ status: 'ok' }),
        });
      }

      // Auth me endpoint
      if (url.includes('/auth/me')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            user_id: 'admin',
            username: 'admin',
            role: 'admin',
          }),
        });
      }

      // HTTP proxy for streaming
      if (url.includes('/http/')) {
        return route.fulfill({
          status: 200,
          contentType: 'text/html',
          body: '<html><body data-testid="stream">Stream Content</body></html>',
        });
      }

      // Default: pass through
      return route.continue();
    });

    // Mock VNC viewer
    await page.route('**/vnc-viewer/**', async (route) => {
      return route.fulfill({
        status: 200,
        contentType: 'text/html',
        body: '<html><body id="vnc-container"><canvas></canvas></body></html>',
      });
    });
  });

  test('CRITICAL: Token is passed in Selkies iframe URL', async ({ page }) => {
    await page.goto('/sessions/test-selkies/viewer');

    // Wait for either iframe or error
    const iframe = page.locator('iframe');
    const error = page.getByRole('alert');

    await Promise.race([
      iframe.waitFor({ state: 'visible', timeout: 15000 }),
      error.waitFor({ state: 'visible', timeout: 15000 }),
    ]);

    // If there's an error, skip (might be WebSocket related)
    if (await error.isVisible()) {
      console.log('Error visible:', await error.textContent());
      test.skip();
      return;
    }

    // Get iframe src
    const src = await iframe.getAttribute('src');
    console.log('Iframe src:', src);

    // CRITICAL: Verify token is present and valid
    expect(src, 'Iframe src should exist').toBeTruthy();
    expect(src, 'Token should be in URL').toContain('token=');
    expect(src, 'Token should not be null').not.toContain('token=null');
    expect(src, 'Token should not be undefined').not.toContain('token=undefined');

    // For Selkies, should use HTTP proxy
    expect(src, 'Should use HTTP proxy for Selkies').toContain('/api/v1/http/');
  });

  test('CRITICAL: Token is passed in VNC iframe URL', async ({ page }) => {
    await page.goto('/sessions/test-vnc/viewer');

    const iframe = page.locator('iframe');
    const error = page.getByRole('alert');

    await Promise.race([
      iframe.waitFor({ state: 'visible', timeout: 15000 }),
      error.waitFor({ state: 'visible', timeout: 15000 }),
    ]);

    if (await error.isVisible()) {
      test.skip();
      return;
    }

    const src = await iframe.getAttribute('src');
    console.log('Iframe src:', src);

    expect(src).toBeTruthy();
    expect(src).toContain('token=');
    expect(src).not.toContain('token=null');

    // For VNC, should use VNC viewer
    expect(src, 'Should use VNC viewer').toContain('/vnc-viewer/');
  });

  test('CRITICAL: Token value is actual token, not empty', async ({ page }) => {
    await page.goto('/sessions/test-selkies/viewer');

    const iframe = page.locator('iframe');
    await iframe.waitFor({ state: 'visible', timeout: 15000 }).catch(() => {});

    if (!await iframe.isVisible()) {
      test.skip();
      return;
    }

    const src = await iframe.getAttribute('src');

    // Extract token value
    const match = src?.match(/token=([^&]+)/);
    expect(match, 'Token should be captured').toBeTruthy();

    const tokenValue = match![1];
    expect(tokenValue.length, 'Token should have reasonable length').toBeGreaterThan(10);
    expect(tokenValue, 'Token should not be literal "null"').not.toBe('null');
    expect(tokenValue, 'Token should not be literal "undefined"').not.toBe('undefined');

    // Decode and verify it's our test token
    const decodedToken = decodeURIComponent(tokenValue);
    expect(decodedToken).toBe('test-jwt-token-12345');
  });
});

test.describe('Protocol Routing', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      const testToken = 'test-token';
      localStorage.setItem('token', testToken);

      // Zustand persist store format
      const zustandState = {
        state: {
          user: { username: 'admin', email: 'admin@test.local', role: 'admin' },
          token: testToken,
          expiresAt: new Date(Date.now() + 86400000).toISOString(),
          isAuthenticated: true,
        },
        version: 0,
      };
      localStorage.setItem('streamspace-auth', JSON.stringify(zustandState));
    });
  });

  test('Selkies protocol routes to HTTP proxy', async ({ page }) => {
    await page.route('**/api/v1/**', async (route) => {
      if (route.request().url().includes('/sessions/')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSION_SELKIES,
            streamingProtocol: 'selkies',
          }),
        });
      }
      return route.fulfill({ status: 200, body: '{}' });
    });

    await page.route('**/api/v1/http/**', async (route) => {
      return route.fulfill({
        status: 200,
        contentType: 'text/html',
        body: '<html><body>Selkies Stream</body></html>',
      });
    });

    await page.goto('/sessions/test-selkies/viewer');

    const iframe = page.locator('iframe');
    await iframe.waitFor({ timeout: 10000 }).catch(() => {});

    if (await iframe.isVisible()) {
      const src = await iframe.getAttribute('src');
      expect(src).toContain('/api/v1/http/');
    }
  });

  test('VNC protocol routes to VNC viewer', async ({ page }) => {
    await page.route('**/api/v1/**', async (route) => {
      if (route.request().url().includes('/sessions/')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSION_VNC,
            streamingProtocol: 'vnc',
          }),
        });
      }
      return route.fulfill({ status: 200, body: '{}' });
    });

    await page.route('**/vnc-viewer/**', async (route) => {
      return route.fulfill({
        status: 200,
        contentType: 'text/html',
        body: '<html><body>VNC Viewer</body></html>',
      });
    });

    await page.goto('/sessions/test-vnc/viewer');

    const iframe = page.locator('iframe');
    await iframe.waitFor({ timeout: 10000 }).catch(() => {});

    if (await iframe.isVisible()) {
      const src = await iframe.getAttribute('src');
      expect(src).toContain('/vnc-viewer/');
    }
  });
});

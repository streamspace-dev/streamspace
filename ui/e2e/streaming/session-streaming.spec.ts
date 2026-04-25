/**
 * Session Streaming Tests
 *
 * Comprehensive tests for session streaming functionality including:
 * - VNC protocol streaming
 * - Selkies/HTTP protocol streaming
 * - Token authentication for iframe
 * - Stream rendering and controls
 *
 * These tests are critical for diagnosing and preventing black screen issues.
 */

import { test, expect, Page } from '@playwright/test';
import { SessionViewerPage } from '../pages/session-viewer.page';
import { APIMocker, MOCK_SESSIONS } from '../fixtures/api.fixture';

/**
 * Helper to set up authenticated page with token in localStorage
 */
async function setupAuthenticatedPage(page: Page, token: string = 'test-jwt-token') {
  await page.addInitScript((tokenValue) => {
    localStorage.setItem('token', tokenValue);
    localStorage.setItem('user', JSON.stringify({ username: 'admin', role: 'admin' }));
  }, token);
}

test.describe('Session Streaming', () => {
  test.describe('Token Authentication', () => {
    test('should include token in iframe src URL for Selkies sessions', async ({ page }) => {
      const testToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test';
      await setupAuthenticatedPage(page, testToken);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();

      // Override session to use Selkies protocol
      await page.route('**/api/v1/sessions/test-selkies-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'test-selkies-session',
            streamingProtocol: 'selkies',
          }),
        });
      });

      await apiMocker.mockHTTPProxy();

      const viewer = new SessionViewerPage(page);
      await viewer.goto('test-selkies-session');
      await viewer.waitForLoad();

      // Verify token is in URL
      const iframeSrc = await viewer.getIframeSrc();
      expect(iframeSrc).not.toBeNull();
      expect(iframeSrc).toContain('token=');
      expect(iframeSrc).toContain(encodeURIComponent(testToken));
    });

    test('should include token in iframe src URL for VNC sessions', async ({ page }) => {
      const testToken = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.vnc-test';
      await setupAuthenticatedPage(page, testToken);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();

      // Override session to use VNC protocol
      await page.route('**/api/v1/sessions/test-vnc-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.vnc,
            name: 'test-vnc-session',
            streamingProtocol: 'vnc',
          }),
        });
      });

      // Mock VNC viewer page
      await page.route('**/vnc-viewer/**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'text/html',
          body: '<html><body id="vnc-container"><canvas></canvas></body></html>',
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('test-vnc-session');
      await viewer.waitForLoad();

      // Verify token is in URL and iframe uses VNC viewer
      const iframeSrc = await viewer.getIframeSrc();
      expect(iframeSrc).not.toBeNull();
      expect(iframeSrc).toContain('/vnc-viewer/');
      expect(iframeSrc).toContain('token=');
    });

    test('should NOT have empty or null token in iframe URL', async ({ page }) => {
      // This test specifically catches the bug where token was read from wrong storage
      await setupAuthenticatedPage(page, 'valid-token-12345');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/test-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'test-session',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('test-session');
      await viewer.waitForLoad();

      const iframeSrc = await viewer.getIframeSrc();

      // Critical assertions - these catch the token bug
      expect(iframeSrc).toContain('token=');
      expect(iframeSrc).not.toContain('token=null');
      expect(iframeSrc).not.toContain('token=undefined');
      expect(iframeSrc).not.toContain('token=&');
      expect(iframeSrc).not.toMatch(/token=$/);

      // Verify actual token value is present
      const tokenMatch = iframeSrc?.match(/token=([^&]+)/);
      expect(tokenMatch).not.toBeNull();
      expect(tokenMatch![1].length).toBeGreaterThan(10);
    });

    test('should redirect to login when no token is available', async ({ page }) => {
      // Don't set up authentication
      await page.goto('/sessions/test-session/view');

      // Should redirect to login
      await expect(page).toHaveURL(/\/login/);
    });
  });

  test.describe('Protocol Routing', () => {
    test('should route to HTTP proxy for Selkies protocol', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/selkies-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'selkies-session',
            streamingProtocol: 'selkies',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('selkies-session');
      await viewer.waitForLoad();

      await viewer.expectProtocol('http');
    });

    test('should route to HTTP proxy for Kasm protocol', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/kasm-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'kasm-session',
            streamingProtocol: 'kasm',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('kasm-session');
      await viewer.waitForLoad();

      await viewer.expectProtocol('http');
    });

    test('should route to HTTP proxy for Guacamole protocol', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/guac-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'guac-session',
            streamingProtocol: 'guacamole',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('guac-session');
      await viewer.waitForLoad();

      await viewer.expectProtocol('http');
    });

    test('should route to VNC viewer for VNC protocol', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();

      await page.route('**/api/v1/sessions/vnc-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.vnc,
            name: 'vnc-session',
            streamingProtocol: 'vnc',
          }),
        });
      });

      await page.route('**/vnc-viewer/**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'text/html',
          body: '<html><body id="vnc-container"><canvas></canvas></body></html>',
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('vnc-session');
      await viewer.waitForLoad();

      await viewer.expectProtocol('vnc');
    });

    test('should default to VNC for sessions without protocol specified', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();

      await page.route('**/api/v1/sessions/default-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'default-session',
            streamingProtocol: undefined, // No protocol specified
          }),
        });
      });

      await page.route('**/vnc-viewer/**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'text/html',
          body: '<html><body id="vnc-container"><canvas></canvas></body></html>',
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('default-session');
      await viewer.waitForLoad();

      // Should default to VNC
      await viewer.expectProtocol('vnc');
    });
  });

  test.describe('Viewer Controls', () => {
    test('should display session information in toolbar', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/info-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'info-session',
            template: 'chromium',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('info-session');
      await viewer.waitForLoad();

      // Verify toolbar elements
      await expect(viewer.toolbar).toBeVisible();
      await expect(viewer.sessionTitle).toContainText('chromium');
      await expect(viewer.closeButton).toBeVisible();
      await expect(viewer.refreshButton).toBeVisible();
      await expect(viewer.fullscreenButton).toBeVisible();
    });

    test('should refresh iframe when refresh button clicked', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/refresh-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'refresh-session',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('refresh-session');
      await viewer.waitForLoad();

      // Get initial iframe src (verify it exists before refresh)
      await viewer.getIframeSrc();

      // Click refresh
      await viewer.refresh();

      // Iframe should still be visible (refreshed)
      await viewer.expectStreamingVisible();
    });

    test('should navigate back to sessions when close button clicked', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/close-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'close-session',
          }),
        });
      });

      await page.route('**/api/v1/sessions/close-session/disconnect**', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ status: 'ok' }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('close-session');
      await viewer.waitForLoad();

      // Click close
      await viewer.close();

      // Should navigate to sessions page
      await expect(page).toHaveURL(/\/sessions/);
    });

    test('should show session info dialog with correct details', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      const sessionDetails = {
        name: 'detailed-session',
        template: 'chromium',
        platform: 'kubernetes',
        agent_id: 'k8s-agent-1',
        state: 'running',
      };

      await page.route('**/api/v1/sessions/detailed-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            ...sessionDetails,
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('detailed-session');
      await viewer.waitForLoad();

      await viewer.expectSessionInfo({
        template: 'chromium',
        platform: 'kubernetes',
        agentId: 'k8s-agent-1',
      });
    });
  });

  test.describe('Error Handling', () => {
    test('should show error when session is not running', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      await page.route('**/api/v1/sessions/hibernated-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.hibernated,
            name: 'hibernated-session',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('hibernated-session');
      await viewer.waitForLoad();

      await viewer.expectError('not running');
    });

    test('should show error when session URL is not available', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();

      await page.route('**/api/v1/sessions/no-url-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'no-url-session',
            status: {
              phase: 'Running',
              url: null, // No URL available
            },
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('no-url-session');
      await viewer.waitForLoad();

      await viewer.expectError('URL not available');
    });

    test('should show error when session not found', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      await page.route('**/api/v1/sessions/nonexistent-session', async (route) => {
        await route.fulfill({
          status: 404,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Session not found' }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('nonexistent-session');
      await viewer.waitForLoad();

      await viewer.expectError();
    });

    test('should show error when connect fails', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      await page.route('**/api/v1/sessions/connect-fail', async (route) => {
        if (route.request().url().includes('/connect')) {
          await route.fulfill({
            status: 503,
            contentType: 'application/json',
            body: JSON.stringify({ error: 'Agent not connected' }),
          });
        } else {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(MOCK_SESSIONS.running),
          });
        }
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('connect-fail');
      await viewer.waitForLoad();

      await viewer.expectError();
    });
  });

  test.describe('Iframe Loading', () => {
    test('should display streaming iframe after load', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/iframe-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'iframe-session',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('iframe-session');
      await viewer.waitForLoad();

      await viewer.expectStreamingVisible();
    });

    test('should have correct iframe attributes', async ({ page }) => {
      await setupAuthenticatedPage(page, 'test-token');

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();
      await apiMocker.mockSessionConnect();
      await apiMocker.mockHeartbeat();
      await apiMocker.mockHTTPProxy();

      await page.route('**/api/v1/sessions/attrs-session', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...MOCK_SESSIONS.running,
            name: 'attrs-session',
          }),
        });
      });

      const viewer = new SessionViewerPage(page);
      await viewer.goto('attrs-session');
      await viewer.waitForLoad();

      // Verify iframe has proper attributes for streaming
      const iframe = viewer.streamingIframe;
      await expect(iframe).toHaveAttribute('title', /Session/);
      await expect(iframe).toHaveAttribute('allow', /clipboard/);
    });
  });
});

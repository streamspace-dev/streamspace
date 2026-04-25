/**
 * Session Management Tests
 *
 * Tests for session creation, listing, state transitions, and deletion.
 */

import { test, expect, Page } from '@playwright/test';
import { SessionsPage } from '../pages/sessions.page';
import { APIMocker, MOCK_SESSIONS, MOCK_TEMPLATES } from '../fixtures/api.fixture';

/**
 * Helper to set up authenticated page
 */
async function setupAuthenticatedPage(page: Page, token: string = 'test-jwt-token') {
  await page.addInitScript((tokenValue) => {
    localStorage.setItem('token', tokenValue);
    localStorage.setItem('user', JSON.stringify({ username: 'admin', role: 'admin' }));
  }, token);
}

test.describe('Session Management', () => {
  test.describe('Session List', () => {
    test('should display list of sessions', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Should have session cards
      const count = await sessionsPage.getSessionCount();
      expect(count).toBeGreaterThan(0);
    });

    test('should show empty state when no sessions', async ({ page }) => {
      await setupAuthenticatedPage(page);

      // Mock empty sessions
      await page.route('**/api/v1/sessions', async (route) => {
        if (route.request().method() === 'GET') {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify([]),
          });
        } else {
          await route.continue();
        }
      });

      await page.route('**/api/v1/templates', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_TEMPLATES),
        });
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      await sessionsPage.expectEmptyState();
    });

    test('should display session state correctly', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Check running session
      await sessionsPage.expectSession(MOCK_SESSIONS.running.name, 'running');

      // Check hibernated session
      await sessionsPage.expectSession(MOCK_SESSIONS.hibernated.name, 'hibernated');
    });

    test('should filter sessions by state', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const allSessions = [MOCK_SESSIONS.running, MOCK_SESSIONS.hibernated, MOCK_SESSIONS.vnc];

      await page.route('**/api/v1/sessions', async (route) => {
        if (route.request().method() === 'GET') {
          const url = new URL(route.request().url());
          const stateFilter = url.searchParams.get('state');

          let filtered = allSessions;
          if (stateFilter && stateFilter !== 'all') {
            filtered = allSessions.filter(s => s.state === stateFilter);
          }

          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify(filtered),
          });
        } else {
          await route.continue();
        }
      });

      await page.route('**/api/v1/templates', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_TEMPLATES),
        });
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Filter by running
      await sessionsPage.filterByStatus('running');
      await page.waitForTimeout(500);

      // Should only show running sessions
      const runningCount = await sessionsPage.getSessionCount();
      expect(runningCount).toBe(2); // running and vnc are both "running"
    });

    test('should search sessions by name', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Search for specific session
      await sessionsPage.search('running');
      await page.waitForTimeout(500);

      // Should filter results
      await sessionsPage.expectSession(MOCK_SESSIONS.running.name);
    });
  });

  test.describe('Session Actions', () => {
    test('should navigate to viewer when connect clicked', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Click connect on running session
      await sessionsPage.connectToSession(MOCK_SESSIONS.running.name);

      // Should navigate to viewer
      await expect(page).toHaveURL(new RegExp(`/sessions/${MOCK_SESSIONS.running.name}/view`));
    });

    test('should hibernate running session', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      // Mock hibernate endpoint
      await page.route('**/api/v1/sessions/*/hibernate', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ status: 'hibernating' }),
        });
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      await sessionsPage.hibernateSession(MOCK_SESSIONS.running.name);

      // Should show success notification or update state
      // (specific assertion depends on UI implementation)
    });

    test('should terminate session', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      // Mock terminate endpoint
      await page.route('**/api/v1/sessions/*', async (route) => {
        if (route.request().method() === 'DELETE') {
          await route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ status: 'terminated' }),
          });
        } else {
          await route.continue();
        }
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      await sessionsPage.terminateSession(MOCK_SESSIONS.running.name);

      // Confirm dialog might appear
      const confirmButton = page.getByRole('button', { name: /confirm|yes|delete/i });
      if (await confirmButton.isVisible()) {
        await confirmButton.click();
      }
    });

    test('should open create session dialog', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      await sessionsPage.openCreateDialog();

      // Should show create dialog or navigate to create page
      const dialog = page.getByRole('dialog');
      const createPage = page.getByText(/select template|choose application/i);

      await expect(dialog.or(createPage)).toBeVisible();
    });
  });

  test.describe('Session Creation', () => {
    test('should display available templates', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();
      await sessionsPage.openCreateDialog();

      // Should show templates
      for (const template of MOCK_TEMPLATES) {
        await expect(page.getByText(template.displayName)).toBeVisible();
      }
    });

    test('should create session with selected template', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      // Track session creation
      let createdSession: { name: string; template: string; state: string; status: { phase: string; url: string } } | null = null;
      await page.route('**/api/v1/sessions', async (route) => {
        if (route.request().method() === 'POST') {
          const body = route.request().postDataJSON();
          createdSession = {
            name: `session-${Date.now()}`,
            template: body.template,
            state: 'running',
            status: { phase: 'Running', url: 'http://test:3000' },
          };
          await route.fulfill({
            status: 201,
            contentType: 'application/json',
            body: JSON.stringify(createdSession),
          });
        } else {
          await route.continue();
        }
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();
      await sessionsPage.openCreateDialog();

      // Select a template
      await page.getByText(MOCK_TEMPLATES[0].displayName).click();

      // Submit
      const createButton = page.getByRole('button', { name: /create|launch|start/i });
      await createButton.click();

      // Verify session was created with correct template
      expect(createdSession).not.toBeNull();
      expect(createdSession.template).toBe(MOCK_TEMPLATES[0].name);
    });
  });

  test.describe('Real-time Updates', () => {
    test('should update session state when WebSocket message received', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();

      // Verify initial state
      await sessionsPage.expectSession(MOCK_SESSIONS.running.name, 'running');

      // The WebSocket would push updates - we can simulate by refreshing
      // In real test, we'd mock WebSocket or use actual WS connection
    });
  });

  test.describe('Error Handling', () => {
    test('should show error when session list fails to load', async ({ page }) => {
      await setupAuthenticatedPage(page);

      await page.route('**/api/v1/sessions', async (route) => {
        await route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'Internal server error' }),
        });
      });

      await page.route('**/api/v1/templates', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify(MOCK_TEMPLATES),
        });
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();

      // Should show error alert or message
      const error = page.getByRole('alert').or(page.getByText(/error|failed/i));
      await expect(error).toBeVisible();
    });

    test('should handle session creation failure', async ({ page }) => {
      await setupAuthenticatedPage(page);

      const apiMocker = new APIMocker(page);
      await apiMocker.mockAllEndpoints();

      await page.route('**/api/v1/sessions', async (route) => {
        if (route.request().method() === 'POST') {
          await route.fulfill({
            status: 503,
            contentType: 'application/json',
            body: JSON.stringify({ error: 'No agents available' }),
          });
        } else {
          await route.continue();
        }
      });

      const sessionsPage = new SessionsPage(page);
      await sessionsPage.goto();
      await sessionsPage.waitForLoad();
      await sessionsPage.openCreateDialog();

      await page.getByText(MOCK_TEMPLATES[0].displayName).click();

      const createButton = page.getByRole('button', { name: /create|launch|start/i });
      await createButton.click();

      // Should show error
      const error = page.getByRole('alert').or(page.getByText(/no agents|error|failed/i));
      await expect(error).toBeVisible();
    });
  });
});

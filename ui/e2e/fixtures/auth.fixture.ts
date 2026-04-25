/**
 * Authentication Fixtures for Playwright E2E Tests
 *
 * Provides authenticated test contexts and helper functions
 * for testing StreamSpace UI with proper authentication.
 */

import { test as base, expect, Page, BrowserContext } from '@playwright/test';

/**
 * Test user credentials
 * These should match the seeded test users in the database
 */
export const TEST_USERS = {
  admin: {
    username: 'admin',
    email: 'admin@streamspace.local',
    password: 'admin123',
    role: 'admin',
  },
  user: {
    username: 'testuser',
    email: 'testuser@streamspace.local',
    password: 'testuser123',
    role: 'user',
  },
};

/**
 * API base URL - defaults to localhost:8000 for local development
 */
export const API_URL = process.env.API_URL || 'http://localhost:8000';

/**
 * Extended test fixtures with authentication support
 */
export interface AuthFixtures {
  /** Authenticated page as admin user */
  authenticatedPage: Page;
  /** Authenticated context with token set */
  authenticatedContext: BrowserContext;
  /** Helper to login programmatically via API */
  loginAsAdmin: () => Promise<string>;
  /** Helper to login as regular user */
  loginAsUser: () => Promise<string>;
  /** Helper to logout */
  logout: (page: Page) => Promise<void>;
  /** Current auth token */
  authToken: string;
}

/**
 * Login via API and return token
 */
async function loginViaAPI(username: string, password: string): Promise<string> {
  const response = await fetch(`${API_URL}/api/v1/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Login failed: ${response.status} - ${error}`);
  }

  const data = await response.json();
  return data.token;
}

/**
 * Extended test with authentication fixtures
 */
export const test = base.extend<AuthFixtures>({
  authToken: async ({}, use) => {
    const token = await loginViaAPI(TEST_USERS.admin.username, TEST_USERS.admin.password);
    await use(token);
  },

  authenticatedContext: async ({ browser, authToken }, use) => {
    const context = await browser.newContext({
      storageState: {
        cookies: [],
        origins: [
          {
            origin: 'http://localhost:5173',
            localStorage: [
              { name: 'token', value: authToken },
              { name: 'user', value: JSON.stringify({ username: TEST_USERS.admin.username, role: TEST_USERS.admin.role }) },
            ],
          },
        ],
      },
    });
    await use(context);
    await context.close();
  },

  authenticatedPage: async ({ authenticatedContext }, use) => {
    const page = await authenticatedContext.newPage();
    await use(page);
  },

  loginAsAdmin: async ({}, use) => {
    const login = async () => {
      return await loginViaAPI(TEST_USERS.admin.username, TEST_USERS.admin.password);
    };
    await use(login);
  },

  loginAsUser: async ({}, use) => {
    const login = async () => {
      return await loginViaAPI(TEST_USERS.user.username, TEST_USERS.user.password);
    };
    await use(login);
  },

  logout: async ({}, use) => {
    const logoutFn = async (page: Page) => {
      await page.evaluate(() => {
        localStorage.removeItem('token');
        localStorage.removeItem('user');
      });
      await page.goto('/login');
    };
    await use(logoutFn);
  },
});

export { expect };

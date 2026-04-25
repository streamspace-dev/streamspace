/**
 * Base Test Fixture with MSW Support
 *
 * Extends Playwright's base test to enable MSW mocking for all tests.
 * Import this instead of @playwright/test for mock-enabled tests.
 */

import { test as base, expect, Page } from '@playwright/test';

/**
 * Extended test fixtures with MSW support
 */
export interface MSWFixtures {
  /** Page with MSW enabled */
  mswPage: Page;
  /** Setup authentication in localStorage */
  setupAuth: (page: Page, token?: string) => Promise<void>;
}

/**
 * Enable MSW on the page by setting localStorage flag
 */
async function enableMSW(page: Page): Promise<void> {
  // Set localStorage flag to enable MSW before any navigation
  await page.addInitScript(() => {
    localStorage.setItem('msw-enabled', 'true');
  });
}

/**
 * Set up authentication in localStorage
 */
async function setupAuthentication(
  page: Page,
  token: string = 'mock-jwt-token'
): Promise<void> {
  await page.addInitScript((tokenValue) => {
    localStorage.setItem('token', tokenValue);
    localStorage.setItem('user', JSON.stringify({
      username: 'admin',
      email: 'admin@streamspace.local',
      role: 'admin',
    }));
    localStorage.setItem('msw-enabled', 'true');
  }, token);
}

/**
 * Extended test with MSW fixtures
 */
export const test = base.extend<MSWFixtures>({
  mswPage: async ({ page }, use) => {
    await enableMSW(page);
    await use(page);
  },

  setupAuth: async ({}, use) => {
    await use(setupAuthentication);
  },
});

export { expect };

/**
 * Helper to navigate with MSW enabled
 */
export async function gotoWithMSW(page: Page, path: string): Promise<void> {
  // Add msw=true to enable mocking
  const separator = path.includes('?') ? '&' : '?';
  await page.goto(`${path}${separator}msw=true`);
}

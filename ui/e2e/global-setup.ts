/**
 * Playwright Global Setup
 *
 * Runs once before all tests to set up the test environment.
 */

import { FullConfig } from '@playwright/test';

async function globalSetup(_config: FullConfig): Promise<void> {
  console.log('🚀 Playwright global setup running...');

  // Set environment variable to enable MSW in the app
  process.env.VITE_ENABLE_MOCKS = 'true';

  console.log('✅ Global setup complete');
}

export default globalSetup;

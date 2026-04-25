import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright Configuration for StreamSpace UI E2E Tests
 *
 * Run all tests: npm run test:e2e
 * Run specific test: npx playwright test streaming/
 * Run with UI: npx playwright test --ui
 * Run headed: npx playwright test --headed
 *
 * MSW (Mock Service Worker) is used to intercept API requests.
 * Tests navigate to /?msw=true to enable mocking.
 */
export default defineConfig({
    testDir: './e2e',
    fullyParallel: true,
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 2 : 0,
    workers: process.env.CI ? 1 : undefined,
    reporter: [
        ['html', { open: 'never' }],
        ['list'],
    ],
    use: {
        baseURL: 'http://localhost:3000',
        trace: 'on-first-retry',
        screenshot: 'only-on-failure',
        video: 'retain-on-failure',
        // Longer timeout for streaming tests
        actionTimeout: 10000,
        navigationTimeout: 30000,
    },
    // Global timeout for each test
    timeout: 60000,
    // Expect timeout
    expect: {
        timeout: 10000,
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
        {
            name: 'firefox',
            use: { ...devices['Desktop Firefox'] },
        },
        {
            name: 'webkit',
            use: { ...devices['Desktop Safari'] },
        },
        // Mobile viewports for responsive testing
        {
            name: 'mobile-chrome',
            use: { ...devices['Pixel 5'] },
        },
    ],
    webServer: {
        command: 'npm run dev',
        url: 'http://localhost:3000',
        reuseExistingServer: true, // Always reuse if running
        timeout: 120000, // 2 minutes for dev server startup
    },
});

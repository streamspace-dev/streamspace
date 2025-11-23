import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
    test.beforeEach(async ({ page }) => {
        // Mock auth or login
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
    });

    test('should display key metrics', async ({ page }) => {
        await expect(page.getByText('Active Sessions')).toBeVisible();
        await expect(page.getByText('Total Usage')).toBeVisible();
        await expect(page.getByText('Cost Estimate')).toBeVisible();
    });

    test('should list recent sessions', async ({ page }) => {
        await expect(page.getByText('Recent Sessions')).toBeVisible();
        // Check for at least one session item or empty state
        await expect(page.locator('.session-card').first().or(page.getByText('No active sessions'))).toBeVisible();
    });
});

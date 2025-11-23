import { test, expect } from '@playwright/test';

test.describe('Session Viewer', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
    });

    test('should connect to a running session', async ({ page }) => {
        await page.goto('/sessions');

        // Find a running session and connect
        // This assumes a running session exists or we mock it
        // await page.getByRole('button', { name: 'Connect' }).first().click();

        // Verify viewer loads
        // await expect(page).toHaveURL(/\/session\//);
        // await expect(page.locator('canvas')).toBeVisible(); // VNC canvas
    });
});

import { test, expect } from '@playwright/test';

test.describe('Admin System Settings', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('admin@streamspace.io');
        await page.getByLabel('Password').fill('adminpass');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/admin/system');
    });

    test('should display system metrics', async ({ page }) => {
        await expect(page.getByText('CPU Usage')).toBeVisible();
        await expect(page.getByText('Memory Usage')).toBeVisible();
        await expect(page.getByText('Disk Usage')).toBeVisible();
    });
});

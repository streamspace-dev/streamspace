import { test, expect } from '@playwright/test';

test.describe('Admin User Management', () => {
    test.beforeEach(async ({ page }) => {
        // Login as admin
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('admin@streamspace.io');
        await page.getByLabel('Password').fill('adminpass');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/admin/users');
    });

    test('should list users', async ({ page }) => {
        await expect(page.getByText('User Management')).toBeVisible();
        await expect(page.locator('table tbody tr')).not.toHaveCount(0);
    });

    test('should filter users', async ({ page }) => {
        await page.getByPlaceholder('Search users...').fill('test@streamspace.io');
        await expect(page.locator('table tbody tr')).toHaveCount(1);
    });
});

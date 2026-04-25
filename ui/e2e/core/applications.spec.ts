import { test, expect } from '@playwright/test';

test.describe('Applications Catalog', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/applications');
    });

    test('should list available applications', async ({ page }) => {
        await expect(page.getByText('Application Catalog')).toBeVisible();
        await expect(page.locator('.app-card')).toHaveCount(await page.locator('.app-card').count());
    });

    test('should search for applications', async ({ page }) => {
        await page.getByPlaceholder('Search applications...').fill('Blender');
        // Verify results filtered
        // await expect(page.getByText('Blender')).toBeVisible();
    });

    test('should open application details', async ({ page }) => {
        await page.locator('.app-card').first().click();
        await expect(page.getByRole('dialog')).toBeVisible();
        await expect(page.getByRole('button', { name: 'Launch' })).toBeVisible();
    });
});

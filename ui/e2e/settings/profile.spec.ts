import { test, expect } from '@playwright/test';

test.describe('User Profile Settings', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/settings/profile');
    });

    test('should update display name', async ({ page }) => {
        await page.getByLabel('Display Name').fill('Updated Name');
        await page.getByRole('button', { name: 'Save Changes' }).click();

        await expect(page.getByText('Profile updated successfully')).toBeVisible();
    });

    test('should upload avatar', async ({ page }) => {
        // Mock file upload
        // const fileChooserPromise = page.waitForEvent('filechooser');
        // await page.getByRole('button', { name: 'Upload Avatar' }).click();
        // const fileChooser = await fileChooserPromise;
        // await fileChooser.setFiles('path/to/avatar.png');

        // await expect(page.getByText('Avatar updated')).toBeVisible();
    });
});

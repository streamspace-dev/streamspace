import { test, expect } from '@playwright/test';

test.describe('Security Settings', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/settings/security');
    });

    test('should change password', async ({ page }) => {
        await page.getByLabel('Current Password').fill('password123');
        await page.getByLabel('New Password').fill('NewSecurePass1!');
        await page.getByLabel('Confirm New Password').fill('NewSecurePass1!');

        await page.getByRole('button', { name: 'Update Password' }).click();

        await expect(page.getByText('Password updated successfully')).toBeVisible();
    });

    test('should toggle 2FA', async ({ page }) => {
        // Check current state and toggle
        // const toggle = page.getByRole('switch', { name: 'Two-factor authentication' });
        // await toggle.click();
        // await expect(page.getByText('2FA updated')).toBeVisible();
    });
});

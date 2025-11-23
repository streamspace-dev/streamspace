import { test, expect } from '@playwright/test';

test.describe('Collaboration Flow', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
    });

    test('should share a session with another user', async ({ page }) => {
        await page.goto('/sessions');

        // Open share dialog
        const sessionCard = page.locator('.session-card').first();
        await sessionCard.getByRole('button', { name: 'Share' }).click();

        // Invite user
        await page.getByPlaceholder('Enter email address').fill('collab@streamspace.io');
        await page.getByRole('button', { name: 'Send Invite' }).click();

        await expect(page.getByText('Invitation sent')).toBeVisible();
    });
});

import { test, expect } from '@playwright/test';

test.describe('Session Management', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await page.goto('/sessions');
    });

    test('should create a new session', async ({ page }) => {
        await page.getByRole('button', { name: 'New Session' }).click();

        // Select template
        await page.getByText('Ubuntu Desktop').click();
        await page.getByRole('button', { name: 'Next' }).click();

        // Configure
        await page.getByLabel('Session Name').fill('Test Session');
        await page.getByRole('button', { name: 'Launch' }).click();

        // Verify creation
        await expect(page.getByText('Provisioning')).toBeVisible();
        await expect(page.getByText('Test Session')).toBeVisible();
    });

    test('should terminate a session', async ({ page }) => {
        // Assuming a session exists
        const sessionCard = page.locator('.session-card').first();
        await sessionCard.getByRole('button', { name: 'More actions' }).click();
        await page.getByRole('menuitem', { name: 'Terminate' }).click();

        // Confirm
        await page.getByRole('button', { name: 'Confirm' }).click();

        await expect(page.getByText('Session terminated')).toBeVisible();
    });
});

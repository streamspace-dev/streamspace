import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
    test('should login successfully with valid credentials', async ({ page }) => {
        await page.goto('/login');

        // Fill in credentials
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');

        // Click login
        await page.getByRole('button', { name: 'Sign In' }).click();

        // Verify redirect to dashboard
        await expect(page).toHaveURL('/dashboard');
        await expect(page.getByText('Welcome back')).toBeVisible();
    });

    test('should show error with invalid credentials', async ({ page }) => {
        await page.goto('/login');

        await page.getByLabel('Email Address').fill('wrong@streamspace.io');
        await page.getByLabel('Password').fill('wrongpass');
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page.getByText('Invalid credentials')).toBeVisible();
    });

    test('should logout successfully', async ({ page }) => {
        // Setup: Login first
        await page.goto('/login');
        await page.getByLabel('Email Address').fill('test@streamspace.io');
        await page.getByLabel('Password').fill('password123');
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL('/dashboard');

        // Perform logout
        await page.getByRole('button', { name: 'User menu' }).click();
        await page.getByRole('menuitem', { name: 'Logout' }).click();

        // Verify redirect to login
        await expect(page).toHaveURL('/login');
    });
});

import { test, expect } from '@playwright/test';

test.describe('New User Onboarding Flow', () => {
    test('should guide new user through setup', async ({ page }) => {
        // 1. Register
        await page.goto('/register');
        await page.getByLabel('Full Name').fill('Flow User');
        await page.getByLabel('Email Address').fill('flow@streamspace.io');
        await page.getByLabel('Password').fill('SecurePass123!');
        await page.getByLabel('Confirm Password').fill('SecurePass123!');
        await page.getByRole('button', { name: 'Create Account' }).click();

        // 2. Onboarding Wizard
        await expect(page).toHaveURL('/onboarding');
        await expect(page.getByText('Welcome to StreamSpace')).toBeVisible();
        await page.getByRole('button', { name: 'Get Started' }).click();

        // 3. Select Preferences
        // await page.getByText('Developer').click();
        // await page.getByRole('button', { name: 'Next' }).click();

        // 4. Complete
        // await page.getByRole('button', { name: 'Finish' }).click();
        // await expect(page).toHaveURL('/dashboard');
    });
});

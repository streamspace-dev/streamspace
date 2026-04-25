import { test, expect } from '@playwright/test';

test.describe('Registration', () => {
    test('should register a new user successfully', async ({ page }) => {
        await page.goto('/register');

        await page.getByLabel('Full Name').fill('New User');
        await page.getByLabel('Email Address').fill('newuser@streamspace.io');
        await page.getByLabel('Password').fill('SecurePass123!');
        await page.getByLabel('Confirm Password').fill('SecurePass123!');

        await page.getByRole('button', { name: 'Create Account' }).click();

        // Expect redirect to dashboard or onboarding
        await expect(page).toHaveURL(/\/dashboard|onboarding/);
    });

    test('should validate password matching', async ({ page }) => {
        await page.goto('/register');

        await page.getByLabel('Password').fill('Password123');
        await page.getByLabel('Confirm Password').fill('DifferentPass123');
        await page.getByRole('button', { name: 'Create Account' }).click();

        await expect(page.getByText('Passwords do not match')).toBeVisible();
    });
});

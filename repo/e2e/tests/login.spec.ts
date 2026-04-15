import { test, expect } from '@playwright/test';
import { webLogin } from './helpers';

test.describe('Login Flow', () => {
  test('shows login page with form fields', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('h2')).toContainText('Sign In');
    await expect(page.locator('input#username')).toBeVisible();
    await expect(page.locator('input#password')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
    await page.screenshot({ path: './screenshots/01-login-page.png', fullPage: true });
  });

  test('rejects invalid credentials with error message', async ({ page }) => {
    await page.goto('/login');
    await page.fill('#username', 'nonexistent');
    await page.fill('#password', 'WrongPass123!');
    await page.click('button[type="submit"]');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('.alert-error')).toBeVisible();
    await page.screenshot({ path: './screenshots/02-login-error.png', fullPage: true });
  });

  test('authenticated member sees their name in nav', async ({ page }) => {
    await webLogin(page, 'member1', 'Seed@Pass1234');
    // After login redirect, should see user name in nav
    await expect(page.locator('.nav-user')).toContainText('Member One');
    await expect(page.locator('.nav-links')).not.toContainText('Admin');
    await page.screenshot({ path: './screenshots/03-login-success-member.png', fullPage: true });
  });

  test('authenticated admin sees Admin link in nav', async ({ page }) => {
    await webLogin(page, 'admin', 'Seed@Pass1234');
    await expect(page.locator('.nav-links')).toContainText('Admin');
    await page.screenshot({ path: './screenshots/04-login-success-admin.png', fullPage: true });
  });

  test('logout clears session', async ({ page }) => {
    await webLogin(page, 'member1', 'Seed@Pass1234');
    await expect(page.locator('.nav-user')).toContainText('Member One');

    await page.locator('button:has-text("Logout")').click();
    await page.waitForLoadState('networkidle');
    await expect(page.locator('body')).not.toContainText('Member One');
    await page.screenshot({ path: './screenshots/05-logout.png', fullPage: true });
  });
});

import { test, expect } from '@playwright/test';
import { webLogin } from './helpers';

test.describe('Admin Flows', () => {
  test('shows admin dashboard with navigation cards', async ({ page }) => {
    await webLogin(page, 'admin', 'Seed@Pass1234');
    await page.goto('/admin');
    await expect(page.locator('h1')).toContainText('Administration');
    await expect(page.locator('body')).toContainText('Configuration');
    await expect(page.locator('body')).toContainText('Feature Flags');
    await expect(page.locator('body')).toContainText('Audit Logs');
    await page.screenshot({ path: './screenshots/16-admin-dashboard.png', fullPage: true });
  });

  test('shows system configuration table', async ({ page }) => {
    await webLogin(page, 'admin', 'Seed@Pass1234');
    await page.goto('/admin/config');
    await expect(page.locator('h1')).toContainText('System Configuration');
    await expect(page.locator('body')).toContainText('facility.timezone');
    await expect(page.locator('.data-table')).toBeVisible();
    await page.screenshot({ path: './screenshots/17-admin-config.png', fullPage: true });
  });

  test('shows feature flags page', async ({ page }) => {
    await webLogin(page, 'admin', 'Seed@Pass1234');
    await page.goto('/admin/feature-flags');
    await expect(page.locator('h1')).toContainText('Feature Flags');
    await page.screenshot({ path: './screenshots/18-admin-feature-flags.png', fullPage: true });
  });

  test('shows audit logs', async ({ page }) => {
    await webLogin(page, 'admin', 'Seed@Pass1234');
    await page.goto('/admin/audit-logs');
    await expect(page.locator('h1')).toContainText('Audit Logs');
    await expect(page.locator('.data-table')).toBeVisible();
    await page.screenshot({ path: './screenshots/19-admin-audit-logs.png', fullPage: true });
  });

  test('member cannot access admin pages', async ({ page }) => {
    await webLogin(page, 'member1', 'Seed@Pass1234');
    await page.goto('/admin/config');
    const content = await page.content();
    expect(content).not.toContain('System Configuration');
    await page.screenshot({ path: './screenshots/20-member-admin-denied.png', fullPage: true });
  });
});

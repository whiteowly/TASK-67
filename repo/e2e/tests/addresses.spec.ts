import { test, expect } from '@playwright/test';
import { webLogin } from './helpers';

test.describe('Address Management Flow', () => {
  test('requires auth for address pages', async ({ page }) => {
    await page.goto('/my/addresses');
    const content = await page.content();
    expect(content).not.toContain('My Addresses');
    await page.screenshot({ path: './screenshots/12-addresses-unauth.png', fullPage: true });
  });

  test('shows addresses page after login', async ({ page }) => {
    await webLogin(page, 'member1', 'Seed@Pass1234');
    await page.goto('/my/addresses');
    await expect(page.locator('h1')).toContainText('My Addresses');
    await page.screenshot({ path: './screenshots/13-addresses-list.png', fullPage: true });
  });

  test('creates a new address via form', async ({ page }) => {
    await webLogin(page, 'member1', 'Seed@Pass1234');
    await page.goto('/my/addresses/new');
    await expect(page.locator('h2')).toContainText('New Address');

    await page.fill('#recipient_name', 'John Doe');
    await page.fill('#phone', '13800138000');
    await page.fill('#line1', '123 Test Street');
    await page.fill('#city', 'Beijing');
    await page.fill('#postal_code', '100000');

    await page.screenshot({ path: './screenshots/14-address-form-filled.png', fullPage: true });

    await page.click('button:has-text("Create Address")');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('body')).toContainText('John Doe');
    await expect(page.locator('body')).toContainText('123 Test Street');
    await page.screenshot({ path: './screenshots/15-address-created.png', fullPage: true });
  });
});

import { test, expect } from '@playwright/test';

test.describe('Catalog Browsing Flow', () => {
  test('displays catalog page with sessions tab', async ({ page }) => {
    await page.goto('/catalog');
    await expect(page.locator('h1')).toContainText('Catalog');
    // Should have sessions and products tabs
    await expect(page.locator('.catalog-tabs')).toBeVisible();
    await expect(page.locator('.tab.active')).toContainText('Sessions');
    await page.screenshot({ path: './screenshots/06-catalog-sessions.png', fullPage: true });
  });

  test('shows session cards with availability badges', async ({ page }) => {
    await page.goto('/catalog?tab=sessions');
    // Should show seeded sessions
    const cards = page.locator('.card');
    await expect(cards.first()).toBeVisible();
    // Cards should have availability badges
    await expect(page.locator('.badge').first()).toBeVisible();
    await page.screenshot({ path: './screenshots/07-catalog-session-cards.png', fullPage: true });
  });

  test('switches to products tab', async ({ page }) => {
    await page.goto('/catalog?tab=products');
    await expect(page.locator('.tab.active')).toContainText('Products');
    // Should show seeded products
    const cards = page.locator('.card');
    await expect(cards.first()).toBeVisible();
    await page.screenshot({ path: './screenshots/08-catalog-products.png', fullPage: true });
  });

  test('search filters results', async ({ page }) => {
    await page.goto('/catalog?tab=sessions');
    await page.fill('input[name="q"]', 'yoga');
    await page.click('button:has-text("Search")');
    // Should show filtered results
    await page.waitForLoadState('networkidle');
    await page.screenshot({ path: './screenshots/09-catalog-search.png', fullPage: true });
  });

  test('navigates to session detail', async ({ page }) => {
    await page.goto('/catalog?tab=sessions');
    // Click first session link
    const firstLink = page.locator('.card-header a').first();
    await expect(firstLink).toBeVisible();
    const sessionTitle = await firstLink.textContent();
    await firstLink.click();
    // Should show detail page
    await expect(page.locator('h1')).toContainText(sessionTitle!);
    await expect(page.locator('.detail-badges')).toBeVisible();
    await expect(page.locator('.detail-info')).toBeVisible();
    await page.screenshot({ path: './screenshots/10-session-detail.png', fullPage: true });
  });

  test('navigates to product detail', async ({ page }) => {
    await page.goto('/catalog?tab=products');
    const firstLink = page.locator('.card-header a').first();
    await expect(firstLink).toBeVisible();
    await firstLink.click();
    await expect(page.locator('.detail-page')).toBeVisible();
    await page.screenshot({ path: './screenshots/11-product-detail.png', fullPage: true });
  });
});

import { Page } from '@playwright/test';

/**
 * Log in by submitting the web login form, then reload to pick up the session cookie.
 */
export async function webLogin(page: Page, username: string, password: string) {
  await page.goto('/login');
  await page.fill('#username', username);
  await page.fill('#password', password);

  await Promise.all([
    page.waitForNavigation({ waitUntil: 'load' }),
    page.click('button[type="submit"]'),
  ]);

  // The 303 redirect lands on / but the cookie may not have been sent
  // on the redirect GET. Reload to ensure the browser sends the stored cookie.
  await page.reload({ waitUntil: 'load' });
}

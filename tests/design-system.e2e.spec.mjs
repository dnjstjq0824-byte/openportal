import { test, expect } from '@playwright/test';
import { randomBytes } from 'node:crypto';
import { startStack } from './helpers/platform-stack.mjs';

// Design-system Slice A+B render coverage. The real gateway stack
// (embedded PostgreSQL + loopback SMTP + JMAP/ClamAV simulators) starts up
// front so the modal test owns a full timeout; the server-error test forces
// its own session failure by route abort and stays deterministic.
let stack;
test.beforeAll(async () => { test.setTimeout(120000); stack = await startStack({ pgPort: 55439 }); });
test.afterAll(async () => { if (stack) await stack.close(); });

test('onboarding banner shows once and dismissal persists', async ({ page }) => {
  await page.goto('/en');
  const banner = page.locator('.onboard-banner');
  await expect(banner.getByRole('heading', { name: 'Search, mail and account in one place' })).toBeVisible();
  await page.getByRole('button', { name: 'Start searching', exact: true }).click();
  await expect(banner).toBeHidden();
  expect(await page.evaluate(() => localStorage.getItem('openscanner.onboarded'))).toBe('1');
  await page.reload();
  await expect(banner).toHaveCount(0);
});

test('info pages render from the footer instead of 404', async ({ page }) => {
  await page.goto('/en');
  for (const [name, body] of [['Store', 'OpenScanner Store'], ['Advertising', 'ad separation'], ['Business', 'indexing APIs']]) {
    await page.locator('.site-footer').getByRole('link', { name, exact: true }).click();
    await expect(page.locator('.policy-page h1')).toHaveText(name);
    await expect(page.locator('.policy-page')).toContainText(body);
  }
  await page.goto('/ko/store');
  await expect(page.locator('.policy-page h1')).toHaveText('스토어');
  await page.goto('/en/definitely-missing');
  await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();
  await page.getByRole('link', { name: 'Back home' }).click();
  await expect(page).toHaveURL(/\/en$/);
});

test('login without a backend shows the server-error section with retry and home', async ({ page }) => {
  await page.route('**/api/v2/session', route => route.abort('failed'));
  await page.goto('/en/login');
  const section = page.locator('.auth-card .serp-state');
  await expect(section.getByRole('heading', { name: 'Account service is unavailable' })).toBeVisible();
  await section.getByRole('button', { name: 'Try again', exact: true }).click();
  await expect(section).toBeVisible();
  await section.getByRole('link', { name: 'Back home' }).click();
  await expect(page).toHaveURL(/\/en$/);
});

test('touched public routes do not overflow at required widths', async ({ page }) => {
  for (const width of [320, 375, 390, 768, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const route of ['/en', '/ko/store', '/en/login', '/th/business']) {
      await page.goto(route);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${route} at ${width}px`).toBe(true);
    }
  }
});

test('mail draft guard uses an in-app confirm dialog, never a native dialog', async ({ page }) => {
  page.on('dialog', () => { throw new Error('native dialog must not appear'); });
  await stack.pool.query('delete from webapp.rate_limits');
  const username = `design.${randomBytes(4).toString('hex')}`;
  const password = randomBytes(18).toString('base64url');
  const session = await page.request.get('/api/v2/session');
  const { csrf } = await session.json();
  const signup = await page.request.post('/api/v2/auth/signup', {
    headers: { Origin: 'http://127.0.0.1:5173', 'X-CSRF-Token': csrf },
    data: { givenName: 'Design', familyName: 'Tester', birthDate: '1990-01-01', gender: 'unspecified', username, password, locale: 'en', agree: true },
  });
  expect(signup.status()).toBe(201);
  await page.goto('/en/mail');
  await expect(page.getByRole('button', { name: 'Compose', exact: true })).toBeVisible();
  const dialog = page.getByRole('alertdialog');

  await page.getByRole('button', { name: 'Compose', exact: true }).click();
  await page.getByLabel('Subject', { exact: true }).fill('Keep my changes');
  await page.getByRole('button', { name: /^Sent/ }).click();
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('Discard the unsaved changes');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');

  await page.getByRole('button', { name: /^Sent/ }).click();
  await expect(dialog).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');

  await page.getByRole('button', { name: /^Sent/ }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Discard', exact: true }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByLabel('Subject', { exact: true })).toHaveCount(0);

  await page.getByRole('button', { name: 'Compose', exact: true }).click();
  await page.getByLabel('Subject', { exact: true }).fill('Leave me');
  await page.getByRole('link', { name: 'OpenScanner', exact: true }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Discard', exact: true }).click();
  await expect(page).toHaveURL(/\/en$/);
});

test('admin console gates by operator flag, then lists and searches accounts', async ({ page }) => {
  await stack.pool.query('delete from webapp.rate_limits');
  const username = `console.${randomBytes(4).toString('hex')}`;
  const password = randomBytes(18).toString('base64url');
  const session = await page.request.get('/api/v2/session');
  const { csrf } = await session.json();
  const signup = await page.request.post('/api/v2/auth/signup', {
    headers: { Origin: 'http://127.0.0.1:5173', 'X-CSRF-Token': csrf },
    data: { givenName: 'Admin', familyName: 'Candidate', birthDate: '1990-01-01', gender: 'unspecified', username, password, locale: 'en', agree: true },
  });
  expect(signup.status()).toBe(201);
  const email = `${username}@openscanner.test`;
  await page.goto('/en/admin');
  await expect(page.getByRole('heading', { name: 'Restricted area' })).toBeVisible();
  await stack.pool.query('update webapp.accounts set is_admin=true where email=$1', [email]);
  await page.reload();
  await expect(page.locator('.stat-grid')).toBeVisible();
  await expect(page.locator('.admin-table')).toContainText(email);
  await page.getByPlaceholder('Search email, username or name').fill('no-such-user-zzz');
  await page.locator('.admin-search').getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByText('No accounts match this search.')).toBeVisible();
});

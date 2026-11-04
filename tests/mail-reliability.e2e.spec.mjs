import { test, expect } from '@playwright/test';
import { randomBytes } from 'node:crypto';
import { mkdir } from 'node:fs/promises';
import { startStack } from './helpers/platform-stack.mjs';

// Real browser + PostgreSQL + loopback SMTP. Mailbox and antivirus providers
// are protocol simulators; no external mail delivery is claimed by these tests.
let stack;
test.beforeAll(async () => { test.setTimeout(60000); stack = await startStack({ pgPort: 55438 }); });
test.afterAll(async () => { if (stack) await stack.close(); });

async function account(page) {
  const email = `mail-ui-${randomBytes(6).toString('hex')}@example.test`;
  const password = randomBytes(18).toString('base64url');
  const session = await page.request.get('/api/v2/session');
  const { csrf } = await session.json();
  const headers = { Origin: 'http://127.0.0.1:5173', 'X-CSRF-Token': csrf };
  const signup = await page.request.post('/api/v2/auth/signup', { headers, data: { email, password, displayName: 'Mail tester', locale: 'en', agree: true } });
  expect(signup.status()).toBe(202);
  const verify = await page.request.post('/api/v2/auth/verify', { headers, data: { token: stack.smtp.tokenFor(email, 'verify') } });
  expect(verify.status()).toBe(200);
  const provider = await stack.provision(email);
  const login = await page.request.post('/api/v2/auth/login', { headers, data: { email, password } });
  expect(login.status()).toBe(200);
  return provider;
}

test('latest selected message wins and invalid provider dates do not crash the reader', async ({ page }, testInfo) => {
  const provider = await account(page);
  const first = stack.jmap.receive(provider.token, { subject: 'Slow first message' });
  const second = stack.jmap.receive(provider.token, { subject: 'Latest selected message' });
  stack.jmap.receive(provider.token, { subject: 'Message without a valid date', receivedAt: 'invalid-date' });
  const pageErrors = []; page.on('pageerror', error => pageErrors.push(error.message));
  let release, started, finished;
  const held = new Promise(resolve => { release = resolve; });
  const entered = new Promise(resolve => { started = resolve; });
  const completed = new Promise(resolve => { finished = resolve; });
  let firstMarked = 0;
  page.on('request', request => { if (request.method() === 'PATCH' && request.url().endsWith('/mail/messages/' + first)) firstMarked++; });
  await page.route('**/api/v2/mail/messages/' + first, async route => {
    if (route.request().method() !== 'GET') return route.continue();
    try { const response = await route.fetch(); started(); await held; await route.fulfill({ response }).catch(() => {}); }
    finally { finished(); }
  });
  try {
    await page.goto('/en/mail');
    await expect(page.getByText('Date unavailable', { exact: true })).toBeVisible();
    await page.locator('.mail-item').filter({ hasText: 'Slow first message' }).click();
    await entered;
    await page.locator('.mail-item').filter({ hasText: 'Latest selected message' }).click();
    await expect(page.locator('.mail-article h2')).toHaveText('Latest selected message');
    await expect(page.locator('.mail-reader')).toHaveAttribute('aria-busy', 'false');
    release(); await completed;
    await expect(page.locator('.mail-article h2')).toHaveText('Latest selected message');
    expect(firstMarked).toBe(0);
    await page.getByLabel('Language', { exact: true }).selectOption('ko');
    await expect(page.locator('.article-from strong')).toHaveText('보낸 사람:');
    expect(pageErrors).toEqual([]);
    const originalViewport=page.viewportSize();
    for(const width of [320,375,390,430,768,1024,1440,1920]){
      await page.setViewportSize({width,height:900});
      expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`mail overflow at ${width}px`).toBe(true);
    }
    await page.setViewportSize(originalViewport);
    await mkdir('docs/evidence/mail-reliability', { recursive: true });
    await page.screenshot({ path: `docs/evidence/mail-reliability/${testInfo.project.name}-reader.png`, fullPage: true });
    const backend = stack.jmap.accounts.get(provider.token);
    expect(backend.messages.get(second).keywords.$seen).toBe(true);
  } finally { release(); }
});

test('uncertain submission disables retries and reopening the same draft retains the warning', async ({ page }) => {
  const provider=await account(page);
  let drafts = 0, sends = 0;
  page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/mail/drafts')) drafts++; });
  // Browser transport fault injection, independent of real-provider acceptance.
  await page.route('**/api/v2/mail/send', async route => {
    sends++;
    const mailbox=stack.jmap.accounts.get(provider.token);
    const id=route.request().postDataJSON().id;
    // Persist the reservation as a gateway crash could, then inject the lost
    // provider response. Refresh must retain the block using server state.
    await stack.pool.query('insert into webapp.mail_send_requests(account_id,email_id) select id,$2 from webapp.accounts where email=$1',[mailbox.email,id]);
    return route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({ code: 'SEND_STATUS_UNKNOWN' }) });
  });
  await page.goto('/en/mail');
  await page.getByRole('button', { name: 'Compose', exact: true }).click();
  await page.getByLabel('To', { exact: true }).fill('recipient@example.test');
  await page.getByLabel('Subject', { exact: true }).fill('Submission status unknown');
  await page.getByLabel('Message', { exact: true }).fill('Preserve this draft until the operator reconciles submission.');
  await page.getByRole('button', { name: 'Send', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('status is uncertain');
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Save draft', exact: true })).toBeDisabled();
  await expect(page.getByLabel('Subject', { exact: true })).toBeDisabled();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await page.reload();
  await page.getByRole('button', { name: /^Drafts/ }).click();
  await page.locator('.mail-item').filter({ hasText: 'Submission status unknown' }).click();
  await expect(page.locator('.mail-reader')).toHaveAttribute('aria-busy', 'false');
  await page.locator('.mail-article').getByRole('button', { name: 'Compose', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Send', exact: true })).toBeDisabled();
  await expect(page.getByRole('alert')).toContainText('status is uncertain');
  expect(drafts).toBe(1); expect(sends).toBe(1);
});

test('unsaved draft changes require explicit discard and a saved draft reopens intact', async ({ page }) => {
  await account(page);
  await page.goto('/en/account');
  await expect(page.getByRole('heading', { name: 'Account', exact: true })).toBeVisible();
  await page.locator('.services-menu summary').click();
  await page.locator('.services-popover a[href="/en/mail"]').click();
  await expect(page).toHaveURL(/\/en\/mail$/);
  await page.locator('.services-menu summary').click();
  await page.getByRole('button', { name: 'Compose', exact: true }).click();
  await page.getByLabel('Subject', { exact: true }).fill('Keep my changes');
  page.once('dialog', dialog => dialog.dismiss());
  await page.getByRole('button', { name: /^Sent/ }).click();
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');
  page.once('dialog', dialog => dialog.dismiss());
  await page.getByRole('link', { name: 'Privacy', exact: true }).click();
  await expect(page).toHaveURL(/\/en\/mail$/);
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');
  page.once('dialog', dialog => dialog.dismiss());
  await page.goBack();
  await expect(page).toHaveURL(/\/en\/mail$/);
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');
  await page.getByLabel('Message', { exact: true }).fill('Draft content survives save and reopen.');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect(page.locator('.mail-reader')).toHaveAttribute('aria-busy', 'false');
  await expect(page.getByRole('status')).toHaveText('Saved.');
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await page.getByRole('button', { name: /^Drafts/ }).click();
  await page.locator('.mail-item').filter({ hasText: 'Keep my changes' }).click();
  await expect(page.locator('.mail-reader')).toHaveAttribute('aria-busy', 'false');
  await page.locator('.mail-article').getByRole('button', { name: 'Compose', exact: true }).click();
  await expect(page.getByLabel('Subject', { exact: true })).toHaveValue('Keep my changes');
  await expect(page.getByLabel('Message', { exact: true })).toHaveValue('Draft content survives save and reopen.');
});

test('guarded navigation preserves search URL state on refresh and browser history', async ({ page }) => {
  // Isolate browser routing from search data supply. This is not a real web
  // search integration test; no result or coverage claim is made here.
  await page.route('**/api/v2/search?**', route => route.fulfill({
    status: 200, contentType: 'application/json',
    body: JSON.stringify({ query: new URL(route.request().url()).searchParams.get('q'), total: 0, hits: [], has_more: false }),
  }));
  await page.goto('/ko');
  await page.getByRole('searchbox').fill('한글 ภาษาไทย & first');
  await page.getByRole('searchbox').press('Enter');
  await expect(page).toHaveURL(/\/ko\/search\?q=/);
  await page.reload();
  await expect(page.getByRole('searchbox')).toHaveValue('한글 ภาษาไทย & first');
  await page.getByRole('searchbox').fill('second ? search');
  await page.getByRole('searchbox').press('Enter');
  await expect(page.getByRole('searchbox')).toHaveValue('second ? search');
  await page.goBack();
  await expect(page.getByRole('searchbox')).toHaveValue('한글 ภาษาไทย & first');
  await page.goForward();
  await expect(page.getByRole('searchbox')).toHaveValue('second ? search');
  await page.getByLabel('언어', { exact: true }).selectOption('th');
  await expect(page).toHaveURL(/\/th\/search\?q=/);
  await expect(page.getByRole('searchbox')).toHaveValue('second ? search');
});

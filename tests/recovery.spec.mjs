import { test, expect } from '@playwright/test';

// Fault injection only; these tests do not establish real search/provider success.
test('HTML fallback is explained safely and retry recovers through the real API', async ({page}) => {
  await page.route('**/api/v1/search?**', route => route.fulfill({contentType:'text/html', body:'<!doctype html><title>fallback</title>'}));
  await page.goto('/search?q=OpenPortal&page=1');
  await expect(page.getByRole('alert')).toContainText('검색 서비스에 연결할 수 없습니다');
  await expect(page.getByRole('alert')).not.toContainText('Unexpected token');
  await page.unroute('**/api/v1/search?**');
  await page.getByRole('button', {name:'다시 시도'}).click();
  await expect(page.locator('.hit').first()).toBeVisible();
});
test('search URLs survive reload and browser back navigation', async ({page}) => {
  await page.goto('/');
  await page.getByRole('combobox', {name:'검색어'}).fill('OpenPortal');
  await page.getByRole('combobox', {name:'검색어'}).press('Enter');
  await expect(page).toHaveURL(/\/search\?q=OpenPortal/);
  await expect(page.locator('.hit').first()).toBeVisible();
  await page.reload();
  await expect(page.locator('.hit').first()).toBeVisible();
  await page.goBack();
  await expect(page.locator('.home-search-panel')).toBeVisible();
});
test('invalid result URLs and network failures do not expose stale results', async ({page}) => {
  await page.route('**/api/v1/search?**', route => route.fulfill({contentType:'application/json',body:JSON.stringify({query:'x',total:1,hits:[{title:'unsafe',url:'javascript:alert(1)',description:'x'}]})}));
  await page.goto('/search?q=x');
  await expect(page.getByRole('alert')).toContainText('검색 응답 형식');
  await expect(page.locator('.hit')).toHaveCount(0);
  await page.unroute('**/api/v1/search?**');
  await page.route('**/api/v1/search?**', route=>route.abort());
  await page.getByRole('button', {name:'다시 시도'}).click();
  await expect(page.getByRole('alert')).toContainText('네트워크');
});
test('production identity rejects fabricated challenge responses', async ({request}) => {
  const response = await request.post('http://127.0.0.1:8080/api/v1/auth/verify', {data:{challenge_response:'signed:attacker'}});
  expect(response.status()).toBe(404);
});

import { test, expect } from '@playwright/test';
import { readFileSync } from 'node:fs';
import YAML from 'yaml';
import Ajv from 'ajv/dist/2020.js';

const spec = YAML.parse(readFileSync('docs/openapi.yaml','utf8'));
const ajv = new Ajv({strict:false, formats:{
  uri: value => { try { return ['http:','https:'].includes(new URL(value).protocol); } catch { return false; } },
  int64: {type:'number',validate:Number.isSafeInteger},
}});
const validate = ajv.compile({...spec.components.schemas.SearchResponse, $defs:spec.components.schemas,
  properties:{...spec.components.schemas.SearchResponse.properties, hits:{type:'array',items:{$ref:'#/$defs/SearchHit'}}}});

test('real OpenSearch -> Go -> Vue Korean and English searches', async ({page,request}) => {
  const response = await request.get('http://127.0.0.1:8080/api/v1/search?q=검색엔진&lang=ko');
  expect(response.status()).toBe(200);
  expect(response.headers()['x-request-id']).toBeTruthy();
  const data = await response.json();
  expect(validate(data), JSON.stringify(validate.errors)).toBe(true);
  expect(data.hits.length).toBeGreaterThan(0);
  expect(data.hits.every(h => h.language === 'ko')).toBe(true);
  await page.goto('/');
  await page.getByRole('combobox',{name:'검색어'}).fill('검색엔진');
  await page.getByRole('button',{name:'검색',exact:true}).click();
  await expect(page.getByRole('link',{name:'오픈포털 검색엔진 프로젝트'})).toBeVisible();
  await page.getByRole('combobox',{name:'검색어'}).fill('OpenPortal');
  await page.getByRole('combobox',{name:'검색어'}).press('Enter');
  await expect(page.getByRole('link',{name:'OpenPortal developer documentation'})).toBeVisible();
  await page.screenshot({path:'docs/evidence/search-browser.png',fullPage:true});
});
test('empty and invalid requests, suggestions, denied consoles',async ({page,request})=>{
  await page.goto('/');
  await page.getByRole('combobox',{name:'검색어'}).fill('zzzxnomatchq781');
  await page.getByRole('combobox',{name:'검색어'}).press('Enter');
  await expect(page.getByText('검색 결과가 없습니다.')).toBeVisible();
  expect((await request.get('http://127.0.0.1:8080/api/v1/search?q=x&page=101')).status()).toBe(400);
  expect((await request.post('http://127.0.0.1:8080/api/v1/search?q=x')).status()).toBe(405);
  expect((await request.get('http://127.0.0.1:8080/admin')).status()).toBe(404);
  expect((await request.get('http://127.0.0.1:8080/api/v1/search?q=x&vertical=image')).status()).toBe(403);
  const suggest = await request.get('http://127.0.0.1:8080/api/v1/suggest?q=Open');
  expect((await suggest.json()).suggestions).toContain('OpenPortal developer documentation');
});
test('backend outage produces visible error without old results',async ({page})=>{
  // Browser transport fault injection. This case is not a real backend success test.
  await page.route('**/api/v1/search?**',route=>route.fulfill({status:503,body:'{"error":"unavailable"}'}));
  await page.goto('/');
  await page.getByRole('combobox',{name:'검색어'}).fill('OpenPortal');
  await page.getByRole('combobox',{name:'검색어'}).press('Enter');
  await expect(page.getByText('검색 서버 오류 (503)')).toBeVisible();
  await expect(page.locator('.hit')).toHaveCount(0);
});
test('real indexable=false document excluded from search and suggest',async ({request})=>{
  const endpoint='http://127.0.0.1:9200/documents_v1/_doc/fixture-hidden-regression?refresh=true';
  try {
    expect((await request.put(endpoint,{data:{
      document_id:'fixture-hidden-regression',title:'Hiddenpolicyregression',
      url:'https://example.org/hidden',description:'policy fixture',body:'Hiddenpolicyregression',
      language:'en',indexable:false,
    }})).ok()).toBe(true);
    const search=await request.get('http://127.0.0.1:8080/api/v1/search?q=Hiddenpolicyregression');
    expect((await search.json()).total).toBe(0);
    const suggest=await request.get('http://127.0.0.1:8080/api/v1/suggest?q=Hiddenpolicyregression');
    expect((await suggest.json()).suggestions).toEqual([]);
  } finally {
    expect((await request.delete(endpoint)).ok()).toBe(true);
  }
});
test('Phase 1 features: query correction, opt-in recent searches, feature-flagged vertical tabs, settings & i18n, passkey/MFA auth, and console/404 boundaries', async ({page}) => {
  await page.goto('/');
  // 1. Accessibility landmarks & skip link check
  await expect(page.locator('a.skip-link')).toHaveAttribute('href', '#main-content');

  // 2. Query correction (typo 'rjatordpswls' -> '검색엔진')
  const searchBox = page.getByRole('combobox', {name: '검색어'});
  await searchBox.fill('rjatordpswls');
  await searchBox.press('Enter');
  await expect(page.getByTestId('query-correction')).toContainText('검색엔진');
  await page.getByTestId('query-correction').getByRole('button').click();
  await expect(page.getByRole('link', {name: '오픈포털 검색엔진 프로젝트'})).toBeVisible();

  // 3. Feature-flagged vertical tabs (non-web tab shows gate notice)
  await page.locator('.serp-tabs button', {hasText: '이미지'}).click();
  await expect(page.getByText(/라이선스/)).toBeVisible();

  // 4. Settings: enable recent searches opt-in, dark theme, English i18n
  await page.getByRole('button', {name: '검색 설정'}).click();
  await expect(page.getByTestId('settings-panel')).toBeVisible();
  await page.locator('#pref-recent-optin').check();
  await page.locator('#pref-theme').selectOption('dark');
  await expect(page.locator('.app.theme-dark')).toBeVisible();
  await page.locator('#pref-lang').selectOption('en');
  await expect(page.getByText('OpenPortal · Web Search')).toBeVisible();
  await page.locator('#pref-lang').selectOption('ko');

  // Verify recent search recorded when opt-in is enabled
  await searchBox.fill('OpenPortal');
  await searchBox.press('Enter');
  await expect(page.getByRole('button', {name: 'OpenPortal', exact: true})).toBeVisible();

  // 5. Unverified authentication must not claim success.
  await page.getByRole('button', {name: '계정·보안'}).click();
  await expect(page.getByTestId('account-panel')).toBeVisible();
  await page.getByRole('button', {name: '패스키로 시작'}).click();
  await expect(page.getByText('로그인을 사용할 수 없습니다. 인증 서비스 연결과 보안 검증이 필요합니다.')).toBeVisible();

  // 6. Admin/Partner console access boundary screen and 404 screen
  await page.goto('/admin');
  await expect(page.getByTestId('console-denied-view')).toBeVisible();
  await page.goto('/nonexistent-route-404');
  await expect(page.getByTestId('not-found-view')).toBeVisible();
});

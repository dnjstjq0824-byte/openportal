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
  const data = await response.json();
  expect(validate(data), JSON.stringify(validate.errors)).toBe(true);
  expect(data.hits.length).toBeGreaterThan(0);
  expect(data.hits.every(h => h.language === 'ko')).toBe(true);
  await page.goto('/');
  await page.getByRole('textbox',{name:'검색어'}).fill('검색엔진');
  await page.getByRole('button',{name:'검색',exact:true}).click();
  await expect(page.getByRole('link',{name:'오픈포털 검색엔진 프로젝트'})).toBeVisible();
  await page.getByRole('textbox',{name:'검색어'}).fill('OpenPortal');
  await page.getByRole('textbox',{name:'검색어'}).press('Enter');
  await expect(page.getByRole('link',{name:'OpenPortal developer documentation'})).toBeVisible();
  await page.screenshot({path:'docs/evidence/search-browser.png',fullPage:true});
});
test('empty and invalid requests, suggestions, denied consoles',async ({page,request})=>{
  await page.goto('/');
  await page.getByRole('textbox',{name:'검색어'}).fill('zzzxnomatchq781');
  await page.getByRole('textbox',{name:'검색어'}).press('Enter');
  await expect(page.getByText('일치하는 샘플 문서가 없습니다.')).toBeVisible();
  expect((await request.get('http://127.0.0.1:8080/api/v1/search?q=x&page=101')).status()).toBe(400);
  expect((await request.post('http://127.0.0.1:8080/api/v1/search?q=x')).status()).toBe(405);
  expect((await request.get('http://127.0.0.1:8080/admin')).status()).toBe(404);
  const suggest = await request.get('http://127.0.0.1:8080/api/v1/suggest?q=Open');
  expect((await suggest.json()).suggestions).toContain('OpenPortal developer documentation');
});
test('backend outage produces visible error without old results',async ({page})=>{
  // Browser transport fault injection. This case is not a real backend success test.
  await page.route('**/api/v1/search?**',route=>route.fulfill({status:503,body:'{"error":"unavailable"}'}));
  await page.goto('/');
  await page.getByRole('textbox',{name:'검색어'}).fill('OpenPortal');
  await page.getByRole('textbox',{name:'검색어'}).press('Enter');
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

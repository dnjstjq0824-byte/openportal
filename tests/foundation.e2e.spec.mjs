import {test,expect} from '@playwright/test';
import {randomBytes} from 'node:crypto';
import {mkdir,writeFile} from 'node:fs/promises';
import {startStack} from './helpers/platform-stack.mjs';
import {seed} from '../scripts/seed.mjs';

let stack;const index=process.env.FOUNDATION_INDEX;
test.beforeAll(async()=>{
  test.setTimeout(60000);
  await mkdir('docs/evidence/foundation',{recursive:true});
  if(!/^openscanner_foundation_e2e_\d+$/.test(index||''))throw Error('A dedicated test index is required');
  await seed({indexName:index});
  let bulk='';
  for(let n=0;n<12;n++){
    bulk+=JSON.stringify({index:{_index:index,_id:`foundation-${n}`}})+'\n'+JSON.stringify({document_id:`foundation-${n}`,url:`https://example.test/foundation?id=${n}`,title:`OpenScanner foundation document ${n}`,description:'Synthetic document for real local index tests.',body:'OpenScanner foundation search pagination test',language:'en',indexable:true})+'\n';
  }
  bulk+=JSON.stringify({index:{_index:index,_id:'foundation-th'}})+'\n'+JSON.stringify({document_id:'foundation-th',url:'https://example.test/th',title:'OpenScanner ภาษาไทย',description:'เอกสารทดสอบภาษาไทย',body:'OpenScanner ภาษาไทย ค้นหา',language:'th',indexable:true})+'\n';
  const response=await fetch('http://127.0.0.1:9200/_bulk?refresh=true',{method:'POST',headers:{'Content-Type':'application/x-ndjson'},body:bulk});
  expect(response.ok).toBe(true);expect((await response.json()).errors).toBe(false);
  stack=await startStack({origin:'http://127.0.0.1:5175',gatewayPort:8094,pgPort:55445,searchURL:'http://127.0.0.1:8084'});
});
test.beforeEach(async()=>{await stack.pool.query('delete from webapp.rate_limits');});
test.afterAll(async()=>{if(stack)await stack.close();if(/^openscanner_foundation_e2e_\d+$/.test(index||''))await fetch(`http://127.0.0.1:9200/${index}`,{method:'DELETE'});});

test('real local search: Enter, pagination, Unicode URLs, history and provenance',async({page},testInfo)=>{
  await page.goto('/ko');await expect(page.getByRole('link',{name:'OpenScanner',exact:true})).toBeVisible();
  await page.getByRole('searchbox').fill('OpenScanner');await page.getByRole('searchbox').press('Enter');
  await expect(page.locator('.hit')).toHaveCount(10);await expect(page.locator('.corpus-notice')).toBeVisible();
  expect(new URL(page.url()).searchParams.get('q')).toBe('OpenScanner');
  await page.getByRole('button',{name:/다음/}).click();await expect(page.locator('.hit')).toHaveCount(3);expect(new URL(page.url()).searchParams.get('page')).toBe('2');
  await page.reload();await expect(page.getByRole('searchbox')).toHaveValue('OpenScanner');await expect(page.locator('.hit')).toHaveCount(3);
  await page.goBack();await expect(page.locator('.hit')).toHaveCount(10);
  await page.goForward();await expect(page.locator('.hit')).toHaveCount(3);
  await page.getByRole('searchbox').fill('ภาษาไทย');await page.getByRole('searchbox').press('Enter');await expect(page.locator('.hit')).toHaveCount(1);
  await page.getByLabel('언어',{exact:true}).selectOption('th');await expect(page).toHaveURL(/\/th\/search\?q=/);await expect(page.getByRole('searchbox')).toHaveValue('ภาษาไทย');
  await page.getByRole('searchbox').fill('한글 ภาษาไทย & ?');await page.getByRole('searchbox').press('Enter');await expect(page.locator('.serp-shell')).toHaveAttribute('aria-busy','false');expect(new URL(page.url()).searchParams.get('q')).toBe('한글 ภาษาไทย & ?');
  await page.getByRole('searchbox').fill('zzzxnomatchq781');await page.getByRole('searchbox').press('Enter');await expect(page.locator('.no-results')).toBeVisible();
  await page.getByRole('searchbox').fill('OpenScanner');await page.getByRole('searchbox').press('Enter');await expect(page.locator('.hit')).toHaveCount(10);
  await mkdir('docs/evidence/foundation',{recursive:true});await page.screenshot({path:`docs/evidence/foundation/${testInfo.project.name}-search.png`,fullPage:true});
  const samples=[];let failures=0;
  for(let batch=0;batch<10;batch++)await Promise.all([0,1].map(async()=>{const start=performance.now();const response=await page.request.get('/api/v2/search?q=OpenScanner');if(!response.ok())failures++;samples.push(performance.now()-start);}));
  samples.sort((a,b)=>a-b);expect(failures).toBe(0);
  await writeFile(`docs/evidence/foundation/${testInfo.project.name}-search-latency.json`,JSON.stringify({scope:'local synthetic index; not global coverage or production capacity',matchingDocuments:13,requests:20,concurrency:2,failures,p50ms:samples[9],p95ms:samples[18]},null,2));
});

test('OpenScanner auth design: three locales, service unavailable retry, responsive steps',async({page},testInfo)=>{
  const errors=[];page.on('pageerror',error=>errors.push(error.message));
  await page.route('**/api/v2/auth/username?*',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({signupEnabled:false,available:false,domain:'openscanner.test',suggestions:[]})}));
  await page.goto('/en/signup');await expect(page.getByRole('alert')).toContainText('cannot be created');await expect(page.getByRole('button',{name:'Next',exact:true})).toBeDisabled();
  await page.unroute('**/api/v2/auth/username?*');await page.getByRole('button',{name:'Try again',exact:true}).click();await expect(page.getByRole('button',{name:'Next',exact:true})).toBeEnabled();
  for(const locale of ['ko','th','en'])for(const path of ['login','signup']){
    await page.goto(`/${locale}/${path}`);await expect(page.locator('html')).toHaveAttribute('lang',locale);
    await expect(page.locator('.brand')).toContainText('OpenScanner');
    for(const width of [320,375,390,430,768,1024,1440,1920]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`${locale}/${path} at ${width}`).toBe(true);}
  }
  await page.setViewportSize(testInfo.project.use.viewport);
  await page.goto('/ko/login');await page.screenshot({path:`docs/evidence/foundation/${testInfo.project.name}-login.png`,fullPage:true});
  await page.goto('/ko/signup');await expect(page.getByRole('progressbar')).toHaveAttribute('aria-valuenow','1');await page.screenshot({path:`docs/evidence/foundation/${testInfo.project.name}-signup.png`,fullPage:true});
  expect(errors).toEqual([]);
});

test('real PostgreSQL and loopback SMTP: wizard, recovery, login, search and mailbox return',async({page})=>{
  const username=`foundation.${randomBytes(4).toString('hex')}`,recovery=`owner.${randomBytes(4).toString('hex')}@example.test`,password=randomBytes(18).toString('base64url');
  const next=()=>page.getByRole('button',{name:'Next',exact:true}).click();
  await page.goto('/en/mail');await expect(page).toHaveURL(/\/en\/login\?returnTo=/);await page.getByRole('link',{name:'Create account',exact:true}).click();
  await page.locator('#su-given').fill('Foundation');await next();
  await page.locator('#su-year').fill('1990');await page.locator('#su-month').selectOption('5');await page.locator('#su-day').fill('17');await page.locator('#su-gender').selectOption('unspecified');await next();
  await page.locator('#su-username').fill(username);await expect(page.locator('.availability.available')).toBeVisible();await next();
  await page.locator('#su-password').fill(password);await page.locator('#su-confirm').fill(password);await next();
  await page.locator('#su-recovery').fill(recovery);await next();await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Create account',exact:true}).click();
  await expect(page.getByRole('heading',{name:'Your account is ready',exact:true})).toBeVisible();await page.getByRole('link',{name:'Open mailbox',exact:true}).click();await expect(page.getByRole('button',{name:'Compose',exact:true})).toBeVisible();
  const verification=stack.smtp.tokenFor(recovery,'verify');await page.goto(`/en/verify-email#token=${verification}`);await page.getByRole('button',{name:'Verify email',exact:true}).click();await expect(page.getByRole('status')).toBeVisible();
  await page.locator('.account-menu summary').click();await page.getByRole('button',{name:'Sign out',exact:true}).click();
  await page.goto('/en/mail');await page.locator('#auth-email').fill(username);await page.locator('#auth-password').fill(password);await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page).toHaveURL(/\/en\/mail$/);
  await page.getByRole('link',{name:'OpenScanner',exact:true}).click();await page.getByRole('searchbox').fill('OpenScanner');await page.getByRole('searchbox').press('Enter');await expect(page.locator('.hit')).toHaveCount(10);
  await page.locator('.services-menu summary').click();await page.locator('.services-popover a[href="/en/mail"]').click();await expect(page.getByRole('button',{name:'Compose',exact:true})).toBeVisible();
  await page.locator('.account-menu summary').click();await page.getByRole('button',{name:'Sign out',exact:true}).click();
  await page.goto('/en/forgot-password');await page.locator('#auth-email').fill(username);await page.getByRole('button',{name:'Reset password',exact:true}).click();await expect(page.getByRole('status')).toBeVisible();
  const reset=stack.smtp.tokenFor(recovery,'reset'),nextPassword=randomBytes(18).toString('base64url');await page.goto(`/en/reset-password#token=${reset}`);await page.locator('#auth-password').fill(nextPassword);await page.locator('#auth-confirm').fill(nextPassword);await page.getByRole('button',{name:'Reset password',exact:true}).click();await expect(page.getByRole('status')).toBeVisible();
  await page.goto('/en/login?returnTo=%2Fen%2Fmail');await page.locator('#auth-email').fill(username);await page.locator('#auth-password').fill(nextPassword);await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page).toHaveURL(/\/en\/mail$/);
});

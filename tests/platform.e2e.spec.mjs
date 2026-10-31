import {test,expect} from '@playwright/test';
import {randomBytes} from 'node:crypto';
import {mkdir,writeFile} from 'node:fs/promises';
import {startStack} from './helpers/platform-stack.mjs';
import {seed} from '../scripts/seed.mjs';
let stack;
test.beforeAll(async()=>{test.setTimeout(60000);await seed({indexName:process.env.PLATFORM_TEST_INDEX});stack=await startStack({pgPort:55437});});
test.afterAll(async()=>{if(stack)await stack.close();await fetch(`http://127.0.0.1:9200/${process.env.PLATFORM_TEST_INDEX}`,{method:'DELETE'});});

test('real local index: Enter, URL state, refresh, history and language switching',async({page},testInfo)=>{
  await page.goto('/ko');await expect(page.locator('.site-header').getByRole('link',{name:'로그인',exact:true})).toHaveCount(1);
  await page.getByRole('searchbox',{name:'검색',exact:true}).fill('검색엔진');await page.getByRole('searchbox',{name:'검색',exact:true}).press('Enter');
  await expect(page).toHaveURL(/\/ko\/search\?q=/);await expect(page.getByRole('link',{name:/오픈포털 검색엔진 프로젝트/})).toBeVisible();
  await page.reload();await expect(page.getByRole('searchbox')).toHaveValue('검색엔진');await expect(page.locator('.hit')).not.toHaveCount(0);
  /* cspell:disable */
  await page.getByRole('searchbox').fill('한글 ภาษาไทย & special ?');await page.getByRole('button',{name:'검색',exact:true}).click();
  await expect(page.getByText('검색 결과가 없습니다.',{exact:true})).toBeVisible();expect(new URL(page.url()).searchParams.get('q')).toBe('한글 ภาษาไทย & special ?');
  await page.goBack();await expect(page.getByRole('searchbox')).toHaveValue('검색엔진');await page.goForward();await expect(page.getByRole('searchbox')).toHaveValue('한글 ภาษาไทย & special ?');
  await page.getByLabel('언어',{exact:true}).selectOption('th');await expect(page).toHaveURL(/\/th\/search/);await expect(page.getByRole('searchbox')).toHaveValue('한글 ภาษาไทย & special ?');await expect(page.locator('html')).toHaveAttribute('lang','th');
  /* cspell:enable */
  await page.goto('/en');await expect(page.getByRole('heading',{name:'Discover beyond search.'})).toBeVisible();await page.getByLabel('Color theme').selectOption('dark');await expect(page.locator('html')).toHaveAttribute('data-theme','dark');
  await mkdir('docs/evidence/platform',{recursive:true});await page.screenshot({path:`docs/evidence/platform/${testInfo.project.name}-home.png`,fullPage:true});
  const timings=[];let failures=0;const started=performance.now();
  for(let group=0;group<10;group++)await Promise.all([0,1].map(async()=>{const before=performance.now();const response=await page.request.get('/api/v2/search?q=OpenPortal');if(!response.ok())failures++;timings.push(performance.now()-before);}));
  timings.sort((a,b)=>a-b);await writeFile(`docs/evidence/platform/${testInfo.project.name}-benchmark.json`,JSON.stringify({scope:'local synthetic index, not external web coverage or production capacity',documents:3,requests:20,concurrency:2,failures,p50ms:timings[9],p95ms:timings[18],elapsedMs:performance.now()-started},null,2));expect(failures).toBe(0);
});

test('actual PostgreSQL + SMTP: signup, verification, session, protected return, mail adapter and reset',async({page},testInfo)=>{
  const email=`browser-${randomBytes(6).toString('hex')}@example.test`;const password=randomBytes(18).toString('base64url');
  await page.goto('/en/mail');await expect(page).toHaveURL(/\/en\/login\?returnTo=/);await page.getByRole('link',{name:'Create account',exact:true}).click();
  await page.getByLabel('Email address',{exact:true}).fill(email);await page.getByLabel('Display name').fill('Browser tester');await page.getByLabel('Password',{exact:true}).fill(password);await page.getByLabel('Confirm password').fill(password);await page.getByRole('checkbox').check();await page.getByRole('button',{name:'Create account',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('verification link');const token=stack.smtp.tokenFor(email,'verify');
  await page.goto(`/en/verify-email#token=${token}`);await page.getByRole('button',{name:'Verify email',exact:true}).click();await expect(page.getByRole('status')).toContainText('Email verified');expect(page.url()).not.toContain(token);
  await stack.provision(email);await page.goto('/en/mail');await expect(page).toHaveURL(/\/en\/login/);
  await page.getByLabel('Email address',{exact:true}).first().fill(email);await page.getByLabel('Password',{exact:true}).fill(password);await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page).toHaveURL(/\/en\/mail$/);await expect(page.getByRole('button',{name:'Compose',exact:true})).toBeVisible();
  await page.reload();await expect(page.getByRole('button',{name:'Compose',exact:true})).toBeVisible();await expect(page.locator('.site-header').getByRole('link',{name:'Sign in',exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'Compose',exact:true}).click();await page.getByLabel('To',{exact:true}).fill('recipient@example.test');await page.getByLabel('Subject',{exact:true}).fill('Browser protocol test');await page.getByLabel('Message',{exact:true}).fill('Protocol simulator message');
  await page.getByLabel('Attach file',{exact:true}).setInputFiles({name:'note.txt',mimeType:'text/plain',buffer:Buffer.from('Browser attachment')});await expect(page.getByText('note.txt',{exact:false})).toBeVisible();
  await page.getByRole('button',{name:'Send',exact:true}).click();await expect(page.getByRole('status')).toContainText('accepted');
  await page.getByRole('button',{name:/^Sent/}).click();await expect(page.getByRole('button',{name:/Browser protocol test/})).toBeVisible();await page.getByRole('button',{name:/Browser protocol test/}).click();await expect(page.getByRole('heading',{name:'Browser protocol test'})).toBeVisible();
  await page.screenshot({path:`docs/evidence/platform/${testInfo.project.name}-mail.png`,fullPage:true});
  await page.locator('.header-nav details').last().locator('summary').click();await page.getByRole('button',{name:'Sign out',exact:true}).click();await page.goto('/en/forgot-password');await page.getByLabel('Email address',{exact:true}).fill(email);await page.getByRole('button',{name:'Reset password',exact:true}).click();await expect(page.getByRole('status')).toContainText('reset link');
  const reset=stack.smtp.tokenFor(email,'reset');const next=randomBytes(18).toString('base64url');await page.goto(`/en/reset-password#token=${reset}`);await page.getByLabel('Password',{exact:true}).fill(next);await page.getByLabel('Confirm password').fill(next);await page.getByRole('button',{name:'Reset password',exact:true}).click();await expect(page.getByRole('status')).toContainText('Password updated');
  await page.goto('/en/login?returnTo=https://attacker.invalid/');await page.getByLabel('Email address',{exact:true}).first().fill(email);await page.getByLabel('Password',{exact:true}).fill(next);await page.getByRole('button',{name:'Sign in',exact:true}).click();await expect(page).toHaveURL(/127\.0\.0\.1:5173\/en\/mail$/);
});

test('responsive public routes do not overflow at required widths',async({page})=>{
  for(const width of [320,375,390,430,768,1024,1440,1920]){await page.setViewportSize({width,height:900});for(const locale of ['ko','th','en']){await page.goto(`/${locale}/signup`);await expect(page.locator('html')).toHaveAttribute('lang',locale);expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);}}
});

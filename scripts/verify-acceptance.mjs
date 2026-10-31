import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from '@playwright/test';
import http from 'node:http';

async function main() {
  console.log('--- STARTING OPENSCAN ACCEPTANCE CRITERIA VERIFICATION (A-01 to A-14) ---\n');

  // A-01 Check screenshots
  const requiredScreenshots = [
    'docs/evidence/openscan-post-desktop-1440-home.png',
    'docs/evidence/openscan-post-desktop-1440-serp.png',
    'docs/evidence/openscan-post-desktop-1440-login.png',
    'docs/evidence/openscan-post-desktop-1440-signup.png',
    'docs/evidence/openscan-post-mobile-390-home.png',
    'docs/evidence/openscan-post-mobile-390-serp.png'
  ];
  let a01Pass = true;
  for (const s of requiredScreenshots) {
    try {
      const st = await fs.stat(s);
      if (st.size < 1000) {
        console.error(`A-01 FAIL: Screenshot ${s} is too small: ${st.size}`);
        a01Pass = false;
      }
    } catch {
      console.error(`A-01 FAIL: Missing screenshot ${s}`);
      a01Pass = false;
    }
  }
  console.log(`[A-01] Screenshots existence (6/6): ${a01Pass ? 'PASS' : 'FAIL'}`);

  // A-03 Grep forbidden strings in apps/web/dist
  const distDir = path.resolve('apps/web/dist');
  let a03Pass = true;
  const distFiles = [];
  async function collectFiles(d) {
    for (const entry of await fs.readdir(d, { withFileTypes: true })) {
      const full = path.join(d, entry.name);
      if (entry.isDirectory()) await collectFiles(full);
      else distFiles.push(full);
    }
  }
  await collectFiles(distDir);

  const forbiddenStrings = ['OpenPortal', 'Open Scanner', 'OpenScanner', '1,280,450', '#F0441E', '#f0441e'];
  for (const file of distFiles) {
    const content = await fs.readFile(file, 'utf-8');
    for (const s of forbiddenStrings) {
      if (content.includes(s)) {
        console.error(`A-03 FAIL: Found forbidden string "${s}" in ${file}`);
        a03Pass = false;
      }
    }
  }
  console.log(`[A-03] Forbidden strings in dist (0 matches): ${a03Pass ? 'PASS' : 'FAIL'}`);

  // A-04 Check modal code/styles in App.vue and portal.css
  const appVueContent = await fs.readFile('apps/web/src/App.vue', 'utf-8');
  const portalCssContent = await fs.readFile('apps/web/src/portal.css', 'utf-8');
  
  const modalClassMatchesInCss = portalCssContent.match(/\.modal[a-zA-Z0-9_-]*/g) || [];
  const modalClassMatchesInVue = appVueContent.match(/class="[^"]*modal[^"]*"/g) || [];
  const a04Pass = modalClassMatchesInCss.length === 0 && modalClassMatchesInVue.length === 0;
  console.log(`[A-04] Modal removal check (CSS matches: ${modalClassMatchesInCss.length}, Vue matches: ${modalClassMatchesInVue.length}): ${a04Pass ? 'PASS' : 'FAIL'}`);

  // A-09 Check href="#" in apps/web/src
  const srcFiles = [];
  async function collectSrc(d) {
    for (const entry of await fs.readdir(d, { withFileTypes: true })) {
      const full = path.join(d, entry.name);
      if (entry.isDirectory()) await collectSrc(full);
      else if (entry.name.endsWith('.vue') || entry.name.endsWith('.ts') || entry.name.endsWith('.html')) srcFiles.push(full);
    }
  }
  await collectSrc(path.resolve('apps/web/src'));
  let a09Pass = true;
  for (const file of srcFiles) {
    const content = await fs.readFile(file, 'utf-8');
    if (content.includes('href="#"') || content.includes("href='#'")) {
      console.error(`A-09 FAIL: Found href="#" in ${file}`);
      a09Pass = false;
    }
  }
  console.log(`[A-09] Empty anchor href="#" (0 occurrences): ${a09Pass ? 'PASS' : 'FAIL'}`);

  // Start test server for browser checks
  /* cspell:disable */
  const MIME_TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.svg': 'image/svg+xml',
  };

  const server = http.createServer(async (req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1:5198');
    if (url.pathname.startsWith('/api/v2/search')) {
      const q = url.searchParams.get('q');
      if (q === 'xyznonexistent') {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ hits: [], total: 0, elapsed_ms: 5 }));
        return;
      }
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({
        hits: [{ url: 'https://example.com/test', title: 'OpenScan Test', snippet: 'A test result snippet.' }],
        total: 1,
        elapsed_ms: 10
      }));
      return;
    }
    if (url.pathname.startsWith('/api/v2/auth/session')) {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(null));
      return;
    }
    if (url.pathname.startsWith('/api/')) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'not_found' }));
      return;
    }
    let filePath = path.join(distDir, url.pathname);
    try {
      const stat = await fs.stat(filePath);
      if (stat.isDirectory()) filePath = path.join(filePath, 'index.html');
    } catch {
      filePath = path.join(distDir, 'index.html');
    }
    try {
      const ext = path.extname(filePath);
      const content = await fs.readFile(filePath);
      res.writeHead(200, { 'Content-Type': MIME_TYPES[ext] || 'application/octet-stream' });
      res.end(content);
    } catch {
      res.writeHead(404, { 'Content-Type': 'text/plain' });
      res.end('Not Found');
    }
  });

  await new Promise(resolve => server.listen(5198, '127.0.0.1', resolve));

  const browser = await chromium.launch();
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

    // A-02 Check Colors
    await page.goto('http://127.0.0.1:5198/ko', { waitUntil: 'networkidle' });
    const bodyBg = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    const loginBtnStyles = await page.evaluate(() => {
      const btn = document.querySelector('.btn-login-gray');
      if (!btn) return null;
      const s = getComputedStyle(btn);
      return { bg: s.backgroundColor, border: s.borderColor, color: s.color };
    });

    const isWhiteBg = bodyBg === 'rgb(255, 255, 255)';
    const isLoginGrayBg = loginBtnStyles?.bg === 'rgb(241, 243, 244)';
    const isLoginGrayBorder = loginBtnStyles?.border === 'rgb(232, 234, 237)';
    const isLoginGrayColor = loginBtnStyles?.color === 'rgb(60, 64, 67)';
    const a02Pass = isWhiteBg && isLoginGrayBg && isLoginGrayBorder && isLoginGrayColor;
    console.log(`[A-02] Design tokens check: bodyBg=${bodyBg} (#FFFFFF), loginBtnBg=${loginBtnStyles?.bg} (#F1F3F4), border=${loginBtnStyles?.border} (#E8EAED), color=${loginBtnStyles?.color} (#3C4043): ${a02Pass ? 'PASS' : 'FAIL'}`);

    // A-08 Toast auto-dismissal
    await page.click('button.ai-chip');
    const toastVisibleBefore = await page.locator('.feature-toast').isVisible();
    await page.waitForTimeout(3600);
    const toastVisibleAfter = await page.locator('.feature-toast').isVisible();
    const a08Pass = toastVisibleBefore && !toastVisibleAfter;
    console.log(`[A-08] Toast notification 3.5s auto-dismiss (visible before: ${toastVisibleBefore}, after: ${toastVisibleAfter}): ${a08Pass ? 'PASS' : 'FAIL'}`);

    // A-07 0-results screen
    await page.goto('http://127.0.0.1:5198/ko/search?q=xyznonexistent', { waitUntil: 'networkidle' });
    const zeroResultsBox = await page.locator('.empty-result-card').isVisible();
    const tipsCount = await page.locator('.empty-tips-list li').count();
    const a07Pass = zeroResultsBox && tipsCount >= 3;
    console.log(`[A-07] 0-results screen (empty-result-card: ${zeroResultsBox}, tipsCount: ${tipsCount} >= 3): ${a07Pass ? 'PASS' : 'FAIL'}`);

    // A-10 Static pages (8 routes)
    const staticRoutes = ['about', 'store', 'advertising', 'business', 'howsearchworks', 'privacy', 'terms', 'settings'];
    let a10Pass = true;
    for (const r of staticRoutes) {
      await page.goto(`http://127.0.0.1:5198/ko/${r}`, { waitUntil: 'networkidle' });
      const wrapVisible = await page.locator('.static-page-wrap').isVisible();
      if (!wrapVisible) {
        console.error(`A-10 FAIL: Static page wrap not visible for /ko/${r}`);
        a10Pass = false;
      }
    }
    console.log(`[A-10] Static pages 8 routes (/about, /store, etc.): ${a10Pass ? 'PASS' : 'FAIL'}`);

    // A-13 Responsive check: no horizontal overflow on 320, 375, 390, 430, 768, 1024, 1440, 1920
    let a13Pass = true;
    for (const w of [320, 375, 390, 430, 768, 1024, 1440, 1920]) {
      await page.setViewportSize({ width: w, height: 800 });
      for (const route of ['/ko', '/ko/signup', '/ko/login']) {
        await page.goto(`http://127.0.0.1:5198${route}`, { waitUntil: 'networkidle' });
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth);
        if (overflow) {
          const sw = await page.evaluate(() => document.documentElement.scrollWidth);
          console.error(`A-13 FAIL: Horizontal overflow at width ${w}px on ${route} (scrollWidth=${sw})`);
          a13Pass = false;
        }
      }
    }
    console.log(`[A-13] Responsive horizontal overflow check (320px ~ 1920px across /, /signup, /login): ${a13Pass ? 'PASS' : 'FAIL'}`);

    // A-11 Routing check: dedicated login & signup pages
    await page.goto('http://127.0.0.1:5198/ko/login', { waitUntil: 'networkidle' });
    const isLoginWrap = await page.locator('.auth-page-wrap').isVisible();
    await page.goto('http://127.0.0.1:5198/ko/signup', { waitUntil: 'networkidle' });
    const isSignupWrap = await page.locator('.auth-page-wrap').isVisible();
    const a11Pass = isLoginWrap && isSignupWrap;
    console.log(`[A-11] Dedicated auth pages routing (/login, /signup): ${a11Pass ? 'PASS' : 'FAIL'}`);

    // A-12 Accessibility: html lang attribute, labels
    await page.goto('http://127.0.0.1:5198/en', { waitUntil: 'networkidle' });
    const langEn = await page.getAttribute('html', 'lang');
    await page.goto('http://127.0.0.1:5198/th', { waitUntil: 'networkidle' });
    const langTh = await page.getAttribute('html', 'lang');
    await page.goto('http://127.0.0.1:5198/ko', { waitUntil: 'networkidle' });
    const langKo = await page.getAttribute('html', 'lang');
    const a12Pass = langEn === 'en' && langTh === 'th' && langKo === 'ko';
    console.log(`[A-12] Accessibility html lang attribute (en, th, ko): ${a12Pass ? 'PASS' : 'FAIL'}`);

  } finally {
    await browser.close();
    await new Promise(resolve => server.close(resolve));
  }
  console.log('\n--- ACCEPTANCE VERIFICATION FINISHED ---');
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});

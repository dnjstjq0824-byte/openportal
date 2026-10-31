import http from 'node:http';
import fs from 'node:fs/promises';
import path from 'node:path';
import { chromium } from '@playwright/test';

const DIST_DIR = path.resolve('apps/web/dist');

const MIME_TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.json': 'application/json; charset=utf-8',
};

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, 'http://127.0.0.1:5199');

  if (url.pathname.startsWith('/api/v2/search')) {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
      results: [
        {
          url: 'https://example.com/item1',
          title: '오픈스캔 웹 검색엔진 프로젝트',
          snippet: 'OpenScan은 실제 웹을 인덱싱하고 빠르고 정확한 검색 결과를 제공하는 차세대 독립 검색엔진입니다.'
        },
        {
          url: 'https://example.org/tech/architecture',
          title: '검색엔진 기술 아키텍처 및 분산 크롤러',
          snippet: 'Go 기반 백엔드와 분산 인덱서, 역색인 및 RRF 랭킹 알고리즘을 결합한 고성능 아키텍처 설계.'
        }
      ],
      total: 2,
      took_ms: 18
    }));
    return;
  }

  if (url.pathname.startsWith('/api/')) {
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not_found' }));
    return;
  }

  let filePath = path.join(DIST_DIR, url.pathname);
  try {
    const stat = await fs.stat(filePath);
    if (stat.isDirectory()) {
      filePath = path.join(filePath, 'index.html');
    }
  } catch {
    filePath = path.join(DIST_DIR, 'index.html');
  }

  try {
    const ext = path.extname(filePath);
    const content = await fs.readFile(filePath);
    res.writeHead(200, { 'Content-Type': MIME_TYPES[ext] || 'application/octet-stream' });
    res.end(content);
  } catch (err) {
    res.writeHead(404, { 'Content-Type': 'text/plain' });
    res.end('Not Found');
  }
});

await new Promise(resolve => server.listen(5199, '127.0.0.1', resolve));
console.log('Static test server listening on http://127.0.0.1:5199');

await fs.mkdir('docs/evidence', { recursive: true });

const browser = await chromium.launch();

try {
  // 1. Desktop 1440x900
  const desktopContext = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1
  });
  const desktopPage = await desktopContext.newPage();

  // Desktop Home
  console.log('Capturing Desktop Home...');
  await desktopPage.goto('http://127.0.0.1:5199/ko', { waitUntil: 'networkidle' });
  await desktopPage.screenshot({ path: 'docs/evidence/openscan-post-desktop-1440-home.png' });

  // Desktop SERP
  console.log('Capturing Desktop SERP...');
  await desktopPage.goto('http://127.0.0.1:5199/ko/search?q=%EA%B2%80%EC%83%89%EC%97%94%EC%A7%84', { waitUntil: 'networkidle' });
  await desktopPage.screenshot({ path: 'docs/evidence/openscan-post-desktop-1440-serp.png' });

  // Desktop Login
  console.log('Capturing Desktop Login...');
  await desktopPage.goto('http://127.0.0.1:5199/ko/login', { waitUntil: 'networkidle' });
  await desktopPage.screenshot({ path: 'docs/evidence/openscan-post-desktop-1440-login.png' });

  // Desktop Signup
  console.log('Capturing Desktop Signup...');
  await desktopPage.goto('http://127.0.0.1:5199/ko/signup', { waitUntil: 'networkidle' });
  await desktopPage.screenshot({ path: 'docs/evidence/openscan-post-desktop-1440-signup.png' });

  await desktopContext.close();

  // 2. Mobile 390x844 (iPhone 12/13/14 size)
  const mobileContext = await browser.newContext({
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 1,
    isMobile: true
  });
  const mobilePage = await mobileContext.newPage();

  // Mobile Home
  console.log('Capturing Mobile Home...');
  await mobilePage.goto('http://127.0.0.1:5199/ko', { waitUntil: 'networkidle' });
  await mobilePage.screenshot({ path: 'docs/evidence/openscan-post-mobile-390-home.png' });

  // Mobile SERP
  console.log('Capturing Mobile SERP...');
  await mobilePage.goto('http://127.0.0.1:5199/ko/search?q=%EA%B2%80%EC%83%89%EC%97%94%EC%A7%84', { waitUntil: 'networkidle' });
  await mobilePage.screenshot({ path: 'docs/evidence/openscan-post-mobile-390-serp.png' });

  await mobileContext.close();
  console.log('All screenshots captured successfully.');
} finally {
  await browser.close();
  await new Promise(resolve => server.close(resolve));
}


import { chromium } from '@playwright/test';

const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  
  for (const path of ['/ko/login', '/ko/signup', '/ko/about', '/ko/howsearchworks']) {
    const url = 'https://openportal.pages.dev' + path;
    const res = await page.goto(url, { waitUntil: 'networkidle' });
    console.log(`[${path}] Status: ${res.status()}, URL: ${page.url()}`);
  }
} finally {
  await browser.close();
}


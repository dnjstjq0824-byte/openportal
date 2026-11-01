import { chromium } from '@playwright/test';

const browser = await chromium.launch();
try {
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  console.log('Navigating to live Cloudflare Pages URL: https://openportal.pages.dev/ko');
  const response = await page.goto('https://openportal.pages.dev/ko', { waitUntil: 'networkidle' });
  console.log('Live HTTP status:', response.status());
  
  const title = await page.title();
  console.log('Page Title:', title);

  const logoWordmark = await page.locator('.logo-wordmark').textContent();
  console.log('Wordmark on page:', logoWordmark);

  await page.screenshot({ path: 'docs/evidence/live-openscan-deployed.png' });
  console.log('Live screenshot saved to docs/evidence/live-openscan-deployed.png');
} finally {
  await browser.close();
}

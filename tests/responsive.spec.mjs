import { test, expect } from '@playwright/test';

const viewports = [
  { name: '320_small_mobile', width: 320, height: 568 },
  { name: '375_mobile', width: 375, height: 667 },
  { name: '390_mobile_standard', width: 390, height: 844 },
  { name: '430_large_mobile', width: 430, height: 932 },
  { name: '768_tablet', width: 768, height: 1024 },
  { name: '1024_desktop_small', width: 1024, height: 768 },
  { name: '1366_desktop_laptop', width: 1366, height: 768 },
  { name: '1440_desktop_standard', width: 1440, height: 900 },
  { name: '1920_desktop_fhd', width: 1920, height: 1080 },
];

test.describe('Responsive Viewport & Google UI/UX Full Audit', () => {
  for (const vp of viewports) {
    test(`Viewport ${vp.name} (${vp.width}x${vp.height}) - Home & SERP & DarkMode`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height });

      // 1. Home Screen
      await page.goto('/');
      await expect(page.locator('.brand')).toBeVisible();
      await expect(page.getByRole('combobox', { name: '검색어' })).toBeVisible();

      // Ensure no horizontal scrollbar (overflow-x: hidden)
      const scrollWidthHome = await page.evaluate(() => document.documentElement.scrollWidth);
      expect(scrollWidthHome).toBeLessThanOrEqual(vp.width);

      if (vp.width >= 1024) {
        await page.screenshot({ path: `docs/evidence/viewport-${vp.name}-home.png`, fullPage: true });
      }

      // 2. Search & SERP
      await page.getByRole('combobox', { name: '검색어' }).fill('검색엔진');
      await page.getByRole('button', { name: '검색', exact: true }).click();
      await expect(page.locator('.hit').first()).toBeVisible();

      const scrollWidthSerp = await page.evaluate(() => document.documentElement.scrollWidth);
      expect(scrollWidthSerp).toBeLessThanOrEqual(vp.width);

      if (vp.width === 375 || vp.width === 1440) {
        await page.screenshot({ path: `docs/evidence/viewport-${vp.name}-serp.png`, fullPage: true });
      }

      // 3. Dark Mode
      await page.getByRole('button', { name: '검색 설정' }).click();
      await page.locator('#pref-theme').selectOption('dark');
      await expect(page.locator('.app.theme-dark')).toBeVisible();

      if (vp.width === 375 || vp.width === 1440) {
        await page.screenshot({ path: `docs/evidence/viewport-${vp.name}-dark.png`, fullPage: true });
      }
    });
  }
});


import { defineConfig } from '@playwright/test';
import { existsSync } from 'node:fs';
const chrome = 'C:/Program Files/Google/Chrome/Application/chrome.exe';
export default defineConfig({
  testDir: 'tests', testMatch: 'mail-reliability.e2e.spec.mjs', workers: 1,
  timeout: 30000, globalTimeout: 180000, reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:5173', browserName: 'chromium', locale: 'en-US', trace: 'off', screenshot: 'off',
    launchOptions: { executablePath: process.env.CHROME_PATH || (existsSync(chrome) ? chrome : undefined) } },
  projects: [
    { name: 'desktop', use: { viewport: { width: 1440, height: 1000 } } },
    { name: 'mobile-emulation', use: { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true } },
  ],
  webServer: { command: `${process.platform === 'win32' ? 'npm.cmd' : 'npm'} run dev -- --strictPort`, cwd: 'apps/web',
    url: 'http://127.0.0.1:5173', reuseExistingServer: false, timeout: 30000 },
});

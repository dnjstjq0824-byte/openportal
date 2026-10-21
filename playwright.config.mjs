import { defineConfig } from '@playwright/test';
export default defineConfig({
  testDir:'tests', workers:1, reporter:'list',
  use:{baseURL:'http://127.0.0.1:5173', browserName:'chromium',
    launchOptions: process.env.CHROME_PATH ? {executablePath:process.env.CHROME_PATH} : {}},
  webServer:[
    {command:'go run ./cmd/api',cwd:'backend',url:'http://127.0.0.1:8080/readyz',reuseExistingServer:!process.env.CI,timeout:60000},
    {command:'npm run dev -- --strictPort',cwd:'apps/web',url:'http://127.0.0.1:5173',reuseExistingServer:!process.env.CI},
  ],
});

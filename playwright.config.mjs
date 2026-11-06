import { defineConfig } from '@playwright/test';
// Default E2E gate: current-UI suites only. foundation.e2e.spec.mjs needs its
// own config (dedicated OpenSearch index, ports 8084/8094/5175); the pre-
// foundation specs (open_scanner_auth, search, recovery, responsive) target a
// removed combobox/modal UI and are deprecated in-file. OpenSearch must be
// reachable at 127.0.0.1:9200 (CI compose provides it; local: node scripts/start-opensearch.mjs).
const index = process.env.PLATFORM_TEST_INDEX || `portal_gateway_e2e_${Date.now()}`;
process.env.PLATFORM_TEST_INDEX = index;
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
export default defineConfig({
  testDir:'tests', testMatch:['platform.e2e.spec.mjs','mail-reliability.e2e.spec.mjs'], workers:1, reporter:'list',
  use:{baseURL:'http://127.0.0.1:5173', browserName:'chromium',
    launchOptions: process.env.CHROME_PATH ? {executablePath:process.env.CHROME_PATH} : {}},
  webServer:[
    {command:'go run ./cmd/api',cwd:'backend',url:'http://127.0.0.1:8080/readyz',reuseExistingServer:!process.env.CI,timeout:60000,env:{OPENSEARCH_INDEX:index}},
    {command:`${npm} run dev -- --strictPort`,cwd:'apps/web',url:'http://127.0.0.1:5173',reuseExistingServer:!process.env.CI},
  ],
});

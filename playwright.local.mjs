import { defineConfig } from '@playwright/test';
// Local verification when 8080 is occupied (e.g. user Docker): Go backend on 18777.
// Same suites and assertions as the default config; only the backend port differs.
const index = process.env.PLATFORM_TEST_INDEX || `portal_gateway_e2e_${Date.now()}`;
process.env.PLATFORM_TEST_INDEX = index;
process.env.SEARCH_BACKEND_URL = 'http://127.0.0.1:18777';
const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
export default defineConfig({
  testDir: 'tests',
  testMatch: ['platform.e2e.spec.mjs', 'mail-reliability.e2e.spec.mjs'],
  workers: 1,
  reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:5173', browserName: 'chromium' },
  webServer: [
    { command: 'go run ./cmd/api', cwd: 'backend', url: 'http://127.0.0.1:18777/readyz', reuseExistingServer: false, timeout: 120000, env: { OPENSEARCH_INDEX: index, PORT: '18777' } },
    { command: `${npm} run dev -- --strictPort`, cwd: 'apps/web', url: 'http://127.0.0.1:5173', reuseExistingServer: false },
  ],
});

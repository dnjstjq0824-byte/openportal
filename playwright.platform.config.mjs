import {defineConfig} from '@playwright/test';
import {resolve} from 'node:path';
const index=process.env.PLATFORM_TEST_INDEX||`portal_gateway_e2e_${Date.now()}`;process.env.PLATFORM_TEST_INDEX=index;
export default defineConfig({
  testDir:'tests',testMatch:'platform.e2e.spec.mjs',workers:1,timeout:30000,globalTimeout:180000,reporter:'list',
  use:{baseURL:'http://127.0.0.1:5173',browserName:'chromium',locale:'en-US',trace:'off',screenshot:'off',launchOptions:{executablePath:process.env.CHROME_PATH||'C:/Program Files/Google/Chrome/Application/chrome.exe'}},
  projects:[{name:'desktop',use:{viewport:{width:1440,height:1000}}},{name:'mobile-emulation',use:{viewport:{width:390,height:844},isMobile:true,hasTouch:true}}],
  webServer:[
    {command:'go run ./cmd/api',cwd:'backend',url:'http://127.0.0.1:8080/healthz',reuseExistingServer:false,timeout:60000,env:{GOCACHE:resolve('work/go-cache'),OPENSEARCH_INDEX:index}},
    {command:'npm.cmd run dev -- --strictPort',cwd:'apps/web',url:'http://127.0.0.1:5173',reuseExistingServer:false,timeout:30000},
  ],
});

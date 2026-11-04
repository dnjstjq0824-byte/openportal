import {defineConfig} from '@playwright/test';
import {existsSync} from 'node:fs';
import {resolve} from 'node:path';
const index=process.env.FOUNDATION_INDEX||`openscanner_foundation_e2e_${Date.now()}`;
process.env.FOUNDATION_INDEX=index;
const chrome='C:/Program Files/Google/Chrome/Application/chrome.exe';
export default defineConfig({
  testDir:'tests',testMatch:'foundation.e2e.spec.mjs',workers:1,timeout:45000,globalTimeout:240000,reporter:'list',
  use:{baseURL:'http://127.0.0.1:5175',browserName:'chromium',locale:'en-US',trace:'off',screenshot:'off',launchOptions:{executablePath:process.env.CHROME_PATH||(existsSync(chrome)?chrome:undefined)}},
  projects:[{name:'desktop',use:{viewport:{width:1440,height:1000}}},{name:'mobile-emulation',use:{viewport:{width:390,height:844},isMobile:true,hasTouch:true}}],
  webServer:[
    {command:'go run ./cmd/api',cwd:'backend',url:'http://127.0.0.1:8084/healthz',reuseExistingServer:false,timeout:60000,env:{PORT:'8084',OPENSEARCH_URL:'http://127.0.0.1:9200',OPENSEARCH_INDEX:index,GOCACHE:resolve('work/go-cache')}},
    {command:`${process.platform==='win32'?'npm.cmd':'npm'} run dev -- --port 5175 --strictPort`,cwd:'apps/web',url:'http://127.0.0.1:5175',reuseExistingServer:false,env:{GATEWAY_PROXY_TARGET:'http://127.0.0.1:8094'}},
  ],
});

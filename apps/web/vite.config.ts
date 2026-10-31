import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
export default defineConfig({ plugins: [vue()], server: { port: 5173, proxy: { '/api/v2': process.env.GATEWAY_PROXY_TARGET || 'http://127.0.0.1:8090' } } });

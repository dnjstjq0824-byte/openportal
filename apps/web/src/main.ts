import { createPinia } from 'pinia';
import { createApp } from 'vue';
import App from './App.vue';
import './portal.css';
import { i18n } from './i18n/index';
import router from './router';

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(i18n);
app.mount('#app');

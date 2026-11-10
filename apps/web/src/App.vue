<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue';
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router';
import BrandMark from './BrandMark.vue';
import ConfirmDialog from './ConfirmDialog.vue';
import SearchBox from './SearchBox.vue';
import { APIError } from './client';
import type { Locale } from './i18n';
import { coreMessages, folderLabelCopy } from './i18n/index';
import { AUTH_PAGES, cancelPendingRoute, confirmPendingRoute, pendingRoute } from './router';
import { useSearchStore } from './stores/search';
import { useSessionStore } from './stores/session';

const session = useSessionStore();
const search = useSearchStore();
const route = useRoute();
const router = useRouter();
const t = coreMessages;
const folders = folderLabelCopy;

const isAuthRoute = computed(() => (AUTH_PAGES as readonly string[]).includes(String(route.name)));
const mainClass = computed(() => ({
  'home-main': route.name === 'home',
  'mail-main': route.name === 'mail',
  'search-main': route.name === 'search',
  'auth-main': isAuthRoute.value,
}));

function changeLanguage(event: Event): void {
  const value = (event.target as HTMLSelectElement).value as Locale;
  const rest = route.path.replace(/^\/(ko|th|en)/, '') || '';
  void router.push({ path: `/${value}${rest}`, query: route.query, hash: route.hash });
}

function submitHeaderSearch(): void {
  if (!search.query.trim()) return;
  session.saveRecentSearch(search.query.trim());
  const params = new URLSearchParams({ q: search.query.trim() });
  if (session.safeSearch !== 'moderate') params.set('safe_search', session.safeSearch);
  void router.push(`/${session.locale}/search?${params}`).catch(() => {});
}

async function doLogout(): Promise<void> {
  try {
    await session.logout();
    await router.push(`/${session.locale}`);
  } catch {
    // The header has no alert surface; a failed logout keeps the session.
  }
}

function logout(): void {
  if (session.mailDraftDirty && route.name === 'mail') {
    pendingRoute.value = { action: () => void doLogout() };
    return;
  }
  void doLogout();
}

let refreshTimer: ReturnType<typeof setInterval> | undefined;

onMounted(() => {
  session.initTheme();
  refreshTimer = setInterval(async () => {
    if (session.user) {
      try {
        await session.refreshAuth();
      } catch (e) {
        if (e instanceof APIError && e.status === 401) {
          session.expire();
          void router.replace({ path: `/${session.locale}/login`, query: { returnTo: route.path } });
        }
      }
    }
  }, 15 * 60 * 1000);
});

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer);
});
</script>
<template>
  <a class="skip-link" href="#main-content">{{ t.skip }}</a>
  <div class="portal">
    <header class="site-header" :class="{ 'serp-header': route.name === 'search' }">
      <RouterLink class="brand" :to="`/${session.locale}`" aria-label="OpenScanner">
        <BrandMark />
        <span class="brand-word">Open<b>Scanner</b></span>
      </RouterLink>
      <div v-if="route.name === 'search'" class="header-search">
        <SearchBox id="header-query" v-model="search.query" :locale="session.locale" :recent="session.recentSearches" :busy="search.busy" @submit="submitHeaderSearch" @remove-recent="session.removeRecent($event)" />
      </div>
      <nav class="header-nav" :aria-label="t.services">
        <details class="services-menu">
          <summary>{{ t.services }}<svg class="chevron" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m4 6 4 4 4-4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg></summary>
          <div class="services-popover">
            <RouterLink :to="`/${session.locale}/mail`">
              <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" /></svg>
              <span>{{ t.mail }}</span>
            </RouterLink>
          </div>
        </details>
        <label class="language-control"><span class="sr-only">{{ t.language }}</span><select :aria-label="t.language" :value="session.locale" @change="changeLanguage"><option value="ko">한국어</option><option value="th">ไทย</option><option value="en">English</option></select></label>
        <label class="theme-control"><span class="sr-only">{{ t.theme }}</span><select :aria-label="t.theme" v-model="session.theme"><option value="system">{{ t.system }}</option><option value="light">{{ t.light }}</option><option value="dark">{{ t.dark }}</option></select></label>
        <details v-if="session.user" class="account-menu">
          <summary class="account-trigger"><span class="avatar" aria-hidden="true">{{ session.user.display_name?.slice(0, 1) }}</span><span class="sr-only">{{ t.account }}</span></summary>
          <div class="services-popover">
            <p class="account-email">{{ session.user.email }}</p>
            <RouterLink :to="`/${session.locale}/account`">{{ t.account }}</RouterLink>
            <RouterLink v-if="session.user.is_admin" :to="`/${session.locale}/admin`">{{ t.admin }}</RouterLink>
            <button @click="logout">{{ t.logout }}</button>
          </div>
        </details>
        <RouterLink v-else-if="!isAuthRoute" class="button primary signin" :to="{ path: `/${session.locale}/login`, query: { returnTo: route.fullPath } }">{{ t.login }}</RouterLink>
      </nav>
    </header>

    <main id="main-content" :class="mainClass">
      <RouterView />
    </main>

    <footer class="site-footer">
      <span class="footer-brand"><BrandMark /><span>© OpenScanner. All rights reserved.</span><span class="footer-divider">/</span><span>{{ t.slogan }}</span></span>
      <nav :aria-label="t.about">
        <RouterLink :to="`/${session.locale}/about`">{{ t.about }}</RouterLink>
        <RouterLink :to="`/${session.locale}/store`">{{ t.store }}</RouterLink>
        <RouterLink :to="`/${session.locale}/advertising`">{{ t.advertising }}</RouterLink>
        <RouterLink :to="`/${session.locale}/business`">{{ t.business }}</RouterLink>
        <RouterLink :to="`/${session.locale}/privacy`">{{ t.privacy }}</RouterLink>
        <RouterLink :to="`/${session.locale}/terms`">{{ t.terms }}</RouterLink>
        <RouterLink :to="`/${session.locale}/settings`">{{ t.settings }}</RouterLink>
      </nav>
    </footer>
    <ConfirmDialog v-if="pendingRoute" :title="t.discardTitle" :body="folders.discard" :confirm="t.discardConfirm" :cancel="t.cancel" @confirm="confirmPendingRoute" @cancel="cancelPendingRoute" />
  </div>
</template>

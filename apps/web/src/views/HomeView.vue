<script setup lang="ts">
import { computed, ref } from 'vue';
import { RouterLink, useRouter } from 'vue-router';
import BrandMark from '../BrandMark.vue';
import ScanArt from '../ScanArt.vue';
import SearchBox from '../SearchBox.vue';
import { coreMessages, designCopy } from '../i18n/index';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const router = useRouter();
const t = coreMessages;
const copy = designCopy;
const search = ref('');

const showOnboarding = computed(() => session.ready && !session.onboarded);

function dismissOnboarding(focusSearch = false): void {
  session.dismissOnboarding();
  if (focusSearch) document.getElementById('home-query')?.focus();
}

function submitSearch(): void {
  if (!search.value.trim()) return;
  session.saveRecentSearch(search.value.trim());
  const params = new URLSearchParams({ q: search.value.trim() });
  if (session.safeSearch !== 'moderate') params.set('safe_search', session.safeSearch);
  void router.push(`/${session.locale}/search?${params}`);
}
</script>
<template>
  <section class="home-grid">
    <div v-if="showOnboarding" class="onboard-banner" role="note" aria-labelledby="onboard-title">
      <div class="onboard-text">
        <h2 id="onboard-title">{{ t.onboardTitle }}</h2>
        <p>{{ t.onboardBody }}</p>
      </div>
      <div class="actions">
        <RouterLink class="text-button" :to="`/${session.locale}/signup`" @click="dismissOnboarding()">{{ t.signup }}</RouterLink>
        <button type="button" class="button primary" @click="dismissOnboarding(true)">{{ t.onboardCta }}</button>
        <button type="button" class="icon-button" :aria-label="t.onboardClose" @click="dismissOnboarding()">
          <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg>
        </button>
      </div>
    </div>
    <div class="hero">
      <h1 class="home-brand">
        <BrandMark class="home-mark" />
        <span class="home-word">Open<b>Scanner</b></span>
      </h1>
      <SearchBox id="home-query" v-model="search" :locale="session.locale" :recent="session.recentSearches" @submit="submitSearch()" @remove-recent="session.removeRecent($event)" />
      <div v-if="session.recentOptIn" class="recent-searches">
        <p class="eyebrow">{{ copy.recent }}</p>
        <p v-if="!session.recentSearches.length" class="muted">{{ copy.noHistory }}</p>
        <button v-for="recent in session.recentSearches" :key="recent" class="chip" @click="search = recent; submitSearch()">
          <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5V12l3 1.8" /></svg>
          <span>{{ recent }}</span>
        </button>
      </div>
    </div>
    <aside class="home-aside">
      <ScanArt caption="SEARCH · DISCOVER · DO" />
      <h2>{{ copy.homeAside }}</h2>
      <RouterLink class="service-card" :to="`/${session.locale}/mail`">
        <span class="service-icon" aria-hidden="true">
          <svg class="icon-svg" viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" /></svg>
        </span>
        <span><strong>{{ t.mail }}</strong><small>{{ copy.mailDescription }}</small></span>
        <span aria-hidden="true"><svg class="icon-svg" viewBox="0 0 24 24"><path d="M7 17L17 7M9 7h8v8" /></svg></span>
      </RouterLink>
      <RouterLink class="service-card" :to="`/${session.locale}/settings`">
        <span class="service-icon" aria-hidden="true">
          <svg class="icon-svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9c.26.6.77 1.02 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>
        </span>
        <span><strong>{{ t.settings }}</strong><small>{{ copy.settingsDescription }}</small></span>
        <span aria-hidden="true"><svg class="icon-svg" viewBox="0 0 24 24"><path d="M7 17L17 7M9 7h8v8" /></svg></span>
      </RouterLink>
      <RouterLink class="service-card" :to="`/${session.locale}/privacy`">
        <span class="service-icon" aria-hidden="true">
          <svg class="icon-svg" viewBox="0 0 24 24"><path d="M12 3l7 3v6c0 4.5-3 7.8-7 9-4-1.2-7-4.5-7-9V6l7-3z" /></svg>
        </span>
        <span><strong>{{ t.privacy }}</strong><small>{{ copy.privacyDescription }}</small></span>
        <span aria-hidden="true"><svg class="icon-svg" viewBox="0 0 24 24"><path d="M7 17L17 7M9 7h8v8" /></svg></span>
      </RouterLink>
    </aside>
  </section>
</template>

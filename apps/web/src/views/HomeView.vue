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
        <button type="button" class="icon-button" :aria-label="t.onboardClose" @click="dismissOnboarding()"><span aria-hidden="true">×</span></button>
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
        <button v-for="recent in session.recentSearches" :key="recent" class="chip" @click="search = recent; submitSearch()">↶ {{ recent }}</button>
      </div>
    </div>
    <aside class="home-aside">
      <ScanArt caption="SEARCH · DISCOVER · DO" />
      <h2>{{ copy.homeAside }}</h2>
      <RouterLink class="service-card" :to="`/${session.locale}/mail`"><span class="service-icon" aria-hidden="true">✉</span><span><strong>{{ t.mail }}</strong><small>{{ copy.mailDescription }}</small></span><span aria-hidden="true">↗</span></RouterLink>
      <RouterLink class="service-card" :to="`/${session.locale}/settings`"><span class="service-icon" aria-hidden="true">☷</span><span><strong>{{ t.settings }}</strong><small>{{ copy.settingsDescription }}</small></span><span aria-hidden="true">↗</span></RouterLink>
      <RouterLink class="service-card" :to="`/${session.locale}/privacy`"><span class="service-icon" aria-hidden="true">◇</span><span><strong>{{ t.privacy }}</strong><small>{{ copy.privacyDescription }}</small></span><span aria-hidden="true">↗</span></RouterLink>
    </aside>
  </section>
</template>

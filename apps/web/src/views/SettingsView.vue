<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';
import type { Locale } from '../i18n';
import { coreMessages, designCopy } from '../i18n/index';
import { useSessionStore, type SafeSearch } from '../stores/session';

const session = useSessionStore();
const route = useRoute();
const router = useRouter();
const t = coreMessages;
const copy = designCopy;

function changeLanguage(event: Event): void {
  const value = (event.target as HTMLSelectElement).value as Locale;
  const rest = route.path.replace(/^\/(ko|th|en)/, '') || '';
  void router.push({ path: `/${value}${rest}`, query: route.query, hash: route.hash });
}

function changeSafeSearch(event: Event): void {
  const value = (event.target as HTMLSelectElement).value;
  if (!['strict', 'moderate', 'off'].includes(value)) return;
  session.setSafeSearch(value as SafeSearch);
}
</script>
<template>
  <section class="settings-page">
    <div class="page-heading">
      <p class="eyebrow">OPENSCANNER / {{ t.settings }}</p>
      <h1>{{ t.settings }}</h1>
      <p>{{ copy.settingsIntro }}</p>
    </div>
    <div class="settings-card">
      <label class="setting-row">
        <span><strong>{{ t.language }}</strong></span>
        <select :aria-label="t.language" :value="session.locale" @change="changeLanguage">
          <option value="ko">한국어</option>
          <option value="th">ไทย</option>
          <option value="en">English</option>
        </select>
      </label>
      <label class="setting-row">
        <span><strong>{{ t.theme }}</strong></span>
        <select :aria-label="t.theme" v-model="session.theme">
          <option value="system">{{ t.system }}</option>
          <option value="light">{{ t.light }}</option>
          <option value="dark">{{ t.dark }}</option>
        </select>
      </label>
      <label class="setting-row">
        <strong>{{ t.safeSearch }}</strong>
        <select :aria-label="t.safeSearch" :value="session.safeSearch" @change="changeSafeSearch">
          <option value="moderate">{{ t.safeSearchModerate }}</option>
          <option value="strict">{{ t.safeSearchStrict }}</option>
          <option value="off">{{ t.safeSearchOff }}</option>
        </select>
      </label>
      <label class="setting-row history-setting">
        <span><strong>{{ copy.rememberSearches }}</strong><small>{{ copy.historyHint }}</small></span>
        <input v-model="session.recentOptIn" type="checkbox" />
      </label>
      <button class="button" @click="session.clearHistory()">{{ copy.clearHistory }}</button>
      <p class="source-note">{{ copy.preferencesSaved }}</p>
    </div>
  </section>
</template>

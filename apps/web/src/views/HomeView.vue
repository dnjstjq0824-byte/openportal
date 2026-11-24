<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import BrandMark from '../BrandMark.vue';
import SearchBox from '../SearchBox.vue';
import { designCopy } from '../i18n/index';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const router = useRouter();
const copy = designCopy;
const search = ref('');

function submitSearch(): void {
  if (!search.value.trim()) return;
  session.saveRecentSearch(search.value.trim());
  const params = new URLSearchParams({ q: search.value.trim() });
  if (session.safeSearch !== 'moderate') params.set('safe_search', session.safeSearch);
  void router.push(`/${session.locale}/search?${params}`);
}
</script>
<template>
  <section class="home-stage">
    <div class="hero">
      <h1 class="home-brand">
        <BrandMark class="home-mark" :size="72" />
        <span class="home-word">Open<b>Scanner</b></span>
      </h1>
      <SearchBox id="home-query" v-model="search" :locale="session.locale" :recent="session.recentSearches" @submit="submitSearch()" @remove-recent="session.removeRecent($event)" />
      <div v-if="session.recentOptIn" class="hot-list">
        <p class="hot-title">{{ copy.recent }}</p>
        <p v-if="!session.recentSearches.length" class="muted">{{ copy.noHistory }}</p>
        <ol v-else class="hot-ranks">
          <li v-for="(recent, index) in session.recentSearches" :key="recent">
            <button type="button" class="hot-item" @click="search = recent; submitSearch()">
              <span class="hot-rank" :class="{ top: index < 3 }">{{ index + 1 }}</span>
              <span class="hot-text">{{ recent }}</span>
            </button>
          </li>
        </ol>
      </div>
    </div>
  </section>
</template>

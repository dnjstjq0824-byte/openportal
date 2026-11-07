<script setup lang="ts">
import { RouterLink } from 'vue-router';
import { coreMessages } from '../i18n/index';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const t = coreMessages;
</script>
<template>
  <section v-if="!session.ready" class="search-state" role="status">{{ t.loading }}</section>
  <section v-else class="search-state not-found">
    <p class="eyebrow">404 / OPENSCANNER</p>
    <h1>{{ t.notFound }}</h1>
    <RouterLink class="button primary" :to="`/${session.locale}`">{{ t.backHome }}</RouterLink>
  </section>
</template>

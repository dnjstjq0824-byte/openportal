<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { coreMessages, designCopy } from '../i18n/index';

const route = useRoute();
const t = coreMessages;
const copy = designCopy;

const page = computed(() => String(route.params.page ?? ''));
const titles = computed(
  () =>
    ({
      about: t.value.about,
      store: t.value.store,
      advertising: t.value.advertising,
      business: t.value.business,
      howsearchworks: t.value.howSearchWorks,
      privacy: t.value.privacy,
      terms: t.value.terms,
    }) as Record<string, string>
);
const policyBody = computed(() => {
  if (page.value === 'about') return copy.value.aboutBody;
  if (page.value === 'howsearchworks') return copy.value.howBody;
  return (t.value as Record<string, string>)[`${page.value}Body`] || '';
});
</script>
<template>
  <section class="policy-page">
    <p class="eyebrow">OPENSCANNER</p>
    <h1>{{ titles[page] }}</h1>
    <template v-if="page === 'privacy' || page === 'terms'">
      <p class="notice">{{ t.policyDraft }}</p>
      <p>{{ page === 'privacy' ? t.privacyBody : t.termsBody }}</p>
    </template>
    <p v-else>{{ policyBody }}</p>
  </section>
</template>

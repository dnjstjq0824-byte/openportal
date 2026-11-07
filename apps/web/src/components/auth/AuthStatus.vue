<script setup lang="ts">
import { RouterLink } from 'vue-router';
import ScanState from '../../ScanState.vue';
import { coreMessages, designCopy, serpCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';

defineProps<{ serverDown: boolean; notice: string; error: string; errorText: string }>();
const emit = defineEmits<{ retry: [] }>();
const session = useSessionStore();
const t = coreMessages;
const copy = designCopy;
const serp = serpCopy;
</script>
<template>
  <p v-if="!session.ready" role="status">{{ t.loading }}</p>
  <div v-else-if="serverDown" class="serp-state" role="alert">
    <ScanState kind="unavailable" />
    <h2>{{ t.serverErrorTitle }}</h2>
    <p>{{ t.serverErrorBody }}</p>
    <div class="actions">
      <button type="button" class="button primary" @click="emit('retry')">{{ t.retry }}</button>
      <RouterLink class="button" :to="`/${session.locale}`">{{ t.backHome }}</RouterLink>
    </div>
    <p v-if="error" class="error-code">{{ serp.errorCode }}: <code>{{ error }}</code></p>
  </div>
  <slot v-else />
  <p v-if="notice" class="notice" role="status">{{ notice }}</p>
  <p v-if="error" class="alert" role="alert">{{ errorText }}</p>
  <p v-else-if="session.sessionUnavailable" class="source-note">{{ copy.authUnavailable }}</p>
</template>

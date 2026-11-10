<script setup lang="ts">
import { onMounted, onUnmounted, ref, useId } from 'vue';
import { APIError, mutate } from './client';
import { serpCopy } from './i18n/index';
import { useSessionStore } from './stores/session';

const props = defineProps<{ url: string; documentId: string }>();
const emit = defineEmits<{ close: []; submitted: [] }>();
const session = useSessionStore();
const serp = serpCopy;

const categories = [
  { value: 'spam', label: () => serp.value.reportSpam },
  { value: 'malware', label: () => serp.value.reportMalware },
  { value: 'phishing', label: () => serp.value.reportPhishing },
  { value: 'adult', label: () => serp.value.reportAdult },
  { value: 'copyright', label: () => serp.value.reportCopyright },
  { value: 'privacy', label: () => serp.value.reportPrivacy },
  { value: 'other', label: () => serp.value.reportOther },
] as const;

const category = ref<string>('spam');
const details = ref('');
const state = ref<'idle' | 'sending' | 'done' | 'error' | 'login'>('idle');
const titleId = `report-title-${useId()}`;
const firstField = ref<HTMLElement | null>(null);

function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.stopPropagation(); emit('close'); }
}
onMounted(() => { document.addEventListener('keydown', onKey, true); firstField.value?.focus(); });
onUnmounted(() => document.removeEventListener('keydown', onKey, true));

async function submit(): Promise<void> {
  if (state.value === 'sending' || state.value === 'done') return;
  state.value = 'sending';
  try {
    await mutate('/report', { document_id: props.documentId, url: props.url, category: category.value, details: details.value.trim() });
    state.value = 'done';
    emit('submitted');
  } catch (e) {
    state.value = e instanceof APIError && (e.code === 'SESSION_REQUIRED' || e.status === 401) ? 'login' : 'error';
  }
}
</script>
<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card" role="dialog" aria-modal="true" :aria-labelledby="titleId">
      <h2 :id="titleId">{{ serp.reportTitle }}</h2>
      <p class="report-lead">{{ serp.reportLead }}</p>
      <p v-if="state === 'done'" class="notice" role="status">{{ serp.reportDone }}</p>
      <p v-else-if="state === 'login' || !session.user" class="notice" role="alert">{{ serp.reportLogin }}</p>
      <p v-else-if="state === 'error'" class="notice" role="alert">{{ serp.reportFailed }}</p>
      <form v-if="state !== 'done' && session.user" @submit.prevent="submit">
        <label class="report-field">
          <span>{{ serp.reportCategory }}</span>
          <select ref="firstField" v-model="category" :disabled="state === 'sending'">
            <option v-for="c in categories" :key="c.value" :value="c.value">{{ c.label() }}</option>
          </select>
        </label>
        <label class="report-field">
          <span>{{ serp.reportDetails }}</span>
          <textarea v-model="details" rows="3" maxlength="2000" :disabled="state === 'sending'" :placeholder="serp.reportDetailsHint"></textarea>
        </label>
        <div class="actions">
          <button type="button" class="button" :disabled="state === 'sending'" @click="emit('close')">{{ serp.reportCancel }}</button>
          <button type="submit" class="button primary" :disabled="state === 'sending'">{{ serp.reportSubmit }}</button>
        </div>
      </form>
      <div v-else class="actions">
        <button type="button" class="button primary" @click="emit('close')">{{ serp.reportCancel }}</button>
      </div>
    </div>
  </div>
</template>

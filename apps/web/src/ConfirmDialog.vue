<script setup lang="ts">
import { onMounted, onUnmounted, ref, useId } from 'vue';
defineProps<{ title: string; body: string; confirm: string; cancel: string }>();
const emit = defineEmits<{ confirm: []; cancel: [] }>();
const titleId = `confirm-title-${useId()}`;
const bodyId = `confirm-body-${useId()}`;
const cancelBtn = ref<HTMLButtonElement | null>(null);
function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') { event.stopPropagation(); emit('cancel'); return; }
  if (event.key !== 'Tab') return;
  const root = cancelBtn.value?.closest('.modal-card');
  const items = root ? Array.from(root.querySelectorAll<HTMLElement>('button:not([disabled]), a[href]')) : [];
  if (!items.length) return;
  const first = items[0];
  const last = items[items.length - 1];
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
}
onMounted(() => { document.addEventListener('keydown', onKey, true); cancelBtn.value?.focus(); });
onUnmounted(() => document.removeEventListener('keydown', onKey, true));
</script>
<template>
  <div class="modal-overlay" @click.self="emit('cancel')">
    <div class="modal-card" role="alertdialog" aria-modal="true" :aria-labelledby="titleId" :aria-describedby="bodyId">
      <h2 :id="titleId">{{ title }}</h2>
      <p :id="bodyId">{{ body }}</p>
      <div class="actions">
        <button ref="cancelBtn" type="button" class="button" @click="emit('cancel')">{{ cancel }}</button>
        <button type="button" class="button danger" @click="emit('confirm')">{{ confirm }}</button>
      </div>
    </div>
  </div>
</template>

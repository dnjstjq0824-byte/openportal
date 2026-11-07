<script setup lang="ts">
import { onMounted, onUnmounted, useId } from 'vue';
defineProps<{ title: string }>();
const emit = defineEmits<{ close: [] }>();
const titleId = `base-modal-${useId()}`;
function onKey(event: KeyboardEvent) {
  if (event.key === 'Escape') emit('close');
}
onMounted(() => document.addEventListener('keydown', onKey));
onUnmounted(() => document.removeEventListener('keydown', onKey));
</script>
<template>
  <div class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card" role="dialog" aria-modal="true" :aria-labelledby="titleId">
      <h2 :id="titleId">{{ title }}</h2>
      <slot />
      <div class="actions"><slot name="actions" /></div>
    </div>
  </div>
</template>

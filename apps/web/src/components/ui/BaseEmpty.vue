<script setup lang="ts">
defineProps<{ title: string; body?: string }>();
</script>
<template>
  <div class="base-empty">
    <slot name="art" />
    <h2>{{ title }}</h2>
    <p v-if="body">{{ body }}</p>
    <div class="actions"><slot name="actions" /></div>
  </div>
</template>
<style scoped>
.base-empty { text-align: center; padding: 40px 20px; }
.base-empty h2 { font-size: 1.15rem; margin: 12px 0 6px; }
.base-empty p { color: var(--muted); margin: 0 0 16px; }
.base-empty .actions { justify-content: center; }
</style>

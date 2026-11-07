<script setup lang="ts">
withDefaults(defineProps<{ lines?: number; label?: string }>(), { lines: 3, label: 'Loading…' });
</script>
<template>
  <div class="base-skeleton" role="status" :aria-label="label">
    <span v-for="n in lines" :key="n" class="base-skeleton-line" :class="{ short: n === lines }" aria-hidden="true"></span>
  </div>
</template>
<style scoped>
.base-skeleton { display: flex; flex-direction: column; gap: 10px; padding: 8px 0; }
.base-skeleton-line { display: block; height: 14px; border-radius: 7px; background: linear-gradient(90deg, var(--line) 25%, var(--bg) 50%, var(--line) 75%); background-size: 200% 100%; animation: base-shimmer 1.4s ease infinite; }
.base-skeleton-line.short { width: 55%; }
@keyframes base-shimmer { to { background-position: -200% 0; } }
@media (prefers-reduced-motion: reduce) { .base-skeleton-line { animation: none; } }
</style>

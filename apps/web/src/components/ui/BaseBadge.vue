<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'default' | 'ok' | 'warn' | 'error' | 'info' }>(), { tone: 'default' });
</script>
<template>
  <span :class="['status-pill', `tone-${tone}`]"><slot /></span>
</template>
<style scoped>
.tone-ok { background: var(--ok-bg); color: var(--ok); border-color: transparent; }
.tone-warn { background: var(--warn-bg); color: var(--warn); border-color: transparent; }
.tone-error { background: var(--error-bg); color: var(--error); border-color: transparent; }
.tone-info { background: var(--primary-soft); color: var(--primary); border-color: transparent; }
</style>

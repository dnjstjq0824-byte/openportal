<script setup lang="ts">
defineProps<{ caption?: string }>();
</script>
<template>
  <div class="table-scroll">
    <table class="base-table">
      <caption v-if="caption" class="sr-only">{{ caption }}</caption>
      <thead><slot name="head" /></thead>
      <tbody><slot /></tbody>
    </table>
  </div>
</template>
<style scoped>
.table-scroll { overflow-x: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); }
.base-table { width: 100%; border-collapse: collapse; font-size: .9rem; }
.base-table :deep(th), .base-table :deep(td) { text-align: left; padding: 10px 14px; border-bottom: 1px solid var(--line); white-space: nowrap; }
.base-table :deep(thead th) { background: var(--bg); font-size: .78rem; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); }
.base-table :deep(tbody tr:last-child td) { border-bottom: 0; }
</style>

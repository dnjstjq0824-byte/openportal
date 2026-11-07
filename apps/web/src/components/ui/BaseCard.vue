<script setup lang="ts">
defineProps<{ title?: string }>();
</script>
<template>
  <section class="base-card">
    <h2 v-if="title" class="base-card-title">{{ title }}</h2>
    <slot />
  </section>
</template>
<style scoped>
.base-card { background: var(--surface); border: 1px solid var(--line); border-radius: var(--radius); box-shadow: var(--shadow); padding: 20px; }
.base-card-title { font-size: 1rem; margin: 0 0 12px; }
</style>

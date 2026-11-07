<script setup lang="ts">
export interface TabItem { value: string; label: string; }
defineProps<{ modelValue: string; tabs: TabItem[]; label: string }>();
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();
</script>
<template>
  <div class="base-tabs" role="tablist" :aria-label="label">
    <button v-for="tab in tabs" :key="tab.value" type="button" role="tab" :aria-selected="modelValue === tab.value" :class="{ active: modelValue === tab.value }" @click="emit('update:modelValue', tab.value)">{{ tab.label }}</button>
  </div>
</template>
<style scoped>
.base-tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--line); }
.base-tabs button { min-height: 44px; padding: 0 16px; border: 0; border-bottom: 2px solid transparent; background: none; color: var(--muted); font-weight: 600; cursor: pointer; }
.base-tabs button.active { color: var(--primary); border-bottom-color: var(--primary); }
</style>

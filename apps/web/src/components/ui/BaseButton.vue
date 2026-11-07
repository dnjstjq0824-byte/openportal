<script setup lang="ts">
withDefaults(defineProps<{ variant?: 'primary' | 'default' | 'danger' | 'text'; loading?: boolean; disabled?: boolean; type?: 'button' | 'submit' | 'reset'; block?: boolean }>(), { variant: 'default', type: 'button' });
</script>
<template>
  <button :type="type" :class="['button', variant === 'default' ? '' : variant, { 'full-width': block }]" :disabled="disabled || loading" :aria-busy="loading || undefined">
    <span v-if="loading" class="base-btn-spinner" aria-hidden="true"></span>
    <slot />
  </button>
</template>
<style scoped>
.base-btn-spinner { width: 14px; height: 14px; border-radius: 50%; border: 2px solid currentColor; border-top-color: transparent; animation: base-spin .7s linear infinite; }
@keyframes base-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce) { .base-btn-spinner { animation: none; } }
</style>

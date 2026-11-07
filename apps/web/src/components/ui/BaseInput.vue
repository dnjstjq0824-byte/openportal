<script setup lang="ts">
import { useId } from 'vue';
const props = withDefaults(defineProps<{ modelValue: string; label: string; type?: string; placeholder?: string; hint?: string; error?: string; required?: boolean; maxlength?: number; autocomplete?: string }>(), { type: 'text' });
const emit = defineEmits<{ 'update:modelValue': [value: string] }>();
const id = `base-input-${useId()}`;
const hintId = `${id}-hint`;
</script>
<template>
  <div class="form-group">
    <label :for="id">{{ label }}</label>
    <input :id="id" :value="modelValue" :type="type" :placeholder="placeholder" :required="required" :maxlength="maxlength" :autocomplete="autocomplete" :aria-invalid="error ? 'true' : undefined" :aria-describedby="hint || error ? hintId : undefined" @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)" />
    <small v-if="error" :id="hintId" class="base-input-error" role="alert">{{ error }}</small>
    <small v-else-if="hint" :id="hintId" class="muted">{{ hint }}</small>
  </div>
</template>
<style scoped>
.form-group { display: flex; flex-direction: column; gap: 6px; margin: 0 0 14px; }
.form-group label { font-weight: 600; font-size: .88rem; }
.form-group input { min-height: 44px; padding: 0 12px; border: 1px solid var(--line-strong); border-radius: 8px; background: var(--surface); }
.form-group input[aria-invalid="true"] { border-color: var(--error); }
.base-input-error { color: var(--error); }
</style>

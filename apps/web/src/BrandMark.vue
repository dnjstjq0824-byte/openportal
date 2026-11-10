<script setup lang="ts">
const props = withDefaults(defineProps<{ size?: number }>(), { size: 32 })
</script>

<template>
  <svg
    class="brand-mark"
    :width="props.size"
    :height="props.size"
    viewBox="0 0 36 36"
    fill="none"
    aria-hidden="true"
  >
    <!-- Aperture Bracket Frame [ ] -->
    <path
      d="M11 6.5H7.5C6.39543 6.5 5.5 7.39543 5.5 8.5V27.5C5.5 28.6046 6.39543 29.5 7.5 29.5H11"
      stroke="var(--text-primary, #0F172A)"
      stroke-width="2.4"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <path
      d="M25 6.5H28.5C29.6046 6.5 30.5 7.39543 30.5 8.5V27.5C30.5 28.6046 29.6046 29.5 28.5 29.5H25"
      stroke="var(--text-primary, #0F172A)"
      stroke-width="2.4"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
    <!-- Precision Scanner Focus Lens & Crosshair -->
    <circle
      cx="17.2"
      cy="17.2"
      r="6.2"
      stroke="var(--brand-primary, #1D4ED8)"
      stroke-width="2.3"
    />
    <circle
      cx="17.2"
      cy="17.2"
      r="1.75"
      fill="var(--brand-primary, #1D4ED8)"
    />
    <path
      d="M17.2 8.6V11M17.2 23.4V25.8M8.6 17.2H11M23.4 17.2H25.8"
      stroke="var(--brand-primary, #1D4ED8)"
      stroke-width="1.65"
      stroke-linecap="round"
    />
    <path
      d="M21.8 21.8L26.2 26.2"
      stroke="var(--brand-primary, #1D4ED8)"
      stroke-width="2.5"
      stroke-linecap="round"
    />
  </svg>
</template>

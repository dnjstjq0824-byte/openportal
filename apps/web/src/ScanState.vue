<script setup lang="ts">
// Scenario illustration for search states: a scanner focus frame with a state-specific glyph.
defineProps<{ kind: 'empty' | 'none' | 'end' | 'offline' | 'rate' | 'timeout' | 'invalid' | 'unavailable' }>();
</script>
<template>
  <svg class="scan-state" :class="`kind-${kind}`" viewBox="0 0 120 96" fill="none" aria-hidden="true" focusable="false">
    <rect x="4" y="4" width="112" height="88" rx="22" class="tile" />
    <g class="frame" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round">
      <path d="M26 30v-6a6 6 0 0 1 6-6h6" /><path d="M82 18h6a6 6 0 0 1 6 6v6" /><path d="M94 66v6a6 6 0 0 1-6 6h-6" /><path d="M38 78h-6a6 6 0 0 1-6-6v-6" />
    </g>
    <g class="glyph" stroke-width="3.6" stroke-linecap="round" stroke-linejoin="round">
      <template v-if="kind==='empty'"><path d="M46 48h22" class="beam" /><path d="M74 38v20" class="cursor" /></template>
      <template v-else-if="kind==='none'"><circle cx="56" cy="46" r="13" /><path d="m66 56 9 9" /><path d="M51 46h10" /></template>
      <template v-else-if="kind==='end'"><path d="M48 66V32" /><path d="M48 33h22l-5 7 5 7H48" /></template>
      <template v-else-if="kind==='offline'"><path d="M40 44a28 28 0 0 1 40 0" /><path d="M47 52a17 17 0 0 1 26 0" /><circle cx="60" cy="61" r="2.4" class="fill" /><path d="m42 30 36 36" class="cut" /></template>
      <template v-else-if="kind==='rate'"><path d="M48 30h24M48 66h24" /><path d="M51 30c0 10 18 12 18 18s-18 8-18 18M69 30c0 10-18 12-18 18s18 8 18 18" /></template>
      <template v-else-if="kind==='timeout'"><circle cx="60" cy="48" r="17" /><path d="M60 38v10l7 5" /></template>
      <template v-else-if="kind==='invalid'"><path d="M60 34v17" /><circle cx="60" cy="61" r="2.4" class="fill" /></template>
      <template v-else><path d="M47 51l-4 4a7 7 0 0 0 10 10l4-4" /><path d="M73 45l4-4a7 7 0 0 0-10-10l-4 4" /><path d="m54 42-3-5M66 54l3 5M48 47h-5M72 49h5" class="cut" /></template>
    </g>
  </svg>
</template>

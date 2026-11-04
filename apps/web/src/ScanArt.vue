<script setup lang="ts">
// Brand illustration matching the OpenScanner mark: a lens ring crossed by a scanning beam inside a focus frame.
defineProps<{ caption?: string }>();
</script>
<template>
  <div class="scan-art" aria-hidden="true">
    <span class="scan-grid"></span>
    <span class="scan-corner tl"></span><span class="scan-corner tr"></span><span class="scan-corner bl"></span><span class="scan-corner br"></span>
    <span class="scan-axis"></span>
    <span class="scan-lens"><span class="scan-lens-dot"></span></span>
    <span class="scan-beam"></span>
    <span v-if="caption" class="scan-caption">{{ caption }}</span>
  </div>
</template>

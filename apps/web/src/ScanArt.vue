<script setup lang="ts">
defineProps<{ caption?: string }>();
</script>
<template>
  <div class="scan-art" aria-hidden="true">
    <span class="scan-grid"></span>
    <span class="scan-corner tl"></span><span class="scan-corner tr"></span><span class="scan-corner bl"></span><span class="scan-corner br"></span>
    <span class="scan-orbit"></span><span class="scan-orbit second"></span>
    <span class="scan-core"></span>
    <span class="scan-beam"></span>
    <span v-if="caption" class="scan-caption">{{ caption }}</span>
  </div>
</template>

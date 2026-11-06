<script setup lang="ts">
// OpenScanner mark: a lens ring crossed by a scanning beam on a Search→Action gradient tile.
import { useId } from 'vue';
const gradient = `os-mark-${useId()}`;
</script>
<template>
  <svg class="brand-mark" viewBox="0 0 40 40" fill="none" aria-hidden="true" focusable="false">
    <defs>
      <linearGradient :id="gradient" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#1F6FEB" />
        <stop offset="1" stop-color="#1F6FEB" />
      </linearGradient>
    </defs>
    <rect width="40" height="40" rx="11" :fill="`url(#${gradient})`" />
    <circle cx="20" cy="20" r="9.5" stroke="#fff" stroke-width="4.2" />
    <path d="M5 20H35" stroke="#BCD6FF" stroke-width="3.2" stroke-linecap="round" />
    <circle cx="20" cy="20" r="2.6" fill="#fff" />
  </svg>
</template>

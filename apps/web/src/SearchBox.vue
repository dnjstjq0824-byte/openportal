<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { request } from './client';
import { messages, type Locale } from './i18n';
import { designMessages } from './design-copy';

const props = defineProps<{ modelValue: string; locale: Locale; id: string; recent: string[]; busy?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: string]; submit: []; removeRecent: [value: string] }>();

const t = computed(() => messages[props.locale]);
const copy = computed(() => designMessages[props.locale]);
const suggestions = ref<string[]>([]);
const open = ref(false);
const focused = ref(false);
const isComposing = ref(false);
const active = ref(-1);
const inputRef = ref<HTMLInputElement | null>(null);

let timer: ReturnType<typeof setTimeout> | undefined;
let closeTimer: ReturnType<typeof setTimeout> | undefined;
let controller: AbortController | undefined;
let sequence = 0;

const items = computed(() => {
  const unique = new Set<string>();
  const q = props.modelValue.trim().toLowerCase();
  return [
    ...props.recent.filter(x => !q || x.toLowerCase().includes(q)).map(text => ({ text, recent: true })),
    ...suggestions.value.map(text => ({ text, recent: false })),
  ]
    .filter(x => {
      if (unique.has(x.text)) return false;
      unique.add(x.text);
      return true;
    })
    .slice(0, 8);
});

function changed(event: Event): void {
  const value = (event.target as HTMLInputElement).value;
  emit('update:modelValue', value);
  clearTimeout(timer);
  controller?.abort();
  ++sequence;
  active.value = -1;
  open.value = true;
  suggestions.value = [];
  if (!value.trim()) return;
  const own = sequence;
  timer = setTimeout(async () => {
    const current = new AbortController();
    controller = current;
    try {
      const data = await request(`/suggest?${new URLSearchParams({ q: value.trim() })}`, {
        signal: AbortSignal.any([current.signal, AbortSignal.timeout(3000)]),
      });
      if (own === sequence && !current.signal.aborted && Array.isArray(data.suggestions)) {
        suggestions.value = data.suggestions.filter((x: unknown) => typeof x === 'string');
      }
    } catch {
      // Ignore aborted or transient suggest errors
    }
  }, 150);
}

function step(direction: number): void {
  open.value = true;
  if (!items.value.length) return;
  active.value = (active.value + direction + items.value.length) % items.value.length;
}

function choose(value: string): void {
  emit('update:modelValue', value);
  open.value = false;
  active.value = -1;
  emit('submit');
}

function submit(): void {
  if (isComposing.value) return;
  if (open.value && active.value >= 0 && items.value[active.value]) {
    choose(items.value[active.value].text);
  } else {
    open.value = false;
    emit('submit');
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (isComposing.value || event.isComposing || event.keyCode === 229) {
    return;
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault();
    step(1);
  } else if (event.key === 'ArrowUp') {
    event.preventDefault();
    step(-1);
  } else if (event.key === 'Escape') {
    open.value = false;
    active.value = -1;
  } else if (event.key === 'Enter' && open.value && active.value >= 0 && items.value[active.value]) {
    event.preventDefault();
    choose(items.value[active.value].text);
  }
}

function onGlobalKeydown(event: KeyboardEvent): void {
  if (event.defaultPrevented || isComposing.value) return;
  const target = event.target as HTMLElement | null;
  const isEditable = Boolean(
    target &&
      (target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.tagName === 'SELECT' ||
        target.isContentEditable)
  );
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault();
    inputRef.value?.focus();
    inputRef.value?.select();
    return;
  }
  if (!isEditable && !event.metaKey && !event.ctrlKey && !event.altKey && event.key === '/') {
    event.preventDefault();
    inputRef.value?.focus();
  }
}

function blur(): void {
  focused.value = false;
  closeTimer = setTimeout(() => {
    open.value = false;
  }, 120);
}

function focus(): void {
  clearTimeout(closeTimer);
  focused.value = true;
  open.value = true;
}

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown);
});

onUnmounted(() => {
  clearTimeout(timer);
  clearTimeout(closeTimer);
  controller?.abort();
  window.removeEventListener('keydown', onGlobalKeydown);
});
</script>
<template>
  <div class="search-control" :class="{ scanning: busy }">
    <form class="search-form" role="search" @submit.prevent="submit">
      <label class="sr-only" :for="id">{{ t.search }}</label>
      <svg class="search-symbol" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <circle cx="10.5" cy="10.5" r="6.5" />
        <path d="m16 16 4 4" stroke-linecap="round" />
      </svg>
      <input
        :id="id"
        ref="inputRef"
        :value="modelValue"
        type="search"
        :placeholder="t.searchPlaceholder"
        maxlength="512"
        enterkeyhint="search"
        autocomplete="off"
        autocapitalize="off"
        spellcheck="false"
        required
        :aria-expanded="open && items.length > 0"
        aria-autocomplete="list"
        :aria-controls="`${id}-suggestions`"
        :aria-activedescendant="open && active >= 0 ? `${id}-option-${active}` : undefined"
        @input="changed"
        @focus="focus"
        @blur="blur"
        @keydown="onKeydown"
        @compositionstart="isComposing = true"
        @compositionend="isComposing = false"
      />
      <kbd v-if="!modelValue && !focused" class="search-kbd" aria-hidden="true">/</kbd>
      <button
        v-if="modelValue"
        type="button"
        class="icon-button clear-query"
        :aria-label="t.clear"
        @click="emit('update:modelValue', ''); suggestions = []; open = false; inputRef?.focus()"
      >
        <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg>
      </button>
      <button class="button primary search-submit" type="submit">{{ copy.searchButton }}</button>
      <span class="scanning-laser-beam" aria-hidden="true"></span>
    </form>
    <ul v-if="open && items.length" :id="`${id}-suggestions`" class="suggestions" role="listbox" :aria-label="copy.recent">
      <li
        v-for="(item, index) in items"
        :id="`${id}-option-${index}`"
        :key="item.text"
        role="option"
        :aria-selected="active === index"
        :class="{ active: active === index }"
      >
        <button type="button" class="suggestion-text" @mousedown.prevent @click="choose(item.text)">
          <span class="glyph" aria-hidden="true">
            <svg v-if="item.recent" class="icon-svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5V12l3 1.8" /></svg>
            <svg v-else class="icon-svg" viewBox="0 0 24 24"><path d="M7 17L17 7M9 7h8v8" /></svg>
          </span>
          <span>{{ item.text }}</span>
        </button>
        <button
          v-if="item.recent"
          type="button"
          class="icon-button"
          :aria-label="`${t.clear}: ${item.text}`"
          @mousedown.prevent
          @click="emit('removeRecent', item.text)"
        >
          <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12" /></svg>
        </button>
      </li>
    </ul>
  </div>
</template>

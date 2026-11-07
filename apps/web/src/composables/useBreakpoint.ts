import { computed, onMounted, onUnmounted, ref } from 'vue';

// Layout tiers from the commercialization spec:
// PC >= 1280 (optimized at 1440), tablet 768–1279, mobile 360–767.
export const BREAKPOINTS = [1440, 1280, 1024, 768, 480, 390, 360] as const;

export type BreakpointTier = 'pc-wide' | 'pc' | 'tablet-wide' | 'tablet' | 'mobile-wide' | 'mobile' | 'mobile-narrow';

const width = ref(typeof window === 'undefined' ? 1440 : window.innerWidth);
let listeners = 0;

function onResize(): void {
  width.value = window.innerWidth;
}

function tierFor(value: number): BreakpointTier {
  if (value >= 1440) return 'pc-wide';
  if (value >= 1280) return 'pc';
  if (value >= 1024) return 'tablet-wide';
  if (value >= 768) return 'tablet';
  if (value >= 480) return 'mobile-wide';
  if (value >= 390) return 'mobile';
  return 'mobile-narrow';
}

export function useBreakpoint() {
  onMounted(() => {
    if (listeners === 0) window.addEventListener('resize', onResize, { passive: true });
    listeners++;
    width.value = window.innerWidth;
  });
  onUnmounted(() => {
    listeners--;
    if (listeners === 0) window.removeEventListener('resize', onResize);
  });

  const tier = computed(() => tierFor(width.value));
  const isDesktop = computed(() => width.value >= 1280);
  const isTablet = computed(() => width.value >= 768 && width.value < 1280);
  const isMobile = computed(() => width.value < 768);
  const isNarrow = computed(() => width.value < 480);
  const atLeast = (min: number) => computed(() => width.value >= min);

  return { width, tier, isDesktop, isTablet, isMobile, isNarrow, atLeast };
}

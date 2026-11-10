<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import BrandMark from '../../BrandMark.vue';
import ScanArt from '../../ScanArt.vue';
import { coreMessages, designCopy, signupCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';

const route = useRoute();
const session = useSessionStore();
const t = coreMessages;
const copy = designCopy;
const page = computed(() => String(route.name ?? ''));
const authReturn = computed(() => {
  const target = route.query.returnTo;
  return target ? `?returnTo=${encodeURIComponent(String(target))}` : '';
});
const authBenefits = computed(() => [copy.value.benefitMail, copy.value.benefitVerify, copy.value.benefitHash, copy.value.benefitHistory]);
const authIntro = computed(() =>
  page.value === 'login'
    ? signupCopy.value.loginIntro
    : page.value === 'signup'
      ? copy.value.signupIntro
      : page.value === 'forgot-password'
        ? signupCopy.value.forgotIntro
        : page.value === 'verify-email'
          ? copy.value.verifyIntro
          : page.value === 'reset-password'
            ? copy.value.resetIntro
            : copy.value.authIntro
);
</script>
<template>
  <section class="auth-shell">
    <aside class="auth-visual">
      <div class="auth-visual-copy">
        <p class="auth-visual-brand"><BrandMark /><span class="brand-word">Open<b>Scanner</b></span></p>
        <h2>{{ t.slogan }}</h2>
        <ul class="auth-benefits">
          <li v-for="item in authBenefits" :key="item">
            <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m5 10.5 3 3 7-7" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" /></svg>{{ item }}
          </li>
        </ul>
      </div>
      <ScanArt />
    </aside>

    <div class="auth-card">
      <nav v-if="page === 'login' || page === 'signup'" class="auth-tabs" :aria-label="copy.authTabs">
        <RouterLink :to="`/${session.locale}/login${authReturn}`" :aria-current="page === 'login' ? 'page' : undefined">{{ t.login }}</RouterLink>
        <RouterLink :to="`/${session.locale}/signup${authReturn}`" :aria-current="page === 'signup' ? 'page' : undefined">{{ t.signup }}</RouterLink>
      </nav>
      <RouterLink v-else class="auth-back" :to="`/${session.locale}/login`">
        <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M12 19l-7-7 7-7" /></svg>
        <span>{{ copy.backToLogin }}</span>
      </RouterLink>
      <slot :intro="authIntro" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { RouterLink, useRoute } from 'vue-router';
import BrandMark from '../../BrandMark.vue';
import { coreMessages, designCopy, signupCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';

const route = useRoute();
const session = useSessionStore();
const t = coreMessages;
const copy = designCopy;
const signup = signupCopy;
const page = computed(() => String(route.name ?? ''));
const authReturn = computed(() => {
  const target = route.query.returnTo;
  return target ? `?returnTo=${encodeURIComponent(String(target))}` : '';
});
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
  <section class="auth-shell" :class="{ 'auth-shell--login': page === 'login' }">
    <div v-if="page === 'login'" class="auth-card auth-card--split">
      <div class="auth-brandcol">
        <BrandMark :size="40" />
        <h1>{{ t.login }}</h1>
        <p>{{ signup.loginIntro }}</p>
      </div>
      <div class="auth-formcol">
        <slot :intro="authIntro" />
      </div>
    </div>
    <div v-else class="auth-card">
      <RouterLink v-if="page !== 'signup'" class="auth-back" :to="`/${session.locale}/login`">
        <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M12 19l-7-7 7-7" /></svg>
        <span>{{ copy.backToLogin }}</span>
      </RouterLink>
      <slot :intro="authIntro" />
      <p v-if="page === 'signup'" class="auth-switch">
        {{ signup.haveAccount }}
        <RouterLink :to="`/${session.locale}/login${authReturn}`">{{ t.login }}</RouterLink>
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { RouterLink, useRoute, useRouter } from 'vue-router';
import { APIError } from '../../client';
import AuthShell from '../../components/auth/AuthShell.vue';
import AuthStatus from '../../components/auth/AuthStatus.vue';
import { coreMessages, designCopy, signupCopy } from '../../i18n/index';
import { safeReturn } from '../../router';
import { useSessionStore } from '../../stores/session';
import { useAuthForm } from './authForm';

const session = useSessionStore();
const route = useRoute();
const router = useRouter();
const t = coreMessages;
const copy = designCopy;
const signup = signupCopy;
const { email, password, showPassword, busy, error, errorStatus, notice, authTitle, authIntro, serverDown, errorText, resetSecrets } = useAuthForm();

async function submit(): Promise<void> {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await session.login(email.value, password.value);
    const target = typeof route.query.returnTo === 'string' ? route.query.returnTo : null;
    await router.push(safeReturn(target, session.locale));
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
    errorStatus.value = e instanceof APIError ? e.status : 0;
  } finally {
    busy.value = false;
    resetSecrets();
  }
}
</script>
<template>
  <AuthShell>
    <header class="auth-header">
      <h1>{{ authTitle }}</h1>
      <p>{{ authIntro }}</p>
    </header>
    <AuthStatus :server-down="serverDown" :notice="notice" :error="error" :error-text="errorText" @retry="error = ''; errorStatus = 0">
      <form class="auth-form" @submit.prevent="submit">
        <div class="form-group">
          <label for="auth-email">{{ signup.loginId }}</label>
          <input id="auth-email" v-model="email" type="text" autocomplete="username" autocapitalize="none" spellcheck="false" maxlength="254" required />
        </div>
        <div class="form-group">
          <div class="label-row">
            <label for="auth-password">{{ t.password }}</label>
            <RouterLink class="forgot-link" :to="`/${session.locale}/forgot-password`">{{ t.forgot }}</RouterLink>
          </div>
          <div class="password-field">
            <input id="auth-password" v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" :minlength="1" maxlength="1024" required />
            <button type="button" class="icon-button" :aria-label="showPassword ? copy.hidePassword : copy.showPassword" :aria-pressed="showPassword" @click="showPassword = !showPassword">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="3" /><path v-if="showPassword" d="m4 4 16 16" /></svg>
            </button>
          </div>
        </div>
        <button class="button primary full-width auth-submit" type="submit" :disabled="busy">{{ busy ? t.loading : t.login }}</button>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

<script setup lang="ts">
import { APIError } from '../../client';
import AuthShell from '../../components/auth/AuthShell.vue';
import AuthStatus from '../../components/auth/AuthStatus.vue';
import { coreMessages, signupCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';
import { useAuthForm } from './authForm';

const session = useSessionStore();
const t = coreMessages;
const signup = signupCopy;
const { email, busy, error, errorStatus, notice, authTitle, authIntro, serverDown, errorText, resetSecrets } = useAuthForm();

async function submit(): Promise<void> {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await session.sendForgotPassword(email.value);
    notice.value = t.value.resetNotice;
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
        <button class="button primary full-width auth-submit" type="submit" :disabled="busy">{{ busy ? t.loading : t.reset }}</button>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

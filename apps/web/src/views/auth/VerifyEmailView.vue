<script setup lang="ts">
import { ref } from 'vue';
import { APIError } from '../../client';
import AuthShell from '../../components/auth/AuthShell.vue';
import AuthStatus from '../../components/auth/AuthStatus.vue';
import { coreMessages } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';
import { useAuthForm } from './authForm';

const session = useSessionStore();
const t = coreMessages;
const { busy, error, errorStatus, notice, authTitle, authIntro, serverDown, errorText, resetSecrets } = useAuthForm();

const token = ref(new URLSearchParams(location.hash.slice(1)).get('token') || '');
if (token.value) history.replaceState(history.state, '', location.pathname + location.search);

async function submit(): Promise<void> {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await session.verifyEmail(token.value);
    token.value = '';
    notice.value = t.value.verified;
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
        <p v-if="!token && !notice" class="notice">{{ t.invalidLink }}</p>
        <button class="button primary full-width auth-submit" type="submit" :disabled="busy || !token">{{ busy ? t.loading : t.verify }}</button>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

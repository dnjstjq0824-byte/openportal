<script setup lang="ts">
import { ref } from 'vue';
import { useRoute } from 'vue-router';
import { APIError } from '../../client';
import AuthShell from '../../components/auth/AuthShell.vue';
import AuthStatus from '../../components/auth/AuthStatus.vue';
import { coreMessages, designCopy } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';
import { useAuthForm } from './authForm';

const session = useSessionStore();
const route = useRoute();
const t = coreMessages;
const copy = designCopy;
const { password, confirm, showPassword, busy, error, errorStatus, notice, passwordScore, passwordVaried, strengthLabel, confirmState, authTitle, authIntro, serverDown, errorText, resetSecrets } = useAuthForm();

const initialHashToken = new URLSearchParams(location.hash.slice(1)).get('token') || '';
const initialQueryToken = typeof route.query.token === 'string' ? route.query.token : '';
const token = ref(initialHashToken || initialQueryToken);
if (initialHashToken) history.replaceState(history.state, '', location.pathname + location.search);

async function submit(): Promise<void> {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    if (password.value !== confirm.value) {
      notice.value = t.value.mismatch;
      return;
    }
    await session.resetPassword(token.value, password.value);
    token.value = '';
    notice.value = t.value.resetDone;
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
          <div class="label-row"><label for="auth-password">{{ t.password }}</label></div>
          <div class="password-field">
            <input id="auth-password" v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" :minlength="12" maxlength="1024" aria-describedby="password-help" required />
            <button type="button" class="icon-button" :aria-label="showPassword ? copy.hidePassword : copy.showPassword" :aria-pressed="showPassword" @click="showPassword = !showPassword">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" /><circle cx="12" cy="12" r="3" /><path v-if="showPassword" d="m4 4 16 16" /></svg>
            </button>
          </div>
          <div id="password-help" class="password-help">
            <div v-if="password" class="strength" :data-score="passwordScore">
              <span class="strength-bar" aria-hidden="true"><i v-for="n in 3" :key="n" :class="{ on: passwordScore >= n }"></i></span>
              <span>{{ copy.strength }}: <strong>{{ strengthLabel }}</strong></span>
            </div>
            <ul class="requirements">
              <li :class="{ met: password.length >= 12 }"><span aria-hidden="true">{{ password.length >= 12 ? '✓' : '•' }}</span>{{ copy.reqLength }}</li>
              <li :class="{ met: passwordVaried }"><span aria-hidden="true">{{ passwordVaried ? '✓' : '•' }}</span>{{ copy.reqVariety }}</li>
            </ul>
          </div>
        </div>
        <div class="form-group">
          <label for="auth-confirm">{{ t.confirmPassword }}</label>
          <input id="auth-confirm" v-model="confirm" :type="showPassword ? 'text' : 'password'" autocomplete="new-password" minlength="12" maxlength="1024" aria-describedby="confirm-help" required />
          <small id="confirm-help" class="match-hint" :class="confirmState" aria-live="polite">{{ confirmState === 'match' ? '✓ ' + copy.passwordsMatch : confirmState === 'differ' ? copy.passwordsDiffer : '' }}</small>
        </div>
        <p v-if="!token && !notice" class="notice">{{ t.invalidLink }}</p>
        <button class="button primary full-width auth-submit" type="submit" :disabled="busy || !token">{{ busy ? t.loading : t.reset }}</button>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

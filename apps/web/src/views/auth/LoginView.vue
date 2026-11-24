<script setup lang="ts">
import { computed, nextTick, ref } from 'vue';
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
const { email, password, showPassword, busy, error, errorStatus, notice, serverDown, errorText, resetSecrets } = useAuthForm();

const step = ref(1);
const emailInput = ref<HTMLInputElement | null>(null);
const passwordInput = ref<HTMLInputElement | null>(null);
const signupTarget = computed(() => {
  const target = route.query.returnTo;
  return `/${session.locale}/signup${typeof target === 'string' && target ? `?returnTo=${encodeURIComponent(target)}` : ''}`;
});

// Two-step sign-in is presentation-only: the identifier is echoed back in step 2
// and the credentials are verified with a single login call. No account-existence
// probe happens between steps, so the flow cannot be used to enumerate accounts.
function goPassword(): void {
  email.value = email.value.trim();
  error.value = '';
  notice.value = '';
  step.value = 2;
  void nextTick(() => passwordInput.value?.focus());
}

function goIdentifier(): void {
  step.value = 1;
  void nextTick(() => emailInput.value?.focus());
}

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
    <AuthStatus :server-down="serverDown" :notice="notice" :error="error" :error-text="errorText" @retry="error = ''; errorStatus = 0">
      <form v-if="step === 1" class="auth-form" @submit.prevent="goPassword">
        <div class="form-group">
          <label for="auth-email">{{ signup.loginId }}</label>
          <input id="auth-email" ref="emailInput" v-model="email" type="text" autocomplete="username" autocapitalize="none" spellcheck="false" maxlength="254" required />
        </div>
        <div class="auth-actions">
          <RouterLink class="auth-textlink" :to="signupTarget">{{ t.signup }}</RouterLink>
          <button class="button primary auth-next" type="submit">{{ t.next }}</button>
        </div>
      </form>
      <form v-else class="auth-form" @submit.prevent="submit">
        <div>
          <button type="button" class="auth-id-chip" :aria-label="signup.loginId" @click="goIdentifier">
            <span class="auth-id-chip-text">{{ email }}</span>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="6 9 12 15 18 9"/></svg>
          </button>
        </div>
        <fieldset class="auth-pass-fieldset">
          <legend>{{ t.password }}</legend>
          <input id="auth-password" ref="passwordInput" v-model="password" :type="showPassword ? 'text' : 'password'" autocomplete="current-password" :aria-label="t.password" :minlength="1" maxlength="1024" required />
        </fieldset>
        <label class="auth-check">
          <input v-model="showPassword" type="checkbox" />
          {{ copy.showPassword }}
        </label>
        <div class="auth-actions">
          <RouterLink class="auth-textlink" :to="`/${session.locale}/forgot-password`">{{ t.forgot }}</RouterLink>
          <button class="button primary auth-next" type="submit" :disabled="busy">{{ busy ? t.loading : t.next }}</button>
        </div>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

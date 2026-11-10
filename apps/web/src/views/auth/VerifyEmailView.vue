<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { APIError, request } from '../../client';
import AuthShell from '../../components/auth/AuthShell.vue';
import AuthStatus from '../../components/auth/AuthStatus.vue';
import { coreMessages } from '../../i18n/index';
import { useSessionStore } from '../../stores/session';
import { useAuthForm } from './authForm';

const session = useSessionStore();
const route = useRoute();
const t = coreMessages;
const { busy, error, errorStatus, notice, authTitle, authIntro, serverDown, errorText, resetSecrets } = useAuthForm();

const initialHashToken = new URLSearchParams(location.hash.slice(1)).get('token') || '';
const initialQueryToken = typeof route.query.token === 'string' ? route.query.token : '';
const token = ref(initialHashToken || initialQueryToken);
if (initialHashToken) history.replaceState(history.state, '', location.pathname + location.search);

const otpDigits = ref<string[]>(['', '', '', '', '', '']);
const otpInputs = ref<HTMLInputElement[]>([]);
const otpCode = computed(() => otpDigits.value.join(''));
const effectiveToken = computed(() => token.value || (otpCode.value.length === 6 ? otpCode.value : ''));
// D-5: OTP verification is account-bound; the account email must accompany the code.
const accountEmail = ref(session.user?.recovery_email || session.user?.email || '');

const cooldown = ref(0);
let cooldownTimer: ReturnType<typeof setInterval> | undefined;

function startCooldown(seconds = 60): void {
  cooldown.value = seconds;
  if (cooldownTimer) clearInterval(cooldownTimer);
  cooldownTimer = setInterval(() => {
    if (cooldown.value > 0) cooldown.value -= 1;
    else if (cooldownTimer) clearInterval(cooldownTimer);
  }, 1000);
}

const formattedCooldown = computed(() => {
  const m = String(Math.floor(cooldown.value / 60)).padStart(2, '0');
  const s = String(cooldown.value % 60).padStart(2, '0');
  return `${m}:${s}`;
});

function setOtpInput(el: unknown, index: number): void {
  if (el instanceof HTMLInputElement) otpInputs.value[index] = el;
}

function onOtpInput(index: number, event: Event): void {
  const input = event.target as HTMLInputElement;
  const digit = input.value.replace(/\D/g, '').slice(-1);
  otpDigits.value[index] = digit;
  input.value = digit;
  if (digit && index < 5) {
    otpInputs.value[index + 1]?.focus();
  }
}

function onOtpKeydown(index: number, event: KeyboardEvent): void {
  if (event.key === 'Backspace' && !otpDigits.value[index] && index > 0) {
    otpDigits.value[index - 1] = '';
    otpInputs.value[index - 1]?.focus();
  } else if (event.key === 'ArrowLeft' && index > 0) {
    event.preventDefault();
    otpInputs.value[index - 1]?.focus();
  } else if (event.key === 'ArrowRight' && index < 5) {
    event.preventDefault();
    otpInputs.value[index + 1]?.focus();
  }
}

function onOtpPaste(event: ClipboardEvent): void {
  const pasted = (event.clipboardData?.getData('text') || '').replace(/\D/g, '').slice(0, 6);
  if (!pasted) return;
  event.preventDefault();
  for (let i = 0; i < 6; i++) {
    otpDigits.value[i] = pasted[i] || '';
  }
  const focusIdx = Math.min(pasted.length, 5);
  otpInputs.value[focusIdx]?.focus();
}

async function resendVerification(): Promise<void> {
  if (cooldown.value > 0 || busy.value) return;
  const targetEmail = session.user?.recovery_email || session.user?.email;
  if (!targetEmail) return;
  busy.value = true;
  error.value = '';
  try {
    await request('/auth/resend', { method: 'POST', body: JSON.stringify({ email: targetEmail }) });
    startCooldown(60);
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
    errorStatus.value = e instanceof APIError ? e.status : 0;
  } finally {
    busy.value = false;
  }
}

async function submit(): Promise<void> {
  if (!effectiveToken.value) return;
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    await session.verifyEmail(effectiveToken.value, accountEmail.value.trim() || undefined);
    token.value = '';
    otpDigits.value = ['', '', '', '', '', ''];
    notice.value = t.value.verified;
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
    errorStatus.value = e instanceof APIError ? e.status : 0;
  } finally {
    busy.value = false;
    resetSecrets();
  }
}

onUnmounted(() => {
  if (cooldownTimer) clearInterval(cooldownTimer);
});
</script>
<template>
  <AuthShell>
    <header class="auth-header">
      <h1>{{ authTitle }}</h1>
      <p>{{ authIntro }}</p>
    </header>
    <AuthStatus :server-down="serverDown" :notice="notice" :error="error" :error-text="errorText" @retry="error = ''; errorStatus = 0">
      <form class="auth-form" @submit.prevent="submit">
        <p v-if="!effectiveToken && !notice" class="notice">{{ t.invalidLink }}</p>

        <div v-if="!token && !notice" class="otp-section">
          <p class="otp-label">6자리 인증번호 입력 (OTP)</p>
          <div class="form-group">
            <label for="verify-email">{{ t.email }}</label>
            <input id="verify-email" v-model="accountEmail" type="email" autocomplete="email" autocapitalize="none" spellcheck="false" maxlength="254" required />
          </div>
          <div class="otp-grid" role="group" aria-label="OTP 6 digits">
            <input
              v-for="(_, idx) in 6"
              :key="idx"
              :ref="el => setOtpInput(el, idx)"
              :value="otpDigits[idx]"
              class="otp-digit"
              type="text"
              inputmode="numeric"
              pattern="[0-9]*"
              maxlength="1"
              autocomplete="one-time-code"
              :aria-label="`Digit ${idx + 1}`"
              @input="onOtpInput(idx, $event)"
              @keydown="onOtpKeydown(idx, $event)"
              @paste="onOtpPaste"
            />
          </div>
          <div v-if="session.user" class="otp-meta">
            <span v-if="cooldown > 0" class="otp-timer">{{ formattedCooldown }}</span>
            <button type="button" class="text-button" :disabled="cooldown > 0 || busy" @click="resendVerification">{{ t.resend }}</button>
          </div>
        </div>

        <button class="button primary full-width auth-submit" type="submit" :disabled="busy || !effectiveToken">{{ busy ? t.loading : t.verify }}</button>
      </form>
    </AuthStatus>
  </AuthShell>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { APIError } from '../client';
import { coreErrors, coreMessages, signupCopy, signupErrorCopy } from '../i18n/index';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const router = useRouter();
const t = coreMessages;
const signup = signupCopy;

const displayName = ref(session.user?.display_name ?? '');
const password = ref('');
const busy = ref(false);
const error = ref('');
const notice = ref('');
const errorText = computed(() => (signupErrorCopy.value as Record<string, string>)[error.value] || coreErrors.value[error.value] || coreErrors.value.fallback);

async function saveAccount(): Promise<void> {
  busy.value = true;
  try {
    await session.saveAccount(displayName.value, session.locale);
    notice.value = t.value.saved;
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
  }
}

async function deleteAccount(): Promise<void> {
  busy.value = true;
  try {
    await session.deleteAccount(password.value);
    await router.push(`/${session.locale}`);
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
    password.value = '';
  }
}

async function resend(): Promise<void> {
  busy.value = true;
  try {
    await session.resendRecovery();
    error.value = '';
    notice.value = signup.value.doneRecoverySent;
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <section class="account-panel">
    <div class="page-heading">
      <p class="eyebrow">OPENSCANNER / {{ t.account }}</p>
      <h1>{{ t.account }}</h1>
      <p>{{ session.user?.display_name }}</p>
    </div>
    <dl class="review-list account-facts">
      <div>
        <dt>{{ signup.mailAddress }}</dt>
        <dd><strong>{{ session.user?.email }}</strong></dd>
      </div>
      <div v-if="session.user?.username">
        <dt>{{ signup.recoveryStatus }}</dt>
        <dd>
          {{ session.user?.recovery_email || signup.none }}
          <span v-if="session.user?.recovery_email" class="status-pill" :class="{ ok: session.user?.recovery_email_verified }">{{ session.user?.recovery_email_verified ? signup.verified : signup.unverified }}</span>
        </dd>
        <button v-if="session.user?.recovery_email && !session.user?.recovery_email_verified" type="button" class="text-button" :disabled="busy" @click="resend">{{ signup.resendRecovery }}</button>
      </div>
    </dl>
    <form class="auth-form" @submit.prevent="saveAccount">
      <div class="form-group">
        <label for="account-name">{{ t.displayName }}</label>
        <input id="account-name" v-model="displayName" maxlength="80" required />
      </div>
      <button class="button primary" :disabled="busy">{{ t.save }}</button>
    </form>
    <hr />
    <details class="danger-panel">
      <summary>{{ t.deleteAccount }}</summary>
      <p>{{ t.deleteWarning }}</p>
      <form class="auth-form" @submit.prevent="deleteAccount">
        <label for="delete-password">{{ t.password }}</label>
        <input id="delete-password" v-model="password" type="password" autocomplete="current-password" required />
        <button class="button danger" :disabled="busy">{{ t.deleteAccount }}</button>
      </form>
    </details>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <p v-if="error" class="alert" role="alert">{{ errorText }}</p>
  </section>
</template>

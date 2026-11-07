<script setup lang="ts">
import { useRouter } from 'vue-router';
import AuthShell from '../../components/auth/AuthShell.vue';
import SignupWizard from '../../SignupWizard.vue';
import { useSessionStore } from '../../stores/session';

const session = useSessionStore();
const router = useRouter();

async function onCreated(): Promise<void> {
  try {
    await session.loadSession();
  } catch {
    /* session refresh retried by the refresh timer */
  }
}
</script>
<template>
  <AuthShell>
    <SignupWizard :locale="session.locale" @created="onCreated" @navigate="router.push($event)" />
  </AuthShell>
</template>

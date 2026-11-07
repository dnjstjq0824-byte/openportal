<script setup lang="ts">
import { useRouter } from 'vue-router';
import MailboxPanel from '../MailView.vue';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const router = useRouter();

function expired(): void {
  session.expire();
  void router.replace({ path: `/${session.locale}/login`, query: { returnTo: `/${session.locale}/mail` } });
}
</script>
<template>
  <MailboxPanel :locale="session.locale" :user="session.user" @expired="expired" @dirty-change="session.mailDraftDirty = $event" />
</template>

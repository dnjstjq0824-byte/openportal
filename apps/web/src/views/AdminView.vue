<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { RouterLink, useRoute, useRouter } from 'vue-router';
import ScanState from '../ScanState.vue';
import { coreErrors, coreMessages, designCopy, signupCopy, signupErrorCopy } from '../i18n/index';
import { useAdminStore } from '../stores/admin';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const admin = useAdminStore();
const route = useRoute();
const router = useRouter();
const t = coreMessages;
const copy = designCopy;
const signup = signupCopy;
const errorText = computed(() => (signupErrorCopy.value as Record<string, string>)[admin.error] || coreErrors.value[admin.error] || coreErrors.value.fallback);

function adminCount(value: number): string {
  return new Intl.NumberFormat(session.locale).format(value);
}

async function handleExpired(status: 'ok' | 'denied' | 'expired'): Promise<void> {
  if (status !== 'expired') return;
  session.expire();
  await router.replace({ path: `/${session.locale}/login`, query: { returnTo: route.path } });
}

async function submitSearch(): Promise<void> {
  await handleExpired(await admin.submitSearch());
}

async function goToPage(next: number): Promise<void> {
  await handleExpired(await admin.goToPage(next));
}

async function toggleRole(email: string, isAdmin: boolean): Promise<void> {
  await handleExpired(await admin.setRole(email, !isAdmin));
}

const chartMax = computed(() => Math.max(1, ...admin.signups.map(x => x.count)));
const chartBars = computed(() =>
  admin.signups.map((x, i) => ({
    ...x,
    x: 8 + i * 34,
    height: Math.round((x.count / chartMax.value) * 90),
    label: x.date.slice(5),
  }))
);

onMounted(async () => {
  admin.reset();
  await handleExpired(await admin.load());
  await admin.loadSignups();
});
</script>
<template>
  <section class="admin-panel">
    <div class="page-heading">
      <p class="eyebrow">OPENSCANNER / {{ t.admin }}</p>
      <h1>{{ t.admin }}</h1>
    </div>
    <div v-if="admin.denied" class="serp-state" role="alert">
      <ScanState kind="unavailable" />
      <h2>{{ t.adminDeniedTitle }}</h2>
      <p>{{ t.adminDeniedBody }}</p>
      <div class="actions"><RouterLink class="button primary" :to="`/${session.locale}`">{{ t.backHome }}</RouterLink></div>
    </div>
    <template v-else>
      <p v-if="admin.busy && !admin.overview" role="status">{{ t.loading }}</p>
      <div v-if="admin.overview" class="stat-grid">
        <div class="stat-card"><strong>{{ adminCount(admin.overview.accounts) }}</strong><span>{{ t.adminAccounts }}</span></div>
        <div class="stat-card"><strong>{{ adminCount(admin.overview.sessions) }}</strong><span>{{ t.adminSessions }}</span></div>
        <div class="stat-card"><strong>{{ adminCount(admin.overview.mailboxes) }}</strong><span>{{ t.adminMailboxes }}</span></div>
        <div class="stat-card"><strong>{{ adminCount(admin.overview.signups_24h) }}</strong><span>{{ t.adminSignups24h }}</span></div>
      </div>
      <figure v-if="admin.overview && admin.signups.length" class="admin-chart">
        <figcaption>{{ t.adminSignupsTitle }}</figcaption>
        <svg viewBox="0 0 484 130" role="img" :aria-label="t.adminSignupsTitle">
          <g v-for="bar in chartBars" :key="bar.date">
            <rect :x="bar.x" :y="110 - bar.height" width="22" :height="Math.max(bar.height, 2)" rx="3" class="chart-bar">
              <title>{{ bar.date }}: {{ adminCount(bar.count) }}</title>
            </rect>
            <text :x="bar.x + 11" y="124" text-anchor="middle" class="chart-label">{{ bar.label }}</text>
          </g>
        </svg>
      </figure>
      <form v-if="admin.overview" class="admin-search" @submit.prevent="submitSearch">
        <label class="sr-only" for="admin-q">{{ t.adminSearchPlaceholder }}</label>
        <input id="admin-q" v-model="admin.query" type="search" :placeholder="t.adminSearchPlaceholder" maxlength="80" />
        <button class="button primary" type="submit" :disabled="admin.busy">{{ copy.searchButton }}</button>
      </form>
      <div v-if="admin.accounts.length" class="table-scroll">
        <table class="admin-table">
          <thead>
            <tr>
              <th scope="col">{{ t.email }}</th>
              <th scope="col">{{ signup.username }}</th>
              <th scope="col">{{ t.displayName }}</th>
              <th scope="col">{{ t.adminVerified }}</th>
              <th scope="col">{{ t.adminRole }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in admin.accounts" :key="row.email">
              <td>{{ row.email }} <span v-if="row.is_admin" class="status-pill ok">{{ t.admin }}</span></td>
              <td>{{ row.username || '—' }}</td>
              <td>{{ row.display_name }}</td>
              <td>{{ row.verified ? '✓' : '—' }}</td>
              <td>
                <button
                  type="button"
                  class="text-button"
                  :disabled="admin.roleBusy === row.email"
                  @click="toggleRole(row.email, row.is_admin)"
                >{{ row.is_admin ? t.adminDemote : t.adminPromote }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else-if="admin.overview && !admin.busy" class="muted">{{ t.adminEmpty }}</p>
      <nav v-if="admin.accounts.length && (admin.page > 1 || admin.hasMore)" class="pagination" :aria-label="t.resultCount">
        <button class="button" :disabled="admin.page <= 1 || admin.busy" @click="goToPage(admin.page - 1)"><span aria-hidden="true">←</span> {{ t.previous }}</button>
        <span class="page-indicator" aria-current="page">{{ adminCount(admin.page) }}</span>
        <button class="button" :disabled="!admin.hasMore || admin.busy" @click="goToPage(admin.page + 1)">{{ t.next }} <span aria-hidden="true">→</span></button>
      </nav>
      <p v-if="admin.error" class="alert" role="alert">{{ errorText }}</p>
    </template>
  </section>
</template>

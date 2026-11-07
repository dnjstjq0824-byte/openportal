import { defineStore } from 'pinia';
import { ref } from 'vue';
import { APIError, mutate, request } from '../client';

export interface SignupPoint {
  date: string;
  count: number;
}

export interface AdminOverview {
  accounts: number;
  sessions: number;
  mailboxes: number;
  signups_24h: number;
}

export interface AdminAccount {
  email: string;
  username?: string;
  display_name: string;
  verified: boolean;
  is_admin: boolean;
}

export const useAdminStore = defineStore('admin', () => {
  const overview = ref<AdminOverview | null>(null);
  const accounts = ref<AdminAccount[]>([]);
  const total = ref(0);
  const page = ref(1);
  const hasMore = ref(false);
  const query = ref('');
  const denied = ref(false);
  const busy = ref(false);
  const error = ref('');
  const signups = ref<SignupPoint[]>([]);
  const roleBusy = ref('');

  function reset(): void {
    overview.value = null;
    accounts.value = [];
    total.value = 0;
    page.value = 1;
    hasMore.value = false;
    query.value = '';
    denied.value = false;
    error.value = '';
    signups.value = [];
  }

  // Returns 'expired' when the session died mid-load so the view can redirect.
  async function load(): Promise<'ok' | 'denied' | 'expired'> {
    busy.value = true;
    denied.value = false;
    error.value = '';
    try {
      const params = new URLSearchParams({ q: query.value.trim(), page: String(page.value) });
      const [nextOverview, list] = await Promise.all([request('/admin/overview'), request(`/admin/accounts?${params}`)]);
      overview.value = nextOverview;
      accounts.value = list.accounts;
      total.value = list.total;
      hasMore.value = list.has_more;
      page.value = list.page;
      return 'ok';
    } catch (e) {
      if (e instanceof APIError && (e.code === 'FORBIDDEN' || e.status === 403)) {
        denied.value = true;
        return 'denied';
      }
      if (e instanceof APIError && e.status === 401) return 'expired';
      error.value = e instanceof APIError ? e.code : 'fallback';
      return 'ok';
    } finally {
      busy.value = false;
    }
  }

  async function submitSearch(): Promise<'ok' | 'denied' | 'expired'> {
    page.value = 1;
    return load();
  }

  async function goToPage(next: number): Promise<'ok' | 'denied' | 'expired'> {
    page.value = next;
    return load();
  }

  async function loadSignups(): Promise<void> {
    try {
      const data = await request('/admin/signups?days=14');
      if (Array.isArray(data.signups)) signups.value = data.signups;
    } catch {
      signups.value = [];
    }
  }

  async function setRole(email: string, isAdmin: boolean): Promise<'ok' | 'denied' | 'expired'> {
    roleBusy.value = email;
    error.value = '';
    try {
      await mutate(`/admin/accounts/${encodeURIComponent(email)}/role`, { is_admin: isAdmin }, 'PATCH');
      const row = accounts.value.find(x => x.email === email);
      if (row) row.is_admin = isAdmin;
      return 'ok';
    } catch (e) {
      if (e instanceof APIError && (e.code === 'FORBIDDEN' || e.status === 403)) {
        if (e.code === 'SELF_DEMOTE_FORBIDDEN') {
          error.value = e.code;
          return 'ok';
        }
        denied.value = true;
        return 'denied';
      }
      if (e instanceof APIError && e.status === 401) return 'expired';
      error.value = e instanceof APIError ? e.code : 'fallback';
      return 'ok';
    } finally {
      roleBusy.value = '';
    }
  }

  return { overview, accounts, total, page, hasMore, query, denied, busy, error, signups, roleBusy, reset, load, submitSearch, goToPage, loadSignups, setRole };
});

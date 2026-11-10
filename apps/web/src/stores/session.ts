import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { mutate, session as fetchSession } from '../client';
import type { Locale } from '../i18n';
import { setPortalLocale } from '../i18n/index';
import { migrateStorage, persist, removeStored, stored } from '../utils/storage';

migrateStorage();

export type SafeSearch = 'strict' | 'moderate' | 'off';

function readSafeSearch(value: string): SafeSearch {
  return value === 'strict' || value === 'moderate' || value === 'off' ? value : 'moderate';
}

export const useSessionStore = defineStore('session', () => {
  const user = ref<any>(null);
  const ready = ref(false);
  const sessionUnavailable = ref(false);

  const locale = ref<Locale>('en');
  const theme = ref('system');

  const recentOptIn = ref(stored('openscanner.recent_opt_in', 'false') === 'true');
  const recentSearches = ref<string[]>([]);
  const safeSearch = ref<SafeSearch>(readSafeSearch(stored('openscanner.safe_search', 'moderate')));
  const preferredSafeSearch = ref<SafeSearch>(safeSearch.value);
  const onboarded = ref(stored('openscanner.onboarded', '0') === '1');
  const mailDraftDirty = ref(false);

  if (recentOptIn.value) {
    try {
      const value = JSON.parse(stored('openscanner.recent_searches', '[]'));
      if (Array.isArray(value)) recentSearches.value = value.filter(x => typeof x === 'string').slice(0, 10);
    } catch {
      /* corrupted history: start empty */
    }
  }

  watch(recentOptIn, value => {
    persist('openscanner.recent_opt_in', String(value));
    if (!value) clearHistory();
  });

  watch(theme, value => {
    document.documentElement.dataset.theme = value;
    persist('openscanner.theme', value);
  });

  function initTheme(): void {
    theme.value = stored('openscanner.theme', 'system');
    document.documentElement.dataset.theme = theme.value;
  }

  function setLocale(value: Locale): void {
    locale.value = value;
    persist('openscanner.locale', value);
    setPortalLocale(value);
  }

  async function loadSession(): Promise<void> {
    try {
      user.value = await fetchSession();
    } catch {
      sessionUnavailable.value = true;
    }
    ready.value = true;
  }

  async function login(email: string, password: string): Promise<void> {
    await mutate('/auth/login', { email, password });
    user.value = await fetchSession();
  }

  async function logout(): Promise<void> {
    await mutate('/auth/logout', {});
    mailDraftDirty.value = false;
    user.value = null;
  }

  function expire(): void {
    mailDraftDirty.value = false;
    user.value = null;
  }

  async function sendForgotPassword(email: string): Promise<void> {
    await mutate('/auth/forgot-password', { email });
  }

  async function verifyEmail(token: string, email?: string): Promise<void> {
    // D-5: 6-digit OTP verification must identify the requesting account.
    await mutate('/auth/verify', email ? { token, email } : { token });
  }

  async function resetPassword(token: string, password: string): Promise<void> {
    await mutate('/auth/reset-password', { token, password });
    user.value = null;
  }

  async function resendRecovery(): Promise<void> {
    await mutate('/auth/resend', {});
  }

  async function saveAccount(displayName: string, accountLocale: Locale): Promise<void> {
    await mutate('/account', { displayName, locale: accountLocale }, 'PATCH');
    user.value = await fetchSession();
  }

  async function deleteAccount(password: string): Promise<void> {
    await mutate('/account', { password }, 'DELETE');
    user.value = null;
  }

  async function refreshAuth(): Promise<void> {
    await mutate('/auth/refresh', {});
  }

  function saveRecentSearch(q: string): void {
    const value = q.trim();
    if (!recentOptIn.value || !value) return;
    recentSearches.value = [value, ...recentSearches.value.filter(x => x !== value)].slice(0, 10);
    persist('openscanner.recent_searches', JSON.stringify(recentSearches.value));
  }

  function removeRecent(text: string): void {
    recentSearches.value = recentSearches.value.filter(x => x !== text);
    if (recentOptIn.value) persist('openscanner.recent_searches', JSON.stringify(recentSearches.value));
  }

  function clearHistory(): void {
    recentSearches.value = [];
    removeStored('openscanner.recent_searches');
  }

  function setSafeSearch(value: SafeSearch): void {
    preferredSafeSearch.value = value;
    safeSearch.value = value;
    persist('openscanner.safe_search', value);
  }

  function useQuerySafeSearch(value: string | null): void {
    safeSearch.value = value && ['strict', 'moderate', 'off'].includes(value) ? (value as SafeSearch) : preferredSafeSearch.value;
  }

  function dismissOnboarding(): void {
    onboarded.value = true;
    persist('openscanner.onboarded', '1');
  }

  return {
    user, ready, sessionUnavailable, locale, theme,
    recentOptIn, recentSearches, safeSearch, preferredSafeSearch,
    onboarded, mailDraftDirty,
    initTheme, setLocale, loadSession, login, logout, expire,
    sendForgotPassword, verifyEmail, resetPassword, resendRecovery,
    saveAccount, deleteAccount, refreshAuth,
    saveRecentSearch, removeRecent, clearHistory,
    setSafeSearch, useQuerySafeSearch, dismissOnboarding,
  };
});

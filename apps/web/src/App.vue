<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import MailView from './MailView.vue';
import { APIError, mutate, request, session } from './client';
import { errors, locales, messages, type Locale } from './i18n';

const locale = ref<Locale>('en');
const route = ref('');
const search = ref('');
const page = ref(1);
const safeSearch = ref('moderate');
const featureToast = ref('');
const result = ref<{
  total: number | null;
  has_more: boolean;
  elapsed_ms?: number;
  corrected_query?: string;
  did_you_mean?: string;
  hits: { title: string; url: string; description: string; site_name?: string; source?: string }[];
} | null>(null);

const busy = ref(false);
const error = ref('');
const notice = ref('');
const user = ref<any>(null);
const ready = ref(false);

// Auth inputs
const email = ref('');
const password = ref('');
const confirm = ref('');
const displayName = ref('');
const agree = ref(false);
const token = ref('');
const theme = ref('light');

// Suggestions & recent searches
const suggestions = ref<string[]>([]);
const showSuggestions = ref(false);
const activeSuggestionIndex = ref(-1);
const recentSearches = ref<string[]>([]);
try {
  recentSearches.value = JSON.parse(localStorage.getItem('openportal.recent_searches') || '[]');
} catch {
  recentSearches.value = [];
}

// Verticals visibility control (D-08: single setting object)
const enabledVerticals = {
  web: true,
  news: false,
  images: false,
  videos: false,
  maps: false,
  shopping: false,
};

const suggestionList = computed(() => {
  const current = search.value.trim().toLowerCase();
  const list: { text: string; isRecent: boolean }[] = [];
  const seen = new Set<string>();
  if (current) {
    for (const r of recentSearches.value) {
      if (r.toLowerCase().includes(current) && !seen.has(r)) {
        list.push({ text: r, isRecent: true });
        seen.add(r);
        if (list.length >= 3) break;
      }
    }
  } else {
    for (const r of recentSearches.value.slice(0, 5)) {
      list.push({ text: r, isRecent: true });
      seen.add(r);
    }
  }
  for (const s of suggestions.value) {
    if (!seen.has(s)) {
      list.push({ text: s, isRecent: false });
      seen.add(s);
      if (list.length >= 10) break;
    }
  }
  return list;
});

let suggestTimer: ReturnType<typeof setTimeout> | undefined;
function onSearchInput() {
  activeSuggestionIndex.value = -1;
  const q = search.value.trim();
  if (!q) {
    suggestions.value = [];
    showSuggestions.value = recentSearches.value.length > 0;
    return;
  }
  showSuggestions.value = true;
  clearTimeout(suggestTimer);
  suggestTimer = setTimeout(async () => {
    try {
      const data = await request(`/suggest?${new URLSearchParams({ q })}`);
      if (Array.isArray(data.suggestions)) suggestions.value = data.suggestions;
    } catch {}
  }, 150);
}

function onSearchFocus() {
  if (search.value.trim() || recentSearches.value.length > 0) showSuggestions.value = true;
}

function onSearchBlur() {
  setTimeout(() => {
    showSuggestions.value = false;
  }, 200);
}

function closeSuggestions() {
  showSuggestions.value = false;
  activeSuggestionIndex.value = -1;
}

function onSuggestDown() {
  if (!showSuggestions.value) {
    showSuggestions.value = true;
    return;
  }
  const max = suggestionList.value.length;
  if (max === 0) return;
  activeSuggestionIndex.value = (activeSuggestionIndex.value + 1) % max;
  if (activeSuggestionIndex.value >= 0 && activeSuggestionIndex.value < max) {
    search.value = suggestionList.value[activeSuggestionIndex.value].text;
  }
}

function onSuggestUp() {
  if (!showSuggestions.value) return;
  const max = suggestionList.value.length;
  if (max === 0) return;
  activeSuggestionIndex.value = (activeSuggestionIndex.value - 1 + max) % max;
  if (activeSuggestionIndex.value >= 0 && activeSuggestionIndex.value < max) {
    search.value = suggestionList.value[activeSuggestionIndex.value].text;
  }
}

function selectSuggestion(text: string) {
  search.value = text;
  showSuggestions.value = false;
  submitSearch(1);
}

function clearSearch() {
  search.value = '';
  suggestions.value = [];
  showSuggestions.value = false;
  activeSuggestionIndex.value = -1;
}

function removeRecent(text: string) {
  recentSearches.value = recentSearches.value.filter(x => x !== text);
  try {
    localStorage.setItem('openportal.recent_searches', JSON.stringify(recentSearches.value));
  } catch {}
}

function saveRecentSearch(q: string) {
  const trimmed = q.trim();
  if (!trimmed) return;
  const filtered = recentSearches.value.filter(x => x !== trimmed);
  recentSearches.value = [trimmed, ...filtered].slice(0, 10);
  try {
    localStorage.setItem('openportal.recent_searches', JSON.stringify(recentSearches.value));
  } catch {}
}

function applyCorrection(corrected: string) {
  search.value = corrected;
  submitSearch(1);
}

function getHostname(urlStr: string): string {
  try {
    return new URL(urlStr).hostname;
  } catch {
    return urlStr;
  }
}

function triggerToast(msg?: string) {
  featureToast.value = msg || t.value.featureComingSoon;
  setTimeout(() => {
    featureToast.value = '';
  }, 3500);
}

const t = computed(() => messages[locale.value]);
const errorText = computed(() => errors[locale.value][error.value] || errors[locale.value].fallback);
let controller: AbortController | undefined;
let sequence = 0;
let refreshTimer: ReturnType<typeof setInterval> | undefined;

const titles = computed(
  () =>
    ({
      login: t.value.login,
      signup: t.value.signup,
      'forgot-password': t.value.reset,
      'reset-password': t.value.reset,
      'verify-email': t.value.verify,
      account: t.value.account,
      mail: t.value.mail,
      privacy: t.value.privacy,
      terms: t.value.terms,
      about: t.value.about,
      store: t.value.store,
      advertising: t.value.advertising,
      business: t.value.business,
      howsearchworks: t.value.howSearchWorks,
      settings: t.value.settings,
    } as Record<string, string>)
);

const isSearch = computed(() => route.value === '' || route.value === 'search');

function safeReturn(value: string | null) {
  if (!value || /[\\\r\n]/.test(value)) return `/${locale.value}`;
  try {
    const url = new URL(value, location.origin);
    return url.origin === location.origin ? url.pathname + url.search : `/${locale.value}`;
  } catch {
    return `/${locale.value}`;
  }
}

function navigate(url: string, replace = false) {
  history[replace ? 'replaceState' : 'pushState'](null, '', url);
  restore();
}

function link(event: MouseEvent, url: string) {
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
  event.preventDefault();
  navigate(url);
}

function goToLogin(event: MouseEvent) {
  if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
  event.preventDefault();
  const current = location.pathname + location.search;
  navigate(`/${locale.value}/login?returnTo=${encodeURIComponent(current)}`);
}

function changeLanguage(event: Event) {
  const value = (event.target as HTMLSelectElement).value as Locale;
  localStorage.setItem('openportal.locale', value);
  navigate(`/${value}${route.value ? '/' + route.value : ''}${location.search}${location.hash}`);
}

function submitSearch(nextPage = 1) {
  if (!search.value.trim()) return;
  saveRecentSearch(search.value.trim());
  showSuggestions.value = false;
  const params = new URLSearchParams({ q: search.value.trim() });
  if (nextPage > 1) params.set('page', String(nextPage));
  if (safeSearch.value && safeSearch.value !== 'moderate') params.set('safe_search', safeSearch.value);
  navigate(`/${locale.value}/search?${params}`);
}

async function feelingLucky() {
  const query = search.value.trim() || 'OpenScan';
  saveRecentSearch(query);
  search.value = query;
  showSuggestions.value = false;
  try {
    const params = new URLSearchParams({ q: query, page: '1' });
    const data = await request(`/search?${params}`);
    if (Array.isArray(data.hits) && data.hits.length > 0 && data.hits[0].url) {
      window.location.href = data.hits[0].url;
      return;
    }
  } catch {}
  submitSearch(1);
}

async function runSearch() {
  const own = ++sequence;
  controller?.abort();
  controller = new AbortController();
  result.value = null;
  error.value = '';
  if (!search.value.trim()) {
    busy.value = false;
    return;
  }
  busy.value = true;
  try {
    const params = new URLSearchParams({ q: search.value, page: String(page.value) });
    if (safeSearch.value && safeSearch.value !== 'moderate') params.set('safe_search', safeSearch.value);
    const data = await request(`/search?${params}`, {
      signal: AbortSignal.any([controller.signal, AbortSignal.timeout(10000)]),
    });
    if (own !== sequence) return;
    if (!Array.isArray(data.hits) || (data.total !== null && !Number.isInteger(data.total))) {
      throw new APIError('SEARCH_UNAVAILABLE', 502);
    }
    result.value = {
      total: data.total,
      has_more: Boolean(data.has_more ?? data.hasMore ?? (data.total !== null && page.value * 10 < data.total)),
      elapsed_ms: typeof data.elapsed_ms === 'number' ? data.elapsed_ms : undefined,
      corrected_query:
        typeof data.corrected_query === 'string'
          ? data.corrected_query
          : typeof data.did_you_mean === 'string'
          ? data.did_you_mean
          : undefined,
      did_you_mean:
        typeof data.did_you_mean === 'string'
          ? data.did_you_mean
          : typeof data.corrected_query === 'string'
          ? data.corrected_query
          : undefined,
      hits: data.hits
        .filter((x: any) => {
          try {
            return typeof x.title === 'string' && typeof x.description === 'string' && ['https:', 'http:'].includes(new URL(x.url).protocol);
          } catch {
            return false;
          }
        })
        .map((x: any) => ({
          title: x.title,
          url: x.url,
          description: x.description,
          site_name: x.site_name,
          source: x.source,
        })),
    };
  } catch (e) {
    if (own === sequence && !controller?.signal.aborted) {
      error.value = e instanceof APIError ? e.code : 'SEARCH_UNAVAILABLE';
    }
  } finally {
    if (own === sequence) busy.value = false;
  }
}

function metadata() {
  document.documentElement.lang = locale.value;
  document.title = `${titles.value[route.value] || t.value.tagline} · OpenScan`;
  document.querySelector('meta[name="description"]')?.setAttribute('content', t.value.intro);
  document.querySelector('meta[name="robots"]')?.setAttribute('content', route.value === '' ? 'index,follow' : 'noindex,nofollow');
  document.querySelectorAll('link[rel="canonical"],link[hreflang]').forEach(x => x.remove());
  if (route.value === '') {
    const canonical = document.createElement('link');
    canonical.rel = 'canonical';
    canonical.href = `${location.origin}/${locale.value}`;
    document.head.append(canonical);
    for (const language of locales) {
      const alt = document.createElement('link');
      alt.rel = 'alternate';
      alt.hreflang = language;
      alt.href = `${location.origin}/${language}`;
      document.head.append(alt);
    }
  }
}

function restore() {
  const segments = location.pathname.split('/').filter(Boolean);
  if (!locales.includes(segments[0] as Locale)) {
    const saved = localStorage.getItem('openportal.locale') as Locale | null;
    const browser = navigator.languages.map(x => x.split('-')[0]).find(x => locales.includes(x as Locale));
    const country = (document.querySelector('meta[name="country"]')?.getAttribute('content') || '').toUpperCase();
    const accountLocale = locales.includes(user.value?.locale) ? user.value.locale : null;
    const value = (saved && locales.includes(saved) ? saved : accountLocale || browser || (country === 'TH' ? 'th' : country === 'KR' ? 'ko' : 'en')) as Locale;
    const legacy = location.pathname === '/register' ? '/signup' : location.pathname;
    return navigate(`/${value}${legacy === '/' ? '' : legacy}${location.search}${location.hash}`, true);
  }
  locale.value = segments[0] as Locale;
  route.value = segments.slice(1).join('/');
  if (route.value === 'register') return navigate(`/${locale.value}/signup${location.search}`, true);

  error.value = '';
  notice.value = '';
  password.value = '';
  confirm.value = '';
  agree.value = false;
  result.value = null;
  controller?.abort();
  ++sequence;
  busy.value = false;
  featureToast.value = '';

  token.value = new URLSearchParams(location.hash.slice(1)).get('token') || '';
  if (token.value) history.replaceState(null, '', location.pathname + location.search);
  metadata();

  if (['mail', 'account'].includes(route.value) && ready.value && !user.value) {
    return navigate(`/${locale.value}/login?${new URLSearchParams({ returnTo: location.pathname + location.search })}`, true);
  }

  if (route.value === 'search') {
    const params = new URLSearchParams(location.search);
    search.value = params.get('q') || '';
    const ss = params.get('safe_search');
    if (ss && ['strict', 'moderate', 'off'].includes(ss)) safeSearch.value = ss;
    const requested = Number(params.get('page') || 1);
    page.value = Number.isInteger(requested) && requested >= 1 && requested <= 100 ? requested : 1;
    runSearch();
  } else {
    search.value = '';
  }

  if (route.value === 'account' && user.value) displayName.value = user.value.display_name;
}

async function authSubmit() {
  busy.value = true;
  error.value = '';
  notice.value = '';
  try {
    if (['signup', 'reset-password'].includes(route.value) && password.value !== confirm.value) {
      notice.value = t.value.mismatch;
      return;
    }
    if (route.value === 'signup') {
      await mutate('/auth/signup', {
        email: email.value,
        password: password.value,
        displayName: displayName.value,
        locale: locale.value,
        agree: agree.value,
      });
      notice.value = t.value.signupNotice;
    } else if (route.value === 'login') {
      await mutate('/auth/login', { email: email.value, password: password.value });
      user.value = await session();
      navigate(safeReturn(new URLSearchParams(location.search).get('returnTo')));
    } else if (route.value === 'forgot-password') {
      await mutate('/auth/forgot-password', { email: email.value });
      notice.value = t.value.resetNotice;
    } else if (route.value === 'verify-email') {
      await mutate('/auth/verify', { token: token.value });
      token.value = '';
      notice.value = t.value.verified;
    } else if (route.value === 'reset-password') {
      await mutate('/auth/reset-password', { token: token.value, password: password.value });
      token.value = '';
      user.value = null;
      notice.value = t.value.resetDone;
    }
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
    password.value = '';
    confirm.value = '';
  }
}

async function resend() {
  busy.value = true;
  try {
    await mutate('/auth/resend', { email: email.value });
    notice.value = t.value.signupNotice;
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
  }
}

async function logout() {
  try {
    await mutate('/auth/logout', {});
    user.value = null;
    navigate(`/${locale.value}`);
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  }
}

async function saveAccount() {
  busy.value = true;
  try {
    await mutate('/account', { displayName: displayName.value, locale: locale.value }, 'PATCH');
    user.value = await session();
    notice.value = t.value.saved;
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
  }
}

async function deleteAccount() {
  busy.value = true;
  try {
    await mutate('/account', { password: password.value }, 'DELETE');
    user.value = null;
    navigate(`/${locale.value}`);
  } catch (e) {
    error.value = e instanceof APIError ? e.code : 'fallback';
  } finally {
    busy.value = false;
    password.value = '';
  }
}

function expired() {
  user.value = null;
  navigate(`/${locale.value}/login?${new URLSearchParams({ returnTo: location.pathname + location.search })}`, true);
}

watch(theme, value => {
  document.documentElement.dataset.theme = value;
  localStorage.setItem('openportal.theme', value);
});

onMounted(async () => {
  theme.value = localStorage.getItem('openportal.theme') || 'system';
  try {
    user.value = await session();
  } catch {
    error.value = 'fallback';
  }
  ready.value = true;
  restore();
  window.addEventListener('popstate', restore);
  refreshTimer = setInterval(async () => {
    if (user.value) {
      try {
        await mutate('/auth/refresh', {});
      } catch (e) {
        if (e instanceof APIError && e.status === 401) expired();
      }
    }
  }, 15 * 60 * 1000);
});

onUnmounted(() => {
  controller?.abort();
  if (refreshTimer) clearInterval(refreshTimer);
  window.removeEventListener('popstate', restore);
});
</script>

<template>
  <a class="sr-only" href="#main-content">{{ t.skip }}</a>
  <div class="portal">
    <!-- Top Bar Navigation (D-04) -->
    <header class="site-header" :class="{ 'serp-header': route === 'search' }">
      <!-- Left side: Information / Store (Home) OR Small Logo (SERP / Auth / Static) -->
      <div v-if="route === ''" class="header-left">
        <a :href="`/${locale}/about`" @click="link($event, `/${locale}/about`)">{{ t.about }}</a>
        <a :href="`/${locale}/store`" @click="link($event, `/${locale}/store`)">{{ t.store }}</a>
      </div>
      <div v-else class="header-left">
        <a :href="`/${locale}`" @click="link($event, `/${locale}`)" class="cloud-lens-logo size-sm">
          <svg class="logo-mark" viewBox="0 0 48 36" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <path d="M12 28h24a8 8 0 0 0 2-15.75A11 11 0 0 0 17 9.5 8 8 0 0 0 8 20a8 8 0 0 0 4 8z" fill="#DCEEFF" stroke="#3F8FE0" stroke-width="2.5" stroke-linejoin="round"/>
            <circle cx="23" cy="18" r="7" fill="#FFFFFF" stroke="#3F8FE0" stroke-width="2.5"/>
            <path d="M28 23l6 6" stroke="#2470C2" stroke-width="3" stroke-linecap="round"/>
          </svg>
          <span class="logo-wordmark">OpenScan</span>
        </a>
      </div>

      <!-- Center Search Bar on SERP -->
      <div v-if="route === 'search'" class="serp-search-wrap">
        <form class="search-pill" role="search" @submit.prevent="submitSearch()">
          <label class="sr-only" for="serp-query">{{ t.search }}</label>
          <input
            id="serp-query"
            v-model="search"
            type="search"
            :placeholder="t.searchPlaceholder"
            maxlength="512"
            enterkeyhint="search"
            autocomplete="off"
            @input="onSearchInput"
            @focus="onSearchFocus"
            @blur="onSearchBlur"
            @keydown.down.prevent="onSuggestDown"
            @keydown.up.prevent="onSuggestUp"
            @keydown.esc="closeSuggestions"
            required
          />
          <button v-if="search" type="button" class="clear-btn" @click="clearSearch" :title="t.clear" aria-label="Clear query">✕</button>
          <button class="tool-icon-btn" type="submit" :title="t.search" aria-label="Search">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
          </button>
          <ul v-if="showSuggestions && suggestionList.length" class="suggest-box" role="listbox">
            <li v-for="(item, idx) in suggestionList" :key="item.text" :class="{ active: idx === activeSuggestionIndex }" @mousedown.prevent="selectSuggestion(item.text)">
              <span class="suggest-icon" :class="{ recent: item.isRecent }">
                <svg v-if="item.isRecent" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
              </span>
              <span class="suggest-text">{{ item.text }}</span>
              <button v-if="item.isRecent" type="button" class="suggest-del-btn" @mousedown.stop.prevent="removeRecent(item.text)" :title="t.clear">✕</button>
            </li>
          </ul>
        </form>
      </div>

      <!-- Right side: Mail, Images, Language, Waffle Menu, Gray Login Button (D-04) -->
      <nav class="header-right" :aria-label="t.services">
        <a :href="`/${locale}/mail`" @click="link($event, `/${locale}/mail`)">{{ t.mail }}</a>
        <a :href="`/${locale}/search?q=images`" @click="link($event, `/${locale}/search?q=images`)">{{ t.images }}</a>

        <label>
          <span class="sr-only">{{ t.language }}</span>
          <select class="lang-select" :aria-label="t.language" :value="locale" @change="changeLanguage">
            <option value="ko">한국어</option>
            <option value="th">ไทย</option>
            <option value="en">English</option>
          </select>
        </label>
        <label class="theme-select-label">
          <span class="sr-only">{{ t.theme }}</span>
          <select class="lang-select" :aria-label="t.theme" v-model="theme">
            <option value="system">{{ t.system }}</option>
            <option value="light">{{ t.light }}</option>
            <option value="dark">{{ t.dark }}</option>
          </select>
        </label>

        <!-- Waffle App Menu (Services) -->
        <details class="services-menu">
          <summary class="waffle-btn" :title="t.services" :aria-label="t.services">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <circle cx="5" cy="5" r="2"/><circle cx="12" cy="5" r="2"/><circle cx="19" cy="5" r="2"/>
              <circle cx="5" cy="12" r="2"/><circle cx="12" cy="12" r="2"/><circle cx="19" cy="12" r="2"/>
              <circle cx="5" cy="19" r="2"/><circle cx="12" cy="19" r="2"/><circle cx="19" cy="19" r="2"/>
            </svg>
          </summary>
          <div class="services-popover">
            <div class="ecosystem-heading">
              <strong>OpenScan Services</strong>
              <span>Search & Discover</span>
            </div>
            <div class="ecosystem-grid">
              <a :href="`/${locale}/mail`" @click="link($event, `/${locale}/mail`)" class="eco-card live">
                <span>✉</span>
                <div>
                  <strong>Open Mail</strong>
                  <small>E2EE Webmail</small>
                </div>
              </a>
              <div class="eco-card planned" title="Search-to-Action">
                <span>✈</span>
                <div>
                  <strong>TripScanner</strong>
                  <small>Metasearch</small>
                </div>
              </div>
              <div class="eco-card planned" title="Search-to-Action">
                <span>🏷</span>
                <div>
                  <strong>PriceScanner</strong>
                  <small>Price Track</small>
                </div>
              </div>
              <div class="eco-card planned" title="Search-to-Action">
                <span>✦</span>
                <div>
                  <strong>Open AI</strong>
                  <small>RAG Intelligence</small>
                </div>
              </div>
              <div class="eco-card planned" title="Search-to-Action">
                <span>⚙</span>
                <div>
                  <strong>Developer</strong>
                  <small>API & Docs</small>
                </div>
              </div>
              <div class="eco-card planned" title="Search-to-Action">
                <span>◈</span>
                <div>
                  <strong>Workspace</strong>
                  <small>Documents</small>
                </div>
              </div>
            </div>
          </div>
        </details>

        <!-- Logged in state (D-07) or Gray Login Button (D-02, D-04) -->
        <details v-if="user" class="account-menu">
          <summary class="account-avatar-btn" :title="user.display_name || user.email">
            {{ (user.display_name || user.email || 'U').charAt(0).toUpperCase() }}
          </summary>
          <div class="account-popover">
            <p><strong>{{ user.display_name }}</strong></p>
            <p><small>{{ user.email }}</small></p>
            <hr style="border:0;border-top:1px solid var(--line);margin:10px 0"/>
            <a :href="`/${locale}/account`" @click="link($event, `/${locale}/account`)" style="display:block;padding:6px 0">{{ t.account }}</a>
            <a :href="`/${locale}/settings`" @click="link($event, `/${locale}/settings`)" style="display:block;padding:6px 0">{{ t.settings }}</a>
            <button type="button" @click="logout" style="background:transparent;border:0;color:var(--deep-sky);padding:6px 0;text-align:left;cursor:pointer">{{ t.logout }}</button>
          </div>
        </details>
        <a v-else class="btn-login-gray" :href="`/${locale}/login`" @click="goToLogin($event)">{{ t.login }}</a>
      </nav>
    </header>

    <!-- Main Viewport Router -->
    <main id="main-content">
      <!-- 1. Home Screen (D-04) -->
      <section v-if="route === ''" class="home-main">
        <h1 class="sr-only">{{ t.tagline }}</h1>
        <div class="cloud-lens-logo size-lg">
          <svg class="logo-mark" viewBox="0 0 48 36" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
            <path d="M12 28h24a8 8 0 0 0 2-15.75A11 11 0 0 0 17 9.5 8 8 0 0 0 8 20a8 8 0 0 0 4 8z" fill="#DCEEFF" stroke="#3F8FE0" stroke-width="2.5" stroke-linejoin="round"/>
            <circle cx="23" cy="18" r="7" fill="#FFFFFF" stroke="#3F8FE0" stroke-width="2.5"/>
            <path d="M28 23l6 6" stroke="#2470C2" stroke-width="3" stroke-linecap="round"/>
          </svg>
          <span class="logo-wordmark">OpenScan</span>
        </div>

        <div class="home-search-wrap">
          <form class="search-pill" role="search" @submit.prevent="submitSearch()">
            <label class="sr-only" for="query">{{ t.search }}</label>
            <span class="search-icon-left" aria-hidden="true">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
            </span>
            <input
              id="query"
              v-model="search"
              type="search"
              :aria-label="t.search"
              :placeholder="t.searchPlaceholder"
              maxlength="512"
              enterkeyhint="search"
              autocomplete="off"
              @input="onSearchInput"
              @focus="onSearchFocus"
              @blur="onSearchBlur"
              @keydown.down.prevent="onSuggestDown"
              @keydown.up.prevent="onSuggestUp"
              @keydown.esc="closeSuggestions"
              required
            />
            <button v-if="search" type="button" class="clear-btn" @click="clearSearch" :title="t.clear" aria-label="Clear query">✕</button>

            <!-- Search Tools (Right) -->
            <div class="search-tools-right">
              <button type="button" class="tool-icon-btn" :title="t.featureComingSoon" @click="triggerToast()">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><line x1="6" y1="8" x2="6" y2="8"/><line x1="10" y1="8" x2="10" y2="8"/><line x1="14" y1="8" x2="14" y2="8"/><line x1="18" y1="8" x2="18" y2="8"/><line x1="6" y1="12" x2="6" y2="12"/><line x1="10" y1="12" x2="10" y2="12"/><line x1="14" y1="12" x2="14" y2="12"/><line x1="18" y1="12" x2="18" y2="12"/><line x1="7" y1="16" x2="17" y2="16"/></svg>
              </button>
              <button type="button" class="tool-icon-btn" :title="t.featureComingSoon" @click="triggerToast()">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z"/><path d="M19 10v2a7 7 0 0 1-14 0v-2"/><line x1="12" y1="19" x2="12" y2="23"/><line x1="8" y1="23" x2="16" y2="23"/></svg>
              </button>
              <button type="button" class="tool-icon-btn" :title="t.featureComingSoon" @click="triggerToast()">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/></svg>
              </button>
              <button type="button" class="ai-chip" :title="t.aiMode" @click="triggerToast(t.aiMode + ': ' + t.featureComingSoon)">
                ✦ {{ t.aiMode }}
              </button>
            </div>

            <!-- Autocomplete / Suggestions -->
            <ul v-if="showSuggestions && suggestionList.length" class="suggest-box" role="listbox">
              <li v-for="(item, idx) in suggestionList" :key="item.text" :class="{ active: idx === activeSuggestionIndex }" @mousedown.prevent="selectSuggestion(item.text)">
                <span class="suggest-icon" :class="{ recent: item.isRecent }">
                  <svg v-if="item.isRecent" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                  <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
                </span>
                <span class="suggest-text">{{ item.text }}</span>
                <button v-if="item.isRecent" type="button" class="suggest-del-btn" @mousedown.stop.prevent="removeRecent(item.text)" :title="t.clear">✕</button>
              </li>
            </ul>
          </form>

          <p v-if="featureToast" class="feature-toast" role="status">
            ℹ {{ featureToast }}
          </p>

          <!-- 2 Buttons Under Search Bar (D-04) -->
          <div class="home-btn-group">
            <button type="button" class="btn-home-gray" @click="submitSearch()">{{ t.search }}</button>
            <button type="button" class="btn-home-gray" @click="feelingLucky()">{{ t.lucky }}</button>
          </div>
        </div>
      </section>

      <!-- 2. Search Results Screen (D-05, D-08) -->
      <section v-else-if="route === 'search'" class="serp-main" aria-live="polite" :aria-busy="busy">
        <!-- Verticals Tabs (D-08: Hidden if provider disabled) -->
        <div class="serp-tabs" role="tablist">
          <button v-if="enabledVerticals.web" type="button" class="serp-tab active" role="tab" aria-selected="true">
            {{ t.verticalAll }}
          </button>
          <button v-if="enabledVerticals.images" type="button" class="serp-tab" role="tab" aria-selected="false">
            {{ t.verticalImages }}
          </button>
          <button v-if="enabledVerticals.news" type="button" class="serp-tab" role="tab" aria-selected="false">
            {{ t.verticalNews }}
          </button>
          <button v-if="enabledVerticals.videos" type="button" class="serp-tab" role="tab" aria-selected="false">
            {{ t.verticalVideos }}
          </button>
          <button v-if="enabledVerticals.maps" type="button" class="serp-tab" role="tab" aria-selected="false">
            {{ t.verticalMaps }}
          </button>
          <button v-if="enabledVerticals.shopping" type="button" class="serp-tab" role="tab" aria-selected="false">
            {{ t.verticalShopping }}
          </button>
        </div>

        <div class="serp-content">
          <!-- Loading State -->
          <p v-if="busy" role="status" style="color:var(--muted)">{{ t.loading }}</p>

          <!-- Error State with Retry Button -->
          <div v-else-if="error" class="auth-notice-alert error" role="alert">
            <p>{{ errorText }}</p>
            <button type="button" class="btn-home-gray" style="margin-top:10px" @click="runSearch">{{ t.retry }}</button>
          </div>

          <!-- Results State -->
          <template v-else-if="result">
            <!-- Typo Correction (Did you mean) -->
            <div v-if="result.did_you_mean || result.corrected_query" class="did-you-mean-box">
              <span>{{ t.didYouMean }} </span>
              <a :href="`/${locale}/search?q=${encodeURIComponent(result.did_you_mean || result.corrected_query || '')}`" @click.prevent="applyCorrection(result.did_you_mean || result.corrected_query || '')">
                {{ result.did_you_mean || result.corrected_query }}
              </a>
            </div>

            <!-- Result Count & Execution Time -->
            <p class="serp-metrics">
              {{ result.total !== null ? `${t.resultCount} 약 ${new Intl.NumberFormat(locale).format(result.total)}건` : t.resultCount }}
              <span v-if="typeof result.elapsed_ms === 'number'">({{ (result.elapsed_ms / 1000).toFixed(2) }}초)</span>
            </p>

            <!-- 0 Results Dedicated Screen (A-07) -->
            <div v-if="!result.hits.length" class="empty-result-card">
              <p class="empty-title">{{ t.empty }}</p>
              <ul class="empty-tips-list">
                <li>{{ t.emptyTipSpelling }}</li>
                <li>{{ t.emptyTipGeneral }}</li>
                <li>{{ t.emptyTipFewer }}</li>
              </ul>
            </div>

            <!-- Result Cards List -->
            <article v-for="hit in result.hits" :key="hit.url" class="hit-card">
              <div class="hit-domain-row">
                <span class="hit-favicon-circle" aria-hidden="true">🌐</span>
                <span class="hit-domain-text">{{ getHostname(hit.url) }}</span>
              </div>
              <a :href="hit.url" class="hit-title-link" rel="noopener noreferrer">
                <h2>{{ hit.title }}</h2>
              </a>
              <p class="hit-snippet">{{ hit.description }}</p>
            </article>

            <!-- Pagination -->
            <nav v-if="result.has_more || (result.total !== null && result.total > 0)" class="serp-pagination" :aria-label="t.resultCount">
              <button v-if="page > 1" type="button" class="btn-home-gray" @click="submitSearch(page - 1)">{{ t.previous }}</button>
              <span style="font-weight:600;padding:0 8px">{{ new Intl.NumberFormat(locale).format(page) }}</span>
              <button v-if="result.has_more || (result.total !== null && page * 10 < result.total && page < 100)" type="button" class="btn-home-gray" @click="submitSearch(page + 1)">{{ t.next }}</button>
            </nav>
          </template>
        </div>
      </section>

      <!-- 3. Mail View -->
      <MailView v-else-if="route === 'mail' && user" :locale="locale" :user="user" @expired="expired" />

      <!-- 4. Dedicated Auth Pages (D-06: No Modal) -->
      <section v-else-if="['login', 'signup', 'forgot-password', 'reset-password', 'verify-email'].includes(route)" class="auth-page-wrap">
        <div class="auth-box">
          <div class="auth-header">
            <a :href="`/${locale}`" @click="link($event, `/${locale}`)" class="cloud-lens-logo size-md" style="justify-content:center">
              <svg class="logo-mark" viewBox="0 0 48 36" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                <path d="M12 28h24a8 8 0 0 0 2-15.75A11 11 0 0 0 17 9.5 8 8 0 0 0 8 20a8 8 0 0 0 4 8z" fill="#DCEEFF" stroke="#3F8FE0" stroke-width="2.5" stroke-linejoin="round"/>
                <circle cx="23" cy="18" r="7" fill="#FFFFFF" stroke="#3F8FE0" stroke-width="2.5"/>
                <path d="M28 23l6 6" stroke="#2470C2" stroke-width="3" stroke-linecap="round"/>
              </svg>
              <span class="logo-wordmark">OpenScan</span>
            </a>
            <h1>{{ titles[route] }}</h1>
          </div>

          <p v-if="!ready" role="status" style="text-align:center;color:var(--muted)">{{ t.loading }}</p>

          <form v-else class="auth-form" @submit.prevent="authSubmit">
            <div v-if="['login', 'signup', 'forgot-password'].includes(route)" class="form-group">
              <label for="auth-email">{{ t.email }}</label>
              <input id="auth-email" v-model="email" type="email" autocomplete="email" required maxlength="254" />
            </div>

            <div v-if="route === 'signup'" class="form-group">
              <label for="auth-displayName">{{ t.displayName }}</label>
              <input id="auth-displayName" v-model="displayName" autocomplete="nickname" required maxlength="80" />
            </div>

            <div v-if="['login', 'signup', 'reset-password'].includes(route)" class="form-group">
              <label for="auth-password">{{ t.password }}</label>
              <input
                id="auth-password"
                v-model="password"
                type="password"
                :autocomplete="route === 'login' ? 'current-password' : 'new-password'"
                :minlength="route === 'login' ? 1 : 12"
                maxlength="1024"
                required
              />
              <small v-if="route !== 'login'" style="color:var(--muted)">{{ t.passwordHint }}</small>
            </div>

            <div v-if="['signup', 'reset-password'].includes(route)" class="form-group">
              <label for="auth-confirm">{{ t.confirmPassword }}</label>
              <input id="auth-confirm" v-model="confirm" type="password" autocomplete="new-password" minlength="12" maxlength="1024" required />
            </div>

            <div v-if="route === 'signup'" style="display:flex;align-items:flex-start;gap:8px;font-size:0.85rem">
              <input id="agree-check" v-model="agree" type="checkbox" required style="margin-top:3px" />
              <label for="agree-check">{{ t.agree }}</label>
            </div>

            <p v-if="['verify-email', 'reset-password'].includes(route) && !token && !notice" class="auth-notice-alert error">
              {{ t.invalidLink }}
            </p>

            <button
              class="btn-primary"
              :disabled="busy || (['verify-email', 'reset-password'].includes(route) && !token)"
              type="submit"
              style="width:100%;margin-top:10px"
            >
              {{ busy ? t.loading : titles[route] }}
            </button>
          </form>

          <!-- Auth Links (Sign up, Forgot password, Back to login) -->
          <div class="auth-bottom-links">
            <template v-if="route === 'login'">
              <a :href="`/${locale}/signup`" @click="link($event, `/${locale}/signup`)">{{ t.signup }}</a>
              <a :href="`/${locale}/forgot-password`" @click="link($event, `/${locale}/forgot-password`)">{{ t.forgot }}</a>
            </template>
            <template v-else>
              <a :href="`/${locale}/login`" @click="link($event, `/${locale}/login`)">{{ t.login }}</a>
              <a :href="`/${locale}`" @click="link($event, `/${locale}`)">{{ t.backToHome }}</a>
            </template>
          </div>

          <!-- Resend option on login page -->
          <details v-if="route === 'login'" style="margin-top:16px;font-size:0.82rem">
            <summary style="cursor:pointer;color:var(--muted)">{{ t.resend }}</summary>
            <form style="display:flex;flex-direction:column;gap:8px;margin-top:8px" @submit.prevent="resend">
              <input v-model="email" type="email" required autocomplete="email" :placeholder="t.email" style="padding:8px" />
              <button class="btn-home-gray" type="submit" :disabled="busy">{{ t.resend }}</button>
            </form>
          </details>

          <p v-if="notice" role="status" class="auth-notice-alert notice">{{ notice }}</p>
          <p v-if="error" role="alert" class="auth-notice-alert error">{{ errorText }}</p>
        </div>
      </section>

      <!-- 5. Account Settings (Logged in) -->
      <section v-else-if="route === 'account' && user" class="auth-page-wrap">
        <div class="auth-box">
          <div class="auth-header">
            <h1>{{ t.account }}</h1>
            <p>{{ user.email }}</p>
          </div>
          <form class="auth-form" @submit.prevent="saveAccount">
            <div class="form-group">
              <label for="acc-name">{{ t.displayName }}</label>
              <input id="acc-name" v-model="displayName" required maxlength="80" />
            </div>
            <button class="btn-primary" :disabled="busy" type="submit">{{ t.save }}</button>
          </form>
          <hr style="border:0;border-top:1px solid var(--line);margin:24px 0" />
          <details style="font-size:0.85rem">
            <summary style="cursor:pointer;color:var(--error);font-weight:600">{{ t.deleteAccount }}</summary>
            <p style="color:var(--muted);margin:8px 0">{{ t.deleteWarning }}</p>
            <form class="auth-form" @submit.prevent="deleteAccount">
              <div class="form-group">
                <label for="del-pass">{{ t.password }}</label>
                <input id="del-pass" v-model="password" type="password" autocomplete="current-password" required />
              </div>
              <button class="btn-home-gray" style="color:var(--error)" :disabled="busy" type="submit">{{ t.deleteAccount }}</button>
            </form>
          </details>
          <p v-if="notice" role="status" class="auth-notice-alert notice">{{ notice }}</p>
          <p v-if="error" role="alert" class="auth-notice-alert error">{{ errorText }}</p>
        </div>
      </section>

      <!-- 6. Static Pages (OS-07) -->
      <section v-else-if="['about', 'store', 'advertising', 'business', 'howsearchworks', 'settings', 'privacy', 'terms'].includes(route)" class="static-page-wrap">
        <h1>{{ titles[route] }}</h1>
        <template v-if="route === 'about'">
          <p>{{ t.aboutBody }}</p>
        </template>
        <template v-else-if="route === 'store'">
          <p>{{ t.storeBody }}</p>
        </template>
        <template v-else-if="route === 'advertising'">
          <p>{{ t.advertisingBody }}</p>
        </template>
        <template v-else-if="route === 'business'">
          <p>{{ t.businessBody }}</p>
        </template>
        <template v-else-if="route === 'howsearchworks'">
          <p>{{ t.howSearchWorksBody }}</p>
        </template>
        <template v-else-if="route === 'settings'">
          <p>{{ t.settingsBody }}</p>
          <div style="margin-top:20px;display:flex;flex-direction:column;gap:14px">
            <label style="display:flex;align-items:center;gap:12px">
              <strong>{{ t.safeSearch }}:</strong>
              <select v-model="safeSearch" class="lang-select">
                <option value="moderate">{{ t.safeSearchModerate }}</option>
                <option value="strict">{{ t.safeSearchStrict }}</option>
                <option value="off">{{ t.safeSearchOff }}</option>
              </select>
            </label>
            <label style="display:flex;align-items:center;gap:12px">
              <strong>{{ t.language }}:</strong>
              <select class="lang-select" :value="locale" @change="changeLanguage">
                <option value="ko">한국어</option>
                <option value="th">ไทย</option>
                <option value="en">English</option>
              </select>
            </label>
          </div>
        </template>
        <template v-else-if="route === 'privacy'">
          <p style="color:var(--muted)">{{ t.policyDraft }}</p>
          <p>{{ t.privacyBody }}</p>
        </template>
        <template v-else-if="route === 'terms'">
          <p style="color:var(--muted)">{{ t.policyDraft }}</p>
          <p>{{ t.termsBody }}</p>
        </template>
      </section>

      <!-- 7. Fallback Loading / 404 -->
      <section v-else-if="!ready" class="static-page-wrap" role="status">
        <p>{{ t.loading }}</p>
      </section>
      <section v-else class="static-page-wrap">
        <h1>{{ t.notFound }}</h1>
        <a :href="`/${locale}`" @click="link($event, `/${locale}`)">{{ t.backHome }}</a>
      </section>
    </main>

    <!-- Two-Tier Gray Footer (D-04, OS-07) -->
    <footer class="site-footer">
      <div class="footer-top-tier">
        <span>{{ t.region }}</span>
      </div>
      <div class="footer-bottom-tier">
        <div class="footer-links-left">
          <a :href="`/${locale}/about`" @click="link($event, `/${locale}/about`)">{{ t.about }}</a>
          <a :href="`/${locale}/advertising`" @click="link($event, `/${locale}/advertising`)">{{ t.advertising }}</a>
          <a :href="`/${locale}/business`" @click="link($event, `/${locale}/business`)">{{ t.business }}</a>
          <a :href="`/${locale}/howsearchworks`" @click="link($event, `/${locale}/howsearchworks`)">{{ t.howSearchWorks }}</a>
        </div>
        <div class="footer-links-right">
          <a :href="`/${locale}/privacy`" @click="link($event, `/${locale}/privacy`)">{{ t.privacy }}</a>
          <a :href="`/${locale}/terms`" @click="link($event, `/${locale}/terms`)">{{ t.terms }}</a>
          <a :href="`/${locale}/settings`" @click="link($event, `/${locale}/settings`)">{{ t.settings }}</a>
        </div>
      </div>
    </footer>
  </div>
</template>

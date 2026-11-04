<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import MailView from './MailView.vue';
import SearchBox from './SearchBox.vue';
import BrandMark from './BrandMark.vue';
import ScanArt from './ScanArt.vue';
import { designMessages } from './design-copy';
import { APIError, mutate, request, session } from './client';
import { errors, locales, messages, folderLabels, type Locale } from './i18n';

const locale = ref<Locale>('en');
const route = ref('');
const search = ref('');
const page = ref(1);
const safeSearch = ref('moderate');
const result = ref<{
  total: number | null;
  has_more: boolean;
  elapsed_ms?: number;
  corrected_query?: string;
  did_you_mean?: string;
  source?: string;
  degraded?: boolean;
  hits: { title: string; url: string; description: string; site_name?: string; source?: string }[];
} | null>(null);

const busy = ref(false);
const error = ref('');
const notice = ref('');
const user = ref<any>(null);
const ready = ref(false);
const sessionUnavailable = ref(false);
const showPassword = ref(false);
const mailDraftDirty = ref(false);
let navigationIndex = Number.isInteger(history.state?.portalIndex) ? history.state.portalIndex : 0;
let currentAddress = location.pathname + location.search + location.hash;
let undoingNavigation = false;
history.replaceState({ ...history.state, portalIndex: navigationIndex }, '', location.href);

// Auth inputs
const email = ref('');
const password = ref('');
const confirm = ref('');
const displayName = ref('');
const agree = ref(false);
const token = ref('');
const sentTo = ref('');
const theme = ref('light');

const authRoutes = ['login', 'signup', 'forgot-password', 'reset-password', 'verify-email'];
const isAuthRoute = computed(() => authRoutes.includes(route.value));
const needsNewPassword = computed(() => route.value === 'signup' || route.value === 'reset-password');
// Advisory only: the gateway enforces the real policy (12+ characters, <= 1024 bytes).
const passwordScore = computed(() => {
  const value = password.value;
  if (!value) return -1;
  if (value.length < 12) return 0;
  const variety = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter(pattern => pattern.test(value)).length;
  return 1 + (variety >= 3 ? 1 : 0) + (variety >= 3 && value.length >= 16 ? 1 : 0);
});
const passwordVaried = computed(() => [/[A-Za-z]/, /\d/, /[^A-Za-z0-9]/].every(pattern => pattern.test(password.value)));
const strengthLabel = computed(() => [copy.value.strengthWeak, copy.value.strengthFair, copy.value.strengthGood, copy.value.strengthStrong][passwordScore.value] || '');
const authReturn = computed(() => { void route.value; const target = new URLSearchParams(location.search).get('returnTo'); return target ? '?' + new URLSearchParams({ returnTo: target }) : ''; });
const confirmState = computed(() => !confirm.value ? '' : confirm.value === password.value ? 'match' : 'differ');

// Search history is opt-in; preferences stay on this device.
function stored(key: string, fallback: string) { try { return localStorage.getItem(key) ?? fallback; } catch { return fallback; } }
function persist(key: string, value: string) { try { localStorage.setItem(key, value); } catch {} }
const recentOptIn = ref(stored('openportal.recent_opt_in', 'false') === 'true');
const recentSearches = ref<string[]>([]);
if (recentOptIn.value) {
  try { const value = JSON.parse(stored('openportal.recent_searches', '[]')); if (Array.isArray(value)) recentSearches.value = value.filter(x => typeof x === 'string').slice(0, 10); } catch {}
}
safeSearch.value = stored('openportal.safe_search', 'moderate');
if (!['strict', 'moderate', 'off'].includes(safeSearch.value)) safeSearch.value = 'moderate';
const preferredSafeSearch = ref(safeSearch.value);
function changeSafeSearch(event: Event) {
  const value=(event.target as HTMLSelectElement).value;
  if(!['strict','moderate','off'].includes(value))return;
  preferredSafeSearch.value=value;safeSearch.value=value;persist('openportal.safe_search',value);
  if(route.value==='search')submitSearch(1);
}
function removeRecent(text: string) {
  recentSearches.value = recentSearches.value.filter(x => x !== text);
  if (recentOptIn.value) persist('openportal.recent_searches', JSON.stringify(recentSearches.value));
}
function clearHistory() {
  recentSearches.value = [];
  try { localStorage.removeItem('openportal.recent_searches'); } catch {}
}
function saveRecentSearch(q: string) {
  const value = q.trim();
  if (!recentOptIn.value || !value) return;
  recentSearches.value = [value, ...recentSearches.value.filter(x => x !== value)].slice(0, 10);
  persist('openportal.recent_searches', JSON.stringify(recentSearches.value));
}
watch(recentOptIn, value => { persist('openportal.recent_opt_in', String(value)); if (!value) clearHistory(); });
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


const t = computed(() => messages[locale.value]);
const copy = computed(() => designMessages[locale.value]);
const authTitle = computed(() => route.value === 'login' ? copy.value.loginTitle : route.value === 'signup' ? copy.value.signupTitle : titles.value[route.value]);
const authBenefits = computed(() => [copy.value.benefitMail, copy.value.benefitVerify, copy.value.benefitHash, copy.value.benefitHistory]);
const authIntro = computed(() => route.value === 'login' ? copy.value.loginIntro : route.value === 'signup' ? copy.value.signupIntro : route.value === 'forgot-password' ? copy.value.forgotIntro : route.value === 'verify-email' ? copy.value.verifyIntro : route.value === 'reset-password' ? copy.value.resetIntro : copy.value.authIntro);
const sourceLabel = computed(() => result.value?.source === 'brave-api' || result.value?.source === 'bing-api' ? copy.value.sourceExternal : result.value?.source === 'hybrid-rrf' ? copy.value.sourceHybrid : copy.value.sourceIndex);
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
  if (!canLeaveMail(url)) return false;
  if (!replace) navigationIndex++;
  history[replace ? 'replaceState' : 'pushState']({ ...history.state, portalIndex: navigationIndex }, '', url);
  currentAddress = location.pathname + location.search + location.hash;
  restore();
  return true;
}

function canLeaveMail(url: string) {
  const destination = new URL(url, location.origin);
  if (!mailDraftDirty.value || route.value !== 'mail' || /^\/(ko|th|en)\/mail\/?$/.test(destination.pathname)) return true;
  return window.confirm(folderLabels[locale.value].discard);
}

function onHistoryNavigation() {
  if (undoingNavigation) { undoingNavigation = false; return; }
  const requestedIndex = history.state?.portalIndex;
  if (!canLeaveMail(location.href)) {
    if (Number.isInteger(requestedIndex) && requestedIndex !== navigationIndex) {
      undoingNavigation = true;
      history.go(navigationIndex - requestedIndex);
    } else history.pushState({ ...history.state, portalIndex: navigationIndex }, '', currentAddress);
    return;
  }
  if (Number.isInteger(requestedIndex)) navigationIndex = requestedIndex;
  currentAddress = location.pathname + location.search + location.hash;
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

  const params = new URLSearchParams({ q: search.value.trim() });
  if (nextPage > 1) params.set('page', String(nextPage));
  if (safeSearch.value && safeSearch.value !== 'moderate') params.set('safe_search', safeSearch.value);
  navigate(`/${locale.value}/search?${params}`);
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
      source: typeof data.source === 'string' ? data.source : undefined,
      degraded: data.degraded === true,
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
  document.title = `${titles.value[route.value] || t.value.tagline} · OpenScanner`;
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
  showPassword.value = false;
  sentTo.value = '';
  confirm.value = '';
  agree.value = false;
  result.value = null;
  controller?.abort();
  ++sequence;
  busy.value = false;


  token.value = new URLSearchParams(location.hash.slice(1)).get('token') || '';
  if (token.value) history.replaceState({ ...history.state }, '', location.pathname + location.search);
  currentAddress = location.pathname + location.search + location.hash;
  metadata();

  if (['mail', 'account'].includes(route.value) && ready.value && !user.value) {
    return navigate(`/${locale.value}/login?${new URLSearchParams({ returnTo: location.pathname + location.search })}`, true);
  }

  if (route.value === 'search') {
    const params = new URLSearchParams(location.search);
    search.value = params.get('q') || '';
    const ss = params.get('safe_search');
    safeSearch.value = ss && ['strict', 'moderate', 'off'].includes(ss) ? ss : preferredSafeSearch.value;
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
      sentTo.value = email.value;
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
  if (!canLeaveMail(`/${locale.value}`)) return;
  try {
    await mutate('/auth/logout', {});
    mailDraftDirty.value = false;
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
  const destination=`/${locale.value}/login?${new URLSearchParams({ returnTo: location.pathname + location.search })}`;
  if (!canLeaveMail(destination)) return;
  mailDraftDirty.value = false;
  user.value = null;
  navigate(destination, true);
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
    sessionUnavailable.value = true;
  }
  ready.value = true;
  restore();
  window.addEventListener('popstate', onHistoryNavigation);
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
  window.removeEventListener('popstate', onHistoryNavigation);
});
</script>

<template>
  <a class="skip-link" href="#main-content">{{t.skip}}</a>
  <div class="portal">
    <header class="site-header" :class="{'serp-header':route==='search'}">
      <a class="brand" :href="`/${locale}`" @click="link($event,`/${locale}`)" aria-label="OpenScanner">
        <BrandMark/>
        <span class="brand-word">Open<b>Scanner</b></span>
      </a>
      <div v-if="route==='search'" class="header-search"><SearchBox id="header-query" v-model="search" :locale="locale" :recent="recentSearches" :busy="busy" @submit="submitSearch()" @remove-recent="removeRecent"/></div>
      <nav class="header-nav" :aria-label="t.services">
        <details class="services-menu"><summary>{{t.services}}<svg class="chevron" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="m4 6 4 4 4-4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg></summary><div class="services-popover"><a :href="`/${locale}/mail`" @click="link($event,`/${locale}/mail`)"><span aria-hidden="true">✉</span>{{t.mail}}</a></div></details>
        <label class="language-control"><span class="sr-only">{{t.language}}</span><select :aria-label="t.language" :value="locale" @change="changeLanguage"><option value="ko">한국어</option><option value="th">ไทย</option><option value="en">English</option></select></label>
        <label class="theme-control"><span class="sr-only">{{t.theme}}</span><select :aria-label="t.theme" v-model="theme"><option value="system">{{t.system}}</option><option value="light">{{t.light}}</option><option value="dark">{{t.dark}}</option></select></label>
        <details v-if="user" class="account-menu"><summary class="account-trigger"><span class="avatar" aria-hidden="true">{{user.display_name?.slice(0,1)}}</span><span class="sr-only">{{t.account}}</span></summary><div class="services-popover"><p class="account-email">{{user.email}}</p><a :href="`/${locale}/account`" @click="link($event,`/${locale}/account`)">{{t.account}}</a><button @click="logout">{{t.logout}}</button></div></details>
        <a v-else-if="!isAuthRoute" class="button primary signin":href="`/${locale}/login?returnTo=${encodeURIComponent(currentAddress.split('#')[0])}`" @click="goToLogin">{{t.login}}</a>
      </nav>
    </header>

    <main id="main-content" :class="{'home-main':route==='','mail-main':route==='mail','search-main':route==='search','auth-main':isAuthRoute}">
      <section v-if="route===''" class="home-grid">
        <div class="hero">
          <p class="eyebrow slogan"><span class="status-dot" aria-hidden="true"></span> {{t.slogan}}</p>
          <h1>{{t.tagline}}</h1><p class="hero-intro">{{copy.homeIntro}}</p>
          <SearchBox id="home-query" v-model="search" :locale="locale" :recent="recentSearches" @submit="submitSearch()" @remove-recent="removeRecent"/>
          <p class="search-help">{{copy.searchHelp}}</p>
          <div v-if="recentOptIn" class="recent-searches"><p class="eyebrow">{{copy.recent}}</p><p v-if="!recentSearches.length" class="muted">{{copy.noHistory}}</p><button v-for="recent in recentSearches" :key="recent" class="chip" @click="search=recent;submitSearch()">↶ {{recent}}</button></div>
        </div>
        <aside class="home-aside">
          <ScanArt caption="SEARCH · DISCOVER · DO"/>
          <h2>{{copy.homeAside}}</h2>
          <a class="service-card" :href="`/${locale}/mail`" @click="link($event,`/${locale}/mail`)"><span class="service-icon" aria-hidden="true">✉</span><span><strong>{{t.mail}}</strong><small>{{copy.mailDescription}}</small></span><span aria-hidden="true">↗</span></a>
          <a class="service-card" :href="`/${locale}/settings`" @click="link($event,`/${locale}/settings`)"><span class="service-icon" aria-hidden="true">☷</span><span><strong>{{t.settings}}</strong><small>{{copy.settingsDescription}}</small></span><span aria-hidden="true">↗</span></a>
          <a class="service-card" :href="`/${locale}/privacy`" @click="link($event,`/${locale}/privacy`)"><span class="service-icon" aria-hidden="true">◇</span><span><strong>{{t.privacy}}</strong><small>{{copy.privacyDescription}}</small></span><span aria-hidden="true">↗</span></a>
        </aside>
      </section>

      <section v-else-if="route==='search'" class="serp-shell" :aria-busy="busy">
        <div class="search-toolbar"><div><p class="eyebrow">{{copy.web}}</p><h1>{{copy.resultsTitle}}</h1></div><label class="safe-control">{{t.safeSearch}}<select :aria-label="t.safeSearch" :value="safeSearch" @change="changeSafeSearch"><option value="moderate">{{t.safeSearchModerate}}</option><option value="strict">{{t.safeSearchStrict}}</option><option value="off">{{t.safeSearchOff}}</option></select></label></div>
        <div v-if="busy" class="search-state loading-state" role="status"><span class="loader" aria-hidden="true"></span><p>{{t.loading}}</p><div v-for="item in 3" :key="item" class="skeleton" aria-hidden="true"><i></i><b></b><span></span></div></div>
        <div v-else-if="error" class="search-state error-card" role="alert"><span class="state-symbol" aria-hidden="true">!</span><h2>{{t.searchErrorTitle}}</h2><p>{{errorText}}</p><div class="actions"><button class="button primary" @click="runSearch">{{t.retry}}</button><a class="button" :href="`/${locale}`" @click="link($event,`/${locale}`)">{{t.backHome}}</a></div></div>
        <template v-else-if="result">
          <div class="results-meta"><span>{{result.total===null?copy.unknownCount:`${t.resultCount} ${new Intl.NumberFormat(locale).format(result.total)}`}}</span><span class="source-badge">{{sourceLabel}}</span></div>
          <p v-if="result.degraded" class="notice" role="status">{{copy.partial}}</p>
          <p v-if="result.corrected_query&&result.corrected_query!==search" class="correction">{{t.didYouMean}} <button class="text-button" @click="applyCorrection(result.corrected_query)">{{result.corrected_query}}</button></p>
          <div v-if="!result.hits.length" class="search-state empty-state"><span class="state-symbol" aria-hidden="true">⌕</span><h2>{{t.empty}}</h2><ul><li>{{t.emptyTipSpelling}}</li><li>{{t.emptyTipGeneral}}</li><li>{{t.emptyTipFewer}}</li></ul></div>
          <div v-else class="results-list"><article v-for="hit in result.hits" :key="hit.url" class="hit"><a class="result-link" :href="hit.url" rel="noopener noreferrer"><span class="result-domain"><span class="domain-mark" aria-hidden="true">{{getHostname(hit.url).slice(0,1).toUpperCase()}}</span>{{getHostname(hit.url)}}</span><h2>{{hit.title}}</h2></a><p class="result-url">{{hit.url}}</p><p class="result-description">{{hit.description}}</p></article></div>
          <nav v-if="result.hits.length||page>1" class="pagination" :aria-label="t.resultCount"><button v-if="page>1" class="button" @click="submitSearch(page-1)">← {{t.previous}}</button><span>{{new Intl.NumberFormat(locale).format(page)}}</span><button v-if="result.has_more&&page<100" class="button" @click="submitSearch(page+1)">{{t.next}} →</button></nav>
          <p class="source-note">{{copy.sourceNote}}</p>
        </template>
      </section>

      <MailView v-else-if="route==='mail'&&user" :locale="locale" :user="user" @expired="expired" @dirty-change="mailDraftDirty=$event"/>

      <section v-else-if="isAuthRoute" class="auth-shell">
        <aside class="auth-visual">
          <div class="auth-visual-copy">
            <p class="auth-visual-brand"><BrandMark/><span>OpenScanner</span></p>
            <h2>{{t.slogan}}</h2>
            <ul class="auth-benefits">
              <li v-for="item in authBenefits" :key="item"><svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="m5 10.5 3 3 7-7" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>{{item}}</li>
            </ul>
          </div>
          <ScanArt/>
        </aside>

        <div class="auth-card">
          <nav v-if="route==='login'||route==='signup'" class="auth-tabs" :aria-label="copy.authTabs">
            <a :href="`/${locale}/login${authReturn}`" :aria-current="route==='login'?'page':undefined" @click="link($event,`/${locale}/login${authReturn}`)">{{t.login}}</a>
            <a :href="`/${locale}/signup${authReturn}`" :aria-current="route==='signup'?'page':undefined" @click="link($event,`/${locale}/signup${authReturn}`)">{{t.signup}}</a>
          </nav>
          <a v-else class="auth-back" :href="`/${locale}/login`" @click="link($event,`/${locale}/login`)"><span aria-hidden="true">←</span> {{copy.backToLogin}}</a>

          <div v-if="route==='signup'&&sentTo" class="auth-sent">
            <span class="auth-sent-icon" aria-hidden="true"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round"><rect x="3" y="5" width="18" height="14" rx="3"/><path d="m4 7 8 6 8-6"/></svg></span>
            <h1>{{copy.checkInbox}}</h1>
            <p role="status">{{notice}}</p>
            <p class="auth-sent-to"><span>{{copy.sentTo}}</span><strong>{{sentTo}}</strong></p>
            <a class="button primary full-width" :href="`/${locale}/login`" @click="link($event,`/${locale}/login`)">{{copy.backToLogin}}</a>
            <button type="button" class="text-button" @click="sentTo='';notice=''">{{copy.useDifferentEmail}}</button>
          </div>

          <template v-else>
            <header class="auth-header"><h1>{{authTitle}}</h1><p>{{authIntro}}</p></header>
            <p v-if="!ready" role="status">{{t.loading}}</p>
            <form v-else class="auth-form" @submit.prevent="authSubmit">
              <div v-if="['login','signup','forgot-password'].includes(route)" class="form-group"><label for="auth-email">{{t.email}}</label><input id="auth-email" v-model="email" type="email" autocomplete="email" inputmode="email" maxlength="254" required/></div>
              <div v-if="route==='signup'" class="form-group"><label for="auth-name">{{t.displayName}}</label><input id="auth-name" v-model="displayName" autocomplete="nickname" maxlength="80" required/></div>
              <div v-if="['login','signup','reset-password'].includes(route)" class="form-group">
                <div class="label-row"><label for="auth-password">{{t.password}}</label><a v-if="route==='login'" class="forgot-link" :href="`/${locale}/forgot-password`" @click="link($event,`/${locale}/forgot-password`)">{{t.forgot}}</a></div>
                <div class="password-field">
                  <input id="auth-password" v-model="password" :type="showPassword?'text':'password'" :autocomplete="route==='login'?'current-password':'new-password'" :minlength="route==='login'?1:12" maxlength="1024" :aria-describedby="needsNewPassword?'password-help':undefined" required/>
                  <button type="button" class="icon-button" :aria-label="showPassword?copy.hidePassword:copy.showPassword" :aria-pressed="showPassword" @click="showPassword=!showPassword">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z"/><circle cx="12" cy="12" r="3"/><path v-if="showPassword" d="m4 4 16 16"/></svg>
                  </button>
                </div>
                <div v-if="needsNewPassword" id="password-help" class="password-help">
                  <div v-if="password" class="strength" :data-score="passwordScore"><span class="strength-bar" aria-hidden="true"><i v-for="n in 3" :key="n" :class="{on:passwordScore>=n}"></i></span><span>{{copy.strength}}: <strong>{{strengthLabel}}</strong></span></div>
                  <ul class="requirements">
                    <li :class="{met:password.length>=12}"><span aria-hidden="true">{{password.length>=12?'✓':'•'}}</span>{{copy.reqLength}}</li>
                    <li :class="{met:passwordVaried}"><span aria-hidden="true">{{passwordVaried?'✓':'•'}}</span>{{copy.reqVariety}}</li>
                  </ul>
                </div>
              </div>
              <div v-if="needsNewPassword" class="form-group"><label for="auth-confirm">{{t.confirmPassword}}</label><input id="auth-confirm" v-model="confirm" :type="showPassword?'text':'password'" autocomplete="new-password" minlength="12" maxlength="1024" aria-describedby="confirm-help" required/><small id="confirm-help" class="match-hint" :class="confirmState" aria-live="polite">{{confirmState==='match'?'✓ '+copy.passwordsMatch:confirmState==='differ'?copy.passwordsDiffer:''}}</small></div>
              <label v-if="route==='signup'" class="checkbox-row"><input v-model="agree" type="checkbox" required/><span>{{t.agree}} <a :href="`/${locale}/terms`" target="_blank" rel="noopener">{{t.terms}}</a> · <a :href="`/${locale}/privacy`" target="_blank" rel="noopener">{{t.privacy}}</a></span></label>
              <p v-if="['verify-email','reset-password'].includes(route)&&!token&&!notice" class="notice">{{t.invalidLink}}</p>
              <button class="button primary full-width auth-submit" type="submit" :disabled="busy||(['verify-email','reset-password'].includes(route)&&!token)">{{busy?t.loading:titles[route]}}</button>
            </form>
            <details v-if="route==='login'" class="resend-panel"><summary>{{t.resend}}</summary><form @submit.prevent="resend"><label for="resend-email">{{t.email}}</label><input id="resend-email" v-model="email" type="email" autocomplete="email" required/><button class="button" :disabled="busy">{{t.resend}}</button></form></details>
            <p v-if="notice" class="notice" role="status">{{notice}}</p><p v-if="error" class="alert" role="alert">{{errorText}}</p><p v-else-if="sessionUnavailable" class="source-note">{{copy.authUnavailable}}</p>
          </template>
        </div>
      </section>

      <section v-else-if="route==='account'&&user" class="account-panel"><div class="page-heading"><p class="eyebrow">OPENSCANNER / {{t.account}}</p><h1>{{t.account}}</h1><p>{{user.email}}</p></div><form class="auth-form" @submit.prevent="saveAccount"><div class="form-group"><label for="account-name">{{t.displayName}}</label><input id="account-name" v-model="displayName" maxlength="80" required/></div><button class="button primary" :disabled="busy">{{t.save}}</button></form><hr/><details class="danger-panel"><summary>{{t.deleteAccount}}</summary><p>{{t.deleteWarning}}</p><form class="auth-form" @submit.prevent="deleteAccount"><label for="delete-password">{{t.password}}</label><input id="delete-password" v-model="password" type="password" autocomplete="current-password" required/><button class="button danger" :disabled="busy">{{t.deleteAccount}}</button></form></details><p v-if="notice" class="notice" role="status">{{notice}}</p><p v-if="error" class="alert" role="alert">{{errorText}}</p></section>

      <section v-else-if="route==='settings'" class="settings-page"><div class="page-heading"><p class="eyebrow">OPENSCANNER / {{t.settings}}</p><h1>{{t.settings}}</h1><p>{{copy.settingsIntro}}</p></div><div class="settings-card"><label class="setting-row"><span><strong>{{t.language}}</strong></span><select :aria-label="t.language" :value="locale" @change="changeLanguage"><option value="ko">한국어</option><option value="th">ไทย</option><option value="en">English</option></select></label><label class="setting-row"><span><strong>{{t.theme}}</strong></span><select :aria-label="t.theme" v-model="theme"><option value="system">{{t.system}}</option><option value="light">{{t.light}}</option><option value="dark">{{t.dark}}</option></select></label><label class="setting-row"><strong>{{t.safeSearch}}</strong><select :aria-label="t.safeSearch" :value="safeSearch" @change="changeSafeSearch"><option value="moderate">{{t.safeSearchModerate}}</option><option value="strict">{{t.safeSearchStrict}}</option><option value="off">{{t.safeSearchOff}}</option></select></label><label class="setting-row history-setting"><span><strong>{{copy.rememberSearches}}</strong><small>{{copy.historyHint}}</small></span><input v-model="recentOptIn" type="checkbox"/></label><button class="button" @click="clearHistory">{{copy.clearHistory}}</button><p class="source-note">{{copy.preferencesSaved}}</p></div></section>

      <section v-else-if="['about','howsearchworks','privacy','terms'].includes(route)" class="policy-page"><p class="eyebrow">OPENSCANNER</p><h1>{{titles[route]}}</h1><template v-if="route==='privacy'||route==='terms'"><p class="notice">{{t.policyDraft}}</p><p>{{route==='privacy'?t.privacyBody:t.termsBody}}</p></template><p v-else>{{route==='about'?copy.aboutBody:copy.howBody}}</p></section>
      <section v-else-if="!ready" class="search-state" role="status">{{t.loading}}</section>
      <section v-else class="search-state not-found"><p class="eyebrow">404 / OPENSCANNER</p><h1>{{t.notFound}}</h1><a class="button primary" :href="`/${locale}`" @click="link($event,`/${locale}`)">{{t.backHome}}</a></section>
    </main>

    <footer class="site-footer"><span class="footer-brand"><BrandMark/><span>OpenScanner</span><span class="footer-divider">/</span><span>{{t.slogan}}</span></span><nav :aria-label="t.about"><a :href="`/${locale}/about`" @click="link($event,`/${locale}/about`)">{{t.about}}</a><a :href="`/${locale}/privacy`" @click="link($event,`/${locale}/privacy`)">{{t.privacy}}</a><a :href="`/${locale}/terms`" @click="link($event,`/${locale}/terms`)">{{t.terms}}</a><a :href="`/${locale}/settings`" @click="link($event,`/${locale}/settings`)">{{t.settings}}</a></nav></footer>
  </div>
</template>

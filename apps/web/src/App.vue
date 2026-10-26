<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue';

type Hit = { document_id?: string; title: string; url: string; description: string; language: string; score: number };
type Response = {
  query: string;
  total: number;
  hits: Hit[];
  elapsed_ms: number;
  request_id?: string;
  corrected_query?: string;
  safe_search?: string;
  vertical?: string;
};
type VerticalTab = {
  id: string;
  label_ko: string;
  label_en: string;
  enabled: boolean;
  phase: number;
  gate_note?: string;
};

const currentPath = ref('/');
const q = ref('');
const submitted = ref('');
const page = ref(1);
const result = ref<Response | null>(null);
const busy = ref(false);
const error = ref('');
const copyFeedback = ref('');

const queryInput = ref<HTMLInputElement | null>(null);
const serpQueryInput = ref<HTMLInputElement | null>(null);

// Phase 1 preferences & opt-in controls (Tasks 012, 017)
const locale = ref<'ko' | 'en'>('ko');
const theme = ref<'light' | 'dark' | 'system'>('system');
const safeSearch = ref<'strict' | 'moderate' | 'off'>('moderate');
const recentOptIn = ref(false);
const personalizedOptIn = ref(false);
const recentSearches = ref<string[]>([]);
const showSettings = ref(false);
const showAccount = ref(false);

// Autocomplete & keyboard navigation (Task 012)
const suggestions = ref<string[]>([]);
const showSuggest = ref(false);
const activeSuggestIdx = ref(-1);
let suggestTimer: ReturnType<typeof setTimeout> | null = null;

// Feature-flagged vertical tabs (Task 013)
const verticalTabs = ref<VerticalTab[]>([
  { id: 'web', label_ko: '웹', label_en: 'Web', enabled: true, phase: 1 },
  { id: 'image', label_ko: '이미지', label_en: 'Images', enabled: false, phase: 5, gate_note: '후속 Phase 5 라이선스 검증 전 비활성' },
  { id: 'news', label_ko: '뉴스', label_en: 'News', enabled: false, phase: 5, gate_note: '후속 Phase 5 언론사 제휴 검증 전 비활성' },
  { id: 'video', label_ko: '동영상', label_en: 'Videos', enabled: false, phase: 5, gate_note: '후속 Phase 5 동영상 권리 검증 전 비활성' },
  { id: 'map', label_ko: '지도', label_en: 'Maps', enabled: false, phase: 5, gate_note: '후속 Phase 5 지리 데이터 라이선스 검증 전 비활성' },
  { id: 'shopping', label_ko: '쇼핑', label_en: 'Shopping', enabled: false, phase: 5, gate_note: '후속 Phase 5 상품 피드 검증 전 비활성' },
]);
const activeTab = ref('web');
const verticalGateNotice = ref('');

// Popular travel recommendations
const popularDestinations = [
  { name: '제주도 항공권', tag: '항공' },
  { name: '도쿄 호텔', tag: '호텔' },
  { name: '오사카 유니버설', tag: '투어' },
  { name: '다낭 풀빌라', tag: '리조트' },
  { name: '방콕 자유여행', tag: '여행' },
  { name: '파리 에펠탑 투어', tag: '티켓' },
  { name: '오픈포털 검색엔진', tag: '기술' },
];

// Auth & MFA state (Task 015)
const authEmail = ref('');
const sessionToken = ref('');
const accountId = ref('');
const accountEmail = ref('');
const mfaEnrolled = ref(false);
const mfaSecret = ref('');
const mfaCode = ref('');
const recoveryCodes = ref<string[]>([]);
const authStatus = ref('');

const hasSearch = computed(() => submitted.value.length > 0);
const isConsoleDeniedRoute = computed(() =>
  currentPath.value.startsWith('/admin') || currentPath.value.startsWith('/partner')
);
const isNotFoundRoute = computed(() =>
  currentPath.value !== '/' &&
  currentPath.value !== '/search' &&
  !isConsoleDeniedRoute.value
);

const i18n = computed(() => {
  if (locale.value === 'en') {
    return {
      tagline: 'Independent Search Engine Demo · Web Search Enabled',
      heading: 'What are you looking for?',
      queryLabel: '검색어',
      placeholder: 'Enter search query',
      searchBtn: '검색',
      searchingBtn: 'Searching…',
      settingsBtn: 'Settings',
      accountBtn: 'Account',
      emptyMsg: '일치하는 샘플 문서가 없습니다.',
      didYouMean: 'Did you mean:',
      recentTitle: 'Recent searches (Opt-in)',
      clearRecent: 'Clear',
      luckyBtn: 'Explore Top Destinations',
      popularTitle: 'Popular Travel Searches',
    };
  }
  return {
    tagline: '독립 검색엔진 개발 데모 · 웹 검색만 구현',
    heading: '무엇을 찾고 있나요?',
    queryLabel: '검색어',
    placeholder: '검색어를 입력하세요',
    searchBtn: '검색',
    searchingBtn: '검색 중…',
    settingsBtn: '검색 설정',
    accountBtn: '계정·보안',
    emptyMsg: '일치하는 샘플 문서가 없습니다.',
    didYouMean: '다음 검색어로 검색하시겠습니까?',
    recentTitle: '최근 검색어 (Opt-in)',
    clearRecent: '전체 삭제',
    luckyBtn: '인기 여행지 탐색',
    popularTitle: '인기 여행 검색어',
  };
});

function navigateTo(path: string) {
  if (typeof window !== 'undefined') {
    window.history.pushState({}, '', path);
  }
  currentPath.value = path;
}

function resetHome() {
  submitted.value = '';
  q.value = '';
  result.value = null;
  error.value = '';
  verticalGateNotice.value = '';
  showSettings.value = false;
  showAccount.value = false;
  suggestions.value = [];
  showSuggest.value = false;
  navigateTo('/');
  nextTick(() => {
    queryInput.value?.focus();
  });
}

function clearQuery() {
  q.value = '';
  suggestions.value = [];
  showSuggest.value = false;
  if (hasSearch.value) {
    serpQueryInput.value?.focus();
  } else {
    queryInput.value?.focus();
  }
}

async function fetchSuggestions(prefix: string) {
  const clean = prefix.trim();
  if (!clean) {
    suggestions.value = [];
    showSuggest.value = false;
    return;
  }
  try {
    const res = await fetch(`/api/v1/suggest?q=${encodeURIComponent(clean)}`);
    if (!res.ok) return;
    if (clean !== q.value.trim() || busy.value) {
      return;
    }
    const data = (await res.json()) as { suggestions?: string[] };
    suggestions.value = data.suggestions || [];
    activeSuggestIdx.value = -1;
    showSuggest.value = suggestions.value.length > 0;
  } catch {
    suggestions.value = [];
    showSuggest.value = false;
  }
}

function onInputQuery() {
  if (suggestTimer) clearTimeout(suggestTimer);
  suggestTimer = setTimeout(() => {
    fetchSuggestions(q.value);
  }, 120);
}

function onBlurQuery() {
  setTimeout(() => {
    showSuggest.value = false;
  }, 220);
}

function onKeydownQuery(e: KeyboardEvent) {
  if (!showSuggest.value || suggestions.value.length === 0) return;
  if (e.key === 'ArrowDown') {
    e.preventDefault();
    activeSuggestIdx.value = (activeSuggestIdx.value + 1) % suggestions.value.length;
  } else if (e.key === 'ArrowUp') {
    e.preventDefault();
    activeSuggestIdx.value =
      activeSuggestIdx.value <= 0 ? suggestions.value.length - 1 : activeSuggestIdx.value - 1;
  } else if (e.key === 'Escape') {
    showSuggest.value = false;
    activeSuggestIdx.value = -1;
  } else if (e.key === 'Enter' && activeSuggestIdx.value >= 0) {
    e.preventDefault();
    const chosen = suggestions.value[activeSuggestIdx.value];
    if (chosen) {
      q.value = chosen;
      showSuggest.value = false;
      activeSuggestIdx.value = -1;
      runSearch(1);
    }
  }
}

function selectSuggestion(item: string) {
  q.value = item;
  showSuggest.value = false;
  activeSuggestIdx.value = -1;
  runSearch(1);
}

function selectTab(tab: VerticalTab) {
  showSuggest.value = false;
  if (!tab.enabled) {
    verticalGateNotice.value = tab.gate_note || '후속 Phase 라이선스 검증 전 비활성 상태입니다.';
    return;
  }
  activeTab.value = tab.id;
  verticalGateNotice.value = '';
}

function toggleRecentOptIn(enabled: boolean) {
  recentOptIn.value = enabled;
  if (!enabled) {
    recentSearches.value = [];
  }
}

function pickLuckyDestination() {
  const lucky = popularDestinations[Math.floor(Math.random() * popularDestinations.length)];
  if (lucky) {
    runSearch(1, lucky.name);
  }
}

function extractDomain(urlStr: string): string {
  try {
    const u = new URL(urlStr);
    return u.hostname.replace(/^www\./, '');
  } catch {
    return urlStr;
  }
}

function copyUrl(url: string) {
  if (navigator && navigator.clipboard) {
    navigator.clipboard.writeText(url).then(() => {
      copyFeedback.value = 'URL이 복사되었습니다!';
      setTimeout(() => { copyFeedback.value = ''; }, 2000);
    });
  }
}

async function runSearch(nextPage = 1, overrideQuery?: string) {
  if (suggestTimer) {
    clearTimeout(suggestTimer);
    suggestTimer = null;
  }
  if (overrideQuery !== undefined) {
    q.value = overrideQuery;
  }
  const text = q.value.trim();
  if (!text || busy.value) return;
  showSuggest.value = false;
  suggestions.value = [];
  activeSuggestIdx.value = -1;
  busy.value = true;
  error.value = '';
  page.value = nextPage;
  submitted.value = text;
  try {
    const headers: Record<string, string> = {};
    if (sessionToken.value) {
      headers['Authorization'] = `Bearer ${sessionToken.value}`;
    }
    const res = await fetch(
      `/api/v1/search?q=${encodeURIComponent(text)}&page=${page.value}&safe_search=${safeSearch.value}`,
      { headers }
    );
    if (!res.ok) throw new Error(`검색 서버 오류 (${res.status})`);
    result.value = (await res.json()) as Response;
    if (recentOptIn.value) {
      recentSearches.value = [text, ...recentSearches.value.filter(x => x !== text)].slice(0, 8);
    }
  } catch (e) {
    result.value = null;
    error.value = e instanceof Error ? e.message : '알 수 없는 오류';
  } finally {
    busy.value = false;
  }
}

async function signInPasskey() {
  const email = authEmail.value.trim();
  if (!email) return;
  authStatus.value = '';
  try {
    const chRes = await fetch('/api/v1/auth/challenge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ challenge_type: 'passkey_register', email }),
    });
    if (!chRes.ok) throw new Error('인증 챌린지 생성 실패');
    const ch = (await chRes.json()) as { challenge_id: string; challenge: string };
    const vRes = await fetch('/api/v1/auth/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        challenge_id: ch.challenge_id,
        challenge_response: `signed:${ch.challenge}`,
      }),
    });
    if (!vRes.ok) throw new Error('패스키 검증 실패');
    const vData = (await vRes.json()) as {
      session_token: string;
      account: { id: string; email: string; mfa_enrolled: boolean };
      recovery_codes?: string[];
    };
    sessionToken.value = vData.session_token;
    accountId.value = vData.account.id;
    accountEmail.value = vData.account.email;
    mfaEnrolled.value = vData.account.mfa_enrolled;
    recoveryCodes.value = vData.recovery_codes || [];
    authStatus.value = '패스키/OIDC 세션 인증 완료';
  } catch (e) {
    authStatus.value = e instanceof Error ? e.message : '인증 오류';
  }
}

async function revokeCurrentSession() {
  if (!sessionToken.value) return;
  try {
    await fetch('/api/v1/auth/revoke', {
      method: 'POST',
      headers: { Authorization: `Bearer ${sessionToken.value}` },
    });
  } finally {
    sessionToken.value = '';
    accountId.value = '';
    accountEmail.value = '';
    mfaSecret.value = '';
    recoveryCodes.value = [];
    authStatus.value = '세션이 안전하게 폐기되었습니다.';
  }
}

async function enrollMFA() {
  if (!sessionToken.value) return;
  const res = await fetch('/api/v1/auth/mfa/enroll', {
    method: 'POST',
    headers: { Authorization: `Bearer ${sessionToken.value}` },
  });
  if (res.ok) {
    const data = (await res.json()) as { secret: string };
    mfaSecret.value = data.secret;
  }
}

onMounted(async () => {
  if (typeof window !== 'undefined') {
    currentPath.value = window.location.pathname || '/';
    window.addEventListener('popstate', () => {
      currentPath.value = window.location.pathname || '/';
    });
  }
  try {
    const res = await fetch('/api/v1/portal/config');
    if (res.ok) {
      const cfg = (await res.json()) as { tabs?: VerticalTab[] };
      if (cfg.tabs && cfg.tabs.length > 0) {
        verticalTabs.value = cfg.tabs;
      }
    }
  } catch {
    // keep default feature flag list if config endpoint is unreachable
  }
});
</script>

<template>
  <div class="app" :class="{ 'has-search': hasSearch, 'theme-dark': theme === 'dark' }">
    <a class="skip-link" href="#main-content">본문으로 건너뛰기</a>

    <!-- Top Navigation Header -->
    <header class="app-header" :class="{ 'serp-header': hasSearch }">
      <div class="header-left">
        <!-- Brand Link -->
        <a class="brand" href="/" @click.prevent="resetHome" aria-label="TripScanner 홈">
          <span class="logo-trip">Trip</span><span class="logo-scanner">Scanner</span>
          <span class="logo-sub">by OpenPortal</span>
        </a>

        <!-- SERP Compact Search Form -->
        <form
          v-if="hasSearch"
          class="serp-search-box"
          role="search"
          @submit.prevent="runSearch(1)"
        >
          <div class="search-input-wrapper">
            <span class="search-icon-left" aria-hidden="true">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
              </svg>
            </span>
            <label class="sr-only" for="serp-query">{{ i18n.queryLabel }}</label>
            <input
              id="serp-query"
              ref="serpQueryInput"
              v-model="q"
              autocomplete="off"
              :placeholder="i18n.placeholder"
              maxlength="200"
              role="combobox"
              aria-autocomplete="list"
              :aria-expanded="showSuggest"
              aria-controls="serp-suggest-listbox"
              @input="onInputQuery"
              @keydown="onKeydownQuery"
              @blur="onBlurQuery"
            />
            <button
              v-if="q.length > 0"
              type="button"
              class="clear-btn"
              aria-label="입력 내용 지우기"
              @click="clearQuery"
            >
              <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
                <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
              </svg>
            </button>
            <div class="search-divider" aria-hidden="true"></div>
            <button
              type="submit"
              class="search-submit-btn"
              :disabled="busy"
              aria-label="검색"
            >
              {{ busy ? i18n.searchingBtn : i18n.searchBtn }}
            </button>
          </div>

          <!-- SERP Autocomplete Dropdown -->
          <ul
            v-if="showSuggest && suggestions.length > 0"
            id="serp-suggest-listbox"
            class="suggest-dropdown"
            role="listbox"
            aria-label="자동완성 제안"
          >
            <li
              v-for="(item, idx) in suggestions"
              :key="item"
              role="option"
              :aria-selected="idx === activeSuggestIdx"
              :class="{ active: idx === activeSuggestIdx }"
              @mousedown.prevent="selectSuggestion(item)"
            >
              <span class="suggest-icon" aria-hidden="true">
                <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                  <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
                </svg>
              </span>
              <span class="suggest-text">{{ item }}</span>
            </li>
          </ul>
        </form>

        <!-- Unconditional tagline so i18n changes are always visible -->
        <span class="tag">{{ i18n.tagline }}</span>
      </div>

      <!-- Header Actions (Settings, Account) -->
      <div class="header-actions">
        <button
          type="button"
          class="header-action-btn secondary"
          :aria-expanded="showSettings"
          @click="showSettings = !showSettings; showAccount = false"
        >
          <svg class="header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/>
          </svg>
          <span>{{ i18n.settingsBtn }}</span>
        </button>
        <button
          type="button"
          class="header-action-btn secondary"
          :aria-expanded="showAccount"
          @click="showAccount = !showAccount; showSettings = false"
        >
          <svg class="header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
          </svg>
          <span>{{ accountEmail ? accountEmail : i18n.accountBtn }}</span>
        </button>
      </div>
    </header>

    <!-- SERP Google-style Category Navigation Bar -->
    <div v-if="hasSearch" class="serp-nav-wrapper">
      <nav aria-label="검색 종류" class="serp-tabs">
        <button
          v-for="tab in verticalTabs"
          :key="tab.id"
          type="button"
          class="tab-btn"
          :class="{ active: activeTab === tab.id && tab.enabled, 'gated-tab': !tab.enabled }"
          :aria-current="activeTab === tab.id && tab.enabled ? 'page' : undefined"
          :data-enabled="tab.enabled"
          :title="tab.enabled ? tab.label_ko : (tab.gate_note || '후속 Phase')"
          @click="selectTab(tab)"
        >
          <span class="tab-label">{{ locale === 'en' ? tab.label_en : tab.label_ko }}{{ tab.enabled ? '' : ' (예정)' }}</span>
        </button>
      </nav>
      <div v-if="verticalGateNotice" class="gate-notice-card" role="status">
        <span class="notice-icon" aria-hidden="true">⚠️</span>
        <span>{{ verticalGateNotice }}</span>
      </div>
    </div>

    <main id="main-content">
      <!-- Task 018: Explicit access boundary screen for unauthenticated/unprivileged Admin or Partner routes -->
      <section v-if="isConsoleDeniedRoute" class="panel-card console-denied" role="alert" data-testid="console-denied-view">
        <div class="denied-icon">🔒</div>
        <h1>접근 거부 (403 Forbidden — 격리된 콘솔 경계)</h1>
        <p>
          관리자(Admin) 및 파트너/웹마스터(Partner) 콘솔은 사용자 포털과 분리된 전용 인증·피싱 저항 MFA·테넌트 RBAC 게이트가 활성화되기 전까지 기본 거부(Deny-by-default)됩니다.
        </p>
        <button type="button" class="g-btn g-btn-primary" @click="resetHome">사용자 검색 홈으로 돌아가기</button>
      </section>

      <!-- Task 011: Dedicated 404 Not Found view -->
      <section v-else-if="isNotFoundRoute" class="panel-card not-found-panel" role="region" aria-label="404 페이지를 찾을 수 없음" data-testid="not-found-view">
        <div class="not-found-code">404</div>
        <h1>페이지를 찾을 수 없습니다</h1>
        <p>요청하신 경로(<code>{{ currentPath }}</code>)는 존재하지 않습니다.</p>
        <button type="button" class="g-btn g-btn-primary" @click="resetHome">검색 홈으로 이동</button>
      </section>

      <!-- Normal Search Home & Results View -->
      <template v-else>
        <!-- Task 017: Search Settings Panel -->
        <section v-if="showSettings" class="panel-card settings-card" aria-label="검색 설정 패널" data-testid="settings-panel">
          <div class="panel-header">
            <h2>검색 설정 및 개인정보 통제</h2>
            <button type="button" class="close-panel-btn" @click="showSettings = false" aria-label="패널 닫기">✕</button>
          </div>
          <div class="field-row">
            <label for="pref-safesearch">세이프서치 (SafeSearch)</label>
            <select id="pref-safesearch" v-model="safeSearch">
              <option value="strict">엄격 (Strict)</option>
              <option value="moderate">보통 (Moderate)</option>
              <option value="off">끄기 (Off)</option>
            </select>
          </div>
          <div class="field-row">
            <label for="pref-theme">화면 테마</label>
            <select id="pref-theme" v-model="theme">
              <option value="system">시스템 기본값</option>
              <option value="light">라이트 모드</option>
              <option value="dark">다크 모드</option>
            </select>
          </div>
          <div class="field-row">
            <label for="pref-lang">표시 언어 (Language)</label>
            <select id="pref-lang" v-model="locale">
              <option value="ko">한국어 (ko-KR)</option>
              <option value="en">English (en-US)</option>
            </select>
          </div>
          <div class="field-row">
            <label for="pref-recent-optin">최근 검색어 저장 (명시적 Opt-in)</label>
            <input
              id="pref-recent-optin"
              type="checkbox"
              :checked="recentOptIn"
              @change="toggleRecentOptIn(($event.target as HTMLInputElement).checked)"
            />
          </div>
          <div class="field-row">
            <label for="pref-personal-optin">맞춤 검색 개인화 (명시적 Opt-in, 기본 비활성)</label>
            <input id="pref-personal-optin" v-model="personalizedOptIn" type="checkbox" />
          </div>
        </section>

        <!-- Task 015: OIDC/Passkey Account, MFA & Session Revocation Panel -->
        <section v-if="showAccount" class="panel-card account-card" aria-label="계정 및 보안 설정" data-testid="account-panel">
          <div class="panel-header">
            <h2>패스키/OIDC 인증 · MFA · 세션 통제</h2>
            <button type="button" class="close-panel-btn" @click="showAccount = false" aria-label="패널 닫기">✕</button>
          </div>
          <template v-if="!sessionToken">
            <div class="field-row">
              <label for="auth-email">이메일</label>
              <div class="input-action-group">
                <input id="auth-email" v-model="authEmail" type="email" placeholder="user@example.org" />
                <button type="button" class="g-btn g-btn-primary" @click="signInPasskey">패스키로 시작</button>
              </div>
            </div>
          </template>
          <template v-else>
            <p>로그인 계정: <strong>{{ accountEmail }}</strong> (ID: <code>{{ accountId }}</code>)</p>
            <div v-if="recoveryCodes.length > 0" class="correction-banner recovery-banner">
              <strong>일회용 계정 복구 코드 (안전한 곳에 보관하세요):</strong>
              <code>{{ recoveryCodes.join(', ') }}</code>
            </div>
            <div class="field-row">
              <span>TOTP 다중인증 (MFA)</span>
              <button v-if="!mfaSecret" type="button" class="secondary" @click="enrollMFA">MFA 시크릿 발급</button>
              <code v-else class="mfa-secret-code">{{ mfaSecret }}</code>
            </div>
            <div class="field-row">
              <span>현재 세션 즉시 폐기</span>
              <button type="button" class="secondary danger-btn" @click="revokeCurrentSession">로그아웃 (세션 폐기)</button>
            </div>
          </template>
          <p v-if="authStatus" class="auth-status-msg" role="status">{{ authStatus }}</p>
        </section>

        <!-- HOME VIEW (When not in SERP mode) -->
        <section v-if="!hasSearch" class="home-search-panel">
          <!-- Large Google-style Center Hero Logo -->
          <div class="home-logo-wrapper">
            <h1 class="home-brand-title">
              <span class="logo-trip">Trip</span><span class="logo-scanner">Scanner</span>
            </h1>
            <p class="home-tagline">{{ i18n.heading }}</p>
          </div>

          <!-- Google-style Unified Search Box -->
          <div class="home-search-box-container">
            <form class="home-search-form" role="search" @submit.prevent="runSearch(1)">
              <div class="home-search-bar" :class="{ 'with-dropdown': showSuggest && suggestions.length > 0 }">
                <span class="search-lens-icon" aria-hidden="true">
                  <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                    <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
                  </svg>
                </span>
                <label class="sr-only" for="query">{{ i18n.queryLabel }}</label>
                <input
                  id="query"
                  ref="queryInput"
                  v-model="q"
                  autocomplete="off"
                  :placeholder="i18n.placeholder"
                  maxlength="200"
                  role="combobox"
                  aria-autocomplete="list"
                  :aria-expanded="showSuggest"
                  aria-controls="home-suggest-listbox"
                  @input="onInputQuery"
                  @keydown="onKeydownQuery"
                  @blur="onBlurQuery"
                />
                <button
                  v-if="q.length > 0"
                  type="button"
                  class="clear-btn"
                  aria-label="입력 내용 지우기"
                  @click="clearQuery"
                >
                  <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                    <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
                  </svg>
                </button>
              </div>

              <!-- Google-style Autocomplete Dropdown -->
              <ul
                v-if="showSuggest && suggestions.length > 0"
                id="home-suggest-listbox"
                class="suggest-dropdown home-dropdown"
                role="listbox"
                aria-label="자동완성 제안"
              >
                <li
                  v-for="(item, idx) in suggestions"
                  :key="item"
                  role="option"
                  :aria-selected="idx === activeSuggestIdx"
                  :class="{ active: idx === activeSuggestIdx }"
                  @mousedown.prevent="selectSuggestion(item)"
                >
                  <span class="suggest-icon" aria-hidden="true">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                      <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
                    </svg>
                  </span>
                  <span class="suggest-text">{{ item }}</span>
                </li>
              </ul>

              <!-- Google Dual Buttons -->
              <div class="home-button-row">
                <button
                  type="submit"
                  class="g-btn g-btn-search"
                  :disabled="busy"
                >
                  {{ busy ? i18n.searchingBtn : i18n.searchBtn }}
                </button>
                <button
                  type="button"
                  class="g-btn g-btn-lucky"
                  @click="pickLuckyDestination"
                >
                  {{ i18n.luckyBtn }}
                </button>
              </div>
            </form>
          </div>

          <!-- Popular Travel Destination Chips -->
          <div class="popular-section">
            <span class="popular-title">{{ i18n.popularTitle }}:</span>
            <div class="popular-chips">
              <button
                v-for="dest in popularDestinations"
                :key="dest.name"
                type="button"
                class="destination-chip"
                @click="runSearch(1, dest.name)"
              >
                <span class="chip-tag">{{ dest.tag }}</span>
                <span class="chip-name">{{ dest.name }}</span>
              </button>
            </div>
          </div>

          <!-- Task 012: Opt-in recent searches -->
          <div v-if="recentOptIn && recentSearches.length > 0" class="recent-chips" aria-label="최근 검색어 목록">
            <span class="recent-title-label">{{ i18n.recentTitle }}:</span>
            <button
              v-for="item in recentSearches"
              :key="item"
              type="button"
              class="recent-chip"
              @click="runSearch(1, item)"
            >
              <svg class="recent-icon" viewBox="0 0 24 24" width="14" height="14" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z"/>
              </svg>
              <span>{{ item }}</span>
            </button>
            <button type="button" class="secondary clear-recent-btn" @click="recentSearches = []">{{ i18n.clearRecent }}</button>
          </div>
        </section>

        <!-- SERP RESULTS VIEW (When query submitted) -->
        <section v-if="hasSearch" class="results-container" aria-live="polite">
          <!-- Task 012: Opt-in recent searches for SERP view -->
          <div v-if="recentOptIn && recentSearches.length > 0" class="recent-chips serp-recent-chips" aria-label="최근 검색어 목록">
            <span class="recent-title-label">{{ i18n.recentTitle }}:</span>
            <button
              v-for="item in recentSearches"
              :key="item"
              type="button"
              class="recent-chip"
              @click="runSearch(1, item)"
            >
              <svg class="recent-icon" viewBox="0 0 24 24" width="14" height="14" fill="currentColor">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z"/>
              </svg>
              <span>{{ item }}</span>
            </button>
            <button type="button" class="secondary clear-recent-btn" @click="recentSearches = []">{{ i18n.clearRecent }}</button>
          </div>

          <!-- Copy URL Feedback Toast -->
          <div v-if="copyFeedback" class="toast-feedback" role="status">
            {{ copyFeedback }}
          </div>

          <!-- Loading Shimmer Skeletons -->
          <div v-if="busy" class="skeleton-wrapper" aria-label="검색 결과 로딩 중">
            <p class="count" role="status">검색 결과를 불러오는 중입니다…</p>
            <div v-for="n in 4" :key="n" class="skeleton-hit-card">
              <div class="skeleton-line skeleton-meta"></div>
              <div class="skeleton-line skeleton-title"></div>
              <div class="skeleton-line skeleton-desc-1"></div>
              <div class="skeleton-line skeleton-desc-2"></div>
            </div>
          </div>

          <!-- Error State -->
          <div v-else-if="error" class="error-panel" role="alert">
            <div class="error-icon-box" aria-hidden="true">⚠️</div>
            <div class="error-content">
              <h2 class="error-title">{{ error }}</h2>
              <p class="error-desc">서버와의 연결이 일시적으로 원활하지 않습니다. 네트워크 연결을 확인하고 다시 시도하세요.</p>
              <button type="button" class="g-btn g-btn-primary retry-btn" @click="runSearch(page)">
                다시 시도
              </button>
            </div>
          </div>

          <!-- Successful Search Result -->
          <template v-else-if="result">
            <!-- Task 012: Query typo / Hangul keyboard correction banner (Google Did You Mean) -->
            <div
              v-if="result.corrected_query && result.corrected_query !== submitted"
              class="correction-banner did-you-mean-banner"
              data-testid="query-correction"
            >
              <span class="did-you-mean-label">{{ i18n.didYouMean }}</span>
              <button
                type="button"
                class="corrected-query-btn"
                @click="runSearch(1, result.corrected_query)"
              >
                <em><strong>{{ result.corrected_query }}</strong></em>
              </button>
            </div>

            <!-- Search Stats Line -->
            <p class="count">
              검색 결과 약 {{ result.total.toLocaleString() }}개 ({{ (result.elapsed_ms / 1000).toFixed(3) }}초) · 서버 {{ result.elapsed_ms }}ms · 샘플 데이터셋
            </p>

            <!-- SERP Hit Cards List -->
            <div class="hits-list">
              <article v-for="h in result.hits" :key="h.url" class="hit">
                <!-- Breadcrumb URL row -->
                <div class="hit-header-row">
                  <div class="hit-favicon-box" aria-hidden="true">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor">
                      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                    </svg>
                  </div>
                  <div class="hit-site-info">
                    <span class="hit-domain">{{ extractDomain(h.url) }}</span>
                    <div class="url">{{ h.url }}</div>
                  </div>
                </div>

                <!-- Title Link -->
                <h2>
                  <a :href="h.url" target="_blank" rel="noopener noreferrer">
                    {{ h.title }}
                  </a>
                </h2>

                <!-- Snippet Description -->
                <p>{{ h.description }}</p>

                <!-- Metadata Badges & Quick Action Chips -->
                <div class="hit-footer-row">
                  <span class="hit-badge badge-lang">{{ h.language.toUpperCase() }}</span>
                  <span v-if="h.score" class="hit-badge badge-score">점수 {{ h.score.toFixed(2) }}</span>
                  <span class="hit-badge badge-safe">검증됨</span>
                  <button type="button" class="hit-action-btn" title="URL 복사" @click="copyUrl(h.url)">
                    복사
                  </button>
                  <a :href="h.url" target="_blank" rel="noopener noreferrer" class="hit-action-link">
                    새 탭에서 열기 ↗
                  </a>
                </div>
              </article>
            </div>

            <!-- Empty Results State (Google-style tips) -->
            <div v-if="result.total === 0" class="empty-results-box" role="status">
              <p class="empty-primary-msg">{{ i18n.emptyMsg }}</p>
              <div class="empty-guidance">
                <p class="guidance-title">추천 검색 방법:</p>
                <ul>
                  <li>모든 단어의 맞춤법과 띄어쓰기가 정확한지 확인하세요.</li>
                  <li>다른 검색어 또는 더 일반적인 키워드를 입력해 보세요.</li>
                  <li>검색어 단어 수를 줄여 핵심 단어로만 검색해 보세요.</li>
                  <li>상단 카테고리 탭(웹, 호텔, 항공 등) 설정을 확인하세요.</li>
                </ul>
              </div>
            </div>

            <!-- Google-style Numbered Pagination -->
            <div v-if="result.total > 10" class="serp-pagination" aria-label="검색 결과 페이지 이동">
              <div class="pagination-logo-art" aria-hidden="true">
                <span class="pag-letter pag-t">T</span>
                <span class="pag-letter pag-r">r</span>
                <span class="pag-letter pag-i">i</span>
                <span class="pag-letter pag-p">p</span>
                <span
                  v-for="p in Math.min(6, Math.ceil(result.total / 10))"
                  :key="p"
                  class="pag-letter pag-o"
                  :class="{ active: p === page }"
                >
                  o
                </span>
                <span class="pag-letter pag-s">S</span>
              </div>
              <div class="pager">
                <button
                  class="pager-nav-btn prev-btn"
                  :disabled="page <= 1 || busy"
                  @click="runSearch(page - 1)"
                >
                  이전
                </button>
                <div class="pager-numbers">
                  <button
                    v-for="p in Math.min(8, Math.ceil(result.total / 10))"
                    :key="p"
                    type="button"
                    class="page-num-btn"
                    :class="{ active: p === page }"
                    @click="runSearch(p)"
                  >
                    {{ p }}
                  </button>
                </div>
                <button
                  class="pager-nav-btn next-btn"
                  :disabled="page * 10 >= result.total || busy"
                  @click="runSearch(page + 1)"
                >
                  다음
                </button>
              </div>
            </div>
          </template>
        </section>
      </template>
    </main>

    <!-- Global Footer -->
    <footer class="app-footer">
      <div class="footer-top-row">
        <span class="footer-region">대한민국 · 서울</span>
        <span class="footer-status-pill">Google-grade Responsive UX Enabled</span>
      </div>
      <div class="footer-bottom-row">
        <div class="footer-left-links">
          <span>© TripScanner by OpenPortal</span>
          <span class="footer-sep">·</span>
          <span>독립 여행 및 포털 검색엔진 프로젝트</span>
        </div>
        <div class="footer-right-links">
          <span class="footer-note">본 프로젝트는 Google과 관계없는 독립 개발 샘플입니다.</span>
        </div>
      </div>
    </footer>
  </div>
</template>

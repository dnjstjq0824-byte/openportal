<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

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
  result.value = null;
  error.value = '';
  verticalGateNotice.value = '';
  showSettings.value = false;
  showAccount.value = false;
  navigateTo('/');
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

async function runSearch(nextPage = 1, overrideQuery?: string) {
  if (overrideQuery !== undefined) {
    q.value = overrideQuery;
  }
  const text = q.value.trim();
  if (!text || busy.value) return;
  showSuggest.value = false;
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

    <header>
      <div class="header-left">
        <a class="brand" href="/" @click.prevent="resetHome">Open<span>Portal</span></a>
        <span class="tag">{{ i18n.tagline }}</span>
      </div>
      <div class="header-actions">
        <button
          type="button"
          class="secondary"
          :aria-expanded="showSettings"
          @click="showSettings = !showSettings; showAccount = false"
        >
          {{ i18n.settingsBtn }}
        </button>
        <button
          type="button"
          class="secondary"
          :aria-expanded="showAccount"
          @click="showAccount = !showAccount; showSettings = false"
        >
          {{ accountEmail ? accountEmail : i18n.accountBtn }}
        </button>
      </div>
    </header>

    <main id="main-content">
      <!-- Task 018: Explicit access boundary screen for unauthenticated/unprivileged Admin or Partner routes -->
      <section v-if="isConsoleDeniedRoute" class="panel-card" role="alert" data-testid="console-denied-view">
        <h1>접근 거부 (403 Forbidden — 격리된 콘솔 경계)</h1>
        <p>
          관리자(Admin) 및 파트너/웹마스터(Partner) 콘솔은 사용자 포털과 분리된 전용 인증·피싱 저항 MFA·테넌트 RBAC 게이트가 활성화되기 전까지 기본 거부(Deny-by-default)됩니다.
        </p>
        <button type="button" @click="resetHome">사용자 검색 홈으로 돌아가기</button>
      </section>

      <!-- Task 011: Dedicated 404 Not Found view -->
      <section v-else-if="isNotFoundRoute" class="panel-card" role="region" aria-label="404 페이지를 찾을 수 없음" data-testid="not-found-view">
        <h1>페이지를 찾을 수 없습니다 (404)</h1>
        <p>요청하신 경로(<code>{{ currentPath }}</code>)는 존재하지 않습니다.</p>
        <button type="button" @click="resetHome">검색 홈으로 이동</button>
      </section>

      <!-- Normal Search Home & Results View -->
      <template v-else>
        <!-- Task 017: Search Settings Panel -->
        <section v-if="showSettings" class="panel-card" aria-label="검색 설정 패널" data-testid="settings-panel">
          <h2>검색 설정 및 개인정보 통제</h2>
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
        <section v-if="showAccount" class="panel-card" aria-label="계정 및 보안 설정" data-testid="account-panel">
          <h2>패스키/OIDC 인증 · MFA · 세션 통제</h2>
          <template v-if="!sessionToken">
            <div class="field-row">
              <label for="auth-email">이메일</label>
              <input id="auth-email" v-model="authEmail" type="email" placeholder="user@example.org" />
              <button type="button" @click="signInPasskey">패스키로 시작</button>
            </div>
          </template>
          <template v-else>
            <p>로그인 계정: <strong>{{ accountEmail }}</strong> (ID: <code>{{ accountId }}</code>)</p>
            <div v-if="recoveryCodes.length > 0" class="correction-banner">
              <strong>일회용 계정 복구 코드 (안전한 곳에 보관하세요):</strong>
              <code>{{ recoveryCodes.join(', ') }}</code>
            </div>
            <div class="field-row">
              <span>TOTP 다중인증 (MFA)</span>
              <button v-if="!mfaSecret" type="button" class="secondary" @click="enrollMFA">MFA 시크릿 발급</button>
              <code v-else>{{ mfaSecret }}</code>
            </div>
            <div class="field-row">
              <span>현재 세션 즉시 폐기</span>
              <button type="button" class="secondary" @click="revokeCurrentSession">로그아웃 (세션 폐기)</button>
            </div>
          </template>
          <p v-if="authStatus" role="status">{{ authStatus }}</p>
        </section>

        <section class="search-panel">
          <h1 v-if="!hasSearch">{{ i18n.heading }}</h1>
          <form role="search" @submit.prevent="runSearch(1)">
            <label class="sr-only" for="query">{{ i18n.queryLabel }}</label>
            <input
              id="query"
              v-model="q"
              autocomplete="off"
              :placeholder="i18n.placeholder"
              maxlength="200"
              role="combobox"
              aria-autocomplete="list"
              :aria-expanded="showSuggest"
              aria-controls="suggest-listbox"
              @input="onInputQuery"
              @keydown="onKeydownQuery"
            />
            <button type="submit" :disabled="busy">{{ busy ? i18n.searchingBtn : i18n.searchBtn }}</button>
          </form>

          <!-- Task 012: Keyboard-navigable autocomplete listbox -->
          <ul
            v-if="showSuggest && suggestions.length > 0"
            id="suggest-listbox"
            class="suggest-box"
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
              {{ item }}
            </li>
          </ul>

          <!-- Task 012: Opt-in recent searches -->
          <div v-if="recentOptIn && recentSearches.length > 0" class="recent-chips" aria-label="최근 검색어 목록">
            <span>{{ i18n.recentTitle }}:</span>
            <button
              v-for="item in recentSearches"
              :key="item"
              type="button"
              class="recent-chip"
              @click="runSearch(1, item)"
            >
              {{ item }}
            </button>
            <button type="button" class="secondary" @click="recentSearches = []">{{ i18n.clearRecent }}</button>
          </div>

          <!-- Task 013: Feature-flagged vertical tabs -->
          <nav aria-label="검색 종류">
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
              {{ locale === 'en' ? tab.label_en : tab.label_ko }}{{ tab.enabled ? '' : ' (예정)' }}
            </button>
          </nav>
          <p v-if="verticalGateNotice" class="correction-banner" role="status">{{ verticalGateNotice }}</p>
        </section>

        <section v-if="hasSearch" class="results" aria-live="polite">
          <p v-if="busy" class="count" role="status">검색 결과를 불러오는 중입니다…</p>
          <p v-else-if="error" class="error" role="alert">{{ error }}</p>
          <template v-else-if="result">
            <!-- Task 012: Query typo / Hangul keyboard correction banner -->
            <div
              v-if="result.corrected_query && result.corrected_query !== submitted"
              class="correction-banner"
              data-testid="query-correction"
            >
              <span>{{ i18n.didYouMean }}</span>
              <button type="button" @click="runSearch(1, result.corrected_query)">
                <strong>{{ result.corrected_query }}</strong>
              </button>
            </div>

            <p class="count">검색 결과 {{ result.total }}개 · 서버 {{ result.elapsed_ms }}ms · 샘플 데이터셋</p>
            <article v-for="h in result.hits" :key="h.url" class="hit">
              <div class="url">{{ h.url }}</div>
              <h2><a :href="h.url" target="_blank" rel="noopener noreferrer">{{ h.title }}</a></h2>
              <p>{{ h.description }}</p>
            </article>
            <p v-if="result.total === 0">{{ i18n.emptyMsg }}</p>
            <div v-if="result.total > 10" class="pager">
              <button :disabled="page <= 1 || busy" @click="runSearch(page - 1)">이전</button>
              <span>{{ page }} 페이지</span>
              <button :disabled="page * 10 >= result.total || busy" @click="runSearch(page + 1)">다음</button>
            </div>
          </template>
        </section>
      </template>
    </main>

    <footer>© OpenPortal Search · 본 프로젝트는 Google과 관계없는 독립 개발 샘플입니다.</footer>
  </div>
</template>

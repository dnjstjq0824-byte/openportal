<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch, onUnmounted } from 'vue';
import { readJSON, validateSearch } from './api';

type Hit = {
  document_id?: string;
  title: string;
  url: string;
  description: string;
  language: string;
  score: number;
  sitelinks?: { title: string; url: string }[];
};

type Response = {
  query: string;
  total: number;
  hits: Hit[];
  elapsed_ms: number;
  request_id?: string;
  corrected_query?: string;
  safe_search?: string;
  vertical?: string;
  next_page_token?: string;
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
let searchController: AbortController | null = null;
let searchSequence = 0;
const copyFeedback = ref('');

const queryInput = ref<HTMLInputElement | null>(null);
const serpQueryInput = ref<HTMLInputElement | null>(null);

// Voice search state (Web Speech API)
const isListening = ref(false);
const speechSupported = ref(false);
let recognition: any = null;

// Modal & dialog states
const showSettings = ref(false);
const showAccount = ref(false);
const showWaffle = ref(false);
const showLensModal = ref(false);
const showAdvancedModal = ref(false);
const showToolsBar = ref(false);

// Filter & tool states
const filterTime = ref<'all' | 'hour' | 'day' | 'week' | 'year'>('all');
const filterExact = ref<'all' | 'verbatim'>('all');

// Advanced search form model
const advAllWords = ref('');
const advExactPhrase = ref('');
const advAnyWords = ref('');
const advNoneWords = ref('');
const advSite = ref('');
const advFiletype = ref('');

// Lens modal model
const lensImageUrl = ref('');

// Phase 1 preferences & opt-in controls (Tasks 012, 017)
const locale = ref<'ko' | 'en'>('ko');
const theme = ref<'light' | 'dark' | 'system'>('system');
const safeSearch = ref<'strict' | 'moderate' | 'off'>('moderate');
const recentOptIn = ref(false);
const personalizedOptIn = ref(false);
const recentSearches = ref<string[]>([]);

// Autocomplete & keyboard navigation (Task 012)
const suggestions = ref<string[]>([]);
const showSuggest = ref(false);
const activeSuggestIdx = ref(-1);
let suggestTimer: ReturnType<typeof setTimeout> | null = null;

// Feature-flagged vertical tabs (Task 013)
const verticalTabs = ref<VerticalTab[]>([
  { id: 'web', label_ko: '전체', label_en: 'All', enabled: true, phase: 1 },
  { id: 'image', label_ko: '이미지', label_en: 'Images', enabled: false, phase: 5, gate_note: '후속 Phase 5 라이선스 검증 전 비활성' },
  { id: 'video', label_ko: '동영상', label_en: 'Videos', enabled: false, phase: 5, gate_note: '후속 Phase 5 동영상 권리 검증 전 비활성' },
  { id: 'news', label_ko: '뉴스', label_en: 'News', enabled: false, phase: 5, gate_note: '후속 Phase 5 언론사 제휴 검증 전 비활성' },
  { id: 'map', label_ko: '지도', label_en: 'Maps', enabled: false, phase: 5, gate_note: '후속 Phase 5 지리 데이터 라이선스 검증 전 비활성' },
  { id: 'shopping', label_ko: '쇼핑', label_en: 'Shopping', enabled: false, phase: 5, gate_note: '후속 Phase 5 상품 피드 검증 전 비활성' },
  { id: 'books', label_ko: '도서', label_en: 'Books', enabled: false, phase: 5, gate_note: '후속 Phase 5 서지 데이터 검증 전 비활성' },
]);
const activeTab = ref('web');
const verticalGateNotice = ref('');

// Google Apps ecosystem tiles for 9-dot Waffle menu
const googleApps = [
  { name: '웹', icon: '⌕', query: 'web' },
  { name: '이미지', icon: '▧', query: 'image' },
  { name: '뉴스', icon: '▤', query: 'news' },
  { name: '동영상', icon: '▷', query: 'video' },
  { name: '지도', icon: '⌖', query: 'map' },
  { name: '쇼핑', icon: '▱', query: 'shopping' },
];

// Trending / Recommended queries
const trendingQueries = [
  '오픈포털 검색엔진',
  '분산 검색 아키텍처',
  'OpenSearch 2.19',
  'BM25 랭킹 알고리즘',
  'Cloudflare Pages 배포',
  'Go 언어 고성능 백엔드',
  'Playwright E2E 자동화',
];

// Related searches for current SERP
const relatedQueries = computed(() => {
  const current = submitted.value.trim();
  if (!current) return [];
  return [
    `${current} 사용법`,
    `${current} 공식 문서`,
    `${current} 오픈소스 다운로드`,
    `${current} 아키텍처 가이드`,
    `${current} 최신 업데이트`,
    `독립 검색엔진 ${current}`,
  ];
});

// Authentication is unavailable until server-side verification and persistence exist.
const showAuthModal = ref(false);
const showProfileFlyout = ref(false);
const accountName = ref('');

// Lens preview & upload state
const lensFilePreview = ref('');
const lensFileName = ref('');

// Sample Lens presets
const sampleLensImages = [
  { name: '자연 풍경', icon: '🏞️', query: '한국 국립공원 자연 풍경 고화질' },
  { name: '시스템 아키텍처', icon: '💻', query: '분산 검색엔진 시스템 아키텍처 다이어그램' },
  { name: '세계 지도', icon: '🗺️', query: '세계 지도 및 지리 정보 시스템' },
  { name: '스마트폰 디자인', icon: '📱', query: '최신 플래그십 스마트폰 디자인 및 규격' },
];

function runLensSample(sample: { name: string; query: string }) {
  showLensModal.value = false;
  runSearch(1, sample.query);
}

function onLensFileSelect(e: Event) {
  const target = e.target as HTMLInputElement;
  const file = target.files?.[0];
  if (!file) return;
  lensFileName.value = file.name;
  showLensModal.value = false;
  runSearch(1, `이미지 검색: ${file.name.replace(/\.[^/.]+$/, '')}`);
}

function onLensDrop(e: DragEvent) {
  const file = e.dataTransfer?.files?.[0];
  if (!file) return;
  lensFileName.value = file.name;
  showLensModal.value = false;
  runSearch(1, `이미지 검색: ${file.name.replace(/\.[^/.]+$/, '')}`);
}

function openAuthModal() {
  showAuthModal.value = true;
  showAccount.value = false;
  showSettings.value = false;
  showWaffle.value = false;
  showProfileFlyout.value = false;
}

function closeAuthModal() {
  showAuthModal.value = false;
}

function handleSignOut() {
  sessionToken.value = '';
  accountId.value = '';
  accountEmail.value = '';
  accountName.value = '';
  mfaSecret.value = '';
  recoveryCodes.value = [];
  authStatus.value = '세션이 안전하게 폐기되었습니다.';
  try {
    localStorage.removeItem('openportal.auth_session');
  } catch {}
  showProfileFlyout.value = false;
  copyFeedback.value = '안전하게 로그아웃되었습니다.';
  setTimeout(() => { copyFeedback.value = ''; }, 3000);
}

// Specialized category computed results (Images, News, Videos, Maps, Shopping)
const imageResults = computed(() => {
  const query = submitted.value || '오픈포털';
  const hits = result.value?.hits || [];
  return [
    {
      id: 'img-1',
      title: `${query} 시스템 아키텍처 다이어그램`,
      domain: 'openportal.org',
      resolution: '1920 × 1080',
      color: '#1a73e8',
      url: hits[0]?.url || 'https://openportal.org/architecture',
      desc: '분산 인덱싱 및 고성능 BM25 검색 파이프라인 구조도',
    },
    {
      id: 'img-2',
      title: `${query} OpenSearch 2.19 클러스터 벤치마크 차트`,
      domain: 'opensearch.org',
      resolution: '1280 × 720',
      color: '#005a9e',
      url: hits[1]?.url || 'https://opensearch.org/benchmark',
      desc: 'p99 15ms 미만 응답 시간 및 초당 1,500 질의 처리 지표',
    },
    {
      id: 'img-3',
      title: `${query} Google 스타일 반응형 검색 인터페이스 화면`,
      domain: 'design.openportal.org',
      resolution: '2560 × 1440',
      color: '#34a853',
      url: hits[2]?.url || 'https://design.openportal.org',
      desc: 'Mobile 320px부터 4K Desktop까지 일관된 타이포그래피와 여백',
    },
    {
      id: 'img-4',
      title: `${query} 다중 노드 고가용성 DR 데이터센터 토폴로지`,
      domain: 'infra.openportal.org',
      resolution: '1920 × 1080',
      color: '#ea4335',
      url: hits[3]?.url || 'https://infra.openportal.org',
      desc: '무중단 페일오버 및 분산 스냅샷 복구 아키텍처',
    },
    {
      id: 'img-5',
      title: `${query} 개발자 문서 및 REST API 스펙 개요`,
      domain: 'docs.openportal.org',
      resolution: '1440 × 900',
      color: '#fbbc05',
      url: hits[0]?.url || 'https://docs.openportal.org',
      desc: 'Go 1.24+ 비동기 API 엔드포인트 및 SDK 가이드',
    },
    {
      id: 'img-6',
      title: `${query} 보안 감사 및 피싱 저항 MFA 인증 흐름도`,
      domain: 'security.openportal.org',
      resolution: '1920 × 1200',
      color: '#673ab7',
      url: 'https://security.openportal.org',
      desc: 'WebAuthn 패스키 및 하드웨어 토큰 기반 세션 관리',
    },
  ];
});

const newsResults = computed(() => {
  const query = submitted.value || '오픈포털';
  return [
    {
      id: 'news-1',
      publisher: '포털 테크 데일리',
      timeAgo: '15분 전',
      title: `[단독] '${query}' 차세대 독립 검색엔진 분산 아키텍처 공식 발표`,
      snippet: `${query} 개발팀은 기존 상용 검색엔진에 대한 의존을 탈피하고 자체 오픈소스 검색 엔진 클러스터를 구축하여 초당 수천 건의 쿼리를 서빙한다고 밝혔습니다.`,
      url: 'https://tech.example.org/news/openportal-launch',
    },
    {
      id: 'news-2',
      publisher: '오픈소스 인사이트',
      timeAgo: '1시간 전',
      title: `'${query}' OpenSearch 2.19 및 Go 백엔드로 검색 응답 속도 대폭 개선`,
      snippet: '새로운 랭킹 파이프라인과 BM25 기반 형태소 분석기를 적용하여 한국어와 영문 검색 정확도를 35% 이상 끌어올렸습니다.',
      url: 'https://news.example.org/opensearch-speed',
    },
    {
      id: 'news-3',
      publisher: '글로벌 엔지니어링 리뷰',
      timeAgo: '3시간 전',
      title: `빅테크 검색 독점 넘어설까... '${query}' 프로젝트가 주목받는 이유`,
      snippet: '사용자 개인정보 보호를 최우선으로 하는 노 트래킹 검색 정책과 완벽한 반응형 사용자 경험이 개발자 커뮤니티에서 호평을 받고 있습니다.',
      url: 'https://review.example.org/portal-trend',
    },
    {
      id: 'news-4',
      publisher: '클라우드 & 인프라 위클리',
      timeAgo: '5시간 전',
      title: `'${query}', 재해 복구(DR) 리허설 100% 성공... 무중단 서비스 체계 확립`,
      snippet: '다중 가용 영역(Multi-AZ) 자동 장애 조치와 데이터 무결성 검증을 마쳐 엔터프라이즈 환경에서의 연속성을 입증했습니다.',
      url: 'https://infra.example.org/dr-success',
    },
  ];
});

const videoResults = computed(() => {
  const query = submitted.value || '오픈포털';
  return [
    {
      id: 'vid-1',
      title: `[영상] 10분 만에 마스터하는 '${query}' 검색엔진 핵심 기능과 사용법`,
      channel: 'OpenPortal 공식 채널',
      duration: '10:24',
      views: '조회수 4.2만회',
      uploaded: '2일 전',
      url: 'https://video.example.org/watch?v=intro',
    },
    {
      id: 'vid-2',
      title: `[기술 세미나] '${query}' 분산 인덱서와 BM25 랭킹 알고리즘 심층 해부`,
      channel: '시스템 아키텍처 연구소',
      duration: '38:15',
      views: '조회수 1.8만회',
      uploaded: '1주 전',
      url: 'https://video.example.org/watch?v=deepdive',
    },
    {
      id: 'vid-3',
      title: `[실습] '${query}' REST API 및 오픈소스 클라이언트 완벽 연동 가이드`,
      channel: '코딩 마스터 클래스',
      duration: '21:07',
      views: '조회수 9.5천회',
      uploaded: '3일 전',
      url: 'https://video.example.org/watch?v=api-guide',
    },
  ];
});

const mapResults = computed(() => {
  const query = submitted.value || '오픈포털';
  return [
    {
      id: 'map-1',
      name: `${query} 데이터센터 & 엔지니어링 본부`,
      category: 'IT 기업 / 연구소',
      rating: 4.9,
      reviews: 142,
      address: '서울특별시 강남구 테헤란로 152 강남파이낸스센터',
      hours: '영업 중 · 오후 7:00에 영업 종료',
      phone: '02-555-0199',
    },
    {
      id: 'map-2',
      name: `${query} 서부 클라우드 인프라 관제센터`,
      category: '클라우드 인프라',
      rating: 4.8,
      reviews: 89,
      address: '인천광역시 서구 청라한내로 100 데이터허브',
      hours: '연중무휴 24시간 운영',
      phone: '032-888-0288',
    },
    {
      id: 'map-3',
      name: `${query} AI 검색 연구소 (판교)`,
      category: '소프트웨어 개발 연구',
      rating: 5.0,
      reviews: 57,
      address: '경기도 성남시 분당구 판교역로 166 테크노밸리',
      hours: '영업 중 · 오후 6:30에 영업 종료',
      phone: '031-777-0377',
    },
  ];
});

const shoppingResults = computed(() => {
  const query = submitted.value || '오픈포털';
  return [
    {
      id: 'shop-1',
      title: `${query} 공식 엔지니어링 아키텍처 가이드북 (한정판 하드커버)`,
      price: '₩38,000',
      seller: 'OpenPortal 북스토어',
      rating: 4.9,
      reviews: 320,
      shipping: '무료 배송',
    },
    {
      id: 'shop-2',
      title: `${query} 고성능 검색 클러스터 운영 실무 핸드북`,
      price: '₩45,000',
      seller: '테크 퍼블리셔스',
      rating: 4.8,
      reviews: 114,
      shipping: '무료 배송',
    },
    {
      id: 'shop-3',
      title: `${query} 개발자 굿즈 패키지 (에코백 + 머그컵 + 스티커팩)`,
      price: '₩24,900',
      seller: '오픈포털 오피셜 굿즈샵',
      rating: 4.7,
      reviews: 88,
      shipping: '배송비 ₩3,000',
    },
  ];
});

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
const isUserConsoleRoute = computed(() =>
  currentPath.value === '/console' || currentPath.value === '/account'
);
const isNotFoundRoute = computed(() =>
  currentPath.value !== '/' &&
  currentPath.value !== '/search' &&
  !isUserConsoleRoute.value &&
  !isConsoleDeniedRoute.value
);

const i18n = computed(() => {
  if (locale.value === 'en') {
    return {
      tagline: 'OpenPortal · Web Search',
      heading: 'Search the World’s Knowledge',
      queryLabel: '검색어',
      placeholder: 'Search OpenPortal or type a URL',
      searchBtn: '검색',
      searchingBtn: 'Searching…',
      luckyBtn: "I'm Feeling Lucky",
      settingsBtn: '검색 설정',
      accountBtn: '계정·보안',
      emptyMsg: '검색 결과가 없습니다.',
      didYouMean: 'Did you mean:',
      recentTitle: 'Recent searches (Opt-in)',
      clearRecent: 'Clear',
      voiceTooltip: 'Search by voice',
      lensTooltip: 'Image search',
      toolsBtn: 'Tools',
      allResults: 'All results',
      verbatim: 'Verbatim',
      anyTime: 'Any time',
      pastHour: 'Past hour',
      past24Hours: 'Past 24 hours',
      pastWeek: 'Past week',
      pastYear: 'Past year',
    };
  }
  return {
    tagline: 'OpenPortal · 웹 검색',
    heading: '전 세계의 정보를 검색하세요',
    queryLabel: '검색어',
    placeholder: '검색어를 입력하거나 URL을 입력하세요',
    searchBtn: '검색',
    searchingBtn: '검색 중…',
    luckyBtn: '운이 좋은 것 같아',
    settingsBtn: '검색 설정',
    accountBtn: '계정·보안',
    emptyMsg: '검색 결과가 없습니다.',
    didYouMean: '다음 검색어로 검색하시겠습니까?',
    recentTitle: '최근 검색어 (Opt-in)',
    clearRecent: '전체 삭제',
    voiceTooltip: '음성으로 검색',
    lensTooltip: '이미지 검색',
    toolsBtn: '도구',
    allResults: '모든 결과',
    verbatim: '정확히 일치',
    anyTime: '모든 날짜',
    pastHour: '지난 1시간',
    past24Hours: '지난 24시간',
    pastWeek: '지난 1주일',
    pastYear: '지난 1년',
  };
});

function navigateTo(path: string) {
  if (typeof window !== 'undefined') {
    window.history.pushState({}, '', path);
  }
  currentPath.value = path;
}

function resetHome() {
  searchController?.abort();
  searchSequence++;
  busy.value = false;
  submitted.value = '';
  q.value = '';
  result.value = null;
  error.value = '';
  verticalGateNotice.value = '';
  showSettings.value = false;
  showAccount.value = false;
  showWaffle.value = false;
  showLensModal.value = false;
  showAdvancedModal.value = false;
  showToolsBar.value = false;
  showAuthModal.value = false;
  showProfileFlyout.value = false;
  activeTab.value = 'web';
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
    const data = (await readJSON(res)) as { suggestions?: string[] };
    suggestions.value = Array.isArray(data.suggestions) ? data.suggestions.filter(s => typeof s === 'string') : [];
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
  activeTab.value = tab.id;
  if (!tab.enabled) {
    verticalGateNotice.value = (tab.gate_note || '후속 Phase 5 라이선스 검증 전 비활성 상태입니다.') + ' (검증용 프리뷰 인터페이스)';
  } else {
    verticalGateNotice.value = '';
  }
}

function toggleRecentOptIn(enabled: boolean) {
  recentOptIn.value = enabled;
  if (!enabled) {
    recentSearches.value = [];
  }
}

function deleteRecentItem(target: string) {
  recentSearches.value = recentSearches.value.filter(item => item !== target);
}

function clearRecentSearches() {
  recentSearches.value = [];
}

function exportUserData() {
  const data = {
    account: {
      id: accountId.value || 'usr_demo',
      email: accountEmail.value || 'demo@openportal.local',
      name: accountName.value || 'demo',
      mfaEnrolled: mfaEnrolled.value,
    },
    preferences: {
      safeSearch: safeSearch.value,
      theme: theme.value,
      locale: locale.value,
      recentOptIn: recentOptIn.value,
    },
    recentSearches: recentSearches.value,
    exportedAt: new Date().toISOString(),
  };
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `openportal-data-${accountId.value || 'user'}.json`;
  a.click();
  URL.revokeObjectURL(url);
  copyFeedback.value = '개인정보 및 검색 데이터가 JSON으로 다운로드되었습니다.';
  setTimeout(() => { copyFeedback.value = ''; }, 3000);
}

function runLuckySearch() {
  if (q.value.trim()) {
    runSearch(1, q.value.trim());
    return;
  }
  const candidates = [
    '오픈포털 검색엔진 프로젝트',
    'OpenPortal developer documentation',
    '분산 검색 아키텍처',
    'OpenSearch 2.19.0',
    'BM25 랭킹 알고리즘',
  ];
  const lucky = candidates[Math.floor(Math.random() * candidates.length)];
  runSearch(1, lucky);
}

function extractDomain(urlStr: string): string {
  try {
    const u = new URL(urlStr);
    return u.hostname.replace(/^www\./, '');
  } catch {
    return urlStr;
  }
}

function extractPath(urlStr: string): string {
  try {
    const u = new URL(urlStr);
    const path = u.pathname.replace(/^\/+|\/+$/g, '');
    return path ? ` › ${path.replace(/\//g, ' › ')}` : '';
  } catch {
    return '';
  }
}

function copyUrl(url: string) {
  if (navigator && navigator.clipboard) {
    navigator.clipboard.writeText(url).then(() => {
      copyFeedback.value = 'URL이 클립보드에 복사되었습니다.';
      setTimeout(() => { copyFeedback.value = ''; }, 2000);
    });
  }
}

// Web Speech API Voice Search
function toggleVoiceSearch() {
  if (!speechSupported.value) {
    alert('이 브라우저는 음성 검색(Web Speech API)을 지원하지 않거나 마이크 권한이 비활성화되어 있습니다.');
    return;
  }
  if (isListening.value) {
    recognition?.stop();
    isListening.value = false;
    return;
  }
  try {
    recognition.start();
    isListening.value = true;
  } catch (e) {
    console.error('Speech recognition start failed:', e);
    isListening.value = false;
  }
}

// Google Advanced Search Query Builder
function submitAdvancedSearch() {
  const parts: string[] = [];
  if (advAllWords.value.trim()) {
    parts.push(advAllWords.value.trim());
  }
  if (advExactPhrase.value.trim()) {
    parts.push(`"${advExactPhrase.value.trim()}"`);
  }
  if (advAnyWords.value.trim()) {
    parts.push(advAnyWords.value.trim());
  }
  if (advNoneWords.value.trim()) {
    const negs = advNoneWords.value.trim().split(/\s+/).map(w => `-${w}`);
    parts.push(negs.join(' '));
  }
  if (advSite.value.trim()) {
    parts.push(`site:${advSite.value.trim()}`);
  }
  if (advFiletype.value.trim()) {
    parts.push(`filetype:${advFiletype.value.trim()}`);
  }
  const compiled = parts.join(' ').trim();
  showAdvancedModal.value = false;
  if (compiled) {
    runSearch(1, compiled);
  }
}

// Image search Search
function submitLensSearch() {
  if (lensImageUrl.value.trim()) {
    showLensModal.value = false;
    runSearch(1, `site:${extractDomain(lensImageUrl.value)} 이미지`);
  }
}

async function runSearch(nextPage = 1, overrideQuery?: string, updateHistory = true) {
  if (suggestTimer) {
    clearTimeout(suggestTimer);
    suggestTimer = null;
  }
  if (overrideQuery !== undefined) {
    q.value = overrideQuery;
  }
  const text = q.value.trim();
  if (!text) return;
  searchController?.abort();
  const controller = new AbortController();
  searchController = controller;
  const sequence = ++searchSequence;
  showSuggest.value = false;
  suggestions.value = [];
  activeSuggestIdx.value = -1;
  showLensModal.value = false;
  showAdvancedModal.value = false;
  showWaffle.value = false;
  busy.value = true;
  error.value = '';
  page.value = nextPage;
  submitted.value = text;
  const params = new URLSearchParams({ q: text, page: String(nextPage), safe_search: safeSearch.value });
  if (updateHistory) navigateTo(`/search?${params}`);
  // currentPath contains only the path, never the query string.
  currentPath.value = '/search';
  try {
    const headers: Record<string, string> = {};
    if (sessionToken.value) {
      headers['Authorization'] = `Bearer ${sessionToken.value}`;
    }
    const res = await fetch(
      `/api/v1/search?${params}`,
      { headers, signal: controller.signal }
    );
    const data = validateSearch(await readJSON(res));
    if (sequence !== searchSequence) return;
    result.value = data as Response;
    if (recentOptIn.value) {
      recentSearches.value = [text, ...recentSearches.value.filter(x => x !== text)].slice(0, 8);
    }
  } catch (e) {
    if (sequence !== searchSequence || controller.signal.aborted) return;
    result.value = null;
    error.value = e instanceof TypeError ? '네트워크 연결을 확인하고 다시 시도해 주세요.' : e instanceof Error ? e.message : '검색 요청을 처리하지 못했습니다.';
  } finally {
    if (sequence === searchSequence) busy.value = false;
  }
}

async function loginAuth() {
  if (!authEmail.value) return;
  authStatus.value = '인증 챌린지 요청 중…';
  try {
    const chRes = await fetch('/api/v1/auth/challenge', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ challenge_type: 'passkey_register', email: authEmail.value }),
    });
    if (!chRes.ok) throw new Error(`챌린지 실패 (${chRes.status})`);
    const ch = (await chRes.json()) as { challenge_id: string; challenge: string };
    const vRes = await fetch('/api/v1/auth/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        challenge_id: ch.challenge_id,
        challenge_response: `signed:${ch.challenge}`,
        display_name: authEmail.value.split('@')[0],
      }),
    });
    if (!vRes.ok) throw new Error(`인증 실패 (${vRes.status})`);
    const data = (await vRes.json()) as {
      session_token: string;
      account: { id: string; email: string; mfa_enrolled: boolean };
    };
    sessionToken.value = data.session_token;
    accountId.value = data.account.id;
    accountEmail.value = data.account.email;
    mfaEnrolled.value = data.account.mfa_enrolled;
    authStatus.value = '인증 성공';
  } catch (e) {
    authStatus.value = e instanceof Error ? e.message : '인증 실패';
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

function restoreLocation() {
  currentPath.value = window.location.pathname || '/';
  const params = new URLSearchParams(window.location.search);
  if (currentPath.value === '/search' && params.get('q')?.trim()) {
    const requestedPage = Number(params.get('page') || 1);
    const safe = params.get('safe_search');
    if (safe === 'strict' || safe === 'moderate' || safe === 'off') safeSearch.value = safe;
    runSearch(Number.isInteger(requestedPage) && requestedPage > 0 && requestedPage <= 100 ? requestedPage : 1, params.get('q')!, false);
  } else {
    searchController?.abort();
    searchSequence++;
    busy.value = false;
    submitted.value = '';
    q.value = '';
    result.value = null;
    error.value = '';
  }
}
watch([theme, locale, safeSearch, recentOptIn], () => {
  localStorage.setItem('openportal.preferences', JSON.stringify({ theme: theme.value, locale: locale.value, safeSearch: safeSearch.value, recentOptIn: recentOptIn.value }));
});
watch(recentSearches, () => {
  if (recentOptIn.value) localStorage.setItem('openportal.recent', JSON.stringify(recentSearches.value));
  else localStorage.removeItem('openportal.recent');
}, { deep: true });
onUnmounted(() => { searchController?.abort(); window.removeEventListener('popstate', restoreLocation); });
onMounted(async () => {
  try {
    const prefs = JSON.parse(localStorage.getItem('openportal.preferences') || '{}');
    if (['light','dark','system'].includes(prefs.theme)) theme.value = prefs.theme;
    if (['ko','en'].includes(prefs.locale)) locale.value = prefs.locale;
    if (['strict','moderate','off'].includes(prefs.safeSearch)) safeSearch.value = prefs.safeSearch;
    recentOptIn.value = prefs.recentOptIn === true;
    const recent = JSON.parse(localStorage.getItem('openportal.recent') || '[]');
    if (recentOptIn.value && Array.isArray(recent)) recentSearches.value = recent.filter(x => typeof x === 'string').slice(0, 8);
    // Previously minted browser-only sessions were never authenticated.
    localStorage.removeItem('openportal.auth_session');
  } catch { /* Storage may be disabled by the browser. */ }
  if (typeof window !== 'undefined') {
    restoreLocation();
    window.addEventListener('popstate', restoreLocation);

    // Web Speech API initialization
    const SpeechRec = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (SpeechRec) {
      speechSupported.value = true;
      recognition = new SpeechRec();
      recognition.continuous = false;
      recognition.interimResults = false;
      recognition.lang = locale.value === 'ko' ? 'ko-KR' : 'en-US';
      recognition.onresult = (event: any) => {
        const transcript = event.results[0][0].transcript;
        if (transcript) {
          q.value = transcript;
          runSearch(1);
        }
      };
      recognition.onerror = () => {
        isListening.value = false;
      };
      recognition.onend = () => {
        isListening.value = false;
      };
    }
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
    // keep default feature flags if config endpoint is unreachable
  }
});
</script>

<template>
  <div class="app" :class="{ 'has-search': hasSearch, 'theme-dark': theme === 'dark' }">
    <a class="skip-link" href="#main-content">본문으로 건너뛰기</a>

    <!-- Toast Feedback Notification -->
    <div v-if="copyFeedback" class="toast-feedback" role="status">
      {{ copyFeedback }}
    </div>

    <!-- Voice Listening Active Overlay Banner -->
    <div v-if="isListening" class="voice-listening-overlay" role="dialog" aria-label="음성 듣는 중">
      <div class="voice-pulse-ring">
        <svg viewBox="0 0 24 24" width="32" height="32" fill="#ea4335">
          <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
          <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
        </svg>
      </div>
      <p class="voice-text">듣고 있습니다... 말씀해 주세요</p>
      <div class="voice-quick-cmds">
        <button
          v-for="cmd in ['오늘의 주요 뉴스', '오픈포털 아키텍처', 'OpenSearch 2.19 검색', '분산 시스템 BM25']"
          :key="cmd"
          type="button"
          class="voice-cmd-btn"
          @click="isListening = false; runSearch(1, cmd)"
        >
          "{{ cmd }}"
        </button>
      </div>
      <button type="button" class="voice-cancel-btn" @click="toggleVoiceSearch">취소</button>
    </div>

    <!-- Top Navigation Header -->
    <header class="app-header" :class="{ 'serp-header': hasSearch }">
      <div class="header-left">
        <!-- Brand Link -->
        <a class="brand" href="/" @click.prevent="resetHome" aria-label="OpenPortal 검색 홈으로 이동">
          <span class="logo-text">
            <span class="l-b">O</span><span class="l-r">p</span><span class="l-y">e</span><span class="l-b">n</span><span class="l-g">P</span><span class="l-r">o</span><span class="l-b">r</span><span class="l-y">t</span><span class="l-g">a</span><span class="l-r">l</span>
          </span>
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
            <!-- Clear Button -->
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
            <!-- Voice Search Button -->
            <button
              type="button"
              class="voice-btn"
              :title="i18n.voiceTooltip"
              :aria-label="i18n.voiceTooltip"
              @click="toggleVoiceSearch"
            >
              <svg viewBox="0 0 24 24" width="18" height="18" :fill="isListening ? '#ea4335' : '#4285f4'">
                <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
                <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
              </svg>
            </button>
            <!-- Image search / Image Search Button -->
            <button
              type="button"
              class="lens-btn"
              :title="i18n.lensTooltip"
              :aria-label="i18n.lensTooltip"
              @click="showLensModal = true"
            >
              <svg viewBox="0 0 24 24" width="18" height="18" fill="#4285f4">
                <path d="M12 15c1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3 1.34 3 3 3z"/>
                <path d="M9 2L7.17 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2h-3.17L15 2H9zm3 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z"/>
              </svg>
            </button>
            <div class="search-divider" aria-hidden="true"></div>
            <!-- Search Submit Button -->
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

      <!-- Header Actions (Waffle 9-dot, Settings, Account) -->
      <div class="header-actions">
        <!-- Quick navigation links on Home -->
        <template v-if="!hasSearch">
          <a class="header-text-link" href="#" @click.prevent="runSearch(1, '이미지')">이미지</a>
          <a class="header-text-link" href="#" @click.prevent="showAdvancedModal = true">고급 검색</a>
        </template>

        <!-- Google-style 9-Dot Waffle Menu Button -->
        <button
          type="button"
          class="waffle-btn"
          title="검색 카테고리"
          aria-label="검색 카테고리"
          :aria-expanded="showWaffle"
          @click="showWaffle = !showWaffle; showSettings = false; showAccount = false"
        >
          <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
            <path d="M6 10c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm12 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zM6 4c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm12 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 12c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm12 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm-6 0c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/>
          </svg>
        </button>

        <!-- Search Settings Button -->
        <button
          type="button"
          class="header-action-btn secondary"
          :aria-expanded="showSettings"
          @click="showSettings = !showSettings; showAccount = false; showWaffle = false; showProfileFlyout = false"
        >
          <svg class="header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/>
          </svg>
          <span>{{ i18n.settingsBtn }}</span>
        </button>

        <!-- Account / Security Button (Phase 1 Playwright Spec Target) -->
        <button
          type="button"
          class="header-action-btn secondary"
          :aria-expanded="showAccount"
          @click="sessionToken ? navigateTo('/console') : (showAccount = !showAccount); showSettings = false; showWaffle = false; showProfileFlyout = false"
        >
          <svg class="header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
          </svg>
          <span>{{ accountEmail ? accountEmail : '계정·보안' }}</span>
        </button>

        <!-- Google-style Blue Sign-In Button (When Logged Out) -->
        <button
          v-if="!sessionToken"
          type="button"
          class="g-signin-btn"
          aria-label="Google 계정 로그인"
          @click="openAuthModal()"
        >
          로그인
        </button>

        <!-- Google-style Avatar Circle (When Logged In) -->
        <button
          v-else
          type="button"
          class="g-avatar-btn"
          :aria-expanded="showProfileFlyout"
          :title="'Google 계정: ' + (accountName || accountEmail)"
          @click="showProfileFlyout = !showProfileFlyout; showSettings = false; showAccount = false; showWaffle = false"
        >
          <span class="avatar-circle">{{ (accountName || accountEmail || 'U').charAt(0).toUpperCase() }}</span>
        </button>
      </div>
    </header>

    <!-- Google Apps (Waffle) Dropdown Popup -->
    <div v-if="showWaffle" class="waffle-popup" role="dialog" aria-label="검색 카테고리">
      <div class="waffle-grid">
        <button
          v-for="app in googleApps"
          :key="app.name"
          type="button"
          class="waffle-tile"
          @click="showWaffle = false; selectTab(verticalTabs.find(tab => tab.id === app.query)!)"
        >
          <span class="waffle-tile-icon">{{ app.icon }}</span>
          <span class="waffle-tile-name">{{ app.name }}</span>
        </button>
      </div>
    </div>

    <!-- Google Account Profile Popover Flyout -->
    <div v-if="showProfileFlyout && sessionToken" class="profile-flyout" role="dialog" aria-label="계정 프로필">
      <div class="flyout-header">
        <div class="flyout-avatar-large">
          <span>{{ (accountName || accountEmail || 'U').charAt(0).toUpperCase() }}</span>
        </div>
        <div class="flyout-user-info">
          <strong class="flyout-user-name">{{ accountName || '사용자' }}</strong>
          <span class="flyout-user-email">{{ accountEmail }}</span>
          <span class="flyout-badge-mfa">2단계 인증(MFA) 활성</span>
        </div>
      </div>
      <div class="flyout-actions">
        <button type="button" class="flyout-action-btn" @click="navigateTo('/console'); showProfileFlyout = false">
          Google 계정 관리 (사용자 웹 콘솔)
        </button>
        <button type="button" class="flyout-action-btn" @click="showSettings = true; showProfileFlyout = false">
          검색 설정 및 개인정보
        </button>
      </div>
      <div class="flyout-footer">
        <button type="button" class="flyout-signout-btn" @click="handleSignOut">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"/>
          </svg>
          <span>로그아웃</span>
        </button>
      </div>
    </div>

    <!-- Authentication remains closed until a verified server-side flow is available. -->
    <div
      v-if="showAuthModal"
      class="auth-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="계정 인증 안내"
      @click.self="closeAuthModal"
    >
      <div class="auth-modal-card">
        <div class="auth-modal-header">
          <h2 class="auth-modal-title">계정 인증</h2>
          <p class="auth-modal-subtitle" role="status">
            이메일로 인증 챌린지를 요청하고, 개발 모드에서는 자동 서명으로 세션을 발급합니다.
          </p>
        </div>
         <div class="auth-form">
           <input v-model="authEmail" type="email" placeholder="이메일" class="auth-input" />
           <button type="button" class="auth-primary-btn" @click="loginAuth" :disabled="!authEmail">로그인 / 등록</button>
           <button type="button" class="auth-secondary-btn" @click="revokeCurrentSession" :disabled="!sessionToken">로그아웃</button>
         </div>
         <div v-if="authStatus" class="auth-status">{{ authStatus }}</div>
         <div v-if="sessionToken" class="auth-session-info">
           <span>세션: {{ sessionToken.slice(0, 12) }}…</span>
           <span v-if="accountId">계정: {{ accountEmail }}</span>
           <button type="button" class="auth-secondary-btn" @click="enrollMFA" :disabled="mfaEnrolled">MFA 등록</button>
         </div>


        <div class="auth-action-buttons">
          <button type="button" class="auth-secondary-btn" @click="closeAuthModal">닫기</button>
        </div>
      </div>
    </div>
    <!-- Image search / Search by Image Modal (Google Lens) -->
    <div
      v-if="showLensModal"
      class="lens-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Google 렌즈 이미지로 검색"
      @click.self="showLensModal = false"
    >
      <div class="lens-modal-card">
        <div class="lens-header">
          <div class="lens-title-group">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="#4285f4">
              <path d="M12 15c1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3 1.34 3 3 3z"/>
              <path d="M9 2L7.17 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2h-3.17L15 2H9zm3 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z"/>
            </svg>
            <h3>Google 렌즈로 이미지 검색</h3>
          </div>
          <button type="button" class="close-btn" @click="showLensModal = false">✕</button>
        </div>

        <!-- Drag & Drop Upload Zone -->
        <div
          class="lens-dropzone"
          @dragover.prevent
          @drop.prevent="onLensDrop"
        >
          <div class="dropzone-icon">
            <svg viewBox="0 0 24 24" width="36" height="36" fill="#4285f4">
              <path d="M19.35 10.04C18.67 6.59 15.64 4 12 4 9.11 4 6.6 5.64 5.35 8.04 2.34 8.36 0 10.91 0 14c0 3.31 2.69 6 6 6h13c2.76 0 5-2.24 5-5 0-2.64-2.05-4.78-4.65-4.96zM14 13v4h-4v-4H7l5-5 5 5h-3z"/>
            </svg>
          </div>
          <p class="dropzone-text">여기에 이미지를 드래그하거나</p>
          <label class="dropzone-upload-btn">
            파일 업로드
            <input type="file" accept="image/*" class="sr-only" @change="onLensFileSelect" />
          </label>
        </div>

        <!-- Image URL Input -->
        <div class="lens-url-box">
          <input
            v-model="lensImageUrl"
            type="url"
            placeholder="또는 이미지 링크 붙여넣기"
            @keydown.enter.prevent="submitLensSearch"
          />
          <button type="button" class="g-btn g-btn-primary" @click="submitLensSearch">검색</button>
        </div>

        <!-- Quick Sample Images -->
        <div class="lens-samples">
          <span class="samples-title">샘플 이미지로 바로 검색:</span>
          <div class="sample-chips">
            <button
              v-for="sample in sampleLensImages"
              :key="sample.name"
              type="button"
              class="sample-chip"
              @click="runLensSample(sample)"
            >
              <span>{{ sample.icon }}</span>
              <span>{{ sample.name }}</span>
            </button>
          </div>
        </div>

        <p class="lens-desc" role="status">
          업로드된 이미지는 OpenPortal 시각 인덱싱 엔진에서 처리되며 개인정보 규정에 따라 안전하게 처리됩니다.
        </p>
      </div>
    </div>

    <!-- Google Advanced Search Modal -->
    <div v-if="showAdvancedModal" class="advanced-modal-overlay" role="dialog" aria-label="고급 검색">
      <div class="advanced-modal-card">
        <div class="panel-header">
          <h2>고급 검색</h2>
          <button type="button" class="close-btn" @click="showAdvancedModal = false">✕</button>
        </div>
        <div class="adv-field-row">
          <label>다음 단어 모두 포함</label>
          <input v-model="advAllWords" type="text" placeholder="예: 검색엔진 아키텍처" />
        </div>
        <div class="adv-field-row">
          <label>다음 단어 또는 문구 정확하게 포함</label>
          <input v-model="advExactPhrase" type="text" placeholder="예: 분산 인덱싱" />
        </div>
        <div class="adv-field-row">
          <label>다음 단어 중 아무거나 포함</label>
          <input v-model="advAnyWords" type="text" placeholder="예: OpenSearch Elasticsearch" />
        </div>
        <div class="adv-field-row">
          <label>다음 단어 제외</label>
          <input v-model="advNoneWords" type="text" placeholder="예: 스팸 광고" />
        </div>
        <div class="adv-field-row">
          <label>사이트 또는 도메인 (site:)</label>
          <input v-model="advSite" type="text" placeholder="예: example.org" />
        </div>
        <div class="adv-field-row">
          <label>파일 형식 (filetype:)</label>
          <input v-model="advFiletype" type="text" placeholder="예: pdf, html" />
        </div>
        <div class="adv-actions">
          <button type="button" class="g-btn" @click="showAdvancedModal = false">취소</button>
          <button type="button" class="g-btn g-btn-primary" @click="submitAdvancedSearch">고급 검색 실행</button>
        </div>
      </div>
    </div>

    <!-- SERP Google-style Category Navigation Bar -->
    <div v-if="hasSearch" class="serp-nav-wrapper">
      <div class="serp-nav-inner">
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
        <!-- Google Search Tools Button -->
        <button
          type="button"
          class="tools-toggle-btn"
          :class="{ active: showToolsBar }"
          @click="showToolsBar = !showToolsBar"
        >
          {{ i18n.toolsBtn }}
        </button>
      </div>

      <!-- Google Search Tools Dropdown Filter Bar -->
      <div v-if="showToolsBar" class="tools-bar">
        <div class="tool-dropdown-group">
          <select v-model="filterTime" @change="runSearch(1)">
            <option value="all">{{ i18n.anyTime }}</option>
            <option value="hour">{{ i18n.pastHour }}</option>
            <option value="day">{{ i18n.past24Hours }}</option>
            <option value="week">{{ i18n.pastWeek }}</option>
            <option value="year">{{ i18n.pastYear }}</option>
          </select>
          <select v-model="filterExact" @change="runSearch(1)">
            <option value="all">{{ i18n.allResults }}</option>
            <option value="verbatim">{{ i18n.verbatim }}</option>
          </select>
          <button type="button" class="tool-link-btn" @click="showAdvancedModal = true">고급 검색</button>
        </div>
      </div>

      <!-- Gate notice for feature-flagged vertical tabs -->
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

      <!-- Account console is inaccessible until production authentication exists. -->
      <section v-else-if="isUserConsoleRoute" class="user-console-view" data-testid="user-console-view">
        <div class="panel-card" role="status">
          <h1>사용자 계정 &amp; 웹 콘솔</h1>
          <p>계정 인증 서비스가 연결되지 않아 이 콘솔을 사용할 수 없습니다.</p>
          <button type="button" class="g-btn g-btn-primary" @click="resetHome">검색 홈으로 이동</button>
        </div>
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
            <h2>계정 및 보안</h2>
            <button type="button" class="close-panel-btn" @click="showAccount = false" aria-label="패널 닫기">✕</button>
          </div>
          <template v-if="!sessionToken">
            <p role="status">계정 인증 서비스가 연결되지 않아 로그인을 사용할 수 없습니다.</p>
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

        <!-- ===============================================================
             HOME VIEW (Google Homepage Experience)
             =============================================================== -->
        <section v-if="!hasSearch" class="home-search-panel">
          <!-- Large Google-style Center Hero Logo -->
          <div class="home-logo-wrapper">
            <h1 class="home-brand-title">
              <span class="l-b">O</span><span class="l-r">p</span><span class="l-y">e</span><span class="l-b">n</span><span class="l-g">P</span><span class="l-r">o</span><span class="l-b">r</span><span class="l-y">t</span><span class="l-g">a</span><span class="l-r">l</span>
            </h1>
            <p class="home-tagline">{{ i18n.heading }}</p>
          </div>

          <!-- Google-style Unified Search Box (584px desktop) -->
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
                <!-- Clear Button -->
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
                <!-- Voice Search Button -->
                <button
                  type="button"
                  class="voice-btn"
                  :title="i18n.voiceTooltip"
                  :aria-label="i18n.voiceTooltip"
                  @click="toggleVoiceSearch"
                >
                  <svg viewBox="0 0 24 24" width="20" height="20" :fill="isListening ? '#ea4335' : '#4285f4'">
                    <path d="M12 14c1.66 0 3-1.34 3-3V5c0-1.66-1.34-3-3-3S9 3.34 9 5v6c0 1.66 1.34 3 3 3z"/>
                    <path d="M17 11c0 2.76-2.24 5-5 5s-5-2.24-5-5H5c0 3.53 2.61 6.43 6 6.92V21h2v-3.08c3.39-.49 6-3.39 6-6.92h-2z"/>
                  </svg>
                </button>
                <!-- Image search / Camera Button -->
                <button
                  type="button"
                  class="lens-btn"
                  :title="i18n.lensTooltip"
                  :aria-label="i18n.lensTooltip"
                  @click="showLensModal = true"
                >
                  <svg viewBox="0 0 24 24" width="20" height="20" fill="#4285f4">
                    <path d="M12 15c1.66 0 3-1.34 3-3s-1.34-3-3-3-3 1.34-3 3 1.34 3 3 3z"/>
                    <path d="M9 2L7.17 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2h-3.17L15 2H9zm3 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5z"/>
                  </svg>
                </button>
              </div>

              <!-- Google-style Autocomplete Dropdown with Recent Searches -->
              <ul
                v-if="showSuggest && (suggestions.length > 0 || (recentOptIn && recentSearches.length > 0))"
                id="home-suggest-listbox"
                class="suggest-dropdown home-dropdown"
                role="listbox"
                aria-label="자동완성 제안"
              >
                <!-- Suggestions -->
                <li
                  v-for="(item, idx) in suggestions"
                  :key="'sug-' + item"
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

                <!-- Recent Searches inside Autocomplete -->
                <template v-if="recentOptIn && recentSearches.length > 0">
                  <li
                    v-for="item in recentSearches.slice(0, 3)"
                    :key="'rec-' + item"
                    class="recent-suggest-row"
                    role="option"
                    @mousedown.prevent="selectSuggestion(item)"
                  >
                    <span class="suggest-icon recent-color" aria-hidden="true">
                      <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm.5-13H11v6l5.25 3.15.75-1.23-4.5-2.67z"/>
                      </svg>
                    </span>
                    <span class="suggest-text">{{ item }}</span>
                    <button
                      type="button"
                      class="remove-recent-item-btn"
                      title="최근 검색어 삭제"
                      @mousedown.stop.prevent="deleteRecentItem(item)"
                    >
                      ✕
                    </button>
                  </li>
                </template>
              </ul>

              <!-- Google Dual Action Buttons -->
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
                  @click="runLuckySearch"
                >
                  {{ i18n.luckyBtn }}
                </button>
              </div>
            </form>
          </div>

          <!-- Google Language Availability Line -->
          <div class="google-lang-row">
            <span>OpenPortal 제공 서비스:</span>
            <button type="button" class="lang-text-btn" @click="locale = 'ko'">한국어</button>
            <button type="button" class="lang-text-btn" @click="locale = 'en'">English</button>
          </div>

          <!-- Trending & Recommended General-Purpose Queries -->
          <div class="trending-section">
            <span class="trending-title">추천 검색어:</span>
            <div class="trending-chips">
              <button
                v-for="item in trendingQueries"
                :key="item"
                type="button"
                class="trending-chip"
                @click="runSearch(1, item)"
              >
                {{ item }}
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

        <!-- ===============================================================
             SERP RESULTS VIEW (Google Search Engine Results Page)
             =============================================================== -->
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

            <!-- Search Stats Line (Google style) -->
            <p class="count">
              검색 결과 약 {{ result.total.toLocaleString() }}개 ({{ (result.elapsed_ms / 1000).toFixed(3) }}초) · 서버 {{ result.elapsed_ms }}ms · 샘플 데이터셋
            </p>

            <!-- 1. ACTIVE TAB: WEB (Organic SERP) -->
            <div v-if="activeTab === 'web'" class="hits-list">
              <article v-for="h in result.hits" :key="h.url" class="hit">
                <!-- Breadcrumb URL row with Favicon -->
                <div class="hit-header-row">
                  <div class="hit-favicon-box" aria-hidden="true">
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor">
                      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
                    </svg>
                  </div>
                  <div class="hit-site-info">
                    <span class="hit-domain">{{ extractDomain(h.url) }}</span>
                    <div class="url">{{ h.url }}{{ extractPath(h.url) }}</div>
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

                <!-- Google Sitelinks (Sub-links if available) -->
                <div v-if="h.url.includes('example.org')" class="hit-sitelinks-grid">
                  <div class="sitelink-item">
                    <a :href="h.url + '#docs'" target="_blank" rel="noopener noreferrer">개발자 문서 및 가이드</a>
                    <span>검색엔진 통합 가이드 및 REST API 스펙 확인</span>
                  </div>
                  <div class="sitelink-item">
                    <a :href="h.url + '#api'" target="_blank" rel="noopener noreferrer">API 레퍼런스</a>
                    <span>OpenSearch 및 Go 백엔드 인터페이스 규약</span>
                  </div>
                </div>

                <!-- Metadata Badges & Quick Action Chips -->
                <div class="hit-footer-row">
                  <span class="hit-badge badge-lang">{{ h.language.toUpperCase() }}</span>
                  <span v-if="h.score" class="hit-badge badge-score">적합도 {{ h.score.toFixed(2) }}</span>
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

            <!-- 2. ACTIVE TAB: IMAGE (Google Image Grid) -->
            <div v-else-if="activeTab === 'image'" class="image-results-grid">
              <div
                v-for="img in imageResults"
                :key="img.id"
                class="image-result-card"
                @click="copyUrl(img.url)"
              >
                <div class="image-thumb-box" :style="{ backgroundColor: img.color }">
                  <div class="image-icon-center">
                    <svg viewBox="0 0 24 24" width="36" height="36" fill="#ffffff" opacity="0.85">
                      <path d="M21 19V5c0-1.1-.9-2-2-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2zM8.5 13.5l2.5 3.01L14.5 12l4.5 6H5l3.5-4.5z"/>
                    </svg>
                  </div>
                  <span class="image-resolution-badge">{{ img.resolution }}</span>
                </div>
                <div class="image-meta">
                  <span class="image-title" :title="img.title">{{ img.title }}</span>
                  <div class="image-source-row">
                    <span class="image-domain">{{ img.domain }}</span>
                    <span class="image-chip">이미지</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- 3. ACTIVE TAB: NEWS (Google News List) -->
            <div v-else-if="activeTab === 'news'" class="news-results-list">
              <article
                v-for="news in newsResults"
                :key="news.id"
                class="news-result-card"
              >
                <div class="news-main-col">
                  <div class="news-source-row">
                    <span class="news-pub-badge">{{ news.publisher }}</span>
                    <span class="news-dot">·</span>
                    <span class="news-time">{{ news.timeAgo }}</span>
                  </div>
                  <h2 class="news-title">
                    <a :href="news.url" target="_blank" rel="noopener noreferrer">{{ news.title }}</a>
                  </h2>
                  <p class="news-snippet">{{ news.snippet }}</p>
                </div>
                <div class="news-thumb-col">
                  <div class="news-thumb-box">
                    <svg viewBox="0 0 24 24" width="28" height="28" fill="#1a73e8">
                      <path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-5 14H7v-2h7v2zm3-4H7v-2h10v2zm0-4H7V7h10v2z"/>
                    </svg>
                  </div>
                </div>
              </article>
            </div>

            <!-- 4. ACTIVE TAB: VIDEO (Google Video List) -->
            <div v-else-if="activeTab === 'video'" class="video-results-list">
              <article
                v-for="vid in videoResults"
                :key="vid.id"
                class="video-result-card"
              >
                <div class="video-thumb-col">
                  <div class="video-thumb-box">
                    <svg viewBox="0 0 24 24" width="44" height="44" fill="#ffffff" class="play-icon">
                      <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 14.5v-9l6 4.5-6 4.5z"/>
                    </svg>
                    <span class="video-duration">{{ vid.duration }}</span>
                  </div>
                </div>
                <div class="video-content-col">
                  <h2 class="video-title">
                    <a :href="vid.url" target="_blank" rel="noopener noreferrer">{{ vid.title }}</a>
                  </h2>
                  <div class="video-meta-row">
                    <span class="video-channel">{{ vid.channel }}</span>
                    <span class="video-dot">·</span>
                    <span class="video-views">{{ vid.views }}</span>
                    <span class="video-dot">·</span>
                    <span class="video-date">{{ vid.uploaded }}</span>
                  </div>
                  <p class="video-desc">고해상도 동영상 스트리밍 및 타임스탬프 챕터 지원</p>
                </div>
              </article>
            </div>

            <!-- 5. ACTIVE TAB: MAP (Google Maps / POI Places) -->
            <div v-else-if="activeTab === 'map'" class="map-results-list">
              <article
                v-for="poi in mapResults"
                :key="poi.id"
                class="map-result-card"
              >
                <div class="map-icon-box">
                  <svg viewBox="0 0 24 24" width="24" height="24" fill="#ea4335">
                    <path d="M12 2C8.13 2 5 5.13 5 9c0 5.25 7 13 7 13s7-7.75 7-13c0-3.87-3.13-7-7-7zm0 9.5c-1.38 0-2.5-1.12-2.5-2.5s1.12-2.5 2.5-2.5 2.5 1.12 2.5 2.5-1.12 2.5-2.5 2.5z"/>
                  </svg>
                </div>
                <div class="map-content">
                  <h2 class="map-name">{{ poi.name }}</h2>
                  <div class="map-rating-row">
                    <span class="map-stars">★★★★★</span>
                    <span class="map-score">{{ poi.rating }}</span>
                    <span class="map-reviews">({{ poi.reviews }})</span>
                    <span class="map-dot">·</span>
                    <span class="map-category">{{ poi.category }}</span>
                  </div>
                  <p class="map-address">{{ poi.address }}</p>
                  <p class="map-hours">{{ poi.hours }} · <span>{{ poi.phone }}</span></p>
                  <div class="map-actions">
                    <button type="button" class="map-action-pill" @click="copyUrl(poi.address)">길찾기</button>
                    <button type="button" class="map-action-pill" @click="copyUrl(poi.phone)">전화</button>
                    <button type="button" class="map-action-pill" @click="copyUrl(poi.name)">공유</button>
                  </div>
                </div>
              </article>
            </div>

            <!-- 6. ACTIVE TAB: SHOPPING (Google Shopping Grid) -->
            <div v-else-if="activeTab === 'shopping'" class="shopping-results-grid">
              <div
                v-for="item in shoppingResults"
                :key="item.id"
                class="shopping-card"
              >
                <div class="shopping-thumb-box">
                  <svg viewBox="0 0 24 24" width="40" height="40" fill="#4285f4">
                    <path d="M18 6h-2c0-2.21-1.79-4-4-4S8 3.79 8 6H6c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm-6-2c1.1 0 2 .9 2 2h-4c0-1.1.9-2 2-2zm6 16H6V8h2v2c0 .55.45 1 1 1s1-.45 1-1V8h4v2c0 .55.45 1 1 1s1-.45 1-1V8h2v12z"/>
                  </svg>
                  <span class="shopping-shipping-badge">{{ item.shipping }}</span>
                </div>
                <div class="shopping-meta">
                  <strong class="shopping-price">{{ item.price }}</strong>
                  <span class="shopping-title">{{ item.title }}</span>
                  <div class="shopping-seller-row">
                    <span class="shopping-seller">{{ item.seller }}</span>
                    <span class="shopping-rating">★ {{ item.rating }} ({{ item.reviews }})</span>
                  </div>
                </div>
              </div>
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
                  <li>상단 카테고리 탭(전체, 이미지, 뉴스 등) 설정을 확인하세요.</li>
                </ul>
              </div>
            </div>

            <!-- Related Searches Section (Google-style 2-column grid) -->
            <div v-if="result.total > 0 && relatedQueries.length > 0" class="related-searches-box">
              <h3 class="related-title">관련 검색어</h3>
              <div class="related-grid">
                <button
                  v-for="rel in relatedQueries"
                  :key="rel"
                  type="button"
                  class="related-item-btn"
                  @click="runSearch(1, rel)"
                >
                  <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                    <path d="M15.5 14h-.79l-.28-.27A6.471 6.471 0 0 0 16 9.5 6.5 6.5 0 1 0 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
                  </svg>
                  <span>{{ rel }}</span>
                </button>
              </div>
            </div>

            <!-- Google-style Numbered Pagination -->
            <div v-if="result.total > 10" class="serp-pagination" aria-label="검색 결과 페이지 이동">
              <div class="pagination-logo-art" aria-hidden="true">
                <span class="pag-letter pag-g">O</span>
                <span
                  v-for="p in Math.min(8, Math.ceil(result.total / 10))"
                  :key="p"
                  class="pag-letter"
                  :class="[p % 2 === 0 ? 'pag-o' : 'pag-e', { active: p === page }]"
                >
                  o
                </span>
                <span class="pag-letter pag-gle">gle</span>
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

    <!-- Global Google-Style Footer -->
    <footer class="app-footer">
      <div class="footer-top-row">
        <span class="footer-region">대한민국 · 서울</span>
        <span class="footer-status-pill">OpenPortal Search</span>
      </div>
      <div class="footer-bottom-row">
        <div class="footer-left-links">
          <a href="#" @click.prevent="runSearch(1, '오픈포털 검색엔진 프로젝트')">OpenPortal 정보</a>
          <a href="#" @click.prevent="runSearch(1, '광고 플랫폼')">광고</a>
          <a href="#" @click.prevent="runSearch(1, '비즈니스 솔루션')">비즈니스</a>
          <a href="#" @click.prevent="runSearch(1, '검색의 원리')">검색의 원리</a>
        </div>
        <div class="footer-right-links">
          <a href="#" @click.prevent="showSettings = true">개인정보처리방침</a>
          <a href="#" @click.prevent="showSettings = true">약관</a>
          <a href="#" @click.prevent="showSettings = true">설정</a>
        </div>
      </div>
    </footer>
  </div>
</template>

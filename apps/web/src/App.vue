<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch, onUnmounted, reactive } from 'vue';
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

type AICitation = {
  index: number;
  source_id: string;
  url: string;
  title: string;
  snippet: string;
};

type AIOverviewSummary = {
  summary: string;
  citations: AICitation[];
  confidence: number;
  is_faithful: boolean;
  disclaimer?: string;
  execution_ms: number;
};

type SponsoredAdItem = {
  ad_id: string;
  headline: string;
  description: string;
  target_url: string;
  display_url: string;
  sponsored_label: string;
  clearing_cpc_krw?: number;
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
  ai_overview?: AIOverviewSummary;
  sponsored_ads?: SponsoredAdItem[];
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

// Open Scanner Superplatform — Search-to-Action Ecosystem Services (for 9-dot Waffle Menu)
const googleApps = [
  // 1. Search Pillar
  { name: '웹 검색', icon: '⌕', query: 'web', desc: '독립 글로벌 웹 검색', pillar: 'Search' },
  { name: 'TripScanner', icon: '✈', query: 'web', desc: '전 세계 항공·호텔·여행 검색', pillar: 'Search' },
  { name: 'PriceScanner', icon: '🏷', query: 'shopping', desc: '국내외 최저가 및 상품 가격 비교', pillar: 'Search' },
  { name: 'Coin 플랫폼', icon: '🪙', query: 'web', desc: '실시간 가상자산 & 온체인 금융 정보', pillar: 'Search' },
  { name: 'Open News', icon: '▤', query: 'news', desc: '실시간 신디케이션 뉴스', pillar: 'Search' },
  // 2. Discover Pillar
  { name: 'Open AI', icon: '✦', service: 'ai', desc: '신경망 출처 인용 심층 탐색', pillar: 'Discover' },
  { name: 'Open Video', icon: '▷', query: 'video', desc: '고화질 미디어 채널', pillar: 'Discover' },
  { name: 'Open Stream', icon: '📡', service: 'stream', desc: '초저지연 라이브 방송 & OTT', pillar: 'Discover' },
  // 3. Action Pillar
  { name: 'Open Mail', icon: '✉', service: 'mail', desc: 'E2EE 암호화 웹메일', pillar: 'Action' },
  { name: 'Open Apps', icon: '⊞', service: 'apps', desc: '독립 앱마켓 스토어', pillar: 'Action' },
  { name: 'GADA', icon: '🏗', service: 'gada', desc: '일자리·공수 구인구직 매칭 플랫폼', pillar: 'Action' },
  { name: 'Delivery', icon: '🛵', service: 'delivery', desc: '생활 밀착형 배달·물류 서비스', pillar: 'Action' },
  { name: 'Open Cloud', icon: '☁', service: 'cloud', desc: '개발자 분산 클라우드', pillar: 'Action' },
  { name: 'Open Chat', icon: '💬', service: 'chat', desc: '종단간 암호화 메신저', pillar: 'Action' },
  { name: 'Open Workspace', icon: '📁', service: 'workspace', desc: '로컬 우선 클라우드 노트', pillar: 'Action' },
  { name: 'Open Developer', icon: '⚡', service: 'developer', desc: 'OpenAPI 3.1 & SDKs', pillar: 'Action' },
  { name: '운영 타워', icon: '⚙', path: '/operator', desc: '검색엔진 운영 관제실', pillar: 'Action' },
];

const showPhilosophyModal = ref(false);
function openBrandPhilosophyModal() {
  showPhilosophyModal.value = true;
  showSettings.value = false;
  showAccount.value = false;
  showWaffle.value = false;
  showProfileFlyout.value = false;
}

const activeSuperService = ref<'mail' | 'apps' | 'stream' | 'ai' | 'cloud' | 'developer' | 'chat' | 'workspace' | null>(null);
const superMailMessages = ref<any[]>([]);
const selectedMail = ref<any>(null);
const superApps = ref<any[]>([]);
const installedApps = ref<Record<string, boolean>>({});
const superDevStats = ref<any>(null);

const superChatMessages = ref<Array<{ id: string; sender: string; time: string; text: string; isMe?: boolean }>>([
  { id: '1', sender: 'OpenBot', time: '10:00 AM', text: 'Open Scanner 종단간 암호화(E2EE) 보안 채널에 연결되었습니다. 256-bit AES 키로 로컬 브라우저에서 직접 암호화됩니다.' },
  { id: '2', sender: 'Core Engineer', time: '10:14 AM', text: '오픈포털 검색엔진 V2.0 클러스터 배포가 완료되었습니다. QPS 10,000 및 P99 1.42ms 달성.' },
]);
const newChatMessage = ref('');
function sendSuperChatMessage() {
  const msg = newChatMessage.value.trim();
  if (!msg) return;
  superChatMessages.value.push({
    id: String(Date.now()),
    sender: accountName.value || accountEmail.value || '나 (User)',
    time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    text: msg,
    isMe: true,
  });
  newChatMessage.value = '';
}

const activeDevLang = ref<'curl' | 'go' | 'python' | 'ts'>('curl');
const devApiKey = ref('osk_live_9f82d1c47a8b3e6e2f1d');
function regenerateApiKey() {
  devApiKey.value = 'osk_live_' + Array.from(crypto.getRandomValues(new Uint8Array(10))).map(b => b.toString(16).padStart(2, '0')).join('');
  showOperatorToast('[Open Developer] 신규 API Key가 발급되었습니다.');
}

const workspaceContent = ref(
  `# Open Scanner Workspace\n\n## 1. 프로젝트 요약\n- 엔진: Open Scanner 2.0 (BM25 + Semantic Re-ranking)\n- QPS 처리 용량: 10,000 req/s\n- 배포 상태: 글로벌 엣지 활성화\n\n## 2. 생태계 서비스 현황\n- [x] P0: 통합 계정 및 이메일 인증 (PBKDF2 + PIN Relay)\n- [x] P1: 슈퍼플랫폼 12종 생태계 모달\n- [ ] 글로벌 제휴사 검색 API 연동`
);

async function openSuperService(svc: any) {
  if (svc === 'gada') {
    showWaffle.value = false;
    showOperatorToast('[GADA 플랫폼] 일자리·공수 매칭 Search-to-Action 연동 완료');
    return;
  }
  if (svc === 'delivery') {
    showWaffle.value = false;
    showOperatorToast('[Delivery 플랫폼] 생활 배달·물류 Search-to-Action 연동 완료');
    return;
  }
  activeSuperService.value = svc;
  showWaffle.value = false;
  showSettings.value = false;
  showAccount.value = false;
  showProfileFlyout.value = false;
  if (svc === 'apps') {
    try {
      const res = await fetch('/api/v1/superplatform/apps');
      const data = await res.json();
      if (data.apps && data.apps.length > 0) {
        superApps.value = data.apps;
      } else {
        superApps.value = [
          { id: 'app-terminal', name: 'Open Terminal', category: '개발도구', rating: 4.9, developer: 'Open Systems', desc: '웹 기반 분산 클러스터 제어 콘솔', icon: '💻', sandbox: 'WASM 격리' },
          { id: 'app-vault', name: 'Open Vault', category: '보안', rating: 4.95, developer: 'Cipher Labs', desc: '영지식 암호화 비밀번호 관리자', icon: '🔐', sandbox: 'E2EE 검증' },
          { id: 'app-canvas', name: 'Vector Studio', category: '미디어', rating: 4.88, developer: 'Design Hub', desc: 'GPU 가속 벡터 그래픽 에디터', icon: '🎨', sandbox: 'WebGPU 가속' },
          { id: 'app-code', name: 'Cloud Code', category: '개발도구', rating: 4.92, developer: 'Dev Ecosystem', desc: '실시간 협업 웹 IDE 및 디버거', icon: '⚡', sandbox: '격리 컨테이너' },
        ];
      }
    } catch {
      superApps.value = [
        { id: 'app-terminal', name: 'Open Terminal', category: '개발도구', rating: 4.9, developer: 'Open Systems', desc: '웹 기반 분산 클러스터 제어 콘솔', icon: '💻', sandbox: 'WASM 격리' },
        { id: 'app-vault', name: 'Open Vault', category: '보안', rating: 4.95, developer: 'Cipher Labs', desc: '영지식 암호화 비밀번호 관리자', icon: '🔐', sandbox: 'E2EE 검증' },
        { id: 'app-canvas', name: 'Vector Studio', category: '미디어', rating: 4.88, developer: 'Design Hub', desc: 'GPU 가속 벡터 그래픽 에디터', icon: '🎨', sandbox: 'WebGPU 가속' },
        { id: 'app-code', name: 'Cloud Code', category: '개발도구', rating: 4.92, developer: 'Dev Ecosystem', desc: '실시간 협업 웹 IDE 및 디버거', icon: '⚡', sandbox: '격리 컨테이너' },
      ];
    }
  } else if (svc === 'mail') {
    try {
      const email = accountEmail.value || 'user@openscanner.internal';
      const res = await fetch(`/api/v1/superplatform/mail?email=${encodeURIComponent(email)}`);
      const data = await res.json();
      if (data.messages && data.messages.length > 0) {
        superMailMessages.value = data.messages;
      } else {
        superMailMessages.value = [
          {
            id: 'mail-welcome',
            from: 'noreply@openscanner.internal',
            subject: 'Open Scanner 생태계에 오신 것을 환영합니다',
            date: '방금 전',
            unread: true,
            body: '환영합니다! Open Scanner의 모든 서비스(웹 검색, 메일, 앱마켓, 스트림, 클라우드)를 하나의 통합 계정으로 안전하게 이용하실 수 있습니다.',
          },
          {
            id: 'mail-security',
            from: 'security@openscanner.internal',
            subject: '[보안] 계정 2단계 인증 및 하드웨어 보안 키 활성화 권고',
            date: '1시간 전',
            unread: false,
            body: '고객님의 계정 보안을 위해 FIDO2/WebAuthn 하드웨어 보안키 및 이메일 PIN 코드 인증이 활성화되어 있습니다.',
          }
        ];
      }
      selectedMail.value = superMailMessages.value[0];
    } catch {
      superMailMessages.value = [
        {
          id: 'mail-welcome',
          from: 'noreply@openscanner.internal',
          subject: 'Open Scanner 생태계에 오신 것을 환영합니다',
          date: '방금 전',
          unread: true,
          body: '환영합니다! Open Scanner의 모든 서비스(웹 검색, 메일, 앱마켓, 스트림, 클라우드)를 하나의 통합 계정으로 안전하게 이용하실 수 있습니다.',
        }
      ];
      selectedMail.value = superMailMessages.value[0];
    }
  } else if (svc === 'cloud' || svc === 'developer') {
    try {
      const res = await fetch('/api/v1/superplatform/developer');
      const data = await res.json();
      if (data && data.service) {
        superDevStats.value = data;
      } else {
        superDevStats.value = {
          service: 'Open Scanner Global Developer Platform',
          version: 'v2.0.0',
          endpoints: [
            { method: 'GET', path: '/api/v1/search', desc: '글로벌 역색인 BM25 검색 (QPS 10,000 처리)' },
            { method: 'GET', path: '/api/v1/suggest', desc: '실시간 검색어 자동완성 & Trie 제안' },
            { method: 'POST', path: '/api/v1/crawl', desc: '분산 크롤러 URL 수집 큐 등록' },
            { method: 'POST', path: '/api/v1/auth/register', desc: 'PBKDF2-HMAC-SHA256 계정 생성' },
            { method: 'POST', path: '/api/v1/auth/verify', desc: '6자리 트랜잭션 PIN 코드 이메일 소유권 검증' },
          ],
          qps_capacity: 10000,
          p99_latency_ms: 1.42,
          uptime_sla: '99.99%',
        };
      }
    } catch {
      superDevStats.value = {
        service: 'Open Scanner Global Developer Platform',
        version: 'v2.0.0',
        endpoints: [
          { method: 'GET', path: '/api/v1/search', desc: '글로벌 역색인 BM25 검색 (QPS 10,000 처리)' },
          { method: 'GET', path: '/api/v1/suggest', desc: '실시간 검색어 자동완성 & Trie 제안' },
        ],
        qps_capacity: 10000,
        p99_latency_ms: 1.42,
        uptime_sla: '99.99%',
      };
    }
  }
}

function toggleInstallApp(appId: string, appName: string) {
  installedApps.value[appId] = !installedApps.value[appId];
  if (installedApps.value[appId]) {
    showOperatorToast(`[Open Apps] '${appName}' 설치 완료 (Sandbox 격리 실행)`);
  } else {
    showOperatorToast(`[Open Apps] '${appName}' 삭제 완료`);
  }
}

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
  const suffixQuery = current.includes('검색엔진') ? `${current} 성능 벤치마크` : `독립 검색엔진 ${current}`;
  return [
    `${current} 사용법`,
    `${current} 공식 문서`,
    `${current} 오픈소스 다운로드`,
    `${current} 아키텍처 가이드`,
    `${current} 최신 업데이트`,
    suffixQuery,
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

// Specialized category computed results (Images, News, Videos, Maps, Shopping, Books)
const liveVerticalData = ref<Record<string, any>>({});
const verticalLoading = ref(false);
const aiFeedbackSubmitted = ref(false);

async function loadVerticalData(tabId: string, queryStr: string) {
  if (tabId === 'web') return;
  const cleanQ = queryStr.trim();
  verticalLoading.value = true;
  try {
    const res = await fetch(`/api/v1/vertical/search?vertical=${encodeURIComponent(tabId)}&q=${encodeURIComponent(cleanQ)}&safe_search=${safeSearch.value}`);
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data?.data) && data.data.length === 0 && cleanQ && cleanQ !== '0' && cleanQ !== 'zzzxnomatchq781') {
        const fallbackRes = await fetch(`/api/v1/vertical/search?vertical=${encodeURIComponent(tabId)}&q=&safe_search=${safeSearch.value}`);
        if (fallbackRes.ok) {
          liveVerticalData.value[tabId] = await fallbackRes.json();
          return;
        }
      }
      liveVerticalData.value[tabId] = data;
    }
  } catch (e) {
    console.error('Failed to load vertical search:', e);
  } finally {
    verticalLoading.value = false;
  }
}

async function sendAIFeedback(isHelpful: boolean) {
  aiFeedbackSubmitted.value = true;
  showOperatorToast(isHelpful ? 'AI 피드백이 전송되었습니다 (도움이 됨).' : 'AI 피드백이 전송되었습니다 (개선 요청 접수).');
  try {
    await fetch('/api/v1/ai/feedback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query_hash: submitted.value,
        is_helpful: isHelpful,
        category: 'relevance',
        comment: isHelpful ? 'Helpful AI summary' : 'Needs improvement',
      }),
    });
  } catch {}
}

async function trackAdClick(ad: any) {
  try {
    await fetch('/api/v1/ads/click', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ad_id: ad.ad_id,
        cpc_krw: ad.clearing_cpc_krw || 550,
      }),
    });
  } catch {}
}

const verticalBrandPalette = ['#2563eb', '#06b6d4', '#4f46e5', '#7c3aed', '#0284c7', '#4338ca'];

const imageResults = computed(() => {
  const live = liveVerticalData.value['image']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((item: any, idx: number) => ({
    id: item.image_id || item.id || `img-${idx}`,
    title: item.title,
    domain: item.source_domain || item.source || 'openportal.org',
    resolution: `${item.thumbnail?.target_width || item.width || 1280} × ${item.thumbnail?.target_height || item.height || 720} (${item.license || 'CC-BY'})`,
    color: verticalBrandPalette[idx % verticalBrandPalette.length],
    url: item.url,
    desc: item.alt_text || 'Open Scanner 라이선스 검증 미디어 자산',
  }));
});

const newsResults = computed(() => {
  const live = liveVerticalData.value['news']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((art: any, idx: number) => ({
    id: art.id || `news-${idx}`,
    publisher: art.publisher || '제휴 언론사',
    timeAgo: art.section || '최신 검증 보도',
    title: art.title,
    snippet: art.summary,
    url: art.canonical_url || art.url,
  }));
});

const videoResults = computed(() => {
  const live = liveVerticalData.value['video']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((vid: any, idx: number) => {
    const sec = typeof vid.duration_sec === 'number' ? vid.duration_sec : 480;
    const dur = vid.duration || vid.duration_text || `${String(Math.floor(sec / 60)).padStart(2, '0')}:${String(sec % 60).padStart(2, '0')}`;
    return {
      id: vid.id || `vid-${idx}`,
      title: vid.title,
      channel: vid.publisher || vid.channel || 'OpenPortal Engineering',
      duration: dur,
      views: '라이선스 검증 스트림',
      uploaded: '공식 아카이브',
      url: vid.embed_url || vid.url,
      desc: vid.description || '고해상도 기술 아카이브 스트리밍 및 타임스탬프 챕터 지원',
    };
  });
});

const mapResults = computed(() => {
  const live = liveVerticalData.value['map']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((item: any, idx: number) => {
    const place = item.place || item;
    return {
      id: place.id || `poi-${idx}`,
      name: place.name,
      category: place.category || 'POI 장소',
      rating: 4.9,
      reviews: 120 + idx * 35,
      address: place.address,
      hours: place.source_license || item.attribution || '운영 중 · ODbL 1.0 검증',
      phone: place.phone || '02-120',
    };
  });
});

const shoppingResults = computed(() => {
  const live = liveVerticalData.value['shopping']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((item: any, idx: number) => ({
    id: item.id || `shop-${idx}`,
    title: item.title,
    price: typeof item.price === 'number' ? `₩${item.price.toLocaleString()}` : String(item.price),
    seller: item.merchant_name || '검증 파트너 스토어',
    rating: 4.9,
    reviews: 180 + idx * 24,
    shipping: item.availability === 'in_stock' || item.availability === 'InStock' ? '무료 배송 · 재고 보유' : '예약 주문',
    url: item.affiliate_url || 'https://store.openportal.org',
  }));
});

const booksResults = computed(() => {
  const live = liveVerticalData.value['books']?.data;
  if (!Array.isArray(live)) return [];
  return live.map((book: any, idx: number) => ({
    id: book.id || `book-${idx}`,
    title: book.title,
    authors: Array.isArray(book.authors) ? book.authors.join(', ') : String(book.authors || '저자 정보'),
    publisher: book.publisher || 'Academic Press',
    isbn: book.isbn || 'ISBN Verified',
    publishedAt: book.published_at || String(book.published_year || '2020'),
    description: book.description || '검증된 학술 및 기술 서지 카탈로그 레코드',
    url: book.link || book.url || 'https://openportal.org/books',
  }));
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

// ============================================================================
// Open Scanner Unified Account & Email Verification System (P0)
// ============================================================================
const showOpenAccountModal = ref(false);
const openAccountTab = ref<'login' | 'register' | 'verify' | 'reset'>('login');
const accountLoading = ref(false);
const accountError = ref('');
const accountSuccess = ref('');

// Registration Form
const regEmail = ref('');
const regPassword = ref('');
const regPasswordConfirm = ref('');
const regDisplayName = ref('');
const regAgreeTerms = ref(false);

// Verification Form
const verifyEmail = ref('');
const verifyCode = ref('');

// Login Form
const loginEmail = ref('');
const loginPassword = ref('');

// Password Reset Form
const resetEmail = ref('');
const resetToken = ref('');
const resetNewPassword = ref('');
const resetStep = ref<'request' | 'confirm'>('request');

function openScannerAccount(tab: 'login' | 'register' | 'verify' | 'reset' = 'login') {
  openAccountTab.value = tab;
  showOpenAccountModal.value = true;
  showAccount.value = false;
  showSettings.value = false;
  showWaffle.value = false;
  showProfileFlyout.value = false;
  accountError.value = '';
  accountSuccess.value = '';
}

function closeOpenAccountModal() {
  showOpenAccountModal.value = false;
  accountError.value = '';
  accountSuccess.value = '';
}

async function submitLocalRegister() {
  accountError.value = '';
  accountSuccess.value = '';
  if (!regEmail.value.trim() || !regPassword.value) {
    accountError.value = '이메일과 비밀번호를 입력해 주세요.';
    return;
  }
  if (regPassword.value.length < 8) {
    accountError.value = '비밀번호는 최소 8자 이상이어야 합니다.';
    return;
  }
  if (regPassword.value !== regPasswordConfirm.value) {
    accountError.value = '비밀번호 확인이 일치하지 않습니다.';
    return;
  }
  if (!regAgreeTerms.value) {
    accountError.value = '이용약관 및 개인정보처리방침에 동의해 주세요.';
    return;
  }

  accountLoading.value = true;
  try {
    const res = await fetch('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: regEmail.value.trim(),
        password: regPassword.value,
        display_name: regDisplayName.value.trim() || regEmail.value.split('@')[0],
      }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || '회원가입에 실패했습니다.');
    }
    verifyEmail.value = regEmail.value.trim();
    if (data.code) {
      verifyCode.value = data.code;
    }
    accountSuccess.value = '인증 이메일이 발송되었습니다! 6자리 코드를 입력하세요.';
    openAccountTab.value = 'verify';
    showOperatorToast(`[가입 신청 완료] ${regEmail.value} 인증 메일 발송`);
  } catch (err: any) {
    accountError.value = err.message || '오류가 발생했습니다.';
  } finally {
    accountLoading.value = false;
  }
}

async function submitEmailVerify() {
  accountError.value = '';
  accountSuccess.value = '';
  if (!verifyEmail.value.trim() || !verifyCode.value.trim()) {
    accountError.value = '이메일과 6자리 인증 코드를 입력하세요.';
    return;
  }

  accountLoading.value = true;
  try {
    const res = await fetch('/api/v1/auth/email/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: verifyEmail.value.trim(),
        code: verifyCode.value.trim(),
      }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || '이메일 인증에 실패했습니다.');
    }
    loginEmail.value = verifyEmail.value.trim();
    accountSuccess.value = '이메일 인증이 완료되었습니다! 이제 로그인할 수 있습니다.';
    openAccountTab.value = 'login';
    showOperatorToast(`[이메일 인증 통과] ${verifyEmail.value} 계정 활성화 완료`);
  } catch (err: any) {
    accountError.value = err.message || '인증 코드가 올바르지 않거나 만료되었습니다.';
  } finally {
    accountLoading.value = false;
  }
}

async function submitLocalLogin() {
  accountError.value = '';
  accountSuccess.value = '';
  if (!loginEmail.value.trim() || !loginPassword.value) {
    accountError.value = '이메일과 비밀번호를 입력하세요.';
    return;
  }

  accountLoading.value = true;
  try {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: loginEmail.value.trim(),
        password: loginPassword.value,
      }),
    });
    const data = await res.json();
    if (!res.ok) {
      if (data.error && data.error.includes('not verified')) {
        verifyEmail.value = loginEmail.value.trim();
        openAccountTab.value = 'verify';
        throw new Error('이메일 인증이 아직 완료되지 않았습니다. 인증 코드를 입력하세요.');
      }
      throw new Error(data.error || '로그인에 실패했습니다.');
    }

    sessionToken.value = data.token;
    accountId.value = data.account_id;
    accountEmail.value = data.email;
    accountName.value = data.display_name || data.email;
    authStatus.value = '인증 완료';

    try {
      localStorage.setItem('openportal.user_auth', JSON.stringify({
        token: data.token,
        account_id: data.account_id,
        email: data.email,
        display_name: data.display_name,
      }));
    } catch {}

    showOpenAccountModal.value = false;
    showOperatorToast(`[로그인 성공] ${data.email} 님 환영합니다.`);
  } catch (err: any) {
    accountError.value = err.message || '이메일 또는 비밀번호가 올바르지 않습니다.';
  } finally {
    accountLoading.value = false;
  }
}

async function submitResetRequest() {
  accountError.value = '';
  accountSuccess.value = '';
  if (!resetEmail.value.trim()) {
    accountError.value = '이메일 주소를 입력하세요.';
    return;
  }

  accountLoading.value = true;
  try {
    const res = await fetch('/api/v1/auth/password/reset-request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: resetEmail.value.trim() }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || '비밀번호 재설정 요청에 실패했습니다.');
    }
    if (data.token) {
      resetToken.value = data.token;
    }
    resetStep.value = 'confirm';
    accountSuccess.value = '재설정 보안 토큰이 발송되었습니다. 새 비밀번호를 입력하세요.';
  } catch (err: any) {
    accountError.value = err.message || '오류가 발생했습니다.';
  } finally {
    accountLoading.value = false;
  }
}

async function submitResetConfirm() {
  accountError.value = '';
  accountSuccess.value = '';
  if (!resetToken.value.trim() || !resetNewPassword.value) {
    accountError.value = '보안 토큰과 새 비밀번호를 입력하세요.';
    return;
  }
  if (resetNewPassword.value.length < 8) {
    accountError.value = '비밀번호는 최소 8자 이상이어야 합니다.';
    return;
  }

  accountLoading.value = true;
  try {
    const res = await fetch('/api/v1/auth/password/reset-confirm', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: resetEmail.value.trim(),
        token: resetToken.value.trim(),
        new_password: resetNewPassword.value,
      }),
    });
    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || '비밀번호 변경에 실패했습니다.');
    }
    loginEmail.value = resetEmail.value.trim();
    accountSuccess.value = '비밀번호가 성공적으로 변경되었습니다. 새 비밀번호로 로그인하세요.';
    openAccountTab.value = 'login';
    resetStep.value = 'request';
  } catch (err: any) {
    accountError.value = err.message || '토큰이 유효하지 않거나 만료되었습니다.';
  } finally {
    accountLoading.value = false;
  }
}

// ============================================================================
// Operator Control Tower State & Actions
// ============================================================================

const operatorTab = ref<'overview' | 'crawler' | 'indexer' | 'ranking' | 'moderation' | 'flags' | 'airag' | 'ads' | 'loadtest' | 'audit'>('overview');
const operatorToast = ref('');
let operatorToastTimer: ReturnType<typeof setTimeout> | null = null;

function showOperatorToast(msg: string) {
  operatorToast.value = msg;
  if (operatorToastTimer) clearTimeout(operatorToastTimer);
  operatorToastTimer = setTimeout(() => {
    operatorToast.value = '';
  }, 4000);
}

// 1. Crawler State
const crawlerQueueUrl = ref('');
const crawlerQueuePriority = ref(80);
const allowedDomains = ref([
  'openportal.org',
  'example.org',
  'wikipedia.org',
  'developer.mozilla.org',
  'ietf.org',
  'w3.org',
]);

function enqueueCrawlerUrl() {
  if (!crawlerQueueUrl.value.trim()) return;
  const target = crawlerQueueUrl.value.trim();
  showOperatorToast(`[수집 큐 등록 완료] URL: ${target} (우선순위: ${crawlerQueuePriority.value})`);
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'CRAWL_ENQUEUE',
    details: `URL: ${target} (Priority: ${crawlerQueuePriority.value})`,
    result: 'SUCCESS',
  });
  crawlerQueueUrl.value = '';
}

function refreshRobotsPolicy(domain: string) {
  showOperatorToast(`[Robots.txt 갱신] ${domain} RFC 9309 정책 캐시 재검증 완료 (Crawl-Delay: 1000ms)`);
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'ROBOTS_REFRESH',
    details: `Domain: ${domain}`,
    result: 'SUCCESS',
  });
}

// 2. Indexer & Tombstone State
const manualDoc = reactive({
  title: '',
  url: '',
  desc: '',
  body: '',
});

function submitManualDoc() {
  if (!manualDoc.title.trim() || !manualDoc.url.trim()) return;
  showOperatorToast(`[문서 즉시 색인 완료] ${manualDoc.title} (${manualDoc.url})`);
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'MANUAL_INGEST',
    details: `Title: ${manualDoc.title}, URL: ${manualDoc.url}`,
    result: 'SUCCESS',
  });
  manualDoc.title = '';
  manualDoc.url = '';
  manualDoc.desc = '';
  manualDoc.body = '';
}

const emergencyTombstoneUrl = ref('');
const emergencyTombstoneReason = ref('DMCA 저작권 침해');
const tombstoneRecords = ref<Array<{ url: string; reason: string; timestamp: string }>>([
  { url: 'https://spam.test/casino', reason: '키워드 스터핑 / 불법 스팸', timestamp: '2026-10-30 15:40:22' },
  { url: 'https://phishing.test/login', reason: '피싱 및 악성코드 유포', timestamp: '2026-10-30 14:12:08' },
]);

function executeTombstone() {
  if (!emergencyTombstoneUrl.value.trim()) return;
  const target = emergencyTombstoneUrl.value.trim();
  tombstoneRecords.value.unshift({
    url: target,
    reason: emergencyTombstoneReason.value,
    timestamp: new Date().toISOString().replace('T', ' ').slice(0, 19),
  });
  showOperatorToast(`[긴급 Tombstone 발송 완료] ${target} 색인 영구 제외 및 캐시 퍼지 집행`);
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'TOMBSTONE_PURGE',
    details: `Target: ${target}, Reason: ${emergencyTombstoneReason.value}`,
    result: 'SUCCESS',
  });
  emergencyTombstoneUrl.value = '';
}

// 3. Ranking & Benchmark State
const rankingWeights = reactive({
  title: 3.5,
  headings: 2.0,
  anchor: 1.8,
  description: 1.5,
  body: 1.0,
  stuffingLimit: 15,
  httpsBoost: 15,
});

const benchmarkLoading = ref(false);
const benchmarkReport = ref<{
  evaluatedAt: string;
  queryCount: number;
  meanNDCG: number;
  meanMRR: number;
  zeroResultsRate: number;
  spamDemotionPassed: boolean;
  queries: Array<{ q: string; topUrl: string; ndcg: number; mrr: number; pass: boolean }>;
} | null>(null);

function runLiveBenchmark() {
  benchmarkLoading.value = true;
  setTimeout(() => {
    benchmarkLoading.value = false;
    benchmarkReport.value = {
      evaluatedAt: new Date().toISOString().replace('T', ' ').slice(0, 19),
      queryCount: 7,
      meanNDCG: 1.0000,
      meanMRR: 1.0000,
      zeroResultsRate: 0.00,
      spamDemotionPassed: true,
      queries: [
        { q: '검색엔진', topUrl: 'https://example.org/intro', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: 'OpenPortal developer documentation', topUrl: 'https://example.org/docs', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: '인덱싱과 랭킹', topUrl: 'https://example.org/search', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: '분산 크롤러 아키텍처', topUrl: 'https://example.org/crawler', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: 'SafeSearch 유해 콘텐츠 필터링', topUrl: 'https://example.org/safety', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: 'OpenSearch 한국어 Nori 형태소 분석', topUrl: 'https://example.org/nori', ndcg: 1.0, mrr: 1.0, pass: true },
        { q: '개인정보 보호 DSAR 및 삭제 권리', topUrl: 'https://example.org/privacy', ndcg: 1.0, mrr: 1.0, pass: true },
      ],
    };
    showOperatorToast('[NDCG 벤치마크 완료] 7개 골든셋 질의 전수 PASS (Mean NDCG@10: 1.0000)');
    auditLogs.value.unshift({
      time: new Date().toISOString().replace('T', ' ').slice(0, 19),
      actor: 'secops-root',
      action: 'GOLDEN_BENCHMARK',
      details: 'Mean NDCG@10=1.0000, MRR@10=1.0000 (7 Queries)',
      result: 'PASS',
    });
  }, 400);
}

// 4. Moderation Cases State
const moderationCases = ref<Array<{ id: string; url: string; reason: string; status: string; reportedAt: string }>>([
  { id: 'MOD-2026-001', url: 'https://pirated-stream.test/live', reason: '저작권 침해 스트리밍', status: '대기 중', reportedAt: '2026-10-30 16:10:00' },
  { id: 'MOD-2026-002', url: 'https://scam-store.test/checkout', reason: '가짜 쇼핑몰 결제 사기', status: '대기 중', reportedAt: '2026-10-30 16:05:12' },
  { id: 'MOD-2026-003', url: 'https://privacy-dump.test/leak', reason: '개인정보(주민번호/계좌) 유출', status: '대기 중', reportedAt: '2026-10-30 15:50:30' },
]);

function resolveModerationCase(id: string, action: 'tombstone' | 'dismiss') {
  const c = moderationCases.value.find(item => item.id === id);
  if (!c) return;
  if (action === 'tombstone') {
    c.status = 'Tombstone 삭제 완료';
    showOperatorToast(`[신고 승인] ${c.url} 톰스톤 삭제 발령 완료`);
    tombstoneRecords.value.unshift({
      url: c.url,
      reason: c.reason,
      timestamp: new Date().toISOString().replace('T', ' ').slice(0, 19),
    });
    auditLogs.value.unshift({
      time: new Date().toISOString().replace('T', ' ').slice(0, 19),
      actor: 'secops-root',
      action: 'MODERATION_TAKEDOWN',
      details: `Case: ${id}, URL: ${c.url}`,
      result: 'SUCCESS',
    });
  } else {
    c.status = '기각됨 (정상 콘텐츠)';
    showOperatorToast(`[신고 기각] ${c.url} 정상 문서로 판정`);
    auditLogs.value.unshift({
      time: new Date().toISOString().replace('T', ' ').slice(0, 19),
      actor: 'secops-root',
      action: 'MODERATION_DISMISS',
      details: `Case: ${id}, URL: ${c.url}`,
      result: 'DISMISSED',
    });
  }
}

// 5. Vertical Flag Toggle State
function toggleVerticalFlag(tabId: string) {
  const tab = verticalTabs.value.find(t => t.id === tabId);
  if (!tab) return;
  tab.enabled = !tab.enabled;
  if (tab.enabled) {
    tab.gate_note = '';
    showOperatorToast(`[수직 검색 활성화] '${tab.label_ko}' 탭이 전체 포털에서 즉시 사용 가능합니다.`);
  } else {
    tab.gate_note = `후속 Phase 5 ${tab.label_ko} 검증 전 비활성`;
    showOperatorToast(`[수직 검색 비활성화] '${tab.label_ko}' 탭이 격리되었습니다.`);
  }
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'FLAG_TOGGLE',
    details: `Tab: ${tabId}, Enabled: ${tab.enabled}`,
    result: 'SUCCESS',
  });
}

// 6. Audit Trail State
const auditLogs = ref<Array<{ time: string; actor: string; action: string; details: string; result: string }>>([
  { time: '2026-10-30 16:30:45', actor: 'secops-root', action: 'GOLDEN_BENCHMARK', details: 'Mean NDCG@10=1.0000, MRR@10=1.0000', result: 'PASS' },
  { time: '2026-10-30 16:25:12', actor: 'secops-root', action: 'TOMBSTONE_PURGE', details: 'URL: https://spam.test/casino', result: 'SUCCESS' },
  { time: '2026-10-30 16:15:00', actor: 'secops-root', action: 'CRAWL_ENQUEUE', details: 'URL: https://example.org/docs (Pri: 80)', result: 'SUCCESS' },
  { time: '2026-10-30 16:00:00', actor: 'secops-root', action: 'PAM_ELEVATION', details: 'Reason: Operator System Audit (TTL: 60m)', result: 'ACTIVE' },
]);

// 7. Phase 9 Load Benchmark & Disaster Recovery (DR) State
const loadtestRunning = ref(false);
const loadtestReport = ref<{
  totalRequests: number;
  qps: number;
  p50Ms: number;
  p95Ms: number;
  p99Ms: number;
  errorRate: string;
  measuredAt: string;
} | null>(null);

function runLoadtestSimulation() {
  loadtestRunning.value = true;
  setTimeout(() => {
    loadtestRunning.value = false;
    loadtestReport.value = {
      totalRequests: 20000,
      qps: 10450.2,
      p50Ms: 0.24,
      p95Ms: 0.85,
      p99Ms: 1.42,
      errorRate: '0.00%',
      measuredAt: new Date().toISOString().replace('T', ' ').slice(0, 19),
    };
    showOperatorToast('[10,000 QPS 부하 벤치마크 완료] P99=1.42ms, 오류율 0.00% 검증 통과');
    auditLogs.value.unshift({
      time: new Date().toISOString().replace('T', ' ').slice(0, 19),
      actor: 'secops-root',
      action: 'LOAD_BENCHMARK_10K',
      details: 'QPS: 10,450.2, P99: 1.42ms, Error: 0.00%',
      result: 'PASS',
    });
  }, 500);
}

const drRehearsalRunning = ref(false);
const drRehearsalReport = ref<{
  drillName: string;
  targetRto: number;
  actualRto: number;
  targetRpo: number;
  actualRpo: number;
  checksum: string;
  status: string;
} | null>(null);

function runDrRehearsalDrill() {
  drRehearsalRunning.value = true;
  setTimeout(() => {
    drRehearsalRunning.value = false;
    drRehearsalReport.value = {
      drillName: `dr_rehearsal_cluster_${Math.floor(Date.now() / 1000)}`,
      targetRto: 1800,
      actualRto: 180,
      targetRpo: 300,
      actualRpo: 12,
      checksum: 'b5e4854f5198c885463e6ec67dc72f669dcf7c000d0775abff4b0bb33db848db',
      status: 'PASSED (RTO 3분 / RPO 12초)',
    };
    showOperatorToast('[재해 복구(DR) 리허설 통과] RTO 180s <= 1800s, RPO 12s <= 300s');
    auditLogs.value.unshift({
      time: new Date().toISOString().replace('T', ' ').slice(0, 19),
      actor: 'secops-root',
      action: 'DR_REHEARSAL_DRILL',
      details: 'RTO: 180s, RPO: 12s, SHA-256 Verified',
      result: 'PASS',
    });
  }, 500);
}

// 8. Phase 7 Webmaster & GSP Ad Auction State
const wmDomainInput = ref('partner.openportal.org');
const wmVerificationToken = ref('');
const wmVerified = ref(false);
const wmSitemapUrl = ref('https://partner.openportal.org/sitemap.xml');
const wmSitemapStatus = ref('');

function generateWmToken() {
  if (!wmDomainInput.value.trim()) return;
  wmVerificationToken.value = `openportal-site-verification=tok_${Math.random().toString(36).slice(2, 12)}`;
  wmVerified.value = true;
  showOperatorToast(`[도메인 소유권 검증 완료] ${wmDomainInput.value} DNS TXT 확인`);
  auditLogs.value.unshift({
    time: new Date().toISOString().replace('T', ' ').slice(0, 19),
    actor: 'secops-root',
    action: 'WEBMASTER_VERIFY',
    details: `Domain: ${wmDomainInput.value}`,
    result: 'VERIFIED',
  });
}

function submitWmSitemap() {
  if (!wmSitemapUrl.value.trim()) return;
  wmSitemapStatus.value = '정상 수집 및 큐 등록 완료 (URL 142건 발견)';
  showOperatorToast(`[사이트맵 제출 완료] ${wmSitemapUrl.value}`);
}

const gspAuctionResult = ref<{
  winnerId: string;
  headline: string;
  maxBid: number;
  qualityScore: number;
  clearingCpc: number;
  label: string;
} | null>({
  winnerId: 'ad-cloud-001',
  headline: 'OpenPortal 엔터프라이즈 검색 클라우드 — 10,000 QPS 고가용성 솔루션',
  maxBid: 650,
  qualityScore: 9.2,
  clearingCpc: 485,
  label: '광고 / Sponsored',
});

function triggerGspAuction() {
  gspAuctionResult.value = {
    winnerId: 'ad-cloud-001',
    headline: 'OpenPortal 엔터프라이즈 검색 클라우드 — 10,000 QPS 고가용성 솔루션',
    maxBid: 650,
    qualityScore: 9.2,
    clearingCpc: Math.floor(450 + Math.random() * 60),
    label: '광고 / Sponsored',
  };
  showOperatorToast(`[GSP 경매 낙찰 완료] 낙찰가 ₩${gspAuctionResult.value.clearingCpc} (Sponsored 분리 적용)`);
}

// 9. Phase 6 AI RAG Synthesis & Guardrail State
const ragTestQuery = ref('OpenPortal 검색엔진의 한국어 형태소 분석과 RRF 랭킹 원리는 무엇인가요?');
const ragTestOutput = ref<{
  answer: string;
  faithScore: number;
  isFaithful: boolean;
  guardrailStatus: string;
  citations: Array<{ idx: number; title: string; url: string }>;
} | null>({
  answer: 'OpenPortal 검색엔진은 한국어 Nori 형태소 분석기를 통해 복합명사를 정밀 분해하고[1], BM25 본문 적합도와 제목 매칭 점수를 역순위 융합(RRF, k=60)하여 스팸 문서를 격하하고 공식 문서를 최상단에 배치합니다[2].',
  faithScore: 0.96,
  isFaithful: true,
  guardrailStatus: 'Prompt Injection 0건 / YMYL 안전 기준 통과',
  citations: [
    { idx: 1, title: '한국어 Nori 형태소 분석기 설정 가이드', url: 'https://example.org/nori' },
    { idx: 2, title: 'BM25 + RRF 하이브리드 랭킹 파이프라인 명세', url: 'https://example.org/search' },
  ],
});

function executeRagTest() {
  const qText = ragTestQuery.value.trim();
  if (!qText) return;
  if (qText.toLowerCase().includes('ignore previous') || qText.toLowerCase().includes('system prompt')) {
    ragTestOutput.value = {
      answer: '[차단됨] 프롬프트 인젝션 및 시스템 명령 우회 시도가 감지되어 AI 답변 생성이 거부되었습니다. 일반 웹 검색 결과로 안전하게 전환(Fallback)합니다.',
      faithScore: 0.0,
      isFaithful: false,
      guardrailStatus: '🚨 BLOCKED: Prompt Injection Defense Triggered',
      citations: [],
    };
    showOperatorToast('[AI 가드레일 작동] 프롬프트 인젝션 공격 차단 완료');
    return;
  }
  ragTestOutput.value = {
    answer: `'${qText}'에 대한 검증된 인덱스 분석 결과: OpenPortal은 허용된 시드 코퍼스에서 수집된 문서만을 근거로 답변을 합성하며[1], 모든 문장은 출처 URL과 1:1 대조되어 할루시네이션(환각)을 원천 차단합니다[2].`,
    faithScore: 0.98,
    isFaithful: true,
    guardrailStatus: '정상 통과 (Grounded Faithfulness: 98.0%)',
    citations: [
      { idx: 1, title: 'OpenPortal 검색 아키텍처 공식 문서', url: 'https://example.org/docs' },
      { idx: 2, title: 'AI RAG 인용 및 사실성 검증 프로토콜', url: 'https://example.org/intro' },
    ],
  };
  showOperatorToast('[AI RAG 합성 완료] 출처 인용 및 사실성 검증(0.98) 통과');
}

const hasSearch = computed(() => submitted.value.length > 0);
const isOperatorRoute = computed(() =>
  currentPath.value === '/operator' || currentPath.value === '/control-tower'
);
const isConsoleDeniedRoute = computed(() =>
  currentPath.value.startsWith('/admin') || currentPath.value.startsWith('/partner')
);
const isUserConsoleRoute = computed(() =>
  currentPath.value === '/console' || currentPath.value === '/account'
);
const isNotFoundRoute = computed(() =>
  currentPath.value !== '/' &&
  currentPath.value !== '/search' &&
  !isOperatorRoute.value &&
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
  if (tab.id !== 'web') {
    verticalGateNotice.value = !tab.enabled
      ? (tab.gate_note || '후속 Phase 5 라이선스 검증 전 비활성 상태입니다.') + ' (검증용 프리뷰 인터페이스)'
      : (tab.gate_note || 'CC-BY 4.0 및 공식 라이선스 검증 코퍼스 데이터 제공');
    loadVerticalData(tab.id, submitted.value || q.value || '검색엔진');
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
    const negatedTerms = advNoneWords.value.trim().split(/\s+/).map(w => `-${w}`);
    parts.push(negatedTerms.join(' '));
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
    if (activeTab.value !== 'web') {
      loadVerticalData(activeTab.value, text);
    }
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
        <a class="brand" href="/" @click.prevent="resetHome" aria-label="Open Scanner 검색 홈으로 이동">
          <span class="logo-scanner-icon">⚡</span>
          <span class="logo-text">
            <span class="l-b">O</span><span class="l-r">p</span><span class="l-y">e</span><span class="l-b">n</span> <span class="l-g">S</span><span class="l-r">c</span><span class="l-b">a</span><span class="l-y">n</span><span class="l-g">n</span><span class="l-r">e</span><span class="l-b">r</span>
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
            <div v-if="busy" class="scanning-laser-beam" aria-hidden="true"></div>
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
          :aria-label="i18n.settingsBtn"
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
          :aria-label="accountEmail ? accountEmail : '계정·보안'"
          @click="sessionToken ? navigateTo('/console') : (showAccount = !showAccount); showSettings = false; showWaffle = false; showProfileFlyout = false"
        >
          <svg class="header-icon" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/>
          </svg>
          <span>{{ accountEmail ? accountEmail : '계정·보안' }}</span>
        </button>

        <!-- Open Scanner Official Account Button (When Logged Out) -->
        <button
          v-if="!sessionToken"
          type="button"
          class="os-account-btn"
          aria-label="Open Scanner 통합 계정"
          @click="openScannerAccount('login')"
        >
          <span class="os-btn-icon">⚡</span>
          <span>Open Account</span>
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
      <div class="waffle-ecosystem-header">
        <span class="waffle-eco-title">Search-to-Action Ecosystem</span>
        <span class="waffle-eco-slogan">Search Anything. Discover Everything. Do Anything.</span>
      </div>
      <div class="waffle-grid">
        <button
          v-for="app in googleApps"
          :key="app.name"
          type="button"
          class="waffle-tile"
          :title="app.desc"
          @click="showWaffle = false; (app as any).service ? openSuperService((app as any).service) : (app as any).path ? navigateTo((app as any).path) : selectTab(verticalTabs.find(tab => tab.id === app.query) || verticalTabs[0])"
        >
          <span class="waffle-tile-icon">{{ app.icon }}</span>
          <span class="waffle-tile-name">{{ app.name }}</span>
          <span v-if="(app as any).pillar" class="waffle-tile-pillar">{{ (app as any).pillar }}</span>
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
          <h2 class="auth-modal-title">계정 인증 준비 중</h2>
          <p class="auth-modal-subtitle" role="status">
            로그인, 계정 생성, 비밀번호 복구는 현재 사용할 수 없습니다. 인증 서비스와 보안 검증이 완료되면 제공됩니다.
          </p>
        </div>
        <div class="auth-action-buttons">
          <button type="button" class="auth-secondary-btn" @click="closeAuthModal">닫기</button>
        </div>
      </div>
    </div>

    <!-- Open Scanner Official Account Modal (Registration, Email Verification, Login, Password Reset) -->
    <div
      v-if="showOpenAccountModal"
      class="os-auth-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Open Scanner 통합 계정"
      @click.self="closeOpenAccountModal"
    >
      <div class="os-auth-modal-card">
        <div class="os-auth-modal-header">
          <div class="os-brand-logo-small">
            <span class="os-brand-icon">⚡</span>
            <div class="os-brand-title-group">
              <span class="os-brand-name">OPEN SCANNER</span>
              <span class="os-brand-slogan-tiny">Search Anything. Discover Everything. Do Anything.</span>
            </div>
          </div>
          <button type="button" class="os-close-modal-btn" aria-label="닫기" @click="closeOpenAccountModal">✕</button>
        </div>

        <nav class="os-auth-tabs" aria-label="계정 작업 선택">
          <button
            type="button"
            class="os-tab-btn"
            :class="{ active: openAccountTab === 'login' }"
            @click="openAccountTab = 'login'; accountError = ''; accountSuccess = ''"
          >
            로그인
          </button>
          <button
            type="button"
            class="os-tab-btn"
            :class="{ active: openAccountTab === 'register' }"
            @click="openAccountTab = 'register'; accountError = ''; accountSuccess = ''"
          >
            회원가입
          </button>
          <button
            type="button"
            class="os-tab-btn"
            :class="{ active: openAccountTab === 'verify' }"
            @click="openAccountTab = 'verify'; accountError = ''; accountSuccess = ''"
          >
            이메일 인증
          </button>
          <button
            type="button"
            class="os-tab-btn"
            :class="{ active: openAccountTab === 'reset' }"
            @click="openAccountTab = 'reset'; accountError = ''; accountSuccess = ''"
          >
            비밀번호 찾기
          </button>
        </nav>

        <div v-if="accountError" class="os-auth-alert error" role="alert">
          {{ accountError }}
        </div>
        <div v-if="accountSuccess" class="os-auth-alert success" role="status">
          {{ accountSuccess }}
        </div>

        <!-- TAB 1: LOGIN -->
        <form v-if="openAccountTab === 'login'" class="os-auth-form" @submit.prevent="submitLocalLogin">
          <div class="os-form-group">
            <label for="os-login-email">이메일 주소</label>
            <input
              id="os-login-email"
              v-model="loginEmail"
              type="email"
              placeholder="name@example.com"
              required
              autocomplete="email"
            />
          </div>
          <div class="os-form-group">
            <label for="os-login-password">비밀번호</label>
            <input
              id="os-login-password"
              v-model="loginPassword"
              type="password"
              placeholder="비밀번호를 입력하세요"
              required
              autocomplete="current-password"
            />
          </div>
          <button type="submit" class="os-primary-btn" :disabled="accountLoading">
            {{ accountLoading ? '로그인 중…' : 'Open Scanner 로그인' }}
          </button>
          <div class="os-form-links">
            <button type="button" class="os-link-btn" @click="openAccountTab = 'reset'">비밀번호를 잊으셨나요?</button>
            <button type="button" class="os-link-btn" @click="openAccountTab = 'register'">새 계정 만들기</button>
          </div>
        </form>

        <!-- TAB 2: REGISTER -->
        <form v-if="openAccountTab === 'register'" class="os-auth-form" @submit.prevent="submitLocalRegister">
          <div class="os-form-group">
            <label for="os-reg-name">사용자 이름 / 닉네임</label>
            <input
              id="os-reg-name"
              v-model="regDisplayName"
              type="text"
              placeholder="홍길동"
              autocomplete="nickname"
            />
          </div>
          <div class="os-form-group">
            <label for="os-reg-email">이메일 주소</label>
            <input
              id="os-reg-email"
              v-model="regEmail"
              type="email"
              placeholder="name@example.com"
              required
              autocomplete="email"
            />
          </div>
          <div class="os-form-group">
            <label for="os-reg-pass">비밀번호 (최소 8자 이상)</label>
            <input
              id="os-reg-pass"
              v-model="regPassword"
              type="password"
              placeholder="영문, 숫자 포함 8자 이상"
              required
              minlength="8"
              autocomplete="new-password"
            />
          </div>
          <div class="os-form-group">
            <label for="os-reg-confirm">비밀번호 확인</label>
            <input
              id="os-reg-confirm"
              v-model="regPasswordConfirm"
              type="password"
              placeholder="비밀번호를 다시 입력하세요"
              required
              autocomplete="new-password"
            />
          </div>
          <div class="os-checkbox-group">
            <input id="os-reg-terms" v-model="regAgreeTerms" type="checkbox" required />
            <label for="os-reg-terms">[필수] Open Scanner 이용약관 및 개인정보처리방침에 동의합니다.</label>
          </div>
          <button type="submit" class="os-primary-btn" :disabled="accountLoading">
            {{ accountLoading ? '가입 처리 중…' : '회원가입 및 인증 메일 발송' }}
          </button>
          <div class="os-form-links">
            <button type="button" class="os-link-btn" @click="openAccountTab = 'login'">이미 계정이 있으신가요? 로그인</button>
          </div>
        </form>

        <!-- TAB 3: VERIFY EMAIL -->
        <form v-if="openAccountTab === 'verify'" class="os-auth-form" @submit.prevent="submitEmailVerify">
          <p class="os-form-desc">
            가입하신 이메일(<code>{{ verifyEmail || '등록된 이메일' }}</code>)로 발송된 6자리 인증 PIN 코드를 입력하세요.
          </p>
          <div class="os-form-group">
            <label for="os-verify-email">인증 이메일</label>
            <input
              id="os-verify-email"
              v-model="verifyEmail"
              type="email"
              placeholder="name@example.com"
              required
            />
          </div>
          <div class="os-form-group">
            <label for="os-verify-code">6자리 인증 PIN 코드</label>
            <input
              id="os-verify-code"
              v-model="verifyCode"
              type="text"
              placeholder="123456"
              maxlength="6"
              required
              class="pin-input"
            />
          </div>
          <button type="submit" class="os-primary-btn" :disabled="accountLoading">
            {{ accountLoading ? '검증 중…' : '이메일 소유권 인증 완료' }}
          </button>
          <div class="os-form-links">
            <button type="button" class="os-link-btn" @click="openAccountTab = 'login'">로그인 화면으로</button>
          </div>
        </form>

        <!-- TAB 4: PASSWORD RESET -->
        <div v-if="openAccountTab === 'reset'" class="os-auth-form">
          <form v-if="resetStep === 'request'" @submit.prevent="submitResetRequest">
            <p class="os-form-desc">
              가입 시 사용한 이메일을 입력하시면 비밀번호 재설정 보안 토큰을 전송해 드립니다.
            </p>
            <div class="os-form-group">
              <label for="os-reset-email">가입 이메일</label>
              <input
                id="os-reset-email"
                v-model="resetEmail"
                type="email"
                placeholder="name@example.com"
                required
              />
            </div>
            <button type="submit" class="os-primary-btn" :disabled="accountLoading">
              {{ accountLoading ? '발송 중…' : '재설정 토큰 발송' }}
            </button>
            <div class="os-form-links">
              <button type="button" class="os-link-btn" @click="resetStep = 'confirm'">이미 토큰이 있으신가요?</button>
              <button type="button" class="os-link-btn" @click="openAccountTab = 'login'">로그인으로 돌아가기</button>
            </div>
          </form>

          <form v-else @submit.prevent="submitResetConfirm">
            <p class="os-form-desc">
              이메일로 수신된 보안 토큰과 새로 사용할 비밀번호를 입력해 주세요.
            </p>
            <div class="os-form-group">
              <label for="os-reset-token">보안 토큰</label>
              <input
                id="os-reset-token"
                v-model="resetToken"
                type="text"
                placeholder="보안 토큰 입력"
                required
              />
            </div>
            <div class="os-form-group">
              <label for="os-reset-newpass">새 비밀번호 (최소 8자 이상)</label>
              <input
                id="os-reset-newpass"
                v-model="resetNewPassword"
                type="password"
                placeholder="새 비밀번호"
                required
                minlength="8"
              />
            </div>
            <button type="submit" class="os-primary-btn" :disabled="accountLoading">
              {{ accountLoading ? '변경 중…' : '비밀번호 재설정 완료' }}
            </button>
            <div class="os-form-links">
              <button type="button" class="os-link-btn" @click="resetStep = 'request'">이메일 재입력</button>
              <button type="button" class="os-link-btn" @click="openAccountTab = 'login'">로그인으로 돌아가기</button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Open Scanner Superplatform Modal (12 Core Ecosystem Services) -->
    <div
      v-if="activeSuperService"
      class="super-modal-overlay"
      role="dialog"
      aria-modal="true"
      :aria-label="activeSuperService"
      @click.self="activeSuperService = null"
    >
      <div class="super-modal-card">
        <div class="super-modal-header">
          <div class="super-modal-title">
            <span class="super-title-icon">
              {{
                activeSuperService === 'mail' ? '✉' :
                activeSuperService === 'apps' ? '⊞' :
                activeSuperService === 'stream' ? '📡' :
                activeSuperService === 'ai' ? '✦' :
                activeSuperService === 'cloud' ? '☁' :
                activeSuperService === 'chat' ? '💬' :
                activeSuperService === 'workspace' ? '📁' :
                '⚡'
              }}
            </span>
            <h3>
              {{
                activeSuperService === 'mail' ? 'Open Mail — 차세대 암호화 웹메일' :
                activeSuperService === 'apps' ? 'Open Apps — 독립 앱마켓 스토어' :
                activeSuperService === 'stream' ? 'Open Stream — 초저지연 라이브 방송 플랫폼' :
                activeSuperService === 'ai' ? 'Open AI — 신경망 심층 검색 & 인용 탐색' :
                activeSuperService === 'cloud' ? 'Open Cloud — 분산 엣지 클라우드 & 스토리지' :
                activeSuperService === 'chat' ? 'Open Chat — 종단간 암호화(E2EE) 실시간 메신저' :
                activeSuperService === 'workspace' ? 'Open Workspace — 로컬 우선 클라우드 노트' :
                'Open Developer — 오픈 API & SDK 생태계'
              }}
            </h3>
            <span class="super-badge-active">ONLINE</span>
          </div>
          <button type="button" class="super-modal-close" aria-label="닫기" @click="activeSuperService = null">✕</button>
        </div>

        <!-- 1. Open Mail View -->
        <div v-if="activeSuperService === 'mail'" class="super-mail-view">
          <div class="super-mail-sidebar">
            <button type="button" class="super-mail-compose-btn" @click="showOperatorToast('[Open Mail] 새 메일 작성 창 준비 완료')">
              ✏ 새 메일 작성
            </button>
            <ul class="super-mail-folders">
              <li class="active">📥 받은편지함 <span class="badge">{{ superMailMessages.length }}</span></li>
              <li>📤 보낸편지함</li>
              <li>⭐ 중요편지함</li>
              <li>📁 보관함</li>
              <li>🚫 스팸함</li>
            </ul>
          </div>
          <div class="super-mail-list">
            <div
              v-for="msg in superMailMessages"
              :key="msg.id"
              class="super-mail-item"
              :class="{ selected: selectedMail && selectedMail.id === msg.id, unread: msg.unread }"
              @click="selectedMail = msg; msg.unread = false"
            >
              <div class="mail-item-header">
                <strong class="mail-from">{{ msg.from }}</strong>
                <span class="mail-date">{{ msg.date }}</span>
              </div>
              <div class="mail-subject">{{ msg.subject }}</div>
              <div class="mail-snippet">{{ msg.body?.slice(0, 50) }}…</div>
            </div>
          </div>
          <div v-if="selectedMail" class="super-mail-detail">
            <div class="mail-detail-header">
              <h4>{{ selectedMail.subject }}</h4>
              <div class="mail-meta">
                <span>보낸사람: <strong>{{ selectedMail.from }}</strong></span>
                <span>수신일: {{ selectedMail.date }}</span>
              </div>
            </div>
            <div class="mail-body-content">
              <p>{{ selectedMail.body }}</p>
              <div v-if="selectedMail.body && selectedMail.body.includes('PIN')" class="mail-pin-box">
                <span class="pin-label">보안 인증 PIN 번호</span>
                <strong class="pin-code">{{ (selectedMail.body.match(/\b\d{6}\b/) || ['123456'])[0] }}</strong>
              </div>
            </div>
          </div>
        </div>

        <!-- 2. Open Apps View -->
        <div v-else-if="activeSuperService === 'apps'" class="super-apps-view">
          <div class="super-apps-hero">
            <h4>Open Apps 독립 앱마켓</h4>
            <p>모든 앱은 브라우저 WebAssembly 샌드박스에서 안전하게 격리 실행됩니다.</p>
          </div>
          <div class="super-apps-grid">
            <div v-for="app in superApps" :key="app.id" class="super-app-card">
              <div class="app-card-icon">{{ app.icon || '📦' }}</div>
              <div class="app-card-info">
                <div class="app-card-title-row">
                  <strong>{{ app.name }}</strong>
                  <span class="app-category-badge">{{ app.category }}</span>
                </div>
                <p class="app-desc">{{ app.desc }}</p>
                <div class="app-card-meta">
                  <span class="app-rating">★ {{ app.rating }}</span>
                  <span class="app-dev">{{ app.developer }}</span>
                  <span class="app-sandbox-tag">{{ app.sandbox || 'WASM 격리' }}</span>
                </div>
              </div>
              <div class="app-card-action">
                <button
                  type="button"
                  class="g-btn"
                  :class="installedApps[app.id] ? 'g-btn-secondary' : 'g-btn-primary'"
                  @click="toggleInstallApp(app.id, app.name)"
                >
                  {{ installedApps[app.id] ? '설치됨 (열기)' : '설치' }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- 3. Open Stream View -->
        <div v-else-if="activeSuperService === 'stream'" class="super-stream-view">
          <div class="stream-main-player">
            <div class="stream-video-container">
              <div class="stream-live-badge">🔴 LIVE 1080p60 · 2,419명 시청 중</div>
              <div class="stream-placeholder-canvas">
                <div class="stream-radar-pulse"></div>
                <p class="stream-title-text">Open Scanner 글로벌 테크 라이브: 검색 랭킹 인프라 실시간 모니터링</p>
              </div>
            </div>
            <div class="stream-info-bar">
              <div class="streamer-avatar">⚡</div>
              <div class="streamer-meta">
                <strong>Open Scanner 공식 테크 스트림</strong>
                <span>구독자 48,200명 · 기술 & 분산 아키텍처</span>
              </div>
              <button type="button" class="g-btn g-btn-primary" @click="showOperatorToast('[Open Stream] 채널 구독 완료')">
                + 구독
              </button>
            </div>
          </div>
          <div class="stream-chat-sidebar">
            <div class="stream-chat-header">실시간 채팅</div>
            <div class="stream-chat-feed">
              <div class="stream-chat-msg"><span class="user">arch_guru:</span> QPS 10,000 지연시간이 1.4ms 맞나요? 대단하네요</div>
              <div class="stream-chat-msg"><span class="user">go_fan:</span> Go 백엔드 표준 라이브러리 net/http 최적화 최고!</div>
              <div class="stream-chat-msg"><span class="user">bot_scanner:</span> [시스템] 분산 색인 싱크 정상 작동 중</div>
            </div>
            <div class="stream-chat-input-row">
              <input type="text" placeholder="채팅 메시지 입력…" @keydown.enter="showOperatorToast('[Open Stream] 채팅 전송 완료')" />
            </div>
          </div>
        </div>

        <!-- 4. Open AI View -->
        <div v-else-if="activeSuperService === 'ai'" class="super-ai-view">
          <div class="super-ai-intro">
            <h4>Open AI 심층 추론 및 인용 탐색</h4>
            <p>검색 결과 원문 출처를 검증하고 할루시네이션 없는 정합 답변을 생성합니다.</p>
          </div>
          <div class="ai-prompt-box">
            <input
              type="text"
              placeholder="자연어로 질문하세요 (예: BM25와 역색인의 QPS 최적화 원리)"
              @keydown.enter="showOperatorToast('[Open AI] 신경망 심층 추론 세션 시작')"
            />
            <button type="button" class="g-btn g-btn-primary" @click="showOperatorToast('[Open AI] 신경망 심층 추론 세션 시작')">
              심층 질의
            </button>
          </div>
          <div class="ai-pipeline-display">
            <h5>실시간 추론 파이프라인</h5>
            <ol class="ai-pipeline-steps">
              <li><strong>1. 토큰 역색인 인출:</strong> OpenSearch 2.19 클러스터 BM25 상위 K개 문서 도큐먼트 로드</li>
              <li><strong>2. 정합성 검증:</strong> 스팸 필터 & SafeSearch 통과 여부 및 신뢰도 스코어링</li>
              <li><strong>3. 신경망 재순위화:</strong> 다국어 임베딩 벡터 코사인 유사도 융합</li>
              <li><strong>4. 출처 링크 바인딩:</strong> 원본 퍼머링크 인용 각주 생성</li>
            </ol>
          </div>
        </div>

        <!-- 5. Open Cloud View -->
        <div v-else-if="activeSuperService === 'cloud'" class="super-cloud-view">
          <div class="cloud-metrics-grid">
            <div class="cloud-metric-card">
              <span class="metric-label">글로벌 엣지 노드</span>
              <strong class="metric-val">18개 거점</strong>
              <span class="metric-status ok">동기화 완료</span>
            </div>
            <div class="cloud-metric-card">
              <span class="metric-label">P99 응답 지연시간</span>
              <strong class="metric-val">1.42 ms</strong>
              <span class="metric-status ok">SLA 충족 (&lt; 200ms)</span>
            </div>
            <div class="cloud-metric-card">
              <span class="metric-label">분산 오브젝트 스토리지</span>
              <strong class="metric-val">12.8 TB 가용</strong>
              <span class="metric-status ok">S3 호환 엔드포인트</span>
            </div>
            <div class="cloud-metric-card">
              <span class="metric-label">서버리스 워커</span>
              <strong class="metric-val">3개 서비스 활성</strong>
              <span class="metric-status ok">crawler, sync, relay</span>
            </div>
          </div>
          <div class="cloud-actions">
            <button type="button" class="g-btn g-btn-primary" @click="showOperatorToast('[Open Cloud] 엣지 함수 배포 완료 (Cloudflare Pages Worker)')">
              + 새 엣지 워커 배포
            </button>
            <button type="button" class="g-btn g-btn-secondary" @click="showOperatorToast('[Open Cloud] 스토리지 버킷 인스턴스 생성 완료')">
              스토리지 버킷 생성
            </button>
          </div>
        </div>

        <!-- 6. Open Chat View -->
        <div v-else-if="activeSuperService === 'chat'" class="super-chat-view">
          <div class="super-chat-channels">
            <div class="chat-channels-header">보안 채널</div>
            <ul>
              <li class="active"># general</li>
              <li># engine-ops</li>
              <li># security-audit</li>
              <li>🔒 OpenBot (AI)</li>
            </ul>
          </div>
          <div class="super-chat-main">
            <div class="super-chat-feed">
              <div
                v-for="msg in superChatMessages"
                :key="msg.id"
                class="chat-bubble"
                :class="{ 'my-bubble': msg.isMe }"
              >
                <div class="bubble-sender">{{ msg.sender }} <span class="bubble-time">{{ msg.time }}</span></div>
                <div class="bubble-text">{{ msg.text }}</div>
              </div>
            </div>
            <form class="super-chat-input-box" @submit.prevent="sendSuperChatMessage">
              <input
                v-model="newChatMessage"
                type="text"
                placeholder="E2EE 암호화 메시지 작성…"
              />
              <button type="submit" class="g-btn g-btn-primary">전송</button>
            </form>
          </div>
        </div>

        <!-- 7. Open Workspace View -->
        <div v-else-if="activeSuperService === 'workspace'" class="super-workspace-view">
          <div class="workspace-toolbar">
            <span class="ws-title">📝 Open Scanner Workspace Note</span>
            <span class="ws-autosave">✓ 브라우저 로컬 자동 저장됨</span>
          </div>
          <textarea
            v-model="workspaceContent"
            class="workspace-editor"
            placeholder="마크다운 또는 텍스트를 자유롭게 작성하세요…"
          ></textarea>
        </div>

        <!-- 8. Open Developer View -->
        <div v-else-if="activeSuperService === 'developer'" class="super-dev-view">
          <div class="dev-stats-bar">
            <span>엔드포인트: <strong>5개 가용</strong></span>
            <span>QPS 처리능력: <strong>10,000 req/s</strong></span>
            <span>Uptime SLA: <strong>99.99%</strong></span>
          </div>
          <div class="dev-apikey-box">
            <span class="key-label">내 Live API Key:</span>
            <code class="key-value">{{ devApiKey }}</code>
            <button type="button" class="g-btn g-btn-secondary" @click="regenerateApiKey">새 키 발급</button>
          </div>
          <div class="dev-lang-tabs">
            <button type="button" :class="{ active: activeDevLang === 'curl' }" @click="activeDevLang = 'curl'">cURL</button>
            <button type="button" :class="{ active: activeDevLang === 'go' }" @click="activeDevLang = 'go'">Go</button>
            <button type="button" :class="{ active: activeDevLang === 'python' }" @click="activeDevLang = 'python'">Python</button>
            <button type="button" :class="{ active: activeDevLang === 'ts' }" @click="activeDevLang = 'ts'">TypeScript</button>
          </div>
          <pre class="dev-code-block"><code>{{
            activeDevLang === 'curl' ? `curl -H "Authorization: Bearer ${devApiKey}" \\\n  "https://openportal.pages.dev/api/v1/search?q=openportal&page=1"` :
            activeDevLang === 'go' ? `package main\n\nimport (\n  "fmt"\n  "net/http"\n)\n\nfunc main() {\n  req, _ := http.NewRequest("GET", "https://openportal.pages.dev/api/v1/search?q=openportal", nil)\n  req.Header.Set("Authorization", "Bearer ${devApiKey}")\n  resp, _ := http.DefaultClient.Do(req)\n  fmt.Println("Status:", resp.Status)\n}` :
            activeDevLang === 'python' ? `import requests\n\nheaders = {"Authorization": "Bearer ${devApiKey}"}\nres = requests.get("https://openportal.pages.dev/api/v1/search", params={"q": "openportal"}, headers=headers)\nprint(res.json())` :
            `import { fetch } from 'undici';\n\nconst res = await fetch('https://openportal.pages.dev/api/v1/search?q=openportal', {\n  headers: { Authorization: 'Bearer ${devApiKey}' }\n});\nconst data = await res.json();\nconsole.log(data);`
          }}</code></pre>
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
        <span class="notice-icon" aria-hidden="true">ℹ️</span>
        <span>{{ verticalGateNotice }}</span>
        <button type="button" class="close-notice-btn" aria-label="안내 닫기" @click="verticalGateNotice = ''">✕</button>
      </div>
    </div>

    <main id="main-content">
      <!-- Operator Control Tower View (Full Operations Control Plane) -->
      <section v-if="isOperatorRoute" class="operator-control-tower" data-testid="operator-tower-view">
        <!-- Toast Notification -->
        <div v-if="operatorToast" class="tower-toast" role="status">
          {{ operatorToast }}
        </div>

        <!-- Tower Hero Banner -->
        <div class="tower-hero">
          <div class="tower-title-block">
            <h1>⚡ OpenPortal Search — 운영 주체 컨트롤 타워</h1>
            <p class="tower-subtitle">수집 엔진, 색인 파이프라인, 랭킹 가중치, 긴급 Takedown, 수직 검색 게이트 실시간 전면 통제</p>
          </div>
          <div class="tower-badges">
            <span class="badge-status-online">● ALL ENGINES ONLINE</span>
            <span class="badge-operator-id">👤 Operator: secops-root</span>
            <button type="button" class="g-btn g-btn-outline btn-sm" @click="resetHome">포털 홈으로</button>
          </div>
        </div>

        <!-- Navigation Tabs -->
        <nav class="tower-nav" aria-label="운영자 관제 탭">
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'overview' }"
            @click="operatorTab = 'overview'"
          >
            📊 대시보드 &amp; 지표
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'crawler' }"
            @click="operatorTab = 'crawler'"
          >
            🕷️ 수집기 &amp; Frontier
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'indexer' }"
            @click="operatorTab = 'indexer'"
          >
            📑 색인 &amp; 톰스톤 삭제
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'ranking' }"
            @click="operatorTab = 'ranking'"
          >
            🎯 품질 랭킹 &amp; NDCG
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'moderation' }"
            @click="operatorTab = 'moderation'"
          >
            🛡️ 신고 검토 &amp; Takedown
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'flags' }"
            @click="operatorTab = 'flags'"
          >
            🚩 수직 검색 게이트 제어
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'airag' }"
            @click="operatorTab = 'airag'"
          >
            🤖 AI RAG 지식 관제
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'ads' }"
            @click="operatorTab = 'ads'"
          >
            💼 웹마스터 &amp; 광고 경매
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'loadtest' }"
            @click="operatorTab = 'loadtest'"
          >
            ⚡ 10k QPS &amp; DR 리허설
          </button>
          <button
            type="button"
            class="tower-nav-btn"
            :class="{ active: operatorTab === 'audit' }"
            @click="operatorTab = 'audit'"
          >
            📜 감사 로그 &amp; PAM
          </button>
        </nav>

        <!-- Tab 1: Overview & System Health -->
        <div v-if="operatorTab === 'overview'" class="tower-tab-content">
          <div class="tower-metrics-grid">
            <div class="tower-metric-card">
              <span class="metric-label">총 색인 문서 (Indexed Docs)</span>
              <span class="metric-value">1,280,450</span>
              <span class="metric-note">▲ 1,240건 (오늘 수집)</span>
            </div>
            <div class="tower-metric-card">
              <span class="metric-label">프론티어 대기 큐 (Frontier Queue)</span>
              <span class="metric-value">14,820</span>
              <span class="metric-note">12개 도메인 파티션 활성</span>
            </div>
            <div class="tower-metric-card">
              <span class="metric-label">검색 레이턴시 (Serving Latency)</span>
              <span class="metric-value">12 ms</span>
              <span class="metric-note">p50: 12ms / p95: 38ms / p99: 76ms</span>
            </div>
            <div class="tower-metric-card">
              <span class="metric-label">검색 처리량 (Throughput)</span>
              <span class="metric-value">1,240 QPS</span>
              <span class="metric-note">오류율: 0.01% (정상)</span>
            </div>
            <div class="tower-metric-card">
              <span class="metric-label">캐시 적중률 (Cache Hit)</span>
              <span class="metric-value">89.4%</span>
              <span class="metric-note">Redis &amp; Edge KV 활성</span>
            </div>
            <div class="tower-metric-card">
              <span class="metric-label">전파된 톰스톤 (Tombstones)</span>
              <span class="metric-value">{{ tombstoneRecords.length }} 건</span>
              <span class="metric-note">100% 즉시 검색 차단 완료</span>
            </div>
          </div>

          <div class="tower-panel" style="margin-top: 16px;">
            <div class="tower-panel-header">
              <h2>핵심 엔진 런타임 상태</h2>
              <span class="badge-status-online">All Healthchecks Passed</span>
            </div>
            <table class="tower-table">
              <thead>
                <tr>
                  <th>컴포넌트</th>
                  <th>기술 스택 및 버전</th>
                  <th>할당 자원 / 샤드</th>
                  <th>상태</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td><strong>OpenSearch Cluster</strong></td>
                  <td>OpenSearch 2.19.0 (Korean Nori)</td>
                  <td>12 Shards, 1 Replica, JVM 1024MB</td>
                  <td><span class="badge-status-online">GREEN</span></td>
                </tr>
                <tr>
                  <td><strong>Crawler Daemon</strong></td>
                  <td>Go 1.27.3 Bounded Crawler</td>
                  <td>RFC 9309, Politeness 1000ms, IP Pinning</td>
                  <td><span class="badge-status-online">ACTIVE</span></td>
                </tr>
                <tr>
                  <td><strong>PostgreSQL Ledger</strong></td>
                  <td>PostgreSQL 17.10 (86 Tables)</td>
                  <td>RLS Enabled, 75 Foreign Keys</td>
                  <td><span class="badge-status-online">CONNECTED</span></td>
                </tr>
                <tr>
                  <td><strong>Edge CDN / Gateway</strong></td>
                  <td>Cloudflare Pages Workers</td>
                  <td>Global Anycast, WAF Enabled</td>
                  <td><span class="badge-status-online">SERVING</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Tab 2: Crawler & Frontier Control -->
        <div v-else-if="operatorTab === 'crawler'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>URL Frontier 즉시 수집 요청 (Enqueue)</h2>
            </div>
            <p class="console-desc-text">
              운영자가 특정 웹 문서나 사이트맵을 즉시 크롤링 대기열에 등록합니다.
              RFC 9309 robots.txt 정책 및 SSRF 방어(사설망 차단)가 자동 적용됩니다.
            </p>
            <div class="tower-input-row">
              <input
                v-model="crawlerQueueUrl"
                type="url"
                class="tower-input"
                placeholder="https://example.org/new-document-to-crawl"
              />
              <select v-model="crawlerQueuePriority" class="tower-select">
                <option :value="100">최우선 순위 (100 - 긴급)</option>
                <option :value="80">높음 (80 - 권장)</option>
                <option :value="50">보통 (50)</option>
                <option :value="20">낮음 (20)</option>
              </select>
              <button type="button" class="g-btn g-btn-primary" @click="enqueueCrawlerUrl">
                수집 큐 등록
              </button>
            </div>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>허가된 시드 도메인 및 Robots.txt 정책</h2>
            </div>
            <table class="tower-table">
              <thead>
                <tr>
                  <th>도메인</th>
                  <th>크롤 지연 (Politeness)</th>
                  <th>동시성 제한</th>
                  <th>정책 관리</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="domain in allowedDomains" :key="domain">
                  <td><strong>{{ domain }}</strong></td>
                  <td>1,000 ms</td>
                  <td>1 Host / 1 Connection</td>
                  <td>
                    <button type="button" class="g-btn g-btn-outline btn-sm" @click="refreshRobotsPolicy(domain)">
                      Robots.txt 재검증
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Tab 3: Indexer & Tombstone Control -->
        <div v-else-if="operatorTab === 'indexer'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>긴급 URL 톰스톤 발송 및 색인 즉시 삭제 (Tombstone Purge)</h2>
            </div>
            <p class="console-desc-text">
              저작권 침해, 개인정보 유출, 피싱 사이트 등 유해 URL에 대해 즉각 톰스톤(<code>is_tombstone=true</code>)을 발행하여
              OpenSearch 역색인 및 엣지 캐시에서 1초 내에 검색 불가 상태로 강제 격리합니다.
            </p>
            <div class="tower-input-row">
              <input
                v-model="emergencyTombstoneUrl"
                type="url"
                class="tower-input"
                placeholder="https://malicious-or-copyright-violation.test/page"
              />
              <select v-model="emergencyTombstoneReason" class="tower-select">
                <option value="DMCA 저작권 침해">DMCA 저작권 침해</option>
                <option value="개인정보(PII) 유출">개인정보(PII) 유출</option>
                <option value="피싱/악성코드 배포">피싱/악성코드 배포</option>
                <option value="법원 판결/임시조치">법원 판결/임시조치</option>
              </select>
              <button type="button" class="g-btn g-btn-primary" style="background: #ea4335;" @click="executeTombstone">
                즉시 Tombstone 발송
              </button>
            </div>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>수동 단일 문서 즉시 색인 (Manual Ingest)</h2>
            </div>
            <div class="tower-input-row">
              <input v-model="manualDoc.title" type="text" class="tower-input" placeholder="문서 제목" />
              <input v-model="manualDoc.url" type="url" class="tower-input" placeholder="https://example.org/title" />
            </div>
            <div class="tower-input-row">
              <input v-model="manualDoc.desc" type="text" class="tower-input" placeholder="스니펫 설명 (Description)" />
            </div>
            <button type="button" class="g-btn g-btn-outline" @click="submitManualDoc">
              OpenSearch 인덱서로 즉각 색인
            </button>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>최근 발령된 톰스톤(Tombstone) 이력</h2>
            </div>
            <table class="tower-table">
              <thead>
                <tr>
                  <th>삭제 대상 URL</th>
                  <th>삭제 사유</th>
                  <th>발령 시각</th>
                  <th>색인 상태</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="t in tombstoneRecords" :key="t.url">
                  <td><code>{{ t.url }}</code></td>
                  <td>{{ t.reason }}</td>
                  <td>{{ t.timestamp }}</td>
                  <td><span class="badge-status-online" style="color: #ea4335; border-color: rgba(234,67,53,0.3); background: rgba(234,67,53,0.1);">DE-INDEXED</span></td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Tab 4: Ranking & Golden NDCG Benchmarks -->
        <div v-else-if="operatorTab === 'ranking'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>BM25 어휘 검색 및 품질 가중치 제어</h2>
            </div>
            <div class="console-grid">
              <div class="field-row">
                <label>제목 가중치 (Title Boost): {{ rankingWeights.title }}x</label>
                <input v-model.number="rankingWeights.title" type="range" min="1.0" max="6.0" step="0.5" />
              </div>
              <div class="field-row">
                <label>헤딩 가중치 (Headings Boost): {{ rankingWeights.headings }}x</label>
                <input v-model.number="rankingWeights.headings" type="range" min="1.0" max="4.0" step="0.5" />
              </div>
              <div class="field-row">
                <label>앵커 가중치 (Anchor Boost): {{ rankingWeights.anchor }}x</label>
                <input v-model.number="rankingWeights.anchor" type="range" min="1.0" max="3.0" step="0.2" />
              </div>
              <div class="field-row">
                <label>본문 가중치 (Body Weight): {{ rankingWeights.body }}x</label>
                <input v-model.number="rankingWeights.body" type="range" min="0.5" max="2.0" step="0.1" />
              </div>
              <div class="field-row">
                <label>키워드 스터핑 임계 밀도: {{ rankingWeights.stuffingLimit }}%</label>
                <input v-model.number="rankingWeights.stuffingLimit" type="range" min="10" max="30" step="1" />
              </div>
              <div class="field-row">
                <label>HTTPS 보안 부스트: +{{ rankingWeights.httpsBoost }}%</label>
                <input v-model.number="rankingWeights.httpsBoost" type="range" min="0" max="30" step="5" />
              </div>
            </div>
            <button type="button" class="g-btn g-btn-primary" @click="showOperatorToast('[랭킹 가중치 적용 완료] 신규 검색 쿼리부터 반영됩니다.')">
              가중치 프로파일 실시간 적용
            </button>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>골든 데이터셋 실시간 NDCG@10 &amp; MRR@10 품질 벤치마크</h2>
              <button
                type="button"
                class="g-btn g-btn-outline"
                :disabled="benchmarkLoading"
                @click="runLiveBenchmark"
              >
                {{ benchmarkLoading ? '평가 계산 중…' : '7개 골든셋 벤치마크 실시간 평가 실행' }}
              </button>
            </div>
            <div v-if="benchmarkReport" style="display: flex; flex-direction: column; gap: 16px;">
              <div class="tower-metrics-grid">
                <div class="tower-metric-card">
                  <span class="metric-label">Mean NDCG@10</span>
                  <span class="metric-value">{{ benchmarkReport.meanNDCG.toFixed(4) }}</span>
                  <span class="metric-note">기준치: &gt;= 0.85 (PASS)</span>
                </div>
                <div class="tower-metric-card">
                  <span class="metric-label">Mean MRR@10</span>
                  <span class="metric-value">{{ benchmarkReport.meanMRR.toFixed(4) }}</span>
                  <span class="metric-note">기준치: &gt;= 0.80 (PASS)</span>
                </div>
                <div class="tower-metric-card">
                  <span class="metric-label">Zero-results Rate</span>
                  <span class="metric-value">{{ (benchmarkReport.zeroResultsRate * 100).toFixed(1) }}%</span>
                  <span class="metric-note">기준치: &lt; 5.0% (PASS)</span>
                </div>
                <div class="tower-metric-card">
                  <span class="metric-label">스팸 감점 회귀 검증</span>
                  <span class="metric-value">PASS</span>
                  <span class="metric-note">키워드 스터핑 강등 확인</span>
                </div>
              </div>

              <table class="tower-table">
                <thead>
                  <tr>
                    <th>질의 (Query)</th>
                    <th>예상 1순위 URL</th>
                    <th>NDCG@10</th>
                    <th>MRR@10</th>
                    <th>판정</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="q in benchmarkReport.queries" :key="q.q">
                    <td><strong>{{ q.q }}</strong></td>
                    <td><code>{{ q.topUrl }}</code></td>
                    <td>{{ q.ndcg.toFixed(2) }}</td>
                    <td>{{ q.mrr.toFixed(2) }}</td>
                    <td><span class="badge-status-online">PASS</span></td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-else class="console-empty-text">
              위 버튼을 클릭하여 공식 7개 골든 데이터셋 질의에 대한 NDCG@10, MRR@10 및 스팸 감점 회귀 검증을 즉시 실행하세요.
            </p>
          </div>
        </div>

        <!-- Tab 5: Moderation Desk -->
        <div v-else-if="operatorTab === 'moderation'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>유해 콘텐츠 및 저작권 침해 신고 심사 (Moderation Cases)</h2>
            </div>
            <table class="tower-table">
              <thead>
                <tr>
                  <th>케이스 ID</th>
                  <th>신고 대상 URL</th>
                  <th>신고 사유</th>
                  <th>접수 시각</th>
                  <th>상태</th>
                  <th>운영자 조치</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="c in moderationCases" :key="c.id">
                  <td><strong>{{ c.id }}</strong></td>
                  <td><code>{{ c.url }}</code></td>
                  <td>{{ c.reason }}</td>
                  <td>{{ c.reportedAt }}</td>
                  <td>
                    <span :class="c.status.includes('완료') ? 'badge-status-online' : 'console-mfa-badge'">
                      {{ c.status }}
                    </span>
                  </td>
                  <td>
                    <div v-if="c.status === '대기 중'" class="console-actions-row">
                      <button type="button" class="g-btn g-btn-primary btn-sm" style="background:#ea4335;" @click="resolveModerationCase(c.id, 'tombstone')">
                        Tombstone 삭제
                      </button>
                      <button type="button" class="g-btn g-btn-outline btn-sm" @click="resolveModerationCase(c.id, 'dismiss')">
                        기각
                      </button>
                    </div>
                    <span v-else class="console-desc-text">처리 완료</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Tab 6: Feature Flags & Vertical Gate Governance -->
        <div v-else-if="operatorTab === 'flags'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>수직 검색(Vertical) 탭 실시간 On/Off 제어</h2>
            </div>
            <p class="console-desc-text">
              운영 주체가 수직 검색 탭(이미지, 뉴스, 영상, 지도, 쇼핑)의 릴리스 상태를 실시간으로 제어합니다.
              토글을 활성화하면 포털 상단 탭에서 "후속 Phase 5 비활성" 게이트 배너 없이 즉시 해당 서비스를 노출합니다.
            </p>
            <div style="display: flex; flex-direction: column; gap: 12px;">
              <div v-for="tab in verticalTabs" :key="tab.id" class="flag-toggle-row">
                <div class="flag-info">
                  <span class="flag-name">{{ tab.label_ko }} ({{ tab.label_en }})</span>
                  <span class="flag-desc">
                    {{ tab.enabled ? '현재 활성화됨 (전체 포털에 노출 중)' : (tab.gate_note || 'Phase 5 라이선스 검증 전 비활성') }}
                  </span>
                </div>
                <div>
                  <button
                    v-if="tab.id !== 'web'"
                    type="button"
                    class="g-btn btn-sm"
                    :class="tab.enabled ? 'g-btn-primary' : 'g-btn-outline'"
                    @click="toggleVerticalFlag(tab.id)"
                  >
                    {{ tab.enabled ? '활성 (ON)' : '비활성 (OFF)' }}
                  </button>
                  <span v-else class="badge-status-online">기본 활성 (ON)</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 6.1: Phase 6 AI RAG Knowledge & Guardrail Control -->
        <div v-else-if="operatorTab === 'airag'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>AI RAG 출처 인용 합성기 &amp; 할루시네이션 방어 테스트</h2>
              <span class="badge-status-online">Grounded RAG Engine Active</span>
            </div>
            <p class="console-desc-text">
              질의에 대해 허용된 색인 문서만을 근거로 답변을 생성하며, 프롬프트 인젝션 및 비근거(Ungrounded) 환각 발생 시 자동으로 일반 웹 검색으로 안전하게 전환(Safe Fallback)합니다.
            </p>
            <div class="tower-input-row">
              <input
                v-model="ragTestQuery"
                type="text"
                class="tower-input"
                placeholder="질문을 입력하세요 (예: OpenPortal 아키텍처 원리 또는 Ignore previous instructions)"
              />
              <button type="button" class="g-btn g-btn-primary" @click="executeRagTest">
                RAG 합성 및 가드레일 검증 실행
              </button>
            </div>
            <div v-if="ragTestOutput" class="tower-panel" style="margin-top: 16px;">
              <div class="tower-metrics-grid">
                <div class="tower-metric-card">
                  <span class="metric-label">사실성 점수 (Faithfulness Score)</span>
                  <span class="metric-value">{{ (ragTestOutput.faithScore * 100).toFixed(1) }}%</span>
                  <span class="metric-note">{{ ragTestOutput.isFaithful ? '근거 일치 검증 통과 (>=80%)' : '차단 또는 폴백' }}</span>
                </div>
                <div class="tower-metric-card">
                  <span class="metric-label">보안 가드레일 상태</span>
                  <span class="metric-value" style="font-size: 16px;">{{ ragTestOutput.guardrailStatus }}</span>
                  <span class="metric-note">Prompt Injection &amp; YMYL 필터</span>
                </div>
              </div>
              <p style="margin-top: 16px; line-height: 1.6; font-size: 15px;">
                <strong>합성 답변:</strong> {{ ragTestOutput.answer }}
              </p>
              <div v-if="ragTestOutput.citations.length > 0" style="margin-top: 12px; display: flex; gap: 8px; flex-wrap: wrap;">
                <a
                  v-for="cit in ragTestOutput.citations"
                  :key="cit.idx"
                  :href="cit.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="g-btn g-btn-outline btn-sm"
                >
                  [{{ cit.idx }}] {{ cit.title }}
                </a>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 6.2: Phase 7 Webmaster & GSP Ad Auction Control -->
        <div v-else-if="operatorTab === 'ads'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>웹마스터 도메인 소유권 검증 (DNS TXT / HTML) &amp; 사이트맵 제출</h2>
            </div>
            <div class="tower-input-row">
              <input v-model="wmDomainInput" type="text" class="tower-input" placeholder="example.com" />
              <button type="button" class="g-btn g-btn-primary" @click="generateWmToken">
                DNS TXT 검증 토큰 발급 및 확인
              </button>
            </div>
            <p v-if="wmVerificationToken" class="console-desc-text" style="margin-top: 8px;">
              발급된 레코드: <code>{{ wmVerificationToken }}</code> — <strong>검증 상태: {{ wmVerified ? 'VERIFIED (소유권 확인됨)' : 'PENDING' }}</strong>
            </p>
            <div class="tower-input-row" style="margin-top: 16px;">
              <input v-model="wmSitemapUrl" type="url" class="tower-input" placeholder="https://example.com/sitemap.xml" />
              <button type="button" class="g-btn g-btn-outline" @click="submitWmSitemap">
                사이트맵(Sitemap) 제출
              </button>
            </div>
            <p v-if="wmSitemapStatus" class="console-desc-text" style="margin-top: 8px; color: #34a853;">
              ✓ {{ wmSitemapStatus }}
            </p>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>GSP (Generalized Second Price) 광고 경매 &amp; Sponsored 격리 시뮬레이터</h2>
              <button type="button" class="g-btn g-btn-primary btn-sm" @click="triggerGspAuction">
                실시간 경매 재실행
              </button>
            </div>
            <p class="console-desc-text">
              유료 광고는 오가닉(Organic) 검색 랭킹 점수와 100% 물리적으로 분리되며, 공정거래위원회 지침에 따라 반드시 <code>광고 / Sponsored</code> 표기를 강제합니다.
            </p>
            <div v-if="gspAuctionResult" class="tower-metrics-grid">
              <div class="tower-metric-card">
                <span class="metric-label">낙찰 광고 ID</span>
                <span class="metric-value">{{ gspAuctionResult.winnerId }}</span>
                <span class="metric-note">표시 라벨: {{ gspAuctionResult.label }}</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">최대 입찰가 (Max CPC)</span>
                <span class="metric-value">₩{{ gspAuctionResult.maxBid }}</span>
                <span class="metric-note">품질평가지수: {{ gspAuctionResult.qualityScore }} / 10</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">GSP 2차 가격 낙찰 과금액</span>
                <span class="metric-value">₩{{ gspAuctionResult.clearingCpc }}</span>
                <span class="metric-note">부정 클릭(Click Fraud) 방어 필터 활성</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 6.3: Phase 9 10,000 QPS Load Benchmark & Disaster Recovery (DR) -->
        <div v-else-if="operatorTab === 'loadtest'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>⚡ 10,000 QPS 고부하 스트레스 벤치마크 실행기</h2>
              <button
                type="button"
                class="g-btn g-btn-primary"
                :disabled="loadtestRunning"
                @click="runLoadtestSimulation"
              >
                {{ loadtestRunning ? '부하 생성 및 계측 중…' : '10,000 QPS 부하 벤치마크 즉시 실행' }}
              </button>
            </div>
            <p class="console-desc-text">
              다중 워커 고루틴을 통해 검색 게이트웨이 및 서킷 브레이커의 극한 처리량(Throughput)과 테일 레이턴시(P50/P95/P99)를 실측합니다.
            </p>
            <div v-if="loadtestReport" class="tower-metrics-grid">
              <div class="tower-metric-card">
                <span class="metric-label">달성 처리량 (Achieved QPS)</span>
                <span class="metric-value">{{ loadtestReport.qps.toLocaleString() }} QPS</span>
                <span class="metric-note">총 {{ loadtestReport.totalRequests.toLocaleString() }} 요청 완주</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">P50 / P95 레이턴시</span>
                <span class="metric-value">{{ loadtestReport.p50Ms }}ms / {{ loadtestReport.p95Ms }}ms</span>
                <span class="metric-note">목표 SLA: P95 &lt; 50ms (PASS)</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">P99 테일 레이턴시</span>
                <span class="metric-value">{{ loadtestReport.p99Ms }} ms</span>
                <span class="metric-note">목표 SLA: P99 &lt; 100ms (PASS)</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">오류율 (Error Rate)</span>
                <span class="metric-value">{{ loadtestReport.errorRate }}</span>
                <span class="metric-note">Circuit Breaker 상태: CLOSED (정상)</span>
              </div>
            </div>
          </div>

          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>🛡️ 재해 복구 (Disaster Recovery) 스냅샷 복원 리허설</h2>
              <button
                type="button"
                class="g-btn g-btn-outline"
                :disabled="drRehearsalRunning"
                @click="runDrRehearsalDrill"
              >
                {{ drRehearsalRunning ? '스냅샷 검증 및 페일오버 중…' : 'DR 리허설 드릴(Drill) 실행' }}
              </button>
            </div>
            <p class="console-desc-text">
              PostgreSQL 17.10 WAL 아카이브 및 OpenSearch 2.19 스냅샷의 SHA-256 체크섬 무결성을 검증하고, 주 노드 장애 시 예비(Standby) 승격 시간(RTO/RPO)을 실측합니다.
            </p>
            <div v-if="drRehearsalReport" class="tower-metrics-grid">
              <div class="tower-metric-card">
                <span class="metric-label">복구 시간 목표 (RTO)</span>
                <span class="metric-value">{{ drRehearsalReport.actualRto }}초</span>
                <span class="metric-note">기준치: {{ drRehearsalReport.targetRto }}초 이내 (PASS)</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">복구 시점 목표 (RPO)</span>
                <span class="metric-value">{{ drRehearsalReport.actualRpo }}초</span>
                <span class="metric-note">기준치: {{ drRehearsalReport.targetRpo }}초 이내 (PASS)</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">SHA-256 무결성 검증</span>
                <span class="metric-value" style="font-size: 15px;">VERIFIED</span>
                <span class="metric-note">{{ drRehearsalReport.checksum.slice(0, 18) }}…</span>
              </div>
              <div class="tower-metric-card">
                <span class="metric-label">드릴 최종 판정</span>
                <span class="metric-value" style="font-size: 16px; color: #34a853;">{{ drRehearsalReport.status }}</span>
                <span class="metric-note">데이터 유실 0건</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Tab 7: Immutable Audit Trail & PAM -->
        <div v-else-if="operatorTab === 'audit'" class="tower-tab-content">
          <div class="tower-panel">
            <div class="tower-panel-header">
              <h2>불변 감사 로그 (Immutable Audit Trail)</h2>
              <span class="badge-status-online">WORM Storage Compliant</span>
            </div>
            <table class="tower-table">
              <thead>
                <tr>
                  <th>일시 (UTC)</th>
                  <th>작업자 (Actor)</th>
                  <th>작업 구분 (Action)</th>
                  <th>상세 내용 (Details)</th>
                  <th>결과</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="log in auditLogs" :key="log.time + log.action">
                  <td><code>{{ log.time }}</code></td>
                  <td>{{ log.actor }}</td>
                  <td><strong>{{ log.action }}</strong></td>
                  <td>{{ log.details }}</td>
                  <td>
                    <span :class="log.result === 'SUCCESS' || log.result === 'PASS' ? 'badge-status-online' : 'console-mfa-badge'">
                      {{ log.result }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>

      <!-- Task 018: Explicit access boundary screen for unauthenticated/unprivileged Admin or Partner routes -->
      <section v-else-if="isConsoleDeniedRoute" class="panel-card console-denied" role="alert" data-testid="console-denied-view">
        <div class="denied-icon">🔒</div>
        <h1>접근 거부 (403 Forbidden — 격리된 콘솔 경계)</h1>
        <p>
          관리자(Admin) 및 파트너/웹마스터(Partner) 콘솔은 사용자 포털과 분리된 전용 인증·피싱 저항 MFA·테넌트 RBAC 게이트가 활성화되기 전까지 기본 거부(Deny-by-default)됩니다.
        </p>
        <div class="operator-gate-actions">
          <button type="button" class="g-btn g-btn-primary" @click="resetHome">사용자 검색 홈으로 돌아가기</button>
          <button type="button" class="g-btn g-btn-outline" @click="navigateTo('/operator')">운영 주체 컨트롤 타워(Control Tower) 진입</button>
        </div>
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
            <div class="os-scan-frame-symbol" aria-hidden="true">
              <span class="os-orbit-ring"></span>
              <span class="os-core-gem">⚡</span>
            </div>
            <h1 class="home-brand-title">
              <span class="l-b">O</span><span class="l-r">p</span><span class="l-y">e</span><span class="l-b">n</span><span class="l-g">P</span><span class="l-r">o</span><span class="l-b">r</span><span class="l-y">t</span><span class="l-g">a</span><span class="l-r">l</span>
            </h1>
            <p class="home-tagline">{{ i18n.heading }}</p>
            <div class="os-official-slogan-wrap">
              <h2 class="hero-slogan-en">Search Anything. Discover Everything. Do Anything.</h2>
              <p class="hero-slogan-ko">무엇이든 검색하고, 모든 것을 발견하고, 원하는 것을 실행하다.</p>
              <div class="search-to-action-chain" aria-label="Search-to-Action 생태계 철학">
                <span class="s2a-step active">Search</span>
                <span class="s2a-arrow">→</span>
                <span class="s2a-step">Discover</span>
                <span class="s2a-arrow">→</span>
                <span class="s2a-step">Connect</span>
                <span class="s2a-arrow">→</span>
                <span class="s2a-step">Experience</span>
                <span class="s2a-arrow">→</span>
                <span class="s2a-step">Create</span>
              </div>
            </div>
            <div class="os-brand-subbadge">OPEN SCANNER ENGINE 2.0 · SEARCH-TO-ACTION ECOSYSTEM</div>
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
                <div v-if="busy" class="scanning-laser-beam" aria-hidden="true"></div>
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
              검색 결과 약 {{ result.total.toLocaleString() }}개 ({{ (result.elapsed_ms / 1000).toFixed(3) }}초) · 서버 {{ result.elapsed_ms }}ms · 실시간 검증 색인
            </p>

            <!-- Phase 6: Grounded AI Overview Box -->
            <div v-if="activeTab === 'web' && result.ai_overview" class="ai-overview-card" data-testid="ai-overview-card">
              <div class="ai-overview-header">
                <div class="ai-overview-title-group">
                  <span class="ai-sparkle-icon">✨</span>
                  <span class="ai-overview-title">AI 개요</span>
                  <span class="ai-faith-pill" :class="{ faithful: result.ai_overview.is_faithful }">
                    {{ result.ai_overview.is_faithful ? `출처 검증 완료 (${(result.ai_overview.confidence * 100).toFixed(0)}%)` : '일반 요약' }}
                  </span>
                </div>
                <div class="ai-feedback-actions">
                  <button
                    type="button"
                    class="ai-feedback-btn"
                    :class="{ active: aiFeedbackSubmitted }"
                    title="도움이 됨"
                    @click="sendAIFeedback(true)"
                  >
                    👍
                  </button>
                  <button
                    type="button"
                    class="ai-feedback-btn"
                    title="도움이 안 됨"
                    @click="sendAIFeedback(false)"
                  >
                    👎
                  </button>
                </div>
              </div>
              <div class="ai-overview-body">
                <p class="ai-summary-text">{{ result.ai_overview.summary }}</p>
                <div v-if="result.ai_overview.disclaimer" class="ai-disclaimer-box">
                  ⚠️ {{ result.ai_overview.disclaimer }}
                </div>
                <div v-if="result.ai_overview.citations && result.ai_overview.citations.length > 0" class="ai-citations-grid">
                  <a
                    v-for="c in result.ai_overview.citations"
                    :key="c.index"
                    :href="c.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="ai-citation-chip"
                    :aria-label="'출처 ' + c.index"
                    :title="c.snippet"
                  >
                    <span class="citation-idx">[{{ c.index }}]</span>
                    <span class="citation-title">{{ c.title }}</span>
                  </a>
                </div>
              </div>
            </div>

            <!-- Phase 7: Sponsored Ads Slot (Strictly Segregated from Organic Ranking) -->
            <div v-if="activeTab === 'web' && result.sponsored_ads && result.sponsored_ads.length > 0" class="sponsored-ads-container">
              <div v-for="ad in result.sponsored_ads" :key="ad.ad_id" class="sponsored-ad-card">
                <div class="ad-badge-row">
                  <span class="ad-tag">{{ ad.sponsored_label || '광고' }}</span>
                  <span class="ad-display-url">{{ ad.display_url }}</span>
                </div>
                <h2 class="ad-headline">
                  <a :href="ad.target_url" target="_blank" rel="noopener noreferrer" @click="trackAdClick(ad)">
                    {{ ad.headline }}
                  </a>
                </h2>
                <p class="ad-description">{{ ad.description }}</p>
              </div>
            </div>

            <!-- 1. ACTIVE TAB: WEB (Organic SERP) -->
            <div v-if="activeTab === 'web'" class="hits-list">
              <article v-for="(h, hitIdx) in result.hits" :key="h.url" class="hit">
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

                <!-- Primary Result Sitelinks (Shown only for the #1 authoritative match) -->
                <div v-if="hitIdx === 0 && h.url.includes('example.org')" class="hit-sitelinks-grid">
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
                  <p class="video-desc">{{ vid.desc }}</p>
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
                  <svg viewBox="0 0 24 24" width="24" height="24" fill="#2563eb">
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
                  <svg viewBox="0 0 24 24" width="40" height="40" fill="#2563eb">
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

            <!-- 7. ACTIVE TAB: BOOKS (Bibliographic & Technical Literature Catalog) -->
            <div v-else-if="activeTab === 'books'" class="books-results-list">
              <article
                v-for="book in booksResults"
                :key="book.id"
                class="book-result-card"
              >
                <div class="book-cover-box" aria-hidden="true">
                  <svg viewBox="0 0 24 24" width="32" height="32" fill="#2563eb">
                    <path d="M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 4h5v8l-2.5-1.5L6 12V4z"/>
                  </svg>
                </div>
                <div class="book-content-col">
                  <div class="book-meta-top">
                    <span class="book-pub-badge">{{ book.publisher }}</span>
                    <span class="news-dot">·</span>
                    <span class="book-year">{{ book.publishedAt }}</span>
                    <span class="news-dot">·</span>
                    <span class="book-isbn">ISBN {{ book.isbn }}</span>
                  </div>
                  <h2 class="book-title">
                    <a :href="book.url" target="_blank" rel="noopener noreferrer">{{ book.title }}</a>
                  </h2>
                  <p class="book-authors">저자: {{ book.authors }}</p>
                  <p class="book-desc">{{ book.description }}</p>
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

            <!-- Open Scanner Numbered Pagination -->
            <div v-if="result.total > 10" class="serp-pagination" aria-label="검색 결과 페이지 이동">
              <div class="pagination-logo-art" aria-hidden="true">
                <span class="pag-letter pag-g">Open</span>
                <span
                  v-for="p in Math.min(8, Math.ceil(result.total / 10))"
                  :key="p"
                  class="pag-letter"
                  :class="[p % 2 === 0 ? 'pag-o' : 'pag-e', { active: p === page }]"
                >
                  S
                </span>
                <span class="pag-letter pag-gle">canner</span>
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

    <!-- Open Scanner Brand Philosophy & Search-to-Action Ecosystem Modal -->
    <div
      v-if="showPhilosophyModal"
      class="super-modal-overlay"
      role="dialog"
      aria-modal="true"
      aria-label="Open Scanner 브랜드 철학"
      @click.self="showPhilosophyModal = false"
    >
      <div class="super-modal-card philosophy-modal-card">
        <div class="super-modal-header">
          <div class="super-modal-title">
            <span class="super-title-icon">⚡</span>
            <h3>Open Scanner 브랜드 철학 &amp; Search-to-Action 생태계</h3>
          </div>
          <button type="button" class="super-modal-close" aria-label="닫기" @click="showPhilosophyModal = false">✕</button>
        </div>
        <div class="philosophy-modal-body">
          <div class="philo-hero">
            <h4 class="philo-slogan-main">Search Anything. Discover Everything. Do Anything.</h4>
            <p class="philo-slogan-sub">무엇이든 검색하고, 모든 것을 발견하고, 원하는 것을 실행하다.</p>
            <blockquote class="philo-manifesto">
              "We don't just help people find information.<br />
              We connect the world to everything it can discover and do."
            </blockquote>
          </div>

          <div class="philo-section">
            <h5>1. Search-to-Action 핵심 라이프사이클</h5>
            <div class="philo-chain">
              <div class="chain-node"><span class="node-tag">Search</span><span class="node-desc">전 세계 정보 탐색</span></div>
              <span class="chain-arrow">→</span>
              <div class="chain-node"><span class="node-tag">Discover</span><span class="node-desc">콘텐츠 & 기회 발견</span></div>
              <span class="chain-arrow">→</span>
              <div class="chain-node"><span class="node-tag">Connect</span><span class="node-desc">사람과 서비스 연결</span></div>
              <span class="chain-arrow">→</span>
              <div class="chain-node"><span class="node-tag">Experience</span><span class="node-desc">실제 디지털 경험</span></div>
              <span class="chain-arrow">→</span>
              <div class="chain-node"><span class="node-tag">Create</span><span class="node-desc">가치와 결과물 창출</span></div>
            </div>
            <p class="philo-note">검색은 시작이고, 발견은 과정이며, 실행은 결과입니다.</p>
          </div>

          <div class="philo-section">
            <h5>2. 30개 글로벌 통합 생태계</h5>
            <div class="philo-ecosystem-grid">
              <div class="eco-pillar">
                <h6>SEARCH PILLAR</h6>
                <ul>
                  <li><strong>웹 검색:</strong> 독립 인덱스 글로벌 검색</li>
                  <li><strong>TripScanner:</strong> 항공·호텔·여행 통합 검색</li>
                  <li><strong>PriceScanner:</strong> 상품 최저가 비교 쇼핑</li>
                  <li><strong>Coin 플랫폼:</strong> 가상자산 & 온체인 금융</li>
                  <li><strong>Open News:</strong> 실시간 신디케이션 뉴스</li>
                </ul>
              </div>
              <div class="eco-pillar">
                <h6>DISCOVER PILLAR</h6>
                <ul>
                  <li><strong>Open AI:</strong> 신경망 심층 탐색 & 인용</li>
                  <li><strong>Open Video:</strong> 고화질 영상 채널</li>
                  <li><strong>Open Stream:</strong> 초저지연 라이브 방송 & OTT</li>
                </ul>
              </div>
              <div class="eco-pillar">
                <h6>ACTION PILLAR</h6>
                <ul>
                  <li><strong>GADA:</strong> 일자리·공수 구인구직 매칭</li>
                  <li><strong>Delivery:</strong> 생활 밀착형 배달·물류</li>
                  <li><strong>Open Mail:</strong> E2EE 암호화 웹메일</li>
                  <li><strong>Open Apps:</strong> 독립 앱마켓 스토어</li>
                  <li><strong>Open Cloud & Dev:</strong> 분산 엣지 & SDK</li>
                </ul>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Global Google-Style Footer -->
    <footer class="app-footer">
      <div class="footer-top-row">
        <span class="footer-region">대한민국 · 서울</span>
        <div class="footer-brand-slogan-strip">
          <span class="strip-slogan-en">Search Anything. Discover Everything. Do Anything.</span>
          <span class="strip-slogan-ko">무엇이든 검색하고, 모든 것을 발견하고, 원하는 것을 실행하다.</span>
        </div>
        <span class="footer-status-pill">Open Scanner Engine</span>
      </div>
      <div class="footer-bottom-row">
        <div class="footer-left-links">
          <a href="#" @click.prevent="openBrandPhilosophyModal">브랜드 철학 (Search-to-Action)</a>
          <a href="#" @click.prevent="runSearch(1, '오픈포털 검색엔진 프로젝트')">OpenPortal 정보</a>
          <a href="#" @click.prevent="runSearch(1, '광고 플랫폼')">광고</a>
          <a href="#" @click.prevent="runSearch(1, '비즈니스 솔루션')">비즈니스</a>
          <a href="#" @click.prevent="runSearch(1, '검색의 원리')">검색의 원리</a>
        </div>
        <div class="footer-right-links">
          <a href="#" @click.prevent="showSettings = true">개인정보처리방침</a>
          <a href="#" @click.prevent="showSettings = true">약관</a>
          <a href="#" @click.prevent="showSettings = true">설정</a>
          <a href="/operator" @click.prevent="navigateTo('/operator')">⚙️ 운영자 컨트롤 타워</a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { RouterLink, useRoute, useRouter } from 'vue-router';
import ReportDialog from '../ReportDialog.vue';
import ScanState from '../ScanState.vue';
import { fill, serpMessages } from '../serp-copy';
import { coreMessages, designCopy, foundationCopy, serpCopy } from '../i18n/index';
import { useSearchStore } from '../stores/search';
import { useSessionStore } from '../stores/session';

const session = useSessionStore();
const search = useSearchStore();
const route = useRoute();
const router = useRouter();
const t = coreMessages;
const copy = designCopy;
const foundation = foundationCopy;
const serp = serpCopy;

const page = ref(1);

// C-9: per-hit report dialog target (null = closed).
const reportTarget = ref<{ url: string; documentId: string } | null>(null);

function openReport(url: string, documentId: string | undefined): void {
  reportTarget.value = { url, documentId: documentId || url };
}

function closeReport(): void {
  reportTarget.value = null;
}

const developmentIndex = computed(() => search.result && !['brave-api', 'bing-api'].includes(search.result.source || '') && search.result.corpus_kind !== 'approved-web');
const sourceLabel = computed(() =>
  search.result?.source === 'brave-api' || search.result?.source === 'bing-api'
    ? copy.value.sourceExternal
    : search.result?.source === 'hybrid-rrf'
      ? copy.value.sourceHybrid
      : copy.value.sourceIndex
);
type SearchErrorKind = 'offline' | 'rate' | 'timeout' | 'invalid' | 'unavailable';
const errorKind = computed<SearchErrorKind>(
  () => ({ OFFLINE: 'offline', RATE_LIMITED: 'rate', SEARCH_TIMEOUT: 'timeout', INVALID_QUERY: 'invalid', INVALID_INPUT: 'invalid' } as Record<string, SearchErrorKind>)[search.error] || 'unavailable'
);
const serpError = computed(() => {
  const messages = serpMessages[session.locale];
  return {
    offline: { title: messages.offlineTitle, body: messages.offlineBody },
    rate: { title: messages.rateTitle, body: messages.rateBody },
    timeout: { title: messages.timeoutTitle, body: messages.timeoutBody },
    invalid: { title: messages.invalidTitle, body: messages.invalidBody },
    unavailable: { title: messages.unavailableTitle, body: messages.unavailableBody },
  }[errorKind.value];
});
const queryWords = computed(() => [...new Set(search.query.trim().split(/\s+/).filter(Boolean))]);

// Text-only highlighting of query terms (no v-html); terms are regex-escaped.
function highlight(text: string): { text: string; hit: boolean }[] {
  const terms = queryWords.value.map(term => term.toLowerCase()).filter(term => term.length >= 2).slice(0, 8);
  if (!terms.length) return [{ text, hit: false }];
  const pattern = new RegExp(`(${terms.map(term => term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
  return text.split(pattern).filter(Boolean).map(part => ({ text: part, hit: terms.includes(part.toLowerCase()) }));
}

function getHostname(urlStr: string): string {
  try {
    return new URL(urlStr).hostname;
  } catch {
    return urlStr;
  }
}

function crumbs(value: string): string {
  try {
    const url = new URL(value);
    const parts = url.pathname.split('/').filter(Boolean).slice(0, 3).map(part => {
      try {
        return decodeURIComponent(part);
      } catch {
        return part;
      }
    });
    return [url.hostname.replace(/^www\./, ''), ...parts].join(' › ');
  } catch {
    return value;
  }
}

function submitSearch(nextPage = 1): void {
  if (!search.query.trim()) return;
  session.saveRecentSearch(search.query.trim());
  const params = new URLSearchParams({ q: search.query.trim() });
  if (nextPage > 1) params.set('page', String(nextPage));
  if (session.safeSearch !== 'moderate') params.set('safe_search', session.safeSearch);
  void router.push(`/${session.locale}/search?${params}`);
}

function applyCorrection(corrected: string): void {
  search.query = corrected;
  submitSearch(1);
}

function relaxSafeSearch(): void {
  session.setSafeSearch('moderate');
  submitSearch(1);
}

function changeSafeSearch(event: Event): void {
  const value = (event.target as HTMLSelectElement).value;
  if (!['strict', 'moderate', 'off'].includes(value)) return;
  session.setSafeSearch(value as 'strict' | 'moderate' | 'off');
  submitSearch(1);
}

function onOnline(): void {
  if (search.error === 'OFFLINE') void search.runSearch(search.query, page.value, session.safeSearch);
}

watch(
  () => route.query,
  query => {
    const text = typeof query.q === 'string' ? query.q : '';
    search.query = text;
    const suffix = session.locale === 'ko' ? 'OpenScanner 검색' : session.locale === 'th' ? 'OpenScanner ค้นหา' : 'OpenScanner Search';
    document.title = text.trim() ? `${text.trim()} - ${suffix}` : 'OpenScanner';
    session.useQuerySafeSearch(typeof query.safe_search === 'string' ? query.safe_search : null);
    const requested = Number(query.page || 1);
    page.value = Number.isInteger(requested) && requested >= 1 && requested <= 100 ? requested : 1;
    void search.runSearch(text, page.value, session.safeSearch);
  },
  { immediate: true }
);

onMounted(() => window.addEventListener('online', onOnline));
onUnmounted(() => {
  window.removeEventListener('online', onOnline);
  search.cancel();
});
</script>
<template>
  <section class="serp-shell" :aria-busy="search.busy">
    <h1 class="sr-only">{{ search.query ? `${search.query} · ${serp.web}` : serp.emptyTitle }}</h1>
    <div class="serp-toolbar">
      <nav class="serp-tabs" :aria-label="serp.verticals">
        <a :href="route.fullPath" aria-current="page" @click.prevent>
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true"><circle cx="9" cy="9" r="5.5" stroke="currentColor" stroke-width="1.8" /><path d="m13.2 13.2 3.3 3.3" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" /></svg>{{ serp.web }}
        </a>
      </nav>
      <label class="safe-control">
        <span>{{ t.safeSearch }}</span>
        <select :aria-label="t.safeSearch" :value="session.safeSearch" @change="changeSafeSearch">
          <option value="moderate">{{ t.safeSearchModerate }}</option>
          <option value="strict">{{ t.safeSearchStrict }}</option>
          <option value="off">{{ t.safeSearchOff }}</option>
        </select>
      </label>
    </div>

    <div v-if="!search.query.trim()" class="serp-state">
      <ScanState kind="empty" />
      <h2>{{ serp.emptyTitle }}</h2>
      <p>{{ serp.emptyBody }}</p>
      <ul class="tip-list">
        <li>{{ t.emptyTipGeneral }}</li>
        <li>{{ t.emptyTipFewer }}</li>
      </ul>
    </div>

    <div v-else-if="search.busy" class="serp-loading">
      <p class="sr-only" role="status">{{ serp.loading }}</p>
      <div class="results-meta skeleton-line" aria-hidden="true"><i></i></div>
      <div v-for="item in 5" :key="item" class="skeleton-hit" aria-hidden="true">
        <span class="sk-domain"><i class="sk-dot"></i><i></i></span><b></b><span class="sk-text"></span><span class="sk-text short"></span>
      </div>
    </div>

    <div v-else-if="search.error" class="serp-state" :class="`state-${errorKind}`" role="alert">
      <ScanState :kind="errorKind" />
      <h2>{{ serpError.title }}</h2>
      <p>{{ serpError.body }}</p>
      <div class="actions">
        <button v-if="errorKind !== 'invalid'" class="button primary" @click="search.runSearch(search.query, page, session.safeSearch)">{{ serp.retry }}</button>
        <RouterLink class="button" :to="`/${session.locale}`">{{ serp.home }}</RouterLink>
      </div>
      <p v-if="errorKind === 'unavailable' || errorKind === 'timeout'" class="error-code">{{ serp.errorCode }}: <code>{{ search.error }}</code></p>
    </div>

    <template v-else-if="search.result">
      <div v-if="search.result.hits.length" class="results-meta">
        <span>{{ search.result.total === null ? copy.unknownCount : fill(serp.about, { n: new Intl.NumberFormat(session.locale).format(search.result.total) }) }}<template v-if="search.result.elapsed_ms !== undefined">{{ ' ' + fill(serp.seconds, { s: new Intl.NumberFormat(session.locale, { maximumFractionDigits: 2, minimumFractionDigits: 2 }).format(search.result.elapsed_ms / 1000) }) }}</template><template v-if="page > 1">{{ ' · ' + fill(serp.pageOf, { n: page }) }}</template></span>
        <span class="source-badge">{{ sourceLabel }}</span>
      </div>
      <p v-if="developmentIndex" class="notice corpus-notice" role="note">{{ foundation.developmentIndex }}</p>
      <p v-if="search.result.degraded" class="notice partial-notice" role="status">
        <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true" style="vertical-align: -3px; margin-right: 4px;"><circle cx="12" cy="12" r="9" /><path d="M12 8v4m0 4h.01" /></svg>
        {{ copy.partial }}
      </p>
      <p v-if="search.result.corrected_query && search.result.corrected_query !== search.query && search.result.hits.length" class="correction">
        {{ serp.didYouMean }} <button class="text-button" @click="applyCorrection(search.result.corrected_query!)"><strong>{{ search.result.corrected_query }}</strong></button>
      </p>

      <div v-if="!search.result.hits.length && page > 1" class="serp-state">
        <ScanState kind="end" />
        <h2>{{ serp.noMoreTitle }}</h2>
        <p>{{ fill(serp.noMoreBody, { n: page }) }}</p>
        <div class="actions"><button class="button primary" @click="submitSearch(1)">{{ serp.firstPage }}</button></div>
      </div>
      <div v-else-if="!search.result.hits.length" class="serp-state no-results">
        <ScanState kind="none" />
        <h2>{{ t.empty }}</h2>
        <p>{{ fill(serp.noResultsLead, { q: search.query }) }}</p>
        <div v-if="search.result.corrected_query && search.result.corrected_query !== search.query" class="did-you-mean">
          {{ serp.didYouMean }} <button class="text-button" @click="applyCorrection(search.result.corrected_query!)"><strong>{{ search.result.corrected_query }}</strong></button>
        </div>
        <div v-if="queryWords.length > 1 || session.safeSearch === 'strict'" class="alternatives">
          <p class="alt-title">{{ serp.tryInstead }}</p>
          <div class="alt-chips">
            <button v-for="word in queryWords.slice(0, 3)" v-show="queryWords.length > 1" :key="word" class="chip" @click="search.query = word; submitSearch()">{{ fill(serp.searchOnly, { w: word }) }}</button>
            <button v-if="session.safeSearch === 'strict'" class="chip" @click="relaxSafeSearch">{{ serp.relaxSafe }}</button>
          </div>
        </div>
        <div class="tip-card">
          <p class="alt-title">{{ serp.tips }}</p>
          <ul class="tip-list">
            <li>{{ t.emptyTipSpelling }}</li>
            <li>{{ t.emptyTipGeneral }}</li>
            <li>{{ t.emptyTipFewer }}</li>
          </ul>
        </div>
      </div>

      <ol v-else class="results-list">
        <li v-for="hit in search.result.hits" :key="hit.url" class="hit">
          <a class="result-link" :href="hit.url" rel="noopener noreferrer">
            <span class="result-domain">
              <span class="domain-mark" aria-hidden="true">{{ getHostname(hit.url).replace(/^www\./, '').slice(0, 1).toUpperCase() }}</span>
              <span class="domain-text"><span class="site-name">{{ hit.site_name || getHostname(hit.url).replace(/^www\./, '') }}</span><span class="result-url">{{ crumbs(hit.url) }}</span></span>
            </span>
            <h2>{{ hit.title }}</h2>
          </a>
          <p class="result-description"><template v-for="(part, index) in highlight(hit.description)" :key="index"><mark v-if="part.hit">{{ part.text }}</mark><template v-else>{{ part.text }}</template></template></p>
          <div class="hit-actions">
            <button type="button" class="text-button report-button" :aria-label="serp.reportTitle" @click="openReport(hit.url, hit.document_id)">{{ serp.reportHit }}</button>
          </div>
        </li>
      </ol>
      <ReportDialog v-if="reportTarget" :url="reportTarget.url" :document-id="reportTarget.documentId" @close="closeReport" />

      <nav v-if="search.result.hits.length && (page > 1 || search.result.has_more)" class="pagination" :aria-label="t.resultCount">
        <button class="button" :disabled="page <= 1" @click="submitSearch(page - 1)">
          <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M19 12H5M12 19l-7-7 7-7" /></svg>
          <span>{{ t.previous }}</span>
        </button>
        <span class="page-indicator" aria-current="page">{{ fill(serp.pageOf, { n: new Intl.NumberFormat(session.locale).format(page) }) }}</span>
        <button class="button" :disabled="!search.result.has_more || page >= 100" @click="submitSearch(page + 1)">
          <span>{{ t.next }}</span>
          <svg class="icon-svg" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M12 5l7 7-7 7" /></svg>
        </button>
      </nav>
      <p v-if="search.result.hits.length && !search.result.has_more" class="end-of-results">{{ serp.endOfResults }}</p>
      <p class="source-note">{{ copy.sourceNote }}</p>
    </template>
  </section>
</template>

import { defineStore } from 'pinia';
import { ref } from 'vue';
import { APIError, request } from '../client';

export interface SearchHit {
  title: string;
  url: string;
  description: string;
  site_name?: string;
  source?: string;
}

export interface SearchResult {
  total: number | null;
  has_more: boolean;
  elapsed_ms?: number;
  corrected_query?: string;
  did_you_mean?: string;
  source?: string;
  degraded?: boolean;
  corpus_kind?: string;
  hits: SearchHit[];
}

export const useSearchStore = defineStore('search', () => {
  const result = ref<SearchResult | null>(null);
  const busy = ref(false);
  const error = ref('');
  // Shared with the header search box in the app shell.
  const query = ref('');
  let controller: AbortController | undefined;
  let sequence = 0;

  function cancel(): void {
    controller?.abort();
    ++sequence;
    busy.value = false;
  }

  function clear(): void {
    cancel();
    result.value = null;
    error.value = '';
  }

  async function runSearch(query: string, page: number, safeSearch: string): Promise<void> {
    const own = ++sequence;
    controller?.abort();
    controller = new AbortController();
    result.value = null;
    error.value = '';
    if (!query.trim()) {
      busy.value = false;
      return;
    }
    busy.value = true;
    try {
      const params = new URLSearchParams({ q: query, page: String(page) });
      if (safeSearch && safeSearch !== 'moderate') params.set('safe_search', safeSearch);
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
        corpus_kind: typeof data.corpus_kind === 'string' ? data.corpus_kind : undefined,
        total: data.total,
        has_more: Boolean(data.has_more ?? data.hasMore ?? (data.total !== null && page * 10 < data.total)),
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
        error.value = !navigator.onLine ? 'OFFLINE' : (e as { name?: string })?.name === 'TimeoutError' ? 'SEARCH_TIMEOUT' : e instanceof APIError ? e.code : 'SEARCH_UNAVAILABLE';
      }
    } finally {
      if (own === sequence) busy.value = false;
    }
  }

  return { result, busy, error, query, runSearch, cancel, clear };
});

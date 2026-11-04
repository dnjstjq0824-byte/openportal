import { test } from 'node:test';
import assert from 'node:assert/strict';
import { normalizeURL, rrfMerge, braveSearch, bingSearch, hybridSearch } from '../gateway/search.mjs';

test('normalizeURL strips trailing slashes, downcases hostname, and handles ports', () => {
  assert.equal(normalizeURL('https://EXAMPLE.ORG/path/'), 'https://example.org/path');
  assert.equal(normalizeURL('https://example.org/'), 'https://example.org');
  assert.equal(normalizeURL('http://Test.Com:8080/deep/link/'), 'http://test.com:8080/deep/link');
});
test('different query strings remain different searchable documents', () => {
  assert.notEqual(normalizeURL('https://example.org/item?id=1'), normalizeURL('https://example.org/item?id=2'));
  const merged = rrfMerge([{url:'https://example.org/item?id=1',title:'First',description:'A'}],[{url:'https://example.org/item?id=2',title:'Second',description:'B'}]);
  assert.equal(merged.length,2);
});

test('rrfMerge applies reciprocal rank formula, combines ranks, and dedupes URLs', () => {
  const localHits = [
    { url: 'https://openportal.org/docs', title: 'Local Docs', description: 'Local description', source: 'local' },
    { url: 'https://rfc-editor.org/rfc9309', title: 'RFC 9309 Local', description: 'Robots spec' },
  ];
  const externalHits = [
    { url: 'https://openportal.org/docs/', title: 'External Docs', description: 'Ignored external duplicate' },
    { url: 'https://w3.org/standards', title: 'W3C Standards', description: 'Web standards' },
  ];

  const merged = rrfMerge(localHits, externalHits, 60);

  // 1. Both systems have https://openportal.org/docs (rank 1 local, rank 1 external)
  // RRF score = 1/(60+1) + 1/(60+1) = 2/61 ≈ 0.032786885
  const top = merged[0];
  assert.equal(top.url, 'https://openportal.org/docs');
  assert.equal(top.title, 'Local Docs'); // Local metadata prioritized
  const expectedTopScore = 1 / 61 + 1 / 61;
  assert.ok(Math.abs(top.rrf_score - expectedTopScore) < 0.0001);

  // 2. Total items should be 3 (deduplicated from 4)
  assert.equal(merged.length, 3);
});

test('braveSearch correctly handles API response and query parameter limits', async () => {
  const mockConfig = { braveApproved: true, braveKey: 'test-key-brave' };
  const mockFetcher = async (url, opts) => {
    assert.equal(opts.headers['X-Subscription-Token'], 'test-key-brave');
    return new Response(JSON.stringify({
      query: { original: 'test query', more_results_available: true },
      web: {
        results: [
          { title: 'Brave Result 1', url: 'https://example.com/1', description: 'Desc 1' },
          { title: 'Brave Result 2', url: 'https://example.com/2', description: 'Desc 2' },
        ],
      },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } });
  };

  const res = await braveSearch(mockConfig, 'test query', 1, { fetcher: mockFetcher, safeSearch: 'strict' });
  assert.equal(res.query, 'test query');
  assert.equal(res.hits.length, 2);
  assert.equal(res.hasMore, true);
  assert.equal(res.source, 'brave-api');
});

test('retired Bing adapter never makes a network request', async () => {
  let called = false;
  await assert.rejects(bingSearch({bingApproved:true,bingKey:'test-key'},'query',1,{fetcher:()=>{called=true;}}), error=>error.code === 'SEARCH_PROVIDER_RETIRED' || error.message === 'SEARCH_PROVIDER_RETIRED');
  assert.equal(called,false);
});
test('hybridSearch merges local backend and external API using RRF', async () => {
  const mockConfig = {
    searchURL: 'http://127.0.0.1:8080',
    braveApproved: true,
    braveKey: 'brave-hybrid-key',
  };
  const mockFetcher = async (reqUrl, opts) => {
    const urlStr = String(reqUrl);
    if (urlStr.includes('/api/v1/search')) {
      return new Response(JSON.stringify({
        query: 'hybrid test',
        total: 1,
        hits: [{ url: 'https://shared.org/item', title: 'Local Shared', description: 'Local Desc' }],
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    if (urlStr.includes('api.search.brave.com')) {
      return new Response(JSON.stringify({
        query: { original: 'hybrid test' },
        web: {
          results: [
            { url: 'https://shared.org/item/', title: 'External Shared', description: 'Ext Desc' },
            { url: 'https://external-only.org', title: 'External Only', description: 'Ext Only Desc' },
          ],
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    }
    throw new Error('Unknown URL: ' + urlStr);
  };

  const res = await hybridSearch(mockConfig, 'hybrid test', 1, { fetcher: mockFetcher });
  assert.equal(res.query, 'hybrid test');
  assert.equal(res.source, 'hybrid-rrf');
  assert.equal(res.hits.length, 2);
  assert.equal(res.hits[0].url, 'https://shared.org/item');
  assert.equal(res.hits[0].title, 'Local Shared'); // Local metadata preserved
});
test('hybrid pagination retains external next pages and does not invent a combined total', async () => {
  const result=await hybridSearch({searchURL:'http://127.0.0.1:8080',braveApproved:true,braveKey:'protocol-fixture'},'test',1,{fetcher:async url=>String(url).includes('/api/v1/search')?Response.json({query:'test',total:1,has_more:false,hits:[]}):Response.json({query:{original:'test',more_results_available:true},web:{results:[]}})});
  assert.equal(result.total,null);
  assert.equal(result.hasMore,true);
});


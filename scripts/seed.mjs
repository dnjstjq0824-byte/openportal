import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, '..');
const base = (process.env.OPENSEARCH_URL || 'http://127.0.0.1:9200').replace(/\/$/, '');
const index = process.env.OPENSEARCH_INDEX || 'documents_v1';

export async function seed({ baseURL = base, indexName = index } = {}) {
  const mapping = JSON.parse(fs.readFileSync(path.join(root, 'infra/opensearch/documents_v1.json'), 'utf-8'));
  
  // Create index if not exists
  const putRes = await fetch(`${baseURL}/${indexName}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(mapping),
  });
  if (!putRes.ok) {
    const body = await putRes.json();
    if (putRes.status !== 400 || body.error?.type !== 'resource_already_exists_exception') {
      throw new Error(`Failed to create index: ${putRes.status}`);
    }
  }

  const fixtures = [
    { document_id: 'fixture-001', url: 'https://example.org/intro', title: '오픈포털 검색엔진 프로젝트', description: '독립 검색엔진 아키텍처 개발을 위한 예시 문서', body: '검색 엔진 크롤러 역색인 랭킹 API Go Vue Flutter 한국어', language: 'ko', indexable: true },
    { document_id: 'fixture-002', url: 'https://example.org/search', title: '검색엔진 인덱싱과 랭킹', description: '검색 색인과 품질 평가에 대한 테스트 문서', body: '웹 검색 랭킹 텍스트 색인 검색 결과', language: 'ko', indexable: true },
    { document_id: 'fixture-003', url: 'https://example.org/docs', title: 'OpenPortal developer documentation', description: 'Example English document in the demo dataset', body: 'OpenPortal full text search engine API integration', language: 'en', indexable: true }
  ];

  let bulkBody = '';
  for (let i = 0; i < fixtures.length; i++) {
    bulkBody += JSON.stringify({ index: { _index: indexName, _id: `fixture-${String(i + 1).padStart(3, '0')}` } }) + '\n';
    bulkBody += JSON.stringify(fixtures[i]) + '\n';
  }

  const bulkRes = await fetch(`${baseURL}/_bulk?refresh=true`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-ndjson' },
    body: bulkBody,
  });

  if (!bulkRes.ok) {
    throw new Error(`Bulk indexing failed: ${bulkRes.status} ${await bulkRes.text()}`);
  }

  const resJson = await bulkRes.json();
  if (resJson.errors !== false || !Array.isArray(resJson.items) || resJson.items.length !== fixtures.length ||
      resJson.items.some(item => !item.index || !Number.isInteger(item.index.status) || item.index.status < 200 || item.index.status >= 300 || item.index.error)) {
    throw new Error(`Bulk indexing had errors: ${JSON.stringify(resJson)}`);
  }

  console.log(`Seeded ${fixtures.length} synthetic fixtures into ${indexName}.`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) seed().catch(err => {
  console.error(err);
  process.exit(1);
});


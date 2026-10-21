#!/usr/bin/env python3
"""Fixture-only OpenSearch initialization. Does not crawl websites or fabricate a real web corpus."""
import json, os, pathlib, urllib.request, urllib.error
base=os.environ.get('OPENSEARCH_URL','http://localhost:9200').rstrip('/')
root=pathlib.Path(__file__).resolve().parents[1]
index=os.environ.get('OPENSEARCH_INDEX','documents_v1')

def request(method, path, payload, content_type='application/json'):
    data=payload if isinstance(payload,bytes) else json.dumps(payload,ensure_ascii=False).encode()
    req=urllib.request.Request(base+path,data=data,method=method,headers={'Content-Type':content_type})
    try:
        with urllib.request.urlopen(req,timeout=8) as resp:return resp.read().decode()
    except urllib.error.HTTPError as e:raise RuntimeError(f'{method} {path} -> {e.code}: {e.read()[:400]!r}') from e

mapping=json.loads((root/'infra/opensearch/documents_v1.json').read_text(encoding='utf-8'))
try:request('PUT','/'+index,mapping)
except RuntimeError as e:
    if 'resource_already_exists_exception' not in str(e):raise
fixtures=[
 {'document_id':'fixture-001','url':'https://example.org/intro','title':'오픈포털 검색엔진 프로젝트','description':'독립 검색엔진 아키텍처 개발을 위한 예시 문서','body':'검색 엔진 크롤러 역색인 랭킹 API Go Vue Flutter 한국어','language':'ko','indexable':True},
 {'document_id':'fixture-002','url':'https://example.org/search','title':'검색엔진 인덱싱과 랭킹','description':'검색 색인과 품질 평가에 대한 테스트 문서','body':'웹 검색 랭킹 텍스트 색인 검색 결과','language':'ko','indexable':True},
 {'document_id':'fixture-003','url':'https://example.org/docs','title':'OpenPortal developer documentation','description':'Example English document in the demo dataset','body':'OpenPortal full text search engine API integration','language':'en','indexable':True}
]
lines=[]
for i,doc in enumerate(fixtures,1):
    lines.append(json.dumps({'index':{'_index':index,'_id':f'fixture-{i:03d}'}},ensure_ascii=False))
    lines.append(json.dumps(doc,ensure_ascii=False))
result=json.loads(request('POST','/_bulk?refresh=true', ('\n'.join(lines)+'\n').encode(),'application/x-ndjson'))
if result.get('errors') or len(result.get('items',[])) != len(fixtures):
    raise RuntimeError('Bulk fixture indexing failed: '+json.dumps(result)[:1000])
print(f'Seeded {len(fixtures)} synthetic fixtures into {index}. Not real crawled web pages.')

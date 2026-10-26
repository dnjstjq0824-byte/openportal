#!/usr/bin/env python3
import os
import sys
import shutil
import subprocess
import hashlib
import json
import zipfile
from pathlib import Path

root_dir = Path(__file__).resolve().parents[1]
work_dir = root_dir / "work"
stage_dir = work_dir / "handoff_stage"
repo_stage_dir = stage_dir / "repo"
history_stage_dir = stage_dir / "history"
zip_out_path = root_dir / "OpenPortal_Search_Gemini_Handoff_Phase3.zip"

print("1. Cleaning previous staging...")
if stage_dir.exists():
    shutil.rmtree(stage_dir, ignore_errors=True)
repo_stage_dir.mkdir(parents=True, exist_ok=True)
history_stage_dir.mkdir(parents=True, exist_ok=True)

print("2. Creating git bundle...")
bundle_path = history_stage_dir / "OpenPortal_Search.git.bundle"
subprocess.run(["git", "bundle", "create", str(bundle_path), "--all"], cwd=root_dir, check=True)

print("3. Archiving clean repo working tree...")
tar_path = stage_dir / "repo.tar"
subprocess.run(["git", "archive", "--format=tar", "HEAD", "-o", str(tar_path)], cwd=root_dir, check=True)
subprocess.run(["tar", "-xf", str(tar_path), "-C", str(repo_stage_dir)], cwd=root_dir, check=True)
tar_path.unlink()

print("4. Writing updated START_HERE_KO.md...")
start_here_ko = """# OpenPortal Search Phase 3 인계 및 빠른 시작 가이드 (2026-10-26)

## 1. 개요 및 인계 기준
본 아카이브는 Phase 0/1/2 NO-GO(Docker Compose, CGO race, Flutter SDK 호스트 설치, 원격 CI, 독립 감사자 승인, 임의 인터넷 도메인 크롤링 비활성) 상태 하에서,
선행 게이트 예외 정책에 따라 독립적으로 진행 가능한 Phase 3(Tasks 031~040)의 색인·추출·형태소분석·중복제거·Blue-Green 재색인·Tombstone 삭제 전파 파이프라인을 구현하고 검증한 산출물입니다.

- 기준 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`
- 선행 커밋: `3fb3af11f61c3120ff8ad8039ef53cba2c9c947c` (Phase 2)
- Phase 3 구현 커밋: `678edd0c734dc23d306c007898862eed6f0fdf14` (HEAD)
- 구현 내용 (Tasks 031~040):
  - HTML 보일러플레이트 제거, 메인 본문/헤딩/앵커 텍스트 추출, 한국어/영어 고속 언어감지 (031)
  - 64-bit SimHash, Hamming distance (<=4 bit), rel="canonical" 우선권, 대표문서 및 alias 클러스터링 (032)
  - OpenSearch 공식 `analysis-nori` 플러그인 탑재, mixed 복합어 분해, 품사 stoptags 필터링, 영어 분석기 분리, documents_v2 매핑 (033)
  - 본문 SHA-256 및 SimHash 기반 변경감지, Transactional Outbox (upsert/delete FIFO 큐), 버전 기반 소비자 멱등성 (034)
  - OpenSearch _reindex API, documents_search/documents_write alias 전환, Blue-Green 무중단 스왑 및 롤백 (035)
  - BM25 다중 필드 랭킹 가중치(title 3.5, headings 2.0, anchor 1.8, desc 1.5, body 1.0) 및 골든 평가 데이터셋 (036)
  - 고급 쿼리 파서(구문 검색, 제외 검색, site: 필터, lang: 필터), Base64 page_token 커서 페이지네이션 (037)
  - Deletion Tombstone 처리(`is_tombstone=true`, `indexable=false`), 검색 캐시 무효화, Outbox delete 전파 (038)
  - 크롤 지연, 성공률, 색인 지연(index lag), 신선도, Outbox 대기 수, Tombstone 전파 수 텔레메트리 메트릭 (039)
  - 크롤러 수집 -> 추출 -> SimHash 중복제거 -> Outbox -> OpenSearch 색인 -> 검색 쿼리 -> Tombstone 삭제 전파 E2E 통합 (040)
  - 데이터베이스 마이그레이션 005_phase3_indexing_and_tombstones.sql 적용 및 스키마 검증 (034, 038)

## 2. 저장소 복원 방법
`history/OpenPortal_Search.git.bundle`에 전체 Git 이력이 포함되어 있습니다.
```bash
# 새 디렉터리에 복원할 경우:
git clone history/OpenPortal_Search.git.bundle searchengine
cd searchengine
git checkout main
```

## 3. 로컬 개발 환경 및 의존성 설치
- Node.js v24.x
- Go v1.27.3
- Python 3.12.x
- OpenSearch 2.19.0 (with analysis-nori plugin, 127.0.0.1:9200)
- Google Chrome (CHROME_PATH 지정)

```bash
npm ci
npm ci --prefix apps/web
```

## 4. 검증 명령어 실행 순서
```bash
# 1. 일회용 PostgreSQL 17.10 DB 마이그레이션(001~005) 및 RLS/BOLA/코퍼스/색인 스키마 검증
node scripts/test-database.mjs

# 2. Protobuf 및 계약 호환성 검사
node scripts/test-proto.mjs
node scripts/check-contracts.mjs

# 3. OpenSearch 합성 fixture 유닛 검사
python -m unittest discover -s scripts -p test_seed.py

# 4. Vue 웹 타입검사 및 빌드
npm run typecheck --prefix apps/web
npm run build --prefix apps/web

# 5. Go 백엔드 전체 패키지 유닛/통합 테스트 (indexer, crawler, search, identity, cmd/api 전체 PASS)
cd backend
go test -v ./...
go test "-coverprofile=coverage.out" "-coverpkg=./..." ./...
cd ..

# 6. 보안 도구 검사
govulncheck ./... (in backend)
gitleaks dir . --config .gitleaks.toml --redact

# 7. OpenSearch 2.19.0 실행 (포트 9200) 및 합성 fixture 시딩
node scripts/start-opensearch.mjs
python scripts/seed.py

# 8. Playwright 브라우저 E2E 전체 회귀 테스트 (5건 전체 PASS)
$env:CHROME_PATH = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
npx playwright test
```

## 5. 다음 Gemini 세션 이어서 작업할 프롬프트
```text
OpenPortal Search Phase 3(Tasks 031~040)가 구현 및 검증 완료된 인계 산출물을 기준으로 Phase 4(Tasks 041~050)를 이어서 개발하라.
Phase 0/1/2/3의 미해결 게이트(Docker Compose, CGO race, Flutter SDK 부재, 독립 감사자 승인, 임의 인터넷 도메인 크롤링 비활성)는 NO-GO 상태로 유지하고,
Phase 4(Tasks 041~050: 쿼리 의도 분류, 인기어/조작 방지, 사이트 품질/스팸 신호, 세이프서치 유해물 분류, 한국어 NDCG/MRR 정량 지표 계산, RRF 하이브리드 실험 플랫폼)의 구현과 로컬 검증을 순차적으로 수행하라.
```
"""
(stage_dir / "START_HERE_KO.md").write_text(start_here_ko, encoding="utf-8")

print("5. Calculating SHA-256 for all files in staging...")
manifest = {}
for root, dirs, files in os.walk(stage_dir):
    for f in sorted(files):
        p = Path(root) / f
        rel_path = p.relative_to(stage_dir).as_posix()
        h = hashlib.sha256(p.read_bytes()).hexdigest()
        manifest[rel_path] = h

manifest_json = json.dumps(manifest, indent=2, ensure_ascii=False)
(stage_dir / "HANDOFF_MANIFEST_SHA256.json").write_text(manifest_json, encoding="utf-8")
print(f"Manifest created with {len(manifest)} files.")

# Re-hash HANDOFF_MANIFEST_SHA256.json itself so we have its hash too
manifest_hash = hashlib.sha256((stage_dir / "HANDOFF_MANIFEST_SHA256.json").read_bytes()).hexdigest()
manifest["HANDOFF_MANIFEST_SHA256.json"] = manifest_hash

print("6. Creating ZIP archive...")
if zip_out_path.exists():
    zip_out_path.unlink()

with zipfile.ZipFile(zip_out_path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as zipf:
    for root, dirs, files in os.walk(stage_dir):
        for f in sorted(files):
            p = Path(root) / f
            arcname = p.relative_to(stage_dir).as_posix()
            zipf.write(p, arcname)

zip_bytes = zip_out_path.read_bytes()
zip_sha256 = hashlib.sha256(zip_bytes).hexdigest()
zip_size_mb = len(zip_bytes) / 1024 / 1024

(root_dir / "OpenPortal_Search_Gemini_Handoff_Phase3.zip.sha256").write_text(
    f"{zip_sha256}  OpenPortal_Search_Gemini_Handoff_Phase3.zip\n", encoding="utf-8"
)

# Also copy ZIP to Artifact Directory so user can download directly
artifact_dir = Path(r"C:\Users\dnjst\.gemini\antigravity\brain\47bdaef4-1b8c-409e-937d-6249a32dbf77")
if artifact_dir.exists():
    shutil.copy2(zip_out_path, artifact_dir / "OpenPortal_Search_Gemini_Handoff_Phase3.zip")
    shutil.copy2(root_dir / "OpenPortal_Search_Gemini_Handoff_Phase3.zip.sha256", artifact_dir / "OpenPortal_Search_Gemini_Handoff_Phase3.zip.sha256")
    print("Copied zip and sha256 to artifact directory.")

print(f"ZIP created successfully: {zip_out_path}")
print(f"Size: {zip_size_mb:.2f} MB")
print(f"SHA-256: {zip_sha256}")


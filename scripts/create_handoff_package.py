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
zip_out_path = root_dir / "OpenPortal_Search_Gemini_Handoff_Phase2.zip"

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
start_here_ko = """# OpenPortal Search Phase 2 인계 및 빠른 시작 가이드 (2026-10-26)

## 1. 개요 및 인계 기준
본 아카이브는 Phase 0/1 NO-GO(Docker Compose, CGO race, Flutter SDK 호스트 설치, 원격 CI, 독립 감사자 승인 부재) 상태 하에서,
선행 게이트 예외 정책에 따라 독립적으로 진행 가능한 Phase 2(Tasks 021~030)의 크롤러 핵심 엔진 및 정책 제어를 구현하고 검증한 산출물입니다.

- 기준 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`
- 선행 커밋: `44e6ea26f3066600c3b28b6d02773cbb0343a42d` (Phase 1)
- Phase 2 구현 커밋: `a7df15e2a54236b63593c0de4091769f35eb6049` (HEAD)
- 구현 내용 (Tasks 021~030):
  - RFC 3986 URL Canonicalization, Punycode IDN 변환, 추적 파라미터 스트립, SHA-256 해시 (022)
  - RFC 9309 robots.txt 파서, 와일드카드/끝점 정규식 컴파일, Longest-match tie-break, Crawl-delay, TTL 캐시 (024)
  - SSRF 사설망/루프백/링크로컬/클라우드 메타데이터 차단, DNS Rebinding 방어 IP Pinning Dialer, 리다이렉트 HTTPS 다운그레이드 방어 (025)
  - 호스트별 crawl-delay 및 동시성 1개 제한, 429/503 지수 백오프, 조건부 GET If-None-Match/If-Modified-Since (026)
  - Content-Type 화이트리스트, 2MiB 응답 크기 제한, 압축폭탄(Decompression bomb) 가드 (027)
  - ISO 28500 WARC/1.0 아카이빙 레코드 직렬화 및 페이로드 SHA-256 다이제스트 (028)
  - HTML noindex/nofollow/nosnippet/noarchive 파싱, rel="canonical" 추출, 하이퍼링크 추출, XML sitemap/sitemapindex 파싱 (029)
  - 호스트 파티셔닝 우선순위 Frontier 큐 원자적 임대/확인/재시도 (021, 023)
  - 10~100개 검증 도메인 한정 허용 Bounded Crawler Engine, 임의 크롤링 차단(ErrArbitraryCrawlDenied), 트래픽/컴플라이언스 리포트 계측 (030)
  - 데이터베이스 마이그레이션 004_phase2_crawler_policy_and_corpus.sql 적용 및 RLS url_submissions 격리 (021, 030)

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
- OpenSearch 2.19.0 (127.0.0.1:9200)
- Google Chrome (CHROME_PATH 지정)

```bash
npm ci
npm ci --prefix apps/web
```

## 4. 검증 명령어 실행 순서
```bash
# 1. 일회용 PostgreSQL 17.10 DB 마이그레이션(001~004) 및 RLS/BOLA/코퍼스 시딩 검증
node scripts/test-database.mjs

# 2. Protobuf 및 계약 호환성 검사
node scripts/test-proto.mjs
node scripts/check-contracts.mjs

# 3. OpenSearch 합성 fixture 유닛 검사
python -m unittest discover -s scripts -p test_seed.py

# 4. Vue 웹 타입검사 및 빌드
npm run typecheck --prefix apps/web
npm run build --prefix apps/web

# 5. Go 백엔드 전체 패키지 유닛/통합 테스트 (crawler 18개 테스트 포함 전체 PASS)
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
# (Windows의 경우 CHROME_PATH 설정)
$env:CHROME_PATH = "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
npx playwright test
```

## 5. 다음 Gemini 세션 이어서 작업할 프롬프트
```text
OpenPortal Search Phase 2(Tasks 021~030)가 구현 및 검증 완료된 인계 산출물을 기준으로 Phase 3(Tasks 031~040)를 이어서 개발하라.
Phase 0/1/2의 미해결 게이트(Docker Compose, CGO race, Flutter SDK 부재, 독립 감사자 승인, 임의 인터넷 도메인 크롤링 비활성)는 NO-GO 상태로 유지하고,
Phase 3(Tasks 031~040: HTML 보일러플레이트 제거, 본문 추출, SimHash 유사중복 제거, 한국어 Nori/영어 분석기 설정, Blue-Green 재색인, 문서 삭제 Tombstone 전파 파이프라인)의 구현과 로컬 검증을 순차적으로 수행하라.
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

(root_dir / "OpenPortal_Search_Gemini_Handoff_Phase2.zip.sha256").write_text(
    f"{zip_sha256}  OpenPortal_Search_Gemini_Handoff_Phase2.zip\n", encoding="utf-8"
)

# Also copy ZIP to Artifact Directory so user can download directly
artifact_dir = Path(r"C:\Users\dnjst\.gemini\antigravity\brain\47bdaef4-1b8c-409e-937d-6249a32dbf77")
if artifact_dir.exists():
    shutil.copy2(zip_out_path, artifact_dir / "OpenPortal_Search_Gemini_Handoff_Phase2.zip")
    shutil.copy2(root_dir / "OpenPortal_Search_Gemini_Handoff_Phase2.zip.sha256", artifact_dir / "OpenPortal_Search_Gemini_Handoff_Phase2.zip.sha256")
    print("Copied zip and sha256 to artifact directory.")

print(f"ZIP created successfully: {zip_out_path}")
print(f"Size: {zip_size_mb:.2f} MB")
print(f"SHA-256: {zip_sha256}")


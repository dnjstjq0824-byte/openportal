# Phase 0~1 버전 및 지원 정책 — 004

본 문서는 모노레포의 언어·프레임워크·데이터베이스·검증 도구의 고정 버전, 업스트림 지원 수명주기(EOL), 보안 패치 정책 및 현재 실행 환경의 한계를 기록한다.

## 1. 런타임·데이터베이스·도구 버전 매트릭스

| 구성 요소 | 고정 버전 | 지정 위치 | 업스트림 지원 정책 및 EOL 기준 | 현재 호스트 검증 상태 |
|---|---|---|---|---|
| **Go** | `1.27.3` | `backend/go.mod`, `backend/Dockerfile`, `.github/workflows/phase0.yml` | 최신 2개 메이저 릴리스 지원(~2027-02 예상). `1.27.0`/`1.27.2`의 표준 라이브러리 취약점(9건+1건) 해결을 위해 `1.27.3` 필수 | **검증완료**: `GOTOOLCHAIN=auto`로 `go1.27.3` 실행, `go test`, `go vet`, `govulncheck`(취약점 0건) 통과 |
| **Node.js** | `24.19.0` | `.node-version`, `infra/compose.yaml`, `.github/workflows/phase0.yml` | Active/Maintenance LTS (EOL: 2028-04-30) | **검증완료**: 로컬 `v24.19.0` 실행 |
| **Python** | `3.12.14` | `.python-version`, `infra/compose.yaml`, `.github/workflows/phase0.yml` | PEP 602 5년 지원 주기 (보안 패치 EOL: 2028-10-31) | **검증완료**: 로컬 `Python 3.12.14`로 `seed.py` 및 `test_seed.py` 실행 |
| **PostgreSQL** | `17.10` | `package.json` (`embedded-postgres`), `infra/compose.yaml` | PGDG 5년 지원 주기 (EOL: 2029-11-08) | **검증완료**: `PostgreSQL 17.10` 네이티브 클러스터에서 86테이블/8스키마/75FK 및 마이그레이션 회귀 통과 |
| **OpenSearch** | `2.19.0` | `infra/compose.yaml`, 로컬 런타임 | 2.x 유지보수 브랜치 (차기 메이저 유지보수 전환 시까지 최소 1년 이상 보장). Phase 3(`033`)에서 `analysis-nori` 플러그인 검증 필수 | **검증완료**: 로컬 `OpenSearch 2.19.0` 클러스터 기동 및 E2E 검색/색인 검증 완료 |
| **Vue** | `3.5.44` | `apps/web/package.json`, `apps/web/package-lock.json` | Vue 3 현행 안정 브랜치 | **검증완료**: 타입체크, 프로덕션 빌드, Playwright E2E 통과 |
| **Nuxt** | `3.17.5` (또는 Vue 3 호환 고정 버전) | Phase 1(`011`) 도입 대상 (`apps/web/package.json`) | Nuxt 3.x 유지보수 라인 (npm 레지스트리 `3.17.5` 가용성 확인됨) | Phase 0에서는 Vite(`6.4.4`) 사용; Phase 1(`011`)에서 Nuxt로 이행 및 검증 |
| **TypeScript** | `5.9.3` | `apps/web/package.json`, `apps/web/package-lock.json` | 5.9 안정 릴리스 (`vue-tsc 3.3.1` 연동) | **검증완료**: `vue-tsc --noEmit` 통과 |
| **Buf / Protobuf** | `buf 1.74.0` / `protoc-gen-go 1.36.6` / `protobuf v1.36.12` | `.github/workflows/phase0.yml`, `backend/go.mod` | proto3 표준 계약 및 코드 생성 고정 | **검증완료**: `buf format`, `buf lint`, `buf generate`, `buf breaking` 통과 |
| **Playwright** | `1.64.1` | `package.json`, `package-lock.json` | E2E 브라우저 자동화 러너 (로컬 Chrome `CHROME_PATH` 또는 Chromium 연동) | **검증완료**: 실제 OpenSearch + Go + 웹 E2E 통과 |
| **govulncheck / gitleaks** | `v1.1.4` / `v8.24.2` | `.github/workflows/phase0.yml` | Go 취약점 DB 대조 및 Git 시크릿 스캐너 | **검증완료**: 로컬 바이너리 실행, 취약점 0건 / 시크릿 유출 0건 확인 |
| **Flutter / Dart** | `Flutter 3.32.x` 후보 / `Dart >=3.4.0 <4.0.0` | `apps/mobile/pubspec.yaml` | 모바일 크로스플랫폼 SDK | **차단(G009)**: 현재 호스트에 Flutter/Dart SDK 미설치. 소스(`apps/mobile/`) 작성 및 정적 검증만 가능하며 네이티브 빌드는 SDK 환경 필요 |

## 2. 업데이트·보안 패치·공급망 관리 규칙

1. **Lockfile 및 결정적 설치 강제**:
   - 모든 Node 의존성은 `package-lock.json`과 `apps/web/package-lock.json`으로 고정하며 `npm ci`로만 복원한다.
   - Go 모듈은 `backend/go.mod` 및 `backend/go.sum`으로 해시를 고정한다.
2. **보안 취약점 게이트**:
   - 버전 변경 또는 신규 패키지 도입 시 반드시 `npm audit --audit-level=moderate`, `govulncheck ./...`, `node scripts/check-licenses.mjs`, CycloneDX SBOM 생성을 재실행한다.
   - 표준 라이브러리나 런타임에서 보안 취약점이 보고된 경우(예: `go1.27.0` → `go1.27.3` 상향, G013) ADR에 사유를 기록하고 즉시 최소 요구 버전을 올린다.
3. **현재 환경 제약 및 잔여 차단 사항**:
   - **Docker Engine 미설치(G005)**: `infra/compose.yaml`의 이미지(`postgres:17.10`, `opensearchproject/opensearch:2.19.0`, `node:24.19.0-bookworm-slim`, `python:3.12.14-slim`, `golang:1.27.3-bookworm`) 버전은 로컬 네이티브 버전과 일치시켜 고정했으나, 컨테이너 런타임 및 이미지 멀티아키텍처 SHA256 digest 고정·서명 검증은 Docker 지원 CI/호스트에서 완료해야 한다.
   - **CGO 컴파일러 미설치(G012)**: Windows 호스트에 `gcc`/`clang`이 없어 `go test -race ./...`는 Linux CI(`ubuntu-latest`)에서 실행하도록 워크플로에 고정했다.
   - **Flutter SDK 미설치(G009)**: `apps/mobile`의 실제 Android/iOS 빌드 및 에뮬레이터 테스트는 Flutter SDK가 설치된 호스트/CI에서 수행해야 한다.

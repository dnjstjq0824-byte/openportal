# 버전 및 지원 정책 — 004 (현행화: 2026-11-24)

본 문서는 모노레포의 언어·프레임워크·데이터베이스·검증 도구의 고정 버전, 업스트림 지원 수명주기(EOL),
보안 패치 정책 및 현재 실행 환경의 실측 상태를 기록한다. 상태 기준은 [GAP_REGISTER.md](GAP_REGISTER.md)다.

## 1. 런타임·데이터베이스·도구 버전 매트릭스

| 구성 요소 | 고정 버전 | 지정 위치 | 업스트림 지원 정책 및 EOL 기준 | 현재 호스트 검증 상태 |
|---|---|---|---|---|
| **Go** | `1.27.3` | `backend/go.mod`, `backend/Dockerfile`, `.github/workflows/phase0.yml` | 최신 2개 메이저 릴리스 지원(~2027-02 예상). 표준 라이브러리 취약점 해결을 위해 `1.27.3` 필수 | **검증완료**: `GOTOOLCHAIN=auto` 해석(호스트 `go1.27.0` + 자동 툴체인), `go test`, `go vet`, `govulncheck`(0건) 통과. `-race`는 Docker Linux에서 전 패키지 통과 |
| **Node.js** | `24.19.0` | `.node-version`, `infra/compose.yaml`, `docker-compose.prod.yml`, `.github/workflows/phase0.yml` | Active/Maintenance LTS (EOL: 2028-04-30) | **검증완료**: 로컬 `v24.19.0` 실행 |
| **Python** | `3.12.14` | `.python-version`, `infra/compose.yaml`, `.github/workflows/phase0.yml` | PEP 602 5년 지원 주기 (보안 패치 EOL: 2028-10-31) | **검증완료**: 핀 `3.12.14`로 `seed.py`·`test_seed.py` 실행. 호스트에 3.14 병존 보고 있으나 핀 우선 |
| **PostgreSQL** | `17.10` | `package.json` (`embedded-postgres` 17.10.0-beta.17), compose 파일 | PGDG 5년 지원 주기 (EOL: 2029-11-08) | **검증완료**: 네이티브·컨테이너 17.10에서 86테이블/8스키마/75FK + 마이그레이션 001~018 회귀 통과 |
| **OpenSearch** | `2.19.0` + `analysis-nori` | `infra/compose.yaml`, `docker-compose.prod.yml`, 로컬 런타임 | 2.x 유지보수 브랜치 (차기 메이저 전환 시까지 최소 1년 이상 보장) | **검증완료**: 로컬·Docker 2.19.0 + Nori 플러그인, `documents_v2` 토큰화·수명주기·E2E 통과 |
| **Vue** | `3.5.44` | `apps/web/package.json`, `apps/web/package-lock.json` | Vue 3 현행 안정 브랜치 | **검증완료**: 타입체크·프로덕션 빌드·Playwright E2E 통과 |
| **Vue Router / Pinia / vue-i18n** | `4.6.4` / `3.0.4` / `11.4.18` | `apps/web/package.json` | 각 현행 안정 릴리스 | **검증완료**: 상용화 B안 이주 후 e2e 재통과 |
| **Vite** | `6.4.5` | `apps/web/package.json` | Vite 6 안정 브랜치 | **검증완료**: 프로덕션 빌드 통과 |
| **Nuxt** | 미도입 (Vite 유지) | — | Nuxt 3.x 유지보수 라인 | **유지 결정(G016)**: `@nuxt/devtools` 하위 CVE로 제로 취약점 정책상 Vite 유지. 업스트림 패치 시 재검토 |
| **TypeScript** | `5.9.3` | `apps/web/package.json`, `apps/web/package-lock.json` | 5.9 안정 릴리스 (`vue-tsc 3.3.1` 연동) | **검증완료**: `vue-tsc --noEmit` 통과 |
| **Buf / Protobuf** | `buf 1.74.0` / `protobuf v1.36.12` | `.github/workflows/phase0.yml`, `backend/go.mod` | proto3 표준 계약 및 코드 생성 고정 | **검증완료**: `buf format`·`lint`·`generate`·`breaking` + negative control 통과 |
| **Playwright** | `1.64.1` | `package.json`, `package-lock.json` | E2E 브라우저 자동화 러너 (로컬 Chrome `CHROME_PATH` 또는 Chromium 연동) | **검증완료**: platform·mail·foundation·design 스펙 실행 |
| **govulncheck / gitleaks** | CI 고정 | `.github/workflows/phase0.yml` | Go 취약점 DB 대조 및 Git 시크릿 스캐너 | **검증완료**: 취약점 0건 / 시크릿 유출 0건 |
| **Flutter / Dart** | `3.47.8` / `3.13.7` (실측) | `apps/mobile/pubspec.yaml` | 모바일 크로스플랫폼 SDK | **검증완료**: `flutter test` 7/7 PASS (2026-11-10). 구 "SDK 부재 차단" 문구는 폐기, cross-platform E2E는 잔여 |
| **Docker Engine** | `29.9.2` (실측) | compose 파일 3종 | 컨테이너 런타임 | **검증완료**: daemon 기동 확인, prod 4종 이미지 빌드 + healthy + `/readyz` 200 실측. 구 "미설치 차단" 문구는 폐기 |

## 2. 업데이트·보안 패치·공급망 관리 규칙

1. **Lockfile 및 결정적 설치 강제**:
   - 모든 Node 의존성은 `package-lock.json`과 `apps/web/package-lock.json`으로 고정하며 `npm ci`로만 복원한다.
   - Go 모듈은 `backend/go.mod` 및 `backend/go.sum`으로 해시를 고정한다.
2. **보안 취약점 게이트**:
   - 버전 변경 또는 신규 패키지 도입 시 반드시 `npm audit --audit-level=moderate`, `govulncheck ./...`,
     라이선스 검사, CycloneDX SBOM 생성을 재실행한다.
   - 표준 라이브러리나 런타임에서 보안 취약점이 보고된 경우 ADR에 사유를 기록하고 즉시 최소 요구 버전을 올린다.
3. **현재 환경 실측 상태 (2026-11-10, GAP_REGISTER 실행 환경 정정 반영)**:
   - **Docker 가용**: `docker-compose.prod.yml`(PG17.10 + nori OS + Go + 게이트웨이) 4종 healthy 실측 완료.
     이미지 digest 고정·서명 검증은 원격 CI(F-5)에서 완료한다.
   - **CGO**: Windows 네이티브 `-race`는 gcc 부재로 불가 유지. **Docker Linux에서 `go test -race ./...`
     전 패키지 통과**로 대체 검증했다.
   - **외부 게이트 잔여**: 원격 CI·CodeQL·이미지 서명·배포 자격증명(F-5), 독립 감사(F-4).

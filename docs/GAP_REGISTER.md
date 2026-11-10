# GAP REGISTER — OpenScanner 감사 추적 단일 기준 문서 (Single Source of Truth)

- 최종 갱신: 2026-11-10 (Phase 1 완료, Phase 2 진입)
- 종합 판정: **NO-GO** (P1~P3 잔여 및 외부 BLOCKED 해소 전까지 공개 출시 금지)
- 출처: OpenScanner 감사 작업지시서 Phase 1~3 (코드 항목 A~D 명시 22건 + 기존 G-ledger 매핑 + F 외부 5건)
- 주의: 감사 원문에 언급된 34건 중 작업지시서에 번호가 명시되지 않은 항목(A-7, B-4, D-1~D-3, E-1~)은
  임의로 창작하지 않고 하단 "미열거(UNENUMERATED)" 섹션에 추적한다. 번호가 확인되는 대로 등록한다.

## 실행 환경 정정 (2026-11-10 실측, 구 문서의 차단 문구는 폐기)

| 항목 | 구 문서 주장 | 2026-11-10 실측 | 비고 |
|---|---|---|---|
| Docker | 미설치 차단 (G005) | Docker 29.9.2, daemon 기동 확인 | Compose 풀스택 검증 가능. G005 Phase 2에서 해소 |
| Flutter | SDK 부재 (G009/G017) | Flutter 3.47.8 / Dart 3.13.7 | `flutter test` 7/7 PASS (2026-11-10) |
| Go | 1.27.3 필요 (G013) | go1.27.0 + 자동 툴체인 해결 | `go test -count=1 ./...` 전체 PASS |
| CGO/gcc | race 차단 (G012) | gcc 없음, CGO_ENABLED=0 | Windows native `-race` 불가. **Docker Linux에서 `go test -count=1 -race ./...` 전체 PASS (2026-11-10)** |
| psql CLI | — | PATH에 없음 | embedded-postgres 기반 테스트/마이그레이션 정상 |
| OpenSearch | — | Docker 2.19.0 yellow 확인 | 로컬 .bat 기동은 좀비 프로세스로 실패, Docker로 대체 |
| Playwright | — | chromium/webkit 설치됨 | E2E 2/3 PASS, 1건은 HEAD既存 UI 드리프트(아래注) |

注: `platform.e2e` line 21 실패(heading 'OpenScanner' vs 렌더 'Open Scanner')는 HEAD부터 존재하는
테스트/UI 불일치로 Phase 1 변경과 무관함을 `git show HEAD:` 대조로 확인. E2E 경화 시 수정 예정.

## P0 — 즉시 차단/보안/허위 제거 (Phase 1, 전체 DONE)

| ID | 상태 | 내용 | 검증/증거 |
|---|---|---|---|
| A-1 | DONE 2026-11-10 | 운영 검색 응답에서 가짜 광고(`ad-cloud-001`) 주입 삭제. `SponsoredAds=[]` 명시 반환 | `TestSearchHonestSlotsByDefault` PASS + 실서버 curl 확인 |
| A-2 | DONE 2026-11-10 | AI Overview를 `AI_OVERVIEW_ENABLED`(기본 false) 뒤로 격리. OFF 시 미노출 | `TestSearchHonestSlotsByDefault` + `TestAIOverviewFlag` PASS |
| A-3 | DONE 2026-11-10 | `signoff.go` Phase 0~4 무조건 GO 제거. 증거 없으면 `ExternalAuditPassed:false` + `"NO-GO (BLOCKED)"`. 런치 핸들러 DR 시뮬레이션도 제거 | `TestMasterReleaseGate` Case 4 회귀 PASS |
| A-4 | DONE 2026-11-10 | `cmd/loadtest` 실측 도구로 재작성. 대상 미설정/미가동/전건 실패 시 FAIL + exit 1 | 빌드 PASS, 미가동 exit 1 확인 |
| A-5 | DONE 2026-11-10 | `cmd/dr-rehearsal` 실연결 프로브(PG LSN + OpenSearch health)로 재작성. 실패 시 exit 1 | 빌드 PASS, 미가동 exit 1 확인 |
| A-6 | DONE 2026-11-10 | `superplatform` 3종 가짜 핸들러 → 501(`NOT_IMPLEMENTED`). i18n `lucky` 3로케일 제거 | 501 회귀 + translations parity PASS |
| A-8 | DONE 2026-11-10 | 모듈 경로 → `github.com/dnjstjq0824-byte/openportal/backend`. `.pb.go` 재생성, baseline 갱신 | `go build/vet/test` + buf lint/breaking PASS |
| D-4 | DONE 2026-11-10 | `/api/v2/admin/*` 무조건 403(`ADMIN_DISABLED`). 핸들러 본문 삭제(기록은 git) | `admin-crud` + platform D-4 회귀 PASS |
| D-5 | DONE 2026-11-10 | OTP 덮어쓰기 제거(충돌 시 재생성), 검증 `email` 귀속 필수, 만료+시도(5회)+계정당 레이트(5/h) | 마이그레이션 015 + `otp-binding` 3건 PASS |
| D-AUDIT | DONE 2026-11-10 | Phase 4: `webapp.audit_log` append-only + 게이트웨이 11종 액션 기록(로그인 성공/실패·로그아웃·가입·검증·재전송·재설정 요청/완료·계정수정/삭제·신고·admin 거부). 비밀번호/토큰 meta 금지 | 마이그레이션 018 + `audit` 2건 PASS, 플랫폼 게이트 편입 |
| D-SESS | DONE(무변경) 2026-11-10 | Phase 4 세션 감사: 재설정 시 전체 세션 파기·30분 슬라이딩+7일 만료·회전·어드민 신선조회 확인, 기존 테스트 커버. 수정 불필요 | platform reset 무효화 회귀 PASS |
| G006 | 부분 해소 | auth/MFA/RLS/SSRF/삭제·정산 통제 (Phase 1~3에서 단계 해소) | 024~029, 038, 078~090 테스트 |
| G010 | 출시 차단 | 독립 감사·권리·결제 승인 증적 없음 → 파트 F로 이관 추적 | 외부 서명 필요 |
| G021 | 완화 유지 | moderation 운영 노출 금지 (production 404 유지) | `production_test.go` PASS |
| SEARCH-BASE-04 | 경계 보완 | 크롤러 기본 allowlist 제거, 운영자 allowlist 요구 | Go 검사 PASS, 외부 수집 미실행 |
| SIGNUP-20261104-02 | BLOCKED | 운영 메일 어댑터/DNS/평판 없음 → fail-closed 유지, F-2로 이관 | `MAIL_PROVISIONING_UNAVAILABLE` |

## P1 — 서비스 가동/코어 필수 (Phase 2 + 크롤러/아웃박스)

| ID | 상태 | 내용 | 검증/증거 |
|---|---|---|---|
| B-1 | DONE 2026-11-10 | `gateway/Dockerfile`(멀티스테이지, node 유저) 신규 + Go 측 기존 파일 검증(curl healthcheck 추가). 4종 이미지 빌드 성공 | `docker compose build` 4/4 Built |
| B-2 | DONE 2026-11-10 | 운영 라우트는 실백엔드(search/suggest/healthz/readyz/metrics)만 유지(메모리 핸들러 개방 불가 판정 문서화). `/readyz` OS+PG 실체크, 503+원인 분류 | readyz 그린/레드 3건 + 실스택 `{"status":"ready"}` |
| B-3 | DONE 2026-11-10 | `docker-compose.prod.yml`(PG17.10+nori OS+Go+게이트웨이) + 마이그레이션/시드 자동화. 4종 healthy, 503 해소 실측 | `/readyz`·`/api/v1/search`·`/api/v2/session`·`/api/v2/search` 200 curl 로그 |
| B-5 | DONE 2026-11-10 | Go `/metrics`(HELP/TYPE+count/sum) + JSON 라인 로깅(ts/request_id/path/status/latency). 게이트웨이 표준 exposition 정리 | 단위 테스트 + 실스택 `/metrics` 출력 |
| R-1 | DONE 2026-11-10 | 리팩토링: `main.go` 2000→58줄, 17개 파일(≤330줄) 분리. `server.mjs` 471→387 + config/http/routes-mail 분리 | Go 전체 + 플랫폼 37/37 PASS |
| C-1 | DONE 2026-11-10 | RescheduleStale 메모리/PG + freshness 스케줄러, Frontier DB 연동 | 통합 PASS (cutoff-now 윈도우 결함 수정, -race PASS) |
| C-2 | DONE 2026-11-10 | Sitemap 파서 파이프라인 연결, WARC 로컬/S3호환 실저장 | 단위 PASS |
| C-3 | DONE 2026-11-10 | Bulk/_bulk·Relay·PGOutbox·Tombstone DB연동·crawler relay 연결, 직접 dispatch 금지 | 통합 3건 PASS |
| G001 | DONE | 부분 실패 fail-closed (shard/hits 누락 검사) | `evidence/go-tests.txt` PASS |
| G003 | DONE | seed bulk errors 검사 | seed-unit 3건 PASS |
| G004 | DONE | DB 이력/체크섬 원자 마이그레이션 | database.txt PASS |
| G005 | Phase 2 해소 예정 | Docker Compose 풀스택 (Docker 실측 설치·기동됨) | B-3에서 해소 |
| G007 | DONE | 자체 fetch→parse→index 파이프라인 | 8/8, 9/9, 18/18 PASS |
| G009 | 부분 DONE | Flutter 위젯 테스트 7/7 PASS (2026-11-10). cross-platform E2E 잔여 | `flutter test` PASS |
| G016 | 차단/완화 | Nuxt CVE로 Vite 유지 | 업스트림 패치 대기 |
| G017 | DONE | `flutter test` 7/7 PASS (2026-11-10, SDK 설치 확인) | All tests passed |
| G018 | DONE | 재랭킹 `language` 누락 수정 | Go PASS, E2E 5/5 |
| G020 | 부분 수정 | Intent/RRF 연결, 영속 저장소 잔여 | 재시작/다중 인스턴스 테스트 |
| G022 | 부분 검증 | 7질의 골든셋 NDCG=1.0 (장난감 수준, C-7에서 교체) | 회귀 PASS, 독립 검토 외부 |
| G024 | DONE | 동시 초기화 mutex | `-count=20` PASS |
| SEARCH-BASE-01/02/03 | DONE/경계 | URL 병합·하이브리드 카운트 수정, 개발 색인 안내 | 단위+E2E PASS |
| SIGNUP-BASE-01 | DONE | 6단계 가입 검증 (운영 어댑터는 BLOCKED) | 통합 18개 PASS |
| SIGNUP-20261104-01/03/04 | DONE/검토필요 | 메일함 트랜잭션 가입 / 수집항목 법률검토 / 대량생성 방지 | F-1/F-2 연계 |
| MAIL-20261104-01/02/04 | DONE/차단 | 잠금·순서검사 수정 / 운영 SMTP·JMAP·ClamAV 차단 | 통합 PASS, F-2 연계 |

## P2 — 검색 품질·보안 고도화 (Phase 3 후반)

| ID | 상태 | 내용 | 검증/증거 |
|---|---|---|---|
| C-4 | DONE 2026-11-10 | 하드코딩 사전 폐기 → completion suggester(+prefix 폴백) + title/body phrase 교정 + 2-bul 레이아웃 구조(인덱스 검증). 매핑 suggest 필드·파이프라인·seed 연동 | stub 단위 7건 + 실OS live PASS (검섹엔진→검색엔진, serach→search, rjatordpswls 구조) |
| C-5 | DONE | SafeSearch 실필터링: 색인라벨+DSL must_not, SearchWithSafety, post-filter 제거 (2026-11-10) | safesearch_dsl 단위 PASS, TestSearchSafeSearchInQuery PASS, strict E2E 계약 픽스처 PASS |
| C-6 | DONE(어휘)/BLOCKED(벡터) 2026-11-10 | Redis 카운터 DONE. 하이브리드 RRF: 키워드+fuzzy/phrase 리콜 2경로 병렬+RRF 융합(동일-10건 재정렬 폐기, Safety 양경로, total=max). 벡터: k-NN seam+명시적 미설정 에러, 임베딩 모델 없어 BLOCKED | stub 융합 5건 + 실OS live PASS, `-race` 14/14 DATA RACE 0 |
| G002 | DONE | proto lint/RPC명 수정 (A-8 후 baseline 갱신, breaking PASS) | buf lint + breaking + negative control PASS (2026-11-10) |
| G008 | DONE | Nori 플러그인 + documents_v2 + Blue-Green 재색인 | 토큰화+수명주기 PASS |
| G011 | 차단 | 원격 CI/CodeQL 미실행 | GitHub CI 실행 필요 (F-5 연계) |
| G012 | DONE(대체경로) | Docker Linux에서 `-race` 전체 PASS (2026-11-10). Windows native는 gcc 부재로 불가 유지 | race 14 패키지 ok, DATA RACE 0 |
| G013 | DONE | Go 1.27.3 (취약점 해소) | govulncheck clean |
| G014 | DONE | REST↔Protobuf 브리지 + Sponsored 분리 | RoundTrip PASS |
| G019 | DONE | 재랭킹 실신호 연동 (발행일/권위/밀도) | golden_eval PASS |
| MAIL-20261104-03 | DONE | 미저장 변경 폐기 확인 | E2E PASS |

## P3 — 확장 제품 영역

| ID | 상태 | 내용 | 검증/증거 |
|---|---|---|---|
| C-7 | DONE 2026-11-10 | 14문서 코퍼스+12질의(ko8/en4, 오타 2건 포함) 골든셋, 실 Nori 인덱스 대상 SearchHybrid 평가. AP/MAP 메트릭 추가 | live GOLDEN: mean NDCG@10=0.9986 MAP@10=1.0000 (임계 0.90/0.90) |
| C-8 | DONE 2026-11-10 | 수직 백엔드 미연결 판정: 운영 `?vertical=` 403→501 VERTICAL_NOT_IMPLEMENTED, 프로토타입 응답에 served_from=prototype-seed 표기 | 단위 PASS, E2E spec 501 갱신 |
| C-9 | DONE 2026-11-10 | `POST/GET /api/v2/report` (인증·레이트제한·PG 영속 017 마이그레이션) + SearchView 신고 다이얼로그(ko/en/th) | node 3건 PASS, 플랫폼 게이트 편입, typecheck+build PASS |
| G015 | DONE | 경쟁 비교·권리 경계 문서 | 공식자료 인용, 법무서명 외부 |
| G023 | 관찰 | 8080 401 오탐 1회 (재현 없음) | 재발 시 포트 기록 |

## 파트 F — STATUS: BLOCKED (Legal/Business Owner Action Required, 코드 해결 불가)

| ID | 상태 | 내용 | 필요한 조치/소유자 |
|---|---|---|---|
| F-1 | BLOCKED | 개인정보 처리방침·수집항목·보관기간·약관 법률 검토 미완료 | 법무 검토 서명 (Legal Owner) |
| F-2 | BLOCKED | 운영 메일 도메인 소유·DNS(MX/SPF/DKIM/DMARC)·발신 평판·운영 JMAP/SMTP 어댑터 없음 | 도메인 결정+메일 infra (Business/Infra Owner) |
| F-3 | BLOCKED | 상용 라이선스·제휴 계약 없음 (지도/쇼핑/뉴스 신디케이션, LLM SLA·저작권 면책) | 계약 체결 (Business Owner) |
| F-4 | BLOCKED | 독립 보안 감사(SOC2/ISO)·외부 침투테스트·재무감사(SOC1) 인증 없음 | 외부 감사 (External Auditor) |
| F-5 | BLOCKED | 원격 CI/CodeQL 실행·이미지 서명·배포 자격증명·결제(PG) 승인 없음 | 승인+자격증명 (Business/Infra Owner) |

## 미열거 감사 번호 (UNENUMERATED — 원문 확인 후 등록, 창작 금지)

- A-7, B-4, D-1, D-2, D-3, E-1~E-n: 작업지시서에 세부 내용 없음. 감사 원문 확보 시 등록.
- E 영역(확장 제품: 실제 광고 엔진, 실제 LLM/RAG, 개발자 플랫폼/SDK, 슈퍼앱 생태계)은
  A-1/A-2/A-6의 Follow-up으로 P0 완료 후 E 번호 부여 예정.

## 아카이브 정책

- 루트 및 `docs/`의 `OPENSCANNER_*` / `OPEN_SCANNER_*` glossy 완료 보고서는
  허위 수치(`10,000 QPS PASSED`, `p95 8.4ms` 등) 및 구버전 차단 문구를 포함하므로
  `docs/archive/`로 이동하고 본 문서를 유일 기준으로 삼는다. 복원은 git history 참조.

---

## 부록: 기존 Ledger 원문 보존 (2026-10-24 ~ 2026-11-04, 증거 유지)

# P0–P3 결함·출시 차단 등록부 — 2026-10-24

## 2026-11-04 OpenScanner 검색 기반

| ID | 심각도 | 상태 | 문제·조치 | 검증 범위 |
|---|---|---|---|---|
| SEARCH-BASE-01 | P1 | 로컬 수정·검증 | 쿼리가 다른 URL을 같은 문서로 병합하던 오류를 수정했다. | 수정 전 실패 재현 후 검색 단위 테스트 통과. |
| SEARCH-BASE-02 | P1 | 로컬 수정·검증 | 하이브리드 검색의 잘못된 전체 건수·다음 페이지 여부, 외부 예산 누락을 수정했다. | 공급원 모의 전송 및 실제 PostgreSQL 전역 제한 검사 통과. |
| SEARCH-BASE-03 | P1 | 경계 보완 | 기존 작은 색인을 글로벌 웹 검색으로 오인하지 않도록 개발 색인 안내를 표시한다. | 실제 로컬 검색 6개 PC/모바일 E2E 통과. 실제 웹 커버리지·최신성·권리 확보는 미검증. |
| SEARCH-BASE-04 | P0 | 경계 보완 | 임의로 승인했다고 가정한 기본 크롤링 사이트 목록을 제거하고 운영자 allowlist를 요구한다. | 국제 도메인명·허용 목록 Go 검사 통과. 외부 수집은 실행하지 않음. |
| SIGNUP-BASE-01 | P1 | 로컬 수정·검증 | 가입 준비 상태를 첫 단계에서 확인하고 재시도·응답 검증을 추가했다. | 실제 PostgreSQL/loopback SMTP 및 메일함 모의 제공업체로 검증. 실제 운영 어댑터·DNS·외부 송수신은 차단 상태. |

세부 실행 명령·측정 한계·외부 의존성은 [기반 보고서](OPENSCANNER_FOUNDATION_2026_11_04.md)를 따른다. 전체 출시 판정은 NO_GO다.

## 2026-11-04 메일함 생성형 회원가입

| ID | 심각도 | 상태 | 내용 | 검증/잔여 |
|---|---|---|---|---|
| SIGNUP-20261104-01 | P1 | 로컬 구현·검증 | 외부 이메일 가입을 없애고 6단계 가입(이름, 생년월일·성별, 아이디, 비밀번호, 복구 정보, 확인)을 마치면 `아이디@MAIL_DOMAIN` 메일함과 계정을 한 트랜잭션으로 만든다. 메일함 생성 실패 시 계정을 롤백하고 생성된 메일함은 삭제한다. 마이그레이션 013. | 일회용 PostgreSQL 통합 테스트 18개 및 데모 게이트웨이 브라우저 가입 흐름 통과. 메일함 생성은 JMAP **시뮬레이터**만 연결되어 있다. |
| SIGNUP-20261104-02 | P0 | **BLOCKED** | 운영 메일 서버의 메일함 생성 어댑터, 메일 도메인 소유·DNS(MX/SPF/DKIM/DMARC), 발신 평판이 없다. 운영 설정에서는 `MAIL_PROVISIONING_UNAVAILABLE`로 가입을 거부한다(fail-closed). | 도메인 결정과 메일 서버 선정 필요. `omail.com`은 현재 마켓 네임서버(atom.com)를 가리키며 소유가 확인되지 않았다. |
| SIGNUP-20261104-03 | P1 | 정책 검토 필요 | 이름, 생년월일, 성별(직접 입력·호칭), 복구 이메일, 전화번호를 수집한다. 만 14세 미만은 법정대리인 동의 기능이 없어 가입을 거부한다. 전화번호는 SMS 인증 없이 미인증 상태로 저장된다. | 개인정보 처리방침 문구를 갱신했으나 법률 검토 전 초안이다. 수집 항목의 필요성·보관 기간 확정 필요. |
| SIGNUP-20261104-04 | P2 | 미구현 | 무료 메일 계정 대량 생성 방지는 IP당 가입 5회/시간 제한뿐이다. 자동 가입 방지(CAPTCHA 등), 발송량 제한, 부가통신사업 신고는 없다. | 공개 가입 전 필수. |

## 2026-11-04 메일 수정

| ID | 심각도 | 상태 | 문제와 조치 | 검증/잔여 |
|---|---|---|---|---|
| MAIL-20261104-01 | P1 | 로컬 수정·검증 | 불확실한 발송 예약이 있어도 임시메일을 새 ID로 교체할 수 있었다. 저장/발송 공통 잠금 및 예약 검사로 차단한다. | 수정 전 200/기대 409 실패 재현. 수정 후 통합 테스트 통과. 실제 제공업체 상태 대조는 별도 필요. |
| MAIL-20261104-02 | P1 | 로컬 수정·검증 | 오래된 읽기 응답이 현재 선택을 덮어쓸 수 있었다. 순서 검사·취소와 요청별 메시지 참조로 수정한다. | 실제 브라우저 지연 응답 주입 검증. JMAP 제공업체는 프로토콜 모의 서버. |
| MAIL-20261104-03 | P2 | 로컬 수정·검증 | 메일 내부 이동·취소·앱 링크·브라우저 뒤로 가기·새로고침/종료에서 미저장 변경 폐기 확인을 적용했다. | 폐기 거부 시 경로와 본문 유지, 저장·재열기 E2E 통과. 실기기 검수는 별도. |
| MAIL-20261104-04 | P1 | 외부 설정 차단 | 실제 SMTP/JMAP, 사용자 메일함, DNS, ClamAV 운영 연결 및 외부 메일 도착·반송 검수가 필요하다. | [실행 안내](MAIL_GATEWAY_RUNBOOK.md). 테스트 성공을 외부 송수신 성공으로 취급하지 않는다. |

세부 증거: [메일 신뢰성 보고서](MAIL_RELIABILITY_2026_11_04.md). 이번 수정은 SQL 변경이나 운영 배포를 포함하지 않는다.

구현 SHA `ba2c1a5badf11b5fe3a7fbe390227a3d4067db01` (이전 HEAD: `67e216a8fe7dd40cb74cd02ba94281584ceb70ed`).
담당은 별도 명시 없으면 기술 리드. P0 즉시 출시차단 보안/정산/권리, P1 핵심 기능, P2 중요 품질, P3 개선.
아래 미구현 통제는 현재 공격 성공이 확인되었다는 뜻이 아니라 출시 전 필수 게이트다.

| ID | 심각도 | 상태 | 재현/기대→실제 | 원인·수정 또는 완화 | 회귀/종료 조건 |
|---|---|---|---|---|---|
| G001 | P1 | 수정·검증 | `go test ./internal/search -run TestIncompleteSearchFailsClosed`; 부분 실패는 오류여야 하나 기존엔 빈 성공 | timed_out/failed shards/hits 누락/JSON trailing 검사, 2MiB 상한 | evidence/go-tests.txt PASS |
| G002 | P2 | 수정·검증 | `buf lint proto`; 초기 RPC 이름 6개 실패 | 미배포 ads/portal 이름 수정, go_package 교정·생성 | evidence/proto.txt; breaking negative control PASS |
| G003 | P1 | 수정·검증 | bulk HTTP200에 errors=true면 실패해야 하나 초기 seed는 성공 표시 | scripts/seed.py 오류/개수 검사 | 실제 정상 seed PASS; seed-unit.txt: item-error/누락 item 거부 포함 3건 PASS |
| G004 | P1 | 수정·검증 | 초기 DB 이력/체크섬 없음 | advisory lock·DDL+이력 원자 트랜잭션, PUBLIC revoke | database.txt: 재실행/변조/실패 rollback/권한거부 PASS |
| G005 | P1 | 차단 | `docker compose -f infra/compose.yaml --profile app up --build`; 실행돼야 하나 Docker 없음 | native PG17.10/OpenSearch2.19.0으로 별도 실행 증거 확보 | Docker 호스트에서 Compose full stack 재검증 |
| G006 | P0 | 출시 차단 (Phase 1/2/3 부분 해소) | 소스 검사: auth/MFA/테넌트 RLS/SSRF crawler/삭제·정산 통제 없음 | Phase 1에서 Passkey/TOTP MFA 인증, 단회성 계정 복구코드, RLS BOLA 격리. Phase 2에서 크롤러 RFC 9309 robots.txt, 엄격한 사설망/메타데이터 SSRF 차단, DNS Rebinding 방어 IP Pinning Dialer, 허가 도메인 게이트 구현. Phase 3에서 삭제/비공개/법적 제거 Tombstone(`is_tombstone=true`, `indexable=false`) 즉시 색인/캐시/Outbox 전파 구현 완료 | 024~029(크롤러 SSRF/robots/politeness PASS), 038(삭제전파 PASS), 078~090(정산/감사) 독립 테스트 |
| G007 | P1 | 수정·검증 | fixture만 있고 자체 fetch→parse→index가 없음 | Phase 2 자체 Bounded Crawler 및 Phase 3 자체 추출(Boilerplate 제거) -> SimHash 중복제거 -> Outbox -> OpenSearch Nori 색인 -> 검색 쿼리 파서 파이프라인 완성. 8/8 indexer, 9/9 search, 18/18 crawler 유닛/통합테스트 PASS | evidence/go-tests.txt PASS; Phase 4 랭킹/품질 평가 연계 |
| G008 | P2 | 수정·검증 | mapping content_analyzer=standard | Phase 3에서 OpenSearch 공식 `analysis-nori` 한국어 형태소 분석기 플러그인 탑재, mixed 복합어 분해, 품사(posfilter) 필터링, 영어 형태소(stemmer) 분석기 분리, documents_v2 매핑 및 Blue-Green 무중단 재색인/스왑/롤백 구현 완료 | `documents_v2` Nori 14개 형태소 토큰화 및 `TestReindexAndAliasLifecycle` PASS |
| G009 | P1 | 부분·차단 | Vue/Vite만 실행; Flutter SDK/플랫폼 호스트 없음 | 버전 후보와 Nuxt 이행 계획을 명시; Flutter UI 및 위젯 테스트 코드 작성 완료 | 011~020 cross-platform E2E; Flutter SDK 환경 구성 |
| G010 | P0 | 승인 차단 | 독립 감사·소스 권리·상표·실결제 승인 증적 없음 | 합성 데이터·로컬 전용, 공개/결제 금지 | 사업 소유자 및 독립 법무/보안/정산 검토 서명 |
| G011 | P2 | 차단 | 로컬 go vet/npm/gitleaks 성공과 원격 CodeQL은 별개 | CI 작성, 아직 remote 없음 | GitHub CI 실제 실행, action/image digest 고정, backend/image SBOM 확장 |
| G012 | P2 | 차단 | `go test -race ./...` → requires cgo | CGO/compiler 환경 미설정; 일반 tests 통과와 구분 | Linux CI race PASS |
| G013 | P1 | 수정·검증 | govulncheck: Go1.27.0 9건, 1.27.2 추가 인증서 1건 | Go1.27.3 최소 버전으로 변경 | govulncheck.txt: No vulnerabilities found; 과거 보고서 보존 |
| G014 | P2 | 수정·검증 | REST와 proto 필드·pagination 다름 | `backend/internal/search/proto_bridge.go` 구현: REST <-> Protobuf v1 양방향 변환 및 `Sponsored=false` 유기적 검색 분리 검증 완료 | TestProtoBridgeRoundTripAndOrganicIsolation PASS |
| G015 | P3 | 수정·검증 | 경쟁 기능 비교 틀만 작성 | `docs/PRODUCT_BOUNDARIES.md`에 Google/Naver/Bing 공식 기술자료 기반 비교표 및 IP/저작권/크롤링 권리 경계 분석 완료 | 공식 기술 자료 인용 완료; 독립 법무 서명은 외부 게이트 |
| G016 | P1 | 차단 / 완화 | Nuxt 3/4 도입 시 `@nuxt/devtools` -> `simple-git`, `node-forge` CVE로 인해 `npm audit` 규칙 위반 | 엄격한 제로 취약점 정책 유지를 위해 Vue 3 + Vite 기반 유지하며 Phase 1 포털 및 화면 구현 | 상위 Nuxt 패키지 취약점 패치 릴리스 시 업그레이드 |
| G017 | P1 | 차단 | Flutter SDK 부재로 `flutter test` 실행 불가 | `apps/mobile/lib/main.dart` 및 `test/widget_test.dart` 코드 작성 완료 | Flutter SDK 설치 후 위젯 및 플랫폼 통합 테스트 실행 |
| G018 | P1 | 수정·검증 | 2026-10-26 Phase 4 게이트웨이에 재랭킹을 연결한 뒤 `npm run test:e2e`: `lang=ko` 검색 결과의 `language`가 빈 값 → 1/5 실패(`search.spec.mjs:21`) | `cmd/api/main.go` 재랭킹이 `search.Hit`를 재구성하며 `Language` 누락 → 원본 hit 맵에서 복사. 회귀 단언을 `TestRankingAbuseAndModerationEndpoints`에 추가 | `go test -count=1 ./...` PASS, E2E 5/5 PASS (evidence/browser-tests.txt) |
| G019 | P2 | 수정·검증 (2026-10-30) | 재랭킹 핸들러가 `PublishedAt=now`, `DomainAuthority=0.5`, `TermFrequencyDensity=0.05` 상수 사용 | `internal/indexer/pipeline.go`에서 `domain_authority`, `published_at`, `term_frequency_density`(최대 단어 빈도 비율) 산출·색인 및 `internal/search/search.go`, `cmd/api/main.go`에서 OpenSearch `_source` 신호 추출·재랭킹 연동 완료. 골든셋 NDCG@10 (1.0000) 및 스팸 감점 회귀 검증 완료 | `internal/ranking/golden_eval_test.go` PASS |
| G020 | P1 | 부분·수정 (2026-10-30) | 의도분류·실험 플랫폼·품질 게이트가 API/DB에 미연결, 남용탐지·신고/이의 상태는 프로세스 메모리 | `cmd/api/main.go` 검색 핸들러에 `IntentClassifier` 및 `ReciprocalRankFusion`(BM25+제목 매칭 하이브리드 RRF) 연결 완료; `backend/internal/identity/repository.go` PostgreSQL RLS 영속 저장소 구현 완료 | 영속 저장소 연결 + 재시작/다중 인스턴스 테스트 |
| G021 | P0 | 부분·완화 (2026-10-30) | 신고/이의 `POST /api/v1/control/moderation/*`는 requireAuth로 감싸 익명 접근 차단, 검토 `/api/v1/control/moderation/review`는 `requireRole("admin")` 적용; productionRoutes에서 404 유지. 관리자 인가(016/081) 영속화 전까지 운영 노출 금지 유지 | 어뷰즈 방지(rate limit·인증) 및 관리자 인가 구축 전까지 운영 노출 금지 | 016/081 인증·인가 완료 후 재검증 |
| G022 | P1 | 부분·검증 (2026-10-30) | 045/050 실제 인적 판정 데이터·독립 relevance 검토자 없음; 047 시맨틱 후보 생성기 없음 | `testdata/ranking_golden_dataset.json`에 7개 질의(한국어/영어, 내비게이션/정보/트랜잭션) 골든셋 정의, `golden_eval_test.go`에서 Mean NDCG@10=1.0000, MRR@10=1.0000 및 키워드스터핑 감점 검증 완료. 상용 릴리스용 독립 relevance 서명은 외부 게이트 유지 | 한국어/영어 골든셋 회귀 PASS, 독립 검토 서명은 외부 게이트 |
| G023 | P3 | 관찰 | 2026-10-26 재시작 전 첫 E2E 실행에서 8080 요청이 401(5/5 실패) — 원인 미규명; 재시작 후 재현되지 않음 | 해당 시점 8080에 다른 프로세스가 있었을 가능성(미확인) | 재발 시 포트 점유 프로세스 기록 |

## 실행 환경 장애

- G024 / P1 / 수정·로컬 검증 (2026-10-28): Go API의 동시 첫 요청이 서로 다른 identity/abuse/safety 객체를 받는 결함. `go test ./cmd/api -run TestConcurrentServiceInitialization -count=20`으로 수정 전 실패 재현, 초기화 mutex 적용 후 PASS. 전체 Go 테스트 PASS. CGO race 검증은 G012 차단 유지. 상세: [진행 보고서](PHASE_PROGRESS_2026_10_28.md).
- G021 정정 (2026-10-28): 실행 진입점 productionRoutes는 moderation을 포함한 변경 API를 404로 차단한다. 내부 routes의 익명 프로토타입은 운영 인가 구현이 아니며 공개하면 안 된다.

- 2026-10-20 Go1.27.2 업데이트 명령은 자동 승인 검토 사용량 제한으로 미실행됐다. 2026-10-21 Go1.27.3으로 수정 완료.
- 2026-10-24 OpenSearch 2.19.0 실행 시 JVM 힙 옵션 `-Xms512m -Xmx512m` 누락 시 메모리 할당 지연 발생. Node 백그라운드 헬스체크 및 데몬 스크립트로 자동화하여 해결.
- 2026-10-24 Nuxt 3/4 설치 시도 시 `node-forge` (CVE-2023-52078) 및 `simple-git` 고위험 취약점이 검출되어 즉시 롤백하고 무결점 Vite 기반 유지.
- 원격 승인·자격 증명이 필요한 배포/법무/결제/독립감사는 요청·수행한 것으로 꾸미지 않는다.

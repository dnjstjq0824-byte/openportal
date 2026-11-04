# 검색·인증·메일 게이트웨이 실행 및 검수

작성일: 2026-11-04. 운영 배포 완료 문서가 아니다. 실제 검증 범위는 [메일 신뢰성 보고서](MAIL_RELIABILITY_2026_11_04.md)를 참고한다.

## 서비스 경계

- Vue/Vite: 사용자 화면과 `/ko`, `/th`, `/en` 경로.
- Node 웹 게이트웨이: `/api/v2/` 인증, 계정, 검색 프록시, JMAP 메일 연동. PostgreSQL `webapp` 스키마를 사용한다.
- Go API: 기존 검색 색인 조회와 검색어 제안. 브라우저 인증은 구형 메모리 인증 API가 아니라 웹 게이트웨이를 사용한다.
- SMTP: 회원가입 인증과 비밀번호 재설정 이메일 발송. 사용자 메일함 수신 서버와는 별개다.
- JMAP: 사용자 메일함 저장·조회·작성·발신 요청. 외부 제공업체 또는 별도 운영하는 호환 메일 서버가 필요하다.
- ClamAV: 업로드와 다운로드 검사. 검사기가 없거나 응답이 실패하면 첨부파일을 차단한다.

86개 테이블의 기본 SQL 스키마 및 100개 작업 계획은 제품 구현·검증과 별개다. 이번 메일 신뢰성 수정은 SQL을 변경하지 않았다.

## 로컬 실행

Node 의존성은 저장소 루트와 웹 앱에 각각 설치한다.

```powershell
npm.cmd ci
npm.cmd ci --prefix apps/web
```

`.env`가 이미 있으면 덮어쓰지 않는다. 없다면 [.env.example](../.env.example)을 복사하고 로컬 운영자가 값을 설정한다. 비밀정보는 저장소, 채팅, 스크린샷, 테스트 보고서에 넣지 않는다.

먼저 PostgreSQL 데이터베이스와 전용 마이그레이션 계정을 준비한다. 기존 DB에는 백업과 마이그레이션 이력을 확인한 후 적용한다. 기존 비관리 DB에 초기 스키마를 무작정 재실행하지 않는다.

```powershell
node --env-file=.env scripts/migrate.mjs
npm.cmd run dev:gateway
```

별도 터미널에서 기존 검색 인프라 설정으로 Go API를 실행한다. Go 실행은 루트 `.env`를 자동으로 읽지 않으므로 필요한 환경변수를 프로세스에 설정한다. 기존 색인 이름을 확인하고, 운영 색인에 테스트 시드를 넣지 않는다.

```powershell
Set-Location backend
go run ./cmd/api
```

다른 터미널에서 웹을 실행한다.

```powershell
npm.cmd run dev --prefix apps/web
```

웹 기본 주소는 `http://127.0.0.1:5173`, 웹 게이트웨이는 `127.0.0.1:8090`, 검색 API는 `127.0.0.1:8080`이다. 개발 서버를 공용 인터넷에 노출하지 않는다.

## 환경변수

| 구분 | 변수 | 설정 사항 |
|---|---|---|
| 기본 | `DATABASE_URL` | `webapp`에 접근하는 PostgreSQL 접속 정보. 운영 앱과 마이그레이션 계정은 분리한다. |
| 기본 | `PUBLIC_ORIGIN` | 브라우저의 정확한 origin. CSRF 검사와 인증 이메일 링크 생성에 사용한다. |
| 기본 | `GATEWAY_HOST`, `GATEWAY_PORT` | 기본 loopback/8090. 공개 ingress와 내부 네트워크 경계를 운영자가 구성한다. |
| 보안 | `NODE_ENV=production` | HTTPS origin, DB, 요청 제한 secret, ingress secret을 요구하고 Secure 쿠키를 사용한다. |
| 보안 | `RATE_LIMIT_SECRET` | 장기간 유지할 무작위 secret. 요청 제한 식별자를 해시하는 데 사용한다. |
| 보안 | `GATEWAY_SHARED_SECRET` | Pages와 운영 게이트웨이 사이의 secret. 클라이언트 번들 변수에 넣지 않는다. |
| 검색 | `SEARCH_BACKEND_URL` | 기존 Go API 주소. 기본 `http://127.0.0.1:8080`. |
| 검색 | `SEARCH_PROVIDER` | 현재 소스의 제공자 선택 설정. API별 계약·할당량·과금과 실제 접속은 별도 검증해야 한다. |
| SMTP | `SMTP_HOST`, `SMTP_PORT`, `SMTP_SECURE` | 실제 인증메일 SMTP 서버. 465의 implicit TLS 사용 시 secure=true, STARTTLS 방식은 제공업체 설정에 따른다. 운영에서는 TLS가 필요하다. |
| SMTP | `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | 발송 계정과 허가된 발신 주소. 계정 발급·도메인 인증은 제공업체 설정이다. |
| 메일 | `MAIL_DOMAIN` | 가입 시 생성하는 `아이디@도메인`의 도메인. 비운영 기본값은 예약 TLD인 `openscanner.test`. 운영에서는 소유·DNS 설정을 마친 도메인을 명시해야 한다. 메일함 생성 어댑터(`createGateway` 의 `provisioner`)가 없으면 가입은 `MAIL_PROVISIONING_UNAVAILABLE`로 거부된다. |
| 메일 | `MAIL_JMAP_URL` | HTTPS JMAP session URL. 현재 어댑터는 session/API/upload/download가 같은 origin인 제공업체를 요구한다. |
| 메일 | `MAIL_ENCRYPTION_KEY` | base64로 인코딩한 32바이트 키. DB에 저장하는 메일함 토큰을 암호화한다. 안전하게 백업하고 무작정 교체하지 않는다. |
| 검사 | `CLAMAV_HOST`, `CLAMAV_PORT` | 실제 ClamAV INSTREAM 검사기. 기본 포트 3310. |
| 운영자 CLI | `MAILBOX_EMAIL`, `MAILBOX_TOKEN` | 검증된 사용자와 그 사용자 전용 JMAP 토큰. 일반 웹 계정이나 공용 관리자 토큰을 사용하지 않는다. |

## 안전한 검수 계정과 메일 연결

1. 로컬 또는 승인된 스테이징 DB와 SMTP를 설정한다.
2. `/en/signup` 등 정상 가입 URL에서 검수자가 관리하는 실제 주소로 가입한다.
3. 수신한 인증 이메일 링크를 열고 인증한다. API 응답에서 토큰을 가져오거나 DB의 인증 상태를 직접 바꿔 운영 인증을 우회하지 않는다.
4. 메일 제공업체에서 해당 사용자의 메일함과 최소 범위 토큰을 생성한다. 메일함 생성 API는 제공업체별이므로 자동화되어 있지 않다.
5. 운영자 환경에 설정한 값으로 아래 CLI를 실행한다. 이 CLI는 인증된 사용자와 JMAP 발신 identity가 일치하는지 확인하고 토큰을 암호화해 저장한다.

```powershell
npm.cmd run provision:mail
```

CLI 실행 후 평문 `MAILBOX_TOKEN`을 일회성 운영자 환경에서 제거한다. 공유 관리자 토큰을 사용자 계정들에 연결하지 않는다.

6. `/en/mail`에 직접 접속해 로그인 복귀를 확인한다. 임시 저장, 메일 발송 수락, 보낸편지함을 확인한다.
7. 다른 실제 외부 메일함에서 테스트 주소로 이메일을 보내 수신을 확인한다. 반대 방향도 보내 외부 수신함 도착과 첨부파일을 확인한다.
8. 별도 사용자로 로그인해 다른 사용자의 메시지·폴더·첨부 ID 접근이 차단되는지 확인한다.

게이트웨이의 `accepted: true`는 제공업체가 발송 요청을 수락했다는 뜻이다. 최종 외부 수신함 도착, 반송 여부, 스팸 분류는 따로 확인한다. 현재 자동 테스트의 JMAP·검사기 모의 서버를 이 외부 검수의 근거로 사용하지 않는다.

## 발송 상태 불확실 시 대응

- 발송 중 타임아웃·연결 종료가 나면 같은 임시메일을 새 ID로 교체하거나 자동 재발송하지 않는다.
- `webapp.mail_send_requests`의 계정 ID/메일 ID와 제공업체 발송 내역을 대조한다. 로그에 본문·비밀번호·토큰을 출력하지 않는다.
- `submission_id`가 확인되면 제공업체 상태와 실제 수신 여부를 확인한다. 값이 비어 있다는 이유로 미발송으로 단정하지 않는다.
- 결과가 불명확한 동안 예약 행을 삭제해서 재발송을 허용하지 않는다. 제공업체가 거부를 확정한 경우에만 운영 절차에 따라 재작성·재발송을 결정한다.
- 현재 화면은 불확실한 임시메일의 재시도를 잠그고, 서버는 저장/발송 공통 잠금과 예약 검사로 다른 탭이나 새로고침을 통한 우회도 차단한다. 자동 상태 대조·운영자 복구 화면은 미구현이다.

## 자동 검증

```powershell
npm.cmd run test:platform
npm.cmd run test:mail:e2e
npm.cmd run typecheck --prefix apps/web
npm.cmd run build --prefix apps/web
```

Go 검사는 `backend`에서 `go test ./...`를 실행한다.

메일 E2E는 실제 브라우저·임시 PostgreSQL·loopback SMTP를 사용하며 JMAP와 악성코드 검사기는 프로토콜 모의 서버다. 임시 계정 비밀번호는 매 실행 무작위로 생성하며 공개 출력하지 않는다. 실계정이나 운영 DB를 사용하지 않는다. 테스트 DB는 `work/` 아래에 격리된다. 메일 전용 E2E에는 OpenSearch가 필요 없다.

전체 검색 E2E인 `npm.cmd run test:platform:e2e`에는 별도의 로컬 OpenSearch와 Go API가 필요하다. 이 테스트의 검색 문서는 합성 테스트 데이터이며 실제 외부 웹 커버리지의 근거가 아니다.

## 운영 개시 전 설정

- JMAP 제공업체 계약·도메인 소유권·사용자별 메일함을 준비한다.
- 수신용 MX, 발신 SPF/DKIM, DMARC 및 반송·스팸·남용 대응은 제공업체의 정확한 값을 받아 설정한다. 임의의 DNS 값을 배포하지 않는다.
- SMTP/JMAP/ClamAV 장애와 외부 송수신·첨부파일을 실제 스테이징에서 검증한다.
- Pages의 `WEB_GATEWAY` 서비스 바인딩 또는 `WEB_GATEWAY_ORIGIN`과 ingress secret을 설정한다. 사용자 API를 구형 데모 응답으로 대체하지 않는다.
- `/healthz`, `/readyz`, 인증된 `/metrics`와 [알림 규칙](../infra/platform-alerts.yaml)을 운영 모니터링에 연결한다. 파일 존재는 실제 알림 발송의 증거가 아니다.
- PostgreSQL과 암호화 키의 접근권한·백업·실제 복구 훈련을 준비한다. 메일 제공업체 저장소의 백업·보관·삭제도 별도 확인한다.
- 계정에 메일 연결이 남아 있으면 계정 삭제는 제공업체 데이터 삭제를 먼저 요구한다. 전체 downstream 삭제 완료를 허위로 표시하지 않는다.
- 개인정보 정책의 운영 주체·연락처·보관 기간·처리 지역과 법적 근거, 독립 보안 검토가 필요하다.

운영 배포는 별도 승인 후 실행한다. 롤백은 승인된 웹/게이트웨이 버전을 함께 복원하고 추가된 `webapp` 데이터를 보존하는 방식으로 준비한다. 구형 메모리 인증 또는 가짜 이메일 성공 응답을 복원하는 방식은 사용하지 않는다.

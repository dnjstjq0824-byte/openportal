# Phase 0 threat model / 010

신뢰 경계: 브라우저 → Go API → 내부 OpenSearch; migration role → PostgreSQL.
인터넷 문서 → 미래 crawler/parser → 색인은 별도의 비신뢰 경계이며 현재 비활성이다.

| 위협 | 현재 통제/검증 | 잔여 위험·필수 게이트 |
|---|---|---|
| 쿼리 주입·과다 페이지 | JSON serialization, 200자/100페이지, 테스트 | quota/분산 rate limit 미구현 |
| upstream 장애·부분 결과 | timeout/failed shards/잘못된 JSON 503 | 실제 부하/가용성 미측정 |
| 저장 XSS·위험 링크 | Vue text escaping, API http(s) 필터 | 크롤러 sanitize/권리 미검증 |
| 관리 권한 탈취 | admin/partner route 없음; 404 검증 | OIDC/MFA/BOLA/RLS 후 공개 |
| DB 권한·변조 | PUBLIC revoke, SHA256/lock/transaction | 운영 역할/테넌트 분리·백업 |
| 개인정보 노출 | 검색 이력 비저장, 오류 로그 일반화 | 전경로 삭제·운영 로그 감사 |
| SSRF/DNS rebinding | crawler 없음 | 주소 pin·redirect·robots 시험 전 수집 금지 |
| 광고 오인·부정 청구 | 광고/결제 없음 | 표시·멱등 원장·sandbox·독립정산 검수 |
| 공급망 | lockfiles, CI scans/SBOM | 독립 검토·이미지 digest·서명 |
| 데이터 권리 | 합성 example.org fixture만 적재 | 소스별 계약/법무 승인 |

독립 승인 요청 패키지: PRD, ADR, GAP_REGISTER, PHASE0_REPORT, evidence 및 구현 commit.
검토자는 실제 DB/브라우저 재현과 보안·권리·회귀 증적을 확인하고 서명해야 한다.
검토자 미지정·승인 미수령. Phase gate 및 상용화는 NO-GO. 연락/승인 사실을 생성하지 않는다.

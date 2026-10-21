# Ownership and review

현재 구현 책임: 이 작업의 Codex 기술 리드. 저장소 소유자: 로컬 git 설정의 dnjstjqppk-dev.
독립 보안/법무/정산/검색품질 승인자는 아직 지정되지 않았다. 소유자와 구현자는 독립감사자를 대신할 수 없다.

| 경로 | 검토 역량 |
|---|---|
| backend, proto | Go/API/검색 |
| database | PostgreSQL/데이터 권한 |
| apps/web, admin, partner | Vue/Nuxt/접근성 |
| apps/mobile | Flutter/플랫폼 |
| infra, .github | 운영/공급망 |
| docs | 제품/검수 증적 |

main은 검증 기준 브랜치. 작업은 phaseN/topic 브랜치와 atomic commit으로 진행한다.
커밋에는 목표·변경·검증을 남기고 증적 보고서가 해당 구현 SHA를 참조한다.
푸시/호스팅/브랜치 보호는 원격 저장소가 지정된 뒤 설정한다. CODEOWNERS만으로 보호가 작동하지 않는다.

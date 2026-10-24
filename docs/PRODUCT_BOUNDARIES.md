# Phase 0 경쟁 기능·권리 경계 — 002

본 문서는 경쟁 검색 포털(Google Search, 네이버 통합검색, Microsoft Bing)의 공식 공개 문서·웹마스터 가이드라인을 기준으로 기능 매트릭스와 OpenPortal Search의 법적·권리 경계를 정의한다.
경쟁사 기능 조사는 기술 설계 참조용이며, 상표·콘텐츠·지도·미디어·결제에 대한 **외부 법무 검토 및 라이선스 계약 체결은 미완료(승인 차단)** 상태다.

## 1. 공식 자료 기반 경쟁 제품 기능 매트릭스

| 기능 축 | Google Search (Search Central / Console) | 네이버 검색 (Search Advisor / 웹마스터 가이드) | Microsoft Bing (Webmaster Guidelines / Copilot) | OpenPortal Phase 0 현황 | OpenPortal Phase 1~9 목표 및 경계 |
|---|---|---|---|---|---|
| **크롤러·접근 제어** | `Googlebot` (RFC 9309 `robots.txt`), AI 학습 제어용 `Google-Extended` 별도 분리(일반 검색 랭킹 무관) | `Yeti` (`User-agent: Yeti`), `robots.txt` 루트 배치, IP 대역 대신 UA 기반 제어 권장, 301 리다이렉트 승계 | `Bingbot` 단일 크롤러(웹 검색·Copilot 공통), `robots.txt` 및 CDN/WAF 허용, `IndexNow` 즉시 제출 지원 | 크롤러 미구현 (합성 fixture 3건만 적재, 외부 수집 비활성) | Phase 2(`021`~`030`): RFC 9309 `robots.txt` 파서/캐시, SSRF·DNS rebinding·비공개 IP 차단, 호스트별 politeness 검증 후 허가 사이트만 제한 수집 |
| **문서·스니펫 지시자** | `<meta name="robots">` 및 `X-Robots-Tag`: `noindex`(색인·AI 제외), `nosnippet`, `data-nosnippet`, `max-snippet` | 표준 `noindex`, `nofollow`, canonical 링크, 정적/SSR HTML 마크업 및 이미지 `alt` 권장 | `noindex`, `noarchive`/`nocache`(캐시 및 Copilot grounding 제외), `nosnippet` 지원 | OpenSearch `indexable=false` 문서 검색/추천 제외(`008` 회귀 검증 완료) | Phase 2~3(`029`, `038`): `noindex`/`nofollow`/`X-Robots-Tag`/canonical 파싱 및 전 경로(DB·색인·캐시·스토리지) tombstone 삭제 전파 |
| **웹마스터·파트너 도구** | Search Console: 도메인/URL 접두어 소유권 확인, Sitemap 제출, URL 검사, 검색 성과 및 삭제 요청 | 서치어드바이저: HTML 파일/메타태그 소유 확인, Sitemap/RSS 제출, 수집 현황·오류 진단 | Webmaster Tools: 소유권 확인, Sitemap, IndexNow API, 크롤링 제어 및 인용 통계 | 미구현 (관리자·파트너 경로 기본 거부 `404`/`403`) | Phase 1(`018`), Phase 2(`021`), Phase 7(`071`~`072`): 별도 파트너 포털, DNS/HTML/meta 소유권 챌린지(`publisher.verification_challenges`), Sitemap·URL 검사 |
| **수직검색 (뉴스·지도·쇼핑·미디어)** | Google News(발행사 정책), Google Maps(자체/라이선스 지리정보), Shopping(Merchant Center 피드), 썸네일/비디오 미리보기 | 뉴스(제휴 언론사 전재 계약), 지도/플레이스(스마트플레이스·공공/제휴 DB), 쇼핑(입점/상품 피드·수수료 공시) | Bing News(발행사 피드), Bing Maps(라이선스 지리 데이터), Shopping(판매자 상품 피드) | 미구현 (`apps/web`에서 비활성 탭 또는 기능 플래그로 숨김) | Phase 1(`013`), Phase 5(`051`~`060`): 기능 플래그 기본 비활성. 무단 스크래핑 금지, 라이선스·제휴 계약된 피드/API와 출처·정정·삭제 전파만 허용 |
| **광고·상업성 분리** | Google Ads: 유기적 검색(Organic)과 독립된 경매, 명확한 `스폰서`/`Sponsored` 라벨 표기 | 파워링크/비즈사이트 등 광고 영역 분리 및 `광고` 명시, 유기적 랭킹과 과금 분리 | Microsoft Advertising: 유기적 결과와 분리된 `Ad`/`Sponsored` 라벨링 | 광고·결제 미구현 | Phase 7(`073`~`080`): 유기적 점수와 완전 분리된 `Sponsored`/`광고` 컨테이너, 불변 원장, 샌드박스 대조 및 독립 감사 전 실결제 금지 |
| **생성형 AI 답변·출처** | AI Overviews / AI Mode: 다중 출처 링크 인용, `noindex`/`nosnippet` 및 Search Console AI 제어 반영 | Cue: 등 생성형 검색 실험에서 출처 명시 및 제휴·검색 문서 기반 답변 | Copilot in Bing: 실시간 웹 grounding, 출처 각주·링크 표시, `noarchive`/`nocache` 시 grounding 제외 | 미구현 | Phase 6(`061`~`070`): 검색된 허용 문서(`robots_indexable=true` 및 라이선스 확인)만 근거로 인용, 근거 부족 시 일반 검색 fallback, 환각·프롬프트 인젝션 차단 |
| **개인정보·검색 이력** | 계정 기반 웹 및 앱 활동 제어, 자동 삭제 주기 설정, 개인정보 삭제 요청(법적·정책적 제거) | 검색어 저장 켜기/끄기, 최근 검색어 삭제, 권리침해·개인정보 게시물 중단 요청 | 검색 기록 관리/삭제, 개인정보 대시보드, 권리 침해 신고 | 서버 검색 이력 기본 비수집 (DB에 쿼리 원문 미저장) | Phase 1(`012`, `017`), Phase 8(`083`~`085`): 최근검색·개인화 명시적 opt-in, 정보주체 열람·삭제·이동 권리 실행 및 백업/색인 연계 파기 |

## 2. 상표·지식재산·콘텐츠·지도·미디어 라이선스 경계

1. **상표 및 브랜드 경계 (Trademark & Brand Dress)**
   - Google, 네이버(Naver), Microsoft Bing, Yeti, Googlebot, Copilot 등의 명칭은 경쟁 기능 분석 및 표준 프로토콜 비교 목적으로만 언급한다.
   - 경쟁사의 로고, 아이콘, 고유 컬러 조합, 독점 폰트, 전용 UI 레이아웃(Brand Dress), 비공개 API나 랭킹 수식을 복제하지 않는다.
   - 가칭 **OpenPortal Search** 명칭 및 도메인에 대한 상표권 출원·저촉 여부 검토는 **법무 미승인(차단)** 상태이므로, 외부 상용 배포를 금지한다.
2. **웹 수집 및 저작권 경계 (Web Crawling & Copyright)**
   - 경쟁 검색엔진의 SERP(검색 결과 페이지) 무단 스크래핑, 자동화 우회 수집, 캐시 미러링은 엄격히 금지한다(`AGENTS.md`, `SECURITY_AND_COMPLIANCE.md`).
   - `robots.txt`(RFC 9309) 허용이 저작권법상 원문 전체 복제·재배포나 유료 모델 학습 라이선스를 자동으로 부여하지 않는다. 공개 웹 검색에서는 제목·표준 길이 이내의 발췌 스니펫·원문 canonical URL 링크만 제공하며, `noindex`/`nosnippet`/`X-Robots-Tag`를 즉시 존중한다.
   - Phase 0의 `example.org` 문서는 로컬 시드 스크립트(`scripts/seed.py`)가 생성한 합성 데이터이며 실제 외부 사이트 수집본이 아니다.
3. **뉴스·미디어·이미지·동영상 경계 (Media & News Rights)**
   - 뉴스 기사 전문 저장·재배포는 언론사/발행사와의 명시적 RSS/API 제휴 계약(`portal.news_publishers.licensed = true`) 없이는 수행하지 않는다.
   - 이미지 썸네일 및 동영상 미리보기는 라이선스(`portal.media_assets.license`), 저작권자 표기(`attribution`), 안전 분류(`safety_status`), 원본 삭제 시 즉시 파기 경로가 검증되기 전까지 프로덕션 노출을 차단한다.
4. **지도·장소(POI)·쇼핑 데이터 경계 (Maps, Places & Commerce)**
   - 지도 타일, 지오코딩, 장소(POI) 정보는 오픈 라이선스(예: ODbL 요구사항을 준수하는 OpenStreetMap) 또는 상용 라이선스 계약이 체결된 파트너 API만 사용하며, 타사 지도 타일/POI 무단 추출을 금지한다.
   - 쇼핑 상품 가격·재고는 입점 판매자/제휴 피드 계약에 기반해야 하며, 제휴·광고 여부를 명확히 공시한다.

## 3. 라이선스 등록 원장 및 법무 승인 게이트 (필수 메타데이터)

외부 데이터 소스, 수직검색 피드, 지도 타일, AI 모델을 연결하기 전 아래 항목을 문서와 DB(`crawl.seed_sources`, `portal.news_publishers`, `control.retention_policies`)에 기록하고 독립 법무/컴플라이언스 서명을 받아야 한다:

- **소스 식별자 및 제공자**: 도메인/피드 URI, 법인/권리자 명칭
- **법적 근거 및 라이선스 종류**: 상용 계약서 번호, 오픈 라이선스(CC-BY, ODbL 등) 또는 웹마스터 직접 제출(`publisher.sites` 소유권 검증 완료)
- **허용 범위**: 색인 가능 여부, 본문 스니펫 최대 길이, 썸네일 생성 허용 여부, AI 답변(RAG) 인용 허용 여부
- **보유 기한 및 삭제 SLA**: 원본 삭제/요청 시 캐시·OpenSearch·S3·PostgreSQL 파기 시한
- **검토 및 승인 상태**: 현재 모든 외부 콘텐츠/상표/지도/뉴스 계약은 **미지정·미승인(BLOCKED — G010)** 상태다.

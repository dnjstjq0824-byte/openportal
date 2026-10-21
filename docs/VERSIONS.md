# Phase 0 version policy / 004

| 도구 | 고정 버전 | 상태 |
|---|---|---|
| Go | 1.27.3 | 로컬 실행; go.mod/CI 최소 1.27.3 |
| Node | 24.19.0 | 로컬 실행; .node-version |
| Python | 3.12.14 | bundled runtime 실행 |
| PostgreSQL | 17.10 | embedded-postgres 17.10.0-beta.17 실제 native 엔진 |
| OpenSearch | 2.19.0 | 공식 Windows ZIP SHA512 검증; fixture standard analyzer |
| Vue | 3.5.44 | package-lock 고정 |
| Vite | 6.4.4 | package-lock 고정 |
| TypeScript | 5.9.3 | package-lock 고정 |
| Buf | 1.74.0 | 로컬 lint/build/breaking |
| protoc-gen-go | 1.36.6 | 생성 코드 버전 고정 |
| Nuxt | 3.17.5 후보 | 011 도입시 호환성·보안 검증 필요; 설치/지원 승인 안 됨 |
| Flutter | 3.32.8 후보 | SDK 없음; 네이티브 빌드 및 Dart 호환성 미검증 |

위 표는 재현용 기준으로 최신·안전·공급자 지원이 검증되었다는 의미가 아니다.
patch 변경도 lock 갱신·DB/계약/E2E 회귀 후 허용한다. 보안 공지는 매 릴리스 확인한다.
Nuxt/Flutter 도입과 공급자 지원/EOL·OS 이미지 digest 감사가 끝나기 전 004는 부분 상태다.

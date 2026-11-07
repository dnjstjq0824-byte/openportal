# 백업·복구 드릴 절차서

- 목표 RPO 24시간, RTO 4시간 (소규모 상용 기준).
- 실드릴 상태: 로컬에 pg_dump 바이너리 없음(내장 PG는 서버 바이너리만, 시스템 설치 없음).
  아래 절차는 운영 PostgreSQL 확보 후 첫 드릴에서 그대로 실행한다.
  마이그레이션(APPLIED 001~014) 멱등성은 테스트 스택 부팅으로 반복 검증됨.

## 1. PostgreSQL 백업 (운영)

```sh
pg_dump --host=$PGHOST --port=5432 --username=$PGUSER --dbname=openscanner \
  --format=custom --compress=6 --file=backup-$(date +%F).dump
sha256sum backup-*.dump > backup.sha256
```

보관: 리전 분리 스토리지 2곳, 30일 보관, 주 1회 무결성 검사.

## 2. OpenSearch 스냅샷 (운영)

```sh
curl -XPUT "$OS/_snapshot/s3_repo/%3Csnap-%7Bnow%2Fd%7D%3E?wait_for_completion=true"
curl "$OS/_snapshot/s3_repo/_all?pretty" | grep state
```

색인은 재생성 가능(크롤러 재수집)하므로 스냅샷 주기는 주 1회로 충분.

## 3. 복구 드릴 (분기 1회, 기록 필수)

1. 격리 DB에 리스트어: `pg_restore --clean --if-exists -d drill backup.dump`.
2. 검증 쿼리: 계정 수, 세션 테이블 존재, 최신 created_at, 마이그레이션 버전.
3. 게이트웨이 smoke: /session 200, /search 200, 로그인 1건.
4. 결과 기록: 소요 시간, 불일치 항목, 조치. RTO 초과 시 절차 개정.

## 4. 게이트웨이 상태

세션·CSRF는 DB 저장이라 무상태 복구 가능. .env·키(MAIL_ENCRYPTION_KEY)는
별도 비밀 보관소 백업 대상이며 DB 덤프에 평문으로 남기지 않는다.

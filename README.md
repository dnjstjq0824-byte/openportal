# OpenPortal Search — Codex leadership pack / 개발 스타터

## Current Phase 0 work (2026-10-21)

See `docs/PHASE0_REPORT.md`, `docs/IMPLEMENTATION_STATUS.md` (all 100 tasks), and
`docs/GAP_REGISTER.md`. The packaging-time claims below are historical.
Real PostgreSQL 17.10 migration and native OpenSearch → Go → Chrome/Vue tests have
been executed; Compose and independent approval remain outstanding. No crawler exists.

Reproduce from this directory with Node 24.19.0, Go 1.27.3, Python 3.12.14 and Buf 1.74.0:
```bash
npm ci
npm ci --prefix apps/web
npm run test:db
node scripts/test-proto.mjs
npm run test:contracts
npm run typecheck --prefix apps/web
npm run build --prefix apps/web
# Start OpenSearch 2.19.0 locally on 127.0.0.1:9200, then:
python scripts/seed.py
npx playwright install chromium
npm run test:e2e
```
`test:e2e` starts Go and Vue and requires real OpenSearch. On Windows an installed
Chrome can be selected via `CHROME_PATH`. `test:db` creates a fresh local cluster
on 127.0.0.1:55432 and stops it after tests; retained data lives under ignored `work/`.
For a Docker host, `docker compose -f infra/compose.yaml --profile app up --build`
defines the complete local stack, but this path has **not** been executed on this host.
Do not enable public bindings. `001` must be applied by `scripts/migrate.mjs` on a
new DB; existing unmanaged starter DBs need explicit baseline adoption, not blind replay.


> This is a **development starter**, not a complete Google clone or production-ready search engine.
> Deliverables: Codex master directive, 100 work items in 10 phases, 86 PostgreSQL control-plane tables, protobuf service contracts, Go search API, Vue browser demo, Flutter source stub, OpenSearch mapping & fixture seeder, local Docker Compose.

## Where to start
1. Read `CODEX_MASTER_PROMPT.md` and pass its entire contents to Codex **in the repository root**.
2. Treat `docs/PHASES_0_TO_9.md` as the source of truth for 100 gates; keep status in `docs/IMPLEMENTATION_STATUS.md` (Codex creates it).
3. Treat `database/migrations/001_initial.sql` as an architectural baseline, not evidence every feature is implemented.

## Run locally (requires Docker, Go 1.23+, Node 22+, Python 3)
```bash
cd infra
docker compose up -d
cd ..
python3 scripts/seed.py
cd backend
go test ./...
go run ./cmd/api
# Another terminal
cd apps/web
npm install
npm run dev
```
Open http://127.0.0.1:5173 and search `검색엔진`, `OpenPortal` or `한글` after indexing fixtures.

## Flutter (source-only example)
Flutter SDK is not available in the packaging environment. In an initialized Flutter project copy `apps/mobile/lib/main.dart` and `pubspec.yaml`; create platform host files by `flutter create --platforms=android,ios .` inside `apps/mobile`. Android emulator reaches the API at `http://10.0.2.2:8080`; for physical devices use your LAN IP and safe dev networking. Do not allow cleartext HTTP on release builds.

## API
- `GET /healthz`
- `GET /api/v1/search?q=%EA%B2%80%EC%83%89%EC%97%94%EC%A7%84&page=1&lang=ko` (lang optional)
- `GET /api/v1/suggest?q=Open`

## Verification status
- `go test ./...` PASSED for the packaged Go backend (unit and mocked OpenSearch transport tests).
- End-to-end Docker integration was **not executed** here (no Docker binary available), so schema migration and Docker image availability still require testing in the user's environment.
- Node dependencies, native Flutter build, real crawler, ranking quality, payments and security audits are **not verified**.
- Seed pages are fictional fixture examples, not real publicly crawled results.

## Important
The web app is a Vue/Vite starter. In the Codex Phase 1, implement the production-facing Nuxt app, while maintaining this smoke-test frontend until replaced. Admin and partner readmes mark intentionally unimplemented consoles; no fake completed placeholders should be deployed.

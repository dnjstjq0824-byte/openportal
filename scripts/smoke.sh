#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
echo '[1/4] go tests'
(cd backend && go test ./...)
echo '[2/4] seed dev OpenSearch fixtures'
python3 scripts/seed.py
echo '[3/4] start Go backend in another terminal: (cd backend && go run ./cmd/api)'
echo '[4/4] manually verify: curl -G --data-urlencode "q=검색엔진" http://localhost:8080/api/v1/search'

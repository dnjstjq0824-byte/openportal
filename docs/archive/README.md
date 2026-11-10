# Archive — Superseded / Unverified Reports (2026-11-10)

This directory holds reports removed from the active tree during Phase 1 Step 1.

## Why archived

- `root/` (33 files): root-level `OPENSCANNER_*` / `OPEN_SCANNER_*` glossy completion
  reports and matrices. They cite fabricated load-test figures (`10,000 QPS PASSED`,
  `p95 8.4ms`), nonexistent source files (`backend/internal/ranking/bm25.go`,
  `backend/internal/ranking/analyzer.go`), and premature GO verdicts.
- `docs/` (30 files): duplicate copies that lived under `docs/`.

Kept active: `docs/OPENSCANNER_FOUNDATION_2026_11_04.md` (referenced evidence),
runbooks, phase reports, contracts, and architecture docs.

## Single source of truth

`docs/GAP_REGISTER.md` is the only progress/status authority.
Do not cite archived files as evidence of completion.

## Restore

History-preserving restore: `git log -- docs/archive/<file>` then
`git checkout <commit> -- <original-path>`.

# Codex repository instructions

- Read `CODEX_MASTER_PROMPT.md` and `docs/PHASES_0_TO_9.md` first.
- Report source implementation separately from the 86-table baseline SQL schema and 100-task plan.
- Never invent tests, deploys, approvals or commit hashes. Mark blocked work as blocked.
- Phase gate needs runtime evidence; no stubs, placeholder mock results or merely documented features count as done.
- No unauthorized Google scraping, no copyrighted/trademarked asset copying, respect robots and privacy.
- Test Go with `cd backend && go test ./...`.
- Before editing SQL, check FK order and execute the migration against a disposable PostgreSQL instance.
- Admin/partner are NOT implemented; deny-by-default and keep endpoints inaccessible until auth/authorization is built.
- Avoid enabling payments, deploys or crawling arbitrary domains without authorized infrastructure and legal review.

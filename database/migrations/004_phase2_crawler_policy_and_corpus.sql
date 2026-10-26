-- Phase 2 forward migration: crawler policy, bounded corpus seeds, and frontier queue performance indexes.
-- Note: outer transaction is managed by scripts/migrate.mjs.

-- 1. Feature flags: ensure arbitrary crawling is strictly disabled by default (Phase 2 gate)
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('crawler.arbitrary_enabled', false, '{"surface":"crawler","phase":2,"description":"Arbitrary internet crawling strictly disabled; requires legal/compliance sign-off and authorized infrastructure"}'::jsonb),
  ('crawler.bounded_corpus_enabled', true, '{"surface":"crawler","phase":2,"description":"Bounded corpus crawling permitted only for approved verified test sites"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  rules = EXCLUDED.rules;

-- 2. Seed 10 authorized test corpus hosts into crawl.hosts (Task 030 Bounded Crawl Gate)
INSERT INTO crawl.hosts (scheme, hostname, port, fetch_enabled, crawl_delay_ms) VALUES
  ('https', 'example.com', 443, true, 1000),
  ('https', 'example.org', 443, true, 1000),
  ('https', 'openportal.org', 443, true, 1000),
  ('https', 'docs.openportal.org', 443, true, 1000),
  ('https', 'standards.iso.org', 443, true, 2000),
  ('https', 'rfc-editor.org', 443, true, 1000),
  ('https', 'w3.org', 443, true, 1000),
  ('https', 'iana.org', 443, true, 1000),
  ('https', 'developer.mozilla.org', 443, true, 1500),
  ('https', 'ko.wikipedia.org', 443, true, 2000)
ON CONFLICT (scheme, hostname, port) DO NOTHING;

-- 3. Performance indexes for frontier queue leasing and attempt lookup
CREATE INDEX IF NOT EXISTS idx_frontier_scheduled_priority
  ON crawl.frontier (state, priority DESC, scheduled_at ASC);

CREATE INDEX IF NOT EXISTS idx_fetch_attempts_frontier
  ON crawl.fetch_attempts (frontier_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_raw_objects_checksum
  ON crawl.raw_objects (checksum_sha256);

-- 4. RLS for URL submissions to ensure account data isolation
ALTER TABLE crawl.url_submissions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS url_submissions_account_policy ON crawl.url_submissions;
CREATE POLICY url_submissions_account_policy ON crawl.url_submissions
  USING (
    submitted_by IS NULL OR
    submitted_by = NULLIF(current_setting('app.current_account_id', true), '')::uuid
  );

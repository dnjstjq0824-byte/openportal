-- Phase 4 (Part D): security audit log. Every sensitive gateway action
-- appends one row (auth events, account changes, reports, denied admin
-- attempts). Rows are append-only by convention: no application path issues
-- UPDATE/DELETE here; retention purge (if ever adopted) must be an explicit,
-- reviewed migration -- never silent trimming.
CREATE TABLE IF NOT EXISTS webapp.audit_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  occurred_at timestamptz NOT NULL DEFAULT now(),
  account_id uuid REFERENCES webapp.accounts(id) ON DELETE SET NULL,
  action text NOT NULL CHECK (char_length(action) BETWEEN 1 AND 64),
  target_type text CHECK (target_type IS NULL OR char_length(target_type) BETWEEN 1 AND 64),
  target_id text CHECK (target_id IS NULL OR char_length(target_id) BETWEEN 1 AND 256),
  ip text CHECK (ip IS NULL OR char_length(ip) BETWEEN 1 AND 80),
  user_agent text CHECK (user_agent IS NULL OR char_length(user_agent) BETWEEN 1 AND 300),
  meta jsonb NOT NULL DEFAULT '{}'::jsonb,
  request_id text CHECK (request_id IS NULL OR char_length(request_id) BETWEEN 1 AND 64)
);
CREATE INDEX IF NOT EXISTS audit_log_account_idx ON webapp.audit_log(account_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS audit_log_action_idx ON webapp.audit_log(action, occurred_at DESC);

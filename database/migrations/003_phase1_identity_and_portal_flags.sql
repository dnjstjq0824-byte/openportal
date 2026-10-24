-- Phase 1 forward migration: passkey challenges, recovery codes, opt-in recent searches,
-- role-based least-privilege boundaries, and initial vertical feature flags.
-- Note: outer transaction is managed by scripts/migrate.mjs.

ALTER TABLE identity.preferences
  ADD COLUMN IF NOT EXISTS recent_searches_opt_in boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS search_correction_opt_in boolean NOT NULL DEFAULT true;

ALTER TABLE identity.preferences
  DROP CONSTRAINT IF EXISTS preferences_safe_search_check,
  ADD CONSTRAINT preferences_safe_search_check CHECK (safe_search IN ('strict', 'moderate', 'off')),
  DROP CONSTRAINT IF EXISTS preferences_theme_check,
  ADD CONSTRAINT preferences_theme_check CHECK (theme IN ('light', 'dark', 'system')),
  DROP CONSTRAINT IF EXISTS preferences_language_check,
  ADD CONSTRAINT preferences_language_check CHECK (language IN ('ko', 'en'));

CREATE TABLE IF NOT EXISTS identity.auth_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid REFERENCES identity.accounts(id) ON DELETE CASCADE,
  challenge_type text NOT NULL CHECK (challenge_type IN ('oidc_nonce', 'passkey_register', 'passkey_login', 'mfa_stepup')),
  challenge_hash bytea NOT NULL UNIQUE,
  rp_id text NOT NULL DEFAULT 'localhost',
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS identity.recovery_codes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  code_hash bytea NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(account_id, code_hash)
);

CREATE TABLE IF NOT EXISTS identity.recent_searches (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  query_normalized text NOT NULL CHECK (char_length(query_normalized) BETWEEN 1 AND 200),
  locale text NOT NULL DEFAULT 'ko-KR',
  searched_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(account_id, query_normalized)
);

CREATE INDEX IF NOT EXISTS identity_recent_searches_account_idx
  ON identity.recent_searches(account_id, searched_at DESC);

ALTER TABLE identity.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.mfa_factors ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.recovery_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.recent_searches ENABLE ROW LEVEL SECURITY;
ALTER TABLE identity.organization_members ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS sessions_self_policy ON identity.sessions;
CREATE POLICY sessions_self_policy ON identity.sessions
  USING (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid);

DROP POLICY IF EXISTS preferences_self_policy ON identity.preferences;
CREATE POLICY preferences_self_policy ON identity.preferences
  USING (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid)
  WITH CHECK (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid);

DROP POLICY IF EXISTS mfa_factors_self_policy ON identity.mfa_factors;
CREATE POLICY mfa_factors_self_policy ON identity.mfa_factors
  USING (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid);

DROP POLICY IF EXISTS recovery_codes_self_policy ON identity.recovery_codes;
CREATE POLICY recovery_codes_self_policy ON identity.recovery_codes
  USING (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid);

DROP POLICY IF EXISTS recent_searches_self_policy ON identity.recent_searches;
CREATE POLICY recent_searches_self_policy ON identity.recent_searches
  USING (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid)
  WITH CHECK (account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid);

DROP POLICY IF EXISTS org_members_tenant_policy ON identity.organization_members;
CREATE POLICY org_members_tenant_policy ON identity.organization_members
  USING (
    account_id = NULLIF(current_setting('app.current_account_id', true), '')::uuid
    OR organization_id = NULLIF(current_setting('app.current_org_id', true), '')::uuid
  );

REVOKE ALL ON ALL TABLES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control FROM PUBLIC;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA identity, crawl, catalog, query, portal, ads, publisher, control FROM PUBLIC;

INSERT INTO identity.roles (id, description) VALUES
  ('partner_owner', 'Verified partner organization owner'),
  ('partner_member', 'Partner organization member'),
  ('security_auditor', 'Independent security and compliance reviewer')
ON CONFLICT (id) DO NOTHING;

INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('vertical.web', true, '{"surface":"user_portal","phase":1,"description":"Web organic search"}'::jsonb),
  ('vertical.image', false, '{"surface":"user_portal","phase":5,"reason":"Requires licensed media pipeline (051-052)"}'::jsonb),
  ('vertical.news', false, '{"surface":"user_portal","phase":5,"reason":"Requires publisher licensing agreements (054-055)"}'::jsonb),
  ('vertical.video', false, '{"surface":"user_portal","phase":5,"reason":"Requires licensed video metadata & embed rights (053)"}'::jsonb),
  ('vertical.map', false, '{"surface":"user_portal","phase":5,"reason":"Requires licensed geo/POI dataset (056-057)"}'::jsonb),
  ('vertical.shopping', false, '{"surface":"user_portal","phase":5,"reason":"Requires merchant feed & disclosure verification (058)"}'::jsonb),
  ('portal.recent_searches_opt_in', true, '{"surface":"user_portal","phase":1,"default_opt_in":false}'::jsonb),
  ('portal.query_correction', true, '{"surface":"user_portal","phase":1}'::jsonb)
ON CONFLICT (name) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  rules = EXCLUDED.rules,
  updated_at = now();

INSERT INTO query.rewrites (source_pattern, replacement, locale, priority, enabled) VALUES
  ('rjatordpswls', '검색엔진', 'ko-KR', 100, true),
  ('dhvmsvhxjf', '오픈포털', 'ko-KR', 95, true),
  ('gksrmf', '한글', 'ko-KR', 90, true),
  ('검섹엔진', '검색엔진', 'ko-KR', 85, true),
  ('오픈포탈', '오픈포털', 'ko-KR', 80, true),
  ('openprotal', 'OpenPortal', 'en-US', 100, true),
  ('serach engine', 'search engine', 'en-US', 90, true)
ON CONFLICT DO NOTHING;


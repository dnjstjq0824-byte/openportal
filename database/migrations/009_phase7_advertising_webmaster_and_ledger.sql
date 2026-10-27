-- 009_phase7_advertising_webmaster_and_ledger.sql
-- Phase 7: Advertising campaign state machine, click fraud detection, double-entry ledger, and webmaster verification

-- Feature flags for Phase 7 advertising and transaction gates
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('ads.auction_pipeline', false, '{"phase": 7, "description": "Commercial ad auction, bidding and sponsored placement"}'::jsonb),
  ('billing.real_transactions', false, '{"phase": 7, "description": "Real financial money transfer and payment processing - fail closed"}'::jsonb),
  ('webmaster.domain_verification', true, '{"phase": 7, "description": "Webmaster site ownership verification via DNS TXT or meta tag"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled, rules = EXCLUDED.rules;

-- Webmaster domain ownership verification records
CREATE TABLE IF NOT EXISTS portal.domain_verifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  domain text NOT NULL,
  verification_method text NOT NULL, -- 'dns_txt', 'html_meta', 'file_upload'
  verification_token text NOT NULL,
  verified boolean NOT NULL DEFAULT false,
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(account_id, domain)
);

-- Campaign state machine transition audit trail
CREATE TABLE IF NOT EXISTS ads.campaign_state_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES ads.campaigns(id) ON DELETE CASCADE,
  from_status text NOT NULL,
  to_status text NOT NULL,
  actor_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  reason text,
  occurred_at timestamptz NOT NULL DEFAULT now()
);

-- Click event ledger with fraud and deduplication tags
CREATE TABLE IF NOT EXISTS ads.click_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_id uuid NOT NULL,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  client_ip text NOT NULL,
  user_agent text NOT NULL,
  is_valid boolean NOT NULL DEFAULT true,
  invalidation_reason text,
  cost_amount numeric(12, 4) NOT NULL DEFAULT 0.0000,
  currency text NOT NULL DEFAULT 'KRW',
  clicked_at timestamptz NOT NULL DEFAULT now()
);

-- Double-entry accounting ledger for advertising balances
CREATE TABLE IF NOT EXISTS ads.ledger_entries (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE RESTRICT,
  entry_type text NOT NULL, -- 'credit', 'debit'
  amount numeric(14, 4) NOT NULL CHECK (amount > 0),
  currency text NOT NULL DEFAULT 'KRW',
  reference_type text NOT NULL, -- 'campaign_charge', 'deposit', 'refund', 'chargeback'
  reference_id text NOT NULL,
  balance_after numeric(14, 4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

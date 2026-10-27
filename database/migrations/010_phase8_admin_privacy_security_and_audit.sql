-- 010_phase8_admin_privacy_security_and_audit.sql
-- Phase 8: Privileged access management (PAM), GDPR/PIPA DSAR requests, dual custody, and security audit log

-- Feature flags for Phase 8 governance and security gates
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('admin.privileged_elevation_enabled', true, '{"phase": 8, "description": "Just-In-Time time-bound privileged role elevation"}'::jsonb),
  ('privacy.dsar_portal_enabled', true, '{"phase": 8, "description": "GDPR and PIPA Data Subject Rights request workflow"}'::jsonb),
  ('security.dual_custody_enforced', true, '{"phase": 8, "description": "Mandatory 2-operator approval for sensitive administrative actions"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled, rules = EXCLUDED.rules;

-- Privileged role elevation logs (PAM JIT)
CREATE TABLE IF NOT EXISTS control.privileged_elevations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  elevated_role text NOT NULL,
  reason text NOT NULL,
  mfa_verified boolean NOT NULL DEFAULT false,
  granted_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked boolean NOT NULL DEFAULT false,
  revoked_at timestamptz
);

-- Data Subject Access & Erasure Requests (DSAR)
CREATE TABLE IF NOT EXISTS identity.dsar_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  request_type text NOT NULL, -- 'export', 'rectify', 'erase', 'revoke_consent'
  status text NOT NULL DEFAULT 'pending', -- 'pending', 'processing', 'completed', 'rejected'
  details text,
  completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Dual-custody (4-eyes principle) approval requests
CREATE TABLE IF NOT EXISTS control.dual_approvals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action_type text NOT NULL, -- 'bulk_user_export', 'permanent_index_purge', 'key_rotation'
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  requester_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE RESTRICT,
  approver_id uuid REFERENCES identity.accounts(id) ON DELETE RESTRICT,
  status text NOT NULL DEFAULT 'pending', -- 'pending', 'approved', 'rejected'
  approved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT check_different_operators CHECK (requester_id <> approver_id)
);

-- Immutable security audit log
CREATE TABLE IF NOT EXISTS control.security_audit_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  event_type text NOT NULL,
  client_ip text NOT NULL,
  severity text NOT NULL, -- 'INFO', 'WARN', 'CRITICAL'
  description text NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now()
);

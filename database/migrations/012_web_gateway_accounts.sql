-- Separate browser account boundary; existing identity data remains untouched.
-- FK order: accounts -> sessions/action_tokens/mail_connections.
CREATE SCHEMA webapp;
REVOKE ALL ON SCHEMA webapp FROM PUBLIC;
CREATE TABLE webapp.accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE CHECK (email = lower(email)),
  display_name text NOT NULL,
  password_hash text NOT NULL,
  locale text NOT NULL CHECK (locale IN ('ko','th','en')),
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE webapp.sessions (
  token_hash text PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX web_sessions_account ON webapp.sessions(account_id);
CREATE TABLE webapp.action_tokens (
  token_hash text PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  purpose text NOT NULL CHECK (purpose IN ('verify','reset')),
  expires_at timestamptz NOT NULL
);
CREATE INDEX web_tokens_account ON webapp.action_tokens(account_id);
CREATE TABLE webapp.rate_limits (
  key_hash text NOT NULL,
  bucket bigint NOT NULL,
  attempts integer NOT NULL DEFAULT 1,
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '2 hours'),
  PRIMARY KEY(key_hash, bucket)
);
CREATE TABLE webapp.mail_connections (
  account_id uuid PRIMARY KEY REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  token_ciphertext text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE webapp.upload_grants (
  account_id uuid NOT NULL REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  blob_id text NOT NULL,
  name text NOT NULL,
  mime_type text NOT NULL,
  size integer NOT NULL CHECK(size BETWEEN 0 AND 5242880),
  expires_at timestamptz NOT NULL,
  PRIMARY KEY(account_id, blob_id)
);
CREATE TABLE webapp.mail_send_requests (
  account_id uuid NOT NULL REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  email_id text NOT NULL,
  submission_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(account_id,email_id)
);
REVOKE ALL ON ALL TABLES IN SCHEMA webapp FROM PUBLIC;

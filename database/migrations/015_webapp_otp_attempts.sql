-- D-5: bind one-time verification codes to the requesting account.
-- A 6-digit OTP lives in a global 10^6 namespace, so the row must never be
-- reassigned to another account on hash collision (the previous
-- "on conflict do update set account_id" overwrote other accounts' rows).
-- The attempts counter bounds consume attempts against a live row; the
-- application deletes the row once the bound is reached. No FK change.
ALTER TABLE webapp.action_tokens ADD COLUMN attempts integer NOT NULL DEFAULT 0;
ALTER TABLE webapp.action_tokens ADD CONSTRAINT action_tokens_attempts_nonnegative CHECK (attempts >= 0);
COMMENT ON COLUMN webapp.action_tokens.attempts IS 'D-5: consume attempts against this row; row is deleted after 5 failed attempts';

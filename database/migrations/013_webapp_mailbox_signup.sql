-- Hosted-mailbox signup: an account is created together with username@MAIL_DOMAIN.
-- Only alters webapp.accounts (created in 012); no new FKs, so FK order is unchanged.
-- Existing external-email accounts keep username NULL and remain valid.
ALTER TABLE webapp.accounts
  ADD COLUMN username text,
  ADD COLUMN given_name text,
  ADD COLUMN family_name text,
  ADD COLUMN birth_date date,
  ADD COLUMN gender text,
  ADD COLUMN gender_custom text,
  ADD COLUMN pronoun text,
  ADD COLUMN recovery_email text,
  ADD COLUMN recovery_email_verified_at timestamptz,
  ADD COLUMN recovery_phone text;

ALTER TABLE webapp.accounts
  ADD CONSTRAINT accounts_username_format CHECK (username IS NULL OR (username ~ '^[a-z0-9][a-z0-9.]{4,28}[a-z0-9]$' AND username !~ '[.][.]')),
  ADD CONSTRAINT accounts_given_name_length CHECK (given_name IS NULL OR char_length(given_name) BETWEEN 1 AND 40),
  ADD CONSTRAINT accounts_family_name_length CHECK (family_name IS NULL OR char_length(family_name) <= 40),
  ADD CONSTRAINT accounts_gender_value CHECK (gender IS NULL OR gender IN ('female','male','unspecified','custom')),
  ADD CONSTRAINT accounts_gender_custom CHECK ((gender = 'custom') = (gender_custom IS NOT NULL) AND (gender_custom IS NULL OR char_length(gender_custom) BETWEEN 1 AND 40)),
  ADD CONSTRAINT accounts_pronoun_value CHECK (pronoun IS NULL OR pronoun IN ('female','male','other')),
  ADD CONSTRAINT accounts_recovery_email_format CHECK (recovery_email IS NULL OR (recovery_email = lower(recovery_email) AND char_length(recovery_email) <= 254)),
  ADD CONSTRAINT accounts_recovery_verified CHECK (recovery_email_verified_at IS NULL OR recovery_email IS NOT NULL),
  ADD CONSTRAINT accounts_recovery_phone_format CHECK (recovery_phone IS NULL OR recovery_phone ~ '^[+][1-9][0-9]{6,14}$');

CREATE UNIQUE INDEX accounts_username_unique ON webapp.accounts(username) WHERE username IS NOT NULL;

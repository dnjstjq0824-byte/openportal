-- Operator administration: flag privileged accounts without touching
-- existing auth/session/mail flows. Grants happen only via operator tooling.
ALTER TABLE webapp.accounts ADD COLUMN is_admin boolean NOT NULL DEFAULT false;
CREATE INDEX accounts_admins ON webapp.accounts(id) WHERE is_admin;

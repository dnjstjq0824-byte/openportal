-- C-9: user reports against search results (spam/malware/phishing/adult/
-- copyright/privacy/other) plus deletion requests. Review workflow stays
-- manual: no admin console exists yet (D-4 deny-by-default), so status moves
-- only via direct DB update until the review UI lands.
CREATE TABLE IF NOT EXISTS webapp.reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES webapp.accounts(id) ON DELETE CASCADE,
  document_id text NOT NULL CHECK (char_length(document_id) BETWEEN 1 AND 256),
  url text NOT NULL CHECK (char_length(url) BETWEEN 1 AND 2048),
  category text NOT NULL CHECK (category IN ('spam','malware','phishing','adult','copyright','privacy','other')),
  details text NOT NULL DEFAULT '' CHECK (char_length(details) <= 2000),
  status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','reviewing','resolved','rejected')),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS reports_account_idx ON webapp.reports(account_id, created_at DESC);

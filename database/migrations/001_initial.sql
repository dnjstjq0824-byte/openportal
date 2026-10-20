-- OpenPortal Search baseline relational schema (all planned product domains; future phases MUST migrate it).
-- PostgreSQL 16+ syntax; transactional metadata only. Not OpenSearch inverted index or raw content.
-- Not a complete production security policy: application authorization/RLS, backups, retention must be implemented.
BEGIN;
SET TIME ZONE 'UTC';
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS crawl;
CREATE SCHEMA IF NOT EXISTS catalog;
CREATE SCHEMA IF NOT EXISTS query;
CREATE SCHEMA IF NOT EXISTS portal;
CREATE SCHEMA IF NOT EXISTS ads;
CREATE SCHEMA IF NOT EXISTS publisher;
CREATE SCHEMA IF NOT EXISTS control;

-- =============== IDENTITY ===============
CREATE TABLE identity.accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text UNIQUE,
  display_name text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted')),
  locale text NOT NULL DEFAULT 'ko-KR',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);
CREATE TABLE identity.auth_identities (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  provider text NOT NULL,
  provider_subject text NOT NULL,
  password_hash text,
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(provider,provider_subject)
);
CREATE TABLE identity.sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz
);
CREATE TABLE identity.mfa_factors (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  factor_type text NOT NULL,
  secret_ciphertext bytea,
  enrolled_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);
CREATE TABLE identity.preferences (
  account_id uuid PRIMARY KEY REFERENCES identity.accounts(id) ON DELETE CASCADE,
  safe_search text NOT NULL DEFAULT 'moderate',
  theme text NOT NULL DEFAULT 'system',
  language text NOT NULL DEFAULT 'ko',
  personalized_search boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE identity.organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  org_type text NOT NULL,
  verified_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE identity.organization_members (
  organization_id uuid NOT NULL REFERENCES identity.organizations(id) ON DELETE CASCADE,
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  role text NOT NULL CHECK(role IN ('owner','admin','editor','viewer','billing')),
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(organization_id,account_id)
);
CREATE TABLE identity.roles (
  id text PRIMARY KEY,
  description text NOT NULL
);
CREATE TABLE identity.account_roles (
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  role_id text NOT NULL REFERENCES identity.roles(id),
  granted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(account_id,role_id)
);
CREATE TABLE identity.api_credentials (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid REFERENCES identity.organizations(id) ON DELETE CASCADE,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE CASCADE,
  key_prefix text NOT NULL,
  secret_hash bytea NOT NULL UNIQUE,
  scopes text[] NOT NULL DEFAULT '{}',
  revoked_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (organization_id IS NOT NULL OR account_id IS NOT NULL)
);
CREATE TABLE identity.consent_records (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  account_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  consent_type text NOT NULL,
  version text NOT NULL,
  granted boolean NOT NULL,
  recorded_at timestamptz NOT NULL DEFAULT now()
);

-- =============== CRAWL ===============
CREATE TABLE crawl.hosts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scheme text NOT NULL CHECK(scheme IN ('http','https')),
  hostname text NOT NULL,
  port integer NOT NULL DEFAULT 443 CHECK(port BETWEEN 1 AND 65535),
  fetch_enabled boolean NOT NULL DEFAULT true,
  crawl_delay_ms integer NOT NULL DEFAULT 1000 CHECK(crawl_delay_ms >= 0),
  next_allowed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(scheme,hostname,port)
);
CREATE TABLE crawl.robots_cache (
  host_id uuid PRIMARY KEY REFERENCES crawl.hosts(id) ON DELETE CASCADE,
  fetched_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  status_code integer,
  body_object_key text,
  body_sha256 text,
  policy_decision text NOT NULL DEFAULT 'unknown',
  parser_version text NOT NULL,
  CHECK(expires_at >= fetched_at)
);
CREATE TABLE crawl.sitemaps (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid NOT NULL REFERENCES crawl.hosts(id) ON DELETE CASCADE,
  url text NOT NULL UNIQUE,
  last_fetched_at timestamptz,
  error_count integer NOT NULL DEFAULT 0,
  next_fetch_at timestamptz,
  etag text,
  last_modified text
);
CREATE TABLE crawl.seed_sources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_type text NOT NULL,
  source_uri text NOT NULL,
  license text,
  enabled boolean NOT NULL DEFAULT true,
  ingestion_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE crawl.frontier (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid NOT NULL REFERENCES crawl.hosts(id),
  url text NOT NULL UNIQUE,
  url_hash text NOT NULL UNIQUE,
  priority integer NOT NULL DEFAULT 0,
  state text NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','leased','done','blocked','failed')),
  scheduled_at timestamptz NOT NULL DEFAULT now(),
  lease_owner text,
  lease_until timestamptz,
  retry_count integer NOT NULL DEFAULT 0,
  discovered_at timestamptz NOT NULL DEFAULT now(),
  last_error text
);
CREATE TABLE crawl.fetch_attempts (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  frontier_id uuid NOT NULL REFERENCES crawl.frontier(id) ON DELETE CASCADE,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  status_code integer,
  decision text NOT NULL DEFAULT 'pending',
  failure_code text,
  transferred_bytes bigint NOT NULL DEFAULT 0 CHECK(transferred_bytes >= 0),
  elapsed_ms integer
);
CREATE TABLE crawl.raw_objects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  frontier_id uuid NOT NULL REFERENCES crawl.frontier(id) ON DELETE CASCADE,
  object_key text NOT NULL UNIQUE,
  mime_type text NOT NULL,
  bytes bigint NOT NULL CHECK(bytes >= 0),
  checksum_sha256 text NOT NULL,
  captured_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);
CREATE TABLE crawl.discovered_urls (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  source_frontier_id uuid REFERENCES crawl.frontier(id) ON DELETE SET NULL,
  target_url text NOT NULL,
  anchor_text text,
  discovered_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE crawl.crawl_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id uuid REFERENCES crawl.hosts(id) ON DELETE CASCADE,
  rule_type text NOT NULL CHECK(rule_type IN ('deny','allow','rate_limit','quarantine')),
  pattern text NOT NULL,
  reason text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);
CREATE TABLE crawl.url_submissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  submitted_by uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  url text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz
);

-- =============== CATALOG ===============
CREATE TABLE catalog.documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  canonical_url text NOT NULL UNIQUE,
  canonical_hash text NOT NULL UNIQUE,
  host_id uuid REFERENCES crawl.hosts(id),
  title text NOT NULL DEFAULT '',
  description text,
  detected_language text,
  document_type text NOT NULL DEFAULT 'web',
  http_status integer,
  robots_indexable boolean NOT NULL DEFAULT false,
  content_safety text NOT NULL DEFAULT 'unreviewed',
  quality_score double precision,
  rank_score double precision,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_crawled_at timestamptz,
  indexed_at timestamptz,
  removed_at timestamptz
);
CREATE TABLE catalog.document_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  raw_object_id uuid REFERENCES crawl.raw_objects(id) ON DELETE SET NULL,
  content_hash text NOT NULL,
  extracted_object_key text,
  title text,
  description text,
  word_count integer NOT NULL DEFAULT 0,
  parser_version text NOT NULL,
  extracted_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(document_id,content_hash)
);
CREATE TABLE catalog.document_links (
  source_document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  target_document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  rel_nofollow boolean NOT NULL DEFAULT false,
  anchor text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(source_document_id,target_document_id)
);
CREATE TABLE catalog.canonical_aliases (
  alias_url text PRIMARY KEY,
  document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  reason text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE catalog.index_events (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK(operation IN ('upsert','delete')),
  state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','leased','done','dead')),
  payload_version integer NOT NULL DEFAULT 1,
  attempts integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz
);
CREATE TABLE catalog.index_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  index_alias text NOT NULL,
  physical_index text NOT NULL,
  status text NOT NULL DEFAULT 'creating',
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  items_upserted bigint NOT NULL DEFAULT 0,
  items_deleted bigint NOT NULL DEFAULT 0,
  error_message text
);
CREATE TABLE catalog.embedding_metadata (
  document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  model_version text NOT NULL,
  vector_store_key text NOT NULL,
  embedding_dimensions integer NOT NULL CHECK(embedding_dimensions > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(document_id,model_version)
);
CREATE TABLE catalog.index_policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  scope_pattern text NOT NULL,
  policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled boolean NOT NULL DEFAULT true,
  changed_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE catalog.removal_tombstones (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  canonical_hash text NOT NULL UNIQUE,
  reason text NOT NULL,
  requested_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  legal_hold boolean NOT NULL DEFAULT false
);

-- =============== QUERY ===============
CREATE TABLE query.requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  query_hash text,
  query_normalized text,
  locale text NOT NULL DEFAULT 'ko-KR',
  vertical text NOT NULL DEFAULT 'web',
  safe_search text NOT NULL DEFAULT 'moderate',
  experiment_key text,
  latency_ms integer,
  result_count integer,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);
CREATE TABLE query.impressions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  request_id uuid NOT NULL REFERENCES query.requests(id) ON DELETE CASCADE,
  document_id uuid REFERENCES catalog.documents(id) ON DELETE SET NULL,
  position integer NOT NULL CHECK(position>=1),
  surface text NOT NULL DEFAULT 'organic',
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE query.clicks (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  request_id uuid NOT NULL REFERENCES query.requests(id) ON DELETE CASCADE,
  document_id uuid REFERENCES catalog.documents(id) ON DELETE SET NULL,
  position integer NOT NULL CHECK(position>=1),
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE query.suggestions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  phrase text NOT NULL,
  locale text NOT NULL,
  source text NOT NULL,
  normalized_phrase text NOT NULL,
  weight double precision NOT NULL DEFAULT 0,
  blocked boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(locale,normalized_phrase)
);
CREATE TABLE query.synonym_sets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  locale text NOT NULL,
  name text NOT NULL,
  version integer NOT NULL DEFAULT 1,
  enabled boolean NOT NULL DEFAULT true,
  UNIQUE(locale,name)
);
CREATE TABLE query.synonyms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  set_id uuid NOT NULL REFERENCES query.synonym_sets(id) ON DELETE CASCADE,
  source_term text NOT NULL,
  target_term text NOT NULL,
  weight double precision NOT NULL DEFAULT 1,
  UNIQUE(set_id,source_term,target_term)
);
CREATE TABLE query.rewrites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_pattern text NOT NULL,
  replacement text NOT NULL,
  locale text NOT NULL,
  priority integer NOT NULL DEFAULT 0,
  enabled boolean NOT NULL DEFAULT true
);
CREATE TABLE query.ranking_profiles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  version integer NOT NULL,
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  active boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE query.experiments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL UNIQUE,
  status text NOT NULL DEFAULT 'draft',
  variants jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz,
  ended_at timestamptz
);
CREATE TABLE query.quality_judgments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  query_text text NOT NULL,
  document_id uuid NOT NULL REFERENCES catalog.documents(id) ON DELETE CASCADE,
  relevance integer NOT NULL CHECK(relevance BETWEEN 0 AND 4),
  reviewer_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  judged_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(query_text,document_id,reviewer_id)
);
CREATE TABLE query.evaluation_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ranking_profile_id uuid REFERENCES query.ranking_profiles(id),
  dataset_version text NOT NULL,
  metrics jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  pass boolean
);
CREATE TABLE query.zero_result_queries (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  query_hash text NOT NULL,
  locale text NOT NULL,
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  occurrences bigint NOT NULL DEFAULT 1
);

-- =============== PORTAL ===============
CREATE TABLE portal.media_assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  object_key text NOT NULL UNIQUE,
  mime_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK(size_bytes>=0),
  license text,
  attribution text,
  safety_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE portal.images (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid REFERENCES catalog.documents(id) ON DELETE CASCADE,
  asset_id uuid REFERENCES portal.media_assets(id) ON DELETE SET NULL,
  source_url text NOT NULL,
  alt_text text,
  width integer,
  height integer,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE portal.videos (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid REFERENCES catalog.documents(id) ON DELETE CASCADE,
  provider text NOT NULL,
  external_id text,
  video_url text NOT NULL,
  title text NOT NULL,
  duration_seconds integer,
  published_at timestamptz
);
CREATE TABLE portal.news_publishers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  website text NOT NULL UNIQUE,
  feed_url text,
  licensed boolean NOT NULL DEFAULT false,
  published_policy jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE portal.news_articles (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  document_id uuid REFERENCES catalog.documents(id) ON DELETE SET NULL,
  publisher_id uuid REFERENCES portal.news_publishers(id) ON DELETE SET NULL,
  url text NOT NULL UNIQUE,
  headline text NOT NULL,
  published_at timestamptz,
  fetched_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE portal.places (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name text NOT NULL,
  address text,
  latitude numeric(9,6),
  longitude numeric(9,6),
  business_url text,
  source_license text,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK(latitude BETWEEN -90 AND 90),
  CHECK(longitude BETWEEN -180 AND 180)
);
CREATE TABLE portal.place_reviews (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  place_id uuid NOT NULL REFERENCES portal.places(id) ON DELETE CASCADE,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  rating integer NOT NULL CHECK(rating BETWEEN 1 AND 5),
  body text,
  moderation_status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE portal.product_offers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_key text NOT NULL,
  merchant_org_id uuid REFERENCES identity.organizations(id) ON DELETE SET NULL,
  title text NOT NULL,
  landing_url text NOT NULL,
  price numeric(18,2) CHECK(price>=0),
  currency char(3) NOT NULL DEFAULT 'KRW',
  availability text NOT NULL DEFAULT 'unknown',
  source_updated_at timestamptz
);
CREATE TABLE portal.knowledge_entities (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_key text NOT NULL UNIQUE,
  name text NOT NULL,
  entity_type text NOT NULL,
  summary text,
  attribution text,
  data jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE portal.modules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  module_key text NOT NULL UNIQUE,
  display_name text NOT NULL,
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled boolean NOT NULL DEFAULT false,
  sort_order integer NOT NULL DEFAULT 0
);
CREATE TABLE portal.module_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  module_id uuid NOT NULL REFERENCES portal.modules(id) ON DELETE CASCADE,
  title text NOT NULL,
  target_url text,
  asset_id uuid REFERENCES portal.media_assets(id) ON DELETE SET NULL,
  sort_order integer NOT NULL DEFAULT 0,
  publish_at timestamptz,
  unpublish_at timestamptz
);

-- =============== ADS ===============
CREATE TABLE ads.accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL UNIQUE REFERENCES identity.organizations(id) ON DELETE CASCADE,
  state text NOT NULL DEFAULT 'pending',
  billing_currency char(3) NOT NULL DEFAULT 'KRW',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.campaigns (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid NOT NULL REFERENCES ads.accounts(id) ON DELETE CASCADE,
  name text NOT NULL,
  state text NOT NULL DEFAULT 'draft',
  daily_budget_minor bigint NOT NULL DEFAULT 0 CHECK(daily_budget_minor>=0),
  total_budget_minor bigint CHECK(total_budget_minor>=0),
  starts_at timestamptz,
  ends_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK(ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);
CREATE TABLE ads.ad_groups (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id uuid NOT NULL REFERENCES ads.campaigns(id) ON DELETE CASCADE,
  name text NOT NULL,
  state text NOT NULL DEFAULT 'draft',
  max_cpc_minor bigint CHECK(max_cpc_minor>=0)
);
CREATE TABLE ads.keywords (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_group_id uuid NOT NULL REFERENCES ads.ad_groups(id) ON DELETE CASCADE,
  phrase text NOT NULL,
  match_type text NOT NULL CHECK(match_type IN ('exact','phrase','broad','negative')),
  state text NOT NULL DEFAULT 'active',
  max_cpc_minor bigint CHECK(max_cpc_minor>=0)
);
CREATE TABLE ads.creatives (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_group_id uuid NOT NULL REFERENCES ads.ad_groups(id) ON DELETE CASCADE,
  headline text NOT NULL,
  description text NOT NULL,
  display_url text,
  destination_url text NOT NULL,
  review_state text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.targeting_rules (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_group_id uuid NOT NULL REFERENCES ads.ad_groups(id) ON DELETE CASCADE,
  rule_type text NOT NULL,
  definition jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE TABLE ads.auctions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id uuid REFERENCES query.requests(id) ON DELETE SET NULL,
  slot text NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  model_version text NOT NULL
);
CREATE TABLE ads.auction_candidates (
  auction_id uuid NOT NULL REFERENCES ads.auctions(id) ON DELETE CASCADE,
  creative_id uuid NOT NULL REFERENCES ads.creatives(id),
  bid_minor bigint NOT NULL CHECK(bid_minor>=0),
  quality_score double precision,
  position integer,
  PRIMARY KEY(auction_id,creative_id)
);
CREATE TABLE ads.impressions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  auction_id uuid REFERENCES ads.auctions(id) ON DELETE SET NULL,
  creative_id uuid NOT NULL REFERENCES ads.creatives(id),
  position integer NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.clicks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  impression_id uuid NOT NULL REFERENCES ads.impressions(id) ON DELETE CASCADE,
  fraud_status text NOT NULL DEFAULT 'unreviewed',
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.conversions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  click_id uuid REFERENCES ads.clicks(id) ON DELETE SET NULL,
  conversion_type text NOT NULL,
  value_minor bigint CHECK(value_minor>=0),
  currency char(3) NOT NULL DEFAULT 'KRW',
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.billing_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  ad_account_id uuid NOT NULL UNIQUE REFERENCES ads.accounts(id) ON DELETE CASCADE,
  provider_customer_ref text,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE ads.invoices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  billing_account_id uuid NOT NULL REFERENCES ads.billing_accounts(id),
  invoice_number text NOT NULL UNIQUE,
  currency char(3) NOT NULL,
  total_minor bigint NOT NULL DEFAULT 0 CHECK(total_minor>=0),
  state text NOT NULL DEFAULT 'draft',
  issued_at timestamptz,
  due_at timestamptz
);
CREATE TABLE ads.invoice_lines (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id uuid NOT NULL REFERENCES ads.invoices(id) ON DELETE CASCADE,
  description text NOT NULL,
  quantity numeric(18,3) NOT NULL DEFAULT 1 CHECK(quantity>=0),
  unit_amount_minor bigint NOT NULL DEFAULT 0 CHECK(unit_amount_minor>=0),
  total_minor bigint NOT NULL CHECK(total_minor>=0)
);
CREATE TABLE ads.payments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  invoice_id uuid REFERENCES ads.invoices(id) ON DELETE SET NULL,
  provider text NOT NULL,
  provider_payment_id text UNIQUE,
  amount_minor bigint NOT NULL CHECK(amount_minor>=0),
  currency char(3) NOT NULL,
  state text NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now()
);

-- =============== PUBLISHER ===============
CREATE TABLE publisher.sites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES identity.accounts(id) ON DELETE CASCADE,
  origin text NOT NULL UNIQUE,
  verification_state text NOT NULL DEFAULT 'pending',
  added_at timestamptz NOT NULL DEFAULT now(),
  verified_at timestamptz
);
CREATE TABLE publisher.verification_challenges (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  site_id uuid NOT NULL REFERENCES publisher.sites(id) ON DELETE CASCADE,
  method text NOT NULL,
  token_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  verified_at timestamptz
);
CREATE TABLE publisher.sitemap_submissions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  site_id uuid NOT NULL REFERENCES publisher.sites(id) ON DELETE CASCADE,
  sitemap_url text NOT NULL,
  submitted_at timestamptz NOT NULL DEFAULT now(),
  last_processed_at timestamptz,
  state text NOT NULL DEFAULT 'queued',
  UNIQUE(site_id,sitemap_url)
);
CREATE TABLE publisher.url_inspections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  site_id uuid NOT NULL REFERENCES publisher.sites(id) ON DELETE CASCADE,
  inspected_url text NOT NULL,
  result jsonb NOT NULL DEFAULT '{}'::jsonb,
  inspected_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE publisher.site_stats_daily (
  site_id uuid NOT NULL REFERENCES publisher.sites(id) ON DELETE CASCADE,
  stat_date date NOT NULL,
  impressions bigint NOT NULL DEFAULT 0,
  clicks bigint NOT NULL DEFAULT 0,
  indexed_pages bigint NOT NULL DEFAULT 0,
  PRIMARY KEY(site_id,stat_date)
);
CREATE TABLE publisher.site_issues (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  site_id uuid NOT NULL REFERENCES publisher.sites(id) ON DELETE CASCADE,
  issue_type text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  state text NOT NULL DEFAULT 'open',
  detected_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);
CREATE TABLE publisher.removal_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  site_id uuid REFERENCES publisher.sites(id) ON DELETE SET NULL,
  submitted_by uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  url text NOT NULL,
  reason text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);
CREATE TABLE publisher.api_usage_daily (
  credential_id uuid NOT NULL REFERENCES identity.api_credentials(id) ON DELETE CASCADE,
  usage_date date NOT NULL,
  endpoint text NOT NULL,
  request_count bigint NOT NULL DEFAULT 0,
  PRIMARY KEY(credential_id,usage_date,endpoint)
);

-- =============== CONTROL ===============
CREATE TABLE control.moderation_cases (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  subject_type text NOT NULL,
  subject_id text NOT NULL,
  reason text NOT NULL,
  status text NOT NULL DEFAULT 'open',
  assigned_to uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  opened_at timestamptz NOT NULL DEFAULT now(),
  closed_at timestamptz
);
CREATE TABLE control.abuse_events (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  event_type text NOT NULL,
  severity text NOT NULL,
  actor_hash text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE control.takedown_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  submitted_by uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  affected_url text NOT NULL,
  claim_type text NOT NULL,
  evidence_object_key text,
  status text NOT NULL DEFAULT 'pending',
  submitted_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);
CREATE TABLE control.privacy_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  request_type text NOT NULL CHECK(request_type IN ('access','erase','correct','export','object')),
  state text NOT NULL DEFAULT 'pending',
  requested_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);
CREATE TABLE control.retention_policies (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  dataset text NOT NULL UNIQUE,
  retention_days integer NOT NULL CHECK(retention_days>=0),
  law_basis text,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE control.audit_logs (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  actor_account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  action text NOT NULL,
  target_type text NOT NULL,
  target_id text,
  correlation_id text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE control.feature_flags (
  name text PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT false,
  rules jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE control.system_jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_type text NOT NULL,
  job_key text UNIQUE,
  state text NOT NULL DEFAULT 'queued',
  attempts integer NOT NULL DEFAULT 0,
  scheduled_at timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  error_code text
);
CREATE TABLE control.alerts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_type text NOT NULL,
  severity text NOT NULL,
  resource text,
  state text NOT NULL DEFAULT 'open',
  first_seen_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE control.service_incidents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL,
  severity text NOT NULL,
  state text NOT NULL DEFAULT 'investigating',
  started_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz,
  retrospective_object_key text
);

CREATE INDEX crawl_frontier_state_schedule_idx ON crawl.frontier(state, scheduled_at, priority DESC);
CREATE INDEX crawl_frontier_host_idx ON crawl.frontier(host_id,scheduled_at);
CREATE INDEX crawl_attempts_frontier_idx ON crawl.fetch_attempts(frontier_id,started_at DESC);
CREATE INDEX crawl_discovered_source_idx ON crawl.discovered_urls(source_frontier_id);
CREATE INDEX catalog_documents_visible_idx ON catalog.documents(robots_indexable,removed_at,detected_language);
CREATE INDEX catalog_versions_doc_idx ON catalog.document_versions(document_id,extracted_at DESC);
CREATE INDEX catalog_links_target_idx ON catalog.document_links(target_document_id);
CREATE INDEX catalog_outbox_idx ON catalog.index_events(state,created_at);
CREATE INDEX query_requests_occurred_idx ON query.requests(occurred_at);
CREATE INDEX query_impressions_request_idx ON query.impressions(request_id);
CREATE INDEX query_clicks_request_idx ON query.clicks(request_id);
CREATE INDEX query_suggest_locale_phrase_idx ON query.suggestions(locale,normalized_phrase);
CREATE INDEX query_judgments_text_idx ON query.quality_judgments(query_text);
CREATE INDEX portal_news_published_idx ON portal.news_articles(published_at DESC);
CREATE INDEX portal_offers_product_idx ON portal.product_offers(product_key);
CREATE INDEX ads_campaign_account_idx ON ads.campaigns(account_id,state);
CREATE INDEX ads_keywords_group_idx ON ads.keywords(ad_group_id);
CREATE INDEX ads_impression_date_idx ON ads.impressions(occurred_at);
CREATE INDEX ads_clicks_date_idx ON ads.clicks(occurred_at);
CREATE INDEX control_audit_occured_idx ON control.audit_logs(occurred_at);
CREATE INDEX control_privacy_state_idx ON control.privacy_requests(state,requested_at);
CREATE INDEX publisher_sites_owner_idx ON publisher.sites(owner_id);
CREATE INDEX identity_sessions_account_idx ON identity.sessions(account_id,expires_at);
CREATE INDEX identity_org_member_account_idx ON identity.organization_members(account_id);
INSERT INTO identity.roles (id,description) VALUES ('user','Standard account'),('staff','Internal staff'),('system_admin','Platform superadmin') ON CONFLICT DO NOTHING;
COMMIT;

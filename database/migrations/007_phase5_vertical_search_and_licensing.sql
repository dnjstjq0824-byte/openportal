-- 007_phase5_vertical_search_and_licensing.sql
-- Phase 5: Image, Video, News, Map, and Shopping operational tables and disabled feature flags

-- Feature flags for Phase 5 vertical gates
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('vertical.image_search_pipeline', false, '{"phase": 5, "description": "Image licensing, feature extraction and visual search pipeline"}'::jsonb),
  ('vertical.video_search_pipeline', false, '{"phase": 5, "description": "Video metadata, authorized embed paths and duration filtering"}'::jsonb),
  ('vertical.news_search_pipeline', false, '{"phase": 5, "description": "News RSS ingestion, publisher licensing and breaking news detection"}'::jsonb),
  ('vertical.maps_poi_pipeline', false, '{"phase": 5, "description": "Geographic POI search, ODbL map data and place verification"}'::jsonb),
  ('vertical.shopping_feed_pipeline', false, '{"phase": 5, "description": "Product feed ingestion, merchant disclosure and price validation"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled, rules = EXCLUDED.rules;

-- Additional Phase 5 operational tables for deduplication, corrections, and reports
CREATE TABLE IF NOT EXISTS portal.image_clusters (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  representative_image_id uuid REFERENCES portal.images(id) ON DELETE CASCADE,
  dhash bigint NOT NULL,
  cluster_size integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS portal.poi_corrections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  place_id uuid NOT NULL REFERENCES portal.places(id) ON DELETE CASCADE,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  field_name text NOT NULL,
  old_value text,
  new_value text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS portal.shopping_reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  offer_id uuid NOT NULL REFERENCES portal.product_offers(id) ON DELETE CASCADE,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  report_type text NOT NULL,
  description text,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Do not seed publishers as licensed or label example coordinates as ODbL data.
-- Source ingestion requires independently verified rights and provenance.
-- All Phase 5 feature flags remain disabled until the runtime and rights gates pass.

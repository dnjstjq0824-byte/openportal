-- ========================================================================
-- OpenPortal Search — Migration 006 (Phase 4 Ranking Quality & Safety)
-- ========================================================================

-- 1. Seed baseline ranking profiles
INSERT INTO query.ranking_profiles (name, version, config, active)
VALUES
  ('bm25_baseline_v1', 1, '{"weights": {"title": 3.5, "headings": 2.0, "anchor_texts": 1.8, "description": 1.5, "body": 1.0}}'::jsonb, true),
  ('rrf_hybrid_candidate_v1', 1, '{"weights": {"lexical_k": 60, "semantic_k": 60, "rrf_boost": 1.0}}'::jsonb, false)
ON CONFLICT (name) DO UPDATE
  SET config = EXCLUDED.config,
      active = EXCLUDED.active;

-- 2. Seed default A/B experiment for ranking quality
INSERT INTO query.experiments (name, status, variants, started_at)
VALUES
  ('ranking_bm25_vs_rrf', 'running', '{"control": "bm25_baseline_v1", "treatment": "rrf_hybrid_candidate_v1", "split_percent": 50}'::jsonb, now())
ON CONFLICT (name) DO UPDATE
  SET status = EXCLUDED.status,
      variants = EXCLUDED.variants;

-- 3. Feature flags for Phase 4 Ranking Quality, SafeSearch & Abuse Defense
INSERT INTO control.feature_flags (name, enabled, rules)
VALUES
  ('ranking.intent_classification_enabled', true, '{"description": "Classify informational, navigational, transactional and local queries"}'::jsonb),
  ('ranking.safesearch_filter_enabled', true, '{"description": "Strict, moderate, and off safe search content filters"}'::jsonb),
  ('ranking.abuse_detector_enabled', true, '{"description": "Detect keyword manipulation, bot bursts, and spam injection"}'::jsonb),
  ('ranking.rrf_hybrid_enabled', true, '{"description": "Reciprocal Rank Fusion lexical/semantic hybrid retrieval"}'::jsonb)
ON CONFLICT (name) DO UPDATE
  SET enabled = EXCLUDED.enabled,
      rules = EXCLUDED.rules,
      updated_at = now();

-- 4. Speed up quality judgments lookups
CREATE INDEX IF NOT EXISTS quality_judgments_query_text_idx
  ON query.quality_judgments (query_text);


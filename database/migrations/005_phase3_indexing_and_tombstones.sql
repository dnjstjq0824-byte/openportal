-- ========================================================================
-- OpenPortal Search — Migration 005 (Phase 3 Indexing, Dedup & Tombstones)
-- ========================================================================

-- 1. Extend catalog.documents for SimHash deduplication and tombstone reasons
ALTER TABLE catalog.documents
  ADD COLUMN IF NOT EXISTS simhash text,
  ADD COLUMN IF NOT EXISTS tombstone_reason text,
  ADD COLUMN IF NOT EXISTS is_representative boolean NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS documents_simhash_idx
  ON catalog.documents (simhash)
  WHERE simhash IS NOT NULL;

CREATE INDEX IF NOT EXISTS documents_removed_at_idx
  ON catalog.documents (removed_at)
  WHERE removed_at IS NOT NULL;

-- 2. Index for transactional outbox queue processing (FIFO pending events)
CREATE INDEX IF NOT EXISTS index_events_state_created_idx
  ON catalog.index_events (state, created_at)
  WHERE state = 'pending';

-- 3. Feature flags for Phase 3 indexing capabilities
INSERT INTO control.feature_flags (name, enabled, rules)
VALUES
  ('indexer.nori_enabled', true, '{"description": "Official OpenSearch analysis-nori morphological analyzer for Korean", "decompound_mode": "mixed"}'::jsonb),
  ('indexer.blue_green_enabled', true, '{"description": "Blue-green zero-downtime index alias switching and rollback", "active_alias": "documents_search"}'::jsonb),
  ('indexer.tombstone_purge_enabled', true, '{"description": "Automatic propagation of deletion tombstones to inverted index and cache", "purge_delay_hours": 24}'::jsonb)
ON CONFLICT (name) DO UPDATE
  SET enabled = EXCLUDED.enabled,
      rules = EXCLUDED.rules,
      updated_at = now();


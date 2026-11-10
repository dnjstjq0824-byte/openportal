-- C-3: make catalog.index_events actually usable as the index outbox.
-- The baseline defined document_id as UUID FK to catalog.documents, but the
-- pipeline issues string document IDs (crawl-<hash>, fixture-NNN) and nothing
-- writes catalog.documents. Align the table with the real pipeline instead of
-- inventing a parallel outbox table. No other object references this table.
ALTER TABLE catalog.index_events DROP CONSTRAINT IF EXISTS index_events_document_id_fkey;
ALTER TABLE catalog.index_events ALTER COLUMN document_id TYPE text USING document_id::text;
-- Lease + payload columns required by the relay worker.
ALTER TABLE catalog.index_events ADD COLUMN leased_at timestamptz;
ALTER TABLE catalog.index_events ADD COLUMN payload jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS index_events_state_created_idx
  ON catalog.index_events (state, created_at);
COMMENT ON TABLE catalog.index_events IS 'C-3: durable index outbox relayed to OpenSearch by the indexer relay worker';

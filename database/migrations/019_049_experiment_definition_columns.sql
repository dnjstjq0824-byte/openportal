-- 049 experiment persistence (G020 follow-up). Extends the 001
-- query.experiments baseline with the ExperimentDef fields used by
-- backend/internal/ranking ExperimentPlatform. All additions are nullable so
-- existing rows migrate safely; application code validates ranges.
ALTER TABLE query.experiments
  ADD COLUMN IF NOT EXISTS layer text,
  ADD COLUMN IF NOT EXISTS salt text,
  ADD COLUMN IF NOT EXISTS split_percent integer,
  ADD COLUMN IF NOT EXISTS control_profile text,
  ADD COLUMN IF NOT EXISTS treatment_profile text,
  ADD COLUMN IF NOT EXISTS max_error_rate_threshold double precision,
  ADD COLUMN IF NOT EXISTS max_p99_latency_threshold double precision,
  ADD COLUMN IF NOT EXISTS min_sample_size integer,
  ADD COLUMN IF NOT EXISTS rollback_reason text,
  ADD COLUMN IF NOT EXISTS rolled_back_at timestamptz;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'experiments_split_percent_check'
  ) THEN
    ALTER TABLE query.experiments
      ADD CONSTRAINT experiments_split_percent_check
      CHECK (split_percent IS NULL OR (split_percent >= 0 AND split_percent <= 100));
  END IF;
END $$;
CREATE INDEX IF NOT EXISTS experiments_status_idx ON query.experiments(status);
-- Backfill the 006 seed row from its own variants JSONB (idempotent). The seed
-- predates the 049 columns; values below are read from the row itself, except
-- layer 'ranking', which is the schema domain of query.experiments.
UPDATE query.experiments
SET layer = 'ranking',
    split_percent = COALESCE((variants ->> 'split_percent')::integer, 50),
    control_profile = COALESCE(variants ->> 'control', control_profile),
    treatment_profile = COALESCE(variants ->> 'treatment', treatment_profile),
    min_sample_size = COALESCE(min_sample_size, 20)
WHERE layer IS NULL;
COMMENT ON TABLE query.experiments IS 'A/B experiment definitions (049). name carries the platform experiment ID; see ranking.ExperimentDef for field semantics.';

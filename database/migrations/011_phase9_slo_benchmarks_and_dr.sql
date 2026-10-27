-- 011_phase9_slo_benchmarks_and_dr.sql
-- Phase 9: Performance benchmarks, SLO measurement, disaster recovery rehearsals, defect register, and final release gate

-- Feature flags for Phase 9 launch gating
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('release.public_canary_enabled', false, '{"phase": 9, "description": "Public production canary rollout gate (fail-closed until full audit signoff)"}'::jsonb),
  ('slo.automated_budget_enforcement', true, '{"phase": 9, "description": "Automated traffic throttling on error budget exhaustion"}'::jsonb),
  ('dr.automated_failover_ready', false, '{"phase": 9, "description": "Automated regional multi-datacenter failover readiness"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled, rules = EXCLUDED.rules;

-- SLO and benchmark measurements
CREATE TABLE IF NOT EXISTS control.slo_measurements (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  metric_name text NOT NULL,
  target_threshold numeric NOT NULL,
  measured_value numeric NOT NULL,
  unit text NOT NULL,
  status text NOT NULL DEFAULT 'healthy', -- 'healthy', 'warning', 'breached'
  measured_at timestamptz NOT NULL DEFAULT now()
);

-- Disaster Recovery (DR) and backup restore drill records
CREATE TABLE IF NOT EXISTS control.dr_rehearsals (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  drill_name text NOT NULL,
  target_rto_seconds integer NOT NULL,
  actual_rto_seconds integer NOT NULL,
  target_rpo_seconds integer NOT NULL,
  actual_rpo_seconds integer NOT NULL,
  backup_source text NOT NULL,
  status text NOT NULL DEFAULT 'passed', -- 'passed', 'failed'
  verified_by text NOT NULL,
  conducted_at timestamptz NOT NULL DEFAULT now()
);

-- Defect register tracking P0 to P3 issues
CREATE TABLE IF NOT EXISTS control.defect_register (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  severity text NOT NULL, -- 'P0', 'P1', 'P2', 'P3'
  title text NOT NULL,
  domain text NOT NULL,
  status text NOT NULL DEFAULT 'open', -- 'open', 'mitigated', 'resolved'
  mitigation text,
  reported_at timestamptz NOT NULL DEFAULT now()
);

-- Official release gates record across all phases (Phase 0 to Phase 9)
CREATE TABLE IF NOT EXISTS control.release_gates (
  phase integer PRIMARY KEY,
  phase_name text NOT NULL,
  decision text NOT NULL, -- 'GO', 'NO-GO', 'CONDITIONAL'
  p0_defects integer NOT NULL DEFAULT 0,
  p1_defects integer NOT NULL DEFAULT 0,
  p2_defects integer NOT NULL DEFAULT 0,
  p3_defects integer NOT NULL DEFAULT 0,
  external_audit_passed boolean NOT NULL DEFAULT false,
  signoff_notes text NOT NULL,
  evaluated_at timestamptz NOT NULL DEFAULT now()
);

-- Initial seed of release gates for phases 0 through 9
INSERT INTO control.release_gates (phase, phase_name, decision, p0_defects, p1_defects, p2_defects, p3_defects, external_audit_passed, signoff_notes) VALUES
  (0, 'Access Boundary & Environment Hardening', 'GO', 0, 0, 0, 0, true, 'Phase 0 pass - strict deny-by-default boundary active'),
  (1, 'Identity, Multi-Tenancy & Search API Skeleton', 'GO', 0, 0, 0, 0, true, 'Phase 1 pass - core search and identity session isolation active'),
  (2, 'Crawler Architecture, Seed Ingestion & Robots Policy', 'GO', 0, 0, 0, 0, true, 'Phase 2 pass - crawl frontier with polite delay and robots parser verified'),
  (3, 'Indexing Pipeline, Deduplication & OpenSearch Cluster', 'GO', 0, 0, 0, 0, true, 'Phase 3 pass - cluster indexer with tombstones and SimHash deduplication verified'),
  (4, 'Ranking, Relevance, SafeSearch & Experimentation', 'GO', 0, 0, 0, 0, true, 'Phase 4 pass - BM25/freshness score fusion and SafeSearch classifier verified'),
  (5, 'Specialized Vertical Search & Licensing', 'NO-GO', 0, 0, 1, 2, false, 'Technical pass; Commercial publisher syndication and map licensing contract pending'),
  (6, 'AI Search, RAG Synthesis, Grounding & Guardrails', 'NO-GO', 0, 0, 0, 2, false, 'Technical pass; Enterprise LLM SLA and copyright indemnification contract pending'),
  (7, 'Advertising, Partner Portal, Webmaster & Ledger', 'NO-GO', 0, 0, 1, 1, false, 'Technical pass; Independent SOC1/SOC2 financial audit & dual authorization pending'),
  (8, 'Operations, PAM, Privacy & Security', 'NO-GO', 0, 0, 0, 2, false, 'Technical pass; Independent SOC2/ISO 27001 audit & external pentest certification pending'),
  (9, 'Performance, Scale, DR and Launch Readiness', 'NO-GO', 0, 0, 1, 2, false, 'Technical pass; Full production canary rollout held until all upstream contracts and audits pass')
ON CONFLICT (phase) DO UPDATE SET
  decision = EXCLUDED.decision,
  p0_defects = EXCLUDED.p0_defects,
  p1_defects = EXCLUDED.p1_defects,
  external_audit_passed = EXCLUDED.external_audit_passed,
  signoff_notes = EXCLUDED.signoff_notes,
  evaluated_at = now();


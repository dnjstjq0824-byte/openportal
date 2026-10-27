-- 008_phase6_ai_knowledge_and_guardrails.sql
-- Phase 6: AI Knowledge, RAG Synthesis, Grounding, and Sensitive Domain Guardrails

-- Feature flags for Phase 6 AI gates
INSERT INTO control.feature_flags (name, enabled, rules) VALUES
  ('ai.rag_overview_pipeline', false, '{"phase": 6, "description": "AI RAG synthesized answers, citation verification, and YMYL guardrails"}'::jsonb),
  ('ai.ymyl_strict_enforcement', true, '{"phase": 6, "description": "Mandatory disclaimer and refusal policy for medical, legal, and financial queries"}'::jsonb)
ON CONFLICT (name) DO UPDATE SET enabled = EXCLUDED.enabled, rules = EXCLUDED.rules;

-- Table for caching grounded AI synthesized summaries
CREATE TABLE IF NOT EXISTS portal.ai_query_caches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  query_hash text NOT NULL UNIQUE,
  normalized_query text NOT NULL,
  model_version text NOT NULL,
  summary_text text NOT NULL,
  citations jsonb NOT NULL DEFAULT '[]'::jsonb,
  grounding_score double precision NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

-- Table for user feedback on AI responses (hallucination reports, helpfulness)
CREATE TABLE IF NOT EXISTS portal.ai_feedback (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  query_hash text NOT NULL,
  account_id uuid REFERENCES identity.accounts(id) ON DELETE SET NULL,
  is_helpful boolean NOT NULL,
  feedback_category text NOT NULL, -- helpful, hallucination, unsupported_claim, unsafe
  user_comment text,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table for token consumption and budget limits
CREATE TABLE IF NOT EXISTS portal.ai_budget_ledgers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  time_bucket date NOT NULL DEFAULT CURRENT_DATE,
  total_prompt_tokens bigint NOT NULL DEFAULT 0,
  total_completion_tokens bigint NOT NULL DEFAULT 0,
  total_cost_usd numeric(12, 6) NOT NULL DEFAULT 0.000000,
  is_budget_exceeded boolean NOT NULL DEFAULT false,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(time_bucket)
);

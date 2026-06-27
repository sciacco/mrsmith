-- Binocolo M&A strategy V2 scaffolding in the centralized LLM registry.
-- Target database: Anisetta PostgreSQL (mrsmith + binocolo schemas).
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.
--
-- Seeds only:
--   * mrsmith.llm_model / mrsmith.llm_prompt for ma_strategy_intent
--   * mrsmith.llm_model / mrsmith.llm_prompt for ma_strategy_ateco
--   * binocolo.ma_parameter kill-switch ma_strategy_pipeline, default v2

BEGIN;

-- Fail loudly if the provider is missing. The registry seed in migration 047
-- creates this row; without it ResolveModel would silently fall through.
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'OpenRouter') THEN
    RAISE EXCEPTION 'Provider "OpenRouter" not found in mrsmith.llm_provider; apply migration 047 or adjust this migration to match your registry';
  END IF;
END $$;

-- Insert/update model bindings as non-default first. The default is assigned in
-- a second step after clearing competing defaults, which keeps the partial
-- unique index llm_model_one_default_per_app_scope satisfied on every run.
INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  v.scope,
  p.id,
  'OpenAI: GPT-5.5',
  'openai/gpt-5.5',
  v.params,
  false,
  true,
  false
FROM (
  VALUES
    ('ma_strategy_intent', '{"temperature":0,"max_tokens":2400}'::jsonb),
    ('ma_strategy_ateco',  '{"temperature":0,"max_tokens":1600}'::jsonb)
) AS v(scope, params)
JOIN mrsmith.llm_provider p ON p.name = 'OpenRouter'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id        = EXCLUDED.provider_id,
    name               = EXCLUDED.name,
    params             = EXCLUDED.params,
    supports_tools     = EXCLUDED.supports_tools,
    supports_json_mode = EXCLUDED.supports_json_mode,
    is_default         = EXCLUDED.is_default;

UPDATE mrsmith.llm_model
SET is_default = false
WHERE app = 'binocolo'
  AND scope IN ('ma_strategy_intent', 'ma_strategy_ateco')
  AND model <> 'openai/gpt-5.5'
  AND is_default;

UPDATE mrsmith.llm_model
SET is_default = true
WHERE app = 'binocolo'
  AND scope IN ('ma_strategy_intent', 'ma_strategy_ateco')
  AND model = 'openai/gpt-5.5';

-- Insert/update prompts as non-default first, then set the seeded prompt as the
-- only default per (app, scope) for the same partial-index reason.
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES
  (
    'binocolo',
    'ma_strategy_intent',
    'M&A strategy intent extractor v1',
    $prompt$You extract structured intent for the Binocolo Target M&A strategy pipeline.

Return only one valid JSON object. Do not include markdown, comments, or prose. Do not use tools. Do not search for ATECO codes, provinces, companies, or market data.

Output shape:
{
  "territory": {
    "includeRegions": [{"value": "region text", "sourceText": "exact user phrase"}],
    "includeProvinces": [{"value": "province text or code", "sourceText": "exact user phrase"}],
    "excludeProvinces": [{"value": "province text or code", "sourceText": "exact user phrase"}]
  },
  "atecoExplicit": [{"code": "ATECO code from the request", "sourceText": "exact user phrase"}],
  "sectors": {
    "include": [{"text": "sector text", "sourceText": "exact user phrase"}],
    "exclude": [{"text": "excluded sector text", "sourceText": "exact user phrase"}]
  },
  "turnover": {"min": null, "max": null, "around": null, "sourceText": ""},
  "employees": {"min": null, "max": null, "sourceText": ""},
  "legalForms": [{"text": "legal form text", "sourceText": "exact user phrase"}],
  "ownerAge": {"min": null, "max": null, "sourceText": ""},
  "status": {"value": "", "sourceText": ""},
  "revenuePerEmployeeMin": {"value": null, "sourceText": ""},
  "maxShareholders": {"value": null, "sourceText": ""},
  "constraints": [
    {"kind": "ebitda_margin|ocf_ratio|group_size|foreign_owned|other", "disposition": "deep|unsupported|missing_confirmation", "text": "constraint text", "sourceText": "exact user phrase"}
  ],
  "thesis": ""
}

Rules:
- Extract only constraints that are explicitly present in the user request.
- Every strong constraint that can affect search, exclusion, or scoring must carry sourceText: sector include/exclude, atecoExplicit, territory include/exclude, turnover, employees, legal form, ownerAge, non-default status, revenuePerEmployeeMin, and maxShareholders.
- sourceText must be a contiguous phrase from the user request after case and whitespace normalization. If you cannot cite the phrase, omit the constraint or set its value to null/empty.
- Keep territory structured. Do not collapse regions into provinces and do not resolve province names.
- Keep explicit ATECO codes separate from sector text. Do not invent ATECO codes from sector text.
- Preserve unsupported or later-stage constraints in constraints with the best disposition; do not silently drop them.
- Use null for unknown numbers and empty arrays for absent lists.$prompt$,
    false
  ),
  (
    'binocolo',
    'ma_strategy_ateco',
    'M&A strategy ATECO constrained rerank v1',
    $prompt$You rerank ATECO candidates for the Binocolo Target M&A strategy pipeline.

Return only one valid JSON object. Do not include markdown, comments, or prose. Do not use tools, web search, company search, or external knowledge.

Input will contain the original request, extracted sector include/exclude constraints with sourceText, and a candidates array with ATECO codes and labels.

Output shape:
{
  "selected": [
    {"code": "candidate code", "fit": "strong|weak", "reason": "short reason grounded in request text"}
  ],
  "excludedPrefixes": [
    {"prefix": "candidate code prefix", "reason": "short reason grounded in explicit exclusion text"}
  ],
  "missingCriteria": [
    {"text": "criterion that could not be mapped safely", "reason": "short reason"}
  ]
}

Rules:
- selected.code must be copied exactly from the input candidates. Never output a code that is not present in candidates.
- excludedPrefixes.prefix must be a prefix of at least one input candidate code and must be grounded in an explicit sector exclusion.
- Prefer the narrowest candidate codes that fit the requested sector. Use weak only when the request is broad or the candidate is adjacent but plausible.
- If candidates do not support the requested sector, return an empty selected array and explain the gap in missingCriteria.
- Do not broaden the strategy to hide uncertainty. Do not add sectors that are not supported by sourceText.$prompt$,
    false
  )
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

UPDATE mrsmith.llm_prompt
SET is_default = false
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_intent'
  AND name <> 'M&A strategy intent extractor v1'
  AND is_default;

UPDATE mrsmith.llm_prompt
SET is_default = false
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_ateco'
  AND name <> 'M&A strategy ATECO constrained rerank v1'
  AND is_default;

UPDATE mrsmith.llm_prompt
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_intent'
  AND name = 'M&A strategy intent extractor v1';

UPDATE mrsmith.llm_prompt
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_ateco'
  AND name = 'M&A strategy ATECO constrained rerank v1';

-- value_type uses the existing ma_parameter CHECK values. The current config UI
-- edits numeric parameters only; future V2 code reads value as the text switch.
INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description)
VALUES (
  'ma_strategy_pipeline',
  'v2',
  'number',
  'Pipeline strategia M&A',
  'Kill-switch per la pipeline Target M&A: v2 usa il nuovo flusso, monolith usa il monolite legacy.'
)
ON CONFLICT (key) DO UPDATE
SET value = CASE
      WHEN binocolo.ma_parameter.value IN ('v2', 'monolith') THEN binocolo.ma_parameter.value
      ELSE EXCLUDED.value
    END,
    value_type  = EXCLUDED.value_type,
    label       = EXCLUDED.label,
    description = EXCLUDED.description;

COMMIT;

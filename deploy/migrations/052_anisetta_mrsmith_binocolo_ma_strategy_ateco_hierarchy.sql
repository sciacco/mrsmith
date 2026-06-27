-- Binocolo M&A strategy V2.1 ATECO hierarchical resolver.
-- Target database: Anisetta PostgreSQL (mrsmith + binocolo schemas).
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.
--
-- Adds:
--   * binocolo.ma_parameter value_type='text' support for ma_strategy_pipeline
--   * mrsmith.llm_model / mrsmith.llm_prompt for ma_strategy_ateco_hierarchy
--   * non-default mrsmith.llm_prompt for ma_strategy_intent thesis guard

BEGIN;

ALTER TABLE binocolo.ma_parameter
  DROP CONSTRAINT IF EXISTS ma_parameter_value_type_check;

ALTER TABLE binocolo.ma_parameter
  ADD CONSTRAINT ma_parameter_value_type_check
  CHECK (value_type IN ('money', 'percent', 'number', 'text'));

UPDATE binocolo.ma_parameter
SET value = CASE
      WHEN value IN ('v2', 'monolith') THEN value
      ELSE 'v2'
    END,
    value_type = 'text',
    label = 'Pipeline strategia M&A',
    description = 'Kill-switch per la pipeline Target M&A: v2 usa il nuovo flusso, monolith usa il monolite legacy.'
WHERE key = 'ma_strategy_pipeline';

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'OpenRouter') THEN
    RAISE EXCEPTION 'Provider "OpenRouter" not found in mrsmith.llm_provider; apply migration 047 or adjust this migration to match your registry';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM mrsmith.llm_model
    WHERE app = 'binocolo'
      AND scope = 'ma_strategy_ateco_hierarchy'
      AND is_default
      AND model <> 'openai/gpt-5.5'
  ) THEN
    RAISE EXCEPTION 'Unexpected default model already exists for binocolo/ma_strategy_ateco_hierarchy; promote the desired model manually or adjust this migration';
  END IF;

  IF EXISTS (
    SELECT 1
    FROM mrsmith.llm_prompt
    WHERE app = 'binocolo'
      AND scope = 'ma_strategy_ateco_hierarchy'
      AND is_default
      AND name <> 'M&A strategy ATECO hierarchical resolver v1'
  ) THEN
    RAISE EXCEPTION 'Unexpected default prompt already exists for binocolo/ma_strategy_ateco_hierarchy; promote the desired prompt manually or adjust this migration';
  END IF;
END $$;

INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  'ma_strategy_ateco_hierarchy',
  p.id,
  'OpenAI: GPT-5.5',
  'openai/gpt-5.5',
  '{"temperature":0,"max_tokens":2400}'::jsonb,
  true,
  true,
  false
FROM mrsmith.llm_provider p
WHERE p.name = 'OpenRouter'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id        = EXCLUDED.provider_id,
    name               = EXCLUDED.name,
    params             = EXCLUDED.params,
    supports_tools     = EXCLUDED.supports_tools,
    supports_json_mode = EXCLUDED.supports_json_mode,
    is_default         = false;

UPDATE mrsmith.llm_model
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_ateco_hierarchy'
  AND model = 'openai/gpt-5.5';

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_strategy_ateco_hierarchy',
  'M&A strategy ATECO hierarchical resolver v1',
  $prompt$You resolve ATECO 2025 codes for the Binocolo Target M&A strategy pipeline.

Return only one valid JSON object. Do not include markdown, comments, or prose. Use only the ATECO divisions provided in the input and descendants returned by list_ateco_children. Do not use web search, company search, external catalogs, or the legacy search_ateco_2025 tool.

Input contains:
- prompt: the original user request;
- intent: the structured intent extracted by the previous step;
- includeSectors and excludeSectors: sector constraints only;
- divisions: all ATECO level-2 divisions available as the initial taxonomy.

Available tool:
- list_ateco_children(code): returns every descendant of an ATECO code already shown to you, excluding the requested code itself. Each item includes childCount and subtreeCount.

Output shape:
{
  "selected": [
    {"code": "ATECO code seen in divisions or tool results", "fit": "strong|weak", "reason": "short reason including why this granularity was chosen"}
  ],
  "excludedPrefixes": [
    {"prefix": "ATECO code seen in divisions or tool results", "reason": "short reason grounded in explicit exclusion text"}
  ],
  "missingCriteria": [
    {"text": "sector criterion that could not be mapped safely", "reason": "short reason"}
  ]
}

Rules:
- You receive all ATECO level-2 divisions and may inspect complete descendant subtrees with list_ateco_children.
- A list_ateco_children call returns the full subtree below the requested code. Do not call list_ateco_children again for returned descendants merely to reach leaves; select any returned descendant directly when it is the best fit.
- Select ATECO codes at the level that best matches the user's sector: division, intermediate node, or leaf.
- Do not prefer leaves by default. Prefer a parent when the request covers multiple descendants.
- Do not select broad parent nodes merely as a shortcut. If a parent has many descendants, inspect children and choose the narrowest set of nodes that still covers the user request.
- Do not select every IT-adjacent branch automatically. Select only branches supported by the user's sector text.
- You may use language understanding to interpret business terms or acronyms, but the final code choice must be grounded in visited ATECO labels.
- If a business term or acronym remains ambiguous after inspecting the taxonomy, return a sector missing or ambiguity instead of forcing a code.
- selected.code must be copied from an input division or from a list_ateco_children result. Never output a code that was not shown in this resolver call.
- excludedPrefixes may only be used when excludeSectors contains an explicit sector exclusion. The prefix must be a code shown in this resolver call.
- missingCriteria may only refer to includeSectors or excludeSectors from the input.
- Do not put territory, turnover, employees, legal forms, owner age, status, revenue-per-employee, max-shareholders, or other non-sector constraints in missingCriteria.
- If a non-sector constraint cannot be mapped to ATECO, ignore it in this scope because it is handled by another resolver.
- Do not broaden the strategy to hide uncertainty. Use fit weak only for adjacent but plausible codes explicitly supported by the visited taxonomy.$prompt$,
  false
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = false;

UPDATE mrsmith.llm_prompt
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'ma_strategy_ateco_hierarchy'
  AND name = 'M&A strategy ATECO hierarchical resolver v1';

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_strategy_intent',
  'M&A strategy intent extractor v1 thesis guard',
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
- Use null for unknown numbers and empty arrays for absent lists.
- thesis must be exactly one of: successione, crescita, consolidamento, tuck_in, generico, or "".
- Use "" when the request does not explicitly imply an acquisition thesis.
- Never copy the full user request into thesis.$prompt$,
  false
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = false;

COMMIT;

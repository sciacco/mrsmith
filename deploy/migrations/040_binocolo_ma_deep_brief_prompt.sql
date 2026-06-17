-- Binocolo M&A — modello + prompt per il brief LLM dell'analisi approfondita (Fase 5).
-- Target database: Anisetta PostgreSQL. Apply after 039 (llm_model/llm_prompt from 026).
-- Scope ma_deep_brief, default server-side. La narrativa gira sui KPI GIÀ calcolati
-- da Go (nessun numero inventato dall'LLM); il brief è tesi-neutro (cache globale).

BEGIN;

INSERT INTO binocolo.llm_model (id, scope, name, model, is_default)
VALUES ('00000000-0000-0000-0000-000000000401', 'ma_deep_brief', 'OpenAI: GPT-5.5', 'openai/gpt-5.5', true)
ON CONFLICT (scope, model) DO UPDATE
SET name = EXCLUDED.name, is_default = EXCLUDED.is_default;

INSERT INTO binocolo.llm_prompt (id, scope, name, prompt, is_default)
VALUES (
  '00000000-0000-0000-0000-000000000402',
  'ma_deep_brief',
  'Brief analista M&A v1',
  $prompt$
Sei un analista M&A senior. Ricevi i KPI finanziari GIA' CALCOLATI di un'azienda
(scorecard) e una valutazione di settore (valuation), in JSON. NON ricalcolare e
NON inventare numeri: usa solo i valori forniti.
Rispondi solo con JSON valido nel formato:
{
  "verdict": "2-3 frasi: giudizio complessivo sulla solidita' finanziaria",
  "rag": "green|amber|red",
  "thesisReading": "2-4 frasi: lettura del profilo (redditivita', leva, liquidita', crescita) e cosa implica per un acquirente",
  "redFlags": [
    {"severity": "warning|info", "claim": "anomalia osservata nei dati", "ddQuestion": "domanda di due diligence conseguente"}
  ]
}
Regole:
- cita solo metriche presenti in scorecard/valuation; se un dato manca, non inventarlo;
- rag deve riflettere overallRag della scorecard quando presente;
- redFlags: massimo 5, concrete e verificabili (es. PFN/EBITDA elevato, patrimonio eroso, margini bassi, ciclo finanziario lungo);
- niente consigli legali o garanzie; tono asciutto e professionale.
$prompt$,
  true
)
ON CONFLICT (scope, name) DO UPDATE
SET prompt = EXCLUDED.prompt, is_default = EXCLUDED.is_default;

COMMIT;

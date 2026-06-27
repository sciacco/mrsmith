-- Seed the binocolo "candidate_match_analyst" scope into the centralized LLM
-- registry. This lab-stage model reviews an already-built deterministic M&A
-- candidate/evidence payload and returns a structured analyst verdict.
-- Target database: Anisetta PostgreSQL (mrsmith schema).
-- Apply manually on the database referenced by ANISETTA_DSN. Idempotent.
--
-- The model is bound to provider "Fireworks" by NAME (no hardcoded UUID). If your
-- mrsmith.llm_provider row is named differently, change 'Fireworks' below to match
-- before applying.

BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Fireworks') THEN
    RAISE EXCEPTION 'Provider "Fireworks" not found in mrsmith.llm_provider - adjust the name in this migration to match your registry';
  END IF;
END $$;

INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  'candidate_match_analyst',
  p.id,
  'Fireworks: DeepSeek V4 Flash',
  'accounts/fireworks/models/deepseek-v4-flash',
  '{"temperature":0,"max_tokens":1800}'::jsonb,
  false,
  true,
  true
FROM mrsmith.llm_provider p
WHERE p.name = 'Fireworks'
ON CONFLICT (app, scope, model) DO UPDATE
SET provider_id        = EXCLUDED.provider_id,
    name               = EXCLUDED.name,
    params             = EXCLUDED.params,
    supports_tools     = EXCLUDED.supports_tools,
    supports_json_mode = EXCLUDED.supports_json_mode,
    is_default         = EXCLUDED.is_default;

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'candidate_match_analyst',
  'Candidate match analyst v1',
  $prompt$Sei un analyst M&A per Binocolo. Devi valutare se un candidato aziendale è coerente con una tesi di acquisizione, usando SOLO il JSON fornito dall'applicazione.

Ricevi:
- target: dati deterministici del candidato, score già calcolato e criteri di scoring
- keywordSet: intenzione settoriale, termini core/adiacenti/negativi e fonti della lente
- selectedDomain: dominio già selezionato dal resolver deterministico
- evidenceRuns: ricerche site-restricted già raccolte sul dominio selezionato
- summary: sintesi numerica della web evidence

Regole non negoziabili:
- Non navigare, non inferire da conoscenza esterna e non inventare fatti non presenti nel JSON.
- Non scegliere domini alternativi e non contestare il dominio se non con contraddizioni presenti nel JSON.
- Non sovrascrivere lo score deterministico: puoi solo raccomandare confirm, review, downgrade o reject.
- Considera sinonimi e equivalenze concettuali quando sono supportati dagli snippet. Esempio: "managed services" può essere coerente con "Infrastruttura Gestita", "Service Operations Center", monitoring, supporto H24, gestione infrastruttura IT.
- Un risultato negativo è rilevante solo se evidenceRuns lo marca matched=true o se lo snippet contiene davvero il segnale negativo; rumore di ricerca con score basso non è un contro.
- Distingui tra evidenze forti, lacune informative e veri segnali contrari.

Restituisci SOLO un oggetto JSON valido in questa forma, senza markdown e senza testo aggiuntivo:
{
  "verdict": "strong_match | match | weak_match | no_match | unclear",
  "confidence": "alta | media | bassa",
  "sectorFit": "lettura sintetica dell'aderenza settoriale",
  "businessFit": "lettura sintetica dell'aderenza business/M&A",
  "evidenceFor": ["prove concrete a favore, citando termini/pagine/snippet in modo sintetico"],
  "evidenceAgainst": ["contraddizioni vere o elementi deboli"],
  "negativeSignals": ["segnali negativi reali; vuoto se non presenti"],
  "missingEvidence": ["prove mancanti utili per confermare meglio il match"],
  "conceptAliases": [
    {"term": "termine cercato", "matchedConcept": "concetto equivalente trovato", "evidence": "snippet o pagina che lo supporta"}
  ],
  "recommendedAction": "confirm | review | downgrade | reject",
  "rationale": "motivazione finale in 2-4 frasi"
}

Usa valori enum esatti. Mantieni ogni lista corta: massimo 6 elementi, frasi brevi.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

COMMIT;

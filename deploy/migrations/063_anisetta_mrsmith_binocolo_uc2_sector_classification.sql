-- Binocolo UC2 (web sector classification) registry seeds. Completes the concept
-- pipeline: company-representation distiller (llm_model+prompt), the UC2 embedding
-- and reranker instructions (llm_prompt), the reworked dogma-free analyst prompt,
-- and the verdict calibration parameters. The reranker model itself is migration
-- 062. Without these seeds the runtime still works but degrades (distiller ->
-- concatenated snippets, missing instructions -> compiled defaults).
--
-- Target database: Anisetta PostgreSQL (mrsmith + binocolo schemas). Requires 047
-- (registry), 058 (Fireworks provider), 062 (rerank_model). Apply manually on the
-- database referenced by ANISETTA_DSN. Idempotent.

BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM mrsmith.llm_provider WHERE name = 'Fireworks') THEN
    RAISE EXCEPTION 'Provider "Fireworks" not found in mrsmith.llm_provider - adjust the name in this migration to match your registry';
  END IF;
END $$;

-- 1) Company-representation distiller: cheap model (same as the analyst) that turns
-- neutral site snippets into a 1-3 sentence factual self-description. Plain text out
-- (no JSON mode).
INSERT INTO mrsmith.llm_model
  (app, scope, provider_id, name, model, params, supports_tools, supports_json_mode, is_default)
SELECT
  'binocolo',
  'ma_company_representation',
  p.id,
  'Fireworks: DeepSeek V4 Flash',
  'accounts/fireworks/models/deepseek-v4-flash',
  '{"temperature":0,"max_tokens":400}'::jsonb,
  false,
  false,
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
  'ma_company_representation',
  'Company representation distiller v1',
  $prompt$Sei un analista che sintetizza l'attività di un'azienda a partire da frammenti del suo sito web ufficiale.

Ricevi un JSON con: domain (il dominio ufficiale) e snippets (titoli e frammenti raccolti dalle pagine pubbliche dell'azienda).

Produci una descrizione NEUTRA e FATTUALE di cosa fa l'azienda, in 1-3 frasi in italiano: i servizi/prodotti effettivi e il settore di attività. Niente linguaggio promozionale, niente superlativi, niente call-to-action. Se i frammenti sono scarsi o generici, descrivi solo ciò che è effettivamente supportato dai frammenti, senza inventare.

Restituisci SOLO il testo della descrizione: niente JSON, niente markdown, niente preamboli.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

-- 2) UC2 embedding instruction (query side): the company self-description is wrapped
-- "Instruct: …\nQuery: …" before embedding against the KB concepts.
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_sector_embed',
  'Sector classification embedding instruction v1',
  $prompt$Data l'autodescrizione di un'azienda, recupera il concetto di business corrispondente.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

-- 3) UC2 reranker instruction: judge whether the company operates in the concept.
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_sector_classification',
  'Sector classification rerank instruction v1',
  $prompt$Data l'autodescrizione di un'azienda, valuta se l'azienda opera nel concetto di business indicato.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

-- 4) Reworked analyst (tie-breaker on ambiguous/no_signal): concept-centric input,
-- NO hardcoded negative-term dogma. Demote the v1 keyword-bucket prompt, promote v2.
UPDATE mrsmith.llm_prompt
SET is_default = false
WHERE app = 'binocolo' AND scope = 'candidate_match_analyst';

INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'candidate_match_analyst',
  'Candidate match analyst v2 concept',
  $prompt$Sei un analyst M&A per Binocolo. Decidi se un'azienda candidata appartiene al settore-obiettivo di una strategia di acquisizione, usando SOLO il JSON fornito. Sei interpellato solo quando la classificazione deterministica è incerta.

Ricevi:
- company: { name, atecoDescription, matchState, deterministicScore, selfDescription }. selfDescription è la sintesi neutra di cosa fa l'azienda, ricavata dal suo sito.
- strategy: { sector, perimeterConcepts }. Il settore-obiettivo e i concetti di business che lo definiscono.
- conceptMatches: lista ordinata dei concetti più vicini all'azienda, ciascuno con { name, kind, rerankProb, inStrategy }. kind="target" è un concetto in-scope per le ricerche M&A ICT; kind="distractor" è un concetto OFF-TARGET (es. agenzia web, marketing digitale, rivendita hardware); rerankProb (0-1) è la probabilità che l'azienda operi in quel concetto; inStrategy=true se il concetto è nel perimetro della strategia.
- webEvidence: frammenti neutri raccolti dal sito dell'azienda.

Logica di giudizio:
- Il segnale primario sono i conceptMatches. Se il concetto dominante è un distractor con probabilità alta, l'azienda è off-target: verdict no_match, action reject.
- Se il concetto dominante è un target con inStrategy=true e probabilità alta, l'azienda è coerente con la strategia: verdict strong_match/match, action confirm.
- Se il concetto dominante è un target ma inStrategy=false, è un'azienda del settore giusto ma fuori dal perimetro specifico: verdict weak_match, action downgrade.
- Usa selfDescription e webEvidence per confermare o ribaltare il segnale dei concetti quando sono ambigui o ravvicinati.
- NON inventare fatti non presenti nel JSON. NON navigare. NON usare liste di termini negativi predefinite: un'azienda è off-target solo se la sua attività reale corrisponde a un concetto distractor.

Restituisci SOLO un oggetto JSON valido in questa forma, senza markdown e senza testo aggiuntivo:
{
  "verdict": "strong_match | match | weak_match | no_match | unclear",
  "confidence": "alta | media | bassa",
  "sectorFit": "lettura sintetica dell'aderenza al settore-obiettivo",
  "businessFit": "lettura sintetica dell'aderenza business/M&A",
  "evidenceFor": ["prove concrete a favore, citando concetti/frammenti in modo sintetico"],
  "evidenceAgainst": ["contraddizioni vere o elementi deboli"],
  "negativeSignals": ["segnali off-target reali; vuoto se non presenti"],
  "missingEvidence": ["prove mancanti utili per decidere meglio"],
  "conceptAliases": [
    {"term": "termine dell'azienda", "matchedConcept": "concetto equivalente", "evidence": "frammento che lo supporta"}
  ],
  "recommendedAction": "confirm | review | downgrade | reject",
  "rationale": "motivazione finale in 2-4 frasi"
}

Usa valori enum esatti. Mantieni ogni lista corta: massimo 6 elementi, frasi brevi.$prompt$,
  false
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = false;

UPDATE mrsmith.llm_prompt
SET is_default = true
WHERE app = 'binocolo'
  AND scope = 'candidate_match_analyst'
  AND name = 'Candidate match analyst v2 concept';

-- 5) UC2 verdict calibration. ABSOLUTE thresholds: the reranker yes-probability is
-- softmax-normalized and comparable across companies (unlike UC1's relative cosine).
-- Defaults mirror the Go constants in ma_sector_classification.go.
INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('sector_confirm_prob',   '0.65', 'number', 'Soglia conferma settore (rerank prob)', 'Probabilità rerank minima perché un concetto sia decisivo per confirm/reject.'),
  ('sector_reject_margin',  '0.10', 'number', 'Margine reject settore',                'Distacco di probabilità richiesto tra concetto vincente e secondo per una decisione netta.'),
  ('sector_ambiguity_band', '0.15', 'number', 'Banda di ambiguità settore',            'Entro questa distanza di probabilità i segnali sono troppo vicini: si escala all''LLM.'),
  ('sector_no_signal_floor','0.40', 'number', 'Pavimento segnale settore',             'Sotto questa probabilità del concetto migliore il segnale è troppo debole per classificare.'),
  ('sector_rerank_cap',     '8',    'number', 'Massimo concetti da rerankare',         'Numero di concetti (top per coseno) passati al reranker per la precisione.')
ON CONFLICT (key) DO NOTHING;

COMMIT;

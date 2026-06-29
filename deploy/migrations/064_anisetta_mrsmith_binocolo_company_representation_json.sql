-- Binocolo UC2 company-representation distiller: switch to JSON output.
--
-- The plain-text distiller (migration 063) let the cheap reasoning model
-- (deepseek-v4-flash) leak its chain-of-thought into the description on large/messy
-- inputs (e.g. "We need to produce a neutral, factual description of what X does…"),
-- which then got embedded/reranked as if it were the company's self-description. The
-- analyst call never had this problem because it runs in json_object mode. This
-- migration aligns the distiller: json_object output, a {"description": "..."} schema
-- in the prompt, and a slightly larger token budget. The Go code also appends a JSON
-- instruction and forces response_format, so it works even before this is applied;
-- this keeps the persisted prompt/model row consistent and auditable.
--
-- Target database: Anisetta PostgreSQL (mrsmith schema). Requires 047 (registry),
-- 058 (Fireworks provider), 063 (UC2 seeds). Apply manually on the database
-- referenced by ANISETTA_DSN. Idempotent.

BEGIN;

-- 1) Model row: enable JSON mode and give the distilled description headroom.
UPDATE mrsmith.llm_model
SET supports_json_mode = true,
    params             = '{"temperature":0,"max_tokens":800}'::jsonb
WHERE app = 'binocolo'
  AND scope = 'ma_company_representation'
  AND model = 'accounts/fireworks/models/deepseek-v4-flash';

-- 2) Prompt: request a JSON object instead of bare text. Keeps the neutral/factual
-- distillation guidance; only the output contract changes.
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES (
  'binocolo',
  'ma_company_representation',
  'Company representation distiller v1',
  $prompt$Sei un analista che sintetizza l'attività di un'azienda a partire da frammenti del suo sito web ufficiale.

Ricevi un JSON con: domain (il dominio ufficiale) e snippets (titoli e frammenti raccolti dalle pagine pubbliche dell'azienda).

Produci una descrizione NEUTRA e FATTUALE di cosa fa l'azienda, in 1-3 frasi in italiano: i servizi/prodotti effettivi e il settore di attività. Niente linguaggio promozionale, niente superlativi, niente call-to-action. Se i frammenti sono scarsi o generici, descrivi solo ciò che è effettivamente supportato dai frammenti, senza inventare. Ignora i frammenti che riguardano cookie/consenso o dati strutturati di pagina.

Rispondi ESCLUSIVAMENTE con un oggetto JSON valido nella forma {"description": "<descrizione>"}. Nessun altro testo, nessun markdown, nessun ragionamento.$prompt$,
  true
)
ON CONFLICT (app, scope, name) DO UPDATE
SET prompt     = EXCLUDED.prompt,
    is_default = EXCLUDED.is_default;

COMMIT;

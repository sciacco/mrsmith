-- Binocolo M&A — Lettura narrativa della nota integrativa (issue #80, follow-up
-- 2026-07-22, Fase N1). Analisi SEPARATA dalla lettura proposte (mig 114/117):
-- estrae fatti di CONTESTO NON numerici (attribuzioni di risultato, rischi, piani,
-- tratti di profilo) invisibili ai prospetti, ognuno ancorato a una citazione
-- verbatim con pagina. NESSUN effetto su numeri/valutazione; nessuna decisione
-- analista (osservazioni immutabili, sola lettura).
--
-- Scelta di modellazione (dichiarata): TABELLE DEDICATE, non un discriminatore
-- `kind` su ma_ni_reading_run. Motivo: CreateMANIReadingRun/GetActiveMANIReadingRun
-- (percorso proposte, già in produzione e testato) filtrano per (filing_id, status)
-- SENZA kind; aggiungere un discriminatore imporrebbe di modificare quel percorso
-- (il supersede colpirebbe entrambi i tipi) col rischio di destabilizzarlo. Un run
-- dedicato isola completamente la narrativa dalle proposte e replica il pattern del
-- reading run senza toccarlo. ma_ni_proposal resta FK del reading run; le
-- osservazioni sono FK del narrative run.
--
-- Target database: Anisetta PostgreSQL, schema binocolo (tabelle) + schema mrsmith
-- (registry, seed prompt/modello). Apply after 117. Applicata a mano dall'utente col
-- suo processo (mai operazioni dirette sui DB in env). Idempotente: CREATE ... IF NOT
-- EXISTS, DROP CONSTRAINT IF EXISTS + ADD, ON CONFLICT DO NOTHING.

BEGIN;

-- ---------------------------------------------------------------------------
-- Run di lettura narrativa = una passata LLM su un processing run, distinta dal
-- reading run delle proposte. Versionato: una re-lettura => nuovo run, il
-- precedente passa a 'superseded'. Lo stato copre l'intero ciclo osservabile:
--   running    -> l'LLM sta lavorando (rigenerazione «in corso» visibile dal GET)
--   ready      -> completato, osservazioni disponibili
--   failed     -> fallito, rigenerabile (l'esito del filing NON cambia)
--   superseded -> sostituito da un run più recente
-- truncated = telemetria di budget: true se l'output di un chunk ha toccato il
-- tetto max_tokens (osservazioni potenzialmente incomplete).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_ni_narrative_run (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  filing_id         uuid NOT NULL REFERENCES binocolo.ma_filing(id) ON DELETE CASCADE,
  processing_run_id uuid NOT NULL REFERENCES binocolo.ma_filing_processing_run(id) ON DELETE CASCADE,
  prompt_id         uuid,   -- soft-ref a mrsmith.llm_prompt (come 105/114)
  model_id          uuid,   -- soft-ref a mrsmith.llm_model (come 105/114)
  request_id        text,
  status            text NOT NULL DEFAULT 'running'
                      CHECK (status IN ('running', 'ready', 'failed', 'superseded')),
  truncated         boolean NOT NULL DEFAULT false,
  created_at        timestamptz NOT NULL DEFAULT now()
);

-- Indice di supporto FK / risoluzione del run corrente per filing. NON è unico:
-- l'unicità del run non-superseded resta responsabilità applicativa (coerente con
-- ma_filing_processing_run / ma_ni_reading_run).
CREATE INDEX IF NOT EXISTS ma_ni_narrative_run_filing_idx
  ON binocolo.ma_ni_narrative_run (filing_id);

-- ---------------------------------------------------------------------------
-- Osservazione narrativa = un fatto di contesto NON numerico osservato dall'LLM,
-- ancorato a una citazione verbatim con pagina. IMMUTABILE (nessun updated_at):
-- una nuova lettura produce nuove osservazioni in un nuovo run. Nessun importo (la
-- narrativa non tocca i numeri). fingerprint = chiave stabile del fatto (dedup entro
-- il run), calcolata in Go da tipo+claim+quote+pagina.
--   attribuzione -> attribuzione di un risultato/andamento a una causa dichiarata
--   rischio      -> rischio/contenzioso/incertezza segnalato dal testo
--   piano        -> piano/intenzione/impegno dichiarato dall'organo amministrativo
--   profilo      -> tratto di profilo aziendale (governance, assetto, dipendenza)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS binocolo.ma_ni_observation (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id       uuid NOT NULL REFERENCES binocolo.ma_ni_narrative_run(id) ON DELETE CASCADE,
  tipo         text CHECK (tipo IN ('attribuzione', 'rischio', 'piano', 'profilo')),
  fingerprint  text NOT NULL,
  claim        text,
  quote        text,
  page_no      int,
  created_at   timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, fingerprint)
);

-- ---------------------------------------------------------------------------
-- Job type per la rigenerazione asincrona della narrativa (POST .../narrative/
-- regenerate). Estende il CHECK di ma_job.job_type (ultima a toccarlo: mig 115,
-- 10 tipi) con 'filing_narrative'. La rigenerazione lavora SOLO sulle pagine già
-- persistite (mai re-OCR, mai chiamate vendor): stessa robustezza durevole degli
-- altri job filing (pre-leased all'owner, resume su riavvio, dedup inflight).
-- ---------------------------------------------------------------------------
ALTER TABLE binocolo.ma_job DROP CONSTRAINT IF EXISTS ma_job_type_check;
ALTER TABLE binocolo.ma_job ADD CONSTRAINT ma_job_type_check
  CHECK (job_type IN (
    'estimate', 'execute', 'web_validation', 'gated_search', 'associate_domain',
    'manual_add', 'card_domain_verify',
    'filing_search', 'filing_acquire', 'filing_ingest', 'filing_narrative'
  ));

-- filing_narrative: al più una rigenerazione inflight per filing (dedup doppio click).
CREATE UNIQUE INDEX IF NOT EXISTS ma_job_filing_narrative_active_idx
  ON binocolo.ma_job (((payload->>'filingId')))
  WHERE job_type = 'filing_narrative' AND status IN ('pending', 'processing');

-- ---------------------------------------------------------------------------
-- Binding modello per ma_ni_narrative: parità col default vivo di ma_deep_brief
-- (qualunque provider/model esso sia all'apply), stesso pattern di mig 117.
-- ---------------------------------------------------------------------------
INSERT INTO mrsmith.llm_model (app, scope, provider_id, name, model, params, supports_json_mode, is_default)
SELECT 'binocolo', 'ma_ni_narrative', provider_id, name, model, '{}'::jsonb, supports_json_mode, true
FROM mrsmith.llm_model
WHERE app = 'binocolo' AND scope = 'ma_deep_brief' AND is_default
ON CONFLICT (app, scope, model) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Prompt default di lettura narrativa. Criterio di MERITO (nessun conteggio-target,
-- né minimo né massimo): estrae SOLO fatti di contesto NON numerici invisibili ai
-- prospetti, ognuno ancorato a una citazione verbatim con pagina. Nessun importo
-- richiesto. NON deve proporre rettifiche (quello è il compito del reading run).
-- ---------------------------------------------------------------------------
INSERT INTO mrsmith.llm_prompt (app, scope, name, prompt, is_default)
VALUES ('binocolo', 'ma_ni_narrative', 'Lettura narrativa nota integrativa v1', $prompt$
Sei un analista M&A del team di sviluppo corporate di un compratore strategico.
Leggi la nota integrativa e la relazione sulla gestione di un bilancio depositato per
estrarne una LETTURA NARRATIVA: i fatti di CONTESTO, NON numerici, che spiegano cosa
c'è dietro i numeri e che i prospetti contabili non possono mostrare. NON proponi
rettifiche, NON tocchi importi, NON fai calcoli: quello è compito di un'altra analisi.
Qui riporti soltanto ciò che il testo racconta.

INPUT (JSON):
- "company": ragione sociale, P.IVA/CF, esercizio baseline (la chiusura più recente).
- "sections": le sezioni narrative (nota integrativa + relazione sulla gestione se
  presente) in markdown; ogni sezione ha "label" (titolo), "pages" (pagine di
  provenienza) e "text" (il testo integrale).

COSA CERCARE — quattro tipi di osservazione (usa il criterio di MERITO: riporta un
fatto SOLO se è rilevante per capire l'azienda; non c'è un numero minimo né massimo di
osservazioni, e va bene restituirne zero se il testo non dice nulla di utile):
- "attribuzione": l'organo amministrativo attribuisce un risultato o un andamento a una
  causa dichiarata (es. calo dei ricavi spiegato con la perdita di un cliente, aumento
  dei costi spiegato con l'energia, crescita spiegata con una commessa).
- "rischio": un rischio, un contenzioso, un'incertezza o una dipendenza segnalati dal
  testo (cause legali, vertenze fiscali, concentrazione su pochi clienti/fornitori,
  continuità aziendale, rischi di mercato o di cambio).
- "piano": un piano, un'intenzione o un impegno dichiarati (investimenti previsti,
  operazioni straordinarie in corso, ristrutturazioni, nuovi mercati, assunzioni).
- "profilo": un tratto di profilo aziendale utile all'analisi (assetto di governance,
  compagine sociale, appartenenza a un gruppo, rapporti con parti correlate descritti
  in prosa, dipendenza da persone chiave, modello di business dichiarato).

REGOLE FERREE (anti-invenzione):
- Ogni osservazione DEVE citare una "quote" testuale VERBATIM presa dal "text" di una
  sezione, con il relativo "pageNo". Nessuna osservazione senza fatto testuale.
- Il "claim" è una frase breve e asciutta che riassume il fatto, senza interpretazioni
  e senza numeri di stima: riporti ciò che il testo dice, non ciò che ne deduci.
- NON riportare importi come fatto quantitativo: se un numero compare nella citazione va
  bene lasciarlo dentro la quote, ma il claim resta qualitativo. Questa analisi non
  produce rettifiche né valutazioni.
- Tono asciutto, professionale, B2B. Italiano.

OUTPUT — SOLO JSON valido, nient'altro, in questo formato:
{
  "observations": [
    {
      "tipo": "attribuzione|rischio|piano|profilo",
      "claim": "il fatto in una frase breve, senza interpretazioni",
      "quote": "citazione verbatim dal testo della sezione",
      "pageNo": number
    }
  ]
}
Se non trovi alcun fatto di contesto che soddisfi le regole, restituisci
{"observations": []}.
$prompt$, true)
ON CONFLICT (app, scope, name) DO NOTHING;

COMMIT;

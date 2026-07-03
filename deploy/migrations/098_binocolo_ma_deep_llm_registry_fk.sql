-- 098 — ma_deep_analysis: ripunta i FK model_id/prompt_id al registry LLM vivo (mrsmith).
--
-- Il cutover del registry (mig 047) ha copiato binocolo.llm_model/llm_prompt in
-- mrsmith.* SENZA preservare gli id (gen_random_uuid() sul nuovo schema): gli uuid
-- risolti a runtime da mrsmith non esistono nelle tabelle legacy, quindi i FK di
-- ma_deep_analysis (mig 037, → binocolo.llm_*) fanno fallire OGNI persistenza del
-- brief post-cutover: sia /ma/deep/regenerate-briefs (500 osservato il 2026-07-03)
-- sia il salvataggio di qualunque NUOVO deep-dive da parte del worker.
-- binocolo.ma_model_audit (mig 026) ha gli stessi FK legacy ma non viene più
-- scritta da nessun codice (audit → mrsmith.llm_call_audit): lasciata invariata.

BEGIN;

-- 1) Sgancia i FK legacy PRIMA del remap: i valori nuovi vivono su mrsmith e
--    violerebbero i vincoli ancora puntati a binocolo.llm_* (nomi auto-generati
--    dai REFERENCES inline di mig 037).
ALTER TABLE binocolo.ma_deep_analysis
  DROP CONSTRAINT IF EXISTS ma_deep_analysis_model_id_fkey,
  DROP CONSTRAINT IF EXISTS ma_deep_analysis_prompt_id_fkey;

-- 2) Rimappa la provenance storica old→new dove il match è deterministico: la
--    copia di mig 047 ha preservato (scope, model) per i modelli e (scope, name)
--    per i prompt, entrambi unici per app su mrsmith.
UPDATE binocolo.ma_deep_analysis d
SET model_id = nm.id
FROM binocolo.llm_model om
JOIN mrsmith.llm_model nm
  ON nm.app = 'binocolo' AND nm.scope = om.scope AND nm.model = om.model
WHERE d.model_id = om.id;

UPDATE binocolo.ma_deep_analysis d
SET prompt_id = np.id
FROM binocolo.llm_prompt op
JOIN mrsmith.llm_prompt np
  ON np.app = 'binocolo' AND np.scope = op.scope AND np.name = op.name
WHERE d.prompt_id = op.id;

-- 3) Azzera i residui non rimappabili (righe registry cambiate fuori-banda dopo
--    la copia): la provenance qui è best-effort, coerente con ON DELETE SET NULL.
UPDATE binocolo.ma_deep_analysis
SET model_id = NULL
WHERE model_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM mrsmith.llm_model m WHERE m.id = ma_deep_analysis.model_id);

UPDATE binocolo.ma_deep_analysis
SET prompt_id = NULL
WHERE prompt_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM mrsmith.llm_prompt p WHERE p.id = ma_deep_analysis.prompt_id);

-- 4) Ripunta i FK al registry vivo.
ALTER TABLE binocolo.ma_deep_analysis
  ADD CONSTRAINT ma_deep_analysis_model_id_fkey
    FOREIGN KEY (model_id) REFERENCES mrsmith.llm_model(id) ON DELETE SET NULL,
  ADD CONSTRAINT ma_deep_analysis_prompt_id_fkey
    FOREIGN KEY (prompt_id) REFERENCES mrsmith.llm_prompt(id) ON DELETE SET NULL;

COMMIT;

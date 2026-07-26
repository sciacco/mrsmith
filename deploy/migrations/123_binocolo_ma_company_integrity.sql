-- 123: Binocolo M&A — vincoli di integrità del registro azienda (issue #88).
--
-- Dichiara nel database gli invarianti già rispettati dai writer dopo il cutover
-- di #86: ogni target ha una company_key, tutte le chiavi delle tabelle di
-- dettaglio risolvono nel registro e una sessione contiene una sola occorrenza
-- della stessa azienda.
--
-- Target database: Anisetta PostgreSQL, schema binocolo. Apply once after 122.
-- Il file NON è ri-eseguibile: un secondo apply fallisce con vincolo duplicato.
-- La transazione garantisce che un errore lasci lo schema interamente invariato.
--
-- Preflight immediatamente prima dell'apply:
--   - IDENTITA-AZIENDA-PROBE.sql Q17: orfane, senza_chiave e vuote tutte a 0;
--   - IDENTITA-AZIENDA-PROBE.sql Q20: nessuna riga.
-- Dopo l'apply eseguire Q18: le prime cinque righe devono restare a 0.
--
-- ATTENZIONE AL ROLLBACK APPLICATIVO: dopo il commit un binario precedente a
-- #86 non può più inserire ma_target, perché non include company_key nella lista
-- delle colonne e il NOT NULL rifiuta la scrittura.
--
-- ROLLBACK SQL (eseguire deliberatamente in una transazione separata):
--   BEGIN;
--   ALTER TABLE binocolo.ma_target DROP CONSTRAINT ma_target_session_company_key_key;
--   ALTER TABLE binocolo.ma_filing_acquisition DROP CONSTRAINT ma_filing_acquisition_context_company_key_fkey;
--   ALTER TABLE binocolo.ma_deep_payload_vintage DROP CONSTRAINT ma_deep_payload_vintage_company_key_fkey;
--   ALTER TABLE binocolo.ma_deep_analysis DROP CONSTRAINT ma_deep_analysis_company_key_fkey;
--   ALTER TABLE binocolo.ma_company_bm_family DROP CONSTRAINT ma_company_bm_family_company_key_fkey;
--   ALTER TABLE binocolo.ma_company_note DROP CONSTRAINT ma_company_note_company_key_fkey;
--   ALTER TABLE binocolo.ma_company_fact DROP CONSTRAINT ma_company_fact_company_key_fkey;
--   ALTER TABLE binocolo.ma_company_domain DROP CONSTRAINT ma_company_domain_company_key_fkey;
--   ALTER TABLE binocolo.ma_session_thesis_reading DROP CONSTRAINT ma_session_thesis_reading_company_key_fkey;
--   ALTER TABLE binocolo.ma_card_thesis_reading DROP CONSTRAINT ma_card_thesis_reading_company_key_fkey;
--   ALTER TABLE binocolo.ma_card_irl_item DROP CONSTRAINT ma_card_irl_item_company_key_fkey;
--   ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT ma_initiative_card_company_key_fkey;
--   ALTER TABLE binocolo.ma_sector_eval_label DROP CONSTRAINT ma_sector_eval_label_company_key_fkey;
--   ALTER TABLE binocolo.ma_target_outcome DROP CONSTRAINT ma_target_outcome_company_key_fkey;
--   ALTER TABLE binocolo.ma_target_web_validation DROP CONSTRAINT ma_target_web_validation_company_key_fkey;
--   ALTER TABLE binocolo.ma_target_rating DROP CONSTRAINT ma_target_rating_company_key_fkey;
--   ALTER TABLE binocolo.ma_target DROP CONSTRAINT ma_target_company_key_fkey;
--   ALTER TABLE binocolo.ma_target ALTER COLUMN company_key DROP NOT NULL;
--   COMMIT;

BEGIN;
SET LOCAL lock_timeout = '5s';

ALTER TABLE binocolo.ma_target_rating
  ADD CONSTRAINT ma_target_rating_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_target_web_validation
  ADD CONSTRAINT ma_target_web_validation_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_target_outcome
  ADD CONSTRAINT ma_target_outcome_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_sector_eval_label
  ADD CONSTRAINT ma_sector_eval_label_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_initiative_card
  ADD CONSTRAINT ma_initiative_card_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_card_irl_item
  ADD CONSTRAINT ma_card_irl_item_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_card_thesis_reading
  ADD CONSTRAINT ma_card_thesis_reading_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_session_thesis_reading
  ADD CONSTRAINT ma_session_thesis_reading_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_company_domain
  ADD CONSTRAINT ma_company_domain_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_company_fact
  ADD CONSTRAINT ma_company_fact_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_company_note
  ADD CONSTRAINT ma_company_note_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_company_bm_family
  ADD CONSTRAINT ma_company_bm_family_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_deep_analysis
  ADD CONSTRAINT ma_deep_analysis_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_deep_payload_vintage
  ADD CONSTRAINT ma_deep_payload_vintage_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_filing_acquisition
  ADD CONSTRAINT ma_filing_acquisition_context_company_key_fkey
  FOREIGN KEY (context_company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

-- ma_target per ultima: è il percorso di scrittura più caldo. Se un lock sulle
-- altre tabelle supera il timeout, la transazione abortisce prima di prenderne
-- l'ACCESS EXCLUSIVE e fermare le scritture target durante l'attesa.
ALTER TABLE binocolo.ma_target
  ALTER COLUMN company_key SET NOT NULL;

ALTER TABLE binocolo.ma_target
  ADD CONSTRAINT ma_target_company_key_fkey
  FOREIGN KEY (company_key) REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT;

ALTER TABLE binocolo.ma_target
  ADD CONSTRAINT ma_target_session_company_key_key UNIQUE (session_id, company_key);

COMMIT;

-- Backfill operativo issue #82/#90: ma_company_note -> ma_target_outcome.
-- Rieseguibile: conserva UUID, autore e data originali e ignora gli UUID già migrati.
-- Eseguire nei punti previsti dal runbook di cutover; non fa parte della sequenza DDL.

INSERT INTO binocolo.ma_target_outcome (
  id, session_id, initiative_id, company_key, event, note, payload,
  created_by_subject, created_by_email, created_at
)
SELECT
  note.id,
  NULL,
  NULL,
  note.company_key,
  'nota',
  note.body,
  jsonb_build_object(
    'annotationOrigin',
    CASE
      WHEN note.body ~ '^Descrizione dal sito \([0-9]{4}-[0-9]{2}-[0-9]{2}\): '
        THEN 'system_domain'
      ELSE 'legacy_registry'
    END
  ),
  note.created_by_subject,
  note.created_by_email,
  note.created_at
FROM binocolo.ma_company_note note
ON CONFLICT (id) DO NOTHING;

-- Verifica: legacy_total deve coincidere con migrated_total e missing deve essere 0.
SELECT
  (SELECT COUNT(*) FROM binocolo.ma_company_note) AS legacy_total,
  (SELECT COUNT(*)
     FROM binocolo.ma_target_outcome outcome
     JOIN binocolo.ma_company_note note ON note.id = outcome.id
    WHERE outcome.event = 'nota') AS migrated_total,
  (SELECT COUNT(*)
     FROM binocolo.ma_company_note note
     LEFT JOIN binocolo.ma_target_outcome outcome ON outcome.id = note.id
    WHERE outcome.id IS NULL) AS missing;

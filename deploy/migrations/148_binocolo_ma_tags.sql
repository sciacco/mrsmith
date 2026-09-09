-- Tag aziendali condivisi in Binocolo (issue #198): catalogo `ma_tag` e
-- associazioni many-to-many con le aziende in `ma_company_tag`.
-- Database di destinazione: Anisetta PostgreSQL, schema binocolo.
-- Idempotente: da applicare a mano sul database indicato da ANISETTA_DSN.
-- L'agente non la esegue mai: sul database la applica l'utente.
--
-- Scelte di progetto registrate qui:
--   * Catalogo condiviso da tutto il team: nessuna proprietà o visibilità
--     per utente, nessun ruolo amministrativo (PRD #198).
--   * L'identità del tag è l'UUID e sopravvive alla rinomina; l'eliminazione
--     è globale e rimuove anche le associazioni (CASCADE), mai le aziende.
--   * L'unicità dei nomi è case/space-insensitive tramite indice univoco su
--     lower(btrim(name)): «Cloud» e « cloud » sono lo stesso tag. Il nome
--     conservato mantiene la capitalizzazione originale senza spazi esterni.
--   * L'associazione vive sulla `company_key` posseduta (migrazione 120),
--     quindi i tag seguono l'azienda in ogni ricerca e iniziativa.
--   * FK a ma_company con ON DELETE RESTRICT: come per le altre tabelle
--     chiave company_key, non si eliminano aziende che hanno tag.
--   * PK composta (company_key, tag_id): niente associazioni duplicate;
--     l'indice inverso (tag_id, company_key) serve al filtro multi-tag.
--   * Nessun trigger: `updated_at` lo scrive l'applicazione, come per
--     ma_company_agreement (146) e segnalazione (147).
--   * Catalogo iniziale vuoto: nessun backfill.

BEGIN;
SET LOCAL lock_timeout = '5s';

CREATE TABLE IF NOT EXISTS binocolo.ma_tag (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ma_tag_name_normalized_key
    ON binocolo.ma_tag (lower(btrim(name)));

CREATE TABLE IF NOT EXISTS binocolo.ma_company_tag (
    company_key text NOT NULL REFERENCES binocolo.ma_company(company_key) ON DELETE RESTRICT,
    tag_id uuid NOT NULL REFERENCES binocolo.ma_tag(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (company_key, tag_id)
);

CREATE INDEX IF NOT EXISTS ma_company_tag_tag_idx
    ON binocolo.ma_company_tag (tag_id, company_key);

COMMIT;

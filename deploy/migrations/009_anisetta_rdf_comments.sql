-- RDF comments and comment notifications on Anisetta.
-- Apply manually on the database referenced by ANISETTA_DSN after
-- deploy/migrations/008_anisetta_mrsmith_diagnostics.sql.

BEGIN;

CREATE TABLE IF NOT EXISTS public.rdf_commenti (
  id bigserial PRIMARY KEY,
  richiesta_id integer NOT NULL
    REFERENCES public.rdf_richieste(id) ON DELETE CASCADE,
  commento text NOT NULL,
  autore_subject text NOT NULL DEFAULT '',
  autore_email text NOT NULL DEFAULT '',
  autore_nome text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT rdf_commenti_commento_not_blank_check
    CHECK (btrim(commento) <> '')
);

CREATE INDEX IF NOT EXISTS rdf_commenti_richiesta_created_idx
  ON public.rdf_commenti (richiesta_id, created_at, id);

CREATE TABLE IF NOT EXISTS public.rdf_commenti_menzioni (
  id bigserial PRIMARY KEY,
  commento_id bigint NOT NULL
    REFERENCES public.rdf_commenti(id) ON DELETE CASCADE,
  richiesta_id integer NOT NULL
    REFERENCES public.rdf_richieste(id) ON DELETE CASCADE,
  utente_subject text NOT NULL DEFAULT '',
  utente_email text NOT NULL,
  utente_nome text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT rdf_commenti_menzioni_email_not_blank_check
    CHECK (btrim(utente_email) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS rdf_commenti_menzioni_commento_email_idx
  ON public.rdf_commenti_menzioni (commento_id, lower(utente_email));

CREATE INDEX IF NOT EXISTS rdf_commenti_menzioni_richiesta_idx
  ON public.rdf_commenti_menzioni (richiesta_id, created_at, id);

CREATE TABLE IF NOT EXISTS public.rdf_richieste_notificati (
  id bigserial PRIMARY KEY,
  richiesta_id integer NOT NULL
    REFERENCES public.rdf_richieste(id) ON DELETE CASCADE,
  utente_subject text NOT NULL DEFAULT '',
  utente_email text NOT NULL,
  utente_nome text NOT NULL DEFAULT '',
  source_commento_id bigint
    REFERENCES public.rdf_commenti(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT rdf_richieste_notificati_email_not_blank_check
    CHECK (btrim(utente_email) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS rdf_richieste_notificati_richiesta_email_idx
  ON public.rdf_richieste_notificati (richiesta_id, lower(utente_email));

CREATE INDEX IF NOT EXISTS rdf_richieste_notificati_richiesta_idx
  ON public.rdf_richieste_notificati (richiesta_id, updated_at DESC, id DESC);

INSERT INTO mrsmith.notification_type (
  type_key,
  app_id,
  title_template,
  body_template,
  severity,
  default_policy
)
VALUES (
  'rdf_comment_created',
  'richieste-fattibilita',
  'Nuovo commento RDF',
  'E presente un nuovo commento su una richiesta di fattibilita.',
  'info',
  '{
    "portal": {
      "enabled": true
    },
    "email": {
      "enabled": true,
      "steps": []
    }
  }'::jsonb
)
ON CONFLICT (type_key) DO UPDATE
SET app_id = EXCLUDED.app_id,
    title_template = EXCLUDED.title_template,
    body_template = EXCLUDED.body_template,
    severity = EXCLUDED.severity,
    default_policy = EXCLUDED.default_policy,
    enabled = true;

COMMIT;

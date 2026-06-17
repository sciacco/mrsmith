-- Binocolo M&A — parametri di business configurabili (prezzi, budget, sconto).
-- Target database: Anisetta PostgreSQL. Apply after 036.
-- Leve di business modificabili da UI; le meccaniche interne restano in codice.
-- decorateMACost e il cancello di costo leggono i prezzi/budget da qui.

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_parameter (
  key               text PRIMARY KEY,
  value             text NOT NULL,
  value_type        text NOT NULL,
  label             text NOT NULL,
  description       text,
  updated_by_email  text,
  updated_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ma_parameter_value_type_check CHECK (value_type IN ('money', 'percent', 'number')),
  CONSTRAINT ma_parameter_key_check CHECK (key ~ '^[a-z][a-z0-9_]*$')
);

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('cost_advanced_eur',         '0.10', 'money',   'Costo enrichment advanced (€/azienda)',  'Prezzo OpenAPI.it per azienda arricchita nella fase shortlist.'),
  ('cost_full_eur',             '0.30', 'money',   'Costo analisi approfondita (€/azienda)', 'Prezzo OpenAPI.it IT-full per azienda nella fase veryshort.'),
  ('cost_dryrun_eur',           '0.01', 'money',   'Costo dry-run (€/ricerca)',              'Prezzo OpenAPI.it per il conteggio dry-run in fase stima.'),
  ('budget_default_eur',        '50',   'money',   'Budget di sessione (€)',                 'Tetto di spesa enrichment oltre il quale serve conferma esplicita.'),
  ('sme_haircut_pct',           '30',   'percent', 'Sconto PMI/illiquidità (%)',             'Sconto applicato ai multipli di valutazione di settore (quotate -> PMI).'),
  ('ebitda_fallback_threshold', '5',    'percent', 'Soglia margine EBITDA fallback (%)',     'Sotto questo margine (o con EBITDA <= 0) la valutazione usa EV/Sales invece di EV/EBITDA.')
ON CONFLICT (key) DO NOTHING;

COMMIT;

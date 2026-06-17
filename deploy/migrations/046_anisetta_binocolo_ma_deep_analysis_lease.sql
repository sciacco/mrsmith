-- Binocolo M&A — lease per-riga sul deep-worker per ownership esclusiva quando più
-- worker insistono sullo stesso DB: DB di staging condiviso fra sviluppatori, repliche
-- k8s (replicas: 2), e finestra di sovrapposizione durante i rolling deploy.
--
-- Senza lease il worker era safe solo sull'addebito (claim atomico queued->running),
-- ma il resto assumeva un drainer unico: un worker non-proprietario vedeva una riga
-- 'running' appena presa in carico (vendor_request_id non ancora scritto) e la falliva
-- come missing_request_id; e in fase 'ready' più worker rigeneravano il brief LLM in
-- parallelo. Con il lease, ogni riga è avanzata da un solo worker alla volta.
--
-- Target database: Anisetta PostgreSQL. Apply after 037 (ma_deep_analysis).

BEGIN;

ALTER TABLE binocolo.ma_deep_analysis
  ADD COLUMN IF NOT EXISTS lease_until timestamptz,
  ADD COLUMN IF NOT EXISTS locked_by   text;

COMMIT;

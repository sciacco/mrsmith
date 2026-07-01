-- Binocolo — alza il pavimento assoluto del retrieval ATECO da 0.30 a 0.45.
--
-- Il floor è ASSOLUTO sul miglior coseno TARGET: se il top-target è sotto il floor, il
-- settore è "non riconosciuto" e la rete resta vuota (nessun candidato ATECO). A 0.30 era
-- troppo permissivo: settori genuinamente FUORI dal dominio ICT superano il pavimento di
-- un soffio e generano una rete spuria.
--
-- Evidenza (fan-out 10 tesi ICT + 2 controlli fuori-ICT): il topTargetCosine separa
-- nettamente i due gruppi —
--   ICT genuini:      0.558 (erp) … 0.721 (msp)          -> KEEP
--   fuori/di confine: web-agency 0.424, prof-services 0.400, food-mfg 0.347  -> REJECT
-- Gap pulito 0.424 → 0.558. Un floor a 0.45 rifiuta i fuori-dominio (food/legale/web,
-- coerente col fatto che web/ecommerce/marketing sono distractor nella KB) mantenendo
-- tutti i 9 settori ICT reali con ampio margine. Scelto 0.45 (conservativo) invece del
-- midpoint 0.49: il floor è assoluto e i coseni driftano per query, meglio non stringere
-- troppo con 12 campioni (evita falsi-reject di nicchie ICT non campionate).
--
-- Distinto da ateco_retrieval_rel_threshold (mig 078, =0.65): quello controlla la
-- LARGHEZZA della rete una volta riconosciuto il settore; questo controlla il
-- RICONOSCIMENTO del dominio.
--
-- UPSERT: aggiorna la riga esistente (seed 061 = '0.30') o la crea. Allineato al default
-- compilato maAtecoRetrievalFloorDefault = 0.45. Editabile a runtime da Config.
--
-- Target database: Anisetta PostgreSQL (schema binocolo). Idempotente. Apply after 078.

BEGIN;

INSERT INTO binocolo.ma_parameter (key, value, value_type, label, description) VALUES
  ('ateco_retrieval_floor', '0.45', 'number', 'Pavimento assoluto retrieval ATECO',
   'Se il miglior coseno target è sotto questo valore il settore è "non riconosciuto" e la rete resta vuota. Alzato da 0.30 a 0.45: a 0.30 i settori fuori-ICT superavano il pavimento e generavano reti spurie; il topTargetCosine separa gli ICT genuini (>=0.558) dai fuori-dominio (<=0.424). Assoluto (non relativo).')
ON CONFLICT (key) DO UPDATE
  SET value = EXCLUDED.value,
      description = EXCLUDED.description;

COMMIT;

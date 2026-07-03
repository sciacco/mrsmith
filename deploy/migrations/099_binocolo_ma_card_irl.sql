-- Binocolo M&A — Information Request List per card (Fase 6 del redesign
-- deep-dive, DEEP-DIVE-IMPLEMENTATION-PLAN.md, filone F). Target: Anisetta
-- PostgreSQL, schema binocolo. Applicata a mano dall'utente. Idempotente.
--
-- L'IRL è l'artefatto che esce dal tool: seminata da 4 fonti con provenienza
-- (flag deterministici, brief neutro, lettura di tesi, template della famiglia
-- di business model) + voci dell'analista. Dopo il seed è dell'analista: il
-- re-seed è SOLO additivo (unique parziale su source_ref, insert ON CONFLICT
-- DO NOTHING) e non tocca mai la curatela. Nessun FK verso la card: l'IRL
-- sopravvive all'archiviazione (memoria istituzionale per deal che tornano).

BEGIN;

CREATE TABLE IF NOT EXISTS binocolo.ma_card_irl_item (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  initiative_id     uuid NOT NULL REFERENCES binocolo.ma_initiative(id) ON DELETE CASCADE,
  company_key       text NOT NULL,
  category          text NOT NULL DEFAULT '',
  question          text NOT NULL,
  -- Provenienza della voce; source_ref è la chiave del re-seed additivo
  -- (codice flag, hash domanda brief/tesi, id template; vuoto per analyst).
  source            text NOT NULL CHECK (source IN ('flag', 'brief', 'thesis', 'template', 'analyst')),
  source_ref        text NOT NULL DEFAULT '',
  status            text NOT NULL DEFAULT 'aperta' CHECK (status IN ('aperta', 'chiesta', 'risposta', 'na')),
  position          int  NOT NULL DEFAULT 0,
  created_by_email  text NOT NULL DEFAULT '',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ma_card_irl_item_card_idx
  ON binocolo.ma_card_irl_item (initiative_id, company_key, position);

-- Il lucchetto del re-seed additivo: una voce seminata (source_ref valorizzato)
-- esiste al più una volta per card; le voci analyst (source_ref vuoto) sono
-- libere.
CREATE UNIQUE INDEX IF NOT EXISTS ma_card_irl_item_seed_unique
  ON binocolo.ma_card_irl_item (initiative_id, company_key, source, source_ref)
  WHERE source_ref <> '';

-- Template standard per famiglia di business model: la componente di
-- COMPLETEZZA del seed (le voci generate dalle fonti sono le ANOMALIE).
CREATE TABLE IF NOT EXISTS binocolo.ma_irl_template (
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  family    text NOT NULL CHECK (family IN ('servizi_ricorrenti', 'progetto_integrazione', 'rivendita_var', 'software_prodotto')),
  category  text NOT NULL,
  question  text NOT NULL,
  position  int  NOT NULL DEFAULT 0,
  UNIQUE (family, question)
);

INSERT INTO binocolo.ma_irl_template (family, category, question, position) VALUES
  -- servizi_ricorrenti (MSP / TLC / managed services)
  ('servizi_ricorrenti', 'commerciale',   'Scomposizione dei ricavi per natura: ricorrenti contrattualizzati vs progetto vs una tantum, per gli ultimi 3 esercizi.', 10),
  ('servizi_ricorrenti', 'commerciale',   'Churn e retention per coorte cliente (logo churn e revenue churn), con motivazioni delle disdette principali.', 20),
  ('servizi_ricorrenti', 'commerciale',   'Concentrazione clienti: quota dei primi 5/10 clienti sui ricavi ricorrenti e durata residua dei relativi contratti.', 30),
  ('servizi_ricorrenti', 'legale',        'Contratti quadro e SLA verso i clienti: penali, livelli di servizio, clausole di recesso e di change of control.', 40),
  ('servizi_ricorrenti', 'legale',        'Trasferibilità dei contratti in caso di cessione: consensi richiesti, contratti intuitu personae.', 50),
  ('servizi_ricorrenti', 'operativa',     'Fornitori critici (connettività, datacenter, licenze rivendute): contratti, durate, esclusive e margini di dipendenza.', 60),
  ('servizi_ricorrenti', 'operativa',     'Metriche di servizio: volumi ticket, tempi di risoluzione, disponibilità (uptime), copertura NOC/help desk.', 70),
  ('servizi_ricorrenti', 'organizzazione','Organico tecnico: certificazioni, figure chiave e non sostituibili, patti di non concorrenza, collaboratori esterni ricorrenti.', 80),
  ('servizi_ricorrenti', 'finanziaria',   'Backlog e pipeline dei rinnovi: scadenzario contratti ricorrenti nei prossimi 24 mesi con probabilità di rinnovo.', 90),
  ('servizi_ricorrenti', 'finanziaria',   'Politica di fatturazione dei canoni (anticipata/posticipata) e risconti: riconciliazione ricavi contabili vs contrattuali.', 100),
  ('servizi_ricorrenti', 'tecnologia',    'Stack tecnologico e strumenti di gestione (RMM/PSA/monitoring): proprietà, licenze e lock-in verso piattaforme terze.', 110),

  -- progetto_integrazione (system integration / impiantistica ICT)
  ('progetto_integrazione', 'finanziaria',   'Stato avanzamento lavori: elenco commesse aperte con SAL, corrispettivo, costi sostenuti/da sostenere e margine a finire.', 10),
  ('progetto_integrazione', 'finanziaria',   'Criteri di riconoscimento ricavi su commessa (percentuale di completamento vs commessa completata) e WIP a bilancio.', 20),
  ('progetto_integrazione', 'finanziaria',   'Crediti per fatture da emettere e anticipi da clienti: aging e riconciliazione con lo stato commesse.', 30),
  ('progetto_integrazione', 'commerciale',   'Portafoglio ordini e backlog: ordini firmati non ancora eseguiti, per committente e data prevista di esecuzione.', 40),
  ('progetto_integrazione', 'commerciale',   'Concentrazione committenti e ricorrenza: quota ricavi da clienti ripetuti vs gare/spot negli ultimi 3 esercizi.', 50),
  ('progetto_integrazione', 'legale',        'Contenziosi e riserve su commesse (attivi e passivi), garanzie prestate, fideiussioni e polizze decennali in essere.', 60),
  ('progetto_integrazione', 'legale',        'Penali per ritardo previste nei contratti attivi e storico penali applicate negli ultimi 3 esercizi.', 70),
  ('progetto_integrazione', 'operativa',     'Subappaltatori e partner ricorrenti: contratti, qualifiche, quota di lavoro esternalizzato per commessa.', 80),
  ('progetto_integrazione', 'operativa',     'Certificazioni aziendali richieste dal mercato (ISO, SOA, qualifiche vendor): titolarità, scadenze e requisiti di mantenimento.', 90),
  ('progetto_integrazione', 'organizzazione','Capacità produttiva: organico tecnico per specializzazione, saturazione e dipendenza da figure chiave di commessa.', 100),

  -- rivendita_var (distribuzione / reselling a valore aggiunto)
  ('rivendita_var', 'commerciale',   'Accordi distributivi con i vendor: durata, esclusive territoriali/di prodotto, condizioni di rinnovo e clausole di change of control.', 10),
  ('rivendita_var', 'commerciale',   'Margini per linea di prodotto/vendor e loro evoluzione negli ultimi 3 esercizi, con separazione hardware/software/servizi.', 20),
  ('rivendita_var', 'commerciale',   'Rebate, sconti obiettivo e marketing fund dai vendor: meccanismi, maturazione e quota sul margine complessivo.', 30),
  ('rivendita_var', 'finanziaria',   'Magazzino: composizione, rotazione, obsolescenza e politiche di svalutazione; merce in conto deposito o conto visione.', 40),
  ('rivendita_var', 'finanziaria',   'Politiche di reso verso vendor e verso clienti: diritti contrattuali, storico resi e relativo impatto economico.', 50),
  ('rivendita_var', 'finanziaria',   'Ciclo di cassa: termini medi incasso/pagamento, affidamenti e assicurazione crediti su clienti principali.', 60),
  ('rivendita_var', 'legale',        'Contratti di supporto/manutenzione rivenduti: obblighi assunti verso il cliente vs copertura back-to-back del vendor.', 70),
  ('rivendita_var', 'commerciale',   'Concentrazione fornitori: quota acquisti dai primi 3 vendor e alternative disponibili a parità di listino.', 80),
  ('rivendita_var', 'organizzazione','Rete commerciale: agenti/venditori, portafogli clienti individuali, patti di non concorrenza e provvigioni.', 90),
  ('rivendita_var', 'operativa',     'Scadenzario rinnovi contratti di supporto e licenze in portafoglio nei prossimi 24 mesi.', 100),

  -- software_prodotto (ISV / prodotto proprietario)
  ('software_prodotto', 'tecnologia',    'Titolarità del codice sorgente: contributi di collaboratori esterni/ex dipendenti, cessioni dei diritti, depositi ed eventuali software escrow.', 10),
  ('software_prodotto', 'tecnologia',    'Componenti open source utilizzate: inventario, licenze (copyleft?) e compliance rispetto al modello di distribuzione.', 20),
  ('software_prodotto', 'tecnologia',    'Architettura e debito tecnico: stack, roadmap di ammodernamento, dipendenze da piattaforme o runtime obsoleti.', 30),
  ('software_prodotto', 'commerciale',   'Metriche prodotto: ARR/MRR, churn licenze, net revenue retention, pricing e sconti effettivi per segmento.', 40),
  ('software_prodotto', 'commerciale',   'Base installata: versioni in esercizio presso i clienti, quota su release correnti vs legacy da migrare.', 50),
  ('software_prodotto', 'legale',        'Contratti di licenza/SaaS: clausole di reversibilità dei dati, SLA, limitazioni di responsabilità e change of control.', 60),
  ('software_prodotto', 'legale',        'Proprietà intellettuale registrata: marchi, brevetti, domini; contenziosi IP attivi o minacciati.', 70),
  ('software_prodotto', 'finanziaria',   'Costi di sviluppo capitalizzati: criteri, ammortamenti e quota di R&S spesata vs capitalizzata negli ultimi 3 esercizi.', 80),
  ('software_prodotto', 'organizzazione','Team di sviluppo: figure chiave, retention, documentazione del prodotto e bus factor sulle componenti core.', 90),
  ('software_prodotto', 'operativa',     'Infrastruttura di erogazione (per SaaS): hosting, costi cloud per cliente, sicurezza, backup e ultimi penetration test.', 100),
  ('software_prodotto', 'commerciale',   'Canali di vendita: diretta vs partner/reseller, accordi OEM/white label e relative dipendenze.', 110)
ON CONFLICT (family, question) DO NOTHING;

COMMIT;

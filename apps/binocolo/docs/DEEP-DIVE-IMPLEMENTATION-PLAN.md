# Binocolo — Deep-dive redesign: piano di implementazione

> Esecuzione delle decisioni in [`DEEP-DIVE-DECISIONS.md`](./DEEP-DIVE-DECISIONS.md)
> (brainstorming 2026-07-03). Branch `poc/aenad`. Migrazioni su `ANISETTA_DSN`,
> **applicate a mano dall'utente** (numerazione da 091 — ricontrollare al momento
> del deploy se altri lavori hanno consumato numeri). Nessuna operazione diretta
> sul DB condiviso; nessuna spesa IT-full negli smoke (motori testati su payload
> archiviati); LLM live solo per lo smoke di Fase 5 (costo in centesimi, azione
> dell'utente).

## Principi

- **Ogni fase è spedibile e verificabile da sola.** Fase 0 ferma la perdita di
  dati e sblocca la validazione empirica; Fase 1 è la fondazione (zero costo,
  zero cambi visibili); le fasi 2-6 cambiano il prodotto in ordine d'impatto
  (B → C+D → A → E → F).
- **Fonte canonica dei numeri da prezzo = CEE**, con provenienza dichiarata;
  i ratio vendor restano per i semafori dove riconciliano.
- **Rollout sulla cache esistente** via `recompute` (esteso): ogni
  ricalibrazione del motore raggiunge le aziende già analizzate senza
  ricomprare nulla.
- Test Go solo per trasformazioni non banali, **proposti e da approvare**
  (regola repo); UI verificata con smoke su dev server attivo (playwright-cli
  + bypass, riusare il server già in esecuzione).

## Rettifica rispetto al documento decisioni

La famiglia di business model **non** può vivere in `ma_company_fact`: la mig
090 ratifica (PRD Iniziative §6) che i fact sono presentation-only e "NEVER
read by gate/routing/scoring", e il `kind` è CHECK-vincolato. La famiglia è
invece un input analitico di soglie e valuation → **tabella propria**
`binocolo.ma_company_bm_family` sul pattern di `ma_company_domain` (mig 082):
`company_key` PK, snapshot vat/nome, audit, campi suggestion vs ratifica.
`DEEP-DIVE-DECISIONS.md` §5.2 è aggiornato di conseguenza.

## Repo-fit (checklist `docs/IMPLEMENTATION-PLANNING.md`)

- **Runtime/dev/deployment fit**: n/a — nessuna nuova app, tutto dentro
  `backend/internal/binocolo` + `apps/binocolo`. Nessun nuovo env var.
- **Auth fit**: tutti i nuovi endpoint dietro `app_binocolo_access` come gli
  esistenti (`handle(...)` in `RegisterRoutes`, `handler.go:131-163`);
  l'export IRL riusa il pattern POST autenticato di `handleExportMASession`
  (`handler.go:1268`) — nessun download link-based.
- **Data-contract fit**: identificatori esistenti (`company_key` da
  `maTargetDedupeKey`; card = `(initiative_id, company_key)` con ownership
  verificata da `requireOperationalInitiativeCard`, riusata per thesis-reading
  e IRL). Vintage payload = PK `(company_key, balance_sheet_date)`.
- **Osservabilità**: ogni nuovo endpoint emette trace event (`maTraceEventWrite`
  pattern); errori interni sanitizzati via `maFailure` come oggi.
- **Migrazioni**: `.sql` idempotenti in `deploy/migrations/`, applicate
  dall'utente; ogni fase elenca le sue e l'ordine.
- **Coesistenza**: cache globale, worker state machine, scoring/rank, gate UC2
  **non cambiano**. Le strutture JSONB esistenti (scorecard/valuation/brief)
  si estendono con campi nuovi opzionali: le righe vecchie restano leggibili,
  il refresh avviene via `recompute`/`regenerate-briefs`.

## Sequenza e dipendenze

```
Fase 0 (vintage + inspect)      ── urgente, indipendente
Fase 1 (strato CEE + fix ROE)   ── fondazione                ← dopo 0 (usa inspect per calibrare)
Fase 2 (B: bridge + banda)      ← dopo 1
Fase 3 (C+D: lente + famiglie)  ← dopo 1 (classifier usa CEE), tocca valuation di 2
Fase 4 (A: momentum + coorte)   ← dopo 1; indipendente da 2/3
Fase 5 (E: lettura di tesi)     ── indipendente da 1-4 (usa artefatti esistenti)
Fase 6 (F: IRL)                 ← dopo 2/3/5 per le fonti (degrada bene con un sottoinsieme)
```

---

## Fase 0 — Versioning payload + endpoint di ispezione

> **STATO 2026-07-03: IMPLEMENTATA, ATTIVATA E CHIUSA** — build/vet/gofmt
> verdi; smoke fixture verde; mig 091 applicata dall'utente; inspect eseguito
> su n=10: 10/10 IIC-only (divisione PL mai osservata, refusi legend in tutti),
> granularità 8 dettaglio / 2 SENZA array debiti (→ la catena di fallback con
> provenienza è necessaria, non teorica), riconciliazione EBITDA 10/10 a 0,00%
> e PFN 6/6 entro 0,03% (regge anche su PFN negative), L2Y assoluti 10/10,
> employeeTrend 8/10, grossFinancialDebt 7/10, B.12/13 sempre zero (questione
> definitoria immateriale; sentinella = flag di riconciliazione), vintage
> backfill 10 righe/10 aziende.
> Nota implementativa: l'archivio vintage è scritto dal worker PRIMA della
> pipeline di analisi (best-effort, ON CONFLICT DO NOTHING), non dentro
> `SaveMADeepReady` come da bozza — così una vintage fallita (es. migrazione
> non applicata) non blocca mai il salvataggio dell'analisi pagata.

**Obiettivo:** fermare la perdita delle vintage (oggi `SaveMADeepReady`
sovrascrive `itfull_payload`) e rendere possibile la validazione empirica di
§1/§8 del documento decisioni senza toccare il DB a mano.

**Migrazione 091** `091_binocolo_ma_deep_payload_vintage.sql`:
- Tabella `binocolo.ma_deep_payload_vintage` (`company_key text`,
  `balance_sheet_date date`, `turnover_year int`, `payload jsonb`,
  `fetched_at timestamptz`, PK `(company_key, balance_sheet_date)`).
- **Backfill**: `INSERT ... SELECT` dalle righe `ready` di `ma_deep_analysis`,
  estraendo `balance_sheet_date` da
  `COALESCE(itfull_payload->'data'->'ecofin'->>'balanceSheetDate', itfull_payload->'ecofin'->>'balanceSheetDate')`
  (payload con e senza envelope `{data:...}`); righe senza data → saltate
  (log del conteggio nel commento della migrazione). Le vintage correnti sono
  così al sicuro prima di qualsiasi refresh.

**Backend:**
- `SaveMADeepReady` (ma_store.go): dopo il salvataggio, `INSERT ... ON
  CONFLICT (company_key, balance_sheet_date) DO NOTHING` nella vintage (stesso
  bilancio ricomprato ≠ duplicato; bilancio nuovo = riga nuova).
- `GET /binocolo/v1/ma/deep/inspect` (nuovo, read-only, pattern degli endpoint
  di manutenzione `handleRecomputeMADeep`): aggregati sui payload in cache —
  mix codici IIC vs IPL, presenza dettaglio D (split vs totali), copertura
  campi `development.*`/`employees.employeeTrend`/`operatingResults.*L2Y`,
  presenza B.12/13 ≠ 0, distribuzione scarto vendor-vs-CEE su EBITDA e PFN
  (calcolo inline, senza persistere). Risposta JSON; trace event
  `ma_deep_inspect`.

**Verifica:** migrazione idempotente (doppia applicazione ok); `go build`/`vet`;
inspect eseguito dall'utente in dev → i numeri decidono le calibrazioni di
Fase 1/2 (soglia riconciliazione, necessità fallback granularità).

**Done:** nessuna vintage può più andare persa; le 5 incognite empiriche del
documento decisioni hanno un canale di risposta.

---

## Fase 1 — Strato di lettura CEE (fondazione)

> **STATO 2026-07-03: IMPLEMENTATA** — `ma_deep_cee.go` (lettura con
> provenienza, catena dettaglio→totale→ratio vendor, PFN con derivati passivi,
> prudenziale, risultato con semantica 177/178/179) + `ma_deep_cee_labels.go`
> (279 etichette GENERATE dalla legend, refusi normalizzati 231/232/351) +
> `MADeepReconciliation` sullo scorecard (EBITDA/PFN in % relativo, ROE in
> punti; attaccata da `buildMADeepScorecard`, la UI la ignora) + parametro
> `vendor_cee_tolerance_pct` in maPricing (default compilato 1.0, seed
> esplicito in mig 092) + inspect rifattorizzato sulla lettura unica (fonte
> singola, niente doppia definizione PFN). Golden test approvati verdi sulle
> due fixture + fallback (totals-only→cee_total, famiglia IPL, vendor_ratio,
> payload senza CEE→nil); build/vet/gofmt ok; suite deterministica del
> pacchetto verde (httptest non eseguibili in sandbox, pre-esistente).
> Rollout sulla cache: `POST /ma/deep/recompute` quando si vuole (la
> riconciliazione appare sulle righe cached; nessun effetto UI fino a Fase 2).

**Obiettivo:** un modulo che legge gli array CEE con entrambe le famiglie di
codici e catena di fallback con provenienza. Zero costo, nessun cambio UI.

**Backend — nuovo file `ma_deep_cee.go`:**
- Normalizzazione codici: indice `{IIC*,IPL*} → voce semantica` (mappa
  generata dalla legend **per intero** — 276 codici IIC; estratto di
  riferimento in `artifacts/claude/iic_legend_map.json`; le due famiglie
  condividono il numero). Parsing difensivo: entrambe le famiglie accettate, e
  i 3 refusi della legend propagati dall'API (`IPL231`, `IPL232`, `IICC351`)
  inclusi **verbatim** come codici canonici (sono C.16.a interessi da
  collegate/controllanti e "central pool management").
- Accessor per voce con **provenienza** (`cee_detail` | `cee_total` |
  `vendor_ratio`): es. debiti banche = IIC094+IIC095 se presenti, altrimenti
  IIC332, altrimenti derivazione vendor.
- Derivazioni: PFN contabile (D.1+D.2+D.3+D.4+D.5 **+ derivati passivi
  IIC219** − cassa IIC070 − titoli IIC065; la definizione vendor include
  IIC219, verificato su CDLAN) + split breve/oltre; debito lordo; TFR
  (IIC089); fondi (IIC085-088); soci (IIC331/184/185); EBITDA CEE (IIC130 −
  IIC149 + IIC144); EBITDA prudenziale (− IIC127 − IIC129); pesi A.5/EBITDA,
  B.8/ricavi, partecipazioni (IIC019/attivo, C.15 IIC150/EBITDA); variazione
  rimanenze B.11 (IIC145); risultato: ante imposte IIC177, imposte IIC178,
  utile netto IIC179 (semantica verificata su entrambi i payload).
- Riconciliazione vendor-vs-CEE (EBITDA, PFN, ROE = IIC179/IIC084): scarto %
  calcolato e trasportato nello scorecard (campo nuovo), soglia di allerta da
  `ma_parameter` (default calibrato sull'output dell'inspect di Fase 0).
  Nota: il ROE vendor riconcilia (il presunto quirk era una lettura errata di
  IIC178, che è l'imposta) — la riconciliazione resta come sanity check, il
  semaforo continua a usare il ratio vendor.

**Test proposti (da approvare, estendono `ma_deep_engine_test.go`):** golden
test su **due fixture reali** (vedi "Decisioni aperte" §a):
- MFT (micro, impiantistica, 2024): PFN = 14.682, EBITDA CEE = 94.934 =
  vendor, TFR = 100.573, soci = 10.000, prudenziale = reported (A.4/contributi
  zero), B.11 = −115.701, utile netto = 486, ROE = 0,60%.
- CDLAN (media, TLC/DC, 2025): debito lordo = 4.922.617 (con IIC219 = 7.241),
  PFN = 1.666.053, split banche 1.762.969/3.152.407, EBITDA CEE = 2.528.884 =
  vendor (B.10.d = 7.944 incluso), TFR = 294.528, prudenziale = 2.499.509
  (contributi 29.375), ciclo negativo, magazzino zero, partecipazioni
  7.693.063 = 49% attivo, C.15 = 1.100.000, utile netto = 2.171.236, ROE =
  27,57%.
- Casi limite: voci mancanti, solo totali, EBITDA ≤ 0, payload vuoto.

**Done:** ogni numero da prezzo ha una fonte CEE con provenienza; le
definizioni vendor sono pinnate da riconciliazioni esatte su due fixture.

---

## Fase 2 — Filone B: equity bridge + banda asimmetrica + flag qualità

> **STATO 2026-07-03: IMPLEMENTATA** — mig 092 (6 parametri: tfr_bridge,
> soglie flag A.5/B.8/partecipazioni, tolleranza vendor-CEE=1%);
> `buildMADeepValuation` v2 (banda asimmetrica su prudenziale con fallback
> EV/Sales sull'estremo basso + clamp, `LowMethod`/`PrudentialEbitda`
> trasparenti) + `buildMADeepBridge` (PFN con provenienza, TFR pesato, fondo
> imposte, soci come riga negoziale informativa) + `buildMADeepQualityFlags`
> (6 flag deterministici con evidenza e domanda DD, warning prima di info);
> PFN canonica dello scorecard = lettura CEE (fallback vendor per payload
> senza CEE); worker e `recompute` estesi (body `{"valuation": true}` →
> ricostruisce anche la valuation, `UpdateMADeepValuation`); export con
> colonne PFN/TFR; UI su TUTTE e tre le superfici (modal sessione con tabella
> bridge + flag stilizzati, card dossier in parità, dossier P.IVA con righe
> bridge nel valGrid + flag). Test: 7 nuovi golden verdi (degenerazione MFT,
> proporzionalità sintetica, fallback EV/Sales, EBITDA≤0, bridge CDLAN al
> centesimo, flag su entrambe le fixture) + suite deterministica verde; tsc
> pulito sui file toccati (errori pre-esistenti solo in
> IniziativaBoardPage/RicerchePage, estranei); smoke UI su dev server attivo
> (dossier CDLAN, valuation vecchio formato → ramo fallback ok, console
> pulita). **ATTIVATA 2026-07-03**: mig 092 applicata, recompute con
> `{"valuation": true}` eseguito (10 righe); dossier CDLAN verificato via API
> (valori identici ai golden: EV 10.693.024–14.637.054 su Telecom 7,19×,
> equity 8.732.443–12.676.473, riconciliazione a zero) e visivamente (bridge
> EV−PFN−TFR=Equity, entrambi i flag con evidenza e domanda DD). Nota: il
> `valuationRationale` del brief LLM cached narra ancora i numeri della
> valuation vecchia — si riallinea con `POST /ma/deep/regenerate-briefs`
> (10 chiamate LLM, centesimi), a discrezione dell'utente.

**Obiettivo:** il numero che l'IC guarda diventa onesto: bridge esplicito,
banda ancorata all'EBITDA prudenziale, flag di confidenza fuori da banda e RAG.

**Migrazione 092** `092_binocolo_ma_bridge_parameters.sql` — seed
`ma_parameter` (ON CONFLICT DO NOTHING): `tfr_bridge_pct=100`,
`a5_ebitda_flag_pct=20`, `b8_revenue_flag_pct=8`,
`participation_assets_flag_pct=25`, `participation_income_flag_pct=20`,
`vendor_cee_tolerance_pct=1` (inspect Fase 0 su n=10: rumore max 0,03% da
rounding del ratio; un mismatch definitorio vero è in scala percentuale).

**Backend:**
- Tipi (`ma_types.go`): `MADeepBridge` (righe: EV low/high, −PFN [provenienza],
  −TFR, −fondo imposte, ±soci [riga separata], = equity low/high; ogni riga
  con valore e fonte); scorecard esteso con `QualityFlags []MADeepQualityFlag`
  (code, severity, evidenza numerica, domanda DD standard) e
  `Reconciliation` (scarti vendor-CEE).
- Engine (`ma_deep_engine.go`): `buildMADeepValuation` v2 — estremo alto su
  EBITDA reported ×(1+0.15), estremo basso su EBITDA prudenziale ×(1−0.15);
  prudenziale ≤ 0 o sotto `ebitda_fallback_threshold` → estremo basso su
  EV/Sales (riusa la logica fallback esistente); bridge costruito dai valori
  CEE di Fase 1. Flag deterministici: A.4>0, A.5/EBITDA>soglia, B.8/ricavi>
  soglia, scarto riconciliazione>soglia, PN≤0 (già esistente, diventa flag),
  **perimetro standalone** (partecipazioni IIC019/attivo > soglia oppure C.15
  IIC150/EBITDA > soglia → caveat "la banda valuta il perimetro standalone,
  controllate escluse" + domanda DD consolidato/somma delle parti; caso CDLAN:
  49% dell'attivo e 43% dell'EBITDA fuori perimetro).
- Rollout: `recomputeMADeepScorecards` esteso per ricostruire **anche la
  valuation** (oggi la lascia intatta di proposito — il commento a
  `ma_service.go:2534` va aggiornato: bridge e banda cambiano semantica, il
  refresh è voluto). Flag `{"valuation": true}` sul body per distinguere i due
  usi.

**Frontend (`TargetPage.tsx` DeepAnalysisTab + `IniziativaCardDossierPage.tsx`
in parità):**
- Tabella bridge nel blocco "Inquadramento di valore" (righe autoportanti,
  soci evidenziata come riga negoziale).
- Banda asimmetrica mostrata com'è (niente ±% simmetrico nel copy).
- Flag qualità = annotazioni di confidenza sul blocco valuation (canale visivo
  separato dal RAG) con domanda DD inline.
- Export `exportSession`: colonne bridge (equity low/high, PFN, TFR) — le
  caveat viaggiano nell'XLSX.

**Verifica:** test engine su fixture (banda MFT: degenerazione ±15% con
A.4/contributi zero; caso sintetico con A.4>0 → asimmetria proporzionale);
smoke UI su dev server attivo (tab deep + card dossier + export); `pnpm
--filter mrsmith-binocolo exec tsc --noEmit`.

**Done:** equity bridge visibile e esportabile; banda che si allarga con la
distorsione misurata; nessun flag dentro la matematica.

---

## Fase 3 — Filoni C+D: lente compratore, famiglie, multipli

> **STATO 2026-07-03: IMPLEMENTATA** — mig 093 (`ma_company_bm_family`:
> suggested vs ratified), 094 (righe `FAMILY:*` nella stessa
> sector_valuation_multiple: Computer Services 12,03×, Engineering/
> Construction 9,71×, Retail Distributors 11,64×, Software 20,85× — valori da
> docs/sector_valuation_multiples.json, vintage confermata), 095 (24 soglie
> famiglia + 5 parametri haircut graduato 35/30/20 con soglie 5M/20M).
> Backend: lente compratore (leverage/capitalizzazione/current/acid/ROE →
> tier "contorno", escluse dall'overall RAG), `suggestBMFamily` (ATECO
> univoci + euristica struttura costi CE per il blob 62/63, con evidenza),
> percorso condiviso `computeMADeepScorecard`/`resolveMADeepValuation`
> (worker + recompute + regenerate), endpoint PUT
> `/ma/companies/{companyKey}/bm-family` (ratifica/revoca con trace),
> famiglia nel dossier P.IVA. **Raffinamento di design rispetto alla bozza**:
> la riga famiglia scavalca il prefisso SOLO dove il prefisso è fuorviante
> (`maFamilyFirstPrefixes`: 43/46/58/62/620/6201/63) — un prefisso specifico
> corretto (61 → Telecom 7,19×) è un comparable migliore del proxy di
> famiglia (senza questa regola CDLAN sarebbe salita immotivatamente a
> Computer Services 12,03×, +67%); altrove famiglia = fallback pre-TOTAL.
> **Rettifica fattuale**: la mig 039 aveva già granularità 4 cifre su 62xx
> (6202/6203/6209 → Computer Services) — il bug "tutto il 62 a 20,85×" era
> più circoscritto di quanto scritto nel brainstorm; resta il disallineamento
> identità-vs-codice che la famiglia risolve. **Cambio semantico ratificato
> nei test**: KRAL (equity negativa, patrimoniale tutto rosso) passa da red a
> amber overall — l'insolvenza urla dal flag patrimonio_eroso e dai contorno
> rossi, non dal semaforo core; MFT resta red ma per ragioni core (ROS 1,56%,
> ricavi in calo), col margine 7,77% che da rosso diventa ambra nelle soglie
> progetto. UI: gruppo "Struttura finanziaria del venditore" subordinato su
> 3 superfici; sezione "Business model" con ratifica sul dossier P.IVA.
> Test: 6 nuovi (classificatore su fixture+sintetici, ordine risoluzione,
> soglie famiglia, contorno, haircut tiers, caveat) tutti verdi; tsc pulito.
> **Da fare per attivarla**: mig 093-095, poi recompute
> `{"valuation": true}` (le famiglie vengono suggerite e applicate); ratifica
> analista dal dossier.

**Obiettivo:** il semaforo risponde all'acquirente; una classificazione
alimenta soglie e riga Damodaran; haircut graduato.

**Migrazione 093** `093_binocolo_ma_company_bm_family.sql`:
`binocolo.ma_company_bm_family` — `company_key text PK`, snapshot
vat/tax/nome, `family text CHECK IN
('servizi_ricorrenti','progetto_integrazione','rivendita_var','software_prodotto')`,
`source text CHECK IN ('ateco','cost_structure','analyst')`, `evidence text`
(es. "B.6/produzione = 45%"), `suggested_family text`, `ratified_by_email`,
`ratified_at`, audit. Pattern mig 082.

**Migrazione 094** `094_binocolo_ma_family_multiples.sql`: righe famiglia
nella **stessa** `binocolo.sector_valuation_multiple` con chiave sintetica
`ateco_prefix = 'FAMILY:servizi_ricorrenti'` ecc. (la colonna è testo, il
longest-prefix match resta intatto). Valori Computer Services ed
Engineering/Construction estratti da `docs/psEurope.xls` /
`docs/vebitdaEurope.xls` (stesso dataset Damodaran già in uso); Retail
(Distributors) e Software (System & Application) già presenti come righe 46/62.

**Migrazione 095** `095_binocolo_ma_family_thresholds.sql` — seed
`ma_parameter` con chiavi piatte `rag_{metric}_{ok|good}_{family}` per
margine EBITDA, ROS, ciclo finanziario (valori della tabella in
DEEP-DIVE-DECISIONS.md §5.3) + `haircut_pct_tier1/2/3` e
`haircut_tier1_max_eur=5000000`, `haircut_tier2_max_eur=20000000`.

**Backend:**
- Classificatore deterministico (in `ma_deep_cee.go`): da ATECO dove non
  ambiguo (46.51→rivendita, 43.2→progetto, 61→servizi); per 62/63 suggerimento
  da struttura costi CE (B.6/produzione>40%→rivendita; magazzino+risconti→
  progetto; A.4 rilevante→software; personale/VA alto→servizi) con evidenza
  testuale. Il worker salva il suggerimento in `ma_company_bm_family`
  (`source='ateco'|'cost_structure'`, non ratificato).
- Endpoint `PUT /binocolo/v1/ma/companies/{companyKey}/bm-family` (ratifica/
  override analista, trace event `ma_bm_family_ratified`).
- Engine: soglie per famiglia da `maPricing` esteso (pattern
  `maPricingFromParameters`, `ma_service.go:683`); metriche **core vs
  contorno** — `deepOverallRAG` v2 guidato solo da core (PFN+TFR/EBITDA,
  margine, cash-conversion, trend); leverage/capitalizzazione/current/acid
  marcate `tier:"contorno"` nel `MADeepMetric` (campo nuovo) e escluse
  dall'aggregazione.
- `ResolveSectorMultiple` (ma_store.go:3737): prova `FAMILY:{family}` (se
  ratificata o suggerita — caveat diverso) prima della catena prefissi.
- Haircut graduato: `buildMADeepValuation` sceglie il tier dal fatturato.
- Caveat eterogeneità: famiglia non ratificata + prefisso in lista dispersa
  (62/63/46) → caveat testuale sul blocco valuation.

**Frontend:** blocco "Famiglia di business model" nel dossier (suggerita +
evidenza + ratifica/override un-click); metriche di contorno rese visivamente
subordinate (sezione "struttura finanziaria del venditore"); ConfigPage
mostra le nuove chiavi (già dinamica sui parametri).

**Verifica:** test classificatore su fixture (MFT → progetto_integrazione via
ATECO 43.2) + casi sintetici per il blob 62; test risoluzione multiplo
famiglia→prefisso→TOTAL; smoke UI ratifica; `recompute` con valuation refresh
su cache dev.

**Done:** MFT esce "nella norma del suo modello" invece che "Critico"; un MSP
sotto 62 non è più prezzato a 20.85×.

---

## Fase 4 — Filone A livello 0: qualità del margine / momentum

**Obiettivo:** i segnali già pagati entrano nel dossier (mai nel rank).

**Backend (engine):**
- Gruppo scorecard `qualita_margine`: ΔEBITDA assoluto (da
  `operatingResults.ebitdaL2Y`), cash-conversion (B.11 vs EBITDA), delta YoY
  vendor (debito lordo, attivo, VA, organico — attenzione scale miste
  documentate: `mol` %, `ebitVariation` frazione).
- RAG su **combinazioni** (EBITDA↓+debito↑ → rosso; ricavi↑+VA↓ → ambra con
  nota "deriva rivendita/body"), non su singoli delta.
- Percentile di coorte: **prerequisito da verificare** — quali KPI espone lo
  stadio Advanced per i target di sessione; se i margini non ci sono,
  percentile solo su fatturato/produttività. Coorte fotografata alla data
  della ricerca, annotazione informativa nel dossier. Se il prerequisito
  fallisce, l'item esce dalla fase senza bloccarla.
- Rollout via `recompute`.

**Frontend:** gruppo nuovo nel DeepAnalysisTab + card dossier; copy asciutto
(dati, non aggettivi).

**Verifica:** test combinazioni su fixture + sintetici; smoke UI.

**Done:** il dossier risponde a "quanto è vero questo EBITDA" senza toccare il
funnel.

---

## Fase 5 — Filone E: lettura di tesi context-scoped

**Obiettivo:** il memo per (iniziativa, azienda), on-demand, ancorato alla
sessione di provenienza.

**Migrazione 096** `096_binocolo_ma_card_thesis_reading.sql`:
`binocolo.ma_card_thesis_reading` — PK `(initiative_id, company_key)`,
`session_id` (provenienza usata), `thesis_snapshot text`, `reading jsonb`,
`web_evidence_date`, `model_id`/`prompt_id` FK, audit + `generated_by_email`.

**Migrazione 097** `097_anisetta_mrsmith_binocolo_thesis_reading_prompt.sql`:
seed su **`mrsmith.llm_prompt`** (app `binocolo`, scope `ma_thesis_reading`,
pattern post-cutover mig 053/084 — NON il vecchio schema binocolo di 040/045)
— prompt con le regole ferree:
fit che cita i fatti; ri-pesatura delle flag esistenti (mai nuove); domande DD
di tesi additive; sinergie etichettate ipotesi; **nessun numero nuovo**; "la
tesi non si esprime su X" quando manca. Binding modello: **chiedere all'utente
la riga live del registry** (il seed non fa fede, convenzione nota).

**Backend:**
- Scope `maModelScopeThesisReading` in `ma_types.go`.
- `POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/thesis-reading`
  (+ GET): ownership via `requireOperationalInitiativeCard`; risoluzione tesi
  = provenienza col rating più recente (`MACardProvenance`), fallback
  `CreatedFromSession`; card senza provenienza → 409 con messaggio chiaro.
  Input LLM: scorecard + valuation + brief neutro + evidenza web (con data) +
  tesi. Audit `llm.CallAudit` (pattern `buildMADeepBriefLLM`,
  `ma_deep_worker.go:215`); trace `ma_thesis_reading_generated`.
- Staleness: la risposta GET include `staleThesis: bool` (snapshot ≠ tesi
  corrente della sessione di provenienza); rigenerazione solo esplicita.
- **Pulizie brief neutro** (stesso giro): prompt v3 che rinomina
  `thesisReading`→`financialReading` (migrazione update in place come 045),
  struct aggiornata con lettura di entrambe le chiavi per le righe cached,
  `ThesisFit` rimosso; rollout `regenerate-briefs`.

**Frontend (`IniziativaCardDossierPage.tsx`):** tab "Lettura di tesi" —
posizionata dopo i fatti (anchor risk), bottone genera/rigenera con stato
stale, data evidenza web visibile.

**Verifica:** smoke su una card reale in dev (1 chiamata LLM, centesimi,
azione utente); 401/403; card senza provenienza; `tsc`.

**Done:** due iniziative leggono la stessa azienda con la propria tesi; il
dossier neutro resta intatto.

---

## Fase 6 — Filone F: Information Request List

**Obiettivo:** flag e domande diventano l'artefatto che esce dal tool.

**Migrazione 098** `098_binocolo_ma_card_irl.sql`:
- `binocolo.ma_card_irl_item` — `id uuid PK`, `(initiative_id, company_key)`
  idx, `category text`, `question text`, `source text CHECK IN
  ('flag','brief','thesis','template','analyst')`, `source_ref text` (per il
  re-seed additivo), `status text CHECK IN
  ('aperta','chiesta','risposta','na')`, `position int`, audit.
- `binocolo.ma_irl_template` — `family text`, `category`, `question`,
  `position`; seed ~10-15 voci per famiglia (MSP: ricorrente contrattualizzato,
  churn, SLA; software: ownership IP, escrow, licenze; progetto: SAL,
  contenziosi; rivendita: accordi distributivi, resi).

**Backend:**
- `POST .../cards/{companyKey}/irl/seed`: assembla da flag deterministici
  (Fase 2, `source_ref` = flag code), brief neutro, lettura di tesi (se
  esiste), template della famiglia (ratificata; senza famiglia → solo fonti
  restanti). Re-seed = inserisce solo `source_ref` nuovi — **mai toccare le
  voci esistenti**.
- CRUD voci (`POST`/`PATCH`/`DELETE` + reorder), stato per voce; ownership
  card sempre verificata; trace events.
- `POST .../irl/export`: XLSX via pattern `exportSession`
  (`ma_service.go:1482`) — colonne categoria/domanda/stato/fonte.
- L'IRL **sopravvive all'archiviazione** della card (nessun CASCADE dalla
  card; la chiave è (initiative_id, company_key)).

**Frontend:** sezione IRL sul card dossier — lista raggruppata per categoria,
chip stato, add/edit inline, bottone seed (con conteggio proposte additive),
export.

**Verifica:** seed → curatela → re-seed additivo (le modifiche sopravvivono);
export apre in Excel; smoke UI; 401/403.

**Done:** il kick-off DD parte da un export del tool invece che da un foglio
bianco.

---

## Test proposti (APPROVATI dall'utente 2026-07-03)

1. Golden test strato CEE su fixture payload reale (Fase 1) — il più
   importante: pinna le riconciliazioni verificate a mano.
2. Test banda asimmetrica + bridge (Fase 2): degenerazione, proporzionalità,
   fallback EV/Sales, EBITDA ≤ 0.
3. Test classificatore famiglia (Fase 3): ATECO univoci + blob 62 sintetici.
4. Test risoluzione multiplo famiglia→prefisso→TOTAL (Fase 3).
5. Test combinazioni momentum (Fase 4).
(Niente test per CRUD IRL, UI, copy — smoke manuale.)

## Decisioni aperte — stato al 2026-07-03

- **(a) Fixture payload nel repo — RISOLTA**: ok dell'utente; entrambe le
  fixture in `backend/internal/binocolo/testdata/` (`itfull_mft_2024.json`,
  `itfull_cdlan_2025.json`).
- **(b) Modello per `ma_thesis_reading` (Fase 5) — RISOLTA (2026-07-03)**:
  dal registry live, `ma_deep_brief` ha binding dedicato → `openai/gpt-5.5`,
  `params {}` (nessuna evidenza di un default d'app). Decisione: **binding
  dedicato in parità** — mig 097 semina anche `mrsmith.llm_model` per
  (binocolo, ma_thesis_reading) con `model` = quello live di ma_deep_brief e
  `provider_id` copiato dalla sua riga default (`INSERT ... SELECT
  provider_id, model FROM mrsmith.llm_model WHERE app='binocolo' AND
  scope='ma_deep_brief' AND is_default`), `params {}`; il `max_tokens` di
  fallback vive nel codice come per il deep brief.
- **(c) Valori Damodaran famiglia — CONFERMATA**: i due XLS in `docs/` sono la
  vintage 2026-01-05 della mig 039; Computer Services ed
  Engineering/Construction si estraggono da lì in Fase 3.
- **(d) Soglia `vendor_cee_tolerance_pct` — RISOLTA (inspect 2026-07-03,
  n=10)**: EBITDA 10/10 a scarto 0,00%, PFN 6/6 entro 0,03% (rounding, regge
  anche su PFN negative) → soglia **1%** (30× il rumore osservato).

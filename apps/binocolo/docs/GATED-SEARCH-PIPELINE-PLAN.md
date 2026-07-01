# Binocolo — Gated Search Pipeline: piano di implementazione

> Nuova pipeline di ricerca aziende M&A: **Address €0.01 → gate UC2 (keep/forse/salta) → Advanced €0.10 + scoring solo sui sopravvissuti**. Branch `poc/aenad`. Migrazioni su `ANISETTA_DSN` (schema `binocolo`/`mrsmith`), applicate a mano dall'utente. Sostituisce *di fatto* il valore dell'execute attuale (Advanced-su-tutto → punteggio spesso scollegato dal business), ma **convive** con esso come modalità distinta finché non è validata.
>
> Contesto e decisioni: vedi memoria `project_binocolo_new_pipeline`, `project_binocolo_uc2_primary_gate`, `project_binocolo_shared_job_queue`. Mappa codice verificata 2026-07-01.

## Obiettivo

Oggi: `enqueueExecute` → `runExecution` fa il prodotto cartesiano (provincia × ateco × forma), IT-search con `DataEnrichment:"advanced"` (€0.10) su **tutta** la superficie, poi `scoreMATargetsV2`. La web-analysis ha mostrato che quel punteggio spesso non riflette *cosa fa davvero* l'azienda rispetto al settore-obiettivo → lista poco significativa.

Nuova pipeline: l'Advanced+scoring si pagano **solo** sulle aziende che il gate semantico UC2 giudica rilevanti (keep/forse). Il gate gira su dati Address (€0.01) + risoluzione dominio + scrape. Risultato: lista finale con punteggio solo su aziende sul bersaglio, a costo minore.

## Decisioni bloccate (2026-07-01)

- **Scope fase 1** = solo riordino funnel, riuso dello scoring attuale sui sopravvissuti. Scoring v2 (sector-fit come dimensione) è fase separata dopo.
- **Modalità affiancata** all'execute attuale (POC, confronto su sessioni reali).
- **Rete ATECO allargata** (il gate a valle rende sicuro includere codici adiacenti/dubbi).
- **Spesa Advanced automatica** (niente checkpoint a metà) → controllo costi **tutto a monte** sullo stadio `surface` (dry-run + estimate + cap).
- **Coda condivisa = step 0**, prima di spendere davvero (doppia esecuzione = doppi soldi).

## Cost model (costi unitari confermati)

Address €0.01/azienda · Advanced €0.10/azienda · fastcrw scrape+crawl $0.001/pagina · fastcrw search $0.001/ricerca · LLM gate (gpt-oss) trascurabile. **Costo gate ≈ €0.020/azienda** (Address ~metà; scrape/crawl ~€0.008-0.012 con crawl fino a 8 pag; search ~€0.001-0.002). **Break-even vs Advanced-su-tutto: conviene finché sopravvivenza keep+forse < ~80%** — l'eval dà salta ~60-75% → s~25-40% → 45-55% più economico *e* più rilevante. Il gate è un "biglietto d'ingresso €0.02": solo i sopravvissuti pagano €0.10 → la larghezza superficie si disaccoppia dal costo. Governatore residuo con auto = **cap sulla superficie** (gate scala lineare: N=5000 → €100 di solo gate).

---

## Step 0 — Ownership della coda `ma_job` (PREREQUISITO, prima di spendere)

> **STATO 2026-07-01: IMPLEMENTATO (staged, non ancora attivo).** Codice + migrazione 073 pronti; build/vet/test verdi. Attivazione (human): applicare 073, riavviare (owner = hostname di default, override `BINOCOLO_INSTANCE_OWNER`). Nessun test SQL unitario aggiunto: il repo testa la logica via fake delle interfacce, non la SQL grezza → correttezza del predicato verificata a DB (due processi con owner diversi, vedi Verifica).

**Problema.** `ma_job` vive nell'Anisetta condiviso. `locked_by` è un lease effimero (`uuid.NewString()` per-processo, `ma_job_worker.go:42`): non impedisce a un worker estraneo (istanza dev/staging altrui con codice vecchio) di **acquisire** il lease e girare un job con codice diverso → "doppie esecuzioni". Su questa pipeline = spesa Advanced+scrape doppia.

**Fix — owner stabile + PRE-LEASE all'insert + claim a due fasi** (tutto lato Go, coerente con il codice esistente):

1. **Config**: nuovo campo `InstanceOwner` (`backend/internal/platform/config/config.go`), env `BINOCOLO_INSTANCE_OWNER`, **default `os.Hostname()`**. Ogni laptop/deploy ha un owner distinto e stabile tra i restart; in prod k8s si setta esplicito (uguale su tutti i pod della stessa release) per non orfanare tra pod.
2. **Migrazione 073**: `ALTER TABLE binocolo.ma_job ADD COLUMN owner text;` + estende `ma_job_type_check` per il nuovo tipo (vedi Step 3); indice `ma_job_pending_idx` esteso a filtrare per owner. Backfill: righe esistenti con `owner IS NULL`.
3. **Enqueue PRE-LEASATO** (`EnqueueMAJob`, `ma_job_store.go:45`): l'insert stampa **già** `owner = InstanceOwner`, `locked_by = InstanceOwner` (sentinella owner) e `lease_until = now() + lease`. Il job **nasce già leasato all'owner corrente**. + `RETURNING id` per il ticket.
   - **Perché è questa la parte che conta** (nota utente): i worker **vecchi** non leggono la colonna `owner` — girano codice e query vecchi. Un filtro `owner = $me` solo nel codice nuovo NON li ferma. L'unico modo per escluderli da tick 0 è far risultare **falso il *loro* predicato** di claim `lease_until IS NULL OR lease_until < now() OR locked_by = <loro_uuid>`: con `lease_until` futuro e `locked_by = InstanceOwner ≠ loro_uuid`, è falso → **saltano la riga**. Zero dipendenza dal fatto che i vecchi capiscano il nuovo schema.
4. **Claim a due fasi** (`ListMAJobs`/`AcquireMAJobLease`, `ma_job_store.go`). `w.id` resta uuid effimero per la concorrenza tra repliche. Il worker nuovo claima `WHERE owner = $myOwner AND (locked_by = $myOwner /*fresh, pre-leasato non ancora preso da un worker concreto*/ OR locked_by = $myWorkerUuid /*già mio, refresh*/ OR lease_until < now() /*scaduto, crash recovery*/)`, poi **CAS** del lease sul proprio uuid (`SET locked_by = $workerUuid, lease_until = now()+lease`). Repliche dello stesso owner si serializzano atomicamente sull'UPDATE (una sola vince). Le righe legacy (`owner IS NULL`, senza pre-lease) restano gestite dal branch `owner IS NULL`.

**Nota transitoria.** La protezione è **totale** quando tutti i worker sono a codice nuovo (il filtro `owner` esclude i job altrui). Nella finestra di migrazione, il pre-lease copre il caso owner-vivo; un worker vecchio potrebbe riprendere una riga **solo** dopo scadenza del lease **e solo** finché esistono ancora worker vecchi → lease abbastanza lungo/rinfrescato dal worker owner rende la finestra trascurabile. Se l'owner muore, l'orfano (dopo scadenza) è il comportamento voluto: meglio non-eseguito che eseguito da codice sbagliato.

**Perché INSERT e non stored procedure**: nel repo le uniche function sono trigger/helper puri (nessun "comando applicativo"); una submission-proc metterebbe logica di business nel DB condiviso → ricrea la deriva multi-versione al layer enqueue, e ogni revisione sarebbe una migrazione in lock-step coi deploy. Il ticket lo dà `INSERT ... RETURNING id`. La logica di enqueue resta versionata col binario.

- **Agent**: config + migrazione 073 + enqueue pre-leasato + claim a due fasi + test store. Build/vet/test verdi.
- **Human**: applica 073, setta `BINOCOLO_INSTANCE_OWNER` (se non basta l'hostname), riavvia.
- **Verifica** (test store): (a) un job appena inserito ha `lease_until` futuro + `locked_by=owner`; (b) un claim con predicato *vecchio* (uuid diverso) **non** lo prende; (c) il worker owner lo prende e fa CAS del lease sul proprio uuid; (d) alla scadenza, solo un worker stesso-owner lo recupera. Smoke con due processi a owner diverso.

---

## Step 1 — Parametri costo + Estimate v2

**Migrazione 074** — nuovi parametri in `binocolo.ma_parameter` (oggi c'è solo `cost_advanced_eur`): `cost_address_eur` (0.01), `cost_scrape_page_eur` (0.001), `cost_search_eur` (0.001). `INSERT ... ON CONFLICT DO NOTHING`, idempotente. `loadPricing` (`ma_service.go:289`) li legge con fallback ai default compilati.

**Estimate v2** — oggi `probeMASearchSurface` (`ma_service.go:4437`) stima `count × cost_advanced`. Nuova formula per la pipeline gated:

```
costo_certo   = N × (cost_address + gate_scrape_search_stimato)   # gate_scrape_search ≈ €0.011
costo_atteso  = N × tasso_sopravvivenza × cost_advanced           # tasso da default param (seed 0.35), poi calibrato
```

Il dry-run resta a costo *address* (non advanced). L'estimate ritorna: N superficie, costo certo (gate) e banda di costo attesa (Advanced su survivor-rate min/mid/max, es. 25/35/50%). `runExecution`/probe già in `advanced` → parametrizzare il livello (vedi Step 2). Il `tasso_sopravvivenza` di default è un parametro `ma_parameter` (`survivor_rate_default`), aggiornabile dai run reali.

- **Agent**: migrazione 074 + estimate v2 (probe address-cost, formula a due parti, banda).
- **Human**: applica 074.
- **Verifica**: estimate su una sessione reale ritorna N + bande coerenti; nessuna spesa (solo dry-run).

---

## Step 2 — Address-only search

Estrarre da `runExecution` (`ma_service.go:4510`) una variante che chiama IT-search con `DataEnrichment:"address"` invece di `"advanced"` (il gancio è a `ma_service.go:4903/4919`) e popola `ma_target` con **sola identità** (name/vatCode/taxCode/province/town/GPS/atecoCode), **senza** scoring.

**Migrazione 075** — `ma_target` per lo stadio address:
- `score`, `match_state` resi `NULL`-abili (oggi required/valorizzati dallo scoring).
- nuova colonna `enrichment_level text` (`'address'|'advanced'`), default `'advanced'` per compatibilità con le righe execute esistenti.
- il gate scrive comunque su `ma_target_web_validation` (chiave `session_id, company_key`, già esistente).

`parseMATargetsFromVendorData` (`ma_vendor.go:20`) gestisce già la mappatura identità dal payload Address (i campi `address.registeredOffice.*` ci sono). Verificare che i campi finanziari mancanti (turnover/employees) restino nil senza rompere il parsing.

- **Agent**: migrazione 075 + `runExecution` variante address + persistenza identità (`ReplaceMATargets`/upsert con `enrichment_level='address'`, score/match_state null).
- **Human**: applica 075.
- **Verifica**: address-pass su una superficie piccola reale → righe `ma_target` con identità, score null, `enrichment_level='address'`. Costo ≈ N × €0.01.

---

## Step 3 — Orchestratore multi-stadio (nuovo `job_type = 'gated_search'`)

Nuovo tipo di job che incolla gli stadi con **checkpoint durevole** (per resume dopo restart e per progresso visibile). Lo stage vive sul run (o su una colonna del job payload/`ma_execution_run`), così un restart riparte dallo stadio giusto senza rifare i precedenti.

Stadi eseguiti dal worker (`runGatedSearchJob`, sul modello di `runExecuteJob` `ma_service.go:820`):
1. `surface` — dry-run (address-cost) + cap check. Se N > cap → fail con errore chiaro (nessuna spesa).
2. `address` — Step 2, popola identità.
3. `gate` — riusa il batch UC2 **esistente** `buildMAWebValidation` (`ma_web_validation_job.go:303`) su tutti i target address (concurrency già presente, default 4 → alzabile via param). Scrive `final_action` = keep(`confirm`)/forse(`deprioritize`/`needs_*`)/salta(`reject`). **Dominio-non-risolto → forse, mai salta** (già policy recall-safe). Checkpoint per-azienda (le righe si scrivono man mano).
4. `enrich_score` — per i target con bucket ∈ {keep, forse}: `GetITAdvanced` by-VAT (`openapiit/company.go:53`) → aggiorna `ma_target.vendor_payload` + `enrichment_level='advanced'` → `scoreMATargetsV2` (`ma_scoring.go:42`, riuso puro) → score/match_state/flags/evidence. I `salta` restano identity-only (nascosti di default, tenuti per audit/telemetria).
5. `ready` — sessione `completed`.

**Migrazione**: estende `ma_job_type_check` con `'gated_search'` (parte della 073, cfr. precedente estensione per `webvalidation`). Anti-doppione: unique `(session_id, job_type)` in volo già copre.

Handler: nuovo `POST /binocolo/v1/ma/sessions/{id}/gated-search` → `enqueueGatedSearch` → 202 + ticket. La UI polla lo stato come per estimate/execute; lo stato riporta lo stadio corrente e i contatori keep/forse/salta man mano.

- **Agent**: job type + worker orchestratore + checkpoint di stadio + handler + registrazione in `newMAJobWorker` (`ma_job_worker.go:40`).
- **Human**: nessuna nuova migrazione oltre 073 (job_type) — riavvio.
- **Verifica**: submission reale su superficie piccola → job attraversa gli stadi, progresso visibile, riparte se il worker riavvia a metà. Costo bounded dal cap.

---

## Step 4 — Advanced+score sui sopravvissuti (dentro Step 3.4, isolato per test)

Già descritto in 3.4; lo isolo come deliverable testabile: la funzione "enrich+score un set di company_key" deve essere pura e ri-eseguibile (idempotente sui `ma_target`), così un re-run non ri-paga l'Advanced se già presente (guardia su `enrichment_level='advanced'` + freshness, cfr. `ma_target_web_validation` freshness mig 055).

- **Verifica**: dato un set di keep/forse, l'Advanced+score aggiorna solo quelli; ri-esecuzione non ri-spende.

---

## Step 5 — Rete ATECO allargata

Nessun nuovo motore: le leve sono **già parametriche** in `ma_parameter` (mig 061), lette da `maAtecoRetrievalConfigFromParameters`. Allargare = abbassare `ateco_retrieval_rel_threshold` (0.75), `core_ratio` (0.92), `floor` (0.30) e alzare `cap` (12), via `PUT /binocolo/v1/ma/parameters` — **nessun deploy**. Leva-codice fine opzionale in `resolveKBFitToCandidates` (`ma_ateco_retrieval.go`, includere qualche distractor/escludere meno).

Approccio: **misurare col gate**. Su una sessione reale, allargare progressivamente e osservare (a) quante aziende in più entrano, (b) quante il gate promuove keep/forse vs salta, (c) il costo. Si tiene l'allargamento che recupera target senza far esplodere il costo-gate.

- **Agent**: eventuale leva-codice in `resolveKBFitToCandidates` (solo se i parametri non bastano).
- **Human**: tuning parametri via API + osservazione.
- **Verifica**: A/B stretta-vs-larga sulla stessa sessione, con salta-rate e costo a confronto.

---

## Step 6 — UI

- Intake perimetro: campo per **più dettaglio sugli ambiti di attività** delle aziende cercate (alimenta `strategy.sector`/`perimeterConcepts` = il "metro di giudizio" del gate → migliora direttamente la qualità del gate).
- Submission gated: pulsante distinto dall'execute attuale (modalità affiancata), con estimate v2 (N + bande di costo) e conferma prima di lanciare.
- Vista di avanzamento: stadio corrente + contatori keep/forse/salta live.
- Lista finale: keep/forse scorate (bucket come asse visibile), salta nascoste ma ispezionabili (audit/telemetria recall-safety, sul modello dei `domainOutcome` diagnostics esistenti).

- **Verifica**: smoke UI (dev server già attivo, no secondo server) su una submission reale piccola.

---

## Recall-safety a scala (trasversale)

`salta` è l'unico bucket che nasconde per sempre un'azienda → deve restare ad **alta precisione**. Invarianti:
- dominio-non-risolto → forse (mai salta).
- keepLeak (true-keep→salta) = metrica inviolabile; validato 0 su 54 aziende, va **monitorato a scala** (contatori nel report + campionamento con l'harness eval che abbiamo tenuto).
- i `salta` persistiti alimentano la telemetria: salta-rate alto = buon targeting; forse-rate alto = perimetro vago (segnale da riportare all'utente nell'intake).

## Migrazioni (riepilogo, in ordine)

- **073** — `ma_job.owner` + estensione `ma_job_type_check` (`gated_search`) + indice pending per owner.
- **074** — parametri costo (`cost_address_eur`, `cost_scrape_page_eur`, `cost_search_eur`, `survivor_rate_default`).
- **075** — `ma_target`: `score`/`match_state` nullable + `enrichment_level`.

Tutte idempotenti, target Anisetta, applicate a mano dall'utente. Nessuna operazione diretta sul DB da parte dell'agent (solo file `.sql`).

## Sequenza consigliata

Step 0 → 1 → 2 → 3(+4) → validazione su superficie piccola reale → 5 (allargamento misurato) → 6 (UI). Ogni step è build/vet/test verde e verificabile in isolamento. Lo scoring v2 è **fuori** da questo piano (fase successiva).

## Rischi / note

- **Coda condivisa**: step 0 mitiga il furto-lease; resta la buona pratica di non lasciare istanze dev orfane puntate sull'Anisetta con codice vecchio.
- **Validazione = spesa reale**: ogni submission reale paga Address+scrape+Advanced. Iniziare con superfici piccole (poche decine) per bounare il costo.
- **WriteTimeout 60s**: tutto async via `ma_job`, nessun endpoint sincrono lungo (già rispettato dal modello estimate/execute).
- **`GetITAddress`/`GetITAdvanced`** esistono già lato client; verificare i limiti rate di OpenAPI.it su un address-pass largo.

# Binocolo — Veryshort: preferiti + analisi approfondita (fase 3)

> Stato: **design consolidato, decisioni complete** — brainstorming 2026-06-16 (branch `poc/aenad`). Tutti gli 8 punti aperti risolti. Non ancora implementato.
> Memoria correlata: `project_binocolo_veryshort_favorites`. Vedi anche `docs/IMPLEMENTATION-PLANNING.md` (repo-fit) e `docs/IMPLEMENTATION-KNOWLEDGE.md`.

## 1. Obiettivo

Completare il funnel M&A con lo stadio **veryshort**: l'analista marca i target con un rating a stelle (memoria della preferenza **e** selezione), e su quelli avvia un'**analisi approfondita** a pagamento basata sul bilancio CEE completo (`IT-full`), con un **brief LLM** da analista. Il rating è il gate della spesa incrementale.

### Funnel e costi (3 gradini)

| Stadio | Cosa | Costo | Quando si paga |
|---|---|---|---|
| Longlist | filtri server-side OpenAPI.it | dry-run €0.01/ricerca | stima |
| Shortlist | IT-search `advanced` + `scoreMATargetsV2` | €0.10/azienda | execute (tutta la run — sunk) |
| **Veryshort** | `IT-full` + scorecard + brief LLM | **€0.30/azienda** | solo sui rated ≥1★, al lancio esplicito |

Il rating controlla **solo** il costo incrementale di IT-full. L'advanced è già speso.

## 2. Decisioni fissate

1. **Scala rating** (intero singola colonna): `-1` escluso · `0`/assente = non valutato · `1·2·3` stelle (1★ interessante, 2★ promettente, 3★ prioritario). Tre livelli positivi.
2. **Sort**: `rating DESC, score DESC` → stelle in testa, non-valutati al centro per score, esclusi in coda.
3. **Esclusione = gesto separato** (icona, riga grigia/strike), non "0 stelle".
4. **La stella è preferenza + selezione**: niente checkbox. **Gate deep-dive = qualsiasi rating ≥1★**; `-1`/`0` mai approfonditi.
5. **Persistenza voto su identità azienda**, scoped alla sessione (`(session_id, company_key)`). Obbligatorio: `ReplaceMATargets` cancella e ricrea i target a ogni execute, quindi un voto legato a `ma_target.id` morirebbe.
6. **Spesa solo al click esplicito** "Approfondisci preferiti" (stellinare resta gratis/reversibile), con cancello di costo. Batch manuale; le cachate non rientrano nell'addebito.
7. **IT-full async** (timeout 30s + fallback async del vendor) → worker di background.
8. **Due motori**: ratio finanziari deterministici in Go + narrativa LLM sulla scorecard già calcolata (l'LLM non fa aritmetica).
9. **Modello brief**: scope `ma_deep_brief`, `openai/gpt-5.5` ("OpenAI: GPT-5.5"), default server-side.
10. **Valutazione**: tabella curata multipli **Damodaran Europa** (EV/EBITDA + fallback EV/Sales), mappata su divisione ATECO; equity implicito = EV − PFN.
11. **Sconto PMI e prezzi vendor in tabella parametri** editabile da pagina di configurazione. Default sconto 30%.

### 2.1 Risoluzioni degli 8 punti aperti (2026-06-16)

1. **Mapping ATECO→Damodaran**: granularità **2 cifre di default, 3 cifre solo sugli straddle** (es. 62, 25; financial 64–66 → EV/Sales / Total-Market-without-financials). LLM propone dalla lista fissa, analista veta nel PR. Curatura in fase impl.
2. **Job async**: **worker di background** (pool di goroutine) con stato durevole in DB + **sweep di resume all'avvio**. Callback webhook = futuro.
3. **Schema brief**: **l'LLM restituisce solo i campi narrativi**; scorecard + valuation + governance li possiede e fonde Go (nessun numero dall'LLM).
4. **Ruolo config**: **nessun ruolo elevato per ora** → pagina dietro `app_binocolo_access`. Mitigazione = audit su ogni modifica + reversibilità. `app_binocolo_manager` rimandato (aggiungibile senza rework).
5. **Posizione brief**: **nuovo 5° tab "Analisi approfondita"**, che diventa il tab **di default quando l'analisi è pronta** (placeholder di stato altrimenti).
6. **Scorecard fase 3 → score fase 2**: **NO**. I percentili dello score presuppongono popolazione uniforme (tutti su `advanced`); alimentarlo coi dati full dei pochi approfonditi romperebbe la coerenza del ranking. Il loop è **umano**: il brief fa ri-stellinare l'analista, non ricalcola lo score.
7. **Export XLSX**: **(b) colonne di sintesi** aggiunte all'XLSX esistente, popolate dove c'è analisi. Dossier per-azienda col brief completo = futuro.
8. **`ebitda_fallback_threshold`**: floor margine EBITDA **default 5%**, **configurabile** in `ma_parameter` (e EBITDA ≤ 0 ripiega sempre su EV/Sales).

## 3. Modello dati (migrazioni 036→)

Sketch indicativi (DDL definitivo in implementazione). Schema `binocolo`.

### 036 — `ma_target_rating` (voto per azienda, per sessione)
```
session_id   uuid    NOT NULL  -- FK ma_session
company_key  text    NOT NULL  -- vat normalizzata > tax > vendor (riusa maTargetDedupeKey)
rating       smallint NOT NULL  -- -1 escluso, 1..3 stelle (assenza riga = non valutato/0)
rated_by_subject text, rated_by_email text, rated_at timestamptz
PRIMARY KEY (session_id, company_key)
```
Al render: `LEFT JOIN` sui target per `(session_id, company_key)`, `COALESCE(rating,0)`. Sort `COALESCE(rating,0) DESC, score DESC`.

### 037 — `ma_deep_analysis` (artefatto fase 3, **globale per azienda**)
```
company_key      text PRIMARY KEY
vat_code text, tax_code text
status           text  -- queued | running | ready | failed
vendor_request_id text -- id job async IT-full (CreateITFullRequest)
itfull_payload   jsonb -- raw IT-full
scorecard        jsonb -- ratio calcolati in Go
brief            jsonb -- structured output LLM (solo narrativa)
valuation        jsonb -- EV/equity range + provenienza (calcolato in Go)
cost_eur         numeric
model_id uuid, prompt_id uuid -- audit
error_code text
created_at, updated_at, refreshed_by_email
```
Cache globale: la stessa azienda promossa in due sessioni paga IT-full una volta. Lo `status` durevole abilita il resume del worker dopo un restart.

### 038 — `ma_parameter` (config business)
```
key text PRIMARY KEY, value text, value_type text, label text, description text,
updated_by_email text, updated_at timestamptz
```
Seed: `cost_advanced_eur=0.10`, `cost_full_eur=0.30`, `cost_dryrun_eur=0.01`, `budget_default_eur=50`, `sme_haircut_pct=30`, `ebitda_fallback_threshold=5` (floor margine EBITDA %). Sostituisce le costanti lette da `decorateMACost`/cancelli. Default globale; override per-sessione resta nel JSONB strategia (come `MaxBudgetEUR`).

### 039 — `sector_valuation_multiple` (Damodaran + mapping)
```
ateco_prefix text  -- 2 o 3 cifre
sector_label text
damodaran_industry text
ev_ebitda numeric, ev_sales numeric, n_firms int
source text, source_date date
```
Caricata da JSON curato in repo (accanto a `codici_ateco_2025.json`). Righe `Total Market` / `Total Market (without financials)` come fallback universali.

### 040 — modello+prompt `ma_deep_brief`
Inserisce riga modello (`scope=ma_deep_brief`, `model=openai/gpt-5.5`, name "OpenAI: GPT-5.5", default) + prompt di default (come 026/034).

## 4. Rating preferiti (UI/comportamento)

- Controllo a 3 stelle per riga in shortlist + azione **Escludi** separata (icona). Stellinare/escludere = `POST .../targets/{companyKey}/rating` (upsert), **ottimistico**: sort e filtro client-side istantanei, persistenza in background.
- Filtri: "solo preferiti (≥1★)", "nascondi esclusi". Compongono coi filtri flag esistenti.
- Re-sort non a ogni click (jank): client-side immediato ma stabile, oppure a refresh esplicito — da rifinire in UI.

## 5. Fase 3 — analisi approfondita

### Trigger e costo
"Approfondisci preferiti" → cancello di costo (`N` aziende rated≥1★ **non in cache** × `cost_full_eur` + token — conferma, riusa pattern `acknowledgeCost`/`errMAEstimateOverBudget`). Le cachate saltano fetch e calcolo (gratis).

### Job async (IT-full) — worker di background
Per azienda: `CreateITFullRequest` → riga `running` + `vendor_request_id`. Un **pool di goroutine** polla `CheckITRequest` fino a `ready`, poi calcola scorecard + brief e salva (`ready`/`failed`). Stato durevole in `ma_deep_analysis` → **sweep di resume all'avvio** sulle righe `running` (sopravvive ai restart). La UI mostra lo stato per riga (in coda / in corso / pronto / errore) e un re-lancio processa solo le nuove. Da definire in impl: intervallo di poll, concorrenza, backoff/retry, timeout di fallimento. Callback webhook del vendor = ottimizzazione futura (dev non riceve webhook dietro il proxy Vite).

### Due motori
- **Motore deterministico (Go)** — parsa i codici CEE di IT-full via mappa `codice→etichetta` derivata da `company-legend.html` (819 voci: SP `IC*` + CE `PL*`). Calcola: ROS, ROE, ROI, margine EBITDA; PFN, PFN/EBITDA, debiti/PN, indipendenza finanziaria; current/quick ratio, CCN; peso avviamento/intangibili; struttura debito breve/lungo; trend multi-anno; semaforo RAG; roster amministratori. Possiede anche **valuation** e **governance**. **Nessun numero dall'LLM.**
- **Motore narrativo (LLM, `ma_deep_brief`)** — riceve la scorecard **già calcolata** + tesi + campi qualitativi e produce **solo i campi narrativi**: `{verdict{headline,rag,thesisFit}, thesisReading, redFlags[]{severity,claim,ddQuestion}}`. Go fonde questa narrativa con scorecard/valuation/governance per il brief finale. L'LLM interpreta e cita campi/anni, non calcola.

### Contenuto brief (sezioni)
1. Verdetto esecutivo (allineato alla tesi) + headline RAG
2. Scorecard finanziaria (ratio + semaforo + trend)
3. Lettura per tesi
4. Red flag + domande di due diligence
5. Inquadramento di valore (vedi §6)
6. Governance & controllo (amministratori + cap table + UBO via `GetITUBO`)
7. Provenienza (citazioni a codici/anni)

### Export
L'XLSX esistente prende **colonne di sintesi** (`Analisi sì/no`, `EV stimato`, `Equity stimato`, `Multiplo`, `Margine EBITDA`, `PFN/EBITDA`, `RAG`, `Verdetto`), popolate solo dove c'è analisi. Il dossier per-azienda col brief narrativo completo è un futuro.

## 6. Valutazione

- Fonte: **Damodaran Europa**, foglio "Industry Averages", aggiornato annualmente (gen). File già acquisiti: `vebitdaEurope.xls`, `psEurope.xls`.
- Multiplo: `EV/EBITDA` colonna **"Only positive EBITDA firms"**; **fallback `EV/Sales`** quando margine EBITDA < `ebitda_fallback_threshold` (default 5%) o EBITDA ≤ 0 o settore senza EBITDA significativo (financial → `NA`).
- Mapping: **divisione ATECO → industria Damodaran** (lista fissa ~94 nomi). LLM propone dalla lista, analista veta. Ambigue (62, 25) a 3 cifre o con scelta motivata. Fallback a `Total Market (without financials)`.
- Catena: `EV = EBITDA × multiplo × (1 − sme_haircut)`; **equity implicito = EV − PFN** (PFN da IT-full). Ogni numero cita fonte + data + sconto + `n_firms` (avviso sui settori a basso campione).

## 7. Parametri & pagina di configurazione

Tabella `ma_parameter` editabile da una pagina di config dentro l'app binocolo. Leve di **business** (prezzi vendor, budget gate, sconto PMI, soglia fallback EBITDA); le meccaniche interne (cap probe sottoalbero, soglie ATECO) restano in codice. **Per ora nessun ruolo elevato**: pagina dietro `app_binocolo_access`; mitigazione = **audit su ogni modifica** (trace event) + valori reversibili. `app_binocolo_manager` aggiungibile dopo senza rework (i prezzi autorizzano spesa).

## 8. Contratti API (nuovi/variati)

| Metodo | Path | Scopo | Auth |
|---|---|---|---|
| POST | `/binocolo/v1/ma/sessions/{id}/targets/{companyKey}/rating` | upsert voto (`{rating}`) | access |
| POST | `/binocolo/v1/ma/sessions/{id}/deep-dive` | avvia batch IT-full sui rated≥1★ (`{acknowledgeCost}`) | access |
| GET | `/binocolo/v1/ma/sessions/{id}/deep-dive` | stato/risultati deep (o incluso nel detail) | access |
| GET/PUT | `/binocolo/v1/ma/parameters` | legge/aggiorna config | access (no ruolo elevato per ora) |

`MASessionDetail` esteso: ogni target porta `rating` + `deepStatus` (+ `brief`/`scorecard`/`valuation` quando pronti). `decorateMACost` legge i prezzi/budget dalla tabella parametri.

## 9. Repo-fit (checklist `IMPLEMENTATION-PLANNING.md`)

- **Runtime**: nessun nuovo mini-app; la pagina config è una route client *dentro* binocolo (hosting/deep-link già gestiti dalla SPA esistente).
- **Dev**: nessuna nuova porta Vite, nessun nuovo `dev:*`. Resta tutto nell'app binocolo.
- **Auth**: tutto dietro `app_binocolo_access` (config inclusa, nessun ruolo elevato per ora; `app_binocolo_manager` rimandato). Deep-dive e rating sono POST autenticati; audit sulle modifiche config. 401/403 nel test plan.
- **Data-contract**: PK rating `(session_id, company_key)`; `company_key` derivato col medesimo criterio di `maTargetDedupeKey`; cache IT-full globale `company_key`; le scritture rating verificano sessione esistente e operativa.
- **Deployment**: migrazioni 036–040 su `ANISETTA_DSN`; nessun nuovo env var (IT-full usa lo stesso client/chiave OpenAPI.it). Worker di polling: definirne ciclo di vita e resume.
- **Verifica**: vedi §10.

## 10. Strategia di verifica

- **Motore deterministico**: test Go su **payload IT-full di esempio/archiviati** (no spesa live). Verifica ratio attesi e fallback EV/Sales (sotto soglia margine / EBITDA≤0).
- **No spesa in smoke**: il DB di dev è condiviso/reale (`project_aenad_dev_db_is_shared`) → niente chiamate IT-full live €0.30 negli smoke; il gate live va dietro conferma esplicita.
- **Invariante chiave**: il rating sopravvive a un re-execute della sessione (ri-ricerca → target nuovi, voto ancora presente via `company_key`).
- **Cancello di costo**: batch sopra budget → 409 finché non `acknowledgeCost`; cachate escluse dall'addebito.
- **Async**: stato job coerente (running→ready/failed), resume all'avvio, idempotenza (re-lancio processa solo le nuove).
- **Export**: le colonne di sintesi compaiono solo per le righe analizzate, vuote altrove.

## 11. Dettagli implementativi residui (non decisioni)

- Curatura dell'artefatto di mapping ATECO→Damodaran (JSON in repo).
- Lista esatta dei campi dello structured output del brief (confine LLM/Go già fissato).
- Colonne esatte della sintesi deep nell'XLSX.
- Worker async: intervallo di poll, concorrenza, backoff/retry, timeout di fallimento.
- DDL definitivo delle migrazioni 036–040.

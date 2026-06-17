# Binocolo — Veryshort: piano di implementazione

> Esecuzione del design in [`VERYSHORT-DESIGN.md`](./VERYSHORT-DESIGN.md). Branch `poc/aenad`. Migrazioni su `ANISETTA_DSN` (come 033/034, applicate a mano).
>
> **STATO 2026-06-17: IMPLEMENTATO** — tutte le 6 fasi, con QA gate adversariale per fase (build/vet/test/tsc + review correttezza/convenzioni). `go build ./...`, `go vet`, `tsc --noEmit`, `gofmt` verdi; test Go deterministici del motore/valutazione/export verdi (httptest preesistenti vanno in panic solo nel sandbox per port-bind).
> **Da fare prima del deploy**: applicare migrazioni **036–040** su `ANISETTA_DSN`; validare il modello brief `openai/gpt-5.5` sul catalogo OpenRouter (il brief è best-effort: se il modello non risponde, scorecard+valutazione restano); **smoke live IT-full NON eseguito** — i path KPI sono derivati dallo schema `Full` di `company.openapi.json`, da calibrare sul primo payload reale (log "empty scorecard" se l'estrazione è vuota).
> Riferimenti codice verificati: `internal/binocolo/ma_store.go` (interfaccia `maWorkspaceStore` :15, `GetMASession` :215, `loadMATargets` :1265 con `ORDER BY score DESC` :1274, `ReplaceMATargets` :528 che cancella evidence per `target_id` poi target per `session_id`), `decorateMACost` (`ma_service.go:160`), `scoreMATargetsV2` (`ma_scoring.go:28`), `maTargetDedupeKey` (`ma_rules.go:424`), client `openapiit.CompanyClient` (`GetITFull`/`CreateITFullRequest`/`CheckITRequest` :37-45), `RegisterRoutes` Deps (`handler.go:34`, wired `cmd/server/main.go:549`), routing FE (`routes.tsx`, `App.tsx` TabNav).

## Principi

- **Ogni fase è spedibile** e verificabile da sola. Fasi 1 e 2 non costano nulla e danno valore subito; il rischio (spesa live IT-full) è isolato in Fase 3+.
- **`company_key`** = stessa derivazione di `maTargetDedupeKey` (vat normalizzata > tax > vendor > nome), centralizzata in un helper riusato da rating e deep-analysis.
- **Niente spesa live negli smoke** (`project_aenad_dev_db_is_shared`): i motori si testano su payload IT-full archiviati; la spesa reale è dietro conferma esplicita.
- Test: per le trasformazioni non banali (motore finanziario, valutazione) **propongo unit test Go**, da approvare prima di scriverli (regola test del repo).

## Sequenza e dipendenze

```
Fase 1 (Rating)        ─┐  indipendenti, in parallelo
Fase 2 (Parametri)     ─┘
Fase 3 (Deep infra: worker + scorecard)   ← dopo 2 (legge cost_full_eur/budget da parametri)
Fase 4 (Valutazione Damodaran)            ← alimenta lo scorecard di Fase 3
Fase 5 (Brief LLM)                        ← dopo 3 e 4 (LLM gira sulla scorecard+valutazione)
Fase 6 (Export sintesi XLSX)              ← dopo 3/4/5
```

---

## Fase 1 — Rating preferiti (end-to-end)

**Obiettivo:** stelle 3 livelli + esclusione, persistite per azienda, con sort/filtro. Nessun costo.

**Migrazione 036** `036_binocolo_ma_target_rating.sql`: tabella `binocolo.ma_target_rating` (PK `(session_id, company_key)`, `rating smallint`, attori+timestamp). FK su `ma_session(id)` ON DELETE CASCADE. **Non** legata a `ma_target` (che viene cancellato a ogni execute).

**Backend:**
- `company_key` helper condiviso (estratto da `maTargetDedupeKey`).
- `maWorkspaceStore`: `UpsertMATargetRating(ctx, sessionID, companyKey, rating, subject, email)` + lettura rating nella `loadMATargets` (LEFT JOIN su `(session_id, company_key)`), campo `Rating *int` su `MATarget`, sort `ORDER BY COALESCE(rating,0) DESC, score DESC` (:1274).
- Handler `POST /binocolo/v1/ma/sessions/{id}/targets/{companyKey}/rating` body `{rating}` (valida ∈ {-1,1,2,3}); verifica sessione operativa; trace event `ma_target_rated`.
- `MATarget` JSON: aggiungi `rating`.

**Frontend (`TargetPage.tsx`, `api/types.ts`):**
- `MATarget.rating?: number`.
- Componente `RatingStars` (3 stelle) + azione **Escludi** (icona) per riga shortlist; update **ottimistico** (stato locale immediato) + `api.post(...rating)` in background con rollback su errore.
- Sort client-side `rating desc, score desc`; riga esclusa grigia/strike; non ri-ordinare a ogni click (ordina a refresh o con transizione stabile).
- Filtri: "solo preferiti (≥1★)", "nascondi esclusi", componibili coi chip flag esistenti.

**Verifica:** voto persiste; **invariante chiave** = ri-esegui la sessione (execute → target ricreati) e il voto è ancora lì via `company_key`; sort/filtro corretti; 401/403.

**Done:** l'analista stella/esclude/ordina/filtra; il voto sopravvive ai re-run.

---

## Fase 2 — Tabella parametri + pagina di configurazione

**Obiettivo:** prezzi/budget/sconto fuori dal codice, editabili in UI.

**Migrazione 038** `038_binocolo_ma_parameter.sql`: tabella `binocolo.ma_parameter` (key PK, value, value_type, label, description, updated_by/at) + seed `cost_advanced_eur=0.10`, `cost_full_eur=0.30`, `cost_dryrun_eur=0.01`, `budget_default_eur=50`, `sme_haircut_pct=30`, `ebitda_fallback_threshold=5` (ON CONFLICT DO NOTHING per re-run idempotente).

**Backend:**
- `maWorkspaceStore`: `GetMAParameters(ctx) map[string]string`, `UpdateMAParameter(ctx, key, value, email)`.
- Cache in-memory dei parametri nel service con refresh (i prezzi cambiano di rado); fallback alle costanti `maCostPerCompanyEUR`/`maDefaultBudgetEUR` se la riga manca.
- **`decorateMACost`** (`ma_service.go:160`) legge `cost_*`/`budget` dai parametri invece che dalle costanti; `maStrategyBudget` resta override per-sessione.
- Handler `GET/PUT /binocolo/v1/ma/parameters` (dietro `app_binocolo_access`, **nessun ruolo elevato per ora**); ogni PUT → trace event `ma_parameter_updated` (audit).

**Frontend:**
- Nuova route `/config` in `routes.tsx` + voce in `navItems` (`App.tsx`).
- `ConfigPage` con form tipizzato sui parametri (number/percent/money), salvataggio per chiave.

**Verifica:** modifica `cost_full_eur` → si riflette nei calcoli/costo UI senza redeploy; audit registrato; valori fuori-range rifiutati.

**Done:** prezzi/budget/sconto gestibili da UI con audit.

---

## Fase 3 — Deep-dive: worker async + motore deterministico (senza brief)

**Obiettivo:** promuovere i rated≥1★ a IT-full, calcolare la scorecard finanziaria, persistere. Niente LLM ancora.

**Migrazione 037** `037_binocolo_ma_deep_analysis.sql`: tabella `binocolo.ma_deep_analysis` (PK `company_key` globale; `status`, `vendor_request_id`, `itfull_payload jsonb`, `scorecard jsonb`, `valuation jsonb` [vuoto in Fase 3], `cost_eur`, `model_id`/`prompt_id` [Fase 5], `error_code`, audit). Indice su `status` per il resume.

**Legend → mappa codici:** parser una-tantum di `apps/binocolo/docs/company-legend.html` (819 voci) → asset `binocolo` `codice→{labelIt,section}` (JSON embedded o tabella). Serve a motore e UI.

**Backend — API & gate:**
- `POST /binocolo/v1/ma/sessions/{id}/deep-dive` body `{acknowledgeCost}`: calcola i rated≥1★ **non già `ready` in cache**; `projected = count × cost_full_eur`; se `> budget && !ack` → `errMAEstimateOverBudget` (409); altrimenti enqueue (righe `queued`) e ritorna subito.
- `GET /binocolo/v1/ma/sessions/{id}/deep-dive` (o esteso in `GetMASession`): stato per `company_key`.
- `maWorkspaceStore`: `UpsertMADeepAnalysis`, `GetMADeepAnalysis(companyKey)`, `ListMADeepByStatus(status)` (resume), `ListMADeepForSession(sessionID)`.

**Backend — worker:**
- Pool di goroutine avviato da `RegisterRoutes` (ha store+openapiit) con `context` di processo; **sweep di resume all'avvio** su `status IN (queued,running)`.
- Per azienda: `CreateITFullRequest(vat/tax)` → salva `vendor_request_id`, `running`; polling `CheckITRequest` (intervallo configurabile, backoff, max tentativi → `failed` con `error_code`); a `ready` salva `itfull_payload`, calcola scorecard, salva `cost_eur`.
- Concorrenza bounded (es. 4); idempotenza: re-lancio processa solo le non-`ready`.

**Backend — motore deterministico (Go):**
- `ma_deep_engine.go`: parsa i codici CEE (`IC*` SP, `PL*` CE) dal payload IT-full via mappa legend; calcola ROS, ROE, ROI, margine EBITDA, PFN, PFN/EBITDA, debiti/PN, indipendenza finanziaria, current/quick ratio, CCN, peso avviamento/intangibili, struttura debito breve/lungo, trend multi-anno, RAG, roster amministratori. **Nessun LLM.**
- *Test proposti (in attesa di ok):* ratio attesi su 2–3 payload IT-full di esempio, casi limite (anni mancanti, valori nulli).

**Frontend:**
- Badge stato deep per riga (in coda/in corso/pronto/errore).
- Bottone **"Approfondisci preferiti"** + modale cancello di costo (riusa pattern `acknowledgeCost`).
- Polling/refresh stato; tab modale "Analisi approfondita" che mostra la **scorecard** (brief in Fase 5).

**Verifica:** gate 409 sopra budget; cachate escluse dall'addebito; worker `running→ready/failed`; **resume dopo restart**; motore corretto su payload archiviati; **nessuna chiamata live €0.30 negli smoke automatici**.

**Done:** l'analista lancia il batch, paga solo le nuove, vede la scorecard quando pronta.

---

## Fase 4 — Valutazione (Damodaran)

**Obiettivo:** range EV/equity per target analizzato, auditabile.

**Ingestione (una-tantum, fuori runtime):** script che estrae da `vebitdaEurope.xls`/`psEurope.xls` (già in `docs/`, CSV in `artifacts/claude/`) le colonne `Industry Name`, `Number of firms`, `EV/EBITDA` ("Only positive EBITDA firms"), `EV/Sales` → JSON curato in `docs/` con `source_date`. **Mapping ATECO→industria Damodaran** (2 cifre default, 3 cifre sugli straddle 62/25, financial→Total-Market-without-financials): LLM propone dalla lista fissa, vetato nel PR.

**Migrazione 039** `039_binocolo_sector_valuation_multiple.sql`: tabella + load dal JSON (`ateco_prefix`, `damodaran_industry`, `ev_ebitda`, `ev_sales`, `n_firms`, `source`, `source_date`).

**Backend:**
- `maWorkspaceStore`: `ResolveSectorMultiple(ateco)` (longest-prefix 3→2 cifre, fallback Total-Market).
- Nel motore: `EV = EBITDA × multiplo × (1 − sme_haircut)`; **fallback EV/Sales** se margine EBITDA `< ebitda_fallback_threshold` (param) o EBITDA≤0; **equity = EV − PFN**; popola `valuation` con metodo, multiplo, `n_firms`, sconto, fonte+data.
- *Test proposti:* selezione multiplo, fallback, `equity = EV − PFN`.

**Frontend:** sezione "Inquadramento di valore" nello scorecard, con provenienza e avviso settori a basso `n_firms`.

**Verifica:** valori coerenti col CSV; fallback corretto; ogni numero cita fonte+data+sconto.

**Done:** ogni target approfondito ha un range di valore difendibile.

---

## Fase 5 — Brief LLM (effetto wow)

**Obiettivo:** narrativa da analista sopra la scorecard, senza numeri inventati.

**Migrazione 040** `040_binocolo_ma_deep_brief_prompt.sql`: riga `binocolo.llm_model` (`scope=ma_deep_brief`, `model=openai/gpt-5.5`, name "OpenAI: GPT-5.5", `is_default=true`) + `binocolo.llm_prompt` default per lo scope (mirror di 026/034, `ON CONFLICT (scope,name)`).

**Backend:**
- `ma_deep_brief.go`: chiama il modello `ma_deep_brief` con la **scorecard+valutazione già calcolate** + tesi + campi qualitativi; structured output **solo narrativa**: `{verdict{headline,rag,thesisFit}, thesisReading, redFlags[]{severity,claim,ddQuestion}}`. Validazione/forzatura JSON come per la strategia.
- Go **fonde** narrativa + scorecard + valuation + governance (cap table/UBO via `GetITUBO`) → `brief` finale; salva `model_id`/`prompt_id` (audit); trace event `ma_deep_brief_completed`.
- Esegue nel worker subito dopo lo scorecard (stessa transizione `ready`).

**Frontend:** rendering completo del tab "Analisi approfondita" (7 sezioni); **diventa il tab di default quando `status=ready`**; citazioni a codici/anni.

**Verifica:** l'LLM non emette numeri (i numeri vengono solo da Go); brief coerente con la scorecard; modello/prompt configurabili via scope.

**Done:** brief completo, auditabile, front-and-center.

---

## Fase 6 — Export sintesi

**Obiettivo:** portare i numeri deep in Excel.

**Backend:** estendi `maExportRows` (`ma_rules.go:434`) con colonne popolate dove c'è analisi: `Analisi`, `EV stimato`, `Equity stimato`, `Multiplo`, `Margine EBITDA`, `PFN/EBITDA`, `RAG`, `Verdetto`. Vuote altrove. (Dossier brief completo = futuro.)

**Verifica:** parità — colonne valorizzate solo per le righe analizzate.

**Done:** shortlist + sintesi valutativa esportabile.

---

## Note trasversali (repo-fit)

- **Nessun nuovo mini-app**: niente modifiche a root `package.json`/`Makefile`/`catalog.go` per nuove app. Solo nuova route+nav *dentro* binocolo (Fase 2). `app_binocolo_manager` non introdotto ora.
- **Env**: nessuna nuova variabile; IT-full usa lo stesso client/chiave OpenAPI.it (`openapiitCli`).
- **Osservabilità**: ogni nuova operazione emette trace event (`ma_target_rated`, `ma_parameter_updated`, `ma_deep_dive_*`, `ma_deep_brief_completed`); errori interni sanitizzati al client, loggati lato server.
- **DI**: nuove dipendenze (worker) iniettate via `Deps`/`RegisterRoutes`, no stato globale di package.
- **Migrazioni**: 036–040 applicate a mano su `ANISETTA_DSN`, idempotenti (`IF NOT EXISTS`/`ON CONFLICT`).

## Checklist pre-merge per fase

- [ ] `go build ./...` + `go vet ./...`
- [ ] `pnpm --filter mrsmith-binocolo exec tsc --noEmit`
- [ ] migrazione applicata e ri-applicabile (idempotente)
- [ ] trace event presenti per le nuove operazioni
- [ ] smoke UI sulla fase (server dev già attivo, no riavvio)
- [ ] nessuna spesa live IT-full nei test automatici

# Statistiche RDA — Implementation Plan (v0)

Mini-app della categoria **Acquisti** per ricercare e visualizzare gli ordini
(RDA / PA) dell'archivio storico `arak` (schema `pa`). v0 = **Archivio PA**
(search + filtri + scheda dettaglio a 8 sezioni). La pagina **RDA** (richieste
nuove, sistema RDA corrente) è presente come placeholder "in costruzione".

---

## Comparable Apps Audit

### Reference 1 — `apps/panoramica-cliente` (master-detail read-only su DB)
File ispezionati:
- `apps/panoramica-cliente/src/App.tsx` — AppShell + TabNavGroup, gate `APP_ACCESS_ROLES['panoramica-cliente']` con `AccessNotice`.
- `apps/panoramica-cliente/src/pages/OrdiniDettaglioPage.tsx` — toolbar filtri (SingleSelect/MultiSelect/SearchInput) + "Cerca" che triggera la query, `useTableFilter` per filtro client-side, tabella con righe-gruppo cliccabili, `SlideOverPanel` come detail (tab `testata`/`riga`/`righe`/`storico`), `useCsvExport`, `Intl.NumberFormat('it-IT', EUR)` per importi, `ServiceUnavailable` su 503, badge stato colorati.
- `apps/panoramica-cliente/src/routes.tsx` — rotte piatte con `Navigate` index.

Pattern riusati:
- Toolbar filtri dimensionali + SearchInput + bottone "Cerca" che attiva la query (modello "filtri → azione → risultati", non ricerca live su ogni keystroke).
- Detail come slide-over/pannello laterale con tab internali.
- `useTableFilter` per filtraggio client-side della tabella risultati.
- `Intl.NumberFormat('it-IT', currency EUR)` per gli importi.
- `ServiceUnavailable` + stati empty/loading/error espliciti.
- Export CSV (differito a v2, ma l'hook è già in famiglia).

Pattern scartati:
- L'ordinamento per "gruppo ordine" con `colSpan` multi-riga: stats-rda ha 1 riga = 1 ordine (pa.issue è già flat), niente raggruppamento righe.

### Reference 2 — `apps/rda` (stesso dominio PA/RDA)
File ispezionati:
- `apps/rda/src/App.tsx` — AppShell + `TabNav`, gate `APP_ACCESS_ROLES.rda`, `Suspense` con fallback `stateCard`.
- `apps/rda/src/routes.tsx` — rotte `/rda`, `/rda/new`, `/rda/po/:poId`.
- `apps/rda/src/pages/RdaListPage.tsx` — cruscotto operativo con SearchInput + SingleSelect stato/area + URL search params (`useSearchParams`) per filtri/sort persistibili, `Skeleton` loading, `stateCard`/`stateBlock` per errori, `RdaDashboardTable`, `ConfirmDialog`.
- `apps/rda/src/pages/PoDetailPage.tsx` — scheda ordine a tab (`PoTabs`) con sezioni (header, righe, commenti, allegati), `CommentsPanel`, `AttachmentsTab`.

Pattern riusati:
- **Linguaggio di scheda ordine** coerente: header (issue_key, summary, stato, date, persone) → sezioni acquisto/righe/commenti/allegati. La scheda Archivio PA seguirà la stessa struttura (8 sezioni da `pa_card.py`) con label business identiche.
- **URL search params** per filtri/sort persistibili e deep-linkabili (panoramica usa useState locale; rda usa `useSearchParams` → preferisco il pattern rda per stats-rda perché i filtri sono deep-linkabili e condivisibili).
- `Skeleton` + `stateCard`/`stateBlock` per loading/error.
- `TabNav` per le due pagine (Archivio PA / RDA) nello stesso AppShell.

Pattern scartati:
- **Cruscotto operativo con KPI/quick-stat** (RdaListPage ha `rdaFocusMetric` + `rdaQuickStats`): stats-rda v0 è **solo ricerca+visualizzazione**, niente KPI/metriche (Metrics Gate: nessuna metrica giustificata dalla feature v0). Le statistiche aggregate arrivano a v2.
- **Azioni di scrittura** (Nuova richiesta, Elimina, transizioni workflow): stats-rda v0 è read-only.
- **Wizard** (`NewRdaWizardPage`): non pertinente.

---

## Archetype Choice

- **Archetipo selezionato:** `master_detail_crud` — adattato a **sola lettura**.
- **Perché calza:** task = cercare ordini per dimensioni (budget/richiedente/fornitore/stato/tipo/date) e visualizzare la scheda di un ordine selezionato. Composizione archetipo: page header compatto + toolbar (search + filtri dimensionali) + tabella risultati (master) + detail panel (scheda 8 sezioni).
- **Stati richiesti:** loading (Skeleton), empty (nessun risultato / invito a filtrare), error (DB non disponibile / 503 → `ServiceUnavailable`), populated (lista + dettaglio), destructive-confirm **non richiesto** (read-only), mobile/narrow (master-detail collassa <1000px come da UI-UX §11/§14).

---

## User Copy Rules

- **Copy policy:** `business-user-only` (default skill).
- **Lingua:** italiano (UI-UX §17).
- **Label scheda** (allineate a `pa_card.py` / `esempio.md`, tutte business):
  - Header: "Ordine PA-1821 · {summary}", "Numero ordine", "Stato", "Tipo", "Richiedente", "Assegnatario", "Creato", "Aggiornato", "Risolto".
  - Sezione "Acquisto": "Importo totale", "Fornitore", "Tipo ordine", "Budget di riferimento", "Budget corrente", "Budget totale", "Limite approvazione", "% approvazione".
  - Sezione "Righe d'ordine" (raggruppate per tipo: Articoli / Merci / Servizi / Leasing).
  - Sezione "Commenti", "Allegati" (con disclosure "solo metadati, file non disponibili"), "Collegamenti", "Storico modifiche".
- **Copy vietato:** termini tecnici (`issue_id`, `EAV`, `record`, `widget`, `server-side`, `schema pa`, `arak`, `issue_key` come etichetta — usare "Ordine PA-XXXX"). Le chiavi tecniche non sono mostrate come label.
- **Metriche:** **nessuna** in v0. Nessuna KPI card, nessun summary card, nessun hero. Le metriche aggregate sono v2 (esplicitata in Exceptions/roadmap).

---

## Repo-Fit

### Frontend
- **App dir:** `apps/stats-rda` (esiste con `docs/jira/`).
- **Vite base:** `command === 'build' ? '/apps/stats-rda/' : '/'` (come rda/panoramica).
- **Vite port:** `5196` (5174–5195 occupate).
- **Proxy:** `/api` e `/config` → `VITE_DEV_BACKEND_URL || http://localhost:8080`.
- **Routing:** `react-router-dom` v7. Rotte v0:
  - index → `Navigate` to `/archivio-pa`
  - `/archivio-pa` → `ArchivioPaPage` (master-detail)
  - `/archivio-pa/:issueKey` → stessa pagina con ordine selezionato (deep-link detail)
  - `/rda` → `RdaPlaceholderPage` ("In costruzione")
  - `*` → `Navigate` to `/archivio-pa`
- **Nav (TabNav):** due item: "Archivio PA" (`/archivio-pa`), "RDA" (`/rda`).
- **Packages:** `@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`, `@tanstack/react-query`, `react-router-dom`.
- **Auth client:** aggiungere `'stats-rda': ['app_stats_rda_access']` a `APP_ACCESS_ROLES` in `packages/auth-client/src/roles.ts`.

### Backend
- **Nuovo modulo:** `backend/internal/statsrda` (Go), `RegisterRoutes(mux, arakDB *sql.DB)`.
- **Data layer:** `arakDB.QueryContext` con SQL diretto su schema `pa` (pattern = `backend/internal/fornitori/handler.go`). **Nessun client HTTP arak** per v0 (il `arak.Client` serve l'app RDA operativa; stats-rda legge l'archivio storico direttamente da Postgres).
- **API prefix:** `/api/stats-rda`.
- **Endpoint v0:**
  - `GET /api/stats-rda/pa/filters` → dimensioni **select-esatte** per i filtri. Response uniforme `{value,count}` per ogni dimensione:
    ```
    {
      budget:    [{value: string, count: number}],
      stati:     [{value: string, count: number}],
      tipi:      [{value: string, count: number}],
      valute:    [{value: string, count: number}],
      range_date: { min: string|null, max: string|null }  // ISO date di created
    }
    ```
    Ordinamento: `count` desc per budget/stati/tipi/valute. Per popolare le option dei SingleSelect.
  - `GET /api/stats-rda/pa/fornitori?q=&limit=` → **autocomplete** su `fornitore_selezionato` (distinct, `ILIKE '%q%'`, top `limit` default 20 max 50, ordinato per frequenza desc). Response: `{ items: [{value,count}] }`. Validazione: `limit` non numerico o fuori range [1,50] → `400` con body `{ error: string }`.
  - `GET /api/stats-rda/pa/richiedenti?q=&limit=` → **autocomplete** su `reporter_name` (distinct, `ILIKE '%q%'`, top `limit` default 20 max 50, ordinato per frequenza desc). Response: `{ items: [{value,count}] }`. Validazione: `limit` non numerico o fuori range [1,50] → `400` con body `{ error: string }`.
  - `GET /api/stats-rda/pa/issues` → lista paginata. Contratto completo:
    - **Query params:** `q` (testo libero, match `ILIKE` su `summary` **e** `numero_ordine`), `budget` (exact, `budget_di_riferimento =`), `stato` (exact, `status =`), `tipo` (exact, `issue_type =`), `fornitore` (exact, `fornitore_selezionato =` — valore selezionato dall'autocomplete), `richiedente` (exact, `reporter_name =`), `valuta` (exact, `valuta =`), `from` / `to` (date `YYYY-MM-DD`, applicati a **`created`**: `created >= from AND created < to+1day`), `sort` (uno tra: `created`, `importo_totale`, `issue_key`, `reporter_name`; default `created`), `dir` (`asc`|`desc`, default `desc`), `page` (default 1, min 1), `limit` (default 50, max 100).
    - **Response envelope:** `{ items: IssueSummary[], total: number, page: number, limit: number }`.
    - **`IssueSummary` campi:** `issue_key, summary, numero_ordine, status, issue_type, importo_totale, valuta, budget_di_riferimento, reporter_name, fornitore_selezionato, created`.
    - **Validazione:** parametri non riconosciuti ignorati; `limit`/`page` fuori range → 400; date malformate → 400; `sort` fuori whitelist (`created`,`importo_totale`,`issue_key`,`reporter_name`) → 400; `dir` diverso da `asc`/`desc` → 400. Errori 400 con body `{ error: string }` leggibile.
  - `GET /api/stats-rda/pa/issues/:issueKey` → scheda completa. Storico modifiche: default **20** eventi, configurabile via query param `history_limit` (min 1, max 100, default 20). Response espone `history_total` (count totale `change_group` dell'ordine) per indicare troncamento.
    - **Envelope `IssueDetail`:**
      ```
      {
        issue: IssueHeader,            // sezione 1
        purchase: PurchaseSection,      // sezione 2
        description: string | null,     // sezione 3
        line_items_by_grid: {           // sezione 4 — raggruppate per grid
          Articoli?: LineItem[],
          Merci?: LineItem[],
          Servizi?: LineItem[],
          Leasing?: LineItem[]
        },
        comments: Comment[],            // sezione 5 (cronologico)
        attachments: Attachment[],      // sezione 6 (solo metadati)
        links: IssueLink[],             // sezione 7
        history: HistoryEntry[],        // sezione 8 (ultime N, DESC)
        history_total: number           // count totale change_group
      }
      ```
    - **`IssueHeader`** (tutti nullable tranne `issue_key`, `summary`): `issue_key: string`, `summary: string`, `numero_ordine: string|null`, `issue_type: string|null`, `status: string|null`, `stato: string|null` (campo dominio, ≠ status), `priority: string|null`, `resolution: string|null`, `valuta: string|null`, `created: string|null` (ISO), `updated: string|null`, `resolution_date: string|null`, `due_date: string|null`, `reporter_name: string|null`, `reporter_email: string|null`, `assignee_name: string|null`, `creator_name: string|null`.
    - **`PurchaseSection`** (tutti nullable): `importo_totale: number|null`, `importo_totale_merci: number|null`, `importo_totale_servizi: number|null`, `importo_totale_leasing: number|null`, `fornitore_selezionato: string|null`, `tipo_di_ordine: string|null`, `tipo_documento: string|null`, `budget_di_riferimento: string|null`, `budget_corrente: number|null`, `budget_totale: number|null`, `limite_approvazione: number|null`, `percentuale_approvazione: number|null`, `inviato_in_approvazione: string|null`, `approvato: string|null`, `ricorrente: string|null`.
    - **`LineItem`**: `grid: string`, `row_no: number`, `articolo_name: string|null`, `vendor: string|null`, `part_number: string|null`, `descrizione: string|null`, `quantita: number|null`, `importo: number|null`, `prezzo_totale: number|null` (null per Articoli → frontend calcola `quantita*importo`), `pagamento_name: string|null`, `durata_name: string|null`, `data_pagamento: string|null`, `vendita: string|null`, `rinnovo: string|null`.
    - **`Comment`**: `created: string` (ISO), `author_name: string|null`, `body: string|null`.
    - **`Attachment`** (solo metadati): `filename: string`, `mimetype: string|null`, `filesize: number|null`, `created: string|null` (ISO), `author_name: string|null`.
    - **`IssueLink`**: `source_key: string`, `destination_key: string`, `link_name: string|null`, `inward: string|null`, `outward: string|null`.
    - **`HistoryEntry`**: `created: string` (ISO), `author_name: string|null`, `field: string|null`, `old_string: string|null`, `new_string: string|null`.
    - **Validazione detail:** `history_limit` non numerico o fuori range [1,100] → `400` con body `{ error: string }`; `issue_key` inesistente → `404`.
- **Ruolo:** `app_stats_rda_access` (convenzione repo `app_{slug}_access`, **non** `{slug}-user`).
- **Gate launcher v0:** `definition.ID == applaunch.StatsRDAAppID && arakDB == nil` → mark non disponibile. Il modulo v0 usa **solo arakDB** (SQL su schema `pa`); `arakCli` non è richiesto in v0. (Quando la pagina RDA «nuove» sarà implementata in v2 e userà `arakCli`, il gate diventerà `arakCli == nil || arakDB == nil`.)
- **Registrazione launcher:** nuovo `StatsRDAAppID = "stats-rda"`, `StatsRDAAppHref = "/apps/stats-rda/"` in `backend/internal/platform/applaunch/catalog.go`, categoria `acquisti`, `Status: "ready"`, `AccessRoles: StatsRDAAccessRoles()`, `statsRdaAccessRoles = []string{"app_stats_rda_access"}`, aggiunto a `AllRoles()`.
- **main.go:** `statsrda.RegisterRoutes(api, arakDB)` + href override `StatsRDAAppID` come RDA.

### Auth
- Tutti gli endpoint `GET` con Bearer auth (middleware esistente).
- **Errori comuni a tutti gli endpoint `stats-rda`:** indisponibilità/errore di connessione `arakDB` → `503` con body `{ error: string }` (mappato a `ServiceUnavailable` lato UI); errore imprevisto non classificabile → `500` con body `{ error: string }` (logging server-side, messaggio generico al client).
- `app_stats_rda_access` come ruolo richiesto (sia frontend `getAppAccessState` sia backend authz).
- 401/403 → `AccessNotice` lato frontend, gestito dal pattern `useOptionalAuth` + `getAppAccessState`.

### Deploy
- Dockerfile multi-stage esistente copia `apps/*/dist` → aggiungere `apps/stats-rda` al build stage (verificare `deploy/Dockerfile` pattern degli altri app).
- Static hosting: backend serve `apps/stats-rda/` da filesystem (come gli altri app).

---

## Normalizzazione importi (opzione c — approvata)

- A livello **lista**: mostriamo `importo_totale` in valuta originale con badge valuta (es. "13.000,00 €", "1.200,00 $"). Nessuna conversione in v0 (nessun tasso disponibile nello schema).
- A livello **scheda dettaglio**: importo totale + componenti (merce/servizi/leasing) in valuta originale; aggiungiamo riga "Importo stimato in €" **solo se** un tasso è disponibile (tabella `pa.currency_rate` o config). In assenza di tasso, mostriamo solo la valuta originale con disclosure "valuta non € non convertita".
- **v0 implementazione minima:** nessun tasso → solo valuta originale + badge. La conversione € è v1.2 (esplicitata in Exceptions).

---

## Scheda dettaglio — 8 sezioni (v0)

1. **Header** — `pa.issue` (issue_key, summary, numero_ordine, issue_type, status, stato, priority, resolution, date, reporter/assignee/creator).
2. **Acquisto / economico** — importi, fornitore, tipo ordine, tipo documento, budget (riferimento/corrente/totale), limite/% approvazione.
3. **Descrizione** — `pa.issue.description`.
4. **Righe d'ordine** — `pa.line_item` raggruppate per `grid` (Articoli/Merci/Servizi/Leasing). Per Articoli: totale = `quantita × importo` (prezzo_totale vuoto).
5. **Commenti** — `pa.comment` (data, autore, testo).
6. **Allegati** — `pa.attachment` (solo metadati + disclosure).
7. **Collegamenti** — `pa.issue_link`.
8. **Storico modifiche** — `pa.change_group` ⨝ `pa.change_item` (ultime N, configurabile).

---

## Exceptions

1. **Nessun KPI/metrica in v0** nonostante il nome "Statistiche RDA". La natura statistica è roadmap v2+. v0 è search+view. Giustificato: la prima funzionalità richiesta è cercare/visualizzare una PA, non aggregare.
2. **Pagina "RDA" placeholder** in v0. La mini-app coprirà anche le RDA nuove (sistema RDA corrente) ma quella pagina è "in costruzione" in v0. Giustificato: la prima feature è Archivio PA; la pagina RDA esiste solo per fissare la nav e il ruolo.
3. **Normalizzazione € differita** (opzione c minima): in v0 mostriamo solo valuta originale + badge. Conversione con tassi in v1.2. Giustificato: lo schema non ha tassi di cambio; inventarli violerebbe il Metrics Gate.
4. **`master_detail_crud` in sola lettura**: l'archetipo CRUD è adattato rimuovendo create/edit/delete. Giustificato: è l'archetipo più vincolato che calza per list+detail; non inventiamo un nuovo archetipo.
5. **Filtri fornitore/richiedente come autocomplete-select** (non select-esatte su tutti i valori distinti, né free-text parziale su `/issues`). L'utente digita → l'autocomplete suggerisce valori distinti (`ILIKE`) → l'utente **seleziona** un suggerimento → il filtro lista è **exact match** sul valore selezionato. Giustificato: `fornitore_selezionato` è denso ~50% e `reporter_name` ha molti nomi; enumerarli tutti in un SingleSelect sarebbe dispersivo, mentre un free-text parziale su `/issues` renderebbe i risultati non predicibili. L'autocomplete-select bilancia usabilità e predicibilità.

---

## Roadmap

- **v0** (questo piano): Archivio PA — search (testo libero su `summary` + `numero_ordine`) + filtri dimensionali (budget, stato, tipo, fornitore, richiedente, valuta, range date) + lista paginata + scheda 8 sezioni. Pagina RDA placeholder.
- **v1**: ricerca testuale estesa a `description` e `commenti` (ILIKE) come modalità aggiuntiva.
- **v1.2**: normalizzazione importi in € con tassi di cambio.
- **v2**: pagina RDA "nuove" (integrazione sistema RDA corrente via arakCli o API RDA).
- **v3**: statistiche aggregate (spesa per budget/richiedente/fornitore, trend mensile, SLA approvazione) + export Excel/CSV.
- **v4**: preset filtri / report salvati.

---

## Verification

### UI review checks (pre-gate + post-gate)
- Stato popolato: lista risultati + scheda dettaglio con 8 sezioni.
- Stato empty: invito a filtrare (nessun filtro attivo) / "Nessun ordine trovato" (filtri attivi, 0 risultati).
- Stato loading: Skeleton righe + Skeleton scheda.
- Stato error: `ServiceUnavailable` su 503/DB down; `AccessNotice` su 403.
- Stato mobile/narrow: master-detail collassa <1000px.
- Deep-link: `/archivio-pa/PA-1821` apre la scheda.
- Copy gate: nessun termine tecnico visibile.
- Metrics gate: nessuna KPI card.
- Style gate: clean theme, background gradient standard, token UI-UX.

### Runtime / auth checks
- Endpoint `/api/stats-rda/pa/*` risponde con Bearer auth.
- 401/403 gestiti.
- Ruolo `app_stats_rda_access` richiesto (frontend + backend).
- Gate launcher: app non visibile se arakDB non configurato.
- Vite dev: proxy `/api`+`/config` su porta 5196.

### Tests
- Nessun test aggiunto salvo approvazione attuale (regola progetto). Se approvati:
  - Backend: query SQL su schema pa con test DB (pattern `fornitori/handler_test.go`).
  - Frontend: view-model della scheda (mapping issue → sezioni) — non UI.

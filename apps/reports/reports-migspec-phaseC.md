# Reports — Phase C: Logic Placement

## Decisioni consolidate

- **Migrazione 1:1**, coesistenza con Appsmith
- **AI analysis (Anomalie MOR Tab 2) differita** a fase successiva (tracked in `docs/TODO.md`)
- **Query AOV mantenute separate** — 4 query verbatim, nessun consolidamento in V1
- **Tutte le query SQL copiate verbatim** dall'audit per evitare drift

---

## 1. JSObject Methods — Placement

| Metodo | Pagine | Classificazione | Placement V1 | Note |
|---|---|---|---|---|
| `utils.getURL()` | Ordini, Accessi attivi, AOV | Orchestrazione | **Eliminato** — il backend gestisce Carbone.io internamente | URL Carbone.io non esposto al frontend |
| `utils.runReport()` | Ordini, Accessi attivi, AOV | Orchestrazione | **Backend** — endpoint `POST /api/reports/{type}/export` | Frontend invia filtri, riceve file XLSX |
| `utils.collega_ordini()` | Anomalie MOR | Dominio (BR9) | **Backend** — cross-DB join server-side | Endpoint restituisce dati già arricchiti con flag `ordine_presente`/`numero_ordine_corretto` |
| `utils.analizza()` | Anomalie MOR | Orchestrazione + Dominio | **Differito** — Phase 2 | |
| `utils.ai_request()` | Anomalie MOR | Infrastruttura | **Differito** — Phase 2 | |
| `utils.abilita_controlli_ai()` | Anomalie MOR | Access control (BR10) | **Differito** — Phase 2 | |
| `utils.generaTenantIdList()` | Accounting TIMOO | Orchestrazione + Dominio (BR11) | **Backend** — costruzione URL TIMOO API server-side | Esclusione tenant test `KlajdiandCo` in config backend |
| `utils.listaTenants()` | Accounting TIMOO | Orchestrazione | **Backend** — parte dell'endpoint daily-stats | |
| `utils.reportData()` | Accounting TIMOO | Orchestrazione | **Backend** — endpoint singolo con merge multi-sorgente | |
| vestigial `_$js_openrouter1$_` | Anomalie MOR | Nessuna | **Eliminato** — codice morto | |

## 2. Business Rules — Placement

| # | Regola | Placement V1 | Strategia |
|---|---|---|---|
| BR1 | AOV: `MRC_new * 12 + NRC`, delta sostituzioni, swap TSC-ORDINE | **Backend SQL** | 4 query separate, verbatim dall'audit |
| BR2 | Date fallback: `data_conferma` con fallback `data_documento` se sentinel | **Backend SQL** | Verbatim nelle query AOV |
| BR3 | Mapping tipo ordine: N→NUOVO, A→SOST, R→RINNOVO, C→CESSAZIONE | **Backend SQL** | CASE expression nelle query, verbatim |
| BR4 | Sales rep HubSpot: join + normalizzazione `/`→`-`, default `'CP'` | **Backend SQL** | Verbatim nelle query AOV |
| BR5 | Bandwidth: `banda_up != banda_down` → CONDIVISA/DEDICATA | **Backend SQL** | Verbatim nella query accessi |
| BR6 | Renewal window: `current_date - 15gg` a `+ N mesi` | **Backend SQL** | Parametro `months` passato dal frontend |
| BR7 | Renewal filter: `durata_rinnovo > 3 OR tacito_rinnovo = 0` | **Backend SQL** | Verbatim nella query rinnovi |
| BR8 | 6 regole validazione anomalie MOR (prompt AI) | **Differito** | Phase 2 |
| BR9 | Cross-ref billing→ERP per serialnumber | **Backend Go** | Join server-side Grappa + Mistra |
| BR10 | AI gate per email `sciacco` | **Differito** | Phase 2 |
| BR11 | Esclusione tenant test `KlajdiandCo` | **Backend SQL** | `WHERE name != 'KlajdiandCo'` nella query — `as7_tenants.name` ora disponibile in DB |
| BR12 | DB function `get_reverse_order_history_path()` | **Backend SQL** | Già nel DB, il backend la chiama nella query |

## 3. Frontend Responsibilities

| Responsabilità | Dettaglio |
|---|---|
| Stato filtri | Date range defaults (`moment().subtract(1,'month')` / `subtract(1,'days')`), multi-select selections |
| Selezione riga master-detail | `selectedRow` per Attivazioni in corso e Rinnovi in arrivo |
| Aggregazione client-side per riepilogo | Stat card AOV e riepilogo anteprima (Ordini, Accessi) calcolati sui dati già ricevuti dal backend |
| Gate UI feature AI | **Differito** — in V1 il Tab 2 di Anomalie MOR non esiste |
| Presentazione | Formattazione valori, tabelle, skeleton loading, animazioni |

## 4. Riepilogo layer

| Layer | Cosa contiene |
|---|---|
| **Backend SQL** | Tutte le query verbatim dall'audit (parametrizzate con prepared statements, non string interpolation). Le business rules BR1-BR7, BR12 restano nelle query. |
| **Backend Go** | Orchestrazione Carbone.io (export XLSX), cross-DB join Anomalie MOR (BR9), orchestrazione TIMOO API (BR11), endpoint REST per ogni report |
| **Frontend React** | Stato UI (filtri, selezione), default date, aggregazione client-side per stat card/riepilogo, presentazione |
| **Differito (Phase 2)** | AI analysis: OpenRouter proxy, prompt rules, RBAC gate, model selector |

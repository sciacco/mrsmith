# Reports — Phase D: Integration and Data Flow

## Decisioni consolidate

- Carbone.io: manteniamo il servizio esterno con gli **stessi template ID come costanti** nel backend Go
- Viste DB: DDL presenti in `docs/mistradb/mistra_loader.json` (`v_ordini_ric_spot`, `v_ordini_sintesi`, `v_ordini_ricorrenti`, `v_ordini_ricorrenti_conrinnovo`, `get_reverse_order_history_path`)
- AI analysis (OpenRouter): differita Phase 2

---

## 1. Datasource e gestione backend

| Datasource | Tipo | Backend connection | Config necessaria |
|---|---|---|---|
| **mistra** | PostgreSQL | Connection pool (già esistente nel backend Go) | DSN in env var |
| **grappa** | MySQL | Connection pool dedicato | DSN in env var |
| **anisetta** | PostgreSQL | Connection pool dedicato | DSN in env var |
| **carbone.io** | REST API | HTTP client nel backend | API key in env var, template IDs come costanti Go |
| **TIMOO API** | REST API interna | **Eliminata** — `as7_tenants.name` disponibile in DB | — |
| **openrouter** | REST API | **Differito Phase 2** | — |

### Carbone.io Template IDs (costanti)

| Template | ID | Usato da |
|---|---|---|
| Ordini XLSX | `d18b310491b0c8d2518841b4e09cc18d8b91c5a59ae5a55c37924fcb169de166` | Ordini, AOV |
| Accessi attivi XLSX | `a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e` | Accessi attivi |

## 2. User journeys end-to-end

### Journey A — Export XLSX (Ordini, Accessi attivi, AOV)

```
Utente → imposta filtri → click "Anteprima"
  → Frontend: POST /api/reports/{type}/preview {filters}
  → Backend: query mistra → response JSON con dati
  → Frontend: mostra riepilogo aggregato (calcolato client-side sui dati ricevuti)
Utente → click "Esporta XLSX"
  → Frontend: POST /api/reports/{type}/export {filters}
  → Backend: query mistra → POST carbone.io (template ID costante + dati) → stream XLSX
  → Frontend: download file
```

### Journey B — Master-detail senza filtri (Attivazioni in corso)

```
Page load
  → Frontend: GET /api/reports/pending-activations
  → Backend: query mistra (v_ordini_sintesi + erp_anagrafiche_clienti, stato='Confermato', riga='Da attivare')
  → Frontend: popola master table
Utente seleziona riga
  → Frontend: GET /api/reports/pending-activations/{orderNumber}/rows
  → Backend: query mistra
  → Frontend: popola detail table
```

### Journey C — Master-detail con filtri (Rinnovi in arrivo)

```
Page load (defaults: months=4, minMrc=11)
  → Frontend: GET /api/reports/upcoming-renewals?months=4&minMrc=11
  → Backend: query mistra (v_ordini_ricorrenti_conrinnovo)
  → Frontend: popola master table
Utente seleziona riga
  → Frontend: GET /api/reports/upcoming-renewals/{customerId}/rows?months=4&minMrc=11
  → Backend: query mistra
  → Frontend: popola detail table
Utente cambia filtri + click button → stessi endpoint con nuovi parametri
```

### Journey D — Anomalie MOR (V1 — solo Tab 1, dati arricchiti)

```
Page load
  → Frontend: GET /api/reports/mor-anomalies
  → Backend: query grappa (importi_telefonici ultimo periodo)
           + query mistra (erp_righe_ordini, codice_prodotto='CDL-TVOCE', attivi)
           → cross-reference server-side per serialnumber
           → response JSON con flag ordine_presente / numero_ordine_corretto
  → Frontend: popola tabella anomalie
```

### Journey E — Accounting TIMOO

```
Page load
  → Frontend: GET /api/reports/timoo/daily-stats
  → Backend: single query anisetta (as7_pbx_accounting JOIN as7_tenants ON as7_tenant_id, ultimi 3 mesi)
           → WHERE as7_tenants.name != 'KlajdiandCo' (esclusione tenant test)
           → response JSON con tenant name incluso
  → Frontend: popola tabella
```

**Semplificazione vs Appsmith**: la catena Anisetta→TIMOO API→Anisetta→merge client-side è sostituita da una singola query SQL con JOIN, perché `as7_tenants` ora include la colonna `name` (varchar(255)).

## 3. Auto-load e trigger

| Comportamento | Pagina | Implementazione frontend |
|---|---|---|
| Fetch on mount (no filtri utente) | Attivazioni, Anomalie MOR, Accounting TIMOO | `useEffect` o route loader |
| Fetch filtri on mount (dropdown options) | Ordini (`get_stati_ordine`), Accessi (`get_tipo_conn`), AOV (`get_stati_ordine`) | `useEffect` o route loader |
| Cascade master→detail | Attivazioni, Rinnovi | `onClick` riga → fetch detail endpoint |
| Refresh su cambio filtri | Rinnovi (button), AOV (button), Ordini/Accessi (button "Anteprima") | `onClick` button → fetch con nuovi parametri |

## 4. Endpoint API — Riepilogo

| Endpoint | Metodo | Pagina | Parametri | Response |
|---|---|---|---|---|
| `/api/reports/order-statuses` | GET | Ordini, AOV | — | `string[]` |
| `/api/reports/connection-types` | GET | Accessi | — | `string[]` |
| `/api/reports/orders/preview` | POST | Ordini | `{dateFrom, dateTo, statuses[]}` | JSON dati ordini |
| `/api/reports/orders/export` | POST | Ordini | `{dateFrom, dateTo, statuses[]}` | XLSX stream |
| `/api/reports/active-lines/preview` | POST | Accessi | `{connectionTypes[], statuses[]}` | JSON dati accessi |
| `/api/reports/active-lines/export` | POST | Accessi | `{connectionTypes[], statuses[]}` | XLSX stream |
| `/api/reports/pending-activations` | GET | Attivazioni | — | JSON master rows |
| `/api/reports/pending-activations/:orderNumber/rows` | GET | Attivazioni | — | JSON detail rows |
| `/api/reports/upcoming-renewals` | GET | Rinnovi | `?months=N&minMrc=N` | JSON master rows |
| `/api/reports/upcoming-renewals/:customerId/rows` | GET | Rinnovi | `?months=N&minMrc=N` | JSON detail rows |
| `/api/reports/mor-anomalies` | GET | Anomalie MOR | — | JSON enriched records |
| `/api/reports/timoo/daily-stats` | GET | Accounting TIMOO | — | JSON merged stats |
| `/api/reports/aov/preview` | POST | AOV | `{dateFrom, dateTo, statuses[]}` | JSON con 4 dataset (by-type, by-category, by-sales, detail) |
| `/api/reports/aov/export` | POST | AOV | `{dateFrom, dateTo, statuses[]}` | XLSX stream |

## 5. Dipendenze esterne — prerequisiti

| Dipendenza | Stato | Azione necessaria |
|---|---|---|
| Viste `loader.v_ordini_*` DDL | Presenti in `docs/mistradb/mistra_loader.json` | Nessuna — usare come riferimento |
| Function `get_reverse_order_history_path` DDL | Presente in `docs/mistradb/mistra_loader.json` | Nessuna — usare come riferimento |
| Carbone.io template XLSX (2 template) | Esistenti su Carbone.io | Nessuna — stessi ID come costanti |
| ~~TIMOO API spec~~ | **Eliminata** | `as7_tenants.name` disponibile in DB — nessuna call API necessaria |
| Grappa MySQL access | Usato solo per Anomalie MOR | Configurare connection pool MySQL nel backend |
| Anisetta PostgreSQL access | Usato solo per Accounting TIMOO | Configurare connection pool PostgreSQL nel backend |

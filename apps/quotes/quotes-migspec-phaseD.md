# Quotes — Phase D: Integration and Data Flow

**Source**: `apps/quotes/APPSMITH-AUDIT.md`, Phase A–C migspec documents
**Date**: 2026-04-09
**Scope**: Quotes/proposals only. E-signature e order conversion esclusi.

---

## 1. External Systems and Purpose

### 1.1 Mistra PostgreSQL (primary data store)

| Aspect | Details |
|---|---|
| **Role** | Source of truth per quotes, products, templates. Mirror per HubSpot e ERP data. |
| **Schemas** | `quotes` (quote, quote_rows, quote_rows_products, template, quote_customer), `products` (kit, product, product_category, kit_product), `loader` (hubs_company, hubs_deal, hubs_owner, hubs_pipeline, hubs_stages, erp_metodi_pagamento, erp_anagrafiche_clienti), `common` (functions, translations) |
| **Access** | Go backend via `database/sql` + `pgx`. Già in uso per altre app nel monorepo. |
| **Connection** | Esistente in `backend/` — stessa pool usata da panoramica, listini, etc. |
| **Data ownership** | Quotes app **scrive** in `quotes.*`. **Legge** da `products.*`, `loader.*`, `common.*`. Non scrive mai fuori dal proprio schema. |

### 1.2 Alyante MS SQL Server (ERP)

| Aspect | Details |
|---|---|
| **Role** | Dati anagrafici clienti (pagamento default) e ordini confermati (per SOSTITUZIONE) |
| **Tables read** | `Tsmi_Anagrafiche_clienti` (payment code), `Tsmi_Ordini` (order names) |
| **Access** | Go backend via `database/sql` + `go-mssqldb`. Pattern già esistente in `backend/internal/listini/`. |
| **Frequency** | On-demand: payment default alla selezione cliente, ordini al cambio proposal_type a SOSTITUZIONE |
| **Data ownership** | Read-only. La quotes app non scrive mai su Alyante. |
| **Cross-ref** | `customer_id` (HubSpot company ID) → `loader.erp_anagrafiche_clienti.numero_azienda` → Alyante `NUMERO_AZIENDA` |

### 1.3 HubSpot CRM REST API

| Aspect | Details |
|---|---|
| **Role** | Pubblicazione quote con line items, associazioni deal/template/company, generazione PDF, status sync |
| **Base URL** | `https://api.hubapi.com` |
| **Auth** | OAuth o Private App token (gestito backend, mai esposto al frontend) |
| **Endpoints used** | Vedi tabella sotto |
| **Data ownership** | Bidirezionale: la nostra app crea/aggiorna quote e line items su HS; HS può modificare lo status (APPROVAL_NOT_NEEDED, ESIGN_COMPLETED) |
| **Rate limits** | HubSpot standard: 100 requests/10 sec (private app). Il publish flow fa ~2N+5 chiamate (N = numero kit rows). Per 10 kit rows ≈ 25 chiamate. |

#### HubSpot endpoints in scope

| Method | Endpoint | Operation | Phase C ref |
|---|---|---|---|
| `POST` | `/crm/v3/objects/quote` | Create HS quote | 2.2 step 3 |
| `PATCH` | `/crm/v3/objects/quotes/{id}` | Update HS quote (status, terms, associations) | 2.2 step 6 |
| `GET` | `/crm/v3/objects/quotes/{id}` | Read HS quote status, PDF link | hs-status endpoint |
| `GET` | `/crm/v3/objects/quotes/{id}?associations=line_items` | Read current line item associations | 2.3 |
| `DELETE` | `/crm/v3/objects/quotes/{id}` | Delete HS quote | 1.2 |
| `POST` | `/crm/v3/objects/line_item` | Create line item | 2.3 |
| `PATCH` | `/crm/v3/objects/line_item/{id}` | Update line item | 2.3 |
| `DELETE` | `/crm/v3/objects/line_item/{id}` | Delete orphan line item | 2.3 |
| `PUT` | `/crm/v4/objects/quotes/{id}/associations/...` | Associate template to quote | 2.2 step 3 |

#### HubSpot endpoints OUT OF SCOPE (e-signature removed)

| Method | Endpoint | Reason |
|---|---|---|
| `PUT` | `/crm/v4/objects/quote/{id}/associations/contact/{cid}` | E-signature signer association |
| `DELETE` | `/crm/v4/objects/quote/{id}/associations/contact/{cid}` | E-signature signer removal |
| `GET` | `/crm/v3/objects/companies/{id}?associations=contacts` | Company contacts for signer picker |
| `GET` | `/crm/v3/objects/contacts/{id}` | Individual contact for signer picker |

#### HubSpot endpoints OUT OF SCOPE (order conversion deferred)

| Method | Endpoint | Reason |
|---|---|---|
| `POST` | `/files/v3/files` | PDF upload |
| `POST` | `/crm/v3/objects/notes` | CRM note with attachment |

### 1.4 Systems NOT integrated (out of scope)

| System | Reason |
|---|---|
| Vodka MySQL | Order conversion deferred |
| GW internal CDLAN REST | Order PDF generation deferred |
| Carbone.io | Experiment abandoned (A8) |

---

## 2. Data Flow Diagrams

### 2.1 Quote Creation (new unified flow)

```
USER (wizard)
  │
  │ Step 1: select deal
  │ Step 2: configure header (type, services, terms, template)
  │ Step 3: select kits + configure products
  │ Step 4: notes, contacts (optional)
  │ Review + confirm
  │
  ▼
FRONTEND ──POST /api/quotes/v1/quotes──► BACKEND (Go)
                                            │
                                            ├─► Mistra PG: common.new_document_number('SP-')
                                            │
                                            ├─► Mistra PG: quotes.ins_quote_head(json)
                                            │     └─► TRIGGER: quote_customer from ERP snapshot
                                            │
                                            ├─► Mistra PG: INSERT quote_rows (per kit)
                                            │     └─► TRIGGER: expand kit → quote_rows_products
                                            │         └─► TRIGGER: recalculate row totals
                                            │
                                            ├─► Mistra PG: upd_quote_row_product (per configured product)
                                            │     └─► TRIGGER: recalculate row totals
                                            │
                                            └─► Response: complete quote with rows + products
                                            
                                            HS NOT CALLED (quote stays DRAFT in DB only)
```

**Key change vs Appsmith**: No HubSpot call at creation. Quote lives only in local DB until explicit publish.

### 2.2 Quote Edit (Dettaglio)

```
USER (editor)
  │
  ├─ Load ──GET /quotes/:id──► BACKEND ──► Mistra PG: quote + rows + products
  │                                         ├─► Mistra PG: loader.hubs_* (dropdowns)
  │                                         └─► Response: full quote state
  │
  ├─ Edit header ──PUT /quotes/:id──► BACKEND
  │                                      ├─► Validate business rules
  │                                      │   (COLOCATION→billing, IaaS lock, spot→MRC)
  │                                      ├─► Mistra PG: quotes.upd_quote_head(json)
  │                                      └─► Response: updated quote
  │
  ├─ Add kit ──POST /quotes/:id/rows──► BACKEND
  │                                        ├─► Mistra PG: INSERT quote_rows
  │                                        │   └─► TRIGGER: expand products
  │                                        └─► Response: new row + products
  │
  ├─ Configure product ──PUT /.../products/:pid──► BACKEND
  │                                                  ├─► Mistra PG: upd_quote_row_product
  │                                                  │   (mutual exclusion, MRC=0, qty floor)
  │                                                  └─► Response: updated product + row totals
  │
  └─ Delete kit ──DELETE /quotes/:id/rows/:rid──► BACKEND
                                                    ├─► Mistra PG: DELETE (CASCADE products)
                                                    └─► Response: OK
```

### 2.3 HubSpot Publish (idempotent)

```
USER
  │
  │ Click "Pubblica su HubSpot"
  │
  ▼
FRONTEND ──POST /quotes/:id/publish──► BACKEND (Go)
  ▲                                       │
  │ Progress SSE/polling                  ├─► Step 1: Save quote to DB (if dirty)
  │ ◄────────────────────────────────     │
  │                                       ├─► Step 2: Validate required products
  │   "Validazione..."                    │   └─ FAIL? → return error + missing products list
  │                                       │
  │                                       ├─► Step 3: Create or update HS quote
  │   "Creazione offerta HS..."           │   ├─ hs_quote_id NULL? → POST /crm/v3/objects/quote
  │                                       │   │   ├─ Build T&C from (template_type, is_colo, lang)
  │                                       │   │   ├─ Build associations (template, deal, company)
  │                                       │   │   ├─ Set expiry = document_date + 30
  │                                       │   │   └─ Write hs_quote_id back to DB
  │                                       │   └─ hs_quote_id set? → PATCH (update properties)
  │                                       │
  │                                       ├─► Step 4: Sync line items (bidirectional)
  │   "Sincronizzazione prodotti..."      │   ├─ Read current HS associations
  │                                       │   ├─ For each kit row:
  │                                       │   │   ├─ hs_line_item_id set? → PATCH (update)
  │                                       │   │   └─ NULL? → POST (create) → write ID to DB
  │                                       │   ├─ Delete orphan HS items (in HS but not in DB)
  │                                       │   └─ Uses v_quote_rows_for_hs for bilingual descriptions
  │                                       │
  │                                       ├─► Step 5: Update HS quote status
  │   "Aggiornamento stato..."            │   ├─ notes non vuoto → PENDING_APPROVAL
  │                                       │   └─ notes vuoto → APPROVED
  │                                       │
  │                                       ├─► Step 6: Save final status to DB
  │                                       │
  │   "Pubblicazione completata ✓"        └─► Response: success + HS quote link + PDF link
  │
  ▼
FRONTEND shows result (success or error at specific step)
```

### 2.4 Quote Delete

```
USER
  │
  │ Click "Elimina" (requires role)
  │
  ▼
FRONTEND ──DELETE /quotes/:id──► BACKEND (Go)
                                    │
                                    ├─► Check Keycloak role (RBAC server-side)
                                    │   └─ FAIL? → 403 Forbidden
                                    │
                                    ├─► hs_quote_id set?
                                    │   └─ YES → DELETE /crm/v3/objects/quotes/{hs_id}
                                    │       └─ FAIL? → 500 (DB NOT touched)
                                    │
                                    ├─► Mistra PG: DELETE FROM quotes.quote WHERE id = :id
                                    │   └─ CASCADE: quote_rows → quote_rows_products, quote_customer
                                    │
                                    └─► Response: OK
```

### 2.5 Reference Data Loading

```
Page load / wizard step
  │
  ▼
FRONTEND (parallel requests)
  │
  ├─► GET /quotes/v1/deals ─────────► BACKEND ──► Mistra PG: loader.hubs_deal (pipeline filter)
  ├─► GET /quotes/v1/customers ─────► BACKEND ──► Mistra PG: loader.hubs_company
  ├─► GET /quotes/v1/owners ────────► BACKEND ──► Mistra PG: loader.hubs_owner
  ├─► GET /quotes/v1/templates ─────► BACKEND ──► Mistra PG: quotes.template (new columns)
  ├─► GET /quotes/v1/categories ────► BACKEND ──► Mistra PG: products.product_category (excl 12-15)
  ├─► GET /quotes/v1/kits ──────────► BACKEND ──► Mistra PG: products.kit (active, quotable)
  └─► GET /quotes/v1/payment-methods► BACKEND ──► Mistra PG: loader.erp_metodi_pagamento

On customer selection:
  ├─► GET /quotes/v1/customer-payment/:id ► BACKEND ──► Alyante MSSQL (default payment code)
  └─► GET /quotes/v1/customer-orders/:id ──► BACKEND ──► Alyante MSSQL (orders for SOSTITUZIONE)
```

**Key change vs Appsmith**: Alyante queries fired on-demand (customer selection) instead of on page load. Eliminates spurious queries with empty customer ID.

---

## 3. Cross-System Identity Mapping

La quotes app attraversa 3 sistemi con chiavi diverse. Mapping critico per le query cross-database.

```
HubSpot Company ID (loader.hubs_company.id)
    │
    ├── quotes.quote.customer_id (FK logica)
    │
    ├── loader.erp_anagrafiche_clienti.numero_azienda (= Alyante ERP ID)
    │       │
    │       └── Alyante MSSQL: Tsmi_Anagrafiche_clienti.NUMERO_AZIENDA
    │                           Tsmi_Ordini.NUMERO_AZIENDA (per cli_orders filter)
    │
    └── loader.hubs_company.id = customers.customer.id (Mistra PG)
```

**Lookup chain per customer payment default**:
1. Frontend seleziona customer → `customer_id` (HubSpot company ID)
2. Backend cerca `loader.erp_anagrafiche_clienti WHERE numero_azienda = :customer_id`
3. Trova il codice pagamento ERP, oppure fallback 402

**Lookup chain per customer orders (SOSTITUZIONE)**:
1. Frontend seleziona customer → `customer_id`
2. Backend query su Alyante MSSQL: `Tsmi_Ordini WHERE NUMERO_AZIENDA = :customer_id AND stato IN ('Confermato', 'Evaso')`

Riferimento: `docs/IMPLEMENTATION-KNOWLEDGE.md` → "Customer Identity Across Systems"

---

## 4. Data Freshness and Sync

### 4.1 HubSpot mirror tables (`loader.*`)

| Table | Sync mechanism | Freshness | Impact on Quotes |
|---|---|---|---|
| `loader.hubs_company` | ETL loader (external) | Periodica (minuti/ore?) | Customer selector: nuovi clienti appaiono con delay |
| `loader.hubs_deal` | ETL loader (external) | Periodica | Deal selector: nuovi deal appaiono con delay |
| `loader.hubs_owner` | ETL loader (external) | Periodica | Owner selector: raramente cambia |
| `loader.hubs_pipeline` | ETL loader (external) | Periodica | Pipeline filter: raramente cambia |
| `loader.hubs_stages` | ETL loader (external) | Periodica | Stage filter: raramente cambia |
| `loader.erp_metodi_pagamento` | ETL loader (external) | Periodica | Payment methods: raramente cambia |
| `loader.erp_anagrafiche_clienti` | ETL loader (external) | Periodica | Customer ERP data per trigger `quote_customer` |

**La quotes app non controlla la sync di queste tabelle**. Consuma dati caricati dal loader esterno. Se un deal nuovo non appare, non è un bug della quotes app.

### 4.2 Quote → HubSpot sync

| Direction | Trigger | Mechanism |
|---|---|---|
| App → HS | Publish esplicita (`POST .../publish`) | Backend REST calls |
| HS → App | Status changes (APPROVAL_NOT_NEEDED) | **Non esiste sync automatica**. Lo status HS viene letto on-demand via `GET .../hs-status`. |

**Implication**: Se un approvatore cambia lo status su HubSpot, la nostra app non lo sa finché qualcuno non apre la quote e il frontend chiama `hs-status`. Il campo `status` nel DB potrebbe essere stale.

**Question D1**: Serve un meccanismo di sync periodica HS→DB per lo status? O il read-on-demand è sufficiente? Oggi Appsmith fa la stessa cosa (legge lo status HS a ogni apertura del Dettaglio).

---

## 5. End-to-End User Journeys (in scope)

### 5.1 Journey: Creare una nuova proposta standard

```
1. Elenco Proposte → click "Nuova proposta"
2. Wizard step 1: cerca e seleziona deal attivo
3. Wizard step 2: configura tipo documento (TSC-ORDINE-RIC), servizi, billing, template
4. Wizard step 3: seleziona kit, configura prodotti per kit (varianti, quantità, NRC/MRC)
5. Wizard step 4 (opzionale): descrizione, note legali, contatti
6. Review: vede riepilogo con totali NRC/MRC
7. Conferma → quote salvata nel DB (DRAFT)
8. Redirect a Dettaglio (quote completa, modificabile)
```

### 5.2 Journey: Creare una nuova proposta IaaS

```
1. Elenco Proposte → click "Nuova proposta" → seleziona tipo "IaaS"
2. Wizard step 1: cerca e seleziona deal attivo
3. Wizard step 2: seleziona lingua → template (auto-filtrato IaaS)
   → kit e services auto-derivati dal template (DB columns)
   → term fields fissi (1 mese), billing fisso
   → trial slider (opzionale)
4. Wizard step 3: conferma kit auto-selezionato, configura prodotti
5. Review: riepilogo
6. Conferma → quote DRAFT nel DB
7. Redirect a Dettaglio
```

### 5.3 Journey: Modificare e pubblicare una proposta

```
1. Elenco Proposte → click sulla riga (o "Modifica")
2. Dettaglio si apre con tutti i dati (quote completa)
3. Tab Intestazione: modifica header fields, servizi, billing
4. Tab Kit e Prodotti: aggiunge/rimuove kit, riconfigura prodotti
   → badge "2/3 obbligatori" guida l'utente
5. Tab Note: scrive descrizione e/o note legali
   → banner warning se note legali presenti ("richiede approvazione")
6. Tab Contatti: compila riferimenti
7. Salva (esplicito) → dirty state cleared
8. Click "Pubblica su HubSpot"
   → Progress step-by-step
   → Se errore: messaggio chiaro, "Riprova" (idempotente)
   → Se successo: status aggiornato, link HS disponibile
9. (Opzionale) Click "Apri su HubSpot" → link diretto
10. (Opzionale) Click "Scarica PDF" → link PDF da HS
```

### 5.4 Journey: Eliminare una proposta

```
1. Elenco Proposte → seleziona riga → "Elimina" (visibile solo con ruolo)
2. Modal conferma (native <dialog>)
3. Backend: RBAC check → HS delete (se esiste) → DB delete
4. Lista aggiornata
```

### 5.5 Journey: Cercare e filtrare proposte

```
1. Elenco Proposte
2. Filter bar: pills per status (Tutti / Bozza / In approvazione / Approvate)
3. Search: testo libero (numero, cliente, deal)
4. Filtri avanzati: owner, date range, tipo documento
5. Preset: "Le mie proposte" (filtro owner = utente corrente)
6. Server-side: pagination + sort
```

---

## 6. Trigger e Automazioni DB

Comportamenti automatici a livello database che la nuova app deve conoscere ma non reimplementare.

| Trigger | Table | Event | Effect | App action needed |
|---|---|---|---|---|
| `set_timestamp` | `quotes.quote` | BEFORE UPDATE | `updated_at = now()` | Nessuna — automatico |
| `update_quote_customer_from_erp` | `quotes.quote` | INSERT | Snapshot dati fiscali cliente | Nessuna — automatico (A1 resolved) |
| `insert_product_rows_trigger` | `quotes.quote_rows` | AFTER INSERT | Espande kit → prodotti con traduzioni | App deve INSERT quote_rows; trigger fa il resto |
| `update_kit_product_rows_trigger` | `quotes.quote_rows` | AFTER UPDATE (kit_id) | Re-espande se kit cambia | App può cambiare kit_id su row esistente |
| `trigger_update_quote_row_totals` | `quotes.quote_rows_products` | AFTER INSERT/DELETE/UPDATE | Ricalcola nrc_row/mrc_row | App deve re-leggere row dopo update prodotto |

**Key implication for frontend**: Dopo ogni modifica a un prodotto, il frontend deve re-fetch i totali del kit row (aggiornati dal trigger). Il backend può includere i totali aggiornati nella response dell'update.

---

## 7. Data Boundaries and Ownership

```
┌─────────────────────────────────────────────────────────┐
│                    QUOTES APP SCOPE                      │
│                                                          │
│  WRITES:                          READS:                 │
│  ┌──────────────┐                ┌──────────────────┐    │
│  │ quotes.quote │                │ products.kit     │    │
│  │ quotes.      │                │ products.        │    │
│  │  quote_rows  │                │  product_category│    │
│  │ quotes.      │                │ products.product │    │
│  │  quote_rows_ │                │ products.        │    │
│  │  products    │                │  kit_product     │    │
│  └──────────────┘                │ loader.hubs_*    │    │
│                                  │ loader.erp_*     │    │
│  AUTO (trigger):                 │ common.*         │    │
│  ┌──────────────┐                └──────────────────┘    │
│  │ quotes.      │                                        │
│  │  quote_      │                READS (Alyante):        │
│  │  customer    │                ┌──────────────────┐    │
│  └──────────────┘                │ Tsmi_Anagrafiche │    │
│                                  │ Tsmi_Ordini      │    │
│  WRITES (HubSpot):               └──────────────────┘    │
│  ┌──────────────┐                                        │
│  │ HS quotes    │                READS (HubSpot):        │
│  │ HS line_items│                ┌──────────────────┐    │
│  └──────────────┘                │ HS quote status  │    │
│                                  │ HS associations  │    │
│  DOES NOT TOUCH:                 └──────────────────┘    │
│  quotes.template (new cols added by migration only)      │
│  products.* (read-only, managed by kit-products app)     │
│  loader.* (read-only, managed by ETL loader)             │
│  orders.* (deferred to order conversion phase)           │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

---

## 8. Questions

**D1**: ~~RESOLVED~~ — Esistono processi esterni di sincronizzazione HS→DB. La quotes app legge lo status dal DB locale (già aggiornato dal sync esterno). Non serve sync aggiuntivo nell'app.

# Phase D: Integration and Data Flow — Panoramica Cliente

## External Systems

| System | Type | Purpose | Access pattern | Pages |
|--------|------|---------|---------------|-------|
| **Mistra** | PostgreSQL | Orders, invoices, access lines, customer lists (via `loader.*` schema) | Go backend → SQL | Ordini ric., Ordini R&S, Fatture, Accessi |
| **Grappa** | MySQL | IaaS accounts, charges, active services | Go backend → SQL | IaaS PPU, Licenze Windows |
| **Anisetta** | PostgreSQL | Timoo tenants and PBX accounting | Go backend → SQL | Timoo |

**Excluded:** TIMOO REST API (too slow, replaced by Anisetta DB queries per user direction).

**Not used in this app:** HubSpot (no writes), Carbone (no PDFs), Alyante ERP (only indirectly via `loader.*` copies).

---

## Database Access Pattern

### Mistra PostgreSQL — `loader` schema

This app reads exclusively from the `loader` schema, which contains:
- **ERP staging tables** (`erp_clienti_con_fatture`, `erp_anagrafiche_clienti`, `erp_ordini`, `erp_righe_ordini`, `erp_anagrafica_articoli_vendita`) — imported/synced from Alyante ERP
- **ERP views** (`v_erp_fatture_nc`, `v_ordini_ricorrenti`, `v_ordini_sintesi`) — pre-joined/computed views
- **Grappa replica tables** (`grappa_foglio_linee`, `grappa_cli_fatturazione`, `grappa_profili`) — copies of Grappa MySQL tables
- **DB functions** (`get_reverse_order_history_path()`) — server-side logic

**Key insight:** This app does NOT access the canonical Mistra schemas (`customers`, `products`, `orders`, etc.). It only reads from `loader`, which is a staging/integration layer.

**Note:** The `loader` schema refresh frequency is out of scope for this migration.

### Grappa MySQL

Direct queries against Grappa production tables:
- `cdl_accounts` — Cloudstack billing accounts
- `cdl_charges` — daily usage charges
- `cli_fatturazione` — customer billing entities (joined for account display)

**No `loader` intermediary** for IaaS data — queries hit Grappa directly.

### Anisetta PostgreSQL

Direct queries against Anisetta `public` schema:
- `as7_tenants` — Timoo tenant list
- `as7_pbx_accounting` — PBX usage snapshots

---

## Cross-Database Data Flows

### Flow 1: Customer ID mapping (used across most pages)

```
Alyante ERP ID (numero_azienda)
    │
    ├── Mistra PG: loader.erp_*.numero_azienda
    │              loader.erp_clienti_con_fatture.numero_azienda
    │              loader.v_erp_fatture_nc.id_cliente
    │
    ├── Mistra PG (Grappa copies): loader.grappa_cli_fatturazione.codice_aggancio_gest
    │                               loader.grappa_cli_fatturazione.id (Grappa internal ID!)
    │
    └── Grappa MySQL: cli_fatturazione.codice_aggancio_gest
                      cli_fatturazione.id (Grappa internal ID!)
```

**Impact on API design:**
- Fatture, Ordini (both pages): customer identified by `numero_azienda` (ERP ID)
- Accessi: customer identified by `cli_fatturazione.id` (Grappa internal ID) — because the `loader.grappa_*` tables use Grappa IDs
- IaaS PPU: accounts linked via `cli_fatturazione.id` (Grappa internal ID)
- The backend must handle both ID spaces and never confuse them

**Decision:** As-is — each endpoint uses the ID type from its original query without normalization.

### Flow 2: Access line → Order context (Accessi page)

Original query cross-domain join (from `get_accessi_cliente`):
```sql
-- This join merges Grappa connectivity data with ERP order data
-- via serialnumber and codice_aggancio_gest bridge
SELECT ...
FROM
    loader.grappa_foglio_linee fl
        JOIN loader.grappa_cli_fatturazione cf ON fl.id_anagrafica = cf.id
        LEFT JOIN loader.grappa_profili p ON fl.id_profilo = p.id
        LEFT JOIN (
        SELECT *,
               ROW_NUMBER() OVER(PARTITION BY serialnumber ORDER BY data_documento DESC, progressivo_riga) AS rn
        FROM loader.v_ordini_ricorrenti
    ) r ON fl.serialnumber = r.serialnumber AND r.numero_azienda = cf.codice_aggancio_gest AND r.rn = 1
WHERE ...
```

This merges:
- Grappa connectivity data (access lines, profiles)
- ERP order data (recurring orders linked by serial number)
- Customer identity bridge (`codice_aggancio_gest`)

**Impact:** Must remain a single backend query. Cannot be split into separate API calls without losing the join efficiency.

### Flow 3: IaaS cascading selection

```
User selects account (tbl_accounts.selectedRow)
    │
    ├── get_daily_charges(domain)      → Daily charges table
    │       │
    │       └── User selects day (tbl_giornalieri.selectedRow)
    │               │
    │               └── get_charges_by_type(domain, day) → Pie chart
    │
    └── get_monthly_charges(domain)    → Monthly bar chart
```

Three levels of cascading data:
1. Account list (page load)
2. Daily + monthly charges (on account select)
3. Charge breakdown (on day select)

**Impact:** Frontend must manage three dependent data-fetch states. Each level depends on selection at the previous level.

---

## End-to-End User Journeys

### Journey 1: "What has this customer been billed for recently?" (Fatture)

1. User opens **Fatture** page → customer list auto-loads
2. User selects customer from dropdown (searchable)
3. Invoice lines auto-load (default: last 6 months)
4. User adjusts period slider (6/12/24/36/all) → table auto-refreshes
5. User browses line items, uses table search/filter
6. Optional: export to CSV

### Journey 2: "What are this customer's active recurring orders?" (Ordini ricorrenti)

1. User opens **Ordini ricorrenti** page → customer list + status list auto-load
2. User selects customer (required)
3. User selects order statuses (default: Evaso + Confermato)
4. User clicks "Cerca" → summary order table loads with visual row grouping
5. User clicks a row → slide-over panel opens with order metadata + line detail
6. User browses orders, sees order history chain (storico) in panel

### Journey 3: "Show me the full detail of this customer's orders" (Ordini R&S)

1. User opens **Ordini Ricorrenti e Spot** page → customer list + status list auto-load
2. User selects customer (required)
3. User selects order statuses (default: Evaso + Confermato)
4. User clicks "GO" → detail order table loads with visual row grouping
5. User clicks a row → slide-over panel (600px) opens with 4 tabs: Testata, Riga selezionata, Tutte le righe, Storico
6. User browses order detail — referents, product codes, families, stato_riga — in structured panel tabs

### Journey 4: "What connectivity lines does this customer have?" (Accessi)

1. User opens **Accessi** page → client list + connection types auto-load
2. User selects one or more clients (multi-select)
3. User optionally adjusts status filter (default: Attiva) and connection type filter (default: all)
4. User clicks "Cerca" → access lines load
5. User browses lines — sees linked order info, billing status, serial numbers

### Journey 5: "How much IaaS is this account consuming?" (IaaS PPU)

1. User opens **IaaS PPU** page → account table auto-loads
2. First account auto-selected → daily + monthly data loads
3. User clicks different account → data refreshes
4. User switches between "Giornaliero" and "Mensile" tabs
5. In daily tab: user clicks a specific day → pie chart shows charge breakdown by resource type

### Journey 6: "How many PBX users does this Timoo tenant have?" (Timoo)

1. User opens **Timoo** page → tenant list auto-loads from Anisetta DB
2. User selects tenant from dropdown
3. User clicks button → PBX stats load
4. User sees PBX instances with user/extension counts and totals

### Journey 7: "How many Windows licenses are active?" (Licenze Windows)

1. User opens **Licenze Windows** page → chart auto-loads
2. User views 14-day trend of daily Windows Server license counts
3. No interaction needed

---

## Background or Triggered Processes

**None.** This app has no:
- Scheduled tasks
- Background data sync
- Websocket connections
- Polling
- Timers
- Notifications

All data is fetched on-demand in response to user actions or page load.

---

## Data Ownership Boundaries

| Data | Owner system | This app's role | Write? |
|------|-------------|-----------------|--------|
| Customers | Alyante ERP → Mistra loader | Read (lookup) | No |
| Invoices/Credit notes | Alyante ERP → Mistra loader | Read (browse) | No |
| Orders + order lines | Alyante ERP → Mistra loader | Read (browse) | No |
| Access lines | Grappa → Mistra loader | Read (browse) | No |
| Connection profiles | Grappa → Mistra loader | Read (browse) | No |
| IaaS accounts | Grappa (direct) | Read (browse) | No |
| IaaS charges | Grappa (direct) | Read (browse) | No |
| Timoo tenants | Anisetta (direct) | Read (browse) | No |
| PBX accounting | Anisetta (direct) | Read (browse) | No |

**This app is entirely read-only.** No writes to any database or external system.

---

## Comparison with listini-e-sconti

| Aspect | listini-e-sconti | panoramica-cliente |
|--------|-----------------|-------------------|
| Read/Write | Read + Write (pricing, credits, discounts) | **Read-only** |
| Databases | Mistra (canonical schemas) + Grappa | Mistra (loader schema only) + Grappa + Anisetta |
| External APIs | HubSpot (audit notes/tasks), Carbone (PDF) | **None** |
| Side effects | DB mutations + HubSpot calls | **None** |
| Complexity driver | Business rules, validation, audit trail | Cross-database joins, data volume, chart rendering |
| Shared infrastructure | Customer ID mapping, Grappa exclusions (385/485) | Same customer ID mapping, same Grappa exclusions |
| Navigation | TabNavGroup (4 groups) | TabNavGroup (same pattern, different groups) |
| Auth | `app_listini_access` | `app_panoramica_access` |

---

## API Contract Sketch

Based on entities and data flows, the backend needs approximately these endpoints:

### Mistra endpoints (loader schema)

| Endpoint | Method | Purpose | Original query | Used by |
|----------|--------|---------|---------------|---------|
| `GET /api/v1/panoramica/customers/with-invoices` | GET | Customer list (invoice context) | `get_clienti_con_fatture` | Fatture |
| `GET /api/v1/panoramica/customers/with-orders` | GET | Customer list (order context) | `get_aziende_con_ordini` / `GET_aziendeConOrdini` | Ordini ric., Ordini R&S |
| `GET /api/v1/panoramica/customers/with-access-lines` | GET | Customer list (access line context) | `get_clients_accessi` | Accessi |
| `GET /api/v1/panoramica/order-statuses` | GET | Distinct order statuses | `get_stati_ordine` / `GET_StatiOrdine` | Ordini ric., Ordini R&S |
| `GET /api/v1/panoramica/orders/summary` | GET | Orders summary view | `get_ordini_ricorrenti` | Ordini ricorrenti |
| `GET /api/v1/panoramica/orders/detail` | GET | Orders full detail | `GET_ordini_Ric_Spot` | Ordini R&S |
| `GET /api/v1/panoramica/invoices` | GET | Invoice lines by customer + period | `get_fatture` | Fatture |
| `GET /api/v1/panoramica/connection-types` | GET | Distinct connection types | `get_tipo_conn` | Accessi |
| `GET /api/v1/panoramica/access-lines` | GET | Access lines with filters | `get_accessi_cliente` | Accessi |

### Grappa endpoints

| Endpoint | Method | Purpose | Original query | Used by |
|----------|--------|---------|---------------|---------|
| `GET /api/v1/panoramica/iaas/accounts` | GET | Active IaaS accounts | `get_cdl_accounts` | IaaS PPU |
| `GET /api/v1/panoramica/iaas/daily-charges` | GET | Daily charges by domain | `get_daily_charges` | IaaS PPU |
| `GET /api/v1/panoramica/iaas/monthly-charges` | GET | Monthly charges by domain | `get_monthly_charges` | IaaS PPU |
| `GET /api/v1/panoramica/iaas/charge-breakdown` | GET | Day charge breakdown by type | `get_charges_by_type` | IaaS PPU |
| `GET /api/v1/panoramica/iaas/windows-licenses` | GET | Windows license daily counts | `get_licenses_by_day` | Licenze Windows |

### Anisetta endpoints

| Endpoint | Method | Purpose | Original query | Used by |
|----------|--------|---------|---------------|---------|
| `GET /api/v1/panoramica/timoo/tenants` | GET | Tenant list | `getAnisettaTenants` | Timoo |
| `GET /api/v1/panoramica/timoo/pbx-stats` | GET | PBX stats + totals by tenant | `getPbxByTenandId` + JS aggregation | Timoo |

**Total: ~16 GET endpoints, 0 write endpoints.**

**Note:** API path prefix follows the convention established by other apps in the monorepo.

# Customer Portal — Application Inventory

Source: `apps/customer-portal/customer-portal.json.gz` (Appsmith export, `clientSchemaVersion=2.0`, `serverSchemaVersion=11.0`).

## Application
- **Name**: Customer Portal
- **Slug**: `customer-portal`
- **Icon / color**: `joystick`, `#F1DEFF`
- **Navigation**: sidebar, static position, theme color, shows sign-in
- **Layout**: FLUID, max-width LARGE
- **Custom JS libs**: none
- **Evaluation version**: 2.0

This is the **back-office / admin companion** to the real Customer Portal end-user app. All copy is Italian and the UI is aimed at CDLAN internal staff managing customers, their admin users, biometric access requests, and (aspirationally) a document area.

## Pages (navigation order preserved)
| # | Page | Default | Purpose (inferred) |
|---|------|---------|--------------------|
| 1 | Home | yes | Static welcome text only |
| 2 | Stato Aziende | no | List customers and edit the "stato" (state) of a customer |
| 3 | Gestione Utenti | no | Per-customer user list + create new admin user |
| 4 | Accessi Biometrico | no | Review biometric access requests, mark each as completed |
| 5 | Area documentale | no | UI shell for category + document management — **not wired** |

## Datasources
| Name | Plugin | Target | Mode |
|------|--------|--------|------|
| `mistra` | postgres-plugin | `10.129.32.20` (Mistra PostgreSQL) | READ_WRITE, direct SQL from UI |
| `GW interno CDLAN - S` | restapi-plugin | `https://gw-int.cdlan.net` | REST, no auth header captured in export |

Both datasources are consumed directly from widget bindings; no server-side middleware in the app itself.

## Queries / Actions
16 total. Full bodies in `DATASOURCE_CATALOG.md`.

| Page | Name | Kind | Datasource | Verb / intent |
|------|------|------|------------|---------------|
| Stato Aziende | GetAllCustomer | REST | GW interno CDLAN - S | GET `/customers/v2/customer` |
| Stato Aziende | GetAllCustomerState | REST | GW interno CDLAN - S | GET `/customers/v2/customer-state` |
| Stato Aziende | EditCustomer | REST | GW interno CDLAN - S | PUT `/customers/v2/customer/{id}` |
| Gestione Utenti | GetAllCustomer | REST | GW interno CDLAN - S | GET `/customers/v2/customer` (duplicate of above) |
| Gestione Utenti | GetAllUsersByCustomer | REST | GW interno CDLAN - S | GET `/users/v2/user?customer_id=…` |
| Gestione Utenti | NewAdmin | REST | GW interno CDLAN - S | POST `/users/v2/admin` |
| Accessi Biometrico | GetTableData | SQL | mistra | SELECT join across `customers.biometric_request`, `customers.user_struct`, `customers.customer`, `customers.user_entrance_detail` |
| Accessi Biometrico | UpdateRequestCompleted | SQL | mistra | `SELECT customers.biometric_request_set_completed(id, stato_richiesta)` stored-fn call |
| Accessi Biometrico | Api1 | REST | GW interno CDLAN - S | **empty path**, dead |
| Accessi Biometrico | onSave, onToggle, onDiscard, onToggleCompletion, onSaveCompletion, Table1primaryColumnsstato_richiestaonCheckChange | JS actions | UNUSED_DATASOURCE | JSObject-backed helpers (see below) |
| Area documentale | Query1 | SQL | mistra | `SELECT * FROM accounting."acct_log" LIMIT 10` — **placeholder / unused** |

## JSObjects (action collections)
- **JSObject1** on "Accessi Biometrico" (3172 chars): defines `onToggle`, `onSave`, `onToggleCompletion`, `onSaveCompletion`, `onDiscard`. Operates on `appsmith.store.tableData` + `appsmith.store.updatedRowIndices` and on `BiometricData.processedTableData` / `BiometricData.triggeredRowIndex`. **No widget currently binds to any of these methods** (see Findings).
- **JSObject2** on "Accessi Biometrico" (97 chars): single empty method `Table1primaryColumnsstato_richiestaonCheckChange()` — dead stub.

## Global findings (summary — full list in `FINDINGS.md`)
- Direct PostgreSQL access from the UI (`mistra` datasource) alongside calls to the internal REST gateway — **inconsistent trust boundary**.
- Credentials / auth for `GW interno CDLAN - S` are not in the export; the UI has no visible auth header logic, so Appsmith is injecting auth at runtime. Any rewrite must reproduce that (cookie/session or added header).
- Two pages (`Area documentale`, `Home`) are effectively placeholders; `Area documentale` has a full modal/table scaffold but no data queries, no save actions, and no onLoad actions.
- `Accessi Biometrico` contains two parallel, inconsistent implementations of the row-edit flow: (a) a live `EditActions1` column + direct stored-function call; (b) a set of JSObject1 methods that would drive a client-side staging buffer. Only (a) is wired.
- Business rule `ok`/`pending` string values vs. SQL boolean `request_completed` — see Findings.
- Column labels are mostly the raw DB/API field names (`nome`, `cognome`, `email`, `azienda`, `tipo_richiesta`, `stato_richiesta`) — weak UX, likely to be re-labeled in the rewrite.
- Several typos / broken callbacks in wired handlers (`howAlert('success')` and `Error(UpdateRequestCompleted.responseMeta.statusCode)` in wrong branch) — see Findings.

# Application inventory — Ordini

## Metadata
- **Application name:** Ordini
- **App slug:** `ordini`
- **Source type:** Single Appsmith application export (JSON), `artifactJsonType: APPLICATION`, `clientSchemaVersion 2.0`, `serverSchemaVersion 11.0`
- **Source file:** `apps/ordini/Ordini.json.gz` (ungzipped `Ordini.json`)
- **Evaluation version:** 2
- **Navigation:** sidebar, only `Home` is visible; all other pages are hidden and reached via `navigateTo()` from widgets
- **Theme:** default Appsmith theme, no custom JS libraries (`customJSLibList: [ {} ]` only contains an empty placeholder entry)

## Pages
| Page | Visible | Purpose (inferred) | Widgets | On-load queries |
|---|---|---|---|---|
| `Home` | yes | Main landing: paginated list of orders (`orders` table), search, row action "Visualizza" that navigates to `Dettaglio ordine`. Also contains a legacy `Dettaglio_ordine` modal with full detail view that appears unused (the row button navigates to the dedicated page instead). | 58 | `Select_Orders_Table`, `Dettaglio_ordine_vero`, `Lista_righe_d_ordine` |
| `Ordini semplificati` | hidden | Scaffold/placeholder. Table bound to HubSpot potentials via `get_potentials`, plus a default-config `ButtonGroup1` (Favorite/Add/More with placeholder options) and an empty `Form1` gated by `utils.globals["formVisibile"]`. No submit wiring. | 8 | `get_potentials` |
| `Draft gp da offerta` | hidden | Empty (`MainContainer` only, no widgets, no actions). | 1 | none |
| `Form ordine` | hidden | New-order form UI — anagraphic, service/billing terms, contact/referents, nested rows table. Only two buttons exist: "Verifica numeri d'ordine" (Button1) and "Aggiungi riga" (Button2), both with no `onClick` wired. No Submit action. Effectively a non-functional draft. | 62 | `Dettaglio_ordine_vero`, `Lista_righe_d_ordine` |
| `Dettaglio ordine` | hidden | Primary working page. Receives `?id=` via URL, loads the order + its rows, exposes 6 tabs (Info, Azienda, Referenti, Righe, Informazioni dai tecnici, Arxivar link). Implements the full lifecycle: edit ragione sociale/PO/data conferma (BOZZA) → upload Arxivar PDF → send to ERP (INVIATO) → per-row activation date (INVIATO → ATTIVO) → cancellation/lost. Also download kick-off / activation form / order PDF. | 92 | `Order`, `RigheOrdine`, `RigheOrdineTecnici`, `erp_anagrafiche_cli` |

## Datasources
| Name | Plugin | Host / Port | Notes |
|---|---|---|---|
| `Alyante` | `mssql-plugin` | `172.16.1.16:1433` | SSL NO_VERIFY. READ_WRITE. Only one read query (`erp_anagrafiche_cli` → `Tsmi_Anagrafiche_clienti`). |
| `db-mistra` | `postgres-plugin` | `10.129.32.20` (default port) | SSL DEFAULT. Used by `Ordini semplificati` for HubSpot loader tables and `loader.erp_metodi_pagamento`. |
| `vodka` | `mysql-plugin` | `10.129.32.7:3306` | SSL DEFAULT. Primary DB; all orders CRUD (`orders`, `orders_rows`). |
| `GW internal CDLAN` | `restapi-plugin` | `https://gw-int.cdlan.net` | Internal gateway (authoritative path to ERP + PDF generation + Arxivar upload). No headers/query-params declared at the datasource level. |

Credentials are stripped from the export (expected).

## Queries / actions — 55 total
55 actions registered. Several are `UNUSED_DATASOURCE` — these are **JSObject methods serialized as actions** by the Appsmith export (duplicates of what is in the `actionCollectionList` JSObjects). They are evidence, not extra logic.

Grouped by role:

- **Order listing** (Home): `Select_Orders_Table` (onLoad), `Total_record_orders1`, `Select_orders1`, `Insert_orders1`, `Update_orders1`, `Dettaglio_ordine_vero`, `Lista_righe_d_ordine`, `Lista_righe_d_ordine_info_tecn`, `Dettaglio_riga_d_ordine`, `Query1` (orphan).
- **HubSpot potentials / payment methods** (Ordini semplificati): `get_potentials` (onLoad), `get_payment_methods` (defined but **no widget bind observed**).
- **Detail page loads** (Form ordine, Dettaglio ordine): `Dettaglio_ordine_vero`, `Lista_righe_d_ordine`, `Order`, `RigheOrdine`, `RigheOrdineTecnici`, `erp_anagrafiche_cli`.
- **Order state mutations** (Dettaglio ordine → vodka): `SaveDataConfermaRifOrderCli`, `SaveActivationDate`, `UpdateOrderState` (→ INVIATO), `SetOrderStateAttivo` (→ ATTIVO), `CheckConfirmRows`, `SaveOrderReferents`, `order_perso` (→ PERSO), `upd_row_serNum`, `upd_row_note_tecnici`.
- **ERP + PDF + Arxivar** (Dettaglio ordine → GW REST): `GW_Kickoff`, `GW_SendToErp`, `GW_SetActivationDate`, `GW_ActivationForm`, `GW_SavePdfToArxivar`, `GW_GetPDFArxivarOrder`, `DownloadOrderPDFintGW`, `GW_CancelOrder`, `GW_SendRequestAnnullaOdv` (duplicate/orphan cancel path).

## JSObjects — 10 total
| Page | JSObject | Methods | Purpose |
|---|---|---|---|
| Home | `JSObject1` | `myFun1`, `myFun2` | Empty boilerplate, no usage. |
| Ordini semplificati | `utils` | `myFun1`, `myFun2`; `globals: { formVisibile: false }` | Only `globals.formVisibile` is read by `Form1.isVisible`. No code logic. |
| Dettaglio ordine | `SendToErp` | `run`, `setState` | Orchestrates multi-row `GW_SendToErp`, then `UpdateOrderState`, then optional `GW_SavePdfToArxivar`, then `navigateTo('Home')`. |
| Dettaglio ordine | `SetActivationDate` | `run`, `saveInVodka`, `saveInErp`, `checkRows`, `SetOrderStateAttivo` | Dual-write of per-row activation date (vodka + ERP), counts confirmed rows, auto-promotes order state to ATTIVO when all rows confirmed. |
| Dettaglio ordine | `GetPdf` | `kickOff`, `activationForm` | Calls GW endpoints that return a PDF and triggers `download()`. |
| Dettaglio ordine | `GetPdfOrdineArx` | `GetPdfOrdineArx(orderId)` | Downloads signed-order PDF from Arxivar via GW; decodes base64-or-raw; blob download. |
| Dettaglio ordine | `OrderTools` | `download(orderId)` | Downloads the generated order PDF via `DownloadOrderPDFintGW` using the same base64-or-raw decode. |
| Dettaglio ordine | `SendRequestAnnullaOdv` | `run` | Calls `GW_SendRequestAnnullaOdv` with `cdlanSystemodv`. Likely orphan — the visible "RICHIEDI ANNULLAMENTO" button calls `GW_CancelOrder` directly, not this wrapper. |
| Dettaglio ordine | `utili` | `salvaRiga`, `salvaNoteTecniche` | Persist table row edits (serial number / technical notes) back to vodka and refresh the data queries. |
| Dettaglio ordine | `JSObject1` | `myFun1` | Debug stub (`console.log(cdlan_dataconferma.formattedDate)`); no usage. |

## Navigation patterns
- `Home → Dettaglio ordine` via row iconButton `customColumn1.onClick`: `navigateTo('Dettaglio ordine', {id: Lista_ordini.triggeredRow.id}, 'SAME_WINDOW').then(() => Dettaglio_ordine_vero.run(...))` — note the post-navigate `.run()` call targets a query on the **Home** page (which is now unmounted), suggesting legacy code; the target page loads its own `Order` via `appsmith.URL.queryParams.id`.
- `Dettaglio ordine → Home`:
  - `TornaIndietro` button (`navigateTo('Home')`).
  - `SendToErp.run()` finishes with `navigateTo('Home')` after a successful ERP push.
- `Home.Nuovo_Ordine` button → `navigateTo('Form ordine')`, but the widget is `isVisible: false` and `isDisabled: true`, so the path is dead.
- `Form ordine` has no navigation/submit wiring at all.
- Modals: `Dettaglio_ordine` (Home, legacy), `ModificaRiga` (Dettaglio ordine), `Modal1` (Dettaglio ordine, shows an Arxivar link in an HTML anchor).

## Cross-page reuse
- **Query name collision:** `Dettaglio_ordine_vero`, `Lista_righe_d_ordine` exist both on Home (keyed off `Lista_ordini.triggeredRow.id`) and on Form ordine (keyed off `appsmith.URL.queryParams.id`). Same SQL, different parameter source.
- JSObject names `JSObject1` appear on both Home and Dettaglio ordine (both are boilerplate stubs).
- No shared datasources beyond the four declared; no shared widgets.

## Global notes / migration risks
- **Direct multi-DB access from the UI**: `vodka` (MySQL), `Alyante` (MSSQL), `db-mistra` (Postgres), plus REST to `GW internal CDLAN`. A rewrite must hide all of these behind Go backend endpoints — the frontend must not hold DB credentials.
- **SQL injection surface**: `Lista_ordini.searchText` (via `LIKE '%…%'`), `Lista_ordini.sortOrder.column`, all `order_id.text` / `ref_order_id.text` values. Every Home query and every mutation on Dettaglio ordine is a raw-string interpolation. Parameterized queries are mandatory in the rewrite.
- **Cross-database entity mapping**: `Tsmi_Anagrafiche_clienti` (Alyante) feeds the "Ragione sociale" dropdown, but the chosen value is saved back as a string into `orders.cdlan_cliente` (not a foreign key). No ID is persisted in the order — only the display name. This is fragile and should map to the canonical identity mapping in `docs/IMPLEMENTATION-KNOWLEDGE.md` (Alyante ID = Mistra customer.id = Grappa codice_aggancio_gest).
- **Authorization is client-side only**: visibility / disabled state uses `appsmith.user.groups.includes('CustomerRelations')`. The Go backend must enforce this on the server; do not trust client gating. Use the `app_ordini_access` / similar role and the `CustomerRelations` group check must be re-expressed via Keycloak role membership.
- **Order state machine implicit in widget bindings**: BOZZA → INVIATO → ATTIVO, with side branches PERSO and ANNULLATO. The state transitions and their pre-conditions are scattered across `isVisible`/`isDisabled` and JSObject `if`-checks; see `page-audit.md` and `findings-summary.md`.
- **Dual-write is the norm**: activation date, state transitions and row data are written both to `vodka` and to the ERP via GW. There is no transactional guarantee between the two. JSObjects use `Promise.all` and inspect each result independently; partial failures surface as UI alerts but leave divergent state.
- **Dead/legacy code**: Home's `Dettaglio_ordine` modal, `Query1`, `Insert_orders1`, `Update_orders1`, `Total_record_orders1`, `Select_orders1`, most of `Form ordine`, the whole `Ordini semplificati` page, `SendRequestAnnullaOdv` JSObject, `GW_SendRequestAnnullaOdv`. See `findings-summary.md` for the complete list — do not port any of it blindly.
- **Unfinished features**: `Form ordine` has no submit/add-row handlers; `Ordini semplificati` has no form action; `Nuovo_Ordine` on Home is hidden+disabled. Order creation is **not** done through this Appsmith app — it likely happens elsewhere (the Customer Portal / ERP) and this app is effectively read + lifecycle-management over already-existing orders.
- **`confirm_data_attivazione=1` is a client-written side-effect**: `SaveActivationDate` sets `confirm_data_attivazione = 1` unconditionally when the row's activation date is saved, and `CheckConfirmRows` then counts those to decide if the whole order is ATTIVO. This business rule is invisible unless the code is read and must be preserved in the rewrite.
- **Credential stripping**: no `authentication` block on datasources; connection strings and REST auth must be recovered from the live Appsmith instance or re-provisioned.

# AFC Tools — Migration Specification

Downstream input for `portal-miniapp-generator`. Self-contained: no need to open the original Appsmith export.

## Summary

- **Application name**: AFC Tools
- **Audit source**: `apps/afc-tools/audit.md` (derived from `AFC-Tools.json.gz`)
- **Spec status**: approved for implementation planning
- **Scope directive**: **1:1 porting** — preserve current user-visible behavior verbatim; fixes and redesigns are out of scope except for the five expert-approved deviations listed below.
- **Expert-approved deviations from 1:1**:
  - Fix the `cdlan_cod_termini_pag == 400` ternary typo on Dettaglio ordini (decision A.5.1a).
  - Treat `null` as equivalent to empty string for `cdlan_note` and `data_decorrenza` (A.5.1c).
  - Default the `TXT_anno` input on Consumi Energia Colo to the current year (A.5.1d).
  - Move carbone.io `templateId` server-side (required by mini-app architecture; A.5.4 = 4a).
  - Drop dead UI (`Dettaglio_ordine` modal on Ordini Sales) and orphan queries/JSObjects (A.5.2, A.5.3).
- **Deliberately preserved anomalies** (logged in `docs/TODO.md` → "AFC Tools App"):
  - `cdlan_dur_rin` = 4 vs `cdlan_int_fatturazione` = 5 for "Quadrimestrale" (A.5.1b).
  - `SELECT *` without LIMIT/filter on `Tsmi_DDT_Verifica_Cespiti` (A.5.1e).

Companion phase docs: `afc-tools-migspec-phaseA.md` (entities), `afc-tools-migspec-phaseB.md` (UX + verbatim SQL appendix), `afc-tools-migspec-phaseC.md` (logic placement + datasource plan), `afc-tools-migspec-phaseD.md` (endpoints + user journeys).

## Current-State Evidence

- **Source pages (8)**: Transazioni whmcs, Fatture Prometeus, Nuovi articoli da inserire, Report XConnect e Remote Hands, Consumi variabili Energia Colo, Ordini Sales, Dettaglio ordini, Report DDT per cespiti.
- **Source entities (9)**: WhmcsTransaction, WhmcsInvoiceLine, MistraProductMissingInAlyante, RemoteHandsTicket, XConnectOrder, EnergiaColoConsumption, SalesOrderSummary, SalesOrder, DdtVerificaCespiti. Pure read app — no writes to any backing system.
- **Source integrations**: 5 databases (WHMCS MySQL, Vodka MySQL, Grappa MySQL, Mistra PG, Alyante MSSQL) + 1 internal gateway (`gw-int.cdlan.net` for PDF download) + 1 external service (carbone.io for XLSX render).
- **Source JSObjects (relevant)**: `utils.runReport` (carbone orchestration), `TicketTools.downloadTicketPDF`, `OrderTools.download`. Two empty placeholders (`JSObject1`, `myFun1/2`) and one current-year helper (`printCurrentYear`, used only as input placeholder text) are dropped.
- **Audit gaps resolved**: Q-A1..Q-A6 resolved by reading `AFC-Tools.json.gz` directly — see Phase B Appendix for every query's verbatim SQL.

## Entity Catalog

### Entity: WhmcsTransaction
- **Purpose**: WHMCS (Prometeus) payment or refund record.
- **Operations**: `list(from, to)`, `exportXlsx(from, to)`.
- **Fields**: `cliente` (string), `fattura` (string), `invoiceid` (int), `userid` (int), `payment_method` (string), `date` (string `YYYY-MM-DD` — formatted server-side from a `YYYYMMDD` int column), `description` (string), `amountin` (decimal), `fees` (decimal), `amountout` (decimal), `rate` (decimal), `transid` (string), `refundid` (int), `accountsid` (int).
- **Relationships**: logical link to `WhmcsInvoiceLine` via `invoiceid` / `fattura`; no UI cross-link.
- **Business rules (preserved)**: floor `date > 20230120`; filter `(fattura <> '' AND invoiceid > 0) OR refundid > 0`.
- **Open questions**: none.

### Entity: WhmcsInvoiceLine
- **Purpose**: WHMCS → Alyante invoice-line feed (audit view).
- **Operations**: `listLatest()` — last 2000 rows by `id DESC`.
- **Fields (30)**: `raggruppamento`, `ragionesocialecliente`, `nomecliente`, `cognomecliente`, `partitaiva`, `codicefiscale`, `codiceiso`, `flagpersonafisica`, `indirizzo`, `numerocivico`, `cap`, `comune`, `provincia`, `nazione`, `numerodocumento`, `datadocumento`, `causale`, `numerolinea`, `quantita`, `descrizioneriga`, `prezzo`, `datainizioperiodo`, `datafineperiodo`, `modalitapagamento`, `ivariga`, `bollo`, `codiceclienteerp`, `tipo`, `invoiceid`, `id`.
- **Business rules (preserved)**: no user filter; `ORDER BY id DESC LIMIT 2000`.

### Entity: MistraProductMissingInAlyante
- **Purpose**: Mistra products flagged for ERP sync but not yet present in the Alyante master data.
- **Operations**: `list()`.
- **Fields**: `code`, `categoria`, `descrizione_it`, `descrizione_en`, `nrc` (decimal), `mrc` (decimal).
- **Business rules (preserved)**: `erp_sync = true` + anti-join on `loader.erp_anagrafica_articoli_vendita` (`a.cod_articolo IS NULL`); IT/EN descriptions pivoted via `MAX(CASE … language='it'|'en')`.

### Entity: RemoteHandsTicket
- **Purpose**: downloadable PDF of a Remote Hands ticket (read-only reference).
- **Operations**: `downloadPdf(ticketId, lang ∈ {it, en})`.
- **Fields**: none persisted in UI state — payload is the PDF blob.
- **Business rules (preserved)**: `ticket_type=RemoteHands` hard-pinned on the gateway call.

### Entity: XConnectOrder
- **Purpose**: completed XConnect orders with per-row PDF download.
- **Operations**: `listEvaso()`, `downloadPdf(orderId)`.
- **Fields**: `id_ordine`, `codice_ordine`, `cliente`, `data_creazione`.
- **Business rules (preserved)**: `kit_category='XCONNECT' AND state='EVASO'`; 404 on PDF → toast "Il PDF non è ancora pronto.".

### Entity: EnergiaColoConsumption
- **Purpose**: colocation energy consumption per customer, with 12-month pivot and per-period detail.
- **Operations**: `listMonthlyPivot(year)`, `listDetail(year)`.
- **Fields (pivot)**: `customer`, `Gennaio…Dicembre` (decimal sums).
- **Fields (detail)**: `customer`, `start_period`, `end_period`, `consumo`, `amount`, `pun`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti`, `tipo_variabile`.
- **Business rules (preserved)**: metric `IF(ampere > 0, ampere, Kw)`; year interpolated as string into `year(...)` (now parameterized server-side as `int`).
- **Open questions (not blocking 1:1)**: business semantics of `tipo_variabile`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti` — fields displayed verbatim.

### Entity: SalesOrderSummary
- **Purpose**: list of Vodka/daiquiri sales orders in `ATTIVO` or `INVIATO` state.
- **Operations**: `listActive()`.
- **Fields**: `id`, `cdlan_tipodoc`, `cdlan_ndoc`, `cdlan_anno`, `Codice ordine` (derived `CONCAT(cdlan_ndoc,'/',cdlan_anno)`), `cdlan_sost_ord`, `cdlan_cliente`, `cdlan_datadoc`, `Tipo di servizi` (coalesce `IF(is_colo!=0, is_colo, service_type)`), `Tipo di ordine` (mapped A/N/R → Sostituzione/Nuovo/Rinnovo), `cdlan_dataconferma`, `cdlan_stato`, `Dal CP?` (mapped `IF(from_cp!=0,'Sì','No')`).
- **Relationships**: drill-down to `SalesOrder` by `id`.
- **Business rules (preserved)**: all four SQL transformations above, `cdlan_stato IN ('ATTIVO','INVIATO')`, `ORDER BY cdlan_datadoc DESC`.

### Entity: SalesOrder
- **Purpose**: full header + rows of a single Vodka order.
- **Operations**: `get(id)`, `listRows(id)`.
- **Fields (header)**: 50+ columns — full list in Phase B Appendix §B.4.10. Notable server-computed fields: `cdlan_int_fatturazione_desc` (label for codes 1/2/3/5/6/else), `cdlan_int_fatturazione_att_desc` (label for codes 1/else).
- **Fields (rows)**: `ID Riga`, `System ODV Riga`, `Codice articolo bundle` (derived `IF(cdlan_codice_kit != '', CONCAT(cdlan_codice_kit,'-',index_kit), '')`), `Codice articolo`, `Descrizione articolo`, `Canone`, `Attivazione`, `Quantità`, `Prezzo cessazione`, `Codice raggruppamento fatturazione`, `Data attivazione`, `Numero seriale`, `confirm_data_attivazione`, `data_annullamento`.
- **Business rules (preserved)**: all enumeration mappings (tipodoc, tipo_ord A/N/R, dur_rin 1/2/3/4/6/12, tacito_rin 1→Sì, int_fatturazione 1/2/3/5/6, int_fatturazione_att 1/else, cod_termini_pag 18 codes); kit-code composition rule.
- **Business rules (deviation from 1:1, expert-approved)**: `cod_termini_pag` typo fixed so code 400 → "SDD FM"; `cdlan_note` / `data_decorrenza` treat `null` and `''` equivalently.
- **Business rules (preserved with TODO)**: Quadrimestrale is code 4 on `cdlan_dur_rin` and code 5 on `cdlan_int_fatturazione` — divergence preserved, flagged for follow-up.

### Entity: DdtVerificaCespiti
- **Purpose**: full dump of Alyante MSSQL view `Tsmi_DDT_Verifica_Cespiti`.
- **Operations**: `list()`.
- **Fields**: defined by the live MSSQL view (the `SELECT *` projection — not present in the Appsmith export). Backend exposes whatever columns the view returns at runtime; frontend binds dynamically.
- **Business rules (preserved with TODO)**: no filter, no pagination.

## View Specifications

Each view follows the pattern library introduced in Phase B (R1..R6). The mini-app shell is React Router + `AppShell` from `@mrsmith/ui` + `TabNavGroup` (the reports / energia-dc pattern).

### View: /transazioni-whmcs (R1 — list-with-date-range)
- **User intent**: ispezionare/esportare le transazioni WHMCS in un range di date.
- **Interaction**: two date pickers (default: 15 days back / today), Cerca button (runs query), Esporta button (runs query + carbone).
- **Main data**: 14-column transactions table (`tbl_transactions`).
- **Entry**: tab click → empty state.
- **Exit**: Esporta → XLSX URL opens in a new tab.
- **Preserved / changed**: no auto-load (preserved); backend owns carbone template id and API token (4a).

### View: /fatture-prometeus (R2 — read-only dump)
- **User intent**: audit delle ultime 2000 righe fattura WHMCS→Alyante.
- **Interaction**: auto-load on mount; no filter.
- **Main data**: 30-column invoice-lines table.
- **Preserved**: 2000-row cap.

### View: /nuovi-articoli (R2)
- **User intent**: articoli Mistra con `erp_sync=true` da creare in Alyante.
- **Interaction**: auto-load on mount.
- **Main data**: 6-column products table.

### View: /report-xconnect-rh (R3 — mixed-tab canvas)
- **User intent**: due flussi PDF-download distinti.
- **Interaction**:
  - Tab 1: manual input (ticket number, lang select `it`/`en`), Scarica PDF button, required-field toasts on empty.
  - Tab 2: auto-loaded list of EVASO XConnect orders, per-row "Scarica PDF" button.
- **Preserved**: `ticket_type=RemoteHands` pin; 404 toast copy; native Blob download (replacing the base64 heuristic — same user outcome).

### View: /consumi-energia-colo (R4 — year-filtered report)
- **User intent**: visualizzare consumi colocation per anno, vista pivot + dettaglio.
- **Interaction**: `TXT_anno` input default = current year (deviation 1:1d), Cerca button runs both queries in parallel, two tables below.
- **Main data**: pivot (13 cols: customer + 12 mesi), detail (11 cols).

### View: /ordini-sales (R5 — list → detail)
- **User intent**: operare sugli ordini Vodka attivi o inviati.
- **Interaction**: auto-load on mount; row icon → `/ordini-sales/:id`.
- **Main data**: 12-column orders list, SQL-side label mappings preserved.
- **Dropped**: dead modal widget.

### View: /ordini-sales/:id (R6 — read-only detail)
- **User intent**: header + righe di un ordine.
- **Interaction**: two parallel fetches (Order, RigheOrdine) on mount; "Torna indietro" button → `/ordini-sales`.
- **Main data**: ~55-field header (text widgets with enum-mapping ternaries) + 14-column rows table.
- **Preserved / changed**: all ternary mappings verbatim, `<b>` → bold typography, `cdlan_cod_termini_pag == 400` typo fixed (A.5.1a), null-as-empty on two fields (A.5.1c).

### View: /report-ddt-cespiti (R2)
- **User intent**: dump full della view Alyante cespiti.
- **Interaction**: auto-load on mount.
- **Main data**: dynamic columns from the MSSQL view.
- **Preserved with TODO**: no pagination / no filter.

## Logic Allocation

### Backend responsibilities (`backend/internal/afctools/`)
- All SQL against WHMCS, Vodka, Grappa, Mistra, Alyante — preserved verbatim.
- All SQL-side label CASE / coalesce expressions — left server-side.
- Kit code composition (`IF/CONCAT`) — left server-side.
- `ticket_type=RemoteHands` pin on Remote Hands ticket proxy.
- Carbone.io orchestration: runs query, POSTs to carbone with backend-owned `templateId` + API token, returns `{renderId, renderUrl}`.
- Gateway (PDF) proxy with OAuth2 token.
- Authz via Keycloak role `app_afctools_access` on every endpoint.

### Frontend responsibilities (`apps/afc-tools/src/`)
- React Router with the 8 routes in Phase D §D.1.
- React Query for all endpoints; cache keys local per view.
- `@mrsmith/ui` Table (client-side pagination) + DatePicker + SearchInput + ToastProvider.
- Enum→label lookup modules for Dettaglio ordini ternaries (payment terms, `tipo_ord`, `dur_rin`, `tacito_rin`, `tipodoc`) + one `isEmpty()` helper.
- XLSX export: `window.open(renderUrl, '_blank')`.
- PDF download: native `fetch` → `Blob` → `URL.createObjectURL` → `<a download>`.

### Shared validation / formatting
- DTOs generated via `@mrsmith/api-client` (OpenAPI spec emitted by the Go backend).
- `lang` enum (`it | en`) exported once.

### Rules being revised rather than ported
- `cdlan_cod_termini_pag == 400` → now correctly maps to "SDD FM".
- `cdlan_note` / `data_decorrenza` → render "Nessun valore" / "Nessuna nota legale" on both `null` and `''`.
- Consumi Energia Colo → auto-populates with current-year data on mount instead of empty tables.

## Integrations and Data Flow

### External systems
See Phase D §D.1. Five databases (reused: Mistra PG, Grappa MySQL, Alyante MSSQL; new: Vodka MySQL, WHMCS MySQL), one internal gateway (Mistra NG via `gw-int.cdlan.net`), one SaaS (carbone.io). All read-only from this app's perspective; only outbound write is the carbone render POST.

### End-to-end user journeys
See Phase D §D.2. Ten journeys, all request/response. No timers, no cron, no reconciliation. Six of eight views auto-load; two require explicit input.

### Data ownership boundaries
- Billing / AFC owns WHMCS + Grappa.
- Sales / CRM owns Vodka.
- Provisioning owns Mistra.
- ERP owns Alyante.
- AFC Tools owns the carbone template id + API token (backend config).

## API Contract Summary

Base path: `/api/afc-tools/`. Role-gated by `app_afctools_access`. All requests return JSON unless the body is a streamed PDF. See Phase D §D.3 for the full endpoint catalog (13 endpoints).

- **Read**: transactions (date range), invoice lines (latest 2000), missing articles, xconnect orders (EVASO), energia-colo pivot + detail (by year), orders list, order header, order rows, ddt cespiti.
- **Proxies**: ticket PDF (`/tickets/{id}/pdf?lang`), order PDF (`/orders/{id}/pdf`). Both stream `application/pdf`.
- **Orchestration**: `POST /whmcs/transactions/export` → returns `{renderId, renderUrl}` (client opens the URL in a new tab per decision 4a).

## Constraints and Non-Functional Requirements

- **Security**:
  - Keycloak role `app_afctools_access` on every endpoint.
  - All DB credentials, gateway OAuth2 creds, carbone API token and template id live in backend env (`VODKA_DSN`, `WHMCS_DSN`, existing `MISTRA_DSN` / `GRAPPA_DSN` / `ALYANTE_DSN`, `CARBONE_API_TOKEN`, `CARBONE_AFCTOOLS_TRANSAZIONI_TEMPLATE_ID`).
  - Never include raw DB connection strings or the carbone template id in the frontend bundle.
- **Performance**:
  - Report DDT per cespiti is an unbounded `SELECT *` (preserved by decision). Back-end request timeout should be set generously; TODO logged.
  - `All_orders_xcon` joins five Mistra tables client-free server-side; cache headers `Cache-Control: private, max-age=60` acceptable.
  - No server-side pagination is introduced (matches Appsmith behavior); tables paginate client-side.
- **Operational**:
  - Two new MySQL DSNs require provisioning steps in preprod/prod (Kubernetes Secret + ConfigMap entries).
  - Carbone template ownership: continues to be managed manually until the cross-app "Portal Admin Module — Carbone Template Management" TODO lands.
- **UX / accessibility**: follow `docs/UI-UX.md` and the portal design system. `<b>` bindings from Appsmith are replaced by semantic bold typography, not literal markup.

## Open Questions and Deferred Decisions

- **Q-E1** — *Quadrimestrale code divergence*: `cdlan_dur_rin` uses 4, `cdlan_int_fatturazione` uses 5. Owner: Sales / Fatturazione domain expert. Needed input: confirmation whether both codings are authoritative or one is a bug. Preserved 1:1; TODO in `docs/TODO.md → AFC Tools App`.
- **Q-E2** — *DDT cespiti pagination / filters*: current full-table load risks timeout at scale. Owner: AFC team. TODO in `docs/TODO.md → AFC Tools App`.
- **Q-E3** — *Energia Colo detail field semantics*: `tipo_variabile`, `coefficiente`, `fisso_cu`, `eccedenti`, `importo_eccedenti` are displayed verbatim but their business meaning is undocumented. Owner: AFC / Billing. Not blocking 1:1.
- **Q-E4** — *Carbone template admin*: hard-coded template id per app continues the pattern flagged in `docs/TODO.md → Listini e Sconti App → Portal Admin Module — Carbone Template Management`. Swap to the central admin module once it ships.
- **Q-E5** — *WHMCS floor `date > 20230120`*: embedded business rule, unclear if permanent. Owner: Billing. Preserved as-is.
- **Q-E6** — *Path ambiguity `/orders/{id}` (Vodka) vs `/orders/{id}/pdf` (Mistra)*: two different backing DBs under the same `/orders/` namespace. Technically distinct via the `/pdf` suffix; worth a rename in a future pass (e.g. `/xconnect-orders/{id}/pdf`). Deferred — not a 1:1 blocker.

## Acceptance Notes

- **What the audit proved directly**:
  - The 8-page inventory, 21-action + 5-JSObject catalog, orphan set.
  - The 10 embedded business rules (§4.1 of the audit).
  - The three duplication classes and three security/operational risks.
- **What the expert confirmed** (this spec):
  - Bug fixes vs preserves (5 decisions, §A.5).
  - Drop dead UI + orphan queries.
  - Carbone flow = 4a (proxy + open renderUrl in new tab).
  - Keycloak role = `app_afctools_access`.
  - Datasource plan = reuse MISTRA/GRAPPA/ALYANTE; add VODKA_DSN + WHMCS_DSN.
- **What still needs validation** (post-migration):
  - Q-E1..Q-E6 as documented above.
  - Behavioral parity: sampling ≥ 5 real orders through Dettaglio ordini to confirm every label mapping matches the Appsmith version (exception: `cod_termini_pag == 400`, which we deliberately changed).

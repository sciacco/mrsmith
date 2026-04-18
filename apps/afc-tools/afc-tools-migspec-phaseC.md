# AFC Tools — Migration Spec Phase C: Logic Placement

**Scope**: for every non-trivial piece of logic currently in Appsmith (SQL, JSObject methods, inline ternaries, query orchestration), classify it as **backend / frontend / shared** and capture the rationale. The 1:1 directive biases placement toward *preserve current semantics exactly*; nothing is refactored for its own sake.

Placement vocabulary:
- **B** — backend (Go service under `backend/internal/afctools/`)
- **F** — frontend (React code under `apps/afc-tools/src/`)
- **S** — shared (reused via `@mrsmith/api-client` generated types, or a constants module)

---

## C.1 Summary matrix

| # | Current location | Description | Placement | Rationale |
|---|---|---|---|---|
| L1 | SQL `getTransactions` (MySQL) | WHMCS transactions, date range, invoice/refund filter, floor `date > 20230120` | **B** | Business rule + DB secret; mini-apps cannot reach WHMCS from browser |
| L2 | SQL `righealiante` (MySQL) | Last 2000 WHMCS→Alyante invoice lines | **B** | Same |
| L3 | SQL `articoli_non_in_alyante` (PG) | Anti-join + `erp_sync=true` | **B** | Business rule ("missing in Alyante" is a join shape) |
| L4 | SQL `Q_select_consumi_colo_filter` / `Q_select_consumi_colo` (MySQL) | Year-pivoted consumption + detail; `IF(ampere>0, ampere, Kw)` | **B** | Metric rule + pivot shape |
| L5 | SQL `Select_Orders_Table` (MySQL) | Active/sent orders, label-mapping CASE expressions | **B** | DB access + label precomputation |
| L6 | SQL `Order` (MySQL) with `CASE cdlan_int_fatturazione` / `cdlan_int_fatturazione_att` | Header + pre-computed billing-frequency labels | **B** | Labels are already server-side; keep it that way |
| L7 | SQL `RigheOrdine` (MySQL) with `IF(cdlan_codice_kit…) CONCAT(...)` | Kit bundle code composition | **B** | Composition rule lives in the query; leave it |
| L8 | SQL `ListaDdtVerificaCespiti` (MSSQL) | `SELECT *` from view | **B** | DB access |
| L9 | REST `DownloadTicketPDF` / `DownloadOrderPDF` (gateway) | PDF fetch from `gw-int.cdlan.net` | **B** | Gateway auth; backend proxies and streams |
| L10 | REST `render_template` (carbone.io) | XLSX render | **B** | Move `templateId` server-side (decision A.5.4 = 4a) |
| L11 | JS `utils.runReport` (JSObject) | Orchestration: query → carbone → navigate | **F+B** | **B** runs the query and the carbone call and returns a `renderId`; **F** opens the resulting URL in a new tab |
| L12 | JS `utils.getURL` | `"https://render.carbone.io/render/" + renderId` | **F** | Trivial URL concat on the renderId returned by backend |
| L13 | JS `TicketTools.downloadTicketPDF` | Base64/raw heuristic + Blob + download | **F** (simplified) | In native `fetch`, the body is a `Blob` — the heuristic disappears. Same user outcome (PDF downloads) |
| L14 | JS `OrderTools.download` + 404 special case | Same + "Il PDF non è ancora pronto." on 404 | **F** | Toast triggered from backend 404; copy lives in FE i18n |
| L15 | UI ternary `cdlan_cod_termini_pag` (18 codes; FIX bug 400) | Code→label | **F** (lookup table constant) | Presentation only; fits in a `paymentTerms.ts` module next to the detail page |
| L16 | UI ternary `cdlan_tipo_ord` A/N/R | Code→label | **F** | Same |
| L17 | UI ternary `cdlan_dur_rin` 1/2/3/4/6/12 (Quadrimestrale=4) | Code→label | **F** | Diverges from `cdlan_int_fatturazione` (=5) — preserved (decision A.5.1b) |
| L18 | UI ternary `cdlan_tacito_rin` 1→Sì | Bool→label | **F** | Trivial |
| L19 | UI ternary `cdlan_tipodoc == 'TSC-ORDINE-RIC'` | Code→label | **F** | Trivial |
| L20 | UI ternary `cdlan_note == '' ? …` / `data_decorrenza == '' ? …` | Null-or-empty placeholder (FIX per A.5.1c) | **F** | Treat `null`, `undefined`, `''` equivalently |
| L21 | Appsmith `date BETWEEN {{i_dal.selectedDate}} AND {{i_al.selectedDate}}` (string→int) | Date interpolation fragility | **B** (parameterized) | Use typed `time.Time` params; compute `YYYYMMDD` integer server-side for `v_transazioni.date` |
| L22 | `TXT_anno` default = current year (decision A.5.1d) | Default input value | **F** | `new Date().getFullYear()` at component init |
| L23 | Multi-query parallel fan-out on Cerca (Energia Colo) | `Q_select_consumi_colo_filter.run(); Q_select_consumi_colo.run()` | **F** | Two `fetch` calls from a single click handler (via `@tanstack/react-query` — see §C.3) |
| L24 | Row→detail navigation on Ordini Sales | `navigateTo('Dettaglio ordini', {id})` | **F** | Just `navigate(\`/ordini-sales/${id}\`)` |
| L25 | Input validation: ticketId + language required (Tab 1 RH) | Show alert on empty | **F** | Form validation, toast |
| L26 | `Select_Orders_Table` on-load | Fetch at mount | **F** | React Query `useQuery` |
| L27 | Static select options `[it, en]` (RH Tab 1) | Enum | **S** | Export from a small `langs.ts` constants module; backend accepts `it|en` |

---

## C.2 Backend responsibilities (`backend/internal/afctools/`)

One Go package per the portal convention. Modules:

- `repo/` — read-only repositories (raw SQL, no ORM), one file per datasource:
  - `whmcs.go` — injected `*sql.DB` for `WHMCS_DSN` (new, MySQL). Methods: `ListTransactions(ctx, from, to)`, `ListInvoiceLines(ctx)`.
  - `vodka.go` — injected `*sql.DB` for `VODKA_DSN` (new, MySQL). Methods: `ListOrders(ctx)`, `GetOrder(ctx, id)`, `ListOrderRows(ctx, orderId)`.
  - `grappa.go` — reuses the existing `GRAPPA_DSN` handle. Methods: `ListEnergiaColoPivot(ctx, year)`, `ListEnergiaColoDetail(ctx, year)`.
  - `mistra.go` — reuses the existing `MISTRA_DSN` handle. Methods: `ListMissingArticles(ctx)`, `ListXConnectOrders(ctx)`.
  - `alyante.go` — reuses the existing `ALYANTE_DSN` handle. Method: `ListDdtCespiti(ctx)`.
- `handler.go` — HTTP handlers for the endpoints listed in §C.3.
- `carbone.go` — wraps the shared carbone service (`internal/platform/carbone` once the cross-app extraction in `docs/TODO.md` lands; until then, a local copy of the `reports` / `listini` pattern).
- `gateway.go` — proxies `gw-int.cdlan.net` calls for PDF download (L9) with the existing OAuth2 token flow used elsewhere in the repo.
- `paymenttermscodes.go` / `orderenums.go` — *(optional)* if we want the payment terms / order-type mappings available server-side for future exports; not needed for the 1:1 UI.

Business rules preserved verbatim in backend SQL:
- `date > 20230120` floor (L1).
- Invoice/refund filter (L1).
- `erp_sync=true` + anti-join (L3).
- `IF(ampere>0, ampere, Kw)` (L4).
- `cdlan_stato IN ('ATTIVO','INVIATO')` (L5).
- `IF(is_colo!=0, is_colo, service_type)` (L5).
- `CASE cdlan_tipo_ord` A/N/R → label (L5).
- `IF(from_cp!=0, 'Sì', 'No')` (L5).
- `CASE cdlan_int_fatturazione` 1/2/3/**5**/6/else (L6).
- `CASE cdlan_int_fatturazione_att` 1→"All'ordine" else (L6).
- `IF(cdlan_codice_kit != '', CONCAT(..., '-', index_kit), '')` (L7).
- `kit_category='XCONNECT' AND state='EVASO'` (L9 companion — `All_orders_xcon`).
- `ticket_type=RemoteHands` pinned when proxying `DownloadTicketPDF` (L9).

Authz: every endpoint requires Keycloak role `app_afctools_access`.

## C.3 Frontend responsibilities (`apps/afc-tools/src/`)

- **Routing** — React Router `routes.tsx` per §B.1.
- **Data fetching** — `@tanstack/react-query` for every endpoint. Cache keys scoped per view.
- **Tables** — `@mrsmith/ui` Table with client-side pagination (matches Appsmith behavior).
- **Forms** — native form + `useState` for filter inputs; no react-hook-form needed (inputs are trivial: 2 date pickers, 1 year input, 1 text input, 1 select).
- **Drill-down** — `useNavigate` for Ordini Sales → Dettaglio ordini.
- **PDF download** — native `fetch(url).then(r => r.blob())` → `URL.createObjectURL` → `<a download>` click. No base64 heuristic.
- **Export XLSX (Transazioni)** — backend returns `{renderId}`; frontend opens `window.open(\`https://render.carbone.io/render/${renderId}\`, '_blank')`.
- **Enum/label mappings** — a tiny `src/pages/ordini-sales/detail/labels.ts` file with the `paymentTerms`, `durRin`, `tipoOrd`, `tipoDoc`, `tacitoRin` maps (L15–L19). Import in the detail page. No backend round-trip needed.
- **Null-equivalent helper** — one small util `isEmpty(v) => v == null || v === ''` used in the detail page (L20).

Shared helpers (via `@mrsmith/api-client` generated types):
- Request / response DTOs per endpoint, including `WhmcsTransaction`, `InvoiceLine`, `MissingArticle`, `XConnectOrder`, `SalesOrderSummary`, `OrderHeader`, `OrderRow`, `EnergiaColoPivotRow`, `EnergiaColoDetailRow`, `DdtCespitoRow`.
- Language enum `it | en`.

## C.4 Datasource decisions

The repo already owns a shared `backend/internal/platform/database` helper wired in `backend/cmd/server/main.go`, with connections for Anisetta (PG), Mistra (PG), Grappa (MySQL), Alyante (MSSQL), Coperture (PG). Direct-DB access from mini-apps is an established pattern (compliance, quotes, listini, reports, energia-dc, kitproducts, panoramica, richieste-fattibilita all do it).

| Datasource | DSN env var | Driver | Reuse vs new | Used by AFC Tools for |
|---|---|---|---|---|
| **Mistra** | `MISTRA_DSN` | postgres (pgx) | **Reuse** — already connected | L3 `articoli_non_in_alyante`, L9 companion `All_orders_xcon` |
| **Grappa** | `GRAPPA_DSN` | mysql | **Reuse** — already connected | L4 Consumi Energia Colo (both pivot + detail) |
| **Alyante** | `ALYANTE_DSN` | mssql | **Reuse** — already connected | L8 `ListaDdtVerificaCespiti` |
| **Vodka / daiquiri** | `VODKA_DSN` *(new)* | mysql | **New adapter** — not yet in main.go | L5 Ordini Sales, L6 Order, L7 RigheOrdine |
| **WHMCS (Prometeus)** | `WHMCS_DSN` *(new)* | mysql | **New adapter** — not yet in main.go | L1 `getTransactions`, L2 `righealiante` |

For the two new adapters the wiring follows the same pattern as the existing MySQL connections: an env-var DSN in `backend/internal/platform/config/config.go`, a `database.New("mysql", cfg.VodkaDSN)` / `WhmcsDSN` call in `backend/cmd/server/main.go`, and the `*sql.DB` handle passed into the `afctools` package constructor. DSN examples are added to `backend/.env.example` and `deploy/k8s/configmap.yaml` per repo convention.

No API-gateway routing for Mistra — Appsmith reads direct PG today, the port does the same. The AGENTS.md note about Mistra NG is about greenfield flows, not 1:1 ports.

---

## C.5 Phase C done-check

- [x] Every JS method / SQL CASE / UI ternary classified B/F/S.
- [x] Preserved rules enumerated, tagged against the matrix.
- [x] Frontend responsibility list aligned to portal mini-app conventions (React Router, React Query, `@mrsmith/ui`).
- [x] Datasource plan finalized: reuse MISTRA/GRAPPA/ALYANTE, add new VODKA_DSN + WHMCS_DSN adapters.

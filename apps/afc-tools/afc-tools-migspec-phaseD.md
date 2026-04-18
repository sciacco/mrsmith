# AFC Tools — Migration Spec Phase D: Integration and Data Flow

**Scope**: enumerate external systems, trace end-to-end user journeys, and call out hidden or triggered behaviors. Datasource decisions from §C.4 are assumed.

---

## D.1 External systems

| System | Kind | Access pattern | Auth | Used for |
|---|---|---|---|---|
| WHMCS (Prometeus) | MySQL (`WHMCS_DSN`, new) | Direct read-only SQL from Go backend | DSN secret | Transazioni WHMCS, Fatture Prometeus |
| Vodka / daiquiri | MySQL (`VODKA_DSN`, new) | Direct read-only SQL | DSN secret | Ordini Sales, Dettaglio ordini |
| Grappa | MySQL (`GRAPPA_DSN`, reuse) | Direct read-only SQL | DSN secret | Consumi Energia Colo |
| Mistra | PostgreSQL (`MISTRA_DSN`, reuse) | Direct read-only SQL | DSN secret | Nuovi articoli, XConnect orders list |
| Alyante ERP | MSSQL (`ALYANTE_DSN`, reuse) | Direct read-only SQL | DSN secret | Report DDT cespiti |
| Mistra NG Internal API | REST (`gw-int.cdlan.net`) | HTTPS GET, proxied by backend | OAuth2 (shared client credentials pattern used by other apps hitting the gateway) | PDF download for Remote Hands tickets and XConnect orders |
| carbone.io | REST (`api.carbone.io` + `render.carbone.io`) | HTTPS POST `/render/{templateId}` from backend; `GET /render/{renderId}` opened by browser in a new tab | API token + template id held server-side | XLSX export of Transazioni WHMCS |

All DB and gateway secrets live in backend env vars; the frontend bundle sees none of them. Per decision A.5.4 (4a), the carbone `templateId` and API token move to backend config — they are never exposed to the browser.

---

## D.2 End-to-end user journeys

### D.2.1 Transazioni WHMCS — Cerca
1. User opens `/transazioni-whmcs`. Table is empty (no auto-load — preserved).
2. User adjusts dates (default: 15 days back → today).
3. Click **Cerca** → `GET /api/afc-tools/whmcs/transactions?from=YYYY-MM-DD&to=YYYY-MM-DD`.
4. Backend: `afctools.Handler.ListTransactions` → `repo.whmcs.ListTransactions(from, to)` → runs the SQL in §B.4.1 (parameterized with typed dates; `YYYYMMDD` int compare built server-side).
5. Backend responds `[]WhmcsTransaction`. Frontend renders in `tbl_transactions`.

### D.2.2 Transazioni WHMCS — Esporta
1. User clicks **Esporta** (requires a date range; Cerca not required first — preserved).
2. Frontend calls `POST /api/afc-tools/whmcs/transactions/export` with `{from, to}`.
3. Backend:
   - runs the same `ListTransactions(from, to)` query.
   - builds `reportName = "transazioni_whmcs_dal_{from}_al_{to}"`.
   - POSTs `{convertTo: "xlsx", reportName, data: {righe: [...]}}` to `carbone.io/render/{templateId}`, using the server-side `CARBONE_API_TOKEN` and `CARBONE_AFCTOOLS_TRANSAZIONI_TEMPLATE_ID`.
   - receives `{renderId}` from carbone, returns `{renderId, renderUrl}` to the frontend.
4. Frontend opens `renderUrl` (`https://render.carbone.io/render/{renderId}`) via `window.open(url, '_blank')`.

### D.2.3 Fatture Prometeus
1. Tab mount → `GET /api/afc-tools/whmcs/invoice-lines` (no params).
2. Backend runs SQL §B.4.2 (cap `LIMIT 2000`, `ORDER BY id DESC`).
3. Response rendered verbatim.

### D.2.4 Nuovi articoli
1. Tab mount → `GET /api/afc-tools/mistra/missing-articles`.
2. Backend runs SQL §B.4.3 against Mistra PG.
3. Response rendered.

### D.2.5 Report XConnect e Remote Hands — Tab 1 (ticket download)
1. User types `numeroTicket`, picks `lang ∈ {it, en}`.
2. Click **Scarica PDF**:
   - If inputs missing → toast warning (preserved copy from `TicketTools.downloadTicketPDF`).
   - Else → `GET /api/afc-tools/tickets/{ticketId}/pdf?lang={it|en}`.
3. Backend `gateway.DownloadTicketPDF`: proxies to `GET https://gw-int.cdlan.net/tickets/v1/pdf/{ticketId}?ticket_type=RemoteHands&lang={lang}` with the service-account OAuth2 token, streams the `application/pdf` body back.
4. Frontend: `fetch(...)` → `Blob` → `URL.createObjectURL` → `<a download="ticket_{ticketId}.pdf">` click.

### D.2.6 Report XConnect e Remote Hands — Tab 2 (XConnect orders list + per-row PDF)
1. Tab mount → `GET /api/afc-tools/mistra/xconnect/orders`.
2. Backend runs SQL §B.4.6 against Mistra PG.
3. Per-row **Scarica PDF**:
   - `GET /api/afc-tools/orders/{orderId}/pdf` → backend proxies `GET https://gw-int.cdlan.net/orders/v1/order/pdf/{orderId}`.
   - On upstream 404 → backend returns `404` with body `{message: "Il PDF non è ancora pronto."}`; frontend shows toast (copy preserved).
   - Otherwise → same Blob → download pattern as D.2.5.

### D.2.7 Consumi variabili Energia Colo
1. Tab mount:
   - `TXT_anno` defaults to current year (`new Date().getFullYear()` — deviation A.5.1d).
   - Two parallel requests via React Query `useQueries`:
     - `GET /api/afc-tools/energia-colo/pivot?year={YYYY}`
     - `GET /api/afc-tools/energia-colo/detail?year={YYYY}`
2. User edits the year and clicks **Cerca** → both queries re-run (invalidate + refetch, parallel).
3. Backend runs SQL §B.4.7 / §B.4.8 on grappa MySQL; `year` parameter validated as 4-digit int server-side.

### D.2.8 Ordini Sales
1. Tab mount → `GET /api/afc-tools/orders`.
2. Backend runs SQL §B.4.9 on Vodka MySQL.
3. User clicks row icon → `navigate('/ordini-sales/{id}')`.

### D.2.9 Dettaglio ordini
1. Route mount (`/ordini-sales/:id`) → two parallel requests:
   - `GET /api/afc-tools/orders/{id}` → SQL §B.4.10 (`Order`, 1 row + pre-computed labels).
   - `GET /api/afc-tools/orders/{id}/rows` → SQL §B.4.11 (`RigheOrdine`, N rows).
2. UI renders header (ternaries L15–L20 applied client-side) + rows table.
3. **Torna indietro** → `navigate('/ordini-sales')`.

### D.2.10 Report DDT cespiti
1. Tab mount → `GET /api/afc-tools/ddt-cespiti`.
2. Backend runs SQL §B.4.12 on Alyante MSSQL (`SELECT *`, no filter, no limit — preserved per A.5.1e).
3. Response rendered. Long response times are acceptable per the 1:1 directive; TODO logged.

---

## D.3 Backend endpoint catalog

All endpoints require Keycloak role `app_afctools_access`. All are read-only except the carbone export, which is write-to-external (no mrsmith DB mutation).

| Method | Path | Handler | Backing SQL / REST |
|---|---|---|---|
| GET | `/api/afc-tools/whmcs/transactions?from&to` | `ListTransactions` | §B.4.1 |
| POST | `/api/afc-tools/whmcs/transactions/export` | `ExportTransactions` | §B.4.1 + carbone |
| GET | `/api/afc-tools/whmcs/invoice-lines` | `ListInvoiceLines` | §B.4.2 |
| GET | `/api/afc-tools/mistra/missing-articles` | `ListMissingArticles` | §B.4.3 |
| GET | `/api/afc-tools/tickets/{ticketId}/pdf?lang` | `DownloadTicketPDF` (proxy) | §B.4.4 |
| GET | `/api/afc-tools/orders/{orderId}/pdf` | `DownloadOrderPDF` (proxy, Mistra gateway) | §B.4.5 |
| GET | `/api/afc-tools/mistra/xconnect/orders` | `ListXConnectOrders` | §B.4.6 |
| GET | `/api/afc-tools/energia-colo/pivot?year` | `EnergiaColoPivot` | §B.4.7 |
| GET | `/api/afc-tools/energia-colo/detail?year` | `EnergiaColoDetail` | §B.4.8 |
| GET | `/api/afc-tools/orders` | `ListOrders` | §B.4.9 |
| GET | `/api/afc-tools/orders/{id}` | `GetOrder` | §B.4.10 |
| GET | `/api/afc-tools/orders/{id}/rows` | `ListOrderRows` | §B.4.11 |
| GET | `/api/afc-tools/ddt-cespiti` | `ListDdtCespiti` | §B.4.12 |

Naming convention mirrors `reports`, `energia-dc`, `listini-e-sconti`: grouped by area, `kebab-case`, nouns-over-verbs.

Note on path collision: `/api/afc-tools/orders/{orderId}/pdf` (proxy to Mistra gateway) and `/api/afc-tools/orders/{id}` (Vodka detail) coexist — the router distinguishes by the trailing segment `/pdf`. The two resources are in *different DBs* (Mistra `orders.order` for XConnect vs Vodka `orders` for Sales), which is historically confusing but accepted for 1:1. Flagged in Phase E open questions.

---

## D.4 Data ownership boundaries

- **WHMCS + Grappa**: owned by Billing/AFC. Read-only from this app.
- **Vodka / daiquiri**: owned by Sales/CRM. Read-only.
- **Mistra**: owned by Provisioning. Read-only.
- **Alyante**: owned by ERP. Read-only via the `Tsmi_*` reporting views.
- **Carbone.io template**: owned by AFC Tools app-team; template id + API token live in backend env.

No writes to any of these systems from AFC Tools. The only outbound write is the carbone render POST, which produces an ephemeral render artifact owned by carbone.

---

## D.5 Hidden / triggered behaviors

Nothing in the Appsmith export triggers on a timer, webhook, or cron. All flows are user-initiated (click Cerca, click Esporta, click download, mount a tab). No reconciliation jobs, no cache warm-ups. This is strictly a request/response read app.

One quasi-automatic behavior: three pages auto-load on tab mount (Fatture Prometeus, Nuovi articoli, Ordini Sales, Report DDT cespiti, Report XConnect Tab 2, Consumi Energia Colo — 6 of 8). The remaining two (Transazioni WHMCS, Report XConnect Tab 1) require explicit user input. The port preserves all six auto-loads.

---

## D.6 Cross-view journeys

Only one: Ordini Sales row click → Dettaglio ordini (path param `:id`), back button returns to `/ordini-sales`. No breadcrumbs needed — single step.

No shared state between pages (each query's cache is local to its React Query key; invalidation is scoped).

---

## D.7 Phase D done-check

- [x] All 7 external systems inventoried with auth + usage.
- [x] 10 user journeys traced end-to-end (6 of 8 views + 2 orchestrated actions: Esporta, drill-down).
- [x] Endpoint catalog finalized (13 endpoints).
- [x] Data-ownership table explicit.
- [x] Hidden/triggered behaviors explicitly "none".
- [x] Cross-view journey documented (only 1).

Ready to assemble the final specification in Phase E.

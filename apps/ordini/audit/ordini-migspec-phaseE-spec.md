# Application Specification — Ordini

## Summary
- **Application name:** Ordini (mrsmith mini-app)
- **Audit source:** `apps/ordini/audit/{app-inventory.md, page-audit.md, datasource-catalog.md, findings-summary.md}`; cross-referenced to `docs/mistra-dist.yaml` and `docs/IMPLEMENTATION-KNOWLEDGE.md`.
- **Spec status:** approved (Phases A–D resolved); ready for downstream implementation planning via `portal-miniapp-generator`.
- **Last updated decisions:** 2026-04-25.

## Current-State Evidence
- **Source pages/views (in scope):** `Home`, `Dettaglio ordine`. Out-of-scope dropped views: `Ordini semplificati`, `Draft gp da offerta`, `Form ordine`, the legacy `Dettaglio_ordine` modal on Home, and the `Arxivar link` tab (collapsed into Info per B3).
- **Source entities:** `Order` (vodka.`orders`), `OrderRow` (vodka.`orders_rows`), `Customer` (Alyante.`Tsmi_Anagrafiche_clienti`, reference only), `ArxivarDocument` (external, referenced via `arx_doc_number`).
- **Source integrations:** vodka MySQL (primary), Alyante MSSQL (read-only customer master), GW internal CDLAN REST (ERP + PDF + Arxivar bridge), Keycloak (auth). `db-mistra` PostgreSQL is dropped from scope.
- **Known audit gaps or ambiguities:** the `arx_doc_number` write path is invisible to the Appsmith export but lives outside this app — preserved as-is under 1:1; cancel-order is deferred post-v1 because the source binding is out of date versus the canonical Mistra NG contract.

---

## Entity Catalog

### Entity: Order
- **Purpose.** Lifecycle record for a sales order (proposta/ordine). Created elsewhere (Customer Portal or ERP). Managed through this app from BOZZA → INVIATO → ATTIVO. Side branches PERSO/ANNULLATO are not driven from this app in v1.
- **Operations.**
  - `listOrders()` — paginated/searchable list view.
  - `getOrder(id)` — single-record read with all header fields.
  - `updateBozzaHeader(id, {customer_po, confirmation_date, customer_id})` — writes `cdlan_rif_ordcli`, `cdlan_dataconferma`, `cdlan_cliente` (RAGIONE_SOCIALE), and **also `cdlan_cliente_id`** (= NUMERO_AZIENDA) per C2. Auth: `app_customer_relations` + state `BOZZA`.
  - `updateReferents(id, payload)` — 9 contact fields. Auth: `app_customer_relations` + state ∈ {BOZZA, INVIATO}.
  - `sendToErp(id, signedPdf)` — per-row push to ERP, vodka state flip and Arxivar upload only on full success (per Q8 / C1). Auth: `app_customer_relations` + state `BOZZA` + preconditions (dataconferma set, customer set, PDF attached). Returns per-row outcome report.
  - `getKickoffPdf(id)`, `getActivationFormPdf(id)`, `getOrderPdf(id)`, `getSignedPdf(id)` — backend-proxied GW PDF endpoints, each with its own auth and state gates (see API Contract).
  - **Excluded in v1 (deferred via TODO):** `requestCancellation` (RICHIEDI ANNULLAMENTO), `markAsLost` (ORDINE PERSO), order creation.
- **Fields and inferred types.** See Phase A §1 for the full table. Salient ones:
  - `id: int` (vodka PK).
  - `cdlan_systemodv: string` (ERP-side identifier).
  - `cdlan_ndoc: int`, `cdlan_anno: int` (composite proposta number).
  - `cdlan_stato: enum {BOZZA, INVIATO, ATTIVO, PERSO, ANNULLATO}`.
  - `cdlan_evaso: 0|1`.
  - `cdlan_cliente: string` (RAGIONE_SOCIALE) and `cdlan_cliente_id: int|null` (NUMERO_AZIENDA — writeable per C2).
  - `cdlan_tipo_ord: enum {A, N, R}` (Sostituzione/Nuovo/Rinnovo).
  - `cdlan_tipodoc: enum {TSC-ORDINE-RIC, TSC-ORDINE}`.
  - `cdlan_int_fatturazione: enum {1,2,3,4,6,12}` mapped to Mensile/…/Annuale; **`4 = Quadrimestrale` is canonical, but the read path also accepts legacy `5`** without migration (Q3).
  - `cdlan_int_fatturazione_att: enum {1, 2}` (All'ordine / All'attivazione).
  - `cdlan_dur_rin: enum {1,2,3,4,6,12}`, `cdlan_tacito_rin: 0|1`.
  - Profile fields (`profile_*`) including `profile_lang: enum {it, en}` used to localize the activation-form PDF filename server-side.
  - `service_type: comma-separated string` (Connettività/Cloud/Security/Voce/Supporto).
  - `is_colo: string` (`0` = Altre soluzioni / `Colocation variabile` / `Iaas payperuse` / `Iaas payperuse indiretto`).
  - `arx_doc_number: string|null` (Arxivar document UUID; written externally to this app).
  - `from_cp: 0|1` (CP-originated flag — relevant only to the deferred cancel flow).
- **Relationships.**
  - `Order` 1:N `OrderRow` via `orders_rows.orders_id = orders.id`.
  - `Order.cdlan_cliente_id` → `Customer.NUMERO_AZIENDA` (newly persisted per C2; `Order.cdlan_cliente` continues to hold the RAGIONE_SOCIALE string for backward compatibility).
  - `Order.arx_doc_number` → external Arxivar document.
- **Constraints and business rules.**
  - State machine: `BOZZA → INVIATO` (sendToErp full success); `INVIATO → ATTIVO` (every row confirmed); `INVIATO → ANNULLATO` is server-side via Mistra NG (not driven from v1 of this app).
  - `confirm_data_attivazione = 1` is set unconditionally as a side-effect of every per-row activation save.
  - Auto-ATTIVO promotion uses the `CheckConfirmRows` count with the **Q2 fix** (`data_annullamento IS NOT NULL` instead of the always-false `<> null`).
  - `cdlan_stato = "CREATO"` is sent to ERP regardless of vodka state — intentional vocabulary divergence; do not "fix".
- **Open questions.** None remaining at v1 scope.

### Entity: OrderRow
- **Purpose.** One article line per order. Holds per-row activation state, serial number, technical notes.
- **Operations.**
  - `listRows(orderId)` — read full row dataset (RigheOrdine equivalent).
  - `listTechnicalRows(orderId)` — same rows projected for the Informazioni dai tecnici tab (different column subset; `note_tecnici` UTF8-converted on read).
  - `setActivationDate(rowId, {activation_date})` — per-row vodka UPDATE + GW sync + auto-ATTIVO check (Q2 fix). Auth: `app_customer_relations` + order state `INVIATO`.
  - `updateSerialNumber(rowId, {serial_number})` — Auth: `app_ordini_access` + order state `BOZZA`.
  - `updateTechnicalNotes(rowId, {technical_notes})` — Auth: `app_ordini_access`, any state.
- **Fields and inferred types.** See Phase A §2. Notable:
  - `id: int` (vodka PK), `orders_id: int` (FK), `cdlan_systemodv_row: int` (stable ERP-correlated business key used by the GW endpoints).
  - `cdlan_codice_kit: string`, `index_kit: int` (bundle composition).
  - `cdlan_codart`, `cdlan_descart`, `cdlan_qta`, `cdlan_prezzo`, `cdlan_prezzo_attivazione`, `cdlan_prezzo_cessazione`, `cdlan_ragg_fatturazione`.
  - `cdlan_data_attivazione: date|null`, `confirm_data_attivazione: 0|1`, `data_annullamento: date|null`.
  - `cdlan_serialnumber: string|null`, `note_tecnici: text` (legacy collation; UTF8-converted on read).
- **Relationships.** Belongs to `Order`.
- **Constraints and business rules.**
  - Inline `Numero seriale` editing only when the **order** is in BOZZA.
  - "Modifica" iconButton (activation date modal) only when the order is in INVIATO and the user is `app_customer_relations`.
  - The DTO surfaces `activation_price` (canonical name per Q4) where the source SQL aliased to `'Attivazione'` or `'Prezzo attivazione'`.
- **Open questions.** None.

### Entity: Customer (reference only — Alyante)
- **Purpose.** Source for the Ragione sociale dropdown on the Info tab.
- **Operations.** `listCustomers()` only.
- **Fields used.** `NUMERO_AZIENDA` (int), `RAGIONE_SOCIALE` (string).
- **Filter.** `DATA_DISMISSIONE IS NULL AND RAGGRUPPAMENTO_3 <> 'Ecommerce' AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`, grouped by both returned columns.
- **Relationships.** Loose reference from `Order.cdlan_cliente_id`.

### Entity: ArxivarDocument (external pointer)
Not a local entity. Referenced only by `orders.arx_doc_number` and the deep-link URL. No CRUD from this app.

---

## View Specifications

### View: Home (Lista ordini)
- **User intent.** Find and open a specific order, typically by proposta number or customer name.
- **Interaction pattern.** List/index. Single table, one row action ("Visualizza"), client-side search/sort/pagination.
- **Main data shown.** 15 columns from `Select_Orders_Table`-equivalent, with display mappings (Tipo proposta A/N/R → label, Tipo documento → "Ordine ricorrente"/"Ordine Spot", Dal CP? → Sì/No, Evaso → Sì/No per B2) handled by the **frontend formatter** rather than SQL CASE.
- **Key actions.** Row "Visualizza" → navigates to `/ordini/:id`.
- **Entry/exit.** Enter from portal sidebar `/ordini` or direct URL; exit on row click; SendToErp success on Dettaglio brings the user back here.
- **Notes.** No order-creation entry point in this app. Server-side pagination is **not** wired (1:1 preserves client-side behaviour); orphan paginated queries from the source are dropped.

### View: Dettaglio ordine
- **User intent.** Drive one order through its lifecycle, edit header/referents/rows, generate PDFs.
- **Interaction pattern.** Detail view; tabbed workspace (Info / Azienda / Referenti / Righe / Informazioni dai tecnici); persistent header bar with PDF actions; modal for per-row activation date.
- **Main data shown or edited.** Per tab:
  - **Info** — readonly header metadata + editable block (`cdlan_rif_ordcli`, `cdlan_dataconferma`, Ragione sociale via `erp_an_cli`); secondary line "ID cliente: <cdlan_cliente_id>" under Ragione sociale (B1); Arxivar deep-link anchor when `arx_doc_number IS NOT NULL` (B3); action bar with INVIA in ERP, Arxivar file picker. **Cancel button removed in v1.**
  - **Azienda** — readonly company/profile fields. The hidden `profile_lang` widget disappears (filename localization moves server-side).
  - **Referenti** — 9-field editor (technical / alt-technical / administrative).
  - **Righe** — order rows table; inline edit of `Numero seriale` (BOZZA + `app_ordini_access`); per-row "Modifica" iconButton (INVIATO + `app_customer_relations`) opens the activation-date modal.
  - **Informazioni dai tecnici** — same rows projected to show `note_tecnici` (UTF8-converted) and `data_annullamento`; inline edit of `note_tecnici` allowed in any state for any `app_ordini_access` user.
- **Key actions.** SALVA (Info, Referenti); INVIA in ERP; per-row CONFERMA (activation modal); inline saves (serial number, technical notes); four PDF buttons.
- **Entry/exit.** Enter via Home row action or direct URL `/ordini/:id`; exit via "Torna indietro" or after a successful sendToErp.
- **Notes on current vs intended.** All display mappings move from SQL CASE / inline ternary into a frontend formatter module; all role/state gates are advisory in the UI and re-enforced at every backend handler. The Arxivar tab is collapsed into the Info tab (B3). The `Modal1` Arxivar deep-link wrapper disappears.

---

## Logic Allocation
- **Backend responsibilities.**
  - All vodka mutations through parameterized queries (no raw-string interpolation).
  - All GW REST calls (ERP push, set-order-activation, send-to-arxivar, all four PDF endpoints).
  - All authorization checks (`app_ordini_access` at the router level; `app_customer_relations` per-handler).
  - Order-state transitions: `BOZZA → INVIATO` (terminal step of `sendToErp` on full success), `INVIATO → ATTIVO` (inside the per-row activation handler when the COUNT matches, with the Q2 fix).
  - Per-row send-to-ERP loop with structured per-row outcome response (per C1).
  - Base64-or-raw PDF normalization: clients always receive `application/pdf`.
  - Activation-form filename localization derived from `orders.profile_lang`.
  - **C2 dual-write:** BOZZA header save sets both `cdlan_cliente` (string) and `cdlan_cliente_id` (NUMERO_AZIENDA).
- **Frontend responsibilities.**
  - All display mappings (Tipo doc, Tipo proposta, Dal CP, Evaso, fatturazione/dur_rin labels, date formatting, service-type chips) — implemented in a single `formatters.ts` module local to the app.
  - Advisory role/state gates for widget visibility and disabled state — `canEditBozzaHeader`, `canSendToErp`, `canEditReferents`, `canOpenActivationModal`, `canEditSerialNumber`, `canEditTechnicalNotes`, `canShowArxivarFilePicker` (Q6 fix applied here).
  - Per-row outcome rendering for partial-failure responses from `sendToErp`.
  - Tab navigation, modal lifecycle, file-picker UX.
- **Shared validation or formatting.** None at the package level. Single-app formatter only — premature abstraction avoided.
- **Rules being revised rather than ported.** See Phase C §9 — the canonical revision table includes Q2/Q3/Q4/Q5/Q6/Q8 fixes plus removal of the cancel and ORDINE PERSO paths and the SQL/security hardening.

---

## Integrations and Data Flow
- **External systems and purpose.**
  - **vodka (MySQL)** — primary store; owns `orders`, `orders_rows`. Backend-only access, parameterized.
  - **Alyante (MSSQL)** — read-only customer master. Backend-only.
  - **GW internal CDLAN (REST, `https://gw-int.cdlan.net`)** — bridge to ERP, PDF generation, Arxivar upload/retrieval. Seven endpoints in active use (see Phase D §1.3); credentials/auth move server-side.
  - **Keycloak** — OAuth2/OIDC; `app_ordini_access` and `app_customer_relations` claims drive authorization.
  - **Mistra NG Internal API** — used **only for the deferred cancel flow** (not in v1).
  - **Arxivar (web UI)** — anchor target only; no API integration.
- **End-to-end user journeys.** See Phase D §3 for the seven flows: open order, BOZZA→INVIATO, edit referents, edit serial number, per-row activation→auto-ATTIVO, edit technical notes, PDF downloads.
- **Background or triggered processes.** None scheduled. All mutations are synchronous side-effects of user actions; auto-ATTIVO and INVIATO transitions live inside their respective handlers (Phase D §4).
- **Data ownership boundaries.** vodka owns the order lifecycle state and metadata; Alyante owns customer master; ERP owns its own document and state vocabulary; Arxivar owns the signed-PDF storage; Keycloak owns identity. The `arx_doc_number` write path lives outside this app — preserved as-is under 1:1.

---

## API Contract Summary
All endpoints are namespaced under `/api/ordini`. Every handler validates state + role server-side; the frontend gates are advisory.

### Read endpoints
| Method | Path | Purpose | Auth |
|---|---|---|---|
| GET | `/api/ordini` | Home list (paginated/searchable; client-side in v1). | `app_ordini_access` |
| GET | `/api/ordini/:id` | Full order header. | `app_ordini_access` |
| GET | `/api/ordini/:id/rows` | Order rows for the Righe tab. | `app_ordini_access` |
| GET | `/api/ordini/:id/technical-rows` | Order rows projected for the Informazioni dai tecnici tab (UTF8 conversion preserved). | `app_ordini_access` |
| GET | `/api/ordini/ref/customers` | Alyante customer list (filtered as in §Customer entity). | `app_ordini_access`; backend may load lazily only when state == BOZZA. |
| GET | `/api/ordini/:id/kickoff.pdf` | Backend proxy to `GW /orders/v1/kick-off/:id`; returns `application/pdf`. Filename `kick off_<ndoc>_<anno>.pdf`. | `app_customer_relations` + state `INVIATO` |
| GET | `/api/ordini/:id/activation-form.pdf` | Backend proxy to `GW /orders/v1/activation-form/:id`; filename localized per `orders.profile_lang`. | `app_customer_relations` + state ∈ {INVIATO, ATTIVO} |
| GET | `/api/ordini/:id/pdf` | Backend proxy to `GW /orders/v1/order/pdf/:id/generate`. | `app_ordini_access` + `arx_doc_number IS NULL` |
| GET | `/api/ordini/:id/signed-pdf` | Backend proxy to `GW /orders/v1/order/pdf/:id?from=vodka`. | `app_ordini_access` + `arx_doc_number IS NOT NULL` |

### Write endpoints
| Method | Path | Purpose | Auth + state gate |
|---|---|---|---|
| PATCH | `/api/ordini/:id` | BOZZA header update; writes both `cdlan_cliente` and `cdlan_cliente_id` (C2). | `app_customer_relations` + state `BOZZA` |
| PATCH | `/api/ordini/:id/referents` | 9 contact fields. | `app_customer_relations` + state ∈ {BOZZA, INVIATO} |
| POST | `/api/ordini/:id/send-to-erp` | Multipart with the signed Arxivar PDF; per-row GW push, terminal vodka state flip + Arxivar upload only on full success; structured per-row response (C1). | `app_customer_relations` + state `BOZZA` + preconditions (dataconferma, customer, PDF) |
| PATCH | `/api/ordini/:id/rows/:rowId/serial-number` | Inline serial-number write. | `app_ordini_access` + state `BOZZA` |
| PATCH | `/api/ordini/:id/rows/:rowId/technical-notes` | Inline technical-note write. | `app_ordini_access` + any state |
| PATCH | `/api/ordini/:id/rows/:rowId/activate` | Per-row activation date; sets `confirm_data_attivazione=1`, calls GW, recounts confirmed rows, auto-flips to ATTIVO when count matches (Q2 fix). | `app_customer_relations` + state `INVIATO` |

### Derived / workflow-specific operations
- `sendToErp` returns `{ rows: [{ rowId, cdlan_systemodv_row, status: 'ok'|'error', error? }], stateTransitioned: bool, arxivarUploaded: bool }` on every call. UI renders this on partial failure.
- The auto-ATTIVO promotion is internal to `setActivationDate`; there is no explicit "mark active" endpoint.
- **Excluded in v1 (post-v1 follow-ups in `docs/TODO.md`):** `POST /api/ordini/:id/cancel-request`; partial-failure retry endpoint.

---

## Constraints and Non-Functional Requirements
- **Security or compliance.**
  - Mandatory parameterized queries (the source has SQLi everywhere — non-negotiable in the rewrite).
  - Frontend never holds DB credentials; vodka, Alyante, and GW are all backend-only.
  - All writes re-validate role + state from the server-held order record; client gating is never trusted.
  - `app_customer_relations` is the elevated role; technicians are implicit (anyone with `app_ordini_access`).
- **Performance or scale.** Home list is client-side in v1 (matches source); server-side pagination tracked as a follow-up if/when the dataset grows. No known hot path beyond the GW PDF endpoints which can return multi-megabyte payloads — backend should stream where possible.
- **Operational constraints.**
  - No DB migrations. Legacy `cdlan_int_fatturazione = '5'` is tolerated by the read-time enum mapper (Q3).
  - GW credentials/auth model must be moved server-side and timeouts + structured logging added (the export carries no auth declaration).
- **UX or accessibility expectations.** Standard mrsmith design system; no special accessibility requirements documented in the audit.

---

## Open Questions and Deferred Decisions

### Deferred to post-v1 (tracked in `docs/TODO.md` → Ordini App)
- **Cancel-order ("RICHIEDI ANNULLAMENTO") re-enablement.** Bind to `POST /orders/v2/order/{order_number}/cancel` per the Mistra NG spec; resolve `order_number` source field, `customer_name` source, and whether the legacy `from_cp != 0` block is re-imposed.
- **Partial-failure retry.** Allow re-sending only the rows that failed on a previous `sendToErp` attempt without duplicating committed rows.
- **`cdlan_int_fatturazione = '5'` data audit.** Decide whether to migrate legacy rows to `'4'` once volume and ownership permit.

### Not in v1 (no follow-up planned)
- Server-side Home pagination (audit flagged orphan queries; revisit only if the list grows large enough to need it).
- Order creation in this app (lives in Customer Portal / ERP).
- ORDINE PERSO transition (no valid flow in source).

---

## Acceptance Notes

### What the audit proved directly
- Two pages carry the workload (Home, Dettaglio); everything else is dead.
- The state machine (BOZZA → INVIATO → ATTIVO) is implicit in widget bindings; the auto-ATTIVO transition is the only route to ATTIVO and is driven by the per-row activation handler.
- All cross-system traffic flows through four datasources, three of which (vodka, Alyante, GW) are kept and one of which (`db-mistra`) is dropped.
- The source carries multiple bugs (Q2 always-false clause, Q6 operator precedence, broken cancel binding) and inconsistencies (Q3/Q4 alias drift) that the rewrite fixes deliberately.

### What the expert confirmed
- Port 1:1 within the in-scope surface; ignore dead features.
- Cancel-order deferred to post-v1; document the Mistra NG contract.
- `CheckConfirmRows` fixed to `IS NOT NULL` (Q2).
- Quadrimestrale canonical value `4`, no DB migration; backend tolerates legacy `5` on read (Q3).
- DTO field `activation_price`; UI label "Prezzo attivazione" (Q4).
- Roles: `app_ordini_access` (baseline) + `app_customer_relations` (elevated, app-agnostic role name); no dedicated technician role (Q5).
- `arxivar.isDisabled` rule fixed to `state NOT IN {ANNULLATO, PERSO, ATTIVO} AND user IN app_customer_relations` (Q6).
- Per-row send-to-ERP with structured per-row UI feedback; no transactional rollback (C1).
- BOZZA header save dual-writes `cdlan_cliente` (string) and `cdlan_cliente_id` (NUMERO_AZIENDA) (C2).
- Home list: render `cdlan_evaso` as Sì/No (B2); collapse the Arxivar tab into the Info tab (B3); render `cdlan_cliente_id` on the Info tab under Ragione sociale (B1).

### What still needs validation
- GW auth model (the export carries no auth declaration — production credentials must be recovered or re-provisioned during implementation).
- Volume of legacy `cdlan_int_fatturazione = '5'` rows (drives whether the dual-accept mapper is a long-term tolerance or a transient one).
- Per-row outcome UX rendering (final visual designed in implementation, not in this spec).

---

## Handoff
This specification is ready for `portal-miniapp-generator` to produce repo-specific implementation planning, UI review gates, and the mini-app scaffolding. The audit folder remains the authoritative source for Appsmith-side evidence; this spec captures the platform-neutral target.

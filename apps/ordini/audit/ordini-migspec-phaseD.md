# Ordini — Migration Spec · Phase D: Integration and Data Flow

Source: `app-inventory.md`, `page-audit.md`, `datasource-catalog.md`, `findings-summary.md`, `docs/mistra-dist.yaml`, `docs/IMPLEMENTATION-KNOWLEDGE.md`. Phase A/B/C resolutions applied.

Scope reminder: only Home + Dettaglio ordine carry forward; cancel-request flow is **deferred post-v1**.

---

## 1. External systems and purpose

### 1.1 vodka — MySQL (`10.129.32.7:3306`)
- **Owner of:** `orders` (header), `orders_rows` (lines).
- **Owner of these fields:** vodka-side state (`cdlan_stato`), `cdlan_evaso`, per-row activation state (`cdlan_data_attivazione`, `confirm_data_attivazione`), `cdlan_serialnumber`, `note_tecnici`, `arx_doc_number` (read; write path is external — see §7), `from_cp`, `cdlan_cliente`, `cdlan_cliente_id` (writeable per C2).
- **Access pattern in rewrite:** Go backend connects directly. All queries are parameterized (no Appsmith-style raw-string interpolation).

### 1.2 Alyante — Microsoft SQL Server (`172.16.1.16:1433`)
- **Owner of:** customer master data (`Tsmi_Anagrafiche_clienti`), ERP-side order document, ERP-side order state.
- **What this app reads:** `NUMERO_AZIENDA`, `RAGIONE_SOCIALE` for the Ragione sociale dropdown (filter: `DATA_DISMISSIONE IS NULL AND RAGGRUPPAMENTO_3 <> 'Ecommerce' AND TIPOLOGIA_AZIENDA <> 'DIPENDENTE'`).
- **What this app writes (directly):** **nothing**. ERP-side mutations are routed through the GW, never through a direct Alyante connection.
- **Access pattern in rewrite:** read-only Go backend connection. Frontend never sees Alyante.

### 1.3 GW internal CDLAN — REST (`https://gw-int.cdlan.net`)
The sanctioned bridge between this app and ERP / PDF generation / Arxivar. The rewrite calls the GW from the **Go backend only**; the React app never touches it.

| Endpoint | Purpose | Called from |
|---|---|---|
| `POST /orders/v1/erp` | Push one order row to the ERP. Payload hard-codes `cdlan_stato = "CREATO"` (intentional ERP-vs-vodka state divergence — keep). | `POST /api/ordini/:id/send-to-erp` (per-row loop). |
| `POST /orders/v1/set-order-activation` | Sync the per-row activation date to the ERP. Body `{cdlan_systemodv, cdlan_systemodv_row, cdlan_data_attivazione}`. | `PATCH /api/ordini/:id/rows/:rowId/activate` (after vodka UPDATE). |
| `POST /orders/v1/send-to-arxivar` | Multipart upload of the signed-order PDF. Form fields: `file`, `orderId`, `filename`, `multipart` (mime). | `POST /api/ordini/:id/send-to-erp` (terminal step on full success). |
| `GET /orders/v1/kick-off/:id` | Kickoff PDF. | `GET /api/ordini/:id/kickoff.pdf`. |
| `GET /orders/v1/activation-form/:id` | Activation form PDF (filename derived server-side from `orders.profile_lang`). | `GET /api/ordini/:id/activation-form.pdf`. |
| `GET /orders/v1/order/pdf/:id/generate` | Generate the order PDF (pre-Arxivar). Returns base64-or-raw. | `GET /api/ordini/:id/pdf`. |
| `GET /orders/v1/order/pdf/:id?from=vodka` | Fetch the signed-order PDF from Arxivar. Returns base64-or-raw. | `GET /api/ordini/:id/signed-pdf`. |

**Auth.** The exported datasource has no declared auth. Production likely uses an Authorization header or IP allowlist; the rewrite must move whatever credential is required server-side and add timeouts + structured logging (with request IDs).

**Payload normalization.** All PDF endpoints return base64-or-raw; the Appsmith app duplicates the heuristic in two JSObjects. The rewrite normalizes server-side and returns clean `application/pdf` to the browser.

### 1.4 Mistra NG Internal API — REST (per `docs/mistra-dist.yaml`)
- **Used in v1:** none (the only Ordini-related endpoint is `POST /orders/v2/order/{order_number}/cancel`, which is deferred post-v1 per the Phase A Q1 resolution and `docs/TODO.md`).
- **Hooked here for the post-v1 follow-up only.**

### 1.5 Arxivar (web UI) — `https://arxivar.cdlan.it`
- **Direct user-facing link**, rendered on the Info tab (Phase B B3) as an anchor of the form `https://arxivar.cdlan.it/#!/view/<arx_doc_number>/…` when `arx_doc_number IS NOT NULL`.
- **No backend integration** beyond the link target. Arxivar document creation goes through the GW (`send-to-arxivar`); document retrieval goes through the GW PDF endpoints.

### 1.6 Keycloak — OAuth2/OIDC
- **Used for:** authentication + role checks (`app_ordini_access`, `app_customer_relations`).
- **Token plumbing:** standard mrsmith pattern (handled by the backend at the router level, claims propagated to handlers).

---

## 2. Datasources dropped from the rewrite

| Dropped | Why |
|---|---|
| `db-mistra` (PostgreSQL, `10.129.32.20`) — Appsmith-era full access | The legacy Appsmith app used Mistra Postgres for `Ordini semplificati` (HubSpot potentials) and `get_payment_methods`. Both pages are dropped from v1. **Partial re-inclusion:** Ordini opens a read-only connection scoped to `orders.legacy_orders` only, used for quote↔order traceability (see §4 below). No access to `quotes.*`, `loader.*`, `products.*`, or the Mistra sequences — those remain exclusively in the quotes domain. |

## 3. Input contract — invariants Ordini assumes on incoming records

Order rows in `vodka.orders` and `vodka.orders_rows` reach Ordini already populated by an external creator (quotes converter or customer portal). Ordini consumes them as-is and trusts the following invariants. Any violation is a defect of the creator, not of Ordini.

| Field | Invariant | Notes for Ordini |
|---|---|---|
| `orders.cdlan_systemodv` | not null, allocated by the creator from Mistra sequence `orders.system_odv_alyante` | Stable ERP-correlated key. Never re-allocate. |
| `orders.cdlan_ndoc`, `orders.cdlan_anno` | stored as strings (MySQL coerces to INT on read) | Treat as `string` in the DTO; concatenate `<ndoc>/<anno>` for display. |
| `orders.cdlan_stato` | always `BOZZA` at creation | Ordini drives the transitions BOZZA→INVIATO→ATTIVO. |
| `orders.cdlan_dataconferma` | always `NULL` at creation | Ordini lets the operator set it in BOZZA Info SALVA. |
| `orders.cdlan_evaso`, `cdlan_chiuso` | always `0` at creation | Ordini sets `cdlan_evaso = 1` on send-to-ERP. |
| `orders.cdlan_valuta` | always `"EURO"` | Display-only. |
| `orders.cdlan_cliente_id` | **may be `NULL`** for orders created by the quotes converter today (bug Q-new-1 tracked in `apps/quotes/package-gpUtils.md`) | Fallback to `cdlan_cliente` (RAGIONE_SOCIALE) when null. Ordini populates the field when the operator edits Ragione sociale. |
| `orders.profile_lang` | 2-char code (`it`/`en`); derived from `quote.lingua` (3-char) by the creator | Used for PDF filename localization. |
| `orders.profile_pv` | 2-char province code (`provincia.slice(0,2)` by the creator) | Display as-is; no validation. |
| `orders.data_decorrenza` | empty string `""`, not NULL | Treat as nullable; display blank. |
| `orders.is_colo` | one of `0` / `Colocation variabile` / `Iaas payperuse` / `Iaas payperuse indiretto` (authoritative list = historical DB values) | Il creator dei valori IaaS non è documentato; Ordini tratta il campo come stringa opaca, lo legge as-is e lo mostra senza logica condizionale. Eventuali nuovi valori introdotti a monte verranno mostrati così come sono. |
| `orders.service_type` | comma-joined category names (derived from `quote.services` JSON by the creator) | Display as-is; do not re-derive. |
| `orders_rows.cdlan_systemodv_row` | not null per row, allocated from the same Mistra sequence | Used by GW endpoints for ERP sync. |
| `orders_rows.cdlan_prezzo` | **Italian-locale string** (`"1234,56"` with comma) | Parse on read for arithmetic/aggregation; render as-is for display. |
| `orders_rows.cdlan_prezzo_attivazione` | string with dot decimal (`"1234.56"`) | Historical asymmetry vs `cdlan_prezzo` — DTO normalizes both to `decimal`. See Phase A Q4. |
| `orders_rows.cdlan_descart` | multiline (CRLF-joined translated short + extended description) | Render with `<pre>`-style preservation of `\r\n`. |
| `orders_rows.cdlan_ragg_fatturazione` | `"A"` at creation | Display-only. |
| `orders_rows.cdlan_prezzo_cessazione` | `"0"` at creation | Display-only. |
| `orders_rows.confirm_data_attivazione` | `0` at creation | Ordini flips to `1` on per-row activation save. |

---

## 4. Traceability — origine ordine (quote ↔ order)

Mistra owns `orders.legacy_orders(quote_id, vodka_id, jdata)`, written by the quotes converter on every successful order creation. The unique writer in the monorepo is `backend/internal/quotes/order_conversion.go:insertLegacyOrder`. Ordini can rely on it as a read-only back-pointer.

**Endpoint contract.**
- `GET /api/ordini/:id` includes an optional `origin` field when the order has a quote ancestor:
  ```json
  "origin": {
    "type": "quote",
    "quote_id": 1234,
    "quote_code": "ABC-2025-0042",
    "quote_url": "/quotes/1234"
  }
  ```
- When no row in `legacy_orders` matches `vodka_id = :order_id`, `origin` is omitted. Ex-novo orders from the customer portal will not have a quote ancestor.

**Backend wiring.**
- Ordini Go module opens a Mistra Postgres connection scoped to `SELECT` on `orders.legacy_orders` only. The connection is reused for the entire request lifecycle of `GET /api/ordini/:id`; no other Mistra schemas are touched.
- The lookup is a single `SELECT quote_id FROM orders.legacy_orders WHERE vodka_id = $1` keyed by `vodka.orders.id`. Joined `quote_code` is resolved by querying `quotes.quote` via the **quotes module's existing API**, not directly — Ordini stays out of the quotes schema.

**UI affordance.**
- Dettaglio ordine header bar: when `origin.type == "quote"`, render an anchor `Da proposta {quote_code}` linking to `quote_url`. Hide if `origin` is absent.

---

---

## 5. End-to-end user journeys

### 3.1 Open and inspect an order
```
User → Portal sidebar → /ordini
  └─ GET /api/ordini  (vodka SELECT_Orders_Table-equivalent)
User → click "Visualizza" on a row → /ordini/:id
  └─ GET /api/ordini/:id          (vodka Order-equivalent + cdlan_cliente_id surfaced)
  └─ GET /api/ordini/:id/rows     (vodka RigheOrdine — header line for §5.4 + §5.5)
  └─ GET /api/ordini/:id/technical-rows (vodka RigheOrdineTecnici — header line for §5.6)
  └─ GET /api/ordini/ref/customers (Alyante erp_anagrafiche_cli — only when state == BOZZA)
```
The Alyante customer call is gated to BOZZA on the backend so non-editable detail loads stay quick and don't hit Alyante for nothing.

### 3.2 Finalize BOZZA → INVIATO (Send to ERP)
```
User edits Info tab → SALVA
  └─ PATCH /api/ordini/:id
       writes: cdlan_rif_ordcli, cdlan_dataconferma, cdlan_cliente, cdlan_cliente_id (C2)
       refresh: GET /api/ordini/:id

User attaches Arxivar PDF + clicks INVIA in ERP
  └─ POST /api/ordini/:id/send-to-erp  (multipart: signed PDF)
       backend loop, per row:
         POST gw /orders/v1/erp        (cdlan_stato="CREATO" hard-coded)
         on row failure: record { rowId, error }
       if all rows succeeded:
         vodka UPDATE orders SET cdlan_stato='INVIATO', cdlan_evaso=1
         POST gw /orders/v1/send-to-arxivar  (signed PDF)
       response: { rows: [...per-row status...], stateTransitioned, arxivarUploaded }

UI:
  - full success → toast + navigate back to Home
  - partial failure → stay on Dettaglio, render per-row outcome list
```

### 3.3 Edit Referenti (BOZZA or INVIATO)
```
PATCH /api/ordini/:id/referents
  writes: 9 cdlan_rif_* fields
  refresh: GET /api/ordini/:id
```

### 3.4 Edit serial number on a Riga (BOZZA only)
```
PATCH /api/ordini/:id/rows/:rowId/serial-number
  writes: orders_rows.cdlan_serialnumber WHERE id = :rowId AND orders_id = :id
  refresh: GET /api/ordini/:id/rows
```

### 3.5 Per-row activation date → auto-ATTIVO
```
User opens Modifica modal on a row → CONFERMA
  └─ PATCH /api/ordini/:id/rows/:rowId/activate  (body: { activation_date })
       backend (single transaction for the vodka writes):
         UPDATE orders_rows SET cdlan_data_attivazione=:date, confirm_data_attivazione=1
         POST gw /orders/v1/set-order-activation
         SELECT COUNT(id) WHERE orders_id=:id AND
                (confirm_data_attivazione=1 OR data_annullamento IS NOT NULL OR cdlan_qta=0)   -- Q2 fix
         if count == total rows:
            UPDATE orders SET cdlan_stato='ATTIVO'
       refresh: GET /api/ordini/:id, GET /api/ordini/:id/rows
```

### 3.6 Edit technical notes (any state, any user with `app_ordini_access`)
```
PATCH /api/ordini/:id/rows/:rowId/technical-notes
  writes: orders_rows.note_tecnici WHERE id = :rowId AND orders_id = :id
  refresh: GET /api/ordini/:id/technical-rows
```

### 3.7 PDF downloads
Four parallel one-shot flows, all backend-proxied (see Phase C §3 for filename rules and auth gates).

### 3.8 Cancel request — **NOT IN V1** (deferred per Q1 + TODO)

---

## 6. Background or triggered processes

The Appsmith app has no timers, schedules, or webhook listeners. All effects are user-initiated. The "background-like" behaviours that exist are all **synchronous side-effects of a user action**:

| Side-effect | When | Owner |
|---|---|---|
| `cdlan_stato → INVIATO` and `cdlan_evaso → 1` | terminal step of `POST /api/ordini/:id/send-to-erp` on full success | backend handler |
| Arxivar PDF upload (`POST gw /orders/v1/send-to-arxivar`) | terminal step of `POST /api/ordini/:id/send-to-erp` on full success | backend handler |
| `confirm_data_attivazione → 1` | side-effect of every `PATCH /api/ordini/:id/rows/:rowId/activate` | backend handler |
| `cdlan_stato → ATTIVO` | conditional on the row count match inside `PATCH /api/ordini/:id/rows/:rowId/activate` | backend handler |
| `cdlan_stato → ANNULLATO` | external — Mistra NG `cancel` endpoint flips the ERP-side state; **vodka is not updated by this flow**. Today this is silent divergence. Out of v1 scope but flagged. | external |
| `arx_doc_number → <value>` (write) | **not visible in the Appsmith export.** The app only reads it. See §7 — preserved as-is under 1:1. | external (out of this app's scope) |

---

## 7. Flows the export cannot reveal

### `arx_doc_number` write path — invisible but irrelevant under 1:1
The Appsmith app only reads `orders.arx_doc_number` (Info-tab Arxivar link, PDF button enable rules). It never writes it. The actual write happens somewhere outside this app — likely the GW after `send-to-arxivar` succeeds, possibly an Arxivar callback into another service. Under 1:1 the rewrite preserves that exact split: the new backend reads `arx_doc_number` the same way Appsmith does and does not attempt to write it. Whatever populates the column today continues to populate it tomorrow.

### ERP-side `cdlan_stato = "CREATO"` divergence
The `GW_SendToErp` payload hard-codes `cdlan_stato = "CREATO"` while vodka stores `INVIATO`. **Intentional** — the ERP owns its state vocabulary and `CREATO` is the right value on its side. Kept as-is. Documented here so a future reviewer doesn't "fix" the constant.

### ERP-side state divergence on cancel
Out of v1 scope (cancel deferred). The post-v1 cancel re-enablement TODO already covers the question of whether vodka needs a write-back when the Mistra cancel returns success. No additional decision required at this phase.

---

## 8. Data ownership boundaries (canonical)

| Data | Owner | Read in this app | Write in this app |
|---|---|---|---|
| Order header (`orders`) | vodka | yes (full SELECT) | yes (BOZZA edits, state flips, activation auto-promote, `cdlan_cliente_id` per C2) |
| Order rows (`orders_rows`) | vodka | yes | yes (activation date + confirm flag, serial number BOZZA, technical notes any state) |
| Customer master | Alyante | yes (filtered SELECT for the dropdown) | no |
| ERP order document | ERP via GW | no (write-only via GW) | no (driven by GW responses) |
| ERP-side state | ERP via GW | no | no |
| Signed PDF document | Arxivar via GW | yes (binary bytes) | yes (binary upload) |
| `arx_doc_number` linkage | external (write path lives outside this app — preserved as-is) | yes | no |
| Identity / authorization | Keycloak | yes (claims) | no |

---

## 9. Phase D exit criteria

**Phase D complete.** No open question — the only invisible flows (the `arx_doc_number` write path, the cancel-side state divergence) are either irrelevant under 1:1 or already covered by the cancel-deferral TODO. Ready for Phase E (final spec assembly).

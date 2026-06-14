# Aenad / Raenad — Backend Preventivi V1

Canonical backend handoff for the **Aenad Preventivi V1** implementation.

This document consolidates the validated backend documentation for the repository implementation. It intentionally describes **backend only**: the new React UI/UX for the Preventivi area is out of scope for this document.

## Status

The backend V1 for new Aenad quotes has been implemented in package:

- `backend/internal/raenad`

It is intentionally separate from the legacy archive package:

- `backend/internal/aenad` — read-oriented Archivio Aenad on legacy PostgreSQL schema `aenad`
- `backend/internal/raenad` — operational quote creation/editing/lifecycle domain on PostgreSQL schema `raenad`

Public API routes stay under the Aenad product namespace:

- external browser/API path: `/api/aenad/v1/...`
- internal Go mux route pattern: `/aenad/v1/...` (`main.go` strips `/api` before routing)

Verification commands for this backend implementation:

```sh
git diff --check
cd backend && go test ./internal/raenad
cd backend && go test ./...
cd backend && go build ./...
```

These commands are expected to pass for the implemented backend. No frontend/UI implementation is included in this backend scope.

---

## Contents

1. Architecture and scope
2. Wiring, auth, and dependencies
3. Configuration
4. Data model
5. API contract
6. Quote lifecycle
7. HubSpot queue and worker
8. PDF export and attach
9. Prospect contact-first flow
10. Validation and errors
11. Operational notes
12. Tests
13. Appendices

---

## 1. Architecture and scope

The new quote flow uses **Raenad** as the operational write model for Aenad quotes.

Core principles:

- **Separate operational domain**: new quote creation/editing lives in `backend/internal/raenad`; the existing `backend/internal/aenad` archive remains read-oriented.
- **Lowercase PostgreSQL schema**: new tables/functions use schema `raenad` on Mistra, avoiding the mixed-case legacy `aenad` schema style.
- **DB-owned totals**: line/header totals are calculated by PostgreSQL triggers. The API does not accept client-supplied totals.
- **Decimal strings**: monetary and quantity values backed by `numeric(18,4)` are exposed and accepted as strings, never JSON floats.
- **HubSpot async writes**: save/update/PDF attach do not depend on synchronous HubSpot writes. They enqueue durable rows in `mrsmith.hubspot_request` on Anisetta.
- **HubSpot is commercial source of truth**: Raenad stores authoring/sync state and snapshots of pipeline/stage, but does not create a parallel commercial taxonomy.
- **PDF binary not persisted**: PDF export revisions persist metadata, checksum, and canonical `render_payload`; download/attach regenerate bytes through Carbone.
- **Backend-only integrations**: HubSpot, Alyante, Mistra, Anisetta, and Carbone are accessed by backend only, never directly by frontend.

### Included in this backend scope

- Reference endpoints: customers, payment methods, stages, defaults, articles.
- Prospect creation/resolution against live HubSpot.
- Quote CRUD and full-save update flow.
- Manual ready transition.
- Manual HubSpot retry.
- Explicit HubSpot stage transition with live conflict check.
- PDF export list/create/download/attach enqueue.
- Mistra schema `raenad` and transactional store.
- Shared HubSpot queue tables on Anisetta.
- Queue worker/reconciler for deal create/update and PDF attach.
- Shared HubSpot client extensions for CRM/file/note operations.
- Backend tests for the implemented behavior.

### Excluded from this backend scope

- React UI/UX and components.
- Granular roles beyond Aenad access.
- Required custom HubSpot properties.
- Local/versioned pipeline-stage taxonomy.
- PDF binary/blob persistence.
- Eco-contributions, withholdings, `qta_shown`, required contact phone.

### Main files

```text
backend/internal/raenad/
├── handler.go            # Deps, RegisterRoutes, route guard, dependency checks
├── types.go              # DTOs and state constants
├── actor.go              # actor extraction from auth claims
├── decimal.go            # numeric(18,4) string validation/normalization
├── store.go              # scan columns and row → DTO mapping
├── reference.go          # customers, payment methods, stages, defaults, articles
├── prospect.go           # live HubSpot prospect contact-first flow
├── quote_lifecycle.go    # list/create/get/update/ready/retry
├── stage_transition.go   # explicit HubSpot dealstage transition
├── pdf_export.go         # PDF revision metadata, download, attach enqueue
├── hubspot_queue.go      # enqueue, dedupe, latest-wins payloads
└── hubspot_worker.go     # worker/reconciler for create/update/attach
```

Other relevant files:

- `backend/internal/platform/hubspot/crm.go` — shared CRM methods.
- `backend/internal/platform/hubspot/files.go` — file upload and note attachment methods.
- `backend/cmd/server/main.go` — route and worker wiring.
- `deploy/migrations/023_raenad_schema.sql` — Mistra `raenad` schema.
- `deploy/migrations/024_mrsmith_hubspot_queue.sql` — Anisetta queue tables and runtime config seed.

---

## 2. Wiring, auth, and dependencies

Routes are registered with:

```go
raenad.RegisterRoutes(api, raenad.Deps{
    Mistra:       mistraDB,
    ConfigDB:     anisettaDB,
    Alyante:      alyanteDB,
    HubSpot:      raenadHubSpot,
    HubSpotStage: raenadHubSpotStage,
    Carbone:      aenadCarboneSvc,
    Logger:       logger,
})
```

All routes are protected by:

```go
acl.RequireRole(applaunch.AenadAccessRoles()...)
```

The relevant Keycloak app role is `app_aenad_access`.

### Dependency fields

| Field | Type / source | Used for |
|---|---|---|
| `Mistra` | `*sql.DB` / `mistraDB` | `raenad` schema, `loader.*` mirrors, `aenad."TIva"` VAT master |
| `ConfigDB` | `*sql.DB` / `anisettaDB` | `mrsmith.runtime_config`, `mrsmith.hubspot_request` |
| `Alyante` | `*sql.DB` / `alyanteDB` | MSSQL article search |
| `HubSpot` | `HubSpotProspectClient` / shared HubSpot client | live prospect contact/company flow |
| `HubSpotStage` | `HubSpotStageClient` / shared HubSpot client | live dealstage read/update |
| `Carbone` | `PDFRenderer` / `aenadCarboneSvc` | PDF rendering for download and worker attach |
| `Logger` | `*slog.Logger` | structured logging |

Each handler checks only the dependencies it needs. Missing services return targeted `503` errors rather than disabling unrelated routes.

Dependency error codes:

| Missing dependency/config | Error code |
|---|---|
| Mistra | `raenad_database_not_configured` |
| Anisetta / runtime config DB | `raenad_config_not_configured` |
| Alyante | `alyante_database_not_configured` |
| HubSpot prospect/stage client | `hubspot_not_configured` |
| Carbone renderer or PDF template | `raenad_pdf_not_configured` |

The Raenad HubSpot queue worker is started only when **Mistra + Anisetta + HubSpot** are configured. Carbone is **not** a startup gate; it is required only when processing `hubspot.attach_pdf` requests.

---

## 3. Configuration

### Environment-level services

| Environment / service | Required for |
|---|---|
| `MISTRA_DSN` | quote store, customers, payment methods, stages mirror, VAT master |
| `ANISETTA_DSN` | runtime config, HubSpot queue |
| `ALYANTE_DSN` | `GET /quotes/articles` |
| `HUBSPOT_API_KEY` | prospect flow, stage transition, worker create/update/attach |
| `CARBONE_API_KEY` | PDF download and worker attach PDF rendering |
| `AENAD_APP_URL` | portal launcher URL, not core backend behavior |

### Runtime config in `mrsmith.runtime_config`

Seeded by `deploy/migrations/024_mrsmith_hubspot_queue.sql`:

| Namespace | Key | Seed value shape | Purpose |
|---|---|---|---|
| `raenad` | `hubspot_queue_worker` | `{"enabled":false,"interval_seconds":30,"batch_size":20,"max_attempts":8,"backoff_base_seconds":30,"backoff_max_seconds":3600}` | worker switch and tuning; disabled by default |
| `raenad` | `hubspot_deal_pipeline` | `{"pipeline_id":"3883934920","initial_dealstage_id":"5521139911"}` | pipeline and initial dealstage for quote-created deals |
| `raenad` | `hubspot_deal_owner` | `{"fallback_owner_email":"service01@cdlan.it"}` | fallback HubSpot owner email |
| `aenad` | `quote_defaults` | `{"cod_iva":"22"}` | default VAT code for new quote lines / article initializers |

Manual per-environment config, **not seeded** by migration `024`:

| Namespace | Key | Expected value shape | Purpose |
|---|---|---|---|
| `raenad` | `carbone_quote_template` | `{"template_id":"<carbone-template-id>"}` | dedicated Raenad quote PDF template |

Important notes:

- `aenad.quote_defaults` is under namespace `aenad`, not `raenad`.
- `raenad.carbone_quote_template` must be created per environment; the legacy `aenad.carbone_offerta` template is not reused as fallback.
- Worker config may include optional `lock_timeout_seconds`; if absent, code defaults apply. Migration `024` does not seed `lock_timeout_seconds`.

---

## 4. Data model

### Mistra schema `raenad` — migration `023_raenad_schema.sql`

#### `raenad.quote`

Quote header. Main column groups:

- Identity/audit: `id`, `quote_number`, `created_at`, `updated_at`, `created_by`, `updated_by`.
- Numbering: `quote_number` defaults to `common.new_document_number('AE-')`, unique. The shared sequence is globally unique but not guaranteed gap-free per prefix.
- Authoring state: `authoring_status` in `draft | ready | archived`.
- Technical HubSpot sync state: `hubspot_sync_status` in `pending | succeeded | failed`, plus `hubspot_sync_error`, `hubspot_synced_at`.
- HubSpot references/snapshots: `hubspot_company_id`, `hubspot_contact_id`, `hubspot_deal_id`, `hubspot_pipeline_id/_label`, `hubspot_dealstage_id/_label`.
- Printable customer snapshot: `customer_name`, VAT/tax fields, PEC/email, address fields, language, `numero_azienda_snapshot`.
- Contact snapshot: first/last/full name, email, role.
- Document/payment fields: `document_date`, `payment_method_code/_label`, `payment_bank_details`, `description`, `internal_notes`.
- DB-owned totals: `total_net`, `total_vat`, `total_gross`, `total_purchase`, `total_gain` (`numeric(18,4)`).

There is no physical FK from quote snapshots to the HubSpot mirror tables; snapshots are deliberately decoupled from potentially delayed mirror refreshes.

#### `raenad.quote_line`

Quote lines. FK `quote_id` cascades on quote delete.

Main fields:

- `position`
- `line_type`: `item`, `description`, or `spacer`
- item/commercial fields: `item_code`, `item_description`, `description`, `unit_of_measure`, `qta`, `unit_price`, `discounts`, `cod_iva`, `iva_percent_snapshot`, `purchase_unit_price`
- DB-owned line totals: `line_net`, `line_vat`, `line_gross`, `line_purchase`, `line_gain`

Only `item` rows carry economic calculations. Non-item rows have economic totals nulled by the trigger.

`iva_percent_snapshot` is sticky: if set, it is not automatically re-resolved when `cod_iva` changes. Set it to `NULL` to force fallback resolution from `aenad."TIva"`.

#### `raenad.quote_pdf_export`

Immutable PDF export revision rows.

- Unique `(quote_id, revision)`.
- Content/identity columns: `revision`, `filename`, `content_type`, `checksum_sha256`, `render_payload`, `created_at`, `created_by`.
- Mutable HubSpot attach status columns: `hubspot_attachment_status`, `hubspot_file_id`, `hubspot_note_id`, `hubspot_deal_id`, `hubspot_attached_at`, `hubspot_error`.
- Attachment status values: `pending | attached | failed | skipped`.
- A BEFORE UPDATE trigger blocks modification of content columns; only the `hubspot_*` attachment fields are mutable.

The PDF binary is not stored in this table or elsewhere in V1.

#### `raenad.quote_event`

Append-only event log:

- `quote_id`
- `event_type`
- `actor_subject`
- `payload jsonb`
- `created_at`

Common events include `created`, `updated`, `marked_ready`, `hubspot_create_enqueued`, `hubspot_update_enqueued`, `hubspot_retry_enqueued`, `hubspot_pdf_attach_enqueued`, `hubspot_enqueue_failed`, `hubspot_stage_transition`, `hubspot_sync_succeeded`, `hubspot_sync_failed`, and `hubspot_update_rearmed`.

### DB-owned calculations

Migration `023` defines calculation helpers and triggers:

- `raenad.discount_multiplier(discounts text)` parses sequential discounts such as `10+5` as `(1 - 0.10) * (1 - 0.05)`. It accepts `%` suffix and comma decimal separator.
- `raenad.line_net(unit_price, quantity, discounts)` returns `unit_price * quantity * discount_multiplier` as `numeric(18,4)`.
- `raenad.resolve_iva_percent(cod_iva)` looks up `aenad."TIva"."PercIva"`.
- `raenad.quote_line_set_totals()` calculates row totals for item rows:
  - `line_purchase = purchase_unit_price * qta`
  - `line_net = qta * unit_price * discount_multiplier(discounts)`
  - `line_gain = line_net - line_purchase`
  - `line_vat = line_net * iva_percent_snapshot / 100`
  - `line_gross = line_net + line_vat`
- `raenad.quote_line_recalculate_header()` recalculates header totals after line changes.
- `raenad.recalculate_quote_totals(...)` supports bulk/data-fix recalculation.

The backend rejects client-owned total fields and reloads persisted rows after commit to return DB-calculated totals.

### Anisetta queue schema — migration `024_mrsmith_hubspot_queue.sql`

#### `mrsmith.hubspot_request`

Durable async request table for HubSpot operations.

Main fields:

- `status`: `pending | locked | succeeded | failed | dead | cancelled`
- `operation`: `hubspot.create_deal | hubspot.update_deal | hubspot.attach_pdf`
- `entity_type`, `entity_id`
- `dedupe_key` unique
- `payload jsonb`, `response jsonb`
- `attempt_count`, `max_attempts`, `next_attempt_at`
- `locked_at`, `locked_by`
- `last_error`
- `created_at`, `updated_at`

Indexes include due-polling, `(entity_type, entity_id)`, and status/next-at lookup indexes.

#### `mrsmith.hubspot_request_attempt`

Per-attempt history for queue processing, linked to `hubspot_request`.

---

## 5. API contract

External base URL: `/api/aenad/v1`  
Internal mux base: `/aenad/v1`

All endpoints require `app_aenad_access`.

### 5.1 Reference endpoints

#### `GET /quotes/customers`

External: `GET /api/aenad/v1/quotes/customers`

Dependencies: Mistra.

Query:

- `q` — optional free text search
- `limit` — default `25`, max `100`
- `include_without_numero_azienda` — default `false`

Source: `loader.hubs_company`. By default, companies without `numero_azienda` are excluded.

Response: array of selections:

```json
{
  "hubspot_company_id": "12345",
  "customer": {
    "name": "Cliente SpA",
    "vat": "...",
    "tax_code": "...",
    "pec": "...",
    "email": "...",
    "address": "...",
    "zip": "...",
    "city": "...",
    "province": "...",
    "country": "...",
    "language": "...",
    "numero_azienda_snapshot": "1001"
  },
  "domain": "cliente.it",
  "alyante_id_anagrafica": "..."
}
```

#### `POST /quotes/prospects`

External: `POST /api/aenad/v1/quotes/prospects`

Dependencies: HubSpot.

Creates/resolves a prospect against live HubSpot using a contact-first flow.

Typical body:

```json
{
  "email": "mario.rossi@example.com",
  "first_name": "Mario",
  "last_name": "Rossi",
  "role": "commercial",
  "company_name": "Example Srl",
  "domain": "example.com",
  "customer": {
    "name": "Example Srl"
  }
}
```

Response `200`: `prospectResponse` with `hubspot_company_id`, optional `hubspot_contact_id`, customer snapshot, contact snapshot.

#### `GET /quotes/payment-methods`

External: `GET /api/aenad/v1/quotes/payment-methods`

Dependencies: Mistra.

Source: `loader.erp_metodi_pagamento` where `selezionabile IS TRUE`.

Response: array of `{ cod_pagamento, desc_pagamento }`.

#### `GET /quotes/stages`

External: `GET /api/aenad/v1/quotes/stages`

Dependencies: Mistra + ConfigDB.

Reads configured pipeline from `raenad.hubspot_deal_pipeline`, then returns stages from `loader.hubs_stages` for that pipeline, with labels from `loader.hubs_pipeline` when available.

Response:

```json
{
  "pipeline_id": "3883934920",
  "pipeline_label": "...",
  "initial_dealstage_id": "5521139911",
  "items": [
    {
      "id": "5521139911",
      "label": "...",
      "pipeline_id": "3883934920",
      "pipeline_label": "...",
      "display_order": 0,
      "is_initial": true
    }
  ]
}
```

#### `GET /quotes/defaults`

External: `GET /api/aenad/v1/quotes/defaults`

Dependencies: ConfigDB.

Reads `mrsmith.runtime_config` namespace `aenad`, key `quote_defaults`.

Response:

```json
{ "line": { "cod_iva": "22" } }
```

#### `GET /quotes/articles`

External: `GET /api/aenad/v1/quotes/articles`

Dependencies: Alyante + ConfigDB.

Query:

- `q` — optional article search text
- `limit` — default `25`, max `100`
- `lang` — `ita` default; `en`/`eng` for English descriptions

Uses direct Alyante MSSQL query over active sales list data (`LI10_LISTARTIC`) and related article/description tables. It does not use a Mistra mirror for article search.

Response: array of line initializers:

```json
{
  "line_type": "item",
  "item_code": "ABC123",
  "item_description": "Descrizione breve",
  "description": "Descrizione estesa",
  "unit_of_measure": "NR",
  "qta": null,
  "unit_price": "123.4500",
  "discounts": null,
  "cod_iva": "22",
  "purchase_unit_price": null
}
```

### 5.2 Quote CRUD and lifecycle endpoints

#### `GET /quotes`

External: `GET /api/aenad/v1/quotes`

Dependencies: Mistra.

Query:

- `page` — default `1`
- `page_size` — default `50`, max `100`
- `q` — search on quote number, customer name, description
- `authoring_status`
- `hubspot_sync_status`

Response `200`:

```json
{
  "items": [
    {
      "id": 1,
      "quote_number": "AE-1003/2026",
      "authoring_status": "draft",
      "hubspot_sync_status": "pending",
      "customer_name": "Cliente SpA",
      "document_date": "2026-06-15",
      "description": "Preventivo servizi",
      "total_net": "0.0000",
      "total_vat": "0.0000",
      "total_gross": "0.0000"
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 50
}
```

#### `POST /quotes`

External: `POST /api/aenad/v1/quotes`

Dependencies: Mistra + ConfigDB.

Creates a quote in `draft`, with `hubspot_sync_status='pending'`, initial pipeline/stage snapshot from runtime config, and post-commit enqueue of `hubspot.create_deal`.

Required for create:

- `hubspot_company_id`
- `customer.name`
- `document_date` (`YYYY-MM-DD`)
- `description`

Optional in draft, but validated if present:

- `payment_method_code` — must exist and be `selezionabile`
- `lines` — decimal/VAT validation applies to economic item lines

Typical body:

```json
{
  "hubspot_company_id": "12345",
  "hubspot_contact_id": "67890",
  "customer": {
    "name": "Cliente SpA",
    "vat": "...",
    "numero_azienda_snapshot": "1001"
  },
  "contact": {
    "first_name": "Mario",
    "last_name": "Rossi",
    "full_name": "Mario Rossi",
    "email": "mario.rossi@example.com",
    "role": "commercial"
  },
  "document_date": "2026-06-15",
  "payment_method_code": "402",
  "payment_bank_details": "IBAN ...",
  "description": "Preventivo servizi",
  "internal_notes": "Nota interna",
  "lines": [
    {
      "line_type": "item",
      "item_code": "ABC123",
      "item_description": "Servizio",
      "description": "Servizio mensile",
      "unit_of_measure": "NR",
      "qta": "1",
      "unit_price": "100.0000",
      "discounts": "10%",
      "cod_iva": "22",
      "purchase_unit_price": "50.0000"
    }
  ]
}
```

Response `201`: full `quoteResponse` with header, snapshots, payment, lines, events, and DB-calculated totals.

Notes:

- The backend derives `payment_method_label` from `loader.erp_metodi_pagamento`; it does not trust a client label.
- Line `position` is assigned from array order during normalization.
- Client-owned fields such as `quote_number`, `total_*`, line IDs, and line totals are rejected.

#### `GET /quotes/{id}`

External: `GET /api/aenad/v1/quotes/{id}`

Dependencies: Mistra.

Returns full quote detail:

- quote summary fields
- nested `customer`, `contact`, `payment`
- `internal_notes`
- `lines` ordered by `position, id`
- `events` ordered by `created_at, id`

#### `PUT /quotes/{id}`

External: `PUT /api/aenad/v1/quotes/{id}`

Dependencies: Mistra.

Full-save update of editable header and entire lines array:

- locks quote with `FOR UPDATE`
- normalizes/validates body and decimal strings
- replaces all lines (`DELETE` + `INSERT`)
- detects relevant business changes by comparing normalized current state vs payload
- if quote was `ready` and relevant fields changed, demotes `ready → draft`
- if relevant fields changed, resets `hubspot_sync_status='pending'`, clears sync error/time, and post-commit enqueues latest `hubspot.update_deal`

`internal_notes` alone does not demote from ready and does not trigger quote-owned HubSpot update.

Response `200`: reloaded `quoteResponse`.

#### `POST /quotes/{id}/ready`

External: `POST /api/aenad/v1/quotes/{id}/ready`

Dependencies: Mistra.

Promotes a quote to `ready` after validation.

Ready requirements:

- `hubspot_company_id` present
- `customer.name` present
- `document_date` present
- `description` present
- `payment_method_code` present and selectable
- at least one `item` line with `qta`, `unit_price`, and either `cod_iva` or `iva_percent_snapshot`

Effects:

- sets `authoring_status='ready'`
- refreshes payment method label from DB
- appends `marked_ready`
- does **not** change HubSpot stage
- does **not** enqueue HubSpot update by itself

Response `200`: reloaded `quoteResponse`.

#### `POST /quotes/{id}/hubspot/retry`

External: `POST /api/aenad/v1/quotes/{id}/hubspot/retry`

Dependencies: Mistra + ConfigDB.

Manual queue retry:

- if `hubspot_deal_id` is missing, re-enqueues `hubspot.create_deal`
- if `hubspot_deal_id` exists, re-enqueues `hubspot.update_deal`
- appends `hubspot_retry_enqueued`

Response `200`: reloaded `quoteResponse`.

#### `POST /quotes/{id}/hubspot/stage`

External: `POST /api/aenad/v1/quotes/{id}/hubspot/stage`

Dependencies: Mistra + ConfigDB + HubSpotStage.

Explicit HubSpot dealstage transition with optimistic conflict check.

Body:

```json
{
  "expected_dealstage_id": "5521139911",
  "target_dealstage_id": "5521139912"
}
```

Rules:

- quote must be `ready`
- `hubspot_deal_id` must exist
- target stage must exist in `loader.hubs_stages` for the configured pipeline
- backend reads live HubSpot stage with `GetDealStage`
- if remote stage differs from `expected_dealstage_id`, returns `409 hubspot_stage_conflict` with remote stage details
- if it matches, backend updates only `dealstage` on HubSpot, persists local pipeline/stage snapshot, and appends `hubspot_stage_transition`

This endpoint does **not** enqueue `hubspot.update_deal` and does **not** demote ready to draft.

### 5.3 PDF endpoints

#### `GET /quotes/{id}/pdf-exports`

External: `GET /api/aenad/v1/quotes/{id}/pdf-exports`

Dependencies: Mistra.

Returns immutable PDF export revisions, newest revision first. Each item includes `is_stale`, computed by comparing the saved revision checksum to the checksum of the quote's current canonical render payload.

Response `200`:

```json
{
  "items": [
    {
      "id": 501,
      "quote_id": 101,
      "revision": 1,
      "filename": "Preventivo AE-1003-2026 rev1.pdf",
      "content_type": "application/pdf",
      "checksum_sha256": "...",
      "hubspot_attachment_status": "pending",
      "is_stale": false
    }
  ]
}
```

#### `POST /quotes/{id}/pdf-exports`

External: `POST /api/aenad/v1/quotes/{id}/pdf-exports`

Dependencies: Mistra + ConfigDB + Carbone dependency present.

Creates or reuses a PDF export revision. This endpoint verifies the dedicated Carbone template configuration but does **not** render or persist the PDF binary.

Rules:

- quote must be `ready`
- `raenad.carbone_quote_template` must exist and contain a valid `template_id`
- build canonical `render_payload`
- compute SHA-256 over canonical payload
- if latest revision has the same checksum, return that existing revision with `200`
- otherwise insert a new immutable revision with `revision = latest + 1` and return `201`

No PDF bytes are generated or stored during create/reuse.

#### `GET /quotes/{id}/pdf-exports/{exportID}/download`

External: `GET /api/aenad/v1/quotes/{id}/pdf-exports/{exportID}/download`

Dependencies: Mistra + ConfigDB + Carbone.

Regenerates PDF bytes from the saved `render_payload` and dedicated Carbone template.

Response: `application/pdf` with `Content-Disposition: attachment`.

On Carbone generation failure: `502 raenad_pdf_generation_failed`.

#### `POST /quotes/{id}/pdf-exports/{exportID}/attach`

External: `POST /api/aenad/v1/quotes/{id}/pdf-exports/{exportID}/attach`

Dependencies: Mistra + ConfigDB.

Queues asynchronous attach of an export revision to the HubSpot deal.

Rules:

- quote and export must exist
- quote must have `hubspot_deal_id`
- endpoint does not call HubSpot synchronously
- endpoint does not call Carbone synchronously
- enqueues `hubspot.attach_pdf` with `entity_type='raenad.quote_pdf_export'`, `entity_id=<exportID>`
- dedupe key: `raenad:quote:{quote_id}:pdf-export:{export_id}:attach`
- appends `hubspot_pdf_attach_enqueued`

Response `202 Accepted`:

```json
{
  "hubspot_request_id": 123,
  "status": "pending",
  "action": "created"
}
```

The worker later regenerates PDF bytes, uploads to HubSpot Files, creates a deal-associated note, and updates only the export `hubspot_*` status fields.

---

## 6. Quote lifecycle

Authoring states:

- `draft` — initial state. Editable freely.
- `ready` — manually validated complete quote. Required for PDF export and explicit stage transition.
- `archived` — present in DB CHECK constraint but not exposed by V1 API.

State transitions:

```text
POST /quotes                  -> draft
POST /quotes/{id}/ready       -> ready, after validation
PUT /quotes/{id} relevant edit: ready -> draft
PUT /quotes/{id} notes only:  ready remains ready
POST /quotes/{id}/hubspot/stage: ready remains ready
```

Relevant changes for ready demotion include changes to:

- HubSpot company/contact IDs
- customer snapshot
- contact snapshot
- document date
- payment method / payment label / bank details
- description
- quote lines and line economics

Not relevant for demotion:

- `internal_notes` alone
- explicit HubSpot stage transition
- worker-maintained sync fields
- PDF export attachment status

HubSpot commercial state lives in HubSpot pipeline/stage. Raenad stores a snapshot and technical sync state only.

---

## 7. HubSpot queue and worker

All quote-owned HubSpot writes are persisted as queue rows in `mrsmith.hubspot_request` on Anisetta.

### Operations and dedupe keys

| Operation | Entity type | Entity id | Dedupe key |
|---|---|---|---|
| `hubspot.create_deal` | `raenad.quote` | quote id | `raenad:quote:{quote_id}:deal:create` |
| `hubspot.update_deal` | `raenad.quote` | quote id | `raenad:quote:{quote_id}:deal:update` |
| `hubspot.attach_pdf` | `raenad.quote_pdf_export` | export id | `raenad:quote:{quote_id}:pdf-export:{export_id}:attach` |

### Enqueue behavior

- **Create deal**: idempotent insert; duplicate dedupe key returns existing request.
- **Update deal**: latest-wins coalescing. Existing non-locked `pending`, `failed`, or `succeeded` request is reset to `pending` with the newest payload. Locked in-flight request is not overwritten; worker detects stale payload and re-arms if needed.
- **Retry**: manual retry can re-arm non-locked request states.
- **Attach PDF**: idempotent per export revision; duplicate attach for the same export returns existing request.

Deal update payloads include `source_quote_updated_at` so the worker can detect whether the quote changed while a payload was in flight.

### Worker lifecycle

`HubSpotQueueWorker` runs from `main.go` when Mistra + Anisetta + HubSpot are configured. Each cycle:

1. Reads `raenad.hubspot_queue_worker` config.
2. Skips if disabled.
3. Opens an Anisetta connection.
4. Attempts advisory lock `44000062`; if another instance owns it, skips the cycle.
5. Recovers stale `locked` requests beyond lock timeout.
6. Reconciles `raenad.quote` rows with `hubspot_sync_status='pending'` and no live pending/locked request.
7. Claims due `pending` requests with `FOR UPDATE SKIP LOCKED`.
8. Processes each request.
9. Records attempts in `mrsmith.hubspot_request_attempt`.
10. Marks success, re-arms with exponential backoff, or marks dead/failed after max attempts.

Code defaults if config is missing/partial include disabled worker, 1 minute interval, batch size 10, max attempts 5, 1 minute to 1 hour backoff, and 15 minute lock timeout. Migration seed values differ where explicitly provided and keep the worker disabled by default.

### Deal create

Worker behavior for `hubspot.create_deal`:

- reloads current quote from Mistra
- if `hubspot_deal_id` already exists, reconciles as succeeded without creating another deal
- resolves owner by authenticated actor email against `loader.hubs_owner.email` where `archived=false`
- if actor email has no owner, uses fallback email from `raenad.hubspot_deal_owner`
- if fallback also fails, fails as misconfiguration; no unowned deal is created
- creates deal with standard properties:
  - `dealname = "{quote_number} - {customer_name}"`
  - `amount = total_net`
  - `closedate = document_date + 30 days`
  - `hubspot_owner_id`
  - `pipeline` and `dealstage` from quote snapshots/config-derived values, only on create
- associates deal to company and optional contact
- stores returned `hubspot_deal_id`, marks `hubspot_sync_status='succeeded'`, clears error, sets `hubspot_synced_at`
- appends worker success event

### Deal update

Worker behavior for `hubspot.update_deal`:

- reloads current quote
- if `hubspot_deal_id` is missing, defers/retries until create completes
- resolves owner
- updates quote-owned fields only:
  - deal name
  - amount
  - close date
  - owner
- does **not** send `pipeline` or `dealstage`
- ensures deal-company and deal-contact associations where present
- marks quote sync succeeded on success
- if payload `source_quote_updated_at` is stale relative to current quote, marks current request succeeded and re-arms a latest update request

### PDF attach

Worker behavior for `hubspot.attach_pdf`:

- loads quote and PDF export revision
- requires `hubspot_deal_id`
- resolves dedicated Raenad Carbone template
- regenerates PDF bytes from saved `render_payload`
- uploads file privately to HubSpot folder `/mrsmith/raenad/quote-pdfs`
- creates generic note associated to the deal with the uploaded file attached
- updates only export attachment fields:
  - `hubspot_attachment_status='attached'`
  - `hubspot_file_id`
  - `hubspot_note_id`
  - `hubspot_deal_id`
  - `hubspot_attached_at`
  - clears `hubspot_error`
- on terminal failure marks export `hubspot_attachment_status='failed'` and stores sanitized `hubspot_error`

PDF attach does not mutate quote authoring/sync fields.

---

## 8. PDF export and attach

Raenad PDF exports are immutable metadata revisions.

### Render payload

Canonical payload shape:

```json
{
  "convertTo": "pdf",
  "data": {
    "quote_number": "AE-1003/2026",
    "document_date": "2026-06-15",
    "description": "Preventivo servizi",
    "customer": { "name": "Cliente SpA" },
    "contact": { "full_name": "Mario Rossi" },
    "payment": { "method_code": "402", "method_label": "..." },
    "lines": [
      {
        "position": 1,
        "line_type": "item",
        "item_code": "ABC123",
        "qta": "1.0000",
        "unit_price": "100.0000",
        "line_net": "100.0000"
      }
    ],
    "totals": {
      "net": "100.0000",
      "vat": "22.0000",
      "gross": "122.0000",
      "purchase": "50.0000",
      "gain": "50.0000"
    }
  }
}
```

The SHA-256 checksum of this canonical payload determines:

- whether `POST /pdf-exports` reuses the latest revision
- whether a listed revision is stale compared with current quote state

### Binary policy

V1 does not persist PDF bytes. This is intentional.

- Create/reuse stores `render_payload`, checksum, filename, metadata.
- Download regenerates bytes from saved payload and template.
- Attach worker regenerates bytes from saved payload and template.

### Attach policy

Attach is asynchronous and revision-scoped.

- The same export revision has a stable dedupe key.
- Multiple revisions of the same quote may be attached independently.
- Stale revisions can still be attached because they are immutable historical artifacts.
- Only `hubspot_*` fields on `raenad.quote_pdf_export` are updated by attach processing.

---

## 9. Prospect contact-first flow

`POST /quotes/prospects` creates/resolves a customer prospect against live HubSpot when the company is not yet available in the Mistra mirror.

Flow:

1. Validate and normalize email.
2. Get contact by email from HubSpot.
3. Create contact if missing.
4. Resolve company:
   - first via contact associations,
   - then by domain search,
   - then create company from domain/company name when needed.
5. Associate contact and company if not already associated.
6. Return `hubspot_company_id`, optional `hubspot_contact_id`, and printable snapshots.

Domain fallback avoids using known public mailbox domains as company domains.

The endpoint returns live HubSpot IDs immediately and does not wait for Mistra mirror refresh.

---

## 10. Validation and errors

### Request validation

- JSON decoders use `DisallowUnknownFields` for mutable endpoints.
- Request bodies are limited to 1 MiB.
- Multiple JSON documents in one body are rejected.
- Dates must be `YYYY-MM-DD`.
- Decimal strings are normalized for `numeric(18,4)` without using float and without rounding.
- Comma decimal separator is accepted and normalized to dot.
- Maximum precision is 14 integer digits plus 4 decimals.
- Empty numeric strings become SQL `NULL`.
- Client-owned totals and IDs are rejected:
  - top-level `quote_number`, `total_*`
  - line `id`, `quote_id`, `line_*`
- Payment method is accepted only if `loader.erp_metodi_pagamento.selezionabile IS TRUE`.
- Economic item lines require resolvable VAT when both `qta` and `unit_price` are present.

### Main error codes

| HTTP | Code | Meaning |
|---:|---|---|
| 400 | `invalid_payload` | malformed JSON, unknown fields, invalid date/decimal, client-owned totals, multiple JSON docs |
| 400 | `validation_failed` | required business fields missing, invalid ready/prospect/stage/body semantics |
| 400 | `invalid_quote_id` | quote id path param invalid |
| 400 | `invalid_pdf_export_id` | export id path param invalid |
| 400 | `payment_method_not_found` | payment method missing or not selectable |
| 400 | `quote_not_ready` | operation requires ready quote |
| 400 | `hubspot_deal_missing` | stage transition requires deal id |
| 404 | `quote_not_found` | quote not found |
| 404 | `pdf_export_not_found` | PDF export not found |
| 409 | `quote_hubspot_deal_required` | attach requested before quote has HubSpot deal id |
| 409 | `hubspot_stage_conflict` | live remote stage differs from expected stage |
| 502 | `hubspot_request_failed` | upstream HubSpot call failed in synchronous prospect/stage path |
| 502 | `raenad_pdf_generation_failed` | Carbone generation failed during download |
| 503 | `raenad_database_not_configured` | Mistra missing |
| 503 | `raenad_config_not_configured` | Anisetta/runtime config missing/invalid |
| 503 | `alyante_database_not_configured` | Alyante missing |
| 503 | `hubspot_not_configured` | required HubSpot client missing |
| 503 | `raenad_pdf_not_configured` | Carbone dependency/template missing |

Errors are sanitized for clients. Logs include structured context such as component, operation, quote/export/request IDs where available. Persisted worker errors are sanitized/truncated.

---

## 11. Operational notes

### Migrations

Apply manually:

- `deploy/migrations/023_raenad_schema.sql` on Mistra.
- `deploy/migrations/024_mrsmith_hubspot_queue.sql` on Anisetta.

Both are intended to be re-runnable via `IF NOT EXISTS`, `ON CONFLICT DO NOTHING`, and trigger recreation patterns.

### Required per-environment setup

Before end-to-end use:

1. Apply both migrations.
2. Ensure `MISTRA_DSN` and `ANISETTA_DSN` are configured.
3. Ensure `HUBSPOT_API_KEY` is configured for prospect/stage/worker behavior.
4. Ensure `ALYANTE_DSN` is configured for article search.
5. Ensure `CARBONE_API_KEY` is configured for PDF download/attach processing.
6. Insert `mrsmith.runtime_config` row `raenad.carbone_quote_template` with real template id.
7. Verify `raenad.hubspot_deal_pipeline` and `raenad.hubspot_deal_owner` match the target environment.
8. Enable `raenad.hubspot_queue_worker.enabled` only after config is verified.

### Worker operations

- Worker is disabled by default by runtime config.
- Multiple backend instances can run safely; advisory lock ensures one active queue processor per cycle.
- If Carbone is not configured, create/update deal processing can still run, but `hubspot.attach_pdf` requests will fail/defer according to worker handling.
- PDF attach endpoint itself only requires Mistra + ConfigDB and a quote with `hubspot_deal_id`; Carbone/HubSpot are used by the worker, not synchronously by the endpoint.

### Frontend / QA notes

- Use external paths with `/api/aenad/v1/...`.
- Treat `PUT /quotes/{id}` as full save of editable state.
- Do not send totals or line IDs in save payloads.
- Do not expect `hubspot_deal_id` immediately after `POST /quotes`; it is populated by worker after create-deal succeeds.
- Show `hubspot_sync_status` as a technical sync status, not the commercial state.
- Commercial state is HubSpot pipeline/stage.
- Ready quotes demote to draft on relevant business edits; internal notes alone do not demote.
- PDF export and download are separate: export stores immutable payload metadata; download regenerates binary.
- PDF attach is asynchronous; after `202`, poll/list the export revision to observe `hubspot_attachment_status`.

---

## 12. Tests

Backend tests for this implementation are in `backend/internal/raenad/*_test.go` and related HubSpot shared-client tests.

Covered areas include:

- decimal string validation
- actor extraction
- route/auth/dependency guard behavior
- reference endpoints
- quote CRUD, ready validation, ready-to-draft demotion
- store/scan helpers
- HubSpot queue enqueue/coalescing/retry behavior
- worker create/update/attach processing
- prospect contact-first flow
- stage transition conflict handling
- PDF export checksum/stale/reuse/download/attach behavior

Verification commands:

```sh
git diff --check
cd backend && go test ./internal/raenad
cd backend && go test ./...
cd backend && go build ./...
```

Tests use fakes/mocks for HubSpot and Carbone; they do not perform live HubSpot/Carbone calls.

---

## 13. Appendices

### Appendix A — Route matrix

Internal paths below are mounted externally under `/api`.

| Method | Internal path | External path | Dependencies | Purpose |
|---|---|---|---|---|
| GET | `/aenad/v1/quotes/customers` | `/api/aenad/v1/quotes/customers` | Mistra | customer/company selection from HubSpot mirror |
| POST | `/aenad/v1/quotes/prospects` | `/api/aenad/v1/quotes/prospects` | HubSpot | live prospect contact/company flow |
| GET | `/aenad/v1/quotes/payment-methods` | `/api/aenad/v1/quotes/payment-methods` | Mistra | selectable payment methods |
| GET | `/aenad/v1/quotes/stages` | `/api/aenad/v1/quotes/stages` | Mistra + ConfigDB | configured pipeline stages |
| GET | `/aenad/v1/quotes/defaults` | `/api/aenad/v1/quotes/defaults` | ConfigDB | quote line defaults |
| GET | `/aenad/v1/quotes/articles` | `/api/aenad/v1/quotes/articles` | Alyante + ConfigDB | Alyante article search |
| GET | `/aenad/v1/quotes` | `/api/aenad/v1/quotes` | Mistra | paginated quote list |
| POST | `/aenad/v1/quotes` | `/api/aenad/v1/quotes` | Mistra + ConfigDB | create draft quote and enqueue create deal |
| GET | `/aenad/v1/quotes/{id}` | `/api/aenad/v1/quotes/{id}` | Mistra | quote detail |
| PUT | `/aenad/v1/quotes/{id}` | `/api/aenad/v1/quotes/{id}` | Mistra | full save, ready demotion, enqueue update on relevant changes |
| POST | `/aenad/v1/quotes/{id}/ready` | `/api/aenad/v1/quotes/{id}/ready` | Mistra | mark ready after validation |
| POST | `/aenad/v1/quotes/{id}/hubspot/retry` | `/api/aenad/v1/quotes/{id}/hubspot/retry` | Mistra + ConfigDB | retry create/update deal queue |
| POST | `/aenad/v1/quotes/{id}/hubspot/stage` | `/api/aenad/v1/quotes/{id}/hubspot/stage` | Mistra + ConfigDB + HubSpotStage | explicit dealstage transition |
| GET | `/aenad/v1/quotes/{id}/pdf-exports` | `/api/aenad/v1/quotes/{id}/pdf-exports` | Mistra | list export revisions with stale flag |
| POST | `/aenad/v1/quotes/{id}/pdf-exports` | `/api/aenad/v1/quotes/{id}/pdf-exports` | Mistra + ConfigDB + Carbone | create/reuse metadata revision, no binary persistence |
| GET | `/aenad/v1/quotes/{id}/pdf-exports/{exportID}/download` | `/api/aenad/v1/quotes/{id}/pdf-exports/{exportID}/download` | Mistra + ConfigDB + Carbone | regenerate PDF from saved render payload |
| POST | `/aenad/v1/quotes/{id}/pdf-exports/{exportID}/attach` | `/api/aenad/v1/quotes/{id}/pdf-exports/{exportID}/attach` | Mistra + ConfigDB | enqueue async PDF attach |

### Appendix B — Runtime config quick reference

| Namespace | Key | Seeded by migration `024` | Value shape | Notes |
|---|---:|---:|---|---|
| `raenad` | `hubspot_deal_pipeline` | yes | `{"pipeline_id":"...","initial_dealstage_id":"..."}` | required for stages/create/stage transition |
| `raenad` | `hubspot_deal_owner` | yes | `{"fallback_owner_email":"..."}` | required for worker owner fallback |
| `raenad` | `hubspot_queue_worker` | yes | `{"enabled":false,"interval_seconds":30,"batch_size":20,"max_attempts":8,"backoff_base_seconds":30,"backoff_max_seconds":3600}` | optional `lock_timeout_seconds` supported by code, not seeded |
| `aenad` | `quote_defaults` | yes | `{"cod_iva":"22"}` | line/article default VAT code |
| `raenad` | `carbone_quote_template` | no | `{"template_id":"<carbone-template-id>"}` | must be created per environment |

### Appendix C — Migrations

| File | Target DB | Purpose |
|---|---|---|
| `deploy/migrations/023_raenad_schema.sql` | Mistra PostgreSQL | `raenad` quote schema, functions, triggers, immutable PDF exports |
| `deploy/migrations/024_mrsmith_hubspot_queue.sql` | Anisetta PostgreSQL | shared HubSpot queue tables, attempt table, seeded runtime config |

### Appendix D — Repository references

- `docs/IMPLEMENTATION-KNOWLEDGE.md` — durable Raenad implementation knowledge.
- `docs/API-CONVENTIONS.md` — API route conventions.
- `deploy/migrations/023_raenad_schema.sql` — Mistra `raenad` schema, functions, triggers, immutable PDF exports.
- `deploy/migrations/024_mrsmith_hubspot_queue.sql` — Anisetta HubSpot queue tables and runtime config seed.
- `backend/internal/raenad/` — backend implementation package.
- `backend/internal/platform/hubspot/` — shared HubSpot CRM/file/note client support.
- `backend/cmd/server/main.go` — route and worker wiring.

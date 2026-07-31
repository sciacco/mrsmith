# Raenad / Aenad

Knowledge entries specific to the Raenad operational quote schema and the Aenad archive.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### Raenad HubSpot Deal Pipeline Config Lives In Anisetta Runtime Config

- Context: creating HubSpot deals for new Aenad quotes (parent #56).
- Discovery: Aenad V1 uses a single HubSpot pipeline/stage pair for new quote deals: pipeline id `3883934920`, initial deal stage id `5521139911`. This is operational runtime configuration, not raenad quote-domain data. A previous local table idea (`raenad.hubspot_pipeline_config`) was removed before deployment to avoid duplicating HubSpot as a second source of truth.
- Practical rule: read the active deal pipeline config from `mrsmith.runtime_config` on Anisetta, namespace `raenad`, key `hubspot_deal_pipeline`, value `{"pipeline_id": "3883934920", "initial_dealstage_id": "5521139911"}`. When a quote/deal is created, copy the ids actually used into `raenad.quote.hubspot_pipeline_id` and `raenad.quote.hubspot_dealstage_id` as per-quote snapshots.
- Evidence: `deploy/migrations/024_mrsmith_hubspot_queue.sql`, `deploy/migrations/023_raenad_schema.sql`.
- Used by: raenad quote → HubSpot deal creation.

### Raenad Stage Choices Come From The HubSpot Loader Mirror

- Context: user-driven stage transitions for new Aenad quotes (parent #56).
- Discovery: HubSpot pipeline and stage metadata is already mirrored in Mistra `loader.hubs_pipeline` (`id`, `label`) and `loader.hubs_stages` (`id`, `label`, `pipeline`, `display_order`). V1 does not need a local stage mapping table or hardcoded action list: the stages in the configured HubSpot pipeline are already aligned with the app.
- Practical rule: list selectable target stages from `loader.hubs_stages` filtered by the configured `pipeline_id`, ordered by `display_order` and `label`, joining `loader.hubs_pipeline` only when the pipeline label is needed. Treat the mirror as read-only and potentially delayed: writes still go through the backend HubSpot integration. For an Aenad-initiated transition, accept `expected_dealstage_id` and `target_dealstage_id`, verify the target exists in the configured pipeline mirror, fetch the live HubSpot deal, stop with conflict if the live stage differs from `expected_dealstage_id`, otherwise update HubSpot and persist the resulting `hubspot_dealstage_id`/label snapshot on `raenad.quote`.
- Evidence: `docs/mistradb/mistra_loader.json`, existing joins in `backend/internal/rdf/handler.go` and `backend/internal/quotes/handler_reference.go`.
- Used by: raenad stage selector and Aenad-initiated HubSpot stage transition.

### Raenad Deal Owner Maps From User Email To HubSpot Owner

- Context: assigning HubSpot owners when creating deals for new Aenad quotes (parent #56).
- Discovery: HubSpot owners are mirrored in Mistra `loader.hubs_owner` with `id`, `email`, `first_name`, `last_name`, and `archived`. The V1 owner should follow the authenticated Aenad user when possible, with an operational fallback owner configured on Anisetta.
- Practical rule: resolve `hubspot_owner_id` by matching the authenticated user's email case-insensitively against `loader.hubs_owner.email` where `archived = false`. If no active owner matches, read `mrsmith.runtime_config` on Anisetta, namespace `raenad`, key `hubspot_deal_owner`, value `{"fallback_owner_email": "service01@cdlan.it"}`, and resolve that email against active `loader.hubs_owner`. If neither lookup resolves, fail deal creation/sync as misconfigured instead of creating an unowned deal. Send the resolved owner id to HubSpot as the deal owner.
- Evidence: `docs/mistradb/mistra_loader.json`, `deploy/migrations/024_mrsmith_hubspot_queue.sql`.
- Used by: raenad quote → HubSpot deal creation.

### Raenad Deal Create Uses Only Standard HubSpot Properties In V1

- Context: minimum HubSpot deal property mapping for new Aenad quotes (parent #56).
- Discovery: V1 does not require custom mandatory HubSpot properties beyond the agreed standard deal mapping.
- Practical rule: create/update HubSpot deals with `dealname = "{quote_number} - {customer_name}"`, `amount = raenad.quote.total_net`, `closedate = document_date + 30 days`, configured `pipeline`, configured initial `dealstage` on create only, resolved `hubspot_owner_id`, company association, and optional contact association. Ordinary quote-owned updates may refresh name, amount, closedate, owner, and associations, but must not change `dealstage`.
- Evidence: issue #56 grill-me decisions; `deploy/migrations/024_mrsmith_hubspot_queue.sql`.
- Used by: raenad quote → HubSpot deal create/update payloads.

### Raenad UI/UX Planning Is Deferred Until Backend Contracts Are Stable

- Context: planning the new Aenad Preventivi area (parent #56/#61).
- Discovery: the UI/UX needs its own grill-me session and likely child subtickets, but should not be designed before backend API, lifecycle, HubSpot sync, article search, PDF, and attachment contracts are stable.
- Practical rule: defer UI/UX specification and frontend implementation until the backend contract is complete. Track the deferred UI/UX scope in #61; do not make frontend layout or workflow decisions in backend planning tickets beyond preserving necessary API affordances.
- Evidence: issue #56/#61 grill-me decisions.
- Used by: future Aenad Preventivi UI planning and implementation.

### Aenad Quote Line Defaults Live In Anisetta Runtime Config

- Context: creating item lines for new Aenad quotes from Alyante article search (parent #56/#60).
- Discovery: the Alyante V1 article query does not return `cod_iva`, and purchase cost is manual. The V1 default VAT code is `22`, stored as runtime config rather than hardcoded in the frontend.
- Practical rule: read quote-line defaults from `mrsmith.runtime_config` on Anisetta, namespace `aenad`, key `quote_defaults`, value `{"cod_iva": "22"}`. Use this only as an initialization default; users can still edit `cod_iva`, and persisted lines keep both `cod_iva` and `iva_percent_snapshot`.
- Evidence: `deploy/migrations/024_mrsmith_hubspot_queue.sql`, `deploy/migrations/023_raenad_schema.sql`.
- Used by: raenad quote line creation, Alyante product import.

### Raenad Payment Methods Come From loader.erp_metodi_pagamento

- Context: selecting and validating payment method on new Aenad quotes (parent #56).
- Discovery: Mistra loader mirrors ERP payment methods in `loader.erp_metodi_pagamento` with `cod_pagamento`, `desc_pagamento`, and `selezionabile`.
- Practical rule: list/select payment methods from `loader.erp_metodi_pagamento` where `selezionabile = true`, ordered by `desc_pagamento`/`cod_pagamento`. Persist `cod_pagamento` into `raenad.quote.payment_method_code` and `desc_pagamento` into `payment_method_label` as a printable snapshot. `payment_bank_details` remains a quote snapshot field supplied by backend rules or later implementation details; do not infer it from the loader table because the mirror exposes only code, description, and selectability.
- Evidence: `docs/mistradb/mistra_loader.json`, `deploy/migrations/023_raenad_schema.sql`.
- Used by: raenad quote header editing and mark-ready validation.

### Raenad Ready Quotes Return To Draft On Commercial Changes

- Context: authoring lifecycle for new Aenad quotes (parent #56).
- Discovery: `ready` is a manual user action, not an automatic promotion, and relevant edits after `ready` must force the quote back to `draft`.
- Practical rule: when `authoring_status = 'ready'`, changes to customer/company/contact snapshot or HubSpot refs, document header fields (`document_date`, description/title, payment method, bank details), quote lines, quantities, prices, discounts, VAT, purchase cost, or any DB-owned totals must set `authoring_status = 'draft'` and make the latest PDF revision stale. Pure internal notes do not force draft. HubSpot stage changes do not force draft because commercial state remains HubSpot-owned.
- Evidence: issue #56/#58 grill-me decisions; `deploy/migrations/023_raenad_schema.sql`.
- Used by: raenad quote save/update endpoints and PDF stale-state handling.

### Aenad Document Totals Are Database-Owned First-Tranche Calculations

- Context: Aenad document rows and headers in Mistra schema `aenad`.
- Discovery: first-tranche totals are derived from `TDocRighe` numeric row fields and `TIva.PercIva`; `Sconti` is free text but current supported values are percentage chains such as `10%` and `5+3%`.
- Practical rule: compute row purchase as `PrezzoAcquisto * Qta`, net as `PrezzoNetto * Qta` after sequential discounts, gross as net plus VAT from `CodIva -> TIva.PercIva`, and gain as net minus purchase. Do not rewrite `TDocRighe.Sconti`; parse it only for calculation. Header totals are sums of the calculated row fields.
- Evidence: `deploy/migrations/021_aenad_document_totals.sql` and schema documentation in `docs/mistradb/mistra_aenad.json`.
- Used by: `apps/aenad` archive totals and future Aenad write flows.
- Open questions: forced VAT, eco-contributions, withholding, split payment, fidelity, and payment-derived totals are not part of the first tranche.

### Aenad Monetary And Quantity Fields Are numeric(18,4) Exposed As Decimal Strings

- Context: Aenad amounts, prices, quantities, and stock fields in Mistra schema `aenad` (DAZERO source structure).
- Discovery: monetary and quantity columns (`TDocTestate` totals, `TDocRighe.Qta`/`PrezzoNetto`/`ImportoNettoRiga`, `TArticoli` prices, and the rest of the per-table list in `docs/mistradb/mistra_aenad.json`) are `numeric(18,4)`. JSON numbers and Go int64/float64 cannot carry these values losslessly.
- Practical rule: Go reads them with a SQL `::text` cast scanned into `sql.NullString` and exposes them as decimal strings (`*string` in DTOs, `string | null` in frontend API types); writes accept validated decimal strings (dot separator, max 14 integer digits, max 4 decimals) and bind with explicit `$n::numeric(18,4)` casts. Rounding happens only in the database functions, to 4 decimals at function boundaries; the frontend converts to number only for non-persisted preview calculations and display formatting.
- Evidence: `backend/internal/aenad/documents.go` (`normalizeDecimal`, `::text` scans), `deploy/migrations/021_aenad_document_totals.sql`, `apps/aenad/src/utils/format.ts`.
- Used by: `apps/aenad` archive, document detail, and row editing; any future Aenad view exposing fields from the numeric list.
- Open questions: none.

### Aenad Offer Print Semantics: Easyfatt Inline Markers, Spacer Rows, Display-Ready Columns

- Context: replicating the legacy Easyfatt "Offerta" printouts (examples in `artifacts/aenad/`) for Carbone PDF generation from Mistra schema `aenad`.
- Discovery: `TDocRighe.Desc` carries Easyfatt inline formatting markers: `**` toggles bold and `//` toggles italic; markers are often unclosed prefixes that style the rest of the line (`"**SERVIZI UNA TANTUM"`, `"//OPZIONALE"`), closed inline pairs also occur (`"**RICONDIZIONATO** Docking…"`), and descriptions can be multi-line with `\r\n`. No `__` markers or URLs appear in 2025-2026 preventivi. Fully-NULL `TDocRighe` rows are intentional visual spacers between items in the printout (the existing rows endpoint filters them out; a print replica must keep them). `Sconti` is already display-formatted (`"30%"`, `"35+5%"`), and the printed "Iva" column is literally `CodIva` (`"22"`), not `TIva.PercIva`. The printed title comes from `TTipiDoc.TitoloReport` (`'Q'` → `"Offerta"`, while `Nome` is `"Preventivo"`). `TDocTestate.NomeReport` records which Easyfatt report printed the document: `"SHELLI CDLAN offerta"` (full supply-conditions closing block, double signature, clausole 1341-1342) vs `"SHELLI CDLAN offerta noleggio-servizi"` (privacy-only closing); the most common report `"SHELLI preventivo"` is a different layout not covered by the offer template.
- Practical rule: when rendering offer rows for print, fetch ALL rows ordered by `IDDocRiga` without the empty-row filter; convert `Desc` markers to HTML per line (escape HTML first, toggle `<b>` on `**` and `<i>` on `//` skipping `://`, close open tags at end of line, join lines with `<br>`); pass `Sconti` and `CodIva` through verbatim; take the document title from `TTipiDoc.TitoloReport`; drive the conditions-block variant from an explicit caller flag (the legacy equivalent is the operator's report choice, recorded in `NomeReport`).
- Evidence: read-only inspection of docs `IDDoc` 215441/215464/215564 (Num 879/901/994, matching `artifacts/aenad/` PDFs); `docs/mistradb/mistra_aenad.json`.
- Used by: aenad offer PDF generation (Carbone template + `backend/internal/aenad` PDF endpoint).
- Open questions: `QtaShown` vs `Qta` divergence never observed — `Qta` is used.

### Raenad Is The Operational Quote Schema, Separate From The Aenad Archive

- Context: new Aenad quote-creation flow (parent #56); the existing `aenad` schema stays the read-only historical archive.
- Discovery: new quotes live in a dedicated lowercase-only Mistra schema `raenad` (`quote`, `quote_line`, `quote_pdf_export`, `quote_event`), not as an extension of legacy mixed-case `aenad`. Totals are DB-owned, replicating the verified `aenad` 021 calc logic (sequential `discount_multiplier`, `line_net`, header recalc triggers with a `raenad.skip_header_recalc` guard and a bulk `raenad.recalculate_quote_totals` procedure). Money/quantity are `numeric(18,4)`; non-economic rows (`line_type` `spacer`/`description`) leave all economic columns NULL.
- Practical rule: build the new quote flow against `raenad`; keep object/column names lowercase; reuse the aenad calc shape (do not invent a new discount/VAT algorithm); expose `numeric(18,4)` as decimal strings over the API exactly as the aenad archive does.
- Evidence: `deploy/migrations/023_raenad_schema.sql`, `deploy/migrations/021_aenad_document_totals.sql`.
- Used by: raenad quote authoring; future raenad backend/API/UI.
- Open questions: none for the schema; lifecycle/state mapping is a later #56 sub-phase.

### Raenad Quote Numbers Use common.new_document_number('AE-')

- Context: numbering new raenad quotes.
- Discovery: `quote_number` defaults to `common.new_document_number('AE-')`, which draws from the shared global `common.document_seq` and returns `AE-<n>/<YYYY>` (e.g. `AE-1003/2026`). The sequence is shared across document types, so numbers are globally unique but not per-prefix contiguous.
- Practical rule: let the database assign the number via the column default (NOT NULL UNIQUE); do not implement an app-side counter or assume gap-free `AE-` numbering. Use the same `common.new_document_number(prefix)` helper for any future document type.
- Evidence: `deploy/migrations/023_raenad_schema.sql`, `common.new_document_number` / `common.document_seq` in `docs/mistradb/mistra_common.json`.
- Used by: raenad quote creation.
- Open questions: none.

### Raenad Stores A Printable Customer/Contact Snapshot Decoupled From The HubSpot Mirror

- Context: persisting customer/contact data on a raenad quote for stable PDFs/exports.
- Discovery: `raenad.quote` keeps a full printable snapshot of customer (`customer_*`, `numero_azienda_snapshot`) and contact (`contact_first_name/_last_name/_full_name/_email/_role`) alongside HubSpot reference ids. There is no physical FK to `loader.hubs_company`/`loader.hubs_contact` because that mirror can lag behind HubSpot writes; `contact_full_name` is retained even though derivable, and the contact mirror only exposes `id/firstname/lastname/email` (no phone in V1).
- Practical rule: copy a customer/contact snapshot onto the quote at authoring time and render PDFs/exports from the snapshot, not from a live mirror join; treat `numero_azienda_snapshot` as the Alyante ERP id captured at that moment. Keep HubSpot ids as text reference fields, not FKs.
- Evidence: `deploy/migrations/023_raenad_schema.sql`; mirror columns in `docs/mistradb/mistra_loader.json`.
- Used by: raenad quote authoring, PDF/export rendering.
- Open questions: none for V1.

### Raenad VAT Is cod_iva Plus A Persisted iva_percent_snapshot

- Context: VAT calculation on raenad quote lines (V1: standard calculation only).
- Discovery: each item line stores both `cod_iva` (the printable code) and `iva_percent_snapshot` (numeric). The line trigger uses `iva_percent_snapshot` when present and only falls back to resolving it from `aenad."TIva".PercIva` via `cod_iva` when NULL — so `aenad."TIva"` remains the shared VAT master on the same Mistra DB (no VAT table is copied into `raenad`). `raenad` models VAT explicitly: `line_vat = round(line_net * pct/100, 4)`, `line_gross = line_net + line_vat` (a deliberate, minor rounding-order difference from aenad's `gross = net*(1+pct)`).
- Practical rule: persist both `cod_iva` and `iva_percent_snapshot` so exports stay stable if the VAT master changes later; resolve the snapshot once at calc time. Forced VAT, eco-contributi, and ritenute are out of V1.
- Evidence: `deploy/migrations/023_raenad_schema.sql`, `aenad."TIva"` in `docs/mistradb/mistra_aenad.json`.
- Used by: raenad line/header totals, PDF/export.
- Open questions: none for V1.

### Raenad PDF Exports Are Immutable Revisions

- Context: persisting generated PDFs and attaching them to HubSpot deals.
- Discovery: `raenad.quote_pdf_export` rows are immutable revisions, unique on `(quote_id, revision)`. A BEFORE UPDATE trigger blocks changes to the content columns (`revision`, `filename`, `content_type`, `checksum_sha256`, `render_payload`, `created_at`, `created_by`); only the `hubspot_*` attachment-status fields are mutable. V1 uses a dedicated raenad PDF template configured by `mrsmith.runtime_config` key `raenad.carbone_quote_template` with value shape `{"template_id":"<id>"}`; it does not reuse or fall back to the legacy `aenad.carbone_offerta` template. The PDF binary is not persisted in V1: the document is reproducible through Carbone from the dedicated template plus the saved `render_payload` and checksum metadata. Manual HubSpot attachment is queue-scoped to the export revision: `operation='hubspot.attach_pdf'`, `entity_type='raenad.quote_pdf_export'`, `entity_id=<export_id>`, and dedupe key `raenad:quote:{quote_id}:pdf-export:{export_id}:attach`.
- Practical rule: never overwrite an export row; create a new revision instead. Store metadata, checksum, and `render_payload`, not PDF bytes. PDF create/download require the dedicated template config and return `raenad_pdf_not_configured` when it is missing or invalid. Attachment enqueue only requires Mistra plus the queue/config DB and does not call Carbone or HubSpot synchronously; Carbone, HubSpot upload, note creation, and retry/dead handling belong to the worker. The worker regenerates from the saved `render_payload`, uploads privately under `/mrsmith/raenad/quote-pdfs`, creates a deal-associated note, and updates only `hubspot_attachment_status` plus the other `hubspot_*` fields, preserving export history and quote sync state.
- Evidence: `deploy/migrations/023_raenad_schema.sql`, `deploy/migrations/024_mrsmith_hubspot_queue.sql`, `backend/internal/raenad/pdf_export.go`, `backend/internal/raenad/hubspot_queue.go`, `backend/internal/raenad/hubspot_worker.go`.
- Used by: raenad PDF generation and HubSpot deal attachment.
- Open questions: the concrete Carbone template asset/id still must be supplied in runtime config per environment.

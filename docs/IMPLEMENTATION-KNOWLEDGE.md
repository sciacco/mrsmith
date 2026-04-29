# Implementation Knowledge Handbook

This document is the canonical handbook for reusable implementation knowledge discovered while building apps in this repo.

Use it to capture facts that are easy to rediscover badly and expensive to relearn later: identifier mappings, cross-system joins, hidden business rules, exclusions, legacy quirks, API/DB mismatches, and operational conventions that affect future implementations.
For Appsmith migrations, use [docs/APPSMITH-MIGRATION-PLAYBOOK.md](APPSMITH-MIGRATION-PLAYBOOK.md) first to extract and pin verified contracts, then record any reusable discoveries here.

## How to Use This Document

- Read the relevant sections before planning a new app, integration, or cross-system feature.
- Update this document in the same change set when implementation work uncovers reusable knowledge that other apps are likely to need.
- Prefer adding curated entries under a stable domain section instead of app-specific notes or a chronological dump.
- Keep each entry actionable: describe the fact, the practical rule it implies, the evidence, and where it matters.

## Entry Format

Use this format for new knowledge entries:

### Entry Title

- Context: where this knowledge applies
- Discovery: the fact that was verified
- Practical rule: how future implementations should use it
- Evidence: source tables, specs, code paths, or repo docs
- Used by: apps or domains already depending on it
- Open questions: only if unresolved details remain

## Domains

Add new discoveries under the most relevant domain:

- Cross-system identity and keys
- Customer eligibility and exclusion rules
- API and backend contract quirks
- Legacy data model constraints
- Auth and transport behavior
- Deployment and runtime integration rules

## Cross-System Identity and Keys

### Customer Identity Across Systems

- Context: customer lookup, filtering, and joins across Alyante, Mistra, and Grappa.
- Discovery: the same customer is represented with different keys across systems. Alyante ERP ID is the shared business identifier. In Mistra, `customers.customer.id` stores that ERP ID directly. In Grappa, `cli_fatturazione.codice_aggancio_gest` stores the ERP ID, while `cli_fatturazione.id` is a separate internal Grappa identifier.
- Practical rule: when moving from Grappa data to Mistra data, use `cli_fatturazione.codice_aggancio_gest -> customers.customer.id`. Do not assume `cli_fatturazione.id` matches Mistra customer IDs.
- Evidence: `customers.customer`, `loader.erp_clienti_provenienza`, `cli_fatturazione`; prior analysis captured in the legacy cross-db identity note.
- Used by: customer selectors and pricing/credit flows in `apps/listini-e-sconti`.
- Open questions: none on the identifier mapping itself.

#### Systems Involved

| System | Database | Main table | Primary key meaning |
| --- | --- | --- | --- |
| Alyante | — | — | ERP company ID |
| Mistra | PostgreSQL | `customers.customer` | `id` = Alyante ERP ID |
| Grappa | MySQL | `cli_fatturazione` | `id` = internal Grappa ID |

#### Key Mapping

```text
Alyante ERP ID
    |
    ├── Mistra PG:  customers.customer.id
    |
    └── Grappa MySQL: cli_fatturazione.codice_aggancio_gest
                      cli_fatturazione.id                    (internal Grappa ID)
```

#### ERP Bridge in Mistra

- Context: filtering customers eligible for billing-related flows.
- Discovery: `loader.erp_clienti_provenienza.numero_azienda` links back to `customers.customer.id`, and `fatgamma > 0` marks a customer as active for billing.
- Practical rule: when a flow needs ERP-linked or billing-eligible customers in Mistra, join through `loader.erp_clienti_provenienza` and treat `fatgamma > 0` as the current eligibility signal unless product requirements say otherwise.
- Evidence: `loader.erp_clienti_provenienza.numero_azienda`, `loader.erp_clienti_provenienza.fatgamma`.
- Used by: customer list variants described in `apps/listini-e-sconti/listini-e-sconti-migspec-phaseA.md`.
- Open questions: confirm with the domain team whether `fatgamma > 0` is the durable business rule or a current operational shortcut.

### HubSpot Company Lookup from Grappa

- Context: audit trail flows that create HubSpot notes/tasks after pricing, credit, or discount changes.
- Discovery: the Grappa customer ID must be resolved to a HubSpot company ID via a two-step cross-database lookup:
  1. Grappa → ERP ID: `SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = :grappa_id` (Grappa MySQL)
  2. ERP ID → HubSpot ID: `SELECT id FROM loader.hubs_company WHERE numero_azienda = :erp_id::varchar` (Mistra PG)
- Practical rule: backend services that need to write to HubSpot from a Grappa context must query both databases sequentially. Cache the mapping if performance is a concern — the mapping changes infrequently.
- Evidence: Appsmith `HS_utils` module method `CompanyByGrappaId`, queries `get_erp_id` and `get_hubspot_id_by_erp_code`.
- Used by: IaaS Prezzi risorse, IaaS Credito omaggio, Sconti variabile energia (all in `apps/listini-e-sconti`).
- Open questions: none.

## Customer Eligibility and Exclusion Rules

### Known Grappa Customer Exclusions

- Context: customer selectors used by IaaS pricing and credit pages.
- Discovery: some flows explicitly exclude specific `cli_fatturazione.codice_aggancio_gest` values.
- Practical rule: do not silently generalize active-billing customer selectors across pages; verify whether exclusion codes must be preserved for that use case.
- Evidence: current documented exclusions from existing migration analysis.
- Used by: IaaS Prezzi risorse, IaaS Credito omaggio.
- Open questions: whether these exclusions are permanent business rules or should become configurable.

| Code | Excluded in |
| --- | --- |
| `385` | IaaS Prezzi risorse, IaaS Credito omaggio |
| `485` | IaaS Credito omaggio |

## API and Backend Contract Quirks

### Fornitori Provider Contacts Follow Appsmith Payload Semantics

- Context: `apps/fornitori` provider detail contacts and backend `POST/PUT /fornitori/v1/provider/{id}/reference`.
- Discovery: Appsmith does not send empty `first_name`, `last_name`, or `email` fields for provider contacts. On reference create, empty `phone` must be omitted because Mistra `provider_ref_new` inserts it directly and the `provider_ref` phone check accepts `NULL` but not `''`; on reference edit, empty `phone` must be sent as `''` because `provider_ref_edit` converts it with `NULLIF` and uses key presence to clear the value. `QUALIFICATION_REF` is also a special reference type: Mistra's standard provider-reference functions reject creating, editing, or deleting it.
- Practical rule: Fornitori contact forms should omit empty name/email fields. The backend `/provider/{id}/reference` proxy must omit empty `phone` on `POST`, include `phone` on `PUT` even when empty, be used **only** for non-qualification reference types (`ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `OTHER_REF`), and reject `QUALIFICATION_REF` with a clear error. The QUALIFICATION_REF contact is owned by Mistra and is created/edited via `PUT /provider/{id}` (the `ref` field of `provider-edit`); never write to `provider_qualifications.provider_ref` directly from the portal backend.
- Evidence: Appsmith contact-save snippet provided during the Fornitori migration; `docs/mistra-dist.yaml` provider-ref schemas; `docs/arak_schema.json` functions `provider_ref_new` and `provider_ref_edit`.
- Used by: `apps/fornitori` detail page contacts.
- Open questions: none.

### RDA New Supplier Requests Must Use Provider Draft Create

- Context: `apps/rda` supplier creation from `/rda/new` and any RDA inline new-provider form.
- Discovery: Mistra `POST /arak/provider-qualification/v1/provider` follows the full provider-create schema and requires `default_payment_method`, while the RDA flow only requests a supplier census draft. The legacy RDA datasource used `POST /arak/provider-qualification/v1/provider/draft`.
- Practical rule: RDA must create new suppliers through `POST /api/fornitori/v1/provider/draft`, not the full `POST /api/fornitori/v1/provider`. Keep full provider create for the Fornitori app where users can manage complete provider records.
- Evidence: `docs/mistra-dist.yaml` schemas `provider-new` and `provider-draft-new`; `docs/arak_schema.json` functions `provider_new` and `provider_draft_new`; RDA audit datasource `nuovoFornitore`.
- Used by: `apps/rda` `/rda/new` supplier request modal and legacy inline new-provider form.
- Open questions: none.

### RDA Payment Method Standard Rule

- Context: `apps/rda` PO create/edit flows and backend `POST/PATCH /api/rda/v1/pos`.
- Discovery: the standard payment methods for an RDA PO are the selected supplier default and the CDLAN default from `provider_qualifications.payment_method_default_cdlan`. Other payment methods are valid only when the catalog row exists and `provider_qualifications.payment_method.rda_available` is true.
- Practical rule: show supplier default first, CDLAN default second, then RDA-available methods. Warn users only when the selected method is neither supplier default nor CDLAN default. Backend create/patch must validate the effective payment code against `provider_qualifications.payment_method`, allowing a non-RDA method only when it matches the selected supplier default.
- Evidence: legacy RDA audit payment option rule, `provider_qualifications.payment_method`, `provider_qualifications.payment_method_default_cdlan`, and backend validation in `backend/internal/rda`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, and RDA backend PO create/patch validation.
- Open questions: none.

### RDA Currency Is A PO-Level Display Contract

- Context: `apps/rda` PO create/edit flows, PO lists, inboxes, row tables, and attachment threshold messaging.
- Discovery: RDA currency belongs to the PO header (`currency` on Mistra RDA create/detail/preview/patch), not to rows. It selects the displayed money symbol for PO and row amounts; it does not convert values, recalculate totals, or change row payloads.
- Practical rule: persist only `EUR`, `USD`, or `GBP` at PO level, default existing/missing values to `EUR`, and pass the PO currency into all money formatting. Keep numeric thresholds, row economics, and Mistra row create/replace payloads unchanged.
- Evidence: `docs/mistra-dist.yaml` RDA `currency` fields; implementation in `apps/rda/src/lib/format.ts`, `apps/rda/src/lib/po-payload.ts`, and `backend/internal/rda`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, PO lists, inboxes, row composer, and RDA backend PO create/patch.
- Open questions: none.

### RDA Attachment Type Is User-Selected At Upload

- Context: `apps/rda` PO attachment uploads and draft submit validation.
- Discovery: Mistra `POST /arak/rda/v1/po/{id}/attachment` requires multipart `attachment_type` with enum `quote`, `transport_document`, or `other`. RDA users need to choose that type during upload; the 3.000 euro submit threshold is a quote rule, not a generic attachment count.
- Practical rule: the RDA frontend should send the selected `attachment_type` with every uploaded file. The BFF must validate the enum and forward it to Mistra, keeping the legacy fallback only when old clients omit the field (`DRAFT -> quote`, `PENDING_VERIFICATION -> transport_document`). For `total_price >= 3000`, submit requires at least two attachments with `attachment_type == quote`.
- Evidence: `docs/mistra-dist.yaml` schema `po-attachment-upload`; RDA backend `handleUploadAttachment` and `handleSubmitPO`; frontend attachment helper in `apps/rda/src/lib/attachments.ts`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, and RDA backend attachment upload/submit validation.
- Open questions: none.

### RDA Patch Payload Null Semantics

- Context: `apps/rda` PO header edits forwarded to Mistra `rda-patch`.
- Discovery: Mistra rejects `budget_user_id: null` with `Value is not nullable`; in `rda-patch`, only `cost_center` is nullable. Optional text fields such as `description`, `note`, and `provider_offer_code` are strings, and `provider_offer_date` must be a valid date when present.
- Practical rule: for RDA PATCH payloads, omit `budget_user_id` for cost-center budgets, send `cost_center: null` only when switching to a user budget, and send optional text fields as strings rather than `null`. Omit an empty `provider_offer_date` instead of sending `null` or an invalid empty date.
- Evidence: `docs/mistra-dist.yaml` schema `rda-patch`; observed Mistra 400 response from `PATCH /rda/v1/pos/{id}`; implementation in `apps/rda/src/lib/po-payload.ts`.
- Used by: `apps/rda` PO detail and new-RDA wizard header save flows.
- Open questions: whether Mistra exposes a supported way to clear an existing `provider_offer_date`; the current safe behavior omits empty dates on PATCH.

### RDA Row Totals Are Normalized By The BFF

- Context: `apps/rda` PO row tables in the new wizard and PO detail page.
- Discovery: Mistra exposes the PO aggregate `total_price`, but row responses may not include a useful `total_price` per row. Legacy Appsmith displayed a computed row total instead.
- Practical rule: `GET /api/rda/v1/pos/{id}` should normalize every row with `price` for goods and `total_price` before returning it to the frontend. Preserve a positive upstream `total_price` or `total` when present; otherwise calculate goods as `price * qty` and services as `(MRC * qty * initial_subscription_months) + (NRC * qty)`, using read names `montly_fee` and `activation_fee`. Because Arak's detail function may omit the stored good `price`, enrich rows from `rda.purchase_order_row` when available.
- Evidence: `docs/mistra-dist.yaml` `rda-row`; `docs/arak_schema.json` functions `get_purchase_order_rows_detail` and `create_purchase_order_row`; Appsmith audit row-total notes; backend `backend/internal/rda/row_totals.go`.
- Used by: `apps/rda` row tables and wizard row composer.
- Open questions: none.

### RDA Good Row Create Still Needs Empty Renew Detail

- Context: backend `POST /api/rda/v1/pos/{id}/rows` for RDA goods.
- Discovery: Mistra's `rda-row-create` schema requires `renew_detail` at the root even for `type=good`, although no renew fields are meaningful for goods. Omitting it returns `400` with `property "renew_detail" is missing`.
- Practical rule: when creating good rows, forward `renew_detail: {}` along with `price` and the calculated `total`. Do not include service-only renewal fields for goods. Send calculated `total` for service rows too, because Arak stores it in `purchase_order_row.total` and the PO aggregate sums that column.
- Evidence: Mistra schema `docs/mistra-dist.yaml` `rda-row-create`; `docs/arak_schema.json` functions `create_purchase_order_row` and `purchase_order_total_calculation`; observed upstream 400 during PO row create; backend `backend/internal/rda/validation.go`.
- Used by: `apps/rda` row composer and row modal.
- Open questions: none.

### RDA Row Edit Is A BFF Replace Operation

- Context: `apps/rda` PO row editing in `/rda/new` and PO detail.
- Discovery: Mistra exposes row create and delete (`POST /arak/rda/v1/po/{id}/row`, `DELETE /arak/rda/v1/po/{id}/row/{rowid}`) but no real row update endpoint.
- Practical rule: expose row edit to the browser only through `PUT /api/rda/v1/pos/{id}/rows/{rowId}`. The BFF must validate the caller and draft PO, create the replacement row first, and delete the old row only after create succeeds. If delete fails after create, return `409 ROW_REPLACE_DELETE_FAILED` so the UI refetches and shows the operator that both rows may be present.
- Evidence: `docs/mistra-dist.yaml` RDA row paths and backend `backend/internal/rda/validation.go`.
- Used by: `apps/rda` row modal and row tables.
- Open questions: none.

### RDA Article Catalog Type Comes From The BFF

- Context: `apps/rda` row creation in `/rda/new` and PO detail row modal.
- Discovery: Mistra exposes RDA articles filtered by `type=good|service`, but the row UI must select an article first and derive the row type from that selected catalog item.
- Practical rule: the RDA row picker should load the unified catalog once with `GET /api/rda/v1/articles`, then filter locally by code, description, and the Italian type labels (`bene` / `servizio`). The RDA BFF fetches both good and service catalogs when `type` is omitted, normalizes every item to `{code, description, type}`, and deduplicates by `code`. Keep `?search=...` and `?type=good|service` only for compatibility or deliberately typed consumers.
- Evidence: backend `backend/internal/rda/articles.go`; frontend `apps/rda/src/api/queries.ts`, `apps/rda/src/components/ArticleCombobox.tsx`, and `apps/rda/src/lib/row-payload.ts`.
- Used by: `apps/rda` `/rda/new` row composer and PO detail row modal.
- Open questions: none.

### Manutenzioni Radar Excludes Terminal Maintenance States

- Context: `GET /api/manutenzioni/v1/maintenances/radar`, used by `apps/manutenzioni` on the Registro Manutenzioni page.
- Discovery: maintenance records with status `cancelled` or `superseded` are lifecycle history, not actionable operational windows. Counting them in the radar makes the upcoming-window buckets noisy and misleading.
- Practical rule: Manutenzioni radar-style operational summaries must always exclude `cancelled` and `superseded`, even when the caller passes explicit status filters. Keep the full register/list endpoints available for searching those terminal records.
- Evidence: `backend/internal/manutenzioni/read.go` `handleMaintenanceRadar`; regression coverage in `backend/internal/manutenzioni/radar_test.go`.
- Used by: `apps/manutenzioni` Registro Manutenzioni radar.
- Open questions: none.

### Cross-Database Mini-App Summaries Must Merge In Code, Not In One SQL Join

- Context: mini-apps that read business records from one DB and enrich them with replica/loader data from another DB in the same request path.
- Discovery: the MrSmith backend wires `ANISETTA_DSN`, `MISTRA_DSN`, `GRAPPA_DSN`, and other stores as separate `*sql.DB` handles. A handler cannot issue a single SQL statement that joins tables across those DSN boundaries. `apps/richieste-fattibilita` hit this when `rdf_richieste` (Anisetta) needed HubSpot deal/company enrichment from `loader.hubs_*` (Mistra).
- Practical rule: when a screen needs cross-DB enrichment, fetch the base rows from the owning DB, batch-load enrichment rows from the secondary DB, merge in Go, and only then apply filters that depend on enriched fields (for example customer/company name filters). Do not plan a “server-side join” as a single SQL query unless the data is confirmed to live behind the same connection.
- Evidence: separate DB wiring in `backend/cmd/server/main.go`; merge implementation in `backend/internal/rdf/handler.go` for `GET /rdf/v1/richieste/summary`.
- Used by: `apps/richieste-fattibilita`.
- Open questions: none.

### Quotes Create Flow Uses Context-Specific Category Exclusions

- Context: `apps/quotes` service-category loading for Nuova Proposta versus other quotes views.
- Discovery: the Appsmith Nuova Proposta `get_product_category` query excludes only category ids `12` and `13`, while other quotes references and later repo specs may exclude `12,13,14,15`. A single hardcoded "standard" exclusion set caused QA drift in the create flow.
- Practical rule: quotes category endpoints should support context-specific exclusions instead of assuming one global standard-flow filter. For the create wizard, pass explicit excluded ids and keep the filtering contract in the request, not hidden in the frontend.
- Evidence: `apps/quotes/check/out_08.md`, `apps/quotes/src/api/queries.ts` `useCategories`, `backend/internal/quotes/handler_reference.go` `exclude_ids` support.
- Used by: `apps/quotes` Nuova Proposta wizard.
- Open questions: if another quotes surface needs the broader `12,13,14,15` exclusion, keep that as an explicit caller decision rather than reusing the create-flow contract.

### Quotes IaaS Template Derivation Must Be DB-Driven

- Context: `apps/quotes` Nuova Proposta IaaS path and `POST /quotes/v1/quotes`.
- Discovery: hardcoded frontend template-ID maps can drift from `quotes.template` and produce dead-end create flows where no kit is derivable, even when template metadata exists in DB.
- Practical rule: derive IaaS kit/services from `quotes.template` (`template_type`, `kit_id`, `service_category_id`) and treat DB metadata as the single source of truth. For template-linked kits, bypass standard catalog eligibility (`is_active/ecommerce/quotable`) when resolving the kit; backend create must still reject missing template kit IDs or non-existent kit IDs.
- Evidence: `apps/quotes/src/pages/QuoteCreatePage.tsx`, `apps/quotes/src/api/queries.ts` (`include_ids`), `backend/internal/quotes/handler_reference.go` (`include_ids` merge), `backend/internal/quotes/handler_quotes.go`, `backend/internal/quotes/handler_create_test.go`.
- Used by: `apps/quotes` create wizard and create endpoint validation.
- Open questions: none.

### Quotes Replacement Orders Need Appsmith Column Names Plus Customer Scoping

- Context: `SOSTITUZIONE` order pickers in quotes create/detail flows.
- Discovery: the Appsmith dataset shape comes from Alyante `Tsmi_Ordini.NOME_TESTATA_ORDINE` with `STATO_ORDINE IN ('Evaso', 'Confermato')`; in this Alyante schema the customer scope column is `ID_CLIENTE`, not `NUMERO_AZIENDA`.
- Practical rule: when loading replacement-order options, query `NOME_TESTATA_ORDINE`, keep the `STATO_ORDINE` filter, and scope orders by the resolved ERP customer via `loader.hubs_company.numero_azienda -> Tsmi_Ordini.ID_CLIENTE`.
- Evidence: `apps/quotes/check/out_07.md`, `backend/internal/quotes/handler_reference.go` `customerOrdersQuery`, `backend/internal/quotes/handler_reference_test.go`.
- Used by: `apps/quotes` create and detail replacement-order selectors.
- Open questions: none.

### Quotes Publish Payment Labels Use Loader ERP Column Names

- Context: quotes publish orchestration when generating HubSpot terms and conditions.
- Discovery: payment-method labels must be read from `loader.erp_metodi_pagamento.desc_pagamento` keyed by `cod_pagamento`; older aliases `descrizione` / `codice` are wrong for this schema and broke the publish path.
- Practical rule: any quotes publish or save logic that needs the payment-method label should use `cod_pagamento` / `desc_pagamento`, and backend tests should pin those column names because similar stale aliases have already regressed once.
- Evidence: `backend/internal/quotes/handler_publish.go` `paymentMethodLabelQuery`, `backend/internal/quotes/handler_publish_test.go`, `apps/quotes/check/fix_QA.md`.
- Used by: `apps/quotes` publish flow.
- Open questions: none.

### Quotes Deal Number Must Come From HubSpot `codice`, Not Deal Title

- Context: `apps/quotes` Nuova Proposta deal picker, quote creation payload, and detail header rendering.
- Discovery: `quotes.quote.deal_number` is the HubSpot deal code, while `loader.hubs_deal.name` is the human title. Reusing `d.name` in the wizard create payload stores the title in `deal_number`, which breaks downstream views that expect the code.
- Practical rule: quotes deal reference APIs should expose both `name` and `deal_number` (`loader.hubs_deal.codice`), wizard search should include the code, and quote create should persist `selectedDeal.deal_number`, never the title.
- Evidence: `backend/internal/quotes/handler_reference.go`, `apps/quotes/src/pages/QuoteCreatePage.tsx`, `apps/quotes/src/components/HeaderTab.tsx`, production quotes `1373` and `1374` created on 2026-04-12 with `deal_number` incorrectly set to `TEST ALESSANDRA - NON ELIMINARE`.
- Used by: `apps/quotes` deal list, create flow, and detail header.
- Open questions: whether to add a separate backfill for already-corrupted `quotes.quote.deal_number` rows.

### Quotes Customer Default Payment Must Use Alyante `CODICE_PAGAMENTO`

- Context: quotes create enrichment endpoint `GET /quotes/v1/customer-payment/{customerId}` against Alyante `Tsmi_Anagrafiche_clienti`.
- Discovery: this Alyante environment exposes the customer default payment as `CODICE_PAGAMENTO`; the stale alias `AN_CONDPAG` is invalid and causes `mssql: Invalid column name 'AN_CONDPAG'`. The legacy Appsmith contract already used `ISNULL(CAST(CODICE_PAGAMENTO as INT), 402)`.
- Practical rule: any quotes customer-payment lookup should query `CODICE_PAGAMENTO` and preserve the `402` fallback semantics in SQL or equivalent null-safe backend logic. Keep a backend test that pins that positive contract.
- Evidence: `apps/quotes/quotes-migspec-phaseA.md`, `apps/quotes/APPSMITH-AUDIT.md`, `backend/internal/quotes/handler_reference.go`, `backend/internal/quotes/handler_reference_test.go`.
- Used by: `apps/quotes` create flow payment-method prefill.
- Open questions: none.

### Quotes Republish Must Unlock Published HubSpot Quotes First

- Context: republishing an existing HubSpot-backed quote from `apps/quotes`.
- Discovery: published HubSpot quotes are locked (`hs_locked=true`) and reject direct property updates with `Published Quote cannot be edited`. The legacy Appsmith `Dettaglio.mainForm.mandaSuHubspot()` flow explicitly PATCHed `hs_status=DRAFT` before syncing changes, and HubSpot's legacy quotes docs require moving published quotes back to `DRAFT`, `PENDING_APPROVAL`, or `REJECTED` before editing.
- Practical rule: any republish/update flow for an existing HubSpot quote must fetch live quote status first and, if the quote is locked or already in a published state (`APPROVED` / `APPROVAL_NOT_NEEDED`), unlock it with `hs_status=DRAFT` before updating properties or line items. Do not rely only on the local DB status.
- Evidence: `apps/quotes/quotes-main.tar.gz` -> `quotes-main/pages/Dettaglio/jsobjects/mainForm/mainForm.js`, `backend/internal/platform/hubspot/quotes.go`, `backend/internal/quotes/handler_publish.go`, HubSpot legacy quotes docs ("Properties set by quote state", last modified 2026-03-30).
- Used by: `apps/quotes` republish flow and `GET /quotes/v1/quotes/:id/hs-status`.
- Open questions: none.

### Panoramica Orders Summary Text Columns Can Be NULL

- Context: `GET /api/panoramica/v1/orders/summary` backed by `loader.v_ordini_sintesi`.
- Discovery: production data can return `NULL` for multiple `loader.v_ordini_sintesi` text fields used by the summary endpoint, including `stato` and `numero_ordine`, even though the original backend/frontend contract modeled them as required strings.
- Practical rule: scan summary text columns with `sql.NullString` in backend handlers and normalize them deliberately before JSON encoding; do not scan those columns directly into Go `string` fields.
- Evidence: backend failures `list_orders_summary_scan` on 2026-04-09 for `stato` and `numero_ordine` (`converting NULL to string is unsupported`), fixed in `backend/internal/panoramica/handler_orders.go`.
- Used by: `apps/panoramica-cliente` recurring orders summary view.
- Open questions: whether the frontend contract should eventually widen affected summary text fields to `string | null` instead of preserving empty-string fallbacks.

### RDF `fornitori_preferiti` Must Be Treated as Nullable Text

- Context: `GET /api/rdf/v1/richieste/summary`, `GET /api/rdf/v1/richieste/{id}/full`, and any RDF flow that reads `public.rdf_richieste.fornitori_preferiti`.
- Discovery: the authoritative schema snapshot marks `rdf_richieste.fornitori_preferiti` as nullable `text` with default `''`, so production rows can legitimately contain `NULL`. Scanning that column directly into Go `string` fields causes runtime failures (`converting NULL to string is unsupported`).
- Practical rule: scan `fornitori_preferiti` with `sql.NullString` in RDF handlers and normalize `NULL`, `''`, and empty array literals to `[]` before encoding JSON. Keep the API contract as `number[]`; do not surface `null` to the frontend for this field.
- Evidence: `docs/anisetta_schema.json` (`rdf_richieste.fornitori_preferiti` `nullable: true`), backend failure `list_richieste_summary_scan` on 2026-04-16, and fixes in `backend/internal/rdf/handler.go`.
- Used by: `apps/richieste-fattibilita` summary/detail flows and manager actions that reload a richiesta after writes.
- Open questions: none.

### Loader `quantita` Must Be Treated as Decimal (Nullable) Across Reports and Panoramica

- Context: report/order endpoints reading `quantita` from loader views such as `v_ordini_ric_spot`, `v_ordini_sintesi`, and `v_ordini_ricorrenti_conrinnovo`.
- Discovery: `quantita` is defined as `double precision` in Mistra loader view contracts and can be fractional (for example `7.5`) and nullable. Scanning into Go `int`/`sql.NullInt64` causes runtime failures (`Scan error ... converting driver.Value type float64 ... to int`) and/or truncation risk.
- Practical rule: for loader-backed APIs, scan `quantita` with `sql.NullFloat64` and expose it as nullable decimal in JSON/TS contracts (`*float64` in Go responses, `number | null` in TS). Do not cast/round to int unless an explicit business rule requires integer quantities.
- Evidence: `docs/mistradb/mistra_loader.json` (`quantita` column type `double precision` on loader views), backend failure on `POST /api/reports/v1/orders/preview` on 2026-04-14 with value `7.5`, and follow-up fixes in reports/panoramica handlers.
- Used by: `apps/reports` (`orders`, `active-lines`, `pending-activations`, `upcoming-renewals`) and `apps/panoramica-cliente` (`orders/summary`, `orders/detail`).
- Open questions: none.

### Reports Carbone Export Payloads May Need Template-Specific Key Aliases

- Context: XLSX exports in `backend/internal/reports` rendered through Carbone templates.
- Discovery: Carbone export payload keys do not have to match the preview API contract exactly. `Accessi attivi` preview still exposes `stato`, but the XLSX template expects Grappa-specific aliases, so the backend now rewrites the export payload to emit `stato grappa` and `stato_grappa` instead of `stato`.
- Practical rule: when a Carbone template is already pinned to legacy field names, adapt the backend export payload in the export path only; do not widen or rename the preview API/frontend contract unless the UI actually needs the new keys too.
- Evidence: `backend/internal/reports/handler_accessi.go` `activeLinesExportRows`, `backend/internal/reports/handler_quantita_test.go`, reports template references `AccessiTemplateID = a482f92419a0c17bb9bfae00c64d251c6a527f95c67993d86bf2d11d9e2e7a9e`.
- Used by: `apps/reports` `Accessi attivi` XLSX export.
- Open questions: none.

### Slow Read Endpoints Must Fit Server Write Timeout

- Context: slow report-style endpoints behind the shared Go HTTP server, including `GET /api/panoramica/v1/iaas/monthly-charges`.
- Discovery: a handler can finish its SQL work and still surface as a client-side transport failure if response delivery exceeds the server write budget or the downstream connection closes first. In that case, naive access logs can still misleadingly report a clean `200` unless the response writer captures downstream write errors.
- Practical rule: when a read endpoint is expected to run for tens of seconds, align `http.Server.WriteTimeout` with that runtime budget and make access logs record downstream write failures and request-context cancellation separately from normal completions.
- Evidence: Panoramica local-dev failure on 2026-04-09 where `monthly-charges` took ~44s, Vite logged `socket hang up`, and backend access logging needed downstream write-error tracking to distinguish true delivery from handler completion.
- Used by: `apps/panoramica-cliente` IaaS PPU monthly charges view; shared backend middleware in `backend/pkg/middleware`.
- Open questions: whether future report endpoints should adopt per-handler query deadlines or asynchronous export flows instead of relying on a larger shared write timeout.

### AFC Tools Order PDF Missing in Arxivar Surfaces as `ARX_DOC_NUMBER_NOT_FOUND`

- Context: `GET /api/afc-tools/v1/orders/{orderId}/pdf`, which proxies the Mistra/gw-int order PDF endpoint used by the AFC Tools XConnect orders view.
- Discovery: when the upstream order document has not yet landed in Arxivar, the external gateway can return HTTP `500` with JSON body `{"message":"ARX_DOC_NUMBER_NOT_FOUND"}` instead of a cleaner 404-style missing-resource response.
- Practical rule: mrsmith should normalize this exact upstream signal to an app-level “PDF not ready yet” state for the AFC Tools order-PDF flow, rather than surfacing it as a generic technical failure. Do not generalize other upstream 500s into the same UX state without an equally specific domain signal.
- Evidence: direct gateway call on 2026-04-19 to `gw-int /orders/v1/order/pdf/301`; AFC domain interpretation that `ARX` refers to Arxivar, the documental system.
- Used by: `apps/afc-tools` XConnect order PDF download flow and `backend/internal/afctools/gateway.go`.
- Open questions: whether other gw-int PDF/document endpoints use the same Arxivar-coded missing-document pattern and should be normalized separately.

## Deployment and Runtime Integration Rules

### New DSN-Backed Mini-Apps Must Update Both Dev and Preprod Env Templates

- Context: introducing a new launcher-backed mini-app that needs backend DSNs and optional split-server frontend URL overrides.
- Discovery: contributor defaults and deployment defaults are documented in two different places: local/backend-facing samples live in `backend/.env.example`, while the repo's pre-production sample lives at the root `.env.preprod.example`. Updating only the backend-local example leaves the real deploy template stale.
- Practical rule: when a new mini-app adds config such as `<APP>_APP_URL` or `<DB>_DSN`, update `backend/internal/platform/config/config.go`, `backend/.env.example`, and the root `.env.preprod.example` in the same change set. Treat both env examples as part of repo-fit wiring, not optional documentation.
- Evidence: Coperture rollout on 2026-04-17 added `COPERTURE_APP_URL` / `DBCOPERTURE_DSN` in `backend/internal/platform/config/config.go`, `backend/.env.example`, and `.env.preprod.example`.
- Used by: `apps/coperture`; future DSN-backed mini-apps.
- Open questions: none.

### Backend-Served SPAs Must Be Copied Explicitly Into `/static/apps/<slug>`

- Context: production and pre-production deployments where the Go server serves multiple Vite bundles from a shared static root.
- Discovery: adding an app to the launcher catalog and giving it a Vite `base` like `/apps/reports/` is not enough to make it deployable. The final runtime image must also copy that app's built dist directory into `/static/apps/<slug>`, otherwise the `staticspa` handler has no `index.html` to fall back to and deep links return the backend's plain 404.
- Practical rule: every new backend-served SPA needs the full pathing chain verified together: launcher/catalog href, Vite build base, local dev override if needed, Docker `COPY --from=frontend /app/apps/<slug>/dist /static/apps/<slug>`, and a `staticspa` deep-link regression test.
- Evidence: `deploy/Dockerfile`, `backend/internal/platform/staticspa/handler.go`, `backend/internal/platform/applaunch/catalog.go`, and the 2026-04-15 production `reports` regression where `/apps/reports/` 404ed because `/static/apps/reports/index.html` was missing from the image.
- Used by: `budget`, `compliance`, `kit-products`, `listini-e-sconti`, `panoramica-cliente`, `quotes`, `reports`.
- Open questions: none.

### Frontend Production Builds Must Exclude Node-Only Test Files

- Context: Vite/React mini-apps with `src/**/*.test.ts` files that use Node's built-in test modules such as `node:test` or `node:assert/strict`.
- Discovery: Docker production builds run `pnpm install --frozen-lockfile && pnpm -r build` in a fresh frontend stage. If a mini-app build script uses `tsc -b` against the default app `tsconfig.json`, TypeScript compiles tests included by `"include": ["src"]`. In that fresh deploy image, Node typings are not guaranteed to be available, so Node-only test imports can break the production build even when the app bundle itself is valid.
- Practical rule: when adding local TypeScript test files to a frontend mini-app, add or reuse `tsconfig.build.json` that extends the app `tsconfig.json` and excludes `src/**/*.test.ts` / `src/**/*.test.tsx`, then point the package `build` script at `tsc -b tsconfig.build.json && vite build`. Keep the test script responsible for running those files directly.
- Evidence: `apps/simulatori-vendita/tsconfig.build.json`; `apps/fornitori/tsconfig.build.json`; `make deploy-prod` failure on 2026-04-28 from `apps/fornitori/src/lib/providerAttention.test.ts` and `providerState.test.ts` importing `node:test` during the Docker frontend stage.
- Used by: `apps/simulatori-vendita`, `apps/fornitori`.
- Open questions: none.

### Docker Frontend Builds Must Exclude Local Vite Env Files

- Context: Vite mini-app production images built by `deploy/Dockerfile`.
- Discovery: `.gitignore` excludes `.env.local`, but Docker does not use `.gitignore`. Without a matching `.dockerignore`, `COPY apps/ apps/` can copy app-local Vite env files into the frontend build stage. Vite then inlines `VITE_*` values at build time, so a local `VITE_DEV_AUTH_BYPASS=true` can produce a production bundle that bypasses Keycloak and sends the literal `dev-token`.
- Practical rule: production Docker contexts must exclude `**/.env.local` and `**/.env.*.local`, and production bundles should be checked for dev-auth markers such as `dev-token` or `VITE_DEV_AUTH_BYPASS=true` before deploy. Runtime env changes cannot repair an already-built Vite bundle; rebuild from a clean context.
- Evidence: `deploy/Dockerfile` copies `apps/` wholesale; no repo `.dockerignore` was present during the 2026-04-28 Fornitori production auth incident; `apps/fornitori/.env.local` contained `VITE_DEV_AUTH_BYPASS=true`; the built `apps/fornitori/dist` bundle inlined the bypass condition as true and included `dev-token`.
- Used by: all Vite mini-app production builds, especially apps using `@mrsmith/auth-client`.
- Open questions: whether to add a hard production-build guard in `@mrsmith/auth-client` in addition to Docker context hygiene.

### Portal Launcher Tiles Must Use Supported Portal Icon Keys

- Context: adding or changing entries in `backend/internal/platform/applaunch/catalog.go`.
- Discovery: launcher tile icons are rendered from the portal-local registry in `apps/portal/src/components/Icon/icons.tsx`, not from an open-ended icon namespace. Reusing a string that is not in that registry leaves the tile without a matching portal icon; during Energia in DC wiring, `bolt` was rejected and the tile used the already-supported `chart` key instead.
- Practical rule: when wiring a new launcher tile, verify the icon key against `apps/portal/src/components/Icon/icons.tsx` or reuse an already-proven key from `apps/portal/src/data/apps.ts`. Do not invent icon names in `catalog.go` without checking portal support first.
- Evidence: `apps/portal/src/components/Icon/icons.tsx`, `apps/portal/src/data/apps.ts`, `backend/internal/platform/applaunch/catalog.go`.
- Used by: launcher-backed apps including `reports`, `coperture`, and `energia-dc`.
- Open questions: none.

## Auth and Transport Behavior

### Devadmin Must Be Centralized as a Superuser Override

- Context: Keycloak-role authorization across launcher visibility, backend ACL middleware, and app-specific elevated permissions.
- Discovery: role checks implemented independently (`acl.RequireRole`, launcher catalog filtering, and direct role checks like quotes delete) drift unless they share a single superuser rule.
- Practical rule: implement `app_devadmin` as a centralized override in shared authz helpers and consume those helpers everywhere role checks are performed (backend ACL, portal catalog filters, app-specific elevated checks, and frontend role-gated controls). Avoid raw `includes`/`slices.Contains` role checks in feature code.
- Evidence: `backend/internal/authz/authz.go`, `backend/internal/acl/acl.go`, `backend/internal/platform/applaunch/catalog.go`, `backend/internal/quotes/handler_quotes.go`, `packages/auth-client/src/roles.ts`, `apps/quotes/src/components/QuoteTable.tsx`.
- Used by: portal app visibility, all ACL-protected backend app routes, quotes delete authorization.
- Open questions: none.

### Shared SPA Clients Must Not Hit Protected APIs Before a Bearer Token Exists

- Context: frontend mini-apps using `@mrsmith/auth-client` plus `@mrsmith/api-client` for Keycloak-protected `/api/*` requests.
- Discovery: if the shared API client sends a request when `getAccessToken()` returns `undefined`, the backend logs a noisy `401 missing_bearer`, then a forced refresh-and-retry can immediately succeed with `200`. When app wrappers also call `login()` from request-level 401 handlers, that pattern can escalate into visible remount/refetch loops.
- Practical rule: shared API clients must acquire a bearer token before the first network request, treat "no token available" as a local unauthorized error, and reserve backend retries for true stale-token 401s. Reauthentication should be driven centrally by `AuthProvider` refresh failure handling, not per-app query error callbacks.
- Evidence: `packages/api-client/src/client.ts`, `packages/auth-client/src/AuthProvider.tsx`, and the 2026-04-17 `apps/richieste-fattibilita` loop on `GET /api/rdf/v1/richieste/summary` alternating `401 missing_bearer` and `200`.
- Used by: portal and all mini-apps using the shared API/auth client stack.
- Open questions: none.

### Mini-App Auth Fallbacks Must Fail Closed and Retry Local Preflight Unauthorized Errors

- Context: Vite mini-app bootstraps using `useOptionalAuth()`, app-shell auth gates, and React Query startup fetches.
- Discovery: if an app-local auth fallback reports `authenticated: true` without a token, routed pages mount before Keycloak state is usable. Once the shared API client correctly refuses to send bearerless requests, those startup fetches fail locally with no backend logs; if React Query also disables retries for every `401`, the page can freeze in a false "not authorized" state.
- Practical rule: optional-auth fallbacks must default to `unauthenticated`, mini-app shells must gate route rendering on `authenticated` rather than `loading` alone, and query retry policies must keep retry disabled for real backend ACL failures while allowing retries for local auth-preflight `401`s.
- Evidence: `apps/*/src/hooks/useOptionalAuth.ts`, `apps/*/src/App.tsx`, `apps/*/src/main.tsx`, `apps/richieste-fattibilita/src/lib/format.ts`, and the 2026-04-17 first-load `richieste-fattibilita` empty-state error with no matching backend request.
- Used by: all mini-apps consuming `@mrsmith/auth-client` and `@mrsmith/api-client`.
- Open questions: none.

## Legacy Data Model Constraints

### Alyante Product Translation Write Contract

- Context: server-side sync of product short descriptions from kit-products into Alyante ERP table `MG87_ARTDESC`.
- Discovery: the live Appsmith datasource query updates `MG87_DESCART` and filters with suffixed legacy column names: `MG87_DITTA_CG18`, `MG87_OPZIONE_MG5E`, `MG87_LINGUA_MG52`, `MG87_CODART_MG66`. Earlier backend assumptions using `MG87_DESCRIZIONE`, `MG87_DITTA`, `MG87_OPZIONE`, `MG87_LINGUA`, `MG87_CODART` do not match this environment.
- Practical rule: when writing product short descriptions to Alyante, use `UPDATE MG87_ARTDESC SET MG87_DESCART = ?` with `MG87_DITTA_CG18 = 1`, `MG87_OPZIONE_MG5E = '                    '`, `MG87_LINGUA_MG52 = 'ITA'/'ING'`, and `MG87_CODART_MG66 = code.padEnd(25, ' ')`.
- Evidence: verified Appsmith query `update MG87_ARTDESC set MG87_DESCART = {{this.params.descr}} where MG87_DITTA_CG18 = 1 and MG87_OPZIONE_MG5E = '                    ' and MG87_LINGUA_MG52 = {{this.params.lang}} AND MG87_CODART_MG66 = {{this.params.code}}`; backend adapter in `backend/internal/kitproducts/alyante.go`.
- Used by: `apps/kit-products` product translation sync.
- Open questions: none for this environment; if another Alyante tenant exposes different column names, verify its datasource query before generalizing.

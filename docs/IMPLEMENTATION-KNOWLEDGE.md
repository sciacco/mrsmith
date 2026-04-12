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
- Practical rule: derive IaaS kit/services from `quotes.template` (`template_type`, `kit_id`, `service_category_id`) and treat DB metadata as the single source of truth. Backend create must reject IaaS templates without a selectable kit.
- Evidence: `apps/quotes/src/pages/QuoteCreatePage.tsx`, `apps/quotes/src/components/HeaderTab.tsx`, `backend/internal/quotes/handler_quotes.go`, `backend/internal/quotes/handler_create_test.go`.
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

### Slow Read Endpoints Must Fit Server Write Timeout

- Context: slow report-style endpoints behind the shared Go HTTP server, including `GET /api/panoramica/v1/iaas/monthly-charges`.
- Discovery: a handler can finish its SQL work and still surface as a client-side transport failure if response delivery exceeds the server write budget or the downstream connection closes first. In that case, naive access logs can still misleadingly report a clean `200` unless the response writer captures downstream write errors.
- Practical rule: when a read endpoint is expected to run for tens of seconds, align `http.Server.WriteTimeout` with that runtime budget and make access logs record downstream write failures and request-context cancellation separately from normal completions.
- Evidence: Panoramica local-dev failure on 2026-04-09 where `monthly-charges` took ~44s, Vite logged `socket hang up`, and backend access logging needed downstream write-error tracking to distinguish true delivery from handler completion.
- Used by: `apps/panoramica-cliente` IaaS PPU monthly charges view; shared backend middleware in `backend/pkg/middleware`.
- Open questions: whether future report endpoints should adopt per-handler query deadlines or asynchronous export flows instead of relying on a larger shared write timeout.

## Legacy Data Model Constraints

### Alyante Product Translation Write Contract

- Context: server-side sync of product short descriptions from kit-products into Alyante ERP table `MG87_ARTDESC`.
- Discovery: the live Appsmith datasource query updates `MG87_DESCART` and filters with suffixed legacy column names: `MG87_DITTA_CG18`, `MG87_OPZIONE_MG5E`, `MG87_LINGUA_MG52`, `MG87_CODART_MG66`. Earlier backend assumptions using `MG87_DESCRIZIONE`, `MG87_DITTA`, `MG87_OPZIONE`, `MG87_LINGUA`, `MG87_CODART` do not match this environment.
- Practical rule: when writing product short descriptions to Alyante, use `UPDATE MG87_ARTDESC SET MG87_DESCART = ?` with `MG87_DITTA_CG18 = 1`, `MG87_OPZIONE_MG5E = '                    '`, `MG87_LINGUA_MG52 = 'ITA'/'ING'`, and `MG87_CODART_MG66 = code.padEnd(25, ' ')`.
- Evidence: verified Appsmith query `update MG87_ARTDESC set MG87_DESCART = {{this.params.descr}} where MG87_DITTA_CG18 = 1 and MG87_OPZIONE_MG5E = '                    ' and MG87_LINGUA_MG52 = {{this.params.lang}} AND MG87_CODART_MG66 = {{this.params.code}}`; backend adapter in `backend/internal/kitproducts/alyante.go`.
- Used by: `apps/kit-products` product translation sync.
- Open questions: none for this environment; if another Alyante tenant exposes different column names, verify its datasource query before generalizing.

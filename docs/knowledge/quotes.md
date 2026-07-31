# Quotes

Knowledge entries specific to `apps/quotes`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

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

### Quotes Pending Approval Status Is Finalized From HubSpot

- Context: `apps/quotes` proposals published with legal notes and stored locally as `PENDING_APPROVAL`.
- Discovery: HubSpot is the source of truth after approval review. Local pending quotes must be synchronized from HubSpot `hs_status`; `hs_sign_status` is separate and remains out of scope for local status transitions.
- Practical rule: the backend scheduled worker should inspect only local quotes with `status = PENDING_APPROVAL` and `hs_quote_id IS NOT NULL`. It may update local status to `APPROVED`, `APPROVAL_NOT_NEEDED`, or `REJECTED`; it must skip `DRAFT`, `PENDING_APPROVAL`, empty, and unknown HubSpot statuses. Conversion to order remains allowed only for local `APPROVED`. HubSpot lookup failures, local update failures, runtime config failures, and advisory-lock release failures must be logged as `WARN` records with `component = quotes` and an explicit `operation`, so deployments with diagnostics enabled persist them to `mrsmith.diagnostic_event`.
- Evidence: GitHub issue #44 implementation plan; `backend/internal/quotes/status_sync.go`; `apps/quotes/QUOTES-SPEC.md`.
- Used by: `apps/quotes` list/detail status display and order-conversion gating.
- Open questions: none.

### Quotes Order Conversion Uses Vodka Bridge Plus HubSpot Note Attachment

- Context: `apps/quotes` conversion from proposal to legacy Vodka/daiquiri sales order.
- Discovery: the active Appsmith conversion flow creates the Vodka `orders` header and `orders_rows`, records `orders.legacy_orders`, generates the order PDF through `GET /orders/v1/order/pdf/{orderId}/generate`, uploads it to HubSpot Files under `/deal-documents`, then creates a HubSpot note associated to the deal with association type `214`. The dormant `AssociateFileToDeal` query is not part of the active flow and contains a bad field reference. The conversion page also blocks every proposal status except `APPROVED`.
- Practical rule: retry and status logic should use `orders.legacy_orders.quote_id -> vodka_id` as the canonical bridge, but only when the source quote is still `APPROVED`. Do not recreate Vodka orders when the bridge exists. If a matching Vodka order exists by `cdlan_ndoc` + `cdlan_anno` without the bridge, stop with a conflict instead of creating a duplicate. Attach the PDF through the note association, not the dormant file-to-deal endpoint. Persist HubSpot conversion metadata in `orders.legacy_orders.jdata.hubspot` (`deal_id`, `file_id`, `note_id`, etc.) so retries skip already-completed HubSpot steps and Ordini revert can best-effort delete the note and file. Use `quotes.template.lang` as the converted order language for both `orders.profile_lang` and `orders_rows.cdlan_descart`; if the template language is missing, fall back to Italian and do not use the customer profile language. Keep Vodka PDF-facing text fields such as `orders.cdlan_note`, `orders.data_decorrenza`, `orders.cdlan_rif_*`, and selected `orders_rows` string display values as empty strings rather than `NULL` for converted orders, and normalize legacy `NULL` values before gateway PDF calls because the gw-int order PDF path can fail while scanning nullable text into non-null Go strings. Do not normalize operational date sentinels such as `orders.cdlan_dataconferma`, `orders_rows.cdlan_data_attivazione`, or `orders_rows.data_annullamento`: Ordini and/or gw-int use `NULL` differently from an empty date string.
- Evidence: `apps/quotes/quotes-main.tar.gz` `Converti in ordine/jsobjects/utilsCopy/utilsCopy.js`; recovered `artifacts/Ordini-gestione-portale.json` `gpUtils.newOrderFromQuote` and `gpUtils.rowsFromQuote`; Mistra trigger `quotes.update_kit_product_rows()` already derives quote row language from `quotes.template.lang`; implementation in `backend/internal/quotes/order_conversion.go`.
- Used by: `apps/quotes` `POST /api/quotes/v1/quotes/{id}/convert-order` and `GET /api/quotes/v1/quotes/{id}/order-conversion`.
- Open questions: none.

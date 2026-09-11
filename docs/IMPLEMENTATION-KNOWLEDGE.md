# Implementation Knowledge Handbook

This document is the index of the canonical handbook for reusable implementation knowledge discovered while building apps in this repo. The entries themselves live in [`docs/knowledge/`](knowledge/), one file per app or cross-cutting domain.

Use the handbook to capture facts that are easy to rediscover badly and expensive to relearn later: identifier mappings, cross-system joins, hidden business rules, exclusions, legacy quirks, API/DB mismatches, and operational conventions that affect future implementations.
For Appsmith migrations, use [docs/APPSMITH-MIGRATION-PLAYBOOK.md](APPSMITH-MIGRATION-PLAYBOOK.md) first to extract and pin verified contracts, then record any reusable discoveries here.

## How to Use This Handbook

- Load progressively: read this index first, then open only the files relevant to the work — the file of the app being changed, plus the cross-cutting files its work touches (identity, auth, data contracts, deployment).
- Update the handbook in the same change set when implementation work uncovers reusable knowledge: add the entry to the right `knowledge/` file AND add its line to this index. An entry without an index line is invisible.
- Placement rule: a rule that only one app enforces goes in that app's file; a rule that binds anyone crossing a system (an identity mapping, a vendor contract, a shared-database column quirk, an auth behavior) goes in the matching domain file — even if it has a single consumer today.
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

## Index

### Cross-System Identity and Keys — [`knowledge/cross-system-identity.md`](knowledge/cross-system-identity.md)

- [Customer Identity Across Systems](knowledge/cross-system-identity.md#customer-identity-across-systems)
- [HubSpot Company Lookup from Grappa](knowledge/cross-system-identity.md#hubspot-company-lookup-from-grappa)
- [Known Grappa Customer Exclusions](knowledge/cross-system-identity.md#known-grappa-customer-exclusions)
- [Cross-Database Mini-App Summaries Must Merge In Code, Not In One SQL Join](knowledge/cross-system-identity.md#cross-database-mini-app-summaries-must-merge-in-code-not-in-one-sql-join)
- [HubSpot Deal Codes Match ERP Orders After Separator Normalization](knowledge/cross-system-identity.md#hubspot-deal-codes-match-erp-orders-after-separator-normalization)

### Platform Integrations — [`knowledge/platform-integrations.md`](knowledge/platform-integrations.md)

- [Google Shared Drive Service Accounts Need Explicit Trash and Access Handling](knowledge/platform-integrations.md#google-shared-drive-service-accounts-need-explicit-trash-and-access-handling)
- [Direct User-Triggered Emails Go Through the Email Ledger](knowledge/platform-integrations.md#direct-user-triggered-emails-go-through-the-email-ledger)
- [OpenAPI.it Wrappers Stay Backend-Side](knowledge/platform-integrations.md#openapiit-wrappers-stay-backend-side)
- [OpenAPI.it CAP `cod_fisco` Can Be Alphanumeric](knowledge/platform-integrations.md#openapiit-cap-cod_fisco-can-be-alphanumeric)
- [OpenAPI.it DocuEngine Payloads Need Defensive Decoding And Vendor-Paced Polling](knowledge/platform-integrations.md#openapiit-docuengine-payloads-need-defensive-decoding-and-vendor-paced-polling)
- [Mistral OCR Page Markdown Excludes Tables When `include_blocks` Is On](knowledge/platform-integrations.md#mistral-ocr-page-markdown-excludes-tables-when-include_blocks-is-on)
- [OpenAPI.it IT-full Closing Dates Are Local Midnight Serialized In UTC](knowledge/platform-integrations.md#openapiit-it-full-closing-dates-are-local-midnight-serialized-in-utc)
- [Company Domain Search Queries Must Never Contain Fiscal Identifiers](knowledge/platform-integrations.md#company-domain-search-queries-must-never-contain-fiscal-identifiers)
- [HubSpot Writes Go Through A Shared Async Queue On Anisetta](knowledge/platform-integrations.md#hubspot-writes-go-through-a-shared-async-queue-on-anisetta)

### Auth and Transport Behavior — [`knowledge/auth-transport.md`](knowledge/auth-transport.md)

- [Keycloak Role User Lookups Must Include Group-Derived Membership](knowledge/auth-transport.md#keycloak-role-user-lookups-must-include-group-derived-membership)
- [Devadmin Must Be Centralized as a Superuser Override](knowledge/auth-transport.md#devadmin-must-be-centralized-as-a-superuser-override)
- [Shared SPA Clients Must Not Hit Protected APIs Before a Bearer Token Exists](knowledge/auth-transport.md#shared-spa-clients-must-not-hit-protected-apis-before-a-bearer-token-exists)
- [Keycloak Initialization Must Be Idempotent In AuthProvider](knowledge/auth-transport.md#keycloak-initialization-must-be-idempotent-in-authprovider)
- [Mini-App Auth Fallbacks Must Fail Closed and Retry Local Preflight Unauthorized Errors](knowledge/auth-transport.md#mini-app-auth-fallbacks-must-fail-closed-and-retry-local-preflight-unauthorized-errors)
- [Mini-App Shells Must Gate Routes With Launcher Roles](knowledge/auth-transport.md#mini-app-shells-must-gate-routes-with-launcher-roles)
- [Route-Scoped Roles Share the App Launcher Role Set](knowledge/auth-transport.md#route-scoped-roles-share-the-app-launcher-role-set)

### Deployment and Runtime Integration Rules — [`knowledge/deployment-runtime.md`](knowledge/deployment-runtime.md)

- [Slow Read Endpoints Must Fit Server Write Timeout](knowledge/deployment-runtime.md#slow-read-endpoints-must-fit-server-write-timeout)
- [Database Diagnostics Are A Low-Volume Error Inbox, Not Access Logging](knowledge/deployment-runtime.md#database-diagnostics-are-a-low-volume-error-inbox-not-access-logging)
- [MrSmith-Owned Tables In Anisetta](knowledge/deployment-runtime.md#mrsmith-owned-tables-in-anisetta)
- [GW Internal CDLAN Calls Use The Shared Arak Client](knowledge/deployment-runtime.md#gw-internal-cdlan-calls-use-the-shared-arak-client)
- [New DSN-Backed Mini-Apps Must Update Both Dev and Preprod Env Templates](knowledge/deployment-runtime.md#new-dsn-backed-mini-apps-must-update-both-dev-and-preprod-env-templates)
- [Production Deploy Builds Run On The Target Host From A Git Archive](knowledge/deployment-runtime.md#production-deploy-builds-run-on-the-target-host-from-a-git-archive)

### Frontend Platform and Builds — [`knowledge/frontend-platform.md`](knowledge/frontend-platform.md)

- [CSS Module Keyframes Are Scoped; Global Animation Names Wake Dormant Rules](knowledge/frontend-platform.md#css-module-keyframes-are-scoped-global-animation-names-wake-dormant-rules)
- [Backend-Served SPAs Must Be Copied Explicitly Into `/static/apps/<slug>`](knowledge/frontend-platform.md#backend-served-spas-must-be-copied-explicitly-into-staticappsslug)
- [Frontend Production Builds Must Exclude Node-Only Test Files](knowledge/frontend-platform.md#frontend-production-builds-must-exclude-node-only-test-files)
- [Packages Imported By `vite.config.ts` Must Ship Runnable JavaScript](knowledge/frontend-platform.md#packages-imported-by-viteconfigts-must-ship-runnable-javascript)
- [Docker Frontend Builds Must Exclude Local Vite Env Files](knowledge/frontend-platform.md#docker-frontend-builds-must-exclude-local-vite-env-files)
- [Portal Launcher Tiles Must Use Supported Portal Icon Keys](knowledge/frontend-platform.md#portal-launcher-tiles-must-use-supported-portal-icon-keys)

### Shared Data Contracts — [`knowledge/shared-data-contracts.md`](knowledge/shared-data-contracts.md)

- [Grappa DATE Columns Serialize as RFC3339 When Scanned to String](knowledge/shared-data-contracts.md#grappa-date-columns-serialize-as-rfc3339-when-scanned-to-string)
- [Panoramica Orders Summary Text Columns Can Be NULL](knowledge/shared-data-contracts.md#panoramica-orders-summary-text-columns-can-be-null)
- [Loader `quantita` Must Be Treated as Decimal (Nullable) Across Reports and Panoramica](knowledge/shared-data-contracts.md#loader-quantita-must-be-treated-as-decimal-nullable-across-reports-and-panoramica)
- [Loader `tipo_documento` Is Fixed-Width Padded; `v_ordini_ric_spot` Is The Canonical Recurring+Spot Source](knowledge/shared-data-contracts.md#loader-tipo_documento-is-fixed-width-padded-v_ordini_ric_spot-is-the-canonical-recurringspot-source)
- [Spot Orders Carry One-Off Amounts In `canone`; MRC Must Be Reclassified As NRC](knowledge/shared-data-contracts.md#spot-orders-carry-one-off-amounts-in-canone-mrc-must-be-reclassified-as-nrc)
- [Alyante Product Translation Write Contract](knowledge/shared-data-contracts.md#alyante-product-translation-write-contract)
- [`common.vocabulary` Is Not Universally Read-Only for Mini-Apps](knowledge/shared-data-contracts.md#commonvocabulary-is-not-universally-read-only-for-mini-apps)

### RDA — [`knowledge/rda.md`](knowledge/rda.md)

- [RDA New Supplier Requests Must Use Provider Draft Create](knowledge/rda.md#rda-new-supplier-requests-must-use-provider-draft-create)
- [RDA Approval Inbox Actionability Is State-Gated](knowledge/rda.md#rda-approval-inbox-actionability-is-state-gated)
- [RDA Payment Method Standard Rule](knowledge/rda.md#rda-payment-method-standard-rule)
- [RDA Currency Is A PO-Level Display Contract](knowledge/rda.md#rda-currency-is-a-po-level-display-contract)
- [RDA Attachment Type Is User-Selected At Upload](knowledge/rda.md#rda-attachment-type-is-user-selected-at-upload)
- [RDA PO PDF Download Is State-Gated](knowledge/rda.md#rda-po-pdf-download-is-state-gated)
- [RDA Patch Payload Null Semantics](knowledge/rda.md#rda-patch-payload-null-semantics)
- [RDA PO Recipients Use The Dedicated Recipients Endpoint](knowledge/rda.md#rda-po-recipients-use-the-dedicated-recipients-endpoint)
- [RDA Budget Selection Keys](knowledge/rda.md#rda-budget-selection-keys)
- [RDA Row Totals Are Normalized By The BFF](knowledge/rda.md#rda-row-totals-are-normalized-by-the-bff)
- [RDA Good Row Create Still Needs Empty Renew Detail](knowledge/rda.md#rda-good-row-create-still-needs-empty-renew-detail)
- [RDA Row Edit Is A BFF Replace Operation](knowledge/rda.md#rda-row-edit-is-a-bff-replace-operation)
- [RDA Portal Deep Links Include App Mount And App Route](knowledge/rda.md#rda-portal-deep-links-include-app-mount-and-app-route)
- [RDA Comment Mentions Notify Through MrSmith, Not Mistra](knowledge/rda.md#rda-comment-mentions-notify-through-mrsmith-not-mistra)
- [RDA Article Catalog Type Comes From The BFF](knowledge/rda.md#rda-article-catalog-type-comes-from-the-bff)
- [RDA Approval Permissions Come From users_int.role](knowledge/rda.md#rda-approval-permissions-come-from-users_introle)
- [RDA DDT Documents Are Queried Directly On Arak With A Half-Open Civil-Day Range](knowledge/rda.md#rda-ddt-documents-are-queried-directly-on-arak-with-a-half-open-civil-day-range)

### Raenad / Aenad — [`knowledge/raenad-aenad.md`](knowledge/raenad-aenad.md)

- [Raenad HubSpot Deal Pipeline Config Lives In Anisetta Runtime Config](knowledge/raenad-aenad.md#raenad-hubspot-deal-pipeline-config-lives-in-anisetta-runtime-config)
- [Raenad Stage Choices Come From The HubSpot Loader Mirror](knowledge/raenad-aenad.md#raenad-stage-choices-come-from-the-hubspot-loader-mirror)
- [Raenad Deal Owner Maps From User Email To HubSpot Owner](knowledge/raenad-aenad.md#raenad-deal-owner-maps-from-user-email-to-hubspot-owner)
- [Raenad Deal Create Uses Only Standard HubSpot Properties In V1](knowledge/raenad-aenad.md#raenad-deal-create-uses-only-standard-hubspot-properties-in-v1)
- [Raenad UI/UX Planning Is Deferred Until Backend Contracts Are Stable](knowledge/raenad-aenad.md#raenad-uiux-planning-is-deferred-until-backend-contracts-are-stable)
- [Aenad Quote Line Defaults Live In Anisetta Runtime Config](knowledge/raenad-aenad.md#aenad-quote-line-defaults-live-in-anisetta-runtime-config)
- [Raenad Payment Methods Come From loader.erp_metodi_pagamento](knowledge/raenad-aenad.md#raenad-payment-methods-come-from-loadererp_metodi_pagamento)
- [Raenad Ready Quotes Return To Draft On Commercial Changes](knowledge/raenad-aenad.md#raenad-ready-quotes-return-to-draft-on-commercial-changes)
- [Aenad Document Totals Are Database-Owned First-Tranche Calculations](knowledge/raenad-aenad.md#aenad-document-totals-are-database-owned-first-tranche-calculations)
- [Aenad Monetary And Quantity Fields Are numeric(18,4) Exposed As Decimal Strings](knowledge/raenad-aenad.md#aenad-monetary-and-quantity-fields-are-numeric184-exposed-as-decimal-strings)
- [Aenad Offer Print Semantics: Easyfatt Inline Markers, Spacer Rows, Display-Ready Columns](knowledge/raenad-aenad.md#aenad-offer-print-semantics-easyfatt-inline-markers-spacer-rows-display-ready-columns)
- [Raenad Is The Operational Quote Schema, Separate From The Aenad Archive](knowledge/raenad-aenad.md#raenad-is-the-operational-quote-schema-separate-from-the-aenad-archive)
- [Raenad Quote Numbers Use common.new_document_number('AE-')](knowledge/raenad-aenad.md#raenad-quote-numbers-use-commonnew_document_numberae-)
- [Raenad Stores A Printable Customer/Contact Snapshot Decoupled From The HubSpot Mirror](knowledge/raenad-aenad.md#raenad-stores-a-printable-customercontact-snapshot-decoupled-from-the-hubspot-mirror)
- [Raenad VAT Is cod_iva Plus A Persisted iva_percent_snapshot](knowledge/raenad-aenad.md#raenad-vat-is-cod_iva-plus-a-persisted-iva_percent_snapshot)
- [Raenad PDF Exports Are Immutable Revisions](knowledge/raenad-aenad.md#raenad-pdf-exports-are-immutable-revisions)

### Binocolo — [`knowledge/binocolo.md`](knowledge/binocolo.md)

- [Binocolo M&A Uses Company `IT-search` With ATECO-First Fallback](knowledge/binocolo.md#binocolo-ma-uses-company-it-search-with-ateco-first-fallback)
- [Binocolo OpenAPI Company Facts Refresh After 24 Hours](knowledge/binocolo.md#binocolo-openapi-company-facts-refresh-after-24-hours)
- [Binocolo Domain Identity, Provenance, and Thesis Fit Are Separate Axes](knowledge/binocolo.md#binocolo-domain-identity-provenance-and-thesis-fit-are-separate-axes)
- [Binocolo M&A Long Session Work Uses `ma_job`](knowledge/binocolo.md#binocolo-ma-long-session-work-uses-ma_job)
- [Binocolo `/azienda` Is A Standalone Quick-Review Tool, Never The MA Dossier Destination](knowledge/binocolo.md#binocolo-azienda-is-a-standalone-quick-review-tool-never-the-ma-dossier-destination)
- [Binocolo Internal Company Finder Uses Fiscal Identity Groups](knowledge/binocolo.md#binocolo-internal-company-finder-uses-fiscal-identity-groups)
- [Binocolo `company_key` Is An Owned Identifier, Never A Derivation](knowledge/binocolo.md#binocolo-company_key-is-an-owned-identifier-never-a-derivation)
- [Binocolo Company Annotations Live in `ma_target_outcome`](knowledge/binocolo.md#binocolo-company-annotations-live-in-ma_target_outcome)
- [Binocolo ATECO 2025 Codes Are Resolver-Gated](knowledge/binocolo.md#binocolo-ateco-2025-codes-are-resolver-gated)
- [Binocolo Province Selection Is Tool-Gated](knowledge/binocolo.md#binocolo-province-selection-is-tool-gated)
- [LLM Registry Cutover Did Not Preserve IDs; Legacy FKs to binocolo.llm_* Break on Write](knowledge/binocolo.md#llm-registry-cutover-did-not-preserve-ids-legacy-fks-to-binocolollm_-break-on-write)

### Quotes — [`knowledge/quotes.md`](knowledge/quotes.md)

- [Quotes Create Flow Uses Context-Specific Category Exclusions](knowledge/quotes.md#quotes-create-flow-uses-context-specific-category-exclusions)
- [Quotes IaaS Template Derivation Must Be DB-Driven](knowledge/quotes.md#quotes-iaas-template-derivation-must-be-db-driven)
- [Quotes Replacement Orders Need Appsmith Column Names Plus Customer Scoping](knowledge/quotes.md#quotes-replacement-orders-need-appsmith-column-names-plus-customer-scoping)
- [Quotes Publish Payment Labels Use Loader ERP Column Names](knowledge/quotes.md#quotes-publish-payment-labels-use-loader-erp-column-names)
- [Quotes Deal Number Must Come From HubSpot `codice`, Not Deal Title](knowledge/quotes.md#quotes-deal-number-must-come-from-hubspot-codice-not-deal-title)
- [Quotes Customer Default Payment Must Use Alyante `CODICE_PAGAMENTO`](knowledge/quotes.md#quotes-customer-default-payment-must-use-alyante-codice_pagamento)
- [Quotes Republish Must Unlock Published HubSpot Quotes First](knowledge/quotes.md#quotes-republish-must-unlock-published-hubspot-quotes-first)
- [Quotes Pending Approval Status Is Finalized From HubSpot](knowledge/quotes.md#quotes-pending-approval-status-is-finalized-from-hubspot)
- [Quotes Order Conversion Uses Vodka Bridge Plus HubSpot Note Attachment](knowledge/quotes.md#quotes-order-conversion-uses-vodka-bridge-plus-hubspot-note-attachment)

### Energia in DC — [`knowledge/energia-dc.md`](knowledge/energia-dc.md)

- [Customer kW Reports Follow the Daily-Summary Source Aggregation](knowledge/energia-dc.md#customer-kw-reports-follow-the-daily-summary-source-aggregation)

### Grappa DCIM — [`knowledge/grappa-dcim.md`](knowledge/grappa-dcim.md)

- [Grappa Rack Customer Display Uses `cli_fatturazione.intestazione`](knowledge/grappa-dcim.md#grappa-rack-customer-display-uses-cli_fatturazioneintestazione)
- [Grappa Rack Equipment Occupancy Uses `apparato.unit`](knowledge/grappa-dcim.md#grappa-rack-equipment-occupancy-uses-apparatounit)
- [Grappa Apparato Types Are Controlled By DCIM Lookup](knowledge/grappa-dcim.md#grappa-apparato-types-are-controlled-by-dcim-lookup)
- [Grappa DCIM Rack Media Is Not A V1 Feature](knowledge/grappa-dcim.md#grappa-dcim-rack-media-is-not-a-v1-feature)
- [Grappa DCIM Grid Layouts Are Visual Blocks, Not One Layout Per Islet](knowledge/grappa-dcim.md#grappa-dcim-grid-layouts-are-visual-blocks-not-one-layout-per-islet)
- [Grappa DCIM Positions Are Whole Tiles; Half Racks Live On `racks`, Not `positions`](knowledge/grappa-dcim.md#grappa-dcim-positions-are-whole-tiles-half-racks-live-on-racks-not-positions)

### Training — [`knowledge/training.md`](knowledge/training.md)

- [Training Directory Chips Are Action-First](knowledge/training.md#training-directory-chips-are-action-first)
- [Training Request Team Is Optional Only Without Active Memberships; People Decides Without TL Opinion](knowledge/training.md#training-request-team-is-optional-only-without-active-memberships-people-decides-without-tl-opinion)
- [Training Rule Populations Stay Training-Side](knowledge/training.md#training-rule-populations-stay-training-side)
- [Training Compliance Courses Become Mandatory Through Rules](knowledge/training.md#training-compliance-courses-become-mandatory-through-rules)
- [Training People Admin Can Create Local Employees](knowledge/training.md#training-people-admin-can-create-local-employees)
- [Training-Factorial Sync Correlates Objects By Embedded Tokens, Not Foreign Keys](knowledge/training.md#training-factorial-sync-correlates-objects-by-embedded-tokens-not-foreign-keys)

### Reports — [`knowledge/reports.md`](knowledge/reports.md)

- [Reports AOV Replacement MRC Matching](knowledge/reports.md#reports-aov-replacement-mrc-matching)
- [Reports AOV CDL-CLOUD Adds Fixed NRC](knowledge/reports.md#reports-aov-cdl-cloud-adds-fixed-nrc)
- [Reports Carbone Export Payloads May Need Template-Specific Key Aliases](knowledge/reports.md#reports-carbone-export-payloads-may-need-template-specific-key-aliases)

### Ordini — [`knowledge/ordini.md`](knowledge/ordini.md)

- [Vodka Orders Match Alyante Extended Rows By Document](knowledge/ordini.md#vodka-orders-match-alyante-extended-rows-by-document)
- [GW `/orders/v1/erp` Accepts Only the Legacy Appsmith Payload Shape](knowledge/ordini.md#gw-ordersv1erp-accepts-only-the-legacy-appsmith-payload-shape)

### Manutenzioni — [`knowledge/manutenzioni.md`](knowledge/manutenzioni.md)

- [Manutenzioni Radar Excludes Terminal Maintenance States](knowledge/manutenzioni.md#manutenzioni-radar-excludes-terminal-maintenance-states)
- [Manutenzioni Service Taxonomy Is A Catalog, Targets Are Instances](knowledge/manutenzioni.md#manutenzioni-service-taxonomy-is-a-catalog-targets-are-instances)

### Fornitori — [`knowledge/fornitori.md`](knowledge/fornitori.md)

- [Fornitori Provider Contacts Follow Appsmith Payload Semantics](knowledge/fornitori.md#fornitori-provider-contacts-follow-appsmith-payload-semantics)

### Panoramica Cliente — [`knowledge/panoramica-cliente.md`](knowledge/panoramica-cliente.md)

- [Cloudstack IaaS Charge Categories Are Fixed Backend-Side](knowledge/panoramica-cliente.md#cloudstack-iaas-charge-categories-are-fixed-backend-side)

### Richieste Fattibilità — [`knowledge/richieste-fattibilita.md`](knowledge/richieste-fattibilita.md)

- [RDF `fornitori_preferiti` Must Be Treated as Nullable Text](knowledge/richieste-fattibilita.md#rdf-fornitori_preferiti-must-be-treated-as-nullable-text)

### CP Backoffice — [`knowledge/cp-backoffice.md`](knowledge/cp-backoffice.md)

- [CP Backoffice Active Biometric Users Are Balance-Based](knowledge/cp-backoffice.md#cp-backoffice-active-biometric-users-are-balance-based)

### AFC Tools — [`knowledge/afc-tools.md`](knowledge/afc-tools.md)

- [AFC Tools Order PDF Missing in Arxivar Surfaces as `ARX_DOC_NUMBER_NOT_FOUND`](knowledge/afc-tools.md#afc-tools-order-pdf-missing-in-arxivar-surfaces-as-arx_doc_number_not_found)

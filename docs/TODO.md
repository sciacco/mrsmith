# Project TODOs

## Developer Experience

### Single-Origin Dev Gateway
Future implementation plan is tracked in [docs/DEV-GATEWAY-IMPLEMENTATION-PLAN.md](DEV-GATEWAY-IMPLEMENTATION-PLAN.md). This work would replace browser-visible per-app localhost ports with a backend-owned single-origin dev gateway while preserving independent app Vite servers as opt-in processes.

## Listini e Sconti App

### Portal Admin Module — Carbone Template Management
Carbone PDF templates are currently referenced by hardcoded template IDs in individual apps (e.g. kit-products, listini-e-sconti). A portal-wide admin module should be developed to centralize template management (upload, versioning, assignment to apps). Once implemented, all apps using Carbone will be updated to fetch template IDs from the admin module instead of hardcoding them.

### Bulk Kit PDF Export
Currently the Kit di vendita page exports one kit PDF at a time via Carbone. A future enhancement should support bulk export — generating PDFs for all kits (or a filtered subset) in a single operation, either as a ZIP download or a merged multi-kit document. Useful for sales teams preparing full product catalogs.

### Configurable HubSpot Task Assignee
The "Sconti variabile energia" page creates a HubSpot task assigned to a hardcoded email (eva.grimaldi@cdlan.it) when rack energy discounts are changed. This should eventually be configurable — either per-role, per-team, or via an admin setting — rather than hardcoded. Kept as-is for now during Appsmith coexistence.

### Kit Product Price Versioning
Kit product prices are currently not versioned — the catalog always shows current prices. A future enhancement should support effective-dated pricing so that PDFs and historical quotes can reference the prices valid at a specific point in time. This affects both the Kit di vendita page (which prices does the PDF show?) and any future quoting workflows.

### Discount Approval Workflow
Currently rack energy discounts (0–20%) are saved immediately without approval. A future enhancement should add an approval workflow for discounts above a configurable threshold (e.g. >15%), requiring a manager or reviewer to approve before the discount takes effect. This could integrate with the HubSpot task system already in place.

## Panoramica Cliente App

### Dashboard Page
The Dashboard page (revenue charts per client: revenue by account, historical billing in K EUR, active services) is a work in progress in Appsmith and is not included in the initial migration scope. It should be migrated once the Appsmith version is stabilized and the remaining pages are live. The audit is captured in `apps/panoramica-cliente/PANORAMICA-AUDIT.md` (page 1).

## Quotes App

### Multi-Product Selection per Group
Currently the quote product configuration enforces single-selection per `group_name` (radio-button behavior in `quotes.upd_quote_row_product`). A future enhancement should support multi-selection within a group (e.g. selecting both "Monitoraggio" and "Backup" in an "Opzioni aggiuntive" group). This requires coordination with kit management (`products.kit_product` model) to define which groups allow multi-select, and changes to both the stored procedure and the UI product configurator.

## Cross-App Infrastructure

### Async HubSpot Request Queue
Design and implement a shared async queue for submitting requests to HubSpot across all mrsmith apps. Current approach is fire-and-forget with failures tolerated. The queue should support: configurable expiry (TTL per message), exponential retry with backoff, notification channel on persistent failure (e.g. Slack, email), dead-letter handling for undeliverable messages, and per-app/per-entity configuration. This replaces the current pattern where each app calls HubSpot synchronously and silently ignores failures.

## Budget Management App

### Audit Logging
Audit logging for budget and approval rule changes (create, edit, delete) is not in scope for the budget app migration. It needs a separate implementation — either server-side middleware on the Arak API or a dedicated audit service. No client-side audit logging exists in the current Appsmith app either.

### Budget Year-End Lifecycle
Unresolved: what happens to budgets at year-end? Do old-year budgets get archived, copied forward, or accumulate indefinitely in the list? This affects long-term UX of the budget list view. Needs a decision from the domain owner before the budget app grows historical data.

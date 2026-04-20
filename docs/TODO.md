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

## Energia in DC App

### Bulk Actions on Low-Consumption Search (deferred from v1)
The "Consumi < 1A" view lists rack sockets whose average ampere falls below a threshold. Today the user can only read the list. A future enhancement should add bulk actions on selected rows — e.g. open a ticket per socket (for on-site verification), notify the owning customer/account manager, or flag the rack for decommissioning review. Requires: (a) a selection model on the results table, (b) backend endpoints for the chosen actions, (c) integration with the async HubSpot queue (see "Cross-App Infrastructure → Async HubSpot Request Queue") for any CRM-side side effects. Scope and exact action set to be defined with Product.

### Addebiti PDF Export (deferred from v1)
The "Addebiti" view ships CSV export in v1. A future pass should add a PDF export option — likely via the shared Carbone template manager (see "Listini e Sconti App → Portal Admin Module — Carbone Template Management") so the template ID isn't hardcoded in this app.

## Simulatori di Vendita App

### DB-Backed Pricing + Admin View (deferred from v1)
The v1 migration is intentionally narrowed to a source-faithful port of the Appsmith IaaS calculator. The two pricing tiers stay in app-local code for now, there is no pricing database table, and there is no in-app admin route for editing rates. A later phase should move pricing into a backend-owned source of truth and add a dedicated maintenance UI. That follow-up must decide the real host store, introduce read/write pricing endpoints, make PDF generation use server-authoritative pricing, and reopen whether the Appsmith-style `Calcola` interaction should evolve into live recompute.

## Quotes App

### Multi-Product Selection per Group
Currently the quote product configuration enforces single-selection per `group_name` (radio-button behavior in `quotes.upd_quote_row_product`). A future enhancement should support multi-selection within a group (e.g. selecting both "Monitoraggio" and "Backup" in an "Opzioni aggiuntive" group). This requires coordination with kit management (`products.kit_product` model) to define which groups allow multi-select, and changes to both the stored procedure and the UI product configurator.

### Wizard Step 2 — Inline Product Configurator (deferred from refactor Phase 3)
The original UX spec (`quotes-migspec-phaseE-ux.md` §2.4) asked for the full `KitAccordion` + `ProductGroupRadio` inline inside the creation wizard, so the sales user could configure product variants and required groups before clicking "Crea proposta". This was deferred during the refactor because `useRowProducts(quoteId, rowId)` requires an existing quote row on the server — products are resolved by joining `quotes.quote_row` against `products.kit_product`, and there is no "draft quote" path. The wizard currently lets users pick kits (via `KitPickerModal`) and compute NRC/MRC totals at the kit level, but product-level configuration happens only in the detail page after creation. Options for a future pass: (a) add a server endpoint `POST /kits/:id/preview-products` that resolves the same product tree without requiring a `quote_row_id`, then wire `KitAccordion` in "preview" mode in the wizard, (b) accept a `draft` quote status and allow creation before commit, or (c) keep the current two-phase UX and document it as intentional. Option (a) is the cleanest but touches backend.

### SegmentedControl Primitive + Wizard Radio Groups (deferred from refactor Phase 3)
Wizard Step 1 still uses native `<input type="radio">` groups for: document type (Ricorrente/Spot), proposal type (Nuovo/Sostituzione/Rinnovo), NRC charge time (`HeaderTab`), and IaaS language (ITA/ENG). The UX spec wanted these as card-toggle / segmented-control patterns. Scope for the refactor was limited to the already-distinctive `TypeSelector` (standard vs IaaS). A proper fix is to add a shared `SegmentedControl` component to `@mrsmith/ui` (takes `options: {value, label, icon?}[]` and behaves like a radio group with pill styling, rounded track, animated thumb), then replace every remaining native radio in the quotes app. The same primitive would also benefit budget/compliance/listini radio UIs.

### TrialSlider Custom Component (deferred from refactor Phase 3)
The IaaS wizard trial field (`QuoteCreatePage.tsx` step 1) currently uses a native `<input type="range">` with `accent-color`. Spec §2.3 asked for a custom slider: track `--color-surface`, thumb 18px `--color-accent` with `--shadow-sm`, live preview label with currency formatting, tick marks every 50€. Native range works but looks inconsistent with the rest of the DS. Implement as a local `components/TrialSlider.tsx` (or promote to `@mrsmith/ui` if another app needs it) using `role="slider"` + keyboard navigation.

### Accordion drag-drop via @dnd-kit
il KitAccordion usa ancora HTML5 drag-and-drop nativo. @dnd-kit porta hint visivi migliori (shadow-float, scale). sostituirlo

## Design System (@mrsmith/ui)

### Stylelint Cleanup for Shared Components (deferred from refactor Phase 2)
The `lint:css` script enforces the `declaration-property-value-disallowed-list` rule (no hex literals in color-related properties) but is currently scoped to `apps/quotes/src/**/*.module.css` only. Running the same rule against `packages/ui/src/components/**/*.module.css` produces ~300 errors: every component (`Modal`, `SingleSelect`, `MultiSelect`, `SearchInput`, `Skeleton`, `TableToolbar`, `ToastProvider`, `ToggleSwitch`, `TabNav`, `TabNavGroup`, `UserMenu`, `AppShell`) still uses the legacy `var(--color-x, #fallback)` pattern where the hex fallback can drift from the token value (and already does in several places — same token has different fallbacks across files). The hex fallbacks exist because the DS was built before the `clean.css` theme was locked in. The cleanup is purely mechanical (remove the `, #xxx` from every `var()`, replace raw hex with the correct token), but touches every shared component so it needs a focused PR. Once done, extend `lint:css` to include `packages/ui/src/components/**/*.module.css` to prevent regressions across all consumers.

## Cross-App Infrastructure

### Shared Carbone Service
`CarboneService` is duplicated in two packages (`internal/listini/carbone.go` and `internal/reports/carbone.go`) with nearly identical code — the only difference is `convertTo: "pdf"` vs `"xlsx"` and template ID handling (per-struct vs per-call). Extract to a shared `internal/platform/carbone` package with a single service that accepts both format and template ID per call. Both listini and reports would receive the same instance from `main.go`.

### Async HubSpot Request Queue
Design and implement a shared async queue for submitting requests to HubSpot across all mrsmith apps. Current approach is fire-and-forget with failures tolerated. The queue should support: configurable expiry (TTL per message), exponential retry with backoff, notification channel on persistent failure (e.g. Slack, email), dead-letter handling for undeliverable messages, and per-app/per-entity configuration. This replaces the current pattern where each app calls HubSpot synchronously and silently ignores failures.

## Budget Management App

### Audit Logging
Audit logging for budget and approval rule changes (create, edit, delete) is not in scope for the budget app migration. It needs a separate implementation — either server-side middleware on the Arak API or a dedicated audit service. No client-side audit logging exists in the current Appsmith app either.

### Budget Year-End Lifecycle
Unresolved: what happens to budgets at year-end? Do old-year budgets get archived, copied forward, or accumulate indefinitely in the list? This affects long-term UX of the budget list view. Needs a decision from the domain owner before the budget app grows historical data.

## AFC Tools App

### Dettaglio ordini — Codifica Quadrimestrale divergente (`cdlan_dur_rin` vs `cdlan_int_fatturazione`)
In Appsmith la label "Quadrimestrale" viene mappata al codice **4** per `cdlan_dur_rin` (durata rinnovo, widget-side) e al codice **5** per `cdlan_int_fatturazione` (intervallo di fatturazione, SQL-side). Non è chiaro se i due campi sul DB Vodka/daiquiri usino codifiche genuinamente diverse o se uno dei due sia un bug. Portato verbatim nella migrazione 1:1 — verificare con il dominio sales/fatturazione quale codifica è autoritativa e allineare se necessario.

### Dettaglio DDT per cespiti — Paginazione e filtri
La pagina `Report DDT per cespiti` esegue `SELECT * FROM Tsmi_DDT_Verifica_Cespiti` (MSSQL Alyante) senza WHERE, LIMIT o ORDER BY ad ogni caricamento. Oggi funziona ma è la pagina più fragile lato performance: al crescere della view porta a timeout. Portata verbatim nella migrazione 1:1 — un follow-up dovrà aggiungere paginazione server-side + filtri minimi (almeno date range e codice cespite) prima che il volume cresca.

## Reports App

### Accessi attivi — Carbone Template Update + Template ID Swap
The current Carbone XLSX template used by `Accessi attivi` is pinned to a field name with spaces (`stato grappa`), but Carbone does not accept JSON keys with spaces reliably in this flow. The follow-up task is: (a) update/upload the Carbone template to use a space-free key such as `stato_grappa`, then (b) replace the hardcoded `AccessiTemplateID` in the reports app/backend with the new Carbone template ID. Only after the template swap is complete should the temporary compatibility aliasing in the export payload be simplified or removed.

### Anomalie MOR — AI Analysis (Phase 2)
The Anomalie MOR page in Appsmith has a Tab 2 with AI-powered anomaly analysis via OpenRouter (model selector, prompt with 6 Italian-language validation rules, HTML output). This feature is deferred from the V1 migration. When implemented it needs: (a) backend proxy for OpenRouter calls (API key must not be in frontend), (b) AI validation prompt moved to backend (Go constant or config file), (c) proper Keycloak RBAC role (`app_reports_ai_access`) replacing the hardcoded email gate (`sciacco`), (d) model selector UI. The audit details are in `apps/reports/APPSMITH-AUDIT.md` §2.6 (BR8, BR10).

### AOV Calculation Inconsistency in `get_report_data_area`
The AOV page has 4 SQL queries sharing ~80% identical logic. The `get_report_data_area` query computes `valore_aov` as `(quantita * canone) * 12 + (quantita * setup)` regardless of `tipo_ordine`, while the other 3 queries subtract old MRC for substitutions (`tipo_ordine = 'A'`). This means the "per categoria" view may show different totals than the other views for the same data. Replicated as-is in the 1:1 migration from Appsmith. Needs review and correction post-coexistence.

### AOV Query Consolidation (Post-Coexistence)
The 4 AOV SQL queries (`get_report_data`, `get_report_data_tipo_ord`, `get_report_data_area`, `get_report_data_sales`) share ~80% identical SQL with different GROUP BY/SELECT. They are kept as 4 separate verbatim queries in the V1 backend to guarantee 1:1 correspondence with Appsmith and avoid LLM-introduced drift during migration. Consolidation into a single parameterized query or stored procedure should happen post-coexistence, validated by diffing query results against the original.

## Customer Portal Back-office App

### Expose `skip_keycloak` toggle in Nuovo Admin modal (hidden in source, omitted in port)
The Appsmith source carries a `new_user_skip_kc` SWITCH_WIDGET in the `Nuovo Admin` modal (Gestione Utenti page) wired to `skip_keycloak` on the `user-admin-new` DTO. Its DSL has `isVisible: false` and `defaultSwitchState: false` with no dynamic binding on either, so operators never see it and every admin creation sends `skip_keycloak: false`. The widget is kept in the source on purpose, so an Appsmith editor can flip `isVisible: true` the day the capability is needed without re-wiring the modal. The React port omits the switch entirely and pins `skip_keycloak: false` in request assembly, which matches observed operator behavior but loses the "flip one property to expose it" affordance Appsmith provided. Follow-up when product asks to surface the toggle: add a `ToggleSwitch` to the `Nuovo Admin` modal bound to a `skipKeycloak` form field, include it in the `createAdmin` request body, and verify Mistra NG enforces a server-side role check on `skip_keycloak=true` before shipping — if it does not, introduce a dedicated backend role (e.g. `app_cpbackoffice_admin_create_bypass_kc`) on the BFF that gates the field and silently forces `false` for callers without it.

### Accessi Biometrico — Label Polish After 1:1 Port
The Appsmith source uses raw lowercase labels in the biometric table (`nome`, `cognome`, `tipo_richiesta`, `stato_richiesta`, etc.). The v1 React port keeps those labels verbatim for strict parity, but the audit flags them as presentation debt. Post-port follow-up: update the visible labels to cleaner business Italian (for example `Nome`, `Cognome`, `Tipo richiesta`, `Stato`) without changing the DTO keys or table behavior.

### Accessi Biometrico — Defensive Ceiling / Filters
The biometric-request list is ported 1:1 with `ORDER BY data_richiesta DESC` and no pagination or filters. That matches the source, but it leaves the endpoint unbounded as volumes grow. Follow-up: decide the smallest safe mitigation for the backend and UI — server-side limit with truncation signal, minimal filters, or true pagination — and validate the chosen contract against operator workflow before shipping it as a post-v1 improvement.

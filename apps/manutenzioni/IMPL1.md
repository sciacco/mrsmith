# Manutenzioni Implementation Plan V1

Source: `apps/manutenzioni/PRD1.md`

Status: draft implementation plan for pre-gate review.

Checked against:
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/UI-UX.md`
- `docs/manutenzioni_schema.sql`
- `.agents/skills/portal-miniapp-generator/references/archetypes.md`
- `.agents/skills/portal-miniapp-generator/references/review-gates.md`

This document is the pre-code handoff for the Manutenzioni mini-app. It does not approve implementation by itself. Per the portal mini-app workflow, run `portal-miniapp-ui-review` before coding and again after implementation.

## Summary

- Product: Manutenzioni
- App slug: `manutenzioni`
- Frontend app path: `apps/manutenzioni`
- Production route: `/apps/manutenzioni/`
- Client default route: `/manutenzioni`
- API prefix: `/api/manutenzioni/v1/*`
- Backend package: `backend/internal/manutenzioni`
- Primary database: PostgreSQL DSN `MANUTENZIONI_DSN`, schema `maintenance`
- Secondary enrichment database: existing `MISTRA_DSN`, schema `customers`
- Vite port: `5188`
- Portal category: `smart-apps` / `SMART APPS`
- Portal icon: `wrench` (verified in `apps/portal/src/components/Icon/icons.tsx`)
- Access roles:
  - `app_manutenzioni_access`
  - `app_manutenzioni_operator` (operational write on maintenances + inline `service-taxonomy` enrichment; no configuration access)
  - `app_manutenzioni_manager` (operator superset + configuration access + `approve` action)
  - `app_manutenzioni_approver` (`approve` action + configuration access)
  - `app_devadmin` remains the shared superuser override through existing authz helpers.

V1 is an internal maintenance register and lifecycle workspace. It covers register, create/edit, detail management, maintenance windows, classification, targets, impacted customers, notice drafting/status tracking, timeline, and lookup/taxonomy configuration. It deliberately excludes customer-facing notices, automatic send jobs, AI generation, bulk import, CMDB links, dashboards, and schema changes.

## Comparable Apps Audit

### Reference 1 - `apps/richieste-fattibilita/src/pages/RequestListPage.tsx`

Why it is comparable:
- Operational register with URL-backed filters, pagination, refresh, empty/loading/error states, status pills, and role-gated row actions.
- Uses `SearchInput`, `MultiSelect`, `Pagination`, `Tooltip`, and `StatusPill` in a dense business workspace.

Patterns to reuse:
- Keep filters close to the table.
- Store register filters in URL search params.
- Use one table-first surface, not a dashboard.
- Use real domain status pills only.
- Provide explicit filtered-empty and load-error states.
- Gate manager actions with shared role helpers.

Patterns rejected:
- RDF-specific AI/PDF tabs and feasibility counters do not apply.
- Expanded row details should be limited; Manutenzioni detail deserves its own route because the aggregate has many child collections.

### Reference 2 - `apps/richieste-fattibilita/src/pages/RequestViewPage.tsx`

Why it is comparable:
- Read-oriented detail workspace with back navigation, status context, `TabNav`, tab panels, invalid ID handling, loading/error states, and manager shortcut.

Patterns to reuse:
- Detail route parses and validates a positive ID before fetching.
- Back navigation returns to the register.
- Header shows code/title/status plus compact context.
- Tabbed sections keep a large aggregate scannable.
- Manager-only actions appear in the detail header or section action areas.

Patterns rejected:
- PDF preview and AI analysis are out of scope.
- The detail should not show generated analysis or implementation-specific payloads.

### Reference 3 - `apps/budget/src/views/gruppi/GruppiPage.tsx`

Why it is comparable:
- Compact admin CRUD page with master list, detail panel, create/edit modal, delete confirmation, skeletons, and business empty states.

Patterns to reuse:
- Bounded create/edit forms in modals or drawers where the entity is small.
- Destructive confirmations for cancellation/removal flows.
- Clear no-selection prompts and empty states.

Patterns rejected:
- Side-by-side master/detail is too constrained for the main maintenance aggregate. Use it for configuration pages only where each resource is small.
- Delete as a default action is rejected; configuration values deactivate/reactivate in V1.

### Reference 4 - `apps/cp-backoffice/src/views/StatoAziende/StatoAziendePage.tsx`

Why it is comparable:
- Table-first admin surface with one primary action, neutral service-unavailable copy, search, row selection, and modal-backed mutation.

Patterns to reuse:
- Neutral failure copy that does not expose dependency names.
- Single primary action in the page header.
- Horizontally scrollable dense tables on narrow viewports.
- Search remains local only where the dataset is already loaded; Manutenzioni register filters stay backend-driven.

Patterns rejected:
- Single selected-row CTA is too narrow for the maintenance register, where each row needs open-detail and optional low-risk actions.

### Reference 5 - `apps/reports/src/pages/OrdiniPage.tsx`

Why it was inspected:
- It is a strong example of report/explorer composition and of what not to use for this CRUD-heavy app.

Rejected patterns:
- KPI row, summary cards, status breakdown chips, report preview/export composition, and report-style landing flow.

## Archetype Choice

- Selected archetype: `master_detail_crud`

Why it fits:
- The central entity is `maintenance.maintenance`.
- Users start from a filterable register, then open one maintenance detail to inspect and mutate child collections.
- Create/edit forms, child rows, configuration resources, and destructive confirmations are core to V1.
- The configuration pages are also registry CRUD screens with search, create, edit, deactivate, and reactivate.

Why not `data_workspace`:
- The app has multiple sections, but all are anchored to one selected maintenance entity. `data_workspace` would be broader than needed and could invite dashboards or loosely related panels.

Required UI states:
- Populated register.
- First-use empty register.
- Filtered empty register.
- Register load error.
- Detail loading.
- Detail invalid ID.
- Detail not found.
- Detail read-only.
- Detail manager editing.
- Dirty form / unsaved changes warning.
- Destructive confirmations for cancellation, reschedule replacement, removing targets/customers/classifications, and notice status changes where needed.
- Configuration list populated/empty/loading/error states.
- Narrow viewport with table horizontal scroll and detail tabs still usable.

## Product Scope

### In Scope

- Filterable and paginated maintenance register.
- Create draft maintenance records.
- Edit maintenance summary fields.
- Generate stable maintenance codes server-side when absent.
- Manage lifecycle transitions with approval before scheduling or announcement.
- Manage multiple maintenance windows, including reschedule, cancellation, and execution actuals.
- Manage service taxonomy, reason classes, impact effects, and quality flags.
- Manage affected targets and impacted customers.
- Enrich impacted customer names from `customers.customer` through `MISTRA_DSN` when available.
- Draft notices with Italian and English locale content.
- Manually move notice send status through the approved V1 workflow.
- Show a lifecycle timeline from `maintenance_event`.
- Manage lookup and taxonomy values from configuration pages.

### Out of Scope

- Customer-facing public maintenance page.
- Email, SMS, portal banner, or API broadcast delivery.
- AI generation, extraction, or recomputation jobs.
- CMDB implementation or deep links to products, orders, assets, circuits, or services.
- Bulk import UI.
- Dashboard or KPI landing page.
- Changes to `docs/manutenzioni_schema.sql`.
- New automated tests without explicit user approval, per repo rule.

## User Copy Rules

- Allowed copy style: `business-user-only`, primarily Italian.
- Do not expose implementation terms to users.
- Do not explain how the app is built.
- Do not surface raw machine values when a business label exists.

Preferred labels:
- `Manutenzioni`
- `Nuova manutenzione`
- `Registro`
- `Riepilogo`
- `Finestra`
- `Impatto`
- `Target`
- `Clienti impattati`
- `Comunicazioni`
- `Storico`
- `Configurazione`
- `Pianificata`
- `Approvata`
- `Annunciata`
- `In corso`
- `Completata`
- `Annullata`

Forbidden user-facing copy:
- `record`
- `datasource`
- `server-side`
- `inline update`
- `widget`
- `ref_table`
- `id.asc`
- `ai_extracted`
- `catalog_mapping`
- `MANUTENZIONI_DSN`
- `MISTRA_DSN`
- `Keycloak`
- `payload`

Source/confidence labels:
- `manual` -> `Manuale`
- `import` -> `Importazione`
- `rule` -> `Regola`
- `ai_extracted` or `ai` -> `AI`
- `catalog_mapping` -> `Catalogo`

Metrics policy:
- No dashboard metrics or KPI cards in V1.
- Allowed numeric UI: pagination total, downtime minutes, confidence percentage, child-row counts inside a selected maintenance detail when they clarify the current maintenance.

## Repo-Fit

### Frontend App

- Create `apps/manutenzioni/`.
- Package name: `mrsmith-manutenzioni`.
- Build base: `/apps/manutenzioni/`.
- Dev base: `/`.
- Vite dev port: `5188`.
- Vite proxies:
  - `/api` -> `process.env.VITE_DEV_BACKEND_URL || http://localhost:8080`
  - `/config` -> same target
- Dependencies should match `apps/richieste-fattibilita`:
  - `@mrsmith/api-client`
  - `@mrsmith/auth-client`
  - `@mrsmith/ui`
  - `@tanstack/react-query`
  - `react`
  - `react-dom`
  - `react-router-dom`

Client routes under the basename:
- index -> redirect to `/manutenzioni`
- `/manutenzioni`
- `/manutenzioni/new`
- `/manutenzioni/:id`
- `/manutenzioni/configurazione`
- `/manutenzioni/configurazione/:resource`

App shell:
- Use `AppShell`.
- Use `TabNavGroup` if grouping is useful:
  - `Registro`: `Manutenzioni`, `Nuova manutenzione`
  - `Gestione`: `Configurazione` (manager only)
- Use shared auth bootstrap from `/config` and fail closed until authenticated.
- Use `hasAnyRole` / `hasRole` from `@mrsmith/auth-client`; do not use raw role array checks.

### Backend Runtime

- Add package `backend/internal/manutenzioni`.
- Add config fields:
  - `ManutenzioniAppURL string` from `MANUTENZIONI_APP_URL`
  - `ManutenzioniDSN string` from `MANUTENZIONI_DSN`
- Extend default CORS origins with `http://localhost:5188`.
- In `backend/cmd/server/main.go`:
  - Open `MANUTENZIONI_DSN` with `database.New(database.Config{Driver: "postgres", DSN: cfg.ManutenzioniDSN})`.
  - Register routes with injected dependencies:
    - `Maintenance *sql.DB`
    - `Mistra *sql.DB`
    - `Logger *slog.Logger`
  - Add split-server href override to `http://localhost:5188` when `StaticDir == ""`.
  - Hide launcher tile when `MANUTENZIONI_DSN` is absent.
- Handlers must still return `503 manutenzioni_database_not_configured` if the route is hit directly while the DB handle is nil.
- Mistra enrichment is optional at runtime for detail/list views: if `MISTRA_DSN` is missing or enrichment fails, impacted customers render with customer ID fallback. Customer search endpoints return `503 customer_lookup_not_configured` when Mistra is missing.

### Launcher Catalog

Add to `backend/internal/platform/applaunch/catalog.go`:
- `ManutenzioniAppID = "manutenzioni"`
- `ManutenzioniAppHref = "/apps/manutenzioni/"`
- `manutenzioniAccessRoles = []string{"app_manutenzioni_access"}`
- `manutenzioniManagerRoles = []string{"app_manutenzioni_manager"}`
- `manutenzioniOperatorRoles = []string{"app_manutenzioni_operator"}`
- `manutenzioniApproverRoles = []string{"app_manutenzioni_approver"}`
- Helper methods:
  - `ManutenzioniAccessRoles()`
  - `ManutenzioniManagerRoles()`
  - `ManutenzioniOperatorRoles()`
  - `ManutenzioniApproverRoles()`
- Catalog definition:
  - ID `manutenzioni`
  - Name `Manutenzioni`
  - Icon `wrench`
  - Href `/apps/manutenzioni/`
  - Status `ready` once implemented
  - Category `smart-apps` / `SMART APPS`
  - Access roles `ManutenzioniAccessRoles()`

The commented placeholder currently using `/apps/smart-apps/manutenzioni` should be superseded by the active `/apps/manutenzioni/` entry.

### Dev and Deployment Wiring

Update:
- Root `package.json`:
  - add `dev:manutenzioni`
  - add the app to the root `dev` concurrently command
  - keep `--names`, `--prefix-colors`, and command count in lockstep
- `Makefile`:
  - add `dev-manutenzioni`
  - include it in `.PHONY`
- `docker-compose.dev.yaml`:
  - add a `manutenzioni` service on port `5188`
  - set `VITE_DEV_BACKEND_URL=http://backend:8080`
  - add a named `manutenzioni_node_modules` volume
- `deploy/Dockerfile`:
  - copy `/app/apps/manutenzioni/dist` to `/static/apps/manutenzioni`
- `deploy/k8s/deployment.yaml`:
  - add optional secret env `MANUTENZIONI_DSN` from `mrsmith-secrets`
- `deploy/k8s/configmap.yaml`:
  - add `MANUTENZIONI_APP_URL` only if a non-empty deployment override is needed; single-origin production can leave it empty.
- `backend/.env.example`:
  - document `MANUTENZIONI_DSN`
  - document `MANUTENZIONI_APP_URL`
  - add port `5188` to `CORS_ORIGINS`
- `.env.preprod.example`:
  - document `MANUTENZIONI_DSN`
  - document `MANUTENZIONI_APP_URL`

Static hosting must be checked end to end:
- Vite base `/apps/manutenzioni/`.
- Catalog href `/apps/manutenzioni/`.
- Docker copy `/static/apps/manutenzioni`.
- `staticspa` fallback supports `/apps/manutenzioni/manutenzioni/:id` browser refresh.

## Data Contract

### Source Tables

Primary schema: `maintenance` from `docs/manutenzioni_schema.sql`.

V1 reads/writes:
- `maintenance.maintenance`
- `maintenance.v_current_window`
- `maintenance.maintenance_window`
- `maintenance.maintenance_event`
- `maintenance.maintenance_service_taxonomy`
- `maintenance.maintenance_reason_class`
- `maintenance.maintenance_impact_effect`
- `maintenance.maintenance_quality_flag`
- `maintenance.maintenance_target`
- `maintenance.maintenance_impacted_customer`
- `maintenance.notice`
- `maintenance.notice_locale`
- `maintenance.notice_quality_flag`
- all lookup/taxonomy tables listed in the PRD.

Customer enrichment:
- `maintenance.maintenance_impacted_customer.customer_id` maps to `customers.customer.id`.
- `customers.customer` has `id integer`, `name varchar(255)`, `group_id`, and `state_id` in `docs/mistradb/mistra_customers.json`.
- Cross-DSN joins must be merged in Go, not one SQL query.

### Identifier Strategy

- Database PKs remain native identity IDs.
- `maintenance.maintenance.code` is generated server-side when missing.
- Proposed code format: `MNT-YYYY-000123`.
- Use the inserted row identity for the numeric suffix and the maintenance `created_at` year for `YYYY`.
- Create flow should run in one transaction:
  1. insert maintenance without code or with a manager-supplied correction code only if explicitly allowed by the endpoint;
  2. insert the first `maintenance_event` with `event_type = 'created'`;
  3. if code is empty, update the row to `MNT-<year>-<maintenance_id padded to 6>`.
- The browser must not generate codes.

### Actor and Owner Caveat

The schema has numeric `owner_admin_id`, `created_by_admin_id`, `updated_by_admin_id`, and event `actor_admin_id`, but current auth claims expose `Subject`, `Email`, `Name`, and roles, not a numeric admin ID.

V1 default:
- Use `auth.Claims.Email` and `auth.Claims.Subject` in `maintenance_event.payload` for audit readability.
- Leave numeric admin ID fields null unless a verified numeric admin/user mapping already exists in the environment.
- Do not expose a raw `owner_admin_id` input to users.

Pre-code verification:
- Confirm whether a stable numeric admin ID can be resolved from Keycloak/Mistra users by email.
- If not confirmed, owner selection is deferred or display-only for rows where `owner_admin_id` already exists. Adding an owner text/email column would require a schema change and is out of scope for this PRD.

### Register Query

Backend filtering must cover:
- free text: code, Italian title, English title, reason, residual service, target display name where practical
- status
- scheduled date range
- technical domain
- maintenance kind
- customer scope
- site
- page and page size

Ordering:
- Current/upcoming maintenance first by current window start.
- Then recent updates.
- Use `maintenance.v_current_window` for current window display.
- Include `window_status` because the view returns the highest `seq_no`, which may be `cancelled`, `superseded`, or `executed`.

Summary row fields:
- `maintenance_id`
- `code`
- title
- status
- kind/domain/customer-scope/site labels
- current window start/end/status
- expected downtime
- primary service taxonomy label or primary impact effect label
- notice status summary only when notices exist

### Detail Aggregate

`GET /maintenances/{id}` returns one aggregate:
- summary fields
- resolved lookup labels
- windows ordered by `seq_no desc`
- service taxonomy classifications
- reason classifications
- impact effects
- quality flags
- targets
- impacted customers with enriched `customer_name` when available
- notices with locales and quality flags
- timeline events ordered newest first or oldest first by UI decision; use a consistent order and document it in types

Nested-resource ownership must be checked on every child mutation:
- window belongs to maintenance
- classification belongs to maintenance
- target belongs to maintenance
- impacted customer row belongs to maintenance
- notice belongs to maintenance
- notice locale and notice quality flag belong to the notice under that maintenance

### Reference Data

Create/edit selectors:
- Default to active values.
- Include inactive values only when already selected by the maintenance being edited.

Reference-data endpoint:
- Provide grouped data for sites, domains, kinds, customer scopes, service taxonomy, reason classes, impact effects, quality flags, target types, and notice channels.
- Sort by `sort_order`, then Italian label, then ID.
- Support `maintenance_id` or explicit selected IDs so edit forms can include inactive selected values.

Configuration pages:
- Show active/inactive filter.
- Search by code, Italian label, English label, description, and domain where applicable.
- Create/update/deactivate/reactivate.
- No hard delete endpoint in V1.
- Code fields are immutable after creation by default.
- Validate code format `^[a-z][a-z0-9_]*$` for all tables with schema checks.
- `site.code` does not have the snake-case check in schema; validate non-empty uniqueness, preserve uppercase-friendly business codes such as `C21`.
- Deactivation is allowed even when referenced, but referenced values must remain visible on existing maintenances/notices.

### Lifecycle Rules

Statuses:
- `draft`
- `approved`
- `scheduled`
- `announced`
- `in_progress`
- `completed`
- `cancelled`
- `superseded`

Allowed V1 actions:
- `draft` -> approve, cancel
- `approved` -> schedule, announce, cancel
- `scheduled` -> announce, start, reschedule, cancel
- `announced` -> schedule if no current window exists, start if a current window exists, reschedule, cancel
- `in_progress` -> complete
- `completed` -> correction only by manager
- `cancelled` -> correction only by manager
- `superseded` -> read-only

Role rules:
- read: `app_manutenzioni_access`
- create/update and most lifecycle actions: `app_manutenzioni_operator` or `app_manutenzioni_manager`
- inline `service-taxonomy` create (catalog enrichment from maintenance editing): `app_manutenzioni_operator`, `app_manutenzioni_manager`, or `app_manutenzioni_approver`
- other configuration endpoints (`/llm-models`, `/service-dependencies`, `/config/*`): `app_manutenzioni_manager` or `app_manutenzioni_approver`
- approve: `app_manutenzioni_manager` or `app_manutenzioni_approver`
- schedule and announce must require the maintenance to already be approved; the actor performing schedule/announce can be an operator or manager, but the status must have passed through an approve action by a manager or approver.

Schema caveat:
- `maintenance_event.event_type` does not include `approved` or `scheduled`.
- Because schema changes are out of scope, record approval and scheduling as `event_type = 'updated'` with payload such as `{ "action": "approved" }` or `{ "action": "scheduled" }`.
- Use the dedicated event types that do exist for `created`, `classified`, `announced`, `rescheduled`, `cancelled`, `started`, `completed`, `analysis_enriched`, and `impact_recomputed`.

### Window Rules

- A maintenance may have multiple windows.
- `seq_no` increments per maintenance.
- Reschedule is transactional:
  - lock the maintenance and current/latest planned window;
  - set previous planned current window to `superseded`;
  - insert a new `maintenance_window` with `seq_no = max(seq_no) + 1`;
  - write `maintenance_event` with `event_type = 'rescheduled'`.
- Cancel window:
  - update `window_status = 'cancelled'`;
  - require Italian cancellation reason when the maintenance is already `scheduled` or `announced`;
  - write `maintenance_event` with `event_type = 'cancelled'`.
- Validate:
  - scheduled end after scheduled start;
  - actual end after actual start when both exist;
  - downtime minutes non-negative.
- Actual start/end/downtime are captured after execution and do not create a new window.

### Classification Rules

Classifications:
- service taxonomy
- reason classes
- impact effects
- quality flags

Rules:
- Source defaults to `manual`.
- Imported, rule, AI, and catalog rows are editable by managers.
- Confidence is `0..1` in storage and should render as a percentage when useful.
- Service taxonomy, reason class, and impact effect support one primary row each.
- The schema does not enforce only one primary row, so the backend must enforce it transactionally by clearing sibling primary flags before setting a new primary.
- Quality flags have no primary marker.
- Duplicate rows are prevented by schema unique constraints and should return a clear 409-style app error.
- A classification mutation writes `event_type = 'classified'` when it materially changes impact classification.

### Targets

Rules:
- Target type and display name are required.
- `ref_table`, `ref_id`, `external_key`, metadata, source, confidence, and primary marker are preserved.
- User-facing labels must say `Origine`, `Riferimento`, or another business label; do not show `ref_table`.
- Backend enforces one primary target per maintenance when `is_primary = true` is set.
- No links to product/order/asset/circuit/service detail pages in V1.

### Impacted Customers

Rules:
- `customer_id` is required and maps to `customers.customer.id`.
- Add customer by search when Mistra is available; allow manual numeric ID entry only if product accepts it during implementation review.
- Display `customer_name` from Mistra when available.
- Fall back to `Cliente #<id>` when enrichment is unavailable.
- Impact scope values:
  - `direct` -> `Diretto`
  - `indirect` -> `Indiretto`
  - `possible` -> `Possibile`
- Derivation source values:
  - `manual` -> `Manuale`
  - `rule` -> `Regola`
  - `ai` -> `AI`
  - `hybrid` -> `Ibrido`

### Notices

Notice rules:
- Required: notice type, audience, channel.
- Locale content:
  - external notices require both Italian and English content before leaving `draft`;
  - internal notices require Italian content, English optional.
- `sent` requires `sent_at`.
- Because V1 has no actual delivery, marking `sent` is a manual status update that sets or requires `sent_at`.
- No email/SMS/banner/API delivery clients are added.
- Quality flags can be attached to notices.
- Notice status updates must validate allowed status values and write timeline events when business-significant:
  - announcement notice ready/sent can write `announced` or `reminder_sent` where applicable;
  - cancellation notice status can be recorded in payload if no exact event type applies.

## API Plan

Routes are registered without `/api`; `backend/cmd/server/main.go` strips `/api` before dispatch.

### Read and Register

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `GET` | `/manutenzioni/v1/maintenances` | access | Paginated filtered register. |
| `GET` | `/manutenzioni/v1/maintenances/{id}` | access | Full aggregate. |
| `GET` | `/manutenzioni/v1/maintenances/{id}/events` | access | Optional standalone timeline if not embedded in detail. |
| `GET` | `/manutenzioni/v1/reference-data` | access | Grouped active references plus selected inactive values. |
| `GET` | `/manutenzioni/v1/customers` | manager | Mistra customer search for impacted-customer add flow. |

### Maintenance Core

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `POST` | `/manutenzioni/v1/maintenances` | manager | Create draft; optional first window and initial classifications/targets. |
| `PATCH` | `/manutenzioni/v1/maintenances/{id}` | manager | Edit summary fields. |
| `POST` | `/manutenzioni/v1/maintenances/{id}/status` | manager/approver by action | Body `{ action, reason_it?, reason_en? }`; approve action requires approver role. |

### Windows

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `POST` | `/manutenzioni/v1/maintenances/{id}/windows` | manager | Add first/additional planned window. |
| `PATCH` | `/manutenzioni/v1/maintenances/{id}/windows/{windowId}` | manager | Edit planned fields or actuals after ownership check. |
| `POST` | `/manutenzioni/v1/maintenances/{id}/windows/{windowId}/cancel` | manager | Cancel with reason rules. |
| `POST` | `/manutenzioni/v1/maintenances/{id}/windows/reschedule` | manager | Supersede current planned window and create next sequence. |

### Impact and Classification

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/service-taxonomy` | manager | Replace or upsert service classifications. |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/reason-classes` | manager | Replace or upsert reason classifications. |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/impact-effects` | manager | Replace or upsert impact effects. |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/quality-flags` | manager | Replace or upsert maintenance quality flags. |

### Targets and Customers

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `POST` | `/manutenzioni/v1/maintenances/{id}/targets` | manager | Add target. |
| `PATCH` | `/manutenzioni/v1/maintenances/{id}/targets/{targetId}` | manager | Edit target after ownership check. |
| `DELETE` | `/manutenzioni/v1/maintenances/{id}/targets/{targetId}` | manager | Remove target after confirmation. |
| `POST` | `/manutenzioni/v1/maintenances/{id}/impacted-customers` | manager | Add customer impact. |
| `PATCH` | `/manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}` | manager | Edit impact scope/source/reason. |
| `DELETE` | `/manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}` | manager | Remove impacted customer after confirmation. |

### Notices

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `POST` | `/manutenzioni/v1/maintenances/{id}/notices` | manager | Create notice with optional locale content. |
| `PATCH` | `/manutenzioni/v1/maintenances/{id}/notices/{noticeId}` | manager | Edit notice metadata. |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/notices/{noticeId}/locales/{locale}` | manager | Upsert `it` or `en` content. |
| `POST` | `/manutenzioni/v1/maintenances/{id}/notices/{noticeId}/status` | manager | Manual status update with V1 validation. |
| `PUT` | `/manutenzioni/v1/maintenances/{id}/notices/{noticeId}/quality-flags` | manager | Replace/upsert notice quality flags. |

### Configuration

| Method | Route | Role | Notes |
| --- | --- | --- | --- |
| `GET` | `/manutenzioni/v1/config/{resource}` | manager | List with search and active filter. |
| `POST` | `/manutenzioni/v1/config/{resource}` | manager | Create. |
| `PATCH` | `/manutenzioni/v1/config/{resource}/{id}` | manager | Update mutable fields. |
| `POST` | `/manutenzioni/v1/config/{resource}/{id}/deactivate` | manager | Soft deactivate. |
| `POST` | `/manutenzioni/v1/config/{resource}/{id}/reactivate` | manager | Reactivate. |

Configuration resources:
- `sites`
- `technical-domains`
- `maintenance-kinds`
- `customer-scopes`
- `reason-classes`
- `impact-effects`
- `quality-flags`
- `target-types`
- `notice-channels`
- `service-taxonomy`

## Backend Implementation Plan

Package layout:
- `backend/internal/manutenzioni/handler.go`
  - `Deps`
  - `Handler`
  - `RegisterRoutes`
  - role wrappers
  - dependency guards
- `backend/internal/manutenzioni/types.go`
  - response and request DTOs
  - enum constants
- `backend/internal/manutenzioni/errors.go`
  - app errors and status mapping
- `backend/internal/manutenzioni/db.go`
  - transaction helper, placeholder helper, scan helpers
- `backend/internal/manutenzioni/reference.go`
  - reference bundle and config resource metadata
- `backend/internal/manutenzioni/maintenances.go`
  - register, create, update, detail aggregate
- `backend/internal/manutenzioni/lifecycle.go`
  - transition matrix and status action handler
- `backend/internal/manutenzioni/windows.go`
  - window add/edit/cancel/reschedule logic
- `backend/internal/manutenzioni/classifications.go`
  - classification replace/upsert helpers
- `backend/internal/manutenzioni/targets.go`
  - target CRUD
- `backend/internal/manutenzioni/customers.go`
  - Mistra search and batch enrichment
- `backend/internal/manutenzioni/notices.go`
  - notices, locales, quality flags, status
- `backend/internal/manutenzioni/events.go`
  - event writing and timeline projection
- `backend/internal/manutenzioni/config.go`
  - config CRUD/deactivate/reactivate handlers

Handler conventions:
- Use dependency injection, not package globals.
- Use `httputil.InternalError` for internal 5xx failures.
- Include log fields:
  - `component = "manutenzioni"`
  - `operation`
  - `maintenance_id` when known
  - child IDs when known
- Client 5xx body remains sanitized as `internal_server_error`.
- Return business validation errors with stable error codes and 400/409/422 status where appropriate.
- Use `auth.GetClaims` only after ACL middleware has passed.
- Use `authz.HasAnyRole` for action-level role checks where one route supports manager/approver actions.

Transaction boundaries:
- Create maintenance + code generation + first event.
- Status transition + relevant child/window update + event.
- Reschedule previous window + insert new window + event.
- Notice status transition + locale validation + event.
- Classification update that changes primary flags.
- Config update where uniqueness/deactivation checks need consistency.

## Frontend Implementation Plan

Suggested file layout:
- `apps/manutenzioni/package.json`
- `apps/manutenzioni/vite.config.ts`
- `apps/manutenzioni/tsconfig.json`
- `apps/manutenzioni/index.html`
- `apps/manutenzioni/src/main.tsx`
- `apps/manutenzioni/src/App.tsx`
- `apps/manutenzioni/src/routes.tsx`
- `apps/manutenzioni/src/api/client.ts`
- `apps/manutenzioni/src/api/queries.ts`
- `apps/manutenzioni/src/api/types.ts`
- `apps/manutenzioni/src/hooks/useOptionalAuth.ts`
- `apps/manutenzioni/src/lib/format.ts`
- `apps/manutenzioni/src/lib/roles.ts`
- `apps/manutenzioni/src/styles/global.css`
- `apps/manutenzioni/src/pages/MaintenanceListPage.tsx`
- `apps/manutenzioni/src/pages/MaintenanceCreatePage.tsx`
- `apps/manutenzioni/src/pages/MaintenanceDetailPage.tsx`
- `apps/manutenzioni/src/pages/ConfigurationIndexPage.tsx`
- `apps/manutenzioni/src/pages/ConfigurationResourcePage.tsx`
- `apps/manutenzioni/src/components/StatusPill.tsx`
- `apps/manutenzioni/src/components/FilterBar.tsx`
- `apps/manutenzioni/src/components/Pagination.tsx`
- `apps/manutenzioni/src/components/ConfirmDialog.tsx`
- `apps/manutenzioni/src/components/forms/*`
- `apps/manutenzioni/src/components/detail/*`

Register page:
- Header: title, short business subtitle, `Aggiorna`, `Nuova manutenzione`.
- Filter bar:
  - search
  - status
  - scheduled date range
  - technical domain
  - kind
  - customer scope
  - site
  - clear filters
- Table columns:
  - codice
  - titolo
  - stato
  - dominio
  - tipo
  - ambito clienti
  - sito
  - finestra corrente
  - downtime previsto
  - impatto/servizio
  - comunicazioni
  - actions
- Preserve filters in URL search params.
- `Aggiorna` triggers React Query refetch.

Create page:
- Minimal required fields:
  - Italian title
  - maintenance kind
  - technical domain
- Customer scope is optional while the maintenance is a draft; approval and operational states require it.
- Optional sections can be collapsed:
  - English title
  - descriptions
  - site
  - reason
  - residual service
  - first planned window
  - initial service taxonomy/reasons/impacts/targets
- Submit creates a draft, shows a toast, and navigates to `/manutenzioni/:id`.

Detail page:
- Back link to register.
- Header:
  - code or fallback `MNT #<id>`
  - Italian title
  - status pill
  - primary manager actions
- Tabs:
  - `Riepilogo`
  - `Finestre`
  - `Impatto`
  - `Target`
  - `Clienti`
  - `Comunicazioni`
  - `Storico`
- Read-only users see the same data without edit controls.
- Managers see add/edit/remove actions.
- Approvers see approve action when status is `draft`.
- Use dirty-state guards in edit forms.

Configuration:
- Index page lists the ten configurable resources.
- Resource page uses compact table + drawer/modal form.
- Active/inactive segmented control.
- Create, edit, deactivate, reactivate.
- No hard delete button.

## Implementation Slices

### Slice 0 - Pre-Code Verification and UI Pre-Gate

- Confirm `MANUTENZIONI_DSN` points to a PostgreSQL database where `docs/manutenzioni_schema.sql` has been applied.
- Confirm whether numeric admin/user IDs can be resolved from current auth identity.
- Confirm Mistra customer search contract: `customers.customer.id` and `customers.customer.name`.
- Confirm acceptable behavior when `MISTRA_DSN` is absent: customer ID fallback in detail, customer search unavailable.
- Confirm whether manual customer ID entry is acceptable if Mistra search is unavailable.
- Run pre-gate UI review against this plan before coding.

### Slice 1 - Repo and Runtime Wiring

- Scaffold `apps/manutenzioni` with the standard Vite + React app shape.
- Add root dev scripts, Makefile target, docker-compose service, CORS port `5188`.
- Add config/env fields for `MANUTENZIONI_DSN` and `MANUTENZIONI_APP_URL`.
- Add launcher catalog constants, access role helpers, active catalog entry, and split-server override.
- Add Docker static copy path.
- Add K8s optional secret reference for `MANUTENZIONI_DSN`.

### Slice 2 - Backend Foundation

- Create `backend/internal/manutenzioni`.
- Implement `Deps`, route registration, role wrappers, dependency guards, error helpers, and shared DTOs.
- Implement reference-data list endpoint with active/default sorting.
- Implement customer enrichment helpers that batch query Mistra and merge in Go.

### Slice 3 - Register and Detail Read Model

- Implement `GET /maintenances` with backend filters, pagination, ordering, and summary projection.
- Implement `GET /maintenances/{id}` aggregate.
- Implement timeline projection.
- Implement frontend API client, query keys, types, register page, and detail read-only skeleton.

### Slice 4 - Create and Summary Editing

- Implement maintenance create transaction with code generation and created event.
- Implement summary patch endpoint with validation and updated event.
- Implement create page.
- Implement summary edit form in detail.

### Slice 5 - Lifecycle and Windows

- Implement lifecycle transition matrix and action endpoint.
- Implement approval role enforcement.
- Implement window add/edit/cancel/reschedule.
- Implement frontend detail actions and confirmations for approve, schedule, announce, start, complete, cancel, add window, reschedule.

### Slice 6 - Impact, Targets, and Customers

- Implement classification endpoints and primary-flag enforcement.
- Implement targets CRUD.
- Implement impacted customers CRUD and Mistra customer search.
- Implement corresponding detail tabs.

### Slice 7 - Notices and Timeline

- Implement notice create/edit/locales/status.
- Implement notice quality flags.
- Enforce bilingual external notices before leaving `draft`.
- Enforce `sent_at` for manual `sent`.
- Implement notices tab and timeline tab.

### Slice 8 - Configuration Pages

- Implement config resource metadata for all lookup/taxonomy tables.
- Implement list/create/update/deactivate/reactivate endpoints.
- Implement configuration index and resource pages.
- Ensure service taxonomy requires technical domain and displays domain label.

### Slice 9 - Polish, Build, and Post-Gate

- Review mobile/narrow viewport layouts.
- Check forbidden copy.
- Verify no dashboard metrics or decorative cards were introduced.
- Run approved build/test commands.
- Capture screenshots for populated, empty, error/destructive, and narrow states.
- Run post-implementation UI review.

## Exceptions

- The selected archetype is `master_detail_crud`, but the detail view uses tabs because the aggregate has many child collections. User benefit: avoids an overlong single page while keeping one selected maintenance as the context.
- Approval and scheduling are written as `maintenance_event.event_type = 'updated'` with action payloads because the schema does not include `approved` or `scheduled` event types and schema changes are out of scope.
- Numeric admin/owner IDs are not assumed. User identity is preserved in event payloads unless a verified numeric mapping exists.
- Mistra customer enrichment degrades to customer IDs in detail views. User benefit: maintenance records remain usable when enrichment is temporarily unavailable.
- No dashboard metrics. User benefit: operators land directly on the register and detail workspace.

## Verification

Automated tests are not to be added unless approved by the user. If approval is granted, prioritize only tests that protect business-critical rules or non-trivial data transformations.

### UI Review Checks

- Comparable apps cited here remain the design references.
- Primary archetype remains `master_detail_crud`.
- No hero, KPI row, marketing card, or launcher-style visual language.
- Copy remains business-facing Italian.
- No forbidden technical terms appear in UI copy.
- Screens to review:
  - populated register
  - filtered empty register
  - register error
  - detail read-only
  - detail manager edit
  - destructive confirmation
  - configuration resource page
  - narrow viewport

### Runtime and Auth Checks

- Portal tile is visible only to users with `app_manutenzioni_access` or `app_devadmin`.
- Backend routes return 401 without bearer auth.
- Backend routes return 403 without the required role.
- Operator endpoints reject access-only users.
- Configuration endpoints (`/config/*`, `/llm-models`, `/service-dependencies`) reject operator-only users (except the inline `POST /config/service-taxonomy` override).
- Approve action rejects operator-only users; both manager and approver can perform it.
- `/config` bootstrap works on port `5188`.
- `/api` calls use same-origin paths from the browser.
- Deep-link refresh works for `/apps/manutenzioni/manutenzioni/:id`.
- With `MANUTENZIONI_DSN` absent, tile is hidden and direct API route returns `503`.

### Data Checks

- Create generates `MNT-YYYY-000123` code from inserted identity.
- Create writes the first `created` event.
- Register filters match visible results.
- Inactive selected reference values remain visible on existing maintenance.
- Status transitions enforce the V1 matrix.
- Scheduling/announcement cannot happen before approval.
- Reschedule preserves the previous window as `superseded`.
- Cancellation reason is required for scheduled/announced maintenance.
- External notices cannot leave `draft` without Italian and English locale content.
- Notice cannot be marked `sent` without `sent_at`.
- Nested-resource ownership checks prevent cross-maintenance child mutation.
- Customer enrichment merges by `maintenance_impacted_customer.customer_id -> customers.customer.id`.

### Build and Manual Commands

Run after implementation:
- `pnpm --filter mrsmith-manutenzioni build`
- `pnpm --filter mrsmith-manutenzioni lint` if the package exposes the standard lint script.
- `go build ./cmd/server` from `backend`.
- Any automated tests only after user approval.

Manual verification:
- `make dev-manutenzioni` serves the app on port `5188`.
- `make dev` includes the app with backend and portal.
- `make dev-docker` exposes port `5188` if the docker-compose service is added.
- Browser network tab shows no direct DB, vendor API, email, SMS, or portal-banner calls from the frontend.

## Acceptance Checklist

- [ ] User with `app_manutenzioni_access` can open the app and view the register.
- [ ] User without the role cannot access backend data.
- [ ] Manager can create a draft maintenance.
- [ ] Manager can create a draft maintenance without customer scope.
- [ ] Code is generated server-side.
- [ ] Approver approval is required before schedule or announcement.
- [ ] Approval, scheduling, announcement, and start are blocked until customer scope is defined.
- [ ] Manager can add, reschedule, cancel, and complete windows with validation.
- [ ] Manager can manage classifications, targets, impacted customers, and notices.
- [ ] Read-only users see data without edit controls.
- [ ] External notices require Italian and English content before leaving draft.
- [ ] Customer names are enriched from Mistra when available.
- [ ] Configuration pages support create/edit/deactivate/reactivate for all V1 resources.
- [ ] Referenced configuration values are not hard-deleted in V1.
- [ ] No dashboard/KPI landing page exists.
- [ ] No browser code calls databases or delivery channels directly.
- [ ] Pre-gate and post-gate UI reviews are completed.

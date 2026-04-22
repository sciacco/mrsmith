# Manutenzioni - PRD V1

## Document Status

- Product: Manutenzioni
- App slug: `manutenzioni`
- Source data model: `docs/manutenzioni_schema.sql`
- Status: Draft V1 for product owner review
- Last updated: 2026-04-22

This PRD defines the first usable version of the technical maintenance management app. It is intentionally scoped to the existing `maintenance` schema and to the MrSmith mini-app conventions already used in the repo.

## Product Summary

Manutenzioni is an internal operations workspace for planning, classifying, tracking, and communicating technical maintenance activities.

V1 focuses on one central job: let authorized internal teams maintain a reliable register of technical maintenance events, including the maintenance window, operational impact, affected targets or customers, draft communications, and lifecycle history.

The app is not a customer portal in V1. It is the internal source of truth that can later feed customer-facing notices, automated email delivery, portal banners, incident correlation, or AI-assisted classification.

## Problem

Technical maintenance work involves several pieces of information that are easy to split across tickets, emails, spreadsheets, and manual reminders:

- what maintenance is planned or happening
- when the current execution window starts and ends
- which technical domain, service, site, and customer scope are involved
- what impact is expected
- which customers, orders, assets, circuits, or services may be affected
- which communications have been drafted, approved, sent, failed, or suppressed
- how the maintenance changed over time

The existing schema already models these concepts. V1 should expose them in a focused internal app without inventing a dashboard or unrelated metrics.

## Goals

- Provide a filterable maintenance register for operations users.
- Support creation and editing of maintenance records from the existing lookup tables.
- Manage one or more maintenance windows, including reschedules and cancellations.
- Capture service classification, reason classes, impact effects, quality flags, and affected targets.
- Track impacted customers when they are known or manually confirmed.
- Draft and review internal or external notices in Italian and English.
- Preserve lifecycle events so the team can answer "what changed, when, and by whom".
- Provide configuration pages for all maintenance lookup and taxonomy tables used by the app.
- Fit the existing MrSmith mini-app architecture, auth model, clean UI language, and static hosting pattern.

## Non-Goals for V1

- No customer-facing public maintenance page.
- No automatic email, SMS, portal banner, or API broadcast in V1.
- No AI generation, AI extraction, or impact recomputation job. The UI may display imported or AI-sourced rows already present in the database.
- No CMDB implementation. `maintenance_target` remains a generic target layer in V1.
- No dashboard/KPI landing page.
- No bulk import UI.
- No changes to `docs/manutenzioni_schema.sql` in this PRD.

## Primary Users

### Operations user

Needs to find current and upcoming maintenance, understand impact, and open details quickly. Can read the register, detail, windows, targets, impacted customers, notices, and history.

### Maintenance manager

Owns the maintenance lifecycle. Can create and update maintenance, classify it, manage windows, add targets and impacted customers, draft notices, change status, and cancel or supersede maintenance.

### Reviewer or approver

Reviews business-critical maintenance before communication. In V1 every maintenance requires explicit approval by a user with the approver role before it can be scheduled or announced.

## Role and Access Model

Default V1 roles:

- `app_manutenzioni_access`: read access to the app and all maintenance details.
- `app_manutenzioni_manager`: create/update access, lifecycle actions, window management, target/customer management, notice authoring, manual notice status updates, and lookup/taxonomy configuration.
- `app_manutenzioni_approver`: approve maintenance before scheduling or announcement.

V1 approval rule: all maintenance must be approved by `app_manutenzioni_approver`. V2 can refine which maintenance kinds require explicit approval.

## Data Model Scope

V1 uses these schema objects:

| Area | Tables or views | V1 use |
| --- | --- | --- |
| Core maintenance | `maintenance.maintenance`, `maintenance.v_current_window` | register, detail, status, owner, current window |
| Reference data | `site`, `technical_domain`, `maintenance_kind`, `customer_scope`, `reason_class`, `impact_effect`, `quality_flag`, `target_type`, `notice_channel`, `service_taxonomy` | selectors, labels, and V1 configuration pages |
| Classification | `maintenance_service_taxonomy`, `maintenance_reason_class`, `maintenance_impact_effect`, `maintenance_quality_flag` | multi-value classification with source/confidence |
| Windows | `maintenance_window` | planned, cancelled, superseded, executed windows |
| Lifecycle | `maintenance_event` | timeline and audit trail |
| Targets | `maintenance_target` | affected sites, services, products, platforms, assets, circuits, customers, orders, locations |
| Impacted customers | `maintenance_impacted_customer` | customer impact list and derivation metadata |
| Notices | `notice`, `notice_locale`, `notice_quality_flag` | communication drafts and send-status tracking |

## Product Assumptions

- Newly created maintenance should receive a stable human code. Proposed format: `MNT-YYYY-000123`, generated server-side from the inserted identity if `code` is empty.
- The UI language is Italian for user-facing labels because the seeded taxonomy labels are Italian-first. English fields exist for notice/customer communication content.
- `is_active = true` reference values are the default in create/edit selectors. Existing inactive values remain visible when already attached to a maintenance.
- `maintenance_impacted_customer.customer_id` stores `customers.customer.id` from Mistra. In this environment that value is the ERP company ID, not a Grappa internal customer ID.
- Customer names are enriched in V1 from Mistra `customers.customer`.
- Notices in V1 are drafted and tracked only. Actual sending is out of scope.
- External notices must have both Italian and English content before they can move out of `draft`.
- Imported or AI-sourced classifications are editable by managers in V1.
- Target references remain display-name only in V1; no links to product, order, asset, circuit, or service detail pages are required.
- Managers can configure all lookup and taxonomy tables in V1. Configuration changes should prefer deactivation over hard deletion when a value may already be referenced.

## V1 User Journeys

### 1. Review the maintenance register

As an operations user, I can open Manutenzioni and see a paginated list of maintenance ordered by the most relevant upcoming/current window first, then recent updates.

The list supports:

- search by code, Italian title, English title, reason, residual service, and target display name where practical
- status filter
- scheduled date range filter
- technical domain filter
- maintenance kind filter
- customer scope filter
- site filter
- quick refresh
- pagination

Primary row content:

- code or fallback `MNT #<id>`
- title
- status
- technical domain
- maintenance kind
- customer scope
- site
- current window start/end
- expected downtime
- primary service taxonomy or impact effect when present
- notice status summary, if notices exist

### 2. Create a maintenance

As a maintenance manager, I can create a draft maintenance with the minimum required business data.

Required fields:

- Italian title
- maintenance kind
- technical domain
- customer scope

Optional fields at creation:

- English title
- Italian and English description
- site
- Italian and English reason
- Italian and English residual service
- owner
- first planned window
- service taxonomy
- reason classes
- impact effects
- targets

Creation result:

- new row in `maintenance.maintenance`
- status defaults to `draft`
- first `maintenance_event` with `event_type = created`
- generated `code` if empty

### 3. Open and manage maintenance detail

As a user, I can open a maintenance and see the full operational context in one detail workspace.

Recommended detail sections:

- Summary: status, owner, kind, domain, customer scope, site, reason, residual service
- Windows: planned/current/past windows with reschedule and cancellation history
- Impact: service taxonomy, reason classes, impact effects, quality warnings
- Targets: affected business or technical objects
- Customers: impacted customer list with direct/indirect/possible scope
- Notices: internal/external communication drafts and send-status tracking
- Timeline: lifecycle events

Manager actions:

- edit summary fields
- change status through allowed lifecycle actions
- add/update/cancel/supersede windows
- add/remove classifications
- add/remove targets
- add/remove impacted customers
- create/update notices and locale content
- mark notice status manually according to approved V1 workflow

### 4. Manage windows and reschedules

As a maintenance manager, I can keep the schedule history intact when a maintenance changes.

Requirements:

- A maintenance may have multiple windows.
- The current window is the highest `seq_no` row returned by `maintenance.v_current_window`.
- Rescheduling creates a new `maintenance_window` with the next `seq_no`; the previous planned window becomes `superseded`.
- Cancelling a window requires an Italian cancellation reason when the maintenance was already scheduled or announced.
- Actual start/end/downtime can be captured after execution.
- Scheduled end must be after scheduled start; actual end must be after actual start when both are present.

### 5. Classify impact

As a maintenance manager, I can classify the maintenance using the seeded taxonomy and preserve how each classification was produced.

Requirements:

- Service taxonomy can include one or more services. One can be marked primary.
- Reason class can include one or more reasons. One can be marked primary.
- Impact effect can include one or more effects. One can be marked primary.
- Quality flags can be attached to maintenance and notices.
- Source and confidence are stored and visible in business language when useful:
  - Manuale
  - Importazione
  - Regola
  - AI
  - Catalogo

V1 does not create AI classifications. It only displays and lets managers correct data already present or manually entered.

### 6. Track targets and impacted customers

As a maintenance manager, I can record what is affected even when there is no single CMDB.

Target requirements:

- Target type is required.
- Display name is required.
- Optional reference fields are `ref_table`, `ref_id`, and `external_key`.
- Source, confidence, primary marker, and metadata are preserved.
- User-facing copy must avoid raw database labels where a business label exists.

Impacted customer requirements:

- Customer ID is required and comes from Mistra `customers.customer.id`.
- Customer names are displayed from Mistra `customers.customer`.
- Impact scope is one of direct, indirect, possible.
- Derivation source is one of manual, rule, AI, hybrid.
- Optional order ID, service ID, confidence, and reason can be captured.
- V1 can fall back to showing the customer ID if enrichment is temporarily unavailable.

### 7. Draft and track notices

As a maintenance manager, I can create notices tied to a maintenance and optionally to a window.

Notice requirements:

- Notice type is required: announcement, reminder, reschedule, cancellation, start, completion, internal update.
- Audience is required: internal or external.
- Channel is required from `notice_channel`.
- Send status is tracked: draft, ready, sent, failed, suppressed.
- Italian and English locale content are required for external notices.
- Internal notices should support both locales, but only Italian is required unless product policy changes.
- A notice cannot be marked `sent` without `sent_at`.
- Quality flags can be attached to notices.

V1 default behavior:

- Users author and review notices in the app.
- Users manually change send status.
- The app does not perform delivery to email, SMS, portal banner, or API.

### 8. Preserve lifecycle history

As a user, I can see a timeline of meaningful changes.

Events to write in V1:

- maintenance created
- maintenance classified
- maintenance announced
- reminder sent or marked sent
- window rescheduled
- maintenance cancelled
- maintenance started
- maintenance updated
- maintenance completed
- analysis enriched, if imported
- impact recomputed, if imported

The timeline should show event type, actor, time, summary, and a compact business-readable payload when available.

### 9. Configure lookup and taxonomy data

As a maintenance manager, I can manage the controlled vocabularies used by maintenance forms and filters.

Configuration pages required in V1:

- Sites
- Technical domains
- Maintenance kinds
- Customer scopes
- Reason classes
- Impact effects
- Quality flags
- Target types
- Notice channels
- Service taxonomy

Configuration requirements:

- Each page is a compact CRUD management surface with search, active/inactive filter, create, edit, and deactivate/reactivate actions.
- Hard delete is not a default V1 action. Values referenced by maintenance records, notices, targets, or classifications must be deactivated instead.
- Code fields remain immutable after creation unless implementation planning proves there are no dependent records and no audit risk.
- Tables with `code` checks must validate lowercase snake-case codes before save.
- Label fields use Italian as required and English as optional where the schema supports it.
- `sort_order`, `is_active`, `description`, `synonyms`, and `metadata` are editable where present.
- `service_taxonomy` must require a technical domain and show the domain label in the list.
- Deactivated values remain visible when already attached to existing maintenance or notices.

## Lifecycle Requirements

Schema statuses:

- `draft`
- `announced`
- `approved`
- `scheduled`
- `in_progress`
- `completed`
- `cancelled`
- `superseded`

Proposed V1 lifecycle actions:

| Current status | Allowed actions |
| --- | --- |
| `draft` | approve with `app_manutenzioni_approver`, cancel |
| `approved` | schedule, announce, cancel |
| `scheduled` | announce, start, reschedule, cancel |
| `announced` | schedule if no window exists, start if a current window exists, reschedule, cancel |
| `in_progress` | complete |
| `completed` | no standard mutation except correction by manager |
| `cancelled` | no standard mutation except correction by manager |
| `superseded` | read-only |

Approval must always come before scheduling or announcement. `announced` without a scheduled window is allowed only as a corner case for approved maintenance whose communication exists before the schedule is finalized.

## Functional Requirements

### Register

- Show filterable, paginated list.
- Preserve filters in URL search params.
- Use backend filtering for search, status, date range, site, domain, kind, and customer scope.
- Show loading, empty, error, and populated states.
- Include row actions: open detail, quick status action only if low-risk and role-gated.

### Detail

- Load full maintenance aggregate by ID.
- Show invalid ID and not-found states clearly.
- Separate read-only users from managers.
- Use dirty-state handling for editable forms before navigation away.
- Use confirm dialogs for cancellation, status rollback, deletion of target/customer rows, or replacing a current window.

### Reference Data

- Provide a single reference-data endpoint or grouped endpoints for active sites, kinds, domains, customer scopes, service taxonomy, reason classes, impact effects, quality flags, target types, and notice channels.
- Sort by `sort_order`, then label.
- Include inactive values only when already selected by an existing maintenance.
- Provide V1 configuration pages and manager-only API endpoints for all lookup and taxonomy tables.
- Support create, update, deactivate, and reactivate for each reference-data type.
- Preserve code uniqueness and schema code-format checks.
- Prevent hard deletion of referenced values in V1.

### Validation

- Required fields are validated in the frontend and backend.
- Date ranges must be valid in the backend.
- Enum values must be enforced in the backend.
- Nested ownership must be checked for child resources. For example, updating `/maintenances/{id}/windows/{windowId}` must verify that the window belongs to that maintenance.
- Deleting or removing child associations should be blocked for read-only users.

### Observability

- Backend errors returned to the client should be sanitized.
- Server logs should include request ID, component `manutenzioni`, operation, maintenance ID when known, and underlying DB error.
- Lifecycle events are product audit data and should not replace server logs.

## UI and UX Direction

### Comparable Apps Audit

Inspected repo screens:

- `apps/budget/src/views/gruppi/GruppiPage.tsx`
- `apps/richieste-fattibilita/src/pages/RequestListPage.tsx`
- `apps/richieste-fattibilita/src/pages/RequestViewPage.tsx`
- `apps/reports/src/pages/OrdiniPage.tsx`

Patterns to reuse:

- Compact page header with direct primary action.
- Filter bar close to the data table.
- URL-backed filters for operational lists.
- Status pills only for real domain statuses.
- Role-gated actions in the row/detail area.
- Empty, loading, and error states that describe the business problem.
- Detail workspace opened from a list, with clear back navigation.
- Tabs or section navigation inside the detail only after selecting a maintenance.

Patterns rejected for this app:

- Dashboard or KPI landing page from report-style screens.
- Full-width hero/banner layout.
- Metrics row used just to fill space.
- Report preview/export flow as the primary interaction.
- Customer selector-first layout from pricing apps, because maintenance is not scoped to one customer by default.

### Archetype Choice

Selected archetype: `master_detail_crud`.

Why it fits:

- The central entity is `maintenance.maintenance`.
- Users need a list, filters, detail, create/edit, child collections, and destructive confirmations.
- The detail may contain multiple sections, but each section belongs to one selected maintenance.
- Configuration pages also follow `master_detail_crud`: each lookup/taxonomy table is a small registry with list, search, create/edit form, and deactivate/reactivate confirmation.

Required UI states:

- populated register
- filtered empty register
- first-use empty register
- register load error
- detail loading
- detail not found
- detail read-only
- configuration list/detail states for every lookup and taxonomy table
- destructive confirmation
- narrow viewport with horizontally scrollable dense tables where needed

### User Copy Rules

Allowed copy style: business-user-only.

Preferred Italian labels:

- Manutenzioni
- Nuova manutenzione
- Finestra
- Impatto
- Cliente impattato
- Servizi coinvolti
- Motivo
- Comunicazioni
- Storico
- Avvisi qualita
- Pianificata
- In corso
- Completata
- Annullata

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
- explanations of how the screen is implemented

When source/confidence must be shown, translate values into business labels:

- `manual` -> Manuale
- `import` -> Importazione
- `rule` -> Regola
- `ai_extracted` or `ai` -> AI
- `catalog_mapping` -> Catalogo

### Metrics Policy

No dashboard metrics or KPI cards in V1.

Allowed numeric UI:

- pagination count
- expected and actual downtime
- confidence percentage where useful
- status or notice counts inside a row/detail only when they directly explain the selected maintenance

## Repo Fit

- Frontend app path: `apps/manutenzioni`
- Production route/base path: `/apps/manutenzioni/`
- Internal routes:
  - `/manutenzioni`
  - `/manutenzioni/new`
  - `/manutenzioni/:id`
  - `/manutenzioni/configurazione`
  - `/manutenzioni/configurazione/:resource`
- API prefix: `/api/manutenzioni/v1/...`
- Backend package: `backend/internal/manutenzioni`
- Vite port: `5188`
- Vite proxies: `/api` and `/config` to the backend
- Static deploy path: `/static/apps/manutenzioni`
- Portal catalog:
  - app ID `manutenzioni`
  - name `Manutenzioni`
  - icon `wrench`
  - category `SMART APPS`
  - href `/apps/manutenzioni/`
  - role `app_manutenzioni_access`

Database fit:

- The `maintenance` schema is accessed through a dedicated PostgreSQL DSN named `MANUTENZIONI_DSN`.
- The backend wires a separate `*sql.DB` for `MANUTENZIONI_DSN` and registers the module only when that connection is available.
- Customer-name enrichment reads Mistra `customers.customer` through the existing Mistra connection, so V1 uses both `MANUTENZIONI_DSN` and `MISTRA_DSN`.
- Deployment must add `MANUTENZIONI_DSN` to environment examples, Kubernetes secrets/config, and local dev configuration.

## Recommended API Shape

The exact OpenAPI contract can be written during implementation planning. V1 should include these stable resource groups:

- `GET /manutenzioni/v1/maintenances`
- `POST /manutenzioni/v1/maintenances`
- `GET /manutenzioni/v1/maintenances/{id}`
- `PATCH /manutenzioni/v1/maintenances/{id}`
- `POST /manutenzioni/v1/maintenances/{id}/status`
- `GET /manutenzioni/v1/reference-data`
- `POST /manutenzioni/v1/maintenances/{id}/windows`
- `PATCH /manutenzioni/v1/maintenances/{id}/windows/{windowId}`
- `POST /manutenzioni/v1/maintenances/{id}/classifications`
- `DELETE /manutenzioni/v1/maintenances/{id}/classifications/{classificationId}`
- `POST /manutenzioni/v1/maintenances/{id}/targets`
- `PATCH /manutenzioni/v1/maintenances/{id}/targets/{targetId}`
- `DELETE /manutenzioni/v1/maintenances/{id}/targets/{targetId}`
- `POST /manutenzioni/v1/maintenances/{id}/impacted-customers`
- `PATCH /manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}`
- `DELETE /manutenzioni/v1/maintenances/{id}/impacted-customers/{customerImpactId}`
- `POST /manutenzioni/v1/maintenances/{id}/notices`
- `PATCH /manutenzioni/v1/maintenances/{id}/notices/{noticeId}`
- `POST /manutenzioni/v1/maintenances/{id}/notices/{noticeId}/status`
- `GET /manutenzioni/v1/maintenances/{id}/events`
- `GET /manutenzioni/v1/config/{resource}`
- `POST /manutenzioni/v1/config/{resource}`
- `PATCH /manutenzioni/v1/config/{resource}/{id}`
- `POST /manutenzioni/v1/config/{resource}/{id}/deactivate`
- `POST /manutenzioni/v1/config/{resource}/{id}/reactivate`

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

Module routes are registered without `/api`; the server mounts them under `/api`.

## Acceptance Criteria

- A user with `app_manutenzioni_access` can open the app from the portal and view the register.
- A user without `app_manutenzioni_access` cannot access backend data.
- The register supports search, status, date, domain, kind, customer-scope, and site filters.
- Deep links to `/apps/manutenzioni/manutenzioni/:id` work after browser refresh in production.
- A manager can create a draft maintenance and receive a generated code.
- A maintenance cannot be scheduled or announced until a user with `app_manutenzioni_approver` approves it.
- A manager can add a valid planned window.
- A manager cannot save an invalid window where end is before start.
- A manager can reschedule by creating a new window while preserving the old one as superseded.
- A manager can cancel a scheduled or announced maintenance only with a cancellation reason.
- A manager can add service taxonomy, reason classes, impact effects, quality flags, targets, impacted customers, and notices.
- A manager can edit imported or AI-sourced classifications.
- A manager can manage sites, technical domains, maintenance kinds, customer scopes, reason classes, impact effects, quality flags, target types, notice channels, and service taxonomy from configuration pages.
- Referenced configuration values cannot be hard-deleted in V1 and can be deactivated instead.
- Deactivated configuration values remain visible on existing maintenance records that already use them.
- A read-only user sees the same detail data without edit controls.
- External notice locale content must be saved for both Italian and English before the notice leaves `draft`.
- Impacted customer names are enriched from Mistra `customers.customer` when available.
- No browser code calls a database, vendor API, email service, SMS service, or portal-banner service directly.

## Verification Guidance

UI review gates before implementation:

- Comparable apps cited above remain the design references.
- Primary archetype remains `master_detail_crud`.
- Copy stays business-facing.
- No dashboard metrics or decorative cards.
- Route, API prefix, role, Vite port, proxy, and deploy path are explicit.

Implementation verification, once approved:

- Manual: populated register, empty register, error state, detail view, manager edit, read-only user, configuration pages, destructive confirmations, mobile/narrow viewport.
- Runtime: auth role checks, deep-link refresh, `/config` bootstrap, `/api` same-origin calls.
- Data: nested-resource ownership checks, status transition validation, window date validation, reschedule history, notice locale uniqueness, reference-data uniqueness, and referenced-value deactivation behavior.
- Tests: do not add automated tests unless approved. Ask for tests only where they protect status transitions, nested ownership, generated codes, non-trivial query filters, or transaction rollback.

## Product Decisions Captured

1. V1 is internal planning and notice drafting only. No channel delivery is included.
2. Approval uses a separate `app_manutenzioni_approver` role. All V1 maintenance requires explicit approval.
3. The maintenance schema uses a dedicated `MANUTENZIONI_DSN`.
4. Impacted customers come from Mistra `customers.customer`; `maintenance_impacted_customer.customer_id` stores `customers.customer.id`.
5. Customer names are enriched in V1 from Mistra `customers.customer`.
6. Approval always comes first. `announced` without a schedule is allowed only after approval.
7. External notices are bilingual in V1.
8. Target display names are enough in V1; no detail-page links are required.
9. Imported or AI-sourced classifications are editable by managers.
10. V1 includes configuration pages for all maintenance lookup and taxonomy tables.

## Future Phases

- Automated notice delivery by channel.
- Customer-facing maintenance calendar or portal banner.
- Impact engine that derives impacted customers from targets.
- AI-assisted classification, quality checks, and notice drafting.
- Bulk import from existing maintenance sources.
- Exports and operational reporting.
- Links from targets to customer, order, product, asset, circuit, or service detail pages.

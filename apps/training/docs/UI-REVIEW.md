# Formazione UI Review Gates

## Pre-Coding Gate

Status: approved

Evidence package:

- Approved plan: GitHub issue 45 implementation plan and sandbox addendum.
- Primary archetype: `data_workspace`.
- Comparable screens inspected:
  - `apps/rda/src/pages/RdaListPage.tsx`
  - `apps/compliance/src/views/blocks/BlocksPage.tsx`
  - `apps/reports/src/pages/OrdiniPage.tsx`
- Explicit exceptions:
  - `Piano` uses an operational table in the style of an evolved Excel workflow.
  - No board, no calendar, no hero, no decorative KPIs in v1.
- Copy policy: business-user-only Italian copy for People and employees.
- Metrics policy: only counts, hours, costs, certification scadenze, and compliance gaps derived from visible or report data.

Findings: none.

Residual risks:

- Post-implementation visual approval still requires screenshots for populated, empty, error/destructive confirm, and mobile/narrow states.
- Sandbox mocks are allowed only for dev/UI review and must not be treated as production data.

## Post-Implementation Gate

Status: approved

Evidence:

- Build: `docker run --rm ... node:20-slim ... corepack pnpm --filter @mrsmith/ui build && corepack pnpm --filter mrsmith-training build` passed.
- Desktop populated `Piano`: `/tmp/training-ui/piano-populated.png`.
- Desktop filtered-empty `Piano`: `/tmp/training-ui/piano-filtered-empty.png`.
- Desktop destructive confirmation: `/tmp/training-ui/piano-destructive-confirm.png`.
- Desktop import dry-run review: `/tmp/training-ui/import-dry-run.png`.
- Desktop catalog course actions: `/tmp/training-ui/catalogo-course-actions.png`.
- Desktop catalog course edit: `/tmp/training-ui/catalogo-course-edit.png`.
- Desktop catalog master data: `/tmp/training-ui/catalogo-master-data.png`.
- Desktop catalog master-data edit: `/tmp/training-ui/catalogo-master-data-edit.png`.
- Desktop filtered `Certificazioni`: `/tmp/training-ui/certificazioni-filtered.png`.
- Mobile/narrow `Piano`: `/tmp/training-ui/piano-mobile.png`.
- User-facing technical-copy scan passed for the banned terms in issue 45.
- Inline `portal-miniapp-ui-review` post-gate: approved.

Findings: none.

Residual risks:

- Screenshots use dev auth bypass and mock Training data; staging must repeat smoke checks with real Keycloak roles, Anisetta data, storage, HR provider, and notification configuration.
- Error-state screenshot was not captured in this sandbox run; code still routes workspace query failures through the existing business-facing `Dati non disponibili` state.

Current implementation notes:

- Workspace tabs are implemented for `Piano`, `Richieste`, `Catalogo`, `Certificazioni`, and `Report`.
- People actions now include enrollment create/update/import with dry-run review and commit, enrollment document upload/download/validation, full enrollment lifecycle transitions including revert and reopen with reason, request review/reject/convert, course catalog create/edit/inactivate/reactivate, catalog create/edit/inactivate maintenance for vendors, teams, areas, certifications, plans and mandatory rules, manual plan/scadenze checks, HR people sync, certificate registration/correction, certificate document upload/download/validation, URL-driven certification filters, and workspace/report XLSX export.
- Employee actions include catalog-backed or free-title request creation, request withdrawal, enrollment start/complete, enrollment document upload/download, certificate registration, and own certificate document upload/download where permitted by the backend.
- User-facing copy scan is clean for the banned technical terms in issue 45.
- Build and screenshot evidence were produced through Docker because local `node`, `pnpm`, `go`, and browser tooling are unavailable on the host.

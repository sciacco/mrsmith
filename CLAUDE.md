@AGENTS.md

## Type-checking
- Always use `pnpm --filter <app> exec tsc --noEmit` to type-check, never bare `npx tsc`. The global TS is 4.x; workspaces depend on TS 5.x.

## Keycloak Roles
- Follow the naming convention `app_{appname}_access` for app-level access roles (e.g., `app_budget_access`, `app_compliance_access`).

## New App Checklist
When creating a new mini-app, these files MUST be updated in addition to the app code itself:
- `package.json` (root) — add to the `dev` concurrently command (name + color + filter) AND add a `dev:{appname}` script
- `Makefile` — add a `dev-{appname}` target AND add it to `.PHONY`
- `backend/internal/platform/applaunch/catalog.go` — add app ID/href constants, access roles, catalog entry
- `backend/cmd/server/main.go` — add import, hrefOverrides (dev port), catalog filter condition, RegisterRoutes call
- `backend/internal/platform/config/config.go` — add `{App}AppURL` field + env var

## Databases
- `docs/grappa/GRAPPA.md` — index for the Grappa MySQL schema dumps in `docs/grappa/`
- `docs/mistradb/MISTRA.md` — index for the Mistra PostgreSQL schema dumps in `docs/mistradb/`
- `docs/IMPLEMENTATION-KNOWLEDGE.md` — canonical handbook for reusable implementation discoveries, including the cross-database customer ID mapping (Alyante ERP ID = Mistra `customers.customer.id` = Grappa `cli_fatturazione.codice_aggancio_gest`; Grappa `cli_fatturazione.id` is a separate internal ID)

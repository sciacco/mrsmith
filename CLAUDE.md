@AGENTS.md

## Type-checking
- Always use `pnpm --filter <app> exec tsc --noEmit` to type-check, never bare `npx tsc`. The global TS is 4.x; workspaces depend on TS 5.x.

## Keycloak Roles
- Follow the naming convention `app_{appname}_access` for app-level access roles (e.g., `app_budget_access`, `app_compliance_access`).

## Databases
- `docs/grappa/GRAPPA.md` — index for the Grappa MySQL schema dumps in `docs/grappa/`
- `docs/mistradb/MISTRA.md` — index for the Mistra PostgreSQL schema dumps in `docs/mistradb/`
- `docs/IMPLEMENTATION-KNOWLEDGE.md` — canonical handbook for reusable implementation discoveries, including the cross-database customer ID mapping (Alyante ERP ID = Mistra `customers.customer.id` = Grappa `cli_fatturazione.codice_aggancio_gest`; Grappa `cli_fatturazione.id` is a separate internal ID)

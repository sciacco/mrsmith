@AGENTS.md

## Type-checking
- Always use `pnpm --filter <app> exec tsc --noEmit` to type-check, never bare `npx tsc`. The global TS is 4.x; workspaces depend on TS 5.x.

## Keycloak Roles
- Follow the naming convention `app_{appname}_access` for app-level access roles (e.g., `app_budget_access`, `app_compliance_access`).
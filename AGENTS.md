## Vision
[Project vision](docs/project_vision.md) — Matrix-themed portal launching corporate mini-apps with Stripe-level design

## Architecture
- **Monorepo** with pnpm workspaces (frontend) + Go backend
- `apps/` — independent Vite+React frontend apps (portal, future mini-apps)
- `packages/` — shared frontend libraries (`@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`)
- `backend/` — Go monolith with modular `internal/` packages per app
- `deploy/` — Dockerfile (multi-stage), K8s manifests (K8s NOT in use yet: config surface = `backend/.env` + `.env.example`; manifests are inert forward-prep)

## Important Reference
- `docs/mistra-dist.yaml` — authoritative Mistra NG Internal API spec; most mini-apps will integrate with these APIs, so use this file as the primary reference for backend contracts, client generation, and shared types.
- [`docs/IMPLEMENTATION-PLANNING.md`](docs/IMPLEMENTATION-PLANNING.md) — before approving an implementation plan, run the repo-fit checklist so hosting, auth, dev wiring, data contracts, and verification strategy are validated against the actual codebase.
- [`docs/MINI-APP-SCAFFOLDING.md`](docs/MINI-APP-SCAFFOLDING.md) — operational, file-by-file guide for adding a new mini-app to the portal; use it together with the canonical generator workflow.
- [`docs/IMPLEMENTATION-KNOWLEDGE.md`](docs/IMPLEMENTATION-KNOWLEDGE.md) — canonical handbook for reusable implementation discoveries (cross-system mappings, hidden rules, exclusions, quirks), including the cross-database customer ID mapping (Alyante ERP ID = Mistra `customers.customer.id` = Grappa `cli_fatturazione.codice_aggancio_gest`; Grappa `cli_fatturazione.id` is a separate internal ID). Read it during planning and update it when new reusable knowledge emerges.

## Databases
- `docs/grappa/GRAPPA.md` — index for the Grappa MySQL schema dumps in `docs/grappa/`
- `docs/mistradb/MISTRA.md` — index for the Mistra PostgreSQL schema dumps in `docs/mistradb/`

## Database safety (ABSOLUTE)
- NEVER run any operation — DDL, DML, migrations, `psql`, or even read-only queries — directly against a database whose connection string is configured in an env file (`backend/.env`: `ANISETTA_DSN`, `MISTRA_DSN`, `MANUTENZIONI_DSN`, grappa/alyante/coperture DSNs, etc.). Never connect with those DSNs and never offer to. They are shared staging/production databases owned by the team — a direct operation risks data loss, breaking colleagues, or an outage.
- Deliver schema changes only as migration `.sql` files in `deploy/migrations/` for the user to apply through their own process. For data inspection, ask the user to run the query and paste the result. Reading the `.env` file is fine; connecting to the database it points to is not.

## Dev
- `make dev` — runs air (Go hot reload) + Vite concurrently
- `make dev-docker` — same via docker-compose
- Backend proxy: Vite proxies `/api` → `localhost:8080`
- Auth: OAuth2/OIDC with remote Keycloak (no local instance)
- When sandbox tooling is missing, use [`docs/TOOLING-WITH-DOCKER.md`](docs/TOOLING-WITH-DOCKER.md) to run Go/Node/Playwright checks through Docker with minimal approval prompts.
- Before running Playwright, browser checks, or similar UI tests, first check whether `make dev` or the relevant Vite dev server is already running and reuse that URL. Do not start a second dev/preview server unless no suitable server is active.

## Type-checking
- Always use `pnpm --filter <app> exec tsc --noEmit` to type-check, never bare `npx tsc`. The global TS is 4.x; workspaces depend on TS 5.x.

## Temp files
- Write all temporary files (screenshots, scratch outputs, intermediate artifacts) under `artifacts/{agent}/` at the repo root, where `{agent}` is the harness name (e.g. `artifacts/claude/`, `artifacts/codex/`). Never use `/tmp` or other paths. The whole `artifacts/` directory is gitignored.

## UI/UX
- [`docs/UI-UX.md`](docs/UI-UX.md) — Mandatory reference for all UI, frontend, and mini-app work. Agents must read it before planning or implementing UI changes and treat it as the canonical design-system source unless the user explicitly overrides it.
- For any new portal mini-app or mini-app UI review, use `.agents/skills/portal-miniapp-generator/` as the canonical workflow.
- For scoped UI/styling work on an existing mini-app (a screen, a component, a table/form/drawer), use the `tintoretto` skill (`.agents/skills/tintoretto/`) — self-contained: it applies `docs/UI-UX.md`, embeds the visual-design and copy craft, and mandates the type-check + smoke-test verification.

## Keycloak Roles
- Follow the naming convention `app_{appname}_access` for app-level access roles (e.g., `app_budget_access`, `app_compliance_access`).

## New App Checklist
When adding a new mini-app, follow the complete operational guide in `docs/MINI-APP-SCAFFOLDING.md` — including its final file checklist (vite config, root `package.json`, `Makefile`, `catalog.go`, `config.go`, `main.go`). The app code alone is never sufficient.

## TODOs
- [`docs/TODO.md`](docs/TODO.md) — project-wide open items, deferred decisions, and out-of-scope work tracked for future implementation

## Test Rule
Don't add tests unless approved by the user. Ask to add tests only when they protect a reproduced bug, a business-critical rule, or non-trivial query/data transformation.
Do not add tests for routine UI copy changes, obvious wiring, low-risk refactors, or speculative regressions unless explicitly requested.

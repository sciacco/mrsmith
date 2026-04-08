# Implementation Plan Prompt — Listini e Sconti

Use this prompt with an LLM that has full access to the mrsmith repository.

---

## Prompt

You are a senior software architect writing a detailed implementation plan for a new mini-app in the mrsmith monorepo. The plan must be written to `apps/listini-e-sconti/LISTINI-IMPL.md`.

### Context

Read these files carefully before starting:

**Specification (your primary input):**
- `apps/listini-e-sconti/SPEC.md` — the complete, approved application specification with all entities, views, API endpoints, logic placement, integrations, validation rules, and design decisions. Every question has been resolved. This is the single source of truth for what to build.

**Repo conventions and planning rules:**
- `CLAUDE.md` and `AGENTS.md` — project-wide conventions (TypeScript version, Keycloak roles, monorepo structure, dev workflow)
- `docs/IMPLEMENTATION-PLANNING.md` — mandatory planning checklist and heuristics. Your plan MUST pass the 6-point Repo-Fit Checklist (Runtime, Dev, Auth, Data-Contract, Deployment, Verification). Do not skip any section.
- `docs/IMPLEMENTATION-KNOWLEDGE.md` — reusable cross-system knowledge: customer identity mapping, HubSpot lookup path, exclusion codes. Reference these discoveries directly instead of restating them.
- `docs/UI-UX.md` — design system reference (clean theme, typography, spacing, components, animation patterns). The app uses the clean theme.

**Prior implementation plans (follow these as structural templates):**
- `apps/kit-products/IMPLEMENTATION-PLAN.md` — the most complete and recent plan. It covers dual-database wiring (Mistra PG + Alyante MSSQL), dependency injection, ERP best-effort patterns, and phased implementation. Use its structure, level of detail, and repo-fit checklist format as your template.
- `apps/compliance/COMP-IMPL-V1.md` — another reference plan showing resolved decisions, phase structure, and verification strategy.

**Existing backend code (verify against, do not guess):**
- `backend/cmd/server/main.go` — see how existing apps are wired (RegisterRoutes, DB connections, catalog entries)
- `backend/internal/platform/config/config.go` — existing DSNs: `MISTRA_DSN` (already exists), `ANISETTA_DSN`, `ALYANTE_DSN`. Note: `GRAPPA_DSN` does NOT exist yet and must be added.
- `backend/internal/platform/applaunch/catalog.go` — the catalog entry for `listini-e-sconti` already exists (ID: `listini-e-sconti`, href: `/apps/mkt-sales/listini-e-sconti`, category: `mkt-sales`). It needs to be updated with a dedicated access role and correct href.
- `backend/internal/kitproducts/` — reference for Mistra DB access patterns (stored procedures, transactions, handler structure)
- `backend/internal/compliance/` — reference for single-DB module structure
- `packages/ui/src/components/` — existing shared components (AppShell, TabNav, Modal, Toast, MultiSelect, SingleSelect, Skeleton, ToggleSwitch, SearchInput, TableToolbar)

**Existing frontend apps (verify patterns):**
- `apps/budget/` — reference for auth bootstrap, API client usage, TanStack Query patterns
- `apps/kit-products/` — reference for Mistra DB access from frontend, tab navigation
- `apps/compliance/` — reference for master-detail layout, export patterns

### What makes this app unique

1. **Dual database:** This app queries BOTH Mistra (PostgreSQL, via existing `MISTRA_DSN`) AND Grappa (MySQL, new `GRAPPA_DSN`). No other app in the repo currently uses Grappa. You must plan the MySQL driver addition, DSN wiring, and dual-DB handler injection.

2. **HubSpot integration:** Three pages create audit notes/tasks on HubSpot after saves. This requires a new `hubspot` service in the backend that performs a two-step cross-database company lookup (Grappa ID → ERP ID via `cli_fatturazione.codice_aggancio_gest` → HubSpot ID via `loader.hubs_company`). HubSpot calls are async and non-blocking — failures are tolerated and logged.

3. **Carbone PDF generation:** The Kit page generates PDFs via Carbone API. Template ID is hardcoded in code for now.

4. **Grouped navigation:** The app has 7 pages — too many for flat `TabNav`. A new `TabNavGroup` component must be created in `packages/ui/` that supports grouped horizontal tabs with dropdown menus on hover. Groups: Catalogo (1 page), Prezzi (2), Sconti (2), Crediti (2). Single-page groups navigate directly on click.

5. **Kit card view:** The Kit di vendita page is NOT a simple table. It must be redesigned as a digital data sheet mirroring the printed PDF layout (see `artifacts/kit Unbreakable CORE.pdf`): master list left, detail card right with header, metadata grid, notes, grouped product table, and actions.

6. **Coexistence:** The app coexists with Appsmith during transition. Both access the same databases. Exclusion codes (385, 485) are hardcoded to match Appsmith behavior. No schema changes allowed.

### Plan requirements

Write the plan with the EXACT structure used in `apps/kit-products/IMPLEMENTATION-PLAN.md`:

1. **Header** with spec source, date, status
2. **Repo-Fit Checklist** (all 6 sections from `docs/IMPLEMENTATION-PLANNING.md`):
   - Runtime Fit (route, base path, deep links, dev split-server, catalog entry)
   - Dev Fit (Vite port, API proxy, root scripts, Makefile, CORS, Docker compose)
   - Auth Fit (Keycloak role, Bearer auth, 401/403, frontend auth)
   - Data-Contract Fit (PKs, identifier strategy, active-only defaults, nested resource checks)
   - Deployment Fit (Dockerfile, env vars, DB drivers, migration story)
   - Verification Fit (transactions, deep-link refresh, integration failure, logging, panic recovery, error sanitization)
3. **Review Findings** for anything that needs special attention (dual DB, HubSpot service, TabNavGroup component, MySQL driver)
4. **Implementation Sequence** broken into phases with:
   - Each phase has a clear goal
   - Every file to create or modify is listed explicitly
   - SQL queries are written out (not just described)
   - Frontend components are specified with their props and data bindings
   - Verification criteria at the end of each phase
   - Dependencies between phases are explicit

### Phase structure guidance

Suggested phases (adjust if needed):

- **Phase 1 — Scaffolding:** Frontend app, backend module, Grappa DSN wiring, MySQL driver, HubSpot service skeleton, TabNavGroup component, infra wiring
- **Phase 2 — Kit catalog:** Backend kit endpoints (list, products, help, PDF), frontend Kit card view with master-detail layout
- **Phase 3 — IaaS Pricing + Timoo Pricing:** Backend pricing endpoints (Grappa + Mistra), frontend forms with validation, price fallback logic
- **Phase 4 — IaaS Credits + Rack Discounts:** Backend batch endpoints, frontend inline-edit tables, HubSpot audit integration wired end-to-end
- **Phase 5 — Customer Groups + Customer Credits:** Backend group sync + credit ledger endpoints, frontend views
- **Phase 6 — Integration & Polish:** End-to-end testing, HubSpot side-effects verified, coexistence validation, accessibility review

### Critical details to include

- The Vite port must be unique (next available after existing apps)
- The Grappa MySQL connection needs a Go MySQL driver (`github.com/go-sql-driver/mysql`) — verify it's not already in go.mod
- The HubSpot service needs an API key from env var (`HUBSPOT_API_KEY`) and the company lookup is cross-database (Grappa → Mistra)
- The Carbone service needs an API key from env var (`CARBONE_API_KEY`) and a template ID constant
- All 22 API endpoints from the spec must be mapped to handler functions with their SQL
- Backend validation rules from the spec (IaaS price ranges, discount 0-20%, credit amount 0-10000, etc.) must be explicitly listed in handler implementations
- The `TabNavGroup` component must be specified with props, CSS, and behavior (hover opens dropdown, click on single-page group navigates directly)
- The Kit card view layout must be specified in detail (sections, components, data flow)

### Output format

Write the complete plan to `apps/listini-e-sconti/LISTINI-IMPL.md`. Use markdown with tables, code blocks for Go structs and SQL, and ASCII diagrams where helpful. The plan must be detailed enough that an implementer can work phase-by-phase without consulting the original Appsmith export.

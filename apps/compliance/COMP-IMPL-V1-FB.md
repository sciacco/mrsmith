# Compliance App — Implementation Plan V1.2 Feedback

Date: 2026-04-07
Reviewed against:
- `docs/IMPLEMENTATION-PLANNING.md`
- current repo runtime/dev/deploy wiring
- `apps/compliance/compliance-migspec.md`
- `apps/compliance/compliance-migspec-phaseD.md`

## Verdict

This revision fixes several important issues from the earlier draft: the SPA path now matches the static host, export no longer relies on unauthenticated browser navigation, the plan adopts dependency injection for the DB-backed handlers, and the auth-response nuance is correctly narrowed to middleware vs handler-owned errors.

It is closer, but it is still **not fully implementation-ready**. The main remaining problems are execution fit, not product scope. The biggest gap is that the plan introduces a brand-new PostgreSQL dependency without turning that into an executable dev/preprod/test contract.

## Findings

### 1. Blocker — the database/dev/test/deploy contract is still not executable

References:
- `apps/compliance/COMP-IMPL-V1.md`: 1.1 DB wiring and manual migration (`lines 54-76`)
- `apps/compliance/COMP-IMPL-V1.md`: handler tests are still "test DB or mock" (`lines 212-220`)
- `apps/compliance/COMP-IMPL-V1.md`: K8s-only DSN wiring (`lines 339-352`)
- `apps/compliance/COMP-IMPL-V1.md`: verification expects `make dev-docker`, manual create flows, and backend tests to work (`lines 747-760`)
- current repo preprod contract is `docker-compose.preprod.yaml` + `.env.preprod.example`, but neither currently carries `ANISETTA_DSN`:
  - `docker-compose.preprod.yaml` (`lines 1-10`)
  - `.env.preprod.example` (`lines 5-21`)

Why this matters:
- Budget can run locally without extra infrastructure because it has fixtures and an optional upstream proxy. Compliance is the opposite: it is DB-backed from day one.
- The plan adds `ANISETTA_DSN`, but it does not define how developers, CI, or preprod obtain a compatible Postgres instance with the required schema and the `is_active` migration applied.
- "test DB or mock" is not a real verification strategy for SQL-heavy handlers. Until the plan chooses one, the test section is still aspirational.
- `make dev-docker` is called out in AGENTS and in the plan, but the proposed docker-compose change only adds the frontend container. It does not make the backend functional for compliance unless an external DSN is already present and reachable.

Required change:
- Pick one concrete strategy and document it end-to-end:
  1. Add a local Postgres service for dev/test, with schema bootstrap instructions and migration application, or
  2. Explicitly require an external shared `ANISETTA_DSN` for dev/preprod and update all repo entry points/docs to match.
- Make the test plan choose a real handler-testing approach:
  - preferred: integration tests against Postgres (ephemeral DB/container or dedicated DSN)
  - acceptable only if truly unavoidable: query layer abstraction plus narrower unit tests
- Update the repo-level runtime docs/configs that operators will actually use:
  - `.env.preprod.example`
  - any preprod rollout notes
  - deployment notes for applying `001_add_is_active.sql`

### 2. High — the React Query key design is wrong for `useOrigins(includeInactive?)`

References:
- proposed query keys: `apps/compliance/COMP-IMPL-V1.md` (`lines 392-405`)
- proposed origins hook usage: `apps/compliance/COMP-IMPL-V1.md` (`lines 502-512`)

Why this matters:
- The plan defines one cache key, `['compliance', 'origins']`, but also defines two different datasets:
  - creation dropdown: active-only
  - management page: `include_inactive=true`
- Those are not the same resource. Reusing one key will cause cache pollution and stale UI behavior.

Required change:
- Parameterize the key, e.g. `origins(includeInactive: boolean)`.
- Use that keyed form consistently for reads and invalidation.

### 3. High — inactive origins are handled for creation and management, but not for editing historical block records

References:
- active-only default for `GET /origins`: `apps/compliance/COMP-IMPL-V1.md` (`lines 201-205`)
- block edit/detail form exists: `apps/compliance/COMP-IMPL-V1.md` (`lines 463-470`)
- create modal explicitly uses active origins only: `apps/compliance/COMP-IMPL-V1.md` (`lines 465-466`)
- management page uses `include_inactive=true`: `apps/compliance/COMP-IMPL-V1.md` (`lines 506-512`)

Why this matters:
- Historical block requests can legitimately reference a soft-deleted origin.
- The plan is explicit about active-only for creation and include-inactive for management, but it does not define how the block edit form behaves when the current `method_id` is inactive.
- Without that rule, the edit form can render an empty select, force an unintended origin change, or fail validation on a previously valid record.

Required change:
- Define edit-mode behavior explicitly. One of these needs to be in the plan:
  - edit forms fetch `include_inactive=true`, or
  - edit forms fetch active-only plus inject the current inactive origin as a locked/displayable option.
- Also decide whether changing a historical request from an inactive origin to a new origin is allowed or intentionally blocked.

### 4. Medium — Phases 1 and 2 are not actually parallel in the current plan

References:
- Phase 1 modifies `backend/internal/platform/config/config.go` and `backend/cmd/server/main.go`: `apps/compliance/COMP-IMPL-V1.md` (`lines 54-64`)
- Phase 2 modifies the same files again: `apps/compliance/COMP-IMPL-V1.md` (`lines 304-316`)
- dependency graph still presents Phase 1 and Phase 2 as parallel tracks: `apps/compliance/COMP-IMPL-V1.md` (`lines 705-713`)
- current single points of change:
  - `backend/internal/platform/config/config.go` (`lines 5-42`)
  - `backend/cmd/server/main.go` (`lines 24-89`)

Why this matters:
- The plan claims Phase 1 and Phase 2 can run in parallel, but they share the exact same backend files.
- That creates merge conflicts and sequencing ambiguity if multiple contributors follow the plan literally.

Required change:
- Move all `config.go` and `main.go` changes into one phase/owner.
- Keep the frontend scaffold parallel only where the write set is actually disjoint.

### 5. Medium — the planned server-level auth test is blocked by current `main.go` structure

References:
- planned test: `apps/compliance/COMP-IMPL-V1.md` (`lines 578-583`)
- current server bootstrap is embedded directly in `main()`: `backend/cmd/server/main.go` (`lines 24-117`)

Why this matters:
- The plan wants a higher-level test of the real auth middleware stack, which is a good goal.
- The current code does not expose a reusable `buildMux`/`newServer` function. Everything is wired directly inside `main()`.
- Without adding that refactor to the plan, this test will either be awkward or get skipped.

Required change:
- Add an explicit pre-step:
  - extract server construction into a testable function that returns the mux or server
  - then write the auth integration test against that function

### 6. Medium — the frontend scaffold is missing `src/vite-env.d.ts`

References:
- compliance scaffold list: `apps/compliance/COMP-IMPL-V1.md` (`lines 293-294`, `356-374`, `627-678`)
- existing apps include Vite env typing:
  - `apps/budget/src/vite-env.d.ts` (`lines 1-5`)
  - `apps/portal/src/vite-env.d.ts` (`line 1`)
- compliance `main.tsx` is planned to use `import.meta.env.BASE_URL`, matching budget's pattern:
  - `apps/budget/src/main.tsx` (`lines 11-14`)

Why this matters:
- If the new app follows the budget bootstrap, it will use Vite env types and CSS Modules.
- The plan does not list the corresponding declaration file, so the type-check section is incomplete.

Required change:
- Add `apps/compliance/src/vite-env.d.ts` to the file inventory and scaffold phase.

### 7. Medium — spec reconciliation is still incomplete; the plan only fixes one of the drifting sources

References:
- Phase 0 only names `apps/compliance/compliance-migspec.md`: `apps/compliance/COMP-IMPL-V1.md` (`lines 39-44`)
- main spec still has stale origins/auth contracts:
  - `apps/compliance/compliance-migspec.md` (`lines 283-295`)
- Phase D spec also still has stale origins contracts and contradictory table-ownership wording:
  - `apps/compliance/compliance-migspec-phaseD.md` (`lines 13-16`)
  - `apps/compliance/compliance-migspec-phaseD.md` (`lines 24-27`)
  - `apps/compliance/compliance-migspec-phaseD.md` (`lines 96-100`)

Why this matters:
- The plan already recognizes that plan and source spec must agree.
- Right now there are still multiple sources in `apps/compliance/` that disagree on:
  - `POST /origins` body
  - `DELETE /origins` response
  - whether 401/403 are JSON
  - whether DNS sync jobs are read-only consumers or write alongside the app

Required change:
- Reconcile all documents that are still being treated as living references, not just the top-level assembled spec.
- If only one file is canonical going forward, state that explicitly in the plan and mark the phase docs as historical.

### 8. Medium — the `/domains` endpoint contract is still underspecified for implementation

References:
- handler plan: `apps/compliance/COMP-IMPL-V1.md` (`lines 174-184`)
- API contract shape in Phase D: `apps/compliance/compliance-migspec-phaseD.md` (`lines 85-90`)

Why this matters:
- The plan says `/compliance/domains` supports blocked/released behavior, export, and search, but it never locks down:
  - whether `status` is required
  - what happens when `status` is missing or invalid
  - whether `?search=` is supported for JSON list responses, export responses, or both
- That ambiguity is small in prose but expensive in implementation because frontend hooks, query keys, and tests all depend on it.

Required change:
- Document the exact contract, for example:
  - `GET /api/compliance/domains?status=blocked|released`
  - missing/invalid `status` => `400 validation_error`
  - `search` is accepted for both JSON and export so visible rows and downloaded rows stay aligned

## What Is Solid

- Runtime pathing is now aligned with the existing static SPA host model.
- Authenticated export via blob download is the correct approach for this repo's Bearer-token transport.
- The move to injected `*sql.DB` handlers is the right backend shape.
- Role/catalog coverage and static deep-link verification are appropriately called out.

## Suggested Exit Criteria Before Approval

- Resolve Finding 1 fully. That is the blocker.
- Resolve Findings 2 and 3 in the data/query contract before frontend work starts.
- Resolve Findings 4 and 5 before assigning implementation slices across contributors.
- Resolve Findings 6, 7, and 8 while updating the plan/spec set so the implementer has one unambiguous contract.

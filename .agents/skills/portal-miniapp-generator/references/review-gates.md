# Portal Mini-App Review Gates

These gates apply during planning and during review of implemented screens.

## 1. Comparable Apps Gate

Pass only if:
- at least 2 comparable mini-app screens from the repo were inspected
- the plan or review cites the exact files inspected
- reused patterns and rejected patterns are both called out

Fail if:
- the screen direction is justified only by general taste
- the repo family is asserted without concrete inspection

## 2. Archetype Gate

Pass only if:
- exactly one primary archetype from `archetypes.md` is chosen
- the planned screen composition fits that archetype

Fail if:
- the layout silently mixes multiple archetypes
- a new archetype is invented without being declared as an exception

## 3. Copy Gate

Default policy: `business-user-only`.

Pass only if user-facing text talks about the business task, domain object, or next action.

Fail if the UI includes technical or machine-facing text such as:
- `server-side`
- `inline update`
- `record`
- `widget`
- `datasource`
- `id.asc`
- `replica dell'app originale`
- `senza aprire modali`
- text that explains how the interface is implemented instead of what the user can do

Notes:
- developer-facing language can exist in docs, comments, or plans
- it should not appear in user-facing UI copy unless the product domain itself requires it
- register and formatting rules for user-facing copy (Italian, dry B2B, it-IT formats) live in `docs/UI-UX.md` §17; this gate only decides pass/fail at review

## 4. Metrics Gate

Pass only if:
- every metric or stat card is based on real feature data
- the metric is useful to the user
- the feature request or approved spec justifies the metric explicitly

Fail if:
- metrics are invented to fill visual space
- a CRUD screen shows summary cards with no operational value
- the metric repeats information already obvious from the visible table

## 5. Style Consistency Gate

Pass only if:
- the screen aligns with the existing clean mini-app family
- the layout resembles the relevant repo patterns more than a one-off concept
- surfaces, spacing, and typography feel consistent with the family anchors: `budget`, `listini-e-sconti`, and `reports` are the historical trio; also compare against the most recently approved app of the same archetype

Fail if:
- the screen introduces a launcher-style hero or visual language
- the screen feels like a bespoke landing page instead of a workspace
- ornamental panels overshadow the working data surface
- a second charting stack is introduced when another clean mini-app already proves a chart library in this repo — default to the existing library and record any deviation in `Exceptions`

## 6. Repo-Fit Gate

Pass only if:
- route/base path is specified
- API prefix is specified (public URLs are `/api/<app-prefix>/v1/...`; module Go code omits `/api` — see `docs/API-CONVENTIONS.md`)
- role/ACL shape is specified (`app_{appname}_access` naming)
- Vite port and proxy needs are specified, including backend CORS defaults and contributor env examples — proxy config alone is not enough
- static deploy path is specified when the backend serves the SPA
- the plan has been checked against `docs/IMPLEMENTATION-PLANNING.md` and the New App Checklist in `CLAUDE.md`

Fail if runtime or deployment wiring is left implicit.

### Proven touchpoint checklist (new app)

Full list proven in code for a new DSN-backed mini-app. The plan covers each item or states why it does not apply:

- root `package.json` (dev concurrently entry + `dev:{appname}` script) and `pnpm-lock.yaml`
- `Makefile` (`dev-{appname}` target + `.PHONY`)
- `docker-compose.dev.yaml` when the repo supports `make dev-docker`
- `deploy/Dockerfile`
- `backend/.env.example` and root `.env.preprod.example`
- `backend/internal/platform/config/config.go`
- `backend/cmd/server/main.go` — DB handles opened via `database.New(...)`, module routes registered on the `api` sub-mux, `/api` stripped once via `http.StripPrefix`
- `backend/internal/platform/applaunch/catalog.go` **and** `catalog_test.go` — test updates are first-class work, never implicit
- `backend/internal/portal/handler_test.go` and `backend/internal/platform/staticspa/handler_test.go`
- launcher icon key verified against `apps/portal/src/components/Icon/icons.tsx` (see the icon entry in `docs/IMPLEMENTATION-KNOWLEDGE.md`)

### DSN and database wiring

- DSN-backed apps: the plan decides explicitly between hiding the launcher tile when the DSN is missing (repo convention: filter dependency-backed apps out of `appCatalog`) and exposing a role-visible app that returns `503 <app>_database_not_configured`.
- A new DB-owned table or seed is repo-fit only with a concrete migration story: checked-in SQL under `deploy/migrations/`, apply rule, env contract, seed stability. "A migration is required" alone fails this gate; the `CLAUDE.md` database-safety rules apply.
- Multi-view apps lazy-load route pages by default so the production bundle splits per route.

### Data contracts (migrations)

- Exact JSON/query shapes read from legacy sources are pinned before implementation and turned into narrow contract tests; "confirm during implementation" fails this gate.
- For still-unpinned legacy shapes, the plan includes an explicit contract/validation gate proportional to app complexity: lightweight (schema docs in `docs/<db>/` plus a few drift-prone regression checks) for small read-only apps, heavier where warranted.
- When live DB access is unavailable, checked-in fixtures derived from the approved source app can unblock implementation if pinned under `backend/internal/<app>/testdata` and guarded by query-shape/decoder regression tests — with explicit later revalidation against live data.
- Cascading lookup screens state backend-owned nested-resource invariants (`parent + context -> child`) and the matching regression tests; the frontend cascade alone does not protect lookup correctness.
- When users and collected data share one business timezone, prefer a single pinned local datetime wire format and state explicitly that no timezone conversion layer exists; format and interval semantics are fixed in the plan.
- If the source spec and the plan disagree on the API namespace (for example unversioned paths vs `/v1`), reconcile the spec before implementation handoff so docs, tests, and routes do not drift.
- These contract/regression tests fall under the allowance in the `AGENTS.md` Test Rule (business-critical rules, non-trivial data transformations); the plan lists them for user approval instead of adding them silently.

## 7. Exception Gate

Pass only if:
- every deviation from archetype, copy policy, metrics policy, or style family is listed explicitly
- each exception includes a concrete user benefit

Fail if:
- the implementation relies on “creative freedom” instead of a recorded exception

## Review artifact expectation

Pre-gate (plan review): the artifact is the implementation plan itself — comparable-apps citations, archetype choice, copy and metric constraints, repo-fit wiring, and the `Exceptions` section.

Post-gate (implemented screens), for non-trivial mini-apps the review covers at least:
- populated desktop state
- empty state
- error or destructive-confirm state
- mobile or narrow viewport state when the screen is responsive

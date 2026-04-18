# portal-miniapp-generator Skill Memory

## Purpose

- Native Codex mirror of the MrSmith mini-app planning skill.
- Lives under `.agents/skills` so Codex can discover it as a repo-scoped skill without replacing the legacy `.claude/skills` copy yet.

## Guardrails

- Always inspect at least 2 comparable mini-app screens in the repo before proposing layout or copy.
- Default CRUD and single-entity registries to `master_detail_crud`.
- Keep mini-apps inside the existing clean family anchored by `budget`, `listini-e-sconti`, and `reports`.
- Treat `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, and `docs/UI-UX.md` as mandatory preflight references.

## Workflow Notes

- Appsmith flow is now: `appsmith-audit -> appsmith-migration-spec -> portal-miniapp-generator -> portal-miniapp-ui-review pre-gate -> portal-miniapp-ui-fixer -> portal-miniapp-ui-review post-gate`.
- The plan produced by this skill is the per-app contract and the handoff artifact for both the fixer and the blocking UI reviewer.
- Keep the `.claude/skills` copy in sync until the repo fully switches to native Codex paths.
- New mini-app plans must describe backend wiring using the repo's real server pattern: DB handles are opened in `backend/cmd/server/main.go` via `database.New(...)`, module routes are registered on the `api` sub-mux, and `/api` is added only once via `http.StripPrefix("/api", api)`.
- When a plan assigns a new Vite port for split-server local development, it must also account for backend CORS defaults and contributor env examples; proxy config alone is not enough.
- If the repo supports `make dev-docker`, new-app plans should cover `docker-compose.dev.yaml` as part of dev wiring, not just `package.json` and `Makefile`.
- For DSN-backed apps, the plan must decide explicitly whether the launcher hides the tile when the DSN is missing or intentionally exposes a role-visible app that can return `503`; current repo convention is to filter dependency-backed apps out of `appCatalog`.
- When a plan adds a new launcher app ID / href / role, it should call out the matching `backend/internal/platform/applaunch/catalog_test.go` updates as first-class work, not leave them implicit.
- If a mini-app introduces a new DB-owned table or seed data, the plan is not repo-fit until it defines a concrete manual migration/bootstrap story (checked-in SQL path, apply rule, env contract, and seed stability), not just "a migration is required."
- For Appsmith migrations that read database functions/views with JSON payloads, exact shapes must be pinned before implementation and turned into narrow contract tests; "confirm during implementation" is not enough.
- For Appsmith migrations with still-unpinned legacy query shapes, the implementation plan should include an explicit contract/validation gate and say which checks must land before signoff. The gate can be heavy or lightweight depending on app complexity, but it must make the drift-prone behaviors explicit.
- That gate should stay proportional to app complexity: for small read-only mini-apps with well-documented schemas already checked into `docs/<db>/`, prefer a lightweight validation gate driven by the schema docs plus a few drift-prone regression checks instead of a heavy pre-implementation fixture phase.
- For cascading lookup mini-apps, plans must state backend-owned nested-resource invariants (`parent + context -> child`) and the matching regression tests; do not assume the frontend cascade alone protects lookup correctness.
- When users and collected data both operate in the same business timezone, prefer one pinned local datetime wire format and explicitly state that no timezone-conversion layer is performed. The plan still needs the format and interval semantics fixed.
- If the source spec and the implementation plan disagree on the repo API namespace (for example unversioned paths vs `/v1`), reconcile the spec before implementation handoff so docs, tests, and routes do not drift.
- For `apps/kit-products` settings registries, the repo-fit pattern is a compact `master_detail_crud` page reusing `SettingsPage.module.css`, selection-driven `Modifica`, modal create/edit, and explicit empty/error states rather than a side detail panel.
- `common.vocabulary` is not universally read-only in mini-apps: `kit_product_group` can be admin-managed from `kit-products`, but runtime consumers may still intentionally keep using `common.vocabulary.name` while translations stay administrative-only for that feature slice.
- For a tabbed Appsmith page that splits into 4-5 peer, read-only work surfaces with mixed filter/chart/table/master-detail behavior, the safest top-level archetype is often `data_workspace`: keep the source tab mental model, but implement it as app-shell sub-routes plus `TabNav` so deep links, refreshes, and shell consistency remain repo-fit.
- When a source Appsmith screen used ECharts but the repo already has a proven charting stack in another clean mini-app, planning should default to the existing repo chart library first and record that deviation explicitly in `Exceptions` instead of silently adding a second charting stack.

## Recent Implementations

- **Coperture** — 2026-04-17
  - The approved `report_explorer` shape stayed intentionally compact: page title, 4 cascading `SingleSelect` filters, explicit `Cerca` and `Reimposta filtri`, one submitted-address summary line, and one 4-column results table. No KPI cards, no export CTA, no launcher-style banner.
  - The full repo-fit touchpoint list for a new DSN-backed mini-app is now proven in code: root `package.json`, `Makefile`, `docker-compose.dev.yaml`, `deploy/Dockerfile`, `backend/.env.example`, root `.env.preprod.example`, `backend/internal/platform/config/config.go`, `backend/cmd/server/main.go`, `backend/internal/platform/applaunch/catalog.go`, `backend/internal/platform/applaunch/catalog_test.go`, `backend/internal/portal/handler_test.go`, `backend/internal/platform/staticspa/handler_test.go`, and `pnpm-lock.yaml`.
  - When live DB access is unavailable during an Appsmith migration, checked-in fixtures derived from the approved source app can unblock implementation if they are pinned under `backend/internal/<app>/testdata` and guarded by query-shape/decoder regression tests. They still need later revalidation against live DB artifacts when the DSN becomes available.
- **Energia in DC planning** — 2026-04-18
  - A five-view analytical app can stay within one explicit `data_workspace` archetype even when individual routes resemble `report_explorer` or master-detail screens; the key is to keep the app shell unified and document the mixed internal surfaces instead of silently mixing archetypes.
  - For five peer routes, `TabNav` is acceptable if the app plan also calls out a horizontally scrollable narrow-viewport nav wrapper; this preserves source-tab parity without forcing grouped dropdown navigation.

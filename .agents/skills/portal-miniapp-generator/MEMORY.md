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
- For `apps/kit-products` settings registries, the repo-fit pattern is a compact `master_detail_crud` page reusing `SettingsPage.module.css`, selection-driven `Modifica`, modal create/edit, and explicit empty/error states rather than a side detail panel.
- `common.vocabulary` is not universally read-only in mini-apps: `kit_product_group` can be admin-managed from `kit-products`, but runtime consumers may still intentionally keep using `common.vocabulary.name` while translations stay administrative-only for that feature slice.

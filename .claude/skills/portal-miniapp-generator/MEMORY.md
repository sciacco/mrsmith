# portal-miniapp-generator Skill Memory

## Guardrails

- Always inspect at least 2 comparable mini-app screens in the repo before proposing layout or copy.
- Default CRUD and single-entity registries to `master_detail_crud`.
- Keep mini-apps inside the existing clean family anchored by `budget`, `listini-e-sconti`, and `reports`.
- Treat `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, and `docs/UI-UX.md` as mandatory preflight references.

## Anti-Patterns To Block

- Unjustified hero or banner shells in CRUD/data-workspace screens.
- KPI or stat cards invented for visual fill instead of user need.
- Copy that explains mechanics rather than business intent.
- UI text exposing transport, sorting syntax, or legacy migration details.
- Visual drift away from existing mini-apps when an approved archetype already fits.

## Seed Regression

- **RDF Backend** — 2026-04-15
  - Regressions to block in future mini-apps:
    - hero banner not anchored to comparable apps
    - invented KPI cards
    - machine-facing copy such as `server-side`, `inline`, `record`, `id.asc`
    - style not grounded in the current mini-app family
  - Preferred correction:
    - compact page title and subtitle
    - functional toolbar with search plus primary action
    - single table card with pagination footer
    - persistent detail panel with business-facing edit copy
    - no shell metadata pills unless they represent real user-facing meaning

## Workflow Notes

- Appsmith flow is now: `appsmith-audit -> appsmith-migration-spec -> portal-miniapp-generator -> portal-miniapp-ui-review pre-gate -> implementation -> portal-miniapp-ui-review post-gate`.
- The plan produced by this skill is the per-app contract and the handoff artifact for the blocking UI reviewer.
- Native Codex mirrors now exist under `.agents/skills/portal-miniapp-generator` and `.agents/skills/portal-miniapp-ui-review`, with a repo-scoped custom reviewer agent at `.codex/agents/portal-ui-reviewer.toml`.

## Recent Implementations

- **Richieste Fattibilita** — 2026-04-16
  - Kept the approved `master_detail_crud` shell and handled `Visualizza RDF` as a tabbed exception inside the same mini-app.
  - Locked repo-fit defaults:
    - SPA path `/apps/richieste-fattibilita/`
    - API prefix `/api/rdf/v1/*`
    - split-server port `5182`
    - launcher hidden when either `ANISETTA_DSN` or `MISTRA_DSN` is missing
  - Important implementation nuance:
    - `rdf_*` lives on Anisetta while HubSpot replica enrichment lives on Mistra, so summary “server-side merge” must happen as two reads plus Go-side merge, not as one SQL join.
  - Post-implementation UI regression discovered:
    - decorative banner shell on the list screen
    - raw `Unauthorized` visible in the user-facing error state
    - implementation drifted from the cited comparable screens despite the plan being correct
  - Process correction:
    - do not treat the generator skill as the final UI approver
    - require blocking review through `portal-miniapp-ui-review`, using screenshots when available and code-first fallback when not

- **Coperture** — 2026-04-17
  - The approved `report_explorer` shape stayed intentionally compact: page title, 4 cascading `SingleSelect` filters, explicit `Cerca` and `Reimposta filtri`, one submitted-address summary line, and one 4-column results table. No KPI cards, no export CTA, no launcher-style banner.
  - The full repo-fit touchpoint list for a new DSN-backed mini-app is now proven in code: root `package.json`, `Makefile`, `docker-compose.dev.yaml`, `deploy/Dockerfile`, `backend/.env.example`, root `.env.preprod.example`, `backend/internal/platform/config/config.go`, `backend/cmd/server/main.go`, `backend/internal/platform/applaunch/catalog.go`, `backend/internal/platform/applaunch/catalog_test.go`, `backend/internal/portal/handler_test.go`, `backend/internal/platform/staticspa/handler_test.go`, and `pnpm-lock.yaml`.
  - When live DB access is unavailable during an Appsmith migration, checked-in fixtures derived from the approved source app can unblock implementation if they are pinned under `backend/internal/<app>/testdata` and guarded by query-shape/decoder regression tests. They still need later revalidation against live DB artifacts when the DSN becomes available.
- **Energia in DC** — 2026-04-18
  - The implementation confirmed the full repo-fit surface for a five-route DSN-backed `data_workspace` mini-app: app package, backend module, launcher/catalog wiring, env/config examples, Docker static copy, split-server dev wiring, portal/static tests, and `pnpm-lock.yaml` importer updates all need to land together.
  - For mixed chart/table analytical apps, route-level lazy loading is worth planning up front; Energia DC stayed repo-fit and avoided a single oversized entry bundle by splitting each top-level route into its own chunk.
  - Launcher icon names are constrained by `apps/portal/src/components/Icon/icons.tsx`; planning should verify supported keys before proposing a new tile icon.

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
    - require screenshot-based blocking review through `portal-miniapp-ui-review`

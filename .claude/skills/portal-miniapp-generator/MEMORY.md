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

## Workflow Notes

- Appsmith flow is now: `appsmith-audit -> appsmith-migration-spec -> portal-miniapp-generator -> implementation`.
- The plan produced by this skill is the per-app contract; do not create extra governance files unless the repo explicitly adopts them later.

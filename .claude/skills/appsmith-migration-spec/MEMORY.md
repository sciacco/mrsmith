# appsmith-migration-spec Skill Memory

## Completed Specs

- **Budget Management** (`apps/budget/BUDGET_MIGRATION_SPEC.md`) — 2026-04-05
  - Source: `apps/budget/APPSMITH_AUDIT.md`
  - Phase docs: `budget-migspec-phaseA.md` through `budget-migspec-phaseD.md`
  - 20 expert questions resolved (Q1–Q20), 2 deferred to `docs/TODO.md`
  - Key decisions: Stripe design, top tabs, two-page budget drill-down, 3 approval levels, read-only panels + modal edit, Go BFF 1:1 proxy, UI-first with WOW effect, build order Gruppi→CC→Budget→Home

## Workflow Notes

- Expert prefers direct answers (e.g., "q4: enable is an useful addition") — keep questions concise
- Verify claims against audit source before writing flags — do not invent evidence the audit doesn't contain (caught on Q1)
- Design questions benefit from firing a Plan agent for multiple options with pros/cons
- Italian domain language preserved for compatibility ("Voci di costo" kept as-is)

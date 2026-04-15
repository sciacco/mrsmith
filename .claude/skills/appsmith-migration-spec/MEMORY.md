# appsmith-migration-spec Skill Memory

## Completed Specs

- **Budget Management** (`apps/budget/BUDGET_MIGRATION_SPEC.md`) — 2026-04-05
  - Source: `apps/budget/APPSMITH_AUDIT.md`
  - Phase docs: `budget-migspec-phaseA.md` through `budget-migspec-phaseD.md`
  - 20 expert questions resolved (Q1–Q20), 2 deferred to `docs/TODO.md`
  - Key decisions: Stripe design, top tabs, two-page budget drill-down, 3 approval levels, read-only panels + modal edit, Go BFF 1:1 proxy, UI-first with WOW effect, build order Gruppi→CC→Budget→Home

- **RDF Backend** (`apps/rdf-backend/RDF_BACKEND_MIGRATION_SPEC.md`) — 2026-04-15
  - Source: `apps/rdf-backend/audit/*.md`
  - Phase docs: `rdf-backend-migspec-phaseA.md` → `…-phaseD.md`
  - 0 expert questions — app triviale (1 entità `fornitori{id,nome}`, 4 query CRUD), policy: porting 1:1

## Workflow Notes

- Expert prefers direct answers (e.g., "q4: enable is an useful addition") — keep questions concise
- For trivial scaffolded apps, skip Q&A and default to 1:1 porting unless the expert asks otherwise — don't manufacture questions proportional to the template when the app is tiny
- Verify claims against audit source before writing flags — do not invent evidence the audit doesn't contain (caught on Q1)
- Design questions benefit from firing a Plan agent for multiple options with pros/cons
- Italian domain language preserved for compatibility ("Voci di costo" kept as-is)

# Approved Mini-App Archetypes

Pick the smallest archetype that fits the user task. Reuse the repo family before inventing a new composition.

## `master_detail_crud`

- Use when: single-entity registries, admin tables, list/detail CRUD, selector + edit workflows
- Default composition:
  - compact page header with title and at most one business subtitle
  - toolbar with search, filters, and primary action
  - primary table or list surface
  - detail panel, inline form, or modal for create/edit
  - explicit loading, empty, error, and destructive-confirm states
- Forbidden defaults:
  - full-width hero banner
  - KPI row or stat cards
  - explanatory side panels that describe implementation mechanics
  - decorative status pills unless status is real domain data
- Reference apps:
  - `apps/budget/src/views/gruppi/GruppiPage.tsx`
  - `apps/listini-e-sconti/src/pages/GruppiScontoPage.tsx`

## `data_workspace`

- Use when: a screen coordinates multiple related data panels, filters, tabs, or secondary inspectors
- Default composition:
  - compact page header
  - clear primary workspace area
  - secondary cards or panels only when they support the main task
  - filters and actions close to the data they affect
- Forbidden defaults:
  - dashboard-style KPI shells unless the feature is actually metric-led
  - marketing-style banner introductions
- Reference apps:
  - `apps/reports/src/pages/OrdiniPage.tsx`

## `report_explorer`

- Use when: the main user value is exploration, preview, and export of report data
- Default composition:
  - concise report header
  - report filters
  - preview surface
  - export or side actions
  - metrics only if they summarize real report output
- Forbidden defaults:
  - placeholder metrics unrelated to report data
  - decorative panels that duplicate visible information
- Reference apps:
  - `apps/reports/src/pages/OrdiniPage.tsx`

## `wizard_flow`

- Use when: users complete a sequence with clear steps, validation, and branching
- Default composition:
  - step framing
  - guided forward/back actions
  - contextual summary only when it reduces user error
- Forbidden defaults:
  - flattening a real multi-step process into a single overstuffed screen
  - padding the first step with decorative banner content

## `settings_form`

- Use when: the screen is primarily a configuration form rather than a data registry
- Default composition:
  - concise header
  - grouped settings sections
  - sticky or clear save/discard actions when needed
- Forbidden defaults:
  - fake dashboards around a simple form
  - extra narrative copy that repeats obvious form intent

## Selection rule

- If more than one archetype seems plausible, choose the more constrained one unless there is a concrete user-task reason not to.
- For CRUD apps like `rdf-backend`, choose `master_detail_crud` by default.
- New archetypes should be treated as exceptions first, not as casual additions.

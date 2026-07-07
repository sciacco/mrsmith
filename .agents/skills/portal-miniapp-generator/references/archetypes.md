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
- Note: the archetype does not mandate a side master-detail split — the detail surface may be a sticky panel, an inline form, or a modal. Per `docs/UI-UX.md` §12, do not force a side detail panel when list and detail need independent filters or exports; a modal or a dedicated route fits better.
- Reference apps:
  - `apps/budget/src/views/gruppi/GruppiPage.tsx`
  - `apps/listini-e-sconti/src/pages/GruppiScontoPage.tsx`
  - `apps/kit-products/src/views/settings/ProductGroupsPage.tsx` (compact settings registry: selection-driven `Modifica`, modal create/edit, explicit empty/error states)

## `data_workspace`

- Use when: a screen coordinates multiple related data panels, filters, tabs, or secondary inspectors — including tabbed multi-surface apps migrated from sources with 4–5 peer tabs
- Default composition:
  - compact page header
  - clear primary workspace area
  - secondary cards or panels only when they support the main task
  - filters and actions close to the data they affect
- Notes:
  - for 4–5 peer surfaces with mixed filter/chart/table behavior, keep the source tab mental model but implement it as app-shell sub-routes plus `TabNav`, so deep links, refreshes, and shell consistency stay repo-fit; with five routes, plan a horizontally scrollable narrow-viewport nav wrapper
  - a multi-view app can stay within one declared `data_workspace` even when individual routes resemble `report_explorer` or master-detail screens — keep the app shell unified and document the mixed internal surfaces instead of silently mixing archetypes
- Forbidden defaults:
  - dashboard-style KPI shells unless the feature is actually metric-led
  - marketing-style banner introductions
- Reference apps:
  - `apps/energia-dc/src/routes.tsx` (five-route workspace; e.g. `apps/energia-dc/src/pages/SituazioneRackPage.tsx`)
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
  - `apps/coperture/src/pages/CoverageLookupPage.tsx` (approved compact shape: title, cascading `SingleSelect` filters, explicit `Cerca`/`Reimposta filtri`, one results table — no KPI cards or export CTA unless real)
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
- Reference apps:
  - `apps/rda/src/pages/NewRdaWizardPage.tsx` (with `WizardStepper`)
  - `apps/quotes/src/pages/QuoteCreatePage.tsx` (with `Stepper`/`WizardNav`)

## `settings_form`

- Use when: the screen is primarily a configuration form rather than a data registry
- Default composition:
  - concise header
  - grouped settings sections
  - sticky or clear save/discard actions when needed
- Forbidden defaults:
  - fake dashboards around a simple form
  - extra narrative copy that repeats obvious form intent
- Reference apps: no clean exemplar in the repo yet — cite the two closest form-heavy screens during planning and record composition deviations in `Exceptions`

## Selection rule

- If more than one archetype seems plausible, choose the more constrained one unless there is a concrete user-task reason not to.
- For CRUD apps like `rdf-backend`, choose `master_detail_crud` by default.
- New archetypes should be treated as exceptions first, not as casual additions.

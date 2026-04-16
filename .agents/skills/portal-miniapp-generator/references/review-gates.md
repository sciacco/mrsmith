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
- surfaces, spacing, and typography feel consistent with `budget`, `listini-e-sconti`, and `reports`

Fail if:
- the screen introduces a launcher-style hero or visual language
- the screen feels like a bespoke landing page instead of a workspace
- ornamental panels overshadow the working data surface

## 6. Repo-Fit Gate

Pass only if:
- route/base path is specified
- API prefix is specified
- role/ACL shape is specified
- Vite port and proxy needs are specified
- static deploy path is specified when the backend serves the SPA
- the plan has been checked against `docs/IMPLEMENTATION-PLANNING.md`

Fail if runtime or deployment wiring is left implicit.

## 7. Exception Gate

Pass only if:
- every deviation from archetype, copy policy, metrics policy, or style family is listed explicitly
- each exception includes a concrete user benefit

Fail if:
- the implementation relies on “creative freedom” instead of a recorded exception

## Review artifact expectation

For non-trivial mini-apps, the review should cover at least:
- populated desktop state
- empty state
- error or destructive-confirm state
- mobile or narrow viewport state when the screen is responsive

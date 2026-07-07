# Portal Mini-App UI Blocking Gates

These gates are strict. A failed gate means `blocked`.

## 1. Evidence Gate

Pass only if:
- the review phase is explicit
- the approved plan and chosen archetype are available
- at least 2 comparable repo screens are cited with exact file paths
- the reviewed screen/route/component is identifiable
- the implementation files are present
- screenshots are included when they are reasonably obtainable

Fail if:
- approval depends on imagination or inferred states
- the reviewed screen or route cannot be identified
- implementation files are missing
- neither code nor available visuals are sufficient to evaluate the relevant behavior

## 2. Archetype Gate

Pass only if:
- the rendered screen still matches the approved archetype
- any deliberate deviation is recorded as an explicit exception

Fail if:
- the UI silently mixes multiple compositions
- a CRUD/data workspace drifts into a launcher or landing-page shell

## 3. Style-Family Gate

Pass only if:
- the screen resembles the cited repo comparables more than a one-off concept
- the working surface is visually primary
- decorative framing does not overpower the task surface

Fail if:
- hero or banner shells dominate a CRUD list screen
- launcher visual language leaks into a mini-app workspace
- ornamental gradients, oversized shells, or decorative panels become the main composition

Calibration (Coperture precedent): subtle page/surface gradients and depth are acceptable and are not findings by themselves. The blocking threshold is decorative framing that dominates or competes with the working surface — not any use of texture.

## 4. Copy Gate

Default policy: `business-user-only`.

Pass only if:
- user-facing text speaks about the business task, object, or next action
- empty and error states are written for the end user
- bootstrap/startup fallbacks (loading, access denied, fatal config errors) use business-facing copy — they are user-facing UI like any other screen (Energia DC precedent). The shared `AccessNotice` component in `packages/ui` is the standard remedy for app bootstrap states.

Fail if the UI shows:
- `Unauthorized`
- raw HTTP, auth, config, or backend status text (including on bootstrap/startup screens)
- technical nouns such as `record`, `datasource`, `server-side`, `inline`, `inline update`, `JSON`, `widget`
- sort or query syntax such as `id.asc`
- source-app framing such as `replica dell'app originale`, `senza aprire modali`
- copy that explains implementation mechanics instead of user intent

This is the canonical banned-terms list for the mini-app family; the generator's Copy Gate defers to it.

## 5. Metrics Gate

Pass only if:
- each metric or stat is explicitly justified in the approved plan
- the metric uses real feature data
- the metric is operationally useful

Fail if:
- cards exist mainly to fill space
- the same information is already obvious from the visible list or table

## 6. Shared Shell Gate

Pass only if:
- shared CSS or layout abstractions follow a screen shape that already passed review

Fail if:
- a generic page shell is created first and then forces multiple screens into the wrong composition
- visual abstraction is driving the design more than the approved archetype

## 7. Exception Gate

Pass only if:
- every deviation from archetype, style, copy, or metrics policy is listed explicitly
- each deviation includes a concrete user benefit

Fail if:
- the rationale is aesthetic preference alone
- the implementation relies on implied creative freedom

## 8. Design System Gate

The canonical design system is `docs/UI-UX.md`; this gate enforces its hard rules at review time.

Pass only if:
- the screen respects the non-negotiables in `docs/UI-UX.md` §0 (clean theme, token discipline, shared components, `Icon` as the only icon source)
- semantic base colors are not used as running text (§4.2 text-safety)
- entrance animations fire on navigation only — no replay on refetch, polling, sort, filter, or pagination (§8.3)
- numeric/money columns are right-aligned with tabular figures and `it-IT` formatting (§13.1, §17)
- clickable rows and overlays are keyboard-operable (§13.2, §16)
- errors requiring user action have a persistent surface, not just a toast (§14.3)

Fail if:
- hardcoded colors/spacing/radii/shadows appear outside the documented recipes
- any §0 non-negotiable is violated without a recorded exception

For newly built screens, the §19 checklist of `docs/UI-UX.md` is the reference checklist.

## Approval rule

Approve only when:
- no blocking findings remain
- the primary rendered states match the approved mini-app family
- residual risks, if any, are verification gaps rather than known UI defects
- missing screenshots are called out explicitly when approval relied on code-first fallback

# Portal Mini-App UI Blocking Gates

These gates are strict. A failed gate means `blocked`.

## 1. Evidence Gate

Pass only if:
- the review phase is explicit
- the approved plan and chosen archetype are available
- at least 2 comparable repo screens are cited with exact file paths
- required screenshots and implementation files are present

Fail if:
- approval depends on imagination or inferred states
- a primary screen is reviewed from code only
- required empty/error/narrow evidence is missing

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

## 4. Copy Gate

Default policy: `business-user-only`.

Pass only if:
- user-facing text speaks about the business task, object, or next action
- empty and error states are written for the end user

Fail if the UI shows:
- `Unauthorized`
- raw HTTP or backend status text
- technical nouns such as `record`, `datasource`, `server-side`, `inline`, `JSON`, `widget`
- copy that explains implementation mechanics instead of user intent

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

## Approval rule

Approve only when:
- no blocking findings remain
- the primary rendered states match the approved mini-app family
- residual risks, if any, are verification gaps rather than known UI defects

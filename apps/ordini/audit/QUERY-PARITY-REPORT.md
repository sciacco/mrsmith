You are auditing whether the new MrSmith "Ordini" mini-app reaches functional parity
with the legacy Appsmith "Ordini" app. We are about to retire the Appsmith version
and need high confidence that nothing the operators rely on today is missing or
silently behaving differently.

REPO ROOT: /home/sciacco/devel/mrsmith

WHAT TO COMPARE

Source of truth (Appsmith side, in priority order):
  1. PRIMARY — the raw Appsmith export at apps/ordini/Ordini.json.gz and
     apps/ordini/Ordini1.json.gz. Decompress (gunzip to a temp file under
     artifacts/claude/ — never commit) and walk the JSON: pages, widgets,
     actions/queries, datasources, JS objects, role visibility expressions.
     This is the authoritative behavior — treat audit docs as supporting only.
  2. SUPPORTING — apps/ordini/audit/page-audit.md, datasource-catalog.md,
     findings-summary.md, ordini-migspec-phaseA..E*.md. Use to orient yourself
     and cross-check, but if JSON and audit disagree, trust the JSON.

Target (MrSmith side):
  - Frontend:  apps/ordini/src/**
  - Backend:   backend/internal/ordini/**
  - Wiring:    backend/internal/platform/applaunch/catalog.go,
               backend/internal/platform/config/config.go,
               backend/cmd/server/main.go,
               packages/auth-client/src/roles.ts

Plan & known-deviation register:
  - apps/ordini/docs/IMPL-ORDINI.md — section 1 (in/out of scope), section 9
    (deliberately revised business rules: Q2-Q8, C1, C2, ERP state hard-code,
    Arxivar write path), section 21 (deferred: cancel-order, lost, ERP retry,
    server-side pagination, cdlan_int_fatturazione=5 migration).

SCOPE OF AUDIT (three dimensions — be exhaustive within these)

  A. Functional / actions
     For every Appsmith query/API call/JS action, identify the MrSmith
     equivalent (backend handler, frontend mutation/query, or PDF proxy).
     Verify trigger, preconditions, payload shape, and side effects match.
     Flag any Appsmith action with no MrSmith counterpart.

  B. Data displayed
     For every field/column rendered by an Appsmith widget (table columns,
     form fields, badges, computed labels, hidden inputs surfaced via JS),
     verify the same field is reachable in MrSmith. Italian label may differ
     — what matters is that the data is not lost.

  C. Role / state gating
     For every Appsmith visibility/disabled expression on a widget or
     action, identify the corresponding MrSmith gate (backend ACL +
     frontend permissions.ts). Flag any case where MrSmith is more
     permissive OR more restrictive than Appsmith.

EXPLICITLY OUT OF SCOPE — do not flag

  - UI layout, ordering, spacing, typography, animations.
  - Italian copy choices (provided the data shown is equivalent).
  - Toast/confirmation wording.
  - Any deviation already documented in IMPL-ORDINI.md §9 or §21.
    Mention these in a dedicated "Intentional deviations — verified" section
    confirming the implementation matches the documented intent, but do NOT
    list them as gaps.

METHOD

  1. Inventory the Appsmith app from the JSON: enumerate every page,
     every widget that produces or consumes data, every query/action, and
     every visibility/role expression. Build this inventory before judging
     the MrSmith side — otherwise you will miss things by anchoring on the
     MrSmith implementation.
  2. For each item in the inventory, locate the MrSmith counterpart by
     searching apps/ordini/src and backend/internal/ordini. Record file
     paths and line numbers.
  3. Compare. Classify each item as:
       - parity_confirmed
       - intentional_deviation (cite the IMPL-ORDINI.md anchor)
       - gap_blocking (operators lose functionality the legacy app provides)
       - gap_minor (functional equivalent exists but with a subtle drift —
         describe the drift precisely)
       - cannot_verify (note exactly what is missing to decide, e.g. a
         JS handler too complex to follow without runtime, or a query that
         needs DB access to confirm)
  4. Pay special attention to corners where audits typically miss things:
       - Hidden widgets / widgets with isVisible="{{false}}" that still
         carry actions wired elsewhere.
       - onPageLoad and JS object actions (not always table-bound).
       - Conditional defaults computed in widget bindings.
       - Server-side queries vs client-side JS filtering — make sure
         filtering hasn't silently moved layers.
       - "Submit" handlers that chain multiple actions (state flip + PDF
        upload + toast) — verify every step exists.

OUTPUT

Write a single markdown file to apps/ordini/audit/PARITY-REPORT.md with
this structure:

  # Ordini — Appsmith ↔ MrSmith parity report

  ## Summary
  - Total Appsmith items audited: N
  - parity_confirmed: N
  - intentional_deviation: N
  - gap_blocking: N
  - gap_minor: N
  - cannot_verify: N
  - Go/no-go recommendation: <one sentence>

  ## Gaps — blocking
  (one entry per finding, with: Appsmith location, MrSmith location or
  "missing", description, suggested resolution)

  ## Gaps — minor
  (same structure)

  ## Cannot verify
  (what would be needed to resolve)

  ## Intentional deviations — verified
  (cite IMPL-ORDINI.md anchors; confirm implementation matches intent)

  ## Per-page inventory
  (one section per Appsmith page, with a table of every audited item and
  its classification — this is the receipt that the audit was exhaustive)

CONSTRAINTS

  - Do not modify any source file. The only file you write is
    apps/ordini/audit/PARITY-REPORT.md.
  - Decompress JSON exports into artifacts/claude/ (gitignored). Do not
    leave decompressed copies under apps/ordini/.
  - Cite file paths with line numbers (path:line) so the user can jump.
  - When in doubt between "gap" and "intentional deviation", search
    IMPL-ORDINI.md for the rule before classifying. Do not invent
    justifications.
  - This is a parity audit, not a code review. Do not comment on style,
    naming, performance, or refactor opportunities.

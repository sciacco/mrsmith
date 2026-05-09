# Overall Remediation 2

## Status

- Source QA: `apps/grappa-dcim/docs/overall-qa.md`
- Overall status: FAIL
- Owning scope: shared Grappa DCIM frontend destructive-confirmation utility
- Required fix report: `apps/grappa-dcim/docs/overall-fix-2-report.md`

## Blocking Finding

`apps/grappa-dcim/docs/overall-qa.md` reports:

- High severity: the shared `ConfirmModal` in `apps/grappa-dcim/src/features/equipment/assetPageUtils.tsx` stores its two checkbox states but does not reset them when reopened.
- Affected shared destructive flows include apparato cease, storage archive, plenum/cable delete, and ring cease/delete.
- Expected behavior: every destructive/lifecycle action requires a fresh two-checkbox confirmation before sending `{ confirmPrimary: true, confirmSecondary: true }`.
- Contrast: facilities and rack local confirmation modals already reset on each open.

## Required Remediation

- Update the shared `ConfirmModal` in `apps/grappa-dcim/src/features/equipment/assetPageUtils.tsx` so it resets `first` and `second` whenever `open` becomes true.
- Match the existing facilities/rack local modal reset behavior.
- Do not change backend behavior.
- Do not add automated tests.

## Verification Required

- `pnpm --filter mrsmith-grappa-dcim build`
- `rg --files apps/grappa-dcim backend/internal/grappadcim | rg '(_test\.go|\.test\.|\.spec\.)'`
- Browser checks only if a suitable Grappa DCIM frontend/backend server is already running.

## Reporting Required

Write `apps/grappa-dcim/docs/overall-fix-2-report.md` with:

- files changed
- behavior implemented
- contracts preserved
- commands run and outputs summarized
- manual/browser checks and skipped-check reasons
- unresolved questions
- deviations from this remediation

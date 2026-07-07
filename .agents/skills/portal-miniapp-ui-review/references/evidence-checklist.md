# Portal Mini-App UI Evidence Checklist

Use this checklist before approving any mini-app screen.

## Pre-gate minimum

Require:
- approved implementation plan (canonical location: `apps/<app>/docs/IMPLEMENTATION-PLAN.md`)
- chosen archetype
- explicit exceptions section, even if empty
- 2 comparable repo screens with exact file paths
- enough screen structure detail to evaluate composition and copy

Block if any of the above is missing.

## Post-gate minimum

Require:
- the approved plan and exceptions
- the implementation files for the reviewed screen
- the route, screen, or component scope for the review

Prefer when reasonably obtainable:
- desktop screenshot of the primary populated or default state
- empty-state screenshot
- error-state screenshot
- destructive-confirm or modal-state screenshot
- narrow viewport screenshot for responsive layouts

## Evidence discipline

- When a dev server is already running and the route is reachable, capture screenshots via the `playwright-cli` skill (cwd `artifacts/claude/`, reuse the running server — never restart it); with that path available, screenshots are expected for post-gate, not optional.
- Approval from code alone is allowed when the reviewed route/component is identifiable and the implementation files expose the relevant behavior clearly.
- Do not approve a primary screen from a screenshot alone when the implementation files are available.
- If a state is important to the task and cannot be inferred from code or shown visually, block with `missing evidence`.
- If the UI leaks raw auth/backend errors, capture that as a blocking copy finding, not as a backend-only issue.
- If screenshots are genuinely unavailable (RBAC-blocked routes, no practical dev server), cite file evidence and record the missing visual verification as a residual risk instead of blocking automatically.

## Primary-screen defaults

For CRUD or data-workspace screens, try to cover these as primary states unless the plan says otherwise:
- main list or workspace state
- empty state
- error state
- narrow viewport state

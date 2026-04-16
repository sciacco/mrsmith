# Portal Mini-App UI Evidence Checklist

Use this checklist before approving any mini-app screen.

## Pre-gate minimum

Require:
- approved implementation plan
- chosen archetype
- explicit exceptions section, even if empty
- 2 comparable repo screens with exact file paths
- enough screen structure detail to evaluate composition and copy

Block if any of the above is missing.

## Post-gate minimum

Require:
- the approved plan and exceptions
- the implementation files for the reviewed screen
- desktop screenshot of the primary populated or default state

Also require when applicable:
- empty-state screenshot
- error-state screenshot
- destructive-confirm or modal-state screenshot
- narrow viewport screenshot for responsive layouts

## Evidence discipline

- Do not approve a primary screen from code alone.
- Do not approve a primary screen from a screenshot alone when the implementation files are available.
- If a state is important to the task and not shown, block with `missing evidence`.
- If the UI leaks raw auth/backend errors, capture that as a blocking copy finding, not as a backend-only issue.

## Primary-screen defaults

For CRUD or data-workspace screens, treat these as primary states unless the plan says otherwise:
- main list or workspace state
- empty state
- error state
- narrow viewport state

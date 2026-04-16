---
name: portal-miniapp-ui-review
description: Use this skill for blocking UI review of MrSmith portal mini-app screens. It validates planned and implemented screens against repo archetypes, comparable apps, copy rules, implementation files, and screenshots when available, and it blocks approval when the UI drifts from the mini-app family.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

This is the standalone blocking UI reviewer for MrSmith portal mini-apps.

Use it to:
- review a planned mini-app screen before coding
- review an implemented mini-app screen before signoff
- block recurring regressions such as hero shells, launcher-style drift, invented KPI filler, and raw transport/auth copy

This skill does not replace `portal-miniapp-generator`.
- `portal-miniapp-generator` owns repo-fit planning and archetype selection
- `portal-miniapp-ui-fixer` owns UI remediation and implementation
- `portal-miniapp-ui-review` owns UI approval

If the UI reviewer blocks a screen, the task is not done.

# When to use

Use this skill when:
- an implementation plan for a new mini-app already exists and needs a UI pre-gate
- a mini-app screen has been implemented and needs UI approval
- a screenshot or code review is requested for a portal mini-app workspace
- there is a concern that the screen drifted from the existing mini-app family

Do not use this skill when:
- extracting behavior from Appsmith exports
- drafting the implementation plan from scratch
- reviewing the Matrix-style launcher UI instead of a mini-app workspace
- making the actual UI changes; use `portal-miniapp-ui-fixer` for remediation

# Required inputs

Do not approve a screen without these inputs:
- approved implementation plan
- chosen archetype
- explicit exceptions, if any
- at least 2 comparable repo screens with exact file paths

For post-implementation review, also require:
- the relevant implementation files for the screen under review
- screenshots when they are reasonably obtainable per `references/evidence-checklist.md`

If evidence is too weak to support a grounded review, block instead of guessing.

# Bundled resources

- `references/blocking-gates.md`
  Use for the blocking review criteria and approval rule.
- `references/evidence-checklist.md`
  Use to verify that the review has enough artifacts to approve or block confidently.

# Workflow

## Step 1: Confirm the review phase

Choose one:
- `pre-gate` for plan review before coding
- `post-gate` for implemented UI review before signoff

If the request is ambiguous, determine the phase from the artifacts instead of inventing a hybrid review.

## Step 2: Validate the evidence package

Read `references/evidence-checklist.md`.

Block immediately if:
- comparable repo screens are missing
- the archetype is not declared
- the reviewed screen or route cannot be identified
- the relevant implementation files are missing
- the code does not expose enough behavior to evaluate the relevant state safely

Code-first approval is allowed when the implementation files and route/component scope are sufficient.
If screenshots are unavailable, record the visual verification gap explicitly instead of pretending certainty.

## Step 3: Run the blocking gates

Read `references/blocking-gates.md`.

Always check:
- archetype fit
- style-family fit against the cited repo screens
- copy and error-language fit
- metrics/KPI discipline
- exception handling
- shared shell abstractions that may be driving the screen in the wrong direction

For post-gate reviews, inspect the implementation files first and screenshots when available.
If screenshots and code disagree, trust the rendered UI and treat the mismatch as a finding.

## Step 4: Produce the verdict

Output must be one of:
- `approved`
- `blocked`

Review format:
- findings first, ordered by severity
- each finding cites the violated gate
- each finding cites file evidence and screenshots when available
- each finding states the required correction

If there are no findings:
- say that explicitly
- mention any residual verification gaps

When approval is granted without screenshots, the residual risks must say that visual verification was not performed and why.

Do not soften a blocking finding into a suggestion.
If the screen is blocked and remediation is requested, hand off the findings to `portal-miniapp-ui-fixer`.

# Core rules

- This reviewer has blocking authority on mini-app UI approval.
- The default visual target is the existing clean mini-app family, not a new concept direction.
- CRUD list screens default to compact workspace headers, toolbar, and one dominant working surface.
- Hero or banner shells on CRUD/data-workspace screens are blocked unless recorded as an explicit exception with user benefit.
- Raw backend, auth, or transport language in user-facing UI is blocked.
- Machine-facing copy is blocked even if the underlying feature is correct.
- KPI or stat cards are blocked unless the approved plan explicitly justifies them with real user value.
- Do not approve a shared page-shell abstraction until at least one real screen using it passes review.
- The reviewer never performs the fix itself; blocked screens go to `portal-miniapp-ui-fixer`.
- Missing screenshots alone are not blocking when RBAC or browser-access friction makes them impractical and the implementation files are sufficient for a grounded review.

# Review output

Use this structure:

```markdown
Status: approved|blocked

Findings
1. <severity> <title> — <why it blocks>
   Evidence: <screenshot and/or file path>
   Required correction: <what must change>

Residual Risks
- <only if no blocking findings remain>
```

Keep findings concrete. Avoid generic design commentary.

# Definition of done

This skill is complete when:
- the review phase is explicit
- the evidence package is sufficient for a grounded review
- blocking gates have been checked against real repo comparables
- approval is granted only when the screen matches the archetype, copy policy, and mini-app family
- any deviation is recorded as an explicit exception instead of being hand-waved as creative freedom

---
name: portal-miniapp-ui-fixer
description: Use this skill to fix the UI of a specific MrSmith portal mini-app under `apps/`. It inspects the target app, applies the existing mini-app planning and blocking review rules, asks the expert human only when real ambiguity remains, implements focused UI corrections, and finishes with mandatory post-fix review.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

This is the implementation skill for MrSmith portal mini-app UI remediation.

Use it to:
- fix the UI of a specific mini-app under `apps/`
- remediate findings produced by `portal-miniapp-ui-review`
- align an implemented mini-app with the approved MrSmith mini-app family without re-planning the app from scratch

This skill does not replace:
- `portal-miniapp-generator`, which owns planning and archetype selection
- `portal-miniapp-ui-review`, which owns blocking approval

The fixer owns implementation. It must not invent a parallel rule system.

# When to use

Use this skill when:
- the user asks to fix UI issues in a specific app under `apps/`
- a mini-app has blocked review findings that now need remediation
- a screenshot or code inspection shows that an implemented mini-app drifted from the approved family and the fix should be carried out now

Do not use this skill when:
- drafting the implementation plan for a new mini-app
- reviewing a screen without making changes
- working on the Matrix-style portal launcher instead of a mini-app workspace
- the task is primarily backend behavior rather than UI correction

# Required input

Required:
- target app path under `apps/`

Optional but useful:
- target route or screen name
- blocked review findings
- screenshots
- approved implementation plan

If the app path is missing or does not resolve to a real mini-app, stop and ask for the correct target.

# Bible And Precedence

Treat these as the authority, in this order:
1. the current user request
2. the approved app-specific plan and explicit `Exceptions`, if they exist
3. `portal-miniapp-ui-review` and its blocking gates
4. `portal-miniapp-generator` and its archetype/repo-fit rules
5. `docs/UI-UX.md`
6. `docs/IMPLEMENTATION-KNOWLEDGE.md`
7. `docs/IMPLEMENTATION-PLANNING.md`
8. comparable mini-app screens already present in the repo

If these do not leave one clear implementation direction, do not improvise. Ask the expert human.

# Workflow

## Step 1: Ground in the target app

Always inspect the real app before editing:
- the route entrypoints and relevant screen components
- page-level CSS or styling modules
- shared UI components the screen depends on
- current empty, loading, and error handling

Also inspect:
- at least 2 comparable mini-app screens already in the repo
- the approved plan if one exists
- prior blocked review findings if they exist
- the docs and sibling skills listed in `Bible And Precedence`

Do not fix from screenshot taste alone when the code and repo patterns are available.

## Step 2: Diagnose against the existing gates

Map the issue to the same rules already enforced by the reviewer:
- archetype drift
- style-family drift
- machine-facing or transport-facing copy
- invented KPI or decorative filler
- shared shell abstractions forcing the wrong composition
- missing or poor empty/error state behavior

Prefer the smallest correction set that returns the screen to the approved family.

## Step 3: Decide whether to fix or block

Proceed autonomously only when one correction path is clearly implied by the repo and the existing rules.

Stop and ask the expert human when:
- multiple repo-consistent layout or interaction fixes remain
- the change requires a new visible product decision rather than UI correction
- the approved plan does not cover the needed exception
- the fix likely belongs in shared UI code and there is real cross-app behavior risk
- the business meaning of a label, metric, empty state, or action is unclear

Blocking questions must be:
- short
- concrete
- framed as decisions, not open-ended brainstorming
- accompanied by a recommended default when possible

## Step 4: Implement with constrained scope

Default scope:
- prefer app-local fixes first
- allow shared `packages/` or shared styling changes only when the issue clearly belongs there and an app-local patch would be a bad workaround

Implementation rules:
- keep changes tightly scoped to the identified UI problem
- prefer removing drift over adding new decorative elements
- use business-user-facing copy only
- do not introduce hero banners, launcher visuals, KPI filler, or explanatory implementation copy unless the approved plan explicitly requires them
- do not leak raw auth, backend, HTTP, or transport text into the UI
- do not use a shared page-shell abstraction as the driver of a new composition unless it already fits the approved family

## Step 5: Mandatory post-fix review

After editing, run the blocking review workflow again using `portal-miniapp-ui-review` as the standard.

If the current client supports explicit skill handoff, hand off to `portal-miniapp-ui-review`.
If it does not, manually apply the same review workflow and gates before declaring the work complete.

Do not self-exempt from post-review.
If screenshots are impractical because of RBAC or browser-access friction, use the reviewer's code-first fallback and record the visual verification gap explicitly.
If a changed behavior cannot be judged safely from code alone, escalate to the expert human or request visual confirmation instead of guessing.
If the screen would still be `blocked`, report the remaining findings instead of claiming success.

# Core rules

- Required input is the target app path; screenshots and prior findings accelerate the work but are not required.
- The fixer is allowed to inspect the app and derive the issue from repo truth when explicit findings are missing.
- The fixer is implementation-focused, not planning-focused and not reviewer-focused.
- Ask the expert human only for real ambiguity, not for every visible UI change.
- App-first, shared-if-needed is the default edit scope.
- Approval still belongs to the blocking review workflow.

# Definition of done

This skill is complete when:
- the target app has been grounded in real code and real comparable screens
- the fix follows the existing MrSmith mini-app rules rather than a fresh design interpretation
- any real ambiguity has been escalated to the expert human instead of guessed
- the implemented result passes the equivalent of the blocking UI review post-gate, or the remaining blocking findings are reported explicitly

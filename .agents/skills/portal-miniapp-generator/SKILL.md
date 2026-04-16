---
name: portal-miniapp-generator
description: Use this skill for MrSmith portal mini-app generation and implementation planning. It turns a feature request or an approved Appsmith migration spec into a repo-fit implementation plan, selects an approved screen archetype, and prepares the UI review gates that must pass before coding and before signoff.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

This is the canonical repo-specific skill for MrSmith mini-apps.

Use it to:
- plan new mini-apps that fit the existing portal family
- convert an approved Appsmith migration spec into a repo-fit implementation plan
- prepare the UI review inputs and exceptions for blocking review
- hand off implementation to the dedicated UI fixer without leaving visual decisions implicit

This skill is the authority for mini-app generation and repo-fit planning.
UI approval belongs to `portal-miniapp-ui-review`.
Do not create a parallel rule system in ad hoc planning notes.

# When to use

Use this skill when:
- building a new mini-app under `apps/`
- planning backend + frontend work for a mini-app feature
- an Appsmith migration spec is approved and the next step is implementation planning

Do not use this skill when:
- auditing raw Appsmith exports or repositories
- extracting business behavior from Appsmith without an approved migration spec
- working on the Matrix-style launcher UI instead of a mini-app workspace
- performing the blocking UI review of a planned or implemented mini-app screen

Use these companion skills first when needed:
- `appsmith-audit` for reverse engineering the current Appsmith app
- `appsmith-migration-spec` for expert-in-the-loop specification drafting

Use this companion skill next when needed:
- `portal-miniapp-ui-fixer` for implementing the app UI once the plan and pre-gate are clear
- `portal-miniapp-ui-review` for blocking UI approval before coding and before signoff

# Required inputs

Before locking a plan or review:
- read `docs/IMPLEMENTATION-PLANNING.md`
- read `docs/IMPLEMENTATION-KNOWLEDGE.md`
- read `docs/UI-UX.md`
- inspect at least 2 comparable mini-app screens already present in the repo

If the source is an Appsmith migration, also read the approved migration spec before making layout or runtime decisions.

# Bundled resources

- `references/archetypes.md`
  Use to choose the smallest approved screen archetype that fits the app.
- `references/review-gates.md`
  Use to prepare layout, copy, metrics, and repo-fit expectations for the blocking UI review.
- `templates/implementation-plan.md`
  Use when producing the final implementation plan so every mini-app follows the same structure.

# Workflow

## Step 1: Inspect comparable apps

Always inspect at least 2 comparable mini-app screens already in the repo.

Record:
- the exact files inspected
- the layout patterns worth reusing
- the patterns rejected for this app

Do not claim consistency with the repo unless you have checked real screens.

## Step 2: Select an approved archetype

Choose exactly one primary archetype from `references/archetypes.md`.

Default rule:
- CRUD and single-entity management apps default to `master_detail_crud`

If no archetype fits cleanly, document the mismatch explicitly as an exception instead of inventing a new pattern silently.

## Step 3: Draft the implementation plan

Use `templates/implementation-plan.md`.

The plan must always include:
- `Comparable Apps Audit`
- `Archetype Choice`
- `User Copy Rules`
- `Repo-Fit`
- `Exceptions`

If any of those sections is missing, the plan is not ready.

## Step 4: Prepare the UI review handoff before coding

Validate the plan against `references/review-gates.md`.

Do not proceed to code if any gate fails, especially:
- comparable apps not inspected
- no archetype chosen
- technical or machine-facing UI copy
- invented KPIs or summary cards
- repo/runtime wiring left implicit

The plan must leave the reviewer with:
- 2 exact comparable repo screens
- one explicit archetype
- an `Exceptions` section
- copy and metric constraints clear enough for blocking review

## Step 5: Hand off implementation to the fixer

Hand off to `portal-miniapp-ui-fixer` when moving from the approved plan to code.

The fixer should inherit:
- the chosen archetype
- the cited comparable repo screens
- the `Exceptions` section
- the copy and metrics constraints

## Step 6: Require the blocking UI reviewer

Hand off to `portal-miniapp-ui-review` twice:
- pre-gate after the implementation plan is drafted
- post-gate after `portal-miniapp-ui-fixer` has completed the implementation work

Do not treat coding as complete until the post-gate reviewer approves.

# Core rules

- Default to the existing mini-app family, not to a brand-new visual direction.
- Use business-user-facing copy only. The default copy policy is `business-user-only`.
- Do not explain implementation mechanics to end users.
- Do not introduce hero banners, KPI cards, or decorative summaries by default.
- Metrics are allowed only when they are real, user-relevant, and explicitly justified by the feature.
- Reuse the clean mini-app design language from existing apps; do not drift toward launcher visuals.
- Any deviation from archetype, copy policy, or metric rules must be called out in `Exceptions`.

# Definition of done

This skill is complete when:
- the plan is grounded in real comparable apps from the repo
- one approved archetype has been selected explicitly
- copy, metrics, and layout expectations are explicit enough for blocking review
- repo/runtime fit is specified before implementation
- any deviation is documented as an explicit exception
- an Appsmith migration can move from approved spec to implementation without inventing UI behavior ad hoc
- the plan is ready to be handed to `portal-miniapp-ui-fixer` for implementation and `portal-miniapp-ui-review` for approval

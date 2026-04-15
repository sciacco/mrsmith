---
name: portal-miniapp-generator
description: Use this skill for any MrSmith portal mini-app generation or mini-app UI review. It turns a feature request or an approved Appsmith migration spec into a repo-fit implementation plan, selects an approved screen archetype, and enforces consistency gates for layout, copy, metrics, and runtime wiring before coding.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

This is the canonical repo-specific skill for MrSmith mini-apps.

Use it to:
- plan new mini-apps that fit the existing portal family
- convert an approved Appsmith migration spec into a repo-fit implementation plan
- review an existing mini-app screen and identify consistency regressions before or after implementation

This skill is the authority for mini-app generation and review. Do not create a parallel rule system in ad hoc planning notes.

# When to use

Use this skill when:
- building a new mini-app under `apps/`
- planning backend + frontend work for a mini-app feature
- reviewing screenshots or code for an existing mini-app
- checking whether a generated screen matches the portal mini-app family
- an Appsmith migration spec is approved and the next step is implementation planning

Do not use this skill when:
- auditing raw Appsmith exports or repositories
- extracting business behavior from Appsmith without an approved migration spec
- working on the Matrix-style launcher UI instead of a mini-app workspace

Use these companion skills first when needed:
- `appsmith-audit` for reverse engineering the current Appsmith app
- `appsmith-migration-spec` for expert-in-the-loop specification drafting

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
  Use to validate layout, copy, metrics, and repo-fit before coding and during review.
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

## Step 4: Run the review gates before coding

Validate the plan against `references/review-gates.md`.

Do not proceed to code if any gate fails, especially:
- comparable apps not inspected
- no archetype chosen
- technical or machine-facing UI copy
- invented KPIs or summary cards
- repo/runtime wiring left implicit

## Step 5: Review the implemented screen

After implementation, review the actual UI against:
- the selected archetype
- the comparable app family
- the review gates

If the review finds regressions, list findings first and block approval until they are resolved or documented as explicit exceptions.

# Core rules

- Default to the existing mini-app family, not to a brand-new visual direction.
- Use business-user-facing copy only. The default copy policy is `business-user-only`.
- Do not explain implementation mechanics to end users.
- Do not introduce hero banners, KPI cards, or decorative summaries by default.
- Metrics are allowed only when they are real, user-relevant, and explicitly justified by the feature.
- Reuse the clean mini-app design language from existing apps; do not drift toward launcher visuals.
- Any deviation from archetype, copy policy, or metric rules must be called out in `Exceptions`.

# Review output

When reviewing an existing implementation:
- findings come first, ordered by severity
- each finding should cite the screen, file, or screenshot evidence
- if there are no findings, say that explicitly and mention any residual verification gaps

# Definition of done

This skill is complete when:
- the plan is grounded in real comparable apps from the repo
- one approved archetype has been selected explicitly
- copy, metrics, and layout pass the review gates
- repo/runtime fit is specified before implementation
- any deviation is documented as an explicit exception
- an Appsmith migration can move from approved spec to implementation without inventing UI behavior ad hoc

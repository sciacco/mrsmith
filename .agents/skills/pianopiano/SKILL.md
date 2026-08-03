---
name: piano-piano
description: Produce implementation plans anchored to verified repository facts and rules, with two mandatory verification passes. Use when the user asks for an implementation plan, a plan-backed issue, or wants a plan reviewed before execution.
user-invocable: true
allowed-tools: Read Grep Glob Bash Edit Write
---

# Piano-Piano

Use this skill for implementation planning in this repository. Its purpose is to prevent plans from presenting inferred repository details as facts.

The skill is **planning-only** unless the user explicitly also asks to create or update a tracking issue. Do not edit production code, configuration, migrations, or tests while preparing a plan.

## Non-negotiable outcome

A plan is not ready until both passes below are complete. Every repository-specific claim in the plan must be one of:

- **Verified** — backed by an inspected repository source, cited with path and relevant symbol/section;
- **Product decision** — explicitly supplied or confirmed by the user;
- **Proposal / assumption** — clearly labelled as such, never phrased as an existing fact.

If a claim cannot receive one of these labels, remove it from the plan and investigate or ask a product question only when the project working contract actually requires one.

Do not invent component names, API methods, routes, schema fields, environment variables, service behavior, or established UX patterns.

## Pass 1 — Evidence and draft

### 1. Establish the applicable rules

1. Read all relevant `AGENTS.md` files from the current directory up to the repository root.
2. Read `docs/IMPLEMENTATION-PLANNING.md` and `docs/IMPLEMENTATION-KNOWLEDGE.md`, then only the handbook entries relevant to the affected domain.
3. For frontend/UI work, read `docs/UI-UX.md` and the matching skill instructions. For scoped work in an existing mini-app, this means `.agents/skills/tintoretto/SKILL.md`.
4. For database work, follow the repository database-safety instructions. Never connect to a database configured in an env file. Use versioned schemas/specifications and source code as evidence; ask the user to inspect live data if that is genuinely needed.

### 2. Build an evidence ledger before drafting

Investigate the actual codebase. At minimum, verify the chain affected by the request:

| Area | What must be checked |
|---|---|
| Comparable behavior | One or two screens/handlers that already perform the closest task |
| UI | Actual `packages/ui` exports and their props; existing app usage; do not infer from a design-system narrative |
| Frontend runtime | Routes, navigation, app bootstrap, API client capabilities, Vite/base/proxy only if the feature affects them |
| Backend | Handler registration, dependency structs, startup injection, auth middleware, error/logging patterns |
| Data | Authoritative versioned schema/specification, table keys, nullable fields, date/time semantics, and query ownership |
| External transport | Existing authenticated client and the exact source-supported request/response behavior |
| Deployment | Env/config, Docker/static serving, and migrations only when the proposed change actually touches them |

Record each fact as `path — symbol/section — observed fact`. A search-result snippet is a lead, not evidence: open the referenced file before treating it as verified.

### 3. Draft the plan from the ledger

- Start with the user-visible behavior and scope, separating it from implementation mechanics.
- State product decisions already given by the user. Apply safe repository-consistent defaults without creating speculative product questions.
- Name only verified files, symbols, components, API methods and dependencies.
- For proposed new routes/types/files, label them as proposed additions rather than existing artifacts.
- Cover the four required planning layers: product behavior; runtime/repository integration; data/auth contract; verification strategy.
- Respect the repository test rule: do not add automated tests unless the user has approved them. Still state meaningful type-check, build, and manual verification steps.
- For authenticated download/export flows, use an auth-capable client path and explicitly account for timeout, streaming/blob behavior, and server-side credential boundaries.

## Pass 2 — Adversarial verification

Review the complete draft sentence by sentence before showing it or placing it in an issue.

### Fact audit

For every repository-specific noun or assertion, reopen the cited source and verify:

- the component is actually exported and has the cited props;
- the comparable screen really uses the claimed pattern;
- the route is registered and does not collide with an existing route;
- the dependency is actually constructed and injectable at startup;
- the API client actually exposes the required transport method;
- table/column/type/nullability and timestamp semantics match the versioned schema;
- external endpoint shape and authorization pattern are supported by source/specification;
- env, Docker, Vite, and deployment statements are included only when their source proves they are affected.

Change any failed assertion to a labelled proposal, replace it with the verified equivalent, or remove it. Never preserve a convenient but unverified detail merely because it sounds conventional.

### Repo-fit audit

Run the relevant parts of `docs/IMPLEMENTATION-PLANNING.md`'s checklist explicitly:

1. runtime/deep-link fit;
2. dev and proxy fit;
3. role and bearer-auth fit;
4. identifiers, ownership and date/data-contract fit;
5. deployment/config/migration fit;
6. verification, sanitised failures, logging and timeout fit.

For UI screens, also apply `docs/UI-UX.md` §19. In particular, confirm component availability rather than assuming a desired component exists.

### Deliverable gate

Do not call the result “verified”, “ready”, or “approved” until both audits pass. The final plan must include:

1. **Verified facts** — concise evidence ledger with paths and symbols;
2. **Product behavior and decisions**;
3. **Implementation plan** — ordered, file-level where the target is verified;
4. **Verification plan**;
5. **Pass-2 outcome** — either “all repository claims rechecked” or a short list of explicitly labelled unresolved assumptions.

## GitHub issue handling

Only create or edit an issue when the user explicitly asks for it.

When an issue is requested:

1. Complete both planning passes first.
2. Put the reviewed plan, evidence section, scope, and acceptance criteria in the issue body.
3. Search for duplicates before creating a new issue.
4. Use the configured GitHub repository and `gh` CLI only after confirming authentication/repository context.
5. Re-read or query the created issue when network access allows; if verification fails, report that fact rather than claiming it was checked.
6. Do not create a branch, commit, or edit implementation files merely because an issue was created.

## Required final response

Be concise and state:

- whether Pass 1 and Pass 2 completed;
- the tracking issue URL, if one was requested;
- any remaining labelled assumption or genuine product decision;
- that no implementation changes were made while planning, unless the user explicitly requested another action.

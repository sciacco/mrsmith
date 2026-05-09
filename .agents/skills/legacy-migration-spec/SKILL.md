---
name: legacy-migration-spec
description: Use this skill after `legacy-app-auditor` or an equivalent source audit to turn evidence into an implementation-neutral migration specification. It is for expert-in-the-loop scope, parity, entity, workflow, logic, data, and integration decisions before handing the approved spec to MrSmith repo-fit planning.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

Convert source-audit evidence into an approved, implementation-neutral migration specification.

This skill owns the specification layer between reverse engineering and MrSmith implementation planning. It extracts what the audit proves, asks only for domain or product decisions that evidence cannot answer, and records intentional deviations from the source app.

After the specification is approved, hand it to `portal-miniapp-generator` for MrSmith-specific repo-fit planning, archetype selection, and UI review gates.

# When to use

Use this skill when:
- `legacy-app-auditor` or an equivalent audit has produced evidence artifacts
- the next step is to define product scope, source parity, entities, workflows, logic, and integrations for a rewrite
- a domain expert may need to answer gaps that static evidence cannot resolve
- the output should be stable enough for downstream implementation planning

Do not use this skill when:
- the source material is still raw and unaudited; run `legacy-app-auditor` first
- the team already has an approved migration spec
- the task is direct React, Go, SQL, or deployment implementation
- the task is MrSmith route/base-path/API-prefix planning; use `portal-miniapp-generator` after this spec is approved

# Expected inputs

Inputs may include:
- legacy audit Markdown or JSON
- migration fact sheet
- screenshots, schemas, API specs, or source references cited by the audit
- domain expert answers
- known constraints, desired changes, or out-of-scope items

If the audit is incomplete, state the missing evidence before asking the expert to compensate for it.

# Workflow

Work in phases. For non-trivial apps, keep phase outputs in Markdown files named like `<appname>-migspec-phaseA.md`, then assemble the final spec as `<appname>-spec.md`.

## Phase A: Scope and parity boundary

Extract:
- current app purpose
- user groups and permissions
- in-scope screens, workflows, reports, jobs, exports, and integrations
- out-of-scope source behavior
- source parity requirements
- intentional improvements or behavioral changes

Ask the expert only about scope, parity, and business decisions the audit cannot prove.

## Phase B: Entity and operation model

For each source entity or business object:
- list fields, identifiers, relationships, and ownership where evidence exists
- list read, create, update, delete, export, state-transition, and domain-specific operations
- preserve exact source names as evidence
- flag ambiguous IDs, join keys, enum values, defaults, and lifecycle states

Do not invent a cleaner domain model without marking it as a proposed deviation.

## Phase C: UX and workflow map

For each view or workflow:
- state the primary user intent
- classify the interaction pattern: CRUD, report explorer, wizard, settings form, data workspace, or other
- describe inputs, filters, actions, confirmations, empty/error states, exports, and navigation
- identify views that should be merged, split, removed, or redesigned

Keep this platform-neutral. Do not choose MrSmith components, routes, or visual layout.

## Phase D: Logic, data, and integration contracts

Specify:
- business rules and where they should live in the rewrite: backend, frontend, shared validation, or external system
- source data contracts: tables, endpoints, payloads, files, statuses, enums, filters, ordering, null/default handling
- integration side effects and orchestration order
- auth, role, and permission expectations
- partial-failure and retry behavior where source evidence exists
- risky facts that should become contract tests or validation checks later

If behavior is only inferred, keep it out of the contract until confirmed.

## Phase E: Specification assembly

Assemble one final specification with:
- summary and status
- current-state evidence sources
- in-scope and out-of-scope behavior
- entity catalog
- view and workflow specifications
- backend/API contract summary at a product level
- logic allocation
- integrations and side effects
- intentional deviations from the source app
- risky contract list
- open questions and deferred decisions

The final spec should be sufficient for `portal-miniapp-generator` without consulting raw source artifacts except for named gaps.

# Output rules

- Extract first, ask second.
- Cite audit sections or source evidence for important behavior.
- Keep source behavior separate from intended new behavior.
- Keep the spec implementation-neutral: no MrSmith app path, Vite port, Go package, React component, or deployment decision.
- Do not silently invent business rules, identifiers, filters, enum values, or UX intent.
- If the expert changes behavior, document both the source behavior and the approved target behavior.
- Identify candidate contract tests, but do not add tests from this skill.
- Carry unresolved gaps into the final spec instead of hiding them in prose.

# Definition of done

This skill is complete when:
- scope and parity are explicit
- major entities, operations, views, workflows, integrations, and permissions are covered
- risky source contracts are listed with evidence and suggested validation
- intentional deviations are documented
- open questions are precise and bounded
- the approved spec can be handed to `portal-miniapp-generator` for MrSmith repo-fit planning

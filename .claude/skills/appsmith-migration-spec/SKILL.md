---
name: appsmith-migration-spec
description: Use this skill after `appsmith-audit` to turn audit JSON or Markdown artifacts into a platform-neutral application specification through a phased conversation with a domain expert. Use it for Phase 2 migration discovery, specification drafting, and gap identification between the current Appsmith app and the intended new system.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

Convert `appsmith-audit` outputs into an implementation-ready, platform-neutral application specification.

This skill is for the expert-in-the-loop Phase 2 workflow. It should extract what the audit proves, surface decisions the audit cannot make, and maintain a structured specification as the conversation progresses.

After the specification is approved, hand it to `portal-miniapp-generator` for repo-specific implementation planning, UI review gates, and mini-app generation.

# When to use

Use this skill when:
- `appsmith-audit` has already produced JSON or Markdown artifacts for an Appsmith app
- the next step is to define entities, operations, user flows, integrations, and logic placement for a rewrite
- a domain expert can answer business and workflow questions that are not statically inferable

Do not use this skill when:
- the source material is still a raw Appsmith export and no audit exists yet
- the goal is direct code generation, scaffolding, or React/Go implementation
- the team already has a normalized, approved product specification

# Inputs

Expected inputs may include:
- `appsmith-audit` normalized JSON
- `appsmith-audit` Markdown artifacts
- domain expert clarifications
- migration constraints and non-functional requirements

If the audit is incomplete or noisy, say exactly what is missing before asking the expert to compensate for it.

# Bundled resources

- `templates/spec-outline.md`
  Use when assembling or updating the evolving specification so the output stays stable across sessions.
- `examples/app-audit.json`
  Use as a compact generic reference fixture for understanding the expected audit shape in production installs.

# Workflow

Work one phase at a time. Do not skip ahead. for reference and to easy expert work write down all phases finding and questions in markdown {appname}-migspec-{phase}.md

## Phase A: Entity-Operation Model

For each entity in the audit:
- list inferred operations, including domain-specific verbs
- aggregate likely fields from parameters, widgets, query columns, and bindings
- flag unknown types, constraints, and ambiguous relationships
- identify overlaps, merges, or missing entities

Present the extracted facts first. Then ask the expert only about gaps, merges, missing entities, and operation completeness.

## Phase B: UX Pattern Map

For each page or view:
- classify the interaction pattern
- state the primary user intent
- group widgets into logical UI sections
- flag mixed or unclear patterns

Ask whether the characterization is correct and whether any views should be merged, split, renamed, or removed in the new system.

## Phase C: Logic Placement

For each non-trivial JSObject method or inline expression:
- classify it as domain logic, orchestration, or presentation
- recommend backend, frontend, or shared placement
- flag duplication, fragility, or business-critical rules

Ask whether the logic is still desired and whether any current behavior should change rather than be ported.

## Phase D: Integration and Data Flow

- list external datasources and APIs with purpose
- map cross-view user journeys
- identify hidden triggers, timers, or automation-like behavior
- call out integrations or flows the export cannot reveal

Ask only about flows and integrations that are missing, changing, or strategically important.

## Phase E: Specification Assembly

Build and maintain a single specification document using `templates/spec-outline.md`.

After each expert answer:
- update the relevant section immediately
- keep extracted facts separate from deliberate changes to current behavior
- carry unresolved gaps into the open-questions section

# Output requirements

Produce one Markdown specification that is detailed enough for downstream design and implementation work without consulting the original Appsmith export.

The spec must include:
- entity catalog with fields, relationships, and operations
- view specifications with user intent and interaction pattern
- backend API contract summary
- logic allocation by responsibility
- integrations and end-to-end flows
- explicit open questions, deferred decisions, and constraints

# Rules

- Extract first, ask second.
- Cite Appsmith names exactly when discussing current-state evidence.
- Keep the specification platform-neutral. Do not prescribe React components, Go types, or Appsmith widgets.
- Never silently invent business rules, field semantics, or UX intent that the audit does not support.
- If the expert changes behavior, document both the current behavior and the intended new behavior.
- If the audit cannot answer a question, say so explicitly instead of filling the gap.

# Definition of done

This skill is complete when:
- all major entities, operations, views, and integrations from the audit have been covered
- the expert has only been asked to resolve true business or design decisions
- unresolved gaps are explicit
- the final Markdown spec can be handed to `portal-miniapp-generator` for downstream implementation planning without needing the raw Appsmith export

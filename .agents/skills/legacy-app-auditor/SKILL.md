---
name: legacy-app-auditor
description: Use this skill when reverse engineering a non-Appsmith legacy application, source repository, export, API collection, database-backed workflow, or live internal tool to produce evidence-based audit artifacts and a migration fact sheet. Use it before writing a migration spec or MrSmith implementation plan. Do not use it for direct code generation or repo-specific MrSmith planning.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

Reverse engineer an existing application into migration-ready evidence.

This skill is source-focused. It captures what the current app does, where the behavior is proven, and which facts are risky or unresolved. It must not design the MrSmith implementation and must not translate source screens directly into React or Go code.

# When to use

Use this skill when:
- the source is a legacy app, custom codebase, internal tool, export, API collection, BI/reporting app, or mixed evidence bundle
- the goal is to understand current behavior before a rewrite
- the next step is an implementation-neutral migration spec
- the team needs page/workflow, data, integration, permission, and side-effect evidence

Do not use this skill when:
- the source is an Appsmith export and `appsmith-audit` is available; use the Appsmith-specific auditor first
- the input is already a reviewed migration specification
- the task is direct code generation, scaffolding, or MrSmith repo-fit planning
- the goal is UI review or remediation inside an existing MrSmith mini-app

After this skill completes, hand the audit artifacts to `legacy-migration-spec`.

# Expected inputs

Inputs may include:
- source repository or export path
- screenshots or screen recordings
- API collections, OpenAPI specs, GraphQL schemas, SQL dumps, database schemas, or query files
- run instructions, environment notes, access notes, or user role descriptions
- domain notes about intended parity or known planned changes

If the source cannot be inspected, stop and ask for a concrete artifact or access path. Do not compensate by guessing.

# Evidence precedence

Use this order when evidence conflicts:

1. source code, query/action definitions, checked-in configuration, and executable workflow definitions
2. authoritative schemas, API specs, stored procedures, migrations, and database dumps
3. screenshots, recordings, logs, and observed live behavior
4. existing migration knowledge and prior reviewed specs
5. human notes and memory
6. model inference

If lower-precedence evidence conflicts with higher-precedence evidence, record the conflict and prefer the higher-precedence source unless new evidence proves otherwise.

# Workflow

## Step 1: Locate and classify the source

Identify:
- source type and technology stack
- app entrypoints and route or screen structure
- available evidence files
- unavailable or inaccessible evidence
- roles, environments, and external dependencies when visible

Record the structure before drawing conclusions.

## Step 2: Build the application inventory

Inventory:
- screens, routes, reports, jobs, or commands
- user roles and permission checks
- navigation and cross-screen workflows
- read and write data sources
- external APIs, queues, files, exports, emails, webhooks, or automation
- shared utilities where business behavior may be hidden

Preserve original names from the source.

## Step 3: Audit each screen or workflow

For every meaningful user surface or automated flow, capture:
- user intent
- inputs, filters, defaults, and derived values
- load behavior and refresh behavior
- actions, mutations, confirmations, and destructive flows
- validation, disabled/visibility logic, and permission effects
- success, empty, error, and partial-failure behavior
- navigation, export, notification, and integration side effects

Separate business rules from source-framework mechanics.

## Step 4: Map data and integration contracts

For each data source, query, endpoint, procedure, or integration:
- purpose and owning workflow
- exact identifiers: table, column, function, endpoint, payload field, enum, status, key, or file name
- inputs and where they come from
- output shape and consumers
- filtering, ordering, grouping, pagination, null handling, fallback/default behavior
- side effects and orchestration order
- auth or credential assumptions

Do not normalize names or join keys without evidence.

## Step 5: Extract hidden rules and risks

Call out:
- business rules embedded in UI state, helper functions, SQL, stored procedures, formulas, or templates
- cross-system identity mappings and join keys
- status machines and allowed transitions
- exclusions, eligibility filters, special constants, and magic fallback values
- client-side security or permission assumptions
- duplicated or inconsistent logic
- behavior that appears accidental and needs domain confirmation

Mark each item as `verified`, `inferred`, `conflicting`, or `unresolved`.

## Step 6: Build the migration fact sheet

For drift-prone behavior, produce a compact fact sheet with:
- fact title
- verified behavior
- exact evidence
- implementation risk
- likely contract test or validation check
- unresolved questions, if any

The fact sheet is the primary handoff to `legacy-migration-spec`.

# Output format

Use this structure unless the user requests another artifact shape:

```markdown
# <App Name> Legacy Audit

## Application Inventory

## Evidence Map

## Screen and Workflow Audits

## Data and Integration Catalog

## Migration Fact Sheet

## Risks and Open Questions

## Handoff Notes for legacy-migration-spec
```

# Core rules

- Evidence first, inference second.
- Cite source paths, query names, endpoint names, screenshots, or schema references for every important behavior.
- Keep `verified`, `inferred`, `conflicting`, and `unresolved` behavior clearly separated.
- Do not invent business semantics, field meanings, SQL, join keys, enum values, or target APIs.
- Do not make MrSmith route, Vite, backend module, UI component, or deployment decisions.
- Lightweight target-shape classification is allowed, such as CRUD, report explorer, wizard, settings form, or data workspace.
- Document intended improvement candidates separately from current behavior.
- If source evidence is incomplete, say exactly what could not be verified.

# Definition of done

This skill is complete when:
- source structure and evidence coverage are explicit
- every meaningful screen, route, report, or workflow has been inventoried
- data sources, writes, integrations, roles, and side effects are cataloged
- risky contracts are captured in a migration fact sheet with evidence
- open questions are precise enough for a domain expert to answer
- downstream `legacy-migration-spec` work can proceed without reopening the source except for named gaps

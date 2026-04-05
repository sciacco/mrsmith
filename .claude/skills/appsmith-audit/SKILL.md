---
name: appsmith-audit
description: Use this skill when analyzing an Appsmith repository or export zip to inventory pages, widgets, datasources, queries, JSObjects, bindings, and hidden workflow logic. This skill produces audit artifacts for migration planning and reverse engineering. Do not use it for direct code generation or greenfield React apps.
user-invocable: true
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash
---

# Purpose

Analyze an Appsmith application and produce a structured audit of its current behavior, dependencies, and hidden logic.

This skill is for reverse engineering and migration discovery. It must not treat Appsmith JSON as a source format for direct React conversion.

# When to use

Use this skill when:
- the workspace contains an Appsmith export zip or extracted Appsmith repository
- the goal is to understand how an existing Appsmith app works
- the goal is to create migration inputs for a rewrite to custom frontend and backend code
- the team needs a page-by-page inventory before redesign or refactoring

After this skill has produced the structural audit, hand the JSON or Markdown artifacts to `appsmith-migration-spec` for the expert-led specification phase.

Do not use this skill when:
- building a new app from scratch
- generating React components directly from Appsmith bindings
- the input is already a normalized product specification

# Inputs

Expected inputs may include:
- an Appsmith export zip
- an extracted Appsmith repository
- page JSON files
- datasource definitions
- JSObjects
- optional user notes about the intended target architecture

# Required outputs

Produce these artifacts whenever the source material allows:

1. **Application inventory**
   - app name
   - page list
   - datasource list
   - global findings
   - migration risks

2. **Per-page audit**
   - page purpose
   - widgets and their roles
   - queries and actions used by the page
   - event flow
   - bindings and dependencies
   - hidden logic
   - open questions or ambiguities

3. **Datasource and query catalog**
   - datasource type
   - query name
   - query purpose
   - parameters and dependencies
   - whether it belongs in backend or frontend orchestration later

4. **Findings summary**
   - duplicated logic
   - business rules embedded in UI
   - fragile bindings
   - candidate domain entities
   - migration blockers

# Core rules

- Never generate target React code from raw Appsmith bindings.
- Treat `{{ ... }}` expressions as audit evidence, not implementation guidance.
- Separate every finding into one of these buckets whenever possible:
  1. business logic
  2. frontend orchestration
  3. presentation
- Identify implicit dependencies between widgets, queries, JSObjects, page load actions, and navigation.
- Flag logic hidden in visibility, disabled state, default values, derived data, and chained actions.
- Prefer normalized Markdown outputs that can be reviewed by engineers and reused as migration input.
- When the source is incomplete, say what is missing and what could not be verified.
- Preserve original names of pages, queries, widgets, and JSObjects unless there is a strong reason to alias them.

# Audit procedure

## Step 1: Locate the source structure

Find and classify:
- pages
- widgets
- actions
- queries
- JSObjects
- datasources
- theme or layout metadata when relevant

Record the repository or archive structure before deeper analysis.

## Step 2: Build the application inventory

Create a top-level inventory of:
- all pages
- all datasources
- all named queries/actions
- all JSObjects
- navigation patterns
- obvious cross-page reuse

## Step 3: Audit each page

For each page, extract:
- purpose of the page
- major widgets
- data sources consumed
- initial load behavior
- user-triggered events
- save/update/delete/export actions
- navigation targets
- dependencies on selected rows, form state, temporary store values, and JSObject methods

## Step 4: Analyze bindings and hidden logic

Inspect bindings such as:
- widget visibility
- disabled state
- default values
- derived display values
- transformation expressions
- chained triggers
- references to selected rows, current items, or query results

Classify each finding as one of:
- likely business rule
- UI orchestration rule
- presentation-only behavior

## Step 5: Map datasources and query intent

For every datasource and query, determine:
- what entity or workflow it appears to support
- whether it reads or writes data
- what parameters it depends on
- whether the logic should likely move to backend APIs in a rewrite

## Step 6: Surface risks and migration notes

Call out:
- duplicated logic across pages
- direct database access from UI
- security-sensitive logic in the client
- tightly coupled widgets and queries
- unclear state dependencies
- places where Appsmith behavior may be hard to reproduce without redesign

## Step 7: Generate audit artifacts

Write the final output using the provided templates when available.

# Output format

Use this structure unless the user requested a different format.

## Application inventory
- application name
- source type
- pages
- datasources
- JSObjects
- global notes

## Page audits
One section per page with:
- purpose
- widgets
- actions and queries
- event flow
- hidden logic
- candidate domain entities
- migration notes

## Datasource catalog
One section per datasource/query with:
- name
- type
- purpose
- inputs
- outputs
- dependencies
- rewrite recommendation

## Findings summary
- embedded business rules
- duplication
- security concerns
- migration blockers
- recommended next steps

# Definition of done

This skill is complete when:
- every page in the source has been inventoried
- all visible datasources and queries are cataloged
- major hidden logic has been called out
- findings are classified into business logic, orchestration, or presentation where possible
- the output is useful as direct input for a migration PRD or architecture plan
- downstream Phase 2 work can proceed from the generated artifacts without reopening raw Appsmith exports unless the audit is incomplete

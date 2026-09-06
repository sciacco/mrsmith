---
name: piano-piano
description: Produce proportionate implementation plans grounded in verified repository facts, with a factual review and a subtractive review before delivery. Use when the user asks for an implementation plan, a plan-backed issue, or wants a plan reviewed before execution.
user-invocable: true
allowed-tools: Read Grep Glob Bash Edit Write
---

# Piano-Piano

Produce the smallest plan that lets an implementer deliver the requested behavior correctly. Ground it in repository facts without turning ordinary implementation work into an exhaustive specification.

The skill is **planning-only** unless the user explicitly also asks to create or update a tracking issue. Do not edit production code, configuration, migrations, or tests while preparing a plan.

## Core rules

- **Proportion before completeness.** A new page in an existing app does not need the same investigation or document as a new application. Length and detail must follow demonstrated complexity, not hypothetical risks.
- **No implicit scope expansion.** Include functionality only when the user requested it or it is indispensable to satisfy a requested requirement. Do not add features to address hypothetical risks. Ordinary defensive engineering does not become a separate feature or workstream.
- **Preserve implementer autonomy.** A delegable plan clarifies the goal, affected files, non-obvious constraints and sequence. It does not prescribe every routine engineering choice, even for a mid-level agent.
- **Separate preparation from delivery.** Evidence notes and audit checklists support planning; they are not mandatory sections of the delivered plan.
- **Keep product and implementation distinct.** Preserve confirmed product decisions. Apply safe repository-consistent defaults without manufacturing questions. Ask only when the project working contract's conditions for a genuine product decision are met.

## Pass 1 — Scope, evidence and draft

### Establish scope and applicable rules

1. Identify what actually changes and what remains unchanged before investigating.
2. Read applicable `AGENTS.md` files, `docs/IMPLEMENTATION-PLANNING.md` and `docs/IMPLEMENTATION-KNOWLEDGE.md`; load only relevant handbook entries.
3. For UI work, read `docs/UI-UX.md` and the matching skill. For a scoped change in an existing mini-app, use `.agents/skills/tintoretto/SKILL.md`.
4. Follow database safety: never connect to a database configured in an env file. Use versioned schemas/specifications and source code; ask the user for a data inspection only when genuinely necessary.

### Investigate only the affected chain

For a page in an existing app, start with a comparable page/handler, the required data, endpoint and route. Check actual shared-component exports/props and client capabilities where the plan relies on them.

Investigate bootstrap, dependency injection, hosting, proxies, deployment, credentials or migrations **only when the change affects them or concrete evidence exposes an integration problem**. Do not re-audit unchanged infrastructure merely to complete a checklist. Apply the same relevance filter to the planning reference's broader checklist.

Concentrate analysis on demonstrated difficulties. One uncertain data calculation does not justify escalating scrutiny across every other part of the feature. Treat an uncertainty as a blocker only when it prevents correct implementation of a requested requirement; state exactly what evidence would resolve it.

Keep working evidence notes as needed: `path — symbol/section — observed fact`. Open relevant sources; search snippets alone are not evidence. Do not invent component names, API methods, routes, schema fields, dependencies or established behavior.

Every repository claim in the draft must be supported by an inspected source. Distinguish existing facts from user decisions and proposed additions using plain wording; a formal label system is not required. Cite paths where they help the implementer, not to publish an evidence catalogue.

### Draft an actionable plan

- Start with the result and scope boundaries, without repeating a product specification already present in the issue.
- Give ordered implementation steps and relevant file paths. Mark new files, routes or contracts as proposed, not existing.
- Explain non-obvious calculations, ownership rules or integration constraints when needed for correctness. Leave routine validation, cleanup and code structure to the implementer unless there is a concrete trap.
- Use the existing architecture and conventions rather than introducing abstractions, services or dependencies for anticipated future needs.
- For exports/downloads, verify the actual transport and auth needs. Do not automatically introduce streaming, jobs, progress, cancellation or a new renderer service.
- Include a few decisive checks tied directly to requested behavior, plus applicable type-check/build and smoke commands. Do not expand every possible edge case into a mandatory checklist item.
- Respect the repository test rule: no new automated tests without user approval. A planning request does not grant that approval.

## Pass 2 — Factual and subtractive review

Both reviews are required before delivery, but their reports are not deliverables.

### Factual review

Check the draft against inspected sources and applicable rules. Reopen sources when support is uncertain. Confirm the files, APIs, components, data semantics and affected integration points actually support the proposed work. Replace unsupported claims with explicit proposals, investigate consequential uncertainties, or remove the claims.

Apply relevant repo-fit checks and, for UI work, `docs/UI-UX.md` §19. Do not turn these internal checks into new project scope or mandatory sections about unchanged systems.

### Subtractive review

For each proposed activity, constraint and verification, ask:

- Which requested requirement does this serve?
- Is it necessary now, or does it address a hypothetical risk?
- Can a competent implementer resolve it as ordinary engineering work?
- Is it already stated elsewhere?

**Remove unnecessary material even when it is technically correct.** Remove unrequested functionality, duplicate requirements, speculative blockers and routine prescriptions. Keep detail where omission would materially risk the requested behavior.

A plan is not ready merely because its claims are accurate. It must also remain within scope and be proportionate. Do not hide consequential unresolved assumptions behind a claim of readiness.

## Default deliverable

For a scoped change, use a short plan containing:

1. Result and boundaries, only if not already clear from the accompanying specification.
2. Ordered implementation steps with affected files.
3. A concrete difficulty or dependency, if one actually exists.
4. A small set of requirement-driven verification steps.

Expand only for demonstrated complexity or an explicit user request for additional detail. There are no mandatory evidence-ledger, architecture-layer, audit-outcome or exhaustive acceptance-checklist sections. Do not add headings with no useful content.

## GitHub issue handling

Only create or edit an issue when explicitly requested.

1. Complete both planning passes before publishing.
2. Preserve the product specification and add the concise implementation plan without duplicating it.
3. Search for duplicates before creating a new issue.
4. Confirm GitHub repository/auth context and use `gh`.
5. Re-read the updated issue to verify the intended content; report verification failure rather than claiming success.
6. Do not create a branch, commit or edit implementation files merely because an issue was updated.

## Final response

State the outcome concisely, with the issue URL if relevant. Mention only material unresolved dependencies or limitations. Do not routinely narrate the planning passes, evidence gathering or internal checklists.

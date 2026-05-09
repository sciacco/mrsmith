# Grappa DCIM Orchestration Plan

## Purpose

Coordinate Grappa DCIM implementation through written Markdown contracts so context survives across specialized LLM agents and QA iterations.

The orchestrator must not rely on hidden chat memory. Every handoff, implementation result, QA finding, and remediation decision is recorded under `apps/grappa-dcim/docs/`.

## Source Contracts

Implementation and QA agents must read these documents before touching code:

- `apps/grappa-dcim/docs/grappa-dcim-spec.md`
- `apps/grappa-dcim/docs/foundation-impl.md`
- `apps/grappa-dcim/docs/facilities-layout-impl.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-impl.md`
- `apps/grappa-dcim/docs/cabling-crossconnects-impl.md`
- `apps/grappa-dcim/docs/fiber-topology-artifacts-impl.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `docs/grappa/GRAPPA.md`

Agents must also read the specific schema files named by their slice plan.

## Slice Order

1. `foundation`
2. `facilities-layout`
3. `equipment-compute-storage`
4. `cabling-crossconnects`
5. `fiber-topology-artifacts`
6. overall QA and integration remediation

This order keeps shared wiring first, then builds domain slices that depend on the app shell and common backend helpers.

## Mandatory Planning Pre-Gate

Before coding begins, run a blocking mini-app UI planning review against:

- the five slice plans
- `docs/UI-UX.md`
- `.agents/skills/portal-miniapp-generator/references/review-gates.md`

The reviewer must write:

- `apps/grappa-dcim/docs/planning-ui-review.md`

Required status values:

- `PASS`: implementation loop may start.
- `FAIL`: the orchestrator writes `apps/grappa-dcim/docs/planning-remediation.md`, updates the relevant `*-impl.md` plans, and repeats the planning review.

Do not start implementation while `planning-ui-review.md` is missing or `FAIL`.

## Iteration Loop Per Slice

For each slice in order:

1. Orchestrator writes a run contract:
   - path: `apps/grappa-dcim/docs/{slice}-run.md`
   - content: current slice status, accepted dependencies, allowed write ownership, required docs, and verification commands.
2. Orchestrator spawns one implementation agent.
3. Implementation agent reads only written contracts and repo files, performs code changes, and writes:
   - `apps/grappa-dcim/docs/{slice}-implementation-report.md`
4. Orchestrator spawns one QA gate agent.
5. QA gate agent reads the slice plan, run contract, implementation report, changed files, and relevant source docs. It writes:
   - `apps/grappa-dcim/docs/{slice}-qa.md`
6. If QA writes `PASS`, the orchestrator marks the slice accepted in `apps/grappa-dcim/docs/orchestration-state.md`.
7. If QA writes `FAIL`, the orchestrator writes:
   - `apps/grappa-dcim/docs/{slice}-remediation-{n}.md`
   Then it respawns or reuses the implementation agent with that remediation document. The implementation agent writes:
   - `apps/grappa-dcim/docs/{slice}-fix-{n}-report.md`
   QA repeats until `PASS`.

No agent handoff may happen only through chat. The chat prompt may point to Markdown files, but the durable instruction/result must be in Markdown.

## Run Contract Template

Each `{slice}-run.md` must use this structure:

```markdown
# {Slice} Run Contract

## Status

- Iteration:
- Dependency status:
- Allowed write scope:
- Disallowed write scope:

## Required Reading

- ...

## Implementation Target

- ...

## Verification Required

- Commands:
- Manual/browser checks:
- UI review states:

## Reporting Required

Write `{slice}-implementation-report.md` with:
- files changed
- behavior implemented
- contracts preserved
- commands run and outputs summarized
- unresolved questions
- deviations from plan
```

## Implementation Agent Prompt Contract

The orchestrator prompt for a slice implementation agent must be short and file-driven:

```text
Implement the Grappa DCIM slice defined in apps/grappa-dcim/docs/{slice}-run.md.

You are not alone in the codebase. Do not revert edits made by others. Stay inside the write scope in the run contract unless the contract explicitly allows a shared file edit. Read all required docs listed in the run contract. Preserve user-facing copy, repo-fit, auth, and data contracts from the slice plan.

When finished, write apps/grappa-dcim/docs/{slice}-implementation-report.md. Do not treat the slice as complete until that report exists.
```

## QA Gate Agent Prompt Contract

The orchestrator prompt for a slice QA gate agent must be short and file-driven:

```text
Perform the QA gate for the Grappa DCIM slice using apps/grappa-dcim/docs/{slice}-run.md and apps/grappa-dcim/docs/{slice}-implementation-report.md.

Review the changed files against the slice plan, the approved source spec, docs/UI-UX.md, docs/IMPLEMENTATION-PLANNING.md, and docs/IMPLEMENTATION-KNOWLEDGE.md. Check product behavior, repo/runtime integration, data/auth contracts, verification, and UI review gates.

Write apps/grappa-dcim/docs/{slice}-qa.md with status PASS or FAIL. If FAIL, include concrete findings with file paths, expected behavior, actual behavior, severity, and remediation instructions. Do not rely on chat-only findings.
```

## Slice QA Gate Requirements

Each slice QA file must include:

- `Status: PASS` or `Status: FAIL`
- source docs checked
- changed files inspected
- product behavior findings
- repo/runtime findings
- data/auth findings
- UI findings
- verification commands run
- manual/browser checks run or explicitly not run
- residual risks

QA must fail if any of these are true:

- implementation contradicts `grappa-dcim-spec.md`
- UI uses launcher/hero/dashboard language instead of mini-app workspace language
- Viewer can mutate data or see server credentials
- Operativo cannot perform a required approved action
- destructive action lacks backend dependency checks and double confirmation
- protected downloads/uploads bypass bearer auth
- app route/base/API prefix/static hosting is left inconsistent
- implementation invents CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici` workflow in V1
- required implementation report is missing
- tests were added without human approval

## Overall QA

After every slice has `Status: PASS`, the orchestrator spawns one overall QA agent.

The overall QA agent reads:

- every `*-impl.md`
- every `*-run.md`
- every `*-implementation-report.md`
- every `*-qa.md`
- current git diff
- mandatory source docs

It writes:

- `apps/grappa-dcim/docs/overall-qa.md`

Overall QA must check:

- build passes for frontend and backend, or failures are recorded with exact causes
- app shell and all slice routes work together
- nav labels and route paths are consistent
- API prefix and backend registration are complete
- launcher, env, CORS, Docker static copy, and Vite base are complete
- Viewer/Operativo behavior is consistent across slices
- destructive-action contract is uniform
- no V2/out-of-scope features leaked into V1
- UI review post-gate expectations are covered with screenshots or manual browser notes
- `docs/IMPLEMENTATION-KNOWLEDGE.md` is updated only if implementation discovered reusable facts not already documented

If overall QA writes `FAIL`, the orchestrator maps each finding to the owning slice and writes one or more remediation documents:

- `apps/grappa-dcim/docs/overall-remediation-{n}.md`

Then the responsible slice loop repeats: implementation agent, QA gate agent, and overall QA again.

The process ends only when:

- every slice QA is `PASS`
- `overall-qa.md` is `PASS`
- planning and post-implementation UI review gates are `PASS`

## Orchestration State File

The orchestrator must maintain:

- `apps/grappa-dcim/docs/orchestration-state.md`

Required format:

```markdown
# Grappa DCIM Orchestration State

## Current Status

- Planning UI review:
- Overall QA:

## Slices

| Slice | Iteration | Implementation report | QA report | Status | Notes |
|---|---:|---|---|---|---|
| foundation | 0 | | | pending | |
| facilities-layout | 0 | | | pending | |
| equipment-compute-storage | 0 | | | pending | |
| cabling-crossconnects | 0 | | | pending | |
| fiber-topology-artifacts | 0 | | | pending | |

## Decisions

- YYYY-MM-DD: ...

## Open Questions

- ...
```

The orchestrator updates this file after every implementation report, QA report, and remediation pass.

## Context Preservation Rules

- Chat is not a durable contract.
- Every agent must write a Markdown output artifact before the next agent starts.
- Remediation instructions must quote or link the QA finding and identify the owning slice.
- If an implementation agent discovers a blocker, it writes it to its implementation report and stops.
- If the blocker needs human expertise, the orchestrator collects the exact question in `orchestration-state.md` before asking the human.
- Any accepted deviation from a slice plan must be added to that slice `*-impl.md` or a dated decision in `orchestration-state.md` before implementation continues.

## Human Approval Points

Ask the human expert before:

- adding automated tests
- changing V1 scope
- enabling server credential writes before `pwd_utenza_cliente` behavior is validated
- implementing CWDM, TIM GEA, Hive sync, polling, alerting, or first-class `cassetti_ottici`
- changing role names
- introducing a new frontend design archetype
- changing source table semantics or normalizing legacy free-text values

## Final Acceptance Contract

The final handoff is complete when these files exist and show pass status:

- `apps/grappa-dcim/docs/planning-ui-review.md`
- `apps/grappa-dcim/docs/foundation-qa.md`
- `apps/grappa-dcim/docs/facilities-layout-qa.md`
- `apps/grappa-dcim/docs/equipment-compute-storage-qa.md`
- `apps/grappa-dcim/docs/cabling-crossconnects-qa.md`
- `apps/grappa-dcim/docs/fiber-topology-artifacts-qa.md`
- `apps/grappa-dcim/docs/overall-qa.md`

The final response to the human must summarize only:

- pass/fail status
- files changed
- verification performed
- residual risks or human decisions still open

# legacy-migration-spec Skill Memory

## Purpose

- Native Codex specification skill for non-Appsmith legacy migrations.
- Converts audited source evidence into an implementation-neutral migration spec.

## Guardrails

- Do not consume raw source directly when an audit is missing; run `legacy-app-auditor` first.
- Keep source behavior, approved target behavior, intentional deviations, and unresolved gaps distinct.
- Preserve evidence citations for risky contracts.
- Do not choose MrSmith routes, app paths, Vite ports, UI components, or deployment wiring; that belongs to `portal-miniapp-generator`.

## Workflow Notes

- Generic legacy flow: `legacy-app-auditor -> legacy-migration-spec -> portal-miniapp-generator`.
- The final spec should give `portal-miniapp-generator` enough product, data, auth, and workflow context to perform repo-fit planning without reopening raw legacy source artifacts except for explicit gaps.

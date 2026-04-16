# portal-miniapp-ui-review Skill Memory

## Purpose

- Native Codex mirror of the MrSmith blocking UI review skill.
- Lives under `.agents/skills` so Codex can discover it as a repo-scoped skill without replacing the legacy `.claude/skills` copy yet.

## Blocking Defaults

- Prefer screenshots when available, but do not block solely because they are missing.
- Block hero or banner shells on CRUD/data-workspace screens unless they are explicit exceptions.
- Block raw auth/backend/transport error text in user-facing UI.
- Block invented KPI or stat cards without approved-plan justification.
- Block shared page-shell abstractions that were introduced before any real screen passed review.

## Workflow Notes

- The reviewer owns UI approval, not implementation planning.
- Findings must cite the violated gate plus file evidence and screenshots when available.
- Missing screenshots are not blocking by themselves; missing grounded evidence still is.
- Blocked screens should be remediated through `portal-miniapp-ui-fixer`; the reviewer does not implement the fix.
- Keep the `.claude/skills` copy in sync until the repo fully switches to native Codex paths.

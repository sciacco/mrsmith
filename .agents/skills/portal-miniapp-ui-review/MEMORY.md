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

## Recent Reviews

- **Coperture** — 2026-04-17
  - Code-first post-gate approval was grounded by the implemented route and screen files (`apps/coperture/src/routes.tsx`, `apps/coperture/src/App.tsx`, `apps/coperture/src/pages/CoverageLookupPage.tsx`, and shared CSS), even without screenshots.
  - The screen passed because the approved `report_explorer` composition remained intact: compact title, working toolbar as the primary surface, explicit search/reset flow, submitted-only address summary, one dominant results table, and business-user copy for empty/error/503 states.
  - Subtle page/surface gradients were acceptable because they did not become a hero shell or compete with the working surface. The blocking threshold remains decorative framing that dominates the workspace, not any use of texture or depth.

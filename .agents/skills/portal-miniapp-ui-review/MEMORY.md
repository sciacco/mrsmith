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
- **Energia in DC** — 2026-04-18
  - Code-first post-gate approval was grounded by the approved plan (`apps/zammu/energia-in-dc-impl.md`), the cited comparables (`apps/coperture/src/pages/CoverageLookupPage.tsx`, `apps/reports/src/pages/AovPage.tsx`, `apps/panoramica-cliente/src/pages/IaaSPayPerUsePage.tsx`), and the implemented route/app/page files under `apps/energia-dc/src/`.
  - The app passed because the shared shell stayed subordinate to the workspace surfaces: compact headers, filter toolbars, one dominant chart/table/master-detail surface per route, no KPI cards, no launcher-style hero, and explicit empty/error/503 states aligned with the approved `data_workspace` plan.
  - The review caught one copy-risk before signoff: bootstrap/startup fallbacks are still user-facing UI and must not surface raw auth/config/bootstrap errors. Energia DC passed only after its fatal bootstrap screen was rewritten to generic business-facing copy.
  - Screenshots were not captured during review; code evidence was sufficient for approval, but visual verification on desktop/narrow populated states remains a residual risk rather than a blocking defect.

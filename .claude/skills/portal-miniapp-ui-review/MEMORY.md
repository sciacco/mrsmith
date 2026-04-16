# portal-miniapp-ui-review Skill Memory

## Purpose

- Standalone blocking reviewer for portal mini-app UI approval.
- Use at two checkpoints:
  - pre-gate after `portal-miniapp-generator`
  - post-gate before implementation signoff

## Blocking Defaults

- Prefer screenshots when available, but do not block solely because they are missing.
- Block hero or banner shells on CRUD/data-workspace screens unless they are explicit exceptions.
- Block raw auth/backend/transport error text in user-facing UI.
- Block invented KPI or stat cards without approved-plan justification.
- Block shared page-shell abstractions that were introduced before any real screen passed review.

## Seed Regression

- **Richieste Fattibilita** — 2026-04-16
  - Regressions that must be blocked:
    - decorative hero/banner shell on `Consultazione RDF`
    - raw `Unauthorized` rendered in the empty/error surface
    - workspace composition drifting away from comparable repo list screens
    - hidden primary filters contributing to an empty polished shell instead of a working surface

## Workflow Notes

- The reviewer owns UI approval, not implementation planning.
- Findings must cite the violated gate plus file evidence and screenshots when available.
- Missing screenshots are not blocking by themselves; missing grounded evidence still is.
- Native Codex mirrors now exist under `.agents/skills/portal-miniapp-ui-review`, with a repo-scoped custom reviewer agent at `.codex/agents/portal-ui-reviewer.toml`.

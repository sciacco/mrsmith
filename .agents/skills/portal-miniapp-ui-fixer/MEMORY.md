# portal-miniapp-ui-fixer Skill Memory

## Purpose

- Native Codex implementation skill for fixing UI in a specific MrSmith mini-app.
- Requires the target app path under `apps/`; screenshots, blocked findings, and approved plans are optional accelerators.

## Defaults

- Treat `portal-miniapp-generator`, `portal-miniapp-ui-review`, `docs/UI-UX.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, and `docs/IMPLEMENTATION-PLANNING.md` as the governing sources of truth.
- Prefer app-local fixes first; allow shared `packages/` or shared style changes only when the issue clearly belongs there.
- Ask the expert human only for real ambiguity, not for routine visual corrections.
- Mandatory post-fix review is part of done; the fixer does not self-exempt from the blocking gates.
- When screenshots are impractical, use the reviewer's code-first fallback and record the residual visual verification gap explicitly.

## Seed Regressions

- **Richieste Fattibilita** — 2026-04-16
  - Regressions the fixer should correct decisively when they appear:
    - decorative hero or banner shell on CRUD/data-workspace screens
    - raw `Unauthorized` or transport/backend text in the user-facing surface
    - workspace composition drifting away from comparable repo screens
    - hidden primary filters that weaken the main working surface

## Workflow Notes

- Native flow is now: `portal-miniapp-generator -> portal-miniapp-ui-review pre-gate -> portal-miniapp-ui-fixer -> portal-miniapp-ui-review post-gate`.
- The fixer owns implementation only; approval still belongs to the reviewer.

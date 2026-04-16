# portal-miniapp-generator Skill Memory

## Purpose

- Native Codex mirror of the MrSmith mini-app planning skill.
- Lives under `.agents/skills` so Codex can discover it as a repo-scoped skill without replacing the legacy `.claude/skills` copy yet.

## Guardrails

- Always inspect at least 2 comparable mini-app screens in the repo before proposing layout or copy.
- Default CRUD and single-entity registries to `master_detail_crud`.
- Keep mini-apps inside the existing clean family anchored by `budget`, `listini-e-sconti`, and `reports`.
- Treat `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, and `docs/UI-UX.md` as mandatory preflight references.

## Workflow Notes

- Appsmith flow is now: `appsmith-audit -> appsmith-migration-spec -> portal-miniapp-generator -> portal-miniapp-ui-review pre-gate -> portal-miniapp-ui-fixer -> portal-miniapp-ui-review post-gate`.
- The plan produced by this skill is the per-app contract and the handoff artifact for both the fixer and the blocking UI reviewer.
- Keep the `.claude/skills` copy in sync until the repo fully switches to native Codex paths.
- For `apps/kit-products` settings registries, the repo-fit pattern is a compact `master_detail_crud` page reusing `SettingsPage.module.css`, selection-driven `Modifica`, modal create/edit, and explicit empty/error states rather than a side detail panel.
- `common.vocabulary` is not universally read-only in mini-apps: `kit_product_group` can be admin-managed from `kit-products`, but runtime consumers may still intentionally keep using `common.vocabulary.name` while translations stay administrative-only for that feature slice.

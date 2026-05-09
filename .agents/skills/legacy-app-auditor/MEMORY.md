# legacy-app-auditor Skill Memory

## Purpose

- Native Codex source-audit skill for non-Appsmith legacy applications.
- Produces evidence-based audit artifacts and a migration fact sheet before specification work.

## Guardrails

- Do not generate target code or MrSmith implementation plans from raw source evidence.
- Preserve source names and cite evidence for important behavior.
- Separate verified, inferred, conflicting, and unresolved facts.
- Treat hidden rules in UI state, helper functions, SQL, stored procedures, reports, and integrations as first-class migration risks.

## Workflow Notes

- Generic legacy flow: `legacy-app-auditor -> legacy-migration-spec -> portal-miniapp-generator -> portal-miniapp-ui-review pre-gate -> portal-miniapp-ui-fixer -> portal-miniapp-ui-review post-gate`.
- Appsmith remains special-cased: prefer `appsmith-audit -> appsmith-migration-spec` when those source-specific skills are available.

# Implementation Planning Reference

## Summary

A plan can be strong on features and still fail at execution if it is not validated against the real repo, runtime, auth model, and deployment shape. The recurring mistake to avoid is feature-centric planning without enough integration-centric verification.

Use this document when drafting or reviewing implementation plans for new apps, major features, or cross-cutting refactors.
Before locking assumptions, also review [docs/IMPLEMENTATION-KNOWLEDGE.md](IMPLEMENTATION-KNOWLEDGE.md) for reusable discoveries that may already define identifiers, exclusions, or cross-system rules.
If the work is an Appsmith migration or another legacy-app port, also follow [docs/APPSMITH-MIGRATION-PLAYBOOK.md](APPSMITH-MIGRATION-PLAYBOOK.md) so risky contracts are verified and pinned before implementation.

## Core Lessons

- Validate against the real host environment first.
  Before locking routes, app base paths, output directories, or dev ports, verify how the current server serves SPAs, how local dev is orchestrated, and how production artifacts are mounted.

- Reuse previously discovered implementation knowledge.
  Before inventing a new rule for identifiers, eligibility, exclusions, or legacy integration behavior, check the implementation knowledge handbook and carry forward verified discoveries.

- Treat "copy an existing app pattern" as a verification task, not a shortcut.
  Reuse only after checking the full chain: Vite base, proxies, auth bootstrap, root scripts, Docker copy paths, launcher overrides, and deep-link behavior.

- Plan auth together with UX flows.
  Any flow that downloads files, opens a new window, or bypasses the shared API client needs an explicit auth strategy. If an endpoint requires Bearer auth, plain link-based downloads are suspect by default.

- Resolve identifier strategy before defining CRUD.
  If a table uses meaningful string keys or legacy codes, creation APIs cannot be designed as if the database autogenerates identifiers.

- Separate management defaults from creation defaults.
  CRUD pages often need full visibility, while creation forms usually need filtered active-only data. Do not let one implicit default leak into both use cases.

- Do not leave infrastructure contracts half-decided.
  Env var names, migration strategy, DB bootstrap pattern, and deployment assumptions should be fixed early. "X or Y" placeholders usually become drift later.

- Do not leave observability implicit.
  For backend features and refactors, decide the log shape, request-correlation strategy, panic/failure logging, and client-facing 5xx sanitization policy as part of the plan. If internal failures should be diagnosable only from server logs, that must be explicit before implementation.

- Prefer dependency injection over package-global state for new backend modules.
  New DB-backed modules should receive dependencies explicitly so tests and handler composition stay predictable.

- Define nested-resource invariants explicitly.
  For routes like `/parents/{id}/children/{childId}`, state that parent-child ownership must be verified. Otherwise subtle data-integrity bugs slip through.

- Test the boundaries, not only the happy path.
  Plans should call out deep-link refresh, auth-protected exports, transaction rollback, filter/export parity, and coexistence with legacy consumers where relevant.

- Review plan fit across all layers, not just product behavior.
  A complete implementation plan covers product behavior, repo/runtime integration, data/auth contracts, and verification strategy.

## Planning Heuristics

- When a plan touches existing domains or legacy integrations, check `docs/IMPLEMENTATION-KNOWLEDGE.md` first and reuse established mappings or quirks instead of restating them from scratch.
- When a plan introduces a new app, confirm the final URL shape, static hosting path, local Vite port, and split-server launch path before writing view work.
- When a plan introduces a new backend integration, confirm the env contract, dependency wiring, migration story, and testability pattern before writing handler breakdowns.
- When a plan introduces backend failure paths or cross-cutting middleware changes, confirm the observability contract: request IDs, structured logging fields, panic handling, and whether internal errors are sanitized for clients.
- When a plan introduces export/download behavior, confirm how authenticated file transfer works before choosing frontend UX.
- When a plan touches legacy tables or shared consumers, define coexistence checks as part of the plan, not as post-implementation cleanup.

## Repo-Fit Checklist

Before approving an implementation plan, verify these explicitly:

### 1. Runtime Fit

- Does the planned route/base path match how the current server hosts SPAs?
- Will deep links and browser refresh work for nested client routes?
- Does the app need href overrides for local split-server development?

### 2. Dev Fit

- Is the Vite port unique?
- Are both `/api` and `/config` proxy requirements covered where auth bootstrap is used?
- Are root workspace scripts updated, not only local Make targets?

### 3. Auth Fit

- Which endpoints require Bearer auth?
- Do all planned UX flows use an auth-capable transport?
- Are 401/403 behaviors and role requirements part of the test plan?

### 4. Data-Contract Fit

- Are primary keys and identifier generation rules defined?
- Are active-only versus include-inactive defaults separated by use case?
- Are nested-resource ownership checks specified?

### 5. Deployment Fit

- Does the Docker/static output path match the runtime router?
- If a frontend app has TypeScript test files, does the production build use a `tsconfig.build.json` that excludes them?
- Are required env vars fully named and consistent?
- Is the migration story real and repo-compatible, not just a placeholder?

### 6. Verification Fit

- Are transaction and rollback cases covered?
- Is export output expected to match visible filtering semantics?
- Are deep-link refresh, auth-protected export, and legacy coexistence checked where applicable?
- Are internal failure responses sanitized while the underlying errors are still observable in server logs?
- Are request correlation, access logging, panic recovery, and dependency-failure logging covered where the plan changes backend runtime behavior?

## Review Rule

Every major implementation plan should be reviewed across four layers before execution:

1. Product behavior
2. Repo/runtime integration
3. Data and auth contract
4. Verification strategy

If any of those layers is still implicit, the plan is not ready.

Reusable discoveries that materially affect future work should be added to `docs/IMPLEMENTATION-KNOWLEDGE.md` as part of the implementation or planning follow-up.

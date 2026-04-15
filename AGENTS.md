## Vision
[Project vision](docs/project_vision.md) — Matrix-themed portal launching corporate mini-apps with Stripe-level design

## Architecture
- **Monorepo** with pnpm workspaces (frontend) + Go backend
- `apps/` — independent Vite+React frontend apps (portal, future mini-apps)
- `packages/` — shared frontend libraries (`@mrsmith/ui`, `@mrsmith/auth-client`, `@mrsmith/api-client`)
- `backend/` — Go monolith with modular `internal/` packages per app
- `deploy/` — Dockerfile (multi-stage), K8s manifests

## Important Reference
- `docs/mistra-dist.yaml` — authoritative Mistra NG Internal API spec; most mini-apps will integrate with these APIs, so use this file as the primary reference for backend contracts, client generation, and shared types.
- [`docs/IMPLEMENTATION-PLANNING.md`](docs/IMPLEMENTATION-PLANNING.md) — before approving an implementation plan, run the repo-fit checklist so hosting, auth, dev wiring, data contracts, and verification strategy are validated against the actual codebase.
- [`docs/IMPLEMENTATION-KNOWLEDGE.md`](docs/IMPLEMENTATION-KNOWLEDGE.md) — canonical handbook for reusable implementation discoveries (cross-system mappings, hidden rules, exclusions, quirks). Read it during planning and update it when new reusable knowledge emerges.

## Databases
- `docs/grappa/GRAPPA.md` — index for the Grappa MySQL schema dumps in `docs/grappa/`
- `docs/mistradb/MISTRA.md` — index for the Mistra PostgreSQL schema dumps in `docs/mistradb/`

## Dev
- `make dev` — runs air (Go hot reload) + Vite concurrently
- `make dev-docker` — same via docker-compose
- Backend proxy: Vite proxies `/api` → `localhost:8080`
- Auth: OAuth2/OIDC with remote Keycloak (no local instance)

## UI/UX
- [`docs/UI-UX.md`](docs/UI-UX.md) — Reference document for all UI/UX decisions implemented in the codebase. Use this as the foundation when building new mini-apps.
- For any new portal mini-app or mini-app UI review, use `.claude/skills/portal-miniapp-generator/` as the canonical workflow. For Appsmith migrations, run it after `.claude/skills/appsmith-migration-spec/` and before implementation.

## TODOs
- [`docs/TODO.md`](docs/TODO.md) — project-wide open items, deferred decisions, and out-of-scope work tracked for future implementation

## Memory Rule
ALWAYS update the relevant skill's `MEMORY.md` when making substantive changes to code, tests, fixtures, heuristics, or contributor workflow. The memory file is the persisted handoff state for future work.

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

## Dev
- `make dev` — runs air (Go hot reload) + Vite concurrently
- `make dev-docker` — same via docker-compose
- Backend proxy: Vite proxies `/api` → `localhost:8080`
- Auth: OAuth2/OIDC with remote Keycloak (no local instance)

## Memory Rule
ALWAYS update the relevant skill's `MEMORY.md` when making substantive changes to code, tests, fixtures, heuristics, or contributor workflow. The memory file is the persisted handoff state for future work.

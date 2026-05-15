# MrSmith

MrSmith is an internal web portal for launching corporate mini-apps. The portal
has a Matrix-inspired visual language, while the individual mini-apps use a
clean, polished dashboard style.

The repo is a monorepo:

- React/Vite frontend apps in `apps/`
- Shared frontend packages in `packages/`
- A Go backend monolith in `backend/`
- Docker and Kubernetes deployment assets in `deploy/`

## Prerequisites

- Node.js 20
- pnpm 10.33.0, preferably via Corepack
- Go 1.26.1 or newer
- `air` for local Go hot reload
- `golangci-lint` if you run backend linting
- Docker only if you use Docker-based dev/build/deploy flows

```sh
corepack enable
go install github.com/air-verse/air@latest
```

## Quick Start

Install dependencies:

```sh
make install
```

Create local backend configuration:

```sh
cp backend/.env.example backend/.env
```

For normal development, fill the Keycloak and DSN values needed by the app you
are working on. For UI-only local work, you can bypass auth:

```sh
# backend/.env
SKIP_KEYCLOAK=true

# apps/<app>/.env.local
VITE_DEV_AUTH_BYPASS=true
```

Run everything:

```sh
make dev
```

Main local URLs:

- Portal: `http://localhost:5173`
- Backend: `http://localhost:8080`
- Backend config endpoint: `http://localhost:8080/config`
- API prefix: `http://localhost:8080/api/...`

To run only one app, start the backend and the app in separate shells:

```sh
make dev-backend
make dev-quotes
```

## Common Commands

```sh
make help              # list Makefile commands
make install           # install pnpm workspace dependencies
make dev               # backend + all frontend apps
make dev-backend       # backend only
make dev-portal        # portal only
make build             # frontend workspace + Go server
make build-frontend    # all frontend packages/apps
make build-backend     # Go server only
make test              # existing backend and frontend tests
make lint              # backend golangci-lint + frontend type lint
make dev-docker        # Docker Compose dev environment
make docker-build      # production Docker image
```

Prefer targeted commands while iterating, for example
`pnpm --filter mrsmith-quotes lint` or `go test ./internal/quotes/...` from
`backend/`.

## App Ports

Each Vite app runs independently in local development. Vite proxies `/api` and
`/config` to the Go backend.

| App | Package | Port | Make target |
| --- | --- | ---: | --- |
| Portal | `mrsmith-portal` | 5173 | `make dev-portal` |
| Budget | `mrsmith-budget` | 5174 | `make dev-budget` |
| Compliance | `mrsmith-compliance` | 5175 | `make dev-compliance` |
| Kit Products | `mrsmith-kit-products` | 5176 | `make dev-kit-products` |
| Listini e Sconti | `mrsmith-listini-e-sconti` | 5177 | `make dev-listini` |
| Panoramica Cliente | `mrsmith-panoramica-cliente` | 5178 | `make dev-panoramica` |
| Quotes | `mrsmith-quotes` | 5179 | `make dev-quotes` |
| Reports | `mrsmith-reports` | 5180 | `make dev-reports` |
| RDF Backend | `mrsmith-rdf-backend` | 5181 | `make dev-rdf-backend` |
| Richieste Fattibilita | `mrsmith-richieste-fattibilita` | 5182 | `make dev-richieste-fattibilita` |
| Coperture | `mrsmith-coperture` | 5183 | `make dev-coperture` |
| Energia DC | `mrsmith-energia-dc` | 5184 | `make dev-energia-dc` |
| Simulatori Vendita | `mrsmith-simulatori-vendita` | 5185 | `make dev-simulatori-vendita` |
| AFC Tools | `@mrsmith/afc-tools` | 5186 | `make dev-afc-tools` |
| CP Backoffice | `mrsmith-cp-backoffice` | 5187 | `make dev-cp-backoffice` |
| Manutenzioni | `mrsmith-manutenzioni` | 5188 | `make dev-manutenzioni` |
| Fornitori | `mrsmith-fornitori` | 5189 | `make dev-fornitori` |
| RDA | `mrsmith-rda` | 5190 | `make dev-rda` |

## Repository Layout

```text
apps/
  portal/                  Matrix-themed launcher
  <mini-app>/              Independent Vite + React mini-apps
packages/
  ui/                      Shared UI components and themes
  auth-client/             Keycloak/OIDC React provider
  api-client/              Fetch client for authenticated /api calls
  tsconfig/                Shared TypeScript config package
backend/
  cmd/server/              Go server entrypoint
  internal/<module>/       App-specific backend modules
  internal/platform/       Shared backend platform code
  pkg/middleware/          Shared HTTP middleware
deploy/
  Dockerfile               Production multi-stage image
  Dockerfile.dev           Backend hot-reload image
  k8s/                     Kubernetes manifests
docs/
  *.md, *.yaml, *.json     Product, API, schema, planning, and UX references
```

`apps/customer-portal/` is a migration workspace for `apps/cp-backoffice/`, not
an active frontend app.

## Architecture Notes

The Go server exposes unauthenticated health/config endpoints and mounts all
application APIs under `/api`. The `/api` boundary applies recovery, request
IDs, access logging, CORS, and Keycloak bearer-token validation.

Frontend apps bootstrap runtime auth config from `GET /config`, then call
same-origin `/api` through `@mrsmith/api-client`. Browser code should not call
Mistra, Grappa, Arak, HubSpot, Carbone, database gateways, or other upstream
systems directly. Put those integrations behind the Go backend.

In production, `deploy/Dockerfile` builds every frontend app, copies the portal
to `/static`, copies mini-apps to `/static/apps/<slug>`, and runs the Go server
with `STATIC_DIR=/static`. Deep links are handled by
`backend/internal/platform/staticspa`.

## Configuration

Local backend config lives in `backend/.env`; use `backend/.env.example` as the
starting point. Important groups:

- Auth: `KEYCLOAK_ISSUER_URL`, `KEYCLOAK_FRONTEND_URL`,
  `KEYCLOAK_FRONTEND_REALM`, `KEYCLOAK_FRONTEND_CLIENT_ID`,
  `KEYCLOAK_ADMIN_CLIENT_ID`, `KEYCLOAK_ADMIN_CLIENT_SECRET`,
  optional `KEYCLOAK_ADMIN_BASE_URL`, `KEYCLOAK_ADMIN_REALM`,
  `KEYCLOAK_ADMIN_TOKEN_URL`, and `SKIP_KEYCLOAK`
- Server/runtime: `PORT`, `CORS_ORIGINS`, `STATIC_DIR`, `INCLUDE_DEV_APPS`
- App databases: `MISTRA_DSN`, `ARAK_DSN`, `ANISETTA_DSN`, `GRAPPA_DSN`,
  `DBCOPERTURE_DSN`, `MANUTENZIONI_DSN`, `ALYANTE_DSN`, `VODKA_DSN`,
  `WHMCS_DSN`
- Upstream services: `ARAK_BASE_URL`, `ARAK_SERVICE_*`, `HUBSPOT_API_KEY`,
  `CARBONE_API_KEY`, `OPENROUTER_API_KEY`, `RDF_TEAMS_*`
- Split-server app URL overrides: `<APP>_APP_URL`, for example
  `RDA_APP_URL` or `MANUTENZIONI_APP_URL`

Do not commit `.env`, `.env.local`, deployment secrets, or copied production
configuration.

`KEYCLOAK_ADMIN_*` is backend-only and is not returned by `GET /config`.
Configure it only when backend code needs to resolve users by realm role. The
Keycloak service account must have read-only Admin API access to users, groups,
and realm roles; direct role-user lookup alone does not include users who
inherit a role through group membership.

## Key References

- `docs/project_vision.md`: product and design direction
- `docs/UI-UX.md`: mandatory UI/design-system reference
- `docs/API-CONVENTIONS.md`: frontend/backend API conventions
- `docs/IMPLEMENTATION-PLANNING.md`: repo-fit checklist for larger work
- `docs/IMPLEMENTATION-KNOWLEDGE.md`: reusable domain discoveries and quirks
- `docs/mistra-dist.yaml`: authoritative Mistra NG Internal API spec
- `docs/grappa/GRAPPA.md`: Grappa MySQL schema dump index
- `docs/mistradb/MISTRA.md`: Mistra PostgreSQL schema dump index
- `docs/APPSMITH-MIGRATION-PLAYBOOK.md`: legacy Appsmith migration workflow
- `docs/PROD-DEPLOY.md`: production deploy process
- `docs/TODO.md`: project-wide deferred work

Read the relevant docs before changing a domain that touches legacy systems,
cross-database mappings, auth, deployment, or mini-app routing.

## Adding or Changing a Mini-App

When adding a new mini-app or making a major app change, verify the whole chain:

1. Use an `apps/<slug>` workspace package with a unique Vite port.
2. Set Vite `base` to `/apps/<slug>/` for builds.
3. Proxy both `/api` and `/config` during local development.
4. Use `@mrsmith/auth-client`, `@mrsmith/api-client`, and shared UI where
   possible.
5. Register backend routes in `backend/internal/<module>` and mount them from
   `backend/cmd/server/main.go`.
6. Prefer app-scoped API namespaces such as `/api/<app-prefix>/v1/...`.
7. Add app catalog metadata, roles, href overrides, and availability filters in
   `backend/internal/platform/applaunch` and `backend/cmd/server/main.go`.
8. Update root scripts, Makefile targets, CORS origins, env examples, and
   `deploy/Dockerfile` static copy paths.
9. Update `docs/IMPLEMENTATION-KNOWLEDGE.md` when you discover reusable
   mappings, exclusions, legacy quirks, or cross-system rules.

For UI work, treat `docs/UI-UX.md` as the source of truth. The portal is Matrix
themed; mini-apps should use the clean theme and the established shared
component patterns.

## API and Data Contracts

Use `docs/mistra-dist.yaml` as the primary reference for Mistra/Arak API
contracts. Use the schema dump indexes under `docs/grappa/` and
`docs/mistradb/` for database-backed work.

The backend wires each external database as a separate `*sql.DB`. Do not plan a
single SQL join across different DSNs; fetch from each system and merge in Go.

Keep authenticated downloads and exports behind backend endpoints and consume
them with blob-capable API client methods instead of plain links.

## Testing and Review

Run the narrowest useful verification before handing off a change:

- Backend changes: targeted `go test` package(s), then broader tests if the
  touched code is shared.
- Frontend changes: package `lint`/build, plus a browser check for UI behavior.
- Shared packages or cross-cutting changes: run the affected workspace builds.

Coordinate before adding new tests. Add them when they protect a reproduced bug,
a business-critical rule, or a non-trivial query/data transformation. Avoid
speculative tests for routine UI copy, simple wiring, or low-risk refactors.

Before browser automation or UI checks, reuse an already running `make dev` or
Vite server when one exists.

## Deployment

Production uses the multi-stage Dockerfile in `deploy/Dockerfile`. The standard
remote deploy flow is documented in `docs/PROD-DEPLOY.md` and driven by:

```sh
make deploy-prod
```

Manual image commands are available when Docker is installed locally:

```sh
make docker-build
make docker-build-amd64
make package-prod-amd64
```

Kubernetes manifests live under `deploy/k8s/`. Keep deployment env names aligned
with `backend/internal/platform/config` and `backend/.env.example`.

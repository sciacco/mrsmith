# RDF-IMPL — Richieste Fattibilita

## Summary
- Mini-app path: `/apps/richieste-fattibilita/`
- API prefix: `/api/rdf/v1/*`
- Roles: `app_rdf_access` for read/create, `app_rdf_manager` for carrier management, `app_devadmin` as shared superuser override
- Runtime dependencies: `ANISETTA_DSN`, `MISTRA_DSN`, optional `OPENROUTER_API_KEY`, optional `RDF_TEAMS_WEBHOOK_URL`, `RDF_TEAMS_NOTIFICATIONS_ENABLED`
- Initial launcher status: `test`

## Comparable Apps Audit
- Inspected `apps/compliance/src/views/blocks/BlocksPage.tsx`
- Inspected `apps/quotes/src/pages/QuoteListPage.tsx`
- Inspected `apps/rdf-backend/src/App.tsx`
- Reused patterns:
  - compact page header + subtitle + right-aligned actions
  - toolbar with URL-driven filters
  - clean card/list workspace inside the existing mini-app family
  - modal-backed mutations for bounded create flows
- Explicitly avoided:
  - hero/KPI dashboards
  - launcher-style visuals
  - machine-facing copy

## Archetype Choice
- Primary archetype: `master_detail_crud`
- Covered screens:
  - `Consultazione RDF`
  - `Nuova RDF`
  - `Gestione RDF Carrier`
  - `Dettaglio RDF Carrier`
  - `Visualizza RDF`
- Explicit exception:
  - `Visualizza RDF` remains a tabbed read-only view inside the same mini-app rather than a second app shell

## Implementation Changes
- Backend:
  - added `backend/internal/rdf` for request, summary, detail, fattibilita mutation, AI, PDF, and Teams notification flows
  - added shared `backend/internal/platform/openrouter`
  - mounted routes in `backend/cmd/server/main.go`
  - added launcher/catalog entry and split-server href override on port `5182`
  - hid the launcher card when either `ANISETTA_DSN` or `MISTRA_DSN` is missing
- Frontend:
  - scaffolded `apps/richieste-fattibilita` as a standard Vite mini-app
  - added routes for `/richieste`, `/richieste/new`, `/richieste/gestione`, `/richieste/:id`, `/richieste/:id/view`
  - implemented typed local API client with `PATCH` and authenticated blob download
  - added list, create, detail, and tabbed view pages
- Runtime and deploy:
  - added `RICHIESTE_FATTIBILITA_APP_URL`
  - added `OPENROUTER_API_KEY`
  - added `RDF_TEAMS_WEBHOOK_URL`
  - added `RDF_TEAMS_NOTIFICATIONS_ENABLED`
  - added Docker copy for `/static/apps/richieste-fattibilita`
  - added root `package.json` and `Makefile` dev targets

## User Copy Rules
- Italian, business-user-facing copy only
- Preserved labels:
  - `Nuova RDF`
  - `Consultazione RDF`
  - `Gestione RDF Carrier`
  - `Visualizza RDF`
  - `Analisi`
  - `Azioni`
- Forbidden in user copy:
  - `LLM`
  - `JSON`
  - `server-side`
  - `record`
  - `datasource`

## Repo-Fit
- Split-server local dev:
  - Vite port `5182`
  - `/api` and `/config` proxied to backend
- SPA hosting:
  - Vite build base `/apps/richieste-fattibilita/`
  - Docker target `/static/apps/richieste-fattibilita`
- Auth:
  - backend ACLs on `/rdf/v1/*`
  - frontend uses authenticated blob download for PDF
- Data contracts:
  - `fornitori_preferiti` exposed as `number[]`, persisted as PG literal text
  - `copertura` exposed as boolean
  - `summary` remains the only list endpoint used by UI

## Exceptions
- PDF rendering is implemented as a pure-Go internal PDF builder instead of adding a browser/runtime dependency.
- Summary enrichment is server-side, but because `rdf_*` and `loader.hubs_*` sit behind separate DSNs, the merge happens in Go after batched reads, not in one SQL join.

## Verification
- `pnpm --filter mrsmith-richieste-fattibilita build`
- `go test ./internal/rdf ./cmd/server`
- Manual runtime wiring checks completed in code for:
  - launcher href override
  - Docker copy path
  - catalog visibility gating
  - API registration

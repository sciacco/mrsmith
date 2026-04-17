# Zammu split — Shared Boundary

> Cross-cutting concerns for the three mini-apps produced by splitting `zammu-main`. Each per-app spec is self-contained; this doc captures only portal-level concerns and the items that are deliberately **not** shared.

## Split summary

| Target app | Portal category | Source page (from Zammu) | Primary datasource |
|------------|-----------------|--------------------------|--------------------|
| Coperture | Smart Apps | `Coperture` | `dbcoperture` (PostgreSQL) |
| Energia in DC | Smart Apps | `Energia variabile` | `grappa` (MySQL) |
| Simulatori di Vendita | MKT&Sales | `IaaS calcolatrice` | Carbone.io REST + new pricing DB table |

## Dropped / deferred (not in any of the three specs)

- **Dropped:** Zammu page `Home` (trivial greeting); JS libs `ExcelJS` and `fast-xml-parser` (not referenced); all debug methods (`test`, `myFun2`, `pippo`); GraphQL stub `getTransazioni`; unreachable weekly-kW branch and `get_kw_weeks` query.
- **Deferred to a future 4th target:** Zammu page `Transazioni whmcs` + its `whmcs_prom` MySQL datasource (hidden page, WHMCS paid-transaction viewer). Not in scope for this split.

## Cross-app entities

**None.** The three targets are domain-disjoint.
- Coperture has no customer concept.
- Energia in DC is the only target that reads a customer table (`cli_fatturazione` in `grappa`). The cross-database customer-ID mapping rule (`cli_fatturazione.codice_aggancio_gest` = Alyante ERP ID = Mistra `customers.customer.id`) is documented in `docs/IMPLEMENTATION-KNOWLEDGE.md` and is a portal-wide constant; it is not duplicated in any per-app spec beyond a reference in the Energia spec's Customer entity.
- Simulatori di Vendita does not query any customer master data.

## Cross-app user journeys

**None identified in the audit.** Each mini-app is a standalone portal tile with no navigation into the others.

## Shared platform concerns

### Authentication and authorization
All three apps sit behind the portal's Keycloak OIDC flow. Proposed roles (compact snake-case, following the catalog convention in `backend/internal/platform/applaunch/catalog.go`):

| App | Access role | Additional role |
|-----|-------------|-----------------|
| Coperture | `app_coperture_access` | — |
| Energia in DC | `app_energiadc_access` | — |
| Simulatori di Vendita | `app_simulatorivendita_access` | `app_simulatorivendita_admin` (pricing admin) |

The `app_{appname}_access` naming is mandated by `CLAUDE.md`; specific compact forms above are proposed and can be adjusted at implementation time.

### Portal catalog + routing
Each target app needs an entry in `backend/internal/platform/applaunch/catalog.go` and the new-app wiring listed in `CLAUDE.md` (root `package.json`, `Makefile`, `backend/cmd/server/main.go`, `backend/internal/platform/config/config.go`). Proposed app IDs and hrefs:

| App | ID | Href | CategoryID | CategoryTitle |
|-----|----|------|------------|---------------|
| Coperture | `coperture` | `/apps/coperture/` | `smart-apps` | `SMART APPS` |
| Energia in DC | `energia-dc` | `/apps/energia-dc/` | `smart-apps` | `SMART APPS` |
| Simulatori di Vendita | `simulatori-vendita` | `/apps/simulatori-vendita/` | `mkt-sales` | `MKT&Sales` |

(Note: the catalog already contains a commented-out `coperture` entry with an older `/apps/smart-apps/coperture` href. The newer convention — `/apps/{app-id}/` — is preferred.)

### Shared frontend libraries
- `@mrsmith/auth-client` — OIDC login, token refresh, unauthorized handling.
- `@mrsmith/api-client` — shared HTTP client and types.
- `@mrsmith/ui` — Matrix/Stripe-level design system per `docs/UI-UX.md`.

### Candidate additions to `@mrsmith/ui`
- **Cascading-select component.** Both Coperture (Provincia → Comune → Indirizzo → Numero civico) and Energia in DC (Cliente → Site → Room → Rack) implement the same pattern. Extracting it into `@mrsmith/ui` avoids duplication across two apps in this split; implementers should confirm an existing shared component does not already cover the use case.

### Shared operational conventions
- Type-checking: `pnpm --filter <app> exec tsc --noEmit` per app (never bare `npx tsc`).
- Backend is a single Go monolith; each app lives in its own `backend/internal/<app>` module with routes registered from `main.go`.
- Dev wiring: `make dev-{appname}` target, root `package.json` concurrently entry, `dev:{appname}` script — all per `CLAUDE.md` new-app checklist.

## Dropped current-state bugs / quirks (non-exhaustive)

These are documented here for the record; each is addressed (or explicitly retained) in the relevant per-app spec's "Rules being revised rather than ported" or "Open Questions" section:

- **Coperture:** hardcoded operator logo map → migrated to backend table.
- **Energia in DC:** SQL-injection-risk unprepared queries → parameterized; `LIKE` on numeric FKs → `=`; hardcoded `id_anagrafica <> 3` → config flag; 225V formula → kept as approximation (deliberate); weekly kW branch → dropped; fire-and-forget loading → replaced with per-hook loading state; master-detail name-keyed join → ID-keyed.
- **Simulatori di Vendita:** hardcoded pricing → DB + admin UI; Carbone API key in frontend → backend proxy; TEXT widgets used as numeric → NUMBER; incomplete `decodifica` display-name map → complete in rewrite.

## What the shared doc is **not**

- Not a replacement for any per-app spec. Each per-app spec is self-contained.
- Not a source of truth for entities — no entity is shared across the three apps.
- Not a portal-core design doc. Portal shell, navigation, theming, and cross-app identity are owned by the portal itself and referenced here only as context.

## Open questions at the shared level

- **S1.** Final app IDs, hrefs, and role names (proposals above). Confirm at implementation time.
- **S2.** Does an existing `@mrsmith/ui` cascading-select component already cover the pattern, or should one be introduced during this work?
- **S3.** Portal icon choices for each new tile.

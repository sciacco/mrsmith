# Simulatori di Vendita Implementation Plan

> Source spec: `apps/zammu/simulatori-di-vendita-spec.md` (ready for hand-off as of 2026-04-18).
> Shared boundary context: `apps/zammu/zammu-shared-boundary.md`.
> Scope override for this phase: v1 is intentionally narrowed to a 1:1 port of the Appsmith `IaaS calcolatrice` page. DB-backed pricing and the pricing-admin route are deferred to `docs/TODO.md`.
> This plan is the pre-gate artifact for `portal-miniapp-ui-review`.

## Comparable Apps Audit

### Reference 1 - `apps/richieste-fattibilita/src/pages/NewRequestPage.tsx`
- **Why comparable:** best current repo precedent for a two-surface task workspace where a contextual panel stays visible while the primary form remains focused and uncluttered.
- **Key files inspected:**
  - `apps/richieste-fattibilita/src/pages/NewRequestPage.tsx`
  - `apps/richieste-fattibilita/src/pages/shared.module.css`
- **Reused patterns (adopt for Simulatori di Vendita):**
  - asymmetric two-column workspace that collapses cleanly below the tablet breakpoint
  - clear action row near the main form instead of detached page-level controls
  - clean card surfaces without dashboard filler
- **Rejected patterns (do NOT adopt):**
  - deal-search specific left rail behavior
  - keyboard-shortcut hints
  - reset confirmation modal for an ordinary calculator reset

### Reference 2 - `apps/energia-dc/src/pages/ConsumiKwPage.tsx`
- **Why comparable:** compact single-purpose mini-app page with one control surface, one result surface, and explicit generic-error versus `503` handling.
- **Key files inspected:**
  - `apps/energia-dc/src/pages/ConsumiKwPage.tsx`
- **Reused patterns (adopt):**
  - concise page header with no hero shell
  - one controls card plus one output card
  - explicit service-unavailable handling for backend dependency failures
- **Rejected patterns (do NOT adopt):**
  - chart-based output
  - extra analytics framing or decorative stats

### Consolidated Reused Patterns
- clean single-page workspace inside the standard mini-app shell
- two-column calculator layout on desktop, stacked flow on narrow screens
- explicit action buttons rather than hidden auto-submit behavior
- loading/error/`503` states only where there is real backend interaction

### Consolidated Rejected Patterns
- no app-specific top nav for v1
- no pricing-admin tab or route in this phase
- no dashboard KPI row
- no customer selector, search flow, or unrelated side panels

## Archetype Choice

- **Selected archetype:** `data_workspace`
- **Why it fits:** the v1 app is one calculator page coordinating two directly related surfaces:
  - left side: active rate table, inclusions, daily breakdown, monthly total
  - right side: tier selector and quantity form
  The user task is still a workspace task, not a report, wizard, or settings-only form.
- **Why not the others:**
  - `settings_form`: the page is not primarily configuration; it computes a quote
  - `report_explorer`: users are not exploring existing records
  - `wizard_flow`: there is no step sequence
  - `master_detail_crud`: the deferred pricing-admin work is out of scope for this phase
- **Planned navigation shape:**
  - single route at app index `/apps/simulatori-vendita/`
  - no app-specific `TabNav` in v1
- **Required states:**
  - default calculator state with `Diretta` selected and source defaults loaded
  - calculated state after `Calcola`
  - calculated state with `Indiretta` selected
  - PDF-in-progress state
  - generic PDF error state
  - `503` PDF-not-configured/service-unavailable state
  - narrow/mobile stacked state with actions still usable

## User Copy Rules

- **Allowed copy style:** business-user-only Italian with source-first labels.
- **Canonical labels to use:**
  - page title: `Calcolatore IaaS`
  - summary title: `Addebiti giornalieri risorse`
  - tier selector: `Diretta`, `Indiretta`
  - actions: `Calcola`, `Azzera`, `Genera PDF`
  - totals: `Totale giornaliero`, `Totale mensile`
  - breakdown labels: `Computing`, `Storage`, `Sicurezza`, `Add On`
- **Source-fidelity rules:**
  - preserve the resolved 10 resource display names from `apps/zammu/simulatori-di-vendita-spec.md`
  - keep the static inclusions block from the source page (Public IP, VPC, Firewall, rete 1Gbps) as compact informational copy
  - fix only the known source typo on `Primary Storage`
- **Forbidden copy risks:**
  - no implementation language such as `endpoint`, `datasource`, `record`, `widget`, `template hash`, or `decodifica`
  - no persistence/admin copy in this phase
  - no launcher-style marketing subtitle
- **Metrics allowed:**
  - per-category daily breakdown
  - `Totale giornaliero`
  - `Totale mensile`
  These are the actual calculator outputs, not decorative KPI cards.

## Source Fidelity Gate

> This phase intentionally prioritizes source parity over the broader future-state spec. The deferred work is tracked in `docs/TODO.md`.

- **Carry forward from the audit/source page:**
  - two hardcoded pricing tiers seeded from `apps/zammu/ZAMMU-AUDIT.md` section 2.5
  - source defaults `Diretta`, `1/0/0/100/100/0/0/0/0/0`
  - explicit `Calcola` action for recomputation in v1
  - `Azzera` reset flow with no confirmation modal
  - Carbone payload keys `qta`, `prezzi`, `totale_giornaliero`
  - monthly multiplier fixed at `30`
- **Source fixes that still apply in v1:**
  - convert `i_fw_standard` and `i_os_windows` from TEXT-style source behavior to real numeric inputs
  - complete the display-name map for all 10 resources
  - replace the source direct Carbone navigation with an authenticated backend proxy call
- **Deferred from the broader spec:**
  - DB-backed pricing source
  - pricing-admin route and write APIs
  - live recompute on every input change

## Repo-Fit

> Verified against `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, `docs/UI-UX.md`, and current runtime wiring.

- **Frontend app location:** `apps/simulatori-vendita/`
- **Package name:** `mrsmith-simulatori-vendita`
- **Route / base path:** build base `/apps/simulatori-vendita/`, dev base `/`, with a single client route at the app index
- **Portal catalog entry:** add `SimulatoriVenditaAppID = "simulatori-vendita"` and `SimulatoriVenditaAppHref = "/apps/simulatori-vendita/"` under category `mkt-sales` / title `MKT&Sales`
- **Portal icon:** use a supported icon key such as `briefcase`
- **API prefix:** `/api/simulatori-vendita/v1/...`
  - concrete endpoint for this phase:
    - `POST /api/simulatori-vendita/v1/iaas/quote`
- **Access role:** `app_simulatorivendita_access`
  - launcher visibility and quote endpoint both require this role
  - no admin-only route or endpoint exists in this phase
- **Pricing source for v1:** app-local constant module seeded from the audit
  - no pricing table
  - no DB migration
  - no `GET/PUT /pricing` endpoints
- **Carbone config:**
  - reuse existing `CARBONE_API_KEY`
  - add `SIMULATORI_VENDITA_IAAS_TEMPLATE_ID`
  - keep the default template ID equal to the audited source value unless the spec is reopened
- **Misconfiguration behavior:**
  - do not hide the launcher tile because pricing is not DB-backed in this phase
  - if the Carbone template ID or API key is unavailable, `POST /quote` returns `503 simulatori_vendita_pdf_not_configured`
  - the frontend handles that as an action error; it does not need a separate pricing-capabilities API
- **Split-server dev URL override:** add `SIMULATORI_VENDITA_APP_URL`; default to `http://localhost:5185` when `StaticDir == ""`
- **Dev port / proxy:** Vite port `5185`, with `/api` and `/config` proxied to `VITE_DEV_BACKEND_URL || http://localhost:8080`
- **Backend default CORS:** extend the default `CORS_ORIGINS` list with `http://localhost:5185`
- **Static hosting / deployment:** add `COPY --from=frontend /app/apps/simulatori-vendita/dist /static/apps/simulatori-vendita` to `deploy/Dockerfile`, plus a `staticspa` deep-link regression
- **Dev wiring updates required:**
  - root `package.json`: add the app to the top-level `dev` command and add `dev:simulatori-vendita`
  - `Makefile`: add `dev-simulatori-vendita`
  - `docker-compose.dev.yaml`: add a frontend service on port `5185` with `VITE_DEV_BACKEND_URL=http://backend:8080`
  - `backend/internal/platform/config/config.go`: add `SimulatoriVenditaAppURL` and `SimulatoriVenditaIaaSTemplateID`
  - `backend/cmd/server/main.go`: add the href override and register the new module
  - `backend/internal/platform/applaunch/catalog.go`: add app ID/href/access-role definitions
  - `backend/internal/platform/applaunch/catalog_test.go`: update launcher visibility and href override coverage
  - `backend/internal/portal/handler_test.go`: update portal launcher expectations
  - `backend/internal/platform/staticspa/handler_test.go`: add a deep-link regression for `/apps/simulatori-vendita/...`
  - `backend/.env.example` and `.env.preprod.example`: document `SIMULATORI_VENDITA_APP_URL` and `SIMULATORI_VENDITA_IAAS_TEMPLATE_ID`
- **Frontend bootstrap:** mirror the existing auth/bootstrap/query-client pattern from `apps/coperture` or `apps/energia-dc`, but keep a single-page shell without app-specific top navigation

## Data Contract

### Local Pricing Catalog

- **Pricing storage for v1:** one frontend module containing the two audited tiers:
  - `diretta`
  - `indiretta`
- **Resource contract:** keep the ordered 10-key set from the audit/spec:
  - `vcpu`
  - `ram_vmware`
  - `ram_os`
  - `storage_pri`
  - `storage_sec`
  - `fw_std`
  - `fw_adv`
  - `priv_net`
  - `os_windows`
  - `ms_sql_std`
- **Resource catalog module:** define one ordered frontend catalog for display label, section, defaults, and UI min/max/step rules
- **Drift rule:** do not rely on object iteration order in the UI; keep one explicit ordered resource list

### Calculation Rules

- **Frontend calculation helper must implement:**
  - per-line total = `quantity * tier.rate`
  - `computing = vcpu + ram_vmware + ram_os`
  - `storage = storage_pri + storage_sec`
  - `sicurezza = fw_std + fw_adv + priv_net`
  - `addon = os_windows + ms_sql_std`
  - `totale_giornaliero = sum(all line totals)`
  - `totale_mensile = totale_giornaliero * 30`
- **Rounding rule:** keep calculations unrounded until the final display boundary; `toFixed(2)` is presentation only
- **Validation rule:** UI enforces the source min/max constraints; quote endpoint validates numeric payload shape only

### Quote API

- **`POST /api/simulatori-vendita/v1/iaas/quote`**
  - requires `app_simulatorivendita_access`
  - request body mirrors the source Carbone payload shape:
    - `qta`
    - `prezzi`
    - `totale_giornaliero`
  - backend wraps that payload as `{ convertTo: "pdf", data: { ... } }`, calls Carbone, and streams `application/pdf`
- **No additional endpoints in this phase:**
  - no pricing read endpoint
  - no pricing write endpoint
  - no quote history endpoint
  - no totals endpoint

### PDF Transport

- **Frontend transport:** authenticated `fetch` with bearer token and `Blob` download
- **Explicitly forbidden:** `window.open('/api/...')` or plain-link download because the route is Bearer-protected

## Implementation Breakdown

### Frontend Structure

- **App shell and routing**
  - `src/main.tsx`: same auth/bootstrap/query-client shape as the existing mini-app family
  - `src/App.tsx`: `AppShell`, auth/reauth/access-required surfaces, single page route
  - `src/routes.tsx`: index route plus fallback redirect if the app follows the standard router split
- **Shared client modules**
  - `src/api/client.ts`: app-local wrapper around `@mrsmith/api-client`
  - `src/api/types.ts`: quantity, pricing, breakdown, and quote payload types
  - `src/api/queries.ts`: `useGenerateQuote`
  - `src/features/iaas/resourceCatalog.ts`
  - `src/features/iaas/pricing.ts`
  - `src/features/iaas/calculateQuote.ts`
  - `src/features/iaas/buildQuotePayload.ts`
  - `src/features/iaas/format.ts`

### Route 1 - `CalcolatoreIaaSPage`

- **Route:** app index
- **Desktop layout:** two-column workspace
  - left column: pricing summary and computed results
  - right column: tier selector and grouped quantity inputs
- **Summary column contents:**
  - `Addebiti giornalieri risorse`
  - ordered per-resource rate table for the active tier
  - static inclusions block mirroring the source page
  - category subtotals
  - prominent daily and monthly totals
- **Input column contents:**
  - tier selector for `Diretta` / `Indiretta`
  - grouped numeric inputs for the 10 resource quantities
  - source defaults prefilled from the audit
  - action row with `Calcola`, `Azzera`, `Genera PDF`
- **Interaction rules:**
  - page load seeds the default tier and quantity values
  - changing tier updates the displayed price table immediately
  - `Calcola` computes and displays the daily breakdown and monthly total from the current form values
  - `Azzera` restores the source defaults and returns the page to the default calculator state
  - `Genera PDF` runs the same local calculation step first, then posts the authenticated quote payload to the backend proxy
  - there is no separate admin route, no customer scope, and no persistence copy
- **Responsive rule:**
  - below the desktop breakpoint the layout stacks into one column
  - actions stay close to the form and totals stay visually close to the summary

### Backend Structure

- **Module:** `backend/internal/simulatorivendita/`
- **Suggested file split:**
  - `handler.go` - route registration and quote endpoint
  - `carbone.go` - app-local Carbone client wrapper
- **Dependency injection:** handler receives template ID, logger helpers, and Carbone client dependencies explicitly
- **Route guards:** quote render requires `app_simulatorivendita_access`
- **Failure contract:**
  - missing template config or disabled renderer: return `503 simulatori_vendita_pdf_not_configured`
  - Carbone render failure: return sanitized `5xx` and log the upstream error with request context

## Exceptions

- **Keep the explicit `Calcola` button in v1.**
  - User benefit: this phase is a source-faithful port of the Appsmith calculator, not the broader future-state UX.
- **Keep pricing in app-local constants in v1.**
  - User benefit: avoids blocking the migration on a new pricing store and admin workflow.
- **No pricing-admin route or top navigation in this phase.**
  - User benefit: the app stays a faithful one-page calculator.
- **Use authenticated blob download instead of opening the PDF in a new window.**
  - User benefit: preserves the export action while fitting the repo auth model.

## Verification

### UI Review Checks

- default desktop calculator state with `Diretta` selected and source defaults loaded
- calculated state after `Calcola`
- calculated state with `Indiretta` selected
- PDF-in-progress state
- generic quote/PDF error state
- `503 simulatori_vendita_pdf_not_configured` state surfaced as a clear action error
- narrow/mobile stacked layout with usable form and actions

### Runtime / Auth Checks

- deep-link refresh works at `/apps/simulatori-vendita/`
- the app does not hit the quote endpoint before a bearer token exists
- users without `app_simulatorivendita_access` do not see the launcher tile and receive `403` if they hit the endpoint directly
- PDF generation uses authenticated blob download, not a plain URL open
- no pricing DB env var, migration, or admin route is introduced in this phase
- missing Carbone config returns `503 simulatori_vendita_pdf_not_configured`

### Tests

- **Frontend:**
  - calculation helper regression tests for 10-line summation, category grouping, and monthly multiplier `30`
  - quote-payload builder test preserving `qta`, `prezzi`, and `totale_giornaliero`
  - quote-download error handling test
- **Backend (Go):**
  - handler test for `503 simulatori_vendita_pdf_not_configured`
  - auth/ACL coverage for missing role
  - proxy-shape test ensuring the backend sends `convertTo: "pdf"` and forwards the source payload keys to Carbone
- **Platform wiring tests:**
  - launcher catalog visibility and href override coverage
  - `staticspa` deep-link regression for `/apps/simulatori-vendita/...`
- **Manual verification:**
  - calculate with default `Diretta` values and confirm the totals match the source
  - switch to `Indiretta`, press `Calcola`, and confirm the daily/monthly totals change as expected
  - press `Azzera` and confirm the source defaults are restored
  - generate a PDF and confirm the document reflects the on-screen calculation

### Handoff

- This document is the pre-gate artifact for `portal-miniapp-ui-review`.
- Post-gate review is required again after implementation.

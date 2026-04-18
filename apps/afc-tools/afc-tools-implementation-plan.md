# AFC Tools Implementation Plan

Source: approved migration spec `apps/afc-tools/afc-tools-migspec.md` (1:1 port from Appsmith).

## Comparable Apps Audit

- **Reference 1 — `apps/reports`** *(primary analog — 8-tab data workspace with read-only tables + one carbone XLSX export)*
  - `apps/reports/src/routes.tsx` — flat `RouteObject[]`, no lazy, index redirects to first tab.
  - `apps/reports/src/App.tsx` — `AppShell` + `TabNavGroup` with section metadata.
  - `apps/reports/src/pages/AccessiAttiviPage.tsx` — filter bar + table + XLSX export. XLSX flow: frontend calls `POST /api/reports/v1/…/export`, backend returns `application/octet-stream` blob, frontend does `URL.createObjectURL` → anchor click. Uses `useApiClient()`, `useToast()`, React Query for dropdown data, local `useState` for filters and `exporting` flag. Plain HTML `<table>`.
- **Reference 2 — `apps/energia-dc`** *(5-tab read-only data explorer with filters)*
  - `apps/energia-dc/src/routes.tsx` — `React.lazy` per route + redirect index → first tab.
  - `apps/energia-dc/src/App.tsx` — `AppShell` + `TabNav` (flat, 5 items).
  - `apps/energia-dc/src/pages/AddebitiPage.tsx` — filter (`SingleSelect customerId`) + React Query `useBillingCharges(customerId)` + CSV export built client-side from the query result.
- **Reference 3 — `apps/quotes`** *(list → detail pattern)*
  - `apps/quotes/src/routes.tsx` — 3 routes: `/quotes`, `/quotes/new`, `/quotes/:id`.
  - `apps/quotes/src/pages/QuoteListPage.tsx` — navigates to detail via `useNavigate()`; URL search params for pagination/filter.
  - `apps/quotes/src/pages/QuoteDetailPage.tsx` — `useParams<{id}>()` → `useQuote(Number(id))`; back button via `navigate('/quotes')`.

**Reused patterns**:
- `AppShell` + `TabNavGroup` (from `reports`) — AFC Tools has 8 views that split into 3 logical groups (§B.1 of the spec), a clean match for `TabNavGroup`.
- React Query + `useApiClient()` per page; stale-while-revalidate cache.
- XLSX download via blob URL + anchor click — same as `reports` (except that, for carbone, backend returns `{renderUrl}` instead of a blob; the frontend opens the URL in a new tab per decision A.5.4 = 4a).
- PDF download via native `fetch → Blob → URL.createObjectURL → <a download>` — analog to the reports export, but for streamed PDF bodies.
- List-to-detail routing with `useParams` + `useNavigate` — from `quotes`.
- Per-page `shared.module.css` local module (reports/energia-dc style).

**Rejected patterns** (present in one or more references but not a fit here):
- `quotes`-style multi-tab detail page with dirty tracking and save/publish sticky bar — the AFC Tools detail view is pure read-only, one page, no edits, no modal.
- `energia-dc`-style lazy routing with `React.Suspense` — 8 routes is within the range where eager imports (reports style) are simpler; no need for code-splitting in v1.
- `reports`-style inline summary metric row above the table — AFC Tools views have no summary metrics in the Appsmith source; adding them would violate the Metrics gate.
- `quotes` URL search params for filter state — AFC Tools filters are ephemeral per view (date range on Transazioni, year on Energia Colo), not worth persisting to the URL in v1. Matches Appsmith behavior.

## Archetype Choice

- **Selected archetype**: `data_workspace`.
- **Why it fits**:
  - The app coordinates multiple related read-only data panels across 8 tabs grouped into 3 domains (Billing / Ordini & XConnect / Energia Colo).
  - Filters/actions live *close to the data they affect* (per-tab filter bar on Transazioni WHMCS and Energia Colo; per-row action on Ordini Sales and XConnect list).
  - No dashboard-style KPI shell is required; the feature is operational, not metric-led — aligns with the archetype's "Forbidden defaults" clause ("dashboard-style KPI shells unless the feature is actually metric-led").
  - Individual tabs that include filters + export (Transazioni WHMCS) share the pattern of `report_explorer`, but the app-level composition (multi-tab workspace with heterogeneous sub-tasks, drill-down to detail) is a `data_workspace`. Same call as `reports`.
- **Required states** per view (coverage mandatory for the UI review):
  - **Populated** — table with data (every view).
  - **Loading** — skeleton or spinner on mount (every view).
  - **Empty** — "Nessun risultato" state (every view; with a tailored hint for Transazioni WHMCS: "Imposta un intervallo di date e premi Cerca").
  - **Error** — generic error banner with retry (every view; 503 gets the shared `ServiceUnavailable` component when available).
  - **Destructive-confirm** — **N/A** (pure read-only app, no destructive actions).

## User Copy Rules

- **Allowed copy style**: `business-user-only` (the portal default).
- Existing business-facing labels used verbatim: "Transazioni", "Fatture Prometeus", "Articoli da creare in Alyante", "Consumi Energia Colo", "Ordini Sales", "Dettaglio ordine", "DDT per cespiti", "Remote Hands", "XConnect", "Scarica PDF", "Esporta XLSX", "Cerca", "Torna alla lista", "Condizioni di pagamento", "Modalità di fatturazione canoni anticipata", "Modalità di fatturazione attivazione", "Durata rinnovo", "Tacito rinnovo", "Tipo di ordine", "Tipo di documento", "Dal CP?", "Tipo di servizi", "Nessun valore", "Nessuna nota legale".
- Tab group labels: "Billing", "Ordini & XConnect", "Energia" (matching the grouping in spec §B.1).
- Toast copy, verbatim from Appsmith where applicable: "Il PDF non è ancora pronto." (404 on order PDF), required-field warnings on ticket download.
- **Forbidden copy risks** (must not appear in user-facing UI):
  - Technical terms: `widget`, `datasource`, `carbone`, `renderId`, `templateId`, `JSObject`, `queryParams`, `orphan`, `pivot table` → write "tabella mensile" or "riepilogo per mese" instead.
  - Implementation terms: "replica dell'app originale", "senza aprire modali", "server-side", "inline update", "record", "id.asc".
  - Legacy field labels that are pure column names — use the business label from the Appsmith ternaries (e.g. "Codice articolo bundle" not `cdlan_codice_kit`).
- **Metrics allowed**: **none**. No stat cards, no KPI row. The Appsmith source has zero summary metrics; the Metrics gate forbids inventing them. The only numeric surfaces are the tables themselves.

## Repo-Fit

- **Route/base path**: `/afc-tools` (following `/reports`, `/energia-dc`, `/listini-e-sconti` convention). Internal routes under `/afc-tools/*` per the table in spec §B.1. `/` redirects to `/afc-tools/transazioni-whmcs` (first tab, matches `energia-dc` pattern).
- **API prefix**: `/api/afc-tools/` (see spec §D.3 for the 13 endpoints). Backend package: `backend/internal/afctools/`. Registered in `backend/cmd/server/main.go` via `afctools.RegisterRoutes(r, ...)` per repo convention (as done for `reports`, `energiadc`, etc.).
- **Access role**: `app_afctools_access` (Keycloak, CLAUDE.md `app_{appname}_access` convention). Role-gated at the handler level for every endpoint.
- **Dev port / proxy notes**:
  - Vite port: **5186** (next free after 5185 used by `simulatori-vendita`).
  - `vite.config.ts` proxies `/api` → `http://localhost:8080` and `/config` → same (matches every other app).
  - `package.json` root: add `dev:afc-tools` script and add the app to the `dev` concurrently command (color + filter) per CLAUDE.md "New App Checklist".
  - `Makefile`: add `dev-afc-tools` target + add to `.PHONY` per CLAUDE.md "New App Checklist".
- **Static hosting / deployment notes**:
  - Production: built SPA served by the Go backend from `backend/static/afctools/` (the pattern used by every mini-app, as the static handler tests in `backend/internal/platform/staticspa/` confirm).
  - `deploy/Dockerfile` — add a `pnpm --filter @mrsmith/afc-tools build` step + `COPY` into the final image's static dir.
  - `deploy/k8s/configmap.yaml` — add `AFCTOOLS_APP_URL` (for portal launcher) and the two new DSNs: `VODKA_DSN`, `WHMCS_DSN`. Carbone template id under `CARBONE_AFCTOOLS_TRANSAZIONI_TEMPLATE_ID` (reuses the existing `CARBONE_API_TOKEN`).
  - `backend/internal/platform/applaunch/catalog.go` — register the app ID (`afctools`), href, and access role per CLAUDE.md checklist.
  - `backend/internal/platform/config/config.go` — add `AFCToolsAppURL`, `VodkaDSN`, `WhmcsDSN`, `CarboneAFCToolsTransazioniTemplateID` fields and env bindings.
  - `backend/cmd/server/main.go` — open Vodka + WHMCS connections (`database.New("mysql", …)`), thread them into the `afctools` package, add `hrefOverrides` entry for dev (`http://localhost:5186`), register routes.
  - `.env.preprod.example` + `backend/.env.example` — add the three new env entries (two DSNs + the carbone template id).
  - `docker-compose.dev.yaml` — add the two new DSNs (if preprod/dev needs direct DB access) and the carbone template id.

## Exceptions

- **E1 — No metrics, no stats, no summary cards** (deliberate choice, not a deviation): flagged here for the UI reviewer because the archetype *allows* metrics when justified. AFC Tools explicitly does not have them in the Appsmith source, and inventing them would violate both the Metrics gate and the 1:1 directive. User benefit: avoids noise in an operational workspace where the table *is* the answer.
- **E2 — Carbone XLSX download opens in a new browser tab** instead of streaming a blob to the current tab. Reason: decision A.5.4 = 4a preserves the Appsmith "open render URL in new tab" UX, which the audit notes as user-learned behavior. Divergence from `apps/reports` is intentional; documented so the reviewer does not flag it as inconsistency. User benefit: identical UX to today's AFC Tools users during the migration window.
- **E3 — DDT cespiti preserves `SELECT *` with no pagination** (decision A.5.1e). Violates the implicit "tables should paginate" expectation of the archetype but is a preserved 1:1 behavior with a TODO in `docs/TODO.md`. User benefit: guaranteed behavioral parity with Appsmith during coexistence; timing of the pagination pass decided later.
- **E4 — Dettaglio ordini header is a grid of labelled text fields**, not a CRUD form. The `master_detail_crud` archetype would be a cleaner fit shape-wise, but AFC Tools detail is read-only and would require inventing a non-existent edit/create path. Staying with `data_workspace` + a custom read-only detail page mirrors `apps/reports`-style detail drill-down. User benefit: same behavior as today, zero risk of accidentally wiring writable UI against read-only SQL.
- **E5 — Two new MySQL DSNs** (`VODKA_DSN`, `WHMCS_DSN`) are required for the port. Not a UX exception; it's an infra exception listed here so the repo-fit review notices that the preprod/prod Secrets + ConfigMap changes are scheduled as part of this change, not left implicit.

## Verification

- **UI review checks** (handoff to `portal-miniapp-ui-review`, pre-gate and post-gate):
  - Comparable Apps: cite `apps/reports/src/pages/AccessiAttiviPage.tsx` (filter+table+export), `apps/energia-dc/src/pages/AddebitiPage.tsx` (filter+table), `apps/quotes/src/pages/QuoteListPage.tsx` + `QuoteDetailPage.tsx` (list→detail).
  - Archetype: `data_workspace`, one primary choice.
  - Copy gate: every label matches the inventory in §User Copy Rules; no forbidden terms reach user-facing UI.
  - Metrics gate: zero stat cards, zero KPI rows (intentional, per E1).
  - Style gate: `clean` theme, `AppShell` + `TabNavGroup`, aligned with `reports`.
  - Repo-Fit gate: route `/afc-tools`, API `/api/afc-tools/`, role `app_afctools_access`, port 5186, static path `backend/static/afctools/`, all five CLAUDE.md checklist files updated.
  - States: populated / loading / empty / error captured for each of the 8 views. Destructive-confirm N/A.
  - Narrow viewport: tables must horizontally scroll at ≤ 768 px (Dettaglio ordini header grid collapses to single-column) — per `reports` convention.
- **Runtime / auth checks**:
  - Keycloak role `app_afctools_access` assigned to the AFC team group.
  - Every endpoint under `/api/afc-tools/` returns 403 without the role (test with a user in `app_reports_access` only).
  - Deep-link refresh at `/afc-tools/ordini-sales/:id` resolves correctly (backend static handler falls through to `index.html` — tested by `backend/internal/platform/staticspa/handler_test.go` pattern).
  - Carbone export: backend holds the API token + template id; browser dev tools confirm no `templateId` leaks into the JS bundle or network payload. `renderUrl` is opened via `window.open(url, '_blank')` (no downloadable artifact leaves the backend).
  - PDF download: gateway proxy attaches the service-account OAuth2 Bearer; frontend does not see the gateway token.
  - Two new DSN secrets present in Kubernetes Secret `mrsmith-db-credentials` (or equivalent) before first preprod deploy.
- **Tests** (per `AGENTS.md` Test Rule — tests only where they protect a reproduced bug, a business-critical rule, or non-trivial query/data transformation):
  - **Backend**:
    - `repo/whmcs_test.go` — `ListTransactions`: verify the `date > 20230120` floor + invoice/refund filter compose correctly with a date-range parameter. Non-trivial SQL.
    - `repo/grappa_test.go` — `ListEnergiaColoPivot`: verify the 12-month pivot returns one row per customer with all 12 month columns, and `IF(ampere>0, ampere, Kw)` metric resolves correctly. Non-trivial business rule.
    - `repo/mistra_test.go` — `ListMissingArticles`: verify the anti-join returns only products with `erp_sync=true` not present in `erp_anagrafica_articoli_vendita`. Business-critical rule.
    - `repo/vodka_test.go` — `GetOrder` + `ListOrderRows`: verify the pre-computed `cdlan_int_fatturazione_desc` label matches all six codes (1,2,3,5,6, else), and `Codice articolo bundle` composition returns `''` when `cdlan_codice_kit = ''`. Business-critical mapping + bug-adjacent (Q-E1).
    - `handler_test.go` — authorization: every endpoint returns 403 without `app_afctools_access`.
    - `carbone_test.go` — `ExportTransactions` returns `{renderId, renderUrl}` with the expected URL shape; `templateId` never appears in the response.
  - **Frontend** (minimal per the Test Rule — no Dettaglio ordini snapshot test, no UI-level regression coverage unless a bug is reproduced):
    - Unit test for the `paymentTermsLabel(code: number)` lookup — covers all 18 codes (301, 303, 304, 311–316, 318, 400, 402–407, 409) including the post-fix `400 → 'SDD FM'` (decision A.5.1a) and the deliberate gaps (317, 308, 309, 401, 408 → `''`).
    - Unit test for `isEmpty(v)` helper: `null`, `undefined`, `''` → `true` (decision A.5.1c).
  - **Behavioral parity check** (manual, post-migration, §Acceptance Notes of the spec): sample ≥ 5 real orders through Dettaglio ordini and confirm every ternary label matches Appsmith — except `cod_termini_pag == 400`, which is the deliberate fix.

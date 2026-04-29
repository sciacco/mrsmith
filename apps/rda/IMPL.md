# RDA — Implementation Plan

> Spec source: `apps/rda/audit/rda-migspec-FINAL.md`
> Supporting references read: `docs/IMPLEMENTATION-PLANNING.md`, `docs/IMPLEMENTATION-KNOWLEDGE.md`, `docs/UI-UX.md`, `docs/mistra-dist.yaml`
> Status: execution-ready draft for an implementation agent

## How To Use This Plan

This document is written for an LLM implementation agent. Treat it as the single implementation handoff for the RDA cutover.

Read these files before editing code:

1. `apps/rda/audit/rda-migspec-FINAL.md`
2. `apps/rda/IMPL.md`
3. `docs/UI-UX.md`
4. `docs/IMPLEMENTATION-PLANNING.md`
5. `docs/IMPLEMENTATION-KNOWLEDGE.md`
6. `docs/mistra-dist.yaml`, only the RDA, budget-for-user, users-int, and provider schemas/endpoints needed for the slice being implemented

Do not let older phase files override `rda-migspec-FINAL.md`. If an older phase document disagrees with the final spec, the final spec wins. In particular, this plan follows the final spec for the submit prerequisite: `total_price >= 3000 EUR` requires at least 2 attachments, counting all attachments 1:1 with the legacy app.

## Non-Negotiable Scope

Build a React + Go portal mini-app that is a 1:1 functional port of the Appsmith RDA app.

Keep these constraints intact:

- No new Mistra endpoints.
- No new database tables, schemas, or migrations.
- No client-side database access.
- No browser-supplied `Requester-Email`; derive it from OIDC claims in Go.
- Reuse the existing Mistra NG `/arak/rda/...` API wire surface unchanged.
- Reuse the existing `fornitori` module for provider/provider-reference UI calls.
- Keep the app desktop-first and clean mini-app styled; no launcher-style Matrix UI.
- Row edit is exposed through the BFF as a create-then-delete replacement because Mistra has no row update endpoint.
- Keep @-mentions cosmetic in v1. Do not send `mentioned_user_ids`.
- Do not add automated tests unless the user explicitly approves them in the implementation session.

## Comparable Apps Audit

Reference 1: `apps/budget/src/views/gruppi/GruppiPage.tsx`

- Reuse the master/detail CRUD discipline: compact title, primary action, data surface, selected-row detail, explicit empty/error/loading states.
- Reuse the way actions stay close to the working surface instead of adding explanatory panels.
- Reject its bespoke inline SVGs for new work where `@mrsmith/ui` `Icon` has an available icon.

Reference 2: `apps/listini-e-sconti/src/pages/GruppiScontoPage.tsx`

- Reuse the business-workspace tone: a short header, a single main surface, selectors near the data they affect, and modal editing where needed.
- Reuse compact table/form composition for dense business data.
- Reject nested card-heavy layout when RDA detail already has tabs, comments, and an action bar.

Reference 3: `apps/reports/src/pages/OrdiniPage.tsx`

- Reuse authenticated downloads via `api.getBlob`/blob save patterns and report/table density.
- Reuse real output metrics only when the screen is report-led.
- Reject report-style metric cards for RDA because RDA is workflow CRUD, not a report explorer.

Additional repo patterns inspected:

- `apps/fornitori/src/App.tsx`, `routes.tsx`, `api/queries.ts`, `styles/global.css` for app shell, auth bootstrap, React Query, and clean-theme background.
- `backend/internal/fornitori/handler.go` for Arak proxying, multipart upload, direct Arak DB catalog reads, sanitized dependency errors, and response streaming.
- `backend/internal/platform/applaunch/catalog.go`, `backend/cmd/server/main.go`, `deploy/Dockerfile`, `package.json`, `Makefile`, and env examples for app registration and deployment fit.

## Archetype Choice

Selected archetype: `master_detail_crud`.

Why it fits:

- RDA is a single aggregate registry: Purchase Orders with nested rows, attachments, comments, recipients, and workflow state.
- `/rda` is the master list plus create dialog.
- `/rda/po/:poId` is the detail editor/workflow surface.
- `/rda/inbox/:kind` is a constrained list variant for approval work.

Required states:

- List loading, empty, error, populated.
- Create-modal validation error and submit pending.
- Delete confirm for DRAFT POs.
- Inbox forbidden state for missing role.
- Detail loading, not found/error, populated read-only, populated editable, dirty header state, transition pending.
- Attachment upload pending/error, file download pending/error.
- Row create validation error.
- Submit confirm.
- Action buttons disabled with business tooltips where the spec requires visible-but-disabled behavior.

Do not introduce another archetype. The detail page has tabs, but they are part of the CRUD detail surface, not a separate dashboard or report workspace.

## User Copy Rules

Allowed copy style:

- Italian, business-user-facing, direct, and action-oriented.
- Preserve legacy business labels unless the final spec explicitly fixes them.
- Use the Q-A1 state label fix: `IN ATTESA VERIFICA CONFORMITÀ`.
- Use concise button labels such as `Nuova richiesta`, `Crea bozza`, `Manda in approvazione`, `Approva`, `Rifiuta`, `Scarica`, `Elimina`.

Forbidden copy risks:

- No technical UI copy such as `server-side`, `inline update`, `record`, `widget`, `datasource`, `id.asc`, `proxy`, `Mistra wire`, or implementation explanations.
- No text explaining that this is a port of Appsmith.
- No invented walkthrough panels or onboarding copy.
- No dashboard KPI or decorative summary cards.

Metrics allowed:

- None by default.
- A total PO amount is allowed because it is real PO data and part of the legacy workflow.
- Do not add counts or totals just to fill space.

## Repo-Fit Summary

Route/base path:

- Production Vite base: `/apps/rda/`.
- Launcher href: `/apps/rda/`.
- Client routes under the app basename:
  - `/` redirects to `/rda`
  - `/rda`
  - `/rda/inbox/:kind`
  - `/rda/po/:poId`
- Full production URLs therefore look like `/apps/rda/rda`, `/apps/rda/rda/inbox/level1-2`, `/apps/rda/rda/po/123`.
- Deep links rely on the existing `staticspa` fallback once Docker copies the built dist to `/static/apps/rda`.

API prefix:

- Browser calls `/api/rda/v1/...`.
- `backend/cmd/server/main.go` strips `/api`, so RDA registers routes under `/rda/v1/...`.
- Provider calls use `/api/fornitori/v1/...` directly, not `/api/rda/v1/providers/...`.

Access roles:

- `app_rda_access`: base app access and launcher tile.
- `app_rda_approver_l1l2`: L1/L2 inbox and L1/L2 approve/reject.
- `app_rda_approver_afc`: leasing, payment-method, and leasing-created actions.
- `app_rda_approver_no_leasing`: no-leasing inbox and approve/reject.
- `app_rda_approver_extra_budget`: budget-increment inbox and approve/reject.
- `app_fornitori_access` must be bundled at Keycloak group level with `app_rda_access` for v1.
- `app_devadmin` bypasses role checks through existing authz helpers.

Dev port / proxy:

- Use Vite port `5190`. The old Phase D suggestion `5184` now collides with `apps/energia-dc`.
- Proxy `/api` and `/config` to `VITE_DEV_BACKEND_URL || http://localhost:8080`.
- Add `http://localhost:5190` to config default `CORS_ORIGINS` and env examples.
- If touching `backend/.env.example`, preserve existing origins and also include `http://localhost:5189` because `fornitori` uses 5189 and the example is currently stale.

Static hosting / deployment:

- Add `COPY --from=frontend /app/apps/rda/dist /static/apps/rda` to `deploy/Dockerfile`.
- Add `RDA_APP_URL` to backend config and env examples for split-server local override.
- Add a local launcher override to `main.go`: configured `RDA_APP_URL`, otherwise `http://localhost:5190` when `STATIC_DIR == ""`.

## Exceptions

- `/rda/po/:poId` is denser than the smallest CRUD screen because the legacy PO aggregate has workflow actions, tabs, comments, rows, attachments, and contacts. This is not a visual exception; it is the minimum faithful surface for the business task.
- The inboxes are not separate launcher apps in v1. They are in-app tabs/routes gated by role. This keeps one app shell and avoids five duplicated mini-app tiles.
- @-mentions remain visually available but have no notification effect in v1. This is an explicit parity decision from the final spec.

## UI Review Gates

Before coding UI, hand this plan to `portal-miniapp-ui-review` for a pre-gate review. The review input must include:

- Comparable screens: `apps/budget/src/views/gruppi/GruppiPage.tsx`, `apps/listini-e-sconti/src/pages/GruppiScontoPage.tsx`, `apps/reports/src/pages/OrdiniPage.tsx`.
- Primary archetype: `master_detail_crud`.
- Exceptions section above.
- Copy rules above.

After implementation, run a post-gate UI review with screenshots for:

- `/rda` populated list + create modal open.
- `/rda` empty state.
- `/rda/inbox/level1-2` populated or mocked populated state.
- `/rda/po/:poId` DRAFT state with tabs visible.
- `/rda/po/:poId` non-DRAFT state with workflow actions visible/disabled as appropriate.
- One error/forbidden state.
- Narrow viewport check for text overlap, even though mobile is not a v1 target.

## Implementation Sequence

Execute the slices in order. Each slice should leave the repo compiling before continuing.

### Slice 0 — Preflight And Contracts

Goal: pin the contract and avoid re-discovery.

Tasks:

- Re-read `apps/rda/audit/rda-migspec-FINAL.md`.
- Use `docs/mistra-dist.yaml` as the source for upstream RDA path spelling and schema names.
- Confirm that `apps/rda/` currently contains only audit/spec files and `.env.local`; the app itself must be scaffolded.
- Confirm there is no existing `backend/internal/rda/`.
- Confirm Vite port `5190` does not collide with existing `vite.config.ts` files.
- Keep a short implementation note in PR/session output that `5184` was not used because it belongs to `energia-dc`.

Do not:

- Start by generating code from OpenAPI. The live response includes audit-only `recipients[]` and `approvers[]`; hand-model the superset.
- Add dependencies just for validation. Use local TypeScript/Go validators unless the user approves a dependency change.

### Slice 1 — Runtime Wiring

Goal: make RDA a first-class app in the monorepo and launcher.

Files to create:

- `apps/rda/package.json`
- `apps/rda/tsconfig.json`
- `apps/rda/index.html`
- `apps/rda/vite.config.ts`
- `apps/rda/src/main.tsx`
- `apps/rda/src/App.tsx`
- `apps/rda/src/routes.tsx`
- `apps/rda/src/styles/global.css`
- `apps/rda/src/vite-env.d.ts`

Files to modify:

- `package.json`
- `Makefile`
- `backend/internal/platform/applaunch/catalog.go`
- `backend/internal/platform/config/config.go`
- `backend/cmd/server/main.go`
- `backend/.env.example`
- `.env.preprod.example`
- `deploy/Dockerfile`
- `docker-compose.dev.yaml` if the repo expects `make dev-docker` to include every mini-app

Frontend scaffold:

- `apps/rda/package.json`
  - `name`: `mrsmith-rda`
  - scripts: `dev`, `build`, `lint`, `preview`
  - dependencies: `@mrsmith/api-client`, `@mrsmith/auth-client`, `@mrsmith/ui`, `@tanstack/react-query`, `react`, `react-dom`, `react-router-dom`
  - devDependencies matching `apps/fornitori` and `apps/manutenzioni`
- `vite.config.ts`
  - base: `command === 'build' ? '/apps/rda/' : '/'`
  - server port: `5190`
  - proxy `/api` and `/config` to `backendTarget`
- `main.tsx`
  - Set `document.documentElement.dataset.theme = 'clean'`.
  - Fetch `/config`.
  - Wrap `AuthProvider`, `QueryClientProvider`, `BrowserRouter`, and `ToastProvider`.
  - Use the same retry policy as `apps/fornitori/src/main.tsx`.
  - Derive basename from `import.meta.env.BASE_URL`.
- `App.tsx`
  - Use `AppShell` with app name `RDA`.
  - Use `TabNav` with role-aware items:
    - Always show `Le mie RDA` -> `/rda`.
    - Show `I/II livello` -> `/rda/inbox/level1-2` when the user has `app_rda_approver_l1l2`.
    - Show `Leasing` -> `/rda/inbox/leasing` when `app_rda_approver_afc`.
    - Show `Metodo pagamento` -> `/rda/inbox/payment-method` when `app_rda_approver_afc`.
    - Show `No leasing` -> `/rda/inbox/no-leasing` when `app_rda_approver_no_leasing`.
    - Show `Incremento budget` -> `/rda/inbox/budget-increment` when `app_rda_approver_extra_budget`.
  - Do not show a separate launcher-like home page.
- `routes.tsx`
  - `{ index: true, element: <Navigate to="/rda" replace /> }`
  - `/rda` -> `RdaListPage`
  - `/rda/inbox/:kind` -> `InboxPage`
  - `/rda/po/:poId` -> `PoDetailPage`
  - `*` -> redirect to `/rda`
- `global.css`
  - Import `@mrsmith/ui/src/themes/clean.css`.
  - Use the approved clean mini-app background from `docs/UI-UX.md`.
  - Copy the app-shell/nav/body reset pattern from `apps/fornitori/src/styles/global.css`, then add only RDA-specific page primitives.

Root package/Makefile:

- Add root script: `"dev:rda": "pnpm --filter mrsmith-rda dev"`.
- Add `rda` to the aggregate `dev` concurrently command and names/colors.
- Add `dev-rda` target to `Makefile`.
- Add `dev-rda` to `.PHONY`.

Launcher catalog:

- Add constants:
  - `RDAAppID = "rda"`
  - `RDAAppHref = "/apps/rda/"`
- Add role slices:
  - `rdaAccessRoles`
  - `rdaApproverL1L2Roles`
  - `rdaApproverAFCRoles`
  - `rdaApproverNoLeasingRoles`
  - `rdaApproverExtraBudgetRoles`
- Add accessors for each role slice.
- Add role groups to `AllRoles()`.
- Add catalog entry under `Acquisti`:
  - ID `RDAAppID`
  - Name `RDA`
  - Description optional, business-facing only
  - Icon `cart`
  - Href `RDAAppHref`
  - Status `ready` only when the implementation is actually complete; use `dev` during incremental work if needed
  - Access roles `RDAAccessRoles()`

Backend config/main:

- Add `RDAAppURL string` to `Config`.
- Load from `RDA_APP_URL`.
- Add default CORS origin `http://localhost:5190`.
- Add RDA href override in `main.go`.
- Add catalog filter: hide RDA when `arakCli == nil || arakDB == nil` because RDA needs both Mistra Arak and Arak Postgres payment-method reads.
- Import `backend/internal/rda`.
- Call `rda.RegisterRoutes(api, rda.Deps{Arak: arakCli, ArakDB: arakDB, Logger: logger})` after dependencies are initialized.

Acceptance:

- `pnpm --filter mrsmith-rda exec tsc --noEmit` passes after placeholder pages exist.
- `cd backend && go build ./cmd/server` passes after backend stubs compile.
- Portal catalog includes RDA only for users with `app_rda_access`.

### Slice 2 — Backend RDA Module

Goal: implement the BFF boundary that proxies Mistra while fixing identity, role, body-shaping, and direct DB issues.

Create backend files:

- `backend/internal/rda/handler.go`
- `backend/internal/rda/types.go`
- `backend/internal/rda/arak.go`
- `backend/internal/rda/payments.go`
- `backend/internal/rda/permissions.go`
- `backend/internal/rda/validation.go`

Suggested package shape:

```go
type Deps struct {
    Arak   *arak.Client
    ArakDB *sql.DB
    Logger *slog.Logger
}

type Handler struct {
    arak   *arak.Client
    arakDB *sql.DB
    logger *slog.Logger
}
```

Constants:

- `component = "rda"`
- `arakRDARoot = "/arak/rda/v1"`
- `arakBudgetRoot = "/arak/budget/v1"`
- `arakUsersRoot = "/arak/users-int/v1"`
- `arakProviderRoot = "/arak/provider-qualification/v1"` only for server-side create body shaping if provider detail must be fetched
- `maxUploadBytes = 25 << 20`

Core helpers:

- `claims(r)` returns `auth.Claims` or 401.
- `currentEmail(r)` trims and validates `claims.Email`.
- `requesterHeaders(email)` returns headers with `Requester-Email: email`.
- `budgetHeaders(email)` returns headers with `user_email: email`.
- `requireArak(w)` returns 503 with `{code:"DEPENDENCY_UNAVAILABLE"}` if missing.
- `requireArakDB(w)` returns 503 with `{code:"DEPENDENCY_UNAVAILABLE"}` if missing.
- `forwardArak(w,r,method,path,rawQuery,body,headers)` streams upstream response and copies `Content-Type`, `Content-Disposition`, and `Content-Length`.
- `upstreamAuthFailed` maps upstream 401/403 to a sanitized 502-style app response like the `fornitori` module.
- `listQueryWithDefaults(r)` forwards whitelisted query params and defaults `page_number=1&disable_pagination=true`.
- `fetchPODetail(ctx,email,id)` GETs `/arak/rda/v1/po/{id}` and decodes the detail superset including audit-only `recipients` and `approvers`.
- `isRequester(po,email)` compares lowercased `po.requester.email`.
- `isApprover(po,email)` checks `po.approvers[*].user.email`.
- `parseTotalPrice(value string)` strips known non-numeric suffixes and returns decimal/float for validation and display.

Route registration:

Use `acl.RequireRole(applaunch.RDAAccessRoles()...)` for every RDA endpoint. For inboxes and privileged transitions, add explicit role checks in handlers or nested middleware.

Register public routes under:

```text
GET    /rda/v1/me/permissions
GET    /rda/v1/budgets
GET    /rda/v1/payment-methods
GET    /rda/v1/payment-methods/default
GET    /rda/v1/articles
GET    /rda/v1/users

GET    /rda/v1/pos
GET    /rda/v1/pos/inbox/{kind}
POST   /rda/v1/pos
GET    /rda/v1/pos/{id}
PATCH  /rda/v1/pos/{id}
DELETE /rda/v1/pos/{id}

POST   /rda/v1/pos/{id}/submit
POST   /rda/v1/pos/{id}/approve
POST   /rda/v1/pos/{id}/reject
POST   /rda/v1/pos/{id}/leasing/approve
POST   /rda/v1/pos/{id}/leasing/reject
POST   /rda/v1/pos/{id}/leasing/created
POST   /rda/v1/pos/{id}/no-leasing/approve
POST   /rda/v1/pos/{id}/payment-method/approve
PATCH  /rda/v1/pos/{id}/payment-method
POST   /rda/v1/pos/{id}/budget-increment/approve
POST   /rda/v1/pos/{id}/budget-increment/reject
POST   /rda/v1/pos/{id}/conformity/confirm
POST   /rda/v1/pos/{id}/conformity/reject
POST   /rda/v1/pos/{id}/send-to-provider
GET    /rda/v1/pos/{id}/pdf

POST   /rda/v1/pos/{id}/rows
DELETE /rda/v1/pos/{id}/rows/{rowId}

POST   /rda/v1/pos/{id}/attachments
GET    /rda/v1/pos/{id}/attachments/{aid}
DELETE /rda/v1/pos/{id}/attachments/{aid}

GET    /rda/v1/pos/{id}/comments
POST   /rda/v1/pos/{id}/comments
```

Do not register `/rda/v1/providers...`; RDA frontend uses `/fornitori/v1/...`.

Identity and permissions:

- `/me/permissions` reads `auth.Claims.Roles` and returns:
  - `is_approver`
  - `is_afc`
  - `is_approver_no_leasing`
  - `is_approver_extra_budget`
- Use `authz.HasAnyRole` so `app_devadmin` works.
- Do not query `users_int.role`.

Catalog endpoints:

- `/budgets` -> `GET /arak/budget/v1/budget-for-user?page_number=1&disable_pagination=true` with `user_email` header from claims.
- `/payment-methods` -> Arak Postgres:
  - `SELECT code, description, COALESCE(rda_available, false) FROM provider_qualifications.payment_method WHERE rda_available IS TRUE ORDER BY description ASC`
- `/payment-methods/default` -> Arak Postgres:
  - `SELECT payment_method_code FROM provider_qualifications.payment_method_default_cdlan LIMIT 1`
  - Return `{payment_method_code:string}` or `{code:string}`; pick one and mirror it in TS. Prefer `{code:string}` for frontend clarity.
- `/articles?type=good|service&search=...` -> `GET /arak/rda/v1/article?page_number=1&disable_pagination=true&type=...&search_string=...`.
- `/users?search=...` -> `GET /arak/users-int/v1/user?page_number=1&disable_pagination=true&enabled=true&search_string=...`.

PO list and inbox endpoints:

- `/pos` forwards to `/arak/rda/v1/po`.
- `/pos/inbox/{kind}` mapping:
  - `level1-2` -> `/arak/rda/v1/po/pending-approval`, role `app_rda_approver_l1l2`
  - `leasing` -> `/arak/rda/v1/po/pending-leasing`, role `app_rda_approver_afc`
  - `no-leasing` -> `/arak/rda/v1/po/pending-approval-no-leasing`, role `app_rda_approver_no_leasing`
  - `payment-method` -> `/arak/rda/v1/po/pending-approval-payment-method`, role `app_rda_approver_afc`
  - `budget-increment` -> `/arak/rda/v1/po-pending-budget-increment`, role `app_rda_approver_extra_budget`
- Unknown kind returns 404.
- All forwards set `Requester-Email` from claims.

PO create:

- Accept a normalized request:
  - `type`: `STANDARD` or `ECOMMERCE`
  - `budget_id`
  - exactly one of `cost_center` or `budget_user_id`
  - `provider_id`
  - optional `payment_method`
  - `project`
  - `object`
  - optional `description`
  - optional `note`
  - optional `provider_offer_code`
  - optional `provider_offer_date`
- Backend shapes the Mistra body:
  - `type`
  - `project`
  - `object`
  - `reference_warehouse: "MILANO"`
  - `currency: "EUR"` if live Mistra still accepts it; harmless extra fields should be confirmed during implementation
  - `language`: provider language from provider detail if available, otherwise `it`
  - `payment_method`: request value, else provider default, else CDLAN default from DB
  - `recipient_ids: []`
  - `provider_id`
  - `budget_id`
  - either `cost_center` or `budget_user_id`, never both
  - `cap`/`vat` only if live Mistra still requires them; derive from provider detail rather than trusting browser-supplied values
- Do not ever use literal `"320"`.
- Forward `POST /arak/rda/v1/po`.
- Return the Mistra `id-object` unchanged enough for frontend navigation.

PO patch:

- Accept partial fields:
  - `type`, `budget_id`, `budget_user_id`, `cost_center`, `description`, `object`, `note`, `payment_method`, `reference_warehouse`, `provider_id`, `project`, `provider_offer_code`, `provider_offer_date`, `recipient_ids`
- Decode into pointer fields or a raw map so empty strings and explicit `null` are not lost.
- Enforce requester + `DRAFT` for generic PATCH.
- If provider changes, allow `recipient_ids: []` in the same request.
- Forward only present fields. This fixes legacy F-7 truthiness bugs.

Delete:

- Fetch PO detail.
- Require requester + `DRAFT`.
- Forward `DELETE /arak/rda/v1/po/{id}`.

Rows:

- `POST /rows`
  - Fetch PO detail.
  - Require requester + `DRAFT`.
  - Validate item rules:
    - Common: `type`, `description`, `qty > 0`, `product_code`, `product_description`, `payment_detail.start_at`.
    - Good: `price > 0`; allowed `start_at` values `activation_date`, `advance_payment`, `specific_date`.
    - Service: at least one of MRC or NRC > 0; `initial_subscription_months` required; `month_recursion` required; if `automatic_renew`, `cancellation_advice` required.
    - If `start_at == specific_date`, `start_at_date` required.
  - Set `requester_email` from claims, not request body.
  - Bridge write/read field divergences:
    - Write `activation_price`; read is `activation_fee`.
    - Write `payment_detail.start_at`; read has `start_pay_at_activation_date`.
    - Preserve upstream typo `montly_fee` in read types.
  - Forward `POST /arak/rda/v1/po/{id}/row`.
- `DELETE /rows/{rowId}`
  - Fetch PO detail.
  - Require requester + `DRAFT`.
  - Optionally verify the row exists under this PO before forwarding for clearer 404.
  - Forward `DELETE /arak/rda/v1/po/{id}/row/{rowId}`.
- `PUT /rows/{rowId}`
  - Fetch PO detail.
  - Require requester + `DRAFT`.
  - Verify the target row exists under this PO.
  - Validate and build the same row body used by `POST /rows`.
  - Forward create first, then delete the old row only after create succeeds.
  - If delete fails after create, return `409 ROW_REPLACE_DELETE_FAILED` so the frontend refetches and warns the operator.

Attachments:

- `POST /attachments`
  - Fetch PO detail.
  - Allow only `DRAFT` or `PENDING_VERIFICATION`.
  - Limit to 25 MiB using `http.MaxBytesReader`.
  - Validate a `file` part exists and is non-empty.
  - Derive `attachment_type` from current state:
    - `DRAFT` -> `quote`
    - anything else allowed here -> `transport_document`
  - Build a fresh multipart body with original file content plus derived `attachment_type`.
  - Forward to `POST /arak/rda/v1/po/{id}/attachment`.
- `GET /attachments/{aid}`
  - Forward to `/arak/rda/v1/po/{po_id}/attachment/{attachment_id}/download`.
  - Stream bytes and copy content headers. A 302 redirect is acceptable only if the upstream returns a signed URL; default plan is streaming because OpenAPI declares octet-stream.
- `DELETE /attachments/{aid}`
  - Fetch PO detail.
  - Require requester + `DRAFT`.
  - Forward delete.

Comments:

- `GET /comments` -> forward to `/arak/rda/v1/po/{id}/comment?page_number=1&disable_pagination=true`.
- `POST /comments`
  - Accept `{comment:string, mentioned_user_ids?:number[]}` but ignore IDs in v1.
  - Forward `{comment}` only.
  - Set `Requester-Email`.
  - Do not query numeric user id unless implementation proves Mistra rejects the body without it. If that happens, pause and record the discovery before adding lookup logic.

Transitions:

- `submit`
  - Fetch PO detail.
  - Require requester + `DRAFT`.
  - Require at least one row.
  - If parsed total price is `>= 3000`, require `len(attachments) >= 2`.
  - Forward `POST /arak/rda/v1/po/{id}/submit`.
- `approve`
  - Fetch PO detail.
  - Require state `PENDING_APPROVAL`.
  - Require role `app_rda_approver_l1l2`.
  - Require current email in `approvers[]`.
  - Forward `/approve`.
- `reject`
  - Fetch PO detail and choose role by state:
    - `PENDING_APPROVAL`: `app_rda_approver_l1l2` and email in `approvers[]`
    - `PENDING_APPROVAL_PAYMENT_METHOD`: `app_rda_approver_afc`
    - `PENDING_APPROVAL_NO_LEASING`: `app_rda_approver_no_leasing`
  - Forward `/reject`.
  - Unknown state returns 403 or 409 with business-facing error.
- `payment-method/approve`: require `app_rda_approver_afc`; forward.
- `payment-method` PATCH:
  - Fetch PO detail.
  - Require requester.
  - Require state `PENDING_APPROVAL_PAYMENT_METHOD`.
  - Forward `{payment_method}` to `/payment-method`.
- `leasing/approve`, `leasing/reject`, `leasing/created`: require `app_rda_approver_afc`; forward.
- `no-leasing/approve`: require `app_rda_approver_no_leasing`; forward.
- `budget-increment/approve` and `budget-increment/reject`: require `app_rda_approver_extra_budget`.
  - For approve, body must contain `increment_promise`.
  - For reject, final spec says same body in public contract; OpenAPI does not require it for reject. Preserve the public body shape and forward only what Mistra accepts. If Mistra rejects a body on reject, send no body and document it.
- `send-to-provider`: no role beyond base access; forward state-only.
- `conformity/confirm` and `conformity/reject`: no role beyond base access; forward state-only.
- `pdf`: no role beyond base access; forward `/download` and stream bytes.

Error and logging rules:

- Use `logging.FromContext(r.Context()).With("component","rda","operation",...)`.
- Log operation, upstream path, and status, but not request bodies with PII.
- Return generic `internal_server_error` for internal failures using existing `httputil.InternalError`.
- Return business-facing 400/403/409 messages for validation failures.
- Preserve upstream JSON error body only when it is safe and user-facing; otherwise sanitize.

Acceptance:

- `cd backend && go build ./cmd/server` passes.
- Existing `go test ./...` may be run, but do not add new tests unless approved.
- With `SKIP_KEYCLOAK=true`, fake roles from `applaunch.AllRoles()` include RDA roles.

### Slice 3 — Frontend API Types And Query Hooks

Goal: give UI slices a typed, centralized API layer.

Create frontend files:

- `apps/rda/src/api/client.ts`
- `apps/rda/src/api/types.ts`
- `apps/rda/src/api/queries.ts`
- `apps/rda/src/hooks/useOptionalAuth.ts`
- `apps/rda/src/hooks/useHasRole.ts` if useful
- `apps/rda/src/lib/roles.ts`
- `apps/rda/src/lib/state-labels.ts`
- `apps/rda/src/lib/format.ts`
- `apps/rda/src/lib/provider-refs.ts`
- `apps/rda/src/lib/validation.ts`

API client:

- Mirror `apps/fornitori/src/api/client.ts`.
- `baseUrl: '/api'`.
- Use auth token and force-refresh from `useOptionalAuth`.

Types:

Model the superset, not only OpenAPI:

- `PoPreview`
- `PoDetail`
- `PoRow`
- `PoAttachment`
- `PoComment`
- `PoCommentUser`
- `ProviderSummary`
- `ProviderReference`
- `BudgetForUser`
- `PaymentMethod`
- `Article`
- `RdaPermissions`
- `PagedEnvelope<T>` supporting both `total_number` and `total_items`
- `BudgetIncrementPoPreview` with `budget_increment_needed`

Important type notes:

- `total_price` is a string from Mistra.
- `created`, `creation_date`, and `updated` may appear with inconsistent names/formats.
- `recipients[]` and `approvers[]` are audit-proven live fields even though `rda-document-detail` omits them.
- Comment text may be `comment` or `comment_text`; normalize in frontend helper.
- `montly_fee` typo stays in read types.
- Do not surface `mentioned_user_ids` as sent data in v1.

Query keys:

- `['rda','permissions']`
- `['rda','budgets']`
- `['rda','payment-methods']`
- `['rda','payment-method-default']`
- `['rda','articles', type, search]`
- `['rda','users', search]`
- `['rda','pos']`
- `['rda','inbox', kind]`
- `['rda','po', id]`
- `['rda','comments', id]`
- Provider data can use `['fornitori', ...]` keys if implemented locally in RDA or imported from Fornitori patterns.

Hooks:

- `usePermissions()`
- `useBudgets()`
- `usePaymentMethods()`
- `usePaymentMethodDefault()`
- `useArticles(type, search)`
- `useUserSearch(search, enabled)`
- `useMyPOs()`
- `useInbox(kind)`
- `usePODetail(id)`
- `usePOComments(id)`
- `useCreatePO()`
- `usePatchPO(id)`
- `useDeletePO()`
- `useSubmitPO()`
- `useTransitionMutation(action)`
- `useCreateRow()`
- `useDeleteRow()`
- `useUploadAttachment()`
- `useDeleteAttachment()`
- `downloadAttachment(id, aid)` using `api.getBlob`
- `downloadPDF(id)` using `api.getBlob`
- `usePostComment()`

Mutation invalidation:

- Any PO mutation invalidates `['rda','po', id]`, `['rda','pos']`, and relevant `['rda','inbox', kind]`.
- Row/attachment/comment mutations invalidate detail and comments as appropriate.
- Provider ref mutations invalidate provider detail and current PO detail if recipients are involved.

Shared helpers:

- `state-labels.ts`
  - `PO_STATES` string constants.
  - `stateLabel(state)` with Q-A1 fix.
  - Optional `stateTone(state)` for badge variants.
- `format.ts`
  - `formatDateIT`
  - `formatDateTimeIT`
  - `formatMoneyEUR`
  - `parseMistraMoney`
  - `extractApproverList`
  - `isRequester(po,userEmail)`
  - `isApprover(po,userEmail)`
  - `downloadBlob(blob, filename)`
- `provider-refs.ts`
  - Provider ref category list.
  - `QUALIFICATION_REF` read-only rule.
  - `availableReferenceTypes` excludes `QUALIFICATION_REF`.
- `validation.ts`
  - Hand-rolled validators returning `{fieldErrors, formErrors}`.
  - Validate new PO, new provider, PO row, provider reference.

Acceptance:

- TypeScript compiles with placeholder consumers.
- No component duplicates raw endpoint strings outside `api/queries.ts`.

### Slice 4 — `/rda` My POs And Create Draft Modal

Goal: implement the requester list and draft creation flow.

Create files:

- `apps/rda/src/pages/RdaListPage.tsx`
- `apps/rda/src/pages/RdaListPage.module.css`
- `apps/rda/src/components/PoListTable.tsx`
- `apps/rda/src/components/PoListTable.module.css`
- `apps/rda/src/components/StateBadge.tsx`
- `apps/rda/src/components/ConfirmDialog.tsx`
- `apps/rda/src/components/NewPoModal.tsx`
- `apps/rda/src/components/NewPoModal.module.css`
- `apps/rda/src/components/BudgetSelect.tsx`
- `apps/rda/src/components/PaymentMethodSelect.tsx`
- `apps/rda/src/components/ProviderSelect.tsx`
- `apps/rda/src/components/NewProviderInlineForm.tsx`

RDA list page:

- Header:
  - `h1`: `Richieste di acquisto`
  - Primary button with plus icon: `Nuova richiesta`
- Data calls on load:
  - `useMyPOs`
  - `useBudgets`
  - provider list from `/fornitori/v1/provider?disable_pagination=true&page_number=1&usable=true`
  - `usePaymentMethods`
  - `usePaymentMethodDefault`
- Use `Skeleton` while loading.
- Error state: `Le richieste non sono disponibili in questo momento.`
- Empty state: `Nessuna richiesta trovata.`

`PoListTable` in requester mode:

- Columns:
  - edit icon, delete icon, view icon
  - `Stato`
  - `Approvatori`
  - `Richiedente`
  - `Data creazione`
  - `Numero PO`
  - `Fornitore`
  - `Progetto`
  - `Prezzo totale`
- Rules:
  - Edit enabled iff requester email equals current user email and state is `DRAFT`.
  - Delete enabled iff same.
  - View enabled iff state is not `DRAFT`.
  - DRAFT rows navigate via edit to `/rda/po/:id`.
  - Non-DRAFT rows navigate via view to `/rda/po/:id`.
- Delete:
  - Show confirm dialog.
  - On confirm call `DELETE /pos/{id}`.
  - Toast success/failure.
  - Refresh list.

Create modal:

- Modal title: `Nuova richiesta`.
- Single-screen form with sections:
  - RDA
  - Fornitore e pagamento
  - Nuovo fornitore, collapsible and hidden by default
- Fields:
  - Budget, required.
  - Tipo PO, required, default `STANDARD`, options `STANDARD`, `ECOMMERCE`.
  - Progetto, required, max 50.
  - Oggetto, required.
  - Fornitore, required, searchable.
  - Payment method, required.
  - Hidden defaults in submitted body: `reference_warehouse = MILANO`, `currency = EUR`.
- Budget selection:
  - Option value is `budget_id`, not a stringified object.
  - Lookup selected budget from a map.
  - Submission includes `cost_center` if selected budget has it; otherwise `budget_user_id`.
  - Enforce exactly one.
- Payment method selection:
  - Option set is union of selected provider default, CDLAN default, and `rda_available` methods.
  - Supplier default wins for initial selection.
  - CDLAN default wins when supplier has no default.
  - Never use literal `320`.
  - Show helper only when chosen method is not CDLAN default: `Il PO sarà sottoposto ad approvazione metodo pagamento.`
- Inline new provider:
  - Use `/api/fornitori/v1/provider` or the existing Fornitori create endpoint shape.
  - Validate fields listed in the final spec.
  - After create, refresh provider list and auto-select the new provider if the response has an id.
  - If `/fornitori` returns 403, show business-facing fallback about supplier data not being available and mention ops configuration only outside the UI/session notes.
- Submit:
  - Call `POST /api/rda/v1/pos`.
  - On success close modal, refresh list, navigate `/rda/po/:id`.

Design:

- Use one main table surface, not cards per row.
- Use icon buttons with tooltips for row actions.
- Use `@mrsmith/ui` buttons/icons/components where available.
- Keep form labels compact and required markers styled, not literal `(*)`.

Acceptance:

- User can load `/rda`.
- User can create a draft using existing provider and navigate to detail.
- Delete is unavailable for non-DRAFT or non-requester rows.
- No technical copy is visible.

### Slice 5 — `/rda/inbox/:kind`

Goal: implement one parameterized approval inbox page.

Create files:

- `apps/rda/src/pages/InboxPage.tsx`
- `apps/rda/src/pages/InboxPage.module.css`
- `apps/rda/src/lib/inbox.ts`

`inbox.ts` config:

```ts
level1-2:
  title: "Approvazioni I° / II° livello"
  role: "app_rda_approver_l1l2"
leasing:
  title: "Approvazioni Leasing"
  role: "app_rda_approver_afc"
no-leasing:
  title: "Approvazioni No-Leasing"
  role: "app_rda_approver_no_leasing"
payment-method:
  title: "Approvazioni Metodo Pagamento"
  role: "app_rda_approver_afc"
budget-increment:
  title: "Approvazioni Incremento Budget"
  role: "app_rda_approver_extra_budget"
```

Page behavior:

- Read `kind` from route params.
- Unknown kind redirects to `/rda` or shows `Pagina non disponibile`.
- If user lacks role, show a clean forbidden state:
  - Title: `Accesso riservato`
  - Body: `Questa lista è disponibile solo agli utenti abilitati.`
- Fetch `GET /api/rda/v1/pos/inbox/{kind}`.
- Table uses `PoListTable` in inbox mode:
  - `Gestisci` icon action only.
  - Columns: `Stato`, `Richiedente`, `Data creazione`, `Numero PO`, `Fornitore`, `Progetto`, `Prezzo totale`.
- Clicking `Gestisci` navigates to `/rda/po/:id`, preserving any `budget_increment_needed` in query string for budget-increment rows if the action later needs it.

Acceptance:

- All five kinds are routable.
- Missing-role UI is business-facing.
- Backend also rejects missing roles; frontend gating is only UX.

### Slice 6 — PO Detail Shell, Header, And Action Bar

Goal: implement the main workflow editor structure.

Create files:

- `apps/rda/src/pages/PoDetailPage.tsx`
- `apps/rda/src/pages/PoDetailPage.module.css`
- `apps/rda/src/components/ActionBar.tsx`
- `apps/rda/src/components/ActionBar.module.css`
- `apps/rda/src/components/PoHeaderForm.tsx`
- `apps/rda/src/components/PoHeaderForm.module.css`
- `apps/rda/src/components/RecipientsList.tsx`

Detail page layout:

- Top compact action bar.
- Header form surface.
- Main tabbed detail body.
- Comments side panel on desktop-width layouts.
- On narrower screens, comments stack below tabs; do not let text overlap.

Data:

- Fetch:
  - `usePODetail(poId)`
  - `usePermissions`
  - budgets
  - payment methods/default
  - provider detail for `po.provider.id` from `/fornitori`
  - comments
- Invalid id state: `Richiesta non valida`.
- Not found/error state: `Richiesta non disponibile`.

Action derivation:

- Compute:
  - `isDraft`
  - `isRequester`
  - `isApproverForPO`
  - `canSubmit`
  - `quoteRuleBlocked`
  - role booleans from `/me/permissions`
- `canSubmit`:
  - state `DRAFT`
  - requester
  - rows length > 0
  - if parsed total `>= 3000`, attachments length >= 2
- Show warning when quote rule blocks submit:
  - `Attenzione: importo superiore a 3.000 €. Aggiungi 2 preventivi.`

Action bar buttons:

- Always: `Chiudi` -> `/rda`.
- DRAFT requester:
  - `Aggiorna bozza PO` -> PATCH header.
  - `Manda PO in approvazione` -> confirm dialog, then PATCH header if dirty, then submit.
- L1/L2:
  - `Approva (Liv 1)` / `Rifiuta (Liv 1)` when state `PENDING_APPROVAL`, approval level 1, role, and current email in approvers.
  - `Approva (Liv 2)` / `Rifiuta (Liv 2)` for level 2.
- Payment:
  - `Approva metodo pagamento` / `Rifiuta metodo pagamento` when state `PENDING_APPROVAL_PAYMENT_METHOD` and AFC.
- Leasing:
  - `Approva leasing` / `Rifiuta leasing` when state `PENDING_LEASING` and AFC.
  - `Leasing creato` when state `PENDING_LEASING_ORDER_CREATION` and AFC.
- No leasing:
  - `Approva no leasing` / `Rifiuta no leasing` when state `PENDING_APPROVAL_NO_LEASING` and no-leasing role.
- Budget increment:
  - `Approva incremento budget` / `Rifiuta incremento budget` when state `PENDING_BUDGET_INCREMENT` and extra-budget role.
  - Send `increment_promise` from current row/query param when present.
- State-only:
  - `Invia ordine al fornitore` when state `PENDING_SEND`.
  - `Erogato e conforme` and `In contestazione` when state `PENDING_VERIFICATION`.
- PDF:
  - `Genera PDF` when state is not `DRAFT`.

After transitions:

- Approval/rejection actions navigate back to the relevant inbox.
- `send-to-provider` navigates back to `/rda`.
- Other transitions reload the detail.
- Toast success/failure with business copy.

Header form:

- Read-only banner:
  - `Ordine Numero: {code} del {date} — Stato Attuale: {stateLabel}`
  - Approver emails grouped by level.
- Editable fields:
  - Budget: DRAFT only.
  - Oggetto: DRAFT only.
  - Progetto: DRAFT only.
  - Fornitore: DRAFT only; on change immediately clears local selected recipients and on save sends `recipient_ids: []`.
  - Metodo pagamento: DRAFT or `PENDING_APPROVAL_PAYMENT_METHOD`.
  - Riferimento preventivo: DRAFT only.
  - Data preventivo: DRAFT only.
  - Notes fields are in tabs, not header.
- Payment-method update button:
  - Visible/enabled only state `PENDING_APPROVAL_PAYMENT_METHOD` and requester.
  - Calls dedicated PATCH `/payment-method`.
- Recipients summary:
  - If recipients exist, list name/email/phone.
  - If empty: `Se non viene selezionato alcun contatto, verrà utilizzato il referente di qualifica.`
  - No HTML injection.

Acceptance:

- Detail page opens in DRAFT and non-DRAFT modes.
- Action availability matches the final spec.
- Submit confirmation awaits PATCH and submit sequentially; do not recreate the Appsmith race.

### Slice 7 — Detail Tabs

Goal: implement attachments, rows, notes, and provider contacts.

Create files:

- `apps/rda/src/components/PoTabs.tsx`
- `apps/rda/src/components/PoTabs.module.css`
- `apps/rda/src/components/AttachmentsTab.tsx`
- `apps/rda/src/components/RowsTab.tsx`
- `apps/rda/src/components/RowModal.tsx`
- `apps/rda/src/components/NotesTab.tsx`
- `apps/rda/src/components/ProviderRefTable.tsx`
- `apps/rda/src/components/ProviderRefTable.module.css`

Tab labels:

- `Allegati`
- `Righe PO`
- `Note`
- `Contatti Fornitore`

Do not port hidden tab `Contatt`.

Attachments tab:

- Show reminder: `Per importi maggiori di 3.000 € sono necessari almeno 3 preventivi.`
- Upload enabled when state is `DRAFT` or `PENDING_VERIFICATION`.
- Upload accepts multiple files if feasible; loop files and call backend once per file.
- Backend derives type; frontend never sends `attachment_type`.
- Table columns:
  - File name
  - Type label: `Preventivo`, `Documento di trasporto`, `Altro`
  - Created date
  - Download icon
  - Delete icon, enabled only DRAFT requester
- Download uses `api.getBlob` and `downloadBlob`.
- Delete confirm before delete.
- Do not port motivazione fields.

Rows tab:

- Show total: parsed `total_price` formatted EUR.
- Add row icon enabled only DRAFT requester.
- Table columns:
  - Descrizione
  - Costo unitario / NRC
  - MRC
  - Q.tà
  - Tipo
  - Totale riga, using backend value if present; do not recompute as source of truth.
  - Edit icon enabled only DRAFT requester.
  - Delete icon enabled only DRAFT requester.

Row modal:

- Fields:
  - Type `good|service`
  - Product select from articles filtered by type
  - Description
  - Quantity
  - Good unit price
  - Service NRC
  - Service MRC
  - Initial subscription months
  - Recurrence months: 1, 3, 6, 12
  - Start at:
    - service: `activation_date`, `specific_date`
    - good: `activation_date`, `advance_payment`, `specific_date`
  - Start date if specific
  - Automatic renew
  - Cancellation advice if automatic renew
- Live preview:
  - service: `(MRC * qty * duration) + (NRC * qty)`
  - good: `unit_price * qty`
  - Mark as preview only visually; do not imply final backend total.
- Submit calls `POST /rows`.

Notes tab:

- Two textareas:
  - `Note fornitore` -> `note`
  - `Descrizione interna` -> `description`
- Editable only DRAFT requester.
- Save flows through the header PATCH.

Provider contacts tab:

- Data source: provider detail from `/api/fornitori/v1/provider/{id}`.
- Table columns:
  - Email
  - Nome
  - Cognome
  - Telefono
  - Tipo
  - Selected as recipient
- `QUALIFICATION_REF`:
  - Read-only.
  - Cannot be added as a new row.
  - Cannot be edited inline.
- New rows:
  - DRAFT requester only.
  - Reference type options exclude `QUALIFICATION_REF`.
  - Save through `/api/fornitori/v1/provider/{id}/reference`.
- Existing row edit:
  - DRAFT requester only and not `QUALIFICATION_REF`.
  - Save through `/api/fornitori/v1/provider/{id}/reference/{ref_id}`.
  - Preserve the final spec's asymmetric semantics if the existing Fornitori endpoint supports them. If not, adapt RDA UI so it does not accidentally clear email when the field is empty.
- Recipients selection:
  - Initial selected ids from `po.recipients`.
  - `Salva contatti selezionati` enabled only DRAFT requester.
  - PATCH PO with `recipient_ids`.
  - Empty selection is valid and means qualification ref fallback.
- Helper text:
  - `Seleziona i contatti a cui inviare l'ordine. Se non viene spuntato alcun contatto, verrà utilizzato il contatto di tipo qualifica.`

Acceptance:

- All four tabs work from the same PO detail response.
- No hidden/dead legacy widgets are present.
- Attachments use real multipart, not base64.
- Row edit is not available.

### Slice 8 — Comments And Mentions

Goal: implement comment thread parity without pretending mentions notify users.

Create files:

- `apps/rda/src/components/CommentsPanel.tsx`
- `apps/rda/src/components/CommentsPanel.module.css`
- `apps/rda/src/components/MentionInput.tsx`
- `apps/rda/src/lib/mentions.ts`

Behavior:

- Fetch comments via `GET /comments`.
- Normalize response:
  - Accept array or paginated `{items}`.
  - Text field can be `comment` or `comment_text`.
  - Replies are single-level if present.
- Render:
  - User initials/avatar circle.
  - User name/email.
  - Timestamp.
  - Comment body as text, not HTML.
  - Replies indented.
- Input:
  - Textarea.
  - Detect trailing `@token`.
  - Query `/api/rda/v1/users?search=token` when token is non-empty.
  - Selecting a user replaces trailing token with `@email `.
  - Keep selected user ids locally only if useful for future, but do not send them.
- Submit:
  - Send `{comment}` only.
  - Clear input and mention state.
  - Refresh comments.
- Empty state:
  - `Nessun commento presente.`

Acceptance:

- Posting a comment does not include `mentioned_user_ids`.
- Comment text is safely rendered.

### Slice 9 — Polish, Verification, And Signoff

Goal: make the app feel native to the mini-app family and verify the integration.

Design checks:

- Clean theme background matches `docs/UI-UX.md`.
- No hero banners.
- No KPI cards.
- No nested UI cards.
- Cards/surfaces use restrained radii and consistent spacing.
- Table text does not overflow icon/action columns.
- Button text fits at desktop and narrow widths.
- All icon-only buttons have `aria-label` and `title` or `Tooltip`.
- Loading, empty, error, forbidden, and confirm states are implemented.

Runtime checks:

- `pnpm --filter mrsmith-rda exec tsc --noEmit`
- `pnpm --filter mrsmith-rda build`
- `cd backend && go build ./cmd/server`
- `cd backend && go test ./...` may be run to protect existing behavior; do not add new test files unless approved.
- `pnpm dev:rda` with backend running.
- Check `/config` and `/api` proxy through Vite on port 5190.
- Check launcher href override points to `http://localhost:5190` in local split-server mode.
- Check production build base creates assets under `/apps/rda/`.

Manual workflow checks against a configured dev environment:

- Open `/apps/rda/` from launcher.
- Browse `/rda` list.
- Create a DRAFT PO.
- Open DRAFT detail.
- Edit header and save.
- Add row.
- Upload attachment in DRAFT; verify backend tags it as `quote`.
- Submit blocked when total >= 3000 and fewer than 2 attachments.
- Submit allowed after prerequisites.
- Open each inbox with suitable roles.
- Approve/reject L1/L2 with an email present in `approvers[]`.
- Verify user with role but not in `approvers[]` cannot approve L1/L2.
- Update payment method in `PENDING_APPROVAL_PAYMENT_METHOD` as requester.
- Approve payment method as AFC.
- Upload DDT in `PENDING_VERIFICATION`; verify backend tags it as `transport_document`.
- Confirm conformity; surface Mistra's DDT-required failure as business copy when missing.
- Download attachment and PDF.
- Post a comment.

Playwright/browser rule:

- Before running Playwright or browser checks, first check whether `make dev` or the relevant Vite dev server is already running and reuse it.
- Do not start a second server if a suitable one is active.

Post-gate:

- Run `portal-miniapp-ui-review` with screenshots listed in the UI review section.
- Fix any UI review blocker before signoff.

## Public API Contract Checklist

Use this as the implementation checklist for `backend/internal/rda`.

```text
GET    /api/rda/v1/me/permissions
GET    /api/rda/v1/budgets
GET    /api/rda/v1/payment-methods
GET    /api/rda/v1/payment-methods/default
GET    /api/rda/v1/articles?type=...
GET    /api/rda/v1/users?search=...

GET    /api/rda/v1/pos
GET    /api/rda/v1/pos/inbox/level1-2
GET    /api/rda/v1/pos/inbox/leasing
GET    /api/rda/v1/pos/inbox/no-leasing
GET    /api/rda/v1/pos/inbox/payment-method
GET    /api/rda/v1/pos/inbox/budget-increment

POST   /api/rda/v1/pos
GET    /api/rda/v1/pos/{id}
PATCH  /api/rda/v1/pos/{id}
DELETE /api/rda/v1/pos/{id}

POST   /api/rda/v1/pos/{id}/submit
POST   /api/rda/v1/pos/{id}/approve
POST   /api/rda/v1/pos/{id}/reject
POST   /api/rda/v1/pos/{id}/leasing/approve
POST   /api/rda/v1/pos/{id}/leasing/reject
POST   /api/rda/v1/pos/{id}/leasing/created
POST   /api/rda/v1/pos/{id}/no-leasing/approve
POST   /api/rda/v1/pos/{id}/payment-method/approve
PATCH  /api/rda/v1/pos/{id}/payment-method
POST   /api/rda/v1/pos/{id}/budget-increment/approve
POST   /api/rda/v1/pos/{id}/budget-increment/reject
POST   /api/rda/v1/pos/{id}/conformity/confirm
POST   /api/rda/v1/pos/{id}/conformity/reject
POST   /api/rda/v1/pos/{id}/send-to-provider
GET    /api/rda/v1/pos/{id}/pdf

POST   /api/rda/v1/pos/{id}/rows
DELETE /api/rda/v1/pos/{id}/rows/{rowId}

POST   /api/rda/v1/pos/{id}/attachments
GET    /api/rda/v1/pos/{id}/attachments/{aid}
DELETE /api/rda/v1/pos/{id}/attachments/{aid}

GET    /api/rda/v1/pos/{id}/comments
POST   /api/rda/v1/pos/{id}/comments
```

Provider calls stay outside this contract:

```text
GET    /api/fornitori/v1/provider?disable_pagination=true&page_number=1&usable=true
POST   /api/fornitori/v1/provider
GET    /api/fornitori/v1/provider/{id}
POST   /api/fornitori/v1/provider/{id}/reference
PUT    /api/fornitori/v1/provider/{id}/reference/{ref_id}
```

## Upstream Mistra Mapping

```text
GET    /api/rda/v1/pos
  -> GET /arak/rda/v1/po

GET    /api/rda/v1/pos/{id}
  -> GET /arak/rda/v1/po/{id}

POST   /api/rda/v1/pos
  -> POST /arak/rda/v1/po

PATCH  /api/rda/v1/pos/{id}
  -> PATCH /arak/rda/v1/po/{id}

DELETE /api/rda/v1/pos/{id}
  -> DELETE /arak/rda/v1/po/{id}

POST   /api/rda/v1/pos/{id}/rows
  -> POST /arak/rda/v1/po/{id}/row

DELETE /api/rda/v1/pos/{id}/rows/{rowId}
  -> DELETE /arak/rda/v1/po/{id}/row/{rowid}

POST   /api/rda/v1/pos/{id}/attachments
  -> POST /arak/rda/v1/po/{id}/attachment

GET    /api/rda/v1/pos/{id}/attachments/{aid}
  -> GET /arak/rda/v1/po/{po_id}/attachment/{attachment_id}/download

DELETE /api/rda/v1/pos/{id}/attachments/{aid}
  -> DELETE /arak/rda/v1/po/{po_id}/attachment/{attachment_id}

GET    /api/rda/v1/pos/{id}/comments
  -> GET /arak/rda/v1/po/{id}/comment

POST   /api/rda/v1/pos/{id}/comments
  -> POST /arak/rda/v1/po/{id}/comment

GET    /api/rda/v1/pos/{id}/pdf
  -> GET /arak/rda/v1/po/{id}/download

GET    /api/rda/v1/pos/inbox/level1-2
  -> GET /arak/rda/v1/po/pending-approval

GET    /api/rda/v1/pos/inbox/leasing
  -> GET /arak/rda/v1/po/pending-leasing

GET    /api/rda/v1/pos/inbox/no-leasing
  -> GET /arak/rda/v1/po/pending-approval-no-leasing

GET    /api/rda/v1/pos/inbox/payment-method
  -> GET /arak/rda/v1/po/pending-approval-payment-method

GET    /api/rda/v1/pos/inbox/budget-increment
  -> GET /arak/rda/v1/po-pending-budget-increment
```

## Business Rules Checklist

- Edit/delete generic PO data only requester + `DRAFT`.
- Row add/delete only requester + `DRAFT`.
- Attachment upload only `DRAFT` or `PENDING_VERIFICATION`.
- Attachment delete only requester + `DRAFT`.
- Attachment type is backend-derived:
  - `DRAFT` -> `quote`
  - otherwise -> `transport_document`
- Submit requires:
  - requester
  - `DRAFT`
  - at least one row
  - if total >= 3000 EUR, at least 2 attachments, counting all attachments per final spec
- Empty recipients means provider qualification ref fallback.
- `QUALIFICATION_REF` provider reference is read-only and cannot be manually added.
- Generic reject endpoint role depends on current state.
- L1/L2 approve/reject requires both role and current email in `approvers[]`.
- Payment-method update in `PENDING_APPROVAL_PAYMENT_METHOD` requires requester.
- Send-to-provider and conformity actions are state-only beyond base app access.
- Currency is EUR and not user-editable.
- Reference warehouse defaults to `MILANO`.
- CDLAN default payment method comes from DB, never from literal `320`.

## Deferred And Follow-Up Items

Record these in implementation notes and update `docs/TODO.md` only if the work uncovers new reusable facts:

- Keycloak group bundling: ensure RDA users also receive `app_fornitori_access`.
- Mistra OpenAPI should eventually add `recipients[]` and `approvers[]` to `rda-document-detail`.
- Mistra should eventually fix `total_price` formatting so frontend parsing can be simplified.
- Mistra should eventually add native row update support; until then BFF row edit remains a non-atomic replacement flow.
- If implementation proves comment creation needs a numeric user id, document the exact lookup and add it to `docs/IMPLEMENTATION-KNOWLEDGE.md`.

## Test Policy

The repository instruction says not to add tests unless approved by the user.

For this implementation:

- Do run existing build/typecheck/test commands as verification.
- Do not create new test files by default.
- If the user approves tests later, prioritize narrow backend tests for:
  - role mapping in `/me/permissions`
  - inbox role enforcement
  - submit prerequisites
  - generic reject state-to-role mapping
  - attachment auto-tag
  - patch body preserving explicit empty values

## Final Definition Of Done

- RDA appears in the launcher for `app_rda_access`.
- `/apps/rda/` deep links work in production build.
- Local split-server launch works on port `5190`.
- All public RDA endpoints derive caller email from auth claims.
- Browser never talks directly to Mistra or Arak Postgres.
- Provider flows reuse `/api/fornitori/v1`.
- The three app routes match the final spec.
- All listed workflow actions are available only under the final spec conditions.
- Dead Appsmith pages/widgets are absent.
- UI passes the mini-app review gates.
- Existing build/typecheck verification passes.

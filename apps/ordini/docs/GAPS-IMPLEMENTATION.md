# Ordini gap remediation plan

Date: 2026-05-23
Status: detailed remediation plan
Source implementation: `b1df0ff` (`feat(ordini): add new order management application`)
Primary gap source: `apps/ordini/docs/SCORE-REVIEWER.md`

## Goal

Close the confirmed Ordini implementation gaps before considering the app for UI post-gate and a later catalog status change from `test` to `ready`.

This plan addresses only confirmed implementation, contract, verification, and UI consistency gaps. It does not expand v1 scope.

## Inputs used

- `apps/ordini/docs/IMPL-ORDINI.md`
- `apps/ordini/docs/SCORE-REVIEWER.md`
- `docs/UI-UX.md`
- `docs/IMPLEMENTATION-PLANNING.md`
- `docs/IMPLEMENTATION-KNOWLEDGE.md`
- `.agents/skills/portal-miniapp-generator/references/archetypes.md`
- `.agents/skills/portal-miniapp-generator/references/review-gates.md`
- External reviews in `/home/sciacco/devel/varie/REVIEW*.md`
- Actual implementation under `apps/ordini/src` and `backend/internal/ordini`

## Non-goals

- Do not implement cancel request, lost-order handling, partial retry after send-to-ERP, server-side pagination, or data migration. These remain post-v1/deferred items already tracked in `docs/TODO.md`.
- Do not flip catalog status to `ready` in this remediation. Keep `test` until implementation fixes, approved contract tests, manual verification, and UI post-gate are complete.
- Do not replace the Ordini workflow with a wizard or dashboard.
- Do not add speculative tests outside the approved contract-test categories in `IMPL-ORDINI.md` section 19.
- Do not change business semantics for already-confirmed activation rows unless Appsmith/live behavior confirms that repeated activation must be forbidden.

## Current verified baseline

The following checks passed during review:

```bash
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm --filter mrsmith-ordini lint
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm --filter mrsmith-ordini build
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./internal/ordini ./internal/platform/applaunch ./internal/platform/config ./internal/platform/staticspa
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./...
```

Important baseline note: `backend/internal/ordini` currently reports `[no test files]`.

## Comparable apps audit

The remediation keeps Ordini inside the existing mini-app family. These screens were inspected for concrete patterns.

### RDA

Files:

- `apps/rda/src/pages/RdaListPage.tsx`
- `apps/rda/src/pages/PoDetailPage.tsx`

Patterns to reuse:

- Compact workspace header.
- Search, filters, and table as the main working surface.
- Explicit loading, empty, and error states.
- Detail page actions gated by role/state.
- Toast-driven mutation feedback.
- Business labels instead of implementation copy.
- Blob download flow for document actions.

Patterns to reject:

- RDA-specific inbox or multi-queue concepts.
- Comments/conversation features.
- Approval-step semantics not present in Ordini.
- Any wizard-like flow.

### AFC tools Ordini sales

Files:

- `apps/afc-tools/src/pages/OrdiniSalesPage.tsx`
- `apps/afc-tools/src/pages/OrdiniSalesDetailPage.tsx`

Patterns to reuse:

- Dense order table optimized for scanning.
- Client-side search, sort, and pagination.
- Date filters where they directly support order lookup.
- Clear order detail grouping.
- Status badges as domain state, not decoration.
- Row navigation that does not hide the primary data surface.

Patterns to reject:

- Read-only sales-order assumptions.
- KPI/summary strips.
- Technical diagnostic sections.
- Server-side filter semantics not planned for Ordini v1.

## Archetype decision

Primary archetype: `master_detail_crud`.

Ordini remains a list/detail CRUD-style workspace:

- compact page header
- toolbar with search/filter/sort/pagination controls
- primary orders table
- detail view with business tabs
- inline row editing and modal activation
- explicit loading, empty, error, and confirmation states

Allowed exceptions:

- The detail page may keep tabs because the order entity has multiple business sections.
- The send-to-ERP result panel may remain persistent because partial failure requires per-row visibility.
- PDF actions may remain grouped in the detail action area.

Rejected screen directions:

- no hero banner
- no KPI cards
- no dashboard shell
- no explanatory implementation panels
- no wizard

## Copy and metrics policy

User-facing copy should remain Italian business copy. It must describe the order, customer, PDF, ERP send, activation, or next business action.

Terms to review during implementation:

- `System ODV`: keep only if this is the accepted business term for users. If not, use a product-approved label.
- `Kickoff`: keep only if users recognize this document name. Otherwise rename to the approved business label while keeping the endpoint unchanged.
- Avoid UI text such as `server-side`, `inline update`, `record`, `datasource`, `widget`, `replica`, or implementation explanations.

Metrics are not a primary part of this app. Allowed count-like indicators:

- status badges
- tab counts
- row counts in section headers
- send-to-ERP row outcome counts

Do not add KPI/stat cards as part of this remediation.

## Gap remediation phases

### Phase 0 - Scope freeze and baseline

Files:

- `apps/ordini/docs/GAPS-IMPLEMENTATION.md`
- `docs/TODO.md`
- `apps/ordini/docs/IMPL-ORDINI.md`

Actions:

- Treat this document as the remediation backlog for confirmed gaps.
- Keep deferred v1 items in `docs/TODO.md`.
- Keep catalog status `test`.
- Re-run the current baseline checks before editing if the worktree has changed materially.

Acceptance criteria:

- Remediation scope is limited to the gaps listed here.
- No post-v1 feature is pulled into the fix set.

### Phase 1 - Backend contract and safety fixes

#### GAP-BE-01 - Return planned `db_failed` for database failures

Files:

- `backend/internal/ordini/handler.go`
- `backend/internal/platform/httputil/respond.go` only if a shared helper is genuinely needed
- `apps/ordini/src/lib/errors.ts`

Problem:

`backend/internal/ordini` uses `httputil.InternalError`, which returns `internal_server_error`. The Ordini plan requires sanitized client error code `db_failed` for ordinary database failures and `db_commit_failed` for commit failures.

Implementation:

- Change `Handler.dbFailure` to log the underlying error and return `httputil.Error(w, http.StatusInternalServerError, "db_failed")`.
- Preserve internal log details with `component=ordini`, `operation`, and any supplied identifiers.
- Keep existing explicit `db_commit_failed` responses where a transaction commit fails after local writes or gateway boundaries.
- Do not expose SQL text, driver errors, DSNs, table names, or raw gateway errors in the client payload.

Acceptance criteria:

- Every `h.dbFailure(...)` client response uses `{ "error": "db_failed" }`.
- Commit failures still use `{ "error": "db_commit_failed" }`.
- Frontend message map already has `db_failed` and `db_commit_failed`.

Verification:

- Add a contract test that forces an Ordini DB failure and asserts `db_failed`.
- Run targeted Go tests.

#### GAP-BE-02 - Complete request logging contract

Files:

- `backend/internal/ordini/handler.go`
- `backend/internal/ordini/workflow_send.go`
- `backend/internal/ordini/workflow_activate.go`
- `backend/internal/ordini/pdf.go`
- any store file with direct `h.logger.Warn/Error`

Problem:

The plan requires failure-path logging to include `component`, `operation`, `request_id`, relevant IDs, gateway path/status when applicable, and `duration_ms`. Current logs include some fields but not consistently request ID or duration.

Implementation:

- Add a small unexported logging helper on `Handler`, for example `logFailure(r, level, message, operation, start, attrs...)`.
- Extract request ID from the platform request context or request headers using the existing request ID middleware conventions.
- Capture `start := time.Now()` at the start of external workflows and gateway/PDF operations.
- Log `duration_ms` using elapsed milliseconds.
- For gateway calls, include `gw_path` and `upstream_status` where the called helper can provide them.
- Apply the helper to direct Ordini warnings/errors, especially:
  - send-to-ERP row failure
  - Arxivar upload warning after state transition
  - origin lookup warning
  - PDF proxy failures
  - activation gateway failure
  - DB failure helper

Acceptance criteria:

- All Ordini failure logs include `component=ordini`, `operation`, `request_id`, and `duration_ms`.
- `order_id` is present when the operation is order-scoped.
- `row_id` is present when the operation is row-scoped.
- `gw_path` and `upstream_status` are present for gateway failures when known.

Verification:

- Add focused tests around the logging helper if it can be tested without brittle slog output assertions.
- Otherwise verify by code review plus manual exercise of one DB failure and one gateway failure in development logs.

#### GAP-BE-03 - Harden date parsing for legacy zero datetimes

Files:

- `backend/internal/ordini/scanners.go`
- `backend/internal/ordini/scanners_test.go`

Problem:

`NullDate` treats `0000-00-00` as null but not `0000-00-00 00:00:00`. Legacy MySQL dumps can contain the datetime form.

Implementation:

- Treat all of these values as null:
  - empty string
  - `0000-00-00`
  - `0000-00-00 00:00:00`
- Keep current support for `2006-01-02`, RFC3339, and `2006-01-02 15:04:05`.
- Do not broaden parsing to ambiguous locale dates.

Acceptance criteria:

- Scanner returns `Valid=false` for both zero date variants.
- Valid legacy datetimes still marshal as `YYYY-MM-DD`.

Verification:

- Add scanner tests for zero date, zero datetime, standard date, RFC3339, and invalid input.

#### GAP-BE-04 - Validate header confirmation date server-side

Files:

- `backend/internal/ordini/scanners.go`
- `backend/internal/ordini/store_orders.go`
- `backend/internal/ordini/types.go` if a request-validation helper needs a typed result
- `apps/ordini/src/lib/errors.ts`

Problem:

`handlePatchOrderHeader` passes `confirmation_date` through `dateOrNil`. The client uses an `<input type="date">`, but the backend contract should not trust client formatting.

Implementation:

- Replace `dateOrNil` usage for header confirmation date with a validation helper.
- Accept blank as `nil`.
- Accept only `YYYY-MM-DD`.
- Return a stable client error code for malformed input. Recommended code: `invalid_confirmation_date`.
- Add `invalid_confirmation_date` to the frontend error map with a business message.
- Keep `missing_confirmation_date` for send-to-ERP precondition failures.

Acceptance criteria:

- Blank confirmation date still clears/stores null.
- Valid `YYYY-MM-DD` stores the normalized date string.
- Malformed dates return HTTP 422 with `invalid_confirmation_date`.
- Send-to-ERP still returns `missing_confirmation_date` when no confirmation date is present.

Verification:

- Add backend tests for valid, blank, and invalid confirmation date payloads.

#### GAP-BE-05 - Add SQL-level state guard to referents update

Files:

- `backend/internal/ordini/store_orders.go`

Problem:

`handlePatchReferents` checks the order state before updating but the SQL update does not include a state predicate. A concurrent state change can permit an update after the pre-check.

Implementation:

- Add `AND cdlan_stato IN ('BOZZA', 'INVIATO')` to the referents `UPDATE`.
- Check `RowsAffected`.
- Return `wrong_state` with HTTP 409 when no row is affected after the pre-check.
- Preserve `order_not_found` for initial load miss.

Acceptance criteria:

- Referents can be updated in `BOZZA` and `INVIATO`.
- Referents cannot be updated in other states even if the state changes between load and update.

Verification:

- Add a backend contract test for the state guard, using a database test double or SQL expectation.

#### GAP-BE-06 - Replace brittle gateway error substring matching

Files:

- `backend/internal/ordini/gateway.go`
- `backend/internal/ordini/workflow_send.go`

Problem:

`sanitizeGatewayError` currently uses `strings.Contains(err.Error(), "precondition_missing")`.

Implementation:

- Introduce a small typed error or sentinel for known gateway business errors.
- Have `gatewaySendToERP` wrap known gateway response codes with that typed error.
- Have `sanitizeGatewayError` use `errors.Is` or `errors.As`.
- Keep fallback as `gateway_error`.

Acceptance criteria:

- Known gateway precondition failures still map to `precondition_missing`.
- Unknown gateway failures map to `gateway_error`.
- No code depends on string matching against formatted error text.

Verification:

- Add unit tests for `sanitizeGatewayError`.

#### GAP-BE-07 - Decide and harden activation repeat/zero-quantity behavior

Files:

- `backend/internal/ordini/workflow_activate.go`
- `apps/ordini/src/lib/permissions.ts`
- `apps/ordini/src/components/RigheTab.tsx`

Problem:

`canOpenActivationModal` does not block rows already confirmed or rows with zero quantity. The original plan's Q2 count rule treats confirmed rows, canceled rows, and zero-quantity rows as already satisfied for auto-ATTIVO. The plan does not explicitly forbid opening activation for already confirmed rows, so this is hardening unless live/Appsmith behavior proves otherwise.

Implementation decision:

- For zero-quantity rows: block activation in frontend and backend because they are counted as satisfied and should not need gateway activation.
- For canceled rows: keep blocking activation in frontend and backend.
- For already-confirmed rows: verify Appsmith/live behavior before changing backend semantics.
  - If Appsmith allows correcting activation dates, keep backend permissive and rename the button/copy if needed.
  - If Appsmith treats confirmed activation as final, add backend rejection with `precondition_missing` or a new specific code, and disable the frontend action.

Minimum safe implementation:

- Update `canOpenActivationModal` to disable the action when `row.data_annullamento != null` or `row.cdlan_qta === 0`.
- Add backend checks for canceled and zero-quantity rows before local update/gateway call.
- Defer the already-confirmed rejection until business behavior is confirmed.

Acceptance criteria:

- Users are not offered activation for rows that are canceled or quantity zero.
- Backend rejects activation for canceled or zero-quantity rows even if called directly.
- Already-confirmed row behavior is documented in the implementation notes or TODO if left permissive.

Verification:

- Add permission tests for frontend helper if the app already has a suitable test setup; otherwise verify manually.
- Add backend activation precondition test if covered by the approved activation contract tests.

#### GAP-BE-08 - Remove unused backend helper

Files:

- `backend/internal/ordini/scanners.go`

Problem:

`ptrStringOrNil` is unused.

Implementation:

- Delete the helper if no upcoming remediation uses it.

Acceptance criteria:

- `rg "ptrStringOrNil"` returns no references.
- Go tests/build still pass.

### Phase 2 - Approved backend contract tests

The tests in this phase are already approved by `IMPL-ORDINI.md` section 19 because they protect critical business and migration rules.

Recommended test structure:

- Use standard-library tests where pure helpers can cover the rule.
- Prefer small refactors toward unexported interfaces only when they reduce fragile SQL setup.
- If SQL transaction/workflow expectations become impractical with only standard library tools, add a narrowly scoped test dependency such as `github.com/DATA-DOG/go-sqlmock` in `backend/go.mod`.
- Do not add broad integration tests that require live Vodka, Alyante, Mistra, or Arak services.

#### GAP-TEST-01 - `CheckConfirmRows` Q2 rule

Files:

- `backend/internal/ordini/workflow_activate_test.go`
- `backend/internal/ordini/workflow_activate.go`

Implementation:

- Extract the confirmed-row count SQL or count operation behind a small helper if needed.
- Test that `data_annullamento IS NOT NULL` rows count as satisfied.
- Test that `confirm_data_attivazione = 1` rows count as satisfied.
- Test that `cdlan_qta = 0` rows count as satisfied.

Acceptance criteria:

- Auto-ATTIVO counting preserves the Q2 fix.

#### GAP-TEST-02 - `sendToErp` partial failure does not transition Vodka

Files:

- `backend/internal/ordini/workflow_send_test.go`
- `backend/internal/ordini/workflow_send.go`

Implementation:

- Simulate at least one row gateway failure and at least one row success.
- Assert response has row outcomes, `stateTransitioned=false`, `arxivarUploaded=false`.
- Assert no `UPDATE orders SET cdlan_stato='INVIATO'` occurs.
- Assert no Arxivar upload occurs.

Acceptance criteria:

- Partial failure remains visible and leaves the local order in `BOZZA`.

#### GAP-TEST-03 - `sendToErp` full success transitions and attempts Arxivar

Files:

- `backend/internal/ordini/workflow_send_test.go`

Implementation:

- Simulate all row gateway sends succeeding.
- Assert local state update to `INVIATO` and `cdlan_evaso = 1`.
- Assert Arxivar upload is attempted after state transition.
- Assert success response has `stateTransitioned=true` and `arxivarUploaded=true`.

Acceptance criteria:

- Full success follows the planned state transition order.

#### GAP-TEST-04 - Arxivar failure after state flip returns warning

Files:

- `backend/internal/ordini/workflow_send_test.go`
- `apps/ordini/src/components/SendToErpResultPanel.tsx`

Implementation:

- Simulate successful row sends and successful local state transition.
- Simulate Arxivar upload failure.
- Assert response has `stateTransitioned=true`, `arxivarUploaded=false`, and `warning="arxivar_upload_failed"`.

Acceptance criteria:

- Local state is not rolled back after Arxivar failure.
- Client can display the persistent warning panel.

#### GAP-TEST-05 - C2 customer dual-write

Files:

- `backend/internal/ordini/store_orders_test.go`
- `backend/internal/ordini/store_orders.go`

Implementation:

- Test header save writes both `cdlan_cliente_id` and `cdlan_cliente`.
- Test customer lookup failure does not write partial data.
- Test BOZZA state guard remains enforced.

Acceptance criteria:

- Header save preserves both the display string and numeric Alyante ID.

#### GAP-TEST-06 - Row ownership check

Files:

- `backend/internal/ordini/store_rows_test.go`
- `backend/internal/ordini/workflow_activate_test.go`

Implementation:

- Test serial-number update rejects a row that does not belong to the order.
- Test technical-notes update rejects a row that does not belong to the order.
- Test activation rejects a row that does not belong to the order.

Acceptance criteria:

- Row-scoped mutations require `orders_rows.orders_id = :orderID`.

#### GAP-TEST-07 - PDF normalization base64/raw

Files:

- `backend/internal/ordini/pdf_test.go`
- `backend/internal/ordini/pdf.go`

Implementation:

- Test raw `%PDF` payload is accepted.
- Test base64-encoded PDF payload is decoded and accepted.
- Test malformed gateway body returns `gw_pdf_malformed`.

Acceptance criteria:

- PDF proxy behavior handles both known gateway shapes without exposing raw gateway errors.

#### GAP-TEST-08 - Permission gates backend

Files:

- `backend/internal/ordini/permissions_test.go`
- `backend/internal/ordini/permissions.go`
- route handlers as needed

Implementation:

- Test base access role can read orders.
- Test non-CR users cannot call elevated actions.
- Test CR-only handlers return `role_insufficient` or forbidden response consistently.
- Test state gates return `wrong_state`.

Acceptance criteria:

- Backend remains authoritative for permissions and state.

#### GAP-TEST-09 - Origin resolver

Files:

- `backend/internal/ordini/store_origin_test.go`
- `backend/internal/ordini/store_origin.go`

Implementation:

- Test resolver path `orders.legacy_orders -> quotes.quote.quote_number`.
- Test missing origin returns null/empty origin without failing the order response if that is current intended behavior.
- Test database failure is logged and does not break the detail endpoint unless the function contract says otherwise.

Acceptance criteria:

- Origin metadata remains compatible with the Appsmith migration rule.

### Phase 3 - Frontend contract and UI consistency fixes

#### GAP-FE-01 - Map all backend error codes

Files:

- `apps/ordini/src/lib/errors.ts`

Problem:

Backend can return `invalid_activation_date`, but the frontend map does not include it. Phase 1 may also add `invalid_confirmation_date`.

Implementation:

- Add `invalid_activation_date` with a business message.
- Add `invalid_confirmation_date` if Phase 1 adds that backend code.
- Audit current backend error codes with `rg "httputil.Error\\(" backend/internal/ordini` and ensure user-facing mappings exist for codes the UI can surface.

Acceptance criteria:

- No expected Ordini backend code falls through to a generic fallback where a business-specific message is required.

Verification:

- Frontend lint/build.
- Manual mutation failure checks for activation and header date.

#### GAP-FE-02 - Restore or explicitly justify the 15-column list contract

Files:

- `apps/ordini/src/components/OrdersTable.tsx`
- `apps/ordini/src/api/types.ts`
- `apps/ordini/src/lib/formatters.ts`
- `apps/ordini/src/pages/OrderListPage.module.css`

Problem:

The plan requested a 15-column list table. The implementation renders 11 visible columns.

Implementation:

- Add a desktop column set that reaches the planned 15 business fields while keeping scanability.
- Recommended visible columns:
  - `Codice ordine`
  - `System ODV` or approved label
  - `Ragione sociale`
  - `Stato`
  - `Data proposta`
  - `Tipo documento`
  - `Tipo proposta`
  - `Tipo servizi`
  - `Conferma`
  - `Evaso`
  - `Dal CP`
  - `Sostituisce`
  - `Lingua`
  - `Documento`
  - `Azioni`
- Keep horizontal scroll for dense desktop data.
- On narrow screens, hide only the lowest-priority supplementary columns with CSS and keep primary order/customer/state/action columns available.
- If product/design decides 11 columns is better, document that as an explicit post-gate exception with rationale. The default remediation path is to implement the 15-column contract.

Acceptance criteria:

- Desktop populated list exposes the planned 15 business fields.
- Narrow layout does not overlap text or controls.
- Search, sort, pagination, and row open behavior are preserved.

Verification:

- Frontend lint/build.
- Playwright or browser screenshots for populated desktop and narrow viewport before UI post-gate.

#### GAP-FE-03 - Add table row accent and active interaction patterns

Files:

- `apps/ordini/src/pages/OrderListPage.module.css`
- `apps/ordini/src/pages/OrderDetailPage.module.css`
- `apps/ordini/src/components/OrdersTable.tsx`
- `apps/ordini/src/components/RigheTab.tsx`
- `apps/ordini/src/components/TechnicalNotesTab.tsx`

Problem:

`docs/UI-UX.md` expects the mini-app table family to use row accent bars and active scale interaction patterns. Current Ordini tables do not implement them.

Implementation:

- Add a subtle left accent bar on hover/focus/active rows.
- Add active scale only where it does not disturb dense table alignment.
- Use CSS custom properties or existing design tokens for accent and transition values.
- Preserve keyboard focus visibility for row actions and sortable headers.

Acceptance criteria:

- Orders table and detail tables visually match the mini-app table family.
- Interaction states do not resize rows or shift columns.
- No text overlaps at desktop or narrow viewport.

Verification:

- Browser check with populated list and detail rows.
- UI post-gate screenshots.

#### GAP-FE-04 - Tighten CSS token discipline

Files:

- `apps/ordini/src/styles/global.css`
- `apps/ordini/src/pages/OrderListPage.module.css`
- `apps/ordini/src/pages/OrderDetailPage.module.css`

Problem:

CSS is mostly token-based but still contains raw values such as `#fff`, `rgba(16, 185, 129, 0.25)`, and many pixel literals. Some global background literals are acceptable because they follow the existing app background recipe, but avoid unnecessary raw color drift.

Implementation:

- Replace raw colors with existing tokens where tokens exist.
- Keep approved global background recipes only when they match `docs/UI-UX.md` and surrounding mini-apps.
- Replace `#fff` with a surface/background token.
- Replace raw success border colors with success/accent tokens or `color-mix` from tokenized colors.
- Leave structural pixel values where the design system has no token and the value is layout-specific.
- Do not turn every layout value into a new token.

Acceptance criteria:

- `rg "#fff|rgba\\(16, 185, 129" apps/ordini/src` returns no matches unless an explicit comment justifies it.
- Color usage is token-aligned.
- Visual appearance remains consistent with comparable mini-apps.

Verification:

- Frontend lint/build.
- Visual browser check.

#### GAP-FE-05 - Make send-to-ERP warning handling code-aware

Files:

- `apps/ordini/src/components/SendToErpResultPanel.tsx`
- `apps/ordini/src/api/types.ts`
- `apps/ordini/src/lib/errors.ts` if shared mapping is useful

Problem:

The panel shows a fixed warning for any `result.warning`. Currently only `arxivar_upload_failed` exists, but code-aware rendering prevents future ambiguous warnings.

Implementation:

- Switch on `result.warning`.
- Render the existing Arxivar warning for `arxivar_upload_failed`.
- Render a generic business fallback for unknown warning codes.
- Do not expose raw technical codes in primary user copy.

Acceptance criteria:

- Known warning displays specific copy.
- Unknown warning displays safe generic copy.

Verification:

- Component-level code review or manual state injection.
- Frontend lint/build.

#### GAP-FE-06 - Split `formatDurRin` semantics from billing frequency

Files:

- `apps/ordini/src/lib/formatters.ts`
- `apps/ordini/src/components/InfoTab.tsx`

Problem:

`formatDurRin` is effectively a wrapper around `formatFatturazione`, while the Appsmith migration notes distinguish duration-renewal code handling from invoice-frequency code handling.

Implementation:

- Keep `formatFatturazione` accepting the known billing-frequency codes, including the documented drift where needed.
- Implement `formatDurRin` as its own mapping for renewal duration.
- Use `Quadrimestrale` for `cdlan_dur_rin = 4` if that matches the Appsmith rule captured in TODO/knowledge.
- Do not guess undocumented values. For unknown values, show the raw value using existing safe fallback style.

Acceptance criteria:

- `formatDurRin` no longer delegates blindly to `formatFatturazione`.
- Known Appsmith renewal duration values display correct business labels.

Verification:

- Frontend lint/build.
- Manual detail check on orders with `cdlan_dur_rin`.

#### GAP-FE-07 - Reset activation modal state when row changes

Files:

- `apps/ordini/src/components/ActivationModal.tsx`
- `apps/ordini/src/pages/OrderDetailPage.tsx`

Problem:

The modal resets its date on close. Resetting when the target row changes makes it robust if the selected row changes while the modal component remains mounted.

Implementation:

- Reset date/error state when `row?.id` changes.
- Keep current close reset.

Acceptance criteria:

- Opening activation on a second row does not retain stale date/error state from a previous row.

Verification:

- Manual detail interaction.
- Frontend lint/build.

#### GAP-FE-08 - Minor frontend cleanup

Files:

- `apps/ordini/src/lib/permissions.ts`
- `apps/ordini/src/hooks/useOptionalAuth.ts`

Implementation:

- Inline or remove `canEditTechnicalNotes` if it remains an unconditional `true` and adds no domain clarity.
- Keep `useOptionalAuth` only if rendering the app outside an auth provider is an intentional local/dev behavior. Otherwise simplify it in a separate low-risk cleanup.

Acceptance criteria:

- No unused or misleading frontend helpers remain.
- Bootstrap fatal error behavior remains unchanged.

### Phase 4 - Gateway and runtime verification

Files:

- `backend/internal/ordini/gateway.go`
- `backend/internal/ordini/workflow_send.go`
- `backend/internal/ordini/workflow_activate.go`
- `docs/TODO.md` if new reusable runtime findings emerge

Actions:

- Verify Alyante/MSSQL named parameter behavior for `@p1` against the real driver path if a live/staging connection is available.
- Verify Arak gateway error payload shapes for known business errors.
- Verify PDF gateway payload shapes for kickoff, activation form, order PDF, and signed PDF.
- If runtime behavior reveals a reusable integration rule, update `docs/IMPLEMENTATION-KNOWLEDGE.md`.

Acceptance criteria:

- No unverified driver/gateway assumption blocks v1 signoff.
- Any unresolved runtime dependency remains documented as a deployment/manual verification item, not hidden in code.

### Phase 5 - Manual verification and UI post-gate

Before browser checks:

- Check whether `make dev` or the relevant Vite server is already running.
- Reuse an existing suitable URL.
- Start a second dev/preview server only if no suitable server is active.

Manual verification checklist from `IMPL-ORDINI.md`:

- Launcher tile visible with correct role.
- Home list loads.
- Search/sort/pagination client-side works.
- Detail direct URL refresh works.
- Base user can read, edit technical notes, and edit serial only in `BOZZA`.
- Non-CR user cannot see/use elevated actions.
- CR can save Info in `BOZZA`.
- CR can save Referents in `BOZZA`/`INVIATO`.
- `INVIA in ERP` full success changes state to `INVIATO`.
- Partial failure shows per-row outcome and leaves `BOZZA`.
- Activation of the final required row changes order to `ATTIVO`.
- PDFs download with Bearer auth and correct filenames.
- `cdlan_cliente_id` is written and verified after header save.
- Row ownership check blocks mutation of rows from other orders.

UI post-gate screenshots/artifacts:

- populated desktop list
- populated desktop detail with at least one editable tab
- send-to-ERP partial failure panel
- activation modal
- empty state
- error state
- narrow/mobile viewport

UI gate expectations:

- Comparable apps gate: cite RDA and AFC files above.
- Archetype gate: `master_detail_crud`.
- Copy gate: business-user-only UI copy.
- Metrics gate: no new KPI/stat cards.
- Style gate: mini-app workspace family, no landing/hero/dashboard treatment.
- Repo-fit gate: route/base/API/roles/Vite/static deployment unchanged.
- Exception gate: list any explicit deviation, especially if the 15-column table contract is intentionally narrowed.

### Phase 6 - Final verification commands

Run after implementation:

```bash
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm --filter mrsmith-ordini lint
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm --filter mrsmith-ordini build
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./internal/ordini ./internal/platform/applaunch ./internal/platform/config ./internal/platform/staticspa
docker run --rm --network host --user 1000:1000 -e GOCACHE=/tmp/go-build -e GOMODCACHE=/tmp/gomod -e GOPATH=/tmp/go -v /home/sciacco/devel/mrsmith:/repo -w /repo/backend golang:1.26.1 go test ./...
```

Optional broader frontend check if time/runtime permits:

```bash
docker run --rm --network host --user 1000:1000 -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack -v /home/sciacco/devel/mrsmith:/repo -w /repo node:20-slim corepack pnpm -r --if-present build
```

## Detailed task matrix

| ID | Priority | Area | Files | Action | Acceptance criteria |
| --- | --- | --- | --- | --- | --- |
| GAP-BE-01 | P0 | Backend error contract | `handler.go`, `errors.ts` | Return `db_failed` for Ordini DB failures | DB failures no longer return `internal_server_error` |
| GAP-BE-02 | P0 | Observability | `handler.go`, workflows, PDF | Add request ID and duration to failure logs | Failure logs match plan fields |
| GAP-BE-03 | P1 | Legacy data | `scanners.go` | Treat zero datetime as null | `0000-00-00 00:00:00` scans as invalid/null |
| GAP-BE-04 | P0 | Backend validation | `store_orders.go`, `errors.ts` | Validate confirmation date | Bad dates return stable 422 code |
| GAP-BE-05 | P0 | Race safety | `store_orders.go` | Add referents SQL state guard | Concurrent state change cannot bypass state rule |
| GAP-BE-06 | P1 | Gateway errors | `gateway.go`, `workflow_send.go` | Replace substring matching | Known business errors use typed mapping |
| GAP-BE-07 | P1 | Activation safety | `workflow_activate.go`, `permissions.ts` | Block canceled/zero-quantity activation; decide confirmed behavior | UI/backend agree on non-actionable rows |
| GAP-BE-08 | P2 | Cleanup | `scanners.go` | Remove unused helper | No unused helper remains |
| GAP-TEST-01 | P0 | Tests | `workflow_activate_test.go` | Test Q2 confirm count | Canceled and zero-qty rows count as satisfied |
| GAP-TEST-02 | P0 | Tests | `workflow_send_test.go` | Test partial failure | Vodka state does not transition |
| GAP-TEST-03 | P0 | Tests | `workflow_send_test.go` | Test full success | Vodka transitions and Arxivar attempted |
| GAP-TEST-04 | P0 | Tests | `workflow_send_test.go` | Test Arxivar warning | Warning returned after state flip |
| GAP-TEST-05 | P0 | Tests | `store_orders_test.go` | Test C2 dual-write | Customer name and ID both written |
| GAP-TEST-06 | P0 | Tests | row/workflow tests | Test row ownership | Cross-order row mutation blocked |
| GAP-TEST-07 | P0 | Tests | `pdf_test.go` | Test PDF normalization | Raw/base64 PDF accepted; malformed rejected |
| GAP-TEST-08 | P0 | Tests | `permissions_test.go` | Test backend role/state gates | Backend is authoritative |
| GAP-TEST-09 | P1 | Tests | `store_origin_test.go` | Test origin resolver | Legacy quote origin mapping preserved |
| GAP-FE-01 | P0 | Frontend error UX | `errors.ts` | Add missing mappings | Backend codes show business messages |
| GAP-FE-02 | P1 | List UI | `OrdersTable.tsx`, CSS | Restore 15 columns or document exception | Desktop list meets plan or has explicit exception |
| GAP-FE-03 | P1 | UI style | CSS, table components | Add row accent/active states | Tables match mini-app family |
| GAP-FE-04 | P1 | UI tokens | CSS | Replace avoidable raw colors | Token discipline improves without visual drift |
| GAP-FE-05 | P2 | Warning UX | `SendToErpResultPanel.tsx` | Switch on warning code | Known/unknown warnings handled safely |
| GAP-FE-06 | P2 | Formatting | `formatters.ts` | Split renewal duration formatter | Renewal and billing labels do not share accidental semantics |
| GAP-FE-07 | P2 | Modal state | `ActivationModal.tsx` | Reset on row change | No stale activation form state |
| GAP-FE-08 | P2 | Cleanup | `permissions.ts`, `useOptionalAuth.ts` | Remove misleading helpers if not intentional | No misleading unused abstractions |
| GAP-RUN-01 | P1 | Runtime | gateway/DB paths | Verify Alyante/GW/PDF assumptions | Runtime assumptions are verified or documented |
| GAP-UX-01 | P0 | Signoff | app screens | Run manual checklist and UI post-gate | App remains `test` until gates pass |

## Suggested implementation order

1. Backend error/date/race fixes: GAP-BE-01, GAP-BE-03, GAP-BE-04, GAP-BE-05.
2. Test scaffolding and approved contract tests: GAP-TEST-01 through GAP-TEST-09.
3. Gateway/logging hardening: GAP-BE-02 and GAP-BE-06.
4. Activation row hardening: GAP-BE-07 after confirmed behavior for already-confirmed rows.
5. Frontend contract fixes: GAP-FE-01, GAP-FE-05, GAP-FE-07.
6. UI consistency fixes: GAP-FE-02, GAP-FE-03, GAP-FE-04, GAP-FE-06.
7. Cleanup: GAP-BE-08 and GAP-FE-08.
8. Runtime/manual verification and UI post-gate.

Rationale:

- Error/date/race fixes are low-level contracts that tests should lock in.
- Approved contract tests should land before or with workflow changes so they protect business behavior.
- UI changes should follow backend error-code stabilization to avoid repeated copy/mapping churn.
- Visual post-gate should happen after all UI-affecting fixes are in place.

## Risk notes

- `sendToErp` and activation workflows cross local Vodka writes and external gateway calls. Do not change operation order casually. Preserve documented partial-failure behavior.
- Adding SQL mocks can make tests brittle. Prefer extracting small pure helpers or narrow interfaces when that makes tests clearer.
- Tightening already-confirmed activation behavior can change live user workflows. Verify before enforcing.
- Expanding the list table to 15 columns can reduce scanability. Use horizontal scroll and responsive priority columns, and document an exception if product/design chooses a narrower table.
- Logging improvements must not leak customer data, SQL, DSNs, or raw upstream error bodies.

## Definition of done

- All P0 and P1 gaps in the task matrix are resolved or explicitly documented as accepted exceptions.
- All nine approved Ordini contract-test categories exist and pass.
- Frontend lint and build pass.
- Targeted and full backend Go tests pass.
- Manual verification checklist is completed against a suitable environment.
- UI post-gate passes with screenshots for populated, empty, error, modal, and narrow states.
- Catalog status remains `test` unless a separate final signoff explicitly approves `ready`.

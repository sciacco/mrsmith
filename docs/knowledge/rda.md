# RDA

Knowledge entries specific to `apps/rda`.
Part of the Implementation Knowledge Handbook — see [docs/IMPLEMENTATION-KNOWLEDGE.md](../IMPLEMENTATION-KNOWLEDGE.md) for the index, entry format, and placement rules.

### RDA New Supplier Requests Must Use Provider Draft Create

- Context: `apps/rda` supplier creation from `/rda/new` and any RDA inline new-provider form.
- Discovery: Mistra `POST /arak/provider-qualification/v1/provider` follows the full provider-create schema and requires `default_payment_method`, while the RDA flow only requests a supplier census draft. The legacy RDA datasource used `POST /arak/provider-qualification/v1/provider/draft`.
- Practical rule: RDA must create new suppliers through the RDA-owned `POST /api/rda/v1/providers/draft`, which proxies Mistra draft create, not the full provider create. Keep full provider create for the Fornitori app where users can manage complete provider records. RDA users should not need `app_fornitori_access` for supplier lookup, supplier draft creation, or PO recipient contact management.
- Evidence: `docs/mistra-dist.yaml` schemas `provider-new` and `provider-draft-new`; `docs/arak_schema.json` functions `provider_new` and `provider_draft_new`; RDA audit datasource `nuovoFornitore`.
- Used by: `apps/rda` `/rda/new` supplier request modal and legacy inline new-provider form.
- Open questions: none.

### RDA Approval Inbox Actionability Is State-Gated

- Context: `apps/rda` dashboard, L1/L2 approval inbox, and any future RDA inbox aggregation.
- Discovery: Mistra `GET /arak/rda/v1/po/pending-approval` can surface intermediate provider-qualification waits such as `PENDING_APPROVAL_PROVIDER`. Those rows can include approver metadata, but the RDA detail action bar only permits L1/L2 approval when `state == PENDING_APPROVAL`.
- Practical rule: treat upstream inbox membership as visibility, not sufficient actionability. Add an inbox action context only when the PO state matches the action handled by that inbox; keep `PENDING_APPROVAL_PROVIDER` visible for tracking but out of "Da fare" and out of actionable approval inbox rows until Mistra advances it to `PENDING_APPROVAL`.
- Evidence: RDA dashboard actionability predicate in `apps/rda/src/lib/rda-dashboard.ts`, detail action guard in `apps/rda/src/components/ActionBar.tsx`, and Mistra pending-approval endpoint contract.
- Used by: `apps/rda` dashboard and approver inbox pages.
- Open questions: none.

### RDA Payment Method Standard Rule

- Context: `apps/rda` PO create/edit flows and backend `POST/PATCH /api/rda/v1/pos`.
- Discovery: the standard payment methods for an RDA PO are the selected supplier default and the CDLAN default from `provider_qualifications.payment_method_default_cdlan`. Other payment methods are valid only when the catalog row exists and `provider_qualifications.payment_method.rda_available` is true.
- Practical rule: show supplier default first, CDLAN default second, then RDA-available methods. Warn users only when the selected method is neither supplier default nor CDLAN default. Backend create/patch must validate the effective payment code against `provider_qualifications.payment_method`, allowing a non-RDA method only when it matches the selected supplier default.
- Evidence: legacy RDA audit payment option rule, `provider_qualifications.payment_method`, `provider_qualifications.payment_method_default_cdlan`, and backend validation in `backend/internal/rda`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, and RDA backend PO create/patch validation.
- Open questions: none.

### RDA Currency Is A PO-Level Display Contract

- Context: `apps/rda` PO create/edit flows, PO lists, inboxes, row tables, and attachment threshold messaging.
- Discovery: RDA currency belongs to the PO header (`currency` on Mistra RDA create/detail/preview/patch), not to rows. It selects the displayed money symbol for PO and row amounts; it does not convert values, recalculate totals, or change row payloads.
- Practical rule: persist only `EUR`, `USD`, or `GBP` at PO level, default existing/missing values to `EUR`, and pass the PO currency into all money formatting. Keep numeric thresholds, row economics, and Mistra row create/replace payloads unchanged.
- Evidence: `docs/mistra-dist.yaml` RDA `currency` fields; implementation in `apps/rda/src/lib/format.ts`, `apps/rda/src/lib/po-payload.ts`, and `backend/internal/rda`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, PO lists, inboxes, row composer, and RDA backend PO create/patch.
- Open questions: none.

### RDA Attachment Type Is User-Selected At Upload

- Context: `apps/rda` PO attachment uploads and draft submit validation.
- Discovery: Mistra `POST /arak/rda/v1/po/{id}/attachment` requires multipart `attachment_type` with enum `quote`, `transport_document`, or `other`. RDA users need to choose that type during upload; the configured submit threshold is a quote rule, not a generic attachment count.
- Practical rule: the RDA frontend should send the selected `attachment_type` with every uploaded file. The BFF must validate the enum and forward it to Mistra, keeping the legacy fallback only when old clients omit the field (`DRAFT -> quote`, `PENDING_VERIFICATION -> transport_document`). For `total_price >= RDA_QUOTE_THRESHOLD`, submit requires at least two attachments with `attachment_type == quote`; `RDA_QUOTE_THRESHOLD` defaults to `3000` and is exposed to the browser as `rdaQuoteThreshold` from `/config`.
- Evidence: `docs/mistra-dist.yaml` schema `po-attachment-upload`; RDA backend `handleUploadAttachment` and `handleSubmitPO`; frontend attachment helper in `apps/rda/src/lib/attachments.ts`; runtime threshold wiring in `backend/internal/platform/config` and `apps/rda/src/runtime-config.ts`.
- Used by: `apps/rda` `/rda/new`, `/rda/po/:id`, and RDA backend attachment upload/submit validation.
- Open questions: none.

### RDA PO PDF Download Is State-Gated

- Context: `apps/rda` PO detail PDF download and backend `GET /api/rda/v1/pos/{id}/pdf`.
- Discovery: users must not download the generated PO PDF until the PO has reached one of the approved/post-approval states explicitly allowed by the business flow.
- Practical rule: show and proxy PO PDF download only for `APPROVED`, `PENDING_SEND`, `SENT`, `PENDING_VERIFICATION`, `PENDING_DISPUTE`, `DELIVERED_AND_COMPLIANT`, and `CLOSED`. Block all other states, including Mistra/server intermediary states such as `PENDING_PDF_GENERATION` and `PENDING_ERP_SAVE`.
- Evidence: RDA implementation plan correction from product; frontend `canDownloadPOPDF`; backend `canDownloadPOPDF`.
- Used by: `apps/rda` `/rda/po/:id` and RDA backend PDF proxy.
- Open questions: none.

### RDA Patch Payload Null Semantics

- Context: `apps/rda` PO header edits forwarded to Mistra `rda-patch`.
- Discovery: Mistra rejects `budget_user_id: null` with `Value is not nullable`; in `rda-patch`, only `cost_center` is nullable. Optional text fields such as `description`, `note`, and `provider_offer_code` are strings, and `provider_offer_date` must be a valid date when present.
- Practical rule: for RDA PATCH payloads, omit `budget_user_id` for cost-center budgets, send `cost_center: null` only when switching to a user budget, and send optional text fields as strings rather than `null`. Omit an empty `provider_offer_date` instead of sending `null` or an invalid empty date.
- Evidence: `docs/mistra-dist.yaml` schema `rda-patch`; observed Mistra 400 response from `PATCH /rda/v1/pos/{id}`; implementation in `apps/rda/src/lib/po-payload.ts`.
- Used by: `apps/rda` PO detail and new-RDA wizard header save flows.
- Open questions: whether Mistra exposes a supported way to clear an existing `provider_offer_date`; the current safe behavior omits empty dates on PATCH.

### RDA PO Recipients Use The Dedicated Recipients Endpoint

- Context: `apps/rda` PO contact selection, clone flows, and any backend code that changes the provider recipients of an RDA PO.
- Discovery: Mistra exposes `PATCH /arak/rda/v1/po/{id}/recipients` for recipient selection. This endpoint accepts `recipient_ids` as an array and an empty array clears the selection. Recipient changes are not part of the generic PO header patch contract.
- Practical rule: the RDA frontend and BFF must save header fields and recipient selections as separate operations. Use `PATCH /api/rda/v1/pos/{id}/recipients` for selection and clearing, including clone recipient copy and provider-change clearing. Do not send `recipient_ids` through `PATCH /api/rda/v1/pos/{id}`.
- Evidence: `docs/mistra-dist.yaml` path `/arak/rda/v1/po/{id}/recipients`; backend `backend/internal/rda/handler.go` and `backend/internal/rda/clone.go`; frontend `apps/rda/src/api/queries.ts`.
- Used by: `apps/rda` PO detail, `/rda/new`, and PO clone.
- Open questions: none.

### RDA Budget Selection Keys

- Context: `apps/rda` budget selection in new/clone/edit RDA flows.
- Discovery: Mistra `budget-for-user` can return multiple spendable entries with the same `budget_id` and different `cost_center` values, for example `Trasferte` for two cost centers. A select value based only on `budget_id` makes those options indistinguishable and always resolves to the first matching budget.
- Practical rule: frontend form state must store a composite budget selection key built from `budget_id` plus the active binding (`cost_center` or `user_id`/`budget_user_id`). Payload builders must then translate that key back to numeric `budget_id` plus exactly one of `cost_center` or `budget_user_id`.
- Detail/read-only views must display the budget association returned on the PO itself. Do not substitute it with an entry from the current viewer's budget catalog by `budget_id` alone; `/budget-for-user` is only the selectable catalog for edits and creation.
- Evidence: `docs/mistra-dist.yaml` schema `budget-for-user`; implementation in `apps/rda/src/lib/budgets.ts`, `apps/rda/src/components/BudgetSelect.tsx`, and `apps/rda/src/lib/po-payload.ts`.
- Used by: `apps/rda` `/rda/new`, PO header edit, new PO modal, and clone PO modal.
- Open questions: none.

### RDA Row Totals Are Normalized By The BFF

- Context: `apps/rda` PO row tables in the new wizard and PO detail page.
- Discovery: Mistra exposes the PO aggregate `total_price`, but row responses may not include a useful `total_price` per row. Legacy Appsmith displayed a computed row total instead.
- Practical rule: `GET /api/rda/v1/pos/{id}` should normalize every row with `price` for goods and `total_price` before returning it to the frontend. Preserve a positive upstream `total_price` or `total` when present; otherwise calculate goods as `price * qty` and services as `(MRC * qty * initial_subscription_months) + (NRC * qty)`, using read names `montly_fee` and `activation_fee`. Because Arak's detail function may omit the stored good `price`, enrich rows from `rda.purchase_order_row` when available.
- Evidence: `docs/mistra-dist.yaml` `rda-row`; `docs/arak_schema.json` functions `get_purchase_order_rows_detail` and `create_purchase_order_row`; Appsmith audit row-total notes; backend `backend/internal/rda/row_totals.go`.
- Used by: `apps/rda` row tables and wizard row composer.
- Open questions: none.

### RDA Good Row Create Still Needs Empty Renew Detail

- Context: backend `POST /api/rda/v1/pos/{id}/rows` for RDA goods.
- Discovery: Mistra's `rda-row-create` schema requires `renew_detail` at the root even for `type=good`, although no renew fields are meaningful for goods. Omitting it returns `400` with `property "renew_detail" is missing`.
- Practical rule: when creating good rows, forward `renew_detail: {}` along with `price` and the calculated `total`. Do not include service-only renewal fields for goods. Send calculated `total` for service rows too, because Arak stores it in `purchase_order_row.total` and the PO aggregate sums that column.
- Evidence: Mistra schema `docs/mistra-dist.yaml` `rda-row-create`; `docs/arak_schema.json` functions `create_purchase_order_row` and `purchase_order_total_calculation`; observed upstream 400 during PO row create; backend `backend/internal/rda/validation.go`.
- Used by: `apps/rda` row composer and row modal.
- Open questions: none.

### RDA Row Edit Is A BFF Replace Operation

- Context: `apps/rda` PO row editing in `/rda/new` and PO detail.
- Discovery: Mistra exposes row create and delete (`POST /arak/rda/v1/po/{id}/row`, `DELETE /arak/rda/v1/po/{id}/row/{rowid}`) but no real row update endpoint.
- Practical rule: expose row edit to the browser only through `PUT /api/rda/v1/pos/{id}/rows/{rowId}`. The BFF must validate the caller and draft PO, create the replacement row first, and delete the old row only after create succeeds. If delete fails after create, return `409 ROW_REPLACE_DELETE_FAILED` so the UI refetches and shows the operator that both rows may be present.
- Evidence: `docs/mistra-dist.yaml` RDA row paths and backend `backend/internal/rda/validation.go`.
- Used by: `apps/rda` row modal and row tables.
- Open questions: none.

### RDA Portal Deep Links Include App Mount And App Route

- Context: portal notifications, emails, and any backend-generated links to RDA PO detail pages.
- Discovery: the production/static RDA app is mounted at `/apps/rda/`, while the React router route for PO detail is `/rda/po/:poId`. A backend-generated production deep link therefore needs both parts: `/apps/rda/rda/po/{poID}`. In local split-server development, an explicit `RDA_APP_URL` should be used when configured; otherwise the RDA Vite dev URL is `http://localhost:5190/rda/po/{poID}`.
- Practical rule: do not link backend notifications directly to `/rda/po/{poID}` in production. Build RDA PO links through the app-mount-aware helper and use `MRSMITH_PUBLIC_BASE_URL` only to make email links absolute.
- Evidence: `apps/rda/vite.config.ts` base `/apps/rda/` for builds, `apps/rda/src/routes.tsx` route `/rda/po/:poId`, backend helper `backend/internal/rda/notifications.go`.
- Used by: Notifications V1 RDA approval notifications.
- Open questions: none.

### RDA Comment Mentions Notify Through MrSmith, Not Mistra

- Context: `apps/rda` PO detail comment mentions and backend `POST /api/rda/v1/pos/{id}/comments`.
- Discovery: Mistra `po-comment-new` accepts only `comment`; it does not persist or process mention recipients. MrSmith owns the notification side effect after Mistra successfully creates the comment.
- Practical rule: the frontend should submit only users selected from the RDA mention dropdown. The RDA BFF must forward only the comment text to Mistra, validate selected mention recipients against enabled `users_int` users, and create `rda_comment_mention` notifications with the standard RDA PO deep link. Manually typed `@email` text is display-only and must not notify. Self-mentions are skipped unless `NOTIFY_SELF_MENTIONS=true`, which is intended only for local/development testing.
- Evidence: `docs/mistra-dist.yaml` schema `po-comment-new`; RDA frontend `MentionInput`; backend `backend/internal/rda/validation.go` and `backend/internal/rda/notifications.go`.
- Used by: `apps/rda` PO detail comments.
- Open questions: none.

### RDA Article Catalog Type Comes From The BFF

- Context: `apps/rda` row creation in `/rda/new` and PO detail row modal.
- Discovery: Mistra exposes RDA articles filtered by `type=good|service`, but the row UI must select an article first and derive the row type from that selected catalog item.
- Practical rule: the RDA row picker should load the unified catalog once with `GET /api/rda/v1/articles`, then filter locally by code, description, and the Italian type labels (`bene` / `servizio`). The RDA BFF fetches both good and service catalogs when `type` is omitted, normalizes every item to `{code, description, type}`, and deduplicates by `code`. Keep `?search=...` and `?type=good|service` only for compatibility or deliberately typed consumers.
- Evidence: backend `backend/internal/rda/articles.go`; frontend `apps/rda/src/api/queries.ts`, `apps/rda/src/components/ArticleCombobox.tsx`, and `apps/rda/src/lib/row-payload.ts`.
- Used by: `apps/rda` `/rda/new` row composer and PO detail row modal.
- Open questions: none.

### RDA Approval Permissions Come From users_int.role

- Context: `apps/rda` approver inboxes, PO action bar, and privileged RDA transitions.
- Discovery: Keycloak controls only base RDA app access with `app_rda_access`. The operational RDA approval flags are stored in Arak Postgres under `users_int.user.role -> users_int.role` and must be read by the backend using the authenticated user's token email.
- Practical rule: expose and consume `GET /api/rda/v1/me/permissions` for `is_approver`, `is_afc`, `is_approver_no_leasing`, `is_approver_extra_budget`, `can_see_all_po`, and `skip_approval`. Inboxes and privileged transitions must check only the action booleans, not `app_rda_approver_*` Keycloak roles, `app_devadmin` overrides, `can_see_all_po`, or `skip_approval`, because Mistra/Arak validates the same domain permissions with `Requester-Email`. Treat `can_see_all_po` as list/detail visibility and `skip_approval` as non-authoritative for manual approval of existing POs.
- Evidence: legacy RDA `user_permissions` SQL in `apps/rda/audit/05_datasource_catalog.md`; A2 plan in `artifacts/A2.md`; backend implementation in `backend/internal/rda/permissions.go`, `arak.go`, and `validation.go`.
- Used by: `apps/rda` navigation, inbox guards, `ActionBar`, and RDA backend transition handlers.
- Open questions: none.

### RDA DDT Documents Are Queried Directly On Arak With A Half-Open Civil-Day Range

- Context: AFC Tools "DDT Purchase Order" (backend `GET /afc-tools/v1/rda/ddt` and `POST /afc-tools/v1/rda/ddt/download`, frontend `apps/afc-tools` DdtPurchaseOrderPage).
- Discovery: RDA transport documents are rows of `rda.purchase_order_attachment` with `attachment_type = 'transport_document'` (enum from `rda.purchase_order_attachment_type`), joined to `files.file` for the stored document and to `rda.purchase_order` for the PO header; the visibility filter is `pa.deleted IS NULL AND f.deleted_at IS NULL`. The upstream per-file download is `GET /arak/rda/v1/po/{po_id}/attachment/{attachment_id}/download`, proxied through the shared OAuth2 `arak.Client` so the service token stays server-side. The call is rejected with `400` by Mistra when the `Requester-Email` header is missing: Arak resolves the acting user from that header (`backend/internal/rda/handler.go` `requesterHeaders`), not from the service token.
- Practical rule: implement DDT list/download by querying Arak directly with the shared query foundation (`backend/internal/afctools/rda_ddt.go` `rdaDDTAttachmentQueryPrefix`): select `rda.purchase_order_attachment` joined to `files.file` and `rda.purchase_order` (left-joined to `users_int."user"` for the requester), filtered by `attachment_type = 'transport_document'`, `pa.deleted IS NULL`, `f.deleted_at IS NULL`, and a `pa.created >= $1 AND pa.created < $2` range ordered by `pa.created DESC, po.id ASC, pa.id ASC`. Resolve the `from`/`to` civil dates in `Europe/Rome` into half-open instants (end = next civil day, `>= start AND < end`) so the PostgreSQL session time zone cannot shift the window and the final day is never truncated. For the ZIP download, stream each file from the Arak client into a ZIP entry; pass the authenticated user's email as `Requester-Email` on every upstream call (from `auth.GetClaims(r.Context())`); clear the server-wide write deadline per request with `http.NewResponseController(w).SetWriteDeadline(time.Time{})`, and on partial failure append a `documenti_non_scaricati.txt` manifest entry instead of failing the whole download. Name the ZIP folder per PO after the **PO id only** (`PO-<id>`): `rda.purchase_order.code` is a path-like string (e.g. `PO-50412/2026`) and is not unique, so a basename-style sanitizer collapses it to its last segment (`2026`) and hides the PO number. No new env vars: reuse `ARAK_DSN` (ArakDB) and the existing `arakCli`.
- Evidence: `backend/internal/afctools/rda_ddt.go` (query constants, `parseRDADDTDateRange`, `handleRDADDT`, `handleRDADDTDownload`, `rdaDDTArchivePaths`); `backend/cmd/server/main.go` afctools deps wiring (`Arak: arakCli, ArakDB: arakDB`); `docs/arak_schema.json` tables `rda.purchase_order_attachment` (`attachment_type`, `deleted`), `files.file` (`deleted_at`), `rda.purchase_order`, and function `get_purchase_order_attachments` (same `poa.deleted IS NULL` visibility); `backend/internal/platform/arak/client.go` `DoWithHeadersContext`.
- Used by: `apps/afc-tools` DDT Purchase Order page (`apps/afc-tools/src/pages/DdtPurchaseOrderPage.tsx` and `apps/afc-tools/src/api/queries.ts` `useRdaDdtAttachments` / `downloadRdaDdtZip`).
- Open questions: none.

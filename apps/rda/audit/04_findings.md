# Findings & migration risks

This file collects cross-cutting observations classified by type:
- **B** = embedded business rule
- **D** = duplication
- **S** = security concern
- **F** = fragile binding / latent bug
- **M** = migration blocker / requires backend work

## Embedded business rules (must be reproduced)

- **B-1.** PO must be in DRAFT and the current user must be its requester to edit/delete from the RDA list. Source: `tbl_po` row-level `isDisabled` on Modifica/Elimina.
- **B-2.** When `total_price ≥ 3000 €`, ≥2 attachments (i.e. quote PDFs) are mandatory before "Manda PO in Approvazione". Banner `Text31` plus `btn_save_draft.isDisabled` enforce it client-side. (`PO Details`)
- **B-3.** During DRAFT, uploads are tagged `attachment_type = "quote"`; otherwise `transport_document`. `attachmentsJs.uploadPreventivi` + `UploadAttachment.bodyFormData`.
- **B-4.** Conformity confirmation requires a DDT (transport document) — surfaced indirectly through the error toast "Errore conferma PO - verifica inserimento DDT" on `ConfirmConformity`.
- **B-5.** If `recipient_ids` is empty when sending an order, the backend uses the qualification reference contact — confirmed by the explanatory `Text24` in the Contatti Fornitore tab.
- **B-6.** The level-1 vs level-2 "Approva" buttons share the same endpoint POST `/po/{id}/approve`; they're disambiguated client-side by `current_approval_level`. Multi-level approval is a single state machine on the backend.
- **B-7.** The current user must be in `GetPoDetails.data.approvers[*].user.email` to approve — `Contact.checkMailIsPresent` is the gating function. Backend should enforce this independently.
- **B-8.** Payment-method change is allowed in DRAFT and additionally during `PENDING_APPROVAL_PAYMENT_METHOD`, but only by the requester (`BRT_upd_pagamento` enables only when state matches AND `requester.email == currentUser`).
- **B-9.** When a budget has a `cost_center`, send `cost_center`; otherwise send `budget_user_id` (mutually exclusive). Source: `NewPo` and `EditPO` body builders.
- **B-10.** Default payment method: supplier's `default_payment_method.code` if any, else `payment_method_default_cdlan.payment_method_code` (currently CDLAN BB60ggFM+10). Hard-coded fallback `"320"` exists and **must be removed** in the rewrite.
- **B-11.** Item economics:
  - `service`: required at least one of NRC/MRC > 0; `f_months_first_period` (durata) required; `sl_recurring_months` required (1/3/6/12); auto-renew → cancellation_advice required.
  - `good`: only `f_item_unit_price` and `qty`; can be `advance_payment`/`activation_date`/`specific_date`.
- **B-12.** Provider qualification reference (`QUALIFICATION_REF`) is locked: cannot be edited inline, cannot be removed, has its own approval state. Other reference categories are administrative/technical/other.

## Duplication (D)

- **D-1.** State-label map duplicated 4× (`LabelJs` on RDA, `LabelJs` on PO Details, `LabelsJS` on App. I-II LIV, `JSObject1` on App. no Leasing). Three of them have the **same** keyset; `LabelsJS` and the `App. no Leasing` `JSObject1` are missing 4 entries (`PENDING_VERIFICATION`, `PENDING_DISPUTE`, `DELIVERED_AND_COMPLIANT`) — so on those pages those statuses would render as raw `PENDING_VERIFICATION` etc. instead of Italian labels. Latent bug.
- **D-2.** `extractApproverList` duplicated on RDA (`Utils`) and `App.  incremento Budget` (`Utils`). Bodies identical.
- **D-3.** `Contact` JSObject exists twice (RDA vs PO Details) with overlapping methods. The PO Details one is fuller (adds `getLabel`, `loadingData`, `emptyContacts`, `updateContactList`, `checkMailIsPresent`, `storeSelectedSupplier`, `storeSelectedContact`).
- **D-4.** The PO list table is repeated, with subtle column changes, on RDA + 5 approver pages — total **6 near-copies** of the same table.
- **D-5.** Several REST queries are redeclared per page rather than shared (`CallBudget`, `ListaFornitori`, `PaymentMethonds`, `GetDefaultPaymentMethod` exist on both RDA and PO Details). In Appsmith this is unavoidable; in React the rewrite should share API client functions via `packages/api-client`.

## Security concerns (S)

- **S-1.** **Direct PostgreSQL access from the client** for sensitive data:
  - `user_permissions` query reads `users_int.user JOIN users_int.role` filtered by `{{user_email.text}}` (a user-controlled input widget bound to `appsmith.user.email`). The query interpolates `'{{user_email.text}}'` directly. If anyone changes the `user_email` input client-side, they get someone else's permissions. **This is an authorization bypass in the current app.** The new portal **must** compute permissions server-side from the trusted Keycloak token.
  - `userID` query: `SELECT id FROM users_int.user WHERE email = {{user_email.text}}` — same risk.
  - `Suppliers`, `GetArticles`, `GetDefaultPaymentMethod`, `PaymentMethonds`, `get_item_types` — read-only catalog data; lower risk but still inappropriate for the client to talk SQL directly.
- **S-2.** REST headers `Requester-Email: {{appsmith.user.email}}` are client-supplied. Today the Mistra NG Internal API **trusts** them. The new app must rely on the OAuth/OIDC token claim, not a header.
- **S-3.** Comments **claim** to support @-mentions but `PostComment` body sends only `{ comment }`. Mention IDs are computed but discarded. No notification is dispatched. (Bug, but listed under "S" because it is misleading to users who think mentions notify someone.)
- **S-4.** The dead Keycloak group reference `Acquisti RDA approvers I-II` in `groupButtono66fk8kt2a.disabledWhenInvalid` should not be carried over. The new portal uses `app_rda_*` role names per `CLAUDE.md`.

## Fragile / latent bugs (F)

- **F-1.** `PO_details_TotalAmount` displays `total_price.slice(0, -1)` — chops the last character of a string. Suggests the backend returns `total_price` with a trailing currency-symbol or unit, and this is a band-aid. Verify the canonical type/format and remove the slice.
- **F-2.** `s_budget` defaultOptionValue uses `b.id` while the source data option value uses `b.budget_id`. If `b.id !== b.budget_id`, the prefilled budget can be wrong.
- **F-3.** `IconButton3Copy` (refresh on attachments tab) has `isDisabled: GetPoDetails.data.state == 'PENDING_DISPUTE' || 'DELIVERED_AND_COMPLIANT'`. The right-hand side is always truthy (non-empty string), so the button is always disabled. Bug — should be `=== 'DELIVERED_AND_COMPLIANT'` second branch.
- **F-4.** `PostComment` body uses `{{Input2.text}}` directly (no JSON.stringify). Single-line comments work, but a comment containing `"` or newlines breaks the JSON. **Confirmed fragility.**
- **F-5.** The "Edit row" path in items reuses `CreateItemRow` (POST), so editing creates a duplicate row. The legacy `save_item_row` JS references a non-existent `upd_po_item` query. **Probable bug carried in production.**
- **F-6.** "Rifiuta no leasing" calls `RejectFirstSecondLevel.run()` (`/po/{id}/reject`), not a leasing-specific reject. May be intentional or a bug — confirm with backend.
- **F-7.** `EditPO` body uses `f_subject.text != null || f_subject.text != ''` (logical OR); at least one branch is always truthy, so the field is **always** included even if empty. Same pattern repeats for `note`, `description`, `project`. As a result, `EditPO` may overwrite fields with empty strings that the user never intended to clear.
- **F-8.** `select_motivation` and `input_motivazion` (motivazione esclusione 3-quote) are not wired to any save action. Either dead UI or unfinished feature.
- **F-9.** `GetProviderRef` path is `/provider/{{appsmith.store.selectedProvider}}/reference/{reference_id}` — `{reference_id}` is **not** an Appsmith expression (no double curlies), so it is sent literally as `{reference_id}`. The query is broken; appears unused.
- **F-10.** `Contact.loadingData` writes recipients HTML into `Text27` via `setText('<span style=…>')`. If a recipient name/email contains HTML special characters, this is an XSS vector. Replace with React rendering.
- **F-11.** `Date()` used as default for `f_date` (the read-only "Data" widget in the unused `Container19` block) gives the **current** date, not the PO created date. Container is hidden so it doesn't matter today, but cleaning up will avoid future confusion.
- **F-12.** `Modal1`, `mdl_supplierContact`, `Container24/Container17/Container19` and several `Copy/Copy2/Copy5Copy` widgets are dead leftovers. Do not port.

## Migration blockers / required backend work (M)

- **M-1.** Replace `user_permissions` SQL with a backend endpoint or claim-based authorization in Keycloak token. Without this, the rewrite cannot rely on the existing pattern. (See `CLAUDE.md` on `app_{appname}_access` role naming.)
- **M-2.** Move all request-body construction (NewPo, EditPO, CreateItemRow) **out** of the client. The frontend should send normalized payloads; backend translates to the legacy Mistra NG shape if needed.
- **M-3.** Decide whether to keep direct PostgreSQL reads for `payment_method` / `item_types` / `articles` **on the new backend** or expose REST endpoints in Mistra NG. Current dependence on `arak_db (nuovo)` couples the frontend to schema.
- **M-4.** Settle the `total_price` shape (`F-1`) — string vs number, with/without currency suffix.
- **M-5.** Settle whether `mentioned_user_ids[]` should be sent in `PostComment` and surfaced to recipients (notification path).
- **M-6.** Settle the broken edit-row path (`F-5`): backend either supports `PUT /po/{po_id}/row/{row_id}` (preferred) or the new UI prevents row edits and forces delete+recreate.
- **M-7.** Resolve the URL inconsistency for budget-increment list (`/po-pending-budget-increment` vs `/po/...`).
- **M-8.** Clarify whether `/leasing/reject` exists or rejection always goes through `/po/{id}/reject` (see `F-6`).
- **M-9.** Remove the hard-coded `"320"` payment-method fallback (`B-10`) and confirm the canonical CDLAN default with the backend.

## Candidate domain entities (for the new schema)

(See also `00_inventory.md`.) The migration plan should design the new backend models around:

- `PurchaseOrder` (the aggregate)
  - `PurchaseOrderRow` (line item, `good` or `service`)
  - `PurchaseOrderAttachment`
  - `PurchaseOrderRecipient` (subset of provider refs)
  - `PurchaseOrderApprover` (per-PO approver assignment with `level` and `user`)
  - `PurchaseOrderComment` (with `mentioned_user_ids` if implemented)
- `Provider` and `ProviderReference` (already managed by the provider-qualification module)
- `Budget` (read-only from `/budget/v1/budget-for-user`)
- `PaymentMethod` (catalog)
- `ItemType` / `Article` (catalog)
- `User` and `Role`/`Permissions` (read from Keycloak; flags align to current `is_*` columns)

## Recommended next steps

1. Review this audit with the RDA business owner (Acquisti) and confirm the rules in §B.
2. Hand the audit to `appsmith-migration-spec` to produce the migration PRD.
3. Run a focused engineering session on §M items to lock the backend contract before frontend work begins.
4. Decide which dead bits (`F-3`, `F-7`, `F-8`, `F-12`) to fix in the audit-derived spec rather than carrying forward as TODOs.

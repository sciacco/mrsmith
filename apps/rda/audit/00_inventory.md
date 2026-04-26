# RDA — Application inventory

**Source:** `apps/rda/rda.json.gz` (Appsmith application export)
**App name:** `RDA Richieste di Acquisto`
**Source type:** Appsmith JSON export (`artifactJsonType=Application`, schema 4/12)
**Audit produced on:** 2026-04-26

This audit reverse-engineers the Appsmith application that today implements RDA (Richieste di Acquisto / Purchase Order requests) for CDLAN. The export is to be used as **input for a migration** into a custom React + Go mini-app under `apps/rda/` and `backend/internal/rda/`. Treat `{{ … }}` Appsmith expressions as *evidence of behavior*, not as target code.

## Pages

| # | Name | Slug | Purpose (one-liner) |
|---|------|------|---------------------|
| 1 | `Home` | `home` | Stub: empty placeholder with a single `Chart1` widget. Effectively unused. |
| 2 | `RDA` | `rda` | List of Purchase Orders (PO) belonging to the user, plus modal-based **New PO** wizard with inline **New supplier** form. |
| 3 | `PO Details` | `po-details` | Full PO editor: header, items, attachments, supplier contacts, comments with @-mentions, state-driven approval/reject/send/PDF actions. |
| 4 | `App. I - II LIV` | `app-i-ii-liv` | Approver inbox for Level 1 / Level 2 approvers (`PENDING_APPROVAL`). Just a table that drills into PO Details. |
| 5 | `App. Leasing` | `app-leasing` | Approver inbox for AFC checking leasing (`PENDING_LEASING`). |
| 6 | `App.  incremento Budget` | `app-incremento-budget` | Approver inbox for over-budget approvals (`PENDING_BUDGET_INCREMENT`). |
| 7 | `App. metodo pagamento` | `app-metodo-pagamento` | Approver inbox for AFC checking non-default payment methods (`PENDING_APPROVAL_PAYMENT_METHOD`). |
| 8 | `App. no Leasing` | `app-no-leasing` | Approver inbox for users approving "non-leasing" path (`PENDING_APPROVAL_NO_LEASING`). |

> Note the leading double space in `App.  incremento Budget` is preserved exactly as in the export — search for it as `App.  incremento Budget` (two spaces).

## Datasources

The app uses **3 datasources**:

| Name | Plugin | Notes |
|------|--------|-------|
| `arak_db (nuovo)` | `postgres-plugin` | Direct PostgreSQL access for: `public.suppliers`, `public.item_types`, `provider_qualifications.payment_method`, `provider_qualifications.payment_method_default_cdlan`, `users_int.user`, `users_int.role`, `articles.article`. **All read-only.** |
| `Arak (mistra-ng-int)` | `restapi-plugin` | The Mistra NG Internal API, see `docs/mistra-dist.yaml`. Endpoints under `/arak/rda/v1/...`, `/arak/provider-qualification/v1/...`, `/arak/budget/v1/...`, `/arak/users-int/v1/...`. |
| `s3cloudlan` | `amazons3-plugin` | Bucket `arak`, signed URL listing. Used only by query `listArak` on PO Details (appears unused in the UI). |

## Top-level objects

- **121 actions (queries / JS functions)** across all pages
- **18 ActionCollections (JSObjects)** — most pages have a `LabelJs` and a `Utils`/`Contact` clone (state-label translation has been duplicated 4 times: see Findings)
- **Custom JS libs:** none
- **Layouts:** every page has a single `layouts[0].dsl` Canvas tree (no multi-layout responsive setup)

## App-level theme

Every widget binds the same set of theme tokens:
`appsmith.theme.colors.primaryColor`, `appsmith.theme.borderRadius.appBorderRadius`, `appsmith.theme.boxShadow.appBoxShadow`, `appsmith.theme.fontFamily.appFont`. Primary brand colour appears as `#e15615` (orange) elsewhere in inline styles. None of this is migration-relevant — the rewrite uses the portal design system in `packages/ui` per `docs/UI-UX.md`.

## Domain entities (candidate)

From the audit of bindings and APIs, the following entities are observable:

- **PO (Purchase Order / RDA):** `id`, `code`, `state`, `type`, `currency`, `language`, `project`, `object`, `description`, `note`, `total_price`, `created`, `updated`, `current_approval_level`, `provider_offer_code`, `provider_offer_date`, `reference_warehouse`, `payment_method`, `requester{email}`, `provider{id, company_name, ref{email}, …}`, `budget{id, name, cost_center?, user_email?, user_id?}`, `rows[]`, `attachments[]`, `recipients[]`, `approvers[{user{email}, level}]`. State-machine values listed in `LabelJs.stateMap`.
- **PO row (item):** `id`, `type` (`good`|`service`), `product_code`, `product_description`, `description`, `qty`, `price` (unit price for goods), `activation_fee` (NRC), `montly_fee` (MRC), `payment_detail{is_recurrent, start_at, start_at_date?, month_recursion?}`, `renew_detail{initial_subscription_months, automatic_renew, cancellation_advice?}`.
- **Attachment:** `id`, `file_id`, `file_name`, `attachment_type` (`quote` while DRAFT, `transport_document` after that), `created_at`, `updated_at`.
- **Comment:** `id`, `user{first_name, last_name, email, id}`, `comment` / `comment_text`, `created_at`, `replies[]` (same shape).
- **Provider (`/provider`):** `id`, `company_name`, `vat_number`, `cf`, `address`, `city`, `province`, `country`, `postal_code`, `language`, `default_payment_method{code, description}`, `refs[]` (provider references).
- **Provider reference (`/provider/{id}/reference`):** `id`, `first_name`, `last_name`, `email`, `phone`, `reference_type` (one of `OTHER_REF | ADMINISTRATIVE_REF | TECHNICAL_REF | QUALIFICATION_REF`).
- **Budget (`/budget/v1/budget-for-user`):** `budget_id`, `name`, `cost_center?`, `user_id?`, `user_email?`. Per-user scoped.
- **PaymentMethod (`provider_qualifications.payment_method`):** `code`, `description`, `rda_available` boolean. Default for CDLAN comes from `provider_qualifications.payment_method_default_cdlan`.
- **User permissions (`users_int.user`+`users_int.role`):** `is_afc`, `is_approver`, `is_approver_no_leasing`, `is_approver_extra_budget` — read directly from DB by email of the logged-in user.
- **User (`/users-int/v1/user`):** `id`, `email`, `first_name`, `last_name`, used only for @-mention search inside comments.
- **ItemType / Article (`/rda/v1/article`):** `code`, `description`, filtered server-side by `type=service|good`.

## Navigation map

```
Home         → (no links; effectively empty)
RDA          ⇄ PO Details (list → detail and back via "Chiudi")
                Modal "ModalNewPO" creates a PO and navigates to PO Details
RDA          ⇆ NuovoFornitore modal (inline supplier registration; stays on RDA)
PO Details   → RDA / App. I-II LIV / App. Leasing / App. metodo pagamento /
               App. no Leasing / App. incremento Budget (after each approval/reject)
App. * pages → PO Details (drill-down)
```

## Cross-cutting findings (preview)

(Detailed in `04_findings.md`.)
- The state machine is duplicated in **four** different `LabelJs/JSObject1/LabelsJS` modules.
- Two flavours of `Utils` and three of `Contact` exist (RDA vs PO Details vs Budget app); the Budget-app one is mostly empty.
- Approver permission logic is hard-coded in widget `isDisabled` bindings using a direct `users_int.role` SQL query, not via the portal Keycloak roles.
- Direct PostgreSQL access from the client for auth-sensitive data (user permissions, payment methods) is the biggest migration concern.

## Files in this audit

- `00_inventory.md` — this file
- `01_pages_rda_home.md` — `Home` and `RDA` page audits
- `02_page_po_details.md` — `PO Details` page audit (the largest one)
- `03_pages_approver_inboxes.md` — the 5 approver-only list pages
- `04_findings.md` — cross-cutting business rules, duplication, security, migration blockers
- `05_datasource_catalog.md` — every query and JS method, datasource by datasource
- `06_jsobject_methods.md` — all JSObject methods, normalised

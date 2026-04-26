# RDA — Migration Specification (v1)

## Summary

- **Application name:** RDA — Richieste di Acquisto (Purchase Order requests)
- **Audit source:** `apps/rda/audit/00_inventory.md` … `06_jsobject_methods.md` (Appsmith export at `apps/rda/rda.json.gz`)
- **Spec status:** complete; ready for `portal-miniapp-generator` (Phase 3 — implementation planning)
- **Last updated decisions:** Q-A1..A13 resolved (see Phase A); B+C+D phases produced no further blocking questions; D.Q-1..D.Q-4 carry sensible defaults
- **Migration intent:** **1:1 functional port** of the Appsmith app to a React + Go portal mini-app, **reusing the existing Mistra NG `/arak/rda/...` API surface unchanged** at the wire level; legacy security and quality issues are fixed at the new module's boundary.

## Current-State Evidence

- **Source pages/views:** 8 Appsmith pages (`Home`, `RDA`, `PO Details`, 5 approver inboxes). The `Home` page is empty in practice; the 5 approver inboxes are near-clones of one another. → reduces to **3 routes** in v1 (`/rda`, `/rda/inbox/:kind`, `/rda/po/:poId`).
- **Source entities and operations:** 13 entities (Phase A §A.1). PO is the aggregate; rows / attachments / comments / recipients / approvers are nested. Provider, ProviderRef, Budget, PaymentMethod, Article, User, UserPermissions are catalogs/identity reads.
- **Source integrations and datasources:** Mistra NG REST (`/arak/rda/...`, `/arak/provider-qualification/...`, `/arak/budget/...`, `/arak/users-int/...`), direct Postgres `arak_db (nuovo)` (catalog + permission reads), Amazon S3 plugin (dead, never bound).
- **Known audit gaps or ambiguities:**
  - OpenAPI `rda-document-detail` does **not** declare `recipients[]` or `approvers[]` though the live response contains them.
  - Read/write field-name divergences: `start_pay_at_activation_date` (read) ↔ `start_at` enum (write); `comment` ↔ `comment_text`; `montly_fee` typo carried through.
  - `total_price` returned as a string with a trailing character; legacy strips it client-side.
  - One auditable widget (`btn_sendOrder`) was placed in the wrong container in audit narrative; verified live in `cnt_fornitore`.

---

## Entity Catalog

### Entity: PurchaseOrder (PO / RDA)

- **Purpose:** the workflow aggregate — header, rows, attachments, comments, recipients, approvers, state.
- **Operations:** list-mine, list-inbox-by-kind (5 kinds), read, create, patch (header), delete (DRAFT only), submit, approve/reject (kind-specific), update payment method, send to provider, mark leasing-created, confirm/reject conformity, generate PDF.
- **Fields (read shape):** `id, code, state, current_approval_level, type ∈ {STANDARD,ECOMMERCE}, currency='EUR', language, project, object, description, note, total_price (string, F-1), created/creation_date/updated, provider_offer_code, provider_offer_date, reference_warehouse, payment_method:{code,description}, requester:{id,email}, provider:{id,company_name,erp_id?,state}, budget:{id,name,year,cost_center?,budget_user_id?}, rows[], attachments[], approvers[]:{user:{email},level:'1'|'2'} (audit-only), recipients[]:ProviderReference subset (audit-only), language`.
- **Relationships:** owns rows / attachments / comments; references budget, provider, payment_method by id; aggregates approvers + recipients as embedded associations.
- **Constraints and business rules:**
  - **B-1** Edit/Delete only by requester AND only while DRAFT.
  - **B-2** `total_price ≥ 3000 €` ⇒ ≥ 2 attachments before submit (per Q-A11: count *all* attachments, 1:1 with legacy).
  - **B-9** Budget binding mutex: exactly one of `cost_center` / `budget_user_id` is sent per PO.
  - **B-10** Default payment method = supplier default → CDLAN default. The legacy `"320"` literal is dropped.
  - **B-13** Currency hard-locked to EUR (UI-side; API allows the field).
  - State machine of 23 states (Phase A §A.3) with state-driven action availability.
- **Open questions:** none (after Q-A1..A13).

### Entity: PurchaseOrderRow

- **Purpose:** a line item of kind `good` or `service`.
- **Operations:** create (POST), delete (DELETE). **No update in v1** (Q-A7; M-6 tracked).
- **Fields:** `id, purchase_order_id, type, description, requester_email, product_code, product_description, qty, price (good), montly_fee (read; service MRC), activation_fee (read) / activation_price (write) (NRC), payment_detail{is_recurrent, start_at|start_pay_at_activation_date, start_at_date, month_recursion ∈ {1,3,6,12}}, renew_detail{initial_subscription_months, next_subscription_months?, automatic_renew, cancellation_advice}`.
- **Relationships:** belongs-to `PurchaseOrder`.
- **Constraints (B-11):**
  - `service`: at least one of MRC / NRC > 0; `initial_subscription_months` required; `month_recursion` required; if `automatic_renew == true`, `cancellation_advice` required.
  - `good`: only `price` and `qty` (+ `start_at`).
- **Open questions:** none.

### Entity: PurchaseOrderAttachment

- **Purpose:** quote / DDT / other document attached to a PO.
- **Operations:** upload (multipart), delete (DRAFT only), download (signed URL or stream).
- **Fields:** `id, attachment_type ∈ {quote, transport_document, other}, file_id, file_name, created_at, updated_at`.
- **Constraints (B-3):** auto-tag rule — if PO state is `DRAFT` → `quote`, otherwise → `transport_document`. Decided server-side.
- **Open questions:** none.

### Entity: PurchaseOrderComment

- **Purpose:** discussion thread on a PO; supports single-level replies; UI offers @-mentions but Q-A6 keeps them cosmetic in v1.
- **Operations:** list, create (POST). No edit, no delete.
- **Fields (read):** `id, user:{id,first_name,last_name,email}, comment | comment_text, created_at, replies[]`. (Per Q-A5: accept either field name; emit `comment` on write.)
- **Fields (write):** `{comment, mentioned_user_ids?}`. v1 ignores `mentioned_user_ids` server-side.
- **Open questions:** D.Q-2 (whether the API needs `user_id` explicitly on the body) — default: assume no, derive from `Requester-Email`.

### Entity: PurchaseOrderRecipient (association)

- **Purpose:** subset of provider refs marked as recipients of the order.
- **Operations:** read (embedded), update (via PATCH PO header with `recipient_ids:int[]`).
- **Fields:** mirrors `ProviderReference` minus the qualification ref.
- **Constraints (B-5):** empty `recipient_ids` ⇒ Mistra falls back to the provider's `QUALIFICATION_REF`. The new UI surfaces this caption when no recipient is selected.

### Entity: PurchaseOrderApprover (association)

- **Purpose:** per-PO approver assignment with level (1 or 2).
- **Operations:** read only (assigned by Mistra at submit time).
- **Fields:** `{user:{email}, level:'1'|'2'}`.
- **Constraints (B-7):** approve/reject buttons require both the matching Keycloak role AND `currentUser.email ∈ approvers[*].user.email`.

### Entity: Provider, ProviderReference, Budget, PaymentMethod, Article, User, UserPermissions

Read-only (or quasi-read-only) catalogs / identity entities. Field maps in Phase A §A.2.7 – §A.2.13. **No new schemas.**

- **Provider** + **ProviderReference**: reused via the existing `fornitori` module. RDA does not proxy them itself.
- **Budget**: read via Mistra `GET /arak/budget/v1/budget-for-user`. Mutex rule B-9 applies when a budget is bound to a PO.
- **PaymentMethod**: read directly from `arak_db (nuovo)` Postgres in the new RDA module (no Mistra REST equivalent).
- **Article**: read via Mistra `GET /arak/rda/v1/article?type=service|good`.
- **User search**: read via Mistra `GET /arak/users-int/v1/user?search_string=…` (used only for the @-mention popup).
- **UserPermissions**: derived from Keycloak token claims; surfaced via `/api/rda/v1/me/permissions`. Replaces the legacy `users_int.role` SQL.

---

## View Specifications

### View: `/rda` — My POs + Create wizard

- **User intent:** browse own POs in any state; start a new RDA via a modal wizard.
- **Interaction pattern:** list with row actions (View / Edit / Delete) + a modal dialog containing a 3-section form (PO header, supplier+payment, optional inline new-supplier).
- **Main data shown or edited:** `PurchaseOrder` previews; create-form bindings for header, supplier, payment, and (optionally) a new `Provider`.
- **Key actions:** open create wizard; edit/delete a DRAFT PO; view a non-DRAFT PO.
- **Entry and exit points:** entry from the launcher (`app_rda_access`); exit to `/rda/po/:id` after creating a draft or selecting a row.
- **Notes on current vs intended behavior:**
  - F-1 fix: parse `total_price` to number in the table rendering.
  - F-2 fix: budget select uses `budget_id` consistently.
  - B-10 fix: `"320"` payment-method literal removed.
  - Dropped: `Modal1`, `NuovoFornitore` modal (legacy duplicate), the empty `Home` page.

### View: `/rda/inbox/:kind` — Approver inbox (parameterised)

- **User intent:** as an approver of `kind`, see only POs awaiting my action and drill down to act.
- **Interaction pattern:** list with a single "Gestisci" row action.
- **Main data shown:** a subset of `PurchaseOrder` columns (`state, requester.email, created, code, provider.company_name, project, total_price`).
- **Key actions:** drill down to `/rda/po/:id`.
- **Entry and exit points:** entry from the launcher tile (visible only when the relevant role is held); exit to PO Details.
- **Notes on current vs intended behavior:**
  - 5 legacy pages collapse to one parameterised view.
  - Per-route Keycloak role enforced both client-side (route guard) and server-side (`acl.RequireRole`).
  - URL anomaly preserved at the Mistra wire (`/po-pending-budget-increment`, Q-A10) but the public RDA endpoint is uniform: `/api/rda/v1/pos/inbox/budget-increment`.

### View: `/rda/po/:poId` — PO Details (workflow editor)

- **User intent:** read or edit the PO; act on its current state.
- **Interaction pattern:** master detail editor with sticky action bar, editable header form, tabbed body (Allegati / Righe PO / Note / Contatti Fornitore), and a comments side panel.
- **Main data shown or edited:** the full `PurchaseOrder` aggregate.
- **Key actions:** 21 actions enumerated in Phase B §B.4.S1 — submit, approve/reject (5 kinds), update payment method, send to provider, leasing-created, confirm/reject conformity, generate PDF, save header, add row, delete row, upload/delete/download attachment, add/edit recipient(s), post comment, close.
- **Entry and exit points:** entry from `/rda` (Modifica/Vedi) or from any `/rda/inbox/:kind` (Gestisci); exit via "Chiudi" or after a transition (toast → relevant inbox or `/rda`).
- **Notes on current vs intended behavior:**
  - Q-A7: "Modifica riga" pencil hidden in v1 (delete + recreate). M-6 tracked.
  - Q-A6: @-mentions remain UI-only; `mentioned_user_ids` not sent.
  - Q-A1: state label fix (`IN ATTESA VERIFICA CONFORMITÀ`).
  - F-3 fix on attachments refresh icon; F-4 fix on comment JSON encoding; F-7 fix on partial PATCH semantics; F-10 closed (no HTML stuffing into recipient summary); F-11 disappears with the dropped hidden container.
  - Dropped: `mdl_supplierContact`, `Modal1`, motivazione fields, hidden orphan containers, `lst_itemsCopy`.

---

## Logic Allocation

### Backend responsibilities (`backend/internal/rda/`)

- **Auth boundary:** read OIDC `email` claim, inject as `Requester-Email` header into every Mistra call.
- **Permission derivation:** map 4 Keycloak roles to the legacy 4 boolean flags; expose `GET /api/rda/v1/me/permissions`.
- **Body shaping** for `POST /pos`, `PATCH /pos/{id}`, `POST /pos/{id}/rows`, `PATCH /provider/.../reference/...` (asymmetric phone/email semantics) — Go DTOs replace the legacy IIFE bodies.
- **Attachment auto-tag** (B-3): server reads current PO state, sets `attachment_type`.
- **PDF / file download:** return signed URL (302) or stream; no client-side base64.
- **Catalog reads** that today are direct PG: `payment_method`, `payment_method_default_cdlan`. Same SQL, server-side.
- **Inbox role enforcement** via `acl.RequireRole` on each `/inbox/:kind` route.
- **Defence-in-depth approver guard:** re-check `email ∈ approvers[]` before forwarding any approve/reject call.

### Frontend responsibilities (`apps/rda/`)

- 3 routes (`/rda`, `/rda/inbox/:kind`, `/rda/po/:poId`).
- 8 shared components (Phase B §B.5): `PoListTable`, `StateBadge`, `ActionBar`, `RecipientsList`, `MentionInput`, `ProviderRefTable`, `BudgetSelect`, `PaymentMethodSelect`.
- Form schemas (Zod or equivalent) for: new-PO, new-provider, item, provider-ref edit/add.
- Action bar derives a single `permissions` object from `state + role + checkMailIsPresent + isRequester`; passed via context to children.
- React Query (or equivalent) for caching shared catalogs across views.
- Routing, modal/dialog state, toast, navigation post-action.

### Shared validation or formatting

- **State labels** module (`apps/rda/src/lib/state-labels.ts`) — collapses 4 legacy duplicates.
- **Format helpers** (`extractApproverList`, `dateConverter`, mention-token helpers).
- **Provider-ref category lists** + `getLabel` (B-12 baked in).
- **Form schemas** are shared between FE (one-shot validation) and BE (echo on the request boundary).

### Rules being revised rather than ported

These were intentionally revised, not copied; recorded so the implementation team does not reintroduce them:

- Direct PG access from the browser → backend reads.
- Trusted `Requester-Email` header → token-derived.
- 4 duplicated state-label modules → 1 shared module.
- Inline `setText('<span>...</span>')` for recipient HTML → React render (closes XSS F-10).
- Sequential `showAlert` validation → form-level validation.
- Hard-coded `"320"` payment-method literal → catalog-driven.
- Edit-row duplicate-creating bug (F-5) → pencil hidden until backend supports `PUT /row/{id}`.
- Comment `mentioned_user_ids` discarded silently → still discarded but explicit (Q-A6); cosmetic only.
- Budget-increment URL anomaly (Q-A10) → exposed cleanly at our public API; legacy URL preserved at the Mistra wire.

---

## Integrations and Data Flow

### External systems and purpose

- **Mistra NG (Arak gateway)** — every PO lifecycle call, every catalog read except payment methods, every download. Accessed via `backend/internal/platform/arak.Client.DoWithHeaders` with the user-derived `Requester-Email`.
- **Keycloak** — OIDC code flow at the portal layer; provides email + roles via `auth.Middleware`.
- **Arak Postgres (`arak_db (nuovo)`)** — read-only catalog reads for `provider_qualifications.payment_method` and `payment_method_default_cdlan`. Same DB pool already injected into the `fornitori` module.
- **Amazon S3 (`arak` bucket)** — *not* accessed directly. PDFs and attachments stream through Mistra endpoints which abstract S3.

### End-to-end user journeys (locked in Phase D)

- **J-1** Requester creates and submits a draft (RDA list → modal → `/rda/po/:id` → rows → attachments → submit).
- **J-2** L1/L2 approver: inbox → drill → approve/reject.
- **J-3** AFC handles leasing (and later Leasing-Created step).
- **J-4** Send order to provider (state-only gate; no role).
- **J-5** Conformity (DDT upload → confirm or reject).

### Background or triggered processes

None on our side. The state machine is server-side at Mistra; our module has no goroutines, no cron, no queue in v1.

### Data ownership boundaries

- PO aggregate, providers, budgets, articles, files, PDFs → **Mistra** owns; we proxy.
- Payment-method catalog → read directly from Arak Postgres (server-side).
- User identity & roles → Keycloak (token).
- **No new tables, no new schemas, no migrations** introduced by RDA in v1.

### Cross-module coupling inside the portal

- **`fornitori` module** is reused for all provider CRUD. RDA frontend imports its typed client directly; RDA backend does not re-proxy.
- A user with `app_rda_access` is also expected to have `app_fornitori_access` (read scope). Default per D.Q-1: bundle at the Keycloak group level.

---

## API Contract Summary

### Required capabilities

- All `/arak/rda/v1/...` Mistra endpoints listed in audit `05_datasource_catalog.md` (28 endpoints).
- All `/arak/provider-qualification/v1/...` calls already proxied by the `fornitori` module.
- `GET /arak/budget/v1/budget-for-user` (RDA module proxies).
- `GET /arak/users-int/v1/user?search_string=...` (RDA module proxies).
- Direct Postgres reads against `provider_qualifications.payment_method` and `payment_method_default_cdlan` (RDA module).

### Public surface of the RDA Go module

(See Phase C §C.3 for the full list. After Phase D's reuse decision, the `/api/rda/v1/providers/...` block is removed — those calls go to `/api/fornitori/v1/...` directly.)

```text
# Identity
GET    /api/rda/v1/me/permissions

# Catalogs
GET    /api/rda/v1/budgets
GET    /api/rda/v1/payment-methods
GET    /api/rda/v1/payment-methods/default
GET    /api/rda/v1/articles?type=...
GET    /api/rda/v1/users?search=...

# PO list & inboxes
GET    /api/rda/v1/pos
GET    /api/rda/v1/pos/inbox/level1-2
GET    /api/rda/v1/pos/inbox/leasing
GET    /api/rda/v1/pos/inbox/no-leasing
GET    /api/rda/v1/pos/inbox/payment-method
GET    /api/rda/v1/pos/inbox/budget-increment

# PO CRUD
POST   /api/rda/v1/pos
GET    /api/rda/v1/pos/{id}
PATCH  /api/rda/v1/pos/{id}
DELETE /api/rda/v1/pos/{id}

# PO transitions
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

# Rows
POST   /api/rda/v1/pos/{id}/rows
DELETE /api/rda/v1/pos/{id}/rows/{rowId}
# (no PUT in v1 — Q-A7; tracked as M-6)

# Attachments
POST   /api/rda/v1/pos/{id}/attachments
GET    /api/rda/v1/pos/{id}/attachments/{aid}
DELETE /api/rda/v1/pos/{id}/attachments/{aid}

# Comments
GET    /api/rda/v1/pos/{id}/comments
POST   /api/rda/v1/pos/{id}/comments
```

### Read endpoints (high-traffic)

- `GET /pos` (list-mine) — paged, per-user.
- `GET /pos/inbox/:kind` — paged, role-gated.
- `GET /pos/{id}` — single PO with embedded rows / attachments / approvers / recipients.
- `GET /budgets`, `/payment-methods`, `/payment-methods/default`, `/articles?type=…` — small, cacheable catalogs.

### Write commands

- `POST/PATCH/DELETE /pos[/{id}]` — header CRUD.
- `POST/DELETE /pos/{id}/rows[/{rowId}]` — row create/delete.
- `POST/DELETE /pos/{id}/attachments[/{aid}]` — multipart upload, delete.
- `POST /pos/{id}/comments` — body `{comment, mentioned_user_ids? (ignored v1)}`.

### Derived or workflow-specific operations

- 13 transition endpoints under `/pos/{id}/...` (submit, approve, reject, leasing/{approve,reject,created}, no-leasing/approve, payment-method/{approve}, budget-increment/{approve,reject}, conformity/{confirm,reject}, send-to-provider).
- `GET /pos/{id}/pdf` — returns 302 to a signed URL (or streams).

---

## Constraints and Non-Functional Requirements

### Security or compliance

- **No client-side database access.** All `arak_db` reads move server-side.
- **No client-supplied identity headers.** `Requester-Email` is set by the Go module from the OIDC token, never from the browser.
- **Role-gated routes.** `/api/rda/v1/...` enforce roles via `acl.RequireRole`; the launcher hides tiles the user cannot access.
- **Defence-in-depth** on transitions: the approver guard (`checkMailIsPresent`) is enforced both client-side (for UX) and server-side (in our Go module, before the Mistra forward).
- **No logging of PII** beyond what Mistra already records; standard `slog` patterns from the other modules.
- **No XSS** — recipient summary becomes a React component (closes legacy F-10).

### Performance or scale

- The legacy app does naive `disable_pagination=true` reads for PO lists and provider lists. v1 keeps that for parity but adds **server-side response caching of catalogs** (`/budgets`, `/payment-methods`, `/articles`) for the request lifetime. Pagination is a v2 concern.
- File uploads: 25 MiB max, mirroring `fornitori`'s `maxUploadBytes`.
- Mistra latency dominates the user experience; the Go module adds < 5 ms per call (token cached, single hop).

### Operational constraints

- Deployed as part of the existing portal binary (`backend/cmd/server`); no separate process.
- Same Docker image, same K8s deployment as the rest of the portal.
- Dev port for Vite: pick the next free port (Phase E confirms; default proposed `5184`).
- Hot-reload via existing `air` for Go and `vite` for the frontend (`make dev` orchestrates).
- Auth dev bypass (`VITE_DEV_AUTH_BYPASS` + `SKIP_KEYCLOAK`) follows the convention used by `manutenzioni` / `fornitori`.

### UX or accessibility expectations

- Italian copy preserved verbatim except for Q-A1 (typo fix).
- Action availability is *visible* (disabled buttons with tooltips) rather than *hidden* — same pattern as the legacy app, easier to test.
- All `(*)` labels are replaced by structured `required` markers with consistent asterisk styling.
- Mobile: not required in v1 (matches legacy; the portal is desktop-first per `docs/UI-UX.md`).
- Stripe-level polish via `packages/ui`, per `docs/UI-UX.md`. The legacy orange `#e15615` brand color is **not** ported.

---

## Open Questions and Deferred Decisions

All blocking questions are resolved. The following are deferred decisions with sensible defaults applied — listed for the implementation phase to revisit:

| # | Question | Default | Decision owner |
|---|----------|---------|----------------|
| **D.Q-1** | Bundle `app_fornitori_access` with `app_rda_access` at Keycloak group level (D-R1) vs UI fallback (D-R2)? | D-R1; track in `docs/TODO.md` | ops + tech-lead |
| **D.Q-2** | Does `POST /comment` on Mistra need a body `user_id`? | No; derived from `Requester-Email` | implementation team (validate in dev) |
| **D.Q-3** | Inbox role enforcement server-side in the Go module? | Yes — `acl.RequireRole` per inbox kind | tech-lead |
| **D.Q-4** | Is the dead `Acquisti RDA approvers I-II` Keycloak group still in use? | Treated as dead; not bound | ops |
| **M-6** | When does Mistra add `PUT /po/{id}/row/{rowId}`? | Tracked in `docs/TODO.md`; UI hides the pencil until then | Mistra/Arak team |
| **F-1** | When does Mistra fix `total_price` trailing-character bug? | Frontend parses to number; tracked | Mistra/Arak team |
| **OpenAPI spec gaps** | Should `recipients[]` and `approvers[]` be added to `rda-document-detail`? | Internal Go/TS types model the live shape; tracked | Mistra/Arak team |

---

## Acceptance Notes

### What the audit proved directly

- 121 actions and 18 ActionCollections enumerated; every JSObject method classified.
- All 8 legacy pages have observable bindings; the action bar's state-driven button availability is fully captured.
- Direct-DB reads, dead-code modules, and the @-mention "fake" feature are demonstrated, not inferred.

### What the expert confirmed (this iteration)

- Q-A1: state-label typo fixed.
- Q-A2/A3: send-to-provider and conformity actions stay state-only (no role check), 1:1.
- Q-A4: OpenAPI gap for `recipients`/`approvers` does not block; we type the live shape.
- Q-A5: comment field-name (`comment` ↔ `comment_text`) handled by the new client.
- Q-A6: @-mentions remain cosmetic in v1.
- Q-A7: hide row-edit pencil; do not replicate the duplicate-creating bug. (User explicitly: "don't replicate the bug.")
- Q-A8: drop dead motivazione fields. (Confirmed via export inspection, not just audit narrative — both widgets `isVisible:false`, no triggers, no consumers.)
- Q-A9..A13: strict 1:1 with legacy.
- Audit narrative correction: `btn_sendOrder` lives in `cnt_fornitore`, not in `ButtonGroup1`. Recorded.

### What still needs validation (during implementation)

- D.Q-2 (comment body `user_id`).
- Mistra response shape for `total_price` (still string-with-suffix? check current sample.)
- Whether the response under `recipients[]` matches a `ProviderReference` subset exactly (the audit infers it from `Table4.defaultSelectedRowIndices` matching by id).
- The exact dev port (collision-free).

### Hand-off

This document is the input for `portal-miniapp-generator` Phase 3:
- repo-fit checklist (`docs/IMPLEMENTATION-PLANNING.md`),
- the new-app dev wiring (root `package.json`, Makefile, `applaunch/catalog.go`, `cmd/server/main.go`, `config/config.go`),
- the actual file scaffolding under `apps/rda/` and `backend/internal/rda/`,
- UI review gates per `docs/UI-UX.md`.

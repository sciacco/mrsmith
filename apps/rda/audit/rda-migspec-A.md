# RDA migration spec — Phase A: Entity-Operation Model

**Source audit:** `apps/rda/audit/00_inventory.md` … `06_jsobject_methods.md`
**Backend contract reference:** `docs/mistra-dist.yaml` (Mistra NG Internal API, tag `arak-rda` + companions `arak-budget`, `arak-provider-qualification`, `arak-users-int`).
**Migration intent (user-stated):** **1:1 porting** of the Appsmith app into the React + Go portal mini-app, reusing the existing `/arak/rda/v1/...` API surface. No backend redesign in v1; the new Go module proxies through the shared Arak client (`backend/internal/platform/arak`).

This phase enumerates the *domain entities the new app must speak about*, the *operations* it performs on each, and the *fields* observed in current bindings. Where the audit and the OpenAPI disagree, both are recorded.

---

## A.1 Inventory of entities

| # | Entity | Owns | Read source (current) | Write source (current) |
|---|--------|------|------------------------|-------------------------|
| 1 | **PurchaseOrder (PO / RDA)** | the aggregate (header + state machine) | `GET /arak/rda/v1/po`, `GET /arak/rda/v1/po/{id}`, plus 5 inbox endpoints | `POST/PATCH/DELETE /arak/rda/v1/po`, transition POSTs |
| 2 | **PurchaseOrderRow** | the line-item, kind = `good` \| `service` | embedded in `GET /arak/rda/v1/po/{id}`.`rows[]` | `POST/DELETE /arak/rda/v1/po/{id}/row[/{rowId}]` |
| 3 | **PurchaseOrderAttachment** | document attached to PO (quote / DDT / other) | embedded `attachments[]` | `POST/DELETE /arak/rda/v1/po/{id}/attachment[/{aid}]`; `GET .../download` |
| 4 | **PurchaseOrderComment** | thread on a PO (with `replies[]`, possibly @-mentions) | `GET /arak/rda/v1/po/{id}/comment` | `POST /arak/rda/v1/po/{id}/comment` |
| 5 | **PurchaseOrderRecipient** *(association)* | subset of provider refs marked as recipients of the order | embedded `recipients[]` (audit) — **not declared in OpenAPI** | `PATCH /arak/rda/v1/po/{id}` body `recipient_ids` |
| 6 | **PurchaseOrderApprover** *(association)* | per-PO approver assignment with `level` and `user.email` | embedded `approvers[]` (audit) — **not declared in OpenAPI** | (server-side, none from client) |
| 7 | **Provider (qualified supplier)** | external company; has refs and default payment method | `GET /arak/provider-qualification/v1/provider[?usable=true]`, `GET .../provider/{id}` | `POST /arak/provider-qualification/v1/provider/draft` (inline new-provider) |
| 8 | **ProviderReference (contact)** | a person at a provider, `reference_type ∈ {OTHER\_REF, ADMINISTRATIVE\_REF, TECHNICAL\_REF, QUALIFICATION\_REF}` | embedded `provider.refs[]` | `POST/PUT /arak/provider-qualification/v1/provider/{id}/reference[/{refId}]` |
| 9 | **Budget** *(read-only catalog scoped to user)* | spendable budget envelope, optionally per cost-center / per-user | `GET /arak/budget/v1/budget-for-user` | — |
| 10 | **PaymentMethod** *(catalog)* | code + description + `rda_available` boolean; CDLAN default | direct PG: `provider_qualifications.payment_method`, `payment_method_default_cdlan` | — |
| 11 | **Article / ItemType** *(catalog)* | catalogue items keyed by `code`, filterable by `type` | `GET /arak/rda/v1/article?type=service\|good` | — |
| 12 | **User (internal Arak user)** | `id, email, first_name, last_name`; only used for @-mention search | `GET /arak/users-int/v1/user?search_string=…` | — |
| 13 | **UserPermissions / Role flags** | `is_afc`, `is_approver`, `is_approver_no_leasing`, `is_approver_extra_budget` | direct PG: `users_int.user JOIN users_int.role` (security blocker S-1) | — |

> **Migration scoping note (1:1).** Entities 1–6 are *owned by the RDA backend*. Entities 7–13 are *consumed*. The new Go module re-uses the existing endpoints for both groups; no new schemas or DB tables are introduced in v1.

---

## A.2 Per-entity field maps

Three columns: **field**, **observed type / shape**, **source of truth**. "Audit-only" means the field is referenced in Appsmith bindings but **does not appear** in the OpenAPI schema for the corresponding endpoint. "Spec-only" means the inverse.

### A.2.1 PurchaseOrder

Source documents: OpenAPI `rda-document-detail` / `rda-document-preview` / `rda-create` / `rda-patch`; audit `02_page_po_details.md` (header bindings) and `01_pages_rda_home.md` (table columns).

| Field | Type / shape | Source | Notes |
|-------|--------------|--------|-------|
| `id` | int64 | both | aggregate id |
| `code` | string | both | human-readable PO number (e.g. `PO-2026-…`) |
| `state` | string (enum, see §A.3) | both | drives the entire workflow |
| `current_approval_level` | int (1 \| 2) | both | only meaningful when `state == PENDING_APPROVAL` |
| `type` | enum `STANDARD` \| `ECOMMERCE` | both | `STANDARD` triggers the order-to-provider step |
| `currency` | string (`EUR`) | both | UI hard-locks to EUR (B-13) |
| `language` | string (`it`/`en`) | both (detail required) | derived from selected provider's `language`; defaults to `it` |
| `project` | string ≤ 50 chars | both | required at create-time |
| `object` | string | both | "Oggetto" |
| `description` | string | both | "Descrizione" (internal note) |
| `note` | string | both | "Note" (sent to provider) |
| `notes` | string | preview-only | duplicate of `note`? (OpenAPI lists both `note` and `notes` on `rda-document-preview`) — **clarify**: list endpoint uses `notes`? |
| `total_price` | string | both | F-1: list view uses raw value; detail page strips trailing char (`slice(0,-1)`) |
| `created` / `creation_date` / `updated` | string (datetime) | both | detail has all three; preview has `created` + `updated` only |
| `provider_offer_code` | string | both | "Riferimento preventivo fornitore" |
| `provider_offer_date` | date | both | |
| `reference_warehouse` | string (default `MILANO`) | both | hidden in UI; always sent |
| `payment_method` | `{code, description}` | both | `code` is the joinable id (e.g. CDLAN default). Spec: required object on detail. |
| `requester` | `{id:int64, email}` | both | detail required |
| `provider` | `{id, company_name, erp_id?, state}` (+ `ref?`, `default_payment_method?` on detail) | both | detail uses `RdaProvider` |
| `budget` | `{id, name, year, cost_center?, budget_user_id?}` | both | mutex: either `cost_center` or `budget_user_id` is set per PO |
| `rows[]` | `PurchaseOrderRow[]` | detail | see §A.2.2 |
| `attachments[]` | `Attachment[]` | detail | see §A.2.3 |
| `approvers[]` | `[{user:{id?,email}, level:1\|2}]` | **audit-only** | not in OpenAPI; gates approve/reject buttons; preserve in 1:1 — must verify the live `GET /po/{id}` returns it |
| `recipients[]` | `ProviderReference[]` (likely shape `{id, …}`) | **audit-only** | not in OpenAPI; the table `Table4.defaultSelectedRowIndices` reads it; preserve |

**Constraints / business rules attached to the entity:**

- B-1 *Edit/delete only the requester's DRAFT POs* (UI-only guard; backend trust unknown).
- B-2 *3-quote rule:* `total_price ≥ 3000 €` ⇒ ≥ 2 attachments before submit.
- B-9 *Budget binding mutex:* exactly one of `cost_center` / `budget_user_id` per PO.
- B-10 *Default payment method* derivation: supplier default → CDLAN default → never the literal `"320"`.
- B-13 *Currency locked to EUR* (UI). API allows the field; the rewrite stays EUR-only.

### A.2.2 PurchaseOrderRow

Source: OpenAPI `rda-row` (read), `rda-row-create` (write), audit `02_page_po_details.md` § "Righe PO" / `mdl_edit_item`.

| Field | Type | Source | Notes |
|-------|------|--------|-------|
| `id` | int64 | read | |
| `purchase_order_id` | int64 | read | |
| `type` | enum `good` \| `service` | both | drives the entire form |
| `description` | string (HTML in UI) | both | line description |
| `requester_email` | string | both | |
| `product_code`, `product_description` | string | both | from `Article` |
| `qty` | int32 | both | |
| `price` | string | create-only | unit price for `good` |
| `montly_fee` (read) / `(no name)` | string | both | MRC for service; **note typo `montly_fee`** preserved |
| `activation_fee` (read) / `activation_price` (create) | string | both | NRC; **note name mismatch** between read and create payloads |
| `payment_detail` | nested object | both | see below |
| `renew_detail` | nested object | both | see below |
| `item_description` | string | create-only | extra free-text |

`payment_detail` (read): `{id, is_recurrent, start_pay_at_activation_date, start_at_date, month_recursion, created}`.
`payment_detail` (create): `{is_recurrent (default false), start_at: enum activation_date|specific_date|advance_payment, start_at_date (date), month_recursion: enum [1,3,6,12]}`.
> **F-shape mismatch:** the read endpoint uses `start_pay_at_activation_date: bool`; the create endpoint uses `start_at: enum`. Same data, different vocabularies. The 1:1 port must convert both directions at the boundary.

`renew_detail` (read & create-similar): `{id?, initial_subscription_months, next_subscription_months?, automatic_renew (default false), cancellation_advice (days)}`.

**Item economics rules (B-11):**
- `service`: at least one of `montly_fee` / `activation_fee` > 0; `initial_subscription_months` required; `month_recursion` required (1/3/6/12); if `automatic_renew == true` then `cancellation_advice` required.
- `good`: only `price` and `qty` (+ `start_at`); MRC/duration ignored.

### A.2.3 PurchaseOrderAttachment

Source: OpenAPI `po-attachment-get` (read), `po-attachment-upload` (write).

| Field | Type | Notes |
|-------|------|-------|
| `id` | int | |
| `attachment_type` | enum `quote` \| `transport_document` \| `other` | **B-3 auto-tag rule** (DRAFT → `quote`; else → `transport_document`). The `other` value exists on the API but is not produced by the current UI. |
| `file_id` | int | |
| `file_name` | string | |
| `created_at`, `updated_at` | datetime | |

### A.2.4 PurchaseOrderComment

Source: audit only (`/po/{id}/comment` schema not detailed in OpenAPI for the read shape; only operationId is exposed).

Observed read shape (from binding `GetComments.data`):

```
[ { id,
    user: { id, first_name, last_name, email },
    comment | comment_text,             // ← UI handles either field name
    created_at,
    replies: [ { same shape, no further replies } ]
  }, ... ]
```

Observed write shape: `{ comment: string }` (and that's all today — see F-4 / S-3).

**Open from audit:** OpenAPI presence of `mentioned_user_ids[]` on `POST /comment` — confirm via the spec body schema (line 5020 onward in `docs/mistra-dist.yaml`). If the API accepts it, the rewrite should send it; if not, the @-mention UI is cosmetic.

### A.2.5 PurchaseOrderRecipient (association)

Modelled implicitly by `PATCH /po/{id}` body `recipient_ids: int[]`. Read view exposes `GetPoDetails.data.recipients[]` shape that the UI treats as a `ProviderReference[]` subset (it pre-selects `Table4` rows whose `id` matches). **OpenAPI does not declare `recipients`** in `rda-document-detail` — gap to confirm.

**Rule B-5:** if `recipient_ids == []`, the backend uses the provider's `QUALIFICATION_REF` as the order recipient.

### A.2.6 PurchaseOrderApprover (association)

Modelled implicitly: detail page reads `GetPoDetails.data.approvers[*].user.email` and `…level` (`'1'` or `'2'`, **stored as string**). **OpenAPI does not declare `approvers`** in `rda-document-detail` — gap to confirm.

**Rule B-7:** approve/reject buttons are visible only if `current_user.email ∈ approvers[*].user.email`.

### A.2.7 Provider

Source: provider-qualification endpoints (already used by the *Fornitori* mini-app — recently scaffolded under `apps/fornitori/`). Reused as-is.

Fields used by RDA: `id`, `company_name`, `vat_number`, `cf`, `address`, `city`, `province`, `country`, `postal_code`, `language`, `default_payment_method:{code,description}`, `refs[]`, `state` (qualification state).

**Reuse note:** the new Go module should treat the provider listing/lookup as a *cross-app shared concern*, not duplicate it. If `backend/internal/fornitori/` (work in progress) already exposes the `/api/fornitori/...` proxies, decide whether RDA proxies independently or reuses (see Phase D).

### A.2.8 ProviderReference

Fields: `id, first_name, last_name, email, phone (≥ E.164 5-digit), reference_type ∈ {OTHER_REF, ADMINISTRATIVE_REF, TECHNICAL_REF, QUALIFICATION_REF}`.
Rules:
- B-12: `QUALIFICATION_REF` is *read-only* (not editable inline; not deletable; not addable).
- PATCH semantics (Contact.updateContact PO Details): empty `phone` clears, empty `email` is *not sent* (preserved). Carry through.

### A.2.9 Budget

Read-only per-user list: `[{ budget_id, name, year, cost_center?, user_id?, user_email? }]`. **F-2:** the audit notes the field is sometimes called `id` and sometimes `budget_id` between the source and default-value bindings. Pick one in the React form (`budget_id` is the OpenAPI canonical for `RdaBudget`).

### A.2.10 PaymentMethod (catalog)

Currently read directly from PostgreSQL. The 1:1 port preserves the *behaviour* but **must not** preserve direct DB access from the client (S-1). Two routes for v1:
1. Have the Go module read from the same `provider_qualifications.payment_method` table (the `arak_db` / new) and serve `/api/rda/v1/payment-methods` + `/payment-methods/default`.
2. If a Mistra REST equivalent exists or is exposed by the provider-qualification module, proxy through Arak.

→ resolve in **Phase D** (Integrations).

Fields used: `code` (joinable id), `description` (label), `rda_available` (filter true). Default record: `payment_method_default_cdlan.payment_method_code` (today returns CDLAN's `BB60ggFM+10`). The literal `"320"` from the legacy app **does not survive**.

### A.2.11 Article / ItemType (catalog)

Source: `GET /arak/rda/v1/article?type=service|good`. Fields used: `code`, `description`. Used only inside the item modal (`sl_product`).

### A.2.12 User (search-only)

Source: `GET /arak/users-int/v1/user?search_string=…&enabled=true`. Used only by the comment-mentions feature.

### A.2.13 UserPermissions

Today: client-side SQL on `users_int.role`. New app: derive **on the backend** from the trusted Keycloak token (per `CLAUDE.md` § "Keycloak Roles"). Concretely the four flags currently used become four boolean roles (proposed; final naming for **Phase D** review):

- `is_approver`              → `app_rda_approver_l1l2`
- `is_afc`                   → `app_rda_approver_afc`
- `is_approver_no_leasing`   → `app_rda_approver_no_leasing`
- `is_approver_extra_budget` → `app_rda_approver_extra_budget`

App-level access: `app_rda_access` (per the New App Checklist convention).

A `GET /api/rda/v1/me/permissions` endpoint (returning the four booleans) **is the minimum needed** to drive the UI — even if it's just a thin wrapper over Keycloak claims, it isolates the change from the UI and is trivial to test.

---

## A.3 State machine (PO state)

Extracted verbatim from `LabelJs.stateMap` (PO Details) and reconciled with action availability in `02_page_po_details.md` and the action-bar table.

```
DRAFT
  └─(submit)──> PENDING_APPROVAL_PROVIDER (server)
                 │
                 ├─ PENDING_APPROVAL  (lvl 1 then lvl 2)
                 │     ├─(approve all)─> PENDING_APPROVAL_PAYMENT_METHOD ─approve─┐
                 │     ├─(approve all)─> PENDING_LEASING ─approve─┐               │
                 │     ├─(reject) ─────> REJECTED                 │               │
                 │     └─                                         ├─> PENDING_BUDGET_INCREMENT_CHECK ─> PENDING_BUDGET_INCREMENT
                 ├─ PENDING_APPROVAL_NO_LEASING (after reject leasing)
                 ├─ PENDING_BUDGET_SUBTRACTION
                 ├─ PENDING_PROVIDER_SAVED_IN_ALYANTE
                 ├─ PENDING_PDF_GENERATION
                 ├─ PENDING_ERP_SAVE
                 ├─ PENDING_LEASING_ORDER_CREATION ─(LeasingIsCreated)─> PENDING_SEND
                 ├─ PENDING_SEND ─(SentToFornitore)─> CLOSED / PENDING_VERIFICATION
                 ├─ PENDING_VERIFICATION ─(ConfirmConformity)─> DELIVERED_AND_COMPLIANT
                 │                       └─(RejectConformity)─> PENDING_DISPUTE
                 ├─ PENDING_CHECK_DOCUMENT
                 └─ CANCELED
SUBMITTED ────── (alias for "submitted to backend, server-side routing")
```

**Italian labels** (from `stateMap`, including the verbatim 3-T typo): see appendix at the bottom of this file.

**Allowed user actions per state** (drawn from `02_page_po_details.md` action-bar table) — collected once here so Phase B/C can reference it:

| State | Available action(s) | Permission |
|-------|---------------------|------------|
| `DRAFT` | edit header & rows; upload quote(s); set recipients; "Aggiorna Bozza"; "Manda PO in Approvazione" | requester only |
| `PENDING_APPROVAL` (lvl=1) | "Approva (Liv 1)" / "Rifiuta (Liv 1)" | `is_approver` AND email ∈ `approvers` |
| `PENDING_APPROVAL` (lvl=2) | "Approva (Liv 2)" / "Rifiuta (Liv 2)" | same |
| `PENDING_APPROVAL_PAYMENT_METHOD` | "Aggiorna metodo di pagamento" (req.) / "Approva pagamento" / "Rifiuta pagamento" | requester for update; `is_afc` for approve/reject |
| `PENDING_LEASING` | "Approva leasing" / "Rifiuta leasing" | `is_afc` |
| `PENDING_APPROVAL_NO_LEASING` | "Approva no leasing" / **"Rifiuta no leasing"** (currently calls plain `/po/{id}/reject` — F-6) | `is_approver_no_leasing` |
| `PENDING_BUDGET_INCREMENT` | "Approva incremento budget" / "Rifiuta incremento budget" (with `increment_promise` query param) | `is_approver_extra_budget` |
| `PENDING_LEASING_ORDER_CREATION` | "Leasing Creato" | `is_afc` |
| `PENDING_SEND` | "Invia ordine al fornitore" | (see Q-A2) |
| `PENDING_VERIFICATION` | "Erogato e conforme" / "In contestazione"; upload DDT | (see Q-A3) |
| any non-DRAFT | "Genera PDF" | (everyone who can read) |
| any | "Chiudi" / back to RDA list | everyone |

---

## A.4 Field-level fragility checklist (carried 1:1 unless flagged)

These are observed quirks in the **current** API. The 1:1 port preserves them at the API boundary; the React form normalises them inside `apps/rda/src/api/`.

| ID | Quirk | 1:1 handling |
|----|-------|--------------|
| F-1 | `total_price` returned with trailing currency suffix on detail page (the legacy code does `.slice(0,-1)`). | Preserve client-side strip; document. (Best fix lives on Mistra API; out of scope.) |
| F-2 | Budget `id` vs `budget_id` mix-up on default selection. | Use `budget_id` as canonical key (per OpenAPI). |
| F-4 | `PostComment` body emits `{{Input2.text}}` un-encoded → quotes/newlines break. | New client always JSON-encodes. |
| F-5 | "Edit row" reuses POST → duplicates row. | The new UI **disables row-edit until** the backend exposes `PUT /row/{id}` (M-6). Until then, edit = delete + add (explicit in the UI). |
| F-7 | `EditPO` body always includes header text fields with `||` truthiness bug. | New client sends only fields the user actually changed (PATCH). |
| typo | `montly_fee` (read) vs no equivalent in create body | Type the contract verbatim; do not silently rename. |
| typo | `IN ATTTESA VERIFICA CONFORMITA'` (3 T's) in state label | **See Q-A1.** |

---

## A.5 Operations matrix

For each entity, the verbs the new app must expose to the UI. Verbs map 1:1 to current Mistra endpoints unless noted.

| Entity | List | Read | Create | Update | Delete | Domain verbs |
|--------|:--:|:--:|:--:|:--:|:--:|---|
| PurchaseOrder | ✓ (mine) + 5 inboxes | ✓ | ✓ | ✓ (PATCH header) | ✓ (DRAFT only) | submit, approve {l1l2 / leasing / no-leasing / pm / budget-incr}, reject {…}, send-to-provider, generate-pdf, confirm-conformity, reject-conformity, leasing-created, update-payment-method |
| PurchaseOrderRow | (embedded) | (embedded) | ✓ | ✗ today | ✓ | — |
| Attachment | (embedded) | download | ✓ (multipart) | ✗ | ✓ | (auto-tag rule) |
| Comment | ✓ | ✗ | ✓ | ✗ | ✗ | search-mention-users |
| Provider | ✓ | ✓ | ✓ (`draft`) | ✗ | ✗ | — |
| ProviderReference | (embedded in provider) | ✗ | ✓ | ✓ | ✗ | — |
| Budget | ✓ (mine) | ✗ | ✗ | ✗ | ✗ | — |
| PaymentMethod | ✓ | default | ✗ | ✗ | ✗ | — |
| Article | ✓ | ✗ | ✗ | ✗ | ✗ | — |
| User | search | ✗ | ✗ | ✗ | ✗ | — |
| UserPermissions | (singleton "me") | ✓ | ✗ | ✗ | ✗ | — |

---

## A.6 Open questions for the expert

Numbered for cross-reference. Only items the audit cannot answer.

| # | Question | Why we need an answer | Default if no answer |
|---|----------|------------------------|----------------------|
| **Q-A1** | Keep the typo `IN ATTTESA VERIFICA CONFORMITA'` in `PENDING_VERIFICATION` Italian label, or fix it (`IN ATTESA VERIFICA CONFORMITÀ`)? | This is user-visible copy; either choice is "1:1" interpretable. | **Fix it.** The state code stays the same. |
| **Q-A2** | "Invia ordine al fornitore" (`POST /po/{id}/send-to-provider`) is gated only by `state == PENDING_SEND` in the source — no role check at all. Is this intentional, or should it be limited (e.g. AFC, requester)? | Determines whether the new UI applies a permission gate. | Mirror the source: state-only gate. |
| **Q-A3** | Conformity actions (`confirm-conformity`, `reject-conformity`) on `PENDING_VERIFICATION` — who is allowed? The current UI shows the buttons to anyone reading the page in that state (no role binding). Real users must include the requester at minimum; possibly a "warehouse" or "AFC" role. | Same as Q-A2. | Mirror source: state-only; backend trusted. |
| **Q-A4** | The `recipients` and `approvers` arrays are **not declared** in OpenAPI `rda-document-detail` but the UI relies on them. Do we have permission to ask the Arak/Mistra team to update the spec, or do we treat them as "documented only by usage" and add Go types matching the audit? | Determines whether v1 ships with a contract patch or just internal types. | Add internal Go/TS types matching the audit; flag the spec gap in `docs/TODO.md`. |
| **Q-A5** | The `comment_text` vs `comment` field-name inconsistency in the comment list — confirm with backend whether the canonical name is `comment` (write) or `comment_text` (read), or whether responses really do toggle. | Affects DTOs in the new app. | Send `comment` (write); accept either on read; surface as `comment` in TS. |
| **Q-A6** | The `mentioned_user_ids[]` payload for `POST /comment` — does the Mistra API already accept it? (See line 5020 of `mistra-dist.yaml`.) If yes, also: does the backend dispatch a notification (email / portal banner) to the mentioned user? If neither: do we want to **drop** the @-mention UI in v1 since today it is cosmetic only (S-3)? | Drives whether @-mentions are real or theatre. | If endpoint doesn't accept it: keep UI but submit a backend change request; surface "Mentions not yet supported" caption. |
| **Q-A7** | Edit-row path (F-5 / M-6). Three options: (a) hide the "Modifica riga" pencil icon entirely until backend supports `PUT /row/{id}`; (b) replace the pencil with an explicit "Sostituisci riga" that does delete + create; (c) ship the broken behaviour (creates duplicate). | UX impact is large. | Default to **(a)** — hide; surface the limitation in `docs/TODO.md`. |
| **Q-A8** | The `select_motivation` / `input_motivazion` fields (motivazione esclusione 3-preventivi: *Accordo quadro*, *Vendor specificato*, *Altro*) are present in the legacy UI but **wired to nothing**. Does the business want this feature in v1, defer it, or drop it? | Optional UI scope. | **Drop in v1.** Track in `docs/TODO.md`. |
| **Q-A9** | The leasing-rejection branch: "Rifiuta leasing" today calls the dedicated `/po/{id}/leasing/reject`; "Rifiuta no leasing" calls plain `/po/{id}/reject` (F-6). Is that the canonical workflow, or should "Rifiuta no leasing" call a `/no-leasing/reject` endpoint that does not exist today? | Determines whether v1 needs a backend addition. | Mirror current: no-leasing reject → `/reject`. |
| **Q-A10** | Budget-increment list endpoint URL inconsistency `/po-pending-budget-increment` (no `/po/...` segment). Carry it through 1:1, or push the backend team to normalise to `/po/pending-budget-increment` first? | Cosmetic API consistency vs migration timing. | **Carry through 1:1.** v1 uses the URL as-is; backend hygiene is a follow-up. |
| **Q-A11** | The 3-quote rule (B-2): is `≥ 3000 €` the right threshold today, and is it an *attachment count* rule or a *quote-typed-attachment count* rule? The legacy logic counts all `attachments[]` — but if a DDT (transport doc) is mistakenly added in DRAFT, does it count? | Tightens the validation. | **Count only `attachment_type == 'quote'`** in the new UI. |
| **Q-A12** | Default reference warehouse: is `MILANO` still correct for everybody, or do we need a per-user / per-budget default? | Drives whether the field should appear in the UI at all. | Keep hidden, default to `MILANO`. |
| **Q-A13** | Do we need `STANDARD` vs `ECOMMERCE` exposed as a user-visible toggle, or is `ECOMMERCE` deprecated? The legacy UI shows the SELECT and defaults to `STANDARD`. | Drives a header-form widget. | Keep the toggle, default `STANDARD`. |

---

## A.7 What changes vs. legacy at the *entity* level (already settled by user instruction or by `CLAUDE.md`)

Recorded here so they don't re-surface as "questions":

- **Permissions:** `users_int.user/role` SQL → Keycloak roles + `GET /me/permissions` (S-1, M-1).
- **No client-side DB:** all `arak_db` queries become backend reads or REST proxies (S-1).
- **No header trust:** drop `Requester-Email` header; the backend infers it from the token (S-2).
- **State labels:** single shared module, no 4-way duplication (D-1).
- **Total per row:** backend-supplied; the two divergent client formulas are dropped (`TotalCalculator` vs `Text17`).
- **PDF / attachment download:** backend returns either signed URL or streamed file; no client-side base64 juggling.
- **Recipients HTML rendering:** React component, not `setText('<span>...')` — closes XSS vector F-10.
- **Comments JSON-encoding fix:** F-4 closed at the new client.
- **Hard-coded `"320"` payment method:** removed (B-10).
- **Dead pages / modals:** `Home`, `Modal1`, `mdl_supplierContact`, hidden duplicate containers — not ported.

---

## Appendix — Italian state-label map (verbatim from `LabelJs.stateMap`)

```
DRAFT                                 → "BOZZA"
SUBMITTED                             → "CONFERMATO"
CANCELED                              → "CANCELLATO"
PENDING_CHECK_DOCUMENT                → "IN ATTESA VERIFICA PREVENTIVI"
PENDING_APPROVAL_PROVIDER             → "VERIFICA QUALIFICA"
PENDING_APPROVAL_PAYMENT_METHOD       → "IN ATTESA VERIFICA METODO PAGAMENTO"
PENDING_APPROVAL                      → "IN APPROVAZIONE"
REJECTED                              → "RIFIUTATO"
PENDING_LEASING                       → "IN ATTESA VERIFICA LEASING"
PENDING_APPROVAL_NO_LEASING           → "IN ATTESA APPROVAZIONE NO-LEASING"
PENDING_CONTRACT_VERIFICATION         → "IN ATTESA VERIFICA CONTRATTO"
PENDING_BUDGET_INCREMENT_CHECK        → "IN ATTESA INCREMENTO BUDGET"
PENDING_BUDGET_INCREMENT              → "AUMENTO BUDGET"
PENDING_BUDGET_SUBTRACTION            → "SCALO BUDGET"
PENDING_PROVIDER_SAVED_IN_ALYANTE     → "CHECK CENSIMENTO FORNITORE"
PENDING_PDF_GENERATION                → "IN ATTESA GENERAZIONE PDF"
PENDING_ERP_SAVE                      → "SALVATAGGIO ERP"
PENDING_LEASING_ORDER_CREATION        → "IN ATTESA CREAZIONE ORDINE LEASING"
PENDING_SEND                          → "IN ATTESA INVIO FORNITORE"
CLOSED                                → "CHIUSO"
PENDING_VERIFICATION                  → "IN ATTTESA VERIFICA CONFORMITA'"   ← Q-A1
PENDING_DISPUTE                       → "IN CONTESTAZIONE"
DELIVERED_AND_COMPLIANT               → "EROGATO E CONFORME"
```

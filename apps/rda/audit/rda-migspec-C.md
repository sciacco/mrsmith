# RDA migration spec — Phase C: Logic Placement

**Source:** audit `06_jsobject_methods.md` + bindings cross-referenced against the export.
**Constraint locked by user (Phase A):** *1:1 porting against the existing Mistra `/arak/rda/...` API surface*. No new Mistra endpoints in v1; the legacy bodies are kept verbatim at the wire level, but their **construction moves out of the browser** into the Go module.

This phase classifies every piece of non-trivial logic into one of four buckets and explains the rationale.

| Bucket | Where it lives | When to put logic here |
|--------|----------------|------------------------|
| **BE-go** | `backend/internal/rda/` (the new Go module) | (a) request-body shaping for Mistra; (b) auth/permission derivation from token; (c) replacing `arak_db` SQL; (d) anything the legacy code did *between* the user click and the network call. |
| **BE-mistra** | already on Mistra (we just call it) | Domain rules already enforced server-side: PO state transitions, total computation, approver assignment, conformity checks. **Not our code in v1.** |
| **FE-shared** | a shared module under `apps/rda/src/lib/` | Pure transforms, label maps, format helpers; *no* network calls. |
| **FE-orchestration** | React components/hooks under `apps/rda/src/` | UI glue: open dialogs, route, store form state, run query, show toast. |
| **DROP** | — | Dead code / scaffolds in the legacy DSL. |

The Go module is a **thin proxy** in v1: each handler translates the OIDC-authenticated request into a Mistra call via `backend/internal/platform/arak`. The point of a "backend" verb is *not* to re-implement business rules — it's to (i) replace the trusted `Requester-Email` header with a token-derived email, (ii) replace the client-side `users_int.role` SQL with a token-claim derivation, and (iii) construct the legacy IIFE bodies safely in Go.

---

## C.1 Method-by-method placement

Order follows `06_jsobject_methods.md`.

### C.1.1 State-label modules — `LabelJs`, `LabelsJS`, `JSObject1` (×4 copies)

Functions: `stateMap` table, `stateLabel(state)`, `translate(key)`.

| Place | Why |
|-------|-----|
| **FE-shared** → `apps/rda/src/lib/state-labels.ts` | Pure presentation; no business effect. Single module replaces 4 copies (D-1) and fills the 4 missing keys in two of the legacy copies (`PENDING_VERIFICATION`, `PENDING_DISPUTE`, `DELIVERED_AND_COMPLIANT`). Q-A1 fix applied (`IN ATTESA VERIFICA CONFORMITÀ`). |

API: `stateLabel(s: PoState): string` + `PO_STATES` typed enum (mirrors backend strings).

---

### C.1.2 `Utils` (RDA page) — 4 methods

| Method | Bucket | Notes |
|--------|--------|-------|
| `newProvider()` | **FE-orchestration** | Reveals the inline new-supplier sub-form; React state toggle. |
| `extractApproverList(approverList)` | **FE-shared** → `apps/rda/src/lib/format.ts` | Pure transform `[{email, level}] → "a@cdlan.it (1), b@cdlan.it (2)"`. Reused in `PoListTable` "Approvatori" column and possibly in PO header `Text23`. |
| `newProviderAdd()` (the 13-step `if/else` validation chain) | **FE-orchestration** (form schema) **+ BE-go** (server-side validation) | Frontend uses a single form schema (Zod or equivalent) with all the rules listed in B.2.S3c so users see *all* errors at once instead of sequential `showAlert`s. The Go module **also** validates before forwarding to Mistra (defence in depth). The actual create call is `POST /api/rda/v1/providers/draft` → BE-go → `arak.POST /arak/provider-qualification/v1/provider/draft`. |
| `newRdaCreate()` (validate → `NewPo.run()` → close modal → refresh list → `navigateTo`) | **BE-go** (body shaping) **+ FE-orchestration** (UI sequence) | The IIFE that builds the `NewPo` body with mutex `cost_center`/`budget_user_id`, supplier-default fallback, etc., **moves to the Go module** so the browser does not own the body shape. Frontend keeps the post-create UI flow (close → refresh → navigate). |

---

### C.1.3 `Utils` (App. incremento Budget) — 3 methods

`myFun1`/`myFun2` empty stubs → **DROP**. `extractApproverList` → already covered by C.1.2 (single shared copy).

---

### C.1.4 `Contact` (RDA page) — 8 methods (subset of PO Details `Contact`)

Consolidate with C.1.5; the RDA-page copy adds nothing the PO Details copy doesn't already have. **DROP after consolidation.**

---

### C.1.5 `Contact` (PO Details page) — 13 methods

| Method | Bucket | Notes |
|--------|--------|-------|
| `allCategory` / `availableCategory` (option lists for `reference_type`) | **FE-shared** → `apps/rda/src/lib/provider-refs.ts` | Pure constants + B-12 rule (QUALIFICATION_REF excluded from new-row options). |
| `getLabel(value)` | **FE-shared** | One-liner over the same constants. |
| `initializeContacts()` / `visbilityContactBox(visibility)` | **FE-orchestration** | UI state toggle; absorbed by the `ProviderRefTable` component. |
| `emptyContacts(poid, providerId)` | **FE-orchestration** | When the user changes supplier on a DRAFT PO, the new app calls `PATCH /api/rda/v1/pos/{id}` with `{provider_id, recipient_ids: []}`. The "clearing on supplier change" rule is a *frontend* effect; backend just accepts the patch. |
| `updateContactList(poid)` | **FE-orchestration** | `Table4.selectedRows.map(r => r.id)` + `PATCH /pos/{id}` with `recipient_ids`. |
| `checkMailIsPresent(approvers)` | **FE-shared** + **BE-go** | Frontend uses it to *display* the approve/reject buttons (B-7). Backend re-checks on the approve/reject endpoints — not because Mistra doesn't already do it (it does), but because the Go module should fail fast with a clean 403 before the proxy hop. |
| `addContact(providerId, …)` | **FE-orchestration** | Builds body, calls `POST /api/rda/v1/providers/{id}/refs`, refreshes provider detail. |
| `updateContact(providerId, refId, …)` (the asymmetric "empty phone clears, empty email preserves" semantics) | **BE-go** | Today this asymmetric body-building lives in the browser. The Go module should encapsulate it: `PUT /api/rda/v1/providers/{id}/refs/{refId}` accepts the new field set, decides per-field which to forward, then proxies. The asymmetry is preserved at the Mistra wire level — only the *origin* of the asymmetric body moves. |
| `loadingData()` (renders HTML into `Text27`) | **FE-orchestration** (re-implemented as React render) | F-10 (XSS) closed by definition. The "use QUALIFICATION_REF when recipients is empty" caption (B-5) is just text rendering; the *behaviour* is server-side at Mistra. |
| `storeSelectedSupplier(id)`, `storeSelectedContact(contacts)` | **FE-orchestration** | Form state. |
| `getContacts(providerId)`, `getProviderData(providerId)` | **FE-orchestration** | Data loaders (`GET /providers/{id}` + nested `refs`). |
| `myVar1`, `myVar2`, `myFun1`, `myFun2` | **DROP** | Stubs. |

---

### C.1.6 `utils` (lowercase u, PO Details) — 5 methods

All five reference Appsmith actions that **don't exist** (`upd_po_item`, `ins_po_item`, `get_list_items`, the wrong tab id `'Items'`):

| Method | Bucket |
|--------|--------|
| `save_item_row()` | **DROP** (dead — references nonexistent actions; explains F-5) |
| `savePoDraft()` | **DROP** (not invoked) |
| `delete_item_row()` | **DROP** (empty) |
| `tab_details_action()` | **DROP** (refers to nonexistent tab id and action) |
| `dateConverter(input)` | **FE-shared** → `apps/rda/src/lib/format.ts` (or use a date library) |

---

### C.1.7 `JSObject1` (PO Details, mentions) — 6 methods

| Method | Bucket | Notes |
|--------|--------|-------|
| `mentionQuery`, `showMentions`, `search_string`, `mentionedUsers` (state) | **FE-orchestration** | Local component state inside `MentionInput`. |
| `handleInputChange(text)` | **FE-orchestration** | Detect `@…` token, run `GET /api/rda/v1/users?search=…`. |
| `insertMention(user)` | **FE-orchestration** | String replace + push to `mentionedUsers`. |
| `extractSearchTextAfterAt(text)` | **FE-shared** | Pure helper. |
| `getMentionedUserIds()` | **FE-shared** (Q-A6: cosmetic only) | Computed but **not sent** in v1 (mirrors legacy bug). Tracked in `docs/TODO.md` for v2. |
| `resetMentions()` | **FE-orchestration** | Clear local state on submit / cancel. |

---

### C.1.8 `TotalCalculator` (PO Details) — 1 method

| Method | Bucket | Notes |
|--------|--------|-------|
| `getTotal(type, unit_price, qta, activation_price, …)` | **DROP** | Mistra returns `total_price` on the PO and a per-row total inside `rows[*]` (per the OpenAPI `rda-row` schema's read fields and the legacy `Table2.total` column which the new app re-binds to backend value). The two divergent client formulas are not ported. The item-modal "preview" total (used while typing in `mdl_edit_item`, before save) **is** kept frontend-side — it's a UX preview, not a source of truth. |

Note: the modal preview is the only client-side total that survives. It uses **the `service`/`good` formula already in `Text17`** (not `TotalCalculator`'s formula) and is purely informational. Final per-row total reads from the PATCHed/POSTed Mistra response.

---

### C.1.9 `PDFGenerator` (PO Details) — 1 method

| Method | Bucket | Notes |
|--------|--------|-------|
| `downloadPOPDF()` (decodes base64 OR raw bytes, builds Blob, triggers download) | **BE-go** | New flow: `GET /api/rda/v1/pos/{id}/pdf` returns either a redirect to a signed URL or streams the file. The browser does a plain `window.location` / `<a download>` — no base64 juggling. |

---

### C.1.10 `attachmentsJs` (PO Details) — 2 methods

| Method | Bucket | Notes |
|--------|--------|-------|
| `uploadPreventivi()` (loops over picker files; auto-tags `quote`/`transport_document` based on PO state) | **BE-go** (auto-tag rule) **+ FE-orchestration** (multipart submit, refresh) | Per B-3 the tag is derived from the *current PO state*. The frontend submits a multipart upload to `POST /api/rda/v1/pos/{id}/attachments`; the Go module reads the current PO state from Mistra and assigns `attachment_type` accordingly before forwarding. The browser never decides the tag. |
| `downloadAttachment(row)` (same base64/raw mishmash as PDFGenerator) | **BE-go** | New: `GET /api/rda/v1/pos/{id}/attachments/{aid}` returns a redirect to a signed URL; frontend opens it. |

---

### C.1.11 `ContactsHelper` (PO Details) — 6 methods

All dead per audit. **DROP entirely.** No port.

---

### C.1.12 `userFunctions.idUser()` (PO Details)

| Method | Bucket | Notes |
|--------|--------|-------|
| `idUser()` (await `userID.run()`; return `userID.data[0].id`) | **DROP** | Replaced by `GET /api/rda/v1/me` (or by including the ID in `/me/permissions`). The OIDC token already identifies the user; the Go module derives email; the user numeric id (used only by the comments feature today) is fetched once on app load. |

---

### C.1.13 `JSObject3`, `JSObject4`, `MemoryManager` (PO Details)

Empty / generic helpers. **DROP.**

---

### C.1.14 Per-page query duplication

`CallBudget`, `ListaFornitori`, `PaymentMethonds`, `GetDefaultPaymentMethod` — declared on both RDA and PO Details (D-5).

| Bucket | Notes |
|--------|-------|
| **FE-shared** (`packages/api-client` adds RDA endpoints) **+ FE-orchestration** (React Query / SWR cache) | One typed client function per endpoint; the cache key is the same across views, so visiting `/rda` then `/rda/po/:id` does not re-fetch the budgets/providers/methods. |

---

## C.2 What the Go module *adds* on top of pure proxying

Because the new app must remove client-side trust (S-1, S-2) without changing the Mistra contract, the Go module is responsible for the following pieces of logic that the browser used to own:

| # | Responsibility | Today (Appsmith) | Tomorrow (Go module) |
|---|----------------|-------------------|-----------------------|
| 1 | Caller email | `Requester-Email: {{appsmith.user.email}}` (client-supplied) | Read OIDC `email` claim; forward as `Requester-Email` to Mistra |
| 2 | Permission flags | SQL `SELECT r.is_afc, r.is_approver, … FROM users_int.user u JOIN users_int.role r WHERE u.email = '<input>'` | Map four Keycloak roles (`app_rda_approver_l1l2`, `app_rda_approver_afc`, `app_rda_approver_no_leasing`, `app_rda_approver_extra_budget`) into the same JSON shape, expose as `GET /api/rda/v1/me/permissions` |
| 3 | `NewPo` body shaping | Inline IIFE in a button handler; mutex `cost_center` ↔ `budget_user_id`; supplier-default → CDLAN-default fallback; **literal `"320"` fallback** | Single Go function that takes a normalised request and emits the same Mistra body without the `"320"` literal |
| 4 | `EditPO` body shaping | Inline IIFE; F-7 truthiness bug overwrites empty fields | Go function constructs the PATCH body from a normalised partial request — only fields present are forwarded |
| 5 | `CreateItemRow` body shaping | Inline IIFE building `payment_detail` / `renew_detail` / `start_at_date` formatting | Go function with explicit DTOs; bridges the read/write field-name mismatch (`start_pay_at_activation_date` ↔ `start_at`) |
| 6 | Attachment auto-tag | `attachment_type = state == 'DRAFT' ? 'quote' : 'transport_document'` decided in JSObject | Go module fetches current PO state from Mistra (or trusts the multipart caller's state hint with re-validation), then sets the tag |
| 7 | Comment write contract | `{ comment: Input2.text }` un-encoded JSON | Go module accepts `{comment: string}` (and `mentioned_user_ids?: int64[]` accepted but ignored in v1, to avoid TS-side breakage when v2 wires it up) |
| 8 | PDF / file download | base64-or-bytes branching client-side | Go module returns a redirect to a signed URL (or streams) |
| 9 | Provider-ref PATCH semantics | Asymmetric body in `Contact.updateContact` (empty phone clears, empty email preserves) | Same asymmetric body produced server-side from a structured request |
| 10 | Inbox URL anomaly | Mixed `/po/...` and `/po-pending-budget-increment` | Go exposes uniform `/api/rda/v1/pos/inbox/budget-increment` and forwards to the legacy URL internally (per Q-A10, 1:1 at the wire to Mistra; clean at our public API) |

---

## C.3 Suggested REST surface (refined from audit `05_datasource_catalog.md`)

This is the public surface of the Go module. Internal calls go through the existing `backend/internal/platform/arak` client. The path style follows `docs/API-CONVENTIONS.md` (`/api/rda/v1/...` public, module code under `/rda/v1/...`).

```text
# Identity
GET    /api/rda/v1/me/permissions      → {is_approver,is_afc,is_approver_no_leasing,is_approver_extra_budget}

# Catalogs
GET    /api/rda/v1/budgets             → mine
GET    /api/rda/v1/payment-methods     → rda_available=true
GET    /api/rda/v1/payment-methods/default
GET    /api/rda/v1/articles?type=...
GET    /api/rda/v1/users?search=...    → mention search

# PO list & inboxes
GET    /api/rda/v1/pos                 → mine (list)
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
# (no PUT in v1 — Q-A7; tracked in docs/TODO.md as M-6)

# Attachments
POST   /api/rda/v1/pos/{id}/attachments    (multipart; backend auto-tags)
GET    /api/rda/v1/pos/{id}/attachments/{aid}    → 302 to signed URL
DELETE /api/rda/v1/pos/{id}/attachments/{aid}

# Comments
GET    /api/rda/v1/pos/{id}/comments
POST   /api/rda/v1/pos/{id}/comments       body {comment, mentioned_user_ids?}  (IDs ignored in v1)

# Providers (proxied to provider-qualification module)
GET    /api/rda/v1/providers?usable=true
POST   /api/rda/v1/providers/draft
GET    /api/rda/v1/providers/{id}
POST   /api/rda/v1/providers/{id}/refs
PUT    /api/rda/v1/providers/{id}/refs/{refId}
```

> Whether `/providers` proxies through this module or reuses the in-progress `apps/fornitori` / `backend/internal/fornitori` is decided in **Phase D** (cross-module ownership).

---

## C.4 Validation logic — split

Validation appears in three different places in the legacy code. The new layout assigns each to its natural owner:

| Rule | Today | New owner |
|------|-------|-----------|
| Required fields in "Nuova richiesta" (project, object, payment method) | sequential `showAlert` chain in `Utils.newRdaCreate` | **FE form schema** (one-shot validation) + **BE-go** echo (defence in depth) |
| New-supplier required fields (Azienda, Address, Citta, Paese, CAP≥5 if IT, etc.) | `Utils.newProviderAdd` chain | **FE form schema** + **BE-go** echo |
| 3-quote rule for submit (B-2) | `btn_save_draft.isDisabled` derived from `total_price ≥ 3000 && attachments.length < 2` | **FE** disables button; **BE-go** re-checks on `POST /pos/{id}/submit` |
| State+role gating of action-bar buttons | `isDisabled` on each button + `Contact.checkMailIsPresent` | **FE** computes a derived `permissions` object; **BE-go** enforces on each transition endpoint (per C.2#10) |
| Item modal "service requires NRC or MRC > 0", auto-renew → cancellation_advice required, etc. | inline `isRequired` + `Save.isDisabled` rules | **FE form schema** (item form) + **BE-go** echo on `POST /pos/{id}/rows` |
| Field-level PATCH semantics (empty-clears vs empty-preserves) | `Contact.updateContact` body construction | **BE-go** alone — frontend sends only the fields the user changed |

---

## C.5 What stays at Mistra (untouched in v1)

These are listed so a reviewer doesn't accidentally re-implement them in Go:

- **State machine transitions** (DRAFT → PENDING_APPROVAL_PROVIDER → … → CLOSED).
- **Multi-level approval routing** (level 1 → level 2 sequencing).
- **Approver-list assignment per PO** (which users go into `approvers[]`).
- **Conformity / DDT requirement check** (Mistra returns the error toast surfaced in B-4).
- **Total price computation** (server returns final `total_price` and per-row totals).
- **PDF generation** (Mistra `/po/{id}/download`).
- **Email notifications** (if any) — assumed Mistra-side.
- **`Requester-Email` filter semantics** for inbox endpoints (the header names the *caller* and Mistra returns POs awaiting that user's action).

---

## C.6 Summary placement table

For quick reference. "X" means "primary owner".

| Concern | BE-go | BE-mistra | FE-shared | FE-orch | DROP |
|---------|:-:|:-:|:-:|:-:|:-:|
| State labels (Italian) |   |   | X |   |   |
| `extractApproverList` |   |   | X |   |   |
| `dateConverter` |   |   | X |   |   |
| Mention helpers (`extractSearchTextAfterAt`, `getMentionedUserIds`) |   |   | X |   |   |
| Provider-ref category lists & `getLabel` |   |   | X |   |   |
| `checkMailIsPresent` (display only) |   |   | X |   |   |
| `checkMailIsPresent` (transition guard) | X |   |   |   |   |
| `newProviderAdd` validation | X |   |   | X (form schema) |   |
| `newRdaCreate` body | X |   |   |   |   |
| `EditPO` body | X |   |   |   |   |
| `CreateItemRow` body | X |   |   |   |   |
| Attachment auto-tag (B-3) | X |   |   |   |   |
| Comment posting (`mentioned_user_ids` ignored) | X |   |   |   |   |
| Provider-ref PATCH semantics | X |   |   |   |   |
| `idUser` | X |   |   |   | X (legacy) |
| `user_permissions` SQL → token | X |   |   |   | X (legacy) |
| PDF download | X |   |   |   | X (legacy) |
| Attachment download | X |   |   |   | X (legacy) |
| State transitions / approval routing |   | X |   |   |   |
| Per-row totals / `total_price` |   | X |   |   |   |
| `TotalCalculator` |   |   |   |   | X |
| Item modal preview total |   |   | X |   |   |
| `loadingData` HTML stuffing |   |   |   | X (React render) |   |
| `tabs_details` etc. orchestration |   |   |   | X |   |
| `utils.save_item_row`, `MemoryManager`, `ContactsHelper`, `JSObject3/4` |   |   |   |   | X |
| `Modal1`, `mdl_supplierContact`, `NuovoFornitore`, `Home` |   |   |   |   | X |
| Motivazione fields |   |   |   |   | X |

---

## C.7 Open questions

None for Phase C. All allocations are determined by the user's "1:1 + reuse Mistra" stance plus `CLAUDE.md` (no client-side DB, Keycloak roles).

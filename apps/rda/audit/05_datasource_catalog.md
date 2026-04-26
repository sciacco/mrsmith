# Datasource & query catalog

Lists every action defined in `rda.json.gz`, grouped by datasource. JSObject methods are catalogued separately in `06_jsobject_methods.md`.

Path conventions: REST queries use placeholders like `{{this.params.foo}}` (passed via `.run({foo})`), `{{appsmith.URL.queryParams.po_id}}` (read from URL), and `{{appsmith.user.email}}`.

Migration column meaning:
- **BE** — should be a backend endpoint in the new Go backend
- **FE-orchestration** — frontend should keep the call (read-only UX glue)
- **DROP** — query is dead/unused; do not migrate

---

## `Arak (mistra-ng-int)` — Mistra NG Internal API (REST)

### PO list / lifecycle

| Query | Method | Path | Body / params | Page(s) | Migration |
|-------|--------|------|---------------|---------|-----------|
| `GetPOList` | GET | `/arak/rda/v1/po` | `disable_pagination=true`, header `Requester-Email` | RDA | BE: list-my-pos endpoint, scoped by token |
| `NewPo` | POST | `/arak/rda/v1/po` | Inline IIFE body: `type, project, object, reference_warehouse, language, payment_method, cap, vat, recipient_ids:[], provider_id, budget_id, cost_center?, budget_user_id?` | RDA | BE: create-PO endpoint; body construction must move to backend |
| `EditPO` | PATCH | `/arak/rda/v1/po/{po_id}` | Inline IIFE body (full header) | PO Details | BE |
| `PartialPoEdit` | PATCH | `/arak/rda/v1/po/{po_id}` | `this.params.body` (used for `recipient_ids`, `provider_id`) | PO Details | BE |
| `DeletePO` | DELETE | `/arak/rda/v1/po/{this.params.id}` | – | RDA | BE |
| `GetPoDetails` | GET | `/arak/rda/v1/po/{po_id}` | – | PO Details | BE: get-PO-by-id |
| `SavePO` | POST | `/arak/rda/v1/po/{po_id}/submit` | – | PO Details | BE: submit-draft |
| `GeneratePDF` | GET | `/arak/rda/v1/po/{po_id}/download` | – | PO Details | BE: PDF download via signed URL |
| `SentToFornitore` | POST | `/arak/rda/v1/po/{po_id}/send-to-provider` | – | PO Details | BE |

### PO rows (items)

| Query | Method | Path | Body | Page(s) | Migration |
|-------|--------|------|------|---------|-----------|
| `CreateItemRow` | POST | `/arak/rda/v1/po/{GetPoDetails.data.id}/row` | Inline IIFE body building `payment_detail` & `renew_detail` | PO Details | BE: create-row; body construction must move to backend |
| `DeletePORow` | DELETE | `/arak/rda/v1/po/{po_id}/row/{Table2.triggeredRow.id}` | – | PO Details | BE |
| *(missing)* | – | – | An update-row endpoint is referenced via the broken `upd_po_item` JS but no query exists | PO Details | BE: add `PUT /po/{id}/row/{row_id}` |
| `GetItemTypes` | GET | `/arak/rda/v1/article` | `disable_pagination=true`, `type=service|good` | PO Details | BE or FE — catalog read |

### Approvals / workflow transitions

| Query | Method | Path | Page(s) | Migration |
|-------|--------|------|---------|-----------|
| `ApproveFirstSecondLevel` | POST | `/arak/rda/v1/po/{po_id}/approve` | PO Details | BE |
| `RejectFirstSecondLevel` | POST | `/arak/rda/v1/po/{po_id}/reject` | PO Details | BE |
| `ApprovePaymentMethod` | POST | `/arak/rda/v1/po/{po_id}/payment-method/approve` | PO Details | BE |
| `UpdatePaymentMethod` | PATCH | `/arak/rda/v1/po/{this.params.po_id}/payment-method` | PO Details | BE |
| `ApproveLeasing` | POST | `/arak/rda/v1/po/{po_id}/leasing/approve` | PO Details | BE |
| `RejectLeasing` | POST | `/arak/rda/v1/po/{po_id}/leasing/reject` | PO Details | BE |
| `ApproveNoLeasing` | POST | `/arak/rda/v1/po/{po_id}/no-leasing/approve` | PO Details | BE |
| `LeasingIsCreated` | POST | `/arak/rda/v1/po/{this.params.po_id}/leasing/created` | PO Details | BE |
| `ApproveBudgetincrement` | POST | `/arak/rda/v1/po/{po_id}/approve-budget-increment` (body `{increment_promise: queryParam}`) | PO Details | BE |
| `RejectBudgetincrement` | POST | `/arak/rda/v1/po/{po_id}/reject-budget-increment` (same body) | PO Details | BE |
| `ConfirmConformity` | POST | `/arak/rda/v1/po/{po_id}/confirm-conformity` | PO Details | BE |
| `RejectConformity` | POST | `/arak/rda/v1/po/{po_id}/reject-conformity` | PO Details | BE |

### Approver inbox lists

| Query | Method | Path | Page |
|-------|--------|------|------|
| `get_rda_to_approve` | GET | `/arak/rda/v1/po/pending-approval` | App. I - II LIV |
| `get_rda_pendingLeasing` (Leasing page) | GET | `/arak/rda/v1/po/pending-leasing` | App. Leasing |
| `get_rda_pendingLeasing` (no-Leasing page) | GET | `/arak/rda/v1/po/pending-approval-no-leasing` | App. no Leasing |
| `get_rda_to_improveBudget` | GET | `/arak/rda/v1/po-pending-budget-increment` | App.  incremento Budget |
| `get_payment_method` | GET | `/arak/rda/v1/po/pending-approval-payment-method` | App. metodo pagamento |

> Naming inconsistency: budget-increment uses `/po-pending-budget-increment` (no `/po/` segment).

### Attachments

| Query | Method | Path | Body | Page(s) | Migration |
|-------|--------|------|------|---------|-----------|
| `UploadAttachment` | POST | `/arak/rda/v1/po/{po_id}/attachment` | multipart `file=this.params.attachment, attachment_type='quote'\|'transport_document'` | PO Details | BE |
| `DownloadAttachment` | GET | `/arak/rda/v1/po/{po_id}/attachment/{tbl_attachment.triggeredRow.id}/download` | – | PO Details | BE: redirect to signed URL |
| `DeleteAttachments` | DELETE | `/arak/rda/v1/po/{po_id}/attachment/{tbl_attachment.triggeredRow.id}` | – | PO Details | BE |

### Comments

| Query | Method | Path | Body | Page(s) | Migration |
|-------|--------|------|------|---------|-----------|
| `GetComments` | GET | `/arak/rda/v1/po/{po_id}/comment` | – | PO Details | BE |
| `PostComment` | POST | `/arak/rda/v1/po/{po_id}/comment` | `{ comment: Input2.text }` (NB: not properly JSON-encoded; see Findings F-4) | PO Details | BE |

### Providers (provider-qualification module)

| Query | Method | Path | Page(s) | Migration |
|-------|--------|------|---------|-----------|
| `ListaFornitori` (RDA) | GET | `/arak/provider-qualification/v1/provider?disable_pagination=true&page_number=1&usable=true` | RDA | BE/FE: list-providers |
| `ListaFornitori` (PO Details) | GET | same path **without** `usable=true` | PO Details | as above (decide canonical filter) |
| `nuovoFornitore` | POST | `/arak/provider-qualification/v1/provider/draft` | inline body | RDA | BE/FE |
| `GetProviderDetail` (RDA) | GET | `/arak/provider-qualification/v1/provider/{this.params.providerId}` | RDA | as below |
| `GetProviderDetail` (PO Details) | GET | `/arak/provider-qualification/v1/provider/{provider.selectedOptionValue}` | PO Details | as below |
| `GetProviderRef` | GET | `/arak/provider-qualification/v1/provider/{appsmith.store.selectedProvider}/reference/{reference_id}` | RDA, PO Details | **DROP** — broken path (literal `{reference_id}`); appears unused |
| `CreateProviderRef` | POST | `/arak/provider-qualification/v1/provider/{this.params.providerId}/reference` | `this.params.body` | RDA, PO Details | BE/FE |
| `EditProviderRef` (RDA) | PUT | `/arak/provider-qualification/v1/provider/{provider_id}/reference/{reference_id}` | – | RDA | BE/FE — note the placeholders are not Appsmith expressions, so this endpoint is broken on RDA. |
| `EditProviderRef` (PO Details) | PUT | `/arak/provider-qualification/v1/provider/{this.params.providerId}/reference/{this.params.referenceId}` | `this.params.body` | PO Details | BE/FE — works on PO Details |

### Budget

| Query | Method | Path | Page(s) | Migration |
|-------|--------|------|---------|-----------|
| `CallBudget` | GET | `/arak/budget/v1/budget-for-user?page_number=1` (header `user_email`) | RDA, PO Details | BE: scope by token |

### Users

| Query | Method | Path | Page(s) | Migration |
|-------|--------|------|---------|-----------|
| `UserQuery` | GET | `/arak/users-int/v1/user?page_number=1&disable_pagination=true&enabled=true&search_string=…` | PO Details | BE: search-users (used for @-mentions) |

---

## `arak_db (nuovo)` — direct PostgreSQL access

| Query | SQL | Page(s) | Migration |
|-------|-----|---------|-----------|
| `Suppliers` | `SELECT *, name || ' - ' || vat_id as slabel FROM public.suppliers ORDER BY name` | RDA | **DROP** — query exists but is not bound to a visible widget. Replaced by REST `ListaFornitori`. |
| `PaymentMethonds` | `SELECT * FROM provider_qualifications.payment_method WHERE rda_available IS TRUE` | RDA, PO Details | BE: expose as REST endpoint or read in backend service layer |
| `GetDefaultPaymentMethod` | `SELECT payment_method_code FROM provider_qualifications.payment_method_default_cdlan` | RDA, PO Details | BE: expose constant via config or REST |
| `get_item_types` | `SELECT *, short_name || ' - ' || description as slabel FROM public.item_types ORDER BY seq` | PO Details | **DROP** if `GetItemTypes` REST endpoint suffices, else BE |
| `GetArticles` | `SELECT * FROM articles.article LIMIT 10` | PO Details | **DROP** — debug query, unused |
| `userID` | `SELECT id FROM users_int.user WHERE email = {{user_email.text}}` | PO Details | BE: derive from token (security S-1) |
| `user_permissions` | `SELECT r.is_afc, r.is_approver, r.is_approver_no_leasing, r.is_approver_extra_budget FROM users_int.user u JOIN users_int.role r ON u.role = r.name WHERE u.email = '{{user_email.text}}'` | PO Details | **BE / token** — biggest security concern (S-1) |

> All `arak_db (nuovo)` queries should be removed from the client. The new app must not have any direct DB credentials.

---

## `s3cloudlan` — Amazon S3 plugin

| Query | Op | Bucket | Path | Page(s) | Migration |
|-------|----|--------|------|---------|-----------|
| `listArak` | LIST | `arak` | (root) | PO Details | **DROP** — defined but not bound to any visible widget; remove. |

---

## Per-page onLoad summary

| Page | onLoad queries (in execution order) |
|------|-------------------------------------|
| Home | (none) |
| RDA | `CallBudget`, `ListaFornitori`, `PaymentMethonds`, `GetPOList`, `GetDefaultPaymentMethod` |
| PO Details | `userID`, `user_permissions`, `idUser` (JS), `GetPoDetails`, `CallBudget`, `ListaFornitori`, `GetItemTypes`, `GetComments`, `PaymentMethonds`, `GetDefaultPaymentMethod`, `UserQuery`, `GetProviderDetail`, `loadingData` (JS) |
| App. I - II LIV | `get_rda_to_approve` |
| App. Leasing | `get_rda_pendingLeasing` (→ `pending-leasing`) |
| App. metodo pagamento | `get_payment_method` |
| App.  incremento Budget | `get_rda_to_improveBudget` |
| App. no Leasing | `get_rda_pendingLeasing` (→ `pending-approval-no-leasing`) |

---

## Suggested REST surface for the new backend

These names align with the structure under `backend/internal/rda/` and follow `docs/API-CONVENTIONS.md`:

```
GET    /api/rda/v1/me/permissions
GET    /api/rda/v1/me/budgets
GET    /api/rda/v1/payment-methods
GET    /api/rda/v1/payment-methods/default
GET    /api/rda/v1/articles?type=service|good

GET    /api/rda/v1/pos                          (mine)
GET    /api/rda/v1/pos/inbox/level1-2           (was /po/pending-approval)
GET    /api/rda/v1/pos/inbox/leasing
GET    /api/rda/v1/pos/inbox/no-leasing
GET    /api/rda/v1/pos/inbox/payment-method
GET    /api/rda/v1/pos/inbox/budget-increment

POST   /api/rda/v1/pos                           (NewPo)
GET    /api/rda/v1/pos/{id}                      (GetPoDetails)
PATCH  /api/rda/v1/pos/{id}                      (EditPO + PartialPoEdit merged)
DELETE /api/rda/v1/pos/{id}
POST   /api/rda/v1/pos/{id}/submit
GET    /api/rda/v1/pos/{id}/pdf
POST   /api/rda/v1/pos/{id}/send-to-provider

POST   /api/rda/v1/pos/{id}/rows
PUT    /api/rda/v1/pos/{id}/rows/{rowId}        (NEW)
DELETE /api/rda/v1/pos/{id}/rows/{rowId}

POST   /api/rda/v1/pos/{id}/attachments         (multipart)
GET    /api/rda/v1/pos/{id}/attachments/{aid}   (returns signed URL)
DELETE /api/rda/v1/pos/{id}/attachments/{aid}

GET    /api/rda/v1/pos/{id}/comments
POST   /api/rda/v1/pos/{id}/comments            (with mentioned_user_ids)

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

GET    /api/rda/v1/providers                    (proxy to provider-qualification module if needed)
POST   /api/rda/v1/providers/draft
GET    /api/rda/v1/providers/{id}
POST   /api/rda/v1/providers/{id}/refs
PUT    /api/rda/v1/providers/{id}/refs/{refId}

GET    /api/rda/v1/users?search=...             (mention search)
```

This is a sketch for the migration spec phase; align with `docs/IMPLEMENTATION-PLANNING.md` repo-fit checklist before locking it in.

# JSObject / action-collection methods

The Appsmith export contains 18 ActionCollections (JSObjects). Many are scaffolded but empty (`myFun1`, `myFun2`, `init`, `JSObject3`, `JSObject4`). The substantive ones are catalogued below, classified into:
- **Business logic** (B): rules that must move to the backend
- **Frontend orchestration** (O): UI glue that survives the rewrite as a React event handler / store
- **Presentation** (P): formatting / labels / pure transformations
- **Dead** (D): no-ops or unused

---

## `LabelJs` (RDA), `LabelJs` (PO Details), `LabelsJS` (App. I-II LIV), `JSObject1` (App. no Leasing)

Four near-copies of the same module. **P** with embedded **B** (state-machine vocabulary).

```js
stateMap: { DRAFT: "BOZZA", SUBMITTED: "CONFERMATO", ..., DELIVERED_AND_COMPLIANT: "EROGATO E CONFORME" }
stateLabel(state) → stateMap[state] ?? state ?? "-"
translate(key) → stateMap[key] ?? key ?? "-"
```

Migration: collapse to a single typed label module in `apps/rda/src/state-labels.ts` (Italian only). The state names themselves become a TypeScript union mirroring the backend enum.

---

## `Utils` (RDA) — **B + O**

- `newProvider()`: opens the inline new-provider container; calls `Contact.initializeContacts(false)`. **O** — replace with React state toggling.
- `extractApproverList(approverList)`: joins `[{email, level}]` into `"a@cdlan (1), b@cdlan (2)"`. **P** — pure transform.
- `newProviderAdd()`: 13-step `if/else` form-validation chain that emits `showAlert` on each missing field, then calls `nuovoFornitore.run()` + `ListaFornitori.run()`. **B** — rules to move to a frontend Zod schema and backend validation.
- `newRdaCreate()`: validates that `txt_object`, `inp_project`, payment-method (if no supplier default) are present, then `NewPo.run()` → `closeModal('ModalNewPO')` → `GetPOList.run()` → `navigateTo('PO Details', { po_id: NewPo.data.id })`. **B + O** — rules belong on backend; navigation in React.

## `Utils` (App.  incremento Budget) — **D + P**

- `myFun1`, `myFun2` → empty stubs (D)
- `extractApproverList(approverList)` → identical duplicate of RDA's (P, but D in this page since it isn't used)

---

## `Contact` (RDA) — **O**

State variables `myVar1: []`, `myVar2: {}` (D).

- `allCategory` / `availableCategory`: option lists for `reference_type` (`OTHER_REF`, `ADMINISTRATIVE_REF`, `TECHNICAL_REF`, `QUALIFICATION_REF`). `availableCategory` excludes `QUALIFICATION_REF` from new-row options. **P + B** — embedded business rule that qualification is read-only.
- `initializeContacts()`: hides `Container10` and resets `selectedContacts` store key. **O**
- `visbilityContactBox(visibility)`: `Container10.isVisible(visibility)`. **O**
- `addContact(providerId, firstName, lastName, email, phone, category)`: builds dataBody and calls `CreateProviderRef.run()` then `GetProviderDetail.run()`. **O**
- `storeSelectedContact(contacts)`: stores selected ids into `selectedContacts`. **O**
- `updateContact(providerId)`: calls `EditProviderRef.run()` (no params — note: this is the broken `EditProviderRef` on RDA, see Findings F-9 / DS catalog). **D / broken**
- `getContacts(providerId)` / `getProviderData(providerId)`: thin wrappers around `GetProviderRef.run()` / `GetProviderDetail.run()`. **O / partly broken** (`GetProviderRef` is broken).

## `Contact` (PO Details) — **B + O**

Larger and more interesting copy:

- `allCategory` / `availableCategory`: same options (P + B as above).
- `getLabel(value)`: `allCategory.find(c => c.value === value)?.label || value`. **P** — single source of truth for category labels.
- `initializeContacts()`, `visbilityContactBox()`: same as RDA copy. **O**
- `emptyContacts(poid, providerId)`: clears recipients on the PO via `PartialPoEdit.run({recipient_ids: [], provider_id: providerId})` then refreshes Text27. Used when the supplier is changed. **B (clearing recipients on supplier change) + O.**
- `updateContactList(poid)`: collects `Table4.selectedRows.map(r => r.id)` and PATCHes `recipient_ids`. **O**
- `checkMailIsPresent(approvers)`: `approvers.some(item => item.user.email === appsmith.user.email)`. **B** — gating rule for approval/reject buttons.
- `addContact(providerId, firstName, lastName, email, phone, category)`: same as RDA but with success/error toasts. **O**
- `updateContact(providerId, referenceId, firstName, lastName, email, phone)`: builds body conditionally — **note** the special handling: empty/null `phone` is sent as `""` (clearing), present `phone` is sent as-is; empty `email` is **not** sent (so the existing email is preserved). **B** — these field-level update semantics belong on the backend (PATCH semantics).
- `loadingData()`: refetches `GetProviderDetail` + `GetPoDetails`, then renders **HTML** into `Text27` listing recipients (or QUALIFICATION_REF fallback). **O + F-10** (XSS risk; replace with React render).
- `storeSelectedSupplier(id)`, `storeSelectedContact(contacts)`: store selectors. **O**
- `getContacts(providerId)`, `getProviderData(providerId)`: wrappers (O).
- `myVar1`, `myVar2`, `myFun1`, `myFun2`: stubs (D).

---

## `utils` (PO Details, lowercase u) — mix

- `myVar1: []`, `myVar2: {}` (D).
- `save_item_row()`: `if _.isFinite(curr_item.id) → upd_po_item.run().then(get_list_items.run())` else `ins_po_item.run().then(get_list_items.run()).then(storeValue('curr_item', ins_po_item.data[0]))`. **D** — `upd_po_item`, `ins_po_item`, `get_list_items` actions **do not exist**. This function is dead (and explains `F-5` row-edit bug).
- `savePoDraft()`: `EditPO.run()` then toast. **O** — referenced from `btn_save_draftCopy` indirectly? No, `btn_save_draftCopy` calls `EditPO.run()` directly. So this method is also **D**.
- `delete_item_row()`: empty (D).
- `tab_details_action()`: `if (tabs_details.selectedTab === 'Items') get_list_items.run();` — **D**, since the tab id `'Items'` doesn't exist on the page (the actual id is `tab2` labeled "Righe PO") and `get_list_items` does not exist.
- `dateConverter(inputDate)`: parses `YYYY-MM-DD HH:mm:ss` (treating space as `T`) and reformats to `it-IT` locale `dd/MM/yyyy HH:mm`. **P** — keep as a small helper (or use the standard date library in the new app).

---

## `JSObject1` (PO Details) — **O**, comment @-mentions

- State: `mentionQuery`, `showMentions`, `search_string`, `mentionedUsers: [{id, email}]`.
- `handleInputChange(text)`: detects `@`, runs `UserQuery.run({search_string: ...})`. **O**
- `insertMention(user)`: replaces the trailing `@token` with `@user.email ` and pushes user into `mentionedUsers`. **O**
- `extractSearchTextAfterAt(text)`: helper. **P**
- `getMentionedUserIds()`: returns deduped ids of mentions present in the current text. **B (computed but unused)** — see Findings S-3.
- `resetMentions()`: clears state. **O**

---

## `TotalCalculator` (PO Details) — **B**

```js
getTotal(type, unit_price, qta, activation_price, initial_subscription_month, recursive_month, recursive)
```

- `good && recMonth>0 && recursive===true` → `(price * (initMonths/recMonth)) * qty + activation*qty`
- `service && (recMonth==null||0)` → `(price + activation) * qty`
- `good` → `price * qty`
- otherwise 0

This calculator is referenced by `Table2`'s "Totale riga" computed column, but the **modal preview** (`Text17`) uses **a different formula**:
- `service`: `(mrc * qty * duration) + (nrc * qty)`
- `good`: `goodsPrice * qty`

So the modal preview and the table cell can disagree. The backend's `total_price` is the source of truth; both client formulas should be replaced with backend-provided per-row totals.

---

## `PDFGenerator` (PO Details) — **O**

- `downloadPOPDF()`: calls `GeneratePDF.run()` and decodes the response as base64 OR raw bytes, then triggers `download(url, …)`. The branching exists because the response shape is inconsistent. **Move to backend** — return a redirect to a signed URL.

## `attachmentsJs` (PO Details) — **O + B**

- `uploadPreventivi()`: loops over `upload_btn_prv.files`, calls `UploadAttachment.run({attachment, attachment_name, attachment_type, po_id})`. **B** — `attachment_type` derived in the action body using `GetPoDetails.data.state == 'DRAFT' ? 'quote' : 'transport_document'`.
- `downloadAttachment(row)`: signs/decodes/downloads (same shape mishmash as PDFGenerator). **O — move to BE.**

---

## `ContactsHelper` (PO Details) — **D / partly D**

- `setMemory_addContactToList(id)` / `setMemory_removeContactToList(id)`: store helpers for a `contactList` collection that is not actually used anywhere in the DSL.
- `setMemory_supplier(supplier_id, supplier_name, raw)`: store the chosen supplier id+name; called from… nowhere visible. The `mdl_supplierContact` modal that would use it is dead.
- `getSupplierContacts`, `addSupplierContact`, `updateSupplierContact`: empty stubs (D).

This entire module appears to be dead code. Do not port.

## `userFunctions` (PO Details) — **O**

- `idUser()`: `await userID.run(); return userID.data[0].id`. Used by `user_id` hidden Input widget. **Replace with token-based user id resolution.**

## `JSObject3` (PO Details), `JSObject4` (PO Details) — **D**

Empty stubs.

## `MemoryManager` (PO Details) — **D**

Generic `saveInMemory(key, value)` / `readFromMemory(key)` helpers. Not referenced elsewhere; remove.

## `Contact` (PO Details, second appearance) — duplicate of the same name

(Already covered above — there is one `Contact` collection on PO Details, not two.)

---

## Summary table

| JSObject | Page | LOC | Verdict |
|----------|------|----:|---------|
| `LabelJs` | RDA | 38 | Collapse all 4 copies into one shared module |
| `LabelJs` | PO Details | 38 | (same) |
| `LabelsJS` | App. I - II LIV | 28 | (same; missing 4 keys) |
| `JSObject1` | App. no Leasing | 28 | (same; missing 4 keys) |
| `Utils` | RDA | 100 | Validation rules → schema; nav → React; supplier-create flow keep |
| `Utils` | App. incremento Budget | 24 | DROP except `extractApproverList` (already shared) |
| `Contact` | RDA | 70 | Subset of PO Details Contact; consolidate |
| `Contact` | PO Details | 160 | Keep; refactor: split B (rules) and O (handlers) |
| `utils` (lowercase) | PO Details | 33 | DROP (dead) |
| `JSObject1` (mentions) | PO Details | 64 | Keep as React state in comments component |
| `TotalCalculator` | PO Details | 18 | DROP — backend supplies row total |
| `PDFGenerator` | PO Details | 35 | DROP — backend serves signed URL |
| `attachmentsJs` | PO Details | 92 | Replace; only keep the auto-tagging rule (B-3) on backend |
| `ContactsHelper` | PO Details | 32 | DROP (dead) |
| `userFunctions` | PO Details | 5 | DROP — use token id |
| `JSObject3` / `JSObject4` / `MemoryManager` | PO Details | 7-15 | DROP |

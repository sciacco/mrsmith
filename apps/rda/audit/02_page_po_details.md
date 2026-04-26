# Page audit — `PO Details`

**Purpose:** read/edit a single PO end-to-end. Contains the header, the items list (PO rows), the attachments tab, the supplier-contacts tab, the comments thread with @-mentions, and the entire **state-driven action bar** that drives the workflow (approve / reject / send to supplier / generate PDF / mark as conformant). This is by far the largest and most complex page in the application.

The page is keyed by `appsmith.URL.queryParams.po_id`; everything is derived from `GetPoDetails.data` (single PO record from `GET /arak/rda/v1/po/{po_id}`).

## onLoad actions

These are flagged `executeOnLoad: true`:

| Query | Datasource | Purpose |
|-------|-----------|---------|
| `GetPoDetails` | Arak REST `GET /arak/rda/v1/po/{po_id}` | Source of truth for the page. |
| `CallBudget` | Arak REST `GET /arak/budget/v1/budget-for-user` | Allowed budgets for the requester. |
| `ListaFornitori` | Arak REST `GET /arak/provider-qualification/v1/provider` (no `usable=true` filter here) | All suppliers — used to swap supplier on a DRAFT. |
| `GetItemTypes` | Arak REST `GET /arak/rda/v1/article` (filtered by `sl_item_type` selection — initially empty) | Article catalog for the item modal. |
| `GetComments` | Arak REST `GET /arak/rda/v1/po/{po_id}/comment` | Comment thread. |
| `PaymentMethonds` | PostgreSQL `provider_qualifications.payment_method` | Allowed payment methods. |
| `GetDefaultPaymentMethod` | PostgreSQL `provider_qualifications.payment_method_default_cdlan` | Default CDLAN payment-method code. |
| `UserQuery` | Arak REST `GET /arak/users-int/v1/user` | All users — used to power @-mentions in comments. |
| `userID` (named `idUser` in JS) | PostgreSQL `users_int.user` | Internal numeric id of the logged-in user (only used for comments?). |
| `GetProviderDetail` | Arak REST `GET /arak/provider-qualification/v1/provider/{providerId}` | Selected supplier's references / contacts. |
| `loadingData` | JS | Renders the recipients HTML into `Text27`. |
| `user_permissions` | PostgreSQL `users_int.user JOIN users_int.role` | `is_afc`, `is_approver`, `is_approver_no_leasing`, `is_approver_extra_budget`. **Drives every approval button.** |

## Logical sections / regions

The DSL has 8 top-level children inside the canvas. The reading order on screen is:

1. **Container2** — header strip with PO number, state, approver lines, the action button-groups (`ButtonGroup1` for approvals, `ButtonGroup1Copy` for rejections, plus solo buttons `btn_save_draftCopy`, `btn_save_draft`, `btn_pending_verification`, `btn_save_draftCopyCopy`, `btn_reset`, banner `Text31`).
2. **Container7** — the comments thread (`Input2` + `Button2` + `List1`/`List2`/`List3`).
3. **Container24, Container17, Container19** — orphan/unused containers (`isVisible:false` at design time): contain duplicated header inputs (`Input1` Numero PO, `f_date`, `rif_datequoteSupplierBack`, `rif_quoteSupplierBack`, `email_fornitore`, hidden `inp_reference_warehouse`).
4. **Container23 / cnt_budgetAndInfo / Container26 / cnt_fornitore / Container30** — the editable PO header: budget, subject, project, supplier, payment method, supplier-quote ref, supplier offer date.
5. **tabs_details** — tabbed details body with **Allegati**, **Righe PO**, **Note**, **Contatti Fornitore** (legacy "Contatt" tab is hidden).
6. **mdl_edit_item** — the item editor modal.
7. **Modal1** — legacy demo modal (unused).
8. **mdl_supplierContact** — supplier address-book modal (referenced but mostly unused; `Table3` has no `tableData` binding).
9. **modal_confirmSendToApprovers** — confirmation modal before submitting a draft.

### Header strip (`Container2`)

`Text22Copy` shows: `Ordine Numero: {{code}} del {{utils.dateConverter(created)}} — Stato Attuale : {{LabelJs.stateLabel(state)}}`.

`Text23` shows the level-1 and level-2 approver emails, derived inline from `GetPoDetails.data.approvers` filtered by `level === '1'` and `level === '2'`.

#### Action: `btn_save_draftCopy` "Aggiorna Bozza PO"
- Visible only if `state == 'DRAFT'`. Disabled when `project.isDisabled` (i.e. the project field locked).
- onClick: `EditPO.run()` then `GetPoDetails.run()` and `Contact.loadingData()`.

#### Action: `btn_save_draft` "Manda PO in Approvazione"
- Visible only if `state == 'DRAFT'`. Disabled when **any** of:
  - state ≠ DRAFT (defensive)
  - no rows present (`!GetPoDetails.data.rows`)
  - **(business rule)** `total_price ≥ 3000 €` AND fewer than 2 attachments → user must upload at least 2 quote PDFs
- onClick: `showModal(modal_confirmSendToApprovers)` (the modal then calls `EditPO.run().then(SavePO.run)` and routes back to `RDA`).
- Companion banner `Text31` is shown when the rule above blocks the button: "Attenzione: importo superiore a 3.000€. Aggiungi 2 preventivi".

#### Action: `btn_pending_verification` "In contestazione"
- Visible only when `state == 'PENDING_VERIFICATION'`. Calls `RejectConformity` (POST `/po/{id}/reject-conformity`).

#### Action: `btn_save_draftCopyCopy` "Erogato e conforme"
- Visible only when `state == 'PENDING_VERIFICATION'`. Calls `ConfirmConformity` (POST `/po/{id}/confirm-conformity`). Toast on success: "PO confermato con successo"; on error: "verifica inserimento DDT" → confirms that **conformity requires a DDT (transport document) to be uploaded**.

#### Action: `btn_reset` "Chiudi"
- Always visible. Navigates back to RDA list.

#### Action button-group `ButtonGroup1` (Approvals)
Each sub-button is enabled by a *combination* of state + permission flag. Mapping:

| Button | enabled when | API call | Then |
|--------|--------------|----------|------|
| Approva (Liv 1) | `state==PENDING_APPROVAL` && `current_approval_level==1` && `is_approver==true` && `Contact.checkMailIsPresent(approvers)` | `ApproveFirstSecondLevel` POST `/po/{id}/approve` | Toast + navigate to `App. I - II LIV` |
| Approva (Liv 2) | same but `current_approval_level==2` | same | same |
| Approva pagamento | `state==PENDING_APPROVAL_PAYMENT_METHOD` && `is_afc==true` | `ApprovePaymentMethod` POST `/po/{id}/payment-method/approve` | Navigate to `App. metodo pagamento` |
| Approva leasing | `state==PENDING_LEASING` && `is_afc==true` | `ApproveLeasing` POST `/po/{id}/leasing/approve` | Navigate to `App. Leasing` |
| Approva no leasing | `state==PENDING_APPROVAL_NO_LEASING` && `is_approver_no_leasing==true` | `ApproveNoLeasing` POST `/po/{id}/no-leasing/approve` | Navigate to `App. no Leasing` |
| Approva incremento budget | `state==PENDING_BUDGET_INCREMENT` && `is_approver_extra_budget==true` | `ApproveBudgetincrement` POST `/po/{id}/approve-budget-increment` (body `{increment_promise: <queryParam>}`) | Navigate to `App. incremento Budget` |
| Genera PDF | `state != DRAFT` | (JS) `PDFGenerator.downloadPOPDF()` → `GeneratePDF` GET `/po/{id}/download` | Browser file download |
| Invia ordine al fornitore | `state == PENDING_SEND` | `SentToFornitore` POST `/po/{id}/send-to-provider` | Toast + navigate back to RDA |
| Leasing Creato | `state == PENDING_LEASING_ORDER_CREATION` && `is_afc==true` | `LeasingIsCreated` POST `/po/{id}/leasing/created` | Reload PO + reload contacts |

> **`Contact.checkMailIsPresent(approvers)`** is the only client-side guard; it returns `true` iff the current user's email matches one of the entries in `GetPoDetails.data.approvers[i].user.email`. This means *the actual approver-vs-current-user check is not in `user_permissions` flags but in the per-PO approvers list*. *Business rule worth preserving in the rewrite.*

#### Action button-group `ButtonGroup1Copy` (Rejections)
Mirror of the approvals, with the same enable conditions and orange/red styling. The reject buttons all call POST `/po/{id}/reject` (`RejectFirstSecondLevel`) for the level-1/level-2/payment-method/no-leasing path, plus `RejectLeasing` (`/po/{id}/leasing/reject`) and `RejectBudgetincrement` (`/po/{id}/reject-budget-increment`).

> Note: "Rifiuta no leasing" actually calls `RejectFirstSecondLevel.run()` (i.e. POST `/po/{id}/reject`), not a dedicated `/no-leasing/reject` endpoint. *Possible bug in the source; check with backend whether `/reject` is the canonical "stop the workflow" endpoint regardless of branch.*

> Note 2: `groupButtono66fk8kt2a` (Rifiuta incremento budget) declares `disabledWhenInvalid: state=='PENDING_BUDGET_INCREMENT' || appsmith.user.groups.includes('Acquisti RDA approvers I-II') == true` — uses a Keycloak group name `Acquisti RDA approvers I-II`. This appears to be a **dead leftover**: the actual `isDisabled` binding is purely state+permission and never references this group. Migration must use the new `app_rda_*` Keycloak roles, not this string.

### Editable PO header (`Container23` / `cnt_budgetAndInfo` / `Container26` / `cnt_fornitore` / `Container30`)

All header inputs become **read-only when state ≠ DRAFT** (binding pattern `isDisabled = {{ GetPoDetails.data.state !== 'DRAFT' }}`). Exceptions: `s_payment_method` is editable also during `PENDING_APPROVAL_PAYMENT_METHOD`, `BRT_upd_pagamento` (Aggiorna metodo di pagamento) is enabled only during `PENDING_APPROVAL_PAYMENT_METHOD` AND only for the requester.

| Widget | Bind | Notes |
|--------|------|-------|
| `s_budget` | `CallBudget.data.items` → label/value `${budget_id}::${cost_center}::${user_id}` | Default value built from `GetPoDetails.data.budget`. **Sneaky:** the source uses `b.id` for the default but `b.budget_id` for new options — likely a bug. Document. |
| `f_subject` | `GetPoDetails.data.object` | Subject text. |
| `project` | `GetPoDetails.data.project` | |
| `provider` | `ListaFornitori.data.items` (sourceData unmapped → uses `id` as value, `company_name` as label by Appsmith default) | onChange clears recipients via `Contact.emptyContacts(po_id, provider_id)` (calls `PartialPoEdit` PATCH with `{recipient_ids:[], provider_id}`). |
| `s_payment_method` | mix of supplier default + CDLAN default + active payment methods | `defaultOptionValue` derived from `GetPoDetails.data.payment_method.code`. |
| `BRT_upd_pagamento` | button | onClick: `UpdatePaymentMethod` PATCH `/po/{id}/payment-method` with `{payment_method: s_payment_method.selectedOptionValue}`. |
| `rif_quoteSupplier`, `rif_datequoteSupplier` | `provider_offer_code` / `provider_offer_date` | Optional supplier-quote reference; included in `EditPO` body. |

`Text27` is filled procedurally by `Contact.loadingData()` with HTML strings listing the PO recipients (or, if none, the qualification reference). This is a **design smell**: the rewrite should render contacts as React components, not as `setText('<span style=…>…</span>')`.

`Text29` notes: "Per aggiungere ulteriori contatti selezionarli dalla tab 'Contatti Fornitore' qui sotto" — explains the relationship to the `Table4` further down in the tabs region.

### Tabs (`tabs_details`)

| Tab id | Label | What it shows |
|--------|-------|---------------|
| `tabtbby2egceu` | Allegati | The attachments table + the upload widget. |
| `tab2` | Righe PO | The PO items table (`Table2`) plus the alternate list `lst_itemsCopy` (hidden) plus `IconButton2` (+) to open `mdl_edit_item`. |
| `tab1` | Note | Two textareas: `txt_note` (sent to provider) and `rt_description` (internal notes). Both editable only when DRAFT. |
| `tabd3n2bz6elj` | "Contatt" | **Hidden tab.** Dead. |
| `tabw44lwg74jl` | Contatti Fornitore | `Table4` editable list of provider references; allows add/edit/remove and selection of recipients. |

`onTabSelected`: `utils.tab_details_action()` (re-fetches items if Items tab is selected — defensive remnant since `get_list_items` does not exist on this page; harmless), then `Contact.storeSelectedSupplier(provider.selectedOptionValue)` and `GetProviderDetail.run()` to refresh contacts.

#### Tab "Allegati"

- `Text18` — banner reminding users that `>3000€` POs need 3 quotes total (i.e. 2 attachments in addition to the order itself).
- `upload_btn_prv` (FilePicker) — disabled unless state is `DRAFT` or `PENDING_VERIFICATION`. onFilesSelected: `attachmentsJs.uploadPreventivi()` then `GetPoDetails.run()`. The JS uploads each file via `UploadAttachment` (multipart `/po/{po_id}/attachment`) and *automatically tags*:
  - `attachment_type = "quote"` if state == DRAFT
  - `attachment_type = "transport_document"` otherwise
  This is **an embedded business rule**.
- `IconButton3Copy` — refresh.
- `tbl_attachment` — table bound to `GetPoDetails.data.attachments`, shows file name, type, created_at (formatted `DD/MM/YYYY` via `moment`). Per-row actions:
  - **Elimina** (`DeleteAttachments` DELETE `/po/{po_id}/attachment/{attachment_id}`). Disabled unless DRAFT.
  - **Scarica** (calls `attachmentsJs.downloadAttachment(currentRow)` which calls `DownloadAttachment` GET `/po/{po_id}/attachment/{id}/download`, decodes base64 or raw bytes, builds a Blob, downloads as PDF). The JS deals with three response shapes (`{response}`, `[{response}]`, raw URL), suggesting backend inconsistencies.
- `select_motivation` and `input_motivazion` are hidden options for "motivazione di esclusione delle 3-quote rule" (`Accordo quadro`, `Vendor specificato`, `Altro`). They are not actually wired to any save action — *probably planned but never finished*. Treat as **TODO**: the bypass-quote rule may exist in business but the UI is incomplete.

#### Tab "Righe PO" — `Table2`

Bound to `{{GetPoDetails.data.rows}}`. Visible columns:

- `description` (HTML) — line description.
- `activation_fee` (currency EUR, label "Costo unitario / NRC").
- `montly_fee` (label "MRC").
- `qty` (number, label "Q.tà").
- `type` (label "Tipo").
- `total` (currency EUR, label "Totale riga", computed via `TotalCalculator.getTotal(...)` per row).
- Plus `customColumn1` ("Modifica") that fires `storeValue('curr_item', currentRow)` and opens `mdl_edit_item`.
- And `customColumn2` ("Elimina") that calls `DeletePORow` (DELETE `/po/{po_id}/row/{row_id}`). Disabled unless state is DRAFT.

Above the table:
- `IconButton2` (+) — add new row, enabled only DRAFT. onClick: `storeValue('curr_item', '{}')` then `showModal('mdl_edit_item')`.
- `PO_details_TotalAmount` text: `Totale PO: € {{GetPoDetails.data.total_price.slice(0, -1)}}`. **The `slice(0,-1)` is a workaround for a backend formatting bug; document.**
- `Text19` warning about the 3-quote rule.
- `IconButton3` refresh.

Hidden duplicate `lst_itemsCopy` is a List-V2 also bound to `rows`. Probably an earlier prototype; ignore.

#### Tab "Note"

Two simple text inputs `txt_note` and `rt_description`, both bound to `GetPoDetails.data.note` / `description`, both disabled when state ≠ DRAFT.

#### Tab "Contatti Fornitore" — `Table4`

Bound to `{{GetProviderDetail.data.refs}}`. Columns:
- `email`, `first_name`, `last_name`, `phone` (with regex validation `^\+[1-9][0-9]{4,19}$`), `reference_type` (select with options from `Contact.allCategory` / `Contact.availableCategory`).
- `EditActions1` — Save/Discard inline-edit actions; only **`isSaveVisible` if `reference_type != 'QUALIFICATION_REF' && state == 'DRAFT'`**, i.e. the qualification ref is read-only. `onSave` calls `Contact.updateContact(provider_id, ref_id, …)` which calls `EditProviderRef` PUT.
- `allowAddNewRow` only when state == DRAFT. `onAddNewRowSave` calls `Contact.addContact(...)` which calls `CreateProviderRef` POST + `GetProviderDetail.run()`.
- `defaultSelectedRowIndices` computed from `GetPoDetails.data.recipients` — the rows currently selected as recipients are pre-selected.
- Cells are read-only for the qualification reference (regex is in `validation` for email).

`Button10` "Salva contatti selezionati" calls `Contact.updateContactList(po_id)` which sends `PartialPoEdit` PATCH with `recipient_ids = selectedRows.map(r => r.id)`. Disabled unless DRAFT.

`Text24` instruction: "Seleziona nella tabella i contatti a cui inviare l'ordine. Se non viene spuntato alcun contatto, verrà utilizzato il contatto di tipo qualifica." — *embedded fallback rule*: empty recipients → qualification ref.

### `mdl_edit_item` modal (item form)

- `sl_item_type` SELECT ("Merci" `good` / "Servizi" `service`). onChange triggers `GetItemTypes.run()` (catalog filtered server-side).
- `sl_product` SELECT — populated from `GetItemTypes.data.items`.
- `rt_item_description` (required) — line description.
- `f_item_qty` (required, defaults to `appsmith.store.curr_item.f_item_qty || 1`).
- `f_item_unit_price` — enabled only for `good`.
- `f_activation_priceNRC` (NRC), `f_price_mrc` (MRC) — enabled only for `service`. Required if both NRC and MRC are 0 (XOR-required).
- `f_months_first_period` "Durata (mesi)" — required for service, default `1`.
- `sl_recurring_months` (Mensile/Trimestrale/Semestrale/Annuale) — required for service.
- `sl_start_at` SELECT with **two different option sets** depending on item type:
  - service → `activation_date` ("Da attivazione"), `specific_date` ("Data fissa")
  - good → `activation_date` ("Alla consegna"), `advance_payment` ("Anticipato"), `specific_date` ("Data fissa")
- `f_start_at_date1` DATE — visible always but disabled unless `sl_start_at == specific_date`.
- `sw_auto_renew` SWITCH — services only; when on, `f_cancel_before_days` (preavviso disdetta) is enabled and required.
- `Text17` — live total preview: `service: (mrc*qty*duration) + (nrc*qty)`, `good: goodsPrice*qty`. Note this differs from `TotalCalculator.getTotal` (which mixes `recursive_month` / `initial_subscription_month`); the two are not consistent. Backend total in `GetPoDetails.data.total_price` is the source of truth.
- `btnSaveItem` "Salva" — disabled when `service && nrc==0 && mrc==0`. onClick: `CreateItemRow.run()` then close modal and refresh PO.

`CreateItemRow` (POST `/po/{po_id}/row`) builds a long body inline that wires together `payment_detail`, `renew_detail`, `start_at_date` (formatted `YYYY-MM-DD`), `month_recursion`, `automatic_renew`, `cancellation_advice`, `qty`, `price` (with `,` → `.` normalisation), `activation_price`, `requester_email`. **All this logic must move to the backend** in the rewrite — it's domain logic, not orchestration.

> Notable bug: when the row is being **edited** (not just created), the modal still calls `CreateItemRow` (POST), not an `UpdateRow` endpoint. There is a JS stub `save_item_row` (in the legacy `utils` JSObject) referencing `upd_po_item.run()` that **does not exist** as an action — so editing a row likely creates a duplicate. Confirm with users; treat as a known issue carried in the spec.

### Comments thread (`Container7`)

- `Input2` — comment textarea. `onTextChanged` → `JSObject1.handleInputChange(text)`. The handler triggers `UserQuery.run({search_string})` whenever the user types `@…`. The list `List2`/`List3` (showing `UserQuery.data.items`) is `isVisible: JSObject1.showMentions`. Clicking a list item calls `JSObject1.insertMention(user)` which replaces the trailing `@token` with `@user.email ` and remembers `{id, email}` in `JSObject1.mentionedUsers`.
- `Button2` "Salva Commento" — onClick: `PostComment.run()` (POST `/po/{po_id}/comment` with `{ comment: Input2.text }`) then `GetComments.run()` and `Input2.setValue('')` and `JSObject1.resetMentions()`.
- `List1` — bound to a normalized projection of `GetComments.data` (handles `Array | {items}` shape and missing `created_at`). Each item shows the user, the comment, and a flat list of replies.

> **Bug:** `PostComment` body sends only `{ comment: Input2.text }`. The parsed `mentionedUsers` ids are computed (`JSObject1.getMentionedUserIds()`) but never sent. So @-mentions today **are not actually delivered to the backend**; the feature is purely visual.

### `mdl_supplierContact` modal

A "rubrica" modal that shows `Table3` (no tableData binding) and a `btn_SaveSupplierContacts` button (no onClick). **This modal is dead** — leftover from a previous design. Do not port.

### `modal_confirmSendToApprovers` modal

Confirmation dialog before submitting the draft. `Button12` "Conferma" runs `EditPO` then `SavePO` then refreshes PO and navigates back to RDA list. **Note the chain races:** `SavePO.run()` is invoked inside the `.then` of `EditPO.run()`, but `showAlert` and `navigateTo` execute concurrently with `SavePO.run()` (no `await`). In React this becomes a single submit-and-await flow.

### Hidden state machine summary (extracted)

| State (English) | Italian label | Trigger to enter | Actions available |
|------|------|------|------|
| `DRAFT` | BOZZA | `NewPo` POST | edit header, add/remove rows, upload quotes, edit recipients, "Aggiorna Bozza", "Manda in Approvazione" |
| `SUBMITTED` | CONFERMATO | server-side after `SavePO` | (read-only header) |
| `CANCELED` | CANCELLATO | (server only) | (read-only) |
| `PENDING_CHECK_DOCUMENT` | IN ATTESA VERIFICA PREVENTIVI | server | (read-only) |
| `PENDING_APPROVAL_PROVIDER` | VERIFICA QUALIFICA | server | (read-only) |
| `PENDING_APPROVAL_PAYMENT_METHOD` | IN ATTESA VERIFICA METODO PAGAMENTO | server (when payment≠CDLAN-default) | requester can edit `s_payment_method` and click `BRT_upd_pagamento`; AFC can Approve/Reject |
| `PENDING_APPROVAL` | IN APPROVAZIONE | `SavePO` POST | Lvl-1/Lvl-2 approver can Approve/Reject (`current_approval_level` 1 then 2) |
| `REJECTED` | RIFIUTATO | reject calls | terminal |
| `PENDING_LEASING` | IN ATTESA VERIFICA LEASING | server | AFC approves/rejects leasing |
| `PENDING_APPROVAL_NO_LEASING` | IN ATTESA APPROVAZIONE NO-LEASING | reject leasing | no-leasing approver acts |
| `PENDING_CONTRACT_VERIFICATION` | IN ATTESA VERIFICA CONTRATTO | server | (read-only) |
| `PENDING_BUDGET_INCREMENT_CHECK` | IN ATTESA INCREMENTO BUDGET | server | (read-only) |
| `PENDING_BUDGET_INCREMENT` | AUMENTO BUDGET | server | extra-budget approver acts (uses URL param `budget_increment_needed` as `increment_promise`) |
| `PENDING_BUDGET_SUBTRACTION` | SCALO BUDGET | server | (read-only) |
| `PENDING_PROVIDER_SAVED_IN_ALYANTE` | CHECK CENSIMENTO FORNITORE | server | (read-only) |
| `PENDING_PDF_GENERATION` | IN ATTESA GENERAZIONE PDF | server | (read-only) |
| `PENDING_ERP_SAVE` | SALVATAGGIO ERP | server | (read-only) |
| `PENDING_LEASING_ORDER_CREATION` | IN ATTESA CREAZIONE ORDINE LEASING | server | AFC clicks "Leasing Creato" |
| `PENDING_SEND` | IN ATTESA INVIO FORNITORE | server | "Invia ordine al fornitore" enabled |
| `CLOSED` | CHIUSO | server | terminal |
| `PENDING_VERIFICATION` | IN ATTTESA VERIFICA CONFORMITA' (sic, three Ts) | server | "Erogato e conforme" / "In contestazione" + DDT upload (PENDING_VERIFICATION is also when uploads are tagged as `transport_document`) |
| `PENDING_DISPUTE` | IN CONTESTAZIONE | reject conformity | terminal-ish |
| `DELIVERED_AND_COMPLIANT` | EROGATO E CONFORME | confirm conformity | terminal |

> The "IN ATTTESA" typo is preserved verbatim in `LabelJs.stateMap`. Decide whether to keep it or fix it in the new portal labels.

## Hidden / non-obvious logic worth preserving

1. **Attachment type auto-tagging** based on PO state (DRAFT → quote, anything else → transport_document).
2. **3-quote rule** for total ≥ 3000 € (need ≥2 attachments before submitting).
3. **Empty `recipient_ids` ⇒ use qualification ref** (server-side rule — UI even tells the user that).
4. **`current_approval_level` discriminates Liv-1 vs Liv-2** approval buttons (single endpoint POST `/approve` is used for both).
5. **`Contact.checkMailIsPresent`** ensures the *current user* is in the per-PO approver list before showing the approve buttons.
6. **`is_afc` permission** drives leasing approval, payment-method approval, leasing-creation buttons.
7. **`is_approver_extra_budget`** drives the budget-increment approval path.
8. **`is_approver_no_leasing`** drives the no-leasing approval path.
9. **Payment-method change while `PENDING_APPROVAL_PAYMENT_METHOD`** uses a dedicated PATCH `/po/{id}/payment-method` (not the generic edit) — the only state in which `s_payment_method` remains editable.

## Open questions

- Editing an item: is `CreateItemRow` reused on edit, or is the edit feature broken? (The Save button always calls POST. There is no `UpdateItemRow` action.) Backend log should show duplicate POSTs for edit attempts.
- `Modal1` and `mdl_supplierContact` look unused; confirm removal is safe.
- The `select_motivation` / `input_motivazion` inputs (motivazione di esclusione 3-preventivi) are wired to nothing. Is the workflow for "Accordo quadro" / "Vendor specificato" supposed to bypass the 3-quote rule? Confirm with PO/business.
- `LeasingIsCreated` body is empty (POST only). `po_id` is passed as a `params` value but `path` already substitutes it from URL params. Ok.
- Confirm whether the `Acquisti RDA approvers I-II` Keycloak group is actually still in use anywhere.

## Migration notes

- The page is the heart of the rewrite. Plan it as a single React route `/rda/po/:poId` with subroutes/tabs for the body. Keep modals as proper dialogs (Item editor, Confirm-submit).
- Replace per-widget `isDisabled` expressions with a derived `permissions` object computed once (state + user_permissions + checkMailIsPresent) and passed down via context or a Zustand-like store.
- Move *all* request-body construction (NewPo, EditPO, CreateItemRow, payment-method translation) to the new Go backend (`backend/internal/rda/`). The frontend should send normalised forms; the backend should be the only place that knows about `payment_detail` vs `renew_detail` shapes.
- Replace the SQL `user_permissions` query with a `/users-int/v1/user/me/permissions` endpoint or include the flags in the Keycloak token (`app_rda_*` roles). **Direct `users_int.user`/`users_int.role` SQL from the client is unacceptable for the rewrite.**
- Drop the manual base64/PDF blob handling in JS by serving signed download URLs from the backend.
- Drop the @-mention "fake" feature unless the backend can accept and process `mentioned_user_ids[]`. If yes, surface it; if not, remove the UI.
- The state-label map should live in a single shared module (Italian labels). The current 4-way duplication is a clear bug.

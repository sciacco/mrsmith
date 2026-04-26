# RDA migration spec — Phase B: UX Pattern Map

**Source:** audit `01_pages_rda_home.md`, `02_page_po_details.md`, `03_pages_approver_inboxes.md`; widget bindings re-verified against `apps/rda/rda.json.gz` where the audit narrative was ambiguous (e.g. `btn_sendOrder` location).
**Approved decisions feeding this phase:** Q-A1 (typo fix), Q-A2/A3 (state-only gates), Q-A4 (use observed shapes), Q-A5 (accept both `comment`/`comment_text`), Q-A6 (mentions cosmetic-only), Q-A7 (hide row-edit pencil), Q-A8 (drop dead motivazione fields), Q-A9–A13 (strict 1:1).

This phase classifies every page-equivalent in the new app, names its primary intent, groups widgets into logical UI sections, and flags dead Appsmith pieces that don't survive the port.

---

## B.1 View inventory and pattern classification

The legacy app has 8 Appsmith pages. The new app reduces to **3 routes** (Home is dropped; the 5 approver inboxes collapse into a single parameterised inbox).

| # | Legacy page(s) | New view | Pattern | User intent |
|---|----------------|----------|---------|-------------|
| 1 | `Home` | — | (dropped) | n/a |
| 2 | `RDA` | **`/rda`** | List + Create-wizard modal | Browse my POs; start a new RDA. |
| 3 | `App. I - II LIV`, `App. Leasing`, `App. metodo pagamento`, `App. no Leasing`, `App. incremento Budget` | **`/rda/inbox/:kind`** | List | Triage POs that need *my* approval action of *kind*. |
| 4 | `PO Details` | **`/rda/po/:poId`** | Master detail editor + tabbed body + workflow action bar | Read or edit a PO end-to-end; act on its current state. |

> The 3-route reduction is itself a pure 1:1 port (same surfaces, same data, same actions); only the *Appsmith plumbing* (per-page query duplication, four `LabelJs` copies, etc.) is collapsed.

---

## B.2 View `/rda` — My POs + New PO wizard

### B.2.1 Intent and pattern

- **User intent:** open the requester landing page; see my POs in any state; start a new RDA. The user is *always* the requester here — even an approver visiting `/rda` sees only their own POs in this list (the approver inboxes live elsewhere).
- **Interaction pattern:** filterable list with row actions (View / Edit / Delete) + a modal wizard for "Nuova richiesta". The wizard is *itself* a 3-section form (PO header, supplier+payment, optional inline new-supplier), and the legacy app keeps it inside a single `MODAL` widget.

### B.2.2 Sections (after collapse)

| § | Section | Widgets in source | New role |
|---|---------|-------------------|----------|
| **B.2.S1** | Page header strip | `Text10` ("Richieste di acquisto") + `Button8` ("nuova richiesta") | h1 + primary action button |
| **B.2.S2** | PO list table | `tbl_po` (`TABLE_V2`) + 3 custom columns (`Modifica`/`Elimina`/`Vedi`) | Data table with per-row actions |
| **B.2.S3** | "Nuova richiesta" wizard (modal) | `ModalNewPO` → `f_new_po` (form) → 3 logical containers | Modal dialog with 3 form sections |
| **B.2.S4** | Inline new-supplier sub-form | `CNT_new_provider`, `CNT_new_provider_data` | Collapsible nested form inside §B.2.S3 |
| **B.2.S5** | Footer (modal) | `btnSaveNewPo`, `BTN_close_mdlNewPO` | Dialog footer |
| ~~B.2.dead~~ | `NuovoFornitore` modal, `Modal1` demo modal | (drop) | — |

### B.2.S1 — Page header strip

- Title text "Richieste di acquisto".
- Primary action: "Nuova richiesta" — opens the wizard modal. No state, no permission gate.

### B.2.S2 — PO list table (`tbl_po`)

Source binding `{{GetPOList.data.items}}` (= `GET /arak/rda/v1/po?disable_pagination=true` with `Requester-Email` header).

**Visible columns (in `columnOrder`):**

| # | Column | Cell renderer | Notes |
|---|--------|---------------|-------|
| 1 | "Modifica" (icon) | pencil button | Disabled unless `requester.email == currentUser` AND `state == 'DRAFT'`. Click → navigate to `/rda/po/:id`. |
| 2 | "Elimina" (icon) | trash button | Same disabled rule. Click → confirm → `DELETE /po/{id}` → refresh table + toast. |
| 3 | "Vedi" (icon) | eye button | Disabled when `state == 'DRAFT'` (drafts are reached via "Modifica"). Click → navigate to `/rda/po/:id`. |
| 4 | "Stato" (text) | Italian label via shared `stateLabel(state)` | — |
| 5 | "Approvatori" (text) | `extractApproverList(approvers)` → `"a@cdlan.it (1), b@cdlan.it (2)"` | — |
| 6 | "Richiedente" (text) | `requester.email` | — |
| 7 | "Data creazione" (date) | format `DD/MM/YYYY` | — |
| 8 | "Numero PO" (text) | `code` | — |
| 9 | "Fornitore" (text) | `provider.company_name` | — |
| 10 | "Progetto" (text) | `project` | — |
| 11 | "Prezzo totale" (currency) | EUR, 2 decimals | F-1: legacy table renders raw `total_price` — may include trailing char. **In the new app: parse to number, render as currency.** |

**Hidden columns** (`isVisible:false` in source, kept in dataset only because some are read by computed columns): `id`, `budget`, `currency`, `payment_method`, `reference_warehouse`, `type`, `updated`, `state`, `approvers`, `description`, `note`, `current_approval_level`, `provider_offer_code`, `provider_offer_date`. The new app **does not** expose them; `state` and `approvers` are reachable through the visible columns above.

**Row-level rules to preserve (B-1):**
- Edit/Delete enabled iff `requester.email == currentUser AND state == 'DRAFT'`.
- View enabled iff `state != 'DRAFT'`.

### B.2.S3 — "Nuova richiesta" wizard modal

Single Appsmith modal `ModalNewPO`. **Pattern:** form-in-dialog, *not* a multi-step wizard (one screen). The "wizard" name in the audit is just the legacy label — there is no Next/Back navigation.

#### Sub-section S3a: PO header

| Field | Source widget | Type | Required | Notes |
|-------|---------------|------|:-------:|-------|
| Budget | `sel_budget` | Select | ✓ | Options from `CallBudget.data.items`. **Legacy stuffs the entire budget object into `value` (stringified JSON)** — the new app uses `budget_id` as `value` and looks up `cost_center`/`budget_user_id` from a parallel map (B-9). |
| Tipo PO | `sl_po_type` | Select | ✓ | Options: `STANDARD` (default), `ECOMMERCE`. |
| Progetto (*) | `inp_project` | Text input | ✓ | `(*)` is in the label text in source; new app uses native `required` + asterisk styling. Max 50 chars (per `rda-create` schema). |
| Oggetto (*) | `txt_object` | Text input | ✓ | Same as above. |

#### Sub-section S3b: Supplier + payment

| Field | Source widget | Type | Required | Notes |
|-------|---------------|------|:-------:|-------|
| Fornitore | `sel_provider` | Select (searchable) | ✓ | Options from `ListaFornitori.data.items` (filtered `usable=true`). On change, fetches `GetProviderDetail` and reveals contact panel. |
| Metodo pagamento default fornitore | `met_default_cli` | Read-only label | — | Shows "Metodo pagamento Default Fornitore: <b>{description}</b>" when present. |
| Metodo pagamento | `inp_payment_method` | Select | ✓ | Options: union of (selected supplier's default) + (CDLAN default from `payment_method_default_cdlan`) + (all `rda_available=true` methods). Default selection: supplier default if any, else CDLAN default. **The hard-coded `"320"` from legacy is dropped (B-10).** |
| Helper banner | `Text13` | Text | — | Shown only when chosen method ≠ CDLAN default `BB60ggFM+10`: "Il PO sarà sottoposto ad approvazione metodo pagamento". |
| (toggle) "Aggiungi nuovo fornitore" | `BTN_new_provider` | Button | — | Reveals S3c (inline new-supplier sub-form). |

#### Sub-section S3c: Inline new-supplier sub-form (collapsible)

Source: `CNT_new_provider` + `CNT_new_provider_data`. Visible only when the user clicked S3b's "Aggiungi nuovo fornitore" button.

Required fields (validated by `Utils.newProviderAdd` in legacy as a chain of `if/else`+`showAlert`; in the new app the validation lives in a single form schema):

| Field | Required | Conditional |
|-------|:-------:|-------------|
| Azienda (ragione sociale) | ✓ | always |
| Indirizzo | ✓ | always |
| Città | ✓ | always |
| Paese | ✓ | default `IT` |
| CAP | ✓ | min 5 chars when `Paese == 'IT'` |
| Lingua (`s_language`) | ✓ | default `it` |
| Provincia | ✓ | only when `Paese == 'IT'` |
| Partita IVA / Codice Fiscale | ✓ | at least one when `Paese == 'IT'` |
| Nome / Cognome / Email referente | ✓ | always |

On Save → `POST /arak/provider-qualification/v1/provider/draft` → refresh `ListaFornitori` → auto-select the newly created supplier. Same legacy behaviour, single error summary instead of sequential `showAlert`s.

#### Sub-section S3d: Hidden defaults

The legacy modal carries two hidden inputs:

- `inp_reference_warehouse` (defaults `MILANO`) — sent in body.
- `s_currency` (locked `eur`) — sent in body as `EUR`.

Per Q-A12 / Q-A13 final answer (1:1): keep both hidden, default unchanged.

#### Sub-section S3e: Footer

- "Crea Bozza" (`btnSaveNewPo`) — primary; calls `Utils.newRdaCreate()` legacy chain, which becomes a single `POST /pos` in the new app, then navigates to `/rda/po/:newId`.
- "Annulla" (`BTN_close_mdlNewPO`) — closes the dialog.

### B.2.S6 — Open questions surfaced by Phase B for `/rda`

None — all wizard fields and rules are now extracted with verified bindings.

---

## B.3 View `/rda/inbox/:kind` — Approver inbox (parameterised)

### B.3.1 Intent and pattern

- **User intent:** as an approver of *kind* X, see only the POs awaiting my action of kind X; click "Gestisci" to drill down.
- **Interaction pattern:** list, no row-create, no row-delete; row action is "drill down".

### B.3.2 The 5 inbox kinds

| `kind` route param | Legacy page name | Endpoint (preserved 1:1) | Title (Italian) | Required role |
|--------------------|------------------|--------------------------|-----------------|----------------|
| `level1-2` | `App. I - II LIV` | `GET /arak/rda/v1/po/pending-approval` | "Approvazioni I° / II° livello" | `app_rda_approver_l1l2` |
| `leasing` | `App. Leasing` | `GET /arak/rda/v1/po/pending-leasing` | "Approvazioni Leasing" | `app_rda_approver_afc` |
| `no-leasing` | `App. no Leasing` | `GET /arak/rda/v1/po/pending-approval-no-leasing` | "Approvazioni No-Leasing" | `app_rda_approver_no_leasing` |
| `payment-method` | `App. metodo pagamento` | `GET /arak/rda/v1/po/pending-approval-payment-method` | "Approvazioni Metodo Pagamento" | `app_rda_approver_afc` |
| `budget-increment` | `App. incremento Budget` | `GET /arak/rda/v1/po-pending-budget-increment` (URL anomaly preserved per Q-A10) | "Approvazioni Incremento Budget" | `app_rda_approver_extra_budget` |

### B.3.3 Sections

| § | Section | New role |
|---|---------|----------|
| **B.3.S1** | Page header | h1 (`title`) + sub-line copy from legacy `Text` widgets, e.g. "RDA che necessitano ancora di approvazioni di I° e/o II° livello. Quando l'RDA è approvata per entrambi i livelli sarà rimossa da questa lista." |
| **B.3.S2** | List table | Same structure as `/rda` table, **without** Modifica / Elimina; with a single "Gestisci" row action. |

### B.3.4 Table columns

A subset of `/rda`:

| # | Column | Notes |
|---|--------|-------|
| 1 | "Gestisci" (icon) | Always enabled; click → `/rda/po/:id`. |
| 2 | "Stato" | shared `stateLabel(state)` |
| 3 | "Richiedente" | `requester.email` |
| 4 | "Data creazione" | `DD/MM/YYYY` |
| 5 | "Numero PO" | `code` |
| 6 | "Fornitore" | `provider.company_name` |
| 7 | "Progetto" | `project` |
| 8 | "Prezzo totale" | currency EUR |

### B.3.5 Permissions and routing

The legacy app shows the table to anyone visiting the page; rows just don't exist for users who can't approve them, because the backend filters by the `Requester-Email` header (which carries the **caller's** email, not the requester's — confusing legacy naming, but the filter does mean "things that need an action from this caller").

For the rewrite: each `/rda/inbox/:kind` route is **gated by the corresponding Keycloak role** (table above). A user without the role gets a 403/redirect; the launcher hides the inbox tile entirely (per `applaunch/catalog.go` access role pattern).

### B.3.6 Open questions for `/rda/inbox/:kind`

None — all five legacy pages reduce to one parameterised view with no behavioural differences other than endpoint, copy, and required role.

---

## B.4 View `/rda/po/:poId` — PO Details

### B.4.1 Intent and pattern

- **User intent:** read or edit the PO; act on its current state (the action set is *state-driven*).
- **Interaction pattern:** master detail editor with a top-mounted action bar, an editable header strip, a tabbed body, and a comments side panel — a classic "document editor" page.

The legacy DSL has 8 top-level children + 4 hidden orphan containers + 3 modals. The new view groups everything into 5 user-facing regions:

| § | New region | Maps to legacy widgets | New role |
|---|------------|------------------------|----------|
| **B.4.S1** | Action bar (state-driven) | `Container2` action area (`ButtonGroup1` approvals, `ButtonGroup1Copy` rejections, solo buttons except `btn_sendOrder`) | Top sticky bar with state-conditional buttons |
| **B.4.S2** | Editable header form | `Container23` / `cnt_budgetAndInfo` / `Container26` / `cnt_fornitore` / `Container30` (incl. `btn_sendOrder` per real DSL location) | 2-column form |
| **B.4.S3** | Tabbed body | `tabs_details` (Allegati / Righe PO / Note / Contatti Fornitore) | Tabs |
| **B.4.S4** | Item editor dialog | `mdl_edit_item` modal | Dialog |
| **B.4.S5** | Comments panel | `Container7` (`Input2` + `Button2` + `List1`) | Side panel |
| ~~B.4.dead~~ | `Container17/19/24`, `Modal1`, `mdl_supplierContact`, `lst_itemsCopy`, hidden tab `tabd3n2bz6elj`, motivazione fields | (drop) | — |

### B.4.S1 — Action bar (state-driven)

The legacy page has 4 *solo* buttons + 2 button-groups + 1 banner. **Action availability is the union of *state* and *role*** (with one no-role-no-state action: "Chiudi"). This is the spec's most critical region.

| Action | Label (IT) | Endpoint | Visible / Enabled when | Required role | After |
|--------|------------|----------|------------------------|----------------|-------|
| Save draft | "Aggiorna Bozza PO" | `PATCH /po/{id}` | `state == DRAFT` | requester | reload PO + reload contacts |
| Submit draft | "Manda PO in Approvazione" | `EditPO` then `POST /po/{id}/submit` (via confirm modal) | `state == DRAFT` AND has rows AND (3-quote rule satisfied per B-2) | requester | toast + back to `/rda` |
| Approve L1 | "Approva (Liv 1)" | `POST /po/{id}/approve` | `state == PENDING_APPROVAL` AND `current_approval_level == 1` AND email ∈ approvers | `app_rda_approver_l1l2` | toast + `/rda/inbox/level1-2` |
| Approve L2 | "Approva (Liv 2)" | `POST /po/{id}/approve` | same with `level == 2` | `app_rda_approver_l1l2` | same |
| Reject L1/L2 | "Rifiuta (Liv 1)" / "Rifiuta (Liv 2)" | `POST /po/{id}/reject` | as above | `app_rda_approver_l1l2` | same |
| Approve payment | "Approva metodo pagamento" | `POST /po/{id}/payment-method/approve` | `state == PENDING_APPROVAL_PAYMENT_METHOD` | `app_rda_approver_afc` | `/rda/inbox/payment-method` |
| Reject payment | "Rifiuta metodo pagamento" | `POST /po/{id}/reject` | same | `app_rda_approver_afc` | same |
| Update payment method | "Aggiorna metodo di pagamento" (`BRT_upd_pagamento`) | `PATCH /po/{id}/payment-method` | `state == PENDING_APPROVAL_PAYMENT_METHOD` AND requester | requester | reload PO |
| Approve leasing | "Approva leasing" | `POST /po/{id}/leasing/approve` | `state == PENDING_LEASING` | `app_rda_approver_afc` | `/rda/inbox/leasing` |
| Reject leasing | "Rifiuta leasing" | `POST /po/{id}/leasing/reject` | same | `app_rda_approver_afc` | same |
| Approve no-leasing | "Approva no leasing" | `POST /po/{id}/no-leasing/approve` | `state == PENDING_APPROVAL_NO_LEASING` | `app_rda_approver_no_leasing` | `/rda/inbox/no-leasing` |
| Reject no-leasing | "Rifiuta no leasing" | `POST /po/{id}/reject` (per Q-A9, 1:1) | same | `app_rda_approver_no_leasing` | same |
| Approve budget incr. | "Approva incremento budget" | `POST /po/{id}/approve-budget-increment` body `{increment_promise}` (from URL param `budget_increment_needed`) | `state == PENDING_BUDGET_INCREMENT` | `app_rda_approver_extra_budget` | `/rda/inbox/budget-increment` |
| Reject budget incr. | "Rifiuta incremento budget" | `POST /po/{id}/reject-budget-increment` (same body) | same | same | same |
| Mark leasing created | "Leasing Creato" | `POST /po/{id}/leasing/created` | `state == PENDING_LEASING_ORDER_CREATION` | `app_rda_approver_afc` | reload PO |
| Send to supplier | "Invia ordine al fornitore" (`btn_sendOrder`) | `POST /po/{id}/send-to-provider` | `state == PENDING_SEND` | (none — Q-A2 confirmed) | toast + back to `/rda` |
| Confirm conformity | "Erogato e conforme" | `POST /po/{id}/confirm-conformity` | `state == PENDING_VERIFICATION` | (none — Q-A3 confirmed) | reload PO; on error: toast "verifica inserimento DDT" (B-4) |
| Reject conformity | "In contestazione" | `POST /po/{id}/reject-conformity` | `state == PENDING_VERIFICATION` | (none) | reload PO |
| Generate PDF | "Genera PDF" | `GET /po/{id}/download` | `state != DRAFT` | (none) | browser download |
| Close (back) | "Chiudi" | — | always | (none) | back to `/rda` |

**Submit-draft prereqs banner.** When the submit button is disabled because of B-2 (3-quote rule), the new app shows the same banner copy: "Attenzione: importo superiore a 3.000 €. Aggiungi 2 preventivi". The banner only triggers on `state == DRAFT`, count of `attachments` (per Q-A11 final = all attachments, 1:1).

**Approver guard (B-7).** Approve/Reject buttons are visible only when `currentUser.email ∈ GetPoDetails.data.approvers[*].user.email`. This is *in addition to* the Keycloak role gate, because the role tells us "this user can approve POs of this kind", and the per-PO approvers list tells us "this user is *the* approver for *this* PO".

**Confirm dialog (`modal_confirmSendToApprovers`).** "Manda in Approvazione" pops a confirm dialog before the actual submit. New app keeps it as an `<AlertDialog>`. Title from legacy `Text33`/Modal text: "Confermi l'invio del PO in approvazione?".

### B.4.S2 — Editable header form

Two-column form. **All fields read-only when `state != DRAFT`** unless flagged below.

| Field | Source widget | Editable when | Notes |
|-------|---------------|----------------|-------|
| Numero PO + data + stato (read-only banner) | `Text22Copy` | always read-only | `Ordine Numero: {code} del {DD/MM/YYYY HH:mm} — Stato Attuale : {stateLabel}` |
| Approvatori (L1 / L2 emails) | `Text23` (computed inline) | always read-only | derived from `approvers[]` filtered by `level === '1' / === '2'` |
| Budget | `s_budget` | DRAFT | F-2 fix: use `budget_id` consistently as option `value`. |
| Oggetto | `f_subject` | DRAFT | |
| Progetto | `project` | DRAFT | |
| Fornitore | `provider` | DRAFT | onChange clears `recipient_ids` (B server-side) and refetches provider detail |
| Metodo pagamento | `s_payment_method` | DRAFT **OR** `PENDING_APPROVAL_PAYMENT_METHOD` | Only state where this field is editable outside DRAFT (B-8). |
| (button) "Aggiorna metodo pagamento" | `BRT_upd_pagamento` | `PENDING_APPROVAL_PAYMENT_METHOD` AND requester | calls `PATCH /payment-method`; see action bar above |
| Riferimento preventivo | `rif_quoteSupplier` | DRAFT | maps to `provider_offer_code` |
| Data preventivo | `rif_datequoteSupplier` | DRAFT | `provider_offer_date` |
| Recipients summary | `Text27` (legacy: HTML stuffed via `setText`) | always read-only | new app: React component listing recipient names+emails or a "verrà utilizzato il referente di qualifica" caption when empty (B-5). XSS hole F-10 closed by definition. |
| (button) "Invia ordine al fornitore" | `btn_sendOrder` | `state == PENDING_SEND` | actual DSL location is here, in `cnt_fornitore`; see action bar |
| Helper text | `Text29` | always | "Per aggiungere ulteriori contatti selezionarli dalla tab 'Contatti Fornitore' qui sotto" |

### B.4.S3 — Tabbed body

#### Tab 1 — "Allegati"

| Element | Source | Notes |
|---------|--------|-------|
| Banner copy (3-quote rule reminder) | `Text18` | always visible: "Per importi maggiori di 3.000 € sono necessari almeno 3 preventivi." |
| Upload file picker | `upload_btn_prv` | enabled when `state ∈ {DRAFT, PENDING_VERIFICATION}`. **Auto-tag rule (B-3):** DRAFT → `quote`, otherwise → `transport_document`. |
| Refresh icon | `IconButton3Copy` | F-3 fixed: enabled unless `state ∈ {PENDING_DISPUTE, DELIVERED_AND_COMPLIANT}`. |
| Attachments table | `tbl_attachment` | columns: file name, type, created_at (`DD/MM/YYYY`); per-row actions Elimina (DELETE; DRAFT only) and Scarica (download — backend returns signed URL or stream, no client-side base64 juggling). |

Dropped: motivazione fields (Q-A8).

#### Tab 2 — "Righe PO"

| Element | Source | Notes |
|---------|--------|-------|
| Add row icon (+) | `IconButton2` | DRAFT only |
| Total banner | `PO_details_TotalAmount` | F-1: parse `total_price` to number (no `slice(0,-1)` workaround) |
| 3-quote rule reminder | `Text19` | always visible |
| Refresh icon | `IconButton3` | always |
| Rows table | `Table2` | columns described below |

**Columns:** description (HTML), activation_fee ("Costo unitario / NRC"), montly_fee ("MRC"), qty ("Q.tà"), type ("Tipo"), total ("Totale riga", **backend-supplied per-row total**, no client-side recompute). Per-row actions:

- **Modifica** (pencil): **hidden in v1** per Q-A7; user message "modifica non disponibile, eliminare e ricreare la riga".
- **Elimina** (trash): DELETE row; DRAFT only.

#### Tab 3 — "Note"

Two textareas:
- `txt_note` → `note` (sent to provider).
- `rt_description` → `description` (internal).

Both editable only when `state == DRAFT`.

#### Tab 4 — "Contatti Fornitore"

| Element | Source | Notes |
|---------|--------|-------|
| Provider refs table | `Table4` | bound to `GetProviderDetail.data.refs`. |
| | columns | `email` (regex validated `^\+[1-9][0-9]{4,19}$` is on `phone`, not email — preserve), `first_name`, `last_name`, `phone`, `reference_type` (select with options from shared `allCategory` / `availableCategory`). |
| | Per-row Save | only when `reference_type != 'QUALIFICATION_REF' AND state == 'DRAFT'` (B-12). |
| | Add row | only when `state == DRAFT`. New rows can pick from `availableCategory` (excludes `QUALIFICATION_REF`). |
| Recipients selection | `defaultSelectedRowIndices` derived from `recipients[]` | `Salva contatti selezionati` button (`Button10`): DRAFT only; calls `PATCH /po/{id}` with `recipient_ids = selectedRows.map(r => r.id)`. |
| Helper text | `Text24` | "Seleziona nella tabella i contatti a cui inviare l'ordine. Se non viene spuntato alcun contatto, verrà utilizzato il contatto di tipo qualifica." (B-5) |

#### Hidden / dropped tab

`tabd3n2bz6elj` ("Contatt") is hidden in source — drop entirely.

### B.4.S4 — Item editor dialog (`mdl_edit_item`)

Dialog opens from S3 Tab 2's `+` icon. Same form for both `good` and `service` with conditional fields.

| Field | Type | When | Required | Notes |
|-------|------|------|:-------:|-------|
| `sl_item_type` | Select `good`/`service` | always | ✓ | onChange triggers article catalog fetch |
| `sl_product` | Select | always | — | options from `GET /article?type=...` |
| `rt_item_description` | Textarea | always | ✓ | line description |
| `f_item_qty` | Number | always | ✓ | default 1 |
| `f_item_unit_price` | Number | `good` only | ✓ for `good` | unit price |
| `f_activation_priceNRC` | Number | `service` only | ✓ if MRC == 0 | NRC |
| `f_price_mrc` | Number | `service` only | ✓ if NRC == 0 | MRC |
| `f_months_first_period` | Number | `service` only | ✓ | "Durata (mesi)"; default 1 |
| `sl_recurring_months` | Select 1/3/6/12 | `service` only | ✓ | Mensile/Trimestrale/Semestrale/Annuale |
| `sl_start_at` | Select | always | ✓ | options depend on type: service=`activation_date`/`specific_date`; good=`activation_date`/`advance_payment`/`specific_date` |
| `f_start_at_date1` | Date | always (UI), enabled iff `sl_start_at == specific_date` | ✓ if specific_date | format `YYYY-MM-DD` |
| `sw_auto_renew` | Switch | `service` only | — | when on, enables `f_cancel_before_days` |
| `f_cancel_before_days` | Number | `service` only AND `sw_auto_renew == on` | ✓ in that case | preavviso disdetta in days |
| Live total preview | Text | always | — | `service`: `(MRC × qty × duration) + (NRC × qty)`. `good`: `unit_price × qty`. **(Preview only — backend total is the source of truth.)** |
| Save button | `btnSaveItem` | — | — | disabled when `service && NRC == 0 && MRC == 0`. Calls `POST /po/{id}/row`. |
| Cancel button | — | — | — | closes dialog |

**Edit mode hidden in v1** (Q-A7). The legacy `storeValue('curr_item', currentRow)` flow does not survive.

### B.4.S5 — Comments panel

Single side region. Shows the thread top-down, oldest first. Replies (single-level) render indented under the parent.

| Element | Source | Notes |
|---------|--------|-------|
| Comment input | `Input2` | Textarea + @-suggest popup |
| @-suggest popup | `List2` / `List3` (driven by `JSObject1.showMentions`) | onTextChanged detects `@…`; runs `UserQuery` with `search_string`. Picking a user replaces the trailing `@token` with `@user.email `. |
| Save button | `Button2` | `POST /po/{id}/comment` body `{comment}` (Q-A6 cosmetic only — `mentioned_user_ids[]` is **not** sent in v1). |
| Thread list | `List1` | `GET /po/{id}/comment` → normalised reader (accepts `comment` *or* `comment_text` per Q-A5). Each item: avatar/initials, user name + email + timestamp, comment body, replies. |

The legacy three-list duplication (`List1` / `List2` / `List3`) collapses into one list + one popup component.

---

## B.5 Cross-view shared components (extracted from this Phase)

These names are platform-neutral (no React/Vue jargon); the implementation phase decides shape.

| Shared component | Used in | Purpose |
|------------------|---------|---------|
| `PoListTable` | `/rda`, `/rda/inbox/:kind` | The 6 near-copies of `tbl_po` in legacy collapse into one parameterised list, with column visibility and row-action set as inputs. |
| `StateBadge` | all 3 views | Renders Italian state label (single source — closes D-1). |
| `ActionBar` | `/rda/po/:poId` | State-driven buttons + role + per-PO approver guard (B-6/B-7/B-8). |
| `RecipientsList` | `/rda/po/:poId` (S2) | Renders recipients (no HTML injection) — closes F-10. |
| `MentionInput` | `/rda/po/:poId` (S5) | Textarea + @-suggest popup; Q-A6 sends body without IDs in v1. |
| `ProviderRefTable` | `/rda/po/:poId` (S3 tab 4) | Editable provider-references table with Save/Discard inline editing; B-12 read-only QUALIFICATION_REF rule baked in. |
| `BudgetSelect` | `/rda` modal, `/rda/po/:poId` header | Renders user budgets; emits `{budget_id, cost_center?, budget_user_id?}` (B-9 mutex enforced upstream). |
| `PaymentMethodSelect` | `/rda` modal, `/rda/po/:poId` header | Merges supplier default + CDLAN default + active methods; default selection rule (B-10). |

---

## B.6 What's dropped (already finalised)

- `Home` page entirely.
- `Modal1`, `mdl_supplierContact`, `NuovoFornitore` modal (legacy duplicate), hidden tab `tabd3n2bz6elj`.
- Hidden orphan containers `Container17`, `Container19`, `Container24`.
- Hidden duplicate `lst_itemsCopy` (alternate List-V2 of the rows table).
- Motivazione fields (Q-A8): `select_motivation`, `input_motivazion` (verified dead via export inspection).
- `f_date` (read-only date in `Container19`) — bug F-11 disappears with the container.
- The hard-coded `"320"` payment-method literal (B-10).
- "Modifica riga" pencil in v1 (Q-A7).
- Mentioned-user-id transmission (Q-A6).

---

## B.7 Open questions surfaced by Phase B

The audit was sufficient. The new questions that arose during this phase are *not* business questions; they're routing/UX detail decisions that downstream phases (planning) absorb. Listed for completeness:

- **B.Q-1** (cosmetic): for `/rda/inbox/:kind` access, do we render the inbox tile in the launcher only when the user has *that* role, or do we always show it and 403 on click? Per existing portal convention (`applaunch/catalog.go` access role pattern), the launcher hides tiles the user lacks access to. Phase D will confirm against `catalog.go`.
- **B.Q-2** (cosmetic): the legacy "Approvazioni" labels include the degree symbol (`Approvazioni I° / II° livello`). Preserve as-is.

These do not block Phase C.

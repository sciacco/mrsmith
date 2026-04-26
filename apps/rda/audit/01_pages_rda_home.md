# Page audit — `Home` and `RDA`

## Page: `Home`

**Purpose:** entry-point page; in practice empty. Holds a single `Chart1` widget on a CANVAS. There is no data binding for the chart, no onLoad action, and no datasource consumption from this page. The Home page is **not used as a real homepage** today; users land on the RDA page via portal navigation.

**Migration note:** drop entirely; the new portal already provides the home/launcher.

---

## Page: `RDA`

**Purpose:** primary list of Purchase Orders for the logged-in user, plus the "Nuova richiesta" wizard implemented as a modal that lets the user pick a budget, fill PO header data, choose a supplier (or create one inline), and then drill into `PO Details` to add items and submit.

### onLoad actions

Triggered automatically when the page loads:

| Query | Datasource | Purpose |
|-------|-----------|---------|
| `CallBudget` | Arak REST `GET /arak/budget/v1/budget-for-user` (header `user_email: {{appsmith.user.email}}`, `page_number=1`) | Fetch the budgets the current user can spend against. |
| `ListaFornitori` | Arak REST `GET /arak/provider-qualification/v1/provider?disable_pagination=true&usable=true&page_number=1` | List of qualified suppliers. |
| `PaymentMethonds` | PostgreSQL `SELECT * FROM provider_qualifications.payment_method WHERE rda_available IS TRUE` | Allowed payment methods for RDA. |
| `GetPOList` | Arak REST `GET /arak/rda/v1/po?disable_pagination=true` (header `Requester-Email: {{appsmith.user.email}}`) | The user's PO list. |
| `GetDefaultPaymentMethod` | PostgreSQL `SELECT payment_method_code FROM provider_qualifications.payment_method_default_cdlan` | The CDLAN-default payment method code (used as fallback). |

### Top-level widgets

| Widget | Type | Role |
|--------|------|------|
| `Text10` | TEXT | Page title "Richieste di acquisto". |
| `Button8` | BUTTON | "nuova richiesta" → `showModal('ModalNewPO')` and hides the inline `CNT_new_provider` panel. |
| `tbl_po` | TABLE_V2 | The PO list (drives the whole page). Bound to `{{GetPOList.data.items}}`. See "Table tbl_po" below. |
| `ModalNewPO` | MODAL | Wraps the New-PO wizard form. |
| `NuovoFornitore` | MODAL | Mostly-empty companion modal (legacy/unused). The actual new-supplier UI lives **inside** `ModalNewPO` via `CNT_new_provider`. |
| `Modal1` | MODAL | Demo/scaffolding modal (Confirm/Close), unused. |

### Table `tbl_po`

Bound to `{{GetPOList.data.items}}`. Visible columns (in `columnOrder`):

1. **`customColumn1` "Modifica" (iconButton)** — `isDisabled` when `currentRow.requester.email != currentUser` OR `state != 'DRAFT'`. `onClick` → `navigateTo('PO Details', { po_id: currentRow.id })`.
2. **`customColumn2` "Elimina" (iconButton)** — same disabled rule as Modifica. `onClick` → `DeletePO.run({id})` then `GetPOList.run()` and toast.
3. **`customColumn8` "Vedi" (iconButton)** — `isDisabled` when `state == 'DRAFT'` (drafts are edited via the pencil; everything else is read-only-ish). `onClick` → `navigateTo('PO Details', { po_id })`.
4. **`customColumn3` "Stato" (text)** — `LabelJs.translate(currentRow.state)` → Italian state label.
5. **`customColumn4` "Approvatori" (text)** — joined string `email (level), …` produced by `Utils.extractApproverList(currentRow.approvers)`.
6. **`requester` "Richiedente" (text)** — `currentRow.requester.email`.
7. **`created` "Data creazione" (date)** — formatted `DD/MM/YYYY`.
8. **`code` "Numero PO" (text)** — `currentRow.code`.
9. **`provider` "Fornitore" (text)** — `currentRow.provider.company_name`.
10. **`project` "Progetto" (text)** — `currentRow.project`.
11. **`total_price` "Prezzo totale" (currency, EUR, 2 decimals)** — note: backend currently returns this with a trailing character (see PO Details `PO_details_TotalAmount` which does `total_price.slice(0,-1)`); on the table column the slice does **not** happen, so the rendered total may include a trailing character. *Migration risk.*

Hidden columns kept in the dataset but `isVisible:false`: `id`, `budget`, `currency`, `payment_method`, `reference_warehouse`, `type`, `updated`, `state`, `approvers`, `description`, `note`, `current_approval_level`, `provider_offer_code`, `provider_offer_date`. Most of these are only used either as derived/computed sources (e.g. `state` feeds `customColumn3`) or are dead columns.

The `budget` and `payment_method` computed values do gymnastics to handle three shapes (`null`, `string-JSON`, `object`, `array`), suggesting backend inconsistencies. Document this when designing the new backend response.

### Modal `ModalNewPO` — "Nuova richiesta"

Hosts a `FORM_WIDGET` (`f_new_po`) and is split in three logical containers:

#### 1. PO header (`Container1`/`Container6`)

- `sel_budget` — required SELECT, `sourceData = CallBudget.data.items.map(b => ({label: b.name, value: JSON.stringify(b)}))`. Stuffing the entire object into the option value is a workaround for Appsmith select limitations and is parsed back inside `NewPo` body. *Migration risk: replace with normalized `{value: budget_id, …}` and a separate map.*
- `sl_po_type` — SELECT defaulted to `STANDARD`. Two options: `STANDARD` (with order to provider) and `ECOMMERCE` (no order to provider). The choice is sent as `type` in `NewPo`.
- `inp_project` (required, label "Progetto (*)"), `txt_object` (required, label "Oggetto (*)"). Note the `(*)` is in the label text rather than via `isRequired`.

#### 2. Supplier + payment (`Container7`/`Container8`)

- `sel_provider` — required SELECT bound to `ListaFornitori.data.items`. `onOptionChange`: `storeValue('selectedSupplier', value)` then `Contact.getProviderData(...)` and shows `Container10` (qualification refs).
- `inp_payment_method` — SELECT, options computed from:
  1. The selected supplier's `default_payment_method.code` (if present) — labelled "Metodo default fornitore".
  2. `GetDefaultPaymentMethod.data[0].payment_method_code` (CDLAN default).
  3. Every active method from `PaymentMethonds`.
  Default option is the supplier default if any, otherwise the hard-coded `"320"` (a payment-method code that **must not be re-introduced** as a literal in the rewrite — pull it from data).
- `Text13` (helper text): if the selected method ≠ CDLAN standard "BB60ggFM+10", the PO will route through payment-method approval.
- `met_default_cli` text: shows "Metodo pagamento Default Fornitore: <b>{{description}}</b>" on the fly.
- `BTN_new_provider` button — opens `CNT_new_provider` (inline supplier creation form, see below).

#### 3. Inline new-supplier form (`CNT_new_provider`/`CNT_new_provider_data`)

A nested form to create a new provider on the fly. Required fields (validated by `Utils.newProviderAdd`):
`Azienda` (ragione sociale), `Address`, `Citta`, `Paese` (default `IT`), `CAP` (must be ≥5 chars when `Paese=IT`), `s_language` (default `it`), `NomeReferente`, `CognomeReferente`, `EmailReferente`, `Provincia` (only if `Paese=IT`), and at least one of `PartitaIva` or `CodiceFiscale` when `Paese=IT`. The validation lives in the JSObject **as a chain of if/else** with `showAlert` calls — it is sequential, so users only see one error at a time. *Migration: replace with proper form-level validation.*
On Save → `nuovoFornitore` (POST `/arak/provider-qualification/v1/provider/draft`) then `ListaFornitori.run()` to refresh.

#### 4. Modal footer

- `btnSaveNewPo` "Crea Bozza" → `Utils.newRdaCreate()` (validates required fields, calls `NewPo`, closes modal, refreshes table, navigates to PO Details with the new id).
- `BTN_close_mdlNewPO` "Annulla" → `closeModal('ModalNewPO')`.
- `inp_reference_warehouse` (default `MILANO`) and `s_currency` (locked to `eur`) are present but `isVisible:false`. The values are still picked up by `NewPo` body, so the *implicit defaults* are EUR + MILANO.

### Hidden / disabled / state-driven logic

- "Modifica" and "Elimina" disabled on rows where the current user is not the requester or the PO is not DRAFT. *Business rule:* "**Only the requester can edit/delete, and only while DRAFT**".
- "Vedi" disabled on DRAFT rows — DRAFT rows are reached via "Modifica" instead.
- The dropdown to pick the budget stores a stringified object in the option value; the create-PO body parses it back and decides whether to send `cost_center` (if present) or `budget_user_id`. *Business rule:* "**A PO is bound to a budget; if the budget has a cost center, that's used; otherwise the budget is bound to a specific user.**"

### `NewPo` request body (POST `/arak/rda/v1/po`)

Constructed inline with extensive fallbacks. Notable fields:
- `type`: `sl_po_type.selectedOptionValue`
- `project`, `object`
- `reference_warehouse`: `inp_reference_warehouse.selectedOptionValue` (defaults to `MILANO` even if hidden)
- `language`: derived from `ListaFornitori.data.items.find(... id == sel_provider).language || 'it'`
- `payment_method`: explicitly chosen value, or supplier default code, or CDLAN default
- `cap`, `vat`: forwarded from the (often hidden) supplier panel
- `recipient_ids: []` (always empty at create-time)
- `provider_id`: number-cast `sel_provider.selectedOptionValue`
- `budget_id` / `cost_center` / `budget_user_id` chosen exclusively (mutex)

### Open questions

- The `ModalNewPO` allows creating a "DRAFT" PO **before** any items are added. Items are added inside `PO Details`. Is this two-step flow desired in the new app or do we collapse to a single-page wizard?
- `inp_reference_warehouse` is hidden but always sent. Is `reference_warehouse` actually used downstream (warehouse for goods)? It defaults to `MILANO`. Confirm with backend.
- `s_currency` is hard-locked to EUR; the request body hard-codes `currency:"EUR"`. Multi-currency is *not* a current requirement.
- `Modal1` and `NuovoFornitore` are mostly stale Appsmith scaffolding. Do not port.
- Why does `Utils.newProvider()` set `sel_provider.setSelectedOption('')` when opening the inline supplier form? *Inferred:* to let the operator add a brand-new supplier and have it auto-selected after `ListaFornitori.run()`. Worth replicating.

### Migration notes

- Page = a list view + a "create" wizard. In React, prefer a single-page flow with two sub-forms: **Step 1 (PO header + supplier choice)** and **Step 2 (items)** on PO Details. Inline supplier creation should remain available because the alternative (round-tripping to a separate Anagrafica) is rejected by today's UX.
- The list table is shared across 5 pages with subtle differences (column visibility, drill-down target, action buttons). Build a single `<PoListTable>` component and pass props for visible columns and row-actions.
- Replace direct `users_int.user`/`users_int.role` SQL by a backend endpoint that returns the current user's permission flags.
- Replace stringified-JSON option values with proper structured option types in the React Select.
- Drop the `defaultOptionValue: "320"` literal — read from `payment_method_default_cdlan` only.

# Customer Portal — Per-Page Audit

Naming preserved from Appsmith. Bindings (`{{ … }}`) are evidence, not implementation guidance. Each finding is tagged:
- **[B]** business logic
- **[O]** frontend orchestration
- **[P]** presentation-only

---

## 1. Home

**Purpose**: landing page for the admin app. No data, no actions.

**Widgets**
- `MainContainer / Text1` — copy: `Welcome!!\nTi trovi nella applicazione che ti permette di gestire i contenuti del Customer Portal`

**Queries / actions**: none.
**Event flow**: none.
**Page-load actions**: none.
**Hidden logic**: none.

**Candidate entities**: —
**Migration notes**: trivially replaceable; in the rewrite this should become either the app shell's dashboard or be dropped in favor of routing straight to "Stato Aziende".

---

## 2. Stato Aziende

**Purpose**: list all customers, open a modal to change a customer's "state" (active / suspended / etc.), persist via REST PUT.

### Widgets (tree)
- `MainContainer`
  - `t_customer_list` — `TABLE_WIDGET_V2`, driven by `{{GetAllCustomer.data.items}}`
  - `edit_customer` — `BUTTON_WIDGET`, text `Aggiorna {{t_customer_list.selectedRow.name}}`, onClick `showModal(m_edit_customer.name)`
  - `m_edit_customer` — `MODAL_WIDGET`
    - `Text1` — `Aggiorna {{t_customer_list.selectedRow.name}}`
    - `IconButton1` — close
    - `edit_customer_state` — `SELECT_WIDGET`, `sourceData = {{GetAllCustomerState.data.items}}`, `optionLabel = name`, `optionValue = id`
    - `edit_customer_confirm` — button "Conferma", onClick chain:
      ```
      EditCustomer.run()
        .then(() => { GetAllCustomer.run(); closeModal(m_edit_customer.name); })
        .catch(() => showAlert('Failed to edit Customer, [HTTP ' + EditCustomer.responseMeta.statusCode + ']: ' + EditCustomer.data.message, 'error'))
      ```

### Table columns (`t_customer_list.primaryColumns`)
| alias | label | type | visible | notes |
|-------|-------|------|---------|-------|
| `group` | group | text | **no** | raw nested object kept hidden, feeds `customColumn1` |
| `id` | id | number | yes | exposed internal identifier |
| `language` | Lingua | text | yes | |
| `name` | Ragione sociale | text | yes | |
| `variables` | variables | text | **no** | raw, hidden |
| `state` | state | text | **no** | raw nested object, feeds `customColumn2` |
| `customColumn1` | Tipologia | text | yes | computed: `currentRow["group"].name` — **[P]** unwrap nested object |
| `customColumn2` | Stato | text | yes | computed: `currentRow["state"].name` — **[P]** unwrap nested object |

### Page-load actions
Parallel group: `GetAllCustomer`, `GetAllCustomerState`.

### Event flow
1. Page load → fetch customers + customer states (parallel).
2. User selects a row → `t_customer_list.selectedRow` is populated → button label updates dynamically (`Aggiorna <name>`).
3. Click "Aggiorna …" → opens modal.
4. User selects a state in `edit_customer_state` → `selectedOptionValue` = state id.
5. Click "Conferma" → `EditCustomer` PUT → on success refresh table + close modal; on error toast HTTP status + message.

### Bindings / hidden logic
- **[O]** `EditCustomer` path is `/customers/v2/customer/{{t_customer_list.selectedRow.id}}` — the write target is the currently selected row; no guard against an empty selection (button does not appear disabled in the DSL).
- **[O]** Body: `{ state_id: edit_customer_state.selectedOptionValue }` — the only field the UI can change is `state_id`.
- **[B]** The backend REST endpoint presumably enforces allowed transitions; nothing client-side validates the new state.
- **[P]** "Tipologia" and "Stato" columns exist only because the API returns embedded objects `{group: {name,…}, state: {name,…}}` — rewrite can flatten at the API client level.

### Candidate domain entities
- **Customer** (`id`, `name`, `language`, `group`, `state`, `variables`) served by `/customers/v2/customer`.
- **Customer state** lookup (`id`, `name`) served by `/customers/v2/customer-state`.

### Open questions / ambiguities
- Is the "Aggiorna" button expected to be disabled until a row is selected? DSL does not bind `isDisabled`.
- What are the semantics of `group` vs. `state`? Both are nested objects with `name`; likely `group = typology`, `state = lifecycle status`. Needs confirmation from the REST contract.
- `variables` column is carried through but hidden — is this used downstream?

### Migration notes
- Read: single `GET /customers` + `GET /customer-states` — move pagination handling server-side.
- Write: single `PATCH /customers/{id}` with `{ state_id }` — server must validate allowed transitions.
- Drop computed columns; return flat DTOs (`typology_name`, `state_name`).
- Add row-selection guard and optimistic update for snappier UX.

---

## 3. Gestione Utenti

**Purpose**: pick a customer, list its users, optionally create a new admin user for that customer.

### Widgets (tree)
- `MainContainer`
  - `select_customer` — `SELECT_WIDGET`, `sourceData = {{GetAllCustomer.data.items}}`, `optionLabel = name`, `optionValue = id`, `labelText = Azienda`
  - `t_user_list` — `TABLE_WIDGET_V2`, `tableData = {{GetAllUsersByCustomer.data.items}}`
  - `new_user_button` — "Nuovo Admin", onClick `showModal(m_new_admin.name)`
  - `m_new_admin` — `MODAL_WIDGET`
    - `Canvas1`
      - `new_user_first_name` (INPUT) — "Nome"
      - `new_user_last_name` (INPUT) — "Cognome"
      - `new_user_email` (INPUT) — "Em@il"
      - `new_user_phone` (PHONE_INPUT) — "Telefono"
      - `new_user_notifications` (CHECKBOX_GROUP) — `[{label:'Manutenzioni',value:'maintenance'},{label:'Marketing',value:'marketing'}]`
      - `new_user_skip_kc` (SWITCH) — "Non creare account su KC"
      - `new_user_create` (BUTTON) — "Crea", onClick:
        ```
        NewAdmin.run()
          .then(() => { GetAllUsersByCustomer.run(); closeModal(m_new_admin.name); })
          .catch(() => showAlert('Failed to create Admin, [HTTP ' + NewAdmin.responseMeta.statusCode + ']: ' + NewAdmin.data.message, 'error'))
        ```
      - `IconButton1` — close
      - `Text1` — "Crea Utente Admin"
  - `Text2` — `Ciao {{appsmith.user.name || appsmith.user.email}}, in questa applicazione vengono visualizzati tutti gli utenti inseriti sul Customer Portal per l'azienda selezionata - da indicare tramite la select`

### Table columns (`t_user_list.primaryColumns`)
| alias | label | type | visible | notes |
|-------|-------|------|---------|-------|
| `created` | Creato il | text | yes | raw ISO/string from API, **[P]** no client-side format |
| `customer_id` | customer_id | number | **no** | hidden |
| `email` | email | text | yes | |
| `enabled` | Accesso CP abilitato | checkbox | yes | read-only display of user enablement |
| `first_name` | Nome | text | yes | |
| `last_name` | Cognome | text | yes | |
| `id` | id | number | **no** | hidden |
| `phone` | Telefono | text | **no** | hidden |
| `role` | role | text | **no** | hidden (feeds `customColumn1`) |
| `customColumn1` | nome ruolo | text | yes | computed: `currentRow["role"].name` — **[P]** unwrap nested `role` object |

### Page-load actions
Sequential groups: `[GetAllCustomer]`, then `[GetAllUsersByCustomer]`.

> **[O] Hidden orchestration rule**: `GetAllUsersByCustomer` is both a page-load action *and* depends on `select_customer.selectedOptionValue`. At first load `select_customer` has no value, so the query runs with `customer_id=undefined`. Appsmith's URL-encoding will either omit the param or send the string `undefined`. This likely returns an empty list or a 400; behavior needs verification. **The page has no explicit onChange for `select_customer`** — Appsmith re-evaluates queries on binding changes automatically, which the rewrite must replicate.

### Event flow
1. Page load → `GetAllCustomer` then `GetAllUsersByCustomer` (initially with empty `customer_id`).
2. User picks a customer in `select_customer` → `selectedOptionValue` changes → `GetAllUsersByCustomer` auto-reruns via binding → `t_user_list` re-renders.
3. Click "Nuovo Admin" → modal opens.
4. Fill form → click "Crea" → `NewAdmin` POST → on success re-fetch users, close modal; on error toast HTTP status + message.

### `NewAdmin` body (verbatim binding)
```
{
  first_name: new_user_first_name.text,
  last_name: new_user_last_name.text,
  email: new_user_email.text,
  customer_id: select_customer.selectedOptionValue,
  phone: new_user_phone.text,
  maintenance_on_primary_email: new_user_notifications.selectedValues.includes("maintenance"),
  marketing_on_primary_email: new_user_notifications.selectedValues.includes("marketing"),
  skip_keycloak: new_user_skip_kc.isSwitchedOn
}
```

### Bindings / hidden logic
- **[B]** `skip_keycloak: new_user_skip_kc.isSwitchedOn` — a **privileged business flag**: when true the backend must not create a Keycloak account. This is a real business rule that must survive the migration and likely requires an operator role gate.
- **[B]** `maintenance_on_primary_email` / `marketing_on_primary_email` come from a checkbox group whose values are hard-coded (`maintenance`, `marketing`). Those string values are the contract with the backend DTO.
- **[O]** `new_user_button.isDisabled` has a dynamic binding but the expression is not surfaced in this export view — may gate on `select_customer.selectedOptionValue` being set. Needs inspection before rewrite.
- **[P]** Table column "nome ruolo" unwraps `role.name` the same way `Stato Aziende` unwraps `group.name`.
- **[P]** `Text2` uses `appsmith.user.name || appsmith.user.email` — greeting the Appsmith-authenticated user, not the customer's user.
- **[B]** No role / permission check visible before submitting `NewAdmin` — the endpoint must enforce.

### Candidate domain entities
- **Customer** (reused from page 2).
- **User / Admin** (`id`, `customer_id`, `created`, `email`, `first_name`, `last_name`, `enabled`, `phone`, `role:{id,name}`, notification flags) from `/users/v2/user`.
- **Admin creation request** — POST shape above; note two notification booleans + `skip_keycloak`.

### Open questions / ambiguities
- When `select_customer` is empty at first load, does `GetAllUsersByCustomer` hit the server with no filter or fail validation? — verify.
- What are the possible `role` values? Only "admin" since this form creates admins only?
- Is `enabled` editable elsewhere? In this app it is display-only.
- Is there a way to edit / disable / delete a user? **Not in this app.** Only create.

### Migration notes
- Unify customer listing (duplicate of page 2's `GetAllCustomer`) into a shared hook/query in the rewrite.
- Move the `skip_keycloak` toggle behind an explicit confirmation + role check — it bypasses SSO provisioning.
- Disable the "Nuovo Admin" button when no customer is selected (if not already enforced via the unseen binding).
- Add user edit/deactivate capability only if the product says so — out of scope of current Appsmith app.

---

## 4. Accessi Biometrico

**Purpose**: review biometric-access requests (`customers.biometric_request`), flip "richiesta completata / in attesa" and persist via a Postgres stored function.

### Widgets (tree)
- `MainContainer`
  - `BiometricData` — `TABLE_WIDGET_V2`, `tableData = {{GetTableData.data}}` (note: `.data`, not `.data.items` — this query is raw SQL, not REST)

### Table columns (`BiometricData.primaryColumns`)
| alias | label | type | visible | editable | notes |
|-------|-------|------|---------|----------|-------|
| `id` | id | number | **no** | no | hidden PK |
| `nome` | nome | text | yes | no | |
| `cognome` | cognome | text | yes | no | |
| `email` | email | text | yes | no | |
| `azienda` | azienda | text | yes | no | |
| `tipo_richiesta` | tipo_richiesta | text | yes | no | |
| `stato_richiesta` | stato_richiesta | checkbox | yes | **yes** | **[B]** see hidden logic |
| `is_biometric_lenel` | In Lenel | checkbox | **no** | no | |
| `data_approvazione` | data conferma | text | yes | no | `new Date(...).toLocaleString()` client-side format |
| `data_richiesta` | data della richiesta | text | yes | no | `new Date(...).toLocaleString()` client-side format |
| `EditActions1` | Save / Discard | editActions | yes | — | contains an `onSave` handler (see below) |

`dynamicTriggerPathList` on the table confirms only these two triggers are actually wired:
- `primaryColumns.stato_richiesta.onCheckChange`
- `primaryColumns.EditActions1.onSave`

### Page-load actions
`[GetTableData]` (a single SQL query, result placed directly on `BiometricData.tableData`).

### Event flow (as actually wired in the DSL)
1. Page load → `GetTableData` runs, populates the table.
2. User toggles `stato_richiesta` on a row → `onCheckChange` fires:
   ```
   {{ storeValue('flag', BiometricData.updatedRow.stato_richiesta)
        .then(() => {})
        .catch(() => showAlert(Error(UpdateRequestCompleted.responseMeta.statusCode), 'error')); }}
   ```
3. User clicks Save on that row → `EditActions1.onSave` fires:
   ```
   {{ UpdateRequestCompleted.run().then(() => {
        GetTableData.run().then(() => { howAlert('success'); });
        showAlert('Perfetto, stato biometrico cambiato', 'success');
      }).catch(() => showAlert('Qualcosa e\' andato storto', 'error')); }}
   ```
4. `UpdateRequestCompleted` runs `SELECT customers.biometric_request_set_completed({{ BiometricData.updatedRow.id }}, {{ BiometricData.updatedRow.stato_richiesta }});`

### Hidden logic (critical)
- **[B] Stored-function boundary.** The UI calls `customers.biometric_request_set_completed(id, stato_richiesta)`. This is the **only** mutation path and likely contains the real workflow (e.g., also pokes Lenel or audit logs). The rewrite's backend must call the same function or replace the function with equivalent domain logic (high-risk item).
- **[B] Type/value mismatch.** The SQL projection is `br.request_completed AS stato_richiesta`, which is a PostgreSQL boolean. The column is declared as `checkbox` (so the UI reads it as boolean). **But** JSObject1's logic treats it as a string `"ok" / "pending"`, and `UpdateRequestCompleted` forwards whatever value is in `BiometricData.updatedRow.stato_richiesta` to the Postgres stored function. The stored function's signature determines whether this is a latent bug. In practice the wired path (EditActions1.onSave) sends a boolean — which only works because JSObject1 is *not* currently wired.
- **[O] Parallel dead implementation.** JSObject1 (`onToggle`, `onSave`, `onDiscard`, `onToggleCompletion`, `onSaveCompletion`) plus the duplicated JS actions under the page all target a **client-side staging buffer** in `appsmith.store.tableData` + `appsmith.store.updatedRowIndices`. No widget binding currently calls these — they are dead code carried in the export. A prior designer clearly intended a batch-edit workflow that was replaced by per-row EditActions but never deleted.
- **[O] Typo `howAlert('success')`** inside the onSave handler — the success-toast-after-reload is silently broken; only the other `showAlert('Perfetto, stato biometrico cambiato', 'success')` fires. Any rewrite should pick one.
- **[O] `onCheckChange` stores a flag but never mutates the row.** `storeValue('flag', …)` writes to `appsmith.store.flag`, nothing reads it. The check also references `UpdateRequestCompleted.responseMeta.statusCode` **in the catch of a storeValue** — wrong target; this is defensive noise.
- **[B] "In Lenel" is computed but hidden.** `is_biometric_lenel = COALESCE(ued.is_biometric, false)` — a real business signal (is this user already enrolled in Lenel?) that today is invisible to the operator. Exposing it in the rewrite is a likely product decision.
- **[P] Italian weak labels.** Columns keep raw DB aliases (`nome`, `cognome`, `tipo_richiesta`, `stato_richiesta`) — no presentational labels.

### Candidate domain entities
- **BiometricRequest** (`id`, `request_type`, `request_completed`, `request_date`, `request_approval_date`, `user_struct_id`, `customer_id`).
- **UserStruct** (internal user — joined for first/last name + primary email).
- **Customer** (joined for `name` → `azienda`).
- **UserEntranceDetail** (`email`, `is_biometric`) — source of Lenel-enrollment signal.
- **Stored procedure** `customers.biometric_request_set_completed(bigint, ?)` — the mutation contract.

### Open questions / ambiguities
- What is the second argument type of `customers.biometric_request_set_completed`? Boolean (matches SQL projection) or text (matches JSObject1 intent)? Requires a look at `docs/mistradb/` or an ERD lookup before reimplementing.
- Is `customColumn is_biometric_lenel` *supposed* to be visible? It's computed and hidden — likely an in-progress feature.
- Should the list filter out already-completed requests by default?
- Any audit of who approved a request? `data_approvazione` is shown but not explained.

### Migration notes
- Mutation must move to a real backend endpoint (e.g., `POST /biometric-requests/{id}/complete`), wrapping or replacing the stored function. Do not expose direct SQL to the new frontend.
- Normalize the state model to a clear boolean (completed/not) or a string enum — pick one and enforce it in the DTO.
- Delete the `appsmith.store` staging flow on migration; the wired flow is single-row, per-click.
- Preserve the stored-function call semantics until we have verified what side effects (Lenel sync, notifications, audit) live inside it.
- Expose "In Lenel" and request dates with real formatting; add a filter for `stato_richiesta`.

---

## 5. Area documentale

**Purpose (inferred from labels)**: manage document categories and per-category documents visible to the end-user Customer Portal. **Not implemented.**

### Widgets (tree)
- `MainContainer`
  - `Container1`
    - `Text1` — "Categories"
    - `IconButton1` — opens `categoryModal`
    - `Table1` — empty table (no `tableData` binding)
    - `categoryDetailsContainer`
      - `categoryNameLabel` — "Category Details"
      - `selectedCategoryName` — "(Select a category)"
      - `descriptionLabel` — "Descrizione"
      - `categoryDescription` — "Descrizione categoria"
      - `areaSelect` (SELECT) — `labelText = Tipologia di utenti`, `sourceData` is a **hard-coded literal** `[{name:'Blue',code:'BLUE'},{name:'Green',code:'GREEN'},{name:'Red',code:'RED'}]`, `optionLabel=name`, `optionValue=code`
      - `visibleToggle` (CHECKBOX) — "Visibile sul customer portal"
      - `Divider1`
      - `filesHeader` — "Documenti nella categoria"
      - `uploadFileBtn` (ICON_BUTTON) — opens `uploadModal`
      - `filesTable` — empty table
  - `categoryModal` (MODAL) — "Aggiungi Categoria"
    - `Input1` (Nome), `RichTextEditor1` (Descrizione, default `"This is the initial <b>content</b> of the editor"`), `Select1` (Area) — same hard-coded Blue/Green/Red list, `visibleToggleCopy`, `Button2` (Confirm — **no onClick**), `Button1` (Close), `IconButton2` (close)
  - `uploadModal` (MODAL) — "Carica documento"
    - `Input2` (Nome documento), `FilePicker1` (Selezione documento), `Button4` (Carica — **no onClick**), `Button3` (Chiudi), `IconButton3` (close)

### Queries / actions
- `Query1` exists (`SELECT * FROM accounting."acct_log" LIMIT 10`) — it is **not bound** to any widget on this page; it's a scratchpad query left over from an earlier investigation.

### Page-load actions
**None** (no widget has real data; `Table1`, `filesTable` are empty).

### Hidden logic
- **[O]** Two modals (`categoryModal`, `uploadModal`) are fully scaffolded but the Confirm / Carica buttons have no handlers.
- **[P]** Blue/Green/Red values are placeholder data — not a domain taxonomy. Must be replaced with the real "tipologia di utenti" enum.
- **[P]** The rich-text default value is literally `"This is the initial <b>content</b> of the editor"`.

### Candidate domain entities
(Inferred from UI labels — **not validated against any API or schema**.)
- **Category** (name, description, area/tipologia utenti, visible_on_portal)
- **Document** (name, file, category_id)
- **Area / Tipologia utenti** lookup.

### Open questions / ambiguities
- Does a backend model already exist for categories/documents? The canonical Mistra API spec (`docs/mistra-dist.yaml`) and schema docs (`docs/mistradb/MISTRA.md`) must be consulted before building this out. Nothing in this export tells us.
- Who are the "Blue / Green / Red" user types actually mapped to? Likely maps to customer segments or Keycloak groups; unconfirmed.
- Should "visible on customer portal" be per-category, per-document, or both? UI has toggles at both levels (`visibleToggle`, `visibleToggleCopy`), with no backend.

### Migration notes
- Treat this page as **greenfield** in the rewrite. The Appsmith version is a wireframe; reuse its structure as UX input only.
- Do not port `Query1` — it's unrelated (accounting log).
- Drive the "tipologia utenti" select from a real backend lookup, not a literal.

# Page audits — RDF Backend StraFatti

## Page: `Home`

- **Purpose:** Default landing page. Contains no widgets and no on-load actions. Effectively a blank canvas — likely a placeholder awaiting content.
- **Widgets:** none (only `MainContainer` shell)
- **Queries / actions:** none
- **Event flow:** none
- **Hidden logic:** none
- **Candidate domain entities:** none
- **Migration notes:**
  - Confirm with stakeholders whether `Home` should ship as an empty page, a landing/overview, or be removed in the mrsmith rewrite.
  - No behavior to reproduce.
- **Open questions:**
  - Is `Home` intentionally empty because navigation lands straight into `Fornitori`?
  - Should there be a dashboard or index across multiple RDF entities (suppliers likely being only the first)?

---

## Page: `Fornitori`

### Purpose
Single-table CRUD admin screen for the "Fornitori" (suppliers) entity stored in `public.rdf_fornitori (id, nome)`. List + free-text search + sort + server-paginated table, plus create, edit, delete flows.

### Widgets (13)

| Widget name | Type | Role |
|---|---|---|
| `Container1` | `CONTAINER_WIDGET` | Layout wrapper (white bg) |
| `Text16` | `TEXT_WIDGET` | Page title "Fornitori" |
| `refresh_btn` | `ICON_BUTTON_WIDGET` | Manually re-run `SelectQuery` |
| `add_btn` | `ICON_BUTTON_WIDGET` | Open `Insert_Modal` |
| `data_table` | `TABLE_WIDGET_V2` | Main list; server-side pagination, sort, search; button column triggers delete modal; driver of `update_form` via `selectedRow` |
| `Delete_Modal` | `MODAL_WIDGET` | Delete confirmation dialog |
| `Alert_text` | `TEXT_WIDGET` | "Delete Row" header inside `Delete_Modal` |
| `Text12` | `TEXT_WIDGET` | "Are you sure you want to delete this item?" |
| `Button1` | `BUTTON_WIDGET` | Cancel — closes `Delete_Modal` |
| `Delete_Button` | `BUTTON_WIDGET` | Confirm — runs `DeleteQuery` → `SelectQuery` → close modal |
| `Insert_Modal` | `MODAL_WIDGET` | Insert dialog container |
| `insert_form` | `JSON_FORM_WIDGET` | Create form; schema **not auto-generated**, manually defined (only `nome` visible; `id` hidden) |
| `update_form` | `JSON_FORM_WIDGET` | Edit form; **auto-generated** schema from `data_table.selectedRow` minus `customColumn1`, `__originalIndex__`, `id` |

### Queries consumed
- `SelectQuery` — on load, on `onSearchTextChanged`, `onSort`, `onPageChange`, on `refresh_btn` click, and as the success callback of Insert/Update/Delete.
- `InsertQuery` — on `insert_form.onSubmit`.
- `UpdateQuery` — on `update_form.onSubmit`.
- `DeleteQuery` — on `Delete_Button.onClick`.

### Event flow

1. **Page load**
   - `layoutOnLoadActions: [[SelectQuery]]` runs before widgets mount → populates `data_table`.

2. **List / search / sort / paginate**
   - `data_table.searchText` change → `SelectQuery.run()`
   - `data_table.sortOrder` change → `SelectQuery.run()`
   - `data_table.pageNo` change → `SelectQuery.run()`
   - All three re-issue the same SQL, with search/sort/pagination interpolated into the query body. `serverSidePaginationEnabled: true`.

3. **Create**
   - User clicks `add_btn` → `showModal('Insert_Modal')`.
   - User fills `insert_form` and submits.
   - `InsertQuery.run(success → SelectQuery.run().then(closeModal('Insert_Modal')), error → showAlert)`.

4. **Update**
   - User selects a row in `data_table`; `update_form.isVisible` becomes true via `{{!!data_table.selectedRow.id}}`.
   - `update_form.sourceData` = `_.omit(data_table.selectedRow, "customColumn1", "__originalIndex__", "id")` (schema auto-generated from remaining fields).
   - On submit: `UpdateQuery.run(success → SelectQuery.run(), error → showAlert)`.
   - There is **no modal** for update; it's inline.

5. **Delete**
   - Table has a custom button column `customColumn1` labeled "Delete".
   - `onClick: showModal('Delete_Modal')` — this fires *per-row click* and sets `data_table.triggeredRow`.
   - User confirms → `DeleteQuery.run(() => SelectQuery.run(() => closeModal('Delete_Modal')), () => {})`.
   - The `DeleteQuery` targets `data_table.triggeredRow.id` (the row whose button was clicked), *not* `selectedRow.id` — this is the idiomatic Appsmith distinction.

### Bindings and hidden logic

| Location | Binding | Classification | Notes |
|---|---|---|---|
| `SelectQuery.body` | `WHERE "nome" ilike '%{{data_table.searchText || ""}}%'` | business logic (search semantics: case-insensitive, substring match on `nome` only) | Must be preserved in rewrite. |
| `SelectQuery.body` | `ORDER BY "{{data_table.sortOrder.column || 'id'}}" {{data_table.sortOrder.order || 'ASC'}}` | business logic (default sort = id ASC; any column user sorts by is trusted verbatim) | **SQL-injection hazard.** Rewrite must whitelist sortable columns. |
| `SelectQuery.body` | `LIMIT {{data_table.pageSize}} OFFSET {{(data_table.pageNo - 1) * data_table.pageSize}}` | orchestration (server-side pagination) | Standard offset/limit pagination. |
| `data_table.totalRecordsCount` | `0` (hard-coded) | **bug / missing piece** | `serverSidePaginationEnabled: true` requires the backend to return a total count, but the widget config leaves `totalRecordsCount` at 0. Pagination UI will not show accurate page count. |
| `UpdateQuery.body` | `"nome" = '{{update_form.fieldState.nome.isVisible ? update_form.formData.nome : update_form.sourceData.nome}}'` | business logic (if the `nome` field is hidden, preserve the existing value) | Idiom for conditional field updates in Appsmith JSON Form. Rewrite: PATCH with only dirty fields. |
| `UpdateQuery.body` | `WHERE "id" = {{data_table.selectedRow.id}}` | business rule (edit target = currently selected row) | SQL-injection hazard. |
| `DeleteQuery.body` | `WHERE "id" = {{data_table.triggeredRow.id}}` | business rule (delete target = row whose button was clicked, not selectedRow) | Same hazard. |
| `InsertQuery.body` | `VALUES ('{{insert_form.formData.nome}}')` | business logic (only `nome` is inserted; `id` is DB-assigned) | SQL-injection hazard. `id` in the form schema is `isVisible: false`. |
| `insert_form.sourceData` | `{{_.omit(data_table.tableData[0], "customColumn1", "__primaryKey__")}}` | orchestration (seed defaults from the first row's shape) | Surprising: insert form defaults take values from the first row of the current page. Likely a leftover from Appsmith's "generate CRUD" scaffold. Pre-fill behavior may or may not be intended. |
| `insert_form.schema.id.isVisible` | `false` | presentation | Hides auto-generated `id` input. |
| `insert_form.schema.nome.isRequired` | `true` | business rule | `nome` is mandatory on insert. |
| `insert_form.autoGenerateForm` | `false` | — | Schema was manually defined (snapshotted). Any DB column addition will **not** appear until someone regenerates. |
| `update_form.isVisible` | `{{!!data_table.selectedRow.id}}` | orchestration | Update form appears only when a row is selected. |
| `update_form.sourceData` | `{{_.omit(data_table.selectedRow, "customColumn1", "__originalIndex__", "id")}}` | orchestration | `id` is deliberately excluded from the editable set (PK is immutable). |
| `update_form.autoGenerateForm` | `true` | — | Schema follows `sourceData` at runtime. Robust to new columns, but makes validation rules column-blind. |
| `data_table.primaryColumns.customColumn1.onClick` | `{{showModal('Delete_Modal')}}` | orchestration | Row-level delete trigger; relies on Appsmith's `triggeredRow` semantic. |
| `Delete_Button.onClick` | `DeleteQuery.run(() => SelectQuery.run(() => closeModal('Delete_Modal')), () => {})` | orchestration | Chained: DELETE → re-SELECT → close modal. Error branch is silent (`() => {}`). |
| `insert_form.onSubmit` error branch | `showAlert('Error while inserting resource!\n ${error}', 'error')` | presentation + orchestration | User-visible error feedback. |
| `update_form.onSubmit` error branch | `showAlert('Error while updating resource!\n ${error}', 'error')` | presentation + orchestration | Same pattern. |
| `DeleteQuery` error branch | `() => {}` | **gap** | Silent on delete failure — user gets no feedback. Rewrite should surface an error toast. |

### Candidate domain entities
- **Fornitore (Supplier):** `{ id: int (PK, DB-generated), nome: string (required, non-empty) }` in table `public.rdf_fornitori`.
- No FK relationships visible in this export.

### Migration notes
- The app is a textbook Appsmith "generate CRUD page from a table" scaffold — one entity, four queries, two modals, two forms. Rewrite complexity is low; the effort will come from (a) designing the proper backend API, (b) integrating it into the mrsmith portal shell (Keycloak auth, role gating, navigation taxonomy), and (c) designing a polished UI consistent with `docs/UI-UX.md`.
- Required backend endpoints (tentative):
  - `GET  /api/rdf/fornitori?search=&sort=&order=&page=&pageSize=` → `{ items, totalCount }` (fixes the `totalRecordsCount: 0` gap).
  - `POST /api/rdf/fornitori` `{ nome }`.
  - `PATCH /api/rdf/fornitori/:id` `{ nome? }` (preserves the "skip hidden/unchanged field" semantic as dirty-only patch).
  - `DELETE /api/rdf/fornitori/:id`.
- Whitelist sort columns (`id`, `nome`) at the backend. Do not accept arbitrary column names.
- Surface delete errors (current Appsmith flow swallows them).
- Clarify whether `insert_form` should really pre-seed from the first row or start blank — the scaffolded default is almost certainly not desired behavior.

### Open questions / ambiguities
- What does `StraFatti` mean in the app title? Is `rdf_fornitori` scoped to a specific context (e.g., RDF domain, StraFatti feature) or general supplier list?
- Does `Home` need real content, or should it be replaced by the mrsmith portal landing?
- Are there other RDF tables (e.g., `rdf_clienti`, `rdf_contratti`) that this app was intended to manage but that never got built?
- Where should access control live? (Suggested: `app_rdf_access` Keycloak role per AGENTS.md naming convention.)

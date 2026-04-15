# Datasource & query catalog — RDF Backend StraFatti

## Datasource: `anisetta`

| Field | Value |
|---|---|
| Plugin | `postgres-plugin` |
| Host | `10.129.32.20` |
| Port | `5432` |
| Mode | READ_WRITE |
| SSL | `DEFAULT` |
| Database name | not present in export (stripped) |
| Credentials | not present in export (stripped) |
| Known tables used | `public.rdf_fornitori (id int, nome text)` |

**Rewrite recommendation:** route all access through the Go backend. The frontend must not connect to `anisetta` directly. A dedicated `backend/internal/rdf/` package (or `rdf_fornitori` sub-module) should own the repository and handlers, wired from `backend/cmd/server/main.go` with an env-configured DSN (`AnisettaDSN` or similar, per `backend/internal/platform/config/config.go`). Apply the mini-app conventions from the "New App Checklist" in `CLAUDE.md`.

---

## Queries

All four queries belong to the `Fornitori` page and target `public."rdf_fornitori"` on `anisetta`.

### `SelectQuery`

```sql
SELECT * FROM public."rdf_fornitori"
WHERE "nome" ilike '%{{data_table.searchText || ""}}%'
ORDER BY "{{data_table.sortOrder.column || 'id'}}" {{data_table.sortOrder.order || 'ASC'}}
LIMIT {{data_table.pageSize}}
OFFSET {{(data_table.pageNo - 1) * data_table.pageSize}};
```

| Aspect | Value |
|---|---|
| Purpose | List suppliers with search/sort/pagination for `data_table`. |
| Read/Write | Read |
| executeOnLoad | **true** (via `layoutOnLoadActions`) |
| Inputs | `data_table.searchText`, `data_table.sortOrder.{column,order}`, `data_table.pageSize`, `data_table.pageNo` |
| Outputs | Rows `[{ id, nome }]` bound to `data_table.tableData` |
| Dependencies | None on other queries. Re-triggered by `refresh_btn`, `data_table.onSearchTextChanged`, `onSort`, `onPageChange`, and as success callback of Insert/Update/Delete. |
| Risks | SQL injection via `sortOrder.column`, `sortOrder.order`, `searchText` (string interpolation in body). Missing total-count return makes `totalRecordsCount` stuck at 0. |
| Rewrite target | `GET /api/rdf/fornitori?search&sort&order&page&pageSize` returning `{ items, total }`. **Business logic** (filter/sort/pagination contract) moves to backend. |

### `InsertQuery`

```sql
INSERT INTO public."rdf_fornitori" ("nome")
VALUES ('{{insert_form.formData.nome}}');
```

| Aspect | Value |
|---|---|
| Purpose | Insert a new supplier. |
| Read/Write | Write |
| executeOnLoad | false |
| Inputs | `insert_form.formData.nome` |
| Outputs | RowsAffected (not consumed) |
| Dependencies | Triggered by `insert_form.onSubmit`; on success chains `SelectQuery` then closes `Insert_Modal`; on error shows alert. |
| Risks | SQL injection on `nome`. `id` is DB-generated (correctly hidden in form). |
| Rewrite target | `POST /api/rdf/fornitori` with `{ nome: string }`. **Business logic** in backend. |

### `UpdateQuery`

```sql
UPDATE public."rdf_fornitori" SET
    "nome" = '{{update_form.fieldState.nome.isVisible ? update_form.formData.nome : update_form.sourceData.nome}}'
WHERE "id" = {{data_table.selectedRow.id}};
```

| Aspect | Value |
|---|---|
| Purpose | Update the selected supplier. |
| Read/Write | Write |
| executeOnLoad | false |
| Inputs | `update_form.fieldState.nome.isVisible`, `update_form.formData.nome`, `update_form.sourceData.nome`, `data_table.selectedRow.id` |
| Outputs | RowsAffected (not consumed) |
| Dependencies | Triggered by `update_form.onSubmit`; on success re-runs `SelectQuery`; on error shows alert. Depends on `data_table` having a selected row. |
| Risks | SQL injection on `nome`. Integer-literal injection on `id` (no type check). Implicit coupling: edit target is driven by table selection, not form identity. |
| Rewrite target | `PATCH /api/rdf/fornitori/:id` with only dirty fields. Preserve the "skip unchanged field" semantic by sending only fields the user changed (equivalent to the `isVisible ? formData : sourceData` trick). |

### `DeleteQuery`

```sql
DELETE FROM public."rdf_fornitori"
  WHERE "id" = {{data_table.triggeredRow.id}};
```

| Aspect | Value |
|---|---|
| Purpose | Delete the supplier whose row-level "Delete" button was clicked. |
| Read/Write | Write |
| executeOnLoad | false |
| Inputs | `data_table.triggeredRow.id` (Appsmith-specific: the row whose custom button fired the event, not `selectedRow`) |
| Outputs | RowsAffected (not consumed) |
| Dependencies | Triggered from `Delete_Button.onClick` inside `Delete_Modal`; on success re-runs `SelectQuery` and closes the modal; error callback is silent (`() => {}`). |
| Risks | SQL injection on `id`. **Silent failure** — user is not notified if delete errors. |
| Rewrite target | `DELETE /api/rdf/fornitori/:id`. Surface errors via toast. |

---

## Where each query belongs in the rewrite

| Query | Target layer | Notes |
|---|---|---|
| `SelectQuery` | Backend (Go handler + repository) | Owns filter/sort/pagination contract; returns `{ items, total }`. |
| `InsertQuery` | Backend | Validate `nome` (non-empty, length limit, trim). |
| `UpdateQuery` | Backend | Accept PATCH semantics; preserve "unchanged field is not overwritten" behavior. |
| `DeleteQuery` | Backend | Use `triggeredRow.id` concept in UI (click handler carries the id) but the backend just takes an `:id` path param. |

None of these queries belong in the frontend in a mrsmith-style rewrite: all of them embed business and security-critical logic (pagination, access to raw SQL on a shared DB) that must live behind authenticated Go endpoints with Keycloak role enforcement.

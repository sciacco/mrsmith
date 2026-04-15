# Application inventory — RDF Backend StraFatti

## Metadata
- **Application name:** RDF Backend StraFatti
- **Source type:** Single Appsmith application export (JSON), `artifactJsonType: APPLICATION`, `clientSchemaVersion 2.0`, `serverSchemaVersion 11.0`
- **Source file:** `apps/rdf-backend/RDF Backend StraFatti.json`
- **App slug:** `rdf-backend-strafatti`
- **Navigation:** sidebar, static, light, logo + app title
- **Theme:** default Appsmith theme (no custom JS libraries, `customJSLibList: []`)

## Pages
| Page | Default | Purpose (inferred) | Widgets | On-load queries |
|---|---|---|---|---|
| `Home` | yes | Empty placeholder / landing page. DSL contains only the MainContainer; no widgets, no actions. | 0 | none |
| `Fornitori` | no | CRUD management of suppliers (`rdf_fornitori` table): list, search, sort, paginate, insert, edit (implicit via `update_form`), delete. | 13 | `SelectQuery` |

## Datasources
| Name | Plugin | Host / Port | Auth | Notes |
|---|---|---|---|---|
| `anisetta` | `postgres-plugin` | `10.129.32.20:5432` | not present in export (credentials stripped) | READ_WRITE mode, `ssl.authType: DEFAULT`. Single DB backing the entire app. |

## Queries / Actions
All four actions are raw SQL against the `anisetta` datasource and belong to the `Fornitori` page. Table referenced: `public."rdf_fornitori"` with columns `id` (number, PK) and `nome` (text).

| Name | Type | executeOnLoad | Purpose |
|---|---|---|---|
| `SelectQuery` | SQL SELECT | **true** | Server-side paginated + search + sort listing feeding `data_table` |
| `InsertQuery` | SQL INSERT | false | Insert new supplier from `insert_form` submission |
| `UpdateQuery` | SQL UPDATE | false | Update selected supplier (`data_table.selectedRow.id`) from `update_form` |
| `DeleteQuery` | SQL DELETE | false | Delete the row whose Delete button was pressed (`data_table.triggeredRow.id`) |

## JSObjects / Custom JS
- `actionCollectionList`: empty — **no JSObjects**
- `customJSLibList`: empty — **no custom JS libraries**
- All logic is inline in widget bindings and SQL query bodies.

## Navigation patterns
- Sidebar navigation (Appsmith default) between `Home` and `Fornitori`.
- No programmatic `navigateTo` calls found.
- Modals (`Insert_Modal`, `Delete_Modal`) used in-page for create/delete confirmation; update is not modal but an inline `update_form` beside the table.

## Cross-page reuse
- None. All queries and widgets live on `Fornitori`. `Home` is empty.

## Global notes / migration risks
- **SQL injection:** every query interpolates widget state directly into SQL bodies via `{{ ... }}` (`data_table.searchText`, `data_table.sortOrder.column`, `update_form.formData.nome`, etc.). A rewrite must use parameterized queries / a proper backend DTO layer.
- **Direct DB access from UI:** frontend connects directly to a Postgres server (`anisetta`, 10.129.32.20). In the mrsmith architecture this must move to Go backend endpoints.
- **Minimal domain model:** the only entity is `rdf_fornitori { id, nome }`. The app is a scaffolded CRUD — likely the starting skeleton of a larger "RDF Backend" feature that has not been built out.
- **Empty Home page:** suggests the export is an early/work-in-progress app; migration should clarify intended scope with stakeholders before assuming parity.
- **Credential stripping:** datasource has no `authentication` block — normal for Appsmith exports; the production DB credentials must be recovered from the live Appsmith instance.

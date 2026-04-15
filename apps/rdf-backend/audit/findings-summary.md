# Findings summary — RDF Backend StraFatti

## TL;DR
Trivial Appsmith CRUD scaffold over a single Postgres table `public.rdf_fornitori (id, nome)`, sitting on a shared DB (`anisetta`). Two pages: `Home` (empty) and `Fornitori` (list + search + sort + paginate + create + edit + delete). No JSObjects, no custom JS, no cross-page logic. Rewrite effort is driven almost entirely by platform integration (backend endpoints, Keycloak access role, portal wiring, UI polish), not by domain complexity.

## Embedded business rules
1. **Suppliers have exactly two fields:** `id` (PK, DB-assigned) and `nome` (required, free text). No other validation is encoded in the export beyond `isRequired: true` on `nome`.
2. **Search is substring ILIKE on `nome`.** Case-insensitive, no wildcards exposed to the user, no multi-field search.
3. **Default sort is `id ASC`.** Any `data_table.sortOrder.column` the user clicks is trusted and injected verbatim into `ORDER BY`.
4. **Update preserves unchanged fields** via the `fieldState.nome.isVisible ? formData.nome : sourceData.nome` idiom — equivalent to PATCH-dirty-only semantics.
5. **Delete targets the clicked row**, not the currently selected row (Appsmith `triggeredRow` vs `selectedRow` distinction).
6. **Inline edit pattern:** the update form is visible only while a table row is selected (`{{!!data_table.selectedRow.id}}`), not in a modal.

## Duplication
- Minimal. `SelectQuery.run()` is called from five places (page load, refresh button, search/sort/page changes, plus as a success callback of all three mutations). Not real duplication — it's the intended "after every write, re-read" pattern. In the rewrite, a single React Query / SWR invalidation replaces all of these.

## Security concerns
1. **SQL injection in all four queries.** User-controlled state (`searchText`, `sortOrder.column`, `sortOrder.order`, `formData.nome`, `selectedRow.id`, `triggeredRow.id`) is string-interpolated into SQL bodies. Must be replaced with parameterized queries and column whitelists in the Go backend.
2. **Frontend-to-DB direct connection.** The Appsmith app holds the Postgres credentials and talks to `10.129.32.20:5432` from the client side of Appsmith. In the mrsmith architecture this is unacceptable — all DB access must flow through the Go backend with Keycloak-authenticated endpoints.
3. **No access control visible in the export.** Anyone who can reach the Appsmith app can CRUD suppliers. Rewrite must gate behind an `app_rdf_access` (or similar) Keycloak role per the naming convention in `CLAUDE.md`.
4. **Credentials not exported** (normal for Appsmith) — production DSN must be obtained separately from the running Appsmith instance or the DB team.

## Fragile bindings / likely bugs
1. **`data_table.totalRecordsCount` hard-coded to `0`** while `serverSidePaginationEnabled: true`. The page-count UI is therefore wrong. Rewrite must return a real total from the backend.
2. **`insert_form.sourceData` = `_.omit(data_table.tableData[0], ...)`** — the new-row form defaults are seeded from the *first row of the current page*. Almost certainly an unintended leftover from Appsmith's CRUD scaffold; insert form should start blank.
3. **`DeleteQuery` error callback is `() => {}`** — failures are silently swallowed. Rewrite must show a toast on failure.
4. **`insert_form.autoGenerateForm: false`** — the schema is a manual snapshot. Any future column added to `rdf_fornitori` will not appear in the insert form until someone regenerates it. `update_form.autoGenerateForm: true` does not have this issue.
5. **Sort column not whitelisted** — `ORDER BY "{{data_table.sortOrder.column}}"` will happily accept any string, including injection payloads.

## Candidate domain entities
- **Fornitore**: `{ id: int PK, nome: string (required) }` in `public.rdf_fornitori`.
- No other entities are present in this export. The app name "RDF Backend StraFatti" hints at a broader RDF domain (possibly other tables prefixed `rdf_*`); those are out of scope of this audit and should be clarified with stakeholders.

## Migration blockers / things to confirm before coding
1. **Scope of the mini-app.** Is this really just supplier CRUD, or is `rdf_fornitori` one of several `rdf_*` entities the mrsmith version should manage? Check the `anisetta` DB for sibling tables.
2. **Meaning of "StraFatti"** in the app name. Is it a feature name, a legacy project codename, or dead text? Affects naming of the mrsmith mini-app.
3. **Access control.** Which Keycloak role(s) should gate this app? Propose `app_rdf_access` per convention.
4. **`anisetta` connectivity from the Go backend.** Is the host `10.129.32.20:5432` reachable from the backend's network? DSN, credentials, TLS policy need to be decided and placed in `backend/internal/platform/config/config.go`.
5. **Expected UX for the insert form defaults.** Blank is the safe rewrite; confirm nobody is relying on the "copy first row" behavior.
6. **Home page content.** Should `Home` be dropped (portal provides the landing) or should the mini-app have its own dashboard?

## Recommended next steps
1. Hand these audit artifacts to the `appsmith-migration-spec` skill to produce a migration PRD covering: backend API design (4 endpoints), data validation, Keycloak role, URL/slug, portal catalog entry, UI screens, and any follow-up questions for stakeholders.
2. Before spec, query the `anisetta` DB for sibling `rdf_*` tables to confirm whether the mini-app should be broader than `fornitori`.
3. Follow the **New App Checklist** in `CLAUDE.md` when scaffolding the mrsmith mini-app (root `package.json`, `Makefile`, `applaunch/catalog.go`, `cmd/server/main.go`, `platform/config/config.go`).
4. Apply the reusable discoveries in `docs/IMPLEMENTATION-KNOWLEDGE.md` and the planning checklist in `docs/IMPLEMENTATION-PLANNING.md` during spec.

## Classification cheat-sheet

| Finding | Bucket |
|---|---|
| `SelectQuery` filter/sort/paginate contract | Business logic → backend |
| `UpdateQuery` dirty-field preservation | Business logic → backend (PATCH) |
| "After every mutation, re-select" chain | Orchestration → frontend data-fetching library |
| "Delete button opens modal; confirm runs DeleteQuery" | Orchestration → frontend |
| `update_form.isVisible = !!selectedRow.id` | Orchestration → frontend |
| Search is ILIKE substring on `nome` only | Business rule → backend |
| Default sort = `id ASC` | Business rule → backend |
| `nome` required, `id` hidden in insert form | Business rule / presentation |
| Column labels ("Delete", "Nome", page title "Fornitori") | Presentation |
| Insert form seeded from `tableData[0]` | Likely-unintended orchestration; drop in rewrite |
| `totalRecordsCount: 0` with server-side pagination | Bug |
| Silent delete error callback | Bug |
| Direct-from-UI Postgres connection | Security blocker |

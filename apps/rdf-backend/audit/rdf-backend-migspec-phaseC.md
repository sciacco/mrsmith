# RDF Backend — Phase C: Logic Placement

Policy: **porting 1:1**. Nessun JSObject esiste nell'export; tutta la logica è inline in bindings SQL o widget.

## Backend (Go, mrsmith `backend/internal/rdf/`)
- Parametrizzazione query (no string interpolation).
- Filtro search `ILIKE '%search%'` su `nome`.
- Sort con whitelist `id`, `nome` — order `ASC`/`DESC`.
- Paginazione `LIMIT/OFFSET` + `COUNT(*)` per `totalRecordsCount`.
- PATCH-dirty semantic su update: aggiorna solo i campi presenti nel payload (equivalente al `fieldState.nome.isVisible ? formData : sourceData` di Appsmith).
- Gate con Keycloak role `app_rdf_access` (convenzione `CLAUDE.md`).

## Frontend (React app `apps/rdf-backend/`)
- Orchestrazione: "dopo ogni mutation → re-fetch list" (come Appsmith: `InsertQuery.then(SelectQuery)` etc.).
- Stato UI: `selectedRow` → mostra `update_form`; `triggeredRow` → target del delete modal.
- Toast su errore (insert/update/delete).

## Shared
- Nessuna validazione oltre `nome` non vuoto (come nell'Appsmith).

## Logica non portata
- Pre-seed del form insert da `tableData[0]` (scaffold artefatto; il form parte vuoto).

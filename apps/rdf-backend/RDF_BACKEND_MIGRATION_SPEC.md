# RDF Backend — Migration Specification

## Summary
- **Application name:** RDF Backend (source: "RDF Backend StraFatti")
- **Audit source:** `apps/rdf-backend/audit/{app-inventory,page-audit,datasource-catalog,findings-summary}.md`
- **Phase docs:** `rdf-backend-migspec-phaseA.md` → `…-phaseD.md`
- **Spec status:** approved — **porting 1:1**, nessuna riprogettazione
- **Policy:** replicare il comportamento Appsmith identico, correggendo solo bug oggettivi (paginazione totale, errore delete silenzioso, SQL injection su sort column).

## Current-State Evidence
- **Pagine:** `Home` (vuota), `Fornitori` (CRUD).
- **Entità:** un'unica tabella `public.rdf_fornitori (id int PK, nome text)` su datasource Postgres `anisetta`.
- **Query:** `SelectQuery` (on-load + search/sort/page), `InsertQuery`, `UpdateQuery`, `DeleteQuery` — tutte SQL inline.
- **JSObjects / custom JS:** nessuno.
- **Gap audit:** nessuno rilevante data la policy 1:1.

## Entity Catalog

### Entity: Fornitore
- **Purpose:** anagrafica fornitori.
- **Storage:** `public.rdf_fornitori` su `anisetta`.
- **Operazioni:** List, Create, Update, Delete.
- **Fields:**
  | Field | Type | Constraints |
  |---|---|---|
  | `id` | int PK, DB-assigned | immutabile, non editabile |
  | `nome` | string | required (non vuoto) |
- **Relazioni:** nessuna.
- **Open questions:** nessuna (1:1).

## View Specifications

### View: Home
- **User intent:** placeholder.
- **Pattern:** pagina vuota.
- **Azioni:** nessuna.
- **Note:** mantenere vuota (o omettere se il portal mrsmith già fornisce landing adeguata — decisione frontend).

### View: Fornitori
- **User intent:** gestire la lista fornitori (CRUD).
- **Pattern:** master-table a pagina singola + modal per create, form inline per edit, modal per delete.
- **Dati principali:** tabella con colonne `id`, `nome`, colonna azione "Delete" per riga.
- **Azioni:**
  - Refresh (icon button) → re-fetch.
  - `+` (icon button) → apre **Insert modal** con form (`nome` required) → Submit → POST → re-fetch → chiudi.
  - Click su riga tabella → seleziona → appare **update form inline** (solo `nome`) → Submit → PATCH → re-fetch.
  - Click "Delete" su riga → apre **Delete modal** ("Are you sure you want to delete this item?", Cancel / Confirm) → Confirm → DELETE → re-fetch → chiudi.
  - Search box, sort header, paginazione: server-side.
- **Entry/exit:** unica route della mini-app; navigazione da sidebar portal.
- **Feedback:** toast di errore su insert/update/**delete** (delete era silenzioso in Appsmith — fix minimo).

## Logic Allocation
- **Backend (Go, `backend/internal/rdf/`):**
  - Query parametrizzate.
  - Search `ILIKE '%search%'` su `nome`.
  - Sort con whitelist (`id`, `nome`) + order (`ASC`/`DESC`), default `id ASC`.
  - Paginazione `LIMIT/OFFSET` + `COUNT(*)` per total.
  - Update PATCH-dirty: aggiorna solo i campi presenti nel payload.
  - Auth: Keycloak role `app_rdf_access`.
- **Frontend (`apps/rdf-backend/`, Vite+React):**
  - State tabella: `selectedRow`, `triggeredRow` (per delete), `searchText`, `sort`, `page`, `pageSize`.
  - "Dopo ogni mutation → re-fetch list".
  - Modal create, form inline update, modal delete.
  - Toast errori.
- **Shared:** nessuna validazione condivisa oltre `nome` non vuoto.
- **Rules revisionate (minimi fix):**
  - `totalRecordsCount` reale (era 0 in Appsmith).
  - Toast su errore delete (era `() => {}`).
  - Sort column whitelisted lato backend (era injection).
  - Insert form parte vuoto (drop del pre-seed da `tableData[0]`).

## Integrations and Data Flow
- **External systems:** Postgres `anisetta` (read/write); Keycloak (auth).
- **End-to-end journey:** login portal → sidebar "RDF Backend" → `/fornitori` → CRUD flows come sopra.
- **Background processes:** nessuno.
- **Data ownership:** mini-app è unica proprietaria della tabella `rdf_fornitori` per quanto l'export mostra.

## API Contract Summary

Prefisso: `/api/rdf/fornitori`. Tutte le risposte JSON. Tutte protette da ruolo `app_rdf_access`.

| Method | Path | Query / Body | Response |
|---|---|---|---|
| `GET` | `/api/rdf/fornitori` | `search?`, `sort?` ∈ {`id`,`nome`} (default `id`), `order?` ∈ {`asc`,`desc`} (default `asc`), `page?` (default 1), `pageSize?` (default 20) | `{ items: [{ id, nome }], total: number }` |
| `POST` | `/api/rdf/fornitori` | `{ nome: string }` | `201 { id, nome }` |
| `PATCH` | `/api/rdf/fornitori/:id` | `{ nome?: string }` | `200 { id, nome }` |
| `DELETE` | `/api/rdf/fornitori/:id` | — | `204` |

Errori: 400 su validazione (`nome` vuoto), 404 su id inesistente, 401/403 su auth/ruolo, 500 su errori DB.

## Constraints and Non-Functional Requirements
- **Security:** nessun accesso DB dal frontend; tutte le chiamate via backend Go autenticato Keycloak; query parametrizzate; sort column whitelisted.
- **Role:** `app_rdf_access` (convenzione `app_{name}_access` da `CLAUDE.md`).
- **Performance:** volumi bassi (anagrafica fornitori); paginazione 20/pagina di default; nessun requisito particolare.
- **UX:** allineata a `docs/UI-UX.md` del portal (tema, tipografia, densità), layout funzionale identico all'Appsmith originale.
- **Dev wiring (new-app checklist `CLAUDE.md`):**
  - root `package.json` → aggiungere concurrently target + `dev:rdf-backend`
  - `Makefile` → `dev-rdf-backend` target + `.PHONY`
  - `backend/internal/platform/applaunch/catalog.go` → costanti app id/href/role + catalog entry
  - `backend/cmd/server/main.go` → import, hrefOverrides (dev port), filtro catalog, RegisterRoutes
  - `backend/internal/platform/config/config.go` → `RDFBackendAppURL` + env var, `AnisettaDSN` + env var

## Open Questions and Deferred Decisions
- Nessuna bloccante. L'unica scelta di polish è se mantenere la pagina `Home` vuota o ometterla — decisione frontend senza impatti funzionali.

## Acceptance Notes
- **Audit proved:** entità singola, 4 operazioni, flussi UI esatti, nessun JSObject, nessuna automazione nascosta.
- **Expert confirmed:** porting 1:1, nessuna modifica funzionale, nessun nuovo campo.
- **Still needs validation at impl time:**
  - credenziali + DSN `anisetta` reali (da recuperare dall'istanza Appsmith o DB team — stripped dall'export);
  - verifica che lo schema live di `public.rdf_fornitori` non abbia colonne aggiuntive non usate dalla UI; in caso, mantenerle intatte (SELECT/INSERT/UPDATE esplicite sui campi noti).

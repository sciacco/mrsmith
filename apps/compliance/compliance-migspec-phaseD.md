# Compliance — Migration Spec Phase D: Integration and Data Flow

**Source**: `apps/compliance/AUDIT.md` + Phase A/B/C decisions
**Date**: 2026-04-07
**Status**: Complete

---

## Decisions Log

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| D15 | Keycloak role | `app_compliance_access` — single role, consistent with `app_budget_access` convention | Naming convention documented in project CLAUDE.md |
| D16 | External processes on tables | Yes — DNS sync jobs read/write the same `dns_bl_*` tables | Backend must not break assumptions of these jobs. No schema-breaking changes. |
| D17 | Domain enforcement | Automatic via specialized scripts | The compliance app is the data entry point; enforcement is downstream and out of scope. |
| D17b | Editing UX pattern | **Detail panel form**, not inline table editing | Legal records require intentional edits. Consistent with budget app pattern. Clearer save semantics. |

---

## External Systems

| System | Role | Direction | Notes |
|--------|------|-----------|-------|
| **PostgreSQL** (`anisetta` DB) | Persistence | Read/Write | Backend connects via `database/sql` + `pgx`. Same tables as Appsmith and DNS sync jobs. |
| **Keycloak** | Authentication + Authorization | Read (token validation) | Frontend: `@mrsmith/auth-client`. Backend: `go-oidc` middleware + `acl.RequireRole("app_compliance_access")`. |
| **Appsmith** (coexistence) | Legacy app during transition | Read/Write on same tables | No schema-breaking changes. Both apps coexist. |
| **DNS sync jobs** | Enforce domain blocks on DNS infrastructure | Read from `dns_bl_*` tables | Out of scope for compliance app. The app is the data entry point; sync jobs consume the data independently. |

---

## Data Ownership Boundaries

```
┌──────────────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│  Compliance App      │     │  PostgreSQL           │     │  DNS Sync Jobs   │
│  (new React+Go)      │────►│  dns_bl_* tables      │◄────│  (scripts)       │
│                      │     │                       │     │                  │
│  Appsmith App        │────►│                       │     │                  │
│  (legacy, coexists)  │     │                       │     │                  │
└──────────────────────┘     └──────────────────────┘     └──────────────────┘

Compliance app: OWNS data entry (create, update requests and domains)
DNS sync jobs: CONSUME blocked domain list (read-only from app's perspective)
Appsmith: COEXISTS during transition (same read/write, will be decommissioned)
```

---

## API Contract

### Authentication & Authorization

All endpoints require:
- `Authorization: Bearer {JWT}` header (Keycloak token)
- Role `app_compliance_access` in JWT claims

### Endpoints

#### Block Requests

| Method | Path | Body | Response | Purpose |
|--------|------|------|----------|---------|
| `GET` | `/api/compliance/blocks` | — | `[{id, request_date, reference, method_id, method_description}]` | List all block requests |
| `GET` | `/api/compliance/blocks/:id` | — | `{id, request_date, reference, method_id, method_description}` | Get single block request |
| `POST` | `/api/compliance/blocks` | `{request_date, reference, method_id, domains[]}` | `{id, domains_count}` | Create block request with domains (transactional) |
| `PUT` | `/api/compliance/blocks/:id` | `{request_date, reference, method_id}` | `{id}` | Update block request header |
| `GET` | `/api/compliance/blocks/:id/domains` | — | `[{id, domain}]` | List domains for a block request |
| `POST` | `/api/compliance/blocks/:id/domains` | `{domains[]}` | `{added_count}` | Add domains to existing block (transactional) |
| `PUT` | `/api/compliance/blocks/:id/domains/:domainId` | `{domain}` | `{id, domain}` | Update a single domain name |

#### Release Requests

| Method | Path | Body | Response | Purpose |
|--------|------|------|----------|---------|
| `GET` | `/api/compliance/releases` | — | `[{id, request_date, reference}]` | List all release requests |
| `GET` | `/api/compliance/releases/:id` | — | `{id, request_date, reference}` | Get single release request |
| `POST` | `/api/compliance/releases` | `{request_date, reference, domains[]}` | `{id, domains_count}` | Create release request with domains (transactional) |
| `PUT` | `/api/compliance/releases/:id` | `{request_date, reference}` | `{id}` | Update release request header |
| `GET` | `/api/compliance/releases/:id/domains` | — | `[{id, domain}]` | List domains for a release request |
| `POST` | `/api/compliance/releases/:id/domains` | `{domains[]}` | `{added_count}` | Add domains to existing release (transactional, new capability) |
| `PUT` | `/api/compliance/releases/:id/domains/:domainId` | `{domain}` | `{id, domain}` | Update a single domain name |

#### Domain Status & History

| Method | Path | Query Params | Response | Purpose |
|--------|------|-------------|----------|---------|
| `GET` | `/api/compliance/domains` | `status=blocked\|released` | `[{domain, block_count, release_count}]` | Domain status view (tab filter) |
| `GET` | `/api/compliance/domains/history` | `format=json` (default) | `[{domain, request_date, reference, request_type}]` | Full history |
| `GET` | `/api/compliance/domains/history` | `format=csv\|xlsx` | File download | Export history |
| `GET` | `/api/compliance/domains` | `status=blocked&format=csv\|xlsx` | File download | Export filtered status |

#### Origins (Provenienze)

| Method | Path | Body | Response | Purpose |
|--------|------|------|----------|---------|
| `GET` | `/api/compliance/origins` | — | `[{method_id, description, is_active}]` | List all origins (active only by default for dropdowns) |
| `GET` | `/api/compliance/origins?include_inactive=true` | — | `[{method_id, description, is_active}]` | List all including soft-deleted |
| `POST` | `/api/compliance/origins` | `{description}` | `{method_id}` | Create new origin |
| `PUT` | `/api/compliance/origins/:id` | `{description}` | `{method_id}` | Update origin description |
| `DELETE` | `/api/compliance/origins/:id` | — | `204` | Soft delete (set `is_active=false`) |

---

## Cross-View User Journeys (Updated)

### Journey 1: Registra provvedimento di blocco
1. Utente apre tab **Blocchi**
2. Clicca "Nuova richiesta" → modale
3. Compila data, riferimento, seleziona origine (default AGCOM), incolla domini nel textarea
4. Frontend analizza il testo: mostra anteprima con lista domini estratti, evidenzia errori per riga
5. Utente corregge e conferma → `POST /api/compliance/blocks`
6. Backend valida tutti i domini, inserisce in transazione unica
7. Frontend invalida query cache → lista si aggiorna, nuova richiesta selezionata
8. Pannello destro mostra domini della richiesta appena creata

### Journey 2: Aggiungi domini a richiesta esistente
1. Utente seleziona una richiesta in tab **Blocchi** o **Rilasci**
2. Clicca "Aggiungi domini" → modale
3. Incolla domini, anteprima, conferma → `POST /api/compliance/blocks/:id/domains` (o releases)
4. Backend valida e inserisce in transazione
5. Lista domini si aggiorna nel pannello destro

### Journey 3: Modifica richiesta (UPDATED — detail panel form)
1. Utente seleziona una richiesta nel pannello master (sinistro)
2. Pannello dettaglio (destro) mostra: header richiesta (data, riferimento, origine) in **modalità lettura** + lista domini
3. Utente clicca "Modifica" → i campi header diventano editabili nel form
4. Utente modifica e clicca "Salva" → `PUT /api/compliance/blocks/:id`
5. Pannello torna in modalità lettura, dati aggiornati
6. Per modificare un singolo dominio: click sul dominio → campo editabile o mini-form → `PUT .../domains/:domainId`

### Journey 4: Consulta stato domini
1. Utente apre tab **Stato domini**
2. Tab "Bloccati" attivo di default — tabella: dominio, conteggio blocchi, conteggio rilasci
3. Può cercare un dominio con la ricerca
4. Può cambiare tab "Rilasciati"
5. Può esportare in CSV/XLSX → `GET /api/compliance/domains?status=blocked&format=csv`

### Journey 5: Esporta storico
1. Utente apre tab **Riepilogo**
2. Storico completo con ricerca client-side
3. Click "Esporta" → sceglie formato (CSV o XLSX)
4. `GET /api/compliance/domains/history?format=xlsx` → browser scarica il file

### Journey 6: Gestisci provenienze
1. Utente apre tab **Provenienze**
2. Lista delle origini con stato (attiva/disattivata)
3. Può creare nuova origine, modificare descrizione, disattivare (soft delete)
4. Origini disattivate non appaiono nel dropdown "Origine" nella creazione blocchi, ma restano visibili nei dati storici

---

## Frontend Data Layer

Following budget app patterns:

| Concern | Pattern | Library |
|---------|---------|---------|
| API client | `@mrsmith/api-client` → `useApiClient()` hook | Shared package |
| Data fetching | React Query `useQuery()` per view | `@tanstack/react-query` |
| Mutations | React Query `useMutation()` + cache invalidation | `@tanstack/react-query` |
| Auth | `@mrsmith/auth-client` `AuthProvider` | Keycloak JS |
| Routing | React Router with flat routes | `react-router-dom` |
| Layout | `AppShell` + `TabNav` from `@mrsmith/ui` | Shared package |

### Query Key Structure (suggested)
```
['compliance', 'blocks']                    — block request list
['compliance', 'blocks', id, 'domains']     — domains for a block
['compliance', 'releases']                  — release request list
['compliance', 'releases', id, 'domains']   — domains for a release
['compliance', 'domains', status]           — domain status view
['compliance', 'domains', 'history']        — full history
['compliance', 'origins']                   — origins list
```

### Cache Invalidation Rules
- Create/update block → invalidate `['compliance', 'blocks']`, `['compliance', 'domains', *]`, `['compliance', 'domains', 'history']`
- Create/update release → invalidate `['compliance', 'releases']`, `['compliance', 'domains', *]`, `['compliance', 'domains', 'history']`
- Add/update domains → invalidate parent request's domains key + domain status + history
- CRUD origins → invalidate `['compliance', 'origins']`

---

## Export Implementation

No export exists in the codebase yet. For the compliance app:

| Concern | Approach |
|---------|----------|
| **Generation** | Server-side (Go backend) — avoids frontend memory limits on 10K+ rows |
| **CSV** | Standard library `encoding/csv` |
| **XLSX** | Go library (e.g., `excelize`) |
| **Trigger** | Frontend sends GET with `format=csv\|xlsx` query param |
| **Response** | `Content-Disposition: attachment; filename=...`, appropriate Content-Type |
| **Filtering** | Export respects active status filter and search term (passed as query params) |

---

## Hidden Triggers / Automation

None in the compliance app. DNS enforcement is handled by external scripts that independently read the `dns_bl_*` tables.

---

## Open Items Carried Forward

| # | Item | From | Needed for |
|---|------|------|------------|
| O1 | Exact current values in `dns_bl_method` | Phase A | Phase E |
| O2 | Pagination strategy for history and status views (~10K+ rows) | Phase A | Phase E |
| O3 | Search filter persistence across sub-tabs in Stato domini | Phase B | Implementation |
| O4 | FQDN validation regex — document canonical pattern | Phase C | Implementation |
| O5 | DNS sync jobs: do they write to `dns_bl_*` tables or only read? | Phase D | Schema constraints |
| O6 | XLSX Go library selection (`excelize` or alternative) | Phase D | Implementation |

---

## Phase D Status: COMPLETE

All integrations mapped, data flows documented, API contract defined, user journeys finalized. Ready for Phase E: Specification Assembly.

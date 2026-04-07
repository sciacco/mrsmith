# Compliance — Application Specification

## Summary

| Field | Value |
|-------|-------|
| Application name | Compliance (DNS Domain Block/Release Management) |
| Audit source | `apps/compliance/AUDIT.md` (from `compliance-main.zip`, Appsmith export 2025-11-04) |
| Database | PostgreSQL, `anisetta` database, `public` schema |
| Spec status | Complete — all expert decisions recorded across Phases A–D |
| Last updated | 2026-04-07 |

## Current-State Evidence

| Aspect | Detail |
|--------|--------|
| Source pages | 6 Appsmith pages: Home, Richiesta Blocco domini, Richiesta Rilascio domini, Riepilogo domini, Domini bloccati, Domini rilasciati |
| Source entities | BlockRequest, BlockDomain, ReleaseRequest, ReleaseDomain, BlockMethod, DomainStatus (computed) |
| Source datasource | 1 PostgreSQL datasource (`compliance`, postgres-plugin), 13 queries, 2 JSObjects |
| Known audit gaps | Exact `dns_bl_method` values unknown; DNS sync job behavior (read-only vs read/write) unconfirmed; no pagination on any view |

---

## Constraint: Schema Retrocompatibility

The existing PostgreSQL schema **must not be altered** in ways that break the Appsmith application or the DNS sync jobs that consume the same tables. Both systems coexist during the transition period.

- No table renames, column renames, or type changes
- New columns must be nullable or have defaults
- No foreign key or constraint changes that would block existing queries
- One permitted addition: `is_active BOOLEAN DEFAULT TRUE` on `dns_bl_method` (for soft delete)

---

## Entity Catalog

### BlockRequest

| Aspect | Detail |
|--------|--------|
| Purpose | A block request represents a directive received from an authority (e.g., AGCOM) to block a set of domains |
| Source table | `dns_bl_block` (1,255 rows) |
| Fields | `id` (PK, serial), `request_date` (date), `reference` (text), `method_id` (FK → `dns_bl_method`) |
| Relationships | has many BlockDomain, belongs to BlockMethod |
| Operations | Create, List, Read, Update |
| Constraints | All fields required on create. Each request is a faithful snapshot of the original directive — no partial saves. |
| Not implementing | Delete |

### BlockDomain

| Aspect | Detail |
|--------|--------|
| Purpose | An individual domain name associated with a block request |
| Source table | `dns_bl_block_domain` (10,270 rows) |
| Fields | `id` (PK, serial), `domain` (text), `block_id` (FK → `dns_bl_block`) |
| Relationships | belongs to BlockRequest |
| Operations | Batch Create (within request creation or add-to-existing), List (by block_id), Update (domain name) |
| Validation | Valid FQDN, one per line. No wildcards, no IPs. No cross-request uniqueness constraint — the same domain may appear in multiple requests. |
| Not implementing | Delete |

### ReleaseRequest

| Aspect | Detail |
|--------|--------|
| Purpose | A release request to unblock previously blocked domains |
| Source table | `dns_bl_release` (14 rows) |
| Fields | `id` (PK, serial), `request_date` (date), `reference` (text) |
| Relationships | has many ReleaseDomain |
| Operations | Create, List, Read, Update |
| Constraints | No `method_id` — releases have no origin/method (confirmed by expert) |
| Not implementing | Delete |

### ReleaseDomain

| Aspect | Detail |
|--------|--------|
| Purpose | An individual domain name associated with a release request |
| Source table | `dns_bl_release_domain` (62 rows) |
| Fields | `id` (PK, serial), `domain` (text), `release_id` (FK → `dns_bl_release`) |
| Relationships | belongs to ReleaseRequest |
| Operations | Batch Create, List (by release_id), Update (domain name). Adding domains to existing release is a **new capability** (absent in legacy app). |
| Validation | Same as BlockDomain |
| Not implementing | Delete |

### BlockMethod (Provenienza / Origin)

| Aspect | Detail |
|--------|--------|
| Purpose | Lookup table for block origins/methods (e.g., AGCOM) |
| Source table | `dns_bl_method` (~few rows) |
| Fields | `method_id` (PK), `description` (text), `is_active` (boolean, **new column**, default TRUE) |
| Relationships | referenced by BlockRequest.method_id |
| Operations | Full CRUD: Create, List, Read, Update, Soft Delete (set `is_active=false`) |
| Constraints | Soft-deleted origins do not appear in the "Origine" dropdown for new block requests but remain visible in historical data. Hard SQL DELETE is never performed. |
| Default | "AGCOM" is the frontend-hardcoded default selection (decision D12) |

### DomainStatus (computed, no source table)

| Aspect | Detail |
|--------|--------|
| Purpose | Computed view showing the current block/release status of each domain |
| Source | Aggregate UNION of block and release domain tables joined with their request headers |
| Computed fields | `domain`, `block_count` (blocchi), `release_count` (rilasci), `is_blocked` (block_count > release_count) |
| Operations | List blocked, List released, List full history (all events) |
| Business rule | **BR1**: A domain is blocked when cumulative block count > cumulative release count. Method does not affect status — only numeric count matters. Confirmed by expert. |
| Export | CSV and XLSX, all records. Export respects active filter (tab + search). |

---

## Operations Summary

| Entity | Create | List | Read | Update | Delete | Batch | Export |
|--------|--------|------|------|--------|--------|-------|--------|
| BlockRequest | Yes | Yes | Yes | Yes (new) | No | — | — |
| BlockDomain | Yes | Yes | — | Yes (new) | No | Yes (new) | — |
| ReleaseRequest | Yes | Yes | Yes | Yes (new) | No | — | — |
| ReleaseDomain | Yes | Yes | — | Yes (new) | No | Yes (new, + add to existing) | — |
| BlockMethod | Yes (new) | Yes | — | Yes (new) | Soft (new) | — | — |
| DomainStatus | — | Yes (3 views) | — | — | — | — | CSV, XLSX |

---

## View Specifications

### Navigation

Uses shared `@mrsmith/ui` components: `AppShell` (sticky header, glassmorphism), `TabNav` (horizontal tabs with animated indicator), `UserMenu`. Same pattern as budget app.

| Tab | Route | Content |
|-----|-------|---------|
| Blocchi | `/blocks` | Master-detail: block requests + domains |
| Rilasci | `/releases` | Master-detail: release requests + domains |
| Stato domini | `/domains` | Unified view with sub-tabs: Bloccati / Rilasciati |
| Riepilogo | `/history` | Full history with search and export |
| Provenienze | `/origins` | CRUD for block origins |

Default landing: Blocchi (`/blocks`).

### View 1: Blocchi (`/blocks`)

| Aspect | Detail |
|--------|--------|
| User intent | Create block requests, manage domains, edit existing requests |
| Interaction pattern | Master-detail (two panels) + modals for creation |
| Left panel (master) | Table of block requests: date, origin, reference. Row selection loads detail. |
| Right panel (detail) | **Read mode**: request header (date, reference, origin) + domain list. **Edit mode**: form with editable fields + "Salva" button. Toggled by "Modifica" button. |
| Modal: new request | Form: date (default today), reference, origin (dropdown, default "AGCOM"), domains textarea (one FQDN per line). Frontend parses and shows preview before submit. |
| Modal: add domains | Form: domains textarea. Adds to currently selected request. Same parse/preview UX. |
| Changes from legacy | Save buttons now functional (PUT). Batch insert replaces N+1. Detail panel form replaces broken inline editing. SQL injection fixed. |

### View 2: Rilasci (`/releases`)

| Aspect | Detail |
|--------|--------|
| User intent | Create release requests, manage domains, edit existing requests |
| Interaction pattern | Master-detail (two panels) + modals for creation |
| Left panel (master) | Table of release requests: date, reference. Row selection loads detail. |
| Right panel (detail) | Same read/edit mode pattern as Blocchi, without origin field. |
| Modal: new request | Form: date (default today), reference, domains textarea. |
| Modal: add domains | **New capability** — absent in legacy app. Same UX as Blocchi. |
| Changes from legacy | Add-domains modal added. Save buttons functional. Detail panel form for editing. |

### View 3: Stato domini (`/domains`)

| Aspect | Detail |
|--------|--------|
| User intent | View which domains are currently blocked or released |
| Interaction pattern | Read-only tabbed list with search and export |
| Sub-tabs | "Bloccati" (default active) / "Rilasciati" |
| Table columns | domain, block count, release count |
| Search | Client-side text filter |
| Export | CSV and XLSX buttons. Export all records matching active tab + search filter. Server-side file generation. |
| Changes from legacy | Two pages merged into one tabbed view. Export formats specified. |

### View 4: Riepilogo (`/history`)

| Aspect | Detail |
|--------|--------|
| User intent | Consult full chronological history of all block/release events. Export. |
| Interaction pattern | Read-only list with search and export |
| Table columns | domain, request date, reference, request type (block/release) |
| Search | Client-side text filter |
| Export | CSV and XLSX. All records (respecting search filter). Server-side generation. |
| Changes from legacy | Export formats specified. Pagination TBD. |

### View 5: Provenienze (`/origins`)

| Aspect | Detail |
|--------|--------|
| User intent | Manage the list of block origins (e.g., AGCOM) |
| Interaction pattern | Simple CRUD list |
| Table columns | description, status (active/inactive) |
| Actions | Create, edit description, soft delete (deactivate) |
| Constraints | Soft-deleted origins hidden from block creation dropdown, visible in historical data. |
| Changes from legacy | Entirely new view — origins were previously unmanaged. |

---

## Logic Allocation

### Backend Responsibilities

| Logic | Implementation |
|-------|---------------|
| Request creation (header + domains) | Single transactional endpoint. Validate all domains; reject entire request if any invalid (400 + error details). |
| Add domains to existing request | Transactional batch insert with same validation. |
| Update request header | PUT endpoint, field validation. |
| Update domain name | PUT endpoint, FQDN validation. |
| Domain status computation (BR1) | Parameterized aggregate query: `blocked` or `released` filter. |
| History UNION query | Server-side query, includes `request_type` field. |
| Origins CRUD + soft delete | Standard CRUD with `is_active` flag. |
| Export (CSV/XLSX) | Server-side file generation and streaming. |
| Domain validation (authoritative) | FQDN regex — rejects entire request if any domain fails. |
| Authorization | `acl.RequireRole("app_compliance_access")` on all endpoints. |

### Frontend Responsibilities

| Logic | Implementation |
|-------|---------------|
| Domain parsing (UX preview) | Parse textarea, extract one FQDN per line, show preview with per-line error highlighting. |
| Domain validation (instant feedback) | Same FQDN regex as backend, run locally for UX. |
| Default origin "AGCOM" | Hardcoded dropdown default value. |
| Default date = today | Form default. |
| UI state (disabled buttons, read/edit mode toggle) | Standard React state management. |
| Dropdown population | Map origins API response to select options (filter `is_active=true`). |
| Export trigger | Initiate download via GET with `format` query param. |
| Data fetching + caching | React Query with structured query keys and invalidation rules. |

### Shared (documented once, implemented in both layers)

| Logic | Contract |
|-------|----------|
| FQDN validation regex | Canonical pattern documented in API spec. Frontend uses for preview; backend uses as authoritative gate. Both must produce identical results. |

---

## API Contract

### Authentication & Authorization

All endpoints require:
- `Authorization: Bearer {JWT}` header (Keycloak token)
- Role `app_compliance_access` in JWT realm_access.roles

### Block Requests

| Method | Path | Body / Params | Response |
|--------|------|---------------|----------|
| `GET` | `/api/compliance/blocks` | — | `[{id, request_date, reference, method_id, method_description}]` |
| `GET` | `/api/compliance/blocks/:id` | — | `{id, request_date, reference, method_id, method_description}` |
| `POST` | `/api/compliance/blocks` | `{request_date, reference, method_id, domains[]}` | `{id, domains_count}` |
| `PUT` | `/api/compliance/blocks/:id` | `{request_date, reference, method_id}` | `{id}` |
| `GET` | `/api/compliance/blocks/:id/domains` | — | `[{id, domain}]` |
| `POST` | `/api/compliance/blocks/:id/domains` | `{domains[]}` | `{added_count}` |
| `PUT` | `/api/compliance/blocks/:id/domains/:domainId` | `{domain}` | `{id, domain}` |

### Release Requests

| Method | Path | Body / Params | Response |
|--------|------|---------------|----------|
| `GET` | `/api/compliance/releases` | — | `[{id, request_date, reference}]` |
| `GET` | `/api/compliance/releases/:id` | — | `{id, request_date, reference}` |
| `POST` | `/api/compliance/releases` | `{request_date, reference, domains[]}` | `{id, domains_count}` |
| `PUT` | `/api/compliance/releases/:id` | `{request_date, reference}` | `{id}` |
| `GET` | `/api/compliance/releases/:id/domains` | — | `[{id, domain}]` |
| `POST` | `/api/compliance/releases/:id/domains` | `{domains[]}` | `{added_count}` |
| `PUT` | `/api/compliance/releases/:id/domains/:domainId` | `{domain}` | `{id, domain}` |

### Domain Status & History

| Method | Path | Query Params | Response |
|--------|------|-------------|----------|
| `GET` | `/api/compliance/domains` | `status=blocked\|released` | `[{domain, block_count, release_count}]` |
| `GET` | `/api/compliance/domains` | `status=...&format=csv\|xlsx` | File download |
| `GET` | `/api/compliance/domains/history` | — | `[{domain, request_date, reference, request_type}]` |
| `GET` | `/api/compliance/domains/history` | `format=csv\|xlsx` | File download |

### Origins (Provenienze)

| Method | Path | Query Params / Body | Response |
|--------|------|---------------------|----------|
| `GET` | `/api/compliance/origins` | `include_inactive=true` (optional) | `[{method_id, description, is_active}]` |
| `POST` | `/api/compliance/origins` | `{description}` | `{method_id}` |
| `PUT` | `/api/compliance/origins/:id` | `{description}` | `{method_id}` |
| `DELETE` | `/api/compliance/origins/:id` | — | `204` (soft delete) |

### Error Responses

| Status | When | Body |
|--------|------|------|
| `400` | Validation failure (invalid domains) | `{error: "invalid_domains", message: "...", invalid: ["bad.domain", ...]}` |
| `400` | Missing required fields | `{error: "validation_error", message: "..."}` |
| `401` | No token or invalid token | `{error: "unauthorized"}` |
| `403` | Missing `app_compliance_access` role | `{error: "forbidden"}` |
| `404` | Request or domain not found | `{error: "not_found"}` |

---

## Integrations and Data Flow

### System Map

```
┌──────────────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│  Compliance App      │     │  PostgreSQL           │     │  DNS Sync Jobs   │
│  (React + Go)        │────►│  anisetta / public    │◄────│  (read dns_bl_*) │
│                      │     │                       │     │                  │
│  Appsmith (legacy)   │────►│  dns_bl_block         │     │  Enforce blocks  │
│  (coexists)          │     │  dns_bl_block_domain  │     │  on DNS infra    │
│                      │     │  dns_bl_release       │     │                  │
│  Keycloak            │     │  dns_bl_release_domain│     │                  │
│  (auth provider)     │     │  dns_bl_method        │     │                  │
└──────────────────────┘     └──────────────────────┘     └──────────────────┘
```

- **Compliance app**: owns data entry (create, update)
- **DNS sync jobs**: consume blocked domain list (read). Out of scope for this app.
- **Appsmith**: coexists during transition. Same tables, same schema. Will be decommissioned.
- **Keycloak**: authentication and authorization for both frontend and backend

### Frontend Data Layer

| Concern | Pattern | Library |
|---------|---------|---------|
| API client | `@mrsmith/api-client` → `useApiClient()` hook | Shared package |
| Data fetching | React Query `useQuery()` | `@tanstack/react-query` |
| Mutations | React Query `useMutation()` + cache invalidation | `@tanstack/react-query` |
| Auth | `@mrsmith/auth-client` `AuthProvider` | Keycloak JS |
| Routing | React Router, flat routes | `react-router-dom` |
| Layout | `AppShell` + `TabNav` | `@mrsmith/ui` |

### Query Keys

```
['compliance', 'blocks']
['compliance', 'blocks', id, 'domains']
['compliance', 'releases']
['compliance', 'releases', id, 'domains']
['compliance', 'domains', status]
['compliance', 'domains', 'history']
['compliance', 'origins']
```

### Cache Invalidation

- Create/update block request → invalidate `blocks`, `domains/*`, `history`
- Create/update release request → invalidate `releases`, `domains/*`, `history`
- Add/update domains → invalidate parent's domains key + `domains/*` + `history`
- CRUD origins → invalidate `origins`

### Export

| Concern | Approach |
|---------|----------|
| Generation | Server-side (Go backend) |
| CSV | Go `encoding/csv` |
| XLSX | Go library (e.g., `excelize`) |
| Trigger | Frontend GET with `format=csv\|xlsx` query param |
| Response | `Content-Disposition: attachment`, appropriate Content-Type |
| Filtering | Respects active status filter and search term |

---

## User Journeys

### J1: Registra provvedimento di blocco
1. Tab **Blocchi** → "Nuova richiesta" → modale
2. Compila: data (default oggi), riferimento, origine (default AGCOM), incolla domini
3. Frontend mostra anteprima domini estratti con errori evidenziati per riga
4. Conferma → `POST /api/compliance/blocks`
5. Backend valida, transazione unica. Se errori → 400 con lista domini invalidi.
6. Successo → lista aggiornata, richiesta selezionata, domini visibili

### J2: Aggiungi domini a richiesta esistente
1. Seleziona richiesta (Blocchi o Rilasci) → "Aggiungi domini" → modale
2. Incolla domini, anteprima, conferma → `POST .../domains`
3. Domini aggiunti, lista aggiornata

### J3: Modifica richiesta
1. Seleziona richiesta → pannello dettaglio in **modalità lettura**
2. "Modifica" → campi editabili nel form → "Salva" → `PUT`
3. Torna in modalità lettura con dati aggiornati
4. Per dominio singolo: click → campo editabile → salva → `PUT .../domains/:id`

### J4: Consulta stato domini
1. Tab **Stato domini** → sub-tab "Bloccati" (default)
2. Ricerca, cambio tab "Rilasciati"
3. Esporta CSV/XLSX

### J5: Esporta storico
1. Tab **Riepilogo** → ricerca → "Esporta" → CSV o XLSX
2. Download file con tutti i record

### J6: Gestisci provenienze
1. Tab **Provenienze** → lista origini (attive/disattivate)
2. Crea, modifica descrizione, disattiva (soft delete)
3. Origini disattivate nascoste dal dropdown creazione blocchi, visibili nello storico

---

## Constraints and Non-Functional Requirements

### Security
- All API endpoints behind Keycloak JWT validation + `app_compliance_access` role
- No direct database access from frontend
- All SQL via parameterized queries (fixes legacy SQL injection S1)
- Domain validation on both frontend and backend (backend authoritative)
- Backend rejects entire request if any domain is invalid (data integrity for legal records)

### Schema Retrocompatibility
- No breaking changes to `dns_bl_*` tables
- Only permitted addition: `is_active` column on `dns_bl_method` (nullable/defaulted)
- Appsmith and DNS sync jobs must continue to function unchanged

### Performance
- Domain status and history views: pagination strategy TBD (~10K+ rows currently)
- Export: server-side streaming to handle large datasets
- Batch domain insert replaces N+1 sequential inserts

### Operational
- Coexistence with Appsmith during transition
- DNS sync jobs are external consumers — app must not interfere with their reads
- Go backend follows existing monolith pattern (`backend/internal/compliance/`)

---

## Open Questions and Deferred Decisions

| # | Question | Needed Input | Decision Owner |
|---|----------|-------------|----------------|
| O1 | Exact current values in `dns_bl_method` table | DB query | Dev team |
| O2 | Pagination strategy for history and status views | UX + performance assessment | Dev team + expert |
| O3 | Should search filter persist across sub-tabs in Stato domini? | UX preference | Expert |
| O4 | Canonical FQDN validation regex pattern | Document and test | Dev team |
| O5 | DNS sync jobs: do they write to `dns_bl_*` tables or only read? | Ops team confirmation | Ops |
| O6 | XLSX Go library selection (`excelize` or alternative) | Dev team evaluation | Dev team |
| O7 | Domain edit UX in detail panel: inline field or mini-modal? | UX refinement | Implementation phase |

---

## Acceptance Notes

### What the audit proved directly
- 6 pages, 13 queries, 5 tables, 2 JSObjects with known methods
- N+1 insert pattern, SQL injection risk, broken save buttons, duplicated logic
- Business rule BR1 (blocked = blocks > releases) embedded in SQL
- Domain validation via regex, domain extraction from free text
- No auth in app layer, no delete, no pagination

### What the expert confirmed
- Schema retrocompatibility required (Appsmith + DNS sync coexistence)
- Entities remain separate (BlockRequest / ReleaseRequest)
- Update yes, delete no (for requests and domains)
- Full CRUD for origins with soft delete
- FQDN only, one per line, no uniqueness constraint across requests
- Each request is a faithful snapshot of an authority directive
- Default "AGCOM" hardcoded in frontend
- Strict validation: reject entire request if any domain invalid
- Shared parsing: frontend for preview, backend for security
- Detail panel form for editing (not inline table editing)
- Unified "Stato domini" with sub-tabs
- Export CSV + XLSX, server-side
- Navigation consistent with budget app (AppShell + TabNav)
- "Provenienze" as tab label for origins management
- DNS enforcement is automatic via external scripts (out of scope)

### What still needs validation
- Exact `dns_bl_method` values (O1)
- DNS sync job write behavior (O5)
- Pagination approach for large views (O2)
- Canonical FQDN regex (O4)

---

## Phase Documents

| Phase | File | Status |
|-------|------|--------|
| A: Entity-Operation Model | `compliance-migspec-phaseA.md` | Complete |
| B: UX Pattern Map | `compliance-migspec-phaseB.md` | Complete |
| C: Logic Placement | `compliance-migspec-phaseC.md` | Complete |
| D: Integration and Data Flow | `compliance-migspec-phaseD.md` | Complete |
| E: Specification Assembly | `compliance-migspec.md` (this file) | Complete |

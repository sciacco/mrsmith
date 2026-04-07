# Compliance — Migration Spec Phase C: Logic Placement

**Source**: `apps/compliance/AUDIT.md` + Phase A/B decisions
**Date**: 2026-04-07
**Status**: Complete

---

## Decisions Log

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| D12 | Default method "AGCOM" | **Hardcoded nel frontend** | Semplice, il default cambia raramente. Se cambia, serve un rilascio. |
| D13 | Backend validation behavior | **Strict reject (A)** — rifiuta l'intera richiesta se un dominio non è valido | Contesto legale/regolamentare: la richiesta è una fotografia del provvedimento, non ammette salvataggio parziale. |
| D14 | Domain parsing location | **Both (C)** — frontend per UX, backend per sicurezza | Frontend: anteprima, feedback, correzione. Backend: trust boundary, non si fida dell'input. Duplicazione intenzionale e corretta. |

---

## Logic Inventory and Placement

### Domain Logic → Backend

| Logic | Current Location | New Placement | API Surface |
|-------|-----------------|---------------|-------------|
| Block request creation (header + domains in transaction) | `inserisciBlocco()` — frontend, N+1 inserts | **Backend** — single transactional endpoint | `POST /api/compliance/blocks` body: `{request_date, reference, method_id, domains[]}` |
| Add domains to existing block | `inserisciDomini()` — frontend, N+1 inserts | **Backend** — transactional batch | `POST /api/compliance/blocks/:id/domains` body: `{domains[]}` |
| Release request creation (header + domains) | `togliBlocco()` — frontend, N+1 inserts | **Backend** — single transactional endpoint | `POST /api/compliance/releases` body: `{request_date, reference, domains[]}` |
| Add domains to existing release | Not implemented | **Backend** — new capability | `POST /api/compliance/releases/:id/domains` body: `{domains[]}` |
| Update block request header | Not implemented (broken save) | **Backend** | `PUT /api/compliance/blocks/:id` body: `{request_date, reference, method_id}` |
| Update release request header | Not implemented (broken save) | **Backend** | `PUT /api/compliance/releases/:id` body: `{request_date, reference}` |
| Update domain name | Not implemented (broken save) | **Backend** | `PUT /api/compliance/blocks/:blockId/domains/:domainId` or equivalent for releases |
| Domain status computation (BR1) | SQL HAVING in UI layer, duplicated x2 | **Backend** — parameterized query or view | `GET /api/compliance/domains?status=blocked\|released` |
| History union query | SQL UNION ALL in UI layer | **Backend** | `GET /api/compliance/domains/history` |
| Block methods CRUD | Read-only in UI, no management | **Backend** — full CRUD with soft delete | `GET/POST/PUT/DELETE /api/compliance/origins` |
| Request type labeling (`'block'`/`'release'`) | Hardcoded strings in SQL UNION | **Backend** — API response includes type field | Part of history endpoint response |

### Domain Logic → Shared (Frontend + Backend)

| Logic | Details | Contract |
|-------|---------|----------|
| Domain name validation (FQDN regex) | Currently `validaDomini()` / `validaDomini2()`, duplicated x3 across pages | **Canonical rule documented in API spec.** Frontend implements for instant per-line feedback. Backend implements as authoritative gate — rejects entire request if any domain is invalid. Both use same regex pattern, maintained independently but tested against shared fixtures. |
| Domain extraction from text (one FQDN per line) | Currently `estraiDomini()`, duplicated x2 | **Frontend**: parses for preview/confirmation UX. **Backend**: re-parses raw text or validates structured array. See parsing contract below. |

### Presentation / UX → Frontend Only

| Logic | Details |
|-------|---------|
| Default method "AGCOM" | Hardcoded in dropdown default value (decision D12) |
| Default date = today | `Date()` as form default |
| `isDisabled` bindings (no row selected, no pending edits) | Standard React state management |
| Dropdown population from methods/origins list | Map API response to select options |
| Contextual titles ("Domini da aggiungere alla richiesta X del Y") | Template string from selected row data |
| Export trigger (CSV/XLSX) | Frontend initiates download from API data |

---

## Parsing Contract

The domain textarea input follows this contract between frontend and backend:

**Input format**: free text, one domain per line. Users paste from PDFs, emails, spreadsheets.

**Frontend responsibility**:
1. Parse text into individual lines
2. Trim whitespace, skip empty lines
3. Validate each line as FQDN (regex)
4. Display extracted domains as preview list with per-line error highlighting
5. Allow user to correct before submission
6. Send to backend as structured array: `domains: ["example.com", "test.org", ...]`

**Backend responsibility**:
1. Receive structured array of domain strings
2. Re-validate each domain against same FQDN rule
3. If ANY domain is invalid → reject entire request with 400 + error details (which domains failed)
4. If all valid → insert in single transaction

**Error response format** (suggested):
```json
{
  "error": "invalid_domains",
  "message": "Some domains failed validation",
  "invalid": ["not-a-domain", "also bad.."]
}
```

---

## Eliminated Logic

| What | Why |
|------|-----|
| `validaDomini()` (widget-bound version) | Absorbed by shared FQDN validation — parameterized version only |
| Duplicate `utils` JSObject across pages | Single shared validation utility in frontend |
| `get_release_domains` (redundant query) | Merged with `get_domains` — one endpoint per entity |
| `get_domains_to_block` name reuse with opposite HAVING | Replaced by single parameterized backend endpoint |
| Prepared statement = false (SQL injection risk S1) | All SQL behind backend with parameterized queries |

---

## Security Improvements

| Issue | Current | New |
|-------|---------|-----|
| S1: SQL injection in `get_block_domains` | Prepared statement disabled, widget value in query | All queries behind backend with parameterized SQL |
| S2: Direct DB access from UI | Appsmith connects directly to PostgreSQL | Backend API layer with authorization |
| S3: No input sanitization beyond regex | Only format check | Backend validates + rejects. No direct SQL from frontend. |

---

## Open Items Carried Forward

| # | Item | From | Needed for |
|---|------|------|------------|
| O1 | Exact current values in `dns_bl_method` | Phase A | Phase E |
| O2 | Pagination strategy for history and status views | Phase A | Phase D |
| O3 | Search filter persistence across sub-tabs in Stato domini | Phase B | Implementation |
| O4 | FQDN validation regex — document canonical pattern | Phase C | Implementation |

---

## Phase C Status: COMPLETE

All logic classified and placed. Ready for Phase D: Integration and Data Flow.

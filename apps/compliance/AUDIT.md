# Compliance App — Appsmith Structural Audit

**Source**: `compliance-main.zip` (Appsmith git export, 2025-11-04)
**Database schema**: `anisetta_schema.json` (PostgreSQL, `anisetta` database)
**Datasource**: `compliance` (postgres-plugin)
**Theme**: Modern (Default-New), sidebar navigation, light color style

---

## 1. Application Inventory

### Pages

| # | Page | Default | Purpose |
|---|------|---------|---------|
| 1 | Home | Yes | Landing page with cover image |
| 2 | Richiesta Blocco domini | No | Create and manage domain block requests |
| 3 | Richiesta Rilascio domini | No | Create and manage domain release (unblock) requests |
| 4 | Riepilogo domini | No | Read-only summary of all domain block/release history |
| 5 | Domini bloccati | No | Read-only list of currently blocked domains (net blocks > releases) |
| 6 | Domini rilasciati | No | Read-only list of released domains (net blocks <= releases) |

### Datasources

| Name | Plugin | Used by |
|------|--------|---------|
| compliance | postgres-plugin | All pages except Home |

### JSObjects

| Name | Page | Methods |
|------|------|---------|
| utils | Richiesta Blocco domini | `validaDomini()`, `validaDomini2(test)`, `estraiDomini(testo)`, `inserisciBlocco()`, `inserisciDomini()` |
| utils | Richiesta Rilascio domini | `validaDomini2(test)`, `estraiDomini(testo)`, `togliBlocco()` |

### Navigation Pattern

Sidebar navigation between all 6 pages. No programmatic cross-page navigation detected.

### Global Notes

- All queries target the `public` schema directly
- No authentication/authorization logic in the app — relies on Appsmith platform auth
- Italian-language UI labels throughout
- Fixed layout positioning (not auto-layout)
- Application version 2, evaluation version 2

---

## 2. Page Audits

### 2.1 Home

**Purpose**: Landing/splash page with a static cover image.

**Widgets**:
| Widget | Type | Role |
|--------|------|------|
| Image1 | IMAGE_WIDGET | Displays cover image from `https://t.sciacco.net/x/copertina_compliance.jpg` |

**Queries**: None

**Hidden logic**: None

**Migration notes**: Trivial page. Replace with portal card/tile or remove.

---

### 2.2 Richiesta Blocco domini

**Purpose**: Primary workflow page for creating domain block requests and managing domains within each request.

**Layout**: Two-panel master-detail with two modals.

**Widgets**:

| Widget | Type | Role |
|--------|------|------|
| Container2 (left panel) | CONTAINER | Houses block request list |
| Text1 | TEXT | Warning: "Non duplicare le richieste che contengono più domini" (red) |
| btn_new_block | ICON_BUTTON (+) | Opens `modal_block` to create new block request |
| IconButton6 | ICON_BUTTON (draw) | Save changes — disabled when `tbl_domini.updatedRows.length < 1` |
| tbl_blocchi | TABLE_V2 | Master table: block requests (id, request_date, description/method, reference) |
| Container1 (right panel) | CONTAINER | Houses domain list for selected block |
| IconButton2 | ICON_BUTTON (+) | Opens `mdl_domain` — disabled when `tbl_blocchi.selectedRow.id == ""` |
| IconButton5 | ICON_BUTTON (draw) | Save changes — disabled when `tbl_domini.updatedRows.length < 1` |
| tbl_domini | TABLE_V2 | Detail table: domains for selected block (domain is inline-editable) |
| modal_block | MODAL | New block request form |
| mdl_domain | MODAL | Add domains to existing block request |

**Queries**:

| Query | Type | Execute on load | Purpose |
|-------|------|-----------------|---------|
| get_blocks | DB | Yes | List all block requests joined with method descriptions, ordered by date desc |
| get_block_domains | DB | Yes | List domains for `tbl_blocchi.selectedRow.id` |
| get_methods | DB | Yes | Populate method dropdown (label/value from `dns_bl_method`) |
| new_block | DB | No | Insert new block request, returns id |
| new_domain | DB | No | Insert single domain into block_domain |

**Event Flow**:

1. Page loads → `get_blocks`, `get_block_domains`, `get_methods` execute
2. User selects row in `tbl_blocchi` → `onRowSelected` triggers `get_block_domains.run()`
3. User clicks `btn_new_block` (+) → opens `modal_block`
4. In `modal_block`: user fills date, reference, method, domains → clicks "Inserisci"
   - Calls `utils.inserisciBlocco()`:
     1. Extracts domains from `Input1.text` via regex
     2. Runs `new_block` with params (request_date, reference, method_id)
     3. Gets returned block id
     4. Loops: runs `new_domain` for each extracted domain
     5. Refreshes `get_blocks` and `get_block_domains`
   - Closes modal
5. User clicks `IconButton2` (+) on right panel → opens `mdl_domain`
6. In `mdl_domain`: user enters domains → clicks "Inserisci"
   - Calls `utils.inserisciDomini()`:
     1. Extracts domains from `in_domains.text`
     2. Uses `tbl_blocchi.selectedRow.id` as block_id
     3. Loops: runs `new_domain` for each domain
     4. Refreshes `get_block_domains`
   - Closes modal

**Hidden Logic**:

| Location | Binding | Classification |
|----------|---------|----------------|
| IconButton2.isDisabled | `tbl_blocchi.selectedRow.id == ""` | UI orchestration — prevent adding domains without a selected block |
| IconButton5.isDisabled | `tbl_domini.updatedRows.length < 1` | UI orchestration — save button only active when edits exist |
| IconButton6.isDisabled | `tbl_domini.updatedRows.length < 1` | UI orchestration — duplicate of above (left panel) |
| Input1.validation | `utils.validaDomini()` | Business rule — domain format validation via regex |
| in_domains.validation | `utils.validaDomini2(in_domains.text)` | Business rule — domain format validation via regex |
| tbl_blocchi.description column | computedValue shows `method_id`, selectOptions from `get_methods.data` | UI orchestration — editable select mapped to method lookup |
| sl_method.sourceData | `get_methods.data.map(...)` | Presentation — dropdown population |
| sl_method.defaultOptionValue | `"AGCOM"` hardcoded | Business rule — default block method is AGCOM |
| date_request_date.defaultDate | `Date()` (current date) | Presentation |
| Text3.text | `"Domini da aggiungere alla richiesta {{tbl_blocchi.selectedRow.reference}} del {{tbl_blocchi.selectedRow.request_date}}"` | Presentation — contextual title |

**Candidate Domain Entities**: Block, BlockDomain, Method

**Migration Notes**:
- `inserisciBlocco()` does N+1 sequential inserts (one per domain) — should be a single backend transaction with batch insert
- Inline editing on `tbl_blocchi` (reference, description/method) and `tbl_domini` (domain) is enabled but **no save handler is wired** to the save buttons (IconButton5, IconButton6 have no `onClick`). This appears to be incomplete/broken functionality
- Domain validation regex is duplicated across `validaDomini()` and `validaDomini2()` — only difference is input source (widget reference vs parameter)
- The `get_block_domains` query uses `pluginSpecifiedTemplates: [{ value: false }]` (prepared statement = false) while binding `tbl_blocchi.selectedRow.id` — potential SQL injection risk

---

### 2.3 Richiesta Rilascio domini

**Purpose**: Create and manage domain release (unblock) requests. Mirrors the block page structure but for releases.

**Layout**: Two-panel master-detail with one modal.

**Widgets**:

| Widget | Type | Role |
|--------|------|------|
| Container1 (left panel) | CONTAINER | Release request list |
| Text1 | TEXT | "Richieste di rilascio" (bold) |
| IconButton1 | ICON_BUTTON (+) | Opens `modal_block` to create new release request |
| IconButton2 | ICON_BUTTON (draw) | Save changes — disabled when `tbl_release.updatedRows.length < 1` |
| tbl_release | TABLE_V2 | Master table: release requests (id, request_date, reference) |
| Container2 (right panel) | CONTAINER | Domain list for selected release |
| IconButton1Copy | ICON_BUTTON | Unclear purpose — no onClick handler visible |
| IconButton2Copy | ICON_BUTTON (draw) | Save — disabled when `tbl_release.updatedRows.length < 1` |
| tbl_domains | TABLE_V2 | Detail table: domains for selected release |
| modal_block | MODAL | New release request form |

**Queries**:

| Query | Type | Execute on load | Purpose |
|-------|------|-----------------|---------|
| get_release | DB | Yes | List all release requests ordered by date desc |
| get_domains | DB | Yes | List domains for `tbl_release.selectedRow.id` |
| get_release_domains | DB | No | Duplicate of `get_domains` — fetches domains for selected release |
| new_release | DB | No | Insert new release request, returns id |
| new_release_domain | DB | No | Insert single domain into release_domain |

**Event Flow**:

1. Page loads → `get_release`, `get_domains` execute
2. User selects row in `tbl_release` → `onRowSelected` triggers `get_domains.run()`
3. User clicks IconButton1 (+) → opens `modal_block`
4. In `modal_block`: user fills date, reference, domains → clicks "Inserisci"
   - Calls `utils.togliBlocco()`:
     1. Extracts domains from `Input1.text` via regex
     2. Runs `new_release` with params (request_date, reference)
     3. Gets returned release id
     4. Loops: runs `new_release_domain` for each domain
     5. Refreshes `get_release` and `get_release_domains`
   - Closes modal

**Hidden Logic**:

| Location | Binding | Classification |
|----------|---------|----------------|
| IconButton2.isDisabled | `tbl_release.updatedRows.length < 1` | UI orchestration |
| Input1.validation | `utils.validaDomini2(Input1.text)` | Business rule — domain format validation |
| date_request_date.defaultDate | `Date()` | Presentation |

**Candidate Domain Entities**: Release, ReleaseDomain

**Migration Notes**:
- `get_domains` and `get_release_domains` are near-duplicates (same table, same filter) — `get_release_domains` is never executed on load and appears redundant
- No method/origin field on releases (unlike blocks) — simpler model
- Same N+1 insert pattern as block page
- Save buttons (IconButton2, IconButton2Copy) have no onClick — inline editing save is non-functional
- No mechanism to add domains to an existing release after creation (no equivalent of `mdl_domain` modal)

---

### 2.4 Riepilogo domini

**Purpose**: Read-only audit log of all domain block and release events.

**Widgets**:

| Widget | Type | Role |
|--------|------|------|
| Text1 | TEXT | Title: "Elenco richieste domini" (bold) |
| Table1 | TABLE_V2 | Full history table with columns: domain, request_date, reference, request_type |

**Queries**:

| Query | Execute on load | Purpose |
|-------|-----------------|---------|
| get_all_domains | Yes | UNION ALL of block_domain+block and release_domain+release, ordered by domain asc |

**Hidden Logic**: None significant. Client-side search enabled, download enabled.

**Migration Notes**:
- The UNION query returns all rows with no pagination — could be expensive at scale (currently ~10K block domains + 62 release domains)
- `request_type` is hardcoded as `'block'` or `'release'` in SQL — this is a business rule embedded in query

---

### 2.5 Domini bloccati

**Purpose**: Read-only view of domains that are currently in a blocked state (net block count > release count).

**Widgets**:

| Widget | Type | Role |
|--------|------|------|
| Text1 | TEXT | Title: "Elenco di domini da tenere bloccati" (bold) |
| Table1 | TABLE_V2 | Columns: domain, blocchi (block count), rilasci (release count) |

**Queries**:

| Query | Execute on load | Purpose |
|-------|-----------------|---------|
| get_domains_to_block | Yes | Aggregate query: `HAVING sum(blocchi) - sum(rilasci) > 0` |

**Hidden Logic**: The "currently blocked" logic is a **business rule embedded in SQL**: a domain is blocked when its cumulative block count exceeds its release count.

**Migration Notes**:
- This computed state (is_blocked) should become a backend-computed view or materialized state

---

### 2.6 Domini rilasciati

**Purpose**: Read-only view of domains that have been fully released (net block count <= 0).

**Widgets**:

| Widget | Type | Role |
|--------|------|------|
| Text1 | TEXT | Title: "Elenco di domini rilasciati" (bold) |
| Table1 | TABLE_V2 | Columns: domain, blocchi, rilasci |

**Queries**:

| Query | Execute on load | Purpose |
|-------|-----------------|---------|
| get_domains_to_block | Yes | Same as "Domini bloccati" but `HAVING sum(blocchi) - sum(rilasci) < 1` |

**Hidden Logic**: Same business rule as "Domini bloccati" but inverted condition.

**Migration Notes**:
- Query name `get_domains_to_block` is misleading on this page — it actually gets released domains
- Nearly identical query to "Domini bloccati" — should be a single parameterized backend endpoint

---

## 3. Datasource & Query Catalog

### Database Tables Used (compliance domain)

| Table | Role | Rows | Used by |
|-------|------|------|---------|
| `dns_bl_block` | Block request header (date, reference, method) | 1,255 | Blocco, Riepilogo, Domini bloccati/rilasciati |
| `dns_bl_block_domain` | Domains associated with a block request | 10,270 | Blocco, Riepilogo, Domini bloccati/rilasciati |
| `dns_bl_method` | Lookup table for block methods/origins | ~few | Blocco |
| `dns_bl_release` | Release request header (date, reference) | 14 | Rilascio, Riepilogo, Domini bloccati/rilasciati |
| `dns_bl_release_domain` | Domains associated with a release request | 62 | Rilascio, Riepilogo, Domini bloccati/rilasciati |

### Database Tables NOT Used by Compliance

The `anisetta` database also contains tables unrelated to the compliance domain:

| Table | Domain | Rows |
|-------|--------|------|
| `dns_bl_block_original` | Legacy/migration data (has `zoho_id`) | 1,238 |
| `dns_bl_domain_original` | Legacy/migration data (has `zoho_id`) | 9,849 |
| `as7_pbx`, `as7_tenants`, `as7_pbx_accounting` | PBX/telephony management | 7–162K |
| `dids` | Phone number (DID) inventory | 100 |
| `rdf_richieste`, `rdf_fattibilita_fornitori`, `rdf_fornitori`, `rdf_tecnologie`, `rdf_allegati` | Feasibility request workflow | 77–137 |

### Query Catalog

| Query | Page | Type | Read/Write | Params | Prepared | Rewrite Recommendation |
|-------|------|------|------------|--------|----------|----------------------|
| get_blocks | Blocco | DB | Read | None | Yes | Backend GET `/api/compliance/blocks` |
| get_block_domains | Blocco | DB | Read | `tbl_blocchi.selectedRow.id` | **No** | Backend GET `/api/compliance/blocks/:id/domains` — **fix SQL injection** |
| get_methods | Blocco | DB | Read | None | Yes | Backend GET `/api/compliance/methods` (or embed in config) |
| new_block | Blocco | DB | Write | request_date, reference, method_id | Yes | Backend POST `/api/compliance/blocks` |
| new_domain | Blocco | DB | Write | domain, block_id | Yes | Part of batch POST above |
| get_release | Rilascio | DB | Read | None | Yes | Backend GET `/api/compliance/releases` |
| get_domains | Rilascio | DB | Read | `tbl_release.selectedRow.id` | Yes | Backend GET `/api/compliance/releases/:id/domains` |
| get_release_domains | Rilascio | DB | Read | `tbl_release.selectedRow.id` | Yes | Redundant — merge with `get_domains` |
| new_release | Rilascio | DB | Write | request_date, reference | Yes | Backend POST `/api/compliance/releases` |
| new_release_domain | Rilascio | DB | Write | domain, release_id | Yes | Part of batch POST above |
| get_all_domains | Riepilogo | DB | Read | None | Yes | Backend GET `/api/compliance/domains/history` |
| get_domains_to_block | Bloccati | DB | Read | None | Yes | Backend GET `/api/compliance/domains/blocked` |
| get_domains_to_block | Rilasciati | DB | Read | None | Yes | Backend GET `/api/compliance/domains/released` |

---

## 4. Findings Summary

### Embedded Business Rules

| # | Rule | Location | Severity |
|---|------|----------|----------|
| BR1 | A domain is "blocked" when cumulative blocks > cumulative releases | `get_domains_to_block` SQL (`HAVING` clause) on both Domini bloccati and Domini rilasciati | High — core domain logic in UI-layer SQL |
| BR2 | Domain name validation regex | `utils.validaDomini()` / `utils.validaDomini2()` JSObjects | Medium — should be shared backend validation |
| BR3 | Domain extraction from free-text input | `utils.estraiDomini()` JSObject | Medium — parsing logic in client |
| BR4 | Default block method is "AGCOM" | `sl_method.defaultOptionValue` hardcoded | Low — but documents a business default |
| BR5 | Block requests have a method/origin; release requests do not | Schema difference between `dns_bl_block` (has `method_id`) and `dns_bl_release` (no method) | Low — implicit in schema |

### Duplication

| # | What | Where |
|---|------|-------|
| D1 | `get_domains_to_block` query — identical SQL except `> 0` vs `< 1` | Domini bloccati, Domini rilasciati |
| D2 | `utils` JSObject — `validaDomini2()` and `estraiDomini()` duplicated across two pages | Richiesta Blocco, Richiesta Rilascio |
| D3 | `get_domains` and `get_release_domains` — near-identical queries on same page | Richiesta Rilascio |
| D4 | Domain validation: `validaDomini()` and `validaDomini2()` differ only in input source | Richiesta Blocco |

### Security Concerns

| # | Issue | Severity | Location |
|---|-------|----------|----------|
| S1 | `get_block_domains` uses **prepared statement = false** while binding widget values via `{{ }}` — potential SQL injection | High | Richiesta Blocco domini |
| S2 | All database access is direct from UI — no backend authorization layer | Medium | All pages |
| S3 | No input sanitization beyond regex format check on domain names | Medium | JSObjects |

### Incomplete / Broken Features

| # | Issue | Location |
|---|-------|----------|
| I1 | Inline editing save buttons (IconButton5, IconButton6, IconButton2, IconButton2Copy) have **no onClick handlers** — edits cannot be persisted | Richiesta Blocco, Richiesta Rilascio |
| I2 | No ability to add domains to an existing release request (no equivalent of `mdl_domain` modal) | Richiesta Rilascio |
| I3 | No delete functionality for blocks, releases, or individual domains | All CRUD pages |
| I4 | No edit functionality for block/release header fields (date, reference) beyond non-functional inline editing | All CRUD pages |

### Migration Blockers

| # | Blocker | Impact |
|---|---------|--------|
| M1 | N+1 sequential domain inserts in `inserisciBlocco()` and `togliBlocco()` | Must become a single transactional batch in backend |
| M2 | Block status is computed on-the-fly via aggregate UNION queries | Need to decide: computed view, materialized column, or real-time query in backend |
| M3 | Shared `anisetta` database contains unrelated tables — compliance tables have no schema isolation | Backend should use a dedicated schema or service boundary |

### Candidate Domain Entities

| Entity | Source Tables | Key Fields |
|--------|-------------|------------|
| **BlockRequest** | `dns_bl_block` | id, request_date, reference, method_id |
| **BlockDomain** | `dns_bl_block_domain` | id, domain, block_id |
| **ReleaseRequest** | `dns_bl_release` | id, request_date, reference |
| **ReleaseDomain** | `dns_bl_release_domain` | id, domain, release_id |
| **BlockMethod** | `dns_bl_method` | method_id, description |
| **DomainStatus** (computed) | aggregate of block/release domains | domain, block_count, release_count, is_blocked |

### Recommended Next Steps

1. **Create backend API layer** — Move all SQL behind Go endpoints with proper authorization, input validation, and batch operations
2. **Fix SQL injection** — `get_block_domains` must use prepared statements
3. **Consolidate domain logic** — Single shared validation and extraction utility
4. **Implement missing CRUD** — Delete, update operations for blocks/releases/domains
5. **Add pagination** — `get_all_domains` and status views will grow; add server-side pagination
6. **Decide on domain status computation** — Materialized view vs. on-the-fly aggregate
7. **Hand off to `appsmith-migration-spec`** — Use this audit as input for the Phase 2 product specification

---

## Definition of Done

- [x] Every page inventoried (6/6)
- [x] All datasources and queries cataloged (13 queries, 1 datasource)
- [x] Major hidden logic called out (5 business rules, widget bindings documented)
- [x] Findings classified into business logic / orchestration / presentation
- [x] Output is usable as direct input for migration PRD
- [x] Downstream Phase 2 can proceed without reopening raw Appsmith export

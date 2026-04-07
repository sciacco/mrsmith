# Compliance — Migration Spec Phase B: UX Pattern Map

**Source**: `apps/compliance/AUDIT.md` + Phase A decisions
**Date**: 2026-04-07
**Status**: Complete

---

## Decisions Log

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| D7 | Home page | **Remove** | Portal mrsmith handles app entry |
| D8 | Merge Blocchi/Rilasci views? | **No — keep separate** | Both include CRUD forms; merging would overload a single view against Stripe design principles |
| D9 | Merge Domini bloccati/rilasciati? | **Yes — unify with tabs** | Same entity, same columns, only filter differs. Stripe pattern: tabs within page, not separate pages. Enables cross-reference and cleaner nav. |
| D10 | Export formats | **CSV and XLSX**, all records (respecting active filter) | |
| D11 | Navigation pattern | **Same as budget app** (AppShell + TabNav from `@mrsmith/ui`) | Consistent portal UX |
| D11b | "Metodi" tab renamed | **"Provenienze"** — label in forms is "Origine", plural concept is "Provenienze" | Clearer business language |
| D11c | Provenienze deletion | **Soft delete** (not SQL DELETE) | Preserve referential integrity with historical block requests |

---

## Navigation Structure

Uses shared components from `@mrsmith/ui`: `AppShell` (sticky header, glassmorphism), `TabNav` (horizontal tabs with animated indicator), `UserMenu`.

| Tab | Route | View |
|-----|-------|------|
| Blocchi | `/blocks` | Master-detail: block requests + domains |
| Rilasci | `/releases` | Master-detail: release requests + domains |
| Stato domini | `/domains` | Unified view with sub-tabs: Bloccati / Rilasciati |
| Riepilogo | `/history` | Full history table with export |
| Provenienze | `/origins` | CRUD for block methods/origins |

Default landing: first tab (Blocchi).

---

## View Specifications

### View 1: Blocchi (`/blocks`)

| Aspect | Detail |
|--------|--------|
| **Removed from** | "Home" (eliminated) + "Richiesta Blocco domini" (renamed) |
| **User intent** | Create block requests, manage domains within each request, edit existing requests |
| **Interaction pattern** | Master-detail (two panels) + modals for creation |
| **UI sections** | |
| — Left panel (master) | Table of block requests: date, origin (method), reference. Row selection loads detail. |
| — Right panel (detail) | Table of domains for selected request. Inline-editable domain field. |
| — Modal: new request | Form: date (default today), reference, origin/method (dropdown, default "AGCOM"), domains (textarea, one FQDN per line) |
| — Modal: add domains | Form: domains textarea. Adds to currently selected request. |
| **Key actions** | Create request (with batch domain insert), add domains to existing request, edit request header (date, reference, method), edit domain name inline |
| **Data** | BlockRequest list (joined with method description, ordered by date desc), BlockDomain list (filtered by selected request) |
| **Changes from current** | Save buttons now functional (update operations). Batch insert replaces N+1 loop. Prepared statements fix SQL injection. |

### View 2: Rilasci (`/releases`)

| Aspect | Detail |
|--------|--------|
| **Removed from** | "Richiesta Rilascio domini" (renamed) |
| **User intent** | Create release requests, manage domains within each request, edit existing requests |
| **Interaction pattern** | Master-detail (two panels) + modals for creation |
| **UI sections** | |
| — Left panel (master) | Table of release requests: date, reference. Row selection loads detail. |
| — Right panel (detail) | Table of domains for selected request. Inline-editable domain field. |
| — Modal: new request | Form: date (default today), reference, domains (textarea, one FQDN per line) |
| — Modal: add domains | Form: domains textarea. Adds to currently selected release. **New capability** (absent in current app — finding I2). |
| **Key actions** | Create request (with batch domain insert), add domains to existing request, edit request header (date, reference), edit domain name inline |
| **Data** | ReleaseRequest list (ordered by date desc), ReleaseDomain list (filtered by selected request) |
| **Changes from current** | Add-domains modal added. Save buttons functional. Batch insert. No method field (correct). |

### View 3: Stato domini (`/domains`)

| Aspect | Detail |
|--------|--------|
| **Merged from** | "Domini bloccati" + "Domini rilasciati" |
| **User intent** | View current domain status — which domains are blocked, which are released |
| **Interaction pattern** | Read-only tabbed list with search and export |
| **UI sections** | |
| — Sub-tabs | "Bloccati" (default active) / "Rilasciati" |
| — Table | Columns: domain, block count, release count |
| — Search | Client-side text filter |
| — Export | CSV and XLSX, all records matching active tab + search filter |
| **Business rule** | Blocked: cumulative blocks > cumulative releases. Released: blocks <= releases. (BR1, confirmed) |
| **Data** | DomainStatus computed view, filtered by tab selection |
| **Changes from current** | Two pages merged into one. Export formats specified. Search filter persists across tab switches (or resets — implementation detail). |

### View 4: Riepilogo (`/history`)

| Aspect | Detail |
|--------|--------|
| **From** | "Riepilogo domini" (renamed) |
| **User intent** | Consult full chronological history of all block and release events. Export data. |
| **Interaction pattern** | Read-only list with search and export |
| **UI sections** | |
| — Table | Columns: domain, request date, reference, request type (block/release) |
| — Search | Client-side text filter |
| — Export | CSV and XLSX, all records matching search filter |
| **Data** | UNION ALL of block_domain+block and release_domain+release, ordered by domain |
| **Changes from current** | Export formats specified. Pagination to be determined (currently ~10K rows, no pagination). |

### View 5: Provenienze (`/origins`)

| Aspect | Detail |
|--------|--------|
| **From** | New view (data from `dns_bl_method`, previously read-only lookup) |
| **User intent** | Manage the list of block origins/methods (e.g., AGCOM) |
| **Interaction pattern** | Simple CRUD list |
| **UI sections** | |
| — Table | Columns: description (editable). Possibly method_id (read-only). |
| — Actions | Create new origin, edit description, soft-delete (hide from active use, preserve for historical references) |
| **Key constraint** | Soft delete only — origins referenced by existing block requests must not be hard-deleted. Soft-deleted origins should not appear in the "Origine" dropdown when creating new block requests, but should still display correctly in historical data. |
| **Data** | BlockMethod entity. Requires a new `deleted_at` or `is_active` column (must be nullable/defaulted for schema retrocompatibility). |

---

## Schema Impact: Soft Delete for BlockMethod

The `dns_bl_method` table needs a soft-delete column. To maintain Appsmith retrocompatibility:

- Add nullable column (e.g., `is_active BOOLEAN DEFAULT TRUE` or `deleted_at TIMESTAMP NULL`)
- Existing Appsmith queries (`get_methods`) do not filter by this column, so they will continue to return all methods including soft-deleted ones — acceptable during coexistence period
- New system filters active-only for dropdowns, shows all for historical display

---

## Open Items Carried Forward

| # | Item | From | Needed for |
|---|------|------|------------|
| O1 | Exact current values in `dns_bl_method` | Phase A | Phase E |
| O2 | Pagination strategy for Riepilogo and Stato domini (~10K+ rows) | Phase A | Phase D |
| O3 | Should search filter persist across sub-tabs in Stato domini? | Phase B | Implementation |

---

## Phase B Status: COMPLETE

All views specified, navigation defined, expert decisions recorded. Ready for Phase C: Logic Placement.

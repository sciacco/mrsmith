# Compliance — Migration Spec Phase A: Entity-Operation Model

**Source**: `apps/compliance/AUDIT.md`
**Date**: 2026-04-07
**Status**: Complete — all expert decisions recorded

---

## Decisions Log

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| D1 | Unify or separate Block/Release entities? | **Separate** | Database must remain retrocompatible with the Appsmith app currently in production. Schema unchanged. |
| D2 | Missing CRUD operations | **Add update** for requests and domains. **No delete** for now. **Add domains to existing release** (missing in current app). | Complete the incomplete functionality. |
| D3 | BlockMethod management | **Full CRUD** in the new system | Currently managed outside the app; new system must be self-sufficient. |
| D4 | Domain status computation | **Keep current rule** (blocks - releases > 0). Riepilogo view is useful and must support **data export**. | Rule confirmed as correct and complete for now. |
| D5 | Domain validation and uniqueness | One valid FQDN per line. No wildcards, no IPs. **No uniqueness constraint** across requests. | Each request is a faithful snapshot of a directive received from an authority — must preserve integrity. |
| D6 | Additional fields/features | **None for now** | No operator tracking, approval workflow, attachments, or audit trail required at this stage. |

---

## Constraint: Schema Retrocompatibility

The existing PostgreSQL schema in the `anisetta` database **must not be altered** in ways that break the Appsmith application. The new system reads and writes the same tables. This means:

- No table renames, no column renames, no type changes
- New columns (if any) must be nullable or have defaults
- No foreign key or constraint changes that would block Appsmith queries
- The new system must coexist with the Appsmith app during the transition period

---

## Entity Catalog

### Entity 1: BlockRequest

| Aspect | Detail |
|--------|--------|
| **Source table** | `dns_bl_block` (1,255 rows) |
| **Fields** | `id` (PK, serial), `request_date`, `reference`, `method_id` (FK → `dns_bl_method`) |
| **Relationships** | has many BlockDomain, belongs to BlockMethod |
| **Current operations** | Create, List (joined with method description, ordered by date desc) |
| **New operations** | **Update** (request_date, reference, method_id) |
| **Not implementing** | Delete |

### Entity 2: BlockDomain

| Aspect | Detail |
|--------|--------|
| **Source table** | `dns_bl_block_domain` (10,270 rows) |
| **Fields** | `id` (PK, serial), `domain`, `block_id` (FK → `dns_bl_block`) |
| **Relationships** | belongs to BlockRequest |
| **Current operations** | Create (single insert, called in loop), List (filtered by block_id) |
| **New operations** | **Update** (domain field). **Batch create** (replace N+1 loop with single transaction). |
| **Not implementing** | Delete |
| **Validation** | One valid FQDN per line. No wildcards, no IPs. No cross-request uniqueness constraint. |
| **Security fix** | `get_block_domains` must use prepared statements (currently vulnerable to SQL injection — finding S1) |

### Entity 3: ReleaseRequest

| Aspect | Detail |
|--------|--------|
| **Source table** | `dns_bl_release` (14 rows) |
| **Fields** | `id` (PK, serial), `request_date`, `reference` |
| **Relationships** | has many ReleaseDomain |
| **Current operations** | Create, List (ordered by date desc) |
| **New operations** | **Update** (request_date, reference) |
| **Not implementing** | Delete |
| **Notes** | No `method_id` — simpler than BlockRequest |

### Entity 4: ReleaseDomain

| Aspect | Detail |
|--------|--------|
| **Source table** | `dns_bl_release_domain` (62 rows) |
| **Fields** | `id` (PK, serial), `domain`, `release_id` (FK → `dns_bl_release`) |
| **Relationships** | belongs to ReleaseRequest |
| **Current operations** | Create (single insert, called in loop), List (filtered by release_id) |
| **New operations** | **Update** (domain field). **Batch create**. **Add domains to existing release** (new capability, absent in current app — finding I2). |
| **Not implementing** | Delete |
| **Validation** | Same as BlockDomain: one valid FQDN per line. |

### Entity 5: BlockMethod

| Aspect | Detail |
|--------|--------|
| **Source table** | `dns_bl_method` (~few rows) |
| **Fields** | `method_id` (PK), `description` |
| **Relationships** | referenced by BlockRequest |
| **Current operations** | List (populates dropdown) |
| **New operations** | **Full CRUD**: Create, Read, Update, Delete |
| **Notes** | Default value "AGCOM" (BR4). Exact current values TBD (need DB query). |

### Entity 6: DomainStatus (computed view, no source table)

| Aspect | Detail |
|--------|--------|
| **Source** | Aggregate UNION of block and release domain tables |
| **Computed fields** | `domain`, `blocchi` (block count), `rilasci` (release count), `is_blocked` (blocchi > rilasci) |
| **Operations** | List blocked domains, List released domains, List full history (all events) |
| **Business rule** | A domain is blocked when cumulative block count > cumulative release count (BR1) — confirmed by expert |
| **Export** | The history view (Riepilogo) must support data export (CSV or similar formats) |
| **Notes** | Currently no pagination (~10K rows). Method does not affect status calculation — only numeric count matters. |

---

## Operations Summary

| Entity | Create | Read/List | Update | Delete | Batch | Export |
|--------|--------|-----------|--------|--------|-------|--------|
| BlockRequest | Yes | Yes | **New** | No | — | — |
| BlockDomain | Yes | Yes | **New** | No | **New** (batch insert) | — |
| ReleaseRequest | Yes | Yes | **New** | No | — | — |
| ReleaseDomain | Yes | Yes | **New** | No | **New** (batch insert + add to existing) | — |
| BlockMethod | **New** | Yes | **New** | **New** | — | — |
| DomainStatus | — | Yes (3 views) | — | — | — | **Required** |

---

## Open Items Carried Forward

| # | Item | Needed for |
|---|------|------------|
| O1 | Exact current values in `dns_bl_method` table | Phase E (spec assembly) |
| O2 | Export format requirements for Riepilogo (CSV only? Excel? PDF?) | Phase B (UX) or Phase E |
| O3 | Pagination strategy for domain status views (~10K+ rows) | Phase D (data flow) |

---

## Phase A Status: COMPLETE

All entities identified, operations defined, expert decisions recorded. Ready for Phase B: UX Pattern Map.

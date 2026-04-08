# Phase D: Integration and Data Flow — Listini e Sconti

## 1. External Datasources and APIs

### 1.1 db-mistra (PostgreSQL)

**Purpose:** Product catalog, customer directory, credits ledger, discount groups.

**Current access:** Direct SQL from Appsmith UI → PostgreSQL. No middleware.

**New system:** Go backend endpoints. 13 queries → ~10 API endpoints.

| Entity | Endpoints | R/W |
|--------|----------|-----|
| Kit | `GET /kits`, `GET /kits/:id/products`, `GET /kits/:id/help-url`, `POST /kits/:id/pdf` | R (+PDF export) |
| Customer | `GET /customers`, `GET /customers/erp-linked` | R |
| CustomerGroup | `GET /customer-groups`, `GET /customers/:id/groups`, `PATCH /customers/:id/groups` | R/W |
| CustomerCredit | `GET /customers/:id/credit`, `GET /customers/:id/transactions`, `POST /customers/:id/transactions` | R/W |
| CustomPricing | `GET /customers/:id/pricing/timoo`, `PUT /customers/:id/pricing/timoo` | R/W |

**Data ownership:** Mistra owns all data. App reads catalog/customers, writes credits, associations, and Timoo pricing.

**Identity:** `customers.customer.id` = Alyante ERP ID. See `docs/IMPLEMENTATION-KNOWLEDGE.md`.

---

### 1.2 Grappa (MySQL)

**Purpose:** CloudStack infrastructure, IaaS pricing, account credits, rack energy discounts.

**Current access:** Direct SQL from Appsmith UI → MySQL. No middleware.

**New system:** Go backend endpoints. 8 queries → ~7 API endpoints.

| Entity | Endpoints | R/W |
|--------|----------|-----|
| Customer | `GET /grappa/customers` | R |
| IaaSPricing | `GET /grappa/customers/:id/iaas-pricing`, `POST /grappa/customers/:id/iaas-pricing` | R/W |
| IaaSAccount | `GET /grappa/iaas-accounts`, `PUT /grappa/iaas-accounts/:domain/credit` | R/W |
| Rack | `GET /grappa/customers/:id/racks`, `PUT /grappa/racks/:id/discount` | R/W |

**Data ownership:** Grappa owns all IaaS and infrastructure data.

**Identity:** `cli_fatturazione.id` = internal Grappa ID. Bridge to ERP: `cli_fatturazione.codice_aggancio_gest`.

**Hardcoded exclusions (coexistence):** code 385 (IaaS Prezzi), codes 385+485 (IaaS Credito).

---

### 1.3 HubSpot CRM

**Purpose:** Write-only audit trail. Creates notes and tasks when pricing/discounts/credits change. Never reads historical data.

**Current access:** Appsmith HS_utils module (v0.0.18) → HubSpot REST API v3. Credentials in Appsmith datasource config.

**New system:** Go backend `hubspot.Service` with:
- `GetCompanyByGrappaId(grappaId) → companyId` — lookup HubSpot company
- `AddAuditNote(companyId, htmlBody)` — create note
- `CreateTask(companyId, subject, assigneeEmail)` — create task

**API key:** Environment variable, not in code.

**Pages that trigger HubSpot:**

| Page | Trigger | HubSpot action |
|------|---------|---------------|
| IaaS Prezzi | Price save (if changed) | Note with HTML table of new values |
| IaaS Credito | Credit save (per row) | Note with old/new credit + CloudStack domain |
| Sconti energia | Discount save (batch) | Note with changed racks table + Task to reviewer |

**Not triggered:** Kit di vendita, Gruppi sconto, Gestione credito, Timoo.

---

### 1.4 Carbone PDF

**Purpose:** Generate branded PDF of kit product lists.

**Current access:** Appsmith carboneUtils module (v0.0.2). Template ID hardcoded: `d7c2d6...b657`.

**New system:** Either:
- **Option A:** Frontend calls Carbone API directly (current approach, simpler)
- **Option B:** Backend endpoint `POST /kits/:id/pdf` proxies to Carbone (centralizes credentials)

**Template management:** IDs in code for now. Portal admin module planned (see `docs/TODO.md`).

---

## 2. Cross-View User Journeys

**All pages are independent.** No inter-page navigation links exist. Each page is a standalone tool accessed via sidebar.

There are no multi-step workflows spanning multiple pages.

**Implicit data relationships (no navigation):**

| Page A | Page B | Relationship |
|--------|--------|-------------|
| Gruppi sconto (assign groups) | Kit di vendita (shows kit discounts per group) | Groups affect kit pricing |
| IaaS Prezzi (pricing) | IaaS Credito (credits) | Same customer in Grappa, different concerns |

**Recommendation:** Keep pages independent. If cross-page navigation is desired in future, implement via backend service layer (e.g. customer detail hub).

---

## 3. Hidden Triggers and Automation

### 3.1 On-Load Queries

| Page | Query | Populates |
|------|-------|-----------|
| Kit di vendita | `get_kit_list` | Kit table |
| Kit di vendita | `get_kit_help` | Help URL (defaults to -1) |
| Kit di vendita | `get_product_category` | **Unused** — remove |
| Kit di vendita | `json_kits` | **Unused** — remove |
| IaaS Prezzi | `get_customers` | Customer dropdown |
| IaaS Credito | `get_cdl_accounts` | Account table |
| Sconti energia | `get_customers` | Customer dropdown |
| Gruppi sconto | `get_customers` | Customer table |
| Gruppi sconto | `get_customer_groups` | Modal multi-select options |
| Gestione credito | `get_customers` | Customer dropdown |
| Timoo | `get_customers` | Customer dropdown |

**Migration:** On-load queries become React `useEffect` or TanStack Query on mount.

### 3.2 Cascading Triggers (User Events)

| Page | User action | Triggered queries |
|------|------------|-------------------|
| Kit di vendita | Row select in `tbl_kit` | `get_kit_products` + `get_kit_help` (sequential) |
| IaaS Prezzi | Customer select | `get_prezzi_per_cliente` + form reset |
| Sconti energia | Click "Cerca" | `get_racks` |
| Gruppi sconto | Row select in `tbl_customers` | `get_group_associations` → `get_kit_group` |
| Gruppi sconto | Row select in `tbl_groups` | `get_kit_group` |
| Gestione credito | Click "Aggiorna" | `get_customer_credit` + `get_customer_transactions` |
| Timoo | Customer select | `get_prezzi_cliente` |

### 3.3 Side-Effects on Save

| Page | DB mutation | Side-effect |
|------|-----------|-------------|
| IaaS Prezzi | UPSERT pricing | HubSpot note (if values changed) |
| IaaS Credito | UPDATE credit (per row) | HubSpot note (old/new per row) |
| Sconti energia | UPDATE discount (per row) | HubSpot note (batch table) + HubSpot task (to reviewer) |
| Gestione credito | INSERT transaction | Operator email captured (`appsmith.user.email` → Keycloak) |
| Timoo | INSERT pricing (buggy) | None |
| Gruppi sconto | DELETE+INSERT associations | None |

### 3.4 No Background Automation

No timers, polling, or scheduled jobs in the Appsmith export.

`customer_credits` balance is updated by external jobs outside this app.

---

## 4. Data Ownership Boundaries

### Read-Only (app never writes)

| System | Entity | Owner |
|--------|--------|-------|
| Mistra | Kit, Product, ProductCategory, KitHelp | Product management |
| Mistra | Customer | ERP import |
| Grappa | Customer (cli_fatturazione) | Grappa admin |
| Grappa | IaaS Accounts (cdl_accounts) | CloudStack integration |
| Grappa | Datacenter, DC Build | Infrastructure management |
| Mistra | customer_credits (balance) | External batch jobs |

### Read-Write (app owns mutations)

| System | Entity | Operations |
|--------|--------|-----------|
| Mistra | group_association | INSERT, DELETE (sync) |
| Mistra | customer_credit_transaction | INSERT only (immutable) |
| Mistra | custom_items (Timoo pricing) | UPSERT |
| Grappa | cdl_prezzo_risorse_iaas | UPSERT |
| Grappa | cdl_accounts.credito | UPDATE |
| Grappa | racks.sconto | UPDATE |
| HubSpot | Notes | CREATE only |
| HubSpot | Tasks | CREATE only |

### Cross-Database: No Foreign Keys

Mistra and Grappa have separate customer ID spaces. No cross-DB joins needed — each page works within one database. Bridge: `cli_fatturazione.codice_aggancio_gest` = ERP ID = `customers.customer.id`.

---

## 5. Integrations the Export Cannot Reveal

### 5.1 HubSpot Company ID Mapping

The `HS_utils1.CompanyByGrappaId(grappaId)` method is inside an Appsmith module not visible in the export. Unknown: does it use a lookup table, a custom HubSpot property, or a search API call?

### 5.2 Carbone Templates

Only one template ID visible (`d7c2d6...b657` for Kit PDF). Unknown: are there other Carbone templates used elsewhere in the portal?

### 5.3 External Credit Balance Updates

`customers.customer_credits` is updated by external jobs. Unknown: what system runs these jobs, how often, and what triggers them.

### 5.4 Kit/Product Pricing Versioning

Kit product prices are read-only. Unknown: are prices versioned/effective-dated? Do PDFs show current or point-in-time prices?

---

## 6. Coexistence Strategy

During transition, both Appsmith and the new Go app access the same databases.

**Risk:** Concurrent writes to same data (e.g. IaaS pricing) from both systems → last-write-wins.

**Mitigation:** During coexistence, route specific pages to the new app incrementally. Don't run both systems for the same page simultaneously.

---

## Phase D Questions for Domain Expert

### ~~D1.~~ RESOLVED. Two-step cross-database lookup:
1. Grappa → ERP ID: `SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = :grappa_id` (Grappa MySQL)
2. ERP ID → HubSpot ID: `SELECT id FROM loader.hubs_company WHERE numero_azienda = :erp_id::varchar` (Mistra PG)

Mapping table: `loader.hubs_company` in Mistra contains HubSpot company IDs indexed by ERP code. Backend must query both databases sequentially.
### ~~D2.~~ RESOLVED. Safe to read without locking. External process details not needed for this app — treat `customer_credits` as read-only eventual-consistency data.
### ~~D3.~~ RESOLVED. Prices are not versioned currently — always show current prices. Price versioning tracked in `docs/TODO.md` for future implementation.
### ~~D4.~~ RESOLVED. Yes, other apps use Carbone. Not relevant for this app now — centralized template management will be addressed after all apps are migrated (see `docs/TODO.md`).
### ~~D5.~~ RESOLVED. Failures tolerated and non-blocking. Async HubSpot queue planned as cross-app infrastructure (see `docs/TODO.md`).
### ~~D6.~~ RESOLVED. Develop all pages together, deploy as a complete app. No incremental page-by-page cutover.
### ~~D7.~~ RESOLVED. Assume 1:1 mapping (one ERP ID → one Grappa cli_fatturazione record).

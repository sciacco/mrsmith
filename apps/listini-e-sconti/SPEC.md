# Application Specification — Listini e Sconti

## Summary

- **Application name:** listini-e-sconti
- **Audit source:** Appsmith Git export (`listini-e-sconti-main.zip`)
- **Spec status:** Complete — all questions resolved
- **Datasources:** db-mistra (PostgreSQL), Grappa (MySQL), HubSpot CRM (REST), Carbone PDF (REST)
- **Pages:** 7 functional (Home removed)
- **Deployment:** All pages developed and deployed together as a complete app
- **Coexistence:** Runs alongside Appsmith during transition; both access same databases

---

## Entity Catalog

### Entity: Customer (cross-database, read-only)

- **Purpose:** Lookup entity for all pages. Never written by this app.
- **Operations:** `list` (3 separate endpoints by datasource)
- **Identity:** Mistra `customers.customer.id` = Alyante ERP ID. Grappa `cli_fatturazione.id` = internal Grappa ID. Bridge: `cli_fatturazione.codice_aggancio_gest` = ERP ID. Always 1:1 mapping. See `docs/IMPLEMENTATION-KNOWLEDGE.md`.
- **Fields:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| id | customers.customer | int | = ERP ID (Mistra) |
| name | customers.customer | string | Display label (Mistra pages) |
| id | cli_fatturazione | int | Internal Grappa ID |
| intestazione | cli_fatturazione | string | Display label (Grappa pages) |
| stato | cli_fatturazione | string | Filter: 'attivo' |
| codice_aggancio_gest | cli_fatturazione | int | = ERP ID; exclusion filter |
| fatgamma | erp_clienti_provenienza | int | Eligibility: > 0 |

- **Endpoints:**

| Endpoint | DB | Filter | Used by |
|----------|----|--------|---------|
| `GET /api/v1/customers` | Mistra | All, ORDER BY name | Gestione credito, Timoo |
| `GET /api/v1/customers/erp-linked` | Mistra | JOIN erp_clienti_provenienza WHERE fatgamma > 0 | Gruppi sconto |
| `GET /api/v1/grappa/customers` | Grappa | stato='attivo', codice_aggancio_gest > 0, exclusions applied per caller | IaaS Prezzi, IaaS Credito, Sconti energia |

- **Exclusions (hardcoded for coexistence):** code 385 (IaaS Prezzi, IaaS Credito), code 485 (IaaS Credito only)

---

### Entity: Kit (read-only)

- **Purpose:** Product kit catalog. Browsing + PDF export for sales team.
- **Operations:** `list`, `getProducts(kitId)`, `getHelpUrl(kitId)`, `exportPDF(kitId)`
- **Fields:**

| Field | Type | Notes |
|-------|------|-------|
| id | int | PK |
| internal_name | string | Display name |
| billing_period | string | Billing cycle |
| initial_subscription_months | int | Initial contract |
| next_subscription_months | int | Renewal period |
| activation_time_days | int | Activation SLA |
| category_id | int | FK → product_category |
| category_name | string | Joined from product_category |
| category_color | string | Badge color |
| is_main_prd_sellable | bool | Controls main product visibility |
| sconto_massimo | decimal | Max discount % |
| variable_billing | bool | Displayed as SI/NO |
| h24_assurance | bool | Displayed as SI/NO |
| sla_resolution_hours | int | SLA hours |
| notes | text | Free-text notes |

- **Kit Products (sub-entity):**

| Field | Type | Notes |
|-------|------|-------|
| group_name | string | Product grouping |
| internal_name | string | Product name |
| nrc | decimal (EUR) | Non-recurring charge |
| mrc | decimal (EUR) | Monthly recurring charge |
| minimum | int | Min quantity |
| maximum | int | Max quantity |
| required | bool | Mandatory product |
| position | int | Sort order |
| product_code | string | SKU |

- **Constraints:**
  - Only `is_active = true AND ecommerce = false` shown
  - Main product included when `is_main_prd_sellable = true` (forced required=true, position=0)
  - Prices not versioned — always current (versioning tracked in `docs/TODO.md`)
  - No CRUD — read-only in this app
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/kits` | GET | Active non-ecommerce, sorted by category + name |
| `GET /api/v1/kits/:id/products` | GET | Component products with conditional main product |
| `GET /api/v1/kits/:id/help-url` | GET | Optional help URL |
| `POST /api/v1/kits/:id/pdf` | POST | Generate PDF via Carbone (template ID in code) |

- **DB tables:** `products.kit`, `products.kit_product`, `products.product`, `products.product_category`, `products.kit_help`

---

### Entity: CustomerGroup

- **Purpose:** Manage customer-to-discount-group associations. Group CRUD is in kit-products app.
- **Operations:** `listGroups`, `getAssociations(customerId)`, `syncAssociations(customerId, groupIds[])`
- **Fields:**

| Field | Type | Notes |
|-------|------|-------|
| id | int | PK (customer_group) |
| name | string | Group display name |
| customer_id | int | FK → customer (in group_association) |
| group_id | int | FK → customer_group (in group_association) |

- **Constraints:**
  - No rules on which customers can join which groups
  - ON CONFLICT DO NOTHING prevents duplicates
  - Sync is diff-based: frontend sends desired state, backend computes adds/removes
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/customer-groups` | GET | All groups, ordered by name |
| `GET /api/v1/customers/:id/groups` | GET | Groups for a customer |
| `PATCH /api/v1/customers/:id/groups` | PATCH | Body: `{groupIds: [...]}`. Transactional diff sync. |

- **DB tables:** `customers.customer_group`, `customers.group_association`

---

### Entity: KitGroupDiscount (read-only)

- **Purpose:** View kit discounts per group.
- **Operations:** `listByGroup(groupId)`
- **Fields:** kit_name, kit_id, group_id, discount_mrc, discount_nrc
- **Constraints:** Only active kits (`is_active = true`). No CRUD in this app.
- **Endpoint:** `GET /api/v1/customer-groups/:id/kit-discounts`
- **DB tables:** `products.kit_customer_group`, `products.kit`

---

### Entity: IaaSPricing

- **Purpose:** Per-customer daily pricing for CloudStack resources.
- **Operations:** `getByCustomer(customerId)`, `upsert(customerId, prices)`
- **Fields:**

| Field | Type | Min | Max | Required |
|-------|------|-----|-----|----------|
| charge_cpu | decimal | 0.05 | 0.1 | Yes |
| charge_ram_kvm | decimal | 0.05 | 0.2 | Yes |
| charge_ram_vmware | decimal | 0.18 | 0.3 | Yes |
| charge_pstor | decimal | 0.0005 | 0.002 | Yes |
| charge_sstor | decimal | 0.0005 | 0.002 | Yes |
| charge_ip | decimal | 0.02 | — | Yes |
| charge_prefix24 | decimal | — | — | No (hidden from UI) |

- **Constraints:**
  - Min/max are **hard business constraints** — enforced backend-side
  - Price fallback: customer-specific overrides default (id_anagrafica IS NULL) via UNION + LIMIT 1
  - UPSERT via ON DUPLICATE KEY UPDATE
  - Prices take effect **immediately** (no effective-date logic)
- **Side-effect:** HubSpot audit note on change (non-blocking; failures tolerated)
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/grappa/customers/:id/iaas-pricing` | GET | Returns customer-specific or default |
| `POST /api/v1/grappa/customers/:id/iaas-pricing` | POST | UPSERT + async HubSpot note if changed |

- **DB table:** `grappa.cdl_prezzo_risorse_iaas`

---

### Entity: IaaSAccount

- **Purpose:** CloudStack account credit allocation.
- **Operations:** `list`, `updateCredit(domainuuid, idCliFatturazione, credito)`
- **Fields:**

| Field | Type | Notes |
|-------|------|-------|
| intestazione | string | Company name (joined) |
| credito | decimal | Editable credit |
| cloudstack_domain | string (UUID) | CloudStack domain |
| id_cli_fatturazione | int | FK → cli_fatturazione |
| abbreviazione | string | Short name |
| serialnumber | string | Serial |
| data_attivazione | date | Activation date |
| infrastructure_platform | string | Controls editability |

- **Constraints:**
  - Credit editable only when `infrastructure_platform == 'cloudstack'` (enforced backend)
  - Filters: attivo=1, fatturazione=1, codice_aggancio_gest NOT IN (385, 485)
  - Composite key for update: (domainuuid, id_cli_fatturazione)
- **Side-effect:** HubSpot audit note with old/new credit values (non-blocking)
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/grappa/iaas-accounts` | GET | All active billing accounts |
| `PATCH /api/v1/grappa/iaas-accounts/credits` | PATCH | Batch update; body: array of {domainuuid, id_cli_fatturazione, credito} |

- **DB tables:** `grappa.cdl_accounts`, `grappa.cli_fatturazione`, `grappa.cdl_services`

---

### Entity: Rack (Energy Discount)

- **Purpose:** Datacenter rack energy discount management.
- **Operations:** `listByCustomer(customerId)`, `updateDiscount(idRack, sconto)`
- **Fields:**

| Field | Type | Notes |
|-------|------|-------|
| id_rack | int | PK |
| name | string | Rack name |
| building | string | From dc_build (joined) |
| room | string | From datacenter (joined) |
| floor | string | Floor |
| island | string | Island |
| type | string | Rack type |
| sconto | decimal | Discount %, 0–20 range |

- **Constraints:**
  - Discount range: 0–20% (hardcoded global, enforced backend)
  - Only active racks (stato='attivo')
  - Only customers with active rack sockets in dropdown (no sockets = no consumption)
- **Side-effects:**
  - HubSpot audit note with HTML table of changes (non-blocking)
  - HubSpot task to `eva.grimaldi@cdlan.it` (hardcoded for now; configurable in future — see `docs/TODO.md`)
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/grappa/customers/:id/racks` | GET | Racks with location details |
| `PATCH /api/v1/grappa/racks/discounts` | PATCH | Batch update; body: array of {id_rack, sconto} |

- **DB tables:** `grappa.racks`, `grappa.datacenter`, `grappa.dc_build`, `grappa.rack_sockets`

---

### Entity: CustomerCredit

- **Purpose:** Customer credit balance and immutable transaction ledger.
- **Operations:** `getBalance(customerId)`, `listTransactions(customerId)`, `addTransaction(customerId, ...)`
- **Fields (transaction):**

| Field | Type | Constraints |
|-------|------|------------|
| customer_id | int | FK → customer |
| amount | decimal | 0–10000, required |
| operation_sign | string | '+' or '-' |
| description | string | Required, max 255 chars |
| operated_by | string (email) | From Keycloak JWT (not appsmith.user.email) |
| transaction_date | timestamp | Auto-generated |

- **Constraints:**
  - **Immutable ledger** — INSERT only, no UPDATE/DELETE. Corrections via storno (opposite-sign). No link between storno and original.
  - `customer_credits` balance is read-only (updated by external jobs, safe to read without locking)
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/customers/:id/credit` | GET | Current balance |
| `GET /api/v1/customers/:id/transactions` | GET | History, newest first |
| `POST /api/v1/customers/:id/transactions` | POST | Insert transaction; captures Keycloak email |

- **DB tables:** `customers.customer_credits`, `customers.customer_credit_transaction`

---

### Entity: CustomPricing (Timoo)

- **Purpose:** Per-customer pricing for Timoo indirect (reseller) service.
- **Operations:** `getByCustomer(customerId)`, `upsert(customerId, prices)`
- **Fields:**

| Field | Type | Notes |
|-------|------|-------|
| key_label | string | Discriminator: 'timoo_indiretta' |
| customer_id | int | FK → customer; -1 = defaults |
| prices | JSON | `{user_month: decimal, se_month: decimal}` |

- **Defaults:** user_month = 0.78, se_month = 0.3
- **Constraints:**
  - No min/max validation on prices
  - Price fallback: customer-specific → default (customer_id = -1)
  - Must use **UPSERT** (fixes Appsmith bug: INSERT created duplicates)
  - No HubSpot audit (intentional)
- **Bugs fixed in migration:**
  - Read query parameterized (was hardcoded customer_id=110)
  - Write changed from INSERT to UPSERT
- **Endpoints:**

| Endpoint | Method | Notes |
|----------|--------|-------|
| `GET /api/v1/customers/:id/pricing/timoo` | GET | Customer-specific or default |
| `PUT /api/v1/customers/:id/pricing/timoo` | PUT | Idempotent UPSERT |

- **DB table:** `products.custom_items`

---

## View Specifications

### Navigation

Grouped horizontal tabs with dropdown menus (extends `TabNav` component). Groups organized by business function.

| Group | Pages | Behavior |
|-------|-------|----------|
| **Catalogo** | Kit di vendita | Single page: hover shows dropdown, click navigates directly |
| **Prezzi** | IaaS Prezzi risorse, Timoo Prezzi Partner | Dropdown on hover |
| **Sconti** | Gruppi sconto, Sconti Energia | Dropdown on hover |
| **Crediti** | Crediti Omaggio IaaS, Gestione crediti | Dropdown on hover |

**Implementation:** New `TabNavGroup` variant in `packages/ui/`. Reuses dropdown animation (ease-spring). Active state highlights both group and page. Mobile: hamburger + expandable sections.

---

### View: Kit di vendita

- **User intent:** Sales team browses kit catalog and views detailed kit data sheets.
- **Interaction pattern:** Master-detail read-only with PDF export.
- **Layout:** Redesigned as **kit card view** mirroring the printed PDF (reference: `artifacts/kit Unbreakable CORE.pdf`).

| Section | Content |
|---------|---------|
| Kit list (left, ~250px) | Filterable/searchable, grouped by category with color badges |
| Kit header (right, top) | Category label + kit name (styled like PDF header) |
| Metadata block | 2-column key-value grid: durata, rinnovi, attivazione, fatturazione, sconto max, fatt. variabile (SI/NO), H24 (SI/NO), SLA ore |
| Notes | Free-text block (if present) |
| Product table | Grouped by group_name. Columns: Nome interno, NRC (EUR), MRC (EUR). Required products marked. |
| Actions | "Genera PDF" (disabled if no kit selected) + "Supporto" (visible if help URL exists) |
| Footer | "Tutti i prezzi presenti sono IVA esclusa" |

- **Data loading:** On page load: kit list. On kit select: products + help URL.
- **PDF export:** Single kit at a time. Bulk export tracked in `docs/TODO.md`.

---

### View: IaaS Prezzi risorse

- **User intent:** Set per-customer daily IaaS resource pricing.
- **Interaction pattern:** Customer dropdown → pricing form with min/max validation.
- **Data loading:** On page load: customer list only. On customer select: load prices (or defaults), reset form.
- **Save:** UPSERT prices → async HubSpot audit note (if values changed) → toast success.
- **Validation:** Min/max enforced both frontend (UX) and backend (business constraint).

---

### View: IaaS Credito omaggio

- **User intent:** Allocate credit to CloudStack accounts.
- **Interaction pattern:** Inline-edit table + batch save.
- **Data loading:** On page load: all active accounts.
- **Editability:** `credito` column editable only for CloudStack rows. Non-CloudStack rows shown with **reduced opacity** (muted style).
- **Save:** Batch update → async HubSpot note per changed row → toast success/error. Simple feedback (no per-row progress).

---

### View: Sconti variabile energia

- **User intent:** Manage rack energy discount percentages.
- **Interaction pattern:** Customer dropdown → inline-edit table + batch save.
- **Data loading:** On page load: customer list only. **No query until customer selected** (auto-load racks on select; "Cerca" button removed).
- **Save:** Batch update (0–20% validated backend) → async HubSpot note + task → toast.

---

### View: Gruppi di sconto x clienti

- **User intent:** Manage customer-to-discount-group associations.
- **Interaction pattern:** Master-detail with modal many-to-many editor.
- **Layout:** Customer table (left) → group associations (middle) → kit discounts per group (right, read-only). "Associa" button opens modal with multi-select (add tooltip for discoverability).
- **Save:** Frontend sends desired group IDs → backend diffs and syncs transactionally.

---

### View: Gestione credito cliente

- **User intent:** View credit balance/transactions and add new entries.
- **Interaction pattern:** Customer dropdown → read-only ledger + modal form for new transaction.
- **Data loading:** On page load: customer list only. **No query until customer selected** (auto-refresh on select; "Aggiorna" button removed).
- **Modal form:** Amount (0–10000), operation (Accredito +/Debito -), description (required, max 255). Operator captured from Keycloak JWT.
- **Immutable ledger:** No edit/delete. Corrections via storno.

---

### View: Timoo prezzi indiretta

- **User intent:** Set per-customer monthly Timoo pricing.
- **Interaction pattern:** Customer dropdown → 2-field form (user_month, se_month).
- **Data loading:** On page load: customer list only. On customer select: load prices (or defaults).
- **Save:** Idempotent UPSERT. No min/max validation. No HubSpot audit.
- **Bugs fixed:** Read query parameterized (was hardcoded). Write changed to UPSERT (was INSERT).

---

## Logic Allocation

### Backend responsibilities

| Responsibility | Details |
|---------------|---------|
| All database queries | No direct SQL from frontend |
| Business rule validation | IaaS price ranges, discount 0–20%, credit amount 0–10000, description length, platform check |
| HubSpot audit trail | Diff detection, note creation, task creation — async, non-blocking |
| HubSpot company lookup | Two-step: Grappa ID → ERP ID → HubSpot ID (via `loader.hubs_company`) |
| Batch operations | Transactional updates for credits and discounts |
| Group sync | Diff-based INSERT/DELETE in single transaction |
| Price fallback | Customer-specific → default (UNION + LIMIT 1 pattern) |
| User identity | Extract email from Keycloak JWT for audit fields |
| Carbone PDF proxy | Optional: proxy PDF generation to centralize credentials |

### Frontend responsibilities

| Responsibility | Details |
|---------------|---------|
| Navigation | Grouped TabNav with dropdowns |
| Customer selector | 3 variants, reusable component |
| Kit card view | Master-detail with metadata block + product table |
| Inline-edit tables | Dirty-row tracking, save button enable/disable |
| Modal forms | Transaction entry, group associations |
| Form validation (UX) | Mirror backend constraints for instant feedback |
| PDF generation trigger | Call backend endpoint, handle download |
| Boolean display | SI/NO with semantic coloring |
| Presentation logic | Button visibility/disabled states, row opacity for non-editable rows |
| Toast feedback | Spinner during save → success/error toast |

### Shared validation (frontend + backend)

| Rule | Frontend (UX) | Backend (enforced) |
|------|--------------|-------------------|
| IaaS price ranges | Input min/max | Reject out-of-range |
| Discount 0–20% | Input min/max | Reject out-of-range |
| Credit amount 0–10000 | Input min/max | Reject out-of-range |
| Description max 255 | Input maxlength | Reject overlength |
| Customer required for writes | Button disabled | Reject customer_id <= 0 |

---

## Integrations and Data Flow

### External systems

| System | Type | Purpose | Access pattern |
|--------|------|---------|---------------|
| **db-mistra** | PostgreSQL | Kit catalog, customers, groups, credits, Timoo pricing | Go backend → SQL |
| **Grappa** | MySQL | IaaS pricing, accounts, racks, Grappa customers | Go backend → SQL |
| **HubSpot CRM** | REST API v3 | Write-only audit trail (notes + tasks) | Go backend → async HTTP; non-blocking |
| **Carbone** | REST API | PDF generation for kit data sheets | Frontend or backend → HTTP |

### HubSpot company lookup (cross-database)

```
Grappa customer ID
    → SELECT codice_aggancio_gest FROM cli_fatturazione WHERE id = :grappa_id  (Grappa MySQL)
    → SELECT id FROM loader.hubs_company WHERE numero_azienda = :erp_id::varchar  (Mistra PG)
    → HubSpot company ID
```

### Side-effects by page

| Page | DB mutation | HubSpot | Notes |
|------|-----------|---------|-------|
| Kit di vendita | None (read-only) | None | PDF export only |
| IaaS Prezzi | UPSERT pricing | Note (if changed) | Async, non-blocking |
| IaaS Credito | UPDATE credit (batch) | Note per row | Async, non-blocking |
| Sconti energia | UPDATE discount (batch) | Note + Task | Task to eva.grimaldi@cdlan.it |
| Gruppi sconto | DELETE + INSERT assoc. | None | Transactional diff |
| Gestione credito | INSERT transaction | None | Keycloak email in operated_by |
| Timoo | UPSERT pricing | None | Intentional: no audit |

### Data ownership

- **Read-only from external:** Kit, Product, Customer (both DBs), IaaS Accounts (structure), customer_credits (balance)
- **Write:** group_association, credit_transaction, custom_items, cdl_prezzo_risorse_iaas, cdl_accounts.credito, racks.sconto, HubSpot notes/tasks

---

## API Contract Summary

### Mistra (PostgreSQL) endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/customers` | GET | All Mistra customers |
| `/api/v1/customers/erp-linked` | GET | Customers with fatgamma > 0 |
| `/api/v1/kits` | GET | Active non-ecommerce kits |
| `/api/v1/kits/:id/products` | GET | Kit component products |
| `/api/v1/kits/:id/help-url` | GET | Optional help URL |
| `/api/v1/kits/:id/pdf` | POST | Generate PDF via Carbone |
| `/api/v1/customer-groups` | GET | All discount groups |
| `/api/v1/customer-groups/:id/kit-discounts` | GET | Kit discounts for group |
| `/api/v1/customers/:id/groups` | GET | Customer's group associations |
| `/api/v1/customers/:id/groups` | PATCH | Sync group associations |
| `/api/v1/customers/:id/credit` | GET | Credit balance |
| `/api/v1/customers/:id/transactions` | GET | Transaction history |
| `/api/v1/customers/:id/transactions` | POST | New transaction (immutable) |
| `/api/v1/customers/:id/pricing/timoo` | GET | Timoo pricing (with fallback) |
| `/api/v1/customers/:id/pricing/timoo` | PUT | UPSERT Timoo pricing |

### Grappa (MySQL) endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/v1/grappa/customers` | GET | Active billing customers (with exclusions) |
| `/api/v1/grappa/customers/:id/iaas-pricing` | GET | IaaS pricing (with fallback) |
| `/api/v1/grappa/customers/:id/iaas-pricing` | POST | UPSERT pricing + HubSpot note |
| `/api/v1/grappa/iaas-accounts` | GET | All active accounts |
| `/api/v1/grappa/iaas-accounts/credits` | PATCH | Batch credit update + HubSpot notes |
| `/api/v1/grappa/customers/:id/racks` | GET | Racks with location |
| `/api/v1/grappa/racks/discounts` | PATCH | Batch discount update + HubSpot note + task |

---

## Constraints and Non-Functional Requirements

### Security
- All database access through backend Go API (no direct SQL from frontend)
- HubSpot API key in environment variable, never in frontend code
- User identity from Keycloak JWT (replaces `appsmith.user.email`)
- Keycloak role: `app_listini_access` for app-level access

### Coexistence
- Both Appsmith and new app access same databases during transition
- Develop all pages together, deploy as complete app (no incremental cutover)
- Customer exclusion codes (385, 485) hardcoded to match Appsmith behavior

### Performance
- Customer lists and kit catalog loaded on page mount
- Dependent data loaded on user selection (no wasted queries)
- HubSpot calls async and non-blocking (failures logged, not blocking save)
- `customer_credits` safe to read without locking (eventual consistency)

---

## Bugs Fixed in Migration

| # | Severity | Original bug | Fix |
|---|----------|-------------|-----|
| 1 | Critical | Timoo read query hardcodes `customer_id = 110` | Parameterize by selected customer |
| 2 | Critical | Timoo INSERT creates duplicates | Change to UPSERT |
| 3 | Medium | IaaS Credito uses bitwise `&` instead of `&&` | Fixed naturally by SQL `AND` in backend |
| 4 | Low | Unused queries loaded on page init (Kit) | Removed: `get_product_category`, `json_kits` |
| 5 | Low | Legacy REST query in Sconti energia | Removed: `hs_create_note_remove` |

---

## Deferred Decisions (tracked in `docs/TODO.md`)

| Item | Description |
|------|------------|
| Bulk Kit PDF export | Single kit only for now; bulk export for future |
| Kit price versioning | Prices not versioned; effective-dating for future |
| Discount approval workflow | No approval for now; threshold-based approval for future |
| Configurable HubSpot task assignee | Hardcoded email for now; configurable for future |
| Carbone template management | IDs in code for now; portal admin module for future |
| Async HubSpot queue | Fire-and-forget for now; shared queue with retry/expiry for future |

---

## Acceptance Notes

### What the audit proved directly
- 9 domain entities across 2 databases with clear CRUD boundaries
- 25+ SQL queries mapping to ~22 backend API endpoints
- 3 pages with HubSpot audit integration (same pattern, different data)
- 2 critical bugs in Timoo page (hardcoded ID, INSERT without UPSERT)
- 1 latent bug in IaaS Credito (bitwise operator)
- All pages are independent (no cross-page navigation)

### What the expert confirmed
- Customer ID mapping: Mistra = ERP ID, Grappa = internal ID, bridge via codice_aggancio_gest
- IaaS price ranges are hard business constraints (not just UI)
- Credit transactions are intentionally immutable (ledger pattern)
- No HubSpot audit for Timoo (intentional)
- HubSpot failures tolerated and non-blocking
- Navigation grouped by business function: Catalogo, Prezzi, Sconti, Crediti
- Kit page redesigned as digital data sheet (mirrors PDF layout)
- No query on page load for Sconti/Credito pages (load on customer select)
- HubSpot lookup: Grappa ID → ERP ID (cli_fatturazione) → HubSpot ID (loader.hubs_company)
- ERP ID → Grappa is always 1:1
- Deploy all pages together as complete app

### What still needs validation
- Nothing — all questions resolved

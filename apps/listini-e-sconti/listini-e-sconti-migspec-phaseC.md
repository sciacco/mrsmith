# Phase C: Logic Placement — Listini e Sconti

## JSObject Methods — Classification & Placement

### 1. carboneIO.generaPDF() — Kit di vendita

| Aspect | Value |
|--------|-------|
| **Current behavior** | Extracts selected kit + products, converts booleans to "SI"/"NO", calls Carbone API, opens PDF download |
| **Classification** | Orchestration + Presentation |
| **Placement** | **Frontend** — operates on already-loaded data, triggers client-side download |
| **Action** | Port as-is. Keep template ID in code (Phase A decision). Replace Appsmith `navigateTo` with browser download. |

---

### 2. utils.aggiungiNotaSuHubspot() — IaaS Prezzi risorse

| Aspect | Value |
|--------|-------|
| **Current behavior** | Diffs form data vs DB values. If changed: builds HTML table, looks up HubSpot company by Grappa ID, creates audit note |
| **Classification** | Orchestration |
| **Placement** | **Backend** — HubSpot credentials must not be in frontend; diff logic tied to business rules; audit trail is critical |
| **Action** | Revise. Backend endpoint handles: validate → save DB → diff old/new → create HubSpot note async. Frontend only calls one endpoint. |

---

### 3. utils.saveCrediti() — IaaS Credito omaggio

| Aspect | Value |
|--------|-------|
| **Current behavior** | Iterates updated rows, updates credit per row, logs old/new to HubSpot. **Bug:** bitwise `&` instead of `&&` |
| **Classification** | Orchestration |
| **Placement** | **Backend** — batch update must be transactional; bug fixed naturally with SQL `AND` |
| **Action** | Revise. Backend batch endpoint: receives array of `{domainuuid, id_cli_fatturazione, credito}`, updates in transaction, creates HubSpot notes async. |

---

### 4. utils.saveSconti() — Sconti variabile energia

| Aspect | Value |
|--------|-------|
| **Current behavior** | Iterates updated rows, updates discount per row, builds HTML change table, creates HubSpot note + task to eva.grimaldi@cdlan.it |
| **Classification** | Orchestration |
| **Placement** | **Backend** — batch update + multi-step side effects (note + task) need transactional handling |
| **Action** | Revise. Backend batch endpoint: validate 0–20% range, update in transaction, create HubSpot note + task. Reviewer email from config. |

---

### 5. utils.salvaGruppi() — Gruppi di sconto x clienti

| Aspect | Value |
|--------|-------|
| **Current behavior** | Diffs selected groups vs existing associations. Deletes removed, inserts new (ON CONFLICT DO NOTHING). Row-by-row loops. |
| **Classification** | Domain logic |
| **Placement** | **Backend** — transactional diff-based sync; frontend sends desired state, backend handles consistency |
| **Action** | Revise. Backend endpoint: `PATCH /customers/:id/groups` with body `{groupIds: [...]}`. Backend computes diff, runs DELETE+INSERT in single transaction. |

---

### 6. utils.salva_form() — Timoo prezzi indiretta

| Aspect | Value |
|--------|-------|
| **Current behavior** | Guards `selectedOptionValue > 0`, runs INSERT. **Bug:** read query hardcodes customer_id=110; INSERT without UPSERT creates duplicates |
| **Classification** | Domain logic |
| **Placement** | **Backend** — must fix both bugs; UPSERT semantics belong server-side |
| **Action** | Revise (critical). Backend endpoint: `PUT /customers/:id/pricing/timoo` — idempotent UPSERT. Parameterize read query. |

---

## Inline Expressions — Classification

### Presentation (stay in frontend)

| Expression | Page | Widget | Purpose |
|------------|------|--------|---------|
| `isDisabled: tbl_kit.selectedRowIndex < 0` | Kit di vendita | Button "Genera PDF" | Disable until kit selected |
| `isVisible: get_kit_help.data[0]?.help_url.length > 0` | Kit di vendita | Button "Supporto" | Show only if help URL exists |
| `isDisabled: tbl_accounts.updatedRowIndices.length == 0` | IaaS Credito | Button "Salva modifiche" | Disable until rows edited |
| `isDisabled: tbl_racks.updatedRowIndices.length == 0` | Sconti energia | Button "Salva modifiche" | Disable until rows edited |
| `isDisabled: !sl_customers.selectedOptionValue` | Gestione credito | Button "Nuova transazione" | Disable until customer selected |
| `sl_groups defaultValue: get_group_associations.data.map(i => i.group_id)` | Gruppi sconto | Multi-select default | Pre-check existing associations |
| `kit.variable_billing ? "SI" : "NO"` | Kit di vendita | PDF data payload | Boolean → Italian text for PDF |
| `Text2: "Gruppi per {{customer.name}}"` | Gruppi sconto | Modal title | Dynamic title |

### Domain logic (move to backend)

| Expression | Page | Widget | Current | Backend enforcement |
|------------|------|--------|---------|---------------------|
| Discount validation 0–20% | Sconti energia | `tbl_racks.sconto` column | Widget validation | Backend rejects out-of-range |
| IaaS price min/max ranges | IaaS Prezzi | `js_form_prezzi` fields | Widget schema | Backend rejects out-of-range (Phase A: hard business constraints) |
| Credit amount 0–10000 | Gestione credito | `i_importo` input | Widget validation | Backend rejects out-of-range |
| Description max 255 chars | Gestione credito | `i_descrizione` input | Widget validation | Backend rejects overlength |
| Credit editable only if `infrastructure_platform == 'cloudstack'` | IaaS Credito | `tbl_accounts.credito` | Widget binding | Backend rejects non-CloudStack updates |
| Customer eligibility filters (stato, codice_aggancio, fatgamma) | Multiple | SQL WHERE | Query filter | Backend endpoint applies same filters |

---

## Backend Validation Rules (Server-Side Required)

| Rule | Values | Entity | Source |
|------|--------|--------|--------|
| charge_cpu | 0.05–0.1 | IaaSPricing | Phase A Q8: hard business constraint |
| charge_ram_kvm | 0.05–0.2 | IaaSPricing | Phase A Q8 |
| charge_ram_vmware | 0.18–0.3 | IaaSPricing | Phase A Q8 |
| charge_pstor | 0.0005–0.002 | IaaSPricing | Phase A Q8 |
| charge_sstor | 0.0005–0.002 | IaaSPricing | Phase A Q8 |
| charge_ip | >= 0.02 | IaaSPricing | Phase A Q8 |
| sconto | 0–20 | Rack | Audit: widget validation |
| amount | 0–10000 | CreditTransaction | Audit: widget validation |
| description | max 255, required | CreditTransaction | Audit: widget validation |
| operation_sign | '+' or '-' | CreditTransaction | Audit: radio group |
| customer_id | > 0 for writes | Timoo pricing | Audit: JSObject guard |
| transaction immutability | no UPDATE/DELETE | CreditTransaction | Phase A Q14: intentional |

---

## Duplication to Consolidate

| Duplicated pattern | Pages | Backend consolidation |
|-------------------|-------|----------------------|
| HubSpot audit (diff → note) | IaaS Prezzi, IaaS Credito, Sconti energia | Single `hubspot.AuditService` in Go backend |
| Customer list query (3 variants) | 6 pages | 3 backend endpoints: `/customers`, `/customers/erp-linked`, `/grappa/customers` |
| Inline-edit + batch-save orchestration | IaaS Credito, Sconti energia | Consistent batch endpoint pattern: `PATCH /resource/batch` |
| Price fallback (customer → default via UNION) | IaaS Prezzi, Timoo | Backend helper: query customer-specific, fallback to default |

---

## Logic Being Revised (Not Ported As-Is)

| Logic | Current | New behavior | Reason |
|-------|---------|-------------|--------|
| Timoo read query | Hardcoded `customer_id = 110` | Parameterized by selection | Bug fix |
| Timoo write | INSERT (duplicates) | UPSERT (idempotent) | Bug fix |
| IaaS Credito record matching | Bitwise `&` | SQL `AND` (natural fix) | Bug fix |
| HubSpot calls | Frontend JSObject | Backend service | Security (credentials) |
| User identity | `appsmith.user.email` | Keycloak JWT email claim | Auth migration |
| HubSpot task assignee | Hardcoded `eva.grimaldi@cdlan.it` | Backend config (env var) | Maintainability |
| Batch updates | Row-by-row loop | Single transaction | Data integrity |
| Validation | UI-only widget constraints | Backend + UI | Business rules enforcement |

---

## Phase C Questions for Domain Expert

### ~~C1.~~ RESOLVED. Failures tolerated and non-blocking. Data save proceeds even if HubSpot fails. Async HubSpot request queue planned as cross-app infrastructure (see `docs/TODO.md`).
### ~~C2.~~ RESOLVED. Immediate effect. Simple UPSERT, no effective-date logic needed.
### ~~C3.~~ RESOLVED. Port as-is (no approval workflow). Future approval workflow tracked in `docs/TODO.md`.
### ~~C4.~~ RESOLVED. Port as-is. No link between storno and original transaction.
### ~~C5.~~ RESOLVED. No rules. Any customer can be assigned to any group.
### ~~C6.~~ RESOLVED. Hardcoded 0–20% global range. Keep as-is for now.

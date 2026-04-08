# Phase A: Entity-Operation Model — Listini e Sconti

## Extracted Facts

The audit identifies 9 candidate domain entities across 2 databases (db-mistra PostgreSQL, Grappa MySQL). Below is each entity with its inferred operations, fields, relationships, constraints, and open questions.

---

### Entity 1: Customer (cross-database)

**Tables:** `customers.customer` (Mistra PG), `cli_fatturazione` (Grappa MySQL), `loader.erp_clienti_provenienza` (Mistra PG)

**Role:** Primary lookup entity — used by every functional page as a selector/filter.

**Inferred operations:**
- `list` — 3 variants depending on page context:
  1. All customers (`customers.customer ORDER BY name`) — Gestione credito, Timoo
  2. Customers with ERP link (`JOIN loader.erp_clienti_provenienza WHERE fatgamma > 0`) — Gruppi sconto
  3. Active billing customers from Grappa (`cli_fatturazione WHERE stato = 'attivo'`) — IaaS Prezzi, IaaS Credito, Sconti energia

**Fields observed:**

| Field | Source | Type (inferred) | Notes |
|-------|--------|-----------------|-------|
| id | customers.customer, cli_fatturazione | int | PK in both DBs — unclear if same ID space |
| name | customers.customer | string | Display label in Mistra pages |
| intestazione | cli_fatturazione | string | Display label in Grappa pages (= company name) |
| stato | cli_fatturazione | string | Filter: 'attivo' |
| codice_aggancio_gest | cli_fatturazione | int | Exclusion filter: <>385, NOT IN (385,485) |
| fatgamma | erp_clienti_provenienza | int | Eligibility: >0 |
| numero_azienda | erp_clienti_provenienza | int | FK to customers.customer.id |

**Relationships:**
- Has many → CustomerGroup (via `group_association`)
- Has many → CustomerCredit / CreditTransaction
- Has many → IaaSPricing (via `cdl_prezzo_risorse_iaas.id_anagrafica`)
- Has many → IaaSAccount (via `cdl_accounts.id_cli_fatturazione`)
- Has many → Rack (via `racks.id_anagrafica`)
- Has many → CustomPricing (via `custom_items.customer_id`)

**Constraints:**
- Different eligibility rules per page (see list variants above)
- Cross-database identity: `customers.customer.id` (Mistra) vs `cli_fatturazione.id` (Grappa) — relationship unclear

**Open questions:**
- ~~**Q1:**~~ **RESOLVED:** `customers.customer.id` = Alyante ERP ID. `cli_fatturazione.id` = internal Grappa ID. Bridge: `cli_fatturazione.codice_aggancio_gest` = ERP ID. See `docs/IMPLEMENTATION-KNOWLEDGE.md#customer-identity-across-systems`.
- ~~**Q2:**~~ **RESOLVED:** Keep separate endpoints per datasource. Mistra and Grappa have different customer ID spaces (ERP ID vs internal Grappa ID) — unifying would add complexity without benefit. Maintain: (A) Mistra all customers, (B) Mistra ERP-linked, (C) Grappa active billing — as separate backend endpoints.
- ~~**Q3:**~~ **RESOLVED:** Keep exclusions hardcoded (385 for IaaS Prezzi, 385+485 for IaaS Credito) for compatibility with Appsmith during coexistence period. The two apps will run side by side temporarily.

---

### Entity 2: Kit

**Tables:** `products.kit`, `products.kit_product`, `products.product`, `products.product_category`, `products.kit_help`

**Role:** Primary entity for "Kit di vendita" page. Read-only catalog browsing + PDF export.

**Inferred operations:**
- `list` — active non-ecommerce kits, sorted by category then name
- `getProducts(kitId)` — component products for a kit (UNION with conditional main product)
- `getHelpUrl(kitId)` — support documentation link
- `exportPDF(kitId)` — generate PDF via Carbone template

**Fields observed:**

| Field | Source | Type (inferred) | Notes |
|-------|--------|-----------------|-------|
| id | kit | int | PK |
| internal_name | kit | string | Display name |
| billing_period | kit | string/enum | Billing cycle |
| initial_subscription_months | kit | int | Initial contract period |
| next_subscription_months | kit | int | Renewal period |
| activation_time_days | kit | int | Activation SLA |
| category_id | kit | int | FK → product_category |
| is_main_prd_sellable | kit | boolean | Controls main product visibility |
| ecommerce | kit | boolean | Filter: false for this app |
| is_active | kit | boolean | Filter: true |
| sconto_massimo | kit | decimal | Max discount percentage |
| variable_billing | kit | boolean | Converted to SI/NO in PDF |
| h24_assurance | kit | boolean | Converted to SI/NO in PDF |
| sla_resolution_hours | kit | int | SLA hours |
| notes | kit | text | Free-text notes |

**Kit Product fields:**
| Field | Source | Type | Notes |
|-------|--------|------|-------|
| group_name | kit_product | string | Product grouping |
| internal_name | product | string | Product display name |
| minimum | kit_product | int | Min quantity |
| maximum | kit_product | int | Max quantity |
| required | kit_product | boolean | Is product mandatory |
| nrc | kit_product | decimal (EUR) | Non-recurring charge |
| mrc | kit_product | decimal (EUR) | Monthly recurring charge |
| position | kit_product | int | Sort order |
| product_code | product | string | Product SKU |

**Relationships:**
- Belongs to → ProductCategory
- Has many → KitProduct (junction to Product)
- Has one → KitHelp (optional help URL)
- Has many → KitCustomerGroup (discount per group, used by Gruppi sconto page)

**Constraints:**
- Only `is_active = true AND ecommerce = false` shown in this app
- Main product included in product list only when `is_main_prd_sellable = true`, forced to required=true, position=0

**Open questions:**
- ~~**Q4:**~~ **RESOLVED:** Kit is read-only in this app. No CRUD needed.
- ~~**Q5:**~~ **RESOLVED:** Keep Carbone template IDs in code for now. A portal-wide admin module for template management is planned (see `docs/TODO.md`). Apps using Carbone will be updated once that module exists.

---

### Entity 3: CustomerGroup

**Tables:** `customers.customer_group`, `customers.group_association`

**Role:** Discount group management. Many-to-many relationship between Customer and Group.

**Inferred operations:**
- `listGroups` — all groups, ordered by name
- `getAssociations(customerId)` — groups assigned to a customer
- `syncAssociations(customerId, groupIds[])` — diff-based insert/delete of associations

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| id | customer_group | int | PK |
| name | customer_group | string | Display name |
| customer_id | group_association | int | FK → customer |
| group_id | group_association | int | FK → customer_group |

**Relationships:**
- Belongs to many → Customer (via group_association)
- Has many → KitCustomerGroup (discount per kit)

**Constraints:**
- ON CONFLICT DO NOTHING on insert (prevents duplicates)
- Composite natural key: (customer_id, group_id)

**Open questions:**
- ~~**Q6:**~~ **RESOLVED:** Group CRUD is managed in the kit-products app. This app only manages customer ↔ group associations.

---

### Entity 4: KitGroupDiscount

**Tables:** `products.kit_customer_group`

**Role:** Read-only view showing what discount a group gets on each active kit.

**Inferred operations:**
- `listByGroup(groupId)` — kit discounts for a specific group

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| kit_id | kit_customer_group | int | FK → kit |
| group_id | kit_customer_group | int | FK → customer_group |
| discount_mrc | kit_customer_group | decimal | Monthly recurring discount |
| discount_nrc | kit_customer_group | decimal | Non-recurring discount |
| kit_name | JOIN products.kit | string | Derived display field |

**Relationships:**
- Belongs to → Kit
- Belongs to → CustomerGroup

**Open questions:**
- ~~**Q7:**~~ **RESOLVED:** No CRUD needed for KitGroupDiscount in this app. Read-only view.

---

### Entity 5: IaaSPricing

**Tables:** `grappa.cdl_prezzo_risorse_iaas`

**Role:** Per-customer daily pricing for CloudStack IaaS resources.

**Inferred operations:**
- `getByCustomer(customerId)` — customer-specific or default pricing (UNION + LIMIT 1 fallback)
- `upsert(customerId, prices)` — insert or update 7 price fields atomically

**Fields observed:**

| Field | Source | Type | Min | Max | Notes |
|-------|--------|------|-----|-----|-------|
| id_anagrafica | cdl_prezzo_risorse_iaas | int | — | — | FK → cli_fatturazione.id; NULL = default |
| charge_cpu | cdl_prezzo_risorse_iaas | decimal | 0.05 | 0.1 | Per CPU daily |
| charge_ram_kvm | cdl_prezzo_risorse_iaas | decimal | 0.05 | 0.2 | Per GB RAM KVM |
| charge_ram_vmware | cdl_prezzo_risorse_iaas | decimal | 0.18 | 0.3 | Per GB RAM VMware |
| charge_pstor | cdl_prezzo_risorse_iaas | decimal | 0.0005 | 0.002 | Per GB primary storage |
| charge_sstor | cdl_prezzo_risorse_iaas | decimal | 0.0005 | 0.002 | Per GB secondary storage |
| charge_ip | cdl_prezzo_risorse_iaas | decimal | 0.02 | — | Per additional IP |
| charge_prefix24 | cdl_prezzo_risorse_iaas | decimal | — | — | Hidden field, /24 prefix |

**Side-effect:** HubSpot audit note on change (via HS_utils1)

**Constraints:**
- Min/max ranges enforced in UI (see table above)
- Default pricing row has `id_anagrafica IS NULL`
- UPSERT via `ON DUPLICATE KEY UPDATE`

**Open questions:**
- ~~**Q8:**~~ **RESOLVED:** Hard business constraints. Must be enforced backend-side, not just UI.
- ~~**Q9:**~~ **RESOLVED:** Not exposed to users of this app. Keep hidden — field exists in DB but is not surfaced in this application's UI.

---

### Entity 6: IaaSAccount (Credit)

**Tables:** `grappa.cdl_accounts`, `grappa.cli_fatturazione`, `grappa.cdl_services`

**Role:** CloudStack account with credit allocation. Inline editable.

**Inferred operations:**
- `list` — all active billing accounts with credit info
- `updateCredit(domainuuid, idCliFatturazione, credito)` — update credit for one account

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| intestazione | cli_fatturazione | string | Company name (joined) |
| credito | cdl_accounts | decimal | Editable credit amount |
| domainuuid (cloudstack_domain) | cdl_accounts | string (UUID) | CloudStack domain identifier |
| id_cli_fatturazione | cdl_accounts | int | FK → cli_fatturazione |
| abbreviazione | cdl_accounts | string | Short account name |
| codice_ordine | cdl_accounts | string | Order code |
| serialnumber | cdl_accounts | string | Serial number |
| data_attivazione | cdl_accounts | date | Activation date |
| infrastructure_platform | cdl_services (joined) | string | 'cloudstack' or other — controls editability |
| attivo | cdl_accounts | int (boolean) | Filter: 1 |
| fatturazione | cdl_accounts | int (boolean) | Filter: 1 |

**Side-effect:** HubSpot audit note with old/new credit values

**Constraints:**
- Credit editable only when `infrastructure_platform == 'cloudstack'`
- Filters: `attivo = 1`, `fatturazione = 1`, `codice_aggancio_gest NOT IN (385, 485)`
- Composite key for update: (domainuuid, id_cli_fatturazione)

**Open questions:**
- ~~**Q10:**~~ **RESOLVED:** Bug is latent (works by accident with booleans). Fix in the new app — the backend will use proper SQL `AND` so the issue disappears naturally.

---

### Entity 7: Rack (Energy Discount)

**Tables:** `grappa.racks`, `grappa.datacenter`, `grappa.dc_build`, `grappa.rack_sockets`

**Role:** Datacenter rack with energy variable discount. Inline editable.

**Inferred operations:**
- `listByCustomer(customerId)` — racks for a customer with location details
- `updateDiscount(idRack, sconto)` — update discount percentage

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| id_rack | racks | int | PK |
| name | racks | string | Rack name |
| floor | racks | string | Floor |
| island | racks | string | Island |
| type | racks | string | Rack type |
| sconto | racks | decimal | Discount %, editable, 0-20 range |
| stato | racks | string | Filter: 'attivo' |
| id_anagrafica | racks | int | FK → cli_fatturazione.id |
| building | dc_build.name | string | Building name (joined) |
| room | datacenter.name | string | Room name (joined) |

**Side-effects:**
- HubSpot audit note with HTML table of changed racks
- HubSpot task assigned to eva.grimaldi@cdlan.it: "Verificare gli sconti alla componente variabile di energia"

**Constraints:**
- Discount: 0–20% (UI validation)
- Only active racks shown
- Only customers with active racks appear in dropdown (subquery on rack_sockets)

**Open questions:**
- ~~**Q11:**~~ **RESOLVED:** Keep hardcoded for now. TODO tracked in `docs/TODO.md` to make it configurable in the future.
- ~~**Q12:**~~ **RESOLVED:** Correct behavior. Rack without sockets has no energy consumption, so no discount to manage. The filter is intentional.

---

### Entity 8: CustomerCredit

**Tables:** `customers.customer_credits`, `customers.customer_credit_transaction`

**Role:** Customer credit balance and immutable transaction ledger.

**Inferred operations:**
- `getBalance(customerId)` — current credit summary
- `listTransactions(customerId)` — transaction history, newest first
- `addTransaction(customerId, amount, sign, description)` — insert new credit/debit entry

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| id | credit_transaction | int | PK |
| customer_id | credit_transaction | int | FK → customer |
| transaction_date | credit_transaction | timestamp | Auto-generated |
| amount | credit_transaction | decimal | 0–10000 range |
| operation_sign | credit_transaction | string | '+' or '-' |
| signed_amount | credit_transaction | decimal | Derived (amount * sign) |
| description | credit_transaction | string | Required, max 255 chars |
| operated_by | credit_transaction | string (email) | `appsmith.user.email` → Keycloak email |

**Constraints:**
- Insert-only (immutable ledger) — no update/delete
- Amount: 0–10000
- Description required
- Operator identity captured automatically

**Open questions:**
- ~~**Q13:**~~ **RESOLVED:** Updated by external jobs. The app treats it as read-only; no need to refresh it after inserting transactions.
- ~~**Q14:**~~ **RESOLVED:** Intentional immutable ledger. Corrections done via storno (opposite-sign transaction). No edit/delete needed.

---

### Entity 9: CustomPricing (Timoo)

**Tables:** `products.custom_items`

**Role:** Per-customer pricing for Timoo indirect (reseller) service.

**Inferred operations:**
- `getByCustomer(customerId)` — customer-specific or default pricing (UNION + LIMIT 1 fallback)
- `save(customerId, prices)` — insert pricing record (**BUG: should be UPSERT**)

**Fields observed:**

| Field | Source | Type | Notes |
|-------|--------|------|-------|
| key_label | custom_items | string | Discriminator: 'timoo_indiretta' |
| customer_id | custom_items | int | FK → customer; -1 = defaults |
| prices | custom_items | JSON | `{user_month: decimal, se_month: decimal}` |

**Default values:** `user_month = 0.78`, `se_month = 0.3`

**Constraints:**
- Price fallback: customer-specific → default (customer_id = -1)
- No validation on price values (no min/max in form)

**Bugs (from audit):**
- Read query hardcodes `customer_id = 110` — must be parameterized
- INSERT without UPSERT — repeated saves create duplicate records

**Open questions:**
- ~~**Q15:**~~ **RESOLVED:** `custom_items` is a generic key/JSON store used by multiple contexts. For this app, access it as-is with `key_label = 'timoo_indiretta'`. No need to generalize the entity — just use the table directly for this specific query.
- ~~**Q16:**~~ **RESOLVED:** No HubSpot audit for Timoo. Intentional — no CRM tracking needed for this pricing.

---

## Cross-Entity Observations

### Potential Entity Merges

| Candidate merge | Rationale | Risk |
|----------------|-----------|------|
| IaaSPricing + CustomPricing → "CustomerPricing" | Both are per-customer pricing with default fallback | Different DBs, different structures (columnar vs JSON), different domains |
| IaaSAccount + Rack → "BillableAsset" | Both are inline-edit + batch-save + HubSpot audit | Very different domains (cloud vs datacenter) |

**Recommendation:** Keep entities separate — the commonality is in the UX pattern, not the domain model.

### Missing Entities

| Possible entity | Evidence | Notes |
|----------------|----------|-------|
| **ProductCategory** | Used in Kit queries, loaded but unused standalone query | May not need its own entity if Kit list includes category |
| **Product** | Referenced in kit_product join | Subordinate to Kit in this app |
| **HubSpotCompany** | HS_utils1.CompanyByGrappaId mapping | External system, not a local entity — but the mapping needs to exist somewhere |

---

## Questions for the Domain Expert

### Identity & Structure
1. ~~**Q1:**~~ **RESOLVED.** See `docs/IMPLEMENTATION-KNOWLEDGE.md#customer-identity-across-systems`.
2. ~~**Q2:**~~ **RESOLVED.** Separate endpoints per datasource — no unification of Mistra/Grappa customer lists.
3. ~~**Q3:**~~ **RESOLVED.** Hardcoded exclusions for coexistence with Appsmith.

### Scope & CRUD
4. ~~**Q4:**~~ **RESOLVED.** Read-only. No CRUD.
5. ~~**Q6:**~~ **RESOLVED.** Group CRUD managed in kit-products app.
6. ~~**Q7:**~~ **RESOLVED.** No CRUD needed. Read-only view.

### Business Rules
7. ~~**Q8:**~~ **RESOLVED.** Hard business constraints — enforce server-side.
8. ~~**Q9:**~~ **RESOLVED.** Not exposed to this app's users. Keep hidden.
9. ~~**Q11:**~~ **RESOLVED.** Hardcoded for now; TODO tracked for future configurability.
10. ~~**Q14:**~~ **RESOLVED.** Intentional immutable ledger.

### Data & Integration
11. ~~**Q5:**~~ **RESOLVED.** Template IDs in code for now; portal admin module planned.
12. ~~**Q10:**~~ **RESOLVED.** Latent bug, fix in new app.
13. ~~**Q12:**~~ **RESOLVED.** Correct — no sockets = no consumption = no discount.
14. ~~**Q13:**~~ **RESOLVED.** Updated by external jobs. Read-only in this app.
15. ~~**Q15:**~~ **RESOLVED.** Generic key/JSON store. Use as-is with `key_label = 'timoo_indiretta'`.
16. ~~**Q16:**~~ **RESOLVED.** No HubSpot audit for Timoo. Intentional.

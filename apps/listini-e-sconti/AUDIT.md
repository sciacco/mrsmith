# Appsmith Audit — Listini e Sconti

## Application Inventory

| Field | Value |
|-------|-------|
| **App name** | listini-e-sconti |
| **Source** | Appsmith Git export (ZIP) |
| **Format version** | 5 (client schema 2, server schema 11) |
| **Theme** | Pampas (system) |
| **Layout** | Fixed, sidebar navigation, light |
| **Pages** | 8 (1 home + 7 functional) |
| **Datasources** | 3 (db-mistra, Grappa, hubs) |
| **Source modules** | 2 (HS_utils, carboneUtils) |

### Pages

| # | Page | Default | Datasource | Domain |
|---|------|---------|------------|--------|
| 1 | Home | Yes | — | Splash/branding |
| 2 | Kit di vendita | No | db-mistra (PG) | Catalogo kit, PDF export |
| 3 | IaaS Prezzi risorse | No | Grappa (MySQL) | Pricing IaaS per cliente |
| 4 | IaaS Credito omaggio | No | Grappa (MySQL) | Crediti IaaS per account |
| 5 | Sconti variabile energia | No | Grappa (MySQL) | Sconti energia su rack |
| 6 | Gruppi di sconto x clienti | No | db-mistra (PG) | Associazione clienti-gruppi sconto |
| 7 | Gestione credito cliente | No | db-mistra (PG) | Crediti e transazioni cliente |
| 8 | Timoo prezzi indiretta | No | db-mistra (PG) | Pricing Timoo per cliente |

### Datasources

| Name | Plugin | Type | Used by |
|------|--------|------|---------|
| **db-mistra** | postgres-plugin | PostgreSQL | Kit di vendita, Gruppi di sconto, Gestione credito cliente, Timoo prezzi indiretta |
| **Grappa** | mysql-plugin | MySQL | IaaS Prezzi risorse, IaaS Credito omaggio, Sconti variabile energia |
| **hubs** | restapi-plugin | REST API | Sconti variabile energia (HubSpot, legacy — replaced by HS_utils module) |

### Source Modules

| Module | Package | Version | Purpose |
|--------|---------|---------|---------|
| **HS_utils** | Hubspot integrations | 0.0.18 | CRM audit trail: note/task creation, company lookup by Grappa ID |
| **carboneUtils** | Carbone Pkg | 0.0.2 | PDF generation via Carbone templates |

---

## Page Audits

### 1. Home

**Purpose:** Splash page with branding image.

- 1 widget: `Image1` — loads `https://t.sciacco.net/x/sconti-e-listini.png`
- No queries, no logic
- Static page

---

### 2. Kit di vendita

**Purpose:** Browse active non-ecommerce product kits, view component products, generate branded PDF.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_kit_list` | db-mistra | `SELECT kit.id, kit.internal_name, kit.billing_period, ... FROM products.kit JOIN products.product_category pc ON pc.id = kit.category_id WHERE is_active is true AND ecommerce is false ORDER BY pc.name, internal_name` | — | Yes |
| `get_kit_products` | db-mistra | UNION: (1) `products.kit_product` LEFT JOIN `products.product` WHERE `kit_id = {{tbl_kit.selectedRow.id}}`; (2) main product from `products.kit` WHERE `is_main_prd_sellable = true` — ORDER BY position, group_name, internal_name | `tbl_kit.selectedRow.id` | No |
| `get_kit_help` | db-mistra | `SELECT help_url, kit_id FROM products.kit_help WHERE kit_id = {{tbl_kit.selectedRow?.id \|\| -1}}` | `tbl_kit.selectedRow?.id` | Yes |
| `get_product_category` | db-mistra | `SELECT * FROM products.product_category ORDER BY name` | — | Yes (unused) |
| `json_kits` | db-mistra | `SELECT products.get_all_kit()->0 AS kits` | — | Yes (unused) |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `tbl_kit` | TABLE_V2 | `get_kit_list.data` | onRowSelected → `get_kit_products.run().then(() => get_kit_help.run())` |
| `tbl_products` | TABLE_V2 | `get_kit_products.data` | — |
| `Button1` "Genera PDF" | BUTTON | `isDisabled: tbl_kit.selectedRowIndex < 0` | onClick → `carboneIO.generaPDF()` |
| `Button2` "Supporto" | BUTTON | `isVisible: get_kit_help.data[0]?.help_url.length > 0` | onClick → `navigateTo(help_url, {}, 'NEW_WINDOW')` |

#### JSObjects

**carboneIO.generaPDF():**
- Extracts `tbl_kit.selectedRow` + `get_kit_products.data`
- Converts booleans to Italian "SI"/"NO" (`variable_billing`, `h24_assurance`)
- Calls `carboneUtils1.generatePDF(templateId, data, {reportName: "kit " + name})`
- Template ID hardcoded: `d7c2d6...b657`
- Opens PDF download in new window

#### Business Rules

- Only active, non-ecommerce kits shown
- Main product included only if `is_main_prd_sellable = true` (required=true, position=0)
- PDF requires kit selection
- Support button shown only if `kit_help.help_url` exists

#### DB Tables

`products.kit`, `products.kit_product`, `products.product`, `products.product_category`, `products.kit_help`

---

### 3. IaaS Prezzi risorse

**Purpose:** Manage per-customer daily pricing for IaaS CloudStack resources (CPU, RAM, storage, IP). Changes logged to HubSpot.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_customers` | Grappa | `SELECT id, intestazione FROM cli_fatturazione WHERE stato = 'attivo' AND codice_aggancio_gest > 0 AND codice_aggancio_gest <> 385 ORDER BY intestazione` | — | Yes |
| `get_prezzi_per_cliente` | Grappa | UNION: customer-specific prices (`id_anagrafica = {{sl_cliente.selectedOptionValue\|\|-1}}`) UNION default prices (`id_anagrafica IS NULL`) — LIMIT 1 | `sl_cliente.selectedOptionValue` | Yes |
| `upsert_prezzi_per_cliente` | Grappa | `INSERT INTO cdl_prezzo_risorse_iaas ... ON DUPLICATE KEY UPDATE` — 7 price fields | customer ID + 7 prices from form | No |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `sl_cliente` | SELECT | `get_customers.data` (label=intestazione, value=id) | onOptionChange → `get_prezzi_per_cliente.run().then(() => resetWidget("js_form_prezzi"))` |
| `js_form_prezzi` | JSON_FORM | sourceData: `get_prezzi_per_cliente.data[0]` | onSubmit → `upsert_prezzi_per_cliente.run().then(() => { utils.aggiungiNotaSuHubspot(); showAlert('Dati salvati') })` |

#### Form Fields & Validation

| Field | Label | Min | Max | Required |
|-------|-------|-----|-----|----------|
| `charge_cpu` | Importo per ogni Cpu | 0.05 | 0.1 | Yes |
| `charge_ram_kvm` | Importo per ogni GB di Ram Kvm | 0.05 | 0.2 | Yes |
| `charge_ram_vmware` | Importo per ogni GB di Ram Vmware | 0.18 | 0.3 | Yes |
| `charge_pstor` | Importo per ogni GB di Primary Storage | 0.0005 | 0.002 | Yes |
| `charge_sstor` | Importo per ogni GB di Secondary Storage | 0.0005 | 0.002 | Yes |
| `charge_ip` | Importo per ogni IP addizionale | 0.02 | — | Yes |
| `charge_prefix24` | Charge Prefix 24 | — | — | No (hidden) |

#### JSObjects

**utils.aggiungiNotaSuHubspot():**
- Compares `js_form_prezzi.formData` vs `get_prezzi_per_cliente.data[0]`
- If any field differs: builds HTML table with new values
- Calls `HS_utils1.CompanyByGrappaId(sl_cliente.selectedOptionValue)` → `HS_utils1.AddNoteToCompany(companyId, tableHTML)`

#### Business Rules

- Customer filter: `stato = 'attivo'`, `codice_aggancio_gest > 0`, `<> 385`
- Price fallback: customer-specific overrides default prices (UNION + LIMIT 1)
- UPSERT: inserts new or updates existing prices atomically
- CRM audit: every change logged to HubSpot with formatted HTML table
- Min/max constraints per resource type enforced in UI

#### DB Tables

`grappa.cli_fatturazione`, `grappa.cdl_prezzo_risorse_iaas`

---

### 4. IaaS Credito omaggio

**Purpose:** Manage IaaS credit allocations for CloudStack accounts. Inline editing with batch save and HubSpot audit trail.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_cdl_accounts` | Grappa | `SELECT c.intestazione, a.credito, domainuuid AS cloudstack_domain, ... FROM cdl_accounts a JOIN cli_fatturazione c ON a.id_cli_fatturazione = c.id JOIN cdl_services ON ... WHERE id_cli_fatturazione > 0 AND attivo = 1 AND fatturazione = 1 AND c.codice_aggancio_gest NOT IN (385,485) ORDER BY intestazione` | — | Yes |
| `upd_credito` | Grappa | `UPDATE cdl_accounts SET credito = {{this.params.credito}} WHERE domainuuid = {{this.params.domainuuid}} AND id_cli_fatturazione = {{this.params.id_cli_fatturazione}}` | credito, domainuuid, id_cli_fatturazione | No |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `tbl_accounts` | TABLE_V2 | `get_cdl_accounts.data` | Inline edit on `credito` column — editable only when `infrastructure_platform == 'cloudstack'` |
| `Button1` "Salva modifiche" | BUTTON | `isDisabled: tbl_accounts.updatedRowIndices.length == 0` | onClick → `utils.saveCrediti()` |

#### JSObjects

**utils.saveCrediti():**
- Iterates `tbl_accounts.updatedRows`
- For each changed row: runs `upd_credito`, then logs to HubSpot with old/new credit values via `HS_utils1.CompanyByGrappaId` + `HS_utils1.AddNoteToCompany`
- Refreshes table + success alert

#### Business Rules

- Credit editing restricted to CloudStack platform accounts only
- Filter: `attivo = 1`, `fatturazione = 1`, `codice_aggancio_gest NOT IN (385,485)`
- HubSpot audit note includes old value, new value, CloudStack domain
- Batch save (button enabled only when rows modified)

#### Bug

- `utils.saveCrediti()` line 7: uses bitwise `&` instead of logical `&&` for domain matching — could cause incorrect record lookup

#### DB Tables

`grappa.cdl_accounts`, `grappa.cli_fatturazione`, `grappa.cdl_services`

---

### 5. Sconti variabile energia

**Purpose:** Manage energy variable component discount percentages on datacenter racks. Changes logged to HubSpot with task assignment.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_customers` | Grappa | `SELECT id, intestazione FROM cli_fatturazione WHERE id IN (SELECT DISTINCT id_anagrafica FROM racks JOIN rack_sockets rs ON racks.id_rack = rs.rack_id WHERE racks.stato = 'attivo') ORDER BY intestazione` | — | Yes |
| `get_racks` | Grappa | `SELECT DISTINCT db.name building, d.name room, r.name, r.floor, r.island, r.type, r.sconto, r.id_rack FROM racks r LEFT JOIN datacenter d ... LEFT JOIN dc_build db ... WHERE r.stato = 'attivo' AND r.id_anagrafica = {{s_cliente.selectedOptionValue\|\|-1}} ORDER BY 1,2,3` | `s_cliente.selectedOptionValue` | Yes |
| `upd_sconto` | Grappa | `UPDATE racks SET sconto = {{this.params.sconto}} WHERE id_rack = {{this.params.id_rack}}` | sconto, id_rack | No |
| `hs_create_note_remove` | hubs (REST) | POST `/crm/v3/objects/notes` — legacy, replaced by HS_utils module | properties, associations | No (unused) |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `s_cliente` | SELECT | `get_customers.data` (label=intestazione, value=id) | — |
| `Button2` "Cerca" | BUTTON | — | onClick → `get_racks.run()` |
| `tbl_racks` | TABLE_V2 | `get_racks.data` — `sconto` column editable (0-20% range validation) | Inline edit, CUSTOM save |
| `Button1` "Salva modifiche" | BUTTON | `isDisabled: tbl_racks.updatedRowIndices.length == 0` | onClick → `utils.saveSconti()` |

#### JSObjects

**utils.saveSconti():**
- Iterates `tbl_racks.updatedRows`
- For each: runs `upd_sconto.run({id_rack, sconto})`
- Builds HTML table of changes (building, room, rack name, floor, island, type, sconto)
- Calls `HS_utils1.CompanyByGrappaId` → `HS_utils1.AddNoteToCompany` (audit note)
- Calls `HS_utils1.AddTaskToCompany` with:
  - Subject: "Verificare gli sconti alla componente variabile di energia"
  - Assigned to: **eva.grimaldi@cdlan.it**
- Refreshes table + alert

#### Business Rules

- Discount range: 0–20% (validated in table column)
- Only active racks shown (`stato = 'attivo'`)
- Only customers with active racks in dropdown
- HubSpot note + task created on every save
- Task hardcoded to specific reviewer (eva.grimaldi@cdlan.it)
- Row-by-row update (not batch SQL)

#### DB Tables

`grappa.racks`, `grappa.datacenter`, `grappa.dc_build`, `grappa.rack_sockets`, `grappa.cli_fatturazione`

---

### 6. Gruppi di sconto x clienti

**Purpose:** Manage customer-to-discount-group associations. View kit discounts per group.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_customers` | db-mistra | `SELECT * FROM customers.customer c JOIN loader.erp_clienti_provenienza ecp ON c.id = ecp.numero_azienda WHERE ecp.fatgamma > 0 ORDER BY name` | — | Yes |
| `get_customer_groups` | db-mistra | `SELECT * FROM customers.customer_group ORDER BY name` | — | Yes |
| `get_group_associations` | db-mistra | `SELECT ga.*, cg.name AS group_name FROM customers.group_association ga JOIN customers.customer_group cg ON cg.id = ga.group_id WHERE ga.customer_id = {{tbl_customers.selectedRow.id}}` | `tbl_customers.selectedRow.id` | No |
| `get_kit_group` | db-mistra | `SELECT k.internal_name AS kit_name, kcg.* FROM products.kit_customer_group kcg JOIN products.kit k ON k.id = kcg.kit_id WHERE group_id = {{tbl_groups.selectedRow.group_id}} AND k.is_active = true ORDER BY internal_name` | `tbl_groups.selectedRow.group_id` | No |
| `ins_group_associations` | db-mistra | `INSERT INTO customers.group_association (customer_id, group_id) VALUES (...) ON CONFLICT DO NOTHING` | customer_id, group_id | No |
| `del_group_associations` | db-mistra | `DELETE FROM customers.group_association WHERE customer_id = ... AND group_id = ...` | customer_id, group_id | No |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `tbl_customers` | TABLE_V2 | `get_customers.data` — custom icon-button column "Associa" (percentage icon) | onRowSelected → `get_group_associations.run().then(() => get_kit_group.run())`; icon onClick → `showModal(Modal1)` |
| `tbl_groups` | TABLE_V2 | `get_group_associations.data` | onRowSelected → `get_kit_group.run()` |
| `tbl_kit` | TABLE_V2 | `get_kit_group.data` | — |
| `Text1` | TEXT | "Selezionare il cliente sulla tabella di sinistra per vedere i gruppi associati" | — |
| **Modal1** | MODAL | Contains: sl_groups (MULTI_SELECT), Button1 "Close", Button2 "Salva gruppi", Text2 "Gruppi per {{customer.name}}" | — |
| `sl_groups` | MULTI_SELECT_V2 | source: `get_customer_groups.data` — default: `get_group_associations.data.map(i => i.group_id)` | — |
| `Button2` "Salva gruppi" | BUTTON | — | onClick → `utils.salvaGruppi(); get_group_associations.run(); closeModal(Modal1)` |

#### JSObjects

**utils.salvaGruppi():**
- Computes diff between `sl_groups.selectedOptionValues` and `get_group_associations.data.map(i => i.group_id)`
- Deletes removed associations (loop: `del_group_associations.run()`)
- Inserts new associations (loop: `ins_group_associations.run()` with ON CONFLICT DO NOTHING)

#### Business Rules

- Customer eligibility: must exist in `loader.erp_clienti_provenienza` with `fatgamma > 0`
- Association is a many-to-many (customer ↔ group) via `customers.group_association`
- Kit discounts shown per group — only active kits (`is_active = true`)
- Diff-based save: only changed associations are inserted/deleted
- ON CONFLICT DO NOTHING prevents duplicate associations

#### DB Tables

`customers.customer`, `loader.erp_clienti_provenienza`, `customers.customer_group`, `customers.group_association`, `products.kit_customer_group`, `products.kit`

---

### 7. Gestione credito cliente

**Purpose:** View and manage customer credit balances and credit transactions (accredito/debito).

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_customers` | db-mistra | `SELECT * FROM customers.customer ORDER BY name` | — | Yes |
| `get_customer_credit` | db-mistra | `SELECT * FROM customers.customer_credits WHERE customer_id = {{sl_customers.selectedOptionValue\|\|0}}` | `sl_customers.selectedOptionValue` | Yes |
| `get_customer_transactions` | db-mistra | `SELECT * FROM customers.customer_credit_transaction WHERE customer_id = {{sl_customers.selectedOptionValue\|\|0}} ORDER BY transaction_date DESC` | `sl_customers.selectedOptionValue` | Yes |
| `ins_credit_transaction` | db-mistra | `INSERT INTO customers.customer_credit_transaction (customer_id, amount, operation_sign, description, operated_by) VALUES ({{sl_customers.selectedOptionValue}}, {{i_importo.text}}, {{rg_segno.selectedOptionValue}}, {{i_descrizione.text}}, {{appsmith.user.email}})` | customer_id, amount, sign, description, user email | No |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `sl_customers` | SELECT | `get_customers.data` (label=name, value=id) | — (manual refresh) |
| `Button3` "Aggiorna" | BUTTON | — | onClick → `get_customer_credit.run(); get_customer_transactions.run()` |
| `Button4` "Nuova transazione" | BUTTON | `isDisabled: !sl_customers.selectedOptionValue` | onClick → `showModal(Modal1)` |
| `Table1` | TABLE_V2 | `get_customer_transactions.data` — read-only | — |
| **Modal1** | MODAL | Transaction entry form | — |
| `i_importo` | INPUT (NUMBER) | min=0, max=10000, required | — |
| `rg_segno` | RADIO_GROUP | "Accredito" (+) / "Debito" (-), default="+" | — |
| `i_descrizione` | INPUT (MULTI_LINE) | max 255 chars, required | — |
| `Button2` "Confirm" | BUTTON | disabledWhenInvalid | onClick → `ins_credit_transaction.run().then(() => { closeModal(); showAlert('Transazione registrata') }); get_customer_credit.run(); get_customer_transactions.run()` |

#### Business Rules

- Transaction operator tracked via `appsmith.user.email` (audit)
- Amount: 0–10000 range
- Operation: Accredito (+) or Debito (-) — radio toggle
- Description required (max 255 chars)
- Insert-only (no edit/delete of transactions — immutable ledger)
- Transactions sorted newest first

#### DB Tables

`customers.customer`, `customers.customer_credits`, `customers.customer_credit_transaction`

---

### 8. Timoo prezzi indiretta

**Purpose:** Configure per-customer monthly pricing for Timoo indirect (reseller) service — user cost and service-extension cost.

#### Queries

| Query | Datasource | SQL / Body | Params | On load |
|-------|-----------|------------|--------|---------|
| `get_customers` | db-mistra | `SELECT * FROM customers.customer ORDER BY name` | — | Yes |
| `get_prezzi_cliente` | db-mistra | UNION: customer-specific prices (`customer_id = 110` **[BUG: hardcoded]**) UNION defaults (`customer_id = -1`) — LIMIT 1 | — (should be `sl_customers.selectedOptionValue`) | Yes |
| `ins_prezzi_cliente` | db-mistra | `INSERT INTO products.custom_items (key_label, customer_id, prices) VALUES ('timoo_indiretta', {{sl_customers.selectedOptionValue}}, {{JSONForm1.formData}})` | customer_id, prices JSON | No |

#### Widgets

| Widget | Type | Data/Binding | Events |
|--------|------|-------------|--------|
| `sl_customers` | SELECT | `get_customers.data` (label=name, value=id), default="-1" | onOptionChange → `get_prezzi_cliente.run()` |
| `JSONForm1` | JSON_FORM | sourceData: `get_prezzi_cliente.data[0].prices` — title "Prezzo mensile" | onSubmit → `utils.salva_form()` |

#### Form Fields

| Field | Label | Default |
|-------|-------|---------|
| `user_month` | User | 0.78 |
| `se_month` | Service Extensions | 0.3 |

#### JSObjects

**utils.salva_form():**
- Guards: `sl_customers.selectedOptionValue > 0`
- Runs `ins_prezzi_cliente.run()` (INSERT, not UPSERT)

#### Business Rules

- Prices stored as JSON in `products.custom_items` (key_label = 'timoo_indiretta')
- Fallback to defaults (customer_id = -1) if no custom pricing
- INSERT only — no update capability (creates duplicate records risk)

#### Critical Bug

- `get_prezzi_cliente` hardcodes `customer_id = 110` instead of `{{sl_customers.selectedOptionValue}}` — dropdown selection is ignored for reads

#### DB Tables

`customers.customer`, `products.custom_items`

---

## Datasource & Query Catalog

### db-mistra (PostgreSQL) — 17 queries

| Page | Query | R/W | Tables | Backend candidate |
|------|-------|-----|--------|-------------------|
| Kit di vendita | get_kit_list | R | products.kit, products.product_category | Yes |
| Kit di vendita | get_kit_products | R | products.kit_product, products.product, products.kit | Yes |
| Kit di vendita | get_kit_help | R | products.kit_help | Yes |
| Kit di vendita | get_product_category | R | products.product_category | Unused |
| Kit di vendita | json_kits | R | products.get_all_kit() | Unused |
| Gruppi di sconto | get_customers | R | customers.customer, loader.erp_clienti_provenienza | Yes |
| Gruppi di sconto | get_customer_groups | R | customers.customer_group | Yes |
| Gruppi di sconto | get_group_associations | R | customers.group_association, customers.customer_group | Yes |
| Gruppi di sconto | get_kit_group | R | products.kit_customer_group, products.kit | Yes |
| Gruppi di sconto | ins_group_associations | W | customers.group_association | Yes |
| Gruppi di sconto | del_group_associations | W | customers.group_association | Yes |
| Gestione credito | get_customers | R | customers.customer | Yes |
| Gestione credito | get_customer_credit | R | customers.customer_credits | Yes |
| Gestione credito | get_customer_transactions | R | customers.customer_credit_transaction | Yes |
| Gestione credito | ins_credit_transaction | W | customers.customer_credit_transaction | Yes |
| Timoo | get_customers | R | customers.customer | Yes |
| Timoo | get_prezzi_cliente | R | products.custom_items | Yes (fix bug) |
| Timoo | ins_prezzi_cliente | W | products.custom_items | Yes (needs UPSERT) |

### Grappa (MySQL) — 8 queries

| Page | Query | R/W | Tables | Backend candidate |
|------|-------|-----|--------|-------------------|
| IaaS Prezzi | get_customers | R | cli_fatturazione | Yes |
| IaaS Prezzi | get_prezzi_per_cliente | R | cdl_prezzo_risorse_iaas | Yes |
| IaaS Prezzi | upsert_prezzi_per_cliente | W | cdl_prezzo_risorse_iaas | Yes |
| IaaS Credito | get_cdl_accounts | R | cdl_accounts, cli_fatturazione, cdl_services | Yes |
| IaaS Credito | upd_credito | W | cdl_accounts | Yes |
| Sconti energia | get_customers | R | cli_fatturazione, racks, rack_sockets | Yes |
| Sconti energia | get_racks | R | racks, datacenter, dc_build | Yes |
| Sconti energia | upd_sconto | W | racks | Yes |

### hubs (REST API) — 1 query (legacy, unused)

| Page | Query | R/W | Endpoint | Backend candidate |
|------|-------|-----|----------|-------------------|
| Sconti energia | hs_create_note_remove | W | POST /crm/v3/objects/notes | Replaced by HS_utils module |

### External Module Calls

| Module | Method | Used by | Purpose |
|--------|--------|---------|---------|
| HS_utils1 | CompanyByGrappaId | IaaS Prezzi, IaaS Credito, Sconti energia | Lookup HubSpot company ID from Grappa ID |
| HS_utils1 | AddNoteToCompany | IaaS Prezzi, IaaS Credito, Sconti energia | Add audit note to HubSpot company |
| HS_utils1 | AddTaskToCompany | Sconti energia | Create task assigned to reviewer |
| carboneUtils1 | generatePDF | Kit di vendita | Render PDF from Carbone template |

---

## Findings Summary

### Embedded Business Rules

| # | Rule | Page | Source | Classification |
|---|------|------|--------|----------------|
| 1 | Only active non-ecommerce kits shown | Kit di vendita | SQL WHERE | Business logic |
| 2 | Main product included conditionally (`is_main_prd_sellable`) | Kit di vendita | SQL UNION | Business logic |
| 3 | Customer eligibility: active + valid aggancio code | IaaS Prezzi, IaaS Credito | SQL WHERE | Business logic |
| 4 | Price fallback: customer-specific → default | IaaS Prezzi, Timoo | SQL UNION + LIMIT | Business logic |
| 5 | IaaS price min/max ranges per resource type | IaaS Prezzi | Widget schema | Business logic |
| 6 | Discount range 0–20% on racks | Sconti energia | Widget validation | Business logic |
| 7 | Credit editing restricted to CloudStack platform | IaaS Credito | Widget binding | Business logic |
| 8 | Customer eligibility: `fatgamma > 0` from ERP | Gruppi sconto | SQL WHERE/JOIN | Business logic |
| 9 | Transaction immutability (insert-only) | Gestione credito | No update/delete queries | Business logic |
| 10 | Transaction amount 0–10000 | Gestione credito | Widget validation | Business logic |
| 11 | Boolean → "SI"/"NO" for PDF localization | Kit di vendita | JSObject ternary | Presentation |
| 12 | HubSpot audit trail on every price/discount/credit change | IaaS Prezzi, IaaS Credito, Sconti energia | JSObject logic | Orchestration |
| 13 | Task assignment to eva.grimaldi@cdlan.it | Sconti energia | JSObject hardcoded | Business logic |
| 14 | Operator tracking via `appsmith.user.email` | Gestione credito | SQL INSERT | Business logic |
| 15 | Excluded aggancio codes: 385 (IaaS Prezzi), 385+485 (IaaS Credito) | IaaS pages | SQL WHERE | Business logic |

### Duplication

| Pattern | Pages | Notes |
|---------|-------|-------|
| `get_customers` query (same pattern) | 6 pages (different SQL per datasource) | 3 variants: Grappa cli_fatturazione, Mistra customers.customer, Mistra customers.customer + ERP join |
| HubSpot audit pattern (detect change → note) | IaaS Prezzi, IaaS Credito, Sconti energia | Same flow, different data shapes |
| Inline-edit + batch-save pattern | IaaS Credito, Sconti energia | Table with editable column + "Salva modifiche" button |
| Customer dropdown → dependent query | IaaS Prezzi, Sconti energia, Gestione credito, Timoo | Same UX pattern, different data |

### Bugs & Issues

| # | Severity | Page | Issue |
|---|----------|------|-------|
| 1 | **Critical** | Timoo prezzi indiretta | `get_prezzi_cliente` hardcodes `customer_id = 110` — dropdown selection ignored for reads |
| 2 | **Medium** | IaaS Credito omaggio | `utils.saveCrediti()` uses bitwise `&` instead of logical `&&` for record matching |
| 3 | **Medium** | Timoo prezzi indiretta | INSERT without UPSERT — creates duplicate records on repeated saves for same customer |
| 4 | **Low** | Kit di vendita | Unused queries `get_product_category` and `json_kits` loaded on page init |
| 5 | **Low** | Sconti energia | Legacy REST query `hs_create_note_remove` still present (replaced by HS_utils module) |

### Security Concerns

| # | Issue | Page |
|---|-------|------|
| 1 | Direct database access from UI for all read/write operations | All pages |
| 2 | No row-level authorization — any authenticated user can modify any customer's data | All pages |
| 3 | HubSpot API credentials stored in Appsmith datasource config | IaaS Prezzi, IaaS Credito, Sconti energia |
| 4 | Hardcoded email address for task assignment | Sconti energia |
| 5 | No CSRF or replay protection on write operations | All pages |

### Candidate Domain Entities

| Entity | Tables | Pages |
|--------|--------|-------|
| **Customer** | `customers.customer`, `cli_fatturazione`, `loader.erp_clienti_provenienza` | All |
| **Kit** | `products.kit`, `products.kit_product`, `products.product`, `products.product_category`, `products.kit_help` | Kit di vendita |
| **Customer Group** | `customers.customer_group`, `customers.group_association` | Gruppi sconto |
| **Kit Group Discount** | `products.kit_customer_group` | Gruppi sconto |
| **IaaS Pricing** | `cdl_prezzo_risorse_iaas` | IaaS Prezzi |
| **IaaS Account / Credit** | `cdl_accounts`, `cdl_services` | IaaS Credito |
| **Rack / Discount** | `racks`, `datacenter`, `dc_build`, `rack_sockets` | Sconti energia |
| **Customer Credit** | `customers.customer_credits`, `customers.customer_credit_transaction` | Gestione credito |
| **Custom Pricing** | `products.custom_items` | Timoo |

### Migration Blockers

| # | Blocker | Impact |
|---|---------|--------|
| 1 | Carbone template ID hardcoded — template must be accessible from new backend | Kit di vendita |
| 2 | HubSpot integration via Appsmith module — must be reimplemented as backend service | 3 pages |
| 3 | Two separate databases (PostgreSQL + MySQL) — backend must maintain dual connections | All pages |
| 4 | `appsmith.user.email` used for audit — must be replaced with Keycloak identity | Gestione credito |
| 5 | Appsmith `resetWidget`, `showModal`, `closeModal`, `showAlert` patterns — must be replaced with React state management | All pages |

### Recommended Next Steps

1. **Fix Timoo bug** before migration — hardcoded customer_id=110 makes the page non-functional
2. **Consolidate customer queries** — 3 variants of "get active customers" should become backend endpoints
3. **Extract HubSpot integration** — single backend service with standard audit-trail pattern
4. **Design backend API** — all 25+ SQL queries should become Go API endpoints behind auth
5. **Map Keycloak roles** — `app_listini_access` for page access, potentially finer roles per section
6. **Hand off to `appsmith-migration-spec`** for Phase 2 specification

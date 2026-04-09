# Quotes — Phase A: Entity-Operation Model

**Source**: `apps/quotes/APPSMITH-AUDIT.md` + `docs/mistradb/mistra_quotes.json` + `docs/mistradb/mistra_products.json` + `docs/mistradb/mistra_common.json` + `apps/quotes/hubspot-integrations-main.zip`
**Date**: 2026-04-09

### Scope decisions (expert input, 2026-04-09)

- **Order conversion deferred**: The "Converti in ordine" page and all Vodka/GW internal/gpUtils dependencies are OUT OF SCOPE for this migration. Second phase.
- **HS_utils module available**: `apps/quotes/hubspot-integrations-main.zip` contains the `HS_utils` Appsmith package. Key function `ListCompanyContacts(company_id)`: calls `GET /crm/v3/objects/companies/{id}?associations=contacts` then fetches each contact via `GET /crm/v3/objects/contacts/{id}`. Straightforward Go reimplementation.
- **Standard vs. IaaS merge**: Whether to merge the two creation wizards is a UI/UX decision, not purely technical. Will be addressed in Phase B.
- **Unified quote creation workflow**: The Appsmith two-step flow (wizard creates empty HS quote → Dettaglio configures products → publish syncs to HS) is an Appsmith workaround, NOT a business requirement. The new app must implement a single workflow that creates a **complete** quote — header, kits, products, and HS sync — in one coherent flow. This fundamentally changes the creation→editing boundary: creation must include product configuration, not just header metadata.
- **Explicit save with dirty-state indicator**: No auto-save. User must explicitly save. UI must show when there are unsaved changes.
- **E-signature removed**: The Firma tab and all e-signature functionality (`hs_esign_enabled`, `hs_esign_contacts`, `hs_sign_status`, `hs_esign_date`, signer contact associations, `firmaForm` logic, `HS_utils1.ListCompanyContacts` for signers) are OUT OF SCOPE — failed experiment, not migrated. DB columns remain but are not exposed in UI or write logic.
- **HubSpot publish**: Step-by-step progress with idempotent retry on partial failure.

---

## 1. Core Entities (owned by Quotes schema)

### 1.1 Quote (head)

**Table**: `quotes.quote` — 41 columns, ~976 rows
**Purpose**: Central entity. A sales proposal tied to a HubSpot deal and customer, containing commercial terms, document metadata, HubSpot sync state, e-signature state, and contact references.

#### Fields (verified from DDL)

| Field | Type | Nullable | Default | Source / Notes |
|---|---|---|---|---|
| `id` | integer | NO | sequence `quote_id_seq` | PK |
| `quote_number` | varchar(32) | NO | `'SP-' \|\| nextval('quote_number_seq') \|\| '/' \|\| YYYY` | Unique, auto-generated |
| `customer_id` | bigint | YES | — | FK → `loader.hubs_company.id` (logical, no DB constraint) |
| `ragione_sociale` | varchar(255) | YES | — | Always NULL from Nuova Proposta; possibly populated from other paths |
| `deal_number` | varchar(32) | YES | — | HubSpot deal code (e.g. `"D-123/2025"`) |
| `owner` | varchar(100) | YES | — | HubSpot owner ID (stored as string) |
| `document_date` | date | YES | `now()` | |
| `document_type` | varchar(32) | YES | `'TSC-ORDINE-RIC'` | `TSC-ORDINE-RIC` (recurring) or `TSC-ORDINE` (spot) |
| `replace_orders` | varchar(255) | YES | — | Semicolon-separated Alyante order names (only for SOSTITUZIONE) |
| `template` | varchar(255) | YES | — | HubSpot template ID (string, e.g. `"853027287235"`) |
| `services` | varchar(255) | YES | — | Comma-separated product category IDs |
| `proposal_type` | varchar(32) | YES | `'NUOVO'` | `NUOVO` / `SOSTITUZIONE` / `RINNOVO` |
| `initial_term_months` | smallint | NO | `12` | Disabled for IaaS/VCloud (fixed 1) |
| `next_term_months` | smallint | NO | `12` | Disabled for IaaS/VCloud (fixed 1) |
| `bill_months` | smallint | NO | `2` | Billing period months (1–24). Forced to 3 for COLOCATION |
| `delivered_in_days` | integer | NO | `60` | |
| `date_sent` | date | YES | — | |
| `status` | varchar(32) | NO | `'DRAFT'` | `DRAFT` / `PENDING_APPROVAL` / `APPROVED` / `APPROVAL_NOT_NEEDED` / `ESIGN_COMPLETED` |
| `notes` | text | YES | — | Legal notes (pattuizioni speciali). Non-empty triggers PENDING_APPROVAL |
| `nrc_charge_time` | smallint | NO | `2` | 1=all'ordine, 2=all'attivazione |
| `created_at` | timestamp | NO | `now()` | |
| `updated_at` | timestamp | NO | `now()` | Trigger: `common.trigger_set_timestamp()` |
| `description` | text | NO | `''` | Quote description (HTML) |
| `hs_deal_id` | bigint | YES | — | HubSpot deal ID |
| `hs_quote_id` | bigint | YES | — | HubSpot quote ID (written after HS creation) |
| `payment_method` | char(6) | YES | — | ERP payment code (default 402 from Alyante) |
| `hs_esign_enabled` | boolean | YES | `false` | |
| `hs_esign_contacts` | jsonb | YES | — | Array of signer contact objects |
| `hs_sign_status` | varchar(20) | YES | — | HubSpot signature status |
| `hs_esign_date` | timestamptz | YES | — | |
| `rif_ordcli` | varchar(240) | YES | — | Customer order reference |
| `rif_tech_nom` | varchar(240) | YES | — | Technical contact name |
| `rif_tech_tel` | varchar(240) | YES | — | Technical contact phone |
| `rif_tech_email` | varchar(240) | YES | — | Technical contact email |
| `rif_altro_tech_nom` | varchar(240) | YES | — | Alternate technical contact name |
| `rif_altro_tech_tel` | varchar(240) | YES | — | Alternate technical contact phone |
| `rif_altro_tech_email` | varchar(240) | YES | — | Alternate technical contact email |
| `rif_adm_nom` | varchar(240) | YES | — | Administrative contact name |
| `rif_adm_tech_tel` | varchar(240) | YES | — | Administrative contact phone |
| `rif_adm_tech_email` | varchar(240) | YES | — | Administrative contact email |
| `trial` | varchar(255) | YES | — | IaaS trial text (bilingual, slider-generated) |

#### Operations

| Operation | Appsmith source | Mechanism | Notes |
|---|---|---|---|
| **Create** | `Nuova Proposta.utils.salvaOfferta()`, `IaaS.creazioneProposta.salvaOfferta()` | `SELECT quotes.ins_quote_head(json)` → returns `{id, status, message}` | Always created as DRAFT. HS quote created first, then DB record. |
| **Read one** | `Dettaglio.get_quote_by_id`, `Converti.get_quote_by_id` | `SELECT * FROM quotes.quote WHERE id = :v_offer_id` | |
| **Read list** | `Elenco.get_quotes` | JOIN `quotes.quote` + `loader.hubs_company` + `loader.hubs_deal` + `loader.hubs_owner` | `ORDER BY quote_number DESC LIMIT 2000` |
| **Update header** | `Dettaglio.mainForm.salvaOfferta()` | `SELECT quotes.upd_quote_head(json)` → returns `{status, message}` | Updates all header fields including contacts and e-sign state |
| **Delete** | `Elenco.utils.eliminaOfferta()` | `DELETE FROM quotes.quote WHERE id = :id` | Hard delete. Role-gated (client-side only). Preceded by HS quote delete if `hs_quote_id > 0` |
| **Publish to HubSpot** | `Dettaglio.mainForm.mandaSuHubspot()` | 16-step orchestration: save → validate → sync line items → sync signers → update HS quote → save status back | Status becomes PENDING_APPROVAL (if legal notes) or APPROVED |
| **Convert to order** | `Converti.utilsCopy.g_orchestra()` | 10-step multi-system flow: Vodka order → PDF → HS file upload → HS note | External module `gpUtils1` drives order creation |

#### Triggers on `quotes.quote`

| Trigger | Event | Function | Effect |
|---|---|---|---|
| `set_timestamp` | BEFORE UPDATE | `common.trigger_set_timestamp()` | Auto-updates `updated_at` |

**Note**: `update_quote_customer_from_erp()` trigger is defined in the schema but its attachment to `quotes.quote` is not visible in the DDL dump. It populates `quote_customer` from `loader.erp_anagrafiche_clienti` when `customer_id` changes.

---

### 1.2 Quote Customer

**Table**: `quotes.quote_customer` — 12 columns
**Purpose**: Snapshot of ERP billing/fiscal data at quote creation time. Auto-populated by trigger from `loader.erp_anagrafiche_clienti`.

#### Fields

| Field | Type | Nullable | Notes |
|---|---|---|---|
| `id` | integer | NO | PK (sequence) |
| `quote_id` | integer | NO | FK → `quotes.quote(id)` ON DELETE CASCADE |
| `codice_fiscale` | varchar(255) | YES | Tax code |
| `partita_iva` | varchar(255) | YES | VAT number |
| `lingua` | varchar(255) | YES | Language |
| `codice_sdi` | varchar(255) | YES | SDI code (e-invoicing) |
| `indirizzo_fatturazione` | varchar(255) | YES | Billing address |
| `citta_fatturazione` | varchar(255) | YES | Billing city |
| `provincia_fatturazione` | varchar(255) | YES | Billing province |
| `cap_fatturazione` | varchar(255) | YES | Billing ZIP |
| `paese_fatturazione` | varchar(255) | YES | Billing country |
| `metodo_pagamento` | varchar(255) | YES | Payment method |

#### Operations

| Operation | Source | Notes |
|---|---|---|
| **Auto-insert/update** | Trigger `update_quote_customer_from_erp()` | Fires on quote INSERT (or customer_id change) — reads from `loader.erp_anagrafiche_clienti` |
| **No direct CRUD in Appsmith** | — | This entity is never read or written by Appsmith UI. Purely a backend snapshot. |

**Question A1**: Is `quote_customer` ever displayed or used downstream (e.g., in PDFs, order conversion)? The Appsmith export never references it.

---

### 1.3 Quote Row (Kit)

**Table**: `quotes.quote_rows` — 10 columns, ~1910 rows
**Purpose**: A kit instance attached to a quote. Each row represents one service kit, ordered by `position`. Totals (`nrc_row`, `mrc_row`) are auto-computed by trigger from included products.

#### Fields

| Field | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| `id` | integer | NO | sequence | PK |
| `quote_id` | integer | NO | — | FK → `quotes.quote(id)` ON DELETE CASCADE |
| `kit_id` | bigint | NO | — | FK → `products.kit(id)` ON DELETE RESTRICT |
| `internal_name` | varchar(255) | YES | `''` | Copied from kit on insert (via trigger) |
| `nrc_row` | numeric(14,4) | YES | `0` | Auto-computed: sum of included product NRC×quantity |
| `mrc_row` | numeric(14,4) | YES | `0` | Auto-computed: sum of included product MRC×quantity |
| `bundle_prefix_row` | varchar(64) | YES | `''` | Copied from kit on insert |
| `hs_line_item_id` | bigint | YES | — | HubSpot MRC line item ID (written after HS sync) |
| `hs_line_item_nrc` | bigint | YES | — | HubSpot NRC line item ID (written after HS sync) |
| `position` | integer | YES | `9000` | Display ordering |

#### Operations

| Operation | Appsmith source | Mechanism | Notes |
|---|---|---|---|
| **Create** | Nuova Proposta (step 3, loop), Dettaglio (add kit modal) | `INSERT INTO quotes.quote_rows (quote_id, kit_id) VALUES (:qid, :kid)` | Trigger `insert_product_rows_trigger` auto-expands kit → products |
| **Read** | Dettaglio: `get_quote_rows` | `SELECT * FROM quotes.quote_rows WHERE quote_id = :id ORDER BY position` | |
| **Update position** | Dettaglio: inline edit on `tbl_quote_rows` | `UPDATE quotes.quote_rows SET position = :pos WHERE id = :id` | |
| **Delete** | Dettaglio: trash button | `DELETE FROM quotes.quote_rows WHERE id = :id` (confirmBeforeExecute) | CASCADE deletes products |
| **Read for HS** | Dettaglio: `get_line_item_hs` | View `v_quote_rows_for_hs` — bilingual descriptions via `common.get_short_translation()` | |

#### Triggers on `quotes.quote_rows`

| Trigger | Event | Function | Effect |
|---|---|---|---|
| `insert_product_rows_trigger` | AFTER INSERT | `quotes.update_kit_product_rows()` | Copies all `products.kit_product` rows into `quote_rows_products`. Sets `internal_name`, `nrc_row`, `mrc_row`, `bundle_prefix_row` from kit. Appends kit `legal_notes` custom value to quote `notes`. |
| `update_kit_product_rows_trigger` | AFTER UPDATE OF `kit_id` | `quotes.update_kit_product_rows()` | Same as above, but deletes old products first. Only fires when `kit_id` actually changes. |

---

### 1.4 Quote Row Product

**Table**: `quotes.quote_rows_products` — 14 columns, ~24183 rows
**Purpose**: Individual product options within a kit row. Grouped by `group_name` in the UI. Only `included = true` products contribute to row totals and are published to HubSpot.

#### Fields

| Field | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| `id` | integer | NO | sequence | PK |
| `quote_row_id` | integer | NO | — | FK → `quotes.quote_rows(id)` ON DELETE CASCADE |
| `product_code` | varchar(32) | NO | — | FK → `products.product(code)` ON DELETE RESTRICT |
| `minimum` | integer | NO | `0` | Min quantity |
| `maximum` | integer | NO | `-1` | Max quantity (-1 = unlimited) |
| `required` | boolean | NO | `false` | Must be included before publish |
| `nrc` | numeric(14,5) | NO | `0` | Unit NRC price |
| `mrc` | numeric(14,5) | NO | `0` | Unit MRC price |
| `position` | integer | NO | `0` | Display ordering within group |
| `group_name` | varchar(255) | YES | — | Product group label (COALESCE with product internal_name) |
| `included` | boolean | NO | `false` | Whether this product variant is selected |
| `quantity` | numeric | NO | `0` | Quantity. Forced to 1 if included but 0 in Appsmith |
| `extended_description` | text | NO | `''` | Long HTML description (auto-populated from translation on insert) |
| `main_product` | boolean | NO | `false` | Is this the kit's main product? |

#### Operations

| Operation | Appsmith source | Mechanism | Notes |
|---|---|---|---|
| **Auto-create** | Trigger on `quote_rows` INSERT | `quotes.update_kit_product_rows()` | Copies from `products.kit_product` + populates translations |
| **Read grouped** | Dettaglio: `get_quote_products_grouped` | View `v_quote_rows_products` — groups by `group_name`, aggregates into `riga` JSON array | Used by the product config UI |
| **Update** | Dettaglio: `detailForm.aggiornaRiga()` | `SELECT quotes.upd_quote_row_product(json)` | Sets `included`, clears previous included in same group. Forces `mrc=0` for spot orders. Forces `quantity=1` if included but 0 |
| **Validate** | Dettaglio: `check_quote_rows` | `SELECT` finds required products not included | Blocks HubSpot publish |
| **Read for HS** | Dettaglio: `get_line_item_hs` | View `v_quote_rows_for_hs` → bilingual `string_agg` descriptions | |

#### Triggers on `quotes.quote_rows_products`

| Trigger | Event | Function | Effect |
|---|---|---|---|
| `trigger_update_quote_row_totals` | AFTER INSERT/DELETE/UPDATE | `quotes.update_quote_row_totals()` | Recalculates `quote_rows.nrc_row` and `mrc_row` from included products |

#### Key business logic in `upd_quote_row_product`

1. Finds `quote_row_id` and `group_name` from the product being updated
2. Sets `included = false` for ALL products in the same group (mutual exclusion within group)
3. Updates the target product with new `included`, `extended_description`, `nrc`, `mrc`, `quantity`
4. Row totals auto-recalculated by trigger

**Question A2**: The mutual exclusion logic (step 2) means only ONE product per group can be `included`. Is this always the case, or are there groups where multiple products should be selectable?

---

### 1.5 Template

**Table**: `quotes.template` — 3 columns + 5 nuove colonne (migrazione), 13 righe
**Purpose**: Registry of HubSpot quote templates. Determines language, T&C variant, and (for IaaS) automatic kit + services selection.

#### Fields (current + new columns)

| Field | Type | Status | Notes |
|---|---|---|---|
| `template_id` | varchar(255) | existing | PK — HubSpot template ID |
| `description` | varchar(255) | existing | Human-readable name |
| `lang` | varchar(2) | existing | `"it"` or `"en"` |
| `template_type` | varchar(16) | **NEW** | `'standard'` / `'iaas'` / `'legacy'`. Default `'standard'` |
| `kit_id` | bigint | **NEW** | FK → `products.kit(id)`. NULL for standard |
| `service_category_id` | integer | **NEW** | NULL for standard |
| `is_colo` | boolean | **NEW** | COLO vs NON COLO (solo standard). Default `false` |
| `is_active` | boolean | **NEW** | `false` per template "vecchio". Default `true` |

#### Full template registry (13 rows)

| template_id | description | lang | template_type | kit_id | service_cat | is_colo | is_active |
|---|---|---|---|---|---|---|---|
| `105348827359` | vecchio | it | legacy | — | — | — | false |
| `111577899484` | COLO IT | it | standard | — | — | true | true |
| `111583049949` | COLO EN | en | standard | — | — | true | true |
| `111583627969` | NON COLO IT | it | standard | — | — | false | true |
| `111583628251` | NON COLO EN | en | standard | — | — | false | true |
| `850825381069` | IaaS Diretta EN | en | iaas | 62 | 12 | false | true |
| `853027287235` | IaaS Diretta IT | it | iaas | 62 | 12 | false | true |
| `853237903587` | VCLOUD IaaS IT | it | iaas | 116 | 14 | false | true |
| `853320143046` | IaaS Indiretta EN | en | iaas | 63 | 13 | false | true |
| `853500178641` | IaaS Indiretta IT | it | iaas | 63 | 13 | false | true |
| `853500899556` | VCLOUD IaaS EN | en | iaas | 116 | 14 | false | true |
| `855439340792` | VCLOUD DRaaS EN | en | iaas | 119 | 15 | false | true |
| `856380863697` | VCLOUD DRaaS IT | it | iaas | 119 | 15 | false | true |

#### Operations (new app)

| Operation | Query | Notes |
|---|---|---|
| **List standard** | `WHERE template_type = 'standard' AND is_active AND is_colo = :has_colocation AND lang = :lang` | Replaces JS `template_suServizio()` |
| **List IaaS** | `WHERE template_type = 'iaas' AND is_active AND lang = :lang` | Replaces `get_templates` with LIKE filter |
| **Derive kit + services** | `SELECT kit_id, service_category_id FROM quotes.template WHERE template_id = :id` | Replaces hardcoded `recuperaServizio()` |
| **Derive T&C variant** | From `(template_type, is_colo, lang)` | Replaces hardcoded switch in `terms_and_conditions()` |

#### Coexistence

Appsmith reads only `template_id`, `description`, `lang` — new columns are nullable, Appsmith ignores them. Zero breaking changes.

---

## 2. Reference Entities (owned by other schemas, read-only from Quotes)

### 2.1 Kit

**Table**: `products.kit` — 21 columns, 83 rows
**Purpose**: Product kit catalog. A kit defines a bundle of products with pricing, terms, and category.

#### Fields relevant to Quotes

| Field | Type | Notes |
|---|---|---|
| `id` | bigint | PK |
| `internal_name` | varchar(255) | Unique. Displayed in kit picker and row table |
| `main_product_code` | varchar(32) | FK → `products.product(code)` |
| `category_id` | integer | FK → `products.product_category(id)` |
| `nrc` / `mrc` | numeric(14,5) | Default pricing copied to quote row |
| `translation_uuid` | uuid | For bilingual descriptions |
| `bundle_prefix` | varchar(64) | Prefix copied to quote row |
| `ecommerce` | boolean | `false` filter in Nuova Proposta kit list |
| `is_active` | boolean | Active filter |
| `quotable` | boolean | Default `true`. Kit available for quoting |
| `is_main_prd_sellable` | boolean | Whether main product gets its own product row |

#### Operations from Quotes

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read active list** | `Dettaglio.get_kit_internal_names` | `WHERE is_active = true` |
| **Read for tree** | `Nuova Proposta.list_kit` | `WHERE ecommerce = false AND is_active = true` — grouped by category in JS `treeOfKits()` |

### 2.2 Product Category

**Table**: `products.product_category`
**Purpose**: Service categories. IDs used for filtering and service selection.

#### Known exclusion rules

| Context | Excluded IDs | Reason |
|---|---|---|
| Dettaglio + Nuova Proposta: `get_product_category` | 12, 13, 14, 15 | IaaS/VCloud categories hidden from standard flow |
| Nuova Proposta: `get_product_category` | 12, 13 | Only IaaS Diretta/Indiretta excluded (14, 15 allowed?) |

**Question A5**: The category exclusion is inconsistent between Dettaglio (excludes 12,13,14,15) and Nuova Proposta (excludes 12,13 only). Which is correct? Should standard quotes ever see categories 14 (Vcloud) or 15 (DRaaS)?

### 2.3 Customer (HubSpot mirror)

**Table**: `loader.hubs_company`
**Purpose**: HubSpot company data synced by ETL loader.

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read list** | `Dettaglio.get_customers`, `Nuova Proposta.get_potentials` (deals carry company) | Used in customer selector dropdowns |
| **Read for join** | `Elenco.get_quotes` | `LEFT JOIN loader.hubs_company c ON q.customer_id = c.id` for display name |

### 2.4 Deal (HubSpot mirror)

**Table**: `loader.hubs_deal`
**Purpose**: HubSpot deal data synced by ETL loader.

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read filtered** | `Nuova Proposta.get_potentials`, `IaaS.get_deals` | Hardcoded pipeline IDs: `255768766`, `255768768` with stage whitelists. `codice <> ''` |
| **Read by ID** | `get_potential_by_id` / `get_deals_by_id` | Single deal detail with `numero_azienda` for Alyante cross-ref |
| **Resolve deal ID** | `Converti.GetDealIdByCodice` | `deal_number → deal_id` for HS navigation |

**Question A6**: The pipeline IDs `255768766` and `255768768` and their stage whitelists are hardcoded in SQL. Do these change frequently? Should they be configurable or are they stable enough to be backend constants?

### 2.5 Owner (HubSpot mirror)

**Table**: `loader.hubs_owner`
**Purpose**: HubSpot user/owner data.

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read list** | `Dettaglio.get_hs_owners`, `Nuova Proposta.get_owners` | Owner selector dropdowns |

### 2.6 Payment Method (ERP mirror)

**Table**: `loader.erp_metodi_pagamento`
**Purpose**: Alyante ERP payment method mirror.

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read list** | `get_payment_method` (all creation pages) | Payment selector dropdown |
| **Read customer default** | `get_pagamento_anagrCli` (Alyante MSSQL) | `ISNULL(CAST(CODICE_PAGAMENTO as INT), 402)` — fallback 402 |

### 2.7 Order (Alyante ERP)

**Source**: Alyante MS SQL Server, `Tsmi_Ordini`
**Purpose**: ERP orders referenced for SOSTITUZIONE proposal type.

| Operation | Appsmith source | Notes |
|---|---|---|
| **Read list** | `cli_orders` (Alyante) | ALL confirmed/delivered orders — **not filtered by customer** |

**Question A7**: `cli_orders` shows all Alyante orders regardless of customer. The audit flags this as a security concern. Should it be filtered by `NUMERO_AZIENDA = :customer_erp_id`?

---

## 3. External System Entities (not in DB, managed via API)

### 3.1 HubSpot Quote

**System**: HubSpot CRM REST API
**Purpose**: Published version of the quote with PDF generation, e-signature, and deal association.

| Operation | Appsmith source | HS Endpoint | Notes |
|---|---|---|---|
| **Create** | `new_hs_quote` | `POST /crm/v3/objects/quote` | Created with associations: template (286), deal (64), company (71) |
| **Read status** | `hs_get_quote_status` | `GET /crm/v3/objects/quotes/{id}` | Returns PDF link, sign status, countersigned status |
| **Update** | `hs_update_quote` | `PATCH /crm/v3/objects/quotes/{id}` | Properties: title, status, sender, terms, associations |
| **Delete** | `Cancella_HS_Quote` | `DELETE /crm/v3/objects/quotes/{id}` | Part of quote delete flow |
| **Get associations** | `hs_get_quote_associations` | `GET /crm/v3/objects/quotes/{id}?associations=...` | Line items and contacts |
| **Set association** | `hs_set_quote_association` | `PUT /crm/v4/objects/quotes/{id}/associations/...` | Template→quote association |

### 3.2 HubSpot Line Item

**System**: HubSpot CRM REST API
**Purpose**: Individual billable items on a HS quote. Two items per kit row: MRC and NRC.

| Operation | Appsmith source | HS Endpoint | Notes |
|---|---|---|---|
| **Create** | `hs_create_line_item` | `POST /crm/v3/objects/line_item` | Properties: name (bilingual), price, quantity, recurringbillingfrequency |
| **Update** | `hs_update_line_item` | `PATCH /crm/v3/objects/line_item/{id}` | |
| **Delete** | `hs_delete_line_item` | `DELETE /crm/v3/objects/line_item/{id}` | Orphan cleanup |
| **Write-back ID** | `update_line_item_id` | DB UPDATE | HS IDs stored in `quote_rows.hs_line_item_id` / `hs_line_item_nrc` |

### 3.3 HubSpot Contact (E-signature)

**System**: HubSpot CRM REST API
**Purpose**: Signer contacts associated with a quote for e-signature.

| Operation | Appsmith source | HS Endpoint | Notes |
|---|---|---|---|
| **List company contacts** | `firmaForm.listaContatti()` | Via external module `HS_utils1.ListCompanyContacts` | Source unavailable |
| **Associate signer** | `hs_associa_contatto` | `PUT /crm/v4/objects/quote/{id}/associations/contact/{cid}` | Association type 702 |
| **Remove signer** | `hs_delete_contact_signer` | `DELETE /crm/v4/objects/quote/{id}/associations/contact/{cid}` | |

### 3.4 Vodka Order — OUT OF SCOPE (deferred to order conversion phase)

### 3.5 GW Internal (PDF) — OUT OF SCOPE (deferred to order conversion phase)

### 3.6 Carbone.io (unused)

**System**: Carbone.io REST API
**Purpose**: PDF rendering from template. Currently unused (no UI trigger in Dettaglio).

**Question A8**: ~~RESOLVED~~ — Carbone.io escluso completamente. Era un esperimento, non più necessario. Nessuna integrazione Carbone nella nuova app.

---

## 4. Stored Procedures and Views Summary

### Stored Procedures

| Function | Input | Output | Critical logic |
|---|---|---|---|
| `quotes.ins_quote_head(json)` | Full quote header as JSON | `{id, status, message}` | INSERT with type casting. Returns -1 on error. |
| `quotes.upd_quote_head(json)` | Full quote header as JSON (must include `id`) | `{status, message}` | UPDATE all fields. Handles `hs_esign_contacts` as JSON. |
| `quotes.upd_quote_row_product(json)` | Product update as JSON (must include `id`) | boolean | **Mutual exclusion**: clears `included` in same group before setting new. |

### Trigger Functions

| Function | Trigger on | Critical logic |
|---|---|---|
| `quotes.update_kit_product_rows()` | `quote_rows` INSERT/UPDATE(kit_id) | Expands kit → product rows with translations + legal notes append |
| `quotes.update_quote_customer_from_erp()` | `quote` INSERT(?) | Snapshots ERP customer data into `quote_customer` |
| `quotes.update_quote_row_totals()` | `quote_rows_products` INSERT/DELETE/UPDATE | Recalculates `nrc_row`/`mrc_row` from included products |
| `common.trigger_set_timestamp()` | `quote` UPDATE | Sets `updated_at = now()` |

### Views

| View | Purpose | Used by |
|---|---|---|
| `v_quote_rows_for_hs` | Bilingual kit descriptions for HubSpot line items (`string_agg` of included products with `[n. X] <strong>name</strong>description`) | `get_line_item_hs` query in Dettaglio publish flow |
| `v_quote_rows_products` | Products grouped by `group_name` with JSON array `riga`, counts, and ordering | `get_quote_products_grouped` in Dettaglio product config UI |

---

## 5. Entity Relationship Map

```
quotes.quote (1) ──────── (0..1) quotes.quote_customer
     │                              [auto-populated by trigger from loader.erp_anagrafiche_clienti]
     │
     ├── (1) ──── (0..N) quotes.quote_rows
     │                        │
     │                        ├── FK → products.kit (N:1)
     │                        │
     │                        └── (1) ──── (0..N) quotes.quote_rows_products
     │                                                 │
     │                                                 └── FK → products.product (N:1)
     │
     ├── customer_id → loader.hubs_company (logical FK)
     ├── hs_deal_id → loader.hubs_deal (logical FK)
     ├── owner → loader.hubs_owner (logical FK, stored as varchar)
     ├── template → quotes.template (logical FK, stored as varchar)
     ├── payment_method → loader.erp_metodi_pagamento (logical FK)
     └── hs_quote_id → HubSpot CRM (external system reference)
```

---

## 6. Overlaps, Merges, and Gaps

### Potential merges

1. **Standard vs. IaaS quote creation**: Two separate pages (`Nuova Proposta`, `Nuova Proposta IaaS`) with ~70% shared logic. Can be merged into a single creation flow with a type selector that conditionally enables/disables fields.

2. **Duplicate queries across pages**: `get_product_category`, `get_payment_method`, `new_quote_number`, `ins_quote`/`ins_quote_rows`, `cli_orders`, `TypeDocument`, `Service` logic — all duplicated. Backend API consolidates naturally.

### Missing entities (not in current schema)

3. **Audit trail**: No soft-delete, no history table. Deletes are permanent. No record of who changed what.

4. **T&C document**: `templates.terms_and_conditions()` generates 6 variants of HTML Terms & Conditions in JS. This is significant business content with no DB persistence — it's regenerated every time from hardcoded strings.

5. **HubSpot sync state**: No dedicated table tracking HS sync attempts, failures, or partial states. The only sync state is `hs_quote_id`, `hs_line_item_id`, `hs_line_item_nrc` on existing tables.

### Ambiguities

6. **`ragione_sociale`**: Always NULL from Nuova Proposta (`ragione_sociale: null` hardcoded). Is it ever used? The column exists in the DB.

7. **`date_sent`**: Populated in some flows but not clear when. Not visible in the UI.

8. **`services` serialization**: Stored as comma-separated string (e.g., `"1,3,7"`). Should this become a proper array or join table?

---

## 7. Questions for Expert

### Entity completeness

**A1**: ~~RESOLVED~~ — Non ci riguarda. Il trigger esiste e continua a funzionare autonomamente. La nuova app non legge, non scrive, non espone `quote_customer`. Fuori scope.

**A2**: ~~RESOLVED~~ — Mutual exclusion confermata: un solo prodotto per gruppo. Logica attuale corretta, migriamo così com'è. Future multi-selection tracciata in `docs/TODO.md` (richiede coordinamento con gestione kit).

**A3**: ~~RESOLVED~~ — Il dump JSON era parziale. La tabella ha 13 righe:

| template_id | description | lang | Type | Notes |
|---|---|---|---|---|
| `105348827359` | vecchio | it | Legacy | Ancora in uso? |
| `111577899484` | COLO IT | it | Standard | |
| `111583049949` | COLO EN | en | Standard | |
| `111583627969` | NON COLO IT | it | Standard | |
| `111583628251` | NON COLO EN | en | Standard | |
| `850825381069` | IaaS Diretta EN | en | IaaS | Kit 62, services [12] |
| `853027287235` | IaaS Diretta IT | it | IaaS | Kit 62, services [12] |
| `853237903587` | VCLOUD IaaS IT | it | IaaS | Kit 116, services [14] |
| `853320143046` | IaaS Indiretta EN | en | IaaS | Kit 63, services [13] |
| `853500178641` | IaaS Indiretta IT | it | IaaS | Kit 63, services [13] |
| `853500899556` | VCLOUD IaaS EN | en | IaaS | Kit 116, services [14] |
| `855439340792` | VCLOUD DRaaS EN | en | IaaS | Kit 119, services [15] |
| `856380863697` | VCLOUD DRaaS IT | it | IaaS | Kit 119, services [15] |

4 standard + 8 IaaS + 1 legacy = 13 totali. Il template "vecchio" (`105348827359`) non appare nell'audit Appsmith.

### Configuration vs. hardcoding

**A4**: ~~RESOLVED~~ — Nuove colonne nullable su `quotes.template` (coesistenza Appsmith garantita):

```sql
ALTER TABLE quotes.template
  ADD COLUMN template_type varchar(16) DEFAULT 'standard',  -- 'standard' | 'iaas' | 'legacy'
  ADD COLUMN kit_id bigint REFERENCES products.kit(id),      -- NULL per standard
  ADD COLUMN service_category_id integer,                    -- NULL per standard
  ADD COLUMN is_colo boolean DEFAULT false,                  -- COLO vs NON COLO (solo standard)
  ADD COLUMN is_active boolean DEFAULT true;                 -- false per "vecchio"
```

Nuova app: filtra e deriva kit/services/T&C dal DB. Appsmith: ignora le nuove colonne, JS hardcoded continua a funzionare.

**A5**: ~~RESOLVED~~ — Esclusione corretta: 12,13,14,15 (tutte le categorie pay-per-use / IaaS/VCloud). Il flow standard non deve mai mostrare queste categorie. Nuova Proposta che esclude solo 12,13 è un bug — dovrebbe escludere anche 14,15.

**A6**: ~~RESOLVED~~ — Costanti backend per la prima implementazione. Configurabilità rimandata a dopo.

### Data integrity

**A7**: ~~RESOLVED~~ — Filtrare per cliente. Bug Appsmith: manca `WHERE NUMERO_AZIENDA = :customer_erp_id`. La nuova app filtra gli ordini Alyante per il cliente selezionato nella proposta.

### Scope

**A8**: Is Carbone.io planned for future use, or should it be dropped from the migration scope?

**A9**: ~~RESOLVED~~ — Order conversion deferred to second phase. `HS_utils` module source obtained from `hubspot-integrations-main.zip`. `gpUtils` not needed for this phase.

**A10**: ~~DEFERRED to Phase B~~ — Standard vs. IaaS merge is a UI/UX decision, will be addressed in the UX Pattern Map.

**A11**: ~~RESOLVED~~ — Residuo. La nuova app la ignora (non legge, non scrive). Colonna mantenuta nel DB per coesistenza, ma fuori scope.

**A12**: ~~RESOLVED~~ — Manteniamo il formato comma-separated per coesistenza con Appsmith (è il formato nativo dei MultiSelect Appsmith). Nessuna normalizzazione.

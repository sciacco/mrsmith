# Appsmith Audit: Quotes Application

**Source**: `quotes-main.tar.gz` (Appsmith Git export, file format v5, schema v11)
**Application name**: Quotes (icon: email, color: #FFEFDB)
**Theme**: Earth (system theme)
**Layout**: Desktop, sidebar navigation (light, text style, side orientation)
**Date**: 2026-04-09

---

## 1. Application Inventory

### Pages

| # | Page | Default | Hidden | Complexity | Purpose |
|---|---|---|---|---|---|
| 1 | Home | Yes | No | Trivial | Static splash page with decorative image |
| 2 | Elenco Proposte | No | No | Medium | Quote list with CRUD actions and navigation hub |
| 3 | Dettaglio | No | Yes | Very High | Full quote editor: header, kit rows, product config, HubSpot publish, e-signature |
| 4 | Nuova Proposta | No | No | High | 3-step wizard to create a new standard quote |
| 5 | Converti in ordine | No | Yes | Medium-High | Converts quote to order across 4 external systems |
| 6 | Nuova Proposta IaaS | No | No | Medium-High | 3-step wizard to create IaaS-specific quotes |

### Datasources

| Name | Plugin | Type | Used By |
|---|---|---|---|
| db-mistra | postgres-plugin | PostgreSQL | All pages (primary data store, `quotes.*`, `loader.*`, `products.*`, `common.*` schemas) |
| Alyante | mssql-plugin | MS SQL Server | Nuova Proposta, Nuova Proposta IaaS, Dettaglio (ERP customer payment codes, order names) |
| hubs | restapi-plugin | HubSpot CRM REST API | Elenco, Dettaglio, Nuova Proposta, Nuova Proposta IaaS, Converti in ordine |
| vodka | mysql-plugin | MySQL | Elenco (dead `Query1`), Converti in ordine (order code lookup) |
| GW internal CDLAN | restapi-plugin | Internal Gateway REST | Converti in ordine (PDF generation endpoint) |
| carbone.io | restapi-plugin | Carbone.io PDF API | Dettaglio (currently unused — no UI trigger) |

### External JS Libraries

| Name | Version | Purpose |
|---|---|---|
| xmlParser (fast-xml-parser) | 3.17.5 | XML parsing (CDN loaded). Usage not observed in active code paths. |

### Source Modules (Appsmith Packages)

| Module | Package | Version | Used By |
|---|---|---|---|
| gpUtils | ordini-gestione-portale | v1.0.3 | Converti in ordine (`gpUtils1.newOrderFromQuote`, `gpUtils1.rowsFromQuote`). Source code is external to this export. |

### Global Navigation Pattern

- Sidebar navigation (always visible, `navStyle: sidebar`)
- Hidden pages (Dettaglio, Converti in ordine) are accessible only via programmatic `navigateTo()`
- State passing: `appsmith.store.v_offer_id` for Dettaglio; URL query params for Converti in ordine
- All pages navigate back to `Elenco Proposte` (except one stale reference to `Elenco Offerte` on the Firma tab)

---

## 2. Page Audits

---

### 2.1 Home

**Purpose**: Static splash page. Displays a decorative image and the text "Questo e il tempo dell'attesa" ("This is the time of waiting"). No functionality.

**Widgets**: Container1 > Image1 (hardcoded URL `https://t.sciacco.net/x/concett-spaziale-attesa.jpg`) + Text1 (static bold text, theme-colored).

**Queries**: None. **JSObjects**: None. **Events**: None.

**Migration notes**: Trivial. Replace with a proper landing page or dashboard. Host image locally.

---

### 2.2 Elenco Proposte

**Purpose**: Main quote list. Displays all quotes in a paginated/sortable table. Provides actions: open (edit), create new, delete (role-gated), refresh, and convert to order.

#### Widgets

| Widget | Type | Role |
|---|---|---|
| Text1 | TEXT | Page title "Elenco proposte" (hardcoded blue #1d4ed8) |
| ButtonGroup1 | BUTTON_GROUP | Toolbar: Modifica, Nuova, Altro (dropdown: Duplica offerta [disabled], Aggiorna lista, Cancella offerta, Converti in ordine) |
| tbl_quote | TABLE_V2 | Main data table bound to `get_quotes.data` |

#### Key table columns

| Column | Visible | Notes |
|---|---|---|
| id | Hidden | PK |
| quote_number | Yes | "Numero" |
| document_date | Yes | Format: DD/MM/YYYY |
| cliente | Yes | Company name (from `loader.hubs_company` join) |
| deal_number | Yes | HubSpot deal number |
| deal_name | Yes | HubSpot deal name |
| owner_name | Yes | `first_name || ' ' || last_name` from `loader.hubs_owner` |
| status | Yes | Color-coded via `utils.bgStatus()`: DRAFT=default, PENDING_APPROVAL=orange, APPROVED=green, unknown=red |
| hs_quote_id | Hidden | Used for HubSpot deletion |
| customer_id | Hidden | Internal customer ID |

#### Queries

**get_quotes** (db-mistra, on load):
```sql
select q.id, q.quote_number, q.document_date, c.name as Cliente, q.status,
       q.deal_number, d.name as deal_name, q.created_at, q.updated_at,
       q.owner, own.first_name || ' ' || own.last_name as owner_name,
       q.hs_quote_id, q.customer_id
from quotes.quote q
    left join loader.hubs_company c on q.customer_id = c.id
    left outer join loader.hubs_deal d on q.hs_deal_id = d.id
    left outer join loader.hubs_owner own on q.owner = own.id
order by q.quote_number desc
limit 2000;
```

**Cancella_Offerta** (db-mistra, confirmBeforeExecute): `DELETE from quotes.quote where id = {{this.params.quote_id}}`

**Cancella_HS_Quote** (hubs, confirmBeforeExecute): `DELETE /crm/v3/objects/quotes/{{this.params.hs_quote_id}}`

#### JSObject: utils

**`bgStatus(status)`**: Maps quote status to cell background color.
```
DRAFT → '' (default) | PENDING_APPROVAL → 'orange' | APPROVAL_NOT_NEEDED → '' | APPROVED → 'green' | anything else → 'red'
```

**`eliminaOfferta()`**: Role-gated delete flow.
```js
async eliminaOfferta() {
  const ruoli = appsmith.user.roles;
  if (ruoli.find(ruolo =>
      ruolo === "Administrator - Sambuca" ||
      ruolo == "Kit and Products manager")) {
    // Authorized: delete HS quote (if exists), then DB record, then refresh
    if (tbl_quote.selectedRow.hs_quote_id > 0)
      await Cancella_HS_Quote.run({hs_quote_id: ...});
    await Cancella_Offerta.run({quote_id: ...});
    await get_quotes.run();
  } else {
    showAlert('Non disponi del permesso di cancellazione', 'error');
  }
}
```

#### Navigation

| Action | Destination | State |
|---|---|---|
| Modifica | Dettaglio | `storeValue('v_offer_id', selectedRow.id)` |
| Nuova | Nuova Proposta | `removeValue('defaulttab')` |
| Converti in ordine | Converti in ordine | URL params: `quote_id`, `quote_num`, `cliente`, `deal_number` |

#### Hidden Logic
- **Client-side RBAC**: Delete requires `"Administrator - Sambuca"` or `"Kit and Products manager"` — enforced only in JS, not server-side
- **Double confirmation dialogs**: Both `Cancella_HS_Quote` and `Cancella_Offerta` have `confirmBeforeExecute: true`
- **No atomicity**: If HS delete succeeds but DB delete is cancelled, data becomes inconsistent
- **Loose equality bug**: `==` used for `"Kit and Products manager"` vs `===` for the other role

#### Dead Code
- `nuovo_numero_offerta` (duplicate of `new_quote_number`)
- `hs_update_quote` (not called from this page)
- `hs_associa_contatto` (not called from this page)
- `Query1` (MySQL/vodka, completely unrelated)
- `contattiPerEsignature: {}` (declared, never used)
- "Duplica offerta" button (hardcoded `isDisabled: true`)

---

### 2.3 Dettaglio

**Purpose**: Full quote editing workspace. Loads a single quote via `appsmith.store.v_offer_id`. Five tabs: Dettagli (header form), Righe (kit rows + product config), Note (description + legal notes), Firma (e-signature, hidden tab), Riferimenti (contact references). Touches 4 external systems: Mistra PostgreSQL, Alyante SQL Server, HubSpot CRM, Carbone.io.

**Page is `isHidden: true`** — not in sidebar, reachable only via `navigateTo()`.

#### Tab: Dettagli — Quote Header Form (`frm_offerta`)

All fields default from `get_quote_by_id.data[0].*`.

| Widget | Label | Type | Default | Notes |
|---|---|---|---|---|
| sl_deal | Riferimento Deal | Select | `hs_deal_id` | |
| sl_owner | Owner | Select | `owner` | Source: `get_hs_owners.data` |
| sl_customer | Cliente | Select | `customer_id` | Source: `get_customers.data`, filterable |
| sl_status | Status | Select | HS status or DB status | **Always disabled** (read-only) |
| i_document_date | Data documento | DatePicker | `document_date` | |
| sl_type_document | Tipo documento | Select | `document_type` | TSC-ORDINE-RIC or TSC-ORDINE. **Disabled for IaaS/VCloud**. onChange: chains TypeDocument + Service |
| sl_proposal_type | Tipo di proposta | Select | `proposal_type` | NUOVO/SOSTITUZIONE/RINNOVO |
| sl_services | Servizi | MultiSelect | `services` | Excludes categories 12,13,14,15 (IaaS/VCloud). **Disabled for IaaS/VCloud** |
| sl_template | Template | Select | `template` | Dynamic source from `TypeDocument.template_suServizio`. **Disabled for IaaS/VCloud** |
| sl_payment_method | Modalita pagamento | Select | `payment_method` or `'402'` | Source: `loader.erp_metodi_pagamento` |
| sl_fatturazione_canoni_ | Fatturazione MRC | Select | `bill_months` | 1-24 months. Disabled for IaaS/VCloud or when COLOCATION selected |
| sl_mod_fatt_attivazione | Fatturazione NRC | Select | `nrc_charge_time` | All'attivazione (2) / All'ordine (1) |
| i_initial_term_months | Durata servizio (mesi) | Input(Number) | `initial_term_months` | Disabled for IaaS/VCloud |
| i_next_term_months | Rinnovi successivi (mesi) | Input(Text) | `next_term_months` | Disabled for IaaS/VCloud |
| i_replace_orders | Sostituisce ordine/i | MultiSelect | `replace_orders` | Only visible when SOSTITUZIONE |
| Button4 | Salva offerta | Button | | `mainForm.salvaOfferta()` |
| Button9 | Pubblica su Hubspot | Button | | `mainForm.mandaSuHubspot()`. Disabled when `sl_template` invalid |
| IconButton7 | Apri offerta su HS | IconButton | | Opens `hs_quote_link`. Disabled when link null or status != APPROVED |
| IconButton8 | Scarica PDF | IconButton | | Opens `hs_pdf_download_link`. Same disable condition |

#### Tab: Righe — Kit Row Management

**Left pane: `tbl_quote_rows`** — table of kit rows for this quote.

| Column | Editable | Notes |
|---|---|---|
| internal_name | No | Kit name |
| nrc_row | No | NRC total |
| mrc_row | No | MRC total |
| position | Yes (inline) | Row ordering, saved via `upd_quote_row_position` |

`onRowSelected` → `get_quote_products_grouped.run()` loads product groups.

**Right pane: `tbl_products`** — product groups for selected kit (visible only when kit row selected).

| Column | Notes |
|---|---|
| group_name | "Item" — product group/component name |
| inc_included | "Quotato": SI or -. **Red cell** when `required && !inc_included` |

**Detail form `frm_details`** — per-product editor (visible when a product group with options is selected):
- `s_productlistCopy` — product variant picker
- `i_nrcCopy`, `i_mrcCopy`, `i_quantityCopy` — pricing/quantity inputs
- `i_extended_descriptionCopy` — extended description
- `sw_includedCopy` — "Quotare?" switch. Label shows "(Obbligatorio)" when required
- `TXT_totalMrc` / `TXT_totalNrc` — computed totals (quantity x unit price)
- `Button8` — "Salva riga" → `detailForm.aggiornaRiga()`

**Modal `mdl_new_kit`** — add a new kit row:
- `sl_new_kit` — kit picker from `get_kit_internal_names.data`
- Confirm → `ins_quote_rows.run()` → refresh

**Toolbar**: Add kit (+), Delete kit (trash, with confirmation), Back to list

#### Tab: Note
- `i_description` — RTE: "Descrizione sommaria della proposta"
- `i_note_legali` — RTE: "Pattuizioni Speciali (note legali)". **Critical**: non-empty value triggers `PENDING_APPROVAL` status on HubSpot publish
- `trial_iaas` — Input (always disabled): IaaS trial text. Prepended to HS comments

#### Tab: Firma (hidden in tab bar)
- `sw_esignature` — Switch: "E-Signature attiva?"
- `Button10` — "Carica lista contatti" → `firmaForm.listaContatti()` (calls external `HS_utils1.ListCompanyContacts`)
- `msl_firmatari` — MultiSelect: signer picker. Pre-populated from DB (`hs_esign_contacts`)
- Text12/Text13 — display signer list and e-sign status

#### Tab: Riferimenti
Five groups of contact reference fields (name/phone/email):
- `rif_ordcli` (customer order reference)
- `rif_tech_nom/tel/email` (technical contact)
- `rif_altro_tech_nom/tel/email` (alternate technical)
- `rif_adm_nom/tel/email` (administrative contact)

#### Key Queries (35+ total)

**PostgreSQL (db-mistra):**

| Query | Purpose |
|---|---|
| `get_quote_by_id` | Load full quote header (`WHERE id = v_offer_id`) — on load |
| `get_quote_rows` | Load kit rows for quote — on load |
| `get_quote_products_grouped` | Product groups for selected kit row (JSONB lateral join) |
| `upd_quote` | `SELECT quotes.upd_quote_head({{dati}})` — stored procedure |
| `upd_quote_row_product` | `SELECT quotes.upd_quote_row_product({{dati}})` — stored procedure |
| `upd_quote_row_position` | Inline position update |
| `ins_quote_rows` | Insert new kit row |
| `del_quote_row` | Delete kit row (with confirmation) |
| `check_quote_rows` | Pre-publish validation: finds required products not included |
| `get_line_item_hs` | Complex query building bilingual descriptions via `common.get_short_translation()` + `string_agg` |
| `update_line_item_id` | Write back HS line item IDs to DB after sync |
| `get_kit_internal_names` | Active kit list for the add-kit modal — on load |
| `get_customers` | Company list — on load |
| `get_hs_owners` | Owner list — on load |
| `get_payment_method` | Payment methods — on load |
| `get_product_category` | Service categories (excluding 12,13,14,15) — on load |

**Alyante (MS SQL Server):**

| Query | Purpose |
|---|---|
| `cli_orders` | All confirmed/delivered order names (for SOSTITUZIONE multi-select) — on load |

**HubSpot REST API:**

| Query | Method | Purpose |
|---|---|---|
| `hs_get_quote_status` | GET | Fetch HS quote status, PDF link, signature info — on load |
| `hs_update_quote` | PATCH | Update HS quote properties and associations |
| `hs_get_quote_associations` | GET | Fetch current line items/contact associations |
| `hs_create_line_item` | POST | Create HS line item |
| `hs_update_line_item` | PATCH | Update HS line item |
| `hs_delete_line_item` | DELETE | Archive HS line item |
| `hs_set_quote_association` | PUT | Associate template to quote |
| `hs_associa_contatto` | PUT | Associate signer contact (type 702) |
| `hs_delete_contact_signer` | DELETE | Remove signer association (type 702) |

**Carbone.io:**

| Query | Purpose |
|---|---|
| `render_template` | PDF rendering (currently unused — no UI trigger) |

#### JSObjects (8 total)

**`mainForm`**:
- `salvaOfferta()` — core save: builds `updRecord` from all form fields, calls `upd_quote.run({dati: updRecord})`
- `mandaSuHubspot()` — 16-step publish flow: save → validate required products → sync line items → manage signers → update HS quote → re-save status to DB. Sets status to `PENDING_APPROVAL` if legal notes exist, else `APPROVED`

**`detailForm`**:
- `productOptionsTbl()` — product variant list for the selected group
- `getRigaDefault()` — returns the included product variant or index 0
- `changeProduct()` — updates NRC/MRC inputs when variant changes
- `aggiornaRiga()` — single-row save. Forces `mrc=0` for TSC-ORDINE. Forces `quantity=1` if included but quantity is 0
- `updateDetails()` — captures changes to in-memory `details[]` array

**`hs_utils`**:
- `hs_save_all_line_items()` — master bidirectional sync: delete orphaned HS items, update existing, create new. Uses position/group counters for `A)`, `B)` labels
- `upsert_line_item()` — create or update based on existing HS ID
- `save_hs_item(quote_row_id)` — builds MRC and NRC line items with bilingual names

**`firmaForm`**:
- `listaContatti()` — fetches contacts from HubSpot via external `HS_utils1` module
- `gestisciContattiEsignature()` — syncs signer contacts: removes deselected, adds new ones
- `loadContatti()` — pre-populates signer state from DB on page load
- `getSignaturePropertiesAsHTML()` — renders e-sign status table

**`templates`**:
- `lingua_template(template_id)` — maps template ID to "it" or "en"
- `terms_and_conditions()` — generates full HTML T&C for HS quote (`hs_terms`). Supports 6 variants: Non Colo IT/EN, Colo IT/EN, IaaS IT/EN
- `owner_data()` — finds current owner object

**`TypeDocument`**:
- `changeTypeDocument()` — enables/disables term/billing fields based on document type
- `template_suServizio()` — builds dynamic template list based on services + document type

**`Service`**:
- `ServiceChange()` — COLOCATION + TSC-ORDINE-RIC → force Trimestrale billing

**`utils`**:
- `includedField()`, `isIncluded()`, `isRequired()`, `currentRow()` — helpers for the product JSONB array

#### Hidden Logic (Critical)

1. **IaaS/VCloud template lock**: 8 hardcoded template IDs (`853027287235`, `850825381069`, `853500178641`, `853320143046`, `856380863697`, `855439340792`, `853237903587`, `853500899556`) disable multiple form fields. Embedded in 10+ widget `isDisabled` expressions.

2. **`salvaOfferta` template condition bug**: `if(template != "X" || template != "Y" || ...)` is always true (logical OR of not-equals). Works by accident because `else if` chain corrects the value.

3. **Missing closing quote in `isDisabled`**: `'853500899556 }}` (no closing `'`) in 5+ widget files. VCloud EN template may not trigger disabled state correctly.

4. **PENDING_APPROVAL trigger**: Non-empty `i_note_legali.text` → `PENDING_APPROVAL` instead of `APPROVED`. No UI indicator warns the user.

5. **ESIGN_COMPLETED blocks re-publish**: Signed quotes cannot be re-published. No visible UI state for this.

6. **E-signature auto-disable**: `sw_esignature` silently set to `false` if no signers are selected.

7. **MRC forced to 0 for spot orders**: `TSC-ORDINE` → `retObject.mrc = 0` in both `aggiornaRiga` and `updateDetails`.

8. **Colocation billing lock**: COLOCATION service → force Trimestrale (3) and disable billing selector.

9. **`check_quote_rows` blocks publish**: Required products not included → error alert, publish blocked.

10. **`btnEsci4` navigates to `'Elenco Offerte'`** (different page name than other back buttons which use `'Elenco Proposte'`)

#### Migration Notes

**Complexity: VERY HIGH**. Recommended migration order:
1. Read-only display + dropdowns (Tab: Dettagli)
2. Save quote header (`salvaOfferta` → `upd_quote_head`)
3. Kit row management (add, delete, reorder)
4. Product detail editing (`aggiornaRiga`)
5. Type/template/service business rules
6. HubSpot publish flow (highest risk, test independently)
7. E-signature management
8. Carbone.io PDF (currently unused)

---

### 2.4 Nuova Proposta

**Purpose**: 3-step wizard for creating a new standard quote. Step 1: select a HubSpot deal. Step 2: fill quote header metadata. Step 3: select kits + template. On save: creates HS quote, inserts DB record, inserts kit rows, navigates to Dettaglio.

#### Wizard Steps

**Step 1 — SelectPotential**: Table of active HubSpot deals from `get_potentials.data`. "Successivo" disabled when no deal selected.

**Step 2 — ConfirmGeneralData** (`frm_offerta`):

| Widget | Default | Notes |
|---|---|---|
| sl_deal | Selected deal ID | |
| sl_owner | Deal's owner_id | |
| sl_customer | Deal's company_id | |
| i_document_date | Today | |
| sl_status | "DRAFT" | Always disabled |
| sl_type_document | "TSC-ORDINE-RIC" | onChange: chains TypeDocument + Service |
| sl_proposal_type | "NUOVO" | NUOVO/SOSTITUZIONE/RINNOVO |
| sl_services | (from product_category) | Multi-select. Excludes categories 12,13. Required |
| sl_payment_method | From Alyante ERP (default 402) | |
| sl_fatturazione_canoni | Bimestrale (2) | Disabled for spot or COLOCATION |
| sl_mod_fatt_attivazione | All'attivazione (2) | |
| i_initial_term_months | "12" | Disabled for spot |
| i_next_term_months | "12" | Disabled for spot |
| i_delivered_in_days | "60" | |
| i_replace_orders | (from Alyante orders) | Only active when SOSTITUZIONE |
| id_alyante_cli | (hidden) | Alyante customer number — cross-system ID bridge |

**Step 3 — SelectKits** (`Form1`):

| Widget | Notes |
|---|---|
| mst_kit | Multi-select tree (categories negative, kits positive) |
| sl_template | Dynamic from TypeDocument (Non Colo IT/EN, Colo IT/EN) |
| i_description | RTE: optional quote description |
| i_note_legali | RTE: optional legal notes (appended to hs_terms) |

#### Save Sequence (`utils.salvaOfferta()`)

1. `new_quote_number.run()` → `common.new_document_number('SP-')`
2. `templates.terms_and_conditions()` → build HTML T&C
3. Compute `expire_date = today + 30 days`
4. Build `dati_hs` for HubSpot (title, status DRAFT, language, sender, terms, expiry, domain `content.cdlan.it`)
5. Build associations: template (286), deal (64), company (71)
6. `new_hs_quote.run()` → `POST /crm/v3/objects/quote`
7. Build `updRecord` with all header fields
8. `ins_quote.run({dati: updRecord})` → `SELECT quotes.ins_quote_head(...)`
9. For each selected kit (value > 0): `ins_quote_rows.run({quote_id, kit_id})`
10. `storeValue('v_offer_id', quote_id)` → `navigateTo('Dettaglio')`

#### Key Queries

| Query | Datasource | SQL |
|---|---|---|
| `get_potentials` | db-mistra | Deals filtered by hardcoded pipeline IDs (`255768766`, `255768768`) and stage IDs. `codice <> ''` |
| `get_potential_by_id` | db-mistra | Single deal detail with `numero_azienda` for Alyante link |
| `get_pagamento_anagrCli` | Alyante | `isnull(CAST(CODICE_PAGAMENTO as INT), 402)` — default payment from ERP |
| `cli_orders` | Alyante | All confirmed/delivered orders (not customer-filtered) |
| `ins_quote` | db-mistra | `SELECT quotes.ins_quote_head({{dati}})` stored procedure |
| `new_hs_quote` | hubs | `POST /crm/v3/objects/quote` |

#### JSObjects (5 total)

- **utils**: `onPageLoad()`, `treeOfKits()` (categories negative, kits positive), `metodoPagDefault()`, `salvaOfferta()`, `newQuoteAssociations()` (deal branch dead: `if (false && ...)`), `test_hs2()` (dead)
- **TypeDocument**: `TypeDocumentChange()` (field enable/disable), `template_suServizio()` (template list by type)
- **Service**: `ServiceChange()` (COLOCATION → Trimestrale)
- **checkValori**: `spot_template()` — blocks save if Colo template selected for spot document
- **templates**: `lista_templates`, `vocabolario_it/en`, `lingua_template()`, `terms_and_conditions()` (6 variants: Non Colo/Colo × IT/EN)

#### Hidden Logic

- **Hardcoded pipeline/stage IDs in SQL**: Pipelines `255768766`, `255768768` with specific stage whitelists. New pipelines/stages require SQL change.
- **Category exclusion**: IDs 12, 13 always excluded from `get_product_category`
- **Kit ecommerce exclusion**: `ecommerce = false` filter on `list_kit`
- **Alyante payment fallback**: Code 402 is the system default (hardcoded in both SQL and JS)
- **Template IDs hardcoded in 3 places**: `TypeDocument.template_suServizio()`, `templates.lingua_template()`, `templates.terms_and_conditions()`
- **`i_description` length==1 check**: Suppresses empty RTE artifact (`"\n"`)
- **Negative category values**: Workaround for Appsmith tree widget including parent values in selection
- **COLOCATION forces Trimestrale billing**: Business rule in `Service.ServiceChange()`
- **Status permanently DRAFT**: No path to create in any other status
- **`ragione_sociale: null`**: Always null from this page
- **Dead code**: `inserisci_righe = false` block (HS line item creation moved to Dettaglio page per inline comment)

#### Migration Risks

1. **3 databases in one page**: Mistra PostgreSQL, Alyante MSSQL, HubSpot REST
2. **`ins_quote_head` stored procedure**: Parameter type (JSON/JSONB/composite) unknown from export
3. **Alyante payment query fires on page load** with empty `id_alyante_cli` (spurious)
4. **`i_next_term_months` type mismatch**: declared TEXT, sibling is NUMBER
5. **HubSpot expiry date**: `new Date(currentDate.setDate(...))` mutates `currentDate` — fragile
6. **No cross-tab validation**: Services set on Tab 2 aren't re-validated after Tab 3 entry

---

### 2.5 Converti in ordine

**Purpose**: Confirmation gate for converting a quote to an order. Receives context via URL query params (`quote_id`, `quote_num`, `cliente`, `deal_number`). Orchestrates a 10-step multi-system workflow.

**Page is `isHidden: true`**.

#### Widgets

| Widget | Type | Role |
|---|---|---|
| Text1 | TEXT | Header: "Conversione in ordine della Proposta {quote_num}, cliente {cliente}" |
| Text2 | TEXT | "Confermi la creazione di un nuovo ordine?" |
| Image1 | IMAGE | Decorative (`https://t.sciacco.net/x/sambuca_to_gb.jpg`) |
| Button1 | BUTTON | "Confermo" — **hidden** (`isVisible: false`). Old flow (`utils.orchestra()`). Dead. |
| Button1Copy1 | BUTTON | "Genera Ordine e invia ad Hubspot" — **active** flow (`utilsCopy.g_orchestra()`) |
| Button1Copy | BUTTON | "Annulla" — navigates to `Elenco Offerte` |

#### Active Flow (`utilsCopy.g_orchestra()`)

1. Read `deal_number` from URL params
2. Split `deal_number` → `cdlan_ndoc` / `cdlan_anno`
3. `get_order_code.run()` — check for existing order (**guard is commented out**)
4. `GetDealIdByCodice.run()` → get HubSpot `dealId`
5. `get_quote_by_id.run()` → read `document_type`
6. `gpUtils1.newOrderFromQuote(quoteId)` → create order in Vodka
7. `DownloadOrderPDFgwint.run()` → fetch PDF from GW internal API
8. Build Appsmith file object from PDF response
9. `UploadFile.run()` → upload PDF to HubSpot Files (`/files/v3/files`)
10. `CreateNoteWithAttachment.run()` → create CRM note with PDF linked to deal
11. Navigate to HubSpot deal page (`https://app-eu1.hubspot.com/contacts/26622471/record/0-3/{dealId}`)

#### Queries

| Query | Datasource | Purpose |
|---|---|---|
| get_quote_by_id | db-mistra | Fetch quote details |
| get_order_code | vodka (MySQL) | Check for existing order by deal code |
| GetDealIdByCodice | db-mistra | Resolve deal_number to HubSpot deal ID |
| DownloadOrderPDFgwint | GW internal CDLAN | `GET /orders/v1/order/pdf/{orderId}/generate` |
| UploadFile | hubs | `POST /files/v3/files` (multipart/form-data) |
| AssociateFileToDeal | hubs | `PUT /crm/v3/objects/deals/{dealId}/associations/files/{fileId}/deal_to_file` — **has a bug and is not called in active flow** |
| CreateNoteWithAttachment | hubs | `POST /crm/v3/objects/notes` (with associations type 214) |

#### Hidden Logic / Bugs

- **Duplicate order guard disabled**: `get_order_code` result is fetched but the blocking condition is commented out
- **`AssociateFileToDeal` bug**: Path uses `GetDealIdByCodice.data.codice` but query returns `deal_id`. Dormant (not called).
- **Month off-by-one**: `date.getMonth()` returns 0-based month in PDF filename
- **Hardcoded HubSpot portal ID**: `26622471` in navigation URL
- **Raw PDF piped to UploadFile**: Fragile — `b64OrBinaryToAppsmithFile` helper is defined but bypassed
- **No error recovery**: Partial state if mid-chain failure (order created in Vodka but PDF not uploaded)
- **External module dependency**: `gpUtils1` source code not in export

---

### 2.6 Nuova Proposta IaaS

**Purpose**: 3-step wizard for IaaS-specific quotes. Step 1: select deal. Step 2: IaaS-specific metadata (template drives kit selection automatically). Step 3: confirm kit + description + legal notes. On save: creates HS quote + DB record + kit row → navigates to Dettaglio.

#### Key Differences from Nuova Proposta

| Aspect | Nuova Proposta | Nuova Proposta IaaS |
|---|---|---|
| Kit selection | Manual multi-select tree | Auto-derived from template (1:1 template→kit mapping) |
| Services | User-selected multi-select | Auto-derived from template (hardcoded switch) |
| Template source | `quotes.template` filtered by document type | `quotes.template` filtered by language + `like 'IaaS%' OR 'VCLOUD%'` |
| Term fields | Editable (unless spot) | All disabled with hardcoded defaults (1 month) |
| Trial | Not available | Slider 0-200, generates bilingual trial text |
| `document_type` | User-selectable | Hardcoded `"TSC-ORDINE-RIC"` |
| Language | Derived from template | User-selectable (`cli_lang`), used to filter templates |

#### Template → Kit Mapping (hardcoded in `recuperaServizio()`)

| Template ID | Template Name | Kit ID | Services |
|---|---|---|---|
| 853027287235 | IaaS Diretta IT | 62 | [12] |
| 850825381069 | IaaS Diretta EN | 62 | [12] |
| 853500178641 | IaaS Indiretta IT | 63 | [13] |
| 853320143046 | IaaS Indiretta EN | 63 | [13] |
| 853237903587 | IaaS Vcloud IT | 116 | [14] |
| 853500899556 | IaaS Vcloud EN | 116 | [14] |
| 856380863697 | DRaaS Vcloud IT | 119 | [15] |
| 855439340792 | DRaaS Vcloud EN | 119 | [15] |

#### Key Queries Unique to This Page

| Query | Purpose |
|---|---|
| `get_templates` | `WHERE lang = substr(LOWER(cli_lang),1,2) AND (description like 'IaaS%' OR description like 'VCLOUD%')`. `left(description, -3)` strips language suffix. |
| `get_deals` | Same hardcoded pipeline/stage filter as Nuova Proposta |
| `get_templates_byid` | Confirms template language in salvaOfferta |

#### JSObject: `creazioneProposta`
- `recuperaServizio()` — switch mapping template ID → kit ID
- `recuperaServizioArray()` — switch mapping template ID → services array string
- `recuperaLingua()` — **bug**: `!= '' || != null` is tautology (always true), default `"ITA"` unreachable
- `recuperaTrial()` — generates bilingual trial text from slider value
- `metodoPagDefault()` — same pattern as Nuova Proposta (Alyante fallback 402)
- `salvaOfferta()` — same pattern: HS quote + `ins_quote_head` + single `ins_quote_rows`

#### Hidden Logic / Bugs

- **`recuperaLingua()` bug**: `||` should be `&&` — default "ITA" fallback never triggers
- **`cli_orders` not scoped to customer**: Shows all Alyante orders regardless of customer
- **All term fields disabled**: IaaS quotes have fixed 1-month terms. Confirm with business.
- **`hs_sender_email` bug in `salvaOfferta`**: Uses `owner.selectedOptionLabel` on a plain object (not a widget) → `undefined`
- **Server-side pagination scaffolding**: 4 auto-generated CRUD queries are dead code
- **7 queries fire on page load** across 3 database systems

---

## 3. Datasource and Query Catalog

### 3.1 PostgreSQL (db-mistra) — Primary Data Store

#### Schemas Used

| Schema | Purpose |
|---|---|
| `quotes` | Quote heads, rows, products, templates. Stored procs: `ins_quote_head`, `upd_quote_head`, `upd_quote_row_product`. Views: `v_quote_rows_for_hs`, `v_quote_rows_products`, `v_quote_products_grouped`. |
| `products` | Product catalog: `product_category`, `kit` |
| `loader` | HubSpot ETL mirror tables: `hubs_company`, `hubs_deal`, `hubs_owner`, `hubs_pipeline`, `hubs_stages` + ERP mirror: `erp_metodi_pagamento` |
| `common` | Shared utilities: `new_document_number('SP-')` function, `get_short_translation()` function |

#### All Unique Queries

| Query | Page(s) | Type | Rewrite |
|---|---|---|---|
| `get_quotes` | Elenco | SELECT (joins quotes + loader) | Backend API endpoint |
| `get_quote_by_id` | Dettaglio, Converti | SELECT | Backend API endpoint |
| `get_quote_rows` | Dettaglio | SELECT | Backend API endpoint |
| `get_quote_products_grouped` | Dettaglio | SELECT (JSONB lateral join) | Backend API endpoint |
| `get_quote_rows_products` | Dettaglio | SELECT | Backend API endpoint |
| `get_line_item_hs` | Dettaglio | SELECT (complex, bilingual) | Backend API endpoint |
| `check_quote_rows` | Dettaglio | SELECT (validation) | Backend validation logic |
| `get_products_for_hs` | Dettaglio, Nuova Proposta | SELECT (view) | Backend API endpoint |
| `get_kit_internal_names` | Dettaglio | SELECT | Backend API endpoint |
| `get_customers` | Dettaglio | SELECT | Backend API endpoint |
| `get_hs_owners` | Dettaglio | SELECT | Backend API endpoint |
| `get_payment_method` | All creation pages | SELECT | Backend API endpoint |
| `get_product_category` | Dettaglio, Nuova Proposta | SELECT (excl 12,13[,14,15]) | Backend API endpoint |
| `get_potentials` / `get_deals` | Nuova Proposta, IaaS | SELECT (hardcoded pipelines) | Backend API endpoint |
| `get_potential_by_id` / `get_deals_by_id` | Nuova Proposta, IaaS | SELECT | Backend API endpoint |
| `get_owners` / `get_deal_owner` | Nuova Proposta, IaaS | SELECT | Backend API endpoint |
| `get_templates` | IaaS | SELECT (IaaS/VCloud filter) | Backend API endpoint |
| `get_templates_byid` | IaaS | SELECT | Backend API endpoint |
| `list_kit` | Nuova Proposta | SELECT | Backend API endpoint |
| `new_quote_number` | Nuova Proposta, IaaS | `common.new_document_number('SP-')` | Backend service |
| `ins_quote` | Nuova Proposta, IaaS | `quotes.ins_quote_head(dati)` stored proc | Backend service |
| `ins_quote_rows` | All creation + Dettaglio | INSERT | Backend service |
| `upd_quote` | Dettaglio | `quotes.upd_quote_head(dati)` stored proc | Backend service |
| `upd_quote_row_product` | Dettaglio | `quotes.upd_quote_row_product(dati)` stored proc | Backend service |
| `upd_quote_row_position` | Dettaglio | UPDATE position | Backend service |
| `del_quote_row` | Dettaglio | DELETE | Backend service |
| `Cancella_Offerta` | Elenco | DELETE | Backend service (with RBAC) |
| `update_line_item_id` | Dettaglio | UPDATE hs_line_item_id | Backend service |
| `GetDealIdByCodice` | Converti | SELECT | Backend service |

### 3.2 MS SQL Server (Alyante) — ERP Lookups

| Query | Page(s) | SQL | Rewrite |
|---|---|---|---|
| `get_pagamento_anagrCli` | Nuova Proposta, IaaS | Customer payment code from `Tsmi_Anagrafiche_clienti` (default 402) | Backend ERP proxy |
| `cli_orders` | All creation + Dettaglio | Order names from `Tsmi_Ordini` (Evaso/Confermato) | Backend ERP proxy (should filter by customer) |

### 3.3 HubSpot REST API (hubs)

| Query | Method | Endpoint | Page(s) | Rewrite |
|---|---|---|---|---|
| `hs_get_quote_status` | GET | `/crm/v3/objects/quotes/{id}` | Dettaglio | Backend HS proxy |
| `hs_update_quote` | PATCH | `/crm/v3/objects/quotes/{id}` | Dettaglio, Elenco | Backend HS proxy |
| `hs_get_quote_associations` | GET | `/crm/v3/objects/quotes/{id}?associations=...` | Dettaglio | Backend HS proxy |
| `new_hs_quote` | POST | `/crm/v3/objects/quote` | Nuova Proposta, IaaS | Backend HS proxy |
| `hs_create_line_item` | POST | `/crm/v3/objects/line_item` | Dettaglio | Backend HS proxy |
| `hs_update_line_item` | PATCH | `/crm/v3/objects/line_item/{id}` | Dettaglio | Backend HS proxy |
| `hs_delete_line_item` | DELETE | `/crm/v3/objects/line_item/{id}` | Dettaglio | Backend HS proxy |
| `hs_set_quote_association` | PUT | `/crm/v4/objects/quotes/{id}/associations/...` | Dettaglio | Backend HS proxy |
| `hs_delete_quote_association` | DELETE | `/crm/v4/objects/quotes/{id}/associations/...` | Dettaglio (unused) | — |
| `hs_associa_contatto` | PUT | `/crm/v4/objects/quote/{id}/associations/contact/{id}` | Dettaglio | Backend HS proxy |
| `hs_delete_contact_signer` | DELETE | `/crm/v4/objects/quote/{id}/associations/contact/{id}` | Dettaglio | Backend HS proxy |
| `Cancella_HS_Quote` | DELETE | `/crm/v3/objects/quotes/{id}` | Elenco | Backend HS proxy |
| `UploadFile` | POST | `/files/v3/files` (multipart) | Converti | Backend HS proxy |
| `AssociateFileToDeal` | PUT | `/crm/v3/objects/deals/{id}/associations/...` | Converti (unused/buggy) | — |
| `CreateNoteWithAttachment` | POST | `/crm/v3/objects/notes` | Converti | Backend HS proxy |

### 3.4 MySQL (vodka)

| Query | Page(s) | Purpose |
|---|---|---|
| `get_order_code` | Converti | Check for existing order by deal code |
| `Query1` | Elenco (dead) | `SELECT * FROM orders` — never wired |

### 3.5 GW Internal CDLAN (REST)

| Query | Page | Endpoint |
|---|---|---|
| `DownloadOrderPDFgwint` | Converti | `GET /orders/v1/order/pdf/{orderId}/generate` |

### 3.6 Carbone.io (REST)

| Query | Page | Endpoint |
|---|---|---|
| `render_template` | Dettaglio (unused) | `POST /render/{templateId}` |

---

## 4. Findings Summary

### 4.1 Embedded Business Rules

| Rule | Location | Classification |
|---|---|---|
| Status color mapping (DRAFT/PENDING_APPROVAL/APPROVED) | Elenco: `utils.bgStatus()` | Presentation |
| Delete requires "Administrator - Sambuca" or "Kit and Products manager" | Elenco: `utils.eliminaOfferta()` | Business logic (must move to backend RBAC) |
| Pipeline/stage filtering for active deals | Nuova Proposta, IaaS: `get_potentials` SQL | Business logic |
| Product category exclusion (12, 13, [14, 15]) | Multiple: `get_product_category` SQL | Business logic |
| Kit ecommerce exclusion | Nuova Proposta: `list_kit` SQL | Business logic |
| Default payment code 402 | Multiple: SQL `isnull(..., 402)` + JS fallback | Business logic |
| COLOCATION → Trimestrale billing | Dettaglio, Nuova Proposta: `Service.ServiceChange()` | Business logic |
| Spot orders → MRC = 0 | Dettaglio: `detailForm.aggiornaRiga()` | Business logic |
| Non-empty legal notes → PENDING_APPROVAL | Dettaglio: `mainForm.mandaSuHubspot()` | Business logic |
| Required products must be included before publish | Dettaglio: `check_quote_rows` validation | Business logic |
| E-signature disabled if no signers selected | Dettaglio: `firmaForm.gestisciContattiEsignature()` | Business logic |
| Signed quotes cannot be re-published | Dettaglio: `mainForm.mandaSuHubspot()` | Business logic |
| IaaS/VCloud template → lock most form fields | Dettaglio: widget `isDisabled` expressions | Business logic |
| Template → kit mapping for IaaS | IaaS: `creazioneProposta.recuperaServizio()` | Business logic |
| Document type controls term/billing field state | Multiple: `TypeDocument.changeTypeDocument()` | Business logic |
| Template list varies by document type + services | Multiple: `TypeDocument.template_suServizio()` | Business logic |
| Colo template blocked for spot documents | Nuova Proposta: `checkValori.spot_template()` | Business logic |
| Quote number generation with SP- prefix | Multiple: `common.new_document_number('SP-')` | Business logic |
| HubSpot quote expiry = document date + 30 days | Multiple: `salvaOfferta()` | Business logic |
| T&C generation (6 variants: Non Colo/Colo/IaaS × IT/EN) | Multiple: `templates.terms_and_conditions()` | Business logic |
| `replace_orders` serialized with `;` separator | Multiple: `salvaOfferta()` | Data format |
| Order replacement only for SOSTITUZIONE proposals | Multiple: widget conditional visibility/required | Business logic |

### 4.2 Duplication

| Item | Pages | Notes |
|---|---|---|
| `get_product_category` query | Dettaglio, Nuova Proposta | Nearly identical (Dettaglio excludes 14,15 additionally) |
| `get_payment_method` query | All creation pages + Dettaglio | Identical SQL |
| `get_potentials` / `get_deals` | Nuova Proposta, IaaS | Same SQL with different query names |
| `new_quote_number` / `nuovo_numero_offerta` | Elenco, Nuova Proposta, IaaS | Exact duplicate on Elenco page |
| `ins_quote` / `ins_quote_rows` | Nuova Proposta, IaaS | Identical |
| `templates.terms_and_conditions()` | Dettaglio, Nuova Proposta (standard T&C) + IaaS (IaaS-specific T&C) | Different implementations with overlapping structure |
| `templates.lingua_template()` | Dettaglio, Nuova Proposta | Same logic, different IDs for IaaS |
| `TypeDocument.changeTypeDocument()` | Dettaglio, Nuova Proposta | Same logic |
| `Service.ServiceChange()` | Dettaglio, Nuova Proposta | Same logic |
| `utils.metodoPagDefault()` | Nuova Proposta, IaaS | Same logic |
| `cli_orders` query | All pages with SOSTITUZIONE | Identical Alyante query (not customer-filtered) |
| HubSpot template IDs | 10+ locations | 8 IaaS/VCloud IDs scattered across widgets and JSObjects |
| 4 standard template IDs | 3+ locations | Non Colo IT/EN, Colo IT/EN hardcoded in multiple JSObjects |

### 4.3 Security Concerns

| Issue | Location | Severity |
|---|---|---|
| Client-side RBAC for delete | Elenco: `eliminaOfferta()` | **High** — no server-side enforcement |
| Direct DB DELETE with no soft-delete | Elenco: `Cancella_Offerta` | Medium — no audit trail |
| HubSpot API calls from browser | All pages | Medium — API keys exposed to Appsmith runtime (managed by Appsmith datasource config) |
| Non-atomic dual-system delete | Elenco: HS delete + DB delete | Medium — inconsistent state on partial failure |
| Non-atomic multi-system write | Nuova Proposta, Converti: HS + DB + Vodka | Medium — orphaned records on partial failure |
| `cli_orders` shows all orders | Nuova Proposta, IaaS, Dettaglio | Low — user can see order names from other customers |

### 4.4 Migration Blockers

| Blocker | Impact | Resolution |
|---|---|---|
| Stored procedures: `ins_quote_head`, `upd_quote_head`, `upd_quote_row_product` | Cannot replicate write logic without procedure source | Extract procedure DDL from `quotes` schema |
| Views: `v_quote_rows_for_hs`, `v_quote_rows_products`, `v_quote_products_grouped` | Cannot replicate read logic without view definitions | Extract view DDL from `quotes` schema |
| Function: `common.new_document_number('SP-')` | Cannot replicate numbering without function source | Extract function DDL |
| Function: `common.get_short_translation()` | Used in bilingual line item descriptions | Extract function DDL |
| External module: `gpUtils` (ordini-gestione-portale v1.0.3) | Order creation logic (`newOrderFromQuote`, `rowsFromQuote`) | Obtain module source or API documentation |
| External module: `HS_utils1.ListCompanyContacts` | E-signature contact loading | Obtain module source or API documentation |
| Alyante connection details | MSSQL server, credentials, VPN requirements | Obtain from infrastructure team |
| HubSpot API authentication | OAuth/API key configuration | Obtain from HubSpot admin |
| GW internal CDLAN base URL | Internal REST API for order PDF generation | Obtain from infrastructure team |
| Carbone.io template IDs | Only `colo_ita` template ID hardcoded in unused code | Obtain full template registry if PDF feature needed |

### 4.5 Bugs Found

| Bug | Page | Severity |
|---|---|---|
| `salvaOfferta` template condition: `if(template != "X" \|\| template != "Y")` is always true | Dettaglio | Low (works by accident via else-if chain) |
| Missing closing quote in `isDisabled`: `'853500899556 }}` | Dettaglio (5+ widgets) | Medium (VCloud EN may not trigger disabled state) |
| `recuperaLingua()`: `!= '' \|\| != null` is tautology | IaaS | Medium (default "ITA" unreachable) |
| `hs_sender_email: owner.selectedOptionLabel` on plain object | IaaS `salvaOfferta` | Medium (always `undefined`) |
| `AssociateFileToDeal` path uses `data.codice` instead of `data[0].deal_id` | Converti (dormant) | Low (query not called) |
| Month off-by-one in PDF filename: `date.getMonth()` is 0-based | Converti | Low (cosmetic) |
| `==` vs `===` inconsistency in role check | Elenco | Low (harmless in practice) |
| Duplicate order guard commented out | Converti | Medium (allows duplicate order creation) |

### 4.6 Candidate Domain Entities

| Entity | Primary Table/Schema | Notes |
|---|---|---|
| Quote (head) | `quotes.quote` | Central entity with ~40 fields |
| Quote Row (kit) | `quotes.quote_rows` | Kit instances attached to a quote, ordered by position |
| Quote Row Product | `quotes.quote_rows_products` | Product options within a kit, with JSONB `riga` array |
| Kit | `products.kit` | Product kit catalog (active, non-ecommerce) |
| Product Category | `products.product_category` | Service categories (some excluded by ID) |
| Template | `quotes.template` | HubSpot quote template registry |
| Customer | `loader.hubs_company` | HubSpot company mirror |
| Deal | `loader.hubs_deal` | HubSpot deal mirror (pipelines, stages) |
| Owner | `loader.hubs_owner` | HubSpot owner mirror |
| Payment Method | `loader.erp_metodi_pagamento` | ERP payment method mirror |
| Order (Alyante) | `Tsmi_Ordini` (via Alyante) | ERP orders for SOSTITUZIONE reference |

### 4.7 Recommended Next Steps

1. **Extract database DDL**: Stored procedures, views, and functions from `quotes`, `common`, and `products` schemas are migration-critical. Without them, the write logic cannot be replicated.

2. **Obtain external module source**: `gpUtils` (ordini-gestione-portale) and `HS_utils1` modules contain logic not available in this export.

3. **Centralize hardcoded IDs**: HubSpot template IDs (12 total), pipeline IDs (2), stage IDs (9), product category exclusion IDs (4), and kit IDs (4 for IaaS) should move to database configuration tables.

4. **Implement server-side RBAC**: Delete authorization is currently client-side only. All mutations must be gated by backend role checks.

5. **Add transactional safety**: Multi-system writes (HS + DB, HS + DB + Vodka) need saga patterns or at minimum idempotency guards. The commented-out duplicate order check should be restored.

6. **Fix known bugs**: Template condition logic, missing quotes in isDisabled, `recuperaLingua` tautology, sender email on plain object.

7. **Design backend API surface**: All 30+ direct DB queries should be consolidated into ~15 backend API endpoints with proper authorization, validation, and error handling.

8. **Preserve T&C generation logic**: `templates.terms_and_conditions()` contains significant business knowledge about billing terms in 6 language/service variants. Migrate verbatim.

9. **Plan Dettaglio page migration in phases**: This page alone accounts for ~60% of the application's complexity. Migrate in the order: display → save header → kit management → product editing → business rules → HubSpot publish → e-signature.

10. **Scope the `cli_orders` query by customer**: Current implementation shows all Alyante orders regardless of customer — add `NUMERO_AZIENDA` filter.

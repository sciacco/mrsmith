# Zammu — Appsmith Application Audit

> **Source:** `apps/zammu/zammu-main.zip`
> **Audit date:** 2026-04-09
> **Appsmith format version:** 5 (server schema 11, client schema 2)
> **Theme:** Pacific (system theme)
> **Layout:** FLUID, sidebar navigation, light color style

---

## 1. Application Inventory

### 1.1 Application Name

**Zammu** (zammu-main)

### 1.2 Pages

| # | Page | Slug | Default | Hidden | Purpose |
|---|------|------|---------|--------|---------|
| 1 | Home | home | Yes | No | Static welcome/landing page |
| 2 | Coperture | coperture | No | No | Network coverage lookup (address → available profiles) |
| 3 | Energia variabile | energia-variabile | No | No | Variable energy billing: rack power readings, kW charts, billing charges |
| 4 | Transazioni whmcs | transazioni-whmcs | No | **Yes** | WHMCS paid transaction viewer (hidden page) |
| 5 | IaaS calcolatrice | iaas-calcolatrice | No | No | IaaS daily pricing calculator with PDF export |

> **Note:** `application.json` lists "Home" twice (indices 0 and 1). This is likely an Appsmith artifact; there is only one Home page directory.

### 1.3 Datasources

| Name | Plugin | Type | Used by Pages |
|------|--------|------|---------------|
| dbcoperture | postgres-plugin | PostgreSQL | Coperture |
| grappa | mysql-plugin | MySQL | Energia variabile |
| whmcs_prom | mysql-plugin | MySQL | Transazioni whmcs |
| transazioni-whmcs | graphql-plugin | GraphQL | Transazioni whmcs (unused stub) |
| carbone.io | restapi-plugin | REST API | IaaS calcolatrice |

### 1.4 JS Libraries

| Library | Version | CDN | Accessor | Used by |
|---------|---------|-----|----------|---------|
| ExcelJS | 4.3.0 | jsdelivr | `ExcelJS`, `regeneratorRuntime` | Not visibly referenced in any page — likely leftover |
| fast-xml-parser | 3.17.5 | cdnjs | `xmlParser` | Not visibly referenced in any page — likely leftover |

### 1.5 Global Notes

- Navigation is sidebar-based (`navStyle: "sidebar"`, `orientation: "side"`)
- No cross-page shared JSObjects — each page has its own `utils` (Coperture, Energia variabile, IaaS calcolatrice)
- No global app-level state management beyond Appsmith's built-in `appsmith.store`
- Three completely independent datasources serving three distinct domains

---

## 2. Page Audits

---

### 2.1 Home

**Purpose:** Static welcome/landing page with user greeting. No data, no queries.

**Widgets:**

| Widget | Type | Role |
|--------|------|------|
| IconButton1 | ICON_BUTTON_WIDGET | Person icon (top-left), no onClick — decorative |
| Text4 | TEXT_WIDGET | `"Hello {{appsmith.user.name \|\| appsmith.user.email}}"` — user greeting |
| Text5 | TEXT_WIDGET | `"Questo spazio è stato lasciato vuoto intenzionalmente"` — placeholder (3rem bold) |

**Event Flow:** None. No queries, no navigation actions.

**Hidden Logic:** None.

**Migration Notes:**
- Trivial page. Replace with portal home/dashboard in the React app.
- The user identity comes from `appsmith.user` — in the rewrite this maps to the Keycloak token.

---

### 2.2 Coperture

**Purpose:** Look up commercial network coverage profiles for a given physical address. Users drill down through cascading selects (Provincia → Comune → Indirizzo → Numero civico) and then search for available coverage.

**Datasource:** `dbcoperture` (PostgreSQL), schema `coperture`

#### Queries

| Query | SQL | Runs On | Depends On | Prepared |
|-------|-----|---------|------------|----------|
| get_states | `SELECT coperture.get_states()` | Page load | — | No |
| get_cities | `SELECT id, name, network_coverage_state_id FROM coperture.network_coverage_cities WHERE network_coverage_state_id = {{parseInt(s_states.selectedOptionValue)}} ORDER BY name` | s_states change | s_states | Yes |
| get_addresses | `SELECT id, name FROM coperture.network_coverage_addresses WHERE network_coverage_city_id = {{parseInt(s_city.selectedOptionValue)}} ORDER BY name` | s_city change | s_city | Yes |
| get_house_numbers | `SELECT id, name FROM coperture.network_coverage_house_numbers WHERE network_coverage_address_id = {{parseInt(s_address.selectedOptionValue)}} ORDER BY name` | s_address change | s_address | Yes |
| get_coverage | `SELECT * FROM coperture.v_get_coverage v WHERE v.house_number_id = {{parseInt(s_housenumber.selectedOptionValue)}} ORDER BY operator, tech` | Button1 click | s_housenumber | Yes |
| get_details_types | `SELECT coperture.get_coverage_details_types()` | Page load | — | Yes |

#### JSObject: `utils`

| Method | Purpose | Classification |
|--------|---------|----------------|
| `formatProfili(p)` | Renders profile names as HTML `<table>` | Presentation |
| `formatDettagli(p)` | Renders detail items as HTML `<table><ul>` with detail-type name lookup | Presentation + business logic (type name resolution) |
| `getImageUrl(o)` | Maps operator_id → operator logo URL (1=TIM, 2=Fastweb, 3=OpenFiber, 4=OpenFiber CD) | Business logic (operator identity) |
| `getDetailName(i)` | Looks up detail type name from `get_details_types.data` by id | Business logic |
| `updateTestoRicerca()` | Builds breadcrumb string from selected labels and sets `t_ricerca` text | UI orchestration |
| `test()` | Debug/test function with hardcoded GEA profile data | Dead code |

#### Widgets

| Widget | Type | Role |
|--------|------|------|
| Text5 | TEXT | Page title: "Ricerca profili commerciali disponibili" |
| Form1 | FORM | Contains cascading select filters |
| s_states | SELECT | "Provincia" — source: `get_states.data[0].get_states`, onOptionChange → `get_cities.run()` |
| s_city | SELECT | "Comune" — source: `get_cities.data`, onOptionChange → `get_addresses.run()` |
| s_address | SELECT | "Indirizzo" — source: `get_addresses.data`, onOptionChange → `get_house_numbers.run()` |
| s_housenumber | SELECT | "Numero civico" — source: `get_house_numbers.data` |
| Button1 | BUTTON | "Cerca copertura" — onClick: `get_coverage.run(); utils.updateTestoRicerca()` |
| Button2 | BUTTON | "Reset" — resetFormOnClick: true |
| t_ricerca | TEXT | Search breadcrumb display (set programmatically by `updateTestoRicerca`) |
| Risultati | LIST_WIDGET_V2 | Coverage results list, data: `get_coverage.data`, keyed on `coverage_id` |
| Text2 | TEXT (in list) | `currentItem.tech` — technology name |
| Text3 | TEXT (in list) | `utils.formatProfili(currentItem.profiles)` — profile names HTML |
| Text4 | TEXT (in list) | `utils.formatDettagli(currentItem.details)` — detail items HTML |
| Image1 | IMAGE (in list) | `utils.getImageUrl(currentItem.operator_id)` — operator logo |

#### Event Flow

```
Page load → get_states(), get_details_types()
  ↓
s_states change → get_cities(state_id)
  ↓
s_city change → get_addresses(city_id)
  ↓
s_address change → get_house_numbers(address_id)
  ↓
Button1 click → get_coverage(house_number_id) + updateTestoRicerca()
  ↓
Risultati list renders coverage results with profiles, details, operator logos
```

#### Hidden Logic

- **Operator logo mapping** is hardcoded in `getImageUrl()` — maps operator_id 1–4 to static CDN URLs at `static.cdlan.business`
- **Detail type name resolution** depends on `get_details_types` being loaded at page start — `getDetailName()` reads from `get_details_types.data[0].get_coverage_details_types`
- **`get_states` returns nested JSON** — consumed as `data[0].get_states` (PostgreSQL function returning JSON)
- **HTML rendering** in Text3/Text4 — `formatProfili` and `formatDettagli` generate raw HTML tables/lists. The `formatDettagli` regex `/0000$/` strips trailing zeros from values.

#### Candidate Domain Entities

- **State** (provincia)
- **City** (comune)
- **Address** (indirizzo)
- **HouseNumber** (numero civico)
- **CoverageResult** (operator, tech, profiles[], details[])
- **Operator** (id, name, logo)
- **CoverageDetailType** (id, name)

#### Database Objects

- `coperture.get_states()` — stored function
- `coperture.get_coverage_details_types()` — stored function
- `coperture.network_coverage_cities` — table
- `coperture.network_coverage_addresses` — table
- `coperture.network_coverage_house_numbers` — table
- `coperture.v_get_coverage` — view (returns operator, tech, profiles JSON, details JSON)

#### Migration Notes

- All queries are simple reads — ideal for a backend REST API with cascading endpoints
- The PostgreSQL stored functions return JSON — the backend can pass through or reshape
- HTML rendering (formatProfili/formatDettagli) should move to React components
- Operator logo mapping should become a backend lookup or config

---

### 2.3 Energia variabile

**Purpose:** Monitor and analyze variable energy billing for datacenter colocation racks. Five functional tabs: rack-level power readings, kW consumption charts, billing charges (addebiti), racks without variable billing, and low-consumption socket detection.

**Datasource:** `grappa` (MySQL)

#### Queries

| Query | Purpose | Runs On | Prepared | Key Dependencies |
|-------|---------|---------|----------|------------------|
| get_customers | List active customers with rack sockets | Page load | Yes | — |
| get_sites | List datacenter buildings for customer | Page load | **No** | s_customers |
| get_rooms | List rooms for site + customer | Page load | Yes | s_sites, s_customers |
| get_racks | List racks in room for customer | Page load | Yes | s_rooms, s_customers |
| get_rack_details | Rack metadata | Page load | Yes | s_racks |
| get_socket_status | Socket avg ampere (last 2 days) | Page load | Yes | s_racks |
| get_power_readings | Power readings (paginated) | Button1 click | **No** | i_from, i_to, s_racks, tbl_power |
| count_power_reading | Count for pagination | Button1 click | **No** | i_from, i_to, s_racks |
| stats_last_days | Hourly ampere/kW (last 2 days) | Page load | Yes | s_racks |
| get_kw_days | Daily kW with cosfi | Button4 click | Yes | s_customers_kw, ns_cosfi |
| get_kw_months | Monthly avg kW with cosfi | Button4 click | Yes | s_customers_kw, ns_cosfi |
| get_kw_weeks | Weekly avg kW with cosfi | Button4 click (unreachable) | Yes | s_customers_kw, ns_cosfi |
| get_addebiti_by_cli | Billing charges per customer | s_cli_addebiti change | Yes | s_cli_addebiti |
| anagrafiche_no_variable | Customers without variable billing | Page load | — | — |
| racks_no_variable | Racks for selected non-variable customer | Table2 row select | **No** | Table2.selectedRow |
| rack_basso_consumo | Low-consumption sockets | Button3 click | Yes | i_min_assorbimento, s_clienti_low_consumo |

#### JSObjects

**`utils`**
| Method/Var | Purpose | Classification |
|------------|---------|----------------|
| `loadData()` | Orchestrates: fires `get_rack_details`, `stats_last_days`, `get_socket_status` (fire-and-forget), awaits `count_power_reading` + `get_power_readings` | UI orchestration |
| `myNoVarCli()` | Extracts distinct `intestazione` values from `racks_no_variable.data` | Business logic |

**`echart_ampere`**
| Method/Var | Purpose | Classification |
|------------|---------|----------------|
| `Rigenera()` | Maps `stats_last_days.data` into ECharts dual-axis line chart (Ampere + kW) | Presentation |
| `option` | Reactive ECharts config: dual Y-axis (Ampere left, kW right), category X-axis | Presentation |

**`jschart_kw`**
| Method/Var | Purpose | Classification |
|------------|---------|----------------|
| `aggiorna()` | Switch on period (day/week/month), run query, call `plot()` | UI orchestration |
| `plot(dataset)` | Set chart title with customer + cosfi, map data, compute yAxis bounds | Presentation + business logic (cosfi label) |
| `myFun2()` | Debug: runs `get_kw_days` sync, returns max/min string | Dead code |
| `options` | Reactive ECharts bar chart config: log yAxis base 2 | Presentation |

#### Widgets (organized by tab)

**Tab 1: "Situazione per rack"**

| Widget | Type | Role |
|--------|------|------|
| Form1 | FORM | Filter form: customer → site → room → rack + date range |
| s_customers | SELECT | "Cliente", source: `get_customers.data`, onChange → `get_sites.run()` |
| s_sites | SELECT | "Site", source: `get_sites.data` mapped to `{name: site, code: dc_build_id}`, onChange → `get_rooms.run()` |
| s_rooms | SELECT | "Room", source: `get_rooms.data` mapped to `{name: room_name, code: id_datacenter}`, onChange → `get_racks.run()` |
| s_racks | SELECT | "Rack", source: `get_racks.data` mapped to `{name: rack_name, code: id_rack}` |
| i_from | DATE_PICKER | "Letture Dal", default: yesterday, format: `YYYY-MM-DD HH:mm` |
| i_to | DATE_PICKER | "Letture Al", default: now, format: `YYYY-MM-DD HH:mm` |
| Button1 | BUTTON | "Aggiorna" → `utils.loadData()` |
| Button2 | BUTTON | "Reset" → form reset |
| Container1 | CONTAINER | Rack details display area |
| Text1 | TEXT | Rack name from `get_rack_details.data[0].name` |
| Text1Copy | TEXT | Floor/Island/Type/Pos from rack details |
| Text1CopyCopy | TEXT | Order code/Serial/Billing type/Committed Ampere/Billing start |
| List1 | LIST_V2 | Socket status list, data: `get_socket_status.data`, keyed on `rack_socket_id` |
| Progress1 | PROGRESS (circular) | `ampere/(maxampere/2)*100`, red if >90%, else green |
| tbl_power | TABLE_V2 | Power readings, server-side pagination, columns: Socket ID, date, Ampere |
| Chart1 | CHART (LINE) | "Assorbimenti ultimi due giorni" — series: Ampere + kW from `stats_last_days` |

**Tab 2: "Consumi in kW"**

| Widget | Type | Role |
|--------|------|------|
| s_customers_kw | SELECT | Customer selector |
| s_period | SELECT | Period: "Giornaliero" (day) / "Mensile" (month) — **"Settimanale" (week) missing** |
| ns_cosfi | NUMBER_SLIDER | cosfi (centesimi), range 70–100, default 95, step 1 |
| Button4 | BUTTON | "Aggiorna" → `jschart_kw.aggiorna()` |
| Chart2 | CHART (CUSTOM_ECHART) | kW bar chart from `jschart_kw.options` |

**Tab 3: "Addebiti"**

| Widget | Type | Role |
|--------|------|------|
| s_cli_addebiti | SELECT | Customer selector, onChange → `get_addebiti_by_cli.run()` |
| Table1 | TABLE_V2 | Billing data: start/end period, ampere, eccedenti, amount (EUR), PUN, coefficiente, fisso CU, importo eccedenti |

**Tab 4: "Racks no variable"**

| Widget | Type | Role |
|--------|------|------|
| Table2 | TABLE_V2 | Customer list (`anagrafiche_no_variable`), onRowSelected → `racks_no_variable.run()` |
| Table3 | TABLE_V2 | Rack details for selected customer (`racks_no_variable`) |

**Tab 5: "Consumi < 1A"**

| Widget | Type | Role |
|--------|------|------|
| Form2 | FORM | Filter: min absorption threshold + customer |
| i_min_assorbimento | INPUT (NUMBER) | Default: 1 — "Socket che assorbono <=" |
| s_clienti_low_consumo | SELECT | Customer filter (empty = all) |
| Button3 | BUTTON | "Cerca" → `rack_basso_consumo.run()` |
| Table4 | TABLE_V2 | Results: intestazione, building, room, name, ampere, power meter, magnetotermico, posizioni |

#### Event Flow

```
Page load → get_customers, get_sites, get_rooms, get_racks, get_rack_details, 
            get_socket_status, stats_last_days, anagrafiche_no_variable

Tab 1 cascading:
  s_customers change → get_sites(customer)
    → s_sites change → get_rooms(site, customer)
      → s_rooms change → get_racks(room, customer)

  Button1 "Aggiorna" → utils.loadData():
    fire-and-forget: get_rack_details, stats_last_days, get_socket_status
    await: count_power_reading, get_power_readings

  tbl_power page change → get_power_readings (server-side pagination)

Tab 2:
  Button4 "Aggiorna" → jschart_kw.aggiorna():
    day → get_kw_days → plot()
    month → get_kw_months → plot()
    week → get_kw_weeks → plot()  [UNREACHABLE]

Tab 3:
  s_cli_addebiti change → get_addebiti_by_cli

Tab 4:
  Table2 row select → racks_no_variable

Tab 5:
  Button3 "Cerca" → rack_basso_consumo
```

#### Hidden Logic & Bugs

1. **cosfi inconsistency (BUG):** `get_kw_weeks` uses `ns_cosfi.value` raw (70–100 range) while `get_kw_days` and `get_kw_months` correctly use `ns_cosfi.value/100`. Weekly kW values would be ~100× inflated. Currently unreachable from UI.
2. **Missing "week" period option:** `s_period` dropdown has only "Giornaliero"/"Mensile" but `jschart_kw.aggiorna()` handles a "week" case via `get_kw_weeks`. This code path is dead.
3. **Hardcoded 225V assumption:** `stats_last_days` SQL computes kW as `sum(ampere)*225/1000` — assumes single-phase 225V for all sockets, ignoring that `magnetotermico = 'trifase 32A'` implies 3-phase power.
4. **Fire-and-forget queries:** `utils.loadData()` calls `get_rack_details.run()`, `stats_last_days.run()`, `get_socket_status.run()` without `await` — widgets may render before data arrives.
5. **SQL injection risk:** `racks_no_variable` uses `prepared: false` with `Table2.selectedRow.intestazione` interpolated directly into WHERE clause via `'{{...}}'`.
6. **LIKE with numeric ID:** `get_sites` and `get_racks` use `r.id_anagrafica like '{{s_customers.selectedOptionValue||'%'}}'` — treats a numeric foreign key as a LIKE pattern, which is fragile.
7. **Progress bar threshold:** Socket ampere gauge computes `ampere/(maxampere/2)*100` — so 50% of maxampere = 100% on the gauge. The `/2` halves the scale, meaning the gauge maxes out at half the actual breaker capacity.
8. **`maxampere` calculation:** `CASE WHEN magnetotermico = 'trifase 32A' THEN 63` — a 32A three-phase breaker gets 63A capacity. The mapping assumes specific breaker types; new types would default to 32.

#### Candidate Domain Entities

- **Customer** (`cli_fatturazione`)
- **Site/Building** (`dc_build`)
- **Room/Datacenter** (`datacenter`)
- **Rack** (`racks` — with metadata: floor, island, type, pos, serial, billing type)
- **RackSocket** (`rack_sockets` — magnetotermico, SNMP device, positions)
- **PowerReading** (`rack_power_readings`)
- **DailySummary** (`rack_power_daily_summary`)
- **BillingCharge** (`importi_corrente_colocation`)

#### Grappa Tables Referenced

| Table | Key Columns |
|-------|-------------|
| `cli_fatturazione` | id, intestazione, codice_aggancio_gest |
| `racks` | id_rack, name, id_datacenter, id_anagrafica, stato, variable_billing, floor, island, type, pos, codice_ordine, serialnumber, committed_power, billing_start_date |
| `rack_sockets` | id, rack_id, magnetotermico, snmp_monitoring_device, detector_ip, posizione/2/3/4 |
| `rack_power_readings` | id, oid, rack_socket_id, date, ampere |
| `rack_power_daily_summary` | id, giorno, kilowatt, id_anagrafica |
| `datacenter` | id_datacenter, name, dc_build_id |
| `dc_build` | id, name |
| `importi_corrente_colocation` | id, customer_id, start_period, end_period, ampere, eccedenti, amount, pun, coefficiente, fisso_cu, importo_eccedenti |

#### Migration Notes

- Most complex page; should be split into multiple React views/tabs
- The cascading filter (Customer → Site → Room → Rack) is a reusable pattern across the app
- Server-side pagination for `tbl_power` must be preserved — large table
- ECharts configs (echart_ampere, jschart_kw) can be ported to a React ECharts wrapper
- cosfi bug and 225V hardcoding should be fixed in the rewrite
- All direct DB access should move to backend API endpoints
- The `anagrafiche_no_variable` and `racks_no_variable` queries exclude `id_anagrafica = 3` — hardcoded exclusion (likely the company's own racks)

---

### 2.4 Transazioni whmcs

**Purpose:** View paid WHMCS transactions. **Page is hidden** (`isHidden: true`).

**Datasources:** `whmcs_prom` (MySQL), `transazioni-whmcs` (GraphQL — unused)

#### Queries

| Query | Purpose | Runs On | Datasource |
|-------|---------|---------|------------|
| fatture_pagate | Paid invoices from `v_transazioni` view | Page load | whmcs_prom (MySQL) |
| getTransazioni | GraphQL stub — only fetches `cliente` | Never (executeOnLoad: false) | transazioni-whmcs |

**`fatture_pagate` SQL:**
```sql
SELECT cliente, fattura, invoiceid, userid, payment_method,
       date_format(date, '%Y-%m-%d') as date, description,
       amountin, fees, amountout, rate, transid, refundid, accountsid
FROM v_transazioni
WHERE ((fattura <> '' AND invoiceid > 0) OR refundid > 0)
  AND date > 20230120
ORDER BY date DESC, fattura ASC
```

#### Widgets

| Widget | Type | Role |
|--------|------|------|
| tbl_transactions | TABLE_V2 | 14-column table bound to `fatture_pagate.data`. Client-side search, no server-side pagination. CSV delimiter: `;`. Date format: DD/MM/YYYY. |

**Columns:** cliente, fattura, invoiceid, userid, payment_method, date, description, amountin, fees, amountout, rate, transid, refundid, accountsid

#### Hidden Logic

- **Hardcoded date filter:** `date > 20230120` — only shows transactions after 2023-01-20. This is a hidden business rule.
- **GraphQL stub:** `getTransazioni` is an incomplete experiment that never runs.
- **The page is hidden** — possibly deprecated or for internal use only.

#### Migration Notes

- Simple read-only table. If still needed, implement as a backend endpoint + table component.
- The hardcoded date cutoff should become a configurable parameter or filter.
- The `v_transazioni` view is in the `whmcs_prom` MySQL database — need to verify if this DB is accessible from the backend.
- Consider whether this page should be migrated at all given it's hidden.

---

### 2.5 IaaS calcolatrice

**Purpose:** Calculate daily and monthly IaaS resource costs for customers. Supports two pricing tiers (Diretta/Indiretta). Can generate PDF quotes via Carbone.io.

**Datasource:** `carbone.io` (REST API)

#### Queries

| Query | Type | Purpose | Runs On |
|-------|------|---------|---------|
| render_template | REST API (POST) | Send data to Carbone.io to render PDF | Button3 click |

**`render_template` request:**
- Endpoint: `POST /render/{{utils.templateId}}`
- Body: `{ convertTo: "pdf", data: { qta, prezzi, totale_giornaliero } }`
- Template ID: `7229f811c77569a9ab09c7f71cd923a942e3d5d5aac1d26b98950a19beb2e920`

#### JSObject: `utils`

**Reactive Variables:**

| Variable | Purpose |
|----------|---------|
| `qta` | Quantities from form inputs (vcpu, ram_vmware, ram_os, storage_pri, storage_sec, fw_std, fw_adv, priv_net, os_windows, ms_sql_std) |
| `prezzi` | Active pricing (switches between diretta/indiretta) |
| `prezzi_diretta` | Direct channel pricing |
| `prezzi_indiretta` | Indirect channel pricing |
| `totale_giornaliero` | Computed daily totals by category (computing, storage, sicurezza, addon, totale) |
| `hours` | 730 — **declared but never used** |
| `days` | 30 — used for monthly calculation |
| `templateId` | Carbone.io template hash |
| `templateHTML` | Dynamic HTML for price table display |

**Hardcoded Daily Pricing (EUR):**

| Resource | Diretta | Indiretta |
|----------|---------|-----------|
| vCPU (1 GHz min) | €0.10 | €0.05 |
| RAM VMware (GB) | €0.30 | €0.20 |
| RAM KVM Linux (GB) | €0.10 | €0.08 |
| Primary Storage (GB) | €0.001 | €0.001 |
| Secondary Storage (GB) | €0.001 | €0.001 |
| Firewall Standard | €0 | €0 |
| Firewall Advanced | €1.80 | €1.80 |
| Private Network | €0 | €0 |
| OS Windows Server | €1.00 | €1.00 |
| MS SQL Server Std | €6.33 | €6.33 |

**Methods:**

| Method | Purpose | Classification |
|--------|---------|----------------|
| `loadPrezzi()` | Read quantities from form inputs into `qta` | UI orchestration |
| `calcolaTotali()` | Compute all daily totals, categories, monthly total | Business logic |
| `updatePrezzi()` | Switch pricing tier based on `rg_dirindir` radio, generate price HTML | Business logic + presentation |
| `getURL()` | Construct Carbone.io download URL from render response | UI orchestration |
| `pippo()` | Debug — returns data payload for template | Dead code |

**Calculation logic (in `calcolaTotali`):**
- Daily line item = quantity × unit price
- computing = vcpu + ram_vmware + ram_os
- storage = storage_pri + storage_sec
- sicurezza = fw_std + fw_adv + priv_net
- addon = os_windows + ms_sql_std
- totale = sum of all line items
- mese (monthly) = totale × 30 days

#### Widgets

| Widget | Type | Role |
|--------|------|------|
| rg_dirindir | RADIO_GROUP | "Diretta" (D) / "Indiretta" (I), default: D, onChange → `utils.updatePrezzi()` |
| Text1 | TEXT | Title: "Addebiti giornalieri risorse" |
| Text2 | TEXT | Dynamic price table: `{{utils.templateHTML}}` |
| Text3 | TEXT | Static HTML: free inclusions (Public IP, VPC, Firewall, 1Gbps network) |
| Text6 | TEXT | Daily total breakdown: Computing/Storage/Sicurezza/Add On |
| Text7 | TEXT | Monthly total (€, 1.875rem bold blue) = `totale × days` |
| Form1 | FORM | Resource input form |
| i_vcpu | INPUT (NUMBER) | vCPU count, default: 1, min: 1, required |
| i_ram_vmware | INPUT (NUMBER) | RAM VMware (GB), default: 0 |
| i_ram_opensource | INPUT (NUMBER) | RAM KVM Linux (GB), default: 0 |
| i_storage_pri | INPUT (NUMBER) | Primary storage (GB), default: 100, min: 10, required |
| i_storage_sec | INPUT (NUMBER) | Secondary storage (GB), default: 100, min: 0, required |
| i_fw_standard | INPUT (**TEXT**) | Firewall standard — **inputType is TEXT, not NUMBER** |
| i_fw_advanced | INPUT (NUMBER) | Firewall advanced, max: 1 |
| i_private_network | INPUT (NUMBER) | Private network, default: 0 |
| i_os_windows | INPUT (**TEXT**) | OS Windows — **inputType is TEXT, not NUMBER** |
| i_sql_server | INPUT (NUMBER) | MS SQL Server Std, default: 0 |
| Button1 | BUTTON | "Calcola" → `utils.calcolaTotali()` |
| Button2 | BUTTON | "Azzera" → form reset |
| Button3 | BUTTON | "Genera PDF" → `utils.calcolaTotali(); render_template.run().then(() => navigateTo(utils.getURL(), {}, 'NEW_WINDOW'))` |

#### Event Flow

```
Page load → utils.updatePrezzi() (sets pricing based on default "Diretta")

rg_dirindir change → utils.updatePrezzi():
  - Copy pricing tier (diretta/indiretta) into active prices
  - Regenerate price table HTML

Button1 "Calcola" → utils.calcolaTotali():
  - Read quantities from all input widgets
  - Compute daily line items, category subtotals, and totals
  - Text6 and Text7 auto-update via reactive bindings

Button3 "Genera PDF":
  - calcolaTotali()
  - POST to Carbone.io /render/{templateId} with qta + prezzi + totali
  - Open PDF download URL in new window
```

#### Hidden Logic

- **Pricing is entirely hardcoded** in the JSObject — no database lookup. Changing prices requires editing the code.
- **Two TEXT-type inputs** (`i_fw_standard`, `i_os_windows`) are used in numeric multiplication — relies on JS implicit type coercion.
- **`hours` variable (730)** is declared but never used — `days` (30) is used for monthly calculation.
- **`toFixed(2)` on subtotals but not line items** — category subtotals (computing, storage, etc.) are strings after `toFixed(2)`, while the final `totale` is recalculated from raw numbers. This avoids string+number bugs but the inconsistency could cause issues if subtotals are used in further calculations.
- **Carbone.io template ID** is hardcoded — changing the PDF template requires code changes.
- **`updatePrezzi()`** generates HTML with an incomplete `decodifica` map — only 5 of 10 resources have display names, so only those 5 appear in the price table.
- **Deep clone via `JSON.parse(JSON.stringify())`** is used to copy pricing tiers — necessary to avoid shared references in Appsmith reactive variables.

#### Candidate Domain Entities

- **PricingTier** (diretta/indiretta, per-resource daily rates)
- **ResourceQuantity** (user-specified quantities)
- **CostCalculation** (daily totals by category, monthly total)
- **PDFQuote** (rendered via Carbone.io)

#### Migration Notes

- Self-contained calculator with no database — can be implemented as a pure frontend feature
- Pricing should move to a configurable backend source (DB or config file)
- The Carbone.io PDF generation should be proxied through the backend (API key should not be in the frontend)
- The `i_fw_standard` and `i_os_windows` TEXT input types should be fixed to NUMBER in the rewrite
- Free inclusions text is static marketing copy — could move to CMS or config

---

## 3. Datasource & Query Catalog

### 3.1 dbcoperture (PostgreSQL)

| Query | Page | R/W | Parameters | Rewrite Target |
|-------|------|-----|------------|----------------|
| get_states | Coperture | Read | None | Backend API: `GET /api/coperture/states` |
| get_cities | Coperture | Read | state_id | Backend API: `GET /api/coperture/states/{id}/cities` |
| get_addresses | Coperture | Read | city_id | Backend API: `GET /api/coperture/cities/{id}/addresses` |
| get_house_numbers | Coperture | Read | address_id | Backend API: `GET /api/coperture/addresses/{id}/house-numbers` |
| get_coverage | Coperture | Read | house_number_id | Backend API: `GET /api/coperture/house-numbers/{id}/coverage` |
| get_details_types | Coperture | Read | None | Backend API: `GET /api/coperture/detail-types` (or inline in coverage response) |

### 3.2 grappa (MySQL)

| Query | Page | R/W | Parameters | Rewrite Target |
|-------|------|-----|------------|----------------|
| get_customers | Energia | Read | None | Backend API: `GET /api/energia/customers` |
| get_sites | Energia | Read | customer_id | Backend API: `GET /api/energia/customers/{id}/sites` |
| get_rooms | Energia | Read | site_id, customer_id | Backend API: `GET /api/energia/sites/{id}/rooms` |
| get_racks | Energia | Read | room_id, customer_id | Backend API: `GET /api/energia/rooms/{id}/racks` |
| get_rack_details | Energia | Read | rack_id | Backend API: `GET /api/energia/racks/{id}` |
| get_socket_status | Energia | Read | rack_id | Backend API: `GET /api/energia/racks/{id}/socket-status` |
| get_power_readings | Energia | Read | from, to, rack_id, page, pageSize | Backend API: `GET /api/energia/racks/{id}/power-readings?from=&to=&page=&size=` |
| count_power_reading | Energia | Read | from, to, rack_id | Merge into power-readings response as total count |
| stats_last_days | Energia | Read | rack_id | Backend API: `GET /api/energia/racks/{id}/stats-last-days` |
| get_kw_days | Energia | Read | customer_id, cosfi | Backend API: `GET /api/energia/customers/{id}/kw?period=day&cosfi=` |
| get_kw_months | Energia | Read | customer_id, cosfi | Backend API: `GET /api/energia/customers/{id}/kw?period=month&cosfi=` |
| get_kw_weeks | Energia | Read | customer_id, cosfi | Backend API: `GET /api/energia/customers/{id}/kw?period=week&cosfi=` |
| get_addebiti_by_cli | Energia | Read | customer_id | Backend API: `GET /api/energia/customers/{id}/addebiti` |
| anagrafiche_no_variable | Energia | Read | None | Backend API: `GET /api/energia/no-variable-billing/customers` |
| racks_no_variable | Energia | Read | intestazione | Backend API: `GET /api/energia/no-variable-billing/customers/{name}/racks` |
| rack_basso_consumo | Energia | Read | min_ampere, customer | Backend API: `GET /api/energia/low-consumption?min=&customer=` |

### 3.3 whmcs_prom (MySQL)

| Query | Page | R/W | Parameters | Rewrite Target |
|-------|------|-----|------------|----------------|
| fatture_pagate | Transazioni | Read | None (hardcoded date filter) | Backend API: `GET /api/transazioni/fatture?from=` |

### 3.4 transazioni-whmcs (GraphQL)

| Query | Page | R/W | Parameters | Rewrite Target |
|-------|------|-----|------------|----------------|
| getTransazioni | Transazioni | Read | None | **Unused stub — do not migrate** |

### 3.5 carbone.io (REST API)

| Query | Page | R/W | Parameters | Rewrite Target |
|-------|------|-----|------------|----------------|
| render_template | IaaS calc | Write | qta, prezzi, totale_giornaliero, templateId | Backend API: `POST /api/iaas/render-quote` (proxy to Carbone.io) |

---

## 4. Findings Summary

### 4.1 Embedded Business Rules

| # | Location | Rule | Impact |
|---|----------|------|--------|
| 1 | Coperture `getImageUrl()` | Operator ID → logo URL mapping (1=TIM, 2=Fastweb, 3=OF, 4=OF CD) | Must be maintained in rewrite |
| 2 | Coperture `formatDettagli()` | Strip trailing `0000` from detail values via regex | Possible data formatting rule |
| 3 | Energia `stats_last_days` | kW = ampere × 225V / 1000 (hardcoded voltage) | Physics assumption — should be configurable |
| 4 | Energia `get_socket_status` | `maxampere` CASE: trifase 32A→63, monofase 16A→16, else→32 | Breaker capacity mapping |
| 5 | Energia `Progress1` | Gauge = ampere/(maxampere/2)×100, red >90% | Threshold at half-capacity |
| 6 | Energia multiple queries | `id_anagrafica <> 3` exclusion | Hardcoded company self-exclusion |
| 7 | IaaS `utils` | All pricing tiers hardcoded (Diretta/Indiretta rates) | Should move to configurable source |
| 8 | IaaS `calcolaTotali` | Monthly = daily × 30 days | Fixed 30-day month assumption |
| 9 | Transazioni `fatture_pagate` | `date > 20230120` cutoff | Hardcoded date filter |

### 4.2 Duplication

| # | Description |
|---|-------------|
| 1 | Three separate `utils` JSObjects (Coperture, Energia, IaaS) with no shared code |
| 2 | `get_customers` pattern (active customers from `cli_fatturazione`) could be shared |
| 3 | Customer→Site→Room→Rack cascading filter is a reusable pattern |

### 4.3 Security Concerns

| # | Risk | Location | Severity |
|---|------|----------|----------|
| 1 | **SQL injection** | `racks_no_variable`: `Table2.selectedRow.intestazione` interpolated directly into SQL (prepared: false) | High |
| 2 | **SQL injection** | `get_sites`, `get_power_readings`, `count_power_reading`: prepared statements OFF with widget values interpolated | Medium |
| 3 | **Direct DB access from UI** | All pages query databases directly — no backend API layer | High (architecture) |
| 4 | **Carbone.io API key** | Likely configured in the Appsmith datasource, not visible in export, but API calls originate from Appsmith server | Medium |
| 5 | **Operator logo URLs** | Hardcoded CDN URLs (`static.cdlan.business`) could be manipulated if CDN is compromised | Low |

### 4.4 Migration Blockers

| # | Blocker | Details |
|---|---------|---------|
| 1 | **Three separate databases** | Backend must connect to PostgreSQL (dbcoperture), MySQL (grappa), and optionally MySQL (whmcs_prom) |
| 2 | **PostgreSQL stored functions** | `coperture.get_states()` and `coperture.get_coverage_details_types()` return JSON — backend must call these functions |
| 3 | **PostgreSQL view** | `coperture.v_get_coverage` returns nested JSON (profiles[], details[]) — structure must be preserved |
| 4 | **Carbone.io integration** | PDF template is externally hosted — need API key, template management, and download proxy |
| 5 | **WHMCS database** | `whmcs_prom` with `v_transazioni` view — need to verify if this DB is accessible and if the page is still needed |
| 6 | **ECharts** | Two custom ECharts configurations (ampere line chart, kW bar chart) need to be ported |

### 4.5 Recommended Next Steps

1. **Decide on Transazioni whmcs:** The page is hidden — confirm if it should be migrated or dropped.
2. **Prioritize pages for migration:** Coperture and IaaS calcolatrice are self-contained and simpler; Energia variabile is the most complex.
3. **Design backend API layer:** 20+ queries need backend endpoints across 3 databases.
4. **Fix known bugs in rewrite:** cosfi inconsistency, 225V hardcoding, SQL injection, TEXT input types.
5. **Extract hardcoded business rules:** Pricing, operator mappings, voltage, breaker capacity, date filters.
6. **Handle Carbone.io:** Proxy PDF generation through the backend; store template IDs in config.
7. **Drop unused code:** `ExcelJS`/`xmlParser` libraries, `getTransazioni` GraphQL stub, `test()`/`myFun2()`/`pippo()` debug methods, unreachable "week" period.

---

## Appendix: File Structure

```
zammu-main/
├── application.json
├── metadata.json
├── theme.json
├── README.md
├── jslibs/
│   ├── ExcelJS_....json
│   └── xmlParser_....json
├── datasources/
│   ├── carbone.io.json          (REST API)
│   ├── dbcoperture.json         (PostgreSQL)
│   ├── grappa.json              (MySQL)
│   ├── transazioni-whmcs.json   (GraphQL)
│   └── whmcs_prom.json          (MySQL)
└── pages/
    ├── Home/                    (3 widgets, 0 queries)
    ├── Coperture/               (15 widgets, 6 SQL queries, 1 JSObject)
    ├── Energia variabile/       (30+ widgets, 16 queries, 3 JSObjects)
    ├── Transazioni whmcs/       (1 widget, 2 queries — hidden page)
    └── IaaS calcolatrice/       (27 widgets, 1 API query, 1 JSObject)
```

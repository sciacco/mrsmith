# Phase B: UX Pattern Map — Panoramica Cliente

## Navigation

**Decision (pre-made):** Same `TabNavGroup` variant as `listini-e-sconti`. Grouped horizontal tabs with dropdown menus. Mobile: hamburger + expandable sections.

**Navigation groups (confirmed):**

| Group | Pages | Notes |
|-------|-------|-------|
| **Ordini** | Ordini ricorrenti, Ordini Ricorrenti e Spot | Dropdown with 2 entries |
| **Fatture** | Fatture | Single page |
| **Servizi** | Accessi, IaaS Pay Per Use, Timoo tenants, Licenze Windows | Dropdown with 4 entries |

---

## View Specifications

### View 1: Ordini ricorrenti

- **User intent:** Browse recurring orders in summary form with customer/status filters and order history chain.
- **Interaction pattern:** Master-Detail Drawer — flat table with visual row grouping + slide-over panel on row click.

#### Filter bar

| Widget | Type | Behavior |
|--------|------|----------|
| Customer selector | Select (searchable, required) | No "Tutti i clienti" — customer must be selected |
| Status multi-select | MultiSelect | Default: "Evaso", "Confermato" |
| "Cerca" button | Button | Triggers order query |

**Changes from Appsmith:**
- "Tutti i clienti" (-1) option: **removed** — customer selection always required.
- "Righe espanse" checkbox: **dropped** (was Appsmith workaround, no handler).

#### Main table

Flat table where each row is an order line item (same data shape as `v_ordini_sintesi`). Rows of the same order are visually grouped:
- **First row of each order:** slightly taller (48px vs 40px), `nome_testata_ordine` in weight 600 + `numero_ordine` mono label, top border 1px to separate from previous order.
- **Subsequent rows:** indented description, tree-line connector (left border) linking to header row.

**Default visible columns (~10 of 24):**

| Column | Field | Notes |
|--------|-------|-------|
| Stato ordine | `stato_ordine` | Colored dot badge |
| Numero | `numero_ordine` | Monospace |
| Ordine / Descrizione | `nome_testata_ordine` / `descrizione_long` | First row: order name bold. Others: line description |
| Qta | `quantita` | Right-aligned |
| NRC | `nrc` | EUR, right-aligned |
| MRC | `mrc` | EUR, right-aligned |
| Totale MRC | `totale_mrc` | EUR, right-aligned |
| Data | `data_documento` | DD-MM-YYYY |
| Stato riga | `stato_riga` | Colored badge |
| Serial | `serialnumber` | — |

**Hidden by default (toggleable via column visibility button):** `data_ultima_fatt`, `metodo_pagamento`, `durata_servizio`, `durata_rinnovo`, `data_cessazione`, `data_attivazione`, `note_legali`, `sost_ord`, `sostituito_da`, `storico`, etc.

#### Slide-over panel (480px, from right)

Opens on row click. Content:

| Section | Content |
|---------|---------|
| Header (sticky) | `nome_testata_ordine`, `numero_ordine`, `stato_ordine` badge. Close button (X). |
| Order metadata | Label/value pairs in 2 columns: `data_documento`, `metodo_pagamento`, `durata_servizio`, `durata_rinnovo`, `storico` (substitution chain), `sost_ord`, `sostituito_da`, `note_legali` |
| Selected line card | Highlighted card with all fields of the clicked line item |
| Sibling lines | Scrollable list of other lines in the same order (condensed cards). Click → updates selected line card. |

**Interactions:**
- Click another row in table → panel content cross-fades to new order/line
- Arrow up/down in table → navigates rows (panel follows if open)
- Escape → closes panel
- Table row stays highlighted while panel is open

- **Data loading:** On page load: customer list + status list. On "Cerca": query orders.
- **Key actions:** Search/filter, browse detail via panel (read-only).
- **Entry/exit:** Tab navigation entry. No cross-page navigation.

---

### View 2: Ordini Ricorrenti e Spot

- **User intent:** Full-detail order viewer per customer — recurring and spot orders with lifecycle, referents, product families, and computed `stato_riga`.
- **Interaction pattern:** Master-Detail Drawer — flat table with visual row grouping + slide-over panel with tabs on row click.

#### Filter bar

| Widget | Type | Behavior |
|--------|------|----------|
| Customer selector | Select (searchable, required) | Customer must be selected |
| Status multi-select | MultiSelect | Default: "Evaso", "Confermato" |
| "GO" button | Button | Triggers order detail query |

**Changes from Appsmith:**
- `Text1` greeting (`Hello {{appsmith.user.name}}`): **dropped**.
- Vestigial `get_ordini_ricorrenti` query on this page: **dropped** (only `GET_ordini_Ric_Spot` is used).

#### Main table

Same visual grouping pattern as View 1 (first-row emphasis + tree-line connector). A few more default columns due to the detail nature of this page.

**Default visible columns:**

| Column | Field | Notes |
|--------|-------|-------|
| Stato ordine | `stato_ordine` | Colored dot badge |
| Ordine | `ORDINE` / `descrizione_long` | First row: order name bold. Others: line description |
| Tipo ordine | `tipo_ordine` | — |
| Commerciale | `commerciale` | — |
| Data ordine | `data_ordine` | Computed: MAX(conferma, documento) |
| Qta | `quantita` | Right-aligned |
| MRC | `mrc` | EUR (quantita * canone) |
| Stato riga | `stato_riga` | Colored badge (8-way CASE) |
| Serial | `serialnumber` | — |
| Codice prodotto | `codice_prodotto` | Mono |

**Hidden by default (toggleable):** all other 50+ columns — accessible via panel.

#### Slide-over panel (600px, from right)

Opens on row click. Contains **4 tabs**:

**Tab "Testata"** — All order-header fields in structured 2-column grid:

| Section | Fields |
|---------|--------|
| Anagrafica | `ragione_sociale`, `commerciale`, `tipo_ordine`, `tipo_documento`, `riferimento_odv_cliente` |
| Condizioni | `tacito_rinnovo`, `durata_servizio`, `durata_rinnovo`, `tempi_rilascio`, `metodo_pagamento`, `note_legali` |
| Referente Amm. | `referente_amm_nome`, `referente_amm_mail`, `referente_amm_tel` |
| Referente Tech. | `referente_tech_nome`, `referente_tech_mail`, `referente_tech_tel` |
| Referente Altro | `referente_altro_nome`, `referente_altro_mail`, `referente_altro_tel` |
| Fatturazione | `raggruppamento_fatturazione`, `intervallo_fatt_attivazione`, `intervallo_fatt_canone`, `data_scadenza_ordine`, `data_fine_fatt` |
| Sostituzioni | `sost_ord`, `sostituito_da`, `intestazione_ordine` |

**Tab "Riga selezionata"** — Full detail of clicked line item:

| Section | Fields |
|---------|--------|
| Prodotto | `codice_prodotto`, `codice_kit`, `descrizione_prodotto`, `descrizione_estesa`, `famiglia`, `sotto_famiglia`, `conto_ricavo` |
| Importi | `setup`, `canone`, `mrc` (computed), `nrc`, `costo_cessazione`, `valuta` |
| Date | `data_attivazione`, `data_disdetta`, `data_cessazione`, `data_ultima_fatt`, `data_fine_fatt`, `data_scadenza_ordine` |
| Stato | `stato_riga` (colored badge), `annullato` flag |

**Tab "Tutte le righe"** — Mini-table of all lines for this order: `codice_prodotto`, `descrizione_prodotto`, `mrc`, `stato_riga`. Click a row → switches to "Riga selezionata" tab with that line.

**Tab "Storico"** — Order substitution chain as vertical timeline: `ORD-001 → ORD-002 → ORD-003 (corrente)`. Each node shows order name, date, status.

**Interactions:** Same as View 1 (cross-fade on row change, arrow keys, Escape to close).

- **Data loading:** On page load: customer list + status list. On "GO": query orders.
- **Key actions:** Search/filter, deep inspection via tabbed panel (read-only).
- **Entry/exit:** Tab navigation entry. No cross-page navigation.

---

### View 3: Fatture

- **User intent:** Browse invoice/credit note line items for a selected customer within a configurable time period.
- **Interaction pattern:** Filter bar (customer select + period slider) → data table. Auto-refresh on filter change (no explicit button).
- **Current Appsmith layout:**
  - Container1: customer select + category slider (6/12/24/36/all months)
  - Table1: invoice lines with visual grouping (document header on `rn=1` only)
- **Proposed layout sections:**

| Section | Content |
|---------|---------|
| Filter bar | Customer selector, period selector (6/12/24/36/Tutti). Auto-refresh on change. |
| Results table | Invoice lines grouped by document. Visible columns: Documento, Descrizione Riga, Qta, Importo Uni (EUR), Totale Riga (EUR), Codice Articolo, Serial N, Rif Cliente, Pagamento, Scadenza, Conto Ricavo, Gruppo, Sottogruppo. Hidden: data_documento, num_documento, id_cliente, progressivo_riga, rn. |

- **Data loading:** On page load: customer list. On customer select or period change: auto-query invoices.
- **Key actions:** Browse + client-side search + CSV export (table built-in).
- **Entry/exit:** Tab navigation entry. No cross-page links.

**Notes on current vs intended behavior:**
- Period slider "all" currently sends 2000 months as interval hack → backend should accept a "no limit" value
- Auto-refresh on both customer change and period change (no "GO" button) — preserve this behavior
- Currency columns formatted as EUR with 2 decimals — preserve

**Questions for expert:**
13. The Appsmith table has download/export enabled (`isVisibleDownload: true`). Should the migrated table also support CSV/Excel export?

---

### View 4: Accessi

- **User intent:** Browse connectivity access lines with filters for client(s), line status, and connection type.
- **Interaction pattern:** Multi-filter bar (multi-select clients + multi-select statuses + multi-select connection types) → data table. Manual "GO" trigger.
- **Current Appsmith layout:**
  - Container1: multi-select clients + multi-select statuses (hardcoded) + multi-select connection types (dynamic, all selected by default) + confirm icon button
  - Table1: access lines with 16 columns
- **Proposed layout sections:**

| Section | Content |
|---------|---------|
| Filter bar | Multi-select clients, multi-select status (default: Attiva), multi-select connection type (default: all), "Cerca" button |
| Results table | Access lines. Key columns: Tipo conn, Fornitore, Prov, Comune, Tipo, Profilo comm.le, Intestatario, Ordine coll., Fatt. fino a, Stato, Serialnumber, ID Grappa, Codice ordine, Id Alyante, stato_riga, stato_ordine |

- **Data loading:** On page load: client list + connection type list. On "Cerca": query access lines.
- **Key actions:** Search/filter + export (table built-in).
- **Entry/exit:** Tab navigation entry. No cross-page links.

**Notes on current vs intended behavior:**
- **Multi-client selector** — this page supports selecting multiple clients simultaneously (unique across the app)
- Status filter is hardcoded in Appsmith (Attiva, Cessata, da attivare, in attivazione, KO) — Q5 from Phase A
- Appsmith runs `get_accessi_cliente` on page load with empty client selection → returns no results. Migration should NOT auto-query with empty selection.

**Questions for expert:**
14. The Appsmith icon button is a small "confirm" icon (checkmark). The other pages use a labeled "Cerca" or "GO" button. Should we standardize to a labeled button?

---

### View 5: IaaS Pay Per Use

- **User intent:** Monitor Cloudstack IaaS consumption per account: daily charges, monthly trends, charge breakdown by resource type.
- **Interaction pattern:** Master-detail with tabs. Account table (master) → tabbed detail (daily table + daily pie chart / monthly bar chart).
- **Current Appsmith layout:**
  - `tbl_accounts`: account selector table (top)
  - `Tabs1`: two tabs
    - Tab "Giornaliero": `tbl_giornalieri` (daily charges table) + `chart_giorno` (pie chart of charge breakdown)
    - Tab "Mensile": `chart_mensili` (monthly bar chart)
- **Proposed layout sections:**

| Section | Content |
|---------|---------|
| Account table | Selectable rows. Columns: Intestazione, Credito, Abbreviazione, Serialnumber, Data attivazione. Selection drives detail. |
| Tab: Giornaliero | Daily charges table (last 120 days): Giorno, utCredit, Total Importo. Row selection drives pie chart. |
| Tab: Giornaliero (chart) | Pie chart: charge breakdown by usage type for selected day. Only non-zero types shown. |
| Tab: Mensile | Bar chart: monthly totals (last 12 months). |

- **Data loading:** On page load: account list. On account select: daily + monthly charges. On day select in daily table: charge breakdown.
- **Key actions:** Browse only (read-only). Cascading selection: account → day → breakdown.
- **Entry/exit:** Tab navigation entry. No cross-page links.

**Notes on current vs intended behavior:**
- All queries currently have `executeOnLoad: true` — but daily/monthly/breakdown depend on `tbl_accounts.selectedRow` which defaults to first row. Preserve auto-selection of first account.
- Usage type code labels (RunningVM, AllocatedVM, etc.) are column aliases in SQL. In the pie chart, they become series labels — should use human-readable labels in migration.
- The `utils.aggiornaSerie()` JSObject filters out zero-value charge types for the pie chart — this presentation logic moves to frontend.

**Original JSObject for chart data transformation:**
```javascript
// Original Appsmith JSObject: utils.aggiornaSerie()
// Classification: presentation logic → frontend
async aggiornaSerie () {
    await get_charges_by_type.run();
    const r = get_charges_by_type.data;
    const serie = [];
    r.forEach((item) => {
        Object.keys(item).forEach((key) => {
            if (key.startsWith("ut") && item[key] > 0) {
                serie.push({x: key, y: item[key]});
            }
        });
    });
    this.daySeries = serie;
    return serie;
}
```

**Questions for expert:**
15. Usage type labels are currently English technical names (utRunningVM, utAllocatedVM, etc.). Should the migration use Italian labels? What labels?
16. Should the account table show the `cloudstack_domain` UUID column, or is it internal-only?

---

### View 6: Licenze Windows su Cloudstack

- **User intent:** Monitor Windows Server license count trend.
- **Interaction pattern:** Static chart, no user interaction. Auto-loads on page entry.
- **Current Appsmith layout:**
  - `Text1`: title "Licenze Windows Server attive su Cloudstack PPU"
  - `Chart1`: line/bar chart, 14 days of license counts
- **Proposed layout (if kept separate):**

| Section | Content |
|---------|---------|
| Title | "Licenze Windows Server attive su Cloudstack PPU" |
| Chart | Bar/line chart: daily license count (last 14 days) |

- **Data loading:** On page load: auto-query.
- **Key actions:** None (view-only).

**Notes on current vs intended behavior:**
- Simplest page in the app — single query, single chart
- Strong candidate for merging into IaaS PPU as a third tab (Q7 from Phase A)
- If merged, the title becomes the tab label

---

### View 7: Timoo tenants

- **User intent:** View Timoo PBX tenants and their PBX instance statistics (user count, service extension count).
- **Interaction pattern:** Selector (tenant dropdown) → action button → statistics table + summary text.
- **Current Appsmith layout:**
  - `sl_tenant`: tenant selector (populated from Anisetta DB)
  - `Button1`: triggers PBX stats query
  - `Table1`: PBX instances with stats
  - `Text1`: total users / total service extensions
- **Proposed layout sections:**

| Section | Content |
|---------|---------|
| Filter bar | Tenant selector + "Cerca" button |
| Summary | Total users, total service extensions (bold/highlighted) |
| PBX table | Rows: PBX Name, PBX ID, Users, Service Extensions, Totale. |

- **Data loading:** On page load: tenant list (from Anisetta DB). On button click: PBX stats for selected tenant.
- **Key actions:** Select tenant → view PBX statistics.

**Original JSObject for tenant list (DROPPED — TIMOO API excluded):**
```javascript
// Original Appsmith JSObject: utils.generaTenantIdList() + utils.listaTenants()
// DROPPED: built REST API URL for TIMOO API which was too slow
// Replaced by direct DB query: getAnisettaTenants
generaTenantIdList () {
    const tenantIds = getAnisettaTenants.data.map(tenant => tenant.as7_tenant_id);
    const idString = tenantIds.join(',');
    const url = `/orgUnits?where=type.eq('tenant').and(id.in(${idString})).and(name.ne('KlajdiandCo'))`;
    return url;
},
async listaTenants () {
    const URL = this.generaTenantIdList() ;
    await getPlaceholder.run({URL: URL});
    this.tenants = getPlaceholder.data.orgUnits;
    return getPlaceholder.data;
}
```

**Notes on current vs intended behavior:**
- Tenant list now comes directly from `as7_tenants` DB table (not TIMOO REST API)
- PBX stats come from `as7_pbx_accounting` (DB)
- JSObject computed totals (`totalUsers`, `totalSE`) move to backend response
- `KlajdiandCo` exclusion was in the REST URL builder — needs explicit DB WHERE if still relevant (Q9)

**Questions for expert:**
17. Should the Timoo page auto-load PBX stats when a tenant is selected (like Fatture auto-refreshes), or keep the explicit button click pattern?

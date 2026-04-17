# Zammu migspec — Phase B: UX pattern map

> Each source page's interaction pattern, primary intent, and logical widget groupings. Per target.

---

## coperture

### View: "Ricerca profili commerciali" (source page `Coperture`)
- **User intent:** Given a physical address, tell me which commercial network coverage profiles are available and from which operator.
- **Primary pattern:** **Cascading filter → on-demand search → results list.** Single-page workflow.
- **Widget groups:**
  1. **Filter form** (`Form1`): 4 dependent selects — Provincia → Comune → Indirizzo → Numero civico.
  2. **Action row**: "Cerca copertura" (primary), "Reset" (secondary).
  3. **Breadcrumb display** (`t_ricerca`): textual echo of selected filters, set programmatically after search.
  4. **Results list** (`Risultati`): per row — operator logo (image), technology (text), profile list (HTML-in-text), detail list (HTML-in-text).
- **Key actions:**
  - Select state → loads cities (auto).
  - Select city → loads addresses (auto).
  - Select address → loads house numbers (auto).
  - Click "Cerca copertura" → loads results + updates breadcrumb.
  - Click "Reset" → clears form.
- **Entry/exit:** Entry from sidebar. No linked flows in or out.
- **Current vs intended:**
  - **Current:** HTML rendering via string concatenation inside TEXT widgets (`formatProfili`, `formatDettagli`). Operator logos from hardcoded CDN URLs.
  - **Intended:** React components for each row; logos driven by operator master data (config/DB/asset bundle — Phase A open question #3).
- **Flags:**
  - No empty-state message in the audit — confirm UX for "no coverage at this house number".
  - The breadcrumb (`t_ricerca`) is a workaround for Appsmith's limited reactive text — in React this can be declarative from form state, no separate widget.

### Pattern classification summary
| Pattern | Present? |
|---------|----------|
| Cascading filter | ✅ |
| Server-side pagination | ❌ |
| Master-detail | Partial (result row = master; profile/detail lists are the "detail" view) |
| Form submission with validation | Minimal — only requires a house number |

---

## energia-in-dc

### Source page has **5 tabs**. In the rewrite these become **5 views** under the app (probably router-level tabs or sub-routes).

### View 1: "Situazione per rack" (current tab "Situazione per rack")
- **User intent:** Inspect a single rack's live power status and a window of historical readings.
- **Primary pattern:** **Cascading filter + date range → detail composite view** (metadata, socket gauges, paginated table, trend chart).
- **Widget groups:**
  1. Filter form: Cliente → Site → Room → Rack (cascading, same pattern as Coperture), plus `Letture Dal` / `Letture Al` datetime range (defaults: yesterday → now).
  2. Action row: "Aggiorna" (primary, fires 5 queries), "Reset".
  3. Rack metadata panel (`Container1` with 3 text blocks): name, floor/island/type/pos, order code/serial/billing type/committed ampere/billing start.
  4. Socket status list (`List1`): per-socket circular progress gauge (ampere vs. breaker-derived max; red >90%).
  5. Power readings table (`tbl_power`): server-side paginated, columns Socket ID / date / Ampere.
  6. Trend chart (`Chart1`): dual-axis ECharts line — ampere (left) + kW (right), last 2 days.
- **Key actions:** Cascade filters; submit search; paginate readings.
- **Flags:**
  - `utils.loadData()` fires fetches both awaited and fire-and-forget → widgets may render stale. Audit bug #4.
  - Progress gauge uses ampere/(maxampere/2)×100 — confirmed with expert whether intended (Phase A question #2).

### View 2: "Consumi in kW" (current tab)
- **User intent:** Chart a customer's kW consumption across time at a chosen cos φ.
- **Primary pattern:** **Parameterized analytic chart.**
- **Widget groups:**
  1. Controls row: customer select, period select (Giornaliero / Mensile), cos φ slider 70–100 default 95.
  2. Action: "Aggiorna".
  3. Chart (`Chart2`): custom ECharts bar, log-base-2 y-axis.
- **Flags:**
  - Period dropdown missing "Settimanale" but orchestration code handles it — dead branch dropped in partition.
  - Chart title embeds customer name + cos φ inside `jschart_kw.plot()` — in React this is a prop on the chart header, not a JS concatenation.

### View 3: "Addebiti" (current tab)
- **User intent:** View billing records for a given customer.
- **Primary pattern:** **Filter-select → table view.**
- **Widget groups:**
  1. Customer select (`s_cli_addebiti`).
  2. Table (`Table1`): period start/end, ampere, eccedenti, amount (EUR), PUN, coefficiente, fisso CU, importo eccedenti.
- **Flags:** No export action in audit. Confirm whether CSV/PDF export is expected in the rewrite.

### View 4: "Racks no variable" (current tab)
- **User intent:** Audit which customers / racks are not on variable billing.
- **Primary pattern:** **Master-detail table.**
- **Widget groups:**
  1. Master table (`Table2`): customer list (`anagrafiche_no_variable`).
  2. On row select → detail table (`Table3`): the racks of that customer.
- **Flags:**
  - Detail query keyed on `intestazione` (display string). Fragile — propose ID-keyed lookup (Phase A question #4).
  - No write actions — read-only audit view.

### View 5: "Consumi < 1A" (current tab "Socket a basso consumo")
- **User intent:** Find sockets consuming below a threshold — candidates for decommissioning or investigation.
- **Primary pattern:** **Form filter → results table.**
- **Widget groups:**
  1. Filter form: min ampere threshold (default 1), customer (optional, empty = all).
  2. Action: "Cerca".
  3. Results table (`Table4`): intestazione, building, room, socket name, ampere, power meter, magnetotermico, posizioni.
- **Flags:** No bulk actions (ticketing, email). Confirm whether rewrite should add any.

### Pattern classification summary
| View | Pattern |
|------|---------|
| Situazione per rack | Cascading filter + composite detail |
| Consumi in kW | Parameterized chart |
| Addebiti | Filter-select + table |
| Racks no variable | Master-detail |
| Consumi < 1A | Form filter + table |

### Cross-view notes
- Customer selector appears in 4 of 5 views with slightly different options (`s_customers`, `s_customers_kw`, `s_cli_addebiti`, `s_clienti_low_consumo`). In the rewrite: single shared `useCustomers` query with per-view filters. Dedup opportunity.
- No navigation from one view to another — each tab is self-contained.

---

## simulatori-di-vendita

### View: "Calcolatore IaaS" (source page `IaaS calcolatrice`)
- **User intent:** Given resource quantities and a channel (Diretta/Indiretta), produce a daily + monthly EUR quote; optionally export as PDF.
- **Primary pattern:** **Interactive calculator with two-tier pricing toggle + side-panel summary + PDF export.**
- **Widget groups:**
  1. Header controls: channel radio (`rg_dirindir` = D/I).
  2. Left column (summary panel, sticky-ish):
     - Title.
     - Dynamic price table (`Text2` reads `utils.templateHTML`).
     - Static inclusions blurb (`Text3`).
     - Daily total breakdown (`Text6` — categories: Computing / Storage / Sicurezza / Add On).
     - Monthly total (`Text7` — prominent, blue, bold).
  3. Right column (input form):
     - Compute: vCPU, RAM VMware, RAM KVM/Linux.
     - Storage: Primary, Secondary.
     - Security: FW standard ⚠(TEXT input), FW advanced (cap 1), private network.
     - Add-ons: OS Windows ⚠(TEXT input), MS SQL Std.
  4. Action row: "Calcola" (primary), "Azzera" (reset), "Genera PDF".
- **Key actions:**
  - Channel toggle → refresh price table display (`updatePrezzi`).
  - "Calcola" → reads inputs, computes totals, updates summary.
  - "Genera PDF" → computes + POSTs to Carbone.io + opens result in new tab.
- **Flags:**
  - Dynamic HTML embedded via `templateHTML` — in React this becomes a proper component.
  - Two inputs with wrong widget type (TEXT for numeric fields) — fix in rewrite.
  - `updatePrezzi` has an incomplete `decodifica` map (5 of 10 resources named) — price table currently doesn't show 5 line items. **Bug.**
  - PDF download flow uses `navigateTo(url, {}, 'NEW_WINDOW')` → OK, but the Carbone URL is constructed client-side. Audit recommends backend proxy.

### Pattern classification summary
| Pattern | Present? |
|---------|----------|
| Live-recompute-on-input | No — explicit "Calcola" button. Confirm whether rewrite should recompute on change instead. |
| Side-by-side input/summary | ✅ |
| PDF export via third-party render | ✅ (Carbone.io) |
| Multi-tier pricing toggle | ✅ |

---

## Expert-review checkpoints (deferred to consolidated review)

- Coperture: empty-state message for no coverage.
- Energia — addebiti export (CSV/PDF)?
- Energia — any bulk actions on low-consumption sockets?
- Simulatori — recompute live vs. on-button?
- Simulatori — persist quote history?

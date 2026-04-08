# Phase B: UX Pattern Map — Listini e Sconti

## Page Classification

| # | Page | Interaction Pattern | Primary User Intent |
|---|------|-------------------|---------------------|
| 1 | Home | Static splash | Display branding |
| 2 | Kit di vendita | Master-detail read-only + export | Browse kits, view components, generate PDF |
| 3 | IaaS Prezzi risorse | Customer filter + form editor | Set per-customer IaaS pricing |
| 4 | IaaS Credito omaggio | Inline-edit table + batch save | Allocate credit to CloudStack accounts |
| 5 | Sconti variabile energia | Customer filter + inline-edit table + batch save | Manage rack energy discounts |
| 6 | Gruppi di sconto x clienti | Master-detail + modal many-to-many editor | Manage customer-group associations |
| 7 | Gestione credito cliente | Customer filter + read-only ledger + modal form | View credit balance/transactions, add entries |
| 8 | Timoo prezzi indiretta | Customer filter + form editor | Set per-customer Timoo pricing |

---

## Per-Page UI Sections

### 1. Home
- Single section: branding image
- No interaction

### 2. Kit di vendita

**Current state (Appsmith):** Two flat tables side by side — kit list + product list. No metadata block. The salesperson has to mentally reconstruct the kit card from table columns.

**Target state:** Redesign as a **kit card view** that mirrors the printed PDF layout (see `artifacts/kit Unbreakable CORE.pdf`). The page should feel like browsing through kit data sheets, not reading a spreadsheet.

**Proposed layout:**

```
┌──────────────────────────────────────────────────────────┐
│  Kit list (left panel, narrow)    │  Kit detail (right)  │
│                                   │                      │
│  🔍 [Search/filter]              │  ┌────────────────┐  │
│  ─────────────────                │  │ ACCESSO        │  │
│  ● Unbreakable CORE              │  │ Unbreakable    │  │
│    Unbreakable PRO                │  │ CORE           │  │
│    Unbreakable ULTRA              │  │ [category tag] │  │
│    Ethernet Access BASIC          │  └────────────────┘  │
│    ...                            │                      │
│                                   │  Metadati            │
│                                   │  ┌────────────────┐  │
│                                   │  │ Durata: 36m    │  │
│                                   │  │ Rinnovi: 12m   │  │
│                                   │  │ Attivaz: 60gg  │  │
│                                   │  │ Fatt: 2 mesi   │  │
│                                   │  │ Sconto max: 10%│  │
│                                   │  │ Fatt var: NO   │  │
│                                   │  │ H24: SI        │  │
│                                   │  │ SLA: 12h       │  │
│                                   │  └────────────────┘  │
│                                   │                      │
│                                   │  Note                │
│                                   │  Lo SLA H24 con...   │
│                                   │                      │
│                                   │  Prodotti            │
│                                   │  ┌────────────────┐  │
│                                   │  │Gruppo│Nome│NRC│MRC│
│                                   │  │──────│────│───│───│
│                                   │  │Circ. │Fib.│590│650│
│                                   │  │prim. │    │   │   │
│                                   │  │...   │    │   │   │
│                                   │  └────────────────┘  │
│                                   │                      │
│                                   │  [Genera PDF] [Help] │
│                                   │                      │
│                                   │  "Prezzi IVA esclusa"│
└──────────────────────────────────────────────────────────┘
```

**UI Sections:**

| Section | Content | Source |
|---------|---------|-------|
| Kit list (master, left) | Filterable/searchable list of kits, grouped by category. Category name + color badge. | `get_kit_list` |
| Kit header (detail, top) | Category label + kit name, styled like the PDF header | `tbl_kit.selectedRow` |
| Metadata block | Key-value grid: durata, rinnovi, attivazione, ciclo fatturazione, sconto massimo, fatturazione variabile (SI/NO), assurance H24 (SI/NO), SLA ore | `tbl_kit.selectedRow` fields |
| Notes | Free-text notes block (if present) | `tbl_kit.selectedRow.notes` |
| Product table | Grouped by `group_name`. Columns: Nome interno, NRC (EUR), MRC (EUR). Required products marked. | `get_kit_products` |
| Actions | "Genera PDF" button + "Supporto" link (if help URL exists) | `carboneIO`, `get_kit_help` |
| Footer | "Tutti i prezzi presenti sono IVA esclusa" | Static |

**Design notes:**
- Left panel: compact list (~250px), searchable, category grouping with colored badges
- Right panel: scrollable card layout that mirrors the PDF structure
- Metadata displayed as a clean key-value grid (2 columns), not a table row
- Product table grouped by `group_name` with visual separation between groups
- Boolean fields displayed as "SI"/"NO" with semantic color (green/gray)
- The detail panel should feel like a **digital version of the printed kit card**
- Reference PDF: `artifacts/kit Unbreakable CORE.pdf`

**Event chain:** Kit select → load products + help → detail panel updates; PDF button always visible (disabled if no kit selected); Supporto visible only if help URL exists.

### 3. IaaS Prezzi risorse

| Section | Widgets | Role |
|---------|---------|------|
| Customer filter | `sl_cliente` SELECT | Active billing customers (Grappa) |
| Pricing form | `js_form_prezzi` JSON_FORM (7 fields) | Edit prices with min/max validation |
| Action | Form submit | UPSERT prices + HubSpot audit |

**Event chain:** Customer select → load prices (or defaults) → reset form → edit → save → audit note.

### 4. IaaS Credito omaggio

| Section | Widgets | Role |
|---------|---------|------|
| Account table | `tbl_accounts` TABLE | All active accounts; `credito` editable only for CloudStack |
| Action | `Button1` "Salva modifiche" | Batch save (disabled if no edits) |

**Event chain:** Page load → all accounts visible → inline edit → batch save → HubSpot audit per row.

### 5. Sconti variabile energia

| Section | Widgets | Role |
|---------|---------|------|
| Customer filter | `s_cliente` SELECT + `Button2` "Cerca" | Select customer, manual trigger |
| Rack table | `tbl_racks` TABLE | `sconto` editable (0–20%), other columns read-only |
| Action | `Button1` "Salva modifiche" | Batch save (disabled if no edits) |

**Event chain:** Customer select → click "Cerca" → racks appear → inline edit discount → save → HubSpot note + task.

**Flag:** Manual "Cerca" button is inconsistent with other pages that auto-load on selection.

### 6. Gruppi di sconto x clienti

| Section | Widgets | Role |
|---------|---------|------|
| Customer list (master) | `tbl_customers` TABLE + icon button "Associa" | Select customer |
| Group associations (detail) | `tbl_groups` TABLE | Groups linked to selected customer |
| Kit discounts (sub-detail) | `tbl_kit` TABLE | Discounts for selected group (read-only) |
| Edit modal | Modal1: `sl_groups` MULTI_SELECT + save/close | Manage group memberships |

**Event chain:** Customer row select → load associations + kit discounts → click "Associa" → modal with checkboxes → save diff → refresh.

### 7. Gestione credito cliente

| Section | Widgets | Role |
|---------|---------|------|
| Customer filter | `sl_customers` SELECT + `Button3` "Aggiorna" | Select customer, manual refresh |
| Credit balance | (from `get_customer_credit`) | Current balance (read-only, updated by external jobs) |
| Transaction ledger | `Table1` TABLE | Immutable transaction history, newest first |
| Add transaction | `Button4` "Nuova transazione" → Modal1 | Modal form: amount, sign, description |

**Event chain:** Customer select → click "Aggiorna" → load balance + transactions → click "Nuova transazione" → fill modal → confirm → insert + refresh.

**Flag:** Manual "Aggiorna" is inconsistent — other pages auto-load on selection.

### 8. Timoo prezzi indiretta

| Section | Widgets | Role |
|---------|---------|------|
| Customer filter | `sl_customers` SELECT (default=-1 for defaults) | All customers (Mistra) |
| Pricing form | `JSONForm1` (2 fields: user_month, se_month) | Edit Timoo pricing |
| Action | Form submit | Save pricing (currently buggy INSERT) |

**Event chain:** Customer select → auto-load prices → edit → save.

**Critical bug:** Read query hardcodes `customer_id = 110`. Page is non-functional.

---

## Shared UX Patterns

| Pattern | Pages | Consolidation |
|---------|-------|---------------|
| **Customer dropdown** | 5 pages (IaaS Prezzi, Sconti, Gestione credito, Timoo, Gruppi sconto) | Reusable `CustomerSelector` component (3 variants: Grappa active, Mistra all, Mistra ERP-linked) |
| **Inline-edit + batch save** | IaaS Credito, Sconti energia | Reusable `BatchEditTable` with dirty-row tracking + save button |
| **Form-based editor** | IaaS Prezzi, Timoo | Reusable `PricingForm` with backend validation |
| **Master-detail** | Kit di vendita, Gruppi sconto | Reusable layout pattern |
| **Modal form entry** | Gruppi sconto, Gestione credito | Reusable `ModalForm` |
| **HubSpot audit on save** | IaaS Prezzi, IaaS Credito, Sconti energia | Backend `AuditService` — not a UI component |

---

## Merge/Split/Rename Recommendations

**Merging:** Keep all 7 functional pages separate. Commonality is in UX patterns, not domain — merge at component level.

**Splitting:** None needed.

**Renaming (suggestions):**

| Current | Proposed | Rationale |
|---------|----------|-----------|
| Kit di vendita | Catalogo Kit | Clearer: read-only catalog |
| IaaS Prezzi risorse | Prezzi IaaS | Shorter |
| IaaS Credito omaggio | Crediti IaaS | Shorter |
| Sconti variabile energia | Sconti Energia Rack | More specific |
| Gruppi di sconto x clienti | Gruppi Sconto | Shorter |
| Gestione credito cliente | Credito Cliente | Shorter |
| Timoo prezzi indiretta | Prezzi Timoo | Shorter |

**Removal:** Home page removed — navigation itself serves as landing.

---

## Navigation Design Decision

The app has 7 functional pages — too many for a flat horizontal `TabNav` (existing apps max out at 5 tabs). Two options were evaluated:

- **Option A: Sidebar collassabile** — scales to 20+ items, but breaks consistency with other mini-apps
- **Option B: Header con dropdown a gruppi** — extends the existing horizontal pattern with grouped categories

**Decision: Option B** — grouped horizontal tabs with dropdown menus, organized by business function (not by underlying system).

### Navigation Groups

| Group | Pages | Rationale |
|-------|-------|-----------|
| **Catalogo** | Kit di vendita | Product catalog browsing + PDF export |
| **Prezzi** | IaaS Prezzi risorse, Timoo Prezzi Partner | Per-customer pricing management |
| **Sconti** | Gruppi sconto, Sconti Energia | Discount management (groups + racks) |
| **Crediti** | Crediti Omaggio IaaS, Gestione crediti | Credit allocation and ledger |

**Notes:**
- Groups are by **operation type**, not by technical system (Grappa vs Mistra)
- "Catalogo" has a single page — hover shows dropdown (for discovery), click navigates directly to the only page
- If pages are added in the future, they slot into the appropriate group
- Requires extending `TabNav` in `packages/ui/` to support grouped items with dropdowns

### Implementation Impact
- Extend `TabNav` component to support `TabNavGroup` with children
- Reuse existing dropdown animation (`ease-spring`, `scale(0.98→1)`)
- Active state: highlight both group label and active page in dropdown
- Mobile: groups collapse into hamburger menu with expandable sections

---

## UX Inconsistencies to Resolve

| Issue | Pages | Recommendation |
|-------|-------|---------------|
| Manual "Cerca" button | Sconti energia | **RESOLVED:** Remove button. No query on page load; auto-load racks on customer select. Avoids useless initial query. |
| Manual "Aggiorna" button | Gestione credito | **RESOLVED:** Remove button. No query on page load; auto-refresh balance + transactions on customer select. |
| Icon button "Associa" (%) not obvious | Gruppi sconto | Add tooltip or label |

---

## Phase B Questions for Domain Expert

### B1. Should IaaS Prezzi and IaaS Credito be on one tabbed page, or separate? Same admins or different?
### ~~B2.~~ RESOLVED. No query on page load; auto-load on customer select. Remove "Cerca" button.
### ~~B3.~~ RESOLVED. Same pattern: no query on page load; auto-refresh on customer select. Remove "Aggiorna" button.
### B4. Who exports Kit PDFs — sales team, technical team, or customers?
### ~~B5.~~ RESOLVED. Single kit export only in this phase. Bulk export tracked in `docs/TODO.md` for future.
### ~~B6.~~ RESOLVED. Simple feedback: spinner during save → toast success/error. No per-row progress.
### ~~B7.~~ RESOLVED. Keep all rows visible. Non-CloudStack rows shown read-only with reduced opacity (muted/dimmed style) to visually distinguish them from editable CloudStack rows.
### ~~B8.~~ RESOLVED. No min/max constraints for Timoo pricing.
### ~~B9.~~ RESOLVED. Hardcoded 10000 max. Keep as-is for now.
### ~~B10.~~ RESOLVED. Page names already defined as part of the navigation group design (Catalogo, Prezzi, Sconti, Crediti groups with specific page labels).

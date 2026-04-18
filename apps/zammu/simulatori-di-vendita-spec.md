# Simulatori di Vendita — Application Specification

## Summary
- **Application name:** Simulatori di Vendita
- **Portal category:** MKT&Sales (Mktg & Sales)
- **Audit source:** `apps/zammu/ZAMMU-AUDIT.md` §2.5 (source page: `IaaS calcolatrice` inside Zammu Appsmith app)
- **Spec status:** Ready for hand-off to `portal-miniapp-generator`
- **Last updated decisions (2026-04-17):**
  - Pricing moves to a backend database table and a dedicated admin UI within the app.
  - Generated quotes are **not persisted**; the app is stateless aside from the pricing master data.
  - Carbone.io PDF rendering is proxied through the backend (API key and template ID live in backend config).
- **Resolved decisions (2026-04-18):**
  - Display names: reuse the existing Appsmith strings — merge `decodifica` (5 entries) with form widget `label` props (the other 5: `Firewall standard`, `Firewall advanced`, `Private network`, `O.S. Windows Server`, `MS SQL Server std`); fix the `"1 GB Primary Storage}"` typo.
  - `fw_adv` cap (and all min/max constraints) is **UI-only** — no server-side rejection beyond basic numeric/non-negative validation.
  - Monthly total uses **30 days exact** (matches current behavior).
  - **No audit metadata** on the pricing table — this is a simple internal simulator with two rows of rates, not a regulated record. Edits overwrite in place.
  - Carbone template hash is **unchanged** from the source value captured in the audit; it lives in backend config.
  - **Live recompute** on every input change; the "Calcola" button is dropped (or kept as a no-op visual cue, TBD by UX).
- **v1 scope note:** This app is named "Simulatori di Vendita" (plural) to anticipate additional sales simulators in future iterations. v1 contains exactly one simulator (IaaS) plus a pricing admin view.

## Current-State Evidence
- **Source pages/views:** Single Appsmith page `IaaS calcolatrice` with form, live computed summary, and a "Genera PDF" action.
- **Source entities and operations:** PricingTier (hardcoded two tiers × ten resources), ResourceQuantity (form state), CostCalculation (computed totals), PDFQuote (Carbone.io render).
- **Source integrations and datasources:** Carbone.io REST (`/render/{templateId}`). No DB.
- **Known audit gaps or ambiguities:**
  - `updatePrezzi()` uses an incomplete `decodifica` display-name map — only 5 of 10 resources are shown in the current price table; the other 5 display names must be defined in the rewrite.
  - `i_fw_standard` and `i_os_windows` are TEXT widgets used in numeric multiplication — fix to NUMBER in the rewrite.
  - `hours = 730` is declared but unused; `days = 30` drives the monthly total. Whether 30 (exact) or ~30.4167 (730/24) is intended is unconfirmed.

## Entity Catalog

### Entity: PricingTier
- **Purpose:** Per-channel daily rates for the 10 IaaS resource line items.
- **Operations:**
  - `list()` — return both tiers.
  - `get(tier_code)` — single tier (`diretta` or `indiretta`).
  - `update(tier_code, rates)` — admin-only; rewrite the rates for one tier.
- **Fields and inferred types:**
  - `tier_code` (string, PK — `diretta` | `indiretta`).
  - `display_name` (string).
  - 10 per-resource daily rates (EUR, numeric with 3 decimals sufficient):
    - `vcpu`, `ram_vmware`, `ram_os`, `storage_pri`, `storage_sec`, `fw_std`, `fw_adv`, `priv_net`, `os_windows`, `ms_sql_std`.
- **Relationships:** 1 → N quotes computed at runtime; not persisted.
- **Constraints and business rules:**
  - Current rates (from audit §2.5) are the v1 seed.
  - No audit metadata on rate edits — overwrite in place (resolved 2026-04-18).

### Entity: ResourceQuantity
- **Purpose:** User-supplied quantities per resource.
- **Operations:** Local form state (not persisted).
- **Fields:**
  - `vcpu` (int, ≥1, required)
  - `ram_vmware` (int GB, ≥0)
  - `ram_os` (int GB, ≥0)
  - `storage_pri` (int GB, ≥10, required)
  - `storage_sec` (int GB, ≥0, required)
  - `fw_std` (int, ≥0) — fix from TEXT widget.
  - `fw_adv` (int, 0..1)
  - `priv_net` (int, ≥0)
  - `os_windows` (int, ≥0) — fix from TEXT widget.
  - `ms_sql_std` (int, ≥0)
- **Constraints (resolved 2026-04-18):** All min/max (incl. `fw_adv` ≤ 1) are UI-only; backend validates only numeric type and non-negative values.

### Entity: CostCalculation
- **Purpose:** Derived daily and monthly totals given quantities and a tier.
- **Operations:** `compute(quantities, tier_code)`.
- **Fields (derived):**
  - Per-line: `lineTotal_resource = qty × tier.rate_resource`.
  - Category subtotals: `computing` (vcpu + ram_vmware + ram_os), `storage` (storage_pri + storage_sec), `sicurezza` (fw_std + fw_adv + priv_net), `addon` (os_windows + ms_sql_std).
  - `totale_giornaliero` — sum of all line totals.
  - `totale_mensile` — `totale_giornaliero × 30`.
- **Constraints and business rules:**
  - Monthly multiplier fixed at 30 (matching current behavior; confirmed 2026-04-18).
  - `toFixed(2)` is a display concern only; never round before final summation.

### Entity: PDFQuote
- **Purpose:** One-shot rendered PDF from the calculation payload.
- **Operations:** `render(quantities, tier_code)` — backend computes totals, POSTs to Carbone.io, returns the PDF.
- **Fields:** Not persisted. The request payload carries `qta`, `prezzi`, `totale_giornaliero`; Carbone template hash lives in backend config.
- **Constraints and business rules:**
  - API key and template ID live only in backend config; **never** in the frontend bundle.
  - No quote history stored.

## View Specifications

### View 1: "Calcolatore IaaS"
- **User intent:** Given resource quantities and a channel (Diretta/Indiretta), produce a daily and monthly EUR quote; optionally export as PDF.
- **Interaction pattern:** Interactive calculator with tier toggle; summary panel + input form; PDF export action.
- **Main data shown or edited:** Form inputs on the right; dynamic price table, daily breakdown, prominent monthly total on the left.
- **Key actions:**
  - Toggle tier → refresh price table.
  - Any input change → live recompute totals (resolved 2026-04-18; "Calcola" button is removed).
  - "Genera PDF" → POST to backend proxy; browser opens the returned PDF.
  - "Azzera" → reset form.
- **Entry and exit:** Entry from portal sidebar. Exit on PDF download (optional) or navigation away.
- **Current vs intended:**
  - Current: frontend-hardcoded prices, dynamic HTML string for price table, incomplete display-name map, two TEXT-typed numeric inputs, client-side construction of the Carbone download URL.
  - Intended: prices fetched from backend, price table as a React component with complete display-name map, all numeric inputs are NUMBER, backend returns the PDF directly.

### View 2: "Gestione listino IaaS" (Pricing admin)
- **User intent:** Maintain the two pricing tiers.
- **Interaction pattern:** Editable grid per tier with save + audit trail (last updated by/at).
- **Main data shown or edited:** For each tier: the 10 per-resource daily rates in EUR.
- **Key actions:**
  - Edit cell → Save → confirmation.
- **Entry and exit:** Entry from within the app, behind a role check (`app_simulatorivendita_admin` proposed).
- **Current vs intended:** **New** — no equivalent in the source.

## Logic Allocation

### Backend responsibilities
- Own the pricing DB table (new) — CRUD for admin, read for calculator.
- Proxy Carbone.io rendering: accept quantities + tier, compute authoritative totals, POST to Carbone, stream the PDF back.
- Hold Carbone API key and template ID in config.
- Enforce role checks:
  - `app_simulatorivendita_access` for the calculator + PDF endpoints.
  - `app_simulatorivendita_admin` for the pricing admin endpoints.
- Validate numeric inputs server-side before rendering.

### Frontend responsibilities
- Calculator form UX (NUMBER inputs throughout), live-recompute on change.
- Price-table rendering from tier data (no HTML string concatenation).
- Pricing admin UI (editable grid) for users with the admin role.
- Show the 10 complete resource display names (complete the `decodifica` map).

### Shared validation or formatting
- Per-resource min/max constraints defined once and shared between form validation and API schema.

### Rules being revised rather than ported
- Pricing moves from frontend constant to DB + admin UI.
- Carbone call moves to backend.
- TEXT-typed inputs become NUMBER.
- Incomplete display-name map is completed.
- Totals computation lives on both frontend (display) and backend (authoritative for PDF); divergence triggers a 400.

## Integrations and Data Flow

### External systems and purpose
- Carbone.io REST — PDF rendering. Accessed only by the backend.

### End-to-end user journeys
- **Calculator flow:** open → fetch pricing → fill inputs → totals update → optional PDF.
- **Admin flow:** open admin view (role-gated) → edit rates → save → audit metadata updated.

### Background or triggered processes
- None.

### Data ownership boundaries
- Pricing master data owned by this app's backend module.
- PDFs are transient; no storage.

## API Contract Summary

### Required capabilities
- Pricing read + admin write.
- PDF render proxy.

### Read endpoints or queries (proposed shape)
- `GET /api/simulatori-vendita/iaas/pricing` — returns both tiers with metadata.

### Write commands or mutations
- `PUT /api/simulatori-vendita/iaas/pricing/{tier_code}` — admin only (`app_simulatorivendita_admin`). Accepts the 10 rates; returns the updated tier.
- `POST /api/simulatori-vendita/iaas/quote` — user role (`app_simulatorivendita_access`). Accepts quantities + tier; returns a PDF stream (or `application/pdf` with `Content-Disposition: attachment`).

### Derived or workflow-specific operations
- Server-side recomputation of totals before rendering (authoritative).

## Constraints and Non-Functional Requirements

### Security or compliance
- Keycloak OIDC. Proposed roles:
  - `app_simulatorivendita_access` — calculator + PDF render.
  - `app_simulatorivendita_admin` — pricing management.
- Carbone API key and template hash in backend config only.
- All numeric form inputs validated server-side.

### Performance or scale
- Calculator is single-user interactive; pricing payload tiny.
- PDF render latency depends on Carbone.io; surface a loading state and a timeout.

### Operational constraints
- Backend requires outbound access to Carbone.io.
- New DB migration for the pricing table.

### UX or accessibility expectations
- Portal conventions per `docs/UI-UX.md`.
- Side-by-side input/summary layout preserved.
- Numeric inputs with step, min/max.

## Open Questions and Deferred Decisions

- **Q1. ~~Display names~~ (resolved 2026-04-18).** Use existing Appsmith strings: `decodifica` map (5) + form widget `label` props (the other 5). Fix the `"1 GB Primary Storage}"` typo. No Product input required.
- **Q2. ~~Input constraints~~ (resolved 2026-04-18).** All min/max are UI-only; backend validates only numeric type and non-negativity.
- **Q3. ~~Monthly multiplier~~ (resolved 2026-04-18).** 30 days exact.
- **Q4. ~~Recompute UX~~ (resolved 2026-04-18).** Live recompute on every input change; "Calcola" button removed.
- **Q5. ~~Audit depth~~ (resolved 2026-04-18).** No audit on the pricing table — this is an internal simulator, not a regulated record. Edits overwrite in place.
- **Q6. ~~Carbone template hash~~ (resolved 2026-04-18).** Hash unchanged from the source value captured in the audit; backend config seeds it directly. Carbone Studio account ownership remains a runbook concern, not a code-shape concern.

## Acceptance Notes

- **What the audit proved directly:** Widget tree, hardcoded pricing, calculation logic, Carbone.io request shape, template hash, incomplete `decodifica`, TEXT widget bugs.
- **What the expert confirmed (2026-04-17):** Pricing → backend DB + admin UI; no quote persistence; Carbone proxied by backend.
- **What the expert confirmed (2026-04-18):** Display names sourced from existing Appsmith strings (Q1); UI-only input constraints (Q2); 30-day monthly multiplier (Q3); live-recompute UX (Q4); no audit on pricing edits (Q5); Carbone template hash unchanged (Q6).
- **What still needs validation:** None — all spec-level questions resolved. Remaining open items belong to the implementation plan (DB choice, backend module location, error contracts, role provisioning, New App Checklist edits).

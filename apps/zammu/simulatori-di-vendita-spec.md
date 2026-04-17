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
  - `updated_at`, `updated_by` (audit metadata).
- **Relationships:** 1 → N quotes computed at runtime; not persisted.
- **Constraints and business rules:**
  - Current rates (from audit §2.5) are the v1 seed.
  - All rate changes are logged with `updated_at` / `updated_by` for audit.

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
- **Open questions:** Confirm whether `fw_adv` cap 1 is a hard constraint or a UI default.

### Entity: CostCalculation
- **Purpose:** Derived daily and monthly totals given quantities and a tier.
- **Operations:** `compute(quantities, tier_code)`.
- **Fields (derived):**
  - Per-line: `lineTotal_resource = qty × tier.rate_resource`.
  - Category subtotals: `computing` (vcpu + ram_vmware + ram_os), `storage` (storage_pri + storage_sec), `sicurezza` (fw_std + fw_adv + priv_net), `addon` (os_windows + ms_sql_std).
  - `totale_giornaliero` — sum of all line totals.
  - `totale_mensile` — `totale_giornaliero × 30`.
- **Constraints and business rules:**
  - Monthly multiplier fixed at 30 (matching current behavior); revisit only if Product requests a change.
  - `toFixed(2)` is a display concern only; never round before final summation.
- **Open questions:** Confirm monthly = 30 days vs average (Q3).

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
  - "Calcola" → recompute totals (recommended change: recompute on input change as well, confirm in Q4).
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
  - View last-update metadata.
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
- `PUT /api/simulatori-vendita/iaas/pricing/{tier_code}` — admin only (`app_simulatorivendita_admin`). Accepts the 10 rates; returns updated tier with audit metadata.
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
- New DB migration for the pricing table (and its audit metadata).

### UX or accessibility expectations
- Portal conventions per `docs/UI-UX.md`.
- Side-by-side input/summary layout preserved.
- Numeric inputs with step, min/max.

## Open Questions and Deferred Decisions

- **Q1.** Complete list of display names for all 10 resources (current app only shows 5 names).
  - *Needed input:* product copy.
  - *Decision owner:* Product.
- **Q2.** Final min/max per input (especially whether `fw_adv` ≤ 1 is a hard rule).
  - *Decision owner:* Product.
- **Q3.** Monthly total: 30 days exact vs 730/24 average?
  - *Decision owner:* Product / Finance.
- **Q4.** Live recompute on every input change vs require the "Calcola" button (current behavior).
  - *Decision owner:* Product.
- **Q5.** Should pricing admin track a full changelog (per-field before/after), or just `updated_at` + `updated_by`?
  - *Decision owner:* Product / Compliance.
- **Q6.** Carbone template versioning: is the current template still the canonical one? Where is it edited?
  - *Decision owner:* Product / Marketing.

## Acceptance Notes

- **What the audit proved directly:** Widget tree, hardcoded pricing, calculation logic, Carbone.io request shape, template hash, incomplete `decodifica`, TEXT widget bugs.
- **What the expert confirmed (2026-04-17):** Pricing → backend DB + admin UI; no quote persistence; Carbone proxied by backend.
- **What still needs validation:** Display-name completion (Q1), input constraints (Q2), monthly multiplier semantics (Q3), recompute UX (Q4), changelog depth (Q5), Carbone template ownership (Q6).

# Quotes Application — Implementation Specification

## Summary

- **Application name**: Quotes (Proposte di Vendita)
- **Audit source**: `apps/quotes/APPSMITH-AUDIT.md`
- **Spec status**: Complete — all questions resolved, UX recommendations adopted
- **Phase documents**: `quotes-migspec-phaseA.md` (entities), `phaseB.md` (UX patterns), `phaseC.md` (logic placement), `phaseD.md` (integrations), `phaseE-ux.md` (UX recommendations)
- **Last updated**: 2026-04-09

### Scope

**IN SCOPE**: Quote creation (unified wizard), editing, publishing to HubSpot, deletion, listing with filters/pagination.

**OUT OF SCOPE**: Order conversion ("Converti in ordine") — deferred to phase 2. E-signature — abandoned experiment. Carbone.io PDF — abandoned experiment. Landing page/dashboard — deferred to after first version. Quote duplication — deferred to after first version.

### Key Architectural Decisions

| Decision | Rationale |
|---|---|
| Unified creation wizard (Standard + IaaS) | Same users use both flows. Single entry point with type selector. IaaS fields removed (not disabled). |
| HubSpot quote created only at explicit publish | Appsmith two-step (wizard→Dettaglio) was a workaround. No orphan empty quotes on HS. |
| Explicit save with dirty-state indicator | No auto-save. Three-layer dirty feedback: tab dots, amber banner, save button state. |
| Publish with idempotent retry | Step-by-step progress. On error: clear message + "Riprova". Steps check current state before acting. |
| Server-side pagination | 25 rows fixed, ~1000 quotes/year growth. |
| Template business rules in DB | New nullable columns on `quotes.template` (coexists with Appsmith). |
| All business rules enforced backend | Frontend mirrors rules for UX, backend is enforcement point. |

---

## Current-State Evidence

### Source pages/views (Appsmith)

| Page | Complexity | Migrated as |
|---|---|---|
| Home | Trivial | Dropped (landing deferred) |
| Elenco Proposte | Medium | `/quotes` — filterable paginated list |
| Dettaglio | Very High | `/quotes/:id` — tabbed quote editor |
| Nuova Proposta | High | `/quotes/new` — unified wizard (Standard) |
| Nuova Proposta IaaS | Medium-High | `/quotes/new` — unified wizard (IaaS path) |
| Converti in ordine | Medium-High | OUT OF SCOPE |

### Source integrations

| System | Role | In scope |
|---|---|---|
| Mistra PostgreSQL | Primary data store | YES |
| Alyante MS SQL Server | ERP lookups (payment, orders) | YES (read-only) |
| HubSpot CRM REST API | Quote publishing, status sync | YES |
| Vodka MySQL | Order creation | NO (deferred) |
| GW internal CDLAN | Order PDF generation | NO (deferred) |
| Carbone.io | PDF rendering | NO (abandoned) |

### Known audit gaps resolved

| Gap | Resolution |
|---|---|
| Template table had only 5 rows in dump | Full registry: 13 templates (4 standard + 8 IaaS + 1 legacy) |
| Stored procedure source unavailable | Full DDL extracted from `mistra_quotes.json` |
| `HS_utils` module source unavailable | Extracted from `hubspot-integrations-main.zip` |
| `gpUtils` module source unavailable | Out of scope (order conversion deferred) |
| Category exclusion inconsistency (12,13 vs 12,13,14,15) | Correct rule: always exclude 12,13,14,15 for standard flow |

---

## Entity Catalog

### Entity: Quote (head)

- **Table**: `quotes.quote` — 41 columns, ~986 rows
- **Purpose**: Central entity. Sales proposal tied to a HubSpot deal and customer.
- **Operations**:
  - CREATE — `POST /api/quotes/v1/quotes` → `quotes.ins_quote_head(json)`. Creates complete quote (header + kit rows + product expansion via trigger). No HS call.
  - READ one — `GET /api/quotes/v1/quotes/:id`
  - READ list — `GET /api/quotes/v1/quotes` — paginated, filterable (status, owner, search, date range). Joins hubs_company, hubs_deal, hubs_owner.
  - UPDATE — `PUT /api/quotes/v1/quotes/:id` → `quotes.upd_quote_head(json)`. Validates: COLOCATION→billing lock, IaaS field lock, spot→MRC=0.
  - DELETE — `DELETE /api/quotes/v1/quotes/:id`. RBAC-gated (Keycloak role). Orchestrates: HS delete (if `hs_quote_id` set) → DB delete. Atomic: HS failure blocks DB delete.
  - PUBLISH — `POST /api/quotes/v1/quotes/:id/publish`. Idempotent 5-step orchestration: save → validate → HS create/update → line item sync → status update.

- **Key fields**: `id` (PK), `quote_number` (unique, auto `SP-{seq}/{YYYY}`), `customer_id` (→ hubs_company), `hs_deal_id`, `owner`, `document_type` (TSC-ORDINE-RIC | TSC-ORDINE), `proposal_type` (NUOVO | SOSTITUZIONE | RINNOVO), `template` (→ quotes.template), `status` (DRAFT | PENDING_APPROVAL | APPROVED | APPROVAL_NOT_NEEDED | ESIGN_COMPLETED), `services` (comma-separated category IDs), `bill_months`, `initial_term_months`, `next_term_months`, `nrc_charge_time`, `payment_method`, `description` (HTML), `notes` (HTML, legal — triggers PENDING_APPROVAL), `trial`, `hs_quote_id`, contact reference fields (10 columns: rif_ordcli, rif_tech_*, rif_altro_tech_*, rif_adm_*).

- **Fields NOT exposed**: `ragione_sociale` (always NULL, legacy), `hs_esign_enabled`, `hs_esign_contacts`, `hs_sign_status`, `hs_esign_date` (e-signature removed).

- **Relationships**: 1:N → quote_rows, 1:0..1 → quote_customer (auto-trigger). Logical FKs: customer_id → hubs_company, hs_deal_id → hubs_deal, owner → hubs_owner, template → quotes.template, payment_method → erp_metodi_pagamento.

- **Triggers**: `set_timestamp` (BEFORE UPDATE → updated_at), `update_quote_customer_from_erp` (auto-snapshot ERP customer data).

### Entity: Quote Row (Kit)

- **Table**: `quotes.quote_rows` — 10 columns, ~1985 rows
- **Purpose**: Kit instance attached to a quote. Totals auto-computed by trigger from included products.
- **Operations**:
  - CREATE — `POST /api/quotes/v1/quotes/:id/rows` → INSERT triggers product expansion from kit template
  - READ — `GET /api/quotes/v1/quotes/:id/rows`
  - UPDATE position — `PUT /api/quotes/v1/quotes/:id/rows/:rowId/position`
  - DELETE — `DELETE /api/quotes/v1/quotes/:id/rows/:rowId` (CASCADE to products)

- **Key fields**: `id` (PK), `quote_id` (FK CASCADE), `kit_id` (FK RESTRICT → products.kit), `internal_name`, `nrc_row` / `mrc_row` (auto-computed), `position`, `hs_line_item_id` / `hs_line_item_nrc` (HS sync state).

- **Triggers**: `insert_product_rows_trigger` (expands kit → products with translations, appends kit legal notes to quote), `update_kit_product_rows_trigger` (re-expands on kit_id change).

### Entity: Quote Row Product

- **Table**: `quotes.quote_rows_products` — 14 columns, ~24314 rows
- **Purpose**: Individual product options within a kit row. Grouped by `group_name`. Mutual exclusion: one `included = true` per group.
- **Operations**:
  - AUTO-CREATE — via trigger on quote_rows INSERT
  - READ grouped — `GET /api/quotes/v1/quotes/:id/rows/:rowId/products` → view `v_quote_rows_products`
  - UPDATE — `PUT /api/quotes/v1/quotes/:id/rows/:rowId/products/:productId` → `quotes.upd_quote_row_product(json)`. Enforces: mutual exclusion in group, quantity floor (0→1 if included), MRC=0 for spot.
  - VALIDATE — backend checks required products before publish

- **Key fields**: `id` (PK), `quote_row_id` (FK CASCADE), `product_code` (FK RESTRICT), `group_name`, `included` (boolean), `required` (boolean), `nrc` / `mrc` (numeric 14,5), `quantity`, `extended_description` (HTML), `main_product` (boolean), `minimum` / `maximum`.

- **Trigger**: `trigger_update_quote_row_totals` (recalculates parent row nrc_row/mrc_row after any change).

- **Business rule**: `upd_quote_row_product` sets `included = false` for all products in the same group before setting the new one. Future multi-select tracked in `docs/TODO.md`.

### Entity: Template

- **Table**: `quotes.template` — 8 columns (3 existing + 5 new), 13 rows
- **Purpose**: HubSpot quote template registry with business rule configuration.
- **Migration**: ADD nullable columns for Appsmith coexistence:

```sql
ALTER TABLE quotes.template
  ADD COLUMN template_type varchar(16) DEFAULT 'standard',  -- 'standard' | 'iaas' | 'legacy'
  ADD COLUMN kit_id bigint REFERENCES products.kit(id),
  ADD COLUMN service_category_id integer,
  ADD COLUMN is_colo boolean DEFAULT false,
  ADD COLUMN is_active boolean DEFAULT true;
```

- **Operations** (new app):
  - List standard: `WHERE template_type = 'standard' AND is_active AND is_colo = :has_colocation AND lang = :lang`
  - List IaaS: `WHERE template_type = 'iaas' AND is_active AND lang = :lang`
  - Derive kit + services: `SELECT kit_id, service_category_id WHERE template_id = :id`
  - Derive T&C variant: from `(template_type, is_colo, lang)`

- **Full registry**: See Phase A for complete 13-row data with new column values.

### Entity: Quote Customer (auto-managed)

- **Table**: `quotes.quote_customer` — 12 columns
- **Purpose**: ERP billing data snapshot. Auto-populated by trigger. Not read or written by Quotes app UI.
- **Operations**: None from app. Trigger handles everything.

### Reference Entities (read-only)

| Entity | Table | Used for |
|---|---|---|
| Kit | `products.kit` | Kit picker (active, non-ecommerce, quotable) |
| Product Category | `products.product_category` | Service selector (excl 12,13,14,15 for standard) |
| Customer | `loader.hubs_company` | Customer selector, list join |
| Deal | `loader.hubs_deal` | Deal picker (pipeline/stage filtered) |
| Owner | `loader.hubs_owner` | Owner selector, list join |
| Payment Method | `loader.erp_metodi_pagamento` | Payment selector |
| Order (Alyante) | `Tsmi_Ordini` via MSSQL | SOSTITUZIONE order picker (filtered by customer) |

---

## View Specifications

### View: Quote List (`/quotes`)

- **User intent**: Find, filter, and act on existing quotes. Create new ones.
- **Interaction pattern**: Filterable paginated data table with contextual actions.

**Layout** (top to bottom):
1. Page header: "Proposte" title + "Nuova proposta" primary CTA
2. Filter bar: status pills (Tutte/Bozza/In approvazione/Approvate) + search (debounced 300ms) + advanced filters (collapsible)
3. Active filter chips (removable)
4. Data table (accent bar rows, 56px height, rowEnter stagger animation)
5. Pagination (25 fixed, prev/next)

**Table columns**: Accent bar | Numero (mono, sortable default desc) | Data (DD/MM/YYYY) | Cliente | Deal | Owner (abbreviated) | Stato (badge) | Kebab menu

**Row actions** (kebab): Apri, Elimina (role-gated), Duplica (future, grayed out)

**Presets**: "Le mie proposte" (owner = current user), "Recenti" (last 30 days)

**Filter state in URL**: `?status=DRAFT&q=CDLAN&owner=123` — shareable, preserved on back

**Status badges**: DRAFT→gray, PENDING_APPROVAL→amber, APPROVED→green, APPROVAL_NOT_NEEDED→light green "Pronta", ESIGN_COMPLETED→gray "Firmata" (legacy)

**Loading**: Skeleton rows (8, shimmer). Filter change: opacity fade + progress bar (not full skeleton swap).

**Empty states**: "Nessuna proposta ancora" + CTA (no data) / "Nessun risultato" + "Cancella filtri" link (filtered)

### View: Creation Wizard (`/quotes/new`)

- **User intent**: Create a complete new quote (header + kits + products) in one flow.
- **Interaction pattern**: Multi-step wizard with horizontal stepper and fixed bottom nav bar.

**URL stays `/quotes/new`** throughout (steps are client-side state). Abandon protection via `beforeunload`.

**Steps**:

| # | Name | Content | Gate |
|---|---|---|---|
| 1 | Deal | Deal search + card selection | Deal selected |
| 2 | Configurazione | Type selector (Standard/IaaS toggle cards) + conditional fields | Required fields valid |
| 3 | Kit e Prodotti | Kit picker + accordion product configurator | ≥1 kit, required products configured |
| 4 | Extra (opzionale) | Description RTE, legal notes RTE (with PENDING_APPROVAL warning), contact cards | None (optional) |
| 5 | Riepilogo | Full summary with NRC/MRC totals, "Modifica" links per section | User confirms |

**Type selector** (Step 2): Two 120px toggle cards. IaaS selection **removes** irrelevant fields entirely (not disables). IaaS-specific: language toggle → template → auto-derived kit/services/terms shown in info card.

**Kit/Product configurator** (Step 3): Accordion panels per kit. Each panel shows: drag handle, kit name, NRC/MRC totals, required badge ("2/3 obbligatori"). Expanded: product groups with radio-button variant selection. Optional groups have "Non incluso" option. Kit reordering via drag-and-drop.

**Post-creation**: Save to DB as DRAFT (no HS call) → navigate to `/quotes/:id` → success toast "Proposta SP-XXXX/2026 creata"

### View: Quote Detail Editor (`/quotes/:id`)

- **User intent**: Edit, refine, and publish an existing quote.
- **Interaction pattern**: Tabbed workspace with sticky action bar.

**Page structure**:
1. AppShell header (60px)
2. Quote header bar (sticky, 56px): ←back + quote number + status badge + action buttons
3. TabNav: Intestazione | Kit e Prodotti | Note e Condizioni | Contatti
4. Dirty state banner (conditional, amber)
5. Tab content (max-width 1200px)

**Tabs with dot indicators**: Orange dot = dirty, Red dot = validation issues.

**Action bar buttons**:
- Salva (primary, enabled when dirty, flash-green on success 600ms)
- Pubblica su HubSpot / Ripubblica (secondary+accent, enabled when saved + validated. Disabled tooltip explains why.)
- Apri su HS (link, visible when hs_quote_id set)
- PDF (link, visible when PDF URL available)

**Tab 1 — Intestazione**: Sectioned form (Deal e Proprietà, Tipo Proposta, Servizi e Template, Condizioni Commerciali). 2-column grid. IaaS fields replaced with read-only info cards. SOSTITUZIONE shows replace_orders with animated reveal.

**Tab 2 — Kit e Prodotti**: Same accordion UI as wizard. Add/remove kit, drag reorder, per-product auto-save. NRC/MRC totals animate on change (fade-swap).

**Tab 3 — Note e Condizioni**: Description RTE + legal notes RTE with PENDING_APPROVAL warning banner (always visible, amber when notes have content).

**Tab 4 — Contatti**: Four contact cards in 2-column grid (name/phone/email per contact type).

**Dirty state**: Three-layer system — tab dots + amber banner + save button state. `beforeunload` on navigate away.

**Keyboard shortcuts**: Cmd+S save, Cmd+Enter publish, 1-4 tab jump, / search (list), ? cheat sheet.

### View: Publish Flow (modal within Detail)

- **User intent**: Push the quote to HubSpot with confidence.
- **Interaction pattern**: Confirmation modal → morphs to progress stepper → success/error.

**Pre-publish confirmation**: Modal showing quote summary + NRC/MRC + status preview (APPROVED or PENDING_APPROVAL). If legal notes: amber warning line.

**Progress view** (modal morphs, doesn't close/reopen): Vertical stepper with 5 steps:
1. Salvataggio dati
2. Validazione prodotti
3. Creazione/Aggiornamento offerta HubSpot
4. Sincronizzazione prodotti (sub-progress: "4/10 line items")
5. Aggiornamento stato

Step icons: green check (done), indigo spinner (in progress), gray circle (pending), red X (error).

**Error**: Failed step shows error message + "Riprova" (primary) + "Chiudi" (secondary). Retry is safe (idempotent).

**Success**: Animated checkmark (SVG stroke draw, 600ms) + "Pubblicazione completata" + "Apri su HubSpot" (primary) + "Chiudi" (secondary). Calm, not celebratory.

---

## Logic Allocation

### Backend responsibilities (Go)

| Category | Items |
|---|---|
| **RBAC** | Delete authorization via Keycloak role |
| **Business rules** | Status determination (notes→PENDING_APPROVAL), MRC=0 for spot, COLOCATION→trimestrale, IaaS field lock, Colo template blocked for spot, quantity floor, category exclusion (12-15), kit ecommerce exclusion, pipeline/stage filtering, default payment 402, quote expiry (date+30d) |
| **Data operations** | All CRUD via stored procedures/SQL. Quote number generation. replace_orders serialization. cli_orders customer filter. |
| **Orchestration** | Unified quote creation (header + rows + products in transaction). HS publish (5-step idempotent). HS+DB delete (atomic). Line item bidirectional sync. |
| **Content generation** | T&C HTML generation (6 variants from template_type + is_colo + lang). Migrate Appsmith content verbatim — contractual text. |
| **External API** | All HubSpot REST calls (credentials never exposed to frontend). Alyante MSSQL queries. |

### Frontend responsibilities (React)

| Category | Items |
|---|---|
| **Presentation** | Status badge color mapping, field enable/disable cascade (derived from template_type, not hardcoded IDs), conditional visibility (SOSTITUZIONE→replace_orders, IaaS→remove fields), product group display helpers |
| **UX feedback** | Dirty state detection + 3-layer indicator, required product badges ("2/3 obbligatori"), filter pills with counts, skeleton/loading states, publish progress display, NRC/MRC total animations |
| **Interactions** | Wizard step navigation + gates, accordion expand/collapse, drag-and-drop kit reorder, keyboard shortcuts, inline kit delete confirmation (3s timeout), search debounce |
| **Trial text** | IaaS trial text generation from slider value (presentation-only) |

### Shared contracts

| Contract | Values |
|---|---|
| Status enum | `DRAFT` \| `PENDING_APPROVAL` \| `APPROVED` \| `APPROVAL_NOT_NEEDED` \| `ESIGN_COMPLETED` |
| Document type | `TSC-ORDINE-RIC` (recurring) \| `TSC-ORDINE` (spot) |
| Proposal type | `NUOVO` \| `SOSTITUZIONE` \| `RINNOVO` |
| Template type | `standard` \| `iaas` \| `legacy` |
| NRC charge time | `1` (all'ordine) \| `2` (all'attivazione) |

---

## Integrations and Data Flow

### External systems

| System | Access | Direction | Credentials |
|---|---|---|---|
| Mistra PostgreSQL | Go `pgx` (existing pool) | Read + Write (`quotes.*` only) | Backend config |
| Alyante MS SQL Server | Go `go-mssqldb` (pattern from listini) | Read-only, on-demand | Backend config |
| HubSpot CRM REST | Go HTTP client | Bidirectional (publish writes, status reads) | Backend-only OAuth/private app token |

### Cross-system identity

```
HubSpot Company ID (loader.hubs_company.id)
  = quotes.quote.customer_id
  = loader.erp_anagrafiche_clienti.numero_azienda
  = Alyante NUMERO_AZIENDA
```

### Data freshness

- `loader.*` tables synced by external ETL process. Quotes app consumes, does not control sync.
- HS→DB status sync handled by external processes. Quotes app reads from local DB.

### DB triggers (automatic, not reimplemented)

| Trigger | Effect |
|---|---|
| `set_timestamp` | Auto-updates `quote.updated_at` |
| `update_quote_customer_from_erp` | Snapshots ERP customer data to `quote_customer` |
| `insert_product_rows_trigger` | Expands kit → products with translations + legal notes |
| `update_kit_product_rows_trigger` | Re-expands on kit_id change |
| `trigger_update_quote_row_totals` | Recalculates row NRC/MRC from included products |

---

## API Contract Summary

### Read endpoints

| Endpoint | Purpose |
|---|---|
| `GET /api/quotes/v1/quotes` | Paginated quote list (filter: status, owner, q, date_from, date_to; sort; page) |
| `GET /api/quotes/v1/quotes/:id` | Full quote header |
| `GET /api/quotes/v1/quotes/:id/rows` | Kit rows for quote |
| `GET /api/quotes/v1/quotes/:id/rows/:rowId/products` | Grouped products for kit row |
| `GET /api/quotes/v1/quotes/:id/hs-status` | Current HS quote status + PDF link |
| `GET /api/quotes/v1/deals` | Active deals (pipeline/stage filtered) |
| `GET /api/quotes/v1/deals/:id` | Deal detail with ERP cross-ref |
| `GET /api/quotes/v1/customers` | Company list |
| `GET /api/quotes/v1/owners` | Owner list |
| `GET /api/quotes/v1/templates` | Template list (filter: type, lang, is_colo) |
| `GET /api/quotes/v1/categories` | Product categories (excl 12-15 for standard) |
| `GET /api/quotes/v1/kits` | Active quotable kits |
| `GET /api/quotes/v1/payment-methods` | Payment methods |
| `GET /api/quotes/v1/customer-payment/:customerId` | Alyante default payment |
| `GET /api/quotes/v1/customer-orders/:customerId` | Alyante orders (customer-filtered) |

### Write endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/quotes/v1/quotes` | Create complete quote (header + kit rows). Returns full quote. |
| `PUT /api/quotes/v1/quotes/:id` | Update quote header (validates business rules) |
| `POST /api/quotes/v1/quotes/:id/rows` | Add kit row (trigger expands products) |
| `DELETE /api/quotes/v1/quotes/:id/rows/:rowId` | Delete kit row (CASCADE) |
| `PUT /api/quotes/v1/quotes/:id/rows/:rowId/position` | Update row ordering |
| `PUT /api/quotes/v1/quotes/:id/rows/:rowId/products/:productId` | Update product (mutual exclusion, MRC=0, qty floor) |
| `DELETE /api/quotes/v1/quotes/:id` | Delete quote (RBAC + HS delete + DB delete) |

### Action endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/quotes/v1/quotes/:id/publish` | Full HS publish orchestration (idempotent, returns step progress) |

**Total: ~17 endpoints** replacing 30+ Appsmith queries and 8 JSObjects.

---

## Constraints and Non-Functional Requirements

### Security
- Server-side RBAC for delete (Keycloak role `app_quotes_delete`)
- All HubSpot credentials backend-only
- No direct DB access from frontend
- Alyante orders filtered by customer (fixes Appsmith bug)

### Performance
- Server-side pagination (25 rows/page)
- Reference data (customers, owners, templates, categories, kits, payment methods) cacheable
- Alyante queries on-demand (customer selection), not page load
- HS publish: ~2N+5 API calls for N kit rows. Rate limit: 100 req/10s.

### Coexistence with Appsmith
- No schema changes that break Appsmith queries
- `quotes.template` new columns are nullable with defaults — Appsmith ignores them
- `services` remains comma-separated string (Appsmith MultiSelect format)
- `ragione_sociale` column kept but not used
- `hs_esign_*` columns kept but not exposed
- All triggers unchanged

### UX
- Italian language default
- Stripe-inspired "clean" theme
- 44px minimum touch targets
- `prefers-reduced-motion` respected
- Keyboard shortcuts (Cmd+S, Cmd+Enter, /, ?)
- Skeleton loading states (never spinners for data loading)

---

## Bug Fixes (resolved by new architecture)

| Bug | Fix |
|---|---|
| `salvaOfferta` template condition always true (`!=` OR chain) | Clean switch/if in Go |
| Missing closing quote in `isDisabled` (VCloud EN) | Template type from DB, no hardcoded IDs |
| `recuperaLingua()` tautology | Proper null check in Go |
| `hs_sender_email` undefined | Owner lookup from DB |
| `cli_orders` unscoped | Customer filter (A7) |
| Category exclusion inconsistency (12,13 vs 12,13,14,15) | Always 12,13,14,15 (A5) |
| Client-side RBAC only | Server-side Keycloak (1.1) |
| Non-atomic HS+DB delete | Backend orchestration (1.2) |
| `i_next_term_months` type mismatch | Typed smallint + number input |
| Alyante payment query on empty ID | On-demand at customer selection |
| HubSpot expiry date mutation | Immutable calculation |
| `==` vs `===` in role check | Typed comparison in Go |

---

## Dead Code — Not Migrated

`nuovo_numero_offerta`, `hs_update_quote` (Elenco), `hs_associa_contatto` (Elenco), `Query1` (vodka), `contattiPerEsignature`, `test_hs2()`, `newQuoteAssociations()` dead branch, `inserisci_righe = false` block, 4 CRUD auto-generated queries (IaaS), `render_template` (Carbone.io), `firmaForm` (entire JSObject), `xmlParser` library, Converti in ordine (entire page).

---

## Component Inventory

### Reuse from `@mrsmith/ui`

AppShell, TabNav (extend with dot indicators), Modal, Toast/ToastProvider, MultiSelect, SingleSelect, Skeleton, ToggleSwitch

### Build new (app-specific)

StatusBadge, QuoteTable (accent bars, sort headers), FilterBar (status pills + search + advanced), Pagination (prev/next + count), Stepper (horizontal, wizard), StepperProgress (vertical, publish flow), KitAccordion (expandable kit panel + product config), ProductGroupRadio (radio variants within group), DirtyBanner (amber unsaved warning), ContactCard (grouped contact form), RichTextEditor (Tiptap or Lexical), ConfirmDialog (themed modal wrapper), KeyboardShortcuts (global handler + cheat sheet), DealCard (wizard step 1), TypeSelector (Standard/IaaS toggle cards)

---

## Open Questions and Deferred Decisions

**All questions resolved.** No open items.

### Deferred to after first version
- Landing page / dashboard (B1)
- Quote duplication (B2)
- Pipeline/stage configurability (A6)
- Multi-product selection per group (TODO.md)

---

## Acceptance Notes

### What the audit proved directly
- Complete entity model with all fields, types, constraints from DDL
- All 30+ Appsmith queries with exact SQL
- All JSObject logic including bugs and dead code
- Full HubSpot API surface with endpoints and methods
- Complete business rule catalog (25 rules)
- Security vulnerabilities (client-side RBAC, non-atomic writes)

### What the expert confirmed
- Order conversion deferred to phase 2
- E-signature removed (failed experiment)
- Carbone.io removed (failed experiment)
- Mutual exclusion per product group is correct (multi-select future TODO)
- Category exclusion: always 12,13,14,15
- Template business rules moved to DB (new columns, Appsmith coexistence)
- `cli_orders` filtered by customer
- Pipeline/stage IDs as backend constants
- `ragione_sociale` is a legacy residue
- `services` stays comma-separated for coexistence
- `APPROVAL_NOT_NEEDED` is a HubSpot-set status (read-only)
- `ESIGN_COMPLETED` is legacy (neutral display, no logic)
- Unified wizard (Standard + IaaS, same users)
- Explicit save with dirty-state indicator
- Hybrid validation (save always, publish blocks on missing required)
- Idempotent retry for publish errors
- Server-side pagination from day one
- Full filter system with presets
- UX recommendations adopted integrally

### What still needs validation
- T&C HTML content: must be migrated verbatim from Appsmith `templates.terms_and_conditions()` — contractual text, needs legal review of the ported content
- HubSpot association type IDs (286 for template, 64 for deal, 71 for company) — verify against current HS configuration
- Alyante MSSQL connection details and VPN requirements — obtain from infrastructure team
- HubSpot OAuth/private app token configuration — obtain from HubSpot admin

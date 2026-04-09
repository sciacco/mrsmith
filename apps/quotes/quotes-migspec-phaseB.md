# Quotes — Phase B: UX Pattern Map

**Source**: `apps/quotes/APPSMITH-AUDIT.md`, `docs/UI-UX.md`
**Date**: 2026-04-09
**Scope**: Quotes/proposals only (order conversion deferred)

---

## In-Scope Pages

| # | Current Appsmith Page | In Scope | Notes |
|---|---|---|---|
| 1 | Home | YES (replace) | Static splash — replace with proper landing |
| 2 | Elenco Proposte | YES | Quote list + CRUD hub |
| 3 | Dettaglio | YES | Full quote editor (5 tabs) |
| 4 | Nuova Proposta | YES | Standard quote creation wizard |
| 5 | Converti in ordine | **NO** | Deferred to order conversion phase |
| 6 | Nuova Proposta IaaS | YES | IaaS quote creation wizard |

---

## 1. Home (Splash Page)

### Current state
- Static decorative image + text "Questo è il tempo dell'attesa"
- Zero functionality, zero queries
- Purpose: placeholder/vanity page

### Interaction pattern: **None** (dead page)

### Migration recommendation
Replace with a **dashboard landing** that provides at-a-glance value:
- Recent quotes (last 5–10, quick-access)
- Status distribution (DRAFT / PENDING / APPROVED counts)
- Quick actions: "Nuova Proposta" button

**Question B1**: Should the landing page show KPIs (quote volume, conversion rate, average deal size) or is a simple recent-quotes list sufficient? What does the user see first thing in the morning — what's the most valuable view?

---

## 2. Elenco Proposte (Quote List)

### Current state
**Primary user intent**: Find, open, or act on an existing quote.

#### Current widget grouping

| Section | Widgets | Purpose |
|---|---|---|
| Title bar | Text1 ("Elenco proposte") | Static header, hardcoded blue |
| Toolbar | ButtonGroup1 (Modifica, Nuova, Altro dropdown) | Actions |
| Data table | tbl_quote (TABLE_V2) | Main list bound to `get_quotes.data` |

#### Current interaction pattern: **List → action hub**
- Table displays all quotes (LIMIT 2000, no server pagination)
- Row selection → enables toolbar actions
- **Modifica** → navigate to Dettaglio (stores `v_offer_id`)
- **Nuova** → navigate to Nuova Proposta
- **Altro** dropdown: Duplica (disabled), Aggiorna lista, Cancella offerta, Converti in ordine
- Delete is role-gated (client-side only): "Administrator - Sambuca" or "Kit and Products manager"
- Delete flow: HS delete (if exists) → DB delete → refresh. Non-atomic, double confirmation dialogs.

#### Visible columns

| Column | Label | Format | Notes |
|---|---|---|---|
| quote_number | Numero | — | Primary identifier |
| document_date | Data documento | DD/MM/YYYY | |
| cliente | Cliente | — | Company name from join |
| deal_number | Deal | — | HubSpot deal number |
| deal_name | Nome deal | — | HubSpot deal name |
| owner_name | Owner | — | first_name + last_name |
| status | Status | Color-coded cell bg | DRAFT=default, PENDING_APPROVAL=orange, APPROVED=green, unknown=red |

Hidden: `id`, `hs_quote_id`, `customer_id`

### Recommended UX pattern: **Filterable data table with contextual actions**

Following the Stripe-like design system in `docs/UI-UX.md`:

| Aspect | Current (Appsmith) | Recommended |
|---|---|---|
| Layout | Full-width table, toolbar above | AppShell + content area, max-width 1400px |
| Pagination | Client-side (LIMIT 2000) | Server-side with backend cursor/offset |
| Filtering | None | Status filter tabs/pills + search field (quote number, customer, deal) |
| Sorting | Table column sort (client) | Server-side sort on key columns |
| Row actions | Toolbar ButtonGroup (must select first) | Row-level actions: click to open, contextual menu for delete/duplicate |
| Status display | Cell background color | Status badges (pill-shaped, color-coded per design system) |
| Loading | None visible | Skeleton rows per UI-UX.md |
| Empty state | None | Centered icon + message per design system |
| Delete | Double confirm dialogs, client-side RBAC | Single confirm modal (native `<dialog>`), server-side RBAC |
| Row animation | None | `rowEnter` with stagger per UI-UX.md |

#### UI sections for the new view

1. **Page header**: Title "Proposte" + primary action button "Nuova proposta"
2. **Filter bar**: Status filter pills (Tutti / Bozza / In approvazione / Approvate) + search input
3. **Data table**: Sortable columns, row click → navigate to detail, accent bar on hover
4. **Row context menu** (or trailing icon button): Elimina, Duplica (future)

**Question B2**: The current "Duplica offerta" button is disabled. Is quote duplication a desired feature for this migration, or still deferred?

**Question B3**: Do users commonly search/filter by owner (to see "my quotes")? Should there be an "I miei" filter or ownership is implicit?

**Question B4**: 2000-row LIMIT with no server pagination — is the dataset growing beyond this? Current count is ~976. Should we implement proper server-side pagination now, or is the dataset small enough that client-side filtering works for the foreseeable future?

---

## 3. Dettaglio (Quote Editor)

### Current state
**Primary user intent**: Edit all aspects of a quote, then publish to HubSpot.

This is the most complex page (~60% of app complexity). Five tabs, 35+ queries, 8 JSObjects, touching 4 external systems.

**Entry**: Hidden page, only via `navigateTo()` from Elenco or creation wizards. State passed via `appsmith.store.v_offer_id`.

#### Current tab grouping

| Tab | Label | Visible | User intent |
|---|---|---|---|
| 1 | Dettagli | Yes | Edit quote header (customer, deal, type, terms, billing) |
| 2 | Righe | Yes | Manage kit rows and configure products |
| 3 | Note | Yes | Edit description and legal notes |
| 4 | Firma | Hidden in tab bar | Manage e-signature contacts |
| 5 | Riferimenti | Yes | Edit contact references (5 groups) |

Plus top-level actions: Salva offerta, Pubblica su Hubspot, Apri su HS, Scarica PDF, Torna a elenco.

### Interaction pattern: **Tabbed form editor with master-detail sub-view**

#### Tab 1: Dettagli (Header Form)

**Pattern**: Single-record form with interdependent fields

| Section | Widgets | Notes |
|---|---|---|
| Deal & ownership | sl_deal, sl_owner, sl_customer | Dropdowns, filterable |
| Document metadata | i_document_date, sl_type_document, sl_proposal_type | Type drives field enable/disable cascade |
| Services & template | sl_services (multi), sl_template | Dependent on type. Disabled for IaaS/VCloud |
| Billing terms | sl_payment_method, sl_fatturazione_canoni_, sl_mod_fatt_attivazione, i_initial_term_months, i_next_term_months | Conditional disable based on type + services |
| Replacement orders | i_replace_orders (multi) | Visible only for SOSTITUZIONE |
| Actions | Salva, Pubblica su Hubspot, Open on HS, Download PDF | Publish disabled when template invalid or ESIGN_COMPLETED |

**Hidden business rules embedded in widget states**:
- 8 IaaS/VCloud template IDs → disable services, template, billing, term fields (10+ `isDisabled` expressions)
- `document_type = TSC-ORDINE` → disable term/billing fields
- COLOCATION service → force Trimestrale billing (3 months)
- SOSTITUZIONE → show/require replace_orders field
- Status is always read-only

**Key UX problems**:
1. Field interdependencies are invisible — user doesn't know why fields are disabled
2. No visual distinction between "not applicable for this quote type" and "not editable"
3. Status dropdown exists but is always disabled — confusing affordance
4. "Pubblica su Hubspot" triggers a 16-step background orchestration with no progress feedback
5. Missing closing quote bug in `isDisabled` for VCloud EN template

#### Tab 2: Righe (Kit Rows + Product Config)

**Pattern**: Master-detail with nested product editor

| Section | Widgets | Purpose |
|---|---|---|
| Left pane | tbl_quote_rows | Kit row list (name, NRC, MRC, position) |
| Left pane toolbar | Add kit (+), Delete kit (trash), Back to list | Row management |
| Right pane | tbl_products | Product groups for selected kit (group_name, quotato SI/-) |
| Right pane detail | frm_details (variant picker, NRC/MRC/qty, description, include switch, save) | Per-product editor |
| Add kit modal | mdl_new_kit (kit picker, confirm) | New kit row creation |

**Interaction flow**:
1. Select kit row in left table → loads product groups in right table
2. Select product group → loads product variant options in detail form
3. Choose variant from dropdown → NRC/MRC update
4. Toggle "Quotare?" switch → mark as included (mutual exclusion within group)
5. Save → `upd_quote_row_product()` + trigger recalculates row totals
6. Row position is inline-editable

**Key UX problems**:
1. Three-level hierarchy (kit list → product groups → product variants) is crammed into a single view with complex conditional visibility
2. Required products marked only with red cell background — subtle visual cue
3. No validation feedback until publish attempt (required products not included)
4. NRC/MRC forced to 0 for spot orders — happens silently in save handler
5. Position reordering is raw number input, not drag-and-drop

#### Tab 3: Note

**Pattern**: Rich-text editor (two fields)

| Section | Widget | Notes |
|---|---|---|
| Description | i_description (RTE) | "Descrizione sommaria della proposta" |
| Legal notes | i_note_legali (RTE) | "Pattuizioni Speciali". **Critical**: non-empty triggers PENDING_APPROVAL |
| Trial | trial_iaas (Input, always disabled) | IaaS trial text (read-only display) |

**Key UX problem**: No indication that writing legal notes will change the quote status to PENDING_APPROVAL. This is a significant hidden side-effect.

#### Tab 4: Firma (E-Signature)

**Pattern**: Toggle + contact selector + status display

| Section | Widget | Purpose |
|---|---|---|
| Enable toggle | sw_esignature | "E-Signature attiva?" |
| Load contacts | Button10 | Calls HS_utils1.ListCompanyContacts |
| Signer picker | msl_firmatari (MultiSelect) | Pre-populated from DB |
| Status display | Text12/Text13 | Signer list + e-sign status HTML table |

**Hidden in tab bar** — user must know it exists.

**Key UX problems**:
1. Tab is hidden — discoverability zero
2. E-signature silently disabled if no signers selected
3. ESIGN_COMPLETED blocks re-publish with no visible UI state
4. Contact loading requires manual button click (no auto-load)

#### Tab 5: Riferimenti (Contact References)

**Pattern**: Multi-group contact form

Five contact sections, each with name/phone/email:
1. `rif_ordcli` — customer order reference (name only)
2. `rif_tech_*` — technical contact
3. `rif_altro_tech_*` — alternate technical contact
4. `rif_adm_*` — administrative contact

**Key UX problem**: Flat form with 10 fields, no grouping or visual separation.

### Recommended UX pattern for Dettaglio

> **Cambio di ruolo**: Con il workflow unificato, Dettaglio diventa un **editor per offerte esistenti e complete** — non più il completamento di una creazione parziale. L'utente arriva qui con kit e prodotti già configurati dal wizard. Dettaglio serve per: modificare, rifinire, e pubblicare.

**Overall**: Tabbed workspace (using `TabNav` component from design system) with clear tab labels and a sticky action bar. Salvataggio esplicito con indicatore dirty state.

| Tab | Recommended name | Pattern |
|---|---|---|
| 1 | Intestazione | Sectioned form with field groups and contextual help |
| 2 | Kit e Prodotti | Master-detail with proper three-level navigation |
| 3 | Note e Condizioni | Rich-text editors with inline status warning |
| 4 | ~~Firma digitale~~ | **RIMOSSO** — funzionalità sperimentale abbandonata |
| 4 | Contatti | Grouped contact cards |

**Sticky action bar** (top or bottom):
- Status badge (prominent, color-coded)
- "Salva" button (disabled when clean, enabled when dirty)
- "Pubblica su HubSpot" button (disabled until saved + validated)
- Dirty-state indicator: visual cue (e.g., dot on Salva button, banner, or tab badge) when unsaved changes exist

#### Key UX improvements to design

| Area | Current problem | Recommended approach |
|---|---|---|
| Save model | Single save, no dirty indication | **Explicit save** with dirty-state indicator. User must know when data is unsaved. (B5 resolved) |
| Field interdependencies | Silent disable, no explanation | Visual sections with headers explaining quote type impact. Disabled groups show "Non applicabile per ordini spot" helper text |
| Status | Read-only dropdown, confusing | Status badge in page header (not a form field). Color-coded, prominent. |
| Legal notes → status | Hidden side effect | Inline warning banner below legal notes RTE: "La presenza di pattuizioni speciali richiede approvazione" |
| Publish to HS | 16-step background, no feedback | See B6 — pending decision on feedback level |
| Kit/product hierarchy | Cramped three-level UI | Two-column layout: kit list left, product config right. Product groups as expandable sections (accordion) rather than secondary table selection. |
| Required products | Red cell only, no pre-validation | Visual badge on kit row indicating "2/3 prodotti obbligatori configurati". Block save (not just publish) if required products missing? |
| E-signature tab | Hidden tab | Always visible. Show "Non configurata" badge if no signers. Auto-load contacts when tab opens. |
| Position reorder | Raw number input | Drag-and-drop handles, or at minimum up/down arrow buttons |
| Contact references | Flat 10-field form | Card-per-contact layout with name/phone/email grouped visually |

**Question B5**: ~~RESOLVED~~ — Salvataggio esplicito con dirty-state indicator.

**Question B6**: ~~RESOLVED~~ — Progress step-by-step visibile + **idempotent retry** per errori parziali. Ogni step è idempotente (controlla stato attuale prima di agire, salta se già completato). Su errore: messaggio chiaro su cosa è fallito + bottone "Riprova" che riesegue in modo sicuro. Il backend traccia lo stato HS via `hs_line_item_id`/`hs_quote_id` già nel DB.

**Question B7**: ~~RESOLVED~~ — E-signature rimossa dallo scope. Funzionalità sperimentale che non ha funzionato. Tab Firma eliminato, campi `hs_esign_*` non esposti nella UI.

**Question B8**: ~~RESOLVED~~ — Ibrido: salvataggio sempre permesso (anche incompleto), ma con warning visivo chiaro (badge "2/3 obbligatori" su kit row, banner di avviso). Pubblicazione HS bloccata finché tutti i prodotti obbligatori sono configurati. L'utente sa sempre a che punto è senza essere bloccato nel workflow.

---

## 4. Nuova Proposta (Standard Quote Creation)

### Current state
**Primary user intent**: Create a new standard (non-IaaS) quote linked to a HubSpot deal.

#### Current wizard steps

| Step | Name | Pattern | User task |
|---|---|---|---|
| 1 | SelectPotential | Table selection | Pick an active HubSpot deal from list |
| 2 | ConfirmGeneralData | Form | Fill document metadata, services, billing terms |
| 3 | SelectKits | Multi-select + form | Choose kits (tree widget), template, description, legal notes |

**Post-save sequence**: Generate quote number → Create HS quote → Insert DB record → Insert kit rows → Navigate to Dettaglio

#### Interaction flow

1. User sees table of active deals (filtered by hardcoded pipeline/stage IDs)
2. Select deal → "Successivo" enabled
3. Form pre-fills: deal owner, customer from deal, payment from Alyante ERP (default 402)
4. User selects document type, proposal type, services, billing terms
5. "Successivo" to step 3
6. Multi-select tree: categories (disabled, negative values) → kits (selectable, positive values)
7. Select template (filtered by document type + services)
8. Optional: description (RTE), legal notes (RTE)
9. "Salva offerta" → 10-step creation flow → redirect to Dettaglio

**Key UX problems**:
1. Tree widget with negative category values is an Appsmith workaround — semantically confusing
2. 7 queries fire on page load across 3 databases (Mistra, Alyante, HubSpot mirror)
3. No back button in wizard steps (only forward or cancel)
4. No validation between steps — services on step 2 not re-validated after step 3
5. `i_next_term_months` type mismatch (TEXT vs sibling NUMBER)
6. Dead code: `inserisci_righe = false` block, `test_hs2()`, dead deal branch in associations
7. Alyante payment query fires on page load with empty customer ID (spurious)

### Interaction pattern: **Multi-step wizard**

### Recommended UX pattern

> **DECISIONE FONDAMENTALE (expert input, 2026-04-09)**: Il flusso Appsmith in due step (wizard crea quote vuota su HS → Dettaglio configura prodotti → pubblica sincronizza HS) è un workaround Appsmith, NON un requisito di business. La nuova app deve implementare un workflow unificato che crea un'offerta **completa** — intestazione, kit, prodotti configurati — in un unico flusso coerente. La quote HubSpot non viene creata al salvataggio del wizard ma solo alla pubblicazione esplicita, quando l'offerta è completa.

Questo cambia radicalmente il confine creazione/modifica. Il wizard attuale (3 step: deal → header → kit) diventa insufficiente: deve includere anche la configurazione dei prodotti per kit.

**Stepper wizard con configurazione prodotti integrata**:

| Step | Content | Gate |
|---|---|---|
| 1 | Deal selection (searchable table or select) | Deal selected |
| 2 | Quote configuration (type, services, terms, billing, template) | Required fields valid |
| 3 | Kit selection + product configuration per kit | At least 1 kit selected, required products configured |
| 4 | Note, contatti, condizioni | — (optional content) |
| Review | Full summary before creation | User confirms |

Dopo il salvataggio: quote completa nel DB locale (status DRAFT, nessuna quote HS). La pubblicazione su HubSpot è un'azione esplicita separata (da Dettaglio o direttamente dal review step).

La pagina **Dettaglio** diventa un **editor per offerte esistenti** (non più il completamento di una creazione parziale). Questo semplifica Dettaglio: non deve più gestire lo stato "offerta appena creata, incompleta".

Key improvements:

| Area | Current | Recommended |
|---|---|---|
| Deal picker | Full table, no search | Searchable select or filtered table with type-ahead |
| Kit picker | Tree widget with negative values | Grouped multi-select (checkboxes per category) or visual kit cards |
| Product config | Only in Dettaglio (separate page) | Inline nel wizard, per ogni kit selezionato |
| Step navigation | Forward only | Back + forward with step indicators |
| Validation | Per-step only at save | Inline validation, step gates, required products validated pre-save |
| Review step | None (direct save) | Summary card con totali NRC/MRC prima di confermare |
| Loading | 7 parallel queries, no skeleton | Lazy load per step + skeleton |
| Template selection | Dropdown | Visual template cards with language badge (IT/EN) |
| HS quote creation | At wizard save (empty quote) | **Deferred to explicit publish** — no orphan HS quotes |

**Question B9**: ~~RESOLVED~~ — La quote HS viene creata solo alla pubblicazione esplicita, non al salvataggio del wizard.

---

## 5. Nuova Proposta IaaS

### Current state
**Primary user intent**: Create an IaaS-specific quote with automatic kit/services derivation from template.

#### Differences from standard wizard

| Aspect | Standard | IaaS |
|---|---|---|
| Kit selection | Manual multi-select tree (step 3) | Automatic: 1 kit derived from template |
| Services | User-selected multi-select | Auto-derived from template (hardcoded switch) |
| Template source | Filtered by doc type + services | Filtered by language + `LIKE 'IaaS%' OR 'VCLOUD%'` |
| Term fields | Editable | All disabled (fixed 1 month) |
| Trial | Not available | Slider 0–200 generating bilingual trial text |
| Document type | Selectable (TSC-ORDINE-RIC / TSC-ORDINE) | Hardcoded TSC-ORDINE-RIC |
| Language | Derived from template | User-selectable, filters templates |

#### Wizard steps

| Step | Name | User task |
|---|---|---|
| 1 | SelectPotential | Pick an active HubSpot deal (same as standard) |
| 2 | IaaS metadata | Select language → template (auto-filters). Trial slider. Billing terms pre-locked. |
| 3 | Confirm | Review kit (auto-selected) + description + legal notes |

**Key UX problems**:
1. `recuperaLingua()` bug: `!= '' || != null` is tautology — default "ITA" unreachable
2. `hs_sender_email` bug: uses `owner.selectedOptionLabel` on plain object → `undefined`
3. 7 queries across 3 databases on page load
4. Server-side pagination scaffolding (4 auto-generated CRUD queries) is dead code

### Interaction pattern: **Simplified wizard (template-driven)**

### The Standard/IaaS merge question (from A10)

Both wizards share:
- Step 1 (deal selection) — identical
- Payment method logic — identical
- Save sequence — nearly identical
- Template/T&C generation — overlapping but different variants

They differ in:
- Kit selection: manual vs. automatic
- Service selection: manual vs. template-derived
- Term fields: editable vs. locked
- Trial support: absent vs. present
- Template filtering: by type+services vs. by language+IaaS filter

**Options**:

| Option | Description | Pros | Cons |
|---|---|---|---|
| **A. Single wizard with type selector** | First choice: "Standard" or "IaaS/VCloud". Fields adapt dynamically. | DRY, single maintenance point, consistent entry | Complex conditional logic, potentially confusing mixed state |
| **B. Two entry points, shared components** | Separate routes/pages but shared step components | Cleaner per-flow UX, simpler per-page logic | Duplication of shared steps, two pages to maintain |
| **C. Keep separate** | Two independent wizards | Simplest per-page | Maximum duplication, maintenance burden |

**Question B10**: This is the key UX decision. The IaaS flow is substantially more constrained (fewer choices, locked fields). Would a single wizard with an initial type choice feel clean, or would users find it confusing to see disabled fields that apply to the "other" type? Consider: who uses each flow — same people or different teams?

---

## 6. Cross-View Navigation Map (in-scope pages only)

```
                  ┌──────────┐
                  │   Home   │
                  │(landing) │
                  └──────────┘

                  ┌──────────────────┐
         ┌──────►│ Elenco Proposte  │◄─────┐
         │       │  (quote list)    │      │
         │       └──┬────┬──────────┘      │
         │          │    │                 │
         │    Modifica  Nuova              │
         │          │    │                 │
         │          ▼    ▼                 │
         │  ┌────────┐  ┌──────────────┐   │
         │  │Dettaglio│  │Nuova Proposta│   │
         │  │(editor) │  │  (standard)  │───┘
         │  └────┬────┘  └──────────────┘
         │       │
         │       │ (back to list)
         └───────┘

                     ┌──────────────────┐
                     │Nuova Proposta IaaS│
                     │   (IaaS wizard)   │──► Dettaglio
                     └──────────────────┘
```

**Navigation observations**:
- `Elenco Proposte` is the hub — all flows return here
- `Dettaglio` is a hidden page, only reachable via programmatic navigation
- Both creation wizards redirect to `Dettaglio` after save
- State passing: `v_offer_id` in Appsmith store (Elenco→Dettaglio), URL params (future)
- Bug: `btnEsci4` in Dettaglio Firma tab navigates to `'Elenco Offerte'` (wrong page name)

### Recommended navigation model

| Pattern | Current | Recommended |
|---|---|---|
| Routing | Appsmith store + `navigateTo()` | React Router with URL params (`/quotes/:id`) |
| Deep linking | Not supported | Full support: `/quotes` (list), `/quotes/new` (wizard), `/quotes/:id` (detail) |
| Back navigation | Programmatic buttons | Browser back + breadcrumbs |
| State passing | `appsmith.store.v_offer_id` | URL path param `:id` |
| Hidden pages | `isHidden: true` | No hidden routes — detail is a parameterized route |

---

## 7. Status Color Mapping

Current Appsmith mapping (`utils.bgStatus()`):

| Status | Current display | Recommended display (per design system) |
|---|---|---|
| `DRAFT` | Default (no color) | Neutral gray badge | App scrive |
| `PENDING_APPROVAL` | Orange cell bg | Warning amber badge | App scrive (publish con note legali) |
| `APPROVAL_NOT_NEEDED` | Default (no color) | Verde chiaro / neutro badge | **HubSpot scrive** — app read-only |
| `APPROVED` | Green cell bg | Success green badge | App scrive (publish senza note legali) |
| `ESIGN_COMPLETED` | Rosso (bug) | Neutral gray badge "Firmata" | **Legacy** — stato non più producibile |
| Unknown | Red cell bg | Danger red badge | Anomalia |

**Question B11**: ~~RESOLVED~~ — `APPROVAL_NOT_NEEDED` è uno stato impostato da HubSpot (non dalla nostra app) quando l'approvazione non è necessaria. La nuova app non lo scrive mai, ma lo mostra in lista come stato valido. Trattamento visivo: badge neutro/verde chiaro, distinto da APPROVED (che implica approvazione avvenuta).

**Question B12**: ~~RESOLVED~~ — `ESIGN_COMPLETED` è uno stato legacy (e-signature rimossa). Offerte storiche con questo stato: badge neutro/informativo "Firmata" in lista, visualizzazione read-only in dettaglio. Nessuna logica di transizione stato verso/da `ESIGN_COMPLETED` nella nuova app.

---

## 8. Summary of Questions

### Landing page
**B1**: ~~DEFERRED~~ — Landing page/dashboard rimandata a dopo la prima versione eseguibile dell'app.

### Quote list
**B2**: ~~DEFERRED~~ — Duplicazione offerta da implementare dopo la prima versione eseguibile. In Appsmith il bottone esiste ma è disabilitato (mai implementata). Va progettata da zero: copia header (nuovo quote_number, reset DRAFT, no hs_quote_id) + copia kit rows + copia prodotti configurati.
**B3**: ~~RESOLVED~~ — Sistema di filtro completo: filtri liberi sui campi principali + preset predefiniti (es. "Le mie proposte" = filtro per owner corrente).
**B4**: ~~RESOLVED~~ — Server-side pagination da subito. Volumi attuali: ~986 quote, ~1985 rows, ~24314 products. Crescita prevista ~1000 quote/anno. Il costo di implementazione è basso ora, il refactor a posteriori è alto.

### Quote editor
**B5**: ~~RESOLVED~~ — Salvataggio esplicito con indicatore di dati non salvati (dirty state). No auto-save.
**B6**: ~~RESOLVED~~ — Progress step-by-step + idempotent retry per errori parziali.
**B7**: E-signature: always-visible tab, form section, or modal?
**B8**: Required-products validation at save or only at publish?

### Creation wizard
**B9**: ~~RESOLVED~~ — La quote HS viene creata solo alla pubblicazione esplicita. Il wizard salva solo nel DB locale (DRAFT completa con kit e prodotti configurati).

### Standard vs. IaaS
**B10**: ~~RESOLVED~~ — Wizard unico con selettore tipo. Stessi utenti usano entrambi i flow. Entry point unico: scelta iniziale Standard/IaaS, poi il wizard adatta campi e logica di conseguenza.

### Status display
**B11**: Is `APPROVAL_NOT_NEEDED` a distinct user-visible state?
**B12**: What should `ESIGN_COMPLETED` look like (currently red = "unknown")?

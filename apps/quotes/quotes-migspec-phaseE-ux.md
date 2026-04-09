# Quotes -- Phase E: UX Recommendations

**Source**: Phase A-D migspec documents, `docs/UI-UX.md` design system
**Date**: 2026-04-09
**Scope**: Complete UX specification for the Quotes native React app

---

## 1. Quote List (Elenco Proposte)

### 1.1 Layout and information hierarchy

The quote list is the command center. A salesperson returning to this view needs three things in under 2 seconds: where did I leave off, what needs attention, and how do I start something new.

**Page structure (top to bottom):**

1. **Page header row** -- single horizontal line containing:
   - Title "Proposte" (page title style: 1.75rem/700, letter-spacing -0.04em)
   - Right-aligned primary CTA: "Nuova proposta" (primary pill button, indigo gradient)
   - The title and button stay on the same row at all widths down to 640px; below that, button goes full-width below title

2. **Filter bar** -- immediately below the header, separated by `--space-4` (16px):
   - Left side: **Status filter pills** (horizontal row of pill-shaped toggles)
   - Right side: **Search field** (320px max-width, magnifying glass icon prefix)
   - Below on a second row (collapsible, hidden by default): **Advanced filters** -- owner select, date range, document type. Revealed by a "Filtri" link/button with a filter icon and active-filter count badge.

3. **Active filter chips** -- if any filters beyond status are active, show them as removable chips between the filter bar and table. Each chip shows the filter name + value + X button. "Cancella filtri" link at the end clears all.

4. **Data table** -- the main content area

5. **Pagination bar** -- bottom of table, sticky to viewport bottom when scrolling

**Spacing**: Page uses `--space-8` (32px) horizontal padding within the `max-width: 1400px` content area. `--space-6` (24px) vertical gap between sections.

### 1.2 Filter/search UX

**Status pills** function as a segmented control, not checkboxes. One active at a time. They act as the primary filter axis:

| Pill label | Filter value | Badge count |
|---|---|---|
| Tutte | no status filter | total count |
| Bozza | DRAFT | count |
| In approvazione | PENDING_APPROVAL | count |
| Approvate | APPROVED + APPROVAL_NOT_NEEDED | count |

Counts are returned by the backend in the list response metadata. The "Tutte" pill is selected by default.

**Pill styling**: Use `--radius-full` (999px), `--space-2` (8px) horizontal padding, height 36px. Inactive: `--color-surface` background, `--color-text-secondary` text. Active: `--color-accent-muted` background, `--color-accent` text, `font-weight: 600`. Transition: `background var(--duration-fast) var(--ease-out)`.

**Search field**: Debounced (300ms). Searches across `quote_number`, `cliente` (company name), `deal_name`, `deal_number`. Placeholder: "Cerca per numero, cliente, deal..." The search is additive to the status filter.

**Preset system**: A "Salvati" dropdown (secondary button, bookmark icon) to the right of the search field. Contains:
- "Le mie proposte" -- sets owner filter to current user
- "Recenti" -- sets date filter to last 30 days
- Custom presets can be added in a future version

Selecting a preset applies its filters and shows the corresponding chips. The preset button shows a filled bookmark icon when a preset is active.

**Filter state in URL**: All filter state is encoded in URL query params (`?status=DRAFT&q=CDLAN&owner=123`). This enables sharing filtered views via URL and preserves filters on browser back.

### 1.3 Data table columns

| Column | Label | Width | Align | Sortable | Notes |
|---|---|---|---|---|---|
| Accent bar | -- | 4px | -- | No | Design system accent bar (indigo, height animated) |
| `quote_number` | Numero | 140px | Left | Yes (default, desc) | Primary identifier. Monospace font (`--font-mono`), weight 600. |
| `document_date` | Data | 100px | Left | Yes | Format DD/MM/YYYY. `--color-text-secondary`. |
| `cliente` | Cliente | 1fr (flex) | Left | Yes | Company name. Truncate with ellipsis. Full name in tooltip on hover. |
| `deal_name` | Deal | 1fr (flex) | Left | Yes | Deal name. Truncate with ellipsis. |
| `owner_name` | Owner | 140px | Left | Yes | First name + last name initial (e.g. "Marco R.") to save space. Full name in tooltip. |
| `status` | Stato | 140px | Left | Yes | Status badge (see 1.5) |
| Row actions | -- | 44px | Center | No | Kebab menu icon (three dots) |

**Table header row**: `--color-surface` background, uppercase labels at 0.6875rem/600, `letter-spacing: 0.06em`, `--color-text-muted` color. 44px height.

**Table body rows**: 56px height, `--color-bg-elevated` background. Bottom border `1px solid var(--color-border-subtle)`. Row enter animation: `rowEnter 0.4s var(--ease-out) both` with staggered `animation-delay` (0.03s increments, max 15 rows visible = 0.45s total).

**Row hover**: `background: var(--color-accent-subtle)` (rgba(99,91,255,0.04)), accent bar height 0 to 20px with `--ease-spring`.

**Row click**: Entire row is clickable. Navigates to `/quotes/:id`. Micro-feedback: `:active` applies `scale(0.995)` for 1 frame.

**Sort behavior**: Click column header to sort. First click = descending, second = ascending, third = remove sort (back to default quote_number desc). Active sort column header gets `--color-accent` text and a directional arrow icon. Sort is server-side.

### 1.4 Row actions and selection model

**No multi-select**. The quote list does not need bulk operations. Each row stands alone.

**Row actions** via kebab menu (click the three-dot icon or right-click the row):
- "Apri" -- same as row click, navigate to detail
- "Elimina" -- visible only if user has `app_quotes_delete` role. Opens delete confirmation modal.
- "Duplica" (future, grayed out with "Disponibile a breve" tooltip)

The kebab menu is a small dropdown panel (`--shadow-lg`, `--radius-md`, `--color-bg-elevated` background). Items are 40px height, full-width. Danger item (Elimina) uses `--color-danger` text.

### 1.5 Status badges

Pill-shaped badges using `--radius-full`:

| Status | Label | Background | Text color | Icon |
|---|---|---|---|---|
| DRAFT | Bozza | `--color-surface` | `--color-text-secondary` | None |
| PENDING_APPROVAL | In approvazione | `rgba(245,158,11,0.12)` | `#b45309` (amber-700) | None |
| APPROVED | Approvata | `--color-success-bg` | `#047857` (emerald-700) | None |
| APPROVAL_NOT_NEEDED | Pronta | `rgba(16,185,129,0.08)` | `#059669` (emerald-600) | None |
| ESIGN_COMPLETED | Firmata (legacy) | `--color-surface` | `--color-text-muted` | Lock icon (12px) |
| Unknown | Errore | `--color-danger-subtle` | `--color-danger` | Alert icon (12px) |

Badge dimensions: height 26px, padding `0 var(--space-3)` (12px), font-size 0.75rem (12px), weight 600.

### 1.6 Empty states

**No quotes at all** (brand new user):
- Icon: `document` from the portal icon set, 32x32px inside 72x72px container with `--color-surface` background
- Title: "Nessuna proposta ancora" (0.9375rem/600, `--color-text-secondary`)
- Description: "Crea la tua prima proposta per iniziare" (0.8125rem, `--color-text-muted`)
- CTA: "Nuova proposta" primary button below description, `margin-top: var(--space-5)`

**No results from filter/search**:
- Icon: `funnel` icon
- Title: "Nessun risultato"
- Description: "Prova a modificare i filtri o il termine di ricerca"
- Link: "Cancella filtri" (`--color-accent`, underline on hover)

### 1.7 Loading states

**Initial load**: Skeleton rows matching exact table column layout. 8 skeleton rows, each 56px height, shimmer animation (2s loop), staggered fadeIn with 80ms delay increments. Skeleton widths randomized between 60% and 90% per cell.

**Pagination/filter change**: Do NOT replace the entire table with skeletons. Instead:
- Add `opacity: 0.5` to the current table rows (150ms transition)
- Show a thin 2px indigo progress bar at the top of the table (indeterminate, left-to-right loop)
- When data arrives, cross-fade to new rows with `rowEnter` animation

### 1.8 Pagination controls

**Bottom bar** layout: Left side shows count ("126 proposte" or "Mostra 1-25 di 126"). Right side shows page controls.

**Page controls**: Previous/Next arrow buttons (secondary style, 36px square, `--radius-md`). Between them: current page indicator "Pagina 1 di 6" in `--color-text-secondary`. Do NOT use numbered page buttons -- with server-side pagination and potentially growing data, prev/next is cleaner and avoids the awkward "1 2 3 ... 47" pattern.

**Page size**: Fixed at 25 rows. No page-size selector -- this is an opinionated choice to keep the interface simple. 25 rows gives enough context without excessive scrolling.

---

## 2. Unified Creation Wizard

### 2.1 Overall structure and navigation

The wizard uses a **horizontal stepper** at the top of the page, below the AppShell header. The stepper is sticky (sticks below the 60px header) so the user always knows where they are.

**URL structure**: `/quotes/new` loads the wizard. Each step is a client-side state change (NOT separate routes) -- this avoids browser-back confusion. The URL stays `/quotes/new` throughout.

**Step indicator**: A horizontal bar with numbered circles connected by lines:
```
  (1)-----(2)-----(3)-----(4)-----(5)
  Deal   Config   Kit    Extra   Riepilogo
```

Circle states:
- **Completed**: `--color-accent` background, white checkmark icon
- **Current**: `--color-accent` border (2px), white fill, accent number text
- **Upcoming**: `--color-border` border, `--color-text-muted` number
- **Error/incomplete**: `--color-warning` border, warning icon replaces number

Connecting lines: `--color-border` by default, `--color-accent` for completed segments. Line fills left-to-right with a 400ms transition when a step completes.

**Navigation buttons**: Fixed to the bottom of the viewport in a bar:
- Left: "Indietro" (secondary button, left arrow), hidden on step 1
- Center: Step label "Passo 2 di 5" in `--color-text-muted`
- Right: "Avanti" (primary button, right arrow) on steps 1-4, "Crea proposta" on step 5

The bottom bar has `--color-bg-elevated` background, top border, `--shadow-sm`. Height 72px. This keeps the CTA always visible without scrolling.

**Step gates**: "Avanti" is disabled (visually and functionally) until the current step's required fields are valid. The button shows a subtle shake animation (2px horizontal oscillation, 300ms) if clicked while disabled, drawing attention to the validation errors above.

**Abandon protection**: If the user has entered any data and tries to navigate away (browser back, close tab, click nav link), show a browser-native `beforeunload` confirmation. Do NOT use a custom modal for this -- the native dialog is more reliable and less intrusive.

### 2.2 Step 1: Deal Selection

**Layout**: Full-width search field at top, deal list below.

**Search field**: Auto-focused on step entry. Large (48px height), prominent placeholder: "Cerca per nome deal, codice, o cliente..." Debounced 300ms. Searches `deal_name`, `deal_number`, and associated company name.

**Deal list**: Card-based, not a table. Each deal is a card (full-width, 72px height, `--color-bg-elevated`, `--radius-md`, `--shadow-xs`):
- Left: Deal code in `--font-mono` weight 600
- Center: Deal name (primary text) + company name below (secondary text, `--color-text-secondary`, 0.8125rem)
- Right: Deal owner avatar circle (initials, 32px) + name

**Selection**: Click a card to select it. Selected card gets `--color-accent-subtle` background + `--color-accent` left border (3px). Only one deal can be selected.

**Loading**: Show 5 skeleton cards on initial load. Deals are fetched once and cached for the wizard session.

**No results**: "Nessun deal trovato" with suggestion to check pipeline status in HubSpot.

**Why cards, not a table**: Deal selection happens once per quote. The user needs to read and compare deal details. Cards give more room for each deal's information and feel less like "data entry" and more like "choosing."

### 2.3 Step 2: Quote Configuration

This step has the most fields. The key principle: **group related fields visually, and hide what is not relevant.**

**Type selector** at the top of the step: Two large toggle cards side by side:

| Card | Label | Description | Icon |
|---|---|---|---|
| Standard | Standard | "Servizi datacenter, connettivita, colocation" | `package` icon |
| IaaS | IaaS / VCloud | "Infrastruttura cloud, virtual datacenter" | `cloud` icon (or `database`) |

Cards are 120px tall, flex: 1 each, with `--radius-lg` (14px). Selected card: `--color-accent` border (2px), `--color-accent-subtle` background, subtle `--shadow-accent`. Unselected: `--color-border`, `--color-bg-elevated`. Transition on selection: 250ms ease-out for border/background.

**Choosing IaaS transforms the rest of the step.** Fields that are irrelevant get removed (not disabled, not grayed -- removed entirely). This is the key design decision: rather than showing disabled fields with "Non applicabile" text, we show only what matters. This prevents the "why can't I click this?" confusion identified in Phase B.

**Standard configuration fields** (shown when type = Standard):

| Section | Fields | Layout |
|---|---|---|
| Documento | Document type (select), Proposal type (select) | 2-column row |
| Servizi | Services (MultiSelect, categories) | Full width |
| Commerciale | Template (SingleSelect, filtered), Payment method (SingleSelect) | 2-column row |
| Durata | Initial term months (number), Next term months (number), Bill months (number) | 3-column row |
| Consegna | Delivered in days (number), NRC charge time (select) | 2-column row |

**IaaS configuration fields** (shown when type = IaaS):

| Section | Fields | Layout |
|---|---|---|
| Lingua | Language toggle: IT / EN (two buttons) | Centered, prominent |
| Template | Template (SingleSelect, auto-filtered by language + IaaS) | Full width |
| Trial | Trial slider (0-200) with live preview text | Full width |

When IaaS is selected, template selection drives everything: kit is auto-derived, services are auto-derived, terms are fixed at 1 month. The user sees a small info card below the template selector: "Kit: {derived_kit_name} | Servizi: {derived_service} | Durata: 1 mese" in a `--color-surface` box with `--radius-md`.

**SOSTITUZIONE conditional**: When `proposal_type = SOSTITUZIONE` is selected, a "Ordini da sostituire" MultiSelect slides in below the Proposal type field (300ms `--ease-out`, max-height transition). This field loads Alyante orders filtered by the deal's customer.

**COLOCATION conditional**: When services include COLOCATION, the bill_months field auto-sets to 3 and becomes read-only. A helper text appears: "Fatturazione fissata a trimestrale per Colocation" in `--color-text-muted`, italic.

**Section headers**: Use the form label style (0.75rem/600, uppercase, letter-spacing 0.06em, `--color-text-muted`). Each section separated by `--space-6` (24px).

**Customer/owner pre-fill**: Customer and owner are derived from the selected deal (step 1) and shown as read-only info chips at the top of this step: "Cliente: CDLAN S.r.l. | Owner: Marco Rossi". The user should not need to re-select these.

### 2.4 Step 3: Kit Selection and Product Configuration

This is the hardest UX challenge in the app. The hierarchy is: Quote contains Kit Rows. Each Kit Row contains Product Groups. Each Product Group contains Product Variants (mutually exclusive). Some groups are required.

**Design approach: Accordion kit panels with inline product configuration.**

**Kit selector** (top of the step):
- "Aggiungi kit" button (secondary, `+` icon) opens a kit picker popover/modal
- Kit picker: grouped by category. Each category is a section header. Kits listed as selectable rows with checkbox, name, and base NRC/MRC. The user can check multiple kits and confirm.
- For IaaS: this section is replaced with a non-interactive info card showing the auto-derived kit. No picker needed.

**Selected kits**: Each selected kit becomes an **accordion panel** in a vertical stack.

**Accordion panel anatomy**:
```
+-------------------------------------------------------------------+
| [drag handle] [expand/collapse arrow]  Kit Name              NRC | MRC |
|                                        "CONNECT PLUS"    45,00 | 12,50 |
|                                        [2/3 obbligatori]  [Remove X] |
+-------------------------------------------------------------------+
| (expanded content: product groups)                                 |
|                                                                    |
|  CONNETTIVITA (required)                                           |
|  +--------------------------------------------------------------+  |
|  | ( ) Fibra 100M          NRC 100,00  MRC 25,00  Qty [1]      |  |
|  | (*) Fibra 1G            NRC 200,00  MRC 45,00  Qty [1]      |  |
|  | ( ) Fibra 10G           NRC 500,00  MRC 120,00 Qty [1]      |  |
|  +--------------------------------------------------------------+  |
|                                                                    |
|  ROUTER (optional)                                                 |
|  +--------------------------------------------------------------+  |
|  | ( ) Cisco ISR 1100      NRC 0,00    MRC 15,00  Qty [1]      |  |
|  | ( ) Cisco ISR 4300      NRC 0,00    MRC 25,00  Qty [1]      |  |
|  | (skip) Non incluso                                           |  |
|  +--------------------------------------------------------------+  |
+-------------------------------------------------------------------+
```

**Accordion panel header**:
- 64px height, `--color-bg-elevated`, `--radius-lg` top corners (or full when collapsed)
- Left: drag handle (6-dot grid, `--color-text-faint`) for reordering + expand/collapse chevron
- Center: kit `internal_name` in weight 600
- Right: NRC/MRC totals in `--font-mono`, weight 500 + required products badge + remove button (X, `--color-danger` on hover)
- Click anywhere on the header (except remove) to expand/collapse

**Required products badge**: Shown in the header when the kit has required product groups. Format: "2/3 obbligatori" -- green when complete (all required groups have a selection), amber/warning when incomplete. This badge uses the same pill style as status badges:
- Complete: `--color-success-bg` background, success text color
- Incomplete: `rgba(245,158,11,0.12)` background, amber text

**Product group** within an expanded kit:
- Section header: group name in 0.8125rem/600, with "(obbligatorio)" tag in `--color-danger` for required groups
- Radio-button list of product variants. Each variant is a row:
  - Radio button (native, styled) on the left
  - Product name
  - NRC and MRC values (right-aligned, `--font-mono`)
  - Quantity input (number spinner, 60px wide) -- only visible for the selected variant
- For optional groups: add a "Non incluso" option at the end (radio button, italic text, `--color-text-muted`). This deselects any variant in the group.

**When a variant is selected**: The radio fills with `--color-accent`. The row gets `--color-accent-subtle` background. NRC/MRC values update in the kit header totals (computed client-side from included products). The row transition: 200ms background color change.

**MRC disabled for spot orders**: If `document_type = TSC-ORDINE`, the MRC column shows "0,00" in `--color-text-faint` with no input. A small info icon with tooltip: "MRC non applicabile per ordini spot."

**Drag-and-drop reordering**: Kit panels can be reordered by dragging the handle. During drag: the panel lifts with `--shadow-float`, `scale(1.01)`, and a 2px `--color-accent` border. Drop target shows a 2px horizontal line at insertion point. Keep it simple -- use @dnd-kit/sortable.

**Accordion auto-expand**: When a kit is first added, auto-expand it so the user immediately starts configuring products. If there is only one kit, it stays expanded.

### 2.5 Step 4: Note, condizioni, contatti (optional)

This step contains lower-priority fields that many quotes may skip entirely. The step indicator shows it as "Extra" and the stepper makes it clear this is optional (dashed connecting line, or a subtle "opzionale" label below the step circle).

**Layout**: Three collapsible sections, all collapsed by default:

1. **Descrizione** -- Rich text editor for the quote description. Compact toolbar (bold, italic, list, link). Max height 300px with scroll.

2. **Pattuizioni speciali** -- Rich text editor for legal notes. Preceded by a warning banner:
   - Banner: `rgba(245,158,11,0.08)` background, `--color-warning` left border (3px), amber text icon
   - Text: "La presenza di pattuizioni speciali imposta lo stato a 'In approvazione' alla pubblicazione"
   - This banner is always visible when the section is expanded, regardless of content

3. **Contatti di riferimento** -- Contact reference cards. Four card slots:
   - Riferimento ordine cliente (name only)
   - Contatto tecnico (name, phone, email)
   - Contatto tecnico alternativo (name, phone, email)
   - Contatto amministrativo (name, phone, email)

   Each contact is a compact card (`--color-surface` background, `--radius-md`, padding `--space-4`). Card header is the contact type label. Fields are laid out as: name full-width, phone and email side by side below.

**Section expand/collapse**: Click section header to toggle. Header shows section name + summary when collapsed (e.g., "Descrizione: 'Proposta per migrazione datacenter...'" truncated, or "Nessuna descrizione" in muted text).

### 2.6 Step 5: Review (Riepilogo)

This is the final step before creation. Its purpose: give the user confidence that everything is correct, and a moment of anticipation.

**Layout**: A single scrollable summary card (`--color-bg-elevated`, `--radius-2xl`, `--shadow-md`, padding `--space-8`).

**Sections within the card**:

1. **Header summary**: Deal name, customer, owner, quote type (Standard/IaaS badge), document date -- all as key-value pairs in a 2-column grid.

2. **Configuration summary**: Document type, proposal type, services, template name, billing terms, delivery days -- as a compact key-value list.

3. **Kit and product summary**: For each kit:
   - Kit name (bold) + NRC/MRC totals
   - Below: compact list of included products (just names and prices, no configuration UI)
   - Required products warning if any are missing (amber banner)

4. **Totals bar**: A highlighted section at the bottom:
   - Background: `--color-accent-subtle`
   - Two large numbers: "NRC Totale: EUR 1.250,00" and "MRC Totale: EUR 340,00"
   - Font: 1.25rem/700, `--color-accent`

5. **Notes/contacts summary**: Only shown if populated. Collapsed by default with "Mostra dettagli" link.

**Edit links**: Each section has a small "Modifica" link (accent text, pencil icon) that jumps back to the relevant step. This is faster than using the stepper for small corrections.

**"Crea proposta" button**: In the fixed bottom bar. Primary style, large (48px height). Label: "Crea proposta". On click:
- Button shows loading state (spinner replaces text)
- On success: navigate to `/quotes/:id` (the new quote's detail page) with a success toast
- Toast: "Proposta SP-1102/2026 creata" (success type, auto-dismiss 4s)

### 2.7 Mobile considerations

At widths below 768px:
- Stepper collapses to "Passo 2 di 5 -- Configurazione" (text-only, no circles/lines)
- Kit accordion panels stack full-width
- Product variant rows become cards instead of a table-like layout
- Type selector cards stack vertically
- Navigation bottom bar remains fixed, buttons full-width
- All form fields go single-column

---

## 3. Quote Detail Editor (Dettaglio)

### 3.1 Page structure

The detail page is the workspace where users refine and publish quotes. It should feel like a well-organized document editor, not a form.

**URL**: `/quotes/:id`

**Page structure (top to bottom)**:

1. **AppShell header** (sticky, 60px) -- standard, with "Proposte" as app name

2. **Quote header bar** (sticky below AppShell, 56px, `--color-bg-elevated`, bottom border):
   - Left: Back arrow (link to `/quotes`) + Quote number ("SP-1102/2026" in `--font-mono`, weight 600, 1rem)
   - Center: Status badge (same style as list)
   - Right: Action buttons group (see 3.5)

3. **Tab navigation** (TabNav component, immediately below header bar):
   - Tabs: Intestazione | Kit e Prodotti | Note e Condizioni | Contatti
   - Animated underline indicator slides to active tab

4. **Tab content area** -- max-width 1200px, centered, padding `--space-8`

5. **Dirty state banner** (conditional, between tabs and content):
   - Shows when there are unsaved changes
   - Full-width, 40px height
   - `rgba(245,158,11,0.08)` background, amber left border
   - Text: "Modifiche non salvate" + "Salva" link button on the right
   - Enters with slideDown animation (200ms ease-out)

### 3.2 Tab indicators

Each tab label can show a small indicator dot to the right of the text:

| Indicator | Color | Meaning |
|---|---|---|
| Orange dot (6px) | `--color-warning` | Tab has unsaved changes (dirty) |
| Red dot (6px) | `--color-danger` | Tab has validation issues (e.g., missing required products) |
| No dot | -- | Tab is clean and valid |

The dot appears/disappears with a 200ms scale animation (0 to 1). Only one type of dot per tab -- dirty takes precedence if both conditions apply (the user needs to save first, then validate).

### 3.3 Tab 1: Intestazione (Header Form)

**Layout approach**: Sections with headers, 2-column grid within each section. Not a flat list of 15+ fields.

**Sections**:

1. **Deal e Proprieta** (section header, uppercase label style)
   - Deal: read-only display (deal code + deal name as a linked value -- click opens HubSpot deal in new tab). `--color-surface` background, not an input.
   - Cliente: read-only display (company name). Same read-only style.
   - Owner: SingleSelect dropdown
   - Document date: date picker

2. **Tipo Proposta** (section header)
   - Document type: SingleSelect (TSC-ORDINE-RIC / TSC-ORDINE)
   - Proposal type: SingleSelect (NUOVO / SOSTITUZIONE / RINNOVO)
   - Replace orders: MultiSelect -- only visible when proposal_type = SOSTITUZIONE (animated reveal, 300ms max-height transition)

3. **Servizi e Template** (section header)
   - Servizi: MultiSelect (categories)
   - Template: SingleSelect (filtered by type + services + language)
   - Both hidden and replaced with read-only info card for IaaS quotes (same as wizard step 2)

4. **Condizioni Commerciali** (section header)
   - Payment method: SingleSelect
   - Fatturazione canoni: SingleSelect (billing frequency)
   - Bill months: number input
   - Initial term months: number input
   - Next term months: number input
   - Delivered in days: number input
   - NRC charge time: SingleSelect
   - For IaaS: all term/billing fields replaced with read-only values + helper: "Valori fissi per offerte IaaS/VCloud"

**Field interdependency communication**: When a field is read-only due to quote type, it shows as a static display element (not a disabled input). The section header includes a small info pill: "IaaS: valori derivati dal template" or "Spot: MRC non applicabile". This is clearer than disabled fields because it explains WHY, not just WHAT.

**2-column grid**: At widths above 768px, fields within each section use a 2-column CSS grid with `gap: var(--space-4)` (16px). Full-width fields (like services multi-select) span both columns with `grid-column: 1 / -1`.

### 3.4 Tab 2: Kit e Prodotti

This is the same accordion-based kit/product UI from the wizard (section 2.4), with additions for editing:

**Additional capabilities**:
- "Aggiungi kit" button at the top (same kit picker as wizard)
- Each kit panel has a delete button (trash icon) that opens a confirmation modal: "Eliminare il kit '{name}' e tutti i prodotti configurati?"
- Kit position reordering via drag handles
- Product configuration updates are saved per-product (not with the page-level save) -- each product change sends `PUT .../products/:productId` immediately. This is consistent with the Appsmith behavior and avoids complex client-side product state management.

**However**, the product-level save should still trigger the dirty state banner if the change affects row totals, because the parent quote needs a save to capture the updated state snapshot. The distinction:
- Product variant selection/quantity: auto-saved via API, but marks the quote as "has changes" for the purpose of the dirty banner
- Kit add/remove: API call, then re-fetch kits, mark dirty

**NRC/MRC totals in kit headers**: Update in real-time after each product save (the API response includes updated row totals from the trigger). Animate the number change: old value fades out (100ms), new value fades in (100ms) with a subtle scale pulse (1.0 to 1.05 to 1.0, 300ms).

### 3.5 Tab 3: Note e Condizioni

Same layout as wizard step 4 (section 2.5), but as a tab content area instead of a wizard step. The two rich text editors (Descrizione and Pattuizioni Speciali) are always visible (not collapsible here, since the user navigated to this tab intentionally).

The PENDING_APPROVAL warning banner for legal notes is always visible below the legal notes editor, regardless of content:
- When legal notes are empty: muted informational style
- When legal notes have content: amber warning style, bolder text

### 3.6 Tab 4: Contatti

Same contact card layout as wizard step 4 (section 2.5), but always expanded. Four contact cards in a 2-column grid (stacks to 1 column below 768px).

### 3.7 Action bar (save, publish, links)

The action bar lives in the quote header bar (row 2 of the page structure). Right-aligned button group:

| Button | Style | Condition | Behavior |
|---|---|---|---|
| Salva | Primary (indigo) | Visible always. Enabled when dirty. | Saves quote header. On success: toast + clear dirty state. |
| Pubblica su HubSpot | Secondary with accent text | Visible always. Enabled when: saved (not dirty) AND all required products configured. | Opens publish flow (see section 4). |
| Apri su HS | Link style, external-link icon | Visible when `hs_quote_id` is set. | Opens HubSpot quote URL in new tab. |
| PDF | Link style, download icon | Visible when HS quote has PDF URL. | Opens PDF in new tab. |

**Save button states**:
- Clean (no changes): `opacity: 0.5`, `cursor: default`, no hover effects
- Dirty (unsaved changes): Full opacity, normal hover/active effects. Additionally, a small orange dot (6px, `--color-warning`) appears on the top-right of the button as an attention marker.
- Saving: Spinner replaces label, disabled

**Publish button disabled tooltip**: When hovering a disabled Pubblica button, show a tooltip explaining why:
- "Salva le modifiche prima di pubblicare" (if dirty)
- "Configura tutti i prodotti obbligatori prima di pubblicare" (if validation fails, with list of missing products)

### 3.8 Dirty state communication

**Three-layer dirty state system** (all active simultaneously):

1. **Tab dot indicators**: Orange dot on tabs with unsaved changes (lightweight, always visible)
2. **Dirty banner**: Amber banner between tabs and content (medium visibility, in the content flow)
3. **Save button state**: Enabled + orange attention dot (part of the action bar, always visible when scrolled up)

**Leave protection**: Same `beforeunload` handler as the wizard. If the user navigates away with dirty state (clicks back, clicks a tab nav link to another app), the browser shows the native "unsaved changes" dialog.

**Dirty detection**: Compare current form values against the last-saved snapshot. Use a shallow comparison for simple fields and deep comparison for complex objects (services array, contacts). Changing a tab does NOT trigger save -- tabs switch freely, dirty state persists across tab switches.

---

## 4. Publish Flow UX

### 4.1 The publish experience

Publishing is the culmination of the user's work. It takes a draft quote through validation and pushes it to HubSpot. This should feel like a significant, deliberate action.

**Trigger**: Click "Pubblica su HubSpot" button in the detail page action bar.

**Pre-publish confirmation modal** (native `<dialog>`, standard Modal component):
- Title: "Pubblica su HubSpot"
- Body: Summary card showing quote number, customer, NRC/MRC totals
- If legal notes exist: amber info line "Questa proposta richiede approvazione"
- Status badge preview: shows what the status will become (APPROVED or PENDING_APPROVAL)
- Buttons: "Annulla" (secondary) | "Pubblica" (primary)

### 4.2 Step-by-step progress

After confirming, the modal transforms into a **progress view** (do not close and reopen -- morph the content, 300ms transition).

**Progress view layout**: A vertical stepper inside the modal, showing each publish step:

```
  [check]  Salvataggio dati                    Completato
  [check]  Validazione prodotti                Completato
  [spin]   Creazione offerta HubSpot           In corso...
  [--]     Sincronizzazione line items         In attesa
  [--]     Aggiornamento stato                 In attesa
```

Each step is a row (48px height):
- Left: status icon (checkmark for done, spinner for in-progress, circle for pending, X for error)
- Center: step description
- Right: status text

**Icon states and colors**:
- Completed: Green checkmark icon (`--color-success`), text "Completato" in `--color-text-muted`
- In progress: Indigo spinning icon (`--color-accent`), text "In corso..." in `--color-accent`
- Pending: Gray empty circle (`--color-border`), text "In attesa" in `--color-text-faint`
- Error: Red X icon (`--color-danger`), error text in `--color-danger`

**Timing feedback**: Use SSE (Server-Sent Events) for real-time step updates. If SSE is not feasible in phase 1, use polling (every 2 seconds). Each step takes 1-5 seconds depending on HubSpot response time.

**Step labels (Italian)**:
1. "Salvataggio dati"
2. "Validazione prodotti"
3. "Creazione offerta HubSpot" (or "Aggiornamento offerta HubSpot" if updating)
4. "Sincronizzazione prodotti" (with sub-progress: "4/10 line items")
5. "Aggiornamento stato"

### 4.3 Error states and retry

If a step fails:
- The failed step shows the red X icon and error message (e.g., "Errore API HubSpot: timeout")
- Steps after the failed one stay in "pending" state
- The modal bottom shows:
  - Error detail text in a `--color-danger-subtle` box
  - "Riprova" button (primary) -- triggers the entire publish flow again. Since all steps are idempotent, already-completed steps will be skipped quickly.
  - "Chiudi" button (secondary) -- closes the modal, returns to the editor. The quote is in a partially-published state, but the UI should NOT show a broken/inconsistent state. The status badge should show whatever the last known status is.

**Error detail**: Show the step that failed and a human-readable message. Not the raw API error. Example:
- "La sincronizzazione dei prodotti su HubSpot non e' riuscita. I dati della proposta sono salvati correttamente nel sistema."
- "Errore di connessione con HubSpot. Verifica la connessione e riprova."

### 4.4 Success celebration

When all steps complete:
- All steps show green checkmarks
- A brief pause (500ms) for the user to register completion
- The step list fades slightly (`opacity: 0.7`, 300ms) and a success section appears below:

**Success section**:
- Large green checkmark icon (48px, with a drawing animation: the check "draws" itself, SVG stroke-dashoffset animation, 600ms)
- Title: "Pubblicazione completata" (1.25rem/700, `--color-success`)
- Subtitle: Status badge showing the new status (APPROVED or PENDING_APPROVAL)
- Two action buttons:
  - "Apri su HubSpot" (primary) -- external link, opens in new tab
  - "Chiudi" (secondary) -- closes modal, updates the page with new status/HS links

**Confetti? No.** This is a B2B tool for daily use. The checkmark draw animation provides a moment of satisfaction without being frivolous. The success state should feel calm and complete, not celebratory -- this is "job well done," not "you won a prize."

### 4.5 Post-publish state

After closing the success modal:
- The detail page refreshes to show:
  - Updated status badge
  - "Apri su HS" link now active (if it wasn't before)
  - "PDF" link active (after HS generates the PDF, which may take a few seconds -- show a subtle "PDF in generazione..." placeholder with a pulse animation)
  - The Pubblica button label changes to "Ripubblica" (since the quote already exists on HS), and the tooltip explains "Aggiorna l'offerta su HubSpot con le modifiche attuali"

---

## 5. Micro-interactions and Delight

### 5.1 Moments where animation adds value

| Moment | Animation | Duration | Purpose |
|---|---|---|---|
| Page load (list or detail) | `pageEnter` (opacity + translateY) | 500ms | Smooth entry, not a jarring pop |
| Table rows appear | `rowEnter` with stagger (30ms) | 400ms each | Sequential reveal guides the eye down the list |
| Tab switch | Content `sectionEnter` + tab underline slide | 500ms / 400ms | Spatial continuity between tabs |
| Kit accordion expand | max-height + opacity transition | 350ms ease-out | Smooth reveal of nested content |
| Kit accordion collapse | max-height + opacity (reverse) | 250ms ease-out | Slightly faster collapse feels snappier |
| Status badge change | scale pulse (1.0 -> 1.15 -> 1.0) | 400ms ease-spring | Draws attention to the status update after publish |
| NRC/MRC total update | Fade-out old + fade-in new, subtle scale | 300ms | Number change is noticeable but not distracting |
| Dirty banner appear | slideDown (translateY -100% to 0) | 200ms ease-out | Smooth entry from above the content |
| Filter chip remove | Scale to 0 + opacity to 0, then collapse width | 250ms | Satisfying dismissal |
| Save success | Button briefly flashes green (`--color-success` bg) for 600ms, then returns to normal | 600ms | Immediate visual confirmation without a toast for saves |
| Error shake | Horizontal oscillation (2px, 3 cycles) | 300ms | Attention on what went wrong |
| Drag-and-drop lift | `--shadow-float` + `scale(1.02)` + subtle border | Instant (60fps) | Physicality of picking up an item |

### 5.2 Transitions between views

**List to Detail**: Standard browser navigation (React Router). The detail page plays `pageEnter`. No route transition animation -- keep it fast, the page should render in under 200ms with cached data.

**Wizard step transitions**: Each step content area cross-fades:
- Outgoing step: `opacity: 1 to 0`, `translateX: 0 to -20px`, 200ms
- Incoming step: `opacity: 0 to 1`, `translateX: 20px to 0`, 300ms
- Direction reverses when going back (translate to right instead of left)

This gives a sense of moving forward/backward through steps.

### 5.3 Confirmation patterns

**Delete quote** (destructive):
- Modal with danger styling
- Title: "Eliminare la proposta?"
- Body: Quote number + customer name for confirmation
- If HS quote exists: additional warning line "L'offerta verra eliminata anche da HubSpot"
- Buttons: "Annulla" (secondary) | "Elimina definitivamente" (danger button style)
- Confirm button requires 1 second hold (or type quote number for extra safety)? No -- that is overengineering for ~1 delete per week. A single click on a clearly labeled danger button in a modal is sufficient.

**Publish** (significant but not destructive):
- Confirmation modal as described in section 4.1
- Clear summary of what will happen
- No artificial delays or extra steps

**Kit removal from quote** (moderate):
- Inline confirmation: clicking the X on a kit panel first changes the X to a "Conferma?" link in `--color-danger`. Click again within 3 seconds to confirm. After 3 seconds, reverts to X. This avoids a modal for a mid-workflow action.

### 5.4 Keyboard shortcuts for power users

Implement a global keyboard shortcut system. Shortcuts are discoverable via a "?" key press that opens a shortcut cheat sheet modal.

| Shortcut | Action | Context |
|---|---|---|
| `Cmd/Ctrl + S` | Save quote | Detail page (any tab) |
| `Cmd/Ctrl + Enter` | Publish (opens modal) | Detail page (if ready) |
| `Cmd/Ctrl + N` | New quote (open wizard) | Quote list |
| `/` or `Cmd/Ctrl + K` | Focus search field | Quote list |
| `Escape` | Close modal/popover, or navigate back | Global |
| `Tab` / `Shift+Tab` | Move between tabs | Detail page (when tab bar is focused) |
| `1`-`4` | Jump to tab 1-4 | Detail page (when no input focused) |
| `?` | Show shortcut cheat sheet | Global |

**Implementation**: Use a `useHotkeys` hook. Prevent shortcuts when focus is inside a text input or RTE. Show a subtle toast on first use: "Scorciatoia: Cmd+S per salvare" (shown once, then remembered via localStorage).

### 5.5 Toast messages tone and timing

Toasts should be concise, factual, and in Italian. No exclamation marks, no emoji. They confirm what happened, not what the user should feel.

| Event | Type | Message | Duration |
|---|---|---|---|
| Quote saved | success | "Proposta salvata" | 3s |
| Quote created | success | "Proposta {number} creata" | 4s |
| Quote deleted | success | "Proposta eliminata" | 3s |
| Publish success | success | "Pubblicazione completata" | 4s |
| Save error | error | "Errore nel salvataggio. Riprova." | 6s (stays longer) |
| Network error | error | "Connessione non disponibile" | 6s |
| Validation block | error | "Prodotti obbligatori mancanti" | 5s |
| Product updated | (no toast) | -- | -- |

**Product updates do not trigger a toast** -- they happen too frequently (the user may configure 10 products in a row). The visual feedback is the NRC/MRC total animation in the kit header.

**Toast position**: Fixed top-right, 24px from edges (per design system). Stack from top, max 3 visible.

---

## 6. Information Architecture

### 6.1 Page/view hierarchy

```
/quotes                         Quote list (Elenco Proposte)
/quotes/new                     Creation wizard (Nuova Proposta)
/quotes/:id                     Quote detail editor (Dettaglio)
/quotes/:id?tab=intestazione    Detail: Intestazione tab (default)
/quotes/:id?tab=kit-prodotti    Detail: Kit e Prodotti tab
/quotes/:id?tab=note            Detail: Note e Condizioni tab
/quotes/:id?tab=contatti        Detail: Contatti tab
```

**Tab state in URL**: The active tab is stored as a query parameter `?tab=`. This enables deep linking to a specific tab (e.g., sharing a link that goes directly to the kit configuration). If no tab param, default to "intestazione".

### 6.2 Navigation flow

```
Portal (/)
  |
  +-- /quotes (list)
        |
        +-- /quotes/new (wizard) --[create]--> /quotes/:id (detail)
        |
        +-- /quotes/:id (detail) --[back]--> /quotes (list)
```

**Key principles**:
- The list is always the hub. Every flow returns to it.
- The wizard always forwards to the detail page after creation.
- The detail page always has a back path to the list.
- There is no direct path from wizard to list (the user always sees the detail page first after creation, confirming the quote was created).

### 6.3 Breadcrumbs and back-paths

**Breadcrumb**: Not a traditional breadcrumb trail. Instead, use the back-arrow + quote number pattern in the quote header bar:

```
[<-] SP-1102/2026    [BOZZA]                    [Salva] [Pubblica]
```

The back arrow is a 32px icon button. Click navigates to `/quotes`. Hover shows tooltip "Torna all'elenco".

**Browser back**: Works naturally because we use React Router. From detail, back goes to list (preserving filter state from URL params). From wizard, back triggers the `beforeunload` guard if there is entered data.

### 6.4 What the user sees first, second, third

**Morning workflow** (the most common path):

1. **First**: Quote list with "Tutte" filter. The user scans recent quotes, checks statuses. The sort-by-date-descending default means the newest quotes are on top.

2. **Second**: Either opens an existing quote (clicks a row) or creates a new one (clicks "Nuova proposta"). For the editing path, they land on the Intestazione tab by default.

3. **Third**: For the creation path, the wizard guides them through Deal -> Config -> Kit/Products -> Review. For editing, they likely navigate to Kit e Prodotti tab to configure or adjust products.

4. **Fourth**: Save and/or Publish. The publish flow is the terminal action for a quote's lifecycle in this app.

**The entire flow from "open the app" to "published quote" should take 5-10 minutes for a standard quote with 2-3 kits**, assuming the user knows which deal and products they want. The wizard's type selector + conditional fields mean the user never sees irrelevant options, and the accordion product configuration means they never leave the kit context to configure individual products.

### 6.5 External links

| Link | Destination | Icon | Opens in |
|---|---|---|---|
| "Apri su HS" | HubSpot quote URL | External link icon (arrow out of box) | New tab |
| Deal name (in header) | HubSpot deal URL | External link icon | New tab |
| "PDF" | HubSpot PDF URL | Download icon | New tab |

All external links use the same pattern: accent text color, external-link icon (12px), open in new tab. No confirmation before leaving -- these are reference links, not navigation.

---

## 7. Annex: Component Inventory

Components to build or reuse, mapped to design system:

| Component | Source | Notes |
|---|---|---|
| AppShell | `@mrsmith/ui` (existing) | Reuse as-is |
| TabNav | `@mrsmith/ui` (existing) | Extend with dot indicator support |
| Modal | `@mrsmith/ui` (existing) | Reuse as-is |
| Toast/ToastProvider | `@mrsmith/ui` (existing) | Reuse as-is |
| MultiSelect | `@mrsmith/ui` (existing) | Reuse as-is |
| SingleSelect | `@mrsmith/ui` (existing) | Reuse as-is |
| Skeleton | `@mrsmith/ui` (existing) | Reuse as-is |
| ToggleSwitch | `@mrsmith/ui` (existing) | Reuse for trial/flags |
| StatusBadge | App-specific (new) | Pill badge with status color mapping |
| QuoteTable | App-specific (new) | Table with accent bars, sort headers |
| FilterBar | App-specific (new) | Status pills + search + advanced filters |
| Pagination | App-specific (new) | Prev/next with count display |
| Stepper | App-specific (new) | Horizontal step indicator for wizard |
| StepperProgress | App-specific (new) | Vertical step indicator for publish flow |
| KitAccordion | App-specific (new) | Expandable kit panel with product config |
| ProductGroupRadio | App-specific (new) | Radio list of product variants within a group |
| DirtyBanner | App-specific (new) | Amber warning banner for unsaved changes |
| ContactCard | App-specific (new) | Grouped contact reference form |
| RichTextEditor | App-specific (new) | Lightweight RTE (consider Tiptap or Lexical) |
| ConfirmDialog | App-specific (new) | Themed confirmation modal wrapper |
| KeyboardShortcuts | App-specific (new) | Global shortcut handler + cheat sheet |
| DealCard | App-specific (new) | Deal selector card for wizard step 1 |
| TypeSelector | App-specific (new) | Standard/IaaS toggle cards for wizard step 2 |

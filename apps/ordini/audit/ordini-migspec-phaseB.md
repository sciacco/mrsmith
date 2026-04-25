# Ordini — Migration Spec · Phase B: UX Pattern Map

Source: `app-inventory.md`, `page-audit.md`, `findings-summary.md`.
Scope: **port 1:1, ignore dead features**. Two views carry over — `Home` and `Dettaglio ordine` — plus the `ModificaRiga` modal that belongs to Dettaglio. The Arxivar deep-link modal (`Modal1` on Dettaglio) is absorbed into the Arxivar tab.

---

## View 1 — Home (Lista ordini)

**User intent.** Find and open a specific order. The user is typically a CustomerRelations operator looking up a known order by proposta number or customer name.

**Interaction pattern.** List / index view. One table, one row action ("Visualizza"), no detail-preview, no bulk actions.

**Widget sections (1:1).**

| # | Section | Widgets | Role |
|---|---|---|---|
| 1 | Page header | `Titolo` (TEXT) | Static heading "Lista ordini". |
| 2 | Orders table | `Lista_ordini` (TABLE_V2) bound to `Select_Orders_Table.data` | 17 data columns + 1 row-action iconButton ("Visualizza"). Client-side search/sort/pagination (no server-side pagination despite the orphan queries — see §Migration notes). |

**Columns in 1:1 order (from `Select_Orders_Table`).**
1. System ODV (`cdlan_systemodv`)
2. Tipo di documento (mapped: `TSC-ORDINE-RIC` → "Ordine ricorrente", else "Ordine Spot")
3. Codice ordine (`cdlan_ndoc/cdlan_anno`)
4. Numero proposta (`cdlan_ndoc`)
5. Anno documento (`cdlan_anno`)
6. Sostituisce ordini (Num/Anno) (`cdlan_sost_ord`)
7. Ragione sociale (`cdlan_cliente`)
8. Data proposta (`cdlan_datadoc`)
9. Tipo di servizi (`is_colo ≠ 0 ? is_colo : service_type`)
10. Tipo di proposta (`A`→Sostituzione, `N`→Nuovo, `R`→Rinnovo)
11. Data conferma (`cdlan_dataconferma`)
12. Stato (`cdlan_stato`)
13. Lingua (`profile_lang`)
14. Evaso (`cdlan_evaso`) — rendered "Sì" when `== 1`, "No" when `== 0` (B2 resolution).
15. Dal CP? (`from_cp ≠ 0 ? "Sì" : "No"`)

**Primary actions.**
- Row "Visualizza" → navigates to `/ordini/:id` (Dettaglio page).

**Entry / exit points.**
- **Enter:** from portal sidebar or direct URL `/ordini`.
- **Exit:** row click opens Dettaglio; `SendToErp.run` success on Dettaglio brings the user back here via `navigateTo('Home')`.

**Current vs intended.**
- The table is **client-side** today (`totalRecordsCount: 0`, no server pagination wiring). 1:1 port preserves client-side. The orphan `Select_orders1` / `Total_record_orders1` queries are dropped per `docs/TODO.md` guidance when/if server-side pagination is needed later.
- The display mappings (Tipo documento, Tipo proposta, Dal CP) move from SQL CASE to a **frontend formatter**. Backend returns raw codes; the React app maps to labels. This keeps the API reusable and the SQL minimal.
- Row click's legacy `.then(() => Dettaglio_ordine_vero.run(...))` call is dropped — the Dettaglio page reloads its own data via URL param.

**Dropped widgets.**
- `Nuovo_Ordine` icon button (hidden + disabled today; order creation not in scope).
- `Dettaglio_ordine` modal and everything inside it (legacy full-detail modal; superseded by the dedicated page).

---

## View 2 — Dettaglio ordine (`/ordini/:id`)

**User intent.** Drive the order through its lifecycle (BOZZA → INVIATO → ATTIVO), update header/referents/row data, and pull PDFs. One screen, one order.

**Interaction pattern.** Detail view with a tabbed workspace. Top bar carries the PDF actions; body is a five-tab container (Arxivar tab dropped per B3, link moved to Info); one modal attaches to the view (`ModificaRiga` for per-row activation date).

**Top-level layout (1:1).**

```
┌──────────────────────────────────────────────────────┐
│ [← Torna]  Codice ordine: <ndoc>/<anno>              │
│                   [Visualizza odv arx] [Scarica PDF] │
│                   [Download kickoff]   [Genera MA]   │
├──────────────────────────────────────────────────────┤
│  Tabs: Info | Azienda | Referenti | Righe |          │
│        Informazioni dai tecnici                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  <tab content>                                 │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

**Widget sections.**

### 2.1 — Header bar

| Widget | Purpose | Enabled-when |
|---|---|---|
| `TornaIndietro` | Back to Home | always |
| `Titolo` | `"Codice ordine: {ndoc}/{anno}"` | — |
| `Visualizza_odv_arx` (PDF) | Fetch signed PDF via `GW_GetPDFArxivarOrder` | `arx_doc_number IS NOT NULL` |
| `Scarica_PDF_button` (PDF) | Fetch raw PDF via `DownloadOrderPDFintGW` | `arx_doc_number IS NULL` |
| `Download_kickoff` (PDF) | Kickoff PDF via `GW_Kickoff` | `stato == 'INVIATO'` + `app_customer_relations` |
| `Genera_MA` (PDF) | Activation Form PDF via `GW_ActivationForm`, filename localized by `profile_lang` | `stato ∈ {'INVIATO','ATTIVO'}` + `app_customer_relations` |

### 2.2 — Tab "Info"
Primary workspace for header-level edits and state transitions.

| Sub-section | Widgets | Notes |
|---|---|---|
| Readonly header metadata | ~20 TEXT widgets showing raw fields from `Order` | e.g. Proposta, System ODV, Tipo doc, Tipo proposta (A/N/R → label), Dur. rinnovo/tacito rinnovo (1/0 → Sì/No), Intervallo fatturazione (CASE → label), etc. All presentation-layer mappings move to the frontend formatter. |
| Editable block (`Container1`) | `cdlan_rif_ordcli` INPUT, `cdlan_dataconferma` DATEPICKER, `erp_an_cli` SINGLE_SELECT_TREE (Ragione sociale), SALVA button (`Button3`) | Editable only when `stato == 'BOZZA'` + `app_customer_relations`. Click → `PATCH /api/ordini/:id` with the three fields, then refetch. |
| Action bar (`cont_bottoni`) | `RICHIEDI ANNULLAMENTO`, `INVIA in ERP` (`invia`), `ORDINE PERSO` (dropped — hidden today), Arxivar file picker (`arxivar`) | See §Action bar rules below. |

**Action bar rules (Q6 applied).**
- **RICHIEDI ANNULLAMENTO** → **NOT RENDERED in v1** (deferred per Phase A Q1 resolution + `docs/TODO.md`).
- **INVIA in ERP** (`invia`): enabled when all of — `stato == 'BOZZA'`, `app_customer_relations`, `cdlan_dataconferma` set, `erp_an_cli` selected, Arxivar file attached.
- **Arxivar file picker** (single file, PDF): enabled when `stato NOT IN {'ANNULLATO','PERSO','ATTIVO'}` AND `app_customer_relations`. **This is the Q6 fix — the original buggy OR-chain is discarded.**
- **ORDINE PERSO** (`butt_perso`): dropped (hidden today, no valid transition from this app).

**Dettaglio-specific rule to flag in Phase B:** the `SALVA` on Info tab is **only** for (rif_ordcli, dataconferma, ragione sociale). Other readonly fields remain immutable from this page.

**Q9 placement (B1 resolution).** `cdlan_cliente_id` is rendered on the Info tab as a small secondary line under "Ragione sociale" in the readonly header metadata (format: `ID cliente: <cdlan_cliente_id>`).

**Arxivar deep-link (B3 resolution).** The standalone "Arxivar link" tab is dropped. The deep-link anchor moves to the Info tab, rendered only when `arx_doc_number IS NOT NULL`, positioned near the top of the tab content (adjacent to the readonly header metadata). Target URL shape remains `https://arxivar.cdlan.it/#!/view/<arx_doc_number>/…` as in the source.

### 2.3 — Tab "Azienda"
Readonly company/profile block.

Widgets: TEXT fields for `profile_iva`, `profile_cf`, `profile_address`, `profile_city`, `profile_cap`, `profile_pv`, `profile_sdi`, plus a **hidden** `profile_lang` INPUT used by `GetPdf.activationForm` to pick IT/EN filename.

**1:1 port:** preserve as a standalone tab. The hidden `profile_lang` disappears from the UI — in the rewrite, the backend derives the PDF filename directly from the order's `profile_lang`, so the frontend doesn't carry the hidden widget.

### 2.4 — Tab "Referenti"
Editable customer contacts.

Widgets: three 3-field groups (Tecnico / Altro tecnico / Amministrativo — `nom`, `tel`, `email` each; note the asymmetric `cdlan_rif_adm_tech_*` column names on the ADM group), SALVA button (`Button6`).

**Rule:** SALVA enabled when `stato ∈ {'BOZZA','INVIATO'}` AND `app_customer_relations`. Click → `PATCH /api/ordini/:id/referents`, refetch.

### 2.5 — Tab "Righe"
Order lines table; inline editing for `Numero seriale` + per-row action to open activation-date modal.

| Widget | Notes |
|---|---|
| `Lista_righe` (TABLE_V2) bound to `RigheOrdine.data` | 12 columns: ID Riga, System ODV Riga, Codice articolo bundle, Codice articolo, Descrizione articolo, Canone, **Attivazione** (standardized DTO field `activation_price`, per Q4), Quantità, Prezzo cessazione, Codice raggruppamento fatturazione, Data attivazione (formatted DD/MM/YYYY or "-"), **Numero seriale** (editable inline when `stato == 'BOZZA'`). |
| `customColumn1` (row iconButton "Modifica") | Visible only when `stato == 'INVIATO'` + `app_customer_relations`. Click → opens `ModificaRiga` modal (see §2.7). |
| Inline editActions (EditActions1) | `onSave` → `PATCH /api/ordini/:id/rows/:rowId/serial-number`, then refetch. |

### 2.6 — Tab "Informazioni dai tecnici"
Technical-notes editor for each row.

| Widget | Notes |
|---|---|
| `Lista_righe_tecnici` (TABLE_V2) bound to `RigheOrdineTecnici.data` | 5 columns: ID riga, codice articolo bundle, codice articolo, **note tecnici** (editable inline; UTF8-converted on read), data annullamento (readonly). |
| Inline editActions | `onSave` → `PATCH /api/ordini/:id/rows/:rowId/technical-notes`, then refetch. |

**Role gate.** Not gated on `app_customer_relations` — anyone with `app_ordini_access` edits technical notes. Parity with today.

### 2.7 — Modal "ModificaRiga" (per-row activation date)

Attached to the Righe tab, opened from `customColumn1.onClick`.

Widgets: `cdlan_data_attivazione` DATEPICKER (required), `cdlan_serialnumber` INPUT (readonly here — the serial is edited inline on the Righe row), CONFERMA button (`BTN_confirm_act_modal`), CHIUDI button.

**Rule:** CONFERMA enabled when `cdlan_data_attivazione` is set. On confirm → `PATCH /api/ordini/:id/rows/:rowId/activate` with `{activation_date}`; backend writes `orders_rows.cdlan_data_attivazione`, sets `confirm_data_attivazione = 1`, calls `GW_SetActivationDate`, re-counts confirmed rows (`CheckConfirmRows` with the Q2 fix — `IS NOT NULL`), and if every row is now confirmed, flips `orders.cdlan_stato` to `ATTIVO` in the same transaction. Returns the updated order; UI refetches and closes modal.

### 2.8 — (was: "Arxivar link" tab) — **dropped per B3**

The standalone tab is removed. The Arxivar deep-link anchor is rendered on the Info tab (see §2.2 Q9/B3 resolution), only when `arx_doc_number IS NOT NULL`.

---

## Dropped views (not ported)

Confirmed by the scope directive; listed for the record.

- `Ordini semplificati` (HubSpot potentials scaffold).
- `Draft gp da offerta` (empty canvas).
- `Form ordine` (unfinished create-order form — order creation lives in Customer Portal / ERP).
- Legacy `Dettaglio_ordine` modal on Home.
- Hidden elements: `Nuovo_Ordine` icon, `butt_perso` (ORDINE PERSO), `SendRequestAnnullaOdv` JSObject and its dead REST action.

---

## Cross-view user journeys

1. **Open an order.** Home → click "Visualizza" on a row → Dettaglio (`/ordini/:id`) opens on the Info tab.
2. **Finalize BOZZA and send to ERP.** Dettaglio Info → edit `rif_ordcli`, `data_conferma`, pick `Ragione sociale` → SALVA → attach Arxivar PDF → **INVIA in ERP** → backend runs the transactional all-or-nothing send (Q8 resolution), state → INVIATO, PDF uploaded → success confirmation → back to Home.
3. **Confirm rows and auto-promote to ATTIVO.** Dettaglio Righe → per-row "Modifica" → set activation date → CONFERMA → backend writes row + sends to ERP + recounts; on the last confirmed row, state → ATTIVO automatically. No explicit "mark active" action exists.
4. **Edit referents.** Dettaglio Referenti → edit fields → SALVA. Enabled in BOZZA or INVIATO.
5. **Edit per-row serial (BOZZA).** Righe → inline edit Numero seriale on a row → save.
6. **Edit technical notes (any user, any state).** Informazioni dai tecnici → inline edit → save.
7. **Pull a PDF.** Header bar: Kickoff (INVIATO) / Activation Form (INVIATO or ATTIVO) / raw PDF (pre-Arxivar) / signed PDF (post-Arxivar).

Cancel-request journey is **removed from v1**.

---

## Phase B open questions

Minimal — 1:1 keeps almost every UX decision frozen. Only three points need expert input:

### B1. `cdlan_cliente_id` placement
Info tab (under Ragione sociale) or Azienda tab (with the rest of the profile fields)? Proposal: Info tab, secondary line. **Confirm or redirect.**

### B2. `cdlan_evaso` column in the Home table
`Select_Orders_Table` returns it raw; the Appsmith table renders it with no mapping (just `0/1`). Options: (a) hide it in the UI but keep in the API for parity, (b) render as "Sì/No", (c) drop it entirely from the table. Proposal: **(a) hide in UI, keep in payload** — matches the current behaviour closest and leaves the door open to use it in later filtering.

### B3. Arxivar tab retention
The tab is a single anchor. Under 1:1 it stays. **Confirm** we don't collapse it into a header link / inline section on the Info tab — the 1:1 directive says keep, but worth a sanity check because the information value is very low.

### B4. (Reminder) Q7 `from_cp` rule
Already rolled into the post-v1 cancel-order TODO. Phase B doesn't re-open it.

---

## Phase B exit criteria

**Phase B complete.** B1 → Info tab; B2 → render "Sì"/"No"; B3 → drop the Arxivar tab and move the anchor to Info. Ready for Phase C (Logic Placement).

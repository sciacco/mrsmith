# Appsmith Audit — Richieste Fattibilità

Source: `apps/richieste-fattibilita/richieste-fattibilita.json` (Appsmith export).
Purpose: reverse-engineer current behaviour to feed a migration spec. Do **not** treat bindings as React source.

---

## 1. Application inventory

- **App name:** Richieste Fattibilità
- **Source type:** Appsmith application export JSON
- **Pages (6):**
  - `Home` — landing / placeholder (single text widget).
  - `Nuova RDF` — creation form for a new feasibility request (RDF) against a HubSpot deal.
  - `Gestione RDF Carrier` — carrier-side management list (stato filter + list, navigates to detail).
  - `Dettaglio RDF Carrier` — carrier-side detail/editing of one RDF: header, list of per-supplier feasibilities, modal to add, form to edit.
  - `Consultazione RDF` — read-only listing for requestors, aggregates feasibility counts per stato, client filter.
  - `Visualizza RDF` — read-only detail with AI analysis tab and PDF export tab.
- **Datasources (3):**
  - `anisetta` (Postgres) — owner of the app's own schema (`rdf_richieste`, `rdf_fattibilita_fornitori`, `rdf_fornitori`, `rdf_tecnologie`). Read + write.
  - `db-mistra` (Postgres) — read-only on `loader.hubs_*` (HubSpot replica: deals, companies, pipelines, stages, owners).
  - `openrouter` (REST API) — LLM completion endpoint (`ai_openrouter` POST, body forwards `this.params.request`).
- **JSObjects:** `utils` (per-page, different contents) and `_$jsGChat1$_jsGChat` (shared Google Chat / Teams webhook helper imported on 2 pages; body not in export, assumed external library module).
- **Actions:** 46 total (SQL queries + JS functions + REST). Many named collisions across pages (each page has its own `get_deals`, `get_fornitori`, etc.).
- **Global app state:** `appsmith.store.v_id_richiesta` (editable detail), `appsmith.store.v_id_richiesta_ro` (readonly detail). Set via `storeValue` before `navigateTo`.
- **Auth/role context:** `appsmith.user.email` used as `created_by`; `appsmith.user.roles` inspected for manager gate (`'straFatti Full'`, `'Administrator - Sambuca'`).

### Global findings / risks
- **Direct DB access from UI.** All reads and writes happen via inline SQL bound to widget state — classic Appsmith anti-pattern for migration; every SQL must be re-implemented as a backend endpoint.
- **SQL injection via string templating.** Filter queries interpolate widget text directly: `ILIKE '{{"%" + i_deal.text + "%"}}'` and `aggiornaDati` concatenates `i_cliente.text` into a raw `AND c.name ILIKE '%${i_cliente.text}%'` fragment. Must be parameterized in migration.
- **Secrets in JSObject.** `utils.chatWebhook` on `Nuova RDF` hard-codes a Power Automate/Teams webhook URL (production signature in plaintext). Must move to backend config.
- **Hard-coded HubSpot pipeline IDs** (`255768766`, `255768768`) and stage `display_order` ranges scattered across multiple `get_deals` queries. Business rule, duplicated 3×.
- **Manager role gate on client.** `utils.IsManager()` whitelists `['straFatti Full', 'Administrator - Sambuca']` purely in UI — any backend endpoint must re-enforce.
- **Duplicated queries.** `get_deals`, `get_fornitori`, `get_tecnologie`, `get_fatt_fornitore`, `get_richiesta_full_by_id` are redefined per page. Single source of truth needed in backend.
- **Widget-coupled business logic.** Updates read from `tbl_fattib_forn.selectedRow`, `ms_fornitori.selectedOptionValues`, etc. — the data layer is implicit in the UI. Migration must define explicit DTOs.
- **Hard-coded test defaults in SQL.** `get_richiesta_by_id` uses `appsmith.store.v_id_richiesta || 0`, `get_fatt_fornitore` uses `|| 3` — fallback id `3` leaks test state into prod queries.
- **`get_richiesta_full_by_id` on `Consultazione RDF` has `where rr.id = 3` hard-coded** (stale/dev code; only the `Visualizza RDF` version parameterizes correctly).
- **OpenRouter model name mismatch.** `analisi` uses `gemini-2.5-flash-lite-preview-09-2025`, `analisi_json` uses `...-06-17`. Model pinning is inconsistent.
- **PDF generation runs client-side via jspdf** (`generate2`) reading from `ai_openrouter.data` directly — tight coupling to query name.

---

## 2. Datasource & query catalog

### 2.1 `anisetta` (Postgres, app-owned schema)

| Query | Pages | Purpose | Inputs | Read/Write | Rewrite target |
|---|---|---|---|---|---|
| `get_fornitori` | Nuova RDF, Dettaglio, Visualizza | List suppliers | — | R | `GET /rdf/fornitori` |
| `get_tecnologie` | Dettaglio, Visualizza | List technologies | — | R | `GET /rdf/tecnologie` |
| `get_richiesta_by_id` | Nuova RDF, Dettaglio | Single request | `id` (param; fallback 0) | R | `GET /rdf/richieste/:id` |
| `get_richiesta_full_by_id` | Consultazione, Visualizza | Request joined with all supplier feasibilities | `v_id_richiesta_ro` (from store) | R | `GET /rdf/richieste/:id/full` — **fix Consultazione variant (hard-coded id=3, unused?)** |
| `get_richieste` (Gestione) | Gestione RDF Carrier | Flat list with text filters | `ms_stato`, `i_deal`, `i_richiedente` | R | `GET /rdf/richieste?stato&deal&richiedente` |
| `get_richieste` (Consultazione) | Consultazione RDF | List with aggregated feasibility counts per stato | same filters | R | `GET /rdf/richieste/summary` returning counts (`draft/inviata/sollecitata/completata/annullata`, `da_ordinare_count`, min/max dates) |
| `get_fatt_fornitore` | Dettaglio, Visualizza | Feasibility rows per request, joined with fornitore/tecnologia | `v_id_richiesta` | R | `GET /rdf/richieste/:id/fattibilita` |
| `get_fatt_for_by_id` | Dettaglio, Visualizza | Single feasibility row | `tbl_fattib_forn.selectedRow.id` | R | `GET /rdf/fattibilita/:id` (Visualizza copy hard-codes `id = 0`, looks unused) |
| `ins_richiesta` | Nuova RDF | Insert new request; returns id | widgets: `Table1.selectedRow`, `i_dettagli`, `i_indirizzo`, `ms_fornitori`, `appsmith.user.email` | W | `POST /rdf/richieste` |
| `ins_fatt_fornitori` | Dettaglio | Insert per-supplier feasibility | `{richiesta_id, fornitore_id, tecnologia_id}` | W | `POST /rdf/richieste/:id/fattibilita` (batch) |
| `upd_fatt_fornitori` | Dettaglio | Update feasibility — all editable fields | 17 named params | W | `PATCH /rdf/fattibilita/:id` |
| `upd_stato_richiesta` | Dettaglio | Change request stato | `id`, `stato` | W | `PATCH /rdf/richieste/:id/stato` |

### 2.2 `db-mistra` (Postgres — HubSpot replica, read-only)

| Query | Pages | Purpose | Filter | Rewrite target |
|---|---|---|---|---|
| `get_deals` (Nuova / Gestione) | Nuova RDF, Gestione | Deal picker | **Hard-coded** pipelines `255768766` (stages 1–5) and `255768768` (stages 3–8), `codice <> ''`, LIMIT 300 | `GET /rdf/deals` (server owns the pipeline/stage rules) |
| `get_deals` (Consultazione) | Consultazione RDF | Parametric filter — appends `paramsDeals.query` (either client filter OR pipeline rule) | `this.params.query` (raw SQL fragment!) | Same endpoint with proper `?cliente=` param; never pass raw SQL |
| `get_deal_by_id` | Dettaglio, Visualizza | Single deal by `d.id` | `get_richiesta_by_id.data[0].deal_id` / `get_richiesta_full_by_id.data[0].deal_id` | `GET /rdf/deals/:id` |

### 2.3 `openrouter` (REST)

| Query | Pages | Purpose | Inputs | Rewrite target |
|---|---|---|---|---|
| `ai_openrouter` | Visualizza RDF | Generic passthrough to OpenRouter chat-completions | `this.params.request` (full chat body) | Backend LLM proxy — never expose API key or raw request shape to client |

Orchestrators on `Visualizza RDF`:
- `analisi` — builds a human-readable analysis (model: `google/gemini-2.5-flash-lite-preview-09-2025`, `temperature: 0`, `max_tokens: 4096`), system prompt from `utils.system_prompt` (not visible in export body printed — assumed in JSObject remainder), user prompt = "Esprimi la tua valutazione…" + `JSON.stringify(get_richiesta_full_by_id.data)`.
- `analisi_json` — structured JSON output (`response_format: json_object`) with `utils.system_prompt3` — drives the "azioni_raccomandate" Table1 widget.

---

## 3. Per-page audits

### 3.1 Home

- **Purpose:** static landing; a single text widget, no data.
- **Migration note:** drop or replace with portal landing.

### 3.2 Nuova RDF

- **Purpose:** operator picks a HubSpot deal and opens a new feasibility request.
- **Widgets & roles:**
  - `Table1` — deal picker bound to `get_deals.data`. Selection drives `ins_richiesta` (`deal_id`, `codice_deal`).
  - `FORM_WIDGET Form1` wrapping: `i_dettagli` (free text — request details), `i_indirizzo` (address/coords), `ms_fornitori` (preferred suppliers, `sourceData={{get_fornitori.data}}`, labelled "opzionale"), `btn_save`, `Button2` (Reset).
- **onLoad actions:** `get_deals`, `get_fornitori`.
- **Event flow (save):**
  1. `btn_save.onClick` = `utils.nuovaRDF(); utils.notificaChat();` — **but** `utils.nuovaRDF` already calls `notificaChat` internally AND navigates, so second call runs after navigation. Possible latent bug.
  2. `utils.nuovaRDF` → `ins_richiesta.run()` → `utils.notificaChat()` → `navigateTo('Consultazione RDF')`. Dead line `ins_richiesta.data;` after `return;` (dead code).
  3. `notificaChat` re-fetches the inserted request (`get_richiesta_by_id` with returned id, fallback id `9`) and posts a Teams Adaptive Card via `jsGChat1.sendCardMessage(utils.chatWebhook, card)` with link `appsmith.URL.hostname`.
- **Hidden logic / bindings:**
  - `ins_richiesta` reads widget state directly (`Table1.selectedRow.id`, `i_dettagli.text`, `i_indirizzo.text`, `ms_fornitori.selectedOptionValues`, `appsmith.user.email`, `Table1.selectedRow.codice`) — an implicit DTO.
  - `fornitori_preferiti` is stored as Postgres array literal (`{1,2,3}`) — later parsed back via `utils.stringaArray`.
  - `appsmith.URL.hostname` is inlined into the Teams card — env-dependent.
- **Classifications:**
  - Business: preferred-supplier list schema, `created_by = user email`, notify on create.
  - Orchestration: order of insert → notify → navigate.
  - Presentation: deal table columns, form labels.
- **Open questions:** why is `notificaChat` called twice? Is the `id ?? 9` fallback a test artifact (suspected)?

### 3.3 Gestione RDF Carrier

- **Purpose:** carrier team list view over `rdf_richieste`, filtered by stato + codice deal + requestor email.
- **Widgets:**
  - Filter bar: `ms_stato` (default `["nuova","in corso"]`, four options), `i_deal`, `i_richiedente`, `Button1` "Aggiorna" → `get_richieste.run()`.
  - `LIST_WIDGET_V2 List1` bound to `get_richieste.data`; each card shows deal code, id+date+stato, address, description; `Button2 "Gestisci"` → `storeValue('v_id_richiesta', currentItem.id)` then `navigateTo('Dettaglio RDF Carrier')`.
- **onLoad:** `get_richieste`.
- **Hidden logic:** stato list hard-coded as inline options; SQL interpolates widget values (injection risk).
- **Classifications:** business = stato taxonomy; orchestration = filter binding; presentation = card layout.

### 3.4 Dettaglio RDF Carrier

- **Purpose:** carrier agent edits a feasibility request and its per-supplier feasibility rows.
- **Structure:**
  - Top `tbl_fattib_forn` — table of `rdf_fattibilita_fornitori` joined with fornitore/tecnologia.
  - `Tabs1 / Riepilogo RDF` tab: read-only header of request (`indirizzo`, `descrizione`, created_at/created_by, preferred suppliers summary via `utils.fornitoriPreferiti`), `slide_stato` (category slider: nuova/in corso/completata/annullata, `onChange` → `upd_stato_richiesta` then `showAlert`), Deal summary (`Text5`), `Input2` "Note Carrier Relations" (no visible save path → **dead input**).
  - `Form1` — editor for selected feasibility row; fields populated from `tbl_fattib_forn.selectedRow.*`: supplier, tecnologia, profilo, descrizione, contatto/riferimento, stato, NRC, MRC, durata_mesi, esito_ricevuto_il, ck_da_ordinare, annotazioni, r_aderenza_budget, rg_copertura (Si=1/No=0), i_giorni_rilascio.
    - `Button1 "Aggiorna"` → `utils.aggiornaRecordFattForn()`; disabled when no selection.
    - `Button2 "Reset"` — no bound action.
  - `MODAL mdl_new_ff` — add new supplier feasibility: `ms_fornitori` (multi) + `sl_tecnologia` (single) → `utils.creaRecordFattForn()` loops `ins_fatt_fornitori` per supplier, refreshes table, closes modal; also re-runs `get_richiesta_by_id` when current `slide_stato.value === 'nuova'` (likely meant to trigger a stato transition side-effect on server — actually no-op here since `get_richiesta_by_id` is SELECT only; suspected bug / misplaced intent).
- **onLoad:** `get_fatt_fornitore`, `get_fornitori`, `get_tecnologie`, `get_richiesta_by_id`, `get_deal_by_id`.
- **Event flow on save feasibility:**
  1. `aggiornaRecordFattForn` assembles 17-field params from widgets → `upd_fatt_fornitori.run`.
  2. `utils.NotificaChat()` (sync) — compares new vs current values for stato/copertura/nrc/mrc; if changed, builds text message and posts via `jsGChat1.sendTextMessage(utils.chatWebhook, testo)`. Reads `get_deal_by_id.data[0]` for deal codice/name.
  3. `get_fatt_fornitore.run()` re-fetches rows.
  4. `showAlert('Dati aggiornati','success')`.
- **Hidden logic:**
  - `fornitoriPreferiti` parses `rdf_richieste.fornitori_preferiti` Postgres array literal via regex (`stringaArray`).
  - `rg_copertura` value is a stringified `"1"`/`"0"`; SQL update passes directly to `copertura` column.
  - `slide_stato.onChange` immediately persists status to DB — no confirm.
  - `IsManager`-style gating not applied here; only ACL is "user can reach this page".
  - Tab2 of `Tabs1` is `isVisible: false` — dead tab.
- **Classifications:**
  - Business: stato transitions, notify-on-change rules (which fields trigger chat), copertura semantics.
  - Orchestration: update → notify → refetch → toast.
  - Presentation: tabs, modal, form layout, disabled state.
- **Candidate entities:** `Richiesta` (aggregate root), `FattibilitaFornitore` (child), `Fornitore`, `Tecnologia`.
- **Open questions:** intent of `get_richiesta_by_id.run()` inside `creaRecordFattForn` when status is 'nuova'; `Input2` Note Carrier Relations unused; `Button2 Reset` unbound.

### 3.5 Consultazione RDF

- **Purpose:** requestor-facing listing with feasibility-progress counters and optional client-name lookup across all HubSpot deals.
- **Widgets:**
  - Filter bar identical to Gestione + extra `i_cliente` (company name filter).
  - `List1` bound to `utils.Richieste` (mutable JS state, not a query).
  - Card shows deal code, `richiesta_id`+date+stato, company/address, description, stato counters (`utils.stato_ff(currentItem)` → "Bozza: X Inv: Y …"), `Button2 "Visualizza"` → `storeValue('v_id_richiesta_ro', currentItem.richiesta_id)` + `navigateTo('Visualizza RDF')`.
  - `IconButton1` (Gestisci shortcut) — `isVisible: {{utils.IsManager()}}`; routes to editable detail with `v_id_richiesta`.
- **onLoad:** none; `utils.aggiornaDati` must be triggered via Button1 "Aggiorna".
- **Event flow (Aggiorna):**
  1. Build `paramsDeals.query` — if `i_cliente.text`, fragment `and c.name ILIKE '%${value}%'`; else the pipeline/stage constraint.
  2. `get_richieste.run()` (aggregated counts) + `get_deals.run(paramsDeals)`.
  3. `utils.mergeDati(richieste, deals)` — left-join in JS on `codice_deal === deal.codice`; if no match: include with blank company/deal *unless* user is filtering by client (then drop).
  4. Assign to `utils.Richieste` (shared mutable) → list re-renders.
- **Classifications:**
  - Business: stato-counter taxonomy, manager-only shortcut, "when client filter is set, hide RDFs without matching deal".
  - Orchestration: parallel fetch + client-side join.
  - Presentation: card layout, HTML in text widget.
- **Risks:** SQL injection via `paramsDeals.query` template string; unused `get_richiesta_full_by_id` query on this page has `where rr.id = 3` hard-coded (dead code).

### 3.6 Visualizza RDF

- **Purpose:** read-only view of one RDF with LLM analysis and PDF export.
- **Widgets / tabs:**
  - `Tabs1`: `Riepilogo`, `Analisi`, `PDF`, plus `tab7o02ffkvvr` (label truncated — likely "Azioni").
  - Riepilogo: `Text1` (header: id/stato/requestor/address/description/deal), `tblFF` bound to `get_richiesta_full_by_id.data` (supplier feasibility rows), `Container1` with 10 inputs + 3 texts reading from `tblFF.selectedRow.*`, `Rating1` (no binding — probably `aderenza_budget` but unset here).
  - Analisi: `Text5` = `ai_openrouter.data.choices[0].message.content`.
  - PDF: `DocumentViewer1` — no binding visible in extract but driven by `utils.generate2()` which returns a `datauristring`; likely bound via a property not in our key list (probably `docUrl`).
  - Azioni: `Text6` + `Table1` bound to `utils.analisi_ai_json.azioni_raccomandate` (structured JSON from `analisi_json`).
- **onLoad:** `ai_openrouter`, `get_richiesta_full_by_id`, `get_deal_by_id`.
  - **Risk:** `ai_openrouter` runs on page load with *whatever* `this.params.request` defaults to (likely empty/last) — normally `analisi`/`analisi_json` orchestrators are what should run; direct onLoad of `ai_openrouter` is suspect.
- **Event flow (analysis):** `analisi` → builds chat body with model/system/user prompts → `ai_openrouter.run({request})` → stashes on `utils.analisi_ai`. `analisi_json` parallel path produces structured output. `generate2` creates jsPDF document in-memory.
- **Hidden logic:**
  - `utils.score_budget` = `["Pessima","Fuori budget","Nella norma","Ottima","Eccezionale"]` — index from `aderenza_budget - 1`.
  - `Input2Copy1Copy` shows `copertura ? 'SI' : ''` — presentational mapping.
  - PDF layout (colors, margins, sections "Richiesta Utente" / "Analisi" / "Richieste di Fattibilità ai Fornitori") embedded in JS.
- **Classifications:**
  - Business: LLM prompt content & model choice, PDF content structure, budget-score labels.
  - Orchestration: page-load chain, parallel text vs json analysis.
  - Presentation: tabs, rating rendering, PDF styling.

---

## 4. Findings summary

### 4.1 Embedded business rules (extract to backend / domain)
- HubSpot pipeline/stage whitelist (`255768766` stages 1–5; `255768768` stages 3–8) defines "deals eligible for RDF".
- Feasibility stato taxonomy: `bozza`, `inviata`, `sollecitata`, `completata`, `annullata`.
- Request stato taxonomy: `nuova`, `in corso`, `completata`, `annullata`.
- Notification rule: Teams chat message fires on create **and** on change of `stato | copertura | nrc | mrc` (not on other fields).
- Manager role = `straFatti Full` OR `Administrator - Sambuca` (controls editable shortcut on Consultazione).
- Preferred suppliers stored as Postgres array literal on `rdf_richieste.fornitori_preferiti`.
- Budget score mapping (1–5 → label).
- "Client filter hides unmatched RDFs" rule in `mergeDati`.

### 4.2 Duplication
- `get_deals` defined 3× with drift (Consultazione version is parametric; others pin pipelines).
- `get_fornitori`, `get_tecnologie`, `get_fatt_fornitore`, `get_richiesta_full_by_id` duplicated per page.
- `stringaArray`, `fornitoriPreferiti`, `creaRecordFattForn`, `aggiornaRecordFattForn`, `NotificaChat`, `stato_ff`, `IsManager`, `mergeDati`, `aggiornaDati`, `generate2`, `formatDate`, `analisi`, `analisi_json` — some split between a standalone action and an identical method in a page's `utils` JSObject.

### 4.3 Security concerns
- **Injection:** every list-page SQL interpolates widget text. Must be parameterized.
- **Raw SQL fragment as API param** (`Consultazione.get_deals` via `this.params.query`) — don't keep this shape.
- **Secrets in client bundle:** Teams webhook URL hard-coded in `utils`; OpenRouter API key presumably in datasource config (not exposed in export but accessible to any Appsmith editor).
- **Client-side role gate only** — backend must re-check manager role for any admin action.
- **AI prompt/model choices in client** — move to backend to avoid prompt injection / abuse.

### 4.4 Fragile bindings / hidden dependencies
- Widgets reference each other by name (`tbl_fattib_forn.selectedRow`, `ms_fornitori.selectedOptionValues`) — any rename breaks queries silently.
- `get_richiesta_by_id.run()` inside `creaRecordFattForn` appears intentionless (SELECT not mutation).
- `get_richiesta_by_id` / `get_fatt_fornitore` use SQL fallbacks `|| 0` / `|| 3` that can return rows for id 3 if store not populated — latent data leak.
- `ai_openrouter` on `Visualizza RDF` onLoad list — unclear that it should run before `analisi()` supplies the `request` body.
- Dead code: `Input2 Note Carrier Relations`, `Button2 Reset`, Tab2 on Dettaglio, `get_richiesta_full_by_id` on Consultazione, `get_fatt_for_by_id` on Visualizza (`where id = 0`).

### 4.5 Migration blockers / notes for Phase 2
- Need a **backend data model** for: `richieste`, `fattibilita_fornitori`, `fornitori`, `tecnologie`; plus read-only DTOs for `deal` coming from HubSpot replica (`loader.hubs_*`). Confirm `anisetta` vs Mistra placement per project conventions.
- Need a **notification service** abstraction (current `jsGChat1` module body is not in the export — confirm with owner whether it's Teams-only now or still dual Google Chat/Teams).
- Need a **backend LLM proxy** with its own prompts (`utils.system_prompt`, `utils.system_prompt3` are not shown in printed export body — must be recovered from the full JSObject text before rewriting).
- Need a **PDF export service** — current approach bakes layout in JS; consider server-side rendering for consistency and to keep AI output out of the client.
- Confirm intended behaviour of double `notificaChat` call on create, and of `get_richiesta_by_id` re-run inside `creaRecordFattForn`.
- Resolve HubSpot eligibility rule ownership: pinned pipelines likely stale.
- Resolve role names: `'straFatti Full'`/`'Administrator - Sambuca'` don't match the portal's `app_{appname}_access` convention — need a mapping decision.

### 4.6 Recommended next steps
1. Recover the full text of the 4 `utils` JSObjects (esp. `system_prompt`, `system_prompt3`, `jsGChat1` body) before Phase 2 — only the first ~3000 chars of each were decoded here.
2. Hand this audit to `appsmith-migration-spec` to produce the PRD / API contract.
3. Verify against `docs/mistradb/MISTRA.md` and `docs/grappa/GRAPPA.md` whether the `anisetta` schema (`rdf_*` tables) is documented; if not, document and pick target home.
4. Decide if Consultazione and Gestione should merge (same filter bar, same underlying entity; only difference is aggregation + edit entry point gated by role).

---

## 5. What is missing / not verified

- Full body of each `utils` JSObject — printed only the first ~3000 chars per object. Re-extract before spec phase.
- `_$jsGChat1$_jsGChat` module body — empty in our slice; confirms how the Teams/Google Chat webhook posts are signed/formatted.
- Exact schema of `rdf_richieste`, `rdf_fattibilita_fornitori`, `rdf_fornitori`, `rdf_tecnologie` — inferred from SELECT/UPDATE column lists only. Cross-check DB.
- `DOCUMENT_VIEWER_WIDGET1` binding to `generate2` output — not in our key list; verify.
- `BUTTON_GROUP_WIDGET` and `ButtonGroup1` button actions on Dettaglio and Visualizza — need `groupButtons` inspection.
- Published vs. unpublished divergence — we only audited `unpublishedPage`; if staging/prod differ, re-check.

# Phase A — Entity-Operation Model

Source audit: `apps/richieste-fattibilita/AUDIT.md`.

## A.1 Entities inferred from audit

### E1 — `Richiesta` (feasibility request)
Source table: `anisetta.public.rdf_richieste`.

**Fields (inferred from INSERT/UPDATE/SELECT columns):**
- `id` — PK, int, auto; returned by `ins_richiesta`.
- `deal_id` — FK to HubSpot `loader.hubs_deal.id` (external; stored as int).
- `codice_deal` — denormalized HubSpot deal code (string; also used as search key).
- `descrizione` — string (free text, request details).
- `indirizzo` — string (address / coords).
- `stato` — enum: `nuova | in corso | completata | annullata` (default `nuova`; updated via slider + `upd_stato_richiesta`).
- `created_by` — string, user email from `appsmith.user.email`.
- `created_at` — timestamp.
- `updated_at` — timestamp.
- `data_richiesta` — timestamp/date (exposed separately from `created_at`; not clear if auto or editable).
- `fornitori_preferiti` — Postgres array literal of `fornitori.id` (parsed via regex in `utils.stringaArray`).
- `annotazioni_richiedente` — string (selected in `get_richiesta_full_by_id`; no UI write path found).
- `annotazioni_carrier` — string (selected; `Input2 "Note Carrier Relations"` on Dettaglio is unbound, so write path missing).

**Operations:**
| Verb | Where | Notes |
|---|---|---|
| create | `ins_richiesta` (Nuova RDF) | Returns id; body from form widgets + user email. |
| get by id | `get_richiesta_by_id` (Nuova RDF fallback check, Dettaglio) | Parametric; fallback `id || 0`. |
| get full (with feasibilities) | `get_richiesta_full_by_id` (Visualizza) | Left-joins `rdf_fattibilita_fornitori` + fornitore/tecnologia. |
| list (paged filter) | `get_richieste` (Gestione) | Filters: stato set, `codice_deal ILIKE`, `created_by ILIKE`. |
| list with feasibility counts | `get_richieste` (Consultazione) | Adds aggregate columns: `totale_fattibilita`, `draft_count`, `inviata_count`, `sollecitata_count`, `completata_count`, `annullata_count`, `da_ordinare_count`, `prima_fattibilita_data`, `ultima_fattibilita_data`. |
| change stato | `upd_stato_richiesta` (Dettaglio slider) | Persists immediately, no confirm. |

**Open questions (per-entity):**
- Is `data_richiesta` editable or server-set? Currently exposed via default of insert (not in INSERT cols — likely DB default).
- `annotazioni_richiedente` / `annotazioni_carrier` — read but never written. Is the intent to support carrier notes (Input2 unbound) and requestor notes (no UI at all)?
- Should `codice_deal` be derived server-side from `deal_id` instead of denormalized?

---

### E2 — `FattibilitaFornitore` (per-supplier feasibility line)
Source table: `anisetta.public.rdf_fattibilita_fornitori`.

**Fields:**
- `id` — PK.
- `richiesta_id` — FK → Richiesta.id.
- `fornitore_id` — FK → Fornitore.id.
- `tecnologia_id` — FK → Tecnologia.id.
- `stato` — enum: `bozza | inviata | sollecitata | completata | annullata`.
- `descrizione` — string.
- `contatto_fornitore` — string.
- `riferimento_fornitore` — string.
- `annotazioni` — string (final outcome notes).
- `esito_ricevuto_il` — date nullable.
- `da_ordinare` — boolean.
- `profilo_fornitore` — string.
- `nrc` — number/string (non-recurring charge; stored as text in SQL — ambiguous).
- `mrc` — number/string (monthly recurring charge).
- `durata_mesi` — number/string.
- `aderenza_budget` — int 1..5 (index into `utils.score_budget`).
- `copertura` — int `0|1` (bound to Radio Si/No — **the SQL stores copertura as int**; confirm column type).
- `giorni_rilascio` — string/number.
- `data_richiesta` — timestamp (insert-time; used as `data_richiesta_ff` in joins).

**Operations:**
| Verb | Where | Notes |
|---|---|---|
| create (batch) | `utils.creaRecordFattForn` loops `ins_fatt_fornitori` per selected supplier | 3-field insert only; remaining fields filled via subsequent update. |
| list by request | `get_fatt_fornitore` | Joined with fornitore name + tecnologia name. |
| get by id | `get_fatt_for_by_id` | Driven by `tbl_fattib_forn.selectedRow.id`. Visualizza copy is dead (`where id = 0`). |
| update (full) | `upd_fatt_fornitori` via `utils.aggiornaRecordFattForn` | 17-field update; triggers `NotificaChat` side-effect. |
| delete | **none found** | No delete SQL action in the export. |

**Open questions:**
- Is delete intentionally absent? Should cancelling = setting stato `annullata`?
- `nrc`, `mrc`, `durata_mesi`, `giorni_rilascio` — are they numeric or free-text? Widgets are `INPUT_WIDGET_V2` without explicit type set in extracted props.
- `copertura` — keep as `0|1` int, or model as boolean?
- Are all fields required for "completata"? No validation rules found.

---

### E3 — `Fornitore` (supplier)
Source table: `anisetta.public.rdf_fornitori`.

**Fields (inferred from SELECT *, `optionLabel: nome`, `optionValue: id`):**
- `id` — PK.
- `nome` — string.
- Other columns unknown (SELECT *; no other refs).

**Operations:**
- list: `get_fornitori` (ordered by `nome`).

**Open questions:**
- Are there any attributes (contact, tech capabilities, active flag) used elsewhere that aren't surfaced in the UI?
- Is this list user-managed or seeded externally? No CRUD UI exists.

---

### E4 — `Tecnologia` (technology)
Source table: `anisetta.public.rdf_tecnologie`.

**Fields:**
- `id` — PK.
- `nome` — string.
- Others unknown.

**Operations:**
- list: `get_tecnologie` (ordered by `nome`).

**Open questions:**
- Same as Fornitore: authoritative source, other attributes, management UI.

---

### E5 — `Deal` (external, HubSpot replica)
Source: `db-mistra.loader.hubs_deal` joined with `hubs_company`, `hubs_pipeline`, `hubs_stages`, `hubs_owner`.

**Fields actually consumed:**
- `id` (HubSpot deal id).
- `codice` — HubSpot deal code (primary UI identifier).
- `name` → aliased `deal_name`.
- `pipeline` (FK → hubs_pipeline) → `label` as `pipeline`.
- `dealstage` (FK → hubs_stages) → `label` as `stage`, `display_order` used for eligibility.
- `company_id` (FK → hubs_company) → `name` as `company_name`.
- `hubspot_owner_id` → `email` as `owner`.

**Operations:**
- list eligible: `get_deals` (Nuova + Gestione) — pipeline/stage whitelist.
- list filterable: `get_deals` (Consultazione) — parametric fragment.
- get by id: `get_deal_by_id` (Dettaglio, Visualizza).

**Business rule (eligibility for RDF):** `(pipeline = '255768766' AND stage.display_order BETWEEN 1 AND 5) OR (pipeline = '255768768' AND stage.display_order BETWEEN 3 AND 8)` AND `codice <> ''`.

**Open questions:**
- Authoritative pipeline IDs — still current? Which pipelines do they name in business terms?
- Should "eligible deal" be a backend concept owned here, or should the caller hit HubSpot directly?

---

### E6 — `User` (implicit)
Source: Appsmith auth (Keycloak in the target portal).

**Fields used:**
- `email` — written as `created_by`.
- `roles` — checked for manager entitlement.

**Operations:**
- none (identity is an input, not managed here).

**Business rule:** `isManager = roles intersect {'straFatti Full', 'Administrator - Sambuca'}`.

**Open questions:**
- Map to portal's `app_{appname}_access` convention — what is the new role name (`app_rdf_access`? `app_rdf_manager`?)?
- Are there non-manager roles that still need access (e.g., requestor-only vs. carrier-only)?

---

### E7 — `AnalisiAI` (derived artifact, transient)
Not a persisted entity; the LLM output is computed on-demand and rendered.

**Fields:**
- `text` — free-form analysis (from `analisi` orchestrator).
- `json.azioni_raccomandate` — structured recommended actions (from `analisi_json`).

**Open questions:**
- Should AI output be cached or persisted per-richiesta? Current flow regenerates on every view.
- Keep as on-demand service, or trigger on stato transition?

---

### E8 — `Notifica` (side-effect, not persisted)
Teams webhook (Power Automate) posts from `utils.notificaChat` (create) and `utils.NotificaChat` (update).

**Triggers:**
- On `Richiesta` create: Adaptive Card with deal code, requestor, address, description, link.
- On `FattibilitaFornitore` update: text message when `stato | copertura | nrc | mrc` change.

**Open questions:**
- Is Teams the final channel? Audit shows Google Chat code commented out — dead or alternate path?
- Should the recipient depend on deal owner / company / user, or is it always the one webhook?
- Should an audit log of notifications be persisted?

---

## A.2 Cross-entity relationships

```
Richiesta 1 ── N FattibilitaFornitore
Richiesta N ── 1 Deal (via codice_deal / deal_id — double key!)
FattibilitaFornitore N ── 1 Fornitore
FattibilitaFornitore N ── 1 Tecnologia
Richiesta ── * Fornitore (via fornitori_preferiti array, "soft" preference)
Richiesta ── 1 User (created_by email)
```

Concern: both `deal_id` and `codice_deal` are stored on Richiesta. `Consultazione.mergeDati` joins on `codice_deal === deal.codice`, but `get_deal_by_id` on Dettaglio/Visualizza joins on `deal_id`. Duplicated key is fragile.

## A.3 Missing / ambiguous entities — candidate additions

- **Deal eligibility rule** — currently a hard-coded WHERE clause. Could become: `AllowedDealFilter` config entity (pipeline → stage-range) with UI.
- **Notifica log** — persisted history of outbound chat messages (currently fire-and-forget).
- **AnalisiAI cache** — persisted snapshot of last LLM analysis with model id + timestamp.
- **AllegatoRichiesta** (attachment) — not in export at all; confirm whether RDFs ever need document attachments.

## A.4 Expert decision (2026-04-15)

**Vincolo globale: massima compatibilità con l'app Appsmith esistente per coesistenza.**
- Stesso DB `anisetta`, stesse tabelle `rdf_*`, stessi nomi colonna, stessi tipi, stessi valori-enum (stringhe lowercase come ora).
- Nessuna migrazione di schema, nessuna ridenominazione.
- Semantica invariata: `copertura` resta int 0/1; `nrc`/`mrc`/`durata_mesi`/`giorni_rilascio` restano come sono in DB (da verificare col DDL, ma non si cambiano); `fornitori_preferiti` resta array literal Postgres.
- `deal_id` + `codice_deal` entrambi persistiti come oggi.
- Regola eleggibilità deal pinnata agli stessi pipeline id (`255768766` / `255768768`) e range `display_order` attuali.
- Ruoli: replicare l'attuale check manager (`straFatti Full`, `Administrator - Sambuca`) finché non c'è mapping esplicito Keycloak; aggiungere in parallelo il ruolo `app_rdf_access` per accesso base (convenzione portal), senza rimuovere i check esistenti.
- Notifiche: Teams via lo stesso webhook; Google Chat resta codice morto (non reintrodotto).
- AI: on-demand come ora, stessi prompt, stessi modelli. Nessuna cache persistita.
- Allegati: fuori scope (non esistono nell'app attuale).
- `annotazioni_*`: porta il comportamento letto-only se non c'è write path esistente. `Input2 "Note Carrier Relations"` resta unbound (dead widget nell'originale, non lo re-implementiamo).
- `FattibilitaFornitore` delete: non esiste oggi → non esiste nel nuovo; "annullare" = set stato `annullata`.
- `Richiesta.data_richiesta`: trattato come server-set (DB default come oggi).
- `Fornitore` / `Tecnologia`: nessuna management UI (seed via DB come oggi); solo list.

**Conseguenza architetturale:** il backend Go è un **thin wrapper** sulle stesse query attuali, esposte come endpoint REST. Nessun re-modeling di dominio. Le regole di business restano dove sono (pipeline eleggibili, stato taxonomy, campi trigger-notifica).

## A.5 Residui irriducibili (non bloccanti, segnalati)

Queste cose non si cambiano ora ma vanno tracciate per il futuro (post-coesistenza):

- SQL injection nelle query di filtro — **dobbiamo risolverla comunque in migrazione** (parametrizzazione backend); comportamento utente identico, ma non possiamo portare `ILIKE '{{"%" + i_deal.text + "%"}}'` come stringa grezza. Parametrizzare non è un semantic change.
- Webhook Teams hard-coded — spostato in config backend (env var), non più nel codice client. Stesso URL, stesso payload.
- Fallback `|| 0` / `|| 3` nelle SQL — riprodotti a livello di endpoint (se id mancante → 404/empty), non silenziosamente alla riga 3.
- `get_richiesta_full_by_id` con `where rr.id = 3` su Consultazione e `get_fatt_for_by_id where id = 0` su Visualizza — dead code, non portati.
- Doppia chiamata `notificaChat` su Nuova RDF → **risolto 2026-04-15:** deduplicata, una sola notifica per creazione.
- `get_richiesta_by_id.run()` dentro `creaRecordFattForn` → **risolto 2026-04-15:** trick Appsmith per forzare refresh; nel nuovo stack non serve, non portato.

---

## A.4-legacy Gap questions originali (archiviate, risolte dal vincolo di coesistenza)

1. **Richiesta.data_richiesta** — server-set at insert, or editable? Is it distinct from `created_at`?
2. **Richiesta annotazioni** — should `annotazioni_richiedente` and `annotazioni_carrier` have UI write paths (currently neither does, carrier input is present but unbound)?
3. **FattibilitaFornitore delete** — intentional omission? Should a "cancel" action set stato `annullata`, or hard-delete?
4. **Numeric fields (nrc/mrc/durata_mesi/giorni_rilascio)** — numeric columns or free-text? What about currency/unit?
5. **`copertura`** — keep int 0/1 or promote to boolean? Are there planned values beyond yes/no (e.g., "parziale")?
6. **Fornitore / Tecnologia** — do they need a management UI, or are they seeded? Any fields beyond `nome` (active flag, contact, supported tecnologie mapping)?
7. **Deal eligibility** — confirm pipelines `255768766` and `255768768` by business name; is this rule stable or should it be configurable?
8. **Role model** — target role names under `app_{appname}_access` convention; do we need separate `requestor` vs `carrier` vs `manager`?
9. **`deal_id` vs `codice_deal`** — keep both, or standardize on one?
10. **LLM output** — persist, cache, or always on-demand? Who chooses the model/prompt — admin-configurable or fixed?
11. **Notification** — Teams only, or reinstate Google Chat? Routing rules (always same webhook, or per-deal-owner)?
12. **Attachments** — do RDFs ever need documents (offer PDFs, coverage maps)?

---

*Extracted: 2026-04-15. Awaiting expert answers before finalizing A.*

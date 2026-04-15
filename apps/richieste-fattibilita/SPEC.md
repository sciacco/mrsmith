# Application Specification — Richieste Fattibilità

## Summary

- **Application name:** Richieste Fattibilità (slug `rdf`)
- **Audit source:** `apps/richieste-fattibilita/AUDIT.md` (da export Appsmith `richieste-fattibilita.json`)
- **Spec status:** draft-1 (2026-04-15), pronto per design/implementazione
- **Last updated decisions (2026-04-15):**
  - Vincolo di coesistenza: zero schema change, stesso DB `anisetta`, stessa semantica dati.
  - Modello ruoli `app_rdf_access` / `app_rdf_manager` con mapping legacy.
  - LLM proxy, PDF render, notifiche Teams → tutti lato backend.
  - Cleanup di bug latenti concordati (doppia notifica, refresh-trick, dead widgets).
  - Phase files di riferimento: `richieste-fattibilita-migspec-{A,B,C,D}.md`.

## Current-State Evidence

### Source pages/views
6 pagine Appsmith: `Home` (droppata), `Nuova RDF`, `Gestione RDF Carrier`, `Dettaglio RDF Carrier`, `Consultazione RDF`, `Visualizza RDF`. Slug `isHidden` per le tre pagine manager-side.

### Source entities and operations
Tabelle `anisetta`: `rdf_richieste`, `rdf_fattibilita_fornitori`, `rdf_fornitori`, `rdf_tecnologie`. Lettura read-only su `db-mistra.loader.hubs_*`. 46 azioni Appsmith (SQL + JS + REST) distribuite tra le pagine, con forte duplicazione.

### Source integrations and datasources
- `anisetta` (Postgres, R/W) — store primario RDF.
- `db-mistra` (Postgres, R) — replica HubSpot.
- `openrouter` (REST) — LLM completion.
- Webhook Teams (Power Automate) — notifiche uscita.
- Keycloak — auth (nel portal).

### Known audit gaps or ambiguities
- Corpo completo dei JSObject `utils` (Visualizza) e modulo `jsGChat1` troncato nell'audit — i prompt LLM esatti e il payload webhook esatti vanno recuperati dall'export raw prima dell'implementazione.
- DDL effettivo di `rdf_*` non letto dalla spec — serve verifica tipi `nrc`, `mrc`, `durata_mesi`, `giorni_rilascio`, `copertura`, `data_richiesta`.
- ACL workspace-side Appsmith non nell'export (solo gate via `isHidden` + `IsManager()` runtime).

---

## Entity Catalog

### Entity: Richiesta
- **Purpose:** una richiesta di fattibilità associata a un deal HubSpot.
- **Tabella:** `anisetta.public.rdf_richieste`.
- **Operations:** create, get by id, get full (con fattibilità), list (filter), list summary (con counter fattibilità + deal enrich), update stato.
- **Fields:**
  - `id` int PK auto
  - `deal_id` int (FK logico → `loader.hubs_deal.id`)
  - `codice_deal` string (denormalizzato, usato come chiave UI)
  - `descrizione` string (required UI)
  - `indirizzo` string (required UI)
  - `stato` enum `nuova | in corso | completata | annullata` (default `nuova`)
  - `created_by` string (email Keycloak)
  - `created_at` timestamp (DB default)
  - `updated_at` timestamp
  - `data_richiesta` timestamp (server-set, DB default)
  - `fornitori_preferiti` int[] (serializzato come PG array literal `{a,b,c}`)
  - `annotazioni_richiedente` string (read-only nell'app, nessun write path)
  - `annotazioni_carrier` string (read-only nell'app)
- **Relationships:**
  - 1..N → `FattibilitaFornitore`
  - N..1 → `Deal` (externo, via `deal_id` e `codice_deal`)
  - N..1 → `User` (via `created_by`)
  - N..* → `Fornitore` (preferenze tramite `fornitori_preferiti`)
- **Constraints & business rules:**
  - Eleggibilità deal al momento della create: pipeline `255768766` stages 1–5 OR pipeline `255768768` stages 3–8 AND `codice <> ''` (costante backend).
  - On create: notifica Teams Adaptive Card (best-effort, log-and-continue).
  - Manager gate su update stato.
- **Open questions:** nessuna bloccante.

### Entity: FattibilitaFornitore
- **Purpose:** una riga di fattibilità per combinazione fornitore+tecnologia nel contesto di una Richiesta.
- **Tabella:** `anisetta.public.rdf_fattibilita_fornitori`.
- **Operations:** create (batch da modal), get by id, list by richiesta (join con fornitore+tecnologia), update full.
- **Fields:**
  - `id` int PK
  - `richiesta_id` int FK → Richiesta.id
  - `fornitore_id` int FK → Fornitore.id
  - `tecnologia_id` int FK → Tecnologia.id
  - `stato` enum `bozza | inviata | sollecitata | completata | annullata` (default `bozza`)
  - `descrizione` string
  - `contatto_fornitore` string
  - `riferimento_fornitore` string
  - `annotazioni` string (note esito finale)
  - `esito_ricevuto_il` date nullable
  - `da_ordinare` bool
  - `profilo_fornitore` string
  - `nrc` — tipo da confermare via DDL (stringa o numeric)
  - `mrc` — idem
  - `durata_mesi` — idem
  - `giorni_rilascio` — idem
  - `aderenza_budget` int 1..5 (indice in `score_budget`)
  - `copertura` int 0/1
  - `data_richiesta` timestamp
- **Relationships:** N..1 Richiesta, N..1 Fornitore, N..1 Tecnologia.
- **Constraints & business rules:**
  - No delete. "Annullare" = set `stato = 'annullata'`.
  - Create batch non-idempotente (come oggi): doppio click → doppio record. FE debounce.
  - Update: se cambia uno di `stato | copertura | nrc | mrc`, notifica Teams text message con diff.
  - Tutte le scritture richiedono `app_rdf_manager`.
- **Open questions:** tipi SQL dei campi numerici.

### Entity: Fornitore
- **Purpose:** anagrafica fornitori.
- **Tabella:** `anisetta.public.rdf_fornitori`.
- **Operations:** list.
- **Fields esposti:** `id`, `nome` (ordering). Altri sconosciuti (SELECT *).
- **Relationships:** referenziato da `FattibilitaFornitore.fornitore_id` e da `Richiesta.fornitori_preferiti[]`.
- **Constraints:** no UI management. Seed esterno/out-of-band.

### Entity: Tecnologia
- **Purpose:** anagrafica tecnologie di accesso (fibra, FWA, dark fiber, …).
- **Tabella:** `anisetta.public.rdf_tecnologie`.
- **Operations:** list.
- **Fields esposti:** `id`, `nome`.
- **Relationships:** referenziata da `FattibilitaFornitore.tecnologia_id`.
- **Constraints:** no UI management.

### Entity: Deal (external, read-only)
- **Purpose:** deal HubSpot di riferimento per la RDF.
- **Source:** `db-mistra.loader.hubs_deal` + join `hubs_company`, `hubs_pipeline`, `hubs_stages`, `hubs_owner`.
- **Operations:** list eligible, list filterable (con filtro cliente), get by id.
- **Fields consumati:** `id`, `codice`, `name` (→ `deal_name`), `pipeline.label`, `stage.label` + `display_order`, `company_id → company.name` (→ `company_name`), `hubspot_owner_id → owner.email` (→ `owner`).
- **Constraints:** read-only. Eleggibilità definita dal backend (vedi Richiesta).

### Entity: User (implicit)
- Fonte: token Keycloak.
- Fields usati: `email` (→ `created_by`), `roles`.
- Ruoli rilevanti: `app_rdf_access` (base), `app_rdf_manager` (carrier/admin). Mapping legacy: `straFatti Full` e `Administrator - Sambuca` → `app_rdf_manager`.

### Derived / transient

- **AnalisiAI** — output LLM (text + json.azioni_raccomandate). On-demand, non persistito. Endpoint backend.
- **Notifica** — side-effect (Teams). Non persistita.
- **PdfRichiesta** — render on-demand server-side. Non persistito.

---

## View Specifications

### View: Nuova RDF (route `/richieste/new`, ruolo `app_rdf_access`)
- **User intent:** il requestor crea una nuova RDF scegliendo un deal HubSpot.
- **Interaction pattern:** master-detail creation (deal picker + form).
- **Main data shown or edited:** lista deal eleggibili; form con `indirizzo`, `descrizione`, `fornitori_preferiti` (opz.).
- **Key actions:** "Inserisci RDF" (POST + toast + navigate), "Reset".
- **Entry/exit:** entry dal menu portal o da Consultazione; exit → Consultazione dopo create.
- **Notes on current vs intended:**
  - Required visuale su `indirizzo`/`descrizione` (nuovo).
  - Una sola notifica Teams (era doppia).
  - Paginazione/ricerca server-side sui deal (ora cap 300 + filtro client-side tabella).

### View: Gestione RDF Carrier (route `/richieste/gestione`, ruolo `app_rdf_manager`)
- **User intent:** il team carrier monitora RDF aperte e apre una per lavorarla.
- **Interaction pattern:** filtered list.
- **Main data shown:** card per RDF con codice deal, id+data+stato, indirizzo, descrizione. Nuovo: anche `company_name` + `deal_name` (join server-side).
- **Key actions:** filtri (stato default `[nuova, in corso]`, codice deal, richiedente), "Aggiorna", "Gestisci" → Dettaglio.
- **Entry/exit:** landing default per manager; exit → Dettaglio.
- **Notes:** componente lista condiviso con Consultazione.

### View: Dettaglio RDF Carrier (route `/richieste/:id`, ruolo `app_rdf_manager`)
- **User intent:** il carrier modifica stato RDF e gestisce righe fattibilità.
- **Interaction pattern:** aggregate editor (header + child list + child editor + add modal).
- **Main data shown/edited:** header richiesta, slider stato, tabella fattibilità, form edit 17 campi, modal add batch.
- **Key actions:** change stato (PATCH immediata), add fattibilità batch, update fattibilità (con diff-notify), reset form.
- **Entry/exit:** entry da Gestione o da Consultazione (icon manager); exit → back.
- **Notes:** dead widgets `Input2 Note Carrier Relations`, `Tab2`, `Button2 Reset` non portati (reset riagganciato alla selezione corrente).

### View: Consultazione RDF (route `/richieste`, ruolo `app_rdf_access`)
- **User intent:** il requestor consulta le proprie RDF (e altrui) con progresso fattibilità; il manager ha shortcut "Gestisci".
- **Interaction pattern:** filtered list + counters + role-gated quick-action.
- **Main data shown:** card con codice deal, id+data+stato, cliente/indirizzo, descrizione, counter "Bozza: X Inv: Y …", bottoni "Visualizza" (tutti) + "Gestisci" (solo manager).
- **Key actions:** auto-load con default filtri (nuovo), filtri, Visualizza, Gestisci (manager).
- **Entry/exit:** landing default per requestor; exit → Visualizza o Dettaglio.
- **Notes:** merge `richieste + deals` ora server-side (era client-side `utils.mergeDati`).

### View: Visualizza RDF (route `/richieste/:id/view`, ruolo `app_rdf_access`)
- **User intent:** read-only viewer con analisi LLM e PDF.
- **Interaction pattern:** tabbed read-only viewer.
- **Main data shown:** 4 tab → Riepilogo, Analisi, PDF, Azioni.
- **Key actions:** tab-switch lazy-load (Analisi/Azioni/PDF triggerano backend al click).
- **Entry/exit:** entry da Consultazione o da link diretto; exit → back.
- **Notes:** `ai_openrouter` rimosso da onLoad (era bacato); PDF renderizzato server-side.

---

## Logic Allocation

### Backend responsibilities
- Tutte le SQL (anisetta + mistra) parametrizzate.
- Regole di dominio:
  - Eleggibilità deal (costanti pipeline/stage).
  - Enum stato richiesta + fattibilità.
  - Diff-notify field set (`stato | copertura | nrc | mrc`).
  - Manager gate sugli endpoint di scrittura.
  - Parsing array `fornitori_preferiti` (PG literal ↔ int[]).
- Side-effect: notifica Teams (Adaptive Card su create, text message su update diff).
- LLM proxy a OpenRouter (prompt, API key, modelli pinnati server-side).
- PDF render (stesso layout/contenuto dell'attuale `generate2`).
- Auth: validazione token Keycloak, mapping ruoli legacy → nuovi.

### Frontend responsibilities
- Orchestrazione UI: fetch on view-load, form binding, submit, navigate.
- Formatter presentazionali: date, counter string, budget label map, `copertura ? 'SI' : ''`, "Preferenza fornitori: …".
- UI gate manager (visibilità shortcut) da token.
- Debounce submit per evitare duplicati batch.
- Lazy-load tab su Visualizza.

### Shared validation or formatting
- Tipi DTO generati da OpenAPI (richiesta, fattibilità, deal, fornitore, tecnologia).
- Enum stati condivisi come costanti generate.

### Rules being revised rather than ported
- Doppia notifica Teams su create → una.
- `get_richiesta_by_id` refresh-trick in `creaRecordFattForn` → rimosso.
- `ai_openrouter` in onLoad → rimosso (lazy tab).
- SQL injection (filtri `ILIKE` + fragment `paramsDeals.query`) → parametrizzata.
- Dead widget/tab/SQL non portati.

---

## Integrations and Data Flow

### External systems and purpose
- `anisetta` Postgres: store RDF (R/W).
- `db-mistra` Postgres: replica HubSpot (R).
- Teams webhook (Power Automate): notifiche out (W, env `RDF_TEAMS_WEBHOOK_URL`).
- OpenRouter: LLM completion (env `OPENROUTER_API_KEY`).
- Keycloak: OIDC auth (infrastruttura portal già esistente).

### End-to-end user journeys
Documentate in `richieste-fattibilita-migspec-D.md` §D.3:
1. Requestor crea RDF → POST + notify + navigate Consultazione.
2. Requestor consulta/visualizza → summary → full + deal → lazy analisi/PDF.
3. Manager gestisce → Gestione → Dettaglio → slider stato, modal batch, form update con diff-notify.
4. Deep-link → path param `/richieste/:id[/view]` (niente Appsmith store).

### Background or triggered processes
Nessuno. Nessun cron, nessun webhook in ingresso. Tutti i trigger sono user-initiated.

### Data ownership boundaries
- `rdf_*` tables: owned da questo app + Appsmith in coesistenza.
- `rdf_fornitori` / `rdf_tecnologie`: seed out-of-band (DBA).
- `loader.hubs_*`: owned da pipeline mistra.
- Secrets (webhook, API key): operations via env var.

### Coesistenza Appsmith
- Stesse tabelle, scritture concorrenti sicure a livello row-level PG.
- Teams notify in Appsmith `utils.notificaChat` va disattivata al go-live del nuovo app per evitare doppie notifiche.
- Nessuna migrazione dati: i record creati da un lato sono immediatamente leggibili dall'altro.

---

## API Contract Summary

Tutti gli endpoint sotto `/api/rdf`. Ruoli tra parentesi.

### Read endpoints
| Endpoint | Ruolo | Scopo |
|---|---|---|
| `GET /rdf/deals?q=&cliente=` | access | Deal eleggibili (regole pipeline/stage) + filtro cliente opz. |
| `GET /rdf/deals/:id` | access | Dettaglio deal per header RDF. |
| `GET /rdf/fornitori` | access | Lista fornitori. |
| `GET /rdf/tecnologie` | access | Lista tecnologie. |
| `GET /rdf/richieste?stato=&deal=&richiedente=` | access | List flat (per Gestione). |
| `GET /rdf/richieste/summary?stato=&deal=&richiedente=&cliente=` | access | List con counters + deal enrich (per Consultazione e Gestione). |
| `GET /rdf/richieste/:id` | access | Singola richiesta. |
| `GET /rdf/richieste/:id/full` | access | Richiesta joined con fattibilità + fornitore/tecnologia (Visualizza). |
| `GET /rdf/richieste/:id/fattibilita` | access | Righe fattibilità della richiesta. |
| `GET /rdf/fattibilita/:id` | access | Singola fattibilità. |
| `GET /rdf/richieste/:id/pdf` | access | PDF render (application/pdf). |

### Write commands
| Endpoint | Ruolo | Scopo |
|---|---|---|
| `POST /rdf/richieste` | access | Crea RDF, trigger notify card. |
| `POST /rdf/richieste/:id/fattibilita` | manager | Batch create (N righe bozza). |
| `PATCH /rdf/richieste/:id/stato` | manager | Cambia stato richiesta. |
| `PATCH /rdf/fattibilita/:id` | manager | Update fattibilità, diff-notify su `stato|copertura|nrc|mrc`. |

### Derived / workflow-specific
| Endpoint | Ruolo | Scopo |
|---|---|---|
| `POST /rdf/richieste/:id/analisi` | access | Triggera LLM analisi testuale (cache opz. futura). |
| `POST /rdf/richieste/:id/analisi-json` | access | Triggera LLM analisi strutturata (`azioni_raccomandate`). |

Formato risposte: JSON con `{ data }` envelope o record diretto, secondo convenzione portal esistente (da uniformare in fase di scaffolding).

---

## Constraints and Non-Functional Requirements

### Security or compliance
- Nessun secret lato client (webhook Teams, OpenRouter API key server-only).
- Tutte le SQL parametrizzate (no string interpolation).
- Manager gate enforced lato backend, non solo UI.
- Scope token Keycloak verificato su ogni request (middleware portal).
- Log strutturato di notifiche e chiamate LLM (audit trail, senza prompt full per GDPR).

### Performance or scale
- Volume atteso: ordine di grandezza uguale all'Appsmith attuale (poche migliaia di RDF). Nessun requisito real-time.
- LLM latency: 2–10s tipico; FE mostra skeleton loader.
- PDF render: synchronous <5s accettabile.
- Cap deal list: 300 come oggi, ricerca server-side se l'utente digita.
- Auto-load Consultazione: con 5 filtri default; accettabile su volume attuale.

### Operational constraints
- Coesistenza scritture con Appsmith durante la transizione.
- Zero downtime per il go-live (feature flag? redirect graduale da Appsmith).
- Rollback plan: tornare a Appsmith per l'URL è banale finché lo schema è invariato.

### UX or accessibility expectations
- Allineamento a `docs/UI-UX.md` del portal (design system Stripe-level + Matrix).
- Label italiane conservate dall'app attuale.
- Slider stato con conferma toast (no modale: come oggi).
- Deep-linking via URL path param.

---

## Open Questions and Deferred Decisions

### Risolte (2026-04-15) — dettaglio in §Appendice A

| # | Question | Resolution |
|---|---|---|
| 1 | Tipi SQL `rdf_*` | **Risolta** via `docs/anisetta_schema.json`. DDL completo in Appendice A. Key finding: `fornitori_preferiti` è `text` (non array PG); `nrc`/`mrc` sono `numeric(12,2)` (currency); `copertura` è `smallint`; entrambi gli enum stato hanno check constraint DB. Esiste tabella `rdf_allegati` (dead, non usata). |
| 3 | Payload Teams webhook | **Risolta** dal body completo di `utils.notificaChat`. Shape: MessageCard con attachment AdaptiveCard v1.2. URL Power Automate firmato via query param (`sig`), nessun header custom. Dettaglio in Appendice A. |
| 4 | Prompt LLM | **Risolti** dal body JSObject `utils` completo su Visualizza RDF. `system_prompt` (markdown) per output testuale, `system_prompt3` (markdown) per output JSON strutturato con schema `{azioni_raccomandate, valutazioni}`. Citati in Appendice A. |

### Ancora aperte

| # | Question | Needed input | Decision owner |
|---|---|---|---|
| 2 | Mapping utenti Keycloak esistenti `straFatti Full` / `Administrator - Sambuca` → `app_rdf_manager` | Lista utenti/ruoli Keycloak | ops / security |
| 5 | Disattivazione `utils.notificaChat` in Appsmith al go-live | Coordinamento release | ops |
| 6 | Default landing per utente con entrambi i ruoli | UX decision | product |
| 7 | Caching LLM output (deferred v2) | UX + budget | product |
| 8 | Ricerca server-side sui deal oltre 300 | UX decision | product |
| 9 (nuova) | `rdf_allegati` è prevista come feature futura o dead code da rimuovere? Schema `(id, allegati text[])` senza FK a `rdf_richieste` — incompleto | product | product |

---

## Acceptance Notes

### What the audit proved directly
- Inventario completo di pagine, widgets, queries, JSObjects, datasources.
- Dependency graph widget → query → DB.
- Regole di business embedded (eleggibilità deal, enum stati, diff-notify set).
- Duplicazioni, dead code, bug latenti.
- Ruoli manager whitelist (nomi esatti).

### What the expert confirmed
- Vincolo di coesistenza (zero schema/semantic change).
- Cleanup di doppia notifica e refresh-trick.
- Modello ruoli `app_rdf_access` + `app_rdf_manager` con mapping legacy.
- Auto-load Consultazione + lazy-load LLM/PDF su Visualizza.
- Required visuale su Nuova RDF.
- Enrichment Gestione con company_name/deal_name.

### What still needs validation
- 8 open question tabulate sopra — nessuna bloccante per kick-off backend/frontend, tutte risolvibili in parallelo con ops/DBA.
- Tipi numerici campi fattibilità (Open #1) bloccano in senso stretto la generazione OpenAPI → va verificato subito quando si parte.
- Prompt LLM (#4) e payload Teams (#3) bloccano solo le funzioni relative, non l'intera app.

---

## Appendice A — Dettagli risolti post-spec

### A.1 DDL autoritativo `rdf_*` (da `docs/anisetta_schema.json`)

**`rdf_richieste`**
```
id                     int PK (serial)
deal_id                bigint NULL
data_richiesta         date NOT NULL default CURRENT_DATE
descrizione            text NOT NULL
indirizzo              text NOT NULL
stato                  varchar(40) NOT NULL default 'nuova'
                         CHECK stato IN ('nuova','in corso','completata','annullata')
annotazioni_richiedente text NULL
annotazioni_carrier    text NULL
created_by             varchar(120) NULL
created_at             timestamp NOT NULL default CURRENT_TIMESTAMP
updated_at             timestamp NULL default CURRENT_TIMESTAMP
fornitori_preferiti    text NOT NULL default ''      -- PG array literal, es. '{5,7}'
codice_deal            varchar(64) NULL default ''
```

**`rdf_fattibilita_fornitori`**
```
id                    int PK (serial)
richiesta_id          int NOT NULL REFERENCES rdf_richieste ON DELETE CASCADE
fornitore_id          int NOT NULL REFERENCES rdf_fornitori ON DELETE RESTRICT
data_richiesta        date NOT NULL default CURRENT_DATE
tecnologia_id         int NOT NULL REFERENCES rdf_tecnologie ON DELETE RESTRICT
descrizione           text NULL
contatto_fornitore    varchar(100) NULL
riferimento_fornitore varchar(100) NULL
stato                 varchar(20) NOT NULL default 'bozza'
                        CHECK stato IN ('bozza','inviata','sollecitata','completata','annullata')
annotazioni           text NULL
esito_ricevuto_il     date NULL
da_ordinare           boolean NULL default false
profilo_fornitore     varchar(100) NULL
nrc                   numeric(12,2) NULL default 0       -- currency EUR, confidenziale
mrc                   numeric(12,2) NULL default 0       -- currency EUR, confidenziale
durata_mesi           int NULL default 24
aderenza_budget       int NULL default 0                 -- 0..5 (0 = non valutato)
copertura             smallint NULL default 0            -- 0|1
giorni_rilascio       int NULL default 0
```

**`rdf_fornitori`** — `(id int PK, nome varchar(50) NULL)`. Nome nullable (intenzionalmente?).
**`rdf_tecnologie`** — `(id int PK, nome varchar(100) NOT NULL)`.
**`rdf_allegati`** — `(id int PK, allegati text[])` — **esiste ma non usata dall'app**. Nessuna FK a `rdf_richieste`. Dead/riservata.

### A.2 Payload webhook Teams (Adaptive Card)

Endpoint: Power Automate URL (env `RDF_TEAMS_WEBHOOK_URL`). La URL stessa contiene il token `sig=...`. Nessun header custom.

**Create-richiesta (card):**
```json
{
  "type": "MessageCard",
  "attachments": [{
    "contentType": "application/vnd.microsoft.card.adaptive",
    "content": {
      "type": "AdaptiveCard",
      "version": "1.2",
      "body": [
        {"type":"TextBlock","text":"Richiesta Nuova Fattibilità - Deal <codice_deal>","weight":"Bolder","size":"Medium"},
        {"type":"TextBlock","text":"Da <created_by>","weight":"Normal","size":"Small"},
        {"type":"TextBlock","text":"Dettagli della richiesta","weight":"Bolder","size":"Small"},
        {"type":"TextBlock","text":"Cliente:<company_name>","wrap":true},
        {"type":"TextBlock","text":"Indirizzo:<indirizzo>","wrap":true},
        {"type":"TextBlock","text":"Descrizione:<descrizione>","wrap":true}
      ],
      "actions": [{"type":"Action.OpenUrl","title":"Apri Smartapp","url":"<portal_hostname>"}]
    }
  }]
}
```

**Update-fattibilità (text):**
```
Aggiornamento RDF *<deal_codice>* (_<deal_name>_)
- <fornitore> / <tecnologia>
- Stato: *<nuovo_stato>* [/ Copertura: *SI*]
```
Il suffisso " / Copertura: *SI* " è presente solo se `copertura == 1`.

### A.3 Prompt LLM

**Modelli pinnati:**
- `analisi` (testuale): `google/gemini-2.5-flash-lite-preview-09-2025`, `temperature: 0`, `max_tokens: 4096`.
- `analisi_json` (JSON strutturato): `google/gemini-2.5-flash-lite-preview-06-17`, stessa temp/tokens, `response_format: {type: "json_object"}`.

**`system_prompt` (per analisi testuale)** — prompt markdown italiano di ~2800 caratteri con:
- interpretazione campi (stato_ff, esito_ricevuto_il, copertura, aderenza_budget, durata_mesi, giorni_rilascio, fornitori_preferiti);
- regole: durata max 3 mesi per FTTH/FTTC/VDSL, 24 mesi altrove, >24 = criticità; `giorni_rilascio > 60` = ritardo;
- restrizione: **mai citare nrc/mrc esplicitamente** (confidenziali);
- formato output: "Azioni raccomandate" in cima + valutazioni per fornitore/tecnologia.

**`system_prompt3` (per analisi JSON)** — stesso contenuto semantico di `system_prompt` + schema JSON vincolante:
```
{
  "azioni_raccomandate": [
    {"azione":"sollecitare|escludere|privilegiare|...","fornitore":"...","tecnologia":"...","motivo":"..."}
  ],
  "valutazioni": [
    {"fornitore":"...","tecnologia":"...","stato":"...",
     "copertura":"Presente|Assente|Non indicata","aderenza_budget":"Pessima|...","durata_mesi":int,
     "giorni_rilascio":int|null,"preferito":bool,"criticita":"..."}
  ]
}
```

I prompt full-text sono checked-in nell'export Appsmith (campo `body` di `utils` JSObject su `Visualizza RDF`) e vanno portati verbatim lato backend Go in una costante / template file (es. `backend/internal/rdf/ai/prompts.go`).

### A.4 Regole di dominio derivate dai prompt (consolidamento)

Le regole LLM descrivono implicitamente il dominio — tenerle allineate col service layer:
- Enum stati confermati da CHECK constraint DB (non re-validare in BE, fidarsi del DB).
- `aderenza_budget` semantic: `0 = non valutato`, `1..5` con label in `score_budget`.
- `copertura` semantic: `1 = presente`, `0 = assente/non-indicata`.
- `giorni_rilascio`: `0 = non specificato / entro SLA`.
- `fornitori_preferiti`: lista ID, omettere dai confronti se vuota.
- Soglie business (durata 3/24 mesi, ritardi 60gg) sono **logica di riepilogo LLM**, non validazioni applicative.

### A.5 Confidenzialità

- `nrc` e `mrc` non devono mai apparire nell'output LLM (prompt-enforced).
- Se in futuro l'AI analysis venisse esposta a pubblico non-autorizzato (es. email allegato), il backend deve stripare qualsiasi citazione numerica sospetta. Fuori scope v1 — il rischio rimane nel prompt.
- Log server-side di chiamate LLM: conservare solo id richiesta + modello + timestamp + eventuale token-usage, **non il prompt full con dati sensibili**. Se servono traces di debug, dietro feature flag ops-only.

### A.6 Coesistenza `fornitori_preferiti`

Il campo è `text` con array literal Postgres come stringa. Per non rompere Appsmith che usa `utils.stringaArray`:
- Il BE scrive nel formato identico: `'{5,7}'` (virgole, graffe, niente spazi, valori numerici).
- Se l'array è vuoto: stringa vuota `''` (default DB).
- Il BE espone in JSON come `int[]` (es. `[5, 7]`), converte in ingresso/uscita a livello repo.

### A.7 `rdf_allegati` — dead table

Schema monco (solo `id` + `allegati text[]`, niente FK). Non referenziata da UI. Non portata nel nuovo app. Tracciata come Open #9 se in futuro serve una feature allegati vera (richiederebbe design: FK a richiesta o fattibilità? storage? versioning?).

---

## Riferimenti

- Audit: `apps/richieste-fattibilita/AUDIT.md`
- Phase A (entità): `apps/richieste-fattibilita/richieste-fattibilita-migspec-A.md`
- Phase B (UX): `apps/richieste-fattibilita/richieste-fattibilita-migspec-B.md`
- Phase C (logic placement): `apps/richieste-fattibilita/richieste-fattibilita-migspec-C.md`
- Phase D (integrazioni): `apps/richieste-fattibilita/richieste-fattibilita-migspec-D.md`
- Portal conventions: `CLAUDE.md` (New App Checklist), `AGENTS.md`, `docs/UI-UX.md`, `docs/IMPLEMENTATION-PLANNING.md`

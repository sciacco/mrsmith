# Phase D — Integration & Data Flow

## D.1 External systems

| Sistema | Ruolo | Tipo accesso | Owner lato nuovo stack | Note |
|---|---|---|---|---|
| `anisetta` Postgres | Store primario delle entità RDF (`rdf_richieste`, `rdf_fattibilita_fornitori`, `rdf_fornitori`, `rdf_tecnologie`) | R/W | BE package `rdf/repo` | Stessa istanza e credenziali usate da Appsmith, coesistenza in scrittura concorrente. |
| `db-mistra` Postgres | Replica HubSpot (`loader.hubs_deal`, `hubs_company`, `hubs_pipeline`, `hubs_stages`, `hubs_owner`) | **R only** | BE package `rdf/repo` | Repo separato, user DB in read-only se possibile. Mai write. |
| Teams (Power Automate) | Destinazione notifiche (webhook HTTP POST) | W only | BE package `rdf/notifier` | URL in env var `RDF_TEAMS_WEBHOOK_URL`. Fire-and-forget, log-and-continue on failure. Timeout 5s. |
| OpenRouter | LLM completion per `analisi` / `analisi_json` | R (completion API) | BE package `rdf/ai` | API key in env var `OPENROUTER_API_KEY`. Modelli pinnati come oggi (`google/gemini-2.5-flash-lite-preview-09-2025` per analisi testuale, `...-06-17` per JSON). |
| Keycloak | Auth (OAuth2/OIDC) | auth only | `packages/auth-client` (già esistente) | Ruoli `app_rdf_access` / `app_rdf_manager` + mapping legacy `straFatti Full` / `Administrator - Sambuca`. |
| HubSpot (upstream) | Fonte di verità dei deal | — | **non integrato direttamente** | Passiamo sempre via `db-mistra`. |

## D.2 Coesistenza Appsmith — pattern di concorrenza

- Scritture sulle stesse tabelle da Appsmith e dal nuovo app. Nessun lock applicativo.
- Nessun cambio schema: ok concorrenza a livello record (PG row-level locking copre update, insert è append).
- Il flag `data_richiesta` / `updated_at` restano gestiti come oggi (default PG `now()` o trigger esistenti — da verificare in DDL).
- `fornitori_preferiti` scritto come array literal Postgres dal BE (stesso formato dell'INSERT attuale `{1,2,3}`), così Appsmith continua a leggerlo con `utils.stringaArray`.
- **Rischio:** una RDF creata da Appsmith non manda notifica dal nuovo stack (ovviamente). Stessa cosa al contrario: se la notifica è ora server-side, Appsmith continua a mandarla dal suo JSObject. Durante la fase di coesistenza si potrebbe ricevere **doppia notifica** (se un utente crea da Appsmith) o **nessuna** (se crea dal nuovo app mentre un'altra via è in uso). **Soluzione:** disattivare `utils.notificaChat` nell'Appsmith una volta che il nuovo app è in prod (rimuovere dalla `onClick` del btn_save). Coesistenza ≠ parità funzionale su side-effect.

## D.3 End-to-end user journeys

### Journey 1 — Requestor crea una nuova RDF
```
User (app_rdf_access)
  → Portal Matrix
  → launch "RDF"
  → redirect Consultazione RDF          (nessuna RDF, lista vuota o pregressa)
  → click "Nuova RDF"
  → View Nuova RDF
     - GET /rdf/deals           (eligibility)
     - GET /rdf/fornitori
  → seleziona deal, compila indirizzo+descrizione, opz. fornitori preferiti
  → click "Inserisci RDF"
     - POST /rdf/richieste { deal_id, codice_deal, indirizzo, descrizione, fornitori_preferiti }
       ├─ INSERT rdf_richieste RETURNING id
       ├─ (BE) notifier.SendCard( webhook, card(richiesta) )  // best-effort
       └─ 201 Created { richiesta }
  → toast "RDF creata"
  → navigate Consultazione RDF
```

### Journey 2 — Requestor consulta e visualizza
```
User (app_rdf_access)
  → Consultazione RDF (auto-load)
     - GET /rdf/richieste/summary?stato=nuova,in corso,completata
        ├─ (BE) join richieste + aggregates(fattibilita) + hubs_deal (su anisetta + db-mistra)
        └─ rows arricchite con deal_name, company_name
  → applica filtri, click "Aggiorna" → stessa GET con parametri nuovi
  → click "Visualizza" su una card
     - storeValue sostituito da path param: /richieste/:id/view
  → View Visualizza RDF (tab Riepilogo aperta di default)
     - GET /rdf/richieste/:id/full
     - GET /rdf/deals/:deal_id
  → click tab "Analisi"
     - POST /rdf/richieste/:id/analisi   (lazy)
        ├─ (BE) build prompt, call OpenRouter
        └─ 200 { text }
  → click tab "Azioni"
     - POST /rdf/richieste/:id/analisi?format=json    (lazy; oppure endpoint separato /analisi-json)
        └─ 200 { azioni_raccomandate: [...] }
  → click tab "PDF"
     - GET /rdf/richieste/:id/pdf           (lazy; binary application/pdf)
     - viewer embeds
```

### Journey 3 — Manager gestisce una RDF
```
User (app_rdf_manager)
  → Portal launch → Gestione RDF Carrier (default per manager)
     - GET /rdf/richieste/summary?...    (stesso endpoint, ruolo elevato)
  → click "Gestisci" su una card
     - navigate /richieste/:id
  → View Dettaglio RDF Carrier (onLoad):
     - GET /rdf/richieste/:id
     - GET /rdf/richieste/:id/fattibilita
     - GET /rdf/fornitori
     - GET /rdf/tecnologie
     - GET /rdf/deals/:deal_id
  → cambia stato via slider
     - PATCH /rdf/richieste/:id/stato { stato: "in corso" }
     - toast
  → apre modal "Nuova Fattibilità Fornitore"
     - seleziona tecnologia + N fornitori
     - click Genera
        - POST /rdf/richieste/:id/fattibilita { tecnologia_id, fornitore_ids: [...] }
           ├─ (BE) INSERT N righe stato=bozza
           └─ 201 { rows: [...] }
        - close modal
        - GET /rdf/richieste/:id/fattibilita   (refresh tabella)
  → seleziona una riga in tabella → popola form edit
  → modifica campi → click Aggiorna
     - PATCH /rdf/fattibilita/:id { ...17 campi }
        ├─ (BE) load pre, UPDATE, compare
        ├─ if changed in {stato, copertura, nrc, mrc}: notifier.SendText(diff)
        └─ 200 { fattibilita }
     - toast "Dati aggiornati"
     - GET /rdf/richieste/:id/fattibilita
```

### Journey 4 — Link diretto (deep link)
```
Email / Teams card contiene link https://portal/rdf/richieste/42/view
  → auth Keycloak se necessario
  → atterra su Visualizza RDF id=42 senza passare da Consultazione
```
Questo richiede che le view siano route-level con path param; nessun bisogno del vecchio `appsmith.store.v_id_richiesta_ro`.

## D.4 Hidden triggers / automazioni

- **Nessun cron/timer** nell'app attuale. Nessun workflow server-side programmato.
- **Nessun webhook ingresso**: Teams manda in uscita, nessuno entra.
- **Trigger impliciti solo UI**: `onLoad` di pagina (rimpiazzati da queries React-Query `enabled: true` all'ingresso view), `onChange` di slider stato (→ PATCH immediata), `onClick` bottoni.
- **`analisi` oggi nell'onLoad** di Visualizza era un trigger automatico (bacato). Nuovo comportamento: trigger esplicito al click tab. Se in futuro vogliamo pre-generazione batch (tipo "genera analisi per tutte le RDF completate ogni notte"), è una feature nuova fuori scope.

## D.5 Data ownership boundaries

| Dato | Owner | Chi può scrivere | Chi può leggere |
|---|---|---|---|
| `rdf_richieste` | questo app + Appsmith (coesistenza) | BE via user `app_rdf_access` (create) o `app_rdf_manager` (update) | qualsiasi authenticated `app_rdf_access` |
| `rdf_fattibilita_fornitori` | questo app + Appsmith | BE via `app_rdf_manager` | `app_rdf_access` |
| `rdf_fornitori`, `rdf_tecnologie` | **out-of-band** (DBA / seed) | nessuna UI | tutti authenticated |
| `loader.hubs_*` | pipeline HubSpot ingestion (mistra) | nessuno da qui | BE read-only |
| webhook Teams URL | secret | operations (env) | runtime BE |
| OpenRouter API key | secret | operations (env) | runtime BE |

## D.6 Integration contract con il portal

Seguendo il checklist "New App Checklist" (CLAUDE.md):
- `package.json` root → aggiungere `dev:richieste-fattibilita` + entry concurrently.
- `Makefile` → `dev-richieste-fattibilita` target.
- `backend/internal/platform/applaunch/catalog.go` → app id `rdf`, href, ruoli `app_rdf_access`/`app_rdf_manager`, catalog entry.
- `backend/cmd/server/main.go` → import, hrefOverrides (dev port), filtro catalog, RegisterRoutes.
- `backend/internal/platform/config/config.go` → `RichiesteFattibilitaAppURL` + env var.
- Backend mount: tutte le route RDF sotto `/api/rdf/*`.
- Vite proxy `/api` → `localhost:8080` già esistente.
- Serve SPA via backend in prod (contratto documentato in `docs/deployment`).

## D.7 Tracking: cosa resta da verificare fuori spec

1. **DDL di `rdf_*`** — validare tipi effettivi di `nrc`, `mrc`, `durata_mesi`, `giorni_rilascio`, `copertura`, `fornitori_preferiti`, `data_richiesta`. Non cambiamo lo schema, ma il BE deve matchare i tipi PG.
2. **Utenti/ruoli Keycloak esistenti** — chi oggi ha `straFatti Full` e `Administrator - Sambuca`? Serve mapping aggiornato + aggiunta `app_rdf_access` ai requestor.
3. **Webhook Teams** — confermare che l'URL attuale sia gestito da operations e riutilizzabile dal backend Go senza rewrite. Payload identico (Adaptive Card + testo).
4. **Prompt LLM completi** — recuperare `utils.system_prompt` e `utils.system_prompt3` dal body completo del JSObject `utils` di `Visualizza RDF` (audit ha troncato a ~3000 chars).
5. **`jsGChat1` module** — recuperare implementazione `sendTextMessage` / `sendCardMessage` per verificare payload Teams esatto (signature/headers/shape).
6. **Disattivazione notifica Appsmith** quando il nuovo app va live, per evitare doppia notifica durante coesistenza.

---

Nessuna domanda aperta in D — tutte le integrazioni sono già decise dalle fasi precedenti. Le 6 verifiche di D.7 sono task operativi, non decisioni di design.

Passo a Phase E (Specification Assembly) se confermi.

# Phase C — Logic Placement

Vincolo: coesistenza con l'app Appsmith. Il backend Go è un **thin wrapper** sulle query attuali (stesso DB `anisetta`, stessa semantica). Spostiamo lato backend solo ciò che *deve* andarci per correttezza/sicurezza; il resto può restare client-side se è già presentazione o orchestrazione UI.

## C.1 Classification & placement table

Legenda: **D** = domain / business, **O** = orchestration, **P** = presentation. Placement: **BE** = backend Go, **FE** = frontend React, **SH** = shared (es. tipi generati da OpenAPI), **DROP** = non portato.

### JSObject methods (utils / jsGChat1)

| Item | Sorgente (current) | Classe | Placement | Note |
|---|---|---|---|---|
| `utils.nuovaRDF` | Nuova RDF | O | **FE** | Flusso UI: submit form → POST → navigate. Orchestrazione di rete. |
| `utils.notificaChat` (create) | Nuova RDF | D+O | **BE** | La notifica Teams è effetto di dominio della create. Deve accadere server-side, nella stessa transazione logica di `POST /rdf/richieste`. Webhook URL → env var. Card payload costruita lato BE. |
| `utils.creaRecordFattForn` | Dettaglio | O | **FE** | Loop di N `POST` (uno per fornitore). Oppure collassato in un unico `POST /rdf/richieste/:id/fattibilita` batch-aware (preferibile — vedi C.2). |
| `utils.aggiornaRecordFattForn` | Dettaglio | O | **FE** | Submit del form → `PATCH`. Orchestrazione UI. |
| `utils.NotificaChat` (update) | Dettaglio | **D** | **BE** | Regola "notify se cambiano stato/copertura/nrc/mrc" è **business logic**. Deve essere nel service `PATCH /rdf/fattibilita/:id` (diff pre/post, emit notifica). NON delegarla al client: un client bacato potrebbe saltarla. |
| `utils.fornitoriPreferiti` | Dettaglio | P | **FE** | Formatta stringa "Preferenza fornitori: X, Y, Z" da array + tabella fornitori. Pura presentazione. |
| `utils.stringaArray` | Dettaglio | D | **BE** | Parse dell'array literal Postgres `{1,2,3}` → int[]. Va col parsing della row: il BE deve ritornare `fornitori_preferiti` già come `int[]` nel JSON, il FE non deve conoscere il formato PG. |
| `utils.stato_ff` | Consultazione | P | **FE** | Formatta "Bozza: X Inv: Y …" dai counter. Pura presentazione. |
| `utils.IsManager` | Consultazione | D | **SH** | Il check di ruolo esiste **sia** backend (enforce) sia frontend (UI gate). BE source of truth; FE legge `user.roles`/`scopes` dal token. |
| `utils.mergeDati` | Consultazione | O | **BE** | Il join `richieste + deals` diventa una query server-side in `GET /rdf/richieste/summary`. FE riceve righe già arricchite. |
| `utils.aggiornaDati` | Consultazione | O | **FE** | Si riduce a: call `GET /rdf/richieste/summary?stato&deal&richiedente&cliente`. |
| `utils.generate2` (PDF) | Visualizza | D+P | **BE** | Layout e contenuto del PDF sono oggi 140+ righe jspdf lato client. Spostati su BE come renderer (jspdf server-side in Go non è diretto; useremo una lib Go tipo `gofpdf` o HTML→PDF). Stesso contenuto, stesso ordine sezioni. Endpoint `GET /rdf/richieste/:id/pdf`. |
| `utils.formatDate` | Visualizza | P | **FE** | Helper triviale (Intl). |
| `utils.analisi` | Visualizza | D | **BE** | Costruisce il prompt LLM con `system_prompt` + user data. Modello pinnato, API key server-side. Endpoint `POST /rdf/richieste/:id/analisi` (o GET con cache). |
| `utils.analisi_json` | Visualizza | D | **BE** | Stesso ragionamento; ritorna `{ azioni_raccomandate: [...] }`. |
| `utils.chatWebhook` | Nuova + Dettaglio | D (secret) | **BE** | Mai nel bundle client. Env var `RDF_TEAMS_WEBHOOK_URL`. |
| `utils.system_prompt*` | Visualizza | D | **BE** | Prompt di sistema sono logica di dominio — configurabili via env/config server-side. |
| `utils.score_budget` | Visualizza | P (label map) | **FE** | Array fisso di 5 label; costante frontend. |
| `jsGChat1.sendTextMessage` | Nuova + Dettaglio | D | **BE** | Wrapper HTTP sul webhook Teams. Lato BE diventa `notifier.SendText(ctx, text)`. |
| `jsGChat1.sendCardMessage` | Nuova + Dettaglio | D | **BE** | Come sopra per Adaptive Card. |

### SQL actions (tutte → BE)

| Action | Placement | Endpoint proposto |
|---|---|---|
| `get_deals` (eligibility) | **BE** | `GET /rdf/deals?q=...` (pipeline/stage rule hard-coded server-side) |
| `get_deals` (parametric) | **BE** | Stesso endpoint + `?cliente=` |
| `get_deal_by_id` | **BE** | `GET /rdf/deals/:id` |
| `get_fornitori` | **BE** | `GET /rdf/fornitori` |
| `get_tecnologie` | **BE** | `GET /rdf/tecnologie` |
| `get_richiesta_by_id` | **BE** | `GET /rdf/richieste/:id` |
| `get_richiesta_full_by_id` | **BE** | `GET /rdf/richieste/:id/full` |
| `get_richieste` (Gestione) | **BE** | `GET /rdf/richieste?stato&deal&richiedente` |
| `get_richieste` (Consultazione counters) | **BE** | `GET /rdf/richieste/summary` (include join deals) |
| `get_fatt_fornitore` | **BE** | `GET /rdf/richieste/:id/fattibilita` |
| `get_fatt_for_by_id` | **BE** | `GET /rdf/fattibilita/:id` |
| `ins_richiesta` | **BE** | `POST /rdf/richieste` — ritorna record con `id` |
| `ins_fatt_fornitori` | **BE** | `POST /rdf/richieste/:id/fattibilita` (batch, accetta array di `{fornitore_id, tecnologia_id}` con `stato=bozza`) |
| `upd_fatt_fornitori` | **BE** | `PATCH /rdf/fattibilita/:id` |
| `upd_stato_richiesta` | **BE** | `PATCH /rdf/richieste/:id/stato` |
| `ai_openrouter` | **BE** | Mai esposto. Usato internamente da `/analisi`. |

### Inline widget expressions

| Espressione | Classe | Placement | Note |
|---|---|---|---|
| `ms_stato.selectedOptionValues.map(v => "'"+v+"'").join(', ')` interpolato in SQL | D (**bug: injection**) | **BE** | Passato come array, parametrizzato. **Non portato così.** |
| `ILIKE '%'+i_deal.text+'%'` | idem | **BE** | Parametrizzato. |
| `tbl_fattib_forn.selectedRow.xxx` → defaultText | P | **FE** | Binding di form su riga selezionata. |
| `rg_copertura.selectedOptionValue == 1 ? 'SI' : ''` | P | **FE** | Formatter presentazionale. |
| `moment(...).format('DD/MM/YY')` | P | **FE** | Formatter presentazionale. |
| `{{utils.IsManager()}}` su IconButton visibility | D (UI gate) | **FE** | Ma decisione arriva dai claim token. |
| `storeValue(...)` + `navigateTo(...)` | O | **FE** | Rimpiazzato da router (`useNavigate`). |

## C.2 Regole di business da consolidare (BE source of truth)

Queste regole oggi sono sparse. Tutte vanno nel service layer backend; il FE non le riproduce.

1. **Eleggibilità deal per RDF**: `(pipeline='255768766' AND stage.display_order BETWEEN 1 AND 5) OR (pipeline='255768768' AND stage.display_order BETWEEN 3 AND 8) AND codice <> ''`. Costanti in un package `rdf/domain/deals.go`.
2. **Stato taxonomy richiesta**: enum `nuova | in corso | completata | annullata`. Validato in `PATCH /rdf/richieste/:id/stato`.
3. **Stato taxonomy fattibilità**: enum `bozza | inviata | sollecitata | completata | annullata`. Validato in `PATCH /rdf/fattibilita/:id`.
4. **Create-richiesta side-effect**: dopo INSERT, notifica Teams con Adaptive Card (stesso payload attuale). Falla nella stessa handler transaction; se la notifica fallisce, log-and-continue (best-effort, non bloccante).
5. **Update-fattibilità diff-notify**: se cambia `stato | copertura | nrc | mrc` (set preciso come oggi), invia text message Teams con formato identico a `NotificaChat`. Calcolato server-side comparando pre/post. Altri campi modificati → nessuna notifica.
6. **Creazione batch fattibilità**: dato `{tecnologia_id, fornitori_ids[]}`, crea N righe in stato `bozza`. Operazione idempotente per `(richiesta_id, fornitore_id, tecnologia_id)`? **Oggi non lo è** (doppio click crea doppio record). Proposta: mantenere comportamento attuale (no idempotency key) per coesistenza; il FE debounce il bottone.
7. **Manager gate**: endpoint di scrittura su `rdf_richieste` (upd_stato, update carrier-notes se mai implementato) e tutto `rdf_fattibilita_fornitori` richiedono ruolo `app_rdf_manager`. `POST /rdf/richieste` (nuova) richiede solo `app_rdf_access`.
8. **Ownership**: un requestor vede in Consultazione solo le proprie RDF? **No, oggi vede tutto** (filtro solo per `created_by ILIKE`). Conservare: nessuna ownership enforcement server-side.

## C.3 Duplicazione da consolidare

| Duplicato oggi | Azione |
|---|---|
| `get_deals` definita 3× | Un solo handler, due varianti via query param. |
| `get_fornitori` / `get_tecnologie` / `get_fatt_fornitore` / `get_richiesta_full_by_id` duplicate | Un solo endpoint per risorsa. |
| `stringaArray`, `fornitoriPreferiti`, `creaRecordFattForn`, ecc. duplicate tra action e JSObject | Una sola implementazione (service BE o helper FE, come da C.1). |
| `chatWebhook` incollato in due JSObject | Una env var server-side. |

## C.4 Regole modificate rispetto al comportamento attuale (cleanup esplicito)

Tutte concordate nelle Phase A/B; le richiamo qui per tracciabilità:

- Doppia notifica Teams su create → una sola.
- `get_richiesta_by_id.run()` dentro `creaRecordFattForn` (trick refresh Appsmith) → rimosso.
- `ai_openrouter` in `onLoad` di Visualizza → rimosso, sostituito da lazy load al click tab.
- `Input2 "Note Carrier Relations"` dead widget → non portato.
- Tab2 invisibile su Dettaglio → non portato.
- `get_richiesta_full_by_id where rr.id = 3` su Consultazione → dead code, non portato.
- `get_fatt_for_by_id where id = 0` su Visualizza → dead code, non portato.
- Fallback SQL `|| 3` / `|| 0` → non riprodotti; id mancante → 404.

## C.5 Outcome architetturale

- **Backend Go (`backend/internal/rdf/`)**: handlers HTTP + service layer + repo layer (pgx verso `anisetta` e verso `db-mistra` read-only). Notifier separato. LLM proxy separato. PDF renderer separato.
- **Frontend React (`apps/richieste-fattibilita/`)**: orchestrazione UI, binding form, formatter presentazionali. Nessuna SQL, nessuna API key, nessuna regola di dominio.
- **Shared**: tipi generati da OpenAPI (o `go-ts-gen`) per DTO comuni.

---

Nessuna domanda aperta in Phase C — tutte le scelte di placement seguono meccanicamente dai vincoli già confermati. Procedo con Phase D se confermi.

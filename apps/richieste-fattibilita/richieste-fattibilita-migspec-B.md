# Phase B — UX Pattern Map

Vincolo: massima coesistenza con l'app Appsmith. Le view del nuovo frontend devono coprire gli stessi casi d'uso, con lo stesso set di campi e la stessa navigazione. Libertà solo su componenti/look (allineamento a `docs/UI-UX.md` del portal).

## B.1 View catalog (current → target)

### V1 — Home
- **Pattern:** static landing.
- **Intent:** nessuno (placeholder).
- **Target:** **droppata.** L'entrypoint diventa la launch card del portal (Matrix). Dopo il click si atterra direttamente su Consultazione RDF (view di default per il requestor) o Gestione RDF (per il carrier/manager). Vedi §B.3.

### V2 — Nuova RDF
- **Pattern:** Master-detail creation (**deal picker + form**).
- **Intent:** un requestor crea una nuova richiesta di fattibilità scegliendo il deal HubSpot di riferimento.
- **Sezioni logiche:**
  1. *Deal picker* — tabella di deal eleggibili (filtrabile/ordinabile). Select-one obbligatoria.
  2. *Form richiesta* — campi: `indirizzo` (obbligatorio), `descrizione` (obbligatorio), `fornitori_preferiti` (multi-select opzionale).
  3. *Azioni* — "Inserisci RDF" (primaria, disabilitata finché deal non selezionato e campi obbligatori vuoti), "Reset".
- **Post-save:** toast + navigate a Consultazione RDF. Una notifica Teams (card Adaptive) con riepilogo + link alla RDF creata.
- **Note su current vs intended:**
  - Current: doppia notifica Teams per bug di binding. Intended: una sola.
  - Current: nessuna validazione lato form. Intended: tenere il comportamento attuale (no hard-validation) o aggiungere required visuale? → **proposta:** visual required su `indirizzo` e `descrizione`, nessun cambio di contratto backend. Conferma?
  - Deal picker: attualmente mostra ~300 deal con filtro client-side della tabella. Teniamo la paginazione/ricerca server-side? → **proposta:** cap a 300 come oggi + ricerca testuale lato server (stesso filtro per `codice`/`company_name`) per evitare di portare solo il filtro client-side di Appsmith.

### V3 — Gestione RDF Carrier
- **Pattern:** Filtered list + drill-down (**card list**).
- **Intent:** il team carrier sorveglia le RDF aperte e ne apre una per lavorarla.
- **Sezioni logiche:**
  1. *Filter bar:* `ms_stato` (multi, default `["nuova","in corso"]`), `i_deal` (codice), `i_richiedente` (email).
  2. *Lista card:* una card per RDF con codice deal, id + data + stato, indirizzo, descrizione. Pulsante "Gestisci" → Dettaglio.
- **Target:** identica in contenuto. Scheletro card allineato al design system portal.
- **Note:** oggi non mostra nome cliente / deal_name perché non joina. Intended: **aggiungere** `company_name` e `deal_name` risolvendo il join server-side (stessa logica di Consultazione). Questo è un visual upgrade senza impatto sui dati; mantiene compatibilità dato che i campi sono in HubSpot comunque. Conferma?

### V4 — Dettaglio RDF Carrier
- **Pattern:** Aggregate editor (**header + child-list + child-editor + add-modal**).
- **Intent:** il carrier modifica lo stato globale della RDF e gestisce le righe di fattibilità per singolo fornitore.
- **Sezioni logiche:**
  1. *Header RDF (read-mostly):* indirizzo, descrizione, creato il/da, riepilogo deal (codice/stage/cliente/owner), testo "preferenza fornitori".
  2. *Stato RDF:* slider categorico (`nuova / in corso / completata / annullata`) — persist on change, toast.
  3. *Tabella fattibilità fornitori* (child list) — seleziona una riga per editarla sotto.
  4. *Form di edit fattibilità* (child editor) — 17 campi; bottoni "Aggiorna" (disabilitato se nessuna selezione) e "Reset".
  5. *Modal "Nuova Fattibilità Fornitore"* — multi-select fornitori + select tecnologia → crea N righe in bozza.
- **Azioni esterne:** notifica Teams su update se cambia `stato | copertura | nrc | mrc`.
- **Target:** invariata. Il dead widget `Input2 "Note Carrier Relations"` e il dead `Tab 2` non vengono portati. Il `Button2 Reset` del form di edit viene collegato (reset a `tbl_fattib_forn.selectedRow` corrente).
- **Note:** valutare se l'header va in uno `<details>`/collapsible (info tutte visibili ora, ma 3 blocchi testuali occupano molto). **Proposta:** tenere visibile; allineare spacing al design system. Conferma?

### V5 — Consultazione RDF
- **Pattern:** Filtered list + counters + role-gated quick-action (**card list con badge di progresso**).
- **Intent:** il requestor (e il manager) vede tutte le proprie RDF con progresso fattibilità e apre la view read-only; il manager ha shortcut "Gestisci" per andare in edit.
- **Sezioni logiche:**
  1. *Filter bar:* `ms_stato` (default `["nuova","in corso","completata"]`), `i_deal`, `i_richiedente`, `i_cliente` (company).
  2. *Lista card:* codice deal, id+data+stato, cliente/indirizzo, descrizione, riepilogo fattibilità `Bozza: X Inv: Y Soll: Z Compl: W Ann: V`, bottone "Visualizza" (tutti), icon-button "Gestisci" (solo manager).
- **Target:** invariata. Nota: il merge `richieste + deals` avviene oggi client-side (`utils.mergeDati`). Nel nuovo stack si **porta lato backend** un endpoint `GET /rdf/richieste/summary` che restituisce già le righe arricchite con `deal_name`/`company_name`. Stessa regola "se filtro cliente è attivo, nascondi RDF senza match deal".
- **Note:** non porta onLoad (oggi richiede "Aggiorna" manuale). → **proposta:** auto-load all'ingresso view (prima query con i default di `ms_stato`), bottone Aggiorna rimane per refresh. Conferma?

### V6 — Visualizza RDF
- **Pattern:** Read-only viewer a tab (**tabs: Riepilogo / Analisi AI / PDF / Azioni consigliate**).
- **Intent:** il requestor (e manager) vede una RDF consolidata con analisi LLM e scarica il PDF.
- **Sezioni logiche:**
  1. *Riepilogo tab:* header RDF + tabella fattibilità + pannello di 10 campi read-only sulla riga selezionata + rating/copertura/score budget.
  2. *Analisi tab:* testo libero generato da `analisi` (LLM).
  3. *PDF tab:* viewer di un PDF renderizzato server-side.
  4. *Azioni tab:* tabella `azioni_raccomandate` strutturata da `analisi_json`.
- **Target:** invariata nella struttura a tab. **Cambio implementativo (non-semantico):** la generazione PDF passa lato backend (stesso layout/contenuto, non più `jspdf` nel browser) — serve un endpoint `GET /rdf/richieste/:id/pdf`. Le chiamate LLM passano da un **proxy backend** (mai più API key lato client); stessi prompt, stessi modelli pinnati.
- **Note:** oggi `ai_openrouter` è in `onLoad` con `request` vuoto → errore silenzioso. Intended: rimuovere dall'onLoad, le orchestration `analisi` / `analisi_json` vengono triggerate esplicitamente quando si apre il tab relativo (lazy tab load). Conferma?

## B.2 Pattern classification (summary)

| View | Pattern | Complessità |
|---|---|---|
| Home | static | trivial (droppata) |
| Nuova RDF | master-detail create | medium |
| Gestione RDF Carrier | filtered list → detail | low |
| Dettaglio RDF Carrier | aggregate editor + modal | **high** (più complessa) |
| Consultazione RDF | filtered list w/ counters | medium |
| Visualizza RDF | tabbed read-only viewer | medium |

## B.3 Navigation map (inter-view)

```
[Portal] ──(launch card)──► Consultazione RDF (default, tutti)
                            │
                            ├──(Visualizza)──► Visualizza RDF   (store: v_id_richiesta_ro)
                            │
                            └──(Gestisci, manager only)──► Dettaglio RDF Carrier
                                                              │
                                                              └──(◄ back)
Gestione RDF Carrier ──(Gestisci)──► Dettaglio RDF Carrier    (store: v_id_richiesta)
Nuova RDF ──(Inserisci RDF)──► Consultazione RDF
```

- I due store-key Appsmith (`v_id_richiesta`, `v_id_richiesta_ro`) diventano **path param** nell'URL della nuova app (`/richieste/:id`, `/richieste/:id/view`). Niente state globale.
- Deep-link/bookmark: supportati per default (stato della view ricavato dall'URL).

## B.4 Merge / split / rename decisions

- **Merge?** Gestione e Consultazione condividono filter bar e entità. Tecnicamente mergeabili in una view unica con toggle "modalità carrier/requestor" gated dal ruolo. → **proposta:** tenerle separate come oggi per coesistenza, ma il componente lista è lo stesso (share del codice). Conferma.
- **Split?** No — Dettaglio è già vicino al limite di complessità; non beneficerebbe di split in questa fase.
- **Rename?** No. Conserviamo i nomi italiani ("Nuova RDF", "Consultazione RDF", ecc.) per continuità con l'originale.

## B.5 Decisioni chiuse (2026-04-15)

1. **Required visuale** su `indirizzo` / `descrizione` in Nuova RDF: **sì.** Asterisco sul label, submit disabilitato se vuoti; nessun cambio contratto/DB.
2. **Arricchimento card Gestione** con `company_name` + `deal_name`: **sì.** Riuso dell'endpoint summary che stiamo comunque creando per Consultazione — effort trascurabile.
3. **Consultazione auto-load** all'ingresso: **sì.** Bottone "Aggiorna" resta come refresh esplicito.
4. **Visualizza lazy-load LLM**: **sì.** `analisi` parte al click del tab Analisi; `analisi_json` al click del tab Azioni; PDF al click del tab PDF.

### Modello ruoli (confermato)

- `app_rdf_access` — accesso base: Consultazione RDF, Nuova RDF, Visualizza RDF.
- `app_rdf_manager` — aggiunge Gestione RDF Carrier, Dettaglio RDF Carrier.
- Mapping legacy per coesistenza: chi oggi ha i ruoli Keycloak `straFatti Full` o `Administrator - Sambuca` viene trattato come `app_rdf_manager`.
- Enforcement **reale backend** (non solo nav): tutti gli endpoint di scrittura su `rdf_richieste` / `rdf_fattibilita_fornitori` richiedono `app_rdf_manager`. `Nuova RDF` (insert) rimane `app_rdf_access` perché è il requestor a creare.

### Navigazione risultante

```
app_rdf_access:
  Portal launch → Consultazione (default)
  - Nuova RDF
  - Visualizza RDF (readonly)

app_rdf_manager (eredita access):
  Portal launch → Gestione (default)  // oppure switcher
  - Gestione RDF Carrier
  - Dettaglio RDF Carrier (edit)
  - tutto quanto sopra
```

**Default landing** per manager: Gestione. Per requestor: Consultazione. Il portal può esporre due launch-card diverse filtrate per ruolo, oppure una card unica che atterra dinamicamente. → **proposta:** una launch-card unica "RDF — Richieste Fattibilità"; il redirect post-launch sceglie la default view in base al ruolo. Default deciso, non più aperto.

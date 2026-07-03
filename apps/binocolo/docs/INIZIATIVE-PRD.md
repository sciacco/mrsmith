# Binocolo — PRD Iniziative e lavorazione target (workstream D3)

> Draft progressivo — **iterazione 2 (2026-07-02)**. Esito del brainstorming 2026-07-02: i punti marcati **[DECISO]** sono ratificati da Salvatore in quella sede; **[PROPOSTA]** = mia, da validare; **[APERTO]** = da sciogliere in un'iterazione successiva. Contesto: D1/D2 gated implementate (`GATED-UX-PRD.md`), proiezione target in corso (`TARGETS-PROJECTION-PLAN.md`). Iterazione 2: esiti di chiusura a 5 valori e confine rimozione/chiusura (§4.3–4.4), su domande di Salvatore.

## 1. Inquadramento

Il flusso gated si ferma al setaccio: D2 produce una shortlist che l'analista tria con le stelle (-1/0/1–3). Il lavoro che segue la promozione — contatti, dialoghi, offerte, su un orizzonte di settimane — oggi non ha casa: tre stelle e un log esiti a tre eventi non sono un metodo di lavorazione.

Fondamenta ratificate:

- **La sessione non è il contesto di acquisizione [DECISO]**: è *un'esecuzione di ricerca*. Una caccia reale (es. "acquisire un MSP") attraversa più sessioni via via raffinate; una lavorazione per-sessione si frammenterebbe. Serve un contenitore sopra la sessione.
- **La valutazione è context-scoped, non company-scoped [DECISO]**: la stessa azienda può essere 3★ nell'iniziativa MSP e scarto nell'iniziativa AI — entrambi giudizi corretti. Il modello dati già concorda: `ma_target_rating` ha PK `(session_id, company_key)`.
- **Le collisioni sono strutturali [DECISO]**: MSP e consulenza AI vivono nella stessa divisione ATECO 62; la stessa azienda in due iniziative è un caso di prima classe, non un caso limite. Il contatto fisico però è unico (un solo titolare): il coordinamento cross-iniziativa è un requisito, non un extra.
- **Due mestieri, due superfici**: D2 resta il **setaccio** (tabella veloce, 1000 righe leggere, gesto = stellina); la lavorazione è il **banco di lavoro** (poche decine di card, ricche: stati, diario, approfondimento).

## 2. Modello concettuale **[DECISO]**

```
Iniziativa ("Acquisizione MSP 2026")
  ├── Sessioni di ricerca (n, agganciate; raffinamenti successivi)
  └── Card di lavorazione (iniziativa, azienda) ← create dalle ≥1★
```

Due **piani di fatti**, con superfici di scrittura separate (ratificato dopo obiezione — niente toggle di scope):

| Piano | Fatti | Dove si scrive | Storage |
|---|---|---|---|
| **Lavoro** (contesto iniziativa) | fit, stelle, stato del deal, diario | solo dalla card | log unico eventi (§5) |
| **Entità** (azienda, globale) | fatti tipizzati, note di caratterizzazione, dominio, deep-dive | solo dal **MA card-dossier** (sezione registro) | registro azienda (§6) + registri esistenti |

La nota di diario ("richiamare lunedì") e la nota d'azienda ("fondatore 70enne, figli fuori dal business") sono **generi diversi, non scope diversi**: nessun meccanismo le trasforma l'una nell'altra.

## 3. Iniziativa

### 3.1 Entità **[DECISO: nome "Iniziativa"]**
Contenitore light: titolo, descrizione breve opzionale, autore, archiviabile. Niente budget, KPI, scadenze (vincolo non-CRM, §10). Rotta `/iniziative`, tabella `binocolo.ma_initiative`.

### 3.2 Aggancio sessioni **[DECISO]**
- In D1 (`/ricerche/nuova`): **campo opzionale** — dropdown iniziative attive + "Nuova iniziativa" inline. Nessun obbligo: le ricerche esplorative restano a zero attrito.
- **Retro-aggancio** e spostamento dall'indice `/ricerche` per le sessioni esistenti/orfane.
- Sessione senza iniziativa = legittima; semplicemente non alimenta nessuna lavorazione.
- **[DECISO 2026-07-02]** Backfill al retro-aggancio: agganciare una sessione che ha già ≥1★ crea le card mancanti (stessa regola d'ingresso di §4.1, applicata retroattivamente).

## 4. Card di lavorazione

### 4.1 Ingresso **[DECISO]**
La **prima ≥1★** su un'azienda in una sessione agganciata crea la card, stato iniziale "Da contattare". La provenienza è registrata sulla card (sessione, stella, score al momento — già fotografato da `score_at_rating`, mig 080).

### 4.2 Autonomia **[DECISO]**
Dopo il **primo evento a diario o avanzamento di stato**, la stella non governa più la card: resta come provenienza. Aziende ripescate da sessioni successive della stessa iniziativa aggiungono provenienze alla card esistente — **nessuna aggregazione delle stelle** (un numero senza semantica): la card mostra le provenienze, lo stato operativo è l'unica verità corrente.

### 4.3 Uscita e riapertura
- **[DECISO]** La card esce solo per **stato terminale (Chiusa)** o **rimozione esplicita**. Togliere una stella non cancella mai un diario avviato.
- **Confine rimozione/chiusura [DECISO 2026-07-02]**: la **rimozione** è per l'errore di triage — la stella era sbagliata, la card non doveva esistere, nessun verdetto. La **chiusura** è il verdetto di una lavorazione avvenuta. Il flusso di rimozione offre (opzionale) la correzione della stella nella sessione di provenienza (-1 con motivo, scrive `ma_target_rating` come oggi): anche l'errore diventa ground truth coerente.
- **[DECISO 2026-07-02]** Riapertura: la chiave è unica `(iniziativa, azienda)` — una nuova ≥1★ su card rimossa/chiusa **riapre la stessa card** (diario continuo, evento `riaperta`), non ne crea una seconda. È anche il meccanismo di ripresa organica delle `rimandata` (§4.4): nessun reminder, la ripresa avviene quando una ricerca la ripesca o l'operatore la riapre a mano.

### 4.4 Stati **[DECISO: set a 6, etichetta UI "Stato" — ratifica 2026-07-02]**
```
Da contattare → Contattata → In dialogo → Approfondimento → Offerta → Chiusa
```
- L'**esito è un attributo della chiusura**, non colonne separate. **[DECISO 2026-07-02]** set chiuso a 5 valori, ciascuno con semantica di calibrazione propria (il log esiti è la ground truth dello score, mig 080):

  | Esito | Significato | Segnale per lo score |
  |---|---|---|
  | `conclusa` | deal fatto | positivo massimo |
  | `no_go` | idonea, ma si sceglie di non procedere | neutro |
  | `non_idonea` | alla prova dei fatti non era in profilo | negativo: errore di screening misurato |
  | `sfumata` | decisione della controparte o di terzi | positivo: target buono, non vinto |
  | `rimandata` | condizioni non mature, da riprendere | sospeso |

  Note di disegno: `rimandata` è un **esito, non uno stato del board** (niente colonna "Parcheggiate": lavoro finto perennemente in vista) e non ha reminder (non-CRM; ripresa organica via riapertura §4.3). `non_idonea` è **context-scoped per natura** (fuori profilo per QUESTA iniziativa; può essere valida altrove) e quindi NON propone mai scritture di registro.
- **[DECISO 2026-07-02]** Transizioni libere (drag sul kanban, nessun vincolo di sequenza): il processo reale salta stati — una chiusura può arrivare da "Da contattare".
- Le chiusure `no_go`/`rimandata` possono proporre la registrazione del fatto strutturale quando il motivo lo è (`non_vende`, `in_trattativa_altrui` — §6, ponte tipizzato).

### 4.5 Assegnatario **[DECISO: no in v1]**
L'autore (subject/email) è su ogni evento, pattern già esistente nel log. Un assegnatario per card si valuta quando volume o team lo richiedono.

## 5. Diario — log unico per evoluzione **[DECISO]**

`ma_target_outcome` (mig 080) **evolve** nel log unico degli eventi di lavorazione — un sistema solo, non due verità: il diario È la ground truth che validerà lo score (scopo dichiarato della 080).

- `initiative_id` nullable (nuovo ancoraggio primario dei nuovi eventi); `session_id` diventa **nullable** (resta come provenienza quando l'evento nasce in contesto sessione); FK sessione da `ON DELETE CASCADE` a **`SET NULL`** — la ground truth sopravvive alle sessioni.
- CHECK del vocabolario esteso (oggi solo `contattato`/`buon_lead`/`no_go`). **[PROPOSTA]** nuovi eventi: `card_creata`, `card_rimossa`, `card_riaperta`, `stato` (con from/to nel payload), `nota`, `chiusura` (con esito). I tre eventi storici restano validi; il dettaglio finale del vocabolario si fissa nel piano esecutivo.
- Nel log vivono **solo eventi di lavorazione**. Le note d'azienda NON sono eventi del log (§6): categorizzazione, non duplicazione.

## 6. Registro azienda **[DECISO]**

Tabella company-level dedicata (pattern `ma_company_domain`), il piano dell'entità:

- **Fatti tipizzati, set chiuso** **[DECISO, inclusi i positivi]**: `non_vende`, `in_trattativa_altrui`, `da_evitare` (badge warning) + `gia_cliente`, `partner` (badge informativi). Ogni fatto: nota, autore, data; **revocabile** da un intervento successivo (la storia si conserva).
- **Note libere di caratterizzazione** **[DECISO]**: append-only, autore+data — l'espressività che il set chiuso non dà. Sono **contenuto del registro**, non eventi del log.
- **Superfici di scrittura** **[DECISO]**: SOLO dal **MA card-dossier** (`/iniziative/:id/dossier/:companyKey`, sezione registro). La card mostra la **scheda azienda in sola lettura** (badge fatti, ultima nota) con «Gestione dal dossier ↗» che apre il card-dossier. Niente toggle. Il tool standalone `/azienda` è indipendente da MA e **non è una superficie di scrittura** del registro (migrato 2026-07-02, remediation S6).
- **Ponte tipizzato** **[DECISO]**: la chiusura `no_go` può proporre "registra anche: non vende / in trattativa con altri" — due scritture ben tipizzate (evento di chiusura nel diario + fatto nel registro), mai una nota che cambia natura.
- **Effetti = SOLO presentazione** **[DECISO]**: badge nelle tabelle risultati di ogni sessione (veicolo naturale: la proiezione `MATargetRow` del piano proiezione), marker sulla card, scheda nel dossier. **MAI** effetti su gate UC2, routing v3, scoring.
- **[DECISO 2026-07-02]** `gia_cliente` **dichiarato manualmente in v1**: la derivazione automatica dalla base clienti Mistra/Grappa (match P.IVA, mapping documentato in `docs/IMPLEMENTATION-KNOWLEDGE.md`) richiederebbe un'integrazione che aggiunge complessità non necessaria ora. Resta un'evoluzione possibile — il fatto è già nel set tipizzato, la derivazione futura non cambierebbe il modello.

### 6.1 Marker di collisione **[DECISO]**
Derivato dalle card (nessun dato nuovo): la card mostra "In lavorazione anche in: *Acquisizione AI*" quando esiste una card **attiva** per la stessa azienda in un'altra iniziativa. **[DECISO 2026-07-02]** lo stesso marker come badge nelle tabelle risultati D2 (informazione gratis, previene il doppio contatto già al triage).

## 7. Approfondimento come azione di card **[DECISO]**

Oggi il deep-dive ("Approfondisci preferiti (N)", TargetPage) accoda le **≥1★ della sessione** per l'analisi IT-full — cioè, in D3, esattamente **la popolazione delle card**: l'azione era già concettualmente di lavorazione. L'artefatto (`MADeepAnalysis`) è già company-keyed e in cache globale cross-sessione.

- Azione **per-card**: avvia l'analisi della singola azienda; se già analizzata, il brief è disponibile subito senza spesa (comportamento cache esistente). Serve la variante per-azienda dell'endpoint (oggi session-scoped); worker, cache e gate di spesa si riusano interi.
- Il brief (scorecard, valutazione, testo) si consulta dal drawer della card e dal **MA card-dossier** (`/iniziative/:id/dossier/:companyKey`), che il drawer apre con «Apri dossier» e che legge la cache deep company-keyed. Il tool standalone `/azienda` è indipendente da MA e **non è accoppiato** al flusso (ratifica D-B, 2026-07-02).
- **Collisione di nomi [DECISO che va sciolta]**: lo stato "Approfondimento" (fase del deal) e l'azione oggi chiamata "Approfondisci" significherebbero cose diverse nella stessa pagina. **[DECISO 2026-07-02]** azione e artefatto hanno nomi distinti: l'**azione è un verbo** — l'attivazione innesca due cose (recupero dati completi IT-full + analisi LLM) sotto un'unica intenzione — l'**artefatto è il "dossier"**. Ciclo di vita del bottone sulla card:
  `Avvia analisi completa` → `Analisi in corso…` → `Apri dossier`.
  Il caso cache (azienda già analizzata altrove) nasce direttamente in "Apri dossier", zero spesa: il comportamento esistente diventa visibile da solo. L'etichetta legacy "Approfondisci preferiti" muore con la TargetPage.
- Spesa: **mai € in UI utente** (vincolo ratificato); la conferma resta deliberata e senza cifre. **[PROPOSTA]** il meccanismo `acknowledgeCost` esistente resta come guardia interna; il copy utente parla di analisi completa, non di costo.
- **[APERTO]** Azione batch dal board (es. "tutte le card in Da contattare"): v2, dopo l'uso reale.

## 8. Superfici UI

- **Rotte [DECISO]**: `/iniziative` (indice) + `/iniziative/:id` (board). Dentro l'app binocolo, accesso col ruolo esistente.
- **Indice [PROPOSTA]**: card per iniziativa — titolo, conteggi per stato, ultima attività; CTA "Nuova iniziativa"; archivio separato. Empty state da `docs/UI-UX.md` §13.
- **Board [DECISO]**: vista **kanban** e vista **tabella** come viste indipendenti con filtri propri (coerente con la regola "viste dati indipendenti" del design system di progetto) — nessun master-detail forzato.
- **Colonne kanban flessibili [DECISO 2026-07-02]**: sei colonne fisse compresse nella viewport ridurrebbero le card a francobolli (obiezione di Salvatore). Meccanismo: larghezza fissa confortevole (~240px), board a **scorrimento orizzontale**, colonne **collassabili a rail** verticale (nome + conteggio); le colonne vuote e "Chiusa" nascono collassate; lo stato di collasso è ricordato per utente.
- **Drawer card [PROPOSTA]**: diario (timeline eventi + composer nota di diario), cambio di stato, provenienze (sessioni/stelle/score al momento), scheda azienda read-only (§6) con link dossier, marker collisione, azione dossier completo (§7).
- **D1**: dropdown opzionale iniziativa **[DECISO]**. **Indice `/ricerche`**: chip iniziativa sulle righe + filtro **[DECISO chip; filtro PROPOSTA]**.

## 9. Requisiti dati/API

| ID | Requisito | Stato |
|----|-----------|-------|
| R-D3-1 | Tabella `binocolo.ma_initiative` + `ma_session.initiative_id` nullable | DECISO (mig, prossima libera **087** — verificare all'atto del piano) |
| R-D3-2 | Tabella card, PK `(initiative_id, company_key)`, stato + esito + timestamps | DECISO |
| R-D3-3 | Evoluzione `ma_target_outcome`: initiative_id, session_id nullable, CASCADE→SET NULL, vocabolario esteso | DECISO (§5) |
| R-D3-4 | Registro azienda: fatti tipizzati + note libere, revoca con storia | DECISO (§6) |
| R-D3-5 | Hook rating→card (creazione/riapertura) + backfill al retro-aggancio | DECISO / backfill PROPOSTA |
| R-D3-6 | Deep-dive per-azienda (variante dell'endpoint session-scoped) | DECISO (§7) |
| R-D3-7 | Badge registro + marker collisione nella proiezione `MATargetRow` | PROPOSTA (dipende dal piano proiezione) |
| R-D3-8 | Endpoint iniziative (CRUD light), card (board, transizioni), diario, registro | DECISO (forma nel piano) |

## 10. Vincoli trasversali

- **Non-CRM [DECISO]**: append-only, nessuna email, task, reminder o scadenza in v1. Il registro è anagrafe, non relationship management.
- **Costi mai in UI utente** (ratificato di progetto).
- **Pipeline intoccata**: gate UC2, routing v3, scoring v3 non leggono né registro né card. Effetti solo di presentazione.
- Design system `docs/UI-UX.md` (clean theme), copy B2B asciutto, review finale col workflow `portal-miniapp-generator`.
- Multi-utente: autore su ogni evento/nota/fatto (pattern subject/email esistente).

## 11. Aperti per la prossima iterazione

1. ~~`gia_cliente` derivato vs dichiarato~~ → **deciso**, §6: dichiarato manualmente in v1 (derivazione Mistra = evoluzione possibile).
2. ~~Nome dell'azione di approfondimento~~ → **deciso**, §7: azione "Avvia analisi completa" / artefatto "dossier", ciclo a tre stati.
3. ~~Esiti di chiusura~~ → **deciso**, §4.4: set a 5. ~~Correzione-stella alla rimozione~~ → **deciso**, §4.3.
4. ~~Riapertura, backfill, transizioni libere, marker in D2~~ → **decisi** (2026-07-02, ratifica in blocco).
5. Vocabolario eventi definitivo del log unico (§5) — dettaglio da fissare nel piano esecutivo.
6. Batch deep-dive dal board — v2.
7. ~~Wireframe~~ → **approvato** (2026-07-02): [`iniziative-wireframe.html`](iniziative-wireframe.html), stati S1–S7 — fonte di verità per layout e copy dei task F.

> **Implementazione**: piano esecutivo in [`INIZIATIVE-IMPLEMENTATION-PLAN.md`](INIZIATIVE-IMPLEMENTATION-PLAN.md) — 11 task (B1–B6 backend con migrazioni 087–090, F1–F5 frontend), riferimenti verificati sul codice al 2026-07-02, pensati per esecuzione da parte di un LLM.

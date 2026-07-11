# PRD — Invio PO al fornitore con email personalizzata

**Feature principale:** GitHub #55  
**Template e copy:** GitHub #66  
**PRD:** GitHub #67  
**Stato:** pronto per revisione

## 1. Contesto

Oggi, da un PO in stato `PENDING_SEND`, l'utente può avviare l'invio al fornitore. MrSmith inoltra l'azione all'endpoint Arak `send-to-provider`; Arak genera il PDF, determina i destinatari, compone e invia l'email predefinita e chiude il PO.

Questo percorso è adeguato per molti utenti e deve rimanere disponibile. Non consente però di modificare la comunicazione o aggiungere all'email altri documenti già associati al PO.

La feature introduce quindi una modalità personalizzata, alternativa a quella standard, e la possibilità di effettuare nuovi invii email da un PO chiuso.

## 2. Obiettivi

- Conservare il flusso standard Arak.
- Consentire una scelta consapevole tra invio standard e personalizzato.
- Permettere di modificare lingua, destinatari, oggetto e testo del messaggio all'interno del template HTML fisso.
- Consentire l'inclusione di documenti già gestiti da Arak, oltre al PDF obbligatorio del PO.
- Chiudere coerentemente il PO senza generare una seconda email Arak.
- Consentire nuovi invii espliciti sui PO chiusi.
- Registrare ogni tentativo personalizzato nel ledger email di MrSmith.
- Restituire l'esito in modo sincrono e comprensibile.

## 3. Principi e vincoli

- La soluzione è additiva e non sostituisce il percorso standard.
- Le operazioni di invio sono sincrone.
- Non sono previste code, worker, bozze persistenti o retry automatici.
- Non è consentito caricare file arbitrari.
- Il contenuto completo dell'email e i file allegati non vengono archiviati localmente.
- I messaggi rivolti all'utente non espongono dettagli tecnici dell'infrastruttura email.
- L'interfaccia RDA segue `docs/UI-UX.md`; il template HTML segue `apps/rda/docs/EMAIL-STYLE.md` e le decisioni di #66.

## 4. Utenti e casi d'uso

Possono operare, secondo le regole Arak esistenti:

- il richiedente del PO;
- un approvatore associato al PO;
- un approvatore extra-budget.

Casi d'uso:

1. inviare rapidamente il PO con la modalità standard;
2. personalizzare la comunicazione prima del primo invio;
3. aggiungere un contatto del fornitore durante la composizione;
4. includere documenti del PO oltre al PDF obbligatorio;
5. riprovare immediatamente quando l'invio personalizzato non riesce;
6. effettuare un nuovo invio esplicito da un PO chiuso;
7. vedere quanti invii personalizzati sono stati accettati e quando è avvenuto l'ultimo.

## 5. Scope funzionale

### 5.1 Modale iniziale su `PENDING_SEND`

Il pulsante esistente **Invia ordine al fornitore** apre una modale, senza avviare immediatamente l'operazione.

La modale mostra:

- dati essenziali del fornitore;
- contatti già selezionati sul PO;
- numero di invii personalizzati accettati, se presenti;
- data dell'ultimo invio personalizzato accettato, se presente;
- due opzioni: **Invio standard** e **Invio personalizzato**.

### 5.2 Invio standard

L'opzione standard usa senza modifiche il flusso esistente:

1. MrSmith richiama l'endpoint Arak `send-to-provider`;
2. Arak genera il PDF;
3. Arak determina destinatari e contenuto;
4. Arak invia l'email;
5. Arak chiude il PO.

La nuova feature non cambia contratto o comportamento di questo percorso.

### 5.3 Invio personalizzato

La composizione avviene nella stessa modale e permette di:

- selezionare la lingua iniziale `it` o `en`;
- assegnare contatti a **A** o **CC**;
- aggiungere inline un nuovo contatto del fornitore;
- modificare l'oggetto e il testo del messaggio precompilati;
- vedere il PDF del PO, obbligatorio e non deselezionabile;
- selezionare documenti aggiuntivi del PO gestiti da Arak;
- vedere il vincolo complessivo di 25 MB;
- confermare o annullare.

Alla conferma, il backend:

1. verifica identità, autorizzazione e stato;
2. ricarica e valida contatti e documenti;
3. recupera PDF e documenti selezionati;
4. verifica il limite complessivo di 25 MB;
5. chiude il PO tramite `arakDB`;
6. solo dopo la chiusura riuscita, invia l'email tramite il ledger MrSmith;
7. restituisce immediatamente l'esito.

Il flusso personalizzato non richiama mai `send-to-provider`.

### 5.4 Nuovo invio da PO chiuso

Su un PO `CLOSED` è disponibile **Invia nuovamente email…**.

L'azione:

- apre direttamente la composizione personalizzata;
- precompila i valori dai dati correnti del PO e del fornitore;
- permette le stesse modifiche e selezioni del primo invio;
- non modifica lo stato del PO;
- registra un nuovo tentativo indipendente nel ledger.

Non si tratta del recupero automatico di una consegna pendente: ogni nuovo invio è avviato esplicitamente dall'utente.

## 6. Destinatari e contatti

- I destinatari provengono dalla gestione contatti del fornitore già disponibile in RDA/Arak.
- La selezione iniziale riflette i contatti associati al PO; in loro assenza segue il fallback al contatto predefinito del fornitore.
- Deve essere presente almeno un destinatario **A**.
- Sono supportati destinatari **CC**.
- Non è previsto **BCC**.
- Lo stesso contatto non può essere contemporaneamente in **A** e **CC**.
- I contatti sono normalizzati e deduplicati prima dell'invio.
- Dalla composizione si può creare un contatto tramite l'API RDA/Arak esistente.
- Il nuovo contatto viene salvato nella rubrica del fornitore, inserito nell'elenco e selezionato nel ruolo dal quale è stata avviata l'aggiunta.
- La composizione corrente non deve essere persa durante la creazione del contatto.

## 7. Template e lingua

Il template iniziale è locale a MrSmith/RDA e disponibile in italiano e inglese. La struttura HTML è fissa: l'utente può modificare soltanto l'oggetto e il testo del messaggio tramite un campo multilinea. Non sono previsti HTML libero, editor rich-text o anteprima completa dell'email nella prima versione.

Il backend esegue l'escape del testo e converte i ritorni a capo in paragrafi HTML, inserendoli nel template approvato. Restano generati dal sistema e non modificabili:

- header CDLAN;
- eyebrow e titolo;
- dati identificativi del PO;
- firma `CDLAN S.p.A.`;
- footer tecnico MrSmith.

Regole principali:

- lingua iniziale configurata per il provider;
- fallback a `en` per lingua assente, invalida o non supportata;
- il cambio lingua rigenera oggetto e messaggio; se l'utente li ha già modificati, la UI chiede conferma prima di sovrascriverli;
- oggetto lungo al massimo 200 caratteri e messaggio lungo al massimo 10.000 caratteri, con validazione frontend e backend;
- identità esterna `CDLAN · Ufficio Acquisti` o `CDLAN · Purchasing Department`;
- nessun riferimento a RDA nel messaggio esterno;
- MrSmith compare soltanto come nota tecnica discreta nel footer;
- nessuna CTA: l'azione principale consiste nell'aprire il PO allegato;
- variabili supportate: `CustomerName`, `OrderNumber`, `OrderDate`, `RequesterFirstName`, `RequesterLastName`;
- `TotalAmount` escluso;
- dati mancanti gestiti senza placeholder tecnici o artefatti.

La fonte canonica per copy, subject, regole editoriali e fallback è #66. Gli esempi approvati sono:

- `apps/rda/docs/email-examples/purchase-order-email.it.html`
- `apps/rda/docs/email-examples/purchase-order-email.it.png`
- `apps/rda/docs/email-examples/purchase-order-email.en.html`
- `apps/rda/docs/email-examples/purchase-order-email.en.png`

## 8. Allegati

- Il PDF generato del PO è sempre incluso e non può essere rimosso.
- Sono selezionabili soltanto documenti associati al PO, gestiti tramite Arak e visibili/scaricabili dall'utente.
- I documenti aggiuntivi sono inizialmente non selezionati.
- Non sono previsti upload arbitrari.
- Il frontend invia soltanto gli identificativi dei documenti scelti.
- Il backend verifica esistenza, appartenenza al PO e accessibilità e scarica i file al momento dell'operazione.
- Il limite complessivo è 25 MB, PDF incluso, calcolato come somma dei byte dei file scaricati.
- Recupero e controllo dimensionale avvengono prima dell'eventuale chiusura del PO.
- Se un documento non è recuperabile o il limite è superato, il PO non viene chiuso e nessuna email viene inviata.

## 9. Autorizzazione e stato

### Primo invio personalizzato

Per un PO `PENDING_SEND`, MrSmith risolve lo `user_id` Arak e chiama direttamente:

```sql
SELECT rda.purchase_order_close($1, $2)
```

La funzione:

- blocca il PO;
- verifica esistenza e stato;
- applica internamente `rda.purchase_order_can_send`;
- registra la transizione;
- porta il PO a `CLOSED`.

Non serve una chiamata preventiva a `purchase_order_can_send`.

Mapping degli errori noto:

| SQLSTATE | Significato prodotto | Risposta attesa |
|---|---|---|
| `T0WZ0` | PO non trovato | `404` |
| `T0WZ2` | utente non autorizzato | `403` |
| `T0WZ3` | stato non valido | `409` |

### Nuovo invio

Per un PO `CLOSED`, il backend:

1. verifica lo stato `CLOSED`;
2. risolve lo `user_id` Arak;
3. chiama `rda.purchase_order_can_send(po_id, user_id)`;
4. consente l'invio soltanto in caso di esito positivo.

## 10. Ledger email

Tutti gli invii personalizzati, iniziali e successivi, passano da `emailledger.Service.Send` con correlazione:

```text
app = rda
entity_type = po
entity_id = <PO ID>
purpose = provider-email
```

Per ogni tentativo il ledger registra:

- PO correlato;
- utente che ha avviato l'azione;
- destinatari A e CC;
- oggetto;
- `Message-ID`;
- stato `accepted` o `failed`;
- errore;
- timestamp.

`accepted` indica che il sistema di consegna ha accettato il messaggio, non che il destinatario lo abbia letto o ricevuto definitivamente.

Il ledger non registra:

- corpo;
- PDF;
- contenuto degli allegati.

Nella prima versione la UI mostra soltanto il numero di invii `accepted` e la data dell'ultimo. Non è prevista una vista dello storico completo.

## 11. Contratto API di prodotto

### Preparazione

```http
GET /api/rda/v1/pos/{id}/provider-email
```

Disponibile per PO `PENDING_SEND` e `CLOSED`, previa autorizzazione. Restituisce almeno:

- lingua iniziale;
- dati essenziali del fornitore;
- contatti e selezione iniziale;
- oggetto e testo del messaggio precompilati;
- documenti selezionabili con metadata disponibili;
- descrizione del PDF obbligatorio;
- conteggio invii accettati e data dell'ultimo.

### Invio

```http
POST /api/rda/v1/pos/{id}/provider-email
Content-Type: application/json
```

Il payload contiene:

- lingua;
- ID dei contatti **A**;
- ID dei contatti **CC**;
- oggetto, lungo al massimo 200 caratteri;
- testo del messaggio, lungo al massimo 10.000 caratteri;
- ID dei documenti Arak selezionati.

Il backend decide dal corrente stato del PO se deve prima chiuderlo (`PENDING_SEND`) o effettuare soltanto il nuovo invio (`CLOSED`). Qualsiasi altro stato è rifiutato.

La creazione inline del contatto usa un'operazione API RDA/Arak separata e non fa parte del payload di invio.

## 12. Esiti ed errori UI

Gli errori che richiedono un'azione restano visibili nella modale; un toast non è sufficiente. Durante l'operazione le azioni duplicate sono disabilitate.

| Caso | Effetto | Comportamento UI |
|---|---|---|
| Successo standard | Arak invia e chiude | conferma, chiusura modale, refresh dettaglio/lista |
| Successo personalizzato iniziale | PO chiuso, tentativo `accepted` | conferma, chiusura modale, refresh dettaglio/lista |
| Successo su PO chiuso | nuovo tentativo `accepted` | conferma, aggiornamento conteggio e ultimo invio |
| PO non trovato | nessun invio | messaggio persistente e uscita sicura |
| Non autorizzato | nessun invio | messaggio persistente; azione non ripetibile senza cambio permessi |
| Stato non valido | nessun invio | informare e aggiornare i dati del PO |
| Contatto non valido/non disponibile | nessun invio | indicare la selezione da correggere |
| Documento non disponibile | nessun invio | identificare il documento e consentire di rimuoverlo |
| Limite 25 MB superato | nessun invio | mostrare il limite e consentire di ridurre gli allegati |
| Chiusura fallita | PO invariato, nessun invio | modale aperta; **Riprova** o **Annulla** |
| PO chiuso, invio fallito | PO chiuso, ledger `failed` | mostrare **L'email non è stata inviata** e **Il PO è stato chiuso correttamente. Controlla i dati e riprova.**; mantenere la modale aperta con le azioni **Riprova** e **Chiudi** |
| Invio fallito su PO già chiuso | PO invariato, ledger `failed` | mostrare **L'email non è stata inviata**; mantenere la modale aperta e consentire di riprovare |

I messaggi rivolti all'utente non menzionano ledger, transazioni o altre operazioni interne. **Riprova** conserva destinatari, oggetto, messaggio e allegati della composizione corrente, senza richiedere di ricomporre l'email.

Il frontend conserva i valori della composizione durante la modale corrente. Dopo chiusura o refresh non è garantito il ripristino: l'utente può comunque avviare **Invia nuovamente email…** dal PO chiuso. In questo caso la composizione viene generata dai dati correnti e dal template iniziale; il contenuto degli invii precedenti non viene recuperato.

Per distinguere i fallimenti di invio a livello API sono previsti errori applicativi separati, ad esempio:

- `PO_CLOSED_EMAIL_FAILED` quando la chiusura è riuscita ma l'invio no;
- `EMAIL_FAILED` quando fallisce un nuovo invio su PO già chiuso.

Una risposta `502` è appropriata per questi fallimenti esterni; il contratto definitivo dei codici viene validato in pianificazione tecnica.

## 13. Requisiti UX e accessibilità

- La modale usa il componente condiviso `Modal` e i controlli condivisi disponibili.
- UI e messaggi sono in italiano; date visualizzate secondo `it-IT`.
- Le due modalità hanno etichette e descrizioni comprensibili, senza gergo tecnico.
- Tutti i campi obbligatori hanno indicazione accessibile e validazione inline.
- Gli errori correggibili sono collegati ai relativi controlli.
- Tutte le azioni sono utilizzabili da tastiera; focus iniziale, chiusura con Escape e ripristino del focus seguono il comportamento del `Modal` condiviso.
- Lo stato di invio è annunciato e impedisce doppie conferme.
- La modale resta utilizzabile su mobile con target interattivi di almeno 44 px.

## 14. Fuori scope

- Rimozione o modifica del flusso standard Arak.
- Upload di file arbitrari.
- Storage locale di PDF o documenti Arak.
- Persistenza di bozze o composizioni pendenti.
- Code, worker e retry automatici.
- Archiviazione di corpo o MIME completo nel ledger.
- Gestione amministrativa dei template.
- Più di due lingue nella prima versione.
- BCC.
- Storico completo degli invii nell'interfaccia.
- Conferma di recapito, apertura, bounce o lettura del messaggio.

## 15. Trade-off approvati

- La chiusura precede l'invio per evitare email inviate con PO ancora aperto.
- Se l'invio fallisce dopo la chiusura, il PO resta chiuso e l'utente può effettuare un nuovo invio.
- Lo storico Arak usa la descrizione fissa `PO inviato al fornitore e chiuso` prima che MrSmith conosca l'esito dell'invio. Questa imprecisione è accettata; il ledger è la fonte dell'esito effettivo del tentativo MrSmith.
- Senza bozze persistenti, la composizione può andare persa chiudendo la modale o aggiornando la pagina.
- Gli allegati vengono recuperati nuovamente da Arak per ogni tentativo.
- Il ledger conserva metadata utili all'audit ma non il contenuto dell'email o dei documenti.

## 16. Criteri di accettazione

### Scelta e flusso standard

- Da un PO `PENDING_SEND`, il pulsante esistente apre la modale di riepilogo.
- La modale offre **Invio standard** e **Invio personalizzato**.
- L'invio standard continua a usare il flusso Arak esistente e chiude il PO senza regressioni.

### Invio personalizzato

- Un utente autorizzato può aprire la composizione da `PENDING_SEND`.
- Lingua, destinatari, oggetto e testo del messaggio sono precompilati; oggetto e messaggio sono modificabili all'interno del template HTML fisso.
- È obbligatorio almeno un contatto **A**; sono supportati contatti **CC**, non BCC.
- Un contatto può essere creato inline, salvato nella rubrica del fornitore e selezionato senza perdere la composizione.
- Il PDF del PO è sempre incluso e non deselezionabile.
- L'utente può selezionare documenti del PO accessibili tramite Arak.
- Nessun file arbitrario può essere caricato.
- Oltre 25 MB complessivi, nessuna chiusura o invio viene eseguito.
- Tutti i documenti vengono validati e recuperati prima della chiusura.
- Il PO viene chiuso tramite `purchase_order_close` prima dell'invio.
- Il flusso personalizzato non richiama `send-to-provider`.
- Ogni tentativo di invio viene registrato nel ledger.

### Errori e nuovi invii

- Se la chiusura fallisce, il PO non cambia stato e nessuna email viene inviata.
- Se l'invio fallisce dopo la chiusura, l'utente vede immediatamente che il PO è chiuso ma l'email non è stata inviata e può riprovare senza una nuova chiusura.
- Da un PO `CLOSED`, un utente autorizzato può usare **Invia nuovamente email…**.
- Il nuovo invio non modifica lo stato del PO e genera un nuovo record ledger.
- Gli errori correggibili restano visibili nella modale e la composizione corrente viene mantenuta.
- Dopo un successo, dettaglio/lista e informazioni ledger vengono aggiornati.

### Lingua, template e audit

- Il template iniziale supporta `it` ed `en`, usa la lingua del provider e ricade su `en`.
- Copy e rendering rispettano #66 e gli artefatti approvati.
- Il cambio lingua rigenera oggetto e messaggio e richiede conferma se sovrascrive modifiche dell'utente.
- Oggetto e messaggio rispettano rispettivamente i limiti di 200 e 10.000 caratteri.
- La prima versione non espone HTML libero, editor rich-text o anteprima completa.
- Dati mancanti non producono placeholder tecnici.
- `TotalAmount` non compare.
- La modale mostra conteggio e data dell'ultimo invio personalizzato accettato.
- Corpo e allegati non vengono memorizzati nel ledger.

## 17. Dipendenze e verifiche per la pianificazione tecnica

Il piano d'implementazione deve verificare nel repository, senza riaprire le decisioni di prodotto:

- risoluzione dell'utente autenticato nello `user_id` Arak;
- modalità e transazione della chiamata `arakDB` e mapping degli SQLSTATE;
- contratti effettivi per elenco, fallback e creazione dei contatti provider;
- contratti effettivi per elenco/download allegati e generazione PDF;
- disponibilità e affidabilità dei metadata dimensionali oppure necessità di conteggiare i byte scaricati;
- modello del PO e lingua provider restituiti dalle API correnti;
- inserimento sicuro nel template HTML del testo multilinea, con escape e conversione dei ritorni a capo;
- validazione server-side degli ID contatto e documento contro il PO/provider;
- comportamento del ledger quando l'invio riesce ma la registrazione fallisce, già definito dal servizio condiviso;
- refresh delle query frontend dopo invio standard, personalizzato e nuovo invio;
- verifica che il sistema di consegna supporti il limite applicativo di 25 MB calcolato sui byte dei file.

Queste verifiche sono attività di repo-fit e progettazione tecnica; non modificano il perimetro del PRD.

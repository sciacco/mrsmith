# Piano di implementazione — Invio PO al fornitore con email personalizzata

**Feature:** GitHub #55  
**Pianificazione:** GitHub #68  
**PRD:** `apps/rda/docs/PRD-PROVIDER-EMAIL.md`  
**Template:** GitHub #66  
**Stato:** pronto per implementazione

## 1. Obiettivo e confini

Implementare, nell'app RDA esistente, una modalità personalizzata di invio del PO al fornitore mantenendo invariato il percorso standard Arak `send-to-provider`.

Il lavoro comprende:

- preparazione server-side della composizione;
- invio personalizzato sincrono con PDF PO e allegati Arak selezionati;
- chiusura diretta del PO quando parte da `PENDING_SEND`;
- nuovo invio da `CLOSED`;
- audit tramite `emailledger.Service`;
- modale frontend con scelta standard/personalizzata, composizione ed errori persistenti.

Non comprende code, worker, bozze persistenti, retry automatici, upload arbitrari, storico completo, HTML libero, editor rich-text o anteprima completa.

## 2. Repo-fit verificato

### Runtime e deployment

- Non viene introdotta una nuova SPA o una nuova route client: la UI vive nel dettaglio PO esistente `apps/rda/src/pages/PoDetailPage.tsx`.
- Base Vite, mount `/apps/rda/`, deep link, porta, proxy, Docker e launcher non cambiano.
- Le nuove API restano sotto il router RDA già montato a `/api/rda/v1`; in `backend/internal/rda/handler.go` i pattern sono registrati senza il prefisso `/api`, come il resto del modulo.
- Non sono richieste nuove variabili d'ambiente o migrazioni: `Arak`, `ArakDB` ed `EmailLedger` sono già iniettati in `rda.Deps` da `backend/cmd/server/main.go`.

### Autenticazione e autorizzazione

- Tutte le route RDA sono protette da `acl.RequireRole(applaunch.RDAAccessRoles()...)` e richiedono Bearer auth tramite il client condiviso.
- L'email dell'utente autenticato è disponibile attraverso `currentClaims`/`currentEmail`; il subject Keycloak è disponibile in `auth.Claims.Subject` per l'actor del ledger.
- Lo `user_id` Arak va risolto su `users_int."user"` con confronto email coerente con le query RDA esistenti.
- Per `PENDING_SEND`, `rda.purchase_order_close(po_id, user_id)` è l'unica verifica autorizzativa e di stato necessaria.
- Per `CLOSED`, la query deve verificare lo stato corrente e chiamare `rda.purchase_order_can_send(po_id, user_id)`.
- Il dettaglio PO e i download Arak usano già `Requester-Email`; il backend non deve fidarsi di dati di contatto o allegato forniti dal browser.

### Contratti dati verificati

- `rda.purchase_order_details` restituisce provider completo, lingua, `refs`, requester, allegati, destinatari e stato.
- `provider_qualifications.provider_get` espone `company_name`, `language` e tutti i riferimenti in `refs`.
- `rda.purchase_order_get_recipients` restituisce i contatti associati al PO.
- `rda.get_purchase_order_attachments` restituisce `id`, `po_id`, `file_id`, `file_name`, tipo e timestamp, ma non la dimensione: il limite deve essere calcolato sui byte effettivamente scaricati.
- Il PDF corrente è ottenuto da `GET /arak/rda/v1/po/{id}/download`; gli allegati da `GET /arak/rda/v1/po/{id}/attachment/{aid}/download`.
- La creazione contatti esiste già su `POST /api/rda/v1/providers/{id}/references` e applica le semantiche Appsmith documentate; la risposta rende disponibile l'ID del nuovo contatto.
- `emailledger.Service.Send` invia in modo sincrono, registra `accepted`/`failed`, non salva il body e considera riuscito un invio SMTP anche se il successivo insert ledger fallisce, registrando la discrepanza nei diagnostics.

### Verifica del limite email

Il client SMTP condiviso supporta allegati multipart. Il limite applicativo viene calcolato prima della chiusura come somma dei byte scaricati di PDF e documenti, massimo `25 << 20`. Non viene stimata la dimensione MIME.

## 3. Decisioni tecniche

### 3.1 Organizzazione backend

Aggiungere file focalizzati sotto `backend/internal/rda/`, lasciando `handler.go` alla registrazione delle route:

- `provider_email.go`: handler GET/POST, orchestrazione, DTO ed errori applicativi;
- `provider_email_template.go`: copy bilingue, fallback, formattazione locale e rendering HTML/plain-text;
- eventuali piccoli helper privati nello stesso file, evitando un package general-purpose di template.

Registrare:

```text
GET  /rda/v1/pos/{id}/provider-email
POST /rda/v1/pos/{id}/provider-email
```

Il percorso standard esistente resta:

```text
POST /rda/v1/pos/{id}/send-to-provider
```

### 3.2 DTO API

La risposta di preparazione deve avere una forma esplicita e stabile, senza inoltrare JSON Arak grezzo:

```text
ProviderEmailPreparation
  po_id
  po_code
  state
  language                 it | en
  provider
    id
    company_name
  contacts[]
    id
    first_name
    last_name
    email
    reference_type
  initial_to_ids[]
  initial_cc_ids[]          inizialmente vuoto
  subject
  message
  required_pdf
    filename
  documents[]
    id                      ID purchase_order_attachment
    filename
    attachment_type
  accepted_count
  last_accepted_at          nullable
```

Payload POST:

```text
ProviderEmailSendRequest
  language                  richiesto: it | en
  to_contact_ids[]          almeno uno
  cc_contact_ids[]
  subject                   trim, 1..200 caratteri, nessun CR/LF
  message                   trim, 1..10000 caratteri
  document_ids[]            unici
```

Risposta di successo:

```text
ProviderEmailSendResponse
  status                    accepted
  po_state                  CLOSED
  accepted_count
  last_accepted_at
```

Usare `document_ids` per gli ID degli allegati PO, non `file_id`: l'ownership è verificabile direttamente contro `rda.purchase_order_attachment`.

### 3.3 Preparazione GET

Sequenza:

1. validare l'ID e leggere claims/email;
2. caricare il PO corrente;
3. accettare solo `PENDING_SEND` o `CLOSED`;
4. risolvere `user_id` e verificare `purchase_order_can_send` per entrambi gli stati, senza modificare dati; il POST resta autorevole e ripete le verifiche;
5. normalizzare la lingua provider: solo `it` resta italiano, ogni altro valore ricade su `en`;
6. costruire oggetto e messaggio iniziali dai dati correnti;
7. caricare tutti i contatti correnti del provider e filtrare quelli con email valida/non vuota;
8. selezionare in **A** i destinatari già associati al PO; se assenti, usare il contatto provider di fallback (primo riferimento valido nell'ordine restituito da Arak); CC parte vuoto;
9. esporre gli allegati correnti come selezionabili e il PDF come obbligatorio;
10. leggere dal ledger `Count` e l'ultimo record `accepted` tramite `List` con limite sufficiente a trovare l'ultimo accettato.

Correlazione unica:

```text
app=rda, entity_type=po, entity_id=<id>, purpose=provider-email
```

Se ledger o email non sono configurati, la preparazione deve fallire con `503 DEPENDENCY_UNAVAILABLE`: non va presentato un flusso che non può essere tracciato.

### 3.4 Template e sicurezza

Il backend mantiene due definizioni locali, IT ed EN, per:

- subject iniziale;
- messaggio multilinea iniziale;
- etichette e shell HTML fissa.

Regole:

- `OrderNumber` è obbligatorio per preparazione e invio;
- data PO formattata `dd/mm/yyyy` in italiano e forma inglese approvata in #66;
- assenza di provider name, data o requester gestita secondo #66 senza placeholder;
- `TotalAmount` mai incluso;
- il messaggio utente viene escapato con `html/template` e trasformato in paragrafi/line break sicuri;
- subject e testo plain-text vengono trattati come testo, mai come markup;
- il renderer produce sia `Text` sia `HTML` per il `email.Message` multipart;
- shell HTML, card PO, firma e footer restano non modificabili.

Gli esempi in `apps/rda/docs/email-examples/` sono il riferimento visivo concreto; `EMAIL-STYLE.md` resta il riferimento tecnico generale.

### 3.5 Validazione autorevole di contatti e documenti

Nel POST, dopo aver ricaricato il PO:

- deduplicare gli ID preservando l'ordine;
- rifiutare intersezioni tra A e CC;
- interrogare `provider_qualifications.provider_ref` per verificare che ogni contatto appartenga al provider corrente, non sia eliminato e abbia email valida;
- normalizzare le email con trim e deduplicazione case-insensitive; se la stessa email compare in contatti diversi, prevale **A** e non compare in CC;
- richiedere almeno un indirizzo finale in A;
- interrogare `rda.purchase_order_attachment` + `files.file` per verificare che ogni `document_id` appartenga al PO e non sia eliminato;
- non accettare allegati non richiesti o ID file arbitrari.

Gli errori devono identificare il contatto o documento correggibile con un codice applicativo e un messaggio business, senza esporre SQL o upstream interni.

### 3.6 Recupero file

Prima di qualsiasi chiusura:

1. scaricare il PDF da Arak con `Requester-Email`;
2. leggere `Content-Disposition`/`Content-Type`, applicando filename e MIME fallback sicuri;
3. scaricare ogni allegato selezionato dal relativo endpoint Arak;
4. leggere con limite cumulativo di `25 << 20` byte, interrompendo appena superato;
5. conservare i contenuti in memoria come `[]byte`, perché il client email consuma `io.Reader` durante l'invio sincrono;
6. costruire gli `email.Attachment` con nuovi `bytes.Reader` per ogni tentativo.

Un download non riuscito o il superamento del limite restituisce un errore correggibile e non chiude il PO.

### 3.7 Chiusura e invio

Dopo validazione e recupero completo:

- risolvere nuovamente lo stato autorevole dal DB nello stesso tratto logico che precede l'azione;
- se `PENDING_SEND`, eseguire `SELECT rda.purchase_order_close($1,$2)`;
- se `CLOSED`, eseguire `SELECT rda.purchase_order_can_send($1,$2)` e richiedere `true`;
- ogni altro stato produce `409 PO_STATE_CHANGED`;
- non chiamare mai `send-to-provider` nel percorso personalizzato;
- dopo la chiusura riuscita (o subito per `CLOSED`), costruire `email.Message` e chiamare `emailledger.Service.Send` con actor Keycloak subject/email.

Mapping SQLSTATE di `purchase_order_close` tramite `errors.As(err, *pgconn.PgError)`:

| SQLSTATE | HTTP | Codice applicativo |
|---|---:|---|
| `T0WZ0` | 404 | `PO_NOT_FOUND` |
| `T0WZ2` | 403 | `PO_SEND_FORBIDDEN` |
| `T0WZ3` | 409 | `PO_STATE_CHANGED` |

Fallimento SMTP:

- PO inizialmente `PENDING_SEND` e chiusura riuscita: `502 PO_CLOSED_EMAIL_FAILED`;
- PO già `CLOSED`: `502 EMAIL_FAILED`.

Il backend registra log strutturati con `component=rda`, operazione, `po_id`, stato iniziale e codice errore. Non logga body né byte degli allegati. Gli errori interni restano sanitizzati verso il client; il ledger/diagnostics conserva la diagnosi prevista dal servizio condiviso.

### 3.8 Conteggio e ultimo invio

Dopo un invio accettato, rileggere count e ultimo accepted dal ledger e includerli nella risposta. Non introdurre endpoint storico né query SQL RDA verso le tabelle del ledger.

## 4. Piano frontend

### 4.1 API e tipi

Estendere:

- `apps/rda/src/api/types.ts` con i DTO di preparazione/invio;
- `apps/rda/src/api/queries.ts` con:
  - `useProviderEmailPreparation(id, enabled)`;
  - `useSendProviderEmail()`;
  - invalidazione di `['rda','po',id]`, liste e inbox dopo successo;
  - invalidazione della preparazione dopo invio o creazione contatto.

Il trasporto resta `useApiClient`, quindi Bearer auth e gestione 401 rimangono quelli condivisi.

### 4.2 Integrazione nel dettaglio PO

Modificare `PoDetailPage.tsx`, `POCommandBar.tsx` e, se ancora usata, `ActionBar.tsx`:

- `PENDING_SEND`: il comando esistente apre la modale e non invoca più immediatamente la transition standard;
- `CLOSED`: mostrare **Invia nuovamente email…** solo quando la preparazione autorizzata è disponibile; un `403` non deve lasciare un'azione operativa;
- il flusso standard continua a chiamare `useTransitionMutation` con `send-to-provider`;
- il flusso personalizzato usa il nuovo POST;
- successo iniziale: chiudere modale, toast di conferma, refresh e navigazione coerente col comportamento corrente;
- successo da chiuso: chiudere modale, toast, refresh del dettaglio e dei dati di preparazione senza navigazione obbligatoria.

### 4.3 Modale

Creare componenti scoped sotto `apps/rda/src/components/ProviderEmailModal/` con CSS Module e shared components:

- `ProviderEmailModal.tsx` come contenitore di stato della composizione;
- eventuali sottocomponenti locali per scelta modalità, destinatari e allegati, solo se riducono realmente la complessità;
- `ProviderEmailModal.module.css` basato esclusivamente sui token clean.

Usare `Modal` condiviso, preferibilmente `size="wide"`, `Button`, `Icon` e controlli condivisi disponibili. Struttura:

1. riepilogo fornitore, destinatari correnti e dati ultimo invio;
2. su `PENDING_SEND`, scelta tra **Invio standard** e **Invio personalizzato**;
3. composizione personalizzata con lingua, A, CC, oggetto, textarea messaggio e allegati;
4. PDF obbligatorio visibile e non selezionabile;
5. allegati aggiuntivi non selezionati;
6. errore persistente e azioni finali.

Requisiti UI:

- campi e target almeno 44 px;
- required marker accessibile secondo `docs/UI-UX.md`;
- errori collegati con `aria-invalid`/`aria-describedby`;
- stato invio annunciato (`aria-live`) e azioni disabilitate durante la mutation;
- errori correggibili persistenti nella modale, non solo toast;
- layout mobile a colonna singola;
- nessuna anteprima completa.

### 4.4 Gestione stato locale

La modale conserva localmente:

- lingua;
- A e CC;
- subject;
- message;
- document IDs;
- flag dirty per subject/message;
- errore corrente.

Cambio lingua:

- se subject/message non sono stati modificati, applicare subito i valori iniziali della nuova lingua;
- se modificati, aprire una conferma; solo dopo conferma rigenerare entrambi;
- per evitare una nuova chiamata speciale, la preparazione può restituire i template iniziali per entrambe le lingue oppure la UI può richiamare GET con `?language=it|en`; scegliere la prima forma nel DTO finale per mantenere il cambio sincrono. Il DTO va quindi implementato con `templates.it` e `templates.en`, mantenendo `language` come scelta iniziale.

Alla chiusura della modale lo stato viene scartato. Alla riapertura da `CLOSED` viene ricaricata una composizione nuova dai dati correnti.

### 4.5 Contatto inline

Riutilizzare la logica e il form di `ProviderContactModal` senza annidare due `<dialog>` aperti contemporaneamente:

- aprire il form contatto come step interno della stessa modale, oppure chiudere temporaneamente la vista di composizione mantenendone lo stato nel parent;
- chiamare `createReference` esistente;
- alla riuscita, aggiornare/invalidate il provider, inserire il contatto restituito e selezionarlo nel ruolo A/CC dal quale è partita l'azione;
- non azzerare gli altri campi della composizione.

Il tipo contatto resta uno dei tipi RDA già ammessi; `QUALIFICATION_REF` non viene creato da questo flusso.

### 4.6 Error mapping e copy

Aggiungere in `apps/rda/src/lib/api-error.ts` un mapping dedicato per i codici provider-email.

Comportamenti principali:

- `PO_CLOSED_EMAIL_FAILED`: **L'email non è stata inviata** / **Il PO è stato chiuso correttamente. Controlla i dati e riprova.**; pulsanti **Riprova** e **Chiudi**;
- `EMAIL_FAILED`: **L'email non è stata inviata. Controlla i dati e riprova.**;
- contatto/documento non valido: errore accanto alla relativa sezione;
- `ATTACHMENTS_TOO_LARGE`: indicare limite 25 MB e mantenere le selezioni;
- `PO_STATE_CHANGED`: informare che il PO è cambiato e aggiornare il dettaglio;
- `403`: messaggio persistente e nessuna ripetizione automatica;
- `404`: chiusura sicura e refresh/navigazione.

Nessun copy UI menziona ledger, SQL, transazioni, Arak o SMTP.

## 5. Ordine di implementazione

1. **Contratti e helper backend**
   - DTO, codici errore, risoluzione user ID, correlazione ledger e template bilingue.
2. **Preparazione GET**
   - autorizzazione, dati correnti, contatti/fallback, documenti, count e ultimo accepted.
3. **Recupero e validazione risorse**
   - contatti, ownership documenti, download PDF/allegati e limite cumulativo.
4. **Orchestrazione POST**
   - chiusura/can-send, mapping SQLSTATE, invio ledger e risposte differenziate.
5. **Client API e tipi frontend**
   - query, mutation e invalidazioni.
6. **Modale e contatto inline**
   - scelta modalità, composizione, cambio lingua, validazioni ed errori persistenti.
7. **Wiring nel dettaglio PO**
   - comando `PENDING_SEND`, reinvio `CLOSED`, standard invariato e refresh.
8. **Verifica integrata e rifinitura accessibilità/mobile**.

## 6. Strategia di verifica

### Check obbligatori senza nuovi test

- type-check/build di `apps/rda`;
- `go test` sui package backend già esistenti, senza aggiungere file di test;
- smoke test manuale della modale su desktop e mobile;
- tastiera: apertura, tab order, Escape, focus restore, conferma cambio lingua;
- verifica visuale template IT/EN contro gli artefatti approvati;
- verifica che body e allegati non compaiano in ledger o log;
- verifica che il flusso standard continui a chiamare esclusivamente `send-to-provider`.

### Matrice funzionale manuale

- GET autorizzato e vietato su `PENDING_SEND`/`CLOSED`;
- invio standard da `PENDING_SEND`;
- invio personalizzato senza allegati opzionali;
- invio con allegati sotto e sopra 25 MB;
- A vuoto, duplicati A/CC, contatto non appartenente al provider;
- documento non appartenente al PO o non più scaricabile;
- cambio lingua con e senza modifiche locali;
- chiusura fallita;
- chiusura riuscita e invio fallito, quindi **Riprova**;
- nuovo invio da `CLOSED`;
- concorrenza/stato cambiato con risposta 409;
- refresh liste, inbox e dettaglio dopo ogni successo.

### Test automatici da sottoporre ad approvazione prima dell'implementazione

La feature contiene regole business-critical e trasformazioni non banali. È opportuno chiedere approvazione per test mirati su:

- mapping SQLSTATE e distinzione `PO_CLOSED_EMAIL_FAILED`/`EMAIL_FAILED`;
- validazione ownership/dedup contatti e documenti;
- limite cumulativo byte prima della chiusura;
- rendering sicuro del messaggio e fallback template;
- comportamento del cambio lingua e conservazione composizione.

Non aggiungere test finché l'approvazione non viene data.

## 7. Osservabilità e gestione errori

- Riutilizzare request ID e logging middleware esistenti.
- Loggare solo metadati operativi: operation, PO ID, stato, conteggio destinatari/allegati, byte totali e codice errore.
- Non loggare messaggio, subject completo se non necessario, indirizzi in chiaro oltre a quanto già previsto dal ledger, né contenuto file.
- Le dipendenze mancanti restituiscono 503 sanitizzato; errori Arak/email restituiscono messaggi business e codici applicativi.
- Il comportamento ledger “email riuscita, insert fallito” resta quello condiviso: successo al chiamante e diagnostica server-side.

## 8. File previsti

### Backend

- `backend/internal/rda/handler.go` — registrazione route.
- `backend/internal/rda/types.go` — solo dipendenze condivise se necessario; preferire DTO nel file feature.
- `backend/internal/rda/provider_email.go` — nuovo.
- `backend/internal/rda/provider_email_template.go` — nuovo.
- `backend/internal/rda/arak.go` o helper feature — riuso fetch autenticato senza cambiare il proxy generale.

### Frontend

- `apps/rda/src/api/types.ts`.
- `apps/rda/src/api/queries.ts`.
- `apps/rda/src/pages/PoDetailPage.tsx`.
- `apps/rda/src/components/POCommandBar.tsx`.
- `apps/rda/src/components/ActionBar.tsx`, se ancora parte del percorso renderizzato.
- `apps/rda/src/components/ProviderEmailModal/ProviderEmailModal.tsx` — nuovo.
- `apps/rda/src/components/ProviderEmailModal/ProviderEmailModal.module.css` — nuovo.
- `apps/rda/src/lib/api-error.ts`.

### Documentazione

- Aggiornare il PRD solo se l'implementazione scopre una divergenza reale di contratto.
- Aggiornare `docs/IMPLEMENTATION-KNOWLEDGE.md` soltanto per nuove conoscenze RDA riutilizzabili non già presenti.

## 9. Repo-fit checklist conclusiva

| Area | Esito |
|---|---|
| Runtime/base path/deep links | Nessuna modifica |
| Dev port/proxy/scripts | Nessuna modifica |
| Bearer auth | Client e ACL condivisi già applicati |
| Authz dominio | Funzioni Arak `close`/`can_send` |
| Identificatori | PO/contact/document ID numerici; ownership verificata |
| Database/migrazioni | Nessuna nuova migrazione |
| Config/env | Nessuna nuova variabile |
| Deployment/static assets | Nessuna modifica |
| Error sanitization | Codici applicativi + log strutturati |
| Failure parziale | Esplicitamente gestito dopo chiusura |
| Refresh frontend | Dettaglio, liste, inbox e preparation invalidati |
| Verifica | Type-check, Go suite esistente e smoke manuale; nuovi test previa approvazione |

## 10. Decisione confermata

La risposta GET include entrambi i template iniziali (`templates.it` e `templates.en`) per rendere il cambio lingua immediato e permettere la conferma prima della sovrascrittura. `language` continua a indicare la selezione iniziale derivata dal provider.

# Piano di Implementazione UI/UX: Creazione Preventivi Aenad V1 (Issue #61)

Questo documento definisce il piano di implementazione per la nuova interfaccia di creazione e modifica dei preventivi nell'applicazione **Aenad**. Il piano segue la direzione approvata basata sulla combinazione delle proposte **A (Anteprima Semantica Responsiva)** e **C (Split-Pane Flessibile e Collassabile)** con riga di inserimento rapido CLI-like da tastiera.

---

## 1. Product Behavior (Esperienza Utente)

L'interfaccia si articolerà in un layout a due colonne (Split-Pane) che si adatta dinamicamente allo schermo.

```
+--------------------------------------------------+--------------------------------------------------+
| PANNELLO EDITING (SINISTRA: 50% - 100%)          | ANTEPRIMA RESPONSIVA (DESTRA: 50% - Overlay)     |
|                                                  |                                                  |
| * Risoluzione Prospect & Cliente                 | * Layout semantico responsivo del preventivo      |
| * Dati documento (Data, Pagamento)              | * Calcoli e totali aggiornati in tempo reale     |
| * Tabella righe con CLI Command Bar              | * Timeline degli eventi & HubSpot sync status     |
+--------------------------------------------------+--------------------------------------------------+
```

### 1.1 Pannello Editing (Sinistra)
* **Risoluzione Cliente/Prospect**: Campo di ricerca unificato. Se l'indirizzo email inserito non appartiene a una company esistente in `loader.hubs_company` (tramite `/quotes/customers`), la UI mostra in-line una scheda compatta per compilare i dati mancanti ed eseguire `POST /quotes/prospects` recuperando in tempo reale gli ID HubSpot associati.
* **CLI Command Bar**: La tabella delle righe ospita in fondo una riga speciale che agisce da input testuale singolo. Digitando es. `ABC123 5x100 -10+5 #22` e premendo `Invio`, la riga viene analizzata lato client, aggiunta alla tabella e inviata per il ricalcolo. Digitando `// nota` o `---` si aggiungono righe descrittive o separatori. È comunque presente un pulsante `+ Aggiungi` per l'inserimento classico via campi di input.
* **Badge e Retrocessione di Stato**: Un badge visivo persistente indica lo stato del preventivo (`draft` o `ready`). Se il preventivo è `ready` e l'utente modifica dati commerciali/strutturali, il badge sfuma a `draft` e compare un micro-banner: *"Questa modifica riporterà il preventivo in bozza."*

### 1.2 Pannello Anteprima Responsiva (Destra - Opzioni A+C)
* **Anteprima Semantica (Opzione A)**: Invece di un PDF A4 cartaceo standard che risulta illeggibile in spazi ridotti, mostriamo una rappresentazione responsive (stile fattura digitale di Stripe). Se lo spazio è ridotto, le colonne delle righe si impilano automaticamente in un layout "stacked".
* **Splitter Flessibile (Opzione C)**: Un divisorio trascinabile consente di modificare il rapporto di larghezza tra sinistra e destra.
* **Drawer automatico su schermi piccoli (Opzione C)**: Sotto i `1200px` di larghezza schermo, il pannello destro collassa. Al suo posto viene mostrato un pulsante flottante "Mostra Anteprima" (e lo shortcut `Cmd+P`) che apre l'anteprima come Drawer in overlay sopra l'area di input, massimizzando lo spazio di inserimento sui laptop.
* **HubSpot Sync Log**: Un sotto-tab permette di passare dall'anteprima del preventivo alla visualizzazione della timeline dei log di sync (`quote_event`) con dettagli sugli errori e pulsante `Retry` rapido.

---

## 2. Repo/Runtime Integration (Integrazione e Routing)

* **Directory di Lavoro**: `apps/aenad`
* **Routing (`apps/aenad/src/routes.tsx`)**:
  * `/preventivi`: Nuovo elenco paginato dei preventivi V1 (caricati tramite `GET /api/aenad/v1/quotes`). Sostituisce l'attuale stato vuoto di `PreventiviPage.tsx`.
  * `/preventivi/nuovo`: Interfaccia vuota di creazione preventivo (inizializza un preventivo `draft` localmente e lo salva al primo inserimento).
  * `/preventivi/:id`: Interfaccia di visualizzazione/modifica del preventivo esistente basata sul layout Split-Pane.
* **Shared Components**:
  * Utilizzo di `AppShell`, `TabNav`, `Button`, `Drawer`, `Skeleton`, `ToggleSwitch` e `SingleSelect` da `@mrsmith/ui`.
  * Gli stili specifici dello Split-Pane e dell'anteprima semantica saranno gestiti tramite CSS Modules dedicati (es. `ModificaPreventivoPage.module.css`).

---

## 3. Data & Auth Contracts (End-point e Sicurezza)

* **Autenticazione**: Tutte le chiamate API includono le credenziali di sessione fornite da `@mrsmith/auth-client`. È richiesto il ruolo Keycloak `app_aenad_access` (controllato lato BFF).
* **API Endpoints Utilizzati (Raenad Backend)**:
  * `GET /api/aenad/v1/quotes/customers?q=...` -> Ricerca clienti mirror.
  * `POST /api/aenad/v1/quotes/prospects` -> Creazione rapida Prospect live su HubSpot.
  * `GET /api/aenad/v1/quotes/articles?q=...` -> Ricerca articoli Alyante live.
  * `GET /api/aenad/v1/quotes/payment-methods` -> Elenco metodi pagamento selezionabili.
  * `GET /api/aenad/v1/quotes/defaults` -> Default di linea (es. Codice IVA predefinito).
  * `GET /api/aenad/v1/quotes` -> Elenco preventivi V1 paginato.
  * `POST /api/aenad/v1/quotes` -> Creazione preventivo bozza (invia header e righe, riceve i totali calcolati a DB).
  * `GET /api/aenad/v1/quotes/{id}` -> Dettaglio completo.
  * `PUT /api/aenad/v1/quotes/{id}` -> Salvataggio completo (invia intero payload, normalizza righe, gestisce retrocessione automatica ready->draft).
  * `POST /api/aenad/v1/quotes/{id}/ready` -> Validazione finale e marcatura come pronto.
  * `POST /api/aenad/v1/quotes/{id}/hubspot/retry` -> Forza il rinvio in coda per sincronizzazione fallita.
  * `GET /api/aenad/v1/quotes/{id}/pdf-exports` -> Lista esportazioni PDF storiche/correnti.
  * `POST /api/aenad/v1/quotes/{id}/pdf-exports` -> Crea una nuova revisione PDF immutabile o riusa l'esistente se il checksum coincide.
  * `POST /api/aenad/v1/quotes/{id}/pdf-exports/{exportID}/attach` -> Accoda l'invio asincrono dell'allegato su HubSpot.

---

## 4. Verification Strategy (Strategia di Verifica e Test)

Poiché l'applicazione deve garantire stabilità ed efficienza assoluta per gli operatori:

1. **Verifica Calcoli Client-Side**:
   * Implementare test unitari in React per la funzione di parsing degli sconti sequenziali (es. `10+5%` o `10+5+2` deve corrispondere esattamente al moltiplicatore calcolato dalla funzione Postgres `discount_multiplier`).
   * Verificare la correttezza del calcolo dell'IVA locale e del prezzo netto riga prima dell'invio.
2. **Test di Validazione di Stato (Ready Transition)**:
   * Testare il comportamento in fase di compilazione incompleta (es. mancanza di righe item o metodo di pagamento) e verificare che l'errore `quote_not_ready` o la convalida client impedisca la transizione.
3. **Verifica della Retrocessione a Draft (Demotion)**:
   * Aprire un preventivo `ready`. Effettuare una modifica alle sole note interne (`internal_notes`) -> Verificare che lo stato rimanga `ready` e non venga accodato nessun update su HubSpot.
   * Modificare invece la quantità o il prezzo di una riga -> Verificare che lo stato passi immediatamente a `draft` sia a frontend che a backend dopo il salvataggio (`PUT`).
4. **Verifica della Reattività Responsiva (Opzioni A+C)**:
   * Ridurre lo schermo sotto i `1200px` -> Verificare che il pannello di anteprima scompaia e compaia il pulsante per aprirlo come overlay (Drawer).
   * Verificare il corretto wrap delle colonne delle righe preventivo a larghezze inferiori a `500px` all'interno dell'anteprima.
5. **Simulazione HubSpot Sync**:
   * Simulare fallimenti temporanei del worker HubSpot e verificare che il frontend mostri lo stato `failed` recuperato da `hubspot_sync_status` e consenta la pressione del tasto `Retry` che attiva l'endpoint dedicato.

# Reports — Phase B: UX Design Report

Report di consulenza UX per la migrazione dell'app Reports da Appsmith a React.

---

## Domanda 1: Home page come hub a card vs TabNav

### Analisi del contesto

L'app Reports e una **collezione di strumenti indipendenti**, non un flusso di lavoro unico. Ogni report ha filtri, logica e output propri. Questo la distingue da app come Budget o Listini e Sconti, dove le pagine sono aspetti diversi di uno stesso dominio.

Vincoli rilevanti:
- Il portale MrSmith usa gia card (AppCard) per lanciare le app — una Home con card creerebbe un "portale nel portale"
- Le altre mini-app (Panoramica Cliente con 7 pagine, Listini e Sconti con 7 pagine) usano `TabNavGroup` con dropdown, non card hub
- Il numero di report crescera nel tempo
- La coerenza con le altre mini-app ha valore: l'utente impara un pattern e lo riusa ovunque

### Proposta A: TabNavGroup con raggruppamento logico (raccomandata)

Usare `TabNavGroup` raggruppando i 7 report in 3-4 categorie logiche, esattamente come fanno Panoramica Cliente e Listini e Sconti. La Home diventa la prima voce di navigazione con un dashboard riassuntivo leggero.

**Esempio di raggruppamento:**

| Gruppo | Pagine |
|--------|--------|
| Business | Ordini, AOV, Rinnovi in arrivo |
| Operations | Attivazioni in corso |
| Servizi | Accessi attivi, Anomalie MOR, Accounting TIMOO |

La Home (`/`) mostra un riepilogo: card compatte con titolo del report, icona (dal sistema Icon esistente: `chart`, `document`, `coins`, etc.), una riga di descrizione e un indicatore di stato (es. "ultimo export: 3 giorni fa" o "12 anomalie rilevate"). Cliccando una card si naviga al report. Ma la navigazione principale resta `TabNavGroup` nell'header.

**Pro:**
- Coerenza al 100% con le altre mini-app del portale
- Scala bene: aggiungere un report = aggiungere una voce al gruppo appropriato nel `TabNavGroup`
- L'utente puo passare da un report all'altro senza tornare alla Home
- I dropdown del `TabNavGroup` gestiscono gia hover, keyboard nav e indicatore animato
- La Home aggiunge valore come dashboard senza essere l'unico punto di accesso

**Contro:**
- Con 7+ report i gruppi nel `TabNavGroup` diventano 4+, ma il pattern regge (Listini e Sconti ne ha gia 4)
- La Home richiede endpoint per i dati riassuntivi (costo di sviluppo aggiuntivo)

**Design tokens e componenti:**
- `TabNavGroup` con `groups[]` nel nav dell'`AppShell`
- Card della Home: `--color-bg-elevated` sfondo, `--shadow-sm` a riposo, `--shadow-md` su hover, `--radius-lg` bordi, animazione `sectionEnter` con stagger `0.1s`
- Icone: sistema Icon esistente (36x36px wrapper con sfondo tintato `--color-accent-subtle`)
- Testo secondario nelle card: `--color-text-muted`, `0.8125rem`

### Proposta B: Home come hub a card, senza TabNav

Eliminare `TabNav`/`TabNavGroup` dall'header. L'AppShell mostra solo logo e UserMenu. La Home e una griglia di card cliccabili. Ogni report e una "sotto-app" a se: si entra, si lavora, si torna alla Home con un pulsante "back" o il logo.

**Griglia card:**

| Breakpoint | Colonne |
|------------|---------|
| >= 1200px | 3 |
| 640-1200px | 2 |
| < 640px | 1 |

Ogni card: icona + titolo + descrizione breve + eventuale badge ("XLSX", "AI", "Auto"). Gap: `--space-6`. Animazione: `sectionEnter` con stagger.

**Pro:**
- Layout piu pulito nell'header (nessuna barra di navigazione affollata)
- Ogni card puo mostrare meta-informazioni contestuali
- Aggiungere un report = aggiungere una card (nessun vincolo di raggruppamento)
- Pattern familiare (simile a Google Analytics Hub, Metabase Home)

**Contro:**
- Rompe la coerenza con tutte le altre mini-app MrSmith che usano `TabNavGroup`
- L'utente deve tornare alla Home ogni volta che vuole cambiare report — nessun quick-switch
- Non riusa il componente `TabNavGroup` gia costruito e testato
- Duplica il pattern gia presente nel portale (card per lanciare app)

### Proposta C: Ibrido — TabNavGroup + Home come dashboard

Come la Proposta A, ma la Home non e solo un punto di atterraggio: e un vero **dashboard operativo** con metriche live. Le card non sono semplici link ma widget informativi.

Contenuto della Home:
- **Sezione "Attenzione"**: card con alert (es. "14 anomalie MOR non revisionate", "3 attivazioni bloccate da >30gg")
- **Sezione "Report recenti"**: cronologia degli ultimi export eseguiti con link per rieseguire
- **Sezione "Accesso rapido"**: griglia compatta di card-link ai report

Il `TabNavGroup` resta nell'header per navigazione diretta.

**Pro:**
- Valore aggiunto reale della Home: non e solo un menu, e un centro operativo
- Coerenza con il pattern di navigazione delle altre app
- L'utente esperto usa il `TabNavGroup`, il nuovo utente parte dalla Home

**Contro:**
- Costo di sviluppo significativamente piu alto (endpoint per metriche, logica di alerting)
- Rischio di overengineering per una V1
- I dati di alert richiedono query aggiuntive su ogni caricamento della Home

### Raccomandazione

**Proposta A** per la V1. Il `TabNavGroup` e un pattern consolidato nel codebase (usato da Panoramica Cliente e Listini e Sconti, entrambe con 7+ pagine), scala bene, e mantiene la coerenza cross-app che e uno dei pilastri del design system MrSmith. La Home con card compatte fornisce un punto di atterraggio utile senza richiedere endpoint aggiuntivi nella V1.

Se in futuro il numero di report supera le 12-15 unita, si puo evolvere verso la Proposta C aggiungendo widget informativi alla Home. La struttura `TabNavGroup` non cambiera.

---

## Domanda 2: AOV — Layout a 4 tabelle

### Analisi del contesto

La pagina AOV produce 4 viste simultanee sugli stessi dati filtrati:
1. Per tipo ordine (poche righe, ~5-8)
2. Per categoria prodotto (medie righe, ~15-30)
3. Per commerciale (medie righe, ~10-20)
4. Dettaglio completo (molte righe, potenzialmente centinaia)

L'utente spesso confronta i totali tra le viste aggregate per capire dove si concentra il valore. Il filtro e condiviso (date + stati ordine). Esiste un export XLSX che genera tutte le viste in un unico file.

Lo stack verticale attuale forza scroll eccessivo e perde il contesto delle tabelle superiori quando si guarda quelle inferiori.

### Proposta A: Tab interni con anteprima riepilogo (raccomandata)

Dopo l'applicazione dei filtri, mostrare un **pannello riassuntivo** in alto con le metriche chiave di tutte le viste, poi un `TabNav` locale (controllato, non routing) per alternare tra le 4 tabelle.

**Layout:**

```
[Filtri: date range + stati ordine] [Esegui] [Esporta XLSX]
-----------------------------------------------------------------
| AOV Totale    | Per tipo (N)  | Per categoria (N) | Per comm. (N) |
| EUR 1.234.567 | 5 tipi        | 23 categorie       | 12 commerc.   |
-----------------------------------------------------------------
[ Per tipo | Per categoria | Per commerciale | Dettaglio ]  <-- TabNav controllato
-----------------------------------------------------------------
| Tabella della vista selezionata                          |
-----------------------------------------------------------------
```

Il pannello riassuntivo usa 4 card compatte in riga (tipo "stat cards" alla Stripe) che mostrano il totale e il conteggio righe di ogni vista. Cliccando una stat card si attiva il tab corrispondente.

**Design tokens e componenti:**
- Stat cards: `--color-bg-elevated`, `--shadow-xs`, `--radius-md`, padding `--space-4` verticale / `--space-5` orizzontale
- Valore principale: `1.75rem`, weight 700, `--color-text`
- Label: `0.75rem` uppercase, weight 600, `--color-text-muted`, `letter-spacing: 0.06em`
- Griglia stat cards: 4 colonne su desktop, 2x2 su tablet, stack su mobile
- `TabNav` con `activeKey`/`onTabChange` (modalita controllata, nessun routing)
- Transizione tabella: `opacity 0->1` con `pageEnter` (0.3s) al cambio tab

**Pro:**
- Zero scroll: tutto e visibile nello schermo
- Il riepilogo fornisce contesto immediato senza cambiare tab
- Le stat card fungono da "scorciatoia" visiva per il tab corrispondente
- `TabNav` in modalita controllata e gia supportato dal componente
- Pattern usato da Stripe nella sezione Payments (tab per viste diverse degli stessi dati)

**Contro:**
- L'utente non puo vedere due tabelle affiancate per confronto diretto
- Richiede il calcolo dei totali per il pannello riassuntivo

**Responsive:**
- >= 1200px: 4 stat card in riga + tabella piena
- 640-1200px: 2x2 stat card + tabella piena
- < 640px: stat card in stack verticale, tabella con scroll orizzontale

### Proposta B: Griglia 2x2 con tabelle compatte

Disporre le 4 tabelle in una griglia 2x2. Le 3 tabelle aggregate (tipo, categoria, commerciale) occupano le celle superiori e la cella inferiore-sinistra. Il dettaglio completo occupa la cella inferiore-destra (o un'area espandibile).

**Layout:**

```
[Filtri: date range + stati ordine] [Esegui] [Esporta XLSX]
-----------------------------------------------------------------
| Per tipo ordine (scroll interno)  | Per categoria prodotto     |
| max-height: 320px                 | max-height: 320px          |
|---------------------------------+-----------------------------|
| Per commerciale                   | Dettaglio completo          |
| max-height: 320px                 | max-height: 320px           |
-----------------------------------------------------------------
```

Ogni cella e un pannello con header (titolo + conteggio righe), bordo `--color-border`, scroll interno indipendente, e possibilita di "espandere" a pieno schermo con un pulsante nell'header.

**Design tokens e componenti:**
- Pannelli: `--color-bg-elevated`, `--radius-lg`, `border: 1px solid var(--color-border)`
- Header pannello: padding `--space-3` / `--space-4`, `border-bottom: 1px solid var(--color-border-subtle)`
- Titolo pannello: `0.875rem`, weight 600
- Conteggio: `--color-text-muted`, `0.8125rem`
- Pulsante espandi: icona `expand` in alto a destra, `--color-text-muted`, hover `--color-accent`
- Griglia: `display: grid; grid-template-columns: 1fr 1fr; gap: var(--space-4)`

**Pro:**
- Tutte le viste visibili simultaneamente — massimo contesto
- Il confronto tra viste e immediato (basta spostare lo sguardo)
- Ogni tabella ha scroll indipendente

**Contro:**
- Spazio limitato per ogni tabella: colonne strette, righe visibili poche
- Il dettaglio completo (molte colonne) non si legge bene in mezza larghezza
- Su tablet/mobile la griglia deve diventare stack verticale, perdendo il vantaggio
- Pattern meno pulito, rischio di interfaccia "affollata" (anti-Stripe)

**Responsive:**
- >= 1200px: griglia 2x2
- 640-1200px: stack verticale con max-height ridotto
- < 640px: stack verticale, scroll orizzontale per ogni tabella

### Proposta C: Tab con pannello laterale di confronto

Come la Proposta A (tab singoli), ma con l'aggiunta di un **pannello laterale** (Drawer) che si apre per mostrare una seconda vista affiancata. L'utente seleziona la vista principale nel tab e puo aprire una seconda vista nel drawer per confronto.

**Pro:**
- Confronto diretto possibile senza sacrificare lo spazio della vista principale
- Il drawer e gia nel design system (`Drawer` component)

**Contro:**
- Complessita di interazione elevata per un caso d'uso che potrebbe non essere frequente
- Il drawer riduce lo spazio della tabella principale
- Overengineering per la V1

### Raccomandazione

**Proposta A** (tab con pannello riassuntivo). E il pattern piu pulito, piu coerente con l'estetica Stripe, e risolve il problema dello scroll senza sacrificare leggibilita. Le stat card in alto mantengono il contesto globale indipendentemente dal tab attivo. Il `TabNav` controllato e gia pronto nel codebase.

Se il feedback degli utenti indica che il confronto diretto tra viste e un'esigenza reale e frequente, si puo evolvere verso la Proposta B in una fase successiva.

---

## Domanda 3: Ordini e Accessi attivi — Export cieco vs anteprima

### Analisi del contesto

Oggi Ordini e Accessi attivi funzionano cosi:
1. L'utente imposta i filtri (date, stati/tipi)
2. Clicca "Genera report"
3. Il sistema esegue la query, invia i dati a Carbone.io, e apre l'XLSX in una nuova finestra

Non c'e modo di verificare cosa si sta esportando prima del download. Se i filtri sono sbagliati, l'utente se ne accorge solo aprendo il file Excel.

Vincoli:
- I dati possono essere voluminosi (centinaia/migliaia di righe)
- Il rendering XLSX via Carbone.io richiede tempo
- L'utente vuole un feedback significativo, non solo "42 righe trovate"

### Proposta A: Anteprima con riepilogo aggregato e dettaglio automatico (raccomandata)

Dopo aver impostato i filtri, l'utente clicca un pulsante **"Anteprima"** (Button secondary). Il sistema esegue la query e mostra:

**Fase 1 — Riepilogo**

Un pannello riassuntivo con metriche aggregate calcolate sui dati filtrati. Il contenuto varia per pagina:

**Ordini:**
- Numero ordini trovati
- MRC totale / NRC totale
- Breakdown per stato ordine (mini tabella o lista)
- Range date effettivo (primo e ultimo ordine nel risultato)

**Accessi attivi:**
- Numero linee trovate
- Breakdown per tipo connessione
- Breakdown per stato linea

Sotto il riepilogo, una sola azione:
- **"Esporta XLSX"** (Button primary) — genera e scarica

**Fase 2 — Dettaglio**

La tabella dettaglio appare automaticamente sotto il riepilogo quando il risultato contiene almeno una riga. La tabella mostra le prime 100 righe con un indicatore di campionamento. L'export XLSX contiene comunque tutte le righe.

**Layout:**

```
[Filtri: date + stati/tipi] [Anteprima]
-----------------------------------------------------------------
                    (stato iniziale: vuoto)
-----------------------------------------------------------------

        dopo click "Anteprima":

[Filtri: date + stati/tipi] [Anteprima]
-----------------------------------------------------------------
| Riepilogo                                                     |
| 1.247 ordini | MRC tot: EUR 45.230 | NRC tot: EUR 12.800    |
|                                                               |
| Per stato:  Confermato 890 | In lavorazione 245 | Nuovo 112  |
| Periodo:    12/01/2026 — 11/04/2026                           |
-----------------------------------------------------------------
[Esporta XLSX]
-----------------------------------------------------------------

-----------------------------------------------------------------
| Tabella (prime 100 di 1.247 righe)                            |
| ...                                                           |
-----------------------------------------------------------------
```

**Design tokens e componenti:**
- Pannello riepilogo: `--color-bg-elevated`, `--radius-lg`, `--shadow-sm`, padding `--space-6`
- Metriche principali: `1.75rem` weight 700, `--color-text`; label `0.75rem` uppercase `--color-text-muted`
- Breakdown: chip/badge per ogni valore con count, `--color-surface` sfondo, `--radius-full`, `0.8125rem`
- Animazione comparsa: `sectionEnter` (0.5s ease-out)
- Pulsante "Esporta XLSX": `Button` primary (pill, indigo gradient)
- Tabella: stile standard con `rowEnter` stagger, `Skeleton` durante il caricamento
- Indicatore troncamento: banner sopra la tabella, `--color-surface` sfondo, `0.8125rem`, `--color-text-secondary`

**Pro:**
- L'utente verifica i filtri prima di esportare grazie a metriche significative
- Il riepilogo e veloce da calcolare (stessi dati, solo aggregazione client-side)
- La tabella completa e immediatamente disponibile quando ci sono risultati
- Il pattern "preview before commit" e molto Stripe (es. preview fattura prima di invio)
- Nessuna chiamata API aggiuntiva: i dati vengono fetchati una volta, il riepilogo e calcolato in-memory

**Contro:**
- Due click invece di uno per l'export (mitigato: il pulsante "Esporta" e prominente nel riepilogo)
- I dati vengono tenuti in memoria per la tabella (accettabile per volumi tipici)

**Responsive:**
- Metriche principali: riga orizzontale su desktop, stack su mobile
- Breakdown: wrap naturale dei chip
- Tabella: scroll orizzontale su schermi stretti

### Proposta B: Anteprima inline immediata con export in coda

Approccio diverso: il pulsante diventa **"Genera report"** e al click mostra immediatamente la tabella dati (prime 50 righe con paginazione), con un banner in alto che contiene le metriche aggregate e il pulsante "Esporta XLSX".

La tabella e la vista principale, non secondaria. L'utente vede i dati, li scorre, e quando e soddisfatto esporta.

**Layout:**

```
[Filtri: date + stati/tipi] [Genera report]
-----------------------------------------------------------------
| Banner: 1.247 ordini trovati | MRC: EUR 45.230 | [Esporta XLSX] |
-----------------------------------------------------------------
| Tabella paginata (50 righe per pagina)                         |
| Pagina 1 di 25                                                 |
-----------------------------------------------------------------
```

**Design tokens e componenti:**
- Banner: `--color-accent-subtle` sfondo, `--radius-md`, padding `--space-3` / `--space-5`, `display: flex; align-items: center; justify-content: space-between`
- Conteggio e metriche nel banner: `0.875rem` weight 600, `--color-text`
- Pulsante export nel banner: `Button` primary, dimensione compatta
- Tabella: paginazione con pulsanti prev/next
- `Skeleton` con righe stagger durante il caricamento

**Pro:**
- Flusso piu diretto: un click mostra tutto
- L'utente vede subito i dati reali, non solo aggregati
- L'export e sempre visibile nel banner (sticky se la tabella scrolls)

**Contro:**
- Caricare centinaia/migliaia di righe per la tabella ha un costo (paginazione server-side o client-side necessaria)
- Piu pesante per il browser rispetto al solo riepilogo
- L'utente potrebbe non aver bisogno di vedere la tabella (vuole solo esportare)
- La paginazione aggiunge complessita (componente da costruire o adattare)

### Proposta C: Modale di conferma con riepilogo

Approccio minimale: il pulsante "Genera report" apre un `Modal` che mostra il riepilogo dei filtri applicati e le metriche aggregate, con due pulsanti: "Esporta" e "Annulla".

**Pro:**
- Minimo sforzo implementativo (il `Modal` esiste gia)
- Pattern "conferma prima di agire" chiaro

**Contro:**
- Il modale interrompe il flusso (l'utente deve chiuderlo per modificare i filtri)
- Non c'e possibilita di vedere i dati di dettaglio
- Pattern piu "dialog di conferma" che "dashboard analitica" — poco Stripe

### Raccomandazione

**Proposta A** (anteprima con dettaglio automatico). E il miglior equilibrio tra verifica dei filtri, velocita del flusso, e polish. Il riepilogo aggregato e piu utile di un semplice conteggio righe, e il dettaglio immediato elimina un click non necessario.

Il flusso diventa: Filtri -> Anteprima (riepilogo + dettaglio se righe > 0) -> Esporta XLSX. L'utente fa 2 click (Anteprima + Esporta) con feedback immediato e significativo.

---

## Riepilogo raccomandazioni

| Domanda | Raccomandazione | Motivazione principale |
|---------|----------------|----------------------|
| 1. Home / Navigazione | TabNavGroup con raggruppamento logico + Home con card compatte | Coerenza con le altre mini-app, pattern gia validato nel codebase |
| 2. AOV 4 tabelle | Tab interni con pannello stat card riassuntivo | Zero scroll, contesto globale mantenuto, estetica Stripe |
| 3. Export cieco | Anteprima con riepilogo aggregato + dettaglio automatico | Verifica filtri significativa, flusso progressivo, nessuna API aggiuntiva |

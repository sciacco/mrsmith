# Binocolo — M&A Strategy v2: brief evolutivo

> Obiettivo: evolvere la generazione della strategia M&A da prompt monolitico a pipeline controllata, mantenendo la flessibilita' del linguaggio naturale ma spostando default, validazioni e decisioni operative nel backend.
>
> Contesto attuale: `ma_strategy` trasforma direttamente la richiesta utente in `MAStrategySpec` vendor-ready, con tool per ATECO/province/surface probe e validazione Go a valle. Funziona, ma il prompt sta diventando il luogo in cui vivono troppe regole di processo.

## Principio guida

La richiesta utente puo' essere incompleta. Questo non e' un errore.

Binocolo deve distinguere esplicitamente tra:

1. **cio' che l'utente ha detto**;
2. **cio' che e' default operativo** (`activityStatus=ATTIVA`, `searchLimit=100`);
3. **cio' che e' derivato deterministicamente** dal backend/tool;
4. **cio' che non e' mappabile**;
5. **cio' che sarebbe solo un'ipotesi** e quindi non va inserito.

La pipeline non deve completare i vuoti con vincoli plausibili. Se la ricerca e' troppo ampia, deve chiedere raffinamento invece di inventare provincia, fatturato, dipendenti, forma legale o tesi.

## Problema da risolvere

Il prompt attuale fa insieme:

- interpretazione semantica della richiesta;
- produzione dei filtri vendor;
- selezione ATECO con `fit` (`core`, `weak`, `excluded`);
- gestione esclusioni;
- deduzione tesi M&A;
- normalizzazione implicita di legal form, fatturato, dipendenti, province;
- ragionamento sulla superficie della ricerca.

Questo aumenta il rischio che il modello:

- inventi vincoli non espressi per rendere la ricerca piu' concreta;
- produca JSON formalmente valido ma semanticamente ambiguo;
- usi `missingCriteria` per campi semplicemente non indicati;
- inserisca esclusioni nel testo positivo di settore;
- salti passaggi diagnostici come il surface probe.

La soluzione non dovrebbe essere aggiungere altre regole al prompt, ma ridurre il ruolo del prompt.

## Architettura proposta

```text
Richiesta utente
   ↓
LLM leggero: intent extraction
   ↓
Backend canonicalizer
   ↓
Resolver deterministici: province, legal forms, range, ATECO
   ↓
Strategy validator / lint semantico
   ↓
Surface probe backend automatico
   ↓
Strategia pronta oppure needs_refinement
```

### Ruolo del modello

Il modello deve estrarre intenzioni, non governare il processo.

Esempio di output intermedio:

```json
{
  "intent": {
    "sectorInclude": ["software gestionale"],
    "sectorExclude": [],
    "territoryText": "Lombardia",
    "turnover": null,
    "employees": null,
    "legalFormText": [],
    "thesis": "generico",
    "ownerAgeMin": null,
    "unmappedConstraints": []
  }
}
```

Regola centrale del prompt leggero:

> Estrai solo vincoli espressi o chiaramente deducibili dalla richiesta. Non completare i campi mancanti. I campi non menzionati restano `null`, `[]` o `generico`.

### Ruolo del backend

Il backend produce la `MAStrategySpec` finale:

- applica solo default operativi dichiarati;
- converte regioni/comuni/province in sigle reali tramite dati cacheati;
- converte legal form testuali in codici vendor tramite lookup/sinonimi;
- calcola range numerici deterministici, ad esempio `intorno a X` → `±30%`;
- genera `sectorDescription` solo dal perimetro positivo;
- tiene le esclusioni separate nei candidati ATECO con `fit=excluded`;
- normalizza enum e rifiuta valori ambigui invece di trasformarli silenziosamente in scelte forti;
- distingue `missingCriteria` da "campo non specificato".

## Comportamento atteso su richieste incomplete

### Esempio 1

Richiesta:

> Trova aziende software in Lombardia

Output corretto:

- settore: software;
- territorio: province lombarde;
- stato: `ATTIVA` default operativo;
- search limit: `100` default operativo;
- fatturato: `null`;
- dipendenti: `null`;
- legal forms: `[]`;
- thesis: `generico`;
- missing criteria: `[]`.

Non deve inventare fatturato, dimensione aziendale o tesi successione.

### Esempio 2

Richiesta:

> Aziende IT italiane acquisibili

Output corretto:

- settore: IT;
- territorio: nazionale / province vuote;
- thesis: probabilmente `generico`, salvo parole che indichino chiaramente successione/crescita/consolidamento/tuck-in;
- surface probe: probabilmente `too_broad`;
- stato UI/API: `needs_refinement`.

Il sistema deve suggerire quali vincoli aggiungere, non aggiungerli da solo.

### Esempio 3

Richiesta:

> Cooperative software in Piemonte, escluse consulenze generiche

Output corretto:

- legal form derivata da "cooperative" tramite lookup backend;
- Piemonte espanso a province reali;
- software come perimetro positivo;
- consulenze generiche come esclusione ATECO, non dentro `sectorDescription`;
- eventuali ambiguita' ATECO segnalate come da confermare.

## Prompt v2: direzione

Non un prompt piu' lungo, ma un prompt piu' stretto.

Sketch:

```text
Sei un estrattore di intenzioni per ricerche M&A su aziende italiane.
Restituisci solo JSON conforme allo schema richiesto.
Estrai soltanto vincoli presenti o chiaramente deducibili dalla richiesta.
Non inventare provincia, fatturato, dipendenti, forma legale, tesi o eta' proprietario.
I campi non indicati restano null, [] o generico.
Le condizioni espresse ma non traducibili in filtri verificabili vanno in unmappedConstraints.
```

Il resto va nello schema e nel backend.

## Schema intermedio suggerito

```json
{
  "intent": {
    "title": "string|null",
    "sectorInclude": ["string"],
    "sectorExclude": ["string"],
    "territoryText": "string|null",
    "turnover": {
      "mode": "around|minmax|min|max|null",
      "around": null,
      "min": null,
      "max": null
    },
    "employees": {
      "min": null,
      "max": null
    },
    "legalFormText": ["string"],
    "activityStatusText": "string|null",
    "thesis": "successione|crescita|consolidamento|tuck_in|generico",
    "ownerAgeMin": null,
    "signalEmphasis": ["string"],
    "unmappedConstraints": ["string"]
  }
}
```

Nota: per i campi che diventano filtri forti si puo' valutare un attributo `evidence`/`sourceText`, cosi' il backend puo' rifiutare vincoli privi di evidenza nella richiesta. Questo andrebbe usato con parsimonia per non complicare troppo il contratto.

## Canonicalizer backend

Il canonicalizer traduce `intent` in `MAStrategySpec`.

Responsabilita':

- `activityStatus`: default `ATTIVA`, salvo richiesta esplicita diversa;
- `searchLimit`: default `100`, salvo richiesta esplicita diversa;
- territorio: risoluzione deterministica con cache province OpenAPI.it;
- legal forms: mappa sinonimi/lookup, non delegata al modello;
- fatturato/dipendenti: range numerici solo se espressi;
- thesis: enum normalizzato; fallback `generico`;
- `missingCriteria`: solo condizioni espresse ma non mappabili;
- `sectorDescription`: costruito dal perimetro positivo;
- esclusioni: candidate ATECO separate con `fit=excluded`.

## ATECO resolver

La selezione ATECO dovrebbe diventare una fase separata.

Proposta:

1. il modello estrae `sectorInclude` e `sectorExclude` testuali;
2. il backend cerca candidati nel DB ATECO 2025;
3. se serve semantica, un mini-step LLM sceglie/classifica solo tra candidati restituiti;
4. il backend valida e canonicalizza;
5. valori non validi vengono rifiutati o mandati a repair, non trasformati in default forti.

Invariante: il modello non puo' introdurre codici ATECO non restituiti dal resolver.

## Surface probe

Il probe non dovrebbe essere una decisione del modello.

Il backend dovrebbe eseguire un probe automatico sulla strategia finale, idealmente cache-backed e con costo/latency controllati.

Esiti:

- `ready`: strategia praticabile;
- `needs_refinement`: superficie troppo ampia;
- `invalid`: vincoli incoerenti o non risolvibili.

Se `needs_refinement`, il sistema non modifica la strategia inventando vincoli. Restituisce invece suggerimenti di raffinamento:

```json
{
  "status": "needs_refinement",
  "reason": "too_broad",
  "suggestedRefinements": ["territorio", "range fatturato", "dipendenti", "settore piu' specifico"]
}
```

## UX proposta

La UI dovrebbe rendere visibile la provenienza dei parametri:

- **Utente**: estratto dalla richiesta;
- **Default operativo**: applicato dal sistema;
- **Derivato**: risolto da backend/tool;
- **Da confermare**: ambiguo;
- **Non mappabile**: espresso ma non traducibile.

Quando la strategia e' `needs_refinement`, mostrare una richiesta guidata:

- "La ricerca e' troppo ampia";
- "Aggiungi almeno uno tra: territorio, fatturato, dipendenti, settore piu' specifico";
- chip/modifica rapida per aggiornare la richiesta o i parametri.

## Migrazione incrementale

### Fase 1 — Intent extraction parallela

- Aggiungere nuovo scope prompt, ad esempio `ma_strategy_intent`.
- Salvare/mostrare l'intent in trace o nello strategy JSON come campo opzionale.
- Mantenere `MAStrategySpec` come contratto finale verso stima/esecuzione.

### Fase 2 — Canonicalizer deterministico

- Implementare mapping territory/legal forms/range/thesis.
- Spostare i default dal prompt al backend.
- Aggiungere lint semantici: niente vincoli forti senza origine, niente esclusioni in `sectorDescription`.

### Fase 3 — ATECO resolver separato

- Separare ricerca candidati e classificazione `fit`.
- Rendere unknown `fit` un errore/repair, non fallback a `core`.

### Fase 4 — Probe backend automatico

- Eseguire il probe dopo la canonicalizzazione.
- Restituire `ready` vs `needs_refinement`.
- UI con refinement loop.

### Fase 5 — Deprecazione prompt monolitico

- Ridurre o rimuovere il vecchio prompt `ma_strategy` come generatore diretto vendor-ready.
- Conservare compatibilita' dei dati persistiti leggendo sempre `MAStrategySpec` finale.

## Repo-fit

- Nessun nuovo mini-app.
- Nessuna nuova variabile ambiente prevista.
- Usa registry LLM esistente (`mrsmith.llm_model`, `mrsmith.llm_prompt`) con nuovo scope o nuova versione prompt.
- Usa trace esistente `binocolo.ma_operation_trace` per registrare intent, canonicalizzazione, resolver, probe e outcome.
- Mantiene `MAStrategySpec` come formato finale per `estimate`/`execute`.
- Eventuali nuove tabelle sono opzionali: si puo' partire migration-free salvando l'intent nello strategy JSON o solo nel trace.

## Verifica proposta

Da approvare prima di aggiungere test, secondo la regola del repo.

Test utili:

- richiesta vaga non genera filtri non espressi;
- campi non menzionati non finiscono in `missingCriteria`;
- legal form testuale esplicita viene mappata o segnalata;
- `fit` sconosciuto non diventa `core` silenziosamente;
- esclusioni non entrano in `sectorDescription`;
- strategia troppo ampia produce `needs_refinement`, non vincoli inventati;
- estimate/execute continuano a leggere `MAStrategySpec` finale come oggi.

## Questioni aperte

- Salvare l'intent in modo persistente o solo in trace?
- Fare il probe automatico gia' alla creazione sessione o solo alla stima?
- Quanto deve essere deterministico il ranking ATECO rispetto al mini-step LLM?
- Come esporre il refinement loop: modale guidata, editor strategia, o nuova interazione conversazionale?
- Le soglie `too_broad`/successo ATECO devono restare costanti o diventare parametri configurabili?

## Sintesi

La direzione consigliata e':

> prompt piu' piccolo, backend piu' responsabile, UI piu' trasparente.

Il modello interpreta. Il backend decide cosa e' valido. Il probe diagnostica. L'utente raffina quando serve.

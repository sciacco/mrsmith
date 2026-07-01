# Binocolo — M&A Strategy v2: brief evolutivo

## Stato del brainstorming — aggiornamento 2026-06-27

> Questa sezione riflette le conclusioni della sessione di brainstorming e **corregge alcune premesse della prima stesura** (conservata qui sotto). In caso di conflitto, vale questa sezione.
>
> Piano operativo derivato: [`MA-STRATEGY-V2-IMPLEMENTATION-PLAN.md`](./MA-STRATEGY-V2-IMPLEMENTATION-PLAN.md).

### Premessa corretta
- Lo scope `ma_strategy` gira **oggi su `openai/gpt-5.5`** (frontier, `max_tokens 6000`, temp 0), **non** sul Flash Lite del seed (migration 026): il registry live `mrsmith.llm_model` diverge dal seed migration.
- Cronologia: Flash Lite **ha fallito subito** sul prompt monolitico → adozione immediata del modello più potente disponibile per non correre rischi.
- Distinzione-cardine: il fallimento di Flash Lite è **sul monolite** (interpretazione + 4 round agentici + fit ATECO + esclusioni + tesi + ragionamento sulla superficie, tutto insieme). Non dimostra nulla su un modello piccolo su un compito **stretto** di sola estrazione. Il fallimento era *task-shaped, non model-shaped*.

### Obiettivo riformulato
L'obiettivo del piano è **uno solo e a basso rischio**: **rimuovere il loop agentico** (fino a 4 round su GPT-5.5 lento) e **de-duplicare la logica già presente in Go**, sostituendo il monolite con una pipeline non-agentica. È una vittoria garantita di costo e latenza, **indipendente dalla taglia del modello** e ottenuta **già sul modello seedato (GPT-5.5)**.

Il **downgrade del modello non è obiettivo del piano**: una volta che la pipeline è model-agnostic, quale modello gira su ogni scope è una **decisione operativa** = una riga in `mrsmith.llm_model`, zero codice. Si abilita **dopo**, con il traffico reale (vedi *Validazione senza dati* e *Due responsabilità*). Il fallimento di Flash Lite resta l'ipotesi da rimisurare *quando ci saranno i dati*, non un lavoro di questo piano.

### Architettura su cui si converge
`estrattore stretto con span obbligatori → resolver/canonicalizer deterministico (già in Go) → resolver ATECO separato con rerank constrained → MAStrategySpec → probe backend a valle (in estimate/execute)`

- `MAStrategySpec` resta il **contratto finale stabile** (retro-compat con estimate/execute); l'intent (`MAIntent`) è intermedio e vive **solo nel trace** _(deciso)_.
- La correttezza è retta da **guardrail by construction** (span obbligatori + `validate`/`canonicalize(requireAllowed=true)`), **non** da un gate statistico: senza dati reali non esiste un corpus su cui misurare (vedi sotto).
- Pipeline **model-agnostic**: ogni step LLM risolve modello+prompt dal registry per scope (`ma_strategy_intent`, `ma_strategy_ateco`). La taglia del modello è fuori scope (operativa).

### Validazione senza dati
**Non abbiamo dati reali.** Niente gold set, niente eval-harness statistico, niente shadow/replay: non c'è un corpus su cui scorare. La validazione ha due livelli:

1. **Guardrail by construction** (a runtime, sempre attivi, **dentro il DoD dell'agent**): `sourceText` obbligatorio su **ogni vincolo** che influenza ricerca/esclusione/scoring + validazione span ⇒ **invenzione di vincoli senza evidenza testuale impedita by construction** (non garantisce l'interpretazione semantica corretta); `validate`/`canonicalize(requireAllowed=true)` ⇒ nessuna spec invalida servita; broadness **non mascherata** (emerge alla stima, non a draft-time).
2. **Prova a mano** su poche richieste rappresentative, a occhio — **a carico umano e post-scrittura** (richiede registry seedato + LLM live).

Lo `sourceText`/span resta **guardrail e manopola** (severità: più stretta → meno invenzione, più rischio di droppare il reale); **non** è più presentato come "metrica" di un harness assente. L'**eval-harness del brief è rinviato a dopo il lancio**, quando V2 genera traffico: solo allora gold/score/confronto-modelli hanno dati su cui esistere. Non è deliverable n.1; non c'è una "Fase 0" prima del traffico.

### Latenza
- Il critical path è **2 hop LLM sequenziali** per dipendenza dati (l'ATECO ha bisogno del settore estratto sui settori a testo): i due step non si parallelizzano tra loro.
- La vittoria grossa sui tempi è il **loop-removal** (fino a 4 round → 1 estrazione + 1 rerank ATECO constrained), ottenuta **già su GPT-5.5**. La parallelizzazione dei resolver deterministici è secondaria.
- **Nessuno skip dell'LLM su retrieval ATECO "netto":** la scelta da settore a testo (fit/esclusioni) non è deterministica; l'unico path senza LLM è quello a **codici ATECO espliciti**. _(valutato e scartato)_
- Pattern operativi tipo escalation/hedging tra modelli **non sono materia del piano**: vivono nel registry, eventualmente post-traffico.

### Due responsabilità: agent vs umano
Il piano operativo separa nettamente (confine = «codice scritto + build/test verdi»):
- **Agent**: scrive codice + il **file** di migration, fino a `go build`/`go vet`/`tsc` verdi; ogni deliverable ha un **DoD** verificabile **senza dati e senza toccare il DB condiviso**.
- **Umano (post-scrittura)**: applica la migration al DB condiviso, verifica il registry live, fa lo smoke a mano **in lettura** sul Mistra remoto, esegue il cut-over (kill-switch `ma_strategy_pipeline` in `ma_parameter`), osserva il traffico, cancella il monolite e — solo allora, con dati — costruisce l'harness.

Dettaglio in [`MA-STRATEGY-V2-IMPLEMENTATION-PLAN.md`](./MA-STRATEGY-V2-IMPLEMENTATION-PLAN.md).

### Già presente nel codice (NON ricostruire)
- Il canonicalizer è in gran parte **già in Go**: province (dedup/normalize/validate), ATECO (resolve descrizione+search code su DB, divisioni, subtree discovery cap 60), turnover ±30% (`turnoverAroundRange`, `ma_rules.go`), default `ATTIVA`/`searchLimit=100` (`validateMAStrategy`), thesis, legal forms, signal weights. ⇒ l'azione reale è **sottrarre dal prompt i doppioni che il Go già rifà a valle**, non "costruire il backend".
- Il **probe è già backend-automatic** in estimate/execute (`estimateJobWork`, `runExecution`); è ridondante come tool del modello in drafting → toglierlo dai tool.
- L'**ATECO ha già un resolver** (tool `search_ateco_2025` restituisce solo codici validi, il Go valida su DB con `requireAllowed`); il cambio è renderlo **fase separata non-agentica**, non agentica nel loop.
- Il **trace** (`binocolo.ma_operation_trace` + eventi) cattura già ogni draft (prompt completo, round, tool, token leggibili dopo il fix redazione del 2026-06-17); è **write-only, nessun read path**.

### Decisioni prese (erano fili aperti)
- **Intent**: persistito **solo nel trace** (non in tabella dedicata, non nello strategy JSON).
- **`needs_refinement`/broadness**: V2 **non** maschera né inventa restringimenti; la superficie troppo ampia emerge **alla stima** via `too_broad`/`surfaceStatus` (il probe sta a valle), non a draft-time.
- **Probe timing**: alla stima, non alla creazione sessione (per non pagare ricerche abbandonate).
- **Rerank ATECO**: sempre constrained sui candidati per i settori a testo; deterministico **solo** per codici espliciti. Nessuno skip dell'LLM sul retrieval netto.
- **Vincoli oltre la ricerca**: l'intent li cattura **tutti** con la loro disposizione; i due economici con dato disponibile (ricavo/dipendente, max soci) diventano **post-filtri di scoring costruiti in V2**; EBITDA resta follow-on (solo nel deep); OCF/gruppi/estere restano `missingCriteria` finché non c'è estrazione dati nuova.

### Fili ancora aperti (non di prodotto)
- **Recall del retrieval ATECO**: se il traffico reale mostra che il codice giusto resta fuori dai candidati, la leva è sostituire il **solo** retrieval con embedding (colonna Postgres + cosine in Go) — **contingenza evidence-gated, non pianificata**.
- **Thesis quasi-deterministica** (keyword-driven) → eventuale follow-on per toglierla in gran parte dall'LLM.
- **UX di provenienza** (Utente/Default/Derivato/Da confermare/Non mappabile) e **severità dello span** lato utente → follow-on, dopo il core.

---

> Obiettivo: evolvere la generazione della strategia M&A da prompt monolitico a pipeline controllata, mantenendo la flessibilita' del linguaggio naturale ma spostando default, validazioni e decisioni operative nel backend.
>
> Contesto attuale: `ma_strategy` trasforma direttamente la richiesta utente in `MAStrategySpec` vendor-ready, con tool per ATECO/province/surface probe e validazione Go a valle. Funziona, ma il prompt sta diventando il luogo in cui vivono troppe regole di processo.
>
> **Aggiornato:** gira oggi su `openai/gpt-5.5` (frontier) con **loop agentico fino a 4 round**; una parte rilevante di validazione/canonicalizzazione (province, ATECO, range, default) **è già in Go**. Vedi *Stato del brainstorming* sopra.

> _Nota di lettura — la prima stesura qui sotto è conservata come storico. Dove confligge con lo Stato del brainstorming sopra, vale lo Stato; i punti riconciliati riportano un callout **Aggiornato**._

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

> **Aggiornato:** il "Backend canonicalizer" e i "Resolver deterministici" **non sono greenfield**: province, ATECO (resolve/divisioni/subtree), range `±30%`, default e normalizzazione enum sono **già in Go**. L'`LLM leggero` diventa **due step** (estrazione `ma_strategy_intent` + rerank ATECO constrained `ma_strategy_ateco` sui settori a testo); la pipeline è **model-agnostic**, la taglia del modello è operativa. La correttezza è retta da **guardrail by construction** (span + `validate`/`canonicalize`), **non** da un gate statistico: senza dati reali l'eval-harness è rinviato a dopo il lancio (vedi *Validazione senza dati*).

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

> **Aggiornato:** quasi tutta questa lista **esiste già in Go** (province, `turnoverAroundRange` per `±30%`, default `ATTIVA`/`searchLimit=100`, `sectorDescription`/esclusioni via `fit`, normalizzazione enum). L'azione della v2 qui è **togliere dal prompt i doppioni**, non implementare il backend da zero. In più: per i filtri forti il backend **rifiuta i vincoli privi di `sourceText`** (vedi Schema).

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

> **Aggiornato — decisione presa:** lo `sourceText` **non è un'opzione "con parsimonia": è il meccanismo centrale**. È **obbligatorio su ogni vincolo** che influenza ricerca/esclusione/scoring (settore include/exclude, `atecoExplicit`, territorio include/exclude, turnover, dipendenti, legal form, ownerAge, status≠default, ricavo/dipendente, max soci), e il backend valida che lo span esista davvero nella richiesta → **invenzione di vincoli senza evidenza testuale impedita by construction** (non garantisce l'interpretazione semantica). È insieme **guardrail runtime** (rifiuto del vincolo senza span) e **manopola** da tarare (**severità dello span**: più stretta → meno invenzione, più rischio di droppare un vincolo reale). Diventa anche *metrica* solo quando l'eval-harness esisterà (post-traffico); oggi non c'è gold set su cui tararlo. Vedi *Validazione senza dati*.

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

> **Aggiornato:** questo canonicalizer è **in larga parte già implementato** (`validateMAStrategy`/`canonicalizeMAStrategy*` in `ma_rules.go`/`ma_service.go`). La v2 lo **estende**, non lo crea: aggiunge la validazione degli span (`sourceText`), la distinzione esplicita "non specificato" vs `missingCriteria`, e la rimozione dei doppioni dal prompt.

## ATECO resolver

La selezione ATECO dovrebbe diventare una fase separata.

Proposta:

1. il modello estrae `sectorInclude` e `sectorExclude` testuali;
2. il backend cerca candidati nel DB ATECO 2025;
3. se serve semantica, un mini-step LLM sceglie/classifica solo tra candidati restituiti;
4. il backend valida e canonicalizza;
5. valori non validi vengono rifiutati o mandati a repair, non trasformati in default forti.

Invariante: il modello non puo' introdurre codici ATECO non restituiti dal resolver.

> **Aggiornato:** l'invariante e il pattern resolver **esistono già** (tool `search_ateco_2025` restituisce solo codici validi, il Go valida su DB). Precisazioni dal brainstorming:
> - il **collo di bottiglia vero non è il modello ma il recall dei candidati**: nessun LLM sceglie un codice che la ricerca non gli mostra → contingenza evidence-gated (embedding sulle descrizioni ATECO 2025) **se** il traffico reale lo mostra, non pianificata;
> - il rerank LLM (punto 3) è **sempre constrained sui candidati per i settori a testo**: la scelta fit/esclusioni non è deterministica. **Nessuno skip dell'LLM su retrieval "netto"** (valutato e scartato); l'unico path senza LLM è quello a **codici ATECO espliciti** forniti dall'utente.

## Surface probe

Il probe non dovrebbe essere una decisione del modello.

Il backend dovrebbe eseguire un probe automatico sulla strategia finale, idealmente cache-backed e con costo/latency controllati.

> **Aggiornato:** il probe è **già backend-automatic** in estimate/execute (`estimateJobWork`, `runExecution`) e **già parallelizzato** nell'async job. La modifica della v2 è quindi una **sottrazione**: toglierlo dai tool che il modello chiama durante il drafting (oggi ridondante). Timing orientato: **alla stima, non alla creazione sessione** (per non pagare €0.01/probe su ricerche poi abbandonate); la decisione finale è tra i fili aperti.

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

> **Aggiornato — sequenza rivista (vince il piano operativo).** Le Fasi 1-5 originali restano valide come scomposizione concettuale, ma la sequenza eseguibile è quella del piano: **Passo 0 (scaffolding registry + kill-switch) → 1 (extractIntent) → 2 (resolveIntent + ATECO) → 3 (cut-over: V2 sostituisce il monolite) → 4 (post-filtri economici)**. **Non c'è una "Fase 0 = eval-harness" prima del traffico**: senza dati reali l'harness non ha un corpus su cui esistere ed è rinviato a dopo il lancio (vedi *Validazione senza dati* e *Due responsabilità*). La validazione del core è **guardrail by construction + prova a mano**.

### Fase 0 — ~~Eval-harness (prerequisito)~~ → rinviata a post-lancio

L'eval-harness **non è più la Fase 0**. Cos'era e perché si rinvia: set fisso di richieste + spec gold → score automatico vs gold (diff strutturale, niente LLM-giudice perché il canonicalizer è deterministico) → confronto fra modelli. Richiede però **dati reali che oggi non esistono**. Si costruisce **dopo** che V2 genera traffico, ed è ciò che renderà *misurato* l'eventuale downgrade del modello (oggi: decisione operativa, non pianificata). Componenti, quando si farà: read-layer sopra il trace, gold set a due livelli (invenzione + fedeltà + span), listino token per-modello in `ma_parameter`.

### Fase 1 — Intent extraction parallela

- Aggiungere nuovo scope prompt, ad esempio `ma_strategy_intent`.
- Salvare/mostrare l'intent in trace o nello strategy JSON come campo opzionale.
- Mantenere `MAStrategySpec` come contratto finale verso stima/esecuzione.

### Fase 2 — Canonicalizer deterministico

- Implementare mapping territory/legal forms/range/thesis.
- Spostare i default dal prompt al backend.
- Aggiungere lint semantici: niente vincoli forti senza origine, niente esclusioni in `sectorDescription`.

> **Aggiornato:** territory/range/thesis/default **sono già in Go** → questa fase è in larga parte **"togliere dal prompt"**, non "implementare nel backend". Il lavoro nuovo reale è: validazione `sourceText`, lint "non specificato vs `missingCriteria`", e (opzionale) thesis quasi-deterministica via keyword per toglierla dall'LLM.

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

> **Aggiornato:** scope distinti = più righe `llm_model` (es. `ma_strategy_intent`, `ma_strategy_ateco`), già supportato dal registry; quale modello vi gira è operativo, non scope del piano. L'**eval-harness** (rinviato a post-lancio) non richiederebbe comunque tabelle nuove obbligatorie: read-layer sopra le tabelle trace esistenti + listino token per-modello in `ma_parameter` (oggi ha solo i prezzi vendor OpenAPI.it).

## Verifica proposta

Da approvare prima di aggiungere test, secondo la regola del repo.

> **Aggiornato:** senza dati reali la verifica primaria della v2 è **guardrail by construction** (span obbligatori + `validate`/`canonicalize`) **+ prova a mano** su poche richieste (livello umano, post-scrittura). Gli unit test coprono la logica deterministica (canonicalize, `validateIntentSpans`, post-filtri economici), da proporre quando proteggono una regola di business o un bug riprodotto. I casi qui sotto diventeranno voci del **gold set dell'eval-harness solo quando ci sarà traffico** per costruirlo (post-lancio).

Test utili (→ casi del futuro gold set / candidati unit test):

- richiesta vaga non genera filtri non espressi;
- campi non menzionati non finiscono in `missingCriteria`;
- legal form testuale esplicita viene mappata o segnalata;
- `fit` sconosciuto non diventa `core` silenziosamente;
- esclusioni non entrano in `sectorDescription`;
- strategia troppo ampia produce `needs_refinement`, non vincoli inventati;
- estimate/execute continuano a leggere `MAStrategySpec` finale come oggi.

## Questioni aperte

> Le decisioni prese durante il brainstorming sono in *Stato del brainstorming → Decisioni prese*: intent trace-only; probe alla stima; rerank ATECO sempre constrained sui candidati (no skip), deterministico solo su codici espliciti; broadness che emerge alla stima; vincoli oltre la ricerca catturati + dati disposizione. Quelle qui sotto restano fuori dal core del piano:

- Come esporre il refinement loop e la **provenienza/severità dello span** in UI (modale guidata, editor, chip): **follow-on**, dopo il core.
- Le soglie `too_broad`/successo ATECO costanti o configurabili: aperto, non bloccante per il core.
- Quale modello gira su ogni scope (ed eventuali pattern escalation/hedging): **decisione operativa** (riga `llm_model`), fuori dal piano; guidata dall'harness quando esisterà.

## Sintesi

La direzione consigliata e':

> prompt piu' piccolo, backend piu' responsabile, UI piu' trasparente.

Il modello interpreta. Il backend decide cosa e' valido. Il probe diagnostica. L'utente raffina quando serve.

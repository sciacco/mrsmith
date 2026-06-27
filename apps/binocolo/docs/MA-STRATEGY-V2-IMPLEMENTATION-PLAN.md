# Binocolo — M&A Strategy v2: piano di implementazione

> Companion operativo di [`MA-STRATEGY-V2-BRIEF.md`](./MA-STRATEGY-V2-BRIEF.md). Il brief è il "perché"; questo è il "come", ancorato alle funzioni reali. Da verificare prima della progettazione/implementazione.

## Principio di esecuzione

- **V2 è la nuova implementazione e sostituisce il monolite** nel path servito. Il monolite **non co-gira** (manterrebbe vivo il loop lento e peggiorerebbe i tempi). Resta come **kill-switch dormiente** (flag default off, non eseguito) per il primo deploy, poi si cancella.
- **Non abbiamo dati reali.** Niente gold set, niente eval-harness statistico, niente shadow/replay: non esiste un corpus su cui misurare. L'harness del brief è **rinviato** a dopo il lancio, quando V2 genera traffico.
- Destrutturare **non riscrive** ATECO, province o probe: sono già funzioni Go autonome. Destrutturare = **chiamarle dal backend in ordine fisso** invece di lasciarle orchestrare al loop agentico. I `canonicalize*` restano identici.

### Due responsabilità: agent vs umano

Il piano ha due esecutori distinti. La linea di confine è **«codice scritto + build/test verdi»**.

- **A carico dell'agent LLM incaricato** (sezione [«Lavoro dell'agent»](#lavoro-dellagent-deliverable--dod)): scrivere il codice Go/TS e il **file** di migration, far passare `go build`/`go vet`/`tsc` e gli eventuali unit test approvati. Ogni deliverable ha un **DoD** verificabile **senza dati e senza toccare il DB condiviso** (compila, firma corretta, guardrail rifiuta l'input di prova in-process). L'agent **non applica** migration, **non scrive** sul DB remoto, **non fa** il cut-over.
- **A carico umano** (sezione [«A carico umano»](#a-carico-umano-dopo-scrittura--buildtest-verdi)): tutto ciò che parte **dopo** che il codice è scritto e compila — applicare la migration al DB condiviso, seed/verifica registry live, smoke a mano in lettura sul Mistra remoto, flip del kill-switch / cut-over, osservazione del traffico, cancellazione del monolite, e — solo a quel punto, con dati — l'harness del brief.

Conseguenza pratica: il DoD dell'agent **non** include «la pipeline produce intent sensati su richieste reali» (richiede registry seedato + LLM live + giudizio umano → bucket umano). Include solo invarianti che il codice impone da sé.

### Come si valida (data-free)

Due livelli, entrambi senza dati:

1. **Guardrail by construction** — a runtime, sempre attivi:
   - `sourceText` obbligatorio su **ogni vincolo** che influenza ricerca/esclusione/scoring (settore include/exclude, `atecoExplicit`, territorio include/exclude, turnover, dipendenti, legal form, ownerAge, status≠default, ricavo/dipendente, max soci) + validazione span → **invenzione di vincoli senza evidenza testuale impedita by construction** (non garantisce l'interpretazione semantica corretta — vedi "Cosa non è garantito");
   - `validateMAStrategy` + `canonicalizeMAStrategy{Provinces,Ateco}(requireAllowed=true)` → **nessuna spec invalida servita**;
   - V2 **non maschera** la broadness (non inventa restringimenti); la superficie troppo ampia emerge **alla stima** via il meccanismo esistente `too_broad`/`surfaceStatus`, non a draft-time (il probe sta a valle).
2. **Prova a mano** su **poche** richieste rappresentative (qualche caso ovvio + un paio di trappole), guardando l'output a occhio. **Nessun set formale (no 20–40), nessun runner, nessun tool di ispezione.** Questo livello è **a carico umano e post-scrittura**: richiede registry seedato + LLM live, quindi **non** rientra nel DoD dell'agent (vedi [«A carico umano»](#a-carico-umano-dopo-scrittura--buildtest-verdi)).

Livello 1 è l'unico **dentro il DoD dell'agent** (lo impone il codice). Con zero dati questo è il tetto onesto: la correttezza si spinge negli **invarianti che il codice impone**, non in eval che i dati dovrebbero provare.

### Ambito sul modello

Il piano consegna V2 **model-agnostico**: ogni step LLM risolve modello+prompt dal registry per scope (`ma_strategy_intent`, `ma_strategy_ateco`). **Quale modello si usa, e quando cambiarlo, non è materia di questo piano**: è una decisione operativa = una riga in `mrsmith.llm_model`, zero codice. La guiderà l'harness quando esisterà (post-traffico), o nel frattempo il giudizio sugli esempi a mano. Il guadagno di latenza arriva già dalla **struttura** (kill del loop), sul modello seedato; il seed iniziale degli scope è GPT-5.5, ma è solo un valore di partenza sostituibile.

## Mappa del codice attuale (verificata)

| Elemento | Posizione |
|---|---|
| Entry drafting | `draftStrategy` — `ma_service.go:1144` |
| Loop agentico | `for round … <= maMaxToolRounds` — `ma_service.go:1201` (`maMaxToolRounds=4`, `:46`) |
| Tool registrati | `[]llm.Tool{maAtecoSearchTool(), maProvinceRegionTool(), maCompanySurfaceProbeTool()}` — `ma_service.go:1197` |
| Istruzioni tool | `maStrategyToolInstructions()` — `ma_service.go:1378` |
| Dispatch tool | `executeMAStrategyTool` — `ma_service.go:1526` |
| Impl ATECO | `executeMAAtecoTool` → `s.ateco.SearchAteco` — `ma_service.go:1539`/`:1547` |
| Impl province | `executeMAProvinceRegionTool` → `listProvincesWithCache` — `ma_service.go:1561`/`:1566` |
| Impl probe | `executeMACompanySurfaceTool` → `s.probeMASearchSurface` — `ma_service.go:1624`/`:1670` |
| Decode + validate | `decodeMAStrategyResponse` → `validateMAStrategy` — `ma_service.go:1356` / `ma_rules.go:21` |
| Canonicalize province | `canonicalizeMAStrategyProvinces(…, requireAllowed=true)` — `ma_service.go:1738` (chiamata `:1299`) |
| Canonicalize ATECO | `canonicalizeMAStrategyAteco(…, requireAllowed=true)` — `ma_service.go:1771` (chiamata `:1316`) |
| Map allowed (gate) | `allowedAteco`/`allowedProvinces` — `ma_service.go:1195-1196`, popolati da `rememberAllowed*` (`:1712`/`:1724`) |
| Contratto finale | `MAStrategySpec` — `ma_types.go:266-317` |
| Scope modello | `maModelScopeStrategy = "ma_strategy"` — `ma_types.go:94` |

Punto chiave: in V2 il backend popola **da sé** `allowedAteco`/`allowedProvinces` chiamando `SearchAteco`/`listProvincesWithCache`, poi richiama gli **stessi** `canonicalize*` con `requireAllowed=true`. La canonicalizzazione non cambia.

## Forma target

```
extractIntent (1 call LLM, no tool, JSON) → MAIntent + sourceText
   ↓
validateIntentSpans (guardrail: filtro forte senza span valido → scartato)
   ↓
resolveIntent (backend, deterministico salvo il fit ATECO sui settori a testo):
   • territory STRUTTURATO {includeRegions, includeProvinces, excludeProvinces}
       → espandi regioni (listProvincesWithCache) ∪ includeProvinces, MENO excludeProvinces → Provinces
   • ATECO espliciti (atecoExplicit) → valida su DB. Settori a testo → SearchAteco → LLM constrained sceglie+classifica fit
   • legalForms / range / default / thesis
   • vincoli oltre la ricerca → catturati nell'intent; i 2 economici (ricavo/dip, max soci) popolano campi spec → applicati a valle (scoring); gli altri solo disposizione
   ↓
validateMAStrategy + canonicalizeMAStrategy{Provinces,Ateco}(requireAllowed=true)  ← invariati
   ↓
MAStrategySpec  → estimate/execute invariati
```

Il loop sparisce: i tre tool diventano una chiamata interna (province), una chiamata interna + LLM constrained (ateco, per i settori a testo), niente (probe, già a valle in estimate/execute).

---

## Lavoro dell'agent (deliverable + DoD)

Tutto in questa sezione è **a carico dell'agent**: scrivere codice e file di migration, fino a build/test verdi. Ogni DoD è verificabile **in-process, senza dati e senza toccare il DB condiviso**. Ciò che viene dopo è in [«A carico umano»](#a-carico-umano-dopo-scrittura--buildtest-verdi).

### Passo 0 — Scaffolding registry (prerequisito dei Passi 1–4)

**Deliverable (agent):** il **file** di migration — non la sua applicazione.
- `deploy/migrations/NNN_*.sql`: seed in `mrsmith.llm_model` + `mrsmith.llm_prompt` di:
  - `ma_strategy_intent` — prompt estrattore stretto + modello **inizialmente `openai/gpt-5.5`** (isola la struttura dal downgrade);
  - `ma_strategy_ateco` — prompt rerank constrained + modello inizialmente `openai/gpt-5.5`.
- Seed anche il **kill-switch** in `binocolo.ma_parameter`: `ma_strategy_pipeline = v2|monolith`, default `v2`.
- Nessun nuovo mini-app, nessuna nuova env.

**DoD (agent):**
- Il file `.sql` esiste in `deploy/migrations/`, segue la numerazione progressiva del repo, e parsa (`psql --no-psqlrc -f … ` in dry/transaction-rollback **non** è compito dell'agent: niente DB condiviso → verifica solo sintattica/visiva).
- Upsert **idempotenti** (rieseguibili) per i due scope registry e per la riga `ma_parameter`.
- Il file **non** viene applicato dall'agent (DB owned dal team → bucket umano).

### Passo 1 — `extractIntent` + `MAIntent` + `validateIntentSpans`

**Deliverable (agent):**
- Nuovo tipo `MAIntent` (`ma_types.go`): schema intent **corretto** rispetto al brief — territorio **strutturato** (`{includeRegions, includeProvinces, excludeProvinces}`), `atecoExplicit` distinto dai settori a testo, e cattura **strutturata** dei vincoli oltre la ricerca (finanziari/ownership/esclusioni con disposizione). `sourceText` su **ogni vincolo** che influenza ricerca/esclusione/scoring: settore include/exclude, `atecoExplicit`, territorio include/exclude, turnover, dipendenti, legal form, ownerAge, status≠default, ricavo/dipendente, max soci. (Per atecoExplicit/territorio il `canonicalize(requireAllowed)` scarta già i valori *inesistenti*; lo span chiude il buco dei valori *reali ma non richiesti*.)
- `extractIntent(ctx, prompt, …) (MAIntent, llm.CallAudit, error)`: **1 sola** `client.Chat`, scope `ma_strategy_intent`, `ResponseFormat=json_object`, **`Tools: nil`**, **nessun loop**.
- `validateIntentSpans(intent, promptText)`: se `sourceText` non fa substring-match (case/spazi-insensitive) della richiesta → campo scartato/declassato. **Vincoli senza evidenza testuale impediti by construction** (la mis-interpretazione semantica resta possibile).
- Testo del prompt estrattore: vive nella migration del Passo 0 (registry), non hard-coded.

**DoD (agent):**
- `MAIntent` compila con i campi sopra; ogni vincolo forte porta il suo `sourceText`.
- `extractIntent` fa **esattamente una** `client.Chat` con `Tools: nil` e `ResponseFormat=json_object`, risolve modello+prompt da scope `ma_strategy_intent`, ritorna un `llm.CallAudit` proprio; **nessun** ciclo di tool-round nel corpo.
- `validateIntentSpans` scarta/declassa, su input in-process, un vincolo forte il cui `sourceText` non è substring della richiesta (verificabile con un caso costruito a mano nel test, non con traffico reale).
- `go build ./... && go vet ./...` verdi.
- _Fuori DoD (umano):_ giudicare se gli intent estratti su richieste reali sono semanticamente corretti.

### Passo 2 — `resolveIntent` + assemble (ATECO + cattura vincoli)

**Deliverable (agent):**
- `resolveIntent(ctx, intent) (MAStrategySpec, error)`:
  - **territorio strutturato** `{includeRegions, includeProvinces, excludeProvinces}`: espandi le regioni via `listProvincesWithCache`, unisci `includeProvinces`, **sottrai** `excludeProvinces` (differenza di insiemi; sigle uniche a livello nazionale) → `Provinces` + `allowedProvinces`. Lint: una `excludeProvince` fuori dalle regioni incluse è sospetta; normalizza i nomi regione (accenti/trattini);
  - ATECO: **deterministico solo se l'utente dà codici espliciti** (valida su DB). Se dà settori a testo → recupera candidati con `s.ateco.SearchAteco` (retriever lessicale ibrido già esistente: match codice + `titolo LIKE` + FTS italiano + trigram) + popola `allowedAteco`, poi **1 chiamata LLM constrained** (scope `ma_strategy_ateco`) sceglie+classifica il fit **solo tra i candidati**. L'invariante "nessun codice fuori dai candidati" e il pruning dei sottoalberi esclusi (prefix, già in Go) restano deterministici.
    - _Nessuno **skip** dell'LLM su retrieval "netto": la scelta da settore a testo non è deterministica (fit/esclusioni semantici); l'unico skip è il path a codici espliciti. Valutato e scartato._
    - _Contingenza (non pianificata, evidence-gated): se il traffico reale mostra che il codice giusto resta fuori dai candidati, si sostituisce il **solo** retrieval con embedding (storage in colonna Postgres + cosine in Go, niente pgvector). Il seam `SearchAteco` rende lo swap contenuto._
  - legalForms: mapping **testo→codice vendor** via `binocolo.company_legal_forms` (sinonimi: SRL/SPA/cooperative/ditta individuale…); forma richiesta **non mappabile → non ignorata**, va tra i vincoli da confermare/non supportabili (`missingCriteria`);
  - range (`turnoverAroundRange` esiste già) / default (`ATTIVA`/`searchLimit=100`) / thesis;
  - **vincoli oltre la ricerca** (finanziari/ownership/esclusioni): catturati nell'intent con la loro **disposizione**; l'**applicazione** non avviene qui (i due economici nel Passo 4; gli altri restano follow-on/`missingCriteria`). Vedi sezione «Vincoli oltre la ricerca»;
  - poi gli **stessi** `validateMAStrategy` + `canonicalizeMAStrategyProvinces(requireAllowed=true)` + `canonicalizeMAStrategyAteco(requireAllowed=true)`.
- A questo punto il path V2 è completo e **senza loop agentico**.

**DoD (agent):**
- `resolveIntent` compila e ritorna `MAStrategySpec`; territorio risolto come differenza di insiemi (regioni espanse ∪ includeProvinces − excludeProvinces).
- Path ATECO espliciti: validato su DB senza LLM. Path settori a testo: `SearchAteco` popola `allowedAteco`, **una** chiamata LLM constrained scope `ma_strategy_ateco`, **nessun codice fuori dai candidati** (invariante imposta dal codice, non dal modello).
- `legalForms`: mappa via lookup; forma non mappabile finisce in `missingCriteria`, non scartata in silenzio.
- Chiama gli **stessi** `validateMAStrategy`/`canonicalize*(requireAllowed=true)`; **nessun** tool-round nel path.
- `go build ./... && go vet ./...` verdi.
- _Fuori DoD (umano):_ giudicare la qualità semantica di fit/esclusioni ATECO su settori reali.

### Passo 3 — V2 diventa il path servito (sostituzione)

**Deliverable (agent):**
- Il caller di creazione sessione legge `ma_strategy_pipeline` da `ma_parameter` e instrada a `draftStrategyV2` (default `v2`) o al monolite (caller da confermare sul codice).
- Monolite: **rimosso dal serving**, dietro **kill-switch in `ma_parameter`** (`ma_strategy_pipeline`). Eseguito solo se `=monolith`. _(deciso)_
- Rimuovi dal path servito: loop, `maStrategyToolInstructions`, registrazione dei tool. **Mantieni** le impl (`SearchAteco`, `listProvincesWithCache`, `probeMASearchSurface`: riusate da V2/estimate). Rimuovi il case probe morto del dispatch.
- `estimate`/`execute` **invariati** (consumano `MAStrategySpec`).

**DoD (agent):**
- Con `ma_strategy_pipeline=v2` (default) il caller chiama V2; con `=monolith` chiama il monolite — instradamento verificabile leggendo il codice/branch, senza eseguire.
- Loop/`maStrategyToolInstructions`/registrazione tool/case probe morto **rimossi dal path servito**; le tre impl restano referenziate (no codice morto, no riferimenti rotti).
- `estimate`/`execute` non toccati.
- `go build ./... && go vet ./...` verdi.
- _Fuori DoD (umano):_ il **flip** del kill-switch in ambiente, l'osservazione che la latenza migliora davvero, la cancellazione del monolite dopo la finestra di confidenza.

### Passo 4 — Post-filtri economici (chiusura gap)

Dipende dai Passi 1–2 (intent + `resolveIntent` esistono); **indipendente dal cut-over** (Passo 3): tocca execute/scoring, non il draft. Oggi questi vincoli non sono applicati affatto.

**Deliverable (agent):**
- **Spec:** aggiungi a `MAStrategySpec` **solo** `RevenuePerEmployeeMin *int` e `MaxShareholders *int` (consumati qui sotto).
- **`resolveIntent`** popola i due campi dai vincoli dell'intent (con `sourceText`, come gli altri filtri forti).
- **Applicazione** in `scoreMATargetsV2` accanto ai knockout esistenti (viability / sector gate, ~`ma_scoring.go:159-162`), riusando `maMatchStateOutside` (fuori criterio):
  - ricavo/dipendente: `Turnover/Employees < min` → fuori criterio;
  - max soci: `len(shareHolders) > max` → fuori criterio.
- **Dato mancante** (turnover/dipendenti o soci assenti → non valutabile): allinea al comportamento viability esistente (flag "non valutabile", non passare in silenzio).
- **Frontend/types:** i due campi nuovi richiedono `apps/binocolo/src/api/types.ts`, la **chiave di invalidazione stima** `strategyKey()` (`TargetPage.tsx`) — **critico**: senza, una stima vecchia prodotta con strategia diversa risulterebbe "fresca" → bug — eventuali chip/summary read-only, e normalizzazione/validazione in `validateMAStrategy`. (Simboli FE esatti da confermare sul codice.)

**DoD (agent):**
- I due campi esistono su `MAStrategySpec` e sono popolati da `resolveIntent` con `sourceText`.
- I due knockout vivono in `scoreMATargetsV2` via `maMatchStateOutside`; il caso **dato mancante** è esplicito (flag "non valutabile", mai pass silenzioso) — verificabile con casi in-process.
- `strategyKey()` include i due campi nuovi (una spec con soglie diverse produce chiave diversa → la stima vecchia si invalida).
- `pnpm --filter mrsmith-binocolo exec tsc --noEmit` verde; `go build ./... && go vet ./...` verdi.
- _Fuori DoD (umano):_ confermare il comportamento sui target reali via smoke in lettura.

---

## A carico umano (dopo scrittura + build/test verdi)

Inizia **quando i deliverable dell'agent sono scritti e compilano**. Niente qui è eseguibile o verificabile dall'agent: tocca il DB condiviso, LLM live, l'ambiente o il giudizio. Sequenza:

1. **Applicare la migration del Passo 0** al DB (processo del team). Il DB è owned dal team: [l'agent consegna solo il `.sql`, non lo applica](../../../CLAUDE.md). Sblocca tutto il resto (registry + kill-switch).
2. **Verificare le righe registry live** per `ma_strategy_intent` e `ma_strategy_ateco` (modello/prompt effettivi): il live può divergere dal seed → leggere la riga reale, non fidarsi della migration.
3. **Smoke a mano in lettura** sul Mistra remoto (mai in scrittura): far girare V2 su poche richieste rappresentative (i casi ovvi + le trappole) e giudicare a occhio intent, ATECO, territorio. È il «livello 2» della validazione: richiede registry seedato + LLM live, quindi non è DoD d'agent.
4. **Cut-over / flip del kill-switch:** tenere `ma_strategy_pipeline=v2` (default), e in caso di problemi flippare a `monolith` a runtime (rollback senza redeploy). Decisione operativa, in ambiente.
5. **Osservare il traffico:** latenza/costo per scope (dagli audit per-call), tasso di `missingCriteria`, broadness alla stima. Conferma il guadagno e fa emergere la fedeltà fine che i guardrail non coprono.
6. **Cancellare il monolite** (loop/tool/istruzioni) dopo una finestra di confidenza.
7. **Decisioni operative abilitate dai dati raccolti**, fuori da questo piano: downgrade modello per scope (riga `mrsmith.llm_model`, zero codice) e — solo ora che il traffico esiste — l'**harness** del brief (gold/eval/replay), oggi prematuro.

## Vincoli oltre la ricerca: cattura + disposizione

L'intent cattura **ogni** vincolo (strutturato) → preservato nel trace. La richiesta è scritta una volta e serve **tutta** la pipeline: "non è un filtro di ricerca" ≠ "perso". Ogni vincolo riceve una **disposizione** secondo dove vive il dato (verificato sul codice execute/scoring):

| Vincolo | Dato | Disposizione | Stato |
|---|---|---|---|
| territorio, ATECO, turnover, dipendenti, legal form | filtri vendor | applicato alla ricerca | ✅ esiste |
| ricavo/dipendente | execute (turnover+dipendenti) | post-filtro scoring | **costruito in V2** (oggi solo signal soft `productivity`, senza soglia) |
| max soci | execute (`shareHolders`, len) | post-filtro scoring | **costruito in V2** (oggi mai contato né filtrato) |
| EBITDA % | solo IT-full deep (`ma_deep_engine`) | criterio allo stadio deep | follow-on |
| OCF ratio | non estratto da nessuna parte | — | non supportabile oggi → `missingCriteria` |
| esclusione gruppi > X | non estratto (gruppo/parent assente) | — | non supportabile oggi → `missingCriteria` |
| esclusione estere | non estratto (paese assente) | — | non supportabile oggi → `missingCriteria` |

Regole:

- V2 **cattura e dà disposizione** a tutto: niente è perso, ogni vincolo è visibile con il suo stato (applicato / post-filtro / deep / non supportabile).
- **Stato attuale onesto:** oggi **nessuno dei sei è applicato come filtro** (ricavo/dipendente è solo un signal soft senza soglia). Quindi i due che V2 costruisce sono un **miglioramento reale**, non parità.
- V2 **costruisce i due post-filtri economici** (ricavo/dipendente, max soci) — chiusura del gap, vedi Passo 4. EBITDA resta follow-on (dato solo nel deep); OCF/gruppi/estere restano `missingCriteria` finché non c'è estrazione dati nuova.

## Cosa è e non è garantito senza dati

- **Garantito (by construction):** nessun vincolo forte **senza evidenza testuale**, nessuna spec invalida servita, nessuna broadness mascherata (la superficie troppo ampia emerge alla stima, non viene nascosta).
- **Non garantito:** fedeltà fine (un vincolo reale droppato) e correttezza semantica ATECO/thesis su input mai visti. Mitigazione: qualche prova a mano sui casi ovvi; il resto emergerà dal **traffico reale dopo il lancio** — ed è allora, con dati, che si potrà costruire l'harness del brief (oggi prematuro).

## Contratti dati

- **Nuovo:** `MAIntent` (intermedio), persistito **solo nel trace** _(deciso)_. Cattura **tutto** il richiesto, strutturato — territorio, `atecoExplicit`, settori a testo, range, e i vincoli oltre la ricerca con la loro disposizione — così la richiesta **non si perde** anche quando un vincolo non è applicabile oggi.
- **`MAStrategySpec` quasi invariato:** V2 aggiunge **solo due** campi, perché ora hanno un consumatore (Passo 4): `RevenuePerEmployeeMin *int`, `MaxShareholders *int`. Persistita come JSON → nessuna migration per questi campi. Il territorio strutturato si risolve nel `Provinces` esistente; gli ATECO negli `AtecoCandidates` esistenti. Gli **altri quattro** vincoli **niente campo** (nessun consumatore → sarebbe codice morto).
- **Audit per call:** ogni call LLM (intent, ateco) emette un `llm.CallAudit` **separato** (scope/model/prompt/request/response/usage) — non uno aggregato — per misurare costo/latenza per scope e alimentare la futura scelta del modello. Oggi `draftStrategy` ritorna un solo audit.
- **Trace:** nuovi `event_type` (`ma_intent_extract`, `ma_ateco_rerank`) nelle tabelle esistenti — **nessun cambio schema**.

## Repo-fit

- Nessun nuovo mini-app, nessuna nuova env.
- Migration solo per seed registry (`ma_strategy_intent`, `ma_strategy_ateco`). Consegnata come `.sql`, applicata dall'utente.
- Verifica build: `cd backend && go build ./... && go vet ./...` (fallback Docker via `docs/TOOLING-WITH-DOCKER.md` se la toolchain locale manca).

## Test (regola del repo: solo se approvati)

Se approvati, sono **deliverable dell'agent** ed entrano nel DoD del passo: tutti in-process, data-free (input costruiti a mano, nessun DB/LLM live). Candidati da proporre quando il passo è pronto:

- `validateIntentSpans`: vincolo forte senza span → scartato (regola-cardine).
- `resolveIntent`: assemblaggio `MAStrategySpec` da intent fissato (trasformazione dati non triviale).
- Post-filtri economici (Passo 4): knockout soglia ricavo/dipendente e conteggio soci, incluso il caso **dato mancante** (regola di business con edge case).

## Questioni aperte

- Punto esatto di sostituzione nel caller di creazione sessione — **lo confermo in implementazione sul codice**, non è una decisione di prodotto.

_(Nessuna decisione di prodotto aperta: la persistenza dell'intent è decisa = trace-only, vedi Contratti dati.)_

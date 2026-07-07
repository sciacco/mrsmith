# Binocolo — Piano "Sgancio funzioni per-azienda" (deep-dive e thesis-reading fuori dal casello card)

> Esegue le decisioni del brainstorming 2026-07-07 (secondo tema, dopo
> `INIZIATIVA-RADICE-PLAN.md`). Ogni task è pensato per essere eseguito **da
> solo, in ordine**, da un LLM esecutore. Riferimenti a simboli/file
> **verificati sul codice al 2026-07-07** (branch `wip/binocolo`, HEAD
> `8d0e33d`): usarli, non inventarne. Se un simbolo citato non esiste più,
> fermarsi e segnalarlo.

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **La funzione segue lo scope naturale del suo dato.** Deep analysis è
   globale per `company_key` (thesis-neutral); thesis-reading dipende da
   (tesi, azienda) e la tesi è della **sessione** (verificato:
   `resolveCardThesis`, `ma_thesis_reading.go:24-46`, risale sempre alla
   sessione di provenienza — la card non contribuisce alcuna informazione).
   Restano sulla card SOLO gli artefatti di lavorazione: stato kanban, note,
   eventi, esiti, IRL. Questo piano non li tocca.
2. **Lancio deep-dive a livello azienda, MAI condizionato al costo.** Il
   lancio di un singolo deep-dive è un'azione a un gesto: niente conferme di
   costo, niente stato `cost_required`, niente gate di budget. I costi non
   compaiono mai in UI utente (regola di progetto preesistente). Il gate
   budget/ack esistente su `deepDiveCard` (`errMAEstimateOverBudget`) e il
   `cost_required` di `companyDossier` si rimuovono per il lancio singolo.
3. **Tre punti di lancio** (mutazione), tutti in contesti dove l'analista
   guarda QUELLA azienda:
   - drawer del target in `RicercaDetailPage` (`TargetDetailModal`);
   - Target Inspector, tab T4 stato D («nessun deep-dive») — **emendamento
     ratificato** del read-only del PRD inspector, limitato a questa sola
     azione;
   - drawer della card sul kanban (`CardDrawer` in `IniziativaBoardPage`).
   Il card-dossier conserva il suo trigger migrando sullo stesso endpoint.
   **Tutte le altre superfici mostrano solo lo stato** (assente / in corso /
   disponibile / fallita), mai il lancio.
4. **Thesis-reading ri-scopata su (sessione, azienda)**: disponibile nel
   funnel prima della stella (l'ordine giusto è deep → lettura di tesi →
   decisione stella). Il card-dossier continua a mostrarla risalendo alla
   sessione di provenienza (stessa logica con cui già oggi la genera).
   Precondizione invariata: deep pronto (`ma_thesis_reading.go:114`).
   Staleness invariata (`ThesisSnapshot`, righe 83-85).
5. **Il drawer del target si ridisegna in questo piano (F5)** secondo la
   gerarchia informativa ratificata dal parere analista M&A (2026-07-07,
   riportata integralmente in F5): kill → angolo d'acquisto → conferme;
   l'anagrafica lascia il posto alla decisione. Solo dati già presenti nel
   sistema, zero nuove fonti.

## Fuori perimetro (in agenda brainstorming, NON eseguire)

- Convergenza delle superfici di rendering deep (inspector/deep/ vs
  card-dossier vs `/azienda`) — non ratificata.
- La "pagina analista" che sostituirà l'inspector.
- Navigazione Ricerche/Iniziative.

## Repo-fit (checklist `docs/IMPLEMENTATION-PLANNING.md`)

- **Runtime/dev/deploy**: nessuna nuova app, nessun env nuovo.
- **Auth**: endpoint nuovi dietro `app_binocolo_access` (`handle(...)` in
  `RegisterRoutes`, `handler.go:112-183`).
- **Data contract**: `company_key` da `maTargetDedupeKey`; deep =
  `ma_deep_analysis` globale; thesis-reading migra a chiave
  `(session_id, company_key)`.
- **Migrazioni**: idempotenti in `deploy/migrations/`, applicate a mano
  dall'utente. **Numerazione**: `INIZIATIVA-RADICE-PLAN.md` riserva 102-104;
  questo piano parte da **105** — ricontrollare la prossima libera al momento
  dell'esecuzione.
- **Osservabilità**: trace event su ogni mutazione (pattern
  `maTraceEventWrite`), errori via `maFailure`.
- **Coesistenza**: worker deep (`ma_deep_worker.go`), engine, scoring, gate
  UC2, funnel gated **non cambiano**. `EnqueueMADeepAnalysis` è già
  cache-first/idempotente lato coda.

## Regole globali per l'esecutore (valgono per OGNI task)

1. **Database**: MAI connettersi ai DB degli env, né in lettura. Schema solo
   come migrazioni idempotenti.
2. **Test**: NON aggiungere test nuovi. Verifica backend per ogni task:
   `cd backend && go build ./... && go vet ./internal/binocolo/ && gofmt -l
   internal/binocolo/` (stampa nulla) e `go test ./internal/binocolo/` verdi
   (fake store aggiornati al minimo se l'interfaccia si estende).
3. **Frontend**: type-check SOLO `pnpm --filter mrsmith-binocolo exec tsc
   --noEmit`. Smoke UI a fine task F: dev server già attivo (mai
   killare/riavviare), skill `playwright-cli` con cwd=`artifacts/claude`,
   bypass verificati via grep. **ATTENZIONE SPESA**: il DB dev è il Mistra/
   Anisetta condiviso con dati reali e il lancio deep-dive SPENDE (~€0.30
   IT-full). Negli smoke NON cliccare mai «Avvia analisi» su aziende senza
   deep esistente: verificare il bottone solo come presenza/stato, e gli
   stati in-corso/pronto su aziende che hanno GIÀ un deep (es. rotte citate
   in `TARGET-INSPECTOR-IMPLEMENTATION-PLAN.md`: sessione `e95445c4-…` deep
   ready). Il flusso di lancio reale lo verifica l'utente.
4. **Costi mai in UI utente** — vale anche per messaggi d'errore.
5. **API**: path Go senza prefisso `/api`; URL pubblico `/api/binocolo/v1/...`.
6. **Copy**: italiano B2B asciutto. L'azione si chiama «Avvia analisi»
   (coerente con «Analisi approfondita» già in uso); mai "deep-dive" in UI.
7. **Hot reload** `air`: ogni task con migrazione degrada in modo innocuo
   finché non applicata, o dichiara "applicare la migrazione NNN prima del
   deploy".
8. **Non toccare**: scoring v3, routing, gate UC2, `ma_deep_engine.go`,
   `ma_deep_worker.go` (il lancio usa la coda esistente), TargetPage legacy.
9. A fine task: file toccati + comandi di verifica con esito. Mai dichiarare
   fatto ciò che non è verificato.

## Ordine di esecuzione e dipendenze

```
B1 (endpoint deep-dive per azienda) ──→ F1 (drawer target: sezione analisi)
                                    ──→ F2 (inspector T4)
                                    ──→ F3 (drawer card kanban + migrazione card-dossier)
B2 (rimozione gate costo /azienda)     [indipendente, piccolo]
B3 (mig 105 + thesis-reading per sessione) ──→ F4 (thesis-reading nel funnel)
F5 (redesign contenuto drawer) [dopo F1 e F4: ne ingloba le sezioni]
B1 prima di B3 (B3 riusa lo stato deep esposto da B1 nel drawer).
```

Nota di coordinamento: se `INIZIATIVA-RADICE-PLAN.md` viene eseguito prima, i
riferimenti riga di `handler.go`/`ma_service.go` saranno slittati — cercare i
simboli per nome.

---

## B1 — Endpoint deep-dive per azienda

**Contesto verificato**: `deepDiveCard` (`ma_service.go`, cercare
`func (s *maService) deepDiveCard`) fa: require card operational →
`ListMADeepAnalysis([companyKey])` → no-op se ready/queued/running → gate
budget (`projected > pricing.BudgetDefault && !ack` → `errMAEstimateOverBudget`)
→ `EnqueueMADeepAnalysis(companyKey, vatCode, taxCode, email)` → trace. Il
worker (`ma_deep_worker.go`) fa il resto. `companyDossier` (stesso file) mostra
il pattern cache-first per VAT.

**Backend:**
- Nuovo `POST /binocolo/v1/ma/companies/{companyKey}/deep-dive` (nessun body).
  Service `deepDiveCompany(ctx, companyKey, subject, email)`:
  1. risolvere VAT/tax code per `companyKey`: prima da una riga `ma_target`
     con quel dedupe key (query store nuova o esistente — verificare se ne
     esiste una per company_key), fallback snapshot card
     (`GetMAInitiativeCard` non va bene: serve initiative — usare una query
     su `ma_initiative_card` per solo company_key), fallback `companyKey`
     come VAT normalizzata (pattern di `companyDossier`). Se nessuna fonte dà
     una VAT plausibile → errore di validazione esplicito;
  2. cache-first identico a `deepDiveCard`: ready/queued/running → no-op con
     stato corrente;
  3. **NESSUN gate budget/ack** (decisione 2): enqueue diretto;
  4. trace event `ma_company_deep_dive_started` con provenienza
     (company_key, fonte VAT).
  Risposta: `{status}` con lo stesso vocabolario di stato già usato dal
  frontend (`queued/running/ready/failed`).
- `deepDiveCard` (card-dossier lo usa oggi): ridurlo a delega verso
  `deepDiveCompany` DOPO il require card (l'ownership check resta: l'endpoint
  card continua a esistere per compatibilità in F3, la logica di spesa è una
  sola). Rimuovere il gate budget anche qui (decisione 2); il campo
  `acknowledgeCost` del body diventa ignorato — non rimuoverlo dal contratto
  finché F3 non aggiorna il client.

**Verifica**: build/vet/gofmt/test.

---

## B2 — `/azienda`: rimozione del gate di costo

**Contesto verificato**: `companyDossier` (`ma_service.go`) ritorna
`status: "cost_required"` + `CostEUR` quando manca l'ack — cioè un costo in
UI utente, in violazione della regola.

- Rimuovere il ramo `cost_required`: il POST con VAT nuova accoda direttamente
  (`EnqueueMADeepAnalysis`). Il parametro `ack` del service e l'eventuale
  campo richiesta lato handler/frontend (`CompanyDossierPage.tsx`) si
  rimuovono; eliminare dal frontend qualunque copy/modale di conferma costo.
  `CostEUR` sparisce dalla risposta.
- Comportamento cache-first invariato (ready/in-flight → servito senza
  enqueue).

**Verifica**: build/vet/gofmt/test; `tsc --noEmit`; smoke `/azienda` con una
P.IVA **già in cache** (nessuna spesa).

---

## F1 — Drawer del target: stato + lancio

**File**: `apps/binocolo/src/pages/ricerche/RicercaDetailPage.tsx`
(`TargetDetailModal`, definizione riga ~1119, uso riga ~528).

- Sezione «Analisi approfondita» nel drawer: stato corrente da
  `target.deep` (il detail singolo lo espone già — verificato in
  `TARGET-INSPECTOR-IMPLEMENTATION-PLAN.md`: `GET
  /sessions/{id}/targets/{targetId}` torna `deep` cached):
  - assente → bottone **«Avvia analisi»** → `POST
    /binocolo/v1/ma/companies/{companyKey}/deep-dive` (B1);
  - queued/running → indicatore in corso + polling (riusare il pattern a
    5s dell'inspector, `TargetInspectorPage.tsx:97-105`, o ricaricare il
    detail singolo del target già lazy-fetchato a riga ~160);
  - ready → sintesi essenziale + link «Ispezione completa ↗» (entry point
    esistente);
  - failed → stato con retry (stesso bottone).
- Nessun testo di costo, nessuna conferma (decisione 2).
- Il drawer verrà ridisegnato (decisione 5): implementazione minima e pulita,
  non investire in styling oltre `docs/UI-UX.md`.

**Verifica**: `tsc --noEmit`; smoke SOLO su target con deep già esistente
(stati ready/in-corso) + presenza bottone su target senza deep **senza
cliccarlo** (regola 3).

---

## F2 — Inspector T4: lancio nello stato D

**File**: `apps/binocolo/src/pages/ricerche/inspector/tabs/DeepTab.tsx`.

- Lo stato D («nessun deep-dive», oggi vicolo cieco) riceve il bottone
  **«Avvia analisi»** → stesso endpoint B1; al lancio lo stato passa a B
  (queued/running) e il polling esistente della pagina
  (`TargetInspectorPage.tsx:97-105`) fa il resto.
- Stato failed (C): aggiungere retry con lo stesso bottone.
- **Emendamento documentale**: aggiungere una riga a
  `TARGET-INSPECTOR-IMPLEMENTATION-PLAN.md` (tabella stato) e/o
  `TARGET-INSPECTOR-PLAN.md`: "2026-07-07 — emendato il read-only (PRD §2/§5):
  unica mutazione ammessa il lancio dell'analisi approfondita da T4, decisione
  brainstorming sgancio funzioni". Non riformulare il resto del PRD.

**Verifica**: `tsc --noEmit`; smoke sulle rotte note del piano inspector
(deep ready e deep nil), senza cliccare il lancio su aziende senza deep.

---

## F3 — Drawer card kanban + migrazione card-dossier sull'endpoint azienda

**File**: `apps/binocolo/src/pages/iniziative/IniziativaBoardPage.tsx`
(`CardDrawer`, definizione riga ~735, uso riga ~617) e
`IniziativaCardDossierPage.tsx`.

- `CardDrawer`: sezione stato analisi (il board espone già `dossierStatus`
  con polling a 5s quando `working`, righe ~175-181) + bottone «Avvia
  analisi» quando assente/failed → endpoint B1 con la `companyKey` della
  card.
- `IniziativaCardDossierPage`: sostituire la chiamata al vecchio
  `POST .../cards/{companyKey}/deep-dive` con l'endpoint B1; rimuovere
  l'`acknowledgeCost:true` dal client e qualunque copy di conferma costo.
  (L'endpoint card resta vivo lato backend come delega — la sua rimozione è
  pulizia futura, non di questo piano.)

**Verifica**: `tsc --noEmit`; smoke su board esistente: stato visibile sulle
card con deep, bottone presente senza cliccarlo altrove.

---

## B3 — Thesis-reading per (sessione, azienda) — mig 105

**Contesto verificato**: `ma_card_thesis_reading` PK `(initiative_id,
company_key)`, FK CASCADE su `ma_initiative` (mig 096:15-29). Generazione:
`generateCardThesisReading` → `resolveCardThesis` (sessione del rating più
recente da `MACardProvenance`, fallback `CreatedFromSession`) →
`composeMAThesisText(version.Strategy)` (`ma_thesis_reading.go:24-63`).
Richiede deep pronto (riga ~114). Scope LLM `maModelScopeThesisReading`.

**Migrazione 105** `105_binocolo_ma_session_thesis_reading.sql` (idempotente):
- Nuova tabella `binocolo.ma_session_thesis_reading`: stesse colonne della
  096 ma PK `(session_id, company_key)`, FK `session_id REFERENCES
  binocolo.ma_session(id) ON DELETE CASCADE` (verificare in mig 096 l'elenco
  colonne esatto e replicarlo, incluso `thesis_snapshot` e audit).
- **Backfill** dalla 096: per ogni riga, `session_id` = colonna sessione se
  la 096 la persiste (verificare lo schema: `resolveCardThesis` ritorna
  `sessionID` — controllare se viene salvato), altrimenti
  `ma_initiative_card.created_from_session` della card corrispondente;
  righe senza sessione risolvibile → saltate con conteggio nel commento.
  `ON CONFLICT DO NOTHING`.
- La 096 NON si droppa (rollback possibile; pulizia futura).

**Backend:**
- Store: `Get/UpsertMASessionThesisReading` con chiave (session, company);
  la logica di generazione (`generateCardThesisReading`) si rinomina/riusa in
  `generateSessionThesisReading(ctx, sessionID, companyKey, ...)`: la tesi si
  compone direttamente dalla strategia attiva della sessione data
  (`composeMAThesisText`), senza passare dalla card; precondizione deep
  pronto e staleness invariate.
- Nuovi endpoint: `GET/POST
  /binocolo/v1/ma/sessions/{id}/targets/{targetId}/thesis-reading` (o per
  companyKey — seguire la forma degli endpoint target esistenti in
  `handler.go`; il target porta la sua companyKey).
- Gli endpoint card esistenti (`handler.go:136-137`) diventano **adapter**:
  risolvono la provenienza con la stessa logica di `resolveCardThesis` e
  delegano alle funzioni session-scoped (lettura e generazione) — il
  card-dossier non cambia comportamento percepito. Il doppio storage NON deve
  esistere: gli adapter leggono/scrivono SOLO la tabella nuova.
- Trace event con session_id e company_key.

**Verifica**: build/vet/gofmt/test; migrazione a doppia applicazione ok
(review testuale; applicazione utente con conteggi prima/dopo).

---

## F4 — Thesis-reading nel funnel

**File**: `RicercaDetailPage.tsx` (drawer, F1) e/o
`inspector/tabs/TesiTab.tsx`.

- Nel drawer del target, sotto la sezione analisi (F1): quando il deep è
  ready, azione **«Lettura di tesi»** → `POST` session-scoped (B3);
  visualizzazione della lettura persistita con indicatore staleness
  (`StaleThesis`) e rigenerazione esplicita — riusare la presentazione già
  esistente nel card-dossier (`IniziativaCardDossierPage.tsx`, sezione
  thesis-reading): estrarre il componente se necessario, non duplicarlo.
- Inspector T5 (Tesi) mostra la lettura session-scoped se esiste (oggi T5
  mostra fit di tesi dello scoring — verificare cosa rende e integrare senza
  stravolgere: se il posto naturale è T4/T5 deciderlo dal contenuto reale del
  tab, segnalando la scelta).
- La generazione è una chiamata LLM (centesimi, nessun vendor a €): nessuna
  conferma; smoke di generazione live ammesso UNA volta su un'azienda con
  deep pronto, dichiarandolo.

**Verifica**: `tsc --noEmit`; smoke lettura esistente via card-dossier
(invariata) + generazione singola dichiarata come sopra.

---

## F5 — Redesign contenuto del drawer target (gerarchia analista)

> Attua il parere analista M&A ratificato 2026-07-07. Frontend-only, nessun
> endpoint nuovo: **tutti i dati sono già nel `MATarget`** del detail singolo
> (`GET /sessions/{id}/targets/{targetId}`) o nella riga. Zero nuove fonti,
> zero chiamate aggiuntive. Per l'esecuzione UI usare la skill di progetto
> `tintoretto` (lavoro scoped su componente esistente).

**File**: `apps/binocolo/src/pages/ricerche/RicercaDetailPage.tsx`
(`TargetDetailModal`, definizione riga ~1119). Tipi:
`apps/binocolo/src/api/types.ts` — `MATarget` (~:838), `MAWebValidation`
(~:158), `candidateMatchAnalysis` (~:208), `MATargetEvidence` (~:916),
`MATargetFlag` (~:910). Semantica flag: `backend/internal/binocolo/ma_flags.go`
e `ma_scoring.go` (i `value` delle evidence sono già formattati:
`"+8%/anno"`, `"320k/dip"`, `"socio 61 anni"`, `"45% PN"`, `"25 anni"`).
Riferimento di resa esistente: `inspector/tabs/SintesiTab.tsx` (mostra già
fatturato/dipendenti/stato) — riusare componenti/pattern dove sensato, non
duplicare.

**Diagnosi ratificata** (non ri-discutere): il drawer attuale mostra
anagrafica catastale e nasconde la decisione. Fatturato/dipendenti/anno non
compaiono; il verdetto testuale del gate (`candidateMatchAnalysis.businessFit`,
`evidenceFor/Against`) è invisibile; i flag (kill e angolo d'acquisto) non
sono renderizzati; l'età del socio di controllo è annegata nelle evidenze.

**Gerarchia di contenuto** (ordine vincolante; layout secondo `docs/UI-UX.md`):

1. **Chi è e cosa fa** — `companyName`, forma giuridica, **`atecoDescription`**
   (codice in piccolo), `province`/`town`, `selectedDomain` come link.
   Sotto: pill del verdetto gate (`webValidationState`/`finalAction`,
   etichette utente già in uso nel funnel — mai gergo pipeline) + la frase
   `businessFit` di `candidateMatchAnalysis`; se presenti, `evidenceFor[]`/
   `evidenceAgainst[]` in forma compatta (collassabile).
2. **I tre numeri** — Fatturato (`turnover` + `turnoverYear`), Dipendenti
   (`employees`), Ricavo/dipendente (dall'evidence post-filter o derivato
   `turnover/employees`), con il trend (`"+8%/anno"` dall'evidence trend)
   come indicatore direzionale.
3. **Angolo d'acquisto** — chip dai flag neutri: `ricambio_generazionale`
   (con età socio dall'evidence), `socio_unico`, `impresa_familiare`,
   `controllo_holding`; più anzianità azienda (evidence `"25 anni"`).
4. **Kill criteria** (solo se accesi, trattamento warning/danger):
   morta/dormiente (`cessata_fiscalmente`, `non_attiva`, adjustment
   `viability` < 1), distress patrimoniale (`patrimonio_netto_negativo`/
   `_eroso`), fuori bersaglio (`fuori_settore`, `ateco_fuori_perimetro`,
   gate reject). Caveat non-kill separati: `bilancio_assente`/
   `bilancio_datato`. **In testa al drawer**, se presenti: badge
   `registryFacts` (da evitare / già cliente / non vende) e
   `inLavorazione[]` — kill a costo zero, priorità massima.
5. **Punteggio** — `score` + `confidence` + bucket utente; breakdown
   `evidence[]` (label · value · points) e `adjustments[]` **collassati**
   dietro «Dettaglio punteggio», chiusi di default.

Le sezioni di F1 (analisi approfondita) e F4 (lettura di tesi) si collocano
tra la 4 e la 5 come blocco di approfondimento; F5 le **ingloba** senza
cambiarne endpoint/logica. Rating stelle ed esclusione restano dove
l'interazione è oggi (header o footer del drawer, a scelta di design), mai
sotto il fold.

**Fuori dal drawer** (non aggiungere, rimuovere se presenti): valutazione
EV/equity, PFN, bridge, scorecard completa, brief (vivono nel deep-dive /
inspector); telemetria (hash, sourcePath, model/prompt id, freshness,
diagnostica domain resolution). Il link «Ispezione completa ↗» resta l'unica
uscita verso il dettaglio tecnico.

**Verifica**: `tsc --noEmit`; smoke playwright-cli su una sessione completata
esistente (sola lettura): aprire il drawer su (a) un target confermato con
dati ricchi, (b) uno con kill criteria accesi, (c) uno con deep pronto —
verificare gerarchia, collassabili, assenza di costi/telemetria. Screenshot
in `artifacts/claude/`.

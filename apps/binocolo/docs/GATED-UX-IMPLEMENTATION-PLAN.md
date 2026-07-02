# Binocolo — Piano di implementazione UI/UX gated (workstream D)

> Esegue la specifica di `GATED-UX-PRD.md` (iterazione 8, tutta [DECISO]). Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore. I riferimenti a simboli/file/endpoint sono **verificati sul codice al 2026-07-02**: usare quelli, non inventarne. Se un simbolo citato non esiste più, fermarsi e segnalarlo — non improvvisare un sostituto.

## Regole globali per l'esecutore (valgono per OGNI task)

1. **Leggere prima**: `apps/binocolo/docs/GATED-UX-PRD.md` (la specifica: ogni task cita le sue sezioni), `docs/UI-UX.md` (design system, obbligatorio per i task F), `docs/API-CONVENTIONS.md`. Per i task F2–F5 anche il **wireframe approvato**: `apps/binocolo/docs/gated-ux-wireframe.html` — HTML autocontenuto con gli stati S1–S10 (S1–S6 = pagina D1, S7–S10 = pagina D2) e note numerate ancorate alle sezioni del PRD. È la fonte di verità per layout e copy: leggerne il markup ed estrarre i testi verbatim, non riscriverli. Non ridiscutere le decisioni del PRD: sono ratificate.
2. **Database**: MAI connettersi ai DB degli env (`ANISETTA_DSN`, ecc.), né in lettura. Le modifiche schema si consegnano SOLO come file `deploy/migrations/NNN_*.sql` idempotenti (numerazione: la prossima libera è **085**). L'utente le applica a mano.
3. **Test**: NON aggiungere test nuovi (regola di progetto). Verifica per ogni task backend: `cd backend && go build ./... && go vet ./internal/binocolo/ && gofmt -l internal/binocolo/` (deve stampare nulla) e `go test ./internal/binocolo/` (i test esistenti restano verdi; se falliscono per sandbox su porte httptest, rilanciare fuori sandbox).
4. **Frontend**: type-check SOLO con `pnpm --filter mrsmith-binocolo exec tsc --noEmit` (mai `npx tsc`). Smoke test UI obbligatorio a fine task F: riusare il dev server già attivo se presente (mai killarlo/riavviarlo), browser via skill `playwright-cli` e solo dopo aver verificato via grep che `VITE_DEV_AUTH_BYPASS` e `SKIP_KEYCLOAK` sono attivi nella configurazione dev.
5. **Costi = mai in UI utente**: nessun €, nessun costo unitario, nessuna banda o survivor-rate. `MAEstimate.EstimatedCost` esiste nel payload: NON renderizzarlo nelle pagine nuove.
6. **API**: il modulo Go registra i path SENZA prefisso `/api` (es. `handle("GET /binocolo/v1/...")` in `backend/internal/binocolo/handler.go`); l'URL pubblico è `/api/binocolo/v1/...`. Il client frontend esistente (`apps/binocolo/src/api/`) gestisce già il prefisso: imitare le chiamate esistenti.
7. **Copy**: italiano, B2B asciutto (sostantivi secchi, niente prima persona, niente domande). Etichette bucket per l'utente: **In tesi / Da approfondire / Da verificare / Fuori tesi** (mappano keep / forse / manual_review / scarta). Il gergo di pipeline (keep, forse, scarta, gate, advanced) NON compare mai in interfaccia.
8. **Non toccare**: scoring v3, routing v3 (`ma_routing.go` salvo dove esplicitamente indicato), la logica del gate UC2, il flusso della TargetPage oltre a quanto specificato in F1. Niente refactor opportunistici, niente rinomini di campi esistenti, niente migliorie non richieste.
9. **Migrazioni e hot-reload**: il backend gira sotto `air` (hot reload). Ogni task con migrazione deve degradare in modo innocuo finché la migrazione non è applicata, oppure dichiarare esplicitamente nel messaggio finale "applicare la migrazione NNN prima del deploy". Le migrazioni di questo piano sono additive.
10. A fine task: elencare i file toccati e i comandi di verifica eseguiti col loro esito. Se qualcosa resta incompleto, dirlo esplicitamente — mai dichiarare fatto ciò che non è verificato.

## Ordine di esecuzione e dipendenze

```
B1 → B2 ─┐
B3       ├→ F2 (D1)
B8 ──────┘
B4 ──────→ F3 (D2 progress)
B5, B6, B7 → F5 (D2 coda)
F1 (rotte) prima di F2/F3/F4/F5
F4 dopo F3
```

---

## B1 — Persistere concetti KB e modalità di retrieval sulla strategia (R1, R1b)

**Obiettivo**: il pannello "Ambiti riconosciuti" di D1 mostra i concetti reali del retrieval. Oggi i nomi non sono persistiti (il candidato porta solo `Rationale: "Concetto KB <id> (coseno …)"`).

**Contesto verificato**: `backend/internal/binocolo/ma_ateco_retrieval.go` — `retrieveMAIntentAtecoEmbedding` produce `matched []kbScored` (campo `concept *kbConcept` con ID/nome) e chiama `resolveKBFitToCandidates(ctx, matched, coreCut, allowed)`; il fit è `maFitCore`/`maFitWeak` per concetto (cosine >= coreCut). `MAStrategySpec` (`ma_types.go`) è persistita come JSONB: aggiungere campi al blob NON richiede migrazione (precedente: `MaxBudgetEUR`).

**Passi**:
1. In `ma_types.go` aggiungere:
   ```go
   type MAStrategyConcept struct {
       ID         string   `json:"id"`
       Name       string   `json:"name"`
       Fit        string   `json:"fit"`                  // core | weak
       Divisions  []string `json:"divisions,omitempty"`  // divisioni a 2 cifre coperte
       AtecoCodes []string `json:"atecoCodes,omitempty"` // codici del fit-mapping InKB
   }
   ```
   e su `MAStrategySpec` (zona campi persistiti, NON tra i transient `json:"-"`):
   ```go
   SectorConcepts     []MAStrategyConcept `json:"sectorConcepts,omitempty"`
   SectorRetrievalMode string             `json:"sectorRetrievalMode,omitempty"` // embedding | fallback_llm | explicit_reverse
   ```
2. In `retrieveMAIntentAtecoEmbedding`: dai `matched` costruire `[]MAStrategyConcept` (fit = core se `cosine >= coreCut`; divisions da `conceptDivisions` già esistente nel file; atecoCodes dai codici `InKB` del concetto). Restituirli al chiamante (aggiungere un valore di ritorno) e in `resolveMAIntentAteco` (`ma_service.go`) propagarli.
3. In `resolveIntent`/assemblaggio strategia (`ma_service.go`, dove viene costruita `MAStrategySpec` con `AtecoCandidates`): valorizzare `SectorConcepts` e `SectorRetrievalMode`: `"embedding"` dal retrieval; `"fallback_llm"` quando si passa dal resolver gerarchico (il punto esatto è il trace `ma_ateco_retrieval_fallback` in `resolveMAIntentAteco`); vuoto per il monolite legacy.
4. Verificare che `validateMAStrategy`/`ma_rules.go` non strippi i campi nuovi (se normalizza campo-per-campo, aggiungere pass-through con sanitizzazione: cap 24 concetti, cleanText sui testi).

**Non fare**: non toccare la logica di soglie/fit; non persistire i cosine; non aggiungere migrazioni.

**Done when**: creando una sessione (pipeline v2) la strategia nel session detail contiene `sectorConcepts` e `sectorRetrievalMode`. Verifica manuale possibile via `POST /api/binocolo/v1/ma/sessions` da UI esistente o curl locale.

## B2 — Reverse lookup KB per codici ATECO espliciti (R5) — dipende da B1

**Obiettivo**: PRD §2.2 caso limite. Se la richiesta contiene codici ATECO espliciti, convertirli nei concetti KB che li includono; codici non coperti restano visibili come "fuori KB".

**Contesto verificato**: `resolveMAIntentExplicitAteco` (`ma_service.go`, ~riga 2730) gestisce il ramo esplicito e NON passa dal retrieval. I concetti si caricano con `LoadKBConcepts(ctx)` (`kb_store.go`, `SQLStore`) — usare la stessa via d'accesso di `matchKBConcepts` (`s.kb`). Ogni `kbConcept` ha il fit-mapping `InKB` (codici inclusi) ed `Excluded`.

**Passi**:
1. Nuova funzione pura in `ma_ateco_retrieval.go`: `conceptsCoveringCodes(concepts []kbConcept, codes []string) []MAStrategyConcept` — un concetto copre un codice se il codice normalizzato (usare `atecoSearchCode`) è in `InKB` oppure è discendente di un codice `InKB` (confronto per prefisso sul search code). Fit del concetto risultante: `core`.
2. In `resolveMAIntentExplicitAteco`: dopo la risoluzione dei codici, caricare i concetti (soft: errore KB → lista vuota, non bloccare) e produrre `SectorConcepts` + i codici non coperti in una lista dedicata; su strategia: `SectorRetrievalMode = "explicit_reverse"`. I codici fuori KB vanno ANCHE in `MissingCriteria` con testo `"Codice ATECO dichiarato fuori dalla base di conoscenza: <code>"` (la UI li mostra da lì).

**Non fare**: non cambiare il comportamento di ricerca del ramo esplicito (candidates/superficie restano identici); il reverse è solo presentazione+persistenza.

**Done when**: sessione creata con prompt contenente un codice (es. "codice ateco 62.01") ha `sectorRetrievalMode: "explicit_reverse"` e `sectorConcepts` popolato quando il codice è coperto dalla KB.

## B3 — Flusso gated: stima solo-espansa, guardia sector-less, limite = cap (R2, R6, R7)

**Contesto verificato**: `MACreateSessionRequest` = `{prompt, modelId, promptId}` (`ma_types.go:247`); `createSession` setta `SearchLimit: maDefaultSearchLimit` (=100, `ma_types.go:128`). `MAEstimateSessionRequest` = `{strategy}` (`ma_types.go:253`). `buildMAEstimateQueries` (`ma_service.go:4653`) produce query per ENTRAMBI i tipi (`maStrategyTypeATECO` e `maStrategyTypeExpanded`); `maSearchQuery.strategyType` distingue. `expandStrategyExpansion` richiede `strategy.SectorDivisions` non vuoto. Endpoint: `POST /binocolo/v1/ma/sessions/{id}/estimate` (handler.go:133). Il gated ammette `min(limit, stimato)` e rifiuta oltre cap (`ma_gated_search_job.go:121-131`, `errMAEstimateTooLarge`). `maVendorLimit = 1000` (`ma_types.go:129`).

**Passi**:
1. `MACreateSessionRequest` += `GatedFlow bool \`json:"gatedFlow,omitempty"\``. In `createSession`: se `GatedFlow`, dopo l'assemblaggio strategia impostare `strategy.SearchLimit` = `pricing.SurfaceCap` se >0, altrimenti `maVendorLimit` (caricare pricing con `s.loadPricing(ctx)` come fanno estimate/gated). **Motivo (R6)**: il default 100 troncherebbe silenziosamente il gated a 100 aziende.
2. `MAEstimateSessionRequest` += `StrategyType string \`json:"strategyType,omitempty"\``. Propagare fino a dove girano le stime (il flusso estimate è asincrono via job: seguire `handleEstimateMASession` → service → job payload) e, quando vale `maStrategyTypeExpanded`, filtrare le query: dopo `buildMAEstimateQueries`, tenere solo quelle con `strategyType == maStrategyTypeExpanded`. Default (campo vuoto) = comportamento attuale (entrambe) — il vecchio flusso non cambia.
3. **Guardia R7**: nel percorso di stima, quando il filtro è `expanded` e dopo la canonicalizzazione `len(strategy.SectorDivisions) == 0`, fallire con `fmt.Errorf("%w: perimetro settoriale mancante", errMAStrategyInvalid)` PRIMA di lanciare probe. Mai lasciar passare il fallback legacy province-only (branch `else` in `buildMAEstimateQueries`, commentato "Legacy fallback for sector-less strategies") per il flusso gated.

**Non fare**: non rimuovere il fallback legacy (serve al vecchio flusso); non cambiare `maDefaultSearchLimit`.

**Done when**: build/vet/test verdi; una stima con `strategyType:"expanded"` produce SOLO righe estimate con `strategyType == "expanded"`.

## B4 — Endpoint di progresso gated (R3/R8) — PRD §3.1.1

**Obiettivo**: `GET /binocolo/v1/ma/sessions/{id}/gated-progress`, aggregazione **read-only**. Nessuna scrittura, nessuna lettura dei trace (write-only per dottrina).

**Contesto verificato**: `GetMASession` restituisce `MASessionDetail` con `Targets` (ognuno con `WebValidation *MAWebValidation` e `EnrichmentLevel` — costanti `maEnrichmentAddress`/`maEnrichmentAdvanced`, `ma_types.go:178-179`) e `Runs`. Bucket per target: `gatedTargetBucket(target)` (`ma_gated_search_job.go:584`) → `maGatedBucketKeep|Forse|Reject|ManualReview`. Stati sessione: `estimating`/`running` (usati oggi dalla UI in polling).

**Forma della risposta** (esatta, dal PRD):
```json
{ "stage": "address|gate|enrich|ready|failed",
  "surface": { "expected": 0, "fetched": 0 },
  "gate":    { "processed": 0, "total": 0,
               "buckets": { "keep": 0, "forse": 0, "scarta": 0, "manualReview": 0 } },
  "enrich":  { "enriched": 0, "survivors": 0 },
  "run":     { "startedAt": "...", "completedAt": null, "errorCode": "" } }
```

**Regole di derivazione** (implementarle così, sono verificate sul modello dati):
- run = l'ultimo `MAExecutionRun` della sessione (per `StartedAt`). `failed` se il run è fallito; `ready` se completato.
- `surface.expected` = `run.EstimatedCount`; `surface.fetched` = `len(targets del run)`.
- Sessione `running` e zero target del run → `stage = "address"`.
- `gate.total` = target del run; `gate.processed` = target con `WebValidation != nil`; bucket con `gatedTargetBucket` sui soli target processati. `stage = "gate"` finché `processed < total`.
- `enrich.survivors` = target con bucket keep|forse; `enrich.enriched` = target con `EnrichmentLevel == maEnrichmentAdvanced`. `stage = "enrich"` quando il gate è completo e la sessione è ancora `running`.
- Approssimazione accettata (documentarla nel commento): una validation stantia conta comunque come `processed`.

**Passi**: metodo service (es. `gatedProgress(ctx, sessionID)`), handler + `handle("GET /binocolo/v1/ma/sessions/{id}/gated-progress", ...)` in `handler.go` accanto agli altri route ma/sessions. Tipi risposta esportati in `ma_types.go` con tag json esattamente come sopra.

**Non fare**: nessun nuovo stato persistito; nessuna lettura di `ma_operation_trace`.

## B5 — Endpoint coda di verifica (R9) — PRD §3.3

**Obiettivo**: `GET /binocolo/v1/ma/sessions/{id}/verification-queue` — righe con motivo derivato dai fatti e rimedi ammessi.

**Contesto verificato**: il fatto sito-di-gruppo è `target.WebValidation.DomainResponse.GroupSiteHint` (`*MAGroupSiteHint {Domain, Identifier, Source}`, `web_search.go`) — già persistito. Candidati: `WebValidation.DomainResponse.Candidates`. Dominio scelto: `WebValidation.SelectedDomain`. Identity-only = `target.MatchState == ""`.

**Forma della risposta**:
```json
{ "items": [ { "targetId": "", "companyKey": "", "companyName": "", "province": "",
    "reason": { "kind": "group_site|identity_unconfirmed|no_candidates|enrich_failed", "detail": "" },
    "remedies": ["associate_domain","confirm_group_site","no_website","retry_search"] } ] }
```

**Regole di derivazione** (esatte):
- Righe = target con `gatedTargetBucket(t) == maGatedBucketManualReview`, PIÙ i target identity-only (`MatchState == ""`) con bucket keep|forse (= sopravvissuti mai arricchiti, "analisi non riuscita") quando la sessione NON è `running`.
- `kind`, in quest'ordine di precedenza:
  1. `GroupSiteHint != nil` → `group_site`, detail = `"Possibile sito di gruppo: <domain> (P.IVA di altra società <identifier>)"`.
  2. bucket keep|forse identity-only → `enrich_failed`, detail = `"Oltre il gate, analisi non riuscita"`.
  3. `len(Candidates) == 0` → `no_candidates`, detail = `"Nessun candidato web trovato"`.
  4. altrimenti → `identity_unconfirmed`, detail = `"Identità non confermata sulle pagine lette"`.
- `remedies`: `group_site` → `[confirm_group_site, associate_domain, no_website]`; `no_candidates`/`identity_unconfirmed` → `[associate_domain, no_website]`; `enrich_failed` → `[retry_search]` (= ri-lanciare `POST .../gated-search`: il job riprende idempotente e ri-tenta solo gli arricchimenti mancanti — comportamento verificato del resume).

**Non fare**: non inventare il motivo "sito irraggiungibile" — il fatto necessario (pagine lette) non è persistito oggi; resta fuori (annotato nel PRD come possibile v2).

## B6 — Rimedio "Conferma sito di gruppo" (policy B1) — migrazione 085

**Contesto verificato**: registro = `binocolo.ma_company_domain` (mig 082: PK `company_key`, `method CHECK IN ('auto_verified','manual')`, guardia applicativa "manual mai declassato"). Rimedio esistente: `POST /binocolo/v1/ma/sessions/{id}/associate-domain` → `enqueueAssociateDomain` (`ma_gated_search_job.go`) → job durevole `maJobTypeAssociateDomain` con payload `maAssociateDomainJobPayload{CompanyKey, Domain}` → `associateDomainWork` (re-gate della singola azienda + registrazione `maDomainMethodManual`). `lookupCompanyDomain` mappa `manual` → `maIdentityStateVouched`.

**Passi**:
1. **Migrazione `085_binocolo_ma_company_domain_group_site.sql`** (idempotente): `ALTER TABLE binocolo.ma_company_domain ADD COLUMN IF NOT EXISTS group_site boolean NOT NULL DEFAULT false;` + commento in testa nello stile delle 082/083.
2. `maCompanyDomain` struct + `GetMACompanyDomain`/`UpsertMACompanyDomain` (`ma_store.go`): aggiungere la colonna `group_site` (lettura e scrittura). La guardia manual-mai-declassato resta invariata (il metodo resta `manual`).
3. Payload job: `maAssociateDomainJobPayload` += `GroupSite bool \`json:"group_site,omitempty"\``. In `associateDomainWork`: quando `GroupSite`, registrare nel registro con `group_site=true` e usare come reason del candidato selezionato `"sito di gruppo confermato dall'operatore"` (l'identity resta `maIdentityStateVouched`, invariata).
4. In `lookupCompanyDomain`/`buildMAWebValidation`: hit di registro con `group_site` → reason `"dominio dal registro (sito di gruppo confermato)"`.
5. **Endpoint** `POST /binocolo/v1/ma/sessions/{id}/confirm-group-site`, body `{"companyKey": "..."}`: carica la validation del target, esige `GroupSiteHint != nil` (altrimenti 400 `errMAStrategyInvalid`), e accoda l'associate-domain con `Domain = hint.Domain` e `GroupSite = true`. Riusare la validazione sincrona di `enqueueAssociateDomain` (sessione idle, ecc.) — estenderla con il flag, non duplicarla.

**Nota deploy**: la colonna è additiva ma il codice di store la referenzia → **applicare la 085 prima del deploy/hot-reload** (dichiararlo nel messaggio finale del task).

## B7 — Rimedio "Nessun sito ufficiale" (workstream C) — migrazione 086

**Decisioni ratificate** (PRD §3.4): durevole cross-sessione; il gate dichiara `no_signal`, MAI `scarta`; l'azienda prosegue sui dati strutturati; **la conferma ammette anche l'Advanced**.

**Contesto verificato**: `sectorActionToBucket` (`ma_sector_eval.go:431`) manda ogni action sconosciuta in `forse` → un target no-website col nuovo `FinalAction` diventa sopravvissuto e paga l'Advanced SENZA toccare il mapper. `gatedTargetBucket` (`ma_gated_search_job.go:584`) marca manual_review solo su `FinalAction == "needs_domain_review"` o `WebValidationState == "domain_unresolved"` → basta usare valori diversi. Nessun CHECK sui valori di `final_action` nelle migrazioni (verificato).

**Passi**:
1. **Migrazione `086_binocolo_ma_company_domain_no_website.sql`** (idempotente): ricreare il CHECK di `method` per includere `'no_website'` (pattern DO-block con `pg_constraint` come nella 083):
   `method IN ('auto_verified','manual','no_website')`. Le righe no_website hanno `domain = ''` (la colonna è `NOT NULL`, la stringa vuota è ammessa).
2. Costanti in `ma_types.go`: `maDomainMethodNoWebsite = "no_website"`, `maWebValidationStateNoWebsite = "no_website_declared"`, `maFinalActionNoWebsite = "no_website_structured"`.
3. Registro: writer dedicato (non passare da `registerCompanyDomain`, che normalizza il dominio e fallirebbe su ""): upsert con method `no_website`, domain `''`. La guardia esistente protegge `manual` dagli `auto_verified`; estenderla in modo che `no_website` non declassi `manual` né viceversa senza intervento operatore (regola: un metodo operatore — manual o no_website — sovrascrive l'altro metodo operatore; auto_verified non sovrascrive nessuno dei due).
4. `buildMAWebValidation`: se il registro risponde `no_website` → NIENTE risoluzione/scrape/classificazione; produrre una validation con `WebValidationState = maWebValidationStateNoWebsite`, `FinalAction = maFinalActionNoWebsite`, `Confidence "alta"`, reason `"Nessun sito ufficiale (dichiarato dall'operatore)"`, `SelectedDomain` vuoto, `IdentityState = maIdentityStateVouched`.
5. **Endpoint** `POST /binocolo/v1/ma/sessions/{id}/no-website`, body `{"companyKey": "..."}`: stessa ossatura durevole dell'associate-domain (estendere `maAssociateDomainJobPayload` con `NoWebsite bool` oppure — preferito — un campo `Action string json:"action"` con valori `associate|group_site|no_website`, retro-compatibile: vuoto = associate). Il job scrive il registro e ri-gata la singola azienda (che ora prende il verdetto no-website e prosegue su enrich).
6. `maRouteTarget` (`ma_routing.go:37`): nessuna modifica necessaria (identity-only keep/forse → `maBucketDaVerificare` finché non arricchita; una volta scorata segue il routing normale). NON toccarlo.
7. UI marker: il frontend riconosce `webValidationState == "no_website_declared"` → badge "nessuna evidenza web" (usato in F4/F5).

**Nota deploy**: applicare la 086 prima del deploy (il CHECK del method viene esteso).

## B8 — Catalogo province con regioni (supporto R4)

**Obiettivo**: D1 deve compattare il recap territorio per regione. `provinces` di `@mrsmith/ui` (`packages/ui/src/data/provinces.ts`) ha SOLO `{code, name}` — **non** inventare una mappa regioni hardcoded nel frontend.

**Contesto verificato**: il backend ha già il catalogo province con regioni da OpenAPI.it, usato dal tool `list_italian_provinces_regions` (`maProvinceRegionTool`) con cache 30gg (`province_cache.go`). 

**Passi**: endpoint `GET /binocolo/v1/ma/catalog/provinces` → `{"items":[{"code":"MI","name":"Milano","region":"Lombardia"}, ...]}`, servito dalla stessa fonte cache del tool (seguire il percorso dati di `maProvinceRegionTool` in `ma_service.go` e riusarne il loader; NON chiamare OpenAPI.it senza passare dalla cache). Read-only, nessun costo (la cache esiste già per il flusso strategia).

---

## F1 — Rotte nuove, indice ricerche, tempo-1 TargetPage — PRD §8

**Contesto verificato**: `apps/binocolo/src/routes.tsx` — oggi `index → /target`, rotte: `target`, `azienda`, `ricerca-web`, `config`, `test`.

**Passi**:
1. Nuove rotte: `ricerche` (indice), `ricerche/nuova` (F2), `ricerche/:id` (F3/F4/F5). Default: `index → Navigate to="/ricerche"`.
2. **RicerchePage** (indice): lista sessioni via API esistente (imitare il caricamento sessioni della TargetPage: `GET /binocolo/v1/ma/sessions`, shape `{items}`), righe con titolo, stato (pill), data; click → `/ricerche/:id`; CTA primaria "Nuova ricerca" → `/ricerche/nuova`. Design: clean theme, empty state da `docs/UI-UX.md` §13.
3. **Tempo-1 TargetPage**: la rotta `/target` resta raggiungibile SOLO via URL diretto (rimuovere eventuali link di navigazione interna verso di essa); dentro TargetPage disabilitare la sola **creazione** di nuove sessioni (nascondere il form di creazione con un notice sobrio "Le nuove ricerche si creano da /ricerche/nuova"), lasciando intatto tutto il resto (consultazione e lavorazione sessioni esistenti). Modifica minima e reversibile: un flag/early-return sul blocco di creazione, NON uno smontaggio.

**Verifica**: tsc + smoke test (aprire `/ricerche`, `/target`).

## F2 — Pagina D1 `/ricerche/nuova` — PRD §2 tutto, wireframe S1–S6

**Fonte di verità**: PRD §2 (ogni comportamento è deciso lì) + wireframe approvato `apps/binocolo/docs/gated-ux-wireframe.html` (stati S1–S6: richiesta → perimetro riconosciuto → stima; degradati A/B/C). Componenti nuovi sotto `apps/binocolo/src/pages/ricerche/` — **NON importare componenti interni della TargetPage** (3.600 righe monolitiche): ricostruire lean, riusando solo `@mrsmith/ui` (`Button`, `Modal`, `MultiSelect`, `Skeleton`, `useToast`, `Icon`).

**Flusso dati (endpoint esatti)**:
1. S1 → `POST /binocolo/v1/ma/sessions` body `{prompt, gatedFlow: true}` (B3). La risposta è il session detail con `strategy.strategy` (spec completa). NIENTE selettori modello/prompt.
2. S2 dal detail: concetti da `strategy.sectorConcepts` + `sectorRetrievalMode` (B1/B2); chevron con divisioni/codici; tesi da `strategy.thesis` (chip read-only, mai selettore); territorio da `strategy.provinces` compattato per regione coi dati di `GET /binocolo/v1/ma/catalog/provinces` (B8; regione intera = tutte le sue province presenti); vincoli editabili (fatturato min/max, dipendenti min/max, ricavo/dipendente, soci, forme giuridiche, età titolare se tesi successione) → le modifiche passano da `POST /binocolo/v1/ma/sessions/{id}/estimate` con body `{strategy: {...}, strategyType: "expanded"}` (la strategia aggiornata viaggia con la stima, come fa oggi la TargetPage col campo `strategy` di `MAEstimateSessionRequest`).
3. Stati degradati (S6): `sectorRetrievalMode == "fallback_llm"` → variante C (divisioni dal catalogo, nota sobria, flusso attivo); `sectorConcepts` vuoto + missingCriteria con "Settore non riconosciuto dalla base di conoscenza ATECO" → variante A (bloccante, copy ratificato: "Il sistema è attualmente configurato per settori ICT e adiacenti."); nessun settore/solo esclusioni → variante B (bloccante, due messaggi distinti — PRD §2.2.1-B). In A e B il bottone stima NON è renderizzato.
4. Stima: polling del session detail su `session.status === 'estimating'` (pattern esistente); preview dai `detail.estimates` filtrati `strategyType === 'expanded'` — una riga per provincia (già aggregate server-side): totale = somma `estimatedCount`, ripartizione per provincia ordinata desc, "più di N" quando `surfaceStatus === 'too_broad'` con `probeCount > 1`. Tre stati (PRD §2.7.1): eseguibile / troppo ampio (bloccante, nessun bottone conferma) / vuoto. **Mai** renderizzare `estimatedCost`.
5. Freshness: qualunque modifica a territorio/vincoli dopo la stima invalida la preview client-side (torna "Calcola stima"); il backend rifiuta comunque stime stantie all'avvio ("stale estimate").
6. Conferma → `POST /binocolo/v1/ma/sessions/{id}/gated-search` body `{strategyType: "expanded", limit: <searchLimit della strategia>}` (imitare la chiamata execute della TargetPage per la forma; l'endpoint gated è handler.go:135) → `navigate('/ricerche/'+id)`.
7. Guard-rail territorio (§2.3.1): avviso soft non bloccante quando le province selezionate appartengono a più di una regione o il territorio è vuoto (= tutta Italia), usando i dati B8.

**Copy**: prendere i testi verbatim da `gated-ux-wireframe.html` dove presenti (guida S1, esempio, messaggi A/B/C, preview) — sono già stati rivisti; non riscriverli creativamente.

**Verifica**: tsc + smoke test guidato: creare una sessione di prova con un prompt ICT reale, verificare pannello concetti, stima, preview. ⚠️ DB condiviso con dati reali: NON confermare l'esecuzione gated durante lo smoke test (fermarsi alla preview); usare la sessione di prova e segnalarla all'utente per pulizia.

## F3 — Pagina D2 `/ricerche/:id`, modalità run in corso — PRD §3.1, wireframe S7

**Passi**:
1. Poll di `GET /binocolo/v1/ma/sessions/{id}/gated-progress` (B4) ogni 4s mentre `stage ∉ {ready, failed}`; stop del polling altrimenti.
2. Render (wireframe S7): pill stato "Ricerca in corso" (pulse); tre stage card (Superficie / Gate semantico → etichetta utente **"Valutazione attività"** / Analisi e punteggio) con stato Completato/In corso/In attesa e contatori; funnel bar proporzionale ai bucket; contatori bucket con etichette utente (In tesi / Da approfondire / Da verificare / Fuori tesi); riga "La valutazione richiede tempo. La pagina si aggiorna da sola."
3. Coda ghost (R10): elenco read-only delle righe della coda (B5) con nota "disponibile a ricerca completata"; nessun bottone rimedio renderizzato mentre il run corre.
4. Stato `failed`: box danger col copy del wireframe ("Ricerca interrotta — … Le aziende già elaborate sono conservate…") + bottone "Riprendi la ricerca" → `POST .../gated-search` (resume idempotente).
5. `stage === 'ready'` → transizione alla modalità risultati (F4) senza reload.

## F4 — Pagina D2, modalità risultati + tab Fuori tesi — PRD §3.2, wireframe S8/S10 — dipende da F3

**Passi**:
1. Riepilogo compatto (summarystrip S8): superficie → oltre il gate → analizzate · coda · fuori tesi; numeri dal progress endpoint (`ready`) — chevron "dettaglio" ri-espande il funnel di F3.
2. Barra azioni: chip tesi + "Cambia tesi e ricalcola" → `POST /binocolo/v1/ma/sessions/{id}/rescore` body `{thesis}` (endpoint verificato, handler.go:126; opzioni tesi = quelle della TargetPage: successione, crescita, consolidamento, tuck_in, generico); Export → `POST .../export` `{format:'xlsx'}` (blob, imitare `exportXLSX` della TargetPage) + variante CSV; Approfondimento → `POST .../deep-dive` (handler.go:132) SOLO se già presente nella TargetPage senza conferme aggiuntive — replicarne il flusso di conferma esistente.
3. Tab Risultati: tabella dai `detail.targets` scorati (MatchState non vuoto), colonne Azienda / Prov. / Esito (bucket routing v3 dal campo che la TargetPage già usa per il routing — verificare il nome esatto nel tipo `MATarget` frontend prima di usarlo) / Punteggio / Preferenza (stelle: riusare l'interazione rating esistente della TargetPage: individuare l'endpoint chiamato e replicarlo). Chip esito: Azionabile / Da verificare / Soppresso. Drawer dettaglio azienda al click (riusare il pattern del detail panel: dati target + link dossier `/azienda`).
4. Badge "nessuna evidenza web" sulle righe con `webValidation.webValidationState === 'no_website_declared'` (B7).
5. Tab Fuori tesi (S10): target con bucket gate `scarta` — nome, motivo dal verdetto (`webValidation.finalDecision.reason` o campo equivalente presente nel payload), dominio giudicato (`webValidation.selectedDomain`), rimedio unico "Associa dominio corretto" → stesso flusso associate-domain di F5.

## F5 — Pagina D2, tab Coda di verifica — PRD §3.3–3.4, wireframe S9 — dipende da B5, B6, B7

**Passi**:
1. Tab con badge conteggio da B5. Righe: nome + motivo (`reason.detail` dal server, MAI ricostruito client-side) + bottoni rimedio dal campo `remedies` (mai dedotti client-side):
   - `associate_domain` → espansione inline con input dominio + "Associa" → `POST /binocolo/v1/ma/sessions/{id}/associate-domain` (body: imitare la chiamata esistente della TargetPage — verificare i nomi campo nel suo handler prima di scrivere il client).
   - `confirm_group_site` (primario quando presente) → `POST .../confirm-group-site` `{companyKey}` (B6).
   - `no_website` → modal di conferma col copy ratificato del wireframe S9 ("…dichiarata senza sito ufficiale. La dichiarazione è permanente… prosegue sui soli dati strutturati, è inclusa nell'analisi completa…") → `POST .../no-website` `{companyKey}` (B7).
   - `retry_search` → `POST .../gated-search` (resume).
2. Dopo un rimedio: la sessione va `running` → la riga mostra "Nuova valutazione in corso" (pulse) e la pagina ri-entra in polling progress; all'esito la riga esce dalla coda (il refetch di B5 la toglie).
3. Rimedi disabilitati quando `session.status === 'running'` (R10) con hint "disponibile a ricerca completata".

**Verifica F3–F5**: tsc + smoke test con una sessione esistente già completata (consultazione read-only; NON lanciare gated-search né rimedi reali sul DB condiviso durante lo smoke — verificare i soli render; le azioni si provano solo su indicazione dell'utente).

---

## Fuori perimetro di questo piano (esplicito, per evitare drift)

- Fase (c) dell'asimmetria identitaria (attivazione regola reject/assumed → manual_review): aspetta i numeri della fase (b).
- Tempo-2 TargetPage (rimozione): dopo che D2 è in uso.
- Motivo "sito irraggiungibile" nella coda: richiede un fatto nuovo (pagine lette) non persistito.
- Qualunque modifica a scoring v3, routing v3, soglie del gate, prompt LLM.

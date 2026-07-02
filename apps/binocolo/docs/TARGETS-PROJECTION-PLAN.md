# Binocolo — Piano proiezione target (perf sessioni a 1000 aziende)

> Emendamento post-`GATED-UX-IMPLEMENTATION-PLAN.md` (chiuso 2026-07-02). Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore. I riferimenti a simboli/file sono **verificati sul codice al 2026-07-02 (a piano gated chiuso)**: usare quelli, non inventarne. Se un simbolo citato non esiste più, fermarsi e segnalarlo.

## Problema (misurato sul codice)

`GetMASession` (`ma_store.go:330`) idrata SEMPRE tutti i target della sessione con `vendor_payload` (JSON vendor grezzo, per gli advanced è il payload IT-Advanced completo) + `MAWebValidation` completa per azienda (`domain_response` con candidati e snippet, `evidence_runs` = pagine lette, `candidate_match_analysis`) + evidence + ratings + outcomes: ~10–40KB per target arricchito, quindi **~10–40MB a 1000 target** (`maVendorLimit`, nuovo limite del flusso gated). La pagina D2 (`RicercaDetailPage.tsx`) polla ogni 4s **tre** endpoint — `GET /sessions/{id}` (detail completo al client), `/gated-progress` e `/verification-queue` (che chiamano `GetMASession` internamente, `ma_gated_progress.go:12` e `:75`) — cioè tre idratazioni complete ogni 4 secondi sul DB condiviso.

Il consumo reale del frontend (inventario verificato su `pages/ricerche/`): la tabella Risultati legge 8 campi; il drawer 12 campi + `evidence[]`; di tutta `webValidation` solo `webValidationState`, `selectedDomain`, `finalDecision.reason` (+ `finalAction`/stato usati da `helpers.ts` per `isGateReject`). `vendorPayload`, `evidenceRuns`, `domainResponse`, `deep`, `outcomes`, `adjustments`: **mai letti in D2**.

## Decisioni ratificate (non ri-discutere)

- **Proiezione, non paginazione**: 1000 righe leggere (~300B) si caricano in un colpo e si filtrano/ordinano client-side come oggi. Il peso è nei blob per-riga, non nel numero di righe.
- **Dettaglio singolo on-demand**: l'idratazione completa di UN target si carica all'apertura del drawer.
- **B4/B5 passano a un loader lite interno**: nessun cambiamento di contratto API, solo il costo.
- La TargetPage legacy (`/target`) NON si tocca: muore al "tempo 2" del piano strangler. Il suo flusso resta sul detail completo.
- **Nessuna migrazione**: tutto il piano è codice; lo schema attuale (mig 054: colonne scalari + JSONB separati su `binocolo.ma_target_web_validation`) rende la proiezione pura SQL.

## Regole globali per l'esecutore

1. **Database**: MAI connettersi ai DB degli env, né in lettura. (Questo piano non ha migrazioni.)
2. **Test**: NON aggiungere test nuovi. Verifica backend per ogni task: `cd backend && go build ./... && go vet ./internal/binocolo/ && gofmt -l internal/binocolo/` (stampa nulla) e `go test ./internal/binocolo/` verdi. Se il piano richiede di estendere l'interfaccia store, aggiornare i fake dei test esistenti al minimo indispensabile.
3. **Frontend**: type-check SOLO `pnpm --filter mrsmith-binocolo exec tsc --noEmit`. Smoke test UI a fine F: dev server già attivo (mai riavviare), skill `playwright-cli`, **solo lettura** su una sessione esistente completata (DB condiviso con dati reali: niente run, niente rimedi).
4. **Costi mai in UI utente**: `MATargetRow` non deve contenere campi di costo; i campi costo del session detail lean possono risultare vuoti — le pagine nuove non li leggono (vietato iniziare a leggerli).
5. **API**: path Go senza prefisso `/api` (`handle("GET /binocolo/v1/...")` in `handler.go`); URL pubblico `/api/binocolo/v1/...`.
6. **Non toccare**: `ma_routing.go`, `sectorActionToBucket`, `gatedTargetBucket` (le derivazioni si INVOCANO, non si modificano); scoring v3; la TargetPage e la TestPage; le forme di risposta di `/gated-progress` e `/verification-queue`.
7. A fine task: file toccati + comandi di verifica con esito. Mai dichiarare fatto ciò che non è verificato.

## Ordine ed esecuzione

```
P1 (tipi + loader righe) → P2 (endpoint proiezione + dettaglio singolo)
                         → P3 (B4/B5 su loader lite)      [dopo P1]
P4 (session detail lean) [indipendente, dopo P1]
P5 (rewiring frontend)   [dopo P2, P3, P4]
```

---

## P1 — Tipo `MATargetRow` e loader di proiezione nello store

**Contesto verificato**: `loadMATargets` (`ma_store.go:1341`) è il loader pesante; le validation si agganciano per `company_key` da `loadMAWebValidations` (`ma_store.go:1746`, tabella `binocolo.ma_target_web_validation`, mig 054: scalari `final_action`, `web_validation_state`, `selected_domain` + JSONB `final_decision`, `domain_response`); i rating da `loadMARatings` (`ma_store.go:1520`, tabella `ma_target_rating`, PK `(session_id, company_key)`).

**Passi**:
1. In `ma_types.go`:
   ```go
   // MATargetRow è la proiezione leggera di un target per liste e derivazioni:
   // niente vendor_payload, niente blob di validation, niente evidence.
   type MATargetRow struct {
       ID              string          `json:"id"`
       RunID           string          `json:"runId"`
       CompanyKey      string          `json:"companyKey,omitempty"`
       CompanyName     string          `json:"companyName"`
       VATCode         string          `json:"vatCode,omitempty"`
       Province        string          `json:"province,omitempty"`
       Town            string          `json:"town,omitempty"`
       AtecoCode       string          `json:"atecoCode,omitempty"`
       Score           int             `json:"score"`
       ScoreVersion    *int            `json:"scoreVersion,omitempty"`
       MatchState      string          `json:"matchState"`
       Confidence      string          `json:"confidence,omitempty"`
       Bucket          string          `json:"bucket,omitempty"` // derivato a lettura, mai persistito
       Rating          *int            `json:"rating,omitempty"`
       Flags           []MATargetFlag  `json:"flags,omitempty"`
       EnrichmentLevel string          `json:"enrichmentLevel,omitempty"`
       WebValidation   *MATargetRowWeb `json:"webValidation,omitempty"` // nil = mai gatata
   }
   // MATargetRowWeb replica i SOLI percorsi JSON di MAWebValidation consumati
   // dalle liste, così il frontend legge row.webValidation.finalDecision.reason
   // con la stessa forma del detail completo.
   type MATargetRowWeb struct {
       WebValidationState string                      `json:"webValidationState"`
       FinalAction        string                      `json:"finalAction"`
       SelectedDomain     string                      `json:"selectedDomain,omitempty"`
       FinalDecision      MATargetRowFinalDecision    `json:"finalDecision"`
       // Campi interni per le derivazioni di B4/B5 (coda): mai serializzati.
       GroupSiteDomain     string `json:"-"`
       GroupSiteIdentifier string `json:"-"`
       CandidateCount      int    `json:"-"`
   }
   type MATargetRowFinalDecision struct {
       Reason string `json:"reason,omitempty"`
   }
   ```
2. In `ma_store.go`: `ListMATargetRows(ctx, sessionID string) ([]MATargetRow, error)` — UNA query:
   ```sql
   SELECT t.id::text, t.run_id::text, COALESCE(t.company_key,''), t.company_name,
          COALESCE(t.vat_code,''), COALESCE(t.province,''), COALESCE(t.town,''),
          COALESCE(t.ateco_code,''), t.score, t.score_version, COALESCE(t.match_state,''),
          COALESCE(t.confidence,''), t.flags, COALESCE(t.enrichment_level,'advanced'),
          r.rating,
          wv.company_key IS NOT NULL AS has_validation,
          COALESCE(wv.web_validation_state,''), COALESCE(wv.final_action,''),
          COALESCE(wv.selected_domain,''),
          COALESCE(wv.final_decision->>'reason',''),
          COALESCE(wv.domain_response->'groupSiteHint'->>'domain',''),
          COALESCE(wv.domain_response->'groupSiteHint'->>'identifier',''),
          COALESCE(jsonb_array_length(NULLIF(wv.domain_response->'candidates','null'::jsonb)),0)
   FROM binocolo.ma_target t
   LEFT JOIN binocolo.ma_target_web_validation wv
     ON wv.session_id = t.session_id AND wv.company_key = t.company_key
   LEFT JOIN binocolo.ma_target_rating r
     ON r.session_id = t.session_id AND r.company_key = t.company_key
   WHERE t.session_id = $1::uuid
   ORDER BY t.score DESC NULLS LAST, t.company_name
   ```
   (stesso ordinamento di `loadMATargets`; verificare i nomi colonna reali su `loadMATargets` e mig 054 prima di scrivere; `score NULL` → 0 come oggi). `has_validation=false` → `WebValidation = nil`.
3. `ListMATargetRows` va aggiunto all'interfaccia store usata da `maService` (stessa zona di `GetMASession`); aggiornare i fake dei test esistenti.

**Non fare**: non toccare `loadMATargets`; non aggiungere campi costo; non serializzare i campi `json:"-"`.

**Done when**: build/vet/test verdi.

## P2 — Endpoint proiezione e dettaglio singolo — dipende da P1

**Contesto verificato**: `getSession` (`ma_service.go:212`) deriva il bucket con `maRouteTarget(target, thesis)` dove `thesis = detail.Strategy.Strategy.Thesis`; la strategia attiva si carica con `loadActiveMAStrategy`. `maRouteTarget` (`ma_routing.go:37`) consuma SOLO: `MatchState`, `Confidence`, `Flags`, `WebValidation.{FinalAction, WebValidationState}` (+ nil-ness) — tutti presenti nella riga.

**Passi**:
1. Helper in `ma_gated_progress.go` (o file nuovo `ma_target_rows.go`):
   ```go
   // rowAsTarget costruisce il MATarget minimo sufficiente per le derivazioni
   // maRouteTarget/gatedTargetBucket (che NON vanno modificate).
   func rowAsTarget(row MATargetRow) MATarget
   ```
   — mappa `MatchState`, `Confidence`, `Flags` e, se `row.WebValidation != nil`, un `&MAWebValidation{FinalAction, WebValidationState, SelectedDomain}` più `DomainResponse.GroupSiteHint` (da `GroupSiteDomain`/`GroupSiteIdentifier` se non vuoti) e `DomainResponse.Candidates` fittizi di lunghezza `CandidateCount` (bastano elementi zero-value: le derivazioni contano solo `len`). Servirà anche a P3.
2. Service `sessionTargetRows(ctx, sessionID)`: `GetMASessionState` (esiste, `ma_store.go:363`; rifiutare deleted come `getSession`), `loadActiveMAStrategy` per la thesis (soft: senza strategia thesis vuota), `ListMATargetRows`, poi per ogni riga `row.Bucket = maRouteTarget(rowAsTarget(row), thesis)`.
3. Store `GetMATargetByID(ctx, sessionID, targetID string) (MATarget, error)`: stessa idratazione COMPLETA di un elemento di `loadMATargets` (validation, evidence, rating, outcomes, deep) ma `WHERE t.session_id=$1 AND t.id=$2`. Riusare le stesse funzioni di attach filtrando per la singola azienda dove possibile; NON caricare gli altri target della sessione. Service `sessionTargetDetail`: deriva `Bucket` come sopra. 404 su assenza (`sql.ErrNoRows` → not found in handler, imitare i pattern esistenti).
4. Handler + rotte in `handler.go` accanto alle altre ma/sessions:
   - `handle("GET /binocolo/v1/ma/sessions/{id}/targets", ...)` → `{"items": []MATargetRow}`
   - `handle("GET /binocolo/v1/ma/sessions/{id}/targets/{targetId}", ...)` → `MATarget` (stessa forma di un elemento di `detail.targets`: il drawer non cambia tipi).

**Non fare**: nessuna paginazione, nessun parametro di filtro server-side (filtri restano client-side); non toccare `maRouteTarget`.

**Done when**: build/vet/test verdi; `curl` locale dei due endpoint su una sessione dev restituisce righe senza `vendorPayload`/`evidenceRuns` e il dettaglio singolo completo.

## P3 — `gated-progress` e `verification-queue` sul loader lite — dipende da P1/P2

**Contesto verificato**: `gatedProgress` e `verificationQueue` (`ma_gated_progress.go:8` e `:71`) chiamano `s.store.GetMASession` e usano: `detail.Runs` (primo = più recente, `latestMAGatedRun`), `detail.Session.Status`, e per ogni target `WebValidation` (nil-ness, `FinalAction`, `WebValidationState`), `MatchState`, `EnrichmentLevel`, `RunID`, più — per la coda — `GroupSiteHint{Domain,Identifier}`, `len(Candidates)`, `CompanyKey/CompanyName/Province/ID`.

**Passi**:
1. Esporre un loader runs nello store se non già nell'interfaccia (oggi `loadMARuns` è privato, usato da `GetMASession`): `ListMARuns(ctx, sessionID)` con lo stesso ordinamento (il primo è il run più recente — verificarne l'ORDER BY prima di dipenderne).
2. Riscrivere i due metodi su: `GetMASessionState` + `ListMARuns` + `ListMATargetRows`, con `rowAsTarget` per `gatedTargetBucket`/`verificationQueueReason`. **La semantica di derivazione resta IDENTICA** (stessi switch, stessa approssimazione "validation stantia conta come processed", stessi testi dei motivi): è un cambio di sorgente dati, non di logica. `targetsForRun` diventa un filtro su `row.RunID`.
3. Le forme di risposta NON cambiano (contratti già consumati da D2).

**Non fare**: non cambiare testi/kind/remedies della coda; non leggere trace.

**Done when**: build/vet/test verdi; su una sessione completata dev, progress e coda restituiscono gli stessi numeri di prima del cambio (confronto manuale prima/dopo con curl).

## P4 — Session detail lean (`?targets=none`) — dipende da P1 (indipendente da P2/P3)

**Obiettivo**: le pagine nuove non devono più scaricare (né far idratare) i target dentro `GET /sessions/{id}` e dentro le risposte dei POST che ritornano il detail.

**Contesto verificato**: `getSession` (`ma_service.go:212`) → `GetMASession` → `loadMATargets`. I POST `rescore`, `gated-search`, `deep-dive`, `estimate`, `create` rispondono con il session detail via `getSession`/equivalenti (verificare i punti esatti in `handler.go`/service).

**Passi**:
1. Store: `GetMASessionLean(ctx, id) (MASessionDetail, error)` — come `GetMASession` ma SENZA `loadMATargets` (`Targets: []MATarget{}`). Riusa `loadMASession`/`loadActiveMAStrategy`/`loadMAEstimates`/`loadMARuns`.
2. Service: `getSessionLean(ctx, id)` — come `getSession` (deleted check, `ScoringPlan`, `decorateMACost`) ma su store lean e senza il loop bucket. Se `decorateMACost` dipende dai target, accettare valori a zero: le pagine nuove non leggono i campi costo (regola 4).
3. Handler: su `handleGetMASession` e sui POST che rispondono col detail usati dalle pagine nuove (`rescore`, `gated-search`, `deep-dive`, `estimate`, `create`), leggere `r.URL.Query().Get("targets") == "none"` → rispondere col lean. Default (param assente) = comportamento attuale, la TargetPage legacy non cambia.

**Non fare**: non cambiare la forma JSON (il lean risponde `targets: []`); non deprecare il default.

**Done when**: build/vet/test verdi; `GET /sessions/{id}?targets=none` risponde senza target e in frazione del tempo.

## P5 — Rewiring frontend D1/D2 — dipende da P2, P3, P4

**Contesto verificato**: `RicercaDetailPage.tsx` — `loadAll` fa `Promise.all` su detail + progress + queue e polla ogni 4s finché `running`/stage attivo; la tabella Risultati legge `detail.targets`; il drawer `TargetDetailModal` legge il target già in memoria (incluso `evidence`); `helpers.ts` ha `targetsToCSV` e `isGateReject`. `NuovaRicercaPage.tsx` polla il detail ogni 2.5s durante `estimating`.

**Passi**:
1. `api/types.ts`: aggiungere `MATargetRow` (+ `MATargetRowWeb`, `MATargetRowFinalDecision` — solo i campi serializzati) e `MATargetListResponse { items: MATargetRow[] }`.
2. `RicercaDetailPage`:
   - Stato separato: `rows: MATargetRow[]` al posto di `detail.targets` per tabella Risultati e tab Fuori tesi (i campi consumati esistono tutti nella riga; `isGateReject`/`helpers` si adattano alla forma row — stessi percorsi JSON di webValidation).
   - `loadAll` → `GET /sessions/{id}?targets=none` + `/gated-progress` + `/verification-queue`. Le **righe** si caricano con `GET /sessions/{id}/targets`: al mount, alla transizione di `stage` verso `ready`/`failed`, e dopo ogni azione che cambia i dati (rescore, resume, rimedio andato a buon fine, rating no — vedi sotto). Durante il run NON si ripollano le righe ogni 4s: il funnel vive di progress/queue (wireframe S7).
   - Drawer: all'apertura `GET /sessions/{id}/targets/{targetId}` con skeleton; il rendering attuale non cambia (stesso tipo `MATarget`). Cache in-memory per target già aperti nella stessa visita della pagina.
   - Rating: update ottimistico della riga locale + `POST .../rating` esistente; su errore revert + toast.
   - POST `rescore`/`gated-search`/`deep-dive` chiamati con `?targets=none`; dopo l'esito refetch di righe+progress+queue.
   - CSV: se `targetsToCSV` è usato dalla UI nuova, adattarlo ai campi della riga; in alternativa (preferito se l'endpoint export supporta `format:'csv'` — verificarlo in `handleExportMASession` prima) passare all'export server-side.
3. `NuovaRicercaPage`: aggiungere `?targets=none` alle GET/POST del detail (creazione, polling stima, avvio gated). Le stime viaggiano nel detail lean (`Estimates` c'è).
4. `TargetPage.tsx` e `TestPage.tsx`: **zero modifiche**.

**Verifica**: `tsc` + smoke test read-only su una sessione completata esistente: tab Risultati e Fuori tesi popolati dalle righe, drawer che carica on-demand (visibile nel network), progress/coda invariati, e nel network **nessun** `vendorPayload`/`evidenceRuns` fuori dal drawer. Niente run né rimedi sul DB condiviso.

---

## Fuori perimetro (esplicito)

- Virtualizzazione della tabella (1000 righe DOM): solo se lo smoke test mostra lentezza di rendering — prima misurare, poi decidere (task separato).
- Accorpare progress+queue+rows in un endpoint unico: i contratti B4/B5 sono appena usciti, non si toccano.
- Paginazione/filtri server-side: non servono a questa scala.
- Tempo 2 TargetPage (rimozione + redirect): resta il piano strangler, non questo.

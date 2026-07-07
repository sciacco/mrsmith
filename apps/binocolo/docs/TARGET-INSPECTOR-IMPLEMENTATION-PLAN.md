# Binocolo — Piano di implementazione Target Inspector (workstream DX)

## Stato implementazione (aggiornato al 2026-07-05)

| Fase | Stato | Note / deviazioni |
|------|-------|-------------------|
| **F1** — Shell + fetch + header + tab bar | ✅ DONE | Marker 🔒 usa `Tooltip` design system (non `title` nativo) per affidabilità UX. |
| **F2** — T1 Sintesi + T5 Tesi + T6 Provenienza | ✅ DONE | Sub-copy «iniziativa «X»» omessa: l'endpoint lean di session espone solo `initiativeId`, non il title (richiederebbe fetch del board intero). |
| **F3** — T2 Punteggio | ✅ DONE | Ricostruzione `Σpoints × Πfactor` validata su target reale (Δ=0). |
| **F4** — T3 Web validation | ✅ DONE | Aggiunta `summary` card (mini-grid score components) oltre al wireframe. |
| **F5** — T4 Deep-dive | ✅ DONE ⚠️ | **Deviazione**: componenti deep ricchi creati in `inspector/deep/` ma **NON swappati** in `IniziativaCardDossierPage` (rendering card-dossier è volontariamente più basilare; swap = refactor con rischio regressione, posticipato). |
| **F6** — T7 Registro & rating | ✅ DONE | Aggiunta prop `readOnly` a `CompanyRegistrySection` (non-breaking). "Assegnato il {data}" / "Score al rating" omessi: dati non presenti su `MATarget.rating` / `MATargetOutcome`. |
| **F7** — T8 JSON grezzo | ✅ DONE | Viewer con toggle MATarget/solo non-null + Copia + size indicator. |
| **F8** — Voci di ingresso | ✅ DONE | **Rettifica piano**: la premessa "RicerchePage row menu" era sbagliata (le righe di `RicerchePage` sono sessioni, non target). Entry point reali: (1) link `Ispezione completa ↗` in fondo al `TargetDetailModal`; (2) icona `external-link` sulla riga target in `ResultsTable` (entrambi in `RicercaDetailPage`). |

**Verifica globale**: `pnpm --filter mrsmith-binocolo exec tsc --noEmit` verde. Tutti i tab T1–T8 popolati + 2 entry point (modal + riga target). Nessun backend toccato. Rotte smoke: `/ricerche/e95445c4-.../target/cc7dc7b0-.../inspect` (deep ready), `/ricerche/5d2b043d-.../target/8b717a73-.../inspect` (deep nil, web validation ricca).

**Emendamento**: 2026-07-07 — emendato il read-only (PRD §2/§5): unica mutazione ammessa il lancio dell'analisi approfondita da T4, decisione brainstorming sgancio funzioni.

---

> Esegue la specifica di `TARGET-INSPECTOR-PLAN.md` (iterazione 1, tutta [DECISO]) col wireframe `target-inspector-wireframe.html` (T1–T8 + stati A/B/C/D per T4). Frontend-only: l'endpoint `GET /binocolo/v1/ma/sessions/{id}/targets/{targetId}` torna già il `MATarget` completo (`vendorPayload`, `evidence`, `adjustments`, `flags`, `webValidation` intero, `deep` cached, `outcomes`) — verificato in `backend/internal/binocolo/ma_service.go:342` (`sessionTargetDetail`) e `ma_store.go:1990` (`GetMATargetByID`). Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore. Riferimenti a simboli/file verificati sul codice al 2026-07-05: usarli, non inventarne. Se un simbolo citato non esiste più, fermarsi e segnalarlo.

## Regole globali per l'esecutore (valgono per OGNI task)

1. **Leggere prima**: `TARGET-INSPECTOR-PLAN.md` (ogni task cita le sue sezioni), il wireframe `target-inspector-wireframe.html` (fonte di verità per layout, copy e marker 🔒: estrarre i testi verbatim, non riscriverli), `docs/UI-UX.md`. Le decisioni del PRD sono ratificate: non ri-discuterle.
2. **Database / backend**: MAI. Questo workstream è **frontend-only**. Nessuna migrazione, nessun endpoint nuovo, nessuna modifica a Go. Se un task sembra richiedere backend, fermarsi e segnalarlo.
3. **Test**: NON aggiungere test nuovi. Verifica backend mai richiesta qui. Verifica frontend per ogni task: `pnpm --filter mrsmith-binocolo exec tsc --noEmit` verde. Smoke test UI a fine task: **riusare il dev server già attivo** (mai riavviarlo — verificare prima con `make dev` status o checkando la porta). La rotta da verificare è la URL **frontend** dell'app binocolo, es. `/ricerche/{id}/target/{targetId}/inspect` (non una URL API). Verifica preferita via skill `playwright-cli` (comando: navigare la rotta del task, assert visivo dei nuovi elementi, **solo lettura** sul DB condiviso — niente rating/fatti/note reali). **Fallback se la skill non è disponibile**: smoke manuale al dev server URL (aprire la rotta nel browser, verificare il rendering, segnalarlo nel messaggio finale).
4. **Read-only**: l'inspector è **strettamente read-only** (PRD §2, §5). Nessuna mutazione, nessun bottone "Avvia" inline, nessuna azione che chiami endpoint POST/PUT/PATCH/DELETE. **Navigazione uscente ammessa**, tutta intra-app e read-only, esplicitamente limitata a:
   - `Apri dossier ↗` → `/iniziative/:id/dossier/:companyKey` (solo se `session.initiativeId` presente) — ovunque compaia;
   - `Apri strategia ↗` / `Apri ricerca ↗` → `/ricerche/${sessionId}` (T6 Provenienza);
   - `Vai alle ricerche ↗` → `/ricerche` (F5 stato D senza iniziativa, per l'aggancio — l'azione "Aggancia a iniziativa" vive solo in `RicerchePage`, riga 29).
   Mai link a `/azienda` (EA-1: tool standalone, non destinazione MA). Mai link che muta stato.
5. **API**: client `useApiClient()` da `../../api/client`; endpoint target `api.get<MATarget>(\`/binocolo/v1/ma/sessions/${id}/targets/${targetId}\`)` (pattern di `RicercaDetailPage.tsx:159`). Tipo `MATarget` da `../../api/types` (riga 831).
6. **Copy**: italiano B2B asciutto, verbatim dal wireframe dove presente. **Mai "D3" come copy utente** (PRD EA-1): "Apri dossier" = navigation link al card-dossier; `/azienda` non compare mai (quick review standalone, non destinazione MA). Marker 🔒 = puntino discreto (PRD §3 convenzione).
7. **Routing**: nuova rotta in `apps/binocolo/src/routes.tsx`: `{ path: 'ricerche/:id/target/:targetId/inspect', element: <TargetInspectorPage /> }`. Page in `apps/binocolo/src/pages/ricerche/TargetInspectorPage.tsx` (convenzione: session-scoped, vive con le pagine ricerca).
8. **Riuso**: i componenti deep (scorecard, valuation+bridge, brief) e il registro (`CompanyRegistrySection`) si **riusano** dal card-dossier, non si duplicano. Se non sono già estratti in modulo condiviso, estrarli (vedi F5/F6).
9. **Non toccare**: scoring v3, routing v3, gate UC2, `RicercaDetailPage` tranne il punto di ingresso di F8, `IniziativaCardDossierPage` tranne l'estrazione componenti di F5. `/azienda` resta intatto e scollegato.
10. A fine task: file toccati + comandi di verifica con esito. Mai dichiarare fatto ciò che non è verificato.

## Ordine di esecuzione e dipendenze

```
F1 (shell + fetch + header + tab bar) → F2, F3, F4, F5, F6, F7 (tab body)
F5 (deep) → prework estrazione componenti da IniziativaCardDossierPage
F8 (voci di ingresso) dopo F1 (deep link testabile) · indipendente dai body
```

Ordine consigliato: **F1 → F2 → F3 → F4 → F5 → F6 → F7 → F8**. F8 può essere anticipato subito dopo F1 se si vuole testare il deep link presto.

---

## F1 — Shell: rotta, banner, identità, tab bar, fetch + stati — PRD §2.1, §3 — ✅ DONE

**Contesto verificato**: `routes.tsx` (sopra); pattern `useApiClient` + `api.get<MATarget>` in `RicercaDetailPage.tsx:159`; tipo `MATarget` in `api/types.ts:831`; `MASession`/`MASessionDetail` per leggere `initiativeId` sono nello stesso file tipo.

**Passi**:
1. **Page `apps/binocolo/src/pages/ricerche/TargetInspectorPage.tsx`**: legge `id` (sessione) e `targetId` da `useParams`; fetch `api.get<MATarget>(\`/binocolo/v1/ma/sessions/${id}/targets/${targetId}\`)` via React Query (`useQuery`, key `['ma-target-inspector', id, targetId]`). Stati loading (`Skeleton`), error (danger box + back link a `/ricerche/${id}`), 404 (target non trovato → back link).
2. **Rotta in `routes.tsx`**: `{ path: 'ricerche/:id/target/:targetId/inspect', element: <TargetInspectorPage /> }` (dopo la rotta `ricerche/:id`).
3. **App-head** (reuse pattern di `RicercaDetailPage`): logo Binocolo + back link "← Ricerca · {titolo sessione}". Il `MATarget` non contiene il titolo sessione: **fetch separata obbligatoria** `api.get<MASessionDetail>(\`/binocolo/v1/ma/sessions/${id}?targets=none\`)` via React Query, key `['ma-session-lean', id]` (pattern già usato in `RicercaDetailPage` per il `loadStatus`). Mai modificare l'endpoint target per aggiungerlo (vincolo frontend-only, regola 2).
4. **Banner "Modalità ispezione"** fisso sotto l'app-head: costante cromatica viola (`--inspect` nel wireframe → mappare a token del design system o variabile CSS locale se assente). Copy verbatim dal wireframe: *"Modalità ispezione. Superficie di scoperta, non analista: dati grezzi e tecnici visibili. Read-only."* + link "Apri in nuova tab ↗" (usa `window.location.href` in un `<a target="_blank">`).
5. **Identità target + score block** fissi sopra la tab bar (verbatim dal wireframe stato CHROME): `companyName`, P.IVA/C.F., provincia, ATECO + descrizione; pill bucket (`bkt-pr`/`bkt-dv`/`bkt-az`/`bkt-so` da `target.bucket`), pill confidence (`conf med/high/low` da `target.confidence`), pill `matchState`, pill `enrichmentLevel`; score block con `target.score` + sub `score · v{scoreVersion}`. Marker 🔒 sui campi `activityStatus`, `confidence`, `matchState`, `enrichmentLevel`, `scoreVersion` (verifica elenco completo nel wireframe).
6. **Tab bar** a 8 tab (T1 Sintesi · T2 Punteggio · T3 Web validation · T4 Deep-dive · T5 Tesi · T6 Provenienza · T7 Registro & rating · T8 JSON). Stato attivo in `useState<TabKey>('sintesi')`. Body del tab attivo: ogni task successivo lo riempie; in F1 mostrare solo un placeholder `"[… corpo del tab …]"` per i tab non ancora implementati.
7. **Marker 🔒 helper**: piccolo componente `<HiddenField label="campo" />` che renderizza `<span class="hide" title="${label} — oggi non visibile all'analista" />` (puntino ● ~7px grigio + `title` nativo, PRD §3 convenzione). Esportarlo da un modulo `apps/binocolo/src/pages/ricerche/inspector/HiddenField.tsx` per riuso cross-tab.

**Done when**: `tsc --noEmit` verde; navigando a `/ricerche/{id}/target/{targetId}/inspect` si vede banner + identità + score + tab bar; il back link torna alla ricerca; 404 gestito. **Nessun backend toccato.**

---

## F2 — Tab 1 Sintesi + Tab 5 Tesi + Tab 6 Provenienza (tab leggeri) — PRD §3.1, §3.5, §3.6 — ✅ DONE

**Contesto verificato**: campi `target.rationale`, `target.webValidation?.selectedDomain/freshness`, `target.deep?.status/overallRAG` (via `deep.scorecard.overallRAG`), `target.adjustments`, `target.runId/sessionId/createdAt/scoreVersion/enrichmentLevel`, `target.webValidation.{pipelineVersion,llmModelId,llmPromptId,llmModel,updatedAt,updatedByEmail}`, `target.deep?.updatedAt`.

**Passi — T1 Sintesi**:
1. Rationale box (`target.rationale`, fallback `target.webValidation?.finalDecision?.reason`).
2. Grid 3 colonne: Numeri struttura (turnover/turnoverYear, employees, activityStatus 🔒), Web validation (selectedDomain, freshness 🔒, stato), Deep-dive (status, overallRAG, equity range da `deep.valuation.equityLow/High` se ready).
3. CTA **navigation link** `Apri dossier ↗` → `/iniziative/${initiativeId}/dossier/${companyKey}` con sub-copy `nell'iniziativa «{titolo}»`, **solo se `session.initiativeId` presente**; altrimenti nessuna CTA (PRD §3.1 + R-DX-2: mai trigger, solo link).

**Passi — T5 Tesi**:
1. Caveat box (verbatim wireframe): *"La lettura di tesi è context-scoped all'iniziativa. Questo target mostra solo il fattore di tesi applicato allo score in questa sessione."*
2. Adjustment `thesis-fit` se presente in `target.adjustments` (code che matcha `thesis-fit`); render come `rowline neutral` con factor.
3. `deep.brief.thesisReading` legacy (spesso vuoto): renderizzare solo se non vuoto, con caveat "legacy pre-v3".
4. Link `Apri dossier ↗` + sub `(lettura di tesi)` al card-dossier (se initiativeId).

**Passi — T6 Provenienza**:
1. Lista `target.{id, sessionId, runId}`, `createdAt`, `scoreVersion` 🔒, `enrichmentLevel`.
2. Blocco pipeline web: `webValidation.{pipelineVersion, llmModel, llmPromptId}`, `inputHash/keywordSetHash` 🔒, `updatedAt/updatedByEmail`.
3. `deep.updatedAt` se presente.
4. Link alla strategia: `Apri strategia ↗` → `/ricerche/${sessionId}` (la pagina ricerca mostra la strategia; non c'è rotta dedicata). `strategyVersionId` 🔒 mostrato come mono accanto al link.

**Done when**: `tsc --noEmit` verde; i 3 tab si popolano dal `MATarget`; CTA dossier compare solo se initiativeId.

---

## F3 — Tab 2 Punteggio — PRD §3.2 — ✅ DONE

**Contesto verificato**: `target.evidence[]` (MATargetEvidence: criterion/status/family/label/value/points/weight/sourcePath), `target.adjustments[]` (MATargetAdjustment: code/label/factor), `target.flags[]` (MATargetFlag: code/label/severity), `target.missingCriteria[]`.

**Passi**:
1. **Evidence table** completa (verbatim struttura wireframe T2): colonne Criterion / Stato (stmatch chip m/p/x/o) / Value / Points / Weight / Source. Righe da `target.evidence[]`. Footer `Σ punti additivi` con somma `points`.
2. **Adjustments card**: ogni `target.adjustments[]` come `rowline warn/neutral` con factor (`× 0,85`). Sotto: riga ricostruzione `Σpoints × Πfactor = score` (calcolo client-side: `(Σ points) * Π(factor)` arrotondato → confrontare con `target.score` per sanity).
3. **Flags card**: `target.flags[]` come `rowline warn/neutral` con severity.
4. **Missing criteria card**: `target.missingCriteria[]` come chip dashed.
5. Marker 🔒 sulle intestazioni card: `adjustments[]`, `flags[]`, `missingCriteria[]`.

**Done when**: `tsc --noEmit` verde; la ricostruzione `Σpoints × Πfactor` è coerente con `target.score` (se diverge, non bloccare: il wireframe è illustrativo; segnalare nel messaggio finale).

---

## F4 — Tab 3 Web validation — PRD §3.3 — ✅ DONE

**Contesto verificato**: `target.webValidation` (MAWebValidation in `ma_types.go:1165`): `selectedDomain/domainConfidence/domainScore/selectedDomainPayload/domainResponse/identityState/webScore/webConfidence/webValidationState/finalAction/finalDecision.reason/analystVerdict/Action/Confidence/freshness/staleAfter/expiresAt/keywordSet/summary/evidenceRuns[]/candidateMatchAnalysis/candidateMatchError/pipelineVersion/llmModelId/llmPromptId/llmModel/inputHash/keywordSetHash/updatedAt/updatedByEmail`.

**Passi**:
1. Grid 2 colonne: Domain resolution (selectedDomain, domainConfidence, domainScore, identityState) + Verdetto pipeline (webValidationState, finalAction, webScore/webConfidence, freshness chip fresh/stale/expired + `expiresAt`).
2. Keyword set card: `keywordSet` come chip (`kw`).
3. Evidence runs card: `evidenceRuns[]` come `rowline` con score colorato (ok/warn).
4. Candidate match analysis card: `summary`/`candidateMatchAnalysis` in rationale box + `finalDecision.reason`; sub-grid `analystVerdict/Action/Confidence` 🔒 + `llmModel/PromptId` + `updatedByEmail/updatedAt`. Se `candidateMatchError` non vuoto: danger box.
5. Marker 🔒 sui campi tecnici (freshness, identityState, domainConfidence, evidenceRuns, candidateMatchAnalysis, analystVerdict, pipelineVersion, llmModelId/PromptId, hash) — elenco completo nel wireframe T3.

**Done when**: `tsc --noEmit` verde; tutti i campi `webValidation` sono resi o marcati 🔒.

---

## F5 — Tab 4 Deep-dive (4 stati + polling + riuso componenti) — PRD §3.4 — ✅ DONE ⚠️ (vedi stato)

**Contesto verificato**: `target.deep` (MADeepAnalysis: status queued/running/ready/failed + scorecard/valuation/brief/costEur/errorCode/updatedAt). Componenti deep rendering oggi **inline** in `apps/binocolo/src/pages/iniziative/IniziativaCardDossierPage.tsx` (scorecard metriche+RAG, reconciliation, quality flags, valuation+bridge, brief a sezioni) — non ancora estratti in modulo condiviso. Pattern polling: `RicercaDetailPage` (polling 4s su status running via `useEffect`+`setInterval`) e `IniziativaBoardPage` (polling board 5s se `dossierStatus === 'working'`).

**Passi**:
1. **Prework — estrazione componenti condivisi**: estrarre da `IniziativaCardDossierPage.tsx` i blocchi deep in `apps/binocolo/src/pages/ricerche/inspector/deep/` (nuova cartella dentro binocolo, stessa app): `DeepScorecard`, `DeepValuation` (con bridge), `DeepBrief`, `DeepQualityFlags`, `DeepReconciliation`. **Importante**: l'estrazione deve essere **non-breaking** per `IniziativaCardDossierPage` (sostituire l'inline con i nuovi componenti, stesso rendering). Verificare con `tsc --noEmit` che il card-dossier compila e smoke-visivo che la pagina dossier renderizza uguale.
2. **Stato A — `deep.status == ready`**: comporre i componenti estratti con `target.deep`. Aggiungere footer tecnico `deep.updatedAt`, `deep.costEur` 🔒, `deep.errorCode`. CTA `Apri dossier ↗` (se `session.initiativeId`).
3. **Stato B — `deep.status == queued || running`**: empty state `Analisi completa in corso…` (pulse, reuse `.statuspill run` + `.pulse`), `deep.updatedAt`. **Polling**: `useEffect`+`setInterval` 5000ms che invalida la query React Query `['ma-target-inspector', id, targetId]` finché `deep.status` resta `queued/running`; cleanup su unmount e su transizione a `ready/failed/nil`.
4. **Stato C — `deep.status == failed`**: empty state `Analisi non riuscita` + `deep.errorCode` + `deep.updatedAt` + link `Apri dossier ↗` (se initiativeId).
5. **Stato D — `deep == nil`**: empty state `Nessuna analisi completa per questa azienda.` + percorso ramificato: se `session.initiativeId` → CTA `Apri dossier ↗ nell'iniziativa «{X}»`; altrimenti copy `Associa la ricerca a un'iniziativa per abilitare l'analisi completa.` + link `Vai alle ricerche ↗` → `/ricerche` (l'azione "Aggancia a iniziativa" vive solo in `RicerchePage` sulla riga della sessione — riga 29 `attachFor`; l'utente la trova dalla lista).
6. **Mai bottone "Avvia" inline** in nessuno stato (PRD §3.4, R-DX-2 opzione D).

**Done when**: `tsc --noEmit` verde; `IniziativaCardDossierPage` compila e renderizza invariato (verifica smoke); i 4 stati sono navigabili (forzare `deep` con mock locali in dev se serve, ma preferire target reali con deep ready/nil); il polling si ferma a `ready`.

---

## F6 — Tab 7 Registro & rating (read-only mirror) — PRD §3.7 — ✅ DONE

**Contesto verificato**: `target.rating` + `target.outcomes[]` (MATargetOutcome); `CompanyRegistrySection` in `apps/binocolo/src/pages/iniziative/CompanyRegistrySection.tsx` ha signature `({ companyKey, vatCode, companyName })` — riusabile **così com'è** (è già read + write, ma nel contesto inspector le sue azioni di scrittura vanno **disabilitate** o omesse: verificare se accetta una prop `readOnly` — se no, aggiungerla o creare un wrapper read-only).

**Passi**:
1. Rating block: `target.rating` come stelle (reuse `RatingStars` pattern di `RicercaDetailPage`, ma **sola lettura**: niente `onRate`). Sub `Assegnato il {data}` se disponibile; `Score al rating: {score}` se presente negli outcomes.
2. Outcomes list: `target.outcomes[]` come timeline read-only (data + testo + badge evento). Reuse pattern di `IniziativaBoardPage` drawer eventi.
3. Registro: importare `CompanyRegistrySection` e renderirlo in modalità **strettamente read-only**. Se il componente non espone `readOnly`: aggiungere prop opzionale `readOnly?: boolean` che nasconde i composer ("+ Registra fatto", "Aggiungi nota") e i bottoni "Revoca". L'aggiunta della prop è non-breaking per il card-dossier (default `false`).
4. Sub-copy: `Diario completo e IRL nel dossier di lavorazione.` + link `Apri dossier ↗` (se initiativeId).

**Done when**: `tsc --noEmit` verde; registro + rating + outcomes si vedono; nessuna azione di scrittura è disponibile (verifica smoke: i composer non ci sono o sono disabilitati).

---

## F7 — Tab 8 JSON grezzo — PRD §3.8 — ✅ DONE

**Contesto verificato**: l'intero `MATarget` è già in mano alla page (F1). Niente endpoint aggiuntivi.

**Passi**:
1. Viewer collassabile dell'intero `MATarget` (JSON.stringify, pretty-print 2 spazi). Dark theme (mono, syntax highlight leggero opzionale ma non obbligatorio).
2. Toggle `MATarget` / `solo non-null` (filtra ricorsivamente i campi `null`/`undefined`/vuoti) + bottone `Copia` (clipboard).
3. Header con size indicator (es. `~ 14 KB`).
4. Deve esporre **l'intera risposta** endpoint, senza filtri nel modo default (PRD §1.2 acceptance boundary).

**Done when**: `tsc --noEmit` verde; il JSON matches il payload reale dell'endpoint (confronto con `curl /api/binocolo/v1/ma/sessions/{id}/targets/{targetId}` in dev, sola lettura).

---

## F8 — Voci di ingresso "Ispezione completa ↗" — PRD §2.1 — ✅ DONE (con rettifica)

**Contesto verificato**: `TargetDetailModal` in `RicercaDetailPage.tsx` (riceve `row`/`target`); righe target nella `ResultsTable` di `RicercaDetailPage` (il `<tr>` clickabile apre il modal). `target.id` e `session.id` (param route) sono disponibili in entrambi i contesti.

> **Rettifica rispetto al piano originale**: il piano citava "RicerchePage row menu (azioni archive/restore/trash)" come secondo entry point. Quella premessa era **errata**: `RicerchePage` è la lista delle **sessioni**, non dei target — le azioni archive/restore/trash sono su righe di sessione. I target vivono solo in `RicercaDetailPage`. Il secondo entry point è quindi stato posizionato sulla riga target della `ResultsTable`.

**Passi**:
1. **In `TargetDetailModal`**: aggiungere in fondo al body un link `Ispezione completa ↗` (target `_blank`) → `/ricerche/${sessionId}/target/${target.id}/inspect`. Style: link secondario, non compete con le CTA principali del modal. Verificare che `sessionId` sia disponibile nel scope del modal (in `RicercaDetailPage` è `id` da `useParams`; passarlo come prop al modal se non già fatto).
2. **In `RicerchePage` row menu**: aggiungere azione `Ispezione completa` accanto alle azioni lifecycle (icona `search`/`eye`), che apre in nuova tab → `/ricerche/${sessionId}/target/${target.id}/inspect`. **Policy UI e rotta, separate**:
   - **Rotta**: funziona su qualunque target esistente, indipendentemente dal `visibility` della sessione (l'endpoint non cambia).
   - **Entry point UI**: mostrare l'azione **solo su righe con `visibility === 'active'`** (coerenza col ciclo di vita: le archive/deleted non offrono azioni operative; l'ispezione resta comunque raggiungibile via URL diretto o dal modal di `RicercaDetailPage` se l'utente ci naviga).
3. **Back link dalla pagina ricerca**: l'`RicercaDetailPage` non ha bisogno di modifiche (l'inspector ha il suo back link "← Ricerca").
4. Smoke: da una ricerca attiva con almeno un target, navigare row menu → "Ispezione completa" apre l'inspector in nuova tab con il target giusto; stesso dal modal di dettaglio target.

**Done when**: `tsc --noEmit` verde; le due voci di ingresso sono raggiungibili e aprono la rotta corretta.

---

## Fuori perimetro di questo piano (esplicito, per evitare drift)

- **Backend**: qualunque modifica a Go/endpoint/migrazioni. L'endpoint target esiste già e torna tutto (R-DX-1).
- **Trigger deep inline**: l'opzione D è ratificata (R-DX-2); nessun bottone "Avvia" nell'inspector.
- **Endpoint deep company-keyed puro** slegato dall'iniziativa: APERTO Fase 2 (PRD §8 punto 3).
- **Comparative across run/sessioni** (stessa `companyKey`, `scoreVersion` diverse): APERTO Fase 2 (PRD §8 punto 4).
- **Curation del modal di dettaglio target** (promozione di campi 🔒 all'analista): Fase 1, altro workstream.
- **Modifiche a `/azienda`**: resta standalone e scollegato (EA-1).
- **Test automatici nuovi**: non richiesti in questo workstream (regola 3).

# Binocolo — PRD Target Inspector (workstream DX)

> Draft progressivo — **iterazione 1 (2026-07-05)**. Esito del brainstorm 2026-07-05: i punti marcati **[DECISO]** sono ratificati in quella sede; **[PROPOSTA]** = da validare; **[APERTO]** = da sciogliere in un'iterazione successiva. Contesto: pipeline gated D1/D2 implementata (`GATED-UX-PRD.md`), proiezione `MATargetRow` e `MATarget` pienamente popolati dal backend, dossier D3 in lavorazione (`INIZIATIVE-PRD.md`).

## 1. Inquadramento

La pipeline di ricerca produce, per ogni target, un `MATarget` ricchissimo: score e sua scomposizione (evidence, adjustments, flags, missingCriteria, confidence), web validation completa (domain, identity, freshness, evidenceRuns, candidateMatchAnalysis), deep-dive cached (scorecard, valuation con bridge, brief LLM), vendorPayload grezzo, traccia di provenienza.

La superficie di ricerca oggi consuma quasi solo **score + bucket + stelle**: il modal di dettaglio mostra `rationale` e 6 evidence sminuzzate. La maggior parte del tesoro è invisibile.

Scopo di questo workstream: una superficie di **ispezione completa, read-only, multi-tab** che faccia emergere *tutto* ciò che la pipeline produce per un target di sessione. Non è una superficie analista: è lo strumento con cui **scoprire** cosa è disponibile e, da lì, **derivare** una versione curata (Fase 1, altro workstream).

### 1.1 Goal / non-goal

- **Goal**: renderizzare ogni campo prodotto dalla pipeline per un target session-scoped, in forma navigabile, onesta (raw incluso), marcatamente separata dalle superfici analista.
- **Non-goal**: curare la presentazione per l'analista (Fase 1, successiva); introdurre superfici di scrittura; sostituire il modal di dettaglio esistente; diventare il dossier D3.

## 2. Modello **[DECISO]**

- **Session-scoped**: l'inspector mostra cosa *questa ricerca* ha prodotto/veduto per un'azienda. La cornice di lavorazione nel tempo (registro, diario, tesi context-scoped, IRL, deep cached) vive nel **MA card-dossier** (`/iniziative/:id/dossier/:companyKey`). Il tool standalone `/azienda` **non è una cornice del flusso MA**: è una quick review di un'azienda qualunque via P.IVA, indipendente da MA (ratifica D-B del PRD Iniziative, emendamento EA-1). L'inspector linka il card-dossier quando la sessione ha un'iniziativa; non linka mai `/azienda` come destinazione di lavorazione MA.
- **Read-only**: nessuna superficie di scrittura. Rispetta il vincolo non-CRM e tiene l'inspector strumento di lettura pura. Rating, registro, diario vivono altrove.
- **Accesso**: chiunque con ruolo Binocolo. Niente flag/ruolo dedicato per ora (la separazione è dichiarata dal banner, non da un ACL).

### 2.1 Forma **[DECISO]**

- **Route dedicata** `/ricerche/:id/target/:targetId/inspect`, apribile in **nuova tab del browser** dal row menu e dal modal di dettaglio target ("Ispezione completa ↗", `target="_blank"`).
- Non modale full-screen: con questo numero di tab diventerebbe un'app-nell'app e romperebbe back/forward/condivisione. La nuova tab è onesta sul fatto che è una digressione dal flusso.
- **Banner "Modalità ispezione"** fisso in header: copy esplicito — superficie di scoperta, non analista, dati grezzi visibili.

## 3. Struttura: tab per "piano di fatti" **[DECISO]**

Coerente col PRD Iniziative (piano lavoro vs piano entità). Tab in ordine di costo cognitivo crescente. Ogni campo è marcato con il marker 🔒 se oggi non surfaced all'analista (mappa diretta della Fase 1 di curation).

**Convenzione marker 🔒** **[DECISO]**: puntino inline ● ~6px grigio + tooltip "Oggi non visibile all'analista", posizionale (uno per campo, niente legenda centralizzata), piccolo e discreto (niente testo rumoroso), **visibile a chiunque** con ruolo Binocolo. Non è un flag dev-only: rafforza la natura "scoperta" dell'inspector.

### 3.1 Tab 1 — Sintesi
"In 10 secondi, cos'è?"

| Path JSON | Surfaced? |
|---|---|
| `companyName`, `vatCode`, `taxCode`, `province`, `town` | ✅ |
| `activityStatus` | 🔒 |
| `atecoCode`, `atecoDescription` | ✅ (modal) |
| `score`, `scoreVersion` | score ✅, version 🔒 |
| `confidence` | 🔒 |
| `bucket` (principale/da_verificare/azionabile/soppresso) | ✅ (chip) |
| `matchState` | 🔒 |
| `rationale` | ✅ (modal) |
| `enrichmentLevel` | 🔒 |
| `webValidation.selectedDomain`, `webValidation.freshness` | dominio ✅, freshness 🔒 |
| `deep.status` | 🔒 |
| CTA: "Apri dossier D3" / "Avvia analisi completa" | — |

### 3.2 Tab 2 — Punteggio (il "perché")
Goal: ricostruire a occhio `score = Σ(points) × Π(factor)`.

| Path JSON | Surfaced? |
|---|---|
| `score`, `scoreVersion`, `confidence` | score ✅, 🔒 gli altri |
| `evidence[].criterion/label/value/status` | parziale (6 righe, 🔒 status) |
| `evidence[].points`, `evidence[].weight` | 🔒 |
| `evidence[].sourcePath` | 🔒 |
| `evidence[].family` | 🔒 |
| `adjustments[]` (code/label/factor) — viability, thesis-fit | 🔒 |
| `flags[]` (code/label/severity) | 🔒 |
| `missingCriteria[]` | 🔒 |

### 3.3 Tab 3 — Web validation

| Path JSON | Surfaced? |
|---|---|
| `selectedDomain`, `domainConfidence`, `domainScore` | dominio ✅, 🔒 il resto |
| `selectedDomainPayload` | 🔒 |
| `domainResponse` | 🔒 |
| `identityState` | 🔒 |
| `webScore`, `webConfidence` | 🔒 |
| `webValidationState`, `finalAction` | stato ✅ via bucket |
| `finalDecision.reason` | ✅ (modal) |
| `analystVerdict/Action/Confidence` | 🔒 |
| `freshness`, `staleAfter`, `expiresAt` | 🔒 |
| `keywordSet` | 🔒 |
| `summary` | 🔒 |
| `evidenceRuns[]` | 🔒 |
| `candidateMatchAnalysis` | 🔒 |
| `candidateMatchError` | 🔒 |
| `pipelineVersion`, `llmModelId/PromptId/Model`, `inputHash`, `keywordSetHash` | 🔒 |

### 3.4 Tab 4 — Deep-dive

Il deep cached è **company-keyed globale**: se l'azienda è già stata analizzata in qualunque altra sessione/iniziativa, qui sarà `ready` a costo zero. Il tab è pertanto soprattutto un **visualizzatore read-only del deep di sistema** per il target ispezionato. Mai trigger: la produzione del deep è un gesto di lavorazione che appartiene al card-dossier (PRD Iniziative §7, ratifica D-B).

**Stato A — `deep.status == ready`**: artefatto cached completo, riuso dei componenti di `IniziativaCardDossierPage`.

| Path JSON |
|---|
| `deep.scorecard.metrics[]` (group/label/value/unit/rag/tier) |
| `deep.scorecard.overallRAG` |
| `deep.scorecard.turnover/turnoverYear/ebitda/netWorth/pfn/atecoCode` |
| `deep.scorecard.reconciliation` (ebitdaPct/pfnPct/pfnProvenance/roePointsDiff) |
| `deep.scorecard.qualityFlags[]` (severity/label/evidence/ddQuestion) |
| `deep.valuation.*` (method, multiple, haircutPct, evLow/High, equityLow/High, sector, nFirms, source, sourceDate, caveat, prudentialEbitda, lowMethod) |
| `deep.valuation.bridge` (pfn/tfr/taxFund/shareholderLoans + equity) |
| `deep.brief.verdict/rag/businessProfile/financialReading` |
| `deep.brief.strengths[]`, `redFlags[]`, `valuationRationale`, `ddQuestions[]` |
| `deep.brief.thesisReading` (**legacy pre-v3**, spesso vuoto) |
| `deep.costEur` 🔒, `deep.errorCode` (vuoto se ready), `deep.updatedAt` |

Footer: "Deep cached aggiornato il {data}" + link **"Apri dossier ↗"** al card-dossier `/iniziative/:id/dossier/:companyKey` (se `session.initiativeId` presente) per la lavorazione completa.

**Stato B — `deep.status == queued || running`**: empty state "Analisi completa in corso…" (pulse) + `deep.updatedAt`. L'artefatto appare quando `status` diventa `ready` (polling del target). Link "Apri dossier ↗" opzionale.

**Stato C — `deep.status == failed`**: empty state "Analisi non riuscita" + `deep.errorCode` + `deep.updatedAt`. Link "Apri dossier ↗" (se iniziativa presente) per la ritentata nel suo contesto.

**Stato D — `deep == nil` (mai avviato)**: empty state "Nessuna analisi completa per questa azienda." + percorso per produrla, ramificato:
- se `session.initiativeId` presente → CTA **"Apri dossier ↗ nell'iniziativa «{X}»"** (al card-dossier, dove "Avvia analisi completa" ha il suo contesto legittimo);
- se `session.initiativeId` assente → "Associa la ricerca a un'iniziativa per abilitare l'analisi completa di questa azienda." (link al flusso di aggancio esistente in `RicerchePage`).

**Mai bottone "Avvia" inline** nell'inspector: coerente con l'opzione D (read-only) e col PRD Iniziative (deep = gesto di lavorazione, contesto card).

### 3.5 Tab 5 — Tesi (session-scoped, ridimensionato) **[DECISO con caveat]**

La `thesisReading` *vera* (fit, blockingFlags, synergyHypotheses, valuationStance, notAddressed) **è context-scoped all'iniziativa** (`MACardThesisReading`) e **non** vive sul target di sessione. Sul session target abbiamo solo:

| Path JSON | Nota |
|---|---|
| `adjustments[]` con codice thesis-fit | fattore di tesi applicato allo score |
| `deep.brief.thesisReading` | legacy pre-v3 (info residua, spesso vuota) |
| **Link**: "Lettura di tesi context-scoped → dossier D3" | fuori scope sessione |

Copy onesto nel tab: la lettura di tesi è prodotta per iniziativa; il dossier è il luogo.

### 3.6 Tab 6 — Provenienza & traccia

| Path JSON | Nota |
|---|---|
| `id`, `sessionId`, `runId` | chiavi |
| `createdAt` | quando è stato prodotto |
| `scoreVersion` | revisione scoring |
| `enrichmentLevel` | profondità analisi |
| `webValidation.pipelineVersion`, `llmModelId`, `llmPromptId`, `llmModel` | versione pipeline web |
| `webValidation.updatedAt`, `updatedByEmail` | ultimo aggiornamento validation |
| `deep.updatedAt` | ultimo aggiornamento deep |
| Link alla strategia (`strategyVersionId`) | per ricostruire il perimetro |

### 3.7 Tab 7 — Registro & rating (mirror D3, read-only)

| Path JSON | Nota |
|---|---|
| `rating` + `outcomes[]` | rating corrente + storico eventi |
| Registry facts/notes (endpoint registro azienda, già in D3) | badge + note |

Read-only; le scritture vivono nel dossier.

### 3.8 Tab 8 — JSON grezzo **[PROPOSTA]**
L'intero `MATarget` — inclusi `vendorPayload`, `webValidation` completo, `deep` — in un viewer collassabile. Verità a monte di qualunque decisione di curation. Copre anche il bisogno "vedere il payload vendor così come torna" (in precedenza tab dedicato, ora assorbito qui). Consigliato in Fase 0; rimuovibile in Fase 1 se ritenuto rumore.

## 4. Requisiti dati/API

| ID | Requisito | Stato |
|----|-----------|-------|
| R-DX-1 | Frontend-only per la v0: `GET /binocolo/v1/ma/sessions/{id}/targets/{targetId}` torna già il `MATarget` completo (`vendorPayload`, `evidence` con `points`/`weight`/`sourcePath`, `adjustments`, `flags`, `webValidation` intero, `deep` cached, `outcomes`) | DECISO — verificato in `sessionTargetDetail` / `GetMATargetByID` |
| R-DX-2 | Trigger deep nell'inspector | **DECISO opzione D**: nessun trigger. Il tab 4 è read-only sul deep cached (stati ready/queued/running/failed/nil). Link al card-dossier `/iniziative/:id/dossier/:companyKey` per la produzione. Endpoint company-keyed puro → APERTO Fase 2 |
| R-DX-3 | Endpoint registro azienda per facts/notes nel tab 8 | GIÀ esiste in D3 (riuso) |
| R-DX-4 | Route frontend `/ricerche/:id/target/:targetId/inspect` + voce "Ispezione completa ↗" in row menu e modal di dettaglio | DECISO |

## 5. Vincoli trasversali

- **Read-only**: nessuna mutazione dal substrate dell'inspector. Niente rating, niente esclusione, niente scrittura registro/diario.
- **Non-CRM**: niente task, reminder, email, assegnatari (coerente col PRD Iniziative §10).
- **Pipeline intoccata**: l'inspector è puro read; nessun effetto su gate UC2, routing v3, scoring.
- **Onestà del raw**: il vendorPayload e il tab JSON non vengono "addomesticati". È uno strumento di scoperta.
- **Confine con D3 dichiarato**: l'inspector linka il dossier per la roba context-scoped (tesi, diario, IRL); non la duplica.
- Design system `docs/UI-UX.md` (clean theme), copy asciutto, ma la superficie è esplicitamente non-analista (banner).

## 6. Riuso componenti

- **Tab 4 (deep)**: estrarre scorecard/valuation+bridge/brief da `CompanyDossierPage` e `IniziativaCardDossierPage` in componenti condivisi. L'inspector è il primo fruitore "puro" di quei pezzi.
- **Tab 8 (registro)**: riuso di `CompanyRegistrySection` in modalità read-only (già esiste in `IniziativaCardDossierPage`).
- **Tab 2 (evidence)**: rendering nuovo ma pattern `evidence` già consumato (top 6) nel modal di dettaglio: generalizzare.

## 7. Phasing

- **Fase 0 — Discovery** (questo PRD): inspector con tutti i tab, rendering read-only, marker 🔒 "nascosto", JSON grezzo. Niente backend nuovo. Qui si fa sul serio l'esplorazione su ricerche reali.
- **Fase 1 — Curation** (altro workstream): dal delta tra "disponibile" e "oggi surfaced", decidere cosa **promuovere** nel modal di dettaglio target. Candidati probabili: `confidence`, breakdown evidence completo, 1–2 segnali web chiave (`freshness`, `identityState`), link esplicito al dossier. Il modal attuale diventa la versione "curata per l'analista".
- **Fase 2 — Evoluzioni** (opzionale): viste comparative across run/sessioni (stessa `companyKey`, `scoreVersion` diverse), drill-down per criterio/famiglia.

## 8. Aperti per la prossima iterazione

1. ~~Forma precisa del marker 🔒 "nascosto in prod"~~ → **DECISO**: marker inline **piccolo e discreto** (puntino ● ~6px grigio + tooltip "Oggi non visibile all'analista", niente testo rumoroso), posizionale (uno per campo, non legenda centralizzata), **visibile a chiunque** con ruolo Binocolo — non è un flag dev-only, rafforza la natura "scoperta" dell'inspector.
2. ~~Tab 8 JSON grezzo~~ → **DECISO**: sì in v0 (assorbe `vendorPayload`, ex-tab Anagrafica grezza rimosso).
3. ~~Endpoint deep per-azienda (R-DX-2)~~ → **DECISO (opzione D)**: l'inspector è strettamente read-only, **nessun trigger deep inline**. Il tab 4 visualizza il deep cached negli stati ready/queued/running/failed e mostra un percorso (link al card-dossier `/iniziative/:id/dossier/:companyKey` se la sessione ha iniziativa; invito ad associare un'iniziativa altrimenti). La produzione del deep resta un gesto di lavorazione del card-dossier, coerente col PRD Iniziative §7 e la ratifica D-B. Endpoint deep company-keyed puro slegato dall'iniziativa: **APERTO Fase 2** (solo se la friction emerge dall'uso reale).
4. Visualizzazioni comparative across run (Fase 2): fuori scope v0. **[APERTO Fase 2]**
5. ~~Naming "Avvia analisi completa"~~ → **DECISO**: "Avvia analisi completa" (allineato a `INIZIATIVE-PRD.md` §7). Nell'inspector non compare mai come bottone (opzione D); compare solo come copy descrittivo negli empty state del tab 4.

> **Implementazione**: piano esecutivo da scrivere (`TARGET-INSPECTOR-IMPLEMENTATION-PLAN.md`) una volta ratificata questa iterazione — task di solo frontend (rotta, header con banner, 8 tab, riuso componenti deep/registro dal card-dossier), riferimenti verificati sul codice al 2026-07-05.

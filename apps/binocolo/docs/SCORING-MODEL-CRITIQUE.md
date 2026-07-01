# Binocolo — Critica al modello di scoring a 10 segnali fissi

> Analisi di tre esperti M&A, consolidamento e raccomandazioni per la valutazione.
> Redatto: 2026-07-01. Stato: **in valutazione — nulla implementato.**

---

## 0. Scopo del documento

Lo score 0–100 che ordina le aziende sopravvissute al funnel (`keep`+`forse`) è il prossimo componente su cui costruiremo la UI dell'analista. Prima di investirci, abbiamo commissionato **tre valutazioni critiche indipendenti** del modello attuale, ciascuna con una lente diversa (sostanza deal, rigore statistico, efficacia operativa). Questo documento raccoglie:

1. la **fotografia esatta** del modello com'è oggi (i fatti, con riferimenti al codice);
2. le **tre analisi** degli esperti, fedeli;
3. il **consolidamento** (convergenze, tensioni, l'insight chiave);
4. il **vincolo di stadio** (non abbiamo dati rappresentativi) e cosa implica;
5. le **raccomandazioni prioritizzate** e le **decisioni aperte** da valutare.

L'obiettivo è darti una base per decidere *cosa* toccare, *in che ordine*, e *cosa rimandare*.

---

## 1. Contesto: cos'è lo score e dove sta

Binocolo scherma aziende italiane da un vendor dati (OpenAPI.it) con un **funnel a costo controllato**:

```
fetch identità (economico) sull'intera superficie
   → gate semantico di settore  (keep / forse / manual_review / scarta)
   → enrichment "advanced" (~€0.10)  SOLO sui sopravvissuti (keep+forse)
   → QUESTO SCORE ordina i sopravvissuti 0–100
   → [stadio "deep" separato]  valutazione completa (EV/EBITDA, PFN, RAG) su pochi eletti
```

- **Il lavoro dello score:** aiutare l'analista a decidere *chi approfondire per primo* tra i (spesso 20–80) sopravvissuti. Non è una valutazione: l'EV/EBITDA vive nello stadio *deep*, a valle e a mano.
- **Dati disponibili allo score:** un payload vendor per azienda — forma giuridica, provincia, stato attività, data inizio, soci (nome/cognome/codice fiscale/quota) e una serie di bilanci (per anno: fatturato, patrimonio netto, attivo, dipendenti). **Niente** EBITDA, debito/PFN, margini, clienti, management. Niente sinergia con i clienti CDLAN (fase futura).
- **Rischio dominante del funnel:** i **falsi negativi** — un buon target scartato/sepolto — perché l'analista non lo vede mai.

Codice: `backend/internal/binocolo/ma_scoring.go`, `ma_catalog.go`, `ma_flags.go`; mappatura intento→strategia in `ma_service.go` (`resolveIntent`); esecuzione ricerca in `ma_service.go` (`runExecution`, `baseMASearchParams`); scoring nel funnel gated in `ma_gated_search_job.go` (`enrichAndScoreSurvivors`).

---

## 2. Il modello attuale (fotografia dei fatti)

### 2.1 Formula

```
score(0–100) = blended × viability × thesisFit × 100      (clamp ≤100)
```

- **`blended`** = media pesata di 10 segnali, **ri-normalizzata per azienda** sui soli segnali con dato presente. Dato mancante non penalizza: il suo peso si ridistribuisce sui segnali presenti.
- **`viability`** e **`thesisFit`** ∈ (0,1] = moltiplicatori *fuori* dalla ri-normalizzazione.
- I pesi di famiglia vengono dalla **tesi** di acquisizione.

### 2.2 I 10 segnali (`maSignalCatalog`, `ma_catalog.go:74`)

| id | Famiglia | Base | Attivo quando | Come misura |
|---|---|---|---|---|
| `ateco_precision` | Aderenza | 20 | esistono candidati ATECO | fit curato: core 1.0 / weak 0.45 / neutral 0.25 / excluded 0 (longest-prefix) |
| `turnover_proximity` | Aderenza | 15 | esiste una "taglia ideale" (`around`, o midpoint di min&max) | `|fatturato−ideale|/ideale` a bande |
| `keyword_match` | Aderenza | 5 | keyword o descrizione settore | ≥2 token significativi condivisi = pieno |
| `ownership_concentration` | Opportunità | 12 | sempre | socio unico/≥90% =1.0, ≥50% =0.7, altrimenti 0.4 |
| `company_age` | Opportunità | 10 | tesi ≠ generico | crescita premia giovani; altre premiano anziane |
| `legal_form` | Opportunità | 8 | **solo se** nessun vincolo di forma dato | capitali 1.0 / persone 0.6 / individuali 0.3 |
| `succession_owner` | Opportunità | 12 | **solo** tesi successione | rampa sull'età del socio-persona di maggioranza vs soglia |
| `turnover_trend` | Economico | 12 | sempre | **percentile** del CAGR nel pool dei sopravvissuti |
| `productivity` | Economico | 8 | sempre | **percentile** di fatturato/dipendente nel pool |
| `equity_solidity` | Economico | 10 | sempre | patrimonio netto/attivo a bande |

Il "Base" è la quota *dentro* la famiglia; il peso di famiglia (dalla tesi) è ripartito proporzionalmente al Base tra i segnali *intended* di quella famiglia (`maSignalNominalWeights`, `ma_catalog.go:124`).

### 2.3 Pesi di famiglia per tesi (`thesisFamilyWeights`, somma 100)

| Tesi | Aderenza | Opportunità | Economico |
|---|---|---|---|
| successione | 30 | **50** | 20 |
| crescita | 35 | 20 | **45** |
| consolidamento | **50** | 25 | 25 |
| tuck-in | 45 | 30 | 25 |
| generico | 40 | 30 | 30 |

### 2.4 Moltiplicatori fuori dal blend

- **`viability`** ∈ (0,1] (`computeViability`, `ma_scoring.go:425`) = `statusFactor × balanceFactor × equityFactor`:
  - `statusFactor`: 0.2 se stato ≠ ATTIVA; **0 (knockout duro)** se cessata.
  - `balanceFactor`: **0.3 se nessun fatturato depositato**; altrimenti degrada 1.0→0.25 con l'età dell'ultimo bilancio (2→6+ anni; lag ≤2 anni fisiologico, nessuna penalità).
  - `equityFactor`: 0.3 se PN negativo; 0.6 se eroso >70% YoY.
  - Se il prodotto < 0.25 → **knockout** a `fuori_criterio`.
- **`thesisFit`** ∈ (0,1] (`computeThesisFit`, `ma_scoring.go:478`) = 1.0 **tranne** sotto tesi successione, dove applica un taglio se una holding/società controlla ≥50% (un titolare-persona in uscita è la tesi; una holding ne è l'antitesi).

### 2.5 Post-filtri knockout + gate settore

- **Post-filtri** (`maPostFilterEvidence`, `ma_scoring.go:238`) → `fuori_criterio`, **non graduati**: `revenue_per_employee_min`, `max_shareholders`.
- **Gate settore** (`inSectorPerimeter`, `ma_scoring.go:580`): fuori dalle divisioni ATECO a 2 cifre (o sotto un sottoalbero escluso) → `fuori_criterio`.

### 2.6 Confidence = coverage (NON tocca lo score)

`coverage = pesoAttivo / pesoIntended` → etichetta `alta≥0.8 / media≥0.5 / bassa`. **Non entra nello score né nell'ordinamento** (sort puro per score, `ma_scoring.go:187`; tie-break alfabetico per nome).

### 2.7 Cosa NON è onorato nello score (dalla query iniziale)

- **Filtri duri a monte** (ogni sopravvissuto già li rispetta, quindi non ri-valutati): province, stato attività, fatturato min/max, dipendenti min/max, forme giuridiche, perimetro ATECO.
- **Mai onorato / solo come stringa in `MissingCriteria`** (`ma_service.go:2088`): tutti i **vincoli free-form** (ricavi ricorrenti %, certificazioni, export %, concentrazione clienti, IP…); l'**età titolare massima** (mai applicabile); l'età titolare è graduata **solo** sotto tesi successione.
- **Fatturato con un solo estremo** (solo min *o* solo max, senza "around") → `turnover_proximity` non si attiva (taglia la superficie ma non premia la taglia).

---

## 3. Le tre analisi degli esperti

Tre agenti specializzati hanno ricevuto **gli stessi fatti esatti** (sopra) e hanno prodotto valutazioni indipendenti.

### 3.1 Lente A — Sostanza deal/tesi (profilo: analista M&A / search fund / acquirente strategico)

**Verdetto.** Come *prioritizzatore di sopravvissuti per il deep-dive* — non motore di valutazione — il modello è fondamentalmente sano e, cosa cruciale, **spiegabile**: deterministico, ricostruibile dalle evidence-row, cost-appropriate, onesto su ciò che non vede. Quella trasparenza vale più della sofisticazione allo stadio di screening → non ribaltare l'architettura. Ma ha tre difetti M&A sostanziali: (1) lascia a terra il miglior proxy di redditività disponibile nei dati — il compounding del patrimonio netto — quindi ha *zero* segnale di redditività pur avendone la materia prima; (2) due segnali economici sono solo-percentile, che fabbrica falsi positivi nei pool deboli ("miglior declinante" in cima); (3) la coverage non tocca né score né ordinamento, quindi un target a dati radi può battere uno pienamente documentato. Spedibile come aiuto al triage, **non** come qualcosa che un comitato investimenti dovrebbe leggere come ranking di qualità senza quei fix.

**Da tenere.**
- Il **posizionamento nel funnel e la separazione dei ruoli** (gate economico → enrichment pagato → ordina sopravvissuti → valutazione deep a mano). Questo score correttamente *non* finge di fare EV/EBITDA. Altitudine giusta.
- **Viability come strato di distress moltiplicativo fuori dal blend**: cessata = knockout duro, taper su stato/età-bilancio/PN-negativo, floor a 0.25. Esattamente come si evita che gusci morti/dormienti/insolventi vengano ri-pesati in un buon punteggio.
- **Il dato mancante ri-normalizza invece di penalizzare**, con un'etichetta di coverage separata. Istinto giusto — non punisci un'azienda per i buchi di deposito del vendor.
- **Pesi di famiglia guidati dalla tesi + gating `intended`** (`succession_owner` solo-successione, `legal_form` solo se non vincolata). Lo scheletro per la thesis-specificità c'è già.
- **`thesisFit` con taglio-holding per la successione** — riconoscere che una società controllata da holding è l'*antitesi* di un titolare-persona in uscita è un tocco genuinamente M&A-literate.

**Gap critici (più dannoso prima).**
1. **Nessun proxy di redditività, benché derivabile.** Senza EBITDA, il modello tratta la serie di bilanci come sola dimensione + solidità. Ma il **CAGR del patrimonio netto lungo la serie è il miglior proxy di redditività disponibile** — PN che compone con fatturato stabile/in crescita ≈ utile trattenuto, auto-finanziato. È il numero che un compratore lower-mid-market guarda per primo quando il conto economico è sottile. Pienamente derivabile, oggi inutilizzato. (Caveat: iniezioni di capitale/dividendi lo distorcono — etichettarlo proxy.)
2. **Trend/produttività solo-percentile distorcono nei pool deboli.** Rankati *dentro il set dei sopravvissuti*. In un settore in declino/omogeneo il top-percentile "crescitore" può calare del −5%/anno e segnare comunque ~1.0 — nessun floor assoluto. Rende anche i punteggi **non-monotoni e non-spiegabili**: allargare la ricerca rimescola i sotto-score di tutti. Passività lato comitato. (`percentileRank` torna 0.5 per n≤1 e bucket grossolani per pool minuscoli — comune dopo un gate stretto.)
3. **La coverage non tocca né score né ordinamento.** Un target che segna 0.9 blended su 2 dei 10 segnali applicabili (peso ri-normalizzato al 100%) batte uno che segna 0.85 su 9 su 10. Viability protegge in parte il caso *senza-finanziari* (balanceFactor 0.3 → score ~30), ma un target a cui mancano solo trend+produttività+keyword schiva lo scrutinio economico senza penalità né costo di rank. Selezione avversa contro le aziende ben documentate.
4. **`turnover_proximity` premia la forma sbagliata, e i mandati solo-min non prendono alcun segnale di taglia.** Da un range min/max l'"ideale" è il midpoint e il punteggio picca lì — quindi la *più grande in range* segna come la *più piccola* (entrambe ~0.55), sotto una di taglia media. Per ogni tesi tranne il tuck-in, più-grande-in-range è strettamente meglio (più EBITDA, muove l'ago). Peggio: `maTurnoverIdeal` torna 0 per un mandato **solo-min** ("almeno €5M di ricavi", comunissimo), quindi il segnale di taglia svanisce in silenzio.
5. **La successione doppio-conta la maturità e ignora il vero discriminante.** Sotto successione, `company_age` (~12% nominale) e `succession_owner` (~14%) proxano entrambi "vecchio/che invecchia" (~26% combinato), mentre la cosa che davvero predice una *vendita* — **nessun erede più giovane già nella compagine** — vale 0. Derivabile: stesso cognome + età da codice fiscale più giovane = successione interna probabile, *minore* acquisibilità. Il flag `impresa_familiare` già rileva il cognome ripetuto ma lo tratta come neutro.
6. **La geografia (`province`) non è mai valutata.** Per consolidamento e tuck-in, la complementarità/densità geografica è un segnale di deal di primo ordine, nel payload e inutilizzato.
7. **La salute patrimoniale è tripla-contata** (`equity_solidity` → 0, viability `equityFactor` 0.3/0.6, più flag) mentre il trend di redditività è zero-contato. Sotto *crescita*, `equity_solidity` netta ~15% nominale — più di `productivity` (~12%) — al contrario per una tesi di crescita.

**Cambiamenti concreti.**
- **P0 — Aggiungere `equity_compounding`** (Economico, Base ~10): CAGR del PN sulla serie come proxy di redditività, a bande assolute, etichettato "proxy (netto iniezioni/dividendi)". Finanziato tagliando `equity_solidity` Base 10→6 (viability già presidia l'insolvenza).
- **P0 — Floor assoluto sotto i percentili.** Tenere il percentile come *tiebreak* ma floor duro: CAGR negativo del fatturato limita il sotto-score trend a ≤0.3 a prescindere dal rank; produttività a bande assolute ancorate al settore. Uccide il "miglior declinante".
- **P1 — Coverage nel ranking.** Moltiplicare lo score per un fattore di coverage mite (~0.85→1.0 su coverage 0.5→1.0), oppure ordinare sulla tupla (score, coverage), così i target a evidenza sottile non possono arrivare in cima.
- **P1 — Ri-formare `turnover_proximity`**: reward a plateau su [min,max], rampa sotto min, taper sopra max; onorare mandati **solo-min** e **solo-max**. Invertire per il tuck-in (più piccola = meglio).
- **P1 — Successione: aggiungere `heir_present`** (cognome+età di un socio di minoranza) come *retrocessione*, e tagliare il peso-successione di `company_age` (ridondante con l'età titolare). Aggiungere **trend headcount** (serie dipendenti) come segnale Economico per la tesi di crescita.
- **P2 — `geographic_fit`** da `province` per consolidamento/tuck-in. Rendere il *set* di segnali thesis-specifico (tuck-in premia la piccola taglia; crescita prende il trend headcount), non solo i pesi.
- **P2 — Trend robusto**: fit log-lineare sull'*intera* serie (non gli estremi a 3 punti che `measureTurnoverTrend` usa) + caveat di volatilità.

**Verdetto architetturale.** "10 segnali fissi + pesi di famiglia guidati dalla tesi" è l'architettura *giusta* per lo screening — tenerla opinionata e preset-driven, non dare agli analisti un pannello pesi libero (opacità + overfitting senza ground truth). Ma evolverla da thesis-weighted a **set di segnali thesis-specifici**, aggiungere lo strato assoluto sotto i percentili, e rendere la coverage un input di primo ordine al rank — non solo un'etichetta.

**Domanda affilata per il product owner.** *Questo score è un ordinamento di triage dentro-il-run che l'analista legge una volta e scarta, o un numero di "qualità" persistito, confrontato tra run e mostrato a un comitato investimenti?* Se il secondo, i segnali percentile e l'ordinamento coverage-cieco devono sparire prima che la UI esca — un rank che cambia in silenzio quando allarghi la ricerca, e lascia una società a 2 segnali battere una a 9, farà distrust lo strumento al primo che se ne accorge.

### 3.2 Lente B — Rigore metodologico/statistico (profilo: quant valuation / model-validation credit-scoring)

**Verdetto.** La matematica è **internamente consistente ma non abbastanza solida per ordinarci sopra così com'è**. Due difetti strutturali — la ri-normalizzazione piena del peso disaccoppiata dallo score, e il percentile su un pool di sopravvissuti mobile — lasciano che un'azienda rada e non verificabile batta una pienamente documentata, e fanno sì che lo stesso "72" significhi cose diverse run-to-run. Funziona come filtro-distress grossolano × gradiente di fit; è inaffidabile come rank fine 0–100. I fix sono economici e localizzati.

**Difetti confermati (ordinati).**

- **D1 — La ri-normalizzazione premia la scarsità; la penalità è parcheggiata in un'etichetta non-scoring** (`ma_scoring.go:130-134, 157-161`). `effectiveWeight = w_i/Σw_active` mette ogni azienda su un blend [0,1] pieno per quanto pochi dati abbia, e la coverage setta solo l'etichetta `alta/media/bassa`, che **non** entra nello score né nel sort (`:167`, sort `:187-192`). Esempio, tesi successione: un'azienda con **solo** i tre segnali Fit presenti — ateco=core(1.0), turnover_proximity=ideale(1.0), keyword=coerente(1.0), fatturato recente & PN assente → viability=1.0, nessun socio → thesisFit=1.0 → **score = 1.0×1.0×1.0×100 = 100**, coverage 30/100="bassa". Un gemello pienamente documentato coi percentili trend/produttività alla mediana 0.5 blenda a **0.933 → 93**. Il fantasma a 3 segnali è **#1**. Viability prende il fatturato mancante (balanceFactor 0.3) ma non la proprietà/età/forma/equity mancanti — quindi ogni azienda con un fatturato recente e nient'altro galleggia in cima.
- **D2 — I percentili sono pool-relativi e non-stazionari** (`ma_scoring.go:107-108`, `percentileRank :197-213`). trend & produttività rankano solo nel pool dei sopravvissuti del run, quindi fondamentali identici segnano diversamente tra run. Un'azienda a +10%/anno CAGR segna ~0.9 in un pool pieno di declinanti e ~0.2 in uno pieno di crescitori; al peso trend della tesi crescita (~18/100) è uno **swing di ~12.6 punti su finanziari invariati**. Piccolo-n è peggio: `n≤1→0.5` significa che l'unica azienda con ≥2 anni di bilancio prende un trend *neutro* a prescindere da un CAGR +50% o −50%; a n=3 il miglior crescitore si ferma a 0.833, non 1.0. I rank si invertono anche quando il gate stringe/allarga il pool. Un "72" **non è comparabile tra sessioni o tesi**.
- **D3 — Il doppio conteggio gonfia la leva equity/status oltre il budget dichiarato** (viability `:456-467` vs equity_solidity `:774-803`). PN negativo colpisce il blend (equity_solidity→0.0, rimuove i suoi 6.67 punti) **e** il moltiplicatore viability (equityFactor 0.3). Gemello solido: blend 0.933×1.0=93. Gemello PN-negativo: blend 0.867×**0.3**=**26** — un calo di 67 punti da un fatto espresso due volte. La sensibilità reale dell'equity è ~3× il suo peso nominale del 6.67%, quindi il budget pesi pubblicato descrive male ciò che guida il rank. Stesso pattern per lo stato attività (viability.statusFactor 0.2 **e** filtro di ricerca a monte) — se il pool è pre-filtrato ad ATTIVA, statusFactor è peso morto; se no, è uno swing 5× contato in due posti.
- **D4 — Compressione + bande quantizzate → ties spezzati alfabeticamente** (`ma_scoring.go:188-189`). Molti sotto-score emettono 3–5 livelli discreti (ateco 4, keyword 3, ownership 3, legal 4, equity 5). I sopravvissuti hanno già passato il gate settore, quindi la famiglia Aderenza (~30 pt) è quasi costante (ateco≈core per tutti), lasciando la discriminazione a ownership×età×forma ≈ 3×4×4 bucket — e quando i finanziari sono radi (entrambi i percentili mancanti) i tie-breaker continui svaniscono. Attendersi ties esatti multipli tra 20–60 sopravvissuti, risolti per **nome** — cioè arbitrariamente. I segnali percentile sono i tie-breaker principali eppure i meno affidabili (D2).
- **D5 — Lo swing più economico = presenza del fatturato** (viability `:439-440`). `balanceFactor = 0.3 se turnover==nil altrimenti 1.0` — un singolo valore di fatturato riportato ribalta l'**intero score ×3.3**. E il CAGR usa solo gli estremi (`measureTurnoverTrend :752-762`): un anno finale sbagliato (es. [100,120,5], 5 refuso) dà −78%/anno → percentile di fondo; un refuso nel primo anno lo manda in cima. Nessuno smoothing, nessun outlier rejection.

**Concerns più morbidi.**
- Il clamp `score>100` (`:168-170`) è codice morto: blend≤1 × due fattori ≤1 ⇒ ≤100.
- I raw dei percentili sono raccolti su target che poi vanno `fuori_criterio` (knockout/sectorOut contribuiscono comunque alla distribuzione, pass-1 `:76-78`), contaminando sottilmente i percentili dei sopravvissuti.
- I tagli moltiplicativi si compongono (viability 0.6 × holding 0.4 = 0.24, un taglio del 76%), comprimendo il fondo del range; va bene per la retrocessione, ma il fondo porta poca informazione d'ordine.

**Fix concreti.**
- **P0 — Floor al denominatore della ri-normalizzazione.** `blended = Σ w_i·score_i / max(Σw_active, κ·Σw_intended)`, κ≈0.6. Il peso mancante sotto il floor resta come contributo-0. Il fantasma D1: 30/max(30,60)=0.5 → **score 50, non 100**. Minimo, mantiene [0,1], rimuove direttamente il vantaggio-scarsità. (Alternativa: moltiplicativo `score *= sqrt(coverage)` → 100·√0.3≈55.)
- **P1 — Rendere i percentili assoluti.** Sostituire `percentileRank` con bande ad ancore fisse sul CAGR (≥15%→1.0, 5–15→0.7, 0–5→0.5, −10–0→0.3, <−10→0.1) e su €/dipendente. Uccide D2 (non-stazionarietà, piccolo-n, n≤1→0.5, incomparabilità cross-run) in una mossa; tenere il percentile solo come overlay di display.
- **P1 — De-duplicare equity & status.** Lasciare a viability il *gate di distress* (PN neg, cessata, obsoleto) e al segnale additivo il *gradiente continuo*, OPPURE eliminarne un lato; come minimo pubblicare la sensibilità-effettiva, non il peso nominale. Verificare se il pool è pre-ATTIVA e, se sì, eliminare statusFactor.
- **P2 — Tie-break sensato.** Sostituire l'alfabetico con coverage↓, poi viability↓, poi fatturato↓; scorare internamente su 0–1000 prima dell'int-round per tagliare i ties artificiali.
- **P2 — Trend robusto.** CAGR da regressione log-lineare o mediana YoY su ≥3 anni; clampare gli outlier di fatturato per-anno; graduare balanceFactor sulla recenza invece che su un test binario di null.

**Cosa misurare.**
1. **Stabilità del rank:** bootstrap-resample del pool e ricalcolo; riportare Spearman/Kendall τ del ranking e il drift di score per aziende identiche (target τ>0.9). Oggi D2 lo fa fallire.
2. **Bias di coverage:** regredire score su coverage su un run reale; dopo P0 il vantaggio a bassa-coverage deve sparire (residuo ≈0).
3. **Spread:** entropia / n° effettivo di score distinti e % di coppie entro ±2 pt tra i top-20 — quantifica D4.
4. **Ground truth:** raccogli già rating a stelle dell'analista ("veryshort preferiti", 1–3★). Calcolare AUC / rank-correlation di score vs quelle stelle e precision@k dei top-N — l'unico vero test di calibrazione; lo score è oggi un'**euristica non validata**.
5. **Ablation:** togliere ogni moltiplicatore/segnale, misurare il cambio di overlap top-N — atteso che viability, non il budget pesi, domini il ranking.

### 3.3 Lente C — Efficacia nel funnel (profilo: head of deal origination / corporate development)

**Verdetto.** Come motore di ranking lo score è sopra-media — il breakdown evidence/adjustment è genuinamente buon decision-support, e la pesatura per tesi è l'idea giusta. Ma come **strumento di screening** fallisce il lavoro di origination su due conti che contano più della precisione d'ordine: **lascia cadere in silenzio i criteri di testa dell'analista** (tutti i vincoli free-form) e **confonde "non documentato" con "distressed"**, seppellendo esattamente i target dal-fondatore-assopito che la tesi di successione esiste per trovare. Oggi è un buon ordinatore di ciò che ha scelto di misurare, non un fedele screen di ciò che l'analista ha chiesto.

**Dove fallisce l'analista (ordinato).**
1. **I vincoli free-form sono invisibili al punto di decisione.** "≥30% ricavi ricorrenti" e "ISO 27001" — i due criteri che *definiscono* la tesi cybersecurity — non toccano mai lo score e appaiono solo come stringhe piatte su `strategy.MissingCriteria` ("Vincolo unsupported non applicato: …", `ma_service.go:2088`). *Conseguenza:* l'analista legge una lista 0–100 dall'aria autorevole che ha ignorato metà del suo brief, senza alcun segnale per-riga che lo abbia fatto. *Fix:* promuovere i vincoli unsupported a **chip di verifica-manuale di prima classe su ogni riga** così viaggiano nel deep-dive, più un banner persistente sui risultati "Il ranking NON ha considerato: …". Non devono mai vivere solo sull'oggetto strategia.
2. **Viability punisce la scarsità-dato, non solo il distress.** In `computeViability` (`ma_scoring.go:425`) `turnover == nil` → `balanceFactor 0.3` — un taglio secco del 70% per un'azienda che semplicemente non ha depositato, che in Italia è *fisiologico* per piccole SRL e *caratteristico* dei target di successione. Combinati sotto 0.25 → **knockout nascosto** a `fuori_criterio`. *Conseguenza:* i lead di successione di più alto valore (titolare anziano, depositi sottili) vengono retrocessi o spariscono — il rischio falso-negativo dominante, ed è *dentro* lo scoring, non nel gating a monte. *Fix:* separare "nessun dato" (neutro, già catturato dall'etichetta coverage) da "distress" (retrocedi). Tenere solo *cessata/dormiente* come veri knockout.
3. **Il knockout viability è thesis-blind.** PN negativo → `equityFactor 0.3`, e abbastanza distress nasconde del tutto l'azienda. Ma per una tesi consolidamento/turnaround, il PN negativo è l'*opportunità*, non lo squalificante. *Fix:* convertire il knockout viability in **retrocessione visibile + flag** (rank basso, ancora visto); riservare il vero nascondere alle entità legalmente-morte.
4. **Doppio gating di settore con una regola più grezza.** Il gate semantico a monte ha già etichettato keep/forse; `inSectorPerimeter` (`ma_scoring.go:580`) poi ri-gata sulla divisione ATECO a 2 cifre e nasconde qualunque cosa fuori-divisione. La cybersecurity legittimamente spazia su divisioni 62/63/80 ed è spesso mis-codificata. *Conseguenza:* un target semanticamente-confermato con ATECO fuori-divisione viene nascosto. *Fix:* se il gate semantico ha detto keep/forse, retrocedi-non-nascondere sul mismatch di divisione ATECO.
5. **Il sort ignora la confidence — high-score a basso-dato galleggiano in cima.** Il ranking è puro score (`ma_scoring.go:187`). Un 92 costruito su 2 segnali ri-normalizzati (confidence "bassa") batte un 85 su dato pieno, quindi i primi — più costosi — slot di deep-dive vanno ai target meno sostanziati. *Fix:* ordinamento confidence-aware (sotto).

**Fix di fedeltà all'intento (prioritizzati).**
1. **Mostrare in cima il registro "valutato / filtrato / ignorato".** Tre colonne: cosa lo score ha graduato (10 segnali), cosa è stato filtro duro (province, taglia, forme, ATECO), cosa è stato lasciato cadere (vincoli free-form, età-titolare-max, fatturato a un estremo). La fiducia viene dall'analista che vede il confine di ciò che il numero significa.
2. **Rendere la tesi inferita un override di prima classe a un click.** La tesi muove ~50% del budget pesi e gata del tutto il segnale succession-owner; un'inferenza LLM sbagliata cancella in silenzio l'età-titolare da un brief "fondatore vicino alla pensione". Deve essere visibile ed editabile sui risultati, con un indicatore "età-titolare graduata: sì/no" legato ad essa.
3. **Chip di verifica-manuale per-riga** per i vincoli unsupported (come al fallimento #1).
4. **Riconsiderare il peso keyword (5).** Quasi decorativo e token-matching grezzo; dove l'ATECO è un proxy debole, dare in pasto la confidence keep/forse del gate semantico a monte come segnale di fedeltà-settore invece del token match.

**Raccomandazione era-UI: IBRIDO (non pesi liberi).** Tenere il **catalogo fisso come motore**, ma esporre tre controlli: (a) **tesi** come selettore impostabile dall'analista, (b) **3–4 preset di enfasi** ("taglia / successione / finanziari / purezza-settore") che spingono i pesi *di famiglia* — non i numeri per-segnale grezzi, e (c) **tiering + sort confidence-aware**: raggruppare i 20–80 sopravvissuti in *Priorità alta / media / da verificare* (low-confidence-high-score finisce in "da verificare", mai in cima all'A-list), score come sort dentro-il-tier, con re-sort per thesis-fit o data-confidence. **Versione minima:** override tesi + registro valutato/filtrato/ignorato + chip vincoli-unsupported + tiering confidence-aware. Colpisce la soglia di adozione — gli analisti adottano uno screen di cui si fidano che abbia fatto ciò che hanno chiesto e possono correggere l'unica cosa (la tesi) che la macchina ha sbagliato; lo abbandonano la prima volta che un ignoto a dati-sottili è in cima o il loro criterio di testa è ignorato in silenzio.

**Una cosa da NON costruire.** **Slider numerici di peso per-segnale per gli analisti.** `normalizeMASignalWeights` già supporta 1–40 per segnale, quindi è tentante esporlo. Non farlo: ri-normalizzazione + segnali percentile + viability/thesisFit moltiplicativi rendono slider→esito non-intuitivo, i ranking diventano non-comparabili tra ricerche, e gli analisti sovra-tuneranno per forzare nomi noti in cima — distruggendo il valore-cardine dello screen come *rete imparziale*. L'LLM che spinge `SignalWeights` è macchineria sufficiente; tenere il controllo umano all'altitudine tesi/enfasi.

---

## 4. Consolidamento

### 4.1 Consenso di fondo

- **L'architettura è giusta, l'ordinamento no.** Tutti e tre: catalogo fisso + pesi per tesi, deterministico e spiegabile → **tenere**. Ma non affidabile per *ordinare* così com'è.
- **Verdetto UI unanime:** niente slider per-segnale agli analisti. Controllo umano all'altitudine **tesi + preset di enfasi di famiglia**.

### 4.2 Difetti convergenti (accordo × gravità)

| # | Difetto | Chi | Fatto verificato |
|---|---|---|---|
| 1 | **Coverage non entra né in score né in sort** → dati radi battono documentati | A, B, C | Fantasma a 3 segnali Fit (tutti 1.0) → **100**; gemello documentato (percentili 0.5) → **93**. Fantasma #1. |
| 2 | **Percentile sul pool dei sopravvissuti** → non-stazionario, "miglior declinante" ~1.0, n≤1→0.5, "72" non comparabile | A, B | allargare la ricerca rimescola i sotto-score di tutti |
| 3 | **Viability confonde "assente"/"distressed", è thesis-blind, doppio-conta** equity/status | A, B, C | `turnover==nil→0.3` punisce il non-deposito fisiologico = **target di successione**; equity contata 2× (segnale→0 e viability); status 2× (viability e filtro) |
| 4 | **Vincoli free-form ignorati in silenzio** | C | stringhe piatte su `MissingCriteria`, mai per-riga |
| 5 | **turnover_proximity mal-formato; min-only sparisce** | A | `maTurnoverIdeal=0` se manca "around" o un estremo |
| 6 | **keyword_match peso 5** quasi decorativo | C | usare la confidence del gate semantico |
| 7 | **Zero proxy redditività** (CAGR del PN derivabile); geografia inutilizzata | A | net-worth compounding ≈ utile auto-finanziato |

### 4.3 L'insight chiave: **un solo archetipo**

I difetti #1, #3 e la preoccupazione centrale di C **puntano tutti alla stessa azienda**: la **SRL piccola, padronale, con bilanci sottili e vecchio proprietario**. È l'azienda che #1 fa arrivare a 100 su 3 segnali, che #3 seppellisce come "distressed", e che C dice essere *proprio il target di successione che cerchiamo*. Non sono difetti scollegati: sono una sola frattura vista da tre lati.

**Trappola:** la correzione "ovvia" di B (moltiplica lo score per la coverage) **punisce esattamente quell'archetipo**. La risoluzione corretta della tensione è: la coverage deve governare **confidenza/ordinamento (tiering)**, non schiacciare lo score.

### 4.4 Le tensioni da arbitrare

1. **Coverage: correggere il numero o la presentazione?** B vuole toccare lo *score* (floor denominatore / ×√coverage). C preferisce lasciare lo score e correggere *ordine + tiering*. → Per l'archetipo, la presentazione (tiering) è più sicura del numero.
2. **Distress: nascondere o mostrare-declassato?** Oggi knockout nascosto (coerente con money-safety). C vuole demote+flag (recall). → richiede thesis-awareness e una decisione sulla visibilità di default. **Vedi §7** (il nodo scarti va oltre "mostrare in fondo").
3. **Nuovo segnale `equity_compounding`** (solo A): promettente ma dato rumoroso — va prima verificata la disponibilità nei payload reali.

---

## 5. La domanda che sblocca lo scope (gate)

> **Questo "72" è un ordinamento usa-e-getta *dentro* il run, o un numero di "qualità" persistito, confrontato tra run e mostrato a un comitato investimenti?**

Convergente su tutti e tre. Se persistito/IC → percentili assoluti (#2) + coverage nel rank (#1) sono **P0 prima della UI**.

**Posizione del team (Salvatore/Claude):** trattarlo come **persistito/IC** — è già salvato, esportato in CSV e ci costruiamo sopra i preferiti; nel momento in cui un numero è in una UI, l'analista lo legge come qualità e lo confronta tra run. Progettare "tanto è solo triage" è come si perde fiducia.

---

## 6. Vincolo di stadio: non abbiamo dati rappresentativi

**Stato reale:** app in sviluppo, **una sessione, un solo analista**. Gli esempi non sono rappresentativi. → **Non c'è ground truth** di valore statistico.

**Implicazioni (cambia la strategia):**

- **Il "misura prima di tagliare" salta.** Validare su dati non rappresentativi sarebbe peggio che non validare. Il piano AUC/precision@k contro le stelle è **rimandato** finché non avremo esiti reali.
- **A questo volume lo score non deve essere *accurato*, deve essere *trasparente e stabile*.** Con un analista che legge comunque tutte le ~30 righe, l'umano *è* il validatore nel loop. I difetti che contano sono quelli che **ingannano o nascondono**, non quelli sulla precisione del ranking. Un peso leggermente sbagliato è irrilevante se una persona rivede tutto; una riga *nascosta* o un criterio *ignorato in silenzio* no.
- **Questo ribalta le priorità** verso la lente C (trasparenza, anti-occultamento) e **le allontana** da B (precisione) e A (segnali più ricchi).
- **Preferire fix giusti *per principio*, non *per calibrazione*.** Robustezza (stesso input → stesso rank; "assente" ≠ "distressed") e onestà (mostrare cosa è stato ignorato; non far overclaim al numero) sono difendibili a priori. Ri-pesatura e nuovi segnali no — servono dati.
- **Instrumentare per uscirne.** Il collo di bottiglia non è il codice dello score, è che non generiamo ancora esiti. Ogni decisione dell'analista — stella, motivo di scarto, "contattato / buon lead" — va loggata pulita, così tra qualche mese *avremo* la ground truth. Leggero, ma da mettere ora.

---

## 7. Il nodo "scarti": tre destinazioni, non due

**Vincolo dell'analista:** finora gli scarti sono stati nascosti perché finivano comunque in fondo alla lista. **Allungare la lista tenendoli in fondo non migliora nulla** (né la UX): o li valorizziamo, o fanno solo rumore.

**Il test corretto:** *c'è un'azione specifica che rimetterebbe questo elemento in gioco?* Se sì → valorizzalo con quell'azione. Se no → è rumore, sopprimilo (ma con onestà: un conteggio, non le righe). Applicando il test → **tre destinazioni**:

**1. Lista principale — l'archetipo che stavamo sbagliando a retrocedere.**
La SRL padronale col bilancio sottile *non è distressed*, è normale. Oggi `turnover==nil → 0.3` la butta giù o la esclude. Questa va **dentro** la lista — è il target. È il fix di logica #3 ("assente ≠ distressed"), non una questione di posizione.

**2. Cassetto azionabile — dove sta il valore vero (poche righe, segnale alto).**
Solo gli scarti con un'**azione a un click** che li recupera:
- **Fuori-ATECO ma confermate dal gate semantico** → "N aziende keep/forse fuori dalle tue divisioni ATECO — probabilmente mis-codificate → rivedi/override". Il gate semantico ha già fatto il lavoro difficile; l'ATECO è solo un codice sbagliato.
- **Dominio irrisolto (manual_review)** → "associa dominio → processa" (leva già prevista nel codice).
- **PN negativo *solo* sotto tesi turnaround/consolidamento** → candidati, non reject (thesis-aware).

**3. Soppressione con conteggio — il rumore vero.**
- **Cessate/dormienti** (niente azione, sono morte).
- **Escluse dalle regole dure che l'analista stesso ha messo** (ricavo/dip, max soci): esclusione *corretta* → mostri "12 escluse dalle tue regole", zero righe.

**Fattibilità:** il *perché* di ogni esclusione è **già calcolato** (`knockout` viability / `sectorOut` / `postFilterOut` / `manual_review`), quindi instradare nei tre destini è routing su segnali esistenti, non nuova analisi.

---

## 8. Raccomandazioni prioritizzate

Filtrate dal vincolo di stadio (§6): **prima trasparenza + robustezza + cattura esiti; rimandare calibrazione + segnali ricchi.**

### Fare ora — giusto per principio, aiuta l'unico analista oggi
| Intervento | Chiude | Chi | Buildable ora? |
|---|---|---|---|
| **Smetti di nascondere:** routing scarti nelle 3 destinazioni (§7); knockout viability/settore → retrocessione visibile o cassetto azionabile, thesis-aware | #3, C-#3/#4 | C, A | Sì (reason già calcolati) |
| **Mostra cosa lo score ha ignorato:** chip per-riga vincoli unsupported + registro "valutato/filtrato/ignorato" | #4, C-#1 | C | Sì (serve alla UI comunque) |
| **Tiering per confidence** nell'ordinamento (non nello score): low-confidence-high-score → "da verificare" | #1 (come ordine) | C, A | Sì |
| **Viability: separa "assente" da "distressed"** (fix di logica) | #3 | C, A, B | Sì |

### Fare quando si tocca quel codice — robustezza, quasi gratis in corsa
| Intervento | Chiude | Chi |
|---|---|---|
| Percentile → bande assolute ancorate | #2 | A, B |
| De-dup equity/status tra viability e segnale additivo | D3 | B, A |
| turnover_proximity: onora min-only/max-only + reshape plateau | #5 | A |

### Rimandare — serve dato che non abbiamo
| Intervento | Perché rimandare |
|---|---|
| Ri-pesatura famiglie / soglie bande | calibrazione senza ground truth = fede |
| `equity_compounding`, `heir_present`, `geographic_fit`, trend headcount | miglioramenti, non rotture di fiducia; verificare prima disponibilità dato |
| Segnali thesis-specifici (set diversi per tesi) | build grosso su base non misurata |
| Preset di enfasi di famiglia (era-UI) | dopo che la versione base è validata sul campo |

### Trasversale — da mettere ora
- **Cattura degli esiti**: stella, motivo di scarto, "contattato/buon lead" loggati puliti → futura ground truth.

---

## 9. Opinione di sintesi

**Niente calibrazione (non possiamo) → sì a trasparenza + anti-occultamento + cattura esiti.** In una riga: *misura (quando avrai i dati) → aggiusta i percentili → tira la coverage nel tiering (non nello score) → cura l'archetipo padronale → valorizza gli scarti solo se hanno un'azione.*

Il primo passo concreto e a basso rischio è il **routing degli scarti nelle 3 destinazioni** (§7) + il **registro degli ignorati** (§8), che serve comunque alla UI e attacca direttamente i due difetti a più alto rischio-recall (#3, #4).

---

## 10. Decisioni aperte (da valutare)

1. **Gate (§5):** confermi "persistito/IC" come stella polare del design? (Determina se #1/#2 sono P0.)
2. **Coverage (§4.4-1):** correggere il *numero* (floor/√coverage) o solo *ordine/tiering*? (Racc.: tiering, per l'archetipo.)
3. **Distress (§4.4-2, §7):** ok alla tripartizione scarti? Visibilità di default del cassetto azionabile?
4. **Scarti (§7):** confermi le 3 destinazioni e la mappatura reason→destino?
5. **equity_compounding (§4.4-3):** vuoi che verifichi prima la disponibilità del dato PN nei payload reali della sessione?
6. **Cattura esiti (§6, §8):** priorità? È il prerequisito per ogni calibrazione futura.

---

## Appendice — Riferimenti codice

- `ma_scoring.go`: `scoreMATargetsV2` (`:42`), `percentileRank` (`:197`), `maConfidenceLabel` (`:215`), `maPostFilterEvidence` (`:238`), `computeViability` (`:425`), `computeThesisFit` (`:478`), `inSectorPerimeter` (`:580`), `measure*` (`:602`+), formula finale (`:167`), sort (`:187`).
- `ma_catalog.go`: `maSignalCatalog` (`:74`), `thesisFamilyWeights` (`:106`), `maSignalNominalWeights` (`:124`), `maTurnoverIdeal` (`:256`), `extractFinancials` (`:301`).
- `ma_flags.go`: `computeMAFlags` (`:21`), `dominantOwnerAge` (`:98`).
- `ma_service.go`: `resolveIntent` (`:1856`), `maIntentUnsupportedCriteria` (`:2088`), `runExecution` (`:4601`), `baseMASearchParams` (`:4993`).
- `ma_gated_search_job.go`: `enrichAndScoreSurvivors` (`:407`), `gatedEnrichPlan` (`:474`).

# Binocolo — contro-analisi FDD della proposta prompt v4 (`PROMPT-FDD.md`)

> Contro-analisi da Partner Senior FDD, 2026-07-03. Ogni verdetto è verificato
> contro il codice reale (parser del brief, harness eval, seed IRL, engine del
> bridge, rendering UI), non solo contro il testo della proposta.

## Verdetto complessivo

**ACCETTATA CON CONDIZIONI.** L'impianto è corretto: prompt-only, GPT-5.5
invariato, gerarchia analitica FDD sensata, rollout riusando l'harness
esistente. Nessuna sezione va rifiutata in blocco. Tre punti però sono
**bloccanti** prima della migration, perché la proposta — scritta guardando il
prompt — non ha verificato tre interazioni col codice che la circonda:

1. il nuovo `valuationRationale` a 6 contenuti obbligatori collide col
   troncamento hard a 600 rune del parser;
2. le metriche di accettazione proposte non proteggono da regressioni: due
   rubriche dell'harness sono in tensione diretta con la spinta alla brevità
   della v4 e potrebbero peggiorare senza far scattare nessun criterio;
3. il rischio dominante della v4 non è misurato da nessuna metrica: la postura
   "FDD senior" induce affermazioni qualitative (EBITDA quality, sostenibilità)
   che i dati CEE pubblici non possono supportare — l'harness intercetta solo
   allucinazioni **numeriche**, non l'overreach narrativo.

## Evidenze verificate

- Prompt v3 live: `deploy/migrations/097_anisetta_mrsmith_binocolo_thesis_reading_prompt.sql:68-103`
- Parser e troncamenti: `parseMADeepBrief` — `backend/internal/binocolo/ma_deep_worker.go:285-331`
  (verdict 600, businessProfile 800, financialReading 1000, **valuationRationale 600**,
  claim/ddQuestion 300 rune; cap liste: strengths 6, redFlags 8, ddQuestions 8)
- Harness eval: `backend/internal/binocolo/ma_deep_llm_eval.go:743-870`
  (rubriche: json_schema 10, rag_coherence 10, valuation_consistency 25,
  no_invented_numbers 20, caveat_coverage 15, dd_questions 10, clarity 10)
- Tolleranza numerica eval: `closeEvalNumber` — `ma_deep_llm_eval.go:1248-1262`
  (±3% relativo, minimo ±1 assoluto; ±5k sopra 100k, ±50k sopra 1M)
- Seed IRL: `backend/internal/binocolo/ma_irl.go:62-137` (4 fonti; le
  `ddQuestion` delle red flag LLM **non** sono una fonte)
- Bridge EV→equity: `MADeepBridge` — `backend/internal/binocolo/ma_types.go:1306-1343`;
  nota finanziamenti soci — `ma_deep_engine.go:467-473`
- Tier "contorno" sui KPI: `backend/internal/binocolo/ma_types.go:1265-1268`
- Input LLM del brief: `buildMADeepBriefLLM` — `ma_deep_worker.go:225-245`
  (scorecard + valuation con bridge + company curata; "No IT-full call";
  max_tokens default 5000 inclusi i reasoning token)
- Rendering UI red flag: `apps/binocolo/src/pages/CompanyDossierPage.tsx:293-296`
  (severity mappata, `category` resa come chip **raw**)

## Verdetti per sezione

| Sezione proposta | Verdetto | Sintesi |
|---|---|---|
| §2/§9 — v4 prompt-only, GPT-5.5 invariato | **ACCETTO** | Baseline eval solida; cambiare modello azzererebbe la comparabilità |
| §5.1 — postura FDD buy-side | **ACCETTO con paletto** | Il brief resta NEUTRO: niente go/no-go assertivo, che è thesis-scoped |
| §5.2 — gerarchia analitica | **ACCETTO con modifica** | "Qualità EBITDA" da soli CEE pubblici non è accertabile: riformulare |
| §5.3 — arrotondamenti, no k/M/mln | **ACCETTO** | Verificato: la tolleranza eval assorbe gli arrotondamenti, zero falsi positivi |
| §5.4 — strengths max 5, anti-ROE/ROI | **ACCETTO** | `tier: contorno` esiste davvero nel payload; regola ancorata a un campo reale |
| §5.5 — red flags: materialità, categorie | **ACCETTO con 4 modifiche** | Incoerenza interna del doc, chip UI raw, go/no-go, tetto lunghezza claim |
| §5.6 — valuationRationale a 6 contenuti | **ACCETTO con modifica — BLOCCANTE** | Collisione col troncamento a 600 rune |
| §5.7 — DD questions IRL-style | **ACCETTO** | Con gap architetturale da tracciare (red flag → IRL) |
| §7.1 — migration update in-place | **ACCETTO con paletto** | Solo `llm_prompt`, mai `llm_model`; verificare la riga registry live |
| §7.2 — validazione | **ACCETTO con integrazioni — BLOCCANTE** | Le metriche proposte non catturano le regressioni possibili |
| §7.3 — rollout cache | **ACCETTO con 2 note** | "No ricompra IT-full" già garantito dal codice; effetto quasi-duplicati sull'IRL |
| §8 — criteri qualitativi | **ACCETTO con aggiunta** | Un dato materiale assente è un finding, non un silenzio |

## Analisi di dettaglio

### A. Mantenere GPT-5.5 e lavorare solo sul prompt — ACCETTO

La decisione è giusta e ben motivata: con 100/100 JSON validi, 0% RAG flip, 0%
numeri sospetti e ~99,7 di media, il collo di bottiglia non è il modello. Il
punteggio quasi-perfetto dice anche un'altra cosa che la proposta non
esplicita: **l'harness è saturo sulla v3** e non discriminerà v3 da v4 sul
totale. La differenza vera si giocherà sulla review manuale — per questo le
condizioni al punto §7.2 sotto sono bloccanti, non pignolerie.

### B. Postura e gerarchia (§5.1–5.2) — ACCETTO con una riformulazione

La gerarchia (EBITDA → debt-like → working capital → perimetro → governance →
valuation) è l'ordine giusto per un memo pre-FDD buy-side. Due correzioni:

1. **"Qualità e sostenibilità dell'EBITDA" promette più di quanto i dati
   permettano.** L'input è un bilancio CEE depositato: niente normalizzazioni,
   niente one-off visibili, niente dettaglio ricavi ricorrenti. Il modello, se
   gli si chiede "qualità dell'EBITDA", produrrà prosa che suona senior ma non
   è supportata — e l'harness non se ne accorge, perché misura solo i numeri
   inventati, non le tesi inventate. Riformulare in: *livello, tendenza e
   prudenzialità dell'EBITDA quando i dati lo consentono; la vera EBITDA
   quality si accerta in DD*. La scorecard espone già l'EBITDA prudenziale
   (`prudentialEbitda` in `MADeepValuation`): quello sì è commentabile.

2. **Il brief è e deve restare NEUTRO.** L'architettura v3 ha appena separato
   la lettura di tesi in `ma_thesis_reading` (mig 097): il go/no-go dipende
   dalla tesi del compratore e vive lì, con `blockingFlags`/`tolerableFlags`.
   Il claim delle red flag nel dossier neutro deve esprimere impatto
   **potenziale** su prezzo/struttura/closing accounts, mai un verdetto
   go/no-go. La v4 così com'è scritta reintroduce dalla finestra ciò che la
   mig 097 ha fatto uscire dalla porta.

3. La v3 aveva *"leggendo anche l'andamento sull'anno precedente quando
   disponibile"* e la v4 lo perde. I delta YoY vendor sono l'unica dimensione
   temporale disponibile nel payload e il trend è core FDD: la clausola va
   reintegrata.

### C. Arrotondamenti (§5.3) — ACCETTO, verificato contro l'harness

Il dubbio ovvio era: se il modello arrotonda `13.441200846309473%` a `13.4%`,
l'eval lo marca come numero fuori input? **No, verificato**:
`closeEvalNumber` tollera ±3% relativo con minimo ±1 assoluto (e ±5k/±50k
sugli importi grandi), quindi percentuali a 1 decimale, ratio a 2 decimali ed
euro interi restano tutti dentro tolleranza. Il ban su `k/M/mln` è inoltre
coerente con una debolezza nota del parser eval, che ha già dovuto essere
patchato per riconoscere la notazione "1.186 M" (`ma_deep_llm_eval.go:1052-1058`).

**Raccomandazione strutturale (follow-up, fuori scope prompt-only):** il fix
vero è arrotondare **a monte**, nel builder del payload backend. Chiedere al
modello di ignorare 15 decimali che gli abbiamo appena mandato è un cerotto:
arrotondare l'input riduce token, elimina il rumore per tutti i consumatori
(UI inclusa) e mantiene coerenti gli allowed numbers dell'eval, che derivano
dall'input stesso. Non bloccante: la regola prompt-side va bene come tattica.

### D. Strengths (§5.4) — ACCETTO

La regola anti-ROE/ROI è ancorata a un campo reale: il payload scorecard
espone `tier: "contorno"` (`ma_types.go:1265-1268`), quindi il modello ha
l'informazione per applicarla — non è un'istruzione al buio. Nota di
consapevolezza: il parser continua a troncare a 6 (`ma_deep_worker.go:310`),
quindi il vincolo a 5 è solo prompt-side. Accettabile, non serve toccare il
codice: il parser è una rete di sicurezza, non l'enforcement.

### E. Red flags (§5.5) — ACCETTO con quattro modifiche

1. **Incoerenza interna del documento**: §5.5 elenca 7 categorie, il template
   JSON in §6 ne ha 8 (compresa `struttura`). Tenere `struttura`: i brief v3
   cached restano in DB con quelle categorie e la UI li mostra fianco a fianco
   dei v4 — rimuoverla creerebbe due popolazioni di chip senza beneficio.
2. **Le categorie nuove arrivano crude in UI.** Il claim "non serve migrazione
   strutturale" è vero a livello API (`category` è string libera, lowercased
   al parse), ma la UI rende la categoria come chip raw
   (`CompanyDossierPage.tsx:296`): l'utente vedrà letteralmente
   `qualita_ebitda` e `debt_like`, con underscore e anglicismi. Per gli
   standard di copy B2B del progetto non è accettabile a regime. Decisione:
   accettare gli slug macchina ora e aggiungere una mappa slug→label nel
   frontend come follow-up dichiarato (2 righe, non rompe il prompt-only).
3. **Claim = anomalia + impatto in 300 rune.** Il parser tronca il claim a 300
   rune (`ma_deep_worker.go:328`): chiedere due contenuti nel claim senza
   tetto di lunghezza invita al troncamento a metà frase. Aggiungere al
   prompt: *claim massimo ~40 parole*.
4. **Go/no-go**: vedi punto B.2 — sostituire con "closing accounts" o
   "perimetro dell'operazione" nel testo del claim.

### F. valuationRationale (§5.6) — ACCETTO con modifica, **BLOCCANTE**

Questa è la collisione più seria. La v4 impone 6 contenuti obbligatori
(metodo+multiplo, EV vs equity, bridge principale, caveat multiplo/famiglia,
standalone vs controllate, impatto red flag sulla banda) in "2-4 frasi" — ma
`parseMADeepBrief` tronca il campo a **600 rune** (`ma_deep_worker.go:308` →
`cleanText`). Sei contenuti in 600 caratteri italiani ci stanno a malapena; il
troncamento a metà frase sul campo più delicato del brief è l'esito probabile
sui casi densi (bridge a 4 righe + caveat famiglia). Due rimedi, non
alternativi:

- **(i)** alzare il cap a 800 rune: una riga in `ma_deep_worker.go`, non tocca
  schema JSON, DB né UI — resta nello spirito della proposta;
- **(ii)** ridurre i contenuti obbligatori a 4: metodo+multiplo; EV vs equity
  col bridge principale; caveat perimetro/multiplo; "la banda non è un
  prezzo". Standalone-vs-controllate e impatto red flag diventano "se lo
  spazio lo consente" — l'impatto delle red flag ha comunque già casa nei
  claim delle flag stesse (punto E), duplicarlo qui è ridondanza.

In più, un rischio di double-counting narrativo: la riga `shareholderLoans`
del bridge è **già inclusa nella PFN** — il dato viaggia con la sua nota
(`ma_deep_engine.go:472`: *"inclusi nella PFN — riga negoziale"*), ma il
prompt v4 elenca "PFN, TFR, finanziamenti soci o fondi" come componenti del
bridge alla pari, invitando il modello a sommarli a parole. Aggiungere al
prompt: *rispetta le note delle righe del bridge; i finanziamenti soci sono
già dentro la PFN, la loro riga è informativa/negoziale*.

### G. DD questions (§5.7) — ACCETTO, con un gap architetturale da tracciare

La direzione IRL-style è giusta ed è coerente col filone F appena
implementato. Ma la proposta non ha guardato **dove finiscono** le domande: il
seed IRL (`ma_irl.go:92-116`) attinge a 4 fonti — flag deterministici,
`brief.ddQuestions`, lettura di tesi, template famiglia — e **mai** a
`redFlags[].ddQuestion`. La regola "non ripetere le domande delle red flag"
(già in v3, rafforzata in v4) produce quindi un buco: una richiesta
informativa che vive solo dentro una red flag non arriva mai nell'IRL, cioè
nell'artefatto che esce dal tool. Follow-up code-side raccomandato (non
bloccante per la v4): aggiungere le `ddQuestion` delle red flag come quinta
fonte del seed — è additivo per `source_ref`, quindi sicuro per la curatela
dell'analista.

Nota harness: la rubrica `dd_questions` premia domande con lunghezza media
≥45 caratteri (`ma_deep_llm_eval.go:828`). Le voci IRL ben formate ("aging
crediti e debiti per fasce di scaduto al 31/12") superano la soglia
naturalmente; domande telegrafiche no. Va monitorato nel breakdown (vedi I).

### H. Migration (§7.1) — ACCETTO con paletto

L'update in-place del prompt default è il pattern già usato dalla mig 097 e la
provenance storica non si perde: `ma_model_audit` salva la request completa di
ogni chiamata, quindi il testo effettivamente usato da ogni brief è
ricostruibile dall'audit anche se `llm_prompt` cambia sotto lo stesso ID.

Paletto: la migration deve toccare **solo** `mrsmith.llm_prompt` e non
avvicinarsi a `llm_model`. Il binding modello di questo scope è già cambiato
fuori banda nel DB in passato: prima dell'apply va verificata la riga registry
**live** (chiederla, non dedurla dal seed), e "modello invariato" va inteso
come "la migration non contiene INSERT/UPDATE su llm_model", non come
un'asserzione su cosa ci sia nel DB.

### I. Validazione (§7.2) — ACCETTO con integrazioni, **BLOCCANTE**

Le metriche proposte (JSON fail 0%, RAG flip 0%, hallucination 0%, riduzione
"prolisso") sono necessarie ma non sufficienti: sono tutte già a zero o
quasi-perfette sul baseline, quindi **nessuna di esse può peggiorare in modo
visibile senza che la v4 sia già un disastro**. Le regressioni plausibili
della v4 passano tutte sotto questa rete:

- la spinta alla brevità è in tensione diretta con `caveat_coverage` (15
  punti: premia la **citazione dei flag di qualità** nel testo —
  `scoreCaveatCoverage`, `ma_deep_llm_eval.go:949-971`): un brief più asciutto
  che smette di citare i caveat perde punti e qualità FDD reale;
- domande IRL troppo corte perdono 3 punti su `dd_questions` (soglia 45 char);
- l'overreach qualitativo (punto B.1) non è misurato da niente.

Integrazioni richieste ai criteri di accettazione:

1. punteggio medio ≥ baseline − 1 punto (baseline ~99,7);
2. nessuna regressione sui breakdown `caveat_coverage` e `dd_questions`
   (confronto per-rubrica, non solo sul totale);
3. review manuale come **confronto cieco v3 vs v4 fianco a fianco sugli stessi
   10 casi** — i criteri di §8 sono comparativi ("più breve ma più
   decision-useful") e non si giudicano guardando solo la v4;
4. la review manuale deve cercare esplicitamente affermazioni di EBITDA
   quality/sostenibilità non supportate dai dati: è l'unico controllo in grado
   di intercettarle.

### J. Rollout cache (§7.3) — ACCETTO con due note

1. "Non ricomprare IT-full" non è una cautela di processo: è **già garantito
   dal codice**. La rigenerazione passa da `buildMADeepBriefLLM`, condiviso
   col worker e documentato "No IT-full call" (`ma_deep_worker.go:221-224`).
2. Conseguenza non menzionata nella proposta: il re-seed IRL è additivo per
   hash del testo normalizzato (`maIRLSourceRef`, dichiaratamente *"sensibile
   alla riformulazione"*). Rigenerare i brief riscrive tutte le `ddQuestions`
   → un nuovo seed su card già seminate aggiunge fino a ~8 proposte
   quasi-duplicate per card. Accettabile (il seed è manuale, la curatela non
   viene toccata), ma va scritto nelle note di rollout per gli analisti, che
   altrimenti lo scopriranno come "l'IRL si è sporcata da sola".

### K. Criteri qualitativi (§8) — ACCETTO con un'aggiunta

Aggiungere due criteri:

- *non fa affermazioni di qualità/sostenibilità dell'EBITDA che i dati non
  mostrano* (specchio del punto B.1);
- *dichiara i dati materiali mancanti invece di tacerli*. La v4 dice solo "se
  un dato manca, non inventarlo e non colmare il vuoto" — ma in FDD un dato
  assente è un finding, non un silenzio: va nominato come limite dell'analisi
  e trasformato in richiesta informativa. Coerente con la regola
  assente≠distressed già ratificata nello scoring v3: l'assenza non è un
  malus, ma nemmeno invisibile.

## Condizioni bloccanti (prima della migration)

1. **valuationRationale vs cap 600 rune** (punto F): alzare il cap a 800 in
   `parseMADeepBrief` *e* ridurre i contenuti obbligatori a 4.
2. **Metriche di accettazione integrate** (punto I): score ≥ baseline−1,
   confronto per-rubrica su `caveat_coverage`/`dd_questions`, review cieca
   v3-vs-v4, caccia esplicita all'overreach qualitativo.
3. **Emendamenti al testo del prompt** (punti B, E, F): riformulazione EBITDA
   quality, reintegro lettura YoY, nota bridge finanziamenti soci, niente
   go/no-go nel dossier neutro, tetto ~40 parole sui claim.

## Raccomandazioni non bloccanti (follow-up da tracciare)

- Arrotondamento a monte nel builder del payload backend (punto C).
- Mappa slug→label per le categorie red flag nel frontend (punto E.2).
- Quinta fonte del seed IRL: `redFlags[].ddQuestion` (punto G).
- Allineare §5.5 del documento proposta al template JSON (8 categorie).

## Prompt emendato — proposta v4.1 FDD

Stesso JSON della v3; le differenze rispetto alla v4 proposta sono solo quelle
motivate sopra.

```text
Sei un analista FDD buy-side senior per il team M&A interno di un compratore strategico.
Ricevi in JSON: "scorecard" (KPI GIA' CALCOLATI con semaforo RAG), "valuation"
(valutazione di settore e bridge EV->equity gia' calcolati, con note per riga)
e "company" (fatti qualitativi dell'azienda: forma giuridica, date, ATECO,
gruppo e controllate, soci, cariche, sedi, dipendenti, gare pubbliche, presenza web).

Obiettivo:
produrre un brief preliminare FDD/M&A NEUTRO (nessuna tesi di acquisizione) per
decidere dove concentrare la due diligence. Non scrivere un elenco di KPI:
spiega cosa cambia per un potenziale acquirente.

Regole ferree:
- NON ricalcolare e NON inventare numeri: ogni dato quantitativo deve provenire da
  scorecard/valuation. Se un dato materiale manca, dichiaralo come limite
  dell'analisi e trasformalo in richiesta informativa; non colmare il vuoto e
  non trattare l'assenza come un problema in se'.
- I fatti qualitativi prendili solo da "company"; non dedurre fatti non presenti.
- Rispetta le note delle righe del bridge: i finanziamenti soci sono GIA' inclusi
  nella PFN (riga informativa/negoziale, non un'ulteriore deduzione).
- "rag" deve riflettere scorecard.overallRag quando presente.
- Non modificare la scala dei numeri e non usare k/M/mln.
- Arrotonda per leggibilita': percentuali max 1 decimale, multipli/ratio max 2 decimali,
  importi euro interi senza centesimi. Non riportare decimali lunghi dall'input.
- Tono asciutto, professionale, B2B; niente consigli legali, garanzie o raccomandazioni vincolanti.

Lettura FDD: dai priorita' a questi temi, quando i dati li coprono:
1. livello, tendenza e prudenzialita' dell'EBITDA (la vera qualita' dell'EBITDA
   si accerta in DD: non affermare sostenibilita' che i dati non mostrano);
2. PFN, TFR, finanziamenti soci, fondi rischi e altri potenziali debt-like items;
3. working capital, liquidita' e cash conversion;
4. perimetro standalone vs gruppo/controllate/partecipazioni;
5. key person, governance e parti correlate;
6. implicazione sulla valuation e sul rischio prezzo.
Leggi anche l'andamento sull'anno precedente quando disponibile.

Rispondi SOLO con JSON valido in questo formato:
{
  "verdict": "2-3 frasi: giudizio complessivo su solidita', attrattivita' e principale tema FDD per un acquirente",
  "rag": "green|amber|red",
  "businessProfile": "2-4 frasi: cosa fa l'azienda, settore/ATECO, dimensione, gruppo/controllate, governance rilevante, presenza territoriale; solo fatti presenti in company",
  "financialReading": "3-5 frasi: lettura FDD dei numeri, non elenco KPI. Evidenzia livello e tendenza dell'EBITDA, leva/PFN, liquidita'/working capital, crescita e perimetro quando rilevanti; chiudi con il 'so what' per un acquirente",
  "strengths": ["punti di forza concreti e deal-relevant, ognuno ancorato a un dato fornito (max 5)"],
  "redFlags": [
    {"severity": "warning|info", "category": "finanziario|qualita_ebitda|working_capital|debt_like|perimetro|governance|mercato|struttura", "claim": "anomalia osservata + potenziale impatto su prezzo, struttura dell'operazione o closing accounts (max 40 parole)", "ddQuestion": "richiesta informativa concreta conseguente"}
  ],
  "valuationRationale": "2-4 frasi: metodo e multiplo; EV vs equity col bridge principale (PFN, TFR, fondi); caveat su multiplo e perimetro. La banda non e' un prezzo certo",
  "ddQuestions": ["richieste informative prioritarie aggiuntive, stile information request list, non gia' coperte dalle redFlags (max 8)"]
}

Vincoli sulle liste:
- strengths: massimo 5; non usare ROE/ROI come strength principale se sono metriche di contorno,
  se il patrimonio netto e' basso/negativo o se la scala micro distorce l'indicatore.
- redFlags: massimo 7; ordinale per materialita' FDD, cioe' impatto potenziale su prezzo,
  struttura dell'operazione o closing accounts.
- ddQuestions: massimo 8; devono essere specifiche e azionabili. Preferisci richieste come:
  aging crediti/debiti, top clienti e contratti, EBITDA normalizzato, dettaglio PFN/scadenze,
  garanzie/covenants, parti correlate, lease/B.8, TFR/fondi rischi/debiti fiscali.
```

## Decisione

Procedere con la v4 **nella forma emendata v4.1**, GPT-5.5 invariato, dopo aver
sciolto le tre condizioni bloccanti. La proposta è sana nel merito FDD; le
condizioni non ne cambiano la direzione, ne impediscono i tre modi concreti in
cui potrebbe fallire in silenzio: un campo troncato, una regressione invisibile
alle metriche, una prosa senior non supportata dai dati.

---

## Addendum — verifica della revisione v4.1 (`PROMPT-FDD.md` aggiornato, 2026-07-03)

**APPROVATA.** La revisione recepie integralmente le tre condizioni bloccanti e
i paletti: contenuti `valuationRationale` ridotti a 4 obbligatori + 2 opzionali
col prerequisito cap 800 (§6.6/§8.1); metriche integrate con score ≥ baseline−1
e confronto per-rubrica (§8.2/§9.2); prompt emendato adottato verbatim (§7:
dossier neutro, EBITDA come livello/trend/prudenzialità, YoY reintegrato, nota
finanziamenti soci, claim ≤40 parole, niente go/no-go); review cieca v3-vs-v4.1
con una hunt list che estende sensatamente la mia (§8.3); migration solo su
`llm_prompt` con verifica del registry live (§9.1); nota IRL quasi-duplicati
nel rollout (§9.3); l'incoerenza sulle categorie è sanata (8 con `struttura`) e
i tre follow-up sono tracciati (§11).

Due riscontri empirici nuovi, misurati sui run baseline GPT-5.5 in
`artifacts/llm-evals/ma-deep-brief/20260703T142816Z-9a8f4a62-openai-gpt-5-5/`:

1. **Il claim di §8.1 è verificato — e sottostimato.** Su 100 run,
   `valuationRationale` ha media 529 rune, mediana 531, 38 run ≥550 e **14 run
   esattamente a 600 rune**: già troncati oggi, in v3, dal parser. Il cap a 800
   non è solo un prerequisito della v4.1: sana un difetto vivo, e la
   rigenerazione cache riparerà anche i brief esistenti che oggi finiscono a
   metà frase.
2. **Le metriche di §8.2 sono misurabili con l'harness attuale, senza
   modifiche**: `outputs.jsonl` esporta il breakdown per rubrica a ogni
   iterazione, ed esiste `-rescore` per rivalutare output esistenti senza
   chiamate LLM. Baseline: `caveat_coverage` 15/15 e `dd_questions` 10/10 su
   tutti i run — "nessuna regressione" significa restare al soffitto: criterio
   severo e verificabile.

Un solo emendamento residuo, non bloccante ma da correggere prima dell'eval:

- **§8.2 indebolisce la metrica prolisso.** La v4 originale chiedeva la
  *riduzione* del warning; la revisione chiede solo che *"non peggiori"*. Ma la
  riduzione della prolissità è l'obiettivo n. 3 della proposta e il warning
  scatta oggi su **33/100 run baseline**: "non peggiora" lascerebbe l'obiettivo
  senza alcuna verifica. Ripristinare: *tasso prolisso ≤ baseline, atteso in
  riduzione; se non si riduce, l'obiettivo 3 non è dimostrato e va spiegato
  nella review manuale*.

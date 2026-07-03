# Binocolo — proposta prompt FDD v4.1 per `ma_deep_brief`

> Proposta aggiornata dopo la contro-analisi `PROMPT-FDD-FAB.md` del
> 2026-07-03. Obiettivo: mantenere **GPT-5.5** come modello default e migliorare
> la qualità FDD dell'output senza cambiare schema JSON, DB o UI prodotto. La
> migration resta sul prompt; prima dell'apply serve però un piccolo prerequisito
> code-side sul parser (`valuationRationale` 600 → 800 rune).

## 1. Contesto

Prompt attuale rilevante:

- `deploy/migrations/040_binocolo_ma_deep_brief_prompt.sql` — v1, brief sintetico.
- `deploy/migrations/045_anisetta_binocolo_ma_deep_brief_rich_prompt.sql` — v2, dossier ricco.
- `deploy/migrations/097_anisetta_mrsmith_binocolo_thesis_reading_prompt.sql` — v3, rinomina `thesisReading` → `financialReading` e separa la vera lettura di tesi nello scope `ma_thesis_reading`.

Valutazione modello:

- GPT-5.5 resta il default consigliato per `ma_deep_brief`.
- Nell'eval su 10 aziende × 10 iterazioni:
  - JSON validi: 100/100;
  - RAG flip: 0%;
  - numeri sospetti: 0%;
  - punteggio medio: ~99,7;
  - unico warning ricorrente: output talvolta prolisso.

Dal punto di vista FDD l'output è buono per **screening M&A interno / pre-FDD**, ma può essere reso più senior: meno elenco KPI, più implicazione deal, più disciplina sui limiti dei dati.

## 2. Decisione aggiornata

Procedere con una **v4.1 FDD**:

- modello invariato: **GPT-5.5**;
- schema JSON invariato: `MADeepBrief`;
- prompt più orientato a lettura FDD buy-side preliminare;
- brief sempre **neutro**, non thesis-scoped;
- niente go/no-go assertivo nel dossier neutro;
- validazione v3 vs v4.1 sullo stesso manifest prima di qualsiasi rollout cache.

La contro-analisi FAB è accettata: la v4 originale era corretta nella direzione, ma troppo ottimista su cap dei campi, metriche di accettazione e rischio di overreach qualitativo.

## 3. Obiettivo della v4.1

Rendere il brief più vicino a una lettura **buy-side FDD preliminare**, utile a decidere dove concentrare la due diligence:

1. evidenziare temi che possono influenzare prezzo, struttura dell'operazione o closing accounts;
2. gerarchizzare le red flag per materialità FDD;
3. ridurre prolissità e precisioni numeriche inutili;
4. leggere EBITDA per livello, trend e prudenzialità, senza affermare una QoE non dimostrabile dai dati pubblici;
5. rafforzare PFN/debt-like, working capital, perimetro e bridge equity;
6. trasformare le DD questions in richieste informative operative.

## 4. Non-obiettivi

- Non cambiare modello: resta GPT-5.5.
- Non cambiare schema JSON di `MADeepBrief`.
- Non aggiungere tabelle, dashboard o UI prodotto.
- Non introdurre nuova logica FDD deterministica nel prompt: i numeri restano calcolati dal backend.
- Non sostituire la FDD advisor-grade: il brief resta preliminare.
- Non spostare la lettura di tesi nel dossier neutro: quella resta nello scope `ma_thesis_reading`.

## 5. Problemi osservati nel prompt attuale

### 5.1 Troppo descrittivo

Il modello tende a riassumere molte metriche in sequenza. È corretto, ma meno utile di una lettura per impatto:

- livello e trend EBITDA;
- prudenzialità dell'EBITDA quando calcolata dal backend;
- tensione di cassa;
- debt-like items;
- perimetro standalone;
- implicazione su valuation.

### 5.2 Materialità non sempre esplicita

Le red flag sono sensate, ma il prompt v3 non impone di ordinarle per potenziale impatto su:

- prezzo;
- struttura dell'operazione;
- closing accounts;
- perimetro dell'operazione.

La v4.1 deve evitare formulazioni go/no-go assertive: il go/no-go dipende dalla tesi e appartiene allo scope `ma_thesis_reading`.

### 5.3 Numeri troppo precisi

Esempi osservati:

- `13.441200846309473%`;
- multipli e ratio con troppe cifre decimali.

Per un memo FDD sono rumore e peggiorano leggibilità. L'harness tollera arrotondamenti ragionevoli, quindi il prompt può imporli senza generare falsi positivi.

### 5.4 Strengths troppo permissive

Talvolta metriche come ROE/ROI possono apparire tra i punti di forza anche quando sono KPI di contorno o distorti da patrimonio netto basso. In FDD vanno usati con cautela.

### 5.5 DD questions buone ma talvolta generiche

Le domande dovrebbero assomigliare di più a una information request list:

- aging crediti/debiti;
- top clienti e contratti;
- normalizzazioni EBITDA;
- scadenzario PFN;
- garanzie/covenants;
- parti correlate;
- lease/B.8;
- TFR/fondi rischi/debiti fiscali.

### 5.6 Overreach qualitativo

La v4 originale parlava di “qualità e sostenibilità dell'EBITDA”. Con il solo dato CEE pubblico questo può diventare prosa senior non supportata:

- non abbiamo dettaglio ricavi ricorrenti;
- non abbiamo management accounts;
- non abbiamo normalizzazioni advisor-grade;
- non vediamo tutti gli one-off.

La v4.1 deve parlare di **livello, tendenza e prudenzialità dell'EBITDA**; la vera EBITDA quality si accerta in DD.

## 6. Regole FDD da aggiungere

### 6.1 Postura

Scrivere come analista FDD buy-side, non come generatore di descrizioni KPI.
Ogni sezione deve rispondere implicitamente a:

> Che cosa cambia per un potenziale acquirente?

Paletto: il brief resta neutro. Deve parlare di impatto potenziale su prezzo, struttura e closing accounts, non di go/no-go assertivo.

### 6.2 Priorità analitiche

Ordinare la lettura secondo questa gerarchia, quando i dati la coprono:

1. livello, tendenza e prudenzialità dell'EBITDA;
2. PFN, TFR, finanziamenti soci, fondi rischi e altri potenziali debt-like items;
3. working capital, liquidità e cash conversion;
4. perimetro standalone vs gruppo/controllate/partecipazioni;
5. key person, governance e parti correlate;
6. implicazione sulla valuation e sul rischio prezzo.

Reintegrare la dimensione temporale: leggere anche l'andamento sull'anno precedente quando disponibile.

### 6.3 Numeri

- Usare solo numeri presenti in `scorecard`/`valuation`.
- Non cambiare scala dei numeri.
- Non usare `k`, `M`, `mln`.
- Percentuali: massimo 1 decimale.
- Multipli/ratio: massimo 2 decimali.
- Euro: interi, senza centesimi.
- Non citare decimali lunghi anche se presenti nell'input.

### 6.4 Strengths

- Massimo 5, non 6.
- Devono essere veri punti di forza deal-relevant.
- Non usare ROE/ROI come strength principale se:
  - sono classificati come `tier: contorno`;
  - il patrimonio netto è basso o negativo;
  - l'indicatore è distorto dalla scala micro.
- Preferire PFN, liquidità, margine EBITDA, crescita, ricorrenza/cash conversion se presenti.

### 6.5 Red flags

- Massimo 7.
- Ordinate per materialità FDD.
- Ogni `claim` deve includere anomalia osservata e possibile impatto su prezzo, struttura dell'operazione o closing accounts.
- `claim` massimo ~40 parole, per evitare troncamenti nel parser.
- Categorie ammesse nel prompt:
  - `finanziario`;
  - `qualita_ebitda`;
  - `working_capital`;
  - `debt_like`;
  - `perimetro`;
  - `governance`;
  - `mercato`;
  - `struttura`.

Nota: lo schema Go accetta `category` string libera. Non serve migrazione strutturale. La UI oggi mostra la categoria raw: la mappa slug → label è follow-up non bloccante.

### 6.6 Valuation

Il campo `valuationRationale` deve essere più disciplinato della v4 originale.

Contenuti obbligatori:

1. metodo e multiplo;
2. EV vs equity con bridge principale;
3. caveat su multiplo e perimetro;
4. chiarimento che la banda non è un prezzo certo.

Contenuti opzionali se lo spazio lo consente:

- standalone vs controllate/partecipazioni;
- impatto delle red flag sulla lettura della banda.

Regola bridge: rispettare le note delle righe. I finanziamenti soci sono già inclusi nella PFN quando indicato dal bridge; la loro riga è informativa/negoziale, non un'ulteriore deduzione.

Prerequisito code-side prima della migration: portare il cap parser di `valuationRationale` da 600 a 800 rune.

### 6.7 DD questions

- Massimo 8.
- Devono essere richieste informative concrete.
- Non ripetere quelle già presenti nelle red flags.
- Ordinare per priorità di DD.
- Se un dato materiale manca, dichiararlo come limite dell'analisi e trasformarlo in richiesta informativa; non trattare l'assenza come un problema in sé.

Nota architetturale: oggi `redFlags[].ddQuestion` non entra nel seed IRL; l'IRL usa `brief.ddQuestions`, flag deterministici, thesis reading e template famiglia. Aggiungere le DD question delle red flag come quinta fonte IRL è follow-up non bloccante.

## 7. Prompt proposto — `Brief dossier M&A v4.1 FDD`

Stesso JSON della v3; differenze principali rispetto alla v4 originale:

- dossier neutro esplicitato;
- qualità EBITDA riformulata come livello/trend/prudenzialità;
- YoY reintegrato;
- niente go/no-go;
- nota sui finanziamenti soci già inclusi nella PFN;
- claim red flag max 40 parole;
- valuationRationale ridotto ai contenuti essenziali.

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

## 8. Condizioni bloccanti prima della migration

### 8.1 Parser `valuationRationale`

Aggiornare `parseMADeepBrief`:

```go
brief.ValuationRationale = cleanText(brief.ValuationRationale, 800)
```

Motivo: la v3 baseline ha già output valuation spesso vicini al cap 600; la v4.1 chiede una valuation più disciplinata e non deve rischiare troncamenti a metà frase.

### 8.2 Metriche di accettazione integrate

Oltre alle metriche già previste, accettare la v4.1 solo se:

- score medio ≥ baseline GPT-5.5 − 1 punto;
- JSON fail: 0%;
- RAG flip: 0%;
- hallucination candidate: 0%;
- nessun numero sospetto;
- nessuna regressione su `caveat_coverage`;
- nessuna regressione su `dd_questions`;
- tasso warning `output molto prolisso` ≤ baseline, atteso in riduzione; se non si riduce, l'obiettivo di minore prolissità non è dimostrato e va spiegato nella review manuale.

### 8.3 Review manuale cieca

Fare review manuale **v3 vs v4.1 fianco a fianco** sugli stessi 10 casi, senza sapere quale output sia quale.

La review deve cercare esplicitamente:

- affermazioni di EBITDA quality/sostenibilità non supportate;
- go/no-go impliciti nel dossier neutro;
- confusione EV/equity;
- double-counting narrativo su finanziamenti soci;
- DD questions troppo generiche;
- caveat rilevanti omessi.

## 9. Strategia di rollout consigliata

### 9.1 Migration

Creare una nuova migration che aggiorna il prompt default `ma_deep_brief` in `mrsmith.llm_prompt`:

- scope: `ma_deep_brief`;
- name: `Brief dossier M&A v4.1 FDD`;
- prompt: testo sopra;
- migration solo su `mrsmith.llm_prompt`;
- nessun `INSERT`/`UPDATE` su `mrsmith.llm_model`.

Prima dell'apply verificare la riga registry live del modello default: “modello invariato” significa che la migration non tocca `llm_model`, non che deduciamo il binding dal seed storico.

Come nelle migration precedenti, è preferibile update in-place del default per mantenere semplice la risoluzione runtime. Il prompt ID può restare stabile; la provenance storica dei brief già generati resta ricostruibile dall'audit delle chiamate.

### 9.2 Eval prima del rollout massivo

Usare lo stesso harness eval già creato:

1. stesso `case_manifest.json` del baseline GPT-5.5;
2. 10 aziende × 10 iterazioni;
3. stesso modello GPT-5.5;
4. solo prompt diverso.

Confrontare non solo `summary.csv`, ma anche il breakdown per rubrica.

### 9.3 Rollout cache

Solo dopo accettazione:

- eseguire rigenerazione brief sui record cached;
- non ricomprare IT-full;
- non sovrascrivere altri artefatti deterministici;
- mantenere model/prompt provenance aggiornata sui brief rigenerati.

Nota rollout: se le card hanno già IRL seminate, una rigenerazione brief può produrre DD questions riformulate. Un nuovo seed IRL è additivo per hash normalizzato e può proporre quasi-duplicati; avvisare gli analisti prima del re-seed.

## 10. Criteri qualitativi di accettazione FDD

Un output v4.1 è migliore della v3 se:

- apre subito con il tema deal principale;
- distingue health score da qualità del deal;
- non fa affermazioni di qualità/sostenibilità dell'EBITDA che i dati non mostrano;
- dichiara i dati materiali mancanti invece di tacerli;
- non confonde EV ed equity;
- menziona debt-like items quando il bridge li espone;
- non promuove KPI di contorno come strength principale;
- le red flag sono ordinate per materialità;
- le DD questions sembrano voci di una information request list;
- è più breve ma più decision-useful;
- resta neutro e non invade la lettura di tesi.

## 11. Follow-up non bloccanti

- Arrotondamento a monte nel builder del payload backend, così il modello non riceve decimali inutili.
- Mappa frontend slug → label per categorie red flag (`qualita_ebitda`, `debt_like`, ecc.).
- Aggiungere `redFlags[].ddQuestion` come quinta fonte del seed IRL.

## 12. Addendum implementativo — eval v4.1 e correzione v4.2

La migration `100_anisetta_mrsmith_binocolo_ma_deep_brief_prompt_v4_1.sql` è stata applicata ed è stato eseguito l'eval GPT-5.5 sul manifest baseline:

```text
artifacts/llm-evals/ma-deep-brief/20260703T161931Z-9a8f4a62-openai-gpt-5-5/
```

Esito: **v4.1 non accettata per rollout**.

Motivi:

- JSON fail: 2/100 (`unexpected end of JSON input`);
- cause osservate: output troncati a `completion_tokens = 5000`;
- warning `output molto prolisso`: 97/100, peggiora rispetto al baseline v3 (33/100);
- `caveat_coverage` e `dd_questions` restano a soffitto, quindi il problema non è la copertura FDD ma la verbosità.

La direzione FDD resta corretta, ma il prompt v4.1 concede troppe liste lunghe:

- strengths quasi sempre a 5/5;
- red flags spesso a 6-7;
- ddQuestions sempre a 8/8.

Correzione: introdurre una **v4.2 FDD concisa**, sempre prompt-only, con:

- output totale target ~4200 caratteri;
- strengths max 4;
- red flags max 5;
- ddQuestions max 6;
- `valuationRationale` max 3 frasi / 550 caratteri;
- claim red flag max 28 parole;
- ddQuestion red flag max 24 parole.

Migration prevista:

```text
deploy/migrations/101_anisetta_mrsmith_binocolo_ma_deep_brief_prompt_v4_2.sql
```

## 13. Addendum implementativo — eval v4.2

La migration `101_anisetta_mrsmith_binocolo_ma_deep_brief_prompt_v4_2.sql` è stata applicata ed è stato eseguito l'eval GPT-5.5 sullo stesso manifest baseline:

```text
artifacts/llm-evals/ma-deep-brief/20260703T163730Z-9a8f4a62-openai-gpt-5-5/
```

Esito automatico: **v4.2 accettata per review manuale**.

Confronto sintetico:

| Metrica | Baseline v3 | v4.1 | v4.2 |
|---|---:|---:|---:|
| Avg score | 99,67 | 97,03 | 100,00 |
| JSON fail | 0/100 | 2/100 | 0/100 |
| RAG flip | 0/100 | 0/100 | 0/100 |
| Numeri sospetti | 0/100 | 0/100 | 0/100 |
| `caveat_coverage` | 15/15 | 15/15 | 15/15 |
| `dd_questions` | 10/10 | 10/10 | 10/10 |
| `output molto prolisso` | 33/100 | 97/100 | 0/100 |
| Lunghezza mediana raw output | ~5.520 char | ~6.791 char | ~4.429 char |
| ValuationRationale mediana | ~531 char | ~569 char | ~302 char |

La v4.2 risolve il problema della v4.1: preserva coverage FDD e validità numerica, ma rimuove troncamenti e prolissità.

## 14. Decisione proposta aggiornata

Procedere con la **v4.2 FDD concisa**, GPT-5.5 invariato:

1. cap `valuationRationale` a 800 — già implementato nel parser;
2. default `max_tokens` backend a 8000 quando il registry non lo specifica — headroom contro spike di reasoning, mentre il prompt limita già l'output;
3. migration `100` v4.1 — applicata ma non accettata per rollout;
4. migration `101` v4.2 concisa — applicata e promossa alla review manuale;
5. review manuale cieca v3 vs v4.2 sugli stessi 10 casi;
6. se la review manuale conferma, rigenerazione cache dei brief.

La qualità misurata del modello è già sufficiente. Il miglioramento atteso non viene da un nuovo LLM, ma da un prompt FDD più vincolato, più onesto sui limiti dei dati e abbastanza conciso da non rischiare troncamenti.

## 15. Addendum modello alternativo — Cerebras gpt-oss-120b su v4.2

Su richiesta è stato rieseguito un ciclo completo con Cerebras `gpt-oss-120b`, usando lo stesso prompt v4.2 e lo stesso manifest baseline:

```text
artifacts/llm-evals/ma-deep-brief/20260703T170348Z-82386b4b-gpt-oss-120b/
```

Durante l'analisi è stato corretto lo scorer per non marcare come sospetti i numeri validi formattati con separatori migliaia inglesi o spazi/narrow no-break space (es. `€1,186,388`, `209 916`). Dopo rescore:

| Metrica | GPT-5.5 v4.2 | gpt-oss-120b v4.2 |
|---|---:|---:|
| Avg score | 100,00 | 96,98 |
| JSON fail | 0/100 | 0/100 |
| RAG flip | 0/100 | 0/100 |
| Hallucination candidate | 0/100 | 12/100 |
| Run < 90 | 0/100 | 14/100 |
| `output molto prolisso` | 0/100 | 0/100 |
| Latenza media | ~51,9s | ~2,5s |

Lettura: v4.2 rende `gpt-oss-120b` stabile sul formato e molto veloce, ma non elimina il problema già osservato sui numeri/unità. Restano casi in cui il modello usa `mln`, `k` o scale ambigue su EV/equity, e alcuni caveat deterministici non vengono citati. Per questo resta non production-safe per il brief finale FDD, pur essendo un possibile candidato futuro per preview interna veloce.

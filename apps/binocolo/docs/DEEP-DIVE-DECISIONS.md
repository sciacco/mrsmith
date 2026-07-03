# Binocolo — Deep-dive: decisioni di redesign dell'analisi

> Sintesi del brainstorming 2026-07-03 (utente + analista FDD) sulla funzionalità
> `POST /binocolo/v1/ma/sessions/:id/deep-dive` e artefatti collegati (scorecard,
> valuation, brief, dossier card). **Questo è un registro di decisioni di design,
> non un piano di implementazione**: le fasi, le migrazioni e la sequenza si
> definiscono in un documento successivo. Contesto vincolante: l'app serve il
> **team M&A interno** (compratore strategico, target PMI ICT + adiacenti) — la
> coerenza tra target vale più della precisione assoluta; la QoE vera resta
> all'advisor esterno in DD.

## 0. Stato attuale (per riferimento)

Il deep-dive promuove i preferiti (rating ≥1★) a dossier per azienda: worker
asincrono compra IT-full (~€0.30, cache globale per `company_key`), motore
deterministico legge i KPI vendor e assegna RAG (`ma_deep_engine.go`),
valutazione da multiplo Damodaran per prefisso ATECO 2 cifre + haircut PMI flat
30% + banda ±15%, brief LLM thesis-neutral (scope `ma_deep_brief`). Varianti:
deep-dive per card (B6) e dossier P.IVA standalone.

## 1. Fatti verificati sul dato IT-full (base empirica)

Verificati su spec (`company.openapi.json`), legend (`company-legend.html`) e
**due payload reali** (MFT ITALIA SRL, micro/impiantistica, bilancio 2024;
CDLAN SPA, media/TLC-DC, bilancio 2025 — n=2):

- **IIC/IPL sono divisioni di bilancio alternative, NON due esercizi.** Un
  deposito l'anno, una divisione, una famiglia di codici → **un solo esercizio
  di dettaglio CEE per payload**.
- **Verifica sistematica codici (2026-07-03)**: legend = **276 codici IIC**
  estratti in mappa machine-readable (`artifacts/claude/iic_legend_map.json`);
  255 codici usati da ciascun payload reale, tutti coperti dalla legend tranne
  3 che sono **refusi della legend stessa propagati fedelmente dall'API**:
  `IPL231`/`IPL232` (la riga C.16.a "interessi da collegate/da controllanti"
  ha IPL stampato anche nella colonna IIC) e `IICC351` (doppia C, "financial
  assets for central pool management"). La mappa codici del motore deve
  includerli **verbatim** come codici canonici. **104/104 identità contabili
  passate** sulle due fixture (attivo/passivo, PN, debiti per voce, CE per
  sezione, risultato, cross-check con ecofin) e **tutte le semantiche
  dichiarate nei nostri documenti coincidono con le etichette ufficiali**.
- **Semantica codici debiti**: codice base = quota entro l'esercizio, codice
  gemello = quota oltre, IIC329-343 = totali per voce. NOTA DI PROVENIENZA: la
  legend NON esplicita "entro" sui codici base — è un fatto **derivato dai
  dati** e provato dalle identità contabili (15 triple base+oltre=totale ×2
  payload, somma basi = IIC116, somma oltre = IIC117). Anche una MICRO
  deposita il dettaglio D completo.
- **Definizione PFN vendor reverse-engineered**: debito lordo = D.1+D.2+D.3
  soci+D.4 banche+D.5 **+ B.3 derivati passivi (IIC219)**; PFN = lordo − cassa
  C.IV. Riconcilia a rounding su entrambi (MFT: scarto 4€ con IIC219=0;
  CDLAN: scarto ~25€ solo includendo IIC219=7.241). `ebitdaGrossLeverage ×
  EBITDA` = debito lordo ✓.
- **Definizione EBITDA vendor esatta**: A(IIC130) − B(IIC149) + B.10(IIC144) =
  `operatingResults.ebitda` al centesimo su entrambi; l'add-back include tutto
  B.10 compresa la svalutazione crediti B.10.d (≠0 in CDLAN). Aperto:
  trattamento B.12/13 (zero in entrambi i sample).
- **Semantica `annualResult`**: IIC177 = risultato ante imposte, IIC178 =
  imposte ("20. Total income tax"), IIC179 = utile netto ("21. PROFIT/LOSS FOR
  THE YEAR"; = A.IX di `netWorth`, IIC083). Fonte primaria: la legend, che
  esplicita ogni codice; confermata per sottrazione esatta su entrambi i
  payload. (Il falso "quirk ROE" nasceva dall'aver inferito la semantica dalle
  grandezze invece di consultare la legend: da qui la regola che la mappa
  codici→voci del motore si genera dalla legend per intero, mai a mano.)
- **`operatingResults` porta gli assoluti dell'anno precedente**: `ebitL2Y`,
  `ebitdaL2Y`, `cashFlowL2Y` (verificati contro i delta %). Due anni di
  EBIT/EBITDA/CF in euro senza acquisti extra.
- **Delta YoY vendor inutilizzati** nei payload già comprati:
  `development.grossFinancialDebt`, `development.totalAssets`,
  `development.addedValue`, `employees.employeeTrend` (oggi si usano solo
  turnoverTrend e ebitVariation).
- **Quirk vendor**: lo spec dichiara `ebitVariation` = "change in EBITDA" ma è
  la variazione EBIT (frazione); `mol` è la variazione EBITDA (percento) —
  scale miste nello stesso subset.
- **RITRATTATO (col secondo payload) il presunto "quirk ROE"**: era una
  lettura errata dei codici `annualResult` (IIC178 = imposte, non utile). Col
  codice giusto il ROE vendor = IIC179/PN riconcilia esattamente su entrambi
  (MFT 486/80.993 = 0,60%; CDLAN 2.171.236/7.876.018 = 27,57%) e ROI =
  EBIT/(PN+PFN) idem (19,90% / 17,33%). **Nessun misgrading in produzione.**
  Resta il principio: i numeri da prezzo si ancorano ai CEE e la
  riconciliazione sistematica è ciò che scova sia le definizioni vere sia i
  falsi allarmi.
- **Voci del bridge verificate negli enum**: TFR = IIC089
  (`liabilitiesAggregateValues`); finanziamenti soci D.3 = IIC331 + IIC184/185
  (`debts`); cassa = IIC067-070; fondi rischi = IIC085-088; A.4 = IIC127;
  contributi = IIC129; B.8 = IIC133; goodwill = IIC007; variazione rimanenze
  B.11 = IIC145.

**Validazione sulla cache ESEGUITA (inspect Fase 0, 2026-07-03, n=10):**
divisione PL mai osservata (10/10 IIC, refusi legend in tutti); granularità:
8 con dettaglio split, 0 solo-totali, **2 senza array debiti** (la catena di
fallback con provenienza serve davvero); riconciliazione EBITDA 10/10 a 0,00%
e PFN 6/6 entro 0,03% incluse PFN negative → soglia `vendor_cee_tolerance_pct`
fissata a **1%**; L2Y assoluti 10/10, employeeTrend 8/10, grossFinancialDebt
7/10; B.12/13 sempre zero (questione definitoria aperta ma immateriale — la
sentinella automatica è il flag di riconciliazione). Resta non verificata solo
la **scala** di `development.grossFinancialDebt` (frazione vs percento):
decidibile solo con un bilancio precedente in mano, arriverà con le vintage.

## 2. Filone B — EBITDA da deal, PFN vera, equity bridge

**Decisioni:**

1. **PFN contabile dai CEE** al posto di `pfnEbitda × EBITDA` (che salta con
   EBITDA ≤ 0 ed eredita definizioni opache). Catena di fallback con
   **provenienza dichiarata** all'analista: dettaglio con scadenze → totali per
   voce → ratio vendor. Bonus: lo split entro/oltre dà la struttura a
   breve/lungo del debito bancario.
2. **Equity bridge esplicito a righe**, non sottrazione secca:
   `EV (banda) − PFN − TFR (100%) − fondo imposte ± finanziamenti soci = equity`.
   I soci sono **riga separata evidenziata** (al closing vengono spesso
   rinunciati/convertiti: tema negoziale, non contabile). Altri fondi rischi:
   riga informativa. Il ponte compare come tabella nel dossier **e
   nell'export** — le caveat viaggiano dentro il numero, non a piè di pagina.
   Caso dimostrativo MFT: PFN 14.7k (verde smagliante) ma TFR 100.6k > PN
   80.9k → l'equity si sposta di ~⅓.
3. **Banda asimmetrica ancorata all'EBITDA prudenziale.** Estremo alto =
   EBITDA reported × multiplo × 1.15; estremo basso = EBITDA prudenziale
   (reported − A.4 capitalizzazioni − contributi IIC129) × multiplo × 0.85.
   La banda si allarga in proporzione alla distorsione **misurata** — niente
   "+N punti per flag". Con A.4/contributi a zero degenera nel ±15%.
   Prudenziale ≤ 0 → estremo basso su EV/Sales (riusa fallback esistente).
4. **Flag non quantificabili** (peso lease B.8, scarto vendor-vs-CEE, equity
   erosa, A.5/EBITDA sopra soglia) = annotazioni di confidenza sul blocco
   valuation + domanda DD ciascuno. **Mai dentro la matematica della banda,
   mai nel RAG** (il RAG resta salute azienda; qualità dati = canale visivo
   separato). Il flag A.5 si pesa **sull'EBITDA, non sui ricavi** (nel sample:
   A.5 = 2.5% dei ricavi ma 33% dell'EBITDA).
   In aggiunta (dal secondo payload): flag **"perimetro standalone"** quando
   le partecipazioni in controllate (IIC019) pesano sull'attivo o i proventi
   da partecipazioni C.15 (IIC150) pesano sull'EBITDA — la banda EV/EBITDA
   standalone non vede le controllate (CDLAN: partecipazione 7,69M = 49%
   dell'attivo, dividendi 1,1M = 43% dell'EBITDA). Caveat sulla valuation +
   domanda DD (consolidato? somma delle parti?). Coerente con il
   ThesisFitHoldingHaircut che il funnel applica già a monte.
5. **EBITDA canonico = vendor, con riconciliazione CEE** come sanity check;
   soglia di scarto calibrata sui primi payload reali (evidence-gated), scarto
   oltre soglia = annotazione + domanda DD, non declassamento RAG.
6. **Convenzioni come policy di team in `ma_parameter`** (con audit), non
   scelte per-analista: comparabilità tra target nel tempo.
7. Rettifica EBITDA dell'analista (compensi soci, ecc.): **futura**,
   context-scoped sulla card (D3), mai nella cache globale.

## 3. Filone A — Profondità temporale

**Decisioni:**

1. **Livello 0 (gratis, dati già comprati)**: gruppo "qualità del margine /
   momentum" nel dossier con ΔEBITDA **assoluto** (da `ebitdaL2Y`), delta YoY
   vendor (debito lordo, attivo, valore aggiunto, organico) letti **in
   combinazione** (EBITDA↓ + debito↑; ricavi↑ + VA↓ = deriva
   reseller/body-rental), segnale **cash-conversion single-year** da B.11
   (nel sample: magazzino +115.7k > EBITDA 94.9k a fatturato piatto). Rollout
   sui payload in cache via `recompute`.
2. **DECISO: informativi nel dossier, FUORI dal rank del funnel** — sono
   qualità del margine, non fit strategico; non ripetere l'errore
   coverage-nel-rank già criticato nello scoring.
3. **Livello 1 — versioning dei payload**: oggi `SaveMADeepReady` sovrascrive
   → la prima vintage va persa al primo refresh. Versionare per data bilancio.
   €0.30/azienda/anno sui target in watch = archivio pluriennale proprietario
   che il vendor non vende. **Decisione da prendere prima che serva.**
4. **Livello 2 (evidence-gated)**: storico 3-5 esercizi da fonte esterna solo
   per i finalisti in lavorazione. Integrazione nel tool da valutare quando il
   bisogno si presenta con numeri in mano.

## 4. Filone E — Lettura di tesi context-scoped

**Decisioni:**

1. **Il dossier deep resta thesis-neutral e globale** (economia della cache
   intoccabile). Si aggiunge un secondo strato: la **"lettura di tesi"** per
   `(iniziativa, azienda)` sulla card — il rapporto FDD e l'investment memo
   sono due documenti, come nella prassi.
2. **La tesi NON va sull'iniziativa.** La ricerca è centrica; l'iniziativa è
   un contenitore pratico per più ricerche territoriali. La lettura si ancora
   alla **tesi della sessione di provenienza** — infrastruttura esistente
   (`CreatedFromSession`, `MACardProvenance`). Provenienze multiple → sessione
   del rating più recente; card senza provenienza → solo dossier neutro.
   Proprietà robusta: anche in un'iniziativa "incoerente" ogni card è letta
   contro la tesi che l'ha davvero fatta emergere.
3. **Generazione on-demand**: azione esplicita sulla card (pattern B6), cache
   per `(iniziativa, azienda)`, rigenerazione esplicita se la tesi cambia.
   Nessun aggancio alla state machine del board, nessun automatismo su
   drag-and-drop. Promozione ad automatica-alla-creazione solo con evidenza
   d'uso.
4. **Contenuto e disciplina**: verdetto di fit che **cita i fatti**; red flags
   ri-pesate (stesse flag del brief neutro, mai nuove: bloccanti/tollerabili
   per questa tesi); domande DD di tesi additive; ipotesi di sinergia
   **etichettate come ipotesi**; postura valutativa **senza numeri nuovi** (la
   banda resta neutra, mai premi di sinergia quantificati). Include evidenza
   web UC2 (già per `company_key`) **con data dichiarata**. Regola prompt: se
   la tesi non specifica un aspetto → "la tesi non si esprime su X", non
   indovinare. Niente vincoli strutturati accanto alla tesi (testo libero).
5. **Anchor risk**: la lettura sta sotto/accanto ai fatti (tab dedicato), mai
   come headline; il disaccordo dell'analista si cattura (pattern override
   tesi dello scoring v3), è il ground truth di domani.
6. **Pulizie collegate**: rinominare `thesisReading` nel brief neutro (è la
   lettura finanziaria) al prossimo giro prompt via `regenerate-briefs`;
   rimuovere `ThesisFit` dalla struct neutra (casa sua = strato di contesto).

## 5. Filoni C+D — Lente compratore, famiglie di business model, multipli

**Decisioni:**

1. **Lente compratore al posto della lente creditizia** (ratificata
   dall'utente). Il semaforo risponde ad "attrattività per un acquirente", non
   a "merito di credito". Metriche **core** (guidano l'overall RAG) = ciò che
   si eredita/compra: PFN+TFR/EBITDA, margine EBITDA e sua qualità (flag di B),
   cash-conversion (A), trend. Metriche **di contorno** (informano, non
   guidano) = leverage attivo/PN, capitalizzazione, current/acid — restano
   visibili (segnale negoziale: dicono quanta fretta ha il venditore) ma un
   rosso lì non affonda il semaforo.
2. **Una classificazione, due consumatori**: la **famiglia di business model**
   per azienda alimenta sia le soglie RAG (C) sia la selezione della riga
   Damodaran (D). Fatto globale thesis-neutral, coerente con la
   stratificazione di E. *Rettifica in progettazione*: NON in `MACompanyFact`
   (la mig 090 ratifica che i fact sono presentation-only, "NEVER read by
   gate/routing/scoring", e il kind è CHECK-vincolato) → tabella propria
   `ma_company_bm_family` sul pattern di `ma_company_domain` (mig 082), con
   campi suggestion vs ratifica. Vedi
   [`DEEP-DIVE-IMPLEMENTATION-PLAN.md`](./DEEP-DIVE-IMPLEMENTATION-PLAN.md).
3. **4 famiglie ICT-native + default** (terreno di caccia dichiarato: ICT +
   adiacenti):

   | Famiglia | Margine EBITDA ok/good | ROS ok/good | Ciclo fin. good/ok | Riga Damodaran |
   |---|---|---|---|---|
   | Servizi ricorrenti (MSP, hosting, TLC gestite) | 10 / 18% | 6 / 12% | 30 / 75 gg | Computer Services |
   | Progetto & integrazione (SI, impiantistica) | 6 / 12% | 4 / 8% | 90 / 150 gg | Engineering/Construction |
   | Rivendita / VAR | 3 / 7% | 2 / 5% | 45 / 90 gg | Retail (Distributors) |
   | Software prodotto (ISV, SaaS) | 15 / 25% | 10 / 18% | 15 / 60 gg | Software (System & Application) |
   | *(default: adiacenti non classificati)* | soglie attuali | — | 60 / 120 gg | riga da prefisso ATECO |

   Soglie indicative da prassi, in `ma_parameter`. Leva/liquidità uniformi
   (sono contorno). **No soglie per-ATECO** (nessuna base dati, nessun
   bisogno).
4. **Assegnazione famiglia**: default deterministico da ATECO dove non ambiguo
   (46.51 → rivendita, 43.2 → progetto, 61 → servizi ricorrenti); per il blob
   62/63 — dove l'ATECO fallisce — **suggerimento deterministico dalla
   struttura dei costi CE** (B.6 merci/produzione > ~40% → rivendita;
   magazzino + risconti → progetto; A.4 rilevante → software prodotto;
   personale/VA alto + magazzino ~0 → servizi), mostrato **con la sua
   evidenza**, ratifica/override un-click dell'analista. No LLM nel percorso.
5. **Bug di selezione riga nel seed 039** (*rettificato in Fase 3*: il seed ha
   già granularità a 4 cifre su 62xx — 6202/6203/6209 → Computer Services
   12.03×; il problema "tutto il 62 a 20.85×" riguarda solo i codici 62/620
   generici e il 6201, e resta il disallineamento identità-vs-codice: la
   software house codificata 6202 o l'MSP codificato 6201 prendono il multiplo
   sbagliato). Restano interi: 58 → "Publishing & Newspapers" (software house
   a multipli da giornali); 43 → "Construction Supplies" (fornitori di
   materiali, non installatori — Engineering/Construction 9.71× è la riga
   giusta); 63 → n=6. Fix: righe Damodaran per famiglia (stesso dataset,
   chiave sintetica `FAMILY:*`), risoluzione = famiglia → prefisso → TOTAL:
   la classificazione segue l'identità dell'azienda, non il codice camerale.
6. **Haircut PMI graduato per taglia** al posto del flat 30%: 3 scaglioni di
   fatturato parametrizzati (indicativi: <€5M → 35-40%; €5-20M → 25-30%;
   >€20M → 15-20%).
7. **Caveat di eterogeneità** dove la famiglia non è ratificata e il prefisso
   è disperso (62, 63, 46…): dichiarare l'ampiezza ("il posizionamento di
   business può spostare il multiplo anche del ±50%"), non fingere precisione.
   **Disclaimer rinforzato sul percorso EV/Sales** (il numero meno
   trasferibile da quotate a PMI).
8. **Percentile di coorte** dalla ricerca (i dati Advanced comprati dallo
   stesso funnel = peer set in-settore per costruzione): annotazione
   informativa nel dossier su coorte **fotografata** alla data della ricerca.
   **Mai nel ranking** — la decisione scoring v3 (niente percentili su pool
   mobile nel rank) resta intatta.
9. **Evidence-gated**: tabella comps interna dal deal flow del team (ogni deal
   chiuso/prezzato/visto = un multiplo osservato su PMI italiane reali — il
   dato che Damodaran non avrà mai; parte vuota, compone nel tempo).

**Caso dimostrativo MFT con lo stack completo**: famiglia progetto &
integrazione → margine 7.8% ambra (non rosso), ciclo 105gg ambra (norma
commesse), leverage/capitalizzazione a contorno, multiplo da
Engineering/Construction. Verdetto da "Critico" a "nella norma del suo modello
— TFR sopra il patrimonio e cassa assorbita dal magazzino: due verifiche prima
di parlare di prezzo".

## 6. Filone F — Information Request List (IRL) per card

**Decisioni:**

1. **Artefatto "IRL" per card** `(iniziativa, azienda)`: documento di lavoro
   del deal (context-scoped come la lettura di tesi), curato dall'analista.
2. **Seminata, non generata**, da quattro fonti + template — ogni voce con
   **provenienza**: (a) domande deterministiche dai flag (A.4, A.5/EBITDA,
   B.8, scarto vendor-CEE, TFR, soci, cash-conversion — ognuna porta la sua
   domanda standard); (b) `ddQuestions`/`redFlags` del brief neutro; (c)
   domande di tesi dalla lettura context-scoped (E); (d) voci manuali
   dell'analista; più **template standard per famiglia di business model**
   (MSP: ricorrente contrattualizzato, churn, SLA; software: ownership IP,
   escrow, licenze; progetto: SAL, contenziosi; ~10-15 voci per famiglia).
   Le domande generate coprono le anomalie, il template la completezza.
3. **Dopo il seed è dell'analista**: edit/add/delete/riordino. Le
   rigenerazioni (dossier aggiornato, tesi cambiata) propongono solo voci
   **additive** — mai sovrascrivere la curatela.
4. **Tracking leggero**: stato per voce (aperta / chiesta / risposta / n.a.).
   Niente allegati, niente data room, niente mail automation.
5. **Export XLSX/clipboard** per advisor/target (riusa la meccanica export
   esistente): l'artefatto deve uscire dal tool.
6. **Memoria istituzionale**: l'IRL persiste con la card e sopravvive
   all'archiviazione — il deal che muore a `offerta` e torna dopo 18 mesi
   riparte da cosa era stato chiesto e risposto.

## 7. Cosa NON fare (igiene anti-sovraingegnerizzazione, trasversale)

- Soglie per-ATECO; multipli regressi su size/growth; DCF/WACC in screening.
- Multiplo aggiustato via classificazione LLM (il qualitativo commenta, non
  calcola).
- Stime di debito lease implicito (solo flag B.8).
- Tesi sull'iniziativa; vincoli strutturati accanto alla tesi.
- Automatismi LLM agganciati alla state machine del board.
- Dedup semantico delle domande DD; data room; workflow di approvazione.
- Numeri inventati dall'LLM ovunque (premi di sinergia in testa): in un tool
  interno il numero diventa verità aziendale in una slide.

## 8. Prerequisiti empirici e decisioni urgenti

1. **Versioning dei payload** (A, livello 1): decidere prima del prossimo
   refresh di massa — i dati sovrascritti non tornano.
2. **Validazione sui payload in cache** (vedi §1): mix divisioni, granularità,
   copertura delta/L2Y, B.12/13, scala grossFinancialDebt. Via query
   dell'utente o endpoint di ispezione read-only.
3. **Soglia scarto vendor-vs-CEE**: calibrare sulla distribuzione osservata,
   non fissare a priori.
4. ~~Fix ROE~~ **RISOLTO senza intervento**: il presunto misgrading era una
   lettura errata dei codici `annualResult` (vedi §1) — il ROE vendor
   riconcilia esattamente su entrambi i payload. Resta, in Fase 1, la
   riconciliazione ROE come sanity check insieme a EBITDA e PFN.

# Binocolo — PRD UI/UX ricerca gated (workstream D)

> Draft progressivo — **iterazione 8 (2026-07-02)**. Contesto: `GATED-PRIMARY-PLAN.md` (workstream D). Ogni punto è marcato **[DECISO]** (ratificato da Salvatore), **[PROPOSTA]** (mia, da validare) o **[APERTO]** (da sciogliere in un'iterazione successiva).

## 1. Inquadramento

Due superfici nuove, pensate per un'esecuzione lunga, asincrona e multi-stadio:

- **D1 — Pagina di ingresso**: nuova pagina dedicata alla creazione di una ricerca gated. Non è un adattamento della TargetPage attuale. **[DECISO]**
- **D2 — Pagina di dettaglio ricerca**: gestione del run (progresso vivo) e dei risultati.

Vincoli trasversali:
- **Zero dati di costo in UI utente** (€/azienda, bande, survivor-rate: valutazione interna esclusiva). Ratificato.
- Design system `docs/UI-UX.md` (clean theme), copy B2B asciutto, niente contatori decorativi.
- Review finale col workflow `portal-miniapp-generator`.
- Destino della TargetPage: **deciso**, strangler in due tempi — vedi §8.

## 2. D1 — Pagina di ingresso

### 2.1 Richiesta in linguaggio naturale **[DECISO]**
- La textarea della richiesta è la protagonista della pagina.
- Guida esplicita all'utente: **esplicitare bene e in dettaglio il perimetro della ricerca**; descrivere gli **ambiti di business** in modo discorsivo; **non usare codici ATECO**.
- **Eliminati** i selettori di modello e versione istruzioni. (Verificato: la pipeline v2 li ignora già — `ModelID`/`PromptID` sono consumati solo dal monolite legacy dietro il kill-switch `ma_strategy_pipeline`; nessuna perdita funzionale. Rimozione **confermata** 2026-07-02.)
- **[PROPOSTA]** 2–3 esempi di richiesta ben formata come placeholder/hint sotto la textarea, per insegnare il livello di dettaglio atteso (il dettaglio nutre direttamente la qualità del gate semantico).

### 2.2 Decomposizione e "perimetro riconosciuto"
- La richiesta viene decomposta in dati strutturati. **Verificato**: pipeline v2 = intent extractor LLM (scope `ma_strategy_intent`, con citazioni `sourceText` obbligatorie per ogni vincolo forte) → canonicalizzazione deterministica (territorio via catalogo province; ATECO via retrieval embedding sulla KB curata, fallback resolver LLM).
- **[DECISO]** L'utente vede i **concetti estratti e sommarizzati** (non i codici): chip/card per concetto con nome e fit (core / adiacente), più un **chevron** che rivela i settori ATECO (divisioni e codici) che verranno usati.
- **[DECISO]** Nessuna possibilità di modificare i codici ATECO in fase 1 (l'utente minerebbe l'impianto: fit mapping, esclusioni KB, superficie espansa).
- **Gap dati (R1) [DECISO: persistere]**: oggi i nomi dei concetti NON sono persistiti sulla strategia — il candidato porta solo `Rationale: "Concetto KB <id> (coseno …)"`. Il preview endpoint (`/test/ateco-retrieval`) restituisce già la forma giusta (`name`, `fit`, `matched`, `divisions`, `atecoCodes`), ma la creazione sessione no. Si persistono i concetti abbinati (id, nome, fit, divisioni) sulla strategy version: il parsing dell'id dal rationale è fragile, la KB evolve nel tempo (il concetto potrebbe cambiare o sparire dopo la creazione), e il trace è write-only per disegno — serve il fatto congelato al momento della decomposizione.
- **Caso limite ATECO espliciti [DECISO]**: se la richiesta contiene codici ATECO, si fa una **reverse lookup sulla KB** e i codici vengono convertiti nei concetti che li includono (match sul fit-mapping `InKB`/sottoalbero) → il pannello concetti resta la vista unica del perimetro. Codici senza concetto corrispondente restano visibili come "codice dichiarato, fuori KB" (R5).

#### 2.2.1 Stati degradati della decomposizione **[DECISO — A, B e C confermati 2026-07-02]**

Tre casi distinti, con trattamenti diversi:

**A — Settore non riconosciuto (sotto-floor, verdetto autorevole). [DECISO: bloccante]** Il retrieval funziona ma nessun concetto supera il floor: la KB non copre l'ambito descritto. È un no-match affidabile, non un errore tecnico. **Stato bloccante**: il pannello perimetro mostra "Ambiti non riconosciuti" e la stima non è lanciabile — pericolo verificato nel codice: con zero candidati la stima degraderebbe alla superficie province-only *senza filtro settoriale* (fallback legacy per strategie sector-less), cioè "tutte le aziende della provincia". Rimedi nel copy: riformulare con termini concreti dell'attività svolta (il retrieval lavora su concetti di business, non su sigle/gergo); e dichiarazione **esplicita** del perimetro di copertura **[DECISO 2026-07-02]**: "il sistema è attualmente configurato per settori ICT e adiacenti". L'"attualmente" lascia spazio alla crescita della KB senza riscrivere il copy.

**B — Perimetro positivo mancante. [DECISO: bloccante]** La richiesta non descrive nessun ambito di attività. Due forme: **solo esclusioni** ("aziende in Lombardia, 1–5 mln, escluse web agency e consulenza" → `include` vuoto, `exclude` popolato) e **solo criteri strutturali** ("aziende a Brescia, 10–50 dipendenti, titolare over 60" → sectors vuoto del tutto). La seconda è insidiosa perché è una richiesta M&A legittima — il vecchio flusso la supportava via superficie province-only — ma il metodo gated non può eseguirla: il gate giudica l'aderenza a un perimetro descritto (senza, non ha metro di giudizio) e la superficie sarebbe indiscriminata, quasi ovunque sopra cap. Stesso stato bloccante di A con messaggi propri: per il caso solo-esclusioni "hai indicato solo cosa escludere"; per il caso strutturale l'invito a esplicitare gli ambiti di business, anche larghi, su cui il gate potrà giudicare. Il tono non è "hai sbagliato" ma "questo metodo ha bisogno di sapere cosa cerchi". Emerge dalla decomposizione, non da validazione client pre-invio.

**C — Fallback LLM (retrieval indisponibile: embedder giù, drift KB). [DECISO: degradazione, non blocco]** Il resolver gerarchico produce comunque candidati validi ma senza concetti. **Degradazione, non blocco**: il pannello mostra il perimetro raggruppato per divisione ATECO (dai candidati, con descrizioni di catalogo) dietro lo stesso chevron, con una nota sobria "perimetro derivato dal catalogo ATECO". La stima e il flusso procedono normalmente — la superficie espansa deriva dalle divisioni come sempre; i codici restano non modificabili. Il fallback è già tracciato internamente (`ma_ateco_retrieval_fallback`) per l'operatore.

- **Requisito (R7)**: nel nuovo flusso una strategia senza candidati settoriali non deve MAI raggiungere la stima (il fallback legacy province-only resta solo per compatibilità del vecchio flusso).
- **Integrazione con R1**: persistere insieme ai concetti anche la **modalità di retrieval** (`embedding` | `fallback_llm` | `explicit_reverse`) così il pannello sa quale variante rendere anche a distanza di tempo.

### 2.3 Territorio **[DECISO]**
- Recap **sintetico**: se le province coprono una regione intera si mostra il nome della regione, non l'elenco; selezioni parziali mostrano le province.
- Piccolo **modal** per aggiungere/eliminare singole province.
- Copy che ricorda: **meglio non allargare troppo** — il metodo effettua uno scan ampio; preferire qualche provincia o al massimo una regione per ricerca.

#### 2.3.1 Guard-rail territorio a monte **[DECISO — confermato 2026-07-02]**

Principio: **a monte si avvisa, a valle si blocca**. Il numero di province è un cattivo proxy della superficie reale — Milano da sola può superare il cap, otto province piccole possono non arrivarci — quindi l'unico blocco legittimo è quello della preview (§2.7.1), fondato sui numeri misurati. Un blocco hard a monte su una soglia di province sarebbe arbitrario in entrambe le direzioni.

- **Avviso soft, non bloccante**, quando la selezione supera **una regione** (province appartenenti a più di una regione, oppure nessun territorio = tutta Italia): notice inline nella sezione territorio, ripetuta accanto al bottone di stima — il perimetro è ampio, la stima sarà più lenta e la ricerca meno mirata; preferire una regione o poche province. Il modal province è il rimedio a un click.
- La stima resta sempre lanciabile: è la preview a dire la verità (stato bloccante "troppo ampio" sui numeri reali).
- **Nota operativa (interna, non-UI)**: il fan-out dei probe di stima cresce col prodotto province × divisioni × forme; oggi non esiste un tetto sul numero di probe per stima. Valutare un ceiling server-side (rifiuto con "perimetro troppo ampio per la stima") come money-safety interna — decisione operatore, fuori dal PRD utente.

### 2.4 Altri vincoli **[DECISO]**
Fatturato, dipendenti, ricavo per dipendente, numero soci, forme giuridiche, età titolare (per successione) restano **editabili come oggi**. Il pattern attuale "aggiungi vincolo" (campi opzionali nascosti finché non valorizzati) è riusabile così com'è.

### 2.5 Tesi d'acquisizione **[DECISO]**
- La tesi di default è **derivata dalla richiesta** (intent extractor → `normalizeMAThesis`, vuoto → `generico`). **Nessun selettore nel form**: il cambio tesi post-esecuzione con rescore esiste già ed è il posto giusto.
- **[PROPOSTA]** Mostrarla come chip informativa read-only nel perimetro riconosciuto ("Tesi: successione — modificabile dopo l'esecuzione"), così la derivazione non è invisibile.

### 2.6 Numero risultati **[DECISO]**
Nessun limite imputabile dall'utente (un taglio arbitrario produrrebbe una ricerca parziale). Il campo "Numero risultati" sparisce; il cap superficie (1000) resta l'unico governor, interno.

### 2.7 Stima e conferma **[DECISO]**
- Bottone unico che produce la stima usando il **solo criterio della ricerca espansa** (superficie = sottoalbero popolato delle divisioni dei concetti core/weak, meno esclusioni). Niente scelta ateco/espansa: la ricerca ATECO sharp è spesso troppo scarna.
- La stima è asincrona (flusso `estimating` esistente); la UI mostra l'attesa senza spinner (skeleton/contatore da design system).
- Dalla preview: **Conferma** (avvia il run gated e naviga a D2) oppure **Rivedi perimetro** (torna al form con i dati mantenuti).
- **Requisito (R2)**: la stima deve girare solo-espansa (endpoint con `strategyType` fissato o filtro lato nuovo flusso; oggi `runEstimates` produce entrambi i gruppi).

#### 2.7.1 Forma della preview **[DECISO — approvata 2026-07-02]**

Fondazione dati (verificata): le stime espanse sono già persistite **una riga per provincia** (`aggregateExpandedEstimates` somma i probe per divisione dentro la provincia) — la ripartizione territoriale è gratis. Il superamento del cap superficie **rifiuta l'avvio up-front senza spesa** (`errMAEstimateTooLarge`): non esiste troncamento silenzioso, quindi la preview può e deve essere il punto in cui il perimetro si restringe.

La preview ha **tre stati mutuamente esclusivi**, decisi dal totale espanso N:

1. **Perimetro eseguibile** (0 < N ≤ cap superficie):
   - **Numero grande**: "N aziende nel perimetro" — il dato primario della decisione.
   - **Ripartizione per provincia**: righe provincia + conteggio, ordinate per volume decrescente, mostrate solo quando le province sono ≥2. È la diagnostica del territorio: rende visibile dove pesa la ricerca ("Milano 420 di 610") e supporta la guidance anti-allargamento. Azione contestuale sulla lista → apre lo stesso modal province di §2.3 (la leva è a un click dal dato). Province a conteggio zero mostrate in coda, non nascoste.
   - CTA: **Conferma e avvia** (primaria) / **Rivedi perimetro** (secondaria). Sotto la primaria, una riga di aspettativa onesta: l'analisi è asincrona, l'avanzamento si segue nella pagina della ricerca.
2. **Perimetro troppo ampio** (N > cap superficie, o `too_broad` dal vendor): **stato bloccante** — la conferma non è disponibile, coerente col rifiuto up-front del backend. Il numero mostra "più di N" quando è lower bound (`too_broad` con probe multipli). Copy: il perimetro supera la capacità di una ricerca completa; restringere il territorio o gli ambiti di attività. Qui la ripartizione per provincia è LA leva: mostra dove tagliare.
3. **Perimetro vuoto** (N = 0): nessuna azienda trovata; rimedi indicati — riformulare gli ambiti di attività o allargare il territorio. (Distinto dal caso "settore non riconosciuto dalla KB", che emerge già alla decomposizione, prima della stima — §6.)

Cosa la preview **non mostra** (vincoli già ratificati):
- **Costi**: mai.
- **Ripartizione per divisione/concetto**: la leva sui codici non esiste per l'utente in fase 1 — mostrare il volume per divisione inviterebbe a voler potare codici senza avere il rimedio (frustrazione senza leva). La qualità settoriale del perimetro si giudica nel pannello concetti, non nella stima. Da rivalutare solo se una fase 2 aprirà l'editing dei fit.
- Matrice province×codici×forme, probe count, forme giuridiche: dettaglio interno.

**Freshness** (già imposta dal backend: avvio con stima stantia → "stale estimate"): ogni modifica al perimetro dopo la stima invalida la preview — la UI torna allo stato "Calcola stima" e la conferma si disabilita. Nessuna conferma su numeri che non corrispondono più al form.

- **Requisito (R6)**: nel nuovo flusso il limite interno di esecuzione = cap superficie, NON il default `searchLimit` 100 — il gated ammette `min(limit, stimato)`: ereditare il default significherebbe troncare silenziosamente a 100 proprio ciò che si è deciso di non troncare.

## 3. D2 — Pagina di dettaglio ricerca (`/ricerche/:id`)

Una sola pagina, due modalità decise dallo stato del run: **in corso** (il progresso è il protagonista) e **completato** (il lavoro sui risultati è il protagonista, il progresso collassa a riepilogo). Header comune: titolo ricerca, stato, tesi attiva con cambio+rescore (funzione esistente), azioni export / web-enrich / deep-dive come oggi.

### 3.1 Run in corso **[DECISO: feedback costante, effetto wow]**
- Rappresentazione ricca del progresso: **funnel** per stadi — `surface/address → gate semantico → advanced + scoring` — con **valori e stati di ogni singola attività**.
- Per stadio: stato (in attesa / in corso / completato), contatore vivo (aziende processate / totale dello stadio).
- Contatori di bucket in tempo reale: keep / forse / scarta / manual_review che si popolano mentre il gate lavora.
- **[PROPOSTA]** Visual: funnel orizzontale con larghezze proporzionali ai sopravvissuti per stadio + rail verticale delle attività con micro-animazioni clean theme (rowEnter/sectionEnter, shimmer sui contatori in aggiornamento). Aspettative oneste sul tempo ("richiederà del tempo"), nessuna percentuale inventata. Fallimento onesto: `errorCode` del run → messaggio chiaro + azione (riprova / contatta operatore), mai un funnel congelato senza spiegazione.

#### 3.1.1 Modello del progresso **[DECISO 2026-07-02 — fondazione verificata nel codice]**
Il job gated persiste incrementalmente tutto ciò che serve: i target dopo l'address (superficie), le web validation per azienda durante il gate (con `final_action` → bucket), la marcatura advanced per azienda durante l'enrich. **L'endpoint R3 è pura aggregazione read-only** — nessuna scrittura nuova, nessuna lettura dei trace (write-only per dottrina):

```
GET /binocolo/v1/ma/sessions/{id}/gated-progress
{ "stage": "address" | "gate" | "enrich" | "ready" | "failed",
  "surface": { "expected": N, "fetched": N },
  "gate":    { "processed": N, "total": N,
               "buckets": { "keep": N, "forse": N, "scarta": N, "manualReview": N } },
  "enrich":  { "enriched": N, "survivors": N },
  "run":     { "startedAt": ..., "completedAt": ..., "errorCode": "" } }
```
Lo stadio si inferisce dagli stati dei dati (target presenti? validation fresche < totale? advanced < sopravvissuti?). Polling UI ogni pochi secondi solo mentre `running`.

#### 3.1.2 Coda consultabile, non lavorabile, durante il run **[DECISO 2026-07-02]**
Le righe manual_review si accumulano già durante il gate e la coda è **visibile subito** (trasparenza, l'operatore vede il lavoro che lo aspetta). I **rimedi si attivano solo a run fermo**: vincolo tecnico verificato — lo stadio enrich legge i target a inizio stadio e fa `ReplaceMATargets` alla fine, quindi un rimedio applicato mid-run verrebbe sovrascritto (race). Copy sobrio sulla riga: "disponibile a ricerca completata".

### 3.2 Run completato — organizzazione dei risultati **[DECISO 2026-07-02]**
Il funnel collassa in un **riepilogo compatto** in testa (numeri finali per stadio: superficie → gate → arricchite — il percorso resta leggibile e ri-espandibile). Sotto, tre superfici:

1. **Risultati** — la shortlist scorata coi bucket del routing v3 (azionabile / da verificare / soppresso), tabella + drawer dettaglio, stelle/esiti, dossier: le funzioni restano "come oggi" (decisione già presa), cambia solo la casa.
2. **Coda di verifica** (badge col conteggio) — le aziende identity-only trattenute dal gate: manual_review + i keep/forse mai arricchiti per fetch fallito. È la §3.3.
3. **Scarti** — audit del gate (recall-safety): le scartate con verdetto, evidenza e motivo ispezionabili; da qui l'unico rimedio è il ripescaggio via associa-dominio se il dominio giudicato era sbagliato.

### 3.3 Coda di verifica **[DECISO 2026-07-02]**
Coda di lavoro di prima classe, non un cassetto. Per ogni riga:
- **Motivo derivato dai fatti persistiti (R9)**, non testo generico: "possibile sito di gruppo: horsa.com (P.IVA di altra società)" da `MAGroupSiteHint`; "nessun candidato web trovato" (0 candidati); "identità non confermata in pagina" (candidati rigettati); "sito irraggiungibile" quando derivabile.
- **Rimedi contestuali al motivo**, ciascuno = job durevole già esistente o variante: **Associa dominio** (esistente); **Conferma sito di gruppo** (B1 — visibile solo con hint presente: registra `vouched` + flag group-site, il gate giudica sul contenuto del gruppo, evidenza tracciata group-level); **Nessun sito ufficiale** (C — §3.4).
- Dopo il rimedio la riga mostra lo stato del re-processo (in coda / in corso / esito) ed esce dalla coda verso il suo bucket: il funnel della singola azienda si completa da solo.
- **Cross-sessione**: ogni rimedio scrive nel registro domini — le ricerche future non ripresenteranno la stessa azienda in coda per lo stesso motivo.

### 3.4 "Nessun sito ufficiale" (workstream C) **[DECISO 2026-07-02, inclusa la spesa]**
- Effetto del click: dichiarazione **durevole e cross-sessione** (registro, `method='no_website'` o stato dedicato — dettaglio impl.); il gate si dichiara non applicabile (`no_signal`, mai `scarta`: assenza di sito ≠ fuori tesi); l'azienda prosegue sui soli dati strutturati, marcata "nessuna evidenza web" ovunque compaia.
- **Decisione di spesa [DECISO]**: la conferma operatore **ammette anche l'Advanced** — un click che dichiara e ammette insieme (l'azione è già manuale e deliberata). Scartata l'alternativa dell'Advanced automatico su tutti i senza-sito.

### Requisiti D2 aggiuntivi
| ID | Requisito | Stato |
|----|-----------|-------|
| R8 | Endpoint progresso read-only (forma §3.1.1) | DECISO |
| R9 | Derivazione del motivo-riga dai fatti persistiti (hint gruppo, candidati, identità) | DECISO |
| R10 | Rimedi attivi solo a run fermo (race con `ReplaceMATargets` dello stadio enrich) | DECISO |
| R11 | Persistenza durevole del verdetto "nessun sito" (registro `no_website` o stato dedicato) | DECISO |

## 4. Requisiti dati/API emersi

| ID | Requisito | Stato | Perché |
|----|-----------|-------|--------|
| R1 | Persistere i concetti KB abbinati (id, nome, fit, divisioni) sulla strategy version | DECISO | Il pannello "perimetro riconosciuto" di D1 li mostra; oggi esiste solo nel preview/test endpoint; KB evolve, trace write-only |
| R2 | Stima solo-espansa per il nuovo flusso | DECISO (implicito in §2.7) | D1 non offre la scelta di strategia |
| R3 | Nuovo endpoint di progresso per stadio del job gated (stato+contatori+bucket) | DECISO | Il funnel vivo di D2 |
| R4 | Recap territorio regione-compatta (province → regioni coperte) | — | Può essere derivato client-side dal catalogo province condiviso (`@mrsmith/ui` esporta `provinces`) |
| R5 | Reverse lookup KB per codici ATECO espliciti (codice → concetti che lo includono) | DECISO | Il pannello concetti resta la vista unica anche quando l'utente incolla codici |
| R6 | Limite interno di esecuzione = cap superficie nel nuovo flusso (non il default `searchLimit` 100) | DECISO (§2.7.1 approvata) | Il gated ammette `min(limit, stimato)`: il default 100 troncherebbe silenziosamente |
| R7 | Strategia senza candidati settoriali → stima non lanciabile nel nuovo flusso | DECISO (§2.2.1) | Il fallback legacy degraderebbe a superficie province-only senza filtro settoriale |
| R1b | Persistere con i concetti anche la modalità di retrieval (`embedding`/`fallback_llm`/`explicit_reverse`) | DECISO (§2.2.1) | Il pannello perimetro deve sapere quale variante rendere, anche a distanza di tempo |

## 5. Prompt intent extractor — v2 IMPLEMENTATO (2026-07-02)

Le sei migliorie individuate nell'iterazione 1 sono implementate in **migrazione 084** (`M&A strategy intent extractor v2 NL-first`, promosso default; v1 e "v1 thesis guard" restano nel registry per rollback via `is_default`, senza deploy):
1. Regola unità euro-interi con esempi di conversione (mln/milioni/M/k); il campo `Unit` del vincolo numerico era già ignorato dal codice.
2. Campo `title` nell'output shape (il codice consumava già `intent.Title` ma non arrivava mai).
3. NL-first: `sectors.summary` distillato (solo perimetro positivo, ≤60 parole) + enumerazioni spezzate + entry auto-contenute. **Codice a corredo**: `MAIntentSectors.Summary`, preferito da `buildMAEmbeddingQuery` (query di retrieval, sopravvive alle richieste discorsive) e da `maIntentSectorDescription` (sector description → gate UC2); fallback ai frammenti coi prompt precedenti.
4. Regola tesi riscritta: segnali espliciti per tesi + tie-break su `""` (→ generico); mai derivare da settore/dimensione/territorio.
5. Semantica delle dispositions (`deep`/`unsupported`/`missing_confirmation`) — alimentano i `missingCriteria` in preview.
6. Valori `status` enumerati (set chiuso di `normalizeCompanyActivityStatus`).

**Validazione (in luogo di A/B live)**: un A/B non conviene — volume basso e operator-driven, la v2 di `extractIntent` risolve sempre il prompt default (nessun override per-richiesta su cui splittare), e il giudizio è qualitativo. Approccio scelto: **replay mirato** — dopo l'applicazione della 084, ricreare 3-4 sessioni di prova con le richieste reali già memorizzate (`ma_session.prompt`) e confrontare il perimetro riconosciuto (concetti, vincoli, tesi, missing) col vecchio esito; rollback = flip di `is_default` sul registry. ⚠️ Prima di applicare: confermare quale prompt è default live (seed ≠ live possibile — mig 052 seedava "thesis guard" NON-default).

## 6. Criterio di selezione allargata — valutazione (richiesta 2026-07-02)

Il criterio è a **due strati**, entrambi verificati nel codice:
1. **Retrieval concetti** (UC1): embedding della parte positiva dell'intent → coseno sui concetti KB; soglie **relative** al top (rel 0.65, core-ratio 0.92, floor assoluto 0.45, cap 12 concetti — migs 078/079). I concetti adiacenti entrano come fit `weak`; le esclusioni KB pesano solo se non core-included.
2. **Superficie espansa**: dalle **divisioni a 2 cifre** dei candidati core/weak → probe del sottoalbero popolato (dry-run, cap sonde), meno i sottoalberi esclusi.

Valutazione per la nuova pagina: il criterio è adeguato come **unico** criterio di ricerca — la larghezza (divisioni intere) è esattamente ciò che compensa la scarsità dell'ATECO sharp, e il gate semantico a valle è il contenimento del rumore introdotto. I punti d'attenzione non sono algoritmici ma di trasparenza UI: (a) i concetti mostrati devono essere quelli reali del retrieval (R1), (b) il caso sotto-floor ("Settore non riconosciuto dalla KB") deve avere un percorso UX chiaro in D1 — messaggio che invita a riformulare la descrizione, non un errore tecnico. **[APERTO]** cosa mostrare quando il retrieval degrada al fallback LLM (concetti assenti, solo codici).

> **Implementazione**: piano esecutivo in [`GATED-UX-IMPLEMENTATION-PLAN.md`](GATED-UX-IMPLEMENTATION-PLAN.md) — 13 task (B1–B8 backend, F1–F5 frontend) con riferimenti verificati, pensati per esecuzione da parte di un LLM. Wireframe D1+D2 approvati (2026-07-02): [`gated-ux-wireframe.html`](gated-ux-wireframe.html), stati S1–S10 — fonte di verità per layout e copy dei task F.

## 7. Aperti per la prossima iterazione

1. ~~Destino TargetPage~~ → **deciso**, §8 (strangler in due tempi).
2. ~~Forma della preview di stima~~ → **decisa**, §2.7.1.
3. ~~Guard-rail territorio~~ → **deciso**, §2.3.1 (avviso soft oltre una regione; blocco solo in preview).
4. ~~UX sotto-floor / fallback LLM~~ → **deciso**, §2.2.1 (A/B bloccanti, C degradato); copertura KB dichiarata esplicita: "attualmente configurato per settori ICT e adiacenti".
5. ~~D2 risultati + coda manual_review~~ → **deciso**, §3 (iterazione 7), inclusa la spesa del senza-sito (conferma operatore ammette Advanced).
6. ~~Wireframe D1~~ → **approvato** (iterazione 6). ~~Wireframe D2~~ → **approvato** (iterazione 8). Versionati nel repo: [`gated-ux-wireframe.html`](gated-ux-wireframe.html) (S1–S6 = D1, S7–S10 = D2).

## 8. Destino della TargetPage **[DECISO — approvato 2026-07-02]**

Stato di fatto (verificato): la TargetPage (`/target`, ~3.600 righe) impacchetta **cinque responsabilità** — (1) elenco/navigazione sessioni, (2) creazione (prompt NL + selettori LLM + editor strategia), (3) stima con scelta ateco/espansa ed etichette di costo, (4) avvio del **classic execute** (`/execute`) e monitoraggio grezzo via polling dello status, (5) lavorazione risultati (tabella target, dettaglio, rescore per tesi, stelle/esiti, deep dive, web enrichment, export CSV/XLSX). Il funnel gated **non ha oggi alcuna UI di produzione**: l'endpoint `/gated-search` non è chiamato da nessuna pagina; solo la TestPage lo esercita (inline dev). La TargetPage è quindi la casa del flusso pre-gated, non un'antenata da adattare.

Mappatura verso le nuove superfici: (2)+(3)+(4-avvio) → D1 (senza selettori, senza scelta strategia, senza costi); (4-monitoraggio) → D2 progress; (5) → D2 risultati (le azioni restano "come oggi" da piano: rescore, stelle, deep dive, export); (1) → nuova rotta indice `/ricerche`. L'unico contenuto che NON migra è la scelta ateco/espansa con le etichette di costo: superficie operatore, il cui posto naturale è la TestPage (dove il confronto strategie già vive).

**Proposta: sostituzione strangler in due tempi, niente console operatore permanente.**
- **Tempo 1** (D1 + D2 pronti con progress + risultati base): nuove rotte `/ricerche` (indice), `/ricerche/nuova` (D1), `/ricerche/:id` (D2); il default dell'app passa da `/target` a `/ricerche`. La TargetPage resta raggiungibile solo via URL diretto come superficie legacy in sola consultazione delle sessioni pre-gated; la creazione al suo interno si disabilita (vietato avere due punti d'ingresso concorrenti).
- **Tempo 2** (D2 completo di coda manual_review, post workstream B/C): TargetPage **rimossa**, rotta `/target` → redirect a `/ricerche`. Le sessioni legacy si aprono in `/ricerche/:id`: stesse tabelle (`ma_session`/`ma_target`), la vista risultati le rende già; per le sessioni senza run gated la sezione progress non compare. Il classic execute non è più raggiungibile da UI (il codice resta finché non si decide un cleanup separato) — coerente con "gated È la direzione".

Perché non tenerla come console operatore: doppia manutenzione del flusso più complesso dell'app con drift garantito su ogni evoluzione del modello dati; le superfici operatore esistono già (TestPage per probe/gated/confronti, ConfigPage per parametri); i dati di costo per la valutazione interna restano disponibili via TestPage e API.

Non toccate dal destino della TargetPage: `/azienda` (CompanyDossierPage, che D2 continuerà a linkare), `/ricerca-web`, `/config`, `/test`.

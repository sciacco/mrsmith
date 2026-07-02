# Binocolo — PRD UI/UX ricerca gated (workstream D)

> Draft progressivo — **iterazione 2 (2026-07-02)**. Contesto: `GATED-PRIMARY-PLAN.md` (workstream D). Ogni punto è marcato **[DECISO]** (ratificato da Salvatore), **[PROPOSTA]** (mia, da validare) o **[APERTO]** (da sciogliere in un'iterazione successiva).

## 1. Inquadramento

Due superfici nuove, pensate per un'esecuzione lunga, asincrona e multi-stadio:

- **D1 — Pagina di ingresso**: nuova pagina dedicata alla creazione di una ricerca gated. Non è un adattamento della TargetPage attuale. **[DECISO]**
- **D2 — Pagina di dettaglio ricerca**: gestione del run (progresso vivo) e dei risultati.

Vincoli trasversali:
- **Zero dati di costo in UI utente** (€/azienda, bande, survivor-rate: valutazione interna esclusiva). Ratificato.
- Design system `docs/UI-UX.md` (clean theme), copy B2B asciutto, niente contatori decorativi.
- Review finale col workflow `portal-miniapp-generator`.
- **[APERTO]** Destino della TargetPage attuale: convivenza temporanea (sessioni esistenti) o deprecazione con redirect. Da decidere quando D1/D2 sono definite.

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

### 2.3 Territorio **[DECISO]**
- Recap **sintetico**: se le province coprono una regione intera si mostra il nome della regione, non l'elenco; selezioni parziali mostrano le province.
- Piccolo **modal** per aggiungere/eliminare singole province.
- Copy che ricorda: **meglio non allargare troppo** — il metodo effettua uno scan ampio; preferire qualche provincia o al massimo una regione per ricerca.
- **[APERTO]** Solo copy o anche un guard-rail (es. avviso oltre N province)? Nessun vincolo hard oggi lato backend.

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
- Preview = **N aziende attese** (mai costi). **[APERTO]** Ripartizione della preview (per provincia? per divisione?) o solo numero secco.
- Dalla preview: **Conferma** (avvia il run gated e naviga a D2) oppure **Rivedi perimetro** (torna al form con i dati mantenuti).
- **Requisito (R2)**: la stima deve girare solo-espansa (endpoint con `strategyType` fissato o filtro lato nuovo flusso; oggi `runEstimates` produce entrambi i gruppi).

## 3. D2 — Pagina di dettaglio ricerca

### 3.1 Run in corso **[DECISO: feedback costante, effetto wow]**
- Rappresentazione ricca del progresso: **funnel** (o progressione lineare) per stadi — `surface → address → gate semantico → advanced + scoring` — con **valori e stati di ogni singola attività**.
- Per stadio: stato (in attesa / in corso / completato), contatore vivo (aziende processate / totale dello stadio).
- Contatori di bucket in tempo reale: keep / forse / scarta / manual_review che si popolano mentre il gate lavora.
- **[PROPOSTA]** Visual: funnel orizzontale con larghezze proporzionali ai sopravvissuti per stadio + rail verticale delle attività con micro-animazioni clean theme (rowEnter/sectionEnter, shimmer sui contatori in aggiornamento). Aspettative oneste sul tempo ("richiederà del tempo"), nessuna percentuale inventata.
- **Gap dati (R3) [DECISO: endpoint dedicato]**: oggi la UI vede solo `session.status` (`estimating`/`running`) e polla il session detail. Si aggiunge un **nuovo endpoint di progresso** (stato + contatori + timestamp per stadio, bucket vivi), derivato dallo stato durevole del job `gated_search` — separato dal session detail, che resta leggero.

### 3.2 Risultati e code di lavoro **[da dettagliare — iterazione dedicata]**
- Shortlist scorata con i bucket del routing v3, esiti/stelle come oggi.
- **Manual_review come coda di lavoro di prima classe**: motivo del trattenimento per riga + rimedi contestuali (associa dominio, "nessun sito" — workstream C, conferma sito di gruppo — workstream B). Da progettare quando B è deciso.
- Scarti ispezionabili per audit (recall-safety).

## 4. Requisiti dati/API emersi

| ID | Requisito | Stato | Perché |
|----|-----------|-------|--------|
| R1 | Persistere i concetti KB abbinati (id, nome, fit, divisioni) sulla strategy version | DECISO | Il pannello "perimetro riconosciuto" di D1 li mostra; oggi esiste solo nel preview/test endpoint; KB evolve, trace write-only |
| R2 | Stima solo-espansa per il nuovo flusso | DECISO (implicito in §2.7) | D1 non offre la scelta di strategia |
| R3 | Nuovo endpoint di progresso per stadio del job gated (stato+contatori+bucket) | DECISO | Il funnel vivo di D2 |
| R4 | Recap territorio regione-compatta (province → regioni coperte) | — | Può essere derivato client-side dal catalogo province condiviso (`@mrsmith/ui` esporta `provinces`) |
| R5 | Reverse lookup KB per codici ATECO espliciti (codice → concetti che lo includono) | DECISO | Il pannello concetti resta la vista unica anche quando l'utente incolla codici |

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

## 7. Aperti per la prossima iterazione

1. Destino TargetPage (convivenza vs deprecazione) e naming rotte (`/ricerche/nuova`, `/ricerche/:id`?).
2. Forma della preview di stima (numero secco vs ripartizione).
3. Guard-rail territorio (solo copy o avviso oltre soglia).
4. UX del caso sotto-floor / fallback LLM (§6).
5. D2 risultati + coda manual_review (dopo decisione workstream B).
6. Wireframe D1/D2 su questa base.

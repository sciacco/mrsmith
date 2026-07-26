# Cutover del registro identità azienda — runbook

Issue [#86](https://github.com/sciacco/mrsmith/issues/86), sub-issue di #81.

Il lavoro è **adottare, non migrare**: le `company_key` esistenti diventano
identificatori nostri, opachi e mai più ricalcolati; le aziende nuove ricevono un
UUID. Nessuna chiave cambia, quindi nessun URL si rompe e nessuna delle ~15
tabelle chiavate su `company_key` va rimappata.

L'esecuzione avviene **ad applicazione e worker fermi**. Il runbook è
**ri-entrante**: ogni passo si può rieseguire.

## Sequenza

```
0.5 migrazione 122                  (fatta il 2026-07-26, Q16 ora vuota)
1.  stop writer (applicazione + worker)
2. migrazione 120  — schema, registro, colonna ma_target.company_key nullable
3. migrazione 121  — backfill di ma_target.company_key (ri-eseguibile)
4. GATE            — sonda Q17: zero righe senza chiave, zero chiavi orfane
5. deploy del binario nuovo
6. smoke (sotto)
7. riapertura scritture
```

### Il commit di compatibilità, e quando serve davvero

`ListMATargetRows` costruiva una CTE con `SELECT t.*` più una `company_key`
**derivata con lo stesso nome**. Finché la colonna non esiste il nome è unico;
nell'istante in cui la 120 la aggiunge, la CTE ne espone due omonime e ogni
`t.company_key` successivo diventa ambiguo — PostgreSQL risponde `column
reference "company_key" is ambiguous` e l'elenco dei target smette di
funzionare. Non è un problema del binario nuovo, che la colonna la legge: è un
problema del binario **già in esercizio** con lo schema nuovo.

`17f1005` — il **primo commit** del ramo — sostituisce `t.*` con l'elenco
esplicito e funziona su **entrambi** gli schemi, senza cambiare comportamento.

**Con l'applicazione ferma per tutto il cutover quel problema non si presenta**:
nessuno serve richieste fra la migrazione e il deploy. Il commit resta comunque
il primo del ramo, isolato e cherry-pickabile, perché serve in due casi:

- vuoi **validare la build nuova contro il DB di produzione mentre l'app gira**,
  che è il flusso descritto in `docs/DATABASE-MIGRATIONS.md`;
- **abortisci** dopo la 120 e riaccendi il binario vecchio (vedi rollback 1).

In entrambi i casi: `git cherry-pick 17f1005` su main, deploy, e poi si procede.

Le migrazioni le applica l'utente con il proprio processo: nessuna operazione
diretta sui DB configurati in env.

### Prima di partire

Eseguire **Q16** della sonda (`IDENTITA-AZIENDA-PROBE.sql`). Deve essere vuota:
è la stessa asserzione su cui la migrazione 120 solleva, e sapere in anticipo se
passa evita di scoprire il problema a metà cutover. Se non è vuota, un
identificatore rivendica due aziende e serve arbitraggio umano prima di
proseguire.

La sonda **non si esegue in blocco**: Q15, Q17 e Q18 leggono oggetti che la 120
deve ancora creare e prima falliscono con «does not exist». Prima del cutover si
eseguono solo **Q16**, **Q19** e **Q4** (più le Q1–Q14, che girano su qualunque
schema).

Eseguire quindi anche **Q19**, e **Q4** il cui esito non è mai stato riportato
nella baseline. C'è una causa nota e concreta per cui Q16 può non essere vuota: lo
strumento standalone `/azienda` accodava il dossier con la **P.IVA come
`company_key`**. Se la stessa azienda ha un dossier sotto la P.IVA e un target
sotto l'ObjectId, quel valore fiscale rivendica due chiavi e la 120 si ferma.

### Esito misurato il 2026-07-26 — serve il passo 0.5

Le tre query sono state eseguite sull'Anisetta e il caso **esiste**:

- **Q16**: 4 identità fiscali con due chiavi ciascuna (la P.IVA e un ObjectId).
  Così com'è, la 120 **si ferma**.
- **Q19**: 6 chiavi a forma di P.IVA in `ma_deep_analysis` e
  `ma_deep_payload_vintage`, 3 in `ma_company_bm_family`. **Zero** in target,
  card, rating, esiti, note, IRL, domini, web validation.
- **Q4**: `identita_duplicate = 0`, `euro_sprecati = 0`.

Le tre insieme dicono che **non è la fusione rimandata dalla issue**. Quella
presuppone storia su entrambi i lati — card, voti, esiti, URL navigati — e va
progettata. Qui la chiave sbagliata è usata solo da righe di cache, la chiave
giusta non ha un dossier concorrente, e non c'è nulla dell'analista da
riconciliare. **Si sposta, non si fonde**: è la migrazione 122.

La 122 si applica **prima** della 120 (il numero è più alto solo perché 120 e
121 sono già citate ovunque per numero). Non decide nulla alla cieca: solleva se
la destinazione ha già un dossier, se una chiave avrebbe due destinazioni, o se
la chiave sorgente porta storia fuori dalle tre tabelle di cache — ognuno di
quei casi È una fusione vera, e allora il cutover si ferma davvero.

Dopo la 122, **rieseguire Q16**: deve essere vuota. Solo allora si applica la 120.

Le due chiavi fiscali che non collidono con nulla (aziende viste solo da
`/azienda`, mai in una ricerca) restano come sono: la chiave è opaca, e se un
domani compariranno in una ricerca il resolver le ritroverà dal loro
identificatore fiscale. Q19 continuerà a contarle, correttamente.

### Dopo la 120

Eseguire **Q15**: verifica che le funzioni SQL `binocolo.ma_normalize_fiscal` e
`binocolo.ma_stable_vat` producano esattamente ciò che producevano le
espressioni scritte a mano. Tutte le colonne di divergenza devono dare 0.

### Il gate (passo 4)

Eseguire **Q17**. `orfane` deve essere 0 su ogni riga; `senza_chiave` deve
essere 0 dove `chiave_obbligatoria`. L'unica riga con chiave facoltativa è
`ma_filing_acquisition`, il cui `context_company_key` è un riferimento
contestuale nullable per contratto: un'acquisizione senza contesto è normale e
non deve far fallire il cutover.

La 121 solleva già da sola sui target, ma il gate va verificato anche a mano su
tutte le tabelle: è la condizione che rende raggiungibili le guardie
applicative, che dopo il cutover **errano** invece di riderivare la chiave.

## Tre confini di rollback, distinti

1. **Schema** — la 120 è additiva e non elimina nulla, quindi un rollback
   applicativo non richiede mai il ripristino di schema. Con una riserva: il
   binario **precedente a `17f1005`** non sopravvive alla colonna nuova, perché
   la sua CTE diventa ambigua. Se abortisci dopo la 120 e devi riaccendere il
   vecchio, o gli porti sopra `17f1005` (cherry-pick + deploy) o elimini la
   colonna.
2. **Semantico** — la prima azienda assegnata con UUID. Prima, il binario vecchio
   è sicuro; dopo, ri-deriva l'ObjectId per quell'azienda e ne crea una seconda
   identità.
3. **Scritture** — `ma_target.company_key` è **nullable** proprio per non
   spostare questo confine: con `NOT NULL` il binario vecchio fallirebbe ogni
   `INSERT` di target, non avendo la colonna nella propria lista.

### Se si torna indietro e si riprova

Il binario vecchio, rimesso in servizio prima del confine semantico, crea target
con `company_key IS NULL`. Al tentativo successivo: **rieseguire la migrazione
121 a writer fermi** prima del deploy. È idempotente (`UPDATE … WHERE
company_key IS NULL`) e registra da sola le identità comparse nel frattempo.

## Smoke (passo 6) — lo esegue l'utente

Scrivono tutti su un DB condiviso, quindi non li esegue un agent.

1. **Ricerca nuova, azienda mai vista** → il target deve nascere con una
   `company_key` UUID, e la stessa chiave deve tornare in `ma_target`,
   `ma_target_rating` e `ma_target_web_validation`. È il round-trip che
   l'assenza della colonna avrebbe rotto.
2. **Ricerca su una sessione esistente** (rescore) → nessuna chiave cambia,
   nessuna azienda nuova compare in `ma_company` con `origin = 'assigned'`.
3. **Aggiunta manuale di una P.IVA già nel corpus** → nessuna entità nuova,
   l'eventuale vendor id nuovo si aggiunge come identificatore alla stessa
   azienda (attach-on-found).
4. **Aggiunta manuale della stessa P.IVA due volte** → la seconda deve dare
   «target già presente», ora deciso sulla chiave risolta e non più sui campi
   grezzi.
5. **Card diretta da P.IVA non in cache** → l'azienda nasce nel registro e la
   card la usa.
6. **Scheda azienda** di una chiave storica → deve mostrare la stessa storia di
   prima (nessuna chiave è cambiata).

## Dopo il cutover

**Q18** è il monitor periodico. Devono restare 0 le prime **cinque** righe,
compresa «aziende senza alcun identificatore»: il resolver non ne crea mai — erra
invece di inventare un'identità da una ragione sociale — quindi un valore
maggiore di zero può venire solo dal backfill, da chiavi storiche presenti
unicamente nelle tabelle di dettaglio. Il controllo sulle chiavi che non
risolvono copre tutte le tabelle chiavate, non i soli target: finché F5 non
introduce le FK verso `ma_company`, nulla impedisce a un writer di coniare una
chiave fuori dal registro.

`aziende_vendor_only > 0` non è un errore di integrità ma un difetto da chiudere:
un'entità senza identità fiscale è precisamente ciò che non sopravvive a un
cambio fornitore.

**Q3/Q11 cambiano ruolo**: due vendor id per la stessa identità non sono più un
invariante rotto, sono deriva del fornitore. Restano come sensore di
rinumerazione OpenAPI.it — un alert, non un errore — perché la continuità è
integra finché entrambi puntano alla stessa `ma_company`.

## Rimasto fuori (F5, lavoro separato)

- `NOT NULL` su `ma_target.company_key`.
- Integrità referenziale dalle tabelle di dettaglio verso `ma_company`.
- Fusione di due aziende (`merged_into`): nessun duplicato osservato, e una
  colonna che non gestisce cicli, catene, storia sulla chiave sorgente e
  navigazione dell'URL vecchio sarebbe una promessa che il codice non mantiene.

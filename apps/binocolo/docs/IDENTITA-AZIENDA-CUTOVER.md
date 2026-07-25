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
1. stop writer (applicazione + worker)
2. migrazione 120  — schema, registro, colonna ma_target.company_key nullable
3. migrazione 121  — backfill di ma_target.company_key (ri-eseguibile)
4. GATE            — sonda Q17: zero righe senza chiave, zero chiavi orfane
5. deploy del binario nuovo
6. smoke (sotto)
7. riapertura scritture
```

Le migrazioni le applica l'utente con il proprio processo: nessuna operazione
diretta sui DB configurati in env.

### Prima di partire

Eseguire **Q16** della sonda (`IDENTITA-AZIENDA-PROBE.sql`). Deve essere vuota:
è la stessa asserzione su cui la migrazione 120 solleva, e sapere in anticipo se
passa evita di scoprire il problema a metà cutover. Se non è vuota, un
identificatore rivendica due aziende e serve arbitraggio umano prima di
proseguire.

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

1. **Schema** — la 120 è additiva. Un rollback applicativo non richiede mai il
   ripristino di schema eliminato.
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

**Q18** è il monitor periodico. Le prime tre righe devono restare 0.
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

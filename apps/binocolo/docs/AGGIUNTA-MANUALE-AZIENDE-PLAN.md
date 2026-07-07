# Binocolo — Piano "Aggiunta manuale aziende" (P.IVA + dominio dentro una ricerca)

> Esegue le decisioni del brainstorming 2026-07-07 (terzo tema). Ogni task è
> pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore.
> Riferimenti a simboli/file **verificati sul codice al 2026-07-07** (branch
> `wip/binocolo`, HEAD `8d0e33d`): usarli, non inventarne. Se un simbolo
> citato non esiste più, fermarsi e segnalarlo. Presuppone concettualmente
> `INIZIATIVA-RADICE-PLAN.md` (ogni ricerca vive in un'iniziativa) e
> `SGANCIO-FUNZIONI-AZIENDA-PLAN.md` (deep/thesis-reading per azienda); non
> ne dipende tecnicamente salvo dove indicato.

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **Gli analisti hanno già aziende da inserire**: l'inserimento manuale
   avviene **dentro una ricerca** con P.IVA (obbligatoria) + dominio
   (raccomandato, opzionale). Una ricerca mai eseguita è un contenitore
   legittimo (watchlist con tesi): la sessione dà strategia per lo scoring,
   tesi per la thesis-reading, iniziativa per le card.
2. **Il gate gira ma non boccia**: la validazione web viene eseguita (il suo
   verdetto è informazione: "per la tua tesi questa fa rivendita"), ma
   un'azienda inserita esplicitamente non è mai soppressa dal gate. Decide
   l'analista (stella / esclusione con motivo, strumenti esistenti).
3. **Provenienza marcata**: il target manuale porta `origin = manual` —
   fuori dai contatori del funnel (superficie/gate/enrich), escluso
   dall'eval del gate (survivor-rate, sector-eval), dichiarato in UI
   («Inserita manualmente»).
4. **Dominio fornito → registro**: il dominio dell'analista si persiste in
   `binocolo.ma_company_domain` come associazione confermata (pattern del
   rimedio associate-domain, mig 082): le ricerche future risolvono gratis.
   Senza dominio: risoluzione automatica standard, incluso l'eventuale esito
   manual_review (flusso esistente).
5. **Nessun condizionamento al costo**: l'inserimento singolo (advanced
   ~€0.10 + gate ~€0.02) è un'azione a un gesto, senza conferme di costo
   (regola generale: costi mai in UI utente).
6. **`/azienda` è un tampone in pensionamento**: questa feature lo
   sostituisce per lo spot lookup dentro il flusso M&A. La rimozione della
   pagina è fuori perimetro (dopo l'adozione).

## Il disegno (verificato sul codice)

Il percorso single-company esiste già quasi tutto:

- **IT-Advanced accetta P.IVA diretta**: `GetITAdvanced(ctx,
  vatCodeTaxCodeOrID)` (`openapiit/company.go:53`); `enrichTargetAdvanced`
  usa già `target.VATCode`/`TaxCode` come identificatore
  (`ma_gated_search_job.go:539-558`). L'azienda manuale salta lo stadio
  address: un solo fetch advanced fornisce identità + bilanci.
- **Il rimedio associate-domain è il prototipo del flusso**:
  `enqueueAssociateDomain` (`ma_gated_search_job.go:615`) +
  `runAssociateDomainJob` (`:731`) + `planDomainAssociationEnrich` (`:897`)
  ri-gateano UNA azienda con dominio fornito, la arricchiscono se necessario
  e la ri-punteggiano da sola, con garanzia "al massimo una paga per
  azione". Il job manuale è una variante: crea il target da P.IVA invece di
  trovarlo tra gli esistenti.
- **Coda job**: nuovo `job_type` su `binocolo.ma_job` (nessuna migrazione:
  il tipo è testo; verificare eventuali CHECK constraint nelle mig 049/057/
  073/076), stesso worker (`ma_job_worker.go`), stessa ownership
  per-istanza.
- **Routing derivato a lettura** (`ma_routing.go`): il bucket si deriva dai
  campi persistiti — serve l'eccezione origin=manual (decisione 2). Questo
  piano È autorizzato a toccare `ma_routing.go` limitatamente a questa
  eccezione (in deroga alla regola "non toccare" degli altri piani).

## Repo-fit (checklist `docs/IMPLEMENTATION-PLANNING.md`)

- **Auth**: endpoint nuovi dietro `app_binocolo_access`.
- **Data contract**: `company_key` da `maTargetDedupeKey` (stessa dedupe del
  funnel: l'azienda manuale che ricompare in una ricerca futura è la stessa
  entità); dominio in `ma_company_domain`.
- **Migrazioni**: idempotenti; **numerazione: questo piano parte da 106**
  (102-104 riservate da INIZIATIVA-RADICE, 105 da SGANCIO) — ricontrollare
  la prossima libera all'esecuzione.
- **Osservabilità**: trace event su ogni mutazione; job con retry/fail come
  gli esistenti.
- **Coesistenza**: funnel gated, scoring v3, gate UC2 invariati nella
  sostanza; l'unica deroga è l'eccezione di routing (sopra).

## Regole globali per l'esecutore

1. **Database**: MAI connettersi ai DB degli env, né in lettura.
2. **Test**: NON aggiungere test nuovi senza approvazione; verifica backend:
   `cd backend && go build ./... && go vet ./internal/binocolo/ && gofmt -l
   internal/binocolo/` (stampa nulla), `go test ./internal/binocolo/` verdi
   (fake store al minimo). NOTA: l'eccezione di routing (B3) è una regola di
   business non banale — proporre all'utente UN test mirato su
   `gatedTargetBucket`/routing con origin=manual, come da regola repo.
3. **Frontend**: `pnpm --filter mrsmith-binocolo exec tsc --noEmit`; smoke
   con playwright-cli (cwd=`artifacts/claude`, bypass verificati).
   **ATTENZIONE SPESA**: l'inserimento manuale SPENDE (~€0.12). Lo smoke di
   inserimento live è UNA azienda, scelta dall'utente o dichiarata prima; il
   resto si verifica su stati esistenti.
4. **Costi mai in UI utente**, nemmeno negli errori.
5. **API**: path Go senza `/api`; URL pubblico `/api/binocolo/v1/...`.
6. **Copy**: italiano B2B asciutto. «Aggiungi azienda», «Inserita
   manualmente». Mai gergo pipeline (gate, advanced, origin).
7. **Hot reload** `air`: degradare in modo innocuo pre-migrazione o
   dichiarare l'ordine di apply.
8. **Non toccare**: scoring v3, gate UC2 (`ma_sector_classification.go`,
   `ma_web_validation_job.go` salvo il punto d'ingresso single-company già
   esistente), worker deep. `ma_routing.go` SOLO per l'eccezione B3.
9. A fine task: file toccati + verifiche con esito.

## Ordine di esecuzione e dipendenze

```
B1 (mig 106: origin su ma_target) ──→ B2 (job manual_add + endpoint) ──→ F1 (UI)
                                  ──→ B3 (eccezione routing + esclusioni contatori/eval)
B2 prima di B3 solo per comodità di test; sono quasi indipendenti.
```

---

## B1 — Migrazione 106: provenienza del target

**Migrazione 106** `106_binocolo_ma_target_origin.sql` (idempotente):
- `ALTER TABLE binocolo.ma_target ADD COLUMN IF NOT EXISTS origin text NOT
  NULL DEFAULT 'search'` (+ CHECK `origin IN ('search','manual')` se lo
  stile delle mig esistenti lo usa — verificare la 026/075).
- Nessun backfill: tutte le righe esistenti sono `search` (default).

**Backend**: campo `Origin` su `MATarget` (`ma_types.go`) + lettura/
scrittura nello store (censire i punti: `ReplaceMATargets`, scan dei
target, proiezione `MATargetRow` se il badge UI serve in riga — sì, serve:
aggiungerlo alla proiezione). TS: `origin` su `MATarget`/`MATargetRow`
(`apps/binocolo/src/api/types.ts`).

**Degradazione**: dichiarare "applicare la 106 prima del deploy" (la SELECT
estesa la richiede).

**Verifica**: build/vet/gofmt/test.

---

## B2 — Job `manual_add` + endpoint

**Backend:**
- `POST /binocolo/v1/ma/sessions/{id}/targets/manual` con body
  `{vatCode, domain?}`:
  - validazione: sessione operational; P.IVA sintatticamente plausibile
    (normalizzazione esistente — cercare come `companyDossier` normalizza la
    VAT); dominio se presente normalizzato come nel rimedio associate-domain;
  - **dedupe**: se un target con lo stesso `maTargetDedupeKey` esiste già
    nella sessione → 409 con messaggio chiaro («Azienda già presente nella
    ricerca»), nessuna spesa;
  - enqueue `maJobTypeManualAdd` (pattern `enqueueAssociateDomain`,
    `ma_gated_search_job.go:615`: pre-lease all'owner, unicità in-flight per
    sessione+tipo — valutare se la chiave di unicità debba includere la VAT
    per permettere inserimenti multipli in coda; in caso contrario, gli
    inserimenti si serializzano: accettabile, documentarlo).
- `runManualAddJob` (nuovo, accanto a `runAssociateDomainJob`,
  `ma_gated_search_job.go:731` come riferimento strutturale):
  1. **fetch IT-Advanced per P.IVA** (`GetITAdvanced`,
     `openapiit/company.go:53`): non trovata → job failed con errore
     leggibile («P.IVA non trovata nel registro»);
  2. costruire il `MATarget` dal dataset advanced (riusare il mapping del
     funnel — cercare dove `enrichTargetAdvanced` applica il payload al
     target e dove il flusso address costruisce il target iniziale;
     `origin='manual'`, `advanced` già arricchito → `MarkMATargetAdvancedEnriched`
     o equivalente del percorso gated) e persisterlo nella sessione
     (verificare che l'inserimento singolo non passi da `ReplaceMATargets`,
     che sostituisce l'intero set — serve un insert singolo; se manca,
     aggiungere `InsertMATarget` allo store);
  3. **dominio fornito**: registrarlo in `ma_company_domain` come confermato
     dall'analista (riusare la persistenza del rimedio associate-domain) e
     lanciare la validazione web single-company con dominio fisso (percorso
     già esistente nel rimedio); **senza dominio**: risoluzione automatica
     standard single-company (Brave), con possibile esito manual_review che
     confluisce nella coda di verifica esistente;
  4. **score**: ri-punteggiare come fa `runAssociateDomainJob` a valle
     dell'enrich (stessa chiamata, set della sessione);
  5. trace event `ma_target_manual_added` (session, company_key, dominio
     fornito sì/no).
- Il gate qui è **informativo** (decisione 2): il verdetto si persiste
  normalmente in `ma_target_web_validation`, ma non deve produrre l'effetto
  "reject = escluso" — vedi B3.

**Verifica**: build/vet/gofmt/test.

---

## B3 — Eccezione di routing + esclusioni contatori/eval

**Backend:**
- `ma_routing.go` (deroga autorizzata, SOLO questo): per `origin ==
  'manual'`, il verdetto gate reject/deprioritize NON instrada mai a
  `soppresso`/fuori-lista: il target resta nei bucket visibili (es.
  `da_verificare` se il gate dissente, `principale`/`azionabile` secondo
  score come gli altri se il gate conferma). Il verdetto del gate resta
  visibile nel drawer/inspector (già renderizzato — F5 sgancio, riga 1).
- **Contatori funnel**: `gatedBucketCounts`/gated-progress
  (`ma_gated_progress.go`) escludono i target manual dai conteggi di
  superficie/gate/enrich (l'azienda manuale non è passata dalla superficie:
  conteggiarla falsa le percentuali). Se l'esclusione complica, in
  alternativa esporli come voce separata («+N inserite manualmente») — MAI
  mescolati.
- **Eval**: `ma_sector_replay.go` / sector-eval e qualunque metrica di
  survivor-rate del gate escludono `origin='manual'` (il gate non poteva
  respingerle: contaminerebbero la precision/recall).
- Censire i punti con grep su `origin` assente: cercare dove si aggregano i
  target per stadio/bucket (`helpers.ts` lato frontend per `isGateReject` —
  verificare che un manual con gate reject non finisca nel tab «Fuori
  tesi» come escluso definitivo: deve stare nei risultati con il verdetto in
  evidenza).

**Verifica**: build/vet/gofmt/test; proporre il test mirato (regola
esecutore 2).

---

## F1 — UI: «Aggiungi azienda» nella pagina ricerca

**File**: `apps/binocolo/src/pages/ricerche/RicercaDetailPage.tsx` (+
`helpers.ts` se serve per il bucket).

- Bottone **«Aggiungi azienda»** nella toolbar dei risultati (visibile anche
  a sessione mai eseguita: è il caso watchlist — verificare che la pagina
  regga una sessione senza run: se oggi lo stato "mai eseguita" non
  renderizza la tabella, prevedere l'empty state «Nessun risultato — esegui
  la ricerca o aggiungi aziende»).
- Modale: campo P.IVA (obbligatorio, validazione client-side basilare) +
  campo Dominio (opzionale, con hint «Se lo conosci, l'inserimento è più
  affidabile»). Submit → POST B2 → toast e riga in stato "in elaborazione"
  (il polling della pagina esiste già: verificare che lo stato job/stage
  del manual_add rientri nel ciclo di refresh; in caso contrario, riusare il
  pattern di polling esistente a 4s finché il job non è terminale).
- Riga target manuale: badge **«Inserita manualmente»** (da
  `MATargetRow.origin`), per il resto identica alle altre (rating, drawer,
  inspector, export).
- Errori leggibili nel modale: P.IVA non trovata, azienda già presente
  (409), job fallito (con retry = ripetere l'inserimento, idempotente per
  dedupe).
- Nessuna menzione di costi (regola 4).

**Verifica**: `tsc --noEmit`; smoke: modale, validazioni, 409 su azienda già
presente (usare una VAT di un target esistente della sessione: nessuna
spesa); l'inserimento live completo è UNA azienda concordata con l'utente
(regola 3) — in alternativa lasciarlo alla verifica dell'utente,
dichiarandolo.

# Binocolo — Piano "Navigazione Scheda azienda" (Plancia + Staffetta)

> Esegue la distillazione del design-review a tre lenti (IA, workflow
> analista, interaction design) del 2026-07-08 sulla Scheda azienda
> (`/aziende/:companyKey`), ratificata dall'utente: **Plancia e Staffetta si
> implementano insieme**. Ogni task è pensato per essere eseguito **da solo,
> in ordine**, da un LLM esecutore. Riferimenti **verificati sul codice al
> 2026-07-08 (HEAD `42b21de`)**. Se un simbolo citato non esiste più,
> fermarsi e segnalarlo.

## Contesto e diagnosi (perché questo piano esiste)

La Scheda azienda è «il distillato di tutta binocolo» ma oggi è un
documento-pozzo: ~5.000px in colonna singola, i numeri decisionali (verdetto,
equity range, red flags) a 2-3.000px dallo header, anagrafica duplicata
above-the-fold, lente di contesto comunicata da uno `<span>` muto e non
commutabile, nessuna navigazione interna né uscita (tutti gli entry point
aprono in `target="_blank"` e la pagina non ha breadcrumb), apparizioni che
portano *fuori* dalla scheda invece di cambiarle lente, disclosure `<details>`
invisibili, zero tastiera. Diagnosi completa nei tre report agent (in
conversazione); questo piano ne implementa la sintesi.

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **Plancia** — barra di contesto sticky (lente nominata + commutabile +
   ritorno all'origine + verdetto + stelle + numeri chiave) e **spina** di
   navigazione sui 4 blocchi con scroll-spy, micro-stati e hotkey.
2. **Staffetta** — navigazione seriale sulla coorte della lente attiva
   (`j`/`k`, «azienda N di M», prefetch della vicina, toast quando il deep
   diventa pronto), coorte passata via `location.state` (frontend-only,
   degradazione elegante al reload).
3. Le due proposte si implementano **contemporaneamente** in questo piano.
4. **Non si ribalta nulla di ratificato**: i 4 blocchi-domanda restano la
   struttura (`SCHEDA-AZIENDA-PLAN.md`), le lenti restano in URL, le stelle
   restano session-scoped con lente ricerca. Cambia come li si naviga.
5. Bonifiche incluse (parte integrante della Plancia): dedup anagrafica,
   disclosure con etichetta di densità, rimozione delle sole progress bar
   quote soci (i grafici dei bilanci camerali si CONSERVANO — rettifica
   2026-07-08), rimozione banner passivi (sostituiti da controlli),
   rimozione dell'auto-apertura `open={!deep?.status}`.
6. **Fuori perimetro**: tab al posto dello scroll (proposta «Dossier a due
   riquadri», scartata: uccide la simultaneità); ⌘K palette (evoluzione
   futura, non ora); fix selezione apparizione advanced-first e registro
   camerale/dettaglio scoring sulla scheda (sono il candidato F1-bis di
   `FUSIONE-CARD-DOSSIER-PLAN.md`, decisione separata); inspector.

## Anatomia attuale (verificata, HEAD `42b21de`)

- Pagina: `apps/binocolo/src/pages/aziende/SchedaAziendaPage.tsx` (~830
  righe) + `SchedaAziendaPage.module.css`. Colonna singola, 4
  `<section className={styles.block}>` con heading id già ancorabili:
  `scheda-identita-title` (:499), `scheda-deep-title` (:585),
  `scheda-controllo-title` (:630), `scheda-storia-title` (:699).
- Lente: `resolveLens(searchParams)` (:64-70) → `ricerca | iniziativa |
  globale` + flag `ambiguous`; oggi resa come pill testuale (:462-463) e tre
  banner passivi impilabili (:474-493). `useSearchParams` usato in sola
  lettura (:276).
- Stelle: `RatingStars` nell'header (:469), solo lente ricerca.
- Dati per la Plancia già in pagina: `overview.appearances`
  (`MACompanyOverviewAppearance[]`: sessionId, sessionTitle, initiativeId,
  initiativeTitle, targetId, rating, bucket, enrichmentLevel…),
  `overview.cards` (iniziative), `target` (payload advanced), `deep`
  (scorecard/valuation/brief), `analysis = target.webValidation.
  candidateMatchAnalysis` (:434).
- Uscite oggi solo nel blocco 4: `HistorySection` (:770-801) linka a
  `/ricerche/{id}`, `CardsSection` (:803-830) al board — mai alla scheda
  stessa sotto altra lente.
- Entry point verso la scheda (tutti `target="_blank"`): builder
  `RicercaDetailPage.tsx:102` (usato a :1358, :1367, :1613),
  `IniziativaBoardPage.tsx:53`, `IniziativaCardDossierPage.tsx:37`.
- Polling deep: `useEffect` :328-335, `setInterval` 5s, refetch di overview
  e target finché `queued|running`.
- Componenti condivisi disponibili (`@mrsmith/ui`): `SingleSelect`,
  `StatusBadge`, `TabNav`, `Tooltip`, `Skeleton`, `Icon`, `Button`,
  `ToastProvider`/`useToast`. Design system: `docs/UI-UX.md` (sticky offset
  §12, animazioni §8.2/8.3, accent bar §13.2, responsive §15).
- Dataviz decorativa da rimuovere: bar chart SVG fatturato
  (`components/company/VendorFinancials.tsx:95-164`), progress bar quote
  (`components/company/ShareholdersDetail.tsx:83-87`).

## Regole globali per l'esecutore

1. **Database: mai.** Nessuna migrazione in questo piano (tutto frontend).
2. **Test: non aggiungerne.** Verifica per ogni task:
   `pnpm --filter mrsmith-binocolo exec tsc --noEmit`; a fine piano anche
   `pnpm --filter mrsmith-binocolo exec vite build`.
3. **Smoke**: playwright-cli (cwd=`artifacts/claude`, bypass verificati),
   **sola lettura** sul DB condiviso: mai stellinare davvero, mai lanciare
   deep reali, mai mutare IRL. Riusare il dev server attivo se presente.
4. **Costi (€) mai in UI.** Copy italiano B2B asciutto (sostantivi secchi,
   no prima persona: «Torna alla ricerca», «azienda 3 di 18», «Analisi
   pronta»). Niente contatori decorativi: ogni numero mostrato deve essere
   cliccabile/azionabile o non esserci.
5. **UI**: leggere `docs/UI-UX.md` prima di ogni task visivo; usare la skill
   `tintoretto` per il lavoro di stile; componenti `@mrsmith/ui` prima di
   inventarne; token, mai colori hard-coded; `prefers-reduced-motion`
   rispettato su ogni animazione/scroll; nessuna dipendenza npm nuova
   (IntersectionObserver nativo, `prefetchQuery` è TanStack Query già
   presente).
6. **Animazioni**: entrance solo alla navigazione, mai ri-triggerate dal
   polling/refetch (§8.3) — keyare su route/companyKey, non sui dati.
7. **Non toccare**: inspector; backend/endpoint; board e drawer (salvo i
   punti d'ingresso indicati in S1); le query esistenti della scheda (si
   estende, non si riscrive).
8. A fine task: file toccati + verifiche con esito.

## Ordine di esecuzione e dipendenze

```
P1 (barra di contesto / lente)        ── prima: sostituisce header meta e banner
P2 (spina + hotkey 1-4)               ── dopo P1 (condivide il layout a colonne)
P3 (bonifiche contenuto)              ── dopo P2 (tocca gli stessi blocchi)
S1 (coorte dagli entry point)         ── indipendente da P1-P3
S2 (staffetta: N di M, j/k, prefetch) ── dopo P1 e S1 (vive nella barra di P1)
S3 (toast deep pronto + pulse)        ── dopo P2 (usa i micro-stati della spina)
V  (verifica finale integrata)        ── ultimo
```

---

## P1 — Barra di contesto: la lente diventa un oggetto di prima classe

**File**: `SchedaAziendaPage.tsx`, nuovo
`apps/binocolo/src/components/scheda/LensBar.tsx` + `LensBar.module.css`.

Barra sticky a tutta larghezza subito sotto l'header AppShell (offset
coerente con §12 di `docs/UI-UX.md`; z-index sotto modal/drawer), sempre
visibile durante lo scroll. Contenuto, da sinistra a destra:

1. **Breadcrumb di lente**: `Ricerca «{sessionTitle}» › {companyName}`
   oppure `Iniziativa «{initiativeTitle}» › {companyName}` oppure
   `Vista globale › {companyName}`. La prima parte è un **link** all'origine
   (`/ricerche/{id}` o `/iniziative/{id}`) — è l'uscita che oggi manca.
   Nome reale della ricerca/iniziativa, mai il tipo astratto: i titoli sono
   già in `overview.appearances[].sessionTitle` e
   `overview.cards[].initiativeTitle` (per la lente ricerca, risolvere il
   titolo dall'appearance con `sessionId === lens.id`).
2. **Selettore lente** (`SingleSelect` condiviso): elenca le lenti reali di
   questa azienda — una voce per ricerca in `overview.appearances`
   (etichetta = sessionTitle), una per iniziativa in `overview.cards`
   (etichetta = initiativeTitle), più «Vista globale». Selezione →
   `setSearchParams` (aggiungere il setter a :276) con il solo param della
   lente scelta (`{ricerca: id}` | `{iniziativa: id}` | `{}`), **senza
   reload né tab nuova**: le query dipendenti sono già keyed su
   `lens`/`sourceAppearance` e si ri-inquadrano da sole. Con molte lenti il
   dropdown scrolla al suo interno; nessun conteggio in etichetta.
3. **Pill verdetto** (lente ricerca): `verdictLabel(analysis?.verdict)` +
   `bucketLabel` — gli stessi valori oggi resi a :542-543; qui è la copia
   sempre visibile, il blocco 1 resta la sede estesa.
4. **Stelle**: spostare `RatingStars` (:466-471) dalla header-box alla
   barra; visibili solo con lente ricerca + `ricercaAppearance`, come oggi.
   Stessa mutation `submitRating` (:366).
5. **Striscia numeri chiave** in `font-variant-numeric: tabular-nums`:
   Fatturato e EBITDA (da deep scorecard se presente, altrimenti da
   `target.turnover`/vendor payload), Equity range (da `deep.valuation`),
   Red flags (conteggio dai flag del brief deep) — ogni tile è un **link di
   salto** al blocco che lo dettaglia (non decorativo). Deep assente → tile
   a «—»; nessun dato affatto → la striscia non compare.

Contestualmente **rimuovere**: i pill di lente `:462-463`; i tre banner
passivi `:474-493`. Il caso ambiguo (entrambi i param in URL) non produce
più un banner: il selettore mostra la lente attiva (precedenza ricerca,
com'è in `resolveLens`) e l'altra è a un click. Il caso «azienda non in
questa ricerca» (`:481-486`) diventa uno stato del selettore/pill (voce
disabilitata o nota inline nella barra), non un banner. Il caso di errore
target (`:488-493`) resta ma si sposta sotto la barra.

Su viewport <900px la barra comprime: breadcrumb troncato con ellipsis,
striscia numeri nascosta, selettore e stelle restano.

**Verifica**: `tsc --noEmit`; smoke (sola lettura): aprire una scheda con
`?ricerca=`, verificare nome lente nel breadcrumb, cambiare lente dal
selettore → URL aggiornato senza reload e verdetto/stelle ri-inquadrati;
link di ritorno → pagina ricerca; nessun banner residuo.

---

## P2 — Spina di navigazione: le 4 domande diventano attraversabili

**File**: `SchedaAziendaPage.tsx`, nuovo
`apps/binocolo/src/components/scheda/SpineNav.tsx` + `SpineNav.module.css`,
nuovo hook `useSectionSpy` (locale al componente o file a parte).

- Layout della pagina a due colonne (grid `220px minmax(0,1fr)`, gap
  esistente): **rail sticky a sinistra** (top sotto la LensBar di P1) con le
  4 voci numerate — riusare numeri e titoli dei `blockHeader` esistenti:
  1 Cosa fa ed è in tesi · 2 È sana e quanto vale · 3 Chi la controlla ·
  4 Cosa ne sappiamo. Sotto ~1000px il rail collassa in **barra segmento
  orizzontale sticky** sotto la LensBar (pattern `TabNav`), coerente con
  §15.
- **Scroll-spy**: `IntersectionObserver` sui 4 heading id già esistenti
  (:499, :585, :630, :699 — non aggiungere markup); voce attiva evidenziata
  con accent bar (§13.2). Calibrare `rootMargin` (es. `-40% 0px -55%`) per
  evitare sfarfallio tra blocchi di altezza molto diversa.
- **Click** → `scrollIntoView({behavior:'smooth'})`, `behavior:'auto'` se
  `prefers-reduced-motion`.
- **Micro-stati per voce** (mai contatori decorativi — ogni stato è
  cliccabile perché la voce lo è): blocco 2 → `StatusBadge` compatto con
  `deepStatusLabel(deep?.status)` (In coda/In corso/Pronta/—); blocco 3 →
  «—» quando i dati controllo sono assenti (condizione dell'`EmptyPanel`
  :636-637); blocco 4 → `{appearances.length} apparizioni ·
  {cards.length} iniziative`.
- **Hotkey `1`-`4`** → salto al blocco corrispondente. Listener globale
  disattivato quando il focus è dentro `input|textarea|select|
  [contenteditable]` o un Modal è aperto. Focus ring del design system
  sulle voci.

**Verifica**: `tsc --noEmit`; smoke: scroll manuale → la voce attiva segue;
click e tasti 1-4 → salto corretto; stato deep visibile nel rail senza
scrollare; sotto 1000px la barra orizzontale funziona.

---

## P3 — Bonifiche di contenuto (dedup, disclosure viventi, dataviz)

**File**: `SchedaAziendaPage.tsx`, `components/company/VendorFinancials.tsx`,
`components/company/ShareholdersDetail.tsx`, nuovo
`apps/binocolo/src/components/scheda/LabeledDisclosure.tsx` (o css module
condiviso).

1. **Dedup anagrafica**: l'header (:454-472) tiene nome, P.IVA/CF, sede; la
   `identityGrid` del blocco 1 (:503-536) perde «Ragione sociale»,
   «Identificativi», «Sede» e tiene solo ciò che l'header non ha: Forma,
   ATECO, Dominio. La griglia si compatta (3 campi).
2. **Disclosure viventi**: sostituire i tre `<details>` a mano (:568-577
   verifica web, :613-623 bilanci, :682-692 soci) con `LabeledDisclosure`,
   che mantiene la semantica `<details>/<summary>` ma aggiunge l'etichetta
   di densità nel summary: «Dettaglio della verifica · dominio confermato»
   (da `webValidation.selectedDomain`/esito), «Bilanci (fonte camerale) ·
   N esercizi» (da `vendorFinancialSheets`), «Tutti i soci · N» (da
   `extractShareholders`). Questi numeri sono indice del contenuto sotto il
   summary, non contatori decorativi.
3. **Rimuovere l'auto-apertura instabile**: `open={!deep?.status}` (:614)
   sparisce. I bilanci sono chiusi di default; il caso "deep assente"
   è già coperto dalla striscia numeri della LensBar (tile a «—» +
   micro-stato spina) e dall'etichetta di densità.
4. **Dataviz** — RETTIFICA proprietario 2026-07-08: i grafici dei bilanci
   da dati camerali NON si toccano — il bar chart SVG «Andamento
   fatturato» (`VendorFinancials.tsx:95-164`) **resta com'è**. Si
   rimuovono solo le progress bar delle quote
   (`ShareholdersDetail.tsx:83-87`) — restano percentuali numeriche.
   Attenzione: componente condiviso col card-dossier — la rimozione vale
   per entrambe le superfici (politica varianti: mai due semantiche).
5. **Le apparizioni cambiano lente, non pagina**: in `HistorySection`
   (:770-801) il link primario di ogni apparizione diventa
   `/aziende/{companyKey}?ricerca={sessionId}` (stessa scheda, lente
   diversa, stessa tab — è il gesto complementare al selettore di P1); il
   link alla pagina ricerca resta come azione secondaria («Apri ricerca ↗»).
   Stesso trattamento in `CardsSection` (:803-830): primario = lente
   iniziativa sulla scheda, secondario = board.

**Verifica**: `tsc --noEmit`; smoke: nessun campo duplicato tra header e
blocco 1; summary con densità; niente grafico/progress bar (controllare
anche il card-dossier); click su un'apparizione → stessa scheda con lente
cambiata.

---

## S1 — La coorte viaggia con la navigazione

**File**: `RicercaDetailPage.tsx` (builder :102 e i tre usi :1358, :1367,
:1613), `IniziativaBoardPage.tsx` (builder :53), `IniziativaCardDossierPage.tsx`
(builder :37), nuovo `apps/binocolo/src/components/scheda/cohort.ts`
(tipi + helpers).

- Definire `SchedaCohort = { lensType: 'ricerca'|'iniziativa', lensId:
  string, companyKeys: string[] }` in `cohort.ts`.
- Gli entry point che oggi costruiscono l'href della scheda passano anche la
  coorte: **l'elenco ordinato dei companyKey così come l'utente lo vede**
  in quel momento (la lista renderizzata della tabella funnel in
  RicercaDetailPage — con filtri/ordinamento correnti applicati — o le card
  del board). Poiché gli entry point sono `<a target="_blank">` e
  `location.state` non attraversa un nuovo tab, il canale è
  **`sessionStorage`**: alla partenza si scrive
  `sessionStorage.setItem('binocolo.scheda.cohort', JSON.stringify(cohort))`
  in un handler `onClick`/`onAuxClick` (il click parte prima
  dell'apertura del tab; `sessionStorage` è condiviso dai tab aperti via
  link dalla stessa sessione). La scheda legge la coorte solo se
  `lensType`/`lensId` coincidono con la lente attiva e il `companyKey`
  corrente è nella lista; altrimenti la ignora.
- **Degradazione elegante**: reload, deep-link condiviso, lente cambiata dal
  selettore di P1 verso una lente diversa da quella della coorte → nessuna
  coorte: la scheda funziona come oggi (niente frecce, niente N di M,
  nessun errore). Cap difensivo alla serializzazione (es. 500 key).

**Verifica**: `tsc --noEmit`; smoke: dal funnel di una ricerca aprire una
scheda → in `sessionStorage` la coorte con l'ordine visibile; aprire per
URL diretto → nessuna coorte e pagina integra.

---

## S2 — Staffetta: «azienda N di M», frecce e j/k con prefetch

**File**: `SchedaAziendaPage.tsx`, `LensBar.tsx` (di P1), `cohort.ts`.

- La LensBar, quando la coorte è presente e coerente con la lente attiva,
  mostra a destra del breadcrumb: **«azienda {N} di {M}»** + frecce ‹ ›
  (Button ghost/icon). N = indice del companyKey corrente nella coorte.
- **Navigazione**: freccia/hotkey → `navigate(
  '/aziende/{nextKey}?{lensType}={lensId}')` nello **stesso tab** (React
  Router, niente reload). Hotkey **`j`** = successiva, **`k`** =
  precedente; stesso guard-focus di P2 (mai rubare tasti a input/modal);
  agli estremi della lista le frecce si disabilitano (nessun wrap).
- **Prefetch**: al mount e a ogni navigazione,
  `queryClient.prefetchQuery` dell'overview della vicina successiva (stessa
  queryKey/fetch dell'overview della scheda) — la transizione `j` deve
  percepirsi istantanea. Solo l'overview: il target detail si carica come
  oggi.
- Le entrance animation (se presenti) si keyano su `companyKey` così la
  transizione comunica il cambio azienda senza ri-animare sul polling.
- Tooltip sulle frecce con la scorciatoia («Successiva · J»).

**Verifica**: `tsc --noEmit`; smoke: dal funnel aprire la prima azienda →
«azienda 1 di M»; `j` → seconda azienda, URL aggiornato, lente conservata,
transizione senza flash di skeleton (prefetch); `k` al contrario; frecce
disabilitate agli estremi; con focus dentro il modal di esclusione `j` non
naviga; senza coorte niente frecce.

---

## S3 — Il deep che arriva si annuncia

**File**: `SchedaAziendaPage.tsx`, `SpineNav.tsx` (P2).

- Nel `useEffect` di polling (:328-335) intercettare la **transizione di
  stato** `queued|running → ready` (confronto con il valore precedente via
  ref — mai su ogni poll): al passaggio, `useToast` (già in `@mrsmith/ui`,
  provider da verificare nel mount dell'app — se assente, montarlo
  nell'App shell di binocolo) con «Analisi pronta» + azione «Vai» che salta
  al blocco 2.
- Il **micro-stato del blocco 2 nella spina** (P2) si aggiorna da solo
  (deriva da `deep.status`); aggiungere un **singolo** highlight-pulse
  sulla sezione blocco 2 alla transizione (una volta, `animation` one-shot,
  niente loop — §8.3; spento con `prefers-reduced-motion`).
- Transizione `→ failed`: toast di errore sobrio («Analisi non riuscita»)
  — il blocco 2 offre già il retry (:600-604).

**Verifica**: `tsc --noEmit`; smoke senza spesa: non lanciare deep reali —
verificare la logica della transizione con un mock locale (es. stato
simulato via React Query devtools o modifica temporanea non committata) o
in code-review; a schermo verificare solo che nessun toast appaia sul
polling normale di una scheda con deep già pronto.

---

## V — Verifica finale integrata

1. `pnpm --filter mrsmith-binocolo exec tsc --noEmit` e
   `pnpm --filter mrsmith-binocolo exec vite build` verdi.
2. Smoke completo (sola lettura, dev server riusato): percorso reale
   dell'analista — funnel ricerca → apri scheda (tab nuovo) → LensBar con
   nome ricerca e numeri → `2` per saltare al valore → `j` per la
   successiva → cambio lente dal selettore verso un'iniziativa → ritorno
   all'origine dal breadcrumb. Verificare anche il card-dossier (condivide
   VendorFinancials/ShareholdersDetail bonificati) e una scheda senza
   alcuna apparizione ricca (vista globale, dati minimi).
3. Screenshot desktop + <1000px in `artifacts/claude/`.
4. Aggiornare `apps/binocolo/docs/SCHEDA-AZIENDA-PLAN.md` con una riga di
   rimando a questo piano (sezione navigazione) — nessun'altra modifica
   documentale.

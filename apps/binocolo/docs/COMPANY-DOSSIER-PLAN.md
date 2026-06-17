# Company Dossier — piano di implementazione

## Obiettivo

Nuova pagina in **binocolo**: l'utente inserisce una **partita IVA**, chiamiamo
**IT-full** di OpenAPI.it e presentiamo in un'unica pagina ricca **(a)** i dati
grezzi dell'azienda, **(b)** le nostre elaborazioni (scorecard, valutazione) e
**(c)** un brief LLM arricchito. Pagina di pura *company intelligence*,
thesis-neutral, scollegata dal funnel M&A.

La pipeline di analisi **esiste già** (deep-dive del funnel: IT-full → scorecard →
valuation → brief, persistito in `binocolo.ma_deep_analysis`, globale per
`company_key`). Il lavoro è soprattutto **un nuovo punto d'ingresso + una nuova
tela di presentazione**, più un fix di calibrazione dell'engine che la pagina rende
non più rinviabile (mostrando il dato grezzo accanto alle nostre elaborazioni, ogni
incongruenza diventa visibile).

## Decisioni bloccate

1. **Brief canonico unico**: si aggiorna l'unico brief a "ricco"; il funnel ne
   mostra una versione condensata. Niente seconda variante.
2. **Solo P.IVA**: IT-full fa il lookup direttamente per partita IVA
   (`CreateITFullRequest(ctx, vat, …)`); niente IT-search, niente ricerca per nome.
   Chi non conosce la P.IVA non cerca.
3. **Legenda IIC → JSON committato**: `company-legend.html` (276 codici, 21 sezioni
   + forma giuridica + ruoli manager) parsato una volta in `iicLegend.json`
   committato; l'HTML resta documentazione.
4. **Backfill via endpoint admin**: endpoint role-gated che ricalcola dai payload
   in cache (mai ri-fetch IT-full → €0; solo il brief usa l'LLM).

## Fase 0 — Calibrazione engine + backfill (fix-first, prerequisito)

### 0.1 Fix `ma_deep_engine.go` ✅ FATTO

Calibrazione sui 5 payload reali. Tutte correzioni certe (no tuning soggettivo):

| Metrica | Problema reale | Fix |
|---|---|---|
| `capitalizzazione` | `capitalizationDegree` è **frazione** (0–1), la soglia `ragHigher(20,40)` si aspettava 0–100 → **red per tutte** | scala ×100 |
| `ebit_variation` | `ebitVariation` è **frazione**, mostrata come % → +33,5% diventava "0,34%" → inchiodata amber | scala ×100 |
| `leverage` | con PN ≤ 0 (insolvenza) il valore va negativo e `ragLower` lo legge come "sano" → green | guardia segno: PN ≤ 0 → red |
| `debt_ratio` | ≈ passività/PN, **ridondante con `leverage`**, scala indefinita, **sempre green** | **rimossa** dalla scorecard |

**Non un bug (mio errore, corretto):** il `roe` vendor è giusto. `IIC179` =
"PROFIT/LOSS FOR THE YEAR" (utile d'esercizio), `IIC178` = "Total income tax" (le
imposte). `roe_vendor = IIC179/netWorth` riconcilia su tutti e 5 → **NON si
ricalcola** (l'avremmo solo regredito). `ros` verificato = EBIT/fatturato. Resto
delle soglie verificato sano sui dati reali.

Convenzione di scala documentata nel commento dell'engine: ros/roe/roi e i trend
arrivano già in %, `capitalizationDegree`/`ebitVariation` in frazione (×100).

### 0.2 Test golden ✅ FATTO

`TestBuildMADeepScorecardRealPayloads` (`ma_deep_engine_test.go`), table-driven su
payload reali ridotti ai campi letti dall'engine:
- **REDOKUN** → overall **amber** (era red), `capitalizzazione` green, `leverage` green
- **KRAL** (PN −81.980) → overall red, `leverage` **red** (guardia segno), `capitalizzazione` red
- **MFT** → resta red (debole reale), ma `ebit_variation` **green** (era amber)
- asserzione che `debt_ratio` non è più emessa

Fixture sintetiche preesistenti riportate alla scala reale (frazioni). Suite
`./internal/binocolo/` verde.

### 0.3 Backfill

- **0.3a — recompute scorecard** ✅ FATTO. Endpoint `POST /binocolo/v1/ma/deep/recompute`
  (gated `BinocoloAccessRoles`, tracciato `ma_deep_recompute`) →
  `maService.recomputeMADeepScorecards`: per ogni riga `ready` ricostruisce la **sola
  scorecard** da `itfull_payload` in cache. **Niente valuation** (il fix non cambia
  ebitda/turnover/PFN/multiplo → bande EV/equity identiche), niente lookup settoriale,
  niente LLM, niente IT-full. Sincrono, idempotente, ri-eseguibile a ogni ritocco di
  soglia. Store: `ListMADeepReadyPayloads`, `UpdateMADeepScorecard`. `go vet` + suite
  `./internal/binocolo/` verdi. **Più snello del previsto**: scoperto che il fix tocca
  solo la scorecard → nessuno stato `recompute` nel worker, non serviva. (Da invocare
  in ambiente con DB; non eseguito su dati live da qui.)
- **0.3b — rigenerazione brief** ⏳ via LLM, **dopo** 1.3 (prompt ricco), senza IT-full.
  Finché non gira, il brief resta coerente con la vecchia `overallRag` (transitorietà
  accettabile: la pagina dossier non è ancora live; nel funnel è solo il riquadro brief).
- **Blast radius voluto**: stessa scorecard → si raddrizza anche il ranking del funnel.

## Fase 1 — Fondamenta backend del dossier

### 1.1 Endpoint standalone + riconciliazione chiave
```
POST /binocolo/v1/companies/{vat}/dossier   { acknowledgeCost }
GET  /binocolo/v1/companies/{vat}/dossier
```
**Problema chiave d'identità**: nel funnel `company_key` = VendorID (id esadecimale
di IT-search), non la P.IVA (confermato: `company_key` ≠ `vat_code` nei record).
Flusso:
- **(a)** cache-check per `vat_code` (nuovo `GetMADeepAnalysisByVAT`) → hit `ready` =
  istantaneo, €0, qualunque sia il `company_key` (intercetta le righe del funnel);
- **(b)** miss → cost-ack (sulla sola P.IVA; ragione sociale al reveal) →
  `EnqueueMADeepAnalysis(company_key = VAT, vat, tax)` → worker globale chiama IT-full
  per `vat_code` (come già fa) → poll `CheckITRequest` → render.

Route sotto `RequireRole(BinocoloAccessRoles())`, `r.PathValue("vat")`, trace
`ma_standalone_dossier`. Cost-gate: hit = €0; miss richiede `acknowledgeCost`.

**Edge**: società vista prima standalone (chiave=VAT) e poi dal funnel
(chiave=VendorID) → riga doppia + un ri-addebito. Mitigazione: check `vat_code`
nell'enqueue del funnel (piccolo).

**✅ FATTO**: store `GetMADeepByVAT` (ready-wins, poi più recente), service
`companyDossier`/`getCompanyDossier`, handler + route (`GET`/`POST`, trace
`ma_company_dossier`, `BinocoloAccessRoles`), validazione P.IVA 11 cifre / CF 16,
risposta `MACompanyDossier` (`absent|cost_required|queued|running|ready|failed`) con
il payload raw incluso (facts layer). L'hardening del funnel-enqueue resta come edge
aperto. `go vet` + suite verdi.

### 1.2 Legenda IIC → bilancio riclassificato
Parsing una-tantum di `company-legend.html` → `apps/binocolo/src/data/iicLegend.json`
(`codice → {descrizione, sezione, IC/PL}`) + decode `detailedLegalForm` e ruoli
manager. Backend resta passthrough del raw; il frontend mappa codici→etichette a
render. Riferimenti chiave già verificati: `IIC074`=TOTAL ASSETS, `IIC179`=utile
d'esercizio, `IIC177`=risultato ante imposte, `IIC178`=imposte.

**✅ FATTO**: `apps/binocolo/src/data/iicLegend.json` (276 codici → `{description,
section}`) generato da `company-legend.html` (parser in `artifacts/claude/`).
Correzione: `detailedLegalForm` e ruoli manager NON servono dalla legenda — il payload
IT-full li porta già decodificati; la legenda serve solo per i codici IIC del bilancio.

### 1.3 Brief ricco (decisione 1)
Riscrittura prompt `ma_deep_brief` (sezioni: sintesi esecutiva, profilo azienda,
lettura finanziaria, **punti di forza**, rischi categorizzati con domanda DD,
razionale valutazione, cosa indagare) alimentato col **raw ricco** (gruppo,
controllate, gare pubbliche, cariche, sedi, mix dipendenti, delta 2 anni, web).
Schema output esteso. **Migration** per la nuova versione del prompt (come la 040).
Deve precedere 0.3b.

**✅ FATTO**: `MADeepBrief` esteso (Go + TS) — `businessProfile`, `strengths[]`,
`redFlags[].category`, `valuationRationale`, `ddQuestions[]`. Worker:
`curateITFullForBrief` (rimuove gli array IIC opachi) alimenta il blocco `company`;
MaxTokens 900→2200; `parseMADeepBrief` ripulisce/limita i nuovi campi. Migration `045`
aggiorna **in place** il prompt di default (id `…402`, `prompt_id` stabile). Sblocca 0.3b.

## Fase 2 — Pagina (frontend)

### 2.1 Wiring
Rotta in `routes.tsx` + voce in `App.tsx navItems` + page + CSS module. **Nessun
new-app checklist**: binocolo è già cablato (package.json/Makefile/proxy `/api`).

### 2.2 Stati di lookup
Input P.IVA (valida 11 cifre); conferma; modale cost-ack solo su miss; **reveal
progressivo** durante l'async (anagrafica → bilanci → scorecard → valutazione →
brief); skeleton.

### 2.3 Dossier (un'unica pagina lunga + rail sticky)
- **Hero verdict-first**: RAG + banda equity, con regole di soppressione/flag quando
  PN<0, EBIT<0 o micro.
- **Provenienza esplicita**: "Elaborazione Binocolo" vs "Fonte: registro / IT-full".
- **Cross-link claim↔evidenza**: ogni red flag / banda linka al numero grezzo.
- **Valutazione** marcata come riferimento settoriale (non perizia).
- **Confronto 2 anni** (non multi-anno: IT-full dà anno corrente + alcuni `*L2Y`).
- **Strato fatti**: anagrafica, sedi (incl. cessate), cariche, gruppo+controllate,
  gare, dipendenti, web/social.
- **Bilancio riclassificato** via mappa IIC.
- **Pavimento "Dati completi"** raw collassabile.

### 2.4 Tipi
Estendo `MADeepAnalysis`/`MADeepBrief` in `api/types.ts`; aggiungo tipi fatti + raw.

## Fase 3 — Verifica

- Test golden engine (0.2) ✅ · riconciliazione backfill sui 5 record.
- **Smoke UI obbligatorio** (dev server + browser, `VITE_DEV_AUTH_BYPASS`+
  `SKIP_KEYCLOAK`, skill `playwright-cli`).
- Boundary cost-gate: hit = €0, miss richiede ack, nessun doppio addebito.
- Deep-link refresh su `/azienda`.
- `pnpm --filter mrsmith-binocolo exec tsc --noEmit`.
- Osservabilità: log enqueue + trace `ma_standalone_dossier`, 5xx sanitizzati.

## Repo-fit

| Layer | Stato |
|---|---|
| Runtime | ✅ pagina in SPA esistente, deep-link già gestiti da react-router |
| Dev | ✅ binocolo già cablato (porta Vite, proxy `/api`) |
| Auth | ✅ `app_binocolo_access` + Bearer via `useApiClient` |
| Dati | ⚠️ riconciliazione VAT↔VendorID = §1.1 (cache-by-vat) |
| Deploy | ✅ migration prompt + JSON legenda; nessun env nuovo |
| Verifica | ✅ §Fase 3 |

## Follow-up / aperti

- `debt_ratio` rimosso: rimettibile con definizione+soglia proprie via l'endpoint di
  recompute, se serve.
- Tuning soglie: rifattibile a costo ~zero (ricalcolo deterministico da cache).
- `IPL*` (forma abbreviata): i payload attuali sono tutti `IIC*` (forma ordinaria);
  gestire `IPL*` se compaiono.

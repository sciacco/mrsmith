# Piano — Scraper diretto per conferma dominio + contenuto di prima mano (UC2)

Stato: **IMPLEMENTATO (2026-06-30) via API esterna** — niente fetcher fatto in casa. Nessuna migrazione DB.

## Implementazione effettiva (sostituisce i "Componenti" sotto)

Invece di costruire `internal/platform/webfetch`, si usa un **servizio di scrape esterno** (Firecrawl-compatibile, `POST /v1/scrape` con `{"url":...,"formats":["markdown"]}`), così non manteniamo logica di crawling.

- **Client**: `backend/internal/platform/scrape` — un solo metodo `Scrape(ctx, url) → {Markdown, Title, SourceURL, StatusCode}`. `New` ritorna `nil` se l'URL base non è configurato (opzionale, come `brave.New`).
- **Config / wiring**: env var `BINOCOLO_SCRAPE_BASE_URL` (vuota = disabilitato) → `cfg.ScrapeBaseURL` → `scrape.New` in `main.go` → `binocolo.Deps.Scrape` → `maService.scrape` (assegnazione guardata per evitare il typed-nil dell'interfaccia).
- **Innesto** (`ma_web_validation_job.go`): `verifyDomainByScrape(candidates, target)` rimpiazza `chooseMAWebValidationDomain` nel job path. Politica **aggressiva** (scelta dall'utente): scrape dei top-`maDomainVerifyCandidateCap`(=5) candidati in ordine di rango; vince subito il primo con **P.IVA/CF on-page** (`target.VATCode`/`TaxCode`, match alfanumerico, ≥8 char) — può promuovere un candidato a basso rango (recupera acceptance_fail tipo MYWAI). In assenza, primo con **nome azienda come parola intera** (non substring: "safe" ≠ "creditsafe"). Se un identificativo era disponibile, ≥1 pagina è stata letta e nessuna combacia → **reject** (`nil` → `domain_unresolved`/`needs_domain_review` = bucket `forse`, **recall-safe**, mai `scarta`). Scraper spento o tutti gli scrape falliti (transport/4xx) → fallback al pick score-only di oggi.
- **Evidenza**: il markdown della homepage del dominio scelto viene riusato come corpus (`gatherNeutralEvidence(..., prefetchedMarkdown)`): `markdownToEvidenceSnippets` riduce link/immagini a testo, scarta righe-nav (<40 char), prende fino a `maScrapeEvidenceChunkCap`(=8) blocchi che precedono gli snippet Brave. Sistema l'evidenza-spazzatura (MDOTM pagina Apache, NG WAY snippet scollegati).
- **Decisione aperta**: la verifica usa `target.VATCode` **a prescindere da `IncludeIdentifiers`** (il flag governa solo la query Brave; il match on-page è esplicitamente consentito). Se si vuole rispettare il toggle anche qui, è una riga.

---

## Piano originale (build-our-own — non seguito)

Stato originale: da implementare. Modifica piccola e contenuta. Nessuna migrazione DB.

## Obiettivo

Dopo che Brave ha proposto i candidati dominio, fare **scraping diretto** per:
1. **Confermare quale candidato è il sito ufficiale** dell'azienda (alza la confidenza, becca i wrong-match);
2. **Raccogliere contenuto di prima mano** (home / chi-siamo) per migliorare il contesto dato al distillatore.

Regola operativa: **se lo scraping è ottimo ci si ferma; se fallisce, fallback al path Brave attuale.** Brave resta la fonte di *scoperta* dei candidati; lo scraper diventa *arbitro + raccoglitore di contenuto*.

Disciplina anti-vicolo-tortuoso: **NON è un crawler.** Pochi URL noti, tetti duri, fail-fast a Brave appena c'è attrito. Niente JS rendering, niente coda di link, niente sitemap, niente retry aggressivi.

## Vincolo legale/di dominio (non violare)

- La **P.IVA NON entra mai in una query di ricerca** (regola dura: in query → solo registri, mai siti ufficiali). Lo scraper la usa **solo per match ON-PAGE** sul sito candidato. Questo è il segnale forte che oggi manca e che lo scraping sblocca in modo bias-free.

## Decisione di design (default scelto, ribaltabile in fase implementativa)

**Default: lo scraper è arbitro tra i top-3 candidati Brave** (può scegliere il #2 se ha la P.IVA giusta e il #1 no). È ciò che alza la confidenza sul sito giusto e recupera gli acceptance-fail dove il sito vero era un candidato di rango più basso.

Alternativa conservativa (toggle): lo scraper può solo **confermare/bocciare il #1**; se lo boccia → fallback senza guardare gli altri. Più semplice, meno potente sugli acceptance-fail.

### Evidenza dal campo — sessione `391e7c67` (61 target)

Run di valutazione che conferma il valore del match P.IVA on-page e raffina la decisione arbiter:

- **Modo A — risolto sull'entità sbagliata (4 falsi positivi "risolti", tutti `media`/score 49–59):** COHERENCY→`linktr.ee` (piattaforma link-in-bio), CREA INFORMATICA→`igioland.it` (un cinema), CARDNOLOGY→`makemu.it` (turbine eoliche), MERZARIO ELECTRONICS→`merzariospa.it` (Merzario logistica, **collisione di cognome**). Solo la P.IVA on-page li smonta: igioland/makemu sono siti aziendali *veri ma dell'azienda sbagliata*, non blocklistabili.
- **Modo B — name-mismatch ad alta confidenza:** BE SAFE GROUP→`creditsafe.it` (**alta/75**). Brave era sicuro e sbagliato → serve conferma first-party, non si risolve con la blocklist né con la soglia di score.
- **Quick-win senza scraper:** `linktr.ee` è l'unica piattaforma che ha passato il cancello (score 49) perché non è in `isDomainResolutionExcludedDomain`; gli altri 7 aggregatori sono finiti correttamente in acceptance_fail. Aggiungerlo (+ valutare `companyreports.it`, `aziendeeasy.it`, `empresite.it`, `amministrazionicomunali.it`) è una riga ciascuno.
- **Conferma decisione arbiter top-3:** lo score Brave NON separa i wrong (BE SAFE wrong @75/alta vs molti corretti a score più basso) → non basta alzare la soglia; serve il segnale identità.

---

## Componenti

### 1. Nuovo fetcher generico — `backend/internal/platform/webfetch`

Client HTTP minimale, riusabile, **non un crawler**:

```go
type Client struct { /* http.Client con timeout */ }
type Result struct {
    FinalURL    string // dopo redirect
    Body        []byte // troncato a MaxBytes
    StatusCode  int
    ContentType string
}
func (c *Client) Fetch(ctx context.Context, rawURL string) (Result, error)
```

Default duri:
- UA onesto (es. `mrsmith-binocolo/1.0 (+contatto)`).
- Timeout ~7s (rispetta `ctx`).
- Max redirect 5; accetta `http→https` e `www`.
- Cap dimensione body ~1,5MB (tronca, non scaricare oltre).
- Niente cookie jar, niente JS.

### 2. Nuovo file binocolo — `backend/internal/binocolo/ma_site_scrape.go`

**Estrazione contenuto** (anti-leak distillatore — punto critico):
- `extractReadableText(body []byte) string` — parser HTML tollerante (`golang.org/x/net/html`), rimuove `script`/`style`/`nav`/`header`/`footer` per il *testo*, collassa whitespace, riusa `cleanText` + `isLowValueEvidence` + `maUnrenderedPlaceholderPattern`.
- **Cap rigido allo stesso budget del testo evidenza di oggi** (`maNeutralEvidenceTextCap`). Il testo di una pagina intera è molto più grosso di uno snippet: senza cap si peggiora il leak chain-of-thought del distillatore (visto su MAINSIM nella run KB-off).

**Estrazione identità** (per la confidenza sul dominio):
- `extractVATCodes(body) []string` — 11 cifre vicino a `P.IVA|partita iva|VAT|C\.?F\.?|codice fiscale`, normalizzando prefisso `IT`, punti, spazi. + `vatID` da JSON-LD.
- `extractJSONLDOrg(body)` — `<script type="application/ld+json">` → `Organization{legalName, vatID, address}`.
- `matchRagioneSociale(text, target.CompanyName) bool` — **riusa `companyResolutionTokens` + `domainResolutionStopword`** del resolver (coerenza: stessi token, stesse stopword incl. forma giuridica estesa).
- match comune (`target.Town`), email-sul-dominio.

**Funzione di scrape singolo:**
```go
type siteScrapeResult struct {
    FinalURL     string
    Text         string   // pulito + cappato
    Signals      []string // es. ["piva_match","ragione_sociale","email_on_domain"]
    PIVAMatch    bool
    PIVAMismatch bool     // pagina ha una P.IVA, ma diversa dal target
    Confidence   string   // alta | media | bassa | (vuoto = fallito)
    Failed       bool     // vuoto/JS/403/timeout
}
func (s *maService) scrapeCompanySite(ctx, candidate DomainResolutionCandidate, target MATarget) siteScrapeResult
```
- Fetch **home**. Se l'identità non emerge dalla home, segue **al massimo 1 link già presente** nella home verso `chi-siamo|contatti|azienda|note-legali|about|contact` (no path-guessing → no 404).

### 3. Integrazione in `buildMAWebValidation` (`ma_web_validation_job.go:~246-271`)

Nuova `chooseDomainWithScraper(ctx, target, candidates) → (selected *DomainResolutionCandidate, scrapedText string, conf string, runs []CandidateMatchEvidenceRun)` che si inserisce tra `resolveDomainCandidates` e il punto in cui oggi si chiama `chooseMAWebValidationDomain`:

1. Itera i **top-`siteScrapeMaxCandidates` (=3)** candidati in ordine di score Brave.
2. Per ciascuno: `scrapeCompanySite`.
3. **P.IVA match (e non-aggregatore — vedi guardia §4)** → scegli, `confidence=alta`, cattura `Text`, **break** (early-exit; di solito è il #1 → 1 fetch).
4. **P.IVA mismatch** → scarta il candidato (fix wrong-match LINFA→codedesign).
5. Nessuna P.IVA → tieni il miglior **soft match** (ragione sociale + comune/email).
6. Dopo il loop, scelta finale:
   - P.IVA match → quel dominio;
   - else soft match forte → quel dominio;
   - **else fallback a `chooseMAWebValidationDomain(candidates)`** (gate SERP attuale, invariato).

**Evidenza:**
- Se `scrapedText` è buono (non vuoto, supera `isLowValueEvidence`) → costruisci `maCompanyEvidence{Domain, Snippets:[scrapedText...]}` da quello e **salta `gatherNeutralEvidence`** (niente 3 probe Brave).
- Altrimenti → `gatherNeutralEvidence` (Brave, come oggi).

### 4. Guardia anti-aggregatore (attenzione sui domini)

Una pagina aggregatore *sull'*azienda può contenere la sua P.IVA → rischio di "confermare" l'aggregatore. Confermo via P.IVA **solo se**:
- il dominio **non** è in `isDomainResolutionExcludedDomain`, **e**
- l'host contiene il brand **oppure** la P.IVA è in footer/contatti/JSON-LD (contesto first-party), non in una lista-directory.

### 5. Cancello "ottimo → stop, altrimenti Brave" (deterministico)

| Esito scrape | Azione |
|---|---|
| P.IVA match (non-aggregatore) | **stop**: dominio confermato `alta`, evidenza da scrape, **niente Brave** |
| Soft match forte (rag. soc. + comune/email) | accetta, evidenza da scrape |
| P.IVA mismatch | scarta il candidato |
| Vuoto / JS / 403 / timeout / zero segnali | **fallback Brave** (path attuale intatto) |

### 6. Config / flag / provenance

- Costanti: `siteScrapeMaxCandidates=3`, `siteScrapeTimeout`, `siteScrapeMaxBytes`, riuso `maNeutralEvidenceTextCap`.
- Flag `useSiteScrape` (default on) per spegnerlo se fa danni; compone con la modalità inline dev.
- **Niente migrazione.** Provenance persistita nei jsonb esistenti: un `CandidateMatchEvidenceRun{Bucket:"scrape", Term: finalURL, Matched, ...}` in `EvidenceRuns`, e i `Signals`/P.IVA-matched nella reason del candidato scelto. Così report e trace mostrano *perché* è stato scelto.

## Rischi e mitigazioni

| Rischio | Mitigazione |
|---|---|
| Distillatore bloat/leak da testo pagina intera | cap rigido + cleaning riusato; è il rischio #1 |
| Siti JS/SPA → HTML vuoto | rileva testo vuoto → fallback Brave |
| Anti-bot / Cloudflare 403 | UA onesto, accetta 403 → fallback Brave (non combattere) |
| Aggregatore con P.IVA giusta | guardia §4 |
| P.IVA assente sul sito (PMI piccole) | soft match o fallback Brave |
| Latenza | scraping spesso *sostituisce* le 3 chiamate Brave-evidenza → costo netto ~pari, e gratis |
| Parking/for-sale domain | nessun match identità → non confermato |

## Verifica

- `go build ./internal/binocolo/ && go vet && gofmt -l`.
- Re-run **inline KB-on** sulla sessione di calibrazione (`84781682-…`) → confronto col baseline KB-on attuale:
  - `metrics.domain.resolutionRate`, `acceptanceFail`;
  - i 7 acceptance-fail (quanti recuperati);
  - casi-chiave: LINFA (deve restare/risultare scartata, P.IVA), CAMELOT (perimetro, separato), i 3 keeper (recall 3/3 invariato).
- Niente test automatici salvo richiesta esplicita (test rule del repo).

## Esplicitamente FUORI da questo giro

- Domain-guessing (`brand.it`/`.com`/DNS-probe) come fonte di scoperta.
- JS rendering / headless browser.
- Crawler completo / sitemap / robots crawler.

Restano idee per dopo, da valutare solo se gli acceptance-fail *senza candidato vero* pesano ancora dopo questa modifica.

# Binocolo — Piano "Scheda azienda" (pagina analista) + stelle sulla kanban

> Esegue le decisioni del brainstorming 2026-07-07 (quarto tema). Ogni task è
> pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore.
> Riferimenti a simboli/file **verificati sul codice al 2026-07-07** (branch
> `wip/binocolo`, HEAD `8d0e33d`). Se un simbolo citato non esiste più,
> fermarsi e segnalarlo.
>
> **Dipendenze da altri piani**: usa l'endpoint deep per azienda e la
> thesis-reading per (sessione, azienda) di `SGANCIO-FUNZIONI-AZIENDA-PLAN.md`
> (B1, B3) e il modulo deep condiviso (F6). Se quel piano non è ancora
> eseguito, eseguire prima quelle fasi. `INIZIATIVA-RADICE-PLAN.md` è
> presupposto concettuale (non tecnico).
>
> **Riscontro 2026-07-08 (HEAD `ffaffda`)** — le dipendenze risultano GIÀ
> soddisfatte: deep-dive per azienda (`handler.go:145`), thesis-reading
> session-scoped (`handler.go:155-156`, endpoint card `:136-137` ancora vivi
> come adapter), modulo deep condiviso in `apps/binocolo/src/components/deep/`
> e `components/ThesisReadingPanel` estratto. Colonna
> `confidence_at_rating` confermata (`ma_store.go:2562`). Endpoint overview
> (B1 di questo piano) NON ancora esistente: il piano è tutto da eseguire.
> Righe slittate: `MACardProvenance` → `ma_types.go:493`, `Provenances` →
> `:509`, `ListMACardProvenances` → `ma_store.go:3191`, `setTargetRating` →
> `ma_service.go:1800`, registry → `handler.go:146-149`, bm-family → `:182`,
> `CardDrawer` → `IniziativaBoardPage.tsx:758`, `RatingStars` →
> `RicercaDetailPage.tsx:1371`. Le righe citate nel corpo sono quelle del
> 2026-07-07: fidarsi di questo riscontro e comunque cercare per nome.

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **La pagina è per azienda** (`company_key`), il contesto è una lente
   passata in URL (`?ricerca=<sessionId>` / `?iniziativa=<initiativeId>`),
   mai un'identità. Stessa azienda in N ricerche = una pagina, N lenti; la
   storia incrociata (apparizioni, rating, esclusioni, esiti) è parte della
   pagina.
2. **Struttura per domande, 4 blocchi**: (1) cosa fa ed è in tesi? (2) è sana
   e quanto vale? (3) chi la controlla e qual è l'angolo? (4) cosa ne
   sappiamo e cosa ne abbiamo fatto? **Nessuna sezione tecnica**: niente T8
   JSON, niente telemetria, niente provenienza di pipeline.
3. **L'inspector resta com'è**: strumento tecnico parallelo, intatto, coi
   suoi entry point. Il suo eventuale pensionamento NON è dominio di questo
   piano.
4. **Assorbimento del card-dossier a tempo 2** (direzione ratificata):
   tempo 1 = la scheda nasce come destinazione dell'approfondimento; tempo 2
   = il card-dossier diventa la scheda con lente iniziativa (azioni di
   lavorazione incluse). Il tempo 2 è fase finale di questo piano, gated su
   conferma utente dopo l'uso reale del tempo 1.
5. **Si stellina anche sulla scheda** (solo con lente ricerca: il rating è
   session-scoped). Stesso endpoint e stessa semantica di tabella/drawer.
6. **Stelle sulla kanban, derivate a lettura, mai snapshot sulla card**: la
   card mostra il rating più recente (chip ★), drawer/dossier mostrano anche
   score-al-rating e data. Il principio di autonomia della card (la stella
   non governa una card attiva) resta intatto: si mostra, non si governa.
   L'ordinamento per stelle dentro le colonne NON è deciso: non implementare.

## Fatti verificati (su cui il piano si appoggia)

- Il payload del board porta GIÀ le provenienze per card:
  `MAInitiativeCardView.Provenances []MACardProvenance` (`ma_types.go:495-502`)
  con `SessionID`, `SessionTitle`, `Rating`, `ScoreAtRating`, `RatedAt`,
  ordinate `rated_at DESC`, solo rating ≥1 (`ListMACardProvenances`,
  `ma_store.go:3004-3045`). Le stelle kanban sono quasi solo rendering.
  Nota: `ConfidenceAtRating` NON è nella query di provenienza (la colonna
  esiste su `ma_target_rating` — verificare nome esatto, mig del rating):
  estenderla è un'aggiunta piccola (B2).
- Endpoint per-azienda esistenti: deep-dive (SGANCIO B1), registry
  (`handler.go:146-149`), bm-family (`:179`). Route frontend attuali in
  `apps/binocolo/src/routes.tsx` (la scheda NON deve collidere con
  `/azienda`, tampone in pensionamento: usare `/aziende/:companyKey`).
- Il rating si scrive con `POST /binocolo/v1/ma/sessions/{id}/rating`
  (`setTargetRating`, `ma_service.go:1646`) — invariato, la scheda lo riusa.

## Repo-fit

- **Auth**: endpoint nuovi dietro `app_binocolo_access`.
- **Migrazioni**: **NESSUNA** — tutto il piano è codice (i dati sono già
  persistiti; l'overview è composizione a lettura).
- **Osservabilità**: la scheda riusa mutazioni esistenti (rating, deep,
  thesis-reading, registry), già tracciate. L'overview è read-only.
- **Coesistenza**: inspector, TargetPage legacy, funnel, scoring: intatti.

## Regole globali per l'esecutore

1. **Database**: MAI connettersi ai DB degli env, né in lettura.
2. **Test**: NON aggiungere test nuovi. Verifica backend: `cd backend && go
   build ./... && go vet ./internal/binocolo/ && gofmt -l internal/binocolo/`
   (stampa nulla), `go test ./internal/binocolo/` verdi.
3. **Frontend**: `pnpm --filter mrsmith-binocolo exec tsc --noEmit`; smoke
   playwright-cli (cwd=`artifacts/claude`, bypass verificati via grep),
   **sola lettura** sul DB condiviso: niente rating reali, niente lanci deep,
   niente mutazioni negli smoke — verificare presenza/stato dei controlli.
4. **Costi mai in UI utente.**
5. **API**: path Go senza `/api`; URL pubblico `/api/binocolo/v1/...`.
6. **Copy**: italiano B2B asciutto. La pagina si chiama **«Scheda azienda»**.
   Mai gergo pipeline. Per il lavoro UI usare la skill di progetto
   `tintoretto`; leggere `docs/UI-UX.md` prima dei task F.
7. **Non toccare**: inspector (`TargetInspectorPage.tsx` e `inspector/*`)
   salvo l'eventuale import del modulo condiviso F6 se già estratto;
   scoring; routing; gate; `IniziativaCardDossierPage` fino al tempo 2.
8. A fine task: file toccati + verifiche con esito. Mai dichiarare fatto ciò
   che non è verificato.

## Ordine di esecuzione e dipendenze

```
B1 (overview per azienda) ──→ F1 (scheda: shell + 4 blocchi + lenti)
                          ──→ F2 (entry point)
B2 (confidence nella provenienza) ──→ F3 (stelle kanban)  [indipendente da B1/F1]
T2 (assorbimento card-dossier)    [ULTIMO, gated su conferma utente]
```

---

## B1 — Endpoint overview per azienda

**Backend**: `GET /binocolo/v1/ma/companies/{companyKey}/overview`,
read-only, che compone in UNA risposta ciò che alla scheda serve oltre agli
endpoint già esistenti:

- **identità**: dall'ultimo `ma_target` per `company_key` (nome, VAT, tax,
  provincia/comune, ATECO+descrizione, dominio dal registro
  `ma_company_domain` se presente) — query nuova nello store (non esiste un
  loader per company_key cross-sessione: verificare e crearlo);
- **deep**: stato + riferimento (`ListMADeepAnalysis([companyKey])`,
  pattern di `deepDiveCard`) — il contenuto lo carica il frontend dagli
  endpoint esistenti/modulo F6, qui basta lo stato;
- **apparizioni**: elenco (sessione: id, titolo, iniziativa id/titolo,
  stato sessione; targetId; score/bucket; rating con
  score/confidence/data/motivo se esclusione; outcomes) — join su
  `ma_target` + `ma_target_rating` + `ma_target_outcome` + `ma_session`;
  include TUTTI i rating (anche -1 esclusioni: nella storia sono
  informazione, diversamente dalla provenienza card che filtra ≥1);
- **card**: le card esistenti per l'azienda (iniziativa id/titolo, stato,
  esito se chiusa) — query su `ma_initiative_card` per company_key;
- **registry**: già esposto da `GET .../registry` — NON duplicare: la
  scheda chiama l'endpoint esistente.

Ordinamento apparizioni: più recente prima. Risposta tipata
(`MACompanyOverview` in `ma_types.go` + TS in `api/types.ts`). Trace event
non necessario (read-only puro, come gli altri GET).

**Verifica**: build/vet/gofmt/test.

---

## F1 — La scheda: route, shell, 4 blocchi, lenti

**Route**: `aziende/:companyKey` in `apps/binocolo/src/routes.tsx` (NON
`/azienda`, che resta il tampone). Query param: `?ricerca=<sessionId>` o
`?iniziativa=<initiativeId>` (al più una lente attiva; entrambe assenti =
vista globale).

**Pagina**: `apps/binocolo/src/pages/aziende/SchedaAziendaPage.tsx`.
Dati: overview (B1) + registry (endpoint esistente) + con lente ricerca il
target detail esistente (`GET /sessions/{id}/targets/{targetId}` —
il targetId arriva dalle apparizioni dell'overview) + deep via modulo F6.

**I 4 blocchi** (ordine vincolante; densità `full`):

1. **Cosa fa ed è in tesi** — identità completa (nome, forma, ATECO con
   descrizione, luogo, dominio come link); con lente ricerca: verdetto gate
   (pill + `businessFit` + `evidenceFor/Against` di
   `candidateMatchAnalysis`) e **thesis-reading** (lettura + generazione,
   endpoint session-scoped di SGANCIO B3, con staleness). Senza lente
   ricerca: il blocco mostra la sola identità (il gate è per-sessione).
2. **È sana e quanto vale** — il deep completo dal modulo F6 (scorecard,
   riconciliazione, quality flags, valuation con bridge, brief); se assente
   → «Avvia analisi» (endpoint SGANCIO B1, nessuna conferma di costo); se in
   corso → polling (pattern 5s dell'inspector).
3. **Chi la controlla e qual è l'angolo** — soci/controllo, età socio di
   controllo, holding, anzianità, flag di successione: dai dati advanced del
   target (lente ricerca) e/o dal deep quando presente (il modulo F6 e il
   card-dossier arricchito, commit `2dd6d90`, mostrano già
   finanziali/azionisti: riusare, non ricostruire).
4. **Cosa ne sappiamo e cosa ne abbiamo fatto** — registry (fatti + note,
   riusare `CompanyRegistrySection`, che ha già la prop `readOnly` — qui in
   modalità attiva: i fatti registry sono per-azienda, mutazione esistente);
   storia delle apparizioni dall'overview (sessione → link a
   `/ricerche/{id}`, rating dato con score-al-rating e data, esclusioni con
   motivo, esiti); card esistenti (iniziativa → link al board/dossier,
   stato, esito). Con lente iniziativa: la card di quell'iniziativa in
   primo piano (stato, ultimo evento) — in **sola lettura** fino al tempo 2.

**Rating sulla scheda** (decisione 5): con lente ricerca, il componente
stelle (riusare `RatingStars` di `RicercaDetailPage`) nell'header o nel
blocco 1 — stesso `POST /sessions/{id}/rating`, stesso optimistic update
con rollback, stessa modale di esclusione con motivo. Senza lente ricerca:
le stelle non compaiono (il rating è session-scoped).

**Stati degradati**: azienda mai vista dal funnel (overview con apparizioni
vuote — possibile arrivando dal registro o dall'aggiunta manuale): la
scheda regge con identità minima + deep + registry; niente errori.

**Verifica**: `tsc --noEmit`; smoke (sola lettura): scheda di un'azienda con
deep pronto via lente ricerca (blocchi 1-4 popolati, stelle presenti), stessa
scheda senza lente (gate/tesi/stelle assenti, storia presente). Screenshot in
`artifacts/claude/`.

---

## F2 — Entry point

Aggiungere **«Apri scheda ↗»** (nuovo tab, come l'inspector):
- drawer del target in `RicercaDetailPage` (con `?ricerca={sessionId}`) —
  coordinare con F5 di SGANCIO se già eseguita (il link vive nell'header del
  drawer);
- menu riga della tabella risultati (accanto all'entry inspector esistente,
  che NON si tocca);
- drawer della card kanban e card-dossier (con `?iniziativa={initiativeId}`);
- blocco 4 della scheda stessa: le apparizioni linkano le ricerche, le card
  linkano i board (già in F1).

Gli entry point dell'inspector restano tutti intatti (decisione 3).

**Verifica**: `tsc --noEmit`; smoke: ogni entry point naviga con la lente
giusta.

---

## B2 — Confidence nella provenienza card

`ListMACardProvenances` (`ma_store.go:3004`): aggiungere
`confidence_at_rating` alla SELECT (verificare il nome colonna su
`ma_target_rating` nelle migrazioni del rating) e `ConfidenceAtRating` a
`MACardProvenance` (`ma_types.go:484-491`) + TS. Nessun consumer esistente
si rompe (campo additivo, `omitempty`).

**Verifica**: build/vet/gofmt/test.

---

## F3 — Stelle sulla kanban

**File**: `apps/binocolo/src/pages/iniziative/IniziativaBoardPage.tsx`
(card kanban + vista tabella + `CardDrawer`, definizione ~riga 735) e
`IniziativaCardDossierPage.tsx` (header).

- **Card kanban**: chip stelle (★★★/★★/★) da `provenances[0].rating` (già
  ordinate più recente prima — nessuna aggregazione, come da commento del
  tipo). Card senza provenienza (caso limite): nessun chip.
- **Vista tabella del board**: colonna stelle equivalente.
- **`CardDrawer`**: sezione giudizio — stelle + «Score al rating: N» +
  confidenza (B2) + data + sessione di provenienza (titolo, già nel
  payload). Se più provenienze: la più recente in evidenza, le altre in
  lista compatta.
- **Card-dossier header**: stessa sintesi (stelle + score al rating + data).
- NIENTE ordinamento per stelle nelle colonne (decisione 6: non deciso, non
  implementare).

**Verifica**: `tsc --noEmit`; smoke su un board esistente con card stellinate
(sola lettura): chip visibili, drawer con dettaglio giudizio. Screenshot.

---

## T2 — Tempo 2: il card-dossier diventa la scheda (GATED)

> **NON eseguire senza conferma esplicita dell'utente**, da chiedere dopo
> l'uso reale del tempo 1. Direzione ratificata, esecuzione subordinata.

Perimetro (da dettagliare alla ratifica): `IniziativaCardDossierPage` viene
sostituita dalla scheda con lente iniziativa; il blocco 4 passa in modalità
attiva (azioni card: stato, chiusura, riapertura, note, IRL — oggi sul
card-dossier); la rotta `iniziative/:id/dossier/:companyKey` reindirizza a
`aziende/:companyKey?iniziativa=:id`; thesis-reading e IRL della card
raggiungibili dalla scheda. A quel punto le destinazioni per un'azienda sono
una sola.

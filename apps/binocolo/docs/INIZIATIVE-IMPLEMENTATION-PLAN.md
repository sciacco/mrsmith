# Binocolo — Piano di implementazione Iniziative e lavorazione (workstream D3)

> Esegue la specifica di `INIZIATIVE-PRD.md` (iterazione 2, tutta [DECISO]) col wireframe approvato `iniziative-wireframe.html` (stati S1–S7). Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM esecutore. I riferimenti a simboli/file/endpoint sono **verificati sul codice al 2026-07-02, a piano proiezione (`TARGETS-PROJECTION-PLAN.md`) completato**: usare quelli, non inventarne. Se un simbolo citato non esiste più, fermarsi e segnalarlo — non improvvisare un sostituto.

## Regole globali per l'esecutore (valgono per OGNI task)

1. **Leggere prima**: `INIZIATIVE-PRD.md` (ogni task cita le sue sezioni), il wireframe `iniziative-wireframe.html` (fonte di verità per layout e copy dei task F: estrarre i testi verbatim, non riscriverli), `docs/UI-UX.md`, `docs/API-CONVENTIONS.md`. Le decisioni del PRD sono ratificate: non ri-discuterle.
2. **Database**: MAI connettersi ai DB degli env, né in lettura. Schema SOLO come migrazioni idempotenti `deploy/migrations/NNN_*.sql` — questo piano usa **087, 088, 089, 090** (verificare che siano ancora libere con `ls deploy/migrations/` prima di scrivere). Ogni task con migrazione dichiara nel messaggio finale "applicare la NNN prima del deploy".
3. **Test**: NON aggiungere test nuovi. Verifica backend per ogni task: `cd backend && go build ./... && go vet ./internal/binocolo/ && gofmt -l internal/binocolo/` (stampa nulla) e `go test ./internal/binocolo/` verdi. L'interfaccia store è `maWorkspaceStore` (`ma_store.go`, righe ~15–60): ogni metodo aggiunto va replicato nel fake `fakeMAWorkspaceStore` (`ma_service_test.go`) col minimo indispensabile.
4. **Frontend**: type-check SOLO `pnpm --filter mrsmith-binocolo exec tsc --noEmit`. Smoke test UI a fine task F: dev server già attivo (mai riavviarlo), skill `playwright-cli`, **solo lettura** sul DB condiviso (niente card/fatti/note reali senza indicazione dell'utente; per le scritture creare un'iniziativa di prova e segnalarla per pulizia).
5. **Costi mai in UI utente**: l'azione "Avvia analisi completa" conferma senza cifre; `acknowledgeCost` resta meccanica interna.
6. **API**: path Go senza prefisso `/api` (`handle("GET /binocolo/v1/...")` in `handler.go`); URL pubblico `/api/binocolo/v1/...`. Attore: `subject, email := companySearchRefreshActor(r.Context())` (pattern di `handleRateMATarget`, handler.go:365). Company key: SEMPRE `normalizeMACompanyKey`.
7. **Copy**: italiano B2B asciutto, verbatim dal wireframe dove presente. Terminologia ratificata: entità = **Iniziativa**; colonna/etichetta = **Stato** (mai "Stazione"); stati (valori interni → etichette UI): `da_contattare`→Da contattare, `contattata`→Contattata, `in_dialogo`→In dialogo, `approfondimento`→Approfondimento, `offerta`→Offerta, `chiusa`→Chiusa (+ `rimossa`, mai colonna); esiti: `conclusa`/`no_go`/`non_idonea`/`sfumata`/`rimandata` → Conclusa/No-go/Non idonea/Sfumata/Rimandata. Azione dossier: "Avvia analisi completa" → "Analisi in corso…" → "Apri dossier".
8. **Non toccare**: scoring v3, routing v3, gate UC2, TargetPage, TestPage. Il registro azienda e le card NON sono mai letti da gate/routing/scoring (PRD §6, §10): effetti solo di presentazione. L'endpoint legacy `POST /sessions/{id}/outcome` e `addTargetOutcome` (ma_service.go:1312) restano funzionanti com'è (D2 li usa).
9. A fine task: file toccati + comandi di verifica con esito. Mai dichiarare fatto ciò che non è verificato.

## Ordine di esecuzione e dipendenze

```
B1 (iniziative, mig 087) → B2 (card + log, mig 088/089) → B4 (flussi card)
B3 (registro azienda, mig 090) ──────────────────────────↗ (ponte chiusura)
B2, B3 → B5 (badge proiezioni)      B2 → B6 (analisi per-card)
F1 dopo B1 · F2 dopo B4+B6 · F3 dopo B4+B3 · F4 dopo B3 · F5 dopo B1+B5
```

---

## B1 — Entità Iniziativa e aggancio sessioni (PRD §3) — migrazione 087

**Contesto verificato**: `binocolo.ma_session` esiste (FK usate da mig 080/082); `MASessionSummary` (`ma_types.go:402`) è la riga dell'indice ricerche; `ListMASessions` è nello store (`ma_store.go`, la query con `result_count` è a ~riga 131). Lifecycle pattern: `handleArchiveMASession`/`updateSessionLifecycle` (`ma_service.go:255`).

**Passi**:
1. **Migrazione `087_binocolo_ma_initiative.sql`** (idempotente, header commento nello stile della 082):
   - `CREATE TABLE IF NOT EXISTS binocolo.ma_initiative (id uuid PRIMARY KEY, title text NOT NULL, description text NOT NULL DEFAULT '', created_by_subject text NOT NULL DEFAULT '', created_by_email text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), archived_at timestamptz, archived_by_subject text, archived_by_email text);`
   - `ALTER TABLE binocolo.ma_session ADD COLUMN IF NOT EXISTS initiative_id uuid REFERENCES binocolo.ma_initiative(id) ON DELETE SET NULL;` + indice parziale su `initiative_id WHERE initiative_id IS NOT NULL`.
2. Tipi in `ma_types.go`: `MAInitiative {ID, Title, Description, CreatedByEmail, CreatedAt, UpdatedAt, ArchivedAt *time.Time}` con tag json camelCase; `MAInitiativeSummary` = MAInitiative + `Counts map[string]int` (per stato; vuota finché B2 non aggiunge la join) + `LastActivityAt *time.Time` + `SessionCount int`.
3. Store (+ interfaccia + fake): `CreateMAInitiative`, `GetMAInitiative`, `ListMAInitiatives(includeArchived bool)`, `UpdateMAInitiativeLifecycle(archive/restore)`, `SetMASessionInitiative(ctx, sessionID, initiativeID string)` (stringa vuota = sgancio, scrive NULL).
4. Service: validazioni (title obbligatorio, `cleanText` 120; description `cleanText` 500); l'aggancio esige sessione operativa (`ensureMASessionOperational`) e iniziativa non archiviata. **Il backfill delle card arriva in B2**: qui l'aggancio scrive solo la colonna.
5. Endpoint in `handler.go` (blocco nuovo accanto alle rotte ma):
   - `GET /binocolo/v1/ma/initiatives` → `{items: []MAInitiativeSummary}` (query param `archived=1` per l'archivio)
   - `POST /binocolo/v1/ma/initiatives` `{title, description}` → MAInitiative
   - `POST /binocolo/v1/ma/initiatives/{id}/archive` e `/restore`
   - `POST /binocolo/v1/ma/sessions/{id}/initiative` `{initiativeId}` (vuoto = sgancio)
6. `MASessionSummary` += `InitiativeID string \`json:"initiativeId,omitempty"\`` e `InitiativeTitle string \`json:"initiativeTitle,omitempty"\``; `ListMASessions` fa LEFT JOIN su ma_initiative. (Il chip in UI arriva con F5.)

**Done when**: build/vet/test verdi; curl locale: creare iniziativa, listarla, agganciare una sessione di prova, vederla nel summary. **Applicare la 087 prima del deploy.**

## B2 — Card di lavorazione e log unico (PRD §4.1–4.2, §5) — migrazioni 088/089 — dipende da B1

**Contesto verificato**: hook = `setTargetRating` (`ma_service.go:1271`), che ha già `session` caricata via `GetMASessionState` e il rating validato (`validMARating`: -1/1..3; 0 = clear). `MATargetOutcome` (`ma_types.go:301`) e `InsertMATargetOutcome` (`ma_store.go:1587`); CHECK eventi attuale in mig 080:33; costanti `maOutcomeContattato/BuonLead/NoGo` (`ma_types.go:246`). Pattern DO-block per ricreare un CHECK: mig 083/086.

**Passi**:
1. **Migrazione `088_binocolo_ma_initiative_card.sql`**: `CREATE TABLE IF NOT EXISTS binocolo.ma_initiative_card (initiative_id uuid NOT NULL REFERENCES binocolo.ma_initiative(id) ON DELETE CASCADE, company_key text NOT NULL, company_name text NOT NULL DEFAULT '', vat_code text NOT NULL DEFAULT '', tax_code text NOT NULL DEFAULT '', province text NOT NULL DEFAULT '', state text NOT NULL DEFAULT 'da_contattare' CHECK (state IN ('da_contattare','contattata','in_dialogo','approfondimento','offerta','chiusa','rimossa')), esito text CHECK (esito IN ('conclusa','no_go','non_idonea','sfumata','rimandata')), created_from_session uuid REFERENCES binocolo.ma_session(id) ON DELETE SET NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), closed_at timestamptz, PRIMARY KEY (initiative_id, company_key));` + indice su `company_key` (per marker collisione e badge B5).
2. **Migrazione `089_binocolo_ma_outcome_evolution.sql`** (PRD §5, ratificato):
   - `ALTER TABLE binocolo.ma_target_outcome ALTER COLUMN session_id DROP NOT NULL;`
   - FK sessione: drop + ricrea con `ON DELETE SET NULL` (DO-block su `pg_constraint`, nome constraint da verificare col dump: è la FK implicita di mig 080:24).
   - `ADD COLUMN IF NOT EXISTS initiative_id uuid REFERENCES binocolo.ma_initiative(id) ON DELETE SET NULL;` + `ADD COLUMN IF NOT EXISTS payload jsonb NOT NULL DEFAULT '{}'::jsonb;`
   - CHECK eventi ricreato: `event IN ('contattato','buon_lead','no_go','card_creata','card_rimossa','card_riaperta','stato','nota','chiusura')`.
   - Indice `(initiative_id, company_key) WHERE initiative_id IS NOT NULL`.
3. Go: costanti nuovi eventi (`maEventCardCreata = "card_creata"`, ecc.); `MATargetOutcome` += `InitiativeID string \`json:"initiativeId,omitempty"\`` e `Payload json.RawMessage \`json:"payload,omitempty"\``; `InsertMATargetOutcome`/`loadMAOutcomes` aggiornati alle colonne nuove (session_id ora nullable: scrivere NULL quando vuoto, non stringa vuota).
4. Tipo `MAInitiativeCard` + store: `GetMAInitiativeCard`, `UpsertMAInitiativeCard`, `ListMAInitiativeCards(initiativeID)`, `ListMAActiveCardsByCompany(companyKeys []string)` (per collisioni/badge: attive = state NOT IN ('chiusa','rimossa')).
5. **Hook in `setTargetRating`** (PRD §4.1–4.3): dopo l'upsert rating riuscito, se `input.Rating >= 1` e `session.InitiativeID != ""` → `ensureInitiativeCard`: se la card non esiste, crearla (stato `da_contattare`, snapshot nome/vat/provincia dal target della sessione se reperibile via `ListMATargetRows`, altrimenti dal solo company key) + evento `card_creata` (payload `{"sessionId": ..., "rating": N}`); se esiste con state `chiusa`/`rimossa` → state `da_contattare` + evento `card_riaperta`. Se esiste attiva: nessun effetto (autonomia §4.2). Errori dell'hook: propagati (il rating non deve riuscire con la card rotta).
   - `MASession` (`ma_types.go:459`) += `InitiativeID string \`json:"initiativeId,omitempty"\`` e `loadMASession`/`GetMASessionState` lo caricano (serve all'hook senza query extra).
6. **Backfill** (PRD §3.2): nel service dell'aggancio (B1), dopo `SetMASessionInitiative` con id non vuoto: `loadMARatings(sessionID)` (già esistente, `ma_store.go:1520`) → per ogni rating ≥1 → `ensureInitiativeCard` (stessa funzione dell'hook).

**Done when**: build/vet/test verdi; su una sessione di prova agganciata, una ★ crea la card (verifica via endpoint B4 o SQL fornito all'utente da eseguire). **Applicare 088 e 089 prima del deploy** (l'hook referenzia le tabelle).

## B3 — Registro azienda (PRD §6) — migrazione 090

**Contesto verificato**: pattern tabella company-level = `binocolo.ma_company_domain` (mig 082: PK company_key, snapshot vat/tax/name, created_by, indici parziali su vat/tax). Il registro è NUOVO e separato dal log eventi (PRD §2: piani distinti).

**Passi**:
1. **Migrazione `090_binocolo_ma_company_registry.sql`**:
   - `binocolo.ma_company_fact (id uuid PRIMARY KEY, company_key text NOT NULL, vat_code text NOT NULL DEFAULT '', tax_code text NOT NULL DEFAULT '', company_name text NOT NULL DEFAULT '', kind text NOT NULL CHECK (kind IN ('non_vende','in_trattativa_altrui','da_evitare','gia_cliente','partner')), note text NOT NULL DEFAULT '', created_by_subject text NOT NULL DEFAULT '', created_by_email text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), revoked_at timestamptz, revoked_by_subject text, revoked_by_email text, revoke_note text NOT NULL DEFAULT '');`
   - Unique parziale: `CREATE UNIQUE INDEX IF NOT EXISTS ma_company_fact_active_idx ON binocolo.ma_company_fact (company_key, kind) WHERE revoked_at IS NULL;` + indice su company_key.
   - `binocolo.ma_company_note (id uuid PRIMARY KEY, company_key text NOT NULL, body text NOT NULL, created_by_subject text NOT NULL DEFAULT '', created_by_email text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now());` + indice su company_key.
2. Tipi `MACompanyFact`, `MACompanyNote`, `MACompanyRegistry {Facts []MACompanyFact, Notes []MACompanyNote}` (facts = attivi + revocati, la UI mostra gli attivi e può rivelare lo storico). Store (+ fake): `InsertMACompanyFact` (errore chiaro su violazione dell'unique attivo → 400 "fatto già presente"), `RevokeMACompanyFact(id)`, `InsertMACompanyNote`, `GetMACompanyRegistry(companyKey)`, `ListMACompanyFactsActive(companyKeys []string) (map[string][]string, error)` (per i badge B5: company_key → kinds attivi).
3. Service: validazioni (kind nel set chiuso; note `cleanText` 500; body nota `cleanText` 1000, non vuoto); revoca solo su fatto attivo.
4. Endpoint:
   - `GET /binocolo/v1/ma/companies/{companyKey}/registry` → MACompanyRegistry
   - `POST /binocolo/v1/ma/companies/{companyKey}/registry/facts` `{kind, note}`
   - `POST /binocolo/v1/ma/companies/{companyKey}/registry/facts/{factId}/revoke` `{note}`
   - `POST /binocolo/v1/ma/companies/{companyKey}/registry/notes` `{body}`
   (companyKey nel path va normalizzato con `normalizeMACompanyKey`; vat/tax/name snapshot: se disponibili dal chiamante passarli nel body come opzionali, altrimenti vuoti — la 082 fa lo stesso.)

**Non fare**: nessuna lettura del registro da gate/routing/scoring; nessun effetto oltre la presentazione.

**Done when**: build/vet/test verdi; curl: fatto creato → GET lo mostra attivo → revoca → GET lo mostra revocato; doppio fatto attivo stesso kind → 400. **Applicare la 090 prima del deploy.**

## B4 — Flussi card: board, diario, transizioni, chiusura, rimozione (PRD §4.3–4.4, §6 ponte) — dipende da B2, B3

**Contesto verificato**: `setTargetRating` riusabile per la correzione-stella (già valida -1 + reason, scrive `ma_target_rating` con snapshot); `ListMADeepAnalysis(ctx, companyKeys)` (`ma_store.go:54`) dà lo stato dossier per il bottone a 3 stati; le provenienze = righe `ma_target_rating` delle sessioni dell'iniziativa (rating + `score_at_rating`, mig 080).

**Passi**:
1. **`GET /binocolo/v1/ma/initiatives/{id}`** → `{initiative, sessions: [MASessionSummary agganciate], cards: [...]}`. Ogni card: campi tabella + `dossierStatus` (da `ListMADeepAnalysis` batch: assente/failed → `none`, queued/running → `working`, ready → `ready`), `collisions: [{initiativeId, title}]` (da `ListMAActiveCardsByCompany` sulle altre iniziative), `registryFacts: []string` (kinds attivi, da B3), `provenances: [{sessionId, sessionTitle, rating, scoreAtRating}]` (query dedicata: rating delle sessioni con questo initiative_id per le company delle card — UNA query batch, non N).
2. **`GET /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/events`** → diario: eventi con (`initiative_id = id` AND company_key) **OR** (`session_id IN (sessioni dell'iniziativa)` AND company_key) — così i `contattato`/`no_go` storici emessi da D2 sulle sessioni agganciate appaiono nel diario. Ordine cronologico discendente.
3. **`POST .../cards/{companyKey}/state`** `{state}`: valida `state` nei 5 stati attivi (per `chiusa` usare /close; `rimossa` usare /remove); scrive card + evento `stato` payload `{"from","to"}`. Transizioni libere (nessun vincolo di sequenza).
4. **`POST .../cards/{companyKey}/close`** `{esito, note, registerFacts: []}`: esito nel set 5; card → `chiusa` + esito + closed_at; evento `chiusura` payload `{"esito"}` + note; `registerFacts` ammesso SOLO per esito `no_go`/`rimandata` e SOLO con kinds `non_vende`/`in_trattativa_altrui` (PRD §4.4/§6: `non_idonea` mai al registro) → per ciascuno `InsertMACompanyFact` (idempotente sul 400 fatto-già-attivo: ignorare e proseguire, la chiusura non deve fallire per un fatto già registrato).
5. **`POST .../cards/{companyKey}/remove`** `{correctRating bool, reason}`: card → `rimossa`; evento `card_rimossa`; se `correctRating` → `setTargetRating` sulla **sessione di provenienza più recente con rating ≥1** (dalle provenances) con `{companyKey, rating: -1, reason}` — riusare il metodo, non duplicarlo. Se la sessione di provenienza non è operativa (archiviata), saltare la correzione e riportarlo nel payload di risposta, non fallire la rimozione.
6. **`POST .../cards/{companyKey}/reopen`**: solo da `chiusa`/`rimossa` → `da_contattare` + evento `card_riaperta` payload `{"manual": true}`.
7. **`POST .../cards/{companyKey}/note`** `{body}`: evento `nota` (body `cleanText` 1000, non vuoto) — la nota di diario del composer S4. Scrive SOLO il log eventi (mai il registro azienda: PRD §2, generi diversi).
8. Guardie comuni: iniziativa esistente e non archiviata per le scritture; card esistente; 404 su assenza.

**Done when**: build/vet/test verdi; ciclo completo via curl su iniziativa di prova: stato → chiusura no_go con fatto → riapertura → rimozione con correzione stella; diario coerente a ogni passo.

## B5 — Badge e marker nelle proiezioni (PRD §6.1, R-D3-7) — dipende da B2, B3

**Contesto verificato**: `MATargetRow` (`ma_types.go:812`) e il suo loader `ListMATargetRows` (interfaccia `ma_store.go:23`) sono la fonte della tabella risultati D2; il service che decora le righe è `sessionTargetRows` (cercarlo in `ma_service.go`, introdotto dal piano proiezione).

**Passi**:
1. `MATargetRow` += `RegistryFacts []string \`json:"registryFacts,omitempty"\`` (kinds attivi) e `InLavorazione []MACardMarker \`json:"inLavorazione,omitempty"\`` con `MACardMarker {InitiativeID, InitiativeTitle string}` (card attive di qualunque iniziativa).
2. Nel service delle righe (NON nel loader SQL, per tenerlo semplice): raccogliere i company_keys delle righe → due chiamate batch `ListMACompanyFactsActive` + `ListMAActiveCardsByCompany` → decorare. Zero effetti su bucket/score.
3. Il `verification-queue` e il `gated-progress` NON cambiano.

**Done when**: build/vet/test verdi; `GET /sessions/{id}/targets` mostra `registryFacts`/`inLavorazione` su una company di prova con fatto/card.

## B6 — Analisi completa per-card (PRD §7) — dipende da B2

**Contesto verificato**: `EnqueueMADeepAnalysis(ctx, companyKey, vatCode, taxCode, email)` (`ma_store.go:55`) è già per-azienda; il worker (`ma_deep_worker.go`) drena la coda; `deepDive` di sessione (`ma_service.go:1906`) mostra il gate di spesa esistente (`loadPricing` + `acknowledgeCost`): leggerlo e replicarne la logica per count=1, non duplicare i calcoli in modo divergente.

**Passi**:
1. Service `deepDiveCard(ctx, initiativeID, companyKey, ack bool, email string)`: card esistente (da B2, con vat/tax snapshot); `ListMADeepAnalysis([companyKey])`: se `ready` → no-op (rispondere lo stato); se queued/running → no-op; altrimenti gate di spesa come nel `deepDive` di sessione con 1 azienda chargeable → `EnqueueMADeepAnalysis`.
2. Endpoint `POST /binocolo/v1/ma/initiatives/{id}/cards/{companyKey}/deep-dive` body `{acknowledgeCost}` → `{dossierStatus}`.
3. Il bottone a 3 stati in UI legge `dossierStatus` dal board detail (B4); il polling del board mentre `working` copre l'aggiornamento (nessun endpoint di polling dedicato).

**Done when**: build/vet/test verdi. ⚠️ L'enqueue reale spende (€0.30/azienda IT-full): in dev NON accodare aziende reali senza indicazione dell'utente — verificare il percorso no-op (già ready / già in coda) e fermarsi.

---

## F1 — Rotte, nav, indice iniziative (wireframe S1) — dipende da B1

**Contesto verificato**: `apps/binocolo/src/routes.tsx` (rotte attuali: ricerche, target, azienda, ricerca-web, config, test); la voce di navigazione vive in `App.tsx`; tipi client in `apps/binocolo/src/api/types.ts`; pattern pagine in `apps/binocolo/src/pages/ricerche/` (componenti lean, `@mrsmith/ui`, CSS module dedicato).

**Passi**:
1. Rotte: `iniziative` (indice) e `iniziative/:id` (board, F2); voce nav "Iniziative" accanto a "Ricerche" in `App.tsx`.
2. Tipi client: `MAInitiative`, `MAInitiativeSummary`, response types (specchiare i tag json di B1).
3. **IniziativePage** (S1): lista da `GET /binocolo/v1/ma/initiatives` — riga = titolo, descrizione, conteggi per stato (pills, click → board filtrato), n ricerche, ultima attività; link "Archivio (N)" che alterna la lista archiviata; CTA "Nuova iniziativa" → modal light (titolo + descrizione opzionale, copy S1) → POST → naviga al board. Empty state verbatim dal wireframe.

**Verifica**: tsc + smoke (aprire `/iniziative`, creare un'iniziativa di prova, segnalarla per pulizia).

## F2 — Board: kanban, tabella, drawer (wireframe S2–S4) — dipende da B4, B6

**Passi**:
1. **IniziativaBoardPage** (`/iniziative/:id`): header (titolo, descrizione, chips ricerche agganciate + "+ Aggancia ricerca" → modal con le sessioni non agganciate via `GET /ma/sessions` + `POST /sessions/{id}/initiative`), toggle Kanban|Tabella (**viste indipendenti**, filtri propri — PRD §8).
2. **Kanban (S2)**: 6 colonne a larghezza fissa (~240px), board a scroll orizzontale, colonne collassabili a rail verticale (nome+conteggio); vuote e "Chiusa" nascono collassate; stato di collasso in localStorage per utente. Card: nome, provincia, badge registro (`registryFacts`), marker collisione (`collisions`), bottone dossier a 3 stati (`dossierStatus`: none → "Avvia analisi completa" → conferma senza cifre → POST B6; working → "Analisi in corso…" pulse; ready → "Apri dossier" → naviga `/azienda?vat=`). Drag tra colonne → `POST .../state` (drop su "Chiusa" apre il flusso F3). Colonna Chiusa: card compatte con pill esito.
3. **Tabella (S3)**: colonne Azienda / Prov. / Stato / Dossier / Registro / Ultima attività; filtri Stato + Esito + ricerca testuale, indipendenti dal kanban.
4. **Drawer card (S4)**: selettore stato (pill; "Chiusa…" apre F3), marker collisione, provenienze (sessione + stelle + score al momento), scheda azienda read-only (badge fatti + ultima nota + link "Gestione dal dossier ↗" → `/azienda?vat=`), azione analisi, diario (timeline eventi da `GET .../events` con autore/data) + composer nota di diario → `POST .../cards/{companyKey}/note` (B4 passo 7).
5. Polling del board ogni 5s SOLO quando esiste almeno una card con `dossierStatus === 'working'`; altrimenti niente polling.

**Verifica**: tsc + smoke read-only sull'iniziativa di prova di F1 (kanban, collasso colonne, tabella, drawer; nessuna azione di spesa).

## F3 — Chiusura e rimozione (wireframe S5) — dipende da B4, B3

**Passi**:
1. **Modal chiusura**: 5 esiti radio con descrizioni verbatim S5; il blocco "Registra anche nel registro azienda (vale per tutte le iniziative)" con i due checkbox (`non_vende`, `in_trattativa_altrui`) compare SOLO per No-go e Rimandata (mai per Non idonea — nota 1 del wireframe); nota; → `POST .../close`.
2. **Modal rimozione**: copy S5 ("La rimozione è per le card create per errore: nessun verdetto viene registrato."), checkbox correzione stella con campo motivo → `POST .../remove`. Se la risposta segnala correzione saltata (sessione non operativa), toast informativo.
3. **Riapertura**: dalle card chiuse/rimosse (drawer) → `POST .../reopen`.
4. Dopo ogni azione: refetch board + diario.

**Verifica**: tsc + smoke sull'iniziativa di prova (chiusura con esito, riapertura, rimozione; il ponte registro solo dove previsto).

## F4 — Registro azienda nel dossier (wireframe S6) — dipende da B3

**Contesto verificato**: `CompanyDossierPage.tsx` (`/azienda?vat=`) è la pagina dossier esistente — aggiungere sezioni, non ristrutturarla.

**Passi**:
1. Sezione "Registro azienda": fatti attivi (badge + nota + autore/data + "Revoca" con conferma), storico revocati dietro un toggle sobrio; "+ Registra fatto" → menu dei 5 tipi + nota → POST.
2. Sezione "Note d'azienda": timeline append-only + composer "Aggiungi nota d'azienda…".
3. Company key per le chiamate: derivarlo come fa il resto del dossier (verificare come la pagina risolve la company da `vat` prima di scrivere il client).

**Verifica**: tsc + smoke (fatto di prova su company fittizia concordata, revoca, nota; segnalare per pulizia).

## F5 — Superfici esistenti (wireframe S7) — dipende da B1, B5

**Passi**:
1. **D1** (`NuovaRicercaPage.tsx`): card "Iniziativa (opzionale)" — dropdown iniziative attive + "+ Nuova iniziativa" inline + hint verbatim S7; la creazione sessione include l'aggancio (`POST /sessions/{id}/initiative` subito dopo la create, o campo nel body della create se B1 l'ha previsto — verificare B1 e imitare).
2. **Indice `/ricerche`** (`RicerchePage.tsx`): chip iniziativa sulla riga (da `initiativeTitle` del summary); sulle righe orfane, menu "Aggancia a iniziativa…" (il backfill è server-side, B2).
3. **D2** (`RicercaDetailPage.tsx`): nella tabella risultati, badge sotto il nome da `registryFacts` (etichette: Non vende/In trattativa con altri/Da evitare = warning; Già cliente/Partner = info) e marker `inLavorazione` ("In lavorazione · <titolo>"). Nessun altro cambiamento a D2.

**Verifica**: tsc + smoke read-only (chip, badge su sessione esistente con la company di prova di F4).

---

## Fuori perimetro di questo piano (esplicito, per evitare drift)

- Derivazione automatica `gia_cliente` da Mistra/Grappa (deciso: dichiarato a mano in v1).
- Batch deep-dive dal board (v2, PRD §11).
- Assegnatario per card, reminder/task/scadenze (non-CRM).
- Motivi "sito irraggiungibile", tempo-2 TargetPage, e tutto il fuori-perimetro dei piani precedenti.
- Qualunque lettura di card/registro da gate, routing, scoring.

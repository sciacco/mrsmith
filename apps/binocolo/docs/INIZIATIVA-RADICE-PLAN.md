# Binocolo — Piano "Iniziativa radice" (ricerca sempre dentro un'iniziativa)

> Esegue le decisioni del brainstorming 2026-07-07 (ratificate: non ri-discuterle).
> Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM
> esecutore. I riferimenti a simboli/file sono **verificati sul codice al
> 2026-07-07** (branch `wip/binocolo`, HEAD `8d0e33d`): usarli, non inventarne.
> Se un simbolo citato non esiste più, fermarsi e segnalarlo.

## Decisioni ratificate (fonte di verità)

1. **Una ricerca senza iniziativa non esiste.** Ogni `ma_session` nasce dentro
   un'iniziativa: l'utente seleziona un'iniziativa esistente **oppure** ne crea
   una nuova contestualmente. La nuova ha titolo manuale **oppure** automatico
   `TGT: <titolo sintetizzato>`; il prefisso `TGT:` è **fisso** e marca i soli
   titoli auto-assegnati (nessun flag nel dato: è convenzione di naming).
2. **Selettore in testa** alla pagina Nuova ricerca, **prima** del prompt, per
   comunicare subito che l'iniziativa è obbligatoria. Default preselezionato:
   **«Nuova iniziativa · titolo automatico»** — chi non tocca nulla ha lo stesso
   percorso veloce di oggi.
3. **Ciclo di vita accoppiato alla nascita**: archiviare / cestinare / purgare /
   ripristinare una ricerca esegue la stessa operazione a cascata
   sull'iniziativa **SE** (a) è l'unica ricerca collegata (le purgate non
   contano) **E** (b) l'iniziativa non ha alcuna card (nessuna azienda
   stellinata, in qualunque stato — anche chiusa/rimossa blocca la cascata).
   Ripristino simmetrico alla stessa condizione. Superata la soglia (altre
   ricerche o ≥1 card), l'iniziativa vive di vita propria.
4. **Cestino/purge iniziativa = tombstone, mai hard-delete** (stesso pattern di
   `ma_session`, `ma_store.go:740-748`): nessuna riga figlia viene cancellata.
   Questo scavalca il blocco di `INIZIATIVE-REMEDIATION-PLAN.md` §7 (D-A), che
   vietava il **delete fisico** per FK incoerenti (card CASCADE vs outcome
   SET NULL): con il tombstone quelle FK non vengono mai esercitate.
5. **Sgancio vietato, spostamento permesso**: `setSessionInitiative` con
   `initiativeID == ""` diventa illegale; spostare una ricerca su un'altra
   iniziativa resta permesso (nessuna cascata sull'iniziativa di origine:
   azione manuale e consapevole).
6. **Bonifica = opzione A**: (1) l'utente purga a mano il cestino dall'UI;
   (2) migrazione di backfill: un'iniziativa `TGT: <titolo sessione>` verbatim
   per ogni sessione orfana superstite, **stato ereditato** dalla sessione;
   (3) a orfani zero verificati, migrazione separata che rende
   `ma_session.initiative_id` NOT NULL.
7. Comportamento accettato: una sessione creata e abbandonata prima
   dell'esecuzione crea comunque la sua iniziativa `TGT:`; la cascata al
   cestino/purge della sessione la riassorbe.

## Repo-fit (checklist `docs/IMPLEMENTATION-PLANNING.md`)

- **Runtime/dev/deploy**: nessuna nuova app, nessun env nuovo; tutto in
  `backend/internal/binocolo` + `apps/binocolo`.
- **Auth**: endpoint nuovi dietro `app_binocolo_access` come gli esistenti
  (`handle(...)` in `RegisterRoutes`, `handler.go:112-183`).
- **Data contract**: identificatori esistenti (`ma_session.initiative_id`,
  mig 087; card = `(initiative_id, company_key)`).
- **Migrazioni**: `.sql` idempotenti in `deploy/migrations/`, applicate a mano
  dall'utente. **Numerazione: la prossima libera è 102** (ultima presente:
  `101_anisetta_mrsmith_binocolo_ma_deep_brief_prompt_v4_2.sql`) —
  ricontrollare al momento dell'esecuzione.
- **Osservabilità**: ogni mutazione nuova emette trace event
  (pattern `maTraceEventWrite`); errori sanitizzati via `maFailure`.
- **Coesistenza**: scoring, gate UC2, funnel gated, worker **non cambiano**.

## Regole globali per l'esecutore (valgono per OGNI task)

1. **Database**: MAI connettersi ai DB degli env (`ANISETTA_DSN`, ecc.), né in
   lettura. Schema changes SOLO come migrazioni idempotenti.
2. **Test**: NON aggiungere test nuovi (regola repo). Verifica backend per ogni
   task: `cd backend && go build ./... && go vet ./internal/binocolo/ &&
   gofmt -l internal/binocolo/` (stampa nulla) e `go test ./internal/binocolo/`
   verdi (se l'interfaccia store si estende, aggiornare i fake esistenti al
   minimo). Se i test falliscono per sandbox su porte httptest, rilanciare
   fuori sandbox.
3. **Frontend**: type-check SOLO `pnpm --filter mrsmith-binocolo exec tsc
   --noEmit`. Smoke UI a fine task F: dev server già attivo (mai
   killare/riavviare), skill `playwright-cli` con cwd=`artifacts/claude`,
   bypass verificati via grep (`VITE_DEV_AUTH_BYPASS`, `SKIP_KEYCLOAK`),
   **solo lettura** sul DB condiviso (niente run a pagamento; creare/cestinare
   sessioni di prova senza eseguirle è permesso ma va dichiarato).
4. **Costi mai in UI utente.**
5. **API**: path Go senza prefisso `/api` (`handle("POST /binocolo/v1/...")` in
   `handler.go`); URL pubblico `/api/binocolo/v1/...`. Consultare
   `docs/API-CONVENTIONS.md`.
6. **Copy**: italiano B2B asciutto (sostantivi secchi, no prima persona, no
   domande). Il gergo interno (tombstone, cascade, orfana) NON compare in UI.
7. **Hot reload**: il backend gira sotto `air`. Ogni task con migrazione deve
   degradare in modo innocuo finché la migrazione non è applicata, oppure
   dichiarare "applicare la migrazione NNN prima del deploy".
8. **Non toccare**: scoring v3, routing, gate UC2, funnel gated
   (`ma_gated_search_job.go`), worker (`ma_job_worker.go`,
   `ma_deep_worker.go`), TargetPage legacy. Niente refactor opportunistici.
9. A fine task: file toccati + comandi di verifica con esito. Mai dichiarare
   fatto ciò che non è verificato.

## Ordine di esecuzione e dipendenze

```
B1 (mig 102 + stati iniziativa) ──→ B2 (creazione atomica) ──→ F1 (selettore)
                                ──→ B3 (cascata)           ──→ F3 (UI cestino iniziative)
B2b (rinomina) [indipendente]                              ──→ F2 (sposta)
B4 (sgancio vietato)  [dopo B2]
B5 (mig 103 backfill) [dopo B1; l'utente purga il cestino PRIMA di applicarla]
B6 (mig 104 NOT NULL) [ultima; applicare solo a orfani zero verificati]
F4 (smoke complessivo) [dopo F1–F3]
```

---

## B1 — Stati cestino/purge sull'iniziativa (tombstone) + visibilità

**Migrazione 102** `102_binocolo_ma_initiative_lifecycle.sql` (idempotente):
- `ALTER TABLE binocolo.ma_initiative ADD COLUMN IF NOT EXISTS` per:
  `deleted_at timestamptz`, `deleted_by_subject text`, `deleted_by_email text`,
  `purged_at timestamptz`, `purged_by_subject text`, `purged_by_email text`
  (specchio delle colonne di `ma_session`, mig 053 — verificarne i nomi esatti
  prima di scrivere la migrazione).

**Backend:**
- `ma_store.go`: estendere `UpdateMAInitiativeLifecycle` (oggi solo
  archive/restore, `ma_store.go:407-449`) con `delete`/`purge`/`restore`
  specchiando la semantica di `UpdateMASessionLifecycle`
  (`ma_store.go:690-760`): delete solo da non-cestinata, purge solo da
  cestinata, restore riporta dal cestino o dall'archivio; purge irreversibile.
  Rimuovere il commento "initiatives have no delete/purge state in v1"
  (`ma_store.go:407-409`).
- `ListMAInitiatives(ctx, includeArchived bool)` (`ma_store.go:301`) →
  `ListMAInitiatives(ctx, visibility string)` con semantica identica a
  `normalizeMASessionVisibility` (`ma_service.go:397-408`): `active` (default),
  `archived`, `deleted`; le purgate non compaiono mai. Aggiornare la clausola
  WHERE e il fake dei test.
- `MAInitiative`/`MAInitiativeSummary` (`ma_types.go`): aggiungere
  `DeletedAt`/`PurgedAt` (+ by subject/email) e serializzazione JSON camelCase
  coerente con `MASession`.
- `ensureMAInitiativeOperational` (`ma_service.go:425-430`): rifiutare anche
  `DeletedAt`/`PurgedAt` valorizzati (nuovi errori specchio di
  `errMASessionDeleted`).
- `handler.go`: aggiungere `DELETE /binocolo/v1/ma/initiatives/{id}` (soft
  delete) e `POST /binocolo/v1/ma/initiatives/{id}/purge`, specchiando i
  corrispondenti endpoint sessione (cercarli in `handler.go:112-183`);
  `GET /binocolo/v1/ma/initiatives` accetta `?visibility=` (mantenere
  compatibilità con l'attuale `?archived=1` di `IniziativePage.tsx:39-44`
  finché F3 non lo sostituisce, poi rimuoverla in F3).
- `getInitiativeBoard` (`ma_service.go:1815`): un'iniziativa cestinata/purgata
  non è consultabile (404 come le sessioni; verificare il comportamento
  sessione equivalente e specchiarlo).
- Trace event per ogni transizione (pattern esistente di archive in
  `updateInitiativeLifecycle`, `ma_service.go:469-486`).

**Degradazione pre-migrazione**: se le colonne non esistono le query nuove
falliscono → guardia come per mig già gestite altrove non necessaria: le rotte
nuove sono inedite (nessun utente le chiama prima del deploy della migrazione);
`ListMAInitiatives` però è caldo — la SELECT estesa richiede la 102: dichiarare
"applicare la migrazione 102 prima del deploy".

**Verifica**: build/vet/gofmt/test verdi.

---

## B2 — Creazione atomica sessione+iniziativa con titolo `TGT:`

**Backend:**
- `MACreateSessionRequest` (`ma_types.go:335-340`): aggiungere
  `InitiativeID string`, `NewInitiativeTitle string` (mutuamente esclusivi;
  entrambi vuoti = nuova iniziativa con titolo automatico).
- `createSession` (`ma_service.go:977`): dopo la generazione della strategia e
  PRIMA di `CreateMASession`:
  1. se `InitiativeID != ""`: caricare con `GetMAInitiative` e validare con
     `ensureMAInitiativeOperational` (già esteso in B1);
  2. altrimenti creare l'iniziativa via `createInitiative`
     (`ma_service.go:434`): titolo = `NewInitiativeTitle` se presente, altrimenti
     `"TGT: " + title` dove `title` è lo stesso valore già calcolato per la
     sessione (`strategy.Title` con fallback `titleFromPrompt`,
     `ma_service.go:1032-1035`) — **nessuna chiamata LLM aggiuntiva**; rispettare
     il limite `cleanText(title, 120)` di `createInitiative` (troncare la
     sintesi, non il prefisso);
  3. passare l'`initiative_id` a `CreateMASession` così la sessione nasce già
     ancorata (estendere `maSessionCreate`/`CreateMASession` per scrivere la
     colonna in INSERT — oggi l'ancora si scrive solo via
     `SetMASessionInitiative`).
  In caso di fallimento della creazione sessione dopo la creazione iniziativa,
  non serve rollback sofisticato: un'iniziativa vuota è innocua e riassorbibile
  (decisione 7); registrare comunque un trace event di warning.
- Il vecchio percorso "sessione senza iniziativa" resta accettato **solo** fino
  a B6/F1 (transizione): se la request non porta né `InitiativeID` né titolo e
  il chiamante è il flusso legacy, il default è comunque la creazione
  automatica — quindi di fatto da questo task in poi nessuna sessione nuova
  nasce orfana.
- La risposta di `createSession` deve includere l'iniziativa risolta
  (id + titolo) così il frontend la mostra senza fetch aggiuntivo (estendere
  `MASessionDetail` solo se i campi non ci sono già — verificare
  `initiativeId`/`initiativeTitle` su `MASession`/summary).

**Verifica**: build/vet/gofmt/test; smoke: `POST /binocolo/v1/ma/sessions` con
solo `prompt` → risposta con iniziativa `TGT: …` creata e ancorata.

---

## B3 — Cascata bidirezionale del ciclo di vita

**Backend:**
- Nuova query store `CountMAInitiativeAttachedSessions(ctx, initiativeID,
  excludeSessionID string) (int, error)`: `COUNT(*)` su `ma_session` con
  `initiative_id = $1 AND id <> $2 AND purged_at IS NULL`. **NON riusare**
  `session_count` di `ListMAInitiatives` (`ma_store.go:321`): non filtra gli
  stati.
- Nuova query store `CountMAInitiativeCards(ctx, initiativeID) (int, error)`:
  `COUNT(*)` su `ma_initiative_card` **senza filtri di stato** (decisione 3:
  qualunque card blocca la cascata).
- In `updateSessionLifecycle` (`ma_service.go:383`), dopo l'update riuscito:
  1. leggere `GetMASessionState` → se `InitiativeID == ""` stop (transizione,
     sparisce con B6);
  2. condizione di cascata: `CountMAInitiativeAttachedSessions == 0 &&
     CountMAInitiativeCards == 0`;
  3. se vera, applicare la **stessa** azione all'iniziativa via
     `updateInitiativeLifecycle` (`archive`/`delete`/`purge`/`restore`), con
     tolleranza: se l'iniziativa è già nello stato bersaglio o la transizione
     non è ammessa (es. restore di una sessione la cui iniziativa non era mai
     stata cestinata), **non fallire** l'operazione sulla sessione — loggare
     trace event `ma_initiative_cascade_skipped`;
  4. trace event `ma_initiative_cascade_applied` con azione e initiative_id.
- Il restore è simmetrico per costruzione (stessa condizione, stessa azione):
  ripristinare l'unica ricerca di un'iniziativa cestinata a cascata la
  ripristina; se nel frattempo l'iniziativa ha ricevuto card o altre ricerche,
  la condizione fallisce e resta com'è (corretto: c'è lavoro di altri).
- Nessuna cascata inversa (agire sull'iniziativa NON tocca le ricerche): fuori
  perimetro, comportamento attuale invariato.

**Verifica**: build/vet/gofmt/test; aggiornare il fake store dei test con i due
count. Smoke con fixture di test esistenti se disponibili.

---

## B2b — Rinomina iniziativa (titolo editabile)

**Contesto verificato**: non esiste alcun endpoint di update titolo/descrizione
iniziativa (solo create/archive/restore, `handler.go:119-124`). L'editabilità
del titolo è parte della decisione 1.

**Backend:**
- `PATCH /binocolo/v1/ma/initiatives/{id}` con body `{title?, description?}`;
  validazione come `createInitiative` (`cleanText` 120/500, titolo non vuoto se
  presente); iniziativa deve essere operational; trace event. Store:
  `UpdateMAInitiativeInfo` (UPDATE mirato + `updated_at`).

**Frontend** (può accorpare a F3): rinomina inline o da modale sulla card
iniziativa in `IniziativePage.tsx` e/o dall'header di `IniziativaBoardPage.tsx`
— pattern editing secondo `docs/UI-UX.md`.

**Verifica**: build/vet/gofmt/test; `tsc --noEmit`; smoke rinomina su
iniziativa di prova.

---

## B4 — Sgancio vietato, spostamento permesso

**Backend:**
- `setSessionInitiative` (`ma_service.go:488-530`): `initiativeID` vuoto →
  errore di validazione (rimuovere il ramo unanchor; il commento
  "anchors or unanchors" va aggiornato). Lo spostamento su altra iniziativa
  resta identico (incluso il backfill card da rating ≥1★ già presente,
  `ma_service.go:513-528`).
- Nessuna cascata sull'iniziativa di origine quando una ricerca viene spostata
  (decisione 5).
- `handleSetMASessionInitiative` (`handler.go:400-416`): messaggio d'errore
  esplicito per body senza `initiativeId`.

**Verifica**: build/vet/gofmt/test.

---

## B5 — Migrazione 103: backfill orfane (bonifica A, passo 2)

> **Prerequisito operativo (passo 1, azione utente)**: purge manuale del
> cestino ricerche dall'UI. La migrazione va applicata DOPO.

**Migrazione 103** `103_binocolo_ma_initiative_backfill.sql` (idempotente,
SQL puro — niente LLM: titoli verbatim):
- Per ogni `ma_session` con `initiative_id IS NULL` **e** `purged_at IS NULL`
  (le purgate restano orfane per sempre: invisibili ovunque, il NOT NULL di B6
  dovrà quindi escluderle — vedi B6):
  1. `INSERT` in `ma_initiative` di una riga con `id = gen_random_uuid()`,
     `title = 'TGT: ' || left(session.title, 115)`, description vuota,
     `created_by_* = session.created_by_*`, `created_at = session.created_at`,
     e **stato ereditato**: `archived_at`/`deleted_at` (+ by subject/email)
     copiati dalla sessione;
  2. `UPDATE ma_session SET initiative_id = <nuova>` sulla sessione.
- Idempotenza: la condizione `initiative_id IS NULL` rende la ri-applicazione
  un no-op. Usare un CTE `INSERT ... RETURNING` + `UPDATE` in un'unica
  transazione per garantire l'accoppiamento 1:1.
- Commento in testa alla migrazione con la query di verifica orfani:
  `SELECT count(*) FROM binocolo.ma_session WHERE initiative_id IS NULL AND purged_at IS NULL;`

**Attenzione**: le sessioni **purgate** orfane restano con `initiative_id
NULL`. Sono invisibili a ogni vista (tombstone) e nessun codice le carica.

**Verifica**: doppia applicazione ok su DB locale di prova NON condiviso — in
assenza, review testuale rigorosa + applicazione dell'utente col conteggio
prima/dopo (chiedere all'utente di incollare i conteggi).

---

## B6 — Migrazione 104: `initiative_id` NOT NULL (chiusura)

> Applicare SOLO dopo: 102+103 applicate, F1 deployata (nessun client crea più
> orfane), e verifica orfani-zero fatta dall'utente.

**Migrazione 104** `104_binocolo_ma_session_initiative_not_null.sql`:
- Le purgate orfane di B5 impediscono un NOT NULL secco. Due opzioni, decidere
  all'esecuzione col dato reale alla mano:
  (a) se il conteggio purgate-orfane è 0 → `ALTER TABLE ... ALTER COLUMN
  initiative_id SET NOT NULL`;
  (b) altrimenti → `ADD CONSTRAINT ma_session_initiative_required CHECK
  (initiative_id IS NOT NULL OR purged_at IS NOT NULL) NOT VALID` +
  `VALIDATE CONSTRAINT` (vincolo dichiarativo equivalente per le righe vive).
- In entrambi i casi l'invariante di prodotto è garantito dallo schema.

---

## F1 — Nuova ricerca: selettore iniziativa obbligatorio in testa

**File**: `apps/binocolo/src/pages/ricerche/NuovaRicercaPage.tsx`.

**Stato attuale verificato**: esiste già un selettore opzionale (state
`initiativeId` riga 62, fetch riga 71, `<select>` con default "Nessuna
iniziativa" righe 277-284, modale di creazione inline righe 485-511) e
l'aggancio avviene con una **seconda chiamata** post-creazione (righe 168-170).

**Cambiamenti**:
1. Spostare il blocco iniziativa **in testa al form**, prima del prompt
   (decisione 2). Tre scelte mutuamente esclusive, radio o segmented control
   secondo `docs/UI-UX.md`:
   - **«Nuova iniziativa · titolo automatico»** (default preselezionato) —
     sub-copy: «Il titolo viene generato dalla ricerca (prefisso TGT).»
   - **«Nuova iniziativa · titolo manuale»** — input testo obbligatorio se
     selezionata.
   - **«Iniziativa esistente»** — select sulle attive (fetch esistente riga
     71); vuota/disabilitata se non ce ne sono.
   Rimuovere l'opzione "Nessuna iniziativa". Il modale inline esistente
   (righe 485-511) diventa ridondante: rimuoverlo se non più raggiungibile.
2. `createSession` (riga 154): passare nel body `initiativeId` **oppure**
   `newInitiativeTitle` (B2); **eliminare** la POST di aggancio separata
   (righe 168-170). Mostrare nel riepilogo post-generazione il titolo
   dell'iniziativa risolta (dalla risposta B2), con hint che è editabile dalla
   pagina iniziative.
3. Copy: mai "obbligatorio/mandatorio" esplicito — l'assenza dell'opzione
   "nessuna" comunica da sola.

**Verifica**: `tsc --noEmit` verde; smoke playwright-cli: pagina nuova ricerca
→ il blocco iniziativa è primo, default auto, creazione sessione con prompt di
prova (senza estimate/execute) → badge iniziativa `TGT:` visibile; dichiarare
la sessione di prova creata e cestinarla a fine smoke (la cascata B3 riassorbe
l'iniziativa: verificarlo è parte dello smoke).

---

## F2 — Ricerche: da "Aggancia" a "Sposta"

**File**: `apps/binocolo/src/pages/ricerche/RicerchePage.tsx`.

- Il menu riga "Aggancia a iniziativa…" (riga 236) diventa **«Sposta in
  iniziativa…»**; il modale (righe 303-323) aggiorna titolo e CTA («Sposta»).
  La POST è la stessa (`/sessions/{id}/initiative`, riga 68).
- Ogni riga ora ha sempre il badge iniziativa (post B5 non esistono orfane):
  il ramo senza `initiativeTitle` (riga 207) resta come fallback difensivo ma
  non deve più offrire "Aggancia" come azione primaria.
- Copy toast: «Ricerca spostata nell'iniziativa.» (riga 72).

**Verifica**: `tsc --noEmit`; smoke: lista ricerche, menu riga, modale sposta
(senza confermare su dati reali, oppure spostare una sessione di prova creata
in F1).

---

## F3 — Iniziative: cestino, purge e filtri di visibilità

**File**: `apps/binocolo/src/pages/iniziative/IniziativePage.tsx` (+ eventuale
`IniziativaBoardPage.tsx` per lo stato cestinata).

- Sostituire il binomio attive/`?archived=1` (righe 35-45, link "Archivio"
  righe 172-179) con i tre filtri **Attive / Archiviate / Cestino** sul pattern
  già implementato in `RicerchePage.tsx` (`?visibility=`), riusandone la UX
  (pill/tab + conferme).
- Azioni per card iniziativa: oltre ad archivia/ripristina (righe 141-158),
  aggiungere cestina (`DELETE /binocolo/v1/ma/initiatives/{id}`) con modale di
  conferma, e nel filtro Cestino: ripristina + elimina definitivamente
  (`POST .../purge`) con doppia conferma — specchiare copy e componenti di
  `RicerchePage`.
- Se l'iniziativa ha ricerche collegate o card, il backend non la cestina in
  cascata (B3 opera solo ricerca→iniziativa): l'azione diretta sull'iniziativa
  resta permessa e NON tocca le ricerche figlie — le ricerche di un'iniziativa
  cestinata restano visibili in Ricerche col loro badge; il click sul badge
  verso una board cestinata deve gestire il 404/stato con messaggio chiaro
  («Iniziativa nel cestino») invece di errore generico.
- Aggiornare `MAInitiativeSummary` TS (`apps/binocolo/src/api/types.ts`) con i
  campi B1.

**Verifica**: `tsc --noEmit`; smoke: filtri, cestina/ripristina su
un'iniziativa di prova (creata via F1, senza spesa).

---

## F4 — Smoke complessivo del flusso

Percorso end-to-end senza spesa (nessun estimate/execute):
1. Nuova ricerca con default auto → sessione + iniziativa `TGT:` create.
2. Rinomina titolo iniziativa (B2b).
3. Cestina la ricerca → l'iniziativa segue (cascata B3, condizione vera).
4. Ripristina la ricerca → l'iniziativa torna (simmetria).
5. Sposta la ricerca su un'altra iniziativa → nessuna cascata sull'origine.
6. Cestino+purge della sessione di prova a fine smoke (pulizia).

Esito di ogni passo nel messaggio finale, con screenshot in
`artifacts/claude/`.

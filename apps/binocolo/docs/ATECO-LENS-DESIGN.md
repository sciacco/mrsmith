# Binocolo — Lenti ATECO: catalogo fit d'organizzazione (fase 2)

> Stato: **design consolidato, decisioni complete** — brainstorming 2026-06-17 (branch `poc/aenad`). I tre bivi aperti risolti. Non ancora implementato.
> Memoria correlata: `project_binocolo_retrieval_cost` (Parte F — fit taxonomy), `project_binocolo_scoring_redesign`. Vedi anche `docs/IMPLEMENTATION-PLANNING.md` (repo-fit) e `docs/IMPLEMENTATION-KNOWLEDGE.md`.
> Prerequisito: **Fase 1 IMPLEMENTATA** — ogni `atecoCandidate` porta un `fit` (core/weak/excluded; vuoto=core), risolto longest-prefix-wins (`resolveAtecoFit` ma_rules.go:264), persistito nel JSONB della strategia.

## 1. Obiettivo

In Fase 1 il `fit` vive **dentro la singola strategia** (`MAStrategyVersion`, immutabile). La conoscenza è quindi **intrappolata per-sessione**: archiviata una ricerca, evapora; la ricerca simile successiva riparte in bianco e l'LLM rifà gli stessi errori (es. `631021` elaborazione-dati che ricompare core in una ricerca IT-MSP).

Fase 2 introduce una **memoria d'organizzazione del fit**, riusabile cross-ricerca. Intuizione fondante: **il fit non è intrinseco al codice, è relativo all'intento** — `631021` è *excluded* per "IT gestiti" ma *core* per "studi contabili". Quindi il catalogo non può essere una mappa piatta `codice→fit` globale: è agganciato a una **lente** = profilo/intento settoriale ("IT managed services", "Studi contabili", "Logistica conto terzi").

Confine pulito: **il motore di scoring non cambia.** Fase 2 sta tutta **a monte** (da dove arriva il fit prima dello snapshot) e **a valle** (write-back nel catalogo). `resolveAtecoFit`, `atecoDivisions` (ma_rules.go:302), `inSectorPerimeter`, `measureAtecoPrecision` continuano a leggere lo snapshot `atecoCandidates[].fit` e basta.

## 2. Decisioni fissate

1. **Lente = profilo settoriale ORG-wide**, non globale-per-codice. Il fit è `(lente, codice)`.
2. **Solo righe non-neutral nel catalogo.** `neutral` = assenza di riga (coerente con la semantica overlay di Fase 1: non-listato = neutral, visibile).
3. **Chiave di match = `search_code` dot-stripped, longest-prefix-wins**, identico a `resolveAtecoFit`. La forma puntata è solo display.
4. **Binding snapshot-a-creazione + write-back esplicito.** Lo snapshot congela il fit nello strategia (immutabile, autocontenuta); la strategia porta solo un `lensId` di provenienza. Le modifiche in editor sono **override di sessione** locali. Il push verso il catalogo è un gesto deliberato.
5. **Composizione (precedenza):** `override-sessione ?? lente(longest-prefix) ?? bootstrap-LLM ?? core`. Materializzata **una volta** nello snapshot, al draft/attach.
6. **Iniezione carve-out:** la composizione inietta le righe lente (specie excluded/weak) la cui divisione ricade nelle divisioni dei core/weak proposti, **anche se l'LLM non le ha proposte** — altrimenti l'esclusione non morde.
7. **L'LLM si restringe a bootstrap dei soli codici non coperti dalla lente.** La scoperta dei codici plausibili resta sua; il fit dei codici coperti è della lente (overlay deterministico post-LLM).

### 2.1 Risoluzione dei tre bivi (2026-06-17)

1. **Selezione lente → auto-suggest + conferma.** Un matcher **deterministico** (non l'LLM: la lente è un asset d'organizzazione) calcola, dai codici proposti, l'overlap con la copertura di ogni lente e **pre-seleziona** la migliore sopra soglia, etichettata "suggerita". L'analista conferma semplicemente procedendo, oppure cambia/azzera la lente nel picker. **"Nessuna lente" sempre disponibile** = bootstrap LLM puro (= comportamento Fase 1). Auto-attach silenzioso scartato; modale di conferma dedicata scartata (la pre-selezione reversibile è la conferma).
2. **Precedenza quando l'utente contraddice la lente nel prompt → la lente vince sui codici coperti.** Deterministico e auditabile: l'eventuale contraddizione esplicita diventa un **override di sessione visibile** in editor, che l'analista può tenere e all'occorrenza promuovere via write-back. L'opzione "l'LLM scavalca la lente se esplicito" è scartata (riapre deriva/non-determinismo e toglie autorità al catalogo).
3. **Governance edit catalogo → accesso piatto `app_binocolo_access` + audit** (`updated_by_email`/`updated_at` su ogni riga). Coerente con la scelta di `ma_parameter`/VERYSHORT (nessun ruolo elevato per ora). Un gate a ruolo (`app_binocolo_curator`) è aggiungibile dopo **senza rework**.

Minori (default confermati): **lente singola** per ricerca in 2a (multi-lente rinviata); **seeding solo via azione esplicita** "crea lente da questa sessione" (nessun mining automatico delle strategie passate).

## 3. Modello dati (migrazione 044)

Schema `binocolo`. Pattern audit/CHECK come `038_ma_parameter`. `gen_random_uuid()` come le altre tabelle `ma_*`.

### 044 — `ma_ateco_lens` + `ma_ateco_lens_fit`
```sql
CREATE TABLE binocolo.ma_ateco_lens (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  name             text NOT NULL,            -- "IT managed services"
  description      text,                     -- intento, note di curatela
  created_by_email text,
  updated_by_email text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (name)
);

CREATE TABLE binocolo.ma_ateco_lens_fit (
  lens_id          uuid NOT NULL REFERENCES binocolo.ma_ateco_lens(id) ON DELETE CASCADE,
  search_code      text NOT NULL,            -- dot-stripped, chiave di match (mirror di SearchCode)
  code             text NOT NULL,            -- forma puntata, display
  description      text,
  fit              text NOT NULL CHECK (fit IN ('core','weak','excluded')),
  note             text,
  updated_by_email text,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (lens_id, search_code)
);
```

Note: solo righe non-neutral (vincolo CHECK esclude `neutral`). `search_code` è la chiave di longest-prefix (vedi `atecoSearchCode` ateco.go:227). `code`/`description` riempiti dal catalogo ATECO (`SearchAteco`) al momento dell'inserimento.

## 4. Composizione del fit (il cuore)

La precedenza `override-sessione ?? lente ?? bootstrap-LLM ?? core` si **materializza una volta sola** nello snapshot, al draft/attach. Per ogni codice proposto:

```
fit = lensFit(longest-prefix su search_code)   se la lente lo copre
    ⊕ fit proposto dall'LLM                      altrimenti (bootstrap)
    ⊕ core                                        default
```

Più l'**iniezione carve-out** (§2.6): le righe lente excluded/weak nelle divisioni dei core/weak proposti entrano nello snapshot anche se non proposte dall'LLM — così l'esclusione (`631021` sotto `631` core) morde davvero.

Dopo lo snapshot la strategia è **autocontenuta**: lo scoring legge `atecoCandidates[].fit` come oggi. Le modifiche in editor = **override di sessione** che divergono dalla lente e restano sulla versione di strategia.

La composizione avviene **solo** al draft/attach (e a un esplicito re-attach, che crea una nuova versione). `estimate`/`execute` **non** ri-applicano la lente: leggono lo snapshot. È quanto rende `lensId` pura provenienza, non un riferimento "live".

## 5. Ruolo dell'LLM (si restringe)

Con una lente attiva l'LLM fa due cose: (a) scopre i codici ATECO plausibili (come oggi, via `maAtecoSearchTool`); (b) assegna fit **solo ai codici non coperti dalla lente**. La composizione (§4) sovrascrive deterministicamente il fit LLM sui codici coperti. In **2a** questo è garantito dall'overlay post-LLM (nessuna modifica al prompt necessaria per la correttezza). Iniettare nel contesto del draft un riassunto della copertura della lente per ridurre lavoro sprecato = rifinitura **2b** (eventuale prompt v5).

## 6. Auto-suggest della lente (matcher deterministico)

Dopo che `canonicalizeMAStrategyAteco` produce i candidati proposti, per ogni lente si calcola un punteggio di overlap tra i `search_code` proposti (core/weak) e la copertura della lente (per prefisso). Si pre-seleziona la lente con overlap massimo **sopra soglia**; sotto soglia → nessuna lente. La pre-selezione **compone** già i candidati mostrati, etichettata "suggerita", e resta trivialmente cambiabile/azzerabile dal picker. Confermare = procedere col draft (stima/salvataggio). Cold start (catalogo vuoto o settore nuovo) → nessuna lente, bootstrap LLM puro.

## 7. Ciclo di vita & write-back

- **Snapshot a creazione** — composizione → fit congelato in `atecoCandidates`; `lensId` salvato in JSONB (migration-free, come `thesis`/`legalForms`/`successionMinOwnerAge` in `MAStrategySpec`).
- **Override di sessione** — l'analista edita righe in `AtecoFitEditor`; locali alla versione, mai sulla lente.
- **Write-back esplicito** — azione "Promuovi nel catalogo": calcola il **diff** tra i fit di sessione e la lente *corrente* (longest-prefix per codice): `aggiunti` (codice assente nella lente), `modificati` (fit diverso). L'analista spunta quali delta persistere → upsert in `ma_ateco_lens_fit` + audit. Mai automatico. La rimozione di una voce dalla lente è un gesto esplicito separato (non dedotto dal "codice tolto in sessione", che sarebbe distruttivo).
- **Senza lente attiva** — il write-back offre "crea nuova lente" (dai candidati non-neutral della sessione) oppure "aggiungi a lente esistente". Questo è anche il **seeding** del catalogo (§2 minori).
- **Immutabilità** — se la lente cambia dopo lo snapshot, le strategie già create non sono toccate; il diff di write-back confronta sempre con la lente corrente.

## 8. Dove tocca il codice (ancore reali)

- **`canonicalizeMAStrategyAteco`** (ma_service.go:1379) — riceve la mappa fit della lente; applica overlay + iniezione carve-out dopo la risoluzione DB; resta il punto unico che fissa `SectorDivisions`/fit.
- **`draftStrategy`** (ma_service.go:766; canonicalize a :928) — risolve la lente (da `lensId` esplicito o matcher §6), passa la mappa a canonicalize; restituisce il `lensId` suggerito/applicato.
- **`estimate`/`execute`** (canonicalize a :346/:368/:430/:456) — invariati: leggono lo snapshot. Passano mappa fit vuota.
- **`maService`** (struct :63) — nuovo campo `lensStore` (o estensione di `maWorkspaceStore`); wiring in `newMAService` (:73) e `Deps`/handler (handler.go:21; store wiring :48).
- **`SQLStore`** — nuovi metodi, pattern `ListMAParameters`/`UpdateMAParameter` (ma_store.go:1496/1528): `ListLenses`, `GetLens`, `CreateLens`, `UpdateLens`, `DeleteLens`, `ReplaceLensFit`, `ResolveLensFit(lensID) → map[search_code]fit`.
- **`MAStrategySpec`** — nuovo `LensID string json:"lensId,omitempty"` (migration-free).
- **types.ts** — `MAAtecoLens`, `MAAtecoLensFit`, `lensId?` su strategy, `suggestedLensId?` sul draft.

## 9. Contratti API (nuovi/variati)

| Metodo | Path | Scopo | Auth |
|---|---|---|---|
| GET | `/binocolo/v1/ma/lenses` | lista (id, name, description, #entries, #usi) | access |
| POST | `/binocolo/v1/ma/lenses` | crea lente | access |
| GET | `/binocolo/v1/ma/lenses/{id}` | dettaglio + entries fit | access |
| PUT | `/binocolo/v1/ma/lenses/{id}` | rinomina/descrizione | access |
| DELETE | `/binocolo/v1/ma/lenses/{id}` | elimina (CASCADE su fit) | access |
| PUT | `/binocolo/v1/ma/lenses/{id}/fit` | sostituisci/patch entries fit | access |
| POST | `/binocolo/v1/ma/sessions/{id}/lens` | aggancia lente → ricompone (nuova versione) | access |
| POST | `/binocolo/v1/ma/lenses/{id}/writeback` | push delta di sessione (con diff) | access |

`MACreateSessionRequest` esteso con `lensId` opzionale. Il draft ritorna `suggestedLensId`. Le scritture verificano sessione esistente/operativa (pattern rating). Audit via trace event (`ma_lens_updated`, `ma_lens_fit_writeback`).

## 10. UI

- **Editor strategia** — sopra `AtecoFitEditor`, un **picker lente** ("Lente: IT managed services ▾ / nessuna"), pre-selezionato dal matcher con badge "suggerita". Cambiare/azzerare ricompone i candidati mostrati. Per riga ATECO, provenienza: "da lente" vs "modificata in sessione". Pulsante **"Promuovi nel catalogo"** → pannello diff (aggiunti/modificati, spuntabili).
- **Pagina catalogo** — nuova route nell'app binocolo (accanto a `ConfigPage`, registrata in `routes.tsx`/`App.tsx`): lista lenti → apri lente → tabella fit (riuso del layout `AtecoFitEditor` con typeahead `GET /ma/ateco/search`). CRUD lenti, edit fit, colonna "usata in N ricerche".

## 11. Fasatura

- **2a — valore core:** migrazione 044 + CRUD store/handler + composizione/snapshot + iniezione carve-out + matcher auto-suggest + picker in editor + write-back con diff + pagina catalogo minima. Qui c'è già la memoria d'organizzazione.
- **2b — rifinitura:** prompt v5 (contesto lente all'LLM), multi-lente per ricerca, "crea lente da sessione" come azione di primo piano, gate a ruolo `app_binocolo_curator`, analytics d'uso/coverage.

## 12. Repo-fit (checklist `IMPLEMENTATION-PLANNING.md`)

- **Runtime**: nessun nuovo mini-app; la pagina catalogo è una route client *dentro* binocolo (SPA esistente, come `ConfigPage`). Nessuna voce New-App-Checklist.
- **Dev**: nessuna nuova porta Vite, nessun nuovo `dev:*`/target Makefile.
- **Auth**: tutto dietro `app_binocolo_access` (catalogo incluso, nessun ruolo elevato per ora; `app_binocolo_curator` rinviato). Mutazioni autenticate + audit. 401/403 nel test plan.
- **Data-contract**: PK `(lens_id, search_code)`; `search_code` col medesimo criterio di `atecoSearchCode`; solo righe non-neutral; `lensId` su strategia è provenienza, lo snapshot è la verità.
- **Deployment**: migrazione 044 su `ANISETTA_DSN` (a mano, come le precedenti); nessun nuovo env var.
- **Verifica**: vedi §13.

## 13. Strategia di verifica

- **Composizione (Go, deterministico)**: tabellare su `canonicalizeMAStrategyAteco` con mappa lente — overlay vince sui codici coperti; bootstrap LLM tenuto sui non coperti; iniezione carve-out (`631` core + lente `631021` excluded → candidato excluded presente; target `631021` → fuori_criterio via gate immutato).
- **Override di sessione**: una modifica in editor sopravvive (resta sullo snapshot; la lente non la tocca).
- **Write-back**: il diff elenca solo i delta reali; l'upsert persiste i selezionati; idempotente; audit scritto.
- **Matcher**: overlap sopra soglia pre-seleziona la lente attesa; sotto soglia → nessuna lente; catalogo vuoto → nessuna lente (= Fase 1).
- **Immutabilità**: modificare la lente dopo lo snapshot non altera strategie esistenti; il diff confronta con la lente corrente.
- **No spesa in smoke**: nessuna chiamata OpenAPI.it nuova (la composizione è locale); DB di dev condiviso/reale (`project_aenad_dev_db_is_shared`) → scritture catalogo in smoke solo su lenti di prova.

## 14. Dettagli implementativi residui (non decisioni)

- Soglia esatta dell'overlap del matcher e formula (Jaccard sui search_code vs prefix-coverage).
- DDL definitivo 044 + eventuali indici (lookup per `lens_id`, prefix su `search_code`).
- Forma esatta del payload diff di write-back e dello structured response del draft (`suggestedLensId`).
- Layout della pagina catalogo e riuso effettivo di `AtecoFitEditor` org-level.
- Conteggio "usata in N ricerche" (query di provenienza su `lensId` nelle versioni di strategia).

# Binocolo — Lenti ATECO: piano di implementazione

> Esecuzione del design in [`ATECO-LENS-DESIGN.md`](./ATECO-LENS-DESIGN.md). Branch `poc/aenad`. Migrazione su `ANISETTA_DSN` (come 033/043, applicata a mano).
> Prerequisito: **Fase 1 (fit per-strategia) implementata** — `atecoCandidates[].fit`, `resolveAtecoFit` (ma_rules.go:264), `atecoDivisions` (ma_rules.go:302), gate/precisione su fit. Questa è la **Fase 2**: il catalogo org "lenti".
> Riferimenti codice verificati: `canonicalizeMAStrategyAteco` (ma_service.go:1379), `draftStrategy` (ma_service.go:766, canonicalize :928), canonicalize in estimate/execute (:346/:368/:430/:456), interfaccia `maWorkspaceStore` + pattern CRUD `ListMAParameters`/`UpdateMAParameter` (ma_store.go:1496/1528), `atecoSearchCode` (ateco.go:227), `handleSearchAteco` (lookups.go:44), `Deps`/store wiring (handler.go:21/:48), routing FE (`routes.tsx`/`App.tsx`, `ConfigPage`), editor `AtecoFitEditor.tsx`.

## Principi

- **Ogni step è spedibile** e verificabile da solo. Step 1 (catalogo) non tocca lo scoring né costa nulla; il rischio è isolato a Step 2 (input del motore).
- **Il motore di scoring NON cambia.** Tutta la Fase 2 sta a monte (composizione → snapshot) e a valle (write-back). `resolveAtecoFit`/gate/precisione leggono lo snapshot come oggi.
- **`search_code` dot-stripped, longest-prefix** ovunque (mirror di `resolveAtecoFit`/`atecoSearchCode`). La lente e lo scoring devono usare lo stesso identico criterio.
- **Snapshot autoritativo.** La composizione gira **una volta** al draft/attach; `lensId` è provenienza. `estimate`/`execute` leggono lo snapshot (lente=nil), non ri-compongono.
- **Niente spesa live negli smoke** (`project_aenad_dev_db_is_shared`): la composizione è locale (nessuna chiamata OpenAPI.it nuova); scritture catalogo in smoke solo su lenti di prova.
- Test: per la **composizione** (overlay + carve-out) e il **matcher** propongo unit test Go deterministici, da approvare prima di scriverli (regola test del repo).

## Sequenza e dipendenze

```
Step 1 (Catalogo CRUD)         ← standalone, no impatto scoring, seeding del catalogo
Step 2 (Composizione+snapshot) ← dopo 1 (servono lenti); aggiunge lensId + overlay + attach + picker manuale
Step 3 (Auto-suggest+picker)   ← dopo 2 (matcher pre-seleziona; provenance badge; polish editor)
Step 4 (Write-back con diff)   ← dopo 2 (chiude il loop: dalla sessione al catalogo)
Fase 2b (rifinitura)           ← dopo 2a (prompt v5, multi-lente, ruolo curatore, analytics)
```

---

## Step 1 — Catalogo lenti (CRUD standalone)

**Obiettivo:** creare/curare lenti e le loro voci fit dal catalogo org. Nessun impatto su strategie/scoring. Abilita il seeding.

**Migrazione 044** `044_anisetta_binocolo_ma_ateco_lens.sql`: tabelle `binocolo.ma_ateco_lens` + `binocolo.ma_ateco_lens_fit` (DDL §3 del design). `gen_random_uuid()` default; `UNIQUE(name)`; FK `ON DELETE CASCADE`; CHECK `fit IN ('core','weak','excluded')`; PK `(lens_id, search_code)`. Indice su `lens_id`. Idempotente (`CREATE TABLE IF NOT EXISTS`).

**Backend:**
- Tipi: `MAAtecoLens{id,name,description,entryCount,usageCount,audit}`, `MAAtecoLensFit{searchCode,code,description,fit,note}` (ma_types.go).
- `maWorkspaceStore` (pattern parametri ma_store.go:1496/1528): `ListLenses(ctx)`, `GetLens(ctx,id)` (lente + entries), `CreateLens`, `UpdateLens` (name/description), `DeleteLens`, `ReplaceLensFit(ctx,lensID,entries,email)` (replace transazionale delle voci). `search_code` derivato con `atecoSearchCode`; `code`/`description` dal candidato/`SearchAteco`.
- Handler (handler.go pattern, dietro `app_binocolo_access`): `GET/POST /binocolo/v1/ma/lenses`, `GET/PUT/DELETE /binocolo/v1/ma/lenses/{id}`, `PUT /binocolo/v1/ma/lenses/{id}/fit`. Validazione: fit ∈ tier, `name` non vuoto/unico (409 su conflitto), codici normalizzati. Trace event `ma_lens_created`/`ma_lens_updated`/`ma_lens_fit_replaced` (audit).
- Wiring: nuovo metodo dell'interfaccia store già coperto da `SQLStore`; nessun cambio a `Deps` (usa `s.store`).

**Frontend (`api/types.ts`, nuova pagina, `routes.tsx`/`App.tsx`):**
- Tipi `MAAtecoLens`/`MAAtecoLensFit` + metodi client.
- Nuova route `/lenti` (o `/catalogo`) + voce nav (come `/config`/`ConfigPage`).
- `LensCatalogPage`: lista lenti (nome, #voci, #usi) → dettaglio lente con tabella fit che **riusa il layout di `AtecoFitEditor`** (riga codice·descrizione·tier·rimuovi + typeahead `GET /ma/ateco/search`). Crea/rinomina/elimina lente; salva voci con `PUT .../fit`.

**Test proposti (in attesa di ok):** nessuno (CRUD lineare); coperto da smoke UI.

**Verifica:** crea lente "IT managed services", aggiungi `631010` core + `631021` excluded; ricarica → persistite; rinomina/elimina; 401/403; nome duplicato → 409.

**Done:** il catalogo org esiste e si cura da UI, indipendente dalle ricerche.

---

## Step 2 — Composizione + snapshot (lente → strategia)

**Obiettivo:** agganciare una lente a una strategia e **comporre** il fit nello snapshot (overlay + carve-out). Selezione manuale (il matcher arriva in Step 3). Qui cambia l'**input** del motore, non il motore.

**Backend — composizione (ma_rules.go + ma_service.go):**
- `MAStrategySpec.LensID string json:"lensId,omitempty"` (JSONB migration-free, come `thesis`/`legalForms`).
- Tipo `lensOverlay` = entries fit della lente risolte; helper `resolveLensFit(entries, searchCode) string` (mirror di `resolveAtecoFit`, longest-prefix su `search_code`).
- **`canonicalizeMAStrategyAteco`** (ma_service.go:1379) — nuovo parametro `lens *lensOverlay` (nil = nessun overlay). Dopo la risoluzione DB dei candidati:
  1. **Overlay**: per ogni candidato non-excluded, se `lens` lo copre (longest-prefix) → `fit = lensFit`; altrimenti tieni il fit del candidato (bootstrap LLM/analista) → default core.
  2. **Iniezione carve-out**: aggiungi le voci `lens` (excluded/weak) la cui divisione ∈ `atecoDivisions(core/weak proposti)` e non già presenti, come candidati con quel fit (le excluded normalizzate senza ResolveAtecoCode, come il ramo excluded esistente :1394).
  3. `SectorDivisions` ricalcolato a valle (invariato :1448).
- Call-site: `draftStrategy` (:928) passa `lens` risolto da `strategy.LensID` (qui ancora nullo nel draft puro; popolato quando l'analista pre-seleziona — Step 3); **`estimate`/`execute` passano `nil`** (leggono lo snapshot, :346/:368/:430/:456).
- Endpoint **`POST /binocolo/v1/ma/sessions/{id}/lens`** body `{lensId}`: carica la strategia attiva, risolve la lente, `canonicalizeMAStrategyAteco(..., lens)`, **crea una nuova versione** (`AddMAStrategyVersion`). Re-attach = ricomposizione deliberata (la lente è autoritativa al momento dell'attach; gli override fatti dopo vivono sulla nuova versione). `lensId=""` stacca (ricompone su bootstrap puro). Trace `ma_lens_attached`.

**Frontend:**
- `strategy.lensId` nei tipi; **picker lente** minimale sopra `AtecoFitEditor` (dropdown da `GET /ma/lenses` + "nessuna"); on-change → `POST .../lens` → ricarica detail (nuova versione, candidati ricomposti).

**Test proposti (in attesa di ok):** Go tabellare su `canonicalizeMAStrategyAteco` con `lens`:
- overlay vince sui codici coperti (lente `631021` excluded sovrascrive bootstrap core);
- bootstrap tenuto sui non coperti;
- **carve-out iniettato** (lente `631021` excluded + proposto `631` core → candidato `631021` excluded presente anche senza che l'LLM lo proponga; target `631021` → `fuori_criterio` via gate immutato);
- `lens=nil` → snapshot invariato (estimate/execute non ricompongono).

**Verifica:** aggancia "IT managed services" a una sessione Novara → `631021` diventa escluso nel perimetro e i SEA-SERVIZI spariscono dalla shortlist (gate); staccare la lente li ri-espone; il re-estimate non cambia il fit (snapshot).

**Done:** la conoscenza del catalogo entra nelle ricerche; il motore resta intatto.

---

## Step 3 — Auto-suggest (matcher) + picker rifinito

**Obiettivo:** la lente giusta si pre-seleziona da sola; l'analista conferma procedendo.

**Backend — matcher (ma_service.go):**
- `suggestLens(ctx, candidates) (lensID string, score float64)`: per ogni lente, overlap tra i `search_code` core/weak proposti e la copertura della lente (per prefisso). Sopra soglia (TBD, §14 design) → ritorna la migliore; sotto → "".
- `draftStrategy` (:766): dopo canonicalize (senza lente), se `strategy.LensID==""` chiama `suggestLens`; se trova una lente, **ricompone** con quell'overlay e setta `LensID`. Il draft ritorna con la lente suggerita già applicata.
- Response del draft espone `suggestedLensId` (provenienza: suggerita vs scelta).

**Frontend (`TargetPage.tsx`, `AtecoFitEditor`):**
- Picker mostra la lente pre-selezionata con badge **"suggerita"**; cambiare/azzerare ricompone (riusa `POST .../lens`).
- Per riga ATECO, **provenienza**: "da lente" vs "modificata in sessione" (confronto fit candidato vs `resolveLensFit` della lente attiva).

**Test proposti (in attesa di ok):** Go su `suggestLens`: overlap sopra soglia → lente attesa; sotto → ""; catalogo vuoto → "" (= Fase 1).

**Verifica:** una nuova ricerca IT su Novara pre-seleziona "IT managed services"; un settore senza lente → "nessuna" (bootstrap puro); cambio lente ricompone i candidati.

**Done:** zero-click per il caso comune, override banale.

---

## Step 4 — Write-back con diff

**Obiettivo:** chiudere il loop — promuovere gli override di sessione nel catalogo, in modo curato.

**Backend:**
- `lensWritebackDiff(strategy, lensEntries) → {added[], changed[]}`: per ogni candidato non-neutral di sessione, confronta `fit` con `resolveLensFit` della lente corrente → `added` (codice non coperto) / `changed` (fit diverso). La rimozione non è dedotta (gesto esplicito separato).
- Endpoint **`POST /binocolo/v1/ma/lenses/{id}/writeback`** body `{sessionId, entries:[{searchCode,fit}]}` (le voci scelte dall'analista): upsert in `ma_ateco_lens_fit` (riusa `ReplaceLensFit`/upsert) + audit `updated_by_email`. Trace `ma_lens_fit_writeback`.
- Senza lente attiva: il diff è verso "nessuna" → tutte `added`; l'azione offre "crea nuova lente" (`CreateLens` + fit) o "aggiungi a esistente" (= **seeding**).

**Frontend:**
- Pulsante **"Promuovi nel catalogo"** in editor → pannello diff (aggiunti/modificati, ciascuno spuntabile); conferma → writeback; toast esito. Se nessuna lente: dialog "crea lente da questa sessione" (nome+descrizione) o scelta lente target.

**Test proposti (in attesa di ok):** Go su `lensWritebackDiff`: classi added/changed corrette; idempotenza (riapplicare lo stesso diff non cambia nulla).

**Verifica:** in sessione cambio `629000` da core a weak → "Promuovi" mostra 1 `changed` → conferma → la lente in catalogo riflette weak; una seconda sessione che la aggancia eredita il weak.

**Done:** il catalogo migliora con l'uso, solo con input curato e auditato.

---

## Fase 2b — Rifinitura (sketch, dopo 2a)

- **Prompt v5** (`045_..._prompt_v5.sql`): inietta nel contesto del draft un riassunto della copertura della lente, così l'LLM si concentra sulla scoperta dei codici non coperti (la correttezza è già garantita dall'overlay; questo riduce lavoro sprecato).
- **Multi-lente** per ricerca: `LensIDs []string`, composizione con risoluzione conflitti (più specifico / excluded-wins).
- **"Crea lente da sessione"** come azione di primo piano (oggi sotto il write-back senza lente).
- **Ruolo curatore** `app_binocolo_curator` a gate delle mutazioni catalogo (oggi piatto + audit).
- **Analytics** d'uso/coverage della lente (#usi via provenienza `lensId` nelle versioni, % codici neutral).

---

## Note trasversali (repo-fit)

- **Nessun nuovo mini-app**: niente modifiche a root `package.json`/`Makefile`/`catalog.go`. Solo nuova route+nav *dentro* binocolo (Step 1). Nessun nuovo env var.
- **Auth**: tutto dietro `app_binocolo_access`; mutazioni catalogo audited; `app_binocolo_curator` rinviato a 2b (aggiungibile senza rework).
- **Osservabilità**: trace event per ogni operazione (`ma_lens_*`, `ma_lens_attached`, `ma_lens_fit_writeback`).
- **DI**: il catalogo usa `s.store` esistente; nessuno stato globale di package.
- **Migrazione**: 044 a mano su `ANISETTA_DSN`, idempotente (`IF NOT EXISTS`/CHECK).
- **Confine invariante**: nessuna modifica a `scoreMATargetsV2`/`measureAtecoPrecision`/`inSectorPerimeter`; se un test del motore cambia, è un campanello (la Fase 2 non deve toccarlo).

## Checklist pre-merge per step

- [ ] `go build ./...` + `go vet ./...` + `gofmt`
- [ ] `pnpm --filter mrsmith-binocolo exec tsc --noEmit`
- [ ] migrazione 044 applicata e ri-applicabile (idempotente)
- [ ] test Go della composizione/matcher/diff verdi (dove approvati)
- [ ] trace event presenti per le nuove operazioni
- [ ] smoke UI sullo step (server dev già attivo, no riavvio; bypass `VITE_DEV_AUTH_BYPASS`)
- [ ] motore di scoring non toccato (diff backend non include `ma_scoring.go`/`ma_catalog.go` salvo lettura)

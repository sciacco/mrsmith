# Binocolo — Piano esecutivo Kanban v2 (stati, board, pipeline aggregata)

> **Stato**: piano approvabile — deriva dal brainstorming ratificato del 2026-07-18 (stati v2, interfaccia dal mockup "Pipeline M&A", dashboard aggregata). Riferimenti al codice verificati sul working tree al 2026-07-18 (branch `feat/binocolo`). Scritto per esecuzione da parte di un LLM, un task per volta, con QA Gate bloccanti fra le fasi.
>
> **Asticella di qualità (non negoziabile, richiesta esplicita)**: la nuova kanban deve **superare la qualità di Linear** — "gli analisti devono restare con la bocca aperta". Il salto è su DUE assi, entrambi deliverable di prima classe:
> 1. **Interaction engineering** — fisica del drag, latenza percepita zero, transizioni FLIP, keyboard.
> 2. **Sprint visivo** — diagnosi esplicita dell'utente sull'attuale interfaccia: "troppo leggera, poco contrastata, poco colorata, poco tutto". La board v2 deve avere un'identità cromatica e un contrasto **deliberatamente più forti** dell'app attuale (§6.3): riprodurre la palette pallida di oggi con più animazioni = fallimento, ed è criterio di bocciatura ai gate G0/G2/GF.
>
> Ogni fase passa da un QA Gate con panel di esperti (§9); un gate non superato blocca la fase successiva.

---

## 1. Decisioni ratificate (2026-07-18) — non ri-litigare

### 1.1 Stati v2

Set a 10 stati operativi + `rimossa` (fuori board, invariata: errore di triage, mai un verdetto):

| Chiave | Label UI | Macrofase | Tipo |
|---|---|---|---|
| `approfondimento` | Approfondimento | Origination | attivo |
| `da_contattare` | Da contattare | Origination | attivo |
| `primo_contatto` | Primo contatto | Origination | attivo |
| `primo_incontro` | Primo incontro | Engagement | attivo |
| `nda` | NDA | Engagement | attivo |
| `loi` | LOI | Engagement | attivo |
| `ricontattare` | Ricontattare | Follow-up | attivo (parcheggio, data opzionale) |
| `won` | WON | Esito | **terminale** |
| `ko_nostro` | KO nostro | Esito | **terminale** (+ esito) |
| `ko_target` | KO target | Esito | **terminale** (+ esito) |

- **Macrofasi = pura presentazione**: nessuna colonna DB, nessun evento dedicato. Origination / Engagement / Follow-up sempre visibili; **Esito solo in "Vista completa"**.
- **Esito**: attributo solo sotto `ko_nostro` / `ko_target`. **Vocabolario suggerito + testo libero** (combobox, input libero ammesso). Suggeriti — KO nostro: `non_idonea`, `no_go_strategico`, `prezzo`; KO target: `non_vende`, `in_trattativa_altrui`, `prezzo_richiesto`, `altro`. WON senza esito. Per la calibrazione dello score: le chiavi note mantengono il segnale (non_idonea = negativo/errore di screening; no_go_strategico = neutro; KO target = positivo "buono ma non vinto"), il testo libero vale neutro/da classificare.
- **Ricontattare**: data **opzionale non obbligatoria**, puramente informativa (chip sulla card, ordinamento colonna per data con null in coda). NIENTE reminder/alert: il vincolo non-CRM sopravvive.
- **Transizioni**: libere fra i 7 non-terminali (drag); drop/azione sui 3 terminali apre modale (KO: esito+nota+ponte registro; WON: conferma secca). Riapertura da terminale/rimossa via nuova ≥1★ o azione manuale → `approfondimento` (invariato).
- **Migrazione pregresso**: mapping secco `contattata→primo_contatto`, `in_dialogo→primo_incontro`, `offerta→loi`, `chiusa→ko_target` (verificato sul DB: 2 sole card chiuse, entrambe `sfumata` — nessuna informazione da preservare oltre l'esito che resta com'è). `approfondimento`, `da_contattare`, `rimossa` invariati.
- **Ponte registro azienda**: le chiusure `ko_target` propongono i fatti tipizzati `non_vende` / `in_trattativa_altrui` (oggi era su `no_go`/`rimandata`). Nessun ponte su `ko_nostro` (context-scoped) né su `ricontattare` (non è un verdetto).

### 1.2 Interfaccia board (mockup "Pipeline M&A" = struttura; contenuti = i nostri)

- **Matrice viste 2×2**: `Pipeline attiva | Vista completa` × `Macrofasi | Tutti gli stati`, persistita per utente (localStorage).
- **Strip funnel** in testa: chip per stato raggruppati per macrofase con conteggi, **sempre tutti gli stati anche a 0** (posizioni stabili), chip cliccabile = filtro.
- **Vista Macrofasi**: 3–4 contenitori, dentro ciascuno gli stati come sotto-sezioni con card compatte e "Apri stato →" (espande lo stato nella vista Tutti gli stati).
- **Vista Tutti gli stati**: kanban classica, colonne collassabili a rail (persistenza esistente riusata), card ricca.
- **Contenuti card invariati** (ratifica: "il contenuto delle card è solo indicativo, teniamo per buono tutti i contenuti delle nostre card"): nome, provincia, stelle provenienza, chip Diretta, badge registro, marker collisione, bottone dossier a 3 stati, ultimo evento. **NIENTE € per card, niente owner, niente campo next-step** (resta l'ultimo evento/diario). Terminologia nostra: "aziende", board intitolata all'iniziativa.
- **"+ Aggiungi azienda" per colonna**: aggiunta diretta con stato iniziale = colonna di destinazione (solo stati non-terminali).
- **Vista tabella**: resta, con stati nuovi + filtro macrofase.

### 1.3 Dashboard aggregata `/pipeline`

- **Rotta nuova che diventa la default di ingresso dell'app** (index redirect + catch-all).
- Scope: card di **tutte le iniziative attive** (escluse archiviate/cestinate). Stessa azienda in N iniziative = N card (chip iniziativa su ogni card; il marker collisione è il segnale primario anti doppio-contatto).
- **Read-only in v1**: nessun drag, nessuna mutazione, nessun "+ Aggiungi". Click su card → drawer in sola lettura con «Apri nella board» e «Apri scheda».
- Filtro multi-select per iniziativa + ricerca nome + stesse viste 2×2 + strip funnel aggregata.

---

## 2. Fotografia del codice (verificata 2026-07-18)

### 2.1 Dove vivono gli stati oggi

| Superficie | File | Cosa |
|---|---|---|
| Costanti Go stati/esiti | `backend/internal/binocolo/ma_types.go:305-349` | `maCardState*`, `maCardEsito*`, `maCardActiveStates` (L325-331), `validMACardState` (L333), `validMACardEsito` (L342) |
| Stato iniziale/riapertura | `backend/internal/binocolo/ma_service.go:2007` e `:2620` | `openMAInitiativeCard` e `reopenCard` → `approfondimento` (il DEFAULT SQL `da_contattare` è inerte: il Go valorizza sempre) |
| Transizioni | `ma_service.go:2446-2471` (`setCardState`, rifiuta chiusa/rimossa), `:2485-2549` (`closeCard`, ponte registro L2476-2479), `:2555-2607` (`removeCard`), `:2611-2637` (`reopenCard`) |
| Filtri SQL "card attiva" | `ma_store.go:2956`, `:3263`, LATERAL `:3213-3214`/`:3259-3273` (`NOT IN ('chiusa','rimossa')`, `card_state = 'chiusa'`) |
| Badge stato /aziende | `ma_store.go:3415-3419` (`buildStatus`: working=stato attivo, closed=esito) |
| Conteggi per stato (indice) | `ma_store.go:468-474` (`jsonb_object_agg(c.state, count)`) |
| Rotte card | `handler.go:131-135` (`/state`, `/close`, `/remove`, `/reopen`, `/note`) |
| Test che fissano chiavi | `ma_company_search_test.go:55-73` (`in_dialogo`, `no_go`) — unico |
| CHECK SQL | `deploy/migrations/088_binocolo_ma_initiative_card.sql:21-23` (state + esito, constraint inline senza nome esplicito); eventi diario `109_...sql:14` (`ma_target_outcome_event_check`, già adeguato: `stato`+payload) |
| Board | `apps/binocolo/src/pages/iniziative/IniziativaBoardPage.tsx` — `STATES` L25-32, `ESITO_LABELS` L34-40, `ESITI` L42-53, `STATE_COLORS` L99-107, renderer diario L1398-1441 (payload `from`/`to` L1402-1404, `esito` L1408) |
| Indice iniziative | `IniziativePage.tsx:23-30` (`STATE_ORDER`, conteggi per tessera) |
| /aziende | `AziendePage.tsx:10-58` (`cardStateLabels`, `outcomeLabels`, `statusPresentation`) |
| Scheda azienda | `SchedaAziendaPage.tsx:52-70` (`CARD_STATE_LABELS`, `OUTCOME_LABELS` — map **ibrida** eventi diario + esiti card), CardsSection L1052-1069 |
| Tipi TS | `api/types.ts:350-364` (`MAInitiativeCard.state: string` — union non stretta, nessuna rottura di tipo) |

**Problema strutturale rilevato**: 5 mappe stati/esiti duplicate lato TS, nessuna fonte condivisa. Il piano la introduce (F1).

### 2.2 Infrastruttura disponibile

- **Migrazioni**: prossimo numero libero **112**. I CHECK di 088 su `state`/`esito` sono inline (nomi auto-generati `ma_initiative_card_state_check` / `ma_initiative_card_esito_check`): drop con `IF EXISTS` + fallback `DO $$ pg_constraint $$` (pattern già usato in 089).
- **DnD**: `@dnd-kit/core ^6.3.1` + `@dnd-kit/sortable ^10.0.0` + `@dnd-kit/utilities ^3.2.2` già validati nel monorepo (quotes, kit-products, manutenzioni). Nessuna libreria di animazione nel monorepo (scelta in §6.2).
- **Data layer**: `@tanstack/react-query ^5.62` già in binocolo (AziendePage, CompanyDossierPage, TestPage; config in `main.tsx:16-21`); la board attuale è imperativa non-optimistic (`load()` completo dopo ogni POST, `IniziativaBoardPage.tsx:330-338`) — da migrare.
- **Rotte**: index → `Navigate /iniziative` (`routes.tsx:23`), catch-all `*` → `/iniziative` (`routes.tsx:37`); nav a tab `TabNavGroup` in `App.tsx:7-29`.
- **Keyboard esistente**: Staffetta j/k in `SchedaAziendaPage.tsx:424-443` + coorte `components/scheda/cohort.ts` (localStorage `binocolo.scheda.cohort`); la board scrive già la coorte (`writeCohort`).
- **Design system**: token clean.css completi (motion: `--ease-out`, `--ease-spring`, `--duration-fast/normal/slow`); `Iniziative.module.css` 1341 righe (lista+board insieme); regole vincolanti `docs/UI-UX.md` §8 (motion), §8.3 (mai entrance su refetch/optimistic), §13.2 (righe tabella keyboard-operable), §16 (WCAG AA).

---

## 3. Migrazione 112 — `112_binocolo_kanban_v2.sql`

Ordine interno obbligato: si **droppa prima il CHECK stati vecchio**, POI si rimappa, POI si aggiunge il CHECK nuovo. (Un `UPDATE` verso un valore del nuovo set — es. `'primo_contatto'` — violerebbe altrimenti il CHECK vecchio, che consente solo i 6 stati pregressi: errore osservato al primo run, `violates check constraint ma_initiative_card_state_check`.)

```sql
-- 1) Drop dei CHECK vecchi PRIMA del remapping (nome inline 088 + fallback pg_constraint)
ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT IF EXISTS ma_initiative_card_state_check;
-- DO $$ ... pg_get_constraintdef ILIKE '%state%' AND '%approfondimento%' ... $$
ALTER TABLE binocolo.ma_initiative_card DROP CONSTRAINT IF EXISTS ma_initiative_card_esito_check;
-- DO $$ ... pg_get_constraintdef ILIKE '%esito%' ... $$

-- 2) Mapping stati pregressi (ora senza CHECK sugli stati)
UPDATE binocolo.ma_initiative_card SET state = 'primo_contatto'  WHERE state = 'contattata';
UPDATE binocolo.ma_initiative_card SET state = 'primo_incontro'  WHERE state = 'in_dialogo';
UPDATE binocolo.ma_initiative_card SET state = 'loi'             WHERE state = 'offerta';
UPDATE binocolo.ma_initiative_card SET state = 'ko_target'       WHERE state = 'chiusa';
-- esito esistente resta com'è (testo ora libero)

-- 3) Nuovo CHECK stati (tutte le righe ora nel nuovo set)
ALTER TABLE binocolo.ma_initiative_card ADD CONSTRAINT ma_initiative_card_state_check
  CHECK (state IN ('approfondimento','da_contattare','primo_contatto','primo_incontro',
                   'nda','loi','ricontattare','won','ko_nostro','ko_target','rimossa'));

-- 4) Data di ricontatto (informativa, nullable)
ALTER TABLE binocolo.ma_initiative_card ADD COLUMN IF NOT EXISTS recontact_on date;
```

Tutto in `BEGIN…COMMIT`: un fallimento fa rollback totale (nessuno stato parziale). Idempotente: i drop sono `IF EXISTS`, l'`ADD CONSTRAINT` è preceduto dal drop, la colonna è `IF NOT EXISTS`.

Note vincolanti:
- Il DEFAULT `'da_contattare'` (088:21) resta valido nel nuovo set: non toccarlo.
- `ma_target_outcome_event_check` **non si tocca**: i nuovi passaggi restano dentro il vocabolario esistente (`stato` con payload from/to, `chiusura` con payload esito+stato terminale).
- **I payload storici del diario NON si riscrivono** (log append-only = ground truth): le chiavi legacy (`contattata`, `in_dialogo`, `offerta`, `chiusa`, più esiti storici) si gestiscono con una label-map legacy lato frontend (F1).
- La migrazione la **applica l'utente** con il suo processo (mai operazioni dirette sui DB configurati in env — vincolo assoluto di progetto).

## 4. Backend (B1–B2)

### B1 — Modello stati v2 (`ma_types.go`, `ma_service.go`, `ma_store.go`, `handler.go`)

1. **`ma_types.go`**: sostituire le costanti stato (L305-311) con le 11 chiavi; aggiungere `maCardTerminalStates = {won, ko_nostro, ko_target}` e helper `isMACardTerminalState()`; `maCardActiveStates` → i 7 non-terminali (l'ordinamento è quello del funnel: approfondimento → … → ricontattare); `validMACardState` → 11 chiavi. **Rimuovere `validMACardEsito`**: l'esito diventa testo libero `cleanText(esito, 120)`, obbligatorio non-vuoto solo per i KO. `MACardStateRequest` guadagna `RecontactOn *string` (ISO date, valido solo verso `ricontattare`; una transizione fuori da `ricontattare` azzera la data). `MAInitiativeCard` guadagna `RecontactOn *time.Time` (json `recontactOn`).
2. **`setCardState`** (L2446): accetta i 7 non-terminali (rifiuta i terminali e `rimossa` come oggi rifiutava `chiusa`/`rimossa`); gestisce `recontact_on` (set su ricontattare, clear altrove); payload evento `stato` invariato `{"from","to"}` (+ `"recontactOn"` quando presente).
3. **`closeCard`** (L2485) → semantica "transizione terminale": request `{state: won|ko_nostro|ko_target, esito?, note?, registerFacts?}`. Validazioni: `esito` obbligatorio (anche libero) per i KO, vietato per WON; `registerFacts` (`non_vende`/`in_trattativa_altrui`, set invariato L2476-2479) ammesso **solo per `ko_target`**. Evento diario `chiusura` con payload `{"stato": <terminale>, "esito": <o vuoto per won>}`. `ClosedAt` valorizzato per tutti e tre i terminali.
4. **`reopenCard`** (L2611): guard aggiornato a `isMACardTerminalState(state) || state == rimossa`; destinazione `approfondimento` invariata.
5. **`ensureInitiativeCard`/`createDirectInitiativeCard`**: guard "card attiva" (L2080, L2185) aggiornato a `!terminal && != rimossa`. `createDirectInitiativeCard` accetta `initialState` opzionale (default `approfondimento`, solo non-terminali) per il "+ Aggiungi" per colonna.
6. **`ma_store.go`**: tutti i filtri `NOT IN ('chiusa','rimossa')` (L2956, L3263, LATERAL L3213-3214/L3259-3273) → `NOT IN ('won','ko_nostro','ko_target','rimossa')`; `closed_card` (`card_state = 'chiusa'`) → `card_state IN ('won','ko_nostro','ko_target')`. `buildStatus` (L3415-3419): `closed` porta **lo stato terminale** come `Value` (won/ko_nostro/ko_target) e l'esito in `Reason` — più informativo dell'esito solo. Upsert/scan aggiornati per `recontact_on`.
7. **Test**: aggiornare `ma_company_search_test.go:55-73` (`in_dialogo`→`primo_incontro`; il caso closed asserisce `Value` = stato terminale). Aggiungere casi: transizione a `ricontattare` con/senza data, terminale con esito libero, reopen da `won`.

### B2 — Endpoint pipeline aggregata

`GET /binocolo/v1/ma/pipeline` (rotta in `handler.go` accanto alle initiative routes; namespace e prefissi secondo `docs/API-CONVENTIONS.md`, modulo senza prefisso `/api`):

```
{ "initiatives": [MAInitiativeSummary...],          // solo attive
  "cards": [MAPipelineCardView...] }                // MAInitiativeCardView + initiativeId, initiativeTitle
```

- Riusa la decorazione batch di `getInitiativeBoard` (`ma_service.go:2287`) generalizzata a N iniziative: dossier status, collisioni, badge registro, provenienze, lastEvent — **query batch, mai N+1** (stesso vincolo della board).
- Sola lettura; stessa protezione ruolo del modulo. Errori sanitizzati come da convenzione (500 → `internal_server_error`, dettaglio nei log).

## 5. Frontend — F1: dominio condiviso stati

Nuovo modulo **`apps/binocolo/src/lib/cardStates.ts`** — unica fonte TS per:

- `CARD_STATES` ordinati (chiave, label, macrofase, tipo, colore token, abbreviazione strip: APP/DC/PC/PI/NDA/LOI/R/W/KN/KT);
- `MACROFASI` (chiave, label, numero, stati figli, visibilità in Pipeline attiva);
- `TERMINAL_STATES`, `ACTIVE_STATES`, `stateLabel()`, `stateBadgeClass()` (mapping ai token semantic di `UI-UX.md` §4);
- `ESITO_SUGGESTIONS` per `ko_nostro` / `ko_target` (chiave + label + descrizione una riga);
- **`LEGACY_STATE_LABELS`** (`contattata`, `in_dialogo`, `offerta`, `chiusa` + esiti storici `conclusa`…`rimandata`): consumata SOLO dai renderer del diario/storico — i payload append-only non si riscrivono.

Adozione (sostituzione delle 5 mappe duplicate): `IniziativaBoardPage.tsx` (STATES/ESITI/ESITO_LABELS/STATE_COLORS/stateTableLabel), `IniziativePage.tsx:23-30`, `AziendePage.tsx:10-58`, `SchedaAziendaPage.tsx:52-70` (separando finalmente la map ibrida eventi/esiti), `RegistroTab.tsx:23-29` (solo verifica: usa eventi storici, non stati card).

F1 va **nella stessa PR/deploy di B1**: dal momento in cui la migrazione gira, il frontend deve conoscere le chiavi nuove.

## 6. Frontend — F2: board di iniziativa v2

### 6.1 Architettura componenti

Nuova directory `apps/binocolo/src/pages/iniziative/board/` (la board esce dal monolite `IniziativaBoardPage.tsx` ~1450 righe / `Iniziative.module.css` 1341 righe):

```
board/
  BoardPage.tsx            # orchestrazione, data layer, view state 2×2 (persist per utente/iniziativa)
  FunnelStrip.tsx          # chip per stato raggruppati per macrofase, conteggi animati, chip=filtro
  MacroView.tsx            # vista Macrofasi: contenitori + sotto-sezioni stato + card compatta
  StatesView.tsx           # vista Tutti gli stati: colonne, rail collassate, drop zones
  BoardCard.tsx            # card ricca + variante compatta (stessi contenuti di oggi)
  CardDrawer.tsx           # drawer estratto e aggiornato (stepper per macrofase, azioni terminali, data ricontatto)
  TerminalModal.tsx        # modale KO (esito combobox+libero, nota, ponte registro solo ko_target) / conferma WON
  BoardTable.tsx           # vista tabella estratta, filtri stato/macrofase/esito + ricerca
  useBoardData.ts          # react-query: query board + mutazioni optimistic
  useBoardKeyboard.ts      # scorciatoie (sotto)
  flip.ts                  # utility FLIP su WAAPI (sotto)
  board.module.css         # CSS nuovo, token-only
```

`IniziativaBoardPage.tsx` resta come entry sottile che monta `BoardPage`. `DirectCompanyModal` riusato con parametro stato iniziale.

### 6.2 Interaction engineering (il cuore del "oltre Linear")

**DnD — `@dnd-kit`** (aggiunto a `apps/binocolo/package.json` nelle stesse versioni già validate nel monorepo: core ^6.3.1, sortable ^10.0.0, utilities ^3.2.2). Spec del feel:
- **Pickup**: lift con `scale(1.03)` + `--shadow-lg` + leggera rotazione (~1.5°), cursore `grabbing`, sorgente resa ghost (opacity 0.4). Attivazione con distance constraint (8px) per non rubare il click.
- **Over**: placeholder animato che apre lo spazio nella colonna di destinazione (spring `--ease-spring`); colonna target con bordo/tinta `--color-accent-subtle`; rail collassate accettano drop ed evidenziano.
- **Drop**: settling animato verso la posizione finale; drop su colonna terminale (o chip Esito della strip in Vista completa) apre `TerminalModal` senza applicare lo stato finché non confermato (la card torna indietro con animazione su annulla).
- **Toccabilità**: `KeyboardSensor` di dnd-kit attivo — il drag è operabile da tastiera (pick con Space, frecce, drop con Space), requisito AA.

**FLIP — utility propria su Web Animations API** (`flip.ts`, ~100 righe: misura First/Last dei nodi per `companyKey`, anima `transform` con i token motion). Nessuna libreria di animazione nuova: il monorepo non ne ha, il design system è CSS-token-driven, e WAAPI basta. Usi:
- **cambio vista Macrofasi ↔ Tutti gli stati**: le stesse card volano verso la nuova posizione (il momento-firma della demo);
- collasso/espansione colonna a rail;
- riordino post-optimistic (una card che cambia colonna senza drag, es. da drawer).
- Rispetto `prefers-reduced-motion`: FLIP disattivato, cambio istantaneo (UI-UX §8.4).

**Optimistic — react-query** (`useBoardData.ts`): query `['ma-board', id]`; mutazioni stato/chiusura/riapertura con `onMutate` che aggiorna la cache (card spostata subito), rollback su errore + toast; invalidation mirata al termine. Il polling dossier (5s se working) diventa `refetchInterval` condizionale; le patch di refetch **non devono mai far saltare la board** (structural sharing di react-query + niente entrance animation su refetch, UI-UX §8.3 — entrance solo su navigazione, keyed per route).

**Keyboard** (`useBoardKeyboard.ts`): `/` focus ricerca (come da mock); frecce per muovere il focus fra card/colonne; `Enter` apre il drawer; `j`/`k` card successiva/precedente (coerente Staffetta); `1..7` sposta la card focalizzata nello stato n-esimo non-terminale; `w`/`x` aprono TerminalModal (WON / KO); `c` collassa/espande la colonna corrente; `Esc` chiude overlay. Focus ring visibile via `:focus-visible` (token glow) + outline per forced-colors (UI-UX §16). Le scorciatoie sono documentate in un popover `?`.

**Strip funnel**: conteggi con transizione numerica animata (count-up/down breve) quando una card attraversa il funnel; chip attivo = filtro con tinta accent; sempre tutti gli stati, anche a 0.

**Micro-dettagli**: chip data di `ricontattare` che scalda il colore all'avvicinarsi della data (token warning, mai testo base — §4.2); empty state per colonna secondo §14.5 con azione contestuale; skeleton solo al primo load; hover card = lift `translateY(-1px)` + bordo accent (§9).

### 6.3 Sprint visivo (il secondo asse del "oltre Linear")

Diagnosi ratificata dall'utente: l'interfaccia attuale è "troppo leggera, poco contrastata, poco colorata, poco tutto". La board v2 non eredita il linguaggio timido di oggi: costruisce **un'identità visiva propria e più forte**, dentro il tema clean ma spingendone i limiti in modo deliberato. Il meccanismo corretto non è violare la token discipline ma **estenderla** (UI-UX §18: "se manca un token, prima si aggiunge il token e si aggiorna il documento"):

1. **Sistema cromatico degli stati** — nuovo set di token app-level in `apps/binocolo/src/styles/tokens.css` (estensione del tema, pattern previsto da UI-UX §18): una **rampa cromatica del funnel** in cui ogni stato ha un colore pieno e riconoscibile (non il grigino attuale) — Origination in blu/ciano progressivi, Engagement in indaco/viola/ambra, Follow-up in arancio, Esito in verde pieno (WON) e rossi distinti (KO nostro / KO target). Ogni stato: colore base + tinta di sfondo + variante testo AA. Il colore attraversa TUTTA l'interfaccia: dot e bordo del titolo colonna, chip della strip funnel, pill di stato in tabella, accento della card (bordo sinistro colorato), stepper del drawer.
2. **Contrasto e profondità** — separazione netta dei piani: canvas della board più scuro/tinto rispetto alle colonne, colonne come superfici distinte con header forti, card **bianche piene con ombra reale** (`--shadow-md` a riposo, `--shadow-lg` su hover) che staccano davvero, non i bordi sottili di oggi. Contenitori macrofase con sfondo tinto, numerazione grande (01/02/03) e titolo in maiuscolo pesante come nel mock — ma più contrastati del mock stesso.
3. **Gerarchia tipografica con più coraggio** — nomi azienda più grandi e più pesanti (≥0.9375rem/650), conteggi colonna grandi e scuri, label di macrofase come elementi grafici, numeri tabulari ovunque.
4. **Il colore è informazione, mai decorazione**: la rampa segue la progressione del funnel (leggibile a colpo d'occhio "quanto è avanti questa azienda"), i KO restano distinguibili dai warning del registro, e le regole testo-AA di UI-UX §4.2 valgono anche per i token nuovi (varianti `-strong` per il testo).
5. **Deliverable G0**: la direzione visiva si esplora PRIMA del codice — il wireframe di G0 include **2–3 esplorazioni cromatiche complete** della stessa schermata (rampa fredda→calda, monocromatica accent-forte, semantica pura) su cui il panel e l'utente scelgono. I token scelti entrano in `tokens.css` e in un aggiornamento di `docs/UI-UX.md` nello stesso change (regola del documento stesso).

**Criterio di bocciatura esplicito (G0/G2/GF)**: se uno screenshot della board v2 affiancato a quello attuale non produce un salto percepito immediato — più colore, più contrasto, più gerarchia — il gate non passa, indipendentemente dalla qualità del motion.

**Prospettiva dichiarata dall'utente (2026-07-18)**: "questa nuova interfaccia potrebbe essere la rivoluzione per tutta l'interfaccia delle mini app". La kanban v2 è quindi il **pilota di una possibile evoluzione del design system** (UI-UX v3), e i deliverable si costruiscono promuovibili fin dall'inizio:
- i token della rampa cromatica si progettano come **candidati token di sistema** (naming neutro, non binocolo-specifico; definiti in `tokens.css` app-level ma con semantica lift-and-shift verso `packages/ui/src/themes/clean.css`);
- la utility FLIP (`flip.ts`), lo spec di feel del DnD e i pattern optimistic/keyboard si scrivono senza dipendenze da binocolo, pronti a salire in `packages/ui` come primitive condivise;
- le esplorazioni cromatiche di G0 si valutano anche con la domanda "reggerebbe su budget, quotes, compliance?" — il verdetto del panel annota esplicitamente cosa è generalizzabile e cosa è identità locale della board.
Il rollout alle altre mini-app resta **fuori da questo piano** (§12): qui si produce il pilota e la sua documentazione, la decisione di sistema si prende a valle di GF con l'evidenza in mano.

### 6.4 Vincoli design system (bloccanti al gate G2)

Token-only con il set ESTESO di §6.3 (§18: niente literal fuori dalle ricette documentate; i token nuovi vanno aggiunti a `tokens.css` e documentati, non hardcodati); colori semantic e di rampa mai come testo base (§4.2, varianti strong per il testo); tabella con righe keyboard-operable (§13.2); numeri tabulari; copy B2B asciutto italiano; `prefers-reduced-motion` completo; niente entrance replay su refetch/sort/filter (§8.3).

## 7. Frontend — F3: dashboard `/pipeline`

- **Rotta**: `pipeline` in `routes.tsx`; index redirect (`routes.tsx:23`) e catch-all (`:37`) → `/pipeline`. Voce in `App.tsx` `navGroups` come prima voce (gruppo Iniziative: "Pipeline" + "Iniziative").
- **Pagina** `pages/pipeline/PipelinePage.tsx`: riusa `FunnelStrip`, `MacroView`, `StatesView`, `BoardCard` in modalità `readOnly` (prop: niente sensori DnD, niente "+ Aggiungi", niente azioni mutative nel drawer). Chip iniziativa sulla card (visibile in entrambe le densità). Filtro multi-select iniziative (`MultiSelect` condiviso) + ricerca nome + viste 2×2 (persistenza per utente, chiave storage separata dalla board).
- **Drawer read-only**: dati completi (giudizio, provenienze, registro, diario in sola lettura) + «Apri nella board» (`/iniziative/:id`) e «Apri scheda» (`/aziende/:companyKey?iniziativa=...` — scrive la coorte Staffetta con la coorte visibile della pipeline, riusando `writeCohort`).
- Doppia card della stessa azienda (iniziative diverse): resa entrambe, il marker collisione già presente le lega visivamente.
- Deep-link refresh: rotta SPA sotto `/apps/binocolo/`, nessun cambio hosting richiesto.

## 8. F4 — superfici collaterali

- **Indice `/iniziative`** (`IniziativePage.tsx:23-30`): conteggi per tessera raggruppati per macrofase (Origination n · Engagement n · Follow-up n · Esito n) con dettaglio per stato in tooltip.
- **Scheda azienda / /aziende**: label via `cardStates.ts`; `statusPresentation` (AziendePage L26-58) aggiornata al nuovo `buildStatus` (closed → stato terminale + esito in reason).
- **Renderer diario** (board L1398-1441 + drawer): chiavi nuove + `LEGACY_STATE_LABELS` per i payload storici.
- **Wireframe**: `docs/kanban-v2-wireframe.html` (pattern `iniziative-wireframe.html`) prodotto in G0 come fonte di verità per layout, copy e spec di motion.

## 9. QA Gates — panel di esperti (bloccanti)

Meccanica comune: ogni gate è un panel di agenti indipendenti con lenti distinte + verifica adversarial; il verdetto è scritto (finding → remediation → ri-verifica) e il gate non passa finché il panel non approva. I panel si orchestrano con i workflow multi-agente del progetto (mandato esplicito dell'utente, 2026-07-18). Gli smoke che scrivono su DB condiviso li esegue l'utente; agli agent la parte read-only (vincolo di progetto).

| Gate | Quando | Oggetto | Panel (lenti) | Criteri bloccanti |
|---|---|---|---|---|
| **G0** | prima del codice | wireframe HTML + spec motion/keyboard + **2–3 esplorazioni cromatiche** (§6.3) | interaction designer · **impatto visivo** (contrasto/colore/gerarchia, mandato: bocciare il "come oggi") · visual vs `UI-UX.md` (token estesi, AA) · copy B2B · analista M&A (flusso triage→contatto→esito) | spec completa e approvata; ogni gesto ha spec di feel (durata/easing/stato intermedio); direzione cromatica scelta dall'utente fra le esplorazioni |
| **G1** | dopo B1+F1 (+mig 112) | modello stati, migrazione, API | code review adversarial · integrità ground truth calibrazione · convenzioni API/repo · label legacy diario | `go test ./internal/binocolo`, tsc pulito, mapping verificato, nessun filtro SQL residuo su chiavi vecchie (grep `'chiusa'`/`'in_dialogo'`… = 0 fuori da legacy-map e migrazioni) |
| **G2** | dopo F2 | board v2 su browser vivo | interaction/motion (feel del drag, FLIP, latenza) · **impatto visivo** (screenshot v2 vs board attuale: salto immediato percepito o bocciato, §6.3) · visual craft (token estesi, coerenza rampa) · copy · regressione funzionale (tutti i segnali card presenti) · a11y (tastiera completa, AA anche sui token nuovi, reduced-motion) | smoke playwright-cli read-only su dev server attivo (riusato, mai riavviato); ogni finding "sotto Linear" = bloccante |
| **G3** | dopo F3 | dashboard `/pipeline` + rotta default | orientamento (chip iniziativa, doppie card, collisioni) · perf (tutte le iniziative attive, niente N+1, TTI) · coerenza nav/deep-link | read-only garantito (nessuna mutazione possibile dall'aggregato), default route verificata |
| **GF** | fine | confronto testa-a-testa con Linear | panel adversarial con mandato di bocciare: pickup, drop, cambio vista, ricerca, vuoti, tastiera, ritmo, **presenza visiva** (colore, contrasto, gerarchia a confronto diretto) | parità o superiorità percepita su ogni gesto E sul colpo d'occhio; verdetto finale umano (Salvatore) — l'effetto "bocca aperta" lo giudica l'utente |

## 10. Sequenza esecutiva e verifiche

```
G0 (wireframe+spec) ──► B1+F1 (mig 112, stati v2, dominio condiviso) ──► G1
        ──► F2 (board v2) ──► G2 ──► B2+F3 (/pipeline + default route) ──► G3
        ──► F4 (collaterali) ──► GF
```

- Ogni task: `pnpm --filter mrsmith-binocolo exec tsc --noEmit` (mai `npx tsc`), `go build ./... && go test ./internal/binocolo/...` dal backend, UI smoke su dev server **già attivo** (riusarlo, mai killare/riavviare), artefatti Playwright con cwd=`artifacts/claude`.
- La migrazione 112 è consegnata come file; **la applica l'utente** prima del deploy di B1+F1 (frontend e backend nuovi non funzionano su schema vecchio: coordinare come per la 049).
- Gli smoke con scritture (drag reale, chiusure, riaperture su dati reali) li esegue l'utente; l'agent verifica il read-only.
- Branch: lavorare su branch dedicato da `feat/binocolo` (chiedere prima di ogni checkout nel primario, policy di progetto).

## 11. Repo-fit checklist (compilata)

- **Runtime**: nessuna app nuova — rotte interne alla SPA binocolo esistente, deep-link ok sotto `/apps/binocolo/`; nessun href override, catalogo, Makefile o vite.config da toccare.
- **Dev**: nessun cambio porte/proxy; dipendenza nuova solo `@dnd-kit/*` (versioni già presenti nel monorepo).
- **Auth**: namespace `/api/binocolo/v1/ma/...` esistente, ruolo app esistente; l'endpoint pipeline eredita il middleware del modulo.
- **Data contract**: PK card invariata `(initiative_id, company_key)`; ownership nested già verificata dai guard esistenti (`requireOperationalInitiativeCard`); response shapes esplicite (§4); `esito` passa a testo libero per decisione di prodotto (constraint applicativo, non DB).
- **Deploy**: migrazione reale (112), nessun env var nuovo, nessun cambio Docker.
- **Osservabilità**: pattern esistente del modulo (errori sanitizzati, dettaglio nei log, request-id middleware condiviso) — nessun cambiamento di contratto.

## 12. Fuori perimetro (esplicito)

- Mutazioni dall'aggregato (drag, aggiunta) — v2 della dashboard.
- Owner/assegnatario per card (PRD §4.5 resta valido), € per card e somme pipeline, campo "next step" dedicato.
- Reminder/notifiche sulla data di ricontatto (non-CRM).
- Batch deep-dive dal board (già rinviato dal PRD).
- Riscrittura payload storici del diario (gestiti da label-map legacy).
- Derivazione automatica `gia_cliente` da Mistra/Grappa (resta dichiarato manuale).
- **Rollout del nuovo linguaggio visivo alle altre mini-app** (UI-UX v3): questo piano produce il pilota con deliverable promuovibili (§6.3); la decisione di sistema è un'iniziativa separata, da prendere dopo GF con l'evidenza in mano.

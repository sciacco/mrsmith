# Binocolo — Remediation plan v2 · Audit di parità contro i wireframe approvati

> 2026-07-02, sostituisce integralmente la v1 (giudicata insufficiente: elencava funzioni mancanti ovvie invece di misurare la distanza dagli artefatti approvati). **Nessuna implementazione parte da questo documento senza ratifica.**
>
> **Metodo**: confronto schermata-per-schermata tra i wireframe approvati (`gated-ux-wireframe.html`, 10 stati; `iniziative-wireframe.html`, 7 stati) e il prodotto a HEAD `996ce47`, screenshot contro screenshot (`artifacts/claude/wf-*.png` vs `live-*.png`, riproducibili). Gli stati non catturabili senza spesa reale (D1 S2–S6, D2 S7 run in corso) sono confrontati su codice e copy. I difetti funzionali della v1 sono assorbiti in §5.
>
> **Re-baseline (2026-07-02, sera)**: l'audit originale è stato condotto a `996ce47`; tre commit successivi nella stessa giornata hanno spostato il codice. Il documento è ora re-baselinato a HEAD `6217051` — vedi §0 per il diff e i claim corretti inline (F2 e Q7 risolti, F3 e Q5 precisati, S2 marcato pre-rewrite). **La tesi centrale (pattern semantici Q) sopravvive intatta.**
>
> **Re-baseline 2 (2026-07-04)**: riscontro codice a HEAD `24bbdf3` (§0.1). Tre commit successivi a `6217051` hanno **chiuso la maggior parte del perimetro attivo**: Q1, Q2, Q3, Q4 ✅ risolti; F1, F3 ✅ risolti; F4 ✅ committato; Q5 quasi completo (2 residui); Q6 parziale (collasso fatto, nomenclatura da unificare). **Perimetro attivo reale ridotto a: Q5 residuo, Q6, F5, F7, F8.** La tesi centrale è confermata ma il remediation residuo è molto più piccolo di quanto il documento §1–§5 dica inline; le sezioni sono aggiornate con marker `✅ risolto (re-baseline 2)`.

## 0. Re-baseline vs HEAD `6217051` (2026-07-02, sera)

L'audit originale (§1–§9) è stato condotto a HEAD `996ce47` (2026-07-02 17:47). Nella stessa giornata tre commit hanno spostato il codice; questo documento è ora re-baselinato a HEAD `6217051`. **Verdetto globale invariato** — la tesi dei pattern semantici Q sopravvive intatta — ma tre claim sono corretti inline (F2 e Q7 risolti, S2 marcato pre-rewrite) e due precisati (F3, Q5).

**Commit intervenuti dopo `996ce47`:**

- `2dd6d90` (17:44, *pre-audit*) — card-dossier arricchito con financiali/azionisti (+385 righe in `IniziativaCardDossierPage`). Impatto: rendering S6 più ricco; la questione «empty state per dati sganciati» resta, ed è di dati non di codice.
- `79169e2` (21:11) — **lifecycle ricerche** in `RicerchePage.tsx`: archivia/cestino/purge/ripristino + filtri Attive/Archiviate/Cestino + modali di conferma. **Risolve F2.**
- `8284248` (21:47) — **rewrite del board iniziative** (`IniziativaBoardPage.tsx` +529 righe, CSS +467): drag-and-drop, state colors. Invalida la descrizione S2 (pre-rewrite). Confermati a codice a `6217051`: Q2 (truncation ancora assente) regge; contatore toggle ancora assente (Q5). Il resto del board va riverificato visivamente in Fase 4.
- `6217051` (22:07) — styling generale (`Iniziative.module.css`, tema `clean.css`). Impatto visivo residuo sui pattern Q, da riverificare in Fase 4.

**Claim corretti inline (nelle rispettive sezioni):**

| Claim | Esito re-baseline | Sezione |
|---|---|---|
| **F2** | ✅ RISOLTO (`79169e2`) — rimosso dal perimetro | §5 |
| **Q7** | ✅ RISOLTO (tabella usa `<DossierButton>` 3-state, `IniziativaBoardPage.tsx:534`) — rimosso dal perimetro | §4 |
| **F3** | ⚠️ PARZIALE — vista «Archivio (N)» presente, manca l'azione archive/restore per-iniziativa | §5 |
| **Q5** | ⚠️ PARZIALE — pill cliccabili fatte; restano valorizzazione (F1), highlight >0, contatore toggle | §4 |
| **S2** | ⚠️ board riscritto da `8284248`; descrizione pre-rewrite, da riverificare visivamente | §2 |

**Claim confermati intatti a `6217051` (verificati a codice):** F1, F4, F5 (funzionali) · Q1, Q2, Q3, Q4, Q6 (pattern semantici). È la sostanza del remediation e regge.

## 0.1. Riscontro codice vs HEAD `24bbdf3` (2026-07-04)

Riscontro puntuale del codice a HEAD `24bbdf3` (working tree pulito). **Tre commit successivi a `6217051` hanno chiuso la maggior parte del perimetro attivo** dichiarato in §0. Il documento §1–§5 rifletteva lo stato a `6217051` ed è ora storto; questa sezione lo corregge e i claim sono aggiornati inline con marker `✅ risolto (re-baseline 2)`.

**Commit intervenuti dopo `6217051`:**

- `a627066` — *feat(board): enhance last activity display with semantic events*. **Risolve Q1** (backend `ListMALatestCardEvents` + `ma_store.go` `last_activity_event` CASE → etichette umane; board tabella usa `card.lastEvent`).
- `9704883` — *feat(iniziative): enhance initiative management and display*. **Risolve Q2** (`.kcardName`/`.companyNameText`: `text-overflow: ellipsis` + `title` tooltip), **Q3** (`relativeDate()` + `shortAuthor()` in `helpers.ts`, usati in S1/diario/drawer), **Q4** (`eventLabel()` in `IniziativaBoardPage.tsx:1160`: stato da→a, chiusura esito+nota, card_creata sessione+stelle+nota, nota inline, contattato/buon_lead/no_go; scheda azienda cita ultima nota con autore breve + data relativa), **F1** (backend aggrega `counts` jsonb per stato; frontend `item.counts[state.key]`), **F3** (`onArchive`/`onRestore` per-iniziativa in `IniziativaCard` + endpoint `POST .../archive`/`.../restore`).
- `24bbdf3` — *docs(remediation): update F4 status and remove company link*. **Risolve F4** (committato, non più working tree): rimozione link `/azienda?vat=` + badge B5 inline in `TargetDetailModal`.

**Claim aggiornati (re-baseline 2):**

| Claim | Esito riscontro codice `24bbdf3` | Sezione |
|---|---|---|
| **Q1** | ✅ RISOLTO (`a627066`) — `last_activity_event` backend + `lastEvent` board | §4 |
| **Q2** | ✅ RISOLTO (`9704883`) — `ellipsis` + `title` su `.kcardName`/`.companyNameText` | §4 |
| **Q3** | ✅ RISOLTO (`9704883`) — `relativeDate()` + `shortAuthor()` | §4 |
| **Q4** | ✅ RISOLTO (`9704883`) — `eventLabel()` completo + citazione ultima nota | §4 |
| **Q5** | ⚠️ QUASI COMPLETO — pill cliccabili ✓, valorizzate ✓ (F1), highlight >0 ✓ (`statePillActive`); **restano**: contatore sul toggle Kanban/Tabella, e click pill → `onOpen` apre il board **non filtrato** per stato (title dice "filtrato" ma l'azione non filtra) | §4 |
| **Q6** | ⚠️ PARZIALE — collasso ✓ (`collapsed`, header cliccabile, riga-riepilogo); **nomenclatura ancora duplice**: legenda funnel "Da approfondire" vs tabella/chip "Da verificare"; "In tesi"/"Fuori tesi" per principale/reject; numeri funnel da verificare | §4 |
| **F1** | ✅ RISOLTO (`9704883`) — `counts` aggregati e renderizzati | §5 |
| **F3** | ✅ RISOLTO (`9704883`) — archive/restore per-iniziativa + endpoint; delete assente (corretto per D-A) | §5 |
| **F4** | ✅ COMMITTATO (`24bbdf3`) — non più working tree | §5 |
| **F5** | ⚠️ APERTO confermato — `ListMAActiveCardsByCompany` non joina `ma_initiative` per escludere archiviate | §5 |

**Perimetro attivo reale dopo il re-baseline 2:**

- Pattern Q: **Q6** attivo (nomenclatura duplice + numeri funnel) · **Q5** quasi fatto (2 residui: contatore toggle, filtraggio stato dal click pill) · Q1, Q2, Q3, Q4, Q7 fatti.
- Difetti F: **F5** attivo · F7 igiene (SQL §8) · F8 da spot-check · F1, F2, F3, F4 fatti · F6 ritirata.

**Impatto sulla Fase 4 (riverifica di parità):** Q1–Q4 erano dichiarati "attivi" sulla base di `6217051` ma sono ora implementati — vanno **verificati visivamente** (non più implementati) nella Fase 4, insieme agli stati non ancora visti (S6 registro renderizzato, D1 S2–S6, D2 S7, badge/marker/collisioni con dati veri).

## 1. Verdetto

Il costruito si divide in tre fasce nette (aggiornato al re-baseline 2, HEAD `24bbdf3`):

- **A standard (da preservare)**: D1 stato iniziale, modali di chiusura/rimozione, coda di verifica D2, impianto kanban (colonne flessibili/rail), backend dei flussi card e schema 087–090.
- **Sotto lo standard approvato (rimediato quasi tutto)**: i pattern di qualità Q1–Q4 sono ora **implementati** (`a627066`/`9704883`): eventi semantici, conteggi valorizzati e evidenziati, date relative, diario che racconta, troncamento ragioni sociali. **Restano sotto standard**: Q6 (nomenclatura bucket duplice nella legenda funnel vs tabella) e i 2 residui di Q5 (contatore toggle, filtraggio stato dal click pill). Il danno residuo è piccolo e puntuale, non più «il grosso».
- **Mancante**: F5 (marker ignora iniziative archiviate), Q6 nomenclatura, Q5 residui; igiene dati di prova (F7, SQL tua); spot-check minori (F8). Il ciclo di vita iniziative (F3) e i contatori (F1) sono ora **fatti**.

**Raccomandazione: si recupera, e in buona parte è già recuperato.** Il modello dati e i flussi reggono (smoke end-to-end); i gap residui sono di resa puntuale (Q6 nomenclatura, Q5 toggle) e di un fix di query (F5 join su `ma_initiative`). Il `TargetDetailModal` di D2 è già disaccoppiato (F4 committato `24bbdf3`): link `/azienda` rimosso, badge B5 inline, dossier profondo resta azione di card sul MA card-dossier autosufficiente (D-B ratificata).

## 2. Audit per schermata — Iniziative (wireframe S1–S7)

### S1 · Indice iniziative — ✅ RISOLTO (re-baseline 2)
| Wireframe (approvato) | Live (`24bbdf3`) | Gap |
|---|---|---|
| Conteggi per stato valorizzati, pill dello stato attivo evidenziata | ✅ Conteggi valorizzati (`item.counts[state.key]`), pill `statePillActive` quando >0 | F1 ✅, Q5 highlight ✅ |
| Pill **cliccabili → board filtrato** («non contatori decorativi», nota 2) | ⚠️ Cliccabili (`onClick={onOpen}`) ma aprono il board **non filtrato** per stato (title dice «filtrato») | Q5 residuo |
| «Ultima attività: **nota su Nexa Systems S.r.l.**» — semantica | ✅ `Ultima attività: {item.lastActivityEvent}` (backend CASE → etichetta umana) | Q1 ✅ |
| «aggiornata 2 g fa» (relativa) | ✅ `aggiornata {relativeDate(item.updatedAt)}` | Q3 ✅ |
| Archiviazione come uscita di scena (nota 3) | ✅ Bottoni Archivia/Ripristina per-iniziativa + endpoint | F3 ✅ |
| Empty state e modal Nuova iniziativa | Verbatim ✓ | — |

### S2 · Board kanban — ✅ Q2 RISOLTO (re-baseline 2); board da riverificare visivamente in Fase 4
> ⚠️ **Re-baseline 2**: a `24bbdf3` la truncation Q2 è **implementata** (`.kcardName`/`.companyNameText`: `text-overflow: ellipsis; white-space: nowrap; overflow: hidden; max-width: 100%` + `title={companyName}`). Contatori colonna ✓ (`kcolCount`), collasso ✓, DnD ✓. **Contatore sul toggle Kanban/Tabella ancora assente** → Q5 residuo confermato. Badge registro, marker collisione, rendering visivo del board riscritto da `8284248`: da riverificare in Fase 4 con dati reali.

_(Audit a `996ce47`, pre-rewrite):_ Colonne flessibili, rail, contatori colonna, collasso ricordato: ✓ conformi. Gap storico: ragioni sociali mai troncate (la card KRAL era un francobollo verticale) → **Q2 ora risolto**.

### S3 · Board tabella — ✅ Q1/Q2/Q3 RISOLTI (re-baseline 2); residui minori
Filtri e colonne presenti ✓. ✅ «Ultima attività» = `card.lastEvent || dateLabel(card.updatedAt)` (semantica + relativa) → Q1+Q3 risolti; ✅ ragioni sociali troncate (`.companyNameText` ellipsis) → Q2 risolto; colonna Dossier: ✅ `<DossierButton>` 3-state (Q7). **Residui**: stato «rimossa» in minuscolo, senza pill esito (wireframe: «Chiusa ~Non idonea~» rossa, «Rimandata» ambra); layout filtri (cerca full-width su riga separata) più sciatto del compatto approvato → F8.

### S4 · Drawer card — ✅ Q4/Q3/Q2 RISOLTI (re-baseline 2)
| Wireframe | Live (`24bbdf3`) | Gap |
|---|---|---|
| Diario che racconta: «**Contattata** — telefonata col titolare…», «**Card creata** da MSP Lombardia · giu (★★★)» | ✅ `eventLabel()` renderizza stato da→a, chiusura esito+nota, card_creata sessione+stelle+nota, nota inline, contattato/buon_lead/no_go | Q4 ✅ |
| Autore breve + data breve (`g.rossi · 30 giu 2026`) | ✅ `shortAuthor()` (email→nome.cognome) + `relativeDate()` | Q4+Q3 ✅ |
| Sezione PROVENIENZE sempre presente (righe: sessione · mese — ★★☆ score) | Sezione **omessa** quando vuota | ~~F6~~ ritirata (falso problema) |
| Scheda azienda: fatti + «ultima nota d'azienda» citata con data | ✅ Citazione «Ultima nota: “…” · {shortAuthor} · {relativeDate}» implementata | Q4 ✅ |
| Titolo compatto | ✅ `.kcardName`/titoli con ellipsis + tooltip | Q2 ✅ |
| Stato a pill, «Chiusa…» apre S5, rimozione, analisi a 3 stati | ✓ | — |

### S5 · Chiusura ed esiti · Rimozione — FEDELE
Copy dei 5 esiti verbatim ✓, ponte registro solo su No-go/Rimandata (verificato nello smoke, incluso il non-mostrarsi su Non idonea) ✓, rimozione con correzione-stella e copy verbatim ✓. Minori: ordine bottoni invertito rispetto al wireframe; titolo con ragione sociale non troncata (Q2).

### S6 · Registro azienda nel dossier — NON VERIFICABILE OGGI
Il registro è migrato (correttamente, dopo la ratifica del 2026-07-02) dalla pagina `/azienda` alla pagina card-dossier. Ma la pagina card-dossier oggi renderizza solo l'empty state (le card di prova hanno la sessione sganciata) → la parità S6 (badge, revoca con conferma, storico dietro toggle, composer nota) **non è mai stata vista renderizzata**. Va coperta nella riverifica di parità con dati reali.

### S7 · Superfici esistenti — PARZIALE
D1: card «Iniziativa (opzionale)» presente con hint verbatim ✓ (flash di caricamento del dropdown al primo render → F8). Indice ricerche: «Aggancia a iniziativa…» ✓, chip su agganciate ✓. D2: badge registro e marker «In lavorazione · titolo» ✓ (verificati nello smoke) — ma il marker ignora le iniziative archiviate → F5.

## 3. Audit per schermata — Gated D1/D2 (wireframe S1–S10)

### S1 · Richiesta NL — FEDELE
Copy, guida, esempio, campo unico col pallino rosso: verbatim ✓.

### S2–S6 · Perimetro, stima, degradati — CONFRONTO SU CODICE (stati a pagamento)
Non catturabili senza lanciare stime/run. Copy dei componenti presente a codice; la verifica visiva va fatta al prossimo run reale che eseguirai, con la checklist §6 fase 4 alla mano. Nessun giudizio emesso qui: **buco di copertura dichiarato**, non assolto.

### S7 · Run in corso — NON CATTURABILE (idem sopra)

### S8 · Run completato · Risultati — ⚠️ PARZIALE (re-baseline 2): collasso fatto, nomenclatura duplice resta
Il wireframe prescrive: «il funnel **collassa a riepilogo** [riga compatta ri-espandibile]; il lavoro diventa protagonista». ✅ **Collasso implementato** (`collapsed`, header cliccabile, «Esecuzione completata» vs «Avanzamento», riga-riepilogo compatta con Superficie/Valutate/Analizzate/Fuori tesi). **Residuo Q6**: la legenda della progress usa ancora una nomenclatura duplice — «Da approfondire» in `BucketLegend` (~riga 637) vs «Da verificare» in tabella/chip (`bucketLabel`/`bucketChipLabel` in `helpers.ts`); e «In tesi»/«Fuori tesi» per principale/reject. Due vocabolari nella stessa schermata → da unificare su quello ratificato (azionabile/da verificare/soppresso). E la riga funnel mostra «oltre il gate 1 → analizzate 5»: **i numeri non quadrano per costruzione**, contro la nota 1 del wireframe → da spiegare o correggere (Q6).

### S9 · Coda di verifica — FEDELE
Motivo derivato verbatim («Identità non confermata sulle pagine lette», «Sito irraggiungibile…») ✓, rimedi Associa dominio / Nessun sito ✓. Non verificati visivamente (nessun caso nei dati): «Conferma sito di gruppo» quando c'è l'hint, «Riprova analisi», stato re-processo per-riga → checklist fase 4.

### S10 · Fuori tesi — presente, vuoto nei dati di prova: parità da verificare con una sessione con scarti reali.

## 4. Pattern trasversali di qualità (Q) — stato a `24bbdf3` (re-baseline 2)

- **Q1 — Ultima attività semantica** · ✅ **RISOLTO** (`a627066`): backend `ma_store.go` calcola `last_activity_event` (CASE WHEN su eventi → «Card creata», «No-go: …», «Buon lead: …»); `IniziativePage` renderizza «Ultima attività: {lastActivityEvent}»; board tabella usa `card.lastEvent` da `ListMALatestCardEvents`.
- **Q2 — Ragioni sociali** · ✅ **RISOLTO** (`9704883`): `.kcardName`/`.companyNameText` con `text-overflow: ellipsis; white-space: nowrap; overflow: hidden; max-width: 100%` + `title={companyName}` tooltip ovunque (kanban, tabella, titoli).
- **Q3 — Date** · ✅ **RISOLTO** (`9704883`): `relativeDate()` («adesso»/«min fa»/«h fa»/«ieri»/«g fa»/data breve) + `shortAuthor()` (email→nome.cognome, default «Sistema») in `helpers.ts`; usati in S1, diario, drawer.
- **Q4 — Diario che racconta** · ✅ **RISOLTO** (`9704883`): `eventLabel()` (`IniziativaBoardPage.tsx:1160`) gestisce stato da→a, chiusura esito+nota, card_creata sessione+stelle+nota, nota inline, contattato/buon_lead/no_go; scheda azienda cita «Ultima nota: “…” · {shortAuthor} · {relativeDate}».
- **Q5 — Conteggi azionabili** · ⚠️ **QUASI COMPLETO (re-baseline 2)**: pill cliccabili ✓ (`onClick`), pill **valorizzate** ✓ (F1 risolto), **evidenziate quando >0** ✓ (`statePillActive`). **Restano 2 residui**: (a) **contatore sul toggle Kanban/Tabella** (il toggle mostra solo «Kanban»/«Tabella», nessun numero); (b) **click pill → board filtrato per stato** — oggi `onClick={onOpen}` apre il board **non filtrato** (il `title` dice «Vai al board filtrato su X» ma l'azione non passa lo stato).
- **Q6 — Funnel S8** · ⚠️ **PARZIALE (re-baseline 2)**: ✅ collasso a riga-riepilogo a run completato (ri-espandibile, header cliccabile); **residui**: UNA nomenclatura bucket — la legenda funnel usa ancora «Da approfondire» vs «Da verificare» in tabella/chip (e «In tesi»/«Fuori tesi» per principale/reject) → da unificare su quella ratificata (azionabile/da verificare/soppresso); numeri del funnel che quadrano o spiegano lo scarto.
- **Q7 — Azioni contestuali in tabella** · ✅ **RISOLTO al re-baseline**: la vista tabella usa `<DossierButton>` con il ciclo a 3 stati. **Fuori perimetro.**

## 5. Difetti funzionali (dalla v1, verificati a codice)

- **F1 — Contatori iniziative** · ✅ **RISOLTO** (`9704883`): l'aggregazione card per stato è in `ListMAInitiatives` (`ma_store.go`: `jsonb_object_agg` per stato → `item.counts`); il frontend renderizza `item.counts[state.key] ?? 0` con highlight `statePillActive` quando >0. Sblocca Q5.
- **F2 — Ciclo di vita ricerche in /ricerche** · ✅ **RISOLTO al re-baseline** (commit `79169e2`, 2026-07-02): `RicerchePage.tsx` ha ora archivia/cestino/purge/ripristino + filtri di visibilità (Attive/Archiviate/Cestino) + modali di conferma. **Fuori perimetro.** (L'audit a `996ce47` lo rilevava ancora aperto: era il gap reale a quel HEAD, chiuso nelle ore successive.)
- **F3 — Ciclo di vita iniziative** · ✅ **RISOLTO** (`9704883`): `IniziativaCard` espone `onArchive`/`onRestore` con bottoni «Archivia»/«Ripristina» per-iniziativa; endpoint backend `POST .../archive` + `.../restore` (`handler.go:121-122`); vista archivio con link «Archivio (N)» + conteggio + empty state. Delete assente (corretto per D-A: archive + restore, no delete/purge).
- **F4 — Dettaglio D2** · ✅ **COMMITTATO** (`24bbdf3`, 2026-07-04): il `TargetDetailModal` (`RicercaDetailPage.tsx`) non ha più il link `/azienda?vat=` — sostituito dai badge B5 inline (`registryFacts` + `inLavorazione`), riusando il pattern già presente nella tabella risultati (`cellBadges` + `registryFactLabels` + `badgeWarn`/`badgeInfo`/`badgeLav`). Il modale resta magro nel ruolo di setaccio (razionale aderenza + contesto scoring + badge B5); il dossier profondo resta azione di card sul MA card-dossier, già autosufficiente e disaccoppiato (`GetMATargetByID` → cache company-keyed via `ListMADeepAnalysis`). Wording PRD §7 aggiornato in committed code (`INIZIATIVE-PRD.md:105`: «Il tool standalone `/azienda` è indipendente da MA e non è accoppiato al flusso (ratifica D-B)»). **Fuori perimetro.**
- **F5 — Marker vs archiviazione** · ⚠️ **APERTO confermato (re-baseline 2)**: `ListMAActiveCardsByCompany` (`ma_store.go`) filtra `state NOT IN ('chiusa','rimossa')` ma **non joina `ma_initiative`** per escludere le iniziative archiviate → le card di iniziative archiviate ancora appaiono come marker di collisione. Fix: join su `ma_initiative` con `archived_at IS NULL`.
- **F6 — Provenienze robuste** · ❌ **RITIRATA (2026-07-02)**: stesso falso problema di D-D. Il rimedio (leggere dallo snapshot invece che dalle sessioni agganciate) servirebbe solo a rendere la card indipendente dalla sessione — ma non è il modello implementato (la card risolve i dettagli attraverso la sessione agganciata). Per le operazioni supportate (archive, purge soft) le provenienze **non** si perdono: la sessione resta con `initiative_id` intatto e le query filtrano solo per quello. Lo sgancio — l'unico caso che rompe — non è supportato. (Se in futuro si volesse supportarlo, servirebbe uno snapshot completo su card: decisione «cambia modello», non un task di remediation.)
- **F7 — Igiene dati di prova**: SQL in §8 (include anche i 2 cambi di stato accidentali fatti oggi durante l'audit da click su riferimenti browser stantii — errore mio, registrato).
- **F8 — Lotto minori**: gli 8 della v1 + flash dropdown D1 + ordine bottoni S5.

## 6. Piano di esecuzione proposto (aggiornato al re-baseline 2, HEAD `24bbdf3`)

1. **Fase 0 — Ratifica**: ✅ chiusa (D-A, D-B, D-C ratificate; D-D ritirata).
2. **Fase 1 — Igiene** (immediata): riga `.gitignore` `**/.playwright-cli/` ✅ fatta (2026-07-02); SQL §8 (tu, one-shot) — copre l'iniziativa audit `aac1ef5b…`; **verifica anche eventuali altri dati di prova** residui dai smoke test recenti prima di dichiarare la fase chiusa.
3. **Fase 2 — Pattern Q residui**: **Q6** (unificare nomenclatura bucket: legenda funnel «Da approfondire» → «Da verificare», allineare «In tesi»/«Fuori tesi»; numeri funnel che quadrano) + **Q5 residui** (contatore sul toggle Kanban/Tabella; click pill → board filtrato per stato). Q1, Q2, Q3, Q4, Q7 ✅ fatti (da verificare visivamente in Fase 4, non più da implementare).
4. **Fase 3 — Funzionali residui**: **F5** (join `ma_initiative` in `ListMAActiveCardsByCompany` per escludere archiviate). F1, F2, F3, F4 ✅ fatti · F6 ritirata · F7 = igiene Fase 1.
5. **Fase 4 — Riverifica di parità formale**: per OGNI stato dei due wireframe, screenshot fianco a fianco e checklist puntuale. **Inclusi i pattern Q1–Q4 ora implementati** (verifica visiva, non più implementazione) e gli stati oggi non verificabili: S6 registro renderizzato, D1 S2–S6 e S7 al primo run reale, badge/marker/collisioni con dati veri, rendering del board riscritto da `8284248`. **Questa checklist è il gate di accettazione: niente «fatto» senza il confronto visivo.** La eseguo io direttamente.

Sequenza e granularità dei task esecutori le definiamo dopo la ratifica — non prima, per non ripetere l'errore di piani che promettono ciò che non specificano.

## 7. Decisioni richieste

> **Stato (2026-07-02)**: D-A, D-B, D-C ✅ ratificate · D-D ❌ ritirata (falso problema).

- **D-A** · ✅ **RATIFICATA (2026-07-02)**: **archive + restore, nessuna rotta delete/purge**. Lo schema rende l'hard-delete incoerente (`ma_initiative_card` CASCADE ma `ma_target_outcome.initiative_id` SET NULL → diario orfano); archive è l'uscita disegnata (S1 nota 3), restore l'undo. Pulizia dati di prova = SQL one-shot (§8), non meccanismo di prodotto. Ortogonale a F3 (che cabla l'azione archive/restore in UI).
- **D-B** · ✅ **RATIFICATA (2026-07-02)**: nessuna delle due. Si **rimuove il link `/azienda`** dal modale D2 (`RicercaDetailPage.tsx:974`) — `CompanyDossierPage` è tool standalone, non va accoppiato a MA. Il modale resta magro nel ruolo di setaccio (razionale + badge B5); il dossier profondo resta azione di card sul MA card-dossier, **già autosufficiente** (`GetMATargetByID` → cache company-keyed). Lavoro minimo: vedi F4. Revisiona PRD §7 («dal dossier `/azienda`» → «dal MA card-dossier»).
- **D-C** · ✅ **RATIFICATA (2026-07-02)**: **assorbito dal gate**. Il vecchio bottone manuale (`TargetPage:609` → `web-validation/enrich`) chiamava lo stesso endpoint del gate; il nuovo `/ricerche` non lo re-espone. La web-validation è una preoccupazione **gate-time** (alimenta il bucketing), fatta in automatico dallo stage `enrich` della pipeline + resume-on-failure; la correzione per-target sta nella coda di verifica S9 + `PUT web-validation`. L'endpoint resta (lo usa la pipeline). **Nessuna azione**: non si re-aggiunge il bottone. (Unica perdita: re-enrich bulk di una ricerca già completata — caso di nicchia, v2 se emerge.)
- **D-D** · ❌ **RITIRATA (2026-07-02) — falso problema**: lo sgancio sessione↔iniziativa non è coerente con l'implementazione. I dettagli della card (target, sessione, provenienze) si risolvono attraverso la sessione **agganciata** (`getCardDossier` → `FindMALatestTargetForCard`; `ListMACardProvenances` → `WHERE session.initiative_id = $1`); il detach (`initiative_id = NULL`) spezza quella risoluzione e la card perde i collegamenti. Lo sgancio non è un'operazione supportata: niente modale, niente toast, niente «sorte delle card». Nota: **archive e purge (soft) NON rompono la card** — non cancellano la riga, lasciano `initiative_id` intatto, e le query di risoluzione filtrano solo per `initiative_id` (non per visibilità). Conseguenze accurate: (1) **F6 diventa moot** — la sua premessa (provenienze perse al detach) descrive un'operazione non supportata; (2) UI «Aggancia a iniziativa…» da rendere **attach-only** (niente unanchor); (3) wording PRD §3.2 «spostamento» da rivedere (incoerente quando la sessione ha già generato card).

## 8. SQL di pulizia dati di prova (esegui tu, in transazione)

```sql
BEGIN;
DELETE FROM binocolo.ma_target_outcome
WHERE initiative_id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43';

DELETE FROM binocolo.ma_initiative_card
WHERE initiative_id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43';

UPDATE binocolo.ma_session SET initiative_id = NULL
WHERE initiative_id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43';

DELETE FROM binocolo.ma_initiative
WHERE id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43';

DELETE FROM binocolo.ma_company_fact
WHERE company_key = '64466166DD8F310FC4298669'
  AND kind = 'non_vende'
  AND created_at::date = DATE '2026-07-02';
COMMIT;

-- verifica post: tutte le colonne a 0
SELECT
  (SELECT count(*) FROM binocolo.ma_initiative      WHERE id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43')  AS iniziativa,
  (SELECT count(*) FROM binocolo.ma_initiative_card WHERE initiative_id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43') AS card,
  (SELECT count(*) FROM binocolo.ma_target_outcome  WHERE initiative_id = 'aac1ef5b-9d8f-47cc-a393-debc62e34f43') AS eventi,
  (SELECT count(*) FROM binocolo.ma_company_fact    WHERE company_key = '64466166DD8F310FC4298669') AS fatti_cremanet;
```

## 9. Root cause della deriva (per non ripeterla)

1. **La QA verificava i piani, non i wireframe.** I task F dicevano «copy verbatim dal wireframe» e la QA controllava le stringhe — ma nessuno ha mai messo wireframe e pagina fianco a fianco. I pattern semantici (Q1, Q4, Q5, Q6) non sono «copy»: sono il design, e sono passati persi.
2. **I piani promettevano completezza che non specificavano** (contatori tra B1 e B2, ciclo di vita mai messo a piano, parità dettaglio D2 mai scritta).
3. **Il censimento funzionale della pagina sostituita non è mai stato eseguito** prima di dichiarare chiuso il trasferimento.
4. **Igiene di test insufficiente**: dati di prova su un modello senza cancellazione; oggi, durante l'audit stesso, 2 scritture accidentali da click su riferimenti browser stantii (coperte dalla SQL §8).

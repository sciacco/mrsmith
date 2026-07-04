# Binocolo — Remediation plan v2 · Audit di parità contro i wireframe approvati

> 2026-07-02, sostituisce integralmente la v1 (giudicata insufficiente: elencava funzioni mancanti ovvie invece di misurare la distanza dagli artefatti approvati). **Nessuna implementazione parte da questo documento senza ratifica.**
>
> **Metodo**: confronto schermata-per-schermata tra i wireframe approvati (`gated-ux-wireframe.html`, 10 stati; `iniziative-wireframe.html`, 7 stati) e il prodotto a HEAD `996ce47`, screenshot contro screenshot (`artifacts/claude/wf-*.png` vs `live-*.png`, riproducibili). Gli stati non catturabili senza spesa reale (D1 S2–S6, D2 S7 run in corso) sono confrontati su codice e copy. I difetti funzionali della v1 sono assorbiti in §5.
>
> **Re-baseline (2026-07-02, sera)**: l'audit originale è stato condotto a `996ce47`; tre commit successivi nella stessa giornata hanno spostato il codice. Il documento è ora re-baselinato a HEAD `6217051` — vedi §0 per il diff e i claim corretti inline (F2 e Q7 risolti, F3 e Q5 precisati, S2 marcato pre-rewrite). **La tesi centrale (pattern semantici Q) sopravvive intatta.**
>
> **Re-baseline 2 (2026-07-04)**: riscontro codice a HEAD `24bbdf3` (§0.1). Tre commit successivi a `6217051` hanno **chiuso la maggior parte del perimetro attivo**: Q1, Q2, Q3, Q4 ✅ risolti; F1, F3 ✅ risolti; F4 ✅ committato; Q5 quasi completo (2 residui); Q6 parziale (collasso fatto, nomenclatura da unificare). **Implementazione completata 2026-07-04**: Q6 nomenclatura implementata (alternativa D); F5 implementato; Q5 residui ritirati (falso problema); F8(1,2,4) risolti (stato «rimossa» pill, filtri compatti, flash dropdown). **Perimetro attivo reale ridotto a: F8(3) ordine bottoni S5 (→ Fase 4) + Fase 4 (riverifica visiva).** La tesi centrale è confermata; il remediation di implementazione è esaurito.
>
> **🔒 CHIUSURA (2026-07-04, decisione utente)**: il piano è dichiarato **chiuso**. L'implementazione del remediation è esaurita (Q1–Q7, F1–F5, F8(1,2,4)). La **Fase 4 (riverifica visiva formale)** è **saltata per decisione utente** — gli stati non ancora visti renderizzati (S6 registro, D1 S2–S6, D2 S7 con dati reali, board riscritto da `8284248`, Q6 catena numerica) restano **non verificati visivamente** e si copriranno operativamente all'uso reale. **F8(3)** (ordine bottoni S5 — allinearsi al wireframe o tenere primary-a-destra) resta **decisione UX aperta**, da prendere se/quando emerge. Il documento resta come record: nessuna ulteriore azione di remediation pianificata.

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
| **Q5** | ✅ RISOLTO — pill cliccabili ✓, valorizzate ✓ (F1), highlight >0 ✓; residui (contatore toggle, filtro da pill) ritirati (falso problema, vedi §4) | §4 |
| **Q6** | ✅ NOMENCLATURA RISOLTA (working tree, 2026-07-04, alternativa D) — etichette gate distinte in legenda + riga collassata routing; residuo catena numerica → Fase 4 | §4 |
| **F1** | ✅ RISOLTO (`9704883`) — `counts` aggregati e renderizzati | §5 |
| **F3** | ✅ RISOLTO (`9704883`) — archive/restore per-iniziativa + endpoint; delete assente (corretto per D-A) | §5 |
| **F4** | ✅ COMMITTATO (`24bbdf3`) — non più working tree | §5 |
| **F5** | ✅ RISOLTO (working tree, 2026-07-04) — join `ma_initiative` + `archived_at IS NULL` | §5 |

**Perimetro attivo reale dopo il re-baseline 2:**

- Pattern Q: **tutti fatti** (Q1, Q2, Q3, Q4, Q5, Q6 nomenclatura, Q7) · Q5 residui ritirati (falso problema) · Q6 catena numerica → Fase 4.
- Difetti F: F8(3) residuo (ordine bottoni S5, → Fase 4) · F1, F2, F3, F4, F5, F7, F8(1,2,4) fatti · F6 ritirata.

**Impatto sulla Fase 4 (riverifica di parità):** Q1–Q4 erano dichiarati "attivi" sulla base di `6217051` ma sono ora implementati — vanno **verificati visivamente** (non più implementati) nella Fase 4, insieme agli stati non ancora visti (S6 registro renderizzato, D1 S2–S6, D2 S7, badge/marker/collisioni con dati veri).

## 1. Verdetto

Il costruito si divide in tre fasce nette (aggiornato al re-baseline 2, HEAD `24bbdf3`):

- **A standard (da preservare)**: D1 stato iniziale, modali di chiusura/rimozione, coda di verifica D2, impianto kanban (colonne flessibili/rail), backend dei flussi card e schema 087–090.
- **Sotto lo standard approvato (rimediato quasi tutto)**: i pattern di qualità Q1–Q6 sono ora **implementati** (`a627066`/`9704883` + Q6 nomenclatura 2026-07-04): eventi semantici, conteggi valorizzati e evidenziati, date relative, diario che racconta, troncamento ragioni sociali, nomenclatura funnel unificata. Il danno residuo è pressoché nullo in implementazione — resta solo la **riverifica visiva** (Fase 4).
- **Mancante**: F8(3) ordine bottoni S5 (decisione UX → Fase 4) + riverifica visiva (Fase 4). Il ciclo di vita iniziative (F3), i contatori (F1), i marker vs archiviate (F5), i pattern Q1–Q6 e i minori F8(1,2,4) sono ora **fatti**.

**Raccomandazione: si recupera, ed è quasi tutto recuperato.** Il modello dati e i flussi reggono (smoke end-to-end); i gap di implementazione sono chiusi (Q1–Q6, F1–F5, F8(1,2,4)). Resta la **riverifica visiva formale** (Fase 4) con dati reali e l'unica decisione di resa pendente F8(3) (ordine bottoni S5). Il `TargetDetailModal` di D2 è già disaccoppiato (F4 committato `24bbdf3`): link `/azienda` rimosso, badge B5 inline, dossier profondo resta azione di card sul MA card-dossier autosufficiente (D-B ratificata).

## 2. Audit per schermata — Iniziative (wireframe S1–S7)

### S1 · Indice iniziative — ✅ RISOLTO (re-baseline 2)
| Wireframe (approvato) | Live (`24bbdf3`) | Gap |
|---|---|---|
| Conteggi per stato valorizzati, pill dello stato attivo evidenziata | ✅ Conteggi valorizzati (`item.counts[state.key]`), pill `statePillActive` quando >0 | F1 ✅, Q5 highlight ✅ |
| Pill **cliccabili → board filtrato** («non contatori decorativi», nota 2) | ✅ Cliccabili (`onClick={onOpen}`) → navigano al board. Il salto «filtrato per stato» ritirato (falso problema: il board default è kanban, il filtro stato vive solo nella vista tabella → pre-set incoerente; le pill già navigano, non sono decorative) | Q5 ✅ (residuo ritirato) |
| «Ultima attività: **nota su Nexa Systems S.r.l.**» — semantica | ✅ `Ultima attività: {item.lastActivityEvent}` (backend CASE → etichetta umana) | Q1 ✅ |
| «aggiornata 2 g fa» (relativa) | ✅ `aggiornata {relativeDate(item.updatedAt)}` | Q3 ✅ |
| Archiviazione come uscita di scena (nota 3) | ✅ Bottoni Archivia/Ripristina per-iniziativa + endpoint | F3 ✅ |
| Empty state e modal Nuova iniziativa | Verbatim ✓ | — |

### S2 · Board kanban — ✅ Q2 RISOLTO (re-baseline 2); board da riverificare visivamente in Fase 4
> ⚠️ **Re-baseline 2**: a `24bbdf3` la truncation Q2 è **implementata** (`.kcardName`/`.companyNameText`: `text-overflow: ellipsis; white-space: nowrap; overflow: hidden; max-width: 100%` + `title={companyName}`). Contatori colonna ✓ (`kcolCount`), collasso ✓, DnD ✓. Contatore sul toggle Kanban/Tabella: **ritirato** (falso problema — il totale card è già nell'h1 `({board.cards.length})`, duplicarlo sul toggle è rumore senza informazione). Badge registro, marker collisione, rendering visivo del board riscritto da `8284248`: da riverificare in Fase 4 con dati reali.

_(Audit a `996ce47`, pre-rewrite):_ Colonne flessibili, rail, contatori colonna, collasso ricordato: ✓ conformi. Gap storico: ragioni sociali mai troncate (la card KRAL era un francobollo verticale) → **Q2 ora risolto**.

### S3 · Board tabella — ✅ Q1/Q2/Q3 RISOLTI; residuo F8(3) allineamento filtri fatto
Filtri e colonne presenti ✓. ✅ «Ultima attività» = `card.lastEvent || dateLabel(card.updatedAt)` (semantica + relativa) → Q1+Q3 risolti; ✅ ragioni sociali troncate (`.companyNameText` ellipsis) → Q2 risolto; colonna Dossier: ✅ `<DossierButton>` 3-state (Q7). ✅ **F8(1)**: stato «rimossa» ora renderizza pill «Rimossa» (`.statusMuted`) invece del grezzo minuscolo. ✅ **F8(2)**: layout filtri compattato (`.filters` inline gap `--space-2`, search `min-width:200px`). **Residuo F8(3)**: stato «rimossa» in minuscolo risolto; resta ordine bottoni S5 (vedi §5 F8).

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
D1: card «Iniziativa (opzionale)» presente con hint verbatim ✓ (✅ **F8(4) risolto**: flash dropdown eliminato — `initiativesLoading` + `disabled` + option «Caricamento…» in `NuovaRicercaPage`). Indice ricerche: «Aggancia a iniziativa…» ✓, chip su agganciate ✓. D2: badge registro e marker «In lavorazione · titolo» ✓ (verificati nello smoke) — ✅ F5 risolto: il marker ora esclude le iniziative archiviate (join `ma_initiative` + `archived_at IS NULL`).

## 3. Audit per schermata — Gated D1/D2 (wireframe S1–S10)

### S1 · Richiesta NL — FEDELE
Copy, guida, esempio, campo unico col pallino rosso: verbatim ✓.

### S2–S6 · Perimetro, stima, degradati — CONFRONTO SU CODICE (stati a pagamento)
Non catturabili senza lanciare stime/run. Copy dei componenti presente a codice; la verifica visiva va fatta al prossimo run reale che eseguirai, con la checklist §6 fase 4 alla mano. Nessun giudizio emesso qui: **buco di copertura dichiarato**, non assolto.

### S7 · Run in corso — NON CATTURABILE (idem sopra)

### S8 · Run completato · Risultati — ✅ NOMENCLATURA RISOLTA (re-baseline 2, alternativa D)
Il wireframe prescrive: «il funnel **collassa a riepilogo** [riga compatta ri-espandibile]; il lavoro diventa protagonista». ✅ **Collasso implementato**. ✅ **Nomenclatura unificata (alternativa D)**: la legenda espansa usa etichette gate distinte («In tesi»/«Da approfondire»/«Da verificare»/«Soppresso») con nota che le collega al vocabolario della tabella; la riga di riepilogo a run completato usa il vocabolario routing della tabella sottostante («In tesi N (di cui analizzate M) · Da verificare N · Soppresso K»). I due vocabolari non convivono mai nella stessa vista. **Residuo (→ Fase 4)**: verifica della catena numerica — riconciliazione `routingCounts` vs `progress.surface.fetched`/`rows.length` (target deduplicati/identity-only potrebbero far slittare i totali).

### S9 · Coda di verifica — FEDELE
Motivo derivato verbatim («Identità non confermata sulle pagine lette», «Sito irraggiungibile…») ✓, rimedi Associa dominio / Nessun sito ✓. Non verificati visivamente (nessun caso nei dati): «Conferma sito di gruppo» quando c'è l'hint, «Riprova analisi», stato re-processo per-riga → checklist fase 4.

### S10 · Fuori tesi — presente, vuoto nei dati di prova: parità da verificare con una sessione con scarti reali.

## 4. Pattern trasversali di qualità (Q) — stato a `24bbdf3` (re-baseline 2)

- **Q1 — Ultima attività semantica** · ✅ **RISOLTO** (`a627066`): backend `ma_store.go` calcola `last_activity_event` (CASE WHEN su eventi → «Card creata», «No-go: …», «Buon lead: …»); `IniziativePage` renderizza «Ultima attività: {lastActivityEvent}»; board tabella usa `card.lastEvent` da `ListMALatestCardEvents`.
- **Q2 — Ragioni sociali** · ✅ **RISOLTO** (`9704883`): `.kcardName`/`.companyNameText` con `text-overflow: ellipsis; white-space: nowrap; overflow: hidden; max-width: 100%` + `title={companyName}` tooltip ovunque (kanban, tabella, titoli).
- **Q3 — Date** · ✅ **RISOLTO** (`9704883`): `relativeDate()` («adesso»/«min fa»/«h fa»/«ieri»/«g fa»/data breve) + `shortAuthor()` (email→nome.cognome, default «Sistema») in `helpers.ts`; usati in S1, diario, drawer.
- **Q4 — Diario che racconta** · ✅ **RISOLTO** (`9704883`): `eventLabel()` (`IniziativaBoardPage.tsx:1160`) gestisce stato da→a, chiusura esito+nota, card_creata sessione+stelle+nota, nota inline, contattato/buon_lead/no_go; scheda azienda cita «Ultima nota: “…” · {shortAuthor} · {relativeDate}».
- **Q5 — Conteggi azionabili** · ✅ **RISOLTO (re-baseline 2)**: pill cliccabili ✓ (`onClick`), pill **valorizzate** ✓ (F1 risolto), **evidenziate quando >0** ✓ (`statePillActive`). I 2 residui del re-baseline 2 sono **ritirati (falso problema)**: (a) contatore sul toggle Kanban/Tabella — il totale card è già nell'h1 `({board.cards.length})`, duplicarlo sul toggle è ridondante; (b) click pill → board filtrato per stato — il board default è kanban (il filtro stato vive solo nella vista tabella), quindi pre-impostare `stato` sarebbe incoerente con la vista di apertura; le pill già navigano al board (non sono decorative, soddisfacendo la nota 2 del wireframe).
- **Q6 — Funnel S8** · ✅ **NOMENCLATURA RISOLTA (working tree, 2026-07-04)**: alternativa D implementata — la legenda espansa usa etichette gate distinte (`keep`→«In tesi», `forse`→«Da approfondire», `manualReview`→«Da verificare», `reject`→«Soppresso») con nota di mapping che le collega al vocabolario della tabella; la riga di riepilogo a run completato (collassata) usa il vocabolario routing coerente con la tabella sottostante: «In tesi N (di cui analizzate M) · Da verificare N · Soppresso K». I due vocabolari non convivono mai nella stessa vista. **Residuo**: verifica della catena numerica (riconciliazione `routingCounts` vs `progress.surface.fetched`/`rows.length`) → Fase 4.
- **Q7 — Azioni contestuali in tabella** · ✅ **RISOLTO al re-baseline**: la vista tabella usa `<DossierButton>` con il ciclo a 3 stati. **Fuori perimetro.**

## 5. Difetti funzionali (dalla v1, verificati a codice)

- **F1 — Contatori iniziative** · ✅ **RISOLTO** (`9704883`): l'aggregazione card per stato è in `ListMAInitiatives` (`ma_store.go`: `jsonb_object_agg` per stato → `item.counts`); il frontend renderizza `item.counts[state.key] ?? 0` con highlight `statePillActive` quando >0. Sblocca Q5.
- **F2 — Ciclo di vita ricerche in /ricerche** · ✅ **RISOLTO al re-baseline** (commit `79169e2`, 2026-07-02): `RicerchePage.tsx` ha ora archivia/cestino/purge/ripristino + filtri di visibilità (Attive/Archiviate/Cestino) + modali di conferma. **Fuori perimetro.** (L'audit a `996ce47` lo rilevava ancora aperto: era il gap reale a quel HEAD, chiuso nelle ore successive.)
- **F3 — Ciclo di vita iniziative** · ✅ **RISOLTO** (`9704883`): `IniziativaCard` espone `onArchive`/`onRestore` con bottoni «Archivia»/«Ripristina» per-iniziativa; endpoint backend `POST .../archive` + `.../restore` (`handler.go:121-122`); vista archivio con link «Archivio (N)» + conteggio + empty state. Delete assente (corretto per D-A: archive + restore, no delete/purge).
- **F4 — Dettaglio D2** · ✅ **COMMITTATO** (`24bbdf3`, 2026-07-04): il `TargetDetailModal` (`RicercaDetailPage.tsx`) non ha più il link `/azienda?vat=` — sostituito dai badge B5 inline (`registryFacts` + `inLavorazione`), riusando il pattern già presente nella tabella risultati (`cellBadges` + `registryFactLabels` + `badgeWarn`/`badgeInfo`/`badgeLav`). Il modale resta magro nel ruolo di setaccio (razionale aderenza + contesto scoring + badge B5); il dossier profondo resta azione di card sul MA card-dossier, già autosufficiente e disaccoppiato (`GetMATargetByID` → cache company-keyed via `ListMADeepAnalysis`). Wording PRD §7 aggiornato in committed code (`INIZIATIVE-PRD.md:105`: «Il tool standalone `/azienda` è indipendente da MA e non è accoppiato al flusso (ratifica D-B)»). **Fuori perimetro.**
- **F5 — Marker vs archiviazione** · ✅ **RISOLTO (working tree, 2026-07-04)**: `ListMAActiveCardsByCompany` (`ma_store.go`) ora joina `ma_initiative` con `archived_at IS NULL` → le card di iniziative archiviate non surfacciano più come marker di collisione (board drawer) né come `InLavorazione` (tabella D2). Fix in un solo punto, chiude entrambi i path (board + D2).
- **F6 — Provenienze robuste** · ❌ **RITIRATA (2026-07-02)**: stesso falso problema di D-D. Il rimedio (leggere dallo snapshot invece che dalle sessioni agganciate) servirebbe solo a rendere la card indipendente dalla sessione — ma non è il modello implementato (la card risolve i dettagli attraverso la sessione agganciata). Per le operazioni supportate (archive, purge soft) le provenienze **non** si perdono: la sessione resta con `initiative_id` intatto e le query filtrano solo per quello. Lo sgancio — l'unico caso che rompe — non è supportato. (Se in futuro si volesse supportarlo, servirebbe uno snapshot completo su card: decisione «cambia modello», non un task di remediation.)
- **F7 — Igiene dati di prova**: SQL in §8 (include anche i 2 cambi di stato accidentali fatti oggi durante l'audit da click su riferimenti browser stantii — errore mio, registrato).
- **F8 — Lotto minori**: degli 8 della v1 (non ricostruibili, v1 sostituita) + flash dropdown D1 + ordine bottoni S5, identificati e risolti 3 residui inline (working tree, 2026-07-04): (1) S3 stato «rimossa» → aggiunta `stateTableLabel` + `.statusMuted` (pill «Rimossa» grigia tenue, coerente col wireframe S3); (2) S3 layout filtri → CSS `.filters`/`.searchWrapper` resi compatti inline (gap `--space-2`, search `min-width:200px` `flex:1 1 200px`) allineati al wireframe; (4) flash dropdown D1 → `initiativesLoading` + `disabled` + option «Caricamento…» in `NuovaRicercaPage`. **Residuo**: (3) ordine bottoni S5 (modale chiusura/rimozione: «Annulla» prima di «Chiudi card»/«Rimuovi» — wireframe li vuole invertiti) → Fase 4 (decisione di convenzione UX: allinearsi al wireframe o tenere primary-a-destra).

## 6. Piano di esecuzione proposto (aggiornato al re-baseline 2, HEAD `24bbdf3`)

1. **Fase 0 — Ratifica**: ✅ chiusa (D-A, D-B, D-C ratificate; D-D ritirata).
2. **Fase 1 — Igiene** · ✅ **CHIUSA (2026-07-04)**: riga `.gitignore` `**/.playwright-cli/` ✅; SQL §8 eseguito (iniziativa audit `aac1ef5b…` + scritture accidentali pulite); verifica residui smoke test completata.
3. **Fase 2 — Pattern Q residui** · ✅ **CHIUSA (2026-07-04)**: Q6 nomenclatura implementata (alternativa D: legenda gate con etichette distinte + nota, riga collessata routing); residuo Q6 catena numerica → Fase 4. Q5 residui (contatore toggle, filtro da pill) ritirati come falso problema. Q1, Q2, Q3, Q4, Q5, Q6, Q7 ✅ tutti fatti.
4. **Fase 3 — Funzionale residuo** · ✅ **CHIUSA (2026-07-04)**: F5 implementato (join `ma_initiative` in `ListMAActiveCardsByCompany` per escludere archiviate). F1, F2, F3, F4 ✅ fatti · F6 ritirata · F7 = igiene Fase 1.
5. **Fase 4 — Riverifica di parità formale** · ⏭️ **SALTATA (2026-07-04, decisione utente)**: la riverifica visiva formale non viene eseguita. Gli stati non ancora visti renderizzati (S6 registro, D1 S2–S6, D2 S7 con dati reali, board riscritto da `8284248`, Q6 catena numerica) restano **non verificati visivamente** e si copriranno operativamente all'uso reale. La checklist di parità (screenshot fianco a fianco per ogni stato dei wireframe) resta come riferimento in questo documento ma non è un gate bloccante. **F8(3)** (ordine bottoni S5) resta decisione UX aperta, da prendere se/quando emerge.

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

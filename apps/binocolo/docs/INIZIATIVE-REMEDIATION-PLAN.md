# Binocolo — Remediation plan v2 · Audit di parità contro i wireframe approvati

> 2026-07-02, sostituisce integralmente la v1 (giudicata insufficiente: elencava funzioni mancanti ovvie invece di misurare la distanza dagli artefatti approvati). **Nessuna implementazione parte da questo documento senza ratifica.**
>
> **Metodo**: confronto schermata-per-schermata tra i wireframe approvati (`gated-ux-wireframe.html`, 10 stati; `iniziative-wireframe.html`, 7 stati) e il prodotto a HEAD `996ce47`, screenshot contro screenshot (`artifacts/claude/wf-*.png` vs `live-*.png`, riproducibili). Gli stati non catturabili senza spesa reale (D1 S2–S6, D2 S7 run in corso) sono confrontati su codice e copy. I difetti funzionali della v1 sono assorbiti in §5.
>
> **Re-baseline (2026-07-02, sera)**: l'audit originale è stato condotto a `996ce47`; tre commit successivi nella stessa giornata hanno spostato il codice. Il documento è ora re-baselinato a HEAD `6217051` — vedi §0 per il diff e i claim corretti inline (F2 e Q7 risolti, F3 e Q5 precisati, S2 marcato pre-rewrite). **La tesi centrale (pattern semantici Q) sopravvive intatta.**

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

**Claim confermati intatti a `6217051` (verificati a codice):** F1, F4, F5, F6 (funzionali) · Q1, Q2, Q3, Q4, Q6 (pattern semantici). È la sostanza del remediation e regge.

**Perimetro attivo dopo il re-baseline:**

- Pattern Q: **5 attivi** (Q1, Q2, Q3, Q4, Q6) · Q5 parziale · Q7 fatto.
- Difetti F: **F1, F4, F5, F6** attivi · F3 parziale · F2 fatto · F7 igiene (SQL §8) · F8 da spot-check.

## 1. Verdetto

Il costruito si divide in tre fasce nette:

- **A standard (da preservare)**: D1 stato iniziale, modali di chiusura/rimozione, coda di verifica D2, impianto kanban (colonne flessibili/rail), backend dei flussi card e schema 087–090.
- **Sotto lo standard approvato (il grosso del danno)**: non sono feature mancanti ma **pattern di qualità mai implementati** — il wireframe prescriveva superfici che *raccontano il lavoro* (eventi semantici, conteggi azionabili, funnel che collassa) e il prodotto mostra scheletri (timestamp ISO, "Stato aggiornato" nudo, pannelloni di avanzamento perenni). Sono 7 pattern trasversali (§4), non 30 difetti sparsi: si rimediano per pattern, non per pagina.
- **Mancante**: ciclo di vita iniziative (le ricerche sono ora coperte — F2 risolto al re-baseline), contatori, parità del dettaglio D2 (§5).

**Raccomandazione: si recupera.** Nulla di ciò che è sotto standard richiede rifacimenti: il modello dati e i flussi reggono (smoke end-to-end), i gap sono di resa e di completamento. Butterei solo il `TargetDetailModal` di D2 — non per ricostruirlo, ma per **togliere il link `/azienda`** (accoppiamento col tool standalone) e tenerlo nel ruolo di setaccio (razionale + badge B5); il dossier profondo resta azione di card sul MA card-dossier, già autosufficiente (D-B ratificata).

## 2. Audit per schermata — Iniziative (wireframe S1–S7)

### S1 · Indice iniziative — SOTTO STANDARD
| Wireframe (approvato) | Live | Gap |
|---|---|---|
| Conteggi per stato valorizzati, pill dello stato attivo evidenziata | Tutti 0, pill tutte grigie | Aggregazione mai scritta (`ListMAInitiatives`, ma_store.go:281) → F1 |
| Pill **cliccabili → board filtrato** («non contatori decorativi», nota 2) | Cliccabili ✓ al re-baseline; ma conteggi a 0 (F1) e niente highlight >0 | Q5 (parziale) |
| «Ultima attività: **nota su Nexa Systems S.r.l.**» — semantica: cosa è successo | `2026-07-02T16:55:57.52851+02:00` — ISO grezzo | Q1 + Q3 |
| «aggiornata 2 g fa» (relativa) | «aggiornata 02/07/2026» | Q3 |
| Archiviazione come uscita di scena (nota 3) | Nessuna azione di archiviazione | F3 |
| Empty state e modal Nuova iniziativa | Verbatim ✓ | — |

### S2 · Board kanban — BOARD RISCRITTO POST-AUDIT (descrizione pre-rewrite)
> ⚠️ **Re-baseline**: il board è stato riscritto da `8284248` (2026-07-02: drag-and-drop + state colors, `IniziativaBoardPage.tsx` +529 righe, CSS +467). La descrizione qui sotto è quella dell'audit a `996ce47` e va **riverificata visivamente in Fase 4**. Confermato a codice a `6217051`: **ragioni sociali ancora non troncate** (nessun `line-clamp`/`ellipsis` in `Iniziative.module.css`; `kcardName`/`companyNameText` senza truncation) → Q2 regge; **contatore sul toggle Tabella ancora assente** → Q5 regge. Collasso/rail/contatori colonna/DnD: da riverificare.

_(Audit a `996ce47`):_ Colonne flessibili, rail, contatori colonna, collasso ricordato: ✓ conformi. Gap: **ragioni sociali mai troncate** (la card KRAL è un francobollo verticale — l'esatto difetto che le colonne flessibili dovevano eliminare) → Q2; manca il contatore sul toggle Tabella; badge registro e marker collisione presenti a codice ma non verificabili visivamente (nessun dato attivo) — da coprire nella riverifica di parità (§6 fase 4).

### S3 · Board tabella — SOTTO STANDARD
Filtri e colonne presenti ✓. Gap: «Ultima attività» = **data nuda** contro «card creata · 2 g fa / contattato · ieri / chiusura · 8 g fa» → Q1+Q3; stato «rimossa» in minuscolo, senza pill esito (wireframe: «Chiusa ~Non idonea~» rossa, «Rimandata» ambra); colonna Dossier: ✅ azione a 3 stati presente in tabella al re-baseline (`<DossierButton>`, Q7 risolto); ragioni sociali intere → Q2; layout filtri (cerca full-width su riga separata) più sciatto del compatto approvato.

### S4 · Drawer card — IL GAP PIÙ GRAVE DEL WORKSTREAM
| Wireframe | Live | Gap |
|---|---|---|
| Diario che racconta: «**Contattata** — telefonata col titolare…», «**Card creata** da MSP Lombardia · giu (★★★)» | «Stato aggiornato» nudo, «Card creata» nuda — senza da→a, senza nota inline, senza provenienza/stelle | Q4 |
| Autore breve + data breve (`g.rossi · 30 giu 2026`) | Email intera cruda + data assoluta | Q4+Q3 |
| Sezione PROVENIENZE sempre presente (righe: sessione · mese — ★★☆ score) | Sezione **omessa** quando vuota (e oggi è vuota perché la query legge solo le sessioni agganciate, non lo snapshot) | F6 |
| Scheda azienda: fatti + «ultima nota d'azienda» citata con data | Solo «Nessun fatto registrato.» — la citazione dell'ultima nota non è implementata | Q4 |
| Titolo compatto | Ragione sociale intera troncata a «K…» | Q2 |
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

### S8 · Run completato · Risultati — GAP STRUTTURALE
Il wireframe prescrive: «il funnel **collassa a riepilogo** [riga compatta ri-espandibile]; il lavoro diventa protagonista». Live: il pannello «Avanzamento» con le tre fasi «Completato» + progress bar **resta permanente** sopra la Lavorazione anche a run finito — metà viewport spesa a dire che è finito. In più: la legenda della progress usa una nomenclatura («In tesi / Da approfondire / Da verificare / Fuori tesi») **diversa dai bucket ratificati** mostrati in tabella (azionabile / da verificare / soppresso) — due vocabolari nella stessa schermata → Q6. E la riga funnel mostra «oltre il gate 1 → analizzate 5»: **i numeri non quadrano per costruzione**, contro la nota 1 del wireframe («i numeri quadrano per costruzione») → da spiegare o correggere (Q6).

### S9 · Coda di verifica — FEDELE
Motivo derivato verbatim («Identità non confermata sulle pagine lette», «Sito irraggiungibile…») ✓, rimedi Associa dominio / Nessun sito ✓. Non verificati visivamente (nessun caso nei dati): «Conferma sito di gruppo» quando c'è l'hint, «Riprova analisi», stato re-processo per-riga → checklist fase 4.

### S10 · Fuori tesi — presente, vuoto nei dati di prova: parità da verificare con una sessione con scarti reali.

## 4. Pattern trasversali di qualità (Q) — i veri responsabili della «mediocrità»

- **Q1 — Ultima attività semantica**: il wireframe la usa in S1, S3 e nel diario («cosa è successo · quanto tempo fa»); il prodotto mostra date. Richiede: ultimo evento per iniziativa/card esposto dal backend (il log unico ce l'ha già — è una query) + formattazione evento→etichetta.
- **Q2 — Ragioni sociali**: line-clamp/troncamento con tooltip ovunque (card kanban, righe tabella, titoli drawer/modali). Le ragioni sociali italiane lunghe erano prevedibili e il wireframe usa ovunque nomi compatti.
- **Q3 — Date**: relative («2 g fa», «ieri») nelle superfici di lavoro, mai ISO grezzo, mai email intera come autore (formato `nome.cognome` breve).
- **Q4 — Diario che racconta**: ogni evento renderizza il suo contenuto — `stato`: da→a; `nota`: corpo inline; `card_creata`: sessione di provenienza + stelle (il payload jsonb c'è già, mig 089); `chiusura`: esito + nota. La scheda azienda cita l'ultima nota d'azienda con data.
- **Q5 — Conteggi azionabili** · ⚠️ **PARZIALE al re-baseline**: le pill di stato sono **cliccabili → board filtrato** (fatte). Restano: pill **valorizzate** (bloccate da F1), **evidenziate quando >0**, e **contatore sul toggle Kanban/Tabella**.
- **Q6 — Funnel S8**: collasso a riga-riepilogo a run completato (ri-espandibile); UNA nomenclatura bucket (quella ratificata: azionabile/da verificare/soppresso) in legenda e tabella; numeri del funnel che quadrano o spiegano lo scarto.
- **Q7 — Azioni contestuali in tabella** · ✅ **RISOLTO al re-baseline**: la vista tabella usa `<DossierButton>` con il ciclo a 3 stati (Avvia analisi completa / Analisi in corso… / Apri dossier), riga `IniziativaBoardPage.tsx:534`. **Fuori perimetro.**

## 5. Difetti funzionali (dalla v1, verificati a codice)

- **F1 — Contatori iniziative**: aggregazione card per stato in `ListMAInitiatives` (il passo è caduto tra B1 e B2 del piano). Prerequisito di Q5.
- **F2 — Ciclo di vita ricerche in /ricerche** · ✅ **RISOLTO al re-baseline** (commit `79169e2`, 2026-07-02): `RicerchePage.tsx` ha ora archivia/cestino/purge/ripristino + filtri di visibilità (Attive/Archiviate/Cestino) + modali di conferma. **Fuori perimetro.** (L'audit a `996ce47` lo rilevava ancora aperto: era il gap reale a quel HEAD, chiuso nelle ore successive.)
- **F3 — Ciclo di vita iniziative** · ⚠️ **PARZIALE al re-baseline**: la vista archivio è presente (link «Archivio (N)» + conteggio + empty state in `IniziativePage.tsx`), ma `IniziativaCard` espone solo `onOpen` — **nessuna azione archive/restore per-iniziativa**, e delete resta assente anche a backend (decisione D-A). Resta da fare: aggiungere l'azione di lifecycle (e decidere D-A sul delete).
- **F4 — Dettaglio D2** · ✅ **decisione D-B ratificata (2026-07-02)**: il `TargetDetailModal` (`RicercaDetailPage.tsx:974`) ha un link `/azienda?vat=` che **accoppia il setaccio MA al tool standalone** (`CompanyDossierPage` è indipendente da MA). **Fix: rimuovere il link, non ripararlo.** Il modale resta magro nel ruolo di setaccio — razionale aderenza + contesto scoring + badge B5 inline (`registryFacts`, `inLavorazione`); il dossier profondo è azione di card (B6 → MA card-dossier). Verificato al re-baseline: il MA card-dossier è **già autosufficiente e disaccoppiato** (`GetMATargetByID` popola `target.Deep` dalla cache company-keyed via `ListMADeepAnalysis`) — niente riuso di sezioni, niente refinement da costruire. Lavoro: 1 riga (rimozione link) + badge B5 + fix wording PRD §7.
- **F5 — Marker vs archiviazione**: `ListMAActiveCardsByCompany` (ma_store.go:2373) non esclude le iniziative archiviate.
- **F6 — Provenienze robuste**: il drawer legge solo le sessioni agganciate; deve leggere lo snapshot (`created_from_session` + rating storici) così lo sgancio non cancella la storia (immagine 3 del tuo report).
- **F7 — Igiene dati di prova**: SQL in §8 (include anche i 2 cambi di stato accidentali fatti oggi durante l'audit da click su riferimenti browser stantii — errore mio, registrato).
- **F8 — Lotto minori**: gli 8 della v1 + flash dropdown D1 + ordine bottoni S5.

## 6. Piano di esecuzione proposto (dopo la tua ratifica)

1. **Fase 0 — Ratifica**: decidi sulle aree (recupera/butta) e sulle decisioni D-A…D-D (§7).
2. **Fase 1 — Igiene** (immediata): SQL §8 (tu); riga `.gitignore` per `**/.playwright-cli/` (residui già rimossi).
3. **Fase 2 — Pattern Q1–Q7**: un intervento per pattern, trasversale alle pagine (non pagina-per-pagina: è così che si ricade nei rattoppi). Q1+Q4 condividono il lavoro sul log; Q5 dipende da F1.
4. **Fase 3 — Funzionali F1–F6**.
5. **Fase 4 — Riverifica di parità formale**: per OGNI stato dei due wireframe, screenshot fianco a fianco e checklist puntuale (inclusi gli stati oggi non verificabili: S6 registro renderizzato, D1 S2–S6 e S7 al primo run reale, badge/marker/collisioni con dati veri). **Questa checklist è il gate di accettazione: niente «fatto» senza il confronto visivo.** La eseguo io direttamente.

Sequenza e granularità dei task esecutori le definiamo dopo la ratifica — non prima, per non ripetere l'errore di piani che promettono ciò che non specificano.

## 7. Decisioni richieste

> **Stato (2026-07-02)**: D-A, D-B, D-C ✅ ratificate · D-D ⏳ aperta.

- **D-A** · ✅ **RATIFICATA (2026-07-02)**: **archive + restore, nessuna rotta delete/purge**. Lo schema rende l'hard-delete incoerente (`ma_initiative_card` CASCADE ma `ma_target_outcome.initiative_id` SET NULL → diario orfano); archive è l'uscita disegnata (S1 nota 3), restore l'undo. Pulizia dati di prova = SQL one-shot (§8), non meccanismo di prodotto. Ortogonale a F3 (che cabla l'azione archive/restore in UI).
- **D-B** · ✅ **RATIFICATA (2026-07-02)**: nessuna delle due. Si **rimuove il link `/azienda`** dal modale D2 (`RicercaDetailPage.tsx:974`) — `CompanyDossierPage` è tool standalone, non va accoppiato a MA. Il modale resta magro nel ruolo di setaccio (razionale + badge B5); il dossier profondo resta azione di card sul MA card-dossier, **già autosufficiente** (`GetMATargetByID` → cache company-keyed). Lavoro minimo: vedi F4. Revisiona PRD §7 («dal dossier `/azienda`» → «dal MA card-dossier»).
- **D-C** · ✅ **RATIFICATA (2026-07-02)**: **assorbito dal gate**. Il vecchio bottone manuale (`TargetPage:609` → `web-validation/enrich`) chiamava lo stesso endpoint del gate; il nuovo `/ricerche` non lo re-espone. La web-validation è una preoccupazione **gate-time** (alimenta il bucketing), fatta in automatico dallo stage `enrich` della pipeline + resume-on-failure; la correzione per-target sta nella coda di verifica S9 + `PUT web-validation`. L'endpoint resta (lo usa la pipeline). **Nessuna azione**: non si re-aggiunge il bottone. (Unica perdita: re-enrich bulk di una ricerca già completata — caso di nicchia, v2 se emerge.)
- **D-D** — Sgancio ricerca: basta F6 (provenienze da snapshot) o vuoi anche l'avviso esplicito allo sgancio?

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

# Binocolo — Piano "Fusione card-dossier → Scheda azienda" (T2)

> Esplicita ed esegue il "Tempo 2" ratificato in `SCHEDA-AZIENDA-PLAN.md`
> (decisione 4, brainstorming 2026-07-07; principio inspector 2026-07-08).
> **L'avvio lo decide l'utente**: la direzione è ratificata, l'esecuzione
> parte solo su sua indicazione esplicita. Ogni task è pensato per essere
> eseguito **da solo, in ordine**, da un LLM esecutore. Riferimenti
> **verificati sul codice al 2026-07-08** (HEAD `ee21c06`, scheda azienda
> implementata). Se un simbolo citato non esiste più, fermarsi e segnalarlo.

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **Una sola destinazione per azienda**: `IniziativaCardDossierPage` viene
   assorbita dalla Scheda azienda con lente iniziativa; la rotta
   `iniziative/:id/dossier/:companyKey` reindirizza a
   `aziende/:companyKey?iniziativa=:id`.
2. **Principio inspector (2026-07-08)**: l'inspector è un tool tecnico
   (fruito prevalentemente dall'owner), **mai la casa di un dato per
   l'analista**. Ogni contenuto del card-dossier deve arrivare sulla scheda
   in forma non tecnica. "Resta solo nell'inspector" NON è un esito ammesso.
3. **Politica varianti F6**: le superfici divergono solo per densità e
   selezione, mai per semantica e formato.

## Rettifica rispetto al testo della decisione originale

La decisione 4 del piano scheda diceva "il blocco 4 passa in modalità attiva
(azioni card: stato, chiusura, riapertura, note, IRL — oggi sul
card-dossier)". **Riscontro a codice**: le azioni di ciclo vita della card
(stato/chiusura/rimozione/riapertura/note) vivono sul **board**
(`IniziativaBoardPage.tsx:294-334` e `CardDrawer` `:858`), NON sul
card-dossier. Il card-dossier ha solo: thesis-reading (già condivisa via
`ThesisReadingPanel`), IRL, lancio deep (già per azienda), e i tab di
contenuto. Conseguenza: **il kanban resta la superficie di workflow** — le
azioni di ciclo vita NON migrano; la fusione sposta solo i contenuti + IRL.
Aggiungere le azioni di stato anche alla scheda è un'evoluzione possibile ma
NON fa parte di questo piano (non decisa: non implementare).

## Censimento dei contenuti da assorbire (verificato, scheda vs card-dossier)

Cuore già condiviso e identico su entrambe: `DeepAnalysisContent`
(`components/deep/DeepComponents`, variant `full`) e `ThesisReadingPanel`.
Da assorbire (tab del card-dossier, `IniziativaCardDossierPage.tsx:71-79`):

| Contenuto | Oggi (card-dossier) | Destinazione sulla scheda |
|---|---|---|
| IRL | tab `irl` (`IRLTab`, riga ~283: lista, seed, add/patch/delete, export) | blocco 4, attivo con lente iniziativa |
| Finanziari vendor | tab `financials` (`FinancialsTab`, payload advanced) | blocco 2, espansione collassata «Bilanci (fonte camerale)» |
| Soci completi | tab `shareholders` (`ShareholdersTab`, elenco integrale) | blocco 3, espansione collassata sotto la sintesi esistente |
| Web detail | tab `web` (reconciliation + candidate match per esteso) | blocco 1, espansione collassata del verdetto, copy non tecnico |
| Razionale aderenza strategia | tab `overview` (riga ~492) | blocco 1: già fallback del `businessFit` — verificare copertura, integrare se qualcosa si perde |

## Regole globali per l'esecutore

1. **Database**: MAI connettersi ai DB degli env. Nessuna migrazione in
   questo piano (tutto frontend + eventuali estrazioni componenti).
2. **Test**: NON aggiungere test nuovi. Verifica frontend per ogni task:
   `pnpm --filter mrsmith-binocolo exec tsc --noEmit`; a fine piano anche
   `pnpm --filter mrsmith-binocolo exec vite build` (import rotti).
3. **Smoke**: playwright-cli (cwd=`artifacts/claude`, bypass verificati),
   **sola lettura** sul DB condiviso — niente mutazioni IRL/rating/deep
   reali; verificare presenza/stato dei controlli.
4. **Riuso, non duplicazione**: i contenuti si spostano **estraendo i
   componenti** dal card-dossier in posizione condivisa (pattern di
   `ThesisReadingPanel`), mai copiando JSX. Fino a F5 il card-dossier
   importa gli stessi componenti estratti (zero divergenza durante la
   transizione).
5. **Copy**: italiano B2B asciutto, mai gergo pipeline («Final
   reconciliation», «candidate match analyst» e simili NON compaiono: il
   task F3 include la riscrittura delle etichette in forma analista).
   Skill `tintoretto` per il lavoro UI; leggere `docs/UI-UX.md` prima.
6. **Costi mai in UI utente.**
7. **Non toccare**: inspector; board e `CardDrawer` (salvo il punto
   d'ingresso in F5); endpoint backend (la fusione è frontend: IRL e
   thesis-reading hanno già endpoint per card/sessione funzionanti).
8. A fine task: file toccati + verifiche con esito.

## Ordine di esecuzione e dipendenze

```
F1 (finanziari vendor) ─┐
F2 (soci completi)      ├─ indipendenti tra loro
F3 (web detail)         │
F4 (IRL sulla scheda)  ─┘
F5 (redirect + rimozione card-dossier)  ── ULTIMO, dopo F1-F4 verificate
```

---

## F1 — Finanziari vendor nella scheda

Estrarre `FinancialsTab` da `IniziativaCardDossierPage.tsx` in componente
condiviso (es. `components/company/VendorFinancials.tsx`); il card-dossier
lo importa (nessun cambiamento visivo lì). Nella scheda: blocco 2,
sezione collassata «Bilanci (fonte camerale)» sotto il deep — chiusa di
default quando il deep è presente, **aperta di default quando il deep è
assente** (è l'unica fonte numerica in quel caso). Fonte dati: il target
detail della lente ricerca o l'apparizione più recente (l'overview espone i
targetId); senza alcun target con payload advanced, la sezione non compare.

**Verifica**: `tsc --noEmit`; smoke: scheda di un'azienda con advanced e
senza deep → sezione aperta con i numeri; con deep → collassata.

---

## F2 — Soci completi nella scheda

Estrarre `ShareholdersTab` in componente condiviso (es.
`components/company/ShareholdersDetail.tsx`); card-dossier lo importa.
Nella scheda: blocco 3, espansione collassata «Tutti i soci» sotto la
sintesi esistente (socio di controllo, gruppo, anzianità — che resta
com'è). Coerenza semantica: percentuali e nomi formattati come nel
componente estratto, nessuna riformattazione locale.

**Verifica**: `tsc --noEmit`; smoke su azienda con più soci.

---

## F3 — Web detail in forma analista

Il tab `web` del card-dossier mostra reconciliation e candidate match con
etichette tecniche. Nella scheda: blocco 1, espansione collassata del
verdetto («Dettaglio della verifica») con: dominio selezionato e come è
stato confermato, lettura estesa del match (testi già presenti in
`candidateMatchAnalysis`: `rationale`, `evidenceFor/Against`,
`negativeSignals`), esito finale — **tutte le etichette riscritte in
italiano non tecnico** (questa è la parte di lavoro vera del task: mappare
i campi su copy analista, non trapiantare il tab). Il componente si
estrae comunque in condiviso e il card-dossier lo importa fino a F5 (le
etichette nuove valgono per entrambi: la politica varianti vieta due
semantiche).

**Verifica**: `tsc --noEmit`; smoke su azienda con web validation ricca:
nessun termine di gergo visibile (criterio di accettazione).

---

## F4 — IRL sulla scheda (lente iniziativa)

Estrarre `IRLTab` in componente condiviso (es.
`components/company/IRLPanel.tsx` — attenzione: le mutazioni usano endpoint
per card `initiatives/{id}/cards/{key}/irl/*`, il componente riceve
`initiativeId`+`companyKey` come oggi, riga ~283). Nella scheda: blocco 4,
sezione «IRL» visibile SOLO con lente iniziativa e card esistente
(`initiativeCard` è già risolta, `SchedaAziendaPage.tsx:228-231`), in
modalità attiva (seed, add, edit, export — le stesse del componente).
Senza lente iniziativa: la sezione non compare (l'IRL è della card).

**Verifica**: `tsc --noEmit`; smoke in sola lettura: sezione presente con
lente iniziativa su card con IRL esistente, assente senza lente; nessuna
mutazione reale.

---

## F5 — Redirect e rimozione del card-dossier

**Prerequisito**: F1-F4 verificate; conferma dell'utente che la scheda
copre l'uso reale.

1. Rotta `iniziative/:id/dossier/:companyKey` (`routes.tsx`) → redirect a
   `/aziende/:companyKey?iniziativa=:id` (componente redirect, non semplice
   `Navigate` statico: servono i param).
2. Aggiornare gli entry point che puntavano al card-dossier: bottone
   dossier su card/riga del board, link «Apri dossier ↗» dell'inspector e
   del drawer — puntano alla scheda con lente iniziativa. (Censire con grep
   `dossier/` su `apps/binocolo/src`.)
3. Rimuovere `IniziativaCardDossierPage.tsx` + CSS module; i componenti
   estratti in F1-F4 e `ThesisReadingPanel`/`DeepAnalysisContent` restano
   (sono condivisi). Verificare con grep che nulla importi più la pagina.
4. Verificare che il razionale aderenza strategia (tab overview) sia
   coperto dal blocco 1 (fallback `businessFit`); se manca qualcosa,
   integrarlo PRIMA della rimozione.

**Verifica**: `tsc --noEmit` + `vite build`; smoke: vecchio URL dossier →
atterra sulla scheda con lente giusta; entry point board/inspector →
scheda; nessun 404. Screenshot in `artifacts/claude/`.

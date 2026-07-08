# Binocolo — Piano "Navigazione" (iniziative-first, pulizia tab)

> Esegue le decisioni del brainstorming 2026-07-07 (quinto e ultimo tema).
> Ogni task è pensato per essere eseguito **da solo, in ordine**, da un LLM
> esecutore. Riferimenti verificati sul codice al 2026-07-07 (branch
> `wip/binocolo`, HEAD `8d0e33d`): tab in `apps/binocolo/src/App.tsx:7-14`,
> route in `apps/binocolo/src/routes.tsx`. Se un simbolo citato non esiste
> più, fermarsi e segnalarlo.
>
> **Riscontro 2026-07-08 (HEAD `ffaffda`)** — riferimenti riverificati:
> `App.tsx` navItems righe 7-14 confermate (sei voci); index redirect
> `routes.tsx:16` confermato; rotta `/target` ora a `routes.tsx:24` (non
> :23); `TargetPage.tsx` ancora presente (F3 eseguibile). **Prerequisito di
> F2 soddisfatto**: il selettore iniziativa di `INIZIATIVA-RADICE-PLAN.md`
> F1 è implementato (`NuovaRicercaPage.tsx`, state `initiativeMode`
> auto/manual/existing, riga 62) — F2 può partire. F4 resta gated
> (l'inserimento manuale è a HEAD `ffaffda` ma "in produzione e adottato" lo
> decide l'utente).

## Decisioni ratificate (fonte di verità — non ri-discutere)

1. **Home = Iniziative**: l'index reindirizza su `/iniziative` (oggi
   `/ricerche`, `routes.tsx:16`). L'analista entra sui mandati e scende:
   iniziativa → board → ricerca → scheda azienda.
2. **Ricerche resta tab**, seconda voce, con ruolo dichiaratamente
   trasversale: indice di tutte le ricerche attraverso le iniziative
   (triage quotidiano + lifecycle archivi/cestino).
3. **«Strumenti»**: Ricerca web, Test e Configurazione si raggruppano sotto
   un'unica voce in fondo alla nav — **solo raggruppamento**, nessun
   role-gating (opzione (a); l'eventuale ruolo operatore è evoluzione
   futura, non di questo piano).
4. **Ricerca rapida azienda in nav: FUORI PERIMETRO** — per ora resta
   com'è, non implementare nulla.
5. **TargetPage legacy: si rimuove** (pagina + rotta `/target`). Era il
   "tempo 2" dello strangler del flusso pre-gated.
6. **Dossier azienda esce dalla nav** solo quando l'aggiunta manuale è in
   produzione (`AGGIUNTA-MANUALE-AZIENDE-PLAN.md`): la sequenza è prima la
   sostituzione, poi la rimozione della voce; la rotta `/azienda` può
   sopravvivere silente.
7. **«Nuova ricerca» parte anche dal board dell'iniziativa**, col selettore
   iniziativa precompilato su quella corrente. Dipende dal selettore di
   `INIZIATIVA-RADICE-PLAN.md` F1.

## Repo-fit

- Interamente frontend (nessuna migrazione, nessun endpoint). L'unico
  possibile tocco backend è la rimozione di endpoint orfani in F3, che è
  **condizionata a verifica** (sotto).
- Copy: italiano B2B asciutto; voci nav = sostantivi secchi.

## Regole globali per l'esecutore

1. `pnpm --filter mrsmith-binocolo exec tsc --noEmit` per ogni task; smoke
   playwright-cli (cwd=`artifacts/claude`, bypass verificati, sola lettura).
2. Database: mai; costi in UI: mai.
3. Per il lavoro visivo sulla nav usare `docs/UI-UX.md` (e la skill
   `tintoretto` se il raggruppamento «Strumenti» richiede un componente
   nuovo).
4. Non toccare: pagine interne, funnel, board — questo piano muove SOLO
   nav, route e la pagina legacy.
5. A fine task: file toccati + verifiche con esito.

## Ordine

```
F1 (nav + home + Strumenti)  ── indipendente
F2 (Nuova ricerca dal board) ── dopo INIZIATIVA-RADICE F1
F3 (rimozione TargetPage)    ── indipendente
F4 (rimozione voce Dossier)  ── SOLO dopo AGGIUNTA-MANUALE in produzione
```

---

## F1 — Nav ristrutturata: home Iniziative + gruppo Strumenti

**File**: `apps/binocolo/src/App.tsx` (tab, righe 7-14),
`apps/binocolo/src/routes.tsx` (index redirect, riga 16).

- Index: `<Navigate to="/iniziative" replace />`.
- Ordine voci: **Iniziative · Ricerche · Dossier azienda · Strumenti**
  («Dossier azienda» resta finché F4 non scatta).
- «Strumenti» raggruppa Ricerca web, Configurazione, Test: dropdown o voce
  con sotto-nav secondo il pattern del design system (verificare se la nav
  dell'app/portale ha già un pattern di gruppo; non inventarne uno nuovo se
  esiste). Le route restano invariate (`/ricerca-web`, `/config`, `/test`):
  cambia solo la presentazione in nav.
- Stato attivo corretto: «Strumenti» evidenziata quando si è su una delle
  tre route figlie.

**Verifica**: `tsc --noEmit`; smoke: entrata sull'app → atterra su
Iniziative; le tre voci sotto Strumenti navigano; deep-link diretti alle
route invariati.

---

## F2 — «Nuova ricerca» dal board dell'iniziativa

**File**: `apps/binocolo/src/pages/iniziative/IniziativaBoardPage.tsx`,
`apps/binocolo/src/pages/ricerche/NuovaRicercaPage.tsx`.

**Prerequisito**: selettore iniziativa in testa a NuovaRicercaPage
(`INIZIATIVA-RADICE-PLAN.md` F1). Se non ancora eseguito, fermarsi e
segnalarlo.

- Bottone «Nuova ricerca» nel board (accanto alle sessioni/chip esistenti)
  → naviga a `/ricerche/nuova?iniziativa={id}`.
- `NuovaRicercaPage`: se `?iniziativa=` presente e l'iniziativa è attiva,
  il selettore parte precompilato su «Iniziativa esistente» con quella
  selezionata (l'utente può comunque cambiare). Parametro invalido/iniziativa
  archiviata: ignorare il param, default normale.

**Verifica**: `tsc --noEmit`; smoke: dal board → form con iniziativa
preselezionata; creazione NON necessaria (nessuna scrittura).

---

## F3 — Rimozione TargetPage legacy

**Contesto verificato**: `TargetPage.tsx` è orfana (non in nav), usa il
flusso pre-gated (`POST .../execute`, `GET /binocolo/v1/ma/llm-options`);
rotta `/target` in `routes.tsx:23`.

- Rimuovere `apps/binocolo/src/pages/TargetPage.tsx`, l'import e la rotta.
  Rimuovere eventuali moduli CSS/helper usati SOLO da lei (verificare con
  grep prima di cancellare qualunque file condiviso).
- **Backend: solo verifica, rimozione condizionata.** Censire i consumer di
  `POST .../sessions/{id}/execute` e `GET .../ma/llm-options`: la TestPage
  (`pages/TestPage.tsx`) guida la pipeline raw e potrebbe usarli. Se un
  endpoint resta senza alcun consumer frontend, segnalarlo nel messaggio
  finale come candidato alla rimozione — NON rimuoverlo in questo task
  (decisione separata: potrebbe servire come superficie di test).

**Verifica**: `tsc --noEmit`; `pnpm --filter mrsmith-binocolo exec vite
build` verde (conferma nessun import rotto); smoke: `/target` → not found o
redirect coerente con il router.

---

## F4 — Rimozione voce «Dossier azienda» (GATED)

> **NON eseguire finché l'aggiunta manuale
> (`AGGIUNTA-MANUALE-AZIENDE-PLAN.md`) non è in produzione e adottata** —
> conferma esplicita dell'utente.

- Rimuovere la voce dalla nav; la rotta `/azienda` resta raggiungibile per
  deep-link (pensionamento completo = decisione futura).

**Verifica**: `tsc --noEmit`; smoke nav.

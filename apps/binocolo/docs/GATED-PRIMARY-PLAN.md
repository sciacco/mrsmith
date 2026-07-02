# Binocolo — Gated come flusso primario: piano complessivo

> **Decisione di fondo (Salvatore, 2026-07-02): il gated È la direzione.** Non esiste più una questione di "promozione" o un criterio di confronto A/B con l'execute: il lavoro fatto (funnel Address→gate→Advanced, scoring v3, qualità del gate) è l'investimento nella direzione scelta. Quello che manca è il percorso operativo per renderla il flusso di lavoro quotidiano: chiudere il cerchio sull'identità dei domini, sciogliere due policy, e soprattutto **progettare la UI/UX dedicata** — l'interfaccia attuale non era pensata per un'esecuzione lunga, asincrona e con code di lavoro operatore.
>
> Contesto: `GATED-SEARCH-PIPELINE-PLAN.md` (pipeline, steps 0-4 fatti), memorie `project_binocolo_new_pipeline`, `project_binocolo_domain_scrape_verify` (fix 2026-07-02: full-page, guardia name-match, registro domini mig 082, identity_state mig 083). Migrazioni 073-083 applicate.

## Stato di partenza (2026-07-02)

Fatto e verificato:
- **Funnel gated** end-to-end (job `gated_search`, cap superficie 1000, money-safety per-stadio, rimedio associate-domain durevole).
- **Scoring v3** (bande assolute, routing 4 bucket, cattura esiti, rescore per tesi).
- **Qualità identitaria del gate**: fix footer-strappato (`ScrapeFull` — la P.IVA vive nel footer), guardia P.IVA-estranea anche sul name-match, crawl-before-reject map-first, registro domini cross-sessione (mig 082, manual > auto), fatto `identity_state` persistito (mig 083) con taglio `rejectByIdentity` già esposto dal sector-eval.
- Limite esterno noto: una quota di siti è **infetchabile per fastcrw** ("Could not verify this URL safely", 6/11 nel campione) — nessuna logica nostra li salva; eventuale segnalazione al fornitore.

---

## Workstream A — Asimmetria identitaria, fasi (b) e (c)

**Principio ratificato**: sopprimere richiede certezza identitaria, sopravvivere no. Un reject ha autorità di `scarta` solo da identità `verified` (P.IVA/CF on-page — realistica post-fix: è obbligo di legge, art. 35 DPR 633/72) o `vouched` (operatore/registro manuale). Reject su `assumed` → manual_review (equivalenza epistemica col dominio irrisolto: giudicare il sito sbagliato non dice nulla dell'azienda). Keep/forse passano sempre: il falso keep costa €0.10 e si autocorregge sui finanziari veri (fetchati per P.IVA).

**Fase (b) — misura (passiva, si accumula da sola).** Ogni run gated / web-validation nuova stampa `identity_state`. Protocollo: dopo 1-2 run reali, leggere `GET /ma/sessions/{id}/sector-eval` → `metrics.domain.rejectByIdentity`. Guida di lettura (decisione finale a Salvatore):
- reject `verified`+`vouched` ≳ 90% → accendere la regola piena;
- 70-90% → variante calibrata: degradare solo i reject `assumed` **senza alcun** identificatore in pagina (il caso taylalynn — i siti con P.IVA diversa sono già fermati a monte dalla guardia);
- sotto → ripensare prima di accendere (il travaso in manual_review costerebbe più del beneficio).

**Fase (c) — regola a lettura (deliverable piccolo, dopo la misura).** Mapping in `gatedTargetBucket` (partizione di spesa) e `maRouteTarget` (bucket UI): `reject && identity_state ∈ {assumed}` → manual_review. Righe legacy (`identity_state` NULL) restano al comportamento attuale — nessun travaso retroattivo. Nessuna migrazione, nessuna ri-validazione: il fatto è già persistito.

Interazione da tenere presente: la fase (c) **aumenta** manual_review → il workstream C (senza sito) e la UX della coda (workstream D) ne assorbono l'impatto. Sequenza consigliata: C e D prima o insieme a (c).

## Workstream B — Policy siti di gruppo (decisione di Salvatore, in valutazione)

Caso: filiale italiana la cui presenza web è il sito del gruppo (HORSA ONE PRO → horsa.com, WEBGAINS ITALY → webgains.es). Il sito porta la P.IVA del gruppo (o nessuna P.IVA italiana) → identità mai `verified` per la filiale: `assumed` strutturale, quindi con la fase (c) accesa questi finiscono stabilmente in manual_review.

Dimensioni della valutazione (annotate, non decise):
- **Cosa si giudica**: accettare il sito di gruppo significa classificare la filiale sul business *del gruppo* — spesso informativo (HORSA ONE PRO fa ciò che horsa.com descrive), a volte fuorviante (gruppo diversificato).
- **Cosa NON si contamina**: i finanziari e l'ATECO della filiale arrivano per P.IVA e restano suoi; il rischio è confinato al verdetto settoriale.
- **Perimetro reale della policy**: molte tesi escludono già l'appartenenza a grandi gruppi (filtri soci/holding) — la policy rileva soprattutto per gruppi piccoli/medi non esclusi a monte.
- **Opzione meccanica** se si decide di ammetterli: conferma-gruppo dell'operatore = `vouched` a livello gruppo (variante del rimedio associate-domain, marcata "sito di gruppo"), così il verdetto ha autorità ma resta tracciato che l'evidenza è group-level.

## Workstream C — Verdetto "senza sito"

**Problema**: un'azienda senza sito ufficiale (micro-impresa, newco, lavoro su commessa) oggi non ha uscita da manual_review — il rimedio esistente presuppone un dominio da associare. Con la fase (c) accesa la popolazione trattenuta cresce.

**Forma proposta** (da dettagliare in progettazione):
- Azione operatore sulla riga in manual_review: "Nessun sito ufficiale" → persistita in modo durevole (registro domini con `method='no_website'` e dominio vuoto, o stato dedicato — da decidere in impl.), cross-sessione come le associazioni.
- Effetto: la riga esce dalla coda; il gate semantico si dichiara non applicabile (`no_signal` esplicito, mai `scarta`: assenza di sito ≠ fuori tesi); l'azienda prosegue sui soli dati strutturati, marcata "nessuna evidenza web".
- **Decisione di spesa da prendere**: nel funnel gated, il senza-sito paga Advanced automaticamente (recall-first) o solo su conferma operatore (spend-first)? Proposta: conferma operatore, visto che l'azione è già manuale — un click che dichiara e ammette insieme.

## Workstream D — UI/UX dedicata al gated (il pezzo grande)

L'interfaccia attuale (TargetPage + submission execute) non è stata pensata per ciò che il gated è diventato: **un'operazione lunga, asincrona, multi-stadio, con code di lavoro operatore e rimedi per-azienda**. Serve una progettazione ad hoc, non un adattamento. Due superfici:

**D1 — Pagina di ingresso (nuova ricerca gated):**
- Intake che alimenta il metro di giudizio del gate: oltre al perimetro attuale, **più dettaglio sugli ambiti di attività** cercati (nutre `sectorDescription`/concetti-perimetro → qualità del gate).
- Portata della ricerca espressa come N aziende analizzate; **niente dati di costo in UI utente** (i costi sono valutazione interna esclusiva — vincolo già ratificato).
- Lancio asincrono con aspettative oneste: "richiederà del tempo", stato consultabile.

**D2 — Gestione del run e dei risultati:**
- Avanzamento per stadi (surface → address → gate → enrich+score) con contatori vivi keep/forse/scarta/manual_review.
- **Manual_review come coda di lavoro di prima classe** (non un cassetto): per ogni azienda trattenuta, il motivo (dominio non trovato / infetchabile / identità non confermata / sito di gruppo) e i rimedi contestuali — associa dominio, "nessun sito", (se policy B lo ammette) conferma sito di gruppo. Ogni rimedio è durevole (registro) e ri-processa da solo.
- Risultati: shortlist scorata (bucket routing v3 già esistenti), scarti ispezionabili per audit (recall-safety), esiti/stelle come oggi.
- Vincoli di progetto: leggere `docs/UI-UX.md` prima di progettare; workflow `portal-miniapp-generator` per la review; run lunghi → nessuna dipendenza da endpoint sincroni (già rispettato dal job).

**Percorso**: prima una progettazione UX dedicata (wireframe/IA della pagina e della coda), poi implementazione. È il workstream con più lavoro e va trattato come progetto a sé.

## Workstream E — Minori / igiene

- Blocklist domini → `ma_parameter` o tabella (oggi compilata: ogni aggregatore nuovo = deploy) + euristica di forma per i registri.
- WriteTimeout 60s sull'endpoint di test sincrono (`/test/sector-classification`): i probe lunghi muoiono lato server; il gate async non è affetto. Eventuale esecuzione fuori richiesta o timeout dedicato.
- Segnalazione a fastcrw della classe "Could not verify this URL safely" (6/11 nel campione — è il collo di bottiglia residuo dell'acceptance).

## Sequenza consigliata

1. **(passivo, già attivo)** Fase (b): i run reali accumulano `identity_state` e popolano il registro domini.
2. **B** — valutazione policy gruppi (Salvatore; sblocca il disegno completo della coda manual_review).
3. **D** — progettazione UX (D1+D2), incorporando gli esiti di B e la forma di C.
4. **C** — verdetto senza-sito (piccolo, ma va disegnato dentro la coda di D2).
5. **(c)** — accensione asimmetria, coi numeri della fase (b) in mano e la coda UX pronta ad assorbirla.
6. **E** — igiene, in coda o in parallelo quando comodo.

# Binocolo KB Enrichment — Brief

> **Stato (2026-06-30):** Fase 0 (spina) ✅ · Fase 1 fan-out ✅ (53 concetti, modello B) ·
> Fase 3 grounding ATECO ✅. **Path (a) deployato in sorgente:** `concept_index_source.json`
> rigenerato (53 concetti = 35 target + 18 distrattori, `kind` derivato dai tag) e loadable dal
> modello attuale — **manca solo `atego build` (passo utente, scrive sul DB)**. Aperti: **(3)
> eval set** (validazione, serve labeling) e, separato, il **build della gate-pipeline**.
> Dettaglio in `business-maps/TAXONOMY-SPINE.md`.
>
> **Contesto trainante:** l'arricchimento serve a una re-architettura della pipeline
> di ricerca in cui **UC2 (analisi semantica web) diventa il gate primario** che decide
> quali aziende tenere/forse/scartare *prima* dell'arricchimento Advanced. Vedi
> [§ La svolta](#la-svolta-uc2-diventa-il-gate-primario-della-ricerca). Questo alza
> drasticamente la posta sulla qualità della KB.

## Perché questo lavoro (movente anti-bias)

La calibrazione di UC2 fatta finora è stata **reattiva, single-session, a N piccolo**:
ogni azienda di test che "stonava" produceva una patch (soglie, perimetro, badge…).
È il meccanismo con cui si overfitta. La decisione è di **smettere di incastrare
singoli casi e lavorare sulla KB**: una KB guidata dalla *tassonomia* è difendibile
a prescindere dai campioni della sessione.

**Caso scatenante** — `Automazioni e Sistemi S.r.l.` (automazione industriale / OT:
DCS, PLC, SCADA, ICSS) è stata **confermata in `confirm` deterministico** (verdetto
`confirm · top 0.32 · alta`, nessun LLM) per una strategia M&A su IT-infra/sicurezza.
Due cause strutturali, non di soglia:

1. **`system_integration` ha `embedding_text` agnostico al dominio** — *"Integration of
   software, hardware and platforms"* — quindi l'integrazione OT lo matcha alla
   perfezione: **0.32, più alto del vero positivo IT NETX64 (0.296)**. Le soglie
   assolute non possono separarli; solo un discriminatore può.
2. **Manca un distrattore per l'automazione industriale / OT.** Il segnale OT non
   aveva nessun concetto dove atterrare se non il target IT `system_integration`.

Nudgeare le soglie sarebbe esattamente il bias da evitare. Il fix giusto è di
**copertura/rappresentazione della tassonomia**.

## Cos'è la KB e dove vive

- **Sorgente di verità:** `apps/binocolo/docs/business-maps/concept_index_source.json`
  (mai editare il DB a mano). Oggi **35 concetti**: 29 `target` + 6 `distractor`.
- **Shape per concetto:** `{ id, name, domain, kind (target|distractor), aliases[],
  ateco_candidates_in_kb[], ateco_candidates_excluded[], embedding_text }`.
  `embedding_text` è il testo che viene embeddato (è la "calamita" semantica del concetto).
- **Loader:** `scripts/atego` (`atego build`) legge i JSON in `business-maps/`, embedda
  `embedding_text` via Fireworks (qwen3-emb 4096), UPSERT in `binocolo.kb_concept`
  (+ `kb_concept_ateco`, + `kb_ateco_node` per il retrieval ATECO).
  **Il loader si connette al DB → lo esegue l'utente.** Io produco/edito solo i JSON.
  Migration `059_binocolo_kb_concept_index.sql` è solo DDL; vettori NULL finché il loader non gira.

### I due consumatori della KB
- **UC1 — retrieval ATECO:** concetti → codici ATECO che compongono il perimetro della strategia.
- **UC2 — classificazione settore (web):** descrizione azienda → embed → recall vs concetti
  (target + distractor) → rerank (Qwen3-reranker) → verdetto
  (`confirm`/`reject`/`weak`/`ambiguous`/`no_signal`). `ambiguous`/`no_signal` escalano all'LLM;
  **`confirm` NON ha rete LLM** (per questo un falso confirm è silenzioso).

## La svolta: UC2 diventa il gate primario della ricerca

### Pipeline attuale (UC2 advisory)
1. Utente fornisce perimetro → cattura intent + parametri.
2. ATECO (attività) + province (localizzazione) limitano la superficie; dry-run per sondarla.
3. Su **tutti** i risultati: arricchimento **Advanced (€0.10/risultato)**.
4. Punteggio per ogni azienda → lista ordinata.

**Problema:** il punteggio Advanced è firmografico/finanziario; risponde a "è un buon
target?" ma non a "fa davvero il business che cerco?". La web analysis (UC2) ha mostrato
che la lista ordinata spesso **non matcha il business cercato** → poco significativa.
Due assi ortogonali (rilevanza semantica vs fit finanziario/strategico) sono oggi confusi.

### Pipeline proposta (UC2 gate, funnel invertito)
1. Utente fornisce **obiettivo + perimetro**, con **più dettaglio sugli ambiti di attività** cercati.
2. Cattura intent + parametri (come ora).
3. Mappa ambiti → ATECO (limite necessario per tenere basso N) + altri parametri (come ora).
4. **Invece di Advanced, solo arricchimento Address (€0.01)** → `companyName, vatCode,
   taxCode, address{provincia, town, gps…}`. Niente finanziari, **niente dominio**.
5. **Scoperta dominio** da quei dati → **analisi semantica UC2 su TUTTE le N** (vedi nota costo).
6. Verdetto UC2 → routing: **tenere / forse / scartare**.
7. Solo sui match (e i "forse", se sappiamo classificarli) → Advanced (€0.10) + **punteggio
   (eventualmente rivisto)**.

Una ricerca diventa una **submission asincrona** (richiede tempo); a fine corsa vale più
della lista odierna col punteggio scollegato dal business.

### Fattibilità (già in codice)
- La scoperta dominio è **web-search based** (`resolveDomainCandidates` in `web_search.go`),
  **non** dipende dall'Advanced. `DomainResolutionRequest` consuma esattamente il payload
  Address: `CompanyName, VATCode, TaxCode, Town, Province`. Il **VAT/taxCode** disambigua gli
  omonimi (P.IVA sul sito ⇒ conferma identità; usare il campo `confidence`).
- Caso "nessun sito" già gestito: `domain_unresolved` / `needs_domain_review` → bucket "forse" nativo.
- Async già presente: `binocolo.ma_job` + worker (`ma_web_validation_job`).
- ⇒ è soprattutto **riordino pipeline + promozione del verdetto a routing**, non un build da zero.
- **Nota costo/scope:** il cap del web search su un subset (`maWebValidationDefaultDomainCount`)
  era un **vincolo artificiale**; il design è eseguire UC2 su **tutti** i risultati.
- **Dipendenza (gate-pipeline ⟂ modello B):** la gate-pipeline **NON richiede il modello B**.
  Poggia su tre cose, nessuna delle quali è B: (1) qualità di UC2 → la dà il contenuto arricchito
  (path a); (2) validazione → l'eval set (prerequisito duro prima dell'auto-scarto); (3) il plumbing
  (Address→dominio→UC2-su-tutte-N→routing→Advanced-sui-sopravvissuti→submission async), terzo
  workstream backend indipendente dal modello dei ruoli. Sotto (a) i verticali sono gestiti dalla
  regola fissa `target = in-stack ∧ horizontal`; B serve **solo** a ribaltare i ruoli per classi di
  strategia diverse (acquirente OT/healthtech) → fuori dal percorso critico della pipeline.

### Perché questo alza la posta sulla KB (asimmetria degli errori)
Promuovere UC2 da advisory a gate **inverte il costo degli errori**:

| errore | oggi (advisory) | nuovo (gate) |
|---|---|---|
| falso **confirm** | fastidio visibile | €0.10 sprecato + lista sporca *(sopportabile)* |
| falso **reject** | declassata ma in lista | **scartata in silenzio, mai vista** *(grave — bias nascosto)* |

Conseguenze di design per l'arricchimento:
1. **Recall co-paritario alla precision.** Un concetto *target* mancante o con testo troppo
   stretto = falso reject = target vero buttato. La copertura dev'essere simmetrica
   (target *e* distrattori), non solo "tappare i buchi off-target".
2. **Sotto incertezza: preferire "forse" a "scarta".** Il bucket "forse" è la valvola di
   recall; generoso finché la KB non è provata.
3. **Gate eval (Fase 4) = prerequisito duro:** nessun auto-scarto prima di aver misurato
   keep/maybe/discard su dati etichettati. Lo scarto è distruttivo.
4. **Scarto auditabile:** l'utente deve vedere *cosa* è stato scartato e *perché* — requisito
   di prodotto, non filtro silenzioso (chiude il cerchio anti-bias).
5. **Il punteggio si semplifica:** gestita la rilevanza a monte, il punteggio si concentra
   solo sul fit finanziario/strategico (due assi separati e puliti).

### Routing verdetto → azione (da definire)
- `confirm` → Advanced + score
- `reject` alta-confidenza → scarta *(sempre mostrato/auditabile)*
- `ambiguous` / `no_signal` / `weak` / `domain_unresolved` → **forse** → all'inizio
  Advanced comunque (preserva recall), o un giro più profondo (analyst LLM / tier deep-brief
  €0.30) prima di decidere.

## Principio guida: **discriminazione (con alta recall), non volume**

La qualità della KB per UC2 si misura in **potere discriminante**, non in ricchezza
di testo. Più testo verboso per concetto ⇒ footprint semantici più sovrapposti ⇒
reranker che fatica a separare i concetti. "Sviscerare al massimo ogni ramo" è la
funzione obiettivo **sbagliata**: riproduce su scala il difetto di `system_integration`
(testo generico che matcha troppo largo).

Obiettivo corretto, su tre assi:
- **Copertura dello spazio-categorie** — ogni settore adiacente in cui un'azienda può
  plausibilmente cadere ha un concetto (target *o* distrattore). Questo chiude la classe
  "buco di distrattori" (es. OT). È l'obiettivo *quantitativo*, ma sulla completezza
  delle categorie, non sul volume di testo.
- **`embedding_text` contrastivo, conciso, domain-anchored** — distintivo *rispetto ai
  concetti fratelli*, non scritto in isolamento. Chiude la classe "match troppo generico".
- **Mapping ATECO grounded** — codici verificati su ATECO 2025 reale, non inventati.

## Perché uno storm piatto e indipendente sotto-rende

La discriminazione è una proprietà **tra** concetti, non **di** un concetto. Agenti
indipendenti su `system_integration` e `managed_services`, ciascuno che massimizza il
proprio ramo, scriveranno testi sovrapposti perché in isolamento ognuno reclama il
territorio largo. Il contrasto va progettato **trasversalmente** → serve una spina +
una revisione contrastiva, non un fan-out piatto.

## Orchestrazione proposta (a fasi, non flat)

- **Fase 0 — Spina tassonomica** *(prima, piccola, da rivedere insieme).* Mappa
  rami/sotto-rami dello spazio ICT + settori adiacenti, inclusi i vicini off-target che
  servono da distrattori (OT/automazione industriale, AV/building automation,
  embedded/firmware, EMS/elettronica, e-commerce retailer, CAD/PLM tipo ENGINEERING,
  BPO/call center, staffing/body-rental, …). Senza spina, gli agenti paralleli non hanno
  sistema di coordinate. **È il next step concreto.**
- **Fase 1 — Fan-out, un agente per foglia.** Contratto stretto (modello B, **niente
  `kind`**): `{id, name, layer, vertical, aliases, ateco_in_kb ancorati, embedding_text
  contrastivo·conciso·positivo·domain-anchored, sibling_contrast_notes}`. Ogni agente
  **riceve i concetti fratelli** e scrive in contrasto con loro. Output in artefatto.
- **Fase 2 — Revisione contrastiva (collision detection).** Embed delle bozze, coseno a
  coppie, individuazione dei cluster che collidono, e refine dei confini più caldi. È il
  passo che lo storm piatto salta — dove la discriminazione si guadagna o si perde.
- **Fase 3 — Verifica ancoraggio ATECO.** Ogni `in_kb` esiste in ATECO 2025 e
  l'assegnazione regge (grounding sul riferimento in-repo).
- **Fase 4 — Gate di validazione.** Ri-embed + passaggio su un **set held-out
  multi-strategia** etichettato dalla definizione di perimetro (non dall'intuizione sul
  singolo esito) → misura che la discriminazione non regredisce, su **due metriche
  separate: precision E recall** (keep/maybe/discard). I ~7 campioni della sessione
  diventano *regression guard*, non banco di taratura. **Prerequisito duro** ora che UC2
  è il gate primario: nessun auto-scarto in produzione prima di aver misurato qui.

## Decisioni di design (Fase 0)

1. ✅ **RISOLTA → opzione B: ruoli relativi alla strategia.** Niente `target`/`distractor`
   fisso nel KB. I concetti sono **settori neutri** con tag strutturati `layer` + `vertical`;
   a tempo-strategia una **regola deterministica** li partiziona in target/distractor
   confrontando i tag con lo **scope strutturato della strategia**. **Vincolo fermo: la
   derivazione è su tag, MAI per similarità embedding-vs-testo-strategia** (già provata e
   declassata — `deriveStrategyConceptsByEmbedding`, fix perimetro Gerico). Runtime in due
   tempi: ora profilo di default "IT-infra" derivato dai tag (≡ stato attuale); poi cattura
   intent estesa → derivazione piena. Il fan-out è identico. Assorbe anche la tensione
   orizzontale↔verticale (l'asse `vertical` separa health/finance/OT anche dove l'ATECO no).
   Conseguenza: `ateco_in_kb` ora popolato su **tutti** i concetti; il filtro UC1 diventa
   "usa solo gli ATECO dei concetti in-scope", non più `must_not(kind=distractor)`.
2. ✅ **Coppie contrastive** = principio guida del fan-out: gli omonimi (system integrator,
   security, engineering…) vanno coperti da **entrambi i lati**, scritti in contrasto.
3. ⬜ **Foglia vs alias** (aperta): nuova foglia solo se footprint semantico distinto.
4. ✅ **Fonte ATECO:** riferimento in-repo (`027_anisetta_binocolo_ateco_2025.sql` /
   `kb_ateco_node`), verificato in Fase 3 — non al web.

Dettaglio operativo, mappa per `layer`, lista gap e contratto della foglia in
[`business-maps/TAXONOMY-SPINE.md`](business-maps/TAXONOMY-SPINE.md).

## Guardrail

- **DB safety:** io edito solo i JSON sorgente; il `atego build` (che tocca il DB) lo
  esegue l'utente.
- **Niente taratura per aneddoto:** le modifiche si accettano solo se migliorano la
  metrica aggregata del set held-out senza regredire.
- **Una modifica è "sana" se è giustificabile senza guardare i campioni della sessione**
  (tassonomia/copertura), non se sistema un singolo run.

## Stato attuale dei concetti (35)

- **target (29):** cloud_infrastructure, managed_services, software_development,
  software_product, system_integration, cybersecurity_services, soc_mdr, networking,
  telecom_operator, internet_service_provider, voip_uc, datacenter_colocation, hosting,
  backup_dr, storage, virtualization, devops, data_platform, ai_ml, erp_crm,
  digital_workplace, identity_access, firewall_network_security, ict_wholesale,
  communication_equipment_installation, computer_hardware, network_equipment, fiber_cables,
  messaging_notifications.
- **distractor (6):** web_agency, digital_marketing, it_training, office_equipment,
  generic_consulting, telecom_civil_works.

## Prossimo passo

Produrre la **spina tassonomica** (Fase 0) come primo artefatto: inventario dei 35
concetti + mappa dello spazio-categorie + lista dei gap strutturali. Rivederla insieme,
poi lanciare il fan-out massivo con review contrastiva.

> **Nota:** la re-architettura "UC2 come gate primario"
> ([§ La svolta](#la-svolta-uc2-diventa-il-gate-primario-della-ricerca)) è il movente che
> giustifica l'investimento sulla KB e ne fissa le priorità (recall co-paritario, bucket
> "forse", gate eval come prerequisito, scarto auditabile). Le due attività vanno tenute
> allineate: la KB serve il gate, il gate detta i requisiti della KB.

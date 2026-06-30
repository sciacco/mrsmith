# KB Taxonomic Spine — Fase 0

> **Scopo.** Sistema di coordinate per il fan-out (Fase 1). Inventaria i concetti, mappa lo
> spazio-categorie che il retrieval ATECO cattura, elenca i gap, fissa il **contratto della
> foglia**.
>
> **Decisione presa (#1 + #2):** ruoli **relativi alla strategia (opzione B)**, derivati da
> **tag strutturati** (`layer` / `vertical`), **mai** da similarità embedding-vs-testo-strategia
> (già provata e declassata — vedi nota perimetro Gerico). Vedi `KB-ENRICHMENT-BRIEF.md`.

## Modello dei ruoli — opzione B

- I concetti sono **settori neutri** con metadati strutturati. **Niente `target`/`distractor`
  fisso nel KB.**
- Ogni concetto porta: `layer`, `vertical`, `ateco_in_kb` (su **TUTTI**, anche i vicini
  off-scope, così possono essere target per altre strategie), `aliases`, `embedding_text`.
- A tempo-strategia una **regola deterministica** partiziona i concetti in **target** (dentro
  lo scope dichiarato) vs **distractor** (vicino fuori-scope), confrontando i tag del concetto
  con lo **scope strutturato della strategia** (quali `layer`, quali `vertical`, solo-orizzontale?).
- Runtime in due tempi (il fan-out è **identico** nei due):
  1. **Ora** — profilo di default *"acquirente IT-infra"* derivato dai tag → effetto ≡ stato
     attuale, ma **derivato, non hardcoded**.
  2. **Poi** — cattura intent estesa (la tua idea: "più dettagli sugli ambiti") estrae lo scope
     → derivazione piena per-strategia.

### Gli assi dei tag
- **`layer`** — dove nello stack:
  *in-stack IT* → `infrastructure | services | software | security | telecom | hardware | distribution`
  *off-stack* → `automation_ot | physical_security | marketing | web_digital | education | consulting | office | construction | staffing | bpo | media | retail | …`
  > I "distrattori" di oggi sono semplicemente concetti con `layer` fuori dallo stack IT-servizi.
- **`vertical`** — settore servito: `horizontal` (agnostico) | `health | finance | legal | public_sector | manufacturing | retail | education | media | …`
  > Separa *software-per-la-sanità* da *software-per-l'infrastruttura* **anche quando l'ATECO no**
  > (entrambi 62.10). È così che B gestisce il caso ENGINEERING: non come kludge-distrattore, ma
  > perché il suo `vertical=manufacturing` cade fuori da uno scope orizzontale.

## La realtà del retrieval (invariata)

UC2 vede **solo** chi porta i codici ATECO della strategia (cardine IT-infra: 62.10/62.20/62.90/
63.10 + 58.29/26.x/46.5x/61.x/27.31/33.20). La KB deve discriminare **tutto** ciò che quei codici
contengono — incluse molte aziende non-IT-infra (es. software OT con ATECO 62.x → caso Automazioni).

---

## Mappa per `layer` — attuale (35) + proposti **(P)**

### In-stack IT (in-scope per il profilo di default)

| layer | concetti attuali | proposti (P) |
|---|---|---|
| **infrastructure** | cloud_infrastructure · datacenter_colocation · hosting · storage · virtualization · networking · backup_dr | edge_cdn |
| **services** | managed_services · system_integration · digital_workplace | it_support_helpdesk |
| **software** *(vertical=horizontal)* | software_development · software_product · devops · ai_ml · data_platform · erp_crm | lowcode_platform · iot_platform |
| **security** | cybersecurity_services · soc_mdr · identity_access · firewall_network_security | grc_compliance |
| **telecom** | telecom_operator · internet_service_provider · voip_uc · messaging_notifications | iot_connectivity |
| **hardware** | computer_hardware · network_equipment · fiber_cables · communication_equipment_installation | electronics_manufacturing_ems |
| **distribution** | ict_wholesale | — |

### Off-stack / vertical (off-scope per il profilo di default → "distractor" per derivazione)

| layer / caso | concetti attuali | proposti (P) | omonimia da contrastare |
|---|---|---|---|
| **automation_ot** | — | industrial_automation_ot · building_av_automation | ⚠️ system_integration *(Automazioni)* |
| **physical_security** | — | electronic_security_surveillance | ⚠️ cybersecurity ("security") |
| **software · vertical≠horizontal** | — | cad_plm_engineering_software *(manufacturing)* · vertical_business_software *(health/finance/legal…)* | ⚠️ software_dev/product *(ENGINEERING)* |
| **web_digital** | web_agency | web_portal_media · ecommerce_retailer | ⚠️ hosting / software_product |
| **marketing** | digital_marketing | — | — |
| **education** | it_training | — | — |
| **consulting** | generic_consulting | — | ⚠️ IT consulting |
| **office** | office_equipment | — | — |
| **construction** | telecom_civil_works | — | — |
| **staffing / bpo / altri** | — | it_staffing_bodyrental · bpo_callcenter · computer_repair · data_entry_digitization · gis_surveying | ⚠️ managed_services / data_platform |

---

## Il principio della coppia contrastiva (guida del fan-out)

Sotto B è più pulito: le coppie omonime **condividono una parola ma differiscono su un tag**.
Servono comunque **entrambi i concetti**, scritti in contrasto, perché il reranker li separi;
i tag poi li instradano.
- *"system integrator"* → system_integration `(services, horizontal)` vs industrial_automation_ot `(automation_ot)` / building_av_automation
- *"security"* → cybersecurity `(security)` vs electronic_security_surveillance `(physical_security)`
- *"engineering/software"* → software_development `(software, horizontal)` vs cad_plm_engineering_software `(software, manufacturing)`

## Stato delle decisioni di design
1. ✅ **RISOLTA → B** — ruoli strategy-relative, tag strutturati, **mai** embedding-similarity.
2. ✅ **ASSORBITA da B** — l'asse `vertical` gestisce orizzontale↔verticale.
3. ✅ **Coppie contrastive** — principio guida del fan-out (confermato).
4. ✅ **RISOLTA — "data-driven, default keep-leaf".** Niente pre-potatura manuale a sentimento.
   Si tengono **tutte** le (P) come foglie; le **fusioni** avvengono in **Fase 2** su evidenza
   (coseno alto tra fratelli) + giudizio del review, non per intuito. Motivo: costi asimmetrici —
   un *under-split* (collasso due settori) fa bleedare un off-stack dentro un target = falso
   verdetto **silenzioso**; un *over-split* è innocuo e **intercettato dalla Fase 2**. Es.
   `computer_repair` (95.11, break-fix) resta foglia: collassarlo in `managed_services` creerebbe
   un falso confirm.
5. ⬜ **ATECO grounding** — ancoraggio a `027_anisetta_binocolo_ateco_2025.sql` / `kb_ateco_node`,
   verificato in Fase 3.

## Contratto della foglia (Fase 1)
Ogni agente, **sibling-aware**, produce per la foglia:
```
{ id, name, layer, vertical,
  aliases[],
  ateco_in_kb[]            // grounded; popolato su TUTTI i concetti
  embedding_text,          // contrastivo · conciso · POSITIVO · domain-anchored
  sibling_contrast_notes } // come si distingue dai fratelli/omonimi
```
**Niente etichetta T/D** — il ruolo si deriva a valle dai tag.

## Cosa NON cambia / cosa cambia (vincoli)
- **Invariato:** `embedding_text` embeddato **grezzo** (qwen3-embedding-8b), nessuna istruzione
  lato doc. Testo **POSITIVO** nel proprio dominio, **niente negazioni** (vale per *tutti* i
  concetti, non solo gli ex-distrattori — gli embedding gestiscono male la negazione).
- **Cambia (per B):** `ateco_in_kb` ora **popolato su tutti** i concetti (anche off-IT), perché
  un concetto può essere target per un'altra strategia. Il filtro UC1 **non** è più
  `must_not(kind=distractor)` ma **"usa solo gli ATECO dei concetti in-scope per la strategia"**
  (per il profilo di default IT-infra, effetto identico a oggi).

## Stato avanzamento
- ✅ **Fase 0** — decisioni #1–#5 risolte.
- ✅ **Fase 1 (fan-out)** — 54 concetti authored/refined (workflow `binocolo-kb-fanout`, 75 agenti) +
  review contrastiva per layer. Output: `concept_index_candidate.json` (10 merge_flags, 71 overlap_warnings).
- ✅ **Fase 3 (grounding ATECO)** — ogni `ateco_in_kb`/`ateco_excluded` validato vs
  `docs/codici_ateco_2025.json`; corretti codici NACE/obsoleti (70.22→70.20, 95.11→95.10, 80.20→80.09…)
  e `bpo_callcenter` (era vuoto). Nessun codice inesistente residuo. Candidato `ateco_grounded`.

## Prossimo passo (aperto)
1. **Merge decisions** (10 flag): unico vero candidato a fusione `telecom_operator`+`internet_service_provider`; gli altri "distinti ma vicini".
2. **Deploy path**: (a) estratto compatibile col modello attuale (testi affilati + off-stack come distractor) → fix Automazioni/ENGINEERING subito; oppure (b) build del modello B pieno (schema/loader/UC1/UC2).
3. **Fase 2 (collision detection meccanica)**: coseno a coppie dopo l'embed via `atego` (passo utente) → conferma/ribalta i merge_flag su evidenza.

**Conteggio:** **54 foglie** (35 rielaborate + 19 nuove), ATECO validati.

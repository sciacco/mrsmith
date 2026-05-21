# Training Management — Handoff per sviluppo in MrSmith

> Mini-app per la gestione della formazione interna CDLAN, da integrare nel portale MrSmith.
> Questo documento è il punto di ingresso per chi (incluso Claude Code) deve proseguire lo sviluppo a partire dagli artefatti già prodotti.

---

## 1. Convenzioni MrSmith (vincolante)

Questa mini-app **segue le convenzioni già adottate per le altre app di MrSmith**: stack, struttura cartelle, naming, autenticazione/SSO, layout, design system, pattern di integrazione, build, deploy, telemetria, error handling. Non c'è alcuna ragione di deviare.

In caso di ambiguità tra questo documento e una convenzione MrSmith esistente, **vince la convenzione MrSmith**. Fa eccezione solo lo schema di dominio (`schema-v2.sql`) e le state machine, che sono il deliverable di questa fase e non vanno reinventati.

---

## 2. Obiettivi

Sostituire la gestione attuale della formazione, oggi frammentata tra:

- Factorial (sezioni "documenti" e "le mie formazioni"), in dismissione a breve;
- `PROPOSTA_FORMAZIONE_2026.xlsx` (foglio multi-utente, multi-sheet, dati incoerenti);
- conoscenza implicita (CV, dichiarazioni verbali).

Obiettivi non negoziabili:

1. **Single source of truth** per dipendente, corsi, iscrizioni, certificazioni, attestati;
2. **Storicizzazione corretta**: nessuna sovrascrittura distruttiva;
3. **Eliminazione duplicati**: il dato vive in un posto solo;
4. **Disaccoppiamento HR**: Training usa l'anagrafica locale `employee`; connettori esterni, fuori scope mini-app, la popolano e sincronizzano.

---

## 3. Decisioni di scope già chiuse

Queste decisioni sono state validate e sono **input** dello sviluppo, non più discusse.

| # | Decisione | Stato |
|---|---|---|
| Q1 | Perimetro utenti = solo dipendenti diretti (no esterni/consulenti) | chiusa |
| Q2 | Nessun workflow di approvazione formale, nessuna soglia di costo, nessuna ratifica CFO | chiusa |
| Q3 | Formazione obbligatoria gestita dal tool; owner = `people_admin` (ruolo unico) | chiusa |
| Q4 | Percorsi pluriennali (`learning_path*`) modellati ma non bloccanti al go-live | chiusa |
| Q5 | Integrazione calendario Outlook | **TBD post go-live** |
| Q6 | Dipendente carica attestati e propone corsi (tabella `training_request` dedicata, separata da `enrollment`) | chiusa |
| Q7 | Strategia di migrazione storica | **TBD, dipende dalla data di go-live** |

### Q5 e Q7: cosa serve per chiuderle

- **Q5** (calendario): nessun impatto sullo schema. Se/quando si deciderà di farla, sarà un endpoint applicativo che espone un feed iCal read-only sottoscrivibile dal calendario personale del dipendente. Non costruire integrazioni Microsoft Graph senza esplicita richiesta.
- **Q7** (migrazione): dipende dalla data di go-live decisa dal business. Lo schema regge entrambi gli scenari (import 1:1 vs anno zero pulito). Quando si deciderà:
  - se go-live entro Q3 2026 → import 1:1 del piano 2026 in corso + storico certificazioni;
  - se go-live Q4 2026 o successivo → solo storico certificazioni, piano 2027 nato pulito nel tool.
  Lo script di migrazione sarà un job Python parametrizzato che legge `PROPOSTA_FORMAZIONE_2026.xlsx` e popola le tabelle del dominio.

---

## 4. Stato attuale degli artefatti

Tutti gli artefatti sono autoritativi e già allineati tra loro. Sono il punto di partenza per il codice.

| Artefatto | Path | Tipo | Stato |
|---|---|---|---|
| Schema DB | `schema-v2.sql` | DDL PostgreSQL 16+ | validato sintatticamente con libpg_query (64 statement) |
| State machine documentate | `state-machines.md` | Markdown con diagrammi Mermaid | validato manualmente |
| State machine runtime | `backend/internal/training/state_machine.go` | Go | fonte runtime |
| Trigger guard DB | `enrollment_state_trigger.sql` | PL/pgSQL | validato sintatticamente |
| File sorgente legacy | `PROPOSTA_FORMAZIONE_2026.xlsx` | Excel | input per migrazione |

### Allineamento verificato

- Le transizioni applicative sono implementate nel backend Go; `state-machines.md` resta riferimento di dominio.
- Il trigger DB copre la matrice meccanica delle transizioni di `enrollment`; le precondizioni "ricche" (attore, reason, stato del piano) vivono nell'application layer.

---

## 5. Modello di dominio

Riferimento autoritativo: `schema-v2.sql`. Qui un riepilogo per orientarsi.

### Entità principali

- `employee` — anagrafica locale letta da Training e alimentata da connettori esterni fuori scope.
- `team` + `team_membership` — appartenenza storicizzata via `tstzrange` con exclusion constraint anti-overlap.
- `vendor` — fornitori di formazione (dedup via `name_normalized citext`).
- `skill_area` — aree formative gerarchiche (self-referencing `parent_id`).
- `course` — catalogo corsi (ripetibile); contiene `is_mandatory`, `recurrence_interval`, `compliance_framework` per la formazione obbligatoria.
- `certification` — catalogo certificazioni (separato da `course`).
- `training_plan` — piano annuale (unità di budget e reporting).
- `enrollment` — istanza di corso per un dipendente in un piano (entità "calda").
- `certification_award` — conseguimento certificazione (può non essere legato a un'enrollment, per il legacy).
- `skill_assessment` — self-assessment AS IS storicizzato.
- `document` — metadati attestati (file su object storage), FK polimorfica via XOR check (`enrollment_id` xor `certification_award_id`).
- `training_request` — wishlist/suggerimenti dei dipendenti, separati dalle iscrizioni ufficiali.
- `mandatory_assignment_rule` — regole "corso X obbligatorio per team Y" usate dal job di compliance.
- `learning_path*` — percorsi pluriennali opzionali (Q4).
- `audit_log` — append-only, mai UPDATE/DELETE.

### Decisioni di design dello schema (da rispettare)

1. **UUID v7-style come PK ovunque** (`gen_random_uuid()` per ora; passare a UUIDv7 se MrSmith ha già una utility apposita). No `bigserial`.
2. **Stati come enum Postgres** per i lifecycle stabili (`enrollment_status`, `award_outcome`, `plan_status`). **Tassonomie aperte come tabelle** (team, skill_area, vendor, certification, course).
3. **Soft-delete via `is_active`** sui master data; **nessuna cancellazione fisica** sui dati storicizzati (iscrizioni chiuse, award).
4. **Snapshot "as-of"** su `enrollment` (`course_title_snapshot`, `vendor_name_snapshot`): popolati lato applicativo al passaggio in stato terminale, per disaccoppiare lo storico dall'evoluzione del catalogo.
5. **`validity daterange` derivato** su `certification_award` (`GENERATED ALWAYS AS ... STORED`) con indice GIST: lo "scaduto" è proiezione, non stato persistito.
6. **Polimorfismo `document`**: FK opzionali con XOR check, non `entity_type`/`entity_id` generici. Postgres-friendly.
7. **`citext`** su email e `vendor.name_normalized` per dedup case-insensitive.
8. **Trigger `set_updated_at`** già applicato a tutte le tabelle con campo `updated_at`.

### Viste già pronte

- `v_employee_certifications` — sostituisce il foglio "Certificazioni" dell'Excel.
- `v_plan_budget` — consuntivo per piano e team.
- `v_expiring_certifications` — scadenze prossime.
- `v_mandatory_compliance_gap` — chi deve fare cosa per compliance.

Sono già SQL standard. Non riscriverle in ORM se la query è complessa: chiamare la view direttamente è più chiaro e performante.

---

## 6. State machine

Riferimento runtime: `backend/internal/training/state_machine.go`. `state-machines.md` documenta il dominio.

Tre macchine:

- **`enrollment`** — 7 stati, 11 transizioni. La più complessa. Trigger DB di rete di sicurezza già fornito.
- **`certification_award`** — solo conseguimenti positivi (`passed_exam`, `attendance_only`) + "expired" calcolato. Le correzioni People sono normali update amministrativi con audit leggero.
- **`training_request`** — coda di triage; `accepted` è intermedio (in attesa che il piano dell'anno target sia apribile), `converted` chiude collegando alla `enrollment` nata dalla richiesta.

### Regole vincolanti per il codice

1. **Mai bypassare la macchina** con UPDATE diretti su `enrollment.status`. Sempre passare dal service layer Go.
2. **Bypass legittimo solo per migrazione**: `SET LOCAL training.allow_status_override = 'true'` dentro la transazione del job di import. Non usare in nessun altro contesto.
3. **`reopen` è la transizione pericolosa**: UI con conferma esplicita, audit entry dedicata (`audit_log.action = 'enrollment.reopened'`, `before_state` JSON completo).
4. **Le precondizioni applicative** (attore, reason, stato piano, ownership) vivono nel backend Go. Il trigger DB blocca solo transizioni meccanicamente assurde.

---

## 7. Effetti collaterali deterministici

Quando una transizione produce side-effect, sono dichiarati in tabella nel `state-machines.md`. Riepilogo dei più rilevanti che il codice deve implementare esplicitamente (non delegare a trigger DB nascosti):

- `enrollment.start` → se `actual_start` è NULL, impostarlo a `CURRENT_DATE`.
- `enrollment.complete` / `fail` / `cancel` / `expire` → popolare `course_title_snapshot` e `vendor_name_snapshot` con i valori correnti dal catalogo.
- `enrollment.complete` su corso con `leads_to_cert_id` valorizzato → non creare automaticamente un `certification_award`; l'utente o People registra esplicitamente la certificazione/frequenza quando serve.
- `certification_award` accetta solo `passed_exam` e `attendance_only`; l'esame non superato resta su `enrollment.status = failed`.
- `training_request.convert` → creare la `enrollment` collegata, popolare `converted_to_enrollment_id`. La transizione fallisce se non esiste un `training_plan(year = desired_year)` in stato `draft` o `open`.

---

## 8. Job applicativi

Tre job ricorrenti da implementare (schedulazione tipica: notturna).

### Job 1 — Expire enrollments di piani chiusi

**Quando**: subito dopo che un `training_plan` transita a stato `closed`, oppure giornalmente come reconciliation.

**Cosa fa**: per ogni `enrollment` in stato `proposed` o `approved` appartenente a un piano `closed` senza `actual_start`, applica la transizione `expire` (attore `system`). Popola gli snapshot.

**Query base**:
```sql
SELECT e.id FROM training.enrollment e
JOIN training.training_plan tp ON tp.id = e.training_plan_id
WHERE tp.status = 'closed'
  AND e.status IN ('proposed', 'approved')
  AND e.actual_start IS NULL;
```

### Job 2 — Compliance gap per formazione obbligatoria

**Quando**: settimanale o al cambio di una `mandatory_assignment_rule`.

**Cosa fa**: legge `v_mandatory_compliance_gap`, per ogni riga con `compliance_status = 'missing_or_expired'` genera (idempotente) un'`enrollment` in stato `proposed` sul piano dell'anno corrente, se non già presente.

**Idempotenza**: prima di creare, controllare l'assenza di una `enrollment` esistente per `(employee_id, course_id, training_plan_id)` in stato non terminale.

### Job 3 — Notifica scadenze certificazioni

**Quando**: giornaliero.

**Cosa fa**: legge `v_expiring_certifications`, manda notifica al dipendente e a `people_admin` per ogni cert in scadenza a 90 / 30 / 7 giorni.

**Stato delle notifiche**: usare il sistema di notifiche già presente in MrSmith. Idempotenza tramite tabella `notification_sent` di MrSmith (o equivalente), non re-inventare.

---

## 9. Identità, autenticazione, autorizzazione

**Identità**: SSO Microsoft 365, come tutte le app MrSmith. Il claim utilizzato per il matching con `employee` è quello già in uso negli altri moduli del portale.

**Mapping utente ↔ `employee`**: via `employee.email` (citext). Se al primo login l'utente SSO non è in `employee`, il flusso è una pagina "in attesa di onboarding HR" — **mai** auto-creare il record `employee` dal flusso applicativo. La popolazione e sincronizzazione dell'anagrafica locale è demandata a connettori esterni fuori scope Training.

**Ruoli applicativi**: quattro:

- `employee` — tutti i dipendenti attivi (default).
- `manager` — modellato ma non assegnato a nessuno al go-live. Lasciare l'hook nel codice.
- `people_admin` — owner unico della gestione. Lista hardcoded inizialmente (`[VERIFICA con People per la lista]`), gestione via tabella `system_role` o equivalente MrSmith dopo il go-live.
- `system` — usato solo dai job. Identità tecnica, non utente.

**Ownership su `enrollment` e `training_request`**: il dipendente può vedere/modificare solo i propri record. Il manager (se mai attivato) i propri riporti via `employee.manager_id`. `people_admin` tutto. Implementare via row-level filtering applicativo (non RLS Postgres — meno portabile, meno test-friendly).

---

## 10. Storage attestati

Gli attestati PDF vivono su object storage. Il record `document` tiene solo i metadati (`storage_key`, `sha256`, `mime`, `size_bytes`).

**Backend di storage**: usare quello già configurato per le altre mini-app MrSmith. `[VERIFICA: probabilmente l'OceanStor Pacific via S3-compatible API, ma confermare con la convenzione MrSmith.]`

**Pattern di upload**: presigned URL lato client se MrSmith già lo fa, altrimenti proxy via backend. **Calcolare `sha256` lato server** post-upload, non fidarsi del client.

**Naming chiave**: `training/<yyyy>/<employee_uuid>/<document_uuid>.<ext>`. Niente nomi originali nelle key (privacy + collisioni).

**Validazione documenti**: l'utente carica con `is_validated = false`. `people_admin` promuove a `is_validated = true` valorizzando `validated_by` e `validated_at`. La validazione è il prerequisito per considerare un `certification_award` con `validation_source = 'document_verified'`.

---

## 11. UI / pagine principali

Le UI seguono il design system MrSmith. Pagine da implementare (priorità decrescente):

### Per `employee`

1. **Dashboard personale** — le mie iscrizioni (passate/in corso/pianificate), le mie certificazioni con stato visibile (valida/in scadenza/scaduta), gli attestati caricati.
2. **Carica attestato / dichiara certificazione** — form con upload, scelta certificazione dal catalogo, date, eventuale link al credential esterno.
3. **Catalogo corsi** — sfogliabile, filtrabile per area; pulsante "richiedi questo corso" che apre il form di `training_request`.
4. **Richiedi formazione (libero)** — form di `training_request` con `free_text_title` se il corso non è a catalogo.
5. **Self-assessment** — periodico, una riga per area skill rilevante, slider 0–5.

### Per `people_admin`

1. **Coda richieste** — `training_request` in `submitted` / `under_review` / `accepted`, con triage rapido (start_review / accept / reject / convert).
2. **Pianificazione annuale** — vista per piano, drag of corsi/iscrizioni, approvazione massiva, chiusura piano (innesca job expire).
3. **Anagrafica catalogo** — CRUD su `team`, `vendor`, `skill_area`, `course`, `certification`.
4. **Regole formazione obbligatoria** — CRUD su `mandatory_assignment_rule`.
5. **Audit / scadenze** — viste `v_employee_certifications`, `v_expiring_certifications`, `v_mandatory_compliance_gap` esposte come pagine filtrabili.
6. **Report** — export Excel/CSV/PDF di qualunque vista (continuità operativa rispetto all'Excel attuale, è un must).

### Note di design

- **Export Excel è prima-class**, non ripiego. È il formato in cui People oggi vive.
- **Filtri persistenti via querystring**, per condividere link a viste filtrate.
- **No bulk actions distruttive** senza conferma esplicita (in particolare `reopen`).
- **Skeleton/loading state** consistenti col resto del portale.

---

## 12. Anagrafica persone

Training usa `training.employee` come anagrafica locale. La mini-app non implementa integrazioni HR, credenziali Factorial, webhook o job di sincronizzazione.

**Responsabilità esterna**: connettori/sync fuori scope popolano e aggiornano `training.employee`. Training legge questa tabella per ownership, viste People, import Excel e report.

**Regole applicative**: login e import Excel non creano dipendenti. Se una persona manca o è ambigua, la UI/report segnala il problema e attende la correzione dell'anagrafica esterna.

**Identità**: `email` resta UNIQUE e viene usata per il matching SSO. `external_id` è opzionale e riservato ai connettori esterni.

---

## 13. Migrazione storica (Q7 — TBD)

Lo script di migrazione **non è ancora da scrivere**: dipende dalla data di go-live (vedi sezione 3).

Quando si potrà scrivere, sarà un job Python parametrizzato che:

1. legge `PROPOSTA_FORMAZIONE_2026.xlsx`;
2. normalizza i nomi dei team (trim, case);
3. dedupica i fornitori via `name_normalized`;
4. crea le `certification` mancanti nel catalogo a partire da quelle elencate nel foglio "Certificazioni";
5. crea i `certification_award` con `validation_source = 'imported_legacy'` per lo storico — esplicitamente marcati come "da verificare" finché l'utente non carica l'attestato;
6. (solo se import 1:1) crea le `enrollment` del piano in corso con bypass `SET LOCAL training.allow_status_override = 'true'` per impostare stati arbitrari coerenti con la storia.

Casi noti da gestire nei dati legacy (osservati nel foglio):

- date come stringhe libere (es. `"06/05/20206"`, `"contattare fornitore per avere info"`) → log + skip + report manuale;
- multi-valore in una cella (5 certificazioni in un bullet list per Angelucci) → split in righe distinte;
- spazi finali nei nomi (`"Team "`, `"Zenoni "`, `"Nardin "`) → trim sistematico;
- macro-aree come colonne (`O365 | FORTINET | LINUX | CCNA | CCNP`) → pivot inverso in `skill_area`;
- stato implicito nel testo (`"BOCCIATO ESAME"`, `"IN CORSO"`, `"NON HA CERTIFICAZIONI"`) → mapping esplicito su `enrollment.status` o warning manuale; non creare `certification_award` negativi/in corso.

---

## 14. Cosa Claude Code può decidere da solo

- Naming dei package/moduli interni, secondo le convenzioni MrSmith.
- Layout delle pagine UI nei dettagli, finché rispetta il design system.
- Suddivisione di endpoint REST in più rotte se utile per chiarezza.
- Scelta di librerie di supporto già usate altrove in MrSmith.
- Strategie di caching applicativo per le viste read-only (`v_*`).
- Aggiunta di indici Postgres se profilando emergono query lente.

## 15. Cosa Claude Code deve chiedere all'autore prima di procedere

- **Q5** (calendario) e **Q7** (migrazione): non implementare nulla senza decisione esplicita.
- Modifiche allo **schema di dominio** (`schema-v2.sql`): qualunque tabella/colonna nuova oltre a quanto già definito richiede validazione, perché va riconciliata con i requisiti business.
- Modifiche alle **state machine**: qualunque transizione aggiunta/rimossa richiede aggiornamento di `state-machines.md`, backend Go e, per `enrollment`, del trigger DB se cambia la matrice meccanica. Da non fare senza conferma.
- **Promozione di un ruolo applicativo** (es. abilitare davvero `manager`): cambia la matrice di autorizzazioni in modo significativo.
- **Bypass del state machine guard** in contesti diversi dalla migrazione storica: vietato senza conferma esplicita.
- **Cancellazione fisica** di qualunque record che abbia un `audit_log` collegato: vietato.
- **Integrazioni HR**: restano fuori scope Training e vanno gestite da connettori esterni.

---

## 16. Compliance e GDPR

- **Dati personali**: nome, cognome, email, formazione svolta. Niente dati sensibili (salute, opinioni, ecc.).
- **Retention**: dipendenti cessati restano per durata rapporto + N anni (da concordare con DPO `[VERIFICA]`). Mai cancellazione fisica; pseudonimizzazione opzionale dopo il termine di retention.
- **Audit log**: append-only, mai modificato. Conservato per durata pari ai dati di dominio.
- **Diritto di accesso**: l'utente vede già tutti i propri dati nella propria dashboard.
- **Diritto di cancellazione**: per dipendenti attivi non applicabile (interesse legittimo del datore di lavoro a tracciare la formazione). Per cessati, applicare retention policy.

---

## 17. Definition of done — go-live MVP

Il prodotto si considera in go-live quando:

- [ ] Schema deployato su ambiente di produzione MrSmith.
- [ ] Anagrafica `training.employee` popolata dai connettori esterni.
- [ ] Auth SSO M365 integrato.
- [ ] CRUD completi per: team, vendor, skill_area, course, certification, training_plan.
- [ ] Lifecycle completo di `enrollment` (tutte le transizioni, audit log popolato).
- [ ] Lifecycle completo di `certification_award` e `training_request`.
- [ ] Upload attestati funzionante (object storage + validazione).
- [ ] Pagine UI per `employee` e `people_admin` (sezione 11).
- [ ] Job 1 (expire), 2 (compliance gap), 3 (scadenze) schedulati.
- [ ] Export Excel/CSV/PDF delle 3 viste principali (`v_employee_certifications`, `v_plan_budget`, `v_expiring_certifications`).
- [ ] Test di integrazione app ↔ trigger DB su tutte le transizioni di stato.
- [ ] Migrazione storica eseguita (modalità dipendente da Q7).

Cosa **non** è MVP:

- Learning paths (Q4) — modellati ma UI rimandata.
- Integrazione calendario (Q5).
- Notifiche push/mobile (oltre alle email).
- Ruolo `manager` attivo.

---

## 18. Glossario

- **Enrollment**: iscrizione di un dipendente a un'edizione di corso in un piano.
- **Award**: conseguimento di una certificazione (può non essere legato a un'enrollment).
- **Request**: suggerimento/wishlist di formazione da parte di un dipendente, in attesa di triage.
- **Piano (`training_plan`)**: contenitore annuale delle iscrizioni; unità di budget e reporting.
- **Validation source**: provenienza del dato (documento verificato, dichiarazione in survey, dichiarazione verbale, CV, import legacy).
- **Compliance gap**: differenza tra ciò che `mandatory_assignment_rule` richiede e ciò che il dipendente ha effettivamente come certificazione valida.
- **People**: il team People & Culture di CDLAN; nel codice corrisponde al ruolo applicativo `people_admin`.

---

## 19. Cronologia handoff

| Versione | Data | Modifiche |
|---|---|---|
| 1.0 | 2026-05-19 | Versione iniziale per kick-off sviluppo in MrSmith |

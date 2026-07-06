# PRD — Riepilogo RDA

## 1. Sintesi

Aggiungere in `apps/stats-rda` una nuova pagina **Riepilogo RDA** raggiungibile al path `/riepilogo-rda`.

La pagina, in italiano, genera statistiche sugli ordini di acquisto RDA presenti nel database `arak`, usando come sorgente principale `rda.purchase_order` e arricchendo i dati con provider, richiedente e budget.

La pagina è read-only e permette all'utente di:

1. scegliere un periodo predefinito o un intervallo date personalizzato;
2. vedere il totale degli importi raggruppato per budget;
3. interagire con un grafico per filtrare il dettaglio;
4. consultare la lista degli ordini sottostanti;
5. scaricare un file Excel con riepilogo e dettagli per budget.

---

## 2. Obiettivi

- Fornire una vista immediata della spesa RDA per budget su un periodo selezionato.
- Rendere confrontabile la nuova pagina con **Riepilogo PA Jira**, mantenendo layout, interazioni e stati coerenti.
- Permettere l'esportazione Excel pronta per analisi e condivisione interna.
- Usare dati RDA nativi da `arak`, senza dipendere dai dati Jira/PA.

---

## 3. Non obiettivi

- Nessuna modifica o scrittura sugli ordini RDA.
- Nessun workflow approvativo o cambio stato ordine.
- Nessuna conversione valuta nella prima versione.
- Nessun confronto periodo-su-periodo nella prima versione.
- Nessuna dashboard multi-dimensione oltre al raggruppamento principale per budget.

---

## 4. Utenti target

- Responsabili amministrativi/acquisti che controllano la spesa RDA.
- Referenti budget che vogliono verificare gli ordini RDA generati in un periodo.
- Utenti interni autorizzati alla mini-app `stats-rda`.

---

## 5. Navigazione e routing

- Nuova pagina: **Riepilogo RDA**
- Path frontend: `/riepilogo-rda`
- La pagina deve essere aggiunta alla navigazione della mini-app `stats-rda` accanto alle pagine esistenti.
- Lingua UI: italiano.

Copy suggerita per nav/tab: `Riepilogo RDA`.

---

## 6. Selezione periodo

La selezione periodo deve essere coerente con **Riepilogo PA Jira**: preset rapidi più intervallo personalizzato.

Preset richiesti:

| Preset | Descrizione | Default |
|---|---|---:|
| `Mese corrente` | Dal primo giorno del mese corrente al primo giorno del mese successivo, esclusivo | No |
| `Mese precedente` | Dal primo giorno del mese precedente al primo giorno del mese corrente, esclusivo | **Sì** |
| `Trimestre corrente` | Dal primo giorno del trimestre corrente al primo giorno del trimestre successivo, esclusivo | No |
| `Trimestre precedente` | Dal primo giorno del trimestre precedente al primo giorno del trimestre corrente, esclusivo | No |
| `Anno corrente` | Dal 1 gennaio dell'anno corrente al 1 gennaio dell'anno successivo, esclusivo | No |
| `Anno precedente` | Dal 1 gennaio dell'anno precedente al 1 gennaio dell'anno corrente, esclusivo | No |
| `Intervallo personalizzato` | Date `from`/`to` scelte dall'utente, con `to` escluso | No |

Per l'intervallo personalizzato il client invia `period=custom&from=YYYY-MM-DD&to=YYYY-MM-DD`; il backend valida che entrambe le date siano presenti e che `from < to`.

Regola tecnica per le date:

- filtro su `rda.purchase_order.created`;
- inizio incluso: `created >= from`;
- fine esclusa: `created < to`;
- evitare `BETWEEN` inclusivo sulla data finale.

---

## 7. Regole dati

### 7.1 Sorgenti

La sorgente principale è:

```sql
rda.purchase_order po
```

Join di arricchimento:

```sql
LEFT JOIN provider_qualifications.provider f ON po.provider_id = f.id
LEFT JOIN users_int."user" u ON po.requester_id = u.id
LEFT JOIN budgets.budget b ON po.budget_id = b.id
```

### 7.2 Clausola base

La clausola base è sempre applicata:

```sql
po.deleted IS NULL
AND po.state IN (
  'CLOSED',
  'DELIVERED_AND_COMPLIANT',
  'PENDING_CHECK_DOCUMENT',
  'PENDING_CONTRACT_VERIFICATION',
  'PENDING_DISPUTE',
  'PENDING_ERP_SAVE',
  'PENDING_LEASING',
  'PENDING_LEASING_ORDER_CREATION',
  'PENDING_PDF_GENERATION',
  'PENDING_PROVIDER_SAVED_IN_ALYANTE',
  'PENDING_SEND',
  'PENDING_VERIFICATION'
)
```

Più filtro periodo:

```sql
AND po.created >= :from
AND po.created < :to
```

### 7.3 Query dettagli base

Query di riferimento:

```sql
SELECT
  po.code,
  po.project,
  po.object,
  u.email AS requester,
  po.budget_id,
  b."name" AS budget_name,
  b."year" AS budget_year,
  po.cost_center,
  po.currency,
  po.total_price,
  po.state,
  po.created,
  f.company_name
FROM rda.purchase_order po
LEFT JOIN provider_qualifications.provider f ON po.provider_id = f.id
LEFT JOIN users_int."user" u ON po.requester_id = u.id
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND po.state IN (
    'CLOSED',
    'DELIVERED_AND_COMPLIANT',
    'PENDING_CHECK_DOCUMENT',
    'PENDING_CONTRACT_VERIFICATION',
    'PENDING_DISPUTE',
    'PENDING_ERP_SAVE',
    'PENDING_LEASING',
    'PENDING_LEASING_ORDER_CREATION',
    'PENDING_PDF_GENERATION',
    'PENDING_PROVIDER_SAVED_IN_ALYANTE',
    'PENDING_SEND',
    'PENDING_VERIFICATION'
  )
  AND po.created >= :from
  AND po.created < :to
ORDER BY po.created DESC, po.code ASC;
```

### 7.4 Aggregazione budget

La prima aggregazione richiesta è per budget RDA.

Chiave budget consigliata:

- `budget_id`, quando presente;
- label visuale composta da `budget_name` e `budget_year`, es. `Trasferte 2026`;
- fallback `Senza budget` quando `budget_id` o `budget_name` non sono valorizzati.

Query aggregata di riferimento:

```sql
SELECT
  COALESCE(po.budget_id::text, 'senza-budget') AS budget_key,
  CASE
    WHEN po.budget_id IS NULL OR b."name" IS NULL OR btrim(b."name") = ''
    THEN 'Senza budget'
    WHEN b."year" IS NULL
    THEN btrim(b."name")
    ELSE btrim(b."name") || ' ' || b."year"::text
  END AS budget_label,
  COUNT(*) AS order_count,
  COALESCE(SUM(po.total_price), 0) AS amount
FROM rda.purchase_order po
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND po.state IN (:allowed_states)
  AND po.created >= :from
  AND po.created < :to
GROUP BY budget_key, budget_label
ORDER BY amount DESC, budget_label ASC;
```

### 7.5 Normalizzazioni e decisioni dati

- Budget vuoto o `NULL`: mostrarlo come `Senza budget`.
- Importo `NULL`: trattarlo come `0` nell'aggregazione, usando `COALESCE(po.total_price, 0)`.
- Valuta: non è prevista conversione valuta nella prima versione. Se nel periodo sono presenti valute diverse, mostrarle nel dettaglio e indicare nei testi che il totale è una somma nominale.
- Provider assente: mostrare `—` nella UI.
- Richiedente assente: mostrare `—` nella UI.
- Stato: mostrare il valore tecnico RDA nella prima versione; eventuale traduzione label può essere aggiunta successivamente.

---

## 8. Presentazione UI

### 8.1 Header pagina

Titolo: `Riepilogo RDA`

Sottotitolo suggerito:

> Totali degli ordini RDA per budget nel periodo selezionato.

### 8.2 Toolbar

Elementi:

- select periodo;
- se `Intervallo personalizzato`, input data `Da` e `A esclusa`;
- indicazione del range calcolato, es. `01/01/2026 – 06/07/2026 escluso`;
- pulsante export Excel: `Scarica Excel`.

### 8.3 KPI riepilogo

Mostrare almeno:

- totale complessivo del periodo;
- numero ordini inclusi;
- numero budget trovati.

Copy suggerite:

- `Totale periodo`
- `Ordini RDA`
- `Budget`

### 8.4 Grafico

Il grafico deve rappresentare i dati per budget.

Opzione preferita: grafico a barre, coerente con **Riepilogo PA Jira**.

Requisiti:

- ogni budget deve essere cliccabile;
- il click imposta il budget selezionato come filtro del dettaglio;
- il budget selezionato deve essere evidenziato;
- deve essere disponibile un'azione per rimuovere il filtro, es. `Mostra tutti`;
- tooltip/label con budget, importo, percentuale sul totale e numero ordini.

### 8.5 Lista dettagli sotto al grafico

Sotto il grafico mostrare la lista degli ordini coerenti con il periodo e con l'eventuale budget selezionato.

Campi minimi consigliati:

- ordine (`code`);
- progetto (`project`);
- oggetto (`object`);
- budget (`budget_name`, `budget_year`);
- centro di costo (`cost_center`);
- importo totale (`total_price`);
- valuta (`currency`);
- richiedente (`requester`);
- fornitore (`company_name`);
- stato (`state`);
- data creazione (`created`).

La lista deve aggiornarsi quando l'utente cambia periodo o seleziona un budget dal grafico.

Il dettaglio del periodo selezionato deve essere caricato integralmente, senza paginazione lato server nella prima versione.

---

## 9. Export Excel

L'utente deve poter scaricare un documento Excel relativo al periodo selezionato.

L'export deve includere sempre tutti i dati del periodo selezionato, anche quando nella UI è attivo un filtro budget tramite grafico.

Nome file suggerito:

```text
riepilogo-rda_<preset>_<from>_<to>.xlsx
```

Esempio:

```text
riepilogo-rda_custom_2026-01-01_2026-07-06.xlsx
```

### Sheet 1 — Totali budget

Nome sheet: `Totali budget`

Colonne minime:

- Budget;
- Anno budget;
- Numero ordini;
- Importo totale;
- Percentuale sul totale periodo.

### Sheet dettagli per budget

Creare un foglio per ciascun budget, con nome sheet compatibile Excel.

Colonne minime:

- Codice ordine;
- Progetto;
- Oggetto;
- Budget;
- Anno budget;
- Centro di costo;
- Importo totale;
- Valuta;
- Richiedente;
- Fornitore;
- Stato;
- Data creazione.

Per budget `NULL` usare sheet `Senza budget`.

---

## 10. Contratto dati suggerito

### Endpoint riepilogo

```text
GET /api/stats-rda/v1/rda/riepilogo?period=this_month|previous_month|this_quarter|previous_quarter|current_year|previous_year
GET /api/stats-rda/v1/rda/riepilogo?period=custom&from=YYYY-MM-DD&to=YYYY-MM-DD
```

Response suggerita:

```json
{
  "period": {
    "preset": "custom",
    "from": "2026-01-01",
    "to": "2026-07-06"
  },
  "totals": {
    "order_count": 10,
    "budget_count": 6,
    "amount": 56189.11
  },
  "budgets": [
    {
      "budget_key": "22",
      "budget": "Hardware 2026",
      "budget_id": 22,
      "budget_name": "Hardware",
      "budget_year": 2026,
      "order_count": 1,
      "amount": 35218.50,
      "percentage": 62.68
    }
  ],
  "details": [
    {
      "code": "PO-50025/2026",
      "project": "HP-7716/2026 - CREA INFORMATICA SRL",
      "object": "Acquisto server DELL",
      "requester": "matteo.redaelli@cdlan.it",
      "budget_id": 22,
      "budget_name": "Hardware",
      "budget_year": 2026,
      "cost_center": "Delivery CLOUD",
      "currency": "EUR",
      "total_price": 35218.50,
      "state": "PENDING_VERIFICATION",
      "created": "2026-03-13T14:33:46+01:00",
      "company_name": "YNVOLVE B.V."
    }
  ]
}
```

### Endpoint export

```text
GET /api/stats-rda/v1/rda/riepilogo/export?period=previous_month
GET /api/stats-rda/v1/rda/riepilogo/export?period=custom&from=YYYY-MM-DD&to=YYYY-MM-DD
```

Response:

- content type Excel `.xlsx`;
- attachment con nome file descrittivo.

---

## 11. Stati e messaggi

- Loading: `Caricamento riepilogo RDA…`
- Empty: `Nessun ordine RDA trovato per il periodo selezionato.`
- Errore dati: `Non è stato possibile caricare il riepilogo RDA.`
- Export in corso: `Preparazione file Excel…`
- Export errore: `Non è stato possibile generare il file Excel.`

---

## 12. Criteri di accettazione

1. La pagina `Riepilogo RDA` è raggiungibile dalla navigazione di `stats-rda`.
2. La pagina mostra i dati RDA filtrati per periodo usando `rda.purchase_order.created`.
3. Il backend include solo ordini con `po.deleted IS NULL` e `po.state` nella allowlist definita.
4. I totali sono raggruppati per budget RDA e ordinati per importo decrescente.
5. Il click su un budget filtra la tabella dettagli.
6. `Mostra tutti` rimuove il filtro budget.
7. L'export Excel include tutti i dettagli del periodo, non solo quelli filtrati in UI.
8. Il range personalizzato usa `from` incluso e `to` escluso.
9. La UI è in italiano e coerente con **Riepilogo PA Jira**.
10. La pagina usa `@mrsmith/api-client` per JSON ed export, senza `fetch` anonimi.

---

## 13. Note implementative

- Riutilizzare dove possibile componenti, helper periodo, formattatori e stile della pagina **Riepilogo PA Jira**.
- Mantenere endpoint RDA separati dagli endpoint PA/Jira.
- Il prefisso reale API deve restare sotto `/api/stats-rda/v1` lato frontend.
- La query SQL deve usare parametri bind per `from` e `to`; non interpolare date in stringa.
- Non aggiungere test automatici senza approvazione esplicita.

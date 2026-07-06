# PRD — Riepilogo PA Jira

## 1. Sintesi

Aggiungere in `apps/stats-rda` una nuova pagina **Riepilogo PA Jira** raggiungibile al path `/riepilogo-pa`.
La pagina, in italiano, genera statistiche sugli ordini di acquisto storici presenti nel database `arak`, schema `pa`, con primo livello di analisi aggregato per **Budget di riferimento**.

La pagina è read-only e permette all'utente di:

1. scegliere un periodo predefinito di creazione ordini;
2. vedere il totale degli importi raggruppato per budget;
3. interagire con un grafico per filtrare il dettaglio;
4. consultare la lista degli ordini sottostanti;
5. scaricare un file Excel con riepilogo e dettagli per budget.

---

## 2. Obiettivi

- Fornire una vista immediata della spesa PA per budget su periodi ricorrenti.
- Rendere il grafico operativo, non solo informativo: il click su un budget filtra il dettaglio.
- Permettere l'esportazione Excel pronta per analisi e condivisione interna.
- Usare come sorgente dati esclusiva `pa.issue`, filtrata sugli ordini di tipo acquisto non annullati.

---

## 3. Non obiettivi

- Nessuna modifica o scrittura su Jira/PA.
- Nessun inserimento di filtri avanzati nella prima versione oltre al periodo.
- Nessuna conversione valuta se gli ordini non sono in euro.
- Nessun confronto periodo-su-periodo nella prima versione.
- Nessuna dashboard multi-dimensione oltre al raggruppamento principale per budget.

---

## 4. Utenti target

- Responsabili amministrativi/acquisti che controllano la spesa per budget.
- Referenti budget che vogliono verificare gli ordini PA generati in un periodo.
- Utenti interni autorizzati alla mini-app `stats-rda`.

---

## 5. Navigazione e routing

- Nuova pagina: **Riepilogo PA Jira**
- Path frontend: `/riepilogo-pa`
- La pagina deve essere aggiunta alla navigazione della mini-app `stats-rda` accanto alle pagine esistenti.
- Lingua UI: italiano.

Copy suggerita per nav/tab: `Riepilogo PA Jira`.

---

## 6. Selezione periodo

La selezione iniziale offre preset rapidi e un intervallo date personalizzato.

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

- filtro su `pa.issue.created`;
- inizio incluso: `created >= from`;
- fine esclusa: `created < to`;
- usare timezone coerente con il backend/database, evitando `BETWEEN` inclusivo sulla data finale per non duplicare ordini a cavallo periodo.

---

## 7. Regole dati

La sorgente dati è `pa.issue`.

La clausola base è sempre applicata:

```sql
issue_type = 'Acquisto'
AND resolution NOT IN ('Annullato','Rifiutato')
```

Più filtro periodo:

```sql
AND created >= :from
AND created < :to
```

Prima aggregazione richiesta:

```sql
SELECT
  budget_di_riferimento,
  SUM(importo_totale) AS importo
FROM pa.issue
WHERE issue_type = 'Acquisto'
  AND resolution NOT IN ('Annullato','Rifiutato')
  AND created >= :from
  AND created < :to
GROUP BY budget_di_riferimento;
```

### Normalizzazioni e decisioni dati

- Budget vuoto o `NULL`: mostrarlo come `Senza budget`.
- Importo `NULL`: trattarlo come `0` nell'aggregazione, usando `COALESCE(importo_totale, 0)`.
- Valuta: tutti gli importi vanno sommati come se fossero in euro. Non è prevista alcuna conversione valuta e il totale aggregato è presentato come importo unico.
- `resolution NULL`: non deve essere incluso. La clausola resta `resolution NOT IN ('Annullato','Rifiutato')`, quindi il comportamento SQL standard esclude anche i valori `NULL`.

---

## 8. Presentazione UI

### 8.1 Header pagina

Titolo: `Riepilogo PA Jira`

Sottotitolo suggerito:

> Totali degli ordini di acquisto PA per budget nel periodo selezionato.

### 8.2 Toolbar

Elementi:

- select preset periodo;
- indicazione del range calcolato, es. `01/06/2026 – 01/07/2026 escluso` oppure copy business `Giugno 2026`;
- pulsante export Excel: `Scarica Excel`.

### 8.3 Stato riepilogo

Mostrare almeno:

- totale complessivo del periodo;
- numero ordini inclusi;
- numero budget trovati.

### 8.4 Grafico

Il grafico deve rappresentare i dati per budget.

Opzioni ammesse:

- grafico a barre, preferibile quando i budget sono molti;
- grafico a torta/donut, accettabile se i budget sono pochi.

Requisiti:

- ogni budget deve essere cliccabile;
- il click imposta il budget selezionato come filtro del dettaglio;
- il budget selezionato deve essere evidenziato;
- deve essere disponibile un'azione per rimuovere il filtro, es. `Mostra tutti`;
- tooltip/label con budget, importo e percentuale sul totale.

### 8.5 Lista dettagli sotto al grafico

Sotto il grafico mostrare la lista degli ordini coerenti con il periodo e con l'eventuale budget selezionato.

Campi minimi consigliati:

- ordine (`issue_key` / `numero_ordine`);
- oggetto (`summary`);
- budget di riferimento;
- importo totale;
- valuta;
- richiedente (`reporter_name`);
- fornitore selezionato;
- stato/resolution;
- data creazione.

La lista deve aggiornarsi quando l'utente cambia periodo o seleziona un budget dal grafico.

Il dettaglio del periodo selezionato deve essere caricato integralmente, senza paginazione lato server nella prima versione.

---

## 9. Export Excel

L'utente deve poter scaricare un documento Excel relativo al periodo selezionato.

L'export deve includere sempre tutti i dati del periodo selezionato, anche quando nella UI è attivo un filtro budget tramite grafico.

Nome file suggerito:

```text
riepilogo-pa-jira_<preset>_<from>_<to>.xlsx
```

Esempio:

```text
riepilogo-pa-jira_mese-precedente_2026-06-01_2026-07-01.xlsx
```

### Sheet 1 — Totali budget

Nome sheet: `Totali budget`

Colonne minime:

- Budget;
- Numero ordini;
- Importo totale;
- Valuta, se applicabile;
- Percentuale sul totale periodo.

### Sheet dettagli per budget

Creare uno sheet di dettaglio per ogni budget presente nel riepilogo.

Nome sheet: nome budget normalizzato e troncato secondo i limiti Excel.

Colonne minime:

- Issue key;
- Numero ordine;
- Summary;
- Budget di riferimento;
- Importo totale;
- Valuta;
- Richiedente;
- Fornitore selezionato;
- Status;
- Resolution;
- Created.

Per budget `NULL` usare sheet `Senza budget`.

---

## 10. Contratto dati suggerito

### Endpoint riepilogo

`GET /api/stats-rda/v1/pa/riepilogo?period=this_month|previous_month|this_quarter|previous_quarter|current_year|previous_year`

Range personalizzato: `GET /api/stats-rda/v1/pa/riepilogo?period=custom&from=YYYY-MM-DD&to=YYYY-MM-DD`

Response suggerita:

```json
{
  "period": {
    "preset": "previous_month",
    "from": "2026-06-01",
    "to": "2026-07-01"
  },
  "totals": {
    "order_count": 42,
    "budget_count": 6,
    "amount": 123456.78
  },
  "budgets": [
    {
      "budget": "Cloud",
      "order_count": 10,
      "amount": 45000.00,
      "percentage": 36.45
    }
  ],
  "details": [
    {
      "issue_key": "PA-1821",
      "numero_ordine": "PA/1821-2021",
      "summary": "Acquisto ...",
      "budget_di_riferimento": "Cloud",
      "importo_totale": 1200.00,
      "valuta": "Euro €",
      "reporter_name": "Mario Rossi",
      "fornitore_selezionato": "Fornitore S.p.A.",
      "status": "Closed",
      "resolution": "Done",
      "created": "2026-06-15T10:30:00Z"
    }
  ]
}
```

### Endpoint export

`GET /api/stats-rda/v1/pa/riepilogo/export?period=previous_month`

Range personalizzato: `GET /api/stats-rda/v1/pa/riepilogo/export?period=custom&from=YYYY-MM-DD&to=YYYY-MM-DD`

Response:

- content type Excel `.xlsx`;
- attachment con nome file descrittivo.

---

## 11. Stati e messaggi

- Loading: `Caricamento riepilogo PA…`
- Empty: `Nessun ordine di acquisto trovato per il periodo selezionato.`
- Errore dati: `Non è stato possibile caricare il riepilogo PA.`
- Export in corso: `Preparazione file Excel…`
- Export errore: `Non è stato possibile generare il file Excel.`

---

## 12. Criteri di accettazione

1. Visitando `/riepilogo-pa`, la pagina mostra il preset `Mese precedente` selezionato.
2. Il riepilogo usa solo ordini con `issue_type = 'Acquisto'` e `resolution NOT IN ('Annullato','Rifiutato')`.
3. Cambiando preset, grafico, totali e lista dettagli si aggiornano.
4. I totali sono raggruppati per `budget_di_riferimento`.
5. Il grafico mostra un dato per ogni budget e permette di filtrare la lista dettagli cliccando su un budget.
6. È possibile rimuovere il filtro budget e tornare alla vista completa del periodo.
7. Il bottone `Scarica Excel` genera un file con uno sheet `Totali budget` e uno sheet dettagli per ogni budget.
8. I budget nulli/vuoti sono visibili come `Senza budget`.
9. In assenza di dati, la pagina mostra uno stato vuoto chiaro e non un errore.
10. La UI è interamente in italiano.

---

## 13. Decisioni confermate

- Gli importi aggregati includono tutti i valori come se fossero in euro; non si applicano conversioni e non si separa per valuta.
- Le righe con `resolution NULL` non sono incluse: resta valida la clausola `resolution NOT IN ('Annullato','Rifiutato')`.
- Il dettaglio sotto il grafico viene caricato integralmente per il periodo selezionato.
- Il file Excel esporta sempre tutti i budget e tutti i dettagli del periodo selezionato, indipendentemente dall'eventuale filtro budget attivo nella UI.

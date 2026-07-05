# Database `arak` — guida all'uso dello schema `pa`

Guida per chi deve usare il database `arak` (schema **`pa`**) come
**archivio storico** degli ordini di acquisto (progetto **PA**) e come base di dati
per produrre **report** (spesa per budget/area, per richiedente, per fornitore,
export Excel, ricerche testuali).

Lo schema è in forma semplice e leggibile: i valori sono testi/etichette
comprensibili, nomi di persone e importi numerici. È sufficiente una conoscenza
di base di SQL.

---

## Cos'è

L'archivio degli **ordini di acquisto** del progetto PA. Una riga della tabella
principale `pa.issue` corrisponde a un ordine (un "ticket"), con i suoi dati
anagrafici, economici, di stato e di dettaglio.

- **Range temporale**: novembre 2019 → maggio 2026
- **Volume**: 8.756 ordini, 7.500 righe di dettaglio, 125.674 eventi di modifica,
  1.808 commenti, 8.151 allegati (solo metadati — vedi i [caveat in schema.md, §7](schema.md))

---

## Come è fatto (in 30 secondi)

Tutto ruota attorno all'**ordine**, identificato da `issue_key` (`PA-1821`) e da
`numero_ordine` (`PA/1821-2021`). Una tabella "flat" centrale + tabelle satelliti
collegate tramite `issue_id`:

```
                         ┌─────────────────────────────┐
                         │  pa.issue        (1 riga)    │  ← l'ordine, tutto in riga
                         │  + 53 campi specifici        │
                         └──────────────┬──────────────┘
                                        │ issue_id
   ┌──────────────┬──────────────┬──────┴───────┬──────────────┬──────────────┐
   ▼              ▼              ▼              ▼              ▼              ▼
pa.line_item  pa.comment   pa.change_*    pa.attachment  pa.issue_link  pa.issue_custom_value
(righe d'ordine) (commenti) (storico)     (allegati)     (collegamenti) (valori in forma EAV)
```

Dettaglio completo: [`schema.md`](schema.md).

---

## Cosa posso fare subito (report tipici)

Le dimensioni utili per i report sono:

| Dimensione | Colonna | Densità | Note |
|---|---|---|---|
| **Budget / area** | `pa.issue.budget_di_riferimento` | 88% | **La più importante**: DC, COLO, Communication, PEOPLE, Cloud, TLC… |
| **Richiedente** | `pa.issue.reporter_name` | 100% | persona che ha aperto l'ordine |
| **Assegnatario** | `pa.issue.assignee_name` | 97% | chi ha lavorato l'ordine |
| **Fornitore** | `pa.issue.fornitore_selezionato` | ~50% | ragione sociale |
| **Tipo ordine** | `pa.issue.tipo_di_ordine` | 92% | Standard / E-commerce |
| **Tipo ordine (macro)** | `pa.issue.issue_type` | 100% | Acquisto, Servizio, Merce, Leasing… |
| **Stato** | `pa.issue.status` | 100% | Closed, Annullato, Non approvato… |
| **Data** | `pa.issue.created` | 100% | per anno/mese/trimestre |
| **Valuta** | `pa.issue.valuta` | ~99% | Euro €, Dollaro $… |

Ricette SQL pronte (spesa per budget, per richiedente, per fornitore, trend mensile,
ricerca full-text, export Excel): [`reporting.md`](reporting.md).

---

## Come leggere un singolo ordine

Per capire com'è fatto un ordine "dall'interno" c'è [`docs/pa_card.py`](pa_card.py):
un **esempio illustrativo da leggere** (non uno strumento da eseguire così com'è)
che mostra quali interrogazioni servono per ricostruire la scheda completa di un
ordine e come si può formattarne l'output.

Il ticket `PA-1821` ("Cage per Isola 7 Piano Terra", 13.000 €, fornitore TECNOSTEEL,
budget COLO) è commentato riga per riga — ogni sezione della scheda è mappata alle
tabelle/colonne sorgente — in [`esempio.md`](esempio.md).

---

## Indice dei documenti

| Documento | Cosa contiene |
|---|---|
| **[schema.md](schema.md)** | Data dictionary: ogni tabella `pa.*`, colonne chiave, significato dei campi specifici, insidie e limiti. **Il documento di riferimento.** |
| **[reporting.md](reporting.md)** | Dimensioni disponibili + ricette SQL pronte per i report + export Excel + ricerca testuale. |
| **[esempio.md](esempio.md)** | L'ordine PA-1821 spiegato riga per riga + descrizione dell'esempio `pa_card.py`. |
| **[pa_card.py](pa_card.py)** | Esempio illustrativo (da leggere): le query e la formattazione per comporre la "scheda" di un ordine. |

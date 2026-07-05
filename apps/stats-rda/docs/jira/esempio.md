# L'ordine PA-1821, spiegato riga per riga

Questa pagina mostra com'è fatto un ordine di acquisto nello schema `pa`, usando
un esempio reale e completo: **PA-1821** — *"Cage per Isola 7 Piano Terra"*,
13.000 €, fornitore TECNOSTEEL, budget COLO.

Per ogni sezione della scheda indichiamo da **quale tabella e quali colonne**
arrivano i dati, così è chiaro come lo schema si traduca in informazioni leggibili.
La scheda è un esempio (prodotto dallo script illustrativo [`pa_card.py`](pa_card.py))
di come si presenta un ordine quando lo si legge per intero.

---

## La scheda (output di esempio) — estratto, abbreviato per leggibilità

```
══════════════════════════════════════════════════════════════════════════════
  PA-1821   ·   Cage per Isola 7 Piano Terra
══════════════════════════════════════════════════════════════════════════════
  Tipo: Acquisto    Stato: Closed    Priorità: Normale    Risoluzione: Done
  Numero ordine: PA/1821-2021    Valuta: Euro €    Stato (custom): APPROVATO
  Creato: 2021-04-14 09:49    Aggiornato: 2021-07-22 06:43    Risolto: 2021-07-22 06:43
  Richiedente: Stefano Vatta (stefano.vatta@cdlan.it)
  Assegnatario: Stefano Vatta    Creatore: Stefano Vatta
──────────────────────────────────────────────────────────────────────────────
  ▪ ACQUISTO / ECONOMICO
    Importo totale: 13.000,00 €    Fornitore: TECNOSTEEL S.R.L.
    Tipo ordine: Standard (con invio ordine al fornitore)    Tipo documento: TSF-ORDINE
    Budget riferimento: COLO    Corrente: 117128.27    Totale: 400000
    Limite approv.: 500    % approvazione: 90
──────────────────────────────────────────────────────────────────────────────
  ▪ RIGHE D'ORDINE (3)
    [Merci]
      1. IMPIANTI — Tecnosteel  qtà 1  importo 11.700,00 €  tot 11.700,00 €
    [Servizi]
      1. SPESE TRASPORTO   qtà 1  importo 300,00 €   tot 300,00 €   (pag. All'attivazione · Una tantum)
      2. CONS-TEC          qtà 1  importo 1.000,00 € tot 1.000,00 € (pag. All'attivazione · Una tantum)
──────────────────────────────────────────────────────────────────────────────
  ▪ COMMENTI (3)
    [2021-04-19 11:03] Amministrazione:
      Ciao Stefano, il fornitore ci ha chiesto l'offerta relativa a questo ordine […]
    [2021-04-19 11:08] Stefano Vatta:
      In realtà mi sono fatto un autosconto… se fanno storie fammi chiamare
    [2021-07-21 15:01] Amministrazione:
      Ciao Stefano, questo ordine è arrivato? Grazie, Agnese
──────────────────────────────────────────────────────────────────────────────
  ▪ ALLEGATI (2) — solo metadati (i file non sono nel DB)
    - conto economico_CDLAN_CAGE ISOLA 7.pdf  (482 kB)  2021-04-16  Stefano Vatta
    - OA-ITA-PA-1821.pdf                      (161 kB)  2021-04-16  cdlan
──────────────────────────────────────────────────────────────────────────────
  ▪ COLLEGAMENTI (2)
    - subtask: PA-1838
    - subtask: PA-1839
──────────────────────────────────────────────────────────────────────────────
  ▪ ULTIME MODIFICHE (max 12)
    [2021-07-22 06:43] Stefano Vatta — resolution: In corso → Done
    [2021-07-22 06:43] Stefano Vatta — status: Articoli da evadere → Closed
    [2021-07-21 15:01] cdlan — Budget Corrente: 78043.12 → 117128.27
    [2021-07-21 15:01] cdlan — Budget Totale: 250000 → 400000
    [2021-04-16 12:07] Stefano Vatta — status: Ordine inviato → Articoli da evadere
    [2021-04-16 12:06] Stefano Vatta — status: Approvato → Ordine inserito
    […]
```

---

## Mappatura sezione → tabella/colonna

### 1. Intestazione
| Elemento | Sorgente |
|---|---|
| `PA-1821` / oggetto | `pa.issue.issue_key`, `pa.issue.summary` |
| Tipo, Stato, Priorità, Risoluzione | `pa.issue.issue_type`, `status`, `priority`, `resolution` |
| `PA/1821-2021`, `Euro €`, `APPROVATO` | `pa.issue.numero_ordine`, `valuta`, `stato` (campo di dominio) |
| Date | `pa.issue.created`, `updated`, `resolution_date`, `due_date` |
| Richiedente / Assegnatario / Creatore | `pa.issue.reporter_*`, `assignee_*`, `creator_*` |

> Nota: `stato` (campo di dominio, "APPROVATO") è diverso da `status` (workflow, "Closed").

### 2. Acquisto / economico
| Elemento | Sorgente |
|---|---|
| Importo totale, Fornitore | `pa.issue.importo_totale`, `fornitore_selezionato` |
| Tipo ordine, Tipo documento | `pa.issue.tipo_di_ordine`, `tipo_documento` |
| Budget riferimento / corrente / totale | `pa.issue.budget_di_riferimento`, `budget_corrente`, `budget_totale` |
| Limite / % approvazione | `pa.issue.limite_approvazione`, `percentuale_approvazione` |

Tutte queste colonne sono **campi specifici del dominio** con valore già leggibile
(vedi [schema.md, §2.2](schema.md)).
L'`importo_totale` (13.000) è coerente con la somma delle righe: 11.700 + 300 + 1.000.

### 3. Righe d'ordine
| Elemento | Sorgente |
|---|---|
| Tipo (`Merci`/`Servizi`) | `pa.line_item.grid` |
| Posizione | `pa.line_item.row_no` |
| Articolo, vendor, part number | `pa.line_item.articolo_name`, `vendor`, `part_number` |
| Quantità, importo, totale | `pa.line_item.quantita`, `importo`, `prezzo_totale` |
| Pagamento, durata, scadenza | `pa.line_item.pagamento_name`, `durata_name`, `data_pagamento` |

Qui il tipo Articoli non è presente (l'ordine ha solo Merci e Servizi); si nota
che `vendor`/`part_number` sono vuoti sui **Servizi** (come atteso, vedi
[schema.md, §3](schema.md)).

### 4. Commenti
| Elemento | Sorgente |
|---|---|
| Data, autore, testo | `pa.comment.created`, `author_name`, `actionbody` |

Il dialogo mostra il processo reale: l'ufficio Amministrazione chiede chiarimenti,
il richiedente risponde, e a distanza di mesi si verifica la consegna.

### 5. Allegati
| Elemento | Sorgente |
|---|---|
| Nome, dimensione, data, autore | `pa.attachment.filename`, `filesize`, `created`, `author_name` |

La tabella contiene **solo metadati**: i file binari non sono inclusi nel database
(vedi [schema.md, §6](schema.md) e le insidie).

### 6. Collegamenti
| Elemento | Sorgente |
|---|---|
| Tipo link, ordine collegato | `pa.issue_link.link_name`, `source_key`/`destination_key` |

PA-1821 ha due subtask collegati (PA-1838, PA-1839).

### 7. Ultime modifiche (storico)
| Elemento | Sorgente |
|---|---|
| Data, autore | `pa.change_group.created`, `pa.change_group.author_name` |
| Campo, vecchio/nuovo valore | `pa.change_item.field`, `old_string`, `new_string` |

Questa sezione è particolarmente interessante perché **mostra l'evoluzione**:
- il budget totale del COLO è passato da **250.000 a 400.000 €** (aumento a metà lavorazione);
- il `budget_corrente` si è aggiornato più volte (91.043 → 78.043 → 117.128);
- il workflow è transitato per gli stati *Approvato → Ordine inserito → Ordine inviato → Articoli da evadere → Closed*.

---

## L'esempio `pa_card.py`

[`pa_card.py`](pa_card.py) è uno **script illustrativo da leggere**, non uno
strumento pronto all'uso: mostra, in concreto, quali interrogazioni servono per
ricostruire la scheda completa di un ordine e come si può formattare l'output.

### Cosa contiene
- **`run_query(sql)`** — il **punto di connessione** al database. È l'unica parte
  da adattare al proprio stack (psycopg2, psycopg, JDBC, SQLAlchemy, …): riceve
  una SELECT e restituisce una lista di dict (chiavi = nomi colonna).
- **`fetch_*`** — una funzione per sezione della scheda (`fetch_issue`,
  `fetch_line_items`, `fetch_comments`, `fetch_attachments`, `fetch_links`,
  `fetch_history`); ciascuna contiene una `SELECT` standard eriusabile così com'è.
- **`render_*`** — un esempio di **formattazione** testuale (separata
  dall'accesso ai dati).

### Esempio di riuso (la singola SELECT)
Le `SELECT` delle `fetch_*` si possono estrarre e usare direttamente, in qualsiasi
linguaggio. Ad esempio, le righe d'ordine di un ticket:
```sql
SELECT grid, row_no, articolo_name, vendor, part_number, descrizione,
       quantita, importo, prezzo_totale, pagamento_name, durata_name, data_pagamento
FROM pa.line_item
WHERE issue_key = 'PA-1821'
ORDER BY grid, row_no;
```

### Sezioni della scheda
Intestazione → Acquisto/economico → Descrizione → Righe d'ordine → Commenti →
Allegati → Collegamenti → Ultime modifiche (il loro numero è configurabile).

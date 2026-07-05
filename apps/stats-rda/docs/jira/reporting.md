# Ricette di reporting

Come costruire i report più utili dallo schema `pa` (database `arak`).
Le query sono SQL standard, eseguibili con qualsiasi client/driver PostgreSQL.

Riferimento colonne: [schema.md](schema.md). Dimensioni e metriche sono riassunte
sotto per consultazione rapida.

---

## 1. Dimensioni disponibili

| Dimensione | Colonna | Note |
|---|---|---|
| **Budget / area** | `pa.issue.budget_di_riferimento` | DC, COLO, Cloud, Communication, TLC, PEOPLE, Technology… (88% popolato) |
| **Richiedente** | `pa.issue.reporter_name` | 100% popolato |
| **Assegnatario** | `pa.issue.assignee_name` | 97% |
| **Fornitore** | `pa.issue.fornitore_selezionato` | ragione sociale (~50%) |
| **Tipo ordine** | `pa.issue.tipo_di_ordine` | Standard / E-commerce |
| **Tipo ticket** | `pa.issue.issue_type` | Acquisto / Servizio / Merce / Leasing… |
| **Stato** | `pa.issue.status` | Closed / Annullato / Non approvato… |
| **Periodo** | `pa.issue.created` | anno / mese / trimestre |
| **Valuta** | `pa.issue.valuta` | Euro €, Dollaro $… |

## 2. Metriche

| Metrica | Espressione |
|---|---|
| Numero ordini | `count(*)` |
| Spesa totale | `sum(importo_totale)` |
| Spesa media / max | `avg(importo_totale)`, `max(importo_totale)` |
| Budget disponibile | `budget_totale` (livello singolo ticket) |
| Spesa sul budget | `budget_corrente` |
| % approvazione | `percentuale_approvazione` |

> ⚠️ **Filtrare sempre la valuta** quando si aggregano gli importi: la maggior
> parte è in Euro ma ci sono valute diverse. Nei report si usa in genere
> `WHERE valuta = 'Euro €'`.

---

## 3. Ricette pronte

### 3.1 Spesa per budget / area
```sql
SELECT budget_di_riferimento AS budget,
       count(*)               AS n_ordini,
       sum(importo_totale)    AS spesa_totale,
       round(avg(importo_totale), 2) AS spesa_media
FROM pa.issue
WHERE importo_totale IS NOT NULL AND valuta = 'Euro €'
GROUP BY 1
ORDER BY spesa_totale DESC NULLS LAST;
```

### 3.2 Spesa per richiedente (top 15)
```sql
SELECT reporter_name      AS richiedente,
       count(*)           AS n_ordini,
       sum(importo_totale) AS spesa_totale
FROM pa.issue
WHERE reporter_name IS NOT NULL AND importo_totale IS NOT NULL AND valuta = 'Euro €'
GROUP BY 1 ORDER BY spesa_totale DESC NULLS LAST LIMIT 15;
```

### 3.3 Spesa per fornitore (top 15)
```sql
SELECT fornitore_selezionato AS fornitore,
       count(*)              AS n_ordini,
       sum(importo_totale)   AS spesa_totale
FROM pa.issue
WHERE fornitore_selezionato IS NOT NULL AND valuta = 'Euro €'
GROUP BY 1 ORDER BY spesa_totale DESC NULLS LAST LIMIT 15;
```

### 3.4 Trend mensile (per anno)
```sql
SELECT to_char(created, 'YYYY-MM') AS mese,
       count(*)                    AS ordini,
       sum(importo_totale) FILTER (WHERE valuta='Euro €') AS spesa_eur
FROM pa.issue
WHERE created >= '2024-01-01'
GROUP BY 1 ORDER BY 1;
```

### 3.5 Ordini per anno e stato
```sql
SELECT extract(year FROM created) AS anno, status, count(*)
FROM pa.issue
GROUP BY 1, 2 ORDER BY 1, 2;
```

### 3.6 Ordini ancora aperti (non chiusi) per assegnatario
```sql
SELECT assignee_name, count(*), sum(importo_totale) AS importo
FROM pa.issue
WHERE status NOT IN ('Closed', 'Annullato')
GROUP BY 1 ORDER BY count(*) DESC;
```

### 3.7 Top articoli per spesa (righe d'ordine)
```sql
SELECT articolo_name, count(*) AS n_righe, sum(importo) AS importo_tot
FROM pa.line_item
WHERE articolo_name IS NOT NULL AND articolo_name <> ''
GROUP BY 1 ORDER BY importo_tot DESC NULLS LAST LIMIT 15;
```
> Per i totali nella sola griglia **Articoli** ricorda che `prezzo_totale` è vuoto:
> usare `quantita * importo`. Vedi [schema.md, §3](schema.md).

### 3.8 Ricerca testuale (su summary + descrizione)
```sql
SELECT issue_key, summary, importo_totale, valuta, created
FROM pa.issue
WHERE summary ILIKE '%firewall%' OR description ILIKE '%firewall%'
ORDER BY created DESC;
```

### 3.9 Ricerca anche nei commenti
```sql
SELECT DISTINCT i.issue_key, i.summary
FROM pa.issue i
JOIN pa.comment c ON c.issue_id = i.id
WHERE c.actionbody ILIKE '%firewall%'
   OR i.summary ILIKE '%firewall%';
```

### 3.10 Tempi medi di approvazione (SLA)
```sql
SELECT extract(year FROM created) AS anno,
       round(avg(extract(epoch FROM (approvato - inviato_in_approvazione)) / 86400)::numeric, 1) AS giorni_media,
       count(*) FILTER (WHERE approvato IS NOT NULL) AS approvati
FROM pa.issue
WHERE inviato_in_approvazione IS NOT NULL AND approvato IS NOT NULL
GROUP BY 1 ORDER BY 1;
```

### 3.11 Evoluzione di un campo nello storico (es. budget di un ticket)
```sql
SELECT to_char(g.created, 'YYYY-MM-DD HH24:MI') AS quando,
       g.author_name, i.field, i.old_string, i.new_string
FROM pa.change_group g
JOIN pa.change_item i ON i.group_id = g.id
WHERE g.issue_key = 'PA-1821' AND i.field = 'Budget Totale'
ORDER BY g.created;
```

---

## 4. Ricerca testuale avanzata (opzionale)

Per ricerche frequenti su summary/descrizione/commenti si può creare un indice
GIN full-text (non presente di default):

```sql
-- indice sulla tabella issue
CREATE INDEX ix_issue_fts ON pa.issue USING gin (to_tsvector('italian',
    coalesce(summary,'') || ' ' || coalesce(description,'')));

-- query (LATERAL per definire gli alias `vec` e `q`)
SELECT issue_key, summary, ts_rank_cd(vec, q) AS rank
FROM pa.issue
     CROSS JOIN LATERAL to_tsvector('italian',
         coalesce(summary,'') || ' ' || coalesce(description,'')) AS vec
     CROSS JOIN to_tsquery('italian', 'firewall & licenze') AS q
WHERE vec @@ q
ORDER BY rank DESC;
```

Per la maggior parte dei casi d'uso l'`ILIKE` (§3.8–3.9) è comunque sufficiente.

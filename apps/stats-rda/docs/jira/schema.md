# Schema del database `arak` — data dictionary

Documento di riferimento sul modello dati. Spiega ogni tabella, le colonne chiave,
il significato dei campi e quali sono i limiti noti. È complementare a
[INDEX.md](INDEX.md).

---

## 1. Modello generale

Schema PostgreSQL **`pa`** all'interno del database `arak`. Tutte le tabelle
usano lo stesso perno: l'**id dell'ordine** (`issue_id`, `bigint`) che corrisponde
a `pa.issue.id`. La chiave di lettura per l'umano è `issue_key` (`PA-1821`).

| Tabella | Grana (1 riga =) | Volume | Scopo |
|---|---|---|---|
| `pa.issue` | **1 ordine** | 8.756 | Tabella flat: campi anagrafici + 53 campi specifici del dominio, in colonna, con valori già leggibili. **Punto di partenza di quasi tutte le query.** |
| `pa.line_item` | 1 riga di dettaglio | 7.500 | Righe di dettaglio dell'ordine, per tipo: Articoli / Merci / Servizi / Leasing. |
| `pa.issue_custom_value` | 1 valore di campo | 200.955 | Tutti i valori di tutti i campi, in forma normalizzata (EAV). Utile per i campi non in colonna o per valori multipli. |
| `pa.custom_field` | 1 campo | 60 | Catalogo dei campi: id, nome, tipo, `is_grid`, `promoted`. |
| `pa.change_group` | 1 evento di modifica | 125.674 | Gruppo di modifiche a un ordine (autore + data). |
| `pa.change_item` | 1 campo modificato | 191.444 | Dettaglio di ogni campo cambiato (vecchio/nuovo valore). |
| `pa.comment` | 1 commento | 1.808 | Commenti testuali degli ordini. |
| `pa.attachment` | 1 allegato | 8.151 | **Solo metadati** (nome, tipo, dimensione, autore, data). I file non sono nel DB. |
| `pa.issue_link` | 1 collegamento | 4.241 | Relazioni tra ordini (blocca / è bloccato da / correlato…). |
| `pa.extract_run` | 1 caricamento | n | Audit interno del dataset (metadati di caricamento, conteggi). Generalmente non rilevante per l'analisi. |

Indici principali (già presenti): `issue(issue_key)`, `issue_custom_value(issue_id)`,
`issue_custom_value(field_id)`, `line_item(issue_id)`, `comment(issue_id)`,
`change_group(issue_id)`, `change_item(issue_id)`, `change_item(group_id)`,
`attachment(issue_id)`, `issue_link(source_id)`, `issue_link(destination_id)`.

---

## 2. `pa.issue` — la tabella centrale

Una riga per ogni ordine. Contiene:

1. i **campi anagrafici** (id, chiavi, stato, date, persone, summary, description…);
2. i **53 campi specifici del dominio**, in colonna, con valori **già leggibili**
   (etichette per le scelte, nomi per le persone, numeri per gli importi, date).

### 2.1 Campi anagrafici (sempre presenti)

| Colonna | Tipo | Significato |
|---|---|---|
| `id` | bigint PK | identificativo interno |
| `issue_key` | text | chiave leggibile (`PA-1821`) |
| `project` | text | sempre `PA` |
| `issue_type` | text | tipo ordine (macro): Acquisto, Servizio, Merce, Richiesta di aumento budget, Fornitore da qualificare, Articoli in vendita, Ordine Leasing, Invia modulo Leasing |
| `status` | text | stato: Closed, Annullato, Non approvato, Articoli da evadere, In attesa merce, Attesa Erogazione, Approvato… |
| `priority` | text | priorità (Normale, Alta…) |
| `resolution` | text | esito (Done, Won't Fix…) |
| `summary` | text | oggetto/titolo dell'ordine |
| `description` | text | descrizione estesa |
| `reporter_key` / `reporter_name` / `reporter_email` | text | **richiedente** (chi apre) — nome 100% popolato |
| `assignee_key` / `assignee_name` / `assignee_email` | text | assegnatario (97% popolato) |
| `creator_key` / `creator_name` | text | creatore materiale |
| `created` / `updated` | timestamptz | creazione / ultimo aggiornamento |
| `due_date` | date | scadenza |
| `resolution_date` | timestamptz | data risoluzione |
| `votes` / `watches` | integer | voti / osservatori (raramente usati) |
| `time_original_estimate` / `time_estimate` / `time_spent` | bigint | stime/tempo (non usati in questo dominio) |

### 2.2 I 53 campi specifici del dominio, in colonna

I nomi colonna derivano dal nome del campo, in snake_case (es. "Budget di
riferimento" → `budget_di_riferimento`). Sono raggruppati qui per categoria; tra
parentesi la **densità di valorizzazione** sui 8.756 ordini.

#### Identificativi e tipo di ordine
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `numero_ordine` | text | ~99% | numero ordine (`PA/1821-2021`) |
| `tipo_di_ordine` | text | 92% | Standard (con invio ordine) / E-commerce |
| `tipo_documento` | text | — | famiglia documento (TSF-ORDINE, OA-ITA…) |
| `ordine_di_vendita_cdlan` | text | **2%** | codice ordine di vendita collegato (sparse) |
| `ordine_di_vendita_cdlan_old` | text | — | come sopra, versione precedente |
| `data_attivazione_servizio` | date | — | data attivazione (per servizi/leasing) |
| `ricorrente` | text | — | indica ordine ricorrente |
| `ufficio_acquisti` | text | ~21% | flag **Sì/No**, sparse (non è una dimensione) |

#### Stato e approvazione
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `stato` | text | — | stato di dominio (es. APPROVATO, NON APPROVATO) — distinto da `status` |
| `inviato_in_approvazione` | timestamptz | — | data invio ad approvazione |
| `approvato` | timestamptz | — | data approvazione |
| `causa_non_approvazione` | text | — | motivazione di respinta |
| `limite_approvazione` | numeric | 49% | soglia di approvazione (€) |
| `percentuale_approvazione` | numeric | 47% | percentuale di approvazione |
| `motivazione_chiusura_con_riserva` | text | — | nota di chiusura con riserva |

#### Importi e valuta
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `importo_totale` | numeric | **99%** | importo complessivo dell'ordine |
| `importo_totale_merci` | numeric | — | componente merci |
| `importo_totale_servizi` | numeric | — | componente servizi |
| `importo_totale_leasing` | numeric | — | componente leasing |
| `valuta` | text | ~99% | Euro €, Dollaro $… |
| `limite_leasing` | numeric | — | soglia leasing |

#### Budget — **la dimensione chiave per i report**
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `budget_di_riferimento` | text | **88%** | **nome del budget / area**: DC, COLO, Communication, PEOPLE, Cloud, Office Management, AFC, TLC, Fiber, Technology, CDLAN Experience… |
| `budget_corrente` | numeric | 51% | spesa corrente sul budget |
| `budget_totale` | numeric | 50% | totale del budget disponibile (€) |
| `incremento_budget` | numeric | 3% | aumento richiesto (solo ordini "Richiesta aumento budget") |
| `richiesta_aumento_budget` | text | — | testo della richiesta di aumento |
| `errore_aumento_budget` | text | — | diagnostica (uso interno) |

#### Fornitore
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `fornitore_selezionato` | text | ~50% | **ragione sociale** del fornitore scelto |
| `fornitore` | text | — | fornitore |
| `altro_fornitore` | text | — | fornitore aggiuntivo |
| `fornitore_leasing` | text | — | fornitore per leasing |
| `riferimento_offerta_fornitore` | text | — | riferimento offerta |
| `riferimento_contratto_banca` | text | — | riferimento contratto / banca |

#### Persone
| Colonna | Tipo | Densità | Significato |
|---|---|---|---|
| `personale_richiedente` | text | ~1% | persona richiedente (sparse — usare `reporter_name`) |
| `amministrazione_richiedente` | text | ~2% | amministrazione richiedente (sparse) |

#### Note e testo libero
| Colonna | Tipo | Significato |
|---|---|---|
| `note_interne` | text | note interne |
| `note_per_il_fornitore` | text | note destinate al fornitore |
| `guida_sugli_articoli` | text | guida compilazione articoli |
| `articoli_old` | text | righe d'ordine nel vecchio formato testuale |

#### Qualificazione fornitore (tipo "Fornitore da qualificare")
| Colonna | Tipo | Significato |
|---|---|---|
| `nome` / `cognome` / `email` / `numero_di_telefono` / `lingua` | text | anagrafica del fornitore in fase di qualifica |

#### Flag e campi tecnici (raramente utili per i report)
`originid`, `issue_key_for_vendita`, `file_caricati_in_precedenza`, `is_impianti`,
`merceinvenditaflag`, `altro_fornitore_flag`, `problema_merce`, `errore`,
`numero_allegati`. Sono stati conservati per completezza ma in genere non servono
ai report.

---

## 3. `pa.line_item` — righe di dettaglio dell'ordine

Una riga per ogni voce di dettaglio di un ordine. La colonna `grid` distingue i
quattro tipi compilati: **Articoli**, **Merci**, **Servizi**, **Leasing**.

> ⚠️ **Importante — colonne popolate diverse per tipo.** Non tutte le colonne hanno
> senso per ogni `grid`. Tabella di popolamento reale (su 7.500 righe):

| Colonna | Articoli (1.033) | Merci (2.924) | Servizi (3.484) | Leasing (59) |
|---|:---:|:---:|:---:|:---:|
| `articolo_name` | ✅ | ✅ | ✅ | — |
| `vendor` | ✅ | ✅ | — | — |
| `part_number` | ✅ | ✅ | — | — |
| `descrizione` | ✅ | ✅ | ✅ | ✅ |
| `quantita` | ✅ | ✅ | ✅ | — |
| `importo` | ✅ | ✅ | ✅ | ✅ |
| **`prezzo_totale`** | ❌ **sempre vuoto** | ✅ | ✅ | — |
| `pagamento` / `pagamento_name` | — | — | ✅ | — |
| `durata` / `durata_name` | ✅ | — | ✅ | ✅ |
| `riferimento_importo(_name)` | ✅ | — | — | — |
| `vendita` | ✅ | — | — | — |
| `rinnovo` | ✅ | — | — | — |
| `riga(_name)` / `durata_totale_leasing` | — | — | — | ✅ |

**Conseguenza pratica**: per i totali, su **Articoli** non c'è `prezzo_totale` →
usare `quantita × importo` oppure il campo `importo_totale_merci` a livello di
ordine. Merci, Servizi e Leasing hanno invece l'`importo` diretto.

Colonne: `id, issue_id, issue_key, grid, row_no, articolo, articolo_name, vendor,
part_number, descrizione, quantita, importo, prezzo_totale, vendita,
riferimento_importo, riferimento_importo_name, rinnovo, durata, durata_name,
pagamento, pagamento_name, canone_anticipato, data_pagamento, riga, riga_name,
durata_totale_leasing, legenda, modified`.

---

## 4. `pa.issue_custom_value` — valori in forma normalizzata (EAV)

Tabella "verticale" con **tutti** i valori di **tutti** i 60 campi (anche quelli
presenti come colonna in `pa.issue`). Per ogni valore memorizza:

| Colonna | Contenuto |
|---|---|
| `issue_id`, `field_id`, `field_name`, `field_type` | riferimenti e tipo |
| `value_text` | valore **leggibile** (etichetta/testo) |
| `value_number` | valore numerico (per i campi numerici) |
| `value_date` | valore data |
| `value_raw` | valore **grezzo** (forma originale, non elaborata) |
| `seq` | progressivo per valori multipli dello stesso campo su un ordine |

**Quando usare la tabella flat (`pa.issue`) vs questa EAV:**
- **flat** per la maggior parte dei report: più rapida, una riga per ordine.
- **EAV** per: campi **non in colonna** (le griglie), valori **multipli** di un
  campo (`seq > 1`), ricostruire il **valore grezzo**, elencare **tutti** i campi
  compilati su un ordine.

Esempio — tutti i valori di un ordine, in forma chiave→valore:
```sql
SELECT field_name, value_text
FROM pa.issue_custom_value
WHERE issue_id = (SELECT id FROM pa.issue WHERE issue_key='PA-1821')
ORDER BY field_name;
```

---

## 5. `pa.custom_field` — catalogo

Catalogo dei 60 campi. Colonne utili: `field_id`, `field_name`, `field_type`
(tipo del campo: text, number, date, select, user, grid, …), `is_grid`, `promoted`
(più alcune colonne tecniche residue, non rilevanti per l'analisi).

> `promoted = true` significa che il campo è presente **anche come colonna** in
> `pa.issue` (con il valore già leggibile).

```sql
SELECT field_name, field_type, promoted FROM pa.custom_field
 WHERE promoted ORDER BY field_name;
```

---

## 6. Storico, commenti, allegati, collegamenti

### `pa.change_group` + `pa.change_item` (storico completo)
Ogni modifica a un ordine è un `change_group` (autore `author`/`author_name` +
`created`); ogni campo cambiato è un `change_item` legato al gruppo via `group_id`.
La colonna `field_type` categorizza il campo (`custom` per i campi specifici del
dominio); `field` riporta il nome del campo.

```sql
SELECT to_char(g.created,'YYYY-MM-DD HH24:MI') AS quando, g.author_name,
       i.field, i.old_string AS vecchio, i.new_string AS nuovo
FROM pa.change_group g JOIN pa.change_item i ON i.group_id = g.id
WHERE g.issue_key = 'PA-1821'
ORDER BY g.created, i.id;
```

### `pa.comment`
Commenti testuali (`actionbody`) con autore (`author_name`) e date (`created`,
`updated`). Sono 1.808 in totale: non tutti gli ordini ne hanno.

### `pa.attachment` — **solo metadati**
Per ogni allegato: `filename`, `mimetype`, `filesize`, `created`, `author(_name)`.
**I file binari non sono inclusi nel database**: la tabella contiene esclusivamente
i metadati descrittivi.

### `pa.issue_link`
Relazioni tra ordini: `source_key`/`destination_key`, `link_name` (es. "Blocks"),
`inward`/`outward` (etichette direzionali).

### `pa.extract_run`
Tabella di audit interno del dataset (`run_id`, `started_at`, filtri, conteggi).
Generalmente non rilevante per l'analisi.

---

## 7. Insidie e limiti

1. **Archivio storico.** I dati rappresentano una fotografia del progetto PA e non
   vengono aggiornati automaticamente: va trattato come archivio, non come sistema
   live.
2. **Allegati: solo metadati.** I file binari non sono inclusi nel database.
3. **`prezzo_totale` vuoto per la griglia Articoli.** Vedi sezione 3.
4. **Griglie mai compilate.** Dei campi di tipo griglia, solo 4 contengono dati
   (Articoli, Merci, Servizi, Leasing). Le altre ("Articoli in vendita", "Merci da
   lavorare", "Servizi da lavorare") sono vuote.
5. **Dimensione "centro di costo" assente.** Non esiste un campo centro di costo.
   La dimensione più prossima, e quella consigliata per i report, è
   **`budget_di_riferimento`** (area/budget: DC, COLO, Technology…). Per raggruppare
   per "team richiedente" usare `reporter_name`. Se servirà una mappatura
   reporter→team o budget→centro di costo, andrà fornita come tabella esterna.
6. **Valute miste.** La maggior parte degli ordini è in Euro, ma ci sono valute
   diverse (Dollaro…). Aggregare `importo_totale` senza filtrare `valuta` può
   mescolare importi non confrontabili.
7. **Valori in forma leggibile.** Le colonne e `value_text` contengono i valori già
   nella forma visualizzata (etichette, nomi, importi). Per la forma grezza usare
   `pa.issue_custom_value.value_raw`.
8. **Campi sparsi.** Alcuni campi (es. `personale_richiedente`,
   `ordine_di_vendita_cdlan`) sono valorizzati su una minoranza degli ordini: non
   sono affidabili come dimensioni di raggruppamento generale.

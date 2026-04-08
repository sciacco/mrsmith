# Schema MISTRA

Database PostgreSQL relativo a Mistra, documentato tramite dump per-schema in formato JSON.

## Introduzione

La directory `docs/mistradb/` raccoglie la documentazione tecnica degli schemi PostgreSQL di Mistra. Ogni file `mistra_{SCHEMA}.json` contiene un singolo schema con metadati, tabelle, viste, funzioni, indici e foreign key utili per analisi, integrazioni backend e pianificazione delle mini-app che leggono o aggiornano dati Mistra.

Questi file descrivono il database applicativo, mentre `docs/mistra-dist.yaml` resta il riferimento separato per le API Mistra NG Internal.

## Indice degli schemi

| #  | Schema      | Tabelle | Viste | Funzioni | Ambito principale                                 | File |
| -- | ----------- | ------- | ----- | -------- | ------------------------------------------------- | ---- |
| 1  | accounting  | 1       | 0     | 1        | Log contabili e audit trail applicativo           | [mistra_accounting.json](mistra_accounting.json) |
| 2  | cart        | 11      | 0     | 4        | Carrello, righe ordine e stati operativi          | [mistra_cart.json](mistra_cart.json) |
| 3  | common      | 4       | 0     | 11       | Traduzioni, vocabolari e utility condivise        | [mistra_common.json](mistra_common.json) |
| 4  | customers   | 24      | 0     | 105      | Clienti, contatti, gruppi, crediti e metadata ACL | [mistra_customers.json](mistra_customers.json) |
| 5  | loader      | 22      | 8     | 3        | Staging/import da ERP e dataset esterni           | [mistra_loader.json](mistra_loader.json) |
| 6  | migrations  | 1       | 0     | 0        | Tracciamento migrazioni database                  | [mistra_migrations.json](mistra_migrations.json) |
| 7  | orders      | 6       | 0     | 5        | Ordini legacy, righe e stati ordine               | [mistra_orders.json](mistra_orders.json) |
| 8  | products    | 12      | 3     | 29       | Catalogo prodotti, kit e regole ecommerce         | [mistra_products.json](mistra_products.json) |
| 9  | public      | 1       | 0     | 5        | Facade pubblica e funzioni esposte                | [mistra_public.json](mistra_public.json) |
| 10 | quotes      | 5       | 2     | 6        | Preventivi, righe preventivo e template correlati | [mistra_quotes.json](mistra_quotes.json) |
| 11 | templates   | 6       | 0     | 0        | Template documentali, email e traduzioni          | [mistra_templates.json](mistra_templates.json) |
| 12 | users       | 11      | 2     | 67       | Utenti, ruoli, pagine, preferenze e accessi       | [mistra_users.json](mistra_users.json) |

## Note di lettura

- Gli schemi con maggiore logica lato database sono `customers`, `users` e `products`, che concentrano la maggior parte delle stored procedure.
- `loader` contiene tabelle e viste di appoggio per importazioni e raccordo con sistemi esterni come ERP e dataset provenienti da altri domini.
- `public` espone funzioni di accesso semplificato, mentre la logica applicativa principale e' distribuita negli schemi funzionali (`products`, `orders`, `quotes`, `customers`, `users`).
- Per analisi trasversali su traduzioni, numerazioni o utility shared conviene partire da `common`, che contiene funzioni riusate anche dagli altri schemi.

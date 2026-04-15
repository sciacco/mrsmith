# RDF Backend — Phase B: UX Pattern Map

Policy: **porting 1:1** del layout Appsmith. Nessuna riprogettazione UX.

## View: `Home`
- **Pattern:** pagina vuota (placeholder).
- **Contenuto:** nessuno (il portal mrsmith fornisce comunque la landing; la pagina può restare vuota o essere omessa senza perdita di funzionalità).

## View: `Fornitori`
- **Pattern:** master-table CRUD a pagina singola con modali per create/delete e form inline per l'edit, esattamente come Appsmith.
- **Intent utente:** listare, cercare, ordinare, paginare, creare, modificare, cancellare fornitori.
- **Sezioni UI (mappatura 1:1):**

  | Appsmith | Rewrite |
  |---|---|
  | `Text16` "Fornitori" | header con titolo "Fornitori" |
  | `refresh_btn` (icon refresh) | icon button → re-fetch lista |
  | `add_btn` (icon +) | icon button → apre modal create |
  | `data_table` (TABLE_WIDGET_V2, server-side pagination) | tabella con colonne `id`, `nome`, colonna azioni con bottone Delete; search, sort, paginazione server-side |
  | `Insert_Modal` + `insert_form` (solo campo `nome`) | modal con form `nome` (required); `id` non presente |
  | `update_form` (inline, visibile se `selectedRow.id`) | form inline accanto/sotto alla tabella, visibile solo con riga selezionata; unico campo editabile `nome` |
  | `Delete_Modal` ("Are you sure…", Cancel + Confirm) | modal di conferma delete con stessi testi |

- **Entry/exit:** unica route nella mini-app (`/fornitori`). Navigazione dal portal mrsmith via sidebar.
- **Feedback:** toast di errore su insert/update/delete (quello di delete era mancante in Appsmith, lo aggiungiamo come fix minimo).

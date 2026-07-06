# Piano implementazione — Riepilogo PA Jira

Piano operativo per implementare in `apps/stats-rda` la nuova pagina **Riepilogo PA Jira**, basato su `apps/stats-rda/docs/PRD-RIEPILOGO-PA-JIRA.md`.

## Obiettivo

Implementare in `apps/stats-rda` una nuova pagina **Riepilogo PA Jira** raggiungibile da:

```text
/riepilogo-pa
```

La pagina deve mostrare statistiche read-only sugli ordini PA storici da `pa.issue`, filtrati per periodo preset, aggregati per `budget_di_riferimento`, con grafico cliccabile, tabella dettagli e download Excel.

Repository target: `/Users/sciacco/devel/mrsmith`.

---

## Vincoli importanti

1. **Non aggiungere test** salvo approvazione esplicita dell'utente.
2. UI tutta in **italiano**.
3. Usare design system clean mini-app: CSS Modules, token, niente Tailwind.
4. Endpoint backend già sotto prefisso reale:

```text
/api/stats-rda/v1/pa
```

Non usare il prefisso del PRD senza `/v1`.

5. Autenticazione: usare `@mrsmith/api-client`, non `fetch` anonimo, soprattutto per export Excel.
6. Sorgente dati esclusiva: `pa.issue`.
7. Clausola base obbligatoria:

```sql
issue_type = 'Acquisto'
AND resolution NOT IN ('Annullato')
```

Nota: questa clausola esclude anche `resolution NULL`, come richiesto dal PRD.

---

## Stato repo rilevante

### Frontend esistente

- App: `apps/stats-rda`
- Routing: `apps/stats-rda/src/routes.tsx`
- Shell/nav: `apps/stats-rda/src/App.tsx`
- API client: `apps/stats-rda/src/api/client.ts`
- Query hooks: `apps/stats-rda/src/api/queries.ts`
- Types: `apps/stats-rda/src/api/types.ts`
- Pagina esistente: `apps/stats-rda/src/pages/ArchivioPaPage.tsx`

Attualmente nav:

```text
Archivio PA -> /archivio-pa
RDA -> /rda
```

Aggiungere:

```text
Riepilogo PA Jira -> /riepilogo-pa
```

### Backend esistente

Modulo:

```text
backend/internal/statsrda
```

File:

```text
backend/internal/statsrda/handler.go
backend/internal/statsrda/queries.go
backend/internal/statsrda/types.go
```

Route già registrate sotto:

```go
/stats-rda/v1/pa/...
```

Il frontend vede queste route come:

```text
/api/stats-rda/v1/pa/...
```

La libreria Excel `github.com/xuri/excelize/v2` è già presente in `backend/go.mod`.

---

# Fase 1 — Backend: contratto dati riepilogo

## 1.1 Aggiungere types Go

In `backend/internal/statsrda/types.go` aggiungere tipi dedicati:

```go
type PeriodPreset string

type RiepilogoPeriod struct {
    Preset string `json:"preset"`
    From   string `json:"from"`
    To     string `json:"to"`
}

type RiepilogoTotals struct {
    OrderCount  int     `json:"order_count"`
    BudgetCount int     `json:"budget_count"`
    Amount      float64 `json:"amount"`
}

type RiepilogoBudget struct {
    Budget     string  `json:"budget"`
    OrderCount int     `json:"order_count"`
    Amount     float64 `json:"amount"`
    Percentage float64 `json:"percentage"`
}

type RiepilogoDetail struct {
    IssueKey             string   `json:"issue_key"`
    NumeroOrdine         *string  `json:"numero_ordine"`
    Summary              string   `json:"summary"`
    BudgetDiRiferimento  string   `json:"budget_di_riferimento"`
    ImportoTotale        float64  `json:"importo_totale"`
    Valuta               *string  `json:"valuta"`
    ReporterName         *string  `json:"reporter_name"`
    FornitoreSelezionato *string  `json:"fornitore_selezionato"`
    Status               *string  `json:"status"`
    Resolution           *string  `json:"resolution"`
    Created              *string  `json:"created"`
}

type RiepilogoResponse struct {
    Period  RiepilogoPeriod   `json:"period"`
    Totals  RiepilogoTotals   `json:"totals"`
    Budgets []RiepilogoBudget `json:"budgets"`
    Details []RiepilogoDetail `json:"details"`
}
```

Usare sempre `"Senza budget"` per `NULL` o stringa vuota.

---

## 1.2 Preset periodo

Aggiungere helper backend, preferibilmente in nuovo file:

```text
backend/internal/statsrda/riepilogo.go
```

Preset accettati:

```text
this_month
previous_month
this_quarter
previous_quarter
current_year
previous_year
```

Default frontend: `previous_month`.

Calcolo:

- `from` incluso
- `to` escluso
- formato JSON `YYYY-MM-DD`
- usare `time.Now()` lato backend e timezone locale/backend coerente.

Implementare funzione tipo:

```go
func resolveRiepilogoPeriod(preset string, now time.Time) (from time.Time, to time.Time, normalizedPreset string, err error)
```

Se `period` mancante o non valido: `400`.

---

## 1.3 Query backend riepilogo

Aggiungere endpoint:

```go
GET /stats-rda/v1/pa/riepilogo?period=previous_month
```

Registrarlo in `RegisterRoutes`.

Query dettagliata: caricare integralmente il periodo, senza paginazione.

Base SQL:

```sql
FROM pa.issue i
WHERE i.issue_type = 'Acquisto'
  AND i.resolution NOT IN ('Annullato')
  AND i.created >= $1
  AND i.created < $2
```

Query budget:

```sql
SELECT
  CASE
    WHEN i.budget_di_riferimento IS NULL OR btrim(i.budget_di_riferimento) = ''
    THEN 'Senza budget'
    ELSE btrim(i.budget_di_riferimento)
  END AS budget,
  COUNT(*) AS order_count,
  COALESCE(SUM(COALESCE(i.importo_totale, 0)), 0) AS amount
FROM pa.issue i
WHERE i.issue_type = 'Acquisto'
  AND i.resolution NOT IN ('Annullato')
  AND i.created >= $1
  AND i.created < $2
GROUP BY 1
ORDER BY amount DESC, budget ASC
```

Query detail:

```sql
SELECT
  i.issue_key,
  i.numero_ordine,
  i.summary,
  CASE
    WHEN i.budget_di_riferimento IS NULL OR btrim(i.budget_di_riferimento) = ''
    THEN 'Senza budget'
    ELSE btrim(i.budget_di_riferimento)
  END AS budget_di_riferimento,
  COALESCE(i.importo_totale, 0) AS importo_totale,
  i.valuta,
  i.reporter_name,
  i.fornitore_selezionato,
  i.status,
  i.resolution,
  to_char(i.created, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created
FROM pa.issue i
WHERE i.issue_type = 'Acquisto'
  AND i.resolution NOT IN ('Annullato')
  AND i.created >= $1
  AND i.created < $2
ORDER BY i.created DESC, i.issue_key ASC
```

Calcolare in Go:

```text
totals.amount = somma importi budget
totals.order_count = len(details)
totals.budget_count = len(budgets)
budget.percentage = amount / totals.amount * 100
```

Se totale zero, percentuali a `0`.

---

# Fase 2 — Backend: export Excel

## 2.1 Endpoint

Aggiungere:

```go
GET /stats-rda/v1/pa/riepilogo/export?period=previous_month
```

Deve riusare la stessa funzione dati del riepilogo, così JSON ed Excel non divergono.

Response:

```http
Content-Type: application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
Content-Disposition: attachment; filename="riepilogo-pa-jira_previous_month_2026-06-01_2026-07-01.xlsx"
```

## 2.2 Generazione Excel

Usare `github.com/xuri/excelize/v2`.

Sheet 1:

```text
Totali budget
```

Colonne:

```text
Budget
Numero ordini
Importo totale
Valuta
Percentuale sul totale periodo
```

Per `Valuta` usare valore statico/pragmatico:

```text
Importi aggregati senza conversione valuta
```

oppure lasciare vuoto se si preferisce evitare ambiguità. Il PRD dice “Valuta, se applicabile”; qui gli importi sono sommati come euro senza conversione.

Sheet dettaglio per ogni budget:

- nome sheet = budget normalizzato
- massimo 31 caratteri
- rimuovere/caratteri vietati Excel: `: \ / ? * [ ]`
- se duplicati dopo troncamento, aggiungere suffisso numerico
- `Senza budget` per budget nullo/vuoto

Colonne dettaglio:

```text
Issue key
Numero ordine
Summary
Budget di riferimento
Importo totale
Valuta
Richiedente
Fornitore selezionato
Status
Resolution
Created
```

Importante: export sempre tutti i dettagli del periodo, ignorando il filtro budget UI.

---

# Fase 3 — Frontend: tipi e API hooks

## 3.1 Types

In `apps/stats-rda/src/api/types.ts` aggiungere tipi equivalenti:

```ts
export type PeriodPreset =
  | 'this_month'
  | 'previous_month'
  | 'this_quarter'
  | 'previous_quarter'
  | 'current_year'
  | 'previous_year';

export interface RiepilogoPeriod {
  preset: PeriodPreset;
  from: string;
  to: string;
}

export interface RiepilogoTotals {
  order_count: number;
  budget_count: number;
  amount: number;
}

export interface RiepilogoBudget {
  budget: string;
  order_count: number;
  amount: number;
  percentage: number;
}

export interface RiepilogoDetail {
  issue_key: string;
  numero_ordine: string | null;
  summary: string;
  budget_di_riferimento: string;
  importo_totale: number;
  valuta: string | null;
  reporter_name: string | null;
  fornitore_selezionato: string | null;
  status: string | null;
  resolution: string | null;
  created: string | null;
}

export interface RiepilogoResponse {
  period: RiepilogoPeriod;
  totals: RiepilogoTotals;
  budgets: RiepilogoBudget[];
  details: RiepilogoDetail[];
}
```

## 3.2 Query hooks

In `apps/stats-rda/src/api/queries.ts` aggiungere:

```ts
export function useRiepilogoPa(period: PeriodPreset) {
  const api = useApiClient();
  return useQuery({
    queryKey: ['stats-rda', 'riepilogo-pa', period],
    queryFn: () => api.get<RiepilogoResponse>(
      `${ROOT}/riepilogo${buildSearch({ period })}`
    ),
    placeholderData: (prev) => prev,
  });
}
```

Aggiungere helper export autenticato. Usare `api.getBlob`.

```ts
export async function downloadRiepilogoPaExcel(api: ApiClient, period: PeriodPreset) {
  const blob = await api.getBlob(`${ROOT}/riepilogo/export${buildSearch({ period })}`);
  // creare Object URL, anchor temporaneo, click, revokeObjectURL
}
```

Se `getBlob` non espone filename da header, costruirlo lato client con stesso schema:

```text
riepilogo-pa-jira_<period>_<from>_<to>.xlsx
```

Meglio: nella pagina usare `data.period.from/to` per il nome file.

---

# Fase 4 — Frontend: routing e nav

## 4.1 Route

In `apps/stats-rda/src/routes.tsx`:

```ts
import { RiepilogoPaPage } from './pages/RiepilogoPaPage';

{ path: 'riepilogo-pa', element: <RiepilogoPaPage /> },
```

Non cambiare redirect index, salvo richiesta. Lasciare index verso `/archivio-pa`.

## 4.2 Nav

In `apps/stats-rda/src/App.tsx` aggiungere:

```ts
{ label: 'Riepilogo PA Jira', path: '/riepilogo-pa' },
```

Posizione suggerita:

```text
Archivio PA | Riepilogo PA Jira | RDA
```

---

# Fase 5 — Frontend: pagina `RiepilogoPaPage`

Creare:

```text
apps/stats-rda/src/pages/RiepilogoPaPage.tsx
apps/stats-rda/src/pages/RiepilogoPaPage.module.css
```

## 5.1 Stato pagina

Usare URL search params per il periodo:

```text
/riepilogo-pa?period=previous_month
```

Se assente, default `previous_month`.

Stato React:

```ts
const [selectedBudget, setSelectedBudget] = useState<string | null>(null);
```

Quando cambia periodo, resettare `selectedBudget`.

## 5.2 Preset periodo

Opzioni UI:

```ts
[
  { value: 'this_month', label: 'Mese corrente' },
  { value: 'previous_month', label: 'Mese precedente' },
  { value: 'this_quarter', label: 'Trimestre corrente' },
  { value: 'previous_quarter', label: 'Trimestre precedente' },
  { value: 'current_year', label: 'Anno corrente' },
  { value: 'previous_year', label: 'Anno precedente' },
]
```

Usare `SingleSelect` se supporta stringhe; altrimenti `<select>` stilizzato localmente.

## 5.3 Header e toolbar

Header:

```text
Titolo: Riepilogo PA Jira
Sottotitolo: Totali degli ordini di acquisto PA per budget nel periodo selezionato.
```

Toolbar:

- select periodo
- range calcolato, ad esempio:

```text
01/06/2026 – 01/07/2026 escluso
```

- bottone:

```text
Scarica Excel
```

Stati export:

```text
Preparazione file Excel…
Non è stato possibile generare il file Excel.
```

## 5.4 KPI riepilogo

Mostrare tre card:

```text
Totale periodo
Ordini inclusi
Budget trovati
```

Formato importi:

- usare `Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' })`
- coerente con PRD: sommare come euro, senza conversione.

## 5.5 Grafico

Preferire barre orizzontali custom CSS invece di introdurre librerie chart.

Ogni riga:

- budget
- barra proporzionale all'importo massimo o al totale
- importo
- percentuale
- numero ordini

Interazione:

```ts
onClick={() => setSelectedBudget(budget.budget)}
```

Evidenziare budget selezionato.

Aggiungere bottone:

```text
Mostra tutti
```

visibile quando `selectedBudget !== null`.

Tooltip semplice via `title` o contenuto visibile:

```text
Cloud — 45.000,00 € — 36,45%
```

## 5.6 Lista dettagli

Filtrare client-side:

```ts
const visibleDetails = selectedBudget
  ? data.details.filter(d => d.budget_di_riferimento === selectedBudget)
  : data.details;
```

Campi tabella:

```text
Ordine
Oggetto
Budget
Importo totale
Valuta
Richiedente
Fornitore
Stato
Resolution
Creato
```

Per ordine mostrare:

```text
issue_key
numero_ordine secondario se presente
```

Se `details.length === 0`:

```text
Nessun ordine di acquisto trovato per il periodo selezionato.
```

Se filtro budget attivo e nessun dettaglio, mostrare comunque empty non errore.

## 5.7 Stati

- Loading: `Caricamento riepilogo PA…`
- Empty: `Nessun ordine di acquisto trovato per il periodo selezionato.`
- Error: `Non è stato possibile caricare il riepilogo PA.`
- Export loading: `Preparazione file Excel…`
- Export error: `Non è stato possibile generare il file Excel.`

Per 503 usare pattern `ServiceUnavailable` se già disponibile.

---

# Fase 6 — Fix trasporto autenticato se necessario

Nel codice esistente `ArchivioPaPage.tsx` ci sono autocomplete con `fetch` anonimo:

```ts
fetch('/api/stats-rda/v1/pa/fornitori...')
```

Questa non è parte diretta del PRD, ma per coerenza auth può fallire. È stata aperta issue GitHub dedicata:

```text
https://github.com/sciacco/mrsmith/issues/64
```

Per Riepilogo PA Jira non introdurre altri `fetch` anonimi. Per export Excel usare necessariamente `ApiClient.getBlob`.

---

# Fase 7 — CSS / UX

Seguire `docs/UI-UX.md`.

Stile:

- pagina con className globale `statsRdaPage`
- card/superfici con token CSS
- layout responsive:
  - desktop: toolbar + KPI, grafico e tabella
  - mobile: stack verticale
- barre cliccabili con focus visibile
- niente colori hardcoded se esiste token
- animazioni leggere e rispettare `prefers-reduced-motion`

File CSS module deve coprire:

```text
.page
.header
.toolbar
.summaryGrid
.metricCard
.chartCard
.barList
.barButton
.barButtonSelected
.detailsCard
.table
.emptyState
.errorState
```

---

# Fase 8 — Verifica manuale

Eseguire almeno:

```bash
pnpm --filter mrsmith-stats-rda lint
pnpm --filter mrsmith-stats-rda build
cd backend && go test ./internal/statsrda
```

Nota: se `go test` richiede DB o fallisce per assenza ambiente, riportare chiaramente il motivo. Non aggiungere nuovi test senza approvazione.

Verifiche funzionali manuali:

1. `/riepilogo-pa` mostra `Mese precedente`.
2. Cambio preset aggiorna dati.
3. Budget null/vuoto appare come `Senza budget`.
4. Click su barra filtra la tabella.
5. `Mostra tutti` rimuove filtro.
6. Export scarica `.xlsx` con:
   - sheet `Totali budget`
   - uno sheet per budget
7. Export contiene tutti i budget anche se UI ha filtro attivo.
8. Empty state non è errore.
9. UI tutta in italiano.
10. Network: export passa con Bearer auth.

---

# File attesi modificati/creati

Backend:

```text
backend/internal/statsrda/handler.go
backend/internal/statsrda/types.go
backend/internal/statsrda/riepilogo.go
```

Frontend:

```text
apps/stats-rda/src/App.tsx
apps/stats-rda/src/routes.tsx
apps/stats-rda/src/api/types.ts
apps/stats-rda/src/api/queries.ts
apps/stats-rda/src/pages/RiepilogoPaPage.tsx
apps/stats-rda/src/pages/RiepilogoPaPage.module.css
```

Possibile, solo se serve:

```text
apps/stats-rda/src/lib/format.ts
```

per formattazione importi/date condivisa.

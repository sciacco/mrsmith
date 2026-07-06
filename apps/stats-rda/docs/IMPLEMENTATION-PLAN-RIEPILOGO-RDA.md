# Implementation Plan — Riepilogo RDA

Documento operativo per un agent LLM incaricato di implementare la pagina **Riepilogo RDA** descritta in [`PRD-RIEPILOGO-RDA.md`](./PRD-RIEPILOGO-RDA.md).

## Regole di esecuzione

- Leggere prima:
  - `apps/stats-rda/docs/PRD-RIEPILOGO-RDA.md`
  - `docs/UI-UX.md`
  - `apps/stats-rda/src/pages/RiepilogoPaPage.tsx`
  - `apps/stats-rda/src/pages/RiepilogoPaPage.module.css`
  - `backend/internal/statsrda/riepilogo.go`
- Non aggiungere test automatici senza approvazione esplicita.
- UI tutta in italiano.
- Usare CSS Modules/design system, no Tailwind.
- Usare `@mrsmith/api-client` / `ApiClient.getBlob`, non `fetch` anonimo.
- Backend sotto prefisso reale `/api/stats-rda/v1` lato frontend; nel mux Go registrare route senza `/api`, come già avviene per PA (`/stats-rda/v1/...`).
- Sorgente dati RDA: `rda.purchase_order`, con join di arricchimento indicate nel PRD.
- La nuova pagina deve essere simile a **Riepilogo PA Jira**, ma endpoint, tipi e query devono restare separati.

---

## Stato attuale rilevante

### Frontend

File principali:

- `apps/stats-rda/src/App.tsx`
  - contiene `navItems` con `Archivio PA`, `Riepilogo PA Jira`, `RDA`.
- `apps/stats-rda/src/routes.tsx`
  - route esistenti:
    - `/archivio-pa`
    - `/riepilogo-pa`
    - `/rda` placeholder
- `apps/stats-rda/src/api/queries.ts`
  - `ROOT = '/stats-rda/v1/pa'`
  - contiene `useRiepilogoPa(...)` e `downloadRiepilogoPaExcel(...)`
- `apps/stats-rda/src/api/types.ts`
  - contiene tipi PA per riepilogo:
    - `RiepilogoPeriodSelection`
    - `RiepilogoPeriod`
    - `RiepilogoTotals`
    - `RiepilogoBudget`
    - `RiepilogoDetail`
    - `RiepilogoResponse`
- `apps/stats-rda/src/pages/RiepilogoPaPage.tsx`
  - riferimento UI/UX da riusare per RDA.

### Backend

File principali:

- `backend/internal/statsrda/handler.go`
  - registra le route PA.
- `backend/internal/statsrda/types.go`
  - contiene tipi JSON PA.
- `backend/internal/statsrda/riepilogo.go`
  - contiene helper periodo, query PA, JSON ed export Excel.

---

# Fase 1 — Backend: tipi e periodo

## 1.1 Tipi Go

In `backend/internal/statsrda/types.go` aggiungere tipi dedicati RDA, evitando di riusare nomi PA per non creare ambiguità.

Suggerimento:

```go
type RiepilogoRDABudget struct {
    BudgetKey  string   `json:"budget_key"`
    Budget     string   `json:"budget"`
    BudgetID   *int64   `json:"budget_id"`
    BudgetName *string  `json:"budget_name"`
    BudgetYear *int     `json:"budget_year"`
    OrderCount int      `json:"order_count"`
    Amount     float64  `json:"amount"`
    Percentage float64  `json:"percentage"`
}

type RiepilogoRDADetail struct {
    Code        string   `json:"code"`
    Project     *string  `json:"project"`
    Object      *string  `json:"object"`
    Requester   *string  `json:"requester"`
    BudgetID    *int64   `json:"budget_id"`
    BudgetName  *string  `json:"budget_name"`
    BudgetYear  *int     `json:"budget_year"`
    CostCenter  *string  `json:"cost_center"`
    Currency    *string  `json:"currency"`
    TotalPrice  float64  `json:"total_price"`
    State       *string  `json:"state"`
    Created     *string  `json:"created"`
    CompanyName *string  `json:"company_name"`
}

type RiepilogoRDAResponse struct {
    Period  RiepilogoPeriod      `json:"period"`
    Totals  RiepilogoTotals      `json:"totals"`
    Budgets []RiepilogoRDABudget `json:"budgets"`
    Details []RiepilogoRDADetail `json:"details"`
}
```

Note:

- `RiepilogoPeriod` e `RiepilogoTotals` possono essere riusati.
- `TotalPrice` deve essere non-null nel JSON: usare `COALESCE(po.total_price, 0)` in SQL.

## 1.2 Period resolver

`backend/internal/statsrda/riepilogo.go` contiene già helper per preset + `custom`:

- `resolveRiepilogoPeriod(...)`
- `resolveRiepilogoRequestPeriod(...)`

Riutilizzarli per RDA. Non duplicare se non necessario.

---

# Fase 2 — Backend: query e handler RDA

## 2.1 Nuovo file backend

Creare:

```text
backend/internal/statsrda/riepilogo_rda.go
```

Responsabilità:

- query aggregata budget RDA;
- query dettagli RDA;
- `handleRiepilogoRDA`;
- `handleRiepilogoRDAExport`;
- `loadRiepilogoRDA`;
- funzioni helper per export Excel RDA.

## 2.2 Allowlist stati RDA

Definire una costante SQL esplicita nella clausola `IN`, come nel PRD:

```sql
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
```

Per questa prima versione va bene tenerli direttamente nella query SQL statica.

## 2.3 Query aggregata

Implementare una query con parametri `$1 = from`, `$2 = to`.

Schema consigliato:

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
  po.budget_id,
  b."name" AS budget_name,
  b."year" AS budget_year,
  COUNT(*) AS order_count,
  COALESCE(SUM(COALESCE(po.total_price, 0)), 0) AS amount
FROM rda.purchase_order po
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND po.state IN (...allowlist...)
  AND po.created >= $1
  AND po.created < $2
GROUP BY budget_key, budget_label, po.budget_id, b."name", b."year"
ORDER BY amount DESC, budget_label ASC
```

Dopo la lettura righe, calcolare `percentage` lato Go:

```go
percentage = 0
if totals.Amount > 0 {
    percentage = budget.Amount / totals.Amount * 100
}
```

`totals.BudgetCount = len(budgets)`.

## 2.4 Query dettagli

Implementare una query dettagli con stessi filtri e parametri.

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
  COALESCE(po.total_price, 0) AS total_price,
  po.state,
  to_char(po.created, 'YYYY-MM-DD"T"HH24:MI:SSOF') AS created,
  f.company_name
FROM rda.purchase_order po
LEFT JOIN provider_qualifications.provider f ON po.provider_id = f.id
LEFT JOIN users_int."user" u ON po.requester_id = u.id
LEFT JOIN budgets.budget b ON po.budget_id = b.id
WHERE po.deleted IS NULL
  AND po.state IN (...allowlist...)
  AND po.created >= $1
  AND po.created < $2
ORDER BY po.created DESC, po.code ASC
```

Nota: se `po.code` potesse essere nullable nel DB, usare `COALESCE(po.code, '')`; altrimenti scansionare in `string`.

## 2.5 Handler JSON

In `riepilogo_rda.go`:

```go
func (h *Handler) handleRiepilogoRDA(w http.ResponseWriter, r *http.Request) {
    if !h.requireDB(w) { return }

    query := r.URL.Query()
    from, to, preset, err := resolveRiepilogoRequestPeriod(query.Get("period"), query.Get("from"), query.Get("to"), time.Now())
    if err != nil {
        badRequest(w, err.Error())
        return
    }

    resp, err := h.loadRiepilogoRDA(r, from, to, preset)
    if err != nil {
        httputil.InternalError(w, r, err, "statsrda riepilogo rda query failed")
        return
    }

    httputil.JSON(w, http.StatusOK, resp)
}
```

## 2.6 RegisterRoutes

In `backend/internal/statsrda/handler.go` aggiungere:

```go
handle("GET /stats-rda/v1/rda/riepilogo", h.handleRiepilogoRDA)
handle("GET /stats-rda/v1/rda/riepilogo/export", h.handleRiepilogoRDAExport)
```

Non rimuovere la route placeholder frontend `/rda` se non richiesto.

---

# Fase 3 — Backend: export Excel RDA

Usare la stessa libreria e stile di `handleRiepilogoExport` in `riepilogo.go`.

## 3.1 Nome file

Schema:

```text
riepilogo-rda_<preset>_<from>_<to>.xlsx
```

Usare `Content-Disposition: attachment`.

## 3.2 Sheet `Totali budget`

Colonne:

1. Budget
2. Anno budget
3. Numero ordini
4. Importo totale
5. Percentuale sul totale periodo

## 3.3 Sheet per budget

Un foglio per budget, con nome sanificato Excel. Riutilizzare/splittare helper esistenti in `riepilogo.go` se disponibili, altrimenti duplicare con nomi specifici.

Colonne:

1. Codice ordine
2. Progetto
3. Oggetto
4. Budget
5. Anno budget
6. Centro di costo
7. Importo totale
8. Valuta
9. Richiedente
10. Fornitore
11. Stato
12. Data creazione

L'export deve includere tutti i dettagli del periodo, ignorando il filtro budget UI.

---

# Fase 4 — Frontend: tipi e API client

## 4.1 Tipi TypeScript

In `apps/stats-rda/src/api/types.ts` aggiungere tipi dedicati RDA:

```ts
export interface RiepilogoRdaBudget {
  budget_key: string;
  budget: string;
  budget_id: number | null;
  budget_name: string | null;
  budget_year: number | null;
  order_count: number;
  amount: number;
  percentage: number;
}

export interface RiepilogoRdaDetail {
  code: string;
  project: string | null;
  object: string | null;
  requester: string | null;
  budget_id: number | null;
  budget_name: string | null;
  budget_year: number | null;
  cost_center: string | null;
  currency: string | null;
  total_price: number;
  state: string | null;
  created: string | null;
  company_name: string | null;
}

export interface RiepilogoRdaResponse {
  period: RiepilogoPeriod;
  totals: RiepilogoTotals;
  budgets: RiepilogoRdaBudget[];
  details: RiepilogoRdaDetail[];
}
```

## 4.2 API hooks

In `apps/stats-rda/src/api/queries.ts`:

- non riusare `ROOT = '/stats-rda/v1/pa'` per RDA;
- aggiungere:

```ts
const PA_ROOT = '/stats-rda/v1/pa';
const RDA_ROOT = '/stats-rda/v1/rda';
```

Rinominare gli usi PA di `ROOT` in `PA_ROOT`.

Aggiungere:

```ts
export interface RiepilogoRdaParams {
  period: RiepilogoPeriodSelection;
  from?: string;
  to?: string;
}

function riepilogoSearchParams(params: { period: RiepilogoPeriodSelection; from?: string; to?: string }) {
  return params.period === 'custom'
    ? { period: params.period, from: params.from, to: params.to }
    : { period: params.period };
}

export function useRiepilogoRda(params: RiepilogoRdaParams) {
  const api = useApiClient();
  const searchParams = riepilogoSearchParams(params);

  return useQuery({
    queryKey: ['stats-rda', 'riepilogo-rda', searchParams],
    queryFn: () => api.get<RiepilogoRdaResponse>(`${RDA_ROOT}/riepilogo${buildSearch(searchParams)}`),
    placeholderData: (prev) => prev,
  });
}

export async function downloadRiepilogoRdaExcel(
  api: ApiClient,
  params: RiepilogoRdaParams,
  filename = `riepilogo-rda_${params.period}.xlsx`,
) {
  const searchParams = riepilogoSearchParams(params);
  const blob = await api.getBlob(`${RDA_ROOT}/riepilogo/export${buildSearch(searchParams)}`);
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
```

Opzionale: far usare `riepilogoSearchParams` anche a PA per evitare duplicazione.

---

# Fase 5 — Frontend: pagina Riepilogo RDA

## 5.1 Nuovi file

Creare:

```text
apps/stats-rda/src/pages/RiepilogoRdaPage.tsx
apps/stats-rda/src/pages/RiepilogoRdaPage.module.css
```

Approccio consigliato:

- partire da `RiepilogoPaPage.tsx`;
- adattare testi, tipi e colonne;
- copiare il CSS module PA e rinominare solo se serve;
- mantenere UI e comportamento coerenti.

## 5.2 Periodo e URL

Supportare:

```text
/riepilogo-rda?period=previous_month
/riepilogo-rda?period=custom&from=2026-01-01&to=2026-07-06
```

Default: `previous_month`.

Opzioni periodo identiche a PA:

```ts
[
  { value: 'this_month', label: 'Mese corrente' },
  { value: 'previous_month', label: 'Mese precedente' },
  { value: 'this_quarter', label: 'Trimestre corrente' },
  { value: 'previous_quarter', label: 'Trimestre precedente' },
  { value: 'current_year', label: 'Anno corrente' },
  { value: 'previous_year', label: 'Anno precedente' },
  { value: 'custom', label: 'Intervallo personalizzato' },
]
```

Quando cambia periodo o date custom, resettare `selectedBudgetKey`.

## 5.3 Header e toolbar

Header:

```text
Titolo: Riepilogo RDA
Sottotitolo: Totali degli ordini RDA per budget nel periodo selezionato.
```

Toolbar:

- select periodo;
- date `Da` e `A esclusa` se custom;
- range calcolato;
- pulsante `Scarica Excel`.

## 5.4 KPI

Tre card:

- `Totale periodo` → `formatEUR(data.totals.amount)` oppure formatter valuta esistente;
- `Ordini RDA` → `data.totals.order_count`;
- `Budget` → `data.totals.budget_count`.

## 5.5 Grafico budget

Come PA, ma usare:

- key: `budget.budget_key`
- label: `budget.budget`
- amount: `budget.amount`
- percentage: `budget.percentage`
- count: `budget.order_count`

Click:

```ts
setSelectedBudgetKey((current) => current === budget.budget_key ? null : budget.budget_key)
```

Filtro dettagli:

```ts
const visibleDetails = selectedBudgetKey
  ? details.filter((detail) => budgetKeyForDetail(detail) === selectedBudgetKey)
  : details;
```

Implementare helper frontend coerente con backend:

```ts
function budgetKeyForDetail(detail: RiepilogoRdaDetail): string {
  return detail.budget_id == null ? 'senza-budget' : String(detail.budget_id);
}
```

## 5.6 Tabella dettagli

Colonne consigliate:

- Ordine (`code`)
- Progetto
- Oggetto
- Budget
- Centro di costo
- Importo totale
- Valuta
- Richiedente
- Fornitore
- Stato
- Creato

Fallback valori vuoti: `—`.

## 5.7 Stati UI

- Loading: `Caricamento riepilogo RDA…`
- Empty: `Nessun ordine RDA trovato per il periodo selezionato.`
- Errore dati: `Non è stato possibile caricare il riepilogo RDA.`
- Export in corso: `Preparazione file Excel…`
- Export errore: `Non è stato possibile generare il file Excel.`

## 5.8 Filename export frontend

Helper:

```ts
function filenameFor(period: RiepilogoPeriodSelection, from?: string, to?: string): string {
  const safeFrom = from ?? 'from';
  const safeTo = to ?? 'to';
  return `riepilogo-rda_${period}_${safeFrom}_${safeTo}.xlsx`;
}
```

Passare `data.period.from/to` quando disponibili.

---

# Fase 6 — Routing e navigazione

## 6.1 Route

In `apps/stats-rda/src/routes.tsx`:

```tsx
import { RiepilogoRdaPage } from './pages/RiepilogoRdaPage';
```

Aggiungere:

```tsx
{ path: 'riepilogo-rda', element: <RiepilogoRdaPage /> },
```

Mantenere `/rda` placeholder se ancora utile, oppure non toccarlo.

## 6.2 Nav

In `apps/stats-rda/src/App.tsx`, aggiornare `navItems`:

```ts
const navItems = [
  { label: 'Archivio PA', path: '/archivio-pa' },
  { label: 'Riepilogo PA Jira', path: '/riepilogo-pa' },
  { label: 'Riepilogo RDA', path: '/riepilogo-rda' },
  { label: 'RDA', path: '/rda' },
];
```

Se si vuole evitare duplicazione/confusione, chiedere al committente prima di rimuovere il tab placeholder `RDA`.

---

# Fase 7 — Documentazione

Aggiornare se emergono decisioni implementative diverse dal PRD:

- `apps/stats-rda/docs/PRD-RIEPILOGO-RDA.md`
- questo file `apps/stats-rda/docs/IMPLEMENTATION-PLAN-RIEPILOGO-RDA.md`

Non modificare documenti PA salvo refactor condivisi effettivi.

---

# Fase 8 — QA gates

Eseguire dopo implementazione:

```bash
pnpm --filter mrsmith-stats-rda lint
pnpm --filter mrsmith-stats-rda build
cd backend && go test ./internal/statsrda
```

Se Go/Node non sono disponibili localmente, usare `docs/TOOLING-WITH-DOCKER.md`.

Controlli manuali consigliati:

1. Aprire `/riepilogo-rda?period=previous_month`.
2. Aprire `/riepilogo-rda?period=custom&from=2026-01-01&to=2026-07-06`.
3. Verificare che il range mostrato sia coerente e che `to` sia indicato come escluso.
4. Cliccare un budget nel grafico e verificare filtro tabella.
5. Cliccare `Mostra tutti` e verificare reset filtro.
6. Scaricare Excel e verificare nome file + sheet `Totali budget` + sheet per budget.
7. Verificare assenza di `fetch(` anonimi nella nuova feature.

---

# Checklist finale per agent

- [ ] Letto PRD Riepilogo RDA.
- [ ] Letto `docs/UI-UX.md`.
- [ ] Backend route JSON `/stats-rda/v1/rda/riepilogo` implementata.
- [ ] Backend route export `/stats-rda/v1/rda/riepilogo/export` implementata.
- [ ] Query usano solo bind params per date.
- [ ] Allowlist stati RDA applicata.
- [ ] `po.deleted IS NULL` applicato.
- [ ] Range custom validato.
- [ ] Tipi TS RDA aggiunti.
- [ ] Hook API RDA aggiunti con `ApiClient`.
- [ ] Pagina `/riepilogo-rda` aggiunta.
- [ ] Nav aggiornata.
- [ ] Export Excel funzionante.
- [ ] `pnpm --filter mrsmith-stats-rda lint` PASS.
- [ ] `pnpm --filter mrsmith-stats-rda build` PASS.
- [ ] `cd backend && go test ./internal/statsrda` PASS.

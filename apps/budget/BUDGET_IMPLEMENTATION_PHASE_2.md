# Budget Management — Implementation Phase 2: Centri di costo

**Goal:** Build the Cost Centers view, refining the master-detail pattern from Phase 1 with more complex forms (multi-select for users + groups, manager dropdown), and adding the disable/enable flow with impact preview.

**Depends on:** Phase 1 complete (navigation shell, `@mrsmith/ui` foundations, Gruppi view working)

---

## Step 1: Go BFF — Additional Fixture Handlers

### 1.1 Add cost center fixtures

```
backend/internal/budget/fixtures/
├── groups.go           # (Phase 1 — already exists)
├── users.go            # (Phase 1 — already exists)
└── cost_centers.go     # NEW: cost center list + details
```

**Cost center list fixture:**
- 4–5 sample cost centers: "Ricerca e Sviluppo", "Marketing", "Vendite", "Risorse Umane", "Amministrazione"
- Shape: `{ name, manager_email, user_count, enabled }` (one disabled for testing)
- Wrapped in paginated envelope

**Cost center details fixture per name:**
- Shape: `{ name, manager: arak-int-user, users: arak-int-user[], groups: group-details[], enabled }`
- Reference manager and users from existing users fixture
- Reference groups from existing groups fixture

### 1.2 New handlers

| Route | Method | Handler | Response |
|-------|--------|---------|----------|
| `GET /api/budget/v1/cost-center` | GET | `handleGetAllCostCenters` | Cost center list fixture |
| `GET /api/budget/v1/cost-center/{name}` | GET | `handleGetCostCenterDetails` | Cost center details fixture |
| `POST /api/budget/v1/cost-center` | POST | `handleNewCostCenter` | Echo created (201) |
| `PUT /api/budget/v1/cost-center/{name}` | PUT | `handleEditCostCenter` | Echo updated (200) |

**Note:** Disable and Enable both use the PUT handler (different body: `{enabled: false}` vs `{enabled: true}`). No separate handler needed.

### 1.3 Reuse existing handlers

- `GET /api/users-int/v1/user` — already exists from Phase 1
- `GET /api/budget/v1/group` — already exists from Phase 1

---

## Step 2: TypeScript Types

Add to `src/api/types.ts`:

```typescript
interface CostCenter {
  name: string;
  manager_email: string;
  user_count: number;
  enabled: boolean;
}

interface CostCenterDetails {
  name: string;
  manager: ArakIntUser;
  users: ArakIntUser[];
  groups: GroupDetails[];
  enabled: boolean;
}

interface CostCenterNew {
  name: string;
  manager_id: number;
  user_ids: number[];
  group_names: string[];
  enabled: boolean;
}

interface CostCenterEdit {
  new_name?: string;
  manager_id?: number;
  user_ids?: number[];
  group_names?: string[];
  enabled?: boolean;
}
```

---

## Step 3: Centri di costo View

### 3.1 View structure

```
src/views/centri-di-costo/
├── CentriDiCostoPage.tsx       # Main page component
├── CostCenterTable.tsx          # Master table
├── CostCenterDetailPanel.tsx    # Side panel (read-only)
├── CostCenterMemberList.tsx     # User list in detail panel
├── CostCenterCreateModal.tsx    # Create form
├── CostCenterEditModal.tsx      # Edit form
├── CostCenterDisableConfirm.tsx # Disable confirmation with impact
└── useCostCenters.ts            # Data fetching hooks
```

### 3.2 Data fetching (`useCostCenters.ts`)

- `useCostCenters()` — list, from shared cache (same data as other views)
- `useCostCenterDetails(name)` — details on selection
- `useCreateCostCenter()` — POST → invalidate CC list cache → toast
- `useEditCostCenter(name)` — PUT → invalidate CC list + details → toast
- `useDisableCostCenter(name)` — PUT `{enabled: false}` → invalidate → toast
- `useEnableCostCenter(name)` — PUT `{enabled: true}` → invalidate → toast

**Shared data reuse:** `useUsers()` and `useGroups()` hooks from Phase 1 are reused here (same cached queries).

### 3.3 Page layout

```
┌────────────────────────────────┬───────────────────────────┐
│ Cost center table (master)     │ Detail panel (side)       │
│                                │                           │
│ [+ Nuovo centro di costo]     │ Nome: Ricerca e Sviluppo  │
│                                │ Manager: mario@acme.com   │
│ ┌────────────────────────────┐ │ Stato: Attivo ●           │
│ │ Nome      Att. Manager  N. │ │                           │
│ │ ──────────────────────────│ │ Membri:                   │
│ │ ▶R&S      ✓   mario@  5  │ │ ┌───────────────────────┐ │
│ │  Mktg     ✓   anna@   3  │ │ │ Nome    Email         │ │
│ │  Vendite  ✓   luca@   4  │ │ │ Mario   mario@...     │ │
│ │  Amm.     ✗   —       0  │ │ │ Giulia  giulia@...    │ │
│ └────────────────────────────┘ │ └───────────────────────┘ │
│                                │                           │
│                                │ [Modifica] [Disabilita]   │
│                                │      or [Abilita]         │
└────────────────────────────────┴───────────────────────────┘
```

### 3.4 Interaction flow

1. **Page load** → skeleton → fetch cost centers + users + groups → animate in
2. **Row select** → fetch details → panel slides in
3. **Detail panel** — read-only fields:
   - Name
   - Manager email
   - Enabled status (badge: "Attivo" green / "Disabilitato" red)
   - Member list table
4. **Action buttons** — context-sensitive:
   - Enabled CC: show "Modifica" + "Disabilita"
   - Disabled CC: show "Modifica" + "Abilita"
5. **"Nuovo centro di costo"** → modal:
   - Name (text input, required)
   - Manager (single-select dropdown from users list)
   - Users (multi-select from users list)
   - Groups (multi-select from groups list)
   - Enabled (toggle, default true)
   - → POST → toast → list refresh
6. **"Modifica"** → modal (pre-populated from details):
   - New name (text input, optional — only include in body if changed)
   - Manager (single-select, pre-selected current)
   - Users (multi-select, pre-selected current: `details.users.map(u => u.id)`)
   - Groups (multi-select, pre-selected current: `details.groups.map(g => g.name)`)
   - → PUT → toast → list + details refresh
7. **"Disabilita"** → confirm dialog (danger variant):
   - Shows impact: "Questo centro di costo ha {n} utenti assegnati:"
   - Lists affected users from current detail data (no extra fetch)
   - "Disabilitare {name}?" → PUT `{enabled: false}` → toast → refresh
8. **"Abilita"** → PUT `{enabled: true}` → toast → refresh (no confirmation needed, non-destructive)

### 3.5 New/refined `@mrsmith/ui` components

Components built in Phase 1 should cover most needs. Potential additions or refinements:

**`Select`** — Single-select dropdown (for manager picker):
- Search/filter
- Single selection
- Displays selected item
- Reuses dropdown animation from MultiSelect

**`StatusBadge`** — Status indicator:
- Variants: active (green), disabled (red), warning (amber)
- Small dot + label

**`FieldList`** — Read-only field display for detail panels:
- Label + value pairs
- Consistent spacing and typography
- Supports slot values (badges, custom renders)

### 3.6 Italian labels

- "Nuovo centro di costo", "Modifica", "Disabilita", "Abilita"
- "Attivo", "Disabilitato" (status badges)
- "Nome", "Manager", "Utenti", "Gruppi", "Stato" (form/panel labels)
- "Centro di costo creato/aggiornato/disabilitato/abilitato" (toasts)
- "Seleziona un centro di costo" (empty panel)
- "Questo centro di costo ha {n} utenti assegnati" (disable confirmation)

---

## Step 4: Shared Data Layer Refinement

### 4.1 Cache sharing validation

Verify that the reference data hooks (`useUsers()`, `useGroups()`, `useCostCenters()`) work correctly across views:

- Navigate from Gruppi to Centri di costo → users should not re-fetch
- Create a group in Gruppi → groups cache invalidated → Centri di costo group dropdowns reflect the change
- This is the first test of cross-view cache sharing

### 4.2 Cost center cache for other views

The cost center list will be needed in Phase 3 (Voci di costo) for allocation dropdowns. Ensure the cache is wired at app level (not per-view).

---

## Step 5: Validation & Polish

### 5.1 Functional testing

- [ ] Cost center list loads with skeleton → data
- [ ] Row selection shows detail panel with member list
- [ ] Create cost center with manager + users + groups → appears in list
- [ ] Edit cost center (rename, change manager, change members) → updates
- [ ] Disable cost center → confirmation shows affected users → status changes
- [ ] Enable cost center → status changes (no confirmation needed)
- [ ] Disabled CC shows "Abilita" instead of "Disabilita"
- [ ] Error states display correctly
- [ ] Empty states render

### 5.2 Cross-view validation

- [ ] Navigate Gruppi ↔ Centri di costo — shared data not re-fetched
- [ ] Group mutations in Gruppi reflect in Centri di costo group dropdowns
- [ ] Tab navigation highlights correct tab
- [ ] Tab switch animation is smooth

### 5.3 WOW effect checklist

- [ ] Phase 1 animations/transitions carry over seamlessly
- [ ] Disable confirmation dialog has appropriate gravity (danger styling, impact list)
- [ ] Enable action feels lightweight (no over-confirmation)
- [ ] Status badges have clean visual weight
- [ ] Multi-select (users + groups) handles 10+ items gracefully
- [ ] Manager single-select with search works smoothly

---

## Deliverables

| Deliverable | Location |
|-------------|----------|
| Go BFF handlers (cost center fixtures) | `backend/internal/budget/` |
| Cost center TypeScript types | `apps/budget/src/api/types.ts` |
| Centri di costo view (complete CRUD + disable/enable) | `apps/budget/src/views/centri-di-costo/` |
| Select component (single-select dropdown) | `packages/ui/src/components/` |
| StatusBadge, FieldList components | `packages/ui/src/components/` |
| Validated cross-view cache sharing | — |

**Phase 2 is complete when:** Centri di costo is fully functional with mocked data, cross-view cache sharing works between Gruppi and Centri di costo, and the disable/enable flow with impact preview is polished.

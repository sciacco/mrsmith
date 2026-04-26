import type { BudgetForUser } from '../api/types';

function budgetID(budget: BudgetForUser): number {
  return budget.budget_id ?? budget.id ?? 0;
}

export function BudgetSelect({
  budgets,
  value,
  disabled,
  onChange,
}: {
  budgets: BudgetForUser[];
  value: number | '';
  disabled?: boolean;
  onChange: (value: number | '') => void;
}) {
  return (
    <select value={value} disabled={disabled} onChange={(event) => onChange(event.target.value ? Number(event.target.value) : '')}>
      <option value="">Seleziona budget</option>
      {budgets.map((budget) => {
        const id = budgetID(budget);
        const scope = budget.cost_center ? ` · ${budget.cost_center}` : budget.budget_user_id ? ` · utente ${budget.budget_user_id}` : '';
        return (
          <option key={id} value={id}>
            {budget.name ?? `Budget ${id}`}{scope}
          </option>
        );
      })}
    </select>
  );
}

export function findBudget(budgets: BudgetForUser[], selected: number | ''): BudgetForUser | undefined {
  if (!selected) return undefined;
  return budgets.find((budget) => budgetID(budget) === selected);
}

export function selectedBudgetID(budget: BudgetForUser | undefined): number {
  return budget ? budgetID(budget) : 0;
}

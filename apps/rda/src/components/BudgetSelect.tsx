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
        return (
          <option key={id} value={id}>
            {budget.name ?? `Budget ${id}`}
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

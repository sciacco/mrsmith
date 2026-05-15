import type { BudgetForUser } from '../api/types';
import { budgetOptionLabel, budgetSelectionKey, type BudgetSelection } from '../lib/budgets';

export { findBudget, selectedBudgetID } from '../lib/budgets';

export function BudgetSelect({
  budgets,
  value,
  disabled,
  onChange,
}: {
  budgets: BudgetForUser[];
  value: BudgetSelection;
  disabled?: boolean;
  onChange: (value: BudgetSelection) => void;
}) {
  return (
    <select value={value === '' ? '' : String(value)} disabled={disabled} onChange={(event) => onChange(event.target.value || '')}>
      <option value="">Seleziona budget</option>
      {budgets.map((budget, index) => {
        const key = budgetSelectionKey(budget);
        return (
          <option key={`${key}-${index}`} value={key}>
            {budgetOptionLabel(budget)}
          </option>
        );
      })}
    </select>
  );
}

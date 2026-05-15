import type { BudgetForUser } from '../api/types';

const BUDGET_KEY_SEPARATOR = '|';

export type BudgetSelection = string | number | '';

export interface BudgetBinding {
  cost_center?: string;
  budget_user_id?: number;
}

export function budgetID(budget: BudgetForUser | undefined): number {
  return budget?.budget_id ?? budget?.id ?? 0;
}

function budgetUserID(budget: BudgetForUser): number | undefined {
  return budget.budget_user_id ?? budget.user_id ?? undefined;
}

function budgetIDFromSelection(selected: BudgetSelection): number {
  if (!selected) return 0;
  if (typeof selected === 'number') return selected;
  const [rawID] = selected.split(BUDGET_KEY_SEPARATOR);
  const id = Number(rawID);
  return Number.isFinite(id) ? id : 0;
}

export function budgetSelectionKey(budget: BudgetForUser): string {
  const id = budgetID(budget);
  const costCenter = budget.cost_center?.trim();
  if (costCenter) return [id, 'cost_center', encodeURIComponent(costCenter)].join(BUDGET_KEY_SEPARATOR);
  const userID = budgetUserID(budget);
  if (userID) return [id, 'user', userID].join(BUDGET_KEY_SEPARATOR);
  return [id, 'unbound'].join(BUDGET_KEY_SEPARATOR);
}

export function budgetOptionLabel(budget: BudgetForUser): string {
  const id = budgetID(budget);
  const label = budget.name?.trim() || `Budget ${id}`;
  const costCenter = budget.cost_center?.trim();
  if (costCenter && !label.includes(costCenter)) return `${label} (${costCenter})`;
  if (!costCenter && budget.user_email?.trim() && !label.includes(budget.user_email.trim())) {
    return `${label} (${budget.user_email.trim()})`;
  }
  return label;
}

export function budgetBinding(budget: BudgetForUser | undefined): BudgetBinding {
  if (!budget) return {};
  const costCenter = budget.cost_center?.trim();
  if (costCenter) return { cost_center: costCenter };
  const userID = budgetUserID(budget);
  return userID ? { budget_user_id: userID } : {};
}

export function findBudget(budgets: BudgetForUser[], selected: BudgetSelection): BudgetForUser | undefined {
  if (!selected) return undefined;
  if (typeof selected === 'string' && selected.includes(BUDGET_KEY_SEPARATOR)) {
    const exact = budgets.find((budget) => budgetSelectionKey(budget) === selected);
    if (exact) return exact;
    const id = budgetIDFromSelection(selected);
    const matches = budgets.filter((budget) => budgetID(budget) === id);
    const [match] = matches;
    return matches.length === 1 ? match : undefined;
  }
  const id = budgetIDFromSelection(selected);
  return budgets.find((budget) => budgetID(budget) === id);
}

export function selectedBudgetID(selected: BudgetSelection, budgets: BudgetForUser[]): number {
  const budget = findBudget(budgets, selected);
  return budget ? budgetID(budget) : budgetIDFromSelection(selected);
}

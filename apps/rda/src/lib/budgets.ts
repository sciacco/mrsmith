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

function budgetName(budget: BudgetForUser | undefined): string {
  if (!budget) return '';
  return budget.name?.trim() || budget.budget_name?.trim() || '';
}

function budgetUserID(budget: BudgetForUser): number | undefined {
  if (budget.budget_user_id && budget.budget_user_id > 0) return budget.budget_user_id;
  if (budget.user_id && budget.user_id > 0) return budget.user_id;
  return undefined;
}

function budgetIDFromSelection(selected: BudgetSelection): number {
  if (!selected) return 0;
  if (typeof selected === 'number') return selected;
  const [rawID] = selected.split(BUDGET_KEY_SEPARATOR);
  const id = Number(rawID);
  return Number.isFinite(id) ? id : 0;
}

function budgetSelectionParts(selected: BudgetSelection): { id: number; kind: string; rawValue: string } | null {
  if (typeof selected !== 'string' || !selected.includes(BUDGET_KEY_SEPARATOR)) return null;
  const parts = selected.split(BUDGET_KEY_SEPARATOR);
  const id = Number(parts[0] ?? '');
  const kind = parts[1] ?? '';
  if (!Number.isFinite(id) || !kind) return null;
  return { id, kind, rawValue: parts[2] ?? '' };
}

function decodeBudgetKeyValue(value: string): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
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
  const label = budgetName(budget) || `Budget ${id}`;
  const costCenter = budget.cost_center?.trim();
  if (costCenter && !label.includes(costCenter)) return `${label} (${costCenter})`;
  if (!costCenter && budget.user_email?.trim() && !label.includes(budget.user_email.trim())) {
    return `${label} (${budget.user_email.trim()})`;
  }
  return label;
}

export function budgetDisplayLabel(budget: BudgetForUser | undefined): string {
  return budget ? budgetOptionLabel(budget) : '-';
}

export function budgetBinding(budget: BudgetForUser | undefined): BudgetBinding {
  if (!budget) return {};
  const costCenter = budget.cost_center?.trim();
  if (costCenter) return { cost_center: costCenter };
  const userID = budgetUserID(budget);
  return userID ? { budget_user_id: userID } : {};
}

export function budgetBindingFromSelection(selected: BudgetSelection): BudgetBinding {
  const parts = budgetSelectionParts(selected);
  if (!parts) return {};
  if (parts.kind === 'cost_center') {
    const costCenter = decodeBudgetKeyValue(parts.rawValue).trim();
    return costCenter ? { cost_center: costCenter } : {};
  }
  if (parts.kind === 'user') {
    const userID = Number(parts.rawValue);
    return Number.isInteger(userID) && userID > 0 ? { budget_user_id: userID } : {};
  }
  return {};
}

export function findBudget(budgets: BudgetForUser[], selected: BudgetSelection): BudgetForUser | undefined {
  if (!selected) return undefined;
  if (typeof selected === 'string' && selected.includes(BUDGET_KEY_SEPARATOR)) {
    return budgets.find((budget) => budgetSelectionKey(budget) === selected);
  }
  const id = budgetIDFromSelection(selected);
  const matches = budgets.filter((budget) => budgetID(budget) === id);
  return matches.length === 1 ? matches[0] : undefined;
}

export function selectedBudgetID(selected: BudgetSelection, budgets: BudgetForUser[]): number {
  const budget = findBudget(budgets, selected);
  return budget ? budgetID(budget) : budgetIDFromSelection(selected);
}

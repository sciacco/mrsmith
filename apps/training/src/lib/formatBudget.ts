import { formatCurrency } from '@mrsmith/format';

export function formatBudget(value: number | undefined | null): string | undefined {
  if (value === undefined || value === null) return undefined;
  return formatCurrency(value, 'EUR', { format: { maximumFractionDigits: 0 } }) ?? undefined;
}

export function formatRequiredBudget(value: number): string {
  return formatBudget(value) ?? '—';
}

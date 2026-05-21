const BUDGET_FORMATTER = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  maximumFractionDigits: 0,
});

export function formatBudget(value: number | undefined | null): string | undefined {
  if (value === undefined || value === null) return undefined;
  return BUDGET_FORMATTER.format(value);
}

/**
 * Formats an API decimal string for Italian display.
 * "50000.00" → "50.000,00 €"
 * Presentation only — never used for input or state.
 */
const moneyFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatMoneyDisplay(apiValue: string): string {
  const value = Number(apiValue);
  if (!Number.isFinite(value)) return apiValue;

  return moneyFormatter.format(value);
}

/** Validates a monetary input string matches API decimal format. */
export function isValidMoneyInput(value: string): boolean {
  return /^\d+(\.\d+)?$/.test(value);
}

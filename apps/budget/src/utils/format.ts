/**
 * Formats an API decimal string for Italian display.
 * "50000.00" → "50.000,00"
 * Presentation only — never used for input or state.
 */
export function formatMoneyDisplay(apiValue: string): string {
  const [intPart, decPart = '00'] = apiValue.split('.');
  const withThousands = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
  return `${withThousands},${decPart.padEnd(2, '0')}`;
}

/** Validates a monetary input string matches API decimal format. */
export function isValidMoneyInput(value: string): boolean {
  return /^\d+(\.\d{1,2})?$/.test(value);
}

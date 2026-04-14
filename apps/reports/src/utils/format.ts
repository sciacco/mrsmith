const moneyFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const numberFormatter = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatMoneyEUR(value: number): string {
  return moneyFormatter.format(value);
}

export function formatNumber(value: number): string {
  return numberFormatter.format(value);
}

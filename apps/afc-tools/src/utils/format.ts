const moneyFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatMoneyEUR(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '';
  return moneyFormatter.format(value);
}

const numberFormatter = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function formatNumber(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '';
  return numberFormatter.format(value);
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '';
  const t = iso.slice(0, 10);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(t)) return iso;
  const [y, m, d] = t.split('-');
  return `${d}/${m}/${y}`;
}

// isEmpty treats null, undefined and empty strings as equivalent — preserves
// the "Nessun valore" behavior in Dettaglio ordini (decision A.5.1c).
export function isEmpty(v: unknown): boolean {
  return v == null || v === '';
}

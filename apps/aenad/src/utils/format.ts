const moneyFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
});

const quantityFormatter = new Intl.NumberFormat('it-IT', {
  maximumFractionDigits: 4,
});

/** Converte una stringa decimale API in numero, solo per preview/calcoli locali non persistiti. */
export function parseDecimal(value: string | null): number | null {
  if (value == null || value.trim() === '') return null;
  const parsed = Number.parseFloat(value);
  return Number.isNaN(parsed) ? null : parsed;
}

export function formatMoney(value: string | number | null): string {
  const parsed = typeof value === 'number' ? value : parseDecimal(value);
  if (parsed == null || Number.isNaN(parsed)) return '-';
  return moneyFormatter.format(parsed);
}

export function formatQuantity(value: string | null): string {
  const parsed = parseDecimal(value);
  if (parsed == null) return '-';
  return quantityFormatter.format(parsed);
}

/** Riduce la forma canonica a 4 decimali ("1.5000") alla forma di editing ("1.5"). */
export function trimDecimalZeros(value: string): string {
  if (!value.includes('.')) return value;
  return value.replace(/0+$/, '').replace(/\.$/, '');
}

/** Calcola il moltiplicatore dello sconto sequenziale (es. "10+5%" o "10+5+2" -> 0.855) */
export function calculateDiscountMultiplier(discounts: string | null | undefined): number {
  if (!discounts) return 1;
  const cleaned = discounts.replace(/\s+/g, '').replace(/,/g, '.');
  if (cleaned === '') return 1;

  let multiplier = 1;
  const parts = cleaned.split('+');
  for (let part of parts) {
    part = part.replace(/%$/, '');
    if (part === '' || Number.isNaN(Number(part))) {
      continue;
    }
    const percent = Number(part);
    multiplier = multiplier * (1 - percent / 100);
  }
  return multiplier;
}


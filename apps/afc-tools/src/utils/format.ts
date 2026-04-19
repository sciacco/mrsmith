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

// Italian currency names → ISO 4217 codes used in Mistra/Alyante data.
const CURRENCY_ALIASES: Record<string, string> = {
  EURO: 'EUR',
  DOLLARO: 'USD',
  'DOLLARO USA': 'USD',
  STERLINA: 'GBP',
  'FRANCO SVIZZERO': 'CHF',
};

const formatterCache = new Map<string, Intl.NumberFormat>();

function getFormatter(isoCode: string): Intl.NumberFormat {
  let f = formatterCache.get(isoCode);
  if (f) return f;
  try {
    f = new Intl.NumberFormat('it-IT', {
      style: 'currency',
      currency: isoCode,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });
  } catch {
    f = moneyFormatter;
  }
  formatterCache.set(isoCode, f);
  return f;
}

export function formatMoney(value: number | null | undefined, valuta: string | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '';
  const key = (valuta ?? '').trim().toUpperCase();
  const iso = CURRENCY_ALIASES[key] ?? (key.length === 3 ? key : 'EUR');
  return getFormatter(iso).format(value);
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

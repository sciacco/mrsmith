// Display formatting helpers for the Archivio PA screen.

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

const dateFormatter = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
});

const dateTimeFormatter = new Intl.DateTimeFormat('it-IT', {
  day: '2-digit',
  month: '2-digit',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
});

function formatDateParts(value: string): string | null {
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return null;
  return `${match[3]}/${match[2]}/${match[1]}`;
}

function parseBrowserDate(value: string): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Format an amount in EUR using Italian locale. */
export function formatEUR(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '—';
  return moneyFormatter.format(value);
}

/** Format a plain number (no currency) using Italian locale. */
export function formatNumber(value: number | null | undefined): string {
  if (value == null || Number.isNaN(value)) return '—';
  return numberFormatter.format(value);
}

/** Format a date in Italian national format (dd/mm/yyyy). */
export function shortDate(value: string | null | undefined): string {
  if (!value) return '—';

  // PostgreSQL may return legacy timestamp strings that browsers do not parse
  // consistently. Date-only values must also remain independent of timezones.
  const formattedParts = formatDateParts(value);
  if (formattedParts) return formattedParts;

  const date = parseBrowserDate(value);
  return date ? dateFormatter.format(date) : value;
}

/** Format a timestamp in Italian national format (dd/mm/yyyy, HH:mm). */
export function shortDateTime(value: string | null | undefined): string {
  if (!value) return '—';

  const formattedParts = formatDateParts(value);
  const timeMatch = value.match(/(?:T|\s)(\d{2}):(\d{2})/);
  if (formattedParts) {
    return timeMatch ? `${formattedParts}, ${timeMatch[1]}:${timeMatch[2]}` : formattedParts;
  }

  const date = parseBrowserDate(value);
  return date ? dateTimeFormatter.format(date) : value;
}

/** Return the fallback dash for empty values. */
export function nz(value: string | null | undefined): string {
  if (value === null || value === undefined || value === '') return '—';
  return value;
}

/** Compute the total for a line item, handling Articoli (prezzo_totale null). */
export function lineItemTotal(item: {
  prezzo_totale: number | null;
  quantita: number | null;
  importo: number | null;
}): number | null {
  if (item.prezzo_totale != null) return item.prezzo_totale;
  if (item.quantita != null && item.importo != null) return item.quantita * item.importo;
  return item.importo ?? null;
}

/** Format file size in human-readable form. */
export function formatFileSize(bytes: number | null | undefined): string {
  if (bytes == null) return '—';
  const units = ['B', 'kB', 'MB', 'GB'];
  let v = bytes;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}

/** Status badge class for a given workflow status. */
export function statusBadgeClass(status: string | null | undefined): string {
  if (!status) return 'badge badgeGray';
  const s = status.toLowerCase();
  if (s === 'closed' || s === 'done') return 'badge badgeGreen';
  if (s.includes('annull') || s.includes('non approv') || s === "won't fix") return 'badge badgeRed';
  if (s.includes('attesa') || s.includes('approvato') || s.includes('attiv')) return 'badge badgeYellow';
  return 'badge badgeGray';
}

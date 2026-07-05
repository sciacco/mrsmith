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

/** Trim a timestamp/date string to a readable short form. */
export function shortDate(value: string | null | undefined): string {
  if (!value) return '—';
  return value.slice(0, 10);
}

/** Format an ISO timestamp as YYYY-MM-DD HH:MM. */
export function shortDateTime(value: string | null | undefined): string {
  if (!value) return '—';
  const s = value.slice(0, 16).replace('T', ' ');
  return s;
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

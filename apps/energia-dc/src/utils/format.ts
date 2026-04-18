const euroFormatter = new Intl.NumberFormat('it-IT', {
  style: 'currency',
  currency: 'EUR',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const decimalFormatter = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const compactFormatter = new Intl.NumberFormat('it-IT', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
});

function pad(value: string) {
  return value.padStart(2, '0');
}

export function formatDate(value?: string): string {
  if (!value) return '-';
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match) return value;
  return `${match[3]}/${match[2]}/${match[1]}`;
}

export function formatDateTime(value?: string): string {
  if (!value) return '-';
  const match = value.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2}))$/);
  if (!match) return value;
  return `${match[3]}/${match[2]}/${match[1]} ${match[4]}:${match[5]}`;
}

export function formatMaybeText(value?: string | number | null): string {
  if (value == null || value === '') return '-';
  return String(value);
}

export function formatAmpere(value?: number | null): string {
  if (value == null) return '-';
  return `${compactFormatter.format(value)} A`;
}

export function formatKw(value?: number | null): string {
  if (value == null) return '-';
  return `${compactFormatter.format(value)} kW`;
}

export function formatMoneyEUR(value?: number | null): string {
  if (value == null) return '-';
  return euroFormatter.format(value);
}

export function formatNumber(value?: number | null): string {
  if (value == null) return '-';
  return decimalFormatter.format(value);
}

export function formatPercent(value?: number | null): string {
  if (value == null) return '-';
  return `${compactFormatter.format(value)}%`;
}

export function formatCount(value: number, singular: string, plural: string) {
  return `${value} ${value === 1 ? singular : plural}`;
}

export function toDateTimeLocalInput(date: Date): string {
  return `${date.getFullYear()}-${pad(String(date.getMonth() + 1))}-${pad(String(date.getDate()))}T${pad(String(date.getHours()))}:${pad(String(date.getMinutes()))}`;
}

function escapeCsvCell(value: string): string {
  const escaped = value.replaceAll('"', '""');
  if (/[;"\n\r]/.test(escaped)) {
    return `"${escaped}"`;
  }
  return escaped;
}

export function downloadCsv(filename: string, headers: string[], rows: string[][]) {
  const csv = [headers, ...rows]
    .map((row) => row.map((cell) => escapeCsvCell(cell)).join(';'))
    .join('\r\n');

  const blob = new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

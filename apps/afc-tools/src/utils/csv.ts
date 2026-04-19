export type CsvValue = string | number | null | undefined;

function escapeField(value: CsvValue): string {
  if (value === null || value === undefined) return '';
  const raw = typeof value === 'number' ? formatItNumber(value) : String(value);
  if (/[;"\r\n]/.test(raw)) {
    return `"${raw.replace(/"/g, '""')}"`;
  }
  return raw;
}

function formatItNumber(value: number): string {
  if (!Number.isFinite(value)) return '';
  return value.toString().replace('.', ',');
}

export function downloadCsv(filename: string, headers: string[], rows: CsvValue[][]): void {
  const lines = [headers.map(escapeField).join(';')];
  for (const row of rows) {
    lines.push(row.map(escapeField).join(';'));
  }
  const bom = '\uFEFF';
  const blob = new Blob([bom + lines.join('\r\n')], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

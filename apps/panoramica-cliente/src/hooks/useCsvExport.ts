import { useCallback } from 'react';

interface CsvColumn<T> {
  key: keyof T;
  label: string;
}

export function useCsvExport<T>(columns: CsvColumn<T>[], filename: string) {
  return useCallback((data: T[]) => {
    const header = columns.map(c => c.label).join(';');
    const rows = data.map(row =>
      columns.map(c => {
        const v = row[c.key];
        if (v == null) return '';
        const s = String(v).replace(/"/g, '""');
        return s.includes(';') || s.includes('"') || s.includes('\n') ? `"${s}"` : s;
      }).join(';'),
    );
    const csv = '\uFEFF' + [header, ...rows].join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${filename}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }, [columns, filename]);
}

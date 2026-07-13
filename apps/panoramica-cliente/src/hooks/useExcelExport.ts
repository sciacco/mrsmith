import { useCallback, useState } from 'react';

export type ExcelCellType = 'string' | 'number' | 'date';

export interface ExcelColumn<T> {
  key: keyof T;
  label: string;
  width?: number;
  type?: ExcelCellType;
  numFmt?: string;
  value?: (row: T) => unknown;
}

interface ExcelExportOptions {
  filename: string;
  sheetName: string;
}

function safeFilename(value: string): string {
  const normalized = value
    .normalize('NFKD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-zA-Z0-9._-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[-_.]+|[-_.]+$/g, '');
  return normalized || 'export';
}

function safeSheetName(value: string): string {
  return value.replace(/[\\/*?:[\]]/g, ' ').trim().slice(0, 31) || 'Dati';
}

function cellValue<T>(row: T, column: ExcelColumn<T>): string | number | Date | null {
  const raw = column.value ? column.value(row) : row[column.key];
  if (raw == null) return null;

  if (column.type === 'number') {
    return typeof raw === 'number' && Number.isFinite(raw) ? raw : null;
  }
  if (column.type === 'date') {
    return raw instanceof Date && !Number.isNaN(raw.getTime()) ? raw : null;
  }
  return String(raw);
}

export function localDateStamp(date = new Date()): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function parseExcelDate(value: string | null | undefined): Date | null {
  const match = value?.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return null;
  const [, year, month, day] = match;
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  if (
    Number.isNaN(date.getTime())
    || date.getUTCFullYear() !== Number(year)
    || date.getUTCMonth() !== Number(month) - 1
    || date.getUTCDate() !== Number(day)
  ) return null;
  return date;
}

export function useExcelExport<T>(columns: ExcelColumn<T>[], options: ExcelExportOptions) {
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState(false);

  const exportExcel = useCallback(async (data: T[]) => {
    if (data.length === 0 || exporting) return;

    setExporting(true);
    setError(false);
    try {
      const { Workbook } = await import('exceljs');
      const workbook = new Workbook();
      workbook.creator = 'MrSmith';
      workbook.created = new Date();

      const worksheet = workbook.addWorksheet(safeSheetName(options.sheetName), {
        views: [{ state: 'frozen', ySplit: 1 }],
      });
      worksheet.columns = columns.map(column => ({
        header: column.label,
        key: String(column.key),
        width: column.width ?? 18,
      }));

      const header = worksheet.getRow(1);
      header.font = { bold: true, color: { argb: 'FF1E293B' } };
      header.fill = { type: 'pattern', pattern: 'solid', fgColor: { argb: 'FFEFF1FF' } };
      header.alignment = { vertical: 'middle' };
      header.height = 22;

      data.forEach(row => {
        const excelRow = worksheet.addRow(columns.map(column => cellValue(row, column)));
        columns.forEach((column, index) => {
          if (column.numFmt) excelRow.getCell(index + 1).numFmt = column.numFmt;
        });
      });

      worksheet.autoFilter = {
        from: { row: 1, column: 1 },
        to: { row: Math.max(1, worksheet.rowCount), column: columns.length },
      };

      const bytes = await workbook.xlsx.writeBuffer();
      const blob = new Blob([bytes], {
        type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
      });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `${safeFilename(options.filename)}.xlsx`;
      anchor.hidden = true;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1_000);
    } catch {
      setError(true);
    } finally {
      setExporting(false);
    }
  }, [columns, exporting, options.filename, options.sheetName]);

  return { exportExcel, exporting, error };
}

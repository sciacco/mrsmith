import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useMemo } from 'react';
import { useDdtCespiti } from '../api/queries';
import { downloadCsv } from '../utils/csv';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';

const CURRENCY_COLUMNS = new Set(['Importo_unitario', 'Importo_totale']);

export default function ReportDdtCespitiPage() {
  const q = useDdtCespiti();

  const columns = useMemo(() => {
    if (!q.data || q.data.length === 0) return [];
    const seen = new Set<string>();
    const ordered: string[] = [];
    for (const row of q.data) {
      for (const k of Object.keys(row)) {
        if (!seen.has(k)) {
          seen.add(k);
          ordered.push(k);
        }
      }
    }
    return ordered;
  }, [q.data]);

  const serializeCell = (v: unknown): string => {
    if (v == null) return '';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
  };

  const renderTableCell = (column: string, value: unknown): string => {
    if (CURRENCY_COLUMNS.has(column)) {
      const numericValue =
        typeof value === 'number'
          ? value
          : typeof value === 'string' && value.trim() !== ''
            ? Number(value)
            : NaN;
      if (Number.isFinite(numericValue)) {
        return formatMoneyEUR(numericValue);
      }
    }
    return serializeCell(value);
  };

  function handleExport() {
    downloadCsv(
      'report-ddt-per-cespiti.csv',
      columns,
      (q.data ?? []).map((row) => columns.map((column) => serializeCell(row[column]))),
    );
  }

  return (
    <div className={shared.page}>
      <div className={shared.titleRow}>
        <h1 className={`${shared.title} ${shared.titleCompact}`}>Report DDT per cespiti</h1>
        <Button
          variant="secondary"
          size="sm"
          onClick={handleExport}
          disabled={!q.data || q.data.length === 0}
          leftIcon={<Icon name="download" size={14} />}
        >
          Esporta CSV
        </Button>
      </div>
      <p className={shared.info}>Verifica DDT/cespiti da Alyante.</p>

      {q.isLoading && <Skeleton rows={10} />}
      {q.isError && <div className={shared.error}>Errore nel caricamento del report.</div>}

      {q.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                {columns.map((c) => (
                  <th key={c}>{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {q.data.map((row, i) => (
                <tr key={i} style={{ animationDelay: `${Math.min(i * 6, 300)}ms` }}>
                  {columns.map((c) => (
                    <td key={c}>{renderTableCell(c, row[c])}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
          {q.data.length === 0 && <div className={shared.empty}>Nessun DDT.</div>}
        </div>
      )}
    </div>
  );
}

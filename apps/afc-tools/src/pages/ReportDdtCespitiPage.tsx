import { Skeleton } from '@mrsmith/ui';
import { useMemo } from 'react';
import { useDdtCespiti } from '../api/queries';
import shared from './shared.module.css';

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

  const renderCell = (v: unknown): string => {
    if (v == null) return '';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
  };

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Report DDT per cespiti</h1>
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
                    <td key={c}>{renderCell(row[c])}</td>
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

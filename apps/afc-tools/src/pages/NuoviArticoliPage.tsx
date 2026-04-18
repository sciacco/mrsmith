import { Skeleton } from '@mrsmith/ui';
import { useMissingArticles } from '../api/queries';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';

export default function NuoviArticoliPage() {
  const q = useMissingArticles();

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Articoli da creare in Alyante</h1>
      <p className={shared.info}>Articoli Mistra con sync ERP attivo e non ancora presenti in Alyante.</p>

      {q.isLoading && <Skeleton rows={10} />}
      {q.isError && <div className={shared.error}>Errore nel caricamento degli articoli.</div>}

      {q.data && (
        <div className={shared.tableWrap}>
          <table className={shared.table}>
            <thead>
              <tr>
                <th>Codice</th>
                <th>Categoria</th>
                <th>Descrizione (IT)</th>
                <th>Descrizione (EN)</th>
                <th className={shared.numCol}>NRC</th>
                <th className={shared.numCol}>MRC</th>
              </tr>
            </thead>
            <tbody>
              {q.data.map((a, i) => (
                <tr key={a.code} style={{ animationDelay: `${Math.min(i * 10, 300)}ms` }}>
                  <td className={shared.mono}>{a.code}</td>
                  <td>{a.categoria ?? ''}</td>
                  <td>{a.descrizione_it ?? ''}</td>
                  <td>{a.descrizione_en ?? ''}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(a.nrc)}</td>
                  <td className={shared.numCol}>{formatMoneyEUR(a.mrc)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {q.data.length === 0 && <div className={shared.empty}>Nessun articolo da creare.</div>}
        </div>
      )}
    </div>
  );
}

import { Button, Icon, Skeleton } from '@mrsmith/ui';
import { useMissingArticles } from '../api/queries';
import { downloadCsv } from '../utils/csv';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';

export default function NuoviArticoliPage() {
  const q = useMissingArticles();

  function handleExport() {
    downloadCsv(
      'articoli-da-creare-in-alyante.csv',
      ['Codice', 'Categoria', 'Descrizione (IT)', 'Descrizione (EN)', 'NRC', 'MRC'],
      (q.data ?? []).map((a) => [
        a.code,
        a.categoria ?? '',
        a.descrizione_it ?? '',
        a.descrizione_en ?? '',
        formatMoneyEUR(a.nrc),
        formatMoneyEUR(a.mrc),
      ]),
    );
  }

  return (
    <div className={shared.page}>
      <div className={shared.titleRow}>
        <h1 className={`${shared.title} ${shared.titleCompact}`}>Articoli da creare in Alyante</h1>
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

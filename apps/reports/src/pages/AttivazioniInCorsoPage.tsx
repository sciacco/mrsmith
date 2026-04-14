import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { usePendingActivations, usePendingActivationRows } from '../api/queries';
import { formatMoneyEUR } from '../utils/format';
import shared from './shared.module.css';
import styles from './AttivazioniInCorsoPage.module.css';

export default function AttivazioniInCorsoPage() {
  const [selectedOrder, setSelectedOrder] = useState<string | null>(null);
  const activationsQ = usePendingActivations();
  const rowsQ = usePendingActivationRows(selectedOrder);

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Attivazioni in corso</h1>
      <p className={styles.subtitle}>
        Elenco ordini in stato confermato con righe da attivare
      </p>

      {activationsQ.isLoading && <Skeleton rows={8} />}

      {activationsQ.error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {activationsQ.data && (
        <>
          <div className={shared.info}>{activationsQ.data.length} ordini</div>
          <div className={shared.tableWrap}>
            <table className={`${shared.table} ${styles.table}`}>
              <thead>
                <tr>
                  <th></th>
                  <th>Cliente</th>
                  <th>N. Ordine</th>
                  <th>Data documento</th>
                  <th>Durata servizio</th>
                  <th>Durata rinnovo</th>
                  <th>Sost. ord.</th>
                  <th>Sostituito da</th>
                  <th>Storico</th>
                </tr>
              </thead>
              <tbody>
                {activationsQ.data.map((row, i) => (
                  <tr
                    key={row.numero_ordine}
                    className={selectedOrder === row.numero_ordine ? styles.selectedRow : undefined}
                    onClick={() => setSelectedOrder(
                      selectedOrder === row.numero_ordine ? null : row.numero_ordine,
                    )}
                    style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                  >
                    <td><div className={styles.accentBar} /></td>
                    <td>{row.ragione_sociale}</td>
                    <td className={shared.mono}>{row.numero_ordine}</td>
                    <td>{row.data_documento?.slice(0, 10) ?? ''}</td>
                    <td>{row.durata_servizio ?? ''}</td>
                    <td>{row.durata_rinnovo ?? ''}</td>
                    <td>{row.sost_ord ?? ''}</td>
                    <td>{row.sostituito_da ?? ''}</td>
                    <td>{row.storico ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {selectedOrder && (
        <div className={styles.detailSection}>
          <h2 className={styles.detailTitle}>
            Righe ordine {selectedOrder}
          </h2>

          {rowsQ.isLoading && <Skeleton rows={4} />}

          {rowsQ.error && <p>Errore nel caricamento delle righe.</p>}

          {rowsQ.data && (
            <div className={shared.tableWrap}>
              <table className={shared.table}>
                <thead>
                  <tr>
                    <th>Descrizione</th>
                    <th className={shared.numCol}>Quantita</th>
                    <th className={shared.numCol}>NRC</th>
                    <th className={shared.numCol}>MRC</th>
                    <th className={shared.numCol}>Totale MRC</th>
                    <th>Stato riga</th>
                    <th>Serial number</th>
                    <th>Note legali</th>
                  </tr>
                </thead>
                <tbody>
                  {rowsQ.data.map((row, i) => (
                    <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
                      <td>{row.descrizione_long ?? ''}</td>
                      <td className={shared.numCol}>{row.quantita ?? ''}</td>
                      <td className={shared.numCol}>{row.nrc != null ? formatMoneyEUR(row.nrc) : ''}</td>
                      <td className={shared.numCol}>{row.mrc != null ? formatMoneyEUR(row.mrc) : ''}</td>
                      <td className={shared.numCol}>{row.totale_mrc != null ? formatMoneyEUR(row.totale_mrc) : ''}</td>
                      <td>{row.stato_riga ?? ''}</td>
                      <td className={shared.mono}>{row.serialnumber ?? ''}</td>
                      <td>{row.note_legali ?? ''}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

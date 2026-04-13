import { useState } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { usePendingActivations, usePendingActivationRows } from '../api/queries';
import styles from './AttivazioniInCorsoPage.module.css';

export default function AttivazioniInCorsoPage() {
  const [selectedOrder, setSelectedOrder] = useState<string | null>(null);
  const activationsQ = usePendingActivations();
  const rowsQ = usePendingActivationRows(selectedOrder);

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Attivazioni in corso</h1>
      <p className={styles.subtitle}>
        Elenco ordini in stato confermato con righe da attivare
      </p>

      {activationsQ.isLoading && <Skeleton rows={8} />}

      {activationsQ.error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {activationsQ.data && (
        <>
          <div className={styles.info}>{activationsQ.data.length} ordini</div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
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
                    <td className={styles.mono}>{row.numero_ordine}</td>
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
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>Descrizione</th>
                    <th className={styles.numCol}>Quantita</th>
                    <th className={styles.numCol}>NRC</th>
                    <th className={styles.numCol}>MRC</th>
                    <th className={styles.numCol}>Totale MRC</th>
                    <th>Stato riga</th>
                    <th>Serial number</th>
                    <th>Note legali</th>
                  </tr>
                </thead>
                <tbody>
                  {rowsQ.data.map((row, i) => (
                    <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
                      <td>{row.descrizione_long ?? ''}</td>
                      <td className={styles.numCol}>{row.quantita ?? ''}</td>
                      <td className={styles.numCol}>{row.nrc != null ? row.nrc.toFixed(2) : ''}</td>
                      <td className={styles.numCol}>{row.mrc != null ? row.mrc.toFixed(2) : ''}</td>
                      <td className={styles.numCol}>{row.totale_mrc != null ? row.totale_mrc.toFixed(2) : ''}</td>
                      <td>{row.stato_riga ?? ''}</td>
                      <td className={styles.mono}>{row.serialnumber ?? ''}</td>
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

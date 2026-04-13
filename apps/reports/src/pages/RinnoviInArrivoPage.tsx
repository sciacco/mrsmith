import { useState, useEffect } from 'react';
import { Skeleton } from '@mrsmith/ui';
import { useUpcomingRenewals, useRenewalRows } from '../api/queries';
import styles from './RinnoviInArrivoPage.module.css';

export default function RinnoviInArrivoPage() {
  const [months, setMonths] = useState(4);
  const [minMrc, setMinMrc] = useState(11);
  const [selectedCustomer, setSelectedCustomer] = useState<string | null>(null);

  const renewalsQ = useUpcomingRenewals(months, minMrc);
  const rowsQ = useRenewalRows(selectedCustomer, months, minMrc);

  // Fetch on mount
  useEffect(() => {
    renewalsQ.refetch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function handleExecute() {
    setSelectedCustomer(null);
    renewalsQ.refetch();
  }

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Rinnovi in arrivo</h1>

      <div className={styles.toolbar}>
        <div className={styles.field}>
          <label>MRC minimo</label>
          <input
            type="number"
            value={minMrc}
            onChange={(e) => setMinMrc(Number(e.target.value))}
          />
        </div>

        <div className={styles.field}>
          <label>Rinnovi entro N mesi</label>
          <div className={styles.rangeWrap}>
            <input
              type="range"
              min={1}
              max={12}
              value={months}
              onChange={(e) => setMonths(Number(e.target.value))}
            />
            <span className={styles.rangeValue}>{months}</span>
          </div>
        </div>

        <button className={styles.btnPrimary} onClick={handleExecute}>
          Esegui
        </button>
      </div>

      {renewalsQ.isLoading && <Skeleton rows={8} />}

      {renewalsQ.error && <p>Errore nel caricamento dei dati.</p>}

      {renewalsQ.data && (
        <>
          <div className={styles.info}>{renewalsQ.data.length} clienti</div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th></th>
                  <th>Cliente</th>
                  <th>Rinnovi dal</th>
                  <th>Rinnovi al</th>
                  <th>Ordini/Servizi</th>
                  <th>Senza tacito rinnovo</th>
                  <th className={styles.numCol}>Canoni</th>
                </tr>
              </thead>
              <tbody>
                {renewalsQ.data.map((row, i) => (
                  <tr
                    key={row.numero_azienda}
                    className={selectedCustomer === row.numero_azienda ? styles.selectedRow : undefined}
                    onClick={() => setSelectedCustomer(
                      selectedCustomer === row.numero_azienda ? null : row.numero_azienda,
                    )}
                    style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}
                  >
                    <td><div className={styles.accentBar} /></td>
                    <td>{row.ragione_sociale}</td>
                    <td>{row.rinnovi_dal?.slice(0, 10) ?? ''}</td>
                    <td>{row.rinnovi_al?.slice(0, 10) ?? ''}</td>
                    <td>{row.ordini_servizi}</td>
                    <td>{row.senza_tacito_rinnovo ? 'Si' : 'No'}</td>
                    <td className={styles.numCol}>{row.canoni != null ? row.canoni.toFixed(2) : ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}

      {selectedCustomer && (
        <div className={styles.detailSection}>
          <h2 className={styles.detailTitle}>
            Dettaglio rinnovi
          </h2>

          {rowsQ.isLoading && <Skeleton rows={4} />}

          {rowsQ.error && <p>Errore nel caricamento delle righe.</p>}

          {rowsQ.data && (
            <div className={styles.tableWrap}>
              <table className={styles.table}>
                <thead>
                  <tr>
                    <th>N. Ordine</th>
                    <th>Stato ordine</th>
                    <th>Descrizione</th>
                    <th className={styles.numCol}>Quantita</th>
                    <th className={styles.numCol}>NRC</th>
                    <th className={styles.numCol}>MRC</th>
                    <th>Stato riga</th>
                    <th>Serial</th>
                    <th>Note</th>
                    <th>Data attivazione</th>
                    <th>Durata</th>
                    <th>Prossimo rinnovo</th>
                    <th>Sost. ord.</th>
                    <th>Sostituito da</th>
                    <th>Tacito rinnovo</th>
                  </tr>
                </thead>
                <tbody>
                  {rowsQ.data.map((row, i) => (
                    <tr key={i} style={{ animationDelay: `${Math.min(i * 20, 300)}ms` }}>
                      <td className={styles.mono}>{row.nome_testata_ordine}</td>
                      <td>{row.stato_ordine ?? ''}</td>
                      <td>{row.descrizione_long ?? ''}</td>
                      <td className={styles.numCol}>{row.quantita ?? ''}</td>
                      <td className={styles.numCol}>{row.nrc != null ? row.nrc.toFixed(2) : ''}</td>
                      <td className={styles.numCol}>{row.mrc != null ? row.mrc.toFixed(2) : ''}</td>
                      <td>{row.stato_riga ?? ''}</td>
                      <td className={styles.mono}>{row.serialnumber ?? ''}</td>
                      <td>{row.note_legali ?? ''}</td>
                      <td>{row.data_attivazione?.slice(0, 10) ?? ''}</td>
                      <td>{row.durata ?? ''}</td>
                      <td>{row.prossimo_rinnovo?.slice(0, 10) ?? ''}</td>
                      <td>{row.sost_ord ?? ''}</td>
                      <td>{row.sostituito_da ?? ''}</td>
                      <td>{row.tacito_rinnovo ?? ''}</td>
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

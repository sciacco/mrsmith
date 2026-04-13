import { Skeleton } from '@mrsmith/ui';
import { useMorAnomalies } from '../api/queries';
import shared from './shared.module.css';
import styles from './AnomalieMorPage.module.css';

export default function AnomalieMorPage() {
  const { data, isLoading, error } = useMorAnomalies();

  return (
    <div className={shared.page}>
      <h1 className={shared.title}>Anomalie MOR</h1>

      {isLoading && <Skeleton rows={8} />}

      {error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {data && (
        <div className={shared.tableWrap}>
          <table className={`${shared.table} ${styles.table}`}>
            <thead>
              <tr>
                <th>Conto</th>
                <th>Cognome</th>
                <th>Nome</th>
                <th>Da fatturare</th>
                <th>Codice ordine</th>
                <th>Serial number</th>
                <th>Periodo</th>
                <th>Importo</th>
                <th>Stato</th>
                <th>Tipologia</th>
                <th>Cliente</th>
                <th>Intestazione</th>
                <th>Ordine presente</th>
                <th>N. ordine corretto</th>
              </tr>
            </thead>
            <tbody>
              {data.map((row, i) => {
                const warn =
                  row.ordine_presente === 'NO' ||
                  row.numero_ordine_corretto === 'NO';
                return (
                  <tr
                    key={i}
                    className={warn ? styles.rowWarning : undefined}
                    style={{ animationDelay: `${i * 0.02}s` }}
                  >
                    <td>{row.conto}</td>
                    <td>{row.lastname}</td>
                    <td>{row.firstname}</td>
                    <td>{row.is_da_fatturare}</td>
                    <td>{row.codice_ordine}</td>
                    <td>{row.serialnumber}</td>
                    <td>{row.periodo_inizio}</td>
                    <td>{row.importo != null ? row.importo.toFixed(2) : ''}</td>
                    <td>{row.stato}</td>
                    <td>{row.tipologia}</td>
                    <td>{row.id_cliente}</td>
                    <td>{row.intestazione}</td>
                    <td>{row.ordine_presente}</td>
                    <td>{row.numero_ordine_corretto}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

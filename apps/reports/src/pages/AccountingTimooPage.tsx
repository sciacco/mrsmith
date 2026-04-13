import { Skeleton } from '@mrsmith/ui';
import { useTimooDailyStats } from '../api/queries';
import styles from './AccountingTimooPage.module.css';

export default function AccountingTimooPage() {
  const { data, isLoading, error } = useTimooDailyStats();

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Accounting TIMOO</h1>

      {isLoading && <Skeleton rows={8} />}

      {error && (
        <p>Errore nel caricamento dei dati.</p>
      )}

      {data && (
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Tenant ID</th>
                <th>Tenant</th>
                <th>Giorno</th>
                <th>Utenti</th>
                <th>Service Extensions</th>
              </tr>
            </thead>
            <tbody>
              {data.map((row, i) => (
                <tr key={i} style={{ animationDelay: `${i * 0.02}s` }}>
                  <td>{row.tenant_id}</td>
                  <td>{row.tenant_name}</td>
                  <td>{row.day}</td>
                  <td>{row.users}</td>
                  <td>{row.service_extensions}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

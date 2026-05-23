import { Icon } from '@mrsmith/ui';
import type { SendToERPResponse } from '../api/types';
import styles from '../pages/OrderDetailPage.module.css';

export function SendToErpResultPanel({ result }: { result: SendToERPResponse | null }) {
  if (!result) return null;
  const errors = result.rows.filter((row) => row.status === 'error').length;
  const success = result.rows.length - errors;
  return (
    <section className={`${styles.resultPanel} ${errors > 0 ? styles.resultPanelWarning : styles.resultPanelSuccess}`} aria-live="polite">
      <div className={styles.resultHeader}>
        <span className={styles.resultIcon}>
          <Icon name={errors > 0 ? 'triangle-alert' : 'check-circle'} size={18} />
        </span>
        <div>
          <strong>{errors > 0 ? 'Invio parziale' : 'Invio completato'}</strong>
          <p>{success} righe inviate, {errors} righe da verificare.</p>
        </div>
      </div>
      {result.warning ? <p className={styles.resultWarningText}>{warningText(result.warning)}</p> : null}
      <div className={styles.resultRows}>
        {result.rows.map((row) => (
          <div key={row.rowId} className={styles.resultRow}>
            <span>Riga {row.cdlan_systemodv_row ?? row.rowId}</span>
            <strong>{row.status === 'ok' ? 'OK' : 'Da verificare'}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function warningText(code: string): string {
  switch (code) {
    case 'arxivar_upload_failed':
      return 'Ordine inviato, ma il documento firmato richiede una verifica.';
    default:
      return 'Ordine inviato, ma una verifica resta in sospeso.';
  }
}

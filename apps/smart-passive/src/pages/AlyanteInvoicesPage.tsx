import { Button, Icon, Skeleton, StatusBadge, VisuallyHidden } from '@mrsmith/ui';
import { useAlyanteInvoices } from '../api/queries';
import type { AlyanteInvoiceRow } from '../types';
import styles from './AlyanteInvoicesPage.module.css';

type ColumnKind = 'text' | 'integer' | 'decimal' | 'money' | 'date' | 'boolean';

interface ColumnDef {
  key: keyof AlyanteInvoiceRow;
  label: string;
  kind: ColumnKind;
  group: 'documento' | 'scadenziario' | 'righe';
}

const columns: ColumnDef[] = [
  { key: 'DO11_DITTA_CG18', label: 'DO11_DITTA_CG18', kind: 'integer', group: 'documento' },
  { key: 'DO11_NUMREG_CO99', label: 'DO11_NUMREG_CO99', kind: 'integer', group: 'documento' },
  { key: 'DO11_DOCUM_MG36', label: 'DO11_DOCUM_MG36', kind: 'text', group: 'documento' },
  { key: 'MG36_DESCDOCUM', label: 'MG36_DESCDOCUM', kind: 'text', group: 'documento' },
  { key: 'DO11_NUMDOC', label: 'DO11_NUMDOC', kind: 'text', group: 'documento' },
  { key: 'DO11_SEZDOC', label: 'DO11_SEZDOC', kind: 'text', group: 'documento' },
  { key: 'DO11_DATADOC', label: 'DO11_DATADOC', kind: 'date', group: 'documento' },
  { key: 'DO11_CLIFOR_CG44', label: 'DO11_CLIFOR_CG44', kind: 'integer', group: 'documento' },
  { key: 'DO11_NUMDOCORIG', label: 'DO11_NUMDOCORIG', kind: 'text', group: 'documento' },
  { key: 'DO11_NOTEDOCUM', label: 'DO11_NOTEDOCUM', kind: 'text', group: 'documento' },
  { key: 'DO13_TOTDOCUMENTO', label: 'DO13_TOTDOCUMENTO', kind: 'money', group: 'scadenziario' },
  { key: 'DO13_TOTAPAGARE', label: 'DO13_TOTAPAGARE', kind: 'money', group: 'scadenziario' },
  { key: 'NUM_RATE_APERTE', label: 'NUM_RATE_APERTE', kind: 'integer', group: 'scadenziario' },
  { key: 'TOTRATE', label: 'TOTRATE', kind: 'integer', group: 'scadenziario' },
  { key: 'EF01_SCADE_S', label: 'EF01_SCADE_S', kind: 'date', group: 'scadenziario' },
  { key: 'EF01_IMPEFFORIG', label: 'EF01_IMPEFFORIG', kind: 'money', group: 'scadenziario' },
  { key: 'RESIDUO', label: 'RESIDUO', kind: 'money', group: 'scadenziario' },
  { key: 'PAGATO_SU_RESIDUO', label: 'PAGATO_SU_RESIDUO', kind: 'money', group: 'scadenziario' },
  { key: 'IN_SCADENZIARIO', label: 'IN_SCADENZIARIO', kind: 'boolean', group: 'scadenziario' },
  { key: 'DO30_PROGRIGA', label: 'DO30_PROGRIGA', kind: 'integer', group: 'righe' },
  { key: 'DO30_PROGVISUASTA', label: 'DO30_PROGVISUASTA', kind: 'integer', group: 'righe' },
  { key: 'DO30_INDTIPORIGA', label: 'DO30_INDTIPORIGA', kind: 'integer', group: 'righe' },
  { key: 'DO30_CODART_MG66', label: 'DO30_CODART_MG66', kind: 'text', group: 'righe' },
  { key: 'DO30_DESCART', label: 'DO30_DESCART', kind: 'text', group: 'righe' },
  { key: 'DO30_UM1', label: 'DO30_UM1', kind: 'text', group: 'righe' },
  { key: 'DO30_QTA1', label: 'DO30_QTA1', kind: 'decimal', group: 'righe' },
  { key: 'DO30_PREZZO1', label: 'DO30_PREZZO1', kind: 'money', group: 'righe' },
  { key: 'DO30_SCPER1', label: 'DO30_SCPER1', kind: 'decimal', group: 'righe' },
  { key: 'DO30_SCPER2', label: 'DO30_SCPER2', kind: 'decimal', group: 'righe' },
  { key: 'DO30_SCPER3', label: 'DO30_SCPER3', kind: 'decimal', group: 'righe' },
  { key: 'DO30_SCIMP', label: 'DO30_SCIMP', kind: 'money', group: 'righe' },
  { key: 'DO30_IMPORTO', label: 'DO30_IMPORTO', kind: 'money', group: 'righe' },
  { key: 'DO30_IMPNETSCP', label: 'DO30_IMPNETSCP', kind: 'money', group: 'righe' },
  { key: 'DO30_ALIVA_CG28', label: 'DO30_ALIVA_CG28', kind: 'text', group: 'righe' },
  { key: 'DO30_IMPORTOIVA', label: 'DO30_IMPORTOIVA', kind: 'money', group: 'righe' },
];

const numberFormatter = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 4 });
const integerFormatter = new Intl.NumberFormat('it-IT', { maximumFractionDigits: 0 });
const moneyFormatter = new Intl.NumberFormat('it-IT', { style: 'currency', currency: 'EUR' });
const dateFormatter = new Intl.DateTimeFormat('it-IT');

function formatDate(value: string | null): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return dateFormatter.format(date);
}

function formatValue(row: AlyanteInvoiceRow, column: ColumnDef): string {
  const value = row[column.key];
  if (value === null || value === undefined || value === '') return '—';
  if (column.kind === 'date') return formatDate(String(value));
  if (column.kind === 'integer' && typeof value === 'number') return integerFormatter.format(value);
  if (column.kind === 'decimal' && typeof value === 'number') return numberFormatter.format(value);
  if (column.kind === 'money' && typeof value === 'number') return moneyFormatter.format(value);
  if (column.kind === 'boolean') return value ? 'Sì' : 'No';
  return String(value);
}

function statusVariant(row: AlyanteInvoiceRow): 'warning' | 'danger' {
  return row.IN_SCADENZIARIO ? 'warning' : 'danger';
}

function statusLabel(row: AlyanteInvoiceRow) {
  return row.IN_SCADENZIARIO ? 'Residuo aperto' : 'Non in scadenziario';
}

export function AlyanteInvoicesPage() {
  const query = useAlyanteInvoices();
  const rows = query.data ?? [];

  return (
    <section className={styles.page}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Utility</p>
          <h1>Fatture Alyante</h1>
          <p className={styles.description}>
            Estrazione tecnica delle fatture di acquisto non saldate o non presenti nello scadenziario.
            La tabella conserva i riferimenti Alyante e ripete i campi di testata per ogni riga DO30.
          </p>
        </div>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
          leftIcon={<Icon name="refresh-cw" size={14} />}
        >
          Aggiorna
        </Button>
      </div>

      {query.isLoading && (
        <div className={styles.panel}>
          <Skeleton rows={12} />
        </div>
      )}

      {query.isError && (
        <div className={styles.error} role="alert">
          <span className={styles.errorIcon}><Icon name="triangle-alert" size={20} /></span>
          <div>
            <strong>Impossibile caricare le fatture Alyante.</strong>
            <p>Verificare la configurazione della connessione Alyante e riprovare.</p>
          </div>
        </div>
      )}

      {query.isSuccess && rows.length === 0 && (
        <div className={styles.empty}>
          <span className={styles.emptyIcon}><Icon name="database" size={32} /></span>
          <strong>Nessuna fattura da mostrare</strong>
          <p>L’estrazione non ha restituito fatture non saldate o mancanti nello scadenziario.</p>
        </div>
      )}

      {query.isSuccess && rows.length > 0 && (
        <div className={styles.tablePanel}>
          <div className={styles.tableMeta}>
            <span>Granularità: riga documento DO30</span>
            <span>Valori grezzi Alyante, formattati solo per consultazione</span>
          </div>
          <div className={styles.tableWrap}>
            <table className={styles.table}>
              <VisuallyHidden as="caption">
                Fatture Alyante non saldate o non presenti nello scadenziario
              </VisuallyHidden>
              <thead>
                <tr>
                  <th className={styles.stickyColumn}>Stato</th>
                  {columns.map((column) => (
                    <th key={column.key} className={styles[column.group]}>
                      {column.label}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((row, index) => (
                  <tr key={`${row.DO11_NUMREG_CO99 ?? 'doc'}-${row.DO30_PROGRIGA ?? index}`}>
                    <td className={styles.stickyColumn}>
                      <StatusBadge value={statusLabel(row)} variant={statusVariant(row)} />
                    </td>
                    {columns.map((column) => (
                      <td
                        key={column.key}
                        className={column.kind === 'money' || column.kind === 'decimal' || column.kind === 'integer' ? styles.numeric : undefined}
                      >
                        {formatValue(row, column)}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </section>
  );
}

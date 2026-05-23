import { useState } from 'react';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import type { OrderDetail, OrderRow } from '../api/types';
import { formatDate, formatEmpty, formatMoney } from '../lib/formatters';
import { canEditSerialNumber, canOpenActivationModal } from '../lib/permissions';
import styles from '../pages/OrderDetailPage.module.css';

interface RigheTabProps {
  order: OrderDetail;
  rows: OrderRow[];
  loading: boolean;
  roles: readonly string[] | undefined;
  savingRowId: number | null;
  onSaveSerial: (rowId: number, serialNumber: string) => void;
  onActivate: (row: OrderRow) => void;
}

export function RigheTab({ order, rows, loading, roles, savingRowId, onSaveSerial, onActivate }: RigheTabProps) {
  const [editingRow, setEditingRow] = useState<number | null>(null);
  const [serialDraft, setSerialDraft] = useState('');
  const serialEditable = canEditSerialNumber(order);

  function startEdit(row: OrderRow) {
    setEditingRow(row.id);
    setSerialDraft(row.cdlan_serialnumber ?? '');
  }

  function cancelEdit() {
    setEditingRow(null);
    setSerialDraft('');
  }

  function save(row: OrderRow) {
    onSaveSerial(row.id, serialDraft);
    cancelEdit();
  }

  if (loading) return <section className={styles.cardSection}><Skeleton rows={8} /></section>;

  return (
    <section className={styles.cardSection}>
      <div className={styles.sectionHeader}>
        <h2>Righe ordine</h2>
        <span className={styles.countPill}>{rows.length} righe</span>
      </div>
      {rows.length === 0 ? (
        <div className={styles.emptyState}>
          <Icon name="package" size={30} />
          <strong>Nessuna riga ordine</strong>
          <p>Le righe associate all'ordine verranno mostrate qui.</p>
        </div>
      ) : (
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Bundle</th>
                <th>Articolo</th>
                <th>Descrizione</th>
                <th>Quantità</th>
                <th>Canone</th>
                <th>Prezzo attivazione</th>
                <th>Seriale</th>
                <th>Data attivazione</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => (
                <tr key={row.id} style={{ animationDelay: `${Math.min(index * 20, 260)}ms` }}>
                  <td className={styles.mono}>{formatEmpty(row.bundle_code)}</td>
                  <td className={styles.mono}>{formatEmpty(row.cdlan_codart)}</td>
                  <td><span className={styles.rowDescription}>{formatEmpty(row.cdlan_descart)}</span></td>
                  <td className={styles.numCell}>{row.cdlan_qta ?? '—'}</td>
                  <td className={styles.numCell}>{formatMoney(row.canone, order.cdlan_valuta ?? 'EUR')}</td>
                  <td className={styles.numCell}>{formatMoney(row.activation_price, order.cdlan_valuta ?? 'EUR')}</td>
                  <td>
                    {editingRow === row.id ? (
                      <div className={styles.inlineEdit}>
                        <input className={styles.inputCompact} value={serialDraft} onChange={(event) => setSerialDraft(event.target.value)} />
                        <button type="button" className={styles.iconButton} onClick={() => save(row)} aria-label="Salva seriale">
                          <Icon name="check" size={15} />
                        </button>
                        <button type="button" className={styles.iconButton} onClick={cancelEdit} aria-label="Annulla modifica seriale">
                          <Icon name="x" size={15} />
                        </button>
                      </div>
                    ) : (
                      <span className={styles.serialValue}>{formatEmpty(row.cdlan_serialnumber)}</span>
                    )}
                  </td>
                  <td>{formatDate(row.cdlan_data_attivazione)}</td>
                  <td>
                    <div className={styles.rowActions}>
                      <Button variant="secondary" size="sm" disabled={!serialEditable || savingRowId === row.id} loading={savingRowId === row.id} onClick={() => startEdit(row)}>
                        Seriale
                      </Button>
                      <Button variant="secondary" size="sm" disabled={!canOpenActivationModal(order, roles, row)} onClick={() => onActivate(row)}>
                        Modifica
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

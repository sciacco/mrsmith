import { useState } from 'react';
import { Button, Drawer, Icon, Skeleton } from '@mrsmith/ui';
import type { OrderDetail, OrderRow } from '../api/types';
import { dateInputValue, formatDate, formatEmpty, formatMoney } from '../lib/formatters';
import { canEditSerialNumber, canOpenActivationModal } from '../lib/permissions';
import styles from '../pages/OrderDetailPage.module.css';

function stripHtml(html: string | null | undefined): string {
  if (!html) return '';
  return html
    .replace(/<[^>]*>/g, ' ')
    .replace(/\s+/g, ' ')
    .replace(/&nbsp;/gi, ' ')
    .replace(/&agrave;/gi, 'à')
    .replace(/&egrave;/gi, 'è')
    .replace(/&igrave;/gi, 'ì')
    .replace(/&ograve;/gi, 'ò')
    .replace(/&ugrave;/gi, 'ù')
    .replace(/&aacute;/gi, 'á')
    .replace(/&eacute;/gi, 'é')
    .replace(/&iacute;/gi, 'í')
    .replace(/&oacute;/gi, 'ó')
    .replace(/&uacute;/gi, 'ú')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .trim();
}

function validDateInputValue(value: string | null | undefined): string {
  const input = dateInputValue(value);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(input)) return '';
  const parsed = new Date(`${input}T00:00:00Z`);
  if (Number.isNaN(parsed.getTime())) return '';
  const [year, month, day] = input.split('-').map(Number);
  if (parsed.getUTCFullYear() !== year || parsed.getUTCMonth() + 1 !== month || parsed.getUTCDate() !== day) {
    return '';
  }
  return input;
}

interface RigheTabProps {
  order: OrderDetail;
  rows: OrderRow[];
  loading: boolean;
  roles: readonly string[] | undefined;
  savingRowId: number | null;
  activationLoading: boolean;
  onSaveSerial: (rowId: number, serialNumber: string) => Promise<void>;
  onActivate: (rowId: number, date: string) => Promise<void>;
}

export function RigheTab({ order, rows, loading, roles, savingRowId, activationLoading, onSaveSerial, onActivate }: RigheTabProps) {
  const [selectedRow, setSelectedRow] = useState<OrderRow | null>(null);
  const [serialDraft, setSerialDraft] = useState('');
  const [activationDate, setActivationDate] = useState('');
  const serialEditable = canEditSerialNumber(order);
  const selectedRowSaving = selectedRow != null && savingRowId === selectedRow.id;
  const drawerBusy = selectedRowSaving || activationLoading;

  const handleRowClick = (row: OrderRow) => {
    setSelectedRow(row);
    setSerialDraft(row.cdlan_serialnumber ?? '');
    setActivationDate(canOpenActivationModal(order, roles, row) ? validDateInputValue(row.cdlan_data_attivazione) : '');
  };

  const handleCloseDrawer = () => {
    if (drawerBusy) return;
    setSelectedRow(null);
    setSerialDraft('');
    setActivationDate('');
  };

  const handleSaveSerial = async () => {
    if (!selectedRow) return;
    try {
      await onSaveSerial(selectedRow.id, serialDraft);
      setSelectedRow(null);
      setSerialDraft('');
      setActivationDate('');
    } catch {
      // The parent page owns the user-facing error toast; keep the drawer open.
    }
  };

  const handleActivate = async () => {
    if (!selectedRow) return;
    try {
      await onActivate(selectedRow.id, activationDate);
      setSelectedRow(null);
      setSerialDraft('');
      setActivationDate('');
    } catch {
      // The parent page owns the user-facing error toast; keep the drawer open.
    }
  };

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
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => {
                const isCanceled = row.data_annullamento != null;
                const trClass = [
                  selectedRow?.id === row.id ? styles.rowActive : '',
                  isCanceled ? styles.rowCanceled : '',
                ].filter(Boolean).join(' ');

                return (
                  <tr
                    key={row.id}
                    style={{ animationDelay: `${Math.min(index * 20, 260)}ms` }}
                    className={trClass || undefined}
                    onClick={() => handleRowClick(row)}
                  >
                    <td className={styles.mono}>{formatEmpty(row.bundle_code)}</td>
                    <td className={styles.mono}>{formatEmpty(row.cdlan_codart)}</td>
                    <td>
                      {row.cdlan_descart ? (
                        <div
                          className={styles.rowDescription}
                          dangerouslySetInnerHTML={{ __html: row.cdlan_descart }}
                        />
                      ) : (
                        <span className={styles.rowDescription}>—</span>
                      )}
                    </td>
                    <td className={styles.numCell}>{row.cdlan_qta ?? '—'}</td>
                    <td className={styles.numCell}>{formatMoney(row.canone, order.cdlan_valuta ?? 'EUR')}</td>
                    <td className={styles.numCell}>{formatMoney(row.activation_price, order.cdlan_valuta ?? 'EUR')}</td>
                    <td>
                      <span className={styles.serialValue}>{formatEmpty(row.cdlan_serialnumber)}</span>
                    </td>
                    <td>
                      {isCanceled ? (
                        <span className={styles.canceledLabel}>Annullata</span>
                      ) : (
                        formatDate(row.cdlan_data_attivazione)
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Drawer
        open={selectedRow != null}
        onClose={handleCloseDrawer}
        title="Gestione Riga Ordine"
        subtitle={selectedRow ? `${selectedRow.cdlan_codart} - ${stripHtml(selectedRow.cdlan_descart)}` : ''}
        size="md"
        footer={
          selectedRow && (
            <div className={styles.drawerActions}>
              <Button variant="secondary" onClick={handleCloseDrawer} disabled={drawerBusy}>
                Chiudi
              </Button>
              {serialEditable && (
                <Button
                  variant="primary"
                  disabled={drawerBusy}
                  loading={selectedRowSaving}
                  onClick={() => void handleSaveSerial()}
                >
                  Salva seriale
                </Button>
              )}
              {canOpenActivationModal(order, roles, selectedRow) && (
                <Button
                  variant="primary"
                  disabled={!activationDate || drawerBusy}
                  loading={activationLoading}
                  onClick={() => void handleActivate()}
                  leftIcon={<Icon name="check" size={14} />}
                >
                  Conferma attivazione
                </Button>
              )}
            </div>
          )
        }
      >
        {selectedRow && (
          <div className={styles.drawerContent}>
            {/* Sezione 1: Dati Articolo */}
            <div className={styles.drawerSection}>
              <h3 className={styles.drawerSectionTitle}>Dati Articolo</h3>
              <div className={styles.drawerGrid}>
                <div className={styles.drawerFactItem}>
                  <span>Bundle</span>
                  <strong className={styles.mono}>{formatEmpty(selectedRow.bundle_code)}</strong>
                </div>
                <div className={styles.drawerFactItem}>
                  <span>Codice articolo</span>
                  <strong className={styles.mono}>{formatEmpty(selectedRow.cdlan_codart)}</strong>
                </div>
                <div className={styles.drawerFactItem}>
                  <span>Quantità</span>
                  <strong>{selectedRow.cdlan_qta ?? '—'}</strong>
                </div>
                <div className={styles.drawerFactItem}>
                  <span>Canone</span>
                  <strong>{formatMoney(selectedRow.canone, order.cdlan_valuta ?? 'EUR')}</strong>
                </div>
                <div className={styles.drawerFactItem}>
                  <span>Prezzo attivazione</span>
                  <strong>{formatMoney(selectedRow.activation_price, order.cdlan_valuta ?? 'EUR')}</strong>
                </div>
              </div>
            </div>

            {/* Sezione 1.5: Descrizione Articolo */}
            {selectedRow.cdlan_descart && (
              <div className={styles.drawerSection}>
                <h3 className={styles.drawerSectionTitle}>Descrizione Articolo</h3>
                <div
                  className={styles.drawerDescriptionHtml}
                  dangerouslySetInnerHTML={{ __html: selectedRow.cdlan_descart }}
                />
              </div>
            )}

            {/* Sezione 2: Seriale e Stato */}
            <div className={styles.drawerSection}>
              <h3 className={styles.drawerSectionTitle}>Seriale & Attivazione</h3>
              <div className={styles.drawerMetaList}>
                {selectedRow.data_annullamento ? (
                  <div className={`${styles.drawerStatusCard} ${styles.drawerStatusCardCanceled}`}>
                    <Icon name="x-circle" size={20} />
                    <div>
                      <strong>Riga Annullata</strong>
                      <span>Servizio annullato il {formatDate(selectedRow.data_annullamento)}</span>
                    </div>
                  </div>
                ) : selectedRow.confirm_data_attivazione === 1 || selectedRow.cdlan_data_attivazione ? (
                  <div className={`${styles.drawerStatusCard} ${styles.drawerStatusCardActive}`}>
                    <Icon name="check-circle" size={20} />
                    <div>
                      <strong>Servizio Attivo</strong>
                      <span>Attivato il {formatDate(selectedRow.cdlan_data_attivazione)}</span>
                    </div>
                  </div>
                ) : (
                  <div className={`${styles.drawerStatusCard} ${styles.drawerStatusCardPending}`}>
                    <Icon name="clock" size={20} />
                    <div>
                      <strong>In attesa di attivazione</strong>
                      <span>Riga d'ordine pronta per l'attivazione.</span>
                    </div>
                  </div>
                )}

                {/* Form Seriale */}
                {serialEditable ? (
                  <label className={styles.drawerFieldLabel}>
                    <span>Numero seriale</span>
                    <input
                      className={styles.drawerInput}
                      value={serialDraft}
                      placeholder="Inserisci il numero di serie..."
                      onChange={(e) => setSerialDraft(e.target.value)}
                    />
                  </label>
                ) : selectedRow.cdlan_serialnumber ? (
                  <div className={styles.drawerFactItem}>
                    <span>Numero seriale</span>
                    <strong className={styles.mono}>{selectedRow.cdlan_serialnumber}</strong>
                  </div>
                ) : null}

                {/* Sezione Attivazione se abilitata */}
                {canOpenActivationModal(order, roles, selectedRow) && (
                  <div className={styles.drawerActivationAction}>
                    <h4>Attivazione Servizio</h4>
                    <p>Conferma la data in cui il servizio è stato effettivamente attivato su questa riga.</p>
                    <label className={styles.drawerFieldLabel} style={{ width: '100%' }}>
                      <span>Data attivazione</span>
                      <input
                        type="date"
                        required
                        className={styles.drawerInput}
                        value={activationDate}
                        onChange={(e) => setActivationDate(e.target.value)}
                      />
                    </label>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}
      </Drawer>
    </section>
  );
}

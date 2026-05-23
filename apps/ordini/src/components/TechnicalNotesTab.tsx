import { useState } from 'react';
import { Button, Icon, Skeleton } from '@mrsmith/ui';
import type { TechnicalRow } from '../api/types';
import { formatDate, formatEmpty } from '../lib/formatters';
import styles from '../pages/OrderDetailPage.module.css';

interface TechnicalNotesTabProps {
  rows: TechnicalRow[];
  loading: boolean;
  savingRowId: number | null;
  onSaveNotes: (rowId: number, notes: string) => void;
}

export function TechnicalNotesTab({ rows, loading, savingRowId, onSaveNotes }: TechnicalNotesTabProps) {
  const [editingRow, setEditingRow] = useState<number | null>(null);
  const [notesDraft, setNotesDraft] = useState('');

  function startEdit(row: TechnicalRow) {
    setEditingRow(row.id);
    setNotesDraft(row.note_tecnici ?? '');
  }

  function cancelEdit() {
    setEditingRow(null);
    setNotesDraft('');
  }

  function save(row: TechnicalRow) {
    onSaveNotes(row.id, notesDraft);
    cancelEdit();
  }

  if (loading) return <section className={styles.cardSection}><Skeleton rows={8} /></section>;

  return (
    <section className={styles.cardSection}>
      <div className={styles.sectionHeader}>
        <h2>Informazioni dai tecnici</h2>
      </div>
      {rows.length === 0 ? (
        <div className={styles.emptyState}>
          <Icon name="clipboard-check" size={30} />
          <strong>Nessuna informazione tecnica</strong>
          <p>Le note tecniche associate alle righe verranno mostrate qui.</p>
        </div>
      ) : (
        <div className={styles.tableScroll}>
          <table className={styles.dataTable}>
            <thead>
              <tr>
                <th>Bundle</th>
                <th>Articolo</th>
                <th>Descrizione</th>
                <th>Note tecniche</th>
                <th>Data annullamento</th>
                <th>Azioni</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => (
                <tr key={row.id} style={{ animationDelay: `${Math.min(index * 20, 260)}ms` }}>
                  <td className={styles.mono}>{formatEmpty(row.bundle_code)}</td>
                  <td className={styles.mono}>{formatEmpty(row.cdlan_codart)}</td>
                  <td><span className={styles.rowDescription}>{formatEmpty(row.cdlan_descart)}</span></td>
                  <td className={styles.notesCell}>
                    {editingRow === row.id ? (
                      <textarea className={styles.textareaCompact} value={notesDraft} onChange={(event) => setNotesDraft(event.target.value)} />
                    ) : (
                      <span>{formatEmpty(row.note_tecnici)}</span>
                    )}
                  </td>
                  <td>{formatDate(row.data_annullamento)}</td>
                  <td>
                    {editingRow === row.id ? (
                      <div className={styles.rowActions}>
                        <Button size="sm" loading={savingRowId === row.id} onClick={() => save(row)}>Salva</Button>
                        <Button variant="secondary" size="sm" onClick={cancelEdit}>Annulla</Button>
                      </div>
                    ) : (
                      <Button variant="secondary" size="sm" onClick={() => startEdit(row)}>
                        Note
                      </Button>
                    )}
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

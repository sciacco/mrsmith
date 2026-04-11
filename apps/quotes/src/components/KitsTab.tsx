import { useCallback, useMemo, useState } from 'react';
import { Button, Icon } from '@mrsmith/ui';
import { useQuoteRows, useAddRow, useDeleteRow, useUpdateRowPosition } from '../api/queries';
import type { DocumentType, QuoteRow } from '../api/types';
import { KitAccordion } from './KitAccordion';
import { KitPickerModal } from './KitPickerModal';
import { ConfirmDialog } from './ConfirmDialog';
import styles from './KitsTab.module.css';

interface KitsTabProps {
  quoteId: number;
  documentType: DocumentType;
}

export function KitsTab({ quoteId, documentType }: KitsTabProps) {
  const { data: rows } = useQuoteRows(quoteId);
  const addRow = useAddRow();
  const deleteRow = useDeleteRow();
  const updatePosition = useUpdateRowPosition();
  const [showPicker, setShowPicker] = useState(false);
  const [dragId, setDragId] = useState<number | null>(null);
  const [pendingDeleteRowId, setPendingDeleteRowId] = useState<number | null>(null);
  const [pendingDeleteRow, setPendingDeleteRow] = useState<QuoteRow | null>(null);

  const handleAdd = useCallback((kitId: number) => {
    addRow.mutate({ quoteId, kitId });
  }, [addRow, quoteId]);

  const requestDelete = useCallback((row: QuoteRow) => {
    setPendingDeleteRow(row);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!pendingDeleteRow) return;
    const rowId = pendingDeleteRow.id;
    setPendingDeleteRow(null);
    setPendingDeleteRowId(rowId);
    try {
      await deleteRow.mutateAsync({ quoteId, rowId });
    } finally {
      setPendingDeleteRowId(null);
    }
  }, [deleteRow, pendingDeleteRow, quoteId]);

  const cancelDelete = useCallback(() => setPendingDeleteRow(null), []);

  const handleDragStart = useCallback((_e: React.DragEvent, rowId: number) => {
    setDragId(rowId);
  }, []);

  const handleDrop = useCallback(async (_e: React.DragEvent, targetId: number) => {
    if (dragId === null || dragId === targetId || !rows) return;
    const dragRow = rows.find(r => r.id === dragId);
    const targetRow = rows.find(r => r.id === targetId);
    if (dragRow && targetRow) {
      await updatePosition.mutateAsync({ quoteId, rowId: dragId, position: targetRow.position });
    }
    setDragId(null);
  }, [dragId, rows, updatePosition, quoteId]);

  const totals = useMemo(() => {
    if (!rows) return { nrc: 0, mrc: 0 };
    return rows.reduce(
      (acc, r) => ({ nrc: acc.nrc + r.nrc_row, mrc: acc.mrc + r.mrc_row }),
      { nrc: 0, mrc: 0 },
    );
  }, [rows]);

  const hasRows = rows && rows.length > 0;

  return (
    <div className={styles.wrap}>
      <div className={styles.header}>
        <div>
          <div className={styles.sectionTitle}>Kit e prodotti</div>
          <p className={styles.hint}>
            Aggiungi kit alla proposta e configura varianti, quantità e gruppi obbligatori.
          </p>
        </div>
        <Button
          variant="primary"
          leftIcon={<Icon name="plus" size={16} />}
          onClick={() => setShowPicker(true)}
        >
          Aggiungi kit
        </Button>
      </div>

      {hasRows ? (
        <div className={styles.list}>
          {rows.map(row => (
            <KitAccordion
              key={row.id}
              row={row}
              quoteId={quoteId}
              documentType={documentType}
              onDelete={() => requestDelete(row)}
              isDeleting={pendingDeleteRowId === row.id}
              draggable
              onDragStart={handleDragStart}
              onDrop={handleDrop}
            />
          ))}
        </div>
      ) : (
        <div className={styles.empty}>
          <div className={styles.emptyIcon}>
            <Icon name="package" size={28} strokeWidth={1.5} />
          </div>
          <div className={styles.emptyTitle}>Nessun kit aggiunto</div>
          <div className={styles.emptyText}>
            Apri il catalogo per aggiungere i primi kit alla proposta.
          </div>
          <Button
            variant="secondary"
            leftIcon={<Icon name="plus" size={16} />}
            onClick={() => setShowPicker(true)}
          >
            Apri catalogo
          </Button>
        </div>
      )}

      {hasRows && (
        <div className={styles.totalsBar}>
          <div className={styles.totalsItem}>
            <span className={styles.totalsLabel}>NRC Totale</span>
            <span className={styles.totalsValue}>{totals.nrc.toFixed(2)}</span>
          </div>
          <div className={styles.totalsDivider} aria-hidden="true" />
          <div className={styles.totalsItem}>
            <span className={styles.totalsLabel}>MRC Totale</span>
            <span className={styles.totalsValue}>{totals.mrc.toFixed(2)}</span>
          </div>
        </div>
      )}

      <KitPickerModal
        open={showPicker}
        onSelect={handleAdd}
        onClose={() => setShowPicker(false)}
      />

      <ConfirmDialog
        open={pendingDeleteRow !== null}
        title="Eliminare il kit?"
        message={
          pendingDeleteRow
            ? `Il kit "${pendingDeleteRow.internal_name}" e tutti i prodotti configurati verranno rimossi dalla proposta. L'operazione non può essere annullata.`
            : ''
        }
        confirmLabel="Elimina kit"
        variant="danger"
        onConfirm={() => void confirmDelete()}
        onCancel={cancelDelete}
      />
    </div>
  );
}

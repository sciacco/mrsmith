import { useCallback, useState } from 'react';
import { useQuoteRows, useAddRow, useDeleteRow, useUpdateRowPosition } from '../api/queries';
import type { DocumentType } from '../api/types';
import { KitAccordion } from './KitAccordion';
import { KitPickerModal } from './KitPickerModal';
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

  const handleAdd = useCallback((kitId: number) => {
    addRow.mutate({ quoteId, kitId });
  }, [addRow, quoteId]);

  const handleDelete = useCallback(async (rowId: number) => {
    setPendingDeleteRowId(rowId);
    try {
      await deleteRow.mutateAsync({ quoteId, rowId });
    } finally {
      setPendingDeleteRowId(null);
    }
  }, [deleteRow, quoteId]);

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

  return (
    <div className={styles.wrap}>
      <button className={styles.addBtn} onClick={() => setShowPicker(true)}>
        + Aggiungi kit
      </button>

      {rows && rows.length > 0 ? (
        rows.map(row => (
          <KitAccordion
            key={row.id}
            row={row}
            quoteId={quoteId}
            documentType={documentType}
            onDelete={handleDelete}
            isDeleting={pendingDeleteRowId === row.id}
            draggable
            onDragStart={handleDragStart}
            onDrop={handleDrop}
          />
        ))
      ) : (
        <div className={styles.empty}>Nessun kit aggiunto. Clicca &quot;Aggiungi kit&quot; per iniziare.</div>
      )}

      <KitPickerModal
        open={showPicker}
        onSelect={handleAdd}
        onClose={() => setShowPicker(false)}
      />
    </div>
  );
}

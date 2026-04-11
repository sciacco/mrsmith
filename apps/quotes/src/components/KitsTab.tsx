import { useCallback, useMemo, useState } from 'react';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
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
  const [pendingDeleteRowId, setPendingDeleteRowId] = useState<number | null>(null);
  const [pendingDeleteRow, setPendingDeleteRow] = useState<QuoteRow | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

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

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event;
    if (!over || active.id === over.id || !rows) return;
    const targetRow = rows.find(r => r.id === Number(over.id));
    if (!targetRow) return;
    updatePosition.mutate({
      quoteId,
      rowId: Number(active.id),
      position: targetRow.position,
    });
  }, [rows, updatePosition, quoteId]);

  const totals = useMemo(() => {
    if (!rows) return { nrc: 0, mrc: 0 };
    return rows.reduce(
      (acc, r) => ({ nrc: acc.nrc + r.nrc_row, mrc: acc.mrc + r.mrc_row }),
      { nrc: 0, mrc: 0 },
    );
  }, [rows]);

  const hasRows = rows && rows.length > 0;
  const rowIds = useMemo(() => (rows ?? []).map(r => r.id), [rows]);

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
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={handleDragEnd}
        >
          <SortableContext items={rowIds} strategy={verticalListSortingStrategy}>
            <div className={styles.list}>
              {rows.map(row => (
                <KitAccordion
                  key={row.id}
                  row={row}
                  quoteId={quoteId}
                  documentType={documentType}
                  onDelete={() => requestDelete(row)}
                  isDeleting={pendingDeleteRowId === row.id}
                  sortable
                />
              ))}
            </div>
          </SortableContext>
        </DndContext>
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

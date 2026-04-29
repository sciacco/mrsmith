import { Button, Icon, useToast } from '@mrsmith/ui';
import { useState } from 'react';
import { useDeleteRow } from '../api/queries';
import type { PoDetail, PoRow } from '../api/types';
import { formatMoney } from '../lib/format';
import { ConfirmDialog } from './ConfirmDialog';
import { RowModal } from './RowModal';
import { RowTable } from './RowTable';

export function RowsTab({ po, editable }: { po: PoDetail; editable: boolean }) {
  const [modalOpen, setModalOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<PoRow | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PoRow | null>(null);
  const remove = useDeleteRow();
  const { toast } = useToast();

  async function confirmDelete() {
    if (!deleteTarget) return;
    try {
      await remove.mutateAsync({ id: po.id, rowId: deleteTarget.id });
      toast('Riga eliminata');
      setDeleteTarget(null);
    } catch {
      toast('Eliminazione non riuscita', 'error');
    }
  }

  function openNewRow() {
    setEditTarget(null);
    setModalOpen(true);
  }

  function openEditRow(row: PoRow) {
    setEditTarget(row);
    setModalOpen(true);
  }

  function closeModal() {
    setModalOpen(false);
    setEditTarget(null);
  }

  return (
    <div className="stack">
      <div className="surfaceHeader">
        <h2>Totale PO: {formatMoney(po.total_price, po.currency)}</h2>
        <Button size="sm" leftIcon={<Icon name="plus" />} disabled={!editable} onClick={openNewRow}>
          Nuova riga
        </Button>
      </div>
      <RowTable
        rows={po.rows ?? []}
        currency={po.currency}
        editable={editable}
        emptyLabel="Nessuna riga PO presente."
        onEdit={openEditRow}
        onDelete={setDeleteTarget}
      />
      <RowModal poId={po.id} currency={po.currency} open={modalOpen} row={editTarget} onClose={closeModal} />
      <ConfirmDialog
        open={deleteTarget != null}
        title="Elimina riga"
        message="Confermi eliminazione della riga selezionata?"
        confirmLabel="Elimina"
        danger
        loading={remove.isPending}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => void confirmDelete()}
      />
    </div>
  );
}
